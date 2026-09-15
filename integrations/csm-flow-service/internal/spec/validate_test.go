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

import "testing"

func validFlow() Flow {
	return Flow{
		Key:     "case-closed-notify",
		Trigger: Trigger{Kind: TriggerRecordUpdate, EntityType: "case"},
		Nodes: []Node{
			{ID: "watchers", Type: NodeLookup, As: "watchers"},
			{ID: "mail", Type: NodeNotify},
		},
	}
}

func TestValidate_BaseFlowIsValid(t *testing.T) {
	f := validFlow()
	if err := f.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
}

// Node ids are half of the (event_id, node_id) idempotency key. A duplicate
// would make two steps share one dedup slot and silently suppress one — the
// exact class of bug this engine exists to prevent, so it must fail at publish.
func TestValidate_RejectsDuplicateNodeIDsAcrossTheWholeTree(t *testing.T) {
	f := validFlow()
	f.Nodes = []Node{
		{ID: "dup", Type: NodeLog},
		{
			ID: "branch", Type: NodeIf,
			Condition: &Condition{Field: "$.event.entityId", Op: OpExists},
			Children:  []Node{{ID: "dup", Type: NodeLog}},
		},
	}
	if err := f.Validate(); err == nil {
		t.Error("Validate() = nil, want an error for a duplicate node id nested in a branch")
	}
}

// ServiceNow saves malformed conditions and only fails at runtime —
// "=3b8b43… is not a valid conditional expression" failed 17 times in
// production. Publish-time rejection is the whole point.
func TestValidate_RejectsUnknownOperator(t *testing.T) {
	f := validFlow()
	f.Nodes = []Node{{
		ID: "branch", Type: NodeIf,
		Condition: &Condition{Field: "state", Op: Operator("definitely-not-real")},
		Children:  []Node{{ID: "x", Type: NodeLog}},
	}}
	if err := f.Validate(); err == nil {
		t.Error("Validate() = nil, want an error for an unknown operator")
	}
}

func TestValidate_UnaryOperatorsTakeNoValue(t *testing.T) {
	for _, op := range []Operator{OpExists, OpNotExists, OpChanged} {
		t.Run(string(op), func(t *testing.T) {
			c := Condition{Field: "state", Op: op, Value: "something"}
			if err := c.Validate(); err == nil {
				t.Errorf("Validate() = nil, want an error: %q takes no value", op)
			}
			c.Value = nil
			if err := c.Validate(); err != nil {
				t.Errorf("Validate() = %v, want nil for a bare %q", err, op)
			}
		})
	}
}

func TestValidate_BinaryOperatorsRequireAValue(t *testing.T) {
	c := Condition{Field: "state", Op: OpEq}
	if err := c.Validate(); err == nil {
		t.Error("Validate() = nil, want an error for eq with no value")
	}
}

// The three operators the original design lacked, all found in live triggers.
func TestValidate_AcceptsTheOperatorsRealTriggersUse(t *testing.T) {
	for _, op := range []Operator{OpEndsWith, OpContains, OpNotContains, OpStartsWith} {
		c := Condition{Field: "account.u_owner.email", Op: op, Value: "wso2.com"}
		if err := c.Validate(); err != nil {
			t.Errorf("Validate() = %v, want nil for %q", err, op)
		}
	}
}

func TestValidate_GroupCannotAlsoBeAComparison(t *testing.T) {
	c := Condition{
		All:   []Condition{{Field: "a", Op: OpExists}},
		Field: "b", Op: OpExists,
	}
	if err := c.Validate(); err == nil {
		t.Error("Validate() = nil, want an error when a group also carries a comparison")
	}
}

func TestValidate_RejectsMoreThanOneGroupKind(t *testing.T) {
	c := Condition{
		All: []Condition{{Field: "a", Op: OpExists}},
		Any: []Condition{{Field: "b", Op: OpExists}},
	}
	if err := c.Validate(); err == nil {
		t.Error("Validate() = nil, want an error when both all and any are set")
	}
}

// $.loop.x outside an enclosing for_each binding x is a publish error, not a
// nil at 2am inside a nested loop.
func TestValidate_RejectsLoopRefWithNoEnclosingLoop(t *testing.T) {
	f := validFlow()
	f.Nodes = []Node{{
		ID: "branch", Type: NodeIf,
		Condition: &Condition{Field: "$.loop.w.email", Op: OpExists},
		Children:  []Node{{ID: "x", Type: NodeLog}},
	}}
	if err := f.Validate(); err == nil {
		t.Error("Validate() = nil, want an error for $.loop.w with no for_each binding w")
	}
}

func TestValidate_AcceptsLoopRefInsideItsLoop(t *testing.T) {
	f := validFlow()
	f.Nodes = []Node{{
		ID: "each", Type: NodeForEach, Over: "$.vars.watchers", As: "w",
		Children: []Node{{
			ID: "branch", Type: NodeIf,
			Condition: &Condition{Field: "$.loop.w.email", Op: OpExists},
			Children:  []Node{{ID: "mail", Type: NodeNotify}},
		}},
	}}
	if err := f.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil for $.loop.w inside for_each binding w", err)
	}
}

func TestValidate_TriggerKinds(t *testing.T) {
	tests := []struct {
		name    string
		trigger Trigger
		wantErr bool
	}{
		{"record_update with entity", Trigger{Kind: TriggerRecordUpdate, EntityType: "case"}, false},
		{"record_update without entity", Trigger{Kind: TriggerRecordUpdate}, true},
		{"daily with cron", Trigger{Kind: TriggerDaily, Cron: "0 2 * * *"}, false},
		{"daily without cron", Trigger{Kind: TriggerDaily}, true},
		{"email", Trigger{Kind: TriggerEmail}, false},
		{"sla_task", Trigger{Kind: TriggerSLATask}, false},
		{"run_once", Trigger{Kind: TriggerRunOnce}, false},
		{"unknown kind", Trigger{Kind: TriggerKind("nope")}, true},
		{"unknown runTrigger", Trigger{Kind: TriggerEmail, RunTrigger: RunTrigger("nope")}, true},
		{"unique_changes", Trigger{Kind: TriggerRecordUpdate, EntityType: "case", RunTrigger: RunTriggerUniqueChanges}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := validFlow()
			f.Trigger = tt.trigger
			err := f.Validate()
			if tt.wantErr && err == nil {
				t.Error("Validate() = nil, want an error")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Validate() = %v, want nil", err)
			}
		})
	}
}

func TestValidate_ContainerNodeRequirements(t *testing.T) {
	tests := []struct {
		name string
		node Node
	}{
		{"if without condition", Node{ID: "a", Type: NodeIf, Children: []Node{{ID: "b", Type: NodeLog}}}},
		{"if with no branches", Node{ID: "a", Type: NodeIf, Condition: &Condition{Field: "x", Op: OpExists}}},
		{"for_each without over", Node{ID: "a", Type: NodeForEach, As: "w", Children: []Node{{ID: "b", Type: NodeLog}}}},
		{"for_each without as", Node{ID: "a", Type: NodeForEach, Over: "$.vars.x", Children: []Node{{ID: "b", Type: NodeLog}}}},
		{"for_each with empty body", Node{ID: "a", Type: NodeForEach, Over: "$.vars.x", As: "w"}},
		{"parallel with one branch", Node{ID: "a", Type: NodeParallel, Children: []Node{{ID: "b", Type: NodeLog}}}},
		{"lookup without as", Node{ID: "a", Type: NodeLookup}},
		{"unknown node type", Node{ID: "a", Type: NodeType("teleport")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := validFlow()
			f.Nodes = []Node{tt.node}
			if err := f.Validate(); err == nil {
				t.Error("Validate() = nil, want an error")
			}
		})
	}
}

func TestDecode_RejectsUnknownFields(t *testing.T) {
	_, err := DecodeBytes([]byte(`{"key":"k","trigger":{"kind":"email"},"nodes":[],"surprise":1}`))
	if err == nil {
		t.Error("DecodeBytes() = nil, want an error for an unknown field")
	}
}

func TestDecode_RejectsTrailingData(t *testing.T) {
	_, err := DecodeBytes([]byte(`{"key":"k","trigger":{"kind":"email"},"nodes":[]}{"another":1}`))
	if err == nil {
		t.Error("DecodeBytes() = nil, want an error for trailing data")
	}
}

func TestDecode_RoundTripsAFlowWithControlFlow(t *testing.T) {
	raw := []byte(`{
	  "key": "case-closed",
	  "trigger": {"kind":"record_update","entityType":"case","runTrigger":"unique_changes",
	              "condition":{"field":"$.event.changes.state.to","op":"eq","value":"closed"}},
	  "nodes": [
	    {"id":"watchers","type":"lookup","as":"watchers"},
	    {"id":"branch","type":"if",
	     "condition":{"field":"$.vars.watchers","op":"exists"},
	     "children":[
	       {"id":"each","type":"for_each","over":"$.vars.watchers","as":"w",
	        "children":[{"id":"mail","type":"notify"}]}
	     ],
	     "else":[{"id":"none","type":"log"}]}
	  ]
	}`)
	f, err := DecodeBytes(raw)
	if err != nil {
		t.Fatalf("DecodeBytes() = %v", err)
	}
	if err := f.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
	if f.Trigger.RunTrigger != RunTriggerUniqueChanges {
		t.Errorf("RunTrigger = %q, want unique_changes", f.Trigger.RunTrigger)
	}
	if len(f.Nodes) != 2 || f.Nodes[1].Children[0].As != "w" {
		t.Error("control-flow structure did not survive the round trip")
	}
}
