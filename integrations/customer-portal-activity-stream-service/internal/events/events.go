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

// Package events defines the wire shape of every record on the case-events
// Kafka topic. entity-service's EventPublisherService is the actual
// producer — shared by both csm-portal-backend and customer-portal's
// backend-v2, since both call the same entity-service HTTP endpoints. This
// package is the consumer-side copy used by internal/caseevents. It's kept
// in sync BY HAND with entity-service's own internal/events.Envelope,
// csm-notification-service's own copy, and the sibling
// csm-portal-activity-stream-service's copy — four independent Go modules
// with no shared import path, all of which must agree on this shape.
package events

import "encoding/json"

// Type identifies which kind of domain event Envelope.Payload holds. Values
// mirror csm-notification-service's internal/events.Type constants exactly.
type Type string

const (
	TypeCaseCreated     Type = "case.created"
	TypeCommentAdded    Type = "case.comment_added"
	TypeStatusChanged   Type = "case.status_changed"
	TypeIncidentCreated Type = "incident.created"
)

// Envelope is the wire shape of every record on the case-events topic.
// EntityID is whatever the event is about (a case ID for the case.* types,
// an incident ID for incident.created) and is also the Kafka partition key
// (see eventbus.Producer.Publish) — every event about the same case/incident
// lands on the same partition and is processed in publish order.
type Envelope struct {
	Type     Type            `json:"type"`
	EntityID string          `json:"entityId"`
	Payload  json.RawMessage `json:"payload"`
}

// CommentAddedPayload is the Payload shape for TypeCommentAdded.
// Deliberately minimal: no comment body, no author — Envelope already
// carries EntityID (the case), so this is only what search/replay by
// timestamp needs.
type CommentAddedPayload struct {
	Timestamp string `json:"timestamp"`
	// IsInternalNote is true when the comment is an internal (work) note that
	// customers must never see. The producer omits the field for ordinary
	// comments, so an absent value means customer-visible. This consumer uses
	// it to skip the customer-facing broadcast entirely.
	IsInternalNote bool `json:"isInternalNote"`
}

// StatusChangedPayload is the Payload shape for TypeStatusChanged.
type StatusChangedPayload struct {
	Timestamp string `json:"timestamp"`
	NewStatus string `json:"newStatus"`
}
