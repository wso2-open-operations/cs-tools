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
// prefix of any kind) -- withPortalRoles matches a held role by ID, resolved
// through these names via testRoleIDsForCSMRoles/newAccessGuardForCSMRoles,
// not by name directly.
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

// testRoleIDsForCSMRoles is a dummy ASGARDEO_ROLE_IDS mapping (real role name
// -> Asgardeo role ID) for every name testAccessConfigForCSMRoles configures
// -- what AccessGuard.WithRoleIDs resolves against. "test-admin-id" is
// deliberately not a UUID shape, the same "plain string, no assumed format"
// posture the role names themselves take.
func testRoleIDsForCSMRoles() map[string]string {
	return map[string]string{
		"test-viewer":                "test-viewer-id",
		"test-escalator":             "test-escalator-id",
		"test-attachment-downloader": "test-attachment-downloader-id",
		"test-usage-metrics-viewer":  "test-usage-metrics-viewer-id",
		"test-cs-engineer":           "test-cs-engineer-id",
		"test-admin":                 "test-admin-id",
		"test-timecard-approver":     "test-timecard-approver-id",
		"test-dashboard-designer":    "test-dashboard-designer-id",
	}
}

// testGroupNamesForCSMRoles is a dummy ASGARDEO_ROLE_GROUP_NAMES mapping
// (portal role key -> Asgardeo group display name) -- what
// AccessGuard.WithGroupNames resolves against, alongside
// testRoleIDsForCSMRoles above.
func testGroupNamesForCSMRoles() map[string][]string {
	return map[string][]string{
		"cs_engineer": {"CSM Engineers"},
	}
}

// newAccessGuardForCSMRoles builds the AccessGuard these tests wire in --
// WithRoleIDs is what makes RolesForIDs (what withPortalRoles now calls)
// resolve anything at all; a guard built without it answers empty for every
// role, the same as if ASGARDEO_ROLE_IDS configured none of these names.
// WithGroupNames is the same for group-based matching.
func newAccessGuardForCSMRoles() *AccessGuard {
	return NewAccessGuard(testAccessConfigForCSMRoles()).
		WithRoleIDs(testRoleIDsForCSMRoles()).
		WithGroupNames(testGroupNamesForCSMRoles())
}

type getUserPortalRolesResponse struct {
	Roles            []string `json:"roles"`
	CsmPlatformRoles []string `json:"csmPlatformRoles"`
}

// TestGetUser_PortalRoles_AddsCsmPlatformRolesForWso2Email: a wso2.com
// target's profile gains a separate `csmPlatformRoles` field, translated from
// SCIM's role assignment through SearchUser (resolving the target's Asgardeo
// ID) then GetUserByID (that ID's full role list) then
// AccessGuard.RolesForIDs -- entity-service's own `roles` field (the
// Customer Portal's own role vocabulary) is left untouched, since the two
// describe different things for the same person. SCIM's full, unfiltered
// role list is passed straight to RolesForIDs: a role belonging to some
// other Asgardeo application ("some-other-app-role-id", no configured
// mapping) is correctly ignored because it simply never matches any
// configured ASGARDEO_ROLE_IDS entry -- not because of any separate
// narrowing step.
func TestGetUser_PortalRoles_AddsCsmPlatformRolesForWso2Email(t *testing.T) {
	const id = "11111111-1111-1111-1111-111111111111"
	const scimUserID = "scim-user-1"
	var gotEmail string
	var gotUserID string
	h := NewUsersHandler(&mockSCIMClient{
		searchUserFn: func(_ context.Context, email string) (*scim.UserInfo, error) {
			gotEmail = email
			return &scim.UserInfo{ID: scimUserID}, nil
		},
		getUserByIDFn: func(_ context.Context, userID string) (scim.UserAssignments, error) {
			gotUserID = userID
			return scim.UserAssignments{Roles: []scim.RoleAssignment{
				{ID: "test-admin-id", Name: "test-admin"},
				{ID: "some-other-app-role-id", Name: "some-other-app-role"},
			}}, nil
		},
	}, &mockEntityUserClient{
		getUserFn: func(_ context.Context, _ string) ([]byte, error) {
			return []byte(`{"id":"` + id + `","email":"staff@wso2.com","userType":"internal","roles":["admin","internal"]}`), nil
		},
	}, testDirectory(t), nil).WithAccessGuard(newAccessGuardForCSMRoles())

	r := withUser(httptest.NewRequest(http.MethodGet, "/users/"+id, nil))
	r.SetPathValue("id", id)
	w := httptest.NewRecorder()
	h.GetUser(w, r)

	assertStatus(t, w, http.StatusOK)
	if gotEmail != "staff@wso2.com" {
		t.Errorf("SearchUser called with email %q, want the profile's email", gotEmail)
	}
	if gotUserID != scimUserID {
		t.Errorf("GetUserByID called with %q, want the SCIM user ID %q", gotUserID, scimUserID)
	}
	got := decodeJSON[getUserPortalRolesResponse](t, w)
	if !reflect.DeepEqual(got.Roles, []string{"admin", "internal"}) {
		t.Errorf("roles = %v, want entity-service's own [admin internal] left untouched", got.Roles)
	}
	if !reflect.DeepEqual(got.CsmPlatformRoles, []string{"admin"}) {
		t.Errorf("csmPlatformRoles = %v, want [admin]", got.CsmPlatformRoles)
	}
}

// TestGetUser_PortalRoles_AddsCsmPlatformRolesFromGroupMembershipAlone: a
// target holding NO role that matches ASGARDEO_ROLE_IDS at all must still
// gain a csmPlatformRoles entry when they belong to a group whose DISPLAY
// NAME is configured against ASGARDEO_ROLE_GROUP_NAMES -- this is the whole
// point of group-based matching: some portal roles are granted by Asgardeo
// group membership instead of (or in addition to) a role assignment, named
// directly by the group's own display name, not its opaque id.
func TestGetUser_PortalRoles_AddsCsmPlatformRolesFromGroupMembershipAlone(t *testing.T) {
	const id = "11111111-1111-1111-1111-111111111111"
	h := NewUsersHandler(&mockSCIMClient{
		searchUserFn: func(_ context.Context, _ string) (*scim.UserInfo, error) {
			return &scim.UserInfo{ID: "scim-user-1"}, nil
		},
		getUserByIDFn: func(_ context.Context, _ string) (scim.UserAssignments, error) {
			return scim.UserAssignments{
				// No role assignment matches any configured ASGARDEO_ROLE_IDS
				// entry at all -- the portal role below must come from the
				// group alone.
				Roles: []scim.RoleAssignment{{ID: "some-other-app-role-id", Name: "some-other-app-role"}},
				Groups: []scim.GroupAssignment{
					// Matched by Name, not ID -- ID is deliberately a different,
					// unrelated string here to prove the match never falls back
					// to it.
					{ID: "some-asgardeo-group-uuid", Name: "CSM Engineers"},
					{ID: "some-unrelated-group-uuid", Name: "Unrelated Group"},
				},
			}, nil
		},
	}, &mockEntityUserClient{
		getUserFn: func(_ context.Context, _ string) ([]byte, error) {
			return []byte(`{"id":"` + id + `","email":"staff@wso2.com","userType":"internal","roles":["admin","internal"]}`), nil
		},
	}, testDirectory(t), nil).WithAccessGuard(newAccessGuardForCSMRoles())

	r := withUser(httptest.NewRequest(http.MethodGet, "/users/"+id, nil))
	r.SetPathValue("id", id)
	w := httptest.NewRecorder()
	h.GetUser(w, r)

	assertStatus(t, w, http.StatusOK)
	got := decodeJSON[getUserPortalRolesResponse](t, w)
	if !reflect.DeepEqual(got.CsmPlatformRoles, []string{"cs_engineer"}) {
		t.Errorf("csmPlatformRoles = %v, want [cs_engineer] from group membership alone", got.CsmPlatformRoles)
	}
}

// TestGetUser_PortalRoles_EmptyWhenNoScimRoleMatchesAnyConfiguredRole: SCIM
// can return a non-empty role list for a wso2.com target where none of their
// role IDs match any configured ASGARDEO_ROLE_IDS entry -- e.g. the person
// genuinely holds no role for this portal in Asgardeo, or holds roles for
// other applications only. csmPlatformRoles must still come back as a
// present, empty array in that case -- not omitted, not an error.
func TestGetUser_PortalRoles_EmptyWhenNoScimRoleMatchesAnyConfiguredRole(t *testing.T) {
	const id = "11111111-1111-1111-1111-111111111111"
	h := NewUsersHandler(&mockSCIMClient{
		searchUserFn: func(_ context.Context, _ string) (*scim.UserInfo, error) {
			return &scim.UserInfo{ID: "scim-user-1"}, nil
		},
		getUserByIDFn: func(_ context.Context, _ string) (scim.UserAssignments, error) {
			return scim.UserAssignments{Roles: []scim.RoleAssignment{
				{ID: "some-other-app-role-id", Name: "some-other-app-role"},
				{ID: "another-unrelated-role-id", Name: "another-unrelated-role"},
			}}, nil
		},
	}, &mockEntityUserClient{
		getUserFn: func(_ context.Context, _ string) ([]byte, error) {
			return []byte(`{"id":"` + id + `","email":"staff@wso2.com","userType":"internal","roles":["admin","internal"]}`), nil
		},
	}, testDirectory(t), nil).WithAccessGuard(newAccessGuardForCSMRoles())

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
			return &scim.UserInfo{ID: "scim-user-1"}, nil
		},
		getUserByIDFn: func(_ context.Context, _ string) (scim.UserAssignments, error) {
			return scim.UserAssignments{Roles: []scim.RoleAssignment{{ID: "test-admin-id", Name: "test-admin"}}}, nil
		},
	}, &mockEntityUserClient{
		getUserFn: func(_ context.Context, _ string) ([]byte, error) {
			return []byte(`{"id":"` + id + `","email":"staff@wso2.com","userType":"external","roles":["customer"]}`), nil
		},
	}, testDirectory(t), nil).WithAccessGuard(newAccessGuardForCSMRoles())

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
	}, testDirectory(t), nil).WithAccessGuard(NewAccessGuard(testAccessConfigForCSMRoles()))

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
	}, testDirectory(t), nil)

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
	}, testDirectory(t), nil).WithAccessGuard(NewAccessGuard(testAccessConfigForCSMRoles()))

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
