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
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// ProductCatalogHandler serves the product catalogue routes of the products
// API (Postgres data source only): reads for internal callers (the CSM Portal
// and integrations/legacy-product-api) and writes for catalogue managers.
// POST /products/search and POST /products/{id}/versions/search stay on
// ProductHandler and ProductVersionHandler.
type ProductCatalogHandler struct {
	svc service.ProductCatalogService
}

// NewProductCatalogHandler constructs a ProductCatalogHandler.
func NewProductCatalogHandler(svc service.ProductCatalogService) *ProductCatalogHandler {
	return &ProductCatalogHandler{svc: svc}
}

func writeCatalogJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// respond writes v with status, or the service error.
func respond[T any](w http.ResponseWriter, r *http.Request, status int, v T, err error) {
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeCatalogJSON(w, status, v)
}

// queryBool reads an optional true/false query parameter.
func queryBool(r *http.Request, name string) (bool, error) {
	v := r.URL.Query().Get(name)
	if v == "" {
		return false, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, &apierror.ValidationError{Msg: name + " must be true or false"}
	}
	return b, nil
}

// GetProduct handles GET /products/{id}.
func (h *ProductCatalogHandler) GetProduct(w http.ResponseWriter, r *http.Request) {
	p, err := h.svc.GetProduct(r.Context(), r.PathValue("id"))
	respond(w, r, http.StatusOK, p, err)
}

// CreateProduct handles POST /products.
func (h *ProductCatalogHandler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	var body json.RawMessage
	if !decodeRequest(w, r, &body) {
		return
	}
	p, err := h.svc.CreateProduct(r.Context(), body)
	respond(w, r, http.StatusCreated, p, err)
}

// UpdateProduct handles PATCH /products/{id}.
func (h *ProductCatalogHandler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	var body json.RawMessage
	if !decodeRequest(w, r, &body) {
		return
	}
	p, err := h.svc.UpdateProduct(r.Context(), r.PathValue("id"), body)
	respond(w, r, http.StatusOK, p, err)
}

// SearchAllProductVersions handles POST /product-versions/search.
func (h *ProductCatalogHandler) SearchAllProductVersions(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchAllProductVersionsRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.SearchAllProductVersions(r.Context(), req)
	respond(w, r, http.StatusOK, resp, err)
}

// GetProductVersion handles GET /product-versions/{id}.
func (h *ProductCatalogHandler) GetProductVersion(w http.ResponseWriter, r *http.Request) {
	v, err := h.svc.GetProductVersion(r.Context(), r.PathValue("id"))
	respond(w, r, http.StatusOK, v, err)
}

// CreateProductVersions handles POST /products/{id}/versions.
func (h *ProductCatalogHandler) CreateProductVersions(w http.ResponseWriter, r *http.Request) {
	var body json.RawMessage
	if !decodeRequest(w, r, &body) {
		return
	}
	resp, err := h.svc.CreateProductVersions(r.Context(), r.PathValue("id"), body)
	respond(w, r, http.StatusCreated, resp, err)
}

// UpdateProductVersion handles PATCH /product-versions/{id}.
func (h *ProductCatalogHandler) UpdateProductVersion(w http.ResponseWriter, r *http.Request) {
	var body json.RawMessage
	if !decodeRequest(w, r, &body) {
		return
	}
	resp, err := h.svc.UpdateProductVersion(r.Context(), r.PathValue("id"), body)
	respond(w, r, http.StatusOK, resp, err)
}

// RetireProductVersion handles POST /product-versions/{id}/retire.
func (h *ProductCatalogHandler) RetireProductVersion(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.RetireProductVersion(r.Context(), r.PathValue("id"))
	respond(w, r, http.StatusOK, resp, err)
}

// ListReleaseLines handles GET /release-lines?includeRetired=.
func (h *ProductCatalogHandler) ListReleaseLines(w http.ResponseWriter, r *http.Request) {
	includeRetired, err := queryBool(r, "includeRetired")
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	resp, err := h.svc.ListReleaseLines(r.Context(), includeRetired)
	respond(w, r, http.StatusOK, resp, err)
}

// CreateReleaseLine handles POST /release-lines.
func (h *ProductCatalogHandler) CreateReleaseLine(w http.ResponseWriter, r *http.Request) {
	var body json.RawMessage
	if !decodeRequest(w, r, &body) {
		return
	}
	l, err := h.svc.CreateReleaseLine(r.Context(), body)
	respond(w, r, http.StatusCreated, l, err)
}

// UpdateReleaseLine handles PATCH /release-lines/{id}.
func (h *ProductCatalogHandler) UpdateReleaseLine(w http.ResponseWriter, r *http.Request) {
	var body json.RawMessage
	if !decodeRequest(w, r, &body) {
		return
	}
	l, err := h.svc.UpdateReleaseLine(r.Context(), r.PathValue("id"), body)
	respond(w, r, http.StatusOK, l, err)
}

// RetireReleaseLine handles POST /release-lines/{id}/retire.
func (h *ProductCatalogHandler) RetireReleaseLine(w http.ResponseWriter, r *http.Request) {
	l, err := h.svc.RetireReleaseLine(r.Context(), r.PathValue("id"))
	respond(w, r, http.StatusOK, l, err)
}

// ListCarbonPlatformHistory handles GET /carbon-platform-history.
func (h *ProductCatalogHandler) ListCarbonPlatformHistory(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.ListCarbonPlatformHistory(r.Context())
	respond(w, r, http.StatusOK, resp, err)
}

// ListCarbonKernelReleases handles GET /carbon-kernel-releases.
func (h *ProductCatalogHandler) ListCarbonKernelReleases(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.ListCarbonKernelReleases(r.Context())
	respond(w, r, http.StatusOK, resp, err)
}
