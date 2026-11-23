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
	"errors"
	"io"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
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
	if !decodeNoOptions(w, r) {
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

// decodeNoOptions accepts an empty body or a single empty JSON object, and
// answers 400 for anything else. The upkeep takes no options, so a body that
// names one is a mistake the caller should hear about.
func decodeNoOptions(w http.ResponseWriter, r *http.Request) bool {
	if r.Body == nil || r.Body == http.NoBody {
		return true
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	var none struct{}
	if err := dec.Decode(&none); err != nil {
		if errors.Is(err, io.EOF) {
			return true
		}
		writeServiceError(w, r, &apierror.ValidationError{Msg: decodeErrMsg(err)})
		return false
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeServiceError(w, r, &apierror.ValidationError{Msg: "request body must contain a single JSON object"})
		return false
	}
	return true
}
