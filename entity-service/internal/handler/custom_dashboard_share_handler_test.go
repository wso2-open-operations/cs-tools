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
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

const (
	testShareDashboardPath = "/users/me/dashboards/5e636a12-0000-4000-8000-000000000001"
	testShareUserID        = "5e636a12-0000-4000-8000-0000000000aa"
)

type stubCustomDashboardShareService struct {
	service.CustomDashboardShareService

	gotDashboardID, gotUserID string
	shares                    domain.CustomDashboardShareList
	sharedList                domain.SharedCustomDashboardList
	sharedOne                 domain.SharedCustomDashboard
	err                       error
}

func (s *stubCustomDashboardShareService) ListShares(_ context.Context, id string) (domain.CustomDashboardShareList, error) {
	s.gotDashboardID = id
	return s.shares, s.err
}
func (s *stubCustomDashboardShareService) AddShare(_ context.Context, id, userID string) (domain.CustomDashboardShareList, error) {
	s.gotDashboardID, s.gotUserID = id, userID
	return s.shares, s.err
}
func (s *stubCustomDashboardShareService) RemoveShare(_ context.Context, id, userID string) error {
	s.gotDashboardID, s.gotUserID = id, userID
	return s.err
}
func (s *stubCustomDashboardShareService) ListSharedWithMe(context.Context) (domain.SharedCustomDashboardList, error) {
	return s.sharedList, s.err
}
func (s *stubCustomDashboardShareService) GetSharedWithMe(_ context.Context, id string) (domain.SharedCustomDashboard, error) {
	s.gotDashboardID = id
	return s.sharedOne, s.err
}

// shareMux registers the handler with the same patterns as routes.go.
func shareMux(svc *stubCustomDashboardShareService) *http.ServeMux {
	h := NewCustomDashboardShareHandler(svc)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /users/me/dashboards/{id}/shares", h.ListShares)
	mux.HandleFunc("PUT /users/me/dashboards/{id}/shares/{userId}", h.AddShare)
	mux.HandleFunc("DELETE /users/me/dashboards/{id}/shares/{userId}", h.RemoveShare)
	mux.HandleFunc("GET /users/me/shared-dashboards", h.ListSharedWithMe)
	mux.HandleFunc("GET /users/me/shared-dashboards/{id}", h.GetSharedWithMe)
	return mux
}

func serveShare(svc *stubCustomDashboardShareService, method, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	shareMux(svc).ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

func TestCustomDashboardShareHandler_OwnerSide(t *testing.T) {
	alice := "alice"
	shares := domain.CustomDashboardShareList{Shares: []domain.CustomDashboardShare{{User: domain.UserReference{ID: &alice, Name: "Alice Perera"}}}}

	t.Run("list", func(t *testing.T) {
		svc := &stubCustomDashboardShareService{shares: shares}
		rec := serveShare(svc, http.MethodGet, testShareDashboardPath+"/shares")
		if rec.Code != http.StatusOK || svc.gotDashboardID != "5e636a12-0000-4000-8000-000000000001" {
			t.Fatalf("status %d, id %q: %s", rec.Code, svc.gotDashboardID, rec.Body.String())
		}
		var got domain.CustomDashboardShareList
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || len(got.Shares) != 1 || got.Shares[0].User.Name != "Alice Perera" {
			t.Fatalf("body = %s (%v)", rec.Body.String(), err)
		}
	})
	t.Run("add answers the shares", func(t *testing.T) {
		svc := &stubCustomDashboardShareService{shares: shares}
		rec := serveShare(svc, http.MethodPut, testShareDashboardPath+"/shares/"+testShareUserID)
		if rec.Code != http.StatusOK || svc.gotUserID != testShareUserID {
			t.Fatalf("status %d, user %q: %s", rec.Code, svc.gotUserID, rec.Body.String())
		}
	})
	t.Run("remove is 204", func(t *testing.T) {
		svc := &stubCustomDashboardShareService{}
		rec := serveShare(svc, http.MethodDelete, testShareDashboardPath+"/shares/"+testShareUserID)
		if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 || svc.gotUserID != testShareUserID {
			t.Fatalf("status %d, body %q, user %q", rec.Code, rec.Body.String(), svc.gotUserID)
		}
	})
}

// TestCustomDashboardShareHandler_RecipientSide covers the routes for the
// dashboards shared with the caller: the list (with each owner) and one
// dashboard by id.
func TestCustomDashboardShareHandler_RecipientSide(t *testing.T) {
	t.Run("list", func(t *testing.T) {
		svc := &stubCustomDashboardShareService{sharedList: domain.SharedCustomDashboardList{
			Dashboards: []domain.SharedCustomDashboardSummary{{ID: "d1", Name: "SLA", Owner: &domain.UserReference{Name: "Jane Doe"}}},
		}}
		rec := serveShare(svc, http.MethodGet, "/users/me/shared-dashboards")
		var got domain.SharedCustomDashboardList
		if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &got) != nil || len(got.Dashboards) != 1 || got.Dashboards[0].Owner.Name != "Jane Doe" {
			t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
		}
	})
	t.Run("get", func(t *testing.T) {
		svc := &stubCustomDashboardShareService{sharedOne: domain.SharedCustomDashboard{ID: "d1", Definition: "{}"}}
		rec := serveShare(svc, http.MethodGet, "/users/me/shared-dashboards/5e636a12-0000-4000-8000-000000000001")
		if rec.Code != http.StatusOK || svc.gotDashboardID != "5e636a12-0000-4000-8000-000000000001" {
			t.Fatalf("status %d, id %q", rec.Code, svc.gotDashboardID)
		}
	})
}

func TestCustomDashboardShareHandler_ServiceErrors(t *testing.T) {
	cases := []struct {
		name, method, path string
		err                error
		want               int
	}{
		{"no token", http.MethodGet, "/users/me/shared-dashboards", &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}, http.StatusUnauthorized},
		{"not the owner's dashboard", http.MethodGet, testShareDashboardPath + "/shares", &apierror.NotFoundError{Msg: "dashboard not found"}, http.StatusNotFound},
		{"share with yourself", http.MethodPut, testShareDashboardPath + "/shares/" + testShareUserID, &apierror.ValidationError{Msg: "you cannot share a dashboard with yourself"}, http.StatusBadRequest},
		{"not internal staff", http.MethodPut, testShareDashboardPath + "/shares/" + testShareUserID, &apierror.ValidationError{Msg: "dashboards can only be shared with internal staff"}, http.StatusBadRequest},
		{"unknown user", http.MethodPut, testShareDashboardPath + "/shares/" + testShareUserID, &apierror.NotFoundError{Msg: "user not found"}, http.StatusNotFound},
		{"not shared", http.MethodDelete, testShareDashboardPath + "/shares/" + testShareUserID, &apierror.NotFoundError{Msg: "dashboard is not shared with this user"}, http.StatusNotFound},
		{"not shared with me", http.MethodGet, "/users/me/shared-dashboards/5e636a12-0000-4000-8000-000000000001", &apierror.NotFoundError{Msg: "dashboard not found"}, http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := serveShare(&stubCustomDashboardShareService{err: tc.err}, tc.method, tc.path)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}
