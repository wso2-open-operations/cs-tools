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
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// ProductIssueDashboardHandler serves the product issue dashboard, report data and
// recipient list. All three routes are internal-only (routes.go).
type ProductIssueDashboardHandler struct {
	svc service.ProductIssueDashboardService
}

// NewProductIssueDashboardHandler constructs a ProductIssueDashboardHandler.
func NewProductIssueDashboardHandler(svc service.ProductIssueDashboardService) *ProductIssueDashboardHandler {
	return &ProductIssueDashboardHandler{svc: svc}
}

// GetDashboard handles GET /product-work-items/dashboard.
func (h *ProductIssueDashboardHandler) GetDashboard(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	resp, err := h.svc.Dashboard(r.Context(), service.ProductIssueDashboardRequest{
		From: strings.TrimSpace(q.Get("from")), To: strings.TrimSpace(q.Get("to")),
		Bucket: strings.TrimSpace(q.Get("bucket")), Products: csvParam(q["products"]),
	})
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeProductIssueJSON(w, http.StatusOK, resp)
}

// GetReport handles GET /reports/product-issues.
func (h *ProductIssueDashboardHandler) GetReport(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	resp, err := h.svc.Report(r.Context(), service.ProductIssueReportRequest{
		Period: strings.TrimSpace(q.Get("period")), Date: strings.TrimSpace(q.Get("date")), Products: csvParam(q["products"]),
	})
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeProductIssueJSON(w, http.StatusOK, resp)
}

// GetRecipients handles GET /reports/product-issues/recipients.
func (h *ProductIssueDashboardHandler) GetRecipients(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.Recipients(r.Context())
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeProductIssueJSON(w, http.StatusOK, resp)
}
