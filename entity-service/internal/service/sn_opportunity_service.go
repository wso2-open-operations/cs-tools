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

// snOpportunitiesResponse mirrors the Choreo POST /opportunities/search response.
type snOpportunitiesResponse struct {
	Opportunities []snOpportunity `json:"opportunities"`
	TotalRecords  int             `json:"totalRecords"`
	Offset        int             `json:"offset"`
	Limit         int             `json:"limit"`
}

// snOpportunity mirrors the Choreo Opportunity shape. Every field but ID is nilable:
// ServiceNow can omit any of them entirely for a sparsely-populated row.
type snOpportunity struct {
	ID                 string           `json:"id"`
	Name               *string          `json:"name"`
	Account            *snCaseEntityRef `json:"account"`
	EulaVersion        *string          `json:"eulaVersion"`
	EulaVersionDecimal *string          `json:"eulaVersionDecimal"`
	Stage              *string          `json:"stage"`
}

// snOpportunitySearchPayload is the Choreo POST /opportunities/search request body.
type snOpportunitySearchPayload struct {
	Filters    snOpportunityFilters `json:"filters,omitempty"`
	Pagination snProjectPagination  `json:"pagination"`
}

type snOpportunityFilters struct {
	AccountID string `json:"accountId,omitempty"`
}

type snOpportunityService struct {
	client *integrationservice.Client
}

// NewServiceNowOpportunityService constructs an OpportunityService backed by the Choreo API.
func NewServiceNowOpportunityService(client *integrationservice.Client) OpportunityService {
	return &snOpportunityService{client: client}
}

// snEntityRefFromCaseRef converts a snCaseEntityRef reference to a domain.EntityRef, returning
// nil when the reference is absent or has an empty id.
func snEntityRefFromCaseRef(ref *snCaseEntityRef) *domain.EntityRef {
	if ref == nil {
		return nil
	}
	if id := sysidToUUID(ref.ID); id != "" {
		return &domain.EntityRef{ID: id, Name: ref.Name}
	}
	return nil
}

func snOpportunityToDomain(o snOpportunity) domain.Opportunity {
	return domain.Opportunity{
		ID:                 sysidToUUID(o.ID),
		Name:               o.Name,
		Account:            snEntityRefFromCaseRef(o.Account),
		EulaVersion:        o.EulaVersion,
		EulaVersionDecimal: o.EulaVersionDecimal,
		Stage:              o.Stage,
	}
}

// SearchOpportunities implements OpportunityService.
func (s *snOpportunityService) SearchOpportunities(ctx context.Context, req domain.SearchOpportunitiesRequest) (domain.SearchOpportunitiesResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchOpportunitiesResponse{}, err
	}
	var accountSysid string
	if req.AccountID != "" {
		if err := validateUUIDs("accountId", []string{req.AccountID}); err != nil {
			return domain.SearchOpportunitiesResponse{}, err
		}
		accountSysid = uuidToSysid(req.AccountID)
	}

	token := middleware.UserIDTokenFromContext(ctx)

	payload := snOpportunitySearchPayload{
		Filters:    snOpportunityFilters{AccountID: accountSysid},
		Pagination: snProjectPagination{Limit: req.Pagination.Limit, Offset: req.Pagination.Offset},
	}

	raw, err := s.client.Post(ctx, "/opportunities/search", token, payload)
	if err != nil {
		return domain.SearchOpportunitiesResponse{}, err
	}

	var snResp snOpportunitiesResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.SearchOpportunitiesResponse{}, fmt.Errorf("sn opportunities: parse response: %w", err)
	}

	opportunities := make([]domain.Opportunity, 0, len(snResp.Opportunities))
	for _, o := range snResp.Opportunities {
		opportunities = append(opportunities, snOpportunityToDomain(o))
	}

	return domain.SearchOpportunitiesResponse{
		Opportunities: opportunities,
		Total:         snResp.TotalRecords,
		Limit:         req.Pagination.Limit,
		Offset:        req.Pagination.Offset,
		HasMore:       req.Pagination.Offset+len(opportunities) < snResp.TotalRecords,
	}, nil
}

// GetOpportunityByID implements OpportunityService.
func (s *snOpportunityService) GetOpportunityByID(ctx context.Context, id string) (domain.Opportunity, error) {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.Opportunity{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)

	raw, err := s.client.Get(ctx, "/opportunities/"+uuidToSysid(id), token)
	if err != nil {
		return domain.Opportunity{}, err
	}

	var o snOpportunity
	if err := json.Unmarshal(raw, &o); err != nil {
		return domain.Opportunity{}, fmt.Errorf("sn opportunities: parse opportunity response: %w", err)
	}

	return snOpportunityToDomain(o), nil
}
