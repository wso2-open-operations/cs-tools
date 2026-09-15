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

package snquery

import (
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-flow-service/internal/spec"
)

// Every query below is REAL, taken verbatim from a live trigger on the WSO2
// instance (docs/servicenow-discovery/RESULTS.md §26). Synthetic test data
// would not have caught the bare-CHANGESFROM case or the "50 - Closed Won"
// value containing spaces and a hyphen.
func TestParse_RealConditionsFromTheInstance(t *testing.T) {
	tests := []struct {
		flow  string
		query string
		check func(*testing.T, *spec.Condition)
	}{
		{
			flow:  "Watch List",
			query: "commentsVALCHANGES",
			check: func(t *testing.T, c *spec.Condition) {
				if c.Field != "comments" || c.Op != spec.OpChanged {
					t.Errorf("got %+v, want comments/changed", c)
				}
				if c.Value != nil {
					t.Errorf("Value = %v, want nil for a unary operator", c.Value)
				}
			},
		},
		{
			flow:  "Account Manager Notifications[Project Status]",
			query: "accountISNOTEMPTY^account.u_ownerISNOTEMPTY^account.u_owner.emailENDSWITHwso2.com",
			check: func(t *testing.T, c *spec.Condition) {
				if len(c.All) != 3 {
					t.Fatalf("All = %d terms, want 3", len(c.All))
				}
				last := c.All[2]
				// Dot-walking survives as a field path; the compiler rewrites
				// it into a lookup later.
				if last.Field != "account.u_owner.email" || last.Op != spec.OpEndsWith || last.Value != "wso2.com" {
					t.Errorf("last term = %+v, want the dot-walked ENDSWITH", last)
				}
			},
		},
		{
			flow:  "CR Approval notifications",
			query: "stateCHANGESTO-4^ORstateCHANGESTO-3^ORstateCHANGESTO5^ORstateCHANGESTO0^ORstateCHANGESTO1",
			check: func(t *testing.T, c *spec.Condition) {
				// A chain of ^OR on one field collapses into a single Any.
				if len(c.Any) != 5 {
					t.Fatalf("Any = %d terms, want 5", len(c.Any))
				}
				if c.Any[0].Value != "-4" {
					t.Errorf("first value = %v, want the negative state -4", c.Any[0].Value)
				}
			},
		},
		{
			flow:  "Twilio alearts when assigning P1 cases",
			query: "priority=10^assigned_toCHANGESFROM",
			check: func(t *testing.T, c *spec.Condition) {
				if len(c.All) != 2 {
					t.Fatalf("All = %d, want 2", len(c.All))
				}
				// A bare CHANGESFROM carries no value; "changed" is the honest
				// reading rather than inventing an empty-string comparison.
				if c.All[1].Op != spec.OpChanged {
					t.Errorf("bare CHANGESFROM became %q, want changed", c.All[1].Op)
				}
			},
		},
		{
			flow:  "Populate Customer Engagements",
			query: "u_stageCHANGESTO50 - Closed Won",
			check: func(t *testing.T, c *spec.Condition) {
				// Values contain spaces and hyphens. Splitting on anything
				// other than the operator token would mangle this.
				if c.Value != "50 - Closed Won" {
					t.Errorf("Value = %q, want %q", c.Value, "50 - Closed Won")
				}
			},
		},
		{
			flow:  "[PIC] Create support case from emails",
			query: "type=received^receive_typeINforward,new^recipientsLIKEnissaneupic-support@wso2.com",
			check: func(t *testing.T, c *spec.Condition) {
				if len(c.All) != 3 {
					t.Fatalf("All = %d, want 3", len(c.All))
				}
				if c.All[1].Op != spec.OpIn || c.All[1].Value != "forward,new" {
					t.Errorf("IN term = %+v, want the comma-separated list intact", c.All[1])
				}
				if c.All[2].Op != spec.OpContains {
					t.Errorf("LIKE became %q, want contains", c.All[2].Op)
				}
			},
		},
		{
			flow:  "AI SP to WOW",
			query: "sys_created_byNOT LIKE@wso2.com^sys_created_byLIKE@^name=sn_customerservice_case",
			check: func(t *testing.T, c *spec.Condition) {
				if len(c.All) != 3 {
					t.Fatalf("All = %d, want 3", len(c.All))
				}
				// "NOT LIKE" must beat "LIKE" in operator ordering, or this
				// parses as field "sys_created_byNOT" — silently wrong.
				if c.All[0].Op != spec.OpNotContains {
					t.Errorf("first op = %q, want not_contains", c.All[0].Op)
				}
				if c.All[1].Op != spec.OpContains {
					t.Errorf("second op = %q, want contains", c.All[1].Op)
				}
			},
		},
		{
			flow:  "end date closure action",
			query: "u_is_approved=true^u_customer_project.u_wso2_closure_state!=Suspended",
			check: func(t *testing.T, c *spec.Condition) {
				// "!=" must beat "=" in ordering.
				if c.All[1].Op != spec.OpNeq {
					t.Errorf("op = %q, want neq", c.All[1].Op)
				}
			},
		},
		{
			flow:  "WSO2 Managed Service PD Flow (uses ^NQ)",
			query: "u_case_type=8d4b87bd^priorityIN14^priorityVALCHANGES^NQu_case_type=8d4b87bd^priorityCHANGESFROM12",
			check: func(t *testing.T, c *spec.Condition) {
				// ^NQ is an OR across WHOLE groups.
				if len(c.Any) != 2 {
					t.Fatalf("Any = %d groups, want 2 from the ^NQ split", len(c.Any))
				}
				if len(c.Any[0].All) != 3 || len(c.Any[1].All) != 2 {
					t.Errorf("group sizes = %d/%d, want 3/2", len(c.Any[0].All), len(c.Any[1].All))
				}
			},
		},
		{
			flow:  "case classification",
			query: "short_descriptionISNOTEMPTY^descriptionISNOTEMPTY",
			check: func(t *testing.T, c *spec.Condition) {
				for i, sub := range c.All {
					if sub.Op != spec.OpExists {
						t.Errorf("term %d op = %q, want exists", i, sub.Op)
					}
				}
			},
		},
		{
			flow:  "Twilio IAM LK P1-Case Created",
			query: "priority=10^u_case_type=8d4b87bd^u_wso2_product!=252a8a6e^ORu_wso2_product!=da90e058",
			check: func(t *testing.T, c *spec.Condition) {
				if len(c.All) != 3 {
					t.Fatalf("All = %d, want 3 (the ^OR folds into the third)", len(c.All))
				}
				// ^OR attaches to the IMMEDIATELY PRECEDING term, so only the
				// third element becomes an Any — not the whole query.
				if len(c.All[2].Any) != 2 {
					t.Errorf("third term Any = %d, want 2", len(c.All[2].Any))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.flow, func(t *testing.T) {
			c, err := Parse(tt.query)
			if err != nil {
				t.Fatalf("Parse(%q) error = %v", tt.query, err)
			}
			if c == nil {
				t.Fatalf("Parse(%q) = nil, want a condition", tt.query)
			}
			// Everything the translator emits must survive the engine's own
			// publish-time validation, or the port produces unpublishable
			// specs.
			if err := c.Validate(); err != nil {
				t.Fatalf("translated condition fails spec validation: %v", err)
			}
			tt.check(t, c)
		})
	}
}

// Several real flows are configured with no condition at all — they fire on
// every matching record. "^EQ" is ServiceNow's end-of-query marker.
func TestParse_EmptyMeansNoCondition(t *testing.T) {
	for _, q := range []string{"", "  ", "^EQ", "^EQ  "} {
		c, err := Parse(q)
		if err != nil {
			t.Errorf("Parse(%q) error = %v, want nil", q, err)
		}
		if c != nil {
			t.Errorf("Parse(%q) = %+v, want nil (no condition)", q, c)
		}
	}
}

// Refuse rather than guess. A silently-wrong condition means automation
// firing on the wrong records, which is worse than a port that stops and asks.
func TestParse_RefusesWhatItCannotTranslate(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{
			// Real, from "Incident commented and state changed".
			name:  "DYNAMIC reference qualifier",
			query: "caller_idDYNAMIC90d1921e5f510100a9ad2572f2b477fe",
		},
		{
			// Real, from "Case Task Assignement".
			name:  "field-to-field comparison",
			query: "assigned_toNSAMEASparent.assigned_to",
		},
		{name: "no operator", query: "justafieldname"},
		{name: "empty field name", query: "=value"},
		{name: "unary with a value", query: "stateISEMPTYfoo"},
		{name: "leading ^OR with nothing to attach to", query: "ORstate=1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Parse(tt.query); err == nil {
				t.Errorf("Parse(%q) = nil error, want a refusal", tt.query)
			}
		})
	}
}

// Operator ordering is the subtlest failure mode: a shorter token matching
// first turns "stateCHANGESTO3" into field "state" op CHANGES value "TO3" —
// which parses cleanly and is completely wrong.
func TestParse_OperatorOrderingIsLongestFirst(t *testing.T) {
	tests := []struct {
		query     string
		wantField string
		wantOp    spec.Operator
		wantValue any
	}{
		{"stateCHANGESTO3", "state", spec.OpChangedTo, "3"},
		{"stateCHANGESFROM3", "state", spec.OpChangedFrom, "3"},
		{"stateVALCHANGES", "state", spec.OpChanged, nil},
		{"emailNOT LIKEwso2", "email", spec.OpNotContains, "wso2"},
		{"emailLIKEwso2", "email", spec.OpContains, "wso2"},
		{"priority!=10", "priority", spec.OpNeq, "10"},
		{"priority>=10", "priority", spec.OpGte, "10"},
		{"priority<=10", "priority", spec.OpLte, "10"},
		{"priority=10", "priority", spec.OpEq, "10"},
		{"nameSTARTSWITHCS", "name", spec.OpStartsWith, "CS"},
		{"nameENDSWITHcom", "name", spec.OpEndsWith, "com"},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			c, err := Parse(tt.query)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if c.Field != tt.wantField {
				t.Errorf("Field = %q, want %q", c.Field, tt.wantField)
			}
			if c.Op != tt.wantOp {
				t.Errorf("Op = %q, want %q", c.Op, tt.wantOp)
			}
			if c.Value != tt.wantValue {
				t.Errorf("Value = %v, want %v", c.Value, tt.wantValue)
			}
		})
	}
}
