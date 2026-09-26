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
	integrationservice "github.com/wso2-open-operations/cs-tools/entity-service/internal/servicenow-integration-service"
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
	URL         *string         `json:"url"`
	CreatedOn   string          `json:"createdOn"`
	UpdatedOn   string          `json:"updatedOn"`
	Project     snDeployProject `json:"project"`
	Type        snDeployType    `json:"type"`
	// DeployedProductCount is sent by ServiceNow on every deployment in the
	// search response. It was previously not declared here and so silently
	// discarded — see domain.DeploymentView.DeployedProductCount.
	DeployedProductCount int `json:"deployedProductCount"`
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
	Filters    snDeploymentFilters `json:"filters"`
	Pagination snProjectPagination `json:"pagination"`
}

type snDeploymentFilters struct {
	SearchQuery string   `json:"searchQuery,omitempty"`
	ProjectIDs  []string `json:"projectIds,omitempty"`
}

type snDeploymentService struct {
	client *integrationservice.Client
}

// NewSNDeploymentService constructs a DeploymentService backed by the Choreo API.
func NewServiceNowDeploymentService(client *integrationservice.Client) DeploymentService {
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

	payload := snDeploymentSearchPayload{
		Filters: snDeploymentFilters{
			SearchQuery: req.SearchQuery,
			ProjectIDs:  uuidsToSysids(req.ProjectIDs),
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
			ID:          sysidToUUID(d.ID),
			Number:      d.Number,
			Name:        d.Name,
			Type:        deployType,
			Description: d.Description,
			CreatedBy:   nil,
			Project:     domain.EntityRef{ID: sysidToUUID(d.Project.ID), Name: d.Project.Name},
			CreatedOn:   createdOn,
			UpdatedOn:   updatedOn,

			URL:                  d.URL,
			DeployedProductCount: d.DeployedProductCount,
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

// snCreateDeploymentPayload is the Choreo POST /deployments request body.
type snCreateDeploymentPayload struct {
	ProjectID   string `json:"projectId"`
	Name        string `json:"name"`
	TypeKey     int    `json:"typeKey"` // always set after nil-check in CreateDeployment
	Description string `json:"description"`
}

type snCreateDeploymentResponse struct {
	Message    string `json:"message"`
	Deployment struct {
		ID        string `json:"id"`
		Number    string `json:"number"`
		CreatedOn string `json:"createdOn"`
		CreatedBy string `json:"createdBy"`
	} `json:"deployment"`
}

// createDeploymentSN validates req and performs the actual ServiceNow
// POST /deployments call, parsing its response. Shared by CreateDeployment
// (below, the public ServiceNow-data-source path, which only exposes
// id/createdOn/createdBy on the wire — domain.CreatedDeployment has no
// Number field, matching backend-v2's own mirror struct) and
// deploymentService.createDeploymentSNFirst (dual-write's Postgres insert,
// which needs Number too: deployment.number is NOT NULL UNIQUE and Postgres
// has no generator for it, so ServiceNow's own assigned number is the only
// source, the same problem CreateCaseFromServiceNow solves for case.number).
func (s *snDeploymentService) createDeploymentSN(ctx context.Context, req domain.CreateDeploymentRequest) (snCreateDeploymentResponse, error) {
	if err := validateUUIDs("projectId", []string{req.ProjectID}); err != nil {
		return snCreateDeploymentResponse{}, err
	}
	if req.Name == "" {
		return snCreateDeploymentResponse{}, &apierror.ValidationError{Msg: "name is required"}
	}
	if req.Type == nil {
		return snCreateDeploymentResponse{}, &apierror.ValidationError{Msg: "type is required"}
	}
	if _, ok := validDeploymentTypes[*req.Type]; !ok {
		return snCreateDeploymentResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("invalid type %q", *req.Type)}
	}
	if req.Description == "" {
		return snCreateDeploymentResponse{}, &apierror.ValidationError{Msg: "description is required"}
	}

	token := middleware.UserIDTokenFromContext(ctx)

	payload := snCreateDeploymentPayload{
		ProjectID:   uuidToSysid(req.ProjectID),
		Name:        req.Name,
		TypeKey:     deploymentTypeToKey[*req.Type],
		Description: req.Description,
	}

	raw, err := s.client.Post(ctx, "/deployments", token, payload)
	if err != nil {
		return snCreateDeploymentResponse{}, err
	}

	var snResp snCreateDeploymentResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return snCreateDeploymentResponse{}, fmt.Errorf("sn create deployment: parse response: %w", err)
	}
	return snResp, nil
}

// CreateDeployment implements DeploymentService for the ServiceNow data source.
func (s *snDeploymentService) CreateDeployment(ctx context.Context, req domain.CreateDeploymentRequest) (domain.CreateDeploymentResponse, error) {
	snResp, err := s.createDeploymentSN(ctx, req)
	if err != nil {
		return domain.CreateDeploymentResponse{}, err
	}

	createdOn, err := time.Parse(snCreatedOnLayout, snResp.Deployment.CreatedOn)
	if err != nil {
		return domain.CreateDeploymentResponse{}, fmt.Errorf("sn create deployment: parse createdOn %q: %w", snResp.Deployment.CreatedOn, err)
	}

	return domain.CreateDeploymentResponse{
		Message: snResp.Message,
		Deployment: domain.CreatedDeployment{
			ID:        sysidToUUID(snResp.Deployment.ID),
			CreatedOn: createdOn,
			CreatedBy: snResp.Deployment.CreatedBy,
		},
	}, nil
}

// createDeploymentSNFirstDetails implements deploymentSNCreator (see
// deployment_service.go) -- the dual-write CREATE path's entry point into
// this service, returning the fields Postgres's own insert needs (including
// Number, which the public CreateDeployment above does not expose) rather
// than the wire-shaped domain.CreateDeploymentResponse.
func (s *snDeploymentService) createDeploymentSNFirstDetails(ctx context.Context, req domain.CreateDeploymentRequest) (id, number, createdBy string, createdOn time.Time, err error) {
	snResp, err := s.createDeploymentSN(ctx, req)
	if err != nil {
		return "", "", "", time.Time{}, err
	}
	createdOn, err = time.Parse(snCreatedOnLayout, snResp.Deployment.CreatedOn)
	if err != nil {
		return "", "", "", time.Time{}, fmt.Errorf("sn create deployment: parse createdOn %q: %w", snResp.Deployment.CreatedOn, err)
	}
	// deployment.number is NOT NULL UNIQUE on the Postgres side (see
	// createDeploymentSNFirst's own doc comment) -- an empty id/number here
	// would either fail the Postgres insert with an opaque constraint
	// violation or, worse, succeed with a blank number that later collides
	// with a real one. Caught here, before it ever reaches the repository.
	if snResp.Deployment.ID == "" {
		return "", "", "", time.Time{}, &apierror.ValidationError{Msg: "sn create deployment: response id is required"}
	}
	if snResp.Deployment.Number == "" {
		return "", "", "", time.Time{}, &apierror.ValidationError{Msg: "sn create deployment: response number is required"}
	}
	return sysidToUUID(snResp.Deployment.ID), snResp.Deployment.Number, snResp.Deployment.CreatedBy, createdOn, nil
}

// snUpdateDeploymentPayload is the Choreo PATCH /deployments/{id} request body.
// Description is json.RawMessage so an explicit null ("description":null) can be
// distinguished from an omitted field — omitempty drops nil RawMessage entirely.
type snUpdateDeploymentPayload struct {
	Name        *string         `json:"name,omitempty"`
	TypeKey     *int            `json:"typeKey,omitempty"`
	Description json.RawMessage `json:"description,omitempty"`
	Active      *bool           `json:"active,omitempty"`
}

type snUpdateDeploymentResponse struct {
	Message    string `json:"message"`
	Deployment struct {
		ID        string `json:"id"`
		UpdatedOn string `json:"updatedOn"`
		UpdatedBy string `json:"updatedBy"`
	} `json:"deployment"`
}

// UpdateDeployment implements DeploymentService for the ServiceNow data source.
func (s *snDeploymentService) UpdateDeployment(ctx context.Context, req domain.UpdateDeploymentRequest) (domain.UpdateDeploymentResponse, error) {
	if err := validateUpdateDeploymentRequest(req); err != nil {
		return domain.UpdateDeploymentResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)

	payload := snUpdateDeploymentPayload{
		Name:   req.Name,
		Active: req.Active,
	}
	if req.Type != nil {
		k := deploymentTypeToKey[*req.Type]
		payload.TypeKey = &k
	}
	if req.Description != nil {
		if *req.Description == nil {
			payload.Description = json.RawMessage("null")
		} else {
			b, err := json.Marshal(**req.Description)
			if err != nil {
				return domain.UpdateDeploymentResponse{}, fmt.Errorf("sn update deployment: marshal description: %w", err)
			}
			payload.Description = b
		}
	}

	raw, err := s.client.Patch(ctx, "/deployments/"+uuidToSysid(req.ID), token, payload)
	if err != nil {
		return domain.UpdateDeploymentResponse{}, err
	}

	var snResp snUpdateDeploymentResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.UpdateDeploymentResponse{}, fmt.Errorf("sn update deployment: parse response: %w", err)
	}

	updatedOn, err := time.Parse(snCreatedOnLayout, snResp.Deployment.UpdatedOn)
	if err != nil {
		return domain.UpdateDeploymentResponse{}, fmt.Errorf("sn update deployment: parse updatedOn %q: %w", snResp.Deployment.UpdatedOn, err)
	}

	return domain.UpdateDeploymentResponse{
		Message: snResp.Message,
		Deployment: domain.UpdatedDeployment{
			ID:        sysidToUUID(snResp.Deployment.ID),
			UpdatedOn: updatedOn,
			UpdatedBy: snResp.Deployment.UpdatedBy,
		},
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

// deploymentTypeToKey maps the domain DeploymentType string to the ServiceNow integer choice-list key.
var deploymentTypeToKey = map[domain.DeploymentType]int{
	domain.DeploymentTypeDevelopment:       1,
	domain.DeploymentTypeQA:                2,
	domain.DeploymentTypeStaging:           3,
	domain.DeploymentTypeStress:            4,
	domain.DeploymentTypeUAT:               5,
	domain.DeploymentTypePrimaryProduction: 6,
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
