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

// Package service is declared in interfaces.go.
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

// snProjectsResponse mirrors the Choreo POST /projects/search response.
type snProjectsResponse struct {
	Projects     []snProject `json:"projects"`
	TotalRecords int         `json:"totalRecords"`
	Offset       int         `json:"offset"`
	Limit        int         `json:"limit"`
}

type snProject struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	Key       string        `json:"key"`
	Type      snProjectType `json:"type"`
	CreatedOn string        `json:"createdOn"`
}

type snProjectType struct {
	Name string `json:"name"`
}

// snSearchProjectsPayload is the Choreo POST /projects/search request body.
type snSearchProjectsPayload struct {
	Filters    snProjectFilters    `json:"filters"`
	Pagination snProjectPagination `json:"pagination"`
}

type snProjectFilters struct {
	SearchQuery string `json:"searchQuery,omitempty"`
}

type snProjectPagination struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// snCreatedOnLayout is the time format used by the Choreo API.
const snCreatedOnLayout = "2006-01-02 15:04:05"

type snProjectService struct {
	client     *snclient.Client
	pgFallback ProjectService
}

// NewSNProjectService constructs a ProjectService backed by the Choreo API.
// pgFallback is used for GetProjectByID (no SN single-project endpoint).
func NewSNProjectService(client *snclient.Client, pgFallback ProjectService) ProjectService {
	return &snProjectService{client: client, pgFallback: pgFallback}
}

// SearchProjects implements ProjectService. It calls the Choreo API, parses the
// response, and maps each item to domain.ProjectView so the shape is identical
// to the Postgres path.
func (s *snProjectService) SearchProjects(ctx context.Context, req domain.SearchProjectsRequest) (domain.SearchProjectsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchProjectsResponse{}, err
	}
	if err := validateSearchQuery(req.SearchQuery); err != nil {
		return domain.SearchProjectsResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return domain.SearchProjectsResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}

	payload := snSearchProjectsPayload{
		Filters:    snProjectFilters{SearchQuery: req.SearchQuery},
		Pagination: snProjectPagination{Limit: req.Pagination.Limit, Offset: req.Pagination.Offset},
	}
	raw, err := s.client.Post(ctx, "/projects/search", token, payload)
	if err != nil {
		return domain.SearchProjectsResponse{}, err
	}

	var snResp snProjectsResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.SearchProjectsResponse{}, fmt.Errorf("sn projects: parse response: %w", err)
	}

	views := make([]domain.ProjectView, 0, len(snResp.Projects))
	for _, p := range snResp.Projects {
		createdOn, err := time.Parse(snCreatedOnLayout, p.CreatedOn)
		if err != nil {
			return domain.SearchProjectsResponse{}, fmt.Errorf("sn projects: parse createdOn %q: %w", p.CreatedOn, err)
		}
		subType, err := snTypeNameToSubscriptionType(p.Type.Name)
		if err != nil {
			return domain.SearchProjectsResponse{}, fmt.Errorf("sn projects: project %q: %w", p.ID, err)
		}
		views = append(views, domain.ProjectView{
			ID:               p.ID,
			Name:             p.Name,
			Key:              p.Key,
			SubscriptionType: subType,
			CreatedOn:        createdOn,
		})
	}

	total := snResp.TotalRecords
	return domain.SearchProjectsResponse{
		Projects: views,
		Total:    total,
		Limit:    req.Pagination.Limit,
		Offset:   req.Pagination.Offset,
		HasMore:  req.Pagination.Offset+len(views) < total,
	}, nil
}

// GetProjectByID implements ProjectService by delegating to the postgres service.
func (s *snProjectService) GetProjectByID(ctx context.Context, id string) (domain.Project, error) {
	return s.pgFallback.GetProjectByID(ctx, id)
}

// validSubscriptionTypes is the set of known SubscriptionType enum values.
var validSubscriptionTypes = map[domain.SubscriptionType]struct{}{
	domain.SubscriptionTypeDevelopmentSupport:       {},
	domain.SubscriptionTypeManagedCloudSubscription: {},
	domain.SubscriptionTypeEvaluationSubscription:   {},
	domain.SubscriptionTypeSubscription:             {},
	domain.SubscriptionTypeCloudEvaluationSupport:   {},
	domain.SubscriptionTypeInternal:                 {},
	domain.SubscriptionTypePlatformerSubscription:   {},
	domain.SubscriptionTypeCloudSupport:             {},
	domain.SubscriptionTypeProfessionalServices:     {},
}

// snTypeNameToSubscriptionType converts a SN project type name (e.g. "Cloud Support")
// to the domain SubscriptionType enum (e.g. "cloud_support"). Returns an error
// if the converted value is not a known enum value.
func snTypeNameToSubscriptionType(name string) (domain.SubscriptionType, error) {
	st := domain.SubscriptionType(strings.ToLower(strings.ReplaceAll(name, " ", "_")))
	if _, ok := validSubscriptionTypes[st]; !ok {
		return "", fmt.Errorf("unknown subscription type %q from ServiceNow", name)
	}
	return st, nil
}
