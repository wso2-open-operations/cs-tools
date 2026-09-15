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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	integrationservice "github.com/wso2-open-operations/cs-tools/entity-service/internal/servicenow-integration-service"
)

// snProjectOpportunityLinksResponse mirrors the Choreo POST /project-opportunity-links/search
// response.
type snProjectOpportunityLinksResponse struct {
	Links        []snProjectOpportunityLink `json:"links"`
	TotalRecords int                        `json:"totalRecords"`
	Offset       int                        `json:"offset"`
	Limit        int                        `json:"limit"`
}

// snProjectOpportunityLink mirrors the Choreo ProjectOpportunityLink shape. Every field but ID
// is nilable: ServiceNow can omit either reference entirely for a sparsely-populated row.
type snProjectOpportunityLink struct {
	ID          string           `json:"id"`
	Project     *snCaseEntityRef `json:"project"`
	Opportunity *snCaseEntityRef `json:"opportunity"`
}

// snProjectOpportunityLinkSearchPayload is the Choreo POST /project-opportunity-links/search
// request body.
type snProjectOpportunityLinkSearchPayload struct {
	Filters    snProjectOpportunityLinkFilters `json:"filters,omitempty"`
	Pagination snProjectPagination             `json:"pagination"`
}

type snProjectOpportunityLinkFilters struct {
	ProjectID     string `json:"projectId,omitempty"`
	OpportunityID string `json:"opportunityId,omitempty"`
}

type snProjectOpportunityLinkService struct {
	client *integrationservice.Client
}

// NewServiceNowProjectOpportunityLinkService constructs a ProjectOpportunityLinkService backed
// by the Choreo API.
func NewServiceNowProjectOpportunityLinkService(client *integrationservice.Client) ProjectOpportunityLinkService {
	return &snProjectOpportunityLinkService{client: client}
}

func snProjectOpportunityLinkToDomain(l snProjectOpportunityLink) domain.ProjectOpportunityLink {
	return domain.ProjectOpportunityLink{
		ID:          sysidToUUID(l.ID),
		Project:     snEntityRefFromCaseRef(l.Project),
		Opportunity: snEntityRefFromCaseRef(l.Opportunity),
	}
}

// SearchProjectOpportunityLinks implements ProjectOpportunityLinkService.
func (s *snProjectOpportunityLinkService) SearchProjectOpportunityLinks(ctx context.Context, req domain.SearchProjectOpportunityLinksRequest) (domain.SearchProjectOpportunityLinksResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchProjectOpportunityLinksResponse{}, err
	}
	var projectSysid, opportunitySysid string
	if req.ProjectID != "" {
		if err := validateUUIDs("projectId", []string{req.ProjectID}); err != nil {
			return domain.SearchProjectOpportunityLinksResponse{}, err
		}
		projectSysid = uuidToSysid(req.ProjectID)
	}
	if req.OpportunityID != "" {
		if err := validateUUIDs("opportunityId", []string{req.OpportunityID}); err != nil {
			return domain.SearchProjectOpportunityLinksResponse{}, err
		}
		opportunitySysid = uuidToSysid(req.OpportunityID)
	}

	token := middleware.UserIDTokenFromContext(ctx)

	payload := snProjectOpportunityLinkSearchPayload{
		Filters:    snProjectOpportunityLinkFilters{ProjectID: projectSysid, OpportunityID: opportunitySysid},
		Pagination: snProjectPagination{Limit: req.Pagination.Limit, Offset: req.Pagination.Offset},
	}

	raw, err := s.client.Post(ctx, "/project-opportunity-links/search", token, payload)
	if err != nil {
		return domain.SearchProjectOpportunityLinksResponse{}, err
	}

	var snResp snProjectOpportunityLinksResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.SearchProjectOpportunityLinksResponse{}, fmt.Errorf("sn project-opportunity links: parse response: %w", err)
	}

	links := make([]domain.ProjectOpportunityLink, 0, len(snResp.Links))
	for _, l := range snResp.Links {
		links = append(links, snProjectOpportunityLinkToDomain(l))
	}

	return domain.SearchProjectOpportunityLinksResponse{
		Links:   links,
		Total:   snResp.TotalRecords,
		Limit:   req.Pagination.Limit,
		Offset:  req.Pagination.Offset,
		HasMore: req.Pagination.Offset+len(links) < snResp.TotalRecords,
	}, nil
}
