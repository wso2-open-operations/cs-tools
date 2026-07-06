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

// ProductHandler handles HTTP requests for the product resource.
type ProductHandler struct {
	svc service.ProductService
}

// NewProductHandler constructs a ProductHandler with the given service.
func NewProductHandler(svc service.ProductService) *ProductHandler {
	return &ProductHandler{svc: svc}
}

// SearchProducts handles POST /products/search.
func (h *ProductHandler) SearchProducts(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchProductsRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.SearchProducts(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// SNProductHandler handles HTTP requests for product operations backed by ServiceNow.
type SNProductHandler struct {
	svc service.SNProductService
}

// NewSNProductHandler constructs an SNProductHandler with the given service.
func NewSNProductHandler(svc service.SNProductService) *SNProductHandler {
	return &SNProductHandler{svc: svc}
}

// SearchProducts handles POST /products/search for the ServiceNow data source.
func (h *SNProductHandler) SearchProducts(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchProductsRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.SearchProducts(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
