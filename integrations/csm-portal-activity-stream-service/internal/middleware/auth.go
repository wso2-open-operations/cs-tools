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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-portal-activity-stream-service/internal/entity"
)

// authErrorBody is the JSON error payload for auth failures.
type authErrorBody struct {
	Message string `json:"message"`
}

func writeAuthError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(authErrorBody{Message: message})
}

const jwtAssertionHeader = "x-jwt-assertion"

type contextKey string

const userInfoKey contextKey = "user-info"

// UserInfo holds the authenticated user's identity extracted from the JWT.
type UserInfo struct {
	Email  string
	UserID string
	Groups []string
	// ExpiresAt is the token's exp claim. Zero when the token carries none
	// (only possible with TokenValidatorEnabled=false, since validation
	// requires exp). Long-lived handlers use it to bound their own lifetime
	// to the credential that opened them — see handler.StreamCaseActivities.
	ExpiresAt time.Time
}

// Config holds JWT validation configuration.
type Config struct {
	JWKSEndpoint          string
	Issuer                string
	Audiences             []string
	ClockSkew             time.Duration
	TokenValidatorEnabled bool
}

// jwtClaims defines the expected JWT payload fields — the custom claims the
// upstream gateway's JWT carries alongside the registered ones.
type jwtClaims struct {
	Email  string   `json:"email"`
	UserID string   `json:"userid"`
	Groups []string `json:"groups"`
	jwt.RegisteredClaims
}

// Auth returns an HTTP middleware that validates the x-jwt-assertion header on
// every request and stores the resulting UserInfo in the request context.
// When Config.TokenValidatorEnabled is false the token is only decoded without
// signature verification — safe for local development only.
func Auth(cfg Config) func(http.Handler) http.Handler {
	var keyFunc jwt.Keyfunc
	if cfg.TokenValidatorEnabled {
		client := &http.Client{Transport: &x5cStrippingTransport{base: http.DefaultTransport}}
		jwks, err := keyfunc.NewDefaultOverrideCtx(context.Background(), []string{cfg.JWKSEndpoint}, keyfunc.Override{Client: client})
		if err != nil {
			// Misconfigured auth must not silently pass — fail at startup.
			panic("auth: failed to initialise JWKS from " + cfg.JWKSEndpoint + ": " + err.Error())
		}
		keyFunc = jwks.Keyfunc
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip auth for the health check endpoint.
			if r.Method == http.MethodGet && r.URL.Path == "/health" {
				next.ServeHTTP(w, r)
				return
			}

			tokenStr := r.Header.Get(jwtAssertionHeader)
			if tokenStr == "" {
				writeAuthError(w, "You are not authorized to perform this action. Please try again.")
				return
			}

			info, err := extractUserInfo(tokenStr, cfg, keyFunc)
			if err != nil {
				slog.ErrorContext(r.Context(), "auth: token validation failed", "err", err)
				writeAuthError(w, "You are not authorized to perform this action. Please try again.")
				return
			}

			ctx := context.WithValue(r.Context(), userInfoKey, info)
			userIDToken := r.Header.Get("x-user-id-token")
			if userIDToken == "" {
				userIDToken = tokenStr
			}
			ctx = entity.WithUserIDToken(ctx, userIDToken)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// x5cStrippingTransport removes the "x5c" certificate chain from every key in
// a JWKS response before it reaches the jwkset parser. Verification only
// needs "n"/"e" (or the EC/OKP equivalents); jwkset unconditionally parses
// "x5c" as X.509 certificates, and some identity providers publish certs
// with a negative serial number that Go's x509 parser rejects since Go 1.23,
// which would otherwise make the whole JWK Set fail to load.
type x5cStrippingTransport struct {
	base http.RoundTripper
}

func (t *x5cStrippingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return resp, err
	}

	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("read JWKS response body: %w", err)
	}

	sanitized, ok := stripX5C(body)
	if !ok {
		// Not a JWKS document we can sanitize (e.g. a non-JWKS JSON body the server
		// returned with a 200 status); hand back the original body untouched rather than
		// risk fabricating a {"keys":null} document a naive unmarshal-then-remarshal would
		// silently produce for any valid JSON that just happens to have no "keys" array.
		resp.Body = io.NopCloser(bytes.NewReader(body))
		return resp, nil
	}

	resp.Body = io.NopCloser(bytes.NewReader(sanitized))
	resp.ContentLength = int64(len(sanitized))
	resp.Header.Set("Content-Length", fmt.Sprint(len(sanitized)))
	return resp, nil
}

// stripX5C removes the "x5c" field from every entry in body's top-level "keys" array.
// Returns ok=false — with sanitized left nil — when body isn't a JWKS document (no
// top-level "keys" array), so the caller can fall back to passing the original body
// through unchanged instead of replacing it with a fabricated result.
func stripX5C(body []byte) (sanitized []byte, ok bool) {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, false
	}
	rawKeys, present := doc["keys"]
	if !present {
		return nil, false
	}
	var keys []map[string]any
	if err := json.Unmarshal(rawKeys, &keys); err != nil {
		return nil, false
	}

	for _, key := range keys {
		delete(key, "x5c")
	}
	keysJSON, err := json.Marshal(keys)
	if err != nil {
		return nil, false
	}
	doc["keys"] = keysJSON

	sanitized, err = json.Marshal(doc)
	if err != nil {
		return nil, false
	}
	return sanitized, true
}

// UserInfoFromContext retrieves the authenticated user's info from the context.
// Returns nil if the auth middleware was not applied.
func UserInfoFromContext(ctx context.Context) *UserInfo {
	v, _ := ctx.Value(userInfoKey).(*UserInfo)
	return v
}

// WithUserInfo returns a copy of ctx carrying the given UserInfo.
// Call this in tests to bypass JWT parsing and inject a fake authenticated user.
func WithUserInfo(ctx context.Context, user *UserInfo) context.Context {
	return context.WithValue(ctx, userInfoKey, user)
}

func extractUserInfo(tokenStr string, cfg Config, keyFunc jwt.Keyfunc) (*UserInfo, error) {
	var c jwtClaims

	if !cfg.TokenValidatorEnabled {
		// Local mode: decode without signature verification.
		_, _, err := new(jwt.Parser).ParseUnverified(tokenStr, &c)
		if err != nil {
			return nil, fmt.Errorf("decode token: %w", err)
		}
	} else {
		token, err := jwt.ParseWithClaims(tokenStr, &c, keyFunc,
			jwt.WithIssuer(cfg.Issuer),
			jwt.WithLeeway(cfg.ClockSkew),
			jwt.WithExpirationRequired(),
		)
		if err != nil {
			return nil, fmt.Errorf("validate token: %w", err)
		}
		if !token.Valid {
			return nil, fmt.Errorf("invalid token")
		}
		if len(cfg.Audiences) > 0 {
			tokenAuds, _ := token.Claims.GetAudience()
			if !hasAnyAudience(tokenAuds, cfg.Audiences) {
				return nil, fmt.Errorf("token audience not accepted")
			}
		}
	}

	if c.Email == "" {
		return nil, fmt.Errorf("token missing email claim")
	}
	if c.UserID == "" {
		return nil, fmt.Errorf("token missing userid claim")
	}

	info := &UserInfo{
		Email:  c.Email,
		UserID: c.UserID,
		Groups: c.Groups,
	}
	if c.ExpiresAt != nil {
		info.ExpiresAt = c.ExpiresAt.Time
	}
	return info, nil
}

func hasAnyAudience(tokenAuds jwt.ClaimStrings, expected []string) bool {
	for _, want := range expected {
		for _, got := range tokenAuds {
			if got == want {
				return true
			}
		}
	}
	return false
}
