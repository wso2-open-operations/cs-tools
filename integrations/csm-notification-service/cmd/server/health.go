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

package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/eventbus"
)

// consumerRegistry is every consumer the process runs, for /health. The
// HTTP server starts before the consumers do (so the platform's probe gets
// an answer during the partition-count checks), hence the lock: the
// handler can read while startup is still adding.
type consumerRegistry struct {
	mu        sync.RWMutex
	consumers []*eventbus.Consumer
}

// add registers consumers and returns them, so a startConsumers call can
// be wrapped in place.
func (r *consumerRegistry) add(consumers []*eventbus.Consumer) []*eventbus.Consumer {
	r.mu.Lock()
	r.consumers = append(r.consumers, consumers...)
	r.mu.Unlock()
	return consumers
}

// drainAndClose waits, up to wait in total, for every registered
// consumer's Run to return (each drains its in-flight record first — see
// eventbus.Consumer.Run), then closes them all. A consumer still busy when
// wait runs out is closed anyway and logged: shutdown must finish inside
// the platform's termination grace period.
func (r *consumerRegistry) drainAndClose(wait time.Duration) {
	r.mu.RLock()
	consumers := append([]*eventbus.Consumer(nil), r.consumers...)
	r.mu.RUnlock()
	deadline := time.NewTimer(wait)
	defer deadline.Stop()
	for _, c := range consumers {
		select {
		case <-c.Done():
		case <-deadline.C:
			slog.Warn("shutdown: consumer did not finish draining in time; closing it anyway", "consumer", c.Name())
			deadline.Reset(0)
		}
	}
	for _, c := range consumers {
		c.Close()
	}
	slog.Info("shutdown: consumers drained and closed", "count", len(consumers))
}

// statuses snapshots every registered consumer.
func (r *consumerRegistry) statuses() []eventbus.ConsumerStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]eventbus.ConsumerStatus, 0, len(r.consumers))
	for _, c := range r.consumers {
		out = append(out, c.Status())
	}
	return out
}

// healthResponse is GET /health's body.
type healthResponse struct {
	Status    string           `json:"status"`
	Consumers []consumerHealth `json:"consumers"`
}

// consumerHealth is one consumer's status plus the verdict Check gave it.
type consumerHealth struct {
	eventbus.ConsumerStatus
	Healthy bool   `json:"healthy"`
	Reason  string `json:"reason,omitempty"`
}

// healthHandler serves GET /health — the platform's liveness probe. 200
// while every consumer passes eventbus.ConsumerStatus.Check against
// stallAfter, 503 as soon as one does not: a consumer whose Run goroutine
// has exited, or that has made no progress for stallAfter, is a
// notification service that is silently not delivering, and the only
// remedy is a restart, which the probe failing is what triggers. The body
// lists every consumer either way so an operator can see which one.
// Before any consumer is registered (startup) it reports healthy, as the
// previous unconditional 200 did.
func healthHandler(statuses func() []eventbus.ConsumerStatus, stallAfter time.Duration, now func() time.Time) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp := healthResponse{Status: "ok"}
		for _, s := range statuses() {
			ch := consumerHealth{ConsumerStatus: s, Healthy: true}
			if err := s.Check(now(), stallAfter); err != nil {
				ch.Healthy = false
				ch.Reason = err.Error()
				resp.Status = "unhealthy"
			}
			resp.Consumers = append(resp.Consumers, ch)
		}
		if resp.Consumers == nil {
			resp.Consumers = []consumerHealth{}
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		if resp.Status != "ok" {
			w.WriteHeader(http.StatusServiceUnavailable)
			for _, ch := range resp.Consumers {
				if !ch.Healthy {
					slog.WarnContext(r.Context(), "health: consumer unhealthy", "consumer", ch.Name, "reason", ch.Reason)
				}
			}
		}
		_ = json.NewEncoder(w).Encode(resp)
	}
}
