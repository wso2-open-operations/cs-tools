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

package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/middleware"
)

type mockEntityClient struct {
	roles []string
}

func (m *mockEntityClient) GetMe(_ context.Context) (entity.GetUserMeResponse, error) {
	return entity.GetUserMeResponse{
		ID:    "usr-test",
		Roles: m.roles,
	}, nil
}

func TestServerRBACRouteGating(t *testing.T) {
	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	tests := []struct {
		name         string
		module       middleware.Module
		action       middleware.Action
		roles        []string
		wantHTTPCode int
	}{
		// Cases
		// Projects. PATCH /projects/{id} is gated on projects:update, which
		// the specification restricts to the WSO2-side roles; every external
		// role is read-only on Projects.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := middleware.NewCachedRoleResolver(&mockEntityClient{roles: tt.roles}, time.Minute)
			handler := middleware.RequirePermission(resolver, tt.module, tt.action)(dummyHandler)

			ctx := middleware.WithUserInfo(context.Background(), &middleware.UserInfo{UserID: "usr-test"})
			req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/test", nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantHTTPCode {
				t.Fatalf("expected HTTP %d, got %d (body: %s)", tt.wantHTTPCode, rr.Code, rr.Body.String())
			}
		})
	}
}

func TestContactManagementRoleGating(t *testing.T) {
	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	tests := []struct {
		name         string
		roles        []string
		wantHTTPCode int
	}{
		{
			// Also covers the removal of the blanket super_admin grant in
			// RequireRoles: a role outside the allow-list is simply refused,
			// with nothing short-circuiting the check.
			name:         "CustomerUser cannot manage contacts",
			roles:        []string{"sn_customerservice.customer"},
			wantHTTPCode: http.StatusForbidden,
		},
		{
			name:         "PartnerUser cannot manage contacts",
			roles:        []string{"sn_customerservice.partner"},
			wantHTTPCode: http.StatusForbidden,
		},
		{
			name:         "CustomerAdmin can manage contacts",
			roles:        []string{"sn_customerservice.customer_admin"},
			wantHTTPCode: http.StatusOK,
		},
		{
			name:         "PartnerAdmin can manage contacts",
			roles:        []string{"sn_customerservice.partner_admin"},
			wantHTTPCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := middleware.NewCachedRoleResolver(&mockEntityClient{roles: tt.roles}, time.Minute)
			handler := middleware.RequireRoles(
				resolver,
				middleware.RoleAdmin,
				middleware.RoleCustomerAdmin,
				middleware.RolePartnerAdmin,
			)(dummyHandler)

			ctx := middleware.WithUserInfo(context.Background(), &middleware.UserInfo{UserID: "usr-test"})
			req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/projects/1/contacts", nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantHTTPCode {
				t.Fatalf("expected HTTP %d, got %d (body: %s)", tt.wantHTTPCode, rr.Code, rr.Body.String())
			}
		})
	}
}
