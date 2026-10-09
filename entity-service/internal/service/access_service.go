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
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// AccessScope is the set of projects (and, through them, cases) a caller may
// see. Cases are scoped by their project, so there is no separate case list.
// An alias of repository.SearchScope (identical shape) so a resolved scope
// passes straight into any repository's scoped query with no field-by-field
// reconstruction at the call site.
type AccessScope = repository.SearchScope

// AccessService turns the verified caller identity into an AccessScope. It is
// meant to be called by every endpoint that returns project- or case-scoped
// data, not only global search -- see each call site for how far that's
// actually been wired up so far.
type AccessService interface {
	// ResolveScope decides what the caller of ctx may see, in this order:
	//   1. Authorization: Bearer names CustomerPortalBackendClientID: ALWAYS resolved
	//      from x-user-id-token (step 4), never unconditionally trusted --
	//      checked first, and deliberately not overridable by this client id
	//      also appearing in M2MClientIDs or equaling CSMPortalBackendClientID by
	//      mistake (see AccessClientConfig's own doc comment).
	//   2. Otherwise, Bearer names CSMPortalBackendClientID: unrestricted ONLY if
	//      x-user-id-token's email also ends in CSMPortalUserDomain
	//      (case-insensitive); otherwise refused (403) outright, not
	//      resolved some other way -- this client id's traffic is expected
	//      to always be that domain, so a mismatch means something upstream
	//      (the IdP, SCIM provisioning) already got the caller's identity
	//      wrong.
	//   3. Otherwise, Bearer names a client id in M2MClientIDs: unrestricted,
	//      full stop, regardless of any x-user-id-token it also carries --
	//      every caller configured here is pure machine-to-machine, so a
	//      user token from one of them (if present at all) is used only for
	//      attribution elsewhere, never for scoping.
	//   4. Otherwise, resolved purely from x-user-id-token: INTERNAL user_type
	//      sees everything, EXTERNAL (customer) sees only projects they are a
	//      REGISTERED project_contact of, and any other user type, an
	//      inactive user, or an unknown email is refused.
	//   5. No user token and none of 1-3 matched: refused (401) -- there's
	//      no legitimate caller to resolve.
	// Refused too if the identity itself isn't validated (only possible if
	// the auth middleware was left out of the chain -- a bug, not a
	// deployment choice): an unverified identity is never used to scope.
	ResolveScope(ctx context.Context) (AccessScope, error)
}

// AccessClientConfig is the subset of config.Config ResolveScope needs to
// classify a client-credentials caller. Grouped into its own type (rather
// than three/four constructor parameters) so a new field here can't silently
// land in the wrong positional slot at the call site.
type AccessClientConfig struct {
	// M2MClientIDs is config.Config.M2MClientIDs: pure machine-to-machine
	// callers, unconditionally unrestricted, no human/domain check.
	M2MClientIDs map[string]bool
	// CSMPortalBackendClientID/CSMPortalUserDomain are config.Config's fields of
	// the same name: apps/csm-portal/backend's client id is unrestricted
	// only if the forwarded user token's email also matches this domain.
	CSMPortalBackendClientID string
	CSMPortalUserDomain      string
	// CustomerPortalBackendClientID is config.Config.CustomerPortalBackendClientID:
	// checked first and always resolved from the forwarded user token,
	// never unconditionally trusted -- see ResolveScope.
	CustomerPortalBackendClientID string
}

type accessService struct {
	repo   repository.AccessRepository
	client AccessClientConfig
}

// NewAccessService constructs an AccessService.
func NewAccessService(repo repository.AccessRepository, client AccessClientConfig) AccessService {
	return &accessService{repo: repo, client: client}
}

// ResolveScope implements AccessService.
//
// Checks repository.CallerIdentityFromContext first: callerIdentityMiddleware
// (internal/server/identity_middleware.go) already calls this exact method
// once per request and attaches its result to ctx for Scoped's benefit, so a
// handler/service that calls ResolveScope again on that same ctx would
// otherwise repeat the full resolution -- for an EXTERNAL caller, two more
// database round trips (UsersByEmail + RegisteredProjectIDs) than the answer
// needs, on every request to any of the several services that both rely on
// Scoped AND call ResolveScope themselves (case_service.go, escalation_
// service.go, global_service.go, project_service.go, schedule_service.go,
// sla_status_service.go, onboarding_step_service.go, announcement_request_
// service.go). Trusting the cached value is safe, not just faster: it can
// only be present because the SAME request's identity already passed this
// exact validation once, earlier in the same middleware chain, and nothing
// downstream can change what auth.Middleware decided about this request's
// identity out from under it. Falling through to the real resolution below
// when nothing is cached also means this keeps working correctly for any
// caller that never went through the middleware (tests, or a future
// non-HTTP caller with its own explicit repository.WithSystemIdentity/
// WithCallerIdentity stamp) -- for the latter, this cache-first check is
// also a correctness fix, not just a speedup: a background job stamped
// WithSystemIdentity has no HTTP request or auth.Identity to resolve from at
// all, so this method would otherwise fail it with "no verified identity"
// even though the caller already explicitly declared itself internal.
func (s *accessService) ResolveScope(ctx context.Context) (AccessScope, error) {
	if cached, ok := repository.CallerIdentityFromContext(ctx); ok {
		return cached, nil
	}

	id := auth.IdentityFromContext(ctx)
	if !id.Validated {
		return AccessScope{}, &apierror.ServiceUnavailableError{Msg: "results cannot be scoped to the caller: no verified identity on this request"}
	}

	if id.ClientID != "" {
		switch {
		// Checked first, and unconditionally: this client id must never
		// reach the unrestricted branches below, regardless of what else
		// it might also be (mis)configured into.
		case s.client.CustomerPortalBackendClientID != "" && id.ClientID == s.client.CustomerPortalBackendClientID:
			if id.UserEmail == "" {
				return AccessScope{}, &apierror.UnauthorizedError{Msg: "a user token (x-user-id-token) is required for this client"}
			}
			scope, err := s.scopeForUser(ctx, id.UserEmail)
			if err != nil {
				return AccessScope{}, err
			}
			// Only marks where the request came from; the data scope is the user's.
			scope.ViaCustomerPortal = true
			return scope, nil
		case s.client.CSMPortalBackendClientID != "" && id.ClientID == s.client.CSMPortalBackendClientID:
			if id.UserEmail != "" && s.isCSMPortalUserDomain(id.UserEmail) {
				return AccessScope{Unrestricted: true, ViewerEmail: id.UserEmail, HasInternalAccess: true}, nil
			}
			return AccessScope{}, &apierror.ForbiddenError{Msg: "csm portal caller's user email is not in an authorized domain"}
		case s.client.M2MClientIDs[id.ClientID]:
			return AccessScope{Unrestricted: true}, nil
		}
	}

	if id.UserEmail == "" {
		return AccessScope{}, &apierror.UnauthorizedError{Msg: "a user token (x-user-id-token) or an authorized internal client credential is required"}
	}
	return s.scopeForUser(ctx, id.UserEmail)
}

// isCSMPortalUserDomain reports whether email ends in
// "@"+AccessClientConfig.CSMPortalUserDomain, case-insensitively -- matching
// the suffix on "@domain" rather than bare "domain" so e.g. "evilwso2.com"
// cannot pass a "wso2.com" check (mirrors sn_case_service.go's wso2EmailDomain
// pattern).
func (s *accessService) isCSMPortalUserDomain(email string) bool {
	if s.client.CSMPortalUserDomain == "" {
		return false
	}
	return strings.HasSuffix(strings.ToLower(email), "@"+strings.ToLower(s.client.CSMPortalUserDomain))
}

// scopeForUser maps a user's type to a scope. user.email is not unique, so the
// active rows for the email are combined conservatively: internal access needs
// every active row to be INTERNAL. An email that is also (or only) an EXTERNAL
// customer is scoped like a customer -- less access, never more, when the data
// is ambiguous. An email with no row at all is refused: unlike an internal
// caller (see ResolveScope), a non-internal caller gets no benefit of the
// doubt for an unknown user.
func (s *accessService) scopeForUser(ctx context.Context, email string) (AccessScope, error) {
	users, err := s.repo.UsersByEmail(ctx, email)
	if err != nil {
		return AccessScope{}, err
	}

	var internal, external, other bool
	for _, u := range users {
		if !u.Active {
			continue
		}
		switch u.UserType {
		case "INTERNAL":
			internal = true
		case "EXTERNAL":
			external = true
		default:
			other = true
		}
	}

	switch {
	case external:
		ids, err := s.repo.RegisteredProjectIDs(ctx, email)
		if err != nil {
			return AccessScope{}, err
		}
		// HasInternalAccess: this email also has an active INTERNAL row (a
		// mixed identity) -- the data-visibility scope still follows the
		// conservative "external wins" rule above, but the caller is still
		// genuinely WSO2 staff; see AccessScope.HasInternalAccess's own doc
		// comment for why that distinction has to survive this branch.
		return AccessScope{ProjectIDs: ids, ViewerEmail: email, HasInternalAccess: internal}, nil
	case internal && !other:
		return AccessScope{Unrestricted: true, HasInternalAccess: true}, nil
	default:
		return AccessScope{}, &apierror.ForbiddenError{Msg: "no access for this user"}
	}
}

// resolveScopeForID is the preamble every scoped by-id read shares: reject a
// malformed id before touching the identity or the database, then resolve the
// caller's scope. Kept in one place so GetProjectByID and GetCaseByID can't
// drift on check ordering (e.g. if audit logging is added later).
func resolveScopeForID(ctx context.Context, access AccessService, id string) (AccessScope, error) {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return AccessScope{}, err
	}
	return access.ResolveScope(ctx)
}

// authorizeProject refuses a caller who may not act on this project, and
// returns the resolved scope so the caller can pass it on to any
// RLS-protected repository query it makes on the project's behalf (see
// repository.SearchScope/runWithCallerIdentity) -- authorizeProject already
// resolves this scope to perform its own check, so returning it here means
// callers never need a second ResolveScope call just to get the identity
// they must forward.
//
// Two kinds of endpoint need this. A by-id read takes the project id straight
// from the request path, so validating only that the project EXISTS lets
// anyone read any project's data by id -- an IDOR. And a write, or a call that
// leaves the service entirely (the Choreo provisioning sequence), has nothing
// to attach a scope predicate to in the first place. Scoped list endpoints get
// this for free by folding the caller's AccessScope into their WHERE clause;
// these check membership here instead, before doing anything.
//
// Refused as NotFound, never Forbidden, matching GetProjectByID/GetCaseByID: a
// 403 would confirm the project exists to someone not entitled to know that.
// Scope is resolved before any existence lookup, so the two cases are not
// distinguishable by timing either.
//
// projectID is compared case-insensitively. Postgres renders uuid values in
// lower case, but the id here comes from the request path, and a caller who
// upper-cases a UUID they legitimately hold must not be locked out.
func authorizeProject(ctx context.Context, access AccessService, projectID string) (AccessScope, error) {
	scope, err := resolveScopeForID(ctx, access, projectID)
	if err != nil {
		return AccessScope{}, err
	}
	if scope.Unrestricted {
		return scope, nil
	}
	for _, id := range scope.ProjectIDs {
		if strings.EqualFold(id, projectID) {
			return scope, nil
		}
	}
	return AccessScope{}, &apierror.NotFoundError{Msg: "project not found"}
}
