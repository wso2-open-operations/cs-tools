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
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{}, testDirectory(t), false, nil)
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
		h := NewUsersHandler(scimClient, &mockEntityUserClient{}, testDirectory(t), false, nil)
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

// TestGetMeDashboardDesignerEmails verifies the DASHBOARD_DESIGNER_EMAILS allow-list
// grants a synthetic "dashboard_designer" role on GET /users/me to a matching
// caller, additively and case-insensitively, and leaves every other caller
// untouched.
func TestGetMeDashboardDesignerEmails(t *testing.T) {
	allowlist := map[string]struct{}{"agent@example.com": {}}

	t.Run("allowlisted email with no entity roles gets exactly dashboard_designer", func(t *testing.T) {
		entityClient := &mockEntityUserClient{
			getUserMeFn: func(_ context.Context) ([]byte, error) {
				return []byte(`{"id":"u-1","email":"agent@example.com","lastName":"Doe"}`), nil
			},
		}
		h := NewUsersHandler(&mockSCIMClient{}, entityClient, testDirectory(t), false, allowlist)
		r := withUser(httptest.NewRequest(http.MethodGet, "/users/me", nil))
		w := httptest.NewRecorder()
		h.GetMe(w, r)

		assertStatus(t, w, http.StatusOK)
		type getMeResp struct {
			Roles []string `json:"roles"`
		}
		resp := decodeJSON[getMeResp](t, w)
		if len(resp.Roles) != 1 || resp.Roles[0] != "dashboard_designer" {
			t.Errorf("roles = %v, want [dashboard_designer]", resp.Roles)
		}
	})

	t.Run("allowlisted email with existing entity roles gets dashboard_designer appended", func(t *testing.T) {
		entityClient := &mockEntityUserClient{
			getUserMeFn: func(_ context.Context) ([]byte, error) {
				return []byte(`{"id":"u-1","email":"agent@example.com","lastName":"Doe","roles":["agent","admin"]}`), nil
			},
		}
		h := NewUsersHandler(&mockSCIMClient{}, entityClient, testDirectory(t), false, allowlist)
		r := withUser(httptest.NewRequest(http.MethodGet, "/users/me", nil))
		w := httptest.NewRecorder()
		h.GetMe(w, r)

		assertStatus(t, w, http.StatusOK)
		type getMeResp struct {
			Roles []string `json:"roles"`
		}
		resp := decodeJSON[getMeResp](t, w)
		want := []string{"agent", "admin", "dashboard_designer"}
		if len(resp.Roles) != len(want) {
			t.Fatalf("roles = %v, want %v", resp.Roles, want)
		}
		for i, r := range want {
			if resp.Roles[i] != r {
				t.Errorf("roles = %v, want %v", resp.Roles, want)
				break
			}
		}
	})

	t.Run("entity roles already containing dashboard_designer are not duplicated", func(t *testing.T) {
		entityClient := &mockEntityUserClient{
			getUserMeFn: func(_ context.Context) ([]byte, error) {
				return []byte(`{"id":"u-1","email":"agent@example.com","lastName":"Doe","roles":["dashboard_designer"]}`), nil
			},
		}
		h := NewUsersHandler(&mockSCIMClient{}, entityClient, testDirectory(t), false, allowlist)
		r := withUser(httptest.NewRequest(http.MethodGet, "/users/me", nil))
		w := httptest.NewRecorder()
		h.GetMe(w, r)

		assertStatus(t, w, http.StatusOK)
		type getMeResp struct {
			Roles []string `json:"roles"`
		}
		resp := decodeJSON[getMeResp](t, w)
		if len(resp.Roles) != 1 || resp.Roles[0] != "dashboard_designer" {
			t.Errorf("roles = %v, want exactly one dashboard_designer entry", resp.Roles)
		}
	})

	t.Run("email not in the allowlist is unaffected", func(t *testing.T) {
		entityClient := &mockEntityUserClient{
			getUserMeFn: func(_ context.Context) ([]byte, error) {
				return []byte(`{"id":"u-1","email":"agent@example.com","lastName":"Doe","roles":["agent"]}`), nil
			},
		}
		h := NewUsersHandler(&mockSCIMClient{}, entityClient, testDirectory(t), false,
			map[string]struct{}{"someone.else@example.com": {}})
		r := withUser(httptest.NewRequest(http.MethodGet, "/users/me", nil))
		w := httptest.NewRecorder()
		h.GetMe(w, r)

		assertStatus(t, w, http.StatusOK)
		type getMeResp struct {
			Roles []string `json:"roles"`
		}
		resp := decodeJSON[getMeResp](t, w)
		if len(resp.Roles) != 1 || resp.Roles[0] != "agent" {
			t.Errorf("roles = %v, want [agent]", resp.Roles)
		}
	})

	t.Run("matching is case-insensitive", func(t *testing.T) {
		// The allow-list is keyed lower-case (as loadDashboardDesignerEmails
		// always stores it), but the caller's JWT email arrives in whatever
		// case the identity provider happened to send -- mixed-case here --
		// so this exercises GetMe's own strings.ToLower on the caller's
		// email before the lookup.
		mixedCaseUser := &middleware.UserInfo{
			Email:  "Agent@Example.com",
			UserID: testUser.UserID,
		}
		entityClient := &mockEntityUserClient{
			getUserMeFn: func(_ context.Context) ([]byte, error) {
				return []byte(`{"id":"u-1","email":"Agent@Example.com","lastName":"Doe"}`), nil
			},
		}
		h := NewUsersHandler(&mockSCIMClient{}, entityClient, testDirectory(t), false, allowlist)
		r := httptest.NewRequest(http.MethodGet, "/users/me", nil)
		r = r.WithContext(middleware.WithUserInfo(r.Context(), mixedCaseUser))
		w := httptest.NewRecorder()
		h.GetMe(w, r)

		assertStatus(t, w, http.StatusOK)
		type getMeResp struct {
			Roles []string `json:"roles"`
		}
		resp := decodeJSON[getMeResp](t, w)
		if len(resp.Roles) != 1 || resp.Roles[0] != "dashboard_designer" {
			t.Errorf("roles = %v, want [dashboard_designer]", resp.Roles)
		}
	})

	t.Run("empty allowlist changes nothing (regression safety net)", func(t *testing.T) {
		entityClient := &mockEntityUserClient{
			getUserMeFn: func(_ context.Context) ([]byte, error) {
				return []byte(`{"id":"u-1","email":"agent@example.com","lastName":"Doe","roles":["agent"]}`), nil
			},
		}
		h := NewUsersHandler(&mockSCIMClient{}, entityClient, testDirectory(t), false, nil)
		r := withUser(httptest.NewRequest(http.MethodGet, "/users/me", nil))
		w := httptest.NewRecorder()
		h.GetMe(w, r)

		assertStatus(t, w, http.StatusOK)
		type getMeResp struct {
			Roles []string `json:"roles"`
		}
		resp := decodeJSON[getMeResp](t, w)
		if len(resp.Roles) != 1 || resp.Roles[0] != "agent" {
			t.Errorf("roles = %v, want [agent]", resp.Roles)
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
		h := NewUsersHandler(scimClient, &mockEntityUserClient{}, testDirectory(t), false, nil)
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
				h := NewUsersHandler(scimClient, &mockEntityUserClient{}, testDirectory(t), false, nil)
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
