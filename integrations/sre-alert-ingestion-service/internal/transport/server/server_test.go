// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"sre-alert-ingestion-service/internal/transport/auth"
)

type fakePipeline struct {
	got    Request
	result Result
}

func (f *fakePipeline) Ingest(_ context.Context, req Request) Result {
	f.got = req
	return f.result
}

func newTestServer(p Pipeline) *Server {
	return New(Options{
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		Auth:         auth.None{},
		Pipeline:     p,
		Sources:      []string{"aws", "prometheus"},
		MaxBodyBytes: 16,
	})
}

func do(t *testing.T, s *Server, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("body %q is not JSON: %v", rec.Body.String(), err)
	}
	return m
}

func TestLivez(t *testing.T) {
	s := newTestServer(nil)
	s.StartDraining()
	if rec := do(t, s, "GET", "/livez", ""); rec.Code != http.StatusOK {
		t.Errorf("livez = %d, want 200 even while draining", rec.Code)
	}
}

func TestHealthz(t *testing.T) {
	s := newTestServer(nil)
	if rec := do(t, s, "GET", "/healthz", ""); rec.Code != http.StatusOK {
		t.Errorf("healthz = %d, want 200", rec.Code)
	}
	s.StartDraining()
	if rec := do(t, s, "GET", "/healthz", ""); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("healthz while draining = %d, want 503", rec.Code)
	}
}

func TestSourceRoute_UnknownSource404(t *testing.T) {
	rec := do(t, newTestServer(&fakePipeline{}), "POST", SourceRoutePrefix+"splunk", "{}")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if m := decode(t, rec); m["status"] != "rejected" || m["error"] != "unknown source" {
		t.Errorf("body = %v", m)
	}
}

func TestSourceRoute_NotPost405(t *testing.T) {
	rec := do(t, newTestServer(&fakePipeline{}), "GET", SourceRoutePrefix+"aws", "")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
	if rec.Header().Get("Allow") != "POST" {
		t.Errorf("Allow = %q, want POST", rec.Header().Get("Allow"))
	}
}

func TestSourceRoute_TooLarge413NeverReachesPipeline(t *testing.T) {
	p := &fakePipeline{result: Result{Status: http.StatusCreated}}
	rec := do(t, newTestServer(p), "POST", SourceRoutePrefix+"aws", strings.Repeat("x", 17))
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", rec.Code)
	}
	if m := decode(t, rec); m["error"] != "payload too large" {
		t.Errorf("body = %v", m)
	}
	if p.got.Source != "" {
		t.Error("pipeline must not be called for an oversized body")
	}
}

func TestSourceRoute_201PassesRequestThrough(t *testing.T) {
	p := &fakePipeline{result: Result{Status: http.StatusCreated, AltIDs: []string{"ALT000000001", "ALT000000002"}}}
	s := newTestServer(p)
	req := httptest.NewRequest("POST", SourceRoutePrefix+"prometheus", strings.NewReader(`{"a":1}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(RequestIDHeader, "req-123")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	if got, want := strings.TrimSpace(rec.Body.String()), `{"status":"OK","alt_ids":["ALT000000001","ALT000000002"]}`; got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
	if p.got.Source != "prometheus" || string(p.got.Body) != `{"a":1}` || p.got.RequestID != "req-123" ||
		p.got.ContentType != "application/json" || p.got.Route != SourceRoutePrefix+"prometheus" {
		t.Errorf("pipeline got %+v", p.got)
	}
	if rec.Header().Get(RequestIDHeader) != "req-123" {
		t.Errorf("request id header = %q, want the incoming one echoed", rec.Header().Get(RequestIDHeader))
	}
}

func TestSourceRoute_400(t *testing.T) {
	p := &fakePipeline{result: Result{Status: http.StatusBadRequest, Error: "INVALID PAYLOAD"}}
	rec := do(t, newTestServer(p), "POST", SourceRoutePrefix+"aws", "x")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if m := decode(t, rec); m["status"] != "rejected" || m["error"] != "INVALID PAYLOAD" {
		t.Errorf("body = %v", m)
	}
}

func TestSourceRoute_503HasRetryAfter(t *testing.T) {
	p := &fakePipeline{result: Result{Status: http.StatusServiceUnavailable, Error: "queue full"}}
	rec := do(t, newTestServer(p), "POST", SourceRoutePrefix+"aws", "{}")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if rec.Header().Get("Retry-After") != "60" {
		t.Errorf("Retry-After = %q, want 60", rec.Header().Get("Retry-After"))
	}
	if m := decode(t, rec); m["status"] != "unavailable" || m["error"] != "queue full" {
		t.Errorf("body = %v", m)
	}
}

func TestSourceRoute_NilPipeline503(t *testing.T) {
	if rec := do(t, newTestServer(nil), "POST", SourceRoutePrefix+"aws", "{}"); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
}

func TestRequestID_GeneratedWhenAbsent(t *testing.T) {
	rec := do(t, newTestServer(nil), "GET", "/livez", "")
	if id := rec.Header().Get(RequestIDHeader); len(id) != 32 {
		t.Errorf("generated request id = %q, want 32 hex chars", id)
	}
}

type denyAll struct{}

func (denyAll) Authenticate(*http.Request, string) error { return auth.ErrUnauthorized }

type allowAll struct{}

func (allowAll) Authenticate(*http.Request, string) error { return nil }

// countingReader reports how much of the body was actually consumed.
type countingReader struct {
	data []byte
	read int
}

func (c *countingReader) Read(p []byte) (int, error) {
	if c.read >= len(c.data) {
		return 0, io.EOF
	}
	n := copy(p, c.data[c.read:])
	c.read += n
	return n, nil
}

// A rejected webhook must not read its body, so an unauthenticated caller cannot make
// a replica allocate max_body_bytes per request.
func TestSourceRoute_AuthRejectionNeverReadsBody(t *testing.T) {
	s := New(Options{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Auth: denyAll{},
		Pipeline: &fakePipeline{}, Sources: []string{"aws"}, MaxBodyBytes: 1 << 20,
	})
	body := &countingReader{data: []byte(strings.Repeat("x", 4096))}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest("POST", SourceRoutePrefix+"aws", body))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if body.read != 0 {
		t.Errorf("read %d body bytes on a 401; auth must run before the body is read", body.read)
	}
}

// A 401 must carry a Basic challenge. GCP Cloud Monitoring only sends its webhook
// credentials after a 401 with this header, and AWS SNS sends the first request of a
// subscription confirmation without credentials; both retry once challenged.
func TestSourceRoute_UnauthorizedSendsBasicChallenge(t *testing.T) {
	s := New(Options{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Auth: denyAll{},
		Pipeline: &fakePipeline{}, Sources: []string{"aws"}, MaxBodyBytes: 1024,
	})
	rec := do(t, s, "POST", SourceRoutePrefix+"aws", "{}")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	challenge := rec.Header().Get("WWW-Authenticate")
	if !strings.HasPrefix(challenge, "Basic ") || !strings.Contains(challenge, "realm=") {
		t.Errorf("WWW-Authenticate = %q, want a Basic challenge with a realm", challenge)
	}
}

// An accepted webhook must not be challenged.
func TestSourceRoute_AuthorizedHasNoChallenge(t *testing.T) {
	s := New(Options{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Auth: allowAll{},
		Pipeline: &fakePipeline{result: Result{Status: http.StatusCreated}},
		Sources:  []string{"aws"}, MaxBodyBytes: 1024,
	})
	rec := do(t, s, "POST", SourceRoutePrefix+"aws", "{}")
	if got := rec.Header().Get("WWW-Authenticate"); got != "" {
		t.Errorf("WWW-Authenticate = %q on a %d, want none", got, rec.Code)
	}
}

func TestSourceRoute_AuthHookRunsBeforePipeline(t *testing.T) {
	p := &fakePipeline{result: Result{Status: http.StatusCreated}}
	s := New(Options{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Auth: denyAll{}, Pipeline: p,
		Sources: []string{"aws"}, MaxBodyBytes: 1024,
	})
	rec := do(t, s, "POST", SourceRoutePrefix+"aws", "{}")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
	if p.got.Source != "" {
		t.Error("pipeline must not run when auth rejects")
	}
}

func TestAccessLog_CarriesSourceAndAltIDs(t *testing.T) {
	var buf strings.Builder
	p := &fakePipeline{result: Result{Status: http.StatusCreated, AltIDs: []string{"ALT000000007"}}}
	s := New(Options{
		Logger: slog.New(slog.NewJSONHandler(&buf, nil)), Auth: auth.None{}, Pipeline: p,
		Sources: []string{"aws"}, MaxBodyBytes: 1024,
	})
	do(t, s, "POST", SourceRoutePrefix+"aws", "{}")
	do(t, s, "GET", "/healthz", "")

	out := buf.String()
	for _, want := range []string{`"msg":"request"`, `"source":"aws"`, `"alt_ids":["ALT000000007"]`, `"status":201`, `"duration_ms"`, `"request_id"`} {
		if !strings.Contains(out, want) {
			t.Errorf("log missing %s:\n%s", want, out)
		}
	}
	if strings.Contains(out, "/healthz") {
		t.Error("health probes should not be access-logged")
	}
}

// TestStoredBody_Shapes: one alert answers alt_id, several answer alt_ids, and none answers status alone.
func TestStoredBody_Shapes(t *testing.T) {
	cases := map[string]struct {
		ids  []string
		want string
	}{
		"none": {nil, `{"status":"OK"}`},
		"one":  {[]string{"ALT000008816"}, `{"status":"OK","alt_id":"ALT000008816"}`},
		"many": {[]string{"ALT000008816", "ALT000008817"}, `{"status":"OK","alt_ids":["ALT000008816","ALT000008817"]}`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := json.Marshal(storedBody(tc.ids))
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Errorf("body = %s, want %s", got, tc.want)
			}
		})
	}
}

// TestPayloadLog: the raw body is logged before the transform, nested when it is JSON, truncated past the cap, and not at all when the cap is 0 or auth fails.
func TestPayloadLog(t *testing.T) {
	run := func(limit int64, authn auth.Authenticator, body string) string {
		var buf strings.Builder
		s := New(Options{
			Logger: slog.New(slog.NewJSONHandler(&buf, nil)), Auth: authn,
			Pipeline: &fakePipeline{result: Result{Status: http.StatusCreated, AltIDs: []string{"ALT000000001"}}},
			Sources:  []string{"datadog"}, MaxBodyBytes: 1024, PayloadLogBytes: limit,
		})
		do(t, s, "POST", SourceRoutePrefix+"datadog", body)
		for _, line := range strings.Split(buf.String(), "\n") {
			if strings.Contains(line, `"msg":"webhook received"`) {
				return line
			}
		}
		return ""
	}

	if line := run(1024, auth.None{}, "{\n  \"alert_id\": \"148502937\"\n}"); !strings.Contains(line, `"payload":{"alert_id":"148502937"}`) ||
		!strings.Contains(line, `"source":"datadog"`) || !strings.Contains(line, `"body_size":`) {
		t.Errorf("json body not logged nested: %s", line)
	}
	if line := run(1024, auth.None{}, "not json"); !strings.Contains(line, `"payload":"not json"`) {
		t.Errorf("non-json body not logged as text: %s", line)
	}
	if line := run(8, auth.None{}, `{"alert_id":"148502937"}`); !strings.Contains(line, `"payload":"{\"alert_"`) || !strings.Contains(line, `"payload_truncated":true`) {
		t.Errorf("oversized body not truncated: %s", line)
	}
	if line := run(0, auth.None{}, `{"a":1}`); line != "" {
		t.Errorf("payload logged with the cap at 0: %s", line)
	}
	if line := run(1024, denyAll{}, `{"a":1}`); line != "" {
		t.Errorf("payload logged for an unauthenticated request: %s", line)
	}
}

func TestTruncateBytes_KeepsCharactersWhole(t *testing.T) {
	if got := truncateBytes([]byte("ab\u00e9cd"), 3); got != "ab" {
		t.Errorf("truncateBytes = %q, want %q", got, "ab")
	}
}

type fakeDB struct{ err error }

func (f fakeDB) Check(context.Context) error { return f.err }

func TestDbz(t *testing.T) {
	for name, tc := range map[string]struct {
		err  error
		want int
	}{
		"reachable":   {nil, http.StatusOK},
		"unreachable": {errors.New("connection refused"), http.StatusServiceUnavailable},
	} {
		t.Run(name, func(t *testing.T) {
			s := New(Options{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Auth: auth.None{}, DB: fakeDB{tc.err}})
			s.StartDraining()
			if rec := do(t, s, "GET", "/dbz", ""); rec.Code != tc.want {
				t.Errorf("dbz = %d, want %d regardless of draining", rec.Code, tc.want)
			}
		})
	}
}

func TestDbz_NotRegisteredWithoutDB(t *testing.T) {
	if rec := do(t, newTestServer(nil), "GET", "/dbz", ""); rec.Code != http.StatusNotFound {
		t.Errorf("dbz = %d, want 404 when no DB is configured", rec.Code)
	}
}
