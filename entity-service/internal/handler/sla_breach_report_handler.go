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

// csvParam flattens a repeated query parameter whose values may also be
// comma-separated, trimming and dropping blanks.
func csvParam(raw []string) []string {
	var out []string
	for _, group := range raw {
		for _, v := range strings.Split(group, ",") {
			if v = strings.TrimSpace(v); v != "" {
				out = append(out, v)
			}
		}
	}
	return out
}

// GetSLABreachReport handles GET /reports/sla-breaches with
// definitions, stages, onboardingStatuses, excludeOnboardingStatuses and
// week.
//
// definitions is required and may be given once comma-separated or
// repeated; the service answers a missing list with a validation error.
// The other filters are optional and each means "no restriction" when
// absent. onboardingStatuses is ServiceNow's "Onboarding Status is one
// of", excludeOnboardingStatuses its "is not one of" -- one report uses
// each, which is why both exist rather than a single flag. week is any
// date inside the week to report on and defaults to the week before now,
// so a missed run can be re-sent for the right week rather than silently
// reporting the wrong one. The scheduled tasks that mail the two reports
// are the only expected callers.
func (h *SLABreachReportHandler) GetSLABreachReport(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	definitions := csvParam(q["definitions"])
	stages := csvParam(q["stages"])
	onboardingStatuses := csvParam(q["onboardingStatuses"])
	excludeOnboardingStatuses := csvParam(q["excludeOnboardingStatuses"])

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
		Week:                      week,
		Definitions:               definitions,
		Stages:                    stages,
		OnboardingStatuses:        onboardingStatuses,
		ExcludeOnboardingStatuses: excludeOnboardingStatuses,
	})
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(report)
}
