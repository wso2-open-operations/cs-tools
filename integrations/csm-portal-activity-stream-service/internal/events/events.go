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

// Package events defines the envelope of every record on the case-events
// Kafka topic. entity-service is the sole producer; this service and
// csm-notification-service consume it. The envelope is kept in sync BY HAND
// with entity-service's and csm-notification-service's own
// internal/events.Envelope, since the modules don't import each other.
//
// Only the envelope (type + entityId) is modelled here: this service never
// reads the payload — the producer's per-type payload structs are the source
// of truth for those, and a local copy would only drift.
package events

import "encoding/json"

// Type identifies which kind of domain event Envelope.Payload holds. Values
// mirror csm-notification-service's internal/events.Type constants exactly.
type Type string

const (
	TypeCaseCreated     Type = "case.created"
	TypeCommentAdded    Type = "case.comment_added"
	TypeStatusChanged   Type = "case.status_changed"
	TypeCaseAssigned    Type = "case.assigned"
	TypeIncidentCreated Type = "incident.created"
)

// Envelope is the wire shape of every record on the case-events topic.
// EntityID is whatever the event is about (a case ID for the case.* types,
// an incident ID for incident.created) and is also the Kafka partition key
// the producer uses — every event about the same case/incident lands on the
// same partition and is processed in publish order.
type Envelope struct {
	Type     Type            `json:"type"`
	EntityID string          `json:"entityId"`
	Payload  json.RawMessage `json:"payload"`
}
