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
// of every event independent of csm-notification-service's consumers.
// Logs type/entityId for every event, and — for the two event types the
// case-activity SSE stream cares about — fans a minimal broadcast payload
// out to internal/stream.BroadcastHub, which is what actually pushes the
// `case_updated` event to any browser subscribed to that case on this
// replica (see internal/handler.StreamCaseActivities).
package caseevents

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-portal-activity-stream-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-portal-activity-stream-service/internal/events"
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

// NewHandler constructs a Handler that fans case.comment_added and
// case.status_changed events out to hub. Pass nil to disable broadcasting
// (Handle will still log every event as before).
func NewHandler(hub broadcastHub) *Handler {
	return &Handler{hub: hub}
}

// broadcastPayload is the minimal shape written to the SSE stream — never
// comment text or field values, see StreamCaseActivities' doc comment for
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
	switch env.Type {
	case events.TypeCommentAdded, events.TypeStatusChanged:
		h.broadcast(ctx, env, record.Time)
	}
	return nil
}

// broadcast publishes the minimal payload for env. timestamp is the
// record's own time on the topic (the producer's payloads carry no
// timestamp field of their own), written as RFC 3339 UTC; omitted when the
// record has none.
func (h *Handler) broadcast(ctx context.Context, env events.Envelope, at time.Time) {
	var ts string
	if !at.IsZero() {
		ts = at.UTC().Format(time.RFC3339Nano)
	}
	body, err := json.Marshal(broadcastPayload{
		CaseID:    env.EntityID,
		Type:      string(env.Type),
		Timestamp: ts,
	})
	if err != nil {
		slog.ErrorContext(ctx, "caseevents: failed to encode broadcast payload", "err", err)
		return
	}
	h.hub.Publish(env.EntityID, string(body))
}
