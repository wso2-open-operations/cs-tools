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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/scim"
)

const (
	phoneTargetID = "11111111-1111-1111-1111-111111111111"
	phoneValid    = "+15550100000"
)

// GET /users/{id} must hand the stored phone to the client untouched through
// every enrichment step, for internal and external targets alike.
func TestGetUser_PassesPhoneThrough(t *testing.T) {
	cases := map[string]struct {
		profile string
		scim    *mockSCIMClient
		guard   bool
	}{
		"plain profile": {
			profile: `{"id":"` + phoneTargetID + `","email":"user@example.com","phone":"` + phoneValid + `","groups":[]}`,
			scim:    &mockSCIMClient{},
		},
		"external contact with SCIM status": {
			profile: `{"id":"` + phoneTargetID + `","email":"user@example.com","userType":"external","phone":"` + phoneValid + `"}`,
			scim: &mockSCIMClient{searchExternalUserFn: func(context.Context, string) (*scim.ExternalUserInfo, error) {
				return &scim.ExternalUserInfo{Exists: true}, nil
			}},
		},
		"internal user with portal roles": {
			profile: `{"id":"` + phoneTargetID + `","email":"staff@wso2.com","userType":"internal","phone":"` + phoneValid + `"}`,
			scim: &mockSCIMClient{
				searchUserFn: func(context.Context, string) (*scim.UserInfo, error) {
					return &scim.UserInfo{ID: "scim-user-1"}, nil
				},
				getUserByIDFn: func(context.Context, string) (scim.UserAssignments, error) {
					return scim.UserAssignments{Roles: []scim.RoleAssignment{{ID: "test-admin-id", Name: "test-admin"}}}, nil
				},
			},
			guard: true,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h := NewUsersHandler(tc.scim, &mockEntityUserClient{
				getUserFn: func(context.Context, string) ([]byte, error) { return []byte(tc.profile), nil },
			}, testDirectory(t), false, nil)
			if tc.guard {
				h = h.WithAccessGuard(newAccessGuardForCSMRoles())
			}
			r := withUser(httptest.NewRequest(http.MethodGet, "/users/"+phoneTargetID, nil))
			r.SetPathValue("id", phoneTargetID)
			w := httptest.NewRecorder()
			h.GetUser(w, r)

			assertStatus(t, w, http.StatusOK)
			got := decodeJSON[map[string]any](t, w)
			if got["phone"] != phoneValid {
				t.Errorf("phone = %v, want %q passed through unchanged", got["phone"], phoneValid)
			}
			if _, renamed := got["phoneNumber"]; renamed {
				t.Error("phoneNumber must not appear on GET /users/{id}; the key is phone")
			}
		})
	}

	t.Run("absent phone stays absent", func(t *testing.T) {
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{
			getUserFn: func(context.Context, string) ([]byte, error) {
				return []byte(`{"id":"` + phoneTargetID + `","email":"user@example.com"}`), nil
			},
		}, testDirectory(t), false, nil)
		r := withUser(httptest.NewRequest(http.MethodGet, "/users/"+phoneTargetID, nil))
		r.SetPathValue("id", phoneTargetID)
		w := httptest.NewRecorder()
		h.GetUser(w, r)
		assertStatus(t, w, http.StatusOK)
		if _, ok := decodeJSON[map[string]any](t, w)["phone"]; ok {
			t.Error("phone key should be absent when the user has none")
		}
	})
}

func TestNormalizePhone(t *testing.T) {
	valid := map[string]string{
		phoneValid:              phoneValid,
		"  " + phoneValid + " ": phoneValid,
		"":                      "",
		"   ":                   "",
		"+14155552671":          "+14155552671",
		"+12345678":             "+12345678",        // 8 digits, the minimum
		"+123456789012345":      "+123456789012345", // 15 digits, the maximum
	}
	for in, want := range valid {
		got, ok := normalizePhone(in)
		if !ok || got != want {
			t.Errorf("normalizePhone(%q) = %q, %v; want %q, true", in, got, ok, want)
		}
	}
	for _, in := range []string{
		"+1 555-010-0000", "+1-555-0100", "15550100000", "+05550100000", "+1234567",
		"+1234567890123456", "+1555010000a", "++15550100000", "(555) 010-0000", "+１５５５０１０００００", "+15550100000\n+1",
	} {
		if got, ok := normalizePhone(in); ok {
			t.Errorf("normalizePhone(%q) = %q, true; want rejected", in, got)
		}
	}
}

func TestPatchMe_PhoneValidation(t *testing.T) {
	invalid := []string{"+1 555-010-0000", "5550100000", "+1555", strings.Repeat("1", 40)}
	for _, in := range invalid {
		t.Run("rejects "+in, func(t *testing.T) {
			called := false
			h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{
				patchUserMeFn: func(context.Context, []byte) ([]byte, error) { called = true; return []byte(`{}`), nil },
			}, testDirectory(t), false, nil)
			r := withUser(httptest.NewRequest(http.MethodPatch, "/users/me", strings.NewReader(`{"phoneNumber":"`+in+`","timeZone":"UTC"}`)))
			w := httptest.NewRecorder()
			h.PatchMe(w, r)
			assertStatus(t, w, http.StatusBadRequest)
			assertErrorMessage(t, w, errMsgPhoneFormat)
			if strings.Contains(w.Body.String(), in) {
				t.Errorf("response echoes the rejected value: %s", w.Body.String())
			}
			if called {
				t.Error("entity must not be called for an invalid phone")
			}
		})
	}

	t.Run("exact error body", func(t *testing.T) {
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{}, testDirectory(t), false, nil)
		w := httptest.NewRecorder()
		h.PatchMe(w, withUser(httptest.NewRequest(http.MethodPatch, "/users/me", strings.NewReader(`{"phoneNumber":"12"}`))))
		if got, want := strings.TrimSpace(w.Body.String()), `{"message":"phone must be in E.164 format, e.g. +14155552671"}`; got != want {
			t.Errorf("body = %s, want %s", got, want)
		}
	})

	t.Run("timeZone only leaves phone absent", func(t *testing.T) {
		var body []byte
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{
			patchUserMeFn: func(_ context.Context, b []byte) ([]byte, error) { body = b; return []byte(`{}`), nil },
		}, testDirectory(t), false, nil)
		w := httptest.NewRecorder()
		h.PatchMe(w, withUser(httptest.NewRequest(http.MethodPatch, "/users/me", strings.NewReader(`{"timeZone":"UTC"}`))))
		assertStatus(t, w, http.StatusOK)
		if string(body) != `{"timeZone":"UTC"}` {
			t.Errorf("entity body = %s, want no phone key", body)
		}
	})

	t.Run("entity 400 message and status reach the client", func(t *testing.T) {
		const msg = "phone must be in E.164 format, e.g. +14155552671"
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{
			patchUserMeFn: func(context.Context, []byte) ([]byte, error) {
				return nil, &apierror.Error{StatusCode: http.StatusBadRequest, Body: `{"message":"` + msg + `"}`}
			},
		}, testDirectory(t), false, nil)
		w := httptest.NewRecorder()
		h.PatchMe(w, withUser(httptest.NewRequest(http.MethodPatch, "/users/me", strings.NewReader(`{"phoneNumber":"`+phoneValid+`"}`))))
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, msg)
	})
}

func TestUpdateUser_PhoneValidation(t *testing.T) {
	patch := func(h *UsersHandler, body string) *httptest.ResponseRecorder {
		r := withUser(httptest.NewRequest(http.MethodPatch, "/users/"+phoneTargetID, strings.NewReader(body)))
		r.SetPathValue("id", phoneTargetID)
		w := httptest.NewRecorder()
		h.UpdateUser(w, r)
		return w
	}

	for _, in := range []string{"+1 555-010-0000", "5550100000", "+1555", strings.Repeat("1", 40)} {
		t.Run("rejects "+in, func(t *testing.T) {
			called := false
			h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{
				updateUserFn: func(context.Context, string, []byte) ([]byte, error) { called = true; return []byte(`{}`), nil },
			}, testDirectory(t), false, nil)
			w := patch(h, `{"phone":"`+in+`"}`)
			assertStatus(t, w, http.StatusBadRequest)
			assertErrorMessage(t, w, errMsgPhoneFormat)
			if strings.Contains(w.Body.String(), in) {
				t.Errorf("response echoes the rejected value: %s", w.Body.String())
			}
			if called {
				t.Error("entity must not be called for an invalid phone")
			}
		})
	}

	for name, tc := range map[string]struct{ in, want string }{
		"padded value is trimmed": {" " + phoneValid + " ", `{"phone":"` + phoneValid + `"}`},
		"blank clears":            {"   ", `{"phone":""}`},
	} {
		t.Run(name, func(t *testing.T) {
			var body []byte
			h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{
				updateUserFn: func(_ context.Context, _ string, b []byte) ([]byte, error) { body = b; return []byte(`{}`), nil },
			}, testDirectory(t), false, nil)
			assertStatus(t, patch(h, `{"phone":"`+tc.in+`"}`), http.StatusOK)
			if string(body) != tc.want {
				t.Errorf("entity body = %s, want %s", body, tc.want)
			}
		})
	}

	t.Run("entity 400 message and status reach the client", func(t *testing.T) {
		const msg = "phone must be in E.164 format, e.g. +14155552671"
		h := NewUsersHandler(&mockSCIMClient{}, &mockEntityUserClient{
			updateUserFn: func(context.Context, string, []byte) ([]byte, error) {
				return nil, &apierror.Error{StatusCode: http.StatusBadRequest, Body: `{"message":"` + msg + `"}`}
			},
		}, testDirectory(t), false, nil)
		w := patch(h, `{"phone":"`+phoneValid+`"}`)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, msg)
	})
}
