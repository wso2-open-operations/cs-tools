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
		{
			name:         "Stakeholder cannot create cases",
			module:       middleware.ModuleCases,
			action:       middleware.ActionCreate,
			roles:        []string{"sn_customerservice.stakeholder"},
			wantHTTPCode: http.StatusForbidden,
		},
		{
			name:         "CustomerUser can create cases",
			module:       middleware.ModuleCases,
			action:       middleware.ActionCreate,
			roles:        []string{"sn_customerservice.customer"},
			wantHTTPCode: http.StatusOK,
		},
		{
			name:         "Agent can create cases",
			module:       middleware.ModuleCases,
			action:       middleware.ActionCreate,
			roles:        []string{"wso2_agent"},
			wantHTTPCode: http.StatusOK,
		},
		{
			name:         "SuperAdmin can delete cases",
			module:       middleware.ModuleCases,
			action:       middleware.ActionDelete,
			roles:        []string{"super_admin"},
			wantHTTPCode: http.StatusOK,
		},
		{
			name:         "Agent cannot delete cases",
			module:       middleware.ModuleCases,
			action:       middleware.ActionDelete,
			roles:        []string{"wso2_agent"},
			wantHTTPCode: http.StatusForbidden,
		},

		// Time Cards
		{
			name:         "Stakeholder cannot read time cards",
			module:       middleware.ModuleTimeCards,
			action:       middleware.ActionRead,
			roles:        []string{"sn_customerservice.stakeholder"},
			wantHTTPCode: http.StatusForbidden,
		},
		{
			name:         "CustomerUser can read time cards",
			module:       middleware.ModuleTimeCards,
			action:       middleware.ActionRead,
			roles:        []string{"sn_customerservice.customer"},
			wantHTTPCode: http.StatusOK,
		},
		{
			name:         "CustomerUser cannot create time cards",
			module:       middleware.ModuleTimeCards,
			action:       middleware.ActionCreate,
			roles:        []string{"sn_customerservice.customer"},
			wantHTTPCode: http.StatusForbidden,
		},
		{
			name:         "Agent can create time cards",
			module:       middleware.ModuleTimeCards,
			action:       middleware.ActionCreate,
			roles:        []string{"wso2_agent"},
			wantHTTPCode: http.StatusOK,
		},

		// Change Requests
		{
			name:         "Stakeholder cannot read change requests",
			module:       middleware.ModuleChangeRequests,
			action:       middleware.ActionRead,
			roles:        []string{"sn_customerservice.stakeholder"},
			wantHTTPCode: http.StatusForbidden,
		},
		{
			name:         "CustomerUser can read change requests",
			module:       middleware.ModuleChangeRequests,
			action:       middleware.ActionRead,
			roles:        []string{"sn_customerservice.customer"},
			wantHTTPCode: http.StatusOK,
		},
		{
			name:         "CustomerUser cannot create change requests",
			module:       middleware.ModuleChangeRequests,
			action:       middleware.ActionCreate,
			roles:        []string{"sn_customerservice.customer"},
			wantHTTPCode: http.StatusForbidden,
		},
		{
			name:         "Agent can create change requests",
			module:       middleware.ModuleChangeRequests,
			action:       middleware.ActionCreate,
			roles:        []string{"wso2_agent"},
			wantHTTPCode: http.StatusOK,
		},

		// Security Admin
		{
			name:         "Admin cannot access Security Admin",
			module:       middleware.ModuleSecurityAdmin,
			action:       middleware.ActionRead,
			roles:        []string{"sn_customerservice.admin"},
			wantHTTPCode: http.StatusForbidden,
		},
		{
			name:         "Agent cannot access Security Admin",
			module:       middleware.ModuleSecurityAdmin,
			action:       middleware.ActionRead,
			roles:        []string{"wso2_agent"},
			wantHTTPCode: http.StatusForbidden,
		},
		{
			name:         "CustomerAdmin cannot access Security Admin",
			module:       middleware.ModuleSecurityAdmin,
			action:       middleware.ActionRead,
			roles:        []string{"sn_customerservice.customer_admin"},
			wantHTTPCode: http.StatusForbidden,
		},
		{
			name:         "Stakeholder cannot access Security Admin",
			module:       middleware.ModuleSecurityAdmin,
			action:       middleware.ActionRead,
			roles:        []string{"sn_customerservice.stakeholder"},
			wantHTTPCode: http.StatusForbidden,
		},
		{
			name:         "SuperAdmin can access Security Admin",
			module:       middleware.ModuleSecurityAdmin,
			action:       middleware.ActionRead,
			roles:        []string{"sn_customerservice.super_admin"},
			wantHTTPCode: http.StatusOK,
		},
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
		{
			name:         "SuperAdmin can manage contacts",
			roles:        []string{"super_admin"},
			wantHTTPCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := middleware.NewCachedRoleResolver(&mockEntityClient{roles: tt.roles}, time.Minute)
			handler := middleware.RequireRoles(
				resolver,
				middleware.RoleSuperAdmin,
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
