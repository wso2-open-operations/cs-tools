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
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// SLABreachReportHandler serves the weekly SLA breach report.
type SLABreachReportHandler struct {
	svc service.SLABreachReportService
}

// NewSLABreachReportHandler constructs a SLABreachReportHandler.
func NewSLABreachReportHandler(svc service.SLABreachReportService) *SLABreachReportHandler {
	return &SLABreachReportHandler{svc: svc}
}

// GetSLABreachReport handles
// GET /reports/sla-breaches?definitions=<name>,<name>&stages=<stage>&week=YYYY-MM-DD.
//
// definitions is required and may be given once comma-separated or
// repeated; the service answers a missing list with a validation error.
// stages is optional (empty means every stage). week is optional, is any
// date inside the week to report on, and defaults to the week before now
// -- it exists so a missed run can be re-sent for the right week rather
// than silently reporting the wrong one. The scheduled task that mails the
// report is the only expected caller.
func (h *SLABreachReportHandler) GetSLABreachReport(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	var definitions []string
	for _, raw := range q["definitions"] {
		for _, d := range strings.Split(raw, ",") {
			if d = strings.TrimSpace(d); d != "" {
				definitions = append(definitions, d)
			}
		}
	}
	var stages []string
	for _, raw := range q["stages"] {
		for _, s := range strings.Split(raw, ",") {
			if s = strings.TrimSpace(s); s != "" {
				stages = append(stages, s)
			}
		}
	}

	var week time.Time
	if raw := strings.TrimSpace(q.Get("week")); raw != "" {
		parsed, err := time.Parse("2006-01-02", raw)
		if err != nil {
			writeServiceError(w, r, &apierror.ValidationError{Msg: `week must be a date, formatted YYYY-MM-DD`})
			return
		}
		week = parsed
	}

	report, err := h.svc.BuildReport(r.Context(), domain.SLABreachReportRequest{
		Week:        week,
		Definitions: definitions,
		Stages:      stages,
	})
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(report)
}
