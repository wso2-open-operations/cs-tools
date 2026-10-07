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

package events

// Service request (SR) events, published on the operations topic
// (sre-events, SRE_EVENT_HUB_TOPIC) by entity-service and read here through
// the same HandleShared consumer as the change-request and outage notices.
// Kept in sync by hand with entity-service's internal/events/service_request.go
// (separate Go modules) -- that file is the source of truth for the wire
// shape. EntityID is the SR's caseId, so all three for one SR share a
// partition and arrive in order.
//
// They port ServiceNow's "SR New Request - Acknowledge & Chat Alert" flow and
// add the devops-sm customer-comment alert, which ServiceNow never had. Every
// reaction is a Google Chat card to the SR's SRE team space -- see
// dispatch.handleSRCreated.
const (
	// TypeSRCreated: a service request was created. Published for every SR;
	// when entity-service automates its SRE team (SR_ALERT_SRE_TEAM_IDS) the
	// SR is assigned to that team first, so AssignmentGroupName reflects it.
	TypeSRCreated Type = "sr.created"
	// TypeSRAcknowledged: the automatic acknowledgement comment was posted to
	// a new SR. Always follows TypeSRCreated for the same SR.
	TypeSRAcknowledged Type = "sr.acknowledged"
	// TypeSRCommentAdded: a comment or work note was added to an SR.
	TypeSRCommentAdded Type = "sr.comment_added"
)

// SRRef is what every SR event carries about the SR itself.
type SRRef struct {
	CaseID     string `json:"caseId"`
	Number     string `json:"number"`
	WSO2CaseID string `json:"wso2CaseId,omitempty"`
	Subject    string `json:"subject"`
	// SRETeamID/SRETeamName are the SRE team of the SR's account. Empty when
	// the account has none. SRETeamName is the Chat audience every SR card
	// routes to (a GOOGLE_CHAT_SPACES key).
	SRETeamID   string `json:"sreTeamId,omitempty"`
	SRETeamName string `json:"sreTeamName,omitempty"`
	// AssignmentGroupName is the SR's assignment group as it stands when the
	// event is published.
	AssignmentGroupName string `json:"assignmentGroupName,omitempty"`
}

// SRCreatedPayload is TypeSRCreated's payload.
type SRCreatedPayload struct {
	SRRef
	// Description is the SR's description as stored -- may be rich-text
	// HTML; the Chat card strips it to plain text.
	Description string `json:"description,omitempty"`
	// State is the SR's state label as the portal shows it, e.g. "Open".
	State       string `json:"state"`
	ProjectID   string `json:"projectId,omitempty"`
	ProjectName string `json:"projectName,omitempty"`
	CreatedBy   string `json:"createdBy,omitempty"`
	CreatedOn   string `json:"createdOn"`
}

// SRAcknowledgedPayload is TypeSRAcknowledged's payload.
type SRAcknowledgedPayload struct {
	SRRef
	// CommentID is the acknowledgement comment's id.
	CommentID string `json:"commentId"`
}

// SRCommentType is which journal a TypeSRCommentAdded entry went to.
type SRCommentType string

const (
	SRCommentTypeComment  SRCommentType = "comment"
	SRCommentTypeWorkNote SRCommentType = "work_note"
)

// SRCommentAddedPayload is TypeSRCommentAdded's payload.
type SRCommentAddedPayload struct {
	SRRef
	CommentID   string        `json:"commentId"`
	CommentType SRCommentType `json:"commentType"`
	Content     string        `json:"content"`
	// AuthorEmail is who wrote it; whether that is a customer is decided
	// here (linkResolver.IsCustomer), the same way checkFrustration decides
	// it. Never logged.
	AuthorEmail string `json:"authorEmail"`
	AuthorName  string `json:"authorName,omitempty"`
	// Tags are the SR's tag names at the time of the comment, as stored --
	// not case-folded; dispatch compares them case-insensitively.
	Tags      []string `json:"tags"`
	CreatedOn string   `json:"createdOn"`
}
