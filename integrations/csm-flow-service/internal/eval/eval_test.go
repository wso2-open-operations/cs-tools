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

package eval

import (
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-flow-service/internal/spec"
)

func testCtx() Context {
	return Context{
		Event: map[string]any{
			"entityType": "case",
			"entityId":   "abc-123",
			"changes": map[string]any{
				"state":    map[string]any{"from": "work_in_progress", "to": "closed"},
				"priority": map[string]any{"from": float64(11), "to": float64(10)},
			},
			"snapshot": map[string]any{
				"number":   "CS0023001",
				"priority": float64(10),
				"state":    "closed",
				"account": map[string]any{
					"u_owner": map[string]any{"email": "Someone@WSO2.com"},
				},
				"watchList": []any{"a@wso2.com", "b@example.com"},
				"emptyStr":  "",
				"emptyList": []any{},
			},
		},
		Vars: map[string]any{
			"watchers": []any{
				map[string]any{"email": "w1@wso2.com"},
				map[string]any{"email": "w2@wso2.com"},
			},
			"none": []any{},
		},
		Loop: map[string]any{
			"w": map[string]any{"email": "w1@wso2.com"},
		},
	}
}

func leaf(field string, op spec.Operator, value any) spec.Condition {
	return spec.Condition{Field: field, Op: op, Value: value}
}

func TestEvaluate_AllOperators(t *testing.T) {
	ctx := testCtx()
	tests := []struct {
		name string
		cond spec.Condition
		want bool
	}{
		// eq / neq, including the coercion that matters most: a spec authored
		// with a JSON number against a snapshot field that arrived as a string.
		{"eq string", leaf("$.event.snapshot.state", spec.OpEq, "closed"), true},
		{"eq mismatch", leaf("$.event.snapshot.state", spec.OpEq, "open"), false},
		{"eq number vs number", leaf("$.event.snapshot.priority", spec.OpEq, float64(10)), true},
		{"eq number vs numeric string", leaf("$.event.snapshot.priority", spec.OpEq, "10"), true},
		{"neq", leaf("$.event.snapshot.state", spec.OpNeq, "open"), true},

		// numeric
		{"lt", leaf("$.event.snapshot.priority", spec.OpLt, 20), true},
		{"lt false", leaf("$.event.snapshot.priority", spec.OpLt, 5), false},
		{"lte equal", leaf("$.event.snapshot.priority", spec.OpLte, 10), true},
		{"gt", leaf("$.event.snapshot.priority", spec.OpGt, 5), true},
		{"gte equal", leaf("$.event.snapshot.priority", spec.OpGte, 10), true},

		// in / nin — array form and ServiceNow's comma-separated string form
		{"in array", leaf("$.event.snapshot.state", spec.OpIn, []any{"closed", "resolved"}), true},
		{"in csv string", leaf("$.event.snapshot.state", spec.OpIn, "closed,resolved"), true},
		{"in csv miss", leaf("$.event.snapshot.state", spec.OpIn, "open,new"), false},
		{"nin", leaf("$.event.snapshot.state", spec.OpNotIn, "open,new"), true},
		{"in collection on the left", leaf("$.event.snapshot.watchList", spec.OpIn, "b@example.com"), true},

		// exists / not_exists — ServiceNow ISEMPTY semantics
		{"exists", leaf("$.event.snapshot.number", spec.OpExists, nil), true},
		{"exists on empty string is false", leaf("$.event.snapshot.emptyStr", spec.OpExists, nil), false},
		{"exists on empty list is false", leaf("$.event.snapshot.emptyList", spec.OpExists, nil), false},
		{"exists on missing key is false", leaf("$.event.snapshot.nope", spec.OpExists, nil), false},
		{"not_exists on missing key", leaf("$.event.snapshot.nope", spec.OpNotExists, nil), true},

		// change family
		{"changed", leaf("state", spec.OpChanged, nil), true},
		{"changed on untouched field", leaf("number", spec.OpChanged, nil), false},
		{"changed_to", leaf("state", spec.OpChangedTo, "closed"), true},
		{"changed_to wrong value", leaf("state", spec.OpChangedTo, "open"), false},
		{"changed_from", leaf("state", spec.OpChangedFrom, "work_in_progress"), true},
		{"changed_to numeric", leaf("priority", spec.OpChangedTo, 10), true},

		// string family — case-insensitive, as ServiceNow is
		{"ends_with", leaf("$.event.snapshot.account.u_owner.email", spec.OpEndsWith, "wso2.com"), true},
		{"ends_with is case-insensitive", leaf("$.event.snapshot.account.u_owner.email", spec.OpEndsWith, "WSO2.COM"), true},
		{"starts_with", leaf("$.event.snapshot.number", spec.OpStartsWith, "CS"), true},
		{"contains", leaf("$.event.snapshot.account.u_owner.email", spec.OpContains, "@wso2"), true},
		{"not_contains", leaf("$.event.snapshot.number", spec.OpNotContains, "INC"), true},

		// matches is anchored
		{"matches full", leaf("$.event.snapshot.number", spec.OpMatches, `CS\d+`), true},
		{"matches is anchored, not a substring", leaf("$.event.snapshot.number", spec.OpMatches, `CS`), false},

		// |len
		{"len gt zero", leaf("$.vars.watchers|len", spec.OpGt, 0), true},
		{"len of empty list", leaf("$.vars.none|len", spec.OpEq, 0), true},

		// loop bindings
		{"loop binding", leaf("$.loop.w.email", spec.OpEndsWith, "wso2.com"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Evaluate(tt.cond, ctx)
			if err != nil {
				t.Fatalf("Evaluate() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("Evaluate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluate_Groups(t *testing.T) {
	ctx := testCtx()
	tests := []struct {
		name string
		cond spec.Condition
		want bool
	}{
		{"all true", spec.Condition{All: []spec.Condition{
			leaf("$.event.snapshot.state", spec.OpEq, "closed"),
			leaf("$.event.snapshot.priority", spec.OpEq, 10),
		}}, true},
		{"all with one false", spec.Condition{All: []spec.Condition{
			leaf("$.event.snapshot.state", spec.OpEq, "closed"),
			leaf("$.event.snapshot.priority", spec.OpEq, 99),
		}}, false},
		{"any with one true", spec.Condition{Any: []spec.Condition{
			leaf("$.event.snapshot.state", spec.OpEq, "open"),
			leaf("$.event.snapshot.state", spec.OpEq, "closed"),
		}}, true},
		{"any all false", spec.Condition{Any: []spec.Condition{
			leaf("$.event.snapshot.state", spec.OpEq, "open"),
			leaf("$.event.snapshot.state", spec.OpEq, "new"),
		}}, false},
		{"not", spec.Condition{Not: &spec.Condition{
			Field: "$.event.snapshot.state", Op: spec.OpEq, Value: "open",
		}}, true},
		// ServiceNow's ^NQ is an OR across whole groups.
		{"NQ as any-of-all", spec.Condition{Any: []spec.Condition{
			{All: []spec.Condition{
				leaf("$.event.snapshot.state", spec.OpEq, "open"),
				leaf("$.event.snapshot.priority", spec.OpEq, 10),
			}},
			{All: []spec.Condition{
				leaf("$.event.snapshot.state", spec.OpEq, "closed"),
				leaf("$.event.snapshot.priority", spec.OpEq, 10),
			}},
		}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Evaluate(tt.cond, ctx)
			if err != nil {
				t.Fatalf("Evaluate() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("Evaluate() = %v, want %v", got, tt.want)
			}
		})
	}
}

// An unresolvable reference must FAIL THE RUN, not quietly evaluate false.
// Silently treating it as "condition not met" is how automation stops firing
// with nobody noticing.
func TestEvaluate_UnresolvedReferenceIsAnError(t *testing.T) {
	ctx := testCtx()
	tests := []struct {
		name  string
		field string
	}{
		{"missing key", "$.event.snapshot.nosuchfield"},
		{"missing nested key", "$.event.snapshot.account.nosuch.email"},
		{"unknown root", "$.nonsense.field"},
		{"traverse through a scalar", "$.event.snapshot.number.deeper"},
		{"unbound loop binding", "$.loop.zzz.email"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Evaluate(leaf(tt.field, spec.OpEq, "x"), ctx)
			if err == nil {
				t.Errorf("Evaluate() = nil error for %q, want a failure rather than a silent false", tt.field)
			}
		})
	}
}

func TestEvaluate_TypeMismatchesAreErrors(t *testing.T) {
	ctx := testCtx()
	tests := []struct {
		name string
		cond spec.Condition
	}{
		{"lt against a non-numeric field", leaf("$.event.snapshot.state", spec.OpLt, 5)},
		{"lt against a non-numeric value", leaf("$.event.snapshot.priority", spec.OpLt, "high")},
		{"ends_with against a non-string value", leaf("$.event.snapshot.number", spec.OpEndsWith, 5)},
		{"in against a non-collection value", leaf("$.event.snapshot.state", spec.OpIn, 5)},
		{"matches with a bad pattern", leaf("$.event.snapshot.number", spec.OpMatches, `CS(\d`)},
		{"matches with a non-string pattern", leaf("$.event.snapshot.number", spec.OpMatches, 5)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Evaluate(tt.cond, ctx); err == nil {
				t.Error("Evaluate() = nil error, want a type-mismatch failure")
			}
		})
	}
}

// A create event has no changes block. "did state change" is then a
// legitimate false, not a failure — otherwise every create-triggered flow
// with a change condition would error.
func TestEvaluate_ChangeOperatorsOnAnEventWithNoChanges(t *testing.T) {
	ctx := Context{Event: map[string]any{"entityType": "case"}}
	for _, op := range []spec.Operator{spec.OpChanged, spec.OpChangedTo, spec.OpChangedFrom} {
		got, err := Evaluate(leaf("state", op, "closed"), ctx)
		if err != nil {
			t.Fatalf("Evaluate(%q) error = %v, want nil", op, err)
		}
		if got {
			t.Errorf("Evaluate(%q) = true, want false when the event carries no changes", op)
		}
	}
}

func TestResolve_LiteralsPassThrough(t *testing.T) {
	ctx := testCtx()
	got, err := ctx.Resolve("just-a-string")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got != "just-a-string" {
		t.Errorf("Resolve() = %v, want the literal unchanged", got)
	}
}

func TestResolve_LenOnUnsupportedType(t *testing.T) {
	ctx := testCtx()
	if _, err := ctx.Resolve("$.event.snapshot.priority|len"); err == nil {
		t.Error("Resolve() = nil error, want a failure for |len on a number")
	}
}
