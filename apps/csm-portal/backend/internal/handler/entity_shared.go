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

import "fmt"

// entity-service wire types shared by the SPL Postgres clients that remain
// (Reports, Usage Metrics, Lookups, User Scan) after Accounts/Projects/Cases
// reads merged onto CS Portal's own /accounts, /projects, and /cases routes
// -- this backend's own copy of entity-service's JSON contract, since
// entity-service is a separate Go module and its domain types can't be
// imported directly (every existing entity-service caller in this codebase
// already works this way, see internal/entity/customer.go). Field
// names/JSON tags mirror entity-service's internal/domain package exactly.

type entityPagination struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

type entityRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type entityUserReference struct {
	ID    *string `json:"id"`
	Email string  `json:"email"`
	Name  string  `json:"name"`
}

// entityAccountRef mirrors entity-service's own domain.AccountRef exactly
// (id/name/type only -- AccountRef has no "number" field).
type entityAccountRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type entityCaseFieldFilter struct {
	Field  string   `json:"field"`
	Op     string   `json:"op"`
	Values []string `json:"values,omitempty"`
}

type entitySearchCasesFilters struct {
	SearchQuery string                  `json:"searchQuery,omitempty"`
	Filters     []entityCaseFieldFilter `json:"filters,omitempty"`
}

type entityCaseSort struct {
	Field string `json:"field"`
	Order string `json:"order"`
}

type entitySearchCasesRequest struct {
	Filters    entitySearchCasesFilters `json:"filters"`
	SortBy     entityCaseSort           `json:"sortBy"`
	Pagination entityPagination         `json:"pagination"`
}

type entitySearchCaseView struct {
	ID               string               `json:"id"`
	InternalID       string               `json:"internalId"`
	Number           string               `json:"number"`
	CreatedOn        string               `json:"createdOn"`
	CreatedBy        *entityUserReference `json:"createdBy"`
	Subject          *string              `json:"subject"`
	Description      *string              `json:"description"`
	State            *string              `json:"state"`
	Product          *entityRef           `json:"product"`
	Project          *entityRef           `json:"project"`
	ProjectKey       *string              `json:"projectKey"`
	AssignedEngineer *entityUserReference `json:"assignedEngineer"`
	AccountDetails   *entityAccountRef    `json:"account"`
}

type entitySearchCasesResponse struct {
	Cases []entitySearchCaseView `json:"cases"`
	Total int                    `json:"total"`
}

type entityProjectView struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Key       string  `json:"key"`
	StartDate *string `json:"startDate"`
	EndDate   *string `json:"endDate"`
	// ClosureState mirrors entity-service's own ProjectView (embedded
	// ProjectClosureFields) -- project.wso2_closure_state, populated on
	// both its data sources.
	ClosureState *string `json:"closureState"`
}

type entitySearchProjectsRequest struct {
	Pagination  entityPagination `json:"pagination"`
	SearchQuery string           `json:"searchQuery,omitempty"`
	AccountID   string           `json:"accountId,omitempty"`
}

type entitySearchProjectsResponse struct {
	Projects []entityProjectView `json:"projects"`
	Total    int                 `json:"total"`
}

type entityProjectAccountRef struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Number string `json:"number"`
}

type entityProjectDetailsView struct {
	ID                  string                  `json:"id"`
	Account             entityProjectAccountRef `json:"account"`
	Name                string                  `json:"name"`
	Key                 string                  `json:"key"`
	SubscriptionType    string                  `json:"subscriptionType"`
	StartDate           *string                 `json:"startDate"`
	EndDate             *string                 `json:"endDate"`
	ClosureState        *string                 `json:"closureState"`
	TotalQueryHours     *float64                `json:"totalQueryHours"`
	RemainingQueryHours *float64                `json:"remainingQueryHours"`
}

// caseStateToDisplay translates entity-service's domain.CaseState wire
// values (lowercase snake_case, e.g. "work_in_progress") to the six display
// labels SPL's UI has always used, which are exactly the backing system's own state
// labels (e.g. "Work In Progress") -- see CaseStateCard.tsx/CasesPage.tsx
// on the frontend, which are NOT changing as part of this.
var caseStateToDisplay = map[string]string{
	"open":              "Open",
	"work_in_progress":  "Work In Progress",
	"awaiting_info":     "Awaiting Info",
	"solution_proposed": "Solution Proposed",
	"waiting_on_wso2":   "Waiting on WSO2",
	"reopened":          "Reopened",
	"closed":            "Closed",
}

// derefStr returns "" for a nil pointer, matching servicenow.CaseDetails's
// own convention of empty-string-not-omitted for a field the backing system itself
// never leaves absent.
func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func hoursOrEmpty(f *float64) string {
	if f == nil {
		return ""
	}
	return fmt.Sprintf("%g", *f)
}
