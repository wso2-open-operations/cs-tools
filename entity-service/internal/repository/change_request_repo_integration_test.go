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
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
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

	// changeRequestAuthorizeGateTestID is its own id, distinct from every
	// other test's change request above (including the Assess-gate tests),
	// so the Authorize-gate/approver-provisioning tests below never race any
	// of them over the same work_item row.
	changeRequestAuthorizeGateTestID = "36666666-0000-0000-0000-00000000000b"

	// changeRequestAuthorizeGateMemberUserID{,2} are seeded as team_member
	// rows against changeRequestAuthorizeGateGroupID (distinct from every
	// other test's own user ids) to exercise Authorize's auto-provisioned
	// approvers.
	changeRequestAuthorizeGateMemberUserID  = "36666666-0000-0000-0000-00000000000c"
	changeRequestAuthorizeGateMemberUserID2 = "36666666-0000-0000-0000-00000000000d"

	// changeRequestAuthorizeGateGroupID is its own dedicated "group" row,
	// deliberately not shared with changeRequestAssessGateGroupID or
	// seededGroupID -- same isolation reasoning as
	// changeRequestAssessGateGroupID's own doc comment.
	changeRequestAuthorizeGateGroupID = "36666666-0000-0000-0000-00000000000e"

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
	// is_customer_approved/is_customer_reviewed -- see
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
	          VALUES ($1, now(), now(), 'cr-approval-test', 'cr-approval-test', 'CRAPPRV01', 'approval guard test', 'CHANGE_REQUEST')`,
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
		mustExec(`INSERT INTO approval_stage_approver (id, created_on, updated_on, created_by, updated_by, stage_id, work_item_id, approver_user_id, status)
		          VALUES ($1, now(), now(), 'cr-approval-test', 'cr-approval-test', $2, $3, $4, 'requested')`,
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
// confirms the cascade is scoped exactly to Assess->Authorize: approving an
// approver on a change request that isn't currently in Assess (e.g. one
// already sitting in Authorize, mid its own separate approval stage) must
// leave state untouched -- this repository deliberately does not attempt
// Authorize's own outgoing cascade yet.
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
		t.Fatalf("state after approval outside Assess = %q, want unchanged \"AUTHORIZE\"", gotState)
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
	stageID := seedApprovalStageForDecisionTest(t, scoped,
		changeRequestApprovalApproverUserID, changeRequestApprovalApproverUserID2, changeRequestApprovalApproverUserID3)

	if _, err := repo.DecideChangeRequestApproval(sys, changeRequestApprovalTestID,
		changeRequestApprovalApproverUserID, "approved", "cr-approval-test"); err != nil {
		t.Fatalf("DecideChangeRequestApproval(approved): %v", err)
	}

	rows, err := scoped.Query(sys,
		`SELECT approver_user_id, status FROM approval_stage_approver WHERE stage_id = $1`, stageID)
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

	if got := statusByApprover[changeRequestApprovalApproverUserID]; got != "approved" {
		t.Errorf("acted-on approver status = %q, want \"approved\"", got)
	}
	if got := statusByApprover[changeRequestApprovalApproverUserID2]; got != "cancelled" {
		t.Errorf("sibling approver 2 status = %q, want \"cancelled\" (not left at \"requested\")", got)
	}
	if got := statusByApprover[changeRequestApprovalApproverUserID3]; got != "cancelled" {
		t.Errorf("sibling approver 3 status = %q, want \"cancelled\" (not left at \"requested\")", got)
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
		`SELECT approver_user_id, status FROM approval_stage_approver WHERE stage_id = $1`, stageID)
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

	if got := statusByApprover[changeRequestApprovalApproverUserID]; got != "rejected" {
		t.Errorf("acted-on approver status = %q, want \"rejected\"", got)
	}
	if got := statusByApprover[changeRequestApprovalApproverUserID2]; got != "cancelled" {
		t.Errorf("sibling approver 2 status = %q, want \"cancelled\" (not left at \"requested\")", got)
	}
	if got := statusByApprover[changeRequestApprovalApproverUserID3]; got != "cancelled" {
		t.Errorf("sibling approver 3 status = %q, want \"cancelled\" (not left at \"requested\")", got)
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
				`SELECT approver_user_id, status FROM approval_stage_approver WHERE stage_id = $1`, stageID)
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

			if got := statusByApprover[changeRequestApprovalApproverUserID]; got != "rejected" {
				t.Errorf("acted-on approver status = %q, want \"rejected\"", got)
			}
			if got := statusByApprover[changeRequestApprovalApproverUserID2]; got != "cancelled" {
				t.Errorf("sibling approver 2 status at %s = %q, want \"cancelled\"", tc.name, got)
			}
			if got := statusByApprover[changeRequestApprovalApproverUserID3]; got != "cancelled" {
				t.Errorf("sibling approver 3 status at %s = %q, want \"cancelled\"", tc.name, got)
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
		`UPDATE approval_stage_approver SET status = 'approved' WHERE stage_id = $1 AND approver_user_id = $2`,
		stageID, changeRequestApprovalApproverUserID2); err != nil {
		t.Fatalf("seed pre-existing approval: %v", err)
	}

	if _, err := repo.DecideChangeRequestApproval(sys, changeRequestApprovalTestID,
		changeRequestApprovalApproverUserID3, "rejected", "cr-approval-test"); err != nil {
		t.Fatalf("DecideChangeRequestApproval(rejected): %v", err)
	}

	rows, err := scoped.Query(sys,
		`SELECT approver_user_id, status FROM approval_stage_approver WHERE stage_id = $1`, stageID)
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

	if got := statusByApprover[changeRequestApprovalApproverUserID3]; got != "rejected" {
		t.Errorf("acted-on approver status = %q, want \"rejected\"", got)
	}
	if got := statusByApprover[changeRequestApprovalApproverUserID2]; got != "approved" {
		t.Errorf("pre-existing approved approver status = %q, want unchanged \"approved\"", got)
	}
	// The real assertion of this test: approver 1 was never decided by
	// anyone and is NOT a sibling of the rejection's own resolution (the
	// stage was already resolved, by the approval, before the rejection
	// ever ran) -- the hasApproval guard must have skipped cancellation
	// entirely, leaving it exactly as it was.
	if got := statusByApprover[changeRequestApprovalApproverUserID]; got != "requested" {
		t.Errorf("untouched sibling approver status = %q, want unchanged \"requested\" (an already-approved stage must not be disturbed by a later rejection)", got)
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
		 VALUES ($1, now(), now(), 'cr-assess-gate-test', 'cr-assess-gate-test', 'CRASSESS01', 'assess gate test', 'CHANGE_REQUEST')`,
		changeRequestAssessGateTestID); err != nil {
		t.Fatalf("seed work_item: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO change_request (id, state) VALUES ($1, 'NEW'::change_request_state_enum)`,
		changeRequestAssessGateTestID); err != nil {
		t.Fatalf("seed change_request: %v", err)
	}
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
		`SELECT approver_user_id::TEXT, status FROM approval_stage_approver WHERE stage_id = $1 ORDER BY approver_user_id`, stageID)
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
		changeRequestAssessGateMemberUserID:  "requested",
		changeRequestAssessGateMemberUserID2: "requested",
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
		`SELECT approver_user_id::TEXT, status FROM approval_stage_approver WHERE stage_id = $1`, stageID)
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
		changeRequestAssessGateMemberUserID:  "cancelled",
		changeRequestAssessGateMemberUserID2: "requested",
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

// seedChangeRequestForAuthorizeGateTest inserts a minimal work_item/
// change_request pair already sitting in Assess -- the realistic starting
// point for every Authorize-gate test below, since in production Authorize
// is only ever reached after Assess's own compulsory assigned-team gate has
// already run.
func seedChangeRequestForAuthorizeGateTest(t *testing.T, pool *repository.Scoped) {
	t.Helper()
	// work_item and change_request are RLS-protected; seed/cleanup as internal.
	ctx := repository.WithSystemIdentity(context.Background())

	cleanup := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM work_item WHERE id = $1`, changeRequestAuthorizeGateTestID)
	}
	cleanup()
	t.Cleanup(cleanup)

	if _, err := pool.Exec(ctx,
		`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, subject, type)
		 VALUES ($1, now(), now(), 'cr-authorize-gate-test', 'cr-authorize-gate-test', 'CRAUTH001', 'authorize gate test', 'CHANGE_REQUEST')`,
		changeRequestAuthorizeGateTestID); err != nil {
		t.Fatalf("seed work_item: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO change_request (id, state) VALUES ($1, 'ASSESS'::change_request_state_enum)`,
		changeRequestAuthorizeGateTestID); err != nil {
		t.Fatalf("seed change_request: %v", err)
	}
}

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

// seedAuthorizeGateGroup inserts changeRequestAuthorizeGateGroupID's own
// "group" row -- a dedicated fixture for the tests below, deliberately not
// shared with changeRequestAssessGateGroupID or seededGroupID (same
// isolation reasoning as changeRequestAssessGateGroupID's own doc comment).
func seedAuthorizeGateGroup(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()

	cleanup := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM "group" WHERE id = $1`, changeRequestAuthorizeGateGroupID)
	}
	cleanup()
	t.Cleanup(cleanup)

	if _, err := pool.Exec(ctx,
		`INSERT INTO "group" (id, created_on, updated_on, created_by, updated_by, name)
		 VALUES ($1, now(), now(), 'cr-authorize-gate-test', 'cr-authorize-gate-test', 'Authorize Gate Test Group')`,
		changeRequestAuthorizeGateGroupID); err != nil {
		t.Fatalf("seed group: %v", err)
	}
}

// seedTeamMembersForAuthorizeGateTest inserts one "user" row and one
// team_member row (keyed by group_id, same reasoning as
// seedTeamMembersForAssessGateTest) per given user id, so they resolve as
// members of changeRequestAuthorizeGateGroupID for the auto-provisioning
// tests below. Callers must seed that group row first (seedAuthorizeGateGroup).
func seedTeamMembersForAuthorizeGateTest(t *testing.T, pool *pgxpool.Pool, userIDs ...string) {
	t.Helper()
	ctx := context.Background()

	for i, userID := range userIDs {
		id := userID
		userCleanup := func() {
			_, _ = pool.Exec(ctx, `DELETE FROM "user" WHERE id = $1`, id)
		}
		userCleanup()
		t.Cleanup(userCleanup)

		email := fmt.Sprintf("cr-authorize-gate-member-%d@example.com", i+1)
		if _, err := pool.Exec(ctx,
			`INSERT INTO "user" (id, created_on, updated_on, created_by, updated_by, user_name, name, first_name, last_name, email, is_active, is_system_user)
			 VALUES ($1, now(), now(), 'cr-authorize-gate-test', 'cr-authorize-gate-test', $2, 'Authorize Gate Member', 'Authorize', 'Gate Member', $2, true, false)`,
			id, email); err != nil {
			t.Fatalf("seed team member user %s: %v", id, err)
		}

		memberCleanup := func() {
			_, _ = pool.Exec(ctx, `DELETE FROM team_member WHERE user_id = $1`, id)
		}
		memberCleanup()
		t.Cleanup(memberCleanup)

		// team_member.team_id is NOT NULL -- same seededGroupID-as-filler
		// reasoning as seedTeamMembersForAssessGateTest's own identical line.
		if _, err := pool.Exec(ctx,
			`INSERT INTO team_member (id, created_on, updated_on, created_by, updated_by, team_id, user_id, group_id)
			 VALUES (gen_random_uuid(), now(), now(), 'cr-authorize-gate-test', 'cr-authorize-gate-test', $1::uuid, $2, $3::uuid)`,
			seededGroupID, id, changeRequestAuthorizeGateGroupID); err != nil {
			t.Fatalf("seed team_member for user %s: %v", id, err)
		}
	}
}

// setAuthorizeGateRequestedBy stamps change_request.requested_by_user_id for
// changeRequestAuthorizeGateTestID, mirroring setAssessGateRequestedBy.
func setAuthorizeGateRequestedBy(t *testing.T, pool *repository.Scoped, userID string) {
	t.Helper()
	ctx := repository.WithSystemIdentity(context.Background())
	if _, err := pool.Exec(ctx,
		`UPDATE change_request SET requested_by_user_id = $1::uuid WHERE id = $2`,
		userID, changeRequestAuthorizeGateTestID); err != nil {
		t.Fatalf("set change_request.requested_by_user_id: %v", err)
	}
}

// TestChangeRequestIntegration_PatchAuthorizeProvisionsApproversFromGroupMembers
// is the Authorize-checkpoint counterpart of
// TestChangeRequestIntegration_PatchAssessProvisionsApproversFromGroupMembers:
// a {state: "authorize"} PATCH against a change request that already has its
// Assess-position stage provisions one requested approval_stage_approver row
// per member of the assigned team, at a new, second approval_stage.
func TestChangeRequestIntegration_PatchAuthorizeProvisionsApproversFromGroupMembers(t *testing.T) {
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
	seedChangeRequestForAuthorizeGateTest(t, scoped)
	seedExistingApprovalStage(t, scoped, changeRequestAuthorizeGateTestID)
	seedAuthorizeGateGroup(t, pool)
	seedTeamMembersForAuthorizeGateTest(t, pool, changeRequestAuthorizeGateMemberUserID, changeRequestAuthorizeGateMemberUserID2)
	t.Cleanup(func() {
		_, _ = scoped.Exec(sys, `DELETE FROM approval_stage WHERE work_item_id = $1`, changeRequestAuthorizeGateTestID)
	})

	teamID := changeRequestAuthorizeGateGroupID
	authorize := domain.ChangeRequestStateAuthorize
	if _, err := repo.PatchChangeRequest(sys, changeRequestAuthorizeGateTestID,
		domain.PatchChangeRequestRequest{AssignedTeamID: &teamID, State: &authorize}, "cr-authorize-gate-test"); err != nil {
		t.Fatalf("PatchChangeRequest(assignedTeamId=%s, state=authorize): %v", teamID, err)
	}

	var stageCount int
	if scanErr := scoped.QueryRow(sys,
		`SELECT COUNT(*) FROM approval_stage WHERE work_item_id = $1`, changeRequestAuthorizeGateTestID).Scan(&stageCount); scanErr != nil {
		t.Fatalf("count approval_stage: %v", scanErr)
	}
	if stageCount != 2 {
		t.Fatalf("approval_stage rows after an authorize patch = %d, want exactly 2 (the pre-existing Assess stage plus the new Authorize stage)", stageCount)
	}

	var stageID, stageGroupID string
	if scanErr := scoped.QueryRow(sys,
		`SELECT id, assignment_group_id::TEXT FROM approval_stage WHERE work_item_id = $1 AND assignment_group_id = $2::uuid`,
		changeRequestAuthorizeGateTestID, teamID).Scan(&stageID, &stageGroupID); scanErr != nil {
		t.Fatalf("read back authorize approval_stage: %v", scanErr)
	}
	if stageGroupID != teamID {
		t.Fatalf("approval_stage.assignment_group_id = %q, want %q", stageGroupID, teamID)
	}

	rows, err := scoped.Query(sys,
		`SELECT approver_user_id::TEXT, status FROM approval_stage_approver WHERE stage_id = $1 ORDER BY approver_user_id`, stageID)
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
		changeRequestAuthorizeGateMemberUserID:  "requested",
		changeRequestAuthorizeGateMemberUserID2: "requested",
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

// TestChangeRequestIntegration_PatchAuthorizeDoesNotReprovisionWhenStageExists
// is the Authorize-checkpoint counterpart of
// TestChangeRequestIntegration_PatchAssessDoesNotReprovisionWhenStageExists:
// a second {state: "authorize"} patch against a change request that already
// has its Authorize-position stage must never create a duplicate stage or
// re-seed approvers.
func TestChangeRequestIntegration_PatchAuthorizeDoesNotReprovisionWhenStageExists(t *testing.T) {
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
	seedChangeRequestForAuthorizeGateTest(t, scoped)
	seedExistingApprovalStage(t, scoped, changeRequestAuthorizeGateTestID)
	seedAuthorizeGateGroup(t, pool)
	seedTeamMembersForAuthorizeGateTest(t, pool, changeRequestAuthorizeGateMemberUserID, changeRequestAuthorizeGateMemberUserID2)
	t.Cleanup(func() {
		_, _ = scoped.Exec(sys, `DELETE FROM approval_stage WHERE work_item_id = $1`, changeRequestAuthorizeGateTestID)
	})

	teamID := changeRequestAuthorizeGateGroupID
	authorize := domain.ChangeRequestStateAuthorize
	if _, err := repo.PatchChangeRequest(sys, changeRequestAuthorizeGateTestID,
		domain.PatchChangeRequestRequest{AssignedTeamID: &teamID, State: &authorize}, "cr-authorize-gate-test"); err != nil {
		t.Fatalf("PatchChangeRequest(assignedTeamId=%s, state=authorize) #1: %v", teamID, err)
	}
	if _, err := repo.PatchChangeRequest(sys, changeRequestAuthorizeGateTestID,
		domain.PatchChangeRequestRequest{State: &authorize}, "cr-authorize-gate-test"); err != nil {
		t.Fatalf("PatchChangeRequest(state=authorize) #2 (resend): %v", err)
	}

	var stageCount int
	if scanErr := scoped.QueryRow(sys,
		`SELECT COUNT(*) FROM approval_stage WHERE work_item_id = $1`, changeRequestAuthorizeGateTestID).Scan(&stageCount); scanErr != nil {
		t.Fatalf("count approval_stage: %v", scanErr)
	}
	if stageCount != 2 {
		t.Fatalf("approval_stage rows after two authorize patches = %d, want exactly 2 (not reprovisioned)", stageCount)
	}

	var approverCount int
	if scanErr := scoped.QueryRow(sys,
		`SELECT COUNT(*) FROM approval_stage_approver asa JOIN approval_stage ast ON ast.id = asa.stage_id WHERE ast.work_item_id = $1`,
		changeRequestAuthorizeGateTestID).Scan(&approverCount); scanErr != nil {
		t.Fatalf("count approval_stage_approver: %v", scanErr)
	}
	if approverCount != 2 {
		t.Fatalf("approval_stage_approver rows after two authorize patches = %d, want exactly 2 (not re-seeded)", approverCount)
	}
}

// TestChangeRequestIntegration_PatchAuthorizeRejectsEmptyGroup is the
// Authorize-checkpoint counterpart of
// TestChangeRequestIntegration_PatchAssessRejectsEmptyGroup: an assigned team
// with no members must reject the whole {state: "authorize"} PATCH with a
// ValidationError and leave no new approval_stage behind -- only the
// pre-existing Assess-position stage this test seeds up front.
func TestChangeRequestIntegration_PatchAuthorizeRejectsEmptyGroup(t *testing.T) {
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
	seedChangeRequestForAuthorizeGateTest(t, scoped)
	seedExistingApprovalStage(t, scoped, changeRequestAuthorizeGateTestID)
	seedAuthorizeGateGroup(t, pool)
	// Deliberately no seedTeamMembersForAuthorizeGateTest call -- the group
	// exists (so assignedTeamId itself is valid) but has zero members.
	t.Cleanup(func() {
		_, _ = scoped.Exec(sys, `DELETE FROM approval_stage WHERE work_item_id = $1`, changeRequestAuthorizeGateTestID)
	})

	teamID := changeRequestAuthorizeGateGroupID
	authorize := domain.ChangeRequestStateAuthorize
	_, err = repo.PatchChangeRequest(sys, changeRequestAuthorizeGateTestID,
		domain.PatchChangeRequestRequest{AssignedTeamID: &teamID, State: &authorize}, "cr-authorize-gate-test")
	if err == nil {
		t.Fatal("PatchChangeRequest(state=authorize) with an empty assigned team succeeded, want a ValidationError")
	}
	var valErr *apierror.ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("PatchChangeRequest(state=authorize) with an empty assigned team error = %v (%T), want *apierror.ValidationError", err, err)
	}

	var stageCount int
	if scanErr := scoped.QueryRow(sys,
		`SELECT COUNT(*) FROM approval_stage WHERE work_item_id = $1`, changeRequestAuthorizeGateTestID).Scan(&stageCount); scanErr != nil {
		t.Fatalf("count approval_stage: %v", scanErr)
	}
	if stageCount != 1 {
		t.Fatalf("approval_stage rows after a rejected authorize patch = %d, want exactly 1 (only the pre-existing Assess stage, no dead-end Authorize stage)", stageCount)
	}
}

// TestChangeRequestIntegration_PatchAuthorizeDeduplicatesGroupMembers is the
// Authorize-checkpoint counterpart of
// TestChangeRequestIntegration_PatchAssessDeduplicatesGroupMembers.
func TestChangeRequestIntegration_PatchAuthorizeDeduplicatesGroupMembers(t *testing.T) {
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
	seedChangeRequestForAuthorizeGateTest(t, scoped)
	seedExistingApprovalStage(t, scoped, changeRequestAuthorizeGateTestID)
	seedAuthorizeGateGroup(t, pool)
	seedTeamMembersForAuthorizeGateTest(t, pool, changeRequestAuthorizeGateMemberUserID)
	t.Cleanup(func() {
		_, _ = scoped.Exec(sys, `DELETE FROM approval_stage WHERE work_item_id = $1`, changeRequestAuthorizeGateTestID)
	})

	// A second team_member row for the SAME user against the SAME group --
	// seedTeamMembersForAuthorizeGateTest's own cleanup (DELETE ... WHERE
	// user_id = $1) already covers this row too, since it shares the user id.
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO team_member (id, created_on, updated_on, created_by, updated_by, team_id, user_id, group_id)
		 VALUES (gen_random_uuid(), now(), now(), 'cr-authorize-gate-test', 'cr-authorize-gate-test', $1::uuid, $2, $3::uuid)`,
		seededGroupID, changeRequestAuthorizeGateMemberUserID, changeRequestAuthorizeGateGroupID); err != nil {
		t.Fatalf("seed duplicate team_member row: %v", err)
	}

	teamID := changeRequestAuthorizeGateGroupID
	authorize := domain.ChangeRequestStateAuthorize
	if _, err := repo.PatchChangeRequest(sys, changeRequestAuthorizeGateTestID,
		domain.PatchChangeRequestRequest{AssignedTeamID: &teamID, State: &authorize}, "cr-authorize-gate-test"); err != nil {
		t.Fatalf("PatchChangeRequest(assignedTeamId=%s, state=authorize): %v", teamID, err)
	}

	var approverCount int
	if scanErr := scoped.QueryRow(sys,
		`SELECT COUNT(*) FROM approval_stage_approver asa
		 JOIN approval_stage ast ON ast.id = asa.stage_id
		 WHERE ast.work_item_id = $1 AND ast.assignment_group_id = $2::uuid`,
		changeRequestAuthorizeGateTestID, teamID).Scan(&approverCount); scanErr != nil {
		t.Fatalf("count approval_stage_approver: %v", scanErr)
	}
	if approverCount != 1 {
		t.Fatalf("approval_stage_approver rows for a user with a duplicated team_member row = %d, want exactly 1", approverCount)
	}
}

// TestChangeRequestIntegration_PatchAuthorizeProvisionsRequesterAsCancelled
// is the Authorize-checkpoint counterpart of
// TestChangeRequestIntegration_PatchAssessProvisionsRequesterAsCancelled --
// the identical self-approval-exclusion rule applies at this checkpoint too.
func TestChangeRequestIntegration_PatchAuthorizeProvisionsRequesterAsCancelled(t *testing.T) {
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
	seedChangeRequestForAuthorizeGateTest(t, scoped)
	seedExistingApprovalStage(t, scoped, changeRequestAuthorizeGateTestID)
	seedAuthorizeGateGroup(t, pool)
	seedTeamMembersForAuthorizeGateTest(t, pool, changeRequestAuthorizeGateMemberUserID, changeRequestAuthorizeGateMemberUserID2)
	setAuthorizeGateRequestedBy(t, scoped, changeRequestAuthorizeGateMemberUserID)
	t.Cleanup(func() {
		_, _ = scoped.Exec(sys, `DELETE FROM approval_stage WHERE work_item_id = $1`, changeRequestAuthorizeGateTestID)
	})

	teamID := changeRequestAuthorizeGateGroupID
	authorize := domain.ChangeRequestStateAuthorize
	if _, err := repo.PatchChangeRequest(sys, changeRequestAuthorizeGateTestID,
		domain.PatchChangeRequestRequest{AssignedTeamID: &teamID, State: &authorize}, "cr-authorize-gate-test"); err != nil {
		t.Fatalf("PatchChangeRequest(assignedTeamId=%s, state=authorize): %v", teamID, err)
	}

	var stageID string
	if scanErr := scoped.QueryRow(sys,
		`SELECT id FROM approval_stage WHERE work_item_id = $1 AND assignment_group_id = $2::uuid`,
		changeRequestAuthorizeGateTestID, teamID).Scan(&stageID); scanErr != nil {
		t.Fatalf("read back approval_stage: %v", scanErr)
	}

	rows, err := scoped.Query(sys,
		`SELECT approver_user_id::TEXT, status FROM approval_stage_approver WHERE stage_id = $1`, stageID)
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
		changeRequestAuthorizeGateMemberUserID:  "cancelled",
		changeRequestAuthorizeGateMemberUserID2: "requested",
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

// TestChangeRequestIntegration_PatchAuthorizeRejectsWhenOnlyMemberIsRequester
// is the Authorize-checkpoint counterpart of
// TestChangeRequestIntegration_PatchAssessRejectsWhenOnlyMemberIsRequester.
func TestChangeRequestIntegration_PatchAuthorizeRejectsWhenOnlyMemberIsRequester(t *testing.T) {
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
	seedChangeRequestForAuthorizeGateTest(t, scoped)
	seedExistingApprovalStage(t, scoped, changeRequestAuthorizeGateTestID)
	seedAuthorizeGateGroup(t, pool)
	seedTeamMembersForAuthorizeGateTest(t, pool, changeRequestAuthorizeGateMemberUserID)
	setAuthorizeGateRequestedBy(t, scoped, changeRequestAuthorizeGateMemberUserID)
	t.Cleanup(func() {
		_, _ = scoped.Exec(sys, `DELETE FROM approval_stage WHERE work_item_id = $1`, changeRequestAuthorizeGateTestID)
	})

	teamID := changeRequestAuthorizeGateGroupID
	authorize := domain.ChangeRequestStateAuthorize
	_, err = repo.PatchChangeRequest(sys, changeRequestAuthorizeGateTestID,
		domain.PatchChangeRequestRequest{AssignedTeamID: &teamID, State: &authorize}, "cr-authorize-gate-test")
	if err == nil {
		t.Fatal("PatchChangeRequest(state=authorize) whose only team member is the requester succeeded, want a ValidationError")
	}
	var valErr *apierror.ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("PatchChangeRequest(state=authorize) whose only team member is the requester error = %v (%T), want *apierror.ValidationError", err, err)
	}

	var stageCount int
	if scanErr := scoped.QueryRow(sys,
		`SELECT COUNT(*) FROM approval_stage WHERE work_item_id = $1`, changeRequestAuthorizeGateTestID).Scan(&stageCount); scanErr != nil {
		t.Fatalf("count approval_stage: %v", scanErr)
	}
	if stageCount != 1 {
		t.Fatalf("approval_stage rows after a rejected authorize patch = %d, want exactly 1 (only the pre-existing Assess stage, no dead-end Authorize stage)", stageCount)
	}
}

// TestChangeRequestIntegration_AssessAndAuthorizeStagesCoexist runs the real,
// full production path end to end -- a {state: "assess"} PATCH (provisioning
// the Assess-position stage), then an approval decision on it
// (DecideChangeRequestApproval's own Assess->Authorize cascade, which this
// change wires into the new Authorize-stage provisioning too, as a
// best-effort step -- see that call site's own comment) -- and confirms both
// stages land correctly and independently: the Assess stage's own approvers
// (one Approved, one Cancelled by the decision's own sibling-cancellation
// rule) are untouched by the Authorize stage's own, separately-provisioned
// approvers (both freshly "requested"), and each stage is attributed to its
// own, distinct id rather than one clobbering the other.
func TestChangeRequestIntegration_AssessAndAuthorizeStagesCoexist(t *testing.T) {
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
		t.Fatalf("read back state: %v", scanErr)
	}
	if gotState != "AUTHORIZE" {
		t.Fatalf("state after approval = %q, want \"AUTHORIZE\"", gotState)
	}

	var stageCount int
	if scanErr := scoped.QueryRow(sys,
		`SELECT COUNT(*) FROM approval_stage WHERE work_item_id = $1`, changeRequestAssessGateTestID).Scan(&stageCount); scanErr != nil {
		t.Fatalf("count approval_stage: %v", scanErr)
	}
	if stageCount != 2 {
		t.Fatalf("approval_stage rows after the assess->authorize cascade = %d, want exactly 2 (Assess plus the newly auto-provisioned Authorize stage)", stageCount)
	}

	var authorizeStageID string
	if scanErr := scoped.QueryRow(sys,
		`SELECT id FROM approval_stage WHERE work_item_id = $1 AND id != $2`,
		changeRequestAssessGateTestID, assessStageID).Scan(&authorizeStageID); scanErr != nil {
		t.Fatalf("read back Authorize approval_stage: %v", scanErr)
	}
	if authorizeStageID == assessStageID {
		t.Fatal("Authorize stage id equals the Assess stage id -- a new stage was not actually created")
	}

	// The Assess stage's own approvers: the acted-on approver is Approved,
	// the sibling is Cancelled by DecideChangeRequestApproval's own
	// sibling-cancellation rule -- neither is disturbed by the Authorize
	// stage's own, entirely separate provisioning.
	assessApprovers := map[string]string{}
	assessRows, err := scoped.Query(sys,
		`SELECT approver_user_id::TEXT, status FROM approval_stage_approver WHERE stage_id = $1`, assessStageID)
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
		changeRequestAssessGateMemberUserID:  "approved",
		changeRequestAssessGateMemberUserID2: "cancelled",
	}
	if len(assessApprovers) != len(wantAssess) {
		t.Fatalf("Assess approval_stage_approver rows = %+v, want exactly %+v", assessApprovers, wantAssess)
	}
	for uid, wantStatus := range wantAssess {
		if assessApprovers[uid] != wantStatus {
			t.Fatalf("Assess approver %s status = %q, want %q", uid, assessApprovers[uid], wantStatus)
		}
	}

	// The Authorize stage's own approvers: both members freshly provisioned
	// as "requested", reusing the same assigned team -- confirming the new
	// checkpoint's provisioning ran independently of, and did not reuse or
	// clobber, the Assess stage's own rows.
	authorizeApprovers := map[string]string{}
	authorizeRows, err := scoped.Query(sys,
		`SELECT approver_user_id::TEXT, status FROM approval_stage_approver WHERE stage_id = $1`, authorizeStageID)
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
		changeRequestAssessGateMemberUserID:  "requested",
		changeRequestAssessGateMemberUserID2: "requested",
	}
	if len(authorizeApprovers) != len(wantAuthorize) {
		t.Fatalf("Authorize approval_stage_approver rows = %+v, want exactly %+v", authorizeApprovers, wantAuthorize)
	}
	for uid, wantStatus := range wantAuthorize {
		if authorizeApprovers[uid] != wantStatus {
			t.Fatalf("Authorize approver %s status = %q, want %q", uid, authorizeApprovers[uid], wantStatus)
		}
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
		 VALUES ($1, now(), now(), 'cr-review-gate-test', 'cr-review-gate-test', 'CRREV001', 'review gate test', 'CHANGE_REQUEST')`,
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
		`SELECT approver_user_id::TEXT, status FROM approval_stage_approver WHERE stage_id = $1 ORDER BY approver_user_id`, stageID)
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
		changeRequestReviewGateMemberUserID:  "requested",
		changeRequestReviewGateMemberUserID2: "requested",
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
		`SELECT approver_user_id::TEXT, status FROM approval_stage_approver WHERE stage_id = $1`, stageID)
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
		changeRequestReviewGateMemberUserID:  "cancelled",
		changeRequestReviewGateMemberUserID2: "requested",
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
		`SELECT approver_user_id::TEXT, status FROM approval_stage_approver WHERE stage_id = $1`, assessStageID)
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
		changeRequestAssessGateMemberUserID:  "approved",
		changeRequestAssessGateMemberUserID2: "cancelled",
	}
	if len(assessApprovers) != len(wantAssess) {
		t.Fatalf("Assess approval_stage_approver rows = %+v, want exactly %+v", assessApprovers, wantAssess)
	}
	for uid, wantStatus := range wantAssess {
		if assessApprovers[uid] != wantStatus {
			t.Fatalf("Assess approver %s status = %q, want %q", uid, assessApprovers[uid], wantStatus)
		}
	}

	// The Authorize stage's own approvers: both freshly "requested", reusing
	// the Assess checkpoint's own assigned team -- unchanged by the Review
	// patch that followed.
	authorizeApprovers := map[string]string{}
	authorizeRows, err := scoped.Query(sys,
		`SELECT approver_user_id::TEXT, status FROM approval_stage_approver WHERE stage_id = $1`, authorizeStageID)
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
		changeRequestAssessGateMemberUserID:  "requested",
		changeRequestAssessGateMemberUserID2: "requested",
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
		`SELECT approver_user_id::TEXT, status FROM approval_stage_approver WHERE stage_id = $1`, reviewStageID)
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
		changeRequestReviewGateMemberUserID:  "requested",
		changeRequestReviewGateMemberUserID2: "requested",
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
// pair in the given state, with is_on_hold/on_hold_reason/on_hold_started_on
// set directly via SQL rather than through PatchChangeRequest -- the tests
// below are split between exercising the GATE (which needs an "already on
// hold" precondition to exist before the PATCH under test ever runs) and the
// WRITE path itself (covered separately), so seeding the precondition
// directly keeps the two concerns from tangling. onHold=false seeds a
// never-been-on-hold record (is_on_hold FALSE, reason/since left NULL),
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
		mustExec(`INSERT INTO change_request (id, state, is_on_hold, on_hold_reason, on_hold_started_on)
		          VALUES ($1, $2::change_request_state_enum, TRUE, $3, now())`,
			changeRequestOnHoldTestID, state, reason)
	} else {
		mustExec(`INSERT INTO change_request (id, state, is_on_hold) VALUES ($1, $2::change_request_state_enum, FALSE)`,
			changeRequestOnHoldTestID, state)
	}
}

// TestChangeRequestIntegration_PatchOnHoldPersistsAndReadsBack confirms
// {onHold: true, onHoldReason: ...} actually persists change_request.is_on_hold/
// on_hold_reason/on_hold_started_on, both in PatchChangeRequest's own
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
	if cr.OnHoldSince == nil || *cr.OnHoldSince == "" {
		t.Fatalf("OnHoldSince after patch = %v, want a non-empty timestamp", cr.OnHoldSince)
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
	if got.OnHoldSince == nil || *got.OnHoldSince != *cr.OnHoldSince {
		t.Fatalf("GetChangeRequestByID OnHoldSince = %v, want %v", got.OnHoldSince, cr.OnHoldSince)
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
// on_hold_reason/on_hold_started_on exactly the same way a standalone
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
	if cr.OnHoldSince != nil {
		t.Fatalf("OnHoldSince after simultaneous clear+advance = %v, want nil", cr.OnHoldSince)
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
			if cr.OnHoldSince != nil {
				t.Fatalf("OnHoldSince after patch = %v, want nil", cr.OnHoldSince)
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
// change_request pair linked to projectID, with is_customer_approved/
// is_customer_reviewed seeded directly via SQL -- bypassing PatchChangeRequest
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
	mustExec(`INSERT INTO change_request (id, state, is_customer_approved, is_customer_reviewed)
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
// confirms an internal caller may flip BOTH is_customer_approved and
// is_customer_reviewed from false to true, in one PATCH, with no project
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
// request's OWN project may flip is_customer_approved false -> true.
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
	if scanErr := scoped.QueryRow(sys, `SELECT is_customer_approved FROM change_request WHERE id = $1`, crCustomerFlagDifferentProjectTestID).Scan(&gotApproved); scanErr != nil {
		t.Fatalf("read back is_customer_approved: %v", scanErr)
	}
	if gotApproved != nil && *gotApproved {
		t.Fatal("is_customer_approved after rejected patch = true, want unchanged false")
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
	if scanErr := scoped.QueryRow(sys, `SELECT is_customer_approved FROM change_request WHERE id = $1`, crCustomerFlagWrongRoleTestID).Scan(&gotApproved); scanErr != nil {
		t.Fatalf("read back is_customer_approved: %v", scanErr)
	}
	if gotApproved != nil && *gotApproved {
		t.Fatal("is_customer_approved after rejected patch = true, want unchanged false")
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
	if scanErr := scoped.QueryRow(sys, `SELECT is_customer_approved, is_customer_reviewed FROM change_request WHERE id = $1`,
		crCustomerFlagLockedTestID).Scan(&gotApproved, &gotReviewed); scanErr != nil {
		t.Fatalf("read back: %v", scanErr)
	}
	if !gotApproved || !gotReviewed {
		t.Fatalf("is_customer_approved/is_customer_reviewed after both rejected patches = %v/%v, want unchanged true/true", gotApproved, gotReviewed)
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
// single PATCH with is_customer_approved ALREADY true (so true -> true, a
// no-op, always allowed) and is_customer_reviewed false -> true (the one
// real gated transition, allowed here since the caller is internal)
// succeeds as a whole, changing only is_customer_reviewed.
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
