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
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/customer-portal-activity-stream-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/integrations/customer-portal-activity-stream-service/internal/stream"
)

func TestJitteredLifetime_WithinTenPercent(t *testing.T) {
	base := 4 * time.Minute
	lo := time.Duration(float64(base) * 0.9)
	hi := time.Duration(float64(base) * 1.1)
	for _, u := range []float64{0, 0.0001, 0.25, 0.5, 0.75, 0.9999} {
		got := jitteredLifetime(base, u)
		if got < lo || got > hi {
			t.Errorf("u=%v: lifetime %v outside [%v, %v]", u, got, lo, hi)
		}
	}
	if got := jitteredLifetime(base, 0); got != lo {
		t.Errorf("u=0: got %v, want %v", got, lo)
	}
	if got := jitteredLifetime(base, 0.5); got != base {
		t.Errorf("u=0.5: got %v, want %v", got, base)
	}
}

func TestJitteredLifetime_ZeroOrNegativeIsUnbounded(t *testing.T) {
	for _, base := range []time.Duration{0, -time.Second} {
		if got := jitteredLifetime(base, 0.7); got != 0 {
			t.Errorf("base=%v: got %v, want 0", base, got)
		}
	}
}

func lifetimeRequest(ctx context.Context) *http.Request {
	ctx = middleware.WithUserInfo(ctx, testUser)
	req := httptest.NewRequest(http.MethodGet, "/cases/"+streamTestCaseID+"/activities/stream", nil).WithContext(ctx)
	req.SetPathValue("id", streamTestCaseID)
	return req
}

func TestStreamCaseActivities_EndsAfterMaxLifetime(t *testing.T) {
	h := NewStreamHandler(&mockEntityCaseClient{}, stream.NewBroadcastHub(), WithMaxLifetime(100*time.Millisecond))
	w := newSyncRecorder()

	done := make(chan struct{})
	go func() {
		h.StreamCaseActivities(w, lifetimeRequest(context.Background()))
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not return after max lifetime")
	}
	if !strings.HasSuffix(w.String(), "event: reconnect\ndata: {}\n\n") {
		t.Errorf("body should end with the reconnect event, got %q", w.String())
	}
}

func TestStreamCaseActivities_ZeroLifetimeIsUnbounded(t *testing.T) {
	h := NewStreamHandler(&mockEntityCaseClient{}, stream.NewBroadcastHub(), WithMaxLifetime(0))
	w := newSyncRecorder()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		h.StreamCaseActivities(w, lifetimeRequest(ctx))
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("handler returned on its own with lifetime 0")
	case <-time.After(300 * time.Millisecond):
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not return after context cancellation")
	}
	if strings.Contains(w.String(), "reconnect") {
		t.Errorf("unexpected reconnect event: %q", w.String())
	}
}

// Runs the handler behind a real http.Server carrying the production stream
// listener's timeout fields and asserts the client sees the reconnect event
// followed by a clean end of body (no unexpected EOF).
func TestStreamCaseActivities_CleanEndThroughRealServer(t *testing.T) {
	h := NewStreamHandler(&mockEntityCaseClient{}, stream.NewBroadcastHub(), WithMaxLifetime(150*time.Millisecond))
	mux := http.NewServeMux()
	mux.HandleFunc("GET /cases/{id}/activities/stream", h.StreamCaseActivities)

	srv := httptest.NewUnstartedServer(nil)
	srv.Config = &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mux.ServeHTTP(w, r.WithContext(middleware.WithUserInfo(r.Context(), testUser)))
		}),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      0,
		IdleTimeout:       0,
	}
	srv.Start()
	defer srv.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(srv.URL + "/cases/" + streamTestCaseID + "/activities/stream")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("body did not end cleanly: %v (body so far %q)", err, body)
	}
	if string(body) != "event: reconnect\ndata: {}\n\n" {
		t.Errorf("body = %q, want only the reconnect event", body)
	}
}
