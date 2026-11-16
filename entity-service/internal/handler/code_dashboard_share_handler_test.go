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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

const (
	testCodeShareID   = "5e636a12-0000-4000-8000-000000000001"
	testCodeShareUser = "5e636a12-0000-4000-8000-0000000000aa"
	testCodeShareTeam = "5e636a12-0000-4000-8000-0000000000dd"
)

// stubCodeShareService records which call was made, with which ids.
type stubCodeShareService struct {
	service.CodeDashboardShareService

	called, gotID, gotOther string
	err                     error
}

func (s *stubCodeShareService) record(call, id, other string) {
	s.called, s.gotID, s.gotOther = call, id, other
}
func (s *stubCodeShareService) ListShares(_ context.Context, id string) (domain.CustomDashboardShareList, error) {
	s.record("ListShares", id, "")
	return domain.CustomDashboardShareList{Shares: []domain.CustomDashboardShare{}}, s.err
}
func (s *stubCodeShareService) AddShare(_ context.Context, id, user string) (domain.CustomDashboardShareList, error) {
	s.record("AddShare", id, user)
	return domain.CustomDashboardShareList{Shares: []domain.CustomDashboardShare{}}, s.err
}
func (s *stubCodeShareService) RemoveShare(_ context.Context, id, user string) error {
	s.record("RemoveShare", id, user)
	return s.err
}
func (s *stubCodeShareService) ListTeamShares(_ context.Context, id string) (domain.CustomDashboardTeamShareList, error) {
	s.record("ListTeamShares", id, "")
	return domain.CustomDashboardTeamShareList{Shares: []domain.CustomDashboardTeamShare{}}, s.err
}
func (s *stubCodeShareService) AddTeamShare(_ context.Context, id, team string) (domain.CustomDashboardTeamShareList, error) {
	s.record("AddTeamShare", id, team)
	return domain.CustomDashboardTeamShareList{Shares: []domain.CustomDashboardTeamShare{}}, s.err
}
func (s *stubCodeShareService) RemoveTeamShare(_ context.Context, id, team string) error {
	s.record("RemoveTeamShare", id, team)
	return s.err
}
func (s *stubCodeShareService) GetLink(_ context.Context, id string) (domain.CustomDashboardLink, error) {
	s.record("GetLink", id, "")
	return domain.CustomDashboardLink{}, s.err
}
func (s *stubCodeShareService) EnableLink(_ context.Context, id string) (domain.CustomDashboardLink, error) {
	s.record("EnableLink", id, "")
	return domain.CustomDashboardLink{Enabled: true, LinkID: "l1"}, s.err
}
func (s *stubCodeShareService) DisableLink(_ context.Context, id string) error {
	s.record("DisableLink", id, "")
	return s.err
}
func (s *stubCodeShareService) OpenLink(_ context.Context, req domain.OpenCustomDashboardLinkRequest) (domain.OpenedCustomDashboardLink, error) {
	s.record("OpenLink", "", req.LinkID)
	return domain.OpenedCustomDashboardLink{DashboardID: testCodeShareID}, s.err
}
func (s *stubCodeShareService) ListSharedWithMe(context.Context) (domain.SharedCustomDashboardList, error) {
	s.record("ListSharedWithMe", "", "")
	return domain.SharedCustomDashboardList{Dashboards: []domain.SharedCustomDashboardSummary{{ID: testCodeShareID, Name: "Open Cases"}}}, s.err
}
func (s *stubCodeShareService) GetSharedWithMe(_ context.Context, id string) (domain.SharedCodeDashboard, error) {
	s.record("GetSharedWithMe", id, "")
	return domain.SharedCodeDashboard{ID: id, Name: "Open Cases", Source: "function render() {}"}, s.err
}

// serveCodeShares registers the handler with the same patterns as routes.go.
func serveCodeShares(svc *stubCodeShareService, method, path, body string) *httptest.ResponseRecorder {
	h := NewCodeDashboardShareHandler(svc)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /users/me/code-dashboards/{id}/shares", h.ListShares)
	mux.HandleFunc("PUT /users/me/code-dashboards/{id}/shares/{userId}", h.AddShare)
	mux.HandleFunc("DELETE /users/me/code-dashboards/{id}/shares/{userId}", h.RemoveShare)
	mux.HandleFunc("GET /users/me/code-dashboards/{id}/team-shares", h.ListTeamShares)
	mux.HandleFunc("PUT /users/me/code-dashboards/{id}/team-shares/{teamId}", h.AddTeamShare)
	mux.HandleFunc("DELETE /users/me/code-dashboards/{id}/team-shares/{teamId}", h.RemoveTeamShare)
	mux.HandleFunc("GET /users/me/code-dashboards/{id}/link", h.GetLink)
	mux.HandleFunc("PUT /users/me/code-dashboards/{id}/link", h.EnableLink)
	mux.HandleFunc("DELETE /users/me/code-dashboards/{id}/link", h.DisableLink)
	mux.HandleFunc("GET /users/me/shared-code-dashboards", h.ListSharedWithMe)
	mux.HandleFunc("GET /users/me/shared-code-dashboards/{id}", h.GetSharedWithMe)
	mux.HandleFunc("POST /users/me/shared-code-dashboards/link", h.OpenLink)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
	return rec
}

func TestCodeDashboardShareHandler_Routes(t *testing.T) {
	own := "/users/me/code-dashboards/" + testCodeShareID
	cases := []struct {
		method, path, body  string
		wantCall, wantOther string
		wantStatus          int
	}{
		{http.MethodGet, own + "/shares", "", "ListShares", "", http.StatusOK},
		{http.MethodPut, own + "/shares/" + testCodeShareUser, "", "AddShare", testCodeShareUser, http.StatusOK},
		{http.MethodDelete, own + "/shares/" + testCodeShareUser, "", "RemoveShare", testCodeShareUser, http.StatusNoContent},
		{http.MethodGet, own + "/team-shares", "", "ListTeamShares", "", http.StatusOK},
		{http.MethodPut, own + "/team-shares/" + testCodeShareTeam, "", "AddTeamShare", testCodeShareTeam, http.StatusOK},
		{http.MethodDelete, own + "/team-shares/" + testCodeShareTeam, "", "RemoveTeamShare", testCodeShareTeam, http.StatusNoContent},
		{http.MethodGet, own + "/link", "", "GetLink", "", http.StatusOK},
		{http.MethodPut, own + "/link", "", "EnableLink", "", http.StatusOK},
		{http.MethodDelete, own + "/link", "", "DisableLink", "", http.StatusNoContent},
		{http.MethodGet, "/users/me/shared-code-dashboards/" + testCodeShareID, "", "GetSharedWithMe", "", http.StatusOK},
	}
	for _, c := range cases {
		t.Run(c.method+" "+c.path, func(t *testing.T) {
			svc := &stubCodeShareService{}
			rec := serveCodeShares(svc, c.method, c.path, c.body)
			if rec.Code != c.wantStatus || svc.called != c.wantCall {
				t.Fatalf("status %d, call %q; want %d, %q: %s", rec.Code, svc.called, c.wantStatus, c.wantCall, rec.Body.String())
			}
			if svc.gotID != testCodeShareID || svc.gotOther != c.wantOther {
				t.Errorf("ids = %q, %q; want %q, %q", svc.gotID, svc.gotOther, testCodeShareID, c.wantOther)
			}
		})
	}
}

func TestCodeDashboardShareHandler_RecipientSide(t *testing.T) {
	svc := &stubCodeShareService{}
	rec := serveCodeShares(svc, http.MethodGet, "/users/me/shared-code-dashboards", "")
	if rec.Code != http.StatusOK || svc.called != "ListSharedWithMe" || !strings.Contains(rec.Body.String(), `"Open Cases"`) {
		t.Fatalf("list: %d %q %s", rec.Code, svc.called, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "source") {
		t.Errorf("the list carries a source: %s", rec.Body.String())
	}
	rec = serveCodeShares(svc, http.MethodGet, "/users/me/shared-code-dashboards/"+testCodeShareID, "")
	if !strings.Contains(rec.Body.String(), `"source":"function render() {}"`) {
		t.Errorf("one shared dashboard has no source: %s", rec.Body.String())
	}

	svc = &stubCodeShareService{}
	rec = serveCodeShares(svc, http.MethodPost, "/users/me/shared-code-dashboards/link", `{"linkId":"`+testCodeShareTeam+`"}`)
	if rec.Code != http.StatusOK || svc.called != "OpenLink" || svc.gotOther != testCodeShareTeam {
		t.Fatalf("open link: %d %q %q %s", rec.Code, svc.called, svc.gotOther, rec.Body.String())
	}
	svc = &stubCodeShareService{}
	rec = serveCodeShares(svc, http.MethodPost, "/users/me/shared-code-dashboards/link", `{"linkId":`)
	if rec.Code != http.StatusBadRequest || svc.called != "" {
		t.Errorf("bad body: %d, call %q; want 400 and no call", rec.Code, svc.called)
	}
}

func TestCodeDashboardShareHandler_ServiceErrors(t *testing.T) {
	own := "/users/me/code-dashboards/" + testCodeShareID
	for _, c := range []struct {
		err        error
		method     string
		path       string
		wantStatus int
	}{
		{&apierror.NotFoundError{Msg: "dashboard not found"}, http.MethodGet, own + "/shares", http.StatusNotFound},
		{&apierror.ValidationError{Msg: "dashboards can only be shared with internal staff"}, http.MethodPut, own + "/shares/" + testCodeShareUser, http.StatusBadRequest},
		{&apierror.NotFoundError{Msg: "dashboard is not shared with this team"}, http.MethodDelete, own + "/team-shares/" + testCodeShareTeam, http.StatusNotFound},
		{&apierror.NotFoundError{Msg: "dashboard not found"}, http.MethodDelete, own + "/link", http.StatusNotFound},
		{&apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}, http.MethodGet, "/users/me/shared-code-dashboards", http.StatusUnauthorized},
	} {
		rec := serveCodeShares(&stubCodeShareService{err: c.err}, c.method, c.path, "")
		if rec.Code != c.wantStatus {
			t.Errorf("%s %s with %v: status %d, want %d", c.method, c.path, c.err, rec.Code, c.wantStatus)
		}
	}
}
