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
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	integrationservice "github.com/wso2-open-operations/cs-tools/entity-service/internal/servicenow-integration-service"
)

// snDeployedProductsResponse mirrors the SN integration service POST /deployed-products/search response.
type snDeployedProductsResponse struct {
	DeployedProducts []snDeployedProduct `json:"deployedProducts"`
	TotalRecords     int                 `json:"totalRecords"`
	Offset           int                 `json:"offset"`
	Limit            int                 `json:"limit"`
}

type snDeployedProduct struct {
	ID         string                    `json:"id"`
	Deployment snDeployedProductRef      `json:"deployment"`
	Product    snDeployedProductRef      `json:"product"`
	Version    *snDeployedProductVersion `json:"version"`
	Cores      *int                      `json:"cores"`
	TPS        *float64                  `json:"tps"` // Ballerina decimal? serialises as 100.0
	Category   *string                   `json:"category"`
	CreatedOn  string                    `json:"createdOn"`
	UpdatedOn  string                    `json:"updatedOn"`
}

type snDeployedProductRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type snDeployedProductVersion struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	ReleasedDate   *string `json:"releasedOn"`
	SupportEoLDate *string `json:"endOfLifeOn"`
}

// snDeployedProductSearchPayload is the SN integration service POST /deployed-products/search request body.
type snDeployedProductSearchPayload struct {
	Filters    snDeployedProductFilters `json:"filters"`
	Pagination snProjectPagination      `json:"pagination"`
}

type snDeployedProductFilters struct {
	DeploymentIDs []string `json:"deploymentIds,omitempty"`
}

type snDeployedProductService struct {
	client *integrationservice.Client
}

// NewServiceNowDeployedProductService constructs a DeployedProductService backed by the SN integration service.
func NewServiceNowDeployedProductService(client *integrationservice.Client) DeployedProductService {
	return &snDeployedProductService{client: client}
}

// snCreateDeployedProductPayload is the Choreo POST /deployed-products request body.
type snCreateDeployedProductPayload struct {
	ProjectID    string `json:"projectId"`
	DeploymentID string `json:"deploymentId"`
	ProductID    string `json:"productId"`
	VersionID    string `json:"versionId"`
	Cores        *int   `json:"cores,omitempty"`
	TPS          *int   `json:"tps,omitempty"`
	Description  *string `json:"description,omitempty"`
}

type snCreateDeployedProductResponse struct {
	Message         string `json:"message"`
	DeployedProduct struct {
		ID        string `json:"id"`
		CreatedOn string `json:"createdOn"`
		CreatedBy string `json:"createdBy"`
	} `json:"deployedProduct"`
}

// snUpdateDeployedProductPayload is the Choreo PATCH /deployed-products/{id} request body.
// Description is json.RawMessage so an explicit null can be distinguished from an omitted field.
type snUpdateDeployedProductPayload struct {
	Cores       *int            `json:"cores,omitempty"`
	TPS         *int            `json:"tps,omitempty"`
	Description json.RawMessage `json:"description,omitempty"`
	Active      *bool           `json:"active,omitempty"`
}

type snUpdateDeployedProductResponse struct {
	Message         string `json:"message"`
	DeployedProduct struct {
		ID        string `json:"id"`
		UpdatedOn string `json:"updatedOn"`
		UpdatedBy string `json:"updatedBy"`
	} `json:"deployedProduct"`
}

// CreateDeployedProduct implements DeployedProductService for the ServiceNow data source.
func (s *snDeployedProductService) CreateDeployedProduct(ctx context.Context, req domain.CreateDeployedProductRequest) (domain.CreateDeployedProductResponse, error) {
	if err := validateUUIDs("projectId", []string{req.ProjectID}); err != nil {
		return domain.CreateDeployedProductResponse{}, err
	}
	if err := validateUUIDs("deploymentId", []string{req.DeploymentID}); err != nil {
		return domain.CreateDeployedProductResponse{}, err
	}
	if err := validateUUIDs("productId", []string{req.ProductID}); err != nil {
		return domain.CreateDeployedProductResponse{}, err
	}
	if err := validateUUIDs("versionId", []string{req.VersionID}); err != nil {
		return domain.CreateDeployedProductResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return domain.CreateDeployedProductResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}

	payload := snCreateDeployedProductPayload{
		ProjectID:    uuidToSysid(req.ProjectID),
		DeploymentID: uuidToSysid(req.DeploymentID),
		ProductID:    uuidToSysid(req.ProductID),
		VersionID:    uuidToSysid(req.VersionID),
		Cores:        req.Cores,
		TPS:          req.TPS,
		Description:  req.Description,
	}

	raw, err := s.client.Post(ctx, "/deployed-products", token, payload)
	if err != nil {
		return domain.CreateDeployedProductResponse{}, err
	}

	var snResp snCreateDeployedProductResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.CreateDeployedProductResponse{}, fmt.Errorf("sn create deployed product: parse response: %w", err)
	}

	createdOn, err := time.Parse(snCreatedOnLayout, snResp.DeployedProduct.CreatedOn)
	if err != nil {
		return domain.CreateDeployedProductResponse{}, fmt.Errorf("sn create deployed product: parse createdOn %q: %w", snResp.DeployedProduct.CreatedOn, err)
	}

	return domain.CreateDeployedProductResponse{
		Message: snResp.Message,
		DeployedProduct: domain.CreatedDeployedProduct{
			ID:        sysidToUUID(snResp.DeployedProduct.ID),
			CreatedOn: createdOn,
			CreatedBy: snResp.DeployedProduct.CreatedBy,
		},
	}, nil
}

// UpdateDeployedProduct implements DeployedProductService for the ServiceNow data source.
func (s *snDeployedProductService) UpdateDeployedProduct(ctx context.Context, req domain.UpdateDeployedProductRequest) (domain.UpdateDeployedProductResponse, error) {
	if err := validateUUIDs("id", []string{req.ID}); err != nil {
		return domain.UpdateDeployedProductResponse{}, err
	}
	if req.DeploymentID != nil {
		if err := validateUUIDs("deploymentId", []string{*req.DeploymentID}); err != nil {
			return domain.UpdateDeployedProductResponse{}, err
		}
	}

	hasDetailFields := req.Cores != nil || req.TPS != nil || req.Description != nil
	if !hasDetailFields && req.Active == nil {
		return domain.UpdateDeployedProductResponse{}, &apierror.ValidationError{Msg: "at least one of cores, tps, or description must be provided, or active must be set to false"}
	}
	if req.Active != nil && *req.Active {
		return domain.UpdateDeployedProductResponse{}, &apierror.ValidationError{Msg: "active can only be set to false"}
	}
	if req.Active != nil && hasDetailFields {
		return domain.UpdateDeployedProductResponse{}, &apierror.ValidationError{Msg: "cores, tps, and description must not be provided when deactivating"}
	}

	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return domain.UpdateDeployedProductResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}

	// When deploymentId is provided, verify the product belongs to that deployment
	// before mutating it to prevent cross-deployment modification (IDOR).
	// Choreo caps pagination at 50, so pages are iterated until the product is found
	// or all results are exhausted.
	if req.DeploymentID != nil {
		const scopePageSize = 50
		productSysid := uuidToSysid(req.ID)
		deploymentSysid := uuidToSysid(*req.DeploymentID)
		found := false
		for offset := 0; !found; offset += scopePageSize {
			searchPayload := snDeployedProductSearchPayload{
				Filters:    snDeployedProductFilters{DeploymentIDs: []string{deploymentSysid}},
				Pagination: snProjectPagination{Limit: scopePageSize, Offset: offset},
			}
			raw, err := s.client.Post(ctx, "/deployed-products/search", token, searchPayload)
			if err != nil {
				return domain.UpdateDeployedProductResponse{}, err
			}
			var searchResp snDeployedProductsResponse
			if err := json.Unmarshal(raw, &searchResp); err != nil {
				return domain.UpdateDeployedProductResponse{}, fmt.Errorf("sn update deployed product: parse scope check: %w", err)
			}
			for _, dp := range searchResp.DeployedProducts {
				if dp.ID == productSysid {
					found = true
					break
				}
			}
			if offset+len(searchResp.DeployedProducts) >= searchResp.TotalRecords {
				break
			}
		}
		if !found {
			return domain.UpdateDeployedProductResponse{}, &apierror.NotFoundError{Msg: "deployed product not found for the given deployment"}
		}
	}

	payload := snUpdateDeployedProductPayload{
		Cores:  req.Cores,
		TPS:    req.TPS,
		Active: req.Active,
	}
	if req.Description != nil {
		if *req.Description == nil {
			payload.Description = json.RawMessage("null")
		} else {
			b, err := json.Marshal(**req.Description)
			if err != nil {
				return domain.UpdateDeployedProductResponse{}, fmt.Errorf("sn update deployed product: marshal description: %w", err)
			}
			payload.Description = b
		}
	}

	raw, err := s.client.Patch(ctx, "/deployed-products/"+uuidToSysid(req.ID), token, payload)
	if err != nil {
		return domain.UpdateDeployedProductResponse{}, err
	}

	var snResp snUpdateDeployedProductResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.UpdateDeployedProductResponse{}, fmt.Errorf("sn update deployed product: parse response: %w", err)
	}

	updatedOn, err := time.Parse(snCreatedOnLayout, snResp.DeployedProduct.UpdatedOn)
	if err != nil {
		return domain.UpdateDeployedProductResponse{}, fmt.Errorf("sn update deployed product: parse updatedOn %q: %w", snResp.DeployedProduct.UpdatedOn, err)
	}

	return domain.UpdateDeployedProductResponse{
		Message: snResp.Message,
		DeployedProduct: domain.UpdatedDeployedProduct{
			ID:        sysidToUUID(snResp.DeployedProduct.ID),
			UpdatedOn: updatedOn,
			UpdatedBy: snResp.DeployedProduct.UpdatedBy,
		},
	}, nil
}

// SearchDeployedProducts implements DeployedProductService.
func (s *snDeployedProductService) SearchDeployedProducts(ctx context.Context, req domain.SearchDeployedProductsRequest) (domain.SearchDeployedProductsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchDeployedProductsResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)
	if token == "" {
		return domain.SearchDeployedProductsResponse{}, &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}
	}

	payload := snDeployedProductSearchPayload{
		Filters:    snDeployedProductFilters{DeploymentIDs: uuidsToSysids(req.DeploymentIDs)},
		Pagination: snProjectPagination{Limit: req.Pagination.Limit, Offset: req.Pagination.Offset},
	}
	raw, err := s.client.Post(ctx, "/deployed-products/search", token, payload)
	if err != nil {
		return domain.SearchDeployedProductsResponse{}, err
	}

	var snResp snDeployedProductsResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.SearchDeployedProductsResponse{}, fmt.Errorf("sn deployed products: parse response: %w", err)
	}

	views := make([]domain.DeployedProductView, 0, len(snResp.DeployedProducts))
	for _, dp := range snResp.DeployedProducts {
		createdOn, err := time.Parse(snCreatedOnLayout, dp.CreatedOn)
		if err != nil {
			return domain.SearchDeployedProductsResponse{}, fmt.Errorf("sn deployed products: parse createdOn %q: %w", dp.CreatedOn, err)
		}
		updatedOn, err := time.Parse(snCreatedOnLayout, dp.UpdatedOn)
		if err != nil {
			return domain.SearchDeployedProductsResponse{}, fmt.Errorf("sn deployed products: parse updatedOn %q: %w", dp.UpdatedOn, err)
		}

		var versionRef *domain.DeployedProductVersionRef
		if dp.Version != nil {
			var releasedDate, eolDate *time.Time
			if dp.Version.ReleasedDate != nil {
				t, err := time.Parse(time.DateOnly, *dp.Version.ReleasedDate)
				if err != nil {
					return domain.SearchDeployedProductsResponse{}, fmt.Errorf("sn deployed products: parse version releasedOn %q: %w", *dp.Version.ReleasedDate, err)
				}
				releasedDate = &t
			}
			if dp.Version.SupportEoLDate != nil {
				t, err := time.Parse(time.DateOnly, *dp.Version.SupportEoLDate)
				if err != nil {
					return domain.SearchDeployedProductsResponse{}, fmt.Errorf("sn deployed products: parse version endOfLifeOn %q: %w", *dp.Version.SupportEoLDate, err)
				}
				eolDate = &t
			}
			versionRef = &domain.DeployedProductVersionRef{
				ID:             sysidToUUID(dp.Version.ID),
				Name:           dp.Version.Name,
				ReleasedDate:   releasedDate,
				SupportEoLDate: eolDate,
			}
		}

		var cores, tps *string
		if dp.Cores != nil {
			s := fmt.Sprintf("%d", *dp.Cores)
			cores = &s
		}
		if dp.TPS != nil {
			s := fmt.Sprintf("%g", *dp.TPS)
			tps = &s
		}

		views = append(views, domain.DeployedProductView{
			ID:         sysidToUUID(dp.ID),
			Deployment: domain.EntityRef{ID: sysidToUUID(dp.Deployment.ID), Name: dp.Deployment.Name},
			Product:    domain.EntityRef{ID: sysidToUUID(dp.Product.ID), Name: dp.Product.Name},
			Version:    versionRef,
			Cores:      cores,
			TPS:        tps,
			Category:   dp.Category,
			CreatedOn:  createdOn,
			UpdatedOn:  updatedOn,
		})
	}

	total := snResp.TotalRecords
	return domain.SearchDeployedProductsResponse{
		DeployedProducts: views,
		Total:            total,
		Limit:            req.Pagination.Limit,
		Offset:           req.Pagination.Offset,
		HasMore:          req.Pagination.Offset+len(views) < total,
	}, nil
}
