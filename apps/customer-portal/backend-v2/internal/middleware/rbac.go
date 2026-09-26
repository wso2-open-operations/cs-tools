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

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

// CanonicalRole represents a normalized role identifier across the customer portal.
type CanonicalRole string

const (
	RoleAdmin         CanonicalRole = "admin"
	RoleAgent         CanonicalRole = "agent"
	RoleCustomerAdmin CanonicalRole = "customer_admin"
	RoleCustomerUser  CanonicalRole = "customer_user"
	RolePartnerAdmin  CanonicalRole = "partner_admin"
	RolePartnerUser   CanonicalRole = "partner_user"
	RoleInternal      CanonicalRole = "internal"
)

// NormalizeRole maps raw incoming role strings (from ServiceNow or Asgardeo)
// into canonical role enums.
func NormalizeRole(roleStr string) CanonicalRole {
	trimmed := strings.TrimSpace(roleStr)
	switch trimmed {
	case "sn_customerservice.admin", "admin":
		return RoleAdmin
	case "wso2_agent", "agent":
		return RoleAgent
	case "sn_customerservice.customer_admin", "customer_admin":
		return RoleCustomerAdmin
	// snc_external is ServiceNow's marker for a customer-side user (the CSM
	// backend calls it "a customer-facing role"), so it resolves to the same
	// persona as the customer role rather than granting nothing.
	case "sn_customerservice.customer", "customer", "customer_user", "snc_external", "external":
		return RoleCustomerUser
	case "sn_customerservice.partner_admin", "partner_admin":
		return RolePartnerAdmin
	case "sn_customerservice.partner", "partner", "partner_user":
		return RolePartnerUser
	// snc_internal is the ServiceNow wire form of internal, so both land on the
	// same persona. They used to diverge: snc_internal became agent and got
	// agent's grants, while the Postgres form became internal and got none, so
	// the same person's access depended on which data source entity-service was
	// running. RoleInternal now carries agent's grants, which is what
	// snc_internal holders already had.
	case "snc_internal", "internal":
		return RoleInternal
	default:
		return CanonicalRole(trimmed)
	}
}

// NormalizeRoles normalizes a slice of role strings, removing duplicates.
func NormalizeRoles(rawRoles []string) []CanonicalRole {
	if len(rawRoles) == 0 {
		return nil
	}
	result := make([]CanonicalRole, 0, len(rawRoles))
	seen := make(map[CanonicalRole]struct{}, len(rawRoles))
	for _, r := range rawRoles {
		nr := NormalizeRole(r)
		if nr == "" {
			continue
		}
		if _, ok := seen[nr]; !ok {
			seen[nr] = struct{}{}
			result = append(result, nr)
		}
	}
	return result
}

// Module represents a functional domain module in the Customer Portal.
type Module string

const (
	ModuleCases               Module = "cases"
	ModuleTimeCards           Module = "time_cards"
	ModuleProjects            Module = "projects"
	ModuleChangeRequests      Module = "change_requests"
	ModuleDeployments         Module = "deployments"
	ModuleDeploymentProducts  Module = "deployment_products"
	ModuleDeploymentResources Module = "deployment_resources"
)

// Action represents an operation performed on a module.
type Action string

const (
	ActionCreate Action = "create"
	ActionRead   Action = "read"
	ActionUpdate Action = "update"
	ActionDelete Action = "delete"
)

// permissionMatrix maps each module and action to the set of canonical roles permitted.
var permissionMatrix = map[Module]map[Action][]CanonicalRole{
	ModuleCases: {
		ActionCreate: {RoleAdmin, RoleAgent, RoleInternal, RoleCustomerAdmin, RoleCustomerUser, RolePartnerAdmin, RolePartnerUser},
		ActionRead:   {RoleAdmin, RoleAgent, RoleInternal, RoleCustomerAdmin, RoleCustomerUser, RolePartnerAdmin, RolePartnerUser},
		ActionUpdate: {RoleAdmin, RoleAgent, RoleInternal, RoleCustomerAdmin, RoleCustomerUser, RolePartnerAdmin, RolePartnerUser},
		ActionDelete: {RoleAdmin},
	},
	ModuleTimeCards: {
		ActionCreate: {RoleAdmin, RoleAgent, RoleInternal},
		ActionRead:   {RoleAdmin, RoleAgent, RoleInternal, RoleCustomerAdmin, RoleCustomerUser, RolePartnerAdmin, RolePartnerUser},
		ActionUpdate: {RoleAdmin, RoleAgent, RoleInternal},
		ActionDelete: {RoleAdmin},
		// Stakeholder has no access to Time Cards.
	},
	ModuleProjects: {
		ActionCreate: {RoleAdmin},
		ActionRead:   {RoleAdmin, RoleAgent, RoleInternal, RoleCustomerAdmin, RoleCustomerUser, RolePartnerAdmin, RolePartnerUser},
		ActionUpdate: {RoleAdmin, RoleAgent, RoleInternal},
		ActionDelete: {RoleAdmin},
	},
	ModuleChangeRequests: {
		ActionCreate: {RoleAdmin, RoleAgent, RoleInternal},
		ActionRead:   {RoleAdmin, RoleAgent, RoleInternal, RoleCustomerAdmin, RoleCustomerUser, RolePartnerAdmin, RolePartnerUser},
		ActionUpdate: {RoleAdmin, RoleAgent, RoleInternal},
		ActionDelete: {RoleAdmin},
		// Stakeholder has no access to Change Requests.
	},
	ModuleDeployments: {
		ActionCreate: {RoleAdmin, RoleAgent, RoleInternal, RoleCustomerAdmin, RoleCustomerUser, RolePartnerAdmin, RolePartnerUser},
		ActionRead:   {RoleAdmin, RoleAgent, RoleInternal, RoleCustomerAdmin, RoleCustomerUser, RolePartnerAdmin, RolePartnerUser},
		ActionUpdate: {RoleAdmin, RoleAgent, RoleInternal, RoleCustomerAdmin, RoleCustomerUser, RolePartnerAdmin, RolePartnerUser},
		ActionDelete: {RoleAdmin, RoleAgent, RoleInternal, RoleCustomerAdmin, RoleCustomerUser, RolePartnerAdmin, RolePartnerUser},
	},
	ModuleDeploymentProducts: {
		ActionCreate: {RoleAdmin, RoleAgent, RoleInternal, RoleCustomerAdmin, RoleCustomerUser, RolePartnerAdmin, RolePartnerUser},
		ActionRead:   {RoleAdmin, RoleAgent, RoleInternal, RoleCustomerAdmin, RoleCustomerUser, RolePartnerAdmin, RolePartnerUser},
		ActionUpdate: {RoleAdmin, RoleAgent, RoleInternal, RoleCustomerAdmin, RoleCustomerUser, RolePartnerAdmin, RolePartnerUser},
		ActionDelete: {RoleAdmin, RoleAgent, RoleInternal, RoleCustomerAdmin, RoleCustomerUser, RolePartnerAdmin, RolePartnerUser},
	},
	ModuleDeploymentResources: {
		ActionCreate: {RoleAdmin, RoleAgent, RoleInternal, RoleCustomerAdmin, RoleCustomerUser, RolePartnerAdmin, RolePartnerUser},
		ActionRead:   {RoleAdmin, RoleAgent, RoleInternal, RoleCustomerAdmin, RoleCustomerUser, RolePartnerAdmin, RolePartnerUser},
		ActionUpdate: {RoleAdmin, RoleAgent, RoleInternal, RoleCustomerAdmin, RoleCustomerUser, RolePartnerAdmin, RolePartnerUser},
		ActionDelete: {RoleAdmin, RoleAgent, RoleInternal, RoleCustomerAdmin, RoleCustomerUser, RolePartnerAdmin, RolePartnerUser},
	},
}

// HasPermission checks whether any of the user's roles grant the requested action on the module.
//
// Every decision comes from permissionMatrix. There is deliberately no role
// that short-circuits it: a blanket grant above the matrix makes the matrix
// stop describing what the portal actually allows.
func HasPermission(userRoles []CanonicalRole, module Module, action Action) bool {
	actions, moduleExists := permissionMatrix[module]
	if !moduleExists {
		return false
	}
	allowedRoles, actionExists := actions[action]
	if !actionExists {
		return false
	}

	for _, userRole := range userRoles {
		for _, allowed := range allowedRoles {
			if userRole == allowed {
				return true
			}
		}
	}
	return false
}

// EntityUserClient defines the upstream client contract required by RoleResolver.
type EntityUserClient interface {
	GetMe(ctx context.Context) (entity.GetUserMeResponse, error)
}

// RoleResolver abstracts user role resolution with caching.
type RoleResolver interface {
	GetRoles(ctx context.Context) ([]CanonicalRole, error)
}

// cachedRoleEntry holds cached user roles alongside an expiration timestamp.
type cachedRoleEntry struct {
	roles     []CanonicalRole
	expiresAt time.Time
}

// CachedRoleResolver implements RoleResolver with an in-memory TTL cache.
type CachedRoleResolver struct {
	client EntityUserClient
	ttl    time.Duration
	mu     sync.RWMutex
	cache  map[string]cachedRoleEntry
}

// NewCachedRoleResolver creates a CachedRoleResolver backed by the given entity client and TTL.
func NewCachedRoleResolver(client EntityUserClient, ttl time.Duration) *CachedRoleResolver {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &CachedRoleResolver{
		client: client,
		ttl:    ttl,
		cache:  make(map[string]cachedRoleEntry),
	}
}

// GetRoles returns the caller's canonical roles from cache or by fetching from entity-service.
func (r *CachedRoleResolver) GetRoles(ctx context.Context) ([]CanonicalRole, error) {
	user := UserInfoFromContext(ctx)
	if user == nil || user.UserID == "" {
		return nil, errors.New("rbac: unauthenticated request")
	}

	r.mu.RLock()
	entry, found := r.cache[user.UserID]
	r.mu.RUnlock()

	if found && time.Now().Before(entry.expiresAt) {
		return entry.roles, nil
	}

	resp, err := r.client.GetMe(ctx)
	if err != nil {
		return nil, fmt.Errorf("rbac: failed to resolve user roles: %w", err)
	}

	roles := NormalizeRoles(resp.Roles)

	// Sweeping the whole map under the write lock is O(n), but it runs only on
	// a cache miss, never on a hit: a hit returns above without ever taking
	// this lock. Misses are bounded by the TTL, so the work is roughly one
	// sweep per user per TTL rather than one per request, which is negligible
	// at this portal's scale (hundreds of concurrent users, not hundreds of
	// thousands).
	//
	// The alternative of never evicting is what this replaced: entries are
	// keyed by user id and the resolver outlives every request, so a long
	// running instance accumulated one entry per user who ever authenticated.
	// If the user population ever grows enough for this sweep to show up in
	// latency, move it to a background ticker rather than dropping eviction.
	r.mu.Lock()
	now := time.Now()
	for uid, e := range r.cache {
		if now.After(e.expiresAt) {
			delete(r.cache, uid)
		}
	}
	r.cache[user.UserID] = cachedRoleEntry{
		roles:     roles,
		expiresAt: now.Add(r.ttl),
	}
	r.mu.Unlock()

	return roles, nil
}

func writeForbiddenError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(authErrorBody{Message: message})
}

// RequirePermission wraps next with authorization checking against the permission matrix.
func RequirePermission(resolver RoleResolver, module Module, action Action) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := UserInfoFromContext(r.Context())
			if user == nil {
				writeAuthError(w, "You are not authorized to perform this action. Please try again.")
				return
			}

			roles, err := resolver.GetRoles(r.Context())
			if err != nil {
				slog.ErrorContext(r.Context(), "rbac: failed to resolve roles", "err", err)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadGateway)
				_ = json.NewEncoder(w).Encode(authErrorBody{Message: "Failed to resolve user roles."})
				return
			}

			if !HasPermission(roles, module, action) {
				writeForbiddenError(w, "You do not have permission to perform this action.")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireRoles wraps next with authorization checking against an explicit set of allowed roles.
func RequireRoles(resolver RoleResolver, allowedRoles ...CanonicalRole) func(http.Handler) http.Handler {
	allowed := make(map[CanonicalRole]struct{}, len(allowedRoles))
	for _, role := range allowedRoles {
		allowed[role] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := UserInfoFromContext(r.Context())
			if user == nil {
				writeAuthError(w, "You are not authorized to perform this action. Please try again.")
				return
			}

			roles, err := resolver.GetRoles(r.Context())
			if err != nil {
				slog.ErrorContext(r.Context(), "rbac: failed to resolve roles", "err", err)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadGateway)
				_ = json.NewEncoder(w).Encode(authErrorBody{Message: "Failed to resolve user roles."})
				return
			}

			// No blanket-grant role here either, for the same reason as
			// HasPermission: the allow-list at the call site is the whole rule.
			for _, role := range roles {
				if _, ok := allowed[role]; ok {
					next.ServeHTTP(w, r)
					return
				}
			}

			writeForbiddenError(w, "You do not have permission to perform this action.")
		})
	}
}
