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
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORSPreflightDoesNotReachHandler(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := CORS([]string{"https://frontend.example.com"})(next)

	req := httptest.NewRequest(http.MethodOptions, "/issues", nil)
	req.Header.Set("Origin", "https://frontend.example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if called {
		t.Fatal("preflight should not reach the wrapped handler")
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://frontend.example.com" {
		t.Fatalf("expected Allow-Origin echoed, got %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Fatal("expected Allow-Methods to be set")
	}
}

func TestCORSRejectsDisallowedOrigin(t *testing.T) {
	handler := CORS([]string{"https://frontend.example.com"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no Allow-Origin for disallowed origin, got %q", got)
	}
	// A denied-origin response varies by Origin exactly as much as an
	// allowed one does — omitting Vary here would let a cache reuse this
	// denied response for a later, legitimately allowed origin's request.
	if got := rec.Header().Get("Vary"); got != "Origin" {
		t.Fatalf("expected Vary: Origin even for a disallowed origin, got %q", got)
	}
}

// TestCORSNeverSetsAllowCredentials guards against reintroducing
// Access-Control-Allow-Credentials: true alongside an unrestricted (or
// reflected-any) Origin — that combination lets any site read authenticated
// responses on the victim's behalf. This backend is authless (D7) and has no
// cookie-based session for a browser to attach automatically, and no
// legitimate reason to ever set this header — see the doc comment on CORS.
func TestCORSNeverSetsAllowCredentials(t *testing.T) {
	handler := CORS(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Origin", "https://anything.example.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Fatalf("Access-Control-Allow-Credentials must never be set, got %q", got)
	}
}

func TestCORSActualRequestPassesThrough(t *testing.T) {
	// CORS only ever governs the Access-Control-Allow-Origin response
	// header — it never blocks the request from reaching the wrapped
	// handler itself (the browser is what enforces the header client-side).
	called := false
	handler := CORS([]string{"https://frontend.example.com"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/issues", nil)
	req.Header.Set("Origin", "https://frontend.example.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("actual request should reach the wrapped handler")
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://frontend.example.com" {
		t.Fatalf("expected Allow-Origin echoed for an allow-listed origin, got %q", got)
	}
}

// TestCORSEmptyAllowListDeniesEveryOrigin guards the fail-closed default:
// an empty/unset allowedOrigins must never fall back to reflecting any
// Origin — see CORS's doc comment for why open-by-default was rejected.
func TestCORSEmptyAllowListDeniesEveryOrigin(t *testing.T) {
	handler := CORS(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Origin", "https://anything.example.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no Allow-Origin with an empty allow-list, got %q", got)
	}
}
