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
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// The harness of the customer's-proposed-time tests: a customer proposes a start
// (customer_updated_on), WSO2 answers it (customer_updated_date_confirmation) --
// the previous system's own conversation, which the synced schema carries. Same crFlow
// harness as every TestChangeRequest*Integration_* test (DSN-gated by
// CHANGE_REQUEST_TEST_DSN, run as a superuser and as the non-superuser csm_app).
// The planned window of every fixture is rsStart1 .. rsEnd1, two hours long, so a
// proposal of start S is the window S .. S + planLength.

// planLength is the length of the fixtures' planned window (rsStart1 .. rsEnd1).
const planLength = 2 * time.Hour

// endFor is the end a proposal of start keeps the planned length for.
func endFor(start string) string {
	t, err := time.Parse(time.RFC3339, start)
	if err != nil {
		panic(err)
	}
	return t.Add(planLength).UTC().Format(time.RFC3339)
}

// reachCustomerApproval creates a change of the type on project A (contacts Alice
// and Bob), planned rsStart1 .. rsEnd1, and drives it to Customer Approval through
// the real flow. An Emergency change cannot be driven there: it takes no customer step
// (it carries no box, and is scheduled by its CAB approval), so asking for one is a bug in
// the test.
func (f *crFlow) reachCustomerApproval(typ domain.ChangeRequestType) string {
	f.t.Helper()
	if typ == domain.ChangeRequestTypeEmergency {
		f.t.Fatal("an Emergency change never reaches Customer Approval")
	}
	id := f.createWithProject(typ, sp(crScopeProjectA), true, false)
	f.setPlanned(id, rsStart1, rsEnd1)
	switch typ {
	case domain.ChangeRequestTypeNormal:
		f.driveToCustomerApproval(id)
	default:
		f.requestApproval(id)
	}
	f.expect(id, "in Customer Approval", "CUSTOMER_APPROVAL", "authorize", "canceled")
	return id
}

// proposeAs is the customer's proposal of a start, the way the portal sends it.
func (f *crFlow) proposeAs(id, userID, start string) (domain.ChangeRequest, error) {
	return f.patchAsContact(id, userID, domain.PatchChangeRequestRequest{PlannedStartOn: sp(start)})
}

// mustPropose is proposeAs that fails the test when it is refused.
func (f *crFlow) mustPropose(id, userID, start string) domain.ChangeRequest {
	f.t.Helper()
	cr, err := f.proposeAs(id, userID, start)
	if err != nil {
		f.t.Fatalf("%s proposing %s: %v", userID, start, err)
	}
	return cr
}

// proposalOf is the conversation as WSO2 reads it (nil when there is none).
func (f *crFlow) proposalOf(id string) *domain.ChangeRequestCustomerProposal {
	f.t.Helper()
	return f.get(id).CustomerProposal
}

func (f *crFlow) wantAnswer(id, when, want string) {
	f.t.Helper()
	got := "none"
	if p := f.proposalOf(id); p != nil {
		got = p.Answer
	}
	if got != want {
		f.t.Fatalf("customerProposal.answer %s = %q, want %q", when, got, want)
	}
}

// conversation reads the two columns straight from the table: the proposed start
// (RFC 3339 UTC, "" when NULL) and the answer ("" when NULL).
func (f *crFlow) conversation(id string) (on, confirmation string) {
	f.t.Helper()
	if err := f.scoped.QueryRow(f.sys, `
		SELECT COALESCE(to_char(customer_updated_on AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'), ''),
		       COALESCE(customer_updated_date_confirmation::text, '')
		FROM change_request WHERE id = $1`, id).Scan(&on, &confirmation); err != nil {
		f.t.Fatalf("read the conversation columns: %v", err)
	}
	return on, confirmation
}

func (f *crFlow) wantConversation(id, when, wantOn, wantConfirmation string) {
	f.t.Helper()
	on, conf := f.conversation(id)
	if on != wantOn || conf != wantConfirmation {
		f.t.Fatalf("customer_updated_on / confirmation %s = %q / %q, want %q / %q", when, on, conf, wantOn, wantConfirmation)
	}
}

// syncWritesConversation writes the pair directly, as csm-sync-service does for a
// proposal made (or answered) in the previous system: a date and an answer ("" = none).
func (f *crFlow) syncWritesConversation(id string, on *string, confirmation string) {
	f.t.Helper()
	f.execSQL(`UPDATE change_request SET customer_updated_on = $2::text::timestamptz,
	                  customer_updated_date_confirmation = NULLIF($3::text, '')::change_request_confirmation_enum
	           WHERE id = $1`, id, on, confirmation)
}

// crSyncStamp is what the sync loader writes as work_item.updated_by (and created_by) on the rows
// it mirrors: the last writer of a migrated change request that nobody at WSO2 or at a customer touched.
const crSyncStamp = "sn-sync"

// syncWritesConversationAs is syncWritesConversation by the named writer (a WSO2 user in the
// previous system, mirrored by the sync): the work_item is stamped with them first, as the sync
// stamps it, so they are the change's last writer -- the only thing that names a proposer.
func (f *crFlow) syncWritesConversationAs(id, writer string, on *string, confirmation string) {
	f.t.Helper()
	f.execSQL(`UPDATE work_item SET updated_by = $2 WHERE id = $1`, id, writer)
	f.syncWritesConversation(id, on, confirmation)
}

// customerWroteProposal leaves the change request as the proposal of the start on by the
// registered contact userID would -- for a date the API itself refuses (in the past, out of
// range), which the tests need on a change whose proposer is recorded: the contact is the last
// writer, as the API leaves it after a proposal, and the date is written afterwards.
func (f *crFlow) customerWroteProposal(id, userID, on string) {
	f.t.Helper()
	f.execSQL(`UPDATE work_item SET updated_by = $2 WHERE id = $1`, id, crFlowEmail(userID))
	f.syncWritesConversation(id, sp(on), "")
}

// onlyTheProposerIsLeft makes the change one nobody can be asked about, while the registered
// contact proposerID -- who proposed the time that waits -- stays registered (an answer is about a
// RECORDED proposer, and a contact who left is no longer one): every other registered contact of
// project A is deactivated, and the proposer is made the requester (who is never asked about their
// own change). The returned function puts both back.
func (f *crFlow) onlyTheProposerIsLeft(id, proposerID string) (restore func()) {
	f.t.Helper()
	rows, err := f.scoped.Query(f.sys, `SELECT id::text FROM project_contact
	                                      WHERE project_id = $1 AND state = 'REGISTERED' AND LOWER(email) <> LOWER($2)`, crScopeProjectA, crFlowEmail(proposerID))
	if err != nil {
		f.t.Fatalf("list the other contacts: %v", err)
	}
	var others []string
	for rows.Next() {
		var cid string
		if err := rows.Scan(&cid); err != nil {
			rows.Close()
			f.t.Fatalf("scan: %v", err)
		}
		others = append(others, cid)
	}
	rows.Close()
	var requester *string
	if err := f.scoped.QueryRow(f.sys, `SELECT requested_by_user_id::text FROM change_request WHERE id = $1`, id).Scan(&requester); err != nil {
		f.t.Fatalf("read the requester: %v", err)
	}
	f.execSQL(`UPDATE project_contact SET state = 'DEACTIVATED'::project_contact_state_enum WHERE id = ANY($1::uuid[])`, others)
	f.execSQL(`UPDATE change_request SET requested_by_user_id = $2::uuid WHERE id = $1`, id, proposerID)
	return func() {
		f.t.Helper()
		f.execSQL(`UPDATE project_contact SET state = 'REGISTERED'::project_contact_state_enum WHERE id = ANY($1::uuid[])`, others)
		f.execSQL(`UPDATE change_request SET requested_by_user_id = $2::uuid WHERE id = $1`, id, requester)
	}
}

// staffPatch is a PATCH by the staff member userID (an internal identity).
func (f *crFlow) staffPatch(id, userID string, req domain.PatchChangeRequestRequest) (domain.ChangeRequest, error) {
	return f.repo.PatchChangeRequest(f.sys, id, req, crFlowEmail(userID))
}

// acceptReq is the "Accept proposed time" request built from what the page read:
// the proposal and the planned window as the detail shows them.
func (f *crFlow) acceptReq(id string) domain.PatchChangeRequestRequest {
	f.t.Helper()
	cr := f.get(id)
	if cr.CustomerProposal == nil {
		f.t.Fatalf("no customerProposal on the change request to accept")
	}
	return domain.PatchChangeRequestRequest{
		ConfirmCustomerUpdatedDate: sp("agree"),
		ExpectedCustomerUpdatedOn:  sp(cr.CustomerProposal.StartOn),
		ExpectedPlannedStartOn:     cr.PlannedStartOn,
		ExpectedPlannedEndOn:       cr.PlannedEndOn,
	}
}

func (f *crFlow) accept(id string) (domain.ChangeRequest, error) {
	f.t.Helper()
	return f.patch(id, f.acceptReq(id))
}

func (f *crFlow) mustAccept(id string) domain.ChangeRequest {
	f.t.Helper()
	cr, err := f.accept(id)
	if err != nil {
		f.t.Fatalf("Accept proposed time: %v", err)
	}
	return cr
}

// counterReq is "Propose a different time" / a decline built from what the page
// read: {state: authorize, the window (nil = keep the plan), the proposal and the
// planned window the page showed}.
func (f *crFlow) counterReq(id string, start, end *string) domain.PatchChangeRequestRequest {
	f.t.Helper()
	cr := f.get(id)
	req := domain.PatchChangeRequestRequest{
		State:                  stateptr(domain.ChangeRequestStateAuthorize),
		PlannedStartOn:         start,
		PlannedEndOn:           end,
		ExpectedPlannedStartOn: cr.PlannedStartOn,
		ExpectedPlannedEndOn:   cr.PlannedEndOn,
	}
	if cr.CustomerProposal != nil && cr.CustomerProposal.Answer == "pending" {
		req.ExpectedCustomerUpdatedOn = sp(cr.CustomerProposal.StartOn)
	}
	return req
}

func (f *crFlow) counter(id string, start, end *string) error {
	f.t.Helper()
	_, err := f.patch(id, f.counterReq(id, start, end))
	return err
}

// snap renders everything an act must leave consistent when it is refused: the
// state, the window, both conversation columns, the hold, the last writer, every
// stage with each approver's state.
func (f *crFlow) snap(id string) string {
	f.t.Helper()
	cr := f.get(id)
	on, conf := f.conversation(id)
	alias := map[string]string{crScopeUserA1: "alice", crScopeUserA2: "bob"}
	var parts []string
	parts = append(parts, f.state(id))
	parts = append(parts, fmt.Sprintf("window=%v..%v", derefStr(cr.PlannedStartOn), derefStr(cr.PlannedEndOn)))
	parts = append(parts, fmt.Sprintf("proposal=%q answer=%q", on, conf))
	parts = append(parts, fmt.Sprintf("hold=%v", cr.OnHold != nil && *cr.OnHold))
	parts = append(parts, "by="+f.updatedBy(id))
	for _, st := range f.stages(id) {
		var rows []string
		for uid, status := range st.approvers {
			name := alias[uid]
			if name == "" {
				name = uid[len(uid)-4:]
			}
			rows = append(rows, name+"="+status)
		}
		sort.Strings(rows)
		parts = append(parts, st.label+"{"+strings.Join(rows, ",")+"}")
	}
	return strings.Join(parts, " | ")
}

func derefStr(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}

// rowCounts is the number of rows of every table of the public schema.
func (f *crFlow) rowCounts() map[string]int {
	f.t.Helper()
	rows, err := f.scoped.Query(f.sys, `SELECT c.relname FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
	                                      WHERE n.nspname = 'public' AND c.relkind IN ('r', 'p') ORDER BY 1`)
	if err != nil {
		f.t.Fatalf("list tables: %v", err)
	}
	var tables []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			rows.Close()
			f.t.Fatalf("scan table: %v", err)
		}
		tables = append(tables, t)
	}
	rows.Close()
	out := map[string]int{}
	for _, t := range tables {
		var n int
		if err := f.scoped.QueryRow(f.sys, fmt.Sprintf(`SELECT COUNT(*) FROM %q`, t)).Scan(&n); err != nil {
			f.t.Fatalf("count %s: %v", t, err)
		}
		out[t] = n
	}
	return out
}

// rowDelta is what an act added to (or removed from) each table, tables with no
// change left out.
func rowDelta(before, after map[string]int) map[string]int {
	out := map[string]int{}
	for t, n := range after {
		if d := n - before[t]; d != 0 {
			out[t] = d
		}
	}
	for t, n := range before {
		if _, ok := after[t]; !ok && n != 0 {
			out[t] = -n
		}
	}
	return out
}

func fmtDelta(d map[string]int) string {
	var parts []string
	for t, n := range d {
		parts = append(parts, fmt.Sprintf("%s%+d", t, n))
	}
	sort.Strings(parts)
	return "{" + strings.Join(parts, ", ") + "}"
}

// wantDelta asserts the act's row delta is exactly want.
func wantDelta(t *testing.T, what string, before, after map[string]int, want map[string]int) {
	t.Helper()
	if got := rowDelta(before, after); fmtDelta(got) != fmtDelta(want) {
		t.Fatalf("%s added %s, want exactly %s", what, fmtDelta(got), fmtDelta(want))
	}
}

// seedMigratedStage inserts an approval stage the way csm-sync-service writes one: NO
// label, the group's own (nil: none), the status in the sync's vocabulary (upper case,
// which is also what an enum-typed column accepts), created ageMinutes ago, one approver row
// per entry of rows (user id -> state).
func (f *crFlow) seedMigratedStage(id string, groupID *string, ageMinutes int, rows map[string]string) string {
	f.t.Helper()
	var stageID string
	if err := f.scoped.QueryRow(f.sys,
		`INSERT INTO approval_stage (id, created_on, updated_on, created_by, updated_by, work_item_id, assignment_group_id, raw_status)
		 VALUES (gen_random_uuid(), now() - make_interval(mins => $2::int), now(), 'sn-sync', 'sn-sync', $1, $3::uuid, 'REQUESTED') RETURNING id::text`,
		id, ageMinutes, groupID).Scan(&stageID); err != nil {
		f.t.Fatalf("seed a migrated stage: %v", err)
	}
	for uid, state := range rows {
		f.execSQL(`INSERT INTO approval_stage_approver (id, created_on, updated_on, created_by, updated_by, stage_id, work_item_id, approver_user_id, state)
		           VALUES (gen_random_uuid(), now(), now(), 'sn-sync', 'sn-sync', $1::uuid, $2, $3::uuid, $4)`, stageID, id, uid, state)
	}
	return stageID
}

// migratedInCustomerApproval is a change in Customer Approval the way the sync leaves
// one: planned rsStart1 .. rsEnd1, the previous system's customer group on it
// (customer_group_id), our own boxes false, an UNLABELED stage in that group asking Alice
// and Bob, a creator that is staff. It carries no proposal.
func (f *crFlow) migratedInCustomerApproval() string {
	f.t.Helper()
	id := f.createWithProject(domain.ChangeRequestTypeNormal, sp(crScopeProjectA), false, false)
	f.setPlanned(id, rsStart1, rsEnd1)
	f.setState(id, "CUSTOMER_APPROVAL")
	f.execSQL(`UPDATE change_request SET customer_group_id = $2::uuid, customer_approval_required = false,
	                  is_customer_approval_required = false WHERE id = $1`, id, crFlowGroupID)
	g := crFlowGroupID
	f.seedMigratedStage(id, &g, 40, map[string]string{crScopeUserA1: "REQUESTED", crScopeUserA2: "REQUESTED"})
	return id
}

// wantConflictExact asserts the act failed with a 409 saying exactly msg.
func (f *crFlow) wantConflictExact(what string, err error, msg string) {
	f.t.Helper()
	var ce *apierror.ConflictError
	if !errors.As(err, &ce) {
		f.t.Fatalf("%s: err = %v (%T), want a *apierror.ConflictError %q", what, err, err, msg)
	}
	if ce.Msg != msg {
		f.t.Fatalf("%s: message = %q, want %q", what, ce.Msg, msg)
	}
}

// wantRefused asserts the act failed and that the change request is exactly as it was.
func (f *crFlow) wantRefusedSame(what, id, before string, err error) {
	f.t.Helper()
	if err == nil {
		f.t.Fatalf("%s: accepted, want a refusal", what)
	}
	if after := f.snap(id); after != before {
		f.t.Fatalf("%s: a refused act changed the change request:\n  before: %s\n  after:  %s", what, before, after)
	}
}

// giveParent hangs the change request under a parent record of project A (the service request
// a change is raised under in the previous system), optionally linked to a GitHub issue of an active
// repository of the account -- the two things the existing parity triggers write
// beside the change itself (the plan-start-date comment on the parent, the GitHub outbound
// queue). It returns the parent's id; everything it inserts is removed with the test.
func (f *crFlow) giveParent(id string, project string, linked bool) string {
	f.t.Helper()
	const parent = "3aaaaaaa-0000-0000-0000-0000000000c1"
	var issue any
	if linked {
		issue = 4242
	}
	f.execSQL(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, subject, type, wso2_id, project_id, account_id, github_issue_number)
	           VALUES ($1::uuid, now(), now(), 'cr-flow-test', 'cr-flow-test', 'CRFLOWPARENT1', $4, 'SERVICE_REQUEST', 'cr-flow-parent', $2::uuid, $3::uuid, $5)`,
		parent, project, crScopeAccountID, crFlowSubject, issue)
	f.execSQL(`UPDATE work_item SET parent_id = $2::uuid WHERE id = $1`, id, parent)
	if linked {
		f.execSQL(`INSERT INTO account_github_repo (id, created_on, updated_on, created_by, updated_by, account_id, owner, repository, is_active)
		           VALUES (gen_random_uuid(), now(), now(), 'cr-flow-test', 'cr-flow-test', $1::uuid, 'example-org', 'example-repo', true)`, crScopeAccountID)
		f.t.Cleanup(func() {
			_, _ = f.scoped.Exec(f.sys, `DELETE FROM account_github_repo WHERE owner = 'example-org' AND repository = 'example-repo'`)
			_, _ = f.scoped.Exec(f.sys, `DELETE FROM github_outbound_queue WHERE owner = 'example-org'`)
		})
	}
	return parent
}

// commentsOn lists the content of the comments on the work item, oldest first.
func (f *crFlow) commentsOn(id string) []string {
	f.t.Helper()
	rows, err := f.scoped.Query(f.sys, `SELECT created_by || ': ' || content FROM comment WHERE work_item_id = $1 ORDER BY created_on, id`, id)
	if err != nil {
		f.t.Fatalf("list comments: %v", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			f.t.Fatalf("scan comment: %v", err)
		}
		out = append(out, c)
	}
	return out
}
