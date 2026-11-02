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

const (
	testLinkID       = "5e636a12-0000-4000-8000-0000000000cc"
	testOpenLinkPath = "/users/me/shared-dashboards/link"
)

type stubCustomDashboardLinkService struct {
	called         bool
	gotDashboardID string
	gotOpen        domain.OpenCustomDashboardLinkRequest
	link           domain.CustomDashboardLink
	opened         domain.OpenedCustomDashboardLink
	err            error
}

func (s *stubCustomDashboardLinkService) GetLink(_ context.Context, id string) (domain.CustomDashboardLink, error) {
	s.called, s.gotDashboardID = true, id
	return s.link, s.err
}
func (s *stubCustomDashboardLinkService) EnableLink(_ context.Context, id string) (domain.CustomDashboardLink, error) {
	s.called, s.gotDashboardID = true, id
	return s.link, s.err
}
func (s *stubCustomDashboardLinkService) DisableLink(_ context.Context, id string) error {
	s.called, s.gotDashboardID = true, id
	return s.err
}
func (s *stubCustomDashboardLinkService) OpenLink(_ context.Context, req domain.OpenCustomDashboardLinkRequest) (domain.OpenedCustomDashboardLink, error) {
	s.called, s.gotOpen = true, req
	return s.opened, s.err
}

// serveLink registers the handler with the same patterns as routes.go.
func serveLink(svc *stubCustomDashboardLinkService, method, path, body string) *httptest.ResponseRecorder {
	h := NewCustomDashboardLinkHandler(svc)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /users/me/dashboards/{id}/link", h.GetLink)
	mux.HandleFunc("PUT /users/me/dashboards/{id}/link", h.EnableLink)
	mux.HandleFunc("DELETE /users/me/dashboards/{id}/link", h.DisableLink)
	mux.HandleFunc("POST /users/me/shared-dashboards/link", h.OpenLink)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
	return rec
}

func TestCustomDashboardLinkHandler(t *testing.T) {
	link := domain.CustomDashboardLink{Enabled: true, LinkID: testLinkID, OpenedBy: 3}

	t.Run("get", func(t *testing.T) {
		svc := &stubCustomDashboardLinkService{link: link}
		rec := serveLink(svc, http.MethodGet, testShareDashboardPath+"/link", "")
		if rec.Code != http.StatusOK || svc.gotDashboardID != "5e636a12-0000-4000-8000-000000000001" {
			t.Fatalf("status %d, id %q: %s", rec.Code, svc.gotDashboardID, rec.Body.String())
		}
		var got domain.CustomDashboardLink
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || got != link {
			t.Fatalf("body = %s (%v)", rec.Body.String(), err)
		}
	})
	t.Run("a link that is off has no linkId", func(t *testing.T) {
		svc := &stubCustomDashboardLinkService{link: domain.CustomDashboardLink{}}
		rec := serveLink(svc, http.MethodGet, testShareDashboardPath+"/link", "")
		if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `{"enabled":false,"openedBy":0}` {
			t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
		}
	})
	t.Run("enable answers the link", func(t *testing.T) {
		svc := &stubCustomDashboardLinkService{link: link}
		rec := serveLink(svc, http.MethodPut, testShareDashboardPath+"/link", "")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), testLinkID) {
			t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
		}
	})
	t.Run("disable is 204", func(t *testing.T) {
		svc := &stubCustomDashboardLinkService{}
		rec := serveLink(svc, http.MethodDelete, testShareDashboardPath+"/link", "")
		if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 || !svc.called {
			t.Fatalf("status %d, body %q, called %v", rec.Code, rec.Body.String(), svc.called)
		}
	})
	t.Run("open takes the link id from the body", func(t *testing.T) {
		svc := &stubCustomDashboardLinkService{opened: domain.OpenedCustomDashboardLink{DashboardID: "5e636a12-0000-4000-8000-000000000001"}}
		rec := serveLink(svc, http.MethodPost, testOpenLinkPath, `{"linkId":"`+testLinkID+`"}`)
		if rec.Code != http.StatusOK || svc.gotOpen.LinkID != testLinkID {
			t.Fatalf("status %d, link %q: %s", rec.Code, svc.gotOpen.LinkID, rec.Body.String())
		}
		if strings.TrimSpace(rec.Body.String()) != `{"dashboardId":"5e636a12-0000-4000-8000-000000000001","own":false}` {
			t.Fatalf("body = %s", rec.Body.String())
		}
	})
	t.Run("open refuses a body it cannot read", func(t *testing.T) {
		for _, body := range []string{"", "{", `{"linkId":"x","dashboardId":"y"}`} {
			svc := &stubCustomDashboardLinkService{}
			rec := serveLink(svc, http.MethodPost, testOpenLinkPath, body)
			if rec.Code != http.StatusBadRequest || svc.called {
				t.Errorf("body %q: status %d, called %v", body, rec.Code, svc.called)
			}
		}
	})
}

func TestCustomDashboardLinkHandler_ServiceErrors(t *testing.T) {
	openBody := `{"linkId":"` + testLinkID + `"}`
	cases := []struct {
		name, method, path, body string
		err                      error
		want                     int
	}{
		{"no token", http.MethodGet, testShareDashboardPath + "/link", "", &apierror.UnauthorizedError{Msg: "x-user-id-token header is required"}, http.StatusUnauthorized},
		{"not the owner's dashboard", http.MethodPut, testShareDashboardPath + "/link", "", &apierror.NotFoundError{Msg: "dashboard not found"}, http.StatusNotFound},
		{"non-UUID dashboard", http.MethodDelete, "/users/me/dashboards/abc/link", "", &apierror.ValidationError{Msg: `dashboard id "abc" is not a UUID`}, http.StatusBadRequest},
		{"non-UUID link", http.MethodPost, testOpenLinkPath, `{"linkId":"abc"}`, &apierror.ValidationError{Msg: "linkId is not a UUID"}, http.StatusBadRequest},
		{"link off or unknown", http.MethodPost, testOpenLinkPath, openBody, &apierror.NotFoundError{Msg: "this link does not open a dashboard; it may have been turned off"}, http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := serveLink(&stubCustomDashboardLinkService{err: tc.err}, tc.method, tc.path, tc.body)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}
