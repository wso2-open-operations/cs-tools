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

package alertprocessor

import (
	"context"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertprocessor/event"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertprocessor/strategy"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// The processor depends only on these ports; the repository, sink and mute rule adapters implement them.

// Store keeps each fingerprint's group and each alert's incident.
type Store interface {
	// InGroup runs fn in one transaction that holds the fingerprint's lock, so one fingerprint is decided by one worker at a time.
	InGroup(ctx context.Context, fingerprint string, fn func(GroupTx) error) error
}

// GroupTx is the transaction InGroup hands to its function.
type GroupTx interface {
	// Group is the fingerprint's group, or nil when it has none.
	Group(ctx context.Context) (*strategy.Group, error)
	SaveGroup(ctx context.Context, g strategy.Group) error
	// AlertLinked reports whether the alert already belongs to an incident, which makes a redelivered event a no-op.
	AlertLinked(ctx context.Context, alertID string) (bool, error)
	LinkAlert(ctx context.Context, alertID, incidentID string) error
}

// Sink is where incidents live.
type Sink interface {
	// Open creates the alert's incident and returns its id; it returns the same incident when called again for the same alert.
	Open(ctx context.Context, e event.Event) (string, error)
	// IsOpen reports whether the incident is still open.
	IsOpen(ctx context.Context, incidentID string) (bool, error)
	// Fold records an alert added to the incident, as a Duplicate or OK work note.
	Fold(ctx context.Context, incidentID string, outcome strategy.Outcome, e event.Event) error
}

// MuteRules returns the mute rules active at now.
type MuteRules func(ctx context.Context, now time.Time) ([]domain.AlertMuteRule, error)
