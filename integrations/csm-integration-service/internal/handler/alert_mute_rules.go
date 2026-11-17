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
)

// entityAlertMuteRuleClient abstracts the entity service alert-mute-rule operation
// used by AlertMuteRuleHandler.
type entityAlertMuteRuleClient interface {
	ListActiveAlertMuteRules(ctx context.Context) ([]byte, error)
}

// AlertMuteRuleHandler serves the alert mute rules sre-alert-core-service reads to
// decide which alerts to mute. Creating and changing them happens in the CSM
// Portal; this service only exposes the active list.
type AlertMuteRuleHandler struct {
	entity entityAlertMuteRuleClient
}

// NewAlertMuteRuleHandler creates an AlertMuteRuleHandler backed by the given entity
// client.
func NewAlertMuteRuleHandler(entity entityAlertMuteRuleClient) *AlertMuteRuleHandler {
	return &AlertMuteRuleHandler{entity: entity}
}

// ListActiveAlertMuteRules handles GET /alert-mute-rules/active, returning
// entity-service's {"alertMuteRules":[...]} unchanged.
func (h *AlertMuteRuleHandler) ListActiveAlertMuteRules(w http.ResponseWriter, r *http.Request) {
	result, err := h.entity.ListActiveAlertMuteRules(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "entity ListActiveAlertMuteRules failed", "err", summarizeErr(err))
		mapUpstreamError(w, err, "Failed to list active alert mute rules.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}
