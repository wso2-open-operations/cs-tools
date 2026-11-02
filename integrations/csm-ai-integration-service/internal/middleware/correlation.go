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
	"crypto/rand"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
)

const correlationIDHeader = "X-CSM-Correlation-ID"

// correlationIDPrefix tags every correlation ID this service produces or echoes
// with "cai-" (CSM AI Integration Service), so a support engineer scanning logs
// across services can immediately tell where in the chain an ID originated or was
// last seen.
const correlationIDPrefix = "cai-"

type correlationIDKey struct{}

// CorrelationID is an HTTP middleware that reads the X-CSM-Correlation-ID request
// header or generates a UUID v4 if absent, then ensures the ID carries the "cai-"
// prefix — whether it arrived from the caller or was generated here. An ID that
// already carries the prefix (e.g. echoed back by a caller from a previous
// response of ours) is left unchanged rather than prefixed twice. The ID is:
//   - stored in the context for automatic inclusion in slog records
//   - echoed in the response header so callers can reference it in support
//     requests
func CorrelationID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(correlationIDHeader)
		if id == "" {
			id = newCorrelationID()
		}
		id = ensureCorrelationIDPrefix(id)
		w.Header().Set(correlationIDHeader, id)
		ctx := context.WithValue(r.Context(), correlationIDKey{}, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ensureCorrelationIDPrefix returns id with exactly one "cai-" prefix, stripping
// any repeated occurrences first — e.g. a caller-supplied ID of "cai-cai-foo"
// normalizes to "cai-foo", not left as-is with two prefixes.
func ensureCorrelationIDPrefix(id string) string {
	for strings.HasPrefix(id, correlationIDPrefix) {
		id = strings.TrimPrefix(id, correlationIDPrefix)
	}
	return correlationIDPrefix + id
}

// CorrelationIDFromContext returns the correlation ID stored in ctx, or ""
// if the CorrelationID middleware was not applied.
func CorrelationIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(correlationIDKey{}).(string)
	return v
}

// newCorrelationID is a random version 4 UUID; it panics only if the system's
// random source fails.
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
// and the portal user's id (see UserID) from the request context into every log
// record produced via *Context methods, under the same keys as
// apps/csm-portal/backend's own version, so one search finds a call in both.
type ctxHandler struct {
	inner slog.Handler
}

func (h ctxHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

// Handle adds the request's correlation ID and user id to the record, when the
// context carries them, and passes it to the wrapped handler.
func (h ctxHandler) Handle(ctx context.Context, r slog.Record) error {
	if id := CorrelationIDFromContext(ctx); id != "" {
		r.AddAttrs(slog.String("correlationID", id))
	}
	if id := UserIDFromContext(ctx); id != "" {
		r.AddAttrs(slog.String("userID", id))
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
// automatically adds the correlation ID and user id from the request context to
// every log record. It writes directly to os.Stderr to avoid a deadlock: slog.SetDefault
// also redirects log.std's output through the new handler, and if the inner
// handler were the old defaultHandler (which itself writes to log.std) a cycle
// would form. Call once at startup before any log statements.
func ConfigureLogger() {
	inner := slog.NewTextHandler(os.Stderr, nil)
	slog.SetDefault(slog.New(ctxHandler{inner: inner}))
}
