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

// snGroupsResponse mirrors the Choreo POST /groups/search response.
type snGroupsResponse struct {
	Groups       []snGroup `json:"groups"`
	TotalRecords int       `json:"totalRecords"`
	Offset       int       `json:"offset"`
	Limit        int       `json:"limit"`
}

type snGroup struct {
	ID     string         `json:"id"`
	Name   string         `json:"name"`
	Active bool           `json:"active"`
	Parent *snGroupParent `json:"parent"`
}

type snGroupParent struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// snGroupSearchPayload is the Choreo POST /groups/search request body.
type snGroupSearchPayload struct {
	Filters    snGroupFilters      `json:"filters"`
	Pagination snProjectPagination `json:"pagination"`
}

type snGroupFilters struct {
	SearchQuery string `json:"searchQuery,omitempty"`
}

type snGroupService struct {
	client *integrationservice.Client
}

// NewServiceNowGroupService constructs a GroupService backed by the Choreo API.
func NewServiceNowGroupService(client *integrationservice.Client) GroupService {
	return &snGroupService{client: client}
}

// SearchGroups implements GroupService.
func (s *snGroupService) SearchGroups(ctx context.Context, req domain.SearchGroupsRequest) (domain.SearchGroupsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchGroupsResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)

	var filters snGroupFilters
	if req.Filters != nil {
		filters.SearchQuery = req.Filters.SearchQuery
	}

	payload := snGroupSearchPayload{
		Filters:    filters,
		Pagination: snProjectPagination{Limit: req.Pagination.Limit, Offset: req.Pagination.Offset},
	}
	raw, err := s.client.Post(ctx, "/groups/search", token, payload)
	if err != nil {
		return domain.SearchGroupsResponse{}, err
	}

	var snResp snGroupsResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.SearchGroupsResponse{}, fmt.Errorf("sn groups: parse response: %w", err)
	}

	groups := make([]domain.Group, 0, len(snResp.Groups))
	for _, g := range snResp.Groups {
		item := domain.Group{
			ID:     sysidToUUID(g.ID),
			Name:   g.Name,
			Active: g.Active,
		}
		if g.Parent != nil {
			item.Parent = &domain.GroupParentRef{
				ID:   sysidToUUID(g.Parent.ID),
				Name: g.Parent.Name,
			}
		}
		groups = append(groups, item)
	}

	return domain.SearchGroupsResponse{
		Groups: groups,
		Total:  snResp.TotalRecords,
		Limit:  req.Pagination.Limit,
		Offset: req.Pagination.Offset,
	}, nil
}
