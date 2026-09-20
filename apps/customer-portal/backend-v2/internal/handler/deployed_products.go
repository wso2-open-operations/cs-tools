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

package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/dto"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/middleware"
)

// entityDeployedProductClient abstracts the entity-service deployed-product
// operations used by DeployedProductHandler.
type entityDeployedProductClient interface {
	SearchDeployedProducts(ctx context.Context, req entity.SearchDeployedProductsRequest) (entity.SearchDeployedProductsResponse, error)
	CreateDeployedProduct(ctx context.Context, req entity.CreateDeployedProductRequest) (entity.CreateDeployedProductResponse, error)
	UpdateDeployedProduct(ctx context.Context, id string, req entity.UpdateDeployedProductRequest) (entity.UpdateDeployedProductResponse, error)
	SearchDeployedProductMetrics(ctx context.Context, id string, req entity.DeployedProductMetricsRequest) (entity.DeployedProductMetricsResponse, error)
	SearchDeployedProductUsageCounts(ctx context.Context, id string, req entity.DeployedProductUsageCountsRequest) (entity.DeployedProductUsageCountsResponse, error)
}

// DeployedProductHandler handles HTTP requests for deployed-product operations.
type DeployedProductHandler struct {
	entity entityDeployedProductClient
}

// NewDeployedProductHandler creates a DeployedProductHandler backed by the given entity client.
func NewDeployedProductHandler(entity entityDeployedProductClient) *DeployedProductHandler {
	return &DeployedProductHandler{entity: entity}
}

// SearchDeployedProducts handles
// POST /deployments/{deploymentId}/products/search.
func (h *DeployedProductHandler) SearchDeployedProducts(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	deploymentID := r.PathValue("deploymentId")
	if deploymentID == "" || !isUUIDOrSysID(deploymentID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req dto.DeployedProductSearchRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	result, err := h.entity.SearchDeployedProducts(r.Context(), dto.BuildEntitySearchDeployedProductsRequest(deploymentID, req))
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchDeployedProducts failed", "userID", user.UserID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to search deployed products.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapSearchDeployedProducts(result))
}

// CreateDeployedProduct handles POST /deployments/{deploymentId}/products.
//
// NOTE: entity-service only supports this route on its ServiceNow data
// source — see internal/entity/deployed_products.go.
func (h *DeployedProductHandler) CreateDeployedProduct(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	deploymentID := r.PathValue("deploymentId")
	if deploymentID == "" || !isUUIDOrSysID(deploymentID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req dto.DeployedProductCreateRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	if !isUUIDOrSysID(req.ProductID) || !isUUIDOrSysID(req.VersionID) || !isUUIDOrSysID(req.ProjectID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	result, err := h.entity.CreateDeployedProduct(r.Context(), dto.BuildEntityCreateDeployedProductRequest(deploymentID, req))
	if err != nil {
		slog.ErrorContext(r.Context(), "entity CreateDeployedProduct failed", "userID", user.UserID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to create deployed product.")
		return
	}

	writeJSONValue(w, http.StatusCreated, dto.MapDeployedProductCreate(result))
}

// PatchDeployedProduct handles
// PATCH /deployments/{deploymentId}/products/{id}. DeploymentID is always
// injected from the path (entity-service documents it as an IDOR-style
// scope guard) — the frontend's own request body never carries it, so the
// path is the only reliable source.
//
// NOTE: entity-service only supports this route on its ServiceNow data
// source — see internal/entity/deployed_products.go.
func (h *DeployedProductHandler) PatchDeployedProduct(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	deploymentID := r.PathValue("deploymentId")
	id := r.PathValue("id")
	if deploymentID == "" || !isUUIDOrSysID(deploymentID) || id == "" || !isUUIDOrSysID(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var portalReq dto.DeployedProductUpdateRequest
	if err := json.Unmarshal(body, &portalReq); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	req := dto.BuildEntityUpdateDeployedProductRequest(id, deploymentID, portalReq)
	// entity-service requires exactly one of the detail-fields group
	// (cores/tps/description/updates) or active=false — never both, never
	// neither. Mirrors its own hasDetailFields check in
	// snDeployedProductService.UpdateDeployedProduct; keep the two in step.
	//
	// updates counts as a detail field there, and omitting it here rejected the
	// Update History tab's save outright: that dialog sends updates on its own,
	// which satisfied neither branch and came back as "provide either ... or
	// active".
	detailFieldsSet := req.Cores != nil || req.TPS != nil || len(req.Description) > 0 || req.Updates != nil
	activeSet := req.Active != nil
	if detailFieldsSet == activeSet {
		writeError(w, http.StatusBadRequest, "Provide either cores/tps/description/updates or active, but not both.")
		return
	}

	result, err := h.entity.UpdateDeployedProduct(r.Context(), toDashedID(id), req)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity UpdateDeployedProduct failed", "userID", user.UserID, "deployedProductID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to update deployed product.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapDeployedProductUpdate(result))
}

// validateDeployedProductDateRange applies this endpoint's own
// isInvalidDateRange/isWithinOneYear checks, writing a 400 and returning
// ok=false on failure.
func validateDeployedProductDateRange(w http.ResponseWriter, startDate, endDate string) (ok bool) {
	if dto.IsInvalidDateRange(startDate, endDate) {
		writeError(w, http.StatusBadRequest, "Invalid date range: startDate must not be after endDate.")
		return false
	}
	withinOneYear, parsed := dto.IsWithinOneYear(startDate, endDate)
	if !parsed {
		writeError(w, http.StatusBadRequest, "Invalid date format. Expected YYYY-MM-DD.")
		return false
	}
	if !withinOneYear {
		writeError(w, http.StatusBadRequest, "Invalid date range: the range between startDate and endDate must not exceed 1 year.")
		return false
	}
	return true
}

// SearchDeployedProductMetrics handles
// POST /deployments/{deploymentId}/products/{productId}/metrics/search.
// productId here is the deployed-product's own ID (a deliberate
// path-naming quirk of this route); deploymentId is injected into the
// entity request server-side.
func (h *DeployedProductHandler) SearchDeployedProductMetrics(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	deploymentID := r.PathValue("deploymentId")
	productID := r.PathValue("productId")
	if !isUUIDOrSysID(deploymentID) || !isUUIDOrSysID(productID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req dto.DateRangePayload
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	if !validateDeployedProductDateRange(w, req.StartDate, req.EndDate) {
		return
	}

	entityReq := entity.DeployedProductMetricsRequest{DeploymentID: toDashedID(deploymentID), StartDate: req.StartDate, EndDate: req.EndDate}
	result, err := h.entity.SearchDeployedProductMetrics(r.Context(), toDashedID(productID), entityReq)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchDeployedProductMetrics failed", "userID", user.UserID, "deployedProductID", productID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve metrics for the deployed product.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapDeployedProductMetrics(result))
}

// SearchDeployedProductUsageCounts handles
// POST /deployments/{deploymentId}/products/{productId}/metrics/usage-counts/search.
func (h *DeployedProductHandler) SearchDeployedProductUsageCounts(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	deploymentID := r.PathValue("deploymentId")
	productID := r.PathValue("productId")
	if !isUUIDOrSysID(deploymentID) || !isUUIDOrSysID(productID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req dto.DateRangePayload
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	if !validateDeployedProductDateRange(w, req.StartDate, req.EndDate) {
		return
	}

	entityReq := entity.DeployedProductUsageCountsRequest{DeploymentID: toDashedID(deploymentID), StartDate: req.StartDate, EndDate: req.EndDate}
	result, err := h.entity.SearchDeployedProductUsageCounts(r.Context(), toDashedID(productID), entityReq)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchDeployedProductUsageCounts failed", "userID", user.UserID, "deployedProductID", productID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve metrics usage counts for the deployed product.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapDeployedProductUsageCounts(result))
}
