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

package service

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

type fakeAccessRepo struct {
	users    []repository.AccessUser
	projects []string
	// userLookups/projectCalls count calls, so tests can prove the internal
	// path never touches the database at all.
	userLookups  int
	projectCalls int
}

func (f *fakeAccessRepo) UsersByEmail(context.Context, string) ([]repository.AccessUser, error) {
	f.userLookups++
	return f.users, nil
}
func (f *fakeAccessRepo) RegisteredProjectIDs(context.Context, string) ([]string, error) {
	f.projectCalls++
	return f.projects, nil
}

func idCtx(id auth.Identity) context.Context { return auth.WithIdentity(context.Background(), id) }

func userOf(t string, active bool) repository.AccessUser {
	return repository.AccessUser{UserType: t, Active: active}
}

var testClientConfig = AccessClientConfig{
	M2MClientIDs:                  map[string]bool{"csm-backend": true, "integration": true},
	CSMPortalBackendClientID:      "csm-portal",
	CSMPortalUserDomain:           "wso2.com",
	CustomerPortalBackendClientID: "customer-portal",
}

func TestAccessService_ResolveScope(t *testing.T) {
	const email = "jane@example.com"
	tests := []struct {
		name                  string
		id                    auth.Identity
		users                 []repository.AccessUser
		projects              []string
		wantErr               any // nil, or a pointer to the expected apierror type
		wantAll               bool
		wantProjects          []string
		wantNoDBCall          bool // internal-client path must never touch the repo
		wantHasInternalAccess bool
	}{
		{name: "identity not validated -> refuse (503), never trust the token",
			id: auth.Identity{Validated: false, UserEmail: email}, wantErr: &apierror.ServiceUnavailableError{}, wantNoDBCall: true},

		// --- Internal client: unconditional full access, user token or not ---
		{name: "internal client, no user token at all -> everything",
			id: auth.Identity{Validated: true, ClientID: "csm-backend"}, wantAll: true, wantNoDBCall: true},
		{name: "internal client forwarding a user token whose email isn't in \"user\" at all -> still everything, DB never consulted",
			id: auth.Identity{Validated: true, ClientID: "csm-backend", UserEmail: email}, users: nil, wantAll: true, wantNoDBCall: true},
		{name: "internal client forwarding a KNOWN customer's token -> still everything: internal role wins outright, no rescue/exception logic",
			id: auth.Identity{Validated: true, ClientID: "csm-backend", UserEmail: email}, users: []repository.AccessUser{userOf("EXTERNAL", true)},
			projects: []string{"p1"}, wantAll: true, wantNoDBCall: true},
		{name: "internal client forwarding an inactive user's token -> still everything",
			id: auth.Identity{Validated: true, ClientID: "csm-backend", UserEmail: email}, users: []repository.AccessUser{userOf("INTERNAL", false)}, wantAll: true, wantNoDBCall: true},

		// --- Not an internal client: resolved purely from the user token ---
		{name: "internal user sees everything",
			id: auth.Identity{Validated: true, UserEmail: email}, users: []repository.AccessUser{userOf("INTERNAL", true)}, wantAll: true, wantHasInternalAccess: true},
		{name: "customer sees only registered projects",
			id: auth.Identity{Validated: true, UserEmail: email}, users: []repository.AccessUser{userOf("EXTERNAL", true)},
			projects: []string{"p1", "p2"}, wantProjects: []string{"p1", "p2"}},
		{name: "customer with no registered projects gets an EMPTY scope, not everything",
			id: auth.Identity{Validated: true, UserEmail: email}, users: []repository.AccessUser{userOf("EXTERNAL", true)},
			projects: []string{}, wantProjects: []string{}},
		// HasInternalAccess must stay true here even though Unrestricted is
		// false: this caller is still genuinely WSO2 staff (a mixed
		// identity), just scoped like a customer for data-VISIBILITY
		// purposes (see AccessScope.HasInternalAccess's own doc comment) --
		// a CodeRabbit-caught gap in an earlier fix that read Unrestricted
		// alone as "is this caller internal".
		{name: "email shared by an internal and an external row -> customer scope (less access), but HasInternalAccess stays true",
			id: auth.Identity{Validated: true, UserEmail: email}, users: []repository.AccessUser{userOf("INTERNAL", true), userOf("EXTERNAL", true)},
			projects: []string{"p1"}, wantProjects: []string{"p1"}, wantHasInternalAccess: true},
		{name: "inactive internal row does not count",
			id: auth.Identity{Validated: true, UserEmail: email}, users: []repository.AccessUser{userOf("INTERNAL", false)}, wantErr: &apierror.ForbiddenError{}},
		{name: "inactive internal + active customer -> customer, HasInternalAccess false (the internal row doesn't count)",
			id: auth.Identity{Validated: true, UserEmail: email}, users: []repository.AccessUser{userOf("INTERNAL", false), userOf("EXTERNAL", true)},
			projects: []string{"p9"}, wantProjects: []string{"p9"}, wantHasInternalAccess: false},
		{name: "internal row alongside a NOT_AVAILABLE row -> denied",
			id: auth.Identity{Validated: true, UserEmail: email}, users: []repository.AccessUser{userOf("INTERNAL", true), userOf("NOT_AVAILABLE", true)}, wantErr: &apierror.ForbiddenError{}},
		{name: "system user is not a person -> denied",
			id: auth.Identity{Validated: true, UserEmail: email}, users: []repository.AccessUser{userOf("SYSTEM", true)}, wantErr: &apierror.ForbiddenError{}},
		{name: "NULL user_type -> denied",
			id: auth.Identity{Validated: true, UserEmail: email}, users: []repository.AccessUser{userOf("", true)}, wantErr: &apierror.ForbiddenError{}},
		{name: "unknown email, non-internal caller -> denied outright (no rescue -- that's internal-client only)",
			id: auth.Identity{Validated: true, UserEmail: email}, users: nil, wantErr: &apierror.ForbiddenError{}},
		{name: "unknown email forwarded by an unlisted client -> still denied",
			id: auth.Identity{Validated: true, ClientID: "stranger", UserEmail: email}, users: nil, wantErr: &apierror.ForbiddenError{}},

		{name: "no user token, unlisted client -> 401 (no legitimate caller to resolve)",
			id: auth.Identity{Validated: true, ClientID: "stranger"}, wantErr: &apierror.UnauthorizedError{}, wantNoDBCall: true},
		{name: "no user and no client at all -> 401",
			id: auth.Identity{Validated: true}, wantErr: &apierror.UnauthorizedError{}, wantNoDBCall: true},

		// --- CSM portal client: unrestricted ONLY with a matching-domain user email ---
		{name: "csm portal client, matching domain -> everything, no DB lookup needed",
			id: auth.Identity{Validated: true, ClientID: "csm-portal", UserEmail: "engineer@wso2.com"}, wantAll: true, wantHasInternalAccess: true, wantNoDBCall: true},
		{name: "csm portal client, matching domain case-insensitively -> everything",
			id: auth.Identity{Validated: true, ClientID: "csm-portal", UserEmail: "Engineer@WSO2.COM"}, wantAll: true, wantHasInternalAccess: true, wantNoDBCall: true},
		{name: "csm portal client, non-matching domain -> refused (403), not resolved some other way",
			id: auth.Identity{Validated: true, ClientID: "csm-portal", UserEmail: "someone@example.com"}, wantErr: &apierror.ForbiddenError{}, wantNoDBCall: true},
		{name: "csm portal client, domain is a suffix trick (evilwso2.com) -> refused, not treated as wso2.com",
			id: auth.Identity{Validated: true, ClientID: "csm-portal", UserEmail: "attacker@evilwso2.com"}, wantErr: &apierror.ForbiddenError{}, wantNoDBCall: true},
		{name: "csm portal client, no user token at all -> refused (403), never falls back to unconditional trust",
			id: auth.Identity{Validated: true, ClientID: "csm-portal"}, wantErr: &apierror.ForbiddenError{}, wantNoDBCall: true},

		// --- Customer portal client: ALWAYS resolved from the user token, never unconditionally trusted ---
		{name: "customer portal client forwarding a customer's token -> scoped to their registered projects, same as any other caller",
			id: auth.Identity{Validated: true, ClientID: "customer-portal", UserEmail: email}, users: []repository.AccessUser{userOf("EXTERNAL", true)},
			projects: []string{"p1"}, wantProjects: []string{"p1"}},
		{name: "customer portal client forwarding a genuinely INTERNAL user's token -> everything, but only because the DB says so, not the client id",
			id: auth.Identity{Validated: true, ClientID: "customer-portal", UserEmail: email}, users: []repository.AccessUser{userOf("INTERNAL", true)},
			wantAll: true, wantHasInternalAccess: true},
		{name: "customer portal client, no user token at all -> 401, never unconditionally trusted",
			id: auth.Identity{Validated: true, ClientID: "customer-portal"}, wantErr: &apierror.UnauthorizedError{}, wantNoDBCall: true},
	}
	for _, tt := range tests {
		repo := &fakeAccessRepo{users: tt.users, projects: tt.projects}
		scope, err := NewAccessService(repo, testClientConfig).ResolveScope(idCtx(tt.id))

		if tt.wantNoDBCall && (repo.userLookups != 0 || repo.projectCalls != 0) {
			t.Errorf("%s: repo was consulted (userLookups=%d projectCalls=%d), want zero DB calls", tt.name, repo.userLookups, repo.projectCalls)
		}

		if tt.wantErr != nil {
			if err == nil {
				t.Errorf("%s: got scope %+v, want an error", tt.name, scope)
				continue
			}
			var ok bool
			switch tt.wantErr.(type) {
			case *apierror.ServiceUnavailableError:
				var e *apierror.ServiceUnavailableError
				ok = errors.As(err, &e)
			case *apierror.ForbiddenError:
				var e *apierror.ForbiddenError
				ok = errors.As(err, &e)
			case *apierror.UnauthorizedError:
				var e *apierror.UnauthorizedError
				ok = errors.As(err, &e)
			}
			if !ok {
				t.Errorf("%s: got %T (%v), want %T", tt.name, err, err, tt.wantErr)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected error %v", tt.name, err)
			continue
		}
		if scope.Unrestricted != tt.wantAll {
			t.Errorf("%s: Unrestricted = %v, want %v", tt.name, scope.Unrestricted, tt.wantAll)
		}
		if !tt.wantAll {
			if scope.ProjectIDs == nil || len(scope.ProjectIDs) != len(tt.wantProjects) {
				t.Errorf("%s: ProjectIDs = %#v, want %v (non-nil)", tt.name, scope.ProjectIDs, tt.wantProjects)
			}
		}
		if scope.HasInternalAccess != tt.wantHasInternalAccess {
			t.Errorf("%s: HasInternalAccess = %v, want %v", tt.name, scope.HasInternalAccess, tt.wantHasInternalAccess)
		}
	}
}

// Internal users must not trigger a project lookup at all.
func TestAccessService_InternalSkipsProjectLookup(t *testing.T) {
	repo := &fakeAccessRepo{users: []repository.AccessUser{userOf("INTERNAL", true)}}
	_, _ = NewAccessService(repo, AccessClientConfig{}).ResolveScope(idCtx(auth.Identity{Validated: true, UserEmail: "a@b.c"}))
	if repo.projectCalls != 0 {
		t.Fatalf("project lookups = %d, want 0", repo.projectCalls)
	}
}

// TestAccessService_ResolveScope_UsesCachedScopeFromContext is the
// regression test for issue #2128: callerIdentityMiddleware already calls
// ResolveScope once per request and stamps its result onto ctx for Scoped's
// benefit (repository.WithCallerIdentity) -- a second call on that same ctx
// (from case_service.go/escalation_service.go/etc, each resolving its own
// AccessScope independently) must reuse that cached value rather than
// repeating the full resolution (UsersByEmail + RegisteredProjectIDs for an
// external caller). Proven here by a fakeAccessRepo that would fail loudly
// (call counts > 0) if ResolveScope re-resolved instead of trusting the
// cache.
func TestAccessService_ResolveScope_UsesCachedScopeFromContext(t *testing.T) {
	repo := &fakeAccessRepo{}
	svc := NewAccessService(repo, testClientConfig)

	want := repository.SearchScope{Unrestricted: false, ProjectIDs: []string{"p1", "p2"}, ViewerEmail: "cached@test.local"}
	// Deliberately NOT auth.WithIdentity: a cache hit must not need to
	// re-derive anything from the auth identity at all -- this ctx carries
	// only the middleware's own cached scope, exactly like a real request
	// reaching a second ResolveScope call downstream.
	ctx := repository.WithCallerIdentity(context.Background(), want)

	got, err := svc.ResolveScope(ctx)
	if err != nil {
		t.Fatalf("ResolveScope() with a cached scope on ctx: unexpected error = %v", err)
	}
	if got.Unrestricted != want.Unrestricted || got.ViewerEmail != want.ViewerEmail || !slices.Equal(got.ProjectIDs, want.ProjectIDs) {
		t.Errorf("ResolveScope() = %+v, want the cached %+v unchanged", got, want)
	}
	if repo.userLookups != 0 || repo.projectCalls != 0 {
		t.Errorf("repo calls = %d userLookups, %d projectCalls, want 0 and 0 -- a cache hit must never touch the database", repo.userLookups, repo.projectCalls)
	}
}

// TestAccessService_ResolveScope_FallsThroughWithoutCachedScope confirms the
// cache-first check in TestAccessService_ResolveScope_UsesCachedScopeFromContext
// doesn't break the ordinary (uncached) path: a ctx with no prior
// repository.WithCallerIdentity stamp -- the normal case for the middleware's
// own first call on a request -- still resolves for real, against the repo.
func TestAccessService_ResolveScope_FallsThroughWithoutCachedScope(t *testing.T) {
	repo := &fakeAccessRepo{users: []repository.AccessUser{userOf("EXTERNAL", true)}, projects: []string{"p1"}}
	svc := NewAccessService(repo, testClientConfig)

	scope, err := svc.ResolveScope(idCtx(auth.Identity{Validated: true, UserEmail: "real@test.local"}))
	if err != nil {
		t.Fatalf("ResolveScope() with no cached scope: unexpected error = %v", err)
	}
	if scope.Unrestricted || len(scope.ProjectIDs) != 1 || scope.ProjectIDs[0] != "p1" {
		t.Errorf("ResolveScope() = %+v, want a real resolution against the repo (ProjectIDs=[p1])", scope)
	}
	if repo.userLookups != 1 || repo.projectCalls != 1 {
		t.Errorf("repo calls = %d userLookups, %d projectCalls, want exactly 1 and 1 -- the uncached path must still do real work", repo.userLookups, repo.projectCalls)
	}
}

// TestAccessService_CustomerPortalBackendClientID_OverlapWithM2M is the regression
// test for the scenario this three-config split exists to prevent: a
// customer-facing BFF's client id ending up ALSO listed in M2MClientIDs
// (a copy-paste mistake in config, or the wrong list entirely) must not
// grant it the unconditional access that list otherwise confers --
// CustomerPortalBackendClientID is checked first and wins regardless of what else
// the id appears in. See AccessClientConfig and ResolveScope's own doc
// comments.
func TestAccessService_CustomerPortalBackendClientID_OverlapWithM2M(t *testing.T) {
	cfg := AccessClientConfig{
		M2MClientIDs:                  map[string]bool{"customer-portal": true}, // the mistake
		CustomerPortalBackendClientID: "customer-portal",
	}
	repo := &fakeAccessRepo{users: []repository.AccessUser{userOf("EXTERNAL", true)}, projects: []string{"p1"}}
	svc := NewAccessService(repo, cfg)

	scope, err := svc.ResolveScope(idCtx(auth.Identity{Validated: true, ClientID: "customer-portal", UserEmail: "customer@example.com"}))
	if err != nil {
		t.Fatalf("ResolveScope() = err %v, want a normal scoped resolution", err)
	}
	if scope.Unrestricted {
		t.Fatal("ResolveScope() returned Unrestricted=true for a customer-portal client id present in M2MClientIDs -- the veto failed to prevent the exact misconfiguration it exists for")
	}
	if len(scope.ProjectIDs) != 1 || scope.ProjectIDs[0] != "p1" {
		t.Errorf("ProjectIDs = %v, want [p1] (resolved from the forwarded user token, not the client id)", scope.ProjectIDs)
	}
	if repo.userLookups != 1 {
		t.Errorf("userLookups = %d, want 1 -- the veto must still resolve from the user token, not skip the DB like an M2M caller would", repo.userLookups)
	}
}

// TestAccessService_CSMPortalBackendClientID_OverlapWithM2M proves the CSM-portal
// domain gate survives the same kind of overlap: even if CSMPortalBackendClientID
// is also mistakenly listed in M2MClientIDs, a non-matching-domain user
// email is still refused, not unconditionally trusted.
func TestAccessService_CSMPortalBackendClientID_OverlapWithM2M(t *testing.T) {
	cfg := AccessClientConfig{
		M2MClientIDs:             map[string]bool{"csm-portal": true}, // the mistake
		CSMPortalBackendClientID: "csm-portal",
		CSMPortalUserDomain:      "wso2.com",
	}
	repo := &fakeAccessRepo{}
	svc := NewAccessService(repo, cfg)

	_, err := svc.ResolveScope(idCtx(auth.Identity{Validated: true, ClientID: "csm-portal", UserEmail: "someone@example.com"}))
	var forbidden *apierror.ForbiddenError
	if !errors.As(err, &forbidden) {
		t.Fatalf("ResolveScope() = %T (%v), want *apierror.ForbiddenError -- the domain gate must survive being also listed in M2MClientIDs", err, err)
	}
}

// ViaCustomerPortal marks where a request came from, and nothing else: it is set for every
// caller the customer portal's backend forwards (a customer and WSO2 staff alike), the data
// scope is still the user's, and no other client sets it.
func TestAccessService_ResolveScope_ViaCustomerPortal(t *testing.T) {
	tests := []struct {
		name          string
		id            auth.Identity
		users         []repository.AccessUser
		wantAll       bool
		wantViaPortal bool
	}{
		{"a customer through the customer portal", auth.Identity{Validated: true, ClientID: "customer-portal", UserEmail: "c@example.com"},
			[]repository.AccessUser{userOf("EXTERNAL", true)}, false, true},
		{"WSO2 staff through the customer portal keeps full data scope, and is marked", auth.Identity{Validated: true, ClientID: "customer-portal", UserEmail: "s@wso2.com"},
			[]repository.AccessUser{userOf("INTERNAL", true)}, true, true},
		{"staff through the CSM portal is not marked", auth.Identity{Validated: true, ClientID: "csm-portal", UserEmail: "s@wso2.com"},
			nil, true, false},
		{"a machine client is not marked", auth.Identity{Validated: true, ClientID: "csm-backend"}, nil, true, false},
		{"a user token with no client is not marked", auth.Identity{Validated: true, UserEmail: "s@wso2.com"},
			[]repository.AccessUser{userOf("INTERNAL", true)}, true, false},
	}
	for _, tt := range tests {
		repo := &fakeAccessRepo{users: tt.users, projects: []string{"p1"}}
		scope, err := NewAccessService(repo, testClientConfig).ResolveScope(idCtx(tt.id))
		if err != nil {
			t.Errorf("%s: unexpected error %v", tt.name, err)
			continue
		}
		if scope.ViaCustomerPortal != tt.wantViaPortal {
			t.Errorf("%s: ViaCustomerPortal = %v, want %v", tt.name, scope.ViaCustomerPortal, tt.wantViaPortal)
		}
		if scope.Unrestricted != tt.wantAll {
			t.Errorf("%s: Unrestricted = %v, want %v -- the data scope must not change", tt.name, scope.Unrestricted, tt.wantAll)
		}
	}
}
