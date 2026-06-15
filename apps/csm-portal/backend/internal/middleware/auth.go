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
	"fmt"
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
	Groups []string
}

// Config holds JWT validation configuration.
type Config struct {
	JWKSEndpoint          string
	Issuer                string
	Audience              string
	ClockSkew             time.Duration
	TokenValidatorEnabled bool
}

// jwtClaims defines the expected JWT payload fields, mirroring the Ballerina
// CustomJwtPayload in the authorization module.
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
		jwks, err := keyfunc.NewDefault([]string{cfg.JWKSEndpoint})
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
			jwt.WithAudience(cfg.Audience),
			jwt.WithLeeway(cfg.ClockSkew),
			jwt.WithExpirationRequired(),
		)
		if err != nil {
			return nil, fmt.Errorf("validate token: %w", err)
		}
		if !token.Valid {
			return nil, fmt.Errorf("invalid token")
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
		Groups: c.Groups,
	}, nil
}

// addSecurityHeaders mirrors the Ballerina ResponseInterceptor security headers.
func addSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "upgrade-insecure-requests")
	w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
}
