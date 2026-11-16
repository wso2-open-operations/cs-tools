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
	"strconv"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// MTTRReportHandler serves the MTTR (mean time to resolution) report.
type MTTRReportHandler struct {
	svc service.MTTRReportService
}

// NewMTTRReportHandler constructs an MTTRReportHandler.
func NewMTTRReportHandler(svc service.MTTRReportService) *MTTRReportHandler {
	return &MTTRReportHandler{svc: svc}
}

// GetMTTRReport handles GET /reports/mttr?months=N|days=N[&creTeamId=<id>&creTeamId=<id>...].
//
// Exactly one window is required: months (1, 3, 6 or 12) or days (7) -- the
// only trailing windows the dashboard offers. creTeamId may repeat to restrict the report to one or
// more CRE teams; omitted, every team is included. The report groups cases
// by severity band (S1-S4, S1-S3, S1, S2, S3, S4) and measures, for each,
// the average time from a case's creation to the LAST time it entered
// Solution Proposed (so a rejected-then-reproposed case is measured against
// the proposal that actually stuck), over cases whose last such entry falls
// within the window.
func (h *MTTRReportHandler) GetMTTRReport(w http.ResponseWriter, r *http.Request) {
	rawMonths := r.URL.Query().Get("months")
	rawDays := r.URL.Query().Get("days")
	if (rawMonths == "") == (rawDays == "") {
		writeServiceError(w, r, &apierror.ValidationError{
			Msg: "exactly one of months (1, 3, 6 or 12) or days (7) is required",
		})
		return
	}
	req := domain.MTTRReportRequest{CreTeamIDs: r.URL.Query()["creTeamId"]}
	if rawMonths != "" {
		months, err := strconv.Atoi(rawMonths)
		if err != nil {
			writeServiceError(w, r, &apierror.ValidationError{
				Msg: "months must be one of 1, 3, 6 or 12",
			})
			return
		}
		req.WindowMonths = months
	} else {
		days, err := strconv.Atoi(rawDays)
		if err != nil {
			writeServiceError(w, r, &apierror.ValidationError{Msg: "days must be 7"})
			return
		}
		req.WindowDays = days
	}
	report, err := h.svc.BuildReport(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(report)
}
