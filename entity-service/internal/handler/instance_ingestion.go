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

// InstanceIngestionHandler handles HTTP requests for telemetry and instance ingestion.
type InstanceIngestionHandler struct {
	svc service.InstanceIngestionService
}

// NewInstanceIngestionHandler constructs an InstanceIngestionHandler with the given service.
func NewInstanceIngestionHandler(svc service.InstanceIngestionService) *InstanceIngestionHandler {
	return &InstanceIngestionHandler{svc: svc}
}

// UpsertInstanceMeta handles POST /instances/meta.
func (h *InstanceIngestionHandler) UpsertInstanceMeta(w http.ResponseWriter, r *http.Request) {
	var req domain.InstanceMetaRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.UpsertInstanceMeta(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// InsertInstanceInfo handles POST /instances/info.
func (h *InstanceIngestionHandler) InsertInstanceInfo(w http.ResponseWriter, r *http.Request) {
	var req domain.InstanceInfoRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.InsertInstanceInfo(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// InsertUsageStats handles POST /instances/usage-stats.
func (h *InstanceIngestionHandler) InsertUsageStats(w http.ResponseWriter, r *http.Request) {
	var req domain.InstanceUsageIngestRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.InsertUsageStats(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// ImportDailyUsagesBatch handles POST /instances/daily-usages/batch.
func (h *InstanceIngestionHandler) ImportDailyUsagesBatch(w http.ResponseWriter, r *http.Request) {
	var req domain.DailyUsageBatchRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.ImportDailyUsagesBatch(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// GetProjectSecretKey handles GET /projects/{key}/secret-key, where key is the
// project key (a project id is accepted too). The body is a credential, so it
// must never be cached on the way back.
func (h *InstanceIngestionHandler) GetProjectSecretKey(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.GetProjectSecretKey(r.Context(), r.PathValue("key"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// GetDeploymentConsumptionSecret handles GET /deployments/by-number/{number}/consumption-secret.
// The body carries a credential, so it must never be cached on the way back.
func (h *InstanceIngestionHandler) GetDeploymentConsumptionSecret(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.GetDeploymentConsumptionSecret(r.Context(), r.PathValue("number"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
