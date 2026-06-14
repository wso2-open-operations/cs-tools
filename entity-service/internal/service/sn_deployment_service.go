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
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/snclient"
)

// snDeploymentsResponse mirrors the Choreo POST /deployments/search response.
type snDeploymentsResponse struct {
	Deployments  []snDeployment `json:"deployments"`
	TotalRecords int            `json:"totalRecords"`
	Offset       int            `json:"offset"`
	Limit        int            `json:"limit"`
}

type snDeployment struct {
	ID          string          `json:"id"`
	Number      string          `json:"number"`
	Name        string          `json:"name"`
	Description *string         `json:"description"`
	CreatedOn   string          `json:"createdOn"`
	UpdatedOn   string          `json:"updatedOn"`
	Project     snDeployProject `json:"project"`
	Type        snDeployType    `json:"type"`
}

type snDeployProject struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type snDeployType struct {
	Label string `json:"label"`
}

// snDeploymentSearchPayload is the Choreo POST /deployments/search request body.
type snDeploymentSearchPayload struct {
	Filters    snDeploymentFilters    `json:"filters"`
	Pagination snProjectPagination    `json:"pagination"`
}

type snDeploymentFilters struct {
	SearchQuery string   `json:"searchQuery,omitempty"`
	ProjectIDs  []string `json:"projectIds,omitempty"`
}

type snDeploymentService struct {
	client *snclient.Client
}

// NewSNDeploymentService constructs a DeploymentService backed by the Choreo API.
func NewSNDeploymentService(client *snclient.Client) DeploymentService {
	return &snDeploymentService{client: client}
}

// SearchDeployments implements DeploymentService.
func (s *snDeploymentService) SearchDeployments(ctx context.Context, req domain.SearchDeploymentsRequest) (domain.SearchDeploymentsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchDeploymentsResponse{}, err
	}
	if err := validateSearchQuery(req.SearchQuery); err != nil {
		return domain.SearchDeploymentsResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return domain.SearchDeploymentsResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}

	payload := snDeploymentSearchPayload{
		Filters: snDeploymentFilters{
			SearchQuery: req.SearchQuery,
			ProjectIDs:  req.ProjectIDs,
		},
		Pagination: snProjectPagination{Limit: req.Pagination.Limit, Offset: req.Pagination.Offset},
	}
	raw, err := s.client.Post(ctx, "/deployments/search", token, payload)
	if err != nil {
		return domain.SearchDeploymentsResponse{}, err
	}

	var snResp snDeploymentsResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.SearchDeploymentsResponse{}, fmt.Errorf("sn deployments: parse response: %w", err)
	}

	views := make([]domain.DeploymentView, 0, len(snResp.Deployments))
	for _, d := range snResp.Deployments {
		createdOn, err := time.Parse(snCreatedOnLayout, d.CreatedOn)
		if err != nil {
			return domain.SearchDeploymentsResponse{}, fmt.Errorf("sn deployments: parse createdOn %q: %w", d.CreatedOn, err)
		}
		updatedOn, err := time.Parse(snCreatedOnLayout, d.UpdatedOn)
		if err != nil {
			return domain.SearchDeploymentsResponse{}, fmt.Errorf("sn deployments: parse updatedOn %q: %w", d.UpdatedOn, err)
		}
		deployType, err := snDeployTypeLabelToEnum(d.Type.Label)
		if err != nil {
			return domain.SearchDeploymentsResponse{}, fmt.Errorf("sn deployments: deployment %q: %w", d.ID, err)
		}
		views = append(views, domain.DeploymentView{
			ID:          d.ID,
			Number:      d.Number,
			Name:        d.Name,
			Type:        deployType,
			Description: d.Description,
			CreatedBy:   nil,
			Project:     domain.EntityRef{ID: d.Project.ID, Name: d.Project.Name},
			CreatedOn:   createdOn,
			UpdatedOn:   updatedOn,
		})
	}

	total := snResp.TotalRecords
	return domain.SearchDeploymentsResponse{
		Deployments: views,
		Total:       total,
		Limit:       req.Pagination.Limit,
		Offset:      req.Pagination.Offset,
		HasMore:     req.Pagination.Offset+len(views) < total,
	}, nil
}

// validDeploymentTypes is the set of known DeploymentType enum values.
var validDeploymentTypes = map[domain.DeploymentType]struct{}{
	domain.DeploymentTypePrimaryProduction: {},
	domain.DeploymentTypeStaging:           {},
	domain.DeploymentTypeQA:                {},
	domain.DeploymentTypeStress:            {},
	domain.DeploymentTypeUAT:               {},
	domain.DeploymentTypeDevelopment:       {},
}

// snDeployTypeLabelToEnum converts a SN deployment type label (e.g. "Primary Production")
// to the domain DeploymentType enum (e.g. "primary_production").
func snDeployTypeLabelToEnum(label string) (domain.DeploymentType, error) {
	dt := domain.DeploymentType(strings.ToLower(strings.ReplaceAll(label, " ", "_")))
	if _, ok := validDeploymentTypes[dt]; !ok {
		return "", fmt.Errorf("unknown deployment type %q from ServiceNow", label)
	}
	return dt, nil
}
