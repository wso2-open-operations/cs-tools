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
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// The standard change suggestions against PostgreSQL: the real service over the
// real repository, run once per DSN given (STANDARD_CHANGE_TEST_DSN, and
// STANDARD_CHANGE_TEST_APP_DSN, a role without BYPASSRLS, so the internal-only
// policies of 100026 / 100029 apply).
//
//	STANDARD_CHANGE_TEST_DSN=postgres://... STANDARD_CHANGE_TEST_APP_DSN=postgres://... \
//	  go test ./internal/service/ -run StandardChangeSuggestionIntegration -p 1

const (
	sgsTag   = "ZZ sgs-test"
	sgsStaff = "sgs-staff@scp.test"
	sgsPlan  = "<p>Stop the gateway, copy the CApp archive to the deployment folder, start the gateway and check the health endpoint</p>"
)

func sgsCleanup(t *testing.T, s *repository.Scoped) {
	t.Helper()
	sys := repository.WithSystemIdentity(context.Background())
	for _, q := range []string{
		`DELETE FROM work_item WHERE subject LIKE '` + sgsTag + `%'`,
		`DELETE FROM work_item WHERE project_id IN (SELECT id FROM project WHERE key LIKE 'ZZSGSTEST-%')`,
		`DELETE FROM standard_change_template WHERE name LIKE '` + sgsTag + `%'`,
		`DELETE FROM project WHERE key LIKE 'ZZSGSTEST-%'`,
		`DELETE FROM account WHERE number LIKE 'ZZSGSTEST-%'`,
		`DELETE FROM "user" WHERE email = '` + sgsStaff + `'`,
	} {
		if _, err := s.Exec(sys, q); err != nil {
			t.Fatalf("cleanup %q: %v", q, err)
		}
	}
}

type sgsEnv struct {
	t        *testing.T
	s        *repository.Scoped
	sys      context.Context
	accounts map[string]string
	projects map[string]string
}

func (e *sgsEnv) exec(q string, args ...any) {
	e.t.Helper()
	if _, err := e.s.Exec(e.sys, q, args...); err != nil {
		e.t.Fatalf("%s: %v", q, err)
	}
}

// cr seeds a change request of a customer's project, completed daysAgo days ago
// (closed_on), or with no closed_on when daysAgo < 0 (then updated a day ago).
func (e *sgsEnv) cr(customer, subject, state, plan string, daysAgo int) string {
	e.t.Helper()
	var closed *time.Time
	if daysAgo >= 0 {
		c := time.Now().AddDate(0, 0, -daysAgo)
		closed = &c
	}
	var id string
	if err := e.s.QueryRow(e.sys, `WITH wi AS (
			INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, subject, type, project_id)
			VALUES (gen_random_uuid(), NOW() - interval '400 days', NOW() - interval '1 day', 't', 't',
			        'CHG-SGS-' || substr(md5(random()::text), 1, 10), $1, 'CHANGE_REQUEST', $2::uuid)
			RETURNING id)
		INSERT INTO change_request (id, state, change_model, implementation_plan, risk, impact, closed_on)
		SELECT id, $3::change_request_state_enum, 'NORMAL', $4, 'LOW', 'LOW', $5 FROM wi RETURNING id::text`,
		sgsTag+" "+subject, e.projects[customer], state, plan, closed).Scan(&id); err != nil {
		e.t.Fatalf("seed change request: %v", err)
	}
	return id
}

func (e *sgsEnv) workItem(typ, number string) string {
	e.t.Helper()
	var id string
	if err := e.s.QueryRow(e.sys, `INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, subject, type, project_id)
		VALUES (gen_random_uuid(), NOW(), NOW(), 't', 't', $2, $3, $1::work_item_type_enum, $4::uuid) RETURNING id::text`,
		typ, number, sgsTag+" "+number, e.projects["A"]).Scan(&id); err != nil {
		e.t.Fatalf("seed %s: %v", typ, err)
	}
	return id
}

func newSgsEnv(t *testing.T, dsn string) (*sgsEnv, StandardChangeSuggestionService) {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	s := repository.NewScoped(pool)
	sgsCleanup(t, s)
	t.Cleanup(func() { sgsCleanup(t, s) })
	e := &sgsEnv{t: t, s: s, sys: repository.WithSystemIdentity(context.Background()), accounts: map[string]string{}, projects: map[string]string{}}
	e.exec(`INSERT INTO "user" (id, created_on, updated_on, created_by, updated_by, user_name, email, name, is_active)
		VALUES (gen_random_uuid(), NOW(), NOW(), 't', 't', $1, $1, 'SGS Staff', true)`, sgsStaff)
	for _, label := range []string{"A", "B"} {
		var acc, proj string
		if err := s.QueryRow(e.sys, `INSERT INTO account (id, created_on, updated_on, created_by, updated_by, name, number)
			VALUES (gen_random_uuid(), NOW(), NOW(), 't', 't', $1, $2) RETURNING id::text`, sgsTag+" Customer "+label, "ZZSGSTEST-"+label).Scan(&acc); err != nil {
			t.Fatalf("seed account: %v", err)
		}
		if err := s.QueryRow(e.sys, `INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, name, account_id)
			VALUES (gen_random_uuid(), NOW(), NOW(), 't', 't', $1, $2, $3::uuid) RETURNING id::text`, "ZZSGSTEST-"+label, sgsTag+" Project "+label, acc).Scan(&proj); err != nil {
			t.Fatalf("seed project: %v", err)
		}
		e.accounts[label], e.projects[label] = acc, proj
	}
	return e, NewStandardChangeSuggestionService(repository.NewStandardChangeSuggestionRepository(s), ctxIdentityAccess{}, domain.StandardChangeSuggestionCriteria{})
}

func sgsFind(resp domain.StandardChangeSuggestionsResponse, customer, key string) *domain.StandardChangeSuggestion {
	for i, s := range resp.Suggestions {
		if s.Customer.ID == customer && s.Key == key {
			return &resp.Suggestions[i]
		}
	}
	return nil
}

func TestStandardChangeSuggestionIntegration(t *testing.T) {
	for role, dsn := range stdDSNs(t) {
		t.Run(role, func(t *testing.T) {
			e, svc := newSgsEnv(t, dsn)
			ctx := stdCtx(sgsStaff)
			const deploy = "zz sgs test deploy capps pekin prod"
			const restart = "zz sgs test restart gateway"
			const firewallSubject = "zz sgs test update firewall rules partner integrations"

			// Customer A: four deployments, a clone of one under another subject (five
			// closed: the default minimum), and
			// three that do not count: raised from a template, too old, canceled.
			d1 := e.cr("A", "Deploy CApps to Pekin prod 2026-10-01", "CLOSED", sgsPlan, 10)
			e.cr("A", "[ACME] Deploy CApps to Pekin prod", "CLOSED", sgsPlan, 20)
			e.cr("A", "Deploy CApps to Pekin prod v2.3", "CLOSED", sgsPlan, 30)
			e.cr("A", "Deploy CApps to Pekin prod (CHG0011223)", "CLOSED", sgsPlan, 40)
			clone := e.cr("A", "Promote the Pekin release to DR", "CLOSED", "<p>Different words were typed in this plan</p>", 5)
			e.exec(`UPDATE change_request SET cloned_from_change_request_id = $2::uuid WHERE id = $1::uuid`, clone, d1)
			e.cr("A", "Deploy CApps to Pekin prod", "CLOSED", sgsPlan, 800) // outside the window
			e.cr("A", "Deploy CApps to Pekin prod", "CANCELED", sgsPlan, 3) // not completed
			// A template of the same change: the change request raised from it is
			// not a candidate (it already is a standard change).
			var tplID, verID string
			if err := e.s.QueryRow(e.sys, `INSERT INTO standard_change_template (name, created_by, updated_by, account_id)
				VALUES ($1, 't', 't', $2::uuid) RETURNING id::text`, sgsTag+" Deploy CApps", e.accounts["A"]).Scan(&tplID); err != nil {
				t.Fatalf("seed template: %v", err)
			}
			if err := e.s.QueryRow(e.sys, `INSERT INTO standard_change_template_version (template_id, version, subject, implementation_plan, backout_plan, test_plan, approved_by)
				VALUES ($1::uuid, 1, $2, $3, '<p>b</p>', '<p>t</p>', 't') RETURNING id::text`,
				tplID, sgsTag+" Deploy CApps to {{env}}", sgsPlan).Scan(&verID); err != nil {
				t.Fatalf("seed version: %v", err)
			}
			e.exec(`UPDATE standard_change_template SET current_version_id = $2::uuid WHERE id = $1::uuid`, tplID, verID)
			templated := e.cr("A", "Deploy CApps to Pekin prod", "CLOSED", sgsPlan, 2)
			e.exec(`UPDATE change_request SET standard_change_template_version_id = $2::uuid WHERE id = $1::uuid`, templated, verID)

			// Customer A: restarts, one with no closed_on (completed: last update), one
			// that caused an incident, one cited by an open proposal.
			r1 := e.cr("A", "Restart gateway", "CLOSED", "", 1)
			r2 := e.cr("A", "Restart gateway", "CLOSED", "", -1)
			e.cr("A", "Restart gateway", "CLOSED", "", 3)
			inc := e.workItem("INCIDENT", "INC-SGS-1")
			e.exec(`INSERT INTO incident (id, caused_by_id) VALUES ($1::uuid, $2::uuid)`, inc, r2)
			scp := e.workItem("STANDARD_CHANGE_PROPOSAL", "SCP-SGS-1")
			e.exec(`INSERT INTO standard_change_proposal (id, kind, state, source_change_request_id, account_id) VALUES ($1::uuid, 'NEW', 'DRAFT', $2::uuid, $3::uuid)`, scp, r1, e.accounts["A"])

			// Customer B: the same deployment, its own cluster, no template.
			for _, d := range []int{4, 8, 12, 16, 20} {
				e.cr("B", "Deploy CApps to Pekin prod", "CLOSED", sgsPlan, d)
			}
			// Customer B: one subject, two changes (plans less than half alike).
			for _, d := range []int{2, 6, 10, 14, 18} {
				e.cr("B", "Update the firewall rules for partner integrations", "CLOSED", "<p>Edit the security group, add the partner CIDR ranges to the inbound rules, save and verify the connection</p>", d)
				e.cr("B", "Update the firewall rules for partner integrations", "CLOSED", "<p>Open the WAF console, create an allow rule for the partner user agents, deploy the rule set to production</p>", d+1)
			}

			// Nothing is computed by a read: before a refresh there is no snapshot
			// (an earlier test's run is cleared first).
			e.exec(`DELETE FROM standard_change_suggestion_run`)
			if resp, err := svc.GetSuggestions(ctx, domain.StandardChangeSuggestionsRequest{}); err != nil || resp.ComputedOn != nil || len(resp.Suggestions) != 0 {
				t.Fatalf("before any refresh: %+v, %v", resp, err)
			}
			// Two refreshes, a customer per page: the second replaces the first.
			for i := 0; i < 2; i++ {
				req := domain.RefreshStandardChangeSuggestionsRequest{Limit: 1}
				for page := 0; ; page++ {
					r, err := svc.Refresh(ctx, req)
					if err != nil {
						t.Fatalf("refresh %d page %d: %v", i, page, err)
					}
					if !r.More {
						if r.ComputedOn == nil || r.Totals == nil || r.Totals.Customers < 2 {
							t.Fatalf("the last page did not publish: %+v", r)
						}
						break
					}
					req = domain.RefreshStandardChangeSuggestionsRequest{RunID: r.RunID, Cursor: r.NextCursor, Limit: 1}
				}
			}
			var runs int
			if err := e.s.QueryRow(e.sys, `SELECT COUNT(*) FROM standard_change_suggestion_run`).Scan(&runs); err != nil || runs != 1 {
				t.Fatalf("runs kept = %d, %v; want only the published one", runs, err)
			}

			resp, err := svc.GetSuggestions(ctx, domain.StandardChangeSuggestionsRequest{IncludeIneligible: true})
			if err != nil {
				t.Fatalf("GetSuggestions: %v", err)
			}
			if resp.ComputedOn == nil {
				t.Fatal("no computedOn after a refresh")
			}
			a := sgsFind(resp, e.accounts["A"], deploy)
			if a == nil {
				t.Fatalf("no deploy suggestion for customer A in %+v", resp.Suggestions)
			}
			if a.MemberCount != 5 || a.ClosedCount != 5 || !a.Eligible || a.ProposeFrom == nil {
				t.Fatalf("customer A deploy = %d members, %d closed, eligible %v (%v), from %+v", a.MemberCount, a.ClosedCount, a.Eligible, a.Unmet, a.ProposeFrom)
			}
			if a.Members[0].ID != clone || a.Members[0].JoinedBy != "clone" || a.Members[1].JoinedBy != "subject" {
				t.Errorf("members = %+v, want the clone first, joined by lineage", a.Members)
			}
			if a.ProposeFrom.ID != clone || len(a.Evidence) != 4 {
				t.Errorf("proposeFrom = %+v, evidence = %v", a.ProposeFrom, a.Evidence)
			}
			b := sgsFind(resp, e.accounts["B"], deploy)
			if b == nil || b.MemberCount != 5 || !b.Eligible {
				t.Fatalf("customer B deploy = %+v, want its own eligible cluster", b)
			}
			var firewall []domain.StandardChangeSuggestion
			for _, s := range resp.Suggestions {
				if s.Customer.ID == e.accounts["B"] && s.SubjectKey == firewallSubject {
					firewall = append(firewall, s)
				}
			}
			if len(firewall) != 2 || firewall[0].Key == firewall[1].Key || !firewall[0].Eligible || !firewall[1].Eligible {
				t.Fatalf("customer B firewall = %+v, want two eligible clusters with keys of their own", firewall)
			}
			var split domain.StandardChangeSuggestion // the one whose key has a suffix
			for _, f := range firewall {
				if f.Key != firewallSubject {
					split = f
				}
			}
			if !strings.HasPrefix(split.Key, firewallSubject+"#") {
				t.Fatalf("firewall keys %q / %q: want the subject key and one with a suffix", firewall[0].Key, firewall[1].Key)
			}
			rs := sgsFind(resp, e.accounts["A"], restart)
			if rs == nil || rs.MemberCount != 3 || rs.Eligible || len(rs.Incidents) != 1 || rs.Incidents[0].ID != inc {
				t.Fatalf("restart = %+v, want three members, ineligible for INC-SGS-1", rs)
			}
			if len(rs.OpenProposals) != 1 || rs.OpenProposals[0].ID != scp || rs.OpenProposals[0].Name != "SCP-SGS-1" {
				t.Errorf("open proposals = %v, want SCP-SGS-1", rs.OpenProposals)
			}

			// An open proposal started after the snapshot shows at once (read live).
			scp2 := e.workItem("STANDARD_CHANGE_PROPOSAL", "SCP-SGS-2")
			e.exec(`INSERT INTO standard_change_proposal (id, kind, state, source_change_request_id, account_id) VALUES ($1::uuid, 'NEW', 'DRAFT', $2::uuid, $3::uuid)`, scp2, d1, e.accounts["A"])
			resp, _ = svc.GetSuggestions(ctx, domain.StandardChangeSuggestionsRequest{AccountID: e.accounts["A"]})
			if len(resp.Suggestions) != 1 || len(resp.Suggestions[0].OpenProposals) != 1 || resp.Suggestions[0].OpenProposals[0].ID != scp2 {
				t.Fatalf("a proposal started since the snapshot: %+v", resp.Suggestions)
			}
			if resp.Scanned.Customers != 1 || resp.Scanned.ChangeRequests != 8 || resp.Scanned.Clusters != 2 {
				t.Errorf("customer A's counts = %+v, want 1 customer, 8 change requests (5 deploy + 3 restart), 2 clusters", resp.Scanned)
			}
			e.exec(`DELETE FROM work_item WHERE id = $1::uuid`, scp2)

			// One customer only; eligible only by default.
			resp, err = svc.GetSuggestions(ctx, domain.StandardChangeSuggestionsRequest{AccountID: e.accounts["A"]})
			if err != nil || len(resp.Suggestions) != 1 || resp.Suggestions[0].Key != deploy {
				t.Fatalf("customer A, eligible only: %v, %+v", err, resp.Suggestions)
			}

			// Dismiss, hidden; again is a 409; restore, back; restore again is a 404.
			d, err := svc.DismissSuggestion(ctx, domain.DismissStandardChangeSuggestionRequest{AccountID: e.accounts["A"], Key: deploy, Reason: "covered already"})
			if err != nil || d.DismissedBy == nil || d.DismissedBy.Name != "SGS Staff" || d.Reason != "covered already" {
				t.Fatalf("dismiss: %+v, %v", d, err)
			}
			resp, _ = svc.GetSuggestions(ctx, domain.StandardChangeSuggestionsRequest{AccountID: e.accounts["A"]})
			if len(resp.Suggestions) != 0 || resp.Scanned.Dismissed != 1 {
				t.Fatalf("after the dismissal: %+v, scanned %+v", resp.Suggestions, resp.Scanned)
			}
			resp, _ = svc.GetSuggestions(ctx, domain.StandardChangeSuggestionsRequest{AccountID: e.accounts["A"], IncludeDismissed: true})
			if len(resp.Suggestions) != 1 || resp.Suggestions[0].Dismissal == nil || resp.Suggestions[0].Dismissal.ID != d.ID {
				t.Fatalf("with the dismissed: %+v", resp.Suggestions)
			}
			if b2 := sgsFind(resp, e.accounts["B"], deploy); b2 != nil {
				t.Error("an accountId-limited request returned another customer's suggestion")
			}
			_, err = svc.DismissSuggestion(ctx, domain.DismissStandardChangeSuggestionRequest{AccountID: e.accounts["A"], Key: deploy, Reason: "again"})
			expectErr[*apierror.ConflictError](t, err, "a second dismissal")
			_, err = svc.DismissSuggestion(ctx, domain.DismissStandardChangeSuggestionRequest{AccountID: "00000000-0000-4000-8000-00000000dead", Key: deploy, Reason: "x"})
			expectErr[*apierror.ValidationError](t, err, "a dismissal for no customer")
			if err := svc.RestoreSuggestion(ctx, d.ID); err != nil {
				t.Fatalf("restore: %v", err)
			}
			resp, _ = svc.GetSuggestions(ctx, domain.StandardChangeSuggestionsRequest{AccountID: e.accounts["A"]})
			if len(resp.Suggestions) != 1 {
				t.Fatalf("after the restore: %+v", resp.Suggestions)
			}
			expectErr[*apierror.NotFoundError](t, svc.RestoreSuggestion(ctx, d.ID), "a second restore")

			// The suffixed key is dismissed on its own, and stays dismissed after
			// the next refresh (the key is carried over).
			if _, err := svc.DismissSuggestion(ctx, domain.DismissStandardChangeSuggestionRequest{AccountID: e.accounts["B"], Key: split.Key, Reason: "the WAF one is not standard"}); err != nil {
				t.Fatalf("dismiss %q: %v", split.Key, err)
			}
			for req := (domain.RefreshStandardChangeSuggestionsRequest{}); ; {
				r, err := svc.Refresh(ctx, req)
				if err != nil {
					t.Fatalf("refresh: %v", err)
				}
				if !r.More {
					break
				}
				req = domain.RefreshStandardChangeSuggestionsRequest{RunID: r.RunID, Cursor: r.NextCursor}
			}
			resp, _ = svc.GetSuggestions(ctx, domain.StandardChangeSuggestionsRequest{AccountID: e.accounts["B"], IncludeDismissed: true})
			var shown, dismissed int
			for _, s := range resp.Suggestions {
				if s.SubjectKey == firewallSubject {
					shown++
					if s.Dismissal != nil {
						dismissed++
						if s.Key != split.Key {
							t.Errorf("the dismissal applies to %q, want %q", s.Key, split.Key)
						}
					}
				}
			}
			if shown != 2 || dismissed != 1 {
				t.Fatalf("after the next refresh: %d firewall suggestions, %d dismissed; want 2 and 1", shown, dismissed)
			}

			// The customers are listed once, when a run starts; one that overran its
			// deadline is recorded as skipped and counted apart.
			repo := repository.NewStandardChangeSuggestionRepository(e.s)
			latest, err := repo.LatestRun(ctx)
			if err != nil || latest == nil {
				t.Fatalf("latest run: %v, %v", latest, err)
			}
			keysOfA, err := repo.SnapshotKeys(ctx, latest.ID, e.accounts["A"])
			if err != nil || len(keysOfA) != 2 {
				t.Fatalf("customer A's keys = %+v, %v; want its two clusters", keysOfA, err)
			}
			run, err := repo.StartRun(ctx, DefaultStandardChangeSuggestionCriteria, func(started time.Time) time.Time { return started.AddDate(-1, 0, 0) })
			if err != nil {
				t.Fatalf("start run: %v", err)
			}
			listed, err := repo.NextCustomers(ctx, run.ID, "", 100000)
			if err != nil || !slices.Contains(listed, e.accounts["A"]) || !slices.Contains(listed, e.accounts["B"]) || !slices.IsSorted(listed) {
				t.Fatalf("listed = %d customers (%v), want A and B, in id order", len(listed), err)
			}
			if after, _ := repo.NextCustomers(ctx, run.ID, e.accounts["A"], 100000); slices.Contains(after, e.accounts["A"]) {
				t.Error("the cursor did not move past A")
			}
			if err := repo.SkipCustomer(ctx, run.ID, e.accounts["A"], "too large"); err != nil {
				t.Fatalf("skip A: %v", err)
			}
			// The skip carried A's keys into this run, for the next one to keep.
			if carried, err := repo.SnapshotKeys(ctx, run.ID, e.accounts["A"]); err != nil || !reflect.DeepEqual(carried, keysOfA) {
				t.Fatalf("carried = %+v, %v; want %+v", carried, err, keysOfA)
			}
			if err := repo.SaveCustomer(ctx, run.ID, e.accounts["B"], 5, nil); err != nil {
				t.Fatalf("save B: %v", err)
			}
			published, err := repo.PublishRun(ctx, run.ID)
			if err != nil || published.Counts.Customers != 1 || published.Counts.Skipped != 1 || published.Counts.ChangeRequests != 5 {
				t.Fatalf("published = %+v, %v; want B computed, A skipped", published.Counts, err)
			}
			if counts, err := repo.CustomerCounts(ctx, run.ID, e.accounts["A"]); err != nil || counts != (domain.StandardChangeSuggestionScan{Skipped: 1}) {
				t.Errorf("customer A's counts = %+v, %v", counts, err)
			}
			resp, err = svc.GetSuggestions(ctx, domain.StandardChangeSuggestionsRequest{})
			if err != nil || len(resp.SkippedCustomers) != 1 || resp.SkippedCustomers[0].Customer.ID != e.accounts["A"] ||
				resp.SkippedCustomers[0].Customer.Name != sgsTag+" Customer A" || resp.SkippedCustomers[0].Reason != "too large" || resp.Scanned.Skipped != 1 {
				t.Fatalf("skipped = %+v, scanned %+v, %v", resp.SkippedCustomers, resp.Scanned, err)
			}
			if resp, _ := svc.GetSuggestions(ctx, domain.StandardChangeSuggestionsRequest{AccountID: e.accounts["B"]}); len(resp.SkippedCustomers) != 0 {
				t.Errorf("customer B's skipped = %+v, want none", resp.SkippedCustomers)
			}
			expectErr[*apierror.ConflictError](t, repo.SkipCustomer(ctx, run.ID, e.accounts["B"], "late"), "a skip on a published run")
			if resp.Scanned.Clusters != published.Counts.Clusters {
				t.Errorf("carried keys counted as clusters: scanned %+v, run %+v", resp.Scanned, published.Counts)
			}

			// Skipped again: the keys the last skip carried come along once more.
			again, err := repo.StartRun(ctx, DefaultStandardChangeSuggestionCriteria, func(started time.Time) time.Time { return started.AddDate(-1, 0, 0) })
			if err != nil {
				t.Fatal(err)
			}
			if err := repo.SkipCustomer(ctx, again.ID, e.accounts["A"], "too large"); err != nil {
				t.Fatal(err)
			}
			if carried, err := repo.SnapshotKeys(ctx, again.ID, e.accounts["A"]); err != nil || !reflect.DeepEqual(carried, keysOfA) {
				t.Fatalf("carried twice = %+v, %v; want %+v", carried, err, keysOfA)
			}
			// Computed after all (a page sent again): nothing carried any more.
			if err := repo.SaveCustomer(ctx, again.ID, e.accounts["A"], 1, nil); err != nil {
				t.Fatal(err)
			}
			if carried, _ := repo.SnapshotKeys(ctx, again.ID, e.accounts["A"]); len(carried) != 0 {
				t.Errorf("after saving A, carried = %+v", carried)
			}
		})
	}
}
