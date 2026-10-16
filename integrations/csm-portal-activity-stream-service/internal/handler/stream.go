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

package handler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-portal-activity-stream-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-portal-activity-stream-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-portal-activity-stream-service/internal/stream"
)

var uuidRe = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// limitRetryAfter is the Retry-After hint on a 429/503 refused by the
// connection limiter. Advisory: the browser polyfill applies its own backoff.
const limitRetryAfter = 30 * time.Second

// streamRetry is the reconnect delay advertised with the SSE `retry:` field
// at the start of every stream.
const streamRetry = 3 * time.Second

// maxLastEventIDLen bounds the Last-Event-ID request header the handler will
// consider; the hub's own IDs are far shorter.
const maxLastEventIDLen = 64

// caseActivityStreamHeartbeat is how often StreamCaseActivities writes a
// comment-only SSE ping to keep the connection alive through intermediate
// proxies that would otherwise time out an idle response.
const caseActivityStreamHeartbeat = 15 * time.Second

// Defaults for the stream-lifetime bounds. Overridable per StreamHandler via
// WithMaxLifetime / WithReauthInterval (wired to STREAM_MAX_LIFETIME and
// STREAM_REAUTH_INTERVAL in cmd/server/main.go).
const (
	// DefaultMaxStreamLifetime caps how long one connection may stay open
	// even when the caller's token outlives it. A client that still wants
	// updates reconnects with a fresh token; the browser hook does this on
	// its own.
	DefaultMaxStreamLifetime = time.Hour
	// DefaultReauthInterval is how often the connect-time case authorization
	// (the upstream GetCase call) is repeated for an open stream.
	DefaultReauthInterval = 10 * time.Minute
)

// SSE event names and close reasons written by StreamCaseActivities besides
// the `case_updated` data event. EventStreamClosed is terminal: it is the
// last thing written before the server ends the response, so a client can
// tell a deliberate server-side close from a dropped connection. Its data
// is `{"reason": "<reason>"}`.
const (
	EventStreamClosed = "stream_closed"
	// EventShutdown is written when this replica is shutting down. Its data
	// is {"reason": "shutdown"}; the client should reconnect, which the
	// load balancer routes to a replica that is still serving.
	EventShutdown = "shutdown"
	// ReasonShutdown is EventShutdown's reason.
	ReasonShutdown = "shutdown"

	// ReasonTokenExpired: the token that opened the stream reached its exp.
	ReasonTokenExpired = "token_expired"
	// ReasonMaxLifetime: the configured maximum stream lifetime elapsed.
	ReasonMaxLifetime = "max_lifetime"
	// ReasonAccessRevoked: a periodic re-authorization found the caller can
	// no longer read the case (upstream answered 401/403/404).
	ReasonAccessRevoked = "access_revoked"
)

// StreamHandler handles GET /cases/{id}/activities/stream: a long-lived
// Server-Sent Events connection that emits a `case_updated` event whenever
// internal/caseevents.Handler observes a case.comment_added or case.status_changed
// record for this case on any backend replica (see internal/stream.BroadcastHub).
// It is registered on the dedicated :9092 listener (see cmd/server/main.go)
// so the health check listener's timeouts can't kill the connection, but it
// sits behind the same middleware.Auth chain as every other endpoint — there
// is no separate auth mechanism for streaming; the browser connects with its
// normal x-jwt-assertion/x-user-id-token headers via a fetch-backed EventSource
// polyfill (native EventSource cannot set custom headers).
//
// The broadcast payload is a minimal {caseId, type, timestamp} — never
// comment text or field values (see internal/caseevents.Handler) — but even
// that is per-case, so a caller must be authorized to read the
// requested case before subscribing, not merely hold a valid token: see the
// GetCase call below, which registers the subscription only once the same
// upstream ACL check every other case-reading endpoint relies on has passed.
// This is unrelated to internal/caseevents.Handler, which is a server-internal
// component with no external caller and legitimately sees every event system-wide.
//
// Lifetime. Identity and case access are checked at connect, but a stream can
// outlive both — the token expires, the user signs out elsewhere, or loses
// access to the case — so the handler bounds itself: the request context gets
// a deadline at min(token exp, now + maxLifetime), and the GetCase check is
// repeated every reauthInterval. Either bound ending the stream is announced
// with a terminal EventStreamClosed event so the client can reconnect (with a
// fresh token) or stop, as appropriate. A transient upstream failure during
// re-authorization (5xx, network) is logged and the stream kept open; only a
// definitive 401/403/404 closes it — the lifetime cap bounds the worst case.
//
// Admission. The dedicated :9092 listener runs with WriteTimeout/IdleTimeout
// disabled (see cmd/server/main.go) to keep long-lived connections alive, so
// every open stream holds a goroutine, a file descriptor and a hub
// subscription until the client leaves or the lifetime bound fires. A
// connLimiter caps that per user (429 Too Many Requests) and per replica
// (503 Service Unavailable), both with Retry-After, before the upstream
// authorization call is made — see WithConnectionLimits for the defaults.
type StreamHandler struct {
	entityClient   entityCaseClient
	hub            *stream.BroadcastHub
	maxLifetime    time.Duration
	reauthInterval time.Duration
	limiter        *connLimiter
}

// entityCaseClient is the minimal interface StreamHandler needs from the
// entity service — only GetCase for the authorization check before subscribing.
type entityCaseClient interface {
	GetCase(ctx context.Context, caseID string) ([]byte, error)
}

// Option configures a StreamHandler beyond its required dependencies.
type Option func(*StreamHandler)

// WithMaxLifetime overrides DefaultMaxStreamLifetime. Non-positive values are
// ignored.
func WithMaxLifetime(d time.Duration) Option {
	return func(h *StreamHandler) {
		if d > 0 {
			h.maxLifetime = d
		}
	}
}

// WithReauthInterval overrides DefaultReauthInterval. Non-positive values are
// ignored.
func WithReauthInterval(d time.Duration) Option {
	return func(h *StreamHandler) {
		if d > 0 {
			h.reauthInterval = d
		}
	}
}

// WithConnectionLimits overrides DefaultMaxStreamsPerUser and
// DefaultMaxStreamsTotal: the maximum number of concurrently open streams per
// authenticated user and per replica. 0 disables that dimension; negative
// values are ignored.
func WithConnectionLimits(perUser, total int) Option {
	return func(h *StreamHandler) {
		if perUser < 0 {
			perUser = h.limiter.maxPerUser
		}
		if total < 0 {
			total = h.limiter.maxTotal
		}
		h.limiter = newConnLimiter(perUser, total)
	}
}

// NewStreamHandler constructs a StreamHandler. hub may be nil —
// StreamCaseActivities checks for that before registering.
func NewStreamHandler(entityClient entityCaseClient, hub *stream.BroadcastHub, opts ...Option) *StreamHandler {
	h := &StreamHandler{
		entityClient:   entityClient,
		hub:            hub,
		maxLifetime:    DefaultMaxStreamLifetime,
		reauthInterval: DefaultReauthInterval,
		limiter:        newConnLimiter(DefaultMaxStreamsPerUser, DefaultMaxStreamsTotal),
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// StreamCaseActivities handles GET /cases/{id}/activities/stream.
func (h *StreamHandler) StreamCaseActivities(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserInfoFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, ErrMsgUnauthorized)
		return
	}

	caseID := r.PathValue("id")
	if caseID == "" || !uuidRe.MatchString(caseID) {
		writeError(w, http.StatusBadRequest, ErrMsgInvalidUUID)
		return
	}

	if h.hub == nil {
		writeError(w, http.StatusServiceUnavailable, "Live updates are not available right now.")
		return
	}

	// Admission control runs before the upstream authorization call so a
	// refused connection costs no entity-service round-trip.
	release, outcome := h.limiter.acquire(user.UserID)
	defer release()
	switch outcome {
	case userLimitReached:
		w.Header().Set("Retry-After", strconv.Itoa(int(limitRetryAfter/time.Second)))
		slog.WarnContext(r.Context(), "case activity stream refused: per-user connection limit reached", "caseID", caseID, "limit", h.limiter.maxPerUser)
		writeError(w, http.StatusTooManyRequests, ErrMsgTooManyStreams)
		return
	case replicaLimitReached:
		w.Header().Set("Retry-After", strconv.Itoa(int(limitRetryAfter/time.Second)))
		slog.WarnContext(r.Context(), "case activity stream refused: replica connection limit reached", "caseID", caseID, "limit", h.limiter.maxTotal)
		writeError(w, http.StatusServiceUnavailable, ErrMsgStreamCapacity)
		return
	}

	// A caller with a valid token but no read access to this specific case
	// must not learn even that it changed. Reuse the same upstream call
	// GetCase itself uses — the entity service resolves the caller's case
	// access from the forwarded x-user-id-token, whichever data source backs
	// it — before registering the subscription, so an unauthorized caseID
	// never reaches h.hub.Subscribe.
	if _, err := h.entityClient.GetCase(r.Context(), caseID); err != nil {
		slog.ErrorContext(r.Context(), "entity GetCase failed during stream authorization", "caseID", caseID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to open the case activity stream.")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}

	// Bound the stream to the credential that opened it (see the type's doc
	// comment). ExpiresAt is zero only when signature validation is off.
	deadline := time.Now().Add(h.maxLifetime)
	closeReason := ReasonMaxLifetime
	if !user.ExpiresAt.IsZero() && user.ExpiresAt.Before(deadline) {
		deadline = user.ExpiresAt
		closeReason = ReasonTokenExpired
	}
	ctx, cancel := context.WithDeadline(r.Context(), deadline)
	defer cancel()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	// Nginx/Choreo-gateway hint to disable response buffering for this
	// endpoint; harmless (ignored) on stacks that don't recognise it.
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	// Pace the client's automatic reconnect (an SSE `retry:` field on its
	// own dispatches no event).
	if _, err := fmt.Fprintf(w, "retry: %d\n\n", streamRetry.Milliseconds()); err != nil {
		return
	}
	flusher.Flush()

	// Best-effort resume: a client reconnecting with Last-Event-ID gets the
	// events for this case it missed, if this replica still holds them (see
	// stream.ReplayCapacity). Over-long values are ignored, not parsed.
	lastEventID := r.Header.Get("Last-Event-ID")
	if len(lastEventID) > maxLastEventIDLen {
		lastEventID = ""
	}
	ch, missed := h.hub.Subscribe(caseID, lastEventID)
	defer h.hub.Unregister(caseID, ch)
	for _, ev := range missed {
		if err := writeCaseUpdated(w, ev); err != nil {
			return
		}
	}
	if len(missed) > 0 {
		flusher.Flush()
	}

	ticker := time.NewTicker(caseActivityStreamHeartbeat)
	defer ticker.Stop()
	reauth := time.NewTicker(h.reauthInterval)
	defer reauth.Stop()

	slog.InfoContext(ctx, "case activity stream connected", "caseID", caseID, "deadline", deadline)

	for {
		select {
		case <-ctx.Done():
			if r.Context().Err() != nil {
				// The client went away (or the server is shutting the
				// connection down) — nothing left to write to.
				slog.InfoContext(ctx, "case activity stream disconnected", "caseID", caseID)
				return
			}
			// Our own deadline: the client is still connected, so tell it
			// why the stream is ending before closing.
			writeTerminalEvent(w, flusher, EventStreamClosed, closeReason)
			slog.InfoContext(ctx, "case activity stream closed", "caseID", caseID, "reason", closeReason)
			return
		case <-reauth.C:
			if _, err := h.entityClient.GetCase(ctx, caseID); err != nil {
				if isAccessDenied(err) {
					writeTerminalEvent(w, flusher, EventStreamClosed, ReasonAccessRevoked)
					slog.WarnContext(ctx, "case activity stream closed", "caseID", caseID, "reason", ReasonAccessRevoked, "err", err)
					return
				}
				slog.WarnContext(ctx, "case activity stream re-authorization failed transiently; keeping stream open", "caseID", caseID, "err", err)
			}
		case <-ticker.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case ev, ok := <-ch:
			if !ok {
				// Only BroadcastHub.CloseAll closes a channel while its
				// subscriber is still reading (Unregister runs after this
				// function returns), i.e. the server is shutting down.
				writeTerminalEvent(w, flusher, EventShutdown, ReasonShutdown)
				slog.InfoContext(ctx, "case activity stream closed", "caseID", caseID, "reason", ReasonShutdown)
				return
			}
			if err := writeCaseUpdated(w, ev); err != nil {
				// The client is gone or stalled past the transport's
				// buffers; return so the subscription is released.
				return
			}
			flusher.Flush()
		}
	}
}

// writeCaseUpdated writes ev as a `case_updated` SSE event with its `id:`.
// ev.Data is always compact, single-line JSON built by
// internal/caseevents.Handler — safe to write as one `data:` line, since
// json.Marshal escapes any literal newline in a string value rather than
// emitting one — and ev.ID is the hub's `<epoch>-<seq>`, [0-9a-z-] only.
func writeCaseUpdated(w io.Writer, ev stream.Event) error {
	_, err := fmt.Fprintf(w, "id: %s\nevent: case_updated\ndata: %s\n\n", ev.ID, ev.Data)
	return err
}

// isAccessDenied reports whether err is a definitive upstream answer that the
// caller may not read the case (as opposed to a transient failure).
func isAccessDenied(err error) bool {
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) {
		return false
	}
	switch apiErr.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound:
		return true
	}
	return false
}

// writeTerminalEvent writes a named SSE event whose data is {"reason": reason}
// and flushes it. Write errors are ignored: the caller returns right after,
// and a failed write just means the client is already gone.
func writeTerminalEvent(w io.Writer, flusher http.Flusher, event, reason string) {
	_, _ = fmt.Fprintf(w, "event: %s\ndata: {\"reason\":%q}\n\n", event, reason)
	flusher.Flush()
}
