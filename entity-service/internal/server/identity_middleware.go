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

package server

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// anonymousRoutes is the explicit allow-list of registered mux patterns that
// are served with no resolvable caller at all. Every other route on the main
// listener needs either a validated user token or an allow-listed internal
// client credential before its handler runs (see callerIdentityMiddleware).
//
// The list is deliberately tiny: the liveness probe on this port is the only
// operation the published contract declares with `security: []`, and
// routes_contract_test.go fails if the two ever disagree in either direction.
// Anything else that must accept an unauthenticated call (an inbound
// webhook, say) belongs behind a dedicated public component that
// authenticates it and forwards with a client credential, the way the GitHub
// delivery endpoint already works -- not on this list.
var anonymousRoutes = map[string]bool{
	"GET /health": true,
}

// callerIdentityMiddleware resolves the caller's identity once per request
// (via the same AccessService.ResolveScope every scoped endpoint already
// calls), attaches it to the request context so repository.Scoped can read
// it without every service re-resolving it, and refuses the request outright
// when there is no caller to resolve.
//
// What it decides, in order:
//
//   - A request no registered route matches is handed straight to the mux,
//     which answers 404 or 405 itself. The gate never changes the status an
//     unknown path or method gets, and it never does an identity lookup for
//     a request that has no handler to reach.
//   - A route on anonymous (see anonymousRoutes) is served as-is.
//   - A request whose identity was never validated -- auth.Middleware missing
//     from the chain, which routes.go never does -- is refused with 503
//     rather than served with nothing trustworthy to scope by.
//   - A request with neither a user token nor an allow-listed internal client
//     id (AccessService.ResolveScope's UnauthorizedError) is refused with 401
//     here, before any handler runs. This is the one failure that used to be
//     passed through: a handler whose repository takes the raw pool (no
//     Scoped wrapper, no row-level security) would then serve an anonymous
//     caller as if it were anyone. Every such route is now closed to
//     anonymous callers in one place, independent of how its repository is
//     built.
//   - Any other resolution failure is still passed through with no identity
//     attached, exactly as before: a user token for an email with no active
//     user row (ForbiddenError) must reach the handlers that provision or
//     register a first-time caller, and a lookup failure is for the handler
//     to report in its own terms. A protected repository method reached this
//     way returns repository.ErrNoCallerIdentity, and underneath that FORCE
//     ROW LEVEL SECURITY fails closed regardless of what Go does.
//
// A successful resolution is attached with repository.WithCallerIdentity, so
// ResolveScope's own cache check short-circuits for every service that calls
// it again on the same request.
func callerIdentityMiddleware(access service.AccessService, mux *http.ServeMux, anonymous map[string]bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, pattern := mux.Handler(r)
		if pattern == "" || anonymous[pattern] {
			mux.ServeHTTP(w, r)
			return
		}

		ctx := r.Context()
		if !auth.IdentityFromContext(ctx).Validated {
			apierror.WriteJSON(w, http.StatusServiceUnavailable,
				"results cannot be scoped to the caller: no verified identity on this request")
			return
		}

		scope, err := access.ResolveScope(ctx)
		switch {
		case err == nil:
			ctx = repository.WithCallerIdentity(ctx, scope)
		case isUnauthorized(err):
			slog.WarnContext(ctx, "request refused: no resolvable caller", "route", pattern)
			apierror.WriteJSON(w, http.StatusUnauthorized, err.Error())
			return
		}
		mux.ServeHTTP(w, r.WithContext(ctx))
	})
}

// isUnauthorized reports whether err is AccessService.ResolveScope's "no user
// token and not an internal client" refusal.
func isUnauthorized(err error) bool {
	var ue *apierror.UnauthorizedError
	return errors.As(err, &ue)
}
