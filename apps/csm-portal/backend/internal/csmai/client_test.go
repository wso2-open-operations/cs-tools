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

package csmai

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// fakeService serves a token endpoint and the service's routes; handle
// answers the routes and records what came in.
type fakeService struct {
	handle    func(w http.ResponseWriter, r *http.Request, body []byte)
	tokens    atomic.Int32
	lastPath  atomic.Value
	lastAuth  atomic.Value
	lastCorr  atomic.Value
	lastUser  atomic.Value
	lastCType atomic.Value
}

// ServeHTTP answers the token endpoint itself; any other request is recorded
// (path, auth, correlation id, user id, content type) and passed to handle.
func (f *fakeService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/token" {
		f.tokens.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"access_token":"tok-1","token_type":"Bearer","expires_in":3600}`)
		return
	}
	body, _ := io.ReadAll(r.Body)
	f.lastPath.Store(r.Method + " " + r.URL.Path)
	f.lastAuth.Store(r.Header.Get("Authorization"))
	f.lastCorr.Store(r.Header.Get("X-CSM-Correlation-ID"))
	f.lastUser.Store(r.Header.Get("X-CSM-User-ID"))
	f.lastCType.Store(r.Header.Get("Content-Type"))
	f.handle(w, r, body)
}

func newTestClient(t *testing.T, f *fakeService, timeout time.Duration) *Client {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	return NewClient(Config{BaseURL: srv.URL + "/", TokenURL: srv.URL + "/token", ClientID: "id", ClientSecret: "secret", Timeout: timeout})
}

// withCorrelationID runs the real middleware to get a context carrying id.
func withCorrelationID(t *testing.T, id string) context.Context {
	t.Helper()
	var ctx context.Context
	h := middleware.CorrelationID(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { ctx = r.Context() }))
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-CSM-Correlation-ID", id)
	h.ServeHTTP(httptest.NewRecorder(), r)
	return ctx
}

func TestGenerateDashboardSendsTheRequestAndReadsTheAnswer(t *testing.T) {
	var got map[string]any
	f := &fakeService{handle: func(w http.ResponseWriter, _ *http.Request, body []byte) {
		_ = json.Unmarshal(body, &got)
		_, _ = io.WriteString(w, `{"candidates":[{"displayName":"A"},{"displayName":"B"}],"usage":{"inputTokens":10,"outputTokens":2}}`)
	}}
	c := newTestClient(t, f, 5*time.Second)
	ctx := withCorrelationID(t, "corr-123")
	res, err := c.GenerateDashboard(ctx, DashboardRequest{
		Prompt: "Open cases", Draft: json.RawMessage(`{"displayName":"Old"}`),
		Teams: []Team{{Key: "rigel", Name: "Rigel", Family: "cre-abt", Field: "creTeam"}},
	})
	if err != nil {
		t.Fatalf("GenerateDashboard: %v", err)
	}
	if len(res.Candidates) != 2 || string(res.Candidates[1]) != `{"displayName":"B"}` || res.Usage.InputTokens != 10 || res.Declined != "" {
		t.Errorf("result = %+v", res)
	}
	if f.lastPath.Load() != "POST /ai/dashboards/generate" || f.lastAuth.Load() != "Bearer tok-1" || f.lastCType.Load() != "application/json" {
		t.Errorf("request = %v, auth %v, content type %v", f.lastPath.Load(), f.lastAuth.Load(), f.lastCType.Load())
	}
	if corr, _ := f.lastCorr.Load().(string); !strings.Contains(corr, "corr-123") {
		t.Errorf("correlation id = %q, want the caller's", corr)
	}
	teams, _ := got["teams"].([]any)
	if got["prompt"] != "Open cases" || got["draft"].(map[string]any)["displayName"] != "Old" || len(teams) != 1 {
		t.Errorf("body = %v", got)
	}
}

// TestGenerateDashboardSendsTheCallersUserIDNotTheirEmail: the request carries
// the signed-in user's id, never their email, and no header at all when the
// context has no user.
func TestGenerateDashboardSendsTheCallersUserIDNotTheirEmail(t *testing.T) {
	f := &fakeService{handle: func(w http.ResponseWriter, _ *http.Request, _ []byte) {
		_, _ = io.WriteString(w, `{"candidates":[{"displayName":"A"}],"usage":{"inputTokens":1,"outputTokens":1}}`)
	}}
	c := newTestClient(t, f, 5*time.Second)

	ctx := middleware.WithUserInfo(context.Background(), &middleware.UserInfo{Email: "someone@example.com", UserID: "user-42"})
	if _, err := c.GenerateDashboard(ctx, DashboardRequest{Prompt: "Open cases"}); err != nil {
		t.Fatalf("GenerateDashboard: %v", err)
	}
	if got := f.lastUser.Load(); got != "user-42" {
		t.Errorf("user id header = %v, want user-42", got)
	}

	if _, err := c.GenerateDashboard(context.Background(), DashboardRequest{Prompt: "Open cases"}); err != nil {
		t.Fatalf("GenerateDashboard without a user: %v", err)
	}
	if got := f.lastUser.Load(); got != "" {
		t.Errorf("user id header without a user = %v, want none", got)
	}
}

func TestGenerateDashboardSendsAnEmptyTeamListAndNoDraft(t *testing.T) {
	var raw string
	f := &fakeService{handle: func(w http.ResponseWriter, _ *http.Request, body []byte) {
		raw = string(body)
		_, _ = io.WriteString(w, `{"declined":"No revenue field.","usage":{}}`)
	}}
	res, err := newTestClient(t, f, 5*time.Second).GenerateDashboard(context.Background(), DashboardRequest{Prompt: "p"})
	if err != nil || res.Declined != "No revenue field." {
		t.Fatalf("res %+v err %v", res, err)
	}
	if raw != `{"prompt":"p","teams":[]}` {
		t.Errorf("body = %s, want no draft and an empty team list", raw)
	}
}

func TestAnErrorKeepsTheStatusAndCode(t *testing.T) {
	for _, tc := range []struct {
		status int
		body   string
		code   string
	}{
		{http.StatusUnprocessableEntity, `{"code":"refused","message":"the model refused the request"}`, CodeRefused},
		{http.StatusGatewayTimeout, `{"code":"timeout","message":"x"}`, CodeTimeout},
		{http.StatusBadGateway, `<html>gateway error</html>`, ""},
	} {
		f := &fakeService{handle: func(w http.ResponseWriter, _ *http.Request, _ []byte) {
			w.WriteHeader(tc.status)
			_, _ = io.WriteString(w, tc.body)
		}}
		_, err := newTestClient(t, f, 5*time.Second).GenerateDashboard(context.Background(), DashboardRequest{Prompt: "p"})
		var e *Error
		if !errors.As(err, &e) || e.StatusCode != tc.status || e.Code != tc.code {
			t.Errorf("%d: err = %v, want status %d code %q", tc.status, err, tc.status, tc.code)
		}
	}
}

func TestATimeoutOrBadAnswerIsErrCall(t *testing.T) {
	slow := &fakeService{handle: func(w http.ResponseWriter, r *http.Request, _ []byte) {
		select {
		case <-time.After(2 * time.Second):
		case <-r.Context().Done():
		}
	}}
	_, err := newTestClient(t, slow, 200*time.Millisecond).GenerateDashboard(context.Background(), DashboardRequest{Prompt: "p"})
	var netErr interface{ Timeout() bool }
	if !errors.Is(err, ErrCall) || !errors.As(err, &netErr) || !netErr.Timeout() {
		t.Errorf("slow: err = %v, want ErrCall wrapping a timeout", err)
	}
	garbled := &fakeService{handle: func(w http.ResponseWriter, _ *http.Request, _ []byte) { _, _ = io.WriteString(w, "not json") }}
	if _, err := newTestClient(t, garbled, 5*time.Second).GenerateDashboard(context.Background(), DashboardRequest{Prompt: "p"}); !errors.Is(err, ErrCall) {
		t.Errorf("garbled: err = %v, want ErrCall", err)
	}
}

func TestHealth(t *testing.T) {
	status := http.StatusOK
	f := &fakeService{handle: func(w http.ResponseWriter, _ *http.Request, _ []byte) { w.WriteHeader(status) }}
	c := newTestClient(t, f, 5*time.Second)
	if err := c.Health(context.Background()); err != nil || f.lastPath.Load() != "GET /health" {
		t.Errorf("up: err = %v, path %v", err, f.lastPath.Load())
	}
	status = http.StatusServiceUnavailable
	var e *Error
	if err := c.Health(context.Background()); !errors.As(err, &e) || e.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("down: err = %v", err)
	}
}
