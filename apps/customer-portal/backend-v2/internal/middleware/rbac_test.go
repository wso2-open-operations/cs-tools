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

package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

type mockEntityUserClient struct {
	getMeFn func(ctx context.Context) (entity.GetUserMeResponse, error)
}

func (m *mockEntityUserClient) GetMe(ctx context.Context) (entity.GetUserMeResponse, error) {
	return m.getMeFn(ctx)
}

type mockRoleResolver struct {
	roles []CanonicalRole
	err   error
}

func (m *mockRoleResolver) GetRoles(_ context.Context) ([]CanonicalRole, error) {
	return m.roles, m.err
}

func TestNormalizeRoles(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []CanonicalRole
	}{
		{
			name:     "empty input",
			input:    nil,
			expected: nil,
		},
		{
			name: "normalizes known ServiceNow wire roles",
			input: []string{
				"sn_customerservice.super_admin",
				"sn_customerservice.admin",
				"wso2_agent",
				"sn_customerservice.customer_admin",
				"sn_customerservice.customer",
				"sn_customerservice.partner_admin",
				"sn_customerservice.partner",
				"sn_customerservice.stakeholder",
			},
			expected: []CanonicalRole{
				RoleSuperAdmin,
				RoleAdmin,
				RoleAgent,
				RoleCustomerAdmin,
				RoleCustomerUser,
				RolePartnerAdmin,
				RolePartnerUser,
				RoleStakeholder,
			},
		},
		{
			name: "deduplicates normalized roles",
			input: []string{
				"sn_customerservice.admin",
				"admin",
				"wso2_agent",
				"agent",
			},
			expected: []CanonicalRole{
				RoleAdmin,
				RoleAgent,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeRoles(tt.input)
			if len(got) != len(tt.expected) {
				t.Fatalf("NormalizeRoles() got %d roles, want %d", len(got), len(tt.expected))
			}
			for i, r := range got {
				if r != tt.expected[i] {
					t.Errorf("NormalizeRoles()[%d] = %v, want %v", i, r, tt.expected[i])
				}
			}
		})
	}
}

func TestHasPermissionMatrix(t *testing.T) {
	allModules := []Module{
		ModuleCases,
		ModuleTimeCards,
		ModuleProjects,
		ModuleChangeRequests,
		ModuleDeployments,
		ModuleDeploymentProducts,
		ModuleDeploymentResources,
		ModuleSecurityAdmin,
	}
	allActions := []Action{ActionCreate, ActionRead, ActionUpdate, ActionDelete}

	t.Run("SuperAdmin has full CRUD on all 8 modules", func(t *testing.T) {
		superAdmin := []CanonicalRole{RoleSuperAdmin}
		for _, mod := range allModules {
			for _, act := range allActions {
				if !HasPermission(superAdmin, mod, act) {
					t.Errorf("SuperAdmin must have %s on %s", act, mod)
				}
			}
		}
	})

	t.Run("Admin has full CRUD on 7 modules and zero access to SecurityAdmin", func(t *testing.T) {
		admin := []CanonicalRole{RoleAdmin}
		for _, mod := range allModules {
			if mod == ModuleSecurityAdmin {
				for _, act := range allActions {
					if HasPermission(admin, mod, act) {
						t.Errorf("Admin must NOT have %s on %s", act, mod)
					}
				}
			} else {
				for _, act := range allActions {
					if !HasPermission(admin, mod, act) {
						t.Errorf("Admin must have %s on %s", act, mod)
					}
				}
			}
		}
	})

	t.Run("Agent permissions", func(t *testing.T) {
		agent := []CanonicalRole{RoleAgent}

		// Cases: CRU (no D)
		if !HasPermission(agent, ModuleCases, ActionCreate) || !HasPermission(agent, ModuleCases, ActionRead) || !HasPermission(agent, ModuleCases, ActionUpdate) {
			t.Error("Agent must have CRU on Cases")
		}
		if HasPermission(agent, ModuleCases, ActionDelete) {
			t.Error("Agent must NOT have Delete on Cases")
		}

		// Time Cards: CRU (no D)
		if !HasPermission(agent, ModuleTimeCards, ActionCreate) || !HasPermission(agent, ModuleTimeCards, ActionRead) || !HasPermission(agent, ModuleTimeCards, ActionUpdate) {
			t.Error("Agent must have CRU on TimeCards")
		}
		if HasPermission(agent, ModuleTimeCards, ActionDelete) {
			t.Error("Agent must NOT have Delete on TimeCards")
		}

		// Projects: RU (no C, no D)
		if HasPermission(agent, ModuleProjects, ActionCreate) || HasPermission(agent, ModuleProjects, ActionDelete) {
			t.Error("Agent must NOT have Create or Delete on Projects")
		}
		if !HasPermission(agent, ModuleProjects, ActionRead) || !HasPermission(agent, ModuleProjects, ActionUpdate) {
			t.Error("Agent must have RU on Projects")
		}

		// Change Requests: CRU (no D)
		if !HasPermission(agent, ModuleChangeRequests, ActionCreate) || !HasPermission(agent, ModuleChangeRequests, ActionRead) || !HasPermission(agent, ModuleChangeRequests, ActionUpdate) {
			t.Error("Agent must have CRU on ChangeRequests")
		}
		if HasPermission(agent, ModuleChangeRequests, ActionDelete) {
			t.Error("Agent must NOT have Delete on ChangeRequests")
		}

		// Deployments, Products, Resources: CRUD
		for _, mod := range []Module{ModuleDeployments, ModuleDeploymentProducts, ModuleDeploymentResources} {
			for _, act := range allActions {
				if !HasPermission(agent, mod, act) {
					t.Errorf("Agent must have %s on %s", act, mod)
				}
			}
		}

		// Security Admin: Zero access
		for _, act := range allActions {
			if HasPermission(agent, ModuleSecurityAdmin, act) {
				t.Errorf("Agent must NOT have %s on SecurityAdmin", act)
			}
		}
	})

	t.Run("External Roles permissions", func(t *testing.T) {
		externalRoles := [][]CanonicalRole{
			{RoleCustomerUser},
			{RoleCustomerAdmin},
			{RolePartnerUser},
			{RolePartnerAdmin},
		}

		for _, roleSet := range externalRoles {
			rName := string(roleSet[0])

			// Cases: CRU (no D)
			if !HasPermission(roleSet, ModuleCases, ActionCreate) || !HasPermission(roleSet, ModuleCases, ActionRead) || !HasPermission(roleSet, ModuleCases, ActionUpdate) {
				t.Errorf("%s must have CRU on Cases", rName)
			}
			if HasPermission(roleSet, ModuleCases, ActionDelete) {
				t.Errorf("%s must NOT have Delete on Cases", rName)
			}

			// Time Cards: Read Only
			if !HasPermission(roleSet, ModuleTimeCards, ActionRead) {
				t.Errorf("%s must have Read on TimeCards", rName)
			}
			if HasPermission(roleSet, ModuleTimeCards, ActionCreate) || HasPermission(roleSet, ModuleTimeCards, ActionUpdate) || HasPermission(roleSet, ModuleTimeCards, ActionDelete) {
				t.Errorf("%s must NOT have CUD on TimeCards", rName)
			}

			// Projects: Read Only
			if !HasPermission(roleSet, ModuleProjects, ActionRead) {
				t.Errorf("%s must have Read on Projects", rName)
			}
			if HasPermission(roleSet, ModuleProjects, ActionCreate) || HasPermission(roleSet, ModuleProjects, ActionUpdate) || HasPermission(roleSet, ModuleProjects, ActionDelete) {
				t.Errorf("%s must NOT have CUD on Projects", rName)
			}

			// Change Requests: Read Only
			if !HasPermission(roleSet, ModuleChangeRequests, ActionRead) {
				t.Errorf("%s must have Read on ChangeRequests", rName)
			}
			if HasPermission(roleSet, ModuleChangeRequests, ActionCreate) || HasPermission(roleSet, ModuleChangeRequests, ActionUpdate) || HasPermission(roleSet, ModuleChangeRequests, ActionDelete) {
				t.Errorf("%s must NOT have CUD on ChangeRequests", rName)
			}

			// Deployments, Products, Resources: CRUD
			for _, mod := range []Module{ModuleDeployments, ModuleDeploymentProducts, ModuleDeploymentResources} {
				for _, act := range allActions {
					if !HasPermission(roleSet, mod, act) {
						t.Errorf("%s must have %s on %s", rName, act, mod)
					}
				}
			}

			// Security Admin: Zero access
			for _, act := range allActions {
				if HasPermission(roleSet, ModuleSecurityAdmin, act) {
					t.Errorf("%s must NOT have %s on SecurityAdmin", rName, act)
				}
			}
		}
	})

	t.Run("Stakeholder permissions", func(t *testing.T) {
		stakeholder := []CanonicalRole{RoleStakeholder}

		// Strictly read-only on Cases, Projects, Deployments, Products, Resources
		readOnlyModules := []Module{
			ModuleCases,
			ModuleProjects,
			ModuleDeployments,
			ModuleDeploymentProducts,
			ModuleDeploymentResources,
		}
		for _, mod := range readOnlyModules {
			if !HasPermission(stakeholder, mod, ActionRead) {
				t.Errorf("Stakeholder must have Read on %s", mod)
			}
			for _, act := range []Action{ActionCreate, ActionUpdate, ActionDelete} {
				if HasPermission(stakeholder, mod, act) {
					t.Errorf("Stakeholder must NOT have %s on %s", act, mod)
				}
			}
		}

		// No access to Time Cards, Change Requests, Security Admin
		noAccessModules := []Module{
			ModuleTimeCards,
			ModuleChangeRequests,
			ModuleSecurityAdmin,
		}
		for _, mod := range noAccessModules {
			for _, act := range allActions {
				if HasPermission(stakeholder, mod, act) {
					t.Errorf("Stakeholder must NOT have %s on %s", act, mod)
				}
			}
		}
	})
}

func TestCachedRoleResolver(t *testing.T) {
	callCount := 0
	mockClient := &mockEntityUserClient{
		getMeFn: func(_ context.Context) (entity.GetUserMeResponse, error) {
			callCount++
			return entity.GetUserMeResponse{
				ID:    "usr-1",
				Roles: []string{"sn_customerservice.customer_admin"},
			}, nil
		},
	}

	resolver := NewCachedRoleResolver(mockClient, 50*time.Millisecond)

	ctx := WithUserInfo(context.Background(), &UserInfo{UserID: "usr-1", Email: "test@wso2.com"})

	// First call: populates cache
	roles, err := resolver.GetRoles(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(roles) != 1 || roles[0] != RoleCustomerAdmin {
		t.Fatalf("expected RoleCustomerAdmin, got %v", roles)
	}
	if callCount != 1 {
		t.Fatalf("expected 1 call, got %d", callCount)
	}

	// Second call: serves from cache
	roles2, err := resolver.GetRoles(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(roles2) != 1 || roles2[0] != RoleCustomerAdmin {
		t.Fatalf("expected RoleCustomerAdmin, got %v", roles2)
	}
	if callCount != 1 {
		t.Fatalf("expected 1 call (cached), got %d", callCount)
	}

	// Wait for TTL expiration
	time.Sleep(60 * time.Millisecond)

	// Third call: cache expired, calls upstream again
	_, err = resolver.GetRoles(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 2 {
		t.Fatalf("expected 2 calls after TTL expiry, got %d", callCount)
	}

	t.Run("evicts expired entries on write", func(t *testing.T) {
		r := NewCachedRoleResolver(mockClient, 20*time.Millisecond)
		ctx1 := WithUserInfo(context.Background(), &UserInfo{UserID: "user-expire", Email: "expire@wso2.com"})
		ctx2 := WithUserInfo(context.Background(), &UserInfo{UserID: "user-active", Email: "active@wso2.com"})

		_, _ = r.GetRoles(ctx1)
		time.Sleep(30 * time.Millisecond)

		// Calling GetRoles for user2 triggers sweep on write, evicting user-expire
		_, _ = r.GetRoles(ctx2)

		r.mu.RLock()
		defer r.mu.RUnlock()
		if _, exists := r.cache["user-expire"]; exists {
			t.Errorf("expected user-expire to be evicted from cache")
		}
		if _, exists := r.cache["user-active"]; !exists {
			t.Errorf("expected user-active to be present in cache")
		}
	})
}

func TestRequirePermissionMiddleware(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	t.Run("unauthenticated request returns 401", func(t *testing.T) {
		resolver := &mockRoleResolver{roles: []CanonicalRole{RoleAdmin}}
		ts := RequirePermission(resolver, ModuleCases, ActionRead)(handler)

		req := httptest.NewRequest(http.MethodGet, "/cases/1", nil)
		rr := httptest.NewRecorder()

		ts.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rr.Code)
		}
	})

	t.Run("upstream resolver error returns 502", func(t *testing.T) {
		resolver := &mockRoleResolver{err: errors.New("upstream failed")}
		ts := RequirePermission(resolver, ModuleCases, ActionRead)(handler)

		req := httptest.NewRequest(http.MethodGet, "/cases/1", nil)
		req = req.WithContext(WithUserInfo(req.Context(), &UserInfo{UserID: "usr-1"}))
		rr := httptest.NewRecorder()

		ts.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadGateway {
			t.Fatalf("expected 502, got %d", rr.Code)
		}
	})

	t.Run("forbidden action returns 403", func(t *testing.T) {
		// Stakeholder attempting to Create a Case
		resolver := &mockRoleResolver{roles: []CanonicalRole{RoleStakeholder}}
		ts := RequirePermission(resolver, ModuleCases, ActionCreate)(handler)

		req := httptest.NewRequest(http.MethodPost, "/cases", nil)
		req = req.WithContext(WithUserInfo(req.Context(), &UserInfo{UserID: "usr-1"}))
		rr := httptest.NewRecorder()

		ts.ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", rr.Code)
		}

		var body authErrorBody
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode response body: %v", err)
		}
		if body.Message != "You do not have permission to perform this action." {
			t.Errorf("unexpected forbidden message: %q", body.Message)
		}
	})

	t.Run("authorized action calls next handler", func(t *testing.T) {
		// CustomerUser creating a Case
		resolver := &mockRoleResolver{roles: []CanonicalRole{RoleCustomerUser}}
		ts := RequirePermission(resolver, ModuleCases, ActionCreate)(handler)

		req := httptest.NewRequest(http.MethodPost, "/cases", nil)
		req = req.WithContext(WithUserInfo(req.Context(), &UserInfo{UserID: "usr-1"}))
		rr := httptest.NewRecorder()

		ts.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rr.Code)
		}
	})
}

func TestRequireRolesMiddleware(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	t.Run("authorized role succeeds", func(t *testing.T) {
		resolver := &mockRoleResolver{roles: []CanonicalRole{RoleCustomerAdmin}}
		ts := RequireRoles(resolver, RoleCustomerAdmin, RolePartnerAdmin)(handler)

		req := httptest.NewRequest(http.MethodPost, "/projects/1/contacts", nil)
		req = req.WithContext(WithUserInfo(req.Context(), &UserInfo{UserID: "usr-1"}))
		rr := httptest.NewRecorder()

		ts.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rr.Code)
		}
	})

	t.Run("super admin always passes", func(t *testing.T) {
		resolver := &mockRoleResolver{roles: []CanonicalRole{RoleSuperAdmin}}
		ts := RequireRoles(resolver, RoleCustomerAdmin)(handler)

		req := httptest.NewRequest(http.MethodPost, "/projects/1/contacts", nil)
		req = req.WithContext(WithUserInfo(req.Context(), &UserInfo{UserID: "usr-1"}))
		rr := httptest.NewRecorder()

		ts.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rr.Code)
		}
	})

	t.Run("unauthorized role returns 403", func(t *testing.T) {
		resolver := &mockRoleResolver{roles: []CanonicalRole{RoleCustomerUser}}
		ts := RequireRoles(resolver, RoleCustomerAdmin, RolePartnerAdmin)(handler)

		req := httptest.NewRequest(http.MethodPost, "/projects/1/contacts", nil)
		req = req.WithContext(WithUserInfo(req.Context(), &UserInfo{UserID: "usr-1"}))
		rr := httptest.NewRecorder()

		ts.ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", rr.Code)
		}
	})
}
