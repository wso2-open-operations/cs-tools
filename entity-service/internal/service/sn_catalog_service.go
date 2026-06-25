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

// snCatalogItem mirrors an item within a catalog from the Choreo response.
type snCatalogItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// snCatalog mirrors a single catalog from the Choreo response.
type snCatalog struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	CatalogItems []snCatalogItem `json:"catalogItems"`
}

// snCatalogsResponse mirrors the Choreo POST /catalogs/search response.
type snCatalogsResponse struct {
	Catalogs     []snCatalog `json:"catalogs"`
	TotalRecords int         `json:"totalRecords"`
	Limit        int         `json:"limit"`
	Offset       int         `json:"offset"`
}

// snSearchCatalogsPayload is the Choreo POST /catalogs/search request body.
type snSearchCatalogsPayload struct {
	DeployedProductID string              `json:"deployedProductId"`
	Pagination        snProjectPagination `json:"pagination"`
}

// snCatalogItemVariable mirrors a single variable from the Choreo catalog-item-variables response.
type snCatalogItemVariable struct {
	ID           string `json:"id"`
	QuestionText string `json:"questionText"`
	Order        int    `json:"order"`
	Type         string `json:"type"`
}

// snCatalogItemVariablesResponse mirrors the Choreo GET /catalogs/{id}/items/{id}/variables response.
type snCatalogItemVariablesResponse struct {
	Variables []snCatalogItemVariable `json:"variables"`
}

type snCatalogService struct {
	client *integrationservice.Client
}

// NewServiceNowCatalogService constructs a CatalogService backed by the Choreo API.
func NewServiceNowCatalogService(client *integrationservice.Client) CatalogService {
	return &snCatalogService{client: client}
}

func (s *snCatalogService) SearchCatalogs(ctx context.Context, req domain.SearchCatalogsRequest) (domain.SearchCatalogsResponse, error) {
	if req.DeployedProductID == "" {
		return domain.SearchCatalogsResponse{}, &apierror.ValidationError{Msg: "deployedProductId is required"}
	}
	if err := validateUUIDs("deployedProductId", []string{req.DeployedProductID}); err != nil {
		return domain.SearchCatalogsResponse{}, err
	}
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchCatalogsResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return domain.SearchCatalogsResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}

	payload := snSearchCatalogsPayload{
		DeployedProductID: uuidToSysid(req.DeployedProductID),
		Pagination:        snProjectPagination{Limit: req.Pagination.Limit, Offset: req.Pagination.Offset},
	}

	raw, err := s.client.Post(ctx, "/catalogs/search", token, payload)
	if err != nil {
		return domain.SearchCatalogsResponse{}, err
	}

	var snResp snCatalogsResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.SearchCatalogsResponse{}, fmt.Errorf("sn search catalogs: parse response: %w", err)
	}

	catalogs := make([]domain.Catalog, 0, len(snResp.Catalogs))
	for _, c := range snResp.Catalogs {
		items := make([]domain.CatalogItem, 0, len(c.CatalogItems))
		for _, item := range c.CatalogItems {
			items = append(items, domain.CatalogItem{
				ID:   sysidToUUID(item.ID),
				Name: item.Name,
			})
		}
		catalogs = append(catalogs, domain.Catalog{
			ID:           sysidToUUID(c.ID),
			Name:         c.Name,
			CatalogItems: items,
		})
	}

	return domain.SearchCatalogsResponse{
		Catalogs: catalogs,
		Total:    snResp.TotalRecords,
		Limit:    snResp.Limit,
		Offset:   snResp.Offset,
	}, nil
}

func (s *snCatalogService) GetCatalogItemVariables(ctx context.Context, catalogID, catalogItemID string) (domain.GetCatalogItemVariablesResponse, error) {
	if catalogID == "" {
		return domain.GetCatalogItemVariablesResponse{}, &apierror.ValidationError{Msg: "catalogId is required"}
	}
	if catalogItemID == "" {
		return domain.GetCatalogItemVariablesResponse{}, &apierror.ValidationError{Msg: "catalogItemId is required"}
	}
	if err := validateUUIDs("catalogId", []string{catalogID}); err != nil {
		return domain.GetCatalogItemVariablesResponse{}, err
	}
	if err := validateUUIDs("catalogItemId", []string{catalogItemID}); err != nil {
		return domain.GetCatalogItemVariablesResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return domain.GetCatalogItemVariablesResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}

	path := "/catalogs/" + uuidToSysid(catalogID) + "/items/" + uuidToSysid(catalogItemID) + "/variables"
	raw, err := s.client.Get(ctx, path, token)
	if err != nil {
		return domain.GetCatalogItemVariablesResponse{}, err
	}

	var snResp snCatalogItemVariablesResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.GetCatalogItemVariablesResponse{}, fmt.Errorf("sn get catalog item variables: parse response: %w", err)
	}

	variables := make([]domain.CatalogItemVariable, 0, len(snResp.Variables))
	for _, v := range snResp.Variables {
		variables = append(variables, domain.CatalogItemVariable{
			ID:           sysidToUUID(v.ID),
			QuestionText: v.QuestionText,
			Order:        v.Order,
			Type:         v.Type,
		})
	}

	return domain.GetCatalogItemVariablesResponse{Variables: variables}, nil
}
