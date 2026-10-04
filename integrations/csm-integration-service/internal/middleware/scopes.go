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
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
)

// assertionHeader carries the caller's access token as the API gateway forwards
// it after authenticating the caller. The token is decoded here, never
// signature-verified: it is minted by the gateway and arrives over a path this
// service already trusts, so re-verifying it adds nothing (the same reasoning
// the entity service applies to its own client-assertion handling). What this
// service reads from it is the scope claim only, to decide whether the caller
// may invoke the operation it is calling.
const assertionHeader = "x-jwt-assertion" // #nosec G101 -- header name, not a credential

// maxAssertionLen bounds the token this service is willing to decode. A real
// client-credentials token is a few KiB; anything larger is not one.
const maxAssertionLen = 16 << 10

// Error messages: the same text as the handler package's constants, so every
// error body on the wire reads identically regardless of which layer wrote it.
const (
	msgUnauthorized = "You are not authorized to perform this action. Please try again."
	msgForbidden    = "Access to the requested resource is forbidden!"
)

var (
	errNoToken        = errors.New("no caller token forwarded")
	errTokenTooLarge  = errors.New("caller token exceeds the size limit")
	errTokenMalformed = errors.New("caller token is not a decodable JWT")
)

// ScopeGuard enforces one required OAuth2 scope per route, read from the
// scope claim of the gateway-forwarded caller token. It is the service's only
// authorization check: the gateway authenticates the caller and validates the
// subscription, and this guard decides which operations that caller may use.
type ScopeGuard struct {
	enforce bool
}

// NewScopeGuard returns a guard that checks scopes when enforce is true and
// passes every request through untouched when it is false. Enforcement is the
// deployed default; disabling it is for local development only.
func NewScopeGuard(enforce bool) *ScopeGuard {
	return &ScopeGuard{enforce: enforce}
}

// Enforcing reports whether the guard checks scopes.
func (g *ScopeGuard) Enforcing() bool {
	return g.enforce
}

// Require returns a handler that serves next only when the forwarded caller
// token carries scope. It fails closed:
//   - no token, an oversized token, or one that cannot be decoded: 401
//   - a token without the scope, including one with no scope claim at all: 403
func (g *ScopeGuard) Require(scope string, next http.Handler) http.Handler {
	if !g.enforce {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		granted, err := tokenScopes(r.Header.Get(assertionHeader))
		if err != nil {
			slog.WarnContext(r.Context(), "scope check: no usable caller token", "err", err)
			writeAuthError(w, http.StatusUnauthorized, msgUnauthorized)
			return
		}
		if !granted[scope] {
			slog.WarnContext(r.Context(), "scope check: required scope not granted", "scope", scope)
			writeAuthError(w, http.StatusForbidden, msgForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// tokenScopes decodes the payload of raw (a compact JWT) and returns the set of
// scopes in its "scope" claim, falling back to "scp". Either claim may be a
// space-separated string or an array of strings. A token with neither claim
// yields an empty set, not an error: it is a valid token that grants nothing.
func tokenScopes(raw string) (map[string]bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errNoToken
	}
	if len(raw) > maxAssertionLen {
		return nil, errTokenTooLarge
	}
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return nil, errTokenMalformed
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return nil, errTokenMalformed
	}
	var claims struct {
		Scope json.RawMessage `json:"scope"`
		Scp   json.RawMessage `json:"scp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, errTokenMalformed
	}
	granted := make(map[string]bool)
	for _, s := range scopeClaimValues(claims.Scope) {
		granted[s] = true
	}
	if len(granted) == 0 {
		for _, s := range scopeClaimValues(claims.Scp) {
			granted[s] = true
		}
	}
	return granted, nil
}

// scopeClaimValues normalizes one scope claim into a list of scope names.
func scopeClaimValues(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return strings.Fields(asString)
	}
	var asList []string
	if err := json.Unmarshal(raw, &asList); err == nil {
		var out []string
		for _, s := range asList {
			if s = strings.TrimSpace(s); s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// writeAuthError writes the same {"message": "..."} error body the handler
// package uses, so callers see one error shape for every status.
func writeAuthError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(struct {
		Message string `json:"message"`
	}{Message: message})
}
