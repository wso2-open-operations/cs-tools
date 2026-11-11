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

// next_wso2_id (migration 0290) heals a stale project.wso2_id_counter: it
// never returns an id that already exists in work_item.wso2_id, never lowers
// the counter, and sees every existing id even when the caller is a customer
// whose row-level security hides most work items. Exercised on a real
// Postgres with the migrations applied and RLS in force, reusing the
// case-create fixture (project CCMPROJ the customer belongs to, CCMOTHER they
// do not). Connect as a role that does NOT bypass RLS; skipped without
// CASE_CREATE_RLS_TEST_DSN.
//
//	CASE_CREATE_RLS_TEST_DSN=postgres://... go test ./internal/repository/ -run NextWSO2ID

package repository_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

type wso2IDHarness struct {
	t        *testing.T
	scoped   *repository.Scoped
	internal context.Context
	customer context.Context
	seq      int
}

func newWSO2IDHarness(t *testing.T) *wso2IDHarness {
	pool := seedCaseCreateMemberFixture(t)
	return &wso2IDHarness{
		t:        t,
		scoped:   repository.NewScoped(pool),
		internal: repository.WithSystemIdentity(context.Background()),
		customer: repository.WithCallerIdentity(context.Background(), repository.SearchScope{ViewerEmail: ccmCustomer}),
	}
}

// reset empties the two fixture projects' work items and sets the counters.
func (h *wso2IDHarness) reset(counter, otherCounter int) {
	h.t.Helper()
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`DELETE FROM work_item WHERE project_id IN ($1, $2)`, []any{ccmProjectID, ccmOtherProj}},
		{`UPDATE project SET wso2_id_counter = $2 WHERE id = $1`, []any{ccmProjectID, counter}},
		{`UPDATE project SET wso2_id_counter = $2 WHERE id = $1`, []any{ccmOtherProj, otherCounter}},
	} {
		if _, err := h.scoped.Exec(h.internal, q.sql, q.args...); err != nil {
			h.t.Fatalf("reset: %v", err)
		}
	}
}

// seed inserts a work item carrying wso2Id into project (synthetic fixture rows).
func (h *wso2IDHarness) seed(project, wso2Id string) {
	h.t.Helper()
	h.seq++
	_, err := h.scoped.Exec(h.internal, `INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type, project_id)
		VALUES (gen_random_uuid(), now(), now(), 't', 't', $1, $2, 'seeded', 'CASE'::work_item_type_enum, $3)`,
		fmt.Sprintf("NWSELF%06d", h.seq), wso2Id, project)
	if err != nil {
		h.t.Fatalf("seed %s: %v", wso2Id, err)
	}
}

func (h *wso2IDHarness) next(ctx context.Context, project string) (string, error) {
	var id string
	err := h.scoped.QueryRow(ctx, `SELECT next_wso2_id($1::uuid)`, project).Scan(&id)
	return id, err
}

func (h *wso2IDHarness) counter(project string) int {
	h.t.Helper()
	var n int
	if err := h.scoped.QueryRow(h.internal, `SELECT wso2_id_counter FROM project WHERE id = $1`, project).Scan(&n); err != nil {
		h.t.Fatalf("read counter: %v", err)
	}
	return n
}

func TestNextWSO2IDStaleCounterCatchesUp(t *testing.T) {
	h := newWSO2IDHarness(t)
	h.reset(0, 0)
	for i := 1; i <= 25; i++ {
		h.seed(ccmProjectID, fmt.Sprintf("CCMPROJ-%d", i))
	}
	got, err := h.next(h.internal, ccmProjectID)
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	if got != "CCMPROJ-26" {
		t.Fatalf("got %q, want CCMPROJ-26 (suffixes 1..25 exist, counter was 0)", got)
	}
	if c := h.counter(ccmProjectID); c != 26 {
		t.Fatalf("counter = %d, want 26 (healing is persisted)", c)
	}
	// Next call is a plain increment, no further healing.
	if got, _ = h.next(h.internal, ccmProjectID); got != "CCMPROJ-27" {
		t.Fatalf("second call = %q, want CCMPROJ-27", got)
	}
}

// Suffixes are compared as integers (10 beats 9), only on the last dash and
// only for exactly "<key>-<digits>": other families sharing the prefix never
// count, and a gap below the maximum is skipped rather than refilled.
func TestNextWSO2IDMaxIsNumericAndExact(t *testing.T) {
	h := newWSO2IDHarness(t)
	h.reset(0, 0)
	for _, id := range []string{
		"CCMPROJ-1", "CCMPROJ-9", "CCMPROJ-10", // 10 beats 9 only if compared as integers
		"CCMPROJ-PORTAL-500", // portal family: not "<key>-<digits>"
		"CCMPROJ-7x",         // trailing junk
		"CCMPROJ-2-3",        // dash inside the suffix
		"CCMPROJX-900",       // another key that starts with this key
		"XCCMPROJ-800",       // key as a suffix of another
		"ccmproj-700",        // case differs
		"CCMPROJ-9999999999", // ten digits: ignored, cannot be a counter value
	} {
		h.seed(ccmProjectID, id)
	}
	got, err := h.next(h.internal, ccmProjectID)
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	if got != "CCMPROJ-11" {
		t.Fatalf("got %q, want CCMPROJ-11", got)
	}
}

func TestNextWSO2IDCounterAheadIsUntouched(t *testing.T) {
	h := newWSO2IDHarness(t)
	h.reset(500, 0)
	for i := 1; i <= 25; i++ {
		h.seed(ccmProjectID, fmt.Sprintf("CCMPROJ-%d", i))
	}
	got, err := h.next(h.internal, ccmProjectID)
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	if got != "CCMPROJ-501" {
		t.Fatalf("got %q, want CCMPROJ-501 (plain increment, counter ahead of the data)", got)
	}
	if c := h.counter(ccmProjectID); c != 501 {
		t.Fatalf("counter = %d, want 501 (never lowered)", c)
	}
}

// A collision that is NOT the highest id (a hole filled by hand) still
// terminates on the next free id and the counter lands on it.
func TestNextWSO2IDSkipsIsolatedCollision(t *testing.T) {
	h := newWSO2IDHarness(t)
	h.reset(4, 0)
	h.seed(ccmProjectID, "CCMPROJ-5")
	got, err := h.next(h.internal, ccmProjectID)
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	if got != "CCMPROJ-6" || h.counter(ccmProjectID) != 6 {
		t.Fatalf("got %q counter %d, want CCMPROJ-6 / 6", got, h.counter(ccmProjectID))
	}
}

// The collision is with rows the customer cannot see (another project's rows
// that carry this project's key prefix, e.g. an item moved between projects).
func TestNextWSO2IDCustomerSeesCollisionsItCannotRead(t *testing.T) {
	h := newWSO2IDHarness(t)
	h.reset(0, 0)
	for i := 1; i <= 25; i++ {
		h.seed(ccmOtherProj, fmt.Sprintf("CCMPROJ-%d", i)) // customer is not a member of ccmOtherProj
	}
	var visible int
	if err := h.scoped.QueryRow(h.customer, `SELECT count(*) FROM work_item WHERE wso2_id LIKE 'CCMPROJ-%'`).Scan(&visible); err != nil {
		t.Fatal(err)
	}
	if visible != 0 {
		t.Fatalf("fixture invalid: customer sees %d of the hidden rows, want 0 (RLS not in force; connect as a non-bypassing role)", visible)
	}
	got, err := h.next(h.customer, ccmProjectID)
	if err != nil {
		t.Fatalf("next as customer: %v", err)
	}
	if got != "CCMPROJ-26" {
		t.Fatalf("got %q, want CCMPROJ-26: a customer-initiated create must see ids it cannot read", got)
	}
}

// The elevated visibility lasts only for the call: in one transaction the
// caller's rows and setting are identical before and after, and a hidden row
// stays hidden.
func TestNextWSO2IDCallerVisibilityUnchanged(t *testing.T) {
	h := newWSO2IDHarness(t)
	h.reset(0, 0)
	h.seed(ccmProjectID, "CCMPROJ-1") // visible to the customer
	h.seed(ccmOtherProj, "CCMPROJ-2") // hidden from the customer
	type snap struct {
		visible  int
		setting  string
		ownedIDs string
	}
	read := func(tx pgx.Tx) snap {
		var s snap
		if err := tx.QueryRow(context.Background(),
			`SELECT (SELECT count(*) FROM work_item), current_setting('app.is_internal', true),
			        coalesce((SELECT string_agg(wso2_id, ',' ORDER BY wso2_id) FROM work_item), '')`).
			Scan(&s.visible, &s.setting, &s.ownedIDs); err != nil {
			t.Fatalf("snapshot: %v", err)
		}
		return s
	}
	var before, after, inside snap
	var id string
	err := h.scoped.InTx(h.customer, func(tx pgx.Tx) error {
		before = read(tx)
		if err := tx.QueryRow(context.Background(), `SELECT next_wso2_id($1::uuid)`, ccmProjectID).Scan(&id); err != nil {
			return err
		}
		after = read(tx)
		inside = after
		return nil
	})
	if err != nil {
		t.Fatalf("tx: %v", err)
	}
	if id != "CCMPROJ-3" {
		t.Fatalf("id = %q, want CCMPROJ-3 (1 and 2 exist, 2 only hidden)", id)
	}
	if before != after {
		t.Fatalf("visibility changed across the call: before %+v after %+v", before, after)
	}
	if before.visible != 1 || before.ownedIDs != "CCMPROJ-1" || before.setting == "true" {
		t.Fatalf("customer snapshot unexpected (RLS not in force?): %+v", before)
	}
	_ = inside
}

func TestNextWSO2IDConcurrentCreatesGetDistinctIDs(t *testing.T) {
	h := newWSO2IDHarness(t)
	h.reset(0, 0)
	for i := 1; i <= 10; i++ {
		h.seed(ccmProjectID, fmt.Sprintf("CCMPROJ-%d", i))
	}
	const workers = 8
	ids := make([]string, workers)
	errs := make([]error, workers)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			<-start
			// A real create: allocate, then insert the row in the same
			// transaction, so the project lock is held until the id is committed.
			ctx := h.internal
			if w%2 == 1 {
				ctx = h.customer
			}
			errs[w] = h.scoped.InTx(ctx, func(tx pgx.Tx) error {
				var id string
				if err := tx.QueryRow(context.Background(), `SELECT next_wso2_id($1::uuid)`, ccmProjectID).Scan(&id); err != nil {
					return err
				}
				ids[w] = id
				time.Sleep(20 * time.Millisecond) // widen the window a racing allocator would exploit
				_, err := tx.Exec(context.Background(), `INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type, project_id)
					VALUES (gen_random_uuid(), now(), now(), 't', 't', $1, $2, 'concurrent', 'CASE'::work_item_type_enum, $3)`,
					fmt.Sprintf("NWCONC%06d", w), id, ccmProjectID)
				return err
			})
		}(w)
	}
	close(start)
	wg.Wait()
	seen := map[string]bool{}
	for w := range ids {
		if errs[w] != nil {
			t.Fatalf("worker %d: %v", w, errs[w])
		}
		if seen[ids[w]] {
			t.Fatalf("duplicate id %s in %v", ids[w], ids)
		}
		seen[ids[w]] = true
	}
	for n := 11; n < 11+workers; n++ {
		if !seen[fmt.Sprintf("CCMPROJ-%d", n)] {
			t.Fatalf("ids %v are not the contiguous CCMPROJ-11..CCMPROJ-%d", ids, 10+workers)
		}
	}
}

func TestNextWSO2IDMissingProjectKeepsMessage(t *testing.T) {
	h := newWSO2IDHarness(t)
	const missing = "c7000000-0000-0000-0000-0000000000ff"
	_, err := h.next(h.internal, missing)
	if err == nil || !strings.Contains(err.Error(), "next_wso2_id: no project with id "+missing) {
		t.Fatalf("err = %v, want 'next_wso2_id: no project with id %s'", err, missing)
	}
	if !strings.Contains(err.Error(), "P0001") {
		t.Fatalf("err = %v, want SQLSTATE P0001 (case_repo maps it)", err)
	}
}
