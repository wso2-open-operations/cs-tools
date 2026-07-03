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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	integrationservice "github.com/wso2-open-operations/cs-tools/entity-service/internal/servicenow-integration-service"
)

// snProductVersionsResponse mirrors the Choreo POST /products/{id}/versions/search response.
type snProductVersionsResponse struct {
	Versions     []snProductVersion `json:"versions"`
	TotalRecords int                `json:"totalRecords"`
	Offset       int                `json:"offset"`
	Limit        int                `json:"limit"`
}

type snProductVersion struct {
	ID                             string  `json:"id"`
	Version                        string  `json:"version"`
	CurrentSupportStatus           string  `json:"currentSupportStatus"`
	ReleaseDate                    string  `json:"releaseDate"`
	SupportEolDate                 *string `json:"supportEolDate"`
	EarliestPossibleSupportEolDate *string `json:"earliestPossibleSupportEolDate"`
}

// snProductVersionSearchPayload is the Choreo POST /products/{id}/versions/search request body.
type snProductVersionSearchPayload struct {
	Filters    snProductVersionFilters `json:"filters,omitempty"`
	Pagination snProjectPagination     `json:"pagination"`
}

type snProductVersionFilters struct {
	SearchQuery string `json:"searchQuery,omitempty"`
}

type snProductVersionService struct {
	client *integrationservice.Client
}

// NewServiceNowProductVersionService constructs an SNProductVersionService backed by the Choreo API.
func NewServiceNowProductVersionService(client *integrationservice.Client) SNProductVersionService {
	return &snProductVersionService{client: client}
}

func (s *snProductVersionService) SearchProductVersions(ctx context.Context, req domain.SearchProductVersionsRequest) (domain.SearchSNProductVersionsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchSNProductVersionsResponse{}, err
	}
	if err := validateSearchQuery(req.SearchQuery); err != nil {
		return domain.SearchSNProductVersionsResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return domain.SearchSNProductVersionsResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}

	productSysid := uuidToSysid(req.ProductID)
	path := fmt.Sprintf("/products/%s/versions/search", productSysid)

	payload := snProductVersionSearchPayload{
		Filters:    snProductVersionFilters{SearchQuery: req.SearchQuery},
		Pagination: snProjectPagination{Limit: req.Pagination.Limit, Offset: req.Pagination.Offset},
	}

	raw, err := s.client.Post(ctx, path, token, payload)
	if err != nil {
		return domain.SearchSNProductVersionsResponse{}, err
	}

	var snResp snProductVersionsResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.SearchSNProductVersionsResponse{}, fmt.Errorf("sn product versions: parse response: %w", err)
	}

	versions := make([]domain.SNProductVersion, 0, len(snResp.Versions))
	for _, v := range snResp.Versions {
		pv := domain.SNProductVersion{
			ID:                   sysidToUUID(v.ID),
			ProductID:            req.ProductID,
			Version:              v.Version,
			CurrentSupportStatus: v.CurrentSupportStatus,
			ReleaseDate:          v.ReleaseDate,
		}
		if v.SupportEolDate != nil && *v.SupportEolDate != "" {
			pv.SupportEOLDate = v.SupportEolDate
		}
		if v.EarliestPossibleSupportEolDate != nil && *v.EarliestPossibleSupportEolDate != "" {
			pv.EarliestPossibleSupportEOLDate = v.EarliestPossibleSupportEolDate
		}
		versions = append(versions, pv)
	}

	total := snResp.TotalRecords
	return domain.SearchSNProductVersionsResponse{
		ProductVersions: versions,
		Total:           total,
		Limit:           req.Pagination.Limit,
		Offset:          req.Pagination.Offset,
		HasMore:         req.Pagination.Offset+len(versions) < total,
	}, nil
}
