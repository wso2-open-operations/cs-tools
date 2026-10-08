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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/dto"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/middleware"
)

const testChangeRequestID = "22222222-2222-2222-2222-222222222222"

// fakeEntityChangeRequestClient records what the handler forwards to
// entity-service. entityChangeRequestClient is embedded (nil) so only the two
// methods under test are implemented.
type fakeEntityChangeRequestClient struct {
	entityChangeRequestClient
	patchCalls  int
	gotPatch    entity.PatchChangeRequestRequest
	patchErr    error
	decideCalls int
	gotDecision string
	// canAnswer is the customerCanAnswer the fake's detail carries.
	canAnswer *bool
	// proposal is the customerProposal the fake's detail carries.
	proposal *entity.ChangeRequestCustomerProposal
}

func (f *fakeEntityChangeRequestClient) GetChangeRequest(_ context.Context, id string) (entity.ChangeRequest, error) {
	var out entity.ChangeRequest
	out.ID = id
	out.Number = "CHG0000001"
	out.CreatedOn = "2026-10-06T00:00:00Z"
	out.UpdatedOn = "2026-10-06T00:00:00Z"
	out.CustomerCanAnswer = f.canAnswer
	out.CustomerProposal = f.proposal
	return out, nil
}

func (f *fakeEntityChangeRequestClient) UpdateChangeRequest(_ context.Context, id string, req entity.PatchChangeRequestRequest) (entity.PatchChangeRequestResponse, error) {
	f.patchCalls++
	f.gotPatch = req
	if f.patchErr != nil {
		return entity.PatchChangeRequestResponse{}, f.patchErr
	}
	var out entity.PatchChangeRequestResponse
	out.ChangeRequest.ID = id
	out.ChangeRequest.UpdatedOn = "2026-10-06T00:00:00Z"
	return out, nil
}

func (f *fakeEntityChangeRequestClient) DecideChangeRequestApproval(_ context.Context, id string, req entity.ChangeRequestApprovalDecisionRequest) (entity.ChangeRequestApprovalDecisionResponse, error) {
	f.decideCalls++
	f.gotDecision = req.Decision
	return entity.ChangeRequestApprovalDecisionResponse{ID: id, State: req.Decision}, nil
}

// patchAs sends PATCH /change-requests/{id} to the handler as a caller the route
// let in at the given level (a zero action is a request that never passed
// through the permission middleware).
func patchAs(t *testing.T, fake *fakeEntityChangeRequestClient, granted middleware.Action, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := authedRequest(http.MethodPatch, "/change-requests/"+testChangeRequestID, body)
	req.SetPathValue("id", testChangeRequestID)
	if granted != "" {
		req = req.WithContext(middleware.WithGrantedAction(req.Context(), granted))
	}
	rec := httptest.NewRecorder()
	handlerAt(fake).PatchChangeRequest(rec, req)
	return rec
}

// testNow is the clock the change-request handler tests run at, so that "a time
// still to come" does not depend on the day the suite runs.
var testNow = time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)

// handlerAt is a ChangeRequestHandler whose clock reads testNow.
func handlerAt(fake *fakeEntityChangeRequestClient) *ChangeRequestHandler {
	h := NewChangeRequestHandler(fake)
	h.now = func() time.Time { return testNow }
	return h
}

func wantMessage(t *testing.T, rec *httptest.ResponseRecorder, contains string) {
	t.Helper()
	var body struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not JSON: %v (%s)", err, rec.Body.String())
	}
	if !strings.Contains(body.Message, contains) {
		t.Fatalf("message = %q, want it to contain %q", body.Message, contains)
	}
}

// What a customer is allowed to send, and exactly what reaches entity-service.
func TestPatchChangeRequest_CustomerAllowedBodies(t *testing.T) {
	yes, no := true, false
	start, end := "2026-10-10 10:00:00", "2026-10-10 12:00:00"
	seenStart, seenEnd := "2026-10-10T10:00:00Z", "2026-10-10T12:00:00Z"
	tests := []struct {
		name, body string
		want       entity.PatchChangeRequestRequest
	}{
		{"approve", `{"isCustomerApproved":true}`, entity.PatchChangeRequestRequest{IsCustomerApproved: &yes}},
		{"reject", `{"isCustomerApproved":false}`, entity.PatchChangeRequestRequest{IsCustomerApproved: &no}},
		{"confirm the review", `{"isCustomerReviewed":true}`, entity.PatchChangeRequestRequest{IsCustomerReviewed: &yes}},
		{"fail the review", `{"isCustomerReviewed":false}`, entity.PatchChangeRequestRequest{IsCustomerReviewed: &no}},
		{"propose a start (what the webapp sends)", `{"plannedStartOn":"2026-10-10 10:00:00"}`, entity.PatchChangeRequestRequest{PlannedStartOn: &start}},
		{"propose a window", `{"plannedStartOn":"2026-10-10 10:00:00","plannedEndOn":"2026-10-10 12:00:00"}`, entity.PatchChangeRequestRequest{PlannedStartOn: &start, PlannedEndOn: &end}},
		{"approve for the window that was shown", `{"isCustomerApproved":true,"expectedPlannedStartOn":"2026-10-10T10:00:00Z","expectedPlannedEndOn":"2026-10-10T12:00:00Z"}`,
			entity.PatchChangeRequestRequest{IsCustomerApproved: &yes, ExpectedPlannedStartOn: &seenStart, ExpectedPlannedEndOn: &seenEnd}},
		{"confirm the review for the window that was shown", `{"isCustomerReviewed":true,"expectedPlannedEndOn":"2026-10-10T12:00:00Z"}`,
			entity.PatchChangeRequestRequest{IsCustomerReviewed: &yes, ExpectedPlannedEndOn: &seenEnd}},
		{"field names are matched case-insensitively like everywhere else", `{"ISCUSTOMERAPPROVED":true}`, entity.PatchChangeRequestRequest{IsCustomerApproved: &yes}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeEntityChangeRequestClient{}
			rec := patchAs(t, fake, middleware.ActionDecide, tt.body)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200. body: %s", rec.Code, rec.Body.String())
			}
			if fake.patchCalls != 1 {
				t.Fatalf("entity-service called %d times, want 1", fake.patchCalls)
			}
			if !reflect.DeepEqual(fake.gotPatch, tt.want) {
				t.Errorf("forwarded %+v, want exactly %+v", fake.gotPatch, tt.want)
			}
		})
	}
}

// Every field of entity-service's PATCH contract other than the six customer
// ones is refused for a customer, with nothing forwarded. Derived from the
// struct, so a field added to entity-service later is covered without anyone
// remembering to list it here: it is refused until a customer is deliberately
// allowed to set it.
func TestPatchChangeRequest_CustomerCannotSendAnyOtherField(t *testing.T) {
	allowed := map[string]bool{"isCustomerApproved": true, "isCustomerReviewed": true, "plannedStartOn": true, "plannedEndOn": true,
		"expectedPlannedStartOn": true, "expectedPlannedEndOn": true}
	var fields []string
	typ := reflect.TypeOf(entity.PatchChangeRequestRequest{})
	for i := 0; i < typ.NumField(); i++ {
		name := strings.Split(typ.Field(i).Tag.Get("json"), ",")[0]
		if name != "" && !allowed[name] {
			fields = append(fields, name)
		}
	}
	// Fields the portal never had in either struct, and spellings that must not
	// slip through.
	fields = append(fields, "state", "onHold", "onHoldReason", "priority", "category", "customerApprovalRequired",
		"customerReviewRequired", "comment", "workNote", "isPlanningVisibleToCustomers", "deploymentIds",
		"notAField", "title ", " title", "TITLE", "Title", "is_customer_approved", "isCustomerApprovedX", "__proto__")
	if len(fields) < 25 {
		t.Fatalf("only %d forbidden fields enumerated; the struct walk is broken", len(fields))
	}

	for _, field := range fields {
		for _, body := range []string{
			`{"` + field + `":"x"}`,
			`{"` + field + `":null}`,
			`{"` + field + `":true}`,
			`{"isCustomerApproved":true,"` + field + `":"x"}`,
			`{"` + field + `":"x","plannedStartOn":"2026-10-10 10:00:00"}`,
		} {
			fake := &fakeEntityChangeRequestClient{}
			rec := patchAs(t, fake, middleware.ActionDecide, body)
			if rec.Code != http.StatusForbidden {
				t.Errorf("%s: status = %d, want 403", body, rec.Code)
			}
			if fake.patchCalls != 0 {
				t.Errorf("%s: reached entity-service (%+v)", body, fake.gotPatch)
			}
		}
	}

	t.Run("the refusal says what a customer can do", func(t *testing.T) {
		rec := patchAs(t, &fakeEntityChangeRequestClient{}, middleware.ActionDecide, `{"title":"x"}`)
		wantMessage(t, rec, "approve or reject")
	})
}

func TestPatchChangeRequest_CustomerMalformedAndAmbiguousBodies(t *testing.T) {
	tests := []struct {
		name, body, wantMsg string
	}{
		{"empty object", `{}`, "At least one of"},
		{"the expected window alone", `{"expectedPlannedStartOn":"2026-10-10T10:00:00Z"}`, "go with isCustomerApproved or isCustomerReviewed only"},
		{"the expected window beside a proposed time", `{"plannedStartOn":"2026-10-10 10:00:00","expectedPlannedEndOn":"2026-10-10T12:00:00Z"}`, "go with isCustomerApproved or isCustomerReviewed only"},
		{"wrong type for the flag", `{"isCustomerApproved":"yes"}`, "Invalid request payload"},
		{"wrong type for the window", `{"plannedStartOn":12}`, "Invalid request payload"},
		{"both outcomes", `{"isCustomerApproved":true,"isCustomerReviewed":true}`, "not both"},
		{"answer and a proposed time together", `{"isCustomerApproved":true,"plannedStartOn":"2026-10-10 10:00:00"}`, "separate requests"},
		{"review and a proposed time together", `{"isCustomerReviewed":false,"plannedEndOn":"2026-10-10 10:00:00"}`, "separate requests"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeEntityChangeRequestClient{}
			rec := patchAs(t, fake, middleware.ActionDecide, tt.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400. body: %s", rec.Code, rec.Body.String())
			}
			wantMessage(t, rec, tt.wantMsg)
			if fake.patchCalls != 0 {
				t.Errorf("a refused body reached entity-service: %+v", fake.gotPatch)
			}
		})
	}
}

// The restriction must hold when the request never went through the permission
// middleware, or went through it at a level other than Update: only a positive
// match on Update gets the wide field set.
func TestPatchChangeRequest_OnlyUpdateGetsTheFullFieldSet(t *testing.T) {
	for _, granted := range []middleware.Action{"", middleware.ActionDecide, middleware.ActionRead, middleware.ActionDelete} {
		fake := &fakeEntityChangeRequestClient{}
		rec := patchAs(t, fake, granted, `{"title":"new title"}`)
		if rec.Code != http.StatusForbidden || fake.patchCalls != 0 {
			t.Errorf("granted %q: status %d, upstream calls %d; want 403 and none", granted, rec.Code, fake.patchCalls)
		}
	}
}

// Staff (Update) are unchanged: the full customer-safe field set, extra keys
// dropped by the struct as before.
func TestPatchChangeRequest_StaffUnchanged(t *testing.T) {
	fake := &fakeEntityChangeRequestClient{}
	rec := patchAs(t, fake, middleware.ActionUpdate,
		`{"title":"new title","impact":"high","isCustomerApproved":true,"state":"scheduled","assignedTeamId":"x"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200. body: %s", rec.Code, rec.Body.String())
	}
	got := fake.gotPatch
	if got.Title == nil || *got.Title != "new title" || got.Impact == nil || *got.Impact != "high" || got.IsCustomerApproved == nil || !*got.IsCustomerApproved {
		t.Errorf("forwarded %+v, want title, impact and isCustomerApproved", got)
	}
	// state / assignedTeamId are not in dto.ChangeRequestUpdateRequest and stay
	// dropped, as they always were.
	if got.State != nil || got.AssignedTeamID != nil {
		t.Errorf("forwarded state/assignedTeamId (%v / %v): the staff field set must stay the customer-safe subset", got.State, got.AssignedTeamID)
	}

	if rec := patchAs(t, &fakeEntityChangeRequestClient{}, middleware.ActionUpdate, `{}`); rec.Code != http.StatusBadRequest {
		t.Errorf("empty body as staff: status = %d, want 400", rec.Code)
	}
}

// The planned window a customer's answer was given for is a precondition of the
// customer's own answer and nothing else. A staff body can never carry one, so a
// staff body that names it is refused with a 400 and nothing is sent: the typed
// decode used to drop the keys, which let an edit that asked for the check succeed
// without it (and turned a body of only those keys into "at least one field").
func TestPatchChangeRequest_StaffCannotCarryTheExpectedWindow(t *testing.T) {
	const want = "go with a customer's own answer (isCustomerApproved or isCustomerReviewed), which staff cannot give"
	for _, tc := range []struct{ name, body string }{
		{"the start beside an edit", `{"title":"new title","expectedPlannedStartOn":"2026-10-10T10:00:00Z"}`},
		{"the end beside an edit", `{"impact":"high","expectedPlannedEndOn":"2026-10-10T12:00:00Z"}`},
		{"both beside a window", `{"plannedStartOn":"2026-10-10 10:00:00","expectedPlannedStartOn":"2026-10-10T10:00:00Z","expectedPlannedEndOn":"2026-10-10T12:00:00Z"}`},
		{"beside an answer (which staff cannot give either)", `{"isCustomerApproved":true,"expectedPlannedStartOn":"2026-10-10T10:00:00Z"}`},
		{"alone", `{"expectedPlannedStartOn":"2026-10-10T10:00:00Z"}`},
		{"spelt in another case, as the decode reads keys", `{"title":"x","EXPECTEDPLANNEDENDON":"2026-10-10T12:00:00Z"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeEntityChangeRequestClient{}
			rec := patchAs(t, fake, middleware.ActionUpdate, tc.body)
			if rec.Code != http.StatusBadRequest || fake.patchCalls != 0 {
				t.Fatalf("status %d, upstream calls %d; want 400 and none (%s)", rec.Code, fake.patchCalls, rec.Body.String())
			}
			wantMessage(t, rec, want)
		})
	}

	// null reads as absent, here and in entity-service: the body is an ordinary edit.
	t.Run("null is absent", func(t *testing.T) {
		fake := &fakeEntityChangeRequestClient{}
		rec := patchAs(t, fake, middleware.ActionUpdate, `{"title":"x","expectedPlannedStartOn":null,"expectedPlannedEndOn":null}`)
		if rec.Code != http.StatusOK || fake.patchCalls != 1 {
			t.Fatalf("status %d, upstream calls %d; want 200 and 1 (%s)", rec.Code, fake.patchCalls, rec.Body.String())
		}
		if fake.gotPatch.ExpectedPlannedStartOn != nil || fake.gotPatch.ExpectedPlannedEndOn != nil {
			t.Errorf("forwarded the expected window: %+v", fake.gotPatch)
		}
	})

	// The customer level is untouched: an answer carries it through, as before.
	t.Run("a customer's answer still carries it", func(t *testing.T) {
		fake := &fakeEntityChangeRequestClient{}
		rec := patchAs(t, fake, middleware.ActionDecide, `{"isCustomerApproved":true,"expectedPlannedStartOn":"2026-10-10T10:00:00Z","expectedPlannedEndOn":"2026-10-10T12:00:00Z"}`)
		if rec.Code != http.StatusOK || fake.patchCalls != 1 {
			t.Fatalf("status %d, upstream calls %d; want 200 and 1 (%s)", rec.Code, fake.patchCalls, rec.Body.String())
		}
		if fake.gotPatch.ExpectedPlannedStartOn == nil || *fake.gotPatch.ExpectedPlannedStartOn != "2026-10-10T10:00:00Z" ||
			fake.gotPatch.ExpectedPlannedEndOn == nil || *fake.gotPatch.ExpectedPlannedEndOn != "2026-10-10T12:00:00Z" {
			t.Errorf("forwarded %+v, want the window the customer was shown", fake.gotPatch)
		}
	})
}

// What the customer is told when entity-service refuses: a 409 carries its
// readable message through, a 403 is the generic one (never upstream text), and
// the id and body are validated before anything is sent.
func TestPatchChangeRequest_CustomerUpstreamErrors(t *testing.T) {
	t.Run("409 passes the readable message through", func(t *testing.T) {
		msg := "this approval is no longer pending: the change request is in Scheduled, but the Customer Approval stage can only be decided while it is in Customer Approval"
		fake := &fakeEntityChangeRequestClient{patchErr: &apierror.Error{StatusCode: http.StatusConflict, Body: msg}}
		rec := patchAs(t, fake, middleware.ActionDecide, `{"isCustomerApproved":true}`)
		if rec.Code != http.StatusConflict {
			t.Fatalf("status = %d, want 409", rec.Code)
		}
		wantMessage(t, rec, "no longer pending")
	})
	t.Run("a proposed time's 400 and 409 reasons pass through", func(t *testing.T) {
		for status, msgs := range map[int][]string{
			http.StatusBadRequest: {
				"a proposed implementation time needs a new start: send plannedStartOn",
				"a proposed time moves the start and keeps the planned length of 2 hours: plannedEndOn must be 2030-03-08T11:00:00Z, or be left out",
				"plannedStartOn is the planned start already: propose a different start",
				"that time is already proposed and is waiting for WSO2's response",
				"WSO2 asked for a different time than that one: propose another start",
			},
			http.StatusConflict: {
				"this change request has no planned window to move, so a new time cannot be proposed for it",
				"this change request is on hold, so a new implementation time cannot be proposed now",
			},
		} {
			for _, msg := range msgs {
				fake := &fakeEntityChangeRequestClient{patchErr: &apierror.Error{StatusCode: status, Body: msg}}
				rec := patchAs(t, fake, middleware.ActionDecide, `{"plannedStartOn":"2026-12-01 09:00:00","plannedEndOn":"2026-12-01 11:00:00"}`)
				if rec.Code != status {
					t.Fatalf("%q: status = %d, want %d", msg, rec.Code, status)
				}
				wantMessage(t, rec, msg)
			}
		}
	})
	t.Run("a start with the end that keeps the planned length is forwarded as typed", func(t *testing.T) {
		fake := &fakeEntityChangeRequestClient{}
		rec := patchAs(t, fake, middleware.ActionDecide, `{"plannedStartOn":"2026-12-01 09:00:00","plannedEndOn":"2026-12-01 11:00:00"}`)
		if rec.Code != http.StatusOK || fake.patchCalls != 1 {
			t.Fatalf("status %d, upstream calls %d; want 200 and 1", rec.Code, fake.patchCalls)
		}
		if fake.gotPatch.PlannedStartOn == nil || *fake.gotPatch.PlannedStartOn != "2026-12-01 09:00:00" ||
			fake.gotPatch.PlannedEndOn == nil || *fake.gotPatch.PlannedEndOn != "2026-12-01 11:00:00" {
			t.Errorf("forwarded %+v, want the start and the derived end exactly as the customer sent them", fake.gotPatch)
		}
	})
	t.Run("a customer cannot answer for WSO2", func(t *testing.T) {
		// confirmCustomerUpdatedDate / expectedCustomerUpdatedOn are not among the customer's six fields:
		// unknown to the strict decoder, so a 403 and nothing forwarded.
		for _, body := range []string{
			`{"confirmCustomerUpdatedDate":"agree"}`,
			`{"plannedStartOn":"2026-12-01 09:00:00","expectedCustomerUpdatedOn":"2026-12-01T09:00:00Z"}`,
			`{"state":"authorize","plannedStartOn":"2026-12-01 09:00:00"}`,
		} {
			fake := &fakeEntityChangeRequestClient{}
			rec := patchAs(t, fake, middleware.ActionDecide, body)
			if rec.Code != http.StatusForbidden || fake.patchCalls != 0 {
				t.Errorf("%s: status %d, upstream calls %d; want 403 and none", body, rec.Code, fake.patchCalls)
			}
		}
	})
	t.Run("403 from a registered contact of another project is the generic message", func(t *testing.T) {
		fake := &fakeEntityChangeRequestClient{patchErr: &apierror.Error{StatusCode: http.StatusForbidden, Body: "only an internal user or a registered PORTAL_USER contact ..."}}
		rec := patchAs(t, fake, middleware.ActionDecide, `{"isCustomerApproved":true}`)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", rec.Code)
		}
		if strings.Contains(rec.Body.String(), "PORTAL_USER") {
			t.Errorf("upstream 403 text leaked: %s", rec.Body.String())
		}
	})
	t.Run("a bad id is refused before anything is sent", func(t *testing.T) {
		fake := &fakeEntityChangeRequestClient{}
		req := authedRequest(http.MethodPatch, "/change-requests/not-a-uuid", `{"isCustomerApproved":true}`)
		req.SetPathValue("id", "not-a-uuid")
		req = req.WithContext(middleware.WithGrantedAction(req.Context(), middleware.ActionDecide))
		rec := httptest.NewRecorder()
		NewChangeRequestHandler(fake).PatchChangeRequest(rec, req)
		if rec.Code != http.StatusBadRequest || fake.patchCalls != 0 {
			t.Errorf("status %d, upstream calls %d; want 400 and none", rec.Code, fake.patchCalls)
		}
	})
}

// GET /change-requests/{id} hands the portal entity-service's per-viewer answer
// exactly as it came: true, false, or nothing at all when entity-service did not
// compute one.
func TestGetChangeRequest_CarriesCustomerCanAnswer(t *testing.T) {
	yes, no := true, false
	for name, tc := range map[string]struct {
		in   *bool
		want any // nil = the key must be absent
	}{
		"not computed": {nil, nil},
		"false":        {&no, false},
		"true":         {&yes, true},
	} {
		t.Run(name, func(t *testing.T) {
			fake := &fakeEntityChangeRequestClient{canAnswer: tc.in}
			req := authedRequest(http.MethodGet, "/change-requests/"+testChangeRequestID, "")
			req.SetPathValue("id", testChangeRequestID)
			rec := httptest.NewRecorder()
			NewChangeRequestHandler(fake).GetChangeRequest(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200. body: %s", rec.Code, rec.Body.String())
			}
			var got map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("body is not JSON: %v (%s)", err, rec.Body.String())
			}
			v, present := got["customerCanAnswer"]
			if tc.want == nil {
				if present {
					t.Fatalf("customerCanAnswer = %v, want it absent", v)
				}
				return
			}
			if !present || v != tc.want {
				t.Fatalf("customerCanAnswer = %v (present %v), want %v", v, present, tc.want)
			}
		})
	}
}

// GET /change-requests/{id} hands the portal the conversation about a time a customer proposed as it
// came from entity-service, renamed to the portal's own words (startDate / endDate), and only the
// customer's part of it: whether the proposal is theirs, never who proposed it.
func TestGetChangeRequest_CarriesTheCustomerProposal(t *testing.T) {
	yes, no := true, false
	end := "2030-03-08T11:00:00Z"
	for name, tc := range map[string]struct {
		in   *entity.ChangeRequestCustomerProposal
		want map[string]any // nil = the key must be absent
	}{
		"none": {nil, nil},
		"pending and theirs": {&entity.ChangeRequestCustomerProposal{StartOn: "2030-03-08T09:00:00Z", EndOn: &end, Answer: "pending", ProposerRecorded: &yes, ProposedByViewer: &yes},
			map[string]any{"startDate": "2030-03-08T09:00:00Z", "endDate": end, "answer": "pending", "proposerRecorded": true, "proposedByViewer": true}},
		"pending, a colleague's": {&entity.ChangeRequestCustomerProposal{StartOn: "2030-03-08T09:00:00Z", EndOn: &end, Answer: "pending", ProposerRecorded: &yes, ProposedByViewer: &no},
			map[string]any{"startDate": "2030-03-08T09:00:00Z", "endDate": end, "answer": "pending", "proposerRecorded": true, "proposedByViewer": false}},
		"pending, nobody can say who (an older entity-service)": {&entity.ChangeRequestCustomerProposal{StartOn: "2030-03-08T09:00:00Z", EndOn: &end, Answer: "pending", ProposerRecorded: &no, ProposedByViewer: &no},
			map[string]any{"startDate": "2030-03-08T09:00:00Z", "endDate": end, "answer": "pending", "proposerRecorded": false, "proposedByViewer": false}},
		// entity-service tells a customer a time waits for WSO2 only when somebody is recorded as having proposed it; a
		// stored time nobody proposed comes as history, with nothing that says it is waiting.
		"unanswered, a stored time nobody is recorded as having proposed": {&entity.ChangeRequestCustomerProposal{StartOn: "2030-03-08T09:00:00Z", Answer: "unanswered"},
			map[string]any{"startDate": "2030-03-08T09:00:00Z", "answer": "unanswered"}},
		"agreed":    {&entity.ChangeRequestCustomerProposal{StartOn: "2030-03-08T09:00:00Z", Answer: "agreed"}, map[string]any{"startDate": "2030-03-08T09:00:00Z", "answer": "agreed"}},
		"disagreed": {&entity.ChangeRequestCustomerProposal{StartOn: "2030-03-08T09:00:00Z", Answer: "disagreed"}, map[string]any{"startDate": "2030-03-08T09:00:00Z", "answer": "disagreed"}},
	} {
		t.Run(name, func(t *testing.T) {
			fake := &fakeEntityChangeRequestClient{proposal: tc.in}
			req := authedRequest(http.MethodGet, "/change-requests/"+testChangeRequestID, "")
			req.SetPathValue("id", testChangeRequestID)
			rec := httptest.NewRecorder()
			NewChangeRequestHandler(fake).GetChangeRequest(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200. body: %s", rec.Code, rec.Body.String())
			}
			var got map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("body is not JSON: %v (%s)", err, rec.Body.String())
			}
			v, present := got["customerProposal"]
			if tc.want == nil {
				if present {
					t.Fatalf("customerProposal = %v, want it absent", v)
				}
				return
			}
			if !present || !reflect.DeepEqual(v, tc.want) {
				t.Fatalf("customerProposal = %v (present %v), want %v", v, present, tc.want)
			}
		})
	}
}

// What WSO2 knows about a proposal and a customer must not -- the proposer, whether Accept would
// work and why not -- never reaches the portal, even if entity-service's payload carried it: the
// portal's entity type has no field for any of it.
func TestMapChangeRequestDetails_NeverPassesOnWhoProposedOrWSO2sOwnFacts(t *testing.T) {
	var cr entity.ChangeRequest
	if err := json.Unmarshal([]byte(`{"id":"cr-1","state":"customer_approval","customerProposal":{
		"startOn":"2030-03-08T09:00:00Z","endOn":"2030-03-08T11:00:00Z","answer":"pending","proposerRecorded":true,
		"proposedByName":"Alice Example","proposedByEmail":"alice@example.com","proposedOn":"2030-03-05T10:00:00Z",
		"proposedByViewer":false,"canAccept":true,"acceptBlockedReason":"change request is on hold"}}`), &cr); err != nil {
		t.Fatalf("decode: %v", err)
	}
	raw, err := json.Marshal(dto.MapChangeRequestDetails(cr))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, leaked := range []string{"Alice", "alice@example.com", "proposedByName", "proposedByEmail", "proposedOn", "canAccept", "acceptBlockedReason", "on hold"} {
		if strings.Contains(string(raw), leaked) {
			t.Fatalf("the customer's detail carries %q: %s", leaked, raw)
		}
	}
}

// A proposed implementation time is checked before anything is sent to
// entity-service: the Postgres values that used to ride through `::timestamptz`
// (tomorrow, now, infinity, a bare date), a time that has passed, an inverted or
// empty window. Each is a 400 in the words entity-service uses, with nothing
// forwarded. entity-service repeats every check; this is only the first layer.
func TestPatchChangeRequest_CustomerProposalIsValidatedBeforeItIsSent(t *testing.T) {
	tests := []struct {
		name, body, wantMsg string
	}{
		{"tomorrow", `{"plannedStartOn":"tomorrow"}`, "plannedStartOn must be a valid date-time"},
		{"now", `{"plannedStartOn":"now"}`, "plannedStartOn must be a valid date-time"},
		{"infinity", `{"plannedEndOn":"infinity"}`, "plannedEndOn must be a valid date-time"},
		{"a bare date", `{"plannedStartOn":"2026-12-01"}`, "plannedStartOn must be a valid date-time"},
		{"an empty value", `{"plannedStartOn":""}`, "plannedStartOn must be a valid date-time"},
		{"a year out of range", `{"plannedStartOn":"2101-01-01 00:00:00"}`, "plannedStartOn must be a valid date-time"},
		{"a start in the past", `{"plannedStartOn":"2026-10-05 10:00:00"}`, "plannedStartOn is in the past"},
		{"a start that is right now", `{"plannedStartOn":"2026-10-06 00:00:00"}`, "plannedStartOn is in the past"},
		{"an end in the past", `{"plannedEndOn":"2026-10-05 10:00:00"}`, "plannedEndOn is in the past"},
		{"a window ending before it starts", `{"plannedStartOn":"2026-12-01 10:00:00","plannedEndOn":"2026-12-01 09:00:00"}`, "must not be after the planned end"},
		{"a window with no duration", `{"plannedStartOn":"2026-12-01 10:00:00","plannedEndOn":"2026-12-01T10:00:00Z"}`, "must not be the same as the planned end"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeEntityChangeRequestClient{}
			rec := patchAs(t, fake, middleware.ActionDecide, tt.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400. body: %s", rec.Code, rec.Body.String())
			}
			wantMessage(t, rec, tt.wantMsg)
			if fake.patchCalls != 0 {
				t.Errorf("a refused proposal reached entity-service: %+v", fake.gotPatch)
			}
		})
	}

	t.Run("a start in the future is sent as typed", func(t *testing.T) {
		fake := &fakeEntityChangeRequestClient{}
		rec := patchAs(t, fake, middleware.ActionDecide, `{"plannedStartOn":"2026-10-06 00:00:01"}`)
		if rec.Code != http.StatusOK || fake.patchCalls != 1 {
			t.Fatalf("status %d, upstream calls %d; want 200 and 1", rec.Code, fake.patchCalls)
		}
		if fake.gotPatch.PlannedStartOn == nil || *fake.gotPatch.PlannedStartOn != "2026-10-06 00:00:01" {
			t.Errorf("forwarded %+v, want the start exactly as the customer sent it", fake.gotPatch)
		}
	})

	t.Run("a proposal is a start: the start alone, or with the end that keeps the planned length, is forwarded as typed", func(t *testing.T) {
		for name, body := range map[string]string{
			"the start alone":                    `{"plannedStartOn":"2026-12-01 10:00:00"}`,
			"the start and the derived end":      `{"plannedStartOn":"2026-12-01 10:00:00","plannedEndOn":"2026-12-01 12:00:00"}`,
			"RFC 3339 with an offset, both ways": `{"plannedStartOn":"2026-12-01T15:30:00+05:30","plannedEndOn":"2026-12-01T17:30:00+05:30"}`,
		} {
			fake := &fakeEntityChangeRequestClient{}
			rec := patchAs(t, fake, middleware.ActionDecide, body)
			if rec.Code != http.StatusOK || fake.patchCalls != 1 {
				t.Fatalf("%s: status %d, upstream calls %d; want 200 and 1", name, rec.Code, fake.patchCalls)
			}
			var sent map[string]any
			raw, _ := json.Marshal(fake.gotPatch)
			if err := json.Unmarshal(raw, &sent); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if len(sent) > 2 {
				t.Errorf("%s: forwarded more than the window: %s", name, raw)
			}
		}
	})

	t.Run("an answer is not a proposal: no clock check on the window it was shown", func(t *testing.T) {
		// The expected window is what the customer was shown, which may be in the
		// past by the time they answer; it is compared by entity-service, not
		// validated as a proposed time.
		fake := &fakeEntityChangeRequestClient{}
		rec := patchAs(t, fake, middleware.ActionDecide,
			`{"isCustomerApproved":true,"expectedPlannedStartOn":"2020-01-01T10:00:00Z","expectedPlannedEndOn":"2020-01-01T12:00:00Z"}`)
		if rec.Code != http.StatusOK || fake.patchCalls != 1 {
			t.Fatalf("status %d, upstream calls %d; want 200 and 1", rec.Code, fake.patchCalls)
		}
	})
}

// Staff edit and create are checked for form and range only: a window that has
// passed is something staff may record, and ordering is entity-service's call.
func TestPatchChangeRequest_StaffWindowIsCheckedForFormAndRangeOnly(t *testing.T) {
	t.Run("a malformed start is a 400 and is not sent", func(t *testing.T) {
		for _, bad := range []string{"tomorrow", "infinity", "2030-03-01", "1999-01-01 00:00:00"} {
			fake := &fakeEntityChangeRequestClient{}
			rec := patchAs(t, fake, middleware.ActionUpdate, `{"plannedStartOn":"`+bad+`"}`)
			if rec.Code != http.StatusBadRequest || fake.patchCalls != 0 {
				t.Errorf("%q: status %d, upstream calls %d; want 400 and none", bad, rec.Code, fake.patchCalls)
				continue
			}
			wantMessage(t, rec, "plannedStartOn must be a valid date-time")
		}
	})
	t.Run("a start in the past is accepted", func(t *testing.T) {
		fake := &fakeEntityChangeRequestClient{}
		rec := patchAs(t, fake, middleware.ActionUpdate, `{"plannedStartOn":"2026-01-02 03:04:05","plannedEndOn":"2026-01-02 02:00:00"}`)
		if rec.Code != http.StatusOK || fake.patchCalls != 1 {
			t.Fatalf("status %d, upstream calls %d; want 200 and 1 (%s)", rec.Code, fake.patchCalls, rec.Body.String())
		}
	})
	t.Run("an edit with no window is untouched", func(t *testing.T) {
		fake := &fakeEntityChangeRequestClient{}
		rec := patchAs(t, fake, middleware.ActionUpdate, `{"title":"x"}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d", rec.Code)
		}
	})
}

// POST /change-requests refuses a malformed planned window before it is sent.
func TestCreateChangeRequest_PlannedWindowIsValidatedBeforeItIsSent(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
		{"a start of 'tomorrow'", `{"subject":"s","plannedStartDate":"tomorrow"}`, "plannedStartDate must be a valid date-time"},
		{"an end of 'infinity'", `{"subject":"s","plannedEndDate":"infinity"}`, "plannedEndDate must be a valid date-time"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &createRecordingClient{}
			req := authedRequest(http.MethodPost, "/change-requests", tc.body)
			rec := httptest.NewRecorder()
			h := NewChangeRequestHandler(fake)
			h.CreateChangeRequest(rec, req)
			if rec.Code != http.StatusBadRequest || fake.createCalls != 0 {
				t.Fatalf("status %d, upstream calls %d; want 400 and none (%s)", rec.Code, fake.createCalls, rec.Body.String())
			}
			wantMessage(t, rec, tc.want)
		})
	}
	t.Run("a well-formed window is sent", func(t *testing.T) {
		fake := &createRecordingClient{}
		req := authedRequest(http.MethodPost, "/change-requests", `{"subject":"s","plannedStartDate":"2030-03-01 09:00:00","plannedEndDate":"2030-03-01T10:00:00Z"}`)
		rec := httptest.NewRecorder()
		NewChangeRequestHandler(fake).CreateChangeRequest(rec, req)
		if rec.Code != http.StatusCreated || fake.createCalls != 1 {
			t.Fatalf("status %d, upstream calls %d; want 201 and 1 (%s)", rec.Code, fake.createCalls, rec.Body.String())
		}
	})
}

// createRecordingClient counts POST /change-requests calls.
type createRecordingClient struct {
	fakeEntityChangeRequestClient
	createCalls int
}

func (f *createRecordingClient) CreateChangeRequest(_ context.Context, _ entity.CreateChangeRequestRequest) (entity.CreateChangeRequestResponse, error) {
	f.createCalls++
	return entity.CreateChangeRequestResponse{}, nil
}

// The whole way through, with the real entity client against a stand-in for
// entity-service that answers as it does: the machine-readable code of each
// refusal the portal branches on reaches the customer's browser, beside the
// message, with the status entity-service gave it; the 403's message stays the
// fixed one. This is the contract the webapp's classification stands on.
func TestPatchChangeRequest_EntityRefusalCodesReachTheCustomer(t *testing.T) {
	for _, tc := range []struct {
		name       string
		status     int
		upstream   string
		wantMsg    string
		wantCode   string
		body       string
		wantStatus int
	}{
		{"on hold", 409, `{"code":409,"message":"this change request is on hold, so a new implementation time cannot be proposed now","errorCode":"change_request_on_hold"}`,
			"this change request is on hold, so a new implementation time cannot be proposed now", "change_request_on_hold", `{"plannedStartOn":"2026-10-10 10:00:00","plannedEndOn":"2026-10-10 12:00:00"}`, 409},
		{"window changed", 409, `{"code":409,"message":"the planned implementation time of this change request changed after you opened it (it is now x)","errorCode":"change_request_schedule_changed"}`,
			"the planned implementation time of this change request changed after you opened it (it is now x)", "change_request_schedule_changed", `{"isCustomerApproved":true}`, 409},
		{"already answered", 409, `{"code":409,"message":"this approval is no longer pending","errorCode":"change_request_approval_not_pending"}`,
			"this approval is no longer pending", "change_request_approval_not_pending", `{"isCustomerApproved":true}`, 409},
		{"not proposable", 409, `{"code":409,"message":"not open to a new time","errorCode":"change_request_not_proposable"}`,
			"not open to a new time", "change_request_not_proposable", `{"plannedStartOn":"2026-10-10 10:00:00"}`, 409},
		{"not asked", 403, `{"code":403,"message":"only members of the customer group may answer","errorCode":"change_request_not_asked"}`,
			ErrMsgForbidden, "change_request_not_asked", `{"isCustomerApproved":true}`, 403},
		{"forbidden", 403, `{"code":403,"message":"only a registered PORTAL_USER contact may answer","errorCode":"change_request_forbidden"}`,
			ErrMsgForbidden, "change_request_forbidden", `{"isCustomerApproved":true}`, 403},
		{"an older entity-service names nothing", 409, `{"code":409,"message":"this approval is no longer pending"}`,
			"this approval is no longer pending", "", `{"isCustomerApproved":true}`, 409},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("POST /token", func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"access_token":"t","token_type":"Bearer","expires_in":3600}`))
			})
			mux.HandleFunc("PATCH /change-requests/{id}", func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.upstream))
			})
			srv := httptest.NewServer(mux)
			defer srv.Close()
			client := entity.NewClient(entity.Config{BaseURL: srv.URL, TokenURL: srv.URL + "/token", ClientID: "c", ClientSecret: "s"})

			req := authedRequest(http.MethodPatch, "/change-requests/"+testChangeRequestID, tc.body)
			req.SetPathValue("id", testChangeRequestID)
			req = req.WithContext(middleware.WithGrantedAction(req.Context(), middleware.ActionDecide))
			rec := httptest.NewRecorder()
			h := NewChangeRequestHandler(client)
			h.now = func() time.Time { return testNow }
			h.PatchChangeRequest(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
			var body map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("body is not JSON: %v (%s)", err, rec.Body.String())
			}
			if body["message"] != tc.wantMsg {
				t.Errorf("message = %v, want %q", body["message"], tc.wantMsg)
			}
			code, has := body["errorCode"]
			if tc.wantCode == "" && has {
				t.Errorf("errorCode = %v, want the key absent", code)
			}
			if tc.wantCode != "" && code != tc.wantCode {
				t.Errorf("errorCode = %v, want %q", code, tc.wantCode)
			}
		})
	}
}

// A refusal this layer raises itself carries a code too: a customer who sends a
// field they may not set is forbidden, under the same name entity-service gives
// its own 403s of that kind, and nothing reaches entity-service.
func TestPatchChangeRequest_CustomerFieldRefusalCarriesTheForbiddenCode(t *testing.T) {
	fake := &fakeEntityChangeRequestClient{}
	rec := patchAs(t, fake, middleware.ActionDecide, `{"title":"x"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	if body["errorCode"] != "change_request_forbidden" {
		t.Errorf("errorCode = %v, want change_request_forbidden", body["errorCode"])
	}
	if fake.patchCalls != 0 {
		t.Errorf("reached entity-service")
	}
	// The other refusals of this layer, 400s about the shape of the request, name nothing.
	rec = patchAs(t, &fakeEntityChangeRequestClient{}, middleware.ActionDecide, `{"isCustomerApproved":true,"isCustomerReviewed":true}`)
	if strings.Contains(rec.Body.String(), "errorCode") {
		t.Errorf("a 400 about the request's shape carries a code: %s", rec.Body.String())
	}
}
