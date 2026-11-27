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
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/model"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// Runs against a database with migrations 0306, 0308 and 100029 applied, and only when ENTITY_TEST_DATABASE_URL is set.
func newAlertIntegrationPool(t *testing.T) *pgxpool.Pool {
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
	// Row-level security hides the table from anyone but the system identity, so cleanup runs as it too.
	clean := func() {
		if _, err := NewScoped(pool).Exec(WithSystemIdentity(ctx), `DELETE FROM alert WHERE service LIKE 'it-alert-%'`); err != nil {
			t.Fatalf("clean alerts: %v", err)
		}
	}
	clean()
	t.Cleanup(clean)
	return pool
}

// integrationAlert is an alert of service tag, which the tests find and clean up by.
func integrationAlert(number, tag string) domain.NewAlert {
	return domain.NewAlert{
		Number:     number,
		Alert:      model.Alert{Source: "Azure Monitor", Service: tag, MetricName: "cpu", Environment: "prod", Severity: "critical", UniqueIdentifier: number},
		Tier:       domain.AlertTierPriority,
		ReceivedOn: time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC),
	}
}

func TestAlertRepositoryIntegration_StoresAlertsWithTheirUUIDs(t *testing.T) {
	pool := newAlertIntegrationPool(t)
	repo := NewAlertRepository(NewScoped(pool))
	ctx := WithSystemIdentity(context.Background())

	ids, err := repo.ReserveAlertIDs(ctx, 2)
	if err != nil {
		t.Fatalf("ReserveAlertIDs: %v", err)
	}
	if len(ids) != 2 || len(ids[0]) != len("ALT000000001") || ids[0] >= ids[1] {
		t.Fatalf("ids = %v, want two ascending ALT numbers", ids)
	}

	stored, err := repo.InsertAlerts(ctx, []domain.NewAlert{integrationAlert(ids[0], "it-alert-1"), integrationAlert(ids[1], "it-alert-1")})
	if err != nil || len(stored) != 2 || stored[0].Number != ids[0] || stored[1].Number != ids[1] || len(stored[0].ID) != 36 || stored[0].ID == stored[1].ID || stored[0].Alert.Environment != "prod" {
		t.Fatalf("InsertAlerts = %+v, %v; want both alerts back in order, each with its own UUID", stored, err)
	}

	var rows int
	var linked *string
	if err := NewScoped(pool).QueryRow(ctx, `SELECT count(*), max(incident_id::text) FROM alert WHERE service = 'it-alert-1'`).Scan(&rows, &linked); err != nil {
		t.Fatalf("count: %v", err)
	}
	if rows != 2 || linked != nil {
		t.Fatalf("rows = %d incident = %v, want 2 rows not yet in an incident", rows, linked)
	}
}

func TestAlertRepositoryIntegration_NotVisibleWithoutInternalIdentity(t *testing.T) {
	pool := newAlertIntegrationPool(t)
	// A superuser or BYPASSRLS role sees every row whatever the policy says, so this check needs an ordinary role.
	var bypassesRLS bool
	if err := pool.QueryRow(context.Background(), `SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = current_user`).Scan(&bypassesRLS); err != nil {
		t.Fatalf("role check: %v", err)
	}
	if bypassesRLS {
		t.Skip("ENTITY_TEST_DATABASE_URL connects as a role that bypasses row-level security; use an ordinary role to run this test")
	}
	repo := NewAlertRepository(NewScoped(pool))
	ctx := WithSystemIdentity(context.Background())
	ids, err := repo.ReserveAlertIDs(ctx, 1)
	if err != nil {
		t.Fatalf("ReserveAlertIDs: %v", err)
	}
	if _, err := repo.InsertAlerts(ctx, []domain.NewAlert{integrationAlert(ids[0], "it-alert-2")}); err != nil {
		t.Fatalf("InsertAlerts: %v", err)
	}

	// A plain pool query carries no app.is_internal setting, the same as a customer request.
	var rows int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM alert WHERE service = 'it-alert-2'`).Scan(&rows); err != nil {
		t.Fatalf("count: %v", err)
	}
	if rows != 0 {
		t.Fatalf("a non-internal caller saw %d alert rows, want 0", rows)
	}
}

func TestAlertRepositoryIntegration_KeepsTheRawBodyAndAlertColumns(t *testing.T) {
	pool := newAlertIntegrationPool(t)
	repo := NewAlertRepository(NewScoped(pool))
	ctx := WithSystemIdentity(context.Background())
	ids, err := repo.ReserveAlertIDs(ctx, 2)
	if err != nil {
		t.Fatalf("ReserveAlertIDs: %v", err)
	}

	// A batch keeps the body once, on its first alert; the second alert leaves its environment and severity empty.
	first := integrationAlert(ids[0], "it-alert-3")
	first.RawBody = []byte(`[{"a":1},{"a":2}]`)
	second := integrationAlert(ids[1], "it-alert-3")
	second.Alert.Environment, second.Alert.Severity = "", ""
	if stored, err := repo.InsertAlerts(ctx, []domain.NewAlert{first, second}); err != nil || len(stored) != 2 {
		t.Fatalf("insert batch stored %d (%v), want 2", len(stored), err)
	}

	type stored struct {
		raw                           []byte
		source, environment, severity *string
	}
	read := func(number string) stored {
		var s stored
		if err := NewScoped(pool).QueryRow(ctx, `SELECT raw_body, source, environment, severity FROM alert WHERE number = $1`, number).
			Scan(&s.raw, &s.source, &s.environment, &s.severity); err != nil {
			t.Fatalf("read %s: %v", number, err)
		}
		return s
	}
	if s := read(ids[0]); string(s.raw) != `[{"a":1},{"a":2}]` || s.source == nil || *s.source != "Azure Monitor" || s.environment == nil || *s.environment != "prod" {
		t.Fatalf("first row = %+v, want the body, source and environment", s)
	}
	// A fingerprint field is matched with =, so an empty one is ''; any other empty field is NULL.
	if s := read(ids[1]); s.raw != nil || s.environment == nil || *s.environment != "" || s.severity != nil {
		t.Fatalf("second row = %+v, want no body, an empty environment and a NULL severity", s)
	}
}

func TestAlertRepositoryIntegration_RefusesWhatCanNeverBeStored(t *testing.T) {
	pool := newAlertIntegrationPool(t)
	repo := NewAlertRepository(NewScoped(pool))
	ctx := WithSystemIdentity(context.Background())
	ids, err := repo.ReserveAlertIDs(ctx, 2)
	if err != nil {
		t.Fatalf("ReserveAlertIDs: %v", err)
	}
	nul := integrationAlert(ids[0], "it-alert-5")
	nul.Alert.Description = "bad \x00 byte"
	// The consumer cuts fields to their column's length; one that reaches the database longer is refused.
	long := integrationAlert(ids[1], "it-alert-6")
	long.Alert.Severity = strings.Repeat("x", 51)
	for name, row := range map[string]domain.NewAlert{"NUL in text": nul, "severity longer than its column": long} {
		_, err := repo.InsertAlerts(ctx, []domain.NewAlert{row})
		var sqlErr interface{ SQLState() string }
		if !errors.As(err, &sqlErr) || (sqlErr.SQLState()[:2] != "22" && sqlErr.SQLState()[:2] != "23") {
			t.Fatalf("%s: err = %v, want a class 22 or 23 error the consumer treats as permanent", name, err)
		}
	}
}
