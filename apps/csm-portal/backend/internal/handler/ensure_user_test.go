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
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

func TestEnsureUserProvisioned(t *testing.T) {
	user := &middleware.UserInfo{
		Email:     "jane.doe@example.com",
		UserID:    "id-on-token",
		FirstName: "Jane",
		LastName:  "Doe",
	}

	t.Run("already provisioned: CreateUser is not called", func(t *testing.T) {
		client := &mockEntityCaseClient{
			createUserFn: func(_ context.Context, _ []byte) ([]byte, error) {
				t.Fatal("CreateUser should not be called when GetUserMe already succeeds")
				return nil, nil
			},
		}
		ensureUserProvisioned(context.Background(), client, user)
	})

	t.Run("no user row yet: CreateUser is called with the token's name/email and the internal role", func(t *testing.T) {
		var gotBody []byte
		var called bool
		client := &mockEntityCaseClient{
			getUserMeFn: func(_ context.Context) ([]byte, error) {
				return nil, &apierror.Error{StatusCode: http.StatusNotFound, Body: "not found"}
			},
			createUserFn: func(_ context.Context, body []byte) ([]byte, error) {
				called = true
				gotBody = body
				return []byte(`{"id":"new-id"}`), nil
			},
		}
		ensureUserProvisioned(context.Background(), client, user)
		if !called {
			t.Fatal("CreateUser was not called")
		}
		var req struct {
			FirstName         string   `json:"firstName"`
			LastName          string   `json:"lastName"`
			Email             string   `json:"email"`
			Roles             []string `json:"roles"`
			SystemProvisioned bool     `json:"systemProvisioned"`
		}
		if err := json.Unmarshal(gotBody, &req); err != nil {
			t.Fatalf("decode CreateUser body: %v", err)
		}
		if req.FirstName != "Jane" || req.LastName != "Doe" || req.Email != "jane.doe@example.com" {
			t.Errorf("CreateUser body = %+v, want firstName/lastName/email from the token", req)
		}
		if len(req.Roles) != 1 || req.Roles[0] != "internal" {
			t.Errorf("CreateUser roles = %v, want [\"internal\"]", req.Roles)
		}
		if !req.SystemProvisioned {
			t.Error("CreateUser body systemProvisioned = false, want true -- this row must not be self-attributed to the person it provisions")
		}
	})

	t.Run("no name claims at all: falls back to the email's local part so entity-service's name-required validation never rejects it", func(t *testing.T) {
		noNameUser := &middleware.UserInfo{
			Email:  "jane.doe@example.com",
			UserID: "id-on-token",
			// FirstName/LastName deliberately both zero-value -- an IdP
			// configuration that never sends given_name/family_name.
		}
		var gotBody []byte
		client := &mockEntityCaseClient{
			getUserMeFn: func(_ context.Context) ([]byte, error) {
				return nil, &apierror.Error{StatusCode: http.StatusNotFound, Body: "not found"}
			},
			createUserFn: func(_ context.Context, body []byte) ([]byte, error) {
				gotBody = body
				return []byte(`{"id":"new-id"}`), nil
			},
		}
		ensureUserProvisioned(context.Background(), client, noNameUser)
		var req struct {
			FirstName string `json:"firstName"`
			LastName  string `json:"lastName"`
		}
		if err := json.Unmarshal(gotBody, &req); err != nil {
			t.Fatalf("decode CreateUser body: %v", err)
		}
		if req.FirstName != "" || req.LastName != "jane.doe" {
			t.Errorf("CreateUser body = %+v, want firstName empty and lastName %q (the email's local part)", req, "jane.doe")
		}
	})

	t.Run("a non-404 GetUserMe failure skips CreateUser entirely", func(t *testing.T) {
		client := &mockEntityCaseClient{
			getUserMeFn: func(_ context.Context) ([]byte, error) {
				return nil, &apierror.Error{StatusCode: http.StatusInternalServerError, Body: "boom"}
			},
			createUserFn: func(_ context.Context, _ []byte) ([]byte, error) {
				t.Fatal("CreateUser should not be called on a non-404 GetUserMe failure")
				return nil, nil
			},
		}
		ensureUserProvisioned(context.Background(), client, user)
	})

	t.Run("a CreateUser failure is swallowed, never panics or blocks the caller", func(t *testing.T) {
		client := &mockEntityCaseClient{
			getUserMeFn: func(_ context.Context) ([]byte, error) {
				return nil, &apierror.Error{StatusCode: http.StatusNotFound, Body: "not found"}
			},
			createUserFn: func(_ context.Context, _ []byte) ([]byte, error) {
				return nil, &apierror.Error{StatusCode: http.StatusInternalServerError, Body: "boom"}
			},
		}
		ensureUserProvisioned(context.Background(), client, user)
	})
}
