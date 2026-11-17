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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.
package handler

import (
	"encoding/json"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// AuditLogMaintenanceHandler handles the audit log upkeep request.
type AuditLogMaintenanceHandler struct {
	svc service.AuditLogMaintenanceService
}

// NewAuditLogMaintenanceHandler constructs an AuditLogMaintenanceHandler.
func NewAuditLogMaintenanceHandler(svc service.AuditLogMaintenanceService) *AuditLogMaintenanceHandler {
	return &AuditLogMaintenanceHandler{svc: svc}
}

// Run handles POST /audit-log/maintenance.
func (h *AuditLogMaintenanceHandler) Run(w http.ResponseWriter, r *http.Request) {
	// The call takes no options: an empty body (or {}) is fine, anything else is a 400.
	var none struct{}
	if !decodeOptionalRequest(w, r, &none) {
		return
	}
	resp, err := h.svc.RunMaintenance(r.Context())
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
