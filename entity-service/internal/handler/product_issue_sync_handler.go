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
	"net/http"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// ProductIssueSyncHandler serves the service-credential routes that reconcile
// linked product issues with GitHub and backfill a repository's issues.
type ProductIssueSyncHandler struct {
	svc service.ProductIssueSyncService
}

// NewProductIssueSyncHandler constructs a ProductIssueSyncHandler.
func NewProductIssueSyncHandler(svc service.ProductIssueSyncService) *ProductIssueSyncHandler {
	return &ProductIssueSyncHandler{svc: svc}
}

// Reconcile handles POST /product-work-items/reconcile. An empty body starts a run.
func (h *ProductIssueSyncHandler) Reconcile(w http.ResponseWriter, r *http.Request) {
	var req domain.ProductIssueReconcileRequest
	if !decodeOptionalRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.Reconcile(r.Context(), req)
	if err != nil {
		writeProductIssueError(w, r, err)
		return
	}
	writeProductIssueJSON(w, http.StatusOK, resp)
}

// Backfill handles POST /product-work-items/backfill.
func (h *ProductIssueSyncHandler) Backfill(w http.ResponseWriter, r *http.Request) {
	var req domain.ProductIssueBackfillRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.Backfill(r.Context(), req)
	if err != nil {
		writeProductIssueError(w, r, err)
		return
	}
	writeProductIssueJSON(w, http.StatusOK, resp)
}
