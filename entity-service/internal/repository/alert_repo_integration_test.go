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

package repository

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// Runs against a database with migrations 0308, 0309 and 100029 applied, and only when ENTITY_TEST_DATABASE_URL is
// set. Every alert row it writes has a message id starting it-alert- and is removed again.

func newAlertIntegrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("ENTITY_TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("ENTITY_TEST_DATABASE_URL is not set; skipping the live-database tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	// Row-level security hides the table from anyone but the system identity, so cleanup runs as it too.
	clean := func() {
		if _, err := NewScoped(pool).Exec(WithSystemIdentity(ctx), `DELETE FROM alert WHERE message_id LIKE 'it-alert-%'`); err != nil {
			t.Fatalf("clean: %v", err)
		}
	}
	clean()
	t.Cleanup(clean)
	return pool
}

func integrationAlert(id, messageID string, index int) domain.NewAlert {
	a := domain.NewAlert{
		ID: id, Source: "prometheus", Alert: json.RawMessage(`{"service":"svc-it","metric_name":"cpu"}`), Fingerprint: "fp-it",
		Tier: domain.AlertTierHigh, MessageID: messageID, MessageIndex: index, ReceivedOn: time.Date(2026, 11, 25, 8, 0, 0, 0, time.UTC),
	}
	if index == 0 {
		a.RawBody, a.ContentType = []byte(`{"status":"firing"}`), "application/json"
	}
	return a
}

func TestAlertRepositoryIntegration_ReservesAscendingIDs(t *testing.T) {
	pool := newAlertIntegrationPool(t)
	ids, err := NewAlertRepository(NewScoped(pool)).ReserveAlertIDs(WithSystemIdentity(context.Background()), 3)
	if err != nil || len(ids) != 3 {
		t.Fatalf("ReserveAlertIDs = %v, %v; want 3 ids", ids, err)
	}
	for i, id := range ids {
		if len(id) != len("ALT000000000") || !strings.HasPrefix(id, "ALT") || (i > 0 && id <= ids[i-1]) {
			t.Fatalf("ids = %v, want ALT plus nine digits, ascending", ids)
		}
	}
}

func TestAlertRepositoryIntegration_KeepsRawBodiesAndErrorRows(t *testing.T) {
	pool := newAlertIntegrationPool(t)
	repo := NewAlertRepository(NewScoped(pool))
	ctx := WithSystemIdentity(context.Background())

	second := integrationAlert("ALT900000002", "it-alert-1", 1)
	errorRow := integrationAlert("ALT900000003", "it-alert-2", 0)
	errorRow.Alert, errorRow.Fingerprint, errorRow.Source = nil, "", ""
	errorRow.ErrorReason, errorRow.Error, errorRow.RawBody = domain.AlertErrorNoSource, "no source property", []byte("x\x00y")
	n, err := repo.InsertAlerts(ctx, []domain.NewAlert{integrationAlert("ALT900000001", "it-alert-1", 0), second, errorRow})
	if err != nil || n != 3 {
		t.Fatalf("InsertAlerts = %d, %v; want 3", n, err)
	}

	rows, err := NewScoped(pool).Query(ctx, `SELECT id, status, raw_body IS NOT NULL, source IS NULL, fingerprint FROM alert WHERE message_id LIKE 'it-alert-%' ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	type row struct {
		id, status  string
		raw, noSrc  bool
		fingerprint *string
	}
	var got []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.status, &r.raw, &r.noSrc, &r.fingerprint); err != nil {
			t.Fatal(err)
		}
		got = append(got, r)
	}
	if len(got) != 3 || got[0].status != domain.AlertStatusNew || !got[0].raw || got[1].raw || got[1].status != domain.AlertStatusNew ||
		got[2].status != domain.AlertStatusError || !got[2].noSrc || got[2].fingerprint != nil || !got[2].raw {
		t.Fatalf("rows = %+v; want two NEW alerts with the body on the first only, and an ERROR row with no source or fingerprint", got)
	}
	var raw []byte
	if err := NewScoped(pool).QueryRow(ctx, `SELECT raw_body FROM alert WHERE id = 'ALT900000003'`).Scan(&raw); err != nil || string(raw) != "x\x00y" {
		t.Fatalf("raw body = %q (%v), want the NUL byte kept", raw, err)
	}
}

func TestAlertRepositoryIntegration_RedeliveredMessageStoresNothingNew(t *testing.T) {
	pool := newAlertIntegrationPool(t)
	repo := NewAlertRepository(NewScoped(pool))
	ctx := WithSystemIdentity(context.Background())
	first := integrationAlert("ALT900000010", "it-alert-10", 0)
	if n, err := repo.InsertAlerts(ctx, []domain.NewAlert{first}); err != nil || n != 1 {
		t.Fatalf("InsertAlerts = %d, %v", n, err)
	}
	again := first
	again.ID = "ALT900000011" // a redelivery reserves new ids
	if n, err := repo.InsertAlerts(ctx, []domain.NewAlert{again}); err != nil || n != 0 {
		t.Fatalf("redelivery = %d, %v; want nothing stored", n, err)
	}
}

func TestAlertRepositoryIntegration_RefusesWhatCanNeverBeStored(t *testing.T) {
	pool := newAlertIntegrationPool(t)
	repo := NewAlertRepository(NewScoped(pool))
	ctx := WithSystemIdentity(context.Background())
	noFingerprint := integrationAlert("ALT900000020", "it-alert-20", 0)
	noFingerprint.Fingerprint = ""
	if _, err := repo.InsertAlerts(ctx, []domain.NewAlert{noFingerprint}); err == nil {
		t.Fatal("an alert row with no fingerprint was stored; the processor could never lock or group it")
	}
	badTier := integrationAlert("ALT900000021", "it-alert-21", 0)
	badTier.Tier = "urgent"
	if _, err := repo.InsertAlerts(ctx, []domain.NewAlert{badTier}); err == nil {
		t.Fatal("an unknown tier was stored")
	}
}

func TestAlertRepositoryIntegration_NotVisibleWithoutInternalIdentity(t *testing.T) {
	pool := newAlertIntegrationPool(t)
	ctx := context.Background()
	var bypass bool
	if err := pool.QueryRow(ctx, `SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = current_user`).Scan(&bypass); err != nil {
		t.Fatal(err)
	}
	if bypass {
		t.Skip("ENTITY_TEST_DATABASE_URL connects as a role that bypasses row-level security; use an ordinary role to run this test")
	}
	if _, err := NewAlertRepository(NewScoped(pool)).InsertAlerts(WithSystemIdentity(ctx), []domain.NewAlert{integrationAlert("ALT900000030", "it-alert-30", 0)}); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM alert WHERE message_id LIKE 'it-alert-%'`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("rows visible without the internal identity = %d (%v), want 0", n, err)
	}
}
