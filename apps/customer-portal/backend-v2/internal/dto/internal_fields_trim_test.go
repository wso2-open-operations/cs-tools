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

package dto

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

func TestMapSearchCallRequests_OmitsAgentInternalFields(t *testing.T) {
	reason := "Need assistance"
	assignee := "support-agent@wso2.com"
	notes := "Internal support notes"
	plan := "Troubleshooting plan"
	attendees := "agent, customer"
	actions := "Follow up tomorrow"
	cancelReason := "No longer needed"
	duration := 45
	caseNum := "CS001"

	resp := MapSearchCallRequests(entity.SearchCallRequestsResponse{
		CallRequests: []entity.CallRequestView{
			{
				ID:                 "call-1",
				Number:             "CRQ001",
				Case:               entity.CallRequestCaseRef{ID: "case-1", Name: "Case 1", Number: &caseNum},
				Reason:             &reason,
				CancellationReason: &cancelReason,
				Assignee:           &assignee,
				Notes:              &notes,
				Plan:               &plan,
				Attendees:          &attendees,
				ActionItems:        &actions,
				ActualDurationMin:  &duration,
				DurationMin:        30,
				CreatedOn:          "2026-09-14 10:00:00",
				UpdatedOn:          "2026-09-14 10:30:00",
				State:              entity.CallRequestState{ID: "1", Label: "Requested"},
			},
		},
		Total: 1,
	})

	raw, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	calls, ok := got["callRequests"].([]any)
	if !ok || len(calls) == 0 {
		t.Fatalf("expected non-empty callRequests array: %v", got)
	}
	call := calls[0].(map[string]any)

	forbidden := []string{
		"cancellationReason",
		"assignee",
		"notes",
		"plan",
		"attendees",
		"actionItems",
		"actualDurationMin",
	}
	for _, k := range forbidden {
		if _, present := call[k]; present {
			t.Errorf("callRequest.%s = %v; want omitted from customer-facing response", k, call[k])
		}
	}

	// Verify required customer fields exist
	required := []string{"id", "number", "case", "durationMin", "createdOn", "updatedOn", "state"}
	for _, k := range required {
		if _, present := call[k]; !present {
			t.Errorf("callRequest.%s is missing; want present", k)
		}
	}
}

func TestMapCaseUpdate_OmitsInternalOperationalFields(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	workState := "In Progress"
	resCode := "Solved"
	cause := "Config issue"
	user := "customer-user@example.com"
	engEmail := "eng@wso2.com"

	resp := MapCaseUpdate(entity.UpdateCaseResponse{
		Case: entity.UpdatedCase{
			ID:             "case-1",
			UpdatedOn:      now,
			State:          "in_progress",
			Severity:       "moderate",
			WorkState:      &workState,
			WatchList:      []entity.WatchListUser{{UserName: "watcher"}},
			AssignedTo:     &entity.AssignedEngineerRef{ID: "eng-1", Name: "Engineer", Email: &engEmail},
			ResolutionCode: &resCode,
			Cause:          &cause,
			ResolvedOn:     &now,
			ParentCase:     &entity.CaseNumberRef{ID: "p-1", Number: "CS000"},
			FixEta:         &now,
			UpdatedBy:      user,
		},
	})

	raw, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	forbidden := []string{
		"workState",
		"severity",
		"assignedTo",
		"resolutionCode",
		"cause",
		"resolvedOn",
		"parentCase",
		"fixEta",
	}
	for _, k := range forbidden {
		if _, present := got[k]; present {
			t.Errorf("CaseUpdateResponse.%s = %v; want omitted", k, got[k])
		}
	}

	// Verify allowed fields
	if got["id"] != "case-1" {
		t.Errorf("id = %v, want case-1", got["id"])
	}
	if got["state"] != "in_progress" {
		t.Errorf("state = %v, want in_progress", got["state"])
	}
	if got["updatedBy"] != user {
		t.Errorf("updatedBy = %v, want %q", got["updatedBy"], user)
	}
	watchList, ok := got["watchList"].([]any)
	if !ok || len(watchList) != 1 || watchList[0] != "watcher" {
		t.Errorf("watchList = %v, want [\"watcher\"]", got["watchList"])
	}
}

func TestMapChangeRequestDetails_OmitsLegalNextStates(t *testing.T) {
	resp := MapChangeRequestDetails(entity.ChangeRequest{
		SearchChangeRequestView: entity.SearchChangeRequestView{
			ID:        "cr-1",
			Number:    "CHG001",
			CreatedOn: "2026-09-14 10:00:00",
			UpdatedOn: "2026-09-14 11:00:00",
		},
		LegalNextStates: []string{"scheduled", "closed"},
	})

	raw, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if _, present := got["legalNextStates"]; present {
		t.Errorf("ChangeRequestDetails.legalNextStates = %v; want omitted", got["legalNextStates"])
	}
}

func TestMapChangeRequestUpdate_MatchesThreeFieldContract(t *testing.T) {
	resp := MapChangeRequestUpdate(entity.PatchChangeRequestResponse{
		Message: "Change request updated successfully",
		ChangeRequest: entity.ChangeRequest{
			SearchChangeRequestView: entity.SearchChangeRequestView{
				ID:        "cr-1",
				UpdatedOn: "2026-09-14 12:00:00",
				UpdatedBy: "customer-user@example.com",
			},
		},
	})

	raw, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if got["id"] != "cr-1" {
		t.Errorf("id = %v, want cr-1", got["id"])
	}
	if got["updatedOn"] != "2026-09-14 12:00:00" {
		t.Errorf("updatedOn = %v, want 2026-09-14 12:00:00", got["updatedOn"])
	}
	if got["updatedBy"] != "customer-user@example.com" {
		t.Errorf("updatedBy = %v, want customer-user@example.com", got["updatedBy"])
	}

	// Ensure 30+ ChangeRequestDetails fields are not leaked in patch response
	leakCheck := []string{"subject", "description", "justification", "impactDescription", "legalNextStates", "rollbackPlan"}
	for _, k := range leakCheck {
		if _, present := got[k]; present {
			t.Errorf("ChangeRequestUpdateResponse.%s = %v; want omitted", k, got[k])
		}
	}
}

func TestMapSearchCases_OmitsDuplicateCaseTypes(t *testing.T) {
	resp := MapSearchCases(entity.SearchCasesResponse{
		Cases: []entity.SearchCaseView{
			{
				ID:     "case-1",
				Number: "CS001",
				Type:   "case",
				Project: entity.EntityRef{
					ID:   "proj-1",
					Name: "Project 1",
				},
			},
		},
		Total: 1,
	})

	raw, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	cases := got["cases"].([]any)
	c := cases[0].(map[string]any)

	if _, present := c["caseTypes"]; present {
		t.Errorf("caseTypes = %v; want omitted (redundant duplicate of type)", c["caseTypes"])
	}
	if _, present := c["type"]; !present {
		t.Errorf("type is missing from case summary")
	}
}

func TestMapSearchDeployments_OmitsCreatedBy(t *testing.T) {
	resp := MapSearchDeployments(entity.SearchDeploymentsResponse{
		Deployments: []entity.DeploymentView{
			{
				ID:        "dep-1",
				Number:    "DEP001",
				Name:      "Production",
				CreatedBy: &entity.EntityRef{ID: "u-1", Name: "Creator"},
			},
		},
		Total: 1,
	})

	raw, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	deployments := got["deployments"].([]any)
	d := deployments[0].(map[string]any)

	if _, present := d["createdBy"]; present {
		t.Errorf("createdBy = %v; want omitted from customer-facing deployment summary", d["createdBy"])
	}
}
