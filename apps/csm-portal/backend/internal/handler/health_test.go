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

package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

type fakePinger struct {
	err error
}

func (f fakePinger) Health(ctx context.Context) error {
	return f.err
}

// countingPinger records how many times Health was actually called, so tests
// can assert on whether GetHealthDependencies's TTL cache/collapsing
// suppressed a would-be upstream call.
type countingPinger struct {
	calls *int32
}

func (p countingPinger) Health(ctx context.Context) error {
	atomic.AddInt32(p.calls, 1)
	return nil
}

func TestHealthHandler_GetHealthDependencies_AllHealthy(t *testing.T) {
	h := NewHealthHandler(fakePinger{}, fakePinger{}, fakePinger{}, fakePinger{}, fakePinger{}).WithAIIntegration(fakePinger{})

	w := httptest.NewRecorder()
	h.GetHealthDependencies(w, httptest.NewRequest(http.MethodGet, "/health/dependencies", nil))

	assertStatus(t, w, http.StatusOK)
	resp := decodeJSON[HealthDependenciesResponse](t, w)
	if resp.Status != "ok" {
		t.Errorf("Status = %q, want ok", resp.Status)
	}
	if len(resp.Dependencies) != 6 {
		t.Fatalf("len(Dependencies) = %d, want 6", len(resp.Dependencies))
	}
	for _, d := range resp.Dependencies {
		if d.Status != "ok" {
			t.Errorf("%s status = %q, want ok", d.Name, d.Status)
		}
	}
}

func TestHealthHandler_GetHealthDependencies_OneDown(t *testing.T) {
	h := NewHealthHandler(fakePinger{}, fakePinger{err: errors.New("boom")}, fakePinger{}, fakePinger{}, fakePinger{})

	w := httptest.NewRecorder()
	h.GetHealthDependencies(w, httptest.NewRequest(http.MethodGet, "/health/dependencies", nil))

	assertStatus(t, w, http.StatusServiceUnavailable)
	resp := decodeJSON[HealthDependenciesResponse](t, w)
	if resp.Status != "degraded" {
		t.Errorf("Status = %q, want degraded", resp.Status)
	}
	var sawUpdatesDown bool
	for _, d := range resp.Dependencies {
		if d.Name == "Updates Service" && d.Status == "down" {
			sawUpdatesDown = true
		}
	}
	if !sawUpdatesDown {
		t.Error("expected Updates Service to be reported down")
	}
}

func TestHealthHandler_GetHealthDependencies_NotConfiguredIsNotDown(t *testing.T) {
	// notification/integration/engineering nil (unconfigured) must never drag
	// the overall status into "degraded" — that's a deployment choice, not a
	// dependency failure.
	h := NewHealthHandler(fakePinger{}, fakePinger{}, nil, nil, nil)

	w := httptest.NewRecorder()
	h.GetHealthDependencies(w, httptest.NewRequest(http.MethodGet, "/health/dependencies", nil))

	assertStatus(t, w, http.StatusOK)
	resp := decodeJSON[HealthDependenciesResponse](t, w)
	if resp.Status != "ok" {
		t.Errorf("Status = %q, want ok", resp.Status)
	}
	for _, d := range resp.Dependencies {
		switch d.Name {
		case "CSM Notification Service", "CSM Integration Service", "Engineering Entity Service", "CSM AI Integration Service":
			if d.Status != "not_configured" {
				t.Errorf("%s status = %q, want not_configured", d.Name, d.Status)
			}
		}
	}
}

func TestHealthHandler_GetHealthDependencies_CachesWithinTTL(t *testing.T) {
	var calls int32
	pinger := countingPinger{calls: &calls}
	h := NewHealthHandler(pinger, pinger, pinger, pinger, pinger)

	fakeNow := time.Now()
	restore := healthNow
	healthNow = func() time.Time { return fakeNow }
	defer func() { healthNow = restore }()

	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		h.GetHealthDependencies(w, httptest.NewRequest(http.MethodGet, "/health/dependencies", nil))
		assertStatus(t, w, http.StatusOK)
	}
	if got := atomic.LoadInt32(&calls); got != 5 {
		t.Errorf("Health calls after 5 requests within the TTL = %d, want 5 (one fan-out of 5 dependencies, cached for the rest)", got)
	}

	// Once the TTL has elapsed, the next request must recompute.
	fakeNow = fakeNow.Add(dependencyHealthCacheTTL)
	w := httptest.NewRecorder()
	h.GetHealthDependencies(w, httptest.NewRequest(http.MethodGet, "/health/dependencies", nil))
	assertStatus(t, w, http.StatusOK)
	if got := atomic.LoadInt32(&calls); got != 10 {
		t.Errorf("Health calls after the TTL elapsed = %d, want 10 (a second fan-out of 5)", got)
	}
}

func TestHealthHandler_GetHealthDependencies_ChecksTheAIIntegrationService(t *testing.T) {
	for _, tc := range []struct {
		name   string
		pinger HealthPinger
		want   string
	}{
		{"up", fakePinger{}, "ok"},
		{"down", fakePinger{err: errors.New("down")}, "down"},
	} {
		h := NewHealthHandler(fakePinger{}, fakePinger{}, nil, nil, nil).WithAIIntegration(tc.pinger)
		w := httptest.NewRecorder()
		h.GetHealthDependencies(w, httptest.NewRequest(http.MethodGet, "/health/dependencies", nil))
		resp := decodeJSON[HealthDependenciesResponse](t, w)
		found := false
		for _, d := range resp.Dependencies {
			if d.Name == "CSM AI Integration Service" {
				found = true
				if d.Status != tc.want {
					t.Errorf("%s: status = %q, want %q", tc.name, d.Status, tc.want)
				}
			}
		}
		if !found {
			t.Errorf("%s: no CSM AI Integration Service entry", tc.name)
		}
	}
}
