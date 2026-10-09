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

// CloudStatusHandler handles the cloud status webhook endpoints.
//
// These are service-to-service endpoints, called by csm-scheduled-tasks rather
// than by the portal. They are mounted under /internal for that reason.
type CloudStatusHandler struct {
	svc service.CloudStatusService
}

// NewCloudStatusHandler constructs a CloudStatusHandler.
func NewCloudStatusHandler(svc service.CloudStatusService) *CloudStatusHandler {
	return &CloudStatusHandler{svc: svc}
}

// Sweep handles POST /internal/cloud-status/sweep.
func (h *CloudStatusHandler) Sweep(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.Sweep(r.Context())
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// Pending handles GET /internal/cloud-status/pending.
func (h *CloudStatusHandler) Pending(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.PendingWebhooks(r.Context())
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// RecordDelivery handles POST /internal/cloud-status/{id}/delivery.
func (h *CloudStatusHandler) RecordDelivery(w http.ResponseWriter, r *http.Request) {
	var req domain.RecordCloudStatusDeliveryRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	req.ID = r.PathValue("id")
	if err := h.svc.RecordDelivery(r.Context(), req); err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Claim handles POST /internal/cloud-status/{id}/claim: csm-notification-service
// claims a webhook it received as outage.status_page_due before posting it.
// 204 means post it; 409 means do not -- already delivered, or no longer
// reserved under this token.
func (h *CloudStatusHandler) Claim(w http.ResponseWriter, r *http.Request) {
	var req domain.ClaimCloudStatusWebhookRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	req.ID = r.PathValue("id")
	if err := h.svc.ClaimWebhook(r.Context(), req); err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
