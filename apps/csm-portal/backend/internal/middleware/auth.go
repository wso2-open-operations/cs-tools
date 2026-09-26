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
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/entity"
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
	// Roles is the token's "roles" claim, which portal authorisation checks
	// (see handler.AccessGuard).
	Roles []string
}

// Config holds JWT validation configuration.
type Config struct {
	JWKSEndpoint          string
	Issuer                string
	Audiences             []string
	ClockSkew             time.Duration
	TokenValidatorEnabled bool
}

// jwtClaims defines the expected JWT payload fields, mirroring the Ballerina
// CustomJwtPayload in the authorization module.
type jwtClaims struct {
	Email  string     `json:"email"`
	UserID string     `json:"userid"`
	Roles  stringList `json:"roles"`
	jwt.RegisteredClaims
}

// stringList decodes a claim that Asgardeo emits as a bare string when it holds
// one value and as an array when it holds several (its "roles" claim does
// this). A plain []string would reject a single-role user's whole token, so
// both shapes are accepted; anything else fails the token.
type stringList []string

func (l *stringList) UnmarshalJSON(b []byte) error {
	var one string
	if err := json.Unmarshal(b, &one); err == nil {
		if one == "" {
			*l = nil
		} else {
			*l = []string{one}
		}
		return nil
	}
	var many []string
	if err := json.Unmarshal(b, &many); err != nil {
		return fmt.Errorf("claim must be a string or an array of strings: %w", err)
	}
	*l = many
	return nil
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
			addSecurityHeaders(w)

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
// "x5c" as X.509 certificates, and some IdPs (e.g. Asgardeo) publish certs
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

	var jwks struct {
		Keys []map[string]any `json:"keys"`
	}
	if err := json.Unmarshal(body, &jwks); err != nil {
		// Not a JWKS document we can sanitize; hand back the original body untouched.
		resp.Body = io.NopCloser(bytes.NewReader(body))
		return resp, nil
	}

	for _, key := range jwks.Keys {
		delete(key, "x5c")
	}
	sanitized, err := json.Marshal(jwks)
	if err != nil {
		return nil, fmt.Errorf("marshal sanitized JWKS: %w", err)
	}

	resp.Body = io.NopCloser(bytes.NewReader(sanitized))
	resp.ContentLength = int64(len(sanitized))
	resp.Header.Set("Content-Length", fmt.Sprint(len(sanitized)))
	return resp, nil
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

	return &UserInfo{
		Email:  c.Email,
		UserID: c.UserID,
		Roles:  []string(c.Roles),
	}, nil
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

// addSecurityHeaders mirrors the Ballerina ResponseInterceptor security headers.
func addSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "upgrade-insecure-requests")
	w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
}
