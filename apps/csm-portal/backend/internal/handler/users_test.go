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
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/scim"
)

// ----- GetMe -----

func TestGetMe(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{}, testDirectory(t), false)
		r := httptest.NewRequest(http.MethodGet, "/users/me", nil)
		w := httptest.NewRecorder()
		h.GetMe(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("returns email from JWT even when SCIM fails", func(t *testing.T) {
		// SCIM errors are non-fatal; the handler degrades gracefully.
		scimClient := &mockSCIMClient{
			searchUserFn: func(_ context.Context, _ string) (*scim.UserInfo, error) {
				return nil, errors.New("scim unavailable")
			},
		}
		h := NewUsersHandler(scimClient, &mockEntityUserClient{}, testDirectory(t), false)
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
			t.Error("phoneNumber should be absent when SCIM fails")
		}
	})

	t.Run("upstream errors from the entity service are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to fetch the current user.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				entityClient := &mockEntityUserClient{
					getUserMeFn: func(_ context.Context) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewUsersHandler(&mockSCIMClient{}, entityClient, testDirectory(t), false)
				r := withUser(httptest.NewRequest(http.MethodGet, "/users/me", nil))
				w := httptest.NewRecorder()
				h.GetMe(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})

	t.Run("returns email from JWT when SCIM returns no user", func(t *testing.T) {
		scimClient := &mockSCIMClient{
			searchUserFn: func(_ context.Context, _ string) (*scim.UserInfo, error) {
				return nil, nil // user not found in SCIM
			},
		}
		h := NewUsersHandler(scimClient, &mockEntityUserClient{}, testDirectory(t), false)
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
		h := NewUsersHandler(&mockSCIMClient{}, entityClient, testDirectory(t), false)
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
		h := NewUsersHandler(&mockSCIMClient{}, entityClient, testDirectory(t), false)
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
		h := NewUsersHandler(&mockSCIMClient{}, entityClient, testDirectory(t), false)
		r := withUser(httptest.NewRequest(http.MethodGet, "/users/me", nil))
		w := httptest.NewRecorder()
		h.GetMe(w, r)

		assertStatus(t, w, http.StatusOK)
		resp := decodeJSON[map[string]any](t, w)
		if _, ok := resp["team"]; ok {
			t.Error("team should be absent when none of the caller's groups is a registry team")
		}
	})

	t.Run("searches SCIM by the JWT email and returns phone number", func(t *testing.T) {
		phone := "+94771234567"
		var capturedEmail string
		scimClient := &mockSCIMClient{
			searchUserFn: func(_ context.Context, email string) (*scim.UserInfo, error) {
				capturedEmail = email
				return &scim.UserInfo{
					PhoneNumber: &phone,
				}, nil
			},
		}
		h := NewUsersHandler(scimClient, &mockEntityUserClient{}, testDirectory(t), false)
		r := withUser(httptest.NewRequest(http.MethodGet, "/users/me", nil))
		w := httptest.NewRecorder()
		h.GetMe(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		if capturedEmail != testUser.Email {
			t.Errorf("SCIM searched email %q, want %q", capturedEmail, testUser.Email)
		}
		type getMeResp struct {
			Email       string  `json:"email"`
			PhoneNumber *string `json:"phoneNumber"`
		}
		resp := decodeJSON[getMeResp](t, w)
		if resp.Email != testUser.Email {
			t.Errorf("email = %q, want %q", resp.Email, testUser.Email)
		}
		if resp.PhoneNumber == nil || *resp.PhoneNumber != phone {
			t.Errorf("phoneNumber = %v, want %q", resp.PhoneNumber, phone)
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
			h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{}, testDirectory(t), enabled)
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
		return NewUsersHandler(&mockSCIMClient{}, entity, testDirectory(t), false).WithAccessGuard(NewAccessGuard(cfg))
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
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{}, testDirectory(t), false)
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
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{}, testDirectory(t), false)
		r := httptest.NewRequest(http.MethodPatch, "/users/me", strings.NewReader(`{"phoneNumber":"+1"}`))
		w := httptest.NewRecorder()
		h.PatchMe(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{}, testDirectory(t), false)
		r := withUser(httptest.NewRequest(http.MethodPatch, "/users/me", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		w := httptest.NewRecorder()
		h.PatchMe(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
	})

	t.Run("rejects empty body", func(t *testing.T) {
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{}, testDirectory(t), false)
		r := withUser(httptest.NewRequest(http.MethodPatch, "/users/me", strings.NewReader("")))
		w := httptest.NewRecorder()
		h.PatchMe(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, "At least one field must be provided for update.")
	})

	t.Run("rejects invalid JSON", func(t *testing.T) {
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{}, testDirectory(t), false)
		r := withUser(httptest.NewRequest(http.MethodPatch, "/users/me", strings.NewReader(`not-json`)))
		w := httptest.NewRecorder()
		h.PatchMe(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
	})

	t.Run("rejects JSON with no updateable fields", func(t *testing.T) {
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{}, testDirectory(t), false)
		r := withUser(httptest.NewRequest(http.MethodPatch, "/users/me", strings.NewReader(`{}`)))
		w := httptest.NewRecorder()
		h.PatchMe(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, "At least one field must be provided for update.")
	})

	t.Run("passes correct userID and phone to SCIM and returns updated phone", func(t *testing.T) {
		updated := "+94777654321"
		var capturedUserID, capturedMobile string
		scimClient := &mockSCIMClient{
			updateUserPhoneFn: func(_ context.Context, userID, mobile string) (*string, error) {
				capturedUserID = userID
				capturedMobile = mobile
				return &updated, nil
			},
		}
		h := NewUsersHandler(scimClient, &mockEntityUserClient{}, testDirectory(t), false)
		r := withUser(httptest.NewRequest(http.MethodPatch, "/users/me", strings.NewReader(`{"phoneNumber":"+94777654321"}`)))
		w := httptest.NewRecorder()
		h.PatchMe(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		if capturedUserID != testUser.UserID {
			t.Errorf("SCIM called with userID %q, want %q", capturedUserID, testUser.UserID)
		}
		if capturedMobile != "+94777654321" {
			t.Errorf("SCIM called with mobile %q, want %q", capturedMobile, "+94777654321")
		}
		type patchResp struct {
			PhoneNumber *string `json:"phoneNumber"`
		}
		resp := decodeJSON[patchResp](t, w)
		if resp.PhoneNumber == nil || *resp.PhoneNumber != updated {
			t.Errorf("phoneNumber = %v, want %q", resp.PhoneNumber, updated)
		}
	})

	t.Run("upstream errors from SCIM are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to update phone number.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				scimClient := &mockSCIMClient{
					updateUserPhoneFn: func(_ context.Context, _, _ string) (*string, error) {
						return nil, tc.err
					},
				}
				h := NewUsersHandler(scimClient, &mockEntityUserClient{}, testDirectory(t), false)
				r := withUser(httptest.NewRequest(http.MethodPatch, "/users/me", strings.NewReader(`{"phoneNumber":"+1"}`)))
				w := httptest.NewRecorder()
				h.PatchMe(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

// ----- SearchUsers -----

func TestSearchUsers(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{}, testDirectory(t), false)
		r := httptest.NewRequest(http.MethodPost, "/users/search", strings.NewReader(`{}`))
		w := httptest.NewRecorder()
		h.SearchUsers(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{}, testDirectory(t), false)
		r := withUser(httptest.NewRequest(http.MethodPost, "/users/search", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		w := httptest.NewRecorder()
		h.SearchUsers(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{}, testDirectory(t), false)
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
		h := NewUsersHandler(&mockSCIMClient{}, entityClient, testDirectory(t), false)
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
				h := NewUsersHandler(&mockSCIMClient{}, entityClient, testDirectory(t), false)
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
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{}, testDirectory(t), false)
		r := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{}`))
		w := httptest.NewRecorder()
		h.CreateUser(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{}, testDirectory(t), false)
		r := withUser(httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		w := httptest.NewRecorder()
		h.CreateUser(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{}, testDirectory(t), false)
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
		h := NewUsersHandler(&mockSCIMClient{}, entityClient, testDirectory(t), false)
		r := withUser(httptest.NewRequest(http.MethodPost, "/users",
			strings.NewReader(`{"firstName":"Jane","email":"jane@example.com","roles":["not-a-real-role"]}`)))
		w := httptest.NewRecorder()
		h.CreateUser(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		if called {
			t.Fatal("entity service must not be called for an invalid role")
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
		h := NewUsersHandler(&mockSCIMClient{}, entityClient, testDirectory(t), false)
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
				h := NewUsersHandler(&mockSCIMClient{}, entityClient, testDirectory(t), false)
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
}

func TestListSavedFilterViews(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{}, testDirectory(t), false)
		r := httptest.NewRequest(http.MethodGet, "/users/me/saved-filter-views?listKey=cases", nil)
		w := httptest.NewRecorder()
		h.ListSavedFilterViews(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
	})

	t.Run("requires listKey", func(t *testing.T) {
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{}, testDirectory(t), false)
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
		h := NewUsersHandler(&mockSCIMClient{}, entityClient, testDirectory(t), false)
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
				h := NewUsersHandler(&mockSCIMClient{}, entityClient, testDirectory(t), false)
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
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{}, testDirectory(t), false)
		r := httptest.NewRequest(http.MethodPatch, "/users/me/saved-filter-views", strings.NewReader(`{}`))
		w := httptest.NewRecorder()
		h.SaveSavedFilterView(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
	})

	t.Run("rejects invalid JSON", func(t *testing.T) {
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{}, testDirectory(t), false)
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
		h := NewUsersHandler(&mockSCIMClient{}, entityClient, testDirectory(t), false)
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
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{}, testDirectory(t), false)
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
		h := NewUsersHandler(&mockSCIMClient{}, entityClient, testDirectory(t), false)
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
		h := NewUsersHandler(&mockSCIMClient{}, entityClient, testDirectory(t), false)
		r := withUser(httptest.NewRequest(http.MethodPost, "/users/me/saved-filter-views/reorder", strings.NewReader(payload)))
		w := httptest.NewRecorder()
		h.ReorderSavedFilterView(w, r)
		assertStatus(t, w, http.StatusOK)
		if string(captured) != payload {
			t.Errorf("body = %q, want %q", captured, payload)
		}
	})
}
