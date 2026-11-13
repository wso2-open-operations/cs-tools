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
	"net/http"
	"strconv"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// StandardChangeSuggestionHandler serves the standard change suggestions and
// their dismissals.
type StandardChangeSuggestionHandler struct {
	svc service.StandardChangeSuggestionService
}

// NewStandardChangeSuggestionHandler constructs a StandardChangeSuggestionHandler.
func NewStandardChangeSuggestionHandler(svc service.StandardChangeSuggestionService) *StandardChangeSuggestionHandler {
	return &StandardChangeSuggestionHandler{svc: svc}
}

// parseSuggestionsQuery reads GET /standard-change-suggestions' query. A flag
// is true only for "true" (any case). The criteria are configuration, applied
// when the snapshot is computed, not parameters of a read. limit and offset must
// be integers when given (a ValidationError otherwise); their default and
// bounds are the service's.
func parseSuggestionsQuery(r *http.Request) (domain.StandardChangeSuggestionsRequest, error) {
	q := r.URL.Query()
	req := domain.StandardChangeSuggestionsRequest{
		AccountID:         q.Get("accountId"),
		IncludeIneligible: strings.EqualFold(strings.TrimSpace(q.Get("includeIneligible")), "true"),
		IncludeDismissed:  strings.EqualFold(strings.TrimSpace(q.Get("includeDismissed")), "true"),
	}
	for _, p := range []struct {
		name string
		to   *int
	}{{"limit", &req.Pagination.Limit}, {"offset", &req.Pagination.Offset}} {
		raw := strings.TrimSpace(q.Get(p.name))
		if raw == "" {
			continue
		}
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			return req, &apierror.ValidationError{Msg: p.name + " must be a non-negative integer"}
		}
		*p.to = n
	}
	return req, nil
}

// GetSuggestions handles GET /standard-change-suggestions.
func (h *StandardChangeSuggestionHandler) GetSuggestions(w http.ResponseWriter, r *http.Request) {
	req, err := parseSuggestionsQuery(r)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	resp, err := h.svc.GetSuggestions(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeStandardChangeJSON(w, http.StatusOK, resp)
}

// Refresh handles POST /standard-change-suggestions/refresh: one page of a
// snapshot run, for the scheduled refresh (a service credential, see routes.go).
func (h *StandardChangeSuggestionHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req domain.RefreshStandardChangeSuggestionsRequest
	if !decodeOptionalRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.Refresh(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeStandardChangeJSON(w, http.StatusOK, resp)
}

// DismissSuggestion handles POST /standard-change-suggestions/dismissals.
func (h *StandardChangeSuggestionHandler) DismissSuggestion(w http.ResponseWriter, r *http.Request) {
	var req domain.DismissStandardChangeSuggestionRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	d, err := h.svc.DismissSuggestion(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeStandardChangeJSON(w, http.StatusCreated, d)
}

// RestoreSuggestion handles DELETE /standard-change-suggestions/dismissals/{id}.
func (h *StandardChangeSuggestionHandler) RestoreSuggestion(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.RestoreSuggestion(r.Context(), r.PathValue("id")); err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
