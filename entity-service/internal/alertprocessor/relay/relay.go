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

// Package relay publishes every stored alert as alert.created: it drains the outbox the consumer writes in the same statement as the alert.
package relay

import (
	"context"
	"log/slog"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertprocessor/event"
)

// Outbox holds the stored alerts not yet published.
type Outbox interface {
	// Drain hands up to limit unpublished events, oldest first, to publish inside one transaction, and marks published the ones publish returns; it returns how many it marked.
	Drain(ctx context.Context, limit int, publish func([]event.Event) []string) (int, error)
}

// Publisher sends one record to the event hub.
type Publisher interface {
	Publish(ctx context.Context, key, value []byte) error
}

// Relay publishes every stored alert as alert.created, at least once.
type Relay struct {
	Outbox    Outbox
	Publisher Publisher
	// Interval is how long the relay waits when the outbox is empty or a drain failed.
	Interval time.Duration
	// Batch is how many events one drain publishes.
	Batch int
}

// Run drains the outbox until ctx ends, going straight on while it is busy.
func (r *Relay) Run(ctx context.Context) {
	for {
		marked, err := r.Outbox.Drain(ctx, r.Batch, func(batch []event.Event) []string { return r.publish(ctx, batch) })
		if err != nil {
			slog.ErrorContext(ctx, "alert relay: drain failed", "retryIn", r.Interval, "err", err)
		}
		if err == nil && marked == r.Batch {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(r.Interval):
		}
	}
}

// publish sends the batch in order and stops at the first failure, so the rest stay queued and nothing is published out of order.
func (r *Relay) publish(ctx context.Context, batch []event.Event) []string {
	published := make([]string, 0, len(batch))
	for _, e := range batch {
		key, value, err := event.Encode(e)
		if err == nil {
			err = r.Publisher.Publish(ctx, key, value)
		}
		if err != nil {
			slog.ErrorContext(ctx, "alert relay: publish failed", "alertId", e.AlertID, "err", err)
			break
		}
		published = append(published, e.AlertID)
	}
	return published
}
