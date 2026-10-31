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
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/directory"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/scim"
)

// ----- GetMe -----

func TestGetMe(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{}, testDirectory(t), false, nil)
		r := httptest.NewRequest(http.MethodGet, "/users/me", nil)
		w := httptest.NewRecorder()
		h.GetMe(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("never consults the identity provider for the phone", func(t *testing.T) {
		scimClient := &mockSCIMClient{
			searchUserFn: func(_ context.Context, _ string) (*scim.UserInfo, error) {
				t.Error("GET /users/me must not read the identity provider")
				return nil, errors.New("scim unavailable")
			},
		}
		h := NewUsersHandler(scimClient, &mockEntityUserClient{}, testDirectory(t), false, nil)
		r := withUser(httptest.NewRequest(http.MethodGet, "/users/me", nil))
		w := httptest.NewRecorder()
		h.GetMe(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		resp := decodeJSON[map[string]any](t, w)
		if resp["email"] != testUser.Email {
			t.Errorf("email = %v, want %q", resp["email"], testUser.Email)
		}
		if _, ok := resp["phoneNumber"]; ok {
			t.Error("phoneNumber should be absent when the platform record has none")
		}
	})

	t.Run("upstream errors from the entity service are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to fetch the current user.") {
			// "apierror 404" is excluded: GetMe deliberately maps a 404 to 403
			// instead of the generic table's 404->404 pass-through, asserted
			// in its own test below.
			if tc.name == "apierror 404" {
				continue
			}
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				entityClient := &mockEntityUserClient{
					getUserMeFn: func(_ context.Context) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewUsersHandler(&mockSCIMClient{}, entityClient, testDirectory(t), false, nil)
				r := withUser(httptest.NewRequest(http.MethodGet, "/users/me", nil))
				w := httptest.NewRecorder()
				h.GetMe(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})

	// Regression guard: a caller authenticated by a valid JWT whose email has
	// no "user" row at all (never provisioned downstream) made entity-service
	// 404 GetUserMe. Passing that 404 straight through used to leave the
	// webapp's profile-fetch hook with nothing to render and no 403 state to
	// fall into, spinning forever instead. GetMe now maps it to the same 403
	// the UI already knows how to show, while the real reason is still logged
	// server-side.
	t.Run("entity 404 (user not found) maps to 403, not 404", func(t *testing.T) {
		entityClient := &mockEntityUserClient{
			getUserMeFn: func(_ context.Context) ([]byte, error) {
				return nil, &apierror.Error{StatusCode: http.StatusNotFound}
			},
		}
		h := NewUsersHandler(&mockSCIMClient{}, entityClient, testDirectory(t), false, nil)
		r := withUser(httptest.NewRequest(http.MethodGet, "/users/me", nil))
		w := httptest.NewRecorder()
		h.GetMe(w, r)
		assertStatus(t, w, http.StatusForbidden)
		assertErrorMessage(t, w, ErrMsgForbidden)
		assertContentType(t, w, "application/json")
	})

	t.Run("entity 404 but the token grants a portal role answers from the token", func(t *testing.T) {
		entityClient := &mockEntityUserClient{
			getUserMeFn: func(_ context.Context) ([]byte, error) {
				return nil, &apierror.Error{StatusCode: http.StatusNotFound}
			},
		}
		cfg := testAccessConfig()
		h := NewUsersHandler(&mockSCIMClient{}, entityClient, testDirectory(t), false, nil).WithAccessGuard(NewAccessGuard(cfg))
		user := &middleware.UserInfo{
			Email: "agent@example.com", UserID: "u-1",
			FirstName: "Jane", LastName: "Doe",
			Roles: []string{"test-escalator"},
		}
		r := httptest.NewRequest(http.MethodGet, "/users/me", nil)
		r = r.WithContext(middleware.WithUserInfo(r.Context(), user))
		w := httptest.NewRecorder()
		h.GetMe(w, r)

		assertStatus(t, w, http.StatusOK)
		type getMeResp struct {
			ID        *string  `json:"id"`
			Email     string   `json:"email"`
			FirstName *string  `json:"firstName"`
			LastName  *string  `json:"lastName"`
			TimeZone  *string  `json:"timeZone"`
			Roles     []string `json:"roles"`
			Team      *string  `json:"team"`
		}
		resp := decodeJSON[getMeResp](t, w)
		if resp.Email != "agent@example.com" {
			t.Errorf("email = %q, want agent@example.com", resp.Email)
		}
		if resp.FirstName == nil || *resp.FirstName != "Jane" || resp.LastName == nil || *resp.LastName != "Doe" {
			t.Errorf("firstName/lastName = %v/%v, want Jane/Doe (from the token)", resp.FirstName, resp.LastName)
		}
		if joined(&resp.Roles) != "escalator" {
			t.Errorf("roles = %s, want escalator", joined(&resp.Roles))
		}
		if resp.ID != nil {
			t.Errorf("id = %v, want null: there is no entity-service user row to source it from", resp.ID)
		}
		if resp.TimeZone != nil {
			t.Errorf("timeZone = %v, want null", resp.TimeZone)
		}
		if resp.Team != nil {
			t.Errorf("team = %v, want null", resp.Team)
		}
	})

	t.Run("entity 404 but the token grants cs_engineer provisions a user row from the token", func(t *testing.T) {
		var createBody []byte
		var patchedID string
		var patchBody []byte
		entityClient := &mockEntityUserClient{
			getUserMeFn: func(_ context.Context) ([]byte, error) {
				return nil, &apierror.Error{StatusCode: http.StatusNotFound}
			},
			createUserFn: func(_ context.Context, body []byte) ([]byte, error) {
				createBody = body
				return []byte(`{"id":"11111111-1111-1111-1111-111111111111","email":"agent@example.com"}`), nil
			},
			updateUserFn: func(_ context.Context, id string, body []byte) ([]byte, error) {
				patchedID = id
				patchBody = body
				return []byte(`{}`), nil
			},
		}
		// SCIM must never be consulted for this path -- the caller IS the
		// person being provisioned, so their own token already has
		// everything needed (unlike the admin Add User flow, which has no
		// token for the person it's creating).
		scimClient := &mockSCIMClient{
			searchUserFn: func(context.Context, string) (*scim.UserInfo, error) {
				t.Fatal("SCIM must not be consulted when provisioning from the token")
				return nil, nil
			},
		}
		cfg := testAccessConfig()
		h := NewUsersHandler(scimClient, entityClient, testDirectory(t), false, nil).WithAccessGuard(NewAccessGuard(cfg))
		user := &middleware.UserInfo{
			Email: "agent@example.com", UserID: "u-1",
			FirstName: "Jane", LastName: "Doe", PhoneNumber: "+1-555-0100",
			Roles: []string{"test-cs-engineer"},
		}
		r := httptest.NewRequest(http.MethodGet, "/users/me", nil)
		r = r.WithContext(middleware.WithUserInfo(r.Context(), user))
		w := httptest.NewRecorder()
		h.GetMe(w, r)

		assertStatus(t, w, http.StatusOK)
		type getMeResp struct {
			ID          *string  `json:"id"`
			FirstName   *string  `json:"firstName"`
			LastName    *string  `json:"lastName"`
			PhoneNumber *string  `json:"phoneNumber"`
			Roles       []string `json:"roles"`
		}
		resp := decodeJSON[getMeResp](t, w)
		if resp.ID == nil || *resp.ID != "11111111-1111-1111-1111-111111111111" {
			t.Errorf("id = %v, want the newly created user's id", resp.ID)
		}
		if resp.FirstName == nil || *resp.FirstName != "Jane" {
			t.Errorf("firstName = %v, want Jane (from the token)", resp.FirstName)
		}
		if resp.LastName == nil || *resp.LastName != "Doe Ⓦ" {
			t.Errorf("lastName = %v, want %q", resp.LastName, "Doe Ⓦ")
		}
		if resp.PhoneNumber == nil || *resp.PhoneNumber != "+1-555-0100" {
			t.Errorf("phoneNumber = %v, want %q (from the token)", resp.PhoneNumber, "+1-555-0100")
		}
		if joined(&resp.Roles) != "cs_engineer" {
			t.Errorf("roles = %s, want cs_engineer", joined(&resp.Roles))
		}

		if createBody == nil {
			t.Fatal("entity CreateUser was never called")
		}
		var created struct {
			FirstName string   `json:"firstName"`
			LastName  string   `json:"lastName"`
			Email     string   `json:"email"`
			Roles     []string `json:"roles"`
		}
		if err := json.Unmarshal(createBody, &created); err != nil {
			t.Fatalf("decode CreateUser body: %v", err)
		}
		if created.FirstName != "Jane" || created.LastName != "Doe Ⓦ" || created.Email != "agent@example.com" {
			t.Errorf("CreateUser body = %+v, want firstName Jane, lastName \"Doe Ⓦ\", email agent@example.com", created)
		}
		if len(created.Roles) != 1 || created.Roles[0] != "internal" {
			t.Errorf("CreateUser roles = %v, want [internal]", created.Roles)
		}

		if patchedID != "11111111-1111-1111-1111-111111111111" {
			t.Errorf("UpdateUser id = %q, want the newly created user's id", patchedID)
		}
		var patched struct {
			Phone string `json:"phone"`
		}
		if err := json.Unmarshal(patchBody, &patched); err != nil {
			t.Fatalf("decode UpdateUser (phone) body: %v", err)
		}
		if patched.Phone != "+1-555-0100" {
			t.Errorf("UpdateUser phone = %q, want %q", patched.Phone, "+1-555-0100")
		}
	})

	t.Run("entity 404, cs_engineer, but the entity create fails: falls back to the token-only answer", func(t *testing.T) {
		entityClient := &mockEntityUserClient{
			getUserMeFn: func(_ context.Context) ([]byte, error) {
				return nil, &apierror.Error{StatusCode: http.StatusNotFound}
			},
			createUserFn: func(_ context.Context, _ []byte) ([]byte, error) {
				return nil, errors.New("entity unavailable")
			},
		}
		cfg := testAccessConfig()
		h := NewUsersHandler(&mockSCIMClient{}, entityClient, testDirectory(t), false, nil).WithAccessGuard(NewAccessGuard(cfg))
		user := &middleware.UserInfo{
			Email: "agent@example.com", UserID: "u-1",
			FirstName: "Jane", LastName: "Doe",
			Roles: []string{"test-cs-engineer"},
		}
		r := httptest.NewRequest(http.MethodGet, "/users/me", nil)
		r = r.WithContext(middleware.WithUserInfo(r.Context(), user))
		w := httptest.NewRecorder()
		h.GetMe(w, r)

		assertStatus(t, w, http.StatusOK)
		type getMeResp struct {
			ID        *string `json:"id"`
			FirstName *string `json:"firstName"`
			LastName  *string `json:"lastName"`
		}
		resp := decodeJSON[getMeResp](t, w)
		if resp.ID != nil {
			t.Errorf("id = %v, want null: no user row was created", resp.ID)
		}
		if resp.FirstName == nil || *resp.FirstName != "Jane" || resp.LastName == nil || *resp.LastName != "Doe" {
			t.Errorf("firstName/lastName = %v/%v, want Jane/Doe (from the token, unmarked)", resp.FirstName, resp.LastName)
		}
	})

	t.Run("returns email from JWT when SCIM returns no user", func(t *testing.T) {
		scimClient := &mockSCIMClient{
			searchUserFn: func(_ context.Context, _ string) (*scim.UserInfo, error) {
				return nil, nil // user not found in SCIM
			},
		}
		h := NewUsersHandler(scimClient, &mockEntityUserClient{}, testDirectory(t), false, nil)
		r := withUser(httptest.NewRequest(http.MethodGet, "/users/me", nil))
		w := httptest.NewRecorder()
		h.GetMe(w, r)

		assertStatus(t, w, http.StatusOK)
		resp := decodeJSON[map[string]any](t, w)
		if resp["email"] != testUser.Email {
			t.Errorf("email = %v, want %q", resp["email"], testUser.Email)
		}
	})

	t.Run("resolves the caller's team from their groups, with no extra upstream call", func(t *testing.T) {
		entityCalls := 0
		entityClient := &mockEntityUserClient{
			getUserMeFn: func(_ context.Context) ([]byte, error) {
				entityCalls++
				return []byte(`{"id":"u-1","email":"agent@example.com","lastName":"Doe","roles":["agent"],` +
					`"groups":[{"id":"g-9","name":"Some Other Group"},{"id":"g-1","name":"ABT One"}]}`), nil
			},
		}
		_ = entityCalls
		h := NewUsersHandler(&mockSCIMClient{}, entityClient, testDirectory(t), false, nil)
		r := withUser(httptest.NewRequest(http.MethodGet, "/users/me", nil))
		w := httptest.NewRecorder()
		h.GetMe(w, r)

		assertStatus(t, w, http.StatusOK)
		type getMeResp struct {
			Team *struct {
				TeamKey  string `json:"teamKey"`
				TeamName string `json:"teamName"`
				Family   string `json:"family"`
			} `json:"team"`
		}
		resp := decodeJSON[getMeResp](t, w)
		if resp.Team == nil {
			t.Fatal("team should be present")
		}
		if resp.Team.TeamKey != "abt-1" || resp.Team.TeamName != "ABT One" || resp.Team.Family != "cre-abt" {
			t.Errorf("team = %+v, want {abt-1 ABT One cre-abt}", resp.Team)
		}
		// One entity call, for the identity. The registry lookup that turned
		// "ABT One" into a team is a map read, not a second round trip.
		if entityCalls != 1 {
			t.Errorf("entity was called %d times, want exactly 1", entityCalls)
		}
	})

	t.Run("resolves a team whose registry row configured no family", func(t *testing.T) {
		entityClient := &mockEntityUserClient{
			getUserMeFn: func(_ context.Context) ([]byte, error) {
				return []byte(`{"id":"u-1","email":"agent@example.com","lastName":"Doe","roles":["agent"],` +
					`"groups":[{"id":"g-2","name":"ABT Two"}]}`), nil
			},
		}
		h := NewUsersHandler(&mockSCIMClient{}, entityClient, testDirectory(t), false, nil)
		r := withUser(httptest.NewRequest(http.MethodGet, "/users/me", nil))
		w := httptest.NewRecorder()
		h.GetMe(w, r)

		assertStatus(t, w, http.StatusOK)
		type getMeResp struct {
			Team *struct {
				TeamKey  string `json:"teamKey"`
				TeamName string `json:"teamName"`
				Family   string `json:"family"`
			} `json:"team"`
		}
		resp := decodeJSON[getMeResp](t, w)
		if resp.Team == nil {
			t.Fatal("team should be present")
		}
		if resp.Team.Family != "" {
			t.Errorf("family = %q, want empty string", resp.Team.Family)
		}
	})

	t.Run("omits team when the caller is in no registry team", func(t *testing.T) {
		entityClient := &mockEntityUserClient{
			getUserMeFn: func(_ context.Context) ([]byte, error) {
				return []byte(`{"id":"u-1","email":"agent@example.com","lastName":"Doe","roles":["agent"],` +
					`"groups":[{"id":"g-9","name":"Some Other Group"}]}`), nil
			},
		}
		h := NewUsersHandler(&mockSCIMClient{}, entityClient, testDirectory(t), false, nil)
		r := withUser(httptest.NewRequest(http.MethodGet, "/users/me", nil))
		w := httptest.NewRecorder()
		h.GetMe(w, r)

		assertStatus(t, w, http.StatusOK)
		resp := decodeJSON[map[string]any](t, w)
		if v, ok := resp["team"]; !ok {
			t.Error("team should be present (null, not omitted) when none of the caller's groups is a registry team")
		} else if v != nil {
			t.Errorf("team = %v, want null", v)
		}
	})

	t.Run("phoneNumber is the platform record's phone", func(t *testing.T) {
		entityClient := &mockEntityUserClient{
			getUserMeFn: func(_ context.Context) ([]byte, error) {
				return []byte(`{"id":"u-1","email":"agent@example.com","lastName":"Doe","phone":"+15550001111","roles":[],"groups":[]}`), nil
			},
		}
		h := NewUsersHandler(&mockSCIMClient{}, entityClient, testDirectory(t), false, nil)
		r := withUser(httptest.NewRequest(http.MethodGet, "/users/me", nil))
		w := httptest.NewRecorder()
		h.GetMe(w, r)

		assertStatus(t, w, http.StatusOK)
		resp := decodeJSON[map[string]any](t, w)
		if got := resp["phoneNumber"]; got != "+15550001111" {
			t.Errorf("phoneNumber = %v, want %q", got, "+15550001111")
		}
		if _, ok := resp["platformPhone"]; ok {
			t.Error("platformPhone is no longer part of the response")
		}
	})

	t.Run("phoneNumber is absent when the platform record has no phone", func(t *testing.T) {
		entityClient := &mockEntityUserClient{
			getUserMeFn: func(_ context.Context) ([]byte, error) {
				return []byte(`{"id":"u-1","email":"agent@example.com","lastName":"Doe","roles":[],"groups":[]}`), nil
			},
		}
		h := NewUsersHandler(&mockSCIMClient{}, entityClient, testDirectory(t), false, nil)
		r := withUser(httptest.NewRequest(http.MethodGet, "/users/me", nil))
		w := httptest.NewRecorder()
		h.GetMe(w, r)

		assertStatus(t, w, http.StatusOK)
		resp := decodeJSON[map[string]any](t, w)
		if v, ok := resp["phoneNumber"]; ok {
			t.Errorf("phoneNumber = %v, want the key absent", v)
		}
	})
}

// TestGetMeSftpgoAttachmentStorageEnabled verifies GET /users/me reports the
// backend's SFTPGO_ATTACHMENT_STORAGE_ENABLED runtime flag value exactly,
// both on and off, so the frontend can tell whether
// AttachmentStorageHandler's routes are reachable without probing them.
func TestGetMeSftpgoAttachmentStorageEnabled(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		t.Run(fmt.Sprintf("flag=%v", enabled), func(t *testing.T) {
			h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{}, testDirectory(t), enabled, nil)
			r := withUser(httptest.NewRequest(http.MethodGet, "/users/me", nil))
			w := httptest.NewRecorder()
			h.GetMe(w, r)

			assertStatus(t, w, http.StatusOK)
			type getMeResp struct {
				SftpgoAttachmentStorageEnabled bool `json:"sftpgoAttachmentStorageEnabled"`
			}
			resp := decodeJSON[getMeResp](t, w)
			if resp.SftpgoAttachmentStorageEnabled != enabled {
				t.Errorf("sftpgoAttachmentStorageEnabled = %v, want %v", resp.SftpgoAttachmentStorageEnabled, enabled)
			}
		})
	}
}

// meBody is the part of the GET /users/me response these tests inspect.
type meBody struct {
	Roles *[]string `json:"roles"`
}

// getMeAs calls GET /users/me for a caller whose token carries tokenRoles and
// whose email is email, against an entity service that reports entityBody.
func getMeAs(t *testing.T, h *UsersHandler, email string, tokenRoles []string) meBody {
	t.Helper()
	user := &middleware.UserInfo{Email: email, UserID: "u-1", Roles: tokenRoles}
	r := httptest.NewRequest(http.MethodGet, "/users/me", nil)
	r = r.WithContext(middleware.WithUserInfo(r.Context(), user))
	w := httptest.NewRecorder()
	h.GetMe(w, r)
	assertStatus(t, w, http.StatusOK)
	return decodeJSON[meBody](t, w)
}

func joined(l *[]string) string {
	if l == nil {
		return "<null>"
	}
	return strings.Join(*l, ",")
}

// TestGetMeRoles verifies GET /users/me reports the portal roles the caller's
// token roles grant, always as an array, and that a caller holding no portal
// role still gets a profile rather than an error.
func TestGetMeRoles(t *testing.T) {
	newHandler := func(t *testing.T, entity *mockEntityUserClient, cfg AccessConfig) *UsersHandler {
		return NewUsersHandler(&mockSCIMClient{}, entity, testDirectory(t), false, nil).WithAccessGuard(NewAccessGuard(cfg))
	}
	def := testAccessConfig()

	t.Run("reports the portal role the token role grants", func(t *testing.T) {
		resp := getMeAs(t, newHandler(t, &mockEntityUserClient{}, def), "agent@example.com", []string{"test-escalator"})
		if joined(resp.Roles) != "escalator" {
			t.Errorf("roles = %s, want escalator", joined(resp.Roles))
		}
	})

	t.Run("a caller can hold several roles, and unrelated token roles are ignored", func(t *testing.T) {
		resp := getMeAs(t, newHandler(t, &mockEntityUserClient{}, def), "agent@example.com",
			[]string{"test-cs-engineer", "test-usage-metrics-viewer", "wso2-everyone"})
		if joined(resp.Roles) != "cs_engineer,usage_metrics_viewer" {
			t.Errorf("roles = %s, want cs_engineer,usage_metrics_viewer", joined(resp.Roles))
		}
	})

	t.Run("the entity service's own roles are not reported", func(t *testing.T) {
		entity := &mockEntityUserClient{
			getUserMeFn: func(_ context.Context) ([]byte, error) {
				return []byte(`{"id":"u-1","email":"agent@example.com","lastName":"Doe","roles":["internal","admin"]}`), nil
			},
		}
		resp := getMeAs(t, newHandler(t, entity, def), "agent@example.com", []string{"test-viewer"})
		if joined(resp.Roles) != "viewer" {
			t.Errorf("roles = %s, want only viewer: the entity roles must not leak in", joined(resp.Roles))
		}
	})

	t.Run("the dashboard designer role is reported", func(t *testing.T) {
		resp := getMeAs(t, newHandler(t, &mockEntityUserClient{}, def), "agent@example.com", []string{"test-dashboard-designer"})
		if joined(resp.Roles) != "dashboard_designer" {
			t.Errorf("roles = %s, want dashboard_designer", joined(resp.Roles))
		}
	})

	t.Run("a caller holding no portal role gets an empty array, not null", func(t *testing.T) {
		resp := getMeAs(t, newHandler(t, &mockEntityUserClient{}, def), "agent@example.com", []string{"wso2-everyone"})
		if resp.Roles == nil || len(*resp.Roles) != 0 {
			t.Errorf("roles = %s, want an empty non-null array", joined(resp.Roles))
		}
	})

	t.Run("no guard wired reports an empty array, not null", func(t *testing.T) {
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{}, testDirectory(t), false, nil)
		resp := getMeAs(t, h, "agent@example.com", []string{"test-admin"})
		if resp.Roles == nil || len(*resp.Roles) != 0 {
			t.Errorf("roles = %s, want an empty non-null array", joined(resp.Roles))
		}
	})

	t.Run("honours configured role names", func(t *testing.T) {
		cfg := testAccessConfig()
		cfg.Admin = []string{"corp-csm-admins"}
		resp := getMeAs(t, newHandler(t, &mockEntityUserClient{}, cfg), "agent@example.com", []string{"corp-csm-admins"})
		if joined(resp.Roles) != "admin" {
			t.Errorf("roles = %s, want admin", joined(resp.Roles))
		}
	})
}

// ----- PatchMe -----

func TestPatchMe(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{}, testDirectory(t), false, nil)
		r := httptest.NewRequest(http.MethodPatch, "/users/me", strings.NewReader(`{"phoneNumber":"+1"}`))
		w := httptest.NewRecorder()
		h.PatchMe(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{}, testDirectory(t), false, nil)
		r := withUser(httptest.NewRequest(http.MethodPatch, "/users/me", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		w := httptest.NewRecorder()
		h.PatchMe(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
	})

	t.Run("rejects empty body", func(t *testing.T) {
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{}, testDirectory(t), false, nil)
		r := withUser(httptest.NewRequest(http.MethodPatch, "/users/me", strings.NewReader("")))
		w := httptest.NewRecorder()
		h.PatchMe(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, "At least one field must be provided for update.")
	})

	t.Run("rejects invalid JSON", func(t *testing.T) {
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{}, testDirectory(t), false, nil)
		r := withUser(httptest.NewRequest(http.MethodPatch, "/users/me", strings.NewReader(`not-json`)))
		w := httptest.NewRecorder()
		h.PatchMe(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
	})

	t.Run("rejects JSON with no updateable fields", func(t *testing.T) {
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{}, testDirectory(t), false, nil)
		r := withUser(httptest.NewRequest(http.MethodPatch, "/users/me", strings.NewReader(`{}`)))
		w := httptest.NewRecorder()
		h.PatchMe(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, "At least one field must be provided for update.")
	})

	t.Run("phone is written to the entity service only and echoed back", func(t *testing.T) {
		var entityBody []byte
		scimClient := &mockSCIMClient{
			updateUserPhoneFn: func(_ context.Context, _, _ string) (*string, error) {
				t.Error("PATCH /users/me must not write the identity provider")
				return nil, nil
			},
		}
		entityClient := &mockEntityUserClient{
			patchUserMeFn: func(_ context.Context, body []byte) ([]byte, error) {
				entityBody = body
				return []byte(`{}`), nil
			},
		}
		h := NewUsersHandler(scimClient, entityClient, testDirectory(t), false, nil)
		r := withUser(httptest.NewRequest(http.MethodPatch, "/users/me", strings.NewReader(`{"phoneNumber":"+94777654321"}`)))
		w := httptest.NewRecorder()
		h.PatchMe(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		if string(entityBody) != `{"phone":"+94777654321"}` {
			t.Errorf("entity body = %s", entityBody)
		}
		resp := decodeJSON[struct {
			PhoneNumber *string `json:"phoneNumber"`
		}](t, w)
		if resp.PhoneNumber == nil || *resp.PhoneNumber != "+94777654321" {
			t.Errorf("phoneNumber = %v, want %q", resp.PhoneNumber, "+94777654321")
		}
	})

	t.Run("empty phone clears via entity", func(t *testing.T) {
		var entityBody []byte
		entityClient := &mockEntityUserClient{
			patchUserMeFn: func(_ context.Context, body []byte) ([]byte, error) {
				entityBody = body
				return []byte(`{}`), nil
			},
		}
		h := NewUsersHandler(&mockSCIMClient{}, entityClient, testDirectory(t), false, nil)
		r := withUser(httptest.NewRequest(http.MethodPatch, "/users/me", strings.NewReader(`{"phoneNumber":""}`)))
		w := httptest.NewRecorder()
		h.PatchMe(w, r)
		assertStatus(t, w, http.StatusOK)
		if string(entityBody) != `{"phone":""}` {
			t.Errorf("entity body = %s", entityBody)
		}
	})

	t.Run("phone is trimmed before it is stored and echoed back", func(t *testing.T) {
		for name, tc := range map[string]struct{ in, stored string }{
			"padded": {" +15555550123 ", "+15555550123"},
			"blank":  {"   ", ""},
		} {
			t.Run(name, func(t *testing.T) {
				var entityBody []byte
				entityClient := &mockEntityUserClient{
					patchUserMeFn: func(_ context.Context, body []byte) ([]byte, error) {
						entityBody = body
						return []byte(`{}`), nil
					},
				}
				h := NewUsersHandler(&mockSCIMClient{}, entityClient, testDirectory(t), false, nil)
				r := withUser(httptest.NewRequest(http.MethodPatch, "/users/me", strings.NewReader(`{"phoneNumber":"`+tc.in+`"}`)))
				w := httptest.NewRecorder()
				h.PatchMe(w, r)
				assertStatus(t, w, http.StatusOK)
				if want := `{"phone":"` + tc.stored + `"}`; string(entityBody) != want {
					t.Errorf("entity body = %s, want %s", entityBody, want)
				}
				resp := decodeJSON[struct {
					PhoneNumber *string `json:"phoneNumber"`
				}](t, w)
				if resp.PhoneNumber == nil || *resp.PhoneNumber != tc.stored {
					t.Errorf("phoneNumber = %v, want %q", resp.PhoneNumber, tc.stored)
				}
			})
		}
	})

	t.Run("phone and timeZone share one entity call", func(t *testing.T) {
		calls := 0
		var entityBody []byte
		entityClient := &mockEntityUserClient{
			patchUserMeFn: func(_ context.Context, body []byte) ([]byte, error) {
				calls++
				entityBody = body
				return []byte(`{}`), nil
			},
		}
		h := NewUsersHandler(&mockSCIMClient{}, entityClient, testDirectory(t), false, nil)
		r := withUser(httptest.NewRequest(http.MethodPatch, "/users/me", strings.NewReader(`{"phoneNumber":"+15555550123","timeZone":"UTC"}`)))
		w := httptest.NewRecorder()
		h.PatchMe(w, r)
		assertStatus(t, w, http.StatusOK)
		if calls != 1 {
			t.Errorf("entity calls = %d, want 1", calls)
		}
		if string(entityBody) != `{"phone":"+15555550123","timeZone":"UTC"}` {
			t.Errorf("entity body = %s", entityBody)
		}
		resp := decodeJSON[struct {
			TimeZone *string `json:"timeZone"`
		}](t, w)
		if resp.TimeZone == nil || *resp.TimeZone != "UTC" {
			t.Errorf("timeZone = %v", resp.TimeZone)
		}
	})

	t.Run("timeZone only does not touch the phone or the identity provider", func(t *testing.T) {
		scimCalled := false
		scimClient := &mockSCIMClient{
			updateUserPhoneFn: func(_ context.Context, _, _ string) (*string, error) {
				scimCalled = true
				return nil, nil
			},
		}
		var entityBody []byte
		entityClient := &mockEntityUserClient{
			patchUserMeFn: func(_ context.Context, body []byte) ([]byte, error) {
				entityBody = body
				return []byte(`{}`), nil
			},
		}
		h := NewUsersHandler(scimClient, entityClient, testDirectory(t), false, nil)
		r := withUser(httptest.NewRequest(http.MethodPatch, "/users/me", strings.NewReader(`{"timeZone":"UTC"}`)))
		w := httptest.NewRecorder()
		h.PatchMe(w, r)
		assertStatus(t, w, http.StatusOK)
		if scimCalled {
			t.Error("SCIM must not be called for timeZone only")
		}
		if string(entityBody) != `{"timeZone":"UTC"}` {
			t.Errorf("entity body = %s", entityBody)
		}
	})

	t.Run("rejects empty or blank timeZone without upstream calls", func(t *testing.T) {
		for name, body := range map[string]string{
			"empty with phone": `{"phoneNumber":"+15555550123","timeZone":""}`,
			"empty alone":      `{"timeZone":""}`,
			"whitespace only":  `{"timeZone":"   "}`,
		} {
			t.Run(name, func(t *testing.T) {
				called := false
				scimClient := &mockSCIMClient{
					updateUserPhoneFn: func(_ context.Context, _, _ string) (*string, error) {
						called = true
						return nil, nil
					},
				}
				entityClient := &mockEntityUserClient{
					patchUserMeFn: func(_ context.Context, _ []byte) ([]byte, error) {
						called = true
						return []byte(`{}`), nil
					},
				}
				h := NewUsersHandler(scimClient, entityClient, testDirectory(t), false, nil)
				r := withUser(httptest.NewRequest(http.MethodPatch, "/users/me", strings.NewReader(body)))
				w := httptest.NewRecorder()
				h.PatchMe(w, r)
				assertStatus(t, w, http.StatusBadRequest)
				assertErrorMessage(t, w, "timeZone must not be empty.")
				if called {
					t.Error("no SCIM or entity call expected")
				}
			})
		}
	})

	t.Run("entity errors are mapped", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to update phone number.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				entityClient := &mockEntityUserClient{
					patchUserMeFn: func(_ context.Context, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewUsersHandler(&mockSCIMClient{}, entityClient, testDirectory(t), false, nil)
				r := withUser(httptest.NewRequest(http.MethodPatch, "/users/me", strings.NewReader(`{"phoneNumber":"+15555550123"}`)))
				w := httptest.NewRecorder()
				h.PatchMe(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
			})
		}
	})
}

// ----- UpdateUser -----

func TestUpdateUser(t *testing.T) {
	const targetID = "11111111-1111-1111-1111-111111111111"

	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{}, testDirectory(t), false, nil)
		r := httptest.NewRequest(http.MethodPatch, "/users/"+targetID, strings.NewReader(`{"phone":"+1"}`))
		r.SetPathValue("id", targetID)
		w := httptest.NewRecorder()
		h.UpdateUser(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
	})

	t.Run("rejects a non-UUID id", func(t *testing.T) {
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{}, testDirectory(t), false, nil)
		r := withUser(httptest.NewRequest(http.MethodPatch, "/users/not-a-uuid", strings.NewReader(`{"phone":"+1"}`)))
		r.SetPathValue("id", "not-a-uuid")
		w := httptest.NewRecorder()
		h.UpdateUser(w, r)
		assertStatus(t, w, http.StatusBadRequest)
	})

	t.Run("rejects a body with neither field", func(t *testing.T) {
		entity := &mockEntityUserClient{}
		h := NewUsersHandler(&mockSCIMClient{}, entity, testDirectory(t), false, nil)
		r := withUser(httptest.NewRequest(http.MethodPatch, "/users/"+targetID, strings.NewReader(`{}`)))
		r.SetPathValue("id", targetID)
		w := httptest.NewRecorder()
		h.UpdateUser(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, "At least one field must be provided for update.")
	})

	t.Run("forwards the given id and phone to entity, not the caller's own id", func(t *testing.T) {
		var capturedID string
		var capturedBody []byte
		entity := &mockEntityUserClient{
			updateUserFn: func(_ context.Context, id string, body []byte) ([]byte, error) {
				capturedID = id
				capturedBody = body
				return []byte(`{"message":"User updated successfully","user":{"id":"` + id + `"}}`), nil
			},
		}
		h := NewUsersHandler(&mockSCIMClient{}, entity, testDirectory(t), false, nil)
		r := withUser(httptest.NewRequest(http.MethodPatch, "/users/"+targetID, strings.NewReader(`{"phone":"+94777654321"}`)))
		r.SetPathValue("id", targetID)
		w := httptest.NewRecorder()
		h.UpdateUser(w, r)
		assertStatus(t, w, http.StatusOK)
		if capturedID != targetID {
			t.Errorf("entity called with id %q, want %q (the path id, not the caller's own)", capturedID, targetID)
		}
		if !strings.Contains(string(capturedBody), "+94777654321") {
			t.Errorf("body = %s, want it to carry the phone number", capturedBody)
		}
	})

	t.Run("forwards timeZone alongside or instead of phone", func(t *testing.T) {
		var capturedBody []byte
		entity := &mockEntityUserClient{
			updateUserFn: func(_ context.Context, id string, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"message":"User updated successfully","user":{"id":"` + id + `"}}`), nil
			},
		}
		h := NewUsersHandler(&mockSCIMClient{}, entity, testDirectory(t), false, nil)
		r := withUser(httptest.NewRequest(http.MethodPatch, "/users/"+targetID, strings.NewReader(`{"timeZone":"Asia/Colombo"}`)))
		r.SetPathValue("id", targetID)
		w := httptest.NewRecorder()
		h.UpdateUser(w, r)
		assertStatus(t, w, http.StatusOK)
		if !strings.Contains(string(capturedBody), "Asia/Colombo") {
			t.Errorf("body = %s, want it to carry the time zone", capturedBody)
		}
		if strings.Contains(string(capturedBody), "phone") {
			t.Errorf("body = %s, want no phone key when none was sent", capturedBody)
		}
	})

	t.Run("surfaces an upstream validation error", func(t *testing.T) {
		entity := &mockEntityUserClient{
			updateUserFn: func(context.Context, string, []byte) ([]byte, error) {
				return nil, &apierror.Error{StatusCode: http.StatusBadRequest, Body: `{"message":"phone must be at most 32 characters"}`}
			},
		}
		h := NewUsersHandler(&mockSCIMClient{}, entity, testDirectory(t), false, nil)
		r := withUser(httptest.NewRequest(http.MethodPatch, "/users/"+targetID, strings.NewReader(`{"phone":"`+strings.Repeat("1", 40)+`"}`)))
		r.SetPathValue("id", targetID)
		w := httptest.NewRecorder()
		h.UpdateUser(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, "phone must be at most 32 characters")
	})
}

// ----- SearchUsers -----

func TestSearchUsers(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{}, testDirectory(t), false, nil)
		r := httptest.NewRequest(http.MethodPost, "/users/search", strings.NewReader(`{}`))
		w := httptest.NewRecorder()
		h.SearchUsers(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{}, testDirectory(t), false, nil)
		r := withUser(httptest.NewRequest(http.MethodPost, "/users/search", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		w := httptest.NewRecorder()
		h.SearchUsers(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{}, testDirectory(t), false, nil)
		r := withUser(httptest.NewRequest(http.MethodPost, "/users/search", strings.NewReader(`not-json`)))
		w := httptest.NewRecorder()
		h.SearchUsers(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
	})

	t.Run("forwards body to upstream and returns 200 with response", func(t *testing.T) {
		const reqPayload = `{"email":"agent@example.com"}`
		var capturedBody []byte
		entityClient := &mockEntityUserClient{
			searchUsersFn: func(_ context.Context, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"users":[{"id":"u-1"}],"total":1}`), nil
			},
		}
		h := NewUsersHandler(&mockSCIMClient{}, entityClient, testDirectory(t), false, nil)
		r := withUser(httptest.NewRequest(http.MethodPost, "/users/search", strings.NewReader(reqPayload)))
		w := httptest.NewRecorder()
		h.SearchUsers(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		if string(capturedBody) != reqPayload {
			t.Errorf("upstream received body %q, want %q", capturedBody, reqPayload)
		}
		resp := decodeJSON[map[string]any](t, w)
		if resp["total"] != float64(1) {
			t.Errorf("total = %v, want 1", resp["total"])
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to search users.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				entityClient := &mockEntityUserClient{
					searchUsersFn: func(_ context.Context, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewUsersHandler(&mockSCIMClient{}, entityClient, testDirectory(t), false, nil)
				r := withUser(httptest.NewRequest(http.MethodPost, "/users/search", strings.NewReader(`{}`)))
				w := httptest.NewRecorder()
				h.SearchUsers(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

// ----- CreateUser -----
//
// Route-level admin-only enforcement (PermAdmin) is covered in access_test.go;
// these tests cover the handler's own request validation and upstream forwarding.
func TestCreateUser(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{}, testDirectory(t), false, nil)
		r := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{}`))
		w := httptest.NewRecorder()
		h.CreateUser(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{}, testDirectory(t), false, nil)
		r := withUser(httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		w := httptest.NewRecorder()
		h.CreateUser(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{}, testDirectory(t), false, nil)
		r := withUser(httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`not-json`)))
		w := httptest.NewRecorder()
		h.CreateUser(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
	})

	t.Run("rejects a role not in the directory's allow-list, before reaching upstream", func(t *testing.T) {
		called := false
		entityClient := &mockEntityUserClient{
			createUserFn: func(context.Context, []byte) ([]byte, error) {
				called = true
				return []byte(`{}`), nil
			},
		}
		h := NewUsersHandler(&mockSCIMClient{}, entityClient, testDirectory(t), false, nil)
		r := withUser(httptest.NewRequest(http.MethodPost, "/users",
			strings.NewReader(`{"firstName":"Jane","email":"jane@example.com","roles":["not-a-real-role"]}`)))
		w := httptest.NewRecorder()
		h.CreateUser(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		if called {
			t.Fatal("entity service must not be called for an invalid role")
		}
	})

	t.Run("enforces the internal-email and external-type-disabled constraints, before reaching upstream", func(t *testing.T) {
		teams, err := directory.ParseTeamRegistry(testTeamRegistry)
		if err != nil {
			t.Fatalf("ParseTeamRegistry: %v", err)
		}
		roles, err := directory.ParseRoles("internal,external,admin,agent,partner,customer,partner_admin,customer_admin")
		if err != nil {
			t.Fatalf("ParseRoles: %v", err)
		}
		dir, err := directory.New(teams, roles)
		if err != nil {
			t.Fatalf("directory.New: %v", err)
		}
		for _, role := range []string{"internal", "admin"} {
			t.Run(role, func(t *testing.T) {
				called := false
				entityClient := &mockEntityUserClient{
					createUserFn: func(context.Context, []byte) ([]byte, error) {
						called = true
						return []byte(`{}`), nil
					},
				}
				h := NewUsersHandler(&mockSCIMClient{}, entityClient, dir, false, nil)
				r := withUser(httptest.NewRequest(http.MethodPost, "/users",
					strings.NewReader(`{"firstName":"Jane","email":"jane@example.com","roles":["`+role+`"]}`)))
				w := httptest.NewRecorder()
				h.CreateUser(w, r)
				assertStatus(t, w, http.StatusBadRequest)
				if called {
					t.Fatal("entity service must not be called when an internal-resolving role is requested for a non-wso2.com email")
				}
			})
		}

		t.Run("allows the same role for a wso2.com email", func(t *testing.T) {
			entityClient := &mockEntityUserClient{
				createUserFn: func(context.Context, []byte) ([]byte, error) {
					return []byte(`{"id":"u-1"}`), nil
				},
			}
			h := NewUsersHandler(&mockSCIMClient{}, entityClient, dir, false, nil)
			r := withUser(httptest.NewRequest(http.MethodPost, "/users",
				strings.NewReader(`{"firstName":"Jane","email":"jane@wso2.com","roles":["internal"]}`)))
			w := httptest.NewRecorder()
			h.CreateUser(w, r)
			assertStatus(t, w, http.StatusCreated)
		})

		t.Run("allows a non-wso2.com email for a role that resolves to neither internal nor external", func(t *testing.T) {
			entityClient := &mockEntityUserClient{
				createUserFn: func(context.Context, []byte) ([]byte, error) {
					return []byte(`{"id":"u-1"}`), nil
				},
			}
			h := NewUsersHandler(&mockSCIMClient{}, entityClient, dir, false, nil)
			r := withUser(httptest.NewRequest(http.MethodPost, "/users",
				strings.NewReader(`{"firstName":"Jane","email":"jane@example.com","roles":["agent"]}`)))
			w := httptest.NewRecorder()
			h.CreateUser(w, r)
			assertStatus(t, w, http.StatusCreated)
		})

		t.Run("rejects an external-resolving role -- creating an external-type user is temporarily disabled", func(t *testing.T) {
			for _, role := range []string{"external", "partner", "customer", "partner_admin", "customer_admin"} {
				t.Run(role, func(t *testing.T) {
					called := false
					entityClient := &mockEntityUserClient{
						createUserFn: func(context.Context, []byte) ([]byte, error) {
							called = true
							return []byte(`{}`), nil
						},
					}
					h := NewUsersHandler(&mockSCIMClient{}, entityClient, dir, false, nil)
					r := withUser(httptest.NewRequest(http.MethodPost, "/users",
						strings.NewReader(`{"firstName":"Jane","email":"jane@example.com","roles":["`+role+`"]}`)))
					w := httptest.NewRecorder()
					h.CreateUser(w, r)
					assertStatus(t, w, http.StatusBadRequest)
					if called {
						t.Fatal("entity service must not be called when an external-resolving role is requested")
					}
				})
			}
		})
	})

	t.Run("rejects grantRoles for a non-wso2.com email, before reaching upstream", func(t *testing.T) {
		called := false
		entityClient := &mockEntityUserClient{
			createUserFn: func(context.Context, []byte) ([]byte, error) {
				called = true
				return []byte(`{}`), nil
			},
		}
		h := NewUsersHandler(&mockSCIMClient{}, entityClient, testDirectory(t), false, nil)
		r := withUser(httptest.NewRequest(http.MethodPost, "/users",
			strings.NewReader(`{"firstName":"Jane","email":"jane@example.com","grantRoles":["cs_engineer"]}`)))
		w := httptest.NewRecorder()
		h.CreateUser(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		if called {
			t.Fatal("entity service must not be called when grantRoles is requested for a non-wso2.com email")
		}
	})

	t.Run("allows grantRoles for a wso2.com email", func(t *testing.T) {
		var grantedRoleID string
		var grantedEmails []string
		entityClient := &mockEntityUserClient{
			createUserFn: func(context.Context, []byte) ([]byte, error) {
				return []byte(`{"id":"u-1","email":"jane@wso2.com"}`), nil
			},
		}
		scimClient := &mockSCIMClient{
			addRoleMembersFn: func(_ context.Context, roleID string, emails []string) error {
				grantedRoleID = roleID
				grantedEmails = emails
				return nil
			},
		}
		h := NewUsersHandler(scimClient, entityClient, testDirectory(t), false, nil).
			WithGrantableRoles([]GrantableRole{{Key: "cs_engineer", RoleID: "role-1"}})
		r := withUser(httptest.NewRequest(http.MethodPost, "/users",
			strings.NewReader(`{"firstName":"Jane","email":"jane@wso2.com","grantRoles":["cs_engineer"]}`)))
		w := httptest.NewRecorder()
		h.CreateUser(w, r)
		assertStatus(t, w, http.StatusCreated)
		if grantedRoleID != "role-1" || len(grantedEmails) != 1 || grantedEmails[0] != "jane@wso2.com" {
			t.Errorf("AddRoleMembers called with roleID=%q emails=%v, want role-1 [jane@wso2.com]", grantedRoleID, grantedEmails)
		}
	})

	t.Run("forwards the body unchanged and returns 201 with the upstream response", func(t *testing.T) {
		const reqPayload = `{"firstName":"Jane","lastName":"Doe","email":"jane.doe@example.com","roles":["agent"]}`
		var capturedBody []byte
		entityClient := &mockEntityUserClient{
			createUserFn: func(_ context.Context, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"id":"u-1","email":"jane.doe@example.com"}`), nil
			},
		}
		h := NewUsersHandler(&mockSCIMClient{}, entityClient, testDirectory(t), false, nil)
		r := withUser(httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(reqPayload)))
		w := httptest.NewRecorder()
		h.CreateUser(w, r)

		assertStatus(t, w, http.StatusCreated)
		assertContentType(t, w, "application/json")
		if string(capturedBody) != reqPayload {
			t.Errorf("upstream received body %q, want %q (unchanged)", capturedBody, reqPayload)
		}
		resp := decodeJSON[map[string]any](t, w)
		if resp["id"] != "u-1" {
			t.Errorf("id = %v, want u-1", resp["id"])
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to create the user.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				entityClient := &mockEntityUserClient{
					createUserFn: func(context.Context, []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewUsersHandler(&mockSCIMClient{}, entityClient, testDirectory(t), false, nil)
				r := withUser(httptest.NewRequest(http.MethodPost, "/users",
					strings.NewReader(`{"firstName":"Jane","email":"jane@example.com"}`)))
				w := httptest.NewRecorder()
				h.CreateUser(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})

	// The Add User form only collects an email; a request with no firstName
	// AND no lastName at all (not merely empty strings on the wire, since
	// JSON omission decodes to "" the same way) means the UI expects this
	// handler to source the rest from SCIM, mirroring provisionCsEngineerUser.
	t.Run("an email-only request fills firstName/lastName/phone from SCIM", func(t *testing.T) {
		phone := "+1-555-0100"
		var capturedCreate []byte
		var patchedID string
		var patchedBody []byte
		entityClient := &mockEntityUserClient{
			createUserFn: func(_ context.Context, body []byte) ([]byte, error) {
				capturedCreate = body
				return []byte(`{"id":"11111111-1111-1111-1111-111111111111","email":"jane.doe@wso2.com"}`), nil
			},
			updateUserFn: func(_ context.Context, id string, body []byte) ([]byte, error) {
				patchedID = id
				patchedBody = body
				return []byte(`{}`), nil
			},
		}
		scimClient := &mockSCIMClient{
			searchUserFn: func(_ context.Context, email string) (*scim.UserInfo, error) {
				if email != "jane.doe@wso2.com" {
					t.Errorf("SearchUser email = %q, want jane.doe@wso2.com", email)
				}
				return &scim.UserInfo{FirstName: "Jane", LastName: "Doe", PhoneNumber: &phone}, nil
			},
		}
		h := NewUsersHandler(scimClient, entityClient, testDirectory(t), false, nil)
		r := withUser(httptest.NewRequest(http.MethodPost, "/users",
			strings.NewReader(`{"email":"jane.doe@wso2.com","roles":["agent"]}`)))
		w := httptest.NewRecorder()
		h.CreateUser(w, r)

		assertStatus(t, w, http.StatusCreated)
		if capturedCreate == nil {
			t.Fatal("entity CreateUser was never called")
		}
		var created struct {
			FirstName string `json:"firstName"`
			LastName  string `json:"lastName"`
			Email     string `json:"email"`
		}
		if err := json.Unmarshal(capturedCreate, &created); err != nil {
			t.Fatalf("decode CreateUser body: %v", err)
		}
		if created.FirstName != "Jane" || created.LastName != "Doe" || created.Email != "jane.doe@wso2.com" {
			t.Errorf("CreateUser body = %+v, want firstName Jane, lastName Doe, email jane.doe@wso2.com", created)
		}

		if patchedID != "11111111-1111-1111-1111-111111111111" {
			t.Errorf("UpdateUser id = %q, want the newly created user's id", patchedID)
		}
		var patched struct {
			Phone string `json:"phone"`
		}
		if err := json.Unmarshal(patchedBody, &patched); err != nil {
			t.Fatalf("decode UpdateUser (phone) body: %v", err)
		}
		if patched.Phone != phone {
			t.Errorf("UpdateUser phone = %q, want %q", patched.Phone, phone)
		}
	})

	t.Run("an email-only request with no SCIM match is rejected, not forwarded upstream", func(t *testing.T) {
		called := false
		entityClient := &mockEntityUserClient{
			createUserFn: func(context.Context, []byte) ([]byte, error) {
				called = true
				return []byte(`{}`), nil
			},
		}
		scimClient := &mockSCIMClient{
			searchUserFn: func(_ context.Context, _ string) (*scim.UserInfo, error) {
				return nil, nil
			},
		}
		h := NewUsersHandler(scimClient, entityClient, testDirectory(t), false, nil)
		r := withUser(httptest.NewRequest(http.MethodPost, "/users",
			strings.NewReader(`{"email":"unknown@wso2.com"}`)))
		w := httptest.NewRecorder()
		h.CreateUser(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		if called {
			t.Fatal("entity service must not be called when SCIM has no matching user")
		}
	})

	t.Run("an email-only request is rejected when the SCIM search itself fails", func(t *testing.T) {
		called := false
		entityClient := &mockEntityUserClient{
			createUserFn: func(context.Context, []byte) ([]byte, error) {
				called = true
				return []byte(`{}`), nil
			},
		}
		scimClient := &mockSCIMClient{
			searchUserFn: func(_ context.Context, _ string) (*scim.UserInfo, error) {
				return nil, errors.New("scim unavailable")
			},
		}
		h := NewUsersHandler(scimClient, entityClient, testDirectory(t), false, nil)
		r := withUser(httptest.NewRequest(http.MethodPost, "/users",
			strings.NewReader(`{"email":"jane.doe@wso2.com"}`)))
		w := httptest.NewRecorder()
		h.CreateUser(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		if called {
			t.Fatal("entity service must not be called when the SCIM search fails")
		}
	})
}

// ----- GrantUserRoles -----

func TestGrantUserRoles(t *testing.T) {
	targetID := "11111111-1111-1111-1111-111111111111"
	grantable := []GrantableRole{{Key: "cs_engineer", RoleID: "role-cs-engineer"}, {Key: "escalator", RoleID: "role-escalator"}}

	newHandler := func(scimClient *mockSCIMClient, entityClient *mockEntityUserClient) *UsersHandler {
		return NewUsersHandler(scimClient, entityClient, testDirectory(t), false, nil).WithGrantableRoles(grantable)
	}

	t.Run("requires authenticated user", func(t *testing.T) {
		h := newHandler(&mockSCIMClient{}, &mockEntityUserClient{})
		r := httptest.NewRequest(http.MethodPost, "/users/"+targetID+"/roles", strings.NewReader(`{"roles":["cs_engineer"]}`))
		r.SetPathValue("id", targetID)
		w := httptest.NewRecorder()
		h.GrantUserRoles(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
	})

	t.Run("rejects a malformed id", func(t *testing.T) {
		h := newHandler(&mockSCIMClient{}, &mockEntityUserClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/users/not-a-uuid/roles", strings.NewReader(`{"roles":["cs_engineer"]}`)))
		r.SetPathValue("id", "not-a-uuid")
		w := httptest.NewRecorder()
		h.GrantUserRoles(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := newHandler(&mockSCIMClient{}, &mockEntityUserClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/users/"+targetID+"/roles", strings.NewReader(`not-json`)))
		r.SetPathValue("id", targetID)
		w := httptest.NewRecorder()
		h.GrantUserRoles(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
	})

	t.Run("rejects an empty roles list", func(t *testing.T) {
		h := newHandler(&mockSCIMClient{}, &mockEntityUserClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/users/"+targetID+"/roles", strings.NewReader(`{"roles":[]}`)))
		r.SetPathValue("id", targetID)
		w := httptest.NewRecorder()
		h.GrantUserRoles(w, r)
		assertStatus(t, w, http.StatusBadRequest)
	})

	t.Run("rejects an unknown role key before looking up the target user", func(t *testing.T) {
		called := false
		entityClient := &mockEntityUserClient{
			getUserFn: func(context.Context, string) ([]byte, error) {
				called = true
				return []byte(`{"id":"`+targetID+`","email":"jane@example.com"}`), nil
			},
		}
		h := newHandler(&mockSCIMClient{}, entityClient)
		r := withUser(httptest.NewRequest(http.MethodPost, "/users/"+targetID+"/roles", strings.NewReader(`{"roles":["not-a-real-role"]}`)))
		r.SetPathValue("id", targetID)
		w := httptest.NewRecorder()
		h.GrantUserRoles(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		if called {
			t.Fatal("the target user must not be looked up for an unknown role key")
		}
	})

	t.Run("rejects a target whose email isn't a wso2.com address, before calling SCIM", func(t *testing.T) {
		called := false
		entityClient := &mockEntityUserClient{
			getUserFn: func(context.Context, string) ([]byte, error) {
				return []byte(`{"id":"` + targetID + `","email":"jane@example.com"}`), nil
			},
		}
		scimClient := &mockSCIMClient{
			addRoleMembersFn: func(context.Context, string, []string) error {
				called = true
				return nil
			},
		}
		h := newHandler(scimClient, entityClient)
		r := withUser(httptest.NewRequest(http.MethodPost, "/users/"+targetID+"/roles", strings.NewReader(`{"roles":["cs_engineer"]}`)))
		r.SetPathValue("id", targetID)
		w := httptest.NewRecorder()
		h.GrantUserRoles(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		if called {
			t.Fatal("SCIM must not be called for a non-wso2.com target")
		}
	})

	t.Run("grants every requested role and reports 200 on full success", func(t *testing.T) {
		var grantedRoleIDs []string
		entityClient := &mockEntityUserClient{
			getUserFn: func(_ context.Context, id string) ([]byte, error) {
				if id != targetID {
					t.Fatalf("GetUser called with id = %q, want %q", id, targetID)
				}
				return []byte(`{"id":"`+targetID+`","email":"jane@wso2.com"}`), nil
			},
		}
		scimClient := &mockSCIMClient{
			addRoleMembersFn: func(_ context.Context, roleID string, emails []string) error {
				if len(emails) != 1 || emails[0] != "jane@wso2.com" {
					t.Fatalf("AddRoleMembers emails = %v, want [jane@wso2.com]", emails)
				}
				grantedRoleIDs = append(grantedRoleIDs, roleID)
				return nil
			},
		}
		h := newHandler(scimClient, entityClient)
		r := withUser(httptest.NewRequest(http.MethodPost, "/users/"+targetID+"/roles",
			strings.NewReader(`{"roles":["cs_engineer","escalator"]}`)))
		r.SetPathValue("id", targetID)
		w := httptest.NewRecorder()
		h.GrantUserRoles(w, r)
		assertStatus(t, w, http.StatusOK)

		var resp grantUserRolesResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if len(resp.Granted) != 2 || len(resp.Failed) != 0 {
			t.Errorf("response = %+v, want both roles granted and none failed", resp)
		}
		if len(grantedRoleIDs) != 2 {
			t.Errorf("AddRoleMembers called %d times, want 2", len(grantedRoleIDs))
		}
	})

	t.Run("reports 502 when every role fails to grant", func(t *testing.T) {
		entityClient := &mockEntityUserClient{
			getUserFn: func(context.Context, string) ([]byte, error) {
				return []byte(`{"id":"`+targetID+`","email":"jane@wso2.com"}`), nil
			},
		}
		scimClient := &mockSCIMClient{
			addRoleMembersFn: func(context.Context, string, []string) error {
				return errors.New("scim unavailable")
			},
		}
		h := newHandler(scimClient, entityClient)
		r := withUser(httptest.NewRequest(http.MethodPost, "/users/"+targetID+"/roles", strings.NewReader(`{"roles":["cs_engineer"]}`)))
		r.SetPathValue("id", targetID)
		w := httptest.NewRecorder()
		h.GrantUserRoles(w, r)
		assertStatus(t, w, http.StatusBadGateway)

		var resp grantUserRolesResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if len(resp.Granted) != 0 || len(resp.Failed) != 1 {
			t.Errorf("response = %+v, want zero granted and one failed", resp)
		}
		// Unmarshaling "granted":null into []string also gives len 0, so the
		// assertion above can't tell a correct empty array apart from the
		// schema-breaking null json.Marshal produces for a nil slice -- check
		// the raw wire bytes directly.
		if strings.Contains(w.Body.String(), `"granted":null`) {
			t.Errorf("response body = %s, want \"granted\":[] (a required array), not null", w.Body.String())
		}
	})

	t.Run("reports 207 and still attempts every role on a partial failure", func(t *testing.T) {
		entityClient := &mockEntityUserClient{
			getUserFn: func(context.Context, string) ([]byte, error) {
				return []byte(`{"id":"`+targetID+`","email":"jane@wso2.com"}`), nil
			},
		}
		var attempted []string
		scimClient := &mockSCIMClient{
			addRoleMembersFn: func(_ context.Context, roleID string, _ []string) error {
				attempted = append(attempted, roleID)
				if roleID == "role-escalator" {
					return errors.New("scim rejected this role")
				}
				return nil
			},
		}
		h := newHandler(scimClient, entityClient)
		r := withUser(httptest.NewRequest(http.MethodPost, "/users/"+targetID+"/roles",
			strings.NewReader(`{"roles":["cs_engineer","escalator"]}`)))
		r.SetPathValue("id", targetID)
		w := httptest.NewRecorder()
		h.GrantUserRoles(w, r)
		assertStatus(t, w, http.StatusMultiStatus)

		var resp grantUserRolesResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if len(resp.Granted) != 1 || len(resp.Failed) != 1 {
			t.Errorf("response = %+v, want exactly one granted and one failed", resp)
		}
		if len(attempted) != 2 {
			t.Errorf("attempted %d roles, want both roles attempted despite the first succeeding and the second failing", len(attempted))
		}
	})

	t.Run("maps a target-lookup failure through mapUpstreamErrorGeneric", func(t *testing.T) {
		entityClient := &mockEntityUserClient{
			getUserFn: func(context.Context, string) ([]byte, error) {
				return nil, &apierror.Error{StatusCode: http.StatusNotFound}
			},
		}
		called := false
		scimClient := &mockSCIMClient{
			addRoleMembersFn: func(context.Context, string, []string) error {
				called = true
				return nil
			},
		}
		h := newHandler(scimClient, entityClient)
		r := withUser(httptest.NewRequest(http.MethodPost, "/users/"+targetID+"/roles", strings.NewReader(`{"roles":["cs_engineer"]}`)))
		r.SetPathValue("id", targetID)
		w := httptest.NewRecorder()
		h.GrantUserRoles(w, r)
		if called {
			t.Fatal("no role should be granted when the target user lookup fails")
		}
	})
}

// ----- GetTimeCardApprovers -----

func TestGetTimeCardApprovers(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{}, testDirectory(t), false, []string{"role-1"})
		r := httptest.NewRequest(http.MethodGet, "/users/time-card-approvers", nil)
		w := httptest.NewRecorder()
		h.GetTimeCardApprovers(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
	})

	t.Run("fetches the configured role id and maps members to id/email", func(t *testing.T) {
		var gotRoleID string
		scimClient := &mockSCIMClient{
			getRoleFn: func(_ context.Context, roleID string) ([]scim.RoleMember, error) {
				gotRoleID = roleID
				return []scim.RoleMember{
					{ID: "u-1", Email: "jane.doe@wso2.com"},
					{ID: "u-2", Email: "john.smith@wso2.com"},
				}, nil
			},
		}
		h := NewUsersHandler(scimClient, &mockEntityUserClient{}, testDirectory(t), false, []string{"role-1"})
		r := withUser(httptest.NewRequest(http.MethodGet, "/users/time-card-approvers", nil))
		w := httptest.NewRecorder()
		h.GetTimeCardApprovers(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		if gotRoleID != "role-1" {
			t.Errorf("roleID passed to scim.GetRole = %q, want %q", gotRoleID, "role-1")
		}
		resp := decodeJSON[timeCardApproversResponse](t, w)
		want := []timeCardApproverRef{
			{ID: "u-1", Email: "jane.doe@wso2.com"},
			{ID: "u-2", Email: "john.smith@wso2.com"},
		}
		if !reflect.DeepEqual(resp.Approvers, want) {
			t.Errorf("approvers = %+v, want %+v", resp.Approvers, want)
		}
	})

	t.Run("returns 404 when no role id is configured, rather than falling through to GetUser", func(t *testing.T) {
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{}, testDirectory(t), false, nil)
		r := withUser(httptest.NewRequest(http.MethodGet, "/users/time-card-approvers", nil))
		w := httptest.NewRecorder()
		h.GetTimeCardApprovers(w, r)
		assertStatus(t, w, http.StatusNotFound)
		assertErrorMessage(t, w, ErrMsgNotFound)
	})

	// A SCIM 401/403 means this backend's own credentials lack a roles-read
	// scope, not that the caller lacks permission -- excluded from the shared
	// upstreamErrorsGeneric table (which asserts the ordinary 401->401/403->403
	// pass-through) and asserted separately below.
	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to list time card approvers.") {
			if tc.wantCode == http.StatusUnauthorized || tc.wantCode == http.StatusForbidden {
				continue
			}
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				scimClient := &mockSCIMClient{
					getRoleFn: func(context.Context, string) ([]scim.RoleMember, error) {
						return nil, tc.err
					},
				}
				h := NewUsersHandler(scimClient, &mockEntityUserClient{}, testDirectory(t), false, []string{"role-1"})
				r := withUser(httptest.NewRequest(http.MethodGet, "/users/time-card-approvers", nil))
				w := httptest.NewRecorder()
				h.GetTimeCardApprovers(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})

	t.Run("a SCIM 401 or 403 is reported as a sanitized 502, never the caller's own 401/403", func(t *testing.T) {
		for _, code := range []int{http.StatusUnauthorized, http.StatusForbidden} {
			t.Run(http.StatusText(code), func(t *testing.T) {
				scimClient := &mockSCIMClient{
					getRoleFn: func(context.Context, string) ([]scim.RoleMember, error) {
						return nil, &apierror.Error{StatusCode: code, Body: "scim detail that must not leak"}
					},
				}
				h := NewUsersHandler(scimClient, &mockEntityUserClient{}, testDirectory(t), false, []string{"role-1"})
				r := withUser(httptest.NewRequest(http.MethodGet, "/users/time-card-approvers", nil))
				w := httptest.NewRecorder()
				h.GetTimeCardApprovers(w, r)
				assertStatus(t, w, http.StatusBadGateway)
				assertErrorMessage(t, w, "Failed to list time card approvers.")
			})
		}
	})
}

func TestListSavedFilterViews(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{}, testDirectory(t), false, nil)
		r := httptest.NewRequest(http.MethodGet, "/users/me/saved-filter-views?listKey=cases", nil)
		w := httptest.NewRecorder()
		h.ListSavedFilterViews(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
	})

	t.Run("requires listKey", func(t *testing.T) {
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{}, testDirectory(t), false, nil)
		r := withUser(httptest.NewRequest(http.MethodGet, "/users/me/saved-filter-views", nil))
		w := httptest.NewRecorder()
		h.ListSavedFilterViews(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, "listKey is required.")
	})

	t.Run("forwards listKey and returns upstream body", func(t *testing.T) {
		var gotKey string
		entityClient := &mockEntityUserClient{
			listSavedFilterViewsFn: func(_ context.Context, listKey string) ([]byte, error) {
				gotKey = listKey
				return []byte(`{"views":[{"name":"Open","qs":"states=open"}]}`), nil
			},
		}
		h := NewUsersHandler(&mockSCIMClient{}, entityClient, testDirectory(t), false, nil)
		r := withUser(httptest.NewRequest(http.MethodGet, "/users/me/saved-filter-views?listKey=cases", nil))
		w := httptest.NewRecorder()
		h.ListSavedFilterViews(w, r)
		assertStatus(t, w, http.StatusOK)
		if gotKey != "cases" {
			t.Errorf("listKey = %q, want cases", gotKey)
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to list saved filter views.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				entityClient := &mockEntityUserClient{
					listSavedFilterViewsFn: func(_ context.Context, _ string) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewUsersHandler(&mockSCIMClient{}, entityClient, testDirectory(t), false, nil)
				r := withUser(httptest.NewRequest(http.MethodGet, "/users/me/saved-filter-views?listKey=cases", nil))
				w := httptest.NewRecorder()
				h.ListSavedFilterViews(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
			})
		}
	})
}

func TestSaveSavedFilterView(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{}, testDirectory(t), false, nil)
		r := httptest.NewRequest(http.MethodPatch, "/users/me/saved-filter-views", strings.NewReader(`{}`))
		w := httptest.NewRecorder()
		h.SaveSavedFilterView(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
	})

	t.Run("rejects invalid JSON", func(t *testing.T) {
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{}, testDirectory(t), false, nil)
		r := withUser(httptest.NewRequest(http.MethodPatch, "/users/me/saved-filter-views", strings.NewReader(`not-json`)))
		w := httptest.NewRecorder()
		h.SaveSavedFilterView(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
	})

	t.Run("forwards body", func(t *testing.T) {
		const payload = `{"listKey":"cases","name":"Open","qs":"states=open"}`
		var captured []byte
		entityClient := &mockEntityUserClient{
			saveSavedFilterViewFn: func(_ context.Context, body []byte) ([]byte, error) {
				captured = body
				return []byte(`{"views":[{"name":"Open","qs":"states=open"}]}`), nil
			},
		}
		h := NewUsersHandler(&mockSCIMClient{}, entityClient, testDirectory(t), false, nil)
		r := withUser(httptest.NewRequest(http.MethodPatch, "/users/me/saved-filter-views", strings.NewReader(payload)))
		w := httptest.NewRecorder()
		h.SaveSavedFilterView(w, r)
		assertStatus(t, w, http.StatusOK)
		if string(captured) != payload {
			t.Errorf("body = %q, want %q", captured, payload)
		}
	})
}

func TestDeleteSavedFilterView(t *testing.T) {
	t.Run("requires listKey and name", func(t *testing.T) {
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{}, testDirectory(t), false, nil)
		r := withUser(httptest.NewRequest(http.MethodDelete, "/users/me/saved-filter-views?listKey=cases", nil))
		w := httptest.NewRecorder()
		h.DeleteSavedFilterView(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, "listKey and name are required.")
	})

	t.Run("forwards query params", func(t *testing.T) {
		var gotKey, gotName string
		entityClient := &mockEntityUserClient{
			deleteSavedFilterViewFn: func(_ context.Context, listKey, name string) ([]byte, error) {
				gotKey, gotName = listKey, name
				return []byte(`{"views":[]}`), nil
			},
		}
		h := NewUsersHandler(&mockSCIMClient{}, entityClient, testDirectory(t), false, nil)
		r := withUser(httptest.NewRequest(http.MethodDelete, "/users/me/saved-filter-views?listKey=incidents&name=Mine", nil))
		w := httptest.NewRecorder()
		h.DeleteSavedFilterView(w, r)
		assertStatus(t, w, http.StatusOK)
		if gotKey != "incidents" || gotName != "Mine" {
			t.Errorf("got %s/%s", gotKey, gotName)
		}
	})
}

func TestReorderSavedFilterView(t *testing.T) {
	t.Run("forwards body", func(t *testing.T) {
		const payload = `{"listKey":"cases","name":"Open","direction":"down"}`
		var captured []byte
		entityClient := &mockEntityUserClient{
			reorderSavedFilterViewFn: func(_ context.Context, body []byte) ([]byte, error) {
				captured = body
				return []byte(`{"views":[]}`), nil
			},
		}
		h := NewUsersHandler(&mockSCIMClient{}, entityClient, testDirectory(t), false, nil)
		r := withUser(httptest.NewRequest(http.MethodPost, "/users/me/saved-filter-views/reorder", strings.NewReader(payload)))
		w := httptest.NewRecorder()
		h.ReorderSavedFilterView(w, r)
		assertStatus(t, w, http.StatusOK)
		if string(captured) != payload {
			t.Errorf("body = %q, want %q", captured, payload)
		}
	})
}
