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

package domain

import (
	"encoding/json"
	"time"
)

// AlertTier is the kind of queue an SRE alert arrived on: a priority queue (high-tier services) or a standard one.
type AlertTier string

// Alert tiers. The relay publishes high before default, so a priority alert is decided first.
const (
	AlertTierHigh    AlertTier = "high"
	AlertTierDefault AlertTier = "default"
)

// NewAlert is one row the alert consumer stores: a canonical alert, or an error row for a message it could not read.
type NewAlert struct {
	ID string
	// Source is the route the gateway named; empty only on an error row whose message named none.
	Source string
	// Alert and Fingerprint are empty on an error row.
	Alert       json.RawMessage
	Fingerprint string
	Tier        AlertTier
	// MessageID and MessageIndex identify the queue message and the alert's place in it, so a redelivery stores nothing.
	MessageID    string
	MessageIndex int
	// RawBody and ContentType are the message as the source sent it, kept on its first row (MessageIndex 0) only.
	RawBody     []byte
	ContentType string
	// ErrorReason and Error say why the message could not be read; both are empty on an alert row.
	ErrorReason string
	Error       string
	ReceivedOn  time.Time
}

// Alert statuses (alert.status, migration 0308). NEW waits for the alert processor; ERROR is a message that could not
// be read, which the processor never takes; the rest are the processor's final outcomes.
const (
	AlertStatusNew       = "NEW"
	AlertStatusError     = "ERROR"
	AlertStatusProcessed = "PROCESSED"
	AlertStatusShadowed  = "SHADOWED"
	AlertStatusFailed    = "FAILED"
)

// Why the consumer stored an error row instead of an alert (alert.error_reason).
const (
	AlertErrorNoSource        = "NoSource"
	AlertErrorUnknownSource   = "UnknownSource"
	AlertErrorTransformFailed = "TransformFailed"
	AlertErrorNoAlerts        = "NoAlerts"
	// AlertErrorStoreRefused: the database refused the alerts themselves, so the message is kept as this error row.
	AlertErrorStoreRefused = "StoreRefused"
)

// ClaimedAlert is one alert the alert processor claimed: a NEW alert row it must decide on.
type ClaimedAlert struct {
	ID          string
	Source      string
	Alert       json.RawMessage
	Fingerprint string
	Tier        AlertTier
	ReceivedOn  time.Time
	Attempts    int
}

// AlertProcessorOutcome is what the alert processor recorded for one alert.
type AlertProcessorOutcome struct {
	Status string
	// IncidentID is the incident the alert was linked to (PROCESSED), empty when none.
	IncidentID string
	// ShadowDecision and ShadowIncidentID are set in hybrid mode only (Status SHADOWED).
	ShadowDecision   string
	ShadowIncidentID string
}

// Shadow decisions: what the alert processor would have done in v2.
const (
	AlertShadowWouldCreate = "WOULD_CREATE"
	AlertShadowWouldAttach = "WOULD_ATTACH"
	AlertShadowWouldIgnore = "WOULD_IGNORE"
)

// AlertToPublish is one waiting alert the alert relay took to publish as alert.created.
type AlertToPublish struct {
	ID          string
	Fingerprint string
	Tier        AlertTier
	ReceivedOn  time.Time
}
