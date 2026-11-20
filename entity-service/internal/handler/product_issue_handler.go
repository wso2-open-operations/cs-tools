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
	"errors"
	"log"
	"math"
	"net/http"
	"strconv"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// ProductIssueHandler serves the read and link routes of product issues.
type ProductIssueHandler struct {
	svc service.ProductIssueService
}

// NewProductIssueHandler constructs a ProductIssueHandler.
func NewProductIssueHandler(svc service.ProductIssueService) *ProductIssueHandler {
	return &ProductIssueHandler{svc: svc}
}

func writeProductIssueJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeProductIssueError(w http.ResponseWriter, r *http.Request, err error) {
	var ufe *service.UpstreamFailureError
	if errors.As(err, &ufe) {
		// The upstream's own words never reach the caller; Retry-After does.
		log.Printf("Upstream failure: %s %s: %d", r.Method, sanitizeLog(r.URL.Path), ufe.Status) // #nosec G706 -- path sanitized
		if ufe.RetryAfter > 0 {
			w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(ufe.RetryAfter.Seconds()))))
		}
		apierror.WriteJSON(w, ufe.Status, ufe.Msg)
		return
	}
	writeServiceError(w, r, err)
}

// ListCaseProductWorkItems handles GET /cases/{id}/product-work-items.
func (h *ProductIssueHandler) ListCaseProductWorkItems(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.ListForCase(r.Context(), r.PathValue("id"))
	if err != nil {
		writeProductIssueError(w, r, err)
		return
	}
	writeProductIssueJSON(w, http.StatusOK, resp)
}

// GetProductWorkItem handles GET /product-work-items/{id}.
func (h *ProductIssueHandler) GetProductWorkItem(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeProductIssueError(w, r, err)
		return
	}
	writeProductIssueJSON(w, http.StatusOK, resp)
}

// SearchProductWorkItems handles POST /product-work-items/search.
func (h *ProductIssueHandler) SearchProductWorkItems(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchProductIssuesRequest
	if !decodeOptionalRequest(w, r, &req) { // an empty body is the unfiltered first page
		return
	}
	resp, err := h.svc.Search(r.Context(), req)
	if err != nil {
		writeProductIssueError(w, r, err)
		return
	}
	writeProductIssueJSON(w, http.StatusOK, resp)
}

// LinkProductWorkItem handles POST /cases/{id}/product-work-items/links.
func (h *ProductIssueHandler) LinkProductWorkItem(w http.ResponseWriter, r *http.Request) {
	var req domain.LinkProductIssueRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	req.CaseID = r.PathValue("id")
	resp, created, err := h.svc.Link(r.Context(), req)
	if err != nil {
		writeProductIssueError(w, r, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeProductIssueJSON(w, status, resp)
}

// UnlinkProductWorkItem handles DELETE /cases/{id}/product-work-items/{workItemId}/link.
func (h *ProductIssueHandler) UnlinkProductWorkItem(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Unlink(r.Context(), r.PathValue("id"), r.PathValue("workItemId")); err != nil {
		writeProductIssueError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
