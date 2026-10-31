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

// Standard change templates (migration 0252). A standard change is pre-authorised:
// the CAB assesses its risk once, when the template is created or changed (a
// StandardChangeProposal), and a change request raised from an active template is
// scheduled without any approval of its own. What the CAB approved -- the plans,
// the risk, the customer gates -- is frozen on every change request raised from it;
// only the declared parameters, the schedule and the scope vary per use.

// StandardChangeTemplateState is a template's lifecycle state.
type StandardChangeTemplateState string

const (
	// StandardChangeTemplateStateActive templates can be used.
	StandardChangeTemplateStateActive StandardChangeTemplateState = "active"
	// StandardChangeTemplateStateSuspended templates are kept but cannot be used
	// until resumed.
	StandardChangeTemplateStateSuspended StandardChangeTemplateState = "suspended"
	// StandardChangeTemplateStateRetired is final, reached only through an approved
	// retirement proposal.
	StandardChangeTemplateStateRetired StandardChangeTemplateState = "retired"
)

// StandardChangeProposalKind says what a proposal asks the CAB to authorise.
type StandardChangeProposalKind string

const (
	StandardChangeProposalKindNew    StandardChangeProposalKind = "new"
	StandardChangeProposalKindModify StandardChangeProposalKind = "modify"
	StandardChangeProposalKindRetire StandardChangeProposalKind = "retire"
)

// StandardChangeProposalState is a proposal's lifecycle state. Approved, rejected
// and withdrawn are final; a proposal is never deleted, so a rejected one stays as
// the record of what was proposed and why it was refused.
type StandardChangeProposalState string

const (
	StandardChangeProposalStateDraft     StandardChangeProposalState = "draft"
	StandardChangeProposalStateCABReview StandardChangeProposalState = "cab_review"
	StandardChangeProposalStateApproved  StandardChangeProposalState = "approved"
	StandardChangeProposalStateRejected  StandardChangeProposalState = "rejected"
	StandardChangeProposalStateWithdrawn StandardChangeProposalState = "withdrawn"
)

// StandardChangeParameterType is the input a parameter takes.
type StandardChangeParameterType string

const (
	// Text is one line, up to 500 characters.
	StandardChangeParameterTypeText StandardChangeParameterType = "text"
	// Textarea is free text over several lines, up to 4000 characters.
	StandardChangeParameterTypeTextarea StandardChangeParameterType = "textarea"
	// Number is a decimal number.
	StandardChangeParameterTypeNumber StandardChangeParameterType = "number"
	// Date is a calendar date, YYYY-MM-DD.
	StandardChangeParameterTypeDate StandardChangeParameterType = "date"
	// Select is one of Options.
	StandardChangeParameterTypeSelect StandardChangeParameterType = "select"
)

// StandardChangeParameter is one value a template leaves open. The template's text
// refers to it as {{key}}; the value given when the change request is raised is
// put in its place (HTML-escaped in the rich-text fields).
type StandardChangeParameter struct {
	// Key is lower case letters, digits and underscores, starting with a letter.
	Key      string                      `json:"key"`
	Label    string                      `json:"label"`
	Type     StandardChangeParameterType `json:"type"`
	Required bool                        `json:"required"`
	HelpText *string                     `json:"helpText,omitempty"`
	// Options are the choices of a select parameter, in the order shown.
	Options []string `json:"options,omitempty"`
}

// StandardChangeContent is what a template fixes on every change request raised
// from it. On a proposal (a draft) any part may still be missing; on a version
// the required parts are always set.
type StandardChangeContent struct {
	// Subject becomes the change request's title (plain text).
	Subject *string `json:"subject"`
	// The rich-text fields (HTML), as on a change request.
	Description        *string `json:"description"`
	Justification      *string `json:"justification"`
	ImplementationPlan *string `json:"implementationPlan"`
	RiskImpactAnalysis *string `json:"riskImpactAnalysis"`
	BackoutPlan        *string `json:"backoutPlan"`
	TestPlan           *string `json:"testPlan"`

	Category             *ChangeRequestCategory `json:"category"`
	Impact               *ChangeRequestImpact   `json:"impact"`
	Risk                 *ChangeRequestRisk     `json:"risk"`
	RollbackDurationText *string                `json:"rollbackDurationText"`

	IsPlanningVisibleToCustomers bool `json:"isPlanningVisibleToCustomers"`
	CustomerApprovalRequired     bool `json:"customerApprovalRequired"`
	CustomerReviewRequired       bool `json:"customerReviewRequired"`

	Parameters []StandardChangeParameter `json:"parameters"`
}

// StandardChangeTemplateUsage counts the change requests raised from a template,
// all versions together.
type StandardChangeTemplateUsage struct {
	Total int `json:"total"`
	// Open are those not yet closed, canceled or rolled back.
	Open       int        `json:"open"`
	Closed     int        `json:"closed"`
	RolledBack int        `json:"rolledBack"`
	Canceled   int        `json:"canceled"`
	LastUsedOn *time.Time `json:"lastUsedOn"`
}

// StandardChangeTemplateVersion is one CAB-approved version of a template.
// Immutable once written.
type StandardChangeTemplateVersion struct {
	ID         string `json:"id"`
	TemplateID string `json:"templateId"`
	Version    int    `json:"version"`
	// Proposal is the proposal it was approved from.
	Proposal   *EntityRef `json:"proposal"`
	ApprovedOn time.Time  `json:"approvedOn"`
	ApprovedBy string     `json:"approvedBy"`
	StandardChangeContent
}

// StandardChangeTemplateVersionSummary lists a version without its content.
type StandardChangeTemplateVersionSummary struct {
	ID         string     `json:"id"`
	Version    int        `json:"version"`
	Proposal   *EntityRef `json:"proposal"`
	ApprovedOn time.Time  `json:"approvedOn"`
	ApprovedBy string     `json:"approvedBy"`
}

// StandardChangeTemplate is a template of the catalogue.
type StandardChangeTemplate struct {
	ID     string                      `json:"id"`
	Number string                      `json:"number"`
	Name   string                      `json:"name"`
	State  StandardChangeTemplateState `json:"state"`
	// Customer is the account the template belongs to (migration 0254): a change
	// request is raised from it only for a project of this account, and its name
	// is unique among this customer's live templates. Null only on a template
	// written before 0254 whose customer could not be told.
	Customer *EntityRef     `json:"customer"`
	Owner    *UserReference `json:"owner"`
	// CurrentVersion is the version a new change request follows.
	CurrentVersion *StandardChangeTemplateVersion `json:"currentVersion"`
	// ReviewRequired is raised when a change request raised from the template was
	// rolled back or deviated from its implementation plan. It does not stop the
	// template being used: the owner and the CAB decide what to do.
	ReviewRequired              bool       `json:"reviewRequired"`
	ReviewRequiredReason        *string    `json:"reviewRequiredReason"`
	ReviewRequiredOn            *time.Time `json:"reviewRequiredOn"`
	ReviewRequiredChangeRequest *EntityRef `json:"reviewRequiredChangeRequest"`
	// SuspendedReason, SuspendedOn and SuspendedBy (an email) record the suspension;
	// all null unless the template is suspended. The reason is optional, so it may
	// be null on a suspended template.
	SuspendedReason *string    `json:"suspendedReason"`
	SuspendedOn     *time.Time `json:"suspendedOn"`
	SuspendedBy     *string    `json:"suspendedBy"`
	// OpenProposal is the modification or retirement now in draft or CAB review,
	// if any (there is at most one).
	OpenProposal *EntityRef                  `json:"openProposal"`
	Usage        StandardChangeTemplateUsage `json:"usage"`
	CreatedOn    time.Time                   `json:"createdOn"`
	CreatedBy    string                      `json:"createdBy"`
	UpdatedOn    time.Time                   `json:"updatedOn"`
	UpdatedBy    string                      `json:"updatedBy"`
	// Versions is filled on GET /standard-change-templates/{id} only, newest first.
	Versions []StandardChangeTemplateVersionSummary `json:"versions,omitempty"`
}

// SearchStandardChangeTemplatesRequest is the body of POST /standard-change-templates/search.
type SearchStandardChangeTemplatesRequest struct {
	// States narrows the list; empty means active and suspended.
	States []StandardChangeTemplateState `json:"states,omitempty"`
	// Query matches the name or number, ignoring case.
	Query *string `json:"query,omitempty"`
	// AccountID narrows the list to one customer's templates.
	AccountID  *string    `json:"accountId,omitempty"`
	Pagination Pagination `json:"pagination"`
}

// SearchStandardChangeTemplatesResponse is the answer of POST /standard-change-templates/search.
type SearchStandardChangeTemplatesResponse struct {
	Templates []StandardChangeTemplate `json:"templates"`
	Total     int                      `json:"total"`
	Limit     int                      `json:"limit"`
	Offset    int                      `json:"offset"`
}

// SuspendStandardChangeTemplateRequest is the optional body of POST
// /standard-change-templates/{id}/suspend (an empty body is the same as {}).
type SuspendStandardChangeTemplateRequest struct {
	// Reason says why the template is suspended: optional, blank means none, at
	// most 1000 characters.
	Reason string `json:"reason"`
}

// StandardChangeEvidence is a completed change request a proposal cites as
// evidence that the change is repeatable.
type StandardChangeEvidence struct {
	ID       string     `json:"id"`
	Number   string     `json:"number"`
	Subject  string     `json:"subject"`
	State    *string    `json:"state"`
	ClosedOn *time.Time `json:"closedOn"`
	// Customer is the change request's customer (its project's account) now.
	Customer *EntityRef `json:"customer"`
}

// StandardChangeProposalApprover is one CAB member asked to decide a proposal.
type StandardChangeProposalApprover struct {
	ID       string         `json:"id"`
	Approver *UserReference `json:"approver"`
	// State is the approver row's state, UPPER_SNAKE (REQUESTED, APPROVED,
	// REJECTED, CANCELLED), as on a change request's approvals.
	State     string    `json:"state"`
	Comments  *string   `json:"comments"`
	UpdatedOn time.Time `json:"updatedOn"`
	// CanDecide is true on the caller's own REQUESTED row while the proposal is
	// in CAB review.
	CanDecide bool `json:"canDecide"`
}

// StandardChangeProposal is a proposal to create, modify or retire a template.
type StandardChangeProposal struct {
	ID     string                      `json:"id"`
	Number string                      `json:"number"`
	Kind   StandardChangeProposalKind  `json:"kind"`
	State  StandardChangeProposalState `json:"state"`
	// Name is the template's name (the work item's subject).
	Name string `json:"name"`
	// Customer is the account the proposal is for: what a NEW proposal names (or
	// takes from the change request it starts from, or the proposal it copies),
	// the template's for a modification or retirement. Null on a new draft that
	// has not named one yet; such a draft cannot be submitted.
	Customer *EntityRef `json:"customer"`
	// Rationale says why this change is low risk and repeatable enough to be
	// pre-authorised (or, for a retirement, why it should go).
	Rationale *string `json:"rationale"`
	// Template is the template acted on (modify, retire), or the one an approved
	// new proposal created.
	Template *EntityRef `json:"template"`
	// BaseVersion is the version a modification was started from.
	BaseVersion         *int       `json:"baseVersion"`
	SourceChangeRequest *EntityRef `json:"sourceChangeRequest"`
	// CopiedFrom is the proposal a new draft was copied from (id and number), null
	// for one that was not copied. A copy is for the copied proposal's customer:
	// only a new draft started from nothing (no source change request, not a
	// copy) may change its customer.
	CopiedFrom *EntityRef `json:"copiedFrom"`
	StandardChangeContent
	Evidence     []StandardChangeEvidence         `json:"evidence"`
	Approvers    []StandardChangeProposalApprover `json:"approvers"`
	ProposedBy   *UserReference                   `json:"proposedBy"`
	CreatedOn    time.Time                        `json:"createdOn"`
	UpdatedOn    time.Time                        `json:"updatedOn"`
	SubmittedOn  *time.Time                       `json:"submittedOn"`
	DecidedOn    *time.Time                       `json:"decidedOn"`
	DecidedBy    *UserReference                   `json:"decidedBy"`
	DecisionNote *string                          `json:"decisionComment"`
	// ResultingVersion is the version an approved new or modify proposal wrote.
	ResultingVersion *int `json:"resultingVersion"`
	// What the caller may do now (advisory; every action re-checks).
	CanEdit     bool `json:"canEdit"`
	CanSubmit   bool `json:"canSubmit"`
	CanWithdraw bool `json:"canWithdraw"`
}

// CreateStandardChangeProposalRequest is the body of POST /standard-change-proposals.
// A new proposal may start from a completed change request (its content is copied
// and it becomes the first piece of evidence) or empty; a modification starts from
// the template's current version, a retirement from the template.
type CreateStandardChangeProposalRequest struct {
	Kind                  StandardChangeProposalKind `json:"kind"`
	TemplateID            *string                    `json:"templateId,omitempty"`
	SourceChangeRequestID *string                    `json:"sourceChangeRequestId,omitempty"`
	// CopyFromProposalID starts a new draft from another proposal's content (for
	// example one the CAB rejected). The draft is new in every other respect.
	CopyFromProposalID *string `json:"copyFromProposalId,omitempty"`
	Name               *string `json:"name,omitempty"`
	// AccountID is the customer a NEW proposal is for. One started from a change
	// request takes that change request's customer, one copied from a proposal
	// takes that proposal's, and a modification or retirement its template's: a
	// different accountId is then a 400. Optional on a NEW proposal started from
	// nothing (a draft may name it later), required to submit it.
	AccountID *string `json:"accountId,omitempty"`
}

// UpdateStandardChangeProposalRequest is the body of PATCH /standard-change-proposals/{id}.
// Only a draft can be edited, only by its proposer. Absent fields are left as
// they are; Parameters and EvidenceChangeRequestIDs replace the whole list. An
// empty Category, Impact or Risk clears it.
type UpdateStandardChangeProposalRequest struct {
	Name *string `json:"name,omitempty"`
	// AccountID sets the customer of a NEW draft that did not start from a change
	// request; the evidence it cites must then be that customer's. A draft that
	// started from a change request, and a modification or retirement, keep the
	// customer they have (resending it is accepted, another is a 400).
	AccountID                    *string                    `json:"accountId,omitempty"`
	Rationale                    *string                    `json:"rationale,omitempty"`
	Subject                      *string                    `json:"subject,omitempty"`
	Description                  *string                    `json:"description,omitempty"`
	Justification                *string                    `json:"justification,omitempty"`
	ImplementationPlan           *string                    `json:"implementationPlan,omitempty"`
	RiskImpactAnalysis           *string                    `json:"riskImpactAnalysis,omitempty"`
	BackoutPlan                  *string                    `json:"backoutPlan,omitempty"`
	TestPlan                     *string                    `json:"testPlan,omitempty"`
	Category                     *ChangeRequestCategory     `json:"category,omitempty"`
	Impact                       *ChangeRequestImpact       `json:"impact,omitempty"`
	Risk                         *ChangeRequestRisk         `json:"risk,omitempty"`
	RollbackDurationText         *string                    `json:"rollbackDurationText,omitempty"`
	IsPlanningVisibleToCustomers *bool                      `json:"isPlanningVisibleToCustomers,omitempty"`
	CustomerApprovalRequired     *bool                      `json:"customerApprovalRequired,omitempty"`
	CustomerReviewRequired       *bool                      `json:"customerReviewRequired,omitempty"`
	Parameters                   *[]StandardChangeParameter `json:"parameters,omitempty"`
	EvidenceChangeRequestIDs     *[]string                  `json:"evidenceChangeRequestIds,omitempty"`
}

// DecideStandardChangeProposalRequest is the body of POST /standard-change-proposals/{id}/decision.
type DecideStandardChangeProposalRequest struct {
	// Decision is "approved" or "rejected".
	Decision string `json:"decision"`
	// Comment is required for a rejection.
	Comment *string `json:"comment,omitempty"`
}

// SearchStandardChangeProposalsRequest is the body of POST /standard-change-proposals/search.
type SearchStandardChangeProposalsRequest struct {
	States     []StandardChangeProposalState `json:"states,omitempty"`
	Kinds      []StandardChangeProposalKind  `json:"kinds,omitempty"`
	TemplateID *string                       `json:"templateId,omitempty"`
	// AccountID narrows the list to the proposals for one customer.
	AccountID *string `json:"accountId,omitempty"`
	// ProposedByMe narrows to the caller's own proposals.
	ProposedByMe bool `json:"proposedByMe,omitempty"`
	// AwaitingMyDecision narrows to proposals in CAB review on which the caller
	// holds a pending approval.
	AwaitingMyDecision bool       `json:"awaitingMyDecision,omitempty"`
	Query              *string    `json:"query,omitempty"`
	Pagination         Pagination `json:"pagination"`
}

// StandardChangeProposalSummary is one row of a proposal search.
type StandardChangeProposalSummary struct {
	ID          string                      `json:"id"`
	Number      string                      `json:"number"`
	Kind        StandardChangeProposalKind  `json:"kind"`
	State       StandardChangeProposalState `json:"state"`
	Name        string                      `json:"name"`
	Customer    *EntityRef                  `json:"customer"`
	Template    *EntityRef                  `json:"template"`
	ProposedBy  *UserReference              `json:"proposedBy"`
	CreatedOn   time.Time                   `json:"createdOn"`
	UpdatedOn   time.Time                   `json:"updatedOn"`
	SubmittedOn *time.Time                  `json:"submittedOn"`
	DecidedOn   *time.Time                  `json:"decidedOn"`
}

// SearchStandardChangeProposalsResponse is the answer of POST /standard-change-proposals/search.
type SearchStandardChangeProposalsResponse struct {
	Proposals []StandardChangeProposalSummary `json:"proposals"`
	Total     int                             `json:"total"`
	Limit     int                             `json:"limit"`
	Offset    int                             `json:"offset"`
}

// ChangeRequestStandardChange is what a change request raised from a template
// carries about it (staff only).
type ChangeRequestStandardChange struct {
	Template EntityRef `json:"template"`
	// TemplateNumber is the template's STD number.
	TemplateNumber string `json:"templateNumber"`
	// Customer is the template's customer: the account of this change request's
	// project (a template is used only for its own customer's projects).
	Customer *EntityRef `json:"customer"`
	// CustomerMismatch is true when the change request's project is not (any
	// more) a project of the template's customer: the Salesforce project sync,
	// the source of truth for projects, moved the project to another account
	// after the change was raised. Request Approval is then refused (400
	// standard_change_template_other_customer); the UI warns.
	CustomerMismatch bool   `json:"customerMismatch"`
	VersionID        string `json:"versionId"`
	Version          int    `json:"version"`
	// Parameters are the template version's parameters with the values this
	// change request was raised with.
	Parameters []ChangeRequestStandardChangeParameter `json:"parameters"`
}

// ChangeRequestStandardChangeParameter is a parameter and its value on one
// change request.
type ChangeRequestStandardChangeParameter struct {
	StandardChangeParameter
	Value *string `json:"value"`
}
