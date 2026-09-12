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

// Package timecardengine is the consumer group for time-card-related
// reactions to case-events records — named for the domain, not the one
// event it happens to handle today, since more time-card-related work is
// expected here later. Currently reacts only to
// events.TypeCaseBillableStatusChanged — see that type's own doc comment in
// internal/events/events.go for the full context (entity-service's Postgres
// data source only, published when a case's severity crosses into or out
// of LOW).
//
// Not yet a real implementation: entity-service has no time_cards table/
// repo/service on its Postgres data source yet — the prerequisite for
// actually bulk-flipping every time card's billable flag for the case — so
// Engine.Handle below only logs the event it would eventually act on.
//
// This package (and its own dedicated consumer group — see
// cmd/server/main.go's TIME_CARD_CONSUMER_GROUP/_COUNT) exists ahead
// of that reaction being buildable, deliberately: the consumer group itself
// — topic wiring, retry/DLQ behavior, schema validation via internal/events
// — has no dependency on time_cards at all, only the eventual bulk-update
// logic inside Handle does. Given its own consumer group rather than folded
// into internal/dispatch.Dispatcher's, for the same reason
// internal/slaengine already has one: eventbus.Consumer.Run processes one
// record at a time, fully sequentially (fetch, handle, commit, repeat) — a
// future bulk update over "several time cards," each its own HTTP round
// trip to entity-service, must not share a consumer group with
// latency-sensitive email/Chat dispatch.
package timecardengine

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/events"
)

// Engine is this feature's consumer entry point.
type Engine struct{}

// NewEngine constructs an Engine.
func NewEngine() *Engine {
	return &Engine{}
}

// Handle implements eventbus.Handle for this engine's own consumer group.
// It shares a topic with events unrelated to it (case.*, incident.created,
// sla.*) — anything other than events.TypeCaseBillableStatusChanged is
// silently ignored, not an error, the same posture
// dispatch.Dispatcher.Handle/slaengine.Engine.Handle already take for types
// that aren't theirs.
func (e *Engine) Handle(ctx context.Context, record eventbus.Record) error {
	var env events.Envelope
	if err := json.Unmarshal(record.Value, &env); err != nil {
		return fmt.Errorf("timecardengine: decode envelope: %w", err)
	}
	if env.Type != events.TypeCaseBillableStatusChanged {
		return nil
	}
	if err := events.Validate(env.EntityID, env.Type, env.Payload); err != nil {
		return fmt.Errorf("timecardengine: invalid payload: %w", err)
	}

	var p events.CaseBillableStatusChangedPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return fmt.Errorf("timecardengine: decode payload: %w", err)
	}

	// TODO: bulk-flip every time card's IsBillable for p.CaseID to
	// p.IsBillable, once entity-service has a Postgres time_cards table/
	// repo/service to search/update against (see
	// events.TypeCaseBillableStatusChanged's own doc comment) — likely a
	// SearchTimeCards-then-loop-UpdateTimeCard shape, mirroring
	// slaengine.Engine.registerClocks' own "one HTTP call per affected
	// record" pattern. For now, only logs: entity-service's own Publish
	// call for this event is itself still commented out, so this never
	// actually receives one in production yet either — this is deliberately
	// ahead-of-need plumbing, not a live reaction.
	slog.InfoContext(ctx, "timecardengine: case billable status changed, no consumer action implemented yet", "caseId", p.CaseID, "isBillable", p.IsBillable)
	return nil
}
