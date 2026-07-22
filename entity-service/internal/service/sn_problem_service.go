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

	token := middleware.UserIDTokenFromContext(ctx)

	payload := snProblemSearchPayload{
		Filters:    snProblemFilters{SearchQuery: req.Filters.SearchQuery},
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
