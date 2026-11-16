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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// HolidayAvailabilityHandler serves working on a public holiday (CRE).
type HolidayAvailabilityHandler struct {
	svc service.HolidayAvailabilityService
}

// NewHolidayAvailabilityHandler constructs a HolidayAvailabilityHandler.
func NewHolidayAvailabilityHandler(svc service.HolidayAvailabilityService) *HolidayAvailabilityHandler {
	return &HolidayAvailabilityHandler{svc: svc}
}

// SearchHolidayAvailability handles POST /team-schedule/holiday-availability/search:
// every team's availability on every holiday in the window.
func (h *HolidayAvailabilityHandler) SearchHolidayAvailability(w http.ResponseWriter, r *http.Request) {
	var req domain.SearchHolidayAvailabilityRequest
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

// ConfirmHolidayAvailability handles
// PUT /team-schedule/holiday-availability/{teamKey}/{date}/confirmation.
func (h *HolidayAvailabilityHandler) ConfirmHolidayAvailability(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.Confirm(r.Context(), r.PathValue("teamKey"), r.PathValue("date"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeScheduleJSON(w, http.StatusOK, resp)
}

// WithdrawHolidayAvailability handles
// DELETE /team-schedule/holiday-availability/{teamKey}/{date}/confirmation.
func (h *HolidayAvailabilityHandler) WithdrawHolidayAvailability(w http.ResponseWriter, r *http.Request) {
	resp, err := h.svc.Withdraw(r.Context(), r.PathValue("teamKey"), r.PathValue("date"))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeScheduleJSON(w, http.StatusOK, resp)
}

// DueHolidayReminders handles POST /team-schedule/holiday-availability/reminders/due
// (service credential only): the reminders the daily task sends today.
func (h *HolidayAvailabilityHandler) DueHolidayReminders(w http.ResponseWriter, r *http.Request) {
	var req domain.DueHolidayRemindersRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	resp, err := h.svc.DueReminders(r.Context(), req)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}
	writeScheduleJSON(w, http.StatusOK, resp)
}

// RecordHolidayReminder handles
// PUT /team-schedule/holiday-availability/reminders/{teamKey}/{date}/{stage}
// (service credential only): a reminder went out. 204.
func (h *HolidayAvailabilityHandler) RecordHolidayReminder(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.RecordReminder(r.Context(), r.PathValue("teamKey"), r.PathValue("date"), r.PathValue("stage")); err != nil {
		writeServiceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
