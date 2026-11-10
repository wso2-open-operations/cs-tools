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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

type fakePublicHolidaySvc struct {
	gotReq  domain.SearchSchedulePublicHolidaysRequest
	err     error
	started bool
}

func (f *fakePublicHolidaySvc) Search(_ context.Context, req domain.SearchSchedulePublicHolidaysRequest) (domain.SchedulePublicHolidaysResponse, error) {
	f.gotReq = req
	if f.err != nil {
		return domain.SchedulePublicHolidaysResponse{}, f.err
	}
	return domain.SchedulePublicHolidaysResponse{Holidays: []domain.SchedulePublicHoliday{
		{UserID: "u1", Date: "2026-11-24", Title: "Poya", Country: "Sri Lanka", CountrySource: "TEAM"},
	}, Count: 1}, nil
}

func (f *fakePublicHolidaySvc) Refresh(context.Context) (domain.PublicHolidayRefreshResult, error) {
	return domain.PublicHolidayRefreshResult{}, nil
}
func (f *fakePublicHolidaySvc) RefreshInBackground() bool                       { return f.started }
func (f *fakePublicHolidaySvc) LastRefresh() *domain.PublicHolidayRefreshResult { return nil }

func TestSearchPublicHolidays(t *testing.T) {
	svc := &fakePublicHolidaySvc{}
	h := NewPublicHolidayHandler(svc)
	w := httptest.NewRecorder()
	h.SearchPublicHolidays(w, httptest.NewRequest(http.MethodPost, "/team-schedule/public-holidays/search",
		strings.NewReader(`{"from":"2026-11-01","to":"2026-11-30","userIds":["u1"]}`)))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if svc.gotReq.From != "2026-11-01" || len(svc.gotReq.UserIDs) != 1 {
		t.Fatalf("request not passed through: %+v", svc.gotReq)
	}
	var resp domain.SchedulePublicHolidaysResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil || resp.Count != 1 || resp.Holidays[0].Title != "Poya" {
		t.Fatalf("body %s (%v)", w.Body.String(), err)
	}
}

func TestSearchPublicHolidays_ValidationAndUnknownField(t *testing.T) {
	h := NewPublicHolidayHandler(&fakePublicHolidaySvc{err: &apierror.ValidationError{Msg: "userIds is required"}})
	w := httptest.NewRecorder()
	h.SearchPublicHolidays(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"from":"2026-11-01","to":"2026-11-30"}`)))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("validation: status %d", w.Code)
	}
	w = httptest.NewRecorder()
	h.SearchPublicHolidays(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"bogus":1}`)))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("unknown field: status %d", w.Code)
	}
}

func TestRefreshPublicHolidays(t *testing.T) {
	w := httptest.NewRecorder()
	NewPublicHolidayHandler(&fakePublicHolidaySvc{started: true}).RefreshPublicHolidays(w, httptest.NewRequest(http.MethodPost, "/", nil))
	if w.Code != http.StatusAccepted || !strings.Contains(w.Body.String(), `"started":true`) {
		t.Fatalf("started: %d %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	NewPublicHolidayHandler(&fakePublicHolidaySvc{}).RefreshPublicHolidays(w, httptest.NewRequest(http.MethodPost, "/", nil))
	if w.Code != http.StatusConflict {
		t.Fatalf("running: %d", w.Code)
	}
}
