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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// SreAlertHandler serves /sre-alerts: the alerts the alert consumer stored, for the portal's Alerts pages.
type SreAlertHandler struct {
	svc service.SreAlertService
}

// NewSreAlertHandler returns the handler for /sre-alerts.
func NewSreAlertHandler(svc service.SreAlertService) *SreAlertHandler {
	return &SreAlertHandler{svc: svc}
}

// SearchSreAlerts handles POST /sre-alerts/search.
func (h *SreAlertHandler) SearchSreAlerts(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchSreAlertsRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.Search(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeSreAlertJSON(w, resp)
}

// GetSreAlert handles GET /sre-alerts/{id}, where id is the alert's UUID or its ALT number.
func (h *SreAlertHandler) GetSreAlert(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeSreAlertJSON(w, resp)
}

// GetSreAlertFacets handles GET /sre-alerts/facets.
func (h *SreAlertHandler) GetSreAlertFacets(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.Facets(r.Context())
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeSreAlertJSON(w, resp)
}

func writeSreAlertJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(v)
}
