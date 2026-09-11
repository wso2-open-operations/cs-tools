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

// correlationIDPrefix tags every correlation ID this service produces or
// echoes with "sais-" (SRE Alert Ingestion Service), so a support engineer
// scanning logs across services can immediately tell where in the chain an
// ID originated or was last seen.
const correlationIDPrefix = "sais-"

type correlationIDKey struct{}

// CorrelationID is an HTTP middleware that reads the X-CSM-Correlation-ID
// request header or generates a UUID v4 if absent, then ensures the ID
// carries the "sais-" prefix — whether it arrived from the caller or was
// generated here. An ID that already carries the prefix is left unchanged
// rather than prefixed twice. The ID is:
//   - stored in the context for automatic inclusion in slog records
//   - propagated onto every background worker attempt for a given buffered
//     alert, via the same context key (see internal/worker)
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

// ensureCorrelationIDPrefix returns id with exactly one "sais-" prefix,
// stripping any repeated occurrences first — e.g. a caller-supplied ID of
// "sais-sais-foo" normalizes to "sais-foo", not left as-is with two prefixes.
func ensureCorrelationIDPrefix(id string) string {
	for strings.HasPrefix(id, correlationIDPrefix) {
		id = strings.TrimPrefix(id, correlationIDPrefix)
	}
	return correlationIDPrefix + id
}

// CorrelationIDFromContext returns the correlation ID stored in ctx, or ""
// if the CorrelationID middleware was not applied (or ctx doesn't descend
// from a request context at all, e.g. the background worker's own root
// context for a given attempt — see WithCorrelationID).
func CorrelationIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(correlationIDKey{}).(string)
	return v
}

// WithCorrelationID returns a copy of ctx carrying id for slog enrichment and
// downstream propagation — used by the background worker to attach a fresh,
// "sais-"-prefixed ID to each buffered-alert delivery attempt, since those
// attempts have no inbound HTTP request for the CorrelationID middleware to
// have run against.
func WithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, correlationIDKey{}, ensureCorrelationIDPrefix(id))
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

// NewCorrelationID generates a fresh "sais-"-prefixed correlation ID, for
// callers that need one without an inbound request (e.g. the worker, once
// per delivery attempt).
func NewCorrelationID() string {
	return correlationIDPrefix + newCorrelationID()
}

// ctxHandler wraps a slog.Handler to automatically inject the correlation ID
// from the context into every log record produced via *Context methods.
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
	return h.inner.Handle(ctx, r)
}

func (h ctxHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return ctxHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h ctxHandler) WithGroup(name string) slog.Handler {
	return ctxHandler{inner: h.inner.WithGroup(name)}
}

// ConfigureLogger sets up the default slog logger with a handler that
// automatically adds the correlation ID from the request context to every
// log record. It writes directly to os.Stderr to avoid a deadlock: see
// csm-integration-service's own ConfigureLogger for the full explanation.
// Call once at startup before any log statements.
func ConfigureLogger() {
	inner := slog.NewTextHandler(os.Stderr, nil)
	slog.SetDefault(slog.New(ctxHandler{inner: inner}))
}
