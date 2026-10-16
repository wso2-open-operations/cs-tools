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
	"time"

	kafka "github.com/segmentio/kafka-go"
)

// Record is the eventbus-agnostic view of a consumed message that Handle
// receives — deliberately not the underlying Kafka client's own message
// type, so callers never need to import github.com/segmentio/kafka-go
// directly.
type Record struct {
	Topic     string
	Partition int
	Offset    int64
	Key       []byte
	Value     []byte
	// Time is the record's timestamp on the topic (zero if the broker
	// supplied none).
	Time time.Time
}

// Handle processes a single record. Unlike csm-notification-service's own
// eventbus.Handle, a non-nil return here does not trigger a retry — see
// Consumer.Run's doc comment for why.
type Handle func(context.Context, Record) error

// messageReader is the subset of *kafka.Reader that Consumer uses — an
// interface so tests can drive Run's exit and commit paths without a broker.
type messageReader interface {
	FetchMessage(ctx context.Context) (kafka.Message, error)
	CommitMessages(ctx context.Context, msgs ...kafka.Message) error
	Close() error
}

// Consumer reads records from a topic as a member of a named consumer
// group, so multiple running instances of a caller split the topic's
// partitions between them instead of each seeing every record. A distinct
// groupID gets its own full copy of every record on the topic, independent
// of every other group — that's the mechanism for adding a second,
// independent reaction to the same events (see internal/caseevents.Handler,
// this package's first caller) without touching the producer or anything
// already consuming the topic.
type Consumer struct {
	reader messageReader
}

// StartOffset controls where a brand new (never-before-committed) consumer
// group begins reading a partition. Has no effect on a group with existing
// committed offsets — those always resume from where they left off,
// regardless of this setting. A named type rather than passing
// github.com/segmentio/kafka-go's own constant directly, matching Record's
// own doc comment: callers of this package never need to import kafka-go
// themselves.
type StartOffset int

const (
	// EarliestOffset replays every retained record for a new consumer
	// group — appropriate for at-least-once delivery guarantees, matching
	// csm-notification-service's own Consumer.
	EarliestOffset StartOffset = iota
	// LatestOffset skips retained history entirely for a new consumer
	// group, only seeing records published from now on — appropriate for
	// a best-effort, live-only consumer where replaying old records would
	// be actively wrong, e.g. internal/caseevents.Handler re-broadcasting
	// stale case_updated notifications to connected SSE clients after
	// every restart or routine redeploy (a new consumer group every time,
	// since its groupID is suffixed per-process — see newReplicaID in
	// cmd/server/main.go).
	LatestOffset
)

func (s StartOffset) kafkaOffset() int64 {
	if s == LatestOffset {
		return kafka.LastOffset
	}
	return kafka.FirstOffset
}

// commitInterval batches offset commits: Run still marks each record
// committed only after handling it, but the reader flushes those marks to
// the broker on this interval (and on Close) instead of one synchronous
// round-trip per record, which would otherwise delay the fan-out of the
// next record. A crash can therefore redeliver up to one interval's worth of
// records — harmless for this live-only, idempotent cache-invalidation ping.
const commitInterval = time.Second

// NewConsumer constructs a Consumer that joins groupID and consumes
// cfg.Topic. Offsets are marked for commit by Run only after a record has
// been handled — never before — and flushed in batches (see commitInterval),
// so a crash mid-processing redelivers the record on restart instead of
// silently skipping it.
func NewConsumer(cfg Config, groupID string, startOffset StartOffset) *Consumer {
	return &Consumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers: []string{cfg.Broker},
			GroupID: groupID,
			Topic:   cfg.Topic,
			Dialer: &kafka.Dialer{
				TLS:           &tls.Config{MinVersion: tls.VersionTLS12},
				SASLMechanism: cfg.saslMechanism(),
			},
			// Only applies to a partition with no committed offset yet (this
			// consumer group's first run) — see StartOffset.
			StartOffset:    startOffset.kafkaOffset(),
			CommitInterval: commitInterval,
			Logger:         kafka.LoggerFunc(logDebug),
			ErrorLogger:    kafka.LoggerFunc(logError),
		}),
	}
}

// ErrReaderClosed is wrapped by the error Run returns when the underlying
// reader reports io.EOF while ctx is still live — the reader is closed and
// will never yield another record, so the consumer must be replaced (see
// Supervisor).
var ErrReaderClosed = errors.New("eventbus: reader closed")

// Run polls for records and calls handle for each one, committing its
// offset once handle returns — regardless of outcome. Run blocks until ctx
// is canceled (then it returns nil) or the reader stops for good (then it
// returns a non-nil error wrapping ErrReaderClosed, already logged at Error);
// call it from its own goroutine, normally via Supervisor, which restarts a
// consumer that exits unexpectedly. Run does not close the reader.
//
// Deliberately simpler than csm-notification-service's own Consumer: no
// retry-then-dead-letter policy, since handle's only implementation so far
// (internal/caseevents.Handler) just logs and can't meaningfully fail in a
// way a retry would fix. A handle error is logged here and the record is
// committed anyway — revisit this (retries, a dead-letter topic) once a
// handle exists whose failure modes are actually worth retrying.
func (c *Consumer) Run(ctx context.Context, handle Handle) error {
	// lastFetchErr de-duplicates consecutive identical fetch errors: kafka-go's
	// Reader already retries internally with its own bounded backoff before
	// FetchMessage returns an error here, but a sustained outage would still
	// produce one log line per retry without this — logging the same error
	// over and over adds nothing once the first line has told the story.
	// Reset on success so a *new* failure (after a recovery) still logs.
	var lastFetchErr string
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			if errors.Is(err, io.EOF) {
				exitErr := fmt.Errorf("%w: %w", ErrReaderClosed, err)
				slog.ErrorContext(ctx, "eventbus: consumer stopped unexpectedly", "err", exitErr)
				return exitErr
			}
			if errMsg := err.Error(); errMsg != lastFetchErr {
				slog.ErrorContext(ctx, "eventbus: fetch error", "err", err)
				lastFetchErr = errMsg
			}
			continue
		}
		lastFetchErr = ""

		record := Record{
			Topic:     msg.Topic,
			Partition: msg.Partition,
			Offset:    msg.Offset,
			Key:       msg.Key,
			Value:     msg.Value,
			Time:      msg.Time,
		}
		if err := handle(ctx, record); err != nil {
			slog.ErrorContext(ctx, "eventbus: handler failed", "topic", record.Topic, "partition", record.Partition, "offset", record.Offset, "err", err)
		}

		if cerr := c.reader.CommitMessages(ctx, msg); cerr != nil {
			slog.ErrorContext(ctx, "eventbus: commit failed", "topic", record.Topic, "partition", record.Partition, "offset", record.Offset, "err", cerr)
		}
	}
}

// Close leaves the consumer group and closes the underlying connection.
func (c *Consumer) Close() {
	_ = c.reader.Close()
}
