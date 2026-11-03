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
	"context"
	"encoding/json"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// ACPReopenChecker is the daily ACP reopen check
// (service.ACPReopen.CheckSuspendedProjects).
type ACPReopenChecker interface {
	CheckSuspendedProjects(ctx context.Context, req domain.ACPReopenCheckRequest) (domain.ACPReopenCheckResponse, error)
}

// ACPReopenCheckHandler serves POST /acp/reopen-check.
type ACPReopenCheckHandler struct {
	svc ACPReopenChecker
}

// NewACPReopenCheckHandler constructs the handler.
func NewACPReopenCheckHandler(svc ACPReopenChecker) *ACPReopenCheckHandler {
	return &ACPReopenCheckHandler{svc: svc}
}

// Check handles POST /acp/reopen-check.
func (h *ACPReopenCheckHandler) Check(w http.ResponseWriter, r *http.Request) {
	var req domain.ACPReopenCheckRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.CheckSuspendedProjects(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
