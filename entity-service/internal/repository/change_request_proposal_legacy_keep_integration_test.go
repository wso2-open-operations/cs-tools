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
	"encoding/json"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// A time that has no record of its proposer (customer_proposed_for NULL: stored before migration
// 0249, or by a writer that records nobody, such as an older build or the sync) is read by the
// change's last writer. Migration 0250's trigger records that writer, when the rule names them, in
// the write that replaces them, so a later save by anybody else no longer makes the waiting
// proposal read "nobody is recorded". The record is exactly what the rule named at that moment,
// and nothing is recorded where the rule named nobody or nothing waits.

// legacyKeepFlow is a change in Customer Approval holding the stored start rsStart2 with no record,
// written by writer: the legacy shape.
func legacyKeepFlow(t *testing.T, writer string) (*crFlow, string) {
	t.Helper()
	f := newCustomerGroupFlow(t)
	id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
	f.syncWritesConversationAs(id, writer, sp(rsStart2), "")
	f.wantProposalRecord(id, "on a legacy row", "", "")
	return f, id
}

// staffWorkNote is the commonest later save: a staff member's work note.
func (f *crFlow) staffWorkNote(id string) {
	f.t.Helper()
	if _, err := f.staffPatch(id, crFlowPeerBID, domain.PatchChangeRequestRequest{WorkNote: sp("looking at it")}); err != nil {
		f.t.Fatalf("a staff work note: %v", err)
	}
}

func TestChangeRequestProposalIntegration_AWriteKeepsALegacyProposer(t *testing.T) {
	t.Run("every writer that replaces the proposing contact records them first, as the rule named them", func(t *testing.T) {
		patchAs := func(req ...domain.PatchChangeRequestRequest) func(f *crFlow, id string) {
			return func(f *crFlow, id string) {
				f.t.Helper()
				for _, r := range req {
					if _, err := f.staffPatch(id, crFlowPeerBID, r); err != nil {
						f.t.Fatalf("a staff edit: %v", err)
					}
				}
			}
		}
		for _, tc := range []struct {
			name string
			run  func(f *crFlow, id string)
		}{
			{"a staff work note", patchAs(domain.PatchChangeRequestRequest{WorkNote: sp("looking at it")})},
			{"an additional note", patchAs(domain.PatchChangeRequestRequest{Comment: sp("we will get back to you")})},
			{"the hold on and off", patchAs(domain.PatchChangeRequestRequest{OnHold: boolp(true), OnHoldReason: sp("freeze")}, domain.PatchChangeRequestRequest{OnHold: boolp(false)})},
			{"a whole-form resend", patchAs(domain.PatchChangeRequestRequest{Title: sp(crFlowSubject)})},
			// An older build, GitHub, the sync or a manual fix writes the work item directly.
			{"another writer of the work item", func(f *crFlow, id string) {
				f.execSQL(`UPDATE work_item SET updated_on = now(), updated_by = $2 WHERE id = $1`, id, crSyncStamp)
			}},
		} {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				f, id := legacyKeepFlow(t, crFlowEmail(crScopeUserA1))
				before := f.proposalOf(id)
				if before == nil || before.ProposerRecorded == nil || !*before.ProposerRecorded || before.ProposedOn == nil {
					t.Fatalf("customerProposal before the write = %+v, want the last writer named, with when", before)
				}
				tc.run(f, id)
				if got := f.updatedBy(id); got == crFlowEmail(crScopeUserA1) {
					t.Fatalf("work_item.updated_by after the write is still the proposing contact: the case does not test a replacement")
				}
				f.wantProposalRecord(id, "after "+tc.name, crScopeUserA1, rsStart2)
				p := f.proposalOf(id)
				if p == nil || p.Answer != "pending" || p.ProposerRecorded == nil || !*p.ProposerRecorded ||
					p.ProposedByEmail == nil || *p.ProposedByEmail != crFlowEmail(crScopeUserA1) ||
					p.ProposedByName == nil || *p.ProposedByName != "Alice Aaron" ||
					p.ProposedOn == nil || *p.ProposedOn != *before.ProposedOn || p.CanAccept == nil || !*p.CanAccept {
					t.Fatalf("customerProposal after %s = %+v, want Alice recorded as proposing it at %s and Accept possible", tc.name, p, *before.ProposedOn)
				}
				f.mustAccept(id)
				f.wantPlanned(id, "after Accept", rsStart2, endFor(rsStart2))
				f.wantConversation(id, "after Accept", rsStart2, "AGREE")
			})
		}
	})

	t.Run("the writer's email is matched as the rule matches it: trimmed and in any case", func(t *testing.T) {
		f, id := legacyKeepFlow(t, "  "+upperASCII(crFlowEmail(crScopeUserA1))+" ")
		f.staffWorkNote(id)
		f.wantProposalRecord(id, "after a staff work note", crScopeUserA1, rsStart2)
	})

	t.Run("nothing is recorded where the rule names nobody", func(t *testing.T) {
		for _, tc := range []struct{ name, writer string }{
			{"a WSO2 user", "wso2.engineer@example.com"},
			{"the sync's own stamp", crSyncStamp},
			{"nobody (a blank last writer)", ""},
			{"a contact of another project", crFlowEmail(crScopeUserB1)},
		} {
			f, id := legacyKeepFlow(t, tc.writer)
			f.staffWorkNote(id)
			f.wantProposalRecord(id, tc.name+", after a staff work note", "", "")
			if p := f.proposalOf(id); p == nil || p.ProposerRecorded == nil || *p.ProposerRecorded {
				t.Fatalf("%s: customerProposal = %+v, want nobody recorded", tc.name, p)
			}
		}
	})

	t.Run("nothing is recorded for a contact who is not a registered portal user of the project", func(t *testing.T) {
		f, id := legacyKeepFlow(t, crFlowEmail(crScopeUserA2))
		restore := f.setContactState(crScopeProjectA, crFlowEmail(crScopeUserA2), "INVITED")
		defer restore()
		f.staffWorkNote(id)
		f.wantProposalRecord(id, "an invited contact, after a staff work note", "", "")
	})

	t.Run("a registered contact with no user record keeps the old rule: nothing to record them by", func(t *testing.T) {
		const ghost = "ghost.contact@example.com"
		f := newCustomerGroupFlow(t)
		restore := f.setContactEmail(crScopeProjectA, crFlowEmail(crScopeUserA2), ghost)
		defer restore()
		id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
		f.syncWritesConversationAs(id, ghost, sp(rsStart2), "")
		if p := f.proposalOf(id); p == nil || p.ProposerRecorded == nil || !*p.ProposerRecorded {
			t.Fatalf("customerProposal = %+v, want the last-writer rule to name the contact", p)
		}
		f.staffWorkNote(id)
		f.wantProposalRecord(id, "after a staff work note", "", "")
		if p := f.proposalOf(id); p == nil || p.ProposerRecorded == nil || *p.ProposerRecorded {
			t.Fatalf("customerProposal = %+v, want nobody recorded once the contact is no longer the last writer", p)
		}
	})

	t.Run("a writer stored with a tab or a newline is trimmed as the rule trims it", func(t *testing.T) {
		f, id := legacyKeepFlow(t, crFlowEmail(crScopeUserA1)+"\t\n")
		f.staffWorkNote(id)
		f.wantProposalRecord(id, "after a staff work note", crScopeUserA1, rsStart2)
	})

	t.Run("nothing is recorded for a registered contact of the project who is not a portal user", func(t *testing.T) {
		// Sam Security is a REGISTERED contact of project A holding only SECURITY_CONTACT: the rule
		// (callerIsRegisteredPortalContact) names nobody, so the trigger records nobody.
		f, id := legacyKeepFlow(t, crFlowEmail(crScopeUserSecurity))
		if p := f.proposalOf(id); p == nil || p.ProposerRecorded == nil || *p.ProposerRecorded {
			t.Fatalf("customerProposal before the write = %+v, want nobody named (not a portal user)", p)
		}
		f.staffWorkNote(id)
		f.wantProposalRecord(id, "a security-only contact, after a staff work note", "", "")
	})

	t.Run("the contact's user is the earliest of the users that share their email, as the read model names them", func(t *testing.T) {
		for _, tc := range []struct {
			name, dupID, after string
		}{
			// Created later than Alice, with the same address in another case: created_on decides.
			{"a later user", "3bbbbbbb-0000-0000-0000-0000000000d1", "1 day"},
			// Created at the same instant, with a greater id: the id breaks the tie.
			{"a user created at the same instant with a greater id", "3bbbbbbb-0000-0000-0000-0000000000f1", "0"},
		} {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				f, id := legacyKeepFlow(t, crFlowEmail(crScopeUserA1))
				f.execSQL(`INSERT INTO "user" (id, created_on, updated_on, created_by, updated_by, user_name, name, first_name, last_name, email, is_active, is_system_user)
				           VALUES ($1::uuid, (SELECT created_on FROM "user" WHERE id = $4::uuid) + $5::interval, now(), 'cr-scope-test', 'cr-scope-test',
				                   $2, 'Alice Twin', 'First', 'Last', $3, true, false)`,
					tc.dupID, "twin."+crFlowEmail(crScopeUserA1), upperASCII(crFlowEmail(crScopeUserA1)), crScopeUserA1, tc.after)
				f.staffWorkNote(id)
				f.wantProposalRecord(id, "after a staff work note", crScopeUserA1, rsStart2)
				if p := f.proposalOf(id); p == nil || p.ProposedByName == nil || *p.ProposedByName != "Alice Aaron" {
					t.Fatalf("customerProposal = %+v, want Alice Aaron (the earliest user of the address) recorded", p)
				}
			})
		}
	})

	t.Run("a work item that is not typed as a change request records nothing, even over a change request row", func(t *testing.T) {
		// work_item.type and the extension row can disagree on synced data; the trigger acts on
		// change requests only (its WHEN clause), whatever row sits under the id.
		f, id := legacyKeepFlow(t, crFlowEmail(crScopeUserA1))
		f.execSQL(`UPDATE work_item SET type = 'INCIDENT'::work_item_type_enum WHERE id = $1`, id)
		f.execSQL(`UPDATE work_item SET updated_on = now(), updated_by = $2 WHERE id = $1`, id, crSyncStamp)
		f.wantProposalRecord(id, "after another writer of a work item typed INCIDENT", "", "")
	})

	t.Run("nothing is recorded where nothing waits, or a record exists", func(t *testing.T) {
		t.Run("an infinite stored time (another door wrote it)", func(t *testing.T) {
			f := newCustomerGroupFlow(t)
			id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
			f.syncWritesConversationAs(id, crFlowEmail(crScopeUserA1), sp("infinity"), "")
			f.wantProposalRecord(id, "on a legacy row with an infinite time", "", "")
			f.staffWorkNote(id)
			f.wantProposalRecord(id, "an infinite stored time, after a staff work note", "", "")
		})
		t.Run("answered", func(t *testing.T) {
			for _, answer := range []string{"AGREE", "DISAGREE"} {
				f := newCustomerGroupFlow(t)
				id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
				f.syncWritesConversationAs(id, crFlowEmail(crScopeUserA1), sp(rsStart2), answer)
				f.staffWorkNote(id)
				f.wantProposalRecord(id, answer+", after a staff work note", "", "")
			}
		})
		t.Run("the stored time is the planned start", func(t *testing.T) {
			f, id := legacyKeepFlow(t, crFlowEmail(crScopeUserA1))
			f.syncWritesConversation(id, sp(rsStart1), "")
			f.staffWorkNote(id)
			f.wantProposalRecord(id, "after a staff work note", "", "")
		})
		t.Run("no stored time", func(t *testing.T) {
			f := newCustomerGroupFlow(t)
			id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
			f.execSQL(`UPDATE work_item SET updated_by = $2 WHERE id = $1`, id, crFlowEmail(crScopeUserA1))
			f.staffWorkNote(id)
			f.wantProposalRecord(id, "after a staff work note", "", "")
		})
		t.Run("not in Customer Approval", func(t *testing.T) {
			f, id := legacyKeepFlow(t, crFlowEmail(crScopeUserA1))
			f.setState(id, "SCHEDULED")
			f.execSQL(`UPDATE work_item SET updated_by = $2 WHERE id = $1`, id, crSyncStamp)
			f.wantProposalRecord(id, "out of Customer Approval, after another writer", "", "")
		})
		t.Run("the writer does not change", func(t *testing.T) {
			f, id := legacyKeepFlow(t, crFlowEmail(crScopeUserA1))
			f.execSQL(`UPDATE work_item SET updated_on = now(), updated_by = $2 WHERE id = $1`, id, crFlowEmail(crScopeUserA1))
			f.wantProposalRecord(id, "after a write by the same writer", "", "")
		})
		t.Run("a proposal's own record is left as it is", func(t *testing.T) {
			f := newCustomerGroupFlow(t)
			id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
			f.mustPropose(id, crScopeUserA2, rsStart2)
			f.staffWorkNote(id)
			f.wantProposalRecord(id, "after a staff work note", crScopeUserA2, rsStart2)
		})
		t.Run("a record voided on entering Customer Approval stays void", func(t *testing.T) {
			f := newCustomerGroupFlow(t)
			id := f.reachCustomerApproval(domain.ChangeRequestTypeNormal)
			f.mustPropose(id, crScopeUserA1, rsStart2)
			f.setState(id, "AUTHORIZE")
			f.setState(id, "CUSTOMER_APPROVAL")
			f.wantProposalRecord(id, "after re-entering Customer Approval", "", rsStart2)
			f.staffWorkNote(id)
			f.wantProposalRecord(id, "after a staff work note", "", rsStart2)
		})
	})

	t.Run("another contact's proposal over it records that contact for the new time", func(t *testing.T) {
		f, id := legacyKeepFlow(t, crFlowEmail(crScopeUserA1))
		f.mustPropose(id, crScopeUserA2, rsStart3)
		f.wantProposalRecord(id, "after Bob's proposal", crScopeUserA2, rsStart3)
	})

	t.Run("the record leaves one outbox row with the three record columns and nothing else", func(t *testing.T) {
		// The same work note on a change with nothing to keep is the baseline.
		g, gid := legacyKeepFlow(t, "wso2.engineer@example.com")
		before := g.rowCounts()
		g.staffWorkNote(gid)
		baseline := rowDelta(before, g.rowCounts())

		f, id := legacyKeepFlow(t, crFlowEmail(crScopeUserA1))
		before = f.rowCounts()
		f.staffWorkNote(id)
		got := rowDelta(before, f.rowCounts())
		want := map[string]int{}
		for k, v := range baseline {
			want[k] = v
		}
		want["event_outbox"]++
		if fmtDelta(got) != fmtDelta(want) {
			t.Fatalf("a staff work note on a legacy proposal added %s, want the baseline %s plus one outbox row", fmtDelta(got), fmtDelta(baseline))
		}
		var changes []byte
		if err := f.scoped.QueryRow(f.sys, `SELECT changes::text FROM event_outbox WHERE entity_type = 'change_request' AND entity_id = $1
		                                    ORDER BY id DESC LIMIT 1`, id).Scan(&changes); err != nil {
			t.Fatalf("read the outbox row: %v", err)
		}
		var diff map[string]any
		if err := json.Unmarshal(changes, &diff); err != nil {
			t.Fatalf("decode the outbox row: %v", err)
		}
		var cols []string
		for k := range diff {
			cols = append(cols, k)
		}
		sort.Strings(cols)
		if len(cols) != 3 || cols[0] != "customer_proposed_by_user_id" || cols[1] != "customer_proposed_for" || cols[2] != "customer_proposed_on" {
			t.Fatalf("the outbox row's changes = %v, want only the three record columns", cols)
		}
	})
}

// Migration 0250 is safe to run again: the function and the trigger replace themselves, the trigger
// is there once, and it still records. DDL needs the table's owner: under a role that is not one
// (csm_app) the test is skipped.
func TestChangeRequestProposalIntegration_KeepLegacyProposerMigrationIsSafeToRunAgain(t *testing.T) {
	f := newCustomerGroupFlow(t)
	var owner bool
	if err := f.pool.QueryRow(context.Background(),
		`SELECT pg_has_role(current_user, c.relowner, 'USAGE') FROM pg_class c WHERE c.oid = 'work_item'::regclass`).Scan(&owner); err != nil {
		t.Fatalf("read the owner of work_item: %v", err)
	}
	if !owner {
		t.Skip("the connected role does not own work_item, so it cannot run the migration's DDL: connect as its owner")
	}
	sqlBytes, err := os.ReadFile("../../migrations/0250_change_request_keep_legacy_proposer.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	for i := 0; i < 2; i++ {
		if _, err := f.pool.Exec(context.Background(), string(sqlBytes)); err != nil {
			t.Fatalf("re-running migration 0250 (pass %d): %v", i+1, err)
		}
	}
	var triggers int
	if err := f.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM pg_trigger WHERE tgrelid = 'work_item'::regclass AND tgname = 'work_item_keep_legacy_customer_proposer'`).Scan(&triggers); err != nil || triggers != 1 {
		t.Fatalf("keep triggers on work_item after re-running the migration = %d (%v), want 1", triggers, err)
	}
	g, id := legacyKeepFlow(t, crFlowEmail(crScopeUserA1))
	g.staffWorkNote(id)
	g.wantProposalRecord(id, "after re-running the migration and a staff work note", crScopeUserA1, rsStart2)

	// Without 0249's columns the migration refuses to run (its guard), instead of installing a
	// function that would fail every write of a change request that changes its writer. Checked in a
	// transaction that is rolled back.
	sqlText := string(sqlBytes)
	start, end := strings.Index(sqlText, "DO $$"), strings.Index(sqlText, "\n$$;")
	if start < 0 || end < start {
		t.Fatalf("migration 0250 has no DO $$ ... $$; guard")
	}
	guard := sqlText[start : end+len("\n$$;")]
	tx, err := f.pool.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(context.Background(), `ALTER TABLE change_request RENAME COLUMN customer_proposed_on TO customer_proposed_on_gone`); err != nil {
		t.Fatalf("rename a 0249 column: %v", err)
	}
	if _, err := tx.Exec(context.Background(), guard); err == nil || !strings.Contains(err.Error(), "needs migration 0249") {
		t.Fatalf("the guard without 0249's columns = %v, want it to refuse naming migration 0249", err)
	}
}

// setContactState sets the state of the project's contact with the given email, and returns what
// puts it back.
func (f *crFlow) setContactState(projectID, email, state string) (restore func()) {
	f.t.Helper()
	var was string
	if err := f.scoped.QueryRow(f.sys, `SELECT state::text FROM project_contact WHERE project_id = $1 AND LOWER(email) = LOWER($2)`,
		projectID, email).Scan(&was); err != nil {
		f.t.Fatalf("read the contact's state: %v", err)
	}
	f.execSQL(`UPDATE project_contact SET state = $3::project_contact_state_enum WHERE project_id = $1 AND LOWER(email) = LOWER($2)`, projectID, email, state)
	return func() {
		f.t.Helper()
		f.execSQL(`UPDATE project_contact SET state = $3::project_contact_state_enum WHERE project_id = $1 AND LOWER(email) = LOWER($2)`, projectID, email, was)
	}
}

// setContactEmail changes the email the project's contact is registered under (to one no "user"
// has), and returns what puts it back.
func (f *crFlow) setContactEmail(projectID, email, to string) (restore func()) {
	f.t.Helper()
	f.execSQL(`UPDATE project_contact SET email = $3 WHERE project_id = $1 AND LOWER(email) = LOWER($2)`, projectID, email, to)
	return func() {
		f.t.Helper()
		f.execSQL(`UPDATE project_contact SET email = $3 WHERE project_id = $1 AND LOWER(email) = LOWER($2)`, projectID, to, email)
	}
}

// upperASCII upper-cases an email address the way a sync could store it.
func upperASCII(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'a' && c <= 'z' {
			b[i] = c - 'a' + 'A'
		}
	}
	return string(b)
}
