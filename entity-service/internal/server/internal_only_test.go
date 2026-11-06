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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

type fakeAccess struct {
	scope service.AccessScope
	err   error
}

func (f fakeAccess) ResolveScope(context.Context) (service.AccessScope, error) { return f.scope, f.err }

func TestInternalOnly(t *testing.T) {
	cases := []struct {
		name   string
		access fakeAccess
		want   int
		called bool
	}{
		{"internal caller passes", fakeAccess{scope: service.AccessScope{Unrestricted: true}}, http.StatusOK, true},
		{"customer is forbidden", fakeAccess{scope: service.AccessScope{ViewerEmail: "c@example.com"}}, http.StatusForbidden, false},
		{"unauthenticated is 401", fakeAccess{err: &apierror.UnauthorizedError{Msg: "no token"}}, http.StatusUnauthorized, false},
		{"unverified identity is 503", fakeAccess{err: &apierror.ServiceUnavailableError{Msg: "unverified"}}, http.StatusServiceUnavailable, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			h := internalOnly(tc.access, func(w http.ResponseWriter, _ *http.Request) { called = true })
			rec := httptest.NewRecorder()
			h(rec, httptest.NewRequest(http.MethodPost, "/slas/search", nil))
			if called != tc.called {
				t.Fatalf("handler called = %v, want %v", called, tc.called)
			}
			if !tc.called && rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}

func TestServiceOnly(t *testing.T) {
	internal := fakeAccess{scope: service.AccessScope{Unrestricted: true}}
	cases := []struct {
		name   string
		access fakeAccess
		user   string
		want   int
		called bool
	}{
		{"service credential passes", internal, "", http.StatusOK, true},
		{"internal user is forbidden", internal, "engineer@example.com", http.StatusForbidden, false},
		{"customer is forbidden", fakeAccess{scope: service.AccessScope{ViewerEmail: "c@example.com"}}, "c@example.com", http.StatusForbidden, false},
		{"unauthenticated is 401", fakeAccess{err: &apierror.UnauthorizedError{Msg: "no token"}}, "", http.StatusUnauthorized, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			h := serviceOnly(tc.access, func(w http.ResponseWriter, _ *http.Request) { called = true })
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/acp/reopen-check", nil)
			req = req.WithContext(auth.WithIdentity(req.Context(), auth.Identity{Validated: true, UserEmail: tc.user}))
			h(rec, req)
			if called != tc.called {
				t.Fatalf("handler called = %v, want %v", called, tc.called)
			}
			if !tc.called && rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}
