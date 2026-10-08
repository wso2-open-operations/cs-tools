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
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// stubAccess answers ResolveScope with a fixed scope or error; every other method of
// the interface is left nil (a call would panic, which these tests never make).
type stubAccess struct {
	service.AccessService
	scope service.AccessScope
	err   error
}

func (s stubAccess) ResolveScope(context.Context) (service.AccessScope, error) {
	return s.scope, s.err
}

// run sends one request through callerIdentityMiddleware and reports what the next
// handler saw in its context.
func runIdentityMiddleware(t *testing.T, access service.AccessService) (scope repository.SearchScope, attached, called bool) {
	t.Helper()
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		called = true
		scope, attached = repository.CallerIdentityFromContext(r.Context())
	})
	callerIdentityMiddleware(access)(next).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	return scope, attached, called
}

// The customer view of the data source that talks to the previous system directly (a customer never gets New, Assess or
// Authorize change requests) is decided from the scope this middleware attaches, and the
// router tests of that rule run without a database, where a customer's scope can never
// be resolved at all (a customer is then the caller with nothing attached). The case
// where a customer's scope IS resolved (a deployment of that data source that also has a
// database to look the user up in) is this one: the middleware must hand the scope on
// exactly as resolved, restricted stays restricted.
func TestCallerIdentityMiddleware_AResolvedCustomerScopeStaysRestricted(t *testing.T) {
	want := service.AccessScope{ViewerEmail: "dana@customer.example", ProjectIDs: []string{"00000000-0000-4000-8000-000000000001"}}

	got, attached, called := runIdentityMiddleware(t, stubAccess{scope: want})

	if !called {
		t.Fatal("the next handler was not called")
	}
	if !attached {
		t.Fatal("a resolved scope was not attached to the context")
	}
	if got.Unrestricted {
		t.Error("a customer's scope came out unrestricted: every customer would get the staff view of change requests on the data source that talks to the previous system directly")
	}
	if got.ViewerEmail != want.ViewerEmail || !slices.Equal(got.ProjectIDs, want.ProjectIDs) {
		t.Errorf("scope = %+v, want %+v", got, want)
	}
}

func TestCallerIdentityMiddleware_AResolvedStaffScopeStaysUnrestricted(t *testing.T) {
	got, attached, _ := runIdentityMiddleware(t, stubAccess{scope: service.AccessScope{Unrestricted: true, HasInternalAccess: true}})

	if !attached || !got.Unrestricted || !got.HasInternalAccess {
		t.Errorf("scope = %+v (attached %v), want the unrestricted staff scope as resolved", got, attached)
	}
}

// A request whose scope cannot be resolved still reaches its handler, with nothing
// attached: that is what makes every consumer fail closed (the customer view applies to
// a request with no resolved scope).
func TestCallerIdentityMiddleware_AnUnresolvedScopeAttachesNothing(t *testing.T) {
	_, attached, called := runIdentityMiddleware(t, stubAccess{err: errors.New("no verified identity")})

	if !called {
		t.Fatal("the next handler was not called: the middleware must never reject")
	}
	if attached {
		t.Error("a scope was attached for a request whose scope could not be resolved")
	}
}
