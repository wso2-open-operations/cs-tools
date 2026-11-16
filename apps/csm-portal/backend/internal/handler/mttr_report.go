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
	"context"
	"log/slog"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// entityMTTRReportClient abstracts the entity service's MTTR report operation.
type entityMTTRReportClient interface {
	GetMTTRReport(ctx context.Context, months, days string, creTeamIDs []string) ([]byte, error)
}

// mttrReportAllowedMonths are the only trailing windows the report accepts,
// mirroring entity-service's own GET /reports/mttr.
var mttrReportAllowedMonths = map[string]bool{"1": true, "3": true, "6": true, "12": true}

// mttrReportAllowedDays are the trailing day windows the report accepts.
var mttrReportAllowedDays = map[string]bool{"7": true}

// mttrWindow is the trailing window asked for: exactly one of Months or Days.
type mttrWindow struct {
	Months string
	Days   string
}

// MTTRReportHandler handles HTTP requests for the MTTR (mean time to
// resolution) report: a thin proxy of entity-service's own GET /reports/mttr,
// which backs the dashboard builder's "mttr" resourceType.
type MTTRReportHandler struct {
	entity entityMTTRReportClient
}

// NewMTTRReportHandler creates an MTTRReportHandler backed by the given
// entity client.
func NewMTTRReportHandler(entity entityMTTRReportClient) *MTTRReportHandler {
	return &MTTRReportHandler{entity: entity}
}

// GetMTTRReport handles GET /reports/mttr?months=N|days=N[&creTeamId=<id>&creTeamId=<id>...].
// Exactly one window is required: months ("1", "3", "6" or "12") or days ("7"). creTeamId may repeat; each
// must be a UUID (a dashboard widget's own creTeamIds are already resolved to
// real group ids at generation time, see internal/handler/dashboard_teams.go).
func (h *MTTRReportHandler) GetMTTRReport(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	q := r.URL.Query()
	window := mttrWindow{Months: q.Get("months"), Days: q.Get("days")}
	if (window.Months == "") == (window.Days == "") ||
		(window.Months != "" && !mttrReportAllowedMonths[window.Months]) ||
		(window.Days != "" && !mttrReportAllowedDays[window.Days]) {
		writeError(w, http.StatusBadRequest, "exactly one of months (1, 3, 6 or 12) or days (7) is required.")
		return
	}
	creTeamIDs := q["creTeamId"]
	for _, id := range creTeamIDs {
		if !uuidRe.MatchString(id) {
			writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
			return
		}
	}

	result, err := h.entity.GetMTTRReport(r.Context(), window.Months, window.Days, creTeamIDs)
	if err != nil {
		slog.ErrorContext(r.Context(), "entity GetMTTRReport failed", "userID", user.UserID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to retrieve the MTTR report.")
		return
	}

	writeJSON(w, http.StatusOK, result)
}
