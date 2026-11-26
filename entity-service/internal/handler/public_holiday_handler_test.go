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
	gotReq    domain.SearchSchedulePublicHolidaysRequest
	gotList   domain.ListPublicHolidaysRequest
	gotAdd    domain.AddPublicHolidaysRequest
	gotRemove string
	err       error
	started   bool
}

// Search records the request and returns the canned result, or the configured error.
func (f *fakePublicHolidaySvc) Search(_ context.Context, req domain.SearchSchedulePublicHolidaysRequest) (domain.SchedulePublicHolidaysResponse, error) {
	f.gotReq = req
	if f.err != nil {
		return domain.SchedulePublicHolidaysResponse{}, f.err
	}
	return domain.SchedulePublicHolidaysResponse{Holidays: []domain.SchedulePublicHoliday{
		{UserID: "u1", Date: "2026-11-24", Title: "Poya", Country: "Sri Lanka", CountrySource: "TEAM"},
	}, Count: 1}, nil
}

// Refresh returns an empty refresh result.
func (f *fakePublicHolidaySvc) Refresh(context.Context) (domain.PublicHolidayRefreshResult, error) {
	return domain.PublicHolidayRefreshResult{}, nil
}

// RefreshInBackground reports whether the fake was set up to start a run.
func (f *fakePublicHolidaySvc) RefreshInBackground() bool { return f.started }

// LastRefresh returns no previous result.
func (f *fakePublicHolidaySvc) LastRefresh() *domain.PublicHolidayRefreshResult { return nil }

// List records the request and returns one manual row.
func (f *fakePublicHolidaySvc) List(_ context.Context, req domain.ListPublicHolidaysRequest) (domain.PublicHolidaysListResponse, error) {
	f.gotList = req
	if f.err != nil {
		return domain.PublicHolidaysListResponse{}, f.err
	}
	return domain.PublicHolidaysListResponse{
		Holidays:  []domain.StoredPublicHoliday{{Country: "Sri Lanka", Date: "2026-12-25", Title: "Christmas Day", Source: "MANUAL"}},
		Countries: []string{"India", "Sri Lanka"},
		CanEdit:   true,
	}, nil
}

// Add records the request and reports every day stored.
func (f *fakePublicHolidaySvc) Add(_ context.Context, req domain.AddPublicHolidaysRequest) (domain.AddPublicHolidaysResponse, error) {
	f.gotAdd = req
	if f.err != nil {
		return domain.AddPublicHolidaysResponse{}, f.err
	}
	return domain.AddPublicHolidaysResponse{Stored: len(req.Holidays)}, nil
}

// Remove records the country and date.
func (f *fakePublicHolidaySvc) Remove(_ context.Context, country, date string) error {
	f.gotRemove = country + "|" + date
	return f.err
}

// TestManualPublicHolidayRoutes checks the editor's three handlers: the list
// reads its query, an add is 201 with the body passed through, a delete takes
// the country and date from the path (a space in the country included), and
// the service's refusal keeps its status.
func TestManualPublicHolidayRoutes(t *testing.T) {
	t.Run("list", func(t *testing.T) {
		svc := &fakePublicHolidaySvc{}
		w := httptest.NewRecorder()
		NewPublicHolidayHandler(svc).ListPublicHolidays(w, httptest.NewRequest(http.MethodGet,
			"/team-schedule/public-holidays?from=2026-01-01&to=2026-12-31&country=Sri%20Lanka", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("status %d: %s", w.Code, w.Body.String())
		}
		if svc.gotList != (domain.ListPublicHolidaysRequest{From: "2026-01-01", To: "2026-12-31", Country: "Sri Lanka"}) {
			t.Fatalf("query not passed through: %+v", svc.gotList)
		}
		if !strings.Contains(w.Body.String(), `"canEdit":true`) || !strings.Contains(w.Body.String(), `"source":"MANUAL"`) {
			t.Fatalf("body %s", w.Body.String())
		}
	})
	t.Run("add", func(t *testing.T) {
		svc := &fakePublicHolidaySvc{}
		w := httptest.NewRecorder()
		NewPublicHolidayHandler(svc).AddPublicHolidays(w, httptest.NewRequest(http.MethodPost, "/team-schedule/public-holidays",
			strings.NewReader(`{"country":"India","holidays":[{"date":"2026-01-26","title":"Republic Day"}]}`)))
		if w.Code != http.StatusCreated {
			t.Fatalf("status %d: %s", w.Code, w.Body.String())
		}
		if svc.gotAdd.Country != "India" || len(svc.gotAdd.Holidays) != 1 || svc.gotAdd.Holidays[0].Title != "Republic Day" {
			t.Fatalf("body not passed through: %+v", svc.gotAdd)
		}
	})
	t.Run("delete", func(t *testing.T) {
		svc := &fakePublicHolidaySvc{}
		mux := http.NewServeMux()
		mux.HandleFunc("DELETE /team-schedule/public-holidays/{country}/{date}", NewPublicHolidayHandler(svc).DeletePublicHoliday)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/team-schedule/public-holidays/Sri%20Lanka/2026-12-25", nil))
		if w.Code != http.StatusNoContent {
			t.Fatalf("status %d: %s", w.Code, w.Body.String())
		}
		if svc.gotRemove != "Sri Lanka|2026-12-25" {
			t.Fatalf("path not passed through: %q", svc.gotRemove)
		}
	})
	t.Run("a refusal keeps its status", func(t *testing.T) {
		svc := &fakePublicHolidaySvc{err: &apierror.ForbiddenError{Msg: "only a portal admin or a rota admin can change the public holidays"}}
		w := httptest.NewRecorder()
		NewPublicHolidayHandler(svc).AddPublicHolidays(w, httptest.NewRequest(http.MethodPost, "/team-schedule/public-holidays",
			strings.NewReader(`{"country":"India","holidays":[{"date":"2026-01-26","title":"Republic Day"}]}`)))
		if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "rota admin") {
			t.Fatalf("status %d: %s", w.Code, w.Body.String())
		}
	})
}

// TestSearchPublicHolidays checks that the search handler passes the request
// to the service and returns the holidays it gets back.
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

// TestSearchPublicHolidays_ValidationAndUnknownField checks that a validation
// error from the service and an unknown request field both answer 400.
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

// TestRefreshPublicHolidays checks that a refresh answers 202 when it starts
// and 409 when one is already running.
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
