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

package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/binara-sachin/git-internals-dashboard/backend/internal/apierror"
)

var testHeaders = map[string]string{
	"X-Content-Type-Options": "nosniff",
	"X-Frame-Options":        "DENY",
	"Cache-Control":          "no-store",
}

// TestSecurityHeadersSetOnSuccessResponse verifies every configured header
// lands on a normal 2xx response.
func TestSecurityHeadersSetOnSuccessResponse(t *testing.T) {
	handler := SecurityHeaders(testHeaders)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/issues", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	for name, value := range testHeaders {
		if got := rec.Header().Get(name); got != value {
			t.Errorf("expected %s=%q, got %q", name, value, got)
		}
	}
}

// TestSecurityHeadersPassesRequestThrough verifies the wrapped handler still
// runs and controls the response body/status — this middleware only adds
// headers, it never short-circuits.
func TestSecurityHeadersPassesRequestThrough(t *testing.T) {
	called := false
	handler := SecurityHeaders(testHeaders)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusTeapot)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("expected the wrapped handler to run")
	}
	if rec.Code != http.StatusTeapot {
		t.Fatalf("expected the wrapped handler's status to pass through, got %d", rec.Code)
	}
}

// TestSecurityHeadersSetOnNotFound verifies headers are set even when no
// route matches — the middleware wraps the mux, so a 404 from net/http's
// own routing still carries them.
func TestSecurityHeadersSetOnNotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := SecurityHeaders(testHeaders)(mux)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/does-not-exist", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 from the mux, got %d", rec.Code)
	}
	if got := rec.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Errorf("expected X-Frame-Options set on a 404, got %q", got)
	}
}

// TestSecurityHeadersSetOnErrorEnvelope verifies headers survive alongside
// apierror's JSON error envelope, not just plain 2xx responses.
func TestSecurityHeadersSetOnErrorEnvelope(t *testing.T) {
	handler := SecurityHeaders(testHeaders)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apierror.ValidationFailed(w, "bad request")
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/issues", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("expected Cache-Control set on an error envelope, got %q", got)
	}
}

// TestSecurityHeadersSetOnCORSPreflight verifies headers land on a CORS
// preflight response too, proving the documented chain position (outside
// CORS) actually works: CORS's OPTIONS branch short-circuits with its own
// WriteHeader before ever reaching the wrapped handler, so SecurityHeaders
// must run and set its headers before calling into CORS, not after.
func TestSecurityHeadersSetOnCORSPreflight(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := SecurityHeaders(testHeaders)(CORS([]string{"https://frontend.example.com"})(inner))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodOptions, "/issues", nil)
	req.Header.Set("Origin", "https://frontend.example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 from the CORS preflight branch, got %d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://frontend.example.com" {
		t.Errorf("expected the CORS preflight's own header to still be set, got %q", got)
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("expected X-Content-Type-Options set on a CORS preflight response, got %q", got)
	}
}

// TestSecurityHeadersEmptyConfigSetsNothing guards the edge case of an empty
// map (e.g. a misconfigured deploy) — it must not panic and must simply set
// no headers, rather than falling back to some hidden default.
func TestSecurityHeadersEmptyConfigSetsNothing(t *testing.T) {
	handler := SecurityHeaders(map[string]string{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Frame-Options"); got != "" {
		t.Errorf("expected no headers set with an empty config, got X-Frame-Options=%q", got)
	}
}
