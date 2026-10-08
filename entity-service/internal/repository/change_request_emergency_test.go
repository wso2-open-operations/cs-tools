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
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// The pure half of the Emergency rule (change_request_emergency.go): an Emergency
// change takes no customer step, and its approval is the one CAB stage the previous
// system itself gives it. The database half is in change_request_emergency_integration_test.go.

func emergencyType(s string) *domain.ChangeRequestType {
	t := domain.ChangeRequestType(s)
	return &t
}

func boolPtrEmergency(b bool) *bool { return &b }

func TestIsEmergencyModel(t *testing.T) {
	for in, want := range map[string]bool{
		"EMERGENCY": true, "emergency": true, " Emergency ": true,
		"NORMAL": false, "STANDARD": false, "": false, "AZURE": false, "EMERGENCY_CHANGE": false,
	} {
		if got := isEmergencyModel(in); got != want {
			t.Errorf("isEmergencyModel(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestEffectiveCustomerGates(t *testing.T) {
	for _, model := range []string{"NORMAL", "STANDARD", "", "AZURE"} {
		for _, a := range []bool{false, true} {
			for _, r := range []bool{false, true} {
				if ga, gr := effectiveCustomerGates(model, a, r); ga != a || gr != r {
					t.Errorf("effectiveCustomerGates(%q, %v, %v) = %v, %v, want them unchanged", model, a, r, ga, gr)
				}
			}
		}
	}
	// An Emergency change has neither gate, whatever the stored boxes say.
	for _, a := range []bool{false, true} {
		for _, r := range []bool{false, true} {
			if ga, gr := effectiveCustomerGates("EMERGENCY", a, r); ga || gr {
				t.Errorf("effectiveCustomerGates(EMERGENCY, %v, %v) = %v, %v, want false, false", a, r, ga, gr)
			}
		}
	}
}

func TestEffectiveChangeModel(t *testing.T) {
	for _, tc := range []struct {
		name      string
		stored    string
		requested *domain.ChangeRequestType
		want      string
	}{
		{"no request type keeps the stored one", "NORMAL", nil, "NORMAL"},
		{"the request's type wins", "NORMAL", emergencyType("emergency"), "EMERGENCY"},
		{"and wins the other way", "EMERGENCY", emergencyType("standard"), "STANDARD"},
		{"a type with no change_model label changes nothing", "EMERGENCY", emergencyType("model"), "EMERGENCY"},
		{"a NULL stored model", "", emergencyType("emergency"), "EMERGENCY"},
	} {
		if got := effectiveChangeModel(tc.stored, tc.requested); got != tc.want {
			t.Errorf("%s: effectiveChangeModel(%q, %v) = %q, want %q", tc.name, tc.stored, tc.requested, got, tc.want)
		}
	}
}

func TestValidateCreateChangeRequestCustomerGates(t *testing.T) {
	yes, no := boolPtrEmergency(true), boolPtrEmergency(false)
	for _, tc := range []struct {
		name             string
		typ              *domain.ChangeRequestType
		approval, review *bool
		wantFields       []string // nil: accepted
	}{
		{"emergency, neither box", emergencyType("emergency"), nil, nil, nil},
		{"emergency, both boxes explicitly off", emergencyType("emergency"), no, no, nil},
		{"emergency with the approval box", emergencyType("emergency"), yes, nil, []string{"customerApprovalRequired"}},
		{"emergency with the review box", emergencyType("emergency"), no, yes, []string{"customerReviewRequired"}},
		{"emergency with both boxes", emergencyType("emergency"), yes, yes, []string{"customerApprovalRequired", "customerReviewRequired"}},
		{"normal with both boxes is the customer's business", emergencyType("normal"), yes, yes, nil},
		{"standard with both boxes is the customer's business", emergencyType("standard"), yes, yes, nil},
		{"no type: that is ValidateCreateChangeRequestType's refusal, not this one", nil, yes, yes, nil},
	} {
		err := ValidateCreateChangeRequestCustomerGates(tc.typ, tc.approval, tc.review)
		if tc.wantFields == nil {
			if err != nil {
				t.Errorf("%s: err = %v, want nil", tc.name, err)
			}
			continue
		}
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) {
			t.Errorf("%s: err = %v (%T), want a ValidationError", tc.name, err, err)
			continue
		}
		if !strings.HasPrefix(ve.Msg, emergencyNoCustomerConsentMsg) {
			t.Errorf("%s: message %q does not give the reason %q", tc.name, ve.Msg, emergencyNoCustomerConsentMsg)
		}
		for _, f := range tc.wantFields {
			if !strings.Contains(ve.Msg, f) {
				t.Errorf("%s: message %q does not name %s", tc.name, ve.Msg, f)
			}
		}
	}
	// The one sentence the user asked for, verbatim.
	if emergencyNoCustomerConsentMsg != "Emergency changes proceed without customer consent, so customer approval and customer review cannot be required" {
		t.Errorf("the refusal reason changed: %q", emergencyNoCustomerConsentMsg)
	}
}

func TestCheckEmergencyCustomerConsent(t *testing.T) {
	yes, no := boolPtrEmergency(true), boolPtrEmergency(false)
	snap := func(model string, approval, review bool) changeRequestGateSnapshot {
		return changeRequestGateSnapshot{state: crStateNew, model: model, approvalRequired: approval, reviewRequired: review}
	}
	for _, tc := range []struct {
		name string
		snap changeRequestGateSnapshot
		req  domain.PatchChangeRequestRequest
		// want is "" for accepted, "set" for the set-a-box refusal, "retype" for the re-type one.
		want       string
		wantFields []string
	}{
		{"an unrelated edit of an Emergency change", snap("EMERGENCY", false, false), domain.PatchChangeRequestRequest{Title: strPtrLock("t")}, "", nil},
		{"Emergency stays Emergency, boxes off", snap("EMERGENCY", false, false), domain.PatchChangeRequestRequest{CustomerApprovalRequired: no, CustomerReviewRequired: no}, "", nil},
		{"tick the approval box on an Emergency change", snap("EMERGENCY", false, false), domain.PatchChangeRequestRequest{CustomerApprovalRequired: yes}, "set", []string{"customerApprovalRequired"}},
		{"tick the review box on an Emergency change", snap("EMERGENCY", false, false), domain.PatchChangeRequestRequest{CustomerReviewRequired: yes}, "set", []string{"customerReviewRequired"}},
		{"tick both", snap("EMERGENCY", false, false), domain.PatchChangeRequestRequest{CustomerApprovalRequired: yes, CustomerReviewRequired: yes}, "set", []string{"customerApprovalRequired", "customerReviewRequired"}},
		{"tick a box and re-send the type", snap("EMERGENCY", false, false), domain.PatchChangeRequestRequest{Type: emergencyType("emergency"), CustomerApprovalRequired: yes}, "set", []string{"customerApprovalRequired"}},
		// A legacy Emergency row (from before the rule) or a migrated one can carry a box:
		// writing the value it already holds is the no-op it is everywhere in the lock.
		{"a whole-form resend of a legacy ticked box", snap("EMERGENCY", true, true), domain.PatchChangeRequestRequest{CustomerApprovalRequired: yes, CustomerReviewRequired: yes, Type: emergencyType("emergency")}, "", nil},
		{"a legacy Emergency row, an unrelated edit", snap("EMERGENCY", true, false), domain.PatchChangeRequestRequest{Title: strPtrLock("t")}, "", nil},
		{"a legacy Emergency row, the other box ticked", snap("EMERGENCY", true, false), domain.PatchChangeRequestRequest{CustomerReviewRequired: yes}, "set", []string{"customerReviewRequired"}},
		// Re-typing INTO Emergency while a box stays ticked.
		{"Normal with a ticked box re-typed to Emergency", snap("NORMAL", true, false), domain.PatchChangeRequestRequest{Type: emergencyType("emergency")}, "retype", []string{"customerApprovalRequired"}},
		{"Standard with the review box re-typed to Emergency", snap("STANDARD", false, true), domain.PatchChangeRequestRequest{Type: emergencyType("emergency")}, "retype", []string{"customerReviewRequired"}},
		{"re-type and tick in one request", snap("NORMAL", false, false), domain.PatchChangeRequestRequest{Type: emergencyType("emergency"), CustomerReviewRequired: yes}, "retype", []string{"customerReviewRequired"}},
		{"re-type to Emergency and untick in the same request", snap("NORMAL", true, true), domain.PatchChangeRequestRequest{Type: emergencyType("emergency"), CustomerApprovalRequired: no, CustomerReviewRequired: no}, "", nil},
		{"re-type to Emergency and untick one of two", snap("NORMAL", true, true), domain.PatchChangeRequestRequest{Type: emergencyType("emergency"), CustomerApprovalRequired: no}, "retype", []string{"customerReviewRequired"}},
		{"Normal re-typed to Emergency with no box", snap("NORMAL", false, false), domain.PatchChangeRequestRequest{Type: emergencyType("emergency")}, "", nil},
		{"a NULL-model legacy row re-typed to Emergency with a box", snap("", true, false), domain.PatchChangeRequestRequest{Type: emergencyType("emergency")}, "retype", []string{"customerApprovalRequired"}},
		// Everything else is the customer's business.
		{"Normal, tick both", snap("NORMAL", false, false), domain.PatchChangeRequestRequest{CustomerApprovalRequired: yes, CustomerReviewRequired: yes}, "", nil},
		{"Emergency re-typed to Normal, tick a box in the same request", snap("EMERGENCY", false, false), domain.PatchChangeRequestRequest{Type: emergencyType("normal"), CustomerApprovalRequired: yes}, "", nil},
		{"Standard, a legacy box", snap("STANDARD", true, true), domain.PatchChangeRequestRequest{Title: strPtrLock("t")}, "", nil},
	} {
		err := checkEmergencyCustomerConsent(tc.snap, tc.req)
		if tc.want == "" {
			if err != nil {
				t.Errorf("%s: err = %v, want nil", tc.name, err)
			}
			continue
		}
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) {
			t.Errorf("%s: err = %v (%T), want a ValidationError", tc.name, err, err)
			continue
		}
		if !strings.HasPrefix(ve.Msg, emergencyNoCustomerConsentMsg) {
			t.Errorf("%s: message %q does not give the reason", tc.name, ve.Msg)
		}
		if retype := strings.Contains(ve.Msg, "before changing the type to emergency"); retype != (tc.want == "retype") {
			t.Errorf("%s: message %q is the wrong kind of refusal (want %s)", tc.name, ve.Msg, tc.want)
		}
		for _, f := range tc.wantFields {
			if !strings.Contains(ve.Msg, f) {
				t.Errorf("%s: message %q does not name %s", tc.name, ve.Msg, f)
			}
		}
	}
}

// The refusal is part of the creation-phase gate every PATCH that carries a type or a box
// goes through, and it comes before the lock's own box rules: a tick on an Emergency change
// is refused for the Emergency reason in every state, not for one about a gate it never has.
func TestValidateCreationPhaseEdits_EmergencyRuleComesFirst(t *testing.T) {
	yes := boolPtrEmergency(true)
	for _, state := range []string{"", crStateNew, crStateAssess, crStateAuthorize, crStateScheduled, crStateImplement, crStateReview, crStateClosed} {
		snap := changeRequestGateSnapshot{state: state, model: "EMERGENCY", projectID: strPtrLock("00000000-0000-4000-8000-0000000000a1")}
		err := validateCreationPhaseEdits(snap, domain.PatchChangeRequestRequest{CustomerApprovalRequired: yes})
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) || !strings.HasPrefix(ve.Msg, emergencyNoCustomerConsentMsg) {
			t.Errorf("state %q: err = %v, want the Emergency refusal", state, err)
		}
	}
	// Normal is not touched by it.
	if err := validateCreationPhaseEdits(changeRequestGateSnapshot{state: crStateNew, model: "NORMAL"}, domain.PatchChangeRequestRequest{CustomerApprovalRequired: yes}); err != nil {
		t.Errorf("a Normal change in New ticking a box: %v, want nil", err)
	}
}

// legalNextStates of an Emergency change, per state: it never offers a customer state, and
// its Review goes straight to Closed whatever its stored review box says (a legacy or
// migrated row can carry one). The same table without the Emergency rule is what a Normal
// change gets, which is the contrast pinned last.
func TestLegalChangeRequestNextStatesForModel_Emergency(t *testing.T) {
	for _, tc := range []struct {
		state string
		want  []string
	}{
		{"new", []string{"assess", "canceled"}},
		{"assess", []string{"canceled"}},
		{"authorize", []string{"canceled"}}, // the CAB approval moves it on, to Scheduled
		{"scheduled", []string{"implement", "canceled"}},
		{"implement", []string{"review", "canceled"}},
		{"review", []string{"closed", "rollback", "canceled"}},
		{"closed", nil},
		{"canceled", nil},
		{"rollback", nil},
	} {
		for _, stored := range []bool{false, true} {
			st := tc.state
			got := legalChangeRequestNextStatesForModel(&st, "EMERGENCY", stored)
			if fmt.Sprint(got) != fmt.Sprint(tc.want) {
				t.Errorf("Emergency %s (stored review box %v): next states = %v, want %v", tc.state, stored, got, tc.want)
			}
			for _, next := range got {
				if next == "customer_approval" || next == "customer_review" {
					t.Errorf("Emergency %s offers %q", tc.state, next)
				}
			}
		}
	}
	// A state a legacy Emergency row can still be in (it was moved there before the rule) keeps
	// the exits that answer nothing for the customer.
	for state, want := range map[string][]string{
		"customer_approval": {"authorize", "canceled"},
		"customer_review":   {"rollback", "canceled"},
	} {
		st := state
		if got := legalChangeRequestNextStatesForModel(&st, "EMERGENCY", true); fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("Emergency %s: next states = %v, want %v", state, got, want)
		}
	}
	// Contrast: a Normal change with the box ticked is sent to Customer Review.
	review := "review"
	if got := legalChangeRequestNextStatesForModel(&review, "NORMAL", true); fmt.Sprint(got) != "[customer_review rollback canceled]" {
		t.Errorf("Normal review with the box ticked: next states = %v", got)
	}
	if got := legalChangeRequestNextStatesForModel(&review, "", true); fmt.Sprint(got) != "[customer_review rollback canceled]" {
		t.Errorf("legacy (NULL type) review with the box ticked: next states = %v", got)
	}
}

// An unlabeled stage on an Emergency change in the CAB group is named for what it is, not
// for its position; a labelled one is shown as it was written, including the label of an
// earlier build.
func TestChangeRequestApprovalStageLabel_Emergency(t *testing.T) {
	str := func(s string) *string { return &s }
	for _, tc := range []struct {
		name  string
		st    changeRequestApprovalStageRow
		pos   int
		model string
		want  string
	}{
		{"migrated Emergency, CAB group, position 0", changeRequestApprovalStageRow{assignmentGroupName: str("CAB Approval")}, 0, "EMERGENCY", "CAB Approval"},
		{"migrated Emergency, CAB group, position 1", changeRequestApprovalStageRow{assignmentGroupName: str("CAB Approval")}, 1, "EMERGENCY", "CAB Approval"},
		{"migrated Emergency, CAB group, an empty label", changeRequestApprovalStageRow{assignmentGroupName: str("CAB Approval"), checkpointLabel: str("")}, 0, "EMERGENCY", "CAB Approval"},
		{"migrated Emergency, another group keeps the positional name", changeRequestApprovalStageRow{assignmentGroupName: str("Devops Approval")}, 0, "EMERGENCY", "Assess"},
		{"migrated Emergency, no group keeps the positional name", changeRequestApprovalStageRow{}, 0, "EMERGENCY", "Assess"},
		{"migrated Normal, CAB group at position 1 keeps the positional name", changeRequestApprovalStageRow{assignmentGroupName: str("CAB Approval")}, 1, "NORMAL", "Authorize"},
		{"migrated Normal, CAB group at position 0 keeps the positional name", changeRequestApprovalStageRow{assignmentGroupName: str("CAB Approval")}, 0, "NORMAL", "Assess"},
		{"no model at all keeps the positional name", changeRequestApprovalStageRow{assignmentGroupName: str("CAB Approval")}, 0, "", "Assess"},
		{"a native Emergency stage", changeRequestApprovalStageRow{assignmentGroupName: str("CAB Approval"), checkpointLabel: str("CAB Approval")}, 0, "EMERGENCY", "CAB Approval"},
		{"an earlier build's Emergency stage is still shown as written", changeRequestApprovalStageRow{assignmentGroupName: str("ECAB Approval"), checkpointLabel: str("ECAB Approval")}, 0, "EMERGENCY", "ECAB Approval"},
	} {
		got, typ := changeRequestApprovalStageLabel(tc.st, tc.pos, tc.model)
		if got != tc.want || typ != domain.ChangeRequestApproverTypeStaticGroup {
			t.Errorf("%s: label = %q / %q, want %q / STATIC_GROUP", tc.name, got, typ, tc.want)
		}
	}
}

// The whole approvals read of a migrated Emergency change: ONE unlabeled stage in the CAB
// group at position 0, UPPER_SNAKE approver states. It reads as the CAB stage, with the
// stage status derived from its approvers like any other.
func TestBuildChangeRequestApprovals_MigratedEmergencyStage(t *testing.T) {
	str := func(s string) *string { return &s }
	stages := []changeRequestApprovalStageRow{
		{id: "stage-1", assignmentGroupName: str("CAB Approval"), assignmentGroupID: str("22222222-2222-4222-8222-222222222222")},
	}
	approvers := []changeRequestApprovalApproverRow{
		{id: "a1", stageID: str("stage-1"), approverUserID: str("u-1"), approverName: "First Approver", rawStatus: str("APPROVED")},
		{id: "a2", stageID: str("stage-1"), approverUserID: str("u-2"), approverName: "Second Approver", rawStatus: str("NOT_REQUIRED")},
	}
	got := buildChangeRequestApprovals(stages, approvers, "EMERGENCY")
	if len(got.Approvals) != 1 {
		t.Fatalf("approvals = %d, want 1", len(got.Approvals))
	}
	a := got.Approvals[0]
	if a.Stage != "CAB Approval" || a.ApproverName != "CAB Approval" || a.Status != domain.ChangeRequestApprovalStatusApproved {
		t.Errorf("stage = %q / %q / %q, want CAB Approval / CAB Approval / APPROVED", a.Stage, a.ApproverName, a.Status)
	}
	if a.AssignmentGroup == nil || a.AssignmentGroup.Name != "CAB Approval" {
		t.Errorf("assignmentGroup = %+v, want the CAB group", a.AssignmentGroup)
	}
	// The same rows on a Normal change read positionally, as they always did.
	if got := buildChangeRequestApprovals(stages, approvers, "NORMAL"); got.Approvals[0].Stage != "Assess" {
		t.Errorf("the same stage on a Normal change reads %q, want the positional Assess (unchanged)", got.Approvals[0].Stage)
	}
}

// A stage an earlier build wrote for an Emergency change keeps its label on the wire and is
// decided as the CAB stage.
func TestBuildChangeRequestApprovals_HistoricECABStageIsStillShown(t *testing.T) {
	str := func(s string) *string { return &s }
	stages := []changeRequestApprovalStageRow{
		{id: "stage-1", assignmentGroupName: str("ECAB Approval"), assignmentGroupID: str("33333333-3333-4333-8333-333333333333"), checkpointLabel: str("ECAB Approval")},
	}
	approvers := []changeRequestApprovalApproverRow{{id: "a1", stageID: str("stage-1"), approverUserID: str("u-1"), approverName: "Approver", rawStatus: str("REQUESTED")}}
	got := buildChangeRequestApprovals(stages, approvers, "EMERGENCY")
	if a := got.Approvals[0]; a.Stage != "ECAB Approval" || a.ApproverName != "ECAB Approval" || a.Status != domain.ChangeRequestApprovalStatusPending || len(a.Approvers) != 1 {
		t.Errorf("the historic stage reads %+v, want it shown as written, pending, with its approver", a)
	}
	if kind := classifyApprovalStage(stages[0].checkpointLabel, 0); kind != stageKindCAB {
		t.Errorf("the historic stage is decided as kind %v, want the CAB stage", kind)
	}
}
