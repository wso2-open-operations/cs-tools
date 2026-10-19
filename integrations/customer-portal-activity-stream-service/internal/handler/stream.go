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
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/customer-portal-activity-stream-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/integrations/customer-portal-activity-stream-service/internal/stream"
)

var uuidRe = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// caseActivityStreamHeartbeat is how often StreamCaseActivities writes a
// comment-only SSE ping to keep the connection alive through intermediate
// proxies that would otherwise time out an idle response.
const caseActivityStreamHeartbeat = 15 * time.Second

// StreamHandler handles GET /cases/{id}/activities/stream: a long-lived
// Server-Sent Events connection that emits a `case_updated` event whenever
// internal/caseevents.Handler observes a customer-visible case.comment_added or
// case.status_changed record for this case on any backend replica (see internal/stream.BroadcastHub).
// It is registered on the dedicated :9092 listener (see cmd/server/main.go)
// so the health check listener's timeouts can't kill the connection, but it
// sits behind the same middleware.Auth chain as every other endpoint — there
// is no separate auth mechanism for streaming; the browser connects with its
// normal x-jwt-assertion/x-user-id-token headers via a fetch-backed EventSource
// polyfill (native EventSource cannot set custom headers).
//
// The broadcast payload is a minimal {caseId, type, timestamp} — never
// comment text or field values (see events.CommentAddedPayload/StatusChangedPayload)
// — but even that is per-case, so a caller must be authorized to read the
// requested case before subscribing, not merely hold a valid token: see the
// GetCase call below, which registers the subscription only once the same
// upstream ACL check every other case-reading endpoint relies on has passed.
// This is unrelated to internal/caseevents.Handler, which is a server-internal
// component with no external caller and legitimately sees every event system-wide.
//
// Known limitation, not yet addressed: there is no per-user or per-replica
// cap on how many of these a single caller can hold open concurrently, and
// the dedicated :9092 listener runs with WriteTimeout/IdleTimeout disabled
// (see cmd/server/main.go) to keep long-lived connections alive — nothing
// here stops a buggy or malicious client from opening unbounded connections
// and exhausting server resources (goroutines, file descriptors). Add
// bounded admission control (e.g. a per-user semaphore rejecting beyond some
// limit) or confirm and document an enforced platform-level limit before
// relying on this in a hostile-client environment.
type StreamHandler struct {
	entityClient entityCaseClient
	hub          *stream.BroadcastHub
}

// entityCaseClient is the minimal interface StreamHandler needs from the
// entity service — only GetCase for the authorization check before subscribing.
type entityCaseClient interface {
	GetCase(ctx context.Context, caseID string) ([]byte, error)
}

// NewStreamHandler constructs a StreamHandler. hub may be nil —
// StreamCaseActivities checks for that before registering.
func NewStreamHandler(entityClient entityCaseClient, hub *stream.BroadcastHub) *StreamHandler {
	return &StreamHandler{entityClient: entityClient, hub: hub}
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

	// A caller with a valid token but no read access to this specific case
	// must not learn even that it changed. Reuse the same upstream call
	// GetCase itself uses — the caller's forwarded x-user-id-token is what
	// ServiceNow enforces the ACL against — before registering the
	// subscription, so an unauthorized caseID never reaches h.hub.Register.
	if _, err := h.entityClient.GetCase(r.Context(), caseID); err != nil {
		slog.ErrorContext(r.Context(), "entity GetCase failed during stream authorization", "userID", user.UserID, "caseID", caseID, "err", err)
		mapUpstreamErrorGeneric(w, err, "Failed to open the case activity stream.")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, ErrMsgInternal)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	// Nginx/Choreo-gateway hint to disable response buffering for this
	// endpoint; harmless (ignored) on stacks that don't recognise it.
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ch := h.hub.Register(caseID)
	defer h.hub.Unregister(caseID, ch)

	ctx := r.Context()
	ticker := time.NewTicker(caseActivityStreamHeartbeat)
	defer ticker.Stop()

	slog.InfoContext(ctx, "case activity stream connected", "userID", user.UserID, "caseID", caseID)

	for {
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "case activity stream disconnected", "userID", user.UserID, "caseID", caseID)
			return
		case <-ticker.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case payload, ok := <-ch:
			if !ok {
				return
			}
			// payload is always compact, single-line JSON built by
			// internal/caseevents.Handler (see BroadcastHub.Publish's
			// caller) — safe to write as one `data:` line, since
			// json.Marshal escapes any literal newline in a string value
			// rather than emitting one.
			if _, err := fmt.Fprintf(w, "event: case_updated\ndata: %s\n\n", payload); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
