//go:build integration

// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package store

import (
	"context"
	"fmt"
	"math/rand"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"

	schema "alert-core-service"
	"alert-core-service/internal/model"
	"alert-core-service/internal/pglock"
	"alert-core-service/internal/postgres"
)

// testDB creates a throwaway database from the PG* environment, applies schema.sql twice to prove it is idempotent, and drops it after the test.
func testDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	cfg, err := postgres.ConfigFromEnv()
	if err != nil {
		t.Skipf("PG* not set: %v", err)
	}
	admin, err := postgres.Connect(cfg, 10*time.Second, 10*time.Second, false)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(admin.Close)
	name := fmt.Sprintf("core_store_test_%d", rand.Int63())
	if _, err := admin.Exec(context.Background(), "CREATE DATABASE "+name); err != nil {
		t.Fatalf("create database: %v", err)
	}
	t.Cleanup(func() { _, _ = admin.Exec(context.Background(), "DROP DATABASE "+name+" WITH (FORCE)") })
	cfg.Database = name
	pool, err := postgres.Connect(cfg, 10*time.Second, 10*time.Second, true)
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}
	t.Cleanup(pool.Close)
	for range 2 {
		if err := postgres.Migrate(context.Background(), pool, schema.SQL); err != nil {
			t.Fatalf("migrate: %v", err)
		}
	}
	return pool
}

// testLocker wraps pool for advisory locks and closes the bridge after the test.
func testLocker(t *testing.T, pool *pgxpool.Pool) *pglock.Locker {
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { _ = db.Close() })
	return pglock.New(db)
}

func insertAlert(t *testing.T, pool *pgxpool.Pool, id, fp string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `INSERT INTO alerts (id, source, alert, fingerprint) VALUES ($1, 'test', '{}', $2)`, id, fp); err != nil {
		t.Fatal(err)
	}
}

func ids(c []ClaimedAlert) []string {
	out := make([]string, len(c))
	for i, a := range c {
		out[i] = a.ID
	}
	slices.Sort(out)
	return out
}

func TestAlertRepo_ClaimPullsSiblingsAndNeverDoubleClaims(t *testing.T) {
	pool := testDB(t)
	repo := NewAlertRepo(pool)
	ctx := context.Background()
	for i, fp := range []string{"a", "b", "c", "a", "d", "a"} {
		insertAlert(t, pool, fmt.Sprintf("ALT%d", i+1), fp)
	}

	first, err := repo.Claim(ctx, "r1", 2, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(first); !slices.Equal(got, []string{"ALT1", "ALT2", "ALT4", "ALT6"}) {
		t.Fatalf("first claim = %v, want seeds ALT1, ALT2 plus fingerprint a's siblings ALT4, ALT6", got)
	}
	if first[0].ReceivedAt.IsZero() {
		t.Fatal("claim must return created_at")
	}
	second, err := repo.Claim(ctx, "r2", 10, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(second); !slices.Equal(got, []string{"ALT3", "ALT5"}) {
		t.Fatalf("second claim = %v, want only the unclaimed rows", got)
	}

	if err := repo.MarkProcessed(ctx, []string{"ALT1", "ALT2"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.Release(ctx, []string{"ALT4", "ALT1"}); err != nil {
		t.Fatal(err)
	}
	third, err := repo.Claim(ctx, "r3", 10, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(third); !slices.Equal(got, []string{"ALT4"}) {
		t.Fatalf("third claim = %v, want only the released unprocessed row", got)
	}

	if _, err := pool.Exec(ctx, `UPDATE alerts SET processed_at = now() - interval '2 days' WHERE id = 'ALT1'`); err != nil {
		t.Fatal(err)
	}
	if n, err := repo.Purge(ctx, time.Now().Add(-24*time.Hour), 100); err != nil || n != 1 {
		t.Fatalf("purge = %d, %v, want 1 old processed row deleted", n, err)
	}

	if _, err := pool.Exec(ctx, `INSERT INTO raw_alerts (received_at, payload) VALUES (now() - interval '2 days', '{"a":1}'), (now(), '{"b":2}')`); err != nil {
		t.Fatal(err)
	}
	if n, err := repo.PurgeRaw(ctx, time.Now().Add(-24*time.Hour), 100); err != nil || n != 1 {
		t.Fatalf("purge raw = %d, %v, want only the old raw body deleted", n, err)
	}
}

func TestIncidentRepo_FoldAndDelivery(t *testing.T) {
	pool := testDB(t)
	repo := NewIncidentRepo(pool, testLocker(t, pool))
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	created := func(cur *model.Incident, recorded map[string]bool) FoldPlan {
		if cur != nil || len(recorded) != 0 {
			t.Fatalf("first fold saw current = %v recorded = %v, want nothing", cur, recorded)
		}
		return FoldPlan{New: []NewIncident{{
			Incident: model.Incident{Fingerprint: "fp1", IncidentNumber: "PENDING-fp1", Severity: 1, Impact: "HIGH", Urgency: "HIGH",
				Service: "svc", AlertCount: 1, FirstSeen: now, LastSeen: now},
			Notes: []model.Note{{AlertID: "ALT1", Kind: model.NoteCreated, Text: "created"}},
		}}}
	}
	if _, err := repo.Fold(ctx, "fp1", []string{"ALT1"}, created); err != nil {
		t.Fatalf("fold create: %v", err)
	}

	var current model.Incident
	dup := func(cur *model.Incident, recorded map[string]bool) FoldPlan {
		if cur == nil || cur.Fingerprint != "fp1" || !recorded["ALT1"] || recorded["ALT2"] {
			t.Fatalf("second fold saw current = %v recorded = %v", cur, recorded)
		}
		current = *cur
		return FoldPlan{Current: &CurrentFold{ID: cur.ID, Added: 1, LastSeen: now.Add(time.Minute), Category: "security",
			Notes: []model.Note{{AlertID: "ALT2", Kind: model.NoteDuplicate, Text: "dup", ChatPending: true}}}}
	}
	if _, err := repo.Fold(ctx, "fp1", []string{"ALT1", "ALT2"}, dup); err != nil {
		t.Fatalf("fold duplicate: %v", err)
	}

	due, err := repo.ListDue(ctx, 10)
	if err != nil || !slices.Equal(due, []int64{current.ID}) {
		t.Fatalf("due = %v, %v, want the incident", due, err)
	}
	inc, found, err := repo.Get(ctx, current.ID)
	if err != nil || !found || inc.AlertCount != 2 || inc.Category != "security" || !inc.LastSeen.Equal(now.Add(time.Minute)) || inc.FoldVersion != 1 {
		t.Fatalf("incident = %+v, %v, %v", inc, found, err)
	}
	notes, err := repo.PendingNotes(ctx, inc.ID, 10)
	if err != nil || len(notes) != 2 || notes[0].Kind != model.NoteCreated || notes[0].ChatPending || !notes[1].ChatPending || !notes[1].CSMPending {
		t.Fatalf("notes = %+v, %v", notes, err)
	}

	unlock, ok, err := repo.TryLock(ctx, inc.ID)
	if err != nil || !ok {
		t.Fatalf("try lock = %v, %v", ok, err)
	}
	if _, ok2, _ := repo.TryLock(ctx, inc.ID); ok2 {
		t.Fatal("a second try lock on the same incident must fail")
	}
	steps := []error{
		repo.RecordCSMAttemptStarted(ctx, inc.ID, 1),
		repo.RecordCSMIncident(ctx, inc.ID, "csm-1", "INC1"),
		repo.MarkFallback(ctx, inc.ID),
		repo.SyncStatus(ctx, inc.ID, "closed", now),
		repo.ClearNotes(ctx, inc.ID, []int64{notes[0].ID}, true, false),
		repo.SettleNotes(ctx, inc.ID, false, true),
		repo.RecordCSMAttemptFailure(ctx, inc.ID, false),
	}
	for i, err := range steps {
		if err != nil {
			t.Fatalf("delivery step %d: %v", i, err)
		}
	}
	next := time.Now().Add(time.Hour)
	if err := repo.FinishDelivery(ctx, inc.ID, inc.FoldVersion+5, &next); err != nil {
		t.Fatal(err)
	}
	if due, _ := repo.ListDue(ctx, 10); len(due) != 1 {
		t.Fatalf("due = %v, want a stale fold version to keep the incident due", due)
	}
	if err := repo.FinishDelivery(ctx, inc.ID, inc.FoldVersion, nil); err != nil {
		t.Fatal(err)
	}
	unlock()

	inc, _, _ = repo.Get(ctx, inc.ID)
	notes, _ = repo.PendingNotes(ctx, inc.ID, 10)
	if due, _ := repo.ListDue(ctx, 10); len(due) != 0 || !inc.CSMConfirmed || inc.Status != "closed" || !inc.Fallback || inc.CSMAttempts != 1 || len(notes) != 1 || notes[0].ChatPending {
		t.Fatalf("after delivery: due = %v incident = %+v notes = %+v", due, inc, notes)
	}

	if _, err := pool.Exec(ctx, `UPDATE incidents_processed SET last_seen = now() - interval '40 days'`); err != nil {
		t.Fatal(err)
	}
	if n, err := repo.Purge(ctx, time.Now().Add(-30*24*time.Hour), 100); err != nil || n != 1 {
		t.Fatalf("purge = %d, %v", n, err)
	}
	var orphans int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM incident_notes`).Scan(&orphans); err != nil || orphans != 0 {
		t.Fatalf("notes left after purge = %d, %v", orphans, err)
	}
}

func TestIncidentRepo_MuteLifecycle(t *testing.T) {
	pool := testDB(t)
	repo := NewIncidentRepo(pool, testLocker(t, pool))
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	newInc := func(fp string, at time.Time, alertID string) model.Incident {
		t.Helper()
		_, err := repo.Fold(ctx, fp, []string{alertID}, func(*model.Incident, map[string]bool) FoldPlan {
			return FoldPlan{New: []NewIncident{{
				Incident: model.Incident{Fingerprint: fp, IncidentNumber: "PENDING", Severity: 1, Impact: "HIGH", Urgency: "HIGH",
					Service: "svc", AlertCount: 1, FirstSeen: at, LastSeen: at, UniqueIdentifier: "host-1", SeverityLabel: "Critical"},
				Notes: []model.Note{{AlertID: alertID, Kind: model.NoteCreated, Text: "created"}},
			}}}
		})
		if err != nil {
			t.Fatalf("fold: %v", err)
		}
		var id int64
		if err := pool.QueryRow(ctx, `SELECT id FROM incidents_processed WHERE fingerprint = $1 AND first_seen = $2`, fp, at).Scan(&id); err != nil {
			t.Fatal(err)
		}
		inc, _, err := repo.Get(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		return inc
	}
	addNote := func(inc model.Incident, alertID, kind string) {
		t.Helper()
		if _, err := repo.Fold(ctx, inc.Fingerprint, []string{alertID}, func(cur *model.Incident, _ map[string]bool) FoldPlan {
			return FoldPlan{Current: &CurrentFold{ID: cur.ID, Added: 1, LastSeen: now.Add(time.Second),
				Notes: []model.Note{{AlertID: alertID, Kind: kind, Text: kind, ChatPending: true}}}}
		}); err != nil {
			t.Fatal(err)
		}
	}

	inc := newInc("fpm", now, "M1")
	if inc.UniqueIdentifier != "host-1" || inc.SeverityLabel != "Critical" || inc.Muted || inc.MutedAt != nil || inc.UnmutedAt != nil || !inc.MuteEligible() {
		t.Fatalf("new incident = %+v", inc)
	}
	if err := repo.Mute(ctx, inc.ID, "b1", "AMR0000001"); err != nil {
		t.Fatal(err)
	}
	if err := repo.Mute(ctx, inc.ID, "b2", "AMR0000002"); err != nil {
		t.Fatal(err)
	}
	muted, _, _ := repo.Get(ctx, inc.ID)
	if !muted.Muted || muted.MutedAt == nil || muted.MuteRuleID != "b2" || muted.MuteRuleNumber != "AMR0000002" {
		t.Fatalf("muted = %+v", muted)
	}

	if resolved, superseded, err := repo.MuteOutcome(ctx, muted); err != nil || resolved || superseded {
		t.Fatalf("outcome = %v %v %v, want still firing", resolved, superseded, err)
	}
	addNote(muted, "M2", model.NoteOK)
	if resolved, _, err := repo.MuteOutcome(ctx, muted); err != nil || !resolved {
		t.Fatalf("resolved = %v %v, want an OK last note to count as resolved", resolved, err)
	}
	newInc("fpm", now.Add(time.Hour), "M3")
	if _, superseded, err := repo.MuteOutcome(ctx, muted); err != nil || !superseded {
		t.Fatalf("superseded = %v %v", superseded, err)
	}

	if closed, err := repo.CloseMuted(ctx, muted.ID, muted.FoldVersion); err != nil || closed {
		t.Fatalf("close with a stale fold version = %v %v, want no-op", closed, err)
	}
	cur, _, _ := repo.Get(ctx, muted.ID)
	if closed, err := repo.CloseMuted(ctx, cur.ID, cur.FoldVersion); err != nil || !closed {
		t.Fatalf("close = %v %v", closed, err)
	}
	cur, _, _ = repo.Get(ctx, muted.ID)
	notes, _ := repo.PendingNotes(ctx, cur.ID, 10)
	var due *time.Time
	_ = pool.QueryRow(ctx, `SELECT delivery_due_at FROM incidents_processed WHERE id = $1`, cur.ID).Scan(&due)
	if cur.Muted || cur.Status != "closed" || cur.UnmutedAt == nil || cur.MuteRuleNumber != "AMR0000002" || len(notes) != 0 || due != nil {
		t.Fatalf("closed = %+v notes = %v due = %v", cur, notes, due)
	}

	other := newInc("fpu", now, "U1")
	if err := repo.Mute(ctx, other.ID, "b1", "AMR0000001"); err != nil {
		t.Fatal(err)
	}
	if ok, err := repo.Unmute(ctx, other.ID, other.FoldVersion+1); err != nil || ok {
		t.Fatalf("stale unmute = %v, %v; want false, nil", ok, err)
	}
	if ok, err := repo.Unmute(ctx, other.ID, other.FoldVersion); err != nil || !ok {
		t.Fatalf("unmute = %v, %v", ok, err)
	}
	other, _, _ = repo.Get(ctx, other.ID)
	if other.Muted || other.UnmutedAt == nil || other.MutedAt == nil || other.MuteEligible() {
		t.Fatalf("unmuted = %+v", other)
	}
}

func TestIncidentRepo_ConcurrentFoldsSerializePerFingerprint(t *testing.T) {
	pool := testDB(t)
	repo := NewIncidentRepo(pool, testLocker(t, pool))
	ctx := context.Background()
	now := time.Now().UTC()

	decide := func(alertID string) Decide {
		return func(cur *model.Incident, recorded map[string]bool) FoldPlan {
			note := []model.Note{{AlertID: alertID, Kind: model.NoteDuplicate, Text: alertID}}
			if cur == nil {
				note[0].Kind = model.NoteCreated
				return FoldPlan{New: []NewIncident{{Incident: model.Incident{Fingerprint: "fp", IncidentNumber: "P", Severity: 1,
					Impact: "HIGH", Urgency: "HIGH", AlertCount: 1, FirstSeen: now, LastSeen: now}, Notes: note}}}
			}
			return FoldPlan{Current: &CurrentFold{ID: cur.ID, Added: 1, LastSeen: now, Notes: note}}
		}
	}
	errs := make(chan error, 20)
	for i := range 20 {
		go func() {
			id := fmt.Sprintf("ALT%d", i)
			_, err := repo.Fold(ctx, "fp", []string{id}, decide(id))
			errs <- err
		}()
	}
	for range 20 {
		if err := <-errs; err != nil {
			t.Fatalf("fold: %v", err)
		}
	}
	var incidents, count int
	if err := pool.QueryRow(ctx, `SELECT count(*), sum(alert_count) FROM incidents_processed`).Scan(&incidents, &count); err != nil {
		t.Fatal(err)
	}
	if incidents != 1 || count != 20 {
		t.Fatalf("incidents = %d alert_count = %d, want 1 incident absorbing all 20 concurrent folds", incidents, count)
	}
}
