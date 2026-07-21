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

// snConfigurationItemsResponse mirrors the Choreo POST /configuration-items/search response.
type snConfigurationItemsResponse struct {
	ConfigurationItems []snConfigurationItem `json:"configurationItems"`
	TotalRecords       int                   `json:"totalRecords"`
	Offset             int                   `json:"offset"`
	Limit              int                   `json:"limit"`
}

type snConfigurationItem struct {
	ID          string  `json:"id"`
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Class       *string `json:"class"`
}

// snConfigurationItemSearchPayload is the Choreo POST /configuration-items/search request body.
type snConfigurationItemSearchPayload struct {
	Filters    snConfigurationItemFilters `json:"filters"`
	Pagination snProjectPagination        `json:"pagination"`
}

type snConfigurationItemFilters struct {
	SearchQuery string `json:"searchQuery,omitempty"`
}

type snConfigurationItemService struct {
	client *integrationservice.Client
}

// NewServiceNowConfigurationItemService constructs a ConfigurationItemService backed by the Choreo API.
func NewServiceNowConfigurationItemService(client *integrationservice.Client) ConfigurationItemService {
	return &snConfigurationItemService{client: client}
}

// SearchConfigurationItems implements ConfigurationItemService.
func (s *snConfigurationItemService) SearchConfigurationItems(ctx context.Context, req domain.SearchConfigurationItemsRequest) (domain.SearchConfigurationItemsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchConfigurationItemsResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)

	var filters snConfigurationItemFilters
	if req.Filters != nil {
		filters.SearchQuery = req.Filters.SearchQuery
	}

	payload := snConfigurationItemSearchPayload{
		Filters:    filters,
		Pagination: snProjectPagination{Limit: req.Pagination.Limit, Offset: req.Pagination.Offset},
	}
	raw, err := s.client.Post(ctx, "/configuration-items/search", token, payload)
	if err != nil {
		return domain.SearchConfigurationItemsResponse{}, err
	}

	var snResp snConfigurationItemsResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.SearchConfigurationItemsResponse{}, fmt.Errorf("sn configuration items: parse response: %w", err)
	}

	items := make([]domain.ConfigurationItem, 0, len(snResp.ConfigurationItems))
	for _, ci := range snResp.ConfigurationItems {
		items = append(items, domain.ConfigurationItem{
			ID:          sysidToUUID(ci.ID),
			Name:        ci.Name,
			Description: ci.Description,
			Class:       ci.Class,
		})
	}

	return domain.SearchConfigurationItemsResponse{
		ConfigurationItems: items,
		Total:              snResp.TotalRecords,
		Limit:              req.Pagination.Limit,
		Offset:             req.Pagination.Offset,
	}, nil
}
