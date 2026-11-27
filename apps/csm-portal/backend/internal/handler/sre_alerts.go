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
	"regexp"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// entitySreAlertClient abstracts the entity service SRE alert operations used by
// SreAlertHandler. SRE alerts are the rows of entity-service's own alert table,
// which is unrelated to the ServiceNow incident_alert behind AlertHandler.
type entitySreAlertClient interface {
	SearchSreAlerts(ctx context.Context, body []byte) ([]byte, error)
	GetSreAlert(ctx context.Context, id string) ([]byte, error)
	GetSreAlertFacets(ctx context.Context) ([]byte, error)
}

// SreAlertHandler proxies the /sre-alerts read routes 1:1 to entity-service.
type SreAlertHandler struct {
	entity entitySreAlertClient
}

// NewSreAlertHandler creates an SreAlertHandler backed by the given entity client.
func NewSreAlertHandler(entity entitySreAlertClient) *SreAlertHandler {
	return &SreAlertHandler{entity: entity}
}

// sreAlertNumberRe matches an SRE alert's human number, e.g. ALT000000042.
var sreAlertNumberRe = regexp.MustCompile(`^ALT\d{9}$`)

// errMsgInvalidSreAlertID is the 400 for a GET /sre-alerts/{id} whose id is
// neither a UUID nor an ALT number.
const errMsgInvalidSreAlertID = "Invalid alert ID. Use the alert's UUID or its ALT number."

// maxSreAlertSearchLimit is entity-service's own page-size cap for the search.
const maxSreAlertSearchLimit = 100

var (
	validSreAlertTiers          = map[string]bool{"priority": true, "standard": true}
	validSreAlertIncidentFilter = map[string]bool{"all": true, "linked": true, "unlinked": true}
	validSreAlertSortFields     = map[string]bool{
		"receivedOn": true, "number": true, "severity": true, "environment": true, "service": true,
	}
	validSreAlertSortOrders = map[string]bool{"asc": true, "desc": true}
)

// searchSreAlertsRequest mirrors the enum-, time- and range-constrained fields of
// SearchSreAlertsPayload. It is decoded only to validate them; the raw body is
// forwarded unchanged.
type searchSreAlertsRequest struct {
	Filters struct {
		Tiers        []string `json:"tiers"`
		Incident     string   `json:"incident"`
		IncidentID   string   `json:"incidentId"`
		ReceivedFrom string   `json:"receivedFrom"`
		ReceivedTo   string   `json:"receivedTo"`
	} `json:"filters"`
	SortBy struct {
		Field string `json:"field"`
		Order string `json:"order"`
	} `json:"sortBy"`
	Pagination struct {
		Offset *int `json:"offset"`
		Limit  *int `json:"limit"`
	} `json:"pagination"`
}

// validateSearchSreAlertsBody rejects a body that is not a JSON object, or whose
// tier, incident, incidentId, sort, received-date or pagination values could
// never be valid.
func validateSearchSreAlertsBody(body []byte) bool {
	var obj map[string]json.RawMessage
	// A JSON null unmarshals into a nil map without error, so check for nil too.
	if err := json.Unmarshal(body, &obj); err != nil || obj == nil {
		return false
	}
	var req searchSreAlertsRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return false
	}
	for _, t := range req.Filters.Tiers {
		if !validSreAlertTiers[t] {
			return false
		}
	}
	if req.Filters.Incident != "" && !validSreAlertIncidentFilter[req.Filters.Incident] {
		return false
	}
	if req.Filters.IncidentID != "" && !uuidRe.MatchString(req.Filters.IncidentID) {
		return false
	}
	for _, ts := range []string{req.Filters.ReceivedFrom, req.Filters.ReceivedTo} {
		if ts == "" {
			continue
		}
		if _, err := time.Parse(time.RFC3339, ts); err != nil {
			return false
		}
	}
	if req.SortBy.Field != "" && !validSreAlertSortFields[req.SortBy.Field] {
		return false
	}
	if req.SortBy.Order != "" && !validSreAlertSortOrders[req.SortBy.Order] {
		return false
	}
	if p := req.Pagination.Offset; p != nil && *p < 0 {
		return false
	}
	if p := req.Pagination.Limit; p != nil && (*p < 0 || *p > maxSreAlertSearchLimit) {
		return false
	}
	return true
}

// SearchSreAlerts handles POST /sre-alerts/search.
func (h *SreAlertHandler) SearchSreAlerts(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	body, ok := readJSONBody(w, r)
	if !ok {
		return
	}
	if !validateSearchSreAlertsBody(body) {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	result, err := h.entity.SearchSreAlerts(r.Context(), body)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity SearchSreAlerts failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to search alerts.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// GetSreAlert handles GET /sre-alerts/{id}; id is the alert's UUID or its ALT number.
func (h *SreAlertHandler) GetSreAlert(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	id := r.PathValue("id")
	if id == "" || (!uuidRe.MatchString(id) && !sreAlertNumberRe.MatchString(id)) {
		writeError(w, http.StatusBadRequest, errMsgInvalidSreAlertID)
		return
	}
	result, err := h.entity.GetSreAlert(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetSreAlert failed", "userID", user.UserID, "id", id, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve alert.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// GetSreAlertFacets handles GET /sre-alerts/facets.
func (h *SreAlertHandler) GetSreAlertFacets(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	result, err := h.entity.GetSreAlertFacets(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetSreAlertFacets failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve alert filters.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}
