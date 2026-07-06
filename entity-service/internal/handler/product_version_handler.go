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

// ProductVersionHandler handles HTTP requests for the product version resource.
type ProductVersionHandler struct {
	svc service.ProductVersionService
}

// NewProductVersionHandler constructs a ProductVersionHandler with the given service.
func NewProductVersionHandler(svc service.ProductVersionService) *ProductVersionHandler {
	return &ProductVersionHandler{svc: svc}
}

// SearchProductVersions handles POST /products/{id}/versions/search.
func (h *ProductVersionHandler) SearchProductVersions(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchProductVersionsRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	req.ProductID = r.PathValue("id")
	resp, err := h.svc.SearchProductVersions(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// SNProductVersionHandler handles POST /products/{id}/versions/search via ServiceNow.
type SNProductVersionHandler struct {
	svc service.SNProductVersionService
}

// NewSNProductVersionHandler constructs an SNProductVersionHandler with the given service.
func NewSNProductVersionHandler(svc service.SNProductVersionService) *SNProductVersionHandler {
	return &SNProductVersionHandler{svc: svc}
}

// SearchProductVersions handles POST /products/{id}/versions/search for the SN data source.
func (h *SNProductVersionHandler) SearchProductVersions(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchProductVersionsRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	req.ProductID = r.PathValue("id")
	resp, err := h.svc.SearchProductVersions(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
