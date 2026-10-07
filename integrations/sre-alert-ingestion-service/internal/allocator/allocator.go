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

// Package allocator batches submissions, claims alert ids from alert_seq and writes each batch in one idempotent statement, retrying the whole batch on failure.
package allocator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
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

// memLogEvery rate-limits the "queue memory limit reached" warning.
const memLogEvery = time.Minute

// Store is the storage the allocator needs; *postgres.Store implements it.
type Store interface {
	// ClaimIDs reserves n unique alert ids in one round trip.
	ClaimIDs(ctx context.Context, n int) ([]int64, error)
	// InsertBatch writes every row or none; retrying the same rows is safe.
	InsertBatch(ctx context.Context, rows []postgres.InsertRow) error
}

// Waker wakes alerts-core once per written batch. Implementations must not block.
type Waker interface {
	Wake()
}

// Fallback is told about alerts that could not be stored after the last retry, and when a batch is stored again; *dbfallback.Client implements it.
type Fallback interface {
	Notify(source, requestID string, alerts []model.Alert)
	Recovered()
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
	// WriteDeadline, from batch start, bounds the claim and every insert retry.
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
	waker    Waker
	fallback Fallback
	cfg      Config

	mu          sync.RWMutex // guards closed against the close of queue
	closed      bool
	queue       chan *submission
	writeSem    chan struct{} // one token per busy writer
	writes      sync.WaitGroup
	claimerDone chan struct{}

	bytes      atomic.Int64 // size of accepted, unfinished submissions
	memRejects atomic.Int64
	memLogAt   atomic.Int64 // unix nanos of the last memory-limit warning

	// Shutdown drain: once Close starts, retries stop at stopAt.
	drainCh   chan struct{}
	drainOnce sync.Once
	stopAt    atomic.Int64 // unix nanos; 0 until a drain deadline is set
}

// New starts the batching goroutine. waker and fallback may be nil.
func New(logger *slog.Logger, store Store, waker Waker, fallback Fallback, cfg Config) *Allocator {
	a := &Allocator{
		logger:      logger,
		store:       store,
		waker:       waker,
		fallback:    fallback,
		cfg:         cfg,
		queue:       make(chan *submission, cfg.QueueSize),
		writeSem:    make(chan struct{}, cfg.WriteConcurrency),
		claimerDone: make(chan struct{}),
		drainCh:     make(chan struct{}),
	}
	go a.batchLoop()
	return a
}

// Submit queues alerts and waits for their ids; a full queue fails immediately, and a ctx timeout still lets the write finish.
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
		// Leave one query timeout for the final attempt to finish.
		a.drainOnce.Do(func() {
			a.stopAt.Store(dl.Add(-a.cfg.QueryTimeout).UnixNano())
			close(a.drainCh)
		})
	}

	select {
	case <-a.claimerDone:
	case <-ctx.Done():
		return fmt.Errorf("batcher did not drain: %w", ctx.Err())
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
		return fmt.Errorf("writers did not finish: %w", ctx.Err())
	}
}

// batchLoop gathers whatever is waiting, up to MaxBatch, never splitting a submission, and hands each batch to a free writer.
func (a *Allocator) batchLoop() {
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
		// Wait for a writer before gathering, so a busy store lets more submissions coalesce into this batch.
		a.writeSem <- struct{}{}

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

		a.writes.Add(1)
		go a.writeBatch(batch, n)
	}
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

// writeBatch claims ids for the batch, writes it in one statement with retries, replies to each submitter, and wakes alerts-core once.
func (a *Allocator) writeBatch(batch []*submission, n int) {
	defer a.writes.Done()
	defer func() { <-a.writeSem }()
	start := time.Now()
	deadline := start.Add(a.cfg.WriteDeadline)

	claimCtx, cancel := context.WithDeadline(context.Background(), deadline)
	seqs, err := a.store.ClaimIDs(claimCtx, n)
	cancel()
	if err != nil {
		a.logger.Error("claim failed; batch rejected, no rows written", "alerts", n, "submissions", len(batch), "error", err)
		// Queued before answering so the fallback holds the alerts by the time Submit returns.
		a.notifyFallback(batch)
		for _, sub := range batch {
			a.finish(sub, Result{Err: ErrClaimFailed})
		}
		return
	}

	rows := make([]postgres.InsertRow, 0, n)
	ids := make([][]string, len(batch))
	k := 0
	for si, sub := range batch {
		ids[si] = make([]string, len(sub.alerts))
		for i, alert := range sub.alerts {
			id := postgres.FormatID(seqs[k])
			k++
			if alert.UniqueIdentifier == "" {
				a.logger.Warn("source sent no id; using the alert id as unique_identifier, so this alert "+
					"won't merge with repeats or resolve on recovery",
					"request_id", sub.requestID, "source", sub.source, "alt_id", id)
				alert.UniqueIdentifier = id
			}
			body, err := json.Marshal(alert)
			if err != nil {
				// Unreachable for a struct of strings.
				panic(fmt.Sprintf("marshal alert %s: %v", id, err))
			}
			ids[si][i] = id
			rows = append(rows, postgres.InsertRow{ID: id, Source: sub.source, Alert: body, Fingerprint: model.Fingerprint(alert)})
		}
	}

	attempts, err := a.insertWithRetry(rows, deadline)
	if err != nil {
		a.logger.Error("batch NOT stored; senders get 503 and should retry", "alerts", n, "submissions", len(batch),
			"first_id", rows[0].ID, "attempts", attempts, "error", err)
		// Queued before answering so the fallback holds the alerts by the time Submit returns.
		a.notifyFallback(batch)
		for _, sub := range batch {
			a.finish(sub, Result{Err: ErrStoreFailed})
		}
		return
	}
	a.logger.Info("batch written", "alerts", n, "submissions", len(batch), "first_id", rows[0].ID,
		"attempts", attempts, "write_ms", time.Since(start).Milliseconds(),
		"queue_len", len(a.queue), "queue_bytes", a.bytes.Load())
	for si, sub := range batch {
		a.finish(sub, Result{IDs: ids[si]})
	}
	if a.fallback != nil {
		a.fallback.Recovered()
	}
	if a.waker != nil {
		a.waker.Wake()
	}
}

// notifyFallback hands a batch that was not stored to the fallback Chat space, if one is configured.
func (a *Allocator) notifyFallback(batch []*submission) {
	if a.fallback == nil {
		return
	}
	for _, sub := range batch {
		a.fallback.Notify(sub.source, sub.requestID, sub.alerts)
	}
}

// insertWithRetry retries the whole batch with doubling backoff until it lands, attempts run out, the deadline nears, or shutdown stops it.
func (a *Allocator) insertWithRetry(rows []postgres.InsertRow, deadline time.Time) (attempts int, err error) {
	delay := a.cfg.InsertBaseDelay
	for {
		if time.Until(deadline) < a.cfg.QueryTimeout {
			return attempts, errors.Join(err, errors.New("write deadline passed"))
		}
		if a.shuttingDown() {
			return attempts, errors.Join(err, errors.New("shutdown stopped retries"))
		}
		ctx, cancel := context.WithDeadline(context.Background(), deadline)
		err = a.store.InsertBatch(ctx, rows)
		cancel()
		attempts++
		if err == nil {
			return attempts, nil
		}
		if attempts >= a.cfg.InsertAttempts {
			return attempts, err
		}
		a.logger.Warn("batch insert failed, retrying", "alerts", len(rows), "first_id", rows[0].ID, "attempt", attempts, "error", err)
		a.sleep(delay)
		delay *= 2
	}
}
