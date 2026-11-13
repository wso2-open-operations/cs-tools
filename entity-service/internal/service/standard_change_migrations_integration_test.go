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

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// The migrations of the standard change work, against PostgreSQL. They share
// the standard change integration tests' fixtures (newStdEnv) and DSNs:
//
//	STANDARD_CHANGE_TEST_DSN=postgres://... STANDARD_CHANGE_TEST_APP_DSN=postgres://... \
//	  go test ./internal/service/ -run StandardChangeIntegration -p 1
//
// They run migration files on the database they are given, as make migrate
// would: use a test database.

// stdBranchMigrations are the migrations of the standard change work, in the
// order make migrate applies them (every NNNN_ file by name, then every
// 1NNNNN_ file). make migrate keys a migration by its filename, and these were
// renumbered after dev's moved past them, so a database that recorded them
// under an earlier number runs every one of them again.
var stdBranchMigrations = []string{
	"0287_standard_change_authority.sql",
	"0288_change_request_closed_on_backfill.sql",
	"0289_standard_change_suggestion_dismissal.sql",
	"0290_change_request_cloned_from.sql",
	"0291_standard_change_suggestion_snapshot.sql",
	"0292_change_request_cloned_from_index.sql",
	"0293_standard_change_suggestion_skipped.sql",
	"0294_standard_change_suggestion_carried_keys.sql",
	"0295_standard_change_no_cab_stage.sql",
	"0296_standard_change_retirement_history.sql",
	"0297_standard_change_cab_authorises.sql",
	"100029_standard_change_suggestion_dismissal_rls.sql",
	"100030_standard_change_suggestion_snapshot_rls.sql",
	"100031_standard_change_template_event_rls.sql",
}

// stdReplaySnapshot is everything the test's own rows hold that a migration of
// this work writes, and the definitions those migrations create, as JSON per
// part, for comparing before and after a replay.
const stdReplaySnapshot = `
WITH props AS (
    SELECT id FROM work_item WHERE type = 'STANDARD_CHANGE_PROPOSAL' AND created_by LIKE 'std-%@scp.test'
), tpls AS (
    SELECT id FROM standard_change_template WHERE name ILIKE '` + stdTag + `%'
), crs AS (
    SELECT id FROM work_item WHERE type = 'CHANGE_REQUEST' AND subject ILIKE '` + stdTag + `%'
)
SELECT jsonb_build_object(
    'proposals', (SELECT jsonb_agg(to_jsonb(p) ORDER BY p.id) FROM standard_change_proposal p WHERE p.id IN (SELECT id FROM props)),
    'proposal work items', (SELECT jsonb_agg(jsonb_build_object('id', w.id, 'updated_on', w.updated_on, 'updated_by', w.updated_by) ORDER BY w.id)
                              FROM work_item w WHERE w.id IN (SELECT id FROM props)),
    'stages', (SELECT jsonb_agg(to_jsonb(s) ORDER BY s.id) FROM approval_stage s WHERE s.work_item_id IN (SELECT id FROM props)),
    'approvers', (SELECT jsonb_agg(to_jsonb(a) ORDER BY a.id) FROM approval_stage_approver a WHERE a.work_item_id IN (SELECT id FROM props)),
    'templates', (SELECT jsonb_agg(to_jsonb(t) ORDER BY t.id) FROM standard_change_template t WHERE t.id IN (SELECT id FROM tpls)),
    'template events', (SELECT jsonb_agg(to_jsonb(e) ORDER BY e.id) FROM standard_change_template_event e WHERE e.template_id IN (SELECT id FROM tpls)),
    'change requests', (SELECT jsonb_agg(jsonb_build_object('id', cr.id, 'state', cr.state, 'closed_on', cr.closed_on, 'closed_by', cr.closed_by_user_id,
                                                            'cloned_from', cr.cloned_from_change_request_id) ORDER BY cr.id)
                          FROM change_request cr WHERE cr.id IN (SELECT id FROM crs)),
    'dismissals', (SELECT jsonb_agg(to_jsonb(d) ORDER BY d.id) FROM standard_change_suggestion_dismissal d
                    WHERE d.account_id IN (SELECT id FROM account WHERE number LIKE 'ZZSTDTEST-%')),
    'suggestion runs', (SELECT jsonb_agg(to_jsonb(r) ORDER BY r.id) FROM standard_change_suggestion_run r WHERE r.id = $1::uuid),
    'suggestion run customers', (SELECT jsonb_agg(to_jsonb(c) ORDER BY c.account_id) FROM standard_change_suggestion_run_customer c WHERE c.run_id = $1::uuid),
    'suggestion snapshots', (SELECT jsonb_agg(to_jsonb(x) ORDER BY x.id) FROM standard_change_suggestion_snapshot x WHERE x.run_id = $1::uuid),
    'authority group', (SELECT to_jsonb(g) FROM "group" g WHERE g.id = '00000000-0000-4000-8000-0000000005ca'::uuid),
    'constraints', (SELECT jsonb_agg(conrelid::regclass::text || ' ' || conname || ' ' || pg_get_constraintdef(oid) ORDER BY conrelid::regclass::text, conname)
                      FROM pg_constraint WHERE conrelid IN ('standard_change_proposal'::regclass, 'standard_change_template_event'::regclass, 'approval_stage'::regclass)),
    'indexes', (SELECT jsonb_agg(indexdef ORDER BY indexname) FROM pg_indexes
                 WHERE tablename IN ('standard_change_proposal', 'standard_change_template_event', 'change_request', 'standard_change_suggestion_run',
                                     'standard_change_suggestion_snapshot', 'standard_change_suggestion_dismissal')),
    'policies', (SELECT jsonb_agg(tablename || ' ' || policyname || ' ' || COALESCE(qual, '') || ' ' || COALESCE(with_check, '') ORDER BY tablename, policyname)
                  FROM pg_policies WHERE tablename LIKE 'standard_change_%')
)::text`

// Every migration of this work runs again on a database that recorded it under
// an earlier number, after the new code has written data there. Replayed in
// order, the files must change nothing of it: a proposal submitted to the CAB
// with one approval given keeps its stage, its approver rows and its state (0295
// once deleted every stage not labelled "Standard Change Authority" from an
// open proposal, today's "CAB Approval" stage included, and 0297 then returned
// the proposal to draft), and so do a decided proposal, a draft, a retired and a
// suspended template and their history, a clone's source, a completed change
// request's closure, a dismissal and a published suggestion run, and the
// checks, indexes and policies the files create.
func TestStandardChangeIntegration_MigrationsReplay(t *testing.T) {
	ownerDSN, ok := stdDSNs(t)["owner"]
	if !ok {
		t.Skip("STANDARD_CHANGE_TEST_DSN not set: only the tables' owner can run the migrations")
	}
	files := make([]string, len(stdBranchMigrations))
	for i, name := range stdBranchMigrations {
		b, err := os.ReadFile("../../migrations/" + name)
		if err != nil {
			t.Fatalf("read migration: %v", err)
		}
		files[i] = string(b)
	}
	env := newStdEnv(t, ownerDSN)
	sys := repository.WithSystemIdentity(context.Background())
	exec := func(what, sql string, args ...any) {
		t.Helper()
		if _, err := env.scoped.Exec(sys, sql, args...); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
	}
	runAll := func(what string) {
		t.Helper()
		for i, name := range stdBranchMigrations {
			stdRunSQL(t, env.pool, what+" of "+name, files[i])
		}
	}
	// The release: every file runs once, as on a database that never had them.
	runAll("the release's run")

	// What the new code then writes. A proposal at the CAB with one approval of
	// the two it needs:
	atCAB := readyDraft(t, env, stdTag+" Replay at the CAB")
	if _, err := env.svc.SubmitProposal(stdCtx(stdProposer), atCAB.ID); err != nil {
		t.Fatal(err)
	}
	atCAB, err := env.svc.DecideProposal(stdCtx(stdAuth1), atCAB.ID, atStage(t, env, atCAB.ID, domain.DecideStandardChangeProposalRequest{Decision: "approved"}))
	if err != nil || atCAB.State != domain.StandardChangeProposalStateAuthorityReview || len(atCAB.Stages) != 1 ||
		atCAB.Stages[0].Label != domain.CABApprovalGroupName || atCAB.Stages[0].Approvals != 1 {
		t.Fatalf("CAB fixture = %s %+v, %v", atCAB.State, atCAB.Stages, err)
	}
	// A draft, never submitted.
	readyDraft(t, env, stdTag+" Replay draft")
	// A template the CAB approved, then retired (a RETIRED entry in its history),
	// and another one suspended (a SUSPENDED entry).
	retired := approveNew(t, env, stdProposer, readyDraft(t, env, stdTag+" Replay retired").ID, stdTag+" Replay retired")
	r, err := env.svc.CreateProposal(stdCtx(stdProposer), domain.CreateStandardChangeProposalRequest{Kind: domain.StandardChangeProposalKindRetire, TemplateID: &retired.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.svc.UpdateProposal(stdCtx(stdProposer), r.ID, domain.UpdateStandardChangeProposalRequest{Rationale: stdp("No longer needed")}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.svc.SubmitProposal(stdCtx(stdProposer), r.ID); err != nil {
		t.Fatal(err)
	}
	authorise(t, env, r.ID)
	if tpl, _ := env.svc.GetTemplate(stdCtx(stdCAB1), retired.ID); tpl.State != domain.StandardChangeTemplateStateRetired {
		t.Fatalf("retired fixture = %s", tpl.State)
	}
	suspended := approveNew(t, env, stdProposer, readyDraft(t, env, stdTag+" Replay suspended").ID, stdTag+" Replay suspended")
	if _, err := env.svc.SuspendTemplate(stdCtx(stdCAB1), suspended.ID, domain.SuspendStandardChangeTemplateRequest{Reason: "Under review"}); err != nil {
		t.Fatal(err)
	}
	// A clone, a dismissed suggestion and a published suggestion run.
	exec("record a clone", `UPDATE change_request SET cloned_from_change_request_id = $2::uuid WHERE id = $1::uuid`, env.crs["closed2"], env.crs["closed1"])
	exec("dismiss a suggestion", `INSERT INTO standard_change_suggestion_dismissal (account_id, suggestion_key, reason, dismissed_by)
		VALUES ($1::uuid, 'zz std replay', 'Not repeatable', $2)`, env.accounts["A"], stdCAB1)
	var run string
	if err := env.scoped.QueryRow(sys, `INSERT INTO standard_change_suggestion_run (completed_on, criteria, customers, change_requests, clusters, eligible)
		VALUES (NOW(), '{}', 1, 2, 1, 1) RETURNING id::text`).Scan(&run); err != nil {
		t.Fatalf("publish a suggestion run: %v", err)
	}
	t.Cleanup(func() {
		if _, err := env.scoped.Exec(sys, `DELETE FROM standard_change_suggestion_run WHERE id = $1::uuid`, run); err != nil {
			t.Errorf("delete the suggestion run: %v", err)
		}
	})
	exec("a run's customer", `INSERT INTO standard_change_suggestion_run_customer (run_id, account_id, change_requests, carried_keys)
		VALUES ($1::uuid, $2::uuid, 2, '[{"key": "zz std replay", "memberIds": []}]')`, run, env.accounts["A"])
	exec("a run's suggestion", `INSERT INTO standard_change_suggestion_snapshot (run_id, account_id, suggestion_key, eligible, member_ids, suggestion)
		VALUES ($1::uuid, $2::uuid, 'zz std replay', true, ARRAY[$3::uuid, $4::uuid], '{}')`, run, env.accounts["A"], env.crs["closed1"], env.crs["closed2"])

	snapshot := func() map[string]json.RawMessage {
		t.Helper()
		var raw string
		if err := env.scoped.QueryRow(sys, stdReplaySnapshot, run).Scan(&raw); err != nil {
			t.Fatalf("snapshot: %v", err)
		}
		out := map[string]json.RawMessage{}
		if err := json.Unmarshal([]byte(raw), &out); err != nil {
			t.Fatalf("snapshot: %v", err)
		}
		return out
	}
	before := snapshot()
	for _, part := range []string{"proposals", "stages", "approvers", "templates", "template events", "change requests", "dismissals", "suggestion snapshots"} {
		if string(before[part]) == "null" {
			t.Fatalf("snapshot: no %s to compare", part)
		}
	}

	runAll("the replay")

	// The proposal at the CAB, as its page reads it, then the rest row by row.
	got, err := env.svc.GetProposal(stdCtx(stdProposer), atCAB.ID)
	if err != nil || got.State != domain.StandardChangeProposalStateAuthorityReview || got.SubmittedOn == nil || len(got.Stages) != 1 ||
		got.Stages[0].ID != atCAB.Stages[0].ID || got.Stages[0].Approvals != 1 || approverStates(got)[stdAuth1] != "APPROVED" {
		t.Fatalf("after the replay, the proposal at the CAB = %s submittedOn %v %+v %v, %v; want it unchanged",
			got.State, got.SubmittedOn, got.Stages, approverStates(got), err)
	}
	after := snapshot()
	for part, was := range before {
		if string(after[part]) != string(was) {
			t.Errorf("the replay changed the %s:\nbefore %s\nafter  %s", part, was, after[part])
		}
	}
	if t.Failed() {
		return
	}
	// And the proposal carries on: the second approval decides it.
	if p, err := env.svc.DecideProposal(stdCtx(stdAuth2), atCAB.ID, atStage(t, env, atCAB.ID, domain.DecideStandardChangeProposalRequest{Decision: "approved"})); err != nil ||
		p.State != domain.StandardChangeProposalStateApproved || p.Template == nil {
		t.Fatalf("the second approval after the replay = %s, %v", p.State, err)
	}
}

// Migration 0295 removes the CAB stage. A proposal still in CAB review goes back
// to draft, submitted_on cleared, and loses its CAB stage: a clean draft again,
// which can be deleted; submitted again it goes to the authority, and a decision
// sent for its old stage is refused. A proposal the CAB approved under the old
// flow, already in authority review, loses its CAB stage too and carries on with
// the authority's. A decided one keeps every stage, the record of who approved
// the template in force. The state check no longer admits CAB_REVIEW. Running it
// twice changes nothing more.
//
// 0295 was written while the "Standard Change Authority" decided proposals, so
// the fixtures are labelled as the code of that time wrote them. Since 0297 a
// submitted proposal's one stage is labelled "CAB Approval" too, the old CAB
// stage's label: 0295 tells the old flow's CAB stage by the authority stage
// after it, and a proposal submitted to the CAB today, with an approval given,
// keeps both through every run.
//
// The tables belong to STANDARD_CHANGE_TEST_DSN's role, the only one here that
// may alter them, so the whole file runs there. Its UPDATEs also run as
// STANDARD_CHANGE_TEST_APP_DSN's role (no BYPASSRLS, so FORCE row-level security
// applies, as it does to the role make migrate uses on staging and production),
// on a plain session nobody flagged internal: they must still find the rows.
func TestStandardChangeIntegration_NoCABStageMigration(t *testing.T) {
	dsns := stdDSNs(t)
	ownerDSN, ok := dsns["owner"]
	if !ok {
		t.Skip("STANDARD_CHANGE_TEST_DSN not set: only the tables' owner can run 0295")
	}
	migration, err := os.ReadFile("../../migrations/0295_standard_change_no_cab_stage.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	env := newStdEnv(t, ownerDSN)
	sys := repository.WithSystemIdentity(context.Background())
	exec := func(what, sql string, args ...any) {
		t.Helper()
		if _, err := env.scoped.Exec(sys, sql, args...); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
	}
	run := func(pool *pgxpool.Pool, what, sql string) {
		t.Helper()
		conn, err := pool.Acquire(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Release()
		if _, err := conn.Exec(context.Background(), sql); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		var flag string
		if err := conn.QueryRow(context.Background(), `SELECT COALESCE(current_setting('app.is_internal', true), '')`).Scan(&flag); err != nil || flag != "" {
			t.Fatalf("%s: app.is_internal = %q afterwards (%v), want it cleared", what, flag, err)
		}
	}
	// The fixtures need the check the old code wrote under; the migration puts
	// the new one back, and so does the cleanup should the test stop first.
	exec("relax the state check", `ALTER TABLE standard_change_proposal DROP CONSTRAINT IF EXISTS standard_change_proposal_state_check`)
	t.Cleanup(func() {
		if _, err := env.pool.Exec(context.Background(), string(migration)); err != nil {
			t.Errorf("restore the state check: %v", err)
		}
	})
	// A proposal as the old code left it in CAB review: one "CAB Approval" stage,
	// a single approval needed, its approvers asked.
	inCAB := func(name string) domain.StandardChangeProposal {
		t.Helper()
		p := readyDraft(t, env, name)
		if _, err := env.svc.SubmitProposal(stdCtx(stdProposer), p.ID); err != nil {
			t.Fatal(err)
		}
		exec("move to CAB review", `UPDATE standard_change_proposal SET state = 'CAB_REVIEW' WHERE id = $1::uuid`, p.ID)
		exec("make the stage the CAB's", `UPDATE approval_stage SET checkpoint_label = 'CAB Approval', required_approvals = 1 WHERE work_item_id = $1::uuid`, p.ID)
		got, err := env.svc.GetProposal(stdCtx(stdProposer), p.ID)
		if err != nil || len(got.Stages) != 1 || got.Stages[0].Label != "CAB Approval" || approverStates(got)[stdAuth1] != "REQUESTED" {
			t.Fatalf("CAB review fixture = %+v, %v", got.Stages, err)
		}
		return got
	}
	// A settled "CAB Approval" stage, an hour older than the authority's, as the
	// old flow left a proposal the CAB had approved.
	addCABStage := func(id string) {
		t.Helper()
		var stage string
		if err := env.scoped.QueryRow(sys, `INSERT INTO approval_stage (id, created_on, updated_on, created_by, updated_by, work_item_id, assignment_group_id, raw_status, checkpoint_label, required_approvals)
			VALUES (gen_random_uuid(), NOW() - INTERVAL '1 hour', NOW() - INTERVAL '1 hour', 't', 't', $1::uuid, $2::uuid, 'APPROVED', 'CAB Approval', 1)
			RETURNING id::text`, id, cabGroupID).Scan(&stage); err != nil {
			t.Fatalf("seed the CAB stage: %v", err)
		}
		exec("seed the CAB approval", `INSERT INTO approval_stage_approver (id, created_on, updated_on, created_by, updated_by, work_item_id, stage_id, approver_user_id, state)
			SELECT gen_random_uuid(), NOW() - INTERVAL '1 hour', NOW() - INTERVAL '1 hour', 't', 't', $1::uuid, $2::uuid, u.id, 'APPROVED' FROM "user" u WHERE u.email = $3`,
			id, stage, stdColleague)
	}
	// The one stage SubmitProposal provisions, labelled as the authority's.
	asAuthority := func(id string) {
		t.Helper()
		exec("label the stage as the authority's", `UPDATE approval_stage SET checkpoint_label = 'Standard Change Authority' WHERE work_item_id = $1::uuid`, id)
	}
	cab := inCAB(stdTag + " Waiting on the CAB")
	waiting := readyDraft(t, env, stdTag+" Waiting on the authority")
	if _, err := env.svc.SubmitProposal(stdCtx(stdProposer), waiting.ID); err != nil {
		t.Fatal(err)
	}
	asAuthority(waiting.ID)
	addCABStage(waiting.ID)
	if got, err := env.svc.GetProposal(stdCtx(stdProposer), waiting.ID); err != nil || len(got.Stages) != 2 {
		t.Fatalf("authority review fixture = %+v, %v", got.Stages, err)
	}
	if _, err := env.svc.DecideProposal(stdCtx(stdAuth1), waiting.ID, atStage(t, env, waiting.ID, domain.DecideStandardChangeProposalRequest{Decision: "approved"})); err != nil {
		t.Fatal(err)
	}
	decided := readyDraft(t, env, stdTag+" Decided")
	if _, err := env.svc.SubmitProposal(stdCtx(stdProposer), decided.ID); err != nil {
		t.Fatal(err)
	}
	asAuthority(decided.ID)
	addCABStage(decided.ID)
	authorise(t, env, decided.ID)

	// Today's flow: submitted to the CAB ("CAB Approval", two approvals needed),
	// one approval given.
	today := readyDraft(t, env, stdTag+" At the CAB today")
	if _, err := env.svc.SubmitProposal(stdCtx(stdProposer), today.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := env.svc.DecideProposal(stdCtx(stdAuth1), today.ID, atStage(t, env, today.ID, domain.DecideStandardChangeProposalRequest{Decision: "approved"})); err != nil {
		t.Fatal(err)
	}
	today, err = env.svc.GetProposal(stdCtx(stdProposer), today.ID)
	if err != nil || len(today.Stages) != 1 || today.Stages[0].Approvals != 1 {
		t.Fatalf("CAB fixture = %+v, %v", today.Stages, err)
	}

	checkReturned := func(what string, before domain.StandardChangeProposal) domain.StandardChangeProposal {
		t.Helper()
		got, err := env.svc.GetProposal(stdCtx(stdProposer), before.ID)
		if err != nil || got.State != domain.StandardChangeProposalStateDraft || got.SubmittedOn != nil || !got.CanEdit || !got.CanSubmit || !got.CanDelete {
			t.Fatalf("%s: returned = %s submittedOn %v canEdit %v canSubmit %v canDelete %v, %v",
				what, got.State, got.SubmittedOn, got.CanEdit, got.CanSubmit, got.CanDelete, err)
		}
		if len(got.Stages) != 0 || len(got.Approvers) != 0 {
			t.Fatalf("%s: the CAB stage is still there: %+v %+v", what, got.Stages, got.Approvers)
		}
		return got
	}
	for run1 := 1; run1 <= 2; run1++ {
		run(env.pool, fmt.Sprintf("run %d of 0295", run1), string(migration))
		checkReturned(fmt.Sprintf("run %d", run1), cab)
		if got, _ := env.svc.GetProposal(stdCtx(stdProposer), waiting.ID); got.State != domain.StandardChangeProposalStateAuthorityReview ||
			len(got.Stages) != 1 || got.Stages[0].Label != "Standard Change Authority" ||
			got.Stages[0].Approvals != 1 || approverStates(got)[stdAuth2] != "REQUESTED" || approverStates(got)[stdColleague] != "" {
			t.Fatalf("run %d: a proposal in authority review = %s %+v %v, want only the authority's stage", run1, got.State, got.Stages, approverStates(got))
		}
		if got, _ := env.svc.GetProposal(stdCtx(stdProposer), decided.ID); got.State != domain.StandardChangeProposalStateApproved ||
			len(got.Stages) != 2 || got.Stages[0].Label != "CAB Approval" || got.Stages[1].Approvals != 2 {
			t.Fatalf("run %d: a decided proposal was touched: %s %+v", run1, got.State, got.Stages)
		}
		if got, _ := env.svc.GetProposal(stdCtx(stdProposer), today.ID); got.State != domain.StandardChangeProposalStateAuthorityReview ||
			len(got.Stages) != 1 || got.Stages[0].ID != today.Stages[0].ID || got.Stages[0].Approvals != 1 || approverStates(got)[stdAuth1] != "APPROVED" {
			t.Fatalf("run %d: a proposal submitted to the CAB today lost its stage or its approval: %s %+v %v", run1, got.State, got.Stages, approverStates(got))
		}
	}
	if _, err := env.scoped.Exec(sys, `UPDATE standard_change_proposal SET state = 'CAB_REVIEW' WHERE id = $1::uuid`, cab.ID); err == nil {
		t.Fatal("the state check still admits CAB_REVIEW")
	}

	// Submitted again (today), the returned draft goes to the CAB, with its one
	// two-approval stage; a decision sent for the old CAB stage (a page loaded
	// before) is refused.
	resubmitted, err := env.svc.SubmitProposal(stdCtx(stdProposer), cab.ID)
	if err != nil || resubmitted.State != domain.StandardChangeProposalStateAuthorityReview || len(resubmitted.Stages) != 1 ||
		resubmitted.Stages[0].Label != domain.CABApprovalGroupName || resubmitted.Stages[0].RequiredApprovals != 2 || resubmitted.Stages[0].ID == cab.Stages[0].ID {
		t.Fatalf("submitted again = %s %+v, %v", resubmitted.State, resubmitted.Stages, err)
	}
	oldStage := cab.Stages[0].ID
	_, err = env.svc.DecideProposal(stdCtx(stdAuth1), cab.ID, domain.DecideStandardChangeProposalRequest{Decision: "approved", StageID: &oldStage})
	expectErr[*apierror.ConflictError](t, err, "a decision for the CAB stage, sent after the proposal was submitted again")
	if !strings.Contains(err.Error(), "the stage you decided on is not the one this proposal now waits on") {
		t.Fatalf("stale-stage refusal = %v", err)
	}
	if got, _ := env.svc.GetProposal(stdCtx(stdProposer), cab.ID); got.Stages[0].Approvals != 0 {
		t.Fatalf("a stale decision was recorded: %+v", got.Stages)
	}
	if got := authorise(t, env, cab.ID); got.Template == nil {
		t.Fatalf("approved without a template: %+v", got)
	}

	// The UPDATEs as a role row-level security applies to.
	appDSN, ok := dsns["app"]
	if !ok {
		return
	}
	// Without FORCE row-level security on the proposal (migration 100026) the
	// app role's run would find the rows with or without 0295's app.is_internal
	// bracket, and prove nothing about it.
	var forced bool
	if err := env.scoped.QueryRow(sys, `SELECT relforcerowsecurity FROM pg_class WHERE oid = 'standard_change_proposal'::regclass`).Scan(&forced); err != nil {
		t.Fatalf("read standard_change_proposal's row-level security: %v", err)
	}
	if !forced {
		t.Skip("standard_change_proposal is not FORCE row-level secured on this database (apply migrations/100026_standard_change_rls.sql): " +
			"the app role's run of 0295 would not exercise its app.is_internal bracket")
	}
	appPool, err := pgxpool.New(context.Background(), appDSN)
	if err != nil {
		t.Fatalf("connect as the app role: %v", err)
	}
	t.Cleanup(appPool.Close)
	exec("relax the state check", `ALTER TABLE standard_change_proposal DROP CONSTRAINT IF EXISTS standard_change_proposal_state_check`)
	again := inCAB(stdTag + " Waiting on the CAB, row-level security")
	// The app role, on a session nobody flagged internal, sees no proposal: what
	// 0295's UPDATEs find below, they find through its bracket.
	var seen int
	if err := appPool.QueryRow(context.Background(), `SELECT COUNT(*) FROM standard_change_proposal WHERE id = $1::uuid`, again.ID).Scan(&seen); err != nil || seen != 0 {
		t.Fatalf("the app role sees %d proposal(s) unflagged (%v): row-level security is not in force for it", seen, err)
	}
	updatesOnly := regexp.MustCompile(`(?s)ALTER TABLE standard_change_proposal [^;]*;`).ReplaceAllString(string(migration), "")
	if strings.Contains(updatesOnly, "ALTER TABLE") || !strings.Contains(updatesOnly, "UPDATE standard_change_proposal") {
		t.Fatalf("could not take the ALTERs out of 0295:\n%s", updatesOnly)
	}
	run(appPool, "0295's updates as the app role", updatesOnly)
	checkReturned("as the app role", again)
	run(env.pool, "0295 again", string(migration))
}

// stdRunSQL runs a migration file on one connection of pool and checks it left
// app.is_internal cleared on that session.
func stdRunSQL(t *testing.T, pool *pgxpool.Pool, what, sql string) {
	t.Helper()
	conn, err := pool.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	if _, err := conn.Exec(context.Background(), sql); err != nil {
		t.Fatalf("%s: %v", what, err)
	}
	var flag string
	if err := conn.QueryRow(context.Background(), `SELECT COALESCE(current_setting('app.is_internal', true), '')`).Scan(&flag); err != nil || flag != "" {
		t.Fatalf("%s: app.is_internal = %q afterwards (%v), want it cleared", what, flag, err)
	}
}

// Migration 0297 has the CAB authorise proposals. A proposal still waiting on
// the Standard Change Authority (its open stage labelled so) goes back to draft:
// submitted_on cleared, customer_locked kept, its stage and approver rows gone
// (an approval already given with them), so a clean draft that can be deleted;
// submitted again, it goes to the CAB. A proposal already submitted to the CAB,
// and a decided one, are left as they are. The authority group's description
// says it is no longer used, reworded only while it reads as 0295 left it.
// Running it twice changes nothing more. It writes no DDL, so it also runs, whole,
// as STANDARD_CHANGE_TEST_APP_DSN's role (no BYPASSRLS: FORCE row-level security
// applies) on a session nobody flagged internal, and must still find the rows.
func TestStandardChangeIntegration_CABAuthorisesMigration(t *testing.T) {
	dsns := stdDSNs(t)
	ownerDSN, ok := dsns["owner"]
	if !ok {
		t.Skip("STANDARD_CHANGE_TEST_DSN not set")
	}
	migration, err := os.ReadFile("../../migrations/0297_standard_change_cab_authorises.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	env := newStdEnv(t, ownerDSN)
	sys := repository.WithSystemIdentity(context.Background())
	exec := func(what, sql string, args ...any) {
		t.Helper()
		if _, err := env.scoped.Exec(sys, sql, args...); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
	}
	const authorityGroupID = "00000000-0000-4000-8000-0000000005ca"
	const description0295 = "Authorises standard change templates: two members must approve every new template, revision and retirement, and only a member may resume a suspended template or clear a review flag."
	var savedDescription *string
	if err := env.scoped.QueryRow(sys, `SELECT description FROM "group" WHERE id = $1::uuid`, authorityGroupID).Scan(&savedDescription); err != nil {
		t.Fatalf("read the authority group (migration 0287): %v", err)
	}
	t.Cleanup(func() {
		if _, err := env.scoped.Exec(sys, `UPDATE "group" SET description = $2 WHERE id = $1::uuid`, authorityGroupID, savedDescription); err != nil {
			t.Errorf("restore the authority group's description: %v", err)
		}
	})
	description := func() string {
		t.Helper()
		var d *string
		if err := env.scoped.QueryRow(sys, `SELECT description FROM "group" WHERE id = $1::uuid`, authorityGroupID).Scan(&d); err != nil {
			t.Fatal(err)
		}
		return stringOr(d)
	}

	// A proposal submitted to the authority, as the code before 0297 left it:
	// its one stage labelled "Standard Change Authority", two approvals needed.
	toAuthority := func(name string) domain.StandardChangeProposal {
		t.Helper()
		p := readyDraft(t, env, name)
		if _, err := env.svc.SubmitProposal(stdCtx(stdProposer), p.ID); err != nil {
			t.Fatal(err)
		}
		exec("label the stage as the authority's", `UPDATE approval_stage SET checkpoint_label = 'Standard Change Authority' WHERE work_item_id = $1::uuid`, p.ID)
		got, err := env.svc.GetProposal(stdCtx(stdProposer), p.ID)
		if err != nil || len(got.Stages) != 1 || got.Stages[0].Label != "Standard Change Authority" || got.State != domain.StandardChangeProposalStateAuthorityReview {
			t.Fatalf("authority review fixture = %s %+v, %v", got.State, got.Stages, err)
		}
		return got
	}
	waiting := toAuthority(stdTag + " Waiting on the authority")
	if _, err := env.svc.DecideProposal(stdCtx(stdAuth1), waiting.ID, atStage(t, env, waiting.ID, domain.DecideStandardChangeProposalRequest{Decision: "approved"})); err != nil {
		t.Fatalf("one authorisation before the migration: %v", err)
	}
	decided := toAuthority(stdTag + " Decided by the authority")
	authorise(t, env, decided.ID)
	atCAB := readyDraft(t, env, stdTag+" Already at the CAB")
	if _, err := env.svc.SubmitProposal(stdCtx(stdProposer), atCAB.ID); err != nil {
		t.Fatal(err)
	}
	exec("the description 0295 left", `UPDATE "group" SET description = $2 WHERE id = $1::uuid`, authorityGroupID, description0295)

	checkReturned := func(what string, before domain.StandardChangeProposal) {
		t.Helper()
		got, err := env.svc.GetProposal(stdCtx(stdProposer), before.ID)
		if err != nil || got.State != domain.StandardChangeProposalStateDraft || got.SubmittedOn != nil || !got.CanEdit || !got.CanSubmit || !got.CanDelete {
			t.Fatalf("%s: returned = %s submittedOn %v canEdit %v canSubmit %v canDelete %v, %v",
				what, got.State, got.SubmittedOn, got.CanEdit, got.CanSubmit, got.CanDelete, err)
		}
		if len(got.Stages) != 0 || len(got.Approvers) != 0 {
			t.Fatalf("%s: the authority's stage is still there: %+v %+v", what, got.Stages, got.Approvers)
		}
		var locked bool
		var by string
		if err := env.scoped.QueryRow(sys, `SELECT p.customer_locked, w.updated_by FROM standard_change_proposal p JOIN work_item w ON w.id = p.id WHERE p.id = $1::uuid`,
			before.ID).Scan(&locked, &by); err != nil || !locked || by != "migration:0297_standard_change_cab_authorises" {
			t.Fatalf("%s: customer_locked %v, updated_by %q, %v", what, locked, by, err)
		}
	}
	checkLeftAlone := func(what string) {
		t.Helper()
		if got, _ := env.svc.GetProposal(stdCtx(stdProposer), atCAB.ID); got.State != domain.StandardChangeProposalStateAuthorityReview ||
			len(got.Stages) != 1 || got.Stages[0].Label != domain.CABApprovalGroupName || approverStates(got)[stdAuth1] != "REQUESTED" {
			t.Fatalf("%s: a proposal already at the CAB was touched: %s %+v %v", what, got.State, got.Stages, approverStates(got))
		}
		if got, _ := env.svc.GetProposal(stdCtx(stdProposer), decided.ID); got.State != domain.StandardChangeProposalStateApproved ||
			len(got.Stages) != 1 || got.Stages[0].Label != "Standard Change Authority" || got.Stages[0].Approvals != 2 {
			t.Fatalf("%s: a decided proposal was touched: %s %+v", what, got.State, got.Stages)
		}
	}
	for run := 1; run <= 2; run++ {
		stdRunSQL(t, env.pool, fmt.Sprintf("run %d of 0297", run), string(migration))
		checkReturned(fmt.Sprintf("run %d", run), waiting)
		checkLeftAlone(fmt.Sprintf("run %d", run))
		if d := description(); !strings.HasPrefix(d, "No longer used") || !strings.Contains(d, "CAB Approval") {
			t.Fatalf("run %d: the authority group's description = %q", run, d)
		}
	}
	// So is 0287's own wording, which an earlier draft of 0295 left; someone's
	// own wording is left alone.
	exec("the description 0287 wrote", `UPDATE "group" SET description = $2 WHERE id = $1::uuid`, authorityGroupID,
		"Authorises standard change templates after the CAB: two members must approve every new template, revision and retirement, and only a member may resume a suspended template or clear a review flag.")
	stdRunSQL(t, env.pool, "0297 over 0287's description", string(migration))
	if d := description(); !strings.HasPrefix(d, "No longer used") {
		t.Fatalf("0287's description was not reworded: %q", d)
	}
	exec("an operator's description", `UPDATE "group" SET description = 'Kept by ops' WHERE id = $1::uuid`, authorityGroupID)
	stdRunSQL(t, env.pool, "0297 over an operator's description", string(migration))
	if d := description(); d != "Kept by ops" {
		t.Fatalf("an operator's description was reworded: %q", d)
	}

	// Submitted again, the returned draft goes to the CAB; the approval given
	// before is gone with the old stage, so two CAB members authorise it afresh.
	resubmitted, err := env.svc.SubmitProposal(stdCtx(stdProposer), waiting.ID)
	if err != nil || resubmitted.State != domain.StandardChangeProposalStateAuthorityReview || len(resubmitted.Stages) != 1 ||
		resubmitted.Stages[0].Label != domain.CABApprovalGroupName || resubmitted.Stages[0].RequiredApprovals != 2 || resubmitted.Stages[0].Approvals != 0 {
		t.Fatalf("submitted again = %s %+v, %v", resubmitted.State, resubmitted.Stages, err)
	}
	oldStage := waiting.Stages[0].ID
	_, err = env.svc.DecideProposal(stdCtx(stdAuth2), waiting.ID, domain.DecideStandardChangeProposalRequest{Decision: "approved", StageID: &oldStage})
	expectErr[*apierror.ConflictError](t, err, "a decision for the authority's stage, sent after the proposal went to the CAB")
	if got := authorise(t, env, waiting.ID); got.Template == nil {
		t.Fatalf("approved without a template: %+v", got)
	}

	// The whole file as a role row-level security applies to.
	appDSN, ok := dsns["app"]
	if !ok {
		return
	}
	var forced bool
	if err := env.scoped.QueryRow(sys, `SELECT relforcerowsecurity FROM pg_class WHERE oid = 'standard_change_proposal'::regclass`).Scan(&forced); err != nil {
		t.Fatalf("read standard_change_proposal's row-level security: %v", err)
	}
	if !forced {
		t.Skip("standard_change_proposal is not FORCE row-level secured on this database (apply migrations/100026_standard_change_rls.sql): " +
			"the app role's run of 0297 would not exercise its app.is_internal bracket")
	}
	appPool, err := pgxpool.New(context.Background(), appDSN)
	if err != nil {
		t.Fatalf("connect as the app role: %v", err)
	}
	t.Cleanup(appPool.Close)
	again := toAuthority(stdTag + " Waiting on the authority, row-level security")
	var seen int
	if err := appPool.QueryRow(context.Background(), `SELECT COUNT(*) FROM standard_change_proposal WHERE id = $1::uuid`, again.ID).Scan(&seen); err != nil || seen != 0 {
		t.Fatalf("the app role sees %d proposal(s) unflagged (%v): row-level security is not in force for it", seen, err)
	}
	stdRunSQL(t, appPool, "0297 as the app role", string(migration))
	checkReturned("as the app role", again)
	checkLeftAlone("as the app role")
}
