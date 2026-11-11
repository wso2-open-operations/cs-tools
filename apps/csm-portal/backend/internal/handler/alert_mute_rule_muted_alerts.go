// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

const (
	errMsgMutedAlertsNotConfigured = "Listing muted alerts is not configured."
	errMsgMutedAlertsUnavailable   = "Muted alerts are unavailable right now. Please try again later."
	errMsgFieldValuesNotConfigured = "Alert value suggestions are not configured."
	errMsgFieldValuesUnavailable   = "Alert value suggestions are unavailable right now."
	// maxFieldValuesQuery matches the longest condition value a rule can hold.
	maxFieldValuesQuery = 255
)

// alertMuteRuleFieldValueFields are the alert fields a mute rule condition can name (entity-service's list).
var alertMuteRuleFieldValueFields = map[string]bool{
	"service": true, "environment": true, "source": true, "metric_name": true,
	"severity": true, "category": true, "unique_identifier": true,
}

// alertCoreMutedClient reads from sre-alert-core-service: the incidents an alert mute rule muted, and the values
// alerts carry in a field.
type alertCoreMutedClient interface {
	MutedIncidents(ctx context.Context, ruleID string, limit, offset int) ([]byte, error)
	FieldValues(ctx context.Context, field, query string, limit int) ([]byte, error)
}

// AlertMuteRuleMutedAlertsHandler serves GET /alert-mute-rules/{id}/muted-alerts. The muted alerts live in alert-core's
// database (incidents_processed rows tagged with the rule's id), so this asks alert-core rather than entity-service.
// A nil client means ALERT_CORE_BASE_URL / ALERT_CORE_API_TOKEN are unset; the route then answers 503.
type AlertMuteRuleMutedAlertsHandler struct {
	alertCore alertCoreMutedClient
}

// NewAlertMuteRuleMutedAlertsHandler returns a handler backed by client, which may be nil.
func NewAlertMuteRuleMutedAlertsHandler(client alertCoreMutedClient) *AlertMuteRuleMutedAlertsHandler {
	return &AlertMuteRuleMutedAlertsHandler{alertCore: client}
}

// ListMutedAlerts handles GET /alert-mute-rules/{id}/muted-alerts?limit=&offset=.
func (h *AlertMuteRuleMutedAlertsHandler) ListMutedAlerts(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	id, ok := alertMuteRulePathID(w, r)
	if !ok {
		return
	}
	limit, okLimit := queryInt(r, "limit", 20, 1, 100)
	offset, okOffset := queryInt(r, "offset", 0, 0, 1<<30)
	if !okLimit || !okOffset {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	if h.alertCore == nil {
		writeError(w, http.StatusServiceUnavailable, errMsgMutedAlertsNotConfigured)
		return
	}
	result, err := h.alertCore.MutedIncidents(r.Context(), id, limit, offset)
	if err != nil {
		slog.ErrorContext(r.Context(), "alert-core MutedIncidents failed", "userID", user.UserID, "id", id, "err", err)
		// alert-core's 401 means this backend's token is wrong, not that the engineer is signed out.
		var apiErr *apierror.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusBadRequest {
			writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
			return
		}
		writeError(w, http.StatusServiceUnavailable, errMsgMutedAlertsUnavailable)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// ListAlertFieldValues handles GET /alert-field-values?field=&q=&limit=: the values real alerts carry in one field,
// which the rule form suggests. A rule matches those strings, not entity-service's names (CSM services are not what
// monitoring tools send as an alert's service).
func (h *AlertMuteRuleMutedAlertsHandler) ListAlertFieldValues(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}
	field := r.URL.Query().Get("field")
	query := r.URL.Query().Get("q")
	limit, okLimit := queryInt(r, "limit", 20, 1, 100)
	if !alertMuteRuleFieldValueFields[field] || len(query) > maxFieldValuesQuery || !okLimit {
		writeError(w, http.StatusBadRequest, ErrMsgBadRequest)
		return
	}
	if h.alertCore == nil {
		writeError(w, http.StatusServiceUnavailable, errMsgFieldValuesNotConfigured)
		return
	}
	result, err := h.alertCore.FieldValues(r.Context(), field, query, limit)
	if err != nil {
		slog.ErrorContext(r.Context(), "alert-core FieldValues failed", "userID", user.UserID, "field", field, "err", err)
		writeError(w, http.StatusServiceUnavailable, errMsgFieldValuesUnavailable)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// queryInt reads query parameter name, defaulting to def when absent; ok is false when it is not an integer in [lo, hi].
func queryInt(r *http.Request, name string, def, lo, hi int) (int, bool) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return def, true
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < lo || v > hi {
		return 0, false
	}
	return v, true
}
