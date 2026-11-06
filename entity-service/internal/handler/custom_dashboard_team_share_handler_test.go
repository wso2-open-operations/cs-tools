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
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

const testShareTeamID = "5e636a12-0000-4000-8000-0000000000bb"

type stubCustomDashboardTeamShareService struct {
	gotDashboardID, gotTeamID string
	shares                    domain.CustomDashboardTeamShareList
	err                       error
}

func (s *stubCustomDashboardTeamShareService) ListTeamShares(_ context.Context, id string) (domain.CustomDashboardTeamShareList, error) {
	s.gotDashboardID = id
	return s.shares, s.err
}
func (s *stubCustomDashboardTeamShareService) AddTeamShare(_ context.Context, id, teamID string) (domain.CustomDashboardTeamShareList, error) {
	s.gotDashboardID, s.gotTeamID = id, teamID
	return s.shares, s.err
}
func (s *stubCustomDashboardTeamShareService) RemoveTeamShare(_ context.Context, id, teamID string) error {
	s.gotDashboardID, s.gotTeamID = id, teamID
	return s.err
}

// serveTeamShare registers the handler with the same patterns as routes.go.
func serveTeamShare(svc *stubCustomDashboardTeamShareService, method, path string) *httptest.ResponseRecorder {
	h := NewCustomDashboardTeamShareHandler(svc)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /users/me/dashboards/{id}/team-shares", h.ListTeamShares)
	mux.HandleFunc("PUT /users/me/dashboards/{id}/team-shares/{teamId}", h.AddTeamShare)
	mux.HandleFunc("DELETE /users/me/dashboards/{id}/team-shares/{teamId}", h.RemoveTeamShare)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

func TestCustomDashboardTeamShareHandler(t *testing.T) {
	shares := domain.CustomDashboardTeamShareList{Shares: []domain.CustomDashboardTeamShare{
		{Team: domain.CustomDashboardTeamShareTeam{ID: testShareTeamID, Name: "Rigel", Type: "cre-abt"}},
	}}

	t.Run("list", func(t *testing.T) {
		svc := &stubCustomDashboardTeamShareService{shares: shares}
		rec := serveTeamShare(svc, http.MethodGet, testShareDashboardPath+"/team-shares")
		if rec.Code != http.StatusOK || svc.gotDashboardID != "5e636a12-0000-4000-8000-000000000001" {
			t.Fatalf("status %d, id %q: %s", rec.Code, svc.gotDashboardID, rec.Body.String())
		}
		var got domain.CustomDashboardTeamShareList
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || len(got.Shares) != 1 || got.Shares[0].Team.Name != "Rigel" || got.Shares[0].Team.Type != "cre-abt" {
			t.Fatalf("body = %s (%v)", rec.Body.String(), err)
		}
	})
	t.Run("add answers the team shares", func(t *testing.T) {
		svc := &stubCustomDashboardTeamShareService{shares: shares}
		rec := serveTeamShare(svc, http.MethodPut, testShareDashboardPath+"/team-shares/"+testShareTeamID)
		if rec.Code != http.StatusOK || svc.gotTeamID != testShareTeamID {
			t.Fatalf("status %d, team %q: %s", rec.Code, svc.gotTeamID, rec.Body.String())
		}
	})
	t.Run("remove is 204", func(t *testing.T) {
		svc := &stubCustomDashboardTeamShareService{}
		rec := serveTeamShare(svc, http.MethodDelete, testShareDashboardPath+"/team-shares/"+testShareTeamID)
		if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 || svc.gotTeamID != testShareTeamID {
			t.Fatalf("status %d, body %q, team %q", rec.Code, rec.Body.String(), svc.gotTeamID)
		}
	})
}

func TestCustomDashboardTeamShareHandler_ServiceErrors(t *testing.T) {
	cases := []struct {
		name, method, path string
		err                error
		want               int
	}{
		{"no token", http.MethodGet, testShareDashboardPath + "/team-shares", &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}, http.StatusUnauthorized},
		{"not the owner's dashboard", http.MethodGet, testShareDashboardPath + "/team-shares", &apierror.NotFoundError{Msg: "dashboard not found"}, http.StatusNotFound},
		{"non-UUID team", http.MethodPut, testShareDashboardPath + "/team-shares/rigel", &apierror.ValidationError{Msg: `team id "rigel" is not a UUID`}, http.StatusBadRequest},
		{"unknown team", http.MethodPut, testShareDashboardPath + "/team-shares/" + testShareTeamID, &apierror.NotFoundError{Msg: "team not found"}, http.StatusNotFound},
		{"not shared", http.MethodDelete, testShareDashboardPath + "/team-shares/" + testShareTeamID, &apierror.NotFoundError{Msg: "dashboard is not shared with this team"}, http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := serveTeamShare(&stubCustomDashboardTeamShareService{err: tc.err}, tc.method, tc.path)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}
