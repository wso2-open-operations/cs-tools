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
	"sort"
	"strings"

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
	if req.Filters != nil && req.Filters.SupportGroupsOnly {
		return s.searchSupportGroups(ctx, token, req.Filters.SearchQuery, req.Pagination)
	}

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

// searchSupportGroups is SearchGroups with supportGroupsOnly: the support
// groups of ServiceNow's services, the set an explicit assignmentGroupId must
// belong to on DATA_SOURCE=servicenow (snIncidentService.assignmentGroupLookups).
//
// ServiceNow's group search has no such filter, so the set is built from one
// complete scan of the services (scanSNServices: an inconclusive scan is an
// error, never a partial list), then matched against searchQuery
// (case-insensitive, anywhere in the name), sorted by name and paged here.
// The service list carries no group's active flag, so every group is reported
// active and an inactive one is not left out.
func (s *snGroupService) searchSupportGroups(ctx context.Context, token, searchQuery string, p domain.Pagination) (domain.SearchGroupsResponse, error) {
	byID := map[string]domain.Group{}
	if _, err := scanSNServices(ctx, s.client, token, "listing the support groups of ServiceNow's services",
		func(svc snITService) bool {
			if svc.SupportGroup == nil || svc.SupportGroup.ID == "" {
				return false
			}
			id := sysidToUUID(svc.SupportGroup.ID)
			if _, seen := byID[id]; !seen {
				byID[id] = domain.Group{ID: id, Name: nameOrID(svc.SupportGroup.Label, id), Active: true}
			}
			return false
		}); err != nil {
		return domain.SearchGroupsResponse{}, err
	}

	query := strings.ToLower(strings.TrimSpace(searchQuery))
	matched := make([]domain.Group, 0, len(byID))
	for _, g := range byID {
		if query == "" || strings.Contains(strings.ToLower(g.Name), query) {
			matched = append(matched, g)
		}
	}
	sort.Slice(matched, func(i, j int) bool {
		a, b := strings.ToLower(matched[i].Name), strings.ToLower(matched[j].Name)
		if a != b {
			return a < b
		}
		return matched[i].ID < matched[j].ID
	})

	page := []domain.Group{}
	if p.Offset < len(matched) {
		end := p.Offset + p.Limit
		if end > len(matched) {
			end = len(matched)
		}
		page = matched[p.Offset:end]
	}
	return domain.SearchGroupsResponse{Groups: page, Total: len(matched), Limit: p.Limit, Offset: p.Offset}, nil
}
