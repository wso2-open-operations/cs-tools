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

package main

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-integration-service/internal/entity"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-integration-service/internal/middleware"
)

const testUUID = "11111111-1111-1111-1111-111111111111"

// testHandlers wires every handler to an entity client whose token endpoint
// and every operation are served by one stub that always succeeds, so a request
// that clears the scope guard ends in a 2xx and one that does not never reaches
// the stub.
func testHandlers(t *testing.T) (handlers, *int) {
	t.Helper()
	upstreamCalls := 0
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/token" {
			_, _ = w.Write([]byte(`{"access_token":"t","token_type":"bearer","expires_in":3600}`))
			return
		}
		upstreamCalls++
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(stub.Close)
	client := entity.NewClient(entity.Config{
		BaseURL:      stub.URL,
		TokenURL:     stub.URL + "/token",
		ClientID:     "client",
		ClientSecret: "secret",
	})
	health := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	return newHandlers(client, "actor@example.com", health), &upstreamCalls
}

func tokenWithScopes(t *testing.T, scopes ...string) string {
	t.Helper()
	payload, err := json.Marshal(map[string]any{"scope": strings.Join(scopes, " ")})
	if err != nil {
		t.Fatal(err)
	}
	enc := base64.RawURLEncoding.EncodeToString
	return enc([]byte(`{"alg":"none"}`)) + "." + enc(payload) + "."
}

// requestFor turns a mux pattern into a request that passes each handler's own
// input validation, so the only thing deciding the outcome is the scope guard.
func requestFor(t *testing.T, pattern, token string) *http.Request {
	t.Helper()
	method, path, ok := strings.Cut(pattern, " ")
	if !ok {
		t.Fatalf("pattern %q has no method", pattern)
	}
	path = strings.ReplaceAll(path, "{id}", testUUID)
	body := ""
	switch {
	case method == http.MethodGet:
	case strings.HasSuffix(path, "/comments"):
		body = `{"type":"comment","content":"hello"}`
	case strings.HasSuffix(path, "/tags"):
		body = `{"label":"triaged"}`
	case strings.HasSuffix(path, "/vulnerabilities/sync"):
		body = `[{"wso2Id":"x"}]`
	default:
		body = `{}`
	}
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		r.Header.Set("x-jwt-assertion", token)
	}
	return r
}

func allScopes(h handlers) []string {
	seen := map[string]bool{}
	var out []string
	for _, rt := range routes(h) {
		if !seen[rt.scope] {
			seen[rt.scope] = true
			out = append(out, rt.scope)
		}
	}
	return out
}

func TestRoutes_EveryRouteHasAScopeAndIsUnique(t *testing.T) {
	h, _ := testHandlers(t)
	seen := map[string]bool{}
	for _, rt := range routes(h) {
		if rt.scope == "" {
			t.Errorf("%s has no scope", rt.pattern)
		}
		if rt.handler == nil {
			t.Errorf("%s has no handler", rt.pattern)
		}
		if seen[rt.pattern] {
			t.Errorf("%s registered twice", rt.pattern)
		}
		seen[rt.pattern] = true
	}
}

func TestRoutes_ScopeGuardPerRoute(t *testing.T) {
	h, upstreamCalls := testHandlers(t)
	mux := newMux(h, middleware.NewScopeGuard(true))
	scopes := allScopes(h)

	for _, rt := range routes(h) {
		t.Run(rt.pattern, func(t *testing.T) {
			// No token at all: 401, upstream never called.
			before := *upstreamCalls
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, requestFor(t, rt.pattern, ""))
			if w.Code != http.StatusUnauthorized {
				t.Errorf("no token: status = %d, want 401; body %s", w.Code, w.Body.String())
			}
			if *upstreamCalls != before {
				t.Error("no token: upstream was called")
			}

			// Every scope except the required one: 403, upstream never called.
			var others []string
			for _, s := range scopes {
				if s != rt.scope {
					others = append(others, s)
				}
			}
			w = httptest.NewRecorder()
			mux.ServeHTTP(w, requestFor(t, rt.pattern, tokenWithScopes(t, others...)))
			if w.Code != http.StatusForbidden {
				t.Errorf("other scopes: status = %d, want 403; body %s", w.Code, w.Body.String())
			}
			if *upstreamCalls != before {
				t.Error("other scopes: upstream was called")
			}

			// Exactly the required scope: the handler runs and the stub answers.
			w = httptest.NewRecorder()
			mux.ServeHTTP(w, requestFor(t, rt.pattern, tokenWithScopes(t, rt.scope)))
			if w.Code < 200 || w.Code > 299 {
				t.Errorf("required scope: status = %d, want 2xx; body %s", w.Code, w.Body.String())
			}
			if *upstreamCalls != before+1 {
				t.Errorf("required scope: upstream calls = %d, want %d", *upstreamCalls, before+1)
			}
		})
	}
}

func TestRoutes_HealthNeedsNoToken(t *testing.T) {
	h, _ := testHandlers(t)
	mux := newMux(h, middleware.NewScopeGuard(true))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))
	if w.Code != http.StatusOK {
		t.Errorf("GET /health: status = %d, want 200", w.Code)
	}
}

func TestRoutes_GuardDisabledServesWithoutToken(t *testing.T) {
	h, _ := testHandlers(t)
	mux := newMux(h, middleware.NewScopeGuard(false))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, requestFor(t, "POST /accounts/search", ""))
	if w.Code != http.StatusOK {
		t.Errorf("guard disabled: status = %d, want 200; body %s", w.Code, w.Body.String())
	}
}

func TestRoutes_UnknownRouteIs404(t *testing.T) {
	h, _ := testHandlers(t)
	mux := newMux(h, middleware.NewScopeGuard(true))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/nope", nil))
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}
