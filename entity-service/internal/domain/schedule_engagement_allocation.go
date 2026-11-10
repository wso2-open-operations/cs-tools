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

// SearchScheduleEngagementAllocationsRequest finds the confirmed customer
// engagement allocations of the given people that overlap a date window.
// UserIDs and both dates are required: the read is for drawing an engineer's
// row, and an unbounded one would return every allocation ever made.
type SearchScheduleEngagementAllocationsRequest struct {
	UserIDs []string `json:"userIds"`
	From    string   `json:"from"`
	To      string   `json:"to"`
}

// ScheduleEngagementAllocation is one person's confirmed allocation to a
// customer engagement, as the team schedule draws it: a read-only bar from
// StartDate to EndDate on that person's row.
//
// Every field the engagement or its account may lack is optional and left out
// rather than invented.
type ScheduleEngagementAllocation struct {
	ID           string  `json:"id"`
	AllocationID *string `json:"allocationId,omitempty"`
	UserID       string  `json:"userId"`
	EngagementID string  `json:"engagementId"`
	// EngagementDisplayID is the engagement's human-readable id, not its uuid.
	EngagementDisplayID *string  `json:"engagementDisplayId,omitempty"`
	AccountName         *string  `json:"accountName,omitempty"`
	EngagementType      *string  `json:"engagementType,omitempty"`
	ModeOfDelivery      *string  `json:"modeOfDelivery,omitempty"`
	Utilization         *float64 `json:"utilization,omitempty"`
	StartDate           string   `json:"startDate"`
	EndDate             string   `json:"endDate"`
	// DismissedDates are the days in the window someone took this allocation
	// off the person's roster (YYYY-MM-DD, ascending). The roster leaves them
	// out and offers to restore them.
	DismissedDates []string `json:"dismissedDates,omitempty"`
}

// DismissScheduleEngagementAllocationRequest takes a synced allocation off a
// person's roster over From..To (weekdays only, cut to the allocation's own
// dates), or, with Restore, puts those days back. ID is the allocation's id as
// the search returns it (ScheduleEngagementAllocation.ID). TeamKey is the team
// whose roster the change is made on, which decides who may make it.
type DismissScheduleEngagementAllocationRequest struct {
	ID      string `json:"id"`
	TeamKey string `json:"teamKey"`
	From    string `json:"from"`
	To      string `json:"to"`
	Restore bool   `json:"restore,omitempty"`
}

// DismissScheduleEngagementAllocationResponse says how many days changed.
type DismissScheduleEngagementAllocationResponse struct {
	Days int `json:"days"`
}

// EngagementAllocationOnRoster is what a removal needs to know about the
// allocation: whose it is, its dates and its chip code.
type EngagementAllocationOnRoster struct {
	UserID    string
	StartDate string
	EndDate   string
	// Code is the engagement's display id, or "" when it has none.
	Code string
}

// ScheduleEngagementAllocationsResponse is the allocations over the window asked for.
type ScheduleEngagementAllocationsResponse struct {
	Allocations []ScheduleEngagementAllocation `json:"allocations"`
	Count       int                            `json:"count"`
}
