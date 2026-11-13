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
	"sort"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// The CAB's pushback against PostgreSQL: which approval stages are the CAB's
// (labelled and unlabelled ones, as migrated rows have), the rejections and comments
// read from them, the look-alikes, and what a suggestion and a proposal show.
// Same DSNs as the other standard change integration tests.

// cabSeed writes approval stages and approver rows on a change request.
type cabSeed struct {
	t   *testing.T
	s   *repository.Scoped
	sys context.Context
	n   int
}

// stage adds a stage to a work item, created minutesAgo minutes ago (so stages
// are ordered as given), with an optional label and group.
func (c *cabSeed) stage(workItem string, label, group *string) string {
	c.t.Helper()
	c.n++
	var id string
	if err := c.s.QueryRow(c.sys, `INSERT INTO approval_stage (id, created_on, updated_on, created_by, updated_by, work_item_id, assignment_group_id, raw_status, checkpoint_label)
		VALUES (gen_random_uuid(), NOW() - make_interval(days => 900) + make_interval(mins => $4), NOW(), 't', 't', $1::uuid, $2::uuid, 'REQUESTED', $3)
		RETURNING id::text`, workItem, group, label, c.n).Scan(&id); err != nil {
		c.t.Fatalf("seed stage: %v", err)
	}
	return id
}

// row adds an approver row, decided daysAgo days ago.
func (c *cabSeed) row(workItem, stage, user, state, comment string, daysAgo int) {
	c.t.Helper()
	var comments *string
	if comment != "" {
		comments = &comment
	}
	if _, err := c.s.Exec(c.sys, `INSERT INTO approval_stage_approver (id, created_on, updated_on, created_by, updated_by, work_item_id, stage_id, approver_user_id, state, comments)
		VALUES (gen_random_uuid(), NOW() - make_interval(days => $6 + 1), NOW() - make_interval(days => $6), 't', 't', $1::uuid, $2::uuid, $3::uuid, $4::approval_stage_approver_state_enum, $5)`,
		workItem, stage, user, state, comments, daysAgo); err != nil {
		c.t.Fatalf("seed approver row: %v", err)
	}
}

func (c *cabSeed) user(email, name string) string {
	c.t.Helper()
	var id string
	if err := c.s.QueryRow(c.sys, `INSERT INTO "user" (id, created_on, updated_on, created_by, updated_by, user_name, email, name, is_active)
		VALUES (gen_random_uuid(), NOW(), NOW(), 't', 't', $1, $1, $2, true) RETURNING id::text`, email, name).Scan(&id); err != nil {
		c.t.Fatalf("seed user: %v", err)
	}
	return id
}

func cabStr(s string) *string { return &s }

func TestStandardChangeCABHistoryIntegration_Suggestions(t *testing.T) {
	for role, dsn := range stdDSNs(t) {
		t.Run(role, func(t *testing.T) {
			e, svc := newSgsEnv(t, dsn)
			ctx := stdCtx(sgsStaff)
			c := &cabSeed{t: t, s: e.s, sys: e.sys}
			clean := func() { e.exec(`DELETE FROM "user" WHERE email LIKE 'sgs-cab-%@scp.test'`) }
			clean()
			t.Cleanup(clean)
			alice, bob := c.user("sgs-cab-alice@scp.test", "Alice CAB"), c.user("sgs-cab-bob@scp.test", "Bob CAB")
			cabGroup := cabStr(cabGroupID)
			const deploy = "zz sgs test deploy capps pekin prod"

			// Customer A's five deployments (eligible), newest first.
			d1 := e.cr("A", "Deploy CApps to Pekin prod", "CLOSED", sgsPlan, 10)
			d2 := e.cr("A", "Deploy CApps to Pekin prod", "CLOSED", sgsPlan, 20)
			d3 := e.cr("A", "Deploy CApps to Pekin prod", "CLOSED", sgsPlan, 30)
			e.cr("A", "Deploy CApps to Pekin prod", "CLOSED", sgsPlan, 40)
			e.cr("A", "Deploy CApps to Pekin prod", "CLOSED", sgsPlan, 50)
			// d1, migrated style (no labels): a peer's rejection at position 0 is not
			// the CAB's; the CAB (position 1) rejected it, then approved it with a comment.
			peer := c.stage(d1, nil, nil)
			c.row(d1, peer, bob, "REJECTED", "peer says no", 40)
			auth := c.stage(d1, nil, nil)
			c.row(d1, auth, alice, "REJECTED", "<p>Plan unclear</p>", 30)
			c.row(d1, auth, bob, "APPROVED", "Fine now", 15)
			c.row(d1, auth, alice, "APPROVED", "", 15) // no comment: not listed
			// d2: the portal's labelled CAB stage, approved with pushback.
			cab2 := c.stage(d2, cabStr("CAB Approval"), cabGroup)
			c.row(d2, cab2, alice, "APPROVED", "<p>Add the verification steps</p>", 25)
			// d3: rejected on a labelled peer stage only: no CAB pushback.
			peer3 := c.stage(d3, cabStr("Peer Approval"), nil)
			c.row(d3, peer3, bob, "REJECTED", "peer only", 35)

			// Look-alikes: canceled change requests the CAB rejected.
			l1 := e.cr("A", "Deploy CApps to Pekin prod 2026-11-01", "CANCELED", sgsPlan, -1)
			s1 := c.stage(l1, cabStr("CAB Approval"), cabGroup)
			c.row(l1, s1, alice, "REJECTED", "Not during the freeze", 5)
			// Unlabelled, first stage, but in the CAB group: the CAB's.
			l2 := e.cr("A", "Deploy CApps to Pekin prod", "CANCELED", sgsPlan, -1)
			s2 := c.stage(l2, nil, cabGroup)
			c.row(l2, s2, bob, "REJECTED", "", 7)
			// Not look-alikes: rejected by a peer only; rejected too long ago; another
			// subject; another customer.
			n1 := e.cr("A", "Deploy CApps to Pekin prod", "CANCELED", sgsPlan, -1)
			c.row(n1, c.stage(n1, cabStr("Peer Approval"), nil), bob, "REJECTED", "peer", 3)
			n2 := e.cr("A", "Deploy CApps to Pekin prod", "CANCELED", sgsPlan, -1)
			c.row(n2, c.stage(n2, cabStr("CAB Approval"), cabGroup), alice, "REJECTED", "old", 800)
			n3 := e.cr("A", "Rotate the keystore", "CANCELED", sgsPlan, -1)
			c.row(n3, c.stage(n3, cabStr("CAB Approval"), cabGroup), alice, "REJECTED", "other", 3)
			for _, d := range []int{4, 8, 12, 16, 20} {
				e.cr("B", "Deploy CApps to Pekin prod", "CLOSED", sgsPlan, d)
			}
			n4 := e.cr("B", "Deploy CApps to Pekin prod", "CANCELED", sgsPlan, -1)
			c.row(n4, c.stage(n4, cabStr("CAB Approval"), cabGroup), alice, "REJECTED", "B's", 3)

			// The repository reads the CAB's rows only.
			repo := repository.NewStandardChangeSuggestionRepository(e.s)
			own, err := repo.CABDecisionsOf(ctx, []string{d1, d2, d3})
			if err != nil {
				t.Fatal(err)
			}
			states := func(cr repository.CABChangeRequest) []string {
				var out []string
				for _, d := range cr.Decisions {
					out = append(out, d.State+":"+d.Comment)
				}
				return out
			}
			if got := states(own[d1]); len(got) != 2 || got[0] != "APPROVED:Fine now" || got[1] != "REJECTED:<p>Plan unclear</p>" || !own[d1].Rejected() {
				t.Errorf("d1's CAB decisions = %v, want Bob's approval then Alice's rejection (newest first)", got)
			}
			if got := states(own[d2]); len(got) != 1 || own[d2].Rejected() {
				t.Errorf("d2's CAB decisions = %v", got)
			}
			if d, ok := own[d3]; !ok || len(d.Decisions) != 0 || d.Subject == "" {
				t.Errorf("d3 = %+v, want it present with no CAB decision", d)
			}
			since := time.Now().AddDate(-1, 0, 0)
			looks, err := repo.CABRejectedLookalikes(ctx, e.accounts["A"], since)
			if err != nil {
				t.Fatal(err)
			}
			var ids []string
			for _, l := range looks {
				ids = append(ids, l.ID)
				if l.ImplementationPlan == "" || l.State != "canceled" {
					t.Errorf("look-alike %s = %+v, want its plan and state", l.Number, l)
				}
			}
			want := []string{l1, l2, n3}
			sort.Strings(ids)
			sort.Strings(want)
			if len(ids) != 3 || ids[0] != want[0] || ids[1] != want[1] || ids[2] != want[2] {
				t.Errorf("look-alikes = %v, want l1, l2 and the other subject (attached to no cluster later)", ids)
			}

			e.exec(`DELETE FROM standard_change_suggestion_run`)
			for req := (domain.RefreshStandardChangeSuggestionsRequest{Limit: 500}); ; {
				r, err := svc.Refresh(ctx, req)
				if err != nil {
					t.Fatal(err)
				}
				if !r.More {
					break
				}
				req = domain.RefreshStandardChangeSuggestionsRequest{RunID: r.RunID, Cursor: r.NextCursor, Limit: 500}
			}
			resp, err := svc.GetSuggestions(ctx, domain.StandardChangeSuggestionsRequest{IncludeIneligible: true})
			if err != nil {
				t.Fatal(err)
			}
			a := sgsFind(resp, e.accounts["A"], deploy)
			if a == nil {
				t.Fatal("no deploy suggestion for customer A")
			}
			if !a.Eligible || a.MemberCount != 5 {
				t.Errorf("eligible %v (%v), %d members: the CAB's pushback must not change them", a.Eligible, a.Unmet, a.MemberCount)
			}
			if a.ProposeFrom == nil || a.ProposeFrom.ID != d2 {
				t.Errorf("proposeFrom = %+v, want d2 (d1 was rejected by the CAB)", a.ProposeFrom)
			}
			if a.CABRejectionCount != 3 || len(a.CABRejections) != 3 {
				t.Fatalf("rejections = %d %+v, want d1, l1 and l2", a.CABRejectionCount, a.CABRejections)
			}
			if r := a.CABRejections[0]; r.ChangeRequest.ID != l1 || r.Member || r.RejectedBy != "Alice CAB" || r.Comment != "Not during the freeze" {
				t.Errorf("newest rejection = %+v", r)
			}
			if r := a.CABRejections[2]; r.ChangeRequest.ID != d1 || !r.Member || r.Comment != "Plan unclear" {
				t.Errorf("oldest rejection = %+v, want d1's, as plain text", r)
			}
			if a.CABCommentCount != 4 {
				t.Errorf("comments = %d %+v, want 4 (d1's two, d2's, l1's)", a.CABCommentCount, a.CABComments)
			}
			var m1 *domain.StandardChangeSuggestionMember
			for i := range a.Members {
				if a.Members[i].ID == d1 {
					m1 = &a.Members[i]
				}
			}
			if m1 == nil || !m1.CABRejected || m1.CABCommentCount != 2 {
				t.Errorf("d1 as a member = %+v", m1)
			}
			if b := sgsFind(resp, e.accounts["B"], deploy); b == nil || b.CABRejectionCount != 1 || b.CABRejections[0].ChangeRequest.ID != n4 {
				t.Errorf("customer B's deploy = %+v, want its own look-alike only", b)
			}
		})
	}
}

func TestStandardChangeCABHistoryIntegration_Proposal(t *testing.T) {
	for role, dsn := range stdDSNs(t) {
		t.Run(role, func(t *testing.T) {
			env := newStdEnv(t, dsn)
			sys := repository.WithSystemIdentity(context.Background())
			svc := WithCABHistory(NewStandardChangeService(repository.NewStandardChangeRepository(env.scoped), ctxIdentityAccess{}),
				repository.NewStandardChangeSuggestionRepository(env.scoped), 12)
			c := &cabSeed{t: t, s: env.scoped, sys: sys}
			var auth1, auth2 string
			if err := env.scoped.QueryRow(sys, `SELECT id::text FROM "user" WHERE email = $1`, stdAuth1).Scan(&auth1); err != nil {
				t.Fatal(err)
			}
			if err := env.scoped.QueryRow(sys, `SELECT id::text FROM "user" WHERE email = $1`, stdAuth2).Scan(&auth2); err != nil {
				t.Fatal(err)
			}
			cr := func(label, state, project string) string {
				var id string
				if err := env.scoped.QueryRow(sys, `WITH wi AS (
						INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, subject, type, project_id)
						VALUES (gen_random_uuid(), NOW(), NOW(), 't', 't', 'CHG-STD-' || substr(md5(random()::text), 1, 10), $1, 'CHANGE_REQUEST', $3::uuid)
						RETURNING id)
					INSERT INTO change_request (id, state, change_model) SELECT id, $2::change_request_state_enum, 'NORMAL' FROM wi RETURNING id::text`,
					stdTag+" "+label, state, project).Scan(&id); err != nil {
					t.Fatalf("seed change request: %v", err)
				}
				return id
			}
			cabGroup := cabStr(cabGroupID)
			// The source (closed1) was rejected by the CAB once; closed2, cited as
			// evidence, was approved with a comment and deviated from its plan.
			src, ev := env.crs["closed1"], env.crs["closed2"]
			c.row(src, c.stage(src, cabStr("CAB Approval"), cabGroup), auth1, "REJECTED", "<p>Say which <b>cluster</b></p>", 20)
			c.row(ev, c.stage(ev, cabStr("CAB Approval"), cabGroup), auth2, "APPROVED", "Add a check", 10)
			if _, err := env.scoped.Exec(sys, `UPDATE change_request SET implementation_deviation = 'Skipped a step' WHERE id = $1::uuid`, ev); err != nil {
				t.Fatal(err)
			}
			// Look-alikes: customer A's canceled change requests with the same
			// subject key ("closed"), one the CAB rejected and one a peer did; and
			// customer B's.
			look := cr("closed9", "CANCELED", env.projects["A2"])
			c.row(look, c.stage(look, cabStr("CAB Approval"), cabGroup), auth2, "REJECTED", "Too risky", 3)
			peerOnly := cr("closed8", "CANCELED", env.projects["A"])
			c.row(peerOnly, c.stage(peerOnly, cabStr("Peer Approval"), nil), auth2, "REJECTED", "peer", 3)
			other := cr("closed7", "CANCELED", env.projects["B"])
			c.row(other, c.stage(other, cabStr("CAB Approval"), cabGroup), auth2, "REJECTED", "B", 3)

			p, err := svc.CreateProposal(stdCtx(stdProposer), domain.CreateStandardChangeProposalRequest{
				Kind: domain.StandardChangeProposalKindNew, SourceChangeRequestID: stdp(src)})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := svc.UpdateProposal(stdCtx(stdProposer), p.ID, domain.UpdateStandardChangeProposalRequest{
				EvidenceChangeRequestIDs: &[]string{src, ev}}); err != nil {
				t.Fatal(err)
			}
			got, err := svc.GetProposal(stdCtx(stdAuth1), p.ID)
			if err != nil {
				t.Fatal(err)
			}
			h := got.CABHistory
			if h == nil {
				t.Fatal("no cabHistory")
			}
			if h.CABRejectionCount != 2 || len(h.CABRejections) != 2 ||
				h.CABRejections[0].ChangeRequest.ID != look || h.CABRejections[0].Member ||
				h.CABRejections[1].ChangeRequest.ID != src || !h.CABRejections[1].Member || h.CABRejections[1].Comment != "Say which cluster" {
				t.Errorf("rejections = %+v, want the look-alike then the source", h.CABRejections)
			}
			if h.CABCommentCount != 3 {
				t.Errorf("comments = %d %+v, want the source's, the evidence's and the look-alike's", h.CABCommentCount, h.CABComments)
			}
			marks := map[string][2]bool{}
			for _, e := range got.Evidence {
				marks[e.ID] = [2]bool{e.CABRejected, e.HasDeviation}
			}
			if marks[src] != [2]bool{true, false} || marks[ev] != [2]bool{false, true} {
				t.Errorf("evidence marks = %v, want the source rejected, the evidence deviated", marks)
			}
			// Without the history wired, the field is null (the plain service).
			if plain, err := env.svc.GetProposal(stdCtx(stdAuth1), p.ID); err != nil || plain.CABHistory != nil {
				t.Errorf("plain service: %v, %+v", err, plain.CABHistory)
			}
		})
	}
}
