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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// PublicHolidayHandler serves the Team Schedule's public holidays.
type PublicHolidayHandler struct {
	svc service.PublicHolidayService
}

// NewPublicHolidayHandler constructs a PublicHolidayHandler.
func NewPublicHolidayHandler(svc service.PublicHolidayService) *PublicHolidayHandler {
	return &PublicHolidayHandler{svc: svc}
}

// SearchPublicHolidays handles POST /team-schedule/public-holidays/search:
// each given person's public holidays in a window, from the local copy.
func (h *PublicHolidayHandler) SearchPublicHolidays(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchSchedulePublicHolidaysRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.Search(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeScheduleJSON(w, http.StatusOK, resp)
}

// publicHolidayRefreshResponse answers POST /team-schedule/public-holidays/refresh.
type publicHolidayRefreshResponse struct {
	Started     bool                               `json:"started"`
	LastRefresh *domain.PublicHolidayRefreshResult `json:"lastRefresh"`
}

// RefreshPublicHolidays handles POST /team-schedule/public-holidays/refresh
// (service credential only): starts a refresh in the background, 202, or
// 409 while one is running. The result is logged and shows as lastRefresh on
// the next call.
func (h *PublicHolidayHandler) RefreshPublicHolidays(w http.ResponseWriter, r *http.Request) {
	if !h.svc.RefreshInBackground() {
		apierror.WriteJSON(w, http.StatusConflict, "a public holiday refresh is already running")
		return
	}
	writeScheduleJSON(w, http.StatusAccepted, publicHolidayRefreshResponse{Started: true, LastRefresh: h.svc.LastRefresh()})
}
