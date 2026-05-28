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
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/scim"
)

// ----- GetMe -----

func TestGetMe(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{})
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
		h := NewUsersHandler(scimClient, &mockEntityUserClient{})
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

	t.Run("returns email from JWT when SCIM returns no user", func(t *testing.T) {
		scimClient := &mockSCIMClient{
			searchUserFn: func(_ context.Context, _ string) (*scim.UserInfo, error) {
				return nil, nil // user not found in SCIM
			},
		}
		h := NewUsersHandler(scimClient, &mockEntityUserClient{})
		r := withUser(httptest.NewRequest(http.MethodGet, "/users/me", nil))
		w := httptest.NewRecorder()
		h.GetMe(w, r)

		assertStatus(t, w, http.StatusOK)
		resp := decodeJSON[map[string]any](t, w)
		if resp["email"] != testUser.Email {
			t.Errorf("email = %v, want %q", resp["email"], testUser.Email)
		}
	})

	t.Run("searches SCIM by the JWT email and returns phone and password timestamp", func(t *testing.T) {
		phone := "+94771234567"
		ts := "2025-01-15T10:00:00Z"
		var capturedEmail string
		scimClient := &mockSCIMClient{
			searchUserFn: func(_ context.Context, email string) (*scim.UserInfo, error) {
				capturedEmail = email
				return &scim.UserInfo{
					PhoneNumber:            &phone,
					LastPasswordUpdateTime: &ts,
				}, nil
			},
		}
		h := NewUsersHandler(scimClient, &mockEntityUserClient{})
		r := withUser(httptest.NewRequest(http.MethodGet, "/users/me", nil))
		w := httptest.NewRecorder()
		h.GetMe(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		if capturedEmail != testUser.Email {
			t.Errorf("SCIM searched email %q, want %q", capturedEmail, testUser.Email)
		}
		type getMeResp struct {
			Email                  string  `json:"email"`
			PhoneNumber            *string `json:"phoneNumber"`
			LastPasswordUpdateTime *string `json:"lastPasswordUpdateTime"`
		}
		resp := decodeJSON[getMeResp](t, w)
		if resp.Email != testUser.Email {
			t.Errorf("email = %q, want %q", resp.Email, testUser.Email)
		}
		if resp.PhoneNumber == nil || *resp.PhoneNumber != phone {
			t.Errorf("phoneNumber = %v, want %q", resp.PhoneNumber, phone)
		}
		if resp.LastPasswordUpdateTime == nil || *resp.LastPasswordUpdateTime != ts {
			t.Errorf("lastPasswordUpdateTime = %v, want %q", resp.LastPasswordUpdateTime, ts)
		}
	})
}

// ----- PatchMe -----

func TestPatchMe(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{})
		r := httptest.NewRequest(http.MethodPatch, "/users/me", strings.NewReader(`{"phoneNumber":"+1"}`))
		w := httptest.NewRecorder()
		h.PatchMe(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{})
		r := withUser(httptest.NewRequest(http.MethodPatch, "/users/me", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		w := httptest.NewRecorder()
		h.PatchMe(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
	})

	t.Run("rejects empty body", func(t *testing.T) {
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{})
		r := withUser(httptest.NewRequest(http.MethodPatch, "/users/me", strings.NewReader("")))
		w := httptest.NewRecorder()
		h.PatchMe(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, "At least one field must be provided for update.")
	})

	t.Run("rejects invalid JSON", func(t *testing.T) {
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{})
		r := withUser(httptest.NewRequest(http.MethodPatch, "/users/me", strings.NewReader(`not-json`)))
		w := httptest.NewRecorder()
		h.PatchMe(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
	})

	t.Run("rejects JSON with no updateable fields", func(t *testing.T) {
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{})
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
		h := NewUsersHandler(scimClient, &mockEntityUserClient{})
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
				h := NewUsersHandler(scimClient, &mockEntityUserClient{})
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
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{})
		r := httptest.NewRequest(http.MethodPost, "/users/search", strings.NewReader(`{}`))
		w := httptest.NewRecorder()
		h.SearchUsers(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/users/search", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		w := httptest.NewRecorder()
		h.SearchUsers(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{})
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
		h := NewUsersHandler(&mockSCIMClient{}, entityClient)
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
		for _, tc := range upstreamErrors("Failed to search users.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				entityClient := &mockEntityUserClient{
					searchUsersFn: func(_ context.Context, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewUsersHandler(&mockSCIMClient{}, entityClient)
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
