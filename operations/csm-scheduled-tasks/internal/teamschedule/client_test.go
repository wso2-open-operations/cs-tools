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

package teamschedule

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/apierror"
)

// The rota read always asks entity-service to leave out people who are away:
// that one field is what stops a reminder reaching somebody on leave.
func TestWorkingAssignments_AlwaysAsksToLeaveOutPeopleWhoAreAway(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/team-schedule/assignments/search" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decode body: %v", err)
		}
		_, _ = w.Write([]byte(`{"assignments":[{"id":"a1","engineer":{"userId":"u1","name":"Ann","email":"ann@example.com"},
			"teamKey":"vega","shiftCode":"CRE_MORNING","rotaDate":"2026-10-12",
			"startsAt":"2026-10-12T00:30:00Z","endsAt":"2026-10-12T03:30:00Z","isOnCall":false,"note":"swap"}],"count":1}`))
	}))
	defer srv.Close()

	rows, err := newClient(srv.Client(), srv.URL+"/").WorkingAssignments(context.Background(), "2026-10-12", "2026-10-12", "CRE")
	if err != nil {
		t.Fatalf("WorkingAssignments: %v", err)
	}
	if got["excludeAbsent"] != true || got["family"] != "CRE" || got["from"] != "2026-10-12" || got["to"] != "2026-10-12" {
		t.Fatalf("request body = %v", got)
	}
	if len(rows) != 1 || rows[0].Engineer.Email != "ann@example.com" || rows[0].ShiftCode != "CRE_MORNING" ||
		rows[0].RotaDate != "2026-10-12" || rows[0].Note != "swap" || rows[0].StartsAt.Hour() != 0 {
		t.Fatalf("rows = %+v", rows)
	}
}

// An entity-service that predates excludeAbsent refuses the unknown field with
// a 400; that must surface as an error, not as an empty rota.
func TestWorkingAssignments_ARefusalIsAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"code":400,"message":"json: unknown field \"excludeAbsent\""}`))
	}))
	defer srv.Close()

	_, err := newClient(srv.Client(), srv.URL).WorkingAssignments(context.Background(), "2026-10-12", "2026-10-12", "CRE")
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("err = %v, want an *apierror.Error with status 400", err)
	}
}

func TestCatalogue_DecodesShiftsAndTeams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/team-schedule/catalogue" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"shifts":[{"code":"CRE_MORNING","label":"Morning 6-9am","family":"CRE",
			"authoringTimeZone":"Asia/Colombo","sortOrder":10}],"teams":[{"key":"vega","name":"Vega"}],"zones":[]}`))
	}))
	defer srv.Close()

	cat, err := newClient(srv.Client(), srv.URL).Catalogue(context.Background())
	if err != nil {
		t.Fatalf("Catalogue: %v", err)
	}
	if len(cat.Shifts) != 1 || cat.Shifts[0].Label != "Morning 6-9am" || cat.Shifts[0].AuthoringTimeZone != "Asia/Colombo" ||
		len(cat.Teams) != 1 || cat.Teams[0].Name != "Vega" {
		t.Fatalf("catalogue = %+v", cat)
	}
}
