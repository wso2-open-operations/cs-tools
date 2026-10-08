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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// TestChangeRequestChangeModelToType covers all eight real
// change_request_change_model_enum labels (migration 0056) -- a coverage
// gap here would let a mistyped or omitted label silently drop a real
// change request's type on read.
func TestChangeRequestChangeModelToType(t *testing.T) {
	want := map[string]domain.ChangeRequestType{
		"AZURE":                domain.ChangeRequestTypeAzure,
		"CHANGE_REGISTRATION":  domain.ChangeRequestTypeChangeRegistration,
		"CLOUD_INFRASTRUCTURE": domain.ChangeRequestTypeCloudInfrastructure,
		"EMERGENCY":            domain.ChangeRequestTypeEmergency,
		"INFRA":                domain.ChangeRequestTypeInfra,
		"NORMAL":               domain.ChangeRequestTypeNormal,
		"STANDARD":             domain.ChangeRequestTypeStandard,
		"UNAUTHORIZED_CHANGE":  domain.ChangeRequestTypeUnauthorizedChange,
	}
	if len(changeRequestChangeModelToType) != len(want) {
		t.Fatalf("changeRequestChangeModelToType has %d entries, want %d", len(changeRequestChangeModelToType), len(want))
	}
	for enumValue, wantType := range want {
		got, ok := changeRequestChangeModelToType[enumValue]
		if !ok {
			t.Errorf("changeRequestChangeModelToType[%q] missing", enumValue)
			continue
		}
		if got != wantType {
			t.Errorf("changeRequestChangeModelToType[%q] = %q, want %q", enumValue, got, wantType)
		}
	}
}

// TestChangeRequestTypeToChangeModel_SupportedTypesRoundTrip proves every
// change-model-backed ChangeRequestType can be written back to its
// original enum label -- this is the map PatchChangeRequest's Type
// handling consults before persisting.
func TestChangeRequestTypeToChangeModel_SupportedTypesRoundTrip(t *testing.T) {
	for enumValue, t1 := range changeRequestChangeModelToType {
		got, ok := changeRequestTypeToChangeModel[t1]
		if !ok {
			t.Errorf("changeRequestTypeToChangeModel[%q] missing (from enum %q)", t1, enumValue)
			continue
		}
		if got != enumValue {
			t.Errorf("changeRequestTypeToChangeModel[%q] = %q, want %q", t1, got, enumValue)
		}
	}
}

// TestChangeRequestTypeToChangeModel_UnsupportedTypesRejected proves the
// two pre-existing ChangeRequestType values that predate change_model
// (migration 0056) have no reverse mapping -- PatchChangeRequest relies
// on this absence to reject them with a ValidationError instead of
// silently writing a wrong or empty change_model value.
func TestChangeRequestTypeToChangeModel_UnsupportedTypesRejected(t *testing.T) {
	for _, unsupported := range []domain.ChangeRequestType{
		domain.ChangeRequestTypeModel,
		domain.ChangeRequestTypeSiteReliabilityOps,
	} {
		if _, ok := changeRequestTypeToChangeModel[unsupported]; ok {
			t.Errorf("changeRequestTypeToChangeModel[%q] unexpectedly present -- PatchChangeRequest would silently accept a type with no real change_model equivalent", unsupported)
		}
	}
}

// fakeChangeRequestDetailRow feeds scanChangeRequestViewAndDetail fixed
// values without a real database connection, exercising the exact Scan
// destination order/types changeRequestSelectColumns+changeRequestDetailColumns
// produce. Zero-value fields are nil pointers, i.e. SQL NULL.
type fakeChangeRequestDetailRow struct {
	id, number                                                         string
	subject, description                                               *string
	projectID, projectName                                             *string
	caseID, caseNumber                                                 *string
	depID, depName                                                     *string
	dpID, dpName                                                       *string
	prodID, prodName                                                   *string
	svcID, svcName                                                     *string
	soID, soName                                                       *string
	aeID, aeName                                                       *string
	startOn, endOn                                                     *time.Time
	impact, state, changeModel                                         *string
	createdOn, updatedOn                                               time.Time
	agID, agName                                                       *string
	isOnHold                                                           *bool
	onHoldReason                                                       *string
	createdBy                                                          string
	justification, impactDescription, serviceOutage                    *string
	communicationPlan, rollbackPlan, testPlan                          *string
	isCustomerApproved, isCustomerReviewed                             *bool
	implementationPlan, priority, category                             *string
	rbID, rbName                                                       *string
	affectedServicesText, affectedComponentsText, rollbackDurationText *string
	changeRequestType, likelihood                                      *string
	isPlanningVisibleToCustomers                                       *bool
	confirmCustomerUpdatedDate                                         *string
	customerUpdatedOn, workStart, workEnd                              *time.Time
	gitReference                                                       *string
	customerApprovalRequired, customerReviewRequired                   bool
}

func (f fakeChangeRequestDetailRow) Scan(dest ...any) error {
	vals := []any{
		f.id, f.number, f.subject, f.description,
		f.projectID, f.projectName,
		f.caseID, f.caseNumber,
		f.depID, f.depName,
		f.dpID, f.dpName,
		f.prodID, f.prodName,
		f.svcID, f.svcName,
		f.soID, f.soName,
		f.aeID, f.aeName,
		f.startOn, f.endOn, f.impact, f.state, f.changeModel,
		f.createdOn, f.updatedOn,
		f.agID, f.agName,
		f.isOnHold, f.onHoldReason,
		f.createdBy, f.justification, f.impactDescription, f.serviceOutage, f.communicationPlan, f.rollbackPlan, f.testPlan,
		f.isCustomerApproved, f.isCustomerReviewed,
		f.implementationPlan, f.priority, f.category,
		f.rbID, f.rbName,
		f.affectedServicesText, f.affectedComponentsText, f.rollbackDurationText,
		f.changeRequestType, f.likelihood, f.isPlanningVisibleToCustomers,
		f.confirmCustomerUpdatedDate, f.customerUpdatedOn,
		f.workStart, f.workEnd, f.gitReference,
		f.customerApprovalRequired, f.customerReviewRequired,
	}
	if len(dest) != len(vals) {
		panic("fakeChangeRequestDetailRow: dest/vals length mismatch -- update this fake to match scanChangeRequestViewAndDetail's Scan call")
	}
	for i, v := range vals {
		switch d := dest[i].(type) {
		case *string:
			*d = v.(string)
		case **string:
			*d = v.(*string)
		case *time.Time:
			*d = v.(time.Time)
		case **time.Time:
			*d = v.(*time.Time)
		case **bool:
			*d = v.(*bool)
		case *bool:
			*d = v.(bool)
		default:
			panic("fakeChangeRequestDetailRow: unhandled dest type at index")
		}
	}
	return nil
}

func strPtrCR(s string) *string { return &s }

// TestScanChangeRequestViewAndDetail_FieldParityAdditions is the regression
// guard for the "written but never read back" gap this fix closes:
// CreateChangeRequestFromServiceNow already wrote ImplementationPlan/
// Priority/RequestedBy/AffectedServicesText/AffectedComponentsText/
// RollbackDurationText/CustomerGroup (plus several columns nothing writes
// yet -- ChangeRequestType/Likelihood/IsPlanningVisibleToCustomers/
// ConfirmCustomerUpdatedDate/CustomerUpdatedOn/WorkStart/WorkEnd/
// GitReference), but GetChangeRequestByID never selected any of them back.
func TestScanChangeRequestViewAndDetail_FieldParityAdditions(t *testing.T) {
	now := time.Now()

	t.Run("all field-parity columns populated", func(t *testing.T) {
		row := fakeChangeRequestDetailRow{
			id: "CR-1", number: "CHG0001", subject: strPtrCR("s"), description: strPtrCR("d"),
			createdOn: now, updatedOn: now, createdBy: "actor@wso2.com",
			agID: strPtrCR("team-1"), agName: strPtrCR("Devops"),
			implementationPlan: strPtrCR("do the thing"), priority: strPtrCR("HIGH"), category: strPtrCR("SOFTWARE"),
			rbID: strPtrCR("user-1"), rbName: strPtrCR("Jane Doe"),
			affectedServicesText: strPtrCR("svc-a"), affectedComponentsText: strPtrCR("comp-a"), rollbackDurationText: strPtrCR("2h"),
			changeRequestType: strPtrCR("INFRA"), likelihood: strPtrCR("HIGH"),
			isPlanningVisibleToCustomers: boolPtrCR(true),
			confirmCustomerUpdatedDate:   strPtrCR("AGREE"),
			customerUpdatedOn:            &now, workStart: &now, workEnd: &now,
			gitReference: strPtrCR("main@abc123"),
		}
		var cr domain.ChangeRequest
		if err := scanChangeRequestViewAndDetail(row, &cr); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cr.ImplementationPlan == nil || *cr.ImplementationPlan != "do the thing" {
			t.Errorf("ImplementationPlan = %v, want \"do the thing\"", cr.ImplementationPlan)
		}
		if cr.Priority == nil || *cr.Priority != "high" {
			t.Errorf("Priority = %v, want \"high\" (lower-cased)", cr.Priority)
		}
		if cr.Category == nil || *cr.Category != "software" {
			t.Errorf("Category = %v, want \"software\" (lower-cased)", cr.Category)
		}
		if cr.RequestedBy == nil || cr.RequestedBy.ID != "user-1" || cr.RequestedBy.Name != "Jane Doe" {
			t.Errorf("RequestedBy = %+v, want {user-1 Jane Doe}", cr.RequestedBy)
		}
		// Real, reported bug: the CSM Portal's own action bar requires
		// AssignedTeam to be set before Assess can be requested at all, but
		// this repository never selected work_item.assignment_group_id back,
		// so no change request could ever be promoted past New through the
		// portal on this data source, regardless of what ServiceNow itself
		// (or csm-sync-service, mirroring it into Postgres) actually had set.
		if cr.AssignedTeam == nil || cr.AssignedTeam.ID != "team-1" || cr.AssignedTeam.Name != "Devops" {
			t.Errorf("AssignedTeam = %+v, want {team-1 Devops}", cr.AssignedTeam)
		}
		if cr.AffectedServicesText == nil || *cr.AffectedServicesText != "svc-a" {
			t.Errorf("AffectedServicesText = %v, want \"svc-a\"", cr.AffectedServicesText)
		}
		if cr.ChangeRequestType == nil || *cr.ChangeRequestType != "infra" {
			t.Errorf("ChangeRequestType = %v, want \"infra\"", cr.ChangeRequestType)
		}
		if cr.Likelihood == nil || *cr.Likelihood != "high" {
			t.Errorf("Likelihood = %v, want \"high\"", cr.Likelihood)
		}
		if !cr.IsPlanningVisibleToCustomers {
			t.Error("IsPlanningVisibleToCustomers = false, want true")
		}
		if cr.ConfirmCustomerUpdatedDate == nil || *cr.ConfirmCustomerUpdatedDate != "agree" {
			t.Errorf("ConfirmCustomerUpdatedDate = %v, want \"agree\"", cr.ConfirmCustomerUpdatedDate)
		}
		if cr.CustomerUpdatedOn == nil || cr.WorkStart == nil || cr.WorkEnd == nil {
			t.Errorf("CustomerUpdatedOn/WorkStart/WorkEnd unexpectedly nil: %v %v %v", cr.CustomerUpdatedOn, cr.WorkStart, cr.WorkEnd)
		}
		if cr.GitReference == nil || *cr.GitReference != "main@abc123" {
			t.Errorf("GitReference = %v, want \"main@abc123\"", cr.GitReference)
		}
	})

	t.Run("NULL field-parity columns (no RequestedBy, nothing written yet) do not error", func(t *testing.T) {
		// Regression: RequestedBy comes from a LEFT JOIN
		// (changeRequestDetailJoins) that can legitimately match no row,
		// and every other new column is nullable -- none of this should
		// panic the way a non-pointer scan destination would.
		row := fakeChangeRequestDetailRow{
			id: "CR-2", number: "CHG0002", subject: strPtrCR("s"), description: strPtrCR("d"),
			createdOn: now, updatedOn: now, createdBy: "actor@wso2.com",
		}
		var cr domain.ChangeRequest
		if err := scanChangeRequestViewAndDetail(row, &cr); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cr.RequestedBy != nil {
			t.Errorf("RequestedBy = %+v, want nil", cr.RequestedBy)
		}
		if cr.AssignedTeam != nil {
			t.Errorf("AssignedTeam = %+v, want nil (no assignment group set)", cr.AssignedTeam)
		}
		if cr.ImplementationPlan != nil || cr.Priority != nil || cr.GitReference != nil {
			t.Errorf("expected NULL field-parity columns to stay nil, got ImplementationPlan=%v Priority=%v GitReference=%v",
				cr.ImplementationPlan, cr.Priority, cr.GitReference)
		}
	})

	// The creation form's two checkboxes are read back as plain booleans and
	// drive LegalNextStates (Review offers customer_review instead of closed).
	t.Run("customer gate flags are read back and drive legalNextStates", func(t *testing.T) {
		for _, tc := range []struct {
			approval, review bool
			wantLegal        []string
		}{
			{false, false, []string{"closed", "rollback", "canceled"}},
			{true, false, []string{"closed", "rollback", "canceled"}},
			{false, true, []string{"customer_review", "rollback", "canceled"}},
			{true, true, []string{"customer_review", "rollback", "canceled"}},
		} {
			row := fakeChangeRequestDetailRow{
				id: "CR-3", number: "CHG0003", subject: strPtrCR("s"), description: strPtrCR("d"),
				createdOn: now, updatedOn: now, createdBy: "actor@wso2.com", state: strPtrCR("REVIEW"),
				customerApprovalRequired: tc.approval, customerReviewRequired: tc.review,
			}
			var cr domain.ChangeRequest
			if err := scanChangeRequestViewAndDetail(row, &cr); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cr.CustomerApprovalRequired != tc.approval || cr.CustomerReviewRequired != tc.review {
				t.Errorf("CustomerApprovalRequired/CustomerReviewRequired = %v/%v, want %v/%v",
					cr.CustomerApprovalRequired, cr.CustomerReviewRequired, tc.approval, tc.review)
			}
			if strings.Join(cr.LegalNextStates, ",") != strings.Join(tc.wantLegal, ",") {
				t.Errorf("review (approval=%v, review=%v) LegalNextStates = %v, want %v", tc.approval, tc.review, cr.LegalNextStates, tc.wantLegal)
			}
		}
	})
}

func boolPtrCR(b bool) *bool { return &b }

func strPtrApproval(s string) *string { return &s }

// TestChangeRequestApprovalStagePosition covers the positional fallback
// derivation exactly -- position 0/1 are STATIC_GROUP labeled Assess/
// Authorize, everything from position 2 onward is DYNAMIC_CONTACT labeled
// Customer Approval, matching the real SN adapter's own fallback ordinal
// scheme (see changeRequestApprovalStagePosition's own doc comment for why
// this is the fallback only, not the two-hardcoded-sys_id primary path).
func TestChangeRequestApprovalStagePosition(t *testing.T) {
	cases := []struct {
		pos          int
		wantStage    string
		wantApprover domain.ChangeRequestApproverType
	}{
		{0, "Assess", domain.ChangeRequestApproverTypeStaticGroup},
		{1, "Authorize", domain.ChangeRequestApproverTypeStaticGroup},
		{2, "Customer Approval", domain.ChangeRequestApproverTypeDynamicContact},
		{3, "Customer Approval", domain.ChangeRequestApproverTypeDynamicContact},
	}
	for _, c := range cases {
		gotStage, gotApprover := changeRequestApprovalStagePosition(c.pos)
		if gotStage != c.wantStage || gotApprover != c.wantApprover {
			t.Errorf("changeRequestApprovalStagePosition(%d) = (%q, %q), want (%q, %q)",
				c.pos, gotStage, gotApprover, c.wantStage, c.wantApprover)
		}
	}
}

// TestNormalizeChangeRequestApprovalStatus covers the shape the table really
// has since migration 0138 -- approval_stage_approver.state already
// UPPER_SNAKE_CASE (REQUESTED, APPROVED, REJECTED, NOT_REQUESTED, NOT_REQUIRED,
// CANCELLED, NOT_ENTITLED), which must come out unchanged (a second
// normalisation is a no-op) -- plus the lowercase raw ServiceNow values
// (migration 0089's own comment) still tolerated for a row a writer has not
// normalised, the empty/nil -> UNKNOWN cases, and an unrecognized value falling
// back to an uppercased passthrough rather than UNKNOWN --
// domain.ChangeRequestApprover.Status is deliberately an open string, not a
// closed enum.
func TestNormalizeChangeRequestApprovalStatus(t *testing.T) {
	cases := []struct {
		name string
		raw  *string
		want string
	}{
		{"REQUESTED as 0138 stores it", strPtrApproval("REQUESTED"), "REQUESTED"},
		{"APPROVED as 0138 stores it", strPtrApproval("APPROVED"), "APPROVED"},
		{"REJECTED as 0138 stores it", strPtrApproval("REJECTED"), "REJECTED"},
		{"NOT_REQUESTED as 0138 stores it", strPtrApproval("NOT_REQUESTED"), "NOT_REQUESTED"},
		{"NOT_REQUIRED as 0138 stores it", strPtrApproval("NOT_REQUIRED"), "NOT_REQUIRED"},
		{"CANCELLED as 0138 stores it", strPtrApproval("CANCELLED"), "CANCELLED"},
		{"NOT_ENTITLED as 0138 stores it", strPtrApproval("NOT_ENTITLED"), "NOT_ENTITLED"},
		{"requested", strPtrApproval("requested"), "REQUESTED"},
		{"approved", strPtrApproval("approved"), "APPROVED"},
		{"rejected", strPtrApproval("rejected"), "REJECTED"},
		{"not_required", strPtrApproval("not_required"), "NOT_REQUIRED"},
		{"cancelled", strPtrApproval("cancelled"), "CANCELLED"},
		{"no_consensus", strPtrApproval("no_consensus"), "NO_CONSENSUS"},
		{"nil", nil, "UNKNOWN"},
		{"empty", strPtrApproval(""), "UNKNOWN"},
		{"unrecognized value uppercased, not UNKNOWN", strPtrApproval("some_new_state"), "SOME_NEW_STATE"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := normalizeChangeRequestApprovalStatus(c.raw); got != c.want {
				t.Errorf("normalizeChangeRequestApprovalStatus(%v) = %q, want %q", c.raw, got, c.want)
			}
		})
	}
}

// TestBuildChangeRequestApprovals_PositionalLabelsAndFirstResponderWinsStatus
// covers the whole assembly: three stages in created_on order get
// Assess/Authorize/Customer Approval labels regardless of their real
// assignment_group_id, and each stage's status is derived first-responder-
// wins from its own approvers only (a REJECTED in stage 2 must not leak
// into stage 1's APPROVED-only result).
func TestBuildChangeRequestApprovals_PositionalLabelsAndFirstResponderWinsStatus(t *testing.T) {
	stages := []changeRequestApprovalStageRow{
		{id: "stage-1", assignmentGroupName: strPtrApproval("SRE Team")},
		{id: "stage-2", assignmentGroupName: strPtrApproval("Change Board")},
		{id: "stage-3", assignmentGroupName: nil},
	}
	updatedOn := time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC)
	approvers := []changeRequestApprovalApproverRow{
		// appr-1's approverUserID ("user-1") must end up as the domain
		// approver's own ID -- not "appr-1" itself (the junction row's own
		// id) -- see changeRequestApprovalApproversQuery's own doc comment
		// for the real bug this guards against: isMyPendingApproval
		// (webapp) can only ever match a real user id, never a junction
		// row's id.
		{id: "appr-1", stageID: strPtrApproval("stage-1"), approverUserID: strPtrApproval("user-1"), approverName: "Alice", rawStatus: strPtrApproval("APPROVED"), updatedOn: updatedOn},
		{id: "appr-2", stageID: strPtrApproval("stage-2"), approverUserID: strPtrApproval("user-2"), approverName: "Bob", rawStatus: strPtrApproval("REQUESTED"), updatedOn: updatedOn},
		// approverUserID nil (approver_user_id null, or a since-deleted
		// user) -- must fall back to the junction row's own id rather than
		// an empty string.
		{id: "appr-3", stageID: strPtrApproval("stage-2"), approverUserID: nil, approverName: "Carol", rawStatus: strPtrApproval("REJECTED"), updatedOn: updatedOn},
		// stage_id NULL -- must be dropped, not attached to any stage.
		{id: "appr-4", stageID: nil, approverUserID: strPtrApproval("user-4"), approverName: "Orphan", rawStatus: strPtrApproval("REQUESTED"), updatedOn: updatedOn},
	}

	got := buildChangeRequestApprovals(stages, approvers, "NORMAL")

	if len(got.Approvals) != 3 {
		t.Fatalf("got %d approvals, want 3", len(got.Approvals))
	}

	a0 := got.Approvals[0]
	if a0.Stage != "Assess" || a0.ApproverType != domain.ChangeRequestApproverTypeStaticGroup {
		t.Errorf("stage 0 = %q/%q, want Assess/STATIC_GROUP", a0.Stage, a0.ApproverType)
	}
	if a0.ApproverName != "SRE Team" {
		t.Errorf("stage 0 approverName = %q, want %q", a0.ApproverName, "SRE Team")
	}
	if a0.Status != domain.ChangeRequestApprovalStatusApproved {
		t.Errorf("stage 0 status = %q, want APPROVED", a0.Status)
	}
	if len(a0.Approvers) != 1 || a0.Approvers[0].RespondedOn == nil {
		t.Errorf("stage 0 approvers = %+v, want 1 approver with a non-nil RespondedOn", a0.Approvers)
	}
	if got := a0.Approvers[0].ID; got != "user-1" {
		t.Errorf("stage 0 approver ID = %q, want the resolved user id %q, not the junction row's own id", got, "user-1")
	}

	a1 := got.Approvals[1]
	if a1.Stage != "Authorize" || a1.ApproverType != domain.ChangeRequestApproverTypeStaticGroup {
		t.Errorf("stage 1 = %q/%q, want Authorize/STATIC_GROUP", a1.Stage, a1.ApproverType)
	}
	// REJECTED beats REQUESTED regardless of the order the two approvers
	// appear in -- first-responder-wins, not first-in-list-wins.
	if a1.Status != domain.ChangeRequestApprovalStatusRejected {
		t.Errorf("stage 1 status = %q, want REJECTED (a single rejection resolves the stage)", a1.Status)
	}
	if len(a1.Approvers) != 2 {
		t.Fatalf("stage 1 has %d approvers, want 2", len(a1.Approvers))
	}
	for _, ap := range a1.Approvers {
		if ap.Status == "REQUESTED" && ap.RespondedOn != nil {
			t.Errorf("REQUESTED approver %q has non-nil RespondedOn %v, want nil", ap.Name, *ap.RespondedOn)
		}
		switch ap.Name {
		case "Bob":
			if ap.ID != "user-2" {
				t.Errorf("Bob's ID = %q, want the resolved user id %q", ap.ID, "user-2")
			}
		case "Carol":
			// approverUserID was nil for this row -- falls back to the
			// junction row's own id ("appr-3"), never an empty string.
			if ap.ID != "appr-3" {
				t.Errorf("Carol's ID = %q, want the junction row's own id %q (approverUserID was nil)", ap.ID, "appr-3")
			}
		}
	}

	a2 := got.Approvals[2]
	if a2.Stage != "Customer Approval" || a2.ApproverType != domain.ChangeRequestApproverTypeDynamicContact {
		t.Errorf("stage 2 = %q/%q, want Customer Approval/DYNAMIC_CONTACT", a2.Stage, a2.ApproverType)
	}
	if a2.ApproverName != "" {
		t.Errorf("stage 2 approverName = %q, want empty (nil assignment_group_id)", a2.ApproverName)
	}
	if a2.Status != domain.ChangeRequestApprovalStatusPending {
		t.Errorf("stage 2 status = %q, want PENDING (zero approvers)", a2.Status)
	}
	if len(a2.Approvers) != 0 {
		t.Errorf("stage 2 has %d approvers, want 0 (the NULL-stage_id row must be dropped, not attached here)", len(a2.Approvers))
	}
}

// TestBuildChangeRequestApprovals_AssignmentGroup pins the group reference on
// each stage that lets the portal open the group (GET /groups/{id}): an
// internal stage carries {id, name}; a customer stage -- recorded against no
// group, its approvers being the project's registered contacts -- carries
// null, and still names the Customer Group in approverName. It also pins the
// wire shape: `assignmentGroup` is always present, `null` when absent.
func TestBuildChangeRequestApprovals_AssignmentGroup(t *testing.T) {
	peer, cab, customer := "Peer Approval", "CAB Approval", "Customer Approval"
	stages := []changeRequestApprovalStageRow{
		{id: "s-peer", assignmentGroupName: strPtrApproval("Example Corp ABT"), assignmentGroupID: strPtrApproval("11111111-1111-4111-8111-111111111111"), checkpointLabel: &peer},
		{id: "s-cab", assignmentGroupName: strPtrApproval("CAB Approval"), assignmentGroupID: strPtrApproval("22222222-2222-4222-8222-222222222222"), checkpointLabel: &cab},
		{id: "s-cust", assignmentGroupName: nil, assignmentGroupID: nil, checkpointLabel: &customer},
	}

	got := buildChangeRequestApprovals(stages, nil, "NORMAL")

	if len(got.Approvals) != 3 {
		t.Fatalf("got %d approvals, want 3", len(got.Approvals))
	}
	if g := got.Approvals[0].AssignmentGroup; g == nil || g.ID != "11111111-1111-4111-8111-111111111111" || g.Name != "Example Corp ABT" {
		t.Errorf("peer stage assignmentGroup = %+v, want {11111111-..., Example Corp ABT}", g)
	}
	if g := got.Approvals[1].AssignmentGroup; g == nil || g.ID != "22222222-2222-4222-8222-222222222222" || g.Name != "CAB Approval" {
		t.Errorf("CAB stage assignmentGroup = %+v, want {22222222-..., CAB Approval}", g)
	}
	if g := got.Approvals[2].AssignmentGroup; g != nil {
		t.Errorf("customer stage assignmentGroup = %+v, want nil", g)
	}
	// approverName is unchanged: the display name, "Customer Group" for the customer stage.
	if got.Approvals[0].ApproverName != "Example Corp ABT" || got.Approvals[2].ApproverName != "Customer Group" {
		t.Errorf("approverName = %q / %q, want %q / %q", got.Approvals[0].ApproverName, got.Approvals[2].ApproverName, "Example Corp ABT", "Customer Group")
	}

	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var wire struct {
		Approvals []map[string]json.RawMessage `json:"approvals"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for i, a := range wire.Approvals {
		if _, ok := a["assignmentGroup"]; !ok {
			t.Errorf("stage %d has no assignmentGroup key on the wire: %s", i, raw)
		}
	}
	if string(wire.Approvals[2]["assignmentGroup"]) != "null" {
		t.Errorf("customer stage assignmentGroup on the wire = %s, want null", wire.Approvals[2]["assignmentGroup"])
	}
	if string(wire.Approvals[0]["assignmentGroup"]) != `{"id":"11111111-1111-4111-8111-111111111111","name":"Example Corp ABT"}` {
		t.Errorf("peer stage assignmentGroup on the wire = %s", wire.Approvals[0]["assignmentGroup"])
	}
}

// TestBuildChangeRequestApprovals_NoStages covers the zero-stage case:
// GetChangeRequestApprovals must return an empty (not nil-panicking)
// ChangeRequestApprovals when a change request has no approval_stage rows
// yet.
func TestBuildChangeRequestApprovals_NoStages(t *testing.T) {
	got := buildChangeRequestApprovals(nil, nil, "")
	if len(got.Approvals) != 0 {
		t.Errorf("got %d approvals, want 0", len(got.Approvals))
	}
}

// TestLegalChangeRequestNextStates pins the forward graph (see
// changeRequestForwardNextStates's own doc comment for how each edge was
// derived) -- a change to this table changes what a caller is allowed to
// promote a change request to, so a regression here would silently offer or
// withhold a real action. The second column is customer_review_required:
// only Review's offer depends on it.
func TestLegalChangeRequestNextStates(t *testing.T) {
	strPtr := func(s string) *string { return &s }

	tests := []struct {
		state          string
		reviewRequired bool
		want           []string
	}{
		{string(domain.ChangeRequestStateNew), false, []string{"assess", "canceled"}},
		{string(domain.ChangeRequestStateNew), true, []string{"assess", "canceled"}},
		// Assess is an approval wait too: it offers only Cancel, the peer approval
		// moves the change on by itself. (Assess -> Authorize used to be listed,
		// although the PATCH has always refused it.)
		{string(domain.ChangeRequestStateAssess), false, []string{"canceled"}},
		{string(domain.ChangeRequestStateAssess), true, []string{"canceled"}},
		// Authorize is an approval wait (CAB): it offers no forward move
		// at all, only Cancel -- CAB approval moves the change on by
		// itself (to Scheduled, or Customer Approval when the customer's
		// approval is required).
		{string(domain.ChangeRequestStateAuthorize), false, []string{"canceled"}},
		// Customer Approval waits for the CUSTOMER's own approval, which no staff
		// action gives for them, so "scheduled" is NOT offered (from any state).
		// "authorize" there means Re-schedule (the planned time changed: back
		// through internal approval, the customer asked again). It is offered
		// from this state only (and Assess's approval path); Cancel stays.
		{string(domain.ChangeRequestStateCustomerApproval), false, []string{"authorize", "canceled"}},
		{string(domain.ChangeRequestStateCustomerApproval), true, []string{"authorize", "canceled"}},
		{string(domain.ChangeRequestStateScheduled), false, []string{"implement", "canceled"}},
		{string(domain.ChangeRequestStateImplement), false, []string{"review", "canceled"}},
		// Review: Closed directly unless the customer's review is required, in
		// which case Customer Review is the only forward move (then Closed).
		// Rollback (the review failed) is offered either way, after the
		// forward move and before Cancel -- and from these two states only.
		{string(domain.ChangeRequestStateReview), false, []string{"closed", "rollback", "canceled"}},
		{string(domain.ChangeRequestStateReview), true, []string{"customer_review", "rollback", "canceled"}},
		// Customer Review waits for the CUSTOMER's own review: "closed" is NOT
		// offered, only Rollback (the review failed) and Cancel.
		{string(domain.ChangeRequestStateCustomerReview), false, []string{"rollback", "canceled"}},
		{string(domain.ChangeRequestStateCustomerReview), true, []string{"rollback", "canceled"}},
		// Terminal states offer nothing, rollback included.
		{string(domain.ChangeRequestStateRollback), false, nil},
		{string(domain.ChangeRequestStateRollback), true, nil},
		{string(domain.ChangeRequestStateClosed), false, nil},
		{string(domain.ChangeRequestStateCanceled), false, nil},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s/reviewRequired=%v", tc.state, tc.reviewRequired), func(t *testing.T) {
			got := legalChangeRequestNextStates(strPtr(tc.state), tc.reviewRequired)
			if len(got) != len(tc.want) {
				t.Fatalf("legalChangeRequestNextStates(%q, %v) = %v, want %v", tc.state, tc.reviewRequired, got, tc.want)
			}
			for i, want := range tc.want {
				if got[i] != want {
					t.Errorf("legalChangeRequestNextStates(%q, %v)[%d] = %q, want %q", tc.state, tc.reviewRequired, i, got[i], want)
				}
			}
		})
	}

	// The customer's approval and review are the customer's alone: no state
	// offers "scheduled" (the CAB cascade, Request Approval on a Standard
	// change and the customer's own approval are the only ways in), and Customer
	// Review does not offer "closed" (only the customer's review closes it).
	t.Run("scheduled is never offered, from any state", func(t *testing.T) {
		for st := range changeRequestForwardNextStates {
			for _, review := range []bool{false, true} {
				s := string(st)
				for _, next := range legalChangeRequestNextStates(&s, review) {
					if next == string(domain.ChangeRequestStateScheduled) {
						t.Errorf("legalChangeRequestNextStates(%q, %v) offers %q; Scheduled is reached by the approval flow or by the customer's own approval, never by a staff action", s, review, next)
					}
				}
			}
		}
	})

	t.Run("closed is never offered from customer_review", func(t *testing.T) {
		s := string(domain.ChangeRequestStateCustomerReview)
		for _, review := range []bool{false, true} {
			for _, next := range legalChangeRequestNextStates(&s, review) {
				if next == string(domain.ChangeRequestStateClosed) {
					t.Errorf("legalChangeRequestNextStates(customer_review, %v) offers closed; only the customer's own review closes it", review)
				}
			}
		}
	})

	t.Run("a customer state never offers a staff exit that answers for the customer", func(t *testing.T) {
		for _, st := range []domain.ChangeRequestState{domain.ChangeRequestStateCustomerApproval, domain.ChangeRequestStateCustomerReview} {
			s := string(st)
			for _, review := range []bool{false, true} {
				for _, next := range legalChangeRequestNextStates(&s, review) {
					if err := refuseStaffExitFromCustomerState(context.Background(), nil, "wi", strings.ToUpper(s), domain.ChangeRequestState(next)); err != nil {
						t.Errorf("legalChangeRequestNextStates(%q, %v) offers %q, which patchChangeRequestTx refuses: %v", s, review, next, err)
					}
				}
			}
		}
	})

	t.Run("authorize is offered from customer_approval (re-schedule) only, never from assess (the peer approval's)", func(t *testing.T) {
		for _, st := range []domain.ChangeRequestState{
			domain.ChangeRequestStateNew, domain.ChangeRequestStateAssess, domain.ChangeRequestStateAuthorize,
			domain.ChangeRequestStateCustomerApproval, domain.ChangeRequestStateScheduled, domain.ChangeRequestStateImplement,
			domain.ChangeRequestStateReview, domain.ChangeRequestStateCustomerReview,
			domain.ChangeRequestStateRollback, domain.ChangeRequestStateClosed, domain.ChangeRequestStateCanceled,
		} {
			want := st == domain.ChangeRequestStateCustomerApproval
			s := string(st)
			got := false
			for _, next := range legalChangeRequestNextStates(&s, false) {
				if next == string(domain.ChangeRequestStateAuthorize) {
					got = true
				}
			}
			if got != want {
				t.Errorf("legalChangeRequestNextStates(%q) offers authorize = %v, want %v", s, got, want)
			}
		}
	})

	t.Run("rollback is offered from review and customer_review only", func(t *testing.T) {
		for _, st := range []domain.ChangeRequestState{
			domain.ChangeRequestStateNew, domain.ChangeRequestStateAssess, domain.ChangeRequestStateAuthorize,
			domain.ChangeRequestStateCustomerApproval, domain.ChangeRequestStateScheduled, domain.ChangeRequestStateImplement,
			domain.ChangeRequestStateReview, domain.ChangeRequestStateCustomerReview,
			domain.ChangeRequestStateRollback, domain.ChangeRequestStateClosed, domain.ChangeRequestStateCanceled,
		} {
			want := st == domain.ChangeRequestStateReview || st == domain.ChangeRequestStateCustomerReview
			for _, review := range []bool{false, true} {
				s := string(st)
				got := false
				for _, next := range legalChangeRequestNextStates(&s, review) {
					if next == string(domain.ChangeRequestStateRollback) {
						got = true
					}
				}
				if got != want {
					t.Errorf("legalChangeRequestNextStates(%q, %v) offers rollback = %v, want %v", s, review, got, want)
				}
			}
		}
		if got := legalChangeRequestNextStates(nil, true); got != nil {
			t.Errorf("legalChangeRequestNextStates(nil) = %v, want nil", got)
		}
	})

	t.Run("customer_review is offered from review only when required", func(t *testing.T) {
		for st := range changeRequestForwardNextStates {
			s := string(st)
			for _, next := range legalChangeRequestNextStates(&s, false) {
				if next == string(domain.ChangeRequestStateCustomerReview) {
					t.Errorf("legalChangeRequestNextStates(%q, false) offers customer_review although it is not required", s)
				}
			}
			for _, next := range legalChangeRequestNextStates(&s, true) {
				if next == string(domain.ChangeRequestStateClosed) && st == domain.ChangeRequestStateReview {
					t.Errorf("legalChangeRequestNextStates(review, true) offers closed although customer review is required")
				}
			}
		}
	})

	t.Run("terminal states have no legal next state", func(t *testing.T) {
		for _, terminal := range []domain.ChangeRequestState{
			domain.ChangeRequestStateRollback,
			domain.ChangeRequestStateClosed,
			domain.ChangeRequestStateCanceled,
		} {
			s := string(terminal)
			for _, review := range []bool{false, true} {
				if got := legalChangeRequestNextStates(&s, review); got != nil {
					t.Errorf("legalChangeRequestNextStates(%q, %v) = %v, want nil (terminal)", s, review, got)
				}
			}
		}
	})

	t.Run("nil state is nil", func(t *testing.T) {
		if got := legalChangeRequestNextStates(nil, true); got != nil {
			t.Errorf("legalChangeRequestNextStates(nil) = %v, want nil", got)
		}
	})

	t.Run("unrecognized state is nil, not a guess", func(t *testing.T) {
		s := "some_future_state_this_repo_does_not_know_about"
		if got := legalChangeRequestNextStates(&s, true); got != nil {
			t.Errorf("legalChangeRequestNextStates(%q) = %v, want nil", s, got)
		}
	})
}

// TestCustomerGateHelpers pins the pure gate rules: where Request Approval and
// CAB approval land, and until which state each checkbox stays editable.
func TestCustomerGateHelpers(t *testing.T) {
	t.Run("Request Approval destination", func(t *testing.T) {
		for _, tc := range []struct {
			model    string
			required bool
			want     domain.ChangeRequestState
		}{
			{"NORMAL", false, domain.ChangeRequestStateAssess},
			{"NORMAL", true, domain.ChangeRequestStateAssess}, // the gate comes after CAB
			{"EMERGENCY", false, domain.ChangeRequestStateAuthorize},
			{"EMERGENCY", true, domain.ChangeRequestStateAuthorize}, // an Emergency change has no customer gate at all
			{"STANDARD", false, domain.ChangeRequestStateScheduled},
			{"STANDARD", true, domain.ChangeRequestStateCustomerApproval}, // no approval to wait for: gate right away
			{"", true, domain.ChangeRequestStateAssess},                   // legacy/unknown follows Normal
		} {
			if got := requestApprovalDestination(changeRequestFlowForModel(tc.model), tc.required); got != tc.want {
				t.Errorf("requestApprovalDestination(%q, %v) = %q, want %q", tc.model, tc.required, got, tc.want)
			}
		}
	})

	t.Run("CAB approval target", func(t *testing.T) {
		if got := approvalGateTarget(false); got != "SCHEDULED" {
			t.Errorf("approvalGateTarget(false) = %q, want SCHEDULED", got)
		}
		if got := approvalGateTarget(true); got != "CUSTOMER_APPROVAL" {
			t.Errorf("approvalGateTarget(true) = %q, want CUSTOMER_APPROVAL", got)
		}
	})

	t.Run("customerApprovalRequired is editable until the approval gate is passed", func(t *testing.T) {
		for state, want := range map[string]bool{
			"": true, "NEW": true, "ASSESS": true, "AUTHORIZE": true,
			"CUSTOMER_APPROVAL": false, "SCHEDULED": false, "IMPLEMENT": false, "REVIEW": false,
			"CUSTOMER_REVIEW": false, "ROLLBACK": false, "CLOSED": false, "CANCELED": false,
		} {
			if got := approvalRequirementEditable(state); got != want {
				t.Errorf("approvalRequirementEditable(%q) = %v, want %v", state, got, want)
			}
		}
	})

	t.Run("customerReviewRequired is editable until the change leaves review", func(t *testing.T) {
		for state, want := range map[string]bool{
			"": true, "NEW": true, "ASSESS": true, "AUTHORIZE": true, "CUSTOMER_APPROVAL": true,
			"SCHEDULED": true, "IMPLEMENT": true, "REVIEW": true,
			"CUSTOMER_REVIEW": false, "ROLLBACK": false, "CLOSED": false, "CANCELED": false,
		} {
			if got := reviewRequirementEditable(state); got != want {
				t.Errorf("reviewRequirementEditable(%q) = %v, want %v", state, got, want)
			}
		}
	})

	t.Run("an unchanged value is never refused", func(t *testing.T) {
		yes, no := true, false
		snap := changeRequestGateSnapshot{state: "CLOSED", approvalRequired: true, reviewRequired: false}
		if err := validateCustomerGateEdits(snap, &yes, &no); err != nil {
			t.Errorf("resending the stored values after the gates = %v, want nil", err)
		}
		if err := validateCustomerGateEdits(snap, &no, nil); err == nil {
			t.Error("flipping customerApprovalRequired after the gate = nil, want a ValidationError")
		}
		if err := validateCustomerGateEdits(snap, nil, &yes); err == nil {
			t.Error("flipping customerReviewRequired after the gate = nil, want a ValidationError")
		}
	})
}

// TestChangeRequestFlowForModel pins what Request Approval does per change
// type: Normal -> Assess + Peer Approval stage, Emergency -> Authorize + the one
// CAB Approval stage (no peer approval; the previous system has no Emergency CAB),
// Standard -> Scheduled with no stage at all. A NULL/legacy type follows the
// Normal flow.
func TestChangeRequestFlowForModel(t *testing.T) {
	tests := []struct {
		model     string
		wantState domain.ChangeRequestState
		wantStage string // "" = no approval stage
	}{
		{"NORMAL", domain.ChangeRequestStateAssess, "Peer Approval"},
		{"EMERGENCY", domain.ChangeRequestStateAuthorize, "CAB Approval"},
		{"STANDARD", domain.ChangeRequestStateScheduled, ""},
		{"", domain.ChangeRequestStateAssess, "Peer Approval"},
		{"AZURE", domain.ChangeRequestStateAssess, "Peer Approval"},
	}
	for _, tc := range tests {
		t.Run(tc.model, func(t *testing.T) {
			got := changeRequestFlowForModel(tc.model)
			if got.requestState != tc.wantState {
				t.Fatalf("requestState = %q, want %q", got.requestState, tc.wantState)
			}
			if tc.wantStage == "" {
				if got.checkpoint != nil {
					t.Fatalf("checkpoint = %+v, want none (no approval)", got.checkpoint)
				}
				return
			}
			if got.checkpoint == nil || got.checkpoint.Label != tc.wantStage {
				t.Fatalf("checkpoint = %+v, want label %q", got.checkpoint, tc.wantStage)
			}
		})
	}
	// Emergency has no peer stage, so its CAB stage sits first; Normal's CAB
	// stage sits right after the peer stage.
	if changeRequestEmergencyCABCheckpoint.Position != 0 || changeRequestPeerCheckpoint.Position != 0 || changeRequestCABCheckpoint.Position != 1 {
		t.Fatalf("checkpoint positions: peer=%d cab=%d emergency cab=%d, want 0/1/0",
			changeRequestPeerCheckpoint.Position, changeRequestCABCheckpoint.Position, changeRequestEmergencyCABCheckpoint.Position)
	}
	// There is no Emergency CAB: both CAB checkpoints are the same stage of the same
	// group (the label, the group and the pool), differing only in where they sit.
	if changeRequestEmergencyCABCheckpoint.Label != changeRequestCABCheckpoint.Label ||
		changeRequestEmergencyCABCheckpoint.GroupName != changeRequestCABCheckpoint.GroupName ||
		changeRequestEmergencyCABCheckpoint.Pool != changeRequestCABCheckpoint.Pool {
		t.Fatalf("Emergency CAB checkpoint %+v must be the CAB checkpoint %+v at position 0", changeRequestEmergencyCABCheckpoint, changeRequestCABCheckpoint)
	}
	if changeRequestCABCheckpoint.Label != "CAB Approval" || changeRequestCABCheckpoint.GroupName != domain.CABApprovalGroupName {
		t.Fatalf("CAB checkpoint = %+v, want the CAB Approval label and group", changeRequestCABCheckpoint)
	}
}

func TestClassifyApprovalStage(t *testing.T) {
	str := func(s string) *string { return &s }
	tests := []struct {
		label *string
		pos   int
		want  approvalStageKind
	}{
		{str("Peer Approval"), 0, stageKindPeer},
		{str("CAB Approval"), 1, stageKindCAB},
		// A stage an earlier build wrote for an Emergency change: no longer written, still
		// recognised, and decided as the CAB stage.
		{str("ECAB Approval"), 0, stageKindCAB},
		{str("Review"), 2, stageKindReview},
		// The customer group's stages are recognised by label, wherever they sit.
		{str("Customer Approval"), 2, stageKindCustomerApproval},
		{str("Customer Approval"), 0, stageKindCustomerApproval},
		{str("Customer Review"), 4, stageKindCustomerReview},
		// Stages written before the CAB flow keep working.
		{str("Assess"), 0, stageKindPeer},
		{str("Authorize"), 1, stageKindCAB},
		// No label: the historical positional convention.
		{nil, 0, stageKindPeer},
		{nil, 1, stageKindCAB},
		{nil, 2, stageKindOther},
		{str("something else"), 0, stageKindOther},
	}
	for _, tc := range tests {
		if got := classifyApprovalStage(tc.label, tc.pos); got != tc.want {
			t.Errorf("classifyApprovalStage(%v, %d) = %v, want %v", tc.label, tc.pos, got, tc.want)
		}
	}
}

// TestValidateCreateChangeRequestType: type is mandatory on create and must be
// Standard, Normal or Emergency.
func TestValidateCreateChangeRequestType(t *testing.T) {
	typ := func(s string) *domain.ChangeRequestType { v := domain.ChangeRequestType(s); return &v }
	for _, ok := range []string{"standard", "normal", "emergency"} {
		if err := ValidateCreateChangeRequestType(typ(ok)); err != nil {
			t.Errorf("ValidateCreateChangeRequestType(%q) = %v, want nil", ok, err)
		}
	}
	for name, bad := range map[string]*domain.ChangeRequestType{
		"missing": nil, "empty": typ(""), "azure": typ("azure"), "model": typ("model"), "bogus": typ("bogus"),
	} {
		var ve *apierror.ValidationError
		if err := ValidateCreateChangeRequestType(bad); !errors.As(err, &ve) {
			t.Errorf("ValidateCreateChangeRequestType(%s) = %v, want *apierror.ValidationError", name, err)
		}
	}
	var ve *apierror.ValidationError
	err := ValidateCreateChangeRequestType(nil)
	if !errors.As(err, &ve) || !strings.Contains(ve.Msg, "standard, normal or emergency") {
		t.Errorf("missing-type message = %v, want it to list standard, normal or emergency", err)
	}
}

// The customer stages: which state each belongs to, where each outcome leads,
// and what legalNextStates drops while one is live.
func TestCustomerStageSpecs(t *testing.T) {
	ca := customerStageSpecForState("CUSTOMER_APPROVAL")
	cr := customerStageSpecForState("CUSTOMER_REVIEW")
	if ca == nil || cr == nil {
		t.Fatal("no spec for CUSTOMER_APPROVAL / CUSTOMER_REVIEW")
	}
	for _, st := range []string{"", "NEW", "ASSESS", "AUTHORIZE", "SCHEDULED", "IMPLEMENT", "REVIEW", "ROLLBACK", "CLOSED", "CANCELED"} {
		if customerStageSpecForState(st) != nil {
			t.Errorf("customerStageSpecForState(%q) != nil", st)
		}
	}
	if ca.label != "Customer Approval" || ca.approvedState != "SCHEDULED" || ca.rejectedState != "CANCELED" || ca.approvedFlagColumn != "is_customer_approval_required" {
		t.Errorf("Customer Approval spec = %+v", *ca)
	}
	if cr.label != "Customer Review" || cr.approvedState != "CLOSED" || cr.rejectedState != "ROLLBACK" || cr.approvedFlagColumn != "is_customer_review_required" {
		t.Errorf("Customer Review spec = %+v", *cr)
	}
	if customerStageSpecForKind(stageKindCustomerApproval) != ca || customerStageSpecForKind(stageKindCustomerReview) != cr || customerStageSpecForKind(stageKindPeer) != nil {
		t.Error("customerStageSpecForKind does not agree with customerStageSpecForState")
	}
}

// withoutStaffRollbackWhileCustomerReviewPending is the only live-stage filter
// left on legalNextStates: Rollback out of Customer Review while the customer
// group's review request is pending. The customer's own approval / review are
// never in the base graph, so there is nothing else to drop.
func TestWithoutStaffRollbackWhileCustomerReviewPending(t *testing.T) {
	str := func(s string) *string { return &s }
	for _, tc := range []struct {
		name  string
		state *string
		in    []string
		live  bool
		want  []string
	}{
		{"customer_approval, live: Re-schedule and Cancel stay", str("CUSTOMER_APPROVAL"), []string{"authorize", "canceled"}, true, []string{"authorize", "canceled"}},
		{"customer_approval, not live", str("CUSTOMER_APPROVAL"), []string{"authorize", "canceled"}, false, []string{"authorize", "canceled"}},
		{"customer_review, live: only Cancel", str("CUSTOMER_REVIEW"), []string{"rollback", "canceled"}, true, []string{"canceled"}},
		{"customer_review, live, lower case", str("customer_review"), []string{"rollback", "canceled"}, true, []string{"canceled"}},
		{"customer_review, not live: Rollback and Cancel", str("CUSTOMER_REVIEW"), []string{"rollback", "canceled"}, false, []string{"rollback", "canceled"}},
		{"review is untouched even with a live customer stage", str("REVIEW"), []string{"closed", "rollback", "canceled"}, true, []string{"closed", "rollback", "canceled"}},
		{"nil states", str("CUSTOMER_REVIEW"), nil, true, nil},
		{"nil state", nil, []string{"rollback"}, true, []string{"rollback"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := withoutStaffRollbackWhileCustomerReviewPending(tc.state, tc.in, tc.live)
			if strings.Join(got, ",") != strings.Join(tc.want, ",") || (got == nil) != (tc.want == nil) {
				t.Errorf("withoutStaffRollbackWhileCustomerReviewPending = %v, want %v", got, tc.want)
			}
		})
	}
}

// The compliance rule at the unit level: nobody but the customer records the
// customer's approval or review. A staff request that carries either flag is
// refused outright (true or false, alone or with anything else), and out of a
// customer state only the exits that answer nothing for the customer pass.
func TestRefuseStaffCustomerOutcomeFlags(t *testing.T) {
	yes, no := true, false
	for _, tc := range []struct {
		name string
		req  domain.PatchChangeRequestRequest
		want string
	}{
		{"isCustomerApproved true", domain.PatchChangeRequestRequest{IsCustomerApproved: &yes}, "isCustomerApproved cannot be set on the customer's behalf: the customer's approval can only be given by the customer in the Customer Portal"},
		{"isCustomerApproved false", domain.PatchChangeRequestRequest{IsCustomerApproved: &no}, "isCustomerApproved cannot be set on the customer's behalf: the customer's approval can only be given by the customer in the Customer Portal"},
		{"isCustomerReviewed true", domain.PatchChangeRequestRequest{IsCustomerReviewed: &yes}, "isCustomerReviewed cannot be set on the customer's behalf: the customer's review can only be given by the customer in the Customer Portal"},
		{"isCustomerReviewed false", domain.PatchChangeRequestRequest{IsCustomerReviewed: &no}, "isCustomerReviewed cannot be set on the customer's behalf: the customer's review can only be given by the customer in the Customer Portal"},
		{"with a state", domain.PatchChangeRequestRequest{State: ptrState(domain.ChangeRequestStateScheduled), IsCustomerApproved: &yes}, "isCustomerApproved cannot be set on the customer's behalf: the customer's approval can only be given by the customer in the Customer Portal"},
		{"both: approval is named first", domain.PatchChangeRequestRequest{IsCustomerApproved: &yes, IsCustomerReviewed: &yes}, "isCustomerApproved cannot be set on the customer's behalf: the customer's approval can only be given by the customer in the Customer Portal"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var ve *apierror.ValidationError
			if err := refuseStaffCustomerOutcomeFlags(tc.req); !errors.As(err, &ve) || ve.Msg != tc.want {
				t.Fatalf("err = %v, want a 400 saying %q", err, tc.want)
			}
		})
	}
	if err := refuseStaffCustomerOutcomeFlags(domain.PatchChangeRequestRequest{State: ptrState(domain.ChangeRequestStateCanceled)}); err != nil {
		t.Errorf("a request without either flag is not refused here: %v", err)
	}
}

func ptrState(s domain.ChangeRequestState) *domain.ChangeRequestState { return &s }

func TestRefuseStaffExitFromCustomerState(t *testing.T) {
	ctx := context.Background()
	const wantApproval = `state "scheduled" cannot be set manually from customer_approval: the customer's approval can only be given by the customer in the Customer Portal; cancel the change or re-schedule it instead`
	// Out of Customer Approval: Cancel and Re-schedule (authorize) pass, as does
	// staying where it is (a resent Request Approval resolves to the state
	// itself); every other destination -- the customer's own outcome first, then
	// the ones that would skip the customer altogether -- is refused.
	for _, ok := range []domain.ChangeRequestState{"canceled", "authorize", "customer_approval", "CANCELED"} {
		if err := refuseStaffExitFromCustomerState(ctx, nil, "wi", "CUSTOMER_APPROVAL", ok); err != nil {
			t.Errorf("customer_approval -> %q was refused: %v", ok, err)
		}
	}
	for _, refused := range []domain.ChangeRequestState{"scheduled", "SCHEDULED", "implement", "review", "customer_review", "closed", "rollback", "assess", "new"} {
		var ve *apierror.ValidationError
		err := refuseStaffExitFromCustomerState(ctx, nil, "wi", "CUSTOMER_APPROVAL", refused)
		if !errors.As(err, &ve) {
			t.Errorf("customer_approval -> %q was accepted", refused)
			continue
		}
		want := strings.Replace(wantApproval, `"scheduled"`, fmt.Sprintf("%q", strings.ToLower(string(refused))), 1)
		if ve.Msg != want {
			t.Errorf("customer_approval -> %q: message %q, want %q", refused, ve.Msg, want)
		}
	}
	// Out of Customer Review: Cancel and Rollback pass, as does staying.
	for _, ok := range []domain.ChangeRequestState{"canceled", "rollback", "customer_review"} {
		if err := refuseStaffExitFromCustomerState(ctx, nil, "wi", "CUSTOMER_REVIEW", ok); err != nil {
			t.Errorf("customer_review -> %q was refused: %v", ok, err)
		}
	}
	// Every other state is not a customer state: nothing here is judged.
	for _, st := range []string{"", "NEW", "ASSESS", "AUTHORIZE", "SCHEDULED", "IMPLEMENT", "REVIEW", "ROLLBACK", "CLOSED", "CANCELED"} {
		if err := refuseStaffExitFromCustomerState(ctx, nil, "wi", st, "scheduled"); err != nil {
			t.Errorf("%q -> scheduled was judged by the customer-state guard: %v", st, err)
		}
	}
}

// customerStageManualRefusal is what a manual Rollback out of Customer Review is
// refused with while the customer group's review request is pending (a failed
// review is theirs to give).
func TestCustomerStageManualRefusal(t *testing.T) {
	err := customerStageManualRefusal("rollback", &customerReviewStageSpec, &liveCustomerStage{})
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want *apierror.ValidationError", err)
	}
	for _, want := range []string{`"rollback"`, "customer's review", "customer group (the registered contacts of the change request's project)", "approving or rejecting", "approvals"} {
		if !strings.Contains(ve.Msg, want) {
			t.Errorf("message %q does not contain %q", ve.Msg, want)
		}
	}
}

// customerGroupId and environmentIds are refused with a clear message, the
// group first; a request carrying neither passes.
func TestRejectRemovedChangeRequestFields(t *testing.T) {
	const group = "customerGroupId is no longer accepted: the customer group is derived from the customer project's registered contacts"
	const env = "environmentIds is no longer supported: deployments carry the environment"
	for _, tc := range []struct {
		name       string
		group, env bool
		want       string
	}{
		{"neither", false, false, ""},
		{"group", true, false, group},
		{"environments", false, true, env},
		{"both: the group is named first", true, true, group},
	} {
		err := rejectRemovedChangeRequestFields(tc.group, tc.env)
		if tc.want == "" {
			if err != nil {
				t.Errorf("%s: err = %v, want nil", tc.name, err)
			}
			continue
		}
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) || ve.Msg != tc.want {
			t.Errorf("%s: err = %v, want ValidationError %q", tc.name, err, tc.want)
		}
	}
	// The struct entry points see an explicit null / empty array too.
	var none *string
	if err := RejectRemovedPatchFields(domain.PatchChangeRequestRequest{CustomerGroupID: &none}); err == nil {
		t.Error("an explicit null customerGroupId was accepted")
	}
	if err := RejectRemovedPatchFields(domain.PatchChangeRequestRequest{EnvironmentIDs: &[]string{}}); err == nil {
		t.Error("an empty environmentIds was accepted")
	}
	if err := RejectRemovedCreateFields(domain.CreateChangeRequestRequest{EnvironmentIDs: []string{}}); err == nil {
		t.Error("an empty environmentIds was accepted on create")
	}
	if err := RejectRemovedCreateFields(domain.CreateChangeRequestRequest{}); err != nil {
		t.Errorf("an empty create request was refused: %v", err)
	}
}

// The customer stages are recognised by their checkpoint label; nothing else is.
func TestCustomerStageSpecForLabel(t *testing.T) {
	if customerStageSpecForLabel("Customer Approval") != &customerApprovalStageSpec || customerStageSpecForLabel("Customer Review") != &customerReviewStageSpec {
		t.Error("the customer stage labels are not recognised")
	}
	for _, label := range []string{"", "Peer Approval", "CAB Approval", "Review", "customer approval"} {
		if customerStageSpecForLabel(label) != nil {
			t.Errorf("%q recognised as a customer stage", label)
		}
	}
}

// TestChangeRequestLinkHelpers covers the pure helpers behind the
// customer-scope rules (change_request_links.go): id lists are normalised
// (trimmed, lower-cased, de-duplicated, blanks dropped, order kept) and
// compared as sets.
func TestChangeRequestLinkHelpers(t *testing.T) {
	got := normalizeUUIDList([]string{" AAAA ", "aaaa", "", "bbbb", "  "})
	if len(got) != 2 || got[0] != "aaaa" || got[1] != "bbbb" {
		t.Fatalf("normalizeUUIDList = %v, want [aaaa bbbb]", got)
	}
	for _, tc := range []struct {
		a, b []string
		want bool
	}{
		{nil, nil, true},
		{[]string{"a", "b"}, []string{"B", "a"}, true},
		{[]string{"a"}, []string{"a", "b"}, false},
		{[]string{"a", "b"}, []string{"a", "c"}, false},
	} {
		if sameIDSet(tc.a, tc.b) != tc.want {
			t.Errorf("sameIDSet(%v, %v) = %v, want %v", tc.a, tc.b, !tc.want, tc.want)
		}
	}
	if firstOrNil(nil) != nil || *firstOrNil([]string{"x", "y"}) != "x" {
		t.Error("firstOrNil")
	}
	for _, locked := range []string{"IMPLEMENT", "REVIEW", "CUSTOMER_REVIEW", "ROLLBACK", "CLOSED", "CANCELED"} {
		if !changeRequestLinksLockedStates[locked] {
			t.Errorf("%s should close the edit window", locked)
		}
	}
	for _, open := range []string{"NEW", "ASSESS", "AUTHORIZE", "CUSTOMER_APPROVAL", "SCHEDULED"} {
		if changeRequestLinksLockedStates[open] {
			t.Errorf("%s should be inside the edit window", open)
		}
	}
	// Every category the API enum offers has an enum label on Postgres.
	for _, c := range []domain.ChangeRequestCategory{
		domain.ChangeRequestCategoryHardware, domain.ChangeRequestCategorySoftware, domain.ChangeRequestCategoryService,
		domain.ChangeRequestCategorySystemSoftware, domain.ChangeRequestCategoryApplicationsSoftware, domain.ChangeRequestCategoryNetwork,
		domain.ChangeRequestCategoryTelecom, domain.ChangeRequestCategoryDocumentation, domain.ChangeRequestCategoryOther,
		domain.ChangeRequestCategoryRegularReleaseCloud, domain.ChangeRequestCategoryHotfixReleaseCloud,
		domain.ChangeRequestCategoryDevOps, domain.ChangeRequestCategoryCloudComputing,
	} {
		if !changeRequestCategoryPGLabels[strings.ToUpper(string(c))] {
			t.Errorf("category %s has no Postgres enum label", c)
		}
	}
}

// ---------------------------------------------------------------------------
// Internal-only approver pools (no database): the eligibility test and the
// decision-time guard, against a fake crQuerier.
// ---------------------------------------------------------------------------

// fakeApproverRows is a pgx.Rows over a list of single-column text values.
type fakeApproverRows struct {
	vals []string
	i    int
}

func (r *fakeApproverRows) Close()                                       {}
func (r *fakeApproverRows) Err() error                                   { return nil }
func (r *fakeApproverRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *fakeApproverRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *fakeApproverRows) Values() ([]any, error)                       { return nil, nil }
func (r *fakeApproverRows) RawValues() [][]byte                          { return nil }
func (r *fakeApproverRows) Conn() *pgx.Conn                              { return nil }
func (r *fakeApproverRows) TypeMap() *pgtype.Map                         { return nil }
func (r *fakeApproverRows) Next() bool {
	r.i++
	return r.i <= len(r.vals)
}
func (r *fakeApproverRows) Scan(dest ...any) error {
	*(dest[0].(*string)) = r.vals[r.i-1]
	return nil
}

// fakeApproverQuerier answers the "user" eligibility query from internal (the
// ids whose user_type is INTERNAL and who are active) and counts every query so
// a test can prove a rule did not look at the database at all.
type fakeApproverQuerier struct {
	internal map[string]bool
	queries  int
	err      error
}

func (f *fakeApproverQuerier) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	f.queries++
	if f.err != nil {
		return nil, f.err
	}
	if !strings.Contains(sql, `FROM "user" u`) {
		panic("unexpected query: " + sql)
	}
	var out []string
	for _, id := range args[0].([]string) {
		if f.internal[id] {
			out = append(out, id)
		}
	}
	return &fakeApproverRows{vals: out}, nil
}

func (f *fakeApproverQuerier) QueryRow(context.Context, string, ...any) pgx.Row {
	panic("unexpected QueryRow")
}

func TestStageKindNeedsInternalApprover(t *testing.T) {
	for kind, want := range map[approvalStageKind]bool{
		stageKindPeer: true, stageKindCAB: true, stageKindReview: true,
		// The customer's stages are decided by the project's (external) contacts;
		// an unclassified stage is not ours to judge.
		stageKindCustomerApproval: false, stageKindCustomerReview: false, stageKindOther: false,
	} {
		if got := stageKindNeedsInternalApprover(kind); got != want {
			t.Errorf("stageKindNeedsInternalApprover(%v) = %v, want %v", kind, got, want)
		}
	}
}

func TestInternalApproverIDsAndOnlyInternalApprovers(t *testing.T) {
	const a, b, c = "AAAAAAAA-0000-0000-0000-000000000001", "bbbbbbbb-0000-0000-0000-000000000002", "cccccccc-0000-0000-0000-000000000003"
	q := &fakeApproverQuerier{internal: map[string]bool{a: true, c: true}}

	got, err := internalApproverIDs(context.Background(), q, []string{a, b, c})
	if err != nil {
		t.Fatalf("internalApproverIDs: %v", err)
	}
	if len(got) != 2 || !got[strings.ToLower(a)] || !got[c] || got[b] {
		t.Errorf("internalApproverIDs = %v, want a and c (lower-cased) only", got)
	}
	// Order is preserved and the external member is dropped.
	kept, err := onlyInternalApprovers(context.Background(), q, []string{c, b, a})
	if err != nil {
		t.Fatalf("onlyInternalApprovers: %v", err)
	}
	if strings.Join(kept, ",") != c+","+a {
		t.Errorf("onlyInternalApprovers = %v, want [c a]", kept)
	}
	// No members: no query at all.
	before := q.queries
	if got, _ := internalApproverIDs(context.Background(), q, nil); len(got) != 0 || q.queries != before {
		t.Errorf("internalApproverIDs(nil) = %v after %d queries, want empty and no query", got, q.queries-before)
	}
	// A database error is returned, not swallowed into "nobody is internal".
	if _, err := internalApproverIDs(context.Background(), &fakeApproverQuerier{err: errors.New("boom")}, []string{a}); err == nil {
		t.Error("internalApproverIDs swallowed a query error")
	}
}

func TestApproverDecisionBlock_InternalOnly(t *testing.T) {
	const internal, external, creator = "11111111-0000-0000-0000-000000000001", "22222222-0000-0000-0000-000000000002", "33333333-0000-0000-0000-000000000003"
	q := &fakeApproverQuerier{internal: map[string]bool{internal: true, creator: true}}
	ctx := context.Background()
	creators := map[string]bool{creator: true}

	// An internal user decides every internal stage.
	for _, kind := range []approvalStageKind{stageKindPeer, stageKindCAB, stageKindReview} {
		if err := approverDecisionBlock(ctx, q, internal, creators, kind); err != nil {
			t.Errorf("internal user on %s: %v, want nil", stageKindName(kind), err)
		}
	}
	// An external user is refused on every one of them, with a readable 403.
	for _, kind := range []approvalStageKind{stageKindPeer, stageKindCAB, stageKindReview} {
		err := approverDecisionBlock(ctx, q, external, creators, kind)
		var fe *apierror.ForbiddenError
		if !errors.As(err, &fe) {
			t.Fatalf("external user on %s: err = %v (%T), want *apierror.ForbiddenError", stageKindName(kind), err, err)
		}
		if !strings.Contains(fe.Msg, "internal") || !strings.Contains(fe.Msg, stageKindName(kind)) {
			t.Errorf("external user on %s: message %q should name the internal-only rule and the stage", stageKindName(kind), fe.Msg)
		}
	}
	// The customer stages (and an unclassified one) are not internal-only, and
	// the rule does not even look the user up there.
	before := q.queries
	for _, kind := range []approvalStageKind{stageKindCustomerApproval, stageKindCustomerReview, stageKindOther} {
		if err := approverDecisionBlock(ctx, q, external, creators, kind); err != nil {
			t.Errorf("external user on a non-internal stage kind %v: %v, want nil", kind, err)
		}
	}
	if q.queries != before {
		t.Errorf("customer stages ran %d user queries, want none", q.queries-before)
	}
	// The creator rule still comes first, whoever the creator is.
	err := approverDecisionBlock(ctx, q, creator, creators, stageKindPeer)
	var fe *apierror.ForbiddenError
	if !errors.As(err, &fe) || !strings.Contains(fe.Msg, "creator") {
		t.Errorf("creator on the peer stage: err = %v, want the creator refusal", err)
	}
	// A failing lookup fails the decision rather than letting it through.
	if err := approverDecisionBlock(ctx, &fakeApproverQuerier{err: errors.New("boom")}, internal, creators, stageKindCAB); err == nil {
		t.Error("a failed user lookup let the decision through")
	}
}

func TestNoInternalMembersMessage(t *testing.T) {
	msg := noInternalMembersMessage(`the "CAB Approval" group`, "CAB Approval")
	for _, want := range []string{`"CAB Approval" group`, "no active internal", "external/customer users", "CAB Approval approvers"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message %q should contain %q", msg, want)
		}
	}
}

// TestApprovalStageDecidableState pins the stage -> state map: the one state in
// which a stage of each kind can be decided. An unknown kind has none.
func TestApprovalStageDecidableState(t *testing.T) {
	for kind, want := range map[approvalStageKind]string{
		stageKindPeer:             "ASSESS",
		stageKindCAB:              "AUTHORIZE",
		stageKindReview:           "REVIEW",
		stageKindCustomerApproval: "CUSTOMER_APPROVAL",
		stageKindCustomerReview:   "CUSTOMER_REVIEW",
		stageKindOther:            "",
	} {
		if got := approvalStageDecidableState(kind); got != want {
			t.Errorf("approvalStageDecidableState(%v) = %q, want %q", kind, got, want)
		}
	}
	// Every decidable state is a state the enum has, and each customer stage's
	// own spec agrees with the map.
	for _, kind := range []approvalStageKind{stageKindPeer, stageKindCAB, stageKindReview, stageKindCustomerApproval, stageKindCustomerReview} {
		if !knownChangeRequestStates[approvalStageDecidableState(kind)] {
			t.Errorf("decidable state %q of kind %v is not a known change request state", approvalStageDecidableState(kind), kind)
		}
		if spec := customerStageSpecForKind(kind); spec != nil && spec.state != approvalStageDecidableState(kind) {
			t.Errorf("customer stage spec state %q != decidable state %q", spec.state, approvalStageDecidableState(kind))
		}
	}
}

// The map is keyed through the same classifier the read and decision paths use:
// the explicit label first, the historical position second.
func TestApprovalStageDecidableState_ThroughClassifier(t *testing.T) {
	str := func(s string) *string { return &s }
	for _, tc := range []struct {
		label *string
		pos   int
		want  string
	}{
		{str("Peer Approval"), 0, "ASSESS"},
		{str("Assess"), 0, "ASSESS"},
		{str("CAB Approval"), 1, "AUTHORIZE"},
		{str("Authorize"), 1, "AUTHORIZE"},
		// The label an earlier build wrote for an Emergency change: decided as CAB.
		{str("ECAB Approval"), 0, "AUTHORIZE"},
		{str("Review"), 2, "REVIEW"},
		{str("Customer Approval"), 2, "CUSTOMER_APPROVAL"},
		{str("Customer Review"), 4, "CUSTOMER_REVIEW"},
		{nil, 0, "ASSESS"},
		{nil, 1, "AUTHORIZE"},
		// Not a stage the flow knows: never tied to a state.
		{nil, 2, ""},
		{nil, 5, ""},
		{str("SN Change Approval"), 0, ""},
		{str(""), 3, ""},
	} {
		if got := approvalStageDecidableState(classifyApprovalStage(tc.label, tc.pos)); got != tc.want {
			t.Errorf("decidable state of (%v, %d) = %q, want %q", tc.label, tc.pos, got, tc.want)
		}
	}
}

// approvalStageOutOfState over every kind and every state: guarded exactly when
// the kind has a state, the change's state is a known one, and they differ.
func TestApprovalStageOutOfState(t *testing.T) {
	states := []string{"NEW", "ASSESS", "AUTHORIZE", "CUSTOMER_APPROVAL", "SCHEDULED", "IMPLEMENT", "REVIEW", "CUSTOMER_REVIEW", "ROLLBACK", "CLOSED", "CANCELED"}
	for _, kind := range []approvalStageKind{stageKindPeer, stageKindCAB, stageKindReview, stageKindCustomerApproval, stageKindCustomerReview} {
		decidable := approvalStageDecidableState(kind)
		for _, state := range states {
			if got, want := approvalStageOutOfState(kind, state), state != decidable; got != want {
				t.Errorf("approvalStageOutOfState(%v, %s) = %v, want %v", kind, state, got, want)
			}
		}
		// Case and padding are normalised, so a state read back in any shape works.
		if approvalStageOutOfState(kind, " "+strings.ToLower(decidable)+" ") {
			t.Errorf("approvalStageOutOfState(%v, %q) = true for the stage's own state", kind, strings.ToLower(decidable))
		}
		// A NULL / unknown state is never guarded (ServiceNow-synced data).
		for _, state := range []string{"", "   ", "ON_HOLD", "garbage"} {
			if approvalStageOutOfState(kind, state) {
				t.Errorf("approvalStageOutOfState(%v, %q) = true, want false for an unknown state", kind, state)
			}
		}
	}
	// An unknown kind is never guarded, in any state.
	for _, state := range append(states, "", "garbage") {
		if approvalStageOutOfState(stageKindOther, state) {
			t.Errorf("approvalStageOutOfState(other, %q) = true, want false", state)
		}
	}
}

// knownChangeRequestStates is the enum: every domain state is in it, and the
// terminal ones are exactly Closed, Canceled and Rollback.
func TestKnownChangeRequestStates(t *testing.T) {
	all := []domain.ChangeRequestState{
		domain.ChangeRequestStateNew, domain.ChangeRequestStateAssess, domain.ChangeRequestStateAuthorize,
		domain.ChangeRequestStateCustomerApproval, domain.ChangeRequestStateScheduled, domain.ChangeRequestStateImplement,
		domain.ChangeRequestStateReview, domain.ChangeRequestStateCustomerReview, domain.ChangeRequestStateRollback,
		domain.ChangeRequestStateClosed, domain.ChangeRequestStateCanceled,
	}
	if len(knownChangeRequestStates) != len(all) {
		t.Errorf("knownChangeRequestStates has %d entries, want %d", len(knownChangeRequestStates), len(all))
	}
	terminal := 0
	for _, s := range all {
		label := strings.ToUpper(string(s))
		if !knownChangeRequestStates[label] {
			t.Errorf("state %s missing from knownChangeRequestStates", label)
		}
		if terminalChangeRequestState(label) {
			terminal++
			if _, ok := changeRequestForwardNextStates[s]; ok {
				t.Errorf("%s is terminal but has forward moves", label)
			}
		}
	}
	if terminal != 3 {
		t.Errorf("%d terminal states, want Closed, Canceled and Rollback", terminal)
	}
	if terminalChangeRequestState("") || terminalChangeRequestState("REVIEW") {
		t.Error("an empty or non-final state reads as terminal")
	}
}

func TestChangeRequestStateDisplayName(t *testing.T) {
	for in, want := range map[string]string{
		"CLOSED": "Closed", "CUSTOMER_REVIEW": "Customer Review", "CUSTOMER_APPROVAL": "Customer Approval",
		"ASSESS": "Assess", "": "",
	} {
		if got := changeRequestStateDisplayName(in); got != want {
			t.Errorf("changeRequestStateDisplayName(%q) = %q, want %q", in, got, want)
		}
	}
}

// The refusal is a 409 that says where the change is and where the stage can be
// decided -- the exact text the portal shows.
func TestStaleApprovalRefusal(t *testing.T) {
	for _, tc := range []struct {
		kind  approvalStageKind
		state string
		want  string
	}{
		{stageKindReview, "CLOSED", "this approval is no longer pending: the change request is in Closed, but the Review stage can only be decided while it is in Review"},
		{stageKindReview, "CUSTOMER_REVIEW", "this approval is no longer pending: the change request is in Customer Review, but the Review stage can only be decided while it is in Review"},
		{stageKindPeer, "AUTHORIZE", "this approval is no longer pending: the change request is in Authorize, but the Peer Approval stage can only be decided while it is in Assess"},
		{stageKindCAB, "SCHEDULED", "this approval is no longer pending: the change request is in Scheduled, but the CAB Approval stage can only be decided while it is in Authorize"},
		{stageKindCustomerApproval, "AUTHORIZE", "this approval is no longer pending: the change request is in Authorize, but the Customer Approval stage can only be decided while it is in Customer Approval"},
		{stageKindCustomerReview, "ROLLBACK", "this approval is no longer pending: the change request is in Rollback, but the Customer Review stage can only be decided while it is in Customer Review"},
	} {
		err := staleApprovalRefusal(tc.kind, tc.state)
		var ce *apierror.ConflictError
		if !errors.As(err, &ce) {
			t.Fatalf("staleApprovalRefusal(%v, %s) = %T, want *apierror.ConflictError", tc.kind, tc.state, err)
		}
		if ce.Msg != tc.want {
			t.Errorf("staleApprovalRefusal(%v, %s) = %q, want %q", tc.kind, tc.state, ce.Msg, tc.want)
		}
	}
}
