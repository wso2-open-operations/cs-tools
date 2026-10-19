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

// Package caseevents is a consumer of the case-events Kafka topic, in its
// own consumer group (see cmd/server/main.go) so it gets its own full copy
// of every event independent of csm-notification-service's and the sibling
// csm-portal-activity-stream-service's own consumers.
// Logs type/entityId for every event, and — for the event types the
// case-activity SSE stream cares about (comment added, status changed) — fans a minimal broadcast payload
// out to internal/stream.BroadcastHub, which is what actually pushes the
// `case_updated` event to any browser subscribed to that case on this
// replica (see internal/handler.StreamCaseActivities).
package caseevents

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/wso2-open-operations/cs-tools/integrations/customer-portal-activity-stream-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/integrations/customer-portal-activity-stream-service/internal/events"
)

// broadcastHub abstracts stream.BroadcastHub for testability.
type broadcastHub interface {
	Publish(caseID, payload string)
}

// Handler reacts to events on the case-events topic. hub may be nil — every
// broadcast call site below must check for that — since Event Hub config
// (and therefore the whole case-events consumer) is optional in this
// service (see cmd/server/main.go).
type Handler struct {
	hub broadcastHub
}

// NewHandler constructs a Handler that fans case.comment_added (customer-visible
// comments only) and case.status_changed events out to hub. Assignment,
// severity and acknowledgement events are deliberately not broadcast: they are
// not relevant to customers. Pass nil to disable broadcasting
// (Handle will still log every event as before).
func NewHandler(hub broadcastHub) *Handler {
	return &Handler{hub: hub}
}

// broadcastPayload is the minimal shape written to the SSE stream — never
// comment text, field values, names or emails (the browser refetches through
// authorization-checked APIs on receipt), see StreamCaseActivities' doc comment for
// why.
type broadcastPayload struct {
	CaseID    string `json:"caseId"`
	Type      string `json:"type"`
	Timestamp string `json:"timestamp,omitempty"`
}

// Handle implements eventbus.Handle. Never returns an error today — a
// malformed record is logged and dropped rather than retried, since
// nothing here has a failure mode retrying would fix (see
// eventbus.Consumer.Run's doc comment on why this consumer skips
// retry/dead-letter handling for now).
//
// Deliberately does not log payload: it can carry PII (recipient emails,
// comment text, etc. — see the case.* payloads in csm-notification-service's
// internal/events), and this service's own logging convention is IDs and
// error summaries only, never request/event bodies.
func (h *Handler) Handle(ctx context.Context, record eventbus.Record) error {
	var env events.Envelope
	if err := json.Unmarshal(record.Value, &env); err != nil {
		slog.ErrorContext(ctx, "caseevents: failed to decode event", "err", err)
		return nil
	}
	slog.InfoContext(ctx, "caseevents: received case event", "type", env.Type, "entityId", env.EntityID)

	if h.hub == nil || env.EntityID == "" {
		return nil
	}
	if env.Type == events.TypeCommentAdded && isInternalNote(env.Payload) {
		// Internal notes are never customer-visible: do not tell customer
		// browsers that anything changed.
		return nil
	}
	switch env.Type {
	case events.TypeCommentAdded, events.TypeStatusChanged:
		h.broadcast(ctx, env)
	}
	return nil
}

// isInternalNote reports whether a case.comment_added payload is flagged as an
// internal note. A missing or undecodable payload counts as not internal, so
// the broadcast behaves as it did before the flag existed.
func isInternalNote(payload json.RawMessage) bool {
	var p events.CommentAddedPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return false
	}
	return p.IsInternalNote
}

func (h *Handler) broadcast(ctx context.Context, env events.Envelope) {
	var ts struct {
		Timestamp string `json:"timestamp"`
	}
	_ = json.Unmarshal(env.Payload, &ts) // best-effort; empty Timestamp is fine

	body, err := json.Marshal(broadcastPayload{
		CaseID:    env.EntityID,
		Type:      string(env.Type),
		Timestamp: ts.Timestamp,
	})
	if err != nil {
		slog.ErrorContext(ctx, "caseevents: failed to encode broadcast payload", "err", err)
		return
	}
	h.hub.Publish(env.EntityID, string(body))
}
