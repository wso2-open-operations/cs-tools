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

// Package handler is declared in user_handler.go.
package handler

import (
	"encoding/json"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// DeployedProductHandler handles HTTP requests for the deployed-product resource.
type DeployedProductHandler struct {
	svc service.DeployedProductService
}

// NewDeployedProductHandler constructs a DeployedProductHandler with the given service.
func NewDeployedProductHandler(svc service.DeployedProductService) *DeployedProductHandler {
	return &DeployedProductHandler{svc: svc}
}

// SearchDeployedProducts handles POST /deployed-products/search.
func (h *DeployedProductHandler) SearchDeployedProducts(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchDeployedProductsRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.SearchDeployedProducts(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// CreateDeployedProduct handles POST /deployed-products.
func (h *DeployedProductHandler) CreateDeployedProduct(w http.ResponseWriter, r *http.Request) {
	var req domain.CreateDeployedProductRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	result, err := h.svc.CreateDeployedProduct(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(result)
}

// PatchDeployedProduct handles PATCH /deployed-products/{id}.
func (h *DeployedProductHandler) PatchDeployedProduct(w http.ResponseWriter, r *http.Request) {
	var req domain.UpdateDeployedProductRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	req.ID = r.PathValue("id")
	result, err := h.svc.UpdateDeployedProduct(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}
