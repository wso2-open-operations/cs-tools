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

package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

type mockEntityUserRolesClient struct {
	calls atomic.Int32
	raw   []byte
	err   error
}

func (m *mockEntityUserRolesClient) GetUserMe(_ context.Context) ([]byte, error) {
	m.calls.Add(1)
	return m.raw, m.err
}

func TestCachedRoleResolver(t *testing.T) {
	t.Run("unauthenticated context returns error", func(t *testing.T) {
		client := &mockEntityUserRolesClient{raw: []byte(`{"roles":["agent"]}`)}
		resolver := NewRoleResolver(client, 5*time.Minute)

		_, err := resolver.GetRoles(context.Background())
		if err == nil {
			t.Fatal("expected error for unauthenticated context, got nil")
		}
		if client.calls.Load() != 0 {
			t.Fatalf("expected 0 client calls, got %d", client.calls.Load())
		}
	})

	t.Run("cache miss fetches upstream and caches result", func(t *testing.T) {
		client := &mockEntityUserRolesClient{raw: []byte(`{"roles":["agent","timecard_approver"]}`)}
		resolver := NewRoleResolver(client, 100*time.Millisecond)

		ctx := WithUserInfo(context.Background(), &UserInfo{
			Email:  "test@wso2.com",
			UserID: "user-123",
		})

		roles, err := resolver.GetRoles(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(roles) != 2 || roles[0] != "agent" || roles[1] != "timecard_approver" {
			t.Fatalf("unexpected roles: %v", roles)
		}
		if client.calls.Load() != 1 {
			t.Fatalf("expected 1 call, got %d", client.calls.Load())
		}

		// Subsequent call within TTL returns cached result
		roles2, err := resolver.GetRoles(ctx)
		if err != nil {
			t.Fatalf("unexpected error on second call: %v", err)
		}
		if len(roles2) != 2 {
			t.Fatalf("unexpected roles on second call: %v", roles2)
		}
		if client.calls.Load() != 1 {
			t.Fatalf("expected call count to remain 1 (cache hit), got %d", client.calls.Load())
		}
	})

	t.Run("cache expires after ttl", func(t *testing.T) {
		client := &mockEntityUserRolesClient{raw: []byte(`{"roles":["agent"]}`)}
		resolver := NewRoleResolver(client, 10*time.Millisecond)

		ctx := WithUserInfo(context.Background(), &UserInfo{
			Email:  "test@wso2.com",
			UserID: "user-456",
		})

		_, _ = resolver.GetRoles(ctx)
		if client.calls.Load() != 1 {
			t.Fatalf("expected 1 call, got %d", client.calls.Load())
		}

		time.Sleep(20 * time.Millisecond)

		_, _ = resolver.GetRoles(ctx)
		if client.calls.Load() != 2 {
			t.Fatalf("expected 2 calls after TTL expiry, got %d", client.calls.Load())
		}
	})

	t.Run("upstream error returns error without caching", func(t *testing.T) {
		client := &mockEntityUserRolesClient{err: errors.New("upstream connection reset")}
		resolver := NewRoleResolver(client, 5*time.Minute)

		ctx := WithUserInfo(context.Background(), &UserInfo{
			Email:  "test@wso2.com",
			UserID: "user-789",
		})

		_, err := resolver.GetRoles(ctx)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

type staticRoleResolver struct {
	roles []string
	err   error
}

func (s *staticRoleResolver) GetRoles(_ context.Context) ([]string, error) {
	return s.roles, s.err
}

func TestRequireRoles(t *testing.T) {
	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		roles := UserRolesFromContext(r.Context())
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "roles": roles})
	})

	t.Run("unauthenticated request returns 401", func(t *testing.T) {
		resolver := &staticRoleResolver{roles: []string{"admin"}}
		handler := RequireRoles(resolver, "admin")(dummyHandler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d", rec.Code)
		}
	})

	t.Run("resolver error returns 502", func(t *testing.T) {
		resolver := &staticRoleResolver{err: errors.New("network failure")}
		handler := RequireRoles(resolver, "admin")(dummyHandler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req = req.WithContext(WithUserInfo(req.Context(), &UserInfo{UserID: "u1"}))
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadGateway {
			t.Fatalf("expected status 502, got %d", rec.Code)
		}
	})

	t.Run("user lacking required role returns 403", func(t *testing.T) {
		resolver := &staticRoleResolver{roles: []string{"agent"}}
		handler := RequireRoles(resolver, "admin")(dummyHandler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req = req.WithContext(WithUserInfo(req.Context(), &UserInfo{UserID: "u1"}))
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected status 403, got %d", rec.Code)
		}

		var body map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if body["message"] != "You do not have permission to perform this action." {
			t.Fatalf("unexpected message: %q", body["message"])
		}
	})

	t.Run("user with matching role succeeds (case-insensitive)", func(t *testing.T) {
		resolver := &staticRoleResolver{roles: []string{"AGENT", "Admin"}}
		handler := RequireRoles(resolver, "admin")(dummyHandler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req = req.WithContext(WithUserInfo(req.Context(), &UserInfo{UserID: "u1"}))
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}

		var body struct {
			Ok    bool     `json:"ok"`
			Roles []string `json:"roles"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if !body.Ok {
			t.Fatal("expected ok to be true")
		}
		if len(body.Roles) != 2 || body.Roles[1] != "Admin" {
			t.Fatalf("unexpected roles in context: %v", body.Roles)
		}
	})

	t.Run("multiple allowed roles matches any", func(t *testing.T) {
		resolver := &staticRoleResolver{roles: []string{"timecard_approver"}}
		handler := RequireRoles(resolver, "admin", "timecard_approver")(dummyHandler)

		req := httptest.NewRequest(http.MethodPatch, "/time-cards/11111111-1111-1111-1111-111111111111", nil)
		req = req.WithContext(WithUserInfo(req.Context(), &UserInfo{UserID: "u1"}))
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
	})
}
