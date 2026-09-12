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

// snProblemsResponse mirrors the Choreo POST /problems/search response.
type snProblemsResponse struct {
	Problems     []snProblem `json:"problems"`
	TotalRecords int         `json:"totalRecords"`
	Offset       int         `json:"offset"`
	Limit        int         `json:"limit"`
}

type snProblem struct {
	ID              *string           `json:"id"`
	Number          *string           `json:"number"`
	Subject         *string           `json:"subject"`
	State           *string           `json:"state"`
	AssignmentGroup *snProblemUserRef `json:"assignmentGroup"`
	AssignedTo      *snProblemUserRef `json:"assignedTo"`
}

// snProblemSearchPayload is the Choreo POST /problems/search request body.
type snProblemSearchPayload struct {
	Filters    snProblemFilters    `json:"filters,omitempty"`
	Pagination snProjectPagination `json:"pagination"`
}

type snProblemFilters struct {
	SearchQuery string `json:"searchQuery,omitempty"`
	// Number: see domain.SearchProblemsFilters.Number doc comment. Exact
	// match against ServiceNow's `number` column -- not part of the
	// free-text SearchQuery scan.
	Number string `json:"number,omitempty"`
	// StateKeys: see domain.SearchProblemsFilters.Filters doc comment.
	StateKeys []int `json:"stateKeys,omitempty"`
	// AssignmentGroupIDs: sys_user_group sys_ids (converted from UUIDs).
	AssignmentGroupIDs []string `json:"assignmentGroupIds,omitempty"`
}

// snProblemStateKeyMap maps domain ProblemState enums to ServiceNow's raw
// problem_state numeric keys. Mirrors the SN Script Include's own
// _PROBLEM_STATE_LABELS map -- note 105 does not exist in ServiceNow's own
// numbering (a real gap, not an omission here).
var snProblemStateKeyMap = map[domain.ProblemState]int{
	domain.ProblemStateNew:               101,
	domain.ProblemStateAssess:            102,
	domain.ProblemStateRootCauseAnalysis: 103,
	domain.ProblemStateFixInProgress:     104,
	domain.ProblemStateResolved:          106,
	domain.ProblemStateClosed:            107,
}

var validProblemState = map[domain.ProblemState]bool{
	domain.ProblemStateNew:               true,
	domain.ProblemStateAssess:            true,
	domain.ProblemStateRootCauseAnalysis: true,
	domain.ProblemStateFixInProgress:     true,
	domain.ProblemStateResolved:          true,
	domain.ProblemStateClosed:            true,
}

type snProblemService struct {
	client *integrationservice.Client
}

// NewServiceNowProblemService constructs a ProblemService backed by the Choreo API.
func NewServiceNowProblemService(client *integrationservice.Client) ProblemService {
	return &snProblemService{client: client}
}

func (s *snProblemService) SearchProblems(ctx context.Context, req domain.SearchProblemsRequest) (domain.SearchProblemsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchProblemsResponse{}, err
	}
	if err := validateSearchQuery(req.Filters.SearchQuery); err != nil {
		return domain.SearchProblemsResponse{}, err
	}
	if err := validateExactNumber("number", req.Filters.Number); err != nil {
		return domain.SearchProblemsResponse{}, err
	}
	parsedFilters, err := ParseProblemFieldFilters(req.Filters.Filters)
	if err != nil {
		return domain.SearchProblemsResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)

	payload := snProblemSearchPayload{
		Filters: snProblemFilters{
			SearchQuery:        req.Filters.SearchQuery,
			Number:             stringPtrValue(req.Filters.Number),
			StateKeys:          parsedFilters.StateKeys,
			AssignmentGroupIDs: uuidsToSysids(parsedFilters.AssignmentGroupIDs),
		},
		Pagination: snProjectPagination{Limit: req.Pagination.Limit, Offset: req.Pagination.Offset},
	}

	raw, err := s.client.Post(ctx, "/problems/search", token, payload)
	if err != nil {
		return domain.SearchProblemsResponse{}, err
	}

	var snResp snProblemsResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.SearchProblemsResponse{}, fmt.Errorf("sn problems: parse response: %w", err)
	}

	views := make([]domain.SearchProblemView, 0, len(snResp.Problems))
	for _, p := range snResp.Problems {
		view := domain.SearchProblemView{
			Subject: p.Subject,
			Number:  p.Number,
			State:   p.State,
		}
		if p.ID != nil && *p.ID != "" {
			id := sysidToUUID(*p.ID)
			view.ID = &id
		}
		if p.AssignmentGroup != nil {
			view.AssignmentGroup = &domain.EntityRef{ID: sysidToUUID(p.AssignmentGroup.ID), Name: p.AssignmentGroup.Name}
		}
		if p.AssignedTo != nil {
			view.AssignedTo = &domain.EntityRef{ID: sysidToUUID(p.AssignedTo.ID), Name: p.AssignedTo.Name}
		}
		views = append(views, view)
	}

	return domain.SearchProblemsResponse{
		Problems: views,
		Total:    snResp.TotalRecords,
		Limit:    req.Pagination.Limit,
		Offset:   req.Pagination.Offset,
	}, nil
}

// snProblemAggregatePayload is the Choreo POST /problems/aggregate request body.
type snProblemAggregatePayload struct {
	Filters   snProblemFilters `json:"filters,omitempty"`
	GroupBy   string           `json:"groupBy"`
	MaxGroups int              `json:"maxGroups,omitempty"`
}

// validProblemAggregateField is the allow-list for
// AggregateProblemsRequest.GroupBy, matching openapi.yaml's
// AggregateProblemsRequest.groupBy enum exactly.
var validProblemAggregateField = map[string]bool{
	"state":           true,
	"assignmentGroup": true,
}

// AggregateProblems implements ProblemService by calling the Choreo POST
// /problems/aggregate endpoint: a single server-side aggregation over the
// requested field, capped to the top MaxGroups buckets with the remainder
// folded into AggregateResponse.OthersCount. Filter parsing and validation
// mirror SearchProblems.
func (s *snProblemService) AggregateProblems(ctx context.Context, req domain.AggregateProblemsRequest) (domain.AggregateResponse, error) {
	if req.GroupBy == "" {
		return domain.AggregateResponse{}, &apierror.ValidationError{Msg: "groupBy is required"}
	}
	if !validProblemAggregateField[req.GroupBy] {
		return domain.AggregateResponse{}, &apierror.ValidationError{Msg: "groupBy contains invalid value: " + req.GroupBy}
	}
	if err := validateSearchQuery(req.Filters.SearchQuery); err != nil {
		return domain.AggregateResponse{}, err
	}
	if err := validateExactNumber("number", req.Filters.Number); err != nil {
		return domain.AggregateResponse{}, err
	}
	parsedFilters, err := ParseProblemFieldFilters(req.Filters.Filters)
	if err != nil {
		return domain.AggregateResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)

	payload := snProblemAggregatePayload{
		Filters: snProblemFilters{
			SearchQuery:        req.Filters.SearchQuery,
			Number:             stringPtrValue(req.Filters.Number),
			StateKeys:          parsedFilters.StateKeys,
			AssignmentGroupIDs: uuidsToSysids(parsedFilters.AssignmentGroupIDs),
		},
		GroupBy:   req.GroupBy,
		MaxGroups: req.MaxGroups,
	}

	raw, err := s.client.Post(ctx, "/problems/aggregate", token, payload)
	if err != nil {
		return domain.AggregateResponse{}, err
	}

	var resp domain.AggregateResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return domain.AggregateResponse{}, fmt.Errorf("sn problems: parse aggregate response: %w", err)
	}
	// "assignmentGroup" is the only ID-valued field in
	// validProblemAggregateField; SN returns its bucket keys as raw
	// sys_ids, so convert them to this platform's UUIDs before returning.
	// "state" is a plain enum and is left as-is.
	if req.GroupBy == "assignmentGroup" {
		for i := range resp.Groups {
			resp.Groups[i].Key = sysidToUUID(resp.Groups[i].Key)
		}
	}
	return resp, nil
}

// snProblemEntityRef is a compact id+number reference used for the problem's
// origin case / primary incident / linked incidents / linked change request.
type snProblemEntityRef struct {
	ID     string `json:"id"`
	Number string `json:"number"`
}

// snProblemUserRef is a compact id+name reference used for assignedTo/resolvedBy.
type snProblemUserRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// snProblemDetailResponse mirrors the Choreo GET /problems/{id} response.
type snProblemDetailResponse struct {
	ID                  string               `json:"id"`
	Number              string               `json:"number"`
	Subject             string               `json:"subject"`
	State               *string              `json:"state"`
	Priority            *string              `json:"priority"`
	Category            *string              `json:"category"`
	Subcategory         *string              `json:"subcategory"`
	OriginCase          *snProblemEntityRef  `json:"originCase"`
	PrimaryIncident     *snProblemEntityRef  `json:"primaryIncident"`
	LinkedIncidents     []snProblemEntityRef `json:"linkedIncidents"`
	LinkedChangeRequest *snProblemEntityRef  `json:"linkedChangeRequest"`
	AssignedTo          *snProblemUserRef    `json:"assignedTo"`
	ResolutionCode      *string              `json:"resolutionCode"`
	CauseNotes          *string              `json:"causeNotes"`
	FixNotes            *string              `json:"fixNotes"`
	Workaround          *string              `json:"workaround"`
	ResolvedOn          *string              `json:"resolvedOn"`
	ResolvedBy          *snProblemUserRef    `json:"resolvedBy"`
	OpenedOn            *string              `json:"openedOn"`
	ClosedOn            *string              `json:"closedOn"`
}

// GetProblem implements ProblemService for the ServiceNow data source.
func (s *snProblemService) GetProblem(ctx context.Context, id string) (domain.ProblemDetail, error) {
	token := middleware.UserIDTokenFromContext(ctx)

	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.ProblemDetail{}, err
	}

	raw, err := s.client.Get(ctx, "/problems/"+uuidToSysid(id), token)
	if err != nil {
		return domain.ProblemDetail{}, err
	}

	var p snProblemDetailResponse
	if err := json.Unmarshal(raw, &p); err != nil {
		return domain.ProblemDetail{}, fmt.Errorf("sn get problem: parse response: %w", err)
	}

	return mapSNProblemDetailToView(p), nil
}

// snCreateProblemPayload is the Choreo POST /problems request body.
type snCreateProblemPayload struct {
	Subject           string  `json:"subject"`
	Category          *string `json:"category,omitempty"`
	Subcategory       *string `json:"subcategory,omitempty"`
	OriginCaseID      *string `json:"originCaseId,omitempty"`
	PrimaryIncidentID *string `json:"primaryIncidentId,omitempty"`
}

// CreateProblem implements ProblemService for the ServiceNow data source.
func (s *snProblemService) CreateProblem(ctx context.Context, req domain.CreateProblemRequest) (domain.ProblemDetail, error) {
	token := middleware.UserIDTokenFromContext(ctx)

	if strings.TrimSpace(req.Subject) == "" {
		return domain.ProblemDetail{}, &apierror.ValidationError{Msg: "subject cannot be empty"}
	}

	uuidFields := map[string]string{}
	if req.OriginCaseID != nil {
		uuidFields["originCaseId"] = *req.OriginCaseID
	}
	if req.PrimaryIncidentID != nil {
		uuidFields["primaryIncidentId"] = *req.PrimaryIncidentID
	}
	for field, val := range uuidFields {
		if err := validateUUIDs(field, []string{val}); err != nil {
			return domain.ProblemDetail{}, err
		}
	}

	payload := snCreateProblemPayload{
		Subject:     req.Subject,
		Category:    req.Category,
		Subcategory: req.Subcategory,
	}
	if req.OriginCaseID != nil {
		payload.OriginCaseID = strPtr(uuidToSysid(*req.OriginCaseID))
	}
	if req.PrimaryIncidentID != nil {
		payload.PrimaryIncidentID = strPtr(uuidToSysid(*req.PrimaryIncidentID))
	}

	raw, err := s.client.Post(ctx, "/problems", token, payload)
	if err != nil {
		return domain.ProblemDetail{}, err
	}

	var resp snCreateProblemResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return domain.ProblemDetail{}, fmt.Errorf("sn create problem: parse response: %w", err)
	}

	return mapSNProblemDetailToView(resp.Problem), nil
}

// snCreateProblemResponse mirrors the Choreo POST /problems response, which wraps
// the created problem's detail payload in a message envelope.
type snCreateProblemResponse struct {
	Message string                  `json:"message"`
	Problem snProblemDetailResponse `json:"problem"`
}

// mapSNProblemDetailToView maps a Choreo problem detail payload to the domain view,
// shared by GetProblem and CreateProblem.
func mapSNProblemDetailToView(p snProblemDetailResponse) domain.ProblemDetail {
	problemID := sysidToUUID(p.ID)
	number := p.Number
	subject := p.Subject

	view := domain.ProblemDetail{
		ID:             &problemID,
		Number:         &number,
		Subject:        &subject,
		State:          p.State,
		Priority:       p.Priority,
		Category:       p.Category,
		Subcategory:    p.Subcategory,
		ResolutionCode: p.ResolutionCode,
		CauseNotes:     p.CauseNotes,
		FixNotes:       p.FixNotes,
		Workaround:     p.Workaround,
		ResolvedOn:     p.ResolvedOn,
		OpenedOn:       p.OpenedOn,
		ClosedOn:       p.ClosedOn,
	}
	if p.OriginCase != nil {
		view.OriginCase = &domain.CaseNumberRef{ID: sysidToUUID(p.OriginCase.ID), Number: p.OriginCase.Number}
	}
	if p.PrimaryIncident != nil {
		view.PrimaryIncident = &domain.CaseNumberRef{ID: sysidToUUID(p.PrimaryIncident.ID), Number: p.PrimaryIncident.Number}
	}
	if len(p.LinkedIncidents) > 0 {
		linked := make([]domain.CaseNumberRef, 0, len(p.LinkedIncidents))
		for _, li := range p.LinkedIncidents {
			linked = append(linked, domain.CaseNumberRef{ID: sysidToUUID(li.ID), Number: li.Number})
		}
		view.LinkedIncidents = linked
	}
	if p.LinkedChangeRequest != nil {
		view.LinkedChangeRequest = &domain.CaseNumberRef{ID: sysidToUUID(p.LinkedChangeRequest.ID), Number: p.LinkedChangeRequest.Number}
	}
	if p.AssignedTo != nil {
		view.AssignedTo = &domain.EntityRef{ID: sysidToUUID(p.AssignedTo.ID), Name: p.AssignedTo.Name}
	}
	if p.ResolvedBy != nil {
		view.ResolvedBy = &domain.EntityRef{ID: sysidToUUID(p.ResolvedBy.ID), Name: p.ResolvedBy.Name}
	}

	return view
}

// snUpdateProblemPayload is the Choreo PATCH /problems/{id} request body.
type snUpdateProblemPayload struct {
	Transition           *string `json:"transition,omitempty"`
	AssignedToID         *string `json:"assignedToId,omitempty"`
	AssignmentGroupID    *string `json:"assignmentGroupId,omitempty"`
	CauseNotes           *string `json:"causeNotes,omitempty"`
	FixNotes             *string `json:"fixNotes,omitempty"`
	Workaround           *string `json:"workaround,omitempty"`
	TargetResolutionDate *string `json:"targetResolutionDate,omitempty"`
}

// snUpdateProblemResult mirrors the Choreo PATCH /problems/{id} response's "problem" object --
// deliberately narrower than snProblemDetailResponse, matching what that endpoint actually
// returns.
type snUpdateProblemResult struct {
	ID              string            `json:"id"`
	UpdatedOn       string            `json:"updatedOn"`
	UpdatedBy       string            `json:"updatedBy"`
	State           *string           `json:"state"`
	ResolutionCode  *string           `json:"resolutionCode"`
	AssignedTo      *snProblemUserRef `json:"assignedTo"`
	AssignmentGroup *snProblemUserRef `json:"assignmentGroup"`
}

// snUpdateProblemResponse mirrors the Choreo PATCH /problems/{id} response.
type snUpdateProblemResponse struct {
	Message string                `json:"message"`
	Problem snUpdateProblemResult `json:"problem"`
}

// UpdateProblem implements ProblemService for the ServiceNow data source. It is a thin
// passthrough: Transition is forwarded unvalidated (see domain.UpdateProblemRequest doc
// comment), and the response always reflects the data source's real post-write state.
func (s *snProblemService) UpdateProblem(ctx context.Context, req domain.UpdateProblemRequest) (domain.UpdateProblemResponse, error) {
	if err := validateUUIDs("id", []string{req.ID}); err != nil {
		return domain.UpdateProblemResponse{}, err
	}

	hasUpdate := req.Transition != nil || req.AssignedToID != nil || req.AssignmentGroupID != nil ||
		req.CauseNotes != nil || req.FixNotes != nil || req.Workaround != nil || req.TargetResolutionDate != nil
	if !hasUpdate {
		return domain.UpdateProblemResponse{}, &apierror.ValidationError{Msg: "at least one field must be provided"}
	}

	optionalUUIDs := map[string]*string{
		"assignedToId":      req.AssignedToID,
		"assignmentGroupId": req.AssignmentGroupID,
	}
	for field, val := range optionalUUIDs {
		if val != nil {
			if err := validateUUIDs(field, []string{*val}); err != nil {
				return domain.UpdateProblemResponse{}, err
			}
		}
	}

	token := middleware.UserIDTokenFromContext(ctx)

	payload := snUpdateProblemPayload{
		Transition:           req.Transition,
		CauseNotes:           req.CauseNotes,
		FixNotes:             req.FixNotes,
		Workaround:           req.Workaround,
		TargetResolutionDate: req.TargetResolutionDate,
	}
	if req.AssignedToID != nil {
		v := uuidToSysid(*req.AssignedToID)
		payload.AssignedToID = &v
	}
	if req.AssignmentGroupID != nil {
		v := uuidToSysid(*req.AssignmentGroupID)
		payload.AssignmentGroupID = &v
	}

	raw, err := s.client.Patch(ctx, "/problems/"+uuidToSysid(req.ID), token, payload)
	if err != nil {
		return domain.UpdateProblemResponse{}, err
	}

	var snResp snUpdateProblemResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.UpdateProblemResponse{}, fmt.Errorf("sn update problem: parse response: %w", err)
	}

	updatedOn := snResp.Problem.UpdatedOn
	updatedBy := snResp.Problem.UpdatedBy
	view := domain.UpdateProblemView{
		UpdatedOn:      &updatedOn,
		UpdatedBy:      &updatedBy,
		State:          snResp.Problem.State,
		ResolutionCode: snResp.Problem.ResolutionCode,
	}
	if snResp.Problem.ID != "" {
		id := sysidToUUID(snResp.Problem.ID)
		view.ID = &id
	}
	if snResp.Problem.AssignedTo != nil {
		view.AssignedTo = &domain.EntityRef{ID: sysidToUUID(snResp.Problem.AssignedTo.ID), Name: snResp.Problem.AssignedTo.Name}
	}
	if snResp.Problem.AssignmentGroup != nil {
		view.AssignmentGroup = &domain.EntityRef{ID: sysidToUUID(snResp.Problem.AssignmentGroup.ID), Name: snResp.Problem.AssignmentGroup.Name}
	}

	return domain.UpdateProblemResponse{
		Message: snResp.Message,
		Problem: view,
	}, nil
}
