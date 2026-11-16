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

const testCodeDashboardPath = "/users/me/code-dashboards/5e636a12-0000-4000-8000-000000000001"

type stubCodeDashboardService struct {
	service.CodeDashboardService

	gotID  string
	gotReq domain.SaveCodeDashboardRequest

	listResp domain.CodeDashboardList
	getResp  domain.CodeDashboard
	saveResp domain.CodeDashboard
	err      error
}

func (s *stubCodeDashboardService) List(context.Context) (domain.CodeDashboardList, error) {
	return s.listResp, s.err
}

func (s *stubCodeDashboardService) Get(_ context.Context, id string) (domain.CodeDashboard, error) {
	s.gotID = id
	return s.getResp, s.err
}

func (s *stubCodeDashboardService) Save(_ context.Context, id string, req domain.SaveCodeDashboardRequest) (domain.CodeDashboard, error) {
	s.gotID = id
	s.gotReq = req
	return s.saveResp, s.err
}

func (s *stubCodeDashboardService) Delete(_ context.Context, id string) error {
	s.gotID = id
	return s.err
}

// serveCodeDashboard registers the handler with the same patterns as
// routes.go, so r.PathValue("id") is filled in as in production.
func serveCodeDashboard(svc *stubCodeDashboardService, method, path, body string) *httptest.ResponseRecorder {
	h := NewCodeDashboardHandler(svc)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /users/me/code-dashboards", h.List)
	mux.HandleFunc("GET /users/me/code-dashboards/{id}", h.Get)
	mux.HandleFunc("PUT /users/me/code-dashboards/{id}", h.Save)
	mux.HandleFunc("DELETE /users/me/code-dashboards/{id}", h.Delete)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
	return rec
}

func TestCodeDashboardHandler_List(t *testing.T) {
	svc := &stubCodeDashboardService{listResp: domain.CodeDashboardList{
		Dashboards: []domain.CodeDashboardSummary{{ID: "d1", Name: "Open Cases"}},
	}}
	rec := serveCodeDashboard(svc, http.MethodGet, "/users/me/code-dashboards", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var got domain.CodeDashboardList
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Dashboards) != 1 || got.Dashboards[0].Name != "Open Cases" {
		t.Fatalf("body = %+v", got)
	}
	if strings.Contains(rec.Body.String(), "source") {
		t.Errorf("list body carries a source: %s", rec.Body.String())
	}
}

func TestCodeDashboardHandler_Get(t *testing.T) {
	svc := &stubCodeDashboardService{getResp: domain.CodeDashboard{ID: "d1", Name: "Open Cases", Source: "function render() {}"}}
	rec := serveCodeDashboard(svc, http.MethodGet, testCodeDashboardPath, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if svc.gotID != "5e636a12-0000-4000-8000-000000000001" {
		t.Errorf("id = %q, want the path's id", svc.gotID)
	}
	var got domain.CodeDashboard
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Source != "function render() {}" {
		t.Errorf("source = %q", got.Source)
	}
}

func TestCodeDashboardHandler_Save(t *testing.T) {
	svc := &stubCodeDashboardService{saveResp: domain.CodeDashboard{ID: "d1", Name: "Open Cases"}}
	rec := serveCodeDashboard(svc, http.MethodPut, testCodeDashboardPath, `{"name":"Open Cases","source":"function render() {}"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if svc.gotID != "5e636a12-0000-4000-8000-000000000001" {
		t.Errorf("id = %q, want the path's id", svc.gotID)
	}
	if svc.gotReq.Name != "Open Cases" || svc.gotReq.Source != "function render() {}" {
		t.Errorf("request = %+v", svc.gotReq)
	}
}

func TestCodeDashboardHandler_Save_RejectsBadBody(t *testing.T) {
	cases := map[string]string{
		"unknown field":     `{"name":"X","source":"x","ownerUserId":"someone"}`,
		"a JSON definition": `{"name":"X","definition":"{}"}`,
		"source not string": `{"name":"X","source":{"code":"x"}}`,
		"not JSON":          `name=X`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			svc := &stubCodeDashboardService{}
			rec := serveCodeDashboard(svc, http.MethodPut, testCodeDashboardPath, body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
			}
			if svc.gotID != "" {
				t.Error("the service was called for a bad body")
			}
		})
	}
}

func TestCodeDashboardHandler_Delete(t *testing.T) {
	svc := &stubCodeDashboardService{}
	rec := serveCodeDashboard(svc, http.MethodDelete, testCodeDashboardPath, "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204: %s", rec.Code, rec.Body.String())
	}
	if svc.gotID != "5e636a12-0000-4000-8000-000000000001" {
		t.Errorf("id = %q, want the path's id", svc.gotID)
	}
}

func TestCodeDashboardHandler_ServiceErrors(t *testing.T) {
	cases := []struct {
		name   string
		method string
		path   string
		body   string
		err    error
		want   int
	}{
		{"no token", http.MethodGet, "/users/me/code-dashboards", "", &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}, http.StatusUnauthorized},
		{"bad id", http.MethodGet, "/users/me/code-dashboards/abc", "", &apierror.ValidationError{Msg: `dashboard id "abc" is not a UUID`}, http.StatusBadRequest},
		{"get missing", http.MethodGet, testCodeDashboardPath, "", &apierror.NotFoundError{Msg: "dashboard not found"}, http.StatusNotFound},
		{"save name taken", http.MethodPut, testCodeDashboardPath, `{"name":"X","source":"x"}`, &apierror.ConflictError{Msg: "you already have an advanced dashboard named: X"}, http.StatusConflict},
		{"save over limit", http.MethodPut, testCodeDashboardPath, `{"name":"X","source":"x"}`, &apierror.ValidationError{Msg: "at most 50 saved advanced dashboards are allowed"}, http.StatusBadRequest},
		{"delete missing", http.MethodDelete, testCodeDashboardPath, "", &apierror.NotFoundError{Msg: "dashboard not found"}, http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := serveCodeDashboard(&stubCodeDashboardService{err: tc.err}, tc.method, tc.path, tc.body)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}
