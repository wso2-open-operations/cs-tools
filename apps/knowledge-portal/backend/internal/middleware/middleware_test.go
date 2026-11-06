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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/knowledge-portal/backend/internal/entity"
)

func TestRecoveryReturns500Envelope(t *testing.T) {
	h := Logger(Recovery(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom with internal detail")
	})))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "internal detail") || !strings.Contains(rec.Body.String(), `"message"`) {
		t.Errorf("body = %s", rec.Body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type = %q", ct)
	}
}

func TestRecoveryRepanicsAbortHandler(t *testing.T) {
	defer func() {
		if r := recover(); r != http.ErrAbortHandler {
			t.Fatalf("recovered %v, want http.ErrAbortHandler", r)
		}
	}()
	Recovery(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic(http.ErrAbortHandler)
	})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/x", nil))
}

func TestCorrelationIDIsAlwaysServerMinted(t *testing.T) {
	tests := []struct {
		name       string
		inbound    string
		wantClient string // sanitised client-supplied value, "" when none
	}{
		{"absent", "", ""},
		{"well-formed is not trusted", "req-123_abc.4", "req-123_abc.4"},
		{"uuid is not trusted", "11111111-1111-4111-8111-111111111111", "11111111-1111-4111-8111-111111111111"},
		{"too long is capped", strings.Repeat("a", 200), strings.Repeat("a", maxClientCorrelationIDLen)},
		{"space sanitised", "a b", "a_b"},
		{"crlf sanitised", "a\r\nX-Evil: 1", "a__X-Evil__1"},
		{"slash sanitised", "a/b", "a_b"},
		{"multibyte sanitised", "caf\u00e9", "caf__"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ctxID, upstreamID, clientID string
			h := CorrelationID(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				ctxID = CorrelationIDFromContext(r.Context())
				clientID = ClientCorrelationIDFromContext(r.Context())
				upstreamID = probeUpstream(t, r)
			}))
			req := httptest.NewRequest(http.MethodGet, "/x", nil)
			if tt.inbound != "" {
				req.Header.Set(correlationIDHeader, tt.inbound)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			got := rec.Header().Get(correlationIDHeader)
			if got == "" || got != ctxID {
				t.Fatalf("response header %q, context %q", got, ctxID)
			}
			if got == tt.inbound {
				t.Errorf("caller-chosen id %q was echoed back", tt.inbound)
			}
			if upstreamID != got {
				t.Errorf("upstream saw %q, want the server-minted %q", upstreamID, got)
			}
			if tt.inbound != "" && upstreamID == tt.inbound {
				t.Errorf("caller-chosen id %q was forwarded upstream", tt.inbound)
			}
			if !uuidV4.MatchString(got) {
				t.Errorf("minted id %q is not a UUID v4", got)
			}
			if clientID != tt.wantClient {
				t.Errorf("client-supplied id = %q, want %q", clientID, tt.wantClient)
			}
		})
	}
}

var uuidV4 = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// probeUpstream makes one real entity-client call under r's context against a
// stub entity-service and returns the X-CSM-Correlation-ID that stub received.
func probeUpstream(t *testing.T, r *http.Request) string {
	t.Helper()
	var got string
	idp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"t","token_type":"Bearer","expires_in":3600}`))
	}))
	defer idp.Close()
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, ur *http.Request) {
		got = ur.Header.Get(correlationIDHeader)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"knowledgeBases":[]}`))
	}))
	defer up.Close()
	c := entity.NewClient(entity.Config{BaseURL: up.URL, TokenURL: idp.URL, ClientID: "id", ClientSecret: "s"})
	if _, err := c.ListKnowledgeBases(r.Context()); err != nil {
		t.Fatalf("probe call: %v", err)
	}
	return got
}

// TestClientCorrelationIDIsLoggedSeparately: the log record carries the minted
// id as correlationID and the inbound value only as clientCorrelationID.
func TestClientCorrelationIDIsLoggedSeparately(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(ctxHandler{inner: slog.NewTextHandler(&buf, nil)})
	var minted string
	h := CorrelationID(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		minted = CorrelationIDFromContext(r.Context())
		logger.InfoContext(r.Context(), "probe")
	}))
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set(correlationIDHeader, "caller-chosen-1")
	h.ServeHTTP(httptest.NewRecorder(), req)

	out := buf.String()
	if !strings.Contains(out, "correlationID="+minted) {
		t.Errorf("log lacks the minted id: %s", out)
	}
	if !strings.Contains(out, "clientCorrelationID=caller-chosen-1") {
		t.Errorf("log lacks the client-supplied field: %s", out)
	}
	if strings.Contains(out, "correlationID=caller-chosen-1") {
		t.Errorf("client value logged as the request's own id: %s", out)
	}
}

func TestSecurityHeaders(t *testing.T) {
	want := map[string]string{
		"X-Content-Type-Options":    "nosniff",
		"Content-Security-Policy":   "upgrade-insecure-requests",
		"Strict-Transport-Security": "max-age=31536000; includeSubDomains",
	}
	// On success and on error responses alike.
	for _, h := range map[string]http.Handler{
		"200": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }),
		"404": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) }),
	} {
		rec := httptest.NewRecorder()
		SecurityHeaders(h).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
		for k, v := range want {
			if got := rec.Header().Get(k); got != v {
				t.Errorf("%s = %q, want %q", k, got, v)
			}
		}
		// Exactly the documented set: anything added or dropped must be a
		// deliberate test change. CORS in particular belongs to the gateway.
		for k := range rec.Header() {
			if _, ok := want[k]; !ok {
				t.Errorf("unexpected header %q set by the middleware", k)
			}
		}
	}
}
