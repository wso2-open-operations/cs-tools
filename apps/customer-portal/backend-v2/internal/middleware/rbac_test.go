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
				"sn_customerservice.admin",
				"wso2_agent",
				"sn_customerservice.customer_admin",
				"sn_customerservice.customer",
				"sn_customerservice.partner_admin",
				"sn_customerservice.partner",
			},
			expected: []CanonicalRole{
				RoleAdmin,
				RoleAgent,
				RoleCustomerAdmin,
				RoleCustomerUser,
				RolePartnerAdmin,
				RolePartnerUser,
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
	}
	allActions := []Action{ActionCreate, ActionRead, ActionUpdate, ActionDelete}

	// snc_external / external is a customer-facing role. It used to normalise to
	// itself, match no matrix entry and grant nothing, so a user carrying only
	// it was locked out of the whole portal.
	t.Run("external resolves to the customer persona", func(t *testing.T) {
		for _, raw := range []string{"snc_external", "external"} {
			if got := NormalizeRole(raw); got != RoleCustomerUser {
				t.Errorf("%s normalised to %q, want %q", raw, got, RoleCustomerUser)
			}
		}
	})

	// snc_internal used to become agent while the Postgres form became internal
	// and held nothing, so the same person's access depended on which data
	// source entity-service was running.
	t.Run("both internal wire forms agree and hold what agent holds", func(t *testing.T) {
		for _, raw := range []string{"snc_internal", "internal"} {
			if got := NormalizeRole(raw); got != RoleInternal {
				t.Errorf("%s normalised to %q, want %q", raw, got, RoleInternal)
			}
		}
		for _, mod := range allModules {
			for _, act := range allActions {
				internal := HasPermission([]CanonicalRole{RoleInternal}, mod, act)
				agent := HasPermission([]CanonicalRole{RoleAgent}, mod, act)
				if internal != agent {
					t.Errorf("%s on %s: internal=%v agent=%v, want equal", act, mod, internal, agent)
				}
			}
		}
	})

	t.Run("Admin has full CRUD on every module", func(t *testing.T) {
		admin := []CanonicalRole{RoleAdmin}
		for _, mod := range allModules {
			for _, act := range allActions {
				if !HasPermission(admin, mod, act) {
					t.Errorf("Admin must have %s on %s", act, mod)
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

			// Projects: Read Only, EXCEPT RoleCustomerAdmin also has Update --
			// PATCH /projects/{id} (the only route ModuleProjects/ActionUpdate
			// gates) only ever touches the project's own AI Assistant settings
			// (hasAgent/hasKbReferences), not general project data, so a
			// customer's own Admin is allowed to toggle those for their own
			// project.
			isCustomerAdmin := rName == string(RoleCustomerAdmin)
			if !HasPermission(roleSet, ModuleProjects, ActionRead) {
				t.Errorf("%s must have Read on Projects", rName)
			}
			if HasPermission(roleSet, ModuleProjects, ActionCreate) || HasPermission(roleSet, ModuleProjects, ActionDelete) {
				t.Errorf("%s must NOT have Create/Delete on Projects", rName)
			}
			if HasPermission(roleSet, ModuleProjects, ActionUpdate) != isCustomerAdmin {
				t.Errorf("%s Update on Projects = %v, want %v", rName, !isCustomerAdmin, isCustomerAdmin)
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
		// An unrecognised role normalises to itself and matches no matrix entry.
		resolver := &mockRoleResolver{roles: NormalizeRoles([]string{"sn_customerservice.stakeholder"})}
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

	// There is no blanket-grant role any more: a caller whose roles are not in
	// the call site's allow-list is refused, full stop.
	t.Run("role outside the allow list is refused", func(t *testing.T) {
		resolver := &mockRoleResolver{roles: []CanonicalRole{RoleCustomerUser}}
		ts := RequireRoles(resolver, RoleCustomerAdmin)(handler)

		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/projects/1/contacts", nil)
		req = req.WithContext(WithUserInfo(req.Context(), &UserInfo{UserID: "usr-1"}))
		rr := httptest.NewRecorder()

		ts.ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", rr.Code)
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

// customerSideRoles are the external roles. They hold ActionDecide on change
// requests -- the customer's own answer -- and nothing that edits one.
var customerSideRoles = []CanonicalRole{RoleCustomerAdmin, RoleCustomerUser, RolePartnerAdmin, RolePartnerUser}

// staffRoles are the WSO2-side roles that hold ActionUpdate on change requests.
var staffRoles = []CanonicalRole{RoleAdmin, RoleAgent, RoleInternal}

// TestChangeRequestDecidePermission pins who may give the customer's answer on a
// change request (approve / reject a Customer Approval or Customer Review,
// propose a new implementation time) and, as importantly, what that grant does
// not carry.
func TestChangeRequestDecidePermission(t *testing.T) {
	t.Run("every customer-side role may decide", func(t *testing.T) {
		for _, role := range customerSideRoles {
			if !HasPermission([]CanonicalRole{role}, ModuleChangeRequests, ActionDecide) {
				t.Errorf("%s must hold Decide on ChangeRequests: the customer answers in this portal", role)
			}
		}
	})

	t.Run("deciding does not let a customer-side role edit, create or delete", func(t *testing.T) {
		for _, role := range customerSideRoles {
			for _, act := range []Action{ActionCreate, ActionUpdate, ActionDelete} {
				if HasPermission([]CanonicalRole{role}, ModuleChangeRequests, act) {
					t.Errorf("%s must NOT hold %s on ChangeRequests", role, act)
				}
			}
			// Read stays: a customer sees the change requests they answer.
			if !HasPermission([]CanonicalRole{role}, ModuleChangeRequests, ActionRead) {
				t.Errorf("%s must keep Read on ChangeRequests", role)
			}
		}
	})

	t.Run("staff roles keep Update and also hold Decide", func(t *testing.T) {
		// The decision route is theirs too (internal stages are decided there),
		// and it now takes Decide rather than Update.
		for _, role := range staffRoles {
			for _, act := range []Action{ActionRead, ActionUpdate, ActionDecide} {
				if !HasPermission([]CanonicalRole{role}, ModuleChangeRequests, act) {
					t.Errorf("%s must hold %s on ChangeRequests", role, act)
				}
			}
		}
		if !HasPermission([]CanonicalRole{RoleAdmin}, ModuleChangeRequests, ActionDelete) {
			t.Error("Admin must keep Delete on ChangeRequests")
		}
	})

	// Stakeholder has no access to change requests and must stay that way. The
	// role is not emitted by any data source and normalises to itself, so it
	// matches no entry for any action, Decide included.
	t.Run("an unrecognised role such as stakeholder holds nothing", func(t *testing.T) {
		roles := NormalizeRoles([]string{"sn_customerservice.stakeholder", "stakeholder"})
		for _, act := range []Action{ActionCreate, ActionRead, ActionUpdate, ActionDelete, ActionDecide} {
			if HasPermission(roles, ModuleChangeRequests, act) {
				t.Errorf("stakeholder must NOT hold %s on ChangeRequests", act)
			}
		}
	})

	t.Run("a caller with no roles holds nothing", func(t *testing.T) {
		if HasPermission(nil, ModuleChangeRequests, ActionDecide) {
			t.Error("a caller with no roles must not hold Decide")
		}
	})

	t.Run("ActionDecide exists on change requests only", func(t *testing.T) {
		everyone := []CanonicalRole{RoleAdmin, RoleAgent, RoleInternal, RoleCustomerAdmin, RoleCustomerUser, RolePartnerAdmin, RolePartnerUser}
		for _, mod := range []Module{ModuleCases, ModuleTimeCards, ModuleProjects, ModuleDeployments, ModuleDeploymentProducts, ModuleDeploymentResources} {
			if HasPermission(everyone, mod, ActionDecide) {
				t.Errorf("Decide must not be defined on %s", mod)
			}
		}
	})
}

// TestPermissionMatrixOtherModulesUnchanged is a golden copy of the matrix as
// it was before ActionDecide: adding the grant must not have moved anything for
// any module's Create / Read / Update / Delete, change requests' included.
func TestPermissionMatrixOtherModulesUnchanged(t *testing.T) {
	all := []CanonicalRole{RoleAdmin, RoleAgent, RoleInternal, RoleCustomerAdmin, RoleCustomerUser, RolePartnerAdmin, RolePartnerUser}
	staff := []CanonicalRole{RoleAdmin, RoleAgent, RoleInternal}
	admin := []CanonicalRole{RoleAdmin}
	withCustomerAdmin := append(append([]CanonicalRole{}, staff...), RoleCustomerAdmin)

	want := map[Module]map[Action][]CanonicalRole{
		ModuleCases:               {ActionCreate: all, ActionRead: all, ActionUpdate: all, ActionDelete: admin},
		ModuleTimeCards:           {ActionCreate: staff, ActionRead: all, ActionUpdate: staff, ActionDelete: admin},
		ModuleProjects:            {ActionCreate: admin, ActionRead: all, ActionUpdate: withCustomerAdmin, ActionDelete: admin},
		ModuleChangeRequests:      {ActionCreate: staff, ActionRead: all, ActionUpdate: staff, ActionDelete: admin},
		ModuleDeployments:         {ActionCreate: all, ActionRead: all, ActionUpdate: all, ActionDelete: all},
		ModuleDeploymentProducts:  {ActionCreate: all, ActionRead: all, ActionUpdate: all, ActionDelete: all},
		ModuleDeploymentResources: {ActionCreate: all, ActionRead: all, ActionUpdate: all, ActionDelete: all},
	}

	if len(permissionMatrix) != len(want) {
		t.Fatalf("permissionMatrix has %d modules, want %d", len(permissionMatrix), len(want))
	}
	for mod, actions := range want {
		for _, act := range []Action{ActionCreate, ActionRead, ActionUpdate, ActionDelete} {
			wantRoles := map[CanonicalRole]bool{}
			for _, r := range actions[act] {
				wantRoles[r] = true
			}
			for _, role := range all {
				if got := HasPermission([]CanonicalRole{role}, mod, act); got != wantRoles[role] {
					t.Errorf("%s on %s for %s = %v, want %v", act, mod, role, got, wantRoles[role])
				}
			}
		}
	}
}

func TestRequirePermissionOneOfMiddleware(t *testing.T) {
	var gotAction Action
	var gotOK bool
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAction, gotOK = GrantedActionFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})
	serve := func(t *testing.T, resolver RoleResolver, authed bool) *httptest.ResponseRecorder {
		t.Helper()
		gotAction, gotOK = "", false
		ts := RequirePermissionOneOf(resolver, ModuleChangeRequests, ActionUpdate, ActionDecide)(handler)
		req := httptest.NewRequest(http.MethodPatch, "/change-requests/1", nil)
		if authed {
			req = req.WithContext(WithUserInfo(req.Context(), &UserInfo{UserID: "usr-1"}))
		}
		rr := httptest.NewRecorder()
		ts.ServeHTTP(rr, req)
		return rr
	}

	t.Run("a staff role is served at the Update level", func(t *testing.T) {
		for _, role := range staffRoles {
			rr := serve(t, &mockRoleResolver{roles: []CanonicalRole{role}}, true)
			if rr.Code != http.StatusOK {
				t.Fatalf("%s: status %d, want 200", role, rr.Code)
			}
			if !gotOK || gotAction != ActionUpdate {
				t.Errorf("%s: granted action = %q (%v), want %q", role, gotAction, gotOK, ActionUpdate)
			}
		}
	})

	t.Run("a customer-side role is served at the Decide level only", func(t *testing.T) {
		for _, role := range customerSideRoles {
			rr := serve(t, &mockRoleResolver{roles: []CanonicalRole{role}}, true)
			if rr.Code != http.StatusOK {
				t.Fatalf("%s: status %d, want 200", role, rr.Code)
			}
			if !gotOK || gotAction != ActionDecide {
				t.Errorf("%s: granted action = %q (%v), want %q", role, gotAction, gotOK, ActionDecide)
			}
		}
	})

	t.Run("the broadest action wins when a caller holds several", func(t *testing.T) {
		rr := serve(t, &mockRoleResolver{roles: []CanonicalRole{RoleCustomerUser, RoleAgent}}, true)
		if rr.Code != http.StatusOK || gotAction != ActionUpdate {
			t.Fatalf("status %d, granted %q; want 200 and %q", rr.Code, gotAction, ActionUpdate)
		}
	})

	t.Run("a stakeholder is refused and the handler never runs", func(t *testing.T) {
		rr := serve(t, &mockRoleResolver{roles: NormalizeRoles([]string{"sn_customerservice.stakeholder"})}, true)
		if rr.Code != http.StatusForbidden {
			t.Fatalf("status %d, want 403", rr.Code)
		}
		if gotOK {
			t.Error("handler ran for a caller with no matching action")
		}
		var body authErrorBody
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil || body.Message != "You do not have permission to perform this action." {
			t.Errorf("body = %+v (%v), want the standard forbidden message", body, err)
		}
	})

	t.Run("unauthenticated is 401 and an unresolvable role is 502", func(t *testing.T) {
		if rr := serve(t, &mockRoleResolver{roles: staffRoles}, false); rr.Code != http.StatusUnauthorized {
			t.Errorf("unauthenticated: status %d, want 401", rr.Code)
		}
		if rr := serve(t, &mockRoleResolver{err: errors.New("upstream failed")}, true); rr.Code != http.StatusBadGateway {
			t.Errorf("resolver error: status %d, want 502", rr.Code)
		}
	})

	t.Run("a request that skipped the middleware reports no granted action", func(t *testing.T) {
		if _, ok := GrantedActionFromContext(context.Background()); ok {
			t.Error("a bare context must not report a granted action")
		}
	})
}
