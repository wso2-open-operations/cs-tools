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

package repository_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// Skipped without CHANGE_REQUEST_TEST_DSN, the same convention every other
// repository integration test in this package uses.
//
//	CHANGE_REQUEST_TEST_DSN=postgres://... go test ./internal/repository/ -run ChangeRequestIntegration

const (
	changeRequestApprovalTestID          = "36666666-0000-0000-0000-000000000001"
	changeRequestApprovalApproverUserID  = "36666666-0000-0000-0000-000000000003"
	changeRequestApprovalApproverUserID2 = "36666666-0000-0000-0000-000000000004"
	changeRequestApprovalApproverUserID3 = "36666666-0000-0000-0000-000000000005"

	// changeRequestAssignedTeamTestID is its own id, distinct from
	// changeRequestApprovalTestID above, so the AssignedTeamID tests below
	// never race the approval tests' seed/cleanup of the same work_item row.
	changeRequestAssignedTeamTestID = "36666666-0000-0000-0000-000000000006"

	// seededGroupID is scripts/csm-compose/seed-entity-service.sql's one
	// "group" row ("Example Corp ABT", id 901) -- reused here rather than
	// inserting a fresh "group" row for this test alone, to avoid growing
	// that seed file for something it already covers.
	seededGroupID = "00000000-0000-0000-0000-000000000901"

	// unknownGroupID is a well-formed UUID that is not the id of any "group"
	// row -- used to exercise assignment_group_id's FK violation path.
	unknownGroupID = "36666666-aaaa-0000-0000-000000000000"

	// changeRequestAssessGateTestID is its own id, distinct from every other
	// test's change request above, so the Assess-gate/approver-provisioning
	// tests below never race any of them over the same work_item row.
	changeRequestAssessGateTestID = "36666666-0000-0000-0000-000000000007"

	// changeRequestAssessGateMemberUserID{,2} are seeded as team_member rows
	// against changeRequestAssessGateGroupID (distinct from every other
	// test's own user ids) to exercise Assess's auto-provisioned approvers.
	changeRequestAssessGateMemberUserID  = "36666666-0000-0000-0000-000000000008"
	changeRequestAssessGateMemberUserID2 = "36666666-0000-0000-0000-000000000009"

	// changeRequestAssessGateGroupID is its own dedicated "group" row,
	// deliberately NOT seededGroupID -- the membership-provisioning tests
	// below assert the *exact, exhaustive* set of a group's members, which
	// is unsafe to share with seededGroupID: that fixture is reused by other
	// tests (and by hand during local manual verification) for its mere FK
	// validity, with no guarantee nothing else ever attaches a team_member
	// row to it. A real collision of exactly this kind was hit once already
	// (a manual verification session left two real users' team_member rows
	// pointed at seededGroupID, inflating this test's own membership count
	// until that residue was cleaned up) -- a dedicated, test-owned group
	// makes that structurally impossible instead of merely unlikely.
	changeRequestAssessGateGroupID = "36666666-0000-0000-0000-00000000000a"

	// changeRequestReviewGateTestID is its own id, distinct from every other
	// test's change request above (including the Assess- and Authorize-gate
	// tests), so the Review-gate/approver-provisioning tests below never race
	// any of them over the same work_item row.
	changeRequestReviewGateTestID = "36666666-0000-0000-0000-00000000000f"

	// changeRequestReviewGateMemberUserID{,2} are seeded as team_member rows
	// against changeRequestReviewGateGroupID (distinct from every other
	// test's own user ids) to exercise Review's auto-provisioned approvers.
	changeRequestReviewGateMemberUserID  = "36666666-0000-0000-0000-000000000010"
	changeRequestReviewGateMemberUserID2 = "36666666-0000-0000-0000-000000000011"

	// changeRequestReviewGateGroupID is its own dedicated "group" row,
	// deliberately not shared with changeRequestAssessGateGroupID,
	// changeRequestAuthorizeGateGroupID or seededGroupID -- same isolation
	// reasoning as changeRequestAssessGateGroupID's own doc comment.
	changeRequestReviewGateGroupID = "36666666-0000-0000-0000-000000000012"

	// changeRequestOnHoldTestID is its own id, distinct from every other
	// test's change request above, so the on-hold gate tests below never
	// race any of them over the same work_item row.
	changeRequestOnHoldTestID = "36666666-0000-0000-0000-000000000013"

	// Customer-approval/customer-review authorization fixtures (change_request.
	// is_customer_approval_required/is_customer_review_required -- see
	// authorizeChangeRequestCustomerFlagWrite's own doc comment in
	// change_request_repo.go for the full rule). A distinct id prefix
	// ("37777777...") from every fixture above, own account/projects/
	// contacts, so these tests never race any other test in this file over
	// a shared row.
	crCustomerFlagAccountID        = "37777777-0000-0000-0000-000000000001"
	crCustomerFlagAccountContactID = "37777777-0000-0000-0000-000000000002"

	// crCustomerFlagProjectID is the change request's OWN project in every
	// test below except the "different project" one -- it gets a REGISTERED
	// PORTAL_USER contact (crCustomerFlagPortalUserEmail) and a REGISTERED,
	// but NOT PORTAL_USER-holding, contact's own membership is never added
	// to it (see crCustomerFlagNoRoleProjectID for that case instead).
	crCustomerFlagProjectID = "37777777-0000-0000-0000-000000000003"
	// crCustomerFlagOtherProjectID is a DIFFERENT project, with its own
	// REGISTERED PORTAL_USER contact (crCustomerFlagOtherProjectEmail) --
	// exercises "a registered contact's own authorization does not transfer
	// to a change request on a different project."
	crCustomerFlagOtherProjectID = "37777777-0000-0000-0000-000000000004"
	// crCustomerFlagNoRoleProjectID has exactly one contact
	// (crCustomerFlagWrongRoleEmail), REGISTERED but holding only
	// SECURITY_CONTACT -- a literal "project with no qualifying contact" per
	// this feature's own product decision (point 3 of its design): nobody
	// external can ever flip a flag on a change request linked to this
	// project, which must have zero bearing on that change request's own
	// state transitions.
	crCustomerFlagNoRoleProjectID = "37777777-0000-0000-0000-000000000005"

	crCustomerFlagPortalUserEmail   = "cr-customer-flag-portal-user@example.test"
	crCustomerFlagOtherProjectEmail = "cr-customer-flag-other-project@example.test"
	crCustomerFlagWrongRoleEmail    = "cr-customer-flag-wrong-role@example.test"
	// crCustomerFlagInvitedEmail is INVITED (not yet REGISTERED) on
	// crCustomerFlagProjectID despite holding PORTAL_USER -- exercises
	// "registration state matters as much as role."
	crCustomerFlagInvitedEmail = "cr-customer-flag-invited@example.test"

	// One change request (work_item) id per test below, so none of them
	// ever race another over the same row.
	crCustomerFlagInternalTestID         = "37777777-0000-0000-0000-000000000010"
	crCustomerFlagQualifyingTestID       = "37777777-0000-0000-0000-000000000011"
	crCustomerFlagDifferentProjectTestID = "37777777-0000-0000-0000-000000000012"
	crCustomerFlagWrongRoleTestID        = "37777777-0000-0000-0000-000000000013"
	crCustomerFlagInvitedTestID          = "37777777-0000-0000-0000-000000000014"
	crCustomerFlagLockedTestID           = "37777777-0000-0000-0000-000000000015"
	crCustomerFlagNoOpTestID             = "37777777-0000-0000-0000-000000000016"
	crCustomerFlagIndependentTestID      = "37777777-0000-0000-0000-000000000017"
)

// seedApprovalUserForDecisionTest inserts one "user" row per given id --
// approval_stage_approver.approver_user_id's FK needs a real one -- as its
// own fresh rows (not relying on any of the local docker-compose stack's own
// incidental seed users) so this test only depends on migrations having run,
// the same assumption every other integration test in this package makes.
func seedApprovalUserForDecisionTest(t *testing.T, pool *pgxpool.Pool, userIDs ...string) {
	t.Helper()
	ctx := context.Background()
	if len(userIDs) == 0 {
		userIDs = []string{changeRequestApprovalApproverUserID}
	}

	for i, userID := range userIDs {
		id := userID
		cleanup := func() {
			_, _ = pool.Exec(ctx, `DELETE FROM "user" WHERE id = $1`, id)
		}
		cleanup()
		t.Cleanup(cleanup)

		email := fmt.Sprintf("cr-approval-test-%d@example.com", i+1)
		if _, err := pool.Exec(ctx,
			`INSERT INTO "user" (id, created_on, updated_on, created_by, updated_by, user_name, name, first_name, last_name, email, is_active, is_system_user)
			 VALUES ($1, now(), now(), 'cr-approval-test', 'cr-approval-test', $2, 'CR Approval Test', 'CR', 'Approval Test', $2, true, false)`,
			id, email); err != nil {
			t.Fatalf("seed approver user %s: %v", id, err)
		}
		setTestUserType(t, pool, ctx, id, userTypeInternal)
	}
}

// seedChangeRequestForApprovalTest inserts a minimal work_item/change_request
// pair in the given state -- enough for PatchChangeRequest's own read (via
// GetChangeRequestByID at the end of a successful patch) to resolve, since
// every other join in changeRequestFromJoins is a LEFT JOIN.
func seedChangeRequestForApprovalTest(t *testing.T, pool *repository.Scoped, state string) {
	t.Helper()
	// work_item and change_request are RLS-protected; seed/cleanup as internal.
	ctx := repository.WithSystemIdentity(context.Background())

	cleanup := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM work_item WHERE id = $1`, changeRequestApprovalTestID)
	}
	cleanup()
	t.Cleanup(cleanup)

	mustExec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed (%.60s): %v", sql, err)
		}
	}

	mustExec(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, subject, type)
	          VALUES ($1, now(), now(), 'cr-approval-test-creator', 'cr-approval-test', 'CRAPPRV01', 'approval guard test', 'CHANGE_REQUEST')`,
		changeRequestApprovalTestID)
	mustExec(`INSERT INTO change_request (id, state) VALUES ($1, $2::change_request_state_enum)`,
		changeRequestApprovalTestID, state)
}

// TestChangeRequestIntegration_RequestApprovalIsBookkeepingOnly confirms the
// corrected behavior: {requestApproval: true} always sets
// change_request.approval = 'REQUESTED' and never touches state, regardless
// of the change request's current state. This replaces two prior tests
// (TestChangeRequestIntegration_RequestApprovalRejectsNonNewState/
// TestChangeRequestIntegration_RequestApprovalAdvancesNewToAssess) that
// asserted the earlier, now-confirmed-wrong model -- PatchChangeRequest used
// to also force state=ASSESS for a change request in New (and reject the
// request with a ConflictError for any other state) modeling New->Assess as
// an approval-gated ceremony. Checked against the real ServiceNow instance:
// New->Assess is a plain, ungated state change like every other transition,
// unrelated to approval at all -- the one real approval-gated transition is
// Assess->Authorize, handled entirely by DecideChangeRequestApproval, which
// this test does not touch.
func TestChangeRequestIntegration_RequestApprovalIsBookkeepingOnly(t *testing.T) {
	dsn := os.Getenv("CHANGE_REQUEST_TEST_DSN")
	if dsn == "" {
		t.Skip("CHANGE_REQUEST_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	scoped := repository.NewScoped(pool)
	sys := repository.WithSystemIdentity(context.Background())
	repo := repository.NewChangeRequestRepository(scoped)

	for _, seedState := range []string{"NEW", "REVIEW", "CLOSED"} {
		t.Run(seedState, func(t *testing.T) {
			seedChangeRequestForApprovalTest(t, scoped, seedState)

			yes := true
			_, err := repo.PatchChangeRequest(sys, changeRequestApprovalTestID,
				domain.PatchChangeRequestRequest{RequestApproval: &yes}, "cr-approval-test")
			if err != nil {
				t.Fatalf("PatchChangeRequest(requestApproval=true) on a %s-state change request: %v", seedState, err)
			}

			var gotState, gotApproval *string
			if scanErr := scoped.QueryRow(sys,
				`SELECT state::TEXT, approval::TEXT FROM change_request WHERE id = $1`, changeRequestApprovalTestID).
				Scan(&gotState, &gotApproval); scanErr != nil {
				t.Fatalf("read back state/approval: %v", scanErr)
			}
			if gotState == nil || *gotState != seedState {
				t.Fatalf("state after requestApproval=true = %v, want unchanged %q", gotState, seedState)
			}
			if gotApproval == nil || *gotApproval != "REQUESTED" {
				t.Fatalf("approval after requestApproval=true = %v, want \"REQUESTED\"", gotApproval)
			}
		})
	}
}

// seedApprovalStageForDecisionTest inserts one approval_stage plus one or
// more approval_stage_approver rows for changeRequestApprovalTestID -- the
// same shared seed changeRequestForApprovalTest/seedChangeRequestForApprovalTest
// use, extended with an actual approval stage so DecideChangeRequestApproval
// has something real to act on. Each entry in approverUserIDs gets its own
// requested approver row; the returned stage id lets a test seed additional
// rows (e.g. a second approver already rejected) directly.
func seedApprovalStageForDecisionTest(t *testing.T, pool *repository.Scoped, approverUserIDs ...string) string {
	t.Helper()
	// approval_stage and approval_stage_approver are RLS-protected; seed/cleanup as internal.
	ctx := repository.WithSystemIdentity(context.Background())

	stageID := "36666666-0000-0000-0000-000000000002"
	mustExec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed (%.60s): %v", sql, err)
		}
	}
	mustExec(`INSERT INTO approval_stage (id, created_on, updated_on, created_by, updated_by, work_item_id, raw_status)
	          VALUES ($1, now(), now(), 'cr-approval-test', 'cr-approval-test', $2, 'requested')`,
		stageID, changeRequestApprovalTestID)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM approval_stage WHERE id = $1`, stageID)
	})

	for i, userID := range approverUserIDs {
		mustExec(`INSERT INTO approval_stage_approver (id, created_on, updated_on, created_by, updated_by, stage_id, work_item_id, approver_user_id, state)
		          VALUES ($1, now(), now(), 'cr-approval-test', 'cr-approval-test', $2, $3, $4, 'REQUESTED')`,
			fmt.Sprintf("36666666-0000-0000-0000-00000000%04d", i+10), stageID, changeRequestApprovalTestID, userID)
	}
	return stageID
}

// TestChangeRequestIntegration_DecideApprovalCascadesAssessToAuthorize is the
// regression guard for a real, reported gap: DecideChangeRequestApproval used
// to only flip the one approval_stage_approver row and never touch
// change_request.state at all -- confirmed live against a real approval on
// this data source. Approving the sole (or last-standing) approver on a
// change request's Assess-stage approval, while it's actually sitting in
// Assess, must now advance it to Authorize.
func TestChangeRequestIntegration_DecideApprovalCascadesAssessToAuthorize(t *testing.T) {
	dsn := os.Getenv("CHANGE_REQUEST_TEST_DSN")
	if dsn == "" {
		t.Skip("CHANGE_REQUEST_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	scoped := repository.NewScoped(pool)
	sys := repository.WithSystemIdentity(context.Background())
	repo := repository.NewChangeRequestRepository(scoped)
	seedApprovalUserForDecisionTest(t, pool)
	seedChangeRequestForApprovalTest(t, scoped, "ASSESS")
	seedApprovalGroupMembers(t, scoped, crCABGroupID, crCABMemberUserID1)
	seedApprovalStageForDecisionTest(t, scoped, changeRequestApprovalApproverUserID)

	if _, err := repo.DecideChangeRequestApproval(sys, changeRequestApprovalTestID,
		changeRequestApprovalApproverUserID, "approved", "cr-approval-test"); err != nil {
		t.Fatalf("DecideChangeRequestApproval(approved): %v", err)
	}

	var gotState string
	if scanErr := scoped.QueryRow(sys,
		`SELECT state::TEXT FROM change_request WHERE id = $1`, changeRequestApprovalTestID).Scan(&gotState); scanErr != nil {
		t.Fatalf("read back state: %v", scanErr)
	}
	if gotState != "AUTHORIZE" {
		t.Fatalf("state after approval = %q, want \"AUTHORIZE\"", gotState)
	}

	// ...and the CAB Approval stage (its own group) is provisioned right away.
	var cabStages int
	if scanErr := scoped.QueryRow(sys,
		`SELECT COUNT(*) FROM approval_stage WHERE work_item_id = $1 AND checkpoint_label = 'CAB Approval' AND assignment_group_id = $2::uuid`,
		changeRequestApprovalTestID, crCABGroupID).Scan(&cabStages); scanErr != nil {
		t.Fatalf("count CAB stages: %v", scanErr)
	}
	if cabStages != 1 {
		t.Fatalf("CAB Approval stages after peer approval = %d, want 1", cabStages)
	}
}

// TestChangeRequestIntegration_DecideApprovalRejectionDoesNotCascade confirms
// a rejection never advances state, even though it resolves the stage (the
// same first-responder-wins quorum rule that lets a single approval resolve
// one also lets a single rejection resolve one) -- there is no confirmed
// target for a rejected Assess stage in this schema, so this must be a
// pure no-op on change_request.state.
func TestChangeRequestIntegration_DecideApprovalRejectionDoesNotCascade(t *testing.T) {
	dsn := os.Getenv("CHANGE_REQUEST_TEST_DSN")
	if dsn == "" {
		t.Skip("CHANGE_REQUEST_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	scoped := repository.NewScoped(pool)
	sys := repository.WithSystemIdentity(context.Background())
	repo := repository.NewChangeRequestRepository(scoped)
	seedApprovalUserForDecisionTest(t, pool)
	seedChangeRequestForApprovalTest(t, scoped, "ASSESS")
	seedApprovalStageForDecisionTest(t, scoped, changeRequestApprovalApproverUserID)

	if _, err := repo.DecideChangeRequestApproval(sys, changeRequestApprovalTestID,
		changeRequestApprovalApproverUserID, "rejected", "cr-approval-test"); err != nil {
		t.Fatalf("DecideChangeRequestApproval(rejected): %v", err)
	}

	var gotState string
	if scanErr := scoped.QueryRow(sys,
		`SELECT state::TEXT FROM change_request WHERE id = $1`, changeRequestApprovalTestID).Scan(&gotState); scanErr != nil {
		t.Fatalf("read back state: %v", scanErr)
	}
	if gotState != "ASSESS" {
		t.Fatalf("state after rejection = %q, want unchanged \"ASSESS\"", gotState)
	}
}

// TestChangeRequestIntegration_DecideApprovalDoesNotCascadeOutsideAssess
// confirms the cascade is scoped exactly to Assess->Authorize: approving a
// Peer-stage approver on a change request that isn't currently in Assess (e.g.
// one already sitting in Authorize) must leave state untouched. A stage can
// only be decided in the state it belongs to (a Peer stage in Assess), so the
// decision is now refused outright with a 409 rather than recorded without a
// cascade: nothing changes, the approver row stays requested (the state
// reconcile does not run on a refused decision). The seeded stage has no
// label, so it is classified by position, as Peer.
func TestChangeRequestIntegration_DecideApprovalDoesNotCascadeOutsideAssess(t *testing.T) {
	dsn := os.Getenv("CHANGE_REQUEST_TEST_DSN")
	if dsn == "" {
		t.Skip("CHANGE_REQUEST_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	scoped := repository.NewScoped(pool)
	sys := repository.WithSystemIdentity(context.Background())
	repo := repository.NewChangeRequestRepository(scoped)
	seedApprovalUserForDecisionTest(t, pool)
	seedChangeRequestForApprovalTest(t, scoped, "AUTHORIZE")
	seedApprovalStageForDecisionTest(t, scoped, changeRequestApprovalApproverUserID)

	_, err = repo.DecideChangeRequestApproval(sys, changeRequestApprovalTestID,
		changeRequestApprovalApproverUserID, "approved", "cr-approval-test")
	var conflict *apierror.ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("DecideChangeRequestApproval(approved) outside Assess: err = %v (%T), want *apierror.ConflictError", err, err)
	}
	if want := "this approval is no longer pending: the change request is in Authorize, but the Peer Approval stage can only be decided while it is in Assess"; conflict.Msg != want {
		t.Fatalf("refusal message = %q, want %q", conflict.Msg, want)
	}

	var gotState, gotStatus string
	if scanErr := scoped.QueryRow(sys,
		`SELECT state::TEXT FROM change_request WHERE id = $1`, changeRequestApprovalTestID).Scan(&gotState); scanErr != nil {
		t.Fatalf("read back state: %v", scanErr)
	}
	if gotState != "AUTHORIZE" {
		t.Fatalf("state after a refused approval outside Assess = %q, want unchanged \"AUTHORIZE\"", gotState)
	}
	if scanErr := scoped.QueryRow(sys,
		`SELECT state FROM approval_stage_approver WHERE work_item_id = $1 AND approver_user_id = $2`,
		changeRequestApprovalTestID, changeRequestApprovalApproverUserID).Scan(&gotStatus); scanErr != nil {
		t.Fatalf("read back approver status: %v", scanErr)
	}
	if gotStatus != "REQUESTED" {
		t.Fatalf("approver status after a refused decision = %q, want unchanged \"REQUESTED\"", gotStatus)
	}
}

// TestChangeRequestIntegration_DecideApprovalCancelsSiblingApprovers is the
// regression guard for the other real gap in the original cascade fix:
// resolving a stage used to leave every other still-pending approver on it
// sitting at Requested forever, with no visible way to tell "this group has
// already been decided" apart from "nobody has looked at this yet". Real
// ServiceNow does not do this (confirmed live against a genuine
// multi-approver group): once enough approvers respond to resolve the group,
// every other pending approver on it is moved to Cancelled. This test seeds
// three approvers on one stage and confirms that deciding as just one of
// them resolves the stage AND cancels the other two -- not just the acted-on
// row.
func TestChangeRequestIntegration_DecideApprovalCancelsSiblingApprovers(t *testing.T) {
	dsn := os.Getenv("CHANGE_REQUEST_TEST_DSN")
	if dsn == "" {
		t.Skip("CHANGE_REQUEST_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	scoped := repository.NewScoped(pool)
	sys := repository.WithSystemIdentity(context.Background())
	repo := repository.NewChangeRequestRepository(scoped)
	seedApprovalUserForDecisionTest(t, pool,
		changeRequestApprovalApproverUserID, changeRequestApprovalApproverUserID2, changeRequestApprovalApproverUserID3)
	seedChangeRequestForApprovalTest(t, scoped, "ASSESS")
	seedApprovalGroupMembers(t, scoped, crCABGroupID, crCABMemberUserID1)
	stageID := seedApprovalStageForDecisionTest(t, scoped,
		changeRequestApprovalApproverUserID, changeRequestApprovalApproverUserID2, changeRequestApprovalApproverUserID3)

	if _, err := repo.DecideChangeRequestApproval(sys, changeRequestApprovalTestID,
		changeRequestApprovalApproverUserID, "approved", "cr-approval-test"); err != nil {
		t.Fatalf("DecideChangeRequestApproval(approved): %v", err)
	}

	rows, err := scoped.Query(sys,
		`SELECT approver_user_id, state FROM approval_stage_approver WHERE stage_id = $1`, stageID)
	if err != nil {
		t.Fatalf("read back approver statuses: %v", err)
	}
	defer rows.Close()

	statusByApprover := map[string]string{}
	for rows.Next() {
		var approverID, status string
		if err := rows.Scan(&approverID, &status); err != nil {
			t.Fatalf("scan approver row: %v", err)
		}
		statusByApprover[approverID] = status
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate approver rows: %v", err)
	}

	if got := statusByApprover[changeRequestApprovalApproverUserID]; got != "APPROVED" {
		t.Errorf("acted-on approver status = %q, want \"APPROVED\"", got)
	}
	if got := statusByApprover[changeRequestApprovalApproverUserID2]; got != "CANCELLED" {
		t.Errorf("sibling approver 2 status = %q, want \"CANCELLED\" (not left at \"REQUESTED\")", got)
	}
	if got := statusByApprover[changeRequestApprovalApproverUserID3]; got != "CANCELLED" {
		t.Errorf("sibling approver 3 status = %q, want \"CANCELLED\" (not left at \"REQUESTED\")", got)
	}

	var gotState string
	if scanErr := scoped.QueryRow(sys,
		`SELECT state::TEXT FROM change_request WHERE id = $1`, changeRequestApprovalTestID).Scan(&gotState); scanErr != nil {
		t.Fatalf("read back change request state: %v", scanErr)
	}
	if gotState != "AUTHORIZE" {
		t.Fatalf("state after multi-approver approval = %q, want \"AUTHORIZE\"", gotState)
	}
}

// seedPriorApprovalStagesForDecisionTest inserts count empty, approver-less
// approval_stage rows for changeRequestApprovalTestID, each created further
// in the past than the last (1 hour per position), so that a stage created
// afterward by seedApprovalStageForDecisionTest (which always inserts at
// "now") sorts after all of them -- pushing that real stage to ordinal
// position `count`, i.e. the same "earlier stage exists" shape
// DecideChangeRequestApproval's own isAssessStage check keys off of. Used
// by the rejection-sibling-cancellation tests below to prove that behavior
// isn't scoped to the Assess position the way the state cascade is. No
// explicit cleanup: seedChangeRequestForApprovalTest's own
// "DELETE FROM work_item" cascades through approval_stage's FK the same way
// it already does for the one seedApprovalStageForDecisionTest itself
// creates.
func seedPriorApprovalStagesForDecisionTest(t *testing.T, pool *repository.Scoped, count int) {
	t.Helper()
	ctx := repository.WithSystemIdentity(context.Background())
	for i := 0; i < count; i++ {
		id := fmt.Sprintf("36666666-0000-0000-0000-0000000040%02d", i)
		if _, err := pool.Exec(ctx,
			`INSERT INTO approval_stage (id, created_on, updated_on, created_by, updated_by, work_item_id, raw_status)
			 VALUES ($1, now() - (INTERVAL '1 hour' * $2::int), now(), 'cr-approval-test', 'cr-approval-test', $3, 'requested')`,
			id, count-i, changeRequestApprovalTestID); err != nil {
			t.Fatalf("seed prior approval_stage %d: %v", i, err)
		}
	}
}

// TestChangeRequestIntegration_DecideRejectionCancelsSiblingApprovers is the
// rejection-side counterpart of
// TestChangeRequestIntegration_DecideApprovalCancelsSiblingApprovers above:
// a rejection now resolves a stage exactly as decisively as an approval
// does, so every other still-Requested sibling approver on the same stage
// must be moved to Cancelled too -- not left sitting at Requested forever,
// which is exactly the gap this fix closes. change_request.state must stay
// completely untouched regardless (see
// TestChangeRequestIntegration_DecideApprovalRejectionDoesNotCascade's own
// doc comment -- a rejection never cascades state, forward or backward).
func TestChangeRequestIntegration_DecideRejectionCancelsSiblingApprovers(t *testing.T) {
	dsn := os.Getenv("CHANGE_REQUEST_TEST_DSN")
	if dsn == "" {
		t.Skip("CHANGE_REQUEST_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	scoped := repository.NewScoped(pool)
	sys := repository.WithSystemIdentity(context.Background())
	repo := repository.NewChangeRequestRepository(scoped)
	seedApprovalUserForDecisionTest(t, pool,
		changeRequestApprovalApproverUserID, changeRequestApprovalApproverUserID2, changeRequestApprovalApproverUserID3)
	seedChangeRequestForApprovalTest(t, scoped, "ASSESS")
	stageID := seedApprovalStageForDecisionTest(t, scoped,
		changeRequestApprovalApproverUserID, changeRequestApprovalApproverUserID2, changeRequestApprovalApproverUserID3)

	if _, err := repo.DecideChangeRequestApproval(sys, changeRequestApprovalTestID,
		changeRequestApprovalApproverUserID, "rejected", "cr-approval-test"); err != nil {
		t.Fatalf("DecideChangeRequestApproval(rejected): %v", err)
	}

	rows, err := scoped.Query(sys,
		`SELECT approver_user_id, state FROM approval_stage_approver WHERE stage_id = $1`, stageID)
	if err != nil {
		t.Fatalf("read back approver statuses: %v", err)
	}
	defer rows.Close()

	statusByApprover := map[string]string{}
	for rows.Next() {
		var approverID, status string
		if err := rows.Scan(&approverID, &status); err != nil {
			t.Fatalf("scan approver row: %v", err)
		}
		statusByApprover[approverID] = status
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate approver rows: %v", err)
	}

	if got := statusByApprover[changeRequestApprovalApproverUserID]; got != "REJECTED" {
		t.Errorf("acted-on approver status = %q, want \"REJECTED\"", got)
	}
	if got := statusByApprover[changeRequestApprovalApproverUserID2]; got != "CANCELLED" {
		t.Errorf("sibling approver 2 status = %q, want \"CANCELLED\" (not left at \"REQUESTED\")", got)
	}
	if got := statusByApprover[changeRequestApprovalApproverUserID3]; got != "CANCELLED" {
		t.Errorf("sibling approver 3 status = %q, want \"CANCELLED\" (not left at \"REQUESTED\")", got)
	}

	var gotState string
	if scanErr := scoped.QueryRow(sys,
		`SELECT state::TEXT FROM change_request WHERE id = $1`, changeRequestApprovalTestID).Scan(&gotState); scanErr != nil {
		t.Fatalf("read back change request state: %v", scanErr)
	}
	if gotState != "ASSESS" {
		t.Fatalf("state after multi-approver rejection = %q, want unchanged \"ASSESS\" (a rejection never cascades state)", gotState)
	}
}

// TestChangeRequestIntegration_DecideRejectionCancelsSiblingApproversAtEveryCheckpoint
// confirms sibling-cancellation-on-rejection is NOT scoped to the Assess
// position the way the state cascade is -- unlike
// TestChangeRequestIntegration_DecideApprovalDoesNotCascadeOutsideAssess
// (which only guards the state write), this proves the cancellation itself
// fires identically at Authorize and Review. seedPriorApprovalStagesForDecisionTest
// pushes the real stage to the given ordinal the same way a genuine
// Assess-then-Authorize(-then-Review) history would.
func TestChangeRequestIntegration_DecideRejectionCancelsSiblingApproversAtEveryCheckpoint(t *testing.T) {
	dsn := os.Getenv("CHANGE_REQUEST_TEST_DSN")
	if dsn == "" {
		t.Skip("CHANGE_REQUEST_TEST_DSN not set")
	}

	for _, tc := range []struct {
		name        string
		state       string
		priorStages int
	}{
		{name: "Authorize", state: "AUTHORIZE", priorStages: 1},
		{name: "Review", state: "REVIEW", priorStages: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pool, err := pgxpool.New(context.Background(), dsn)
			if err != nil {
				t.Fatalf("connect: %v", err)
			}
			t.Cleanup(pool.Close)

			scoped := repository.NewScoped(pool)
			sys := repository.WithSystemIdentity(context.Background())
			repo := repository.NewChangeRequestRepository(scoped)
			seedApprovalUserForDecisionTest(t, pool,
				changeRequestApprovalApproverUserID, changeRequestApprovalApproverUserID2, changeRequestApprovalApproverUserID3)
			seedChangeRequestForApprovalTest(t, scoped, tc.state)
			seedPriorApprovalStagesForDecisionTest(t, scoped, tc.priorStages)
			stageID := seedApprovalStageForDecisionTest(t, scoped,
				changeRequestApprovalApproverUserID, changeRequestApprovalApproverUserID2, changeRequestApprovalApproverUserID3)

			if _, err := repo.DecideChangeRequestApproval(sys, changeRequestApprovalTestID,
				changeRequestApprovalApproverUserID, "rejected", "cr-approval-test"); err != nil {
				t.Fatalf("DecideChangeRequestApproval(rejected): %v", err)
			}

			rows, err := scoped.Query(sys,
				`SELECT approver_user_id, state FROM approval_stage_approver WHERE stage_id = $1`, stageID)
			if err != nil {
				t.Fatalf("read back approver statuses: %v", err)
			}
			defer rows.Close()

			statusByApprover := map[string]string{}
			for rows.Next() {
				var approverID, status string
				if err := rows.Scan(&approverID, &status); err != nil {
					t.Fatalf("scan approver row: %v", err)
				}
				statusByApprover[approverID] = status
			}
			if err := rows.Err(); err != nil {
				t.Fatalf("iterate approver rows: %v", err)
			}

			if got := statusByApprover[changeRequestApprovalApproverUserID]; got != "REJECTED" {
				t.Errorf("acted-on approver status = %q, want \"REJECTED\"", got)
			}
			if got := statusByApprover[changeRequestApprovalApproverUserID2]; got != "CANCELLED" {
				t.Errorf("sibling approver 2 status at %s = %q, want \"CANCELLED\"", tc.name, got)
			}
			if got := statusByApprover[changeRequestApprovalApproverUserID3]; got != "CANCELLED" {
				t.Errorf("sibling approver 3 status at %s = %q, want \"CANCELLED\"", tc.name, got)
			}

			var gotState string
			if scanErr := scoped.QueryRow(sys,
				`SELECT state::TEXT FROM change_request WHERE id = $1`, changeRequestApprovalTestID).Scan(&gotState); scanErr != nil {
				t.Fatalf("read back change request state: %v", scanErr)
			}
			if gotState != tc.state {
				t.Fatalf("state after %s rejection = %q, want unchanged %q", tc.name, gotState, tc.state)
			}
		})
	}
}

// TestChangeRequestIntegration_DecideRejectionDoesNotDisturbAlreadyApprovedStage
// is the edge case this fix's own design decision needed: a stage that was
// already resolved by an APPROVAL -- seeded here directly, as a stand-in
// for a ServiceNow-synced stage or one seeded before this fix shipped,
// since this repository's own serialized flow can no longer produce this
// shape itself (see DecideChangeRequestApproval's "rejected" branch's own
// doc comment on hasApproval) -- must not have its other still-Requested
// approvers disturbed by a later, late rejection on a different approver of
// the same stage. A destructive retroactive cancellation here would be
// exactly wrong: the earlier approval already had every right to leave
// those rows alone.
func TestChangeRequestIntegration_DecideRejectionDoesNotDisturbAlreadyApprovedStage(t *testing.T) {
	dsn := os.Getenv("CHANGE_REQUEST_TEST_DSN")
	if dsn == "" {
		t.Skip("CHANGE_REQUEST_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	scoped := repository.NewScoped(pool)
	sys := repository.WithSystemIdentity(context.Background())
	repo := repository.NewChangeRequestRepository(scoped)
	seedApprovalUserForDecisionTest(t, pool,
		changeRequestApprovalApproverUserID, changeRequestApprovalApproverUserID2, changeRequestApprovalApproverUserID3)
	seedChangeRequestForApprovalTest(t, scoped, "ASSESS")
	stageID := seedApprovalStageForDecisionTest(t, scoped,
		changeRequestApprovalApproverUserID, changeRequestApprovalApproverUserID2, changeRequestApprovalApproverUserID3)

	// Simulate a stage some other path already resolved via approval
	// without running this method's own sibling-cancellation -- directly
	// flipping approver 2's row, bypassing DecideChangeRequestApproval
	// entirely, the same way a ServiceNow sync (or pre-fix data) could have
	// left it. Approver 3 is deliberately left "requested" to prove it
	// survives untouched below.
	if _, err := scoped.Exec(sys,
		`UPDATE approval_stage_approver SET state = 'APPROVED' WHERE stage_id = $1 AND approver_user_id = $2`,
		stageID, changeRequestApprovalApproverUserID2); err != nil {
		t.Fatalf("seed pre-existing approval: %v", err)
	}

	if _, err := repo.DecideChangeRequestApproval(sys, changeRequestApprovalTestID,
		changeRequestApprovalApproverUserID3, "rejected", "cr-approval-test"); err != nil {
		t.Fatalf("DecideChangeRequestApproval(rejected): %v", err)
	}

	rows, err := scoped.Query(sys,
		`SELECT approver_user_id, state FROM approval_stage_approver WHERE stage_id = $1`, stageID)
	if err != nil {
		t.Fatalf("read back approver statuses: %v", err)
	}
	defer rows.Close()

	statusByApprover := map[string]string{}
	for rows.Next() {
		var approverID, status string
		if err := rows.Scan(&approverID, &status); err != nil {
			t.Fatalf("scan approver row: %v", err)
		}
		statusByApprover[approverID] = status
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate approver rows: %v", err)
	}

	if got := statusByApprover[changeRequestApprovalApproverUserID3]; got != "REJECTED" {
		t.Errorf("acted-on approver status = %q, want \"REJECTED\"", got)
	}
	if got := statusByApprover[changeRequestApprovalApproverUserID2]; got != "APPROVED" {
		t.Errorf("pre-existing approved approver status = %q, want unchanged \"APPROVED\"", got)
	}
	// The real assertion of this test: approver 1 was never decided by
	// anyone and is NOT a sibling of the rejection's own resolution (the
	// stage was already resolved, by the approval, before the rejection
	// ever ran) -- the hasApproval guard must have skipped cancellation
	// entirely, leaving it exactly as it was.
	if got := statusByApprover[changeRequestApprovalApproverUserID]; got != "REQUESTED" {
		t.Errorf("untouched sibling approver status = %q, want unchanged \"REQUESTED\" (an already-approved stage must not be disturbed by a later rejection)", got)
	}

	var gotState string
	if scanErr := scoped.QueryRow(sys,
		`SELECT state::TEXT FROM change_request WHERE id = $1`, changeRequestApprovalTestID).Scan(&gotState); scanErr != nil {
		t.Fatalf("read back change request state: %v", scanErr)
	}
	if gotState != "ASSESS" {
		t.Fatalf("state after rejection on an already-approved stage = %q, want unchanged \"ASSESS\"", gotState)
	}
}

// seedChangeRequestForAssignedTeamTest inserts a minimal work_item/
// change_request pair for the AssignedTeamID tests below, following the same
// shape as seedChangeRequestForApprovalTest but under its own id so the two
// test groups can never collide.
func seedChangeRequestForAssignedTeamTest(t *testing.T, pool *repository.Scoped) {
	t.Helper()
	// work_item and change_request are RLS-protected; seed/cleanup as internal.
	ctx := repository.WithSystemIdentity(context.Background())

	cleanup := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM work_item WHERE id = $1`, changeRequestAssignedTeamTestID)
	}
	cleanup()
	t.Cleanup(cleanup)

	if _, err := pool.Exec(ctx,
		`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, subject, type)
		 VALUES ($1, now(), now(), 'cr-assigned-team-test', 'cr-assigned-team-test', 'CRTEAM001', 'assigned team patch test', 'CHANGE_REQUEST')`,
		changeRequestAssignedTeamTestID); err != nil {
		t.Fatalf("seed work_item: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO change_request (id, state) VALUES ($1, 'NEW'::change_request_state_enum)`,
		changeRequestAssignedTeamTestID); err != nil {
		t.Fatalf("seed change_request: %v", err)
	}
}

// TestChangeRequestIntegration_PatchAssignedTeamID is the regression guard for
// the bug this change fixes: PatchChangeRequestRequest.AssignedTeamID used to
// be read but never written anywhere in PatchChangeRequest's own UPDATE,
// unlike the adjacent AssignedEngineerID handling. Patching it to a real
// "group" id must round-trip through work_item.assignment_group_id and come
// back as ChangeRequest.AssignedTeam on a subsequent read.
func TestChangeRequestIntegration_PatchAssignedTeamID(t *testing.T) {
	dsn := os.Getenv("CHANGE_REQUEST_TEST_DSN")
	if dsn == "" {
		t.Skip("CHANGE_REQUEST_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	scoped := repository.NewScoped(pool)
	repo := repository.NewChangeRequestRepository(scoped)
	seedChangeRequestForAssignedTeamTest(t, scoped)
	sys := repository.WithSystemIdentity(context.Background())

	teamID := seededGroupID
	updated, err := repo.PatchChangeRequest(sys, changeRequestAssignedTeamTestID,
		domain.PatchChangeRequestRequest{AssignedTeamID: &teamID}, "cr-assigned-team-test")
	if err != nil {
		t.Fatalf("PatchChangeRequest(assignedTeamId=%s): %v", teamID, err)
	}
	if updated.AssignedTeam == nil || updated.AssignedTeam.ID != teamID {
		t.Fatalf("PatchChangeRequest response AssignedTeam = %+v, want ID %q", updated.AssignedTeam, teamID)
	}

	// Read back directly, independent of the repository's own response, to
	// confirm the column itself -- not just the in-memory return value --
	// actually changed.
	var gotAssignmentGroupID string
	if scanErr := scoped.QueryRow(sys,
		`SELECT assignment_group_id::TEXT FROM work_item WHERE id = $1`, changeRequestAssignedTeamTestID).
		Scan(&gotAssignmentGroupID); scanErr != nil {
		t.Fatalf("read back assignment_group_id: %v", scanErr)
	}
	if gotAssignmentGroupID != teamID {
		t.Fatalf("work_item.assignment_group_id = %q, want %q", gotAssignmentGroupID, teamID)
	}

	// GetChangeRequestByID, the way a caller would actually re-read the
	// change request, must agree too.
	fetched, err := repo.GetChangeRequestByID(sys, changeRequestAssignedTeamTestID)
	if err != nil {
		t.Fatalf("GetChangeRequestByID: %v", err)
	}
	if fetched.AssignedTeam == nil || fetched.AssignedTeam.ID != teamID {
		t.Fatalf("GetChangeRequestByID AssignedTeam = %+v, want ID %q", fetched.AssignedTeam, teamID)
	}
}

// TestChangeRequestIntegration_PatchAssignedTeamIDUnknownTeamIsValidationError
// confirms an unknown team id produces a clean ValidationError (the
// changeRequestPatchFKField mapping's "assignedTeamId" entry), not a raw
// Postgres foreign-key-violation error surfaced to the caller.
func TestChangeRequestIntegration_PatchAssignedTeamIDUnknownTeamIsValidationError(t *testing.T) {
	dsn := os.Getenv("CHANGE_REQUEST_TEST_DSN")
	if dsn == "" {
		t.Skip("CHANGE_REQUEST_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	scoped := repository.NewScoped(pool)
	repo := repository.NewChangeRequestRepository(scoped)
	seedChangeRequestForAssignedTeamTest(t, scoped)
	sys := repository.WithSystemIdentity(context.Background())

	badTeamID := unknownGroupID
	_, err = repo.PatchChangeRequest(sys, changeRequestAssignedTeamTestID,
		domain.PatchChangeRequestRequest{AssignedTeamID: &badTeamID}, "cr-assigned-team-test")
	if err == nil {
		t.Fatal("PatchChangeRequest(assignedTeamId=<unknown>) succeeded, want a ValidationError")
	}
	var valErr *apierror.ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("PatchChangeRequest(assignedTeamId=<unknown>) error = %v (%T), want *apierror.ValidationError", err, err)
	}
	if valErr.Msg == "" {
		t.Fatal("ValidationError.Msg is empty")
	}

	// The column must be left untouched by the rolled-back transaction.
	var gotAssignmentGroupID *string
	if scanErr := scoped.QueryRow(sys,
		`SELECT assignment_group_id::TEXT FROM work_item WHERE id = $1`, changeRequestAssignedTeamTestID).
		Scan(&gotAssignmentGroupID); scanErr != nil {
		t.Fatalf("read back assignment_group_id: %v", scanErr)
	}
	if gotAssignmentGroupID != nil {
		t.Fatalf("work_item.assignment_group_id = %v after a failed patch, want unchanged NULL", *gotAssignmentGroupID)
	}
}

// seedChangeRequestForAssessGateTest inserts a minimal NEW-state work_item/
// change_request pair with no assigned team -- the starting point for every
// test below.
func seedChangeRequestForAssessGateTest(t *testing.T, pool *repository.Scoped) {
	t.Helper()
	// work_item and change_request are RLS-protected; seed/cleanup as internal.
	ctx := repository.WithSystemIdentity(context.Background())

	cleanup := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM work_item WHERE id = $1`, changeRequestAssessGateTestID)
	}
	cleanup()
	t.Cleanup(cleanup)

	if _, err := pool.Exec(ctx,
		`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, subject, type)
		 VALUES ($1, now(), now(), 'cr-assess-gate-test-creator', 'cr-assess-gate-test', 'CRASSESS01', 'assess gate test', 'CHANGE_REQUEST')`,
		changeRequestAssessGateTestID); err != nil {
		t.Fatalf("seed work_item: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO change_request (id, state) VALUES ($1, 'NEW'::change_request_state_enum)`,
		changeRequestAssessGateTestID); err != nil {
		t.Fatalf("seed change_request: %v", err)
	}
	// A Normal change cannot be sent for approval unless the CAB Approval
	// group has someone to give the second approval.
	seedApprovalGroupMembers(t, pool, crCABGroupID, crCABMemberUserID1, crCABMemberUserID2)
}

// seedAssessGateGroup inserts changeRequestAssessGateGroupID's own "group"
// row -- a dedicated fixture for the tests below, deliberately not
// seededGroupID (see that constant's own doc comment on the collision this
// avoids).
func seedAssessGateGroup(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()

	cleanup := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM "group" WHERE id = $1`, changeRequestAssessGateGroupID)
	}
	cleanup()
	t.Cleanup(cleanup)

	if _, err := pool.Exec(ctx,
		`INSERT INTO "group" (id, created_on, updated_on, created_by, updated_by, name)
		 VALUES ($1, now(), now(), 'cr-assess-gate-test', 'cr-assess-gate-test', 'Assess Gate Test Group')`,
		changeRequestAssessGateGroupID); err != nil {
		t.Fatalf("seed group: %v", err)
	}
}

// seedTeamMembersForAssessGateTest inserts one "user" row and one
// team_member row (keyed by group_id, not team_id -- see
// PatchChangeRequest's own doc comment on why) per given user id, so they
// resolve as members of changeRequestAssessGateGroupID for the
// auto-provisioning tests below. Callers must seed that group row first
// (seedAssessGateGroup) -- group_id's FK requires it to already exist.
func seedTeamMembersForAssessGateTest(t *testing.T, pool *pgxpool.Pool, userIDs ...string) {
	t.Helper()
	ctx := context.Background()

	for i, userID := range userIDs {
		id := userID
		userCleanup := func() {
			_, _ = pool.Exec(ctx, `DELETE FROM "user" WHERE id = $1`, id)
		}
		userCleanup()
		t.Cleanup(userCleanup)

		email := fmt.Sprintf("cr-assess-gate-member-%d@example.com", i+1)
		if _, err := pool.Exec(ctx,
			`INSERT INTO "user" (id, created_on, updated_on, created_by, updated_by, user_name, name, first_name, last_name, email, is_active, is_system_user)
			 VALUES ($1, now(), now(), 'cr-assess-gate-test', 'cr-assess-gate-test', $2, 'Assess Gate Member', 'Assess', 'Gate Member', $2, true, false)`,
			id, email); err != nil {
			t.Fatalf("seed team member user %s: %v", id, err)
		}
		setTestUserType(t, pool, ctx, id, userTypeInternal)

		memberCleanup := func() {
			_, _ = pool.Exec(ctx, `DELETE FROM team_member WHERE user_id = $1`, id)
		}
		memberCleanup()
		t.Cleanup(memberCleanup)

		// team_member.team_id is NOT NULL, but no test here exercises the
		// internal team registry -- the one seeded team row
		// (scripts/csm-compose/seed-entity-service.sql's "Example Corp ABT",
		// id 901) satisfies the column's NOT NULL constraint without
		// implying anything about this test's own group_id-keyed membership
		// (changeRequestAssessGateGroupID, a wholly separate "group" row).
		if _, err := pool.Exec(ctx,
			`INSERT INTO team_member (id, created_on, updated_on, created_by, updated_by, team_id, user_id, group_id)
			 VALUES (gen_random_uuid(), now(), now(), 'cr-assess-gate-test', 'cr-assess-gate-test', $1::uuid, $2, $3::uuid)`,
			seededGroupID, id, changeRequestAssessGateGroupID); err != nil {
			t.Fatalf("seed team_member for user %s: %v", id, err)
		}
	}
}

// TestChangeRequestIntegration_PatchAssessRequiresAssignedTeam is the
// regression guard for the compulsory gate: PatchChangeRequest must reject a
// {state: "assess"} patch with a clean ValidationError when the change
// request has no assigned team and the request itself doesn't supply one --
// never a silent state change with nothing to assign the Assess stage to.
func TestChangeRequestIntegration_PatchAssessRequiresAssignedTeam(t *testing.T) {
	dsn := os.Getenv("CHANGE_REQUEST_TEST_DSN")
	if dsn == "" {
		t.Skip("CHANGE_REQUEST_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	scoped := repository.NewScoped(pool)
	sys := repository.WithSystemIdentity(context.Background())
	repo := repository.NewChangeRequestRepository(scoped)
	seedChangeRequestForAssessGateTest(t, scoped)

	assess := domain.ChangeRequestStateAssess
	_, err = repo.PatchChangeRequest(sys, changeRequestAssessGateTestID,
		domain.PatchChangeRequestRequest{State: &assess}, "cr-assess-gate-test")
	if err == nil {
		t.Fatal("PatchChangeRequest(state=assess) with no assigned team succeeded, want a ValidationError")
	}
	var valErr *apierror.ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("PatchChangeRequest(state=assess) with no assigned team error = %v (%T), want *apierror.ValidationError", err, err)
	}

	var gotState string
	if scanErr := scoped.QueryRow(sys,
		`SELECT state::TEXT FROM change_request WHERE id = $1`, changeRequestAssessGateTestID).Scan(&gotState); scanErr != nil {
		t.Fatalf("read back state: %v", scanErr)
	}
	if gotState != "NEW" {
		t.Fatalf("state after a rejected assess patch = %q, want unchanged \"NEW\"", gotState)
	}
}

// TestChangeRequestIntegration_PatchAssessWithTeamAlreadyOnRecordSucceeds
// confirms the compulsory gate accepts a team set by an earlier, separate
// PATCH (the real flow: the Edit dialog saves assignedTeamId first, then
// "Move to Assess" is sent as its own request with no assignedTeamId at
// all) -- not just a team supplied in the very same request.
func TestChangeRequestIntegration_PatchAssessWithTeamAlreadyOnRecordSucceeds(t *testing.T) {
	dsn := os.Getenv("CHANGE_REQUEST_TEST_DSN")
	if dsn == "" {
		t.Skip("CHANGE_REQUEST_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	scoped := repository.NewScoped(pool)
	sys := repository.WithSystemIdentity(context.Background())
	repo := repository.NewChangeRequestRepository(scoped)
	seedChangeRequestForAssessGateTest(t, scoped)
	seedAssessGateGroup(t, pool)
	// A non-empty group: the Assess provisioning path now rejects an empty
	// one outright (see TestChangeRequestIntegration_PatchAssessRejectsEmptyGroup),
	// so this test -- about the team-already-on-record path specifically --
	// needs a real member for its own second PatchChangeRequest call to
	// reach "succeeds" at all.
	seedTeamMembersForAssessGateTest(t, pool, changeRequestAssessGateMemberUserID)
	t.Cleanup(func() {
		_, _ = scoped.Exec(sys, `DELETE FROM approval_stage WHERE work_item_id = $1`, changeRequestAssessGateTestID)
	})

	teamID := changeRequestAssessGateGroupID
	if _, err := repo.PatchChangeRequest(sys, changeRequestAssessGateTestID,
		domain.PatchChangeRequestRequest{AssignedTeamID: &teamID}, "cr-assess-gate-test"); err != nil {
		t.Fatalf("PatchChangeRequest(assignedTeamId=%s): %v", teamID, err)
	}

	assess := domain.ChangeRequestStateAssess
	updated, err := repo.PatchChangeRequest(sys, changeRequestAssessGateTestID,
		domain.PatchChangeRequestRequest{State: &assess}, "cr-assess-gate-test")
	if err != nil {
		t.Fatalf("PatchChangeRequest(state=assess) with a team already on the record: %v", err)
	}
	if updated.State == nil || *updated.State != string(assess) {
		t.Fatalf("state after patch = %v, want %q", updated.State, assess)
	}
}

// TestChangeRequestIntegration_PatchAssessProvisionsApproversFromGroupMembers
// is the regression guard for the auto-provisioning feature: the moment a
// change request enters Assess, every team_member row keyed to the assigned
// team's group_id must become a Requested approval_stage_approver on a
// freshly created Assess-stage approval_stage -- not an empty Approvals tab
// with nobody to approve it.
func TestChangeRequestIntegration_PatchAssessProvisionsApproversFromGroupMembers(t *testing.T) {
	dsn := os.Getenv("CHANGE_REQUEST_TEST_DSN")
	if dsn == "" {
		t.Skip("CHANGE_REQUEST_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	scoped := repository.NewScoped(pool)
	sys := repository.WithSystemIdentity(context.Background())
	repo := repository.NewChangeRequestRepository(scoped)
	seedChangeRequestForAssessGateTest(t, scoped)
	seedAssessGateGroup(t, pool)
	seedTeamMembersForAssessGateTest(t, pool, changeRequestAssessGateMemberUserID, changeRequestAssessGateMemberUserID2)
	t.Cleanup(func() {
		_, _ = scoped.Exec(sys, `DELETE FROM approval_stage WHERE work_item_id = $1`, changeRequestAssessGateTestID)
	})

	teamID := changeRequestAssessGateGroupID
	assess := domain.ChangeRequestStateAssess
	if _, err := repo.PatchChangeRequest(sys, changeRequestAssessGateTestID,
		domain.PatchChangeRequestRequest{AssignedTeamID: &teamID, State: &assess}, "cr-assess-gate-test"); err != nil {
		t.Fatalf("PatchChangeRequest(assignedTeamId=%s, state=assess): %v", teamID, err)
	}

	var stageID, stageGroupID string
	if scanErr := scoped.QueryRow(sys,
		`SELECT id, assignment_group_id::TEXT FROM approval_stage WHERE work_item_id = $1`, changeRequestAssessGateTestID).
		Scan(&stageID, &stageGroupID); scanErr != nil {
		t.Fatalf("read back approval_stage: %v", scanErr)
	}
	if stageGroupID != teamID {
		t.Fatalf("approval_stage.assignment_group_id = %q, want %q", stageGroupID, teamID)
	}

	rows, err := scoped.Query(sys,
		`SELECT approver_user_id::TEXT, state FROM approval_stage_approver WHERE stage_id = $1 ORDER BY approver_user_id`, stageID)
	if err != nil {
		t.Fatalf("query approval_stage_approver: %v", err)
	}
	defer rows.Close()
	gotApprovers := map[string]string{}
	for rows.Next() {
		var uid, status string
		if err := rows.Scan(&uid, &status); err != nil {
			t.Fatalf("scan approval_stage_approver: %v", err)
		}
		gotApprovers[uid] = status
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("approval_stage_approver rows: %v", err)
	}

	want := map[string]string{
		changeRequestAssessGateMemberUserID:  "REQUESTED",
		changeRequestAssessGateMemberUserID2: "REQUESTED",
	}
	if len(gotApprovers) != len(want) {
		t.Fatalf("approval_stage_approver rows = %+v, want exactly %+v", gotApprovers, want)
	}
	for uid, wantStatus := range want {
		if gotApprovers[uid] != wantStatus {
			t.Fatalf("approver %s status = %q, want %q", uid, gotApprovers[uid], wantStatus)
		}
	}
}

// TestChangeRequestIntegration_PatchAssessDoesNotReprovisionWhenStageExists
// confirms a second {state: "assess"} patch against a change request that
// already has an approval_stage (e.g. a resend, or an unrelated field edit
// sent while already in Assess) never creates a duplicate stage or
// re-seeds approvers -- DecideChangeRequestApproval owns everything about
// an existing stage from the moment it's created.
func TestChangeRequestIntegration_PatchAssessDoesNotReprovisionWhenStageExists(t *testing.T) {
	dsn := os.Getenv("CHANGE_REQUEST_TEST_DSN")
	if dsn == "" {
		t.Skip("CHANGE_REQUEST_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	scoped := repository.NewScoped(pool)
	sys := repository.WithSystemIdentity(context.Background())
	repo := repository.NewChangeRequestRepository(scoped)
	seedChangeRequestForAssessGateTest(t, scoped)
	seedAssessGateGroup(t, pool)
	seedTeamMembersForAssessGateTest(t, pool, changeRequestAssessGateMemberUserID, changeRequestAssessGateMemberUserID2)
	t.Cleanup(func() {
		_, _ = scoped.Exec(sys, `DELETE FROM approval_stage WHERE work_item_id = $1`, changeRequestAssessGateTestID)
	})

	teamID := changeRequestAssessGateGroupID
	assess := domain.ChangeRequestStateAssess
	if _, err := repo.PatchChangeRequest(sys, changeRequestAssessGateTestID,
		domain.PatchChangeRequestRequest{AssignedTeamID: &teamID, State: &assess}, "cr-assess-gate-test"); err != nil {
		t.Fatalf("PatchChangeRequest(assignedTeamId=%s, state=assess) #1: %v", teamID, err)
	}
	if _, err := repo.PatchChangeRequest(sys, changeRequestAssessGateTestID,
		domain.PatchChangeRequestRequest{State: &assess}, "cr-assess-gate-test"); err != nil {
		t.Fatalf("PatchChangeRequest(state=assess) #2 (resend): %v", err)
	}

	var stageCount int
	if scanErr := scoped.QueryRow(sys,
		`SELECT COUNT(*) FROM approval_stage WHERE work_item_id = $1`, changeRequestAssessGateTestID).Scan(&stageCount); scanErr != nil {
		t.Fatalf("count approval_stage: %v", scanErr)
	}
	if stageCount != 1 {
		t.Fatalf("approval_stage rows after two assess patches = %d, want exactly 1", stageCount)
	}

	var approverCount int
	if scanErr := scoped.QueryRow(sys,
		`SELECT COUNT(*) FROM approval_stage_approver asa JOIN approval_stage ast ON ast.id = asa.stage_id WHERE ast.work_item_id = $1`,
		changeRequestAssessGateTestID).Scan(&approverCount); scanErr != nil {
		t.Fatalf("count approval_stage_approver: %v", scanErr)
	}
	if approverCount != 2 {
		t.Fatalf("approval_stage_approver rows after two assess patches = %d, want exactly 2 (not re-seeded)", approverCount)
	}
}

// TestChangeRequestIntegration_PatchAssessRejectsEmptyGroup is the
// regression guard for a CodeRabbit-caught gap: the approval_stage used to
// be created before team_member was ever queried, so an assigned team with
// no members still committed an empty, un-approvable stage -- the change
// request would be stuck in Assess forever, since "no approval_stage exists
// yet" is exactly the condition that gates (re-)provisioning. A team with no
// members must instead reject the whole PATCH with a ValidationError and
// leave no approval_stage behind at all.
func TestChangeRequestIntegration_PatchAssessRejectsEmptyGroup(t *testing.T) {
	dsn := os.Getenv("CHANGE_REQUEST_TEST_DSN")
	if dsn == "" {
		t.Skip("CHANGE_REQUEST_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	scoped := repository.NewScoped(pool)
	sys := repository.WithSystemIdentity(context.Background())
	repo := repository.NewChangeRequestRepository(scoped)
	seedChangeRequestForAssessGateTest(t, scoped)
	seedAssessGateGroup(t, pool)
	// The local seed populates the Devops Approval peer fallback group, which
	// would rescue an empty assigned team; this test is about nobody being
	// left, so the fallback group is emptied for its duration.
	isolateGroupsNamed(t, scoped, domain.PeerApprovalFallbackGroupName)
	// Deliberately no seedTeamMembersForAssessGateTest call -- the group
	// exists (so assignedTeamId itself is valid) but has zero members.
	t.Cleanup(func() {
		_, _ = scoped.Exec(sys, `DELETE FROM approval_stage WHERE work_item_id = $1`, changeRequestAssessGateTestID)
	})

	teamID := changeRequestAssessGateGroupID
	assess := domain.ChangeRequestStateAssess
	_, err = repo.PatchChangeRequest(sys, changeRequestAssessGateTestID,
		domain.PatchChangeRequestRequest{AssignedTeamID: &teamID, State: &assess}, "cr-assess-gate-test")
	if err == nil {
		t.Fatal("PatchChangeRequest(state=assess) with an empty assigned team succeeded, want a ValidationError")
	}
	var valErr *apierror.ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("PatchChangeRequest(state=assess) with an empty assigned team error = %v (%T), want *apierror.ValidationError", err, err)
	}

	var gotState string
	if scanErr := scoped.QueryRow(sys,
		`SELECT state::TEXT FROM change_request WHERE id = $1`, changeRequestAssessGateTestID).Scan(&gotState); scanErr != nil {
		t.Fatalf("read back state: %v", scanErr)
	}
	if gotState != "NEW" {
		t.Fatalf("state after a rejected assess patch = %q, want unchanged \"NEW\"", gotState)
	}

	var stageCount int
	if scanErr := scoped.QueryRow(sys,
		`SELECT COUNT(*) FROM approval_stage WHERE work_item_id = $1`, changeRequestAssessGateTestID).Scan(&stageCount); scanErr != nil {
		t.Fatalf("count approval_stage: %v", scanErr)
	}
	if stageCount != 0 {
		t.Fatalf("approval_stage rows after a rejected assess patch = %d, want 0 (no empty stage left behind)", stageCount)
	}
}

// TestChangeRequestIntegration_PatchAssessDeduplicatesGroupMembers is the
// regression guard for a second CodeRabbit catch: team_member has no unique
// constraint on (user_id, group_id), so a duplicated membership row must
// still provision exactly one requested approval_stage_approver per person,
// never two.
func TestChangeRequestIntegration_PatchAssessDeduplicatesGroupMembers(t *testing.T) {
	dsn := os.Getenv("CHANGE_REQUEST_TEST_DSN")
	if dsn == "" {
		t.Skip("CHANGE_REQUEST_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	scoped := repository.NewScoped(pool)
	sys := repository.WithSystemIdentity(context.Background())
	repo := repository.NewChangeRequestRepository(scoped)
	seedChangeRequestForAssessGateTest(t, scoped)
	seedAssessGateGroup(t, pool)
	seedTeamMembersForAssessGateTest(t, pool, changeRequestAssessGateMemberUserID)
	t.Cleanup(func() {
		_, _ = scoped.Exec(sys, `DELETE FROM approval_stage WHERE work_item_id = $1`, changeRequestAssessGateTestID)
	})

	// A second team_member row for the SAME user against the SAME group --
	// seedTeamMembersForAssessGateTest's own cleanup (DELETE ... WHERE
	// user_id = $1) already covers this row too, since it shares the user id.
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO team_member (id, created_on, updated_on, created_by, updated_by, team_id, user_id, group_id)
		 VALUES (gen_random_uuid(), now(), now(), 'cr-assess-gate-test', 'cr-assess-gate-test', $1::uuid, $2, $3::uuid)`,
		seededGroupID, changeRequestAssessGateMemberUserID, changeRequestAssessGateGroupID); err != nil {
		t.Fatalf("seed duplicate team_member row: %v", err)
	}

	teamID := changeRequestAssessGateGroupID
	assess := domain.ChangeRequestStateAssess
	if _, err := repo.PatchChangeRequest(sys, changeRequestAssessGateTestID,
		domain.PatchChangeRequestRequest{AssignedTeamID: &teamID, State: &assess}, "cr-assess-gate-test"); err != nil {
		t.Fatalf("PatchChangeRequest(assignedTeamId=%s, state=assess): %v", teamID, err)
	}

	var approverCount int
	if scanErr := scoped.QueryRow(sys,
		`SELECT COUNT(*) FROM approval_stage_approver asa JOIN approval_stage ast ON ast.id = asa.stage_id WHERE ast.work_item_id = $1`,
		changeRequestAssessGateTestID).Scan(&approverCount); scanErr != nil {
		t.Fatalf("count approval_stage_approver: %v", scanErr)
	}
	if approverCount != 1 {
		t.Fatalf("approval_stage_approver rows for a user with a duplicated team_member row = %d, want exactly 1", approverCount)
	}
}

// setAssessGateRequestedBy stamps change_request.requested_by_user_id for
// changeRequestAssessGateTestID -- seedChangeRequestForAssessGateTest itself
// leaves this column NULL (no test before the requester-self-approval tests
// below cared who "requested" the change), so those tests set it explicitly
// after seeding. userID must already be a real "user" row (the column is a
// real FK) -- callers pass one of seedTeamMembersForAssessGateTest's own
// seeded ids.
func setAssessGateRequestedBy(t *testing.T, pool *repository.Scoped, userID string) {
	t.Helper()
	ctx := repository.WithSystemIdentity(context.Background())
	if _, err := pool.Exec(ctx,
		`UPDATE change_request SET requested_by_user_id = $1::uuid WHERE id = $2`,
		userID, changeRequestAssessGateTestID); err != nil {
		t.Fatalf("set change_request.requested_by_user_id: %v", err)
	}
}

// TestChangeRequestIntegration_PatchAssessProvisionsRequesterAsCancelled is
// the regression guard for ServiceNow's real, confirmed self-approval
// prevention: when the change request's own requested_by_user_id is also a
// member of the assigned team, that person's approval_stage_approver row
// must be provisioned already "cancelled" (mirroring the same state
// ServiceNow itself assigns at creation -- confirmed live against
// CHG0039122's own activity log, whose very first "Field changes" entry
// already shows State: Cancelled, not a later transition from Requested),
// while every other team member's row stays "requested" exactly as before.
func TestChangeRequestIntegration_PatchAssessProvisionsRequesterAsCancelled(t *testing.T) {
	dsn := os.Getenv("CHANGE_REQUEST_TEST_DSN")
	if dsn == "" {
		t.Skip("CHANGE_REQUEST_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	scoped := repository.NewScoped(pool)
	sys := repository.WithSystemIdentity(context.Background())
	repo := repository.NewChangeRequestRepository(scoped)
	seedChangeRequestForAssessGateTest(t, scoped)
	seedAssessGateGroup(t, pool)
	seedTeamMembersForAssessGateTest(t, pool, changeRequestAssessGateMemberUserID, changeRequestAssessGateMemberUserID2)
	setAssessGateRequestedBy(t, scoped, changeRequestAssessGateMemberUserID)
	t.Cleanup(func() {
		_, _ = scoped.Exec(sys, `DELETE FROM approval_stage WHERE work_item_id = $1`, changeRequestAssessGateTestID)
	})

	teamID := changeRequestAssessGateGroupID
	assess := domain.ChangeRequestStateAssess
	if _, err := repo.PatchChangeRequest(sys, changeRequestAssessGateTestID,
		domain.PatchChangeRequestRequest{AssignedTeamID: &teamID, State: &assess}, "cr-assess-gate-test"); err != nil {
		t.Fatalf("PatchChangeRequest(assignedTeamId=%s, state=assess): %v", teamID, err)
	}

	var stageID string
	if scanErr := scoped.QueryRow(sys,
		`SELECT id FROM approval_stage WHERE work_item_id = $1`, changeRequestAssessGateTestID).Scan(&stageID); scanErr != nil {
		t.Fatalf("read back approval_stage: %v", scanErr)
	}

	rows, err := scoped.Query(sys,
		`SELECT approver_user_id::TEXT, state FROM approval_stage_approver WHERE stage_id = $1`, stageID)
	if err != nil {
		t.Fatalf("query approval_stage_approver: %v", err)
	}
	defer rows.Close()
	gotApprovers := map[string]string{}
	for rows.Next() {
		var uid, status string
		if err := rows.Scan(&uid, &status); err != nil {
			t.Fatalf("scan approval_stage_approver: %v", err)
		}
		gotApprovers[uid] = status
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("approval_stage_approver rows: %v", err)
	}

	want := map[string]string{
		changeRequestAssessGateMemberUserID:  "CANCELLED",
		changeRequestAssessGateMemberUserID2: "REQUESTED",
	}
	if len(gotApprovers) != len(want) {
		t.Fatalf("approval_stage_approver rows = %+v, want exactly %+v", gotApprovers, want)
	}
	for uid, wantStatus := range want {
		if gotApprovers[uid] != wantStatus {
			t.Fatalf("approver %s status = %q, want %q", uid, gotApprovers[uid], wantStatus)
		}
	}
}

// TestChangeRequestIntegration_PatchAssessRejectsWhenOnlyMemberIsRequester is
// the regression guard for the edge case the requester-exclusion rule above
// can reintroduce: a dead-end approval_stage nobody can ever approve, this
// time because the assigned team's only member is the change request's own
// requester, so excluding them leaves zero requested approvers. Same
// "validate BEFORE the stage is created" discipline as
// TestChangeRequestIntegration_PatchAssessRejectsEmptyGroup -- the whole
// {state: "assess"} PATCH must be rejected with a ValidationError and leave
// no approval_stage row behind at all.
func TestChangeRequestIntegration_PatchAssessRejectsWhenOnlyMemberIsRequester(t *testing.T) {
	dsn := os.Getenv("CHANGE_REQUEST_TEST_DSN")
	if dsn == "" {
		t.Skip("CHANGE_REQUEST_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	scoped := repository.NewScoped(pool)
	sys := repository.WithSystemIdentity(context.Background())
	repo := repository.NewChangeRequestRepository(scoped)
	seedChangeRequestForAssessGateTest(t, scoped)
	seedAssessGateGroup(t, pool)
	seedTeamMembersForAssessGateTest(t, pool, changeRequestAssessGateMemberUserID)
	setAssessGateRequestedBy(t, scoped, changeRequestAssessGateMemberUserID)
	// As above: no seeded Devops Approval fallback for the duration.
	isolateGroupsNamed(t, scoped, domain.PeerApprovalFallbackGroupName)
	t.Cleanup(func() {
		_, _ = scoped.Exec(sys, `DELETE FROM approval_stage WHERE work_item_id = $1`, changeRequestAssessGateTestID)
	})

	teamID := changeRequestAssessGateGroupID
	assess := domain.ChangeRequestStateAssess
	_, err = repo.PatchChangeRequest(sys, changeRequestAssessGateTestID,
		domain.PatchChangeRequestRequest{AssignedTeamID: &teamID, State: &assess}, "cr-assess-gate-test")
	if err == nil {
		t.Fatal("PatchChangeRequest(state=assess) whose only team member is the requester succeeded, want a ValidationError")
	}
	var valErr *apierror.ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("PatchChangeRequest(state=assess) whose only team member is the requester error = %v (%T), want *apierror.ValidationError", err, err)
	}

	var gotState string
	if scanErr := scoped.QueryRow(sys,
		`SELECT state::TEXT FROM change_request WHERE id = $1`, changeRequestAssessGateTestID).Scan(&gotState); scanErr != nil {
		t.Fatalf("read back state: %v", scanErr)
	}
	if gotState != "NEW" {
		t.Fatalf("state after a rejected assess patch = %q, want unchanged \"NEW\"", gotState)
	}

	var stageCount int
	if scanErr := scoped.QueryRow(sys,
		`SELECT COUNT(*) FROM approval_stage WHERE work_item_id = $1`, changeRequestAssessGateTestID).Scan(&stageCount); scanErr != nil {
		t.Fatalf("count approval_stage: %v", scanErr)
	}
	if stageCount != 0 {
		t.Fatalf("approval_stage rows after a rejected assess patch = %d, want 0 (no dead-end stage left behind)", stageCount)
	}
}

// --- Authorize checkpoint ("Risk approvals" in real ServiceNow) ---
//
// Mirrors the Assess-gate tests above exactly, with one structural
// difference: provisionApprovalStage only creates a stage when the number of
// approval_stage rows already on the work item equals the checkpoint's own
// ordinal position (0 for Assess, 1 for Authorize -- see that function's own
// doc comment for why). Every test below therefore seeds a single,
// pre-existing Assess-position stage first (seedExistingApprovalStage,
// bypassing provisionApprovalStage entirely -- its own approvers are never
// queried by these tests), so the {state: "authorize"} PATCH under test
// lands its own new stage at position 1, the ordinal
// changeRequestApprovalStagePosition reads as "Authorize".

// seedExistingApprovalStage inserts a single approval_stage row directly --
// bypassing provisionApprovalStage entirely -- so a test can establish "this
// work item already has an earlier checkpoint's own stage" as a
// precondition, without caring about that earlier stage's own approvers.
// assignment_group_id is seededGroupID purely for FK validity (see that
// constant's own doc comment on why it's safe to reuse for exactly this) --
// no test that calls this ever queries that group's members.
func seedExistingApprovalStage(t *testing.T, pool *repository.Scoped, workItemID string) {
	t.Helper()
	ctx := repository.WithSystemIdentity(context.Background())
	if _, err := pool.Exec(ctx,
		`INSERT INTO approval_stage (id, created_on, updated_on, created_by, updated_by, work_item_id, assignment_group_id)
		 VALUES (gen_random_uuid(), now(), now(), 'cr-authorize-gate-test', 'cr-authorize-gate-test', $1, $2::uuid)`,
		workItemID, seededGroupID); err != nil {
		t.Fatalf("seed existing (Assess-position) approval_stage: %v", err)
	}
}

// seedChangeRequestForReviewGateTest inserts a minimal work_item/
// change_request pair already sitting in Implement -- the realistic
// starting point for every Review-gate test below, since in production
// Review is only ever reached after Assess and Authorize have already run
// (there is no cascading approval-decision mechanism past Authorize, so
// Review is reached exclusively via a direct {state: "review"} PATCH along
// the main ...->Scheduled->Implement->Review path -- see
// patchChangeRequestTx's own Review-branch comment).
func seedChangeRequestForReviewGateTest(t *testing.T, pool *repository.Scoped) {
	t.Helper()
	// work_item and change_request are RLS-protected; seed/cleanup as internal.
	ctx := repository.WithSystemIdentity(context.Background())

	cleanup := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM work_item WHERE id = $1`, changeRequestReviewGateTestID)
	}
	cleanup()
	t.Cleanup(cleanup)

	if _, err := pool.Exec(ctx,
		`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, subject, type)
		 VALUES ($1, now(), now(), 'cr-review-gate-test-creator', 'cr-review-gate-test', 'CRREV001', 'review gate test', 'CHANGE_REQUEST')`,
		changeRequestReviewGateTestID); err != nil {
		t.Fatalf("seed work_item: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO change_request (id, state) VALUES ($1, 'IMPLEMENT'::change_request_state_enum)`,
		changeRequestReviewGateTestID); err != nil {
		t.Fatalf("seed change_request: %v", err)
	}
}

// seedReviewGateGroup inserts changeRequestReviewGateGroupID's own "group"
// row -- a dedicated fixture for the tests below, deliberately not shared
// with changeRequestAssessGateGroupID, changeRequestAuthorizeGateGroupID or
// seededGroupID (same isolation reasoning as changeRequestAssessGateGroupID's
// own doc comment).
func seedReviewGateGroup(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()

	cleanup := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM "group" WHERE id = $1`, changeRequestReviewGateGroupID)
	}
	cleanup()
	t.Cleanup(cleanup)

	if _, err := pool.Exec(ctx,
		`INSERT INTO "group" (id, created_on, updated_on, created_by, updated_by, name)
		 VALUES ($1, now(), now(), 'cr-review-gate-test', 'cr-review-gate-test', 'Review Gate Test Group')`,
		changeRequestReviewGateGroupID); err != nil {
		t.Fatalf("seed group: %v", err)
	}
}

// seedTeamMembersForReviewGateTest inserts one "user" row and one
// team_member row (keyed by group_id, same reasoning as
// seedTeamMembersForAssessGateTest/seedTeamMembersForAuthorizeGateTest) per
// given user id, so they resolve as members of changeRequestReviewGateGroupID
// for the auto-provisioning tests below. Callers must seed that group row
// first (seedReviewGateGroup).
func seedTeamMembersForReviewGateTest(t *testing.T, pool *pgxpool.Pool, userIDs ...string) {
	t.Helper()
	ctx := context.Background()

	for i, userID := range userIDs {
		id := userID
		userCleanup := func() {
			_, _ = pool.Exec(ctx, `DELETE FROM "user" WHERE id = $1`, id)
		}
		userCleanup()
		t.Cleanup(userCleanup)

		email := fmt.Sprintf("cr-review-gate-member-%d@example.com", i+1)
		if _, err := pool.Exec(ctx,
			`INSERT INTO "user" (id, created_on, updated_on, created_by, updated_by, user_name, name, first_name, last_name, email, is_active, is_system_user)
			 VALUES ($1, now(), now(), 'cr-review-gate-test', 'cr-review-gate-test', $2, 'Review Gate Member', 'Review', 'Gate Member', $2, true, false)`,
			id, email); err != nil {
			t.Fatalf("seed team member user %s: %v", id, err)
		}
		setTestUserType(t, pool, ctx, id, userTypeInternal)

		memberCleanup := func() {
			_, _ = pool.Exec(ctx, `DELETE FROM team_member WHERE user_id = $1`, id)
		}
		memberCleanup()
		t.Cleanup(memberCleanup)

		// team_member.team_id is NOT NULL -- same seededGroupID-as-filler
		// reasoning as seedTeamMembersForAssessGateTest's own identical line.
		if _, err := pool.Exec(ctx,
			`INSERT INTO team_member (id, created_on, updated_on, created_by, updated_by, team_id, user_id, group_id)
			 VALUES (gen_random_uuid(), now(), now(), 'cr-review-gate-test', 'cr-review-gate-test', $1::uuid, $2, $3::uuid)`,
			seededGroupID, id, changeRequestReviewGateGroupID); err != nil {
			t.Fatalf("seed team_member for user %s: %v", id, err)
		}
	}
}

// setReviewGateRequestedBy stamps change_request.requested_by_user_id for
// changeRequestReviewGateTestID, mirroring setAssessGateRequestedBy/
// setAuthorizeGateRequestedBy.
func setReviewGateRequestedBy(t *testing.T, pool *repository.Scoped, userID string) {
	t.Helper()
	ctx := repository.WithSystemIdentity(context.Background())
	if _, err := pool.Exec(ctx,
		`UPDATE change_request SET requested_by_user_id = $1::uuid WHERE id = $2`,
		userID, changeRequestReviewGateTestID); err != nil {
		t.Fatalf("set change_request.requested_by_user_id: %v", err)
	}
}

// TestChangeRequestIntegration_PatchReviewProvisionsApproversFromGroupMembers
// is the Review-checkpoint counterpart of
// TestChangeRequestIntegration_PatchAuthorizeProvisionsApproversFromGroupMembers:
// a {state: "review"} PATCH against a change request that already has its
// Assess- and Authorize-position stages provisions one requested
// approval_stage_approver row per member of the assigned team, at a new,
// third approval_stage.
func TestChangeRequestIntegration_PatchReviewProvisionsApproversFromGroupMembers(t *testing.T) {
	dsn := os.Getenv("CHANGE_REQUEST_TEST_DSN")
	if dsn == "" {
		t.Skip("CHANGE_REQUEST_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	scoped := repository.NewScoped(pool)
	sys := repository.WithSystemIdentity(context.Background())
	repo := repository.NewChangeRequestRepository(scoped)
	seedChangeRequestForReviewGateTest(t, scoped)
	// Two pre-existing stages -- standing in for the Assess- and
	// Authorize-position stages a real change request would already carry by
	// the time it reaches Review -- so provisionApprovalStage's own
	// COUNT(*) == checkpoint.Position gate (2) is satisfied.
	seedExistingApprovalStage(t, scoped, changeRequestReviewGateTestID)
	seedExistingApprovalStage(t, scoped, changeRequestReviewGateTestID)
	seedReviewGateGroup(t, pool)
	seedTeamMembersForReviewGateTest(t, pool, changeRequestReviewGateMemberUserID, changeRequestReviewGateMemberUserID2)
	t.Cleanup(func() {
		_, _ = scoped.Exec(sys, `DELETE FROM approval_stage WHERE work_item_id = $1`, changeRequestReviewGateTestID)
	})

	teamID := changeRequestReviewGateGroupID
	review := domain.ChangeRequestStateReview
	if _, err := repo.PatchChangeRequest(sys, changeRequestReviewGateTestID,
		domain.PatchChangeRequestRequest{AssignedTeamID: &teamID, State: &review}, "cr-review-gate-test"); err != nil {
		t.Fatalf("PatchChangeRequest(assignedTeamId=%s, state=review): %v", teamID, err)
	}

	var stageCount int
	if scanErr := scoped.QueryRow(sys,
		`SELECT COUNT(*) FROM approval_stage WHERE work_item_id = $1`, changeRequestReviewGateTestID).Scan(&stageCount); scanErr != nil {
		t.Fatalf("count approval_stage: %v", scanErr)
	}
	if stageCount != 3 {
		t.Fatalf("approval_stage rows after a review patch = %d, want exactly 3 (the two pre-existing stages plus the new Review stage)", stageCount)
	}

	var stageID, stageGroupID string
	if scanErr := scoped.QueryRow(sys,
		`SELECT id, assignment_group_id::TEXT FROM approval_stage WHERE work_item_id = $1 AND assignment_group_id = $2::uuid`,
		changeRequestReviewGateTestID, teamID).Scan(&stageID, &stageGroupID); scanErr != nil {
		t.Fatalf("read back review approval_stage: %v", scanErr)
	}
	if stageGroupID != teamID {
		t.Fatalf("approval_stage.assignment_group_id = %q, want %q", stageGroupID, teamID)
	}

	rows, err := scoped.Query(sys,
		`SELECT approver_user_id::TEXT, state FROM approval_stage_approver WHERE stage_id = $1 ORDER BY approver_user_id`, stageID)
	if err != nil {
		t.Fatalf("query approval_stage_approver: %v", err)
	}
	defer rows.Close()
	gotApprovers := map[string]string{}
	for rows.Next() {
		var uid, status string
		if err := rows.Scan(&uid, &status); err != nil {
			t.Fatalf("scan approval_stage_approver: %v", err)
		}
		gotApprovers[uid] = status
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("approval_stage_approver rows: %v", err)
	}

	want := map[string]string{
		changeRequestReviewGateMemberUserID:  "REQUESTED",
		changeRequestReviewGateMemberUserID2: "REQUESTED",
	}
	if len(gotApprovers) != len(want) {
		t.Fatalf("approval_stage_approver rows = %+v, want exactly %+v", gotApprovers, want)
	}
	for uid, wantStatus := range want {
		if gotApprovers[uid] != wantStatus {
			t.Fatalf("approver %s status = %q, want %q", uid, gotApprovers[uid], wantStatus)
		}
	}
}

// TestChangeRequestIntegration_PatchReviewDoesNotReprovisionWhenStageExists
// is the Review-checkpoint counterpart of
// TestChangeRequestIntegration_PatchAuthorizeDoesNotReprovisionWhenStageExists:
// a second {state: "review"} patch against a change request that already has
// its Review-position stage must never create a duplicate stage or re-seed
// approvers.
func TestChangeRequestIntegration_PatchReviewDoesNotReprovisionWhenStageExists(t *testing.T) {
	dsn := os.Getenv("CHANGE_REQUEST_TEST_DSN")
	if dsn == "" {
		t.Skip("CHANGE_REQUEST_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	scoped := repository.NewScoped(pool)
	sys := repository.WithSystemIdentity(context.Background())
	repo := repository.NewChangeRequestRepository(scoped)
	seedChangeRequestForReviewGateTest(t, scoped)
	seedExistingApprovalStage(t, scoped, changeRequestReviewGateTestID)
	seedExistingApprovalStage(t, scoped, changeRequestReviewGateTestID)
	seedReviewGateGroup(t, pool)
	seedTeamMembersForReviewGateTest(t, pool, changeRequestReviewGateMemberUserID, changeRequestReviewGateMemberUserID2)
	t.Cleanup(func() {
		_, _ = scoped.Exec(sys, `DELETE FROM approval_stage WHERE work_item_id = $1`, changeRequestReviewGateTestID)
	})

	teamID := changeRequestReviewGateGroupID
	review := domain.ChangeRequestStateReview
	if _, err := repo.PatchChangeRequest(sys, changeRequestReviewGateTestID,
		domain.PatchChangeRequestRequest{AssignedTeamID: &teamID, State: &review}, "cr-review-gate-test"); err != nil {
		t.Fatalf("PatchChangeRequest(assignedTeamId=%s, state=review) #1: %v", teamID, err)
	}
	if _, err := repo.PatchChangeRequest(sys, changeRequestReviewGateTestID,
		domain.PatchChangeRequestRequest{State: &review}, "cr-review-gate-test"); err != nil {
		t.Fatalf("PatchChangeRequest(state=review) #2 (resend): %v", err)
	}

	var stageCount int
	if scanErr := scoped.QueryRow(sys,
		`SELECT COUNT(*) FROM approval_stage WHERE work_item_id = $1`, changeRequestReviewGateTestID).Scan(&stageCount); scanErr != nil {
		t.Fatalf("count approval_stage: %v", scanErr)
	}
	if stageCount != 3 {
		t.Fatalf("approval_stage rows after two review patches = %d, want exactly 3 (not reprovisioned)", stageCount)
	}

	var approverCount int
	if scanErr := scoped.QueryRow(sys,
		`SELECT COUNT(*) FROM approval_stage_approver asa JOIN approval_stage ast ON ast.id = asa.stage_id WHERE ast.work_item_id = $1`,
		changeRequestReviewGateTestID).Scan(&approverCount); scanErr != nil {
		t.Fatalf("count approval_stage_approver: %v", scanErr)
	}
	if approverCount != 2 {
		t.Fatalf("approval_stage_approver rows after two review patches = %d, want exactly 2 (not re-seeded)", approverCount)
	}
}

// TestChangeRequestIntegration_PatchReviewRejectsEmptyGroup is the
// Review-checkpoint counterpart of
// TestChangeRequestIntegration_PatchAuthorizeRejectsEmptyGroup: an assigned
// team with no members must reject the whole {state: "review"} PATCH with a
// ValidationError and leave no new approval_stage behind -- only the two
// pre-existing stages this test seeds up front.
func TestChangeRequestIntegration_PatchReviewRejectsEmptyGroup(t *testing.T) {
	dsn := os.Getenv("CHANGE_REQUEST_TEST_DSN")
	if dsn == "" {
		t.Skip("CHANGE_REQUEST_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	scoped := repository.NewScoped(pool)
	sys := repository.WithSystemIdentity(context.Background())
	repo := repository.NewChangeRequestRepository(scoped)
	seedChangeRequestForReviewGateTest(t, scoped)
	seedExistingApprovalStage(t, scoped, changeRequestReviewGateTestID)
	seedExistingApprovalStage(t, scoped, changeRequestReviewGateTestID)
	seedReviewGateGroup(t, pool)
	// Deliberately no seedTeamMembersForReviewGateTest call -- the group
	// exists (so assignedTeamId itself is valid) but has zero members.
	t.Cleanup(func() {
		_, _ = scoped.Exec(sys, `DELETE FROM approval_stage WHERE work_item_id = $1`, changeRequestReviewGateTestID)
	})

	teamID := changeRequestReviewGateGroupID
	review := domain.ChangeRequestStateReview
	_, err = repo.PatchChangeRequest(sys, changeRequestReviewGateTestID,
		domain.PatchChangeRequestRequest{AssignedTeamID: &teamID, State: &review}, "cr-review-gate-test")
	if err == nil {
		t.Fatal("PatchChangeRequest(state=review) with an empty assigned team succeeded, want a ValidationError")
	}
	var valErr *apierror.ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("PatchChangeRequest(state=review) with an empty assigned team error = %v (%T), want *apierror.ValidationError", err, err)
	}

	var stageCount int
	if scanErr := scoped.QueryRow(sys,
		`SELECT COUNT(*) FROM approval_stage WHERE work_item_id = $1`, changeRequestReviewGateTestID).Scan(&stageCount); scanErr != nil {
		t.Fatalf("count approval_stage: %v", scanErr)
	}
	if stageCount != 2 {
		t.Fatalf("approval_stage rows after a rejected review patch = %d, want exactly 2 (only the two pre-existing stages, no dead-end Review stage)", stageCount)
	}
}

// TestChangeRequestIntegration_PatchReviewDeduplicatesGroupMembers is the
// Review-checkpoint counterpart of
// TestChangeRequestIntegration_PatchAuthorizeDeduplicatesGroupMembers.
func TestChangeRequestIntegration_PatchReviewDeduplicatesGroupMembers(t *testing.T) {
	dsn := os.Getenv("CHANGE_REQUEST_TEST_DSN")
	if dsn == "" {
		t.Skip("CHANGE_REQUEST_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	scoped := repository.NewScoped(pool)
	sys := repository.WithSystemIdentity(context.Background())
	repo := repository.NewChangeRequestRepository(scoped)
	seedChangeRequestForReviewGateTest(t, scoped)
	seedExistingApprovalStage(t, scoped, changeRequestReviewGateTestID)
	seedExistingApprovalStage(t, scoped, changeRequestReviewGateTestID)
	seedReviewGateGroup(t, pool)
	seedTeamMembersForReviewGateTest(t, pool, changeRequestReviewGateMemberUserID)
	t.Cleanup(func() {
		_, _ = scoped.Exec(sys, `DELETE FROM approval_stage WHERE work_item_id = $1`, changeRequestReviewGateTestID)
	})

	// A second team_member row for the SAME user against the SAME group --
	// seedTeamMembersForReviewGateTest's own cleanup (DELETE ... WHERE
	// user_id = $1) already covers this row too, since it shares the user id.
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO team_member (id, created_on, updated_on, created_by, updated_by, team_id, user_id, group_id)
		 VALUES (gen_random_uuid(), now(), now(), 'cr-review-gate-test', 'cr-review-gate-test', $1::uuid, $2, $3::uuid)`,
		seededGroupID, changeRequestReviewGateMemberUserID, changeRequestReviewGateGroupID); err != nil {
		t.Fatalf("seed duplicate team_member row: %v", err)
	}

	teamID := changeRequestReviewGateGroupID
	review := domain.ChangeRequestStateReview
	if _, err := repo.PatchChangeRequest(sys, changeRequestReviewGateTestID,
		domain.PatchChangeRequestRequest{AssignedTeamID: &teamID, State: &review}, "cr-review-gate-test"); err != nil {
		t.Fatalf("PatchChangeRequest(assignedTeamId=%s, state=review): %v", teamID, err)
	}

	var approverCount int
	if scanErr := scoped.QueryRow(sys,
		`SELECT COUNT(*) FROM approval_stage_approver asa
		 JOIN approval_stage ast ON ast.id = asa.stage_id
		 WHERE ast.work_item_id = $1 AND ast.assignment_group_id = $2::uuid`,
		changeRequestReviewGateTestID, teamID).Scan(&approverCount); scanErr != nil {
		t.Fatalf("count approval_stage_approver: %v", scanErr)
	}
	if approverCount != 1 {
		t.Fatalf("approval_stage_approver rows for a user with a duplicated team_member row = %d, want exactly 1", approverCount)
	}
}

// TestChangeRequestIntegration_PatchReviewProvisionsRequesterAsCancelled is
// the Review-checkpoint counterpart of
// TestChangeRequestIntegration_PatchAuthorizeProvisionsRequesterAsCancelled --
// the identical self-approval-exclusion rule applies at this checkpoint too.
func TestChangeRequestIntegration_PatchReviewProvisionsRequesterAsCancelled(t *testing.T) {
	dsn := os.Getenv("CHANGE_REQUEST_TEST_DSN")
	if dsn == "" {
		t.Skip("CHANGE_REQUEST_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	scoped := repository.NewScoped(pool)
	sys := repository.WithSystemIdentity(context.Background())
	repo := repository.NewChangeRequestRepository(scoped)
	seedChangeRequestForReviewGateTest(t, scoped)
	seedExistingApprovalStage(t, scoped, changeRequestReviewGateTestID)
	seedExistingApprovalStage(t, scoped, changeRequestReviewGateTestID)
	seedReviewGateGroup(t, pool)
	seedTeamMembersForReviewGateTest(t, pool, changeRequestReviewGateMemberUserID, changeRequestReviewGateMemberUserID2)
	setReviewGateRequestedBy(t, scoped, changeRequestReviewGateMemberUserID)
	t.Cleanup(func() {
		_, _ = scoped.Exec(sys, `DELETE FROM approval_stage WHERE work_item_id = $1`, changeRequestReviewGateTestID)
	})

	teamID := changeRequestReviewGateGroupID
	review := domain.ChangeRequestStateReview
	if _, err := repo.PatchChangeRequest(sys, changeRequestReviewGateTestID,
		domain.PatchChangeRequestRequest{AssignedTeamID: &teamID, State: &review}, "cr-review-gate-test"); err != nil {
		t.Fatalf("PatchChangeRequest(assignedTeamId=%s, state=review): %v", teamID, err)
	}

	var stageID string
	if scanErr := scoped.QueryRow(sys,
		`SELECT id FROM approval_stage WHERE work_item_id = $1 AND assignment_group_id = $2::uuid`,
		changeRequestReviewGateTestID, teamID).Scan(&stageID); scanErr != nil {
		t.Fatalf("read back approval_stage: %v", scanErr)
	}

	rows, err := scoped.Query(sys,
		`SELECT approver_user_id::TEXT, state FROM approval_stage_approver WHERE stage_id = $1`, stageID)
	if err != nil {
		t.Fatalf("query approval_stage_approver: %v", err)
	}
	defer rows.Close()
	gotApprovers := map[string]string{}
	for rows.Next() {
		var uid, status string
		if err := rows.Scan(&uid, &status); err != nil {
			t.Fatalf("scan approval_stage_approver: %v", err)
		}
		gotApprovers[uid] = status
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("approval_stage_approver rows: %v", err)
	}

	want := map[string]string{
		changeRequestReviewGateMemberUserID:  "CANCELLED",
		changeRequestReviewGateMemberUserID2: "REQUESTED",
	}
	if len(gotApprovers) != len(want) {
		t.Fatalf("approval_stage_approver rows = %+v, want exactly %+v", gotApprovers, want)
	}
	for uid, wantStatus := range want {
		if gotApprovers[uid] != wantStatus {
			t.Fatalf("approver %s status = %q, want %q", uid, gotApprovers[uid], wantStatus)
		}
	}
}

// TestChangeRequestIntegration_PatchReviewRejectsWhenOnlyMemberIsRequester is
// the Review-checkpoint counterpart of
// TestChangeRequestIntegration_PatchAuthorizeRejectsWhenOnlyMemberIsRequester.
func TestChangeRequestIntegration_PatchReviewRejectsWhenOnlyMemberIsRequester(t *testing.T) {
	dsn := os.Getenv("CHANGE_REQUEST_TEST_DSN")
	if dsn == "" {
		t.Skip("CHANGE_REQUEST_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	scoped := repository.NewScoped(pool)
	sys := repository.WithSystemIdentity(context.Background())
	repo := repository.NewChangeRequestRepository(scoped)
	seedChangeRequestForReviewGateTest(t, scoped)
	seedExistingApprovalStage(t, scoped, changeRequestReviewGateTestID)
	seedExistingApprovalStage(t, scoped, changeRequestReviewGateTestID)
	seedReviewGateGroup(t, pool)
	seedTeamMembersForReviewGateTest(t, pool, changeRequestReviewGateMemberUserID)
	setReviewGateRequestedBy(t, scoped, changeRequestReviewGateMemberUserID)
	t.Cleanup(func() {
		_, _ = scoped.Exec(sys, `DELETE FROM approval_stage WHERE work_item_id = $1`, changeRequestReviewGateTestID)
	})

	teamID := changeRequestReviewGateGroupID
	review := domain.ChangeRequestStateReview
	_, err = repo.PatchChangeRequest(sys, changeRequestReviewGateTestID,
		domain.PatchChangeRequestRequest{AssignedTeamID: &teamID, State: &review}, "cr-review-gate-test")
	if err == nil {
		t.Fatal("PatchChangeRequest(state=review) whose only team member is the requester succeeded, want a ValidationError")
	}
	var valErr *apierror.ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("PatchChangeRequest(state=review) whose only team member is the requester error = %v (%T), want *apierror.ValidationError", err, err)
	}

	var stageCount int
	if scanErr := scoped.QueryRow(sys,
		`SELECT COUNT(*) FROM approval_stage WHERE work_item_id = $1`, changeRequestReviewGateTestID).Scan(&stageCount); scanErr != nil {
		t.Fatalf("count approval_stage: %v", scanErr)
	}
	if stageCount != 2 {
		t.Fatalf("approval_stage rows after a rejected review patch = %d, want exactly 2 (only the two pre-existing stages, no dead-end Review stage)", stageCount)
	}
}

// TestChangeRequestIntegration_AssessAuthorizeAndReviewStagesCoexist extends
// TestChangeRequestIntegration_AssessAndAuthorizeStagesCoexist's own real,
// full production path one checkpoint further: a {state: "assess"} PATCH
// (provisioning the Assess-position stage), an approval decision on it
// (DecideChangeRequestApproval's own Assess->Authorize cascade, which also
// provisions the Authorize-position stage as a best-effort step), and then a
// direct {state: "review"} PATCH -- the one real entry point Review's own
// provisioning has (see patchChangeRequestTx's own Review-branch comment) --
// provisioning a third, independent stage. Confirms all three stages land
// correctly and independently: the Assess stage's own approvers (one
// Approved, one Cancelled by the decision's own sibling-cancellation rule)
// and the Authorize stage's own approvers (both freshly "requested", reusing
// the Assess checkpoint's own assigned team) are both left completely
// untouched by the Review stage's own, separately-provisioned approvers
// (against a different, explicitly-supplied team), and each stage is
// attributed to its own, distinct id rather than any one clobbering another.
//
// The one thing that is NOT left untouched: the change left Authorize for
// Review without the CAB stage ever being decided, so its still-requested
// approvers are no longer actionable and are cancelled by the move
// (reconcileStaleApprovers) -- a stage's approvers can only act while the change
// is in the stage's own state.
func TestChangeRequestIntegration_AssessAuthorizeAndReviewStagesCoexist(t *testing.T) {
	dsn := os.Getenv("CHANGE_REQUEST_TEST_DSN")
	if dsn == "" {
		t.Skip("CHANGE_REQUEST_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	scoped := repository.NewScoped(pool)
	sys := repository.WithSystemIdentity(context.Background())
	repo := repository.NewChangeRequestRepository(scoped)
	seedChangeRequestForAssessGateTest(t, scoped)
	seedAssessGateGroup(t, pool)
	seedTeamMembersForAssessGateTest(t, pool, changeRequestAssessGateMemberUserID, changeRequestAssessGateMemberUserID2)
	seedReviewGateGroup(t, pool)
	seedTeamMembersForReviewGateTest(t, pool, changeRequestReviewGateMemberUserID, changeRequestReviewGateMemberUserID2)
	t.Cleanup(func() {
		_, _ = scoped.Exec(sys, `DELETE FROM approval_stage WHERE work_item_id = $1`, changeRequestAssessGateTestID)
	})

	assessTeamID := changeRequestAssessGateGroupID
	assess := domain.ChangeRequestStateAssess
	if _, err := repo.PatchChangeRequest(sys, changeRequestAssessGateTestID,
		domain.PatchChangeRequestRequest{AssignedTeamID: &assessTeamID, State: &assess}, "cr-assess-gate-test"); err != nil {
		t.Fatalf("PatchChangeRequest(assignedTeamId=%s, state=assess): %v", assessTeamID, err)
	}

	var assessStageID string
	if scanErr := scoped.QueryRow(sys,
		`SELECT id FROM approval_stage WHERE work_item_id = $1`, changeRequestAssessGateTestID).Scan(&assessStageID); scanErr != nil {
		t.Fatalf("read back Assess approval_stage: %v", scanErr)
	}

	if _, err := repo.DecideChangeRequestApproval(sys, changeRequestAssessGateTestID,
		changeRequestAssessGateMemberUserID, "approved", "cr-assess-gate-test"); err != nil {
		t.Fatalf("DecideChangeRequestApproval(approved): %v", err)
	}

	var gotState string
	if scanErr := scoped.QueryRow(sys,
		`SELECT state::TEXT FROM change_request WHERE id = $1`, changeRequestAssessGateTestID).Scan(&gotState); scanErr != nil {
		t.Fatalf("read back state after cascade: %v", scanErr)
	}
	if gotState != "AUTHORIZE" {
		t.Fatalf("state after approval = %q, want \"AUTHORIZE\"", gotState)
	}

	var authorizeStageID string
	if scanErr := scoped.QueryRow(sys,
		`SELECT id FROM approval_stage WHERE work_item_id = $1 AND id != $2`,
		changeRequestAssessGateTestID, assessStageID).Scan(&authorizeStageID); scanErr != nil {
		t.Fatalf("read back Authorize approval_stage: %v", scanErr)
	}

	// Review's own entry point: a direct {state: "review"} PATCH, supplying
	// a deliberately different assigned team so this test also confirms
	// Review's own provisioning uses THIS request's team, not whatever was
	// left on work_item by the Assess/Authorize steps above.
	reviewTeamID := changeRequestReviewGateGroupID
	review := domain.ChangeRequestStateReview
	if _, err := repo.PatchChangeRequest(sys, changeRequestAssessGateTestID,
		domain.PatchChangeRequestRequest{AssignedTeamID: &reviewTeamID, State: &review}, "cr-assess-gate-test"); err != nil {
		t.Fatalf("PatchChangeRequest(assignedTeamId=%s, state=review): %v", reviewTeamID, err)
	}

	var stageCount int
	if scanErr := scoped.QueryRow(sys,
		`SELECT COUNT(*) FROM approval_stage WHERE work_item_id = $1`, changeRequestAssessGateTestID).Scan(&stageCount); scanErr != nil {
		t.Fatalf("count approval_stage: %v", scanErr)
	}
	if stageCount != 3 {
		t.Fatalf("approval_stage rows after assess, the assess->authorize cascade, and a review patch = %d, want exactly 3", stageCount)
	}

	var reviewStageID string
	if scanErr := scoped.QueryRow(sys,
		`SELECT id FROM approval_stage WHERE work_item_id = $1 AND id != $2 AND id != $3`,
		changeRequestAssessGateTestID, assessStageID, authorizeStageID).Scan(&reviewStageID); scanErr != nil {
		t.Fatalf("read back Review approval_stage: %v", scanErr)
	}
	if reviewStageID == assessStageID || reviewStageID == authorizeStageID {
		t.Fatal("Review stage id collides with an earlier stage's id -- a new stage was not actually created")
	}

	// The Assess stage's own approvers: unchanged by anything that followed.
	assessApprovers := map[string]string{}
	assessRows, err := scoped.Query(sys,
		`SELECT approver_user_id::TEXT, state FROM approval_stage_approver WHERE stage_id = $1`, assessStageID)
	if err != nil {
		t.Fatalf("query Assess approval_stage_approver: %v", err)
	}
	for assessRows.Next() {
		var uid, status string
		if err := assessRows.Scan(&uid, &status); err != nil {
			assessRows.Close()
			t.Fatalf("scan Assess approval_stage_approver: %v", err)
		}
		assessApprovers[uid] = status
	}
	assessRows.Close()
	if err := assessRows.Err(); err != nil {
		t.Fatalf("Assess approval_stage_approver rows: %v", err)
	}
	wantAssess := map[string]string{
		changeRequestAssessGateMemberUserID:  "APPROVED",
		changeRequestAssessGateMemberUserID2: "CANCELLED",
	}
	if len(assessApprovers) != len(wantAssess) {
		t.Fatalf("Assess approval_stage_approver rows = %+v, want exactly %+v", assessApprovers, wantAssess)
	}
	for uid, wantStatus := range wantAssess {
		if assessApprovers[uid] != wantStatus {
			t.Fatalf("Assess approver %s status = %q, want %q", uid, assessApprovers[uid], wantStatus)
		}
	}

	// The CAB stage's own approvers: both CAB Approval members, requested
	// when it was provisioned and cancelled by the Review patch that followed
	// (the change left Authorize without the stage being decided).
	authorizeApprovers := map[string]string{}
	authorizeRows, err := scoped.Query(sys,
		`SELECT approver_user_id::TEXT, state FROM approval_stage_approver WHERE stage_id = $1`, authorizeStageID)
	if err != nil {
		t.Fatalf("query Authorize approval_stage_approver: %v", err)
	}
	for authorizeRows.Next() {
		var uid, status string
		if err := authorizeRows.Scan(&uid, &status); err != nil {
			authorizeRows.Close()
			t.Fatalf("scan Authorize approval_stage_approver: %v", err)
		}
		authorizeApprovers[uid] = status
	}
	authorizeRows.Close()
	if err := authorizeRows.Err(); err != nil {
		t.Fatalf("Authorize approval_stage_approver rows: %v", err)
	}
	wantAuthorize := map[string]string{
		crCABMemberUserID1: "CANCELLED",
		crCABMemberUserID2: "CANCELLED",
	}
	if len(authorizeApprovers) != len(wantAuthorize) {
		t.Fatalf("Authorize approval_stage_approver rows = %+v, want exactly %+v", authorizeApprovers, wantAuthorize)
	}
	for uid, wantStatus := range wantAuthorize {
		if authorizeApprovers[uid] != wantStatus {
			t.Fatalf("Authorize approver %s status = %q, want %q", uid, authorizeApprovers[uid], wantStatus)
		}
	}

	// The Review stage's own approvers: both members of the deliberately
	// different, explicitly-supplied review team, freshly "requested".
	reviewApprovers := map[string]string{}
	reviewRows, err := scoped.Query(sys,
		`SELECT approver_user_id::TEXT, state FROM approval_stage_approver WHERE stage_id = $1`, reviewStageID)
	if err != nil {
		t.Fatalf("query Review approval_stage_approver: %v", err)
	}
	for reviewRows.Next() {
		var uid, status string
		if err := reviewRows.Scan(&uid, &status); err != nil {
			reviewRows.Close()
			t.Fatalf("scan Review approval_stage_approver: %v", err)
		}
		reviewApprovers[uid] = status
	}
	reviewRows.Close()
	if err := reviewRows.Err(); err != nil {
		t.Fatalf("Review approval_stage_approver rows: %v", err)
	}
	wantReview := map[string]string{
		changeRequestReviewGateMemberUserID:  "REQUESTED",
		changeRequestReviewGateMemberUserID2: "REQUESTED",
	}
	if len(reviewApprovers) != len(wantReview) {
		t.Fatalf("Review approval_stage_approver rows = %+v, want exactly %+v", reviewApprovers, wantReview)
	}
	for uid, wantStatus := range wantReview {
		if reviewApprovers[uid] != wantStatus {
			t.Fatalf("Review approver %s status = %q, want %q", uid, reviewApprovers[uid], wantStatus)
		}
	}
}

// seedChangeRequestForOnHoldTest inserts a minimal work_item/change_request
// pair in the given state, with is_on_hold/on_hold_reason
// set directly via SQL rather than through PatchChangeRequest -- the tests
// below are split between exercising the GATE (which needs an "already on
// hold" precondition to exist before the PATCH under test ever runs) and the
// WRITE path itself (covered separately), so seeding the precondition
// directly keeps the two concerns from tangling. onHold=false seeds a
// never-been-on-hold record (is_on_hold FALSE, reason left NULL),
// matching every change request created before migration 0178 ever ran.
func seedChangeRequestForOnHoldTest(t *testing.T, pool *repository.Scoped, state string, onHold bool, reason *string) {
	t.Helper()
	// work_item and change_request are RLS-protected; seed/cleanup as internal.
	ctx := repository.WithSystemIdentity(context.Background())

	cleanup := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM work_item WHERE id = $1`, changeRequestOnHoldTestID)
	}
	cleanup()
	t.Cleanup(cleanup)

	mustExec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed (%.60s): %v", sql, err)
		}
	}

	mustExec(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, subject, type)
	          VALUES ($1, now(), now(), 'cr-onhold-test', 'cr-onhold-test', 'CRONHOLD1', 'on-hold gate test', 'CHANGE_REQUEST')`,
		changeRequestOnHoldTestID)

	if onHold {
		mustExec(`INSERT INTO change_request (id, state, is_on_hold, on_hold_reason)
		          VALUES ($1, $2::change_request_state_enum, TRUE, $3)`,
			changeRequestOnHoldTestID, state, reason)
	} else {
		mustExec(`INSERT INTO change_request (id, state, is_on_hold) VALUES ($1, $2::change_request_state_enum, FALSE)`,
			changeRequestOnHoldTestID, state)
	}
}

// TestChangeRequestIntegration_PatchOnHoldPersistsAndReadsBack confirms
// {onHold: true, onHoldReason: ...} actually persists change_request.is_on_hold/
// on_hold_reason, both in PatchChangeRequest's own
// response and independently on a fresh GetChangeRequestByID read.
func TestChangeRequestIntegration_PatchOnHoldPersistsAndReadsBack(t *testing.T) {
	dsn := os.Getenv("CHANGE_REQUEST_TEST_DSN")
	if dsn == "" {
		t.Skip("CHANGE_REQUEST_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	scoped := repository.NewScoped(pool)
	sys := repository.WithSystemIdentity(context.Background())
	repo := repository.NewChangeRequestRepository(scoped)
	seedChangeRequestForOnHoldTest(t, scoped, "NEW", false, nil)

	yes := true
	reason := "maintenance window"
	cr, err := repo.PatchChangeRequest(sys, changeRequestOnHoldTestID,
		domain.PatchChangeRequestRequest{OnHold: &yes, OnHoldReason: &reason}, "cr-onhold-test")
	if err != nil {
		t.Fatalf("PatchChangeRequest(onHold=true): %v", err)
	}
	if cr.OnHold == nil || !*cr.OnHold {
		t.Fatalf("OnHold after patch = %v, want true", cr.OnHold)
	}
	if cr.OnHoldReason == nil || *cr.OnHoldReason != reason {
		t.Fatalf("OnHoldReason after patch = %v, want %q", cr.OnHoldReason, reason)
	}

	// Read back independently via GetChangeRequestByID -- not just trusting
	// PatchChangeRequest's own response -- to confirm this actually
	// persisted to change_request rather than only round-tripping in memory.
	got, err := repo.GetChangeRequestByID(sys, changeRequestOnHoldTestID)
	if err != nil {
		t.Fatalf("GetChangeRequestByID: %v", err)
	}
	if got.OnHold == nil || !*got.OnHold {
		t.Fatalf("GetChangeRequestByID OnHold = %v, want true", got.OnHold)
	}
	if got.OnHoldReason == nil || *got.OnHoldReason != reason {
		t.Fatalf("GetChangeRequestByID OnHoldReason = %v, want %q", got.OnHoldReason, reason)
	}
}

// TestChangeRequestIntegration_PatchStateRejectedWhileOnHold is the gate's
// main regression guard: a state-advancing PATCH against a change request
// that is CURRENTLY on hold (seeded directly, not via this same PATCH) must
// be refused with a ValidationError, and the state column itself must stay
// untouched.
func TestChangeRequestIntegration_PatchStateRejectedWhileOnHold(t *testing.T) {
	dsn := os.Getenv("CHANGE_REQUEST_TEST_DSN")
	if dsn == "" {
		t.Skip("CHANGE_REQUEST_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	scoped := repository.NewScoped(pool)
	sys := repository.WithSystemIdentity(context.Background())
	repo := repository.NewChangeRequestRepository(scoped)
	reason := "customer change freeze"
	seedChangeRequestForOnHoldTest(t, scoped, "NEW", true, &reason)

	// CANCELED rather than ASSESS/AUTHORIZE/REVIEW deliberately -- this test
	// is about the on-hold gate specifically, not approver auto-provisioning,
	// and CANCELED needs no assignedTeamId and triggers no provisioning path
	// at all, so a failure here can only be the on-hold gate.
	target := domain.ChangeRequestStateCanceled
	_, err = repo.PatchChangeRequest(sys, changeRequestOnHoldTestID,
		domain.PatchChangeRequestRequest{State: &target}, "cr-onhold-test")
	if err == nil {
		t.Fatal("PatchChangeRequest(state) against an on-hold record: want error, got nil")
	}
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("PatchChangeRequest(state) against an on-hold record: got %T (%v), want *apierror.ValidationError", err, err)
	}

	var gotState string
	if scanErr := scoped.QueryRow(sys,
		`SELECT state::TEXT FROM change_request WHERE id = $1`, changeRequestOnHoldTestID).Scan(&gotState); scanErr != nil {
		t.Fatalf("read back state: %v", scanErr)
	}
	if gotState != "NEW" {
		t.Fatalf("state after rejected patch = %q, want unchanged \"NEW\"", gotState)
	}
}

// TestChangeRequestIntegration_PatchClearsOnHoldAndAdvancesStateTogether
// confirms the one deliberate exception to the gate above: {state: X,
// onHold: false} in the SAME request is allowed through even though the
// record is currently on hold -- "take it off hold and advance in one
// call". Also confirms clearing OnHold in this combined request clears
// on_hold_reason exactly the same way a standalone
// {onHold: false} would.
func TestChangeRequestIntegration_PatchClearsOnHoldAndAdvancesStateTogether(t *testing.T) {
	dsn := os.Getenv("CHANGE_REQUEST_TEST_DSN")
	if dsn == "" {
		t.Skip("CHANGE_REQUEST_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	scoped := repository.NewScoped(pool)
	sys := repository.WithSystemIdentity(context.Background())
	repo := repository.NewChangeRequestRepository(scoped)
	reason := "customer change freeze"
	seedChangeRequestForOnHoldTest(t, scoped, "NEW", true, &reason)

	no := false
	target := domain.ChangeRequestStateCanceled
	cr, err := repo.PatchChangeRequest(sys, changeRequestOnHoldTestID,
		domain.PatchChangeRequestRequest{State: &target, OnHold: &no}, "cr-onhold-test")
	if err != nil {
		t.Fatalf("PatchChangeRequest(state=canceled, onHold=false) against an on-hold record: %v", err)
	}
	if cr.OnHold == nil || *cr.OnHold {
		t.Fatalf("OnHold after simultaneous clear+advance = %v, want false", cr.OnHold)
	}
	if cr.OnHoldReason != nil {
		t.Fatalf("OnHoldReason after simultaneous clear+advance = %v, want nil", cr.OnHoldReason)
	}
	if cr.State == nil || *cr.State != string(domain.ChangeRequestStateCanceled) {
		t.Fatalf("state after simultaneous clear+advance = %v, want %q", cr.State, domain.ChangeRequestStateCanceled)
	}
}

// TestChangeRequestIntegration_PatchOffHoldAlwaysSucceeds confirms taking a
// record off hold (OnHold: false, with no other field at all -- no State)
// is never blocked, regardless of the record's current lifecycle state,
// terminal states included.
func TestChangeRequestIntegration_PatchOffHoldAlwaysSucceeds(t *testing.T) {
	dsn := os.Getenv("CHANGE_REQUEST_TEST_DSN")
	if dsn == "" {
		t.Skip("CHANGE_REQUEST_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	scoped := repository.NewScoped(pool)
	sys := repository.WithSystemIdentity(context.Background())
	repo := repository.NewChangeRequestRepository(scoped)

	for _, seedState := range []string{"NEW", "ASSESS", "CLOSED", "CANCELED"} {
		t.Run(seedState, func(t *testing.T) {
			reason := "temporary freeze"
			seedChangeRequestForOnHoldTest(t, scoped, seedState, true, &reason)

			no := false
			cr, err := repo.PatchChangeRequest(sys, changeRequestOnHoldTestID,
				domain.PatchChangeRequestRequest{OnHold: &no}, "cr-onhold-test")
			if err != nil {
				t.Fatalf("PatchChangeRequest(onHold=false) on a %s-state record: %v", seedState, err)
			}
			if cr.OnHold == nil || *cr.OnHold {
				t.Fatalf("OnHold after patch = %v, want false", cr.OnHold)
			}
			if cr.OnHoldReason != nil {
				t.Fatalf("OnHoldReason after patch = %v, want nil", cr.OnHoldReason)
			}
			if cr.State == nil || strings.ToUpper(*cr.State) != seedState {
				t.Fatalf("state after off-hold patch = %v, want unchanged %q", cr.State, seedState)
			}
		})
	}
}

// TestChangeRequestIntegration_PatchNonStateFieldSucceedsWhileOnHold confirms
// the gate is scoped exactly to State: a PATCH that never touches State at
// all (editing Description here) must succeed normally even while the
// record is on hold, and must leave the on-hold columns completely
// untouched -- being on hold only ever blocks a state-changing PATCH, never
// any other field.
func TestChangeRequestIntegration_PatchNonStateFieldSucceedsWhileOnHold(t *testing.T) {
	dsn := os.Getenv("CHANGE_REQUEST_TEST_DSN")
	if dsn == "" {
		t.Skip("CHANGE_REQUEST_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	scoped := repository.NewScoped(pool)
	sys := repository.WithSystemIdentity(context.Background())
	repo := repository.NewChangeRequestRepository(scoped)
	reason := "vendor maintenance window"
	seedChangeRequestForOnHoldTest(t, scoped, "NEW", true, &reason)

	newDescription := "updated while on hold"
	cr, err := repo.PatchChangeRequest(sys, changeRequestOnHoldTestID,
		domain.PatchChangeRequestRequest{Description: &newDescription}, "cr-onhold-test")
	if err != nil {
		t.Fatalf("PatchChangeRequest(description) on an on-hold record: %v", err)
	}
	if cr.Description == nil || *cr.Description != newDescription {
		t.Fatalf("Description after patch = %v, want %q", cr.Description, newDescription)
	}
	if cr.OnHold == nil || !*cr.OnHold {
		t.Fatalf("OnHold after unrelated patch = %v, want unchanged true", cr.OnHold)
	}
	if cr.OnHoldReason == nil || *cr.OnHoldReason != reason {
		t.Fatalf("OnHoldReason after unrelated patch = %v, want unchanged %q", cr.OnHoldReason, reason)
	}
}

// seedChangeRequestCustomerFlagAccount inserts one account/account_contact
// pair shared by every customer-flag test below (account_contact_id is
// just an FK -- project_contact carries its own, independent email column,
// so one account_contact row can back every project_contact fixture these
// tests seed).
func seedChangeRequestCustomerFlagAccount(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()

	cleanup := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM account_contact WHERE id = $1`, crCustomerFlagAccountContactID)
		_, _ = pool.Exec(ctx, `DELETE FROM account WHERE id = $1`, crCustomerFlagAccountID)
	}
	cleanup()
	t.Cleanup(cleanup)

	if _, err := pool.Exec(ctx,
		`INSERT INTO account (id, created_on, updated_on, created_by, updated_by, name, number, sf_id)
		 VALUES ($1, now(), now(), 'cr-customer-flag-test', 'cr-customer-flag-test', 'CR Customer Flag Test Account', 'CR-CF-ACC-1', 'CR-CF-SF-ACC-1')`,
		crCustomerFlagAccountID); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO account_contact (id, created_on, updated_on, created_by, updated_by, user_name, account_id)
		 VALUES ($1, now(), now(), 'cr-customer-flag-test', 'cr-customer-flag-test', 'CR Customer Flag Test Contact', $2)`,
		crCustomerFlagAccountContactID, crCustomerFlagAccountID); err != nil {
		t.Fatalf("seed account_contact: %v", err)
	}
}

// seedChangeRequestCustomerFlagProject inserts one project row under
// crCustomerFlagAccountID -- id/key must be unique per call site (project.key
// is UNIQUE).
func seedChangeRequestCustomerFlagProject(t *testing.T, pool *pgxpool.Pool, projectID, key string) {
	t.Helper()
	ctx := context.Background()

	cleanup := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM project WHERE id = $1`, projectID)
	}
	cleanup()
	t.Cleanup(cleanup)

	if _, err := pool.Exec(ctx,
		`INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, sf_id, name, account_id)
		 VALUES ($1, now(), now(), 'cr-customer-flag-test', 'cr-customer-flag-test', $2, $2, $2, $3)`,
		projectID, key, crCustomerFlagAccountID); err != nil {
		t.Fatalf("seed project %s: %v", key, err)
	}
}

// ensureProjectRole guarantees a project_role row for role exists --
// idempotent, same "safe to run against a database that already has it"
// discipline as migration 0128_project_admin_role_group.sql's own ADMIN-role
// seed. project_role rows are otherwise only ever created by the ServiceNow
// sync job (see case_repo_announcement_visibility_integration_test.go's own
// doc comment on project_role/project_group/project_group_role), which a
// bare local docker-compose stack never runs -- PORTAL_USER/SECURITY_CONTACT
// cannot be assumed present there. Left in place afterward, not cleaned up:
// it is reference/catalog data, same as every project_role row any other
// caller might also depend on existing.
func ensureProjectRole(t *testing.T, pool *pgxpool.Pool, role string) string {
	t.Helper()
	ctx := context.Background()

	var id string
	err := pool.QueryRow(ctx, `SELECT id::text FROM project_role WHERE role = $1::project_role_enum`, role).Scan(&id)
	if err == nil {
		return id
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("look up project_role %s: %v", role, err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO project_role (id, created_on, updated_on, created_by, updated_by, role)
		 VALUES (gen_random_uuid(), now(), now(), 'cr-customer-flag-test', 'cr-customer-flag-test', $1::project_role_enum)
		 RETURNING id::text`, role).Scan(&id); err != nil {
		t.Fatalf("seed project_role %s: %v", role, err)
	}
	return id
}

// seedProjectContactWithRole inserts a project_contact row (email, state) on
// projectID, holding role via its own dedicated project_group/
// project_group_role pair -- the identical project_contact ->
// project_contact_group -> project_group_role -> project_role join chain
// CaseRepository.ProjectContactEmailsByRole/ProjectContactRepository's own
// projectContactColumns already use, which
// callerMayGrantChangeRequestCustomerFlag (change_request_repo.go) reuses
// verbatim for this feature's own authorization check. groupName must be
// unique across this whole test file (project_group."group" is UNIQUE).
func seedProjectContactWithRole(t *testing.T, pool *pgxpool.Pool, projectID, email, state, role, groupName string) {
	t.Helper()
	ctx := context.Background()
	roleID := ensureProjectRole(t, pool, role)

	var groupID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO project_group (id, created_on, updated_on, created_by, updated_by, "group")
		 VALUES (gen_random_uuid(), now(), now(), 'cr-customer-flag-test', 'cr-customer-flag-test', $1)
		 RETURNING id::text`, groupName).Scan(&groupID); err != nil {
		t.Fatalf("seed project_group %s: %v", groupName, err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM project_group WHERE id = $1`, groupID) })

	if _, err := pool.Exec(ctx,
		`INSERT INTO project_group_role (id, created_on, updated_on, created_by, updated_by, project_group_id, project_role_id)
		 VALUES (gen_random_uuid(), now(), now(), 'cr-customer-flag-test', 'cr-customer-flag-test', $1, $2)`,
		groupID, roleID); err != nil {
		t.Fatalf("seed project_group_role: %v", err)
	}

	var contactID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO project_contact (id, created_on, updated_on, created_by, updated_by, email, account_contact_id, project_id, state)
		 VALUES (gen_random_uuid(), now(), now(), 'cr-customer-flag-test', 'cr-customer-flag-test', $1, $2, $3, $4::project_contact_state_enum)
		 RETURNING id::text`, email, crCustomerFlagAccountContactID, projectID, state).Scan(&contactID); err != nil {
		t.Fatalf("seed project_contact %s: %v", email, err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM project_contact WHERE id = $1`, contactID) })

	if _, err := pool.Exec(ctx,
		`INSERT INTO project_contact_group (id, created_on, updated_on, created_by, updated_by, project_contact_id, project_group_id)
		 VALUES (gen_random_uuid(), now(), now(), 'cr-customer-flag-test', 'cr-customer-flag-test', $1, $2)`,
		contactID, groupID); err != nil {
		t.Fatalf("seed project_contact_group: %v", err)
	}
}

// seedChangeRequestForCustomerFlagTest inserts a minimal work_item/
// change_request pair linked to projectID, with is_customer_approval_required/
// is_customer_review_required seeded directly via SQL -- bypassing PatchChangeRequest
// entirely -- to whatever precondition a given test needs to exist before the
// PATCH under test ever runs. Same split-precondition-from-write-path
// discipline as seedChangeRequestForOnHoldTest's own doc comment.
func seedChangeRequestForCustomerFlagTest(t *testing.T, pool *repository.Scoped, id, number, projectID string, approved, reviewed bool) {
	t.Helper()
	// work_item/change_request are RLS-protected; seed/cleanup as internal.
	ctx := repository.WithSystemIdentity(context.Background())

	cleanup := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM work_item WHERE id = $1`, id)
	}
	cleanup()
	t.Cleanup(cleanup)

	mustExec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed (%.70s): %v", sql, err)
		}
	}

	mustExec(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, subject, type, project_id)
	          VALUES ($1, now(), now(), 'cr-customer-flag-test', 'cr-customer-flag-test', $2, 'customer flag auth test', 'CHANGE_REQUEST', $3::uuid)`,
		id, number, projectID)
	mustExec(`INSERT INTO change_request (id, state, is_customer_approval_required, is_customer_review_required)
	          VALUES ($1, 'NEW'::change_request_state_enum, $2, $3)`,
		id, approved, reviewed)
}

// externalCallerCtx builds a ctx carrying a non-internal (Unrestricted:
// false) caller identity for email -- repository.Scoped's own InTx/Query/
// QueryRow/Exec only need SOME identity present (CallerIdentityFromContext's
// ok=true), and change_request/work_item's own RLS policies resolve
// project membership for this email server-side from project_contact
// (rls.go's setViewerProjectIDsSQL), not from anything set here.
func externalCallerCtx(email string) context.Context {
	return repository.WithCallerIdentity(context.Background(), repository.SearchScope{ViewerEmail: email})
}

// TestChangeRequestIntegration_PatchCustomerFlagInternalCallerCanSetBoth
// confirms an internal caller may flip BOTH is_customer_approval_required and
// is_customer_review_required from false to true, in one PATCH, with no project
// membership or project_contact row involved at all.
func TestChangeRequestIntegration_PatchCustomerFlagInternalCallerCanSetBoth(t *testing.T) {
	dsn := os.Getenv("CHANGE_REQUEST_TEST_DSN")
	if dsn == "" {
		t.Skip("CHANGE_REQUEST_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	scoped := repository.NewScoped(pool)
	sys := repository.WithSystemIdentity(context.Background())
	repo := repository.NewChangeRequestRepository(scoped)

	seedChangeRequestCustomerFlagAccount(t, pool)
	seedChangeRequestCustomerFlagProject(t, pool, crCustomerFlagProjectID, "CRCFPROJ01")
	seedChangeRequestForCustomerFlagTest(t, scoped, crCustomerFlagInternalTestID, "CRCFINT001", crCustomerFlagProjectID, false, false)

	yes := true
	cr, err := repo.PatchChangeRequest(sys, crCustomerFlagInternalTestID,
		domain.PatchChangeRequestRequest{IsCustomerApproved: &yes, IsCustomerReviewed: &yes}, "cr-customer-flag-test")
	if err != nil {
		t.Fatalf("PatchChangeRequest(isCustomerApproved=true, isCustomerReviewed=true) as internal: %v", err)
	}
	if !cr.HasCustomerApproved {
		t.Error("HasCustomerApproved after patch = false, want true")
	}
	if !cr.HasCustomerReviewed {
		t.Error("HasCustomerReviewed after patch = false, want true")
	}

	got, err := repo.GetChangeRequestByID(sys, crCustomerFlagInternalTestID)
	if err != nil {
		t.Fatalf("GetChangeRequestByID: %v", err)
	}
	if !got.HasCustomerApproved || !got.HasCustomerReviewed {
		t.Errorf("GetChangeRequestByID HasCustomerApproved/HasCustomerReviewed = %v/%v, want true/true", got.HasCustomerApproved, got.HasCustomerReviewed)
	}
}

// TestChangeRequestIntegration_PatchCustomerFlagQualifyingPortalUserContactCanApprove
// confirms a REGISTERED project_contact holding PORTAL_USER on the change
// request's OWN project may flip is_customer_approval_required false -> true.
func TestChangeRequestIntegration_PatchCustomerFlagQualifyingPortalUserContactCanApprove(t *testing.T) {
	dsn := os.Getenv("CHANGE_REQUEST_TEST_DSN")
	if dsn == "" {
		t.Skip("CHANGE_REQUEST_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	scoped := repository.NewScoped(pool)
	repo := repository.NewChangeRequestRepository(scoped)

	seedChangeRequestCustomerFlagAccount(t, pool)
	seedChangeRequestCustomerFlagProject(t, pool, crCustomerFlagProjectID, "CRCFPROJ02")
	seedProjectContactWithRole(t, pool, crCustomerFlagProjectID,
		crCustomerFlagPortalUserEmail, "REGISTERED", "PORTAL_USER", "CR Customer Flag Portal User Group")
	seedChangeRequestForCustomerFlagTest(t, scoped, crCustomerFlagQualifyingTestID, "CRCFQUAL01", crCustomerFlagProjectID, false, false)

	yes := true
	cr, err := repo.PatchChangeRequest(externalCallerCtx(crCustomerFlagPortalUserEmail), crCustomerFlagQualifyingTestID,
		domain.PatchChangeRequestRequest{IsCustomerApproved: &yes}, crCustomerFlagPortalUserEmail)
	if err != nil {
		t.Fatalf("PatchChangeRequest(isCustomerApproved=true) as a REGISTERED PORTAL_USER contact on the same project: %v", err)
	}
	if !cr.HasCustomerApproved {
		t.Error("HasCustomerApproved after patch = false, want true")
	}
}

// TestChangeRequestIntegration_PatchCustomerFlagContactOnDifferentProjectCannot
// confirms a REGISTERED PORTAL_USER contact on a DIFFERENT project than the
// change request's own cannot flip its flag -- expected as *apierror.
// ForbiddenError, this feature's own authorization check
// (callerMayGrantChangeRequestCustomerFlag only matches a project_contact
// row against THIS change request's own project_id, so a contact registered
// on some other project never matches). Confirmed live against this
// suite's own CHANGE_REQUEST_TEST_DSN that this is genuinely what fires
// here, not work_item's own RLS (migration 0147) rejecting the write one
// layer earlier as a NotFoundError instead: this stack's DSN connects as
// the `postgres` role, a real Postgres superuser, which unconditionally
// bypasses every RLS policy regardless of FORCE ROW LEVEL SECURITY (a
// Postgres behavior, not a bug here or in migration 0147) -- so the
// work_item UPDATE this PATCH also performs always passes in this
// environment regardless of project membership, and this feature's own
// check is genuinely the only thing standing between this caller and the
// write. A deployment connecting as a non-superuser role would intercept
// this same scenario one layer earlier, as a NotFoundError from work_item's
// own RLS instead -- either way, the caller cannot flip the flag.
func TestChangeRequestIntegration_PatchCustomerFlagContactOnDifferentProjectCannot(t *testing.T) {
	dsn := os.Getenv("CHANGE_REQUEST_TEST_DSN")
	if dsn == "" {
		t.Skip("CHANGE_REQUEST_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	scoped := repository.NewScoped(pool)
	sys := repository.WithSystemIdentity(context.Background())
	repo := repository.NewChangeRequestRepository(scoped)

	seedChangeRequestCustomerFlagAccount(t, pool)
	seedChangeRequestCustomerFlagProject(t, pool, crCustomerFlagProjectID, "CRCFPROJ03")
	seedChangeRequestCustomerFlagProject(t, pool, crCustomerFlagOtherProjectID, "CRCFPROJ04")
	seedProjectContactWithRole(t, pool, crCustomerFlagOtherProjectID,
		crCustomerFlagOtherProjectEmail, "REGISTERED", "PORTAL_USER", "CR Customer Flag Other Project Group")
	seedChangeRequestForCustomerFlagTest(t, scoped, crCustomerFlagDifferentProjectTestID, "CRCFDIFF01", crCustomerFlagProjectID, false, false)

	yes := true
	_, err = repo.PatchChangeRequest(externalCallerCtx(crCustomerFlagOtherProjectEmail), crCustomerFlagDifferentProjectTestID,
		domain.PatchChangeRequestRequest{IsCustomerApproved: &yes}, crCustomerFlagOtherProjectEmail)
	if err == nil {
		t.Fatal("PatchChangeRequest(isCustomerApproved=true) from a different project's own contact: want error, got nil")
	}
	var fe *apierror.ForbiddenError
	if !errors.As(err, &fe) {
		t.Fatalf("got %T (%v), want *apierror.ForbiddenError", err, err)
	}

	var gotApproved *bool
	if scanErr := scoped.QueryRow(sys, `SELECT is_customer_approval_required FROM change_request WHERE id = $1`, crCustomerFlagDifferentProjectTestID).Scan(&gotApproved); scanErr != nil {
		t.Fatalf("read back is_customer_approval_required: %v", scanErr)
	}
	if gotApproved != nil && *gotApproved {
		t.Fatal("is_customer_approval_required after rejected patch = true, want unchanged false")
	}
}

// TestChangeRequestIntegration_PatchCustomerFlagWrongRoleContactForbiddenStateUnaffected
// covers two of this feature's own required guarantees together: (1) a
// REGISTERED contact who holds no PORTAL_USER role anywhere on the change
// request's project -- a literal "project with no qualifying contact" -- is
// rejected with a ForbiddenError (this feature's own authorization check,
// not RLS: the contact genuinely IS a project member, so the write reaches
// change_request_repo.go's own gate), and (2) that rejection has NO bearing
// whatsoever on the change request's own state transitions: a separate
// {state: "canceled"} PATCH immediately afterward (CANCELED needs no
// assignedTeamId and triggers no approval-provisioning path, isolating this
// assertion to the on-hold/customer-flag gates alone, same reasoning
// TestChangeRequestIntegration_PatchStateRejectedWhileOnHold's own doc
// comment already uses) still succeeds normally.
func TestChangeRequestIntegration_PatchCustomerFlagWrongRoleContactForbiddenStateUnaffected(t *testing.T) {
	dsn := os.Getenv("CHANGE_REQUEST_TEST_DSN")
	if dsn == "" {
		t.Skip("CHANGE_REQUEST_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	scoped := repository.NewScoped(pool)
	sys := repository.WithSystemIdentity(context.Background())
	repo := repository.NewChangeRequestRepository(scoped)

	seedChangeRequestCustomerFlagAccount(t, pool)
	seedChangeRequestCustomerFlagProject(t, pool, crCustomerFlagNoRoleProjectID, "CRCFPROJ05")
	seedProjectContactWithRole(t, pool, crCustomerFlagNoRoleProjectID,
		crCustomerFlagWrongRoleEmail, "REGISTERED", "SECURITY_CONTACT", "CR Customer Flag Wrong Role Group")
	seedChangeRequestForCustomerFlagTest(t, scoped, crCustomerFlagWrongRoleTestID, "CRCFWRNG01", crCustomerFlagNoRoleProjectID, false, false)

	yes := true
	_, err = repo.PatchChangeRequest(externalCallerCtx(crCustomerFlagWrongRoleEmail), crCustomerFlagWrongRoleTestID,
		domain.PatchChangeRequestRequest{IsCustomerApproved: &yes}, crCustomerFlagWrongRoleEmail)
	if err == nil {
		t.Fatal("PatchChangeRequest(isCustomerApproved=true) from a REGISTERED non-PORTAL_USER contact: want error, got nil")
	}
	var fe *apierror.ForbiddenError
	if !errors.As(err, &fe) {
		t.Fatalf("got %T (%v), want *apierror.ForbiddenError", err, err)
	}

	var gotApproved *bool
	if scanErr := scoped.QueryRow(sys, `SELECT is_customer_approval_required FROM change_request WHERE id = $1`, crCustomerFlagWrongRoleTestID).Scan(&gotApproved); scanErr != nil {
		t.Fatalf("read back is_customer_approval_required: %v", scanErr)
	}
	if gotApproved != nil && *gotApproved {
		t.Fatal("is_customer_approval_required after rejected patch = true, want unchanged false")
	}

	// The rejection above must have zero bearing on this same change
	// request's own state transitions.
	target := domain.ChangeRequestStateCanceled
	cr, err := repo.PatchChangeRequest(sys, crCustomerFlagWrongRoleTestID,
		domain.PatchChangeRequestRequest{State: &target}, "cr-customer-flag-test")
	if err != nil {
		t.Fatalf("PatchChangeRequest(state=canceled) after a customer-flag rejection on the same record: %v", err)
	}
	if cr.State == nil || *cr.State != string(domain.ChangeRequestStateCanceled) {
		t.Fatalf("state after patch = %v, want %q", cr.State, domain.ChangeRequestStateCanceled)
	}
}

// TestChangeRequestIntegration_PatchCustomerFlagInvitedContactCannotReach
// confirms registration state matters as much as role: a contact holding
// PORTAL_USER but still only INVITED (never accepted) on the change
// request's own project cannot flip its flag -- callerMayGrantChangeRequestCustomerFlag's
// own query requires state = REGISTERED exactly, so an INVITED contact
// never matches, surfacing the same *apierror.ForbiddenError* as the
// different-project case above and for the identical reason explained
// there (this environment's DSN connects as a Postgres superuser, which
// unconditionally bypasses work_item's own RLS, so this feature's own
// check is genuinely what is exercised here).
func TestChangeRequestIntegration_PatchCustomerFlagInvitedContactCannotReach(t *testing.T) {
	dsn := os.Getenv("CHANGE_REQUEST_TEST_DSN")
	if dsn == "" {
		t.Skip("CHANGE_REQUEST_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	scoped := repository.NewScoped(pool)
	repo := repository.NewChangeRequestRepository(scoped)

	seedChangeRequestCustomerFlagAccount(t, pool)
	seedChangeRequestCustomerFlagProject(t, pool, crCustomerFlagProjectID, "CRCFPROJ06")
	seedProjectContactWithRole(t, pool, crCustomerFlagProjectID,
		crCustomerFlagInvitedEmail, "INVITED", "PORTAL_USER", "CR Customer Flag Invited Group")
	seedChangeRequestForCustomerFlagTest(t, scoped, crCustomerFlagInvitedTestID, "CRCFINVT01", crCustomerFlagProjectID, false, false)

	yes := true
	_, err = repo.PatchChangeRequest(externalCallerCtx(crCustomerFlagInvitedEmail), crCustomerFlagInvitedTestID,
		domain.PatchChangeRequestRequest{IsCustomerApproved: &yes}, crCustomerFlagInvitedEmail)
	if err == nil {
		t.Fatal("PatchChangeRequest(isCustomerApproved=true) from a still-INVITED PORTAL_USER contact: want error, got nil")
	}
	var fe *apierror.ForbiddenError
	if !errors.As(err, &fe) {
		t.Fatalf("got %T (%v), want *apierror.ForbiddenError", err, err)
	}
}

// TestChangeRequestIntegration_PatchCustomerFlagLockedTrueCannotRevert is the
// main regression guard for this feature's one-way lock: once either flag
// is true, a PATCH attempting to set it back to false is always rejected
// with a ValidationError -- regardless of whether the caller is internal or
// the same qualifying customer contact that could have set it true in the
// first place.
func TestChangeRequestIntegration_PatchCustomerFlagLockedTrueCannotRevert(t *testing.T) {
	dsn := os.Getenv("CHANGE_REQUEST_TEST_DSN")
	if dsn == "" {
		t.Skip("CHANGE_REQUEST_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	scoped := repository.NewScoped(pool)
	sys := repository.WithSystemIdentity(context.Background())
	repo := repository.NewChangeRequestRepository(scoped)

	seedChangeRequestCustomerFlagAccount(t, pool)
	seedChangeRequestCustomerFlagProject(t, pool, crCustomerFlagProjectID, "CRCFPROJ07")
	seedProjectContactWithRole(t, pool, crCustomerFlagProjectID,
		crCustomerFlagPortalUserEmail, "REGISTERED", "PORTAL_USER", "CR Customer Flag Portal User Group")
	seedChangeRequestForCustomerFlagTest(t, scoped, crCustomerFlagLockedTestID, "CRCFLOCK01", crCustomerFlagProjectID, true, true)

	no := false

	// Internal caller: still rejected -- there is no override path.
	_, err = repo.PatchChangeRequest(sys, crCustomerFlagLockedTestID,
		domain.PatchChangeRequestRequest{IsCustomerApproved: &no}, "cr-customer-flag-test")
	if err == nil {
		t.Fatal("PatchChangeRequest(isCustomerApproved=false) as internal against an already-true record: want error, got nil")
	}
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("got %T (%v), want *apierror.ValidationError", err, err)
	}

	// The same qualifying contact that could have set it true: also rejected.
	_, err = repo.PatchChangeRequest(externalCallerCtx(crCustomerFlagPortalUserEmail), crCustomerFlagLockedTestID,
		domain.PatchChangeRequestRequest{IsCustomerReviewed: &no}, crCustomerFlagPortalUserEmail)
	if err == nil {
		t.Fatal("PatchChangeRequest(isCustomerReviewed=false) as the qualifying contact against an already-true record: want error, got nil")
	}
	if !errors.As(err, &ve) {
		t.Fatalf("got %T (%v), want *apierror.ValidationError", err, err)
	}

	var gotApproved, gotReviewed bool
	if scanErr := scoped.QueryRow(sys, `SELECT is_customer_approval_required, is_customer_review_required FROM change_request WHERE id = $1`,
		crCustomerFlagLockedTestID).Scan(&gotApproved, &gotReviewed); scanErr != nil {
		t.Fatalf("read back: %v", scanErr)
	}
	if !gotApproved || !gotReviewed {
		t.Fatalf("is_customer_approval_required/is_customer_review_required after both rejected patches = %v/%v, want unchanged true/true", gotApproved, gotReviewed)
	}
}

// TestChangeRequestIntegration_PatchCustomerFlagFalseToFalseNoOpSucceeds
// confirms setting an already-false flag to false again is a no-op that
// always succeeds trivially -- deliberately exercised by a caller who does
// NOT qualify to grant it (a REGISTERED contact holding no PORTAL_USER
// role), proving no-op writes need no authorization check at all.
func TestChangeRequestIntegration_PatchCustomerFlagFalseToFalseNoOpSucceeds(t *testing.T) {
	dsn := os.Getenv("CHANGE_REQUEST_TEST_DSN")
	if dsn == "" {
		t.Skip("CHANGE_REQUEST_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	scoped := repository.NewScoped(pool)
	repo := repository.NewChangeRequestRepository(scoped)

	seedChangeRequestCustomerFlagAccount(t, pool)
	seedChangeRequestCustomerFlagProject(t, pool, crCustomerFlagNoRoleProjectID, "CRCFPROJ08")
	seedProjectContactWithRole(t, pool, crCustomerFlagNoRoleProjectID,
		crCustomerFlagWrongRoleEmail, "REGISTERED", "SECURITY_CONTACT", "CR Customer Flag No-op Wrong Role Group")
	seedChangeRequestForCustomerFlagTest(t, scoped, crCustomerFlagNoOpTestID, "CRCFNOOP01", crCustomerFlagNoRoleProjectID, false, false)

	no := false
	cr, err := repo.PatchChangeRequest(externalCallerCtx(crCustomerFlagWrongRoleEmail), crCustomerFlagNoOpTestID,
		domain.PatchChangeRequestRequest{IsCustomerApproved: &no, IsCustomerReviewed: &no}, crCustomerFlagWrongRoleEmail)
	if err != nil {
		t.Fatalf("PatchChangeRequest(isCustomerApproved=false, isCustomerReviewed=false) no-op from a non-qualifying contact: %v", err)
	}
	if cr.HasCustomerApproved || cr.HasCustomerReviewed {
		t.Fatalf("HasCustomerApproved/HasCustomerReviewed after no-op patch = %v/%v, want false/false", cr.HasCustomerApproved, cr.HasCustomerReviewed)
	}
}

// TestChangeRequestIntegration_PatchCustomerFlagsLockIndependentPerField
// confirms one field's own lock state has no bearing on the other's: a
// single PATCH with is_customer_approval_required ALREADY true (so true -> true, a
// no-op, always allowed) and is_customer_review_required false -> true (the one
// real gated transition, allowed here since the caller is internal)
// succeeds as a whole, changing only is_customer_review_required.
func TestChangeRequestIntegration_PatchCustomerFlagsLockIndependentPerField(t *testing.T) {
	dsn := os.Getenv("CHANGE_REQUEST_TEST_DSN")
	if dsn == "" {
		t.Skip("CHANGE_REQUEST_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	scoped := repository.NewScoped(pool)
	sys := repository.WithSystemIdentity(context.Background())
	repo := repository.NewChangeRequestRepository(scoped)

	seedChangeRequestCustomerFlagAccount(t, pool)
	seedChangeRequestCustomerFlagProject(t, pool, crCustomerFlagProjectID, "CRCFPROJ09")
	seedChangeRequestForCustomerFlagTest(t, scoped, crCustomerFlagIndependentTestID, "CRCFINDP01", crCustomerFlagProjectID, true, false)

	yes := true
	cr, err := repo.PatchChangeRequest(sys, crCustomerFlagIndependentTestID,
		domain.PatchChangeRequestRequest{IsCustomerApproved: &yes, IsCustomerReviewed: &yes}, "cr-customer-flag-test")
	if err != nil {
		t.Fatalf("PatchChangeRequest(isCustomerApproved=true [no-op], isCustomerReviewed=true [new]): %v", err)
	}
	if !cr.HasCustomerApproved {
		t.Error("HasCustomerApproved after patch = false, want unchanged true")
	}
	if !cr.HasCustomerReviewed {
		t.Error("HasCustomerReviewed after patch = false, want true")
	}
}

// ---------------------------------------------------------------------------
// Type-dependent approval flow (change_request_approval_flow.go).
//
// Lifecycle tests for Normal (peer then CAB), Emergency (ECAB only) and
// Standard (no approval), the creator/INTERNAL-only approver rules, the CAB/ECAB groups
// the migration creates, the automatic move to Scheduled, and the mandatory
// type on create -- all against the real Postgres this file's neighbours use:
//
//	CHANGE_REQUEST_TEST_DSN=postgres://... go test ./internal/repository/ -run ChangeRequestFlowIntegration
// ---------------------------------------------------------------------------

const (
	// crCABGroupID / crECABGroupID are the fixed ids migration
	// 0188_change_request_approval_groups.sql gives the two groups.
	crCABGroupID  = "00000000-0000-4000-8000-00000000ca01"
	crECABGroupID = "00000000-0000-4000-8000-00000000eca1"

	// crCABMemberUserID{1,2} / crECABMemberUserID are seeded as members of the
	// CAB / ECAB groups by the tests that need an eligible second approver.
	crCABMemberUserID1 = "3aaaaaaa-0000-0000-0000-0000000000c1"
	crCABMemberUserID2 = "3aaaaaaa-0000-0000-0000-0000000000c2"
	crECABMemberUserID = "3aaaaaaa-0000-0000-0000-0000000000e1"

	crFlowSubject = "cr-approval-flow integration test"

	crFlowCreatorID  = "3aaaaaaa-0000-0000-0000-000000000001"
	crFlowPeerAID    = "3aaaaaaa-0000-0000-0000-000000000002"
	crFlowPeerBID    = "3aaaaaaa-0000-0000-0000-000000000003"
	crFlowSREID      = "3aaaaaaa-0000-0000-0000-000000000004"
	crFlowOutsiderID = "3aaaaaaa-0000-0000-0000-000000000005"

	// Users the INTERNAL-only pool rule must keep out of every internal stage:
	// two customers (EXTERNAL), an internal user who has been deactivated, and
	// a user with no role at all (user_type NOT_AVAILABLE). crFlowSREPeerID is a
	// second, ordinary internal member of the SRE team.
	crFlowExternalID  = "3aaaaaaa-0000-0000-0000-000000000006"
	crFlowExternalID2 = "3aaaaaaa-0000-0000-0000-000000000007"
	crFlowSREPeerID   = "3aaaaaaa-0000-0000-0000-000000000008"
	crFlowInactiveID  = "3aaaaaaa-0000-0000-0000-000000000009"
	crFlowNoTypeID    = "3aaaaaaa-0000-0000-0000-00000000000a"

	// External members of the CAB / ECAB groups and the Devops Approval
	// fallback group, and its internal members.
	crCABExternalID    = "3aaaaaaa-0000-0000-0000-0000000000c3"
	crECABExternalID   = "3aaaaaaa-0000-0000-0000-0000000000e2"
	crDevopsMemberID1  = "3aaaaaaa-0000-0000-0000-0000000000d1"
	crDevopsMemberID2  = "3aaaaaaa-0000-0000-0000-0000000000d2"
	crDevopsExternalID = "3aaaaaaa-0000-0000-0000-0000000000d3"

	// crFlowGroupID is the change's assigned group: creator, both peers and
	// the outsider belong to it.
	crFlowGroupID = "3aaaaaaa-0000-0000-0000-0000000000a1"
	// crFlowSRETeamID is an SRE team (Apollo-like): the "team" row of type
	// sre-abt, mirrored by a "group" row with the SAME id (as every synced team
	// is), which is what a change assigned to that team points at.
	crFlowSRETeamID = "3aaaaaaa-0000-0000-0000-0000000000a3"
	// crFlowSREGroupID is that mirror group.
	crFlowSREGroupID = crFlowSRETeamID
	// crFlowDevopsGroupID is the peer approval fallback group ("Devops Approval").
	crFlowDevopsGroupID = "3aaaaaaa-0000-0000-0000-0000000000a4"
)

func crFlowEmail(userID string) string {
	return fmt.Sprintf("crflow-%s@example.com", userID[len(userID)-12:])
}

// "user".user_type values the tests seed. recompute_user_type() derives them
// from a user's roles (internal/admin -> INTERNAL, customer/external/partner...
// -> EXTERNAL, nothing -> NOT_AVAILABLE); the tests set the column directly
// instead of creating role rows, so they do not depend on (or collide with)
// the seed's fixed-id role rows. Approver pools are INTERNAL-only, so a test
// user is INTERNAL unless the test says otherwise.
const (
	userTypeInternal = "INTERNAL"
	userTypeExternal = "EXTERNAL"
)

// testUserExecer is what both *pgxpool.Pool and *repository.Scoped offer.
type testUserExecer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// setTestUserType sets a seeded test user's user_type.
func setTestUserType(t *testing.T, db testUserExecer, ctx context.Context, userID, userType string) {
	t.Helper()
	if _, err := db.Exec(ctx, `UPDATE "user" SET user_type = $2::user_type_enum WHERE id = $1`, userID, userType); err != nil {
		t.Fatalf("set user_type %s on %s: %v", userType, userID, err)
	}
}

// seedApprovalGroupMembers makes each userID a (freshly seeded) INTERNAL user
// and a member of the "group" groupID (team_member.group_id). The group row
// must exist. Everything is removed again on cleanup.
func seedApprovalGroupMembers(t *testing.T, pool *repository.Scoped, groupID string, userIDs ...string) {
	t.Helper()
	seedGroupMembersOfType(t, pool, groupID, userTypeInternal, userIDs...)
}

// seedExternalGroupMembers is seedApprovalGroupMembers for EXTERNAL (customer)
// users: members of the group who must never be provisioned as approvers of an
// internal stage.
func seedExternalGroupMembers(t *testing.T, pool *repository.Scoped, groupID string, userIDs ...string) {
	t.Helper()
	seedGroupMembersOfType(t, pool, groupID, userTypeExternal, userIDs...)
}

func seedGroupMembersOfType(t *testing.T, pool *repository.Scoped, groupID, userType string, userIDs ...string) {
	t.Helper()
	ctx := repository.WithSystemIdentity(context.Background())
	isolateApprovalGroup(t, pool, groupID)
	for _, uid := range userIDs {
		id := uid
		cleanup := func() { _, _ = pool.Exec(ctx, `DELETE FROM "user" WHERE id = $1`, id) }
		cleanup()
		t.Cleanup(cleanup)
		if _, err := pool.Exec(ctx,
			`INSERT INTO "user" (id, created_on, updated_on, created_by, updated_by, user_name, name, first_name, last_name, email, is_active, is_system_user)
			 VALUES ($1, now(), now(), 'cr-flow-test', 'cr-flow-test', $2, 'CR Flow User', 'CR', 'Flow User', $2, true, false)`,
			id, crFlowEmail(id)); err != nil {
			t.Fatalf("seed user %s: %v", id, err)
		}
		setTestUserType(t, pool, ctx, id, userType)
		if _, err := pool.Exec(ctx,
			`INSERT INTO team_member (id, created_on, updated_on, created_by, updated_by, team_id, user_id, group_id)
			 VALUES (gen_random_uuid(), now(), now(), 'cr-flow-test', 'cr-flow-test', $1::uuid, $2, $3::uuid)`,
			seededGroupID, id, groupID); err != nil {
			t.Fatalf("seed team_member %s: %v", id, err)
		}
	}
}

var isolatedApprovalGroups sync.Map

// isolateApprovalGroup removes whoever is already a member of the CAB / ECAB
// group (local seed data puts the two dev users there; a synced environment
// has real members) for the duration of the calling test, so the test sees
// exactly the members it seeds itself, and puts them back on cleanup. Once per
// test and group; any other group is left alone.
func isolateApprovalGroup(t *testing.T, pool *repository.Scoped, groupID string) {
	t.Helper()
	if groupID != crCABGroupID && groupID != crECABGroupID {
		return
	}
	key := t.Name() + "/" + groupID
	if _, done := isolatedApprovalGroups.LoadOrStore(key, true); done {
		return
	}
	ctx := repository.WithSystemIdentity(context.Background())
	rows, err := pool.Query(ctx, `SELECT id::text, team_id::text, user_id::text FROM team_member WHERE group_id = $1::uuid`, groupID)
	if err != nil {
		t.Fatalf("snapshot approval group members: %v", err)
	}
	type member struct{ id, team, user string }
	var saved []member
	for rows.Next() {
		var m member
		if err := rows.Scan(&m.id, &m.team, &m.user); err != nil {
			rows.Close()
			t.Fatalf("scan approval group member: %v", err)
		}
		saved = append(saved, m)
	}
	rows.Close()
	if _, err := pool.Exec(ctx, `DELETE FROM team_member WHERE group_id = $1::uuid`, groupID); err != nil {
		t.Fatalf("clear approval group members: %v", err)
	}
	t.Cleanup(func() {
		isolatedApprovalGroups.Delete(key)
		for _, m := range saved {
			_, _ = pool.Exec(ctx,
				`INSERT INTO team_member (id, created_on, updated_on, created_by, updated_by, team_id, user_id, group_id)
				 VALUES ($1::uuid, now(), now(), 'cr-flow-test', 'cr-flow-test', $2::uuid, $3::uuid, $4::uuid) ON CONFLICT (id) DO NOTHING`,
				m.id, m.team, m.user, groupID)
		}
	})
}

type crFlow struct {
	t      *testing.T
	pool   *pgxpool.Pool
	scoped *repository.Scoped
	repo   repository.ChangeRequestRepository
	sys    context.Context
}

func newCRFlow(t *testing.T) *crFlow {
	t.Helper()
	f := newCRFlowNoIsolation(t)
	// Tests that seed no CAB/ECAB members expect those groups empty, and so for
	// the Devops Approval fallback group (the local seed populates all three).
	isolateApprovalGroup(t, f.scoped, crCABGroupID)
	isolateApprovalGroup(t, f.scoped, crECABGroupID)
	isolateGroupsNamed(t, f.scoped, domain.PeerApprovalFallbackGroupName)
	return f
}

// newCRFlowNoIsolation is newCRFlow without emptying the CAB / ECAB / Devops
// Approval groups: for tests about what a seeded database holds in them.
func newCRFlowNoIsolation(t *testing.T) *crFlow {
	t.Helper()
	dsn := os.Getenv("CHANGE_REQUEST_TEST_DSN")
	if dsn == "" {
		t.Skip("CHANGE_REQUEST_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	scoped := repository.NewScoped(pool)
	f := &crFlow{t: t, pool: pool, scoped: scoped, repo: repository.NewChangeRequestRepository(scoped), sys: repository.WithSystemIdentity(context.Background())}

	clean := func() {
		_, _ = scoped.Exec(f.sys, `DELETE FROM work_item WHERE subject = $1`, crFlowSubject)
		for _, g := range []string{crFlowGroupID, crFlowSREGroupID, crFlowDevopsGroupID} {
			_, _ = scoped.Exec(f.sys, `DELETE FROM "group" WHERE id = $1`, g)
		}
		_, _ = scoped.Exec(f.sys, `DELETE FROM team WHERE id = $1`, crFlowSRETeamID)
	}
	clean()
	t.Cleanup(clean)
	return f
}

// isolateGroupsNamed is isolateApprovalGroup for every "group" row of the
// given name (the Devops Approval fallback group may exist more than once, and
// namedGroup counts the members of all of them): their members are removed for
// the duration of the calling test and put back on cleanup.
func isolateGroupsNamed(t *testing.T, pool *repository.Scoped, name string) {
	t.Helper()
	ctx := repository.WithSystemIdentity(context.Background())
	rows, err := pool.Query(ctx,
		`SELECT id::text, team_id::text, user_id::text, group_id::text, role FROM team_member
		 WHERE group_id IN (SELECT id FROM "group" WHERE name = $1)`, name)
	if err != nil {
		t.Fatalf("snapshot members of groups named %q: %v", name, err)
	}
	type member struct{ id, team, user, group, role string }
	var saved []member
	for rows.Next() {
		var m member
		if err := rows.Scan(&m.id, &m.team, &m.user, &m.group, &m.role); err != nil {
			rows.Close()
			t.Fatalf("scan member: %v", err)
		}
		saved = append(saved, m)
	}
	rows.Close()
	if len(saved) == 0 {
		return
	}
	if _, err := pool.Exec(ctx, `DELETE FROM team_member WHERE group_id IN (SELECT id FROM "group" WHERE name = $1)`, name); err != nil {
		t.Fatalf("clear members of groups named %q: %v", name, err)
	}
	t.Cleanup(func() {
		for _, m := range saved {
			_, _ = pool.Exec(ctx,
				`INSERT INTO team_member (id, created_on, updated_on, created_by, updated_by, team_id, user_id, group_id, role)
				 VALUES ($1::uuid, now(), now(), 'cr-flow-test', 'cr-flow-test', $2::uuid, $3::uuid, $4::uuid, $5) ON CONFLICT (id) DO NOTHING`,
				m.id, m.team, m.user, m.group, m.role)
		}
	})
}

// seedAssignedGroup creates the assigned group: the creator, peers A/B and the
// outsider, all INTERNAL users.
func (f *crFlow) seedAssignedGroup() {
	f.t.Helper()
	if _, err := f.scoped.Exec(f.sys,
		`INSERT INTO "group" (id, created_on, updated_on, created_by, updated_by, name) VALUES ($1, now(), now(), 'cr-flow-test', 'cr-flow-test', 'CR Flow Assigned Group')`,
		crFlowGroupID); err != nil {
		f.t.Fatalf("seed assigned group: %v", err)
	}
	seedApprovalGroupMembers(f.t, f.scoped, crFlowGroupID, crFlowCreatorID, crFlowPeerAID, crFlowPeerBID)
	seedApprovalGroupMembers(f.t, f.scoped, crFlowGroupID, crFlowOutsiderID)
}

// seedSREGroup creates an SRE team the way a synced one looks (Apollo): a
// "team" row of type sre-abt and a "group" row with the same id, whose members
// -- the creator, two ordinary internal engineers and a customer -- have
// team_id = group_id = that id. A change assigned to it points at the group.
func (f *crFlow) seedSREGroup() {
	f.t.Helper()
	mustExec := func(sql string, args ...any) {
		f.t.Helper()
		if _, err := f.scoped.Exec(f.sys, sql, args...); err != nil {
			f.t.Fatalf("seed (%.60s): %v", sql, err)
		}
	}
	mustExec(`INSERT INTO team (id, created_on, updated_on, created_by, updated_by, name, type, key)
	          VALUES ($1, now(), now(), 'cr-flow-test', 'cr-flow-test', 'CR Flow SRE Team', 'sre-abt', 'crflow-sre')`, crFlowSRETeamID)
	mustExec(`INSERT INTO "group" (id, created_on, updated_on, created_by, updated_by, name) VALUES ($1, now(), now(), 'cr-flow-test', 'cr-flow-test', 'CR Flow SRE Team')`, crFlowSREGroupID)
	seedApprovalGroupMembers(f.t, f.scoped, crFlowSREGroupID, crFlowCreatorID, crFlowSREID, crFlowSREPeerID)
	seedExternalGroupMembers(f.t, f.scoped, crFlowSREGroupID, crFlowExternalID)
	for _, uid := range []string{crFlowCreatorID, crFlowSREID, crFlowSREPeerID, crFlowExternalID} {
		f.makeSRE(uid, crFlowSREGroupID)
	}
}

// makeSRE puts userID into the sre-abt team (team_id = the SRE team) while
// keeping their membership of group groupID.
func (f *crFlow) makeSRE(userID, groupID string) {
	f.t.Helper()
	if _, err := f.scoped.Exec(f.sys, `DELETE FROM team_member WHERE user_id = $1`, userID); err != nil {
		f.t.Fatalf("reset team_member: %v", err)
	}
	if _, err := f.scoped.Exec(f.sys,
		`INSERT INTO team_member (id, created_on, updated_on, created_by, updated_by, team_id, user_id, group_id)
		 VALUES (gen_random_uuid(), now(), now(), 'cr-flow-test', 'cr-flow-test', $1::uuid, $2, $3::uuid)`,
		crFlowSRETeamID, userID, groupID); err != nil {
		f.t.Fatalf("seed SRE team_member: %v", err)
	}
}

func (f *crFlow) create(typ domain.ChangeRequestType, groupID string) string {
	f.t.Helper()
	g := groupID
	resp, err := f.repo.CreateChangeRequest(f.sys, domain.CreateChangeRequestRequest{
		Subject: crFlowSubject, Type: &typ, GroupID: &g,
	}, crFlowEmail(crFlowCreatorID))
	if err != nil {
		f.t.Fatalf("CreateChangeRequest(%s): %v", typ, err)
	}
	// RequestedBy is the creator too (the portal sends the signed-in user).
	if _, err := f.scoped.Exec(f.sys, `UPDATE change_request SET requested_by_user_id = $1::uuid WHERE id = $2`, crFlowCreatorID, resp.ChangeRequest.ID); err != nil {
		f.t.Fatalf("set requested_by: %v", err)
	}
	return resp.ChangeRequest.ID
}

func (f *crFlow) patchState(id string, state domain.ChangeRequestState) (domain.ChangeRequest, error) {
	s := state
	return f.repo.PatchChangeRequest(f.sys, id, domain.PatchChangeRequestRequest{State: &s}, crFlowEmail(crFlowCreatorID))
}

func (f *crFlow) requestApproval(id string) {
	f.t.Helper()
	if _, err := f.patchState(id, domain.ChangeRequestStateAssess); err != nil {
		f.t.Fatalf("Request Approval ({state: assess}): %v", err)
	}
}

func (f *crFlow) state(id string) string {
	f.t.Helper()
	var st string
	if err := f.scoped.QueryRow(f.sys, `SELECT COALESCE(state::text, '') FROM change_request WHERE id = $1`, id).Scan(&st); err != nil {
		f.t.Fatalf("read state: %v", err)
	}
	return st
}

func (f *crFlow) legal(id string) []string {
	f.t.Helper()
	cr, err := f.repo.GetChangeRequestByID(f.sys, id)
	if err != nil {
		f.t.Fatalf("GetChangeRequestByID: %v", err)
	}
	return cr.LegalNextStates
}

type crFlowStage struct {
	label     string
	groupID   string
	approvers map[string]string
}

func (f *crFlow) stages(id string) []crFlowStage {
	f.t.Helper()
	rows, err := f.scoped.Query(f.sys,
		`SELECT id, COALESCE(checkpoint_label, ''), COALESCE(assignment_group_id::text, '') FROM approval_stage WHERE work_item_id = $1 ORDER BY created_on, id`, id)
	if err != nil {
		f.t.Fatalf("query stages: %v", err)
	}
	type raw struct{ id, label, group string }
	var raws []raw
	for rows.Next() {
		var r raw
		if err := rows.Scan(&r.id, &r.label, &r.group); err != nil {
			rows.Close()
			f.t.Fatalf("scan stage: %v", err)
		}
		raws = append(raws, r)
	}
	rows.Close()
	var out []crFlowStage
	for _, r := range raws {
		st := crFlowStage{label: r.label, groupID: r.group, approvers: map[string]string{}}
		arows, err := f.scoped.Query(f.sys, `SELECT approver_user_id::text, state FROM approval_stage_approver WHERE stage_id = $1`, r.id)
		if err != nil {
			f.t.Fatalf("query approvers: %v", err)
		}
		for arows.Next() {
			var uid, status string
			if err := arows.Scan(&uid, &status); err != nil {
				arows.Close()
				f.t.Fatalf("scan approver: %v", err)
			}
			st.approvers[uid] = status
		}
		arows.Close()
		out = append(out, st)
	}
	return out
}

func (f *crFlow) decide(id, userID, decision string) error {
	_, err := f.repo.DecideChangeRequestApproval(f.sys, id, userID, decision, crFlowEmail(userID))
	return err
}

func assertStates(t *testing.T, what string, got []string, want ...string) {
	t.Helper()
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("%s = %v, want %v", what, got, want)
	}
}

func assertApprovers(t *testing.T, what string, got map[string]string, want map[string]string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s approvers = %v, want %v", what, got, want)
	}
	for uid, st := range want {
		if got[uid] != st {
			t.Fatalf("%s approver %s = %q, want %q (all: %v)", what, uid, got[uid], st, got)
		}
	}
}

// Normal: New -> (Request Approval) Assess [Peer Approval] -> Authorize [CAB
// Approval] -> Scheduled automatically on CAB approval -> Implement -> Review
// -> Closed. Also pins legalNextStates at every step and that Scheduled is
// never offered or accepted manually.
func TestChangeRequestFlowIntegration_NormalFullLifecycle(t *testing.T) {
	f := newCRFlow(t)
	f.seedAssignedGroup()
	seedApprovalGroupMembers(t, f.scoped, crCABGroupID, crCABMemberUserID1, crCABMemberUserID2)
	id := f.create(domain.ChangeRequestTypeNormal, crFlowGroupID)

	if got := f.state(id); got != "NEW" {
		t.Fatalf("state after create = %q, want NEW", got)
	}
	assertStates(t, "legalNextStates(New)", f.legal(id), "assess", "canceled")

	// Request Approval -> Assess with the PEER stage: the two peers are
	// requested, the creator is cancelled (never approves their own change).
	f.requestApproval(id)
	if got := f.state(id); got != "ASSESS" {
		t.Fatalf("state after Request Approval = %q, want ASSESS", got)
	}
	stages := f.stages(id)
	if len(stages) != 1 || stages[0].label != "Peer Approval" || stages[0].groupID != crFlowGroupID {
		t.Fatalf("stages after Request Approval = %+v, want exactly one \"Peer Approval\" stage on the assigned group", stages)
	}
	assertApprovers(t, "peer stage", stages[0].approvers, map[string]string{
		crFlowCreatorID: "CANCELLED", crFlowPeerAID: "REQUESTED", crFlowPeerBID: "REQUESTED", crFlowOutsiderID: "REQUESTED",
	})
	assertStates(t, "legalNextStates(Assess)", f.legal(id), "authorize", "canceled")

	// There is no manual shortcut past the approvals.
	for _, target := range []domain.ChangeRequestState{domain.ChangeRequestStateScheduled, domain.ChangeRequestStateAuthorize} {
		_, err := f.patchState(id, target)
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) {
			t.Fatalf("manual {state: %s} from Assess err = %v (%T), want *apierror.ValidationError", target, err, err)
		}
	}
	if got := f.state(id); got != "ASSESS" {
		t.Fatalf("state after refused manual transitions = %q, want still ASSESS", got)
	}

	// Peer approval right after Request Approval: CAB is its own group and
	// comes next.
	if err := f.decide(id, crFlowPeerAID, "approved"); err != nil {
		t.Fatalf("peer approval: %v", err)
	}
	if got := f.state(id); got != "AUTHORIZE" {
		t.Fatalf("state after peer approval = %q, want AUTHORIZE", got)
	}
	stages = f.stages(id)
	if len(stages) != 2 || stages[1].label != "CAB Approval" || stages[1].groupID != crCABGroupID {
		t.Fatalf("stages after peer approval = %+v, want a second \"CAB Approval\" stage on the CAB group", stages)
	}
	assertApprovers(t, "peer stage after approval", stages[0].approvers, map[string]string{
		crFlowCreatorID: "CANCELLED", crFlowPeerAID: "APPROVED", crFlowPeerBID: "CANCELLED", crFlowOutsiderID: "CANCELLED",
	})
	assertApprovers(t, "CAB stage", stages[1].approvers, map[string]string{crCABMemberUserID1: "REQUESTED", crCABMemberUserID2: "REQUESTED"})
	assertStates(t, "legalNextStates(Authorize)", f.legal(id), "canceled")

	// A peer cannot also give the CAB approval (not in the CAB group), and no
	// manual Schedule exists.
	if err := f.decide(id, crFlowPeerBID, "approved"); err == nil {
		t.Fatal("a peer approver with no CAB row decided the CAB stage, want NotFound")
	}
	if _, err := f.patchState(id, domain.ChangeRequestStateScheduled); err == nil {
		t.Fatal("manual {state: scheduled} from Authorize succeeded, want a refusal")
	}

	// CAB approval moves the change to Scheduled by itself.
	if err := f.decide(id, crCABMemberUserID1, "approved"); err != nil {
		t.Fatalf("CAB approval: %v", err)
	}
	if got := f.state(id); got != "SCHEDULED" {
		t.Fatalf("state after CAB approval = %q, want SCHEDULED (automatic)", got)
	}
	assertStates(t, "legalNextStates(Scheduled)", f.legal(id), "implement", "canceled")

	for _, step := range []struct {
		to   domain.ChangeRequestState
		want string
	}{
		{domain.ChangeRequestStateImplement, "IMPLEMENT"},
		{domain.ChangeRequestStateReview, "REVIEW"},
		{domain.ChangeRequestStateClosed, "CLOSED"},
	} {
		if _, err := f.patchState(id, step.to); err != nil {
			t.Fatalf("PATCH state %s: %v", step.to, err)
		}
		if got := f.state(id); got != step.want {
			t.Fatalf("state after %s = %q, want %q", step.to, got, step.want)
		}
	}
	if got := f.legal(id); got != nil {
		t.Fatalf("legalNextStates(Closed) = %v, want none", got)
	}
}

// Emergency: no peer approval; Request Approval goes straight to Authorize
// with ONLY the ECAB stage (its own group, not CAB); ECAB approval schedules.
func TestChangeRequestFlowIntegration_EmergencyLifecycle(t *testing.T) {
	f := newCRFlow(t)
	f.seedAssignedGroup()
	// CAB members exist too -- an emergency must NOT involve them.
	seedApprovalGroupMembers(t, f.scoped, crCABGroupID, crCABMemberUserID1)
	seedApprovalGroupMembers(t, f.scoped, crECABGroupID, crECABMemberUserID)
	id := f.create(domain.ChangeRequestTypeEmergency, crFlowGroupID)

	assertStates(t, "legalNextStates(New)", f.legal(id), "assess", "canceled")
	f.requestApproval(id)

	if got := f.state(id); got != "AUTHORIZE" {
		t.Fatalf("emergency state after Request Approval = %q, want AUTHORIZE (no peer step)", got)
	}
	stages := f.stages(id)
	if len(stages) != 1 || stages[0].label != "ECAB Approval" || stages[0].groupID != crECABGroupID {
		t.Fatalf("emergency stages = %+v, want exactly one \"ECAB Approval\" stage on the ECAB group", stages)
	}
	assertApprovers(t, "ECAB stage", stages[0].approvers, map[string]string{crECABMemberUserID: "REQUESTED"})
	assertStates(t, "legalNextStates(Authorize)", f.legal(id), "canceled")

	// Neither a peer nor a regular CAB member can decide an emergency.
	for _, uid := range []string{crFlowPeerAID, crCABMemberUserID1} {
		if err := f.decide(id, uid, "approved"); err == nil {
			t.Fatalf("user %s decided the ECAB stage without an ECAB row", uid)
		}
	}

	if err := f.decide(id, crECABMemberUserID, "approved"); err != nil {
		t.Fatalf("ECAB approval: %v", err)
	}
	if got := f.state(id); got != "SCHEDULED" {
		t.Fatalf("state after ECAB approval = %q, want SCHEDULED (automatic)", got)
	}
	assertStates(t, "legalNextStates(Scheduled)", f.legal(id), "implement", "canceled")
	if len(f.stages(id)) != 1 {
		t.Fatalf("emergency ended with %d stages, want 1", len(f.stages(id)))
	}
}

// Standard: no approval stages at all; Request Approval goes straight to
// Scheduled, then Implement -> Review -> Closed.
func TestChangeRequestFlowIntegration_StandardLifecycle(t *testing.T) {
	f := newCRFlow(t)
	f.seedAssignedGroup()
	id := f.create(domain.ChangeRequestTypeStandard, crFlowGroupID)

	assertStates(t, "legalNextStates(New)", f.legal(id), "assess", "canceled")
	f.requestApproval(id)

	if got := f.state(id); got != "SCHEDULED" {
		t.Fatalf("standard state after Request Approval = %q, want SCHEDULED (no approval needed)", got)
	}
	if n := len(f.stages(id)); n != 0 {
		t.Fatalf("standard change has %d approval stages, want none (neither Peer nor CAB)", n)
	}
	assertStates(t, "legalNextStates(Scheduled)", f.legal(id), "implement", "canceled")

	// Request Approval is only legal from New: it must not drag a Scheduled
	// change back.
	if _, err := f.patchState(id, domain.ChangeRequestStateAssess); err != nil {
		t.Fatalf("idempotent resend of Request Approval on a Scheduled Standard change: %v", err)
	}
	for _, to := range []domain.ChangeRequestState{domain.ChangeRequestStateImplement, domain.ChangeRequestStateReview} {
		if _, err := f.patchState(id, to); err != nil {
			t.Fatalf("PATCH state %s: %v", to, err)
		}
	}
	if _, err := f.patchState(id, domain.ChangeRequestStateClosed); err != nil {
		t.Fatalf("PATCH state closed: %v", err)
	}
	if n := len(f.stages(id)); n != 0 {
		t.Fatalf("standard change ended with %d approval stages, want none (Review approval is Normal-only)", n)
	}
}

// Request Approval is only legal from New; once the change has moved on it is
// refused rather than dragging the state backward.
func TestChangeRequestFlowIntegration_RequestApprovalOnlyFromNew(t *testing.T) {
	f := newCRFlow(t)
	f.seedAssignedGroup()
	id := f.create(domain.ChangeRequestTypeStandard, crFlowGroupID)
	f.requestApproval(id)
	if _, err := f.patchState(id, domain.ChangeRequestStateImplement); err != nil {
		t.Fatalf("implement: %v", err)
	}
	_, err := f.patchState(id, domain.ChangeRequestStateAssess)
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("Request Approval on an Implement change err = %v (%T), want *apierror.ValidationError", err, err)
	}
	if got := f.state(id); got != "IMPLEMENT" {
		t.Fatalf("state after refused Request Approval = %q, want IMPLEMENT", got)
	}
}

// The creator may never approve their own change request -- neither the
// peer stage nor CAB/ECAB -- but may still cancel it.
func TestChangeRequestFlowIntegration_CreatorCannotApproveAnyStage(t *testing.T) {
	f := newCRFlow(t)
	f.seedAssignedGroup()
	seedApprovalGroupMembers(t, f.scoped, crCABGroupID, crCABMemberUserID1)
	id := f.create(domain.ChangeRequestTypeNormal, crFlowGroupID)
	f.requestApproval(id)

	assertForbidden := func(what, userID string) {
		t.Helper()
		err := f.decide(id, userID, "approved")
		var fe *apierror.ForbiddenError
		if !errors.As(err, &fe) {
			t.Fatalf("%s: err = %v (%T), want *apierror.ForbiddenError", what, err, err)
		}
		if !strings.Contains(fe.Msg, "creator") {
			t.Fatalf("%s: message %q should say the creator cannot approve", what, fe.Msg)
		}
	}

	// Peer stage: the creator's row was born cancelled; even a REQUESTED row
	// forced in by drift must not be decidable.
	assertForbidden("creator on the peer stage (cancelled row)", crFlowCreatorID)
	if _, err := f.scoped.Exec(f.sys,
		`UPDATE approval_stage_approver SET state = 'REQUESTED' WHERE work_item_id = $1 AND approver_user_id = $2`, id, crFlowCreatorID); err != nil {
		t.Fatalf("force creator row to requested: %v", err)
	}
	assertForbidden("creator on the peer stage (requested row)", crFlowCreatorID)
	rejectErr := f.decide(id, crFlowCreatorID, "rejected")
	var fe *apierror.ForbiddenError
	if !errors.As(rejectErr, &fe) {
		t.Fatalf("creator rejecting err = %v, want ForbiddenError (the creator does not decide at all)", rejectErr)
	}

	// CAB stage: put the creator in the CAB group's stage by force.
	if err := f.decide(id, crFlowPeerAID, "approved"); err != nil {
		t.Fatalf("peer approval: %v", err)
	}
	if _, err := f.scoped.Exec(f.sys,
		`INSERT INTO approval_stage_approver (id, created_on, updated_on, created_by, updated_by, stage_id, work_item_id, approver_user_id, state)
		 SELECT gen_random_uuid(), now(), now(), 'cr-flow-test', 'cr-flow-test', s.id, s.work_item_id, $2::uuid, 'REQUESTED'
		 FROM approval_stage s WHERE s.work_item_id = $1 AND s.checkpoint_label = 'CAB Approval'`, id, crFlowCreatorID); err != nil {
		t.Fatalf("force creator onto the CAB stage: %v", err)
	}
	assertForbidden("creator on the CAB stage", crFlowCreatorID)
	if got := f.state(id); got != "AUTHORIZE" {
		t.Fatalf("state after refused creator approvals = %q, want AUTHORIZE (unchanged)", got)
	}

	// The creator may still cancel.
	if _, err := f.patchState(id, domain.ChangeRequestStateCanceled); err != nil {
		t.Fatalf("creator cancelling their own change: %v", err)
	}
	if got := f.state(id); got != "CANCELED" {
		t.Fatalf("state after cancel = %q, want CANCELED", got)
	}
}

// The creator is also recognised through work_item.created_by (their email)
// when change_request.requested_by_user_id was never set.
func TestChangeRequestFlowIntegration_CreatorRecognisedByCreatedByEmail(t *testing.T) {
	f := newCRFlow(t)
	f.seedAssignedGroup()
	seedApprovalGroupMembers(t, f.scoped, crCABGroupID, crCABMemberUserID1)
	id := f.create(domain.ChangeRequestTypeNormal, crFlowGroupID)
	if _, err := f.scoped.Exec(f.sys, `UPDATE change_request SET requested_by_user_id = NULL WHERE id = $1`, id); err != nil {
		t.Fatalf("clear requested_by: %v", err)
	}
	f.requestApproval(id)
	stages := f.stages(id)
	if got := stages[0].approvers[crFlowCreatorID]; got != "CANCELLED" {
		t.Fatalf("creator (identified only by created_by email) peer row = %q, want CANCELLED", got)
	}
}

// helpers shared by the INTERNAL-only pool tests ---------------------------

// seedDevopsGroup creates the Devops Approval peer fallback group with two
// internal members and one customer.
func (f *crFlow) seedDevopsGroup() {
	f.t.Helper()
	if _, err := f.scoped.Exec(f.sys,
		`INSERT INTO "group" (id, created_on, updated_on, created_by, updated_by, name) VALUES ($1, now(), now(), 'cr-flow-test', 'cr-flow-test', $2)`,
		crFlowDevopsGroupID, domain.PeerApprovalFallbackGroupName); err != nil {
		f.t.Fatalf("seed devops group: %v", err)
	}
	seedApprovalGroupMembers(f.t, f.scoped, crFlowDevopsGroupID, crDevopsMemberID1, crDevopsMemberID2)
	seedExternalGroupMembers(f.t, f.scoped, crFlowDevopsGroupID, crDevopsExternalID)
}

// forceApprover puts userID on the change's (latest) stage with the given label
// as a REQUESTED approver -- the drift the decision-time guard exists for: a row
// that predates the INTERNAL-only rule, or a user whose type or membership
// changed after provisioning.
func (f *crFlow) forceApprover(id, label, userID string) {
	f.t.Helper()
	if _, err := f.scoped.Exec(f.sys,
		`INSERT INTO approval_stage_approver (id, created_on, updated_on, created_by, updated_by, stage_id, work_item_id, approver_user_id, state)
		 SELECT gen_random_uuid(), now(), now(), 'cr-flow-test', 'cr-flow-test', s.id, s.work_item_id, $3::uuid, 'REQUESTED'
		 FROM approval_stage s WHERE s.work_item_id = $1 AND s.checkpoint_label = $2
		 ORDER BY s.created_on DESC, s.id DESC LIMIT 1`, id, label, userID); err != nil {
		f.t.Fatalf("force %s onto the %s stage: %v", userID, label, err)
	}
}

// stage returns the change's stage with the given label (the first, if the
// stage was repeated), failing the test when there is none.
func (f *crFlow) stage(id, label string) crFlowStage {
	f.t.Helper()
	for _, st := range f.stages(id) {
		if st.label == label {
			return st
		}
	}
	f.t.Fatalf("change %s has no %q stage (stages: %v)", id, label, f.labels(id))
	return crFlowStage{}
}

// wantPeerPool asserts the change's Peer Approval stage is on groupID with
// exactly the given approvers.
func (f *crFlow) wantPeerPool(id, groupID string, want map[string]string) {
	f.t.Helper()
	st := f.stage(id, "Peer Approval")
	if st.groupID != groupID {
		f.t.Fatalf("peer stage group = %s, want %s", st.groupID, groupID)
	}
	assertApprovers(f.t, "peer stage", st.approvers, want)
}

// Peer approval is the WHOLE assigned group: when that group is an SRE team
// (Apollo-like: a sre-abt team and its same-id group), its active internal
// members are the peer approvers -- belonging to an SRE team neither keeps
// anyone out of the pool nor stops them deciding. The creator is still listed,
// cancelled, and a customer who is a member of the team is not provisioned.
func TestChangeRequestFlowIntegration_SRETeamAssignedGroupMembersArePeerApprovers(t *testing.T) {
	f := newCRFlow(t)
	f.seedSREGroup()
	seedApprovalGroupMembers(t, f.scoped, crCABGroupID, crCABMemberUserID1)
	// The Devops Approval fallback must not be what supplies the approvers.
	f.seedDevopsGroup()
	id := f.create(domain.ChangeRequestTypeNormal, crFlowSREGroupID)
	f.requestApproval(id)

	f.wantPeerPool(id, crFlowSREGroupID, map[string]string{
		crFlowCreatorID: "CANCELLED", crFlowSREID: "REQUESTED", crFlowSREPeerID: "REQUESTED",
	})

	// They may decide, and see it: canDecide on their own row.
	if got := f.canDecideAs(id, crFlowSREID); len(got) != 1 || !got["Peer Approval/"+crFlowSREID] {
		t.Fatalf("canDecide for an SRE-team peer = %v, want their own Peer Approval row", got)
	}
	if err := f.decide(id, crFlowSREID, "approved"); err != nil {
		t.Fatalf("SRE-team peer approval: %v", err)
	}
	f.expect(id, "after the SRE-team peer approved", "AUTHORIZE", "canceled")
	assertApprovers(t, "peer stage after approval", f.stage(id, "Peer Approval").approvers, map[string]string{
		crFlowCreatorID: "CANCELLED", crFlowSREID: "APPROVED", crFlowSREPeerID: "CANCELLED",
	})
	if f.stage(id, "CAB Approval").groupID != crCABGroupID {
		t.Fatal("the CAB stage was not provisioned on the CAB group")
	}
}

// The Devops Approval group is the peer pool's fallback and nothing else: only
// when the assigned group yields no eligible member -- no active internal
// member other than the creator -- is it used, under the same rules (internal
// users only, creator cancelled). While the assigned group has an eligible
// member, a populated Devops Approval group is ignored.
func TestChangeRequestFlowIntegration_PeerPoolFallsBackToDevopsApprovalOnlyWhenAssignedGroupYieldsNobody(t *testing.T) {
	newFlow := func(t *testing.T) *crFlow {
		f := newCRFlow(t)
		seedApprovalGroupMembers(t, f.scoped, crCABGroupID, crCABMemberUserID1)
		if _, err := f.scoped.Exec(f.sys,
			`INSERT INTO "group" (id, created_on, updated_on, created_by, updated_by, name) VALUES ($1, now(), now(), 'cr-flow-test', 'cr-flow-test', 'CR Flow Assigned Group')`,
			crFlowGroupID); err != nil {
			t.Fatalf("seed assigned group: %v", err)
		}
		return f
	}
	devopsPool := map[string]string{crDevopsMemberID1: "REQUESTED", crDevopsMemberID2: "REQUESTED"}

	t.Run("assigned group with an eligible member does not use Devops Approval", func(t *testing.T) {
		f := newFlow(t)
		f.seedDevopsGroup()
		seedApprovalGroupMembers(t, f.scoped, crFlowGroupID, crFlowCreatorID, crFlowPeerAID)
		id := f.create(domain.ChangeRequestTypeNormal, crFlowGroupID)
		f.requestApproval(id)
		f.wantPeerPool(id, crFlowGroupID, map[string]string{crFlowCreatorID: "CANCELLED", crFlowPeerAID: "REQUESTED"})
	})

	t.Run("assigned group of customers only falls back", func(t *testing.T) {
		f := newFlow(t)
		f.seedDevopsGroup()
		seedApprovalGroupMembers(t, f.scoped, crFlowGroupID, crFlowCreatorID)
		seedExternalGroupMembers(t, f.scoped, crFlowGroupID, crFlowExternalID, crFlowExternalID2)
		id := f.create(domain.ChangeRequestTypeNormal, crFlowGroupID)
		f.requestApproval(id)
		// The customer in the Devops group is skipped there too.
		f.wantPeerPool(id, crFlowDevopsGroupID, devopsPool)
	})

	t.Run("assigned group with only the creator falls back", func(t *testing.T) {
		f := newFlow(t)
		f.seedDevopsGroup()
		seedApprovalGroupMembers(t, f.scoped, crFlowGroupID, crFlowCreatorID)
		id := f.create(domain.ChangeRequestTypeNormal, crFlowGroupID)
		f.requestApproval(id)
		f.wantPeerPool(id, crFlowDevopsGroupID, devopsPool)
	})

	t.Run("assigned group of an inactive internal user and a user with no type falls back", func(t *testing.T) {
		f := newFlow(t)
		f.seedDevopsGroup()
		seedApprovalGroupMembers(t, f.scoped, crFlowGroupID, crFlowCreatorID, crFlowInactiveID)
		f.execSQL(`UPDATE "user" SET is_active = false WHERE id = $1`, crFlowInactiveID)
		seedGroupMembersOfType(t, f.scoped, crFlowGroupID, "NOT_AVAILABLE", crFlowNoTypeID)
		id := f.create(domain.ChangeRequestTypeNormal, crFlowGroupID)
		f.requestApproval(id)
		f.wantPeerPool(id, crFlowDevopsGroupID, devopsPool)
	})

	t.Run("an SRE-team assigned group with nobody eligible falls back like any other", func(t *testing.T) {
		f := newFlow(t)
		f.seedDevopsGroup()
		f.seedSREGroup()
		// Only the creator and a customer are left in the SRE group.
		f.execSQL(`DELETE FROM team_member WHERE user_id IN ($1, $2)`, crFlowSREID, crFlowSREPeerID)
		id := f.create(domain.ChangeRequestTypeNormal, crFlowSREGroupID)
		f.requestApproval(id)
		f.wantPeerPool(id, crFlowDevopsGroupID, devopsPool)
	})

	t.Run("nobody eligible anywhere is refused, and says why", func(t *testing.T) {
		f := newFlow(t)
		// No Devops Approval members at all (the group may not even exist).
		seedApprovalGroupMembers(t, f.scoped, crFlowGroupID, crFlowCreatorID)
		seedExternalGroupMembers(t, f.scoped, crFlowGroupID, crFlowExternalID)
		id := f.create(domain.ChangeRequestTypeNormal, crFlowGroupID)
		_, err := f.patchState(id, domain.ChangeRequestStateAssess)
		f.wantValidationError("Request Approval with no eligible peer", err, "no eligible peer approvers")
		f.wantValidationError("Request Approval with no eligible peer", err, "external/customer users cannot approve")
		f.wantValidationError("Request Approval with no eligible peer", err, domain.PeerApprovalFallbackGroupName)
		if got := f.state(id); got != "NEW" {
			t.Fatalf("state after refused Request Approval = %q, want NEW", got)
		}
		if n := len(f.stages(id)); n != 0 {
			t.Fatalf("refused Request Approval left %d stages", n)
		}
	})
}

// An EXTERNAL (customer) member of the assigned team is never provisioned as
// an approver of an internal stage -- Peer, CAB, ECAB or Review -- and neither
// is an inactive user or one with no derivable type; the creator stays a
// cancelled row. Mixed pools keep exactly their active internal users.
func TestChangeRequestFlowIntegration_MixedPoolsKeepOnlyActiveInternalUsers(t *testing.T) {
	f := newCRFlow(t)
	f.seedAssignedGroup()
	seedExternalGroupMembers(t, f.scoped, crFlowGroupID, crFlowExternalID, crFlowExternalID2)
	seedApprovalGroupMembers(t, f.scoped, crFlowGroupID, crFlowInactiveID)
	f.execSQL(`UPDATE "user" SET is_active = false WHERE id = $1`, crFlowInactiveID)
	seedGroupMembersOfType(t, f.scoped, crFlowGroupID, "NOT_AVAILABLE", crFlowNoTypeID)
	seedApprovalGroupMembers(t, f.scoped, crCABGroupID, crCABMemberUserID1)
	seedExternalGroupMembers(t, f.scoped, crCABGroupID, crCABExternalID)
	seedApprovalGroupMembers(t, f.scoped, crECABGroupID, crECABMemberUserID)
	seedExternalGroupMembers(t, f.scoped, crECABGroupID, crECABExternalID)

	// Normal: Peer, then CAB, then (after Implement) Review.
	id := f.create(domain.ChangeRequestTypeNormal, crFlowGroupID)
	f.requestApproval(id)
	f.wantPeerPool(id, crFlowGroupID, map[string]string{
		crFlowCreatorID: "CANCELLED", crFlowPeerAID: "REQUESTED", crFlowPeerBID: "REQUESTED", crFlowOutsiderID: "REQUESTED",
	})
	if err := f.decide(id, crFlowPeerAID, "approved"); err != nil {
		t.Fatalf("peer approval: %v", err)
	}
	assertApprovers(t, "CAB stage", f.stage(id, "CAB Approval").approvers, map[string]string{crCABMemberUserID1: "REQUESTED"})
	if err := f.decide(id, crCABMemberUserID1, "approved"); err != nil {
		t.Fatalf("CAB approval: %v", err)
	}
	f.step(id, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
	f.step(id, domain.ChangeRequestStateReview, "REVIEW", "closed", "rollback", "canceled")
	assertApprovers(t, "Review stage", f.stage(id, "Review").approvers, map[string]string{
		crFlowCreatorID: "CANCELLED", crFlowPeerAID: "REQUESTED", crFlowPeerBID: "REQUESTED", crFlowOutsiderID: "REQUESTED",
	})

	// Emergency: ECAB only.
	eid := f.create(domain.ChangeRequestTypeEmergency, crFlowGroupID)
	f.requestApproval(eid)
	assertApprovers(t, "ECAB stage", f.stage(eid, "ECAB Approval").approvers, map[string]string{crECABMemberUserID: "REQUESTED"})
}

// A group made only of customers has no one to give an internal approval: the
// request is refused up front, with a message that names the group and says
// customers cannot approve -- for the CAB, the ECAB, and the Review stage
// (whose assigned team has lost its internal members by then).
func TestChangeRequestFlowIntegration_AllExternalGroupsAreRefusedClearly(t *testing.T) {
	t.Run("CAB", func(t *testing.T) {
		f := newCRFlow(t)
		f.seedAssignedGroup()
		seedExternalGroupMembers(t, f.scoped, crCABGroupID, crCABExternalID)
		id := f.create(domain.ChangeRequestTypeNormal, crFlowGroupID)
		_, err := f.patchState(id, domain.ChangeRequestStateAssess)
		f.wantValidationError("CAB of customers only", err, `the "CAB Approval" group has no active internal (WSO2) members`)
		f.wantValidationError("CAB of customers only", err, "external/customer users")
		if got := f.state(id); got != "NEW" {
			t.Fatalf("state after refused Request Approval = %q, want NEW", got)
		}
		if n := len(f.stages(id)); n != 0 {
			t.Fatalf("refused Request Approval left %d stages", n)
		}
	})
	t.Run("ECAB", func(t *testing.T) {
		f := newCRFlow(t)
		f.seedAssignedGroup()
		seedExternalGroupMembers(t, f.scoped, crECABGroupID, crECABExternalID)
		id := f.create(domain.ChangeRequestTypeEmergency, crFlowGroupID)
		_, err := f.patchState(id, domain.ChangeRequestStateAssess)
		f.wantValidationError("ECAB of customers only", err, `the "ECAB Approval" group has no active internal (WSO2) members`)
	})
	t.Run("CAB cascade rolls the peer decision back", func(t *testing.T) {
		f := newCRFlow(t)
		f.seedAssignedGroup()
		seedApprovalGroupMembers(t, f.scoped, crCABGroupID, crCABMemberUserID1)
		id := f.create(domain.ChangeRequestTypeNormal, crFlowGroupID)
		f.requestApproval(id)
		// The CAB's only member becomes a customer after the peer stage exists.
		f.execSQL(`UPDATE "user" SET user_type = 'EXTERNAL'::user_type_enum WHERE id = $1`, crCABMemberUserID1)
		err := f.decide(id, crFlowPeerAID, "approved")
		f.wantValidationError("peer approval into a CAB of customers only", err, `the "CAB Approval" group has no active internal (WSO2) members`)
		f.expect(id, "after the refused peer approval", "ASSESS", "authorize", "canceled")
	})
	t.Run("Review", func(t *testing.T) {
		f := newCRFlow(t)
		f.seedAssignedGroup()
		seedApprovalGroupMembers(t, f.scoped, crCABGroupID, crCABMemberUserID1)
		id := f.create(domain.ChangeRequestTypeNormal, crFlowGroupID)
		f.requestApproval(id)
		f.approvePeerAndCAB(id, "SCHEDULED", "implement", "canceled")
		f.step(id, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
		// The assigned team turns out to be customers only.
		f.execSQL(`UPDATE "user" SET user_type = 'EXTERNAL'::user_type_enum WHERE id IN ($1, $2, $3, $4)`,
			crFlowCreatorID, crFlowPeerAID, crFlowPeerBID, crFlowOutsiderID)
		_, err := f.patchState(id, domain.ChangeRequestStateReview)
		f.wantValidationError("Review of a team of customers", err, "the assigned team has no active internal (WSO2) members")
		f.expect(id, "after the refused review", "IMPLEMENT", "review", "canceled")
	})
}

// Decision time: an external user (or an inactive one, or one whose type
// changed after the row was written) holding a REQUESTED row on an internal
// stage is refused with a readable 403 and sees canDecide=false, on every
// internal stage; the internal approvers beside them are unaffected.
func TestChangeRequestFlowIntegration_ExternalUserCannotDecideInternalStage(t *testing.T) {
	f := newCRFlow(t)
	f.seedAssignedGroup()
	seedExternalGroupMembers(t, f.scoped, crFlowGroupID, crFlowExternalID)
	seedApprovalGroupMembers(t, f.scoped, crCABGroupID, crCABMemberUserID1)
	id := f.create(domain.ChangeRequestTypeNormal, crFlowGroupID)
	f.requestApproval(id)

	wantRefused := func(stage, userID string) {
		t.Helper()
		f.wantForbidden(stage+" decision by "+userID, f.decide(id, userID, "approved"), "only active internal (WSO2) users")
		f.wantForbidden(stage+" rejection by "+userID, f.decide(id, userID, "rejected"), stage)
		if got := f.canDecideAs(id, userID); len(got) != 0 {
			t.Fatalf("canDecide for %s on %s = %v, want none", userID, stage, got)
		}
	}

	// Peer: a stale REQUESTED row for the customer who is a member of the team.
	f.forceApprover(id, "Peer Approval", crFlowExternalID)
	wantRefused("Peer Approval", crFlowExternalID)
	// An internal peer beside them can decide, and sees it.
	if got := f.canDecideAs(id, crFlowPeerAID); len(got) != 1 || !got["Peer Approval/"+crFlowPeerAID] {
		t.Fatalf("canDecide for an internal peer = %v, want only their own row", got)
	}
	// A peer whose type changed after provisioning is refused too.
	f.execSQL(`UPDATE "user" SET user_type = 'EXTERNAL'::user_type_enum WHERE id = $1`, crFlowPeerBID)
	wantRefused("Peer Approval", crFlowPeerBID)
	f.execSQL(`UPDATE "user" SET user_type = 'INTERNAL'::user_type_enum, is_active = false WHERE id = $1`, crFlowPeerBID)
	f.wantForbidden("an inactive peer deciding", f.decide(id, crFlowPeerBID, "approved"), "only active internal (WSO2) users")
	f.wantState(id, "ASSESS")
	if st := f.stage(id, "Peer Approval"); st.approvers[crFlowExternalID] != "REQUESTED" {
		t.Fatalf("refused decision changed the customer's row: %v", st.approvers)
	}

	// The internal peers are unaffected: approval cascades to CAB.
	if err := f.decide(id, crFlowPeerAID, "approved"); err != nil {
		t.Fatalf("internal peer approval: %v", err)
	}
	// CAB stage.
	f.forceApprover(id, "CAB Approval", crFlowExternalID)
	wantRefused("CAB Approval", crFlowExternalID)
	f.wantState(id, "AUTHORIZE")
	if err := f.decide(id, crCABMemberUserID1, "approved"); err != nil {
		t.Fatalf("internal CAB approval: %v", err)
	}
	f.wantState(id, "SCHEDULED")

	// Review stage.
	f.step(id, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
	f.step(id, domain.ChangeRequestStateReview, "REVIEW", "closed", "rollback", "canceled")
	f.forceApprover(id, "Review", crFlowExternalID)
	wantRefused("Review", crFlowExternalID)

	// ECAB stage of an Emergency change.
	seedApprovalGroupMembers(t, f.scoped, crECABGroupID, crECABMemberUserID)
	eid := f.create(domain.ChangeRequestTypeEmergency, crFlowGroupID)
	f.requestApproval(eid)
	f.forceApprover(eid, "ECAB Approval", crFlowExternalID)
	f.wantForbidden("ECAB decision by a customer", f.decide(eid, crFlowExternalID, "approved"), "ECAB Approval")
	if got := f.canDecideAs(eid, crFlowExternalID); len(got) != 0 {
		t.Fatalf("canDecide for a customer on the ECAB stage = %v, want none", got)
	}
	f.wantState(eid, "AUTHORIZE")
	if err := f.decide(eid, crECABMemberUserID, "approved"); err != nil {
		t.Fatalf("internal ECAB approval: %v", err)
	}
	f.wantState(eid, "SCHEDULED")
}

// wantState asserts the stored state is want (used after a refused decision,
// which must not move the change).
func (f *crFlow) wantState(id, want string) {
	f.t.Helper()
	if got := f.state(id); got != want {
		f.t.Fatalf("state = %q, want %q", got, want)
	}
}

// The customer stages are not subject to the INTERNAL-only rule: the project's
// registered (external) contacts are asked and decide them -- Customer Approval
// then Customer Review -- while a customer who is only a member of the assigned
// team (not a contact of the project) cannot.
func TestChangeRequestFlowIntegration_CustomerStagesStillWorkForExternalContacts(t *testing.T) {
	f := newCustomerGroupFlow(t)
	seedExternalGroupMembers(t, f.scoped, crFlowGroupID, crFlowExternalID)
	for _, uid := range []string{crScopeUserA1, crScopeUserA2} {
		var typ string
		if err := f.scoped.QueryRow(f.sys, `SELECT user_type::text FROM "user" WHERE id = $1`, uid).Scan(&typ); err != nil || typ != userTypeExternal {
			t.Fatalf("customer contact %s user_type = %q (%v), want EXTERNAL", uid, typ, err)
		}
	}
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, true)
	f.driveToCustomerApproval(id)

	assertApprovers(t, "Customer Approval", f.customerStages(id)[0].approvers, map[string]string{crScopeUserA1: "REQUESTED", crScopeUserA2: "REQUESTED"})
	if got := f.canDecideAs(id, crScopeUserA1); len(got) != 1 || !got[stageCustApproval+"/"+crScopeUserA1] {
		t.Fatalf("canDecide for an external contact = %v, want their own Customer Approval row", got)
	}
	// A customer on the team but not a contact of the project is told so.
	f.wantForbidden("a team customer who is no contact", f.decide(id, crFlowExternalID, "approved"), "only members of the customer group")
	if err := f.decide(id, crScopeUserA1, "approved"); err != nil {
		t.Fatalf("external contact approving Customer Approval: %v", err)
	}
	f.expect(id, "after the contact approved", "SCHEDULED", "implement", "canceled")

	f.driveToCustomerReview(id)
	if got := f.canDecideAs(id, crScopeUserA2); len(got) != 1 || !got[stageCustReview+"/"+crScopeUserA2] {
		t.Fatalf("canDecide for an external contact on Customer Review = %v", got)
	}
	if err := f.decide(id, crScopeUserA2, "approved"); err != nil {
		t.Fatalf("external contact approving Customer Review: %v", err)
	}
	f.expect(id, "after the contact reviewed", "CLOSED")
}

// The stage approver rows after every step of a Normal change (both customer
// boxes ticked) whose assigned team, CAB group and project mix internal and
// external people: the internal stages hold exactly the active internal users
// (creator cancelled), the customer stages exactly the project's contacts.
func TestChangeRequestFlowIntegration_LifecycleStageRowsWithInternalAndExternalMembers(t *testing.T) {
	f := newCustomerGroupFlow(t)
	seedExternalGroupMembers(t, f.scoped, crFlowGroupID, crFlowExternalID, crFlowExternalID2)
	seedExternalGroupMembers(t, f.scoped, crCABGroupID, crCABExternalID)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, true)

	labels := func(want ...string) {
		t.Helper()
		if got := strings.Join(f.labels(id), ","); got != strings.Join(want, ",") {
			t.Fatalf("stages = %s, want %s", got, strings.Join(want, ","))
		}
	}
	labels()

	// 1. Request Approval -> Peer Approval: the team's internal members.
	f.requestApproval(id)
	labels("Peer Approval")
	assertApprovers(t, "peer", f.stage(id, "Peer Approval").approvers, map[string]string{
		crFlowCreatorID: "CANCELLED", crFlowPeerAID: "REQUESTED", crFlowPeerBID: "REQUESTED", crFlowOutsiderID: "REQUESTED",
	})

	// 2. A peer approves -> Authorize, CAB Approval: the CAB's internal members.
	if err := f.decide(id, crFlowPeerAID, "approved"); err != nil {
		t.Fatalf("peer approval: %v", err)
	}
	f.expect(id, "after the peer approved", "AUTHORIZE", "canceled")
	labels("Peer Approval", "CAB Approval")
	assertApprovers(t, "peer", f.stage(id, "Peer Approval").approvers, map[string]string{
		crFlowCreatorID: "CANCELLED", crFlowPeerAID: "APPROVED", crFlowPeerBID: "CANCELLED", crFlowOutsiderID: "CANCELLED",
	})
	assertApprovers(t, "CAB", f.stage(id, "CAB Approval").approvers, map[string]string{crCABMemberUserID1: "REQUESTED", crCABMemberUserID2: "REQUESTED"})

	// 3. CAB approves -> Customer Approval: exactly the project's contacts.
	if err := f.decide(id, crCABMemberUserID2, "approved"); err != nil {
		t.Fatalf("CAB approval: %v", err)
	}
	f.expect(id, "after CAB approved", "CUSTOMER_APPROVAL", "authorize", "canceled")
	labels("Peer Approval", "CAB Approval", "Customer Approval")
	assertApprovers(t, "CAB", f.stage(id, "CAB Approval").approvers, map[string]string{crCABMemberUserID1: "CANCELLED", crCABMemberUserID2: "APPROVED"})
	assertApprovers(t, "Customer Approval", f.stage(id, "Customer Approval").approvers, map[string]string{crScopeUserA1: "REQUESTED", crScopeUserA2: "REQUESTED"})

	// 4. A contact approves -> Scheduled.
	if err := f.decide(id, crScopeUserA2, "approved"); err != nil {
		t.Fatalf("customer approval: %v", err)
	}
	f.expect(id, "after the customer approved", "SCHEDULED", "implement", "canceled")
	assertApprovers(t, "Customer Approval", f.stage(id, "Customer Approval").approvers, map[string]string{crScopeUserA1: "CANCELLED", crScopeUserA2: "APPROVED"})

	// 5. Implement -> Review: the team's internal members again, fresh.
	f.step(id, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
	f.step(id, domain.ChangeRequestStateReview, "REVIEW", "customer_review", "rollback", "canceled")
	labels("Peer Approval", "CAB Approval", "Customer Approval", "Review")
	assertApprovers(t, "Review", f.stage(id, "Review").approvers, map[string]string{
		crFlowCreatorID: "CANCELLED", crFlowPeerAID: "REQUESTED", crFlowPeerBID: "REQUESTED", crFlowOutsiderID: "REQUESTED",
	})
	if err := f.decide(id, crFlowPeerBID, "approved"); err != nil {
		t.Fatalf("review approval: %v", err)
	}
	f.expect(id, "after the internal review", "REVIEW", "customer_review", "rollback", "canceled")
	assertApprovers(t, "Review", f.stage(id, "Review").approvers, map[string]string{
		crFlowCreatorID: "CANCELLED", crFlowPeerAID: "CANCELLED", crFlowPeerBID: "APPROVED", crFlowOutsiderID: "CANCELLED",
	})

	// 6. Customer Review: exactly the project's contacts again; a contact closes it.
	f.step(id, domain.ChangeRequestStateCustomerReview, "CUSTOMER_REVIEW", "canceled")
	labels("Peer Approval", "CAB Approval", "Customer Approval", "Review", "Customer Review")
	assertApprovers(t, "Customer Review", f.stage(id, "Customer Review").approvers, map[string]string{crScopeUserA1: "REQUESTED", crScopeUserA2: "REQUESTED"})
	if err := f.decide(id, crScopeUserA1, "approved"); err != nil {
		t.Fatalf("customer review: %v", err)
	}
	f.expect(id, "after the customer reviewed", "CLOSED")
	assertApprovers(t, "Customer Review", f.stage(id, "Customer Review").approvers, map[string]string{crScopeUserA1: "APPROVED", crScopeUserA2: "CANCELLED"})
}

// A Normal change cannot be sent for approval into a flow with nobody to give
// the CAB approval; the request is refused and nothing is written.
func TestChangeRequestFlowIntegration_NormalRequiresEligibleCABApprovers(t *testing.T) {
	f := newCRFlow(t)
	f.seedAssignedGroup()
	id := f.create(domain.ChangeRequestTypeNormal, crFlowGroupID)

	_, err := f.patchState(id, domain.ChangeRequestStateAssess)
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) || !strings.Contains(ve.Msg, domain.CABApprovalGroupName) {
		t.Fatalf("Request Approval with an empty CAB group err = %v (%T), want a ValidationError naming %q", err, err, domain.CABApprovalGroupName)
	}
	if got := f.state(id); got != "NEW" {
		t.Fatalf("state after refused Request Approval = %q, want NEW", got)
	}
	if n := len(f.stages(id)); n != 0 {
		t.Fatalf("refused Request Approval left %d approval stages", n)
	}

	// Same for Emergency and the ECAB group.
	eid := f.create(domain.ChangeRequestTypeEmergency, crFlowGroupID)
	_, err = f.patchState(eid, domain.ChangeRequestStateAssess)
	if !errors.As(err, &ve) || !strings.Contains(ve.Msg, domain.ECABApprovalGroupName) {
		t.Fatalf("emergency Request Approval with an empty ECAB group err = %v (%T), want a ValidationError naming %q", err, err, domain.ECABApprovalGroupName)
	}

	// A CAB group whose only member is the creator is also a dead end.
	seedApprovalGroupMembers(t, f.scoped, crCABGroupID, crFlowCreatorID)
	if _, err = f.patchState(id, domain.ChangeRequestStateAssess); err == nil {
		t.Fatal("Request Approval succeeded although the only CAB member is the creator")
	}
}

// CAB approval must not be able to strand a change: if the CAB group has lost
// its members by the time the peer approves, the peer's decision is refused
// (rolled back) instead of advancing to an Authorize nobody can decide.
func TestChangeRequestFlowIntegration_PeerApprovalRefusedWhenCABGroupEmptied(t *testing.T) {
	f := newCRFlow(t)
	f.seedAssignedGroup()
	seedApprovalGroupMembers(t, f.scoped, crCABGroupID, crCABMemberUserID1)
	id := f.create(domain.ChangeRequestTypeNormal, crFlowGroupID)
	f.requestApproval(id)
	if _, err := f.scoped.Exec(f.sys, `DELETE FROM team_member WHERE user_id = $1`, crCABMemberUserID1); err != nil {
		t.Fatalf("empty the CAB group: %v", err)
	}

	err := f.decide(id, crFlowPeerAID, "approved")
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("peer approval into an empty CAB group err = %v (%T), want *apierror.ValidationError", err, err)
	}
	if got := f.state(id); got != "ASSESS" {
		t.Fatalf("state after refused peer approval = %q, want ASSESS (rolled back)", got)
	}
	stages := f.stages(id)
	if len(stages) != 1 || stages[0].approvers[crFlowPeerAID] != "REQUESTED" {
		t.Fatalf("stages after rolled-back approval = %+v, want the peer still pending", stages)
	}
}

// A CAB/ECAB rejection keeps the existing rejection behaviour: siblings are
// cancelled and the state is NOT advanced (nor rolled back).
func TestChangeRequestFlowIntegration_CABRejectionDoesNotSchedule(t *testing.T) {
	f := newCRFlow(t)
	f.seedAssignedGroup()
	seedApprovalGroupMembers(t, f.scoped, crCABGroupID, crCABMemberUserID1, crCABMemberUserID2)
	id := f.create(domain.ChangeRequestTypeNormal, crFlowGroupID)
	f.requestApproval(id)
	if err := f.decide(id, crFlowPeerAID, "approved"); err != nil {
		t.Fatalf("peer approval: %v", err)
	}
	if err := f.decide(id, crCABMemberUserID1, "rejected"); err != nil {
		t.Fatalf("CAB rejection: %v", err)
	}
	if got := f.state(id); got != "AUTHORIZE" {
		t.Fatalf("state after CAB rejection = %q, want AUTHORIZE (no cascade)", got)
	}
	stages := f.stages(id)
	assertApprovers(t, "CAB stage after rejection", stages[1].approvers, map[string]string{crCABMemberUserID1: "REJECTED", crCABMemberUserID2: "CANCELLED"})
}

// The type cannot be changed once approval has been requested: the stages
// provisioned belong to the type they were provisioned for.
func TestChangeRequestFlowIntegration_TypeLockedAfterApprovalRequested(t *testing.T) {
	f := newCRFlow(t)
	f.seedAssignedGroup()
	seedApprovalGroupMembers(t, f.scoped, crCABGroupID, crCABMemberUserID1)
	id := f.create(domain.ChangeRequestTypeNormal, crFlowGroupID)

	// Before approval is requested the type may still be corrected.
	std := domain.ChangeRequestTypeStandard
	if _, err := f.repo.PatchChangeRequest(f.sys, id, domain.PatchChangeRequestRequest{Type: &std}, "x@example.com"); err != nil {
		t.Fatalf("changing the type on a New change: %v", err)
	}
	nrm := domain.ChangeRequestTypeNormal
	if _, err := f.repo.PatchChangeRequest(f.sys, id, domain.PatchChangeRequestRequest{Type: &nrm}, "x@example.com"); err != nil {
		t.Fatalf("changing the type back: %v", err)
	}
	f.requestApproval(id)
	emg := domain.ChangeRequestTypeEmergency
	_, err := f.repo.PatchChangeRequest(f.sys, id, domain.PatchChangeRequestRequest{Type: &emg}, "x@example.com")
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("changing the type after Request Approval err = %v (%T), want *apierror.ValidationError", err, err)
	}
}

// canDecide is true only on the viewer's own pending row, and only when they
// may actually decide it.
func TestChangeRequestFlowIntegration_CanDecide(t *testing.T) {
	f := newCRFlow(t)
	f.seedAssignedGroup()
	seedExternalGroupMembers(t, f.scoped, crFlowGroupID, crFlowExternalID)
	seedApprovalGroupMembers(t, f.scoped, crCABGroupID, crCABMemberUserID1)
	id := f.create(domain.ChangeRequestTypeNormal, crFlowGroupID)
	f.requestApproval(id)
	// Drift: rows for the creator and for a customer exist as requested (the
	// customer was never provisioned, so this is a row that predates the
	// INTERNAL-only rule).
	for _, uid := range []string{crFlowCreatorID, crFlowExternalID} {
		if _, err := f.scoped.Exec(f.sys,
			`DELETE FROM approval_stage_approver WHERE work_item_id = $1 AND approver_user_id = $2`, id, uid); err != nil {
			t.Fatalf("reset row: %v", err)
		}
		if _, err := f.scoped.Exec(f.sys,
			`INSERT INTO approval_stage_approver (id, created_on, updated_on, created_by, updated_by, stage_id, work_item_id, approver_user_id, state)
			 SELECT gen_random_uuid(), now(), now(), 'cr-flow-test', 'cr-flow-test', s.id, s.work_item_id, $2::uuid, 'REQUESTED'
			 FROM approval_stage s WHERE s.work_item_id = $1`, id, uid); err != nil {
			t.Fatalf("seed row: %v", err)
		}
	}

	canDecide := func(viewerID string) map[string]bool {
		t.Helper()
		ctx := repository.WithCallerIdentity(context.Background(), repository.SearchScope{Unrestricted: true, ViewerEmail: crFlowEmail(viewerID)})
		got, err := f.repo.GetChangeRequestApprovals(ctx, id)
		if err != nil {
			t.Fatalf("GetChangeRequestApprovals: %v", err)
		}
		out := map[string]bool{}
		for _, a := range got.Approvals {
			for _, ap := range a.Approvers {
				if ap.CanDecide {
					out[ap.ID] = true
				}
			}
		}
		return out
	}

	if got := canDecide(crFlowPeerAID); len(got) != 1 || !got[crFlowPeerAID] {
		t.Fatalf("canDecide for a peer = %v, want only their own row", got)
	}
	if got := canDecide(crFlowCreatorID); len(got) != 0 {
		t.Fatalf("canDecide for the creator = %v, want none", got)
	}
	if got := canDecide(crFlowExternalID); len(got) != 0 {
		t.Fatalf("canDecide for an external user holding a peer row = %v, want none", got)
	}
	if got := canDecide(crCABMemberUserID1); len(got) != 0 {
		t.Fatalf("canDecide for a CAB member while still in Assess = %v, want none (no CAB row yet)", got)
	}
	// System identity carries no viewer: never true.
	sysGot, err := f.repo.GetChangeRequestApprovals(f.sys, id)
	if err != nil {
		t.Fatalf("GetChangeRequestApprovals(system): %v", err)
	}
	for _, a := range sysGot.Approvals {
		for _, ap := range a.Approvers {
			if ap.CanDecide {
				t.Fatalf("canDecide set without a viewer identity: %+v", ap)
			}
		}
	}

	if err := f.decide(id, crFlowPeerAID, "approved"); err != nil {
		t.Fatalf("peer approval: %v", err)
	}
	if got := canDecide(crFlowPeerAID); len(got) != 0 {
		t.Fatalf("canDecide for the peer after deciding = %v, want none", got)
	}
	if got := canDecide(crCABMemberUserID1); len(got) != 1 || !got[crCABMemberUserID1] {
		t.Fatalf("canDecide for a CAB member on the CAB stage = %v, want their own row", got)
	}
}

// The CAB Approval and ECAB Approval groups exist after the migrations, and
// re-running the migration neither fails nor duplicates them.
func TestChangeRequestFlowIntegration_ApprovalGroupsExistAndMigrationIsIdempotent(t *testing.T) {
	f := newCRFlow(t)
	for _, name := range []string{domain.CABApprovalGroupName, domain.ECABApprovalGroupName} {
		var n int
		if err := f.scoped.QueryRow(f.sys, `SELECT COUNT(*) FROM "group" WHERE name = $1`, name).Scan(&n); err != nil {
			t.Fatalf("count %q: %v", name, err)
		}
		if n < 1 {
			t.Fatalf("group %q does not exist after migrations", name)
		}
	}
	var cabID, ecabID string
	if err := f.scoped.QueryRow(f.sys, `SELECT id::text FROM "group" WHERE name = $1 ORDER BY created_on LIMIT 1`, domain.CABApprovalGroupName).Scan(&cabID); err != nil {
		t.Fatalf("read CAB group: %v", err)
	}
	if err := f.scoped.QueryRow(f.sys, `SELECT id::text FROM "group" WHERE name = $1 ORDER BY created_on LIMIT 1`, domain.ECABApprovalGroupName).Scan(&ecabID); err != nil {
		t.Fatalf("read ECAB group: %v", err)
	}
	if cabID == ecabID {
		t.Fatal("CAB Approval and ECAB Approval are the same group; they must be separate groups")
	}

	sqlBytes, err := os.ReadFile("../../migrations/0188_change_request_approval_groups.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	var before, after []string
	collect := func(into *[]string) {
		rows, err := f.scoped.Query(f.sys, `SELECT id::text FROM "group" WHERE name IN ($1, $2) ORDER BY id`, domain.CABApprovalGroupName, domain.ECABApprovalGroupName)
		if err != nil {
			t.Fatalf("list groups: %v", err)
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				t.Fatalf("scan: %v", err)
			}
			*into = append(*into, id)
		}
	}
	collect(&before)
	for i := 0; i < 2; i++ {
		if _, err := f.pool.Exec(context.Background(), string(sqlBytes)); err != nil {
			t.Fatalf("re-running migration 0188 (pass %d): %v", i+1, err)
		}
	}
	collect(&after)
	sort.Strings(before)
	sort.Strings(after)
	if strings.Join(before, ",") != strings.Join(after, ",") {
		t.Fatalf("re-running the migration changed the groups: before %v, after %v", before, after)
	}
}

// A type is mandatory on create, and only standard/normal/emergency are
// allowed -- in both Postgres create paths (portal and ServiceNow-first).
func TestChangeRequestFlowIntegration_CreateRequiresCreatableType(t *testing.T) {
	f := newCRFlow(t)
	var ve *apierror.ValidationError

	if _, err := f.repo.CreateChangeRequest(f.sys, domain.CreateChangeRequestRequest{Subject: crFlowSubject}, "x@example.com"); !errors.As(err, &ve) || !strings.Contains(ve.Msg, "type is required") {
		t.Fatalf("CreateChangeRequest without a type err = %v (%T), want a \"type is required\" ValidationError", err, err)
	}
	if _, err := f.repo.CreateChangeRequestFromServiceNow(f.sys, domain.CreateChangeRequestRequest{Subject: crFlowSubject}, "3aaaaaaa-0000-0000-0000-0000000000f1", "CRFLOWSN001", "x@example.com"); !errors.As(err, &ve) || !strings.Contains(ve.Msg, "type is required") {
		t.Fatalf("CreateChangeRequestFromServiceNow without a type err = %v (%T), want a \"type is required\" ValidationError", err, err)
	}
	azure := domain.ChangeRequestTypeAzure
	if _, err := f.repo.CreateChangeRequest(f.sys, domain.CreateChangeRequestRequest{Subject: crFlowSubject, Type: &azure}, "x@example.com"); !errors.As(err, &ve) {
		t.Fatalf("CreateChangeRequest with type azure err = %v (%T), want a ValidationError", err, err)
	}

	for _, typ := range domain.ChangeRequestCreatableTypes {
		typ := typ
		resp, err := f.repo.CreateChangeRequest(f.sys, domain.CreateChangeRequestRequest{Subject: crFlowSubject, Type: &typ}, "x@example.com")
		if err != nil {
			t.Fatalf("CreateChangeRequest(%s): %v", typ, err)
		}
		cr, err := f.repo.GetChangeRequestByID(f.sys, resp.ChangeRequest.ID)
		if err != nil {
			t.Fatalf("GetChangeRequestByID: %v", err)
		}
		if cr.Type == nil || !strings.EqualFold(*cr.Type, string(typ)) {
			t.Fatalf("created type = %v, want %q", cr.Type, typ)
		}
	}
}

// ---------------------------------------------------------------------------
// Customer Approval / Customer Review checkboxes
// (change_request.customer_approval_required / customer_review_required,
// migration 0189). They add an optional customer step on each side of the
// implementation: CAB / ECAB approval (or Request Approval on a Standard
// change) moves the change to Customer Approval instead of Scheduled, where a
// human records the customer's approval; Review offers Customer Review (then
// Closed) instead of Closed.
// ---------------------------------------------------------------------------

func boolp(b bool) *bool { return &b }

// createGated is crFlow.create with the creation form's two checkboxes.
func (f *crFlow) createGated(typ domain.ChangeRequestType, groupID string, approval, review *bool) string {
	f.t.Helper()
	g := groupID
	resp, err := f.repo.CreateChangeRequest(f.sys, domain.CreateChangeRequestRequest{
		Subject: crFlowSubject, Type: &typ, GroupID: &g,
		CustomerApprovalRequired: approval, CustomerReviewRequired: review,
	}, crFlowEmail(crFlowCreatorID))
	if err != nil {
		f.t.Fatalf("CreateChangeRequest(%s): %v", typ, err)
	}
	if _, err := f.scoped.Exec(f.sys, `UPDATE change_request SET requested_by_user_id = $1::uuid WHERE id = $2`, crFlowCreatorID, resp.ChangeRequest.ID); err != nil {
		f.t.Fatalf("set requested_by: %v", err)
	}
	return resp.ChangeRequest.ID
}

func (f *crFlow) patch(id string, req domain.PatchChangeRequestRequest) (domain.ChangeRequest, error) {
	return f.repo.PatchChangeRequest(f.sys, id, req, crFlowEmail(crFlowCreatorID))
}

func (f *crFlow) get(id string) domain.ChangeRequest {
	f.t.Helper()
	cr, err := f.repo.GetChangeRequestByID(f.sys, id)
	if err != nil {
		f.t.Fatalf("GetChangeRequestByID: %v", err)
	}
	return cr
}

// step PATCHes a state and asserts the resulting stored state and legalNextStates.
func (f *crFlow) step(id string, to domain.ChangeRequestState, wantState string, wantLegal ...string) {
	f.t.Helper()
	if _, err := f.patchState(id, to); err != nil {
		f.t.Fatalf("PATCH {state: %s}: %v", to, err)
	}
	f.expect(id, "after PATCH "+string(to), wantState, wantLegal...)
}

// expect asserts the stored state and legalNextStates.
func (f *crFlow) expect(id, when, wantState string, wantLegal ...string) {
	f.t.Helper()
	if got := f.state(id); got != wantState {
		f.t.Fatalf("state %s = %q, want %q", when, got, wantState)
	}
	assertStates(f.t, "legalNextStates "+when, f.legal(id), wantLegal...)
}

func (f *crFlow) customerOutcome(id string) (approved, reviewed bool) {
	f.t.Helper()
	cr := f.get(id)
	return cr.HasCustomerApproved, cr.HasCustomerReviewed
}

func (f *crFlow) wantValidationError(what string, err error, contains string) {
	f.t.Helper()
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		f.t.Fatalf("%s: err = %v (%T), want *apierror.ValidationError", what, err, err)
	}
	if !strings.Contains(ve.Msg, contains) {
		f.t.Fatalf("%s: message %q should contain %q", what, ve.Msg, contains)
	}
}

// approvePeerAndCAB drives a Normal change from Assess through peer approval
// and CAB approval, asserting the state after each step; wantAfterCAB is where
// CAB approval must leave the change.
func (f *crFlow) approvePeerAndCAB(id, wantAfterCAB string, wantLegalAfterCAB ...string) {
	f.t.Helper()
	if err := f.decide(id, crFlowPeerAID, "approved"); err != nil {
		f.t.Fatalf("peer approval: %v", err)
	}
	f.expect(id, "after peer approval", "AUTHORIZE", "canceled")
	if err := f.decide(id, crCABMemberUserID1, "approved"); err != nil {
		f.t.Fatalf("CAB approval: %v", err)
	}
	f.expect(id, "after CAB approval", wantAfterCAB, wantLegalAfterCAB...)
}

// Normal x {Customer Approval ticked, unticked} x {Customer Review ticked,
// unticked}: the state, legalNextStates and the recorded customer outcome after
// EVERY step.
func TestChangeRequestFlowIntegration_NormalCustomerGateLifecycles(t *testing.T) {
	for _, tc := range []struct {
		name             string
		approval, review bool
	}{
		{"approval-unticked/review-unticked", false, false},
		{"approval-ticked/review-unticked", true, false},
		{"approval-unticked/review-ticked", false, true},
		{"approval-ticked/review-ticked", true, true},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			f := newCRFlow(t)
			f.seedAssignedGroup()
			seedApprovalGroupMembers(t, f.scoped, crCABGroupID, crCABMemberUserID1, crCABMemberUserID2)
			id := f.createGated(domain.ChangeRequestTypeNormal, crFlowGroupID, boolp(tc.approval), boolp(tc.review))

			// The checkboxes are stored and read back as ticked.
			cr := f.get(id)
			if cr.CustomerApprovalRequired != tc.approval || cr.CustomerReviewRequired != tc.review {
				t.Fatalf("created flags read back = %v/%v, want %v/%v", cr.CustomerApprovalRequired, cr.CustomerReviewRequired, tc.approval, tc.review)
			}
			f.expect(id, "after create", "NEW", "assess", "canceled")

			f.requestApproval(id)
			f.expect(id, "after Request Approval", "ASSESS", "authorize", "canceled")

			// Both gates sit AFTER the internal approvals: nothing short-circuits them.
			f.approvePeerAndCAB(id, map[bool]string{true: "CUSTOMER_APPROVAL", false: "SCHEDULED"}[tc.approval],
				map[bool][]string{true: {"scheduled", "authorize", "canceled"}, false: {"implement", "canceled"}}[tc.approval]...)
			if approved, _ := f.customerOutcome(id); approved {
				t.Fatal("is_customer_approval_required is already true before the customer approved anything")
			}

			if tc.approval {
				// The customer's approval is recorded by the human action
				// "scheduled", which is legal only here.
				f.step(id, domain.ChangeRequestStateScheduled, "SCHEDULED", "implement", "canceled")
				if approved, _ := f.customerOutcome(id); !approved {
					t.Fatal("is_customer_approval_required = false after recording the customer's approval, want true")
				}
			} else if approved, _ := f.customerOutcome(id); approved {
				t.Fatal("is_customer_approval_required = true although no customer approval was required or given")
			}

			f.step(id, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
			if tc.review {
				f.step(id, domain.ChangeRequestStateReview, "REVIEW", "customer_review", "rollback", "canceled")
				// Review cannot close directly when the customer's review is required.
				_, err := f.patchState(id, domain.ChangeRequestStateClosed)
				f.wantValidationError("closed from review (customer review required)", err, "customer review is required")
				f.expect(id, "after refused close", "REVIEW", "customer_review", "rollback", "canceled")
				f.step(id, domain.ChangeRequestStateCustomerReview, "CUSTOMER_REVIEW", "closed", "rollback", "canceled")
				if _, reviewed := f.customerOutcome(id); reviewed {
					t.Fatal("is_customer_review_required is already true before the customer review was recorded")
				}
				f.step(id, domain.ChangeRequestStateClosed, "CLOSED")
				if _, reviewed := f.customerOutcome(id); !reviewed {
					t.Fatal("is_customer_review_required = false after closing from customer_review, want true")
				}
			} else {
				f.step(id, domain.ChangeRequestStateReview, "REVIEW", "closed", "rollback", "canceled")
				// Customer Review is not offered, and not accepted, when not required.
				_, err := f.patchState(id, domain.ChangeRequestStateCustomerReview)
				f.wantValidationError("customer_review (not required)", err, "customer review is not required")
				f.expect(id, "after refused customer_review", "REVIEW", "closed", "rollback", "canceled")
				f.step(id, domain.ChangeRequestStateClosed, "CLOSED")
				if _, reviewed := f.customerOutcome(id); reviewed {
					t.Fatal("is_customer_review_required = true although no customer review was required or given")
				}
			}
			if approved, _ := f.customerOutcome(id); approved != tc.approval {
				t.Fatalf("is_customer_approval_required at the end = %v, want %v", approved, tc.approval)
			}
			// The two internal approval stages (peer, CAB) are untouched by the
			// customer gates: Customer Approval is a state, not an approval stage.
			if n := len(f.stages(id)); n != 3 {
				t.Fatalf("stages at the end = %d, want 3 (peer, CAB, review)", n)
			}
		})
	}
}

// Emergency with Customer Approval ticked: ECAB approval moves the change to
// Customer Approval (not Scheduled); recording the customer's approval then
// schedules it. Unticked is covered by EmergencyLifecycle.
func TestChangeRequestFlowIntegration_EmergencyCustomerApprovalLifecycle(t *testing.T) {
	f := newCRFlow(t)
	f.seedAssignedGroup()
	seedApprovalGroupMembers(t, f.scoped, crECABGroupID, crECABMemberUserID)
	id := f.createGated(domain.ChangeRequestTypeEmergency, crFlowGroupID, boolp(true), nil)

	f.expect(id, "after create", "NEW", "assess", "canceled")
	f.requestApproval(id)
	f.expect(id, "after Request Approval", "AUTHORIZE", "canceled")
	if stages := f.stages(id); len(stages) != 1 || stages[0].label != "ECAB Approval" {
		t.Fatalf("stages = %+v, want only the ECAB stage", stages)
	}

	// The gate is NOT reached by Request Approval for an Emergency change: the
	// ECAB approval still comes first.
	if err := f.decide(id, crECABMemberUserID, "approved"); err != nil {
		t.Fatalf("ECAB approval: %v", err)
	}
	f.expect(id, "after ECAB approval", "CUSTOMER_APPROVAL", "scheduled", "authorize", "canceled")
	if approved, _ := f.customerOutcome(id); approved {
		t.Fatal("is_customer_approval_required is true before the customer's approval was recorded")
	}

	f.step(id, domain.ChangeRequestStateScheduled, "SCHEDULED", "implement", "canceled")
	if approved, _ := f.customerOutcome(id); !approved {
		t.Fatal("is_customer_approval_required = false after recording the customer's approval")
	}
	f.step(id, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
	f.step(id, domain.ChangeRequestStateReview, "REVIEW", "closed", "rollback", "canceled") // review not ticked: straight to Closed
	f.step(id, domain.ChangeRequestStateClosed, "CLOSED")
}

// Standard with Customer Approval ticked: it has no internal approval to wait
// for, so Request Approval itself lands in Customer Approval (the assumption
// recorded in the PR), with no approval stage at all.
func TestChangeRequestFlowIntegration_StandardCustomerApprovalLifecycle(t *testing.T) {
	f := newCRFlow(t)
	f.seedAssignedGroup()
	id := f.createGated(domain.ChangeRequestTypeStandard, crFlowGroupID, boolp(true), boolp(true))

	f.expect(id, "after create", "NEW", "assess", "canceled")
	f.requestApproval(id)
	f.expect(id, "after Request Approval", "CUSTOMER_APPROVAL", "scheduled", "authorize", "canceled")
	if n := len(f.stages(id)); n != 0 {
		t.Fatalf("standard change has %d approval stages, want none", n)
	}
	// A resend of Request Approval while it waits for the customer is idempotent.
	if _, err := f.patchState(id, domain.ChangeRequestStateAssess); err != nil {
		t.Fatalf("resent Request Approval: %v", err)
	}
	f.expect(id, "after resent Request Approval", "CUSTOMER_APPROVAL", "scheduled", "authorize", "canceled")

	f.step(id, domain.ChangeRequestStateScheduled, "SCHEDULED", "implement", "canceled")
	if approved, _ := f.customerOutcome(id); !approved {
		t.Fatal("is_customer_approval_required = false after recording the customer's approval")
	}
	f.step(id, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
	f.step(id, domain.ChangeRequestStateReview, "REVIEW", "customer_review", "rollback", "canceled")
	f.step(id, domain.ChangeRequestStateCustomerReview, "CUSTOMER_REVIEW", "closed", "rollback", "canceled")
	f.step(id, domain.ChangeRequestStateClosed, "CLOSED")
	if _, reviewed := f.customerOutcome(id); !reviewed {
		t.Fatal("is_customer_review_required = false after closing from customer_review")
	}
}

// Ticking Customer Approval together with Request Approval (one PATCH) on a
// Standard change routes it to Customer Approval; so does ticking it in a
// separate PATCH before Request Approval.
func TestChangeRequestFlowIntegration_StandardCustomerApprovalTickedWithRequestApproval(t *testing.T) {
	f := newCRFlow(t)
	f.seedAssignedGroup()

	together := f.create(domain.ChangeRequestTypeStandard, crFlowGroupID)
	st := domain.ChangeRequestStateAssess
	if _, err := f.patch(together, domain.PatchChangeRequestRequest{State: &st, CustomerApprovalRequired: boolp(true)}); err != nil {
		t.Fatalf("PATCH {state: assess, customerApprovalRequired: true}: %v", err)
	}
	f.expect(together, "after the combined PATCH", "CUSTOMER_APPROVAL", "scheduled", "authorize", "canceled")

	separate := f.create(domain.ChangeRequestTypeStandard, crFlowGroupID)
	if _, err := f.patch(separate, domain.PatchChangeRequestRequest{CustomerApprovalRequired: boolp(true)}); err != nil {
		t.Fatalf("PATCH {customerApprovalRequired: true}: %v", err)
	}
	f.expect(separate, "after ticking the box", "NEW", "assess", "canceled")
	f.requestApproval(separate)
	f.expect(separate, "after Request Approval", "CUSTOMER_APPROVAL", "scheduled", "authorize", "canceled")
}

// The checkboxes are accepted on create (both create paths) and PATCH, and
// returned on the detail response.
func TestChangeRequestFlowIntegration_CustomerGateFlagsAcceptedAndReturned(t *testing.T) {
	f := newCRFlow(t)
	f.seedAssignedGroup()

	// Omitted on create: both false.
	plain := f.create(domain.ChangeRequestTypeNormal, crFlowGroupID)
	if cr := f.get(plain); cr.CustomerApprovalRequired || cr.CustomerReviewRequired {
		t.Fatalf("flags after a create that omitted them = %v/%v, want false/false", cr.CustomerApprovalRequired, cr.CustomerReviewRequired)
	}

	// Ticked on the portal create path.
	both := f.createGated(domain.ChangeRequestTypeNormal, crFlowGroupID, boolp(true), boolp(true))
	if cr := f.get(both); !cr.CustomerApprovalRequired || !cr.CustomerReviewRequired {
		t.Fatalf("flags after a create that ticked both = %v/%v, want true/true", cr.CustomerApprovalRequired, cr.CustomerReviewRequired)
	}

	// Ticked on the ServiceNow-first create path.
	typ := domain.ChangeRequestTypeNormal
	resp, err := f.repo.CreateChangeRequestFromServiceNow(f.sys, domain.CreateChangeRequestRequest{
		Subject: crFlowSubject, Type: &typ, CustomerApprovalRequired: boolp(true), CustomerReviewRequired: boolp(false),
	}, "3aaaaaaa-0000-0000-0000-0000000000f2", "CRFLOWSN002", "x@example.com")
	if err != nil {
		t.Fatalf("CreateChangeRequestFromServiceNow: %v", err)
	}
	if cr := f.get(resp.ChangeRequest.ID); !cr.CustomerApprovalRequired || cr.CustomerReviewRequired {
		t.Fatalf("flags after a ServiceNow-first create = %v/%v, want true/false", cr.CustomerApprovalRequired, cr.CustomerReviewRequired)
	}

	// PATCH sets them, returns them in the receipt, and a PATCH touching only
	// one leaves the other alone.
	cr, err := f.patch(plain, domain.PatchChangeRequestRequest{CustomerApprovalRequired: boolp(true)})
	if err != nil {
		t.Fatalf("PATCH customerApprovalRequired: %v", err)
	}
	if !cr.CustomerApprovalRequired || cr.CustomerReviewRequired {
		t.Fatalf("receipt flags = %v/%v, want true/false", cr.CustomerApprovalRequired, cr.CustomerReviewRequired)
	}
	cr, err = f.patch(plain, domain.PatchChangeRequestRequest{CustomerReviewRequired: boolp(true)})
	if err != nil {
		t.Fatalf("PATCH customerReviewRequired: %v", err)
	}
	if !cr.CustomerApprovalRequired || !cr.CustomerReviewRequired {
		t.Fatalf("receipt flags = %v/%v, want true/true", cr.CustomerApprovalRequired, cr.CustomerReviewRequired)
	}
	cr, err = f.patch(plain, domain.PatchChangeRequestRequest{CustomerApprovalRequired: boolp(false), CustomerReviewRequired: boolp(false)})
	if err != nil {
		t.Fatalf("PATCH both off: %v", err)
	}
	if cr.CustomerApprovalRequired || cr.CustomerReviewRequired {
		t.Fatalf("receipt flags = %v/%v, want false/false", cr.CustomerApprovalRequired, cr.CustomerReviewRequired)
	}
	if got := f.get(plain); got.CustomerApprovalRequired || got.CustomerReviewRequired {
		t.Fatalf("stored flags = %v/%v, want false/false", got.CustomerApprovalRequired, got.CustomerReviewRequired)
	}
}

// customerApprovalRequired is editable while the change is New, Assess or
// Authorize -- and what it is at CAB approval decides the cascade. After that
// the gate has been passed and an edit is refused with a clear 400.
func TestChangeRequestFlowIntegration_CustomerApprovalRequiredEditableUntilGatePassed(t *testing.T) {
	f := newCRFlow(t)
	f.seedAssignedGroup()
	seedApprovalGroupMembers(t, f.scoped, crCABGroupID, crCABMemberUserID1)
	id := f.create(domain.ChangeRequestTypeNormal, crFlowGroupID)

	set := func(v bool) error {
		_, err := f.patch(id, domain.PatchChangeRequestRequest{CustomerApprovalRequired: boolp(v)})
		return err
	}
	if err := set(true); err != nil {
		t.Fatalf("tick in New: %v", err)
	}
	f.requestApproval(id)
	if err := set(false); err != nil {
		t.Fatalf("untick in Assess: %v", err)
	}
	if err := set(true); err != nil {
		t.Fatalf("tick in Assess: %v", err)
	}
	if err := f.decide(id, crFlowPeerAID, "approved"); err != nil {
		t.Fatalf("peer approval: %v", err)
	}
	f.expect(id, "after peer approval", "AUTHORIZE", "canceled")
	if err := set(false); err != nil {
		t.Fatalf("untick in Authorize: %v", err)
	}
	if err := set(true); err != nil {
		t.Fatalf("tick in Authorize: %v", err)
	}

	// The value at CAB approval decides: ticked -> Customer Approval.
	if err := f.decide(id, crCABMemberUserID1, "approved"); err != nil {
		t.Fatalf("CAB approval: %v", err)
	}
	f.expect(id, "after CAB approval", "CUSTOMER_APPROVAL", "scheduled", "authorize", "canceled")

	// The gate has been passed: refused, state and flag unchanged.
	f.wantValidationError("untick in Customer Approval", set(false), "customerApprovalRequired can no longer be changed")
	if !f.get(id).CustomerApprovalRequired {
		t.Fatal("customerApprovalRequired changed although the edit was refused")
	}
	f.step(id, domain.ChangeRequestStateScheduled, "SCHEDULED", "implement", "canceled")
	f.wantValidationError("untick in Scheduled", set(false), "customerApprovalRequired can no longer be changed")
	// Resending the stored value is not an edit.
	if err := set(true); err != nil {
		t.Fatalf("resending the stored value after the gate: %v", err)
	}
	// ... and the refusal is atomic: nothing else in the same PATCH was written.
	title := "should not be written"
	_, err := f.patch(id, domain.PatchChangeRequestRequest{Title: &title, CustomerApprovalRequired: boolp(false)})
	f.wantValidationError("title + untick in Scheduled", err, "can no longer be changed")
	var subject string
	if err := f.scoped.QueryRow(f.sys, `SELECT subject FROM work_item WHERE id = $1`, id).Scan(&subject); err != nil {
		t.Fatalf("read subject: %v", err)
	}
	if subject != crFlowSubject {
		t.Fatalf("subject = %q after a refused PATCH, want it untouched (%q)", subject, crFlowSubject)
	}

	// Unticked at CAB approval: straight to Scheduled.
	seedApprovalGroupMembers(t, f.scoped, crCABGroupID, crCABMemberUserID1)
	id2 := f.create(domain.ChangeRequestTypeNormal, crFlowGroupID)
	if err := func() error {
		_, err := f.patch(id2, domain.PatchChangeRequestRequest{CustomerApprovalRequired: boolp(true)})
		return err
	}(); err != nil {
		t.Fatalf("tick: %v", err)
	}
	if _, err := f.patch(id2, domain.PatchChangeRequestRequest{CustomerApprovalRequired: boolp(false)}); err != nil {
		t.Fatalf("untick: %v", err)
	}
	f.requestApproval(id2)
	f.approvePeerAndCAB(id2, "SCHEDULED", "implement", "canceled")
}

// customerReviewRequired is editable until the change leaves Review -- it
// decides what Review offers -- and refused afterwards.
func TestChangeRequestFlowIntegration_CustomerReviewRequiredEditableUntilReviewLeft(t *testing.T) {
	f := newCRFlow(t)
	f.seedAssignedGroup()
	id := f.create(domain.ChangeRequestTypeStandard, crFlowGroupID)
	set := func(v bool) error {
		_, err := f.patch(id, domain.PatchChangeRequestRequest{CustomerReviewRequired: boolp(v)})
		return err
	}
	for _, step := range []struct {
		state domain.ChangeRequestState // "" = stay
	}{{""}, {domain.ChangeRequestStateAssess}, {domain.ChangeRequestStateImplement}} {
		if step.state != "" {
			if _, err := f.patchState(id, step.state); err != nil {
				t.Fatalf("PATCH state %s: %v", step.state, err)
			}
		}
		if err := set(true); err != nil {
			t.Fatalf("tick in %s: %v", f.state(id), err)
		}
		if err := set(false); err != nil {
			t.Fatalf("untick in %s: %v", f.state(id), err)
		}
	}

	f.step(id, domain.ChangeRequestStateReview, "REVIEW", "closed", "rollback", "canceled")
	// Still editable in Review, and it flips what Review offers.
	if err := set(true); err != nil {
		t.Fatalf("tick in Review: %v", err)
	}
	f.expect(id, "after ticking in Review", "REVIEW", "customer_review", "rollback", "canceled")
	if err := set(false); err != nil {
		t.Fatalf("untick in Review: %v", err)
	}
	f.expect(id, "after unticking in Review", "REVIEW", "closed", "rollback", "canceled")
	if err := set(true); err != nil {
		t.Fatalf("tick in Review (again): %v", err)
	}

	f.step(id, domain.ChangeRequestStateCustomerReview, "CUSTOMER_REVIEW", "closed", "rollback", "canceled")
	f.wantValidationError("untick in Customer Review", set(false), "customerReviewRequired can no longer be changed")
	if err := set(true); err != nil {
		t.Fatalf("resending the stored value: %v", err)
	}
	f.step(id, domain.ChangeRequestStateClosed, "CLOSED")
	f.wantValidationError("untick in Closed", set(false), "customerReviewRequired can no longer be changed")
	f.wantValidationError("tick approval in Closed", func() error {
		_, err := f.patch(id, domain.PatchChangeRequestRequest{CustomerApprovalRequired: boolp(true)})
		return err
	}(), "customerApprovalRequired can no longer be changed")
	if !f.get(id).CustomerReviewRequired {
		t.Fatal("customerReviewRequired changed although the edit was refused")
	}
}

// Unticking Customer Review in the same PATCH that closes a Review lets the
// close through; closing a required Review without unticking is refused.
func TestChangeRequestFlowIntegration_CloseFromReviewHonoursTheFlagInTheSamePatch(t *testing.T) {
	f := newCRFlow(t)
	f.seedAssignedGroup()
	id := f.createGated(domain.ChangeRequestTypeStandard, crFlowGroupID, nil, boolp(true))
	f.requestApproval(id)
	f.step(id, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
	f.step(id, domain.ChangeRequestStateReview, "REVIEW", "customer_review", "rollback", "canceled")

	closed := domain.ChangeRequestStateClosed
	_, err := f.patch(id, domain.PatchChangeRequestRequest{State: &closed})
	f.wantValidationError("close a review that requires the customer", err, "customer review is required")
	f.expect(id, "after the refused close", "REVIEW", "customer_review", "rollback", "canceled")

	if _, err := f.patch(id, domain.PatchChangeRequestRequest{State: &closed, CustomerReviewRequired: boolp(false)}); err != nil {
		t.Fatalf("PATCH {state: closed, customerReviewRequired: false}: %v", err)
	}
	f.expect(id, "after closing with the box unticked", "CLOSED")
	if _, reviewed := f.customerOutcome(id); reviewed {
		t.Fatal("is_customer_review_required = true although the change closed straight from Review")
	}
}

// Manual {state: scheduled} is legal ONLY from Customer Approval; from every
// other state it is refused and nothing changes.
func TestChangeRequestFlowIntegration_ManualScheduledOnlyFromCustomerApproval(t *testing.T) {
	f := newCRFlow(t)
	f.seedAssignedGroup()
	seedApprovalGroupMembers(t, f.scoped, crCABGroupID, crCABMemberUserID1)
	normal := f.create(domain.ChangeRequestTypeNormal, crFlowGroupID)
	std := f.create(domain.ChangeRequestTypeStandard, crFlowGroupID)

	refused := func(id, from string) {
		t.Helper()
		_, err := f.patchState(id, domain.ChangeRequestStateScheduled)
		f.wantValidationError("manual scheduled from "+from, err, "cannot be set manually")
		if got := f.state(id); got != from {
			t.Fatalf("state after refused scheduled from %s = %q", from, got)
		}
		if approved, _ := f.customerOutcome(id); approved {
			t.Fatalf("a refused scheduled from %s still stamped is_customer_approval_required", from)
		}
	}
	refused(normal, "NEW")
	f.requestApproval(normal)
	refused(normal, "ASSESS")
	if err := f.decide(normal, crFlowPeerAID, "approved"); err != nil {
		t.Fatalf("peer approval: %v", err)
	}
	refused(normal, "AUTHORIZE")
	if err := f.decide(normal, crCABMemberUserID1, "approved"); err != nil {
		t.Fatalf("CAB approval: %v", err)
	}
	f.expect(normal, "after CAB approval (not ticked)", "SCHEDULED", "implement", "canceled")
	// Scheduled itself: a resent scheduled is not "recording the customer's approval".
	refused(normal, "SCHEDULED")
	f.step(normal, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
	refused(normal, "IMPLEMENT")
	f.step(normal, domain.ChangeRequestStateReview, "REVIEW", "closed", "rollback", "canceled")
	refused(normal, "REVIEW")

	// authorize / customer_approval stay unreachable by hand even when required.
	if _, err := f.patch(std, domain.PatchChangeRequestRequest{CustomerApprovalRequired: boolp(true)}); err != nil {
		t.Fatalf("tick: %v", err)
	}
	for _, target := range []domain.ChangeRequestState{domain.ChangeRequestStateCustomerApproval, domain.ChangeRequestStateAuthorize} {
		_, err := f.patchState(std, target)
		f.wantValidationError("manual "+string(target), err, "cannot be set manually")
		f.expect(std, "after refused manual "+string(target), "NEW", "assess", "canceled")
	}
}

// Recording the customer's approval: stamps is_customer_approval_required through the
// same one-way lock as a direct isCustomerApproved write, refuses a
// contradictory isCustomerApproved:false, and is blocked while on hold.
func TestChangeRequestFlowIntegration_CustomerApprovalRecordsTheApproval(t *testing.T) {
	f := newCRFlow(t)
	f.seedAssignedGroup()
	id := f.createGated(domain.ChangeRequestTypeStandard, crFlowGroupID, boolp(true), nil)
	f.requestApproval(id)
	f.expect(id, "at Customer Approval", "CUSTOMER_APPROVAL", "scheduled", "authorize", "canceled")

	// Contradictory flag in the same PATCH.
	sched := domain.ChangeRequestStateScheduled
	_, err := f.patch(id, domain.PatchChangeRequestRequest{State: &sched, IsCustomerApproved: boolp(false)})
	f.wantValidationError("scheduled + isCustomerApproved false", err, "isCustomerApproved cannot be false")
	f.expect(id, "after the contradictory PATCH", "CUSTOMER_APPROVAL", "scheduled", "authorize", "canceled")

	// On hold blocks the state change like any other.
	if _, err := f.patch(id, domain.PatchChangeRequestRequest{OnHold: boolp(true)}); err != nil {
		t.Fatalf("put on hold: %v", err)
	}
	_, err = f.patchState(id, domain.ChangeRequestStateScheduled)
	f.wantValidationError("scheduled while on hold", err, "on hold")
	f.expect(id, "while on hold", "CUSTOMER_APPROVAL", "scheduled", "authorize", "canceled")
	if approved, _ := f.customerOutcome(id); approved {
		t.Fatal("is_customer_approval_required stamped by a PATCH that was refused")
	}
	// Off hold and approve in one call.
	if _, err := f.patch(id, domain.PatchChangeRequestRequest{State: &sched, OnHold: boolp(false)}); err != nil {
		t.Fatalf("take off hold and record the approval: %v", err)
	}
	f.expect(id, "after recording the approval", "SCHEDULED", "implement", "canceled")
	if approved, _ := f.customerOutcome(id); !approved {
		t.Fatal("is_customer_approval_required = false after recording the customer's approval")
	}

	// With the explicit flag alongside (true), also fine; a second record is refused as manual scheduled.
	id2 := f.createGated(domain.ChangeRequestTypeStandard, crFlowGroupID, boolp(true), nil)
	f.requestApproval(id2)
	if _, err := f.patch(id2, domain.PatchChangeRequestRequest{State: &sched, IsCustomerApproved: boolp(true)}); err != nil {
		t.Fatalf("scheduled + isCustomerApproved true: %v", err)
	}
	f.expect(id2, "after scheduled + isCustomerApproved", "SCHEDULED", "implement", "canceled")
}

// The customer declining: Cancel is offered at Customer Approval and works.
func TestChangeRequestFlowIntegration_CustomerApprovalCanBeCancelled(t *testing.T) {
	f := newCRFlow(t)
	f.seedAssignedGroup()
	id := f.createGated(domain.ChangeRequestTypeStandard, crFlowGroupID, boolp(true), nil)
	f.requestApproval(id)
	f.step(id, domain.ChangeRequestStateCanceled, "CANCELED")
	if approved, _ := f.customerOutcome(id); approved {
		t.Fatal("is_customer_approval_required = true on a change the customer declined")
	}
}

// Creator / INTERNAL-only approval rules from the CAB flow still hold when the
// customer gates are ticked.
func TestChangeRequestFlowIntegration_ApproverRulesHoldWithCustomerGates(t *testing.T) {
	f := newCRFlow(t)
	f.seedAssignedGroup()
	seedExternalGroupMembers(t, f.scoped, crFlowGroupID, crFlowExternalID)
	seedApprovalGroupMembers(t, f.scoped, crCABGroupID, crCABMemberUserID1)
	id := f.createGated(domain.ChangeRequestTypeNormal, crFlowGroupID, boolp(true), boolp(true))
	f.requestApproval(id)

	// The creator cannot approve the peer stage; a customer who is a member of
	// the team is not even a peer approver (not provisioned).
	var fe *apierror.ForbiddenError
	if err := f.decide(id, crFlowCreatorID, "approved"); !errors.As(err, &fe) {
		t.Fatalf("creator approving the peer stage err = %v (%T), want ForbiddenError", err, err)
	}
	if stages := f.stages(id); len(stages) != 1 {
		t.Fatalf("stages = %d, want 1", len(stages))
	} else if _, ok := stages[0].approvers[crFlowExternalID]; ok {
		t.Fatal("the customer was provisioned as a peer approver")
	}
	if err := f.decide(id, crFlowPeerAID, "approved"); err != nil {
		t.Fatalf("peer approval: %v", err)
	}
	f.expect(id, "after peer approval", "AUTHORIZE", "canceled")

	// Creator on the CAB stage by force: still refused, and the change does not
	// reach the customer gate.
	if _, err := f.scoped.Exec(f.sys,
		`INSERT INTO approval_stage_approver (id, created_on, updated_on, created_by, updated_by, stage_id, work_item_id, approver_user_id, state)
		 SELECT gen_random_uuid(), now(), now(), 'cr-flow-test', 'cr-flow-test', s.id, s.work_item_id, $2::uuid, 'REQUESTED'
		 FROM approval_stage s WHERE s.work_item_id = $1 AND s.checkpoint_label = 'CAB Approval'`, id, crFlowCreatorID); err != nil {
		t.Fatalf("force creator onto the CAB stage: %v", err)
	}
	if err := f.decide(id, crFlowCreatorID, "approved"); !errors.As(err, &fe) {
		t.Fatalf("creator approving the CAB stage err = %v (%T), want ForbiddenError", err, err)
	}
	f.expect(id, "after the refused creator approval", "AUTHORIZE", "canceled")

	// A CAB rejection does not move the change to either gate.
	if err := f.decide(id, crCABMemberUserID1, "rejected"); err != nil {
		t.Fatalf("CAB rejection: %v", err)
	}
	f.expect(id, "after CAB rejection", "AUTHORIZE", "canceled")

	// The creator may still cancel.
	f.step(id, domain.ChangeRequestStateCanceled, "CANCELED")
}

// Rows that predate the checkboxes (false/false): approval behaves as before
// (no customer step), a Review offers Closed directly, and a row already
// sitting in Customer Approval / Customer Review gets their outgoing moves.
func TestChangeRequestFlowIntegration_LegacyRowsDefaultToNoCustomerSteps(t *testing.T) {
	f := newCRFlow(t)
	f.seedAssignedGroup()
	id := f.create(domain.ChangeRequestTypeNormal, crFlowGroupID)
	cr := f.get(id)
	if cr.CustomerApprovalRequired || cr.CustomerReviewRequired {
		t.Fatalf("defaults = %v/%v, want false/false", cr.CustomerApprovalRequired, cr.CustomerReviewRequired)
	}
	for _, tc := range []struct {
		state string
		legal []string
	}{
		{"REVIEW", []string{"closed", "rollback", "canceled"}},
		{"CUSTOMER_APPROVAL", []string{"scheduled", "authorize", "canceled"}},
		{"CUSTOMER_REVIEW", []string{"closed", "rollback", "canceled"}},
		{"SCHEDULED", []string{"implement", "canceled"}},
	} {
		if _, err := f.scoped.Exec(f.sys, `UPDATE change_request SET state = $2::change_request_state_enum WHERE id = $1`, id, tc.state); err != nil {
			t.Fatalf("seed state %s: %v", tc.state, err)
		}
		f.expect(id, "for a legacy row in "+tc.state, tc.state, tc.legal...)
	}
	// A legacy row already in customer_approval can have its approval recorded.
	if _, err := f.scoped.Exec(f.sys, `UPDATE change_request SET state = 'CUSTOMER_APPROVAL' WHERE id = $1`, id); err != nil {
		t.Fatalf("seed state: %v", err)
	}
	f.step(id, domain.ChangeRequestStateScheduled, "SCHEDULED", "implement", "canceled")
}

// Migration 0189 is idempotent: re-running it changes neither the columns nor
// any stored value, and the columns are NOT NULL DEFAULT false.
func TestChangeRequestFlowIntegration_CustomerGateMigrationIsIdempotent(t *testing.T) {
	f := newCRFlow(t)
	f.seedAssignedGroup()
	id := f.createGated(domain.ChangeRequestTypeNormal, crFlowGroupID, boolp(true), boolp(true))

	for _, col := range []string{"customer_approval_required", "customer_review_required"} {
		var nullable, def string
		if err := f.scoped.QueryRow(f.sys,
			`SELECT is_nullable, COALESCE(column_default, '') FROM information_schema.columns WHERE table_name = 'change_request' AND column_name = $1`, col).Scan(&nullable, &def); err != nil {
			t.Fatalf("describe %s: %v", col, err)
		}
		if nullable != "NO" || def != "false" {
			t.Fatalf("%s: nullable=%q default=%q, want NOT NULL DEFAULT false", col, nullable, def)
		}
	}

	sqlBytes, err := os.ReadFile("../../migrations/0189_change_request_customer_gates.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	for i := 0; i < 2; i++ {
		if _, err := f.pool.Exec(context.Background(), string(sqlBytes)); err != nil {
			t.Fatalf("re-running migration 0189 (pass %d): %v", i+1, err)
		}
	}
	cr := f.get(id)
	if !cr.CustomerApprovalRequired || !cr.CustomerReviewRequired {
		t.Fatalf("flags after re-running the migration = %v/%v, want the stored true/true", cr.CustomerApprovalRequired, cr.CustomerReviewRequired)
	}
}

// ---------------------------------------------------------------------------
// Customer-scope fields: Customer Project, Deployments and Deployment
// products, plus Category, the read-only Customer Group (the project's
// registered contacts) and the "Additional comments" / "Work notes" journal
// entries (migration 0191, change_request_links.go). Same harness as the
// lifecycle tests above.
// ---------------------------------------------------------------------------

const (
	crScopeAccountID = "3bbbbbbb-0000-0000-0000-000000000001"
	// A second customer: project B belongs to it, so project A's groups are
	// another customer's groups from B's point of view (and vice versa).
	crScopeAccountB = "3bbbbbbb-0000-0000-0000-000000000002"
	crScopeProjectA = "3bbbbbbb-0000-0000-0000-000000000011"
	crScopeProjectB = "3bbbbbbb-0000-0000-0000-000000000012"
	// Project C belongs to account A and has no customer group at all.
	crScopeProjectC = "3bbbbbbb-0000-0000-0000-000000000013"

	// The project contacts (the Customer Group). Project A (customer A) has two
	// registered portal users plus three who do not count: an invited one, a
	// registered one with only the SECURITY_CONTACT role, and a registered one
	// whose user is deactivated. Project B (customer B) has one registered
	// portal user. Project C has none.
	crScopeUserAlice    = "3bbbbbbb-0000-0000-0000-000000000071" // Alice Aaron, registered, A
	crScopeUserBob      = "3bbbbbbb-0000-0000-0000-000000000072" // Bob Bell, registered, A
	crScopeUserInvited  = "3bbbbbbb-0000-0000-0000-000000000073" // invited, A
	crScopeUserSecurity = "3bbbbbbb-0000-0000-0000-000000000074" // registered, SECURITY_CONTACT only, A
	crScopeUserInactive = "3bbbbbbb-0000-0000-0000-000000000075" // registered, user deactivated, A
	crScopeUserCarol    = "3bbbbbbb-0000-0000-0000-000000000076" // Carol Cook, registered, B

	// Project A's deployments. Prod and Stage carry deployed products; Stage2
	// is a second Staging deployment with none; Dev has none; Old is
	// deactivated.
	crScopeDepProd   = "3bbbbbbb-0000-0000-0000-000000000021"
	crScopeDepStage  = "3bbbbbbb-0000-0000-0000-000000000022"
	crScopeDepStage2 = "3bbbbbbb-0000-0000-0000-000000000023"
	crScopeDepDev    = "3bbbbbbb-0000-0000-0000-000000000024"
	crScopeDepOld    = "3bbbbbbb-0000-0000-0000-000000000025"
	// Project B's only deployment.
	crScopeDepOtherB = "3bbbbbbb-0000-0000-0000-000000000026"

	crScopeProductOne = "3bbbbbbb-0000-0000-0000-000000000031"
	crScopeProductTwo = "3bbbbbbb-0000-0000-0000-000000000032"
	crScopeVersion1   = "3bbbbbbb-0000-0000-0000-000000000041"
	crScopeVersion2   = "3bbbbbbb-0000-0000-0000-000000000042"
	crScopeVersion3   = "3bbbbbbb-0000-0000-0000-000000000043"

	crScopeDPProdOne  = "3bbbbbbb-0000-0000-0000-000000000051" // Prod: product one 1.0
	crScopeDPProdTwo  = "3bbbbbbb-0000-0000-0000-000000000052" // Prod: product two 3.0
	crScopeDPStageOne = "3bbbbbbb-0000-0000-0000-000000000053" // Stage: product one 2.0
	crScopeDPInactive = "3bbbbbbb-0000-0000-0000-000000000054" // Prod, deactivated
	crScopeDPOtherB   = "3bbbbbbb-0000-0000-0000-000000000055" // B's deployment
)

// scopeContact describes one project contact for seedScopeContacts.
type scopeContact struct {
	userID, name, project, account, state string
	roles                                 []string // project roles the contact holds
	inactiveUser                          bool
}

// seedScopeContacts creates the users, account contacts, project contacts and
// their PORTAL_USER / SECURITY_CONTACT project roles described by the
// crScopeUser* constants. A contact's email is crFlowEmail(user id), so the user
// can also be the caller of an approval decision.
func (f *crFlow) seedScopeContacts(exec func(sql string, args ...any)) {
	f.t.Helper()
	groups := map[string]string{} // role -> project_group id
	for _, role := range []string{"PORTAL_USER", "SECURITY_CONTACT"} {
		var roleID string
		if err := f.scoped.QueryRow(f.sys, `SELECT id::text FROM project_role WHERE role = $1::project_role_enum`, role).Scan(&roleID); err != nil {
			if err := f.scoped.QueryRow(f.sys,
				`INSERT INTO project_role (id, created_on, updated_on, created_by, updated_by, role)
				 VALUES (gen_random_uuid(), now(), now(), 'cr-scope-test', 'cr-scope-test', $1::project_role_enum) RETURNING id::text`, role).Scan(&roleID); err != nil {
				f.t.Fatalf("seed project_role %s: %v", role, err)
			}
		}
		var gid string
		if err := f.scoped.QueryRow(f.sys,
			`INSERT INTO project_group (id, created_on, updated_on, created_by, updated_by, "group")
			 VALUES (gen_random_uuid(), now(), now(), 'cr-scope-test', 'cr-scope-test', $1) RETURNING id::text`, "CR Scope "+role).Scan(&gid); err != nil {
			f.t.Fatalf("seed project_group %s: %v", role, err)
		}
		exec(`INSERT INTO project_group_role (id, created_on, updated_on, created_by, updated_by, project_group_id, project_role_id)
		      VALUES (gen_random_uuid(), now(), now(), 'cr-scope-test', 'cr-scope-test', $1, $2)`, gid, roleID)
		groups[role] = gid
	}
	for _, c := range []scopeContact{
		{crScopeUserAlice, "Alice Aaron", crScopeProjectA, crScopeAccountID, "REGISTERED", []string{"PORTAL_USER"}, false},
		{crScopeUserBob, "Bob Bell", crScopeProjectA, crScopeAccountID, "REGISTERED", []string{"PORTAL_USER"}, false},
		{crScopeUserInvited, "Ivy Invited", crScopeProjectA, crScopeAccountID, "INVITED", []string{"PORTAL_USER"}, false},
		{crScopeUserSecurity, "Sam Security", crScopeProjectA, crScopeAccountID, "REGISTERED", []string{"SECURITY_CONTACT"}, false},
		{crScopeUserInactive, "Ian Inactive", crScopeProjectA, crScopeAccountID, "REGISTERED", []string{"PORTAL_USER"}, true},
		{crScopeUserCarol, "Carol Cook", crScopeProjectB, crScopeAccountB, "REGISTERED", []string{"PORTAL_USER"}, false},
	} {
		email := crFlowEmail(c.userID)
		exec(`INSERT INTO "user" (id, created_on, updated_on, created_by, updated_by, user_name, name, first_name, last_name, email, is_active, is_system_user)
		      VALUES ($1, now(), now(), 'cr-scope-test', 'cr-scope-test', $2, $3, 'First', 'Last', $2, $4, false)`, c.userID, email, c.name, !c.inactiveUser)
		// The customer's own people: external users, who answer the customer
		// stages (and only those).
		exec(`UPDATE "user" SET user_type = 'EXTERNAL'::user_type_enum WHERE id = $1`, c.userID)
		var acID, pcID string
		if err := f.scoped.QueryRow(f.sys,
			`INSERT INTO account_contact (id, created_on, updated_on, created_by, updated_by, user_name, account_id)
			 VALUES (gen_random_uuid(), now(), now(), 'cr-scope-test', 'cr-scope-test', $1, $2) RETURNING id::text`, email, c.account).Scan(&acID); err != nil {
			f.t.Fatalf("seed account_contact: %v", err)
		}
		if err := f.scoped.QueryRow(f.sys,
			`INSERT INTO project_contact (id, created_on, updated_on, created_by, updated_by, email, account_contact_id, project_id, state)
			 VALUES (gen_random_uuid(), now(), now(), 'cr-scope-test', 'cr-scope-test', $1, $2, $3, $4::project_contact_state_enum) RETURNING id::text`,
			email, acID, c.project, c.state).Scan(&pcID); err != nil {
			f.t.Fatalf("seed project_contact: %v", err)
		}
		for _, role := range c.roles {
			exec(`INSERT INTO project_contact_group (id, created_on, updated_on, created_by, updated_by, project_contact_id, project_group_id)
			      VALUES (gen_random_uuid(), now(), now(), 'cr-scope-test', 'cr-scope-test', $1, $2)`, pcID, groups[role])
		}
	}
}

// contactNames returns the names of a change request's customer contacts.
func contactNames(cs []domain.ChangeRequestCustomerContact) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.Name
	}
	return out
}

// seedScope inserts project A (five deployments of four types, three active
// deployed products and one deactivated; two registered contacts) and project B
// (one deployment with one deployed product; one registered contact of a
// different customer) and project C (nothing), and removes them again on
// cleanup.
func (f *crFlow) seedScope() {
	f.t.Helper()
	exec := func(sql string, args ...any) {
		f.t.Helper()
		if _, err := f.scoped.Exec(f.sys, sql, args...); err != nil {
			f.t.Fatalf("seed scope (%.60s): %v", sql, err)
		}
	}
	clean := func() {
		for _, q := range []string{
			`DELETE FROM work_item WHERE subject = 'cr-approval-flow integration test'`,
			`DELETE FROM deployed_product WHERE id::text LIKE '3bbbbbbb-%'`,
			`DELETE FROM deployment WHERE id::text LIKE '3bbbbbbb-%'`,
			`DELETE FROM project WHERE id::text LIKE '3bbbbbbb-%'`,
			`DELETE FROM product_version WHERE id::text LIKE '3bbbbbbb-%'`,
			`DELETE FROM product WHERE id::text LIKE '3bbbbbbb-%'`,
			`DELETE FROM account WHERE id::text LIKE '3bbbbbbb-%'`,
			`DELETE FROM project_group WHERE "group" LIKE 'CR Scope %'`,
			`DELETE FROM "user" WHERE id::text LIKE '3bbbbbbb-%'`,
		} {
			_, _ = f.scoped.Exec(f.sys, q)
		}
	}
	clean()
	f.t.Cleanup(clean)

	exec(`INSERT INTO account (id, created_on, updated_on, created_by, updated_by, name, number, sf_id)
	      VALUES ($1, now(), now(), 'cr-scope-test', 'cr-scope-test', 'CR Scope Test Account', 'CR-SCOPE-ACC', 'CR-SCOPE-SF')`, crScopeAccountID)
	exec(`INSERT INTO account (id, created_on, updated_on, created_by, updated_by, name, number, sf_id)
	      VALUES ($1, now(), now(), 'cr-scope-test', 'cr-scope-test', 'CR Scope Test Account B', 'CR-SCOPE-ACC-B', 'CR-SCOPE-SF-B')`, crScopeAccountB)
	for _, p := range []struct{ id, key, account string }{
		{crScopeProjectA, "CRSCOPEA", crScopeAccountID}, {crScopeProjectB, "CRSCOPEB", crScopeAccountB}, {crScopeProjectC, "CRSCOPEC", crScopeAccountID},
	} {
		exec(`INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, sf_id, name, account_id)
		      VALUES ($1, now(), now(), 'cr-scope-test', 'cr-scope-test', $2, $2, $2, $3)`, p.id, p.key, p.account)
	}
	f.seedScopeContacts(exec)
	for _, d := range []struct {
		id, number, name, typ, project string
		active                         bool
	}{
		{crScopeDepProd, "CRS-DEP-1", "Scope Prod", "PRIMARY_PRODUCTION", crScopeProjectA, true},
		{crScopeDepStage, "CRS-DEP-2", "Scope Stage", "STAGING", crScopeProjectA, true},
		{crScopeDepStage2, "CRS-DEP-3", "Scope Stage 2", "STAGING", crScopeProjectA, true},
		{crScopeDepDev, "CRS-DEP-4", "Scope Dev", "DEVELOPMENT", crScopeProjectA, true},
		{crScopeDepOld, "CRS-DEP-5", "Scope Old", "QA", crScopeProjectA, false},
		{crScopeDepOtherB, "CRS-DEP-6", "Scope B Prod", "PRIMARY_PRODUCTION", crScopeProjectB, true},
	} {
		exec(`INSERT INTO deployment (id, created_on, updated_on, created_by, updated_by, number, name, type, is_active, project_id)
		      VALUES ($1, now(), now(), 'cr-scope-test', 'cr-scope-test', $2, $3, $4::deployment_type_enum, $5, $6)`,
			d.id, d.number, d.name, d.typ, d.active, d.project)
	}
	for id, name := range map[string]string{crScopeProductOne: "Scope Product One", crScopeProductTwo: "Scope Product Two"} {
		exec(`INSERT INTO product (id, created_on, updated_on, created_by, updated_by, manufacturer, category, name)
		      VALUES ($1, now(), now(), 'cr-scope-test', 'cr-scope-test', 'WSO2', 'SOFTWARE', $2)`, id, name)
	}
	for _, v := range []struct{ id, version, product string }{
		{crScopeVersion1, "1.0", crScopeProductOne}, {crScopeVersion2, "2.0", crScopeProductOne}, {crScopeVersion3, "3.0", crScopeProductTwo},
	} {
		exec(`INSERT INTO product_version (id, created_on, updated_on, created_by, updated_by, version, product_id, current_support_status, release_date)
		      VALUES ($1, now(), now(), 'cr-scope-test', 'cr-scope-test', $2, $3, 'AVAILABLE', '2024-01-01')`, v.id, v.version, v.product)
	}
	for _, p := range []struct {
		id, number, deployment, product, version string
		active                                   bool
	}{
		{crScopeDPProdOne, "CRS-DP-1", crScopeDepProd, crScopeProductOne, crScopeVersion1, true},
		{crScopeDPProdTwo, "CRS-DP-2", crScopeDepProd, crScopeProductTwo, crScopeVersion3, true},
		{crScopeDPStageOne, "CRS-DP-3", crScopeDepStage, crScopeProductOne, crScopeVersion2, true},
		{crScopeDPInactive, "CRS-DP-4", crScopeDepProd, crScopeProductOne, crScopeVersion2, false},
		{crScopeDPOtherB, "CRS-DP-5", crScopeDepOtherB, crScopeProductOne, crScopeVersion1, true},
	} {
		exec(`INSERT INTO deployed_product (id, created_on, updated_on, created_by, updated_by, number, name, active, deployment_id, product_id, version_id, product_category)
		      VALUES ($1, now(), now(), 'cr-scope-test', 'cr-scope-test', $2, $2, $3, $4, $5, $6, 'PDP')`,
			p.id, p.number, p.active, p.deployment, p.product, p.version)
	}
}

func scopeStrp(s string) *string { return &s }

// createScoped creates a Normal change request with the given scope fields.
func (f *crFlow) createScoped(mod func(*domain.CreateChangeRequestRequest)) (string, error) {
	f.t.Helper()
	typ := domain.ChangeRequestTypeNormal
	req := domain.CreateChangeRequestRequest{Subject: crFlowSubject, Type: &typ}
	if mod != nil {
		mod(&req)
	}
	resp, err := f.repo.CreateChangeRequest(f.sys, req, crFlowEmail(crFlowCreatorID))
	if err != nil {
		return "", err
	}
	return resp.ChangeRequest.ID, nil
}

func (f *crFlow) mustCreateScoped(mod func(*domain.CreateChangeRequestRequest)) string {
	f.t.Helper()
	id, err := f.createScoped(mod)
	if err != nil {
		f.t.Fatalf("CreateChangeRequest: %v", err)
	}
	return id
}

func (f *crFlow) mustPatch(id string, req domain.PatchChangeRequestRequest) domain.ChangeRequest {
	f.t.Helper()
	cr, err := f.patch(id, req)
	if err != nil {
		f.t.Fatalf("PATCH: %v", err)
	}
	return cr
}

func (f *crFlow) setStoredState(id, state string) {
	f.t.Helper()
	if _, err := f.scoped.Exec(f.sys, `UPDATE change_request SET state = $1::change_request_state_enum WHERE id = $2`, state, id); err != nil {
		f.t.Fatalf("force state %s: %v", state, err)
	}
}

func scopeIDs(refs []domain.EntityRef) []string {
	out := make([]string, len(refs))
	for i, r := range refs {
		out[i] = r.ID
	}
	sort.Strings(out)
	return out
}

func scopeNames(refs []domain.EntityRef) []string {
	out := make([]string, len(refs))
	for i, r := range refs {
		out[i] = r.Name
	}
	sort.Strings(out)
	return out
}

func (f *crFlow) assertScope(what string, cr domain.ChangeRequest, project string, deployments, products []string) {
	f.t.Helper()
	if cr.Project.ID != project {
		f.t.Fatalf("%s: project = %q, want %q", what, cr.Project.ID, project)
	}
	for name, c := range map[string]struct {
		got  []domain.EntityRef
		want []string
	}{"deployments": {cr.Deployments, deployments}, "deploymentProducts": {cr.DeploymentProducts, products}} {
		if c.got == nil {
			f.t.Fatalf("%s: %s is nil, want a (possibly empty) array", what, name)
		}
		want := append([]string{}, c.want...)
		sort.Strings(want)
		if got := scopeIDs(c.got); strings.Join(got, ",") != strings.Join(want, ",") {
			f.t.Fatalf("%s: %s = %v, want %v", what, name, got, want)
		}
	}
}

func (f *crFlow) comments(id string) map[string][]string {
	f.t.Helper()
	rows, err := f.scoped.Query(f.sys, `SELECT type::text, content, created_by FROM comment WHERE work_item_id = $1 ORDER BY created_on, id`, id)
	if err != nil {
		f.t.Fatalf("read comments: %v", err)
	}
	defer rows.Close()
	out := map[string][]string{}
	for rows.Next() {
		var typ, content, by string
		if err := rows.Scan(&typ, &content, &by); err != nil {
			f.t.Fatalf("scan comment: %v", err)
		}
		out[typ] = append(out[typ], content+"|"+by)
	}
	return out
}

func (f *crFlow) crCount() int {
	f.t.Helper()
	var n int
	if err := f.scoped.QueryRow(f.sys, `SELECT count(*) FROM work_item WHERE subject = $1`, crFlowSubject).Scan(&n); err != nil {
		f.t.Fatalf("count change requests: %v", err)
	}
	return n
}

// Every scope field persists on create and comes back on GET: project,
// deployments, deployment products (derived, deactivated ones excluded),
// category, and the two journal entries as comment rows of the right types; the
// Customer Group is not stored but derived from the project's registered
// contacts.
func TestChangeRequestScopeIntegration_CreatePersistsEveryField(t *testing.T) {
	for _, path := range []string{"portal", "servicenow-first"} {
		path := path
		t.Run(path, func(t *testing.T) {
			f := newCRFlow(t)
			f.seedScope()
			cat := domain.ChangeRequestCategoryDevOps
			mod := func(r *domain.CreateChangeRequestRequest) {
				r.ProjectID = scopeStrp(crScopeProjectA)
				r.DeploymentIDs = []string{crScopeDepProd, crScopeDepStage}
				r.Category = &cat
				r.Comment = scopeStrp("customer visible note")
				r.WorkNote = scopeStrp("internal note")
			}
			var id string
			if path == "portal" {
				id = f.mustCreateScoped(mod)
			} else {
				typ := domain.ChangeRequestTypeNormal
				req := domain.CreateChangeRequestRequest{Subject: crFlowSubject, Type: &typ}
				mod(&req)
				id = "3bbbbbbb-0000-0000-0000-0000000000f1"
				if _, err := f.repo.CreateChangeRequestFromServiceNow(f.sys, req, id, "CRSCOPESN01", crFlowEmail(crFlowCreatorID)); err != nil {
					t.Fatalf("CreateChangeRequestFromServiceNow: %v", err)
				}
			}
			cr := f.get(id)
			f.assertScope("after create", cr, crScopeProjectA,
				[]string{crScopeDepProd, crScopeDepStage},
				[]string{crScopeDPProdOne, crScopeDPProdTwo, crScopeDPStageOne})
			if got := scopeNames(cr.DeploymentProducts); strings.Join(got, ",") != "Scope Product One 1.0,Scope Product One 2.0,Scope Product Two 3.0" {
				t.Fatalf("deploymentProducts names = %v, want \"<product> <version>\"", got)
			}
			if cr.Category == nil || *cr.Category != "devops" {
				t.Fatalf("category = %v, want devops", cr.Category)
			}
			if got := contactNames(cr.CustomerContacts); strings.Join(got, ",") != "Alice Aaron,Bob Bell" {
				t.Fatalf("customerContacts = %v, want project A's registered portal-user contacts (name order)", got)
			}
			if got := f.storedCustomerGroup(id); got != nil {
				t.Fatalf("customer_group_id = %s, want it left unwritten", *got)
			}
			// The single-valued columns the list views read follow the first
			// deployment (name order) and its first deployed product.
			if cr.Deployment == nil || cr.Deployment.ID != crScopeDepProd {
				t.Fatalf("deployment = %+v, want %s", cr.Deployment, crScopeDepProd)
			}
			if cr.DeployedProduct == nil || cr.DeployedProduct.ID != crScopeDPProdOne {
				t.Fatalf("deployedProduct = %+v, want %s", cr.DeployedProduct, crScopeDPProdOne)
			}
			got := f.comments(id)
			by := crFlowEmail(crFlowCreatorID)
			if len(got["COMMENT"]) != 1 || got["COMMENT"][0] != "customer visible note|"+by || len(got["WORK_NOTE"]) != 1 || got["WORK_NOTE"][0] != "internal note|"+by || len(got) != 2 {
				t.Fatalf("comment rows = %v, want one COMMENT and one WORK_NOTE by %s", got, by)
			}
		})
	}
}

// A change request with no scope fields reads back with empty arrays, not null,
// and a blank journal entry creates no comment row.
func TestChangeRequestScopeIntegration_CreateWithoutScopeReadsEmptyArrays(t *testing.T) {
	f := newCRFlow(t)
	f.seedScope()
	id := f.mustCreateScoped(func(r *domain.CreateChangeRequestRequest) {
		r.Comment = scopeStrp("   ")
		r.WorkNote = scopeStrp("")
	})
	cr := f.get(id)
	f.assertScope("bare create", cr, "", nil, nil)
	if cr.Category != nil {
		t.Fatalf("category = %v, want unset", cr.Category)
	}
	if cr.CustomerContacts == nil || len(cr.CustomerContacts) != 0 {
		t.Fatalf("customerContacts = %v, want an empty array without a project", cr.CustomerContacts)
	}
	if got := f.comments(id); len(got) != 0 {
		t.Fatalf("blank journal entries created comment rows: %v", got)
	}
}

// A project alone is stored (no deployments needed).
func TestChangeRequestScopeIntegration_CreateProjectOnly(t *testing.T) {
	f := newCRFlow(t)
	f.seedScope()
	id := f.mustCreateScoped(func(r *domain.CreateChangeRequestRequest) { r.ProjectID = scopeStrp(crScopeProjectA) })
	f.assertScope("project only", f.get(id), crScopeProjectA, nil, nil)
}

// Every combination the rules refuse is a ValidationError naming the field and
// leaves no change request behind (the create is all-or-nothing).
func TestChangeRequestScopeIntegration_CreateRejectsInconsistentSelections(t *testing.T) {
	f := newCRFlow(t)
	f.seedScope()
	unknown := "3bbbbbbb-9999-0000-0000-000000000000"
	for _, tc := range []struct {
		name     string
		mod      func(*domain.CreateChangeRequestRequest)
		contains string
	}{
		{"deployment of another project", func(r *domain.CreateChangeRequestRequest) {
			r.ProjectID = scopeStrp(crScopeProjectA)
			r.DeploymentIDs = []string{crScopeDepProd, crScopeDepOtherB}
		}, "does not belong to the selected project: " + crScopeDepOtherB},
		{"deployments without a project", func(r *domain.CreateChangeRequestRequest) {
			r.DeploymentIDs = []string{crScopeDepProd}
		}, "projectId is required when deploymentIds are provided"},
		{"unknown deployment", func(r *domain.CreateChangeRequestRequest) {
			r.ProjectID = scopeStrp(crScopeProjectA)
			r.DeploymentIDs = []string{unknown}
		}, "unknown deployment: " + unknown},
		{"inactive deployment", func(r *domain.CreateChangeRequestRequest) {
			r.ProjectID = scopeStrp(crScopeProjectA)
			r.DeploymentIDs = []string{crScopeDepOld}
		}, "inactive deployment: " + crScopeDepOld},
		{"unknown project", func(r *domain.CreateChangeRequestRequest) {
			r.ProjectID = scopeStrp(unknown)
		}, "projectId does not refer to an existing project"},
		{"deployment products without deployments", func(r *domain.CreateChangeRequestRequest) {
			r.ProjectID = scopeStrp(crScopeProjectA)
			r.DeploymentProductIDs = []string{crScopeDPProdOne}
		}, "deploymentProductIds requires deploymentIds"},
		{"deployment products: a subset of the derived set", func(r *domain.CreateChangeRequestRequest) {
			r.ProjectID = scopeStrp(crScopeProjectA)
			r.DeploymentIDs = []string{crScopeDepProd}
			r.DeploymentProductIDs = []string{crScopeDPProdOne}
		}, "deploymentProductIds is read-only"},
		{"deployment products: another deployment's", func(r *domain.CreateChangeRequestRequest) {
			r.ProjectID = scopeStrp(crScopeProjectA)
			r.DeploymentIDs = []string{crScopeDepProd}
			r.DeploymentProductIDs = []string{crScopeDPProdOne, crScopeDPProdTwo, crScopeDPStageOne}
		}, "deploymentProductIds is read-only"},
		{"deployment products: includes a deactivated one", func(r *domain.CreateChangeRequestRequest) {
			r.ProjectID = scopeStrp(crScopeProjectA)
			r.DeploymentIDs = []string{crScopeDepProd}
			r.DeploymentProductIDs = []string{crScopeDPProdOne, crScopeDPProdTwo, crScopeDPInactive}
		}, "deploymentProductIds is read-only"},
		{"customerGroupId is no longer accepted", func(r *domain.CreateChangeRequestRequest) {
			r.ProjectID = scopeStrp(crScopeProjectA)
			r.CustomerGroupID = scopeStrp(seededGroupID)
		}, crScopeMsgGroupRemoved},
		{"environmentIds is no longer supported", func(r *domain.CreateChangeRequestRequest) {
			r.ProjectID = scopeStrp(crScopeProjectA)
			r.DeploymentIDs = []string{crScopeDepProd}
			r.EnvironmentIDs = []string{"3bbbbbbb-0000-0000-0000-0000000000e1"}
		}, crScopeMsgEnvRemoved},
		{"too many deployments", func(r *domain.CreateChangeRequestRequest) {
			r.ProjectID = scopeStrp(crScopeProjectA)
			for i := 0; i < 101; i++ {
				r.DeploymentIDs = append(r.DeploymentIDs, fmt.Sprintf("3bbbbbbb-0000-0000-0001-%012d", i))
			}
		}, "deploymentIds must contain at most 100 entries"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			_, err := f.createScoped(tc.mod)
			f.wantValidationError(tc.name, err, tc.contains)
			if n := f.crCount(); n != 0 {
				t.Fatalf("%d change request(s) left behind by a refused create", n)
			}
		})
	}
}

// Deployment products stated exactly are accepted; duplicates in the lists are
// collapsed; a deployment with no deployed products derives none.
func TestChangeRequestScopeIntegration_CreateExplicitProductsAndDuplicates(t *testing.T) {
	f := newCRFlow(t)
	f.seedScope()
	id := f.mustCreateScoped(func(r *domain.CreateChangeRequestRequest) {
		r.ProjectID = scopeStrp(crScopeProjectA)
		r.DeploymentIDs = []string{crScopeDepProd, crScopeDepStage}
		r.DeploymentProductIDs = []string{crScopeDPProdOne, crScopeDPProdTwo, crScopeDPStageOne}
	})
	f.assertScope("explicit products", f.get(id), crScopeProjectA,
		[]string{crScopeDepProd, crScopeDepStage},
		[]string{crScopeDPProdOne, crScopeDPProdTwo, crScopeDPStageOne})

	id = f.mustCreateScoped(func(r *domain.CreateChangeRequestRequest) {
		r.ProjectID = scopeStrp(crScopeProjectA)
		r.DeploymentIDs = []string{crScopeDepStage, crScopeDepStage2, crScopeDepStage}
	})
	f.assertScope("two staging deployments", f.get(id), crScopeProjectA,
		[]string{crScopeDepStage, crScopeDepStage2}, []string{crScopeDPStageOne})

	id = f.mustCreateScoped(func(r *domain.CreateChangeRequestRequest) {
		r.ProjectID = scopeStrp(crScopeProjectA)
		r.DeploymentIDs = []string{crScopeDepDev}
	})
	f.assertScope("dev only", f.get(id), crScopeProjectA, []string{crScopeDepDev}, nil)
}

// Category: all 13 values of the API enum persist (four needed new enum labels),
// on create and on PATCH.
func TestChangeRequestScopeIntegration_PersistsEveryCategory(t *testing.T) {
	f := newCRFlow(t)
	for _, c := range []domain.ChangeRequestCategory{
		domain.ChangeRequestCategoryHardware, domain.ChangeRequestCategorySoftware, domain.ChangeRequestCategoryService,
		domain.ChangeRequestCategorySystemSoftware, domain.ChangeRequestCategoryApplicationsSoftware,
		domain.ChangeRequestCategoryNetwork, domain.ChangeRequestCategoryTelecom, domain.ChangeRequestCategoryDocumentation,
		domain.ChangeRequestCategoryOther, domain.ChangeRequestCategoryRegularReleaseCloud,
		domain.ChangeRequestCategoryHotfixReleaseCloud, domain.ChangeRequestCategoryDevOps, domain.ChangeRequestCategoryCloudComputing,
	} {
		c := c
		id := f.mustCreateScoped(func(r *domain.CreateChangeRequestRequest) { r.Category = &c })
		if got := f.get(id).Category; got == nil || *got != string(c) {
			t.Fatalf("category after create = %v, want %s", got, c)
		}
		// ...and via PATCH, to a different value and back.
		other := domain.ChangeRequestCategoryOther
		if c == other {
			other = domain.ChangeRequestCategorySoftware
		}
		oc := &other
		f.mustPatch(id, domain.PatchChangeRequestRequest{Category: &oc})
		if got := f.get(id).Category; got == nil || *got != string(other) {
			t.Fatalf("category after PATCH = %v, want %s", got, other)
		}
		cc := &c
		f.mustPatch(id, domain.PatchChangeRequestRequest{Category: &cc})
		if got := f.get(id).Category; got == nil || *got != string(c) {
			t.Fatalf("category after PATCH back = %v, want %s", got, c)
		}
	}
	bogus := domain.ChangeRequestCategory("bogus")
	_, err := f.createScoped(func(r *domain.CreateChangeRequestRequest) { r.Category = &bogus })
	f.wantValidationError("bogus category", err, "not supported")
}

// PATCH arrays replace: deployments, the environments and deployment products
// that follow, and the single-valued columns, through to an empty array.
func TestChangeRequestScopeIntegration_PatchArraysReplace(t *testing.T) {
	f := newCRFlow(t)
	f.seedScope()
	id := f.mustCreateScoped(func(r *domain.CreateChangeRequestRequest) {
		r.ProjectID = scopeStrp(crScopeProjectA)
		r.DeploymentIDs = []string{crScopeDepProd, crScopeDepStage}
	})

	cr := f.mustPatch(id, domain.PatchChangeRequestRequest{DeploymentIDs: &[]string{crScopeDepStage}})
	f.assertScope("replace with [stage]", cr, crScopeProjectA, []string{crScopeDepStage}, []string{crScopeDPStageOne})
	if cr.Deployment == nil || cr.Deployment.ID != crScopeDepStage || cr.DeployedProduct == nil || cr.DeployedProduct.ID != crScopeDPStageOne {
		t.Fatalf("single-valued deployment/deployedProduct = %+v/%+v, want stage / its product", cr.Deployment, cr.DeployedProduct)
	}

	cr = f.mustPatch(id, domain.PatchChangeRequestRequest{DeploymentIDs: &[]string{crScopeDepProd, crScopeDepDev}})
	f.assertScope("replace with [prod, dev]", cr, crScopeProjectA,
		[]string{crScopeDepProd, crScopeDepDev}, []string{crScopeDPProdOne, crScopeDPProdTwo})

	cr = f.mustPatch(id, domain.PatchChangeRequestRequest{DeploymentIDs: &[]string{}})
	f.assertScope("cleared", cr, crScopeProjectA, nil, nil)
	if cr.Deployment != nil || cr.DeployedProduct != nil {
		t.Fatalf("single-valued deployment/deployedProduct after clearing = %+v/%+v, want nil", cr.Deployment, cr.DeployedProduct)
	}
}

// An unrelated PATCH leaves the deployments and products alone; a client that
// still sends customerGroupId (a group, or null) or environmentIds is refused
// with a clear 400 and nothing of the request is applied -- the Customer Group
// is derived from the project and a deployment carries its environment.
func TestChangeRequestScopeIntegration_PatchRefusesRemovedFields(t *testing.T) {
	f := newCRFlow(t)
	f.seedScope()
	id := f.mustCreateScoped(func(r *domain.CreateChangeRequestRequest) {
		r.ProjectID = scopeStrp(crScopeProjectA)
		r.DeploymentIDs = []string{crScopeDepProd, crScopeDepStage}
	})
	both := []string{crScopeDepProd, crScopeDepStage}
	allProducts := []string{crScopeDPProdOne, crScopeDPProdTwo, crScopeDPStageOne}

	title := "unrelated edit"
	cr := f.mustPatch(id, domain.PatchChangeRequestRequest{Title: &title})
	f.assertScope("after unrelated PATCH", cr, crScopeProjectA, both, allProducts)

	group := scopeStrp(seededGroupID)
	var none *string
	other := "must not be applied"
	for name, tc := range map[string]struct {
		req  domain.PatchChangeRequestRequest
		want string
	}{
		"customerGroupId set":  {domain.PatchChangeRequestRequest{Title: &other, CustomerGroupID: &group}, crScopeMsgGroupRemoved},
		"customerGroupId null": {domain.PatchChangeRequestRequest{Title: &other, CustomerGroupID: &none}, crScopeMsgGroupRemoved},
		"environmentIds":       {domain.PatchChangeRequestRequest{Title: &other, EnvironmentIDs: &[]string{"3bbbbbbb-0000-0000-0000-0000000000e1"}}, crScopeMsgEnvRemoved},
		"environmentIds empty": {domain.PatchChangeRequestRequest{Title: &other, EnvironmentIDs: &[]string{}}, crScopeMsgEnvRemoved},
	} {
		_, err := f.patch(id, tc.req)
		f.wantValidationMessage(name, err, tc.want)
	}
	cr = f.get(id)
	f.assertScope("after refused PATCHes", cr, crScopeProjectA, both, allProducts)
	if cr.Subject != nil && *cr.Subject == other {
		t.Fatal("a refused PATCH applied its other fields")
	}
	if g := f.storedCustomerGroup(id); g != nil {
		t.Fatalf("customer_group_id = %s, want it never written", *g)
	}
}

// Deployments must belong to the project on PATCH too, and a refused PATCH
// leaves everything as it was.
func TestChangeRequestScopeIntegration_PatchRejectsInconsistentSelections(t *testing.T) {
	f := newCRFlow(t)
	f.seedScope()
	id := f.mustCreateScoped(func(r *domain.CreateChangeRequestRequest) {
		r.ProjectID = scopeStrp(crScopeProjectA)
		r.DeploymentIDs = []string{crScopeDepProd}
	})
	title := "must not be applied"
	manyIDs := make([]string, 101)
	for i := range manyIDs {
		manyIDs[i] = fmt.Sprintf("3bbbbbbb-0000-0000-0002-%012d", i)
	}
	for _, tc := range []struct {
		name     string
		req      domain.PatchChangeRequestRequest
		contains string
	}{
		{"deployment of another project", domain.PatchChangeRequestRequest{Title: &title, DeploymentIDs: &[]string{crScopeDepOtherB}}, "does not belong to the selected project"},
		{"inactive deployment", domain.PatchChangeRequestRequest{DeploymentIDs: &[]string{crScopeDepOld}}, "inactive deployment"},
		{"unknown deployment", domain.PatchChangeRequestRequest{DeploymentIDs: &[]string{"3bbbbbbb-9999-0000-0000-000000000000"}}, "unknown deployment"},
		{"project changed without deployments", domain.PatchChangeRequestRequest{ProjectID: scopeStrp(crScopeProjectB)}, "projectId cannot be changed without deploymentIds"},
		{"project changed, old deployments kept", domain.PatchChangeRequestRequest{ProjectID: scopeStrp(crScopeProjectB), DeploymentIDs: &[]string{crScopeDepProd}}, "does not belong to the selected project"},
		{"unknown project", domain.PatchChangeRequestRequest{ProjectID: scopeStrp("3bbbbbbb-9999-0000-0000-000000000000"), DeploymentIDs: &[]string{}}, "projectId does not refer to an existing project"},
		{"deployment products: not the derived set", domain.PatchChangeRequestRequest{DeploymentProductIDs: &[]string{crScopeDPProdOne}}, "deploymentProductIds is read-only"},
		{"deployment products: another project's", domain.PatchChangeRequestRequest{DeploymentProductIDs: &[]string{crScopeDPOtherB}}, "deploymentProductIds is read-only"},
		{"single deployment fields with deploymentIds", domain.PatchChangeRequestRequest{DeploymentIDs: &[]string{crScopeDepProd}, DeploymentID: scopeStrp(crScopeDepProd)}, "cannot be combined with deploymentIds"},
		{"too many ids", domain.PatchChangeRequestRequest{DeploymentProductIDs: &manyIDs}, "deploymentProductIds must contain at most 100 entries"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			_, err := f.patch(id, tc.req)
			f.wantValidationError(tc.name, err, tc.contains)
			cr := f.get(id)
			f.assertScope("after refused PATCH", cr, crScopeProjectA, []string{crScopeDepProd}, []string{crScopeDPProdOne, crScopeDPProdTwo})
			if cr.Subject != nil && *cr.Subject == title {
				t.Fatal("a refused PATCH applied its other fields")
			}
		})
	}
}

// Moving the change to another project: the deployments must be re-chosen from
// the new project in the same PATCH (an empty array clears them).
func TestChangeRequestScopeIntegration_PatchMovesProject(t *testing.T) {
	f := newCRFlow(t)
	f.seedScope()
	id := f.mustCreateScoped(func(r *domain.CreateChangeRequestRequest) {
		r.ProjectID = scopeStrp(crScopeProjectA)
		r.DeploymentIDs = []string{crScopeDepProd, crScopeDepStage}
	})
	cr := f.mustPatch(id, domain.PatchChangeRequestRequest{ProjectID: scopeStrp(crScopeProjectB), DeploymentIDs: &[]string{crScopeDepOtherB}})
	f.assertScope("moved to B", cr, crScopeProjectB, []string{crScopeDepOtherB}, []string{crScopeDPOtherB})

	cr = f.mustPatch(id, domain.PatchChangeRequestRequest{ProjectID: scopeStrp(crScopeProjectA), DeploymentIDs: &[]string{}})
	f.assertScope("back to A with no deployments", cr, crScopeProjectA, nil, nil)

	// With no deployments stored, the project alone can change.
	cr = f.mustPatch(id, domain.PatchChangeRequestRequest{ProjectID: scopeStrp(crScopeProjectB)})
	f.assertScope("project alone", cr, crScopeProjectB, nil, nil)

	// A change request created without a project can be given one later.
	id2 := f.mustCreateScoped(nil)
	cr = f.mustPatch(id2, domain.PatchChangeRequestRequest{ProjectID: scopeStrp(crScopeProjectA), DeploymentIDs: &[]string{crScopeDepDev}})
	f.assertScope("project added later", cr, crScopeProjectA, []string{crScopeDepDev}, nil)
}

// Deployment products are read-only: stated exactly (or as the stored
// snapshot) they are accepted, and the stored list is always the derived one.
func TestChangeRequestScopeIntegration_PatchDeploymentProductsReadOnly(t *testing.T) {
	f := newCRFlow(t)
	f.seedScope()
	id := f.mustCreateScoped(func(r *domain.CreateChangeRequestRequest) {
		r.ProjectID = scopeStrp(crScopeProjectA)
		r.DeploymentIDs = []string{crScopeDepProd}
	})
	derived := []string{crScopeDPProdOne, crScopeDPProdTwo}
	cr := f.mustPatch(id, domain.PatchChangeRequestRequest{DeploymentProductIDs: &derived})
	f.assertScope("exact set resent", cr, crScopeProjectA, []string{crScopeDepProd}, derived)

	// The deployment gains a product after the change request was raised: the
	// stored snapshot stays until the deployments are re-chosen, and resending
	// the snapshot is still accepted.
	const extra = "3bbbbbbb-0000-0000-0000-000000000056"
	if _, err := f.scoped.Exec(f.sys,
		`INSERT INTO deployed_product (id, created_on, updated_on, created_by, updated_by, number, name, active, deployment_id, product_id, version_id, product_category)
		 VALUES ($1, now(), now(), 'cr-scope-test', 'cr-scope-test', 'CRS-DP-6', 'CRS-DP-6', true, $2, $3, $4, 'PDP')`,
		extra, crScopeDepProd, crScopeProductTwo, crScopeVersion3); err != nil {
		t.Fatalf("add deployed product: %v", err)
	}
	f.assertScope("snapshot unchanged", f.get(id), crScopeProjectA, []string{crScopeDepProd}, derived)
	f.mustPatch(id, domain.PatchChangeRequestRequest{DeploymentProductIDs: &derived})
	cr = f.mustPatch(id, domain.PatchChangeRequestRequest{DeploymentIDs: &[]string{crScopeDepProd}})
	f.assertScope("snapshot kept while the deployments are the same", cr, crScopeProjectA, []string{crScopeDepProd}, derived)
	// Re-choosing the deployments re-derives, picking up the new product.
	cr = f.mustPatch(id, domain.PatchChangeRequestRequest{DeploymentIDs: &[]string{crScopeDepStage, crScopeDepProd}})
	f.assertScope("re-derived", cr, crScopeProjectA, []string{crScopeDepProd, crScopeDepStage},
		[]string{crScopeDPProdOne, crScopeDPProdTwo, crScopeDPStageOne, extra})
}

// The edit window: project, deployments and deployment products
// change freely through Scheduled and are refused from Implement on; resending
// the stored values is always accepted; everything else stays editable.
func TestChangeRequestScopeIntegration_PatchEditWindow(t *testing.T) {
	for state, open := range map[string]bool{
		"NEW": true, "ASSESS": true, "AUTHORIZE": true, "CUSTOMER_APPROVAL": true, "SCHEDULED": true,
		"IMPLEMENT": false, "REVIEW": false, "CUSTOMER_REVIEW": false, "ROLLBACK": false, "CLOSED": false, "CANCELED": false,
	} {
		state, open := state, open
		t.Run(state, func(t *testing.T) {
			f := newCRFlow(t)
			f.seedScope()
			id := f.mustCreateScoped(func(r *domain.CreateChangeRequestRequest) {
				r.ProjectID = scopeStrp(crScopeProjectA)
				r.DeploymentIDs = []string{crScopeDepProd, crScopeDepStage}
			})
			f.setStoredState(id, state)
			stored := f.get(id)

			// Resending what is stored is always fine.
			storedProducts := scopeIDs(stored.DeploymentProducts)
			f.mustPatch(id, domain.PatchChangeRequestRequest{
				ProjectID: scopeStrp(crScopeProjectA), DeploymentIDs: &[]string{crScopeDepStage, crScopeDepProd},
				DeploymentProductIDs: &storedProducts,
			})

			attempts := map[string]domain.PatchChangeRequestRequest{
				"deploymentIds": {DeploymentIDs: &[]string{crScopeDepProd}},
				"projectId":     {ProjectID: scopeStrp(crScopeProjectB), DeploymentIDs: &[]string{crScopeDepOtherB}},
			}
			for field, req := range attempts {
				_, err := f.patch(id, req)
				if open {
					if err != nil {
						t.Fatalf("%s change in %s: %v", field, state, err)
					}
					// Put it back for the next attempt.
					f.mustPatch(id, domain.PatchChangeRequestRequest{ProjectID: scopeStrp(crScopeProjectA), DeploymentIDs: &[]string{crScopeDepProd, crScopeDepStage}})
					continue
				}
				f.wantValidationError(field+" in "+state, err, "can no longer be changed")
				if !strings.Contains(err.Error(), strings.ToLower(state)) {
					t.Fatalf("message %q should name the state %q", err.Error(), strings.ToLower(state))
				}
				f.assertScope("after refused "+field, f.get(id), crScopeProjectA, []string{crScopeDepProd, crScopeDepStage}, storedProducts)
			}

			// Not part of the window, in any state: journal entries and category.
			cat := domain.ChangeRequestCategoryNetwork
			cp := &cat
			f.mustPatch(id, domain.PatchChangeRequestRequest{Comment: scopeStrp("still allowed"), WorkNote: scopeStrp("still allowed"), Category: &cp})
		})
	}
}

// Comment / workNote on PATCH append comment rows of the right types, authored
// by the caller; blank values are refused; each works alone.
func TestChangeRequestScopeIntegration_PatchAppendsJournalEntries(t *testing.T) {
	f := newCRFlow(t)
	f.seedScope()
	id := f.mustCreateScoped(nil)
	by := crFlowEmail(crFlowCreatorID)

	f.mustPatch(id, domain.PatchChangeRequestRequest{Comment: scopeStrp("first comment")})
	got := f.comments(id)
	if len(got) != 1 || len(got["COMMENT"]) != 1 || got["COMMENT"][0] != "first comment|"+by {
		t.Fatalf("after comment alone: %v", got)
	}
	f.mustPatch(id, domain.PatchChangeRequestRequest{WorkNote: scopeStrp("a work note")})
	f.mustPatch(id, domain.PatchChangeRequestRequest{Comment: scopeStrp("second comment"), WorkNote: scopeStrp("second work note")})
	got = f.comments(id)
	if len(got["COMMENT"]) != 2 || len(got["WORK_NOTE"]) != 2 || got["COMMENT"][1] != "second comment|"+by || got["WORK_NOTE"][0] != "a work note|"+by {
		t.Fatalf("after three PATCHes: %v", got)
	}

	for _, req := range []domain.PatchChangeRequestRequest{
		{Comment: scopeStrp("")}, {Comment: scopeStrp("  \n")}, {WorkNote: scopeStrp("")}, {WorkNote: scopeStrp("\t")},
	} {
		_, err := f.patch(id, req)
		f.wantValidationError("blank journal entry", err, "must not be empty")
	}
	if after := f.comments(id); len(after["COMMENT"]) != 2 || len(after["WORK_NOTE"]) != 2 {
		t.Fatalf("a refused blank entry changed the journal: %v", after)
	}
}

// The Customer Group is derived live from the project's registered portal-user
// contacts, read-only: it follows a project change, tracks a contact being
// deregistered or deactivated, is empty without a project or without contacts,
// and never mixes two customers' contacts.
func TestChangeRequestScopeIntegration_CustomerContactsAreDerivedFromTheProject(t *testing.T) {
	f := newCRFlow(t)
	f.seedScope()
	names := func(id string) string { return strings.Join(contactNames(f.get(id).CustomerContacts), ",") }

	id := f.mustCreateScoped(nil)
	if cr := f.get(id); cr.CustomerContacts == nil || len(cr.CustomerContacts) != 0 {
		t.Fatalf("customerContacts without a project = %v, want []", cr.CustomerContacts)
	}
	// Setting the project derives its contacts: registered + PORTAL_USER only
	// (the invited, security-only and deactivated users of project A do not count).
	f.mustPatch(id, domain.PatchChangeRequestRequest{ProjectID: scopeStrp(crScopeProjectA)})
	if got := names(id); got != "Alice Aaron,Bob Bell" {
		t.Fatalf("project A contacts = %q, want Alice Aaron,Bob Bell", got)
	}
	cr := f.get(id)
	if cr.CustomerContacts[0].ID == "" || cr.CustomerContacts[0].Email != crFlowEmail(crScopeUserAlice) {
		t.Fatalf("contact = %+v, want an id and the contact's email", cr.CustomerContacts[0])
	}
	// Changing the project re-derives: customer B's contact replaces customer A's.
	f.mustPatch(id, domain.PatchChangeRequestRequest{ProjectID: scopeStrp(crScopeProjectB), DeploymentIDs: &[]string{}})
	if got := names(id); got != "Carol Cook" {
		t.Fatalf("project B contacts = %q, want only Carol Cook (customer B's)", got)
	}
	f.mustPatch(id, domain.PatchChangeRequestRequest{ProjectID: scopeStrp(crScopeProjectC)})
	if got := names(id); got != "" {
		t.Fatalf("project C contacts = %q, want none", got)
	}
	// Computed live on every read: a contact who stops being registered, or whose
	// user is deactivated, drops out; one who registers appears.
	f.mustPatch(id, domain.PatchChangeRequestRequest{ProjectID: scopeStrp(crScopeProjectA)})
	f.execSQL(`UPDATE project_contact SET state = 'DEACTIVATED' WHERE email = $1`, crFlowEmail(crScopeUserBob))
	if got := names(id); got != "Alice Aaron" {
		t.Fatalf("after deactivating Bob = %q, want Alice Aaron", got)
	}
	f.execSQL(`UPDATE project_contact SET state = 'REGISTERED' WHERE email = $1`, crFlowEmail(crScopeUserInvited))
	if got := names(id); got != "Alice Aaron,Ivy Invited" {
		t.Fatalf("after Ivy registers = %q, want Alice Aaron,Ivy Invited", got)
	}
	f.execSQL(`UPDATE "user" SET is_active = false WHERE id = $1`, crScopeUserAlice)
	if got := names(id); got != "Ivy Invited" {
		t.Fatalf("after deactivating Alice's user = %q, want Ivy Invited", got)
	}
}

func (f *crFlow) execSQL(sql string, args ...any) {
	f.t.Helper()
	if _, err := f.scoped.Exec(f.sys, sql, args...); err != nil {
		f.t.Fatalf("exec %.60s: %v", sql, err)
	}
}

// The scope fields survive the whole lifecycle: create with project,
// deployments, group and journal entries; Request Approval, peer and CAB
// approval, Implement, Review, Closed -- unchanged after every step, with the
// edit window closing at Implement.
func TestChangeRequestScopeIntegration_SurvivesNormalLifecycle(t *testing.T) {
	f := newCRFlow(t)
	f.seedAssignedGroup()
	f.seedScope()
	seedApprovalGroupMembers(t, f.scoped, crCABGroupID, crCABMemberUserID1, crCABMemberUserID2)
	typ := domain.ChangeRequestTypeNormal
	group := crFlowGroupID
	resp, err := f.repo.CreateChangeRequest(f.sys, domain.CreateChangeRequestRequest{
		Subject: crFlowSubject, Type: &typ, GroupID: &group,
		ProjectID: scopeStrp(crScopeProjectA), DeploymentIDs: []string{crScopeDepProd, crScopeDepStage},
		Comment: scopeStrp("lifecycle comment"), WorkNote: scopeStrp("lifecycle note"),
	}, crFlowEmail(crFlowCreatorID))
	if err != nil {
		t.Fatalf("CreateChangeRequest: %v", err)
	}
	id := resp.ChangeRequest.ID
	if _, err := f.scoped.Exec(f.sys, `UPDATE change_request SET requested_by_user_id = $1::uuid WHERE id = $2`, crFlowCreatorID, id); err != nil {
		t.Fatalf("set requested_by: %v", err)
	}
	deps := []string{crScopeDepProd, crScopeDepStage}
	prods := []string{crScopeDPProdOne, crScopeDPProdTwo, crScopeDPStageOne}
	check := func(when string) {
		t.Helper()
		cr := f.get(id)
		f.assertScope(when, cr, crScopeProjectA, deps, prods)
		if got := contactNames(cr.CustomerContacts); strings.Join(got, ",") != "Alice Aaron,Bob Bell" {
			t.Fatalf("%s: customerContacts = %v", when, got)
		}
		if c := f.comments(id); len(c["COMMENT"]) < 1 || len(c["WORK_NOTE"]) < 1 || c["COMMENT"][0] != "lifecycle comment|"+crFlowEmail(crFlowCreatorID) {
			t.Fatalf("%s: journal = %v", when, c)
		}
	}
	check("after create")

	// Editable while New: narrow, then swap back.
	f.mustPatch(id, domain.PatchChangeRequestRequest{DeploymentIDs: &[]string{crScopeDepProd}})
	f.assertScope("narrowed while New", f.get(id), crScopeProjectA, []string{crScopeDepProd}, []string{crScopeDPProdOne, crScopeDPProdTwo})
	f.mustPatch(id, domain.PatchChangeRequestRequest{DeploymentIDs: &deps})
	check("after narrowing and back")

	f.requestApproval(id)
	check("after Request Approval")
	if err := f.decide(id, crFlowPeerAID, "approved"); err != nil {
		t.Fatalf("peer approval: %v", err)
	}
	check("after peer approval")
	if err := f.decide(id, crCABMemberUserID1, "approved"); err != nil {
		t.Fatalf("CAB approval: %v", err)
	}
	if got := f.state(id); got != "SCHEDULED" {
		t.Fatalf("state = %s, want SCHEDULED", got)
	}
	check("after CAB approval")
	// Still editable while Scheduled.
	f.mustPatch(id, domain.PatchChangeRequestRequest{DeploymentIDs: &[]string{crScopeDepProd}})
	f.mustPatch(id, domain.PatchChangeRequestRequest{DeploymentIDs: &deps})
	for _, step := range []domain.ChangeRequestState{domain.ChangeRequestStateImplement, domain.ChangeRequestStateReview, domain.ChangeRequestStateClosed} {
		step := step
		f.mustPatch(id, domain.PatchChangeRequestRequest{State: &step})
		check("after " + string(step))
		if step == domain.ChangeRequestStateImplement {
			_, err := f.patch(id, domain.PatchChangeRequestRequest{DeploymentIDs: &[]string{crScopeDepProd}})
			f.wantValidationError("edit after implement", err, "can no longer be changed")
			check("after refused edit")
		}
	}
}

// The form's lookup: a project's active deployments (with their type);
// the project's registered contacts (the read-only Customer Group); the
// read-only deployment products that follow from the
// chosen deployments; the same validation errors as create.
func TestChangeRequestScopeIntegration_LinkOptions(t *testing.T) {
	f := newCRFlow(t)
	f.seedScope()

	opts, err := f.repo.GetChangeRequestLinkOptions(f.sys, domain.ChangeRequestLinkOptionsRequest{ProjectID: crScopeProjectA})
	if err != nil {
		t.Fatalf("GetChangeRequestLinkOptions: %v", err)
	}
	var depNames []string
	for _, d := range opts.Deployments {
		depNames = append(depNames, d.Name+"/"+d.Type)
	}
	if want := "Scope Dev/development,Scope Prod/primary_production,Scope Stage/staging,Scope Stage 2/staging"; strings.Join(depNames, ",") != want {
		t.Fatalf("deployments = %v, want %s (active only, name order)", depNames, want)
	}
	if opts.DeploymentProducts == nil || len(opts.DeploymentProducts) != 0 {
		t.Fatalf("without chosen deployments: products = %v, want an empty array", opts.DeploymentProducts)
	}
	// The read-only Customer Group of the project, name order: the registered
	// portal-user contacts of project A only.
	if got := contactNames(opts.CustomerContacts); strings.Join(got, ",") != "Alice Aaron,Bob Bell" {
		t.Fatalf("customerContacts = %v, want project A's Alice Aaron,Bob Bell", got)
	}
	if opts.CustomerContacts[0].ID == "" || opts.CustomerContacts[0].Email == "" {
		t.Fatalf("contact = %+v, want id and email", opts.CustomerContacts[0])
	}
	for project, want := range map[string]string{crScopeProjectB: "Carol Cook", crScopeProjectC: ""} {
		o, err := f.repo.GetChangeRequestLinkOptions(f.sys, domain.ChangeRequestLinkOptionsRequest{ProjectID: project})
		if err != nil {
			t.Fatalf("GetChangeRequestLinkOptions(%s): %v", project, err)
		}
		if o.CustomerContacts == nil || strings.Join(contactNames(o.CustomerContacts), ",") != want {
			t.Fatalf("project %s customerContacts = %v, want %q (never another customer's)", project, o.CustomerContacts, want)
		}
	}

	opts, err = f.repo.GetChangeRequestLinkOptions(f.sys, domain.ChangeRequestLinkOptionsRequest{ProjectID: crScopeProjectA, DeploymentIDs: []string{crScopeDepProd, crScopeDepStage, crScopeDepStage2}})
	if err != nil {
		t.Fatalf("GetChangeRequestLinkOptions(chosen): %v", err)
	}
	var prods []string
	for _, p := range opts.DeploymentProducts {
		prods = append(prods, p.ID+"@"+p.Deployment.ID)
	}
	sort.Strings(prods)
	want := []string{crScopeDPProdOne + "@" + crScopeDepProd, crScopeDPProdTwo + "@" + crScopeDepProd, crScopeDPStageOne + "@" + crScopeDepStage}
	sort.Strings(want)
	if strings.Join(prods, ",") != strings.Join(want, ",") {
		t.Fatalf("deploymentProducts = %v, want %v (deactivated product excluded, each with its deployment)", prods, want)
	}
	// What the lookup offers is exactly what create accepts.
	id := f.mustCreateScoped(func(r *domain.CreateChangeRequestRequest) {
		r.ProjectID = scopeStrp(crScopeProjectA)
		r.DeploymentIDs = []string{crScopeDepProd, crScopeDepStage, crScopeDepStage2}
		for _, p := range opts.DeploymentProducts {
			r.DeploymentProductIDs = append(r.DeploymentProductIDs, p.ID)
		}
	})
	f.assertScope("created from the lookup's options", f.get(id), crScopeProjectA,
		[]string{crScopeDepProd, crScopeDepStage, crScopeDepStage2},
		[]string{crScopeDPProdOne, crScopeDPProdTwo, crScopeDPStageOne})

	_, err = f.repo.GetChangeRequestLinkOptions(f.sys, domain.ChangeRequestLinkOptionsRequest{ProjectID: crScopeProjectA, DeploymentIDs: []string{crScopeDepOtherB}})
	f.wantValidationError("foreign deployment", err, "does not belong to the selected project")
	_, err = f.repo.GetChangeRequestLinkOptions(f.sys, domain.ChangeRequestLinkOptionsRequest{ProjectID: "3bbbbbbb-9999-0000-0000-000000000000"})
	f.wantValidationError("unknown project", err, "projectId does not refer to an existing project")

	// Pre-flight validation (the ServiceNow-first create) writes nothing.
	set, err := f.repo.ValidateChangeRequestLinks(f.sys, domain.ChangeRequestLinkSelection{ProjectID: scopeStrp(crScopeProjectB), DeploymentIDs: []string{crScopeDepOtherB}})
	if err != nil || set.ProjectID != crScopeProjectB || len(set.Deployments) != 1 || len(set.DeploymentProducts) != 1 {
		t.Fatalf("ValidateChangeRequestLinks = %+v, %v", set, err)
	}
	_, err = f.repo.ValidateChangeRequestLinks(f.sys, domain.ChangeRequestLinkSelection{ProjectID: scopeStrp(crScopeProjectB), DeploymentIDs: []string{crScopeDepProd}})
	f.wantValidationError("pre-flight foreign deployment", err, "does not belong to the selected project")
}

// Deleting a deployment, a deployed product or the change request removes the
// join rows (ON DELETE CASCADE) and nothing else.
func TestChangeRequestScopeIntegration_JoinRowsCascade(t *testing.T) {
	f := newCRFlow(t)
	f.seedScope()
	id := f.mustCreateScoped(func(r *domain.CreateChangeRequestRequest) {
		r.ProjectID = scopeStrp(crScopeProjectA)
		r.DeploymentIDs = []string{crScopeDepProd, crScopeDepStage}
	})
	count := func(table string) int {
		var n int
		if err := f.scoped.QueryRow(f.sys, fmt.Sprintf(`SELECT count(*) FROM %s WHERE change_request_id = $1`, table), id).Scan(&n); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		return n
	}
	if count("change_request_deployment") != 2 || count("change_request_deployed_product") != 3 {
		t.Fatal("join rows not written as expected")
	}
	if _, err := f.scoped.Exec(f.sys, `DELETE FROM deployed_product WHERE id = $1`, crScopeDPStageOne); err != nil {
		t.Fatalf("delete deployed product: %v", err)
	}
	if count("change_request_deployed_product") != 2 {
		t.Fatalf("deployed-product rows after deleting one = %d, want 2", count("change_request_deployed_product"))
	}
	if _, err := f.scoped.Exec(f.sys, `DELETE FROM deployment WHERE id = $1`, crScopeDepStage); err != nil {
		t.Fatalf("delete deployment: %v", err)
	}
	f.assertScope("after deleting the stage deployment", f.get(id), crScopeProjectA, []string{crScopeDepProd}, []string{crScopeDPProdOne, crScopeDPProdTwo})
	if _, err := f.scoped.Exec(f.sys, `DELETE FROM work_item WHERE id = $1`, id); err != nil {
		t.Fatalf("delete work item: %v", err)
	}
	if count("change_request_deployment")+count("change_request_deployed_product") != 0 {
		t.Fatal("join rows survived the change request")
	}
}

// Migrations 0191 and 0192 are idempotent: re-running them (0191 then 0192, as
// a database that already has both would see on a replay) changes nothing, keeps
// the data, leaves the category enum with all 13 labels, and leaves no
// environment catalogue, no change_request_environment table and no
// project_customer_group table (0192 drops all three; the environment
// concept is gone because a deployment carries its type).
func TestChangeRequestScopeIntegration_MigrationIsIdempotent(t *testing.T) {
	f := newCRFlow(t)
	f.seedScope()
	id := f.mustCreateScoped(func(r *domain.CreateChangeRequestRequest) {
		r.ProjectID = scopeStrp(crScopeProjectA)
		r.DeploymentIDs = []string{crScopeDepProd}
	})
	exists := func(table string) bool {
		var ok bool
		if err := f.scoped.QueryRow(f.sys, `SELECT to_regclass($1) IS NOT NULL`, table).Scan(&ok); err != nil {
			t.Fatal(err)
		}
		return ok
	}
	for _, name := range []string{"0191_change_request_project_links.sql", "0192_change_request_drop_environments.sql"} {
		sqlBytes, err := os.ReadFile("../../migrations/" + name)
		if err != nil {
			t.Fatalf("read migration: %v", err)
		}
		for i := 0; i < 2; i++ {
			if _, err := f.pool.Exec(context.Background(), string(sqlBytes)); err != nil {
				t.Fatalf("re-running migration %s (pass %d): %v", name, i+1, err)
			}
		}
	}
	f.assertScope("after re-running the migrations", f.get(id), crScopeProjectA, []string{crScopeDepProd}, []string{crScopeDPProdOne, crScopeDPProdTwo})
	for _, table := range []string{"environment", "change_request_environment", "project_customer_group"} {
		if exists(table) {
			t.Fatalf("table %s still exists after migration 0192", table)
		}
	}
	for _, table := range []string{"change_request_deployment", "change_request_deployed_product"} {
		if !exists(table) {
			t.Fatalf("table %s is gone", table)
		}
	}
	var labels int
	if err := f.scoped.QueryRow(f.sys, `SELECT count(*) FROM pg_enum WHERE enumtypid = 'change_request_category_enum'::regtype`).Scan(&labels); err != nil {
		t.Fatal(err)
	}
	if labels != 13 {
		t.Fatalf("category labels = %d, want 13", labels)
	}
	var fks, idx int
	if err := f.scoped.QueryRow(f.sys, `SELECT count(*) FROM pg_constraint WHERE contype = 'f' AND conrelid IN ('change_request_deployment'::regclass, 'change_request_deployed_product'::regclass) AND confdeltype = 'c'`).Scan(&fks); err != nil {
		t.Fatal(err)
	}
	if err := f.scoped.QueryRow(f.sys, `SELECT count(*) FROM pg_indexes WHERE tablename IN ('change_request_deployment','change_request_deployed_product')`).Scan(&idx); err != nil {
		t.Fatal(err)
	}
	// want 4 FKs (2 cascading FKs per table, as this migration itself defines).
	// want 6 indexes, not the 4 this migration alone would create (a PK + a
	// lookup index per table): change_request_deployment is ALSO defined by
	// csm-sync-service's own 0136_change_request_deployment_table.sql, mirrored
	// verbatim into this repo and numbered to run first, so that surrogate-id
	// schema (its own pkey plus a UNIQUE(change_request_id, deployment_id))
	// is what actually exists by the time this migration's own
	// CREATE TABLE IF NOT EXISTS for the same name becomes a no-op -- two
	// extra indexes on that one table, none on change_request_deployed_product
	// (0137, this table's csm-sync-service namesake, never defines
	// deployed_product, so no collision there). See writeChangeRequestDeployments'
	// own doc comment (change_request_links.go) for the write-path half of
	// this collision.
	if fks != 4 || idx != 6 {
		t.Fatalf("cascading FKs = %d, indexes = %d, want 4 and 6", fks, idx)
	}
}

// ---------------------------------------------------------------------------
// Rollback: the failed-review off-ramp. Offered from exactly Review and
// Customer Review; terminal. (A customer-group member rejecting the Customer
// Review stage also lands here -- change_request_customer_group_integration_test.go.)
// ---------------------------------------------------------------------------

const rollbackOnlyFromReviewMsg = `state "rollback" can only be set from review or customer_review`

// requestedApprovers counts the REQUESTED approver rows across all of the
// change's stages.
func (f *crFlow) requestedApprovers(id string) int {
	f.t.Helper()
	var n int
	if err := f.scoped.QueryRow(f.sys,
		`SELECT COUNT(*) FROM approval_stage_approver WHERE work_item_id = $1 AND state = 'REQUESTED'`, id).Scan(&n); err != nil {
		f.t.Fatalf("count requested approvers: %v", err)
	}
	return n
}

// driveNormalToImplement takes a Normal change (no customer approval) through
// Request Approval, peer and CAB approval to Implement.
func (f *crFlow) driveNormalToImplement(id string) {
	f.t.Helper()
	f.requestApproval(id)
	f.approvePeerAndCAB(id, "SCHEDULED", "implement", "canceled")
	f.step(id, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
}

// wantRolledBack asserts the terminal outcome of a manual rollback: state
// ROLLBACK, nothing left to do, no review stamp, nothing left to approve, and
// no way out of it.
func (f *crFlow) wantRolledBack(id string) {
	f.t.Helper()
	f.expect(id, "after Roll back", "ROLLBACK")
	if _, reviewed := f.customerOutcome(id); reviewed {
		f.t.Fatal("is_customer_review_required = true after a rollback: the review failed")
	}
	if n := f.requestedApprovers(id); n != 0 {
		f.t.Fatalf("%d approver rows still REQUESTED after the rollback, want 0", n)
	}
	if cr := f.get(id); cr.LegalNextStates != nil {
		f.t.Fatalf("legalNextStates(Rollback) = %v, want none (terminal)", cr.LegalNextStates)
	}
	for _, to := range []domain.ChangeRequestState{
		domain.ChangeRequestStateNew, domain.ChangeRequestStateImplement, domain.ChangeRequestStateReview,
		domain.ChangeRequestStateCustomerReview, domain.ChangeRequestStateClosed, domain.ChangeRequestStateCanceled,
	} {
		_, err := f.patchState(id, to)
		f.wantValidationError("PATCH {state: "+string(to)+"} out of rollback", err, "rollback is final")
	}
	_, err := f.patchState(id, domain.ChangeRequestStateRollback)
	f.wantValidationError("PATCH {state: rollback} again", err, rollbackOnlyFromReviewMsg)
	f.expect(id, "after the refused moves out of rollback", "ROLLBACK")
}

// Normal, Customer Review unticked and ticked: ...-> Implement -> Review ->
// Roll back. State and legalNextStates after every step; the Review stage's
// requested approvers are cancelled by the rollback.
func TestChangeRequestFlowIntegration_NormalRollbackFromReview(t *testing.T) {
	for _, review := range []bool{false, true} {
		review := review
		t.Run(fmt.Sprintf("customerReviewRequired=%v", review), func(t *testing.T) {
			f := newCRFlow(t)
			f.seedAssignedGroup()
			seedApprovalGroupMembers(t, f.scoped, crCABGroupID, crCABMemberUserID1, crCABMemberUserID2)
			id := f.createGated(domain.ChangeRequestTypeNormal, crFlowGroupID, nil, boolp(review))
			f.expect(id, "after create", "NEW", "assess", "canceled")

			// Rollback is not available before Review.
			_, err := f.patchState(id, domain.ChangeRequestStateRollback)
			f.wantValidationError("rollback from New", err, rollbackOnlyFromReviewMsg)
			f.driveNormalToImplement(id)
			_, err = f.patchState(id, domain.ChangeRequestStateRollback)
			f.wantValidationError("rollback from Implement", err, rollbackOnlyFromReviewMsg)
			f.expect(id, "after the refused rollback from Implement", "IMPLEMENT", "review", "canceled")

			forward := "closed"
			if review {
				forward = "customer_review"
			}
			f.step(id, domain.ChangeRequestStateReview, "REVIEW", forward, "rollback", "canceled")
			if n := f.requestedApprovers(id); n == 0 {
				t.Fatal("the Review stage has no REQUESTED approvers before the rollback, so the cancellation below proves nothing")
			}

			// A rollback is a failed review: it cannot also record the review.
			_, err = f.patch(id, domain.PatchChangeRequestRequest{
				State: stateptr(domain.ChangeRequestStateRollback), IsCustomerReviewed: boolp(true)})
			f.wantValidationError("rollback with isCustomerReviewed", err, "isCustomerReviewed cannot be true when rolling back")
			f.expect(id, "after the refused rollback", "REVIEW", forward, "rollback", "canceled")

			f.step(id, domain.ChangeRequestStateRollback, "ROLLBACK")
			f.wantRolledBack(id)
			// The stages stay as a record: peer, CAB and Review, all settled.
			if got := f.labels(id); len(got) != 3 {
				t.Fatalf("stages after the rollback = %v, want the 3 existing stages and no new one", got)
			}
		})
	}
}

// Normal with Customer Review ticked and no customer group: ... -> Review ->
// Customer Review -> Roll back (the manual fallback path).
func TestChangeRequestFlowIntegration_NormalRollbackFromCustomerReview(t *testing.T) {
	f := newCRFlow(t)
	f.seedAssignedGroup()
	seedApprovalGroupMembers(t, f.scoped, crCABGroupID, crCABMemberUserID1, crCABMemberUserID2)
	id := f.createGated(domain.ChangeRequestTypeNormal, crFlowGroupID, nil, boolp(true))
	f.driveNormalToImplement(id)
	f.step(id, domain.ChangeRequestStateReview, "REVIEW", "customer_review", "rollback", "canceled")
	f.step(id, domain.ChangeRequestStateCustomerReview, "CUSTOMER_REVIEW", "closed", "rollback", "canceled")
	if _, reviewed := f.customerOutcome(id); reviewed {
		t.Fatal("is_customer_review_required is already true before the customer review was recorded")
	}
	f.step(id, domain.ChangeRequestStateRollback, "ROLLBACK")
	f.wantRolledBack(id)
}

// Rollback is refused, with the exact message and nothing changed, from every
// state other than Review and Customer Review (a state-less row included).
func TestChangeRequestFlowIntegration_RollbackRefusedFromEveryOtherState(t *testing.T) {
	f := newCRFlow(t)
	f.seedAssignedGroup()
	id := f.create(domain.ChangeRequestTypeNormal, crFlowGroupID)
	for _, st := range []string{"NEW", "ASSESS", "AUTHORIZE", "CUSTOMER_APPROVAL", "SCHEDULED", "IMPLEMENT", "CLOSED", "CANCELED", "ROLLBACK", ""} {
		var seed any = st
		if st == "" {
			seed = nil
		}
		if _, err := f.scoped.Exec(f.sys, `UPDATE change_request SET state = $2::change_request_state_enum WHERE id = $1`, id, seed); err != nil {
			t.Fatalf("seed state %q: %v", st, err)
		}
		_, err := f.patchState(id, domain.ChangeRequestStateRollback)
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) || ve.Msg != rollbackOnlyFromReviewMsg {
			t.Fatalf("PATCH {state: rollback} from %q: err = %v, want a 400 %q", st, err, rollbackOnlyFromReviewMsg)
		}
		if got := f.state(id); got != st {
			t.Fatalf("state after the refused rollback from %q = %q, want unchanged", st, got)
		}
		for _, next := range f.legal(id) {
			if next == "rollback" {
				t.Fatalf("legalNextStates(%q) offers rollback: %v", st, f.legal(id))
			}
		}
	}
}

// The on-hold gate applies: a change on hold cannot be rolled back until it is
// taken off hold -- which may happen in the same PATCH.
func TestChangeRequestFlowIntegration_RollbackRespectsOnHold(t *testing.T) {
	f := newCRFlow(t)
	f.seedAssignedGroup()
	seedApprovalGroupMembers(t, f.scoped, crCABGroupID, crCABMemberUserID1, crCABMemberUserID2)
	id := f.create(domain.ChangeRequestTypeNormal, crFlowGroupID)
	f.driveNormalToImplement(id)
	f.step(id, domain.ChangeRequestStateReview, "REVIEW", "closed", "rollback", "canceled")

	reason := "customer change freeze"
	if _, err := f.patch(id, domain.PatchChangeRequestRequest{OnHold: boolp(true), OnHoldReason: &reason}); err != nil {
		t.Fatalf("put on hold: %v", err)
	}
	_, err := f.patchState(id, domain.ChangeRequestStateRollback)
	f.wantValidationError("rollback while on hold", err, "change request is on hold")
	f.expect(id, "after the refused rollback", "REVIEW", "closed", "rollback", "canceled")
	if n := f.requestedApprovers(id); n == 0 {
		t.Fatal("the refused rollback cancelled approver rows")
	}

	if _, err := f.patch(id, domain.PatchChangeRequestRequest{State: stateptr(domain.ChangeRequestStateRollback), OnHold: boolp(false)}); err != nil {
		t.Fatalf("PATCH {state: rollback, onHold: false}: %v", err)
	}
	f.wantRolledBack(id)
}

func stateptr(s domain.ChangeRequestState) *domain.ChangeRequestState { return &s }

// ---------------------------------------------------------------------------
// The Customer Group is the Customer Project's registered contacts, read-only:
// no group is picked or stored, the detail exposes customerContacts, and
// customerGroupId / environmentIds are refused.
// ---------------------------------------------------------------------------

const (
	crScopeMsgGroupRemoved = "customerGroupId is no longer accepted: the customer group is derived from the customer project's registered contacts"
	crScopeMsgEnvRemoved   = "environmentIds is no longer supported: deployments carry the environment"
)

func (f *crFlow) storedCustomerGroup(id string) *string {
	f.t.Helper()
	var g *string
	if err := f.scoped.QueryRow(f.sys, `SELECT customer_group_id::text FROM change_request WHERE id = $1`, id).Scan(&g); err != nil {
		f.t.Fatalf("read stored customer group: %v", err)
	}
	return g
}

// wantValidationMessage is wantValidationError with the message compared exactly.
func (f *crFlow) wantValidationMessage(what string, err error, want string) {
	f.t.Helper()
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		f.t.Fatalf("%s: err = %v (%T), want *apierror.ValidationError", what, err, err)
	}
	if ve.Msg != want {
		f.t.Fatalf("%s: message = %q, want %q", what, ve.Msg, want)
	}
}

// Create refuses the removed fields on every path (portal insert, ServiceNow-
// first insert, pre-flight) and leaves nothing behind.
func TestChangeRequestScopeIntegration_CreateRefusesRemovedFields(t *testing.T) {
	f := newCRFlow(t)
	f.seedScope()
	for name, tc := range map[string]struct {
		mod  func(*domain.CreateChangeRequestRequest)
		want string
	}{
		"customerGroupId": {func(r *domain.CreateChangeRequestRequest) {
			r.ProjectID, r.CustomerGroupID = scopeStrp(crScopeProjectA), scopeStrp(seededGroupID)
		}, crScopeMsgGroupRemoved},
		"customerGroupId without a project": {func(r *domain.CreateChangeRequestRequest) {
			r.CustomerGroupID = scopeStrp(seededGroupID)
		}, crScopeMsgGroupRemoved},
		"environmentIds": {func(r *domain.CreateChangeRequestRequest) {
			r.ProjectID, r.EnvironmentIDs = scopeStrp(crScopeProjectA), []string{}
		}, crScopeMsgEnvRemoved},
	} {
		_, err := f.createScoped(tc.mod)
		f.wantValidationMessage(name+" (portal)", err, tc.want)
		typ := domain.ChangeRequestTypeNormal
		req := domain.CreateChangeRequestRequest{Subject: crFlowSubject, Type: &typ}
		tc.mod(&req)
		_, err = f.repo.CreateChangeRequestFromServiceNow(f.sys, req, "3bbbbbbb-0000-0000-0000-0000000000f2", "CRSCOPESN02", crFlowEmail(crFlowCreatorID))
		f.wantValidationMessage(name+" (ServiceNow-first insert)", err, tc.want)
	}
	if n := f.crCount(); n != 0 {
		t.Fatalf("%d change request(s) left behind by refused creates", n)
	}
}

// A legacy change request that still has a stored customer_group_id (from
// before the group was derived) reads fine: customerContacts come from the
// project, the stored column is ignored, and an unrelated edit works.
func TestChangeRequestScopeIntegration_LegacyStoredGroupIsIgnored(t *testing.T) {
	f := newCRFlow(t)
	f.seedScope()
	id := f.mustCreateScoped(func(r *domain.CreateChangeRequestRequest) { r.ProjectID = scopeStrp(crScopeProjectB) })
	f.execSQL(`UPDATE change_request SET customer_group_id = $1 WHERE id = $2`, seededGroupID, id)
	if got := contactNames(f.get(id).CustomerContacts); strings.Join(got, ",") != "Carol Cook" {
		t.Fatalf("customerContacts = %v, want project B's Carol Cook regardless of the stored group", got)
	}
	title := "edited"
	f.mustPatch(id, domain.PatchChangeRequestRequest{Title: &title})
	if g := f.storedCustomerGroup(id); g == nil || *g != seededGroupID {
		t.Fatalf("the legacy column was rewritten: %v", g)
	}
}

// ---------------------------------------------------------------------------
// Re-schedule: the process diagram's Time Change loop. In Customer Approval,
// PATCH {state: "authorize", plannedStartOn/plannedEndOn} with a changed window
// sends the change back through internal approval (Normal: a fresh CAB stage,
// Emergency: a fresh ECAB stage), supersedes the customer's pending request and,
// once the new approval is given, asks the customer again. Standard has no
// internal approval to repeat: dates applied, stays in Customer Approval, the
// customer is asked again.
// ---------------------------------------------------------------------------

const (
	rsStart1 = "2030-03-01T09:00:00Z"
	rsEnd1   = "2030-03-01T11:00:00Z"
	rsStart2 = "2030-03-08T09:00:00Z"
	rsEnd2   = "2030-03-08T11:00:00Z"
	rsStart3 = "2030-03-15T09:00:00Z"
	// an earlier start inside the first window, for start-only changes
	rsStartEarly = "2030-03-01T08:00:00Z"
	rsEnd3       = "2030-03-15T11:00:00Z"

	rescheduleOnlyFromCustomerApprovalMsg = `state "authorize" cannot be set manually: it is reached automatically through the approval flow (Request Approval, then peer approval); it can only be set by hand to re-schedule a change from customer_approval`
)

// setPlanned stores a planned window directly (the create path under test is
// not this file's concern).
func (f *crFlow) setPlanned(id, start, end string) {
	f.t.Helper()
	if _, err := f.scoped.Exec(f.sys,
		`UPDATE change_request SET start_on = $2::text::timestamptz, end_on = $3::text::timestamptz WHERE id = $1`, id, start, end); err != nil {
		f.t.Fatalf("set planned window: %v", err)
	}
}

func (f *crFlow) wantPlanned(id, when, start, end string) {
	f.t.Helper()
	cr := f.get(id)
	if cr.PlannedStartOn == nil || cr.PlannedEndOn == nil || *cr.PlannedStartOn != start || *cr.PlannedEndOn != end {
		f.t.Fatalf("planned window %s = %v .. %v, want %s .. %s", when, cr.PlannedStartOn, cr.PlannedEndOn, start, end)
	}
}

// reschedule is the Re-schedule action: {state: authorize} with the new window.
func (f *crFlow) reschedule(id string, start, end *string) error {
	_, err := f.patch(id, domain.PatchChangeRequestRequest{State: stateptr(domain.ChangeRequestStateAuthorize), PlannedStartOn: start, PlannedEndOn: end})
	return err
}

// stageLabels lists the labels of the change's stages in creation order.
func (f *crFlow) stageLabels(id string) string {
	f.t.Helper()
	var out []string
	for _, st := range f.stages(id) {
		out = append(out, st.label)
	}
	return strings.Join(out, ",")
}

// liveStageRows counts REQUESTED approver rows on stages of the given label.
func (f *crFlow) liveStageRows(id, label string) int {
	f.t.Helper()
	n := 0
	for _, st := range f.stages(id) {
		if st.label != label {
			continue
		}
		for _, status := range st.approvers {
			if status == "REQUESTED" {
				n++
			}
		}
	}
	return n
}

// Normal, Customer Approval ticked, a customer group with two members: ->
// Customer Approval (live stage) -> Re-schedule -> Authorize (fresh CAB stage,
// customer stage cancelled) -> CAB approves -> Customer Approval (fresh customer
// stage) -> a member approves -> Scheduled -> Implement -> Review. State,
// legalNextStates and stages after every step.
func TestChangeRequestFlowIntegration_RescheduleNormalWithCustomerGroup(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
	f.setPlanned(id, rsStart1, rsEnd1)
	f.driveToCustomerApproval(id)
	f.expect(id, "in Customer Approval", "CUSTOMER_APPROVAL", "authorize", "canceled") // live stage: no manual scheduled
	if got := f.stageLabels(id); got != "Peer Approval,CAB Approval,Customer Approval" {
		t.Fatalf("stages in Customer Approval = %s", got)
	}

	// "Time Change = No": no new window, or the stored one, is refused.
	for what, args := range map[string][2]*string{
		"no window at all":  {nil, nil},
		"the stored start":  {sp(rsStart1), nil},
		"the stored window": {sp(rsStart1), sp(rsEnd1)},
		"the same instant":  {sp("2030-03-01T10:00:00+01:00"), nil},
	} {
		f.wantValidationError("re-schedule with "+what, f.reschedule(id, args[0], args[1]), "re-scheduling requires a changed planned start or end")
	}
	f.wantValidationError("re-schedule ending before it starts", f.reschedule(id, sp(rsStart3), sp(rsEnd1)), "planned start must not be after the planned end")
	f.wantValidationError("re-schedule with a garbage date", f.reschedule(id, sp("next tuesday"), nil), "valid date-times")
	f.expect(id, "after the refused re-schedules", "CUSTOMER_APPROVAL", "authorize", "canceled")
	f.wantPlanned(id, "after the refused re-schedules", rsStart1, rsEnd1)
	if got := f.stageLabels(id); got != "Peer Approval,CAB Approval,Customer Approval" {
		t.Fatalf("a refused re-schedule changed the stages: %s", got)
	}

	// Re-schedule: back to Authorize with the new window and a fresh CAB stage.
	if err := f.reschedule(id, sp(rsStart2), sp(rsEnd2)); err != nil {
		t.Fatalf("re-schedule: %v", err)
	}
	f.expect(id, "after Re-schedule", "AUTHORIZE", "canceled")
	f.wantPlanned(id, "after Re-schedule", rsStart2, rsEnd2)
	if got := f.stageLabels(id); got != "Peer Approval,CAB Approval,Customer Approval,CAB Approval" {
		t.Fatalf("stages after Re-schedule = %s, want a fresh CAB stage after the customer's", got)
	}
	stages := f.stages(id)
	assertApprovers(t, "customer stage after Re-schedule", stages[2].approvers, map[string]string{crScopeUserA1: "CANCELLED", crScopeUserA2: "CANCELLED"})
	assertApprovers(t, "fresh CAB stage", stages[3].approvers, map[string]string{crCABMemberUserID1: "REQUESTED", crCABMemberUserID2: "REQUESTED"})
	if stages[3].groupID != crCABGroupID {
		t.Fatalf("fresh CAB stage group = %s, want the CAB group", stages[3].groupID)
	}
	assertApprovers(t, "first CAB stage stays as a record", stages[1].approvers, map[string]string{crCABMemberUserID1: "APPROVED", crCABMemberUserID2: "CANCELLED"})
	if n := f.liveStageRows(id, "Peer Approval"); n != 0 {
		t.Fatalf("%d peer approver rows requested again; the peer approval stands", n)
	}
	if approved, _ := f.customerOutcome(id); approved {
		t.Fatal("is_customer_approval_required stamped by a re-schedule")
	}

	// Authorize is an approval wait again: no second Re-schedule, no manual way on.
	f.wantValidationError("re-schedule from Authorize", f.reschedule(id, sp(rsStart3), nil), rescheduleOnlyFromCustomerApprovalMsg)
	if _, err := f.patchState(id, domain.ChangeRequestStateScheduled); err == nil {
		t.Fatal("manual {state: scheduled} from Authorize succeeded")
	}
	// A customer member cannot answer the superseded request, nor the CAB's.
	if err := f.decide(id, crScopeUserA1, "approved"); err == nil {
		t.Fatal("a customer member decided although their request was superseded")
	}
	f.expect(id, "after the refused decision", "AUTHORIZE", "canceled")

	// The new CAB approval asks the customer again, with a fresh stage.
	if err := f.decide(id, crCABMemberUserID2, "approved"); err != nil {
		t.Fatalf("CAB approval after Re-schedule: %v", err)
	}
	f.expect(id, "after the new CAB approval", "CUSTOMER_APPROVAL", "authorize", "canceled")
	if got := f.stageLabels(id); got != "Peer Approval,CAB Approval,Customer Approval,CAB Approval,Customer Approval" {
		t.Fatalf("stages after the new CAB approval = %s", got)
	}
	stages = f.stages(id)
	assertApprovers(t, "first customer stage", stages[2].approvers, map[string]string{crScopeUserA1: "CANCELLED", crScopeUserA2: "CANCELLED"})
	assertApprovers(t, "fresh customer stage", stages[4].approvers, map[string]string{crScopeUserA1: "REQUESTED", crScopeUserA2: "REQUESTED"})
	assertApprovers(t, "new CAB stage", stages[3].approvers, map[string]string{crCABMemberUserID1: "CANCELLED", crCABMemberUserID2: "APPROVED"})
	f.wantPlanned(id, "back in Customer Approval", rsStart2, rsEnd2)

	// A member approves: Scheduled, customer approval recorded; the tail runs and
	// the Review stage is still provisioned (the repeated CAB stage is not a new checkpoint).
	if err := f.decide(id, crScopeUserA2, "approved"); err != nil {
		t.Fatalf("customer approval: %v", err)
	}
	f.expect(id, "after the customer's approval", "SCHEDULED", "implement", "canceled")
	if approved, _ := f.customerOutcome(id); !approved {
		t.Fatal("is_customer_approval_required not stamped by the member's approval")
	}
	f.wantValidationError("re-schedule from Scheduled", f.reschedule(id, sp(rsStart3), nil), rescheduleOnlyFromCustomerApprovalMsg)
	f.step(id, domain.ChangeRequestStateImplement, "IMPLEMENT", "review", "canceled")
	f.step(id, domain.ChangeRequestStateReview, "REVIEW", "closed", "rollback", "canceled")
	if got := f.stageLabels(id); got != "Peer Approval,CAB Approval,Customer Approval,CAB Approval,Customer Approval,Review" {
		t.Fatalf("stages in Review = %s, want the Review stage after a re-schedule too", got)
	}
}

// Normal, Customer Approval ticked, NO customer group (manual fallback), the
// creator also sits in the CAB group: Re-schedule twice. The creator never gets a
// decidable row on a new CAB stage; rejecting the new stage behaves as a CAB
// rejection does (the change stays in Authorize).
func TestChangeRequestFlowIntegration_RescheduleManualFallbackTwice(t *testing.T) {
	f := newCRFlow(t)
	f.seedAssignedGroup()
	seedApprovalGroupMembers(t, f.scoped, crCABGroupID, crCABMemberUserID1, crCABMemberUserID2)
	// The creator is a CAB member as well.
	if _, err := f.scoped.Exec(f.sys,
		`INSERT INTO team_member (id, created_on, updated_on, created_by, updated_by, team_id, user_id, group_id)
		 VALUES (gen_random_uuid(), now(), now(), 'cr-flow-test', 'cr-flow-test', $1::uuid, $2, $3::uuid)`,
		seededGroupID, crFlowCreatorID, crCABGroupID); err != nil {
		t.Fatalf("put the creator in the CAB group: %v", err)
	}
	t.Cleanup(func() {
		_, _ = f.scoped.Exec(f.sys, `DELETE FROM team_member WHERE user_id = $1 AND group_id = $2::uuid`, crFlowCreatorID, crCABGroupID)
	})
	id := f.createGated(domain.ChangeRequestTypeNormal, crFlowGroupID, boolp(true), nil)
	f.setPlanned(id, rsStart1, rsEnd1)
	f.requestApproval(id)
	f.approvePeerAndCAB(id, "CUSTOMER_APPROVAL", "scheduled", "authorize", "canceled")

	// Re-schedule #1.
	if err := f.reschedule(id, sp(rsStartEarly), nil); err != nil { // only the start changes
		t.Fatalf("re-schedule #1: %v", err)
	}
	f.expect(id, "after Re-schedule #1", "AUTHORIZE", "canceled")
	f.wantPlanned(id, "after Re-schedule #1", rsStartEarly, rsEnd1)
	if got := f.stageLabels(id); got != "Peer Approval,CAB Approval,CAB Approval" {
		t.Fatalf("stages after Re-schedule #1 = %s (no customer stage without a group)", got)
	}
	stages := f.stages(id)
	assertApprovers(t, "fresh CAB stage", stages[2].approvers, map[string]string{
		crCABMemberUserID1: "REQUESTED", crCABMemberUserID2: "REQUESTED", crFlowCreatorID: "CANCELLED"})
	var fe *apierror.ForbiddenError
	if err := f.decide(id, crFlowCreatorID, "approved"); !errors.As(err, &fe) {
		t.Fatalf("creator deciding the fresh CAB stage: err = %v, want ForbiddenError", err)
	}

	// A CAB rejection of the new stage changes nothing about the state (as today).
	if err := f.decide(id, crCABMemberUserID1, "rejected"); err != nil {
		t.Fatalf("CAB rejection: %v", err)
	}
	f.expect(id, "after the new stage was rejected", "AUTHORIZE", "canceled")
	if n := f.requestedApprovers(id); n != 0 {
		t.Fatalf("%d rows still requested after the rejection resolved the stage", n)
	}

	// Start over on a second change to approve the second round and loop again.
	id = f.createGated(domain.ChangeRequestTypeNormal, crFlowGroupID, boolp(true), nil)
	f.setPlanned(id, rsStart1, rsEnd1)
	f.requestApproval(id)
	f.approvePeerAndCAB(id, "CUSTOMER_APPROVAL", "scheduled", "authorize", "canceled")
	for round, w := range [][2]string{{rsStart2, rsEnd2}, {rsStart3, rsEnd3}} {
		if err := f.reschedule(id, sp(w[0]), sp(w[1])); err != nil {
			t.Fatalf("re-schedule round %d: %v", round+1, err)
		}
		f.expect(id, "after re-schedule round", "AUTHORIZE", "canceled")
		f.wantPlanned(id, "after re-schedule round", w[0], w[1])
		if err := f.decide(id, crCABMemberUserID1, "approved"); err != nil {
			t.Fatalf("CAB approval round %d: %v", round+1, err)
		}
		f.expect(id, "back in Customer Approval", "CUSTOMER_APPROVAL", "scheduled", "authorize", "canceled")
	}
	if got := f.stageLabels(id); got != "Peer Approval,CAB Approval,CAB Approval,CAB Approval" {
		t.Fatalf("stages after two re-schedules = %s", got)
	}
	f.step(id, domain.ChangeRequestStateScheduled, "SCHEDULED", "implement", "canceled")
	if approved, _ := f.customerOutcome(id); !approved {
		t.Fatal("manual Record customer approval did not stamp is_customer_approval_required")
	}
}

// Emergency with Customer Approval ticked and a customer group: a fresh ECAB stage.
func TestChangeRequestFlowIntegration_RescheduleEmergency(t *testing.T) {
	f := newCustomerGroupFlow(t)
	seedApprovalGroupMembers(t, f.scoped, crECABGroupID, crECABMemberUserID)
	id := f.createWithProject(domain.ChangeRequestTypeEmergency, sp(crScopeProjectA), true, false)
	f.setPlanned(id, rsStart1, rsEnd1)
	f.requestApproval(id)
	f.expect(id, "after Request Approval", "AUTHORIZE", "canceled")
	if err := f.decide(id, crECABMemberUserID, "approved"); err != nil {
		t.Fatalf("ECAB approval: %v", err)
	}
	f.expect(id, "after ECAB approval", "CUSTOMER_APPROVAL", "authorize", "canceled")

	if err := f.reschedule(id, nil, sp(rsEnd2)); err != nil { // only the end changes
		t.Fatalf("re-schedule: %v", err)
	}
	f.expect(id, "after Re-schedule", "AUTHORIZE", "canceled")
	f.wantPlanned(id, "after Re-schedule", rsStart1, rsEnd2)
	if got := f.stageLabels(id); got != "ECAB Approval,Customer Approval,ECAB Approval" {
		t.Fatalf("emergency stages after Re-schedule = %s", got)
	}
	stages := f.stages(id)
	assertApprovers(t, "fresh ECAB stage", stages[2].approvers, map[string]string{crECABMemberUserID: "REQUESTED"})
	if stages[2].groupID != crECABGroupID {
		t.Fatalf("fresh ECAB stage group = %s, want the ECAB group", stages[2].groupID)
	}
	assertApprovers(t, "superseded customer stage", stages[1].approvers, map[string]string{crScopeUserA1: "CANCELLED", crScopeUserA2: "CANCELLED"})

	if err := f.decide(id, crECABMemberUserID, "approved"); err != nil {
		t.Fatalf("new ECAB approval: %v", err)
	}
	f.expect(id, "after the new ECAB approval", "CUSTOMER_APPROVAL", "authorize", "canceled")
	if got := f.stageLabels(id); got != "ECAB Approval,Customer Approval,ECAB Approval,Customer Approval" {
		t.Fatalf("emergency stages after the new ECAB approval = %s", got)
	}
	if err := f.decide(id, crScopeUserA1, "approved"); err != nil {
		t.Fatalf("customer approval: %v", err)
	}
	f.expect(id, "after the customer's approval", "SCHEDULED", "implement", "canceled")
}

// Standard has no internal approval to repeat: Re-schedule applies the dates,
// stays in Customer Approval and asks the customer again (a fresh stage when the
// change has a customer group; the manual path stays without one).
func TestChangeRequestFlowIntegration_RescheduleStandard(t *testing.T) {
	t.Run("with a customer group", func(t *testing.T) {
		f := newCustomerGroupFlow(t)
		id := f.createWithProject(domain.ChangeRequestTypeStandard, sp(crScopeProjectA), true, false)
		f.setPlanned(id, rsStart1, rsEnd1)
		f.requestApproval(id)
		f.expect(id, "after Request Approval", "CUSTOMER_APPROVAL", "authorize", "canceled")
		f.wantValidationError("re-schedule without a change", f.reschedule(id, sp(rsStart1), sp(rsEnd1)), "re-scheduling requires a changed planned start or end")

		if err := f.reschedule(id, sp(rsStart2), sp(rsEnd2)); err != nil {
			t.Fatalf("re-schedule: %v", err)
		}
		f.expect(id, "after Re-schedule", "CUSTOMER_APPROVAL", "authorize", "canceled")
		f.wantPlanned(id, "after Re-schedule", rsStart2, rsEnd2)
		if got := f.stageLabels(id); got != "Customer Approval,Customer Approval" {
			t.Fatalf("stages after Re-schedule = %s, want the customer asked again with a fresh stage", got)
		}
		stages := f.stages(id)
		assertApprovers(t, "superseded customer stage", stages[0].approvers, map[string]string{crScopeUserA1: "CANCELLED", crScopeUserA2: "CANCELLED"})
		assertApprovers(t, "fresh customer stage", stages[1].approvers, map[string]string{crScopeUserA1: "REQUESTED", crScopeUserA2: "REQUESTED"})

		if err := f.decide(id, crScopeUserA1, "approved"); err != nil {
			t.Fatalf("customer approval: %v", err)
		}
		f.expect(id, "after the customer's approval", "SCHEDULED", "implement", "canceled")
	})
	t.Run("without a customer group", func(t *testing.T) {
		f := newCRFlow(t)
		f.seedAssignedGroup()
		id := f.createGated(domain.ChangeRequestTypeStandard, crFlowGroupID, boolp(true), nil)
		f.setPlanned(id, rsStart1, rsEnd1)
		f.requestApproval(id)
		f.expect(id, "after Request Approval", "CUSTOMER_APPROVAL", "scheduled", "authorize", "canceled")
		if err := f.reschedule(id, sp(rsStartEarly), nil); err != nil {
			t.Fatalf("re-schedule: %v", err)
		}
		f.expect(id, "after Re-schedule", "CUSTOMER_APPROVAL", "scheduled", "authorize", "canceled")
		f.wantPlanned(id, "after Re-schedule", rsStartEarly, rsEnd1)
		if n := len(f.stages(id)); n != 0 {
			t.Fatalf("standard change has %d stages after Re-schedule, want none", n)
		}
		f.step(id, domain.ChangeRequestStateScheduled, "SCHEDULED", "implement", "canceled")
	})
}

// Manual authorize is refused, with the exact message, from every state but
// Customer Approval -- nothing changes.
func TestChangeRequestFlowIntegration_RescheduleRefusedFromEveryOtherState(t *testing.T) {
	f := newCRFlow(t)
	f.seedAssignedGroup()
	id := f.create(domain.ChangeRequestTypeNormal, crFlowGroupID)
	f.setPlanned(id, rsStart1, rsEnd1)
	for _, st := range []string{"NEW", "ASSESS", "AUTHORIZE", "SCHEDULED", "IMPLEMENT", "REVIEW", "CUSTOMER_REVIEW", "CLOSED", "CANCELED", ""} {
		var seed any = st
		if st == "" {
			seed = nil
		}
		if _, err := f.scoped.Exec(f.sys, `UPDATE change_request SET state = $2::change_request_state_enum WHERE id = $1`, id, seed); err != nil {
			t.Fatalf("seed state %q: %v", st, err)
		}
		err := f.reschedule(id, sp(rsStart2), sp(rsEnd2))
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) || ve.Msg != rescheduleOnlyFromCustomerApprovalMsg {
			t.Fatalf("manual authorize from %q: err = %v, want a 400 %q", st, err, rescheduleOnlyFromCustomerApprovalMsg)
		}
		if got := f.state(id); got != st {
			t.Fatalf("state after the refused re-schedule from %q = %q", st, got)
		}
		f.wantPlanned(id, "after the refused re-schedule from "+st, rsStart1, rsEnd1)
	}
	// Rollback is final: the terminal refusal wins.
	if _, err := f.scoped.Exec(f.sys, `UPDATE change_request SET state = 'ROLLBACK' WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}
	f.wantValidationError("re-schedule from rollback", f.reschedule(id, sp(rsStart2), nil), "rollback is final")
}

// The on-hold gate applies, and an unsatisfiable re-schedule (nobody can give
// the new CAB approval) is refused as a whole -- window, state and the customer's
// pending request are untouched.
func TestChangeRequestFlowIntegration_RescheduleOnHoldAndAtomic(t *testing.T) {
	f := newCustomerGroupFlow(t)
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), true, false)
	f.setPlanned(id, rsStart1, rsEnd1)
	f.driveToCustomerApproval(id)

	reason := "customer freeze"
	if _, err := f.patch(id, domain.PatchChangeRequestRequest{OnHold: boolp(true), OnHoldReason: &reason}); err != nil {
		t.Fatalf("put on hold: %v", err)
	}
	f.wantValidationError("re-schedule while on hold", f.reschedule(id, sp(rsStart2), sp(rsEnd2)), "change request is on hold")
	f.expect(id, "after the refused re-schedule", "CUSTOMER_APPROVAL", "authorize", "canceled")
	f.wantPlanned(id, "while on hold", rsStart1, rsEnd1)
	if _, err := f.patch(id, domain.PatchChangeRequestRequest{OnHold: boolp(false)}); err != nil {
		t.Fatalf("take off hold: %v", err)
	}

	// The CAB group is emptied: the new approval could never be given.
	if _, err := f.scoped.Exec(f.sys, `DELETE FROM team_member WHERE group_id = $1::uuid`, crCABGroupID); err != nil {
		t.Fatalf("empty the CAB group: %v", err)
	}
	f.wantValidationError("re-schedule into an empty CAB group", f.reschedule(id, sp(rsStart2), sp(rsEnd2)), "no members")
	f.expect(id, "after the refused re-schedule", "CUSTOMER_APPROVAL", "authorize", "canceled")
	f.wantPlanned(id, "after the refused re-schedule", rsStart1, rsEnd1)
	if n := f.liveStageRows(id, "Customer Approval"); n != 2 {
		t.Fatalf("customer request has %d live rows after the refused re-schedule, want 2 (untouched)", n)
	}
	if got := f.stageLabels(id); got != "Peer Approval,CAB Approval,Customer Approval" {
		t.Fatalf("stages after the refused re-schedule = %s", got)
	}

	// Taking it off hold in the same PATCH works.
	seedApprovalGroupMembers(t, f.scoped, crCABGroupID, crCABMemberUserID1)
	if _, err := f.patch(id, domain.PatchChangeRequestRequest{OnHold: boolp(true)}); err != nil {
		t.Fatalf("put on hold again: %v", err)
	}
	if _, err := f.patch(id, domain.PatchChangeRequestRequest{
		State: stateptr(domain.ChangeRequestStateAuthorize), PlannedStartOn: sp(rsStartEarly), OnHold: boolp(false)}); err != nil {
		t.Fatalf("PATCH {state: authorize, plannedStartOn, onHold: false}: %v", err)
	}
	f.expect(id, "after re-scheduling and releasing the hold", "AUTHORIZE", "canceled")
}
