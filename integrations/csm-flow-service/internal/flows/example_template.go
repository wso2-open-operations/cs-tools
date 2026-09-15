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

	"github.com/wso2-open-operations/cs-tools/integrations/csm-flow-service/internal/events"
)

// exampleFlow is the copy-me template for a ported flow. It is NOT registered
// (see register.go) and Match always returns false, so it never runs — it
// exists purely to show the shape a real port takes. Delete it once a couple
// of real flows exist to copy from instead.
//
// A real port is written from that flow's row in docs/flow-porting-specs.md:
//   - Key():   a stable snake_case id.
//   - Match(): the trigger kind + entity type + the encoded condition,
//     translated to Go (or to a spec.Condition evaluated with
//     internal/eval). Pure — no I/O.
//   - Run():   the flow's actions, in order, using deps. Idempotent, because
//     a retry re-runs it (see Registry.Handle).
type exampleFlow struct{}

func (exampleFlow) Key() string { return "example_template" }

// Match gates on the event. This template reacts to nothing.
func (exampleFlow) Match(evt Event) bool {
	// Typical shape, e.g. for a case-comment reaction:
	//   return evt.Envelope.Type == events.TypeCommentAdded
	// or, for the generic change event with a condition:
	//   return evt.Envelope.Type == events.TypeEntityChanged &&
	//          <condition over the decoded EntityChangedPayload>
	return false
}

// Run performs the actions. This template does nothing.
func (exampleFlow) Run(ctx context.Context, evt Event, deps Deps) error {
	// Decode the payload for the event type this flow matched on, e.g.:
	var payload events.CommentAddedPayload
	if err := json.Unmarshal(evt.Envelope.Payload, &payload); err != nil {
		return err
	}
	_ = payload
	_ = ctx
	_ = deps
	// ... then: resolve recipients, publish a notification request, or patch a
	// native entity via deps.Entity. Return a non-nil error only for a
	// genuinely retryable failure.
	return nil
}

// Compile-time check that the template satisfies the interface — the same line
// every real flow file should carry.
var _ Flow = exampleFlow{}
