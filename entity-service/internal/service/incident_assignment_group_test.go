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
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

const (
	testSupportGroup   = "5aaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	testOtherGroup     = "5bbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	testDefaultGroup   = "5ccccccc-cccc-4ccc-8ccc-cccccccccccc"
	testDefaultService = "5ddddddd-dddd-4ddd-8ddd-dddddddddddd"
)

func strPtrGroup(s string) *string { return &s }

// createCapturing returns a stub repo with the given support groups that
// records the request the native create receives.
func createCapturing(groups map[string]string, got *domain.CreateIncidentRequest) *stubIncidentRepo {
	return &stubIncidentRepo{
		supportGroups: groups,
		// The publish after create reads the incident back for the call
		// ladder's routing fields. That read is best-effort; answering "not
		// found" takes its failure path (publish without them) instead of
		// the stub's "not implemented" panic. Nothing here asserts on it.
		getIncidentByID: func(context.Context, string) (domain.IncidentView, error) {
			return domain.IncidentView{}, &apierror.NotFoundError{Msg: "incident not found"}
		},
		createIncident: func(_ context.Context, req domain.CreateIncidentRequest, _ string, _ *string, _ string) (domain.CreateIncidentResponse, error) {
			*got = req
			resp := domain.CreateIncidentResponse{}
			resp.Incident.ID = "66666666-6666-6666-6666-666666666666"
			return resp, nil
		},
	}
}

// refusingCreate is a stub repo whose create fails the test if reached.
func refusingCreate(t *testing.T, repo *stubIncidentRepo) *stubIncidentRepo {
	t.Helper()
	repo.createIncident = func(context.Context, domain.CreateIncidentRequest, string, *string, string) (domain.CreateIncidentResponse, error) {
		t.Fatal("the incident was created; the request should have been refused first")
		return domain.CreateIncidentResponse{}, nil
	}
	return repo
}

func userCtx() context.Context {
	return auth.WithIdentity(context.Background(), auth.Identity{Validated: true, UserEmail: "jane.doe@example.com"})
}

// groupOf is the request's assignment group, or "" when it has none.
func groupOf(r domain.CreateIncidentRequest) string {
	if r.AssignmentGroupID == nil {
		return "<none>"
	}
	return *r.AssignmentGroupID
}

// workNotesOf is the request's work notes, or "" when it has none.
func workNotesOf(r domain.CreateIncidentRequest) string {
	if r.WorkNotes == nil {
		return ""
	}
	return *r.WorkNotes
}

// pgService is the plain Postgres incident service over repo, with the
// default service set.
func pgService(repo *stubIncidentRepo, defaultService string) IncidentService {
	return WithIncidentDefaultService(NewIncidentService(repo, &mockEventPublisher{}), defaultService)
}

// Rule 2. *** ONE CALL CARRIES EVERYTHING. *** A caller that names only the
// service -- an alert-born incident, any M2M client -- gets the service's
// support group, and the work note says so.
func TestCreateIncident_NoGroupSentTakesTheServicesSupportGroup(t *testing.T) {
	var got domain.CreateIncidentRequest
	req := validCreateIncidentRequest()
	repo := createCapturing(map[string]string{req.ServiceID: testSupportGroup}, &got)
	repo.serviceNames = map[string]string{req.ServiceID: "Choreo"}

	if _, err := pgService(repo, testDefaultService).CreateIncident(userCtx(), req); err != nil {
		t.Fatalf("CreateIncident: %v", err)
	}
	if groupOf(got) != testSupportGroup {
		t.Errorf("assignmentGroupId = %s, want the service's support group %s", groupOf(got), testSupportGroup)
	}
	if want := "Assignment group set from service Choreo's support group"; workNotesOf(got) != want {
		t.Errorf("work notes = %q, want %q", workNotesOf(got), want)
	}
}

// Rule 3: a service with no support group goes to the default service's
// support group, with a warning that names the service.
func TestCreateIncident_ServiceWithoutGroupGoesToTheDefaultTeam(t *testing.T) {
	logs := captureSlog(t)
	var got domain.CreateIncidentRequest
	req := validCreateIncidentRequest()
	repo := createCapturing(map[string]string{req.ServiceID: "", testDefaultService: testDefaultGroup}, &got)
	repo.serviceNames = map[string]string{req.ServiceID: "Billing"}
	repo.groupNames = map[string]string{testDefaultGroup: "Default Team"}

	if _, err := pgService(repo, testDefaultService).CreateIncident(userCtx(), req); err != nil {
		t.Fatalf("CreateIncident: %v", err)
	}
	if groupOf(got) != testDefaultGroup {
		t.Errorf("assignmentGroupId = %s, want the default team %s", groupOf(got), testDefaultGroup)
	}
	if want := "Service Billing has no support group; assigned to the default team (Default Team)"; workNotesOf(got) != want {
		t.Errorf("work notes = %q, want %q", workNotesOf(got), want)
	}
	out := logs.String()
	if !strings.Contains(out, "level=WARN") || !strings.Contains(out, req.ServiceID) || !strings.Contains(out, "Billing") {
		t.Errorf("want a warning naming the service, got: %s", out)
	}
}

// A service id no service has is treated like a service with no group; the
// note then names it by id.
func TestCreateIncident_UnknownServiceGoesToTheDefaultTeam(t *testing.T) {
	_ = captureSlog(t)
	var got domain.CreateIncidentRequest
	req := validCreateIncidentRequest()
	repo := createCapturing(map[string]string{testDefaultService: testDefaultGroup}, &got)

	if _, err := pgService(repo, testDefaultService).CreateIncident(userCtx(), req); err != nil {
		t.Fatalf("CreateIncident: %v", err)
	}
	if groupOf(got) != testDefaultGroup {
		t.Errorf("assignmentGroupId = %s, want %s", groupOf(got), testDefaultGroup)
	}
	if want := "Service " + req.ServiceID + " has no support group; assigned to the default team (" + testDefaultGroup + ")"; workNotesOf(got) != want {
		t.Errorf("work notes = %q, want %q", workNotesOf(got), want)
	}
}

// Rule 4: no default team to fall back to is a misconfiguration, logged as an
// error -- the incident is still created, unassigned, with no note.
func TestCreateIncident_NoDefaultTeamLeavesItUnassignedAndLogsAnError(t *testing.T) {
	for name, tc := range map[string]struct {
		defaultService string
		groups         map[string]string
	}{
		"default unset":                {defaultService: "", groups: map[string]string{testCaseUUID: ""}},
		"default service missing":      {defaultService: testDefaultService, groups: map[string]string{testCaseUUID: ""}},
		"default service has no group": {defaultService: testDefaultService, groups: map[string]string{testCaseUUID: "", testDefaultService: ""}},
		"the service is the default":   {defaultService: testCaseUUID, groups: map[string]string{testCaseUUID: ""}},
	} {
		t.Run(name, func(t *testing.T) {
			logs := captureSlog(t)
			var got domain.CreateIncidentRequest
			req := validCreateIncidentRequest()
			req.AssignmentGroupID = nil
			if _, err := pgService(createCapturing(tc.groups, &got), tc.defaultService).CreateIncident(userCtx(), req); err != nil {
				t.Fatalf("CreateIncident: %v", err)
			}
			if got.AssignmentGroupID != nil {
				t.Errorf("assignmentGroupId = %s, want none", *got.AssignmentGroupID)
			}
			if got.WorkNotes != nil {
				t.Errorf("work notes = %q, want none", *got.WorkNotes)
			}
			if !strings.Contains(logs.String(), "level=ERROR") {
				t.Errorf("want an error log, got: %s", logs.String())
			}
		})
	}
}

// Rule 1: a group the caller sends is used when it is the support group of a
// service, and the note names who chose it -- a user by email, a machine
// caller by client id.
func TestCreateIncident_AnAllowedGroupIsUsedAndAttributed(t *testing.T) {
	for name, tc := range map[string]struct {
		ctx   context.Context
		actor string
	}{
		"user":    {userCtx(), "jane.doe@example.com"},
		"machine": {auth.WithIdentity(context.Background(), auth.Identity{Validated: true, ClientID: "sre-alert-core"}), "sre-alert-core"},
	} {
		t.Run(name, func(t *testing.T) {
			var got domain.CreateIncidentRequest
			req := validCreateIncidentRequest()
			req.AssignmentGroupID = strPtrGroup(testOtherGroup)
			repo := createCapturing(map[string]string{req.ServiceID: testSupportGroup, "99999999-9999-4999-8999-999999999999": testOtherGroup}, &got)

			if _, err := pgService(repo, testDefaultService).CreateIncident(tc.ctx, req); err != nil {
				t.Fatalf("CreateIncident: %v", err)
			}
			if groupOf(got) != testOtherGroup {
				t.Errorf("assignmentGroupId = %s, want the sent group %s", groupOf(got), testOtherGroup)
			}
			if want := "Assignment group chosen by " + tc.actor; workNotesOf(got) != want {
				t.Errorf("work notes = %q, want %q", workNotesOf(got), want)
			}
		})
	}
}

// Sending the service's own support group is allowed and lands the same group.
func TestCreateIncident_SendingTheServicesOwnGroupLandsThatGroup(t *testing.T) {
	var got domain.CreateIncidentRequest
	req := validCreateIncidentRequest()
	req.AssignmentGroupID = strPtrGroup(testSupportGroup)

	if _, err := pgService(createCapturing(map[string]string{req.ServiceID: testSupportGroup}, &got), "").CreateIncident(userCtx(), req); err != nil {
		t.Fatalf("CreateIncident: %v", err)
	}
	if groupOf(got) != testSupportGroup {
		t.Errorf("assignmentGroupId = %s, want %s", groupOf(got), testSupportGroup)
	}
}

// A blank assignmentGroupId counts as not sent.
func TestCreateIncident_ABlankGroupIsNotSent(t *testing.T) {
	var got domain.CreateIncidentRequest
	req := validCreateIncidentRequest()
	req.AssignmentGroupID = strPtrGroup("  ")

	if _, err := pgService(createCapturing(map[string]string{req.ServiceID: testSupportGroup}, &got), "").CreateIncident(userCtx(), req); err != nil {
		t.Fatalf("CreateIncident: %v", err)
	}
	if groupOf(got) != testSupportGroup {
		t.Errorf("assignmentGroupId = %s, want the service's group %s", groupOf(got), testSupportGroup)
	}
}

// Rule 1's refusals: a group outside the set (unknown, supporting no service,
// or inactive) and a value that is not a UUID are 400s, and nothing is created.
func TestCreateIncident_AGroupOutsideTheSetIsRefused(t *testing.T) {
	for name, tc := range map[string]struct {
		sent     string
		allowed  map[string]bool
		wantMsg  string
		wantCode string
	}{
		"supports no service": {sent: testOtherGroup, wantMsg: errAssignmentGroupNotAllowed, wantCode: apierror.CodeIncidentAssignmentGroupNotAllowed},
		"inactive":            {sent: testSupportGroup, allowed: map[string]bool{}, wantMsg: errAssignmentGroupNotAllowed, wantCode: apierror.CodeIncidentAssignmentGroupNotAllowed},
		"not a UUID":          {sent: "not-a-uuid", wantMsg: "assignmentGroupId"},
	} {
		t.Run(name, func(t *testing.T) {
			req := validCreateIncidentRequest()
			req.AssignmentGroupID = strPtrGroup(tc.sent)
			repo := refusingCreate(t, &stubIncidentRepo{supportGroups: map[string]string{req.ServiceID: testSupportGroup}, allowedGroups: tc.allowed})

			_, err := pgService(repo, testDefaultService).CreateIncident(userCtx(), req)
			var ve *apierror.ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("err = %v, want a ValidationError (400)", err)
			}
			if !strings.Contains(ve.Msg, tc.wantMsg) {
				t.Errorf("message = %q, want it to contain %q", ve.Msg, tc.wantMsg)
			}
			if ve.Code != tc.wantCode {
				t.Errorf("code = %q, want %q", ve.Code, tc.wantCode)
			}
		})
	}
}

// Rule 5: a lookup that fails fails the create; the incident is never created
// unassigned instead.
func TestCreateIncident_ALookupFailureCreatesNothing(t *testing.T) {
	boom := errors.New("database is down")
	for name, sent := range map[string]*string{"group sent": strPtrGroup(testSupportGroup), "no group sent": nil} {
		t.Run(name, func(t *testing.T) {
			req := validCreateIncidentRequest()
			req.AssignmentGroupID = sent
			repo := refusingCreate(t, &stubIncidentRepo{lookupErr: boom})
			if _, err := pgService(repo, testDefaultService).CreateIncident(userCtx(), req); !errors.Is(err, boom) {
				t.Errorf("err = %v, want the lookup error", err)
			}
		})
	}
}

// The caller's own work notes are kept as sent; ours follows in a paragraph
// of its own.
func TestCreateIncident_TheNoteFollowsTheCallersWorkNotes(t *testing.T) {
	var got domain.CreateIncidentRequest
	req := validCreateIncidentRequest()
	req.WorkNotes = strPtrGroup("Alert fired at 10:00.\nSee the runbook.")
	repo := createCapturing(map[string]string{req.ServiceID: testSupportGroup}, &got)
	repo.serviceNames = map[string]string{req.ServiceID: "Choreo"}

	if _, err := pgService(repo, "").CreateIncident(userCtx(), req); err != nil {
		t.Fatalf("CreateIncident: %v", err)
	}
	want := "Alert fired at 10:00.\nSee the runbook.\n\nAssignment group set from service Choreo's support group"
	if workNotesOf(got) != want {
		t.Errorf("work notes = %q, want %q", workNotesOf(got), want)
	}
}

// The body may carry assignmentGroupId again; other unknown fields are still
// refused by the handler's decoder.
func TestCreateIncidentRequest_BodyCarriesAGroup(t *testing.T) {
	var req domain.CreateIncidentRequest
	if err := json.Unmarshal([]byte(`{"assignmentGroupId":"`+testOtherGroup+`"}`), &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if req.AssignmentGroupID == nil || *req.AssignmentGroupID != testOtherGroup {
		t.Errorf("assignmentGroupId = %v, want %s", req.AssignmentGroupID, testOtherGroup)
	}
}

// dualWrite builds the dual-write incident service over repo, recording what
// the ServiceNow mirror receives (and failing the test if refuse is set and
// the mirror is reached).
func dualWrite(t *testing.T, repo *stubIncidentRepo, defaultService string, toSN *domain.CreateIncidentRequest, refuse bool) IncidentService {
	t.Helper()
	mirror := &stubMirrorIncidentService{
		createIncident: func(_ context.Context, r domain.CreateIncidentRequest) (domain.CreateIncidentResponse, error) {
			if refuse {
				t.Fatal("ServiceNow was called; the request should have been refused first")
			}
			*toSN = r
			resp := domain.CreateIncidentResponse{}
			resp.Incident.ID, resp.Incident.Number, resp.Incident.CreatedBy = "77777777-7777-7777-7777-777777777777", "INC0000001", "jane.doe@example.com"
			return resp, nil
		},
	}
	return WithIncidentDefaultService(NewIncidentServiceWithSNMirror(repo, nil, mirror, nil, nil), defaultService)
}

// In dual-write mode the group is decided once, on Postgres, and ServiceNow
// and the Postgres row get the same group -- whichever rule chose it -- and
// the same note.
func TestCreateIncident_DualWriteSendsTheSameGroupToBothStores(t *testing.T) {
	for name, tc := range map[string]struct {
		sent      *string
		groups    map[string]string
		wantGroup string
	}{
		"from the service":     {groups: map[string]string{testCaseUUID: testSupportGroup}, wantGroup: testSupportGroup},
		"from the default":     {groups: map[string]string{testCaseUUID: "", testDefaultService: testDefaultGroup}, wantGroup: testDefaultGroup},
		"chosen by the caller": {sent: strPtrGroup(testOtherGroup), groups: map[string]string{testCaseUUID: testSupportGroup, testDefaultService: testOtherGroup}, wantGroup: testOtherGroup},
	} {
		t.Run(name, func(t *testing.T) {
			_ = captureSlog(t)
			req := validCreateIncidentRequest()
			req.AssignmentGroupID = tc.sent
			var toSN, toPG domain.CreateIncidentRequest
			repo := &stubIncidentRepo{
				supportGroups: tc.groups,
				createIncidentFromServiceNow: func(_ context.Context, r domain.CreateIncidentRequest, id, _, _ string) (domain.CreateIncidentResponse, error) {
					toPG = r
					resp := domain.CreateIncidentResponse{}
					resp.Incident.ID = id
					return resp, nil
				},
			}
			if _, err := dualWrite(t, repo, testDefaultService, &toSN, false).CreateIncident(userCtx(), req); err != nil {
				t.Fatalf("CreateIncident: %v", err)
			}
			if groupOf(toSN) != tc.wantGroup || groupOf(toPG) != tc.wantGroup {
				t.Errorf("ServiceNow got %s, Postgres got %s, want both %s", groupOf(toSN), groupOf(toPG), tc.wantGroup)
			}
			if workNotesOf(toSN) == "" || workNotesOf(toSN) != workNotesOf(toPG) {
				t.Errorf("work notes: ServiceNow %q, Postgres %q, want the same note", workNotesOf(toSN), workNotesOf(toPG))
			}
		})
	}
}

// In dual-write mode a refused group, or a failed lookup, never reaches
// ServiceNow.
func TestCreateIncident_DualWriteRefusesBeforeServiceNow(t *testing.T) {
	for name, repo := range map[string]*stubIncidentRepo{
		"group outside the set": {supportGroups: map[string]string{testCaseUUID: testSupportGroup}},
		"lookup failure":        {lookupErr: errors.New("database is down")},
	} {
		t.Run(name, func(t *testing.T) {
			req := validCreateIncidentRequest()
			req.AssignmentGroupID = strPtrGroup(testOtherGroup)
			var toSN domain.CreateIncidentRequest
			if _, err := dualWrite(t, repo, testDefaultService, &toSN, true).CreateIncident(userCtx(), req); err == nil {
				t.Fatal("CreateIncident succeeded, want a refusal")
			}
		})
	}
}

// GET /incidents/create-defaults: the default service and its support group.
func TestGetIncidentCreateDefaults(t *testing.T) {
	for name, tc := range map[string]struct {
		defaultService string
		repo           *stubIncidentRepo
		want           string
		wantErr        bool
	}{
		"unset": {repo: &stubIncidentRepo{}, want: `{"defaultServiceId":null,"defaultGroup":null}`},
		"with a group": {defaultService: testDefaultService,
			repo: &stubIncidentRepo{supportGroups: map[string]string{testDefaultService: testDefaultGroup}, groupNames: map[string]string{testDefaultGroup: "Default Team"}},
			want: `{"defaultServiceId":"` + testDefaultService + `","defaultGroup":{"id":"` + testDefaultGroup + `","name":"Default Team"}}`},
		"groupless":       {defaultService: testDefaultService, repo: &stubIncidentRepo{supportGroups: map[string]string{testDefaultService: ""}}, want: `{"defaultServiceId":"` + testDefaultService + `","defaultGroup":null}`},
		"missing service": {defaultService: testDefaultService, repo: &stubIncidentRepo{}, want: `{"defaultServiceId":"` + testDefaultService + `","defaultGroup":null}`},
		"lookup failure":  {defaultService: testDefaultService, repo: &stubIncidentRepo{lookupErr: errors.New("down")}, wantErr: true},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := pgService(tc.repo, tc.defaultService).GetIncidentCreateDefaults(userCtx())
			if tc.wantErr {
				if err == nil {
					t.Fatal("want an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("GetIncidentCreateDefaults: %v", err)
			}
			b, _ := json.Marshal(got)
			if string(b) != tc.want {
				t.Errorf("got %s, want %s", b, tc.want)
			}
		})
	}
}

// The startup check logs an error for a default team that cannot be used, and
// never fails.
func TestCheckIncidentDefaultService(t *testing.T) {
	for name, tc := range map[string]struct {
		repo      *stubIncidentRepo
		wantLevel string
	}{
		"usable":         {repo: &stubIncidentRepo{supportGroups: map[string]string{testDefaultService: testDefaultGroup}}, wantLevel: "level=INFO"},
		"groupless":      {repo: &stubIncidentRepo{supportGroups: map[string]string{testDefaultService: ""}}, wantLevel: "level=ERROR"},
		"missing":        {repo: &stubIncidentRepo{}, wantLevel: "level=ERROR"},
		"lookup failure": {repo: &stubIncidentRepo{lookupErr: errors.New("down")}, wantLevel: "level=WARN"},
	} {
		t.Run(name, func(t *testing.T) {
			logs := captureSlog(t)
			CheckIncidentDefaultService(context.Background(), pgService(tc.repo, testDefaultService), testDefaultService)
			if !strings.Contains(logs.String(), tc.wantLevel) {
				t.Errorf("logs = %s, want %s", logs.String(), tc.wantLevel)
			}
		})
	}
}
