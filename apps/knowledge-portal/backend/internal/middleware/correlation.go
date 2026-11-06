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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package middleware

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/wso2-open-operations/cs-tools/apps/knowledge-portal/backend/internal/entity"
)

const correlationIDHeader = "X-CSM-Correlation-ID"

// maxClientCorrelationIDLen caps the logged copy of an inbound correlation id.
const maxClientCorrelationIDLen = 64

type (
	correlationIDKey       struct{}
	clientCorrelationIDKey struct{}
)

// CorrelationID is an HTTP middleware that gives every request a fresh,
// server-minted correlation id (UUID v4). The id is stored in the context for
// automatic inclusion in slog records and in the entity client context so it is
// sent on every outgoing entity-service request, and it is returned in the
// response header.
//
// This API is public, so an inbound X-CSM-Correlation-ID is attacker-controlled
// and is never trusted: it is not echoed, not stored as the request's id and
// never forwarded to entity-service, where it could be used to forge or collide
// with ids in another system's logs. It is kept only as a separate, sanitised
// and length-capped client-supplied field on this request's log records, so a
// caller can still be matched to their own request.
func CorrelationID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := newCorrelationID()
		w.Header().Set(correlationIDHeader, id)
		ctx := context.WithValue(r.Context(), correlationIDKey{}, id)
		ctx = entity.WithCorrelationID(ctx, id)
		if client := sanitizeClientCorrelationID(r.Header.Get(correlationIDHeader)); client != "" {
			ctx = context.WithValue(ctx, clientCorrelationIDKey{}, client)
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// sanitizeClientCorrelationID makes an inbound value safe to log: every byte
// outside letters, digits, '-', '_' and '.' becomes '_' (so no control
// characters or line breaks, and no multi-byte sequences) and the result is cut
// to maxClientCorrelationIDLen.
func sanitizeClientCorrelationID(raw string) string {
	if len(raw) > maxClientCorrelationIDLen {
		raw = raw[:maxClientCorrelationIDLen]
	}
	b := []byte(raw)
	for i, c := range b {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-', c == '_', c == '.':
		default:
			b[i] = '_'
		}
	}
	return string(b)
}

// ClientCorrelationIDFromContext returns the sanitised, caller-supplied
// correlation id for this request, or "" if the caller sent none.
func ClientCorrelationIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(clientCorrelationIDKey{}).(string)
	return v
}

// CorrelationIDFromContext returns the correlation ID stored in ctx, or ""
// if the CorrelationID middleware was not applied.
func CorrelationIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(correlationIDKey{}).(string)
	return v
}

func newCorrelationID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("correlationid: failed to read random bytes: " + err.Error())
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant bits
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// ctxHandler wraps a slog.Handler to automatically inject the correlation ID
// from the request context into every log record produced via *Context methods.
type ctxHandler struct {
	inner slog.Handler
}

func (h ctxHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h ctxHandler) Handle(ctx context.Context, r slog.Record) error {
	if id := CorrelationIDFromContext(ctx); id != "" {
		r.AddAttrs(slog.String("correlationID", id))
	}
	if id := ClientCorrelationIDFromContext(ctx); id != "" {
		r.AddAttrs(slog.String("clientCorrelationID", id))
	}
	return h.inner.Handle(ctx, r)
}

func (h ctxHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return ctxHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h ctxHandler) WithGroup(name string) slog.Handler {
	return ctxHandler{inner: h.inner.WithGroup(name)}
}

// ConfigureLogger sets up the default slog logger with a handler that
// automatically adds the correlation ID from the request context to every log
// record. It writes directly to os.Stderr to avoid a deadlock: slog.SetDefault
// also redirects log.std's output through the new handler, and if the inner
// handler were the old defaultHandler (which itself writes to log.std) a cycle
// would form. Call once at startup before any log statements.
func ConfigureLogger() {
	inner := slog.NewTextHandler(os.Stderr, nil)
	slog.SetDefault(slog.New(ctxHandler{inner: inner}))
}
