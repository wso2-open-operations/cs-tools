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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	integrationservice "github.com/wso2-open-operations/cs-tools/entity-service/internal/servicenow-integration-service"
)

// snServiceOfferingsResponse mirrors the Choreo POST /service-offerings/search response.
type snServiceOfferingsResponse struct {
	ServiceOfferings []snServiceOffering `json:"serviceOfferings"`
	TotalRecords     int                 `json:"totalRecords"`
	Offset           int                 `json:"offset"`
	Limit            int                 `json:"limit"`
}

type snServiceOffering struct {
	ID      string              `json:"id"`
	Name    string              `json:"name"`
	Service *snOfferingService  `json:"service"`
}

type snOfferingService struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// snServiceOfferingSearchPayload is the Choreo POST /service-offerings/search request body.
type snServiceOfferingSearchPayload struct {
	Filters    snServiceOfferingFilters `json:"filters"`
	Pagination snProjectPagination      `json:"pagination"`
}

type snServiceOfferingFilters struct {
	ServiceIDs  []string `json:"serviceIds,omitempty"`
	SearchQuery string   `json:"searchQuery,omitempty"`
}

type snServiceOfferingService struct {
	client *integrationservice.Client
}

// NewServiceNowServiceOfferingService constructs a ServiceOfferingService backed by the Choreo API.
func NewServiceNowServiceOfferingService(client *integrationservice.Client) ServiceOfferingService {
	return &snServiceOfferingService{client: client}
}

// SearchServiceOfferings implements ServiceOfferingService.
func (s *snServiceOfferingService) SearchServiceOfferings(ctx context.Context, req domain.SearchServiceOfferingsRequest) (domain.SearchServiceOfferingsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchServiceOfferingsResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return domain.SearchServiceOfferingsResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}

	var filters snServiceOfferingFilters
	if req.Filters != nil {
		filters.ServiceIDs = uuidsToSysids(req.Filters.ServiceIDs)
		filters.SearchQuery = req.Filters.SearchQuery
	}

	payload := snServiceOfferingSearchPayload{
		Filters:    filters,
		Pagination: snProjectPagination{Limit: req.Pagination.Limit, Offset: req.Pagination.Offset},
	}
	raw, err := s.client.Post(ctx, "/service-offerings/search", token, payload)
	if err != nil {
		return domain.SearchServiceOfferingsResponse{}, err
	}

	var snResp snServiceOfferingsResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.SearchServiceOfferingsResponse{}, fmt.Errorf("sn service offerings: parse response: %w", err)
	}

	offerings := make([]domain.ServiceOffering, 0, len(snResp.ServiceOfferings))
	for _, o := range snResp.ServiceOfferings {
		item := domain.ServiceOffering{
			ID:   sysidToUUID(o.ID),
			Name: o.Name,
		}
		if o.Service != nil {
			item.Service = &domain.ServiceOfferingServiceRef{
				ID:   sysidToUUID(o.Service.ID),
				Name: o.Service.Name,
			}
		}
		offerings = append(offerings, item)
	}

	return domain.SearchServiceOfferingsResponse{
		ServiceOfferings: offerings,
		Total:            snResp.TotalRecords,
		Limit:            req.Pagination.Limit,
		Offset:           req.Pagination.Offset,
	}, nil
}
