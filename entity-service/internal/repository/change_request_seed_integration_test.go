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
	"os"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// The change request approval personas and fixtures of the local compose seed
// (scripts/csm-compose/seed-entity-service.sql): three WSO2 (INTERNAL) people
// who approve the internal stages, two customers (EXTERNAL) who are the
// registered contacts of project 401 and answer the customer stages, and the
// CHG-FIXED-* change requests built on them. Same harness as the other
// ChangeRequestFlowIntegration tests (CHANGE_REQUEST_TEST_DSN); every test here
// is skipped when the database was not built from the seed.

const (
	seedAliceID = "00000000-0000-0000-0000-000000000011"
	seedBobID   = "00000000-0000-0000-0000-000000000012"
	seedCarolID = "00000000-0000-0000-0000-000000000013"
	seedDaveID  = "00000000-0000-0000-0000-000000000021"
	seedErinID  = "00000000-0000-0000-0000-000000000022"
	seedJaneID  = "00000000-0000-0000-0000-000000000001"
	seedJohnID  = "00000000-0000-0000-0000-000000000002"

	seedProject401 = "00000000-0000-0000-0000-000000000401"
	seedProject402 = "00000000-0000-0000-0000-000000000402"

	seedCR001 = "00000000-0000-0000-0000-000000001001"
	seedCR002 = "00000000-0000-0000-0000-000000001002"
	seedCR003 = "00000000-0000-0000-0000-000000001003"
	seedCR004 = "00000000-0000-0000-0000-000000001004"
	seedCR005 = "00000000-0000-0000-0000-000000001201"
	seedCR006 = "00000000-0000-0000-0000-000000001202"
	seedCR007 = "00000000-0000-0000-0000-000000001303"
	seedCR008 = "00000000-0000-0000-0000-000000001304"

	seedSQLPath = "../../../scripts/csm-compose/seed-entity-service.sql"
)

// newSeededFlow is a crFlow on the seeded database, WITHOUT newCRFlow's
// isolation of the CAB / ECAB / Devops Approval groups: these tests are about
// the seeded members of those groups. Skipped when the personas are not there.
func newSeededFlow(t *testing.T) *crFlow {
	t.Helper()
	f := newCRFlowNoIsolation(t)
	var n int
	if err := f.scoped.QueryRow(f.sys,
		`SELECT COUNT(*) FROM "user" WHERE id = ANY($1::uuid[])`,
		[]string{seedAliceID, seedBobID, seedCarolID, seedDaveID, seedErinID}).Scan(&n); err != nil {
		t.Fatalf("look for the seeded personas: %v", err)
	}
	if n != 5 {
		t.Skipf("the change request approval personas are not seeded (found %d of 5)", n)
	}
	return f
}

// seededStages reads the stage/approver rows of a fixture by id.
func (f *crFlow) seededStages(id string) map[string]map[string]string {
	f.t.Helper()
	out := map[string]map[string]string{}
	for _, st := range f.stages(id) {
		out[st.label] = st.approvers
	}
	return out
}

func (f *crFlow) groupNamesOf(userID string) string {
	f.t.Helper()
	rows, err := f.scoped.Query(f.sys,
		`SELECT g.name FROM team_member tm JOIN "group" g ON g.id = tm.group_id WHERE tm.user_id = $1 ORDER BY g.name`, userID)
	if err != nil {
		f.t.Fatalf("group names of %s: %v", userID, err)
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			f.t.Fatalf("scan: %v", err)
		}
		names = append(names, n)
	}
	return strings.Join(names, ",")
}

func (f *crFlow) roleNamesOf(userID string) string {
	f.t.Helper()
	rows, err := f.scoped.Query(f.sys,
		`SELECT r.name FROM user_role ur JOIN role r ON r.id = ur.role_id WHERE ur.user_id = $1 ORDER BY r.name`, userID)
	if err != nil {
		f.t.Fatalf("roles of %s: %v", userID, err)
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			f.t.Fatalf("scan: %v", err)
		}
		names = append(names, n)
	}
	return strings.Join(names, ",")
}

// The personas: who they are, what the roles made of them, and which groups and
// project they belong to. jane.doe and john.smith keep their rows but hold none
// of the approver / contact seats any more.
func TestChangeRequestSeedIntegration_Personas(t *testing.T) {
	f := newSeededFlow(t)

	for _, p := range []struct{ id, email, wantType, wantRole string }{
		{seedAliceID, "alice.perera@example.com", "INTERNAL", "internal"},
		{seedBobID, "bob.fernando@example.com", "INTERNAL", "internal"},
		{seedCarolID, "carol.silva@example.com", "INTERNAL", "internal"},
		{seedDaveID, "dave.mendis@example.com", "EXTERNAL", "customer"},
		{seedErinID, "erin.jayawardena@example.com", "EXTERNAL", "customer"},
	} {
		var email, typ string
		var active bool
		if err := f.scoped.QueryRow(f.sys, `SELECT email, user_type::text, COALESCE(is_active, true) FROM "user" WHERE id = $1`, p.id).Scan(&email, &typ, &active); err != nil {
			t.Fatalf("read %s: %v", p.email, err)
		}
		if email != p.email || typ != p.wantType || !active {
			t.Errorf("%s = (%s, %s, active=%v), want (%s, %s, active)", p.id, email, typ, active, p.email, p.wantType)
		}
		if got := f.roleNamesOf(p.id); got != p.wantRole {
			t.Errorf("%s roles = %q, want %q", p.email, got, p.wantRole)
		}
	}

	// Group membership: the internal personas sit in the assigned group and in
	// the two approval groups that are resolved (CAB, which an Emergency change uses
	// too, and the Devops Approval peer fallback); the customers in none of them. The
	// ECAB group (unused: the previous system has no Emergency CAB) is seeded empty.
	const internalGroups = "CAB Approval,Devops Approval,Example Corp ABT"
	for _, id := range []string{seedAliceID, seedBobID, seedCarolID} {
		if got := f.groupNamesOf(id); got != internalGroups {
			t.Errorf("groups of %s = %q, want %q", id, got, internalGroups)
		}
	}
	for _, id := range []string{seedDaveID, seedErinID} {
		if got := f.groupNamesOf(id); got != "" {
			t.Errorf("groups of the customer %s = %q, want none", id, got)
		}
	}
	// jane.doe is out of every group (still a team member); john.smith stays in
	// the assigned group only -- the standing external-member probe.
	if got := f.groupNamesOf(seedJaneID); got != "" {
		t.Errorf("groups of jane.doe = %q, want none", got)
	}
	if got := f.groupNamesOf(seedJohnID); got != "Example Corp ABT" {
		t.Errorf("groups of john.smith = %q, want only Example Corp ABT", got)
	}
	// The trap the seed warns about: no customer ever holds the internal role.
	var n int
	if err := f.scoped.QueryRow(f.sys,
		`SELECT COUNT(*) FROM user_role ur JOIN role r ON r.id = ur.role_id
		 WHERE r.name = 'internal' AND ur.user_id = ANY($1::uuid[])`, []string{seedJohnID, seedDaveID, seedErinID}).Scan(&n); err != nil || n != 0 {
		t.Errorf("customers holding the internal role = %d (%v), want 0", n, err)
	}

	// Project 401's registered contacts are the two customers; project 402
	// keeps its own (Sam Other); jane and john are no project contacts.
	contacts := func(project string) string {
		rows, err := f.scoped.Query(f.sys,
			`SELECT pc.email FROM project_contact pc WHERE pc.project_id = $1 AND pc.state = 'REGISTERED' ORDER BY pc.email`, project)
		if err != nil {
			t.Fatalf("contacts of %s: %v", project, err)
		}
		defer rows.Close()
		var emails []string
		for rows.Next() {
			var e string
			if err := rows.Scan(&e); err != nil {
				t.Fatalf("scan: %v", err)
			}
			emails = append(emails, e)
		}
		return strings.Join(emails, ",")
	}
	if got := contacts(seedProject401); got != "dave.mendis@example.com,erin.jayawardena@example.com" {
		t.Errorf("registered contacts of project 401 = %q", got)
	}
	if got := contacts(seedProject402); got != "sam.other@othercorp.example" {
		t.Errorf("registered contacts of project 402 = %q", got)
	}
}

// The CHG-FIXED-* fixtures in their starting state: which stages exist and who
// holds which row. (A database a spec has already driven forward reads
// differently: re-run the seed -- `docker-compose up -d migrate` -- to reset.)
func TestChangeRequestSeedIntegration_FixtureApprovers(t *testing.T) {
	f := newSeededFlow(t)
	const reset = " (re-run the seed to reset the fixtures)"

	for _, id := range []string{seedCR001, seedCR002, seedCR005, seedCR006} {
		if got := f.labels(id); len(got) != 0 {
			t.Errorf("fixture %s has stages %v, want none%s", id, got, reset)
		}
	}
	peer := f.seededStages(seedCR003)
	if len(peer) != 1 {
		t.Fatalf("CHG-FIXED-003 stages = %v, want one Peer Approval stage%s", f.labels(seedCR003), reset)
	}
	assertApprovers(t, "CHG-FIXED-003 peer"+reset, peer["Peer Approval"], map[string]string{
		seedAliceID: "REQUESTED", seedBobID: "REQUESTED", seedCarolID: "REQUESTED"})
	assertApprovers(t, "CHG-FIXED-004 peer"+reset, f.seededStages(seedCR004)["Peer Approval"], map[string]string{
		seedAliceID: "APPROVED", seedBobID: "CANCELLED", seedCarolID: "CANCELLED"})
	assertApprovers(t, "CHG-FIXED-007 customer approval"+reset, f.seededStages(seedCR007)["Customer Approval"], map[string]string{
		seedDaveID: "REQUESTED", seedErinID: "REQUESTED"})
	assertApprovers(t, "CHG-FIXED-008 customer review"+reset, f.seededStages(seedCR008)["Customer Review"], map[string]string{
		seedDaveID: "REQUESTED", seedErinID: "REQUESTED"})

	// jane.doe and john.smith hold no approver row on any fixture.
	var n int
	if err := f.scoped.QueryRow(f.sys,
		`SELECT COUNT(*) FROM approval_stage_approver WHERE approver_user_id = ANY($1::uuid[]) AND work_item_id = ANY($2::uuid[])`,
		[]string{seedJaneID, seedJohnID},
		[]string{seedCR001, seedCR002, seedCR003, seedCR004, seedCR005, seedCR006, seedCR007, seedCR008}).Scan(&n); err != nil || n != 0 {
		t.Errorf("jane/john approver rows on the fixtures = %d (%v), want 0", n, err)
	}

	// The unattended states of the others.
	for _, tc := range []struct{ id, state string }{
		{seedCR001, "NEW"}, {seedCR002, "NEW"}, {seedCR003, "ASSESS"}, {seedCR004, "AUTHORIZE"},
		{seedCR005, "NEW"}, {seedCR006, "REVIEW"}, {seedCR007, "CUSTOMER_APPROVAL"}, {seedCR008, "CUSTOMER_REVIEW"},
	} {
		if got := f.state(tc.id); got != tc.state {
			t.Errorf("fixture %s state = %q, want %q%s", tc.id, got, tc.state, reset)
		}
	}
}

// The seeded assigned group (Example Corp ABT, which holds john.smith, a
// customer, alongside the three internal personas): Request Approval on a
// change assigned to it provisions exactly alice, bob and carol, never john;
// and the CAB group seeded with the personas carries the change on to
// Scheduled, for a Normal change and for an Emergency one alike. Driven end to
// end on the real seed.
func TestChangeRequestSeedIntegration_AssignedGroupProvisionsOnlyTheInternalPersonas(t *testing.T) {
	f := newSeededFlow(t)
	create := func(typ domain.ChangeRequestType) string {
		t.Helper()
		g := seededGroupID
		resp, err := f.repo.CreateChangeRequest(f.sys, domain.CreateChangeRequestRequest{Subject: crFlowSubject, Type: &typ, GroupID: &g}, "jane.doe@example.com")
		if err != nil {
			t.Fatalf("CreateChangeRequest(%s): %v", typ, err)
		}
		// jane.doe, the requester persona, is the creator.
		f.execSQL(`UPDATE change_request SET requested_by_user_id = $1::uuid WHERE id = $2`, seedJaneID, resp.ChangeRequest.ID)
		return resp.ChangeRequest.ID
	}

	id := create(domain.ChangeRequestTypeNormal)
	if _, err := f.repo.PatchChangeRequest(f.sys, id, domain.PatchChangeRequestRequest{State: stateptr(domain.ChangeRequestStateAssess)}, "jane.doe@example.com"); err != nil {
		t.Fatalf("Request Approval: %v", err)
	}
	f.wantPeerPool(id, seededGroupID, map[string]string{seedAliceID: "REQUESTED", seedBobID: "REQUESTED", seedCarolID: "REQUESTED"})

	// john.smith (external, in the group) cannot decide even with a row forced
	// in; alice can, and the seeded CAB group takes over.
	f.forceApprover(id, "Peer Approval", seedJohnID)
	f.wantForbidden("john.smith on the peer stage", f.decide(id, seedJohnID, "approved"), "only active internal (WSO2) users")
	if err := f.decide(id, seedAliceID, "approved"); err != nil {
		t.Fatalf("alice peer approval: %v", err)
	}
	f.expect(id, "after alice approved", "AUTHORIZE", "canceled")
	cab := f.stage(id, "CAB Approval")
	if cab.groupID != crCABGroupID {
		t.Fatalf("CAB stage group = %s, want the CAB group", cab.groupID)
	}
	assertApprovers(t, "CAB", cab.approvers, map[string]string{seedAliceID: "REQUESTED", seedBobID: "REQUESTED", seedCarolID: "REQUESTED"})
	if err := f.decide(id, seedBobID, "approved"); err != nil {
		t.Fatalf("bob CAB approval: %v", err)
	}
	f.expect(id, "after bob approved", "SCHEDULED", "implement", "canceled")

	// Emergency: the same seeded CAB group, one stage and no peer stage.
	eid := create(domain.ChangeRequestTypeEmergency)
	if _, err := f.repo.PatchChangeRequest(f.sys, eid, domain.PatchChangeRequestRequest{State: stateptr(domain.ChangeRequestStateAssess)}, "jane.doe@example.com"); err != nil {
		t.Fatalf("Emergency Request Approval: %v", err)
	}
	if got := f.labels(eid); strings.Join(got, ",") != "CAB Approval" {
		t.Fatalf("Emergency stages = %v, want the one CAB Approval stage", got)
	}
	ecab := f.stage(eid, "CAB Approval")
	if ecab.groupID != crCABGroupID {
		t.Fatalf("Emergency CAB stage group = %s, want the CAB group", ecab.groupID)
	}
	assertApprovers(t, "Emergency CAB", ecab.approvers, map[string]string{seedAliceID: "REQUESTED", seedBobID: "REQUESTED", seedCarolID: "REQUESTED"})
	if err := f.decide(eid, seedCarolID, "approved"); err != nil {
		t.Fatalf("carol Emergency CAB approval: %v", err)
	}
	f.expect(eid, "after carol approved", "SCHEDULED", "implement", "canceled")
}

// The seeded Devops Approval group makes the peer fallback exercisable locally:
// a change assigned to a group that yields nobody eligible -- here the seeded
// Castor team, whose members carry no team_member.group_id -- gets the Devops
// Approval members (alice, bob, carol) as its peer approvers.
func TestChangeRequestSeedIntegration_DevopsApprovalFallback(t *testing.T) {
	f := newSeededFlow(t)
	var castor string
	if err := f.scoped.QueryRow(f.sys,
		`SELECT g.id::text FROM "group" g WHERE g.name = 'Castor'
		   AND NOT EXISTS (SELECT 1 FROM team_member tm WHERE tm.group_id = g.id)`).Scan(&castor); err != nil {
		t.Skipf("no member-less seeded Castor group (seed-team-schedule.sql not loaded?): %v", err)
	}
	typ := domain.ChangeRequestTypeNormal
	resp, err := f.repo.CreateChangeRequest(f.sys, domain.CreateChangeRequestRequest{Subject: crFlowSubject, Type: &typ, GroupID: &castor}, "jane.doe@example.com")
	if err != nil {
		t.Fatalf("CreateChangeRequest: %v", err)
	}
	id := resp.ChangeRequest.ID
	f.execSQL(`UPDATE change_request SET requested_by_user_id = $1::uuid WHERE id = $2`, seedJaneID, id)
	if _, err := f.repo.PatchChangeRequest(f.sys, id, domain.PatchChangeRequestRequest{State: stateptr(domain.ChangeRequestStateAssess)}, "jane.doe@example.com"); err != nil {
		t.Fatalf("Request Approval: %v", err)
	}
	st := f.stage(id, "Peer Approval")
	if st.groupID == castor || st.groupID == "" {
		t.Fatalf("peer stage group = %q, want the Devops Approval group", st.groupID)
	}
	var name string
	if err := f.scoped.QueryRow(f.sys, `SELECT name FROM "group" WHERE id = $1`, st.groupID).Scan(&name); err != nil || name != domain.PeerApprovalFallbackGroupName {
		t.Fatalf("peer stage group name = %q (%v), want %q", name, err, domain.PeerApprovalFallbackGroupName)
	}
	assertApprovers(t, "fallback peer stage", st.approvers, map[string]string{seedAliceID: "REQUESTED", seedBobID: "REQUESTED", seedCarolID: "REQUESTED"})
}

// Re-running the seed converges a database seeded BEFORE the personas (jane.doe
// and john.smith as the approvers and the project's contacts) to the current
// fixtures, and resets fixtures a spec has driven forward -- twice over, with
// the same result. Simulated inside one transaction that is rolled back, so the
// database is left exactly as it was found.
func TestChangeRequestSeedIntegration_SeedIsSelfHealing(t *testing.T) {
	f := newSeededFlow(t)
	raw, err := os.ReadFile(seedSQLPath)
	if err != nil {
		t.Skipf("seed file not readable from here (%v)", err)
	}
	// The seed is its own transaction; run it inside this one instead.
	var kept []string
	for _, line := range strings.Split(string(raw), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed == "BEGIN;" || trimmed == "COMMIT;" {
			continue
		}
		kept = append(kept, line)
	}
	seedSQL := strings.Join(kept, "\n")

	ctx := context.Background()
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	mustExec := func(sql string, args ...any) {
		t.Helper()
		if _, err := tx.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("exec (%.70s): %v", sql, err)
		}
	}
	scalar := func(sql string, args ...any) string {
		t.Helper()
		var out string
		if err := tx.QueryRow(ctx, sql, args...).Scan(&out); err != nil {
			t.Fatalf("query (%.70s): %v", sql, err)
		}
		return out
	}

	// 1. Put the database back into the OLD seed's shape (as the user's was):
	//    jane/john are registered contacts of project 401 and approvers of the
	//    fixtures and CAB members; the new fixture rows hold the wrong people.
	mustExec(`INSERT INTO project_contact (id, created_on, updated_on, created_by, updated_by, email, state, account_contact_id, project_id)
	          VALUES ('00000000-0000-0000-0000-000000001421', now(), now(), 'seed', 'seed', 'jane.doe@example.com', 'REGISTERED', '00000000-0000-0000-0000-000000001411', $1),
	                 ('00000000-0000-0000-0000-000000001422', now(), now(), 'seed', 'seed', 'john.smith@example.com', 'REGISTERED', '00000000-0000-0000-0000-000000001412', $1)
	          ON CONFLICT (id) DO NOTHING`, seedProject401)
	mustExec(`INSERT INTO project_contact_group (id, created_on, updated_on, created_by, updated_by, project_contact_id, project_group_id)
	          SELECT c.id, now(), now(), 'seed', 'seed', c.contact, pg.id
	          FROM (VALUES ('00000000-0000-0000-0000-000000001431'::uuid, '00000000-0000-0000-0000-000000001421'::uuid),
	                       ('00000000-0000-0000-0000-000000001432'::uuid, '00000000-0000-0000-0000-000000001422'::uuid)) AS c(id, contact), project_group pg
	          WHERE pg."group" = 'General Access' ON CONFLICT (id) DO NOTHING`)
	mustExec(`DELETE FROM approval_stage_approver WHERE work_item_id IN ($1, $2, $3)`, seedCR003, seedCR007, seedCR008)
	mustExec(`INSERT INTO approval_stage_approver (id, created_on, updated_on, created_by, updated_by, stage_id, work_item_id, approver_user_id, state)
	          VALUES (gen_random_uuid(), now(), now(), 'seed', 'seed', '00000000-0000-0000-0000-000000001005', $1, $4, 'REQUESTED'),
	                 (gen_random_uuid(), now(), now(), 'seed', 'seed', '00000000-0000-0000-0000-000000001005', $1, $5, 'REQUESTED'),
	                 (gen_random_uuid(), now(), now(), 'seed', 'seed', '00000000-0000-0000-0000-000000001305', $2, $4, 'REQUESTED'),
	                 (gen_random_uuid(), now(), now(), 'seed', 'seed', '00000000-0000-0000-0000-000000001305', $2, $5, 'REQUESTED'),
	                 (gen_random_uuid(), now(), now(), 'seed', 'seed', '00000000-0000-0000-0000-000000001306', $3, $4, 'REQUESTED')`,
		seedCR003, seedCR007, seedCR008, seedJaneID, seedJohnID)
	mustExec(`INSERT INTO team_member (id, created_on, updated_on, created_by, updated_by, team_id, user_id, group_id)
	          VALUES ('00000000-0000-0000-0000-000000001101', now(), now(), 'seed', 'seed', '00000000-0000-0000-0000-000000000901', $1, $3),
	                 ('00000000-0000-0000-0000-000000001102', now(), now(), 'seed', 'seed', '00000000-0000-0000-0000-000000000901', $2, $3)
	          ON CONFLICT (id) DO NOTHING`, seedJaneID, seedJohnID, crCABGroupID)
	mustExec(`UPDATE team_member SET group_id = '00000000-0000-0000-0000-000000000901' WHERE id = '00000000-0000-0000-0000-000000000902'`)
	// ...and a spec has driven fixtures forward: CHG-FIXED-007 was approved,
	// CHG-FIXED-003 cascaded to Authorize with a CAB stage and a stamp.
	mustExec(`UPDATE change_request SET state = 'SCHEDULED', is_customer_approval_required = true WHERE id = $1`, seedCR007)
	mustExec(`UPDATE change_request SET state = 'AUTHORIZE', approval = 'REQUESTED' WHERE id = $1`, seedCR003)
	mustExec(`INSERT INTO approval_stage (id, created_on, updated_on, created_by, updated_by, work_item_id, assignment_group_id, checkpoint_label)
	          VALUES (gen_random_uuid(), now(), now(), 'seed', 'seed', $1, $2, 'CAB Approval')`, seedCR003, crCABGroupID)

	if got := scalar(`SELECT COUNT(*)::text FROM project_contact WHERE project_id = $1 AND state = 'REGISTERED'`, seedProject401); got != "4" {
		t.Fatalf("simulated old state has %s contacts on project 401, want 4", got)
	}

	// 2. Run the seed -- twice -- and read the result back each time.
	check := func(pass string) {
		t.Helper()
		if got := scalar(`SELECT string_agg(email, ',' ORDER BY email) FROM project_contact WHERE project_id = $1 AND state = 'REGISTERED'`, seedProject401); got != "dave.mendis@example.com,erin.jayawardena@example.com" {
			t.Errorf("%s: contacts of project 401 = %q", pass, got)
		}
		for _, tc := range []struct{ id, want string }{
			{seedCR003, "Peer Approval:alice.perera=REQUESTED,bob.fernando=REQUESTED,carol.silva=REQUESTED"},
			{seedCR004, "Peer Approval:alice.perera=APPROVED,bob.fernando=CANCELLED,carol.silva=CANCELLED"},
			{seedCR007, "Customer Approval:dave.mendis=REQUESTED,erin.jayawardena=REQUESTED"},
			{seedCR008, "Customer Review:dave.mendis=REQUESTED,erin.jayawardena=REQUESTED"},
		} {
			got := scalar(`SELECT string_agg(ast.checkpoint_label || ':' ||
			                 (SELECT string_agg(split_part(u.email, '@', 1) || '=' || asa.state, ',' ORDER BY u.email)
			                    FROM approval_stage_approver asa JOIN "user" u ON u.id = asa.approver_user_id WHERE asa.stage_id = ast.id),
			               ' | ' ORDER BY ast.created_on, ast.id)
			               FROM approval_stage ast WHERE ast.work_item_id = $1`, tc.id)
			if got != tc.want {
				t.Errorf("%s: stages of %s = %q, want %q", pass, tc.id, got, tc.want)
			}
		}
		if got := scalar(`SELECT state::text || '/' || COALESCE(is_customer_approval_required::text, '-') || '/' || COALESCE(approval::text, '-') FROM change_request WHERE id = $1`, seedCR007); got != "CUSTOMER_APPROVAL/-/-" {
			t.Errorf("%s: CHG-FIXED-007 = %q, want back at CUSTOMER_APPROVAL with no stamp", pass, got)
		}
		if got := scalar(`SELECT state::text || '/' || COALESCE(approval::text, '-') FROM change_request WHERE id = $1`, seedCR003); got != "ASSESS/-" {
			t.Errorf("%s: CHG-FIXED-003 = %q, want back at ASSESS", pass, got)
		}
		if got := scalar(`SELECT COUNT(*)::text FROM team_member WHERE user_id = ANY($1::uuid[]) AND group_id IS NOT NULL
		                    AND group_id <> '00000000-0000-0000-0000-000000000901'`, []string{seedJaneID, seedJohnID}); got != "0" {
			t.Errorf("%s: jane/john hold %s approval-group seats, want 0", pass, got)
		}
		if got := scalar(`SELECT (group_id IS NULL)::text FROM team_member WHERE id = '00000000-0000-0000-0000-000000000902'`); got != "true" {
			t.Errorf("%s: jane.doe is still in the assigned group (group_id IS NULL = %s)", pass, got)
		}
	}
	mustExec(seedSQL)
	check("first run")
	mustExec(seedSQL)
	check("second run")
}

// TestChangeRequestSeedIntegration_LumenWorksPlatformContacts: the seed registers two customer
// contacts (mira.santos, noel.prasad) on the generated project "Lumen Works Platform", found by
// NAME because its id is random per database. They must meet the Customer Group criteria
// (REGISTERED, PORTAL_USER, active customer user) so a change request on that project is put
// to them, never to another customer's contacts. Where no such project exists the block does
// nothing. Runs inside a rolled-back transaction, twice (idempotent).
func TestChangeRequestSeedIntegration_LumenWorksPlatformContacts(t *testing.T) {
	f := newSeededFlow(t)
	raw, err := os.ReadFile(seedSQLPath)
	if err != nil {
		t.Skipf("seed file not readable from here (%v)", err)
	}
	var kept []string
	for _, line := range strings.Split(string(raw), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed == "BEGIN;" || trimmed == "COMMIT;" {
			continue
		}
		kept = append(kept, line)
	}
	seedSQL := strings.Join(kept, "\n")

	ctx := context.Background()
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	mustExec := func(sql string, args ...any) {
		t.Helper()
		if _, err := tx.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("exec (%.70s): %v", sql, err)
		}
	}
	// The Customer Group derivation (customerContactsSQL in change_request_links.go, not
	// exported to this package): REGISTERED + PORTAL_USER role + not a deactivated user.
	contactNames := func(projectID string) string {
		t.Helper()
		var names string
		if err := tx.QueryRow(ctx, `
			SELECT COALESCE(string_agg(n, ',' ORDER BY n), '') FROM (
				SELECT COALESCE(NULLIF(TRIM(u.name), ''), pc.email) AS n
				FROM project_contact pc
				JOIN account_contact ac ON ac.id = pc.account_contact_id
				LEFT JOIN "user" u ON LOWER(u.user_name) = LOWER(ac.user_name)
				WHERE pc.project_id = $1::uuid
				  AND pc.state = 'REGISTERED'::project_contact_state_enum
				  AND COALESCE(u.is_active, true)
				  AND EXISTS (
				      SELECT 1
				      FROM project_contact_group pcg
				      JOIN project_group_role pgr ON pgr.project_group_id = pcg.project_group_id
				      JOIN project_role pr ON pr.id = pgr.project_role_id
				      WHERE pcg.project_contact_id = pc.id AND pr.role = 'PORTAL_USER'::project_role_enum)
			) c`, projectID).Scan(&names); err != nil {
			t.Fatalf("derive customer contacts: %v", err)
		}
		return names
	}

	// A database that already has a generated project of that name must not
	// confuse the test: set it aside inside this transaction.
	mustExec(`UPDATE project SET name = 'Lumen Works Platform (set aside by the test)' WHERE name = 'Lumen Works Platform'`)

	// A database seeded earlier (the user's) already holds the Lumen contacts: clear
	// them inside this transaction so the "no project" step starts from nothing.
	mustExec(`DELETE FROM project_contact_group WHERE id IN ('00000000-0000-0000-0000-000000001436', '00000000-0000-0000-0000-000000001437')`)
	mustExec(`DELETE FROM project_contact WHERE id IN ('00000000-0000-0000-0000-000000001426', '00000000-0000-0000-0000-000000001427')`)
	mustExec(`DELETE FROM account_contact WHERE id IN ('00000000-0000-0000-0000-000000001416', '00000000-0000-0000-0000-000000001417')`)

	// 1. No such project: the block is a no-op, and the contacts exist nowhere.
	mustExec(seedSQL)
	var n int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM project_contact WHERE id IN ('00000000-0000-0000-0000-000000001426', '00000000-0000-0000-0000-000000001427')`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("with no Lumen Works Platform project, %d Lumen contacts were created, want 0", n)
	}

	// 2. The generator has created the project (random id, with an account): the next seed run registers them.
	const acct, proj = "00000000-0000-0000-0000-00000000aa01", "00000000-0000-0000-0000-00000000aa02"
	mustExec(`INSERT INTO account (id, created_on, updated_on, created_by, updated_by, name, number, sf_id, country, city)
	          VALUES ($1, now(), now(), 'seed', 'seed', 'Lumen Works', 'ACC-LUMEN-T', 'SF-LUMEN-T', 'Sri Lanka', 'Colombo')`, acct)
	mustExec(`INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, sf_id, name, account_id, is_active)
	          VALUES ($1, now(), now(), 'seed', 'seed', 'LUMEN-T', 'SF-PROJ-LUMEN-T', 'Lumen Works Platform', $2, true)`, proj, acct)
	// A generated contact that does not qualify (no user row, no PORTAL_USER role) stays out.
	mustExec(`INSERT INTO account_contact (id, created_on, updated_on, created_by, updated_by, is_active, user_name, account_id)
	          VALUES ('00000000-0000-0000-0000-00000000aa03', now(), now(), 'gen', 'gen', true, 'Gen Contact <gen@example.net>', $1)`, acct)
	mustExec(`INSERT INTO project_contact (id, created_on, updated_on, created_by, updated_by, email, state, account_contact_id, project_id)
	          VALUES ('00000000-0000-0000-0000-00000000aa04', now(), now(), 'gen', 'gen', 'gen@example.net', 'REGISTERED', '00000000-0000-0000-0000-00000000aa03', $1)`, proj)

	for _, pass := range []string{"first run", "second run"} {
		mustExec(seedSQL)
		if got := contactNames(proj); got != "Mira Santos,Noel Prasad" {
			t.Errorf("%s: Lumen Works Platform customer contacts = %q, want \"Mira Santos,Noel Prasad\"", pass, got)
		}
		var types string
		if err := tx.QueryRow(ctx, `SELECT string_agg(user_type::text, ',' ORDER BY email) FROM "user" WHERE id IN ('00000000-0000-0000-0000-000000000023', '00000000-0000-0000-0000-000000000024')`).Scan(&types); err != nil {
			t.Fatal(err)
		}
		if types != "EXTERNAL,EXTERNAL" {
			t.Errorf("%s: Mira and Noel are %q, want customers (EXTERNAL,EXTERNAL)", pass, types)
		}
		// Isolation: Example Corp's project still only has its own contacts.
		if got := contactNames(seedProject401); got != "Dave Mendis,Erin Jayawardena" {
			t.Errorf("%s: project 401 contacts = %q, want Dave and Erin only", pass, got)
		}
	}
}

// The project types of the local project fixtures (fixtures/0031_project_type_table.sql).
const (
	seedTypeManagedCloudID = "00000000-0000-0000-0000-0000000000a1"
	seedTypeEvaluationID   = "00000000-0000-0000-0000-0000000000a2"
	seedTypeSubscriptionID = "00000000-0000-0000-0000-0000000000a3"
	seedTypeCloudSupportID = "00000000-0000-0000-0000-0000000000a4"
)

// TestChangeRequestSeedIntegration_CustomerPortalEntitlements: the customer portal offers
// its Operations menu (service requests, change requests) only when GET /projects/{id}/features
// -- read from the project's project_type -- grants read access to them. The seed must therefore
// leave the local "Subscription" type (the type of projects 401 / 402) granting both, and put
// "Lumen Works Platform" (whose type the generator picks at random) on a type that does, while
// touching no other flag and no other type. It must also correct a database that was seeded
// BEFORE this existed (flags still FALSE), which an ON CONFLICT DO NOTHING insert would not.
// Runs in a rolled-back transaction, the seed twice per scenario (idempotent).
func TestChangeRequestSeedIntegration_CustomerPortalEntitlements(t *testing.T) {
	f := newSeededFlow(t)
	raw, err := os.ReadFile(seedSQLPath)
	if err != nil {
		t.Skipf("seed file not readable from here (%v)", err)
	}
	var kept []string
	for _, line := range strings.Split(string(raw), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed == "BEGIN;" || trimmed == "COMMIT;" {
			continue
		}
		kept = append(kept, line)
	}
	seedSQL := strings.Join(kept, "\n")

	ctx := context.Background()
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	mustExec := func(sql string, args ...any) {
		t.Helper()
		if _, err := tx.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("exec (%.70s): %v", sql, err)
		}
	}
	scalar := func(sql string, args ...any) string {
		t.Helper()
		var out string
		if err := tx.QueryRow(ctx, sql, args...).Scan(&out); err != nil {
			t.Fatalf("query (%.70s): %v", sql, err)
		}
		return out
	}
	// What GET /projects/{id}/features reads for the project (reference_data_repo.go's
	// GetProjectByID joins the same columns), as "type:change-request-read/service-request-read".
	features := func(projectID string) string {
		t.Helper()
		return scalar(`SELECT pt.name || ':' || pt.has_change_request_read_access::text || '/' || pt.has_service_request_read_access::text
		               FROM project p JOIN project_type pt ON pt.id = p.project_type_id WHERE p.id = $1`, projectID)
	}
	// Every flag of every project type EXCEPT the two the seed owns on a3, so any collateral change shows.
	collateral := func() string {
		t.Helper()
		return scalar(`SELECT string_agg(id::text || '=' || name || ':' ||
		                 has_service_request_write_access::text || has_sra_write_access::text || has_sra_read_access::text ||
		                 has_engagements_read_access::text || has_updates_read_access::text || has_deployment_write_access::text ||
		                 has_deployment_read_access::text || has_time_logs_read_access::text || has_component_analysis_read_access::text ||
		                 has_usage_metrics_read_access::text ||
		                 CASE WHEN id = $1::uuid THEN '' ELSE ':' || has_service_request_read_access::text || has_change_request_read_access::text END,
		               ';' ORDER BY id) FROM project_type`, seedTypeSubscriptionID)
	}

	// A database seeded BEFORE the entitlements existed: every flag of the local type is FALSE.
	mustExec(`UPDATE project_type SET has_change_request_read_access = FALSE, has_service_request_read_access = FALSE WHERE id = $1`, seedTypeSubscriptionID)
	if got := features(seedProject401); got != "Subscription:false/false" {
		t.Fatalf("simulated old state: project 401 features = %q, want Subscription:false/false", got)
	}
	// The set-aside / generated "Lumen Works Platform", on a type that grants nothing ("Cloud Support").
	mustExec(`UPDATE project SET name = 'Lumen Works Platform (set aside by the test)' WHERE name = 'Lumen Works Platform'`)
	const acct, proj = "00000000-0000-0000-0000-00000000bb01", "00000000-0000-0000-0000-00000000bb02"
	mustExec(`INSERT INTO account (id, created_on, updated_on, created_by, updated_by, name, number, sf_id, country, city)
	          VALUES ($1, now(), now(), 'seed', 'seed', 'Lumen Works', 'ACC-LUMEN-E', 'SF-LUMEN-E', 'Sri Lanka', 'Colombo')`, acct)
	mustExec(`INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, sf_id, name, account_id, is_active, project_type_id)
	          VALUES ($1, now(), now(), 'seed', 'seed', 'LUMEN-E', 'SF-PROJ-LUMEN-E', 'Lumen Works Platform', $2, true, $3)`, proj, acct, seedTypeCloudSupportID)
	if got := features(proj); got != "Cloud Support:false/false" {
		t.Fatalf("simulated generated state: Lumen features = %q, want Cloud Support:false/false", got)
	}
	before := collateral()

	// 1. Seed twice: the local type grants both, Lumen is moved off the type that grants nothing,
	//    nothing else about any project type changed.
	for _, pass := range []string{"first run", "second run"} {
		mustExec(seedSQL)
		for _, id := range []string{seedProject401, seedProject402} {
			if got := features(id); got != "Subscription:true/true" {
				t.Errorf("%s: features of project %s = %q, want Subscription:true/true", pass, id, got)
			}
		}
		if got := features(proj); got != "Subscription:true/true" {
			t.Errorf("%s: features of Lumen Works Platform = %q, want it moved to Subscription:true/true", pass, got)
		}
		if got := collateral(); got != before {
			t.Errorf("%s: another project type flag changed.\n before: %s\n after:  %s", pass, before, got)
		}
		// The types that grant nothing locally still grant nothing; the one that grants everything still does.
		for _, tc := range []struct{ id, want string }{
			{seedTypeEvaluationID, "false/false"},
			{seedTypeCloudSupportID, "false/false"},
			{seedTypeManagedCloudID, "true/true"},
		} {
			if got := scalar(`SELECT has_change_request_read_access::text || '/' || has_service_request_read_access::text FROM project_type WHERE id = $1`, tc.id); got != tc.want {
				t.Errorf("%s: project type %s = %s, want %s", pass, tc.id, got, tc.want)
			}
		}
	}

	// 2. A Lumen Works Platform already on a type that grants Operations is left on it (not forced to a3).
	mustExec(`UPDATE project SET project_type_id = $2 WHERE id = $1`, proj, seedTypeManagedCloudID)
	mustExec(seedSQL)
	if got := features(proj); got != "Managed Cloud Subscription:true/true" {
		t.Errorf("Lumen on Managed Cloud Subscription = %q after the seed, want it left as Managed Cloud Subscription:true/true", got)
	}

	// 3. Only the local fixture row is ever touched: a "Subscription" type that came from elsewhere
	//    (a synced environment's own row) keeps whatever it has.
	mustExec(`UPDATE project_type SET has_change_request_read_access = FALSE, has_service_request_read_access = FALSE, created_by = 'servicenow-sync' WHERE id = $1`, seedTypeSubscriptionID)
	mustExec(seedSQL)
	if got := scalar(`SELECT has_change_request_read_access::text || '/' || has_service_request_read_access::text FROM project_type WHERE id = $1`, seedTypeSubscriptionID); got != "false/false" {
		t.Errorf("a non-fixture Subscription type = %s after the seed, want it untouched (false/false)", got)
	}
}
