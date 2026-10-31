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
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/usermanagement"
)

const (
	contactTestProject = "11111111-1111-1111-1111-111111111111"
	contactTestCaller  = "jane.doe@example.com"
)

// contactEntityFake is entity-service for the contact routes: it answers the
// role resolver's GetMe and the handler's GetMe/ListProjectContacts, and counts
// the writes that reach it.
type contactEntityFake struct {
	roles    []string
	userType string
	contacts []entity.ProjectContact
	writes   int
}

func (f *contactEntityFake) GetMe(context.Context) (entity.GetUserMeResponse, error) {
	return entity.GetUserMeResponse{Email: contactTestCaller, Roles: f.roles, UserType: f.userType}, nil
}

func (f *contactEntityFake) GetProject(context.Context, string) (entity.ProjectDetailsView, error) {
	return entity.ProjectDetailsView{}, nil
}

func (f *contactEntityFake) ListProjectContacts(context.Context, string) ([]entity.ProjectContact, error) {
	return f.contacts, nil
}

func (f *contactEntityFake) CreateProjectMembership(context.Context, string, entity.CreateProjectMembershipRequest) (entity.ProjectMembership, error) {
	f.writes++
	return entity.ProjectMembership{}, nil
}

func (f *contactEntityFake) UpdateProjectMembershipRoles(context.Context, string, string, entity.UpdateProjectMembershipRolesRequest) (entity.ProjectMembership, error) {
	f.writes++
	return entity.ProjectMembership{}, nil
}

func (f *contactEntityFake) DeactivateProjectMembership(context.Context, string, string) error {
	f.writes++
	return nil
}

func (f *contactEntityFake) ResendProjectMembershipInvitation(context.Context, string, string) error {
	f.writes++
	return nil
}

func (f *contactEntityFake) ValidateProjectMembership(context.Context, string, entity.ValidateProjectMembershipRequest) (entity.ProjectMembershipValidation, error) {
	f.writes++
	return entity.ProjectMembershipValidation{Valid: true}, nil
}

// contactLegacyFake is the pre-cutover onboarding service; it only counts calls.
type contactLegacyFake struct{ calls int }

func (f *contactLegacyFake) GetProjectContacts(context.Context, string) ([]usermanagement.Contact, error) {
	f.calls++
	return nil, nil
}

func (f *contactLegacyFake) CreateProjectContact(context.Context, string, usermanagement.OnBoardContactPayload) (usermanagement.Membership, error) {
	f.calls++
	return usermanagement.Membership{}, nil
}

func (f *contactLegacyFake) RemoveProjectContact(context.Context, string, string, string) (usermanagement.Membership, error) {
	f.calls++
	return usermanagement.Membership{}, nil
}

func (f *contactLegacyFake) UpdateMembershipRole(context.Context, string, string, usermanagement.MembershipRolePayload) (usermanagement.Membership, error) {
	f.calls++
	return usermanagement.Membership{}, nil
}

func (f *contactLegacyFake) ValidateProjectContact(context.Context, usermanagement.ValidationPayload) (*usermanagement.Contact, bool, error) {
	f.calls++
	return nil, false, nil
}

// TestContactRouteGating drives the contact write routes through
// registerContactRoutes, the registration main uses, with the real role
// resolver and the real handler. entity-service and the legacy service are
// fakes that count what reaches them.
func TestContactRouteGating(t *testing.T) {
	member := func(roles ...string) []entity.ProjectContact {
		return []entity.ProjectContact{{Email: contactTestCaller, RegistrationState: "REGISTERED", Roles: roles}}
	}
	base := "/projects/" + contactTestProject + "/contacts"
	type route struct{ name, method, path, body string }
	routes := []route{
		{"invite", http.MethodPost, base, `{"contactEmail":"john.roe@example.com","contactFirstName":"John","contactLastName":"Roe","isPortalUser":true}`},
		{"remove", http.MethodDelete, base + "/john.roe@example.com", ``},
		{"change role", http.MethodPatch, base + "/john.roe@example.com", `{"isCsAdmin":true}`},
		{"resend", http.MethodPost, base + "/john.roe@example.com/resend-invitation", ``},
		{"validate", http.MethodPost, base + "/validate", `{"contactEmail":"john.roe@example.com"}`},
	}

	cases := []struct {
		name      string
		portalOn  bool
		roles     []string
		userType  string
		contacts  []entity.ProjectContact
		want      int
		wantWrite bool
	}{
		{"on: project-only admin, no account role", true, []string{"external"}, "", member("PORTAL_USER", "ADMIN"), http.StatusOK, true},
		{"on: member with neither signal", true, []string{"external"}, "", member("PORTAL_USER"), http.StatusForbidden, false},
		{"on: account admin who is not a member", true, []string{"customer_admin"}, "", nil, http.StatusForbidden, false},
		{"on: project-only admin in the blocked internal group", true, []string{"external"}, entity.UserTypeInternal, member("ADMIN"), http.StatusForbidden, false},
		{"on: project ADMIN on a deactivated membership", true, []string{"external"}, "",
			[]entity.ProjectContact{{Email: contactTestCaller, RegistrationState: "DEACTIVATED", Roles: []string{"ADMIN"}}}, http.StatusForbidden, false},
		{"on: account admin member", true, []string{"customer_admin"}, "", member("PORTAL_USER"), http.StatusOK, true},
		{"off: project-only admin", false, []string{"external"}, "", member("ADMIN"), http.StatusForbidden, false},
	}

	for _, tc := range cases {
		for _, rt := range routes {
			t.Run(tc.name+"/"+rt.name, func(t *testing.T) {
				if !tc.portalOn && rt.name == "resend" {
					t.Skip("resend is not offered on the legacy path")
				}
				ent := &contactEntityFake{roles: tc.roles, userType: tc.userType, contacts: tc.contacts}
				legacy := &contactLegacyFake{}
				h := handler.NewContactHandler(ent, legacy, ent, tc.portalOn)
				// An empty group claim never matches, so an internal-type caller is blocked.
				resolver := middleware.NewCachedRoleResolver(ent, time.Minute).WithInternalWriteGroups([]string{"write-group"})
				mux := http.NewServeMux()
				registerContactRoutes(mux, resolver, h)

				ctx := middleware.WithUserInfo(context.Background(), &middleware.UserInfo{UserID: "00000000-0000-0000-0000-000000000000", Email: contactTestCaller})
				req := httptest.NewRequestWithContext(ctx, rt.method, rt.path, strings.NewReader(rt.body))
				req.Header.Set("Content-Type", "application/json")
				rr := httptest.NewRecorder()
				mux.ServeHTTP(rr, req)

				if rr.Code != tc.want {
					t.Fatalf("status = %d, want %d; body %s", rr.Code, tc.want, rr.Body)
				}
				if got := ent.writes > 0; got != tc.wantWrite {
					t.Fatalf("entity-service writes = %d, want a write: %v", ent.writes, tc.wantWrite)
				}
				if !tc.portalOn && legacy.calls != 0 {
					t.Fatalf("legacy service called %d time(s) for a refused request", legacy.calls)
				}
			})
		}
	}
}
