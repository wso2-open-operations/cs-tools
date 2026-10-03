// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

// Package allocator batches alerts into id-range claims against alert_seq, then writes every row in parallel, retrying failures until a filler row stands in.
package allocator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"sre-alert-ingestion-service/internal/model"
	"sre-alert-ingestion-service/internal/postgres"
)

// Errors returned by Submit; all of them map to 503.
var (
	ErrQueueFull      = errors.New("alert queue full")
	ErrQueueBytesFull = errors.New("alert queue memory limit reached")
	ErrShuttingDown   = errors.New("shutting down")
	ErrClaimFailed    = errors.New("could not claim alert ids")
	ErrStoreFailed    = errors.New("alert could not be stored")
	ErrTimeout        = errors.New("timed out waiting for storage")
)

// fillerPrefix marks a row alerts-core can't parse as an alert, so it skips the id without waiting.
const fillerPrefix = "VOID: "

// memLogEvery rate-limits the "queue memory limit reached" warning.
const memLogEvery = time.Minute

// Store is the storage the allocator needs; *postgres.Store implements it.
type Store interface {
	// ClaimRange reserves n consecutive alert ids in one round trip and returns the first.
	ClaimRange(ctx context.Context, n int) (start int64, err error)
	// InsertBatch writes every row in one pipelined round trip, one error per row in input order.
	InsertBatch(ctx context.Context, rows []postgres.InsertRow) []error
	Insert(ctx context.Context, id, source string, alert []byte) error
	// InsertFiller writes the filler only if id has no row; otherwise existing is that row's alert.
	InsertFiller(ctx context.Context, id, source, filler string) (applied bool, existing string, err error)
}

// StoreFailure describes an alert that couldn't be written after every attempt.
type StoreFailure struct {
	Source        string
	RequestID     string
	AltID         string
	Alert         model.Alert
	Err           error
	FillerWritten bool
}

// FailureNotifier is told about every alert that couldn't be stored.
type FailureNotifier interface {
	StoreFailed(StoreFailure)
}

// Waker wakes alerts-core once per written batch. Implementations must not block.
type Waker interface {
	Wake()
}

// Config tunes the allocator; see config.toml.example.
type Config struct {
	QueueSize int
	// QueueMaxBytes caps the size of everything accepted but not yet finished.
	QueueMaxBytes    int64
	MaxBatch         int
	WriteConcurrency int
	InsertAttempts   int
	InsertBaseDelay  time.Duration
	// QueryTimeout is store.query_timeout; no insert starts with less than this left.
	QueryTimeout time.Duration
	// WriteDeadline, from claim time, bounds retries of failed writes and the claim call itself.
	WriteDeadline time.Duration
}

// Result is what a submitter receives: its ids, in submission order, or an error.
type Result struct {
	IDs []string
	Err error
}

type submission struct {
	source    string
	requestID string
	alerts    []model.Alert
	size      int64
	released  atomic.Bool
	done      chan Result // buffered(1): the writer never blocks on a submitter that gave up
}

// Allocator is safe for concurrent Submit calls.
type Allocator struct {
	logger   *slog.Logger
	store    Store
	notifier FailureNotifier
	waker    Waker
	cfg      Config

	mu          sync.RWMutex // guards closed against the close of queue
	closed      bool
	queue       chan *submission
	writeSem    chan struct{} // one token per busy writer; the claimer reserves them
	writes      sync.WaitGroup
	claimerDone chan struct{}

	bytes      atomic.Int64 // size of accepted, unfinished submissions
	memRejects atomic.Int64
	memLogAt   atomic.Int64 // unix nanos of the last memory-limit warning

	pendingMu sync.Mutex
	pending   map[int64]struct{} // claimed ids whose row or filler isn't written yet

	// Shutdown drain: once Close starts, retries stop at stopAt and each unwritten id gets one filler attempt.
	drainCh   chan struct{}
	drainOnce sync.Once
	stopAt    atomic.Int64 // unix nanos; 0 until a drain deadline is set
}

// New starts the claimer goroutine. notifier and waker may be nil.
func New(logger *slog.Logger, store Store, notifier FailureNotifier, waker Waker, cfg Config) *Allocator {
	a := &Allocator{
		logger:      logger,
		store:       store,
		notifier:    notifier,
		waker:       waker,
		cfg:         cfg,
		queue:       make(chan *submission, cfg.QueueSize),
		writeSem:    make(chan struct{}, cfg.WriteConcurrency),
		claimerDone: make(chan struct{}),
		pending:     map[int64]struct{}{},
		drainCh:     make(chan struct{}),
	}
	go a.claimLoop()
	return a
}

// Submit queues alerts and waits for their ids; a full queue fails immediately, and a ctx timeout still lets the write finish since a claimed id must never be left empty.
func (a *Allocator) Submit(ctx context.Context, source, requestID string, alerts []model.Alert) ([]string, error) {
	if len(alerts) == 0 {
		return []string{}, nil
	}
	size := sizeOf(alerts)
	if !a.reserveBytes(size) {
		a.noteMemReject()
		return nil, ErrQueueBytesFull
	}
	sub := &submission{source: source, requestID: requestID, alerts: alerts, size: size, done: make(chan Result, 1)}

	a.mu.RLock()
	if a.closed {
		a.mu.RUnlock()
		a.releaseBytes(sub)
		return nil, ErrShuttingDown
	}
	select {
	case a.queue <- sub:
		a.mu.RUnlock()
	default:
		a.mu.RUnlock()
		a.releaseBytes(sub)
		return nil, ErrQueueFull
	}

	select {
	case res := <-sub.done:
		return res.IDs, res.Err
	case <-ctx.Done():
		a.logger.Warn("request gave up waiting; its alerts are still being written",
			"request_id", requestID, "source", source, "alerts", len(alerts))
		return nil, ErrTimeout
	}
}

// QueueBytes is the size of accepted submissions not yet finished.
func (a *Allocator) QueueBytes() int64 { return a.bytes.Load() }

// sizeOf estimates a submission's memory, counting each distinct description once (a Prometheus batch shares one).
func sizeOf(alerts []model.Alert) int64 {
	var n int64
	seen := make(map[string]struct{}, 1)
	for _, al := range alerts {
		n += int64(len(al.MetricName) + len(al.UniqueIdentifier) + len(al.Service) + len(al.Category) +
			len(al.Environment) + len(al.Source) + len(al.Severity))
		if _, ok := seen[al.Description]; !ok {
			seen[al.Description] = struct{}{}
			n += int64(len(al.Description))
		}
	}
	return n
}

func (a *Allocator) reserveBytes(size int64) bool {
	for {
		cur := a.bytes.Load()
		if cur+size > a.cfg.QueueMaxBytes {
			return false
		}
		if a.bytes.CompareAndSwap(cur, cur+size) {
			return true
		}
	}
}

// releaseBytes returns a submission's size exactly once.
func (a *Allocator) releaseBytes(sub *submission) {
	if sub.released.CompareAndSwap(false, true) {
		a.bytes.Add(-sub.size)
	}
}

// finish releases a submission's bytes and delivers its result.
func (a *Allocator) finish(sub *submission, res Result) {
	a.releaseBytes(sub)
	sub.done <- res
}

func (a *Allocator) noteMemReject() {
	a.memRejects.Add(1)
	now := time.Now().UnixNano()
	last := a.memLogAt.Load()
	if now-last < int64(memLogEvery) || !a.memLogAt.CompareAndSwap(last, now) {
		return
	}
	a.logger.Warn("queue memory limit reached", "rejected", a.memRejects.Swap(0),
		"queue_bytes", a.bytes.Load(), "queue_max_bytes", a.cfg.QueueMaxBytes)
}

// Close stops accepting submissions, drains the queue, and waits for every write to finish or ctx to end; safe to call more than once.
func (a *Allocator) Close(ctx context.Context) error {
	a.mu.Lock()
	if !a.closed {
		a.closed = true
		close(a.queue)
	}
	a.mu.Unlock()
	if dl, ok := ctx.Deadline(); ok {
		// Leave time for one filler write per unwritten id.
		a.drainOnce.Do(func() {
			a.stopAt.Store(dl.Add(-2 * a.cfg.QueryTimeout).UnixNano())
			close(a.drainCh)
		})
	}

	select {
	case <-a.claimerDone:
	case <-ctx.Done():
		a.logUnwritten()
		return fmt.Errorf("claimer did not drain: %w", ctx.Err())
	}
	written := make(chan struct{})
	go func() {
		a.writes.Wait()
		close(written)
	}()
	select {
	case <-written:
		return nil
	case <-ctx.Done():
		a.logUnwritten()
		return fmt.Errorf("writers did not finish: %w", ctx.Err())
	}
}

// maxLoggedIDs bounds the id list in the unwritten-ids log line.
const maxLoggedIDs = 100

// logUnwritten names the claimed ids left without a row, which alerts-core will wait gap_timeout on.
func (a *Allocator) logUnwritten() {
	a.pendingMu.Lock()
	seqs := make([]int64, 0, len(a.pending))
	for s := range a.pending {
		seqs = append(seqs, s)
	}
	a.pendingMu.Unlock()
	if len(seqs) == 0 {
		return
	}
	slices.Sort(seqs)
	ids := make([]string, 0, min(len(seqs), maxLoggedIDs))
	for _, s := range seqs[:min(len(seqs), maxLoggedIDs)] {
		ids = append(ids, postgres.FormatID(s))
	}
	a.logger.Error("shutdown cut off writes; these ids have no row and alerts-core will wait gap_timeout on them",
		"count", len(seqs), "first_id", postgres.FormatID(seqs[0]),
		"last_id", postgres.FormatID(seqs[len(seqs)-1]), "ids", ids)
}

// reserveSlots blocks until k writer slots are held.
func (a *Allocator) reserveSlots(k int) {
	for range k {
		a.writeSem <- struct{}{}
	}
}

func (a *Allocator) releaseSlots(k int) {
	for range k {
		<-a.writeSem
	}
}

// claimLoop batches whatever is waiting, up to MaxBatch, never splitting a submission across claims; write_concurrency gates concurrent claimed-group writes.
func (a *Allocator) claimLoop() {
	defer close(a.claimerDone)
	var carry *submission
	for {
		first := carry
		carry = nil
		if first == nil {
			var ok bool
			if first, ok = <-a.queue; !ok {
				return
			}
		}
		a.reserveSlots(1)
		batch := []*submission{first}
		n := len(first.alerts)
	gather:
		for n < a.cfg.MaxBatch {
			select {
			case sub, ok := <-a.queue:
				if !ok {
					break gather
				}
				if n+len(sub.alerts) > a.cfg.MaxBatch {
					carry = sub
					break gather
				}
				batch = append(batch, sub)
				n += len(sub.alerts)
			default:
				break gather
			}
		}

		claimStart := time.Now()
		start, err := a.claim(n)
		if err != nil {
			a.releaseSlots(1)
			a.logger.Error("claim failed; batch rejected, no ids claimed", "alerts", n,
				"submissions", len(batch), "error", err)
			for _, sub := range batch {
				a.finish(sub, Result{Err: ErrClaimFailed})
			}
			continue
		}
		a.logger.Info("batch claimed", "alerts", n, "submissions", len(batch),
			"first_id", postgres.FormatID(start), "last_id", postgres.FormatID(start+int64(n)-1),
			"claim_ms", time.Since(claimStart).Milliseconds(),
			"queue_len", len(a.queue), "queue_bytes", a.bytes.Load())
		a.pendingMu.Lock()
		for s := start; s < start+int64(n); s++ {
			a.pending[s] = struct{}{}
		}
		a.pendingMu.Unlock()
		a.writes.Add(1)
		go a.writeBatch(batch, start, claimStart)
	}
}

// claim reserves n consecutive ids with a single call; a Postgres sequence can never collide under concurrent claimers, so there is no retry loop here.
func (a *Allocator) claim(n int) (start int64, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.WriteDeadline)
	defer cancel()
	start, err = a.store.ClaimRange(ctx, n)
	if err != nil {
		return 0, fmt.Errorf("claim %d ids: %w", n, err)
	}
	return start, nil
}

// sleep waits d, cut short to the shutdown stop time once a drain starts.
func (a *Allocator) sleep(d time.Duration) {
	start := time.Now()
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return
	case <-a.drainCh:
	}
	rest := min(d-time.Since(start), time.Until(time.Unix(0, a.stopAt.Load())))
	if rest > 0 {
		time.Sleep(rest)
	}
}

// shuttingDown reports whether a shutdown drain has reached its stop time.
func (a *Allocator) shuttingDown() bool {
	at := a.stopAt.Load()
	return at != 0 && time.Now().UnixNano() >= at
}

type alertJob struct {
	sub      *submission
	subIndex int // this submission's position in the claimed batch, for outcomes[subIndex]
	index    int
	seq      int64
	deadline time.Time
	id       string
	alert    model.Alert
	body     []byte
}

// writeBatch writes the claimed group in one pipelined round trip, retries any row that failed individually, replies to each submitter, and wakes alerts-core once if anything was stored.
func (a *Allocator) writeBatch(batch []*submission, start int64, claimedAt time.Time) {
	defer a.releaseSlots(1)
	defer a.writes.Done()
	writeStart := time.Now()
	deadline := claimedAt.Add(a.cfg.WriteDeadline)

	type outcome struct {
		ids    []string
		failed atomic.Bool
	}
	outcomes := make([]outcome, len(batch))
	for si, sub := range batch {
		outcomes[si].ids = make([]string, len(sub.alerts))
	}

	jobs := make([]alertJob, 0, len(batch))
	seq := start
	for si, sub := range batch {
		for i := range sub.alerts {
			alert := sub.alerts[i]
			id := postgres.FormatID(seq)
			if alert.UniqueIdentifier == "" {
				a.logger.Warn("source sent no id; using the alert id as unique_identifier, so this alert "+
					"won't merge with repeats or resolve on recovery",
					"request_id", sub.requestID, "source", sub.source, "alt_id", id)
				alert.UniqueIdentifier = id
			}
			job := alertJob{sub: sub, subIndex: si, index: i, seq: seq, deadline: deadline, id: id, alert: alert}
			outcomes[si].ids[i] = id
			body, err := json.Marshal(alert)
			if err != nil {
				// Unreachable for a struct of strings; handled so the id still gets a row.
				outcomes[si].failed.Store(true)
				a.fail(job, id, alert, err)
				a.pendingMu.Lock()
				delete(a.pending, seq)
				a.pendingMu.Unlock()
			} else {
				job.body = body
				jobs = append(jobs, job)
			}
			seq++
		}
	}

	rows := make([]postgres.InsertRow, len(jobs))
	for i, job := range jobs {
		rows[i] = postgres.InsertRow{ID: job.id, Source: job.sub.source, Alert: job.body}
	}
	var errs []error
	if len(rows) > 0 {
		if time.Until(deadline) >= a.cfg.QueryTimeout && !a.shuttingDown() {
			ctx, cancel := context.WithDeadline(context.Background(), deadline)
			errs = a.store.InsertBatch(ctx, rows)
			cancel()
		} else {
			errs = make([]error, len(rows))
			for i := range errs {
				errs[i] = a.stopErr()
			}
		}
	}

	var stored atomic.Bool
	var retryWG sync.WaitGroup
	for i, job := range jobs {
		if errs[i] == nil {
			stored.Store(true)
			a.pendingMu.Lock()
			delete(a.pending, job.seq)
			a.pendingMu.Unlock()
			continue
		}
		job, firstErr := job, errs[i]
		a.writes.Add(1)
		retryWG.Add(1)
		go func() {
			defer a.writes.Done()
			defer retryWG.Done()
			if ok := a.retryAfterBatchFailure(job, firstErr); ok {
				stored.Store(true)
			} else {
				outcomes[job.subIndex].failed.Store(true)
			}
		}()
	}
	retryWG.Wait()

	failed := 0
	for i := range outcomes {
		if outcomes[i].failed.Load() {
			failed++
		}
	}
	a.logger.Info("batch written", "alerts", seq-start, "first_id", postgres.FormatID(start),
		"failed_submissions", failed, "write_ms", time.Since(writeStart).Milliseconds())

	for si, sub := range batch {
		if outcomes[si].failed.Load() {
			a.finish(sub, Result{IDs: outcomes[si].ids, Err: ErrStoreFailed})
		} else {
			a.finish(sub, Result{IDs: outcomes[si].ids})
		}
	}
	if stored.Load() && a.waker != nil {
		a.waker.Wake()
	}
}

// retryAfterBatchFailure continues retrying one alert after its batch attempt failed with firstErr, then writes a filler row; ok reports whether the real alert was stored.
func (a *Allocator) retryAfterBatchFailure(job alertJob, firstErr error) (ok bool) {
	defer func() {
		a.pendingMu.Lock()
		delete(a.pending, job.seq)
		a.pendingMu.Unlock()
	}()
	ctx := context.Background() // detached from the request
	w := writeState{a: a, job: job, id: job.id, attempts: 1}
	failures := 0
	delay := a.cfg.InsertBaseDelay
	if w.failed(firstErr, &failures, &delay) {
		return a.fail(job, job.id, job.alert, firstErr)
	}
	for {
		if time.Until(job.deadline) < a.cfg.QueryTimeout || a.shuttingDown() {
			return a.fail(job, job.id, job.alert, w.stopErr())
		}
		err := a.store.Insert(ctx, job.id, job.sub.source, job.body)
		w.attempts++
		if err == nil {
			return true
		}
		if w.failed(err, &failures, &delay) {
			return a.fail(job, job.id, job.alert, err)
		}
	}
}

// stopErr reports why a write phase stopped before it could attempt an insert.
func (a *Allocator) stopErr() error {
	if a.shuttingDown() {
		return errors.New("shutdown stopped retries before the alert was stored")
	}
	return errors.New("write deadline passed before the alert could be written")
}

// writeState tracks one alert's retries for logging.
type writeState struct {
	a        *Allocator
	job      alertJob
	id       string
	attempts int
}

// failed counts an insert error, waits before the next attempt, and reports whether the attempts are used up.
func (w *writeState) failed(err error, failures *int, delay *time.Duration) bool {
	*failures++
	w.a.logger.Warn("insert failed, retrying on the same id", "request_id", w.job.sub.requestID,
		"source", w.job.sub.source, "alt_id", w.id, "attempt", *failures, "error", err)
	if *failures >= w.a.cfg.InsertAttempts {
		return true
	}
	w.a.sleep(*delay)
	*delay *= 2
	return false
}

func (w *writeState) stopErr() error {
	if w.a.shuttingDown() {
		return errors.New("shutdown stopped retries before the alert was stored")
	}
	return errors.New("write deadline passed before the alert could be written")
}

// fail writes the filler row without overwriting a row that already landed, and returns writeOne's ok.
func (a *Allocator) fail(job alertJob, id string, alert model.Alert, cause error) bool {
	filler := fillerPrefix + cause.Error()
	var existing string
	var fillerErr error
	failures := 0
	delay := a.cfg.InsertBaseDelay
	for {
		applied, prev, err := a.store.InsertFiller(context.Background(), id, job.sub.source, filler)
		fillerErr = err
		if err == nil {
			if !applied {
				existing = prev
			}
			break
		}
		if a.shuttingDown() {
			break
		}
		failures++
		a.logger.Warn("filler row failed, retrying", "request_id", job.sub.requestID,
			"source", job.sub.source, "alt_id", id, "attempt", failures, "error", err)
		if failures >= a.cfg.InsertAttempts {
			break
		}
		a.sleep(delay)
		delay *= 2
	}
	if existing != "" && !strings.HasPrefix(existing, fillerPrefix) {
		a.logger.Warn("alert stored although its write reported a failure; filler not written",
			"request_id", job.sub.requestID, "source", job.sub.source, "alt_id", id, "error", cause)
		return true
	}
	if fillerErr != nil {
		// Postgres is likely down: alerts-core will wait gap_timeout on this id.
		a.logger.Error("alert NOT stored and filler row failed; alerts-core will stall on this id until gap_timeout",
			"request_id", job.sub.requestID, "source", job.sub.source, "alt_id", id,
			"alert", alert, "error", cause, "filler_error", fillerErr)
	} else {
		a.logger.Error("alert NOT stored; filler row written so alerts-core skips the id",
			"request_id", job.sub.requestID, "source", job.sub.source, "alt_id", id,
			"alert", alert, "error", cause)
	}
	if a.notifier != nil {
		a.notifier.StoreFailed(StoreFailure{
			Source: job.sub.source, RequestID: job.sub.requestID, AltID: id,
			Alert: alert, Err: cause, FillerWritten: fillerErr == nil,
		})
	}
	return false
}
