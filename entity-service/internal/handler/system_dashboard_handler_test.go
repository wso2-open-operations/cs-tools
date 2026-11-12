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

type stubSystemDashboardService struct {
	service.SystemDashboardService

	gotKey string
	gotReq domain.SaveSystemDashboardRequest

	listResp domain.SystemDashboardList
	getResp  domain.SystemDashboard
	saveResp domain.SystemDashboard
	err      error
}

func (s *stubSystemDashboardService) List(context.Context) (domain.SystemDashboardList, error) {
	return s.listResp, s.err
}

func (s *stubSystemDashboardService) Get(_ context.Context, key string) (domain.SystemDashboard, error) {
	s.gotKey = key
	return s.getResp, s.err
}

func (s *stubSystemDashboardService) Save(_ context.Context, key string, req domain.SaveSystemDashboardRequest) (domain.SystemDashboard, error) {
	s.gotKey = key
	s.gotReq = req
	return s.saveResp, s.err
}

// serveSystemDashboard registers the handler with the same patterns as
// routes.go, so r.PathValue("key") is filled in as in production.
func serveSystemDashboard(svc *stubSystemDashboardService, method, path, body string) *httptest.ResponseRecorder {
	h := NewSystemDashboardHandler(svc)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /system-dashboards", h.List)
	mux.HandleFunc("GET /system-dashboards/{key}", h.Get)
	mux.HandleFunc("PUT /system-dashboards/{key}", h.Save)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
	return rec
}

func TestSystemDashboardHandler_List(t *testing.T) {
	svc := &stubSystemDashboardService{listResp: domain.SystemDashboardList{
		Dashboards: []domain.SystemDashboard{{ID: "d1", SystemKey: "overview.json", IsSystem: true, Name: "Overview", Definition: `{}`}},
	}}
	rec := serveSystemDashboard(svc, http.MethodGet, "/system-dashboards", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var got domain.SystemDashboardList
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Dashboards) != 1 || got.Dashboards[0].SystemKey != "overview.json" || !got.Dashboards[0].IsSystem {
		t.Fatalf("body = %+v", got)
	}
	for _, field := range []string{`"systemKey"`, `"isSystem"`, `"definition"`} {
		if !strings.Contains(rec.Body.String(), field) {
			t.Errorf("body lacks %s: %s", field, rec.Body.String())
		}
	}
}

func TestSystemDashboardHandler_Get(t *testing.T) {
	svc := &stubSystemDashboardService{getResp: domain.SystemDashboard{ID: "d1", SystemKey: "overview.json", Name: "Overview"}}
	rec := serveSystemDashboard(svc, http.MethodGet, "/system-dashboards/overview.json", "")
	if rec.Code != http.StatusOK || svc.gotKey != "overview.json" {
		t.Fatalf("status = %d key = %q: %s", rec.Code, svc.gotKey, rec.Body.String())
	}
	svc.err = &apierror.NotFoundError{Msg: "system dashboard not found"}
	if rec := serveSystemDashboard(svc, http.MethodGet, "/system-dashboards/x", ""); rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestSystemDashboardHandler_Save(t *testing.T) {
	svc := &stubSystemDashboardService{saveResp: domain.SystemDashboard{ID: "d1", SystemKey: "overview.json", Name: "Overview"}}
	rec := serveSystemDashboard(svc, http.MethodPut, "/system-dashboards/overview.json", `{"name":"Overview","definition":"{\"widgets\":[]}"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if svc.gotKey != "overview.json" || svc.gotReq.Name != "Overview" || svc.gotReq.Definition != `{"widgets":[]}` {
		t.Errorf("key = %q request = %+v", svc.gotKey, svc.gotReq)
	}
}

func TestSystemDashboardHandler_SaveErrors(t *testing.T) {
	svc := &stubSystemDashboardService{}
	for name, body := range map[string]string{
		"unknown field": `{"name":"a","definition":"{}","isSystem":false}`,
		"not json":      `nope`,
	} {
		if rec := serveSystemDashboard(svc, http.MethodPut, "/system-dashboards/overview.json", body); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", name, rec.Code)
		}
	}
	svc.err = &apierror.ForbiddenError{Msg: "not the backend"}
	if rec := serveSystemDashboard(svc, http.MethodPut, "/system-dashboards/overview.json", `{"name":"a","definition":"{}"}`); rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}
