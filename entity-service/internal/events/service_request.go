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

// Service request (SR) events.
//
// SRs belong to SRE, so these go on the operations topic (sre-events,
// SRE_EVENT_HUB_TOPIC) with the change-request and outage notices, not on
// the case topic. Each type states what happened to an SR; what to do about
// it -- which Chat space, which card -- is csm-notification-service's call.
// The structs are duplicated there by hand (separate Go modules); keep the
// two in step.
//
// They port ServiceNow's "SR New Request - Acknowledge & Chat Alert" flow
// (discovery scripts 73/74) and add the devops-sm customer-comment alert,
// which ServiceNow never had.
package events

const (
	// TypeSRCreated: a service request was created. Published for every SR.
	// When its SRE team is one this deployment automates
	// (SR_ALERT_SRE_TEAM_IDS), the SR is assigned to that team first, so
	// AssignmentGroupName already reflects it -- ServiceNow's card reads
	// "Assigned to <group>".
	TypeSRCreated Type = "sr.created"
	// TypeSRAcknowledged: the automatic acknowledgement comment was posted to
	// a new SR. Always follows TypeSRCreated for the same SR, on the same
	// partition key, so a consumer sees the two in order.
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
	// SRETeamID/SRETeamName are the SRE team of the SR's account
	// (account.sre_team_id) -- ServiceNow's account.u_sre_team, which its
	// flow routes on. Empty when the account has none.
	SRETeamID   string `json:"sreTeamId,omitempty"`
	SRETeamName string `json:"sreTeamName,omitempty"`
	// AssignmentGroupName is the SR's assignment group as it stands when the
	// event is published.
	AssignmentGroupName string `json:"assignmentGroupName,omitempty"`
}

// SRCreatedPayload is TypeSRCreated's payload.
type SRCreatedPayload struct {
	SRRef
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
	// AuthorEmail is who wrote it; whether that is a customer is the
	// consumer's call, the same way its frustration check decides it. Never
	// empty: a comment with no author email is not published.
	AuthorEmail string `json:"authorEmail"`
	AuthorName  string `json:"authorName,omitempty"`
	// Tags are the SR's tag names at the time of the comment, as stored --
	// not case-folded.
	Tags      []string `json:"tags"`
	CreatedOn string   `json:"createdOn"`
}
