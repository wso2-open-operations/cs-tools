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

package repository

import (
	"testing"
)

// Unlabeled (ServiceNow-synced) stages: what a stage with no checkpoint_label is
// taken for, and the "nobody is eligible" counts.

func TestRuntimeApprovalStageKind(t *testing.T) {
	str := func(s string) *string { return &s }
	for _, tc := range []struct {
		name         string
		label, group *string
		pos          int
		model, state string
		want         approvalStageKind
	}{
		// A label wins, wherever the stage sits and whatever the change is doing.
		{"labelled Peer, any state", str("Peer Approval"), nil, 0, "NORMAL", "CLOSED", stageKindPeer},
		{"labelled customer stage keeps its kind", str("Customer Approval"), nil, 2, "NORMAL", "ASSESS", stageKindCustomerApproval},
		{"labelled with something unknown", str("SN Change Approval"), nil, 0, "NORMAL", "AUTHORIZE", stageKindOther},
		// Position guesses count only in the state they imply.
		{"position 0 in Assess is Peer", nil, nil, 0, "NORMAL", "ASSESS", stageKindPeer},
		{"position 0 in Authorize is not a guess to act on", nil, nil, 0, "NORMAL", "AUTHORIZE", stageKindOther},
		{"position 1 in Authorize is CAB", nil, nil, 1, "NORMAL", "AUTHORIZE", stageKindCAB},
		{"position 1 in Assess is not", nil, nil, 1, "NORMAL", "ASSESS", stageKindOther},
		{"position 0 in Implement", nil, nil, 0, "NORMAL", "IMPLEMENT", stageKindOther},
		{"position 2 is nothing the flow knows", nil, nil, 2, "NORMAL", "REVIEW", stageKindOther},
		{"position 2 is never the customer's", nil, nil, 2, "NORMAL", "CUSTOMER_APPROVAL", stageKindOther},
		{"position 3 in Customer Review is never the customer's", nil, nil, 3, "NORMAL", "CUSTOMER_REVIEW", stageKindOther},
		{"an empty label is no label", str(""), nil, 1, "NORMAL", "AUTHORIZE", stageKindCAB},
		// The group the stage is assigned to says what it is.
		{"ECAB group in Authorize", nil, str("ECAB Approval"), 3, "NORMAL", "AUTHORIZE", stageKindECAB},
		{"CAB group in Authorize, any position", nil, str("CAB Approval"), 4, "NORMAL", "AUTHORIZE", stageKindCAB},
		{"CAB group in Scheduled is not cancelled for it", nil, str("CAB Approval"), 1, "NORMAL", "SCHEDULED", stageKindOther},
		{"CAB group name is exact", nil, str("CAB Approval Board"), 5, "NORMAL", "AUTHORIZE", stageKindOther},
		{"another group falls back to the position", nil, str("Devops"), 1, "NORMAL", "AUTHORIZE", stageKindCAB},
		// An Emergency change in Authorize has only the ECAB's stage.
		{"Emergency in Authorize, position 0, no label", nil, nil, 0, "EMERGENCY", "AUTHORIZE", stageKindECAB},
		{"Emergency in Authorize, position 2, no label", nil, nil, 2, "EMERGENCY", "AUTHORIZE", stageKindECAB},
		{"Emergency in Scheduled", nil, nil, 0, "EMERGENCY", "SCHEDULED", stageKindOther},
		{"Normal in Authorize, position 0", nil, nil, 0, "NORMAL", "AUTHORIZE", stageKindOther},
		{"no model, position 0, Authorize", nil, nil, 0, "", "AUTHORIZE", stageKindOther},
		// No state: nothing to match the guess against.
		{"NULL state, position 0", nil, nil, 0, "NORMAL", "", stageKindOther},
		{"padding and case are normalised", nil, nil, 1, " normal ", " authorize ", stageKindCAB},
	} {
		if got := runtimeApprovalStageKind(tc.label, tc.pos, tc.group, tc.model, tc.state); got != tc.want {
			t.Errorf("%s: kind = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// An unlabeled stage is never of a kind that approvalStageOutOfState can refuse,
// in any state: the guess only ever returns a kind in the state it is decided in.
func TestRuntimeApprovalStageKind_NeverOutOfState(t *testing.T) {
	groups := []*string{nil, strPtrLock("CAB Approval"), strPtrLock("ECAB Approval"), strPtrLock("Something")}
	for _, group := range groups {
		for _, model := range []string{"", "NORMAL", "EMERGENCY", "STANDARD"} {
			for state := range knownChangeRequestStates {
				for pos := 0; pos < 5; pos++ {
					kind := runtimeApprovalStageKind(nil, pos, group, model, state)
					if approvalStageOutOfState(kind, state) {
						t.Errorf("an unlabeled stage (group %v, model %q, position %d) in %s reads as %v, which is out of state", group, model, pos, state, kind)
					}
					if kind == stageKindCustomerApproval || kind == stageKindCustomerReview {
						t.Errorf("an unlabeled stage (group %v, model %q, position %d) in %s was taken for a customer stage", group, model, pos, state)
					}
				}
			}
		}
	}
}

func strPtrLock(s string) *string { return &s }

func TestExcludedMembersSummary(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   excludedMembers
		want string
	}{
		{"the staging shape", excludedMembers{total: 14, inactive: 3, byType: map[string]int{"NOT_AVAILABLE": 9, "EXTERNAL": 1}, creator: 1},
			"14 members, none eligible: 9 user_type NOT_AVAILABLE, 3 inactive, 1 creator, 1 external"},
		{"missing users and no type", excludedMembers{total: 3, noUser: 2, byType: map[string]int{"": 1}}, "3 members, none eligible: 2 no user record, 1 no user_type"},
		{"one member", excludedMembers{total: 1, byType: map[string]int{"SYSTEM": 1}}, "1 member, none eligible: 1 system"},
		{"some eligible", excludedMembers{total: 5, inactive: 1, eligible: 4}, "5 members, 4 eligible"},
		{"empty", excludedMembers{}, "0 members, none eligible"},
	} {
		if got := tc.in.summary(); got != tc.want {
			t.Errorf("%s: %q, want %q", tc.name, got, tc.want)
		}
	}
}
