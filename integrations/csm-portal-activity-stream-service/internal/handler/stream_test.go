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
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-portal-activity-stream-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-portal-activity-stream-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-portal-activity-stream-service/internal/stream"
)

const streamTestCaseID = "11111111-1111-1111-1111-111111111111"

var testUser = &middleware.UserInfo{
	Email:  "agent@wso2.com",
	UserID: "uid-123",
	Groups: []string{"csm-agents"},
}

type mockEntityCaseClient struct {
	getCaseFn func(ctx context.Context, caseID string) ([]byte, error)
}

func (m *mockEntityCaseClient) GetCase(ctx context.Context, caseID string) ([]byte, error) {
	if m.getCaseFn != nil {
		return m.getCaseFn(ctx, caseID)
	}
	return []byte(`{"id":"` + caseID + `"}`), nil
}

func TestStreamCaseActivities_Unauthorized(t *testing.T) {
	h := NewStreamHandler(&mockEntityCaseClient{}, stream.NewBroadcastHub())
	req := httptest.NewRequest(http.MethodGet, "/cases/"+streamTestCaseID+"/activities/stream", nil)
	req.SetPathValue("id", streamTestCaseID)
	w := httptest.NewRecorder()

	h.StreamCaseActivities(w, req)

	assertStatus(t, w, http.StatusUnauthorized)
}

func TestStreamCaseActivities_InvalidCaseID(t *testing.T) {
	h := NewStreamHandler(&mockEntityCaseClient{}, stream.NewBroadcastHub())
	req := withUser(httptest.NewRequest(http.MethodGet, "/cases/not-a-uuid/activities/stream", nil))
	req.SetPathValue("id", "not-a-uuid")
	w := httptest.NewRecorder()

	h.StreamCaseActivities(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}

func TestStreamCaseActivities_EmptyCaseID(t *testing.T) {
	h := NewStreamHandler(&mockEntityCaseClient{}, stream.NewBroadcastHub())
	req := withUser(httptest.NewRequest(http.MethodGet, "/cases//activities/stream", nil))
	req.SetPathValue("id", "")
	w := httptest.NewRecorder()

	h.StreamCaseActivities(w, req)

	assertStatus(t, w, http.StatusBadRequest)
}

func TestStreamCaseActivities_HubNotConfigured(t *testing.T) {
	h := NewStreamHandler(&mockEntityCaseClient{}, nil)
	req := withUser(httptest.NewRequest(http.MethodGet, "/cases/"+streamTestCaseID+"/activities/stream", nil))
	req.SetPathValue("id", streamTestCaseID)
	w := httptest.NewRecorder()

	h.StreamCaseActivities(w, req)

	assertStatus(t, w, http.StatusServiceUnavailable)
}

// A caller with a valid token but no read access to the requested case must
// not be able to subscribe to it — see the GetCase authorization check added
// ahead of hub.Register.
func TestStreamCaseActivities_UnauthorizedCase(t *testing.T) {
	client := &mockEntityCaseClient{
		getCaseFn: func(ctx context.Context, caseID string) ([]byte, error) {
			return nil, &apierror.Error{StatusCode: http.StatusForbidden}
		},
	}
	hub := stream.NewBroadcastHub()
	h := NewStreamHandler(client, hub)
	req := withUser(httptest.NewRequest(http.MethodGet, "/cases/"+streamTestCaseID+"/activities/stream", nil))
	req.SetPathValue("id", streamTestCaseID)
	w := httptest.NewRecorder()

	h.StreamCaseActivities(w, req)

	assertStatus(t, w, http.StatusForbidden)
	if ct := w.Header().Get("Content-Type"); ct == "text/event-stream" {
		t.Error("stream headers were written for a case the caller cannot read")
	}
}

// syncRecorder is a minimal, mutex-protected http.ResponseWriter/http.Flusher
// used only by TestStreamCaseActivities_StreamsPublishedEvent below.
// httptest.ResponseRecorder's Body is a plain *bytes.Buffer with no internal
// locking, so a test that reads it from one goroutine while the handler
// under test writes to it from another (unavoidable here — StreamCaseActivities
// blocks for the life of the connection) trips the race detector make test
// runs under.
type syncRecorder struct {
	mu          sync.Mutex
	header      http.Header
	status      int
	wroteHeader bool
	body        bytes.Buffer
}

func newSyncRecorder() *syncRecorder {
	return &syncRecorder{header: http.Header{}, status: http.StatusOK}
}

func (r *syncRecorder) Header() http.Header { return r.header }

func (r *syncRecorder) WriteHeader(status int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.status = status
	r.wroteHeader = true
}

// started reports whether the handler has committed the response headers.
// Tests wait on this (rather than reading Header() concurrently, which races
// with the handler's own Set calls) before acting on an open stream.
func (r *syncRecorder) started() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.wroteHeader
}

func (r *syncRecorder) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.body.Write(p)
}

func (r *syncRecorder) Flush() {}

func (r *syncRecorder) String() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.body.String()
}

func (r *syncRecorder) Status() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.status
}

func TestStreamCaseActivities_StreamsPublishedEvent(t *testing.T) {
	hub := stream.NewBroadcastHub()
	h := NewStreamHandler(&mockEntityCaseClient{}, hub)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = middleware.WithUserInfo(ctx, testUser)
	req := httptest.NewRequest(http.MethodGet, "/cases/"+streamTestCaseID+"/activities/stream", nil).WithContext(ctx)
	req.SetPathValue("id", streamTestCaseID)
	w := newSyncRecorder()

	done := make(chan struct{})
	go func() {
		h.StreamCaseActivities(w, req)
		close(done)
	}()

	// StreamCaseActivities registers with the hub asynchronously relative to
	// this goroutine, so publish on a short interval until it lands (or the
	// deadline below fires) rather than racing a single Publish against an
	// unknown registration time.
	const wantPayload = `{"caseId":"` + streamTestCaseID + `","type":"case.comment_added"}`
	deadline := time.After(2 * time.Second)
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
waitForEvent:
	for {
		hub.Publish(streamTestCaseID, wantPayload)
		if strings.Contains(w.String(), "event: case_updated") {
			break waitForEvent
		}
		select {
		case <-ticker.C:
			continue
		case <-deadline:
			t.Fatal("timed out waiting for a case_updated event in the stream body")
		}
	}

	if w.Status() != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Status(), http.StatusOK)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want %q", ct, "text/event-stream")
	}
	if !strings.Contains(w.String(), wantPayload) {
		t.Errorf("stream body missing published payload: %s", w.String())
	}

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not return after context cancellation")
	}
}

// ---- test helpers ----

func withUser(r *http.Request) *http.Request {
	return r.WithContext(middleware.WithUserInfo(r.Context(), testUser))
}

func assertStatus(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Errorf("status = %d, want %d; body: %s", w.Code, want, w.Body.String())
	}
}

// ---- stream lifetime (token expiry, max lifetime, re-authorization) ----

// startStream runs StreamCaseActivities for user on a cancellable request
// context and returns the recorder, a channel closed when the handler
// returns, and the cancel func. Callers must call cancel.
func startStream(t *testing.T, h *StreamHandler, user *middleware.UserInfo) (*syncRecorder, <-chan struct{}, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	ctx = middleware.WithUserInfo(ctx, user)
	req := httptest.NewRequest(http.MethodGet, "/cases/"+streamTestCaseID+"/activities/stream", nil).WithContext(ctx)
	req.SetPathValue("id", streamTestCaseID)
	w := newSyncRecorder()
	done := make(chan struct{})
	go func() {
		h.StreamCaseActivities(w, req)
		close(done)
	}()
	return w, done, cancel
}

func waitDone(t *testing.T, done <-chan struct{}, within time.Duration) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(within):
		t.Fatal("handler did not return within " + within.String())
	}
}

func assertTerminalEvent(t *testing.T, w *syncRecorder, reason string) {
	t.Helper()
	body := w.String()
	want := "event: " + EventStreamClosed + "\ndata: {\"reason\":\"" + reason + "\"}\n\n"
	if !strings.HasSuffix(body, want) {
		t.Errorf("stream body does not end with terminal event %q; body: %q", want, body)
	}
}

func TestStreamCaseActivities_ClosesAtTokenExpiry(t *testing.T) {
	h := NewStreamHandler(&mockEntityCaseClient{}, stream.NewBroadcastHub())
	user := &middleware.UserInfo{Email: testUser.Email, UserID: testUser.UserID, ExpiresAt: time.Now().Add(150 * time.Millisecond)}
	w, done, cancel := startStream(t, h, user)
	defer cancel()

	waitDone(t, done, 3*time.Second)
	if w.Status() != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Status())
	}
	assertTerminalEvent(t, w, ReasonTokenExpired)
}

func TestStreamCaseActivities_ClosesAtMaxLifetime(t *testing.T) {
	h := NewStreamHandler(&mockEntityCaseClient{}, stream.NewBroadcastHub(), WithMaxLifetime(100*time.Millisecond))
	// No ExpiresAt (local-dev style token): only the lifetime cap applies.
	w, done, cancel := startStream(t, h, testUser)
	defer cancel()

	waitDone(t, done, 3*time.Second)
	assertTerminalEvent(t, w, ReasonMaxLifetime)
}

func TestStreamCaseActivities_TokenExpiryWinsOverLongerLifetime(t *testing.T) {
	h := NewStreamHandler(&mockEntityCaseClient{}, stream.NewBroadcastHub(), WithMaxLifetime(time.Hour))
	user := &middleware.UserInfo{Email: testUser.Email, UserID: testUser.UserID, ExpiresAt: time.Now().Add(100 * time.Millisecond)}
	w, done, cancel := startStream(t, h, user)
	defer cancel()

	waitDone(t, done, 3*time.Second)
	assertTerminalEvent(t, w, ReasonTokenExpired)
}

// countingCaseClient answers the first GetCase (the connect-time check) with
// success and every later one (the periodic re-authorization) with after.
type countingCaseClient struct {
	mu    sync.Mutex
	calls int
	after error
}

func (c *countingCaseClient) GetCase(ctx context.Context, caseID string) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	if c.calls == 1 {
		return []byte(`{}`), nil
	}
	return nil, c.after
}

func (c *countingCaseClient) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}

func TestStreamCaseActivities_ClosesWhenReauthorizationDenied(t *testing.T) {
	client := &countingCaseClient{after: &apierror.Error{StatusCode: http.StatusForbidden}}
	h := NewStreamHandler(client, stream.NewBroadcastHub(), WithReauthInterval(20*time.Millisecond))
	w, done, cancel := startStream(t, h, testUser)
	defer cancel()

	waitDone(t, done, 3*time.Second)
	assertTerminalEvent(t, w, ReasonAccessRevoked)
	if client.count() < 2 {
		t.Errorf("GetCase called %d times, want the connect check plus at least one re-authorization", client.count())
	}
}

func TestStreamCaseActivities_TransientReauthorizationErrorKeepsStreamOpen(t *testing.T) {
	client := &countingCaseClient{after: &apierror.Error{StatusCode: http.StatusServiceUnavailable}}
	hub := stream.NewBroadcastHub()
	h := NewStreamHandler(client, hub, WithReauthInterval(10*time.Millisecond))
	w, done, cancel := startStream(t, h, testUser)
	defer cancel()

	// Let several re-authorization attempts fail transiently...
	deadline := time.Now().Add(2 * time.Second)
	for client.count() < 4 {
		if time.Now().After(deadline) {
			t.Fatal("re-authorization never ran")
		}
		time.Sleep(5 * time.Millisecond)
	}
	select {
	case <-done:
		t.Fatalf("stream closed on a transient upstream error; body: %q", w.String())
	default:
	}

	// ...and prove the stream is still live by delivering an event through it.
	const payload = `{"caseId":"` + streamTestCaseID + `","type":"case.status_changed"}`
	deadline = time.Now().Add(2 * time.Second)
	for !strings.Contains(w.String(), payload) {
		if time.Now().After(deadline) {
			t.Fatalf("event not delivered after transient re-authorization errors; body: %q", w.String())
		}
		hub.Publish(streamTestCaseID, payload)
		time.Sleep(5 * time.Millisecond)
	}
	if strings.Contains(w.String(), "event: "+EventStreamClosed) {
		t.Errorf("unexpected terminal event in body: %q", w.String())
	}
}

func TestStreamCaseActivities_ClientDisconnect_NoTerminalEvent(t *testing.T) {
	h := NewStreamHandler(&mockEntityCaseClient{}, stream.NewBroadcastHub())
	w, done, cancel := startStream(t, h, testUser)

	// Wait for the headers to be written, then simulate the client leaving.
	deadline := time.Now().Add(2 * time.Second)
	for !w.started() {
		if time.Now().After(deadline) {
			t.Fatal("stream never started")
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	waitDone(t, done, 3*time.Second)
	if strings.Contains(w.String(), "event: "+EventStreamClosed) {
		t.Errorf("terminal event written to a client that already disconnected: %q", w.String())
	}
}

// ---- SSE protocol fields (retry, id, Last-Event-ID) and write failures ----

func TestStreamCaseActivities_WritesRetryThenIDs(t *testing.T) {
	hub := stream.NewBroadcastHub()
	h := NewStreamHandler(&mockEntityCaseClient{}, hub)
	w, done, cancel := startStream(t, h, testUser)
	defer func() { cancel(); waitDone(t, done, 3*time.Second) }()

	deadline := time.Now().Add(2 * time.Second)
	for !strings.Contains(w.String(), "event: case_updated") {
		if time.Now().After(deadline) {
			t.Fatalf("no event delivered; body: %q", w.String())
		}
		hub.Publish(streamTestCaseID, `{"caseId":"x"}`)
		time.Sleep(5 * time.Millisecond)
	}
	body := w.String()
	if !strings.HasPrefix(body, "retry: 3000\n\n") {
		t.Errorf("stream does not start with the retry field; body: %q", body)
	}
	if !strings.Contains(body, "\nevent: case_updated\n") || !strings.Contains(body, "id: ") {
		t.Errorf("case_updated event without an id field; body: %q", body)
	}
}

func TestStreamCaseActivities_LastEventIDReplaysMissedEvents(t *testing.T) {
	hub := stream.NewBroadcastHub()
	// Learn a real ID from a first subscriber, then publish two more events
	// the reconnecting client has not seen (plus one for another case).
	first := hub.Register(streamTestCaseID)
	hub.Publish(streamTestCaseID, `{"n":1}`)
	seen := <-first
	hub.Unregister(streamTestCaseID, first)
	hub.Publish(streamTestCaseID, `{"n":2}`)
	hub.Publish("22222222-2222-2222-2222-222222222222", `{"other":true}`)
	hub.Publish(streamTestCaseID, `{"n":3}`)

	h := NewStreamHandler(&mockEntityCaseClient{}, hub)
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/cases/"+streamTestCaseID+"/activities/stream", nil).
		WithContext(middleware.WithUserInfo(ctx, testUser))
	req.SetPathValue("id", streamTestCaseID)
	req.Header.Set("Last-Event-ID", seen.ID)
	w := newSyncRecorder()
	done := make(chan struct{})
	go func() { h.StreamCaseActivities(w, req); close(done) }()
	defer func() { cancel(); waitDone(t, done, 3*time.Second) }()

	deadline := time.Now().Add(2 * time.Second)
	for !strings.Contains(w.String(), `{"n":3}`) {
		if time.Now().After(deadline) {
			t.Fatalf("missed events not replayed; body: %q", w.String())
		}
		time.Sleep(5 * time.Millisecond)
	}
	body := w.String()
	if strings.Contains(body, `{"n":1}`) || strings.Contains(body, `{"other":true}`) {
		t.Errorf("replay included an already-seen or foreign event; body: %q", body)
	}
	if strings.Index(body, `{"n":2}`) > strings.Index(body, `{"n":3}`) {
		t.Errorf("replay out of order; body: %q", body)
	}
}

// failingRecorder is a syncRecorder whose writes start failing once broken
// is set, standing in for a client that has gone away mid-stream.
type failingRecorder struct {
	*syncRecorder
	mu     sync.Mutex
	broken bool
}

func (f *failingRecorder) Write(p []byte) (int, error) {
	f.mu.Lock()
	broken := f.broken
	f.mu.Unlock()
	if broken {
		return 0, io.ErrClosedPipe
	}
	return f.syncRecorder.Write(p)
}

func TestStreamCaseActivities_WriteErrorEndsStreamAndReleasesSlot(t *testing.T) {
	hub := stream.NewBroadcastHub()
	h := NewStreamHandler(&mockEntityCaseClient{}, hub, WithConnectionLimits(1, 0))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "/cases/"+streamTestCaseID+"/activities/stream", nil).
		WithContext(middleware.WithUserInfo(ctx, testUser))
	req.SetPathValue("id", streamTestCaseID)
	w := &failingRecorder{syncRecorder: newSyncRecorder()}
	done := make(chan struct{})
	go func() { h.StreamCaseActivities(w, req); close(done) }()

	deadline := time.Now().Add(2 * time.Second)
	for !w.started() {
		if time.Now().After(deadline) {
			t.Fatal("stream never started")
		}
		time.Sleep(2 * time.Millisecond)
	}
	w.mu.Lock()
	w.broken = true
	w.mu.Unlock()

	// The next event's write fails; the handler must return on its own
	// (the request context is still live) and free its slot.
	deadline = time.Now().Add(2 * time.Second)
	for {
		select {
		case <-done:
			if u, _ := h.limiter.counts(testUser.UserID); u != 0 {
				t.Errorf("connection slot still held after write failure (%d)", u)
			}
			return
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("handler kept running after its writes started failing")
		}
		hub.Publish(streamTestCaseID, `{"caseId":"x"}`)
		time.Sleep(5 * time.Millisecond)
	}
}
