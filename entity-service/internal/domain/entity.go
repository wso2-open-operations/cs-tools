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

// Package domain defines the core data types shared across all layers of the
// entity service. No business logic lives here — only plain structs and enums.
package domain

import "time"

// UserType classifies a user's role within the system.
type UserType string

const (
	// UserTypeInternal identifies WSO2 or partner staff users.
	UserTypeInternal UserType = "internal"
	// UserTypeCustomer identifies end-customer users.
	UserTypeCustomer UserType = "customer"
)

// User represents a single user entity as stored in the database.
// Phone and Timezone are optional and omitted from JSON when absent.
type User struct {
	ID        string    `json:"id"`
	UserName  string    `json:"userName"`
	FirstName string    `json:"firstName"`
	LastName  string    `json:"lastName"`
	Email     string    `json:"email"`
	Phone     *string   `json:"phone,omitempty"`
	Timezone  *string   `json:"timezone,omitempty"`
	UserType  UserType  `json:"userType"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Pagination controls which page of results is returned.
// Limit defaults to 20 and is capped at 100 by the service layer.
type Pagination struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// SearchUsersRequest is the input for a user search operation.
// SearchQuery is matched case-insensitively against username and email.
type SearchUsersRequest struct {
	Pagination  Pagination `json:"pagination"`
	SearchQuery string     `json:"searchQuery"`
}

// SearchUsersResponse is the paginated result of a user search.
// HasMore is true when additional pages are available beyond the current offset.
type SearchUsersResponse struct {
	Users   []User `json:"users"`
	Total   int    `json:"total"`
	Limit   int    `json:"limit"`
	Offset  int    `json:"offset"`
	HasMore bool   `json:"hasMore"`
}

// AccountTier represents the subscription tier of an account.
type AccountTier string

const (
	// AccountTierBasic is the standard subscription tier.
	AccountTierBasic AccountTier = "basic"
	// AccountTierEnterprise is the premium subscription tier.
	AccountTierEnterprise AccountTier = "enterprise"
)

// Account represents a customer account as stored in the database.
// TechnicalOwnerID, Region, and DeactivationDate are optional.
type Account struct {
	ID                  string      `json:"id"`
	SfID                string      `json:"sfId"`
	Name                string      `json:"name"`
	Tier                AccountTier `json:"tier"`
	Region              *string     `json:"region,omitempty"`
	ActivationDate      time.Time   `json:"activationDate"`
	DeactivationDate    *time.Time  `json:"deactivationDate,omitempty"`
	OwnerID             string      `json:"ownerId"`
	TechnicalOwnerID    *string     `json:"technicalOwnerId,omitempty"`
	AgentEnabled        bool        `json:"agentEnabled"`
	KbReferencesEnabled bool        `json:"kbReferencesEnabled"`
	CreatedAt           time.Time   `json:"createdAt"`
	UpdatedAt           time.Time   `json:"updatedAt"`
}

// SearchAccountsRequest is the input for an account search operation.
// SearchQuery is matched case-insensitively against name and sf_id.
type SearchAccountsRequest struct {
	Pagination  Pagination `json:"pagination"`
	SearchQuery string     `json:"searchQuery"`
}

// SearchAccountsResponse is the paginated result of an account search.
// HasMore is true when additional pages are available beyond the current offset.
type SearchAccountsResponse struct {
	Accounts []Account `json:"accounts"`
	Total    int       `json:"total"`
	Limit    int       `json:"limit"`
	Offset   int       `json:"offset"`
	HasMore  bool      `json:"hasMore"`
}

// SubscriptionType classifies the subscription type of a project.
type SubscriptionType string

const (
	SubscriptionTypeDevelopmentSupport       SubscriptionType = "development_support"
	SubscriptionTypeManagedCloudSubscription SubscriptionType = "managed_cloud_subscription"
	SubscriptionTypeEvaluationSubscription   SubscriptionType = "evaluation_subscription"
	SubscriptionTypeSubscription             SubscriptionType = "subscription"
	SubscriptionTypeCloudEvaluationSupport   SubscriptionType = "cloud_evaluation_support"
	SubscriptionTypeInternal                 SubscriptionType = "internal"
	SubscriptionTypePlatformerSubscription   SubscriptionType = "platformer_subscription"
	SubscriptionTypeCloudSupport             SubscriptionType = "cloud_support"
	SubscriptionTypeProfessionalServices     SubscriptionType = "professional_services"
)

// Project represents a customer project linked to an account.
type Project struct {
	ID               string           `json:"id"`
	AccountID        string           `json:"accountId"`
	SfID             string           `json:"sfId"`
	Name             string           `json:"name"`
	ProjectKey       string           `json:"projectKey"`
	SubscriptionType SubscriptionType `json:"subscriptionType"`
	StartDate        time.Time        `json:"startDate"`
	EndDate          time.Time        `json:"endDate"`
	CreatedAt        time.Time        `json:"createdAt"`
	UpdatedAt        time.Time        `json:"updatedAt"`
}

// SearchProjectsRequest is the input for a project search operation.
// SearchQuery is matched case-insensitively against name, project_key, and subscription_type.
type SearchProjectsRequest struct {
	Pagination  Pagination `json:"pagination"`
	SearchQuery string     `json:"searchQuery"`
}

// SearchProjectsResponse is the paginated result of a project search.
type SearchProjectsResponse struct {
	Projects []Project `json:"projects"`
	Total    int       `json:"total"`
	Limit    int       `json:"limit"`
	Offset   int       `json:"offset"`
	HasMore  bool      `json:"hasMore"`
}
