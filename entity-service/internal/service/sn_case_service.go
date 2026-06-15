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
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	integrationservice "github.com/wso2-open-operations/cs-tools/entity-service/internal/servicenow-integration-service"
)

// snCasesResponse mirrors the Choreo POST /cases/search response.
type snCasesResponse struct {
	Cases        []snCase `json:"cases"`
	TotalRecords int      `json:"totalRecords"`
	Offset       int      `json:"offset"`
	Limit        int      `json:"limit"`
}

type snCase struct {
	ID              string           `json:"id"`
	InternalID      string           `json:"internalId"`
	Number          string           `json:"number"`
	Title           string           `json:"title"`
	Description     string           `json:"description"`
	CreatedOn       string           `json:"createdOn"`
	UpdatedOn       *string          `json:"updatedOn"`
	CreatedBy       string           `json:"createdBy"`
	Project         snCaseEntityRef  `json:"project"`
	Deployment      snCaseEntityRef  `json:"deployment"`
	DeployedProduct snCaseEntityRef  `json:"deployedProduct"`
	State           *snCaseState     `json:"state"`
	Severity        *snCaseLabel     `json:"severity"`
	IssueType       *snCaseIssueType `json:"issueType"`
}

type snCaseEntityRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type snCaseState struct {
	ID    int    `json:"id"`
	Label string `json:"label"`
}

type snCaseLabel struct {
	Label string `json:"label"`
}

type snCaseIssueType struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// snCaseSearchPayload is the Choreo POST /cases/search request body.
type snCaseSearchPayload struct {
	Filters    snCaseFilters       `json:"filters,omitempty"`
	Pagination snProjectPagination `json:"pagination"`
}

type snCaseFilters struct {
	CaseTypes          []string `json:"caseTypes"`
	SearchQuery        string   `json:"searchQuery,omitempty"`
	ProjectIDs         []string `json:"projectIds,omitempty"`
	DeploymentIDs      []string `json:"deploymentIds,omitempty"`
	DeployedProductIDs []string `json:"deployedProductIds,omitempty"`
}

type snCaseService struct {
	client     *integrationservice.Client
	pgFallback CaseService
}

// NewSNCaseService constructs a CaseService that delegates SearchCases to the
// Choreo API and all write/read-by-id operations to pgFallback.
func NewServiceNowCaseService(client *integrationservice.Client, pgFallback CaseService) CaseService {
	return &snCaseService{client: client, pgFallback: pgFallback}
}

func (s *snCaseService) CreateCase(ctx context.Context, req domain.CreateCaseRequest) (domain.Case, error) {
	return s.pgFallback.CreateCase(ctx, req)
}

func (s *snCaseService) GetCaseByID(ctx context.Context, id string) (domain.CaseView, error) {
	return s.pgFallback.GetCaseByID(ctx, id)
}

func (s *snCaseService) CreateCaseComment(ctx context.Context, req domain.CreateCaseCommentRequest) (domain.CaseComment, error) {
	return s.pgFallback.CreateCaseComment(ctx, req)
}

func (s *snCaseService) SearchCaseComments(ctx context.Context, req domain.SearchCaseCommentsRequest) (domain.SearchCaseCommentsResponse, error) {
	return s.pgFallback.SearchCaseComments(ctx, req)
}

func (s *snCaseService) UpdateCase(ctx context.Context, req domain.UpdateCaseRequest) (domain.Case, error) {
	return s.pgFallback.UpdateCase(ctx, req)
}

// SearchCases implements CaseService by calling the Choreo POST /cases/search endpoint.
func (s *snCaseService) SearchCases(ctx context.Context, req domain.SearchCasesRequest) (domain.SearchCasesResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchCasesResponse{}, err
	}
	if err := validateSearchQuery(req.SearchQuery); err != nil {
		return domain.SearchCasesResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return domain.SearchCasesResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}

	payload := snCaseSearchPayload{
		Filters: snCaseFilters{
			CaseTypes:          []string{"default_case"},
			SearchQuery:        req.SearchQuery,
			ProjectIDs:         req.ProjectIDs,
			DeploymentIDs:      req.DeploymentIDs,
			DeployedProductIDs: req.DeployedProductIDs,
		},
		Pagination: snProjectPagination{Limit: req.Pagination.Limit, Offset: req.Pagination.Offset},
	}

	raw, err := s.client.Post(ctx, "/cases/search", token, payload)
	if err != nil {
		return domain.SearchCasesResponse{}, err
	}

	var snResp snCasesResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.SearchCasesResponse{}, fmt.Errorf("sn cases: parse response: %w", err)
	}

	views := make([]domain.SearchCaseView, 0, len(snResp.Cases))
	for _, c := range snResp.Cases {
		createdOn, err := time.Parse(snCreatedOnLayout, c.CreatedOn)
		if err != nil {
			return domain.SearchCasesResponse{}, fmt.Errorf("sn cases: parse createdOn %q: %w", c.CreatedOn, err)
		}

		updatedOn := createdOn
		if c.UpdatedOn != nil && *c.UpdatedOn != "" {
			updatedOn, err = time.Parse(snCreatedOnLayout, *c.UpdatedOn)
			if err != nil {
				return domain.SearchCasesResponse{}, fmt.Errorf("sn cases: parse updatedOn %q: %w", *c.UpdatedOn, err)
			}
		}

		state, err := snCaseStateLabelToEnum(c.State)
		if err != nil {
			return domain.SearchCasesResponse{}, fmt.Errorf("sn cases: case %q: %w", c.ID, err)
		}

		views = append(views, domain.SearchCaseView{
			ID:                     c.ID,
			Number:                 c.Number,
			InternalID:             c.InternalID,
			Subject:                c.Title,
			Description:            c.Description,
			Priority:               snSeverityToPriority(c.Severity),
			IssueType:              snIssueTypeToEnum(c.IssueType),
			State:                  state,
			CreatedOn:              createdOn,
			UpdatedOn:              updatedOn,
			CreatedBy:              domain.UserIDEmailRef{Email: c.CreatedBy},
			ProjectDetails:         domain.EntityRef{ID: c.Project.ID, Name: c.Project.Name},
			DeploymentDetails:      domain.EntityRef{ID: c.Deployment.ID, Name: c.Deployment.Name},
			DeployedProductDetails: domain.DeployedProductRef{ID: c.DeployedProduct.ID, DisplayName: c.DeployedProduct.Name},
		})
	}

	total := snResp.TotalRecords
	return domain.SearchCasesResponse{
		Cases:   views,
		Total:   total,
		Limit:   req.Pagination.Limit,
		Offset:  req.Pagination.Offset,
		HasMore: req.Pagination.Offset+len(views) < total,
	}, nil
}

// snCaseStateMap maps SN state labels (lowercased) to domain CaseState enums.
var snCaseStateMap = map[string]domain.CaseState{
	"open":              domain.CaseStateOpen,
	"work in progress":  domain.CaseStateWorkInProgress,
	"waiting on wso2":   domain.CaseStateWaitingOnWSO2,
	"awaiting info":     domain.CaseStateAwaitingInfo,
	"reopened":          domain.CaseStateReopened,
	"solution proposed": domain.CaseStateSolutionProposed,
	"closed":            domain.CaseStateClosed,
}

func snCaseStateLabelToEnum(state *snCaseState) (domain.CaseState, error) {
	if state == nil {
		return domain.CaseStateOpen, nil
	}
	if v, ok := snCaseStateMap[strings.ToLower(state.Label)]; ok {
		return v, nil
	}
	return "", fmt.Errorf("unknown case state %q from ServiceNow", state.Label)
}

func snSeverityToPriority(severity *snCaseLabel) domain.CasePriority {
	if severity == nil {
		return ""
	}
	p := domain.CasePriority(strings.ToLower(severity.Label))
	if validCasePriority[p] {
		return p
	}
	return ""
}

func snIssueTypeToEnum(issueType *snCaseIssueType) domain.CaseIssueType {
	if issueType == nil {
		return ""
	}
	it := domain.CaseIssueType(strings.ToLower(strings.ReplaceAll(issueType.ID, " ", "_")))
	if validCaseIssueType[it] {
		return it
	}
	return ""
}
