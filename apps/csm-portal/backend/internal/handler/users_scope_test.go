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

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/directory"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/scim"
)

// Tests for scoping user listings/profiles to staff accounts for callers
// outside user management (scopeUserSearchToStaff, withoutExternalAccessDetail).

func scopedUserSearch(t *testing.T, body string, asManager bool) (captured string, called bool, w *httptest.ResponseRecorder) {
	t.Helper()
	h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{
		searchUsersFn: func(_ context.Context, b []byte) ([]byte, error) {
			captured, called = string(b), true
			return []byte(`{"users":[],"total":0}`), nil
		},
	}, scopeTestDirectory(t), false, "").WithAccessGuard(NewAccessGuard(testAccessConfig()))
	r := httptest.NewRequest(http.MethodPost, "/users/search", strings.NewReader(body))
	if asManager {
		r = withCsEngineerUser(r)
	} else {
		r = withViewerUser(r)
	}
	w = httptest.NewRecorder()
	h.SearchUsers(w, r)
	return captured, called, w
}

func forwardedRoleIDs(t *testing.T, body string) []string {
	t.Helper()
	var req struct {
		Filters struct {
			RoleIDs []string `json:"roleIds"`
		} `json:"filters"`
	}
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatalf("forwarded body not JSON: %v (%s)", err, body)
	}
	return req.Filters.RoleIDs
}

func TestSearchUsers_ScopedToStaffOutsideUserManagement(t *testing.T) {
	t.Run("no roleIds: staff roles injected, other fields kept", func(t *testing.T) {
		captured, _, w := scopedUserSearch(t, `{"filters":{"searchQuery":"jane","active":true},"pagination":{"limit":10}}`, false)
		assertStatus(t, w, http.StatusOK)
		got := forwardedRoleIDs(t, captured)
		if strings.Join(got, ",") != "internal,agent,admin" {
			t.Fatalf("roleIds = %v, want the staff roles", got)
		}
		if !strings.Contains(captured, `"searchQuery":"jane"`) || !strings.Contains(captured, `"pagination":{"limit":10}`) {
			t.Fatalf("other fields lost: %s", captured)
		}
	})

	t.Run("empty and null bodies are scoped too", func(t *testing.T) {
		for _, body := range []string{`{}`, `null`, `{"filters":null}`} {
			captured, _, w := scopedUserSearch(t, body, false)
			assertStatus(t, w, http.StatusOK)
			if len(forwardedRoleIDs(t, captured)) == 0 {
				t.Fatalf("body %s forwarded unscoped: %s", body, captured)
			}
		}
	})

	t.Run("staff-only roleIds are forwarded as sent", func(t *testing.T) {
		const body = `{"filters":{"roleIds":["timecard_approver"],"active":true}}`
		captured, _, w := scopedUserSearch(t, body, false)
		assertStatus(t, w, http.StatusOK)
		if captured != body {
			t.Fatalf("body rewritten: %s", captured)
		}
	})

	t.Run("a non-staff role is refused without an upstream call", func(t *testing.T) {
		_, called, w := scopedUserSearch(t, `{"filters":{"roleIds":["agent","customer"]}}`, false)
		assertStatus(t, w, http.StatusForbidden)
		assertErrorMessage(t, w, ErrMsgForbidden)
		if called {
			t.Fatal("upstream must not be called")
		}
	})

	t.Run("user management caller is not scoped", func(t *testing.T) {
		const body = `{"filters":{"roleIds":["customer"]}}`
		captured, _, w := scopedUserSearch(t, body, true)
		assertStatus(t, w, http.StatusOK)
		if captured != body {
			t.Fatalf("manager body rewritten: %s", captured)
		}
	})

	t.Run("handler without a guard fails closed", func(t *testing.T) {
		var captured string
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{
			searchUsersFn: func(_ context.Context, b []byte) ([]byte, error) {
				captured = string(b)
				return []byte(`{"users":[]}`), nil
			},
		}, testDirectory(t), false, "")
		w := httptest.NewRecorder()
		h.SearchUsers(w, withCsEngineerUser(httptest.NewRequest(http.MethodPost, "/users/search", strings.NewReader(`{}`))))
		assertStatus(t, w, http.StatusOK)
		if len(forwardedRoleIDs(t, captured)) == 0 {
			t.Fatalf("unguarded handler forwarded unscoped body: %s", captured)
		}
	})
}

func TestGetUser_ExternalAccessDetailOnlyForUserManagement(t *testing.T) {
	const id = "11111111-1111-1111-1111-111111111111"
	external := `{"id":"` + id + `","email":"jane.doe@example.com","userType":"external","projectAccess":[{"projectId":"p-1","granted":true}]}`
	internal := `{"id":"` + id + `","email":"jane.doe@example.com","userType":"internal","projectAccess":[]}`

	newHandler := func(profile string, scimCalled *bool) *UsersHandler {
		locked := false
		return NewUsersHandler(&mockSCIMClient{
			searchExternalUserFn: func(context.Context, string) (*scim.ExternalUserInfo, error) {
				*scimCalled = true
				return &scim.ExternalUserInfo{Exists: true, Locked: &locked}, nil
			},
		}, &mockEntityUserClient{
			getUserFn: func(context.Context, string) ([]byte, error) { return []byte(profile), nil },
		}, testDirectory(t), false, "").WithAccessGuard(NewAccessGuard(testAccessConfig()))
	}
	get := func(h *UsersHandler, asManager bool) map[string]json.RawMessage {
		r := httptest.NewRequest(http.MethodGet, "/users/"+id, nil)
		r.SetPathValue("id", id)
		if asManager {
			r = withCsEngineerUser(r)
		} else {
			r = withViewerUser(r)
		}
		w := httptest.NewRecorder()
		h.GetUser(w, r)
		assertStatus(t, w, http.StatusOK)
		return decodeJSON[map[string]json.RawMessage](t, w)
	}

	t.Run("viewer gets an external profile without project access or account status", func(t *testing.T) {
		scimCalled := false
		got := get(newHandler(external, &scimCalled), false)
		if _, ok := got["projectAccess"]; ok {
			t.Fatal("projectAccess must be omitted")
		}
		if _, ok := got["externalAccount"]; ok {
			t.Fatal("externalAccount must be omitted")
		}
		if scimCalled {
			t.Fatal("no external account lookup for a non-manager")
		}
		if string(got["email"]) != `"jane.doe@example.com"` {
			t.Fatalf("profile fields lost: %v", got)
		}
	})

	t.Run("viewer gets an internal profile unchanged", func(t *testing.T) {
		scimCalled := false
		got := get(newHandler(internal, &scimCalled), false)
		if _, ok := got["projectAccess"]; !ok {
			t.Fatal("internal profile must be unchanged")
		}
	})

	t.Run("user management caller gets the full external profile", func(t *testing.T) {
		scimCalled := false
		got := get(newHandler(external, &scimCalled), true)
		if _, ok := got["projectAccess"]; !ok {
			t.Fatal("projectAccess must be kept")
		}
		if _, ok := got["externalAccount"]; !ok || !scimCalled {
			t.Fatal("externalAccount must be added")
		}
	})
}

// scopeTestDirectory is testDirectory with "customer" also on the role
// allow-list, so a search can name a non-staff role that passes directory
// validation and reaches the staff scoping.
func scopeTestDirectory(t *testing.T) *directory.Directory {
	t.Helper()
	teams, err := directory.ParseTeamRegistry(testTeamRegistry)
	if err != nil {
		t.Fatal(err)
	}
	roles, err := directory.ParseRoles(testRoles + ",customer")
	if err != nil {
		t.Fatal(err)
	}
	dir, err := directory.New(teams, roles)
	if err != nil {
		t.Fatal(err)
	}
	return dir
}
