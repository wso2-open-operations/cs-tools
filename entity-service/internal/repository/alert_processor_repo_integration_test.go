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
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// Runs against a database with migrations 0292, 0306, 0309, 0310 and 100029 applied, and only when
// ENTITY_TEST_DATABASE_URL is set. Every row it writes is removed again.

const (
	itProcessorIncidentOpen   = "11111111-0000-4000-8000-0000000000a1"
	itProcessorIncidentOld    = "11111111-0000-4000-8000-0000000000a2"
	itProcessorIncidentClosed = "11111111-0000-4000-8000-0000000000a3"
	itProcessorService        = "11111111-0000-4000-8000-0000000000b1"
)

func newAlertProcessorIntegrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool := newAlertIntegrationPool(t)
	ctx := WithSystemIdentity(context.Background())
	clean := func() {
		db := NewScoped(pool)
		for _, q := range []string{
			`DELETE FROM alert WHERE message_id LIKE 'it-alert-processor-%'`,
			`DELETE FROM incident WHERE id IN ('` + itProcessorIncidentOpen + `', '` + itProcessorIncidentOld + `', '` + itProcessorIncidentClosed + `')`,
			`DELETE FROM work_item WHERE id IN ('` + itProcessorIncidentOpen + `', '` + itProcessorIncidentOld + `', '` + itProcessorIncidentClosed + `')`,
			`DELETE FROM service WHERE id = '` + itProcessorService + `'`,
		} {
			if _, err := db.Exec(ctx, q); err != nil {
				t.Fatalf("clean: %v", err)
			}
		}
	}
	clean()
	t.Cleanup(clean)
	return pool
}

func insertProcessorAlerts(t *testing.T, pool *pgxpool.Pool, alerts ...domain.NewAlert) {
	t.Helper()
	if _, err := NewAlertRepository(NewScoped(pool)).InsertAlerts(WithSystemIdentity(context.Background()), alerts); err != nil {
		t.Fatalf("InsertAlerts: %v", err)
	}
}

func processorAlert(id, messageID string, tier domain.AlertTier) domain.NewAlert {
	return domain.NewAlert{
		ID: id, Source: "azure", Alert: json.RawMessage(`{"service":"svc-a","metric_name":"cpu"}`), Fingerprint: "fp-it",
		Tier: tier, MessageID: messageID, ReceivedOn: time.Date(2026, 11, 25, 8, 0, 0, 0, time.UTC),
	}
}

func insertIncident(t *testing.T, pool *pgxpool.Pool, id, state, correlationID string, createdOn time.Time) {
	t.Helper()
	db := NewScoped(pool)
	ctx := WithSystemIdentity(context.Background())
	if _, err := db.Exec(ctx, `INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, subject, type)
		VALUES ($1::uuid, $2, $2, 'it', 'it', $3, 'it', 'INCIDENT')`, id, createdOn, "INC-IT-"+id[len(id)-4:]); err != nil {
		t.Fatalf("insert work_item: %v", err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO incident (id, state, correlation_id) VALUES ($1, $2::incident_state_enum, $3)`, id, state, correlationID); err != nil {
		t.Fatalf("insert incident: %v", err)
	}
}

func TestAlertProcessorRepositoryIntegration_ClaimsPriorityFirstAndOnlyOnce(t *testing.T) {
	pool := newAlertProcessorIntegrationPool(t)
	repo := NewAlertProcessorRepository(NewScoped(pool))
	ctx := WithSystemIdentity(context.Background())

	failed := processorAlert("ALT900000001", "it-alert-processor-1", domain.AlertTierHigh)
	failed.Alert, failed.Fingerprint, failed.ErrorReason, failed.Error, failed.RawBody = nil, "", "TransformFailed", "bad body", []byte("x\x00y")
	insertProcessorAlerts(t, pool,
		processorAlert("ALT900000002", "it-alert-processor-2", domain.AlertTierDefault),
		processorAlert("ALT900000003", "it-alert-processor-3", domain.AlertTierHigh),
		processorAlert("ALT900000004", "it-alert-processor-4", domain.AlertTierHigh),
		failed,
	)

	first, err := repo.ClaimAlerts(ctx, "worker-a", time.Minute, 2)
	if err != nil {
		t.Fatalf("ClaimAlerts: %v", err)
	}
	if len(first) != 2 || first[0].ID != "ALT900000003" || first[1].ID != "ALT900000004" || first[0].Tier != domain.AlertTierHigh {
		t.Fatalf("first claim = %+v, want the two priority alerts, oldest first", first)
	}
	second, err := repo.ClaimAlerts(ctx, "worker-b", time.Minute, 10)
	if err != nil {
		t.Fatalf("ClaimAlerts: %v", err)
	}
	if len(second) != 1 || second[0].ID != "ALT900000002" || string(second[0].Alert) == "" || second[0].Fingerprint != "fp-it" {
		t.Fatalf("second claim = %+v, want only the standard alert: claimed and error rows are never handed out", second)
	}

	var raw []byte
	var status string
	if err := NewScoped(pool).QueryRow(ctx, `SELECT raw_body, status FROM alert WHERE id = 'ALT900000001'`).Scan(&raw, &status); err != nil || string(raw) != "x\x00y" || status != domain.AlertStatusError {
		t.Fatalf("raw body = %q, status %q (%v), want the NUL kept and the row ERROR", raw, status, err)
	}
}

func TestAlertProcessorRepositoryIntegration_RecordsOutcomesForTheClaimOwnerOnly(t *testing.T) {
	pool := newAlertProcessorIntegrationPool(t)
	repo := NewAlertProcessorRepository(NewScoped(pool))
	ctx := WithSystemIdentity(context.Background())
	insertIncident(t, pool, itProcessorIncidentOpen, "NEW", "[fp:fpitfpitfpit:1]", time.Now())
	insertProcessorAlerts(t, pool, processorAlert("ALT900000010", "it-alert-processor-10", domain.AlertTierHigh))
	if _, err := repo.ClaimAlerts(ctx, "worker-a", time.Minute, 1); err != nil {
		t.Fatalf("ClaimAlerts: %v", err)
	}

	outcome := domain.AlertProcessorOutcome{Status: domain.AlertStatusProcessed, IncidentID: itProcessorIncidentOpen}
	err := repo.WithFingerprintLock(ctx, "fp-it", func(tx AlertFingerprintTx) error {
		return tx.RecordOutcome(ctx, "ALT900000010", "worker-b", outcome)
	})
	if !errors.Is(err, ErrAlertClaimLost) {
		t.Fatalf("outcome from a worker without the claim = %v, want ErrAlertClaimLost", err)
	}
	if err := repo.WithFingerprintLock(ctx, "fp-it", func(tx AlertFingerprintTx) error {
		return tx.RecordOutcome(ctx, "ALT900000010", "worker-a", outcome)
	}); err != nil {
		t.Fatalf("RecordOutcome: %v", err)
	}

	var status, incident string
	var claimedBy *string
	if err := NewScoped(pool).QueryRow(ctx, `SELECT status, incident_id::text, claimed_by FROM alert WHERE id = 'ALT900000010' AND processed_on IS NOT NULL`).
		Scan(&status, &incident, &claimedBy); err != nil {
		t.Fatalf("read: %v", err)
	}
	if status != domain.AlertStatusProcessed || incident != itProcessorIncidentOpen || claimedBy != nil {
		t.Fatalf("row = %s %s %v, want PROCESSED, linked, claim released", status, incident, claimedBy)
	}
}

func TestAlertProcessorRepositoryIntegration_FailedAttemptsWaitAndThenFail(t *testing.T) {
	pool := newAlertProcessorIntegrationPool(t)
	repo := NewAlertProcessorRepository(NewScoped(pool))
	ctx := WithSystemIdentity(context.Background())
	insertProcessorAlerts(t, pool, processorAlert("ALT900000020", "it-alert-processor-20", domain.AlertTierHigh))

	if _, err := repo.ClaimAlerts(ctx, "worker-a", time.Minute, 1); err != nil {
		t.Fatalf("ClaimAlerts: %v", err)
	}
	if err := repo.RecordFailure(ctx, "ALT900000020", "worker-a", "connection reset", time.Now().Add(time.Hour), false); err != nil {
		t.Fatalf("RecordFailure: %v", err)
	}
	if again, err := repo.ClaimAlerts(ctx, "worker-a", time.Minute, 1); err != nil || len(again) != 0 {
		t.Fatalf("claim before the retry is due = %+v (%v), want nothing", again, err)
	}
	if _, err := NewScoped(pool).Exec(ctx, `UPDATE alert SET next_attempt_on = now() - interval '1 second' WHERE id = 'ALT900000020'`); err != nil {
		t.Fatalf("make the retry due: %v", err)
	}
	due, err := repo.ClaimAlerts(ctx, "worker-a", time.Minute, 1)
	if err != nil || len(due) != 1 || due[0].Attempts != 1 {
		t.Fatalf("claim once due = %+v (%v), want the alert with one attempt counted", due, err)
	}
	if err := repo.RecordFailure(ctx, "ALT900000020", "worker-a", "invalid serviceId", time.Now(), true); err != nil {
		t.Fatalf("RecordFailure: %v", err)
	}
	var status, lastError string
	var attempts int
	if err := NewScoped(pool).QueryRow(ctx, `SELECT status, attempts, last_error FROM alert WHERE id = 'ALT900000020' AND processed_on IS NOT NULL`).
		Scan(&status, &attempts, &lastError); err != nil {
		t.Fatalf("read: %v", err)
	}
	if status != domain.AlertStatusFailed || attempts != 2 || lastError != "invalid serviceId" {
		t.Fatalf("row = %s %d %q, want FAILED after 2 attempts with the last error", status, attempts, lastError)
	}
}

func TestAlertProcessorRepositoryIntegration_FindsOnlyARecentOpenIncidentOfTheFingerprint(t *testing.T) {
	pool := newAlertProcessorIntegrationPool(t)
	repo := NewAlertProcessorRepository(NewScoped(pool))
	ctx := WithSystemIdentity(context.Background())
	now := time.Now()
	insertIncident(t, pool, itProcessorIncidentClosed, "CLOSED", "[fp:aaaaaaaaaaaa:3]", now)
	insertIncident(t, pool, itProcessorIncidentOld, "IN_PROGRESS", "[fp:aaaaaaaaaaaa:1]", now.Add(-48*time.Hour))
	insertIncident(t, pool, itProcessorIncidentOpen, "NEW", "[fp:bbbbbbbbbbbb:2]", now)

	find := func(prefix string, maxAge time.Duration) (id string, found bool) {
		t.Helper()
		if err := repo.WithFingerprintLock(ctx, prefix, func(tx AlertFingerprintTx) (err error) {
			id, found, err = tx.OpenIncidentByTagPrefix(ctx, prefix, maxAge)
			return err
		}); err != nil {
			t.Fatalf("OpenIncidentByTagPrefix: %v", err)
		}
		return id, found
	}
	if id, ok := find("[fp:aaaaaaaaaaaa:", 0); !ok || id != itProcessorIncidentOld {
		t.Fatalf("no age limit = %q %v, want the old open incident, never the closed one", id, ok)
	}
	if id, ok := find("[fp:aaaaaaaaaaaa:", 24*time.Hour); ok {
		t.Fatalf("24h limit found %q, want nothing: the only open incident is 48h old", id)
	}
	if id, ok := find("[fp:bbbbbbbbbbbb:", 24*time.Hour); !ok || id != itProcessorIncidentOpen {
		t.Fatalf("recent = %q %v, want the new open incident", id, ok)
	}
	if id, ok := find("[fp:bbbbbbbbbbb_:", 0); ok {
		t.Fatalf("a LIKE wildcard in the prefix matched %q, want it taken literally", id)
	}

	var byTag string
	var found bool
	if err := repo.WithFingerprintLock(ctx, "x", func(tx AlertFingerprintTx) (err error) {
		byTag, found, err = tx.IncidentByCorrelationID(ctx, "[fp:aaaaaaaaaaaa:3]")
		return err
	}); err != nil || !found || byTag != itProcessorIncidentClosed {
		t.Fatalf("exact tag = %q %v (%v), want the incident in any state", byTag, found, err)
	}
}

func TestAlertProcessorRepositoryIntegration_ServiceByNameIgnoresCase(t *testing.T) {
	pool := newAlertProcessorIntegrationPool(t)
	repo := NewAlertProcessorRepository(NewScoped(pool))
	ctx := WithSystemIdentity(context.Background())
	if _, err := NewScoped(pool).Exec(ctx, `INSERT INTO service (id, created_on, updated_on, created_by, updated_by, name, number)
		VALUES ($1, now(), now(), 'it', 'it', 'IT Alert Checkout Service', 'SVC-IT-1')`, itProcessorService); err != nil {
		t.Fatalf("insert service: %v", err)
	}
	if id, found, err := repo.ServiceIDByName(ctx, "it alert checkout SERVICE"); err != nil || !found || id != itProcessorService {
		t.Fatalf("lookup = %q %v (%v), want the service matched ignoring case", id, found, err)
	}
	if _, found, err := repo.ServiceIDByName(ctx, "IT Alert Checkout"); err != nil || found {
		t.Fatalf("partial name found=%v (%v), want only an exact name to match", found, err)
	}
}
