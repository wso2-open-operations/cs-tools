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
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type stubHealth struct {
	calls atomic.Int32
	err   atomic.Value // error or nil-wrapper
}

type errBox struct{ err error }

func (s *stubHealth) set(err error) { s.err.Store(errBox{err}) }

func (s *stubHealth) Health(context.Context) error {
	s.calls.Add(1)
	if v, ok := s.err.Load().(errBox); ok {
		return v.err
	}
	return nil
}

func serveHealth(t *testing.T, h *HealthHandler) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))
	return w
}

func TestHealth_ReportsReachability(t *testing.T) {
	stub := &stubHealth{}
	h := NewHealthHandler(stub)
	clock := time.Unix(1_000_000, 0)
	h.now = func() time.Time { return clock }

	w := serveHealth(t, h)
	assertStatus(t, w, http.StatusOK)
	assertContentType(t, w, "application/json")
	if got := decodeJSON[map[string]string](t, w)["status"]; got != "ok" {
		t.Errorf("status = %q, want ok", got)
	}

	// Inside the TTL the cached result is served without probing again,
	// even though the upstream has gone down.
	stub.set(errors.New("connection refused"))
	clock = clock.Add(healthCacheTTL - time.Second)
	assertStatus(t, serveHealth(t, h), http.StatusOK)
	if n := stub.calls.Load(); n != 1 {
		t.Fatalf("probes inside TTL = %d, want 1", n)
	}

	// Past the TTL the outage shows.
	clock = clock.Add(2 * time.Second)
	w = serveHealth(t, h)
	assertStatus(t, w, http.StatusServiceUnavailable)
	if got := decodeJSON[map[string]string](t, w)["status"]; got != "unavailable" {
		t.Errorf("status = %q, want unavailable", got)
	}

	// And recovery shows after the next TTL.
	stub.set(nil)
	clock = clock.Add(healthCacheTTL)
	assertStatus(t, serveHealth(t, h), http.StatusOK)
	if n := stub.calls.Load(); n != 3 {
		t.Errorf("total probes = %d, want 3", n)
	}
}

func TestHealth_FirstProbeFailureIs503(t *testing.T) {
	stub := &stubHealth{}
	stub.set(errors.New("no route to host"))
	assertStatus(t, serveHealth(t, NewHealthHandler(stub)), http.StatusServiceUnavailable)
}

func TestHealth_ConcurrentPollsShareOneProbe(t *testing.T) {
	stub := &stubHealth{}
	h := NewHealthHandler(stub)
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))
			if w.Code != http.StatusOK {
				t.Errorf("status = %d, want 200", w.Code)
			}
		}()
	}
	wg.Wait()
	if n := stub.calls.Load(); n != 1 {
		t.Errorf("probes = %d, want 1", n)
	}
}
