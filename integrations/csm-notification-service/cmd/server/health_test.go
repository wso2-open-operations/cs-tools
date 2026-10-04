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
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/eventbus"
)

func serveHealth(t *testing.T, statuses []eventbus.ConsumerStatus, now time.Time) (*httptest.ResponseRecorder, healthResponse) {
	t.Helper()
	h := healthHandler(func() []eventbus.ConsumerStatus { return statuses }, 5*time.Minute, func() time.Time { return now })
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	var body healthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body %q: %v", rec.Body.String(), err)
	}
	return rec, body
}

func TestHealthHandler_AllConsumersHealthy(t *testing.T) {
	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	statuses := []eventbus.ConsumerStatus{
		{Name: "main", State: "running", LastActivity: now.Add(-10 * time.Second)},
		{Name: "dlq", State: "running", LastActivity: now.Add(-4 * time.Minute)},
	}
	rec, body := serveHealth(t, statuses, now)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	if body.Status != "ok" || len(body.Consumers) != 2 {
		t.Errorf("body = %+v, want ok with both consumers listed", body)
	}
	for _, c := range body.Consumers {
		if !c.Healthy || c.Reason != "" {
			t.Errorf("consumer %s reported unhealthy: %+v", c.Name, c)
		}
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
}

func TestHealthHandler_ExitedConsumerIs503(t *testing.T) {
	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	statuses := []eventbus.ConsumerStatus{
		{Name: "main", State: "running", LastActivity: now},
		{Name: "project", State: "exited", ExitReason: "reader closed", LastActivity: now},
	}
	rec, body := serveHealth(t, statuses, now)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if body.Status != "unhealthy" {
		t.Errorf("Status = %q, want unhealthy", body.Status)
	}
	var sick []string
	for _, c := range body.Consumers {
		if !c.Healthy {
			sick = append(sick, c.Name+": "+c.Reason)
		}
	}
	if len(sick) != 1 || sick[0] != "project: consumer project exited (reader closed)" {
		t.Errorf("unhealthy consumers = %v, want exactly the exited one with its reason", sick)
	}
}

func TestHealthHandler_StalledConsumerIs503(t *testing.T) {
	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	statuses := []eventbus.ConsumerStatus{
		{Name: "cr", State: "running", LastActivity: now.Add(-6 * time.Minute), LastError: "reader reported 12 errors during the last poll window"},
	}
	rec, body := serveHealth(t, statuses, now)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if len(body.Consumers) != 1 || body.Consumers[0].Healthy || body.Consumers[0].Reason == "" {
		t.Errorf("body = %+v, want the stalled consumer flagged with a reason", body)
	}
}

func TestHealthHandler_NoConsumersYetIsHealthy(t *testing.T) {
	rec, body := serveHealth(t, nil, time.Now())
	if rec.Code != http.StatusOK || body.Status != "ok" {
		t.Fatalf("status = %d / %q, want 200 ok during startup", rec.Code, body.Status)
	}
	if body.Consumers == nil {
		t.Error("consumers should encode as an empty list, not null")
	}
}

func TestConsumerRegistry_AddReturnsAndLists(t *testing.T) {
	reg := &consumerRegistry{}
	cfg := eventbus.Config{Broker: "localhost:1", ConnectionString: "x", Topic: "t"}
	got := reg.add([]*eventbus.Consumer{eventbus.NewConsumer(cfg, "g", eventbus.WithName("a")), eventbus.NewConsumer(cfg, "g", eventbus.WithName("b"))})
	if len(got) != 2 {
		t.Fatalf("add returned %d consumers, want 2", len(got))
	}
	st := reg.statuses()
	if len(st) != 2 || st[0].Name != "a" || st[1].Name != "b" || st[0].State != "created" {
		t.Errorf("statuses = %+v", st)
	}
	for _, c := range got {
		c.Close()
	}
}

// TestConsumerRegistry_DrainAndCloseWaitsForRun: drainAndClose returns
// once every consumer's Run has returned, and closes them.
func TestConsumerRegistry_DrainAndCloseWaitsForRun(t *testing.T) {
	reg := &consumerRegistry{}
	cfg := eventbus.Config{Broker: "127.0.0.1:1", ConnectionString: "x", Topic: "t"}
	c := eventbus.NewConsumer(cfg, "g", eventbus.WithName("main"), eventbus.WithDrainTimeout(10*time.Millisecond))
	reg.add([]*eventbus.Consumer{c})
	ctx, cancel := context.WithCancel(context.Background())
	go c.Run(ctx, func(context.Context, eventbus.Record) error { return nil }, nil)
	cancel()
	start := time.Now()
	reg.drainAndClose(5 * time.Second)
	select {
	case <-c.Done():
	default:
		t.Fatal("drainAndClose returned before Run did")
	}
	if time.Since(start) > 4*time.Second {
		t.Errorf("drainAndClose took %v, want it to return as soon as Run did", time.Since(start))
	}
}

// TestConsumerRegistry_DrainAndCloseIsBounded: a consumer whose Run never
// started (so never finishes) does not hold shutdown past the wait.
func TestConsumerRegistry_DrainAndCloseIsBounded(t *testing.T) {
	reg := &consumerRegistry{}
	cfg := eventbus.Config{Broker: "127.0.0.1:1", ConnectionString: "x", Topic: "t"}
	reg.add([]*eventbus.Consumer{eventbus.NewConsumer(cfg, "g"), eventbus.NewConsumer(cfg, "g")})
	start := time.Now()
	reg.drainAndClose(50 * time.Millisecond)
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("drainAndClose took %v with a 50ms budget", elapsed)
	}
}
