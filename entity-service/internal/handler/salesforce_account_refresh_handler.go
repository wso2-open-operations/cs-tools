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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// SalesforceAccountRefreshHandler handles POST /salesforce/accounts/{sfId}/refresh.
type SalesforceAccountRefreshHandler struct {
	svc service.AccountRefreshService
}

// NewSalesforceAccountRefreshHandler constructs a SalesforceAccountRefreshHandler.
func NewSalesforceAccountRefreshHandler(svc service.AccountRefreshService) *SalesforceAccountRefreshHandler {
	return &SalesforceAccountRefreshHandler{svc: svc}
}

// Refresh handles POST /salesforce/accounts/{sfId}/refresh: it re-reads the
// account from Salesforce and writes it, answering 204. Internal callers only
// (the service enforces it).
func (h *SalesforceAccountRefreshHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Refresh(r.Context(), r.PathValue("sfId")); err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
