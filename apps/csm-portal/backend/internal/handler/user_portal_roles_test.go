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
	"reflect"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/scim"
)

// testAccessConfigForCSMRoles mirrors testAccessConfig, but with dummy token
// role names shaped like SCIM role display names (plain strings, no assumed
// prefix of any kind) -- withPortalRoles hands SCIM's roles straight to
// RolesFor, so these are what it matches against.
func testAccessConfigForCSMRoles() AccessConfig {
	return AccessConfig{
		Viewer:               []string{"test-viewer"},
		Escalator:            []string{"test-escalator"},
		AttachmentDownloader: []string{"test-attachment-downloader"},
		UsageMetricsViewer:   []string{"test-usage-metrics-viewer"},
		CsEngineer:           []string{"test-cs-engineer"},
		Admin:                []string{"test-admin"},
		TimecardApprover:     []string{"test-timecard-approver"},
		DashboardDesigner:    []string{"test-dashboard-designer"},
	}
}

type getUserPortalRolesResponse struct {
	Roles            []string `json:"roles"`
	CsmPlatformRoles []string `json:"csmPlatformRoles"`
}

// TestGetUser_PortalRoles_AddsCsmPlatformRolesForWso2Email: a wso2.com
// target's profile gains a separate `csmPlatformRoles` field, translated from
// SCIM's role assignment through AccessGuard.RolesFor -- entity-service's own
// `roles` field (the Customer Portal's own role vocabulary) is left
// untouched, since the two describe different things for the same person.
// SCIM's full, unfiltered role list is passed straight to RolesFor: a role
// belonging to some other Asgardeo application ("some-other-app-role") is
// correctly ignored because it simply never matches any configured
// AUTH_<ROLE>_ROLES value -- not because of any separate narrowing step.
func TestGetUser_PortalRoles_AddsCsmPlatformRolesForWso2Email(t *testing.T) {
	const id = "11111111-1111-1111-1111-111111111111"
	var gotEmail string
	h := NewUsersHandler(&mockSCIMClient{
		searchUserFn: func(_ context.Context, email string) (*scim.UserInfo, error) {
			gotEmail = email
			return &scim.UserInfo{Roles: []string{
				"test-admin", "some-other-app-role",
			}}, nil
		},
	}, &mockEntityUserClient{
		getUserFn: func(_ context.Context, _ string) ([]byte, error) {
			return []byte(`{"id":"` + id + `","email":"staff@wso2.com","userType":"internal","roles":["admin","internal"]}`), nil
		},
	}, testDirectory(t), false, nil).WithAccessGuard(NewAccessGuard(testAccessConfigForCSMRoles()))

	r := withUser(httptest.NewRequest(http.MethodGet, "/users/"+id, nil))
	r.SetPathValue("id", id)
	w := httptest.NewRecorder()
	h.GetUser(w, r)

	assertStatus(t, w, http.StatusOK)
	if gotEmail != "staff@wso2.com" {
		t.Errorf("SearchUser called with email %q, want the profile's email", gotEmail)
	}
	got := decodeJSON[getUserPortalRolesResponse](t, w)
	if !reflect.DeepEqual(got.Roles, []string{"admin", "internal"}) {
		t.Errorf("roles = %v, want entity-service's own [admin internal] left untouched", got.Roles)
	}
	if !reflect.DeepEqual(got.CsmPlatformRoles, []string{"admin"}) {
		t.Errorf("csmPlatformRoles = %v, want [admin]", got.CsmPlatformRoles)
	}
}

// TestGetUser_PortalRoles_EmptyWhenNoScimRoleMatchesAnyConfiguredRole: SCIM
// can return a non-empty role list for a wso2.com target where none of them
// match any configured AUTH_<ROLE>_ROLES value -- e.g. the person genuinely
// holds no role for this portal in Asgardeo, or holds roles for other
// applications only. csmPlatformRoles must still come back as a present,
// empty array in that case -- not omitted, not an error.
func TestGetUser_PortalRoles_EmptyWhenNoScimRoleMatchesAnyConfiguredRole(t *testing.T) {
	const id = "11111111-1111-1111-1111-111111111111"
	h := NewUsersHandler(&mockSCIMClient{
		searchUserFn: func(_ context.Context, _ string) (*scim.UserInfo, error) {
			return &scim.UserInfo{Roles: []string{"some-other-app-role", "another-unrelated-role"}}, nil
		},
	}, &mockEntityUserClient{
		getUserFn: func(_ context.Context, _ string) ([]byte, error) {
			return []byte(`{"id":"` + id + `","email":"staff@wso2.com","userType":"internal","roles":["admin","internal"]}`), nil
		},
	}, testDirectory(t), false, nil).WithAccessGuard(NewAccessGuard(testAccessConfigForCSMRoles()))

	r := withUser(httptest.NewRequest(http.MethodGet, "/users/"+id, nil))
	r.SetPathValue("id", id)
	w := httptest.NewRecorder()
	h.GetUser(w, r)

	assertStatus(t, w, http.StatusOK)
	got := decodeJSON[getUserPortalRolesResponse](t, w)
	if !reflect.DeepEqual(got.Roles, []string{"admin", "internal"}) {
		t.Errorf("roles = %v, want entity-service's own [admin internal] left untouched", got.Roles)
	}
	if got.CsmPlatformRoles == nil || len(got.CsmPlatformRoles) != 0 {
		t.Errorf("csmPlatformRoles = %v, want a present, empty array", got.CsmPlatformRoles)
	}
}

// TestGetUser_PortalRoles_AddedForAWso2EmailEvenWhenTaggedExternal: a wso2.com
// address is reserved for WSO2 staff regardless of what userType the backing
// data source recorded for that row -- the same edge case
// withExternalAccountStatus already special-cases.
func TestGetUser_PortalRoles_AddedForAWso2EmailEvenWhenTaggedExternal(t *testing.T) {
	const id = "11111111-1111-1111-1111-111111111111"
	h := NewUsersHandler(&mockSCIMClient{
		searchUserFn: func(_ context.Context, _ string) (*scim.UserInfo, error) {
			return &scim.UserInfo{Roles: []string{"test-admin"}}, nil
		},
	}, &mockEntityUserClient{
		getUserFn: func(_ context.Context, _ string) ([]byte, error) {
			return []byte(`{"id":"` + id + `","email":"staff@wso2.com","userType":"external","roles":["customer"]}`), nil
		},
	}, testDirectory(t), false, nil).WithAccessGuard(NewAccessGuard(testAccessConfigForCSMRoles()))

	r := withUser(httptest.NewRequest(http.MethodGet, "/users/"+id, nil))
	r.SetPathValue("id", id)
	w := httptest.NewRecorder()
	h.GetUser(w, r)

	assertStatus(t, w, http.StatusOK)
	got := decodeJSON[getUserPortalRolesResponse](t, w)
	if !reflect.DeepEqual(got.Roles, []string{"customer"}) {
		t.Errorf("roles = %v, want entity-service's own [customer] left unchanged", got.Roles)
	}
	if !reflect.DeepEqual(got.CsmPlatformRoles, []string{"admin"}) {
		t.Errorf("csmPlatformRoles = %v, want [admin] despite userType=external", got.CsmPlatformRoles)
	}
}

// TestGetUser_PortalRoles_SkippedForNonWso2Email: a customer/partner
// contact's profile gets no `csmPlatformRoles` field at all -- there is no
// Asgardeo/SCIM role assignment to translate for a non-wso2.com address.
func TestGetUser_PortalRoles_SkippedForNonWso2Email(t *testing.T) {
	const id = "11111111-1111-1111-1111-111111111111"
	called := false
	h := NewUsersHandler(&mockSCIMClient{
		searchUserFn: func(_ context.Context, _ string) (*scim.UserInfo, error) {
			called = true
			return &scim.UserInfo{Roles: []string{"test-admin"}}, nil
		},
	}, &mockEntityUserClient{
		getUserFn: func(_ context.Context, _ string) ([]byte, error) {
			return []byte(`{"id":"` + id + `","email":"contact@example.com","userType":"external","roles":["customer"]}`), nil
		},
	}, testDirectory(t), false, nil).WithAccessGuard(NewAccessGuard(testAccessConfigForCSMRoles()))

	r := withUser(httptest.NewRequest(http.MethodGet, "/users/"+id, nil))
	r.SetPathValue("id", id)
	w := httptest.NewRecorder()
	h.GetUser(w, r)

	assertStatus(t, w, http.StatusOK)
	if called {
		t.Error("SearchUser was called for a non-wso2.com email, want no SCIM internal lookup")
	}
	got := decodeJSON[getUserPortalRolesResponse](t, w)
	if !reflect.DeepEqual(got.Roles, []string{"customer"}) {
		t.Errorf("roles = %v, want entity-service's own [customer] left unchanged", got.Roles)
	}
	if got.CsmPlatformRoles != nil {
		t.Errorf("csmPlatformRoles = %v, want no field at all", got.CsmPlatformRoles)
	}
}

// TestGetUser_PortalRoles_SkippedWhenAccessGuardNotWired: every existing
// caller/test that constructs a UsersHandler without WithAccessGuard must see
// no behavior change -- entity-service's own roles pass through untouched
// and no csmPlatformRoles field is added.
func TestGetUser_PortalRoles_SkippedWhenAccessGuardNotWired(t *testing.T) {
	const id = "11111111-1111-1111-1111-111111111111"
	called := false
	h := NewUsersHandler(&mockSCIMClient{
		searchUserFn: func(_ context.Context, _ string) (*scim.UserInfo, error) {
			called = true
			return &scim.UserInfo{Roles: []string{"test-admin"}}, nil
		},
	}, &mockEntityUserClient{
		getUserFn: func(_ context.Context, _ string) ([]byte, error) {
			return []byte(`{"id":"` + id + `","email":"staff@wso2.com","userType":"internal","roles":["admin"]}`), nil
		},
	}, testDirectory(t), false, nil)

	r := withUser(httptest.NewRequest(http.MethodGet, "/users/"+id, nil))
	r.SetPathValue("id", id)
	w := httptest.NewRecorder()
	h.GetUser(w, r)

	assertStatus(t, w, http.StatusOK)
	if called {
		t.Error("SearchUser was called with no AccessGuard wired, want no SCIM lookup at all")
	}
	got := decodeJSON[getUserPortalRolesResponse](t, w)
	if !reflect.DeepEqual(got.Roles, []string{"admin"}) {
		t.Errorf("roles = %v, want entity-service's own [admin] left unchanged", got.Roles)
	}
	if got.CsmPlatformRoles != nil {
		t.Errorf("csmPlatformRoles = %v, want no field at all", got.CsmPlatformRoles)
	}
}

// TestGetUser_PortalRoles_FailureDoesNotFailTheRequest: a SCIM error must not
// turn a 200 into an error response -- this enrichment is best-effort, same
// as the external-account and teams enrichments on this same profile.
func TestGetUser_PortalRoles_FailureDoesNotFailTheRequest(t *testing.T) {
	const id = "11111111-1111-1111-1111-111111111111"
	h := NewUsersHandler(&mockSCIMClient{
		searchUserFn: func(_ context.Context, _ string) (*scim.UserInfo, error) {
			return nil, errors.New("scim unavailable")
		},
	}, &mockEntityUserClient{
		getUserFn: func(_ context.Context, _ string) ([]byte, error) {
			return []byte(`{"id":"` + id + `","email":"staff@wso2.com","userType":"internal","roles":["admin"]}`), nil
		},
	}, testDirectory(t), false, nil).WithAccessGuard(NewAccessGuard(testAccessConfigForCSMRoles()))

	r := withUser(httptest.NewRequest(http.MethodGet, "/users/"+id, nil))
	r.SetPathValue("id", id)
	w := httptest.NewRecorder()
	h.GetUser(w, r)

	assertStatus(t, w, http.StatusOK)
	got := decodeJSON[getUserPortalRolesResponse](t, w)
	if !reflect.DeepEqual(got.Roles, []string{"admin"}) {
		t.Errorf("roles = %v, want entity-service's own [admin] left unchanged despite the SCIM failure", got.Roles)
	}
	if got.CsmPlatformRoles != nil {
		t.Errorf("csmPlatformRoles = %v, want no field at all when SCIM fails", got.CsmPlatformRoles)
	}
}
