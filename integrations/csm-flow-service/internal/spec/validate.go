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

package spec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// maxSpecBytes caps a submitted spec, matching entity-service's own 1 MiB
// decode cap.
const maxSpecBytes = 1 << 20

// Decode parses a flow spec with strict decoding: unknown fields rejected,
// trailing data rejected, size capped. Mirrors
// entity-service/internal/handler/decode.go.
func Decode(r io.Reader) (*Flow, error) {
	dec := json.NewDecoder(io.LimitReader(r, maxSpecBytes+1))
	dec.DisallowUnknownFields()

	var f Flow
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("spec: decode: %w", err)
	}
	if dec.More() {
		return nil, fmt.Errorf("spec: unexpected trailing data after the flow object")
	}
	return &f, nil
}

// DecodeBytes is Decode over a byte slice.
func DecodeBytes(b []byte) (*Flow, error) { return Decode(bytes.NewReader(b)) }

// Validate checks a flow is publishable.
//
// EVERYTHING HERE FAILS AT PUBLISH, DELIBERATELY. ServiceNow will save a
// malformed condition and only surface it at runtime — `=3b8b43…` failed 17
// times in production because nothing rejected it at authoring time. A flow
// that cannot run must not reach the engine.
func (f *Flow) Validate() error {
	if strings.TrimSpace(f.Key) == "" {
		return fmt.Errorf("spec: key is required")
	}
	if err := f.Trigger.validate(); err != nil {
		return err
	}
	if len(f.Nodes) == 0 {
		return fmt.Errorf("spec: a flow needs at least one node")
	}

	// Node IDs must be unique across the WHOLE tree, not just per level:
	// they are half of the (event_id, node_id) idempotency key, so a
	// duplicate would make two different steps share a dedup slot and
	// silently suppress one of them.
	seen := map[string]bool{}
	// Loop bindings in scope, so `$.loop.x` outside a loop binding x is a
	// compile error rather than a runtime nil.
	return validateNodes(f.Nodes, seen, map[string]bool{})
}

func (t *Trigger) validate() error {
	switch t.Kind {
	case TriggerRecordCreate, TriggerRecordUpdate, TriggerRecordCreateOrUpdate:
		if strings.TrimSpace(t.EntityType) == "" {
			return fmt.Errorf("spec: trigger %q requires entityType", t.Kind)
		}
	case TriggerDaily, TriggerWeekly:
		if strings.TrimSpace(t.Cron) == "" {
			return fmt.Errorf("spec: trigger %q requires cron", t.Kind)
		}
	case TriggerEmail, TriggerRunOnce, TriggerSLATask:
		// no extra requirements
	default:
		return fmt.Errorf("spec: unknown trigger kind %q", t.Kind)
	}

	switch t.RunTrigger {
	case "", RunTriggerEvery, RunTriggerUniqueChanges, RunTriggerAlways:
	default:
		return fmt.Errorf("spec: unknown runTrigger %q", t.RunTrigger)
	}

	if t.Condition != nil {
		return t.Condition.Validate()
	}
	return nil
}

func validateNodes(nodes []Node, seen map[string]bool, loops map[string]bool) error {
	for _, n := range nodes {
		if strings.TrimSpace(n.ID) == "" {
			return fmt.Errorf("spec: every node needs an id (type %q)", n.Type)
		}
		if seen[n.ID] {
			return fmt.Errorf("spec: duplicate node id %q — ids are half of the idempotency key and must be unique across the whole tree", n.ID)
		}
		seen[n.ID] = true

		if err := n.validateSelf(loops); err != nil {
			return err
		}

		// A for_each introduces its binding for its children only.
		childLoops := loops
		if n.Type == NodeForEach {
			childLoops = map[string]bool{}
			for k := range loops {
				childLoops[k] = true
			}
			childLoops[n.As] = true
		}

		if err := validateNodes(n.Children, seen, childLoops); err != nil {
			return err
		}
		if err := validateNodes(n.Else, seen, childLoops); err != nil {
			return err
		}
	}
	return nil
}

func (n Node) validateSelf(loops map[string]bool) error {
	switch n.Type {
	case NodeIf:
		if n.Condition == nil {
			return fmt.Errorf("spec: node %q (if) requires a condition", n.ID)
		}
		if len(n.Children) == 0 && len(n.Else) == 0 {
			return fmt.Errorf("spec: node %q (if) has neither a body nor an else branch", n.ID)
		}
	case NodeForEach:
		if strings.TrimSpace(n.Over) == "" {
			return fmt.Errorf("spec: node %q (for_each) requires `over`", n.ID)
		}
		if strings.TrimSpace(n.As) == "" {
			return fmt.Errorf("spec: node %q (for_each) requires `as`", n.ID)
		}
		if len(n.Children) == 0 {
			return fmt.Errorf("spec: node %q (for_each) has an empty body", n.ID)
		}
	case NodeUntil:
		if n.Condition == nil {
			return fmt.Errorf("spec: node %q (until) requires a condition", n.ID)
		}
	case NodeParallel:
		if len(n.Children) < 2 {
			return fmt.Errorf("spec: node %q (parallel) needs at least two branches", n.ID)
		}
	case NodeLookup:
		if n.As == "" {
			return fmt.Errorf("spec: node %q (lookup) requires `as` to name its result", n.ID)
		}
	case NodeWait, NodeSetVars, NodePatchEntity, NodeCreateEntity, NodeDeleteEntity,
		NodeNotify, NodeLog, NodeCallHTTP, NodeStartTimer, NodeCancelTimer,
		NodeRequestApproval, NodeRunFlow:
		// leaf types; params are checked by their action implementation
	default:
		return fmt.Errorf("spec: unknown node type %q on node %q", n.Type, n.ID)
	}

	if n.Condition != nil {
		if err := n.Condition.Validate(); err != nil {
			return fmt.Errorf("spec: node %q: %w", n.ID, err)
		}
		if err := checkLoopRefs(*n.Condition, loops, n.ID); err != nil {
			return err
		}
	}
	return nil
}

// checkLoopRefs rejects `$.loop.x` where x is not a binding in scope. Catching
// it at publish beats a nil at 2am inside a nested for_each.
func checkLoopRefs(c Condition, loops map[string]bool, nodeID string) error {
	if c.IsLeaf() {
		const prefix = "$.loop."
		if strings.HasPrefix(c.Field, prefix) {
			rest := strings.TrimPrefix(c.Field, prefix)
			binding := rest
			if i := strings.Index(rest, "."); i >= 0 {
				binding = rest[:i]
			}
			if !loops[binding] {
				return fmt.Errorf("spec: node %q references $.loop.%s but no enclosing for_each binds %q", nodeID, binding, binding)
			}
		}
		return nil
	}
	for _, sub := range append(append([]Condition{}, c.All...), c.Any...) {
		if err := checkLoopRefs(sub, loops, nodeID); err != nil {
			return err
		}
	}
	if c.Not != nil {
		return checkLoopRefs(*c.Not, loops, nodeID)
	}
	return nil
}

// Validate checks a condition tree.
func (c Condition) Validate() error {
	groups := 0
	if len(c.All) > 0 {
		groups++
	}
	if len(c.Any) > 0 {
		groups++
	}
	if c.Not != nil {
		groups++
	}

	if groups > 1 {
		return fmt.Errorf("condition: set exactly one of all/any/not, not %d", groups)
	}
	if groups == 1 {
		if c.Field != "" || c.Op != "" {
			return fmt.Errorf("condition: a group cannot also be a comparison")
		}
		for _, sub := range append(append([]Condition{}, c.All...), c.Any...) {
			if err := sub.Validate(); err != nil {
				return err
			}
		}
		if c.Not != nil {
			return c.Not.Validate()
		}
		return nil
	}

	// Leaf.
	if strings.TrimSpace(c.Field) == "" {
		return fmt.Errorf("condition: comparison requires a field")
	}
	if !validOperators[c.Op] {
		return fmt.Errorf("condition: unknown operator %q on field %q", c.Op, c.Field)
	}
	if unaryOperators[c.Op] {
		if c.Value != nil {
			return fmt.Errorf("condition: operator %q takes no value", c.Op)
		}
		return nil
	}
	if c.Value == nil {
		return fmt.Errorf("condition: operator %q on field %q requires a value", c.Op, c.Field)
	}
	return nil
}
