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
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// The dual-write create refuses what ServiceNow would answer with a bare 404: a
// person it has no account for (a user created in this database, such as a
// load-test user, has an id ServiceNow never issued) and a group it does not know.

const (
	// A person ServiceNow knows by this id (a synced user).
	crPeopleSyncedID = "e903eae2-ebd7-8b10-fcf5-f5dabad0cda1"
	// A person only this database knows: a random id, no ServiceNow record.
	crPeopleLocalID = "0047cedf-69e4-4698-841f-2e218886a4fd"
	// That same person's ServiceNow record, found by email.
	crPeopleLocalSNID = "11111111-2222-3333-4444-555555555555"
	crPeopleGroupID   = "f4a2d253-9747-5190-b20d-fe3bf253afb8"
)

// crPeopleSN is a ServiceNow user directory: byID answers a UserIDs search with the
// users it knows, byEmail an Emails search. Every call is recorded.
type crPeopleSN struct {
	SNUserService
	byID    map[string]domain.SNUser
	byEmail map[string][]domain.SNUser
	err     error // every search fails
	// emailErr fails only the searches by email.
	emailErr error
	// hang makes every search wait for its context, like a ServiceNow that never answers.
	hang    bool
	idCalls [][]string
	emails  []string
}

func (f *crPeopleSN) SearchUsers(ctx context.Context, req domain.SearchUsersRequest) (domain.SearchSNUsersResponse, error) {
	if f.hang {
		<-ctx.Done()
		return domain.SearchSNUsersResponse{}, ctx.Err()
	}
	if f.err != nil {
		return domain.SearchSNUsersResponse{}, f.err
	}
	if len(req.Filters.Emails) > 0 && f.emailErr != nil {
		return domain.SearchSNUsersResponse{}, f.emailErr
	}
	var out []domain.SNUser
	if len(req.Filters.UserIDs) > 0 {
		f.idCalls = append(f.idCalls, req.Filters.UserIDs)
		for _, id := range req.Filters.UserIDs {
			if u, ok := f.byID[id]; ok {
				out = append(out, u)
			}
		}
	}
	for _, e := range req.Filters.Emails {
		f.emails = append(f.emails, e)
		out = append(out, f.byEmail[e]...)
	}
	return domain.SearchSNUsersResponse{Users: out, Total: len(out)}, nil
}

// crPeopleUsers answers "user" rows by id for the email lookup.
type crPeopleUsers struct {
	stubUserRepo
	users map[string]domain.User
	err   error
}

func (r *crPeopleUsers) GetUsersByIDs(_ context.Context, ids []string) ([]domain.User, error) {
	if r.err != nil {
		return nil, r.err
	}
	var out []domain.User
	for _, id := range ids {
		if u, ok := r.users[id]; ok {
			out = append(out, u)
		}
	}
	return out, nil
}

// crPeopleHarness wires the dual-write service to a recording mirror and PostgreSQL repo.
type crPeopleHarness struct {
	svc        ChangeRequestService
	sn         *crPeopleSN
	toSN, toPG *domain.CreateChangeRequestRequest
	snCalls    int
	pgCalls    int
	links      []domain.ChangeRequestLinkSelection
	users      *crPeopleUsers
}

func newCRPeopleHarness(t *testing.T, sn *crPeopleSN, withLookup bool) *crPeopleHarness {
	t.Helper()
	h := &crPeopleHarness{sn: sn}
	mirror := &stubMirrorChangeRequestService{createChangeRequest: func(_ context.Context, r domain.CreateChangeRequestRequest) (domain.CreateChangeRequestResponse, error) {
		h.snCalls++
		h.toSN = &r
		resp := domain.CreateChangeRequestResponse{}
		resp.ChangeRequest.ID = testUUID
		return resp, nil
	}}
	repo := &stubChangeRequestRepo{
		validateChangeRequestLinks: func(_ context.Context, sel domain.ChangeRequestLinkSelection) (domain.ChangeRequestLinkSet, error) {
			h.links = append(h.links, sel)
			return domain.ChangeRequestLinkSet{}, nil
		},
		createChangeRequestFromServiceNow: func(_ context.Context, r domain.CreateChangeRequestRequest, id, _, _ string) (domain.CreateChangeRequestResponse, error) {
			h.pgCalls++
			h.toPG = &r
			resp := domain.CreateChangeRequestResponse{}
			resp.ChangeRequest.ID = id
			return resp, nil
		},
	}
	users := &crPeopleUsers{users: map[string]domain.User{
		crPeopleLocalID: {ID: crPeopleLocalID, FirstName: "Load", LastName: "User", Email: "load.user@example.com"},
	}}
	h.users = users
	svc := NewChangeRequestServiceWithSNMirror(repo, users, mirror)
	if withLookup {
		svc = WithChangeRequestSNUserLookup(svc, sn)
	}
	h.svc = svc
	return h
}

func TestChangeRequestCreate_PeopleServiceNowKnowsByIDAreSentAsTheyAre(t *testing.T) {
	sn := &crPeopleSN{byID: map[string]domain.SNUser{crPeopleSyncedID: {ID: crPeopleSyncedID, Active: true}}}
	h := newCRPeopleHarness(t, sn, true)
	req := validCreateChangeRequestRequest()
	req.AssignedEngineerID, req.RequestedByID = strp(crPeopleSyncedID), strp(crPeopleSyncedID)

	if _, err := h.svc.CreateChangeRequest(context.Background(), req); err != nil {
		t.Fatalf("create: %v", err)
	}
	if *h.toSN.AssignedEngineerID != crPeopleSyncedID || *h.toSN.RequestedByID != crPeopleSyncedID {
		t.Errorf("ServiceNow got %v / %v, want the ids unchanged", *h.toSN.AssignedEngineerID, *h.toSN.RequestedByID)
	}
	if len(sn.idCalls) != 1 || len(sn.idCalls[0]) != 1 {
		t.Errorf("id lookups = %v, want one search for the one distinct id", sn.idCalls)
	}
	if len(sn.emails) != 0 {
		t.Errorf("email lookups = %v, want none for a person ServiceNow already knew", sn.emails)
	}
}

func TestChangeRequestCreate_PersonOnlyThisDatabaseKnowsIsFoundByEmail(t *testing.T) {
	sn := &crPeopleSN{
		byID: map[string]domain.SNUser{crPeopleSyncedID: {ID: crPeopleSyncedID, Active: true}},
		byEmail: map[string][]domain.SNUser{"load.user@example.com": {
			{ID: "99999999-9999-9999-9999-999999999999", Email: "load.user@example.com", Active: false},
			{ID: crPeopleLocalSNID, Email: "Load.User@example.com", Active: true},
		}},
	}
	h := newCRPeopleHarness(t, sn, true)
	req := validCreateChangeRequestRequest()
	req.AssignedEngineerID, req.RequestedByID = strp(crPeopleLocalID), strp(crPeopleSyncedID)

	if _, err := h.svc.CreateChangeRequest(context.Background(), req); err != nil {
		t.Fatalf("create: %v", err)
	}
	if got := *h.toSN.AssignedEngineerID; got != crPeopleLocalSNID {
		t.Errorf("ServiceNow got assignee %s, want their ServiceNow id %s (the active record)", got, crPeopleLocalSNID)
	}
	if got := *h.toSN.RequestedByID; got != crPeopleSyncedID {
		t.Errorf("ServiceNow got requester %s, want %s unchanged", got, crPeopleSyncedID)
	}
	// PostgreSQL keeps the person this database knows: the foreign key is to "user".
	if got := *h.toPG.AssignedEngineerID; got != crPeopleLocalID {
		t.Errorf("PostgreSQL got assignee %s, want %s", got, crPeopleLocalID)
	}
	if got := *h.toPG.RequestedByID; got != crPeopleSyncedID {
		t.Errorf("PostgreSQL got requester %s, want %s", got, crPeopleSyncedID)
	}
	// ...and the caller's own request value is never rewritten through the copy.
	if *req.AssignedEngineerID != crPeopleLocalID || *req.RequestedByID != crPeopleSyncedID {
		t.Errorf("the caller's request was changed: %s / %s", *req.AssignedEngineerID, *req.RequestedByID)
	}
}

func TestChangeRequestCreate_PersonWithNoServiceNowAccountIsRefusedBeforeServiceNow(t *testing.T) {
	cases := map[string]*crPeopleSN{
		"no record by email":     {},
		"only a deactivated one": {byEmail: map[string][]domain.SNUser{"load.user@example.com": {{ID: crPeopleLocalSNID, Email: "load.user@example.com", Active: false}}}},
		"another person's email": {byEmail: map[string][]domain.SNUser{"load.user@example.com": {{ID: crPeopleLocalSNID, Email: "someone.else@wso2.com", Active: true}}}},
	}
	for name, sn := range cases {
		t.Run(name, func(t *testing.T) {
			h := newCRPeopleHarness(t, sn, true)
			req := validCreateChangeRequestRequest()
			req.AssignedEngineerID = strp(crPeopleLocalID)

			_, err := h.svc.CreateChangeRequest(context.Background(), req)
			var ve *apierror.ValidationError
			if !asValidationError(err, &ve) {
				t.Fatalf("err = %v, want a 400", err)
			}
			// Worded for the person on the form: who, why, and which field to change.
			for _, want := range []string{"Load User (load.user@example.com)", "no ServiceNow account", "cannot be assigned this change request", `"Assigned to"`} {
				if !strings.Contains(ve.Msg, want) {
					t.Errorf("message %q does not mention %q", ve.Msg, want)
				}
			}
			if strings.Contains(ve.Msg, "assignedEngineerId") || strings.Contains(ve.Msg, crPeopleLocalID) {
				t.Errorf("message %q shows a field name or an id", ve.Msg)
			}
			if h.snCalls != 0 || h.pgCalls != 0 {
				t.Errorf("ServiceNow calls = %d, PostgreSQL writes = %d, want none", h.snCalls, h.pgCalls)
			}
		})
	}
}

func TestChangeRequestCreate_OneLookupForAPersonNamedTwice(t *testing.T) {
	sn := &crPeopleSN{byEmail: map[string][]domain.SNUser{"load.user@example.com": {{ID: crPeopleLocalSNID, Email: "load.user@example.com", Active: true}}}}
	h := newCRPeopleHarness(t, sn, true)
	req := validCreateChangeRequestRequest()
	req.AssignedEngineerID, req.RequestedByID = strp(crPeopleLocalID), strp(crPeopleLocalID)

	if _, err := h.svc.CreateChangeRequest(context.Background(), req); err != nil {
		t.Fatalf("create: %v", err)
	}
	if *h.toSN.AssignedEngineerID != crPeopleLocalSNID || *h.toSN.RequestedByID != crPeopleLocalSNID {
		t.Errorf("ServiceNow got %v / %v, want both replaced", *h.toSN.AssignedEngineerID, *h.toSN.RequestedByID)
	}
	if *h.toPG.AssignedEngineerID != crPeopleLocalID || *h.toPG.RequestedByID != crPeopleLocalID {
		t.Errorf("PostgreSQL got %v / %v, want the caller's id for both", *h.toPG.AssignedEngineerID, *h.toPG.RequestedByID)
	}
	if len(sn.emails) != 1 {
		t.Errorf("email lookups = %v, want one for one person", sn.emails)
	}
}

func TestChangeRequestCreate_UnknownUserRowIsRefused(t *testing.T) {
	h := newCRPeopleHarness(t, &crPeopleSN{}, true)
	req := validCreateChangeRequestRequest()
	req.AssignedEngineerID = strp("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")

	_, err := h.svc.CreateChangeRequest(context.Background(), req)
	var ve *apierror.ValidationError
	if !asValidationError(err, &ve) || !strings.Contains(ve.Msg, `"Assigned to" no longer exists`) {
		t.Fatalf("err = %v, want a 400 about the person chosen in Assigned to", err)
	}
	if h.snCalls != 0 {
		t.Error("ServiceNow was called")
	}
}

func TestChangeRequestCreate_NoPeopleNoLookup(t *testing.T) {
	sn := &crPeopleSN{}
	h := newCRPeopleHarness(t, sn, true)
	if _, err := h.svc.CreateChangeRequest(context.Background(), validCreateChangeRequestRequest()); err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(sn.idCalls) != 0 || len(sn.emails) != 0 {
		t.Errorf("lookups = %v / %v, want none when nobody is named", sn.idCalls, sn.emails)
	}
}

// Without the lookup wired (every mode but dual-write, and any test that builds the
// service directly) nothing changes: the ids go to ServiceNow as they were.
func TestChangeRequestCreate_WithoutTheLookupIdsAreSentAsGiven(t *testing.T) {
	h := newCRPeopleHarness(t, &crPeopleSN{}, false)
	req := validCreateChangeRequestRequest()
	req.AssignedEngineerID = strp(crPeopleLocalID)

	if _, err := h.svc.CreateChangeRequest(context.Background(), req); err != nil {
		t.Fatalf("create: %v", err)
	}
	if *h.toSN.AssignedEngineerID != crPeopleLocalID {
		t.Errorf("ServiceNow got %s, want %s as given", *h.toSN.AssignedEngineerID, crPeopleLocalID)
	}
}

// A lookup that cannot answer is not "no account": the create goes on with the ids as
// they were, which is what it did before the check existed. Only a positive "ServiceNow
// has no such person" refuses.
func TestChangeRequestCreate_ALookupThatCannotAnswerLeavesTheIdsAsSent(t *testing.T) {
	boom := errors.New("upstream unavailable")

	t.Run("the check by id fails", func(t *testing.T) {
		h := newCRPeopleHarness(t, &crPeopleSN{err: boom}, true)
		req := validCreateChangeRequestRequest()
		req.AssignedEngineerID, req.RequestedByID = strp(crPeopleSyncedID), strp(crPeopleLocalID)

		if _, err := h.svc.CreateChangeRequest(context.Background(), req); err != nil {
			t.Fatalf("create: %v", err)
		}
		if *h.toSN.AssignedEngineerID != crPeopleSyncedID || *h.toSN.RequestedByID != crPeopleLocalID {
			t.Errorf("ServiceNow got %v / %v, want both ids as sent", *h.toSN.AssignedEngineerID, *h.toSN.RequestedByID)
		}
		if h.snCalls != 1 || h.pgCalls != 1 {
			t.Errorf("ServiceNow creates = %d, PostgreSQL writes = %d, want one each", h.snCalls, h.pgCalls)
		}
	})

	t.Run("the lookup by email fails", func(t *testing.T) {
		sn := &crPeopleSN{emailErr: boom}
		h := newCRPeopleHarness(t, sn, true)
		req := validCreateChangeRequestRequest()
		req.AssignedEngineerID = strp(crPeopleLocalID)

		if _, err := h.svc.CreateChangeRequest(context.Background(), req); err != nil {
			t.Fatalf("create: %v", err)
		}
		if *h.toSN.AssignedEngineerID != crPeopleLocalID {
			t.Errorf("ServiceNow got %v, want the id as sent", *h.toSN.AssignedEngineerID)
		}
	})

	t.Run("the user row cannot be read", func(t *testing.T) {
		h := newCRPeopleHarness(t, &crPeopleSN{}, true)
		h.users.err = boom
		req := validCreateChangeRequestRequest()
		req.AssignedEngineerID = strp(crPeopleLocalID)

		if _, err := h.svc.CreateChangeRequest(context.Background(), req); err != nil {
			t.Fatalf("create: %v", err)
		}
		if *h.toSN.AssignedEngineerID != crPeopleLocalID {
			t.Errorf("ServiceNow got %v, want the id as sent", *h.toSN.AssignedEngineerID)
		}
	})
}

// The assignment group goes through the same pre-flight as the project and
// deployments: checked before ServiceNow, and refused in words.
func TestChangeRequestCreate_AssignmentGroupIsValidatedBeforeServiceNow(t *testing.T) {
	h := newCRPeopleHarness(t, &crPeopleSN{}, true)
	req := validCreateChangeRequestRequest()
	req.GroupID = strp(crPeopleGroupID)
	if _, err := h.svc.CreateChangeRequest(context.Background(), req); err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(h.links) != 1 || h.links[0].AssignmentGroupID == nil || *h.links[0].AssignmentGroupID != crPeopleGroupID {
		t.Fatalf("pre-flight selection = %+v, want the group named", h.links)
	}

	// A refusal stops the create before ServiceNow, which would answer with a bare 404.
	refused := &stubChangeRequestRepo{validateChangeRequestLinks: func(context.Context, domain.ChangeRequestLinkSelection) (domain.ChangeRequestLinkSet, error) {
		return domain.ChangeRequestLinkSet{}, &apierror.ValidationError{Msg: "groupId does not refer to an assignment group"}
	}}
	mirror := &stubMirrorChangeRequestService{createChangeRequest: func(context.Context, domain.CreateChangeRequestRequest) (domain.CreateChangeRequestResponse, error) {
		t.Fatal("ServiceNow was called although the group is not an assignment group")
		return domain.CreateChangeRequestResponse{}, nil
	}}
	svc := NewChangeRequestServiceWithSNMirror(refused, stubUserRepo{}, mirror)
	_, err := svc.CreateChangeRequest(context.Background(), req)
	var ve *apierror.ValidationError
	if !asValidationError(err, &ve) || !strings.Contains(ve.Msg, "groupId") {
		t.Fatalf("err = %v, want a 400 naming groupId", err)
	}
}

func TestChangeRequestCreate_MalformedPersonIdNamesTheField(t *testing.T) {
	sn := &crPeopleSN{}
	h := newCRPeopleHarness(t, sn, true)
	req := validCreateChangeRequestRequest()
	req.RequestedByID = strp("not-a-uuid")

	_, err := h.svc.CreateChangeRequest(context.Background(), req)
	var ve *apierror.ValidationError
	if !asValidationError(err, &ve) || !strings.Contains(ve.Msg, "requestedById") {
		t.Fatalf("err = %v, want a 400 naming requestedById", err)
	}
	if len(sn.idCalls) != 0 || h.snCalls != 0 {
		t.Errorf("ServiceNow was reached: searches %v, creates %d", sn.idCalls, h.snCalls)
	}
}

// The requester is refused in its own words, naming its own field.
func TestChangeRequestCreate_RequesterWithNoServiceNowAccountNamesTheRequestedByField(t *testing.T) {
	h := newCRPeopleHarness(t, &crPeopleSN{}, true)
	req := validCreateChangeRequestRequest()
	req.RequestedByID = strp(crPeopleLocalID)

	_, err := h.svc.CreateChangeRequest(context.Background(), req)
	var ve *apierror.ValidationError
	if !asValidationError(err, &ve) {
		t.Fatalf("err = %v, want a 400", err)
	}
	for _, want := range []string{"Load User", "the requester of this change request", `"Requested by"`} {
		if !strings.Contains(ve.Msg, want) {
			t.Errorf("message %q does not mention %q", ve.Msg, want)
		}
	}
}

// ServiceNow answering "not found" to a create can only mean a record it was handed is
// not one it knows. The portal would show that as "The requested resource was not
// found!", so it is reported as a 400 saying what is likely wrong; other failures pass
// through unchanged, and nothing is written to PostgreSQL either way.
func TestChangeRequestCreate_ServiceNowNotFoundIsReportedInWords(t *testing.T) {
	create := func(snErr error) (*crPeopleHarness, error) {
		h := newCRPeopleHarness(t, &crPeopleSN{}, true)
		// Replace the recording mirror with one that fails.
		repo := &stubChangeRequestRepo{createChangeRequestFromServiceNow: func(context.Context, domain.CreateChangeRequestRequest, string, string, string) (domain.CreateChangeRequestResponse, error) {
			h.pgCalls++
			return domain.CreateChangeRequestResponse{}, nil
		}}
		mirror := &stubMirrorChangeRequestService{createChangeRequest: func(context.Context, domain.CreateChangeRequestRequest) (domain.CreateChangeRequestResponse, error) {
			h.snCalls++
			return domain.CreateChangeRequestResponse{}, snErr
		}}
		h.svc = NewChangeRequestServiceWithSNMirror(repo, stubUserRepo{}, mirror)
		_, err := h.svc.CreateChangeRequest(context.Background(), validCreateChangeRequestRequest())
		return h, err
	}

	t.Run("not found", func(t *testing.T) {
		h, err := create(&apierror.NotFoundError{Msg: "not found"})
		var ve *apierror.ValidationError
		if !asValidationError(err, &ve) {
			t.Fatalf("err = %v (%T), want a 400", err, err)
		}
		for _, want := range []string{"was not created", "did not recognise", "assignment group", "person it is assigned to", "service offering"} {
			if !strings.Contains(ve.Msg, want) {
				t.Errorf("message %q does not mention %q", ve.Msg, want)
			}
		}
		if h.snCalls != 1 || h.pgCalls != 0 {
			t.Errorf("ServiceNow creates = %d, PostgreSQL writes = %d, want 1 and 0", h.snCalls, h.pgCalls)
		}
	})

	t.Run("any other failure is passed through", func(t *testing.T) {
		boom := errors.New("upstream unavailable")
		_, err := create(boom)
		if !errors.Is(err, boom) {
			t.Fatalf("err = %v, want the upstream error unchanged", err)
		}
		var ve *apierror.ValidationError
		if asValidationError(err, &ve) {
			t.Fatalf("a non-404 failure was turned into a 400: %v", err)
		}
	})
}

// The ServiceNow client reports an upstream 400 as a ValidationError, the very type a
// refusal is. It is "could not tell", not "no such person": the create goes on with the
// ids as sent.
func TestChangeRequestCreate_AnUpstreamBadRequestIsNotARefusal(t *testing.T) {
	upstream400 := &apierror.ValidationError{Msg: "downstream service rejected the request"}

	t.Run("on the check by id", func(t *testing.T) {
		h := newCRPeopleHarness(t, &crPeopleSN{err: upstream400}, true)
		req := validCreateChangeRequestRequest()
		req.AssignedEngineerID = strp(crPeopleLocalID)
		if _, err := h.svc.CreateChangeRequest(context.Background(), req); err != nil {
			t.Fatalf("create: %v", err)
		}
		if *h.toSN.AssignedEngineerID != crPeopleLocalID {
			t.Errorf("ServiceNow got %v, want the id as sent", *h.toSN.AssignedEngineerID)
		}
	})

	t.Run("on the lookup by email", func(t *testing.T) {
		h := newCRPeopleHarness(t, &crPeopleSN{emailErr: upstream400}, true)
		req := validCreateChangeRequestRequest()
		req.AssignedEngineerID = strp(crPeopleLocalID)
		if _, err := h.svc.CreateChangeRequest(context.Background(), req); err != nil {
			t.Fatalf("create refused on an upstream 400: %v", err)
		}
		if *h.toSN.AssignedEngineerID != crPeopleLocalID {
			t.Errorf("ServiceNow got %v, want the id as sent", *h.toSN.AssignedEngineerID)
		}
	})
}

// A lookup that cannot answer leaves the id exactly as the caller wrote it: not trimmed,
// not lower-cased.
func TestChangeRequestCreate_AnUnresolvedIdIsSentExactlyAsGiven(t *testing.T) {
	h := newCRPeopleHarness(t, &crPeopleSN{emailErr: errors.New("upstream unavailable")}, true)
	h.users.users[strings.ToUpper(crPeopleLocalID)] = domain.User{ID: crPeopleLocalID, Email: "load.user@example.com"}
	upper := strings.ToUpper(crPeopleLocalID)
	req := validCreateChangeRequestRequest()
	req.AssignedEngineerID = &upper

	if _, err := h.svc.CreateChangeRequest(context.Background(), req); err != nil {
		t.Fatalf("create: %v", err)
	}
	if got := *h.toSN.AssignedEngineerID; got != upper {
		t.Errorf("ServiceNow got %q, want %q exactly as given", got, upper)
	}
}

// A padded id is refused before ServiceNow, as the create always refused it: it must
// never reach ServiceNow clean while PostgreSQL gets the padded text.
func TestChangeRequestCreate_APaddedPersonIdIsRefusedBeforeServiceNow(t *testing.T) {
	sn := &crPeopleSN{byEmail: map[string][]domain.SNUser{"load.user@example.com": {{ID: crPeopleLocalSNID, Email: "load.user@example.com", Active: true}}}}
	h := newCRPeopleHarness(t, sn, true)
	req := validCreateChangeRequestRequest()
	req.AssignedEngineerID = strp(" " + crPeopleLocalID + " ")

	_, err := h.svc.CreateChangeRequest(context.Background(), req)
	var ve *apierror.ValidationError
	if !asValidationError(err, &ve) || !strings.Contains(ve.Msg, "assignedEngineerId") {
		t.Fatalf("err = %v, want a 400 naming assignedEngineerId", err)
	}
	if h.snCalls != 0 || h.pgCalls != 0 || len(sn.idCalls) != 0 {
		t.Errorf("ServiceNow creates = %d, PostgreSQL writes = %d, searches = %v, want none", h.snCalls, h.pgCalls, sn.idCalls)
	}
}

// A ServiceNow that never answers costs the create a bounded wait, not its whole
// deadline, and the create then goes on as it always did.
func TestChangeRequestCreate_ASlowServiceNowLookupIsBounded(t *testing.T) {
	old := snPeopleLookupTimeout
	snPeopleLookupTimeout = 50 * time.Millisecond
	t.Cleanup(func() { snPeopleLookupTimeout = old })

	h := newCRPeopleHarness(t, &crPeopleSN{hang: true}, true)
	req := validCreateChangeRequestRequest()
	req.AssignedEngineerID = strp(crPeopleLocalID)

	// The request itself has plenty of time.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	start := time.Now()
	if _, err := h.svc.CreateChangeRequest(ctx, req); err != nil {
		t.Fatalf("create: %v", err)
	}
	if took := time.Since(start); took > 5*time.Second {
		t.Errorf("create took %v: the lookup was not bounded", took)
	}
	if *h.toSN.AssignedEngineerID != crPeopleLocalID || h.snCalls != 1 {
		t.Errorf("ServiceNow got %v (creates %d), want the id as sent and one create", *h.toSN.AssignedEngineerID, h.snCalls)
	}
}
