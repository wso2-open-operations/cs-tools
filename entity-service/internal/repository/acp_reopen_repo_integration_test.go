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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

// ACPReopenRepository against a real Postgres with every migration applied.
// Skipped without ENTITY_TEST_DATABASE_URL:
//
//	ENTITY_TEST_DATABASE_URL=postgres://... go test ./internal/repository/ -run ACPReopen

package repository_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	acpReopenTestProjectID = "acf00000-0000-4000-8000-000000000001"
	acpReopenTestSfID      = "a0dACPREOPENTEST01"
	acpReopenTestTag       = "acp-reopen-test"
)

// seedACPReopenProject writes one project with the given closure states and
// suspensionProcessState, removed again when the test ends.
func seedACPReopenProject(t *testing.T, pool *pgxpool.Pool, endState, invoiceState string, blob string) {
	t.Helper()
	ctx := context.Background()
	cleanup := func() {
		if _, err := pool.Exec(ctx, `DELETE FROM project WHERE id = $1`, acpReopenTestProjectID); err != nil {
			t.Fatalf("clean project: %v", err)
		}
	}
	cleanup()
	t.Cleanup(cleanup)
	_, err := pool.Exec(ctx, `
		INSERT INTO project (id, created_on, updated_on, created_by, updated_by, key, sf_id, name,
		                     end_date_closure_state, invoice_due_date_closure_state, wso2_closure_state, suspension_process_state)
		VALUES ($1, now(), now(), $2, $2, 'ACPREOPENTEST', $3, '[S] ACP Reopen Test',
		        $4::end_date_closure_state_enum, $5::invoice_due_date_closure_state_enum, 'SUSPENDED', $6::jsonb)`,
		acpReopenTestProjectID, acpReopenTestTag, acpReopenTestSfID, endState, invoiceState, blob)
	if err != nil {
		t.Fatalf("seed project: %v", err)
	}
}

func acpReopenPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("ENTITY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ENTITY_TEST_DATABASE_URL is not set; skipping the live-database tests")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func acpDeref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func readACPReopenRow(t *testing.T, pool *pgxpool.Pool) (end, invoice, overall, updatedBy string, blob json.RawMessage) {
	t.Helper()
	err := pool.QueryRow(context.Background(), `
		SELECT end_date_closure_state::TEXT, invoice_due_date_closure_state::TEXT, wso2_closure_state::TEXT,
		       updated_by, suspension_process_state
		FROM project WHERE id = $1`, acpReopenTestProjectID).Scan(&end, &invoice, &overall, &updatedBy, &blob)
	if err != nil {
		t.Fatalf("read project: %v", err)
	}
	return end, invoice, overall, updatedBy, blob
}

func TestACPReopenRepoIntegration_GetProjectBySfID(t *testing.T) {
	pool := acpReopenPool(t)
	blob := `{"based_on_subscription_end_date": {"event_type": "suspend"}}`
	seedACPReopenProject(t, pool, "CLOSED", "OPEN", blob)
	repo := repository.NewACPReopenRepository(repository.NewScoped(pool))

	got, err := repo.GetProjectBySfID(context.Background(), acpReopenTestSfID)
	if err != nil || got == nil {
		t.Fatalf("GetProjectBySfID: %v %v", got, err)
	}
	if got.ID != acpReopenTestProjectID || got.Key != "ACPREOPENTEST" || got.Name != "[S] ACP Reopen Test" ||
		acpDeref(got.EndDateClosureState) != "CLOSED" || acpDeref(got.InvoiceDueDateClosureState) != "OPEN" ||
		got.ComplianceViolationClosureState != nil || acpDeref(got.ClosureState) != "SUSPENDED" {
		t.Errorf("project = %+v", got)
	}
	var section map[string]map[string]string
	if err := json.Unmarshal(got.SuspensionProcessState, &section); err != nil || section["based_on_subscription_end_date"]["event_type"] != "suspend" {
		t.Errorf("suspensionProcessState = %s (%v)", got.SuspensionProcessState, err)
	}

	none, err := repo.GetProjectBySfID(context.Background(), "a0dNOSUCHPROJECT01")
	if err != nil || none != nil {
		t.Errorf("unknown sf id: %v %v, want nil, nil", none, err)
	}
}

func TestACPReopenRepoIntegration_ReopenWritesAndRecomputes(t *testing.T) {
	pool := acpReopenPool(t)
	seedACPReopenProject(t, pool, "CLOSED", "OPEN", `{"based_on_subscription_end_date": {"event_type": "suspend"}}`)
	repo := repository.NewACPReopenRepository(repository.NewScoped(pool))

	newBlob := json.RawMessage(`{"based_on_subscription_end_date":{"event_type":"open"}}`)
	var seen domain.ACPReopenProject
	var committedWith domain.ACPReopenOutcome
	outcome, done, err := repo.Reopen(context.Background(), acpReopenTestProjectID,
		func(cur domain.ACPReopenProject) (domain.ACPReopenWrite, bool, error) {
			seen = cur
			return domain.ACPReopenWrite{ReopenEndDate: true, SuspensionProcessState: newBlob}, true, nil
		},
		func(_ context.Context, o domain.ACPReopenOutcome) error { committedWith = o; return nil }, nil)
	if err != nil || !done {
		t.Fatalf("Reopen: done=%v err=%v", done, err)
	}
	if acpDeref(seen.EndDateClosureState) != "CLOSED" {
		t.Errorf("plan saw end=%s, want the locked row", acpDeref(seen.EndDateClosureState))
	}
	if !outcome.Open || acpDeref(outcome.ClosureState) != "OPEN" || !committedWith.Open {
		t.Errorf("outcome = %+v, beforeCommit saw %+v", outcome, committedWith)
	}
	end, invoice, overall, by, blob := readACPReopenRow(t, pool)
	if end != "OPEN" || invoice != "OPEN" || overall != "OPEN" || by != domain.ACPReopenActor {
		t.Errorf("row = end %s invoice %s overall %s by %s", end, invoice, overall, by)
	}
	var stored map[string]map[string]string
	if err := json.Unmarshal(blob, &stored); err != nil || stored["based_on_subscription_end_date"]["event_type"] != "open" {
		t.Errorf("stored state = %s", blob)
	}
}

func TestACPReopenRepoIntegration_OtherTrackKeepsTheProjectSuspended(t *testing.T) {
	pool := acpReopenPool(t)
	seedACPReopenProject(t, pool, "CLOSED", "SUSPENDED",
		`{"based_on_subscription_end_date": {"event_type": "suspend"}, "based_on_due_invoices": {"event_type": "suspend"}}`)
	repo := repository.NewACPReopenRepository(repository.NewScoped(pool))

	called := false
	outcome, done, err := repo.Reopen(context.Background(), acpReopenTestProjectID,
		func(cur domain.ACPReopenProject) (domain.ACPReopenWrite, bool, error) {
			return domain.ACPReopenWrite{ReopenEndDate: true, SuspensionProcessState: cur.SuspensionProcessState}, true, nil
		},
		func(_ context.Context, o domain.ACPReopenOutcome) error { called = true; return nil }, nil)
	if err != nil || !done || !called {
		t.Fatalf("Reopen: done=%v called=%v err=%v", done, called, err)
	}
	if outcome.Open || acpDeref(outcome.ClosureState) != "SUSPENDED" {
		t.Errorf("outcome = %+v, want SUSPENDED (the invoice track is still suspended)", outcome)
	}
	end, invoice, overall, _, _ := readACPReopenRow(t, pool)
	if end != "OPEN" || invoice != "SUSPENDED" || overall != "SUSPENDED" {
		t.Errorf("row = end %s invoice %s overall %s", end, invoice, overall)
	}
}

func TestACPReopenRepoIntegration_BeforeCommitFailureRollsBack(t *testing.T) {
	pool := acpReopenPool(t)
	seedACPReopenProject(t, pool, "CLOSED", "OPEN", `{"based_on_subscription_end_date": {"event_type": "suspend"}}`)
	repo := repository.NewACPReopenRepository(repository.NewScoped(pool))

	refused := errors.New("salesforce refused")
	_, done, err := repo.Reopen(context.Background(), acpReopenTestProjectID,
		func(cur domain.ACPReopenProject) (domain.ACPReopenWrite, bool, error) {
			return domain.ACPReopenWrite{ReopenEndDate: true, SuspensionProcessState: json.RawMessage(`{"x":1}`)}, true, nil
		},
		func(context.Context, domain.ACPReopenOutcome) error { return refused }, nil)
	if !errors.Is(err, refused) || done {
		t.Fatalf("Reopen: done=%v err=%v, want the beforeCommit error", done, err)
	}
	end, _, overall, by, blob := readACPReopenRow(t, pool)
	if end != "CLOSED" || overall != "SUSPENDED" || by != acpReopenTestTag {
		t.Errorf("row changed: end %s overall %s by %s", end, overall, by)
	}
	if string(blob) == `{"x": 1}` {
		t.Errorf("suspensionProcessState was kept: %s", blob)
	}
}

func TestACPReopenRepoIntegration_NothingToWrite(t *testing.T) {
	pool := acpReopenPool(t)
	seedACPReopenProject(t, pool, "CLOSED", "OPEN", `{"based_on_subscription_end_date": {"event_type": "suspend"}}`)
	repo := repository.NewACPReopenRepository(repository.NewScoped(pool))

	called := false
	_, done, err := repo.Reopen(context.Background(), acpReopenTestProjectID,
		func(domain.ACPReopenProject) (domain.ACPReopenWrite, bool, error) {
			return domain.ACPReopenWrite{}, false, nil
		},
		func(context.Context, domain.ACPReopenOutcome) error { called = true; return nil }, nil)
	if err != nil || done || called {
		t.Fatalf("done=%v called=%v err=%v, want nothing", done, called, err)
	}
	if end, _, _, by, _ := readACPReopenRow(t, pool); end != "CLOSED" || by != acpReopenTestTag {
		t.Errorf("row changed: end %s by %s", end, by)
	}

	// An unknown project id is nothing to do as well.
	_, done, err = repo.Reopen(context.Background(), "acf00000-0000-4000-8000-0000000000ff",
		func(domain.ACPReopenProject) (domain.ACPReopenWrite, bool, error) {
			t.Error("plan called for a project that does not exist")
			return domain.ACPReopenWrite{}, false, nil
		}, nil, nil)
	if err != nil || done {
		t.Errorf("unknown project: done=%v err=%v", done, err)
	}
}

// The notice is queued in the same transaction as the reopen: kept with a
// committed reopen, gone with a rolled-back one.
func TestACPReopenRepoIntegration_NoticeIsQueuedWithTheReopen(t *testing.T) {
	pool := acpReopenPool(t)
	ctx := context.Background()
	clean := func() {
		if _, err := pool.Exec(ctx, `DELETE FROM event_outbox WHERE entity_type = $1 AND entity_id = $2`,
			repository.ACPReactivationOutboxEntityType, acpReopenTestProjectID); err != nil {
			t.Fatalf("clean outbox: %v", err)
		}
	}
	clean()
	t.Cleanup(clean)
	countQueued := func() int {
		var n int
		if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM event_outbox WHERE entity_type = $1 AND entity_id = $2 AND published_on IS NULL`,
			repository.ACPReactivationOutboxEntityType, acpReopenTestProjectID).Scan(&n); err != nil {
			t.Fatalf("count outbox: %v", err)
		}
		return n
	}
	repo := repository.NewACPReopenRepository(repository.NewScoped(pool))
	plan := func(domain.ACPReopenProject) (domain.ACPReopenWrite, bool, error) {
		return domain.ACPReopenWrite{ReopenEndDate: true, SuspensionProcessState: json.RawMessage(`{"based_on_subscription_end_date":{"event_type":"open"}}`)}, true, nil
	}
	notice := func(o domain.ACPReopenOutcome) (json.RawMessage, error) {
		return json.RawMessage(`{"projectId":"` + acpReopenTestProjectID + `"}`), nil
	}

	seedACPReopenProject(t, pool, "CLOSED", "OPEN", `{"based_on_subscription_end_date": {"event_type": "suspend"}}`)
	if _, _, err := repo.Reopen(ctx, acpReopenTestProjectID, plan,
		func(context.Context, domain.ACPReopenOutcome) error { return errors.New("salesforce refused") }, notice); err == nil {
		t.Fatal("a beforeCommit failure was not returned")
	}
	if n := countQueued(); n != 0 {
		t.Errorf("a rolled-back reopen left %d queued notices", n)
	}
	if _, done, err := repo.Reopen(ctx, acpReopenTestProjectID, plan, nil, notice); err != nil || !done {
		t.Fatalf("Reopen: done=%v err=%v", done, err)
	}
	if n := countQueued(); n != 1 {
		t.Errorf("queued notices = %d, want 1", n)
	}

	// The drainer's side: publish, failure, park.
	outbox := repository.NewACPReactivationOutbox(repository.NewScoped(pool))
	ids, err := outbox.Pending(ctx, 1000)
	if err != nil {
		t.Fatalf("Pending: %v", err)
	}
	var id int64
	for _, candidate := range ids {
		var entity string
		_ = pool.QueryRow(ctx, `SELECT entity_id::text FROM event_outbox WHERE id = $1`, candidate).Scan(&entity)
		if entity == acpReopenTestProjectID {
			id = candidate
		}
	}
	if id == 0 {
		t.Fatal("the queued notice is not pending")
	}
	if _, err := outbox.Process(ctx, id, func(context.Context, string, json.RawMessage) error { return errors.New("event hub down") }); err == nil {
		t.Fatal("a publish failure was not returned")
	}
	if parked, err := outbox.RecordFailure(ctx, id, "event hub down", 2); err != nil || parked {
		t.Fatalf("first failure: parked=%v err=%v", parked, err)
	}
	if n := countQueued(); n != 1 {
		t.Errorf("a failed publish removed the notice: queued = %d", n)
	}
	var gotProject string
	var gotPayload json.RawMessage
	if processed, err := outbox.Process(ctx, id, func(_ context.Context, projectID string, payload json.RawMessage) error {
		gotProject, gotPayload = projectID, payload
		return nil
	}); err != nil || !processed {
		t.Fatalf("Process: processed=%v err=%v", processed, err)
	}
	if gotProject != acpReopenTestProjectID || !strings.Contains(string(gotPayload), acpReopenTestProjectID) {
		t.Errorf("published %s %s", gotProject, gotPayload)
	}
	if n := countQueued(); n != 0 {
		t.Errorf("a published notice is still queued: %d", n)
	}
}

// SectionsOnly rewrites the record with no track change: the daily check
// finishing a project a person set back to Open.
func TestACPReopenRepoIntegration_SectionsOnly(t *testing.T) {
	pool := acpReopenPool(t)
	seedACPReopenProject(t, pool, "OPEN", "OPEN", `{"based_on_subscription_end_date": {"event_type": "suspend"}}`)
	repo := repository.NewACPReopenRepository(repository.NewScoped(pool))
	outcome, done, err := repo.Reopen(context.Background(), acpReopenTestProjectID,
		func(domain.ACPReopenProject) (domain.ACPReopenWrite, bool, error) {
			return domain.ACPReopenWrite{SectionsOnly: true, SuspensionProcessState: json.RawMessage(`{"based_on_subscription_end_date":{"event_type":"open"}}`)}, true, nil
		}, nil, nil)
	if err != nil || !done || !outcome.Open {
		t.Fatalf("Reopen: done=%v open=%v err=%v", done, outcome.Open, err)
	}
	end, _, overall, by, blob := readACPReopenRow(t, pool)
	if end != "OPEN" || overall != "OPEN" || by != domain.ACPReopenActor || !strings.Contains(string(blob), `"open"`) {
		t.Errorf("row = end %s overall %s by %s blob %s", end, overall, by, blob)
	}
}

// The two queries the event pre-filter and the daily check read.
func TestACPReopenRepoIntegration_SuspendedListings(t *testing.T) {
	pool := acpReopenPool(t)
	ctx := context.Background()
	repo := repository.NewACPReopenRepository(repository.NewScoped(pool))
	listed := func() bool {
		after := ""
		for {
			page, err := repo.ListACPNotedProjects(ctx, after, 50)
			if err != nil {
				t.Fatalf("ListACPNotedProjects: %v", err)
			}
			for _, p := range page {
				if p.ID == acpReopenTestProjectID {
					return true
				}
			}
			if len(page) < 50 {
				return false
			}
			after = page[len(page)-1].ID
		}
	}

	seedACPReopenProject(t, pool, "OPEN", "SUSPENDED", `{"based_on_due_invoices": {"event_type": "suspend"}}`)
	if found, err := repo.AnyACPSuspendedInvoiceTrack(ctx); err != nil || !found {
		t.Errorf("AnyACPSuspendedInvoiceTrack = %v, %v, want true", found, err)
	}
	if !listed() {
		t.Error("an ACP-suspended project is not listed for the daily check")
	}

	// Suspended by hand: no ACP record, so not listed.
	seedACPReopenProject(t, pool, "OPEN", "SUSPENDED", `{"based_on_due_invoices": {"event_type": "7_days_notice"}}`)
	if listed() {
		t.Error("a project suspended by hand is listed")
	}
	// A record that is not an object is never a match.
	seedACPReopenProject(t, pool, "OPEN", "SUSPENDED", `"suspend"`)
	if listed() {
		t.Error("a malformed record is listed")
	}
}
