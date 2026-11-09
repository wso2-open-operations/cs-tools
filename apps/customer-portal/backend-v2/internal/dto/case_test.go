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
	"reflect"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

// TestBuildEntitySearchCasesRequest_AllFiltersTranslate guards against
// reintroducing the bug where this backend sent entity-service's own
// named-filter-field case-search shape (or an invented one) directly instead
// of building the generic filter-predicate array entity-service's current
// /cases/search requires (which rejects unknown fields outright) — every
// filter the portal exposes, using the frontend's actual field names and
// ServiceNow numeric ids, must produce exactly the CaseFieldFilter entry
// entity-service's case_filters.go expects.
func TestBuildEntitySearchCasesRequest_AllFiltersTranslate(t *testing.T) {
	req := CaseSearchRequest{
		Filters: CaseSearchFilters{
			SearchQuery:        "login issue",
			StatusIDs:          []int{1}, // open
			CaseTypes:          []string{"default_case", "engagement"},
			SeverityIDs:        []int{11}, // high
			IssueIDs:           []int{4},  // question
			DeploymentIDs:      []string{"dep-1"},
			CreatedBy:          []string{"user-1"},
			EngagementTypeKeys: []int{1}, // migration
			StartCreatedDate:   "2026-01-01",
			EndCreatedDate:     "2026-01-31",
			StartUpdatedDate:   "2026-02-01",
			EndUpdatedDate:     "2026-02-28",
			ClosedStartDate:    "2026-03-01",
			ClosedEndDate:      "2026-03-31",
		},
		SortBy:     entity.CaseSort{Field: "createdOn", Order: "desc"},
		Pagination: entity.Pagination{Limit: 20, Offset: 0},
	}

	got := BuildEntitySearchCasesRequest("proj-1", req)

	if got.Filters.SearchQuery != "login issue" {
		t.Fatalf("SearchQuery = %q", got.Filters.SearchQuery)
	}
	if got.SortBy != req.SortBy {
		t.Fatalf("SortBy = %+v, want %+v", got.SortBy, req.SortBy)
	}
	if got.Pagination != req.Pagination {
		t.Fatalf("Pagination = %+v, want %+v", got.Pagination, req.Pagination)
	}

	want := []entity.CaseFieldFilter{
		{Field: "projectId", Op: "in", Values: []string{"proj-1"}},
		{Field: "type", Op: "in", Values: []string{"default_case", "engagement"}},
		{Field: "state", Op: "in", Values: []string{"open"}},
		{Field: "severity", Op: "in", Values: []string{"high"}},
		{Field: "issueType", Op: "in", Values: []string{"question"}},
		{Field: "engagementType", Op: "in", Values: []string{"migration"}},
		{Field: "deploymentId", Op: "in", Values: []string{"dep-1"}},
		{Field: "createdOn", Op: "gte", Values: []string{"2026-01-01"}},
		{Field: "createdOn", Op: "lte", Values: []string{"2026-01-31"}},
		{Field: "updatedOn", Op: "gte", Values: []string{"2026-02-01"}},
		{Field: "updatedOn", Op: "lte", Values: []string{"2026-02-28"}},
		{Field: "closedOn", Op: "gte", Values: []string{"2026-03-01"}},
		{Field: "closedOn", Op: "lte", Values: []string{"2026-03-31"}},
		{Field: "createdBy", Op: "in", Values: []string{"user-1"}},
	}
	if !reflect.DeepEqual(got.Filters.Filters, want) {
		t.Fatalf("Filters = %+v,\nwant     %+v", got.Filters.Filters, want)
	}
}

// TestBuildEntitySearchCasesRequest_UnrecognizedNumericIDsSkipped verifies an
// id with no entry in the caseXxxIDToEnum tables is silently dropped rather
// than sent to entity-service verbatim (which would 400, since entity-service
// only accepts its own lowercase-snake-case enum values, never a raw
// ServiceNow numeric id).
func TestBuildEntitySearchCasesRequest_UnrecognizedNumericIDsSkipped(t *testing.T) {
	req := CaseSearchRequest{Filters: CaseSearchFilters{StatusIDs: []int{999999}}}

	got := BuildEntitySearchCasesRequest("proj-1", req)

	want := []entity.CaseFieldFilter{
		{Field: "projectId", Op: "in", Values: []string{"proj-1"}},
	}
	if !reflect.DeepEqual(got.Filters.Filters, want) {
		t.Fatalf("Filters = %+v, want %+v (unrecognized status id must be dropped, not forwarded)", got.Filters.Filters, want)
	}
}

// TestBuildEntitySearchCasesRequest_ProjectIDAlwaysScoped verifies the
// projectID parameter (the POST /projects/{id}/cases/search path value) is
// always translated into a projectId+in filter, regardless of what the
// request body's filters contain — CaseSearchFilters has no client-settable
// projectIds field at all, so this is the only source of project scoping.
func TestBuildEntitySearchCasesRequest_ProjectIDAlwaysScoped(t *testing.T) {
	got := BuildEntitySearchCasesRequest("proj-42", CaseSearchRequest{})

	want := []entity.CaseFieldFilter{
		{Field: "projectId", Op: "in", Values: []string{"proj-42"}},
	}
	if !reflect.DeepEqual(got.Filters.Filters, want) {
		t.Fatalf("Filters = %+v, want %+v", got.Filters.Filters, want)
	}
	if got.Filters.SearchQuery != "" {
		t.Fatalf("expected empty SearchQuery, got %q", got.Filters.SearchQuery)
	}
}

// TestBuildEntitySearchCasesRequest_CreatedByMe verifies CreatedByMe becomes
// a createdBy+eq filter carrying entity-service's exact current-user
// placeholder string — a mismatch here would make entity-service reject it
// as "createdBy eq only supports values: [...]" instead of resolving the
// caller.
func TestBuildEntitySearchCasesRequest_CreatedByMe(t *testing.T) {
	req := CaseSearchRequest{Filters: CaseSearchFilters{CreatedByMe: true}}

	got := BuildEntitySearchCasesRequest("proj-1", req)

	want := []entity.CaseFieldFilter{
		{Field: "projectId", Op: "in", Values: []string{"proj-1"}},
		{Field: "createdBy", Op: "eq", Values: []string{"__current_user_email__"}},
	}
	if !reflect.DeepEqual(got.Filters.Filters, want) {
		t.Fatalf("Filters = %+v, want %+v", got.Filters.Filters, want)
	}
}

// TestBuildEntitySearchCasesRequest_CreatedByMeTakesPrecedenceOverCreatedBy
// guards against sending both a createdBy+in and a createdBy+eq filter for
// the same request: entity-service's filters array is AND-only, so both
// together could (barring coincidence) never match anything, silently
// returning an empty result set. If a client sends both, CreatedByMe must
// win and CreatedBy must be dropped entirely, not merged.
func TestBuildEntitySearchCasesRequest_CreatedByMeTakesPrecedenceOverCreatedBy(t *testing.T) {
	req := CaseSearchRequest{Filters: CaseSearchFilters{
		CreatedBy:   []string{"someone-else@example.com"},
		CreatedByMe: true,
	}}

	got := BuildEntitySearchCasesRequest("proj-1", req)

	want := []entity.CaseFieldFilter{
		{Field: "projectId", Op: "in", Values: []string{"proj-1"}},
		{Field: "createdBy", Op: "eq", Values: []string{"__current_user_email__"}},
	}
	if !reflect.DeepEqual(got.Filters.Filters, want) {
		t.Fatalf("Filters = %+v, want %+v", got.Filters.Filters, want)
	}
}

// TestMapSearchCases_MatchesFrontendCaseListItemShape verifies the full
// response translation: entity-service's plain-string/label fields become
// {id, label} refs, title comes from Subject, status comes from State, and
// TotalRecords (not Total) carries the response's pagination envelope — the
// exact set of mismatches that left the case list rendering "--" for every
// column and "0-0 of 0" for pagination despite a successful 200 response.
func TestMapSearchCases_MatchesFrontendCaseListItemShape(t *testing.T) {
	assignee := "engineer@example.com"
	subject := "Login error"
	description := "Cannot log in"
	severity := "High (P2)"
	issueType := "Question"

	r := entity.SearchCasesResponse{
		Total:  878,
		Offset: 0,
		Limit:  5,
		Cases: []entity.SearchCaseView{
			{
				ID:               "case-1",
				InternalID:       "internal-1",
				Number:           "CS0440883",
				CreatedOn:        "2026-07-27 08:39:48",
				UpdatedOn:        "2026-08-01 10:00:00",
				CreatedBy:        &entity.UserReference{Email: "customer@example.com", Name: "A Customer"},
				Subject:          &subject,
				Description:      &description,
				State:            "Work In Progress",
				Severity:         &severity,
				IssueType:        &issueType,
				Type:             "case",
				Project:          entity.EntityRef{ID: "proj-1", Name: "Customer Project"},
				Deployment:       &entity.EntityRef{ID: "dep-1", Name: "Primary Production"},
				DeployedProduct:  &entity.EntityRef{ID: "dp-1", Name: "WSO2 API Manager 4.5.0"},
				AssignedEngineer: &entity.AssignedEngineerRef{ID: "eng-1", Name: "Jane Engineer", Email: &assignee},
			},
		},
	}

	got := MapSearchCases(r)

	if got.TotalRecords != 878 {
		t.Fatalf("TotalRecords = %d, want 878", got.TotalRecords)
	}
	if len(got.Cases) != 1 {
		t.Fatalf("expected 1 case, got %d", len(got.Cases))
	}
	c := got.Cases[0]

	if c.Title == nil || *c.Title != "Login error" {
		t.Fatalf("Title = %v, want \"Login error\" (from entity-service's Subject field)", c.Title)
	}
	if c.UpdatedOn != "2026-08-01 10:00:00" {
		t.Fatalf("UpdatedOn = %q, want the entity-service updatedOn value", c.UpdatedOn)
	}
	if c.CreatedBy != "customer@example.com" {
		t.Fatalf("CreatedBy = %q, want the entity-service createdBy value", c.CreatedBy)
	}
	if c.Project.ID != "proj-1" || c.Project.Label != "Customer Project" {
		t.Fatalf("Project = %+v, want {id: proj-1, label: Customer Project}", c.Project)
	}
	if c.Status == nil || c.Status.Label != "Work In Progress" || c.Status.ID != "10" {
		t.Fatalf("Status = %+v, want {id: 10, label: Work In Progress}", c.Status)
	}
	if c.Severity == nil || c.Severity.Label != "High (P2)" || c.Severity.ID != "11" {
		t.Fatalf("Severity = %+v, want {id: 11, label: High (P2)}", c.Severity)
	}
	if c.IssueType == nil || c.IssueType.Label != "Question" || c.IssueType.ID != "4" {
		t.Fatalf("IssueType = %+v, want {id: 4, label: Question}", c.IssueType)
	}
	if c.AssignedEngineer == nil || c.AssignedEngineer.Label != "Jane Engineer" || c.AssignedEngineer.ID != "eng-1" {
		t.Fatalf("AssignedEngineer = %+v, want {id: eng-1, label: Jane Engineer}", c.AssignedEngineer)
	}
	if c.DeployedProduct == nil || c.DeployedProduct.Label != "WSO2 API Manager 4.5.0" {
		t.Fatalf("DeployedProduct = %+v, want label WSO2 API Manager 4.5.0", c.DeployedProduct)
	}
	if c.Deployment == nil || c.Deployment.Label != "Primary Production" {
		t.Fatalf("Deployment = %+v, want label Primary Production", c.Deployment)
	}
	if c.Type == nil || c.Type.ID != "default_case" {
		t.Fatalf("Type = %+v, want id \"default_case\" (entity-service's canonical \"case\" translated for the frontend's CaseType.DEFAULT_CASE)", c.Type)
	}
}

// TestCaseIDsToEnums_UnknownIDsSkipped verifies unrecognized numeric ids
// don't produce an empty-string enum entry.
func TestCaseIDsToEnums_UnknownIDsSkipped(t *testing.T) {
	got := caseIDsToEnums([]int{11, 999}, caseSeverityIDToEnum)
	want := []string{"high"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("caseIDsToEnums = %+v, want %+v", got, want)
	}
}
