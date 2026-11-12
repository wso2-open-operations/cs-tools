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

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	integrationservice "github.com/wso2-open-operations/cs-tools/entity-service/internal/servicenow-integration-service"
)

// snTimeCardStateLabelToSN maps domain TimeCardState to the SN label string.
var snTimeCardStateLabelToSN = map[domain.TimeCardState]string{
	domain.TimeCardStatePending:   "Pending",
	domain.TimeCardStateSubmitted: "Submitted",
	domain.TimeCardStateApproved:  "Approved",
	domain.TimeCardStateRejected:  "Rejected",
	domain.TimeCardStateProcessed: "Processed",
	domain.TimeCardStateRecalled:  "Recalled",
}

// validTimeCardStates is used to validate incoming state values.
var validTimeCardStates = map[domain.TimeCardState]struct{}{
	domain.TimeCardStatePending:   {},
	domain.TimeCardStateSubmitted: {},
	domain.TimeCardStateApproved:  {},
	domain.TimeCardStateRejected:  {},
	domain.TimeCardStateProcessed: {},
	domain.TimeCardStateRecalled:  {},
}

// snTimeCardStateLabelToDomain maps SN state labels (lowercased) to domain strings.
var snTimeCardStateLabelToDomain = map[string]string{
	"pending":   string(domain.TimeCardStatePending),
	"submitted": string(domain.TimeCardStateSubmitted),
	"approved":  string(domain.TimeCardStateApproved),
	"rejected":  string(domain.TimeCardStateRejected),
	"processed": string(domain.TimeCardStateProcessed),
	"recalled":  string(domain.TimeCardStateRecalled),
}

type snTimeCardSearchPayload struct {
	Filters    *snTimeCardFilters  `json:"filters,omitempty"`
	SortBy     *snTimeCardSort     `json:"sortBy,omitempty"`
	Pagination snProjectPagination `json:"pagination"`
}

type snTimeCardSort struct {
	Field string `json:"field"`
	Order string `json:"order"`
}

// snTimeCardSortFieldMap maps domain TimeCardSortField values to SN field names.
var snTimeCardSortFieldMap = map[domain.TimeCardSortField]string{
	domain.TimeCardSortFieldUpdatedOn: "updatedOn",
	domain.TimeCardSortFieldWorkDate:  "workDate",
}

type snTimeCardFilters struct {
	ProjectIDs   []string `json:"projectIds,omitempty"`
	CaseID       string   `json:"caseId,omitempty"`
	UserID       string   `json:"userId,omitempty"`
	ApproverID   string   `json:"approverId,omitempty"`
	ApprovedByID string   `json:"approvedById,omitempty"`
	UserIDs      []string `json:"userIds,omitempty"`
	StartDate    string   `json:"startDate,omitempty"`
	EndDate      string   `json:"endDate,omitempty"`
	States       []string `json:"states,omitempty"`
}

type snTimeCardsResponse struct {
	TimeCards    []snTimeCard `json:"timeCards"`
	TotalRecords int          `json:"totalRecords"`
	Limit        int          `json:"limit"`
	Offset       int          `json:"offset"`
}

type snTimeCard struct {
	ID                       string             `json:"id"`
	TotalTime                float64            `json:"totalTime"`
	CreatedOn                string             `json:"createdOn"`
	WorkDate                 string             `json:"workDate"`
	HasBillable              bool               `json:"hasBillable"`
	TimeAnalyzing            int                `json:"timeAnalyzing"`
	TimeSettingUp            int                `json:"timeSettingUp"`
	TimeReproducingDebugging int                `json:"timeReproducingDebugging"`
	TimeProvidingSolution    int                `json:"timeProvidingSolution"`
	TimePatching             int                `json:"timePatching"`
	IssueComplexity          *string            `json:"issueComplexity"`
	WorkLogComment           *string            `json:"workLogComment"`
	RejectionReason          *string            `json:"rejectionReason"`
	State                    *snTimeCardLabel   `json:"state"`
	Approvers                []snTimeCardRef    `json:"approvers"`
	User                     *snTimeCardRef     `json:"user"`
	ApprovedBy               *snTimeCardRef     `json:"approvedBy"`
	Project                  *snTimeCardRef     `json:"project"`
	Case                     *snTimeCardCaseRef `json:"case"`
}

type snTimeCardLabel struct {
	Label string `json:"label"`
}

type snTimeCardRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type snTimeCardCaseRef struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Number string `json:"number"`
}

func snTimeCardToView(tc snTimeCard) domain.TimeCardView {
	view := domain.TimeCardView{
		ID:                       sysidToUUID(tc.ID),
		TotalTime:                tc.TotalTime,
		CreatedOn:                tc.WorkDate, // deprecated alias — always mirrors WorkDate, not SN's separate createdOn field
		WorkDate:                 tc.WorkDate,
		HasBillable:              tc.HasBillable,
		TimeAnalyzing:            tc.TimeAnalyzing,
		TimeSettingUp:            tc.TimeSettingUp,
		TimeReproducingDebugging: tc.TimeReproducingDebugging,
		TimeProvidingSolution:    tc.TimeProvidingSolution,
		TimePatching:             tc.TimePatching,
		IssueComplexity:          tc.IssueComplexity,
		WorkLogComment:           tc.WorkLogComment,
		RejectionReason:          tc.RejectionReason,
	}

	if tc.State != nil {
		label := tc.State.Label
		if mapped, ok := snTimeCardStateLabelToDomain[strings.ToLower(label)]; ok {
			view.State = &mapped
		} else {
			view.State = &label
		}
	}

	if len(tc.Approvers) > 0 {
		approvers := make([]domain.TimeCardRef, 0, len(tc.Approvers))
		for _, a := range tc.Approvers {
			approvers = append(approvers, domain.TimeCardRef{ID: sysidToUUID(a.ID), Name: a.Name})
		}
		view.Approvers = approvers
	}
	if tc.User != nil {
		view.User = &domain.TimeCardRef{ID: sysidToUUID(tc.User.ID), Name: tc.User.Name}
	}
	if tc.ApprovedBy != nil {
		view.ApprovedBy = &domain.TimeCardRef{ID: sysidToUUID(tc.ApprovedBy.ID), Name: tc.ApprovedBy.Name}
	}
	if tc.Project != nil {
		view.Project = &domain.TimeCardRef{ID: sysidToUUID(tc.Project.ID), Name: tc.Project.Name}
	}
	if tc.Case != nil {
		view.Case = &domain.TimeCardCaseRef{
			ID:     sysidToUUID(tc.Case.ID),
			Name:   tc.Case.Name,
			Number: tc.Case.Number,
		}
	}

	return view
}

type snTimeCardService struct {
	client *integrationservice.Client
}

// NewServiceNowTimeCardService constructs a TimeCardService backed by the Choreo API.
func NewServiceNowTimeCardService(client *integrationservice.Client) TimeCardService {
	return &snTimeCardService{client: client}
}

// buildTimeCardSearchPayload validates req and builds the SN request payload
// shared by both POST /time-cards/search and POST /cases/time-cards/search —
// the two endpoints take an identical request shape and differ only in
// response shape (flat time-card rows vs. rolled up by case).
func buildTimeCardSearchPayload(req domain.SearchTimeCardsRequest) (snTimeCardSearchPayload, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return snTimeCardSearchPayload{}, err
	}

	var snSortBy *snTimeCardSort
	if req.SortBy.Field != "" {
		snField, ok := snTimeCardSortFieldMap[req.SortBy.Field]
		if !ok {
			return snTimeCardSearchPayload{}, &apierror.ValidationError{Msg: "sortBy.field " + string(req.SortBy.Field) + " is not supported by ServiceNow"}
		}
		order := string(req.SortBy.Order)
		if order == "" {
			order = "desc"
		}
		snSortBy = &snTimeCardSort{Field: snField, Order: order}
	}

	payload := snTimeCardSearchPayload{
		SortBy:     snSortBy,
		Pagination: snProjectPagination{Limit: req.Pagination.Limit, Offset: req.Pagination.Offset},
	}

	if req.Filters != nil {
		for _, state := range req.Filters.States {
			if _, ok := validTimeCardStates[state]; !ok {
				return snTimeCardSearchPayload{}, &apierror.ValidationError{Msg: "states contains invalid value: " + string(state)}
			}
		}
		if err := validateUUIDs("projectIds", req.Filters.ProjectIDs); err != nil {
			return snTimeCardSearchPayload{}, err
		}
		if req.Filters.CaseID != nil {
			if err := validateUUIDs("caseId", []string{*req.Filters.CaseID}); err != nil {
				return snTimeCardSearchPayload{}, err
			}
		}
		if req.Filters.UserID != nil {
			if err := validateUUIDs("userId", []string{*req.Filters.UserID}); err != nil {
				return snTimeCardSearchPayload{}, err
			}
		}
		if req.Filters.ApproverID != nil {
			if err := validateUUIDs("approverId", []string{*req.Filters.ApproverID}); err != nil {
				return snTimeCardSearchPayload{}, err
			}
		}
		if req.Filters.ApprovedByID != nil {
			if err := validateUUIDs("approvedById", []string{*req.Filters.ApprovedByID}); err != nil {
				return snTimeCardSearchPayload{}, err
			}
		}
		if err := validateUUIDs("userIds", req.Filters.UserIDs); err != nil {
			return snTimeCardSearchPayload{}, err
		}
		// Refused rather than dropped: without it the search would match
		// every engineer's cards, not the team's.
		if len(req.Filters.TeamIDs) > 0 {
			return snTimeCardSearchPayload{}, &apierror.ValidationError{Msg: "filters.teamIds is not supported when DATA_SOURCE=servicenow"}
		}

		snStates := make([]string, 0, len(req.Filters.States))
		for _, state := range req.Filters.States {
			snStates = append(snStates, snTimeCardStateLabelToSN[state])
		}

		filters := snTimeCardFilters{
			ProjectIDs: uuidsToSysids(req.Filters.ProjectIDs),
			States:     snStates,
		}
		if req.Filters.CaseID != nil {
			filters.CaseID = uuidToSysid(*req.Filters.CaseID)
		}
		if req.Filters.UserID != nil {
			filters.UserID = uuidToSysid(*req.Filters.UserID)
		}
		if req.Filters.ApproverID != nil {
			filters.ApproverID = uuidToSysid(*req.Filters.ApproverID)
		}
		if req.Filters.ApprovedByID != nil {
			filters.ApprovedByID = uuidToSysid(*req.Filters.ApprovedByID)
		}
		if len(req.Filters.UserIDs) > 0 {
			filters.UserIDs = uuidsToSysids(req.Filters.UserIDs)
		}
		if req.Filters.StartDate != nil {
			filters.StartDate = *req.Filters.StartDate
		}
		if req.Filters.EndDate != nil {
			filters.EndDate = *req.Filters.EndDate
		}
		payload.Filters = &filters
	}

	return payload, nil
}

func (s *snTimeCardService) SearchTimeCards(ctx context.Context, req domain.SearchTimeCardsRequest) (domain.SearchTimeCardsResponse, error) {
	payload, err := buildTimeCardSearchPayload(req)
	if err != nil {
		return domain.SearchTimeCardsResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)

	raw, err := s.client.Post(ctx, "/time-cards/search", token, payload)
	if err != nil {
		return domain.SearchTimeCardsResponse{}, err
	}

	var snResp snTimeCardsResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.SearchTimeCardsResponse{}, fmt.Errorf("sn time cards: parse response: %w", err)
	}

	views := make([]domain.TimeCardView, 0, len(snResp.TimeCards))
	for _, tc := range snResp.TimeCards {
		views = append(views, snTimeCardToView(tc))
	}

	return domain.SearchTimeCardsResponse{
		TimeCards: views,
		Total:     snResp.TotalRecords,
		Limit:     snResp.Limit,
		Offset:    snResp.Offset,
	}, nil
}

// AggregateTimeCards implements TimeCardService. The ServiceNow integration
// has no time card aggregate, so this mode refuses it rather than paging
// through every matching card to count them here.
func (s *snTimeCardService) AggregateTimeCards(context.Context, domain.AggregateTimeCardsRequest) (domain.AggregateResponse, error) {
	return domain.AggregateResponse{}, &apierror.ValidationError{Msg: "the time card aggregate is not supported when DATA_SOURCE=servicenow"}
}

// snCaseTimeCardBillingInfo mirrors the Choreo CaseTimeCardBillingInfo shape.
type snCaseTimeCardBillingInfo struct {
	TotalTime float64 `json:"totalTime"`
	Count     int     `json:"count"`
}

// snCaseTimeCardCaseRef mirrors the case reference embedded in the Choreo
// CaseTimeCardSummary shape.
type snCaseTimeCardCaseRef struct {
	ID        string       `json:"id"`
	Number    string       `json:"number"`
	Name      string       `json:"name"`
	UpdatedOn string       `json:"updatedOn"`
	Project   *snEntityRef `json:"project"`
	// Choreo sends these three alongside updatedOn; leaving them undeclared
	// meant encoding/json discarded them, so the portal's time-tracking card had
	// no author to render. Pointers per this service's optional-field convention:
	// nil serialises as null rather than an empty string.
	CreatedOn *string `json:"createdOn"`
	CreatedBy *string `json:"createdBy"`
	UpdatedBy *string `json:"updatedBy"`
}

// snCaseTimeCardSummary mirrors the Choreo CaseTimeCardSummary shape.
type snCaseTimeCardSummary struct {
	Case        snCaseTimeCardCaseRef     `json:"case"`
	TotalTime   float64                   `json:"totalTime"`
	TotalCount  int                       `json:"totalCount"`
	Billable    snCaseTimeCardBillingInfo `json:"billable"`
	NonBillable snCaseTimeCardBillingInfo `json:"nonBillable"`
}

// snCaseTimeCardsSearchResponse mirrors the Choreo POST /cases/time-cards/search response.
type snCaseTimeCardsSearchResponse struct {
	Cases        []snCaseTimeCardSummary `json:"cases"`
	TotalRecords int                     `json:"totalRecords"`
	Limit        int                     `json:"limit"`
	Offset       int                     `json:"offset"`
}

func (s *snTimeCardService) SearchCaseTimeCards(ctx context.Context, req domain.SearchTimeCardsRequest) (domain.SearchCaseTimeCardsResponse, error) {
	payload, err := buildTimeCardSearchPayload(req)
	if err != nil {
		return domain.SearchCaseTimeCardsResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)

	raw, err := s.client.Post(ctx, "/cases/time-cards/search", token, payload)
	if err != nil {
		return domain.SearchCaseTimeCardsResponse{}, err
	}

	var snResp snCaseTimeCardsSearchResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.SearchCaseTimeCardsResponse{}, fmt.Errorf("sn case time cards: parse response: %w", err)
	}

	cases := make([]domain.CaseTimeCardSummary, 0, len(snResp.Cases))
	for _, c := range snResp.Cases {
		summary := domain.CaseTimeCardSummary{
			Case: domain.CaseTimeCardCaseRef{
				ID:        sysidToUUID(c.Case.ID),
				Number:    c.Case.Number,
				Name:      c.Case.Name,
				UpdatedOn: c.Case.UpdatedOn,
				CreatedOn: c.Case.CreatedOn,
				CreatedBy: c.Case.CreatedBy,
				UpdatedBy: c.Case.UpdatedBy,
			},
			TotalTime:   c.TotalTime,
			TotalCount:  c.TotalCount,
			Billable:    domain.CaseTimeCardBillingInfo{TotalTime: c.Billable.TotalTime, Count: c.Billable.Count},
			NonBillable: domain.CaseTimeCardBillingInfo{TotalTime: c.NonBillable.TotalTime, Count: c.NonBillable.Count},
		}
		if c.Case.Project != nil {
			summary.Case.Project = &domain.EntityRef{ID: sysidToUUID(c.Case.Project.ID), Name: c.Case.Project.Name}
		}
		cases = append(cases, summary)
	}

	return domain.SearchCaseTimeCardsResponse{
		Cases:  cases,
		Total:  snResp.TotalRecords,
		Limit:  snResp.Limit,
		Offset: snResp.Offset,
	}, nil
}

// --- write path (create / update / approve / reject) ---------------------------

type snTimeCardCreatePayload struct {
	CaseID                   string   `json:"caseId"`
	ProjectID                string   `json:"projectId"`
	Date                     string   `json:"date"`
	ApproverIDs              []string `json:"approverIds"`
	IsBillable               bool     `json:"isBillable"`
	IssueComplexity          string   `json:"issueComplexity,omitempty"`
	WorkLogComment           string   `json:"workLogComment,omitempty"`
	TimeAnalyzing            int      `json:"timeAnalyzing"`
	TimeSettingUp            int      `json:"timeSettingUp"`
	TimeReproducingDebugging int      `json:"timeReproducingDebugging"`
	TimeProvidingSolution    int      `json:"timeProvidingSolution"`
	TimePatching             int      `json:"timePatching"`
}

type snTimeCardUpdatePayload struct {
	State                    string   `json:"state,omitempty"`
	LeadComment              *string  `json:"leadComment,omitempty"`
	Date                     *string  `json:"date,omitempty"`
	ApproverIDs              []string `json:"approverIds,omitempty"`
	IsBillable               *bool    `json:"isBillable,omitempty"`
	IssueComplexity          *string  `json:"issueComplexity,omitempty"`
	WorkLogComment           *string  `json:"workLogComment,omitempty"`
	TimeAnalyzing            *int     `json:"timeAnalyzing,omitempty"`
	TimeSettingUp            *int     `json:"timeSettingUp,omitempty"`
	TimeReproducingDebugging *int     `json:"timeReproducingDebugging,omitempty"`
	TimeProvidingSolution    *int     `json:"timeProvidingSolution,omitempty"`
	TimePatching             *int     `json:"timePatching,omitempty"`
}

type snTimeCardMutationResponse struct {
	Message  string      `json:"message"`
	TimeCard *snTimeCard `json:"timeCard"`
}

func parseTimeCardMutation(raw []byte, op string) (domain.TimeCardMutationResponse, error) {
	var snResp snTimeCardMutationResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.TimeCardMutationResponse{}, fmt.Errorf("sn time cards: parse %s response: %w", op, err)
	}
	out := domain.TimeCardMutationResponse{Message: snResp.Message}
	if snResp.TimeCard != nil {
		v := snTimeCardToView(*snResp.TimeCard)
		out.TimeCard = &v
	}
	return out, nil
}

func nonNegativeMinutes(field string, v int) error {
	if v < 0 {
		return &apierror.ValidationError{Msg: field + " must not be negative"}
	}
	return nil
}

func (s *snTimeCardService) CreateTimeCard(ctx context.Context, req domain.CreateTimeCardRequest) (domain.TimeCardMutationResponse, error) {
	token := middleware.UserIDTokenFromContext(ctx)

	if req.CaseID == "" {
		return domain.TimeCardMutationResponse{}, &apierror.ValidationError{Msg: "caseId is required"}
	}
	if req.ProjectID == "" {
		return domain.TimeCardMutationResponse{}, &apierror.ValidationError{Msg: "projectId is required"}
	}
	if req.Date == "" {
		return domain.TimeCardMutationResponse{}, &apierror.ValidationError{Msg: "date is required"}
	}
	if len(req.ApproverIDs) == 0 {
		return domain.TimeCardMutationResponse{}, &apierror.ValidationError{Msg: "approverIds must not be empty"}
	}
	if err := validateUUIDs("caseId", []string{req.CaseID}); err != nil {
		return domain.TimeCardMutationResponse{}, err
	}
	if err := validateUUIDs("projectId", []string{req.ProjectID}); err != nil {
		return domain.TimeCardMutationResponse{}, err
	}
	if err := validateUUIDs("approverIds", req.ApproverIDs); err != nil {
		return domain.TimeCardMutationResponse{}, err
	}
	for f, v := range map[string]int{
		"timeAnalyzing": req.TimeAnalyzing, "timeSettingUp": req.TimeSettingUp,
		"timeReproducingDebugging": req.TimeReproducingDebugging,
		"timeProvidingSolution":    req.TimeProvidingSolution, "timePatching": req.TimePatching,
	} {
		if err := nonNegativeMinutes(f, v); err != nil {
			return domain.TimeCardMutationResponse{}, err
		}
	}

	payload := snTimeCardCreatePayload{
		CaseID:                   uuidToSysid(req.CaseID),
		ProjectID:                uuidToSysid(req.ProjectID),
		Date:                     req.Date,
		ApproverIDs:              uuidsToSysids(req.ApproverIDs),
		IsBillable:               req.IsBillable,
		TimeAnalyzing:            req.TimeAnalyzing,
		TimeSettingUp:            req.TimeSettingUp,
		TimeReproducingDebugging: req.TimeReproducingDebugging,
		TimeProvidingSolution:    req.TimeProvidingSolution,
		TimePatching:             req.TimePatching,
	}
	if req.IssueComplexity != nil {
		payload.IssueComplexity = *req.IssueComplexity
	}
	if req.WorkLogComment != nil {
		payload.WorkLogComment = *req.WorkLogComment
	}

	raw, err := s.client.Post(ctx, "/time-cards", token, payload)
	if err != nil {
		return domain.TimeCardMutationResponse{}, err
	}
	return parseTimeCardMutation(raw, "create")
}

func (s *snTimeCardService) UpdateTimeCard(ctx context.Context, req domain.UpdateTimeCardRequest) (domain.TimeCardMutationResponse, error) {
	token := middleware.UserIDTokenFromContext(ctx)
	if err := validateUUIDs("id", []string{req.ID}); err != nil {
		return domain.TimeCardMutationResponse{}, err
	}
	// Reject no-op and ambiguous updates: the PATCH carries either a state
	// transition OR editable fields, never neither and never both (a transition
	// ignores field edits downstream, so combining them would silently drop them).
	hasEdit := req.Date != nil || req.ApproverIDs != nil || req.IsBillable != nil ||
		req.IssueComplexity != nil || req.WorkLogComment != nil ||
		req.TimeAnalyzing != nil || req.TimeSettingUp != nil ||
		req.TimeReproducingDebugging != nil || req.TimeProvidingSolution != nil ||
		req.TimePatching != nil
	if req.State == nil && !hasEdit {
		return domain.TimeCardMutationResponse{}, &apierror.ValidationError{Msg: "no fields to update"}
	}
	if req.State != nil && hasEdit {
		return domain.TimeCardMutationResponse{}, &apierror.ValidationError{Msg: "a state transition cannot be combined with field edits"}
	}
	if req.State != nil {
		if *req.State != domain.TimeCardStateApproved && *req.State != domain.TimeCardStateRejected {
			return domain.TimeCardMutationResponse{}, &apierror.ValidationError{Msg: "state must be approved or rejected"}
		}
		if *req.State == domain.TimeCardStateRejected &&
			(req.LeadComment == nil || strings.TrimSpace(*req.LeadComment) == "") {
			return domain.TimeCardMutationResponse{}, &apierror.ValidationError{Msg: "leadComment is required when rejecting"}
		}
	}
	if req.ApproverIDs != nil {
		if len(req.ApproverIDs) == 0 {
			return domain.TimeCardMutationResponse{}, &apierror.ValidationError{Msg: "approverIds must not be empty when provided"}
		}
		if err := validateUUIDs("approverIds", req.ApproverIDs); err != nil {
			return domain.TimeCardMutationResponse{}, err
		}
	}
	for f, v := range map[string]*int{
		"timeAnalyzing": req.TimeAnalyzing, "timeSettingUp": req.TimeSettingUp,
		"timeReproducingDebugging": req.TimeReproducingDebugging,
		"timeProvidingSolution":    req.TimeProvidingSolution, "timePatching": req.TimePatching,
	} {
		if v != nil {
			if err := nonNegativeMinutes(f, *v); err != nil {
				return domain.TimeCardMutationResponse{}, err
			}
		}
	}

	payload := snTimeCardUpdatePayload{
		LeadComment:              req.LeadComment,
		Date:                     req.Date,
		IsBillable:               req.IsBillable,
		IssueComplexity:          req.IssueComplexity,
		WorkLogComment:           req.WorkLogComment,
		TimeAnalyzing:            req.TimeAnalyzing,
		TimeSettingUp:            req.TimeSettingUp,
		TimeReproducingDebugging: req.TimeReproducingDebugging,
		TimeProvidingSolution:    req.TimeProvidingSolution,
		TimePatching:             req.TimePatching,
	}
	if req.State != nil {
		payload.State = snTimeCardStateLabelToSN[*req.State]
	}
	if req.ApproverIDs != nil {
		payload.ApproverIDs = uuidsToSysids(req.ApproverIDs)
	}

	raw, err := s.client.Patch(ctx, fmt.Sprintf("/time-cards/%s", uuidToSysid(req.ID)), token, payload)
	if err != nil {
		return domain.TimeCardMutationResponse{}, err
	}
	return parseTimeCardMutation(raw, "update")
}

// DeleteTimeCard forwards a delete straight to SN with the caller's own
// token, the same trust model UpdateTimeCard uses — no fetch-then-check
// here; SN's own ACL is what actually enforces submitter-only +
// submitted-state-only, exactly as it already does for edits.
func (s *snTimeCardService) DeleteTimeCard(ctx context.Context, req domain.DeleteTimeCardRequest) (domain.DeleteTimeCardResponse, error) {
	token := middleware.UserIDTokenFromContext(ctx)
	if err := validateUUIDs("id", []string{req.ID}); err != nil {
		return domain.DeleteTimeCardResponse{}, err
	}

	raw, err := s.client.Delete(ctx, fmt.Sprintf("/time-cards/%s", uuidToSysid(req.ID)), token)
	if err != nil {
		return domain.DeleteTimeCardResponse{}, err
	}
	var snResp struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.DeleteTimeCardResponse{}, fmt.Errorf("sn time cards: parse delete response: %w", err)
	}
	return domain.DeleteTimeCardResponse{Message: snResp.Message}, nil
}
