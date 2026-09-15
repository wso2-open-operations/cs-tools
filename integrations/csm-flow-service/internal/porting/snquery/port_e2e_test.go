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

package snquery_test

import (
	"encoding/json"
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-flow-service/internal/eval"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-flow-service/internal/porting/snquery"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-flow-service/internal/spec"
)

// TestPortOneFlowEndToEnd is the porting procedure itself, executed as a test.
// It is the answer to "how do these packages replace a flow", and it runs the
// four steps in order on a real flow.
//
// Subject: the live "Watch List" flow — 295 executions, 0 errors in 90 days,
// trigger record_update on sn_customerservice_case, condition
// "commentsVALCHANGES", Run Trigger=every. Chosen because it works today, so
// its intended behaviour is not in question.
func TestPortOneFlowEndToEnd(t *testing.T) {
	// STEP 1 — take the encoded query verbatim from ServiceNow and translate.
	const snCondition = "commentsVALCHANGES"

	cond, err := snquery.Parse(snCondition)
	if err != nil {
		t.Fatalf("step 1: translating %q: %v", snCondition, err)
	}

	got, _ := json.Marshal(cond)
	const want = `{"field":"comments","op":"changed"}`
	if string(got) != want {
		t.Errorf("step 1: translated to %s, want %s", got, want)
	}

	// STEP 2 — author the flow around it. The trigger fields come straight
	// from the ServiceNow trigger; the nodes are the port of its actions.
	flow := spec.Flow{
		Key: "watch-list",
		Trigger: spec.Trigger{
			Kind:       spec.TriggerRecordUpdate,
			EntityType: "case",
			// ServiceNow reported Run Trigger=every for this flow. Copying it
			// is not optional: unique_changes would fire only on the
			// transition, and this flow is meant to fire on every comment.
			RunTrigger: spec.RunTriggerEvery,
			Condition:  cond,
		},
		Nodes: []spec.Node{
			// The lookup the original design had no primitive for.
			{ID: "watchers", Type: spec.NodeLookup, As: "watchers"},
			{
				ID:        "has-watchers",
				Type:      spec.NodeIf,
				Condition: &spec.Condition{Field: "$.vars.watchers|len", Op: spec.OpGt, Value: 0},
				Children: []spec.Node{{
					ID: "each", Type: spec.NodeForEach, Over: "$.vars.watchers", As: "w",
					Children: []spec.Node{{ID: "mail", Type: spec.NodeNotify}},
				}},
			},
		},
	}

	// STEP 3 — validate at publish. Nothing unpublishable reaches the engine.
	if err := flow.Validate(); err != nil {
		t.Fatalf("step 3: flow is not publishable: %v", err)
	}

	// STEP 4 — evaluate against real inbound events.
	tests := []struct {
		name  string
		event map[string]any
		want  bool
	}{
		{
			name: "a customer comment fires it",
			event: map[string]any{
				"entityType": "case",
				"entityId":   "abc-123",
				"changes": map[string]any{
					"comments": map[string]any{"from": "", "to": "customer replied"},
				},
			},
			want: true,
		},
		{
			name: "an unrelated priority change does not",
			event: map[string]any{
				"entityType": "case",
				"changes": map[string]any{
					"priority": map[string]any{"from": 11.0, "to": 10.0},
				},
			},
			want: false,
		},
		{
			name: "a create event with no changes block does not, and does not error",
			event: map[string]any{
				"entityType": "case",
				"entityId":   "new-1",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fire, err := eval.Evaluate(*flow.Trigger.Condition, eval.Context{Event: tt.event})
			if err != nil {
				t.Fatalf("step 4: %v", err)
			}
			if fire != tt.want {
				t.Errorf("step 4: fire = %v, want %v", fire, tt.want)
			}
		})
	}

	// The inner condition reads run scope, which a lookup fills at execution.
	// Proving it evaluates correctly against both shapes is what makes the
	// `lookup` + `$.vars` design usable.
	inner := *flow.Nodes[1].Condition
	for _, tc := range []struct {
		name string
		vars map[string]any
		want bool
	}{
		{"watchers found", map[string]any{"watchers": []any{map[string]any{"email": "a@wso2.com"}}}, true},
		{"no watchers", map[string]any{"watchers": []any{}}, false},
	} {
		t.Run("inner/"+tc.name, func(t *testing.T) {
			ok, err := eval.Evaluate(inner, eval.Context{Vars: tc.vars})
			if err != nil {
				t.Fatalf("%v", err)
			}
			if ok != tc.want {
				t.Errorf("got %v, want %v", ok, tc.want)
			}
		})
	}
}

// TestPortEveryRealCondition runs every trigger condition captured from the
// instance through translate -> validate. It is the coverage gate for the
// port: a condition that cannot survive both steps is a flow that cannot be
// ported without hand work, and we want that list to be short and known.
func TestPortEveryRealCondition(t *testing.T) {
	real := map[string]string{
		"Watch List":                     "commentsVALCHANGES",
		"case classification":            "short_descriptionISNOTEMPTY^descriptionISNOTEMPTY",
		"Asgardeo Account bine":          "product=720bf91d1bf92010cb6898aebd4bcb10",
		"Twilio alerts creating P1":      "priority=10",
		"Twilio alearts assigning P1":    "priority=10^assigned_toCHANGESFROM",
		"TimeCard Billable state":        "u_case_typeCHANGESTO0d5b8fbd1b18f010cb6898aebd4bcba5",
		"TimeCard billable Manager":      "u_billable_stateVALCHANGES^ORu_case_typeVALCHANGES",
		"CR Approval notifications":      "stateCHANGESTO-4^ORstateCHANGESTO-3^ORstateCHANGESTO5",
		"CR change start plan date":      "u_customer_updatedVALCHANGES^state=5^sys_updated_byNOT LIKE@wso2.com",
		"Create Incident Report Task":    "stateCHANGESTO2",
		"Update Project State":           "u_wso2_closure_stateVALCHANGES",
		"Update Contact State":           "locked_outVALCHANGES",
		"Monitor Password Change":        "user_passwordVALCHANGES",
		"Outage update":                  "u_internal_communicationVALCHANGES^u_outage_communication=true^beginISNOTEMPTY^endISEMPTY",
		"end date closure action":        "u_is_approved=true^u_customer_project.u_wso2_closure_state!=Suspended",
		"Populate Customer Engagements":  "u_stageCHANGESTO50 - Closed Won^ORu_stageCHANGESTO40 - Negotiation/Review",
		"[Change Request] Add Comment":   "u_git_referenceISNOTEMPTY^commentsVALCHANGES",
		"[Change Request] Update State":  "u_git_referenceISNOTEMPTY^stateVALCHANGES",
		"[CR] Fields Changes Comments":   "state=5^u_customer_updatedVALCHANGES",
		"Account Manager Notifications":  "accountISNOTEMPTY^account.u_ownerISNOTEMPTY^account.u_owner.emailENDSWITHwso2.com",
		"when customer comment add Att.": "table_name=sn_customerservice_case^sys_created_byISNOTEMPTY",
		"Update Project - Last Case":     "sys_created_byNOT LIKE@wso2.com^sys_created_byLIKE@^u_case_type!=3b8b4331",
		"Runbook selector for incident":  "contact_type=1^ORcontact_type=2",
		"DORA Metrics":                   "business_service=b9c999f8^stateCHANGESTO1^ORstateCHANGESTO6",
		"[PIC] Create case from email":   "type=received^receive_typeINforward,new^recipientsLIKEsupport@wso2.com",
		"WSO2 Managed Service PD Flow":   "u_case_type=8d4b87bd^priorityIN14^priorityVALCHANGES^NQu_case_type=8d4b87bd^priorityCHANGESFROM12",
		"CallMgt.Notes Pendind state":    "state=3^u_meeting_dateISNOTEMPTY",
		"CallMgt.Reschedule call alert":  "stateCHANGESTO2^u_final_timesVALCHANGES^ORu_durationVALCHANGES",
		"[WSO2 Cloud Ops] Post res.":     "business_service=b9c999f8^ORbusiness_service=97ed1b8b^stateCHANGESTO6",
		"CS_Project_Announcements":       "^EQ",
		"Outage Communication":           "^EQ",
	}

	var failed []string
	translated := 0

	for name, q := range real {
		cond, err := snquery.Parse(q)
		if err != nil {
			failed = append(failed, name+": "+err.Error())
			continue
		}
		if cond == nil {
			// "^EQ" — no condition, fires on every matching record. Valid.
			translated++
			continue
		}
		if err := cond.Validate(); err != nil {
			failed = append(failed, name+": translated but unpublishable: "+err.Error())
			continue
		}
		translated++
	}

	if len(failed) > 0 {
		for _, f := range failed {
			t.Errorf("could not port: %s", f)
		}
	}
	t.Logf("translated %d/%d real trigger conditions to publishable specs", translated, len(real))
}
