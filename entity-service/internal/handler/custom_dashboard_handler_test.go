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
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

const testDashboardPath = "/users/me/dashboards/5e636a12-0000-4000-8000-000000000001"

type stubCustomDashboardService struct {
	service.CustomDashboardService

	gotID  string
	gotReq domain.SaveCustomDashboardRequest

	listResp domain.CustomDashboardList
	getResp  domain.CustomDashboard
	saveResp domain.CustomDashboard
	err      error
}

func (s *stubCustomDashboardService) List(context.Context) (domain.CustomDashboardList, error) {
	return s.listResp, s.err
}

func (s *stubCustomDashboardService) Get(_ context.Context, id string) (domain.CustomDashboard, error) {
	s.gotID = id
	return s.getResp, s.err
}

func (s *stubCustomDashboardService) Save(_ context.Context, id string, req domain.SaveCustomDashboardRequest) (domain.CustomDashboard, error) {
	s.gotID = id
	s.gotReq = req
	return s.saveResp, s.err
}

func (s *stubCustomDashboardService) Delete(_ context.Context, id string) error {
	s.gotID = id
	return s.err
}

// customDashboardMux registers the handler with the same patterns as routes.go,
// so r.PathValue("id") is filled in as in production.
func customDashboardMux(svc *stubCustomDashboardService) *http.ServeMux {
	h := NewCustomDashboardHandler(svc)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /users/me/dashboards", h.List)
	mux.HandleFunc("GET /users/me/dashboards/{id}", h.Get)
	mux.HandleFunc("PUT /users/me/dashboards/{id}", h.Save)
	mux.HandleFunc("DELETE /users/me/dashboards/{id}", h.Delete)
	return mux
}

func serveCustomDashboard(svc *stubCustomDashboardService, method, path, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	customDashboardMux(svc).ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
	return rec
}

func TestCustomDashboardHandler_List(t *testing.T) {
	svc := &stubCustomDashboardService{listResp: domain.CustomDashboardList{
		Dashboards: []domain.CustomDashboardSummary{{ID: "d1", Name: "My Cases"}},
	}}
	rec := serveCustomDashboard(svc, http.MethodGet, "/users/me/dashboards", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var got domain.CustomDashboardList
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Dashboards) != 1 || got.Dashboards[0].Name != "My Cases" {
		t.Fatalf("body = %+v", got)
	}
	if strings.Contains(rec.Body.String(), "definition") {
		t.Errorf("list body carries a definition: %s", rec.Body.String())
	}
}

func TestCustomDashboardHandler_Get(t *testing.T) {
	svc := &stubCustomDashboardService{getResp: domain.CustomDashboard{ID: "d1", Name: "My Cases", Definition: `{"widgets":[]}`}}
	rec := serveCustomDashboard(svc, http.MethodGet, testDashboardPath, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if svc.gotID != "5e636a12-0000-4000-8000-000000000001" {
		t.Errorf("id = %q, want the path's id", svc.gotID)
	}
	var got domain.CustomDashboard
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Definition != `{"widgets":[]}` {
		t.Errorf("definition = %q", got.Definition)
	}
}

func TestCustomDashboardHandler_Save(t *testing.T) {
	svc := &stubCustomDashboardService{saveResp: domain.CustomDashboard{ID: "d1", Name: "My Cases"}}
	rec := serveCustomDashboard(svc, http.MethodPut, testDashboardPath, `{"name":"My Cases","definition":"{\"widgets\":[]}"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if svc.gotID != "5e636a12-0000-4000-8000-000000000001" {
		t.Errorf("id = %q, want the path's id", svc.gotID)
	}
	if svc.gotReq.Name != "My Cases" || svc.gotReq.Definition != `{"widgets":[]}` {
		t.Errorf("request = %+v", svc.gotReq)
	}
}

func TestCustomDashboardHandler_Save_RejectsBadBody(t *testing.T) {
	cases := map[string]string{
		"unknown field":         `{"name":"X","definition":"{}","ownerUserId":"someone"}`,
		"definition not string": `{"name":"X","definition":{"widgets":[]}}`,
		"not JSON":              `name=X`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			svc := &stubCustomDashboardService{}
			rec := serveCustomDashboard(svc, http.MethodPut, testDashboardPath, body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
			}
			if svc.gotID != "" {
				t.Error("the service was called for a bad body")
			}
		})
	}
}

func TestCustomDashboardHandler_Delete(t *testing.T) {
	svc := &stubCustomDashboardService{}
	rec := serveCustomDashboard(svc, http.MethodDelete, testDashboardPath, "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204: %s", rec.Code, rec.Body.String())
	}
	if svc.gotID != "5e636a12-0000-4000-8000-000000000001" {
		t.Errorf("id = %q, want the path's id", svc.gotID)
	}
}

func TestCustomDashboardHandler_ServiceErrors(t *testing.T) {
	cases := []struct {
		name   string
		method string
		path   string
		body   string
		err    error
		want   int
	}{
		{"no token", http.MethodGet, "/users/me/dashboards", "", &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}, http.StatusUnauthorized},
		{"bad id", http.MethodGet, "/users/me/dashboards/abc", "", &apierror.ValidationError{Msg: `dashboard id "abc" is not a UUID`}, http.StatusBadRequest},
		{"get missing", http.MethodGet, testDashboardPath, "", &apierror.NotFoundError{Msg: "dashboard not found"}, http.StatusNotFound},
		{"save name taken", http.MethodPut, testDashboardPath, `{"name":"X","definition":"{}"}`, &apierror.ConflictError{Msg: "you already have a dashboard named: X"}, http.StatusConflict},
		{"save over limit", http.MethodPut, testDashboardPath, `{"name":"X","definition":"{}"}`, &apierror.ValidationError{Msg: "at most 50 saved dashboards are allowed"}, http.StatusBadRequest},
		{"delete missing", http.MethodDelete, testDashboardPath, "", &apierror.NotFoundError{Msg: "dashboard not found"}, http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := serveCustomDashboard(&stubCustomDashboardService{err: tc.err}, tc.method, tc.path, tc.body)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}

func TestCustomDashboardRoutes_PostIsMethodNotAllowed(t *testing.T) {
	rec := serveCustomDashboard(&stubCustomDashboardService{}, http.MethodPost, "/users/me/dashboards", `{}`)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST = %d, want 405", rec.Code)
	}
}
