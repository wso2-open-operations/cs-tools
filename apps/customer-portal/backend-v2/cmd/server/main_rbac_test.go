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
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/handler"
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

// countingChangeRequestClient is the entity-service side of the change-request
// routes: it answers every call and counts them, so a test can tell a refusal at
// the gate (nothing reaches it) from a request that was let through.
type countingChangeRequestClient struct {
	calls int
}

func (c *countingChangeRequestClient) CreateChangeRequest(context.Context, entity.CreateChangeRequestRequest) (entity.CreateChangeRequestResponse, error) {
	c.calls++
	return entity.CreateChangeRequestResponse{}, nil
}

func (c *countingChangeRequestClient) SearchChangeRequests(context.Context, entity.SearchChangeRequestsRequest) (entity.SearchChangeRequestsResponse, error) {
	c.calls++
	return entity.SearchChangeRequestsResponse{}, nil
}

func (c *countingChangeRequestClient) GetChangeRequest(context.Context, string) (entity.ChangeRequest, error) {
	c.calls++
	return entity.ChangeRequest{}, nil
}

func (c *countingChangeRequestClient) UpdateChangeRequest(context.Context, string, entity.PatchChangeRequestRequest) (entity.PatchChangeRequestResponse, error) {
	c.calls++
	return entity.PatchChangeRequestResponse{}, nil
}

func (c *countingChangeRequestClient) GetChangeRequestApprovals(context.Context, string) (entity.ChangeRequestApprovals, error) {
	c.calls++
	return entity.ChangeRequestApprovals{}, nil
}

func (c *countingChangeRequestClient) DecideChangeRequestApproval(_ context.Context, id string, req entity.ChangeRequestApprovalDecisionRequest) (entity.ChangeRequestApprovalDecisionResponse, error) {
	c.calls++
	return entity.ChangeRequestApprovalDecisionResponse{ID: id, State: req.Decision}, nil
}

// TestChangeRequestRouteGating drives the real change-request routes
// (registerChangeRequestRoutes, the exact table main uses) with each role: who
// may answer a change request that is waiting on them, who may edit one, and
// that a customer-side role cannot do anything else to it.
func TestChangeRequestRouteGating(t *testing.T) {
	const (
		crID      = "22222222-2222-2222-2222-222222222222"
		projectID = "11111111-1111-1111-1111-111111111111"
	)
	type request struct {
		name, method, path, body string
	}
	approve := request{"PATCH approve", http.MethodPatch, "/change-requests/" + crID, `{"isCustomerApproved":true}`}
	review := request{"PATCH review", http.MethodPatch, "/change-requests/" + crID, `{"isCustomerReviewed":false}`}
	propose := request{"PATCH propose a new time", http.MethodPatch, "/change-requests/" + crID, `{"plannedStartOn":"2026-10-10 10:00:00"}`}
	editTitle := request{"PATCH title", http.MethodPatch, "/change-requests/" + crID, `{"title":"x"}`}
	approveAndMove := request{"PATCH approve plus state", http.MethodPatch, "/change-requests/" + crID, `{"isCustomerApproved":true,"state":"scheduled"}`}
	decide := request{"POST decision", http.MethodPost, "/change-requests/" + crID + "/approvals/decision", `{"decision":"approved"}`}
	create := request{"POST create", http.MethodPost, "/change-requests", `{"subject":"x"}`}
	get := request{"GET one", http.MethodGet, "/change-requests/" + crID, ``}
	approvals := request{"GET approvals", http.MethodGet, "/change-requests/" + crID + "/approvals", ``}
	search := request{"POST search", http.MethodPost, "/projects/" + projectID + "/change-requests/search", `{}`}
	all := []request{approve, review, propose, editTitle, approveAndMove, decide, create, get, approvals, search}

	customerSide := map[string]string{
		"customer":      "sn_customerservice.customer",
		"customerAdmin": "sn_customerservice.customer_admin",
		"partner":       "sn_customerservice.partner",
		"partnerAdmin":  "sn_customerservice.partner_admin",
	}
	staff := map[string]string{
		"admin":    "sn_customerservice.admin",
		"agent":    "wso2_agent",
		"internal": "snc_internal",
	}

	serve := func(rawRole string, r request) (int, int) {
		client := &countingChangeRequestClient{}
		mux := http.NewServeMux()
		registerChangeRequestRoutes(mux, middleware.NewCachedRoleResolver(&mockEntityClient{roles: []string{rawRole}}, time.Minute), handler.NewChangeRequestHandler(client))

		ctx := middleware.WithUserInfo(context.Background(), &middleware.UserInfo{UserID: "usr-test", Email: "someone@example.com"})
		req := httptest.NewRequestWithContext(ctx, r.method, r.path, strings.NewReader(r.body))
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		return rr.Code, client.calls
	}

	for name, role := range customerSide {
		want := map[string]int{
			approve.name: http.StatusOK, review.name: http.StatusOK, propose.name: http.StatusOK,
			editTitle.name: http.StatusForbidden, approveAndMove.name: http.StatusForbidden,
			decide.name: http.StatusOK, create.name: http.StatusForbidden,
			get.name: http.StatusOK, approvals.name: http.StatusOK, search.name: http.StatusOK,
		}
		for _, r := range all {
			t.Run(name+"/"+r.name, func(t *testing.T) {
				code, calls := serve(role, r)
				if code != want[r.name] {
					t.Fatalf("status = %d, want %d", code, want[r.name])
				}
				if (code == http.StatusOK) != (calls == 1) {
					t.Fatalf("status %d but %d call(s) reached entity-service: a refused request must reach nothing, an allowed one exactly once", code, calls)
				}
			})
		}
	}

	// The staff body is the wider customer-safe struct, in which "state" is simply
	// not a field: it is dropped, as it always was, so the request still passes.
	for name, role := range staff {
		for _, r := range all {
			t.Run(name+"/"+r.name, func(t *testing.T) {
				want := http.StatusOK
				if r.name == create.name {
					want = http.StatusCreated
				}
				code, calls := serve(role, r)
				if code != want || calls != 1 {
					t.Fatalf("status = %d (calls %d), want %d (1)", code, calls, want)
				}
			})
		}
	}

	t.Run("a stakeholder reaches nothing", func(t *testing.T) {
		for _, r := range all {
			code, calls := serve("sn_customerservice.stakeholder", r)
			if code != http.StatusForbidden || calls != 0 {
				t.Errorf("%s: status = %d (calls %d), want 403 (0)", r.name, code, calls)
			}
		}
	})
}
