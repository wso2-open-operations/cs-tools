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

// These tests run the Salesforce Account upsert and soft delete against a
// live PostgreSQL instance. The unit tests pin the statements' text; only a
// real database shows that the parameters infer the right types, that the
// COALESCEd columns really keep a stored value, and that the ledger row
// commits with the account row.
//
// Skipped unless ENTITY_TEST_DATABASE_URL is set, so `go test ./...` on a
// machine with no database stays green. Apply every migration in order first
// (account comes from 0012, deleted_on from 0171, the ledger from 0170):
//
//	createdb entity_test
//	for f in migrations/*.sql; do psql -v ON_ERROR_STOP=1 -d entity_test -f "$f"; done
//	ENTITY_TEST_DATABASE_URL="postgres:///entity_test" go test -v -run TestAccountSalesforceIntegration ./internal/repository/

package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// Fixture ids, distinct from every other integration file's.
const (
	sfiAccountID   = "5f1a0000-0000-4000-8000-000000000001"
	sfiManagerID   = "5f1a0000-0000-4000-8000-000000000002"
	sfiCsmID       = "5f1a0000-0000-4000-8000-000000000003"
	sfiSfID        = "001SFITEST000001AAA"
	sfiNewSfID     = "001SFITEST000002AAA"
	sfiNumber      = "ACC-SFI-1"
	sfiManagerMail = "sfi.manager@example.test"
)

func newAccountSalesforceIntegrationRepo(t *testing.T) (*accountRepo, *pgxpool.Pool) {
	t.Helper()
	dsn := os.Getenv("ENTITY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ENTITY_TEST_DATABASE_URL is not set; skipping the live-database tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	clean := func() {
		for _, stmt := range []string{
			`DELETE FROM salesforce_ingest_state WHERE entity = 'account' AND sf_id IN ($1, $2)`,
			`DELETE FROM account WHERE sf_id IN ($1, $2)`,
		} {
			if _, err := pool.Exec(ctx, stmt, sfiSfID, sfiNewSfID); err != nil {
				t.Fatalf("clean: %v", err)
			}
		}
		if _, err := pool.Exec(ctx, `DELETE FROM "user" WHERE id IN ($1, $2)`, sfiManagerID, sfiCsmID); err != nil {
			t.Fatalf("clean users: %v", err)
		}
	}
	clean()
	t.Cleanup(clean)

	// A ServiceNow-synced account: CSM-only columns set, and the SE-1
	// columns (CSM, vertical, deactivation date) carrying csm-sync values.
	for _, stmt := range []string{
		`INSERT INTO "user" (id, created_on, updated_on, user_name, email) VALUES
			('` + sfiManagerID + `', now(), now(), 'sfi.manager', '` + sfiManagerMail + `'),
			('` + sfiCsmID + `', now(), now(), 'sfi.csm', 'sfi.csm@example.test')`,
		`INSERT INTO account (id, created_on, updated_on, created_by, updated_by, name, number, sf_id,
			drive_location, ai_gen_response_enabled, suspension_process_state, support_tier,
			customer_success_manager_id, account_vertical, deactivation_date, phone)
		 VALUES ('` + sfiAccountID + `', now(), now(), 't', 't', 'Old Name', '` + sfiNumber + `', '` + sfiSfID + `',
			'drive://x', true, '{"step":1}', 'ENTERPRISE',
			'` + sfiCsmID + `', 'Banking', DATE '2027-01-31', '+1 555 0100')`,
	} {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	return &accountRepo{db: NewScoped(pool)}, pool
}

type sfiAccount struct {
	name, number, driveLocation, supportTier, suspension string
	aiGen                                                bool
	street, city, country, accountVertical, phone        *string
	accountManagerID, csmID                              *string
	activationDate, deactivationDate                     *time.Time
	deletedOn                                            *time.Time
}

func readSfiAccount(t *testing.T, pool *pgxpool.Pool, sfID string) sfiAccount {
	t.Helper()
	var a sfiAccount
	err := pool.QueryRow(context.Background(), `
		SELECT name, number, COALESCE(drive_location, ''), COALESCE(support_tier::text, ''),
		       COALESCE(suspension_process_state::text, ''), COALESCE(ai_gen_response_enabled, false),
		       street, city, country, account_vertical, phone,
		       account_manager_id::text, customer_success_manager_id::text,
		       activation_date, deactivation_date, deleted_on
		FROM account WHERE sf_id = $1`, sfID).Scan(
		&a.name, &a.number, &a.driveLocation, &a.supportTier, &a.suspension, &a.aiGen,
		&a.street, &a.city, &a.country, &a.accountVertical, &a.phone,
		&a.accountManagerID, &a.csmID, &a.activationDate, &a.deactivationDate, &a.deletedOn)
	if err != nil {
		t.Fatalf("read account %s: %v", sfID, err)
	}
	return a
}

func readSfiLedger(t *testing.T, pool *pgxpool.Pool, sfID string) (eventType, status string) {
	t.Helper()
	if err := pool.QueryRow(context.Background(),
		`SELECT event_type, status FROM salesforce_ingest_state WHERE entity = 'account' AND sf_id = $1`, sfID).Scan(&eventType, &status); err != nil {
		t.Fatalf("read ledger %s: %v", sfID, err)
	}
	return eventType, status
}

func sfiSearch(t *testing.T, r *accountRepo) int {
	t.Helper()
	// The search serves internal callers only (internalOnly on the route), so it
	// runs on that caller's identity, which the HTTP middleware supplies.
	_, total, err := r.SearchAccounts(WithSystemIdentity(context.Background()), domain.SearchAccountsRequest{
		Pagination: domain.Pagination{Limit: 10},
		Filters:    domain.SearchAccountsFilters{SearchQuery: sfiSfID},
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	return total
}

func sfiState(sfID, eventType string, on time.Time) domain.UpsertSalesforceIngestStateRequest {
	return domain.UpsertSalesforceIngestStateRequest{
		Entity: domain.SalesforceIngestEntityAccount, SfID: sfID, EventType: eventType,
		EventModifiedOn: on, Status: domain.SalesforceIngestSucceeded,
	}
}

func sp(s string) *string { return &s }

// TestAccountSalesforceIntegration walks UPDATED -> DELETED -> RESTORED on a
// ServiceNow-synced account, then CREATED for an account CSM does not have.
func TestAccountSalesforceIntegration(t *testing.T) {
	r, pool := newAccountSalesforceIntegrationRepo(t)
	ctx := context.Background()
	v1 := time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)
	activation := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	managerID := sfiManagerID

	// UPDATED carrying the CSM, vertical and deactivation date the sync loaded.
	csmSeed := sfiCsmID
	seedDeactivation := time.Date(2027, 1, 31, 0, 0, 0, 0, time.UTC)
	row := domain.SalesforceAccountUpsert{
		SfID: sfiSfID, Name: "Acme", Number: sfiSfID, KeepExistingPhone: true,
		Street: sp("1 Main St"), City: sp("Hamburg"), Country: sp("Germany"),
		AccountManagerID: &managerID, ActivationDate: &activation,
		CustomerSuccessManagerID: &csmSeed, AccountVertical: sp("Banking"), DeactivationDate: &seedDeactivation,
	}
	if err := r.UpsertFromSalesforce(ctx, row, sfiState(sfiSfID, domain.SalesforceEventUpdated, v1)); err != nil {
		t.Fatalf("UPDATED: %v", err)
	}
	a := readSfiAccount(t, pool, sfiSfID)
	if a.name != "Acme" || a.number != sfiNumber {
		t.Errorf("name/number = %q/%q, want Acme/%s (number kept)", a.name, a.number, sfiNumber)
	}
	if !eqp(a.street, "1 Main St") || !eqp(a.city, "Hamburg") || !eqp(a.country, "Germany") ||
		!eqp(a.accountManagerID, sfiManagerID) || a.activationDate == nil || !a.activationDate.Equal(activation) {
		t.Errorf("Salesforce columns not written: %+v", a)
	}
	if a.driveLocation != "drive://x" || !a.aiGen || a.supportTier != "ENTERPRISE" || a.suspension == "" || !eqp(a.phone, "+1 555 0100") {
		t.Errorf("CSM-only columns (or kept phone) changed: %+v", a)
	}
	if !eqp(a.csmID, sfiCsmID) || !eqp(a.accountVertical, "Banking") ||
		a.deactivationDate == nil || a.deactivationDate.Format(time.DateOnly) != "2027-01-31" {
		t.Errorf("SE-1 columns not written: csm %v vertical %v deactivation %v", a.csmID, a.accountVertical, a.deactivationDate)
	}
	if eventType, status := readSfiLedger(t, pool, sfiSfID); eventType != domain.SalesforceEventUpdated || status != string(domain.SalesforceIngestSucceeded) {
		t.Errorf("ledger = %s/%s, want UPDATED/SUCCEEDED", eventType, status)
	}
	if n := sfiSearch(t, r); n != 1 {
		t.Errorf("search total = %d, want 1", n)
	}

	// DELETED: deleted_on set, deactivation_date untouched, gone from search,
	// still readable by id.
	found, err := r.SoftDeleteBySfID(ctx, sfiSfID, sfiState(sfiSfID, domain.SalesforceEventDeleted, time.Now().UTC()))
	if err != nil || !found {
		t.Fatalf("DELETED: found %v err %v", found, err)
	}
	a = readSfiAccount(t, pool, sfiSfID)
	if a.deletedOn == nil || a.deactivationDate == nil || a.deactivationDate.Format(time.DateOnly) != "2027-01-31" {
		t.Errorf("after DELETED: deleted_on %v deactivation %v", a.deletedOn, a.deactivationDate)
	}
	if eventType, _ := readSfiLedger(t, pool, sfiSfID); eventType != domain.SalesforceEventDeleted {
		t.Errorf("ledger event type = %s, want DELETED", eventType)
	}
	if n := sfiSearch(t, r); n != 0 {
		t.Errorf("search total after DELETED = %d, want 0", n)
	}
	if _, err := r.GetAccountByID(WithSystemIdentity(ctx), sfiAccountID); err != nil {
		t.Errorf("GetAccountByID after DELETED: %v, want the row", err)
	}

	// RESTORED, carrying the pre-delete LastModifiedDate.
	if err := r.UpsertFromSalesforce(ctx, row, sfiState(sfiSfID, domain.SalesforceEventRestored, v1)); err != nil {
		t.Fatalf("RESTORED: %v", err)
	}
	if a = readSfiAccount(t, pool, sfiSfID); a.deletedOn != nil {
		t.Errorf("deleted_on after RESTORED = %v, want NULL", a.deletedOn)
	}
	if eventType, _ := readSfiLedger(t, pool, sfiSfID); eventType != domain.SalesforceEventRestored {
		t.Errorf("ledger event type = %s, want RESTORED", eventType)
	}
	if n := sfiSearch(t, r); n != 1 {
		t.Errorf("search total after RESTORED = %d, want 1", n)
	}

	// UPDATED with every SE-1 field NULL (cleared in Salesforce, or an email
	// with no user): the six columns clear, the rest of the row is unaffected.
	cleared := row
	cleared.CustomerSuccessManagerID, cleared.SecondaryTechnicalOwnerID, cleared.RenewalAccountManagerID = nil, nil, nil
	cleared.AccountVertical, cleared.LostReasonCategory, cleared.DeactivationDate = nil, nil, nil
	v2 := v1.Add(time.Minute)
	if err := r.UpsertFromSalesforce(ctx, cleared, sfiState(sfiSfID, domain.SalesforceEventUpdated, v2)); err != nil {
		t.Fatalf("UPDATED (cleared): %v", err)
	}
	a = readSfiAccount(t, pool, sfiSfID)
	if a.csmID != nil || a.accountVertical != nil || a.deactivationDate != nil {
		t.Errorf("SE-1 columns not cleared: csm %v vertical %v deactivation %v", a.csmID, a.accountVertical, a.deactivationDate)
	}
	if a.driveLocation != "drive://x" || !a.aiGen || a.supportTier != "ENTERPRISE" || !eqp(a.accountManagerID, sfiManagerID) {
		t.Errorf("clearing touched other columns: %+v", a)
	}

	// CREATED for an account CSM does not have: inserted, number = sf_id.
	deactivation := time.Date(2028, 3, 1, 0, 0, 0, 0, time.UTC)
	csmID := sfiCsmID
	created := domain.SalesforceAccountUpsert{
		SfID: sfiNewSfID, Name: "Newco", Number: sfiNewSfID, Phone: sp("+94 11 000 0000"),
		CustomerSuccessManagerID: &csmID, AccountVertical: sp("Retail"), DeactivationDate: &deactivation,
	}
	if err := r.UpsertFromSalesforce(ctx, created, sfiState(sfiNewSfID, domain.SalesforceEventCreated, v1)); err != nil {
		t.Fatalf("CREATED: %v", err)
	}
	a = readSfiAccount(t, pool, sfiNewSfID)
	if a.number != sfiNewSfID || !eqp(a.phone, "+94 11 000 0000") || !eqp(a.csmID, sfiCsmID) || !eqp(a.accountVertical, "Retail") ||
		a.deactivationDate == nil || !a.deactivationDate.Equal(deactivation) || a.deletedOn != nil {
		t.Errorf("inserted account = %+v", a)
	}
}

func eqp(p *string, want string) bool { return p != nil && *p == want }
