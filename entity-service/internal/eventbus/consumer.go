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

// The consumer half of this package, added for the deferred Salesforce
// membership writes (service.ProjectMembershipWriteService): entity-service
// publishes salesforce.membership_write.requested to itself and consumes it
// back on its own consumer group. Modeled on csm-notification-service's
// internal/eventbus/consumer.go -- same Record/Handle/OnExhausted shape, same
// commit-after-handle discipline -- with one deliberate difference: the retry
// policy is per Consumer and can back off exponentially, instead of a fixed
// package constant. The main topic keeps a short policy so one unreachable
// downstream cannot stall the partition for long; the dead-letter topic's
// Consumer gets a long one, because waiting IS its job (see RetryPolicy).

// RetryPolicy says how many times one record's Handle is called in total
// (Attempts; 3 means 3 calls, 2 of them retries) and how long to wait between
// calls: Delay before the second attempt, doubling each time up to MaxDelay.
// A zero MaxDelay means a fixed Delay, which is csm-notification-service's
// policy.
type RetryPolicy struct {
	Attempts int
	Delay    time.Duration
	MaxDelay time.Duration
}

// DefaultRetryPolicy is csm-notification-service's fixed policy: 3 attempts,
// 2 seconds apart, covering a transient blip and nothing longer.
var DefaultRetryPolicy = RetryPolicy{Attempts: 3, Delay: 2 * time.Second}

// normalized fills in anything a caller left at zero so Run always has a
// usable policy.
func (p RetryPolicy) normalized() RetryPolicy {
	if p.Attempts < 1 {
		p.Attempts = DefaultRetryPolicy.Attempts
	}
	if p.Delay <= 0 {
		p.Delay = DefaultRetryPolicy.Delay
	}
	if p.MaxDelay < 0 {
		p.MaxDelay = 0
	}
	return p
}

// wait is the pause before attempt number next (2 or more): Delay doubled
// once per completed attempt, capped at MaxDelay when one is set.
func (p RetryPolicy) wait(next int) time.Duration {
	d := p.Delay
	if p.MaxDelay <= 0 {
		return d
	}
	for i := 2; i < next; i++ {
		d *= 2
		if p.MaxDelay > 0 && d >= p.MaxDelay {
			return p.MaxDelay
		}
	}
	if p.MaxDelay > 0 && d > p.MaxDelay {
		return p.MaxDelay
	}
	return d
}

// TotalWait is how long a record can be retried in place before this policy
// gives up on it: the sum of every pause. Logged at startup so an operator
// can see how long an outage one tier covers.
func (p RetryPolicy) TotalWait() time.Duration {
	p = p.normalized()
	var total time.Duration
	for n := 2; n <= p.Attempts; n++ {
		total += p.wait(n)
	}
	return total
}

// Record is the eventbus-agnostic view of a consumed message.
type Record struct {
	Topic     string
	Partition int
	Offset    int64
	Key       []byte
	Value     []byte
	// IsFinalAttempt is true on the policy's last call for this record on
	// this topic. With an OnExhausted set (the main topic's Consumer), one
	// more tier follows on the dead-letter topic, so NoMoreRetries stays
	// false there.
	IsFinalAttempt bool
	// NoMoreRetries is true when nothing will attempt this content again
	// anywhere: the final attempt on a Consumer with no OnExhausted (the
	// dead-letter topic's own), or a dead-letter publish that itself failed.
	// A handler uses it to leave a durable record of the give-up.
	NoMoreRetries bool
}

// Handle processes a single record. A non-nil error makes Run retry per the
// policy, then hand the record to OnExhausted.
type Handle func(ctx context.Context, record Record) error

// OnExhausted is called once every attempt failed -- typically to publish the
// record to a dead-letter topic. Its error is logged and does not change
// whether the offset is committed: it always is, so one permanently failing
// record cannot block its partition forever.
type OnExhausted func(ctx context.Context, record Record, handleErr error) error

// Consumer reads one topic as a member of a consumer group and commits
// offsets explicitly, after the record has been handled (or given up on).
type Consumer struct {
	reader *kafka.Reader
	policy RetryPolicy
}

// NewConsumer joins groupID on cfg.Topic. StartOffset FirstOffset only applies
// to a partition with no committed offset yet: a brand-new group reads the
// backlog rather than skipping it.
func NewConsumer(cfg Config, groupID string, policy RetryPolicy) *Consumer {
	return &Consumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers: []string{cfg.Broker},
			GroupID: groupID,
			Topic:   cfg.Topic,
			Dialer: &kafka.Dialer{
				TLS:           &tls.Config{MinVersion: tls.VersionTLS12},
				SASLMechanism: cfg.saslMechanism(),
			},
			StartOffset: kafka.FirstOffset,
			// Without this a group that joins while its topic has no
			// partitions yet (the hub created after the service started, or
			// auto-created by the first publish) is assigned nothing and
			// stays that way until the next rebalance, which may never
			// come: the member looks healthy and consumes nothing. Watching
			// re-triggers the rebalance when the partition set changes.
			WatchPartitionChanges: true,
			Logger:                kafka.LoggerFunc(logDebug),
			ErrorLogger:           kafka.LoggerFunc(logError),
		}),
		policy: policy.normalized(),
	}
}

// Policy is the retry policy this consumer runs with.
func (c *Consumer) Policy() RetryPolicy { return c.policy }

// Run polls until ctx is cancelled, handling each record per the policy and
// committing its offset afterwards. A fetch error backs off briefly instead
// of spinning: a broker that is down would otherwise be hammered in a hot
// loop.
func (c *Consumer) Run(ctx context.Context, handle Handle, onExhausted OnExhausted) {
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, io.EOF) {
				return
			}
			slog.ErrorContext(ctx, "eventbus: fetch error", "err", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
			continue
		}
		record := Record{
			Topic:     msg.Topic,
			Partition: msg.Partition,
			Offset:    msg.Offset,
			Key:       msg.Key,
			Value:     msg.Value,
		}
		if !processRecord(ctx, record, handle, onExhausted, c.policy) {
			// ctx was cancelled mid-wait (shutdown): no commit, so the
			// record is redelivered to whoever takes the partition next.
			continue
		}
		if cerr := c.reader.CommitMessages(ctx, msg); cerr != nil {
			slog.ErrorContext(ctx, "eventbus: commit failed", "topic", record.Topic, "partition", record.Partition, "offset", record.Offset, "err", cerr)
		}
	}
}

// processRecord calls handle up to policy.Attempts times with the policy's
// pauses between, then, if every attempt failed, calls onExhausted (or logs
// the drop). Returns whether Run should commit -- false only when ctx was
// cancelled mid-wait. Split from Run so the retry logic is testable without
// a broker.
func processRecord(ctx context.Context, record Record, handle Handle, onExhausted OnExhausted, policy RetryPolicy) bool {
	var err error
	for attempt := 1; attempt <= policy.Attempts; attempt++ {
		record.IsFinalAttempt = attempt == policy.Attempts
		record.NoMoreRetries = record.IsFinalAttempt && onExhausted == nil
		if err = handle(ctx, record); err == nil {
			return true
		}
		slog.ErrorContext(ctx, "eventbus: handler failed",
			"topic", record.Topic, "partition", record.Partition, "offset", record.Offset,
			"attempt", attempt, "maxAttempts", policy.Attempts, "err", err)
		if attempt < policy.Attempts {
			wait := policy.wait(attempt + 1)
			slog.InfoContext(ctx, "eventbus: retrying record", "topic", record.Topic, "offset", record.Offset, "nextAttempt", attempt+1, "in", wait.String())
			select {
			case <-ctx.Done():
				return false
			case <-time.After(wait):
			}
		}
	}
	if onExhausted != nil {
		if dlqErr := onExhausted(ctx, record, err); dlqErr != nil {
			slog.ErrorContext(ctx, "eventbus: dead-letter publish also failed; record is now unrecoverable outside Event Hub's retention window",
				"topic", record.Topic, "partition", record.Partition, "offset", record.Offset, "handleErr", err, "onExhaustedErr", dlqErr)
			// Nothing will try this content again: give the handler one
			// last call with NoMoreRetries so it can record the give-up.
			record.NoMoreRetries = true
			if cleanupErr := handle(ctx, record); cleanupErr != nil {
				slog.ErrorContext(ctx, "eventbus: final handle call after dead-letter failure returned an error (ignored; record is being dropped)",
					"topic", record.Topic, "partition", record.Partition, "offset", record.Offset, "err", cleanupErr)
			}
		}
		return true
	}
	slog.ErrorContext(ctx, "eventbus: handler exhausted retries, dropping record",
		"topic", record.Topic, "partition", record.Partition, "offset", record.Offset, "err", err)
	return true
}

// Close releases the reader; a Run in progress returns on its next fetch.
func (c *Consumer) Close() {
	_ = c.reader.Close()
}
