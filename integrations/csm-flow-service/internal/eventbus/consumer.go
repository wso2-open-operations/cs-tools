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
	"io"
	"log/slog"
	"time"

	kafka "github.com/segmentio/kafka-go"
)

// handleAttempts is how many times a single record's Handle func is called in
// total before giving up — a record that still fails is handed to OnExhausted
// (typically dead-lettered) and its offset committed anyway, so one
// permanently-failing record cannot block its partition forever.
const handleAttempts = 3

// handleRetryDelay is the fixed pause between attempts — deliberately simple
// (no backoff), covering transient blips, not sustained outages.
const handleRetryDelay = 2 * time.Second

// Record is the eventbus-agnostic view of a consumed message that Handle
// receives, so no other package needs to import kafka-go directly.
type Record struct {
	Topic     string
	Partition int
	Offset    int64
	Key       []byte
	Value     []byte
	// IsFinalAttempt is true on the last of handleAttempts calls to Handle for
	// this record on THIS topic.
	IsFinalAttempt bool
	// NoMoreRetries is true only when there is no further tier of retries
	// coming for this event's content, on any topic — the property a Handle
	// that keys idempotency off event content should gate cleanup on.
	NoMoreRetries bool
}

// Consumer reads records from a topic as a member of a named consumer group,
// so multiple running instances split the topic's partitions between them.
type Consumer struct {
	reader *kafka.Reader
}

// NewConsumer constructs a Consumer that joins groupID and consumes cfg.Topic.
// Offsets are committed explicitly by Run, only after a record has been
// handled (or exhausted its retries) — never before, so a crash mid-processing
// redelivers the record on restart instead of silently skipping it.
func NewConsumer(cfg Config, groupID string) *Consumer {
	return &Consumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers: []string{cfg.Broker},
			GroupID: groupID,
			Topic:   cfg.Topic,
			Dialer: &kafka.Dialer{
				TLS:           &tls.Config{MinVersion: tls.VersionTLS12},
				SASLMechanism: cfg.saslMechanism(),
			},
			// Process backlog, not just the tail, the first time a group runs.
			StartOffset: kafka.FirstOffset,
			Logger:      kafka.LoggerFunc(logDebug),
			ErrorLogger: kafka.LoggerFunc(logError),
		}),
	}
}

// Handle processes a single record. A non-nil error causes Run to retry (see
// handleAttempts).
type Handle func(context.Context, Record) error

// OnExhausted is called once a record's Handle call has failed on every
// attempt. Run commits the record's offset right after this returns regardless
// of outcome. Passing nil falls back to logging the failure and dropping the
// record.
type OnExhausted func(ctx context.Context, record Record, handleErr error) error

// Run polls for records and calls handle for each, committing its offset once
// handle succeeds or its retries are exhausted. Blocks until ctx is canceled
// or the Consumer is closed; call it from its own goroutine.
func (c *Consumer) Run(ctx context.Context, handle Handle, onExhausted OnExhausted) {
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, io.EOF) {
				return
			}
			slog.ErrorContext(ctx, "eventbus: fetch error", "err", err)
			continue
		}
		record := Record{
			Topic:     msg.Topic,
			Partition: msg.Partition,
			Offset:    msg.Offset,
			Key:       msg.Key,
			Value:     msg.Value,
		}
		if !processRecord(ctx, record, handle, onExhausted, handleAttempts, handleRetryDelay) {
			continue
		}
		if cerr := c.reader.CommitMessages(ctx, msg); cerr != nil {
			slog.ErrorContext(ctx, "eventbus: commit failed", "topic", record.Topic, "partition", record.Partition, "offset", record.Offset, "err", cerr)
		}
	}
}

// processRecord calls handle up to attempts times (pausing retryDelay between
// attempts), then, if every attempt failed, calls onExhausted (or logs and
// drops if nil). Returns whether Run should commit the record's offset — false
// only when ctx was canceled mid-retry-wait.
func processRecord(ctx context.Context, record Record, handle Handle, onExhausted OnExhausted, attempts int, retryDelay time.Duration) bool {
	var err error
	for attempt := 1; attempt <= attempts; attempt++ {
		record.IsFinalAttempt = attempt == attempts
		record.NoMoreRetries = record.IsFinalAttempt && onExhausted == nil
		if err = handle(ctx, record); err == nil {
			return true
		}
		slog.ErrorContext(ctx, "eventbus: handler failed",
			"topic", record.Topic, "partition", record.Partition, "offset", record.Offset,
			"attempt", attempt, "maxAttempts", attempts, "err", err)
		if attempt < attempts {
			select {
			case <-ctx.Done():
				return false
			case <-time.After(retryDelay):
			}
		}
	}
	if onExhausted != nil {
		if dlqErr := onExhausted(ctx, record, err); dlqErr != nil {
			slog.ErrorContext(ctx, "eventbus: dead-letter publish also failed; record is now unrecoverable outside Event Hub's retention window",
				"topic", record.Topic, "partition", record.Partition, "offset", record.Offset, "handleErr", err, "onExhaustedErr", dlqErr)
			record.NoMoreRetries = true
			if cleanupErr := handle(ctx, record); cleanupErr != nil {
				slog.ErrorContext(ctx, "eventbus: final cleanup handle call after dead-letter failure returned an error (ignored — record is being dropped)",
					"topic", record.Topic, "partition", record.Partition, "offset", record.Offset, "err", cleanupErr)
			}
		}
		return true
	}
	slog.ErrorContext(ctx, "eventbus: handler exhausted retries, dropping record",
		"topic", record.Topic, "partition", record.Partition, "offset", record.Offset, "err", err)
	return true
}

// Close leaves the consumer group and closes the underlying connection.
func (c *Consumer) Close() {
	_ = c.reader.Close()
}
