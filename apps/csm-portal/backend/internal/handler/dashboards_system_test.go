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
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/dashboard"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/entity"
)

// failingSystemStore fails every read of the stored dashboards.
type failingSystemStore struct{ err error }

func (f failingSystemStore) ListSystemDashboards(context.Context) ([]entity.SystemDashboard, error) {
	return nil, f.err
}

func (f failingSystemStore) SaveSystemDashboard(context.Context, string, string, string) (entity.SystemDashboard, error) {
	return entity.SystemDashboard{}, f.err
}

// TestDashboards_DatabaseReadFailure: a failed read of the stored dashboards
// is an error response, never an empty list or a 404, and an upstream 4xx
// (this backend's own credential being rejected) is not echoed as the
// caller's own 401/403/404.
func TestDashboards_DatabaseReadFailure(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"upstream 503", &apierror.Error{StatusCode: http.StatusServiceUnavailable}, http.StatusServiceUnavailable},
		{"upstream 500", &apierror.Error{StatusCode: http.StatusInternalServerError}, http.StatusInternalServerError},
		{"transport error", errors.New("connection refused"), http.StatusInternalServerError},
		{"upstream 401 is not the caller's", &apierror.Error{StatusCode: http.StatusUnauthorized}, http.StatusServiceUnavailable},
		{"upstream 403 is not the caller's", &apierror.Error{StatusCode: http.StatusForbidden}, http.StatusServiceUnavailable},
		{"upstream 404 is not a missing dashboard", &apierror.Error{StatusCode: http.StatusNotFound}, http.StatusServiceUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			previous := dashboard.Active()
			t.Cleanup(func() { dashboard.SetActive(previous) })
			reg, err := dashboard.NewSystemRegistry(failingSystemStore{err: tc.err}, nil)
			if err != nil {
				t.Fatal(err)
			}
			dashboard.SetActive(reg)

			h := NewDashboardHandler(NewAccessGuard(testAccessConfig()))

			w := httptest.NewRecorder()
			h.GetDashboards(w, withUser(httptest.NewRequest(http.MethodGet, "/dashboards", nil)))
			assertStatus(t, w, tc.want)

			r := withUser(httptest.NewRequest(http.MethodGet, "/dashboards/some-id", nil))
			r.SetPathValue("dashboardId", "some-id")
			w = httptest.NewRecorder()
			h.GetDashboardDetail(w, r)
			assertStatus(t, w, tc.want)
		})
	}
}
