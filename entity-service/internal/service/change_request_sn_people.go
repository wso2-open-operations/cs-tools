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
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// snUserSearcher is the one ServiceNow user operation change request creation
// needs. SNUserService satisfies it.
type snUserSearcher interface {
	SearchUsers(ctx context.Context, req domain.SearchUsersRequest) (domain.SearchSNUsersResponse, error)
}

// WithChangeRequestSNUserLookup gives a dual-write ChangeRequestService the
// ServiceNow user lookup its create path uses to check the people a change is
// assigned to (see resolveServiceNowPeople). It is a post-construction step, like
// WithProductCategoryEnforcement, so the existing constructors and the tests that
// call them are untouched. With no lookup the create path behaves as it did: the ids
// are sent to ServiceNow as they are.
//
// svc must be what NewChangeRequestServiceWithSNWriteback or
// NewChangeRequestServiceWithSNMirror returned; any other value is returned unchanged.
func WithChangeRequestSNUserLookup(svc ChangeRequestService, users SNUserService) ChangeRequestService {
	if s, ok := svc.(*changeRequestService); ok && users != nil {
		s.snUsers = users
	}
	return svc
}

// snPersonField is one person-valued field of a create request. label is what the
// form calls it, and as is how the person is described in a refusal.
type snPersonField struct {
	name  string
	label string
	as    string
	id    **string
}

// snPeopleLookupTimeout (a variable so a test can shorten it) bounds the ServiceNow user lookups of a create, together. They run
// before the create on the request's own deadline, so without a bound of their own a slow
// user search would eat the time the create itself needs; with one, a slow search is a
// lookup that "could not answer" and the create goes on as it always did.
var snPeopleLookupTimeout = 8 * time.Second

// resolveServiceNowPeople makes the people named on a change request creation
// ones ServiceNow knows, before ServiceNow is called. It rewrites the ids in
// mirrorReq (the copy that goes to ServiceNow; the Postgres insert keeps the
// caller's own ids).
//
// Why: a person's id in "user" is their ServiceNow sys_id only when the row was
// synced from there. A user created in this database (a load-test user, someone added
// through POST /users) has a random id ServiceNow has never seen, and ServiceNow
// answers a create that names one with a bare 404 that reaches the form as "The
// requested resource was not found!", with nothing saying which field is at fault.
//
// For each distinct assignedEngineerId / requestedById:
//  1. ServiceNow knows the id (one search for all of them, which also finds a
//     deactivated user, since an id lookup lifts the active-only default): kept.
//  2. Otherwise the person is looked up in "user" and then in ServiceNow by email:
//     found, their ServiceNow id is sent instead; not found, a 400 says who cannot be
//     used and which field to change.
//
// A person ServiceNow already knew by id is never re-judged by email, so nothing that
// worked before can start failing here. Neither does a lookup that fails or cannot
// answer (ServiceNow slow, down or answering an error, the "user" row unreadable):
// then nothing is known, so that id goes to ServiceNow exactly as the caller sent it,
// as before this check existed, and only a positive "ServiceNow has no such person"
// refuses the create. That is why a refusal is its own return value below and not an
// error: the ServiceNow client reports an upstream 400 as a ValidationError too, which
// must not be mistaken for one.
func (s *changeRequestService) resolveServiceNowPeople(ctx context.Context, mirrorReq *domain.CreateChangeRequestRequest) error {
	if s.snUsers == nil {
		return nil
	}
	fields := []snPersonField{
		{name: "assignedEngineerId", label: "Assigned to", as: "assigned this change request", id: &mirrorReq.AssignedEngineerID},
		{name: "requestedById", label: "Requested by", as: "the requester of this change request", id: &mirrorReq.RequestedByID},
	}

	var ids []string
	seen := map[string]bool{}
	for _, f := range fields {
		if *f.id == nil || strings.TrimSpace(**f.id) == "" {
			continue
		}
		// The value as the caller sent it, named here (the user search would say
		// "userIds"): a padded or malformed id is refused as the create always refused it,
		// so the id that reaches ServiceNow is never one PostgreSQL would then reject.
		if err := validateUUIDs(f.name, []string{**f.id}); err != nil {
			return err
		}
		id := strings.ToLower(**f.id)
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil
	}

	lookupCtx, cancel := context.WithTimeout(ctx, snPeopleLookupTimeout)
	defer cancel()

	known, err := s.snUsers.SearchUsers(lookupCtx, domain.SearchUsersRequest{
		Filters:    domain.SearchUsersFilters{UserIDs: ids},
		Pagination: domain.Pagination{Limit: len(ids)},
	})
	if err != nil {
		slog.WarnContext(ctx, "change request create: could not check the people against ServiceNow, sending the ids as given", "error", err)
		return nil
	}
	inServiceNow := make(map[string]bool, len(known.Users))
	for _, u := range known.Users {
		inServiceNow[strings.ToLower(u.ID)] = true
	}

	// resolved maps a person ServiceNow does not know by id to their ServiceNow id, or
	// to "" when that could not be found out (the id is then left as the caller sent it).
	resolved := map[string]string{}
	for _, f := range fields {
		if *f.id == nil || strings.TrimSpace(**f.id) == "" {
			continue
		}
		id := strings.ToLower(**f.id)
		if inServiceNow[id] {
			continue
		}
		snID, ok := resolved[id]
		if !ok {
			var refusal *apierror.ValidationError
			snID, refusal, err = s.serviceNowIDForUser(lookupCtx, f, id)
			switch {
			case refusal != nil:
				return refusal
			case err != nil:
				slog.WarnContext(ctx, "change request create: could not resolve the person in ServiceNow, sending the id as given",
					"field", f.name, "userId", id, "error", err)
				snID = ""
			default:
				slog.InfoContext(ctx, "change request create: person is not a ServiceNow id, resolved by email",
					"field", f.name, "userId", id, "serviceNowUserId", snID)
			}
			resolved[id] = snID
		}
		if snID != "" {
			v := snID
			*f.id = &v
		}
	}
	return nil
}

// serviceNowIDForUser finds the ServiceNow user for the "user" row id: by that
// row's email, active users only (the default of an email search). It returns exactly one of:
//   - the ServiceNow id;
//   - a refusal, worded for the person on the form (who cannot be used, why, which field
//     to change), when the answer is positively "no": there is no such row, it has no
//     email, or ServiceNow has no active user with that email;
//   - an error, when that could not be found out.
func (s *changeRequestService) serviceNowIDForUser(ctx context.Context, f snPersonField, id string) (string, *apierror.ValidationError, error) {
	rows, err := s.userRepo.GetUsersByIDs(ctx, []string{id})
	if err != nil {
		return "", nil, err
	}
	if len(rows) == 0 {
		return "", &apierror.ValidationError{Msg: fmt.Sprintf(
			"The person chosen in \"%s\" no longer exists, so the change request cannot be created. Choose someone else.", f.label)}, nil
	}
	u := rows[0]
	who := strings.TrimSpace(strings.TrimSpace(u.FirstName) + " " + strings.TrimSpace(u.LastName))
	if who == "" {
		who = u.Email
	}
	if e := strings.TrimSpace(u.Email); e != "" && who != e {
		who += " (" + e + ")"
	}
	noAccount := &apierror.ValidationError{Msg: fmt.Sprintf(
		"The change request was not created: %s has no ServiceNow account, so they cannot be %s. "+
			"A change request is created in ServiceNow first, and ServiceNow does not know them. "+
			"Choose someone else in \"%s\".", who, f.as, f.label)}
	email := strings.TrimSpace(u.Email)
	if email == "" {
		return "", noAccount, nil
	}

	found, err := s.snUsers.SearchUsers(ctx, domain.SearchUsersRequest{
		Filters:    domain.SearchUsersFilters{Emails: []string{email}},
		Pagination: domain.Pagination{Limit: 5},
	})
	if err != nil {
		return "", nil, err
	}
	for _, c := range found.Users {
		if strings.EqualFold(strings.TrimSpace(c.Email), email) && c.Active && c.ID != "" {
			return strings.ToLower(c.ID), nil, nil
		}
	}
	return "", noAccount, nil
}
