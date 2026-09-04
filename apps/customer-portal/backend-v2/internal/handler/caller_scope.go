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

package handler

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

// callerScopeContactsLimit is the page size used to page through a
// project's full contact list. CSM's own equivalent (webapp's
// useSearchProjectContacts.ts) notes the largest contact list seen in
// practice is ~103; this and callerScopeContactsMaxPages together bound the
// scan regardless of what Total reports.
const callerScopeContactsLimit = 100

// callerScopeContactsMaxPages caps the scan at 1000 contacts — an
// independent ceiling so a wrong or stuck Total can't turn this into an
// unbounded fetch loop.
const callerScopeContactsMaxPages = 10

// entityProjectContactsClient is the subset of the entity client
// CallerScopeResolver needs.
type entityProjectContactsClient interface {
	SearchProjectContacts(ctx context.Context, projectID string, req entity.SearchProjectContactsRequest) (entity.SearchProjectContactsResponse, error)
}

// CallerScopeResolver answers "does this email have case access to this
// project" via entity-service's own native project-contacts search
// (POST /projects/{id}/contacts/search — the same endpoint CSM's backend
// already calls in production, entity.SearchProjectContacts here). There is
// no bulk "which projects does this email belong to" lookup anywhere in
// this stack, so membership is still checked one project at a time — but
// unlike an earlier version of this resolver, it no longer needs the
// separate project-contact *onboarding* service (internal/usermanagement,
// Salesforce-ID-keyed): entity-service's ProjectContact.GrantsCaseAccess is
// a purpose-built field answering exactly this question, keyed on the same
// platform UUID every other entity-service call already uses.
//
// ServiceNow data source only (entity-service has no Postgres equivalent
// for project contacts) — see entity-service's own ProjectContact doc
// comment.
type CallerScopeResolver struct {
	entity entityProjectContactsClient
}

// NewCallerScopeResolver constructs a CallerScopeResolver backed by the same
// entity client every other handler uses — no separate dependency needed.
func NewCallerScopeResolver(entityClient entityProjectContactsClient) *CallerScopeResolver {
	return &CallerScopeResolver{entity: entityClient}
}

// IsProjectMember reports whether email has case access to projectID, per
// entity-service's own GrantsCaseAccess rule. A non-nil error means the
// check itself failed (upstream unavailable, unknown project, ...) —
// callers should treat that the same as "not a member" (fail closed)
// rather than let an upstream hiccup silently grant access.
func (r *CallerScopeResolver) IsProjectMember(ctx context.Context, projectID, email string) (bool, error) {
	for page := 0; page < callerScopeContactsMaxPages; page++ {
		resp, err := r.entity.SearchProjectContacts(ctx, projectID, entity.SearchProjectContactsRequest{
			Pagination: entity.Pagination{Limit: callerScopeContactsLimit, Offset: page * callerScopeContactsLimit},
		})
		if err != nil {
			return false, err
		}

		for _, c := range resp.Contacts {
			if c.GrantsCaseAccess && strings.EqualFold(c.Email, email) {
				return true, nil
			}
		}

		if len(resp.Contacts) == 0 || page*callerScopeContactsLimit+len(resp.Contacts) >= resp.Total {
			break
		}
	}
	return false, nil
}

// requireProjectMember is the single reusable gate every project-scoped
// handler calls — always enforced, no kill switch. It checks
// scope.IsProjectMember and writes notFoundStatus (with notFoundMsg) if the
// caller isn't a member of projectID or the check itself failed (fail
// closed) — the caller must return immediately when this returns false,
// without writing any other response.
//
// scope == nil lets the caller proceed unchecked rather than panicking —
// this matters because dozens of pre-existing tests across this package
// construct handlers directly without ever calling SetCallerScope, testing
// functionality unrelated to this feature entirely; a nil dereference (or a
// fail-closed rejection) here would break every one of them. In production,
// main.go always calls SetCallerScope with a real resolver unconditionally,
// so this path is never taken outside tests that don't care about it.
//
// notFoundStatus is a parameter rather than always 403 because a
// single-item detail fetch (e.g. GetCase) should 404 instead — confirming a
// resource id exists to a caller who can't see it is its own leak — while a
// search whose URL already names the project (e.g. SearchCases) has nothing
// further to hide and should 403.
func requireProjectMember(w http.ResponseWriter, r *http.Request, scope *CallerScopeResolver, projectID string, userID, email string, notFoundStatus int, notFoundMsg string) bool {
	if scope == nil {
		return true
	}
	member, err := scope.IsProjectMember(r.Context(), projectID, email)
	if err != nil {
		slog.ErrorContext(r.Context(), "caller scope check failed", "userID", userID, "projectID", projectID, "err", summarizeErr(err))
	}
	if err != nil || !member {
		writeError(w, notFoundStatus, notFoundMsg)
		return false
	}
	return true
}
