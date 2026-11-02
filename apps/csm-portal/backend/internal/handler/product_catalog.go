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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// entityProductCatalogClient is the entity-service products API calls the
// Products section (the product catalogue behind the public Support Matrix and
// Release Matrix) needs, beyond the two searches ProductHandler proxies.
type entityProductCatalogClient interface {
	GetProduct(ctx context.Context, id string) ([]byte, error)
	CreateProduct(ctx context.Context, body []byte) ([]byte, error)
	UpdateProduct(ctx context.Context, id string, body []byte) ([]byte, error)
	CreateProductVersions(ctx context.Context, productID string, body []byte) ([]byte, error)
	SearchAllProductVersions(ctx context.Context, body []byte) ([]byte, error)
	GetProductVersion(ctx context.Context, id string) ([]byte, error)
	UpdateProductVersion(ctx context.Context, id string, body []byte) ([]byte, error)
	RetireProductVersion(ctx context.Context, id string) ([]byte, error)
	ListReleaseLines(ctx context.Context, q url.Values) ([]byte, error)
	CreateReleaseLine(ctx context.Context, body []byte) ([]byte, error)
	UpdateReleaseLine(ctx context.Context, id string, body []byte) ([]byte, error)
	RetireReleaseLine(ctx context.Context, id string) ([]byte, error)
	ListCarbonPlatformHistory(ctx context.Context) ([]byte, error)
	ListCarbonKernelReleases(ctx context.Context) ([]byte, error)
}

// ProductCatalogHandler proxies the product catalogue routes of entity-service's
// products API under the same paths. Reads are PermView; writes are
// PermManageProductCatalog here, and entity-service re-checks the same role.
// Bodies are passed on as sent once they are a JSON object: the fields each
// route accepts, and their rules, are entity-service's (its 400 names the bad
// field, and a write surfaces it).
type ProductCatalogHandler struct {
	entity entityProductCatalogClient
}

// NewProductCatalogHandler creates a ProductCatalogHandler.
func NewProductCatalogHandler(entity entityProductCatalogClient) *ProductCatalogHandler {
	return &ProductCatalogHandler{entity: entity}
}

// ----- products -----

// GetProduct handles GET /products/{id}.
func (h *ProductCatalogHandler) GetProduct(w http.ResponseWriter, r *http.Request) {
	id, ok := catalogPathID(w, r)
	if !ok {
		return
	}
	h.read(w, r, "GetProduct", "Failed to load the product.",
		func(ctx context.Context) ([]byte, error) { return h.entity.GetProduct(ctx, id) })
}

// CreateProduct handles POST /products.
func (h *ProductCatalogHandler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	body, ok := catalogObjectBody(w, r)
	if !ok {
		return
	}
	h.write(w, r, "CreateProduct", "Failed to add the product.", http.StatusCreated,
		func(ctx context.Context) ([]byte, error) { return h.entity.CreateProduct(ctx, body) })
}

// UpdateProduct handles PATCH /products/{id}.
func (h *ProductCatalogHandler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	id, ok := catalogPathID(w, r)
	if !ok {
		return
	}
	body, ok := catalogObjectBody(w, r)
	if !ok {
		return
	}
	h.write(w, r, "UpdateProduct", "Failed to save the product.", http.StatusOK,
		func(ctx context.Context) ([]byte, error) { return h.entity.UpdateProduct(ctx, id, body) })
}

// ----- product versions -----

// CreateProductVersions handles POST /products/{id}/versions: a version with
// all its deployment profiles, created together upstream.
func (h *ProductCatalogHandler) CreateProductVersions(w http.ResponseWriter, r *http.Request) {
	id, ok := catalogPathID(w, r)
	if !ok {
		return
	}
	body, ok := catalogObjectBody(w, r)
	if !ok {
		return
	}
	h.write(w, r, "CreateProductVersions", "Failed to add the version.", http.StatusCreated,
		func(ctx context.Context) ([]byte, error) { return h.entity.CreateProductVersions(ctx, id, body) })
}

// SearchAllProductVersions handles POST /product-versions/search.
func (h *ProductCatalogHandler) SearchAllProductVersions(w http.ResponseWriter, r *http.Request) {
	body, ok := catalogObjectBody(w, r)
	if !ok {
		return
	}
	h.read(w, r, "SearchAllProductVersions", "Failed to load product versions.",
		func(ctx context.Context) ([]byte, error) { return h.entity.SearchAllProductVersions(ctx, body) })
}

// GetProductVersion handles GET /product-versions/{id}.
func (h *ProductCatalogHandler) GetProductVersion(w http.ResponseWriter, r *http.Request) {
	id, ok := catalogPathID(w, r)
	if !ok {
		return
	}
	h.read(w, r, "GetProductVersion", "Failed to load the product version.",
		func(ctx context.Context) ([]byte, error) { return h.entity.GetProductVersion(ctx, id) })
}

// UpdateProductVersion handles PATCH /product-versions/{id}.
func (h *ProductCatalogHandler) UpdateProductVersion(w http.ResponseWriter, r *http.Request) {
	id, ok := catalogPathID(w, r)
	if !ok {
		return
	}
	body, ok := catalogObjectBody(w, r)
	if !ok {
		return
	}
	h.write(w, r, "UpdateProductVersion", "Failed to save the version.", http.StatusOK,
		func(ctx context.Context) ([]byte, error) { return h.entity.UpdateProductVersion(ctx, id, body) })
}

// RetireProductVersion handles POST /product-versions/{id}/retire. It takes no
// body.
func (h *ProductCatalogHandler) RetireProductVersion(w http.ResponseWriter, r *http.Request) {
	id, ok := catalogPathID(w, r)
	if !ok {
		return
	}
	h.write(w, r, "RetireProductVersion", "Failed to retire the version.", http.StatusOK,
		func(ctx context.Context) ([]byte, error) { return h.entity.RetireProductVersion(ctx, id) })
}

// ----- release lines -----

// ListReleaseLines handles GET /release-lines?includeRetired=.
func (h *ProductCatalogHandler) ListReleaseLines(w http.ResponseWriter, r *http.Request) {
	if middleware.UserInfoFromContext(r.Context()) == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	q := url.Values{}
	switch values := r.URL.Query()["includeRetired"]; {
	case len(values) == 0 || (len(values) == 1 && values[0] == ""):
	case len(values) == 1 && (values[0] == "true" || values[0] == "false"):
		q.Set("includeRetired", values[0])
	default:
		writeError(w, http.StatusBadRequest, "Invalid value for includeRetired.")
		return
	}
	h.read(w, r, "ListReleaseLines", "Failed to load release lines.",
		func(ctx context.Context) ([]byte, error) { return h.entity.ListReleaseLines(ctx, q) })
}

// CreateReleaseLine handles POST /release-lines.
func (h *ProductCatalogHandler) CreateReleaseLine(w http.ResponseWriter, r *http.Request) {
	body, ok := catalogObjectBody(w, r)
	if !ok {
		return
	}
	h.write(w, r, "CreateReleaseLine", "Failed to add the release line.", http.StatusCreated,
		func(ctx context.Context) ([]byte, error) { return h.entity.CreateReleaseLine(ctx, body) })
}

// UpdateReleaseLine handles PATCH /release-lines/{id}.
func (h *ProductCatalogHandler) UpdateReleaseLine(w http.ResponseWriter, r *http.Request) {
	id, ok := catalogPathID(w, r)
	if !ok {
		return
	}
	body, ok := catalogObjectBody(w, r)
	if !ok {
		return
	}
	h.write(w, r, "UpdateReleaseLine", "Failed to save the release line.", http.StatusOK,
		func(ctx context.Context) ([]byte, error) { return h.entity.UpdateReleaseLine(ctx, id, body) })
}

// RetireReleaseLine handles POST /release-lines/{id}/retire. It takes no body.
func (h *ProductCatalogHandler) RetireReleaseLine(w http.ResponseWriter, r *http.Request) {
	id, ok := catalogPathID(w, r)
	if !ok {
		return
	}
	h.write(w, r, "RetireReleaseLine", "Failed to retire the release line.", http.StatusOK,
		func(ctx context.Context) ([]byte, error) { return h.entity.RetireReleaseLine(ctx, id) })
}

// ----- reference data -----

// ListCarbonPlatformHistory handles GET /carbon-platform-history.
func (h *ProductCatalogHandler) ListCarbonPlatformHistory(w http.ResponseWriter, r *http.Request) {
	h.read(w, r, "ListCarbonPlatformHistory", "Failed to load the Carbon platform history.", h.entity.ListCarbonPlatformHistory)
}

// ListCarbonKernelReleases handles GET /carbon-kernel-releases.
func (h *ProductCatalogHandler) ListCarbonKernelReleases(w http.ResponseWriter, r *http.Request) {
	h.read(w, r, "ListCarbonKernelReleases", "Failed to load Carbon kernel releases.", h.entity.ListCarbonKernelReleases)
}

// ----- helpers -----

// catalogPathID returns the {id} path segment once it is a UUID.
func catalogPathID(w http.ResponseWriter, r *http.Request) (string, bool) {
	if middleware.UserInfoFromContext(r.Context()) == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return "", false
	}
	id := r.PathValue("id")
	if !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return "", false
	}
	return id, true
}

// catalogObjectBody reads a size-capped body that must be one JSON object.
func catalogObjectBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	if middleware.UserInfoFromContext(r.Context()) == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return nil, false
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, ErrMsgTooLarge)
			return nil, false
		}
		writeError(w, http.StatusBadRequest, errMsgReadBody)
		return nil, false
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' || !json.Valid(trimmed) {
		writeError(w, http.StatusBadRequest, "The request body must be a JSON object.")
		return nil, false
	}
	return trimmed, true
}

// read runs one read upstream and writes its result, mapping a failure
// through mapUpstreamErrorGeneric.
func (h *ProductCatalogHandler) read(w http.ResponseWriter, r *http.Request, op, fallback string, call func(context.Context) ([]byte, error)) {
	h.run(w, r, op, http.StatusOK, call, func(err error) { mapUpstreamErrorGeneric(w, err, fallback) })
}

// write runs one write upstream. A refusal (e.g. an EOL date before the
// release date, or a duplicate) names the field or rule, which only the caller
// can fix, so it is surfaced through mapUpstreamError.
func (h *ProductCatalogHandler) write(w http.ResponseWriter, r *http.Request, op, fallback string, status int, call func(context.Context) ([]byte, error)) {
	h.run(w, r, op, status, call, func(err error) { mapUpstreamError(w, err, fallback) })
}

func (h *ProductCatalogHandler) run(w http.ResponseWriter, r *http.Request, op string, status int, call func(context.Context) ([]byte, error), fail func(error)) {
	w.Header().Set("Cache-Control", "no-store")
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	result, err := call(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "entity "+op+" failed", "userID", user.UserID, "err", err)
		fail(err)
		return
	}
	writeJSON(w, status, result)
}
