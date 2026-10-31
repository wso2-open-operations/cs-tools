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
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// SLAStatusHandler handles HTTP requests for SLA status — see
// domain.SLAStatus's doc comment for what it's for.
type SLAStatusHandler struct {
	svc service.SLAStatusService
}

// NewSLAStatusHandler constructs an SLAStatusHandler with the given service.
func NewSLAStatusHandler(svc service.SLAStatusService) *SLAStatusHandler {
	return &SLAStatusHandler{svc: svc}
}

// SearchActiveSLAStatuses handles GET /sla-status. A plain GET with
// limit/offset/source/admin query params, not a POST /search: filtering is a
// handful of optional values, so a request body would carry nothing a query
// string can't — same reasoning as the deprecated GET /tags form (see
// CaseHandler.SearchTagsQuery's own doc comment) minus that one's
// backward-compatibility concern, since this is a new endpoint. source is
// optional; see SLAStatusService.SearchActiveSLAStatuses's own doc comment
// for its accepted values. admin=true is the CSM portal's own "Recalculate
// SLA" admin page requesting the CSM-admin-gated view (same query, same
// response shape, just an extra role check) — any other value, or the
// param's absence, is the plain internal-caller request every other caller
// (csm-notification-service's own startup reconciliation included) already
// makes, completely unaffected by this parameter's existence.
func (h *SLAStatusHandler) SearchActiveSLAStatuses(w http.ResponseWriter, r *http.Request) {
	pagination, ok := parseSLAStatusPaginationQuery(w, r)
	if !ok {
		return
	}
	admin := strings.EqualFold(r.URL.Query().Get("admin"), "true")
	resp, err := h.svc.SearchActiveSLAStatuses(r.Context(), pagination, r.URL.Query().Get("source"), admin)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// RequestRecalculation handles POST /sla-engine/recalculate — no request
// body. Returns 204 once the request has been published; the actual Redis
// rebuild happens asynchronously in csm-notification-service, so a 204 here
// means "requested", not "done" — see
// SLAStatusService.RequestRecalculation's own doc comment.
func (h *SLAStatusHandler) RequestRecalculation(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.RequestRecalculation(r.Context()); err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// parseSLAStatusPaginationQuery parses the limit/offset query params
// SearchActiveSLAStatuses accepts. Writes the error response itself and
// returns ok=false on a malformed value.
func parseSLAStatusPaginationQuery(w http.ResponseWriter, r *http.Request) (domain.Pagination, bool) {
	pagination := domain.Pagination{}
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			writeServiceError(w, r, &apierror.ValidationError{Msg: "limit must be a non-negative integer"})
			return domain.Pagination{}, false
		}
		pagination.Limit = parsed
	}
	if raw := r.URL.Query().Get("offset"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			writeServiceError(w, r, &apierror.ValidationError{Msg: "offset must be a non-negative integer"})
			return domain.Pagination{}, false
		}
		pagination.Offset = parsed
	}
	return pagination, true
}

// GetClockState handles GET /sla-status/clock-state. A plain GET with
// workItemId/target/source query params — workItemId and target are
// required, source is optional; see SLAStatusService.GetClockState's own
// doc comment for their accepted values.
func (h *SLAStatusHandler) GetClockState(w http.ResponseWriter, r *http.Request) {
	workItemID := r.URL.Query().Get("workItemId")
	if workItemID == "" {
		writeServiceError(w, r, &apierror.ValidationError{Msg: "workItemId is required"})
		return
	}
	target := r.URL.Query().Get("target")
	if target == "" {
		writeServiceError(w, r, &apierror.ValidationError{Msg: "target is required"})
		return
	}

	resp, err := h.svc.GetClockState(r.Context(), workItemID, target, r.URL.Query().Get("source"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
