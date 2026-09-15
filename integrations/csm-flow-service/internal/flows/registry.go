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

package flows

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-flow-service/internal/eventbus"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-flow-service/internal/events"
)

// Registry routes each consumed record to the flows that match it. It is the
// eventbus consumer's Handle: cmd/consumer wires Registry.Handle as the
// consumer's handler.
type Registry struct {
	deps  Deps
	flows []Flow
}

// NewRegistry builds a Registry over the given flows and shared deps.
func NewRegistry(deps Deps, flows ...Flow) *Registry {
	return &Registry{deps: deps, flows: flows}
}

// Flows returns the registered flows (for logging/health/tests).
func (r *Registry) Flows() []Flow { return r.flows }

// Handle decodes a record's envelope and runs every flow whose Match returns
// true, in registration order.
//
// Error semantics:
//   - A record whose bytes are not a valid envelope is a poison message: it is
//     logged and dropped (nil returned), not retried forever.
//   - If a flow's Run fails, Handle returns the first such error, which makes
//     the eventbus retry the WHOLE record. That re-runs every matching flow,
//     including ones that already succeeded — so a ported flow MUST be
//     idempotent (or notify-only) until the per-(event, flow) dispatch log
//     lands (docs/architecture.md §9, Phase 2). This is the deliberate,
//     documented limitation of the pre-dispatch-log scaffold, not an oversight.
func (r *Registry) Handle(ctx context.Context, rec eventbus.Record) error {
	var env events.Envelope
	if err := json.Unmarshal(rec.Value, &env); err != nil {
		slog.ErrorContext(ctx, "flows: undecodable record dropped (poison message)",
			"topic", rec.Topic, "partition", rec.Partition, "offset", rec.Offset, "err", err)
		return nil
	}

	evt := Event{Envelope: env, Record: rec}
	var firstErr error
	matched := 0
	for _, f := range r.flows {
		if !f.Match(evt) {
			continue
		}
		matched++
		if err := f.Run(ctx, evt, r.deps); err != nil {
			slog.ErrorContext(ctx, "flows: flow run failed",
				"flow", f.Key(), "eventType", string(env.Type), "entityId", env.EntityID, "err", err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	if matched == 0 {
		slog.DebugContext(ctx, "flows: no flow matched", "eventType", string(env.Type), "entityId", env.EntityID)
	}
	return firstErr
}
