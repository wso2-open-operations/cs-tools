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
	"encoding/json"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// GET /standard-change-suggestions pages in the query: the order across
// customers is the one sort (sgListOrder) whatever the page size, total and the
// dismissed count are the whole list's, and a dismissal is joined to its row.
// Snapshot rows are written directly, so the sort keys and their ties are exact.
//
//	STANDARD_CHANGE_TEST_DSN=postgres://... STANDARD_CHANGE_TEST_APP_DSN=postgres://... \
//	  go test ./internal/service/ -run StandardChangeSuggestionPageIntegration -p 1
func TestStandardChangeSuggestionPageIntegration(t *testing.T) {
	for role, dsn := range stdDSNs(t) {
		t.Run(role, func(t *testing.T) {
			e, svc := newSgsEnv(t, dsn)
			ctx := stdCtx(sgsStaff)
			e.exec(`DELETE FROM standard_change_suggestion_run`)
			var runID string
			if err := e.s.QueryRow(e.sys, `INSERT INTO standard_change_suggestion_run (completed_on, criteria)
				VALUES (NOW(), '{}'::jsonb) RETURNING id::text`).Scan(&runID); err != nil {
				t.Fatalf("seed run: %v", err)
			}

			day := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
			type seed struct {
				account, name, key string
				eligible           bool
				closed             int
				last               time.Time
			}
			// Ties on every key in turn: closed count, then the completion, then the
			// customer's name ("Zeta" sorts before "acme" in byte order, after it in
			// most locales), then the key, then the row (the same key twice).
			seeds := []seed{
				{"A", "acme", "k-near-miss", false, 30, day},
				{"B", "Zeta", "k-closed-6", true, 6, day},
				{"A", "acme", "k-closed-9", true, 9, day.Add(-48 * time.Hour)},
				{"A", "acme", "k-closed-9-newer", true, 9, day},
				{"A", "acme", "k-tie-name", true, 7, day},
				{"B", "Zeta", "k-tie-name", true, 7, day},
				{"B", "Zeta", "k-tie-key-b", true, 5, day},
				{"B", "Zeta", "k-tie-key-a", true, 5, day},
				{"A", "acme", "k-same", true, 4, day},
				{"A", "acme", "k-same", true, 4, day},
				{"B", "Zeta", "k-dismissed", true, 8, day},
				{"A", "acme", "k-near-miss-2", false, 2, day},
			}
			var stored []repository.SuggestionSnapshotRow
			for i, s := range seeds {
				sg := domain.StandardChangeSuggestion{
					Key: s.key, SubjectKey: s.key, Subject: fmt.Sprintf("row %d", i),
					Customer: domain.EntityRef{ID: e.accounts[s.account], Name: s.name},
					Eligible: s.eligible, ClosedCount: s.closed, MemberCount: s.closed,
					FirstCompletedOn: s.last.Add(-24 * time.Hour), LastCompletedOn: s.last,
				}
				raw, _ := json.Marshal(sg)
				e.exec(`INSERT INTO standard_change_suggestion_snapshot (run_id, account_id, suggestion_key, eligible, member_ids, suggestion)
					VALUES ($1::uuid, $2::uuid, $3, $4, '{}', $5::jsonb)`, runID, e.accounts[s.account], s.key, s.eligible, raw)
				stored = append(stored, repository.SuggestionSnapshotRow{AccountID: e.accounts[s.account], Key: s.key, Suggestion: sg})
			}
			e.exec(`INSERT INTO standard_change_suggestion_dismissal (account_id, suggestion_key, reason, dismissed_by)
				VALUES ($1::uuid, 'k-dismissed', 'one-off', 'Jane Staff')`, e.accounts["B"])

			// The expected order: the reference sort over the rows as stored (by id).
			subjects := func(rows []repository.SuggestionSnapshotRow, keep func(repository.SuggestionSnapshotRow) bool) []string {
				rows = slices.Clone(rows)
				sgListOrder(rows)
				var out []string
				for _, r := range rows {
					if keep(r) {
						out = append(out, r.Suggestion.Subject)
					}
				}
				return out
			}
			got := func(resp domain.StandardChangeSuggestionsResponse) []string {
				var out []string
				for _, s := range resp.Suggestions {
					out = append(out, s.Subject)
				}
				return out
			}
			all := subjects(stored, func(repository.SuggestionSnapshotRow) bool { return true })
			// The reference sort itself, spelled out: the near misses last, "Zeta"
			// before "acme" on a tie, the two "k-same" rows in id order.
			wantAll := []string{"row 3", "row 2", "row 10", "row 5", "row 4", "row 1", "row 7", "row 6", "row 8", "row 9", "row 0", "row 11"}
			if !slices.Equal(all, wantAll) {
				t.Fatalf("reference order %v, want %v", all, wantAll)
			}

			full := domain.StandardChangeSuggestionsRequest{IncludeIneligible: true, IncludeDismissed: true, Pagination: domain.Pagination{Limit: 50}}
			resp, err := svc.GetSuggestions(ctx, full)
			if err != nil {
				t.Fatalf("GetSuggestions: %v", err)
			}
			if !slices.Equal(got(resp), all) || resp.Total != len(seeds) || resp.Scanned.Dismissed != 1 {
				t.Fatalf("order %v\nwant  %v\ntotal %d, dismissed %d", got(resp), all, resp.Total, resp.Scanned.Dismissed)
			}
			for _, s := range resp.Suggestions {
				if (s.Key == "k-dismissed") != (s.Dismissal != nil) {
					t.Errorf("%s: dismissal %+v", s.Key, s.Dismissal)
				}
				if s.Dismissal != nil && (s.Dismissal.Reason != "one-off" || s.Dismissal.DismissedBy == nil || s.Dismissal.DismissedBy.Name != "Jane Staff" || s.Dismissal.ID == "") {
					t.Errorf("dismissal = %+v", s.Dismissal)
				}
			}

			// Every page size walks the same list.
			for _, size := range []int{1, 2, 5} {
				var walked []string
				for offset := 0; ; offset += size {
					req := full
					req.Pagination = domain.Pagination{Limit: size, Offset: offset}
					page, err := svc.GetSuggestions(ctx, req)
					if err != nil {
						t.Fatalf("page %d+%d: %v", offset, size, err)
					}
					if page.Total != len(seeds) || page.Limit != size || page.Offset != offset {
						t.Fatalf("page %d+%d: total %d, limit %d, offset %d", offset, size, page.Total, page.Limit, page.Offset)
					}
					if len(page.Suggestions) == 0 {
						break
					}
					walked = append(walked, got(page)...)
				}
				if !slices.Equal(walked, all) {
					t.Errorf("pages of %d walked %v\nwant %v", size, walked, all)
				}
			}

			// The defaults: eligible and not dismissed only; the dismissed one is
			// still counted, and total is what is listed.
			resp, err = svc.GetSuggestions(ctx, domain.StandardChangeSuggestionsRequest{})
			wantDefault := subjects(stored, func(r repository.SuggestionSnapshotRow) bool {
				return r.Suggestion.Eligible && r.Key != "k-dismissed"
			})
			if err != nil || !slices.Equal(got(resp), wantDefault) || resp.Total != len(wantDefault) || resp.Scanned.Dismissed != 1 || resp.Limit != 20 {
				t.Fatalf("default: %v (total %d, dismissed %d, limit %d), %v\nwant %v", got(resp), resp.Total, resp.Scanned.Dismissed, resp.Limit, err, wantDefault)
			}

			// One customer: its own rows, in the same order.
			one := full
			one.AccountID = e.accounts["B"]
			resp, err = svc.GetSuggestions(ctx, one)
			wantB := subjects(stored, func(r repository.SuggestionSnapshotRow) bool { return r.AccountID == e.accounts["B"] })
			if err != nil || !slices.Equal(got(resp), wantB) || resp.Total != len(wantB) {
				t.Fatalf("customer B: %v (total %d), %v\nwant %v", got(resp), resp.Total, err, wantB)
			}

			// Past the end: no rows, and the counts still answer.
			past := full
			past.Pagination = domain.Pagination{Limit: 10, Offset: 100}
			resp, err = svc.GetSuggestions(ctx, past)
			if err != nil || len(resp.Suggestions) != 0 || resp.Suggestions == nil || resp.Total != len(seeds) || resp.Scanned.Dismissed != 1 {
				t.Fatalf("past the end: %+v, %v", resp, err)
			}
		})
	}
}
