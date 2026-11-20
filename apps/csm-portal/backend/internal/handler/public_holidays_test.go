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
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
)

// TestPublicHolidayEditor pins the holiday editor's three routes: the list
// passes its query, an add is 201 with the body passed through and keeps
// entity-service's 400 and 403 reasons, a delete reads the country (a space
// included) and date from the path and refuses a bad date without an upstream
// call, and a 5xx stays generic.
func TestPublicHolidayEditor(t *testing.T) {
	t.Run("list passes the query", func(t *testing.T) {
		var got [3]string
		h := NewScheduleHandler(&mockEntityScheduleClient{
			listHolidaysFn: func(_ context.Context, from, to, country string) ([]byte, error) {
				got = [3]string{from, to, country}
				return []byte(`{"holidays":[],"countries":["India","Sri Lanka"],"canEdit":true}`), nil
			},
		})
		w := httptest.NewRecorder()
		h.ListPublicHolidays(w, withUser(httptest.NewRequest(http.MethodGet,
			"/team-schedule/public-holidays?from=2026-01-01&to=2026-12-31&country=Sri%20Lanka", nil)))
		if w.Code != http.StatusOK {
			t.Fatalf("status %d: %s", w.Code, w.Body.String())
		}
		if got != [3]string{"2026-01-01", "2026-12-31", "Sri Lanka"} {
			t.Fatalf("query = %v", got)
		}
		if !strings.Contains(w.Body.String(), `"canEdit":true`) {
			t.Fatalf("body %s", w.Body.String())
		}
	})

	t.Run("add is 201 with the body passed through", func(t *testing.T) {
		var body string
		h := NewScheduleHandler(&mockEntityScheduleClient{
			addHolidaysFn: func(_ context.Context, b []byte) ([]byte, error) {
				body = string(b)
				return []byte(`{"stored":1}`), nil
			},
		})
		w := httptest.NewRecorder()
		payload := `{"country":"India","holidays":[{"date":"2027-01-26","title":"Republic Day"}]}`
		h.AddPublicHolidays(w, withUser(httptest.NewRequest(http.MethodPost, "/team-schedule/public-holidays", strings.NewReader(payload))))
		if w.Code != http.StatusCreated {
			t.Fatalf("status %d: %s", w.Code, w.Body.String())
		}
		if body != payload {
			t.Fatalf("body %q, want it unchanged", body)
		}
	})

	reasons := []struct {
		name     string
		err      error
		wantCode int
		wantMsg  string
	}{
		{"400 keeps the day it names", &apierror.Error{StatusCode: http.StatusBadRequest,
			Body: `{"message":"holiday 2: \"2027-13-01\" is not a YYYY-MM-DD date"}`},
			http.StatusBadRequest, "holiday 2"},
		{"403 keeps who may", &apierror.Error{StatusCode: http.StatusForbidden,
			Body: `{"message":"only a portal admin or a rota admin can change the public holidays"}`},
			http.StatusForbidden, "only a portal admin or a rota admin"},
		{"500 stays generic", &apierror.Error{StatusCode: http.StatusInternalServerError,
			Body: `{"message":"pq: relation does not exist"}`},
			http.StatusInternalServerError, ""},
	}
	for _, tc := range reasons {
		t.Run("add "+tc.name, func(t *testing.T) {
			h := NewScheduleHandler(&mockEntityScheduleClient{
				addHolidaysFn: func(context.Context, []byte) ([]byte, error) { return nil, tc.err },
			})
			w := httptest.NewRecorder()
			h.AddPublicHolidays(w, withUser(httptest.NewRequest(http.MethodPost, "/team-schedule/public-holidays",
				strings.NewReader(`{"country":"India","holidays":[]}`))))
			if w.Code != tc.wantCode {
				t.Fatalf("status %d, want %d (%s)", w.Code, tc.wantCode, w.Body.String())
			}
			if tc.wantMsg != "" && !strings.Contains(w.Body.String(), tc.wantMsg) {
				t.Fatalf("body %s, want %q", w.Body.String(), tc.wantMsg)
			}
			if tc.wantMsg == "" && strings.Contains(w.Body.String(), "pq:") {
				t.Fatalf("body %s leaks the upstream detail", w.Body.String())
			}
		})
	}

	t.Run("delete reads the path", func(t *testing.T) {
		var got string
		h := NewScheduleHandler(&mockEntityScheduleClient{
			deleteHolidayFn: func(_ context.Context, country, date string) ([]byte, error) {
				got = country + "|" + date
				return nil, nil
			},
		})
		mux := http.NewServeMux()
		mux.HandleFunc("DELETE /team-schedule/public-holidays/{country}/{date}", h.DeletePublicHoliday)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, withUser(httptest.NewRequest(http.MethodDelete, "/team-schedule/public-holidays/Sri%20Lanka/2026-12-25", nil)))
		if w.Code != http.StatusNoContent {
			t.Fatalf("status %d: %s", w.Code, w.Body.String())
		}
		if got != "Sri Lanka|2026-12-25" {
			t.Fatalf("path = %q", got)
		}
	})

	t.Run("delete refuses a bad date without calling upstream", func(t *testing.T) {
		called := false
		h := NewScheduleHandler(&mockEntityScheduleClient{
			deleteHolidayFn: func(context.Context, string, string) ([]byte, error) { called = true; return nil, nil },
		})
		mux := http.NewServeMux()
		mux.HandleFunc("DELETE /team-schedule/public-holidays/{country}/{date}", h.DeletePublicHoliday)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, withUser(httptest.NewRequest(http.MethodDelete, "/team-schedule/public-holidays/India/christmas", nil)))
		if w.Code != http.StatusBadRequest || called {
			t.Fatalf("status %d, called %v", w.Code, called)
		}
	})

	t.Run("an unsigned caller is refused", func(t *testing.T) {
		w := httptest.NewRecorder()
		NewScheduleHandler(&mockEntityScheduleClient{}).ListPublicHolidays(w,
			httptest.NewRequest(http.MethodGet, "/team-schedule/public-holidays?from=2026-01-01&to=2026-12-31", nil))
		if w.Code != http.StatusUnauthorized {
			b, _ := io.ReadAll(w.Body)
			t.Fatalf("status %d: %s", w.Code, b)
		}
	})
}
