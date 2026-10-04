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

package eventbus

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	kafka "github.com/segmentio/kafka-go"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/apierror"
)

// HeaderNotBefore is the record header a dead-letter publish stamps with
// the earliest time (RFC 3339) the dead-letter consumer may attempt the
// record. Run honours it by waiting — without committing — until that
// instant, so the second retry tier is genuinely later than the first
// rather than milliseconds after it: a record dead-lettered during an
// upstream outage gets its next chance once the outage has had time to
// pass. It is absolute, not relative, so a backlog of records dead-lettered
// together waits once, not once per record, and a record redelivered after
// a restart does not restart its wait. A missing or unparseable header
// means no wait.
const HeaderNotBefore = "x-retry-not-before"

// notBeforeWaitSlice bounds each sleep while waiting on a record's
// NotBefore (or a retry backoff), so a shutdown is noticed promptly and the
// consumer's activity timestamp (see Status) keeps moving while it waits.
const notBeforeWaitSlice = 10 * time.Second

// defaultPollTimeout bounds one FetchMessage call. A poll that returns
// nothing within it is an ordinary empty poll on a quiet topic — healthy —
// unless the reader reported errors meanwhile; see Run. It is what lets an
// idle consumer prove it is alive without a record arriving.
const defaultPollTimeout = 30 * time.Second

// DefaultDrainTimeout is how long, once shutdown starts, an in-flight
// record's handler, dead-letter/park publish and offset commit may still
// run before their context is cancelled too. See drainContext.
const DefaultDrainTimeout = 20 * time.Second

// Record is the eventbus-agnostic view of a consumed message that Handle
// receives — deliberately not the underlying Kafka client's own message
// type, so dispatch (and any future caller) never needs to import
// github.com/segmentio/kafka-go directly.
type Record struct {
	Topic     string
	Partition int
	Offset    int64
	Key       []byte
	Value     []byte
	// NotBefore is the HeaderNotBefore value, when the record carried one;
	// Run has already waited for it by the time Handle sees the record.
	NotBefore time.Time
	// Attempt is the 1-based number of this Handle call for this record on
	// THIS topic (1 on the first call, RetryPolicy.MaxAttempts on the last).
	Attempt int
	// IsFinalAttempt is true on the last of RetryPolicy.MaxAttempts calls to
	// Handle for this record on THIS topic — set by processRecord, which is
	// about to call onExhausted (or park/drop) and move on regardless of
	// this call's outcome. This does not by itself mean no further attempt
	// will ever come for this event's content: the main consumer's
	// onExhausted republishes the exact same Value to the dead-letter
	// topic, which gets its own fresh IsFinalAttempt cycle. See
	// NoMoreRetries for the property a Handle implementation actually wants
	// when deciding whether it's safe to release content-keyed idempotency
	// state.
	IsFinalAttempt bool
	// NoMoreRetries is true only when there is no further tier of retries
	// coming for this event's content, on any topic. It's set two ways:
	// on IsFinalAttempt when onExhausted is nil (the DLQ topic's own
	// Consumer always has onExhausted=nil — see cmd/server/main.go — so
	// this is true on its final attempt; the main topic's Consumer has
	// onExhausted set, so its own final attempt leaves this false, since
	// one more tier of attempts is coming on the DLQ topic); or on the
	// extra cleanup call processRecord makes when onExhausted itself
	// fails (the dead-letter publish didn't go through, so the DLQ tier
	// that would otherwise be coming never will — this really is the end
	// of the line for this content, not just this topic's own attempts;
	// the record is parked instead, which is not a retry).
	//
	// A Handle that keys its own idempotency tracking off event content
	// rather than Kafka coordinates (see dispatch.recordBaseKey) must gate
	// releasing that tracking on this, not IsFinalAttempt: releasing on the
	// main topic's final attempt would let an already-succeeded channel be
	// reclaimed and resent once the dead-lettered record's own first
	// attempt arrives, since it would compute the exact same content key.
	NoMoreRetries bool
}

// Consumer lifecycle states, as reported by Status.
const (
	stateCreated int32 = iota
	stateRunning
	stateExited
)

// Consumer reads records from a topic as a member of a named consumer group,
// so multiple running instances of this service split the topic's partitions
// between them instead of each seeing every record.
type Consumer struct {
	reader      *kafka.Reader
	name        string
	topic       string
	group       string
	policy      RetryPolicy
	park        ParkFunc
	pollTimeout time.Duration
	drain       time.Duration
	stats       Stats
	// done is closed when Run returns; see Done.
	done chan struct{}

	// Liveness, for Status: state is one of the state* constants;
	// lastActivity (unix nanoseconds) is the last moment Run proved it was
	// making progress — a record fetched, an attempt finished, an offset
	// committed, a backoff slice elapsed, or an empty poll on a quiet topic
	// with no reader errors behind it. A consumer whose lastActivity stops
	// moving is stuck or disconnected even if its goroutine is alive.
	state        atomic.Int32
	lastActivity atomic.Int64
	fetchErrors  atomic.Int64
	mu           sync.Mutex
	lastError    string
	exitReason   string
}

// Stats are a Consumer's lifetime counters, readable while it runs.
type Stats struct {
	// Handled counts records whose Handle call eventually returned nil.
	Handled atomic.Uint64
	// FailedAttempts counts individual Handle calls that returned an error.
	FailedAttempts atomic.Uint64
	// DeadLettered counts records handed to a successful onExhausted.
	DeadLettered atomic.Uint64
	// Parked counts records handed to a successful ParkFunc.
	Parked atomic.Uint64
	// Dropped counts records that exhausted every attempt and could be
	// neither dead-lettered nor parked — the one outcome that loses the
	// record; it is always logged at ERROR as well.
	Dropped atomic.Uint64
}

// ConsumerStatus is a point-in-time snapshot of one Consumer, for health
// reporting (see Check).
type ConsumerStatus struct {
	Name  string `json:"name"`
	Topic string `json:"topic"`
	Group string `json:"group"`
	// State is "created" (Run not yet called), "running" or "exited".
	State      string `json:"state"`
	ExitReason string `json:"exitReason,omitempty"`
	// LastActivity is the last time Run made observable progress — see
	// Consumer.lastActivity; Idle is now minus that, for a human reader.
	LastActivity time.Time `json:"lastActivity"`
	Idle         string    `json:"idle"`
	// LastError is the most recent fetch-side error (not a handler error),
	// cleared by the next successful fetch.
	LastError      string `json:"lastError,omitempty"`
	FetchErrors    int64  `json:"fetchErrors"`
	Handled        uint64 `json:"handled"`
	FailedAttempts uint64 `json:"failedAttempts"`
	DeadLettered   uint64 `json:"deadLettered"`
	Parked         uint64 `json:"parked"`
	Dropped        uint64 `json:"dropped"`
}

// Check reports why the consumer should be considered unhealthy at now, or
// nil when it should not: it has exited (nothing restarts a Run goroutine,
// so the process must be restarted), or — when stallAfter is positive — it
// has shown no activity for longer than stallAfter, whether because Run
// was never started or because every poll since has failed. A quiet topic
// does not trip this: an empty poll with no reader errors counts as
// activity (see Run).
func (s ConsumerStatus) Check(now time.Time, stallAfter time.Duration) error {
	if s.State == "exited" {
		return fmt.Errorf("consumer %s exited (%s)", s.Name, s.ExitReason)
	}
	if stallAfter <= 0 {
		return nil
	}
	idle := now.Sub(s.LastActivity)
	if idle <= stallAfter {
		return nil
	}
	if s.State == "created" {
		return fmt.Errorf("consumer %s never started (created %s ago)", s.Name, idle.Round(time.Second))
	}
	return fmt.Errorf("consumer %s: no successful poll, handle or commit for %s (limit %s); last error: %s", s.Name, idle.Round(time.Second), stallAfter, s.LastError)
}

// Option configures a Consumer at construction.
type Option func(*Consumer)

// WithName labels the consumer in logs and status output (e.g. "main",
// "dlq"); it has no effect on consumption.
func WithName(name string) Option {
	return func(c *Consumer) { c.name = name }
}

// WithRetryPolicy sets how a record is retried before it is handed to
// OnExhausted. The default is DefaultRetryPolicy.
func WithRetryPolicy(p RetryPolicy) Option {
	return func(c *Consumer) { c.policy = p.normalized() }
}

// WithParking sets where a record goes once no retry tier remains for it
// (see ParkFunc). Without it such a record is logged at ERROR and dropped.
func WithParking(park ParkFunc) Option {
	return func(c *Consumer) { c.park = park }
}

// WithDrainTimeout sets how long in-flight work may continue after
// shutdown starts (see DefaultDrainTimeout). Non-positive keeps the
// default.
func WithDrainTimeout(d time.Duration) Option {
	return func(c *Consumer) {
		if d > 0 {
			c.drain = d
		}
	}
}

// NewConsumer constructs a Consumer that joins groupID and consumes
// cfg.Topic. Auto-commit is not used: offsets are committed explicitly by
// Run, only after a record has been handled (or exhausted its retries) —
// never before, so a crash mid-processing redelivers the record on restart
// instead of silently skipping it.
func NewConsumer(cfg Config, groupID string, opts ...Option) *Consumer {
	c := &Consumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers: []string{cfg.Broker},
			GroupID: groupID,
			Topic:   cfg.Topic,
			Dialer: &kafka.Dialer{
				TLS:           &tls.Config{MinVersion: tls.VersionTLS12},
				SASLMechanism: cfg.saslMechanism(),
			},
			// Only applies to a partition with no committed offset yet (e.g.
			// the very first time this consumer group ever runs) — this is
			// kafka-go's own default, set explicitly here for clarity and to
			// document the reason: a notification service should process
			// backlog, not silently start from the tail. The Kafka client
			// used before this one defaulted the other way and needed this
			// set explicitly to avoid dropping events published just before
			// its first join — confirmed against the real namespace.
			StartOffset: kafka.FirstOffset,
			Logger:      kafka.LoggerFunc(logDebug),
			ErrorLogger: kafka.LoggerFunc(logError),
			// kafka-go's consumer-group rebalancing only offers Range and
			// RoundRobin balancers (its default, left unset here) — there is
			// no cooperative/incremental strategy like the Kafka client used
			// before this one had. In practice this only matters once this
			// service scales beyond one instance: a rebalance briefly pauses
			// every partition in the group instead of only the ones actually
			// moving. Not a concern for a single running instance.
		}),
		name:        cfg.Topic,
		topic:       cfg.Topic,
		group:       groupID,
		policy:      DefaultRetryPolicy,
		pollTimeout: defaultPollTimeout,
		drain:       DefaultDrainTimeout,
		done:        make(chan struct{}),
	}
	c.touch()
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Name is the label given by WithName (the topic name when none was).
func (c *Consumer) Name() string { return c.name }

// Stats exposes the consumer's counters.
func (c *Consumer) Stats() *Stats { return &c.stats }

// Status snapshots the consumer for health reporting.
func (c *Consumer) Status() ConsumerStatus {
	c.mu.Lock()
	lastError, exitReason := c.lastError, c.exitReason
	c.mu.Unlock()
	last := time.Unix(0, c.lastActivity.Load())
	state := "created"
	switch c.state.Load() {
	case stateRunning:
		state = "running"
	case stateExited:
		state = "exited"
	}
	return ConsumerStatus{
		Name:           c.name,
		Topic:          c.topic,
		Group:          c.group,
		State:          state,
		ExitReason:     exitReason,
		LastActivity:   last,
		Idle:           time.Since(last).Round(time.Second).String(),
		LastError:      lastError,
		FetchErrors:    c.fetchErrors.Load(),
		Handled:        c.stats.Handled.Load(),
		FailedAttempts: c.stats.FailedAttempts.Load(),
		DeadLettered:   c.stats.DeadLettered.Load(),
		Parked:         c.stats.Parked.Load(),
		Dropped:        c.stats.Dropped.Load(),
	}
}

// touch records that Run just made progress — see Consumer.lastActivity.
func (c *Consumer) touch() {
	c.lastActivity.Store(time.Now().UnixNano())
}

func (c *Consumer) setLastError(msg string) {
	c.mu.Lock()
	c.lastError = msg
	c.mu.Unlock()
}

func (c *Consumer) exit(reason string) {
	c.mu.Lock()
	c.exitReason = reason
	c.mu.Unlock()
	c.state.Store(stateExited)
}

// Handle processes a single record. A non-nil error causes Run to retry (see
// RetryPolicy).
type Handle func(context.Context, Record) error

// OnExhausted is called once a record's Handle call has failed on every one
// of the policy's attempts. Run commits the record's offset right after
// this call returns regardless of its outcome — either way nothing will
// attempt this record again on this topic, so there's nothing left to gate
// the commit on.
//
// Passing nil (as the DLQ consumer's own Run call does — see cmd/server/
// main.go) marks this consumer as the last retry tier: an exhausted record
// is parked (see ParkFunc/WithParking) or, with no parking configured,
// logged at ERROR and dropped. The main consumer instead passes a func that
// publishes the record to the dead-letter topic, so a persistently-failing
// record (e.g. a downstream outage) gets a second, later tier of attempts
// before it is given up on.
//
// A non-nil return means the dead-letter publish failed. Run then treats
// this consumer as the last tier for that record after all — it makes one
// cleanup Handle call with NoMoreRetries set (see Record.NoMoreRetries) and
// parks the record — and still commits, since a failure here has no lower
// tier to fall back to either.
type OnExhausted func(ctx context.Context, record Record, handleErr error) error

// Done is closed once Run has returned — after any in-flight record has
// drained (see Run). A Consumer whose Run was never started never closes
// it.
func (c *Consumer) Done() <-chan struct{} { return c.done }

// drainContext returns the context in-flight work runs under: it ignores
// stop's cancellation (so a shutdown signal does not abort an HTTP send
// half-way, which would leave a notification delivered but its offset
// uncommitted and redelivered after restart), but is cancelled grace after
// stop is, so draining is bounded. Not a per-record timeout: in normal
// operation a handler keeps the full time its own HTTP timeouts allow.
func drainContext(stop context.Context, grace time.Duration) (context.Context, context.CancelFunc) {
	work, cancel := context.WithCancel(context.WithoutCancel(stop))
	go func() {
		select {
		case <-stop.Done():
			timer := time.NewTimer(grace)
			defer timer.Stop()
			select {
			case <-timer.C:
				cancel()
			case <-work.Done():
			}
		case <-work.Done():
		}
	}()
	return work, cancel
}

// Run polls for records and calls handle for each one, committing its offset
// once handle succeeds or its retries are exhausted. Run blocks until ctx is
// canceled or the Consumer is closed; call it from its own goroutine. Once
// it returns the consumer is "exited" (see Status) and nothing restarts
// it: a health check that reports that is how the process gets restarted.
//
// Shutdown is a drain, not an abort: cancelling ctx stops fetching and any
// retry backoff or not-before wait, but a handler call already in flight
// runs to completion (bounded by the drain timeout — see drainContext),
// and so do the dead-letter/park publish and the offset commit that follow
// it. Only then does Run return and Done close; the caller should wait on
// Done before Close.
func (c *Consumer) Run(ctx context.Context, handle Handle, onExhausted OnExhausted) {
	defer close(c.done)
	work, cancelWork := drainContext(ctx, c.drain)
	defer cancelWork()
	c.state.Store(stateRunning)
	c.touch()
	for {
		// Bounded per poll so an idle consumer still proves it is alive:
		// a timeout with no reader errors behind it is a healthy empty
		// poll. The reader's own fetch loop is independent of this
		// context, so cancelling it drops nothing — a message that arrives
		// as the deadline fires stays queued for the next call.
		pollCtx, cancelPoll := context.WithTimeout(ctx, c.pollTimeout)
		msg, err := c.reader.FetchMessage(pollCtx)
		cancelPoll()
		if err != nil {
			switch {
			case ctx.Err() != nil:
				c.exit("stopped")
				return
			case errors.Is(err, io.EOF):
				c.exit("reader closed")
				return
			case errors.Is(err, context.DeadlineExceeded):
				// Stats() returns the counters accumulated since its last
				// call, so this is exactly "errors during this poll window".
				if st := c.reader.Stats(); st.Errors > 0 {
					c.fetchErrors.Add(st.Errors)
					c.setLastError(fmt.Sprintf("reader reported %d errors during the last poll window", st.Errors))
					slog.WarnContext(ctx, "eventbus: poll window ended with reader errors", "consumer", c.name, "errors", st.Errors)
				} else {
					c.touch()
				}
				continue
			default:
				c.fetchErrors.Add(1)
				c.setLastError(err.Error())
				slog.ErrorContext(ctx, "eventbus: fetch error", "consumer", c.name, "err", err)
				continue
			}
		}
		// A fetched record is progress whatever the reader counted on the
		// way to it; drain the window's counters so they don't count
		// against the next empty poll.
		_ = c.reader.Stats()
		c.setLastError("")
		c.touch()
		record := Record{
			Topic:     msg.Topic,
			Partition: msg.Partition,
			Offset:    msg.Offset,
			Key:       msg.Key,
			Value:     msg.Value,
			NotBefore: notBeforeOf(msg.Headers),
		}
		if !waitUntil(ctx, record.NotBefore, c.touch) {
			// Shutdown while waiting out the record's NotBefore — skip the
			// commit; it is redelivered after restart and waits out
			// whatever remains of the same absolute instant.
			continue
		}
		if !c.processRecordUntil(ctx, work, record, handle, onExhausted) {
			// ctx was canceled mid-retry-wait (shutdown) — skip the commit,
			// same as the fetch loop above; the next FetchMessage call will
			// see ctx.Err() != nil and return.
			continue
		}
		// The commit runs on the drain context: a record that finished
		// just as shutdown began must not lose its commit to the signal,
		// or it is redelivered and its notifications sent again.
		if cerr := c.reader.CommitMessages(work, msg); cerr != nil {
			slog.ErrorContext(ctx, "eventbus: commit failed", "consumer", c.name, "topic", record.Topic, "partition", record.Partition, "offset", record.Offset, "err", cerr)
		} else {
			c.touch()
		}
	}
}

// notBeforeOf extracts HeaderNotBefore from a record's headers; zero when
// absent or malformed (logged — a malformed stamp is a bug in the
// publisher, not a reason to hold the record).
func notBeforeOf(headers []kafka.Header) time.Time {
	for _, h := range headers {
		if h.Key != HeaderNotBefore {
			continue
		}
		ts, err := time.Parse(time.RFC3339Nano, string(h.Value))
		if err != nil {
			slog.Warn("eventbus: ignoring malformed not-before header on record", "header", HeaderNotBefore, "err", err)
			return time.Time{}
		}
		return ts
	}
	return time.Time{}
}

// NotBeforeHeader formats t as the HeaderNotBefore value a publisher stamps.
func NotBeforeHeader(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

// waitUntil blocks until t (immediately when t is zero or past), returning
// false if ctx is canceled first. Sleeps in notBeforeWaitSlice pieces,
// calling tick (when non-nil) after each, so a shutdown is noticed
// promptly and the caller's activity timestamp keeps moving however far
// off t is.
func waitUntil(ctx context.Context, t time.Time, tick func()) bool {
	for {
		remaining := time.Until(t)
		if remaining <= 0 {
			return true
		}
		if remaining > notBeforeWaitSlice {
			remaining = notBeforeWaitSlice
		}
		timer := time.NewTimer(remaining)
		select {
		case <-ctx.Done():
			timer.Stop()
			return false
		case <-timer.C:
			if tick != nil {
				tick()
			}
		}
	}
}

// processRecord calls handle up to policy.MaxAttempts times (pausing
// policy.Delay between attempts), then, if every attempt failed, escalates:
// to onExhausted when set (the dead-letter tier), otherwise — or if
// onExhausted itself fails — to park when set, and failing that it logs
// and drops. Factored out of Run so the retry/escalation logic is testable
// without a real Kafka broker — it never touches c.reader itself.
//
// Returns whether Run should commit the record's offset afterward — false
// only when ctx was canceled mid-retry-wait (a shutdown in progress), so a
// record that was never actually finished being handled isn't marked done.
func (c *Consumer) processRecord(ctx context.Context, record Record, handle Handle, onExhausted OnExhausted) bool {
	return c.processRecordUntil(ctx, ctx, record, handle, onExhausted)
}

// processRecordUntil is processRecord with the two contexts Run uses kept
// apart: stop ends waiting (backoff between attempts), ctx is what handle,
// onExhausted and the park run under — the drain context, which outlives
// stop by the drain timeout. So a shutdown lets the attempt in flight
// finish and be committed, but never starts another one.
func (c *Consumer) processRecordUntil(stop, ctx context.Context, record Record, handle Handle, onExhausted OnExhausted) bool {
	attempts := c.policy.MaxAttempts
	var err error
	for attempt := 1; attempt <= attempts; attempt++ {
		record.Attempt = attempt
		record.IsFinalAttempt = attempt == attempts
		record.NoMoreRetries = record.IsFinalAttempt && onExhausted == nil
		err = handle(ctx, record)
		c.touch()
		if err == nil {
			c.stats.Handled.Add(1)
			return true
		}
		c.stats.FailedAttempts.Add(1)
		slog.ErrorContext(ctx, "eventbus: handler failed",
			"consumer", c.name, "topic", record.Topic, "partition", record.Partition, "offset", record.Offset,
			"attempt", attempt, "maxAttempts", attempts, "err", apierror.Summary(err))
		if attempt < attempts {
			delay := c.policy.Delay(attempt)
			slog.InfoContext(ctx, "eventbus: retrying record after backoff",
				"consumer", c.name, "topic", record.Topic, "partition", record.Partition, "offset", record.Offset,
				"nextAttempt", attempt+1, "delay", delay)
			if !waitUntil(stop, time.Now().Add(delay), c.touch) {
				return false
			}
		}
	}
	if onExhausted != nil {
		dlqErr := onExhausted(ctx, record, err)
		if dlqErr == nil {
			c.stats.DeadLettered.Add(1)
			return true
		}
		slog.ErrorContext(ctx, "eventbus: dead-letter publish also failed; parking the record instead",
			"consumer", c.name, "topic", record.Topic, "partition", record.Partition, "offset", record.Offset, "handleErr", apierror.Summary(err), "onExhaustedErr", apierror.Summary(dlqErr))
		// The DLQ publish itself failed, so — unlike the ordinary
		// exhaustion case above NoMoreRetries deliberately stays false
		// for — there is truly no future delivery of this content
		// coming on any topic; parking is not a retry. Call handle once
		// more with NoMoreRetries now true, purely so a Handle that
		// tracks content-keyed idempotency state (see dispatch.Dispatcher)
		// gets a chance to release it — without this, it would never
		// learn this record is done and would leak that state forever.
		// Any channel already claimed is a no-op here (claim() rejects a
		// key already held); this call's own error changes nothing about
		// the commit decision below (the record is being parked or
		// dropped either way) but is still logged.
		record.NoMoreRetries = true
		if cleanupErr := handle(ctx, record); cleanupErr != nil {
			slog.ErrorContext(ctx, "eventbus: final cleanup handle call after dead-letter failure returned an error (ignored — record is being parked)",
				"consumer", c.name, "topic", record.Topic, "partition", record.Partition, "offset", record.Offset, "err", apierror.Summary(cleanupErr))
		}
	}
	c.parkOrDrop(ctx, record, err)
	return true
}

// parkOrDrop is the end of the line for a record: parked when a ParkFunc
// is configured and succeeds, otherwise dropped — in both cases logged at
// ERROR with the record's coordinates and counted, so giving up on a
// record is never silent.
func (c *Consumer) parkOrDrop(ctx context.Context, record Record, handleErr error) {
	attrs := []any{"consumer", c.name, "topic", record.Topic, "partition", record.Partition, "offset", record.Offset, "attempts", record.Attempt}
	if c.park != nil {
		parkErr := c.park(ctx, record, handleErr)
		if parkErr == nil {
			c.stats.Parked.Add(1)
			slog.ErrorContext(ctx, "eventbus: record exhausted every retry tier and was parked; manual replay required", attrs...)
			return
		}
		attrs = append(attrs, "parkErr", apierror.Summary(parkErr))
	}
	c.stats.Dropped.Add(1)
	slog.ErrorContext(ctx, "eventbus: record exhausted every retry tier and could not be parked; dropping it (unrecoverable outside the topic's retention window)",
		append(attrs, "handleErr", apierror.Summary(handleErr))...)
}

// Close leaves the consumer group and closes the underlying connection.
func (c *Consumer) Close() {
	_ = c.reader.Close()
}
