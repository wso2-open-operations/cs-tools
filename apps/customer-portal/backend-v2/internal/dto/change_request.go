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

package dto

import "github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"

// ChangeRequestCreateRequest is the portal's request shape for
// POST /change-requests — a deliberately restricted subset of
// entity-service's CreateChangeRequestRequest. Excluded fields are internal
// WSO2 support operations, not customer self-service actions: GroupID/
// AssignedEngineerID (support team/engineer assignment), State (entity-service
// defaults this; a customer shouldn't set the initial workflow state
// directly), RequestedByID (an arbitrary "on behalf of" WSO2/ServiceNow user
// id — not something an external customer has or should supply), and
// WorkNote (an internal support annotation, same rationale as excluding
// entity-service's "work_note" comment type from case comments).
type ChangeRequestCreateRequest struct {
	Subject             string  `json:"subject"`
	Category            *string `json:"category,omitempty"`
	ServiceID           *string `json:"serviceId,omitempty"`
	ServiceOfferingID   *string `json:"serviceOfferingId,omitempty"`
	ConfigurationItemID *string `json:"configurationItemId,omitempty"`
	Priority            *string `json:"priority,omitempty"`
	Impact              *string `json:"impact,omitempty"`
	Type                *string `json:"type,omitempty"`
	Risk                *string `json:"risk,omitempty"`
	Description         *string `json:"description,omitempty"`
	Justification       *string `json:"justification,omitempty"`
	ImplementationPlan  *string `json:"implementationPlan,omitempty"`
	RiskImpactAnalysis  *string `json:"riskImpactAnalysis,omitempty"`
	BackoutPlan         *string `json:"backoutPlan,omitempty"`
	TestPlan            *string `json:"testPlan,omitempty"`
	PlannedStartDate    *string `json:"plannedStartDate,omitempty"`
	PlannedEndDate      *string `json:"plannedEndDate,omitempty"`
	Comment             *string `json:"comment,omitempty"`
}

// BuildEntityCreateChangeRequestRequest converts the portal's restricted
// create request into entity-service's full request shape, leaving every
// excluded field nil.
func BuildEntityCreateChangeRequestRequest(req ChangeRequestCreateRequest) entity.CreateChangeRequestRequest {
	return entity.CreateChangeRequestRequest{
		Subject:             req.Subject,
		Category:            req.Category,
		ServiceID:           req.ServiceID,
		ServiceOfferingID:   req.ServiceOfferingID,
		ConfigurationItemID: req.ConfigurationItemID,
		Priority:            req.Priority,
		Impact:              req.Impact,
		Type:                req.Type,
		Risk:                req.Risk,
		Description:         req.Description,
		Justification:       req.Justification,
		ImplementationPlan:  req.ImplementationPlan,
		RiskImpactAnalysis:  req.RiskImpactAnalysis,
		BackoutPlan:         req.BackoutPlan,
		TestPlan:            req.TestPlan,
		PlannedStartDate:    req.PlannedStartDate,
		PlannedEndDate:      req.PlannedEndDate,
		Comment:             req.Comment,
	}
}

// ChangeRequestCreateResponse is the portal's response for POST /change-requests.
type ChangeRequestCreateResponse struct {
	ID        string `json:"id"`
	Number    string `json:"number"`
	CreatedOn string `json:"createdOn"`
}

// MapChangeRequestCreate builds the portal response from entity-service's CreateChangeRequestResponse.
func MapChangeRequestCreate(r entity.CreateChangeRequestResponse) ChangeRequestCreateResponse {
	return ChangeRequestCreateResponse{
		ID:        r.ChangeRequest.ID,
		Number:    r.ChangeRequest.Number,
		CreatedOn: r.ChangeRequest.CreatedOn,
	}
}

// ChangeRequestSummary is one item of the portal's response for
// POST /projects/{id}/change-requests/search — shaped to match the
// frontend's own ChangeRequestItem type
// (apps/customer-portal/webapp/src/features/operations/types/
// changeRequests.ts) field-for-field: Title (not Subject), StartDate/
// EndDate (not PlannedStartOn/PlannedEndOn), every reference field as
// IDLabelRef (not Ref), and Impact/State/Type as IDLabelRef too — see
// change_request_enum_mapping.go for the translation. InternalID and
// HasServiceOutage are in the frontend's type but have no entity-service
// equivalent at all (entity.SearchChangeRequestView carries neither) — not
// fixable in this DTO layer alone.
type ChangeRequestSummary struct {
	ID               string      `json:"id"`
	Number           string      `json:"number"`
	Title            string      `json:"title"`
	Description      *string     `json:"description,omitempty"`
	Project          IDLabelRef  `json:"project"`
	Case             *IDLabelRef `json:"case,omitempty"`
	Deployment       *IDLabelRef `json:"deployment,omitempty"`
	DeployedProduct  *IDLabelRef `json:"deployedProduct,omitempty"`
	Product          *IDLabelRef `json:"product,omitempty"`
	AssignedEngineer *IDLabelRef `json:"assignedEngineer,omitempty"`
	AssignedTeam     *IDLabelRef `json:"assignedTeam,omitempty"`
	StartDate        *string     `json:"startDate,omitempty"`
	EndDate          *string     `json:"endDate,omitempty"`
	Duration         *string     `json:"duration,omitempty"`
	Impact           *IDLabelRef `json:"impact,omitempty"`
	State            *IDLabelRef `json:"state,omitempty"`
	Type             *IDLabelRef `json:"type,omitempty"`
	CreatedOn        string      `json:"createdOn"`
	UpdatedOn        string      `json:"updatedOn"`
}

// SearchChangeRequestsResponse is the portal's response for
// POST /projects/{id}/change-requests/search. TotalRecords (not Total) to
// match the frontend's shared pagination envelope.
type SearchChangeRequestsResponse struct {
	ChangeRequests []ChangeRequestSummary `json:"changeRequests"`
	TotalRecords   int                    `json:"totalRecords"`
	Offset         int                    `json:"offset"`
	Limit          int                    `json:"limit"`
}

func mapChangeRequestSummary(v entity.SearchChangeRequestView) ChangeRequestSummary {
	var title string
	if v.Subject != nil {
		title = *v.Subject
	}
	return ChangeRequestSummary{
		ID:               v.ID,
		Number:           v.Number,
		Title:            title,
		Description:      v.Description,
		Project:          IDLabelRef{ID: v.Project.ID, Label: v.Project.Name},
		Case:             entityRefToIDLabel(v.Case),
		Deployment:       entityRefToIDLabel(v.Deployment),
		DeployedProduct:  entityRefToIDLabel(v.DeployedProduct),
		Product:          entityRefToIDLabel(v.Product),
		AssignedEngineer: entityRefToIDLabel(v.AssignedEngineer),
		AssignedTeam:     entityRefToIDLabel(v.AssignedTeam),
		StartDate:        v.PlannedStartOn,
		EndDate:          v.PlannedEndOn,
		Duration:         v.Duration,
		Impact:           crImpactRef(v.Impact),
		State:            crStateRef(v.State),
		Type:             crTypeRef(v.Type),
		CreatedOn:        v.CreatedOn,
		UpdatedOn:        v.UpdatedOn,
	}
}

// MapSearchChangeRequests builds the portal response from entity-service's SearchChangeRequestsResponse.
func MapSearchChangeRequests(r entity.SearchChangeRequestsResponse) SearchChangeRequestsResponse {
	items := make([]ChangeRequestSummary, 0, len(r.ChangeRequests))
	for _, v := range r.ChangeRequests {
		items = append(items, mapChangeRequestSummary(v))
	}
	return SearchChangeRequestsResponse{
		ChangeRequests: items,
		TotalRecords:   r.Total,
		Offset:         r.Offset,
		Limit:          r.Limit,
	}
}

// ChangeRequestSearchFilters holds the optional filter criteria for
// POST /projects/{id}/change-requests/search — shaped to match the
// frontend's own ChangeRequestSearchFilters type. StateKeys/ImpactKeys
// carry ServiceNow's numeric choice-list ids (the frontend was built
// against the old Ballerina backend and still sends these, not
// entity-service's own string enum) — see change_request_enum_mapping.go
// for the translation. No ProjectIDs field: project scoping comes
// exclusively from the {id} path parameter, same reasoning as
// dto.CaseSearchFilters.
type ChangeRequestSearchFilters struct {
	SearchQuery     string  `json:"searchQuery,omitempty"`
	StateKeys       []int   `json:"stateKeys,omitempty"`
	ImpactKeys      []int   `json:"impactKeys,omitempty"`
	ClosedStartDate *string `json:"closedStartDate,omitempty"`
	ClosedEndDate   *string `json:"closedEndDate,omitempty"`
}

// ChangeRequestSearchRequest is the portal's request body for
// POST /projects/{id}/change-requests/search.
type ChangeRequestSearchRequest struct {
	Filters    ChangeRequestSearchFilters `json:"filters"`
	SortBy     entity.ChangeRequestSort   `json:"sortBy"`
	Pagination entity.Pagination          `json:"pagination"`
}

// BuildEntitySearchChangeRequestsRequest translates the portal's request
// into entity-service's SearchChangeRequestsRequest. projectID (the {id}
// path parameter) always populates Filters.ProjectIDs — never the request
// body, which the frontend never sends one in.
//
// Which change requests a caller may see is entity-service's decision, made for
// the signed-in contact on every read (a change request is visible to the
// customer it was designated to, in whatever state it is in now), not this
// translator's: StateKeys only NARROWS what that returns. No state list is
// invented here, so a search that names none gets every change request the
// customer may see, in every state, and one that names a state nobody can see
// (New, Assess) gets none.
func BuildEntitySearchChangeRequestsRequest(projectID string, req ChangeRequestSearchRequest) entity.SearchChangeRequestsRequest {
	return entity.SearchChangeRequestsRequest{
		Filters: entity.SearchChangeRequestsFilters{
			ProjectIDs:      []string{projectID},
			SearchQuery:     req.Filters.SearchQuery,
			States:          crIDsToEnums(req.Filters.StateKeys, crStateIDToEnum),
			Impacts:         crIDsToEnums(req.Filters.ImpactKeys, crImpactIDToEnum),
			ClosedStartDate: req.Filters.ClosedStartDate,
			ClosedEndDate:   req.Filters.ClosedEndDate,
		},
		SortBy:     req.SortBy,
		Pagination: req.Pagination,
	}
}

// ChangeRequestDetails is the portal's response for GET /change-requests/{id}.
type ChangeRequestDetails struct {
	ChangeRequestSummary
	CreatedBy           string      `json:"createdBy"`
	Justification       *string     `json:"justification,omitempty"`
	ImpactDescription   *string     `json:"impactDescription,omitempty"`
	ServiceOutage       *string     `json:"serviceOutage,omitempty"`
	CommunicationPlan   *string     `json:"communicationPlan,omitempty"`
	RollbackPlan        *string     `json:"rollbackPlan,omitempty"`
	TestPlan            *string     `json:"testPlan,omitempty"`
	HasCustomerApproved bool        `json:"hasCustomerApproved"`
	HasCustomerReviewed bool        `json:"hasCustomerReviewed"`
	ApprovedBy          *IDLabelRef `json:"approvedBy,omitempty"`
	ApprovedOn          *string     `json:"approvedOn,omitempty"`

	// CustomerCanAnswer is whether the signed-in customer may answer this change
	// request RIGHT NOW: approve or reject it in Customer Approval, confirm or
	// fail it in Customer Review (and, in Customer Approval, propose a new
	// implementation time unless the change is on hold). entity-service computes
	// it for the caller from the approval it asked of them, so it is exact where
	// hasCustomerApproved / hasCustomerReviewed (the recorded OUTCOME, not
	// "is it waiting for me") are not; the portal shows the buttons from it.
	// Passed through untouched, and omitted when entity-service did not compute
	// it (nil): an absent value means "unknown", which is not the same as false.
	CustomerCanAnswer *bool `json:"customerCanAnswer,omitempty"`

	// IsOnHold is whether WSO2 has this change request on hold (the reason is
	// WSO2's own note and is not passed on). A held change refuses a proposed
	// implementation time (but not an answer), so the portal turns Propose New
	// Time off, and says why, instead of letting a customer type a window only to
	// be refused. Omitted when entity-service did not say, which is not "not held".
	IsOnHold *bool `json:"isOnHold,omitempty"`

	// CustomerProposal is the conversation about a time a customer proposed: present when one was
	// (entity-service derives it from the proposed start and its confirmation, customer_updated_on /
	// customer_updated_date_confirmation). While it is pending the planned window (startDate /
	// endDate) is still the one WSO2 planned and a customer who approves approves THAT; once WSO2
	// answers, the window is either the proposal
	// (agreed: the change is scheduled for it) or WSO2's different time (disagreed). No names or
	// emails are passed on, only whether the proposal is the signed-in customer's own.
	CustomerProposal *ChangeRequestCustomerProposal `json:"customerProposal,omitempty"`
}

// ChangeRequestCustomerProposal is the customer's view of a proposed time.
type ChangeRequestCustomerProposal struct {
	StartDate string  `json:"startDate"`
	EndDate   *string `json:"endDate,omitempty"`
	// Answer is "pending", "agreed", "disagreed" or "unanswered".
	Answer string `json:"answer"`
	// ProposerRecorded is true when the proposer can be named at all while pending (a registered
	// contact wrote it); false for a date a WSO2 user wrote or one left over from an older cycle.
	ProposerRecorded *bool `json:"proposerRecorded,omitempty"`
	// ProposedByViewer is true when the pending proposal is the signed-in customer's own, false for
	// a colleague's (or an unknown proposer's).
	ProposedByViewer *bool `json:"proposedByViewer,omitempty"`
}

func mapChangeRequestCustomerProposal(p *entity.ChangeRequestCustomerProposal) *ChangeRequestCustomerProposal {
	if p == nil {
		return nil
	}
	return &ChangeRequestCustomerProposal{
		StartDate:        p.StartOn,
		EndDate:          p.EndOn,
		Answer:           p.Answer,
		ProposerRecorded: p.ProposerRecorded,
		ProposedByViewer: p.ProposedByViewer,
	}
}

// MapChangeRequestDetails builds the portal response from entity-service's ChangeRequest.
func MapChangeRequestDetails(r entity.ChangeRequest) ChangeRequestDetails {
	return ChangeRequestDetails{
		ChangeRequestSummary: mapChangeRequestSummary(r.SearchChangeRequestView),
		CreatedBy:            r.CreatedBy,
		Justification:        r.Justification,
		ImpactDescription:    r.ImpactDescription,
		ServiceOutage:        r.ServiceOutage,
		CommunicationPlan:    r.CommunicationPlan,
		RollbackPlan:         r.RollbackPlan,
		TestPlan:             r.TestPlan,
		HasCustomerApproved:  r.HasCustomerApproved,
		HasCustomerReviewed:  r.HasCustomerReviewed,
		ApprovedBy:           entityRefToIDLabel(r.ApprovedBy),
		ApprovedOn:           r.ApprovedOn,
		CustomerCanAnswer:    r.CustomerCanAnswer,
		IsOnHold:             r.OnHold,
		CustomerProposal:     mapChangeRequestCustomerProposal(r.CustomerProposal),
	}
}

// ChangeRequestUpdateResponse is the portal's response for PATCH /change-requests/{id}.
// Matches Ballerina v1 UpdatedChangeRequest and the webapp's PatchChangeRequestResponse contract.
type ChangeRequestUpdateResponse struct {
	ID        string `json:"id"`
	UpdatedOn string `json:"updatedOn"`
	UpdatedBy string `json:"updatedBy,omitempty"`
}

// MapChangeRequestUpdate builds the portal response from entity-service's PatchChangeRequestResponse.
func MapChangeRequestUpdate(r entity.PatchChangeRequestResponse) ChangeRequestUpdateResponse {
	return ChangeRequestUpdateResponse{
		ID:        r.ChangeRequest.ID,
		UpdatedOn: r.ChangeRequest.UpdatedOn,
		UpdatedBy: r.ChangeRequest.UpdatedBy,
	}
}

// ChangeRequestUpdateRequest is the portal's request shape for
// PATCH /change-requests/{id} at the staff level (ActionUpdate) — a deliberately
// restricted subset of entity-service's PatchChangeRequestRequest. Excluded
// fields are internal WSO2 support operations: ProjectID/CaseID/DeploymentID/
// DeployedProductID (change-request relinking), AssignedEngineerID/
// AssignedTeamID (support assignment), and State (a state key is dropped by the
// decode; RequestApproval below records that approval was requested, on
// entity-service's PostgreSQL data source with no state change).
//
// IsCustomerApproved / IsCustomerReviewed are on this shape only because the
// body is decoded into it: they are the CUSTOMER's own answer, which no staff
// action records on a customer's behalf. They are forwarded unchanged and
// entity-service refuses them from a staff caller (a 400 that writes nothing) on
// its PostgreSQL data source; a customer gives them at the other level
// (ChangeRequestCustomerUpdateRequest).
//
// ExpectedPlannedStartOn / ExpectedPlannedEndOn go with a customer's answer and
// nothing else, so they are on this shape only to be REFUSED: a staff body that
// carries either is a 400 (the handler), never silently dropped by the decode.
type ChangeRequestUpdateRequest struct {
	Title              *string `json:"title,omitempty"`
	Description        *string `json:"description,omitempty"`
	PlannedStartOn     *string `json:"plannedStartOn,omitempty"`
	PlannedEndOn       *string `json:"plannedEndOn,omitempty"`
	Impact             *string `json:"impact,omitempty"`
	Type               *string `json:"type,omitempty"`
	Justification      *string `json:"justification,omitempty"`
	ImpactDescription  *string `json:"impactDescription,omitempty"`
	ServiceOutage      *string `json:"serviceOutage,omitempty"`
	CommunicationPlan  *string `json:"communicationPlan,omitempty"`
	RollbackPlan       *string `json:"rollbackPlan,omitempty"`
	TestPlan           *string `json:"testPlan,omitempty"`
	IsCustomerApproved *bool   `json:"isCustomerApproved,omitempty"`
	IsCustomerReviewed *bool   `json:"isCustomerReviewed,omitempty"`
	RequestApproval    *bool   `json:"requestApproval,omitempty"`

	// Refused, never forwarded: see the type's doc comment.
	ExpectedPlannedStartOn *string `json:"expectedPlannedStartOn,omitempty"`
	ExpectedPlannedEndOn   *string `json:"expectedPlannedEndOn,omitempty"`
}

// HasExpectedWindow reports whether the staff body names the planned window a
// customer's answer was given for, which a staff body can never carry.
func (r ChangeRequestUpdateRequest) HasExpectedWindow() bool {
	return r.ExpectedPlannedStartOn != nil || r.ExpectedPlannedEndOn != nil
}

// BuildEntityPatchChangeRequestRequest converts the portal's restricted
// update request into entity-service's full request shape, leaving every
// excluded field nil (the expected window included: the handler has refused a
// body that carries it before this runs).
func BuildEntityPatchChangeRequestRequest(req ChangeRequestUpdateRequest) entity.PatchChangeRequestRequest {
	return entity.PatchChangeRequestRequest{
		Title:              req.Title,
		Description:        req.Description,
		PlannedStartOn:     req.PlannedStartOn,
		PlannedEndOn:       req.PlannedEndOn,
		Impact:             req.Impact,
		Type:               req.Type,
		Justification:      req.Justification,
		ImpactDescription:  req.ImpactDescription,
		ServiceOutage:      req.ServiceOutage,
		CommunicationPlan:  req.CommunicationPlan,
		RollbackPlan:       req.RollbackPlan,
		TestPlan:           req.TestPlan,
		IsCustomerApproved: req.IsCustomerApproved,
		IsCustomerReviewed: req.IsCustomerReviewed,
		RequestApproval:    req.RequestApproval,
	}
}

// ChangeRequestCustomerUpdateRequest is the ONLY request shape a caller who
// holds the customer-decision grant (middleware.ActionDecide) but not
// ActionUpdate may send to PATCH /change-requests/{id}: the customer's own
// answer on a change request that is waiting on them, or a proposal for a
// different implementation window.
//
//   - IsCustomerApproved: approve (true) or reject (false) a change request in
//     Customer Approval.
//   - IsCustomerReviewed: confirm the implementation succeeded (true) or failed
//     (false) on a change request in Customer Review.
//   - ExpectedPlannedStartOn / ExpectedPlannedEndOn: with an answer, the planned
//     window the customer was shown (the detail's startDate / endDate). The answer
//     is recorded only while that is still the change's window, so a page opened
//     before the change was re-scheduled cannot approve a time its reader never
//     saw. Optional; the webapp sends them.
//   - PlannedStartOn / PlannedEndOn: "propose new implementation time". A proposal
//     is a START: the change request keeps its planned length, so the webapp sends the
//     start plus the end that keeps it (the derived end, start + planned length) and
//     entity-service refuses any other end; a start alone is accepted as well, an end
//     alone is not. The proposal waits for WSO2's answer (it moves nothing by itself);
//     see ChangeRequestDetails.CustomerProposal.
//
// It is a struct of exactly these six fields on purpose: the handler decodes the
// body into it with unknown fields refused, so a field that is not here cannot
// reach entity-service however the body is spelled, and the entity-service
// request is built from it field by field (BuildEntityCustomerPatchChangeRequestRequest)
// rather than by copying a wider shape. Add a field here only after deciding that
// a customer may set it.
type ChangeRequestCustomerUpdateRequest struct {
	IsCustomerApproved     *bool   `json:"isCustomerApproved,omitempty"`
	IsCustomerReviewed     *bool   `json:"isCustomerReviewed,omitempty"`
	PlannedStartOn         *string `json:"plannedStartOn,omitempty"`
	PlannedEndOn           *string `json:"plannedEndOn,omitempty"`
	ExpectedPlannedStartOn *string `json:"expectedPlannedStartOn,omitempty"`
	ExpectedPlannedEndOn   *string `json:"expectedPlannedEndOn,omitempty"`
}

// HasExpectedWindow reports whether the request names the planned window the
// customer's answer was given for.
func (r ChangeRequestCustomerUpdateRequest) HasExpectedWindow() bool {
	return r.ExpectedPlannedStartOn != nil || r.ExpectedPlannedEndOn != nil
}

// HasDecision reports whether the request carries the customer's answer.
func (r ChangeRequestCustomerUpdateRequest) HasDecision() bool {
	return r.IsCustomerApproved != nil || r.IsCustomerReviewed != nil
}

// HasWindow reports whether the request proposes a different planned window.
func (r ChangeRequestCustomerUpdateRequest) HasWindow() bool {
	return r.PlannedStartOn != nil || r.PlannedEndOn != nil
}

// BuildEntityCustomerPatchChangeRequestRequest builds entity-service's PATCH
// request from a customer's restricted one, setting nothing but the fields it
// carries.
func BuildEntityCustomerPatchChangeRequestRequest(req ChangeRequestCustomerUpdateRequest) entity.PatchChangeRequestRequest {
	return entity.PatchChangeRequestRequest{
		IsCustomerApproved:     req.IsCustomerApproved,
		IsCustomerReviewed:     req.IsCustomerReviewed,
		PlannedStartOn:         req.PlannedStartOn,
		PlannedEndOn:           req.PlannedEndOn,
		ExpectedPlannedStartOn: req.ExpectedPlannedStartOn,
		ExpectedPlannedEndOn:   req.ExpectedPlannedEndOn,
	}
}

// ChangeRequestApprover is a single approver's response within an approval stage.
type ChangeRequestApprover struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Status      string  `json:"status"`
	RespondedOn *string `json:"respondedOn,omitempty"`
}

// ChangeRequestApproval represents a single approval stage on a change request.
type ChangeRequestApproval struct {
	Stage        string                  `json:"stage"`
	ApproverType string                  `json:"approverType"`
	ApproverName string                  `json:"approverName"`
	Status       string                  `json:"status"`
	Approvers    []ChangeRequestApprover `json:"approvers"`
}

// ChangeRequestApprovals is the portal's response for GET /change-requests/{id}/approvals.
type ChangeRequestApprovals struct {
	Approvals []ChangeRequestApproval `json:"approvals"`
}

// MapChangeRequestApprovals builds the portal response from entity-service's ChangeRequestApprovals.
func MapChangeRequestApprovals(r entity.ChangeRequestApprovals) ChangeRequestApprovals {
	approvals := make([]ChangeRequestApproval, 0, len(r.Approvals))
	for _, a := range r.Approvals {
		approvers := make([]ChangeRequestApprover, 0, len(a.Approvers))
		for _, ap := range a.Approvers {
			approvers = append(approvers, ChangeRequestApprover{
				ID:          ap.ID,
				Name:        ap.Name,
				Status:      ap.Status,
				RespondedOn: ap.RespondedOn,
			})
		}
		approvals = append(approvals, ChangeRequestApproval{
			Stage:        a.Stage,
			ApproverType: a.ApproverType,
			ApproverName: a.ApproverName,
			Status:       a.Status,
			Approvers:    approvers,
		})
	}
	return ChangeRequestApprovals{Approvals: approvals}
}

// ChangeRequestApprovalDecisionResponse is the portal's response for
// POST /change-requests/{id}/approvals/decision.
type ChangeRequestApprovalDecisionResponse struct {
	ID    string `json:"id"`
	State string `json:"state"`
}

// MapChangeRequestApprovalDecision builds the portal response from
// entity-service's ChangeRequestApprovalDecisionResponse.
func MapChangeRequestApprovalDecision(r entity.ChangeRequestApprovalDecisionResponse) ChangeRequestApprovalDecisionResponse {
	return ChangeRequestApprovalDecisionResponse{ID: r.ID, State: r.State}
}
