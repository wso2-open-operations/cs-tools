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
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

const userRolesKey contextKey = "user-roles"

// EntityUserRolesClient defines the upstream client contract required by RoleResolver.
type EntityUserRolesClient interface {
	GetUserMe(ctx context.Context) ([]byte, error)
}

// RoleResolver abstracts user role resolution with caching.
type RoleResolver interface {
	GetRoles(ctx context.Context) ([]string, error)
}

// cachedRoleEntry holds cached user roles alongside an expiration timestamp.
type cachedRoleEntry struct {
	roles     []string
	expiresAt time.Time
}

// CachedRoleResolver implements RoleResolver with an in-memory TTL cache.
type CachedRoleResolver struct {
	client EntityUserRolesClient
	ttl    time.Duration
	mu     sync.RWMutex
	cache  map[string]cachedRoleEntry
}

// NewRoleResolver creates a CachedRoleResolver backed by the given entity client and TTL.
func NewRoleResolver(client EntityUserRolesClient, ttl time.Duration) *CachedRoleResolver {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &CachedRoleResolver{
		client: client,
		ttl:    ttl,
		cache:  make(map[string]cachedRoleEntry),
	}
}

// entityRolesResponse models the payload returned by entity-service GET /users/me.
type entityRolesResponse struct {
	Roles []string `json:"roles"`
}

// GetRoles returns the caller's roles from cache or by fetching from the entity service.
func (r *CachedRoleResolver) GetRoles(ctx context.Context) ([]string, error) {
	user := UserInfoFromContext(ctx)
	if user == nil || user.UserID == "" {
		return nil, errors.New("rbac: unauthenticated request")
	}

	r.mu.RLock()
	entry, ok := r.cache[user.UserID]
	r.mu.RUnlock()

	if ok && time.Now().Before(entry.expiresAt) {
		return append([]string(nil), entry.roles...), nil
	}

	raw, err := r.client.GetUserMe(ctx)
	if err != nil {
		return nil, fmt.Errorf("rbac: fetch user roles: %w", err)
	}

	var resp entityRolesResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("rbac: parse user roles: %w", err)
	}

	roles := resp.Roles
	if roles == nil {
		roles = []string{}
	}

	r.mu.Lock()
	r.cache[user.UserID] = cachedRoleEntry{
		roles:     roles,
		expiresAt: time.Now().Add(r.ttl),
	}
	r.mu.Unlock()

	return append([]string(nil), roles...), nil
}

// UserRolesFromContext retrieves resolved user roles from context.
func UserRolesFromContext(ctx context.Context) []string {
	if roles, ok := ctx.Value(userRolesKey).([]string); ok {
		return roles
	}
	return nil
}

// WithUserRoles attaches user roles to the given context.
func WithUserRoles(ctx context.Context, roles []string) context.Context {
	return context.WithValue(ctx, userRolesKey, roles)
}

// RequireRoles returns an HTTP middleware that verifies the authenticated caller
// has at least one of the specified allowed roles.
func RequireRoles(resolver RoleResolver, allowedRoles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := UserInfoFromContext(r.Context())
			if user == nil {
				writeAuthError(w, "You are not authorized to perform this action. Please try again.")
				return
			}

			roles, err := resolver.GetRoles(r.Context())
			if err != nil {
				slog.ErrorContext(r.Context(), "rbac: failed to resolve roles", "userID", user.UserID, "err", err)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadGateway)
				_ = json.NewEncoder(w).Encode(authErrorBody{Message: "Failed to resolve user roles."})
				return
			}

			if !hasAnyRole(roles, allowedRoles) {
				slog.WarnContext(r.Context(), "rbac: access denied: user lacks required roles", "userID", user.UserID)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_ = json.NewEncoder(w).Encode(authErrorBody{Message: "You do not have permission to perform this action."})
				return
			}

			ctx := WithUserRoles(r.Context(), roles)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// hasAnyRole reports whether userRoles contains at least one role from allowedRoles (case-insensitive).
func hasAnyRole(userRoles, allowedRoles []string) bool {
	for _, userRole := range userRoles {
		for _, allowed := range allowedRoles {
			if strings.EqualFold(strings.TrimSpace(userRole), strings.TrimSpace(allowed)) {
				return true
			}
		}
	}
	return false
}
