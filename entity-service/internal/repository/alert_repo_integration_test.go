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
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// Runs against a database with migrations 0292 and 100029 applied, and only when ENTITY_TEST_DATABASE_URL is set.
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
		Fingerprint:  "fp-1",
		Tier:         domain.AlertTierHigh,
		MessageID:    messageID,
		MessageIndex: index,
		ReceivedOn:   time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC),
	}
}

func TestAlertRepositoryIntegration_RedeliveredMessageStoresNothingNew(t *testing.T) {
	pool := newAlertIntegrationPool(t)
	repo := NewAlertRepository(NewScoped(pool))
	ctx := WithSystemIdentity(context.Background())

	ids, err := repo.ReserveAlertIDs(ctx, 4)
	if err != nil {
		t.Fatalf("ReserveAlertIDs: %v", err)
	}
	if len(ids) != 4 || len(ids[0]) != len("ALT000000001") || ids[0] >= ids[3] {
		t.Fatalf("ids = %v, want four ascending ALT ids", ids)
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
	var processed *time.Time
	if err := NewScoped(pool).QueryRow(ctx, `SELECT count(*), max(processed_on) FROM alert WHERE message_id = 'it-alert-1'`).Scan(&rows, &processed); err != nil {
		t.Fatalf("count: %v", err)
	}
	if rows != 2 || processed != nil {
		t.Fatalf("rows = %d processed = %v, want 2 unprocessed rows", rows, processed)
	}
}

func TestAlertRepositoryIntegration_NotVisibleWithoutInternalIdentity(t *testing.T) {
	pool := newAlertIntegrationPool(t)
	repo := NewAlertRepository(NewScoped(pool))
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
