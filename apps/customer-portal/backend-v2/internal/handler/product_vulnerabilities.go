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
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/dto"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/middleware"
)

// vulnerabilityCveIDMaxLength is the longest cveId filter accepted, matching
// entity-service's own limit.
const vulnerabilityCveIDMaxLength = 100

// entityProductVulnerabilityClient abstracts the entity-service product
// vulnerability operations used by ProductVulnerabilityHandler.
type entityProductVulnerabilityClient interface {
	SearchProductVulnerabilities(ctx context.Context, req entity.SearchProductVulnerabilitiesRequest) (entity.SearchProductVulnerabilitiesResponse, error)
	GetProductVulnerability(ctx context.Context, id string) (entity.ProductVulnerabilityView, error)
}

// ProductVulnerabilityHandler handles HTTP requests for product vulnerability operations.
type ProductVulnerabilityHandler struct {
	entity entityProductVulnerabilityClient
	// bulkCache holds fully-aggregated result sets for "fetch all" requests, which
	// have to be assembled from many upstream pages. See fetchAllVulnerabilities.
	bulkCache *vulnerabilityBulkCache
}

// NewProductVulnerabilityHandler creates a ProductVulnerabilityHandler backed by the given entity client.
func NewProductVulnerabilityHandler(entity entityProductVulnerabilityClient) *ProductVulnerabilityHandler {
	return &ProductVulnerabilityHandler{entity: entity, bulkCache: newVulnerabilityBulkCache()}
}

// SearchProductVulnerabilities handles POST /products/vulnerabilities/search.
func (h *ProductVulnerabilityHandler) SearchProductVulnerabilities(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}

	var req dto.SearchProductVulnerabilitiesRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	// Same cap as entity-service's cveId column; rejecting here spares the
	// round trip and keeps the 400 in this API's own wording.
	if req.Filters != nil && utf8.RuneCountInString(strings.TrimSpace(req.Filters.CveID)) > vulnerabilityCveIDMaxLength {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}

	// The frontend's security page asks for the whole advisory set in one call
	// (PRODUCT_VULNERABILITIES_ALL_FETCH_LIMIT = 5000), but entity-service rejects
	// any limit above 50 because its own backing data source does. Forwarding that
	// verbatim produced "limit cannot exceed 50" and an empty page. Requests above
	// the cap are therefore assembled from batched upstream calls here, the same
	// way the Ballerina backend does it.
	if req.Pagination.Limit > vulnerabilityUpstreamMaxLimit {
		h.searchAllProductVulnerabilities(w, r, user.UserID, req)
		return
	}

	result, err := h.entity.SearchProductVulnerabilities(r.Context(), dto.BuildEntitySearchProductVulnerabilitiesRequest(req))
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchProductVulnerabilities failed", "userID", user.UserID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to search product vulnerabilities.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapSearchProductVulnerabilities(result))
}

// GetProductVulnerability handles GET /products/vulnerabilities/{id}.
func (h *ProductVulnerabilityHandler) GetProductVulnerability(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	id := r.PathValue("id")
	if id == "" || !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	result, err := h.entity.GetProductVulnerability(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetProductVulnerability failed", "userID", user.UserID, "vulnerabilityID", id, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to retrieve product vulnerability.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.MapProductVulnerability(result))
}

// searchAllProductVulnerabilities serves a request whose limit exceeds
// entity-service's per-request cap by batching upstream and slicing locally.
//
// The response still echoes the caller's own limit and offset and reports the
// true total, so a client that does paginate keeps working; a client that asked
// for everything gets everything.
func (h *ProductVulnerabilityHandler) searchAllProductVulnerabilities(
	w http.ResponseWriter,
	r *http.Request,
	userID string,
	req dto.SearchProductVulnerabilitiesRequest,
) {
	// Many sequential upstream calls can outlast the server's global WriteTimeout
	// even when each one is fast, so extend this response's deadline the same way
	// the license-provisioning flow does.
	if rc := http.NewResponseController(w); rc != nil {
		if err := rc.SetWriteDeadline(time.Now().Add(vulnerabilityBulkDeadline)); err != nil {
			slog.WarnContext(r.Context(), "could not extend the write deadline for a bulk vulnerability fetch", "userID", userID, "err", summarizeErr(err))
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), vulnerabilityBulkDeadline)
	defer cancel()

	all, err := fetchAllVulnerabilities(ctx, h.entity, h.bulkCache, req, time.Now())
	if err != nil {
		slog.ErrorContext(ctx, "entity SearchProductVulnerabilities failed during a bulk fetch", "userID", userID, "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to search product vulnerabilities.")
		return
	}

	writeJSONValue(w, http.StatusOK, dto.SearchProductVulnerabilitiesResponse{
		ProductVulnerabilities: sliceVulnerabilityPage(all, req.Pagination.Offset, req.Pagination.Limit),
		TotalRecords:           len(all),
		Limit:                  req.Pagination.Limit,
		Offset:                 req.Pagination.Offset,
	})
}
