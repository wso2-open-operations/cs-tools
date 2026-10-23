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
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// ProductRegressionReportHandler serves the monthly product regression
// reports.
type ProductRegressionReportHandler struct {
	svc service.ProductRegressionReportService
}

// NewProductRegressionReportHandler constructs a ProductRegressionReportHandler.
func NewProductRegressionReportHandler(svc service.ProductRegressionReportService) *ProductRegressionReportHandler {
	return &ProductRegressionReportHandler{svc: svc}
}

// GetProductRegressionReport handles
// GET /reports/product-regressions?products=<name>,<name>&month=YYYY-MM.
//
// products is required and may be given once comma-separated or repeated;
// the service answers a missing list with a validation error, which
// writeServiceError maps to 400 like every other one. month is optional
// and defaults to the month before now. The scheduled task that mails the
// reports is the only expected caller.
func (h *ProductRegressionReportHandler) GetProductRegressionReport(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var products []string
	for _, raw := range q["products"] {
		for _, p := range strings.Split(raw, ",") {
			if p = strings.TrimSpace(p); p != "" {
				products = append(products, p)
			}
		}
	}
	report, err := h.svc.BuildReport(r.Context(), domain.ProductRegressionReportRequest{
		Products: products,
		Month:    strings.TrimSpace(q.Get("month")),
	})
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(report)
}
