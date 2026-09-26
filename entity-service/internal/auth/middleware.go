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

package auth

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
)

const (
	userTokenHeader       = "x-user-id-token" // #nosec G101 -- header name, not a credential
	clientAssertionHeader = "x-jwt-assertion" // #nosec G101 -- header name, not a credential
)

// Identity is the caller identity established for a request.
type Identity struct {
	// Validated is true only when token validation is enabled and every token
	// presented on the request passed it. When false, nothing in the request
	// may be trusted, and anything that scopes results by caller must refuse.
	Validated bool
	// UserEmail/UserSubject/UserID come from a validated x-user-id-token;
	// empty when the request carried none (a machine-to-machine call). See
	// UserClaims.UserID's own doc comment for why UserID, not UserSubject, is
	// the field that correlates with what a caller (e.g. csm-portal-backend)
	// already logs for the same request.
	UserEmail   string
	UserSubject string
	UserID      string
	// ClientID comes from a decoded (not signature-verified -- see
	// Validator.ExtractClientID) x-jwt-assertion token; empty when the
	// request carried none.
	ClientID string
}

type ctxKey struct{}

// WithIdentity returns a copy of ctx carrying id. Tests use it to inject an
// identity without minting tokens.
func WithIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// IdentityFromContext returns the request's Identity, or the zero (unvalidated)
// Identity if the middleware did not run.
func IdentityFromContext(ctx context.Context) Identity {
	id, _ := ctx.Value(ctxKey{}).(Identity)
	return id
}

// IdentityHolder is a mutable, request-scoped slot for the caller identity
// Middleware resolves, installed into the request context by
// middleware.Logger (the entity-service access logger) before Middleware
// runs, so Logger can report who called on the same access-log line it
// already writes.
//
// A plain context.WithValue(ctx, key, Identity{...}) -- the mechanism
// WithIdentity/IdentityFromContext above already provide, and still use --
// is only ever visible to a handler further INSIDE the chain than the one
// that set it, never back out to a middleware that wraps it. Logger wraps
// Middleware, and Middleware returns early on a 401 (never calling
// next.ServeHTTP) without handing any mutated request back up the chain the
// normal way -- so Logger could never observe an Identity value passed via
// WithIdentity/IdentityFromContext alone, on the rejection path that matters
// most for a security access log. IdentityHolder sidesteps that: Logger
// creates one and keeps a direct Go reference to it (not just a context
// entry), so writes Middleware makes through the *same* pointer, reached via
// the context, are visible to Logger's own reference regardless of whether
// Middleware ever calls next.ServeHTTP.
type IdentityHolder struct {
	UserID   string
	ClientID string
}

type identityHolderCtxKey struct{}

// WithIdentityHolder returns a copy of ctx carrying a fresh, empty
// IdentityHolder, plus a direct reference to that same holder. The caller
// (middleware.Logger) must read the returned *IdentityHolder directly, not
// by re-deriving it from a context -- see IdentityHolder's own doc comment
// for why.
func WithIdentityHolder(ctx context.Context) (context.Context, *IdentityHolder) {
	h := &IdentityHolder{}
	return context.WithValue(ctx, identityHolderCtxKey{}, h), h
}

func identityHolderFromContext(ctx context.Context) *IdentityHolder {
	h, _ := ctx.Value(identityHolderCtxKey{}).(*IdentityHolder)
	return h
}

// Middleware validates the tokens on every request and attaches the resulting
// Identity to the context. routes.go always supplies a real Validator -- there
// is no config flag to disable this. A nil Validator is a purely defensive
// fallback (attaches an unvalidated Identity and never rejects); it should
// only ever happen if a caller wires this middleware without one, a bug.
//
// A user token that is PRESENT but invalid is always rejected with 401 --
// never downgraded to "no token", which would turn a forged user token into
// an anonymous (and, for a system client, less restricted) request. A request
// with no tokens at all passes through: whether that is acceptable is decided
// per endpoint by the service that scopes by caller. x-jwt-assertion is only
// decoded, never signature-verified (see ExtractClientID's own doc comment),
// so it is rejected only when it can't even be decoded, or carries neither a
// client_id nor an azp claim.
func Middleware(v *Validator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if v == nil {
				next.ServeHTTP(w, r.WithContext(WithIdentity(r.Context(), Identity{})))
				return
			}

			id := Identity{Validated: true}

			if raw := strings.TrimSpace(r.Header.Get(clientAssertionHeader)); raw != "" {
				cc, err := v.ExtractClientID(raw)
				if err != nil {
					reject(w, r, clientAssertionHeader, err)
					return
				}
				id.ClientID = cc.ClientID
			}
			if raw := strings.TrimSpace(r.Header.Get(userTokenHeader)); raw != "" {
				uc, err := v.ValidateUserToken(raw)
				if err != nil {
					reject(w, r, userTokenHeader, err)
					return
				}
				id.UserEmail, id.UserSubject, id.UserID = uc.Email, uc.Subject, uc.UserID
			}

			// Only reached once every token presented actually parsed --
			// never on a reject() path above, so an access log reading this
			// holder never attributes a request to a caller id parsed from a
			// token that failed even to decode. UserID here is still backed
			// by a cryptographically verified x-user-id-token; ClientID is
			// trusted at face value, by design -- see ExtractClientID.
			if h := identityHolderFromContext(r.Context()); h != nil {
				h.UserID, h.ClientID = id.UserID, id.ClientID
			}

			next.ServeHTTP(w, r.WithContext(WithIdentity(r.Context(), id)))
		})
	}
}

// reject logs why (never the token itself) and answers 401 with a generic body.
func reject(w http.ResponseWriter, r *http.Request, which string, err error) {
	slog.WarnContext(r.Context(), "auth: token rejected", "token", which, "err", err)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(apierror.ErrorResponse{Code: http.StatusUnauthorized, Message: "invalid or expired token"})
}
