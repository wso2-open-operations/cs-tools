// Package middleware carries the one piece of PLG's own request plumbing that
// survives the merge.
//
// PLG's standalone BFF had four middleware files. Three — CORS, logging,
// correlation IDs, security headers — were reimplementations of what
// csm-portal's backend already does, and they are gone: csm-portal's chain runs
// first and PLG's handlers sit inside it.
//
// This one stays because it does something csm-portal's does not: it turns the
// authenticated caller into the PLG `"user".id` that every PLG write records.
package middleware

import (
	"context"
	"net/http"
	"strings"

	csm "github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/domain"
)

// UserResolver turns the caller's email into the platform user it names.
//
// Satisfied by entityclient.Client. Declared here, narrowly, rather than
// importing the client: this package needs one method, and a one-method
// interface is what makes the middleware testable without an HTTP server.
type UserResolver interface {
	GetCSUser(ctx context.Context, email string) (*domain.UserRef, error)
}

type userIDKey struct{}
type userRefKey struct{}

// ResolveIdentity turns the authenticated caller into a PLG `"user".id`.
//
// WHAT THE MERGE CHANGED, AND WHY IT MATTERS.
//
// Standalone, this read an email from the `X-PLG-User` request header and
// believed it. That was honest for a demo on localhost and indefensible
// deployed: anyone who could reach the BFF could be anyone.
//
// **The header is gone.** csm-portal's `Auth` middleware has already validated
// the Asgardeo token against JWKS — issuer, audience, expiry, signature — and
// put the result in the request context. This reads the email from there. There
// is no inbound header to strip, because there is no header: a client cannot
// assert an identity that is only ever derived from a validated token.
//
// WHY IT STILL RESOLVES, rather than using the token's own user id. csm-portal's
// `UserInfo.UserID` is the IdP's subject — Asgardeo's identifier for the person.
// PLG's foreign keys reference `"user".id`, the database's own UUID. They are
// different identifiers for the same human, and only the email joins them. So
// the resolution survives; only its input changed.
//
// A caller the platform does not know is a 401 here rather than a foreign-key
// violation on their first write. A caller who is not active INTERNAL staff
// resolves to nothing for the same reason — see plg_users_repo.go, where that
// restriction is prepended to every query rather than left to the caller.
func ResolveIdentity(resolver UserResolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only PLG's own routes need a PLG user. Everything else in
			// csm-portal passes straight through — this middleware is mounted
			// on the plg/ subtree, and the guard is belt and braces.
			if !strings.HasPrefix(r.URL.Path, "/plg/") {
				next.ServeHTTP(w, r)
				return
			}

			info := csm.UserInfoFromContext(r.Context())
			if info == nil || strings.TrimSpace(info.Email) == "" {
				// csm-portal's Auth middleware rejects an unauthenticated
				// request before this runs, so reaching here means the chain
				// was assembled wrongly rather than that a caller misbehaved.
				apierror.WriteJSON(w, http.StatusUnauthorized, "caller identity is required")
				return
			}

			user, err := resolver.GetCSUser(r.Context(), info.Email)
			if err != nil {
				// The lookup itself failed — entity-service unreachable, most
				// likely. That is a 503, not a 401: the caller may be perfectly
				// valid and we simply cannot tell.
				apierror.WriteJSON(w, http.StatusServiceUnavailable,
					"cannot verify the caller right now, please try again")
				return
			}
			if user == nil {
				// Authenticated, but not PLG staff. 403 rather than 401: their
				// token is valid and signing in again cannot fix it. The PLG
				// nav should be hidden for these callers so they never reach a
				// page that rejects every request.
				apierror.WriteJSON(w, http.StatusForbidden,
					"no PLG CS engineer is registered for "+info.Email)
				return
			}

			ctx := context.WithValue(r.Context(), userIDKey{}, user.ID)
			ctx = context.WithValue(ctx, userRefKey{}, *user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserIDFromContext returns the resolved PLG user id, or "" if absent.
func UserIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(userIDKey{}).(string)
	return v
}

// UserRefFromContext returns the resolved caller, or false if absent.
//
// Handlers that only need the id use UserIDFromContext. This exists for
// GET /plg/me, which wants the whole reference and would otherwise fetch again
// what this middleware has already looked up.
func UserRefFromContext(ctx context.Context) (domain.UserRef, bool) {
	v, ok := ctx.Value(userRefKey{}).(domain.UserRef)
	return v, ok
}
