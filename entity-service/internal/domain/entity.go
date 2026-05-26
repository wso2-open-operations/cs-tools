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
// SearchQuery is matched case-insensitively against first name, last name,
// username, and email. UserType narrows results to a single user class.
type SearchUsersRequest struct {
	Pagination  Pagination `json:"pagination"`
	SearchQuery string     `json:"searchQuery"`
	UserType    *UserType  `json:"userType,omitempty"`
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
