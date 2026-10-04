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

package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// countingAccess is a fakeAccess that also records how often it was asked.
type countingAccess struct {
	fakeAccess
	calls int
}

func (c *countingAccess) ResolveScope(ctx context.Context) (service.AccessScope, error) {
	c.calls++
	return c.fakeAccess.ResolveScope(ctx)
}

// identityGateHarness builds a mux with one guarded route and one anonymous
// route behind callerIdentityMiddleware, recording whether the guarded
// handler ran and what identity it saw.
type identityGateHarness struct {
	handler  http.Handler
	access   *countingAccess
	called   bool
	identity service.AccessScope
	stamped  bool
}

func newIdentityGateHarness(access *countingAccess) *identityGateHarness {
	h := &identityGateHarness{access: access}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /guarded", func(w http.ResponseWriter, r *http.Request) {
		h.called = true
		h.identity, h.stamped = repository.CallerIdentityFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /open", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h.handler = callerIdentityMiddleware(access, mux, map[string]bool{"GET /open": true})
	return h
}

// validated returns a request whose identity auth.Middleware would have
// marked as validated (the normal production case).
func validated(method, path string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	return req.WithContext(auth.WithIdentity(req.Context(), auth.Identity{Validated: true}))
}

func TestCallerIdentityMiddleware_StampsAResolvedCaller(t *testing.T) {
	access := &countingAccess{fakeAccess: fakeAccess{scope: service.AccessScope{Unrestricted: true}}}
	h := newIdentityGateHarness(access)

	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, validated(http.MethodPost, "/guarded"))

	if rec.Code != http.StatusOK || !h.called {
		t.Fatalf("status = %d, called = %v; want the handler to run", rec.Code, h.called)
	}
	if !h.stamped || !h.identity.Unrestricted {
		t.Fatalf("identity on context = %+v (stamped %v); want the resolved scope", h.identity, h.stamped)
	}
}

func TestCallerIdentityMiddleware_RefusesAnAnonymousCaller(t *testing.T) {
	access := &countingAccess{fakeAccess: fakeAccess{err: &apierror.UnauthorizedError{Msg: "a user token or an authorized internal client credential is required"}}}
	h := newIdentityGateHarness(access)

	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, validated(http.MethodPost, "/guarded"))

	if h.called {
		t.Fatal("handler ran for a caller with no resolvable identity")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	var body apierror.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not the standard error envelope: %v: %s", err, rec.Body.String())
	}
	if body.Code != http.StatusUnauthorized || body.Message == "" {
		t.Fatalf("body = %+v, want code 401 with the resolver's message", body)
	}
}

func TestCallerIdentityMiddleware_RefusesAnUnvalidatedIdentity(t *testing.T) {
	access := &countingAccess{fakeAccess: fakeAccess{scope: service.AccessScope{Unrestricted: true}}}
	h := newIdentityGateHarness(access)

	// No auth.Identity on the context at all: the auth middleware never ran.
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/guarded", nil))

	if h.called {
		t.Fatal("handler ran for a request whose identity was never validated")
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if access.calls != 0 {
		t.Fatalf("ResolveScope called %d times for an unvalidated identity, want 0", access.calls)
	}
}

func TestCallerIdentityMiddleware_PassesOtherFailuresThroughUnstamped(t *testing.T) {
	for name, err := range map[string]error{
		"forbidden (known token, no active user row)": &apierror.ForbiddenError{Msg: "no access for this user"},
		"lookup failure":         errors.New("connection refused"),
		"no database configured": &apierror.ServiceUnavailableError{Msg: "no database configured"},
	} {
		t.Run(name, func(t *testing.T) {
			access := &countingAccess{fakeAccess: fakeAccess{err: err}}
			h := newIdentityGateHarness(access)

			rec := httptest.NewRecorder()
			h.handler.ServeHTTP(rec, validated(http.MethodPost, "/guarded"))

			if rec.Code != http.StatusOK || !h.called {
				t.Fatalf("status = %d, called = %v; want the handler to decide", rec.Code, h.called)
			}
			if h.stamped {
				t.Fatalf("identity %+v attached after a failed resolution; want none", h.identity)
			}
		})
	}
}

func TestCallerIdentityMiddleware_LeavesUnmatchedRoutesToTheMux(t *testing.T) {
	access := &countingAccess{fakeAccess: fakeAccess{err: &apierror.UnauthorizedError{Msg: "no token"}}}
	h := newIdentityGateHarness(access)

	for _, tc := range []struct {
		method, path string
		want         int
	}{
		{http.MethodPost, "/nowhere", http.StatusNotFound},
		{http.MethodGet, "/guarded", http.StatusMethodNotAllowed},
	} {
		rec := httptest.NewRecorder()
		h.handler.ServeHTTP(rec, validated(tc.method, tc.path))
		if rec.Code != tc.want {
			t.Errorf("%s %s = %d, want %d from the mux, not a 401 from the gate", tc.method, tc.path, rec.Code, tc.want)
		}
	}
	if access.calls != 0 {
		t.Fatalf("ResolveScope called %d times for requests no route matches, want 0", access.calls)
	}
}

func TestCallerIdentityMiddleware_ServesAnonymousRoutesWithoutResolving(t *testing.T) {
	access := &countingAccess{fakeAccess: fakeAccess{err: &apierror.UnauthorizedError{Msg: "no token"}}}
	h := newIdentityGateHarness(access)

	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/open", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /open = %d, want 200 for an allow-listed anonymous route", rec.Code)
	}
	if access.calls != 0 {
		t.Fatalf("ResolveScope called %d times for an anonymous route, want 0", access.calls)
	}
}
