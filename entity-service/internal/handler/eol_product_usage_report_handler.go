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

// EolProductUsageReportHandler serves the monthly EOL product usage report.
type EolProductUsageReportHandler struct {
	svc service.EolProductUsageReportService
}

// NewEolProductUsageReportHandler constructs an EolProductUsageReportHandler.
func NewEolProductUsageReportHandler(svc service.EolProductUsageReportService) *EolProductUsageReportHandler {
	return &EolProductUsageReportHandler{svc: svc}
}

// GetEolProductUsageReport handles GET /reports/eol-product-usage?asOf=YYYY-MM-DD.
// asOf is optional and defaults to today; the scheduled task that mails the
// report is the only expected caller.
func (h *EolProductUsageReportHandler) GetEolProductUsageReport(w http.ResponseWriter, r *http.Request) {
	report, err := h.svc.BuildReport(r.Context(), domain.EolProductUsageReportRequest{
		AsOf: strings.TrimSpace(r.URL.Query().Get("asOf")),
	})
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(report)
}
