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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package apierror

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

func TestInvalidValue(t *testing.T) {
	if got := InvalidValue("sortBy", "x", "sort field", []string{"endDate"}).Msg; got != `sortBy: "x" is not a valid sort field; use endDate` {
		t.Errorf("single = %q", got)
	}
	if got := InvalidValue("f", "a\"b", "thing", []string{"A", "B"}).Msg; got != `f: "a\"b" is not a valid thing; use one of A, B` {
		t.Errorf("multi = %q", got)
	}
}

// TestWriteJSON_HasNoErrorCodeUnlessGiven pins that a body without a
// machine-readable name is exactly the body this API has always returned: a
// client that does not know errorCode sees no new key.
func TestWriteJSON_HasNoErrorCodeUnlessGiven(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteJSON(rec, http.StatusConflict, "stale")
	if got, want := strings.TrimSpace(rec.Body.String()), `{"code":409,"message":"stale"}`; got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}

func TestWriteJSONWithCode_AddsErrorCodeBesideTheMessage(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteJSONWithCode(rec, http.StatusConflict, "this change request is on hold", CodeChangeRequestOnHold)
	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rec.Code)
	}
	var body ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	if body.Code != 409 || body.Message != "this change request is on hold" || body.ErrorCode != "change_request_on_hold" {
		t.Errorf("body = %+v", body)
	}
	if !strings.Contains(rec.Body.String(), `"errorCode":"change_request_on_hold"`) {
		t.Errorf("wire name is not errorCode: %s", rec.Body.String())
	}
}

// TestCodes_AreStableLowerSnakeCase guards the contract: a code is only ever
// added to, so a rename shows up here as a failure, and each is a plain
// lower-case snake_case string no client has to escape.
func TestCodes_AreStableLowerSnakeCase(t *testing.T) {
	want := map[string]string{
		"CodeChangeRequestOnHold":               "change_request_on_hold",
		"CodeChangeRequestScheduleChanged":      "change_request_schedule_changed",
		"CodeChangeRequestApprovalNotPending":   "change_request_approval_not_pending",
		"CodeChangeRequestNotProposable":        "change_request_not_proposable",
		"CodeChangeRequestNotAsked":             "change_request_not_asked",
		"CodeChangeRequestForbidden":            "change_request_forbidden",
		"CodeChangeRequestProposalNotNow":       "change_request_proposal_not_now",
		"CodeChangeRequestNoPlannedWindow":      "change_request_no_planned_window",
		"CodeChangeRequestProposerNotRecorded":  "change_request_proposer_not_recorded",
		"CodeIncidentAssignmentGroupNotAllowed": "incident_assignment_group_not_allowed",
	}
	got := map[string]string{
		"CodeChangeRequestOnHold":               CodeChangeRequestOnHold,
		"CodeChangeRequestScheduleChanged":      CodeChangeRequestScheduleChanged,
		"CodeChangeRequestApprovalNotPending":   CodeChangeRequestApprovalNotPending,
		"CodeChangeRequestNotProposable":        CodeChangeRequestNotProposable,
		"CodeChangeRequestNotAsked":             CodeChangeRequestNotAsked,
		"CodeChangeRequestForbidden":            CodeChangeRequestForbidden,
		"CodeChangeRequestProposalNotNow":       CodeChangeRequestProposalNotNow,
		"CodeChangeRequestNoPlannedWindow":      CodeChangeRequestNoPlannedWindow,
		"CodeChangeRequestProposerNotRecorded":  CodeChangeRequestProposerNotRecorded,
		"CodeIncidentAssignmentGroupNotAllowed": CodeIncidentAssignmentGroupNotAllowed,
	}
	snake := regexp.MustCompile(`^[a-z][a-z0-9]*(_[a-z0-9]+)*$`)
	seen := map[string]string{}
	for name, code := range got {
		if code != want[name] {
			t.Errorf("%s = %q, want %q (codes are a contract: never rename)", name, code, want[name])
		}
		if !snake.MatchString(code) {
			t.Errorf("%s = %q is not lower snake_case", name, code)
		}
		if other, dup := seen[code]; dup {
			t.Errorf("%s and %s share the code %q", name, other, code)
		}
		seen[code] = name
	}
}
