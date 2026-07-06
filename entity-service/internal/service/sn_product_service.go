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

// snProductsResponse mirrors the Choreo POST /products/search response.
type snProductsResponse struct {
	Products     []snProduct `json:"products"`
	TotalRecords int         `json:"totalRecords"`
	Offset       int         `json:"offset"`
	Limit        int         `json:"limit"`
}

type snProduct struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Class string `json:"class"`
}

// snProductSearchPayload is the Choreo POST /products/search request body.
type snProductSearchPayload struct {
	Filters    snProductFilters    `json:"filters,omitempty"`
	Pagination snProjectPagination `json:"pagination"`
}

type snProductFilters struct {
	SearchQuery string `json:"searchQuery,omitempty"`
}

type snProductService struct {
	client *integrationservice.Client
}

// NewServiceNowProductService constructs an SNProductService backed by the Choreo API.
func NewServiceNowProductService(client *integrationservice.Client) SNProductService {
	return &snProductService{client: client}
}

func (s *snProductService) SearchProducts(ctx context.Context, req domain.SearchProductsRequest) (domain.SearchSNProductsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchSNProductsResponse{}, err
	}
	if err := validateSearchQuery(req.SearchQuery); err != nil {
		return domain.SearchSNProductsResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return domain.SearchSNProductsResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}

	payload := snProductSearchPayload{
		Filters:    snProductFilters{SearchQuery: req.SearchQuery},
		Pagination: snProjectPagination{Limit: req.Pagination.Limit, Offset: req.Pagination.Offset},
	}

	raw, err := s.client.Post(ctx, "/products/search", token, payload)
	if err != nil {
		return domain.SearchSNProductsResponse{}, err
	}

	var snResp snProductsResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.SearchSNProductsResponse{}, fmt.Errorf("sn products: parse response: %w", err)
	}

	products := make([]domain.SNProduct, 0, len(snResp.Products))
	for _, p := range snResp.Products {
		products = append(products, domain.SNProduct{
			ID:    sysidToUUID(p.ID),
			Name:  p.Name,
			Class: p.Class,
		})
	}

	total := snResp.TotalRecords
	return domain.SearchSNProductsResponse{
		Products: products,
		Total:    total,
		Limit:    req.Pagination.Limit,
		Offset:   req.Pagination.Offset,
		HasMore:  req.Pagination.Offset+len(products) < total,
	}, nil
}
