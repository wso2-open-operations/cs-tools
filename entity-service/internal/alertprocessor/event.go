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

// Package alertprocessor turns stored alerts into incidents: a relay publishes each stored alert as alert.created, and the processor folds those events into incidents through a chain of strategies.
package alertprocessor

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/model"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/events"
)

// EventType is the type of the event published for every stored alert.
const EventType events.Type = "alert.created"

// Event is one stored alert, as the processor needs it.
type Event struct {
	AlertID    string           `json:"alertId"`
	Tier       domain.AlertTier `json:"tier"`
	ReceivedOn time.Time        `json:"receivedOn"`
	Alert      model.Alert      `json:"alert"`
}

// Fingerprint is the key an alert folds by; it is computed, never stored on the alert.
func (e Event) Fingerprint() string {
	return model.Fingerprint(e.Alert)
}

// Encode returns the record key and value: keyed by fingerprint, so one fingerprint's alerts stay on one partition, in order.
func Encode(e Event) (key, value []byte, err error) {
	payload, err := json.Marshal(e)
	if err != nil {
		return nil, nil, fmt.Errorf("encode %s: %w", EventType, err)
	}
	value, err = json.Marshal(events.Envelope{Type: EventType, EntityID: e.AlertID, Payload: payload})
	if err != nil {
		return nil, nil, fmt.Errorf("encode %s envelope: %w", EventType, err)
	}
	return []byte(e.Fingerprint()), value, nil
}

// Decode reads an alert.created record; ok is false for any other event type, which the processor skips.
func Decode(value []byte) (e Event, ok bool, err error) {
	var env events.Envelope
	if err := json.Unmarshal(value, &env); err != nil {
		return Event{}, false, fmt.Errorf("decode envelope: %w", err)
	}
	if env.Type != EventType {
		return Event{}, false, nil
	}
	if err := json.Unmarshal(env.Payload, &e); err != nil {
		return Event{}, false, fmt.Errorf("decode %s: %w", EventType, err)
	}
	if e.AlertID == "" {
		return Event{}, false, fmt.Errorf("decode %s: no alert id", EventType)
	}
	return e, true, nil
}
