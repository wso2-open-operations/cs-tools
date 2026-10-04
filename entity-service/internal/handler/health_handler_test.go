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
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type stubPinger struct {
	err    error
	called bool
	calls  int
}

func (s *stubPinger) Ping(context.Context) error {
	s.called = true
	s.calls++
	return s.err
}

func decodeHealth(t *testing.T, body []byte) map[string]string {
	t.Helper()
	var got map[string]string
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode body %q: %v", body, err)
	}
	return got
}

func TestDatabaseCheck_Up(t *testing.T) {
	pinger := &stubPinger{}
	rec := httptest.NewRecorder()

	NewHealthHandler(pinger).DatabaseCheck(rec, httptest.NewRequest(http.MethodGet, "/health/database", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !pinger.called {
		t.Error("expected the database to actually be pinged")
	}
	got := decodeHealth(t, rec.Body.Bytes())
	if got["status"] != statusOK || got["database"] != dbStatusUp {
		t.Errorf("body = %+v, want status %q with database %q", got, statusOK, dbStatusUp)
	}
}

func TestDatabaseCheck_DownReturns503(t *testing.T) {
	// The whole point of this endpoint over the plain liveness probe: a
	// database outage has to surface as a non-200, or it is useless as an
	// alerting target.
	rec := httptest.NewRecorder()

	NewHealthHandler(&stubPinger{err: errors.New("connection refused")}).
		DatabaseCheck(rec, httptest.NewRequest(http.MethodGet, "/health/database", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	got := decodeHealth(t, rec.Body.Bytes())
	if got["status"] != statusUnavailable || got["database"] != dbStatusDown {
		t.Errorf("body = %+v, want status %q with database %q", got, statusUnavailable, dbStatusDown)
	}
}

func TestDatabaseCheck_FailureLeaksNoDetail(t *testing.T) {
	// This endpoint is publicly reachable, so a failure must not describe
	// the infrastructure behind it. Assert on the driver text specifically:
	// pgx errors routinely carry the host and port.
	rec := httptest.NewRecorder()

	NewHealthHandler(&stubPinger{err: errors.New("dial tcp 10.0.0.5:5432: connection refused")}).
		DatabaseCheck(rec, httptest.NewRequest(http.MethodGet, "/health/database", nil))

	if body := rec.Body.String(); contains(body, "10.0.0.5") || contains(body, "5432") || contains(body, "dial tcp") {
		t.Errorf("failure body leaked connection detail: %s", body)
	}
}

func TestDatabaseCheck_NoPoolIsNotAFailure(t *testing.T) {
	// This probe alerts on a Postgres outage, and a DATA_SOURCE=servicenow
	// deployment has no Postgres to be out — answering 503 there would
	// alert continuously on a database that is not supposed to exist.
	rec := httptest.NewRecorder()

	NewHealthHandler(nil).DatabaseCheck(rec, httptest.NewRequest(http.MethodGet, "/health/database", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	got := decodeHealth(t, rec.Body.Bytes())
	if got["status"] != statusOK || got["database"] != dbStatusNotConfigured {
		t.Errorf("body = %+v, want status %q with database %q", got, statusOK, dbStatusNotConfigured)
	}
}

func TestDatabaseCheck_NotCached(t *testing.T) {
	// A cached 200 would keep reporting healthy straight through an outage.
	rec := httptest.NewRecorder()

	NewHealthHandler(&stubPinger{}).DatabaseCheck(rec, httptest.NewRequest(http.MethodGet, "/health/database", nil))

	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want %q", got, "no-store")
	}
}

func TestHealthCheck_StaysDependencyFree(t *testing.T) {
	// The liveness probe on the main port must keep answering 200 with no
	// dependency call, so a database outage never gets this instance
	// restarted or pulled — that is what the readiness probe is for.
	rec := httptest.NewRecorder()

	HealthCheck(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	got := decodeHealth(t, rec.Body.Bytes())
	if got["status"] != statusOK {
		t.Errorf("status = %q, want %q", got["status"], statusOK)
	}
}

func contains(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) &&
		func() bool {
			for i := 0; i+len(needle) <= len(haystack); i++ {
				if haystack[i:i+len(needle)] == needle {
					return true
				}
			}
			return false
		}()
}

// TestDatabaseCheck_SharesOneRoundTripPerTTL: the probe is public, so a burst
// of polls must not each take a pooled connection. Within dbPingTTL one ping
// answers every request; after it, the next request pings again and sees the
// database's current state.
func TestDatabaseCheck_SharesOneRoundTripPerTTL(t *testing.T) {
	pinger := &stubPinger{}
	h := NewHealthHandler(pinger)
	clock := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	h.now = func() time.Time { return clock }

	probe := func() int {
		rec := httptest.NewRecorder()
		h.DatabaseCheck(rec, httptest.NewRequest(http.MethodGet, "/health/database", nil))
		return rec.Code
	}

	for range 3 {
		if code := probe(); code != http.StatusOK {
			t.Fatalf("status = %d, want 200", code)
		}
	}
	if pinger.calls != 1 {
		t.Fatalf("pinged %d times within the TTL, want 1", pinger.calls)
	}

	pinger.err = errors.New("connection refused")
	clock = clock.Add(dbPingTTL)
	if code := probe(); code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d after the TTL with the database down, want 503", code)
	}
	if pinger.calls != 2 {
		t.Fatalf("pinged %d times, want a fresh ping once the TTL elapsed", pinger.calls)
	}
}
