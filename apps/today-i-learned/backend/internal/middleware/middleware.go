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

// Package middleware holds the HTTP layers every request passes through:
// security headers, CORS, a correlation id, a body size limit and token
// verification. Requests aren't logged one by one: the API gateway's own
// logs record each call under the same correlation id.
package middleware

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
)

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(struct {
		Message string `json:"message"`
	}{message})
}

// SecurityHeaders sets the headers every response carries.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		next.ServeHTTP(w, r)
	})
}

const (
	corsAllowedMethods = "GET, POST, PUT, DELETE, OPTIONS"
	corsAllowedHeaders = "Content-Type, Authorization, x-jwt-assertion, X-Correlation-ID"
)

// CORS lets the listed browser origins call this API and answers their
// preflight requests. With no origins listed, no cross-origin browser request
// is allowed. Credentials are never allowed: the token travels in a header,
// not a cookie. It must wrap Auth, because a preflight carries no token.
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if origin := r.Header.Get("Origin"); origin != "" {
				// The response differs by Origin whether or not it's allowed.
				w.Header().Add("Vary", "Origin")
				if allowed[origin] {
					w.Header().Set("Access-Control-Allow-Origin", origin)
				}
			}
			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				w.Header().Set("Access-Control-Allow-Methods", corsAllowedMethods)
				w.Header().Set("Access-Control-Allow-Headers", corsAllowedHeaders)
				w.Header().Set("Access-Control-Max-Age", "3600")
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

const correlationIDHeader = "X-Correlation-ID"

type correlationIDKey struct{}

// CorrelationID gives each request an id, taken from X-Correlation-ID when
// the caller sent one. It is echoed back in the response and added to every
// log line written for the request.
func CorrelationID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(correlationIDHeader)
		if id == "" || len(id) > 64 {
			id = newCorrelationID()
		}
		w.Header().Set(correlationIDHeader, id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), correlationIDKey{}, id)))
	})
}

func newCorrelationID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// ConfigureLogger makes slog's default logger add the request's correlation
// id and the signed-in user's subject to every line logged with a request
// context, writing to w. Call it once at startup.
func ConfigureLogger(w io.Writer) {
	slog.SetDefault(slog.New(contextHandler{inner: slog.NewTextHandler(w, nil)}))
}

type contextHandler struct {
	inner slog.Handler
}

func (h contextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h contextHandler) Handle(ctx context.Context, r slog.Record) error {
	if id, _ := ctx.Value(correlationIDKey{}).(string); id != "" {
		r.AddAttrs(slog.String("correlationID", id))
	}
	if u := UserFromContext(ctx); u != nil && u.Subject != "" {
		r.AddAttrs(slog.String("userID", u.Subject))
	}
	return h.inner.Handle(ctx, r)
}

func (h contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return contextHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h contextHandler) WithGroup(name string) slog.Handler {
	return contextHandler{inner: h.inner.WithGroup(name)}
}

// BodyLimit rejects a request body larger than max bytes with a 413: up
// front when Content-Length says so, and while reading otherwise.
func BodyLimit(max int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > max {
				writeError(w, http.StatusRequestEntityTooLarge, "The request is too large.")
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, max)
			next.ServeHTTP(w, r)
		})
	}
}
