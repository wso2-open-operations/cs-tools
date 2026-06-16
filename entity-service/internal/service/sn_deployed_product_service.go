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
	TPS        *int                      `json:"tps"`
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
			s := fmt.Sprintf("%d", *dp.TPS)
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
