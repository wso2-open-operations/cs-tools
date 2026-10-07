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

package domain

// AllocationEvent is the allocation snapshot the Finance Entity posts (its types:Allocation
// record, as sent to ServiceNow's allocation_added_or_modified), plus customerCode.
type AllocationEvent struct {
	ID                 string                     `json:"id"`
	Email              string                     `json:"email"`
	AllocationType     int                        `json:"allocationType"`
	AllocationTypeName string                     `json:"allocationTypeName"`
	StartDate          string                     `json:"startDate"`
	EndDate            string                     `json:"endDate"`
	Comment            *string                    `json:"comment"`
	ClearanceStatus    *string                    `json:"clearanceStatus"`
	IsTravelRequired   *string                    `json:"isTravelRequired"`
	AddedBy            string                     `json:"addedBy"`
	IsRecurring        *string                    `json:"isRecurring"`
	StartTime          *string                    `json:"startTime"`
	EndTime            *string                    `json:"endTime"`
	TimeZone           *string                    `json:"timeZone"`
	ConsultantRole     *string                    `json:"consultantRole"`
	Status             *string                    `json:"status"`
	CustomerCode       *string                    `json:"customerCode"`
	Engagement         *AllocationEventEngagement `json:"engagement"`
}

// AllocationEventEngagement is the Finance Entity's EngagementDetails record.
type AllocationEventEngagement struct {
	Type               int     `json:"type"`
	EngagementTypeName string  `json:"engagementTypeName"`
	EngagementType     *string `json:"engagementType"`
	EngagementID       string  `json:"engagementId"`
	CustomerName       string  `json:"customerName"`
	OpportunityName    string  `json:"opportunityName"`
	ProductName        string  `json:"productName"`
	ProductID          *string `json:"productId"`
	EngagementCode     string  `json:"engagementCode"`
	EngagementNature   string  `json:"engagementNature"`
	Country            string  `json:"country"`
	CustomerCode       *string `json:"customerCode"`
}

// AllocationEventResultKind is what an allocation event did.
type AllocationEventResultKind string

const (
	AllocationEventCreated AllocationEventResultKind = "created"
	AllocationEventUpdated AllocationEventResultKind = "updated"
	AllocationEventSkipped AllocationEventResultKind = "skipped"
)

// AllocationEventResult answers POST /customer-engagements/allocation-events.
type AllocationEventResult struct {
	Result               AllocationEventResultKind `json:"result"`
	Reason               string                    `json:"reason,omitempty"`
	EngagementID         *string                   `json:"engagementId"`
	AllocationResourceID *string                   `json:"allocationResourceId"`
	EngagementCreated    bool                      `json:"engagementCreated"`
}

// NewCustomerEngagement is an engagement created from an allocation event.
type NewCustomerEngagement struct {
	EngagementID     string
	EngagementCode   *string
	Name             string
	AccountID        string
	IsPaid           bool
	DeliveryMode     *string
	EngagementType   string // a customer_engagement_type_enum label, e.g. "FIREFIGHTING"
	PlannedStartDate *string
	PlannedEndDate   *string
}

// AllocationResourceFields are the columns an allocation event writes on
// customer_engagement_allocation_resource.
type AllocationResourceFields struct {
	EngagementID string
	AllocationID string
	StartDate    *string
	EndDate      *string
	StartTime    *string
	EndTime      *string
	TimeZone     *string
	State        *string
}

// AccountCandidate is an account matched by name; Live is false for a soft-deleted row.
type AccountCandidate struct {
	ID   string
	Live bool
}
