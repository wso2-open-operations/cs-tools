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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// EngagementAllocationHandler handles HTTP requests for the
// customer-engagement allocation tables — see
// domain.StatusUpdateReminderRecipient's doc comment for what they are.
type EngagementAllocationHandler struct {
	svc service.EngagementAllocationService
}

// NewEngagementAllocationHandler constructs an EngagementAllocationHandler
// with the given service.
func NewEngagementAllocationHandler(svc service.EngagementAllocationService) *EngagementAllocationHandler {
	return &EngagementAllocationHandler{svc: svc}
}

// StatusUpdateReminderRecipients handles
// GET /engagement-allocations/status-update-reminders?cycleStartDate=YYYY-MM-DD.
//
// Backs the weekly reminder sub-cron in operations/csm-scheduled-tasks. A GET
// with a query parameter rather than the POST /search shape the case endpoints
// use: there is exactly one parameter, it is not a filter set, and the result
// is a derived audience rather than a page of rows.
func (h *EngagementAllocationHandler) StatusUpdateReminderRecipients(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.StatusUpdateReminderRecipients(r.Context(), r.URL.Query().Get("cycleStartDate"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
