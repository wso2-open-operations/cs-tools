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

package domain

import "time"

// EngagementAllocationFilters are the optional criteria of an allocation
// search. Every field narrows the result; an empty one is no filter.
type EngagementAllocationFilters struct {
	// SearchQuery matches the allocation reference, the engagement's name and
	// its display id, case-insensitively.
	SearchQuery string `json:"searchQuery"`
	AccountID   string `json:"accountId"`
	// AccountName is a case-insensitive substring match on the account's name.
	AccountName string `json:"accountName"`
	// EngineerID is the platform user id of the allocated engineer.
	EngineerID   string `json:"engineerId"`
	EngagementID string `json:"engagementId"`
	// States are allocation states (e.g. CONFIRMED); a row matches any of them.
	States []string `json:"states"`
	// From and To (YYYY-MM-DD) keep allocations whose date span overlaps them,
	// both ends inclusive. Either may be given alone.
	From string `json:"from"`
	To   string `json:"to"`
}

// EngagementAllocationSortBy orders an allocation search. Field is one of
// startDate, endDate, state, allocationId, accountName, engineerName,
// utilization; Order is asc or desc. Unset, the newest start date comes first.
type EngagementAllocationSortBy struct {
	Field string `json:"field"`
	Order string `json:"order"`
}

// SearchEngagementAllocationsRequest is the body of
// POST /customer-engagements/allocations/search.
type SearchEngagementAllocationsRequest struct {
	Pagination Pagination                  `json:"pagination"`
	Filters    EngagementAllocationFilters `json:"filters"`
	SortBy     EngagementAllocationSortBy  `json:"sortBy"`
}

// EngagementAllocationPerson is a platform user as an allocation shows them.
type EngagementAllocationPerson struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Email *string `json:"email,omitempty"`
}

// EngagementAllocationAccount is the customer account an engagement is for.
type EngagementAllocationAccount struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// EngagementAllocationEngagement is the engagement an allocation belongs to.
// Fields the engagement lacks are left out rather than invented.
type EngagementAllocationEngagement struct {
	ID string `json:"id"`
	// DisplayID is the engagement's human-readable id, not its uuid.
	DisplayID        *string `json:"displayId,omitempty"`
	Name             *string `json:"name,omitempty"`
	Type             *string `json:"type,omitempty"`
	ModeOfDelivery   *string `json:"modeOfDelivery,omitempty"`
	State            *string `json:"state,omitempty"`
	BusinessUnit     *string `json:"businessUnit,omitempty"`
	IsPaid           *bool   `json:"isPaid,omitempty"`
	ProjectID        *string `json:"projectId,omitempty"`
	PlannedStartDate *string `json:"plannedStartDate,omitempty"`
	PlannedEndDate   *string `json:"plannedEndDate,omitempty"`
	ActualStartDate  *string `json:"actualStartDate,omitempty"`
	ActualEndDate    *string `json:"actualEndDate,omitempty"`
}

// EngagementAllocationView is one allocation of an engineer to a customer
// engagement, with the engagement, account and engineer it refers to.
type EngagementAllocationView struct {
	ID string `json:"id"`
	// AllocationID is the human-readable reference, not the uuid.
	AllocationID *string                        `json:"allocationId,omitempty"`
	State        *string                        `json:"state,omitempty"`
	StartDate    *string                        `json:"startDate,omitempty"`
	EndDate      *string                        `json:"endDate,omitempty"`
	StartTime    *string                        `json:"startTime,omitempty"`
	EndTime      *string                        `json:"endTime,omitempty"`
	Timezone     *string                        `json:"timezone,omitempty"`
	Utilization  *float64                       `json:"utilization,omitempty"`
	Engagement   EngagementAllocationEngagement `json:"engagement"`
	Account      *EngagementAllocationAccount   `json:"account,omitempty"`
	Engineer     *EngagementAllocationPerson    `json:"engineer,omitempty"`
	// LatestUpdateDate is when the newest published update was published.
	LatestUpdateDate *time.Time `json:"latestUpdateDate,omitempty"`
}

// SearchEngagementAllocationsResponse is one page of an allocation search.
type SearchEngagementAllocationsResponse struct {
	Allocations []EngagementAllocationView `json:"allocations"`
	Total       int                        `json:"total"`
	Limit       int                        `json:"limit"`
	Offset      int                        `json:"offset"`
	HasMore     bool                       `json:"hasMore"`
}

// EngagementAllocationUpdate is one status update written against an allocation.
type EngagementAllocationUpdate struct {
	ID             string                      `json:"id"`
	Subject        *string                     `json:"subject,omitempty"`
	Content        *string                     `json:"content,omitempty"`
	State          *string                     `json:"state,omitempty"`
	Visibility     *string                     `json:"visibility,omitempty"`
	CycleStartDate *string                     `json:"cycleStartDate,omitempty"`
	PublishedDate  *time.Time                  `json:"publishedDate,omitempty"`
	CreatedOn      time.Time                   `json:"createdOn"`
	Author         *EngagementAllocationPerson `json:"author,omitempty"`
}

// ListEngagementAllocationUpdatesResponse is one page of an allocation's
// updates, newest first.
type ListEngagementAllocationUpdatesResponse struct {
	Updates []EngagementAllocationUpdate `json:"updates"`
	Total   int                          `json:"total"`
	Limit   int                          `json:"limit"`
	Offset  int                          `json:"offset"`
	HasMore bool                         `json:"hasMore"`
}

// CreateEngagementAllocationUpdateRequest is the body of
// POST /customer-engagements/allocations/{id}/updates. Everything else about
// the update (author, state, visibility, cycle) is the server's to set.
type CreateEngagementAllocationUpdateRequest struct {
	Subject string `json:"subject"`
	Content string `json:"content"`
}
