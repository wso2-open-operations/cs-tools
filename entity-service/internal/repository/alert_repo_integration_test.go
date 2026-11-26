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
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// Runs against a database with migrations 0306 and 100029 applied, and only when ENTITY_TEST_DATABASE_URL is set.
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
		if _, err := NewScoped(pool).Exec(WithSystemIdentity(ctx), `DELETE FROM alert WHERE message_id LIKE 'it-alert-%'`); err != nil {
			t.Fatalf("clean alerts: %v", err)
		}
	}
	clean()
	t.Cleanup(clean)
	return pool
}

func integrationAlert(id, messageID string, index int) domain.NewAlert {
	return domain.NewAlert{
		ID:           id,
		Source:       "azure",
		Alert:        json.RawMessage(`{"service":"svc-a","metric_name":"cpu"}`),
		Fingerprint:  "it-fingerprint",
		Tier:         domain.AlertTierPriority,
		MessageID:    messageID,
		MessageIndex: index,
		ReceivedOn:   time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC),
	}
}

func TestAlertRepositoryIntegration_RedeliveredMessageStoresNothingNew(t *testing.T) {
	pool := newAlertIntegrationPool(t)
	repo := NewAlertRepository(NewScoped(pool), true)
	ctx := WithSystemIdentity(context.Background())

	ids, err := repo.ReserveAlertIDs(ctx, 4)
	if err != nil {
		t.Fatalf("ReserveAlertIDs: %v", err)
	}
	if len(ids) != 4 || len(ids[0]) != len("ALT000000001") {
		t.Fatalf("ids = %v, want four ALT ids", ids)
	}
	for i := 1; i < len(ids); i++ {
		if ids[i-1] >= ids[i] {
			t.Fatalf("ids = %v, want each id strictly after the one before", ids)
		}
	}

	first, err := repo.InsertAlerts(ctx, []domain.NewAlert{integrationAlert(ids[0], "it-alert-1", 0), integrationAlert(ids[1], "it-alert-1", 1)})
	if err != nil || first != 2 {
		t.Fatalf("first insert stored %d (%v), want 2", first, err)
	}
	again, err := repo.InsertAlerts(ctx, []domain.NewAlert{integrationAlert(ids[2], "it-alert-1", 0), integrationAlert(ids[3], "it-alert-1", 1)})
	if err != nil || again != 0 {
		t.Fatalf("redelivered insert stored %d (%v), want 0", again, err)
	}

	var rows int
	var linked *string
	if err := NewScoped(pool).QueryRow(ctx, `SELECT count(*), max(incident_id::text) FROM alert WHERE message_id = 'it-alert-1'`).Scan(&rows, &linked); err != nil {
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
	repo := NewAlertRepository(NewScoped(pool), true)
	ctx := WithSystemIdentity(context.Background())
	ids, err := repo.ReserveAlertIDs(ctx, 1)
	if err != nil {
		t.Fatalf("ReserveAlertIDs: %v", err)
	}
	if _, err := repo.InsertAlerts(ctx, []domain.NewAlert{integrationAlert(ids[0], "it-alert-2", 0)}); err != nil {
		t.Fatalf("InsertAlerts: %v", err)
	}

	// A plain pool query carries no app.is_internal setting, the same as a customer request.
	var rows int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM alert WHERE message_id = 'it-alert-2'`).Scan(&rows); err != nil {
		t.Fatalf("count: %v", err)
	}
	if rows != 0 {
		t.Fatalf("a non-internal caller saw %d alert rows, want 0", rows)
	}
}

func TestAlertRepositoryIntegration_KeepsRawBodiesAndErrorRows(t *testing.T) {
	pool := newAlertIntegrationPool(t)
	repo := NewAlertRepository(NewScoped(pool), true)
	ctx := WithSystemIdentity(context.Background())
	ids, err := repo.ReserveAlertIDs(ctx, 3)
	if err != nil {
		t.Fatalf("ReserveAlertIDs: %v", err)
	}

	// A batch keeps the body once, on its first row.
	first := integrationAlert(ids[0], "it-alert-3", 0)
	first.RawBody, first.ContentType = []byte(`[{"a":1},{"a":2}]`), "application/json"
	if n, err := repo.InsertAlerts(ctx, []domain.NewAlert{first, integrationAlert(ids[1], "it-alert-3", 1)}); err != nil || n != 2 {
		t.Fatalf("insert batch stored %d (%v), want 2", n, err)
	}
	// A message with no source that could not be normalised, whose body holds a NUL byte jsonb could not.
	errorRow := domain.NewAlert{
		ID: ids[2], Tier: domain.AlertTierStandard, MessageID: "it-alert-4", MessageIndex: 0,
		RawBody: []byte("bad\x00body"), ContentType: "text/plain",
		ErrorReason: "MissingSource", Error: "the message has no source property",
		ReceivedOn: time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC),
	}
	if n, err := repo.InsertAlerts(ctx, []domain.NewAlert{errorRow}); err != nil || n != 1 {
		t.Fatalf("insert error row stored %d (%v), want 1", n, err)
	}

	type stored struct {
		raw                         []byte
		contentType, source, reason *string
		hasAlert                    bool
	}
	read := func(messageID string, index int) stored {
		var s stored
		if err := NewScoped(pool).QueryRow(ctx, `SELECT raw_body, content_type, source, error_reason, alert IS NOT NULL FROM alert WHERE message_id = $1 AND message_index = $2`, messageID, index).
			Scan(&s.raw, &s.contentType, &s.source, &s.reason, &s.hasAlert); err != nil {
			t.Fatalf("read %s/%d: %v", messageID, index, err)
		}
		return s
	}
	if s := read("it-alert-3", 0); string(s.raw) != `[{"a":1},{"a":2}]` || s.contentType == nil || *s.contentType != "application/json" || s.reason != nil || !s.hasAlert {
		t.Fatalf("first row = %+v, want the body, content type and alert", s)
	}
	if s := read("it-alert-3", 1); s.raw != nil || s.contentType != nil {
		t.Fatalf("second row = %+v, want no body", s)
	}
	if s := read("it-alert-4", 0); string(s.raw) != "bad\x00body" || s.source != nil || s.reason == nil || *s.reason != "MissingSource" || s.hasAlert {
		t.Fatalf("error row = %+v, want the raw body, no source, the reason and no alert", s)
	}
}

func TestAlertRepositoryIntegration_RefusesWhatCanNeverBeStored(t *testing.T) {
	pool := newAlertIntegrationPool(t)
	repo := NewAlertRepository(NewScoped(pool), true)
	ctx := WithSystemIdentity(context.Background())
	ids, err := repo.ReserveAlertIDs(ctx, 2)
	if err != nil {
		t.Fatalf("ReserveAlertIDs: %v", err)
	}
	nul := integrationAlert(ids[0], "it-alert-5", 0)
	nul.Alert = json.RawMessage(`{"description":"bad \u0000 byte"}`)
	// An error row that also carries an alert breaks the row-kind check.
	mixed := integrationAlert(ids[1], "it-alert-6", 0)
	mixed.ErrorReason, mixed.Error = "TransformFailed", "boom"
	for name, row := range map[string]domain.NewAlert{"NUL in jsonb": nul, "error row with an alert": mixed} {
		_, err := repo.InsertAlerts(ctx, []domain.NewAlert{row})
		var sqlErr interface{ SQLState() string }
		if !errors.As(err, &sqlErr) || (sqlErr.SQLState()[:2] != "22" && sqlErr.SQLState()[:2] != "23") {
			t.Fatalf("%s: err = %v, want a class 22 or 23 error the consumer treats as permanent", name, err)
		}
	}
}
