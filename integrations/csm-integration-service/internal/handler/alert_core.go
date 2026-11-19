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
	"log/slog"
	"net/http"
	"net/url"
	"strings"
)

// alertCoreClient abstracts sre-alert-core-service's read API used by
// AlertCoreHandler (internal/alertcore).
type alertCoreClient interface {
	MutedIncidents(ctx context.Context, ruleID string, query url.Values) ([]byte, error)
	MutedCounts(ctx context.Context, query url.Values) ([]byte, error)
	FieldValues(ctx context.Context, query url.Values) ([]byte, error)
}

const (
	errMsgMutedAlertsNotConfigured = "Listing muted alerts is not configured."
	errMsgFieldValuesNotConfigured = "Alert value suggestions are not configured."
	errMsgIDsRequired              = "The 'ids' query parameter is required."
	errMsgTooManyIDs               = "At most 100 ids."
	// maxMutedCountIDs matches alert-core's own cap.
	maxMutedCountIDs = 100
)

// AlertCoreHandler exposes sre-alert-core-service's read API to M2M callers (the
// CSM portal): which incidents each alert mute rule muted, and the values real
// alerts carry in a field. Alert-core answers only with its shared token, which
// stays in this service. Responses are passed through unchanged.
type AlertCoreHandler struct {
	core alertCoreClient
}

// NewAlertCoreHandler creates an AlertCoreHandler. A nil client (ALERT_CORE_BASE_URL
// unset or not https) makes every route answer 503.
func NewAlertCoreHandler(core alertCoreClient) *AlertCoreHandler {
	return &AlertCoreHandler{core: core}
}

// MutedIncidents handles GET /alert-mute-rules/{id}/muted-incidents?limit=&offset=.
func (h *AlertCoreHandler) MutedIncidents(w http.ResponseWriter, r *http.Request) {
	if h.core == nil {
		writeError(w, http.StatusServiceUnavailable, errMsgMutedAlertsNotConfigured)
		return
	}
	// alert-core stores rule ids lower-case and compares them as text.
	id := strings.ToLower(r.PathValue("id"))
	if !uuidRe.MatchString(id) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}
	result, err := h.core.MutedIncidents(r.Context(), id, pick(r.URL.Query(), "limit", "offset"))
	h.respond(w, r, result, err, "MutedIncidents", "Failed to list muted alerts.")
}

// MutedCounts handles GET /alert-mute-rules/muted-counts?ids=a,b,c.
func (h *AlertCoreHandler) MutedCounts(w http.ResponseWriter, r *http.Request) {
	if h.core == nil {
		writeError(w, http.StatusServiceUnavailable, errMsgMutedAlertsNotConfigured)
		return
	}
	// alert-core stores rule ids lower-case, compares them as text and keys the
	// counts by the ids sent: lower-case them so a case-only difference never reads 0.
	raw := strings.ToLower(r.URL.Query().Get("ids"))
	if raw == "" {
		writeError(w, http.StatusBadRequest, errMsgIDsRequired)
		return
	}
	ids := strings.Split(raw, ",")
	if len(ids) > maxMutedCountIDs {
		writeError(w, http.StatusBadRequest, errMsgTooManyIDs)
		return
	}
	for _, id := range ids {
		if !uuidRe.MatchString(id) {
			writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
			return
		}
	}
	result, err := h.core.MutedCounts(r.Context(), url.Values{"ids": {raw}})
	h.respond(w, r, result, err, "MutedCounts", "Failed to count muted alerts.")
}

// FieldValues handles GET /alert-field-values?field=&q=&limit=.
func (h *AlertCoreHandler) FieldValues(w http.ResponseWriter, r *http.Request) {
	if h.core == nil {
		writeError(w, http.StatusServiceUnavailable, errMsgFieldValuesNotConfigured)
		return
	}
	result, err := h.core.FieldValues(r.Context(), pick(r.URL.Query(), "field", "q", "limit"))
	h.respond(w, r, result, err, "FieldValues", "Failed to list alert field values.")
}

func (h *AlertCoreHandler) respond(w http.ResponseWriter, r *http.Request, result []byte, err error, op, fallback string) {
	if err != nil {
		slog.ErrorContext(r.Context(), "alert-core "+op+" failed", "err", summarizeErr(err))
		mapUpstreamError(w, err, fallback)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// pick copies only the named query parameters (their first value), so nothing
// else a caller sends reaches alert-core.
func pick(q url.Values, names ...string) url.Values {
	out := url.Values{}
	for _, n := range names {
		if v, ok := q[n]; ok && len(v) > 0 {
			out.Set(n, v[0])
		}
	}
	return out
}
