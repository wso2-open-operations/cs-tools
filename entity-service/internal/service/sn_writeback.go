// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	// snWritebackTimeout bounds a single ServiceNow mirror write. It runs on
	// a context.WithoutCancel copy of the triggering request's context, so
	// without its own bound it could hang indefinitely on a slow downstream
	// call, now that it's no longer tied to the request's own deadline.
	snWritebackTimeout = 10 * time.Second
	// snWritebackQueueSize is the dispatcher's buffered job channel size.
	// Dispatch drops (logs and records as a failure) rather than blocking
	// the caller's request when the queue is full — see Dispatch's own doc
	// comment.
	snWritebackQueueSize = 256
	// snWritebackWorkers is the fixed number of worker goroutines the
	// dispatcher starts at construction — a small bounded pool, not one
	// goroutine per call.
	snWritebackWorkers = 4
)

// snWritebackJob is one queued best-effort ServiceNow mirror write.
type snWritebackJob struct {
	ctx        context.Context
	entityType string
	entityID   string
	operation  string
	payload    any
	writeFn    func(context.Context) error
	// key identifies the entity this job mirrors ("entityType:entityID").
	// The dispatcher uses it to serialize jobs for the same entity — see
	// keyQueues below — so two writes queued for the same case/entity always
	// apply to ServiceNow in the order they were dispatched, even though
	// different entities' jobs run concurrently across the worker pool.
	key string
}

// SNWritebackDispatcher runs best-effort, one-way ServiceNow mirror writes
// for DATA_SOURCE=postgres-servicenow-dual-write (see
// config.DataSourcePostgresServiceNowDualWrite). Postgres is always written
// synchronously, in the request path, before Dispatch is ever called —
// Dispatch only ever fires after that commit already succeeded. The
// ServiceNow write it queues runs on a fixed background worker pool, fully
// detached from the triggering request: the request returns to its caller
// without waiting for it, and its own context is never cancelled by the
// request completing (see snWritebackTimeout).
//
// No automatic retry: a single attempt, and on failure a row is inserted into
// sn_writeback_failures (SNWritebackFailureRepository) and an ERROR is logged
// with the record, the operation and the reason -- a failure is never silent.
// Nothing reads that table back automatically. It is read back for people:
// the record's own detail lists what of it the previous system is missing
// (ListFailures; domain.ChangeRequest.MirrorFailures) and a row is re-sent on
// request (Replay, through the replayer the owning service registered with
// RegisterReplay), which clears it only when the re-send succeeds.
type SNWritebackDispatcher struct {
	failures repository.SNWritebackFailureRepository
	jobs     chan snWritebackJob

	// keyMu guards keyQueues, the per-entity ("entityType:entityID")
	// ordering state described on run and startOrQueue below.
	keyMu     sync.Mutex
	keyQueues map[string][]snWritebackJob

	// replayMu guards replayers, keyed "entityType:operation" -- see
	// RegisterReplay.
	replayMu  sync.RWMutex
	replayers map[string]SNWritebackReplayFunc
}

// SNWritebackReplayFunc re-sends one recorded failure: the entity's id and
// the payload exactly as the failure row stored it (the marshaled payload the
// original Dispatch was given). It is registered per entity type and operation
// by the service that owns that write (RegisterReplay), so the dispatcher
// itself never needs to know what a payload means.
type SNWritebackReplayFunc func(ctx context.Context, entityID string, payload json.RawMessage) error

// NewSNWritebackDispatcher constructs an SNWritebackDispatcher and starts
// its fixed pool of background workers. failures must not be nil — the
// dispatcher only makes sense where a Postgres pool (and therefore this
// table) is available, which DATA_SOURCE=postgres-servicenow-dual-write
// guarantees (see config.Config.Validate's dbRequired check).
func NewSNWritebackDispatcher(failures repository.SNWritebackFailureRepository) *SNWritebackDispatcher {
	d := &SNWritebackDispatcher{
		failures:  failures,
		jobs:      make(chan snWritebackJob, snWritebackQueueSize),
		keyQueues: make(map[string][]snWritebackJob),
		replayers: make(map[string]SNWritebackReplayFunc),
	}
	for i := 0; i < snWritebackWorkers; i++ {
		go d.worker()
	}
	return d
}

// RegisterReplay registers how a recorded failure of entityType/operation is
// re-sent (see Replay). The last registration for a pair wins; the services
// that own the writes register theirs at construction.
func (d *SNWritebackDispatcher) RegisterReplay(entityType, operation string, fn SNWritebackReplayFunc) {
	d.replayMu.Lock()
	defer d.replayMu.Unlock()
	d.replayers[entityType+":"+operation] = fn
}

func (d *SNWritebackDispatcher) replayer(entityType, operation string) (SNWritebackReplayFunc, bool) {
	d.replayMu.RLock()
	defer d.replayMu.RUnlock()
	fn, ok := d.replayers[entityType+":"+operation]
	return fn, ok
}

// ListFailures returns the outstanding failure rows of one entity, oldest
// first: the order the writes were made in, which is the order to replay them.
func (d *SNWritebackDispatcher) ListFailures(ctx context.Context, entityType, entityID string) ([]domain.SNWritebackFailure, error) {
	return d.failures.ListByEntity(ctx, entityType, entityID)
}

// ListSNWritebackFailures implements SNWritebackFailureService: the
// outstanding rows newest first, optionally narrowed to one entity type and
// id, at most limit (the repository caps it).
func (d *SNWritebackDispatcher) ListSNWritebackFailures(ctx context.Context, entityType, entityID string, limit int) (domain.SNWritebackFailureListResponse, error) {
	rows, err := d.failures.List(ctx, entityType, entityID, limit)
	if err != nil {
		return domain.SNWritebackFailureListResponse{}, err
	}
	return domain.SNWritebackFailureListResponse{Failures: rows}, nil
}

// ReplaySNWritebackFailure implements SNWritebackFailureService: it re-sends
// the recorded write SYNCHRONOUSLY, with the registered replayer for its
// entity type and operation, and clears the row only when the re-send
// succeeds. A re-send that fails again leaves the row, with its error replaced
// by the new reason, and returns that reason (a refusal by the previous system
// comes back as the 4xx it was, so the caller sees why). A row no replayer is
// registered for is a 400 naming the pair. Bounded by snWritebackTimeout like a
// dispatched write.
//
// A replay is for a quiet record: it runs outside the dispatcher's per-entity
// ordering, so replay once no write of that record is still queued (the
// failure rows of one record are listed in dispatch order; replay them in that
// order).
func (d *SNWritebackDispatcher) ReplaySNWritebackFailure(ctx context.Context, id string) (domain.SNWritebackReplayResponse, error) {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.SNWritebackReplayResponse{}, err
	}
	f, err := d.failures.GetByID(ctx, id)
	if err != nil {
		return domain.SNWritebackReplayResponse{}, err
	}
	fn, ok := d.replayer(f.EntityType, f.Operation)
	if !ok {
		return domain.SNWritebackReplayResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf(
			"mirror write failure %s cannot be replayed: no replay is registered for %s/%s on this data source", f.ID, f.EntityType, f.Operation)}
	}
	writeCtx, cancel := context.WithTimeout(ctx, snWritebackTimeout)
	defer cancel()
	if err := fn(writeCtx, f.EntityID, f.Payload); err != nil {
		slog.ErrorContext(ctx, "sn writeback: replay of a mirror write failed again",
			"failureId", f.ID, "entityType", f.EntityType, "entityId", f.EntityID, "operation", f.Operation, "error", err)
		if uerr := d.failures.UpdateError(ctx, f.ID, err.Error()); uerr != nil {
			slog.ErrorContext(ctx, "sn writeback: replay failed and recording the new reason also failed",
				"failureId", f.ID, "entityType", f.EntityType, "entityId", f.EntityID, "operation", f.Operation, "writeErr", err, "recordErr", uerr)
		}
		return domain.SNWritebackReplayResponse{}, err
	}
	if err := d.failures.Delete(ctx, f.ID); err != nil {
		// The write reached the previous system; the row is the only thing
		// left wrong. Said loudly: a stale row would be replayed once more,
		// which for an idempotent PATCH is harmless and for a journal entry
		// is a duplicate.
		slog.ErrorContext(ctx, "sn writeback: replay succeeded but the failure row could not be cleared",
			"failureId", f.ID, "entityType", f.EntityType, "entityId", f.EntityID, "operation", f.Operation, "error", err)
		return domain.SNWritebackReplayResponse{}, err
	}
	slog.InfoContext(ctx, "sn writeback: mirror write replayed and the failure row cleared",
		"failureId", f.ID, "entityType", f.EntityType, "entityId", f.EntityID, "operation", f.Operation)
	return domain.SNWritebackReplayResponse{Message: "Mirror write replayed; the failure record is cleared", Failure: f}, nil
}

func (d *SNWritebackDispatcher) worker() {
	for job := range d.jobs {
		d.run(job)
	}
}

// run executes job, then drains any later jobs that were queued for the
// same entity (job.key) while job was in flight, running them in the exact
// order they were dispatched — one at a time, on this same goroutine —
// before releasing the key. This is what prevents two writeback jobs for
// the same entity (e.g. two case WatchList updates) from being picked up by
// different workers and completing out of order: without it, a slower
// worker could apply an older write to ServiceNow after a newer one already
// landed, and both would report success with nothing to flag the drift.
// Jobs for different keys are unaffected and keep running fully in
// parallel across the worker pool.
func (d *SNWritebackDispatcher) run(job snWritebackJob) {
	for {
		d.runOne(job)

		next, ok := d.dequeueNext(job.key)
		if !ok {
			return
		}
		job = next
	}
}

// dequeueNext pops the next pending job queued for key, if any. Returning
// false also releases key: startOrQueue treats an absent key as free, so the
// next Dispatch for this entity starts a fresh job on the channel/worker
// pool instead of piggy-backing on this (finished) run.
func (d *SNWritebackDispatcher) dequeueNext(key string) (snWritebackJob, bool) {
	d.keyMu.Lock()
	defer d.keyMu.Unlock()

	queue := d.keyQueues[key]
	if len(queue) == 0 {
		delete(d.keyQueues, key)
		return snWritebackJob{}, false
	}

	next := queue[0]
	if len(queue) == 1 {
		d.keyQueues[key] = nil // key stays claimed (about to run next) but empty
	} else {
		d.keyQueues[key] = queue[1:]
	}
	return next, true
}

// startOrQueue registers job under its key. If no job for that key is
// currently in flight, it claims the key and returns true — the caller
// (Dispatch) is then responsible for handing job to a worker. Otherwise job
// is appended behind whatever is already queued for that key and startOrQueue
// returns false: the goroutine currently draining that key's queue (see run)
// will pick job up in order, so Dispatch must not hand it to a worker itself
// — doing so would let it run concurrently with, and possibly finish before,
// the job(s) ahead of it for the same entity.
func (d *SNWritebackDispatcher) startOrQueue(job snWritebackJob) bool {
	d.keyMu.Lock()
	defer d.keyMu.Unlock()

	if _, inFlight := d.keyQueues[job.key]; inFlight {
		d.keyQueues[job.key] = append(d.keyQueues[job.key], job)
		return false
	}
	d.keyQueues[job.key] = nil
	return true
}

func (d *SNWritebackDispatcher) runOne(job snWritebackJob) {
	writeCtx, cancel := context.WithTimeout(job.ctx, snWritebackTimeout)
	defer cancel()

	err := job.writeFn(writeCtx)
	if err == nil {
		return
	}
	d.record(writeCtx, job, err)
}

// record writes the failure row for job and logs it at ERROR with everything
// a person needs to find it: the record, the operation, the reason (the
// previous system's status and message, as the client reports them, or this
// service's own reason for not sending it) and the row's id, which the
// replay takes. Every path that ends in a failure -- a refused or failed
// write, a write a job declined to send, a full queue -- comes through here,
// so none is quieter than another.
func (d *SNWritebackDispatcher) record(ctx context.Context, job snWritebackJob, err error) {
	payload, marshalErr := json.Marshal(job.payload)
	if marshalErr != nil {
		// The payload itself couldn't be recorded — still record the
		// failure, with the marshal error folded into the message, rather
		// than silently dropping it. An empty JSON object keeps the column
		// NOT NULL-valid.
		payload = json.RawMessage(`{}`)
		err = fmt.Errorf("%w (payload could not be marshaled for the failure record: %v)", err, marshalErr)
	}

	f, recErr := d.failures.Create(ctx, domain.CreateSNWritebackFailureRequest{
		EntityType: job.entityType,
		EntityID:   job.entityID,
		Operation:  job.operation,
		Payload:    payload,
		Error:      err.Error(),
	})
	if recErr != nil {
		slog.ErrorContext(ctx, "sn writeback: mirror write to the previous system failed and recording the failure also failed",
			"entityType", job.entityType, "entityId", job.entityID, "operation", job.operation, "writeErr", err, "recordErr", recErr)
		return
	}
	slog.ErrorContext(ctx, "sn writeback: mirror write to the previous system failed; recorded for replay",
		"failureId", f.ID, "entityType", job.entityType, "entityId", job.entityID, "operation", job.operation, "error", err)
}

// Dispatch queues writeFn to run on the background worker pool and returns
// immediately — it never blocks the caller on the ServiceNow write itself
// (see the queue-full case below for the one situation where it still adds
// a small amount of local, non-ServiceNow latency).
// ctx is only used to derive the detached background context (via
// context.WithoutCancel); it is not otherwise consulted, so Dispatch always
// enqueues regardless of ctx's own state.
//
// Jobs sharing the same entityType+entityID are serialized: if a job for
// that entity is already queued or running, this one is appended behind it
// (via startOrQueue) and handed to a worker only once its predecessor(s)
// finish, in the order they were dispatched — see run's doc comment. This
// is what stops two writes for the same entity (e.g. two case WatchList
// updates) from racing across different workers and applying to ServiceNow
// out of order. Jobs for different entities are unaffected and still run
// fully in parallel across the pool.
//
// If the queue is full (snWritebackQueueSize jobs already pending — meaning
// ServiceNow mirror writes are backing up faster than the pool can drain
// them), Dispatch still never touches ServiceNow itself: it logs and
// records the drop as a failure via a synchronous local Postgres insert
// instead (the same outcome a queued attempt would have on failure). That
// insert briefly blocks the caller — deliberately: firing it into yet
// another goroutine would just let failure records pile up unbounded
// against a queue that's already full, the same problem this branch exists
// to avoid. What Dispatch guarantees is no blocking on ServiceNow network
// I/O, never zero added latency. This synthetic failure never touches
// ServiceNow, so it runs immediately regardless of per-entity ordering —
// it can only ever precede, never race, a real write for the same entity,
// because startOrQueue already claimed the entity's key on this call's
// behalf (jobs appended after it are held back until this run drains).
func (d *SNWritebackDispatcher) Dispatch(ctx context.Context, entityType, entityID, operation string, payload any, writeFn func(context.Context) error) {
	job := snWritebackJob{
		ctx:        context.WithoutCancel(ctx),
		entityType: entityType,
		entityID:   entityID,
		operation:  operation,
		payload:    payload,
		writeFn:    writeFn,
		key:        entityType + ":" + entityID,
	}

	if !d.startOrQueue(job) {
		// A job for this entity is already in flight; the goroutine draining
		// it (run) will pick this one up in order once it's done.
		return
	}

	select {
	case d.jobs <- job:
	default:
		slog.WarnContext(ctx, "sn writeback: queue full, dropping ServiceNow mirror write without attempting it",
			"entityType", entityType, "entityId", entityID, "operation", operation)
		d.run(snWritebackJob{
			ctx:        job.ctx,
			entityType: entityType,
			entityID:   entityID,
			operation:  operation,
			payload:    payload,
			key:        job.key,
			writeFn: func(context.Context) error {
				return errQueueFull
			},
		})
	}
}

// errQueueFull is the synthetic error recorded when Dispatch's queue is full
// — see Dispatch's own doc comment. It never reaches ServiceNow: run's
// writeFn call fails immediately with this instead of attempting the write.
var errQueueFull = fmt.Errorf("sn writeback queue full (%d jobs pending); write not attempted", snWritebackQueueSize)
