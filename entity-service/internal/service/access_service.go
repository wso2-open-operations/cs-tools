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
	//   1. Authorization: Bearer names a client id in AUTH_INTERNAL_CLIENT_IDS:
	//      unrestricted, full stop -- an internal caller is trusted
	//      unconditionally, regardless of any x-user-id-token it also
	//      carries. Every caller configured here is itself an already-trusted
	//      internal service, so a user token from one of them (if present at
	//      all) is used only for attribution elsewhere, never for scoping.
	//   2. Otherwise, resolved purely from x-user-id-token: INTERNAL user_type
	//      sees everything, EXTERNAL (customer) sees only projects they are a
	//      REGISTERED project_contact of, and any other user type, an
	//      inactive user, or an unknown email is refused.
	//   3. No user token and not an internal client: refused (401) -- there's
	//      no legitimate caller to resolve.
	// Refused too if the identity itself isn't validated (only possible if
	// the auth middleware was left out of the chain -- a bug, not a
	// deployment choice): an unverified identity is never used to scope.
	ResolveScope(ctx context.Context) (AccessScope, error)
}

type accessService struct {
	repo              repository.AccessRepository
	internalClientIDs map[string]bool
}

// NewAccessService constructs an AccessService. internalClientIDs is
// config.Config.AuthInternalClientIDs.
func NewAccessService(repo repository.AccessRepository, internalClientIDs map[string]bool) AccessService {
	return &accessService{repo: repo, internalClientIDs: internalClientIDs}
}

// ResolveScope implements AccessService.
func (s *accessService) ResolveScope(ctx context.Context) (AccessScope, error) {
	id := auth.IdentityFromContext(ctx)
	if !id.Validated {
		return AccessScope{}, &apierror.ServiceUnavailableError{Msg: "results cannot be scoped to the caller: no verified identity on this request"}
	}

	if id.ClientID != "" && s.internalClientIDs[id.ClientID] {
		return AccessScope{Unrestricted: true}, nil
	}

	if id.UserEmail == "" {
		return AccessScope{}, &apierror.UnauthorizedError{Msg: "a user token (x-user-id-token) or an authorized internal client credential is required"}
	}
	return s.scopeForUser(ctx, id.UserEmail)
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
		return AccessScope{ProjectIDs: ids, ViewerEmail: email}, nil
	case internal && !other:
		return AccessScope{Unrestricted: true}, nil
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
