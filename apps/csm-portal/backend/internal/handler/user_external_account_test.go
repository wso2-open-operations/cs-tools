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
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/scim"
)

// TestGetUser_ExternalAccountStatus_AppendedForExternalContacts: an external
// contact's profile gets the SCIM exists/locked block appended.
func TestGetUser_ExternalAccountStatus_AppendedForExternalContacts(t *testing.T) {
	const id = "11111111-1111-1111-1111-111111111111"
	var gotEmail string
	locked := true
	h := NewUsersHandler(&mockSCIMClient{
		searchExternalUserFn: func(_ context.Context, email string) (*scim.ExternalUserInfo, error) {
			gotEmail = email
			return &scim.ExternalUserInfo{Exists: true, Locked: &locked}, nil
		},
	}, &mockEntityUserClient{
		getUserFn: func(_ context.Context, _ string) ([]byte, error) {
			return []byte(`{"id":"` + id + `","email":"contact@example.com","userType":"external"}`), nil
		},
	}, testDirectory(t), false)

	r := withUser(httptest.NewRequest(http.MethodGet, "/users/"+id, nil))
	r.SetPathValue("id", id)
	w := httptest.NewRecorder()
	h.GetUser(w, r)

	assertStatus(t, w, http.StatusOK)
	if gotEmail != "contact@example.com" {
		t.Errorf("SearchExternalUser called with email %q, want the profile's email", gotEmail)
	}

	got := decodeJSON[struct {
		ExternalAccount struct {
			Exists bool  `json:"exists"`
			Locked *bool `json:"locked"`
		} `json:"externalAccount"`
	}](t, w)
	if !got.ExternalAccount.Exists {
		t.Error("externalAccount.exists = false, want true")
	}
	if got.ExternalAccount.Locked == nil || !*got.ExternalAccount.Locked {
		t.Errorf("externalAccount.locked = %v, want true", got.ExternalAccount.Locked)
	}
}

// TestGetUser_ExternalAccountStatus_SkippedForInternalStaff: WSO2 staff live
// in the SCIM "internal" org, so the external lookup must not run at all.
func TestGetUser_ExternalAccountStatus_SkippedForInternalStaff(t *testing.T) {
	const id = "11111111-1111-1111-1111-111111111111"
	called := false
	h := NewUsersHandler(&mockSCIMClient{
		searchExternalUserFn: func(_ context.Context, _ string) (*scim.ExternalUserInfo, error) {
			called = true
			return &scim.ExternalUserInfo{Exists: true}, nil
		},
	}, &mockEntityUserClient{
		getUserFn: func(_ context.Context, _ string) ([]byte, error) {
			return []byte(`{"id":"` + id + `","email":"staff@example.com","userType":"internal"}`), nil
		},
	}, testDirectory(t), false)

	r := withUser(httptest.NewRequest(http.MethodGet, "/users/"+id, nil))
	r.SetPathValue("id", id)
	w := httptest.NewRecorder()
	h.GetUser(w, r)

	assertStatus(t, w, http.StatusOK)
	if called {
		t.Error("SearchExternalUser was called for an internal user, want no SCIM external lookup")
	}
	got := decodeJSON[map[string]any](t, w)
	if _, ok := got["externalAccount"]; ok {
		t.Error("externalAccount present on an internal user's profile, want absent")
	}
}

// TestGetUser_ExternalAccountStatus_SkippedForWso2Email: a ServiceNow row can
// carry a wso2.com email under a customer-facing role/userType (e.g. a
// wso2.com contact recorded under snc_external for testing) -- that account
// can never exist in the SCIM "external" org, so the lookup must not run.
func TestGetUser_ExternalAccountStatus_SkippedForWso2Email(t *testing.T) {
	const id = "11111111-1111-1111-1111-111111111111"
	called := false
	h := NewUsersHandler(&mockSCIMClient{
		searchExternalUserFn: func(_ context.Context, _ string) (*scim.ExternalUserInfo, error) {
			called = true
			return &scim.ExternalUserInfo{Exists: true}, nil
		},
	}, &mockEntityUserClient{
		getUserFn: func(_ context.Context, _ string) ([]byte, error) {
			return []byte(`{"id":"` + id + `","email":"tester@wso2.com","userType":"external"}`), nil
		},
	}, testDirectory(t), false)

	r := withUser(httptest.NewRequest(http.MethodGet, "/users/"+id, nil))
	r.SetPathValue("id", id)
	w := httptest.NewRecorder()
	h.GetUser(w, r)

	assertStatus(t, w, http.StatusOK)
	if called {
		t.Error("SearchExternalUser was called for a wso2.com email, want no SCIM external lookup")
	}
	got := decodeJSON[map[string]any](t, w)
	if _, ok := got["externalAccount"]; ok {
		t.Error("externalAccount present for a wso2.com email, want absent")
	}
}

// TestGetUser_ExternalAccountStatus_FailureDoesNotFailTheRequest: a SCIM
// error must not turn a 200 into an error response -- this enrichment is
// best-effort, same as teams.
func TestGetUser_ExternalAccountStatus_FailureDoesNotFailTheRequest(t *testing.T) {
	const id = "11111111-1111-1111-1111-111111111111"
	h := NewUsersHandler(&mockSCIMClient{
		searchExternalUserFn: func(_ context.Context, _ string) (*scim.ExternalUserInfo, error) {
			return nil, errors.New("scim unavailable")
		},
	}, &mockEntityUserClient{
		getUserFn: func(_ context.Context, _ string) ([]byte, error) {
			return []byte(`{"id":"` + id + `","email":"contact@example.com","userType":"external"}`), nil
		},
	}, testDirectory(t), false)

	r := withUser(httptest.NewRequest(http.MethodGet, "/users/"+id, nil))
	r.SetPathValue("id", id)
	w := httptest.NewRecorder()
	h.GetUser(w, r)

	assertStatus(t, w, http.StatusOK)
	got := decodeJSON[map[string]any](t, w)
	if _, ok := got["externalAccount"]; ok {
		t.Error("externalAccount present despite a SCIM failure, want absent")
	}
	if got["email"] != "contact@example.com" {
		t.Errorf("email = %v, want the rest of the profile preserved", got["email"])
	}
}
