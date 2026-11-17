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

// SkillCapability is one task of the skills module that is not everyone's.
// Code checks a capability, never a role name: each resolves to the admin
// roles plus its own configured role list.
type SkillCapability string

const (
	// SkillCapDirectory: search everybody's skills and read a person's profile and history.
	SkillCapDirectory SkillCapability = "skills.directory.view"
	// SkillCapReviewRequests: the request queue, approve and reject.
	SkillCapReviewRequests SkillCapability = "skills.requests.review"
	// SkillCapManageCatalogue: create, edit and deactivate skills and categories.
	SkillCapManageCatalogue SkillCapability = "skills.catalogue.manage"
)

// SkillCapabilities is the body of GET /skills/me/capabilities: which of the
// gated tasks the caller holds, so the page builds its tabs from it.
// PendingRequestCount is set for a request reviewer.
type SkillCapabilities struct {
	CanViewDirectory    bool `json:"canViewDirectory"`
	CanReviewRequests   bool `json:"canReviewRequests"`
	CanManageCatalogue  bool `json:"canManageCatalogue"`
	PendingRequestCount *int `json:"pendingRequestCount,omitempty"`
}

// --- categories ---

// CreateSkillCategoryRequest is the body of POST /skills/categories.
type CreateSkillCategoryRequest struct {
	Name        string  `json:"name"`
	ParentID    *string `json:"parentId"`
	Description *string `json:"description"`
}

// UpdateSkillCategoryRequest is the body of PATCH /skills/categories/{id}.
// An absent field is left as it is.
type UpdateSkillCategoryRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Active      *bool   `json:"active"`
}

// --- skill requests ---

// SkillRequestStatus is where a request stands.
type SkillRequestStatus string

const (
	SkillRequestPending   SkillRequestStatus = "PENDING"
	SkillRequestApproved  SkillRequestStatus = "APPROVED"
	SkillRequestRejected  SkillRequestStatus = "REJECTED"
	SkillRequestWithdrawn SkillRequestStatus = "WITHDRAWN"
)

// Approval modes and resolutions.
const (
	SkillApproveCreate     = "create"
	SkillApproveReactivate = "reactivate"
	SkillApproveExisting   = "existing"

	SkillResolutionCreated     = "CREATED"
	SkillResolutionReactivated = "REACTIVATED"
	SkillResolutionExisting    = "EXISTING"
)

// What approving did to the requester's profile (skill_request.auto_add_note).
const (
	SkillAutoAddAdded         = "ADDED"
	SkillAutoAddReadded       = "READDED"
	SkillAutoAddAlreadyHeld   = "ALREADY_HELD"
	SkillAutoAddLevelAdjusted = "LEVEL_ADJUSTED"
	SkillAutoAddInactiveUser  = "SKIPPED_INACTIVE_USER"
	SkillAutoAddNoScale       = "SKIPPED_NO_SCALE"
)

// SkillPersonRef names a person on a request.
type SkillPersonRef struct {
	ID    string  `json:"id"`
	Name  *string `json:"name"`
	Email *string `json:"email"`
}

// SkillRequest is one request for a skill the catalogue lacks.
type SkillRequest struct {
	ID                string             `json:"id"`
	RequestedName     string             `json:"requestedName"`
	Description       *string            `json:"description"`
	Category          *SkillCategoryRef  `json:"category"`
	SuggestedCategory *string            `json:"suggestedCategory"`
	RequestedLevel    *SkillLevel        `json:"requestedLevel"`
	Status            SkillRequestStatus `json:"status"`
	RequestedBy       SkillPersonRef     `json:"requestedBy"`
	CreatedOn         time.Time          `json:"createdOn"`
	ReviewedBy        *SkillPersonRef    `json:"reviewedBy"`
	ReviewedOn        *time.Time         `json:"reviewedOn"`
	ReviewComment     *string            `json:"reviewComment"`
	Resolution        *string            `json:"resolution"`
	ResolvedSkill     *SkillRef          `json:"resolvedSkill"`
	AutoAddedLevel    *SkillLevel        `json:"autoAddedLevel"`
	AutoAddNote       *string            `json:"autoAddNote"`
}

// SkillRef names a skill.
type SkillRef struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

// CheckSkillRequestRequest is the body of POST /skills/requests/check: the
// duplicate preview the request form shows while the name is typed.
type CheckSkillRequestRequest struct {
	Name string `json:"name"`
}

// CheckSkillRequestResponse: an exact match (active or not), similar names,
// and a pending request for the same name, if any.
type CheckSkillRequestResponse struct {
	ExactMatch     *SkillRef     `json:"exactMatch"`
	Similar        []SkillRef    `json:"similar"`
	PendingRequest *SkillRequest `json:"pendingRequest"`
}

// CreateSkillRequestRequest is the body of POST /skills/requests. LevelID is
// required: it is the level the skill is added at when the request is approved.
type CreateSkillRequestRequest struct {
	Name              string  `json:"name"`
	CategoryID        *string `json:"categoryId"`
	SuggestedCategory *string `json:"suggestedCategory"`
	Description       *string `json:"description"`
	LevelID           string  `json:"levelId"`
}

// SearchSkillRequestsRequest is the body of POST /skills/requests/search.
type SearchSkillRequestsRequest struct {
	Statuses   []SkillRequestStatus `json:"statuses"`
	Pagination Pagination           `json:"pagination"`
}

// SkillRequestsResponse is a page of requests.
type SkillRequestsResponse struct {
	Requests []SkillRequest `json:"requests"`
	Total    int            `json:"total"`
	Limit    int            `json:"limit"`
	Offset   int            `json:"offset"`
}

// ApproveSkillRequestRequest is the body of POST /skills/requests/{id}/approve.
//   - mode "create": a new skill named Name (default: the requested name) in
//     CategoryID (default: the request's), on the default scale;
//   - mode "reactivate": SkillID is an inactive skill made active again;
//   - mode "existing": SkillID is an active skill that already covers it.
//
// LevelID overrides the level the requester gets (it must be on the approved
// skill's scale). Comment is shown to the requester.
type ApproveSkillRequestRequest struct {
	Mode        string  `json:"mode"`
	Name        *string `json:"name"`
	CategoryID  *string `json:"categoryId"`
	Description *string `json:"description"`
	Keywords    *string `json:"keywords"`
	SkillID     *string `json:"skillId"`
	LevelID     *string `json:"levelId"`
	Comment     *string `json:"comment"`
}

// RejectSkillRequestRequest is the body of POST /skills/requests/{id}/reject.
type RejectSkillRequestRequest struct {
	Reason string `json:"reason"`
}

// SkillApproval is what the repository needs to approve, already validated.
type SkillApproval struct {
	RequestID   string
	Mode        string
	Name        string
	CategoryID  *string
	Description *string
	Keywords    *string
	SkillID     string
	LevelID     *string
	Comment     *string
	ReviewerID  string
	Actor       string
}

// --- directory ---

// SkillCriterion is one "has skill X at level ≥ N" condition. MinLevelValue 0
// (or absent) means any level.
type SkillCriterion struct {
	SkillID       string `json:"skillId"`
	MinLevelValue int    `json:"minLevelValue"`
}

// SearchSkillDirectoryRequest is the body of POST /skills/directory/search.
// Match is ALL (every criterion) or ANY (at least one); ALL by default.
type SearchSkillDirectoryRequest struct {
	SearchQuery string           `json:"searchQuery"`
	TeamKeys    []string         `json:"teamKeys"`
	CategoryIDs []string         `json:"categoryIds"`
	Criteria    []SkillCriterion `json:"criteria"`
	Match       string           `json:"match"`
	Pagination  Pagination       `json:"pagination"`
}

// SkillDirectoryPerson is one person in the directory with all their active
// skills. MatchedCount is how many criteria they meet.
type SkillDirectoryPerson struct {
	UserID       string      `json:"userId"`
	UserName     *string     `json:"userName"`
	UserEmail    *string     `json:"userEmail"`
	TeamKeys     []string    `json:"teamKeys"`
	MatchedCount int         `json:"matchedCount"`
	Skills       []UserSkill `json:"skills"`
}

// SearchSkillDirectoryResponse is one row per person.
type SearchSkillDirectoryResponse struct {
	Items  []SkillDirectoryPerson `json:"items"`
	Total  int                    `json:"total"`
	Limit  int                    `json:"limit"`
	Offset int                    `json:"offset"`
}

// UserSkillProfileResponse is the body of GET /skills/users/{userId}.
type UserSkillProfileResponse struct {
	UserID    string      `json:"userId"`
	UserName  *string     `json:"userName"`
	UserEmail *string     `json:"userEmail"`
	TeamKeys  []string    `json:"teamKeys"`
	Skills    []UserSkill `json:"skills"`
}
