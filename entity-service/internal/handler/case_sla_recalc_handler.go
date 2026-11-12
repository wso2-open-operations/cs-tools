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
	"net/http"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// CaseSLARecalcHandler handles HTTP requests for the per-case SLA pause
// recalculation admin action -- see service.CaseSLARecalcService's own doc
// comment for the admin gate, and service.SLAEngineService.RecalculateCasePauseTotals'
// own doc comment for what it actually computes and corrects.
type CaseSLARecalcHandler struct {
	svc service.CaseSLARecalcService
}

// NewCaseSLARecalcHandler constructs a CaseSLARecalcHandler with the given
// service.
func NewCaseSLARecalcHandler(svc service.CaseSLARecalcService) *CaseSLARecalcHandler {
	return &CaseSLARecalcHandler{svc: svc}
}

// caseSLAPauseRecalcClockResponse/caseSLAPauseRecalcResponse are this
// endpoint's own wire shape -- a plain, hand-written mirror of
// service.CaseClockPauseRecalcResult/CaseSLAPauseRecalcReport (every field
// here always public JSON, unlike several of this service's own case-data
// response types, so no domain.CaseView-style struct reuse is warranted).
type caseSLAPauseRecalcClockResponse struct {
	Target                  string `json:"target"`
	Schedule                string `json:"schedule"`
	Skipped                 bool   `json:"skipped"`
	SkipReason              string `json:"skipReason,omitempty"`
	OldPauseSeconds         int64  `json:"oldPauseSeconds"`
	NewPauseSeconds         int64  `json:"newPauseSeconds"`
	OldBusinessPauseSeconds int64  `json:"oldBusinessPauseSeconds,omitempty"`
	NewBusinessPauseSeconds int64  `json:"newBusinessPauseSeconds,omitempty"`
	Updated                 bool   `json:"updated"`
}

type caseSLAPauseRecalcResponse struct {
	HasHistory bool                              `json:"hasHistory"`
	Clocks     []caseSLAPauseRecalcClockResponse `json:"clocks"`
}

// RecalculatePauseTotals handles POST /cases/{id}/sla/recalculate-pause --
// admin-only (gated by service.CaseSLARecalcService itself, trusting
// csm-portal-backend's own PermAdmin route check -- see that service's own
// doc comment). No request body.
func (h *CaseSLARecalcHandler) RecalculatePauseTotals(w http.ResponseWriter, r *http.Request) {
	caseID := r.PathValue("id")
	report, err := h.svc.RecalculateCasePauseTotals(r.Context(), caseID)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}

	resp := caseSLAPauseRecalcResponse{HasHistory: report.HasHistory, Clocks: make([]caseSLAPauseRecalcClockResponse, 0, len(report.Clocks))}
	for _, c := range report.Clocks {
		resp.Clocks = append(resp.Clocks, caseSLAPauseRecalcClockResponse{
			Target:                  c.Target,
			Schedule:                c.Schedule,
			Skipped:                 c.Skipped,
			SkipReason:              c.SkipReason,
			OldPauseSeconds:         c.OldPauseSeconds,
			NewPauseSeconds:         c.NewPauseSeconds,
			OldBusinessPauseSeconds: c.OldBusinessPauseSeconds,
			NewBusinessPauseSeconds: c.NewBusinessPauseSeconds,
			Updated:                 c.Updated,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
