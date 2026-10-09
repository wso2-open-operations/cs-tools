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

package poll

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/stdlib"

	schema "alert-core-service"
	"alert-core-service/internal/engine"
	"alert-core-service/internal/model"
	"alert-core-service/internal/pglock"
	"alert-core-service/internal/postgres"
	"alert-core-service/internal/store"
)

// chatOnly is a notifier with CSM off whose Chat calls always succeed instantly, so the test measures folding alone.
type chatOnly struct{}

func (chatOnly) CSMEnabled() bool { return false }
func (chatOnly) NotifyCSM(context.Context, model.Incident, string) (string, string, bool, bool) {
	return "", "", false, false
}
func (chatOnly) NotifyChat(context.Context, model.Incident) bool                   { return true }
func (chatOnly) NotifyChatAnnotation(context.Context, model.Incident, string) bool { return true }
func (chatOnly) PushWorkNote(context.Context, string, string) error                { return nil }
func (chatOnly) IncidentState(context.Context, string, string) (bool, bool, error) {
	return true, true, nil
}

// TestLoad_FoldsBurstExactly loads LOAD_ALERTS alerts (default 5000) into a throwaway database, runs one poller until all are processed, and checks every incident count.
func TestLoad_FoldsBurstExactly(t *testing.T) {
	total := 5000
	if v, err := strconv.Atoi(os.Getenv("LOAD_ALERTS")); err == nil {
		total = v
	}
	cfg, err := postgres.ConfigFromEnv()
	if err != nil {
		t.Skipf("PG* not set: %v", err)
	}
	ctx := context.Background()
	admin, err := postgres.Connect(cfg, 10*time.Second, 10*time.Second, false)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	name := fmt.Sprintf("core_load_test_%d", rand.Int63())
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = admin.Exec(context.Background(), "DROP DATABASE "+name+" WITH (FORCE)") }()
	cfg.Database = name
	cfg.PoolMaxConns = 64
	pool, err := postgres.Connect(cfg, 10*time.Second, 10*time.Second, true)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := postgres.Migrate(ctx, pool, schema.SQL); err != nil {
		t.Fatal(err)
	}

	rttStart := time.Now()
	for range 5 {
		if err := pool.Ping(ctx); err != nil {
			t.Fatal(err)
		}
	}
	rtt := time.Since(rttStart) / 5

	// 70% unique fingerprints, 30% spread over a few hot ones, plus one fingerprint split across the dedup window.
	now := time.Now().UTC()
	hot := map[string]bool{}
	var unique int
	ids, sources, bodies, fps, times := []string{}, []string{}, []string{}, []string{}, []time.Time{}
	add := func(uid string, at time.Time) {
		a := model.Alert{Service: "svc", MetricName: "cpu", Severity: "critical", Source: "load", UniqueIdentifier: uid}
		body, _ := json.Marshal(a)
		ids = append(ids, fmt.Sprintf("ALT%09d", len(ids)+1))
		sources, bodies, times = append(sources, "load"), append(bodies, string(body)), append(times, at)
		fps = append(fps, model.Fingerprint(a.Source, a.Service, a.MetricName, a.Environment, a.UniqueIdentifier))
	}
	for i := range total - 2 {
		if i%10 < 7 {
			unique++
			add(fmt.Sprintf("u%d", i), now)
		} else {
			uid := fmt.Sprintf("hot%d", i%50)
			hot[uid] = true
			add(uid, now.Add(time.Duration(i)*time.Millisecond))
		}
	}
	add("span", now.Add(-10*time.Minute))
	add("span", now)
	if _, err := pool.Exec(ctx, `INSERT INTO alerts (id, source, alert, fingerprint, created_at)
		SELECT * FROM unnest($1::text[], $2::text[], $3::jsonb[], $4::text[], $5::timestamptz[])`, ids, sources, bodies, fps, times); err != nil {
		t.Fatal(err)
	}

	lockDB := stdlib.OpenDBFromPool(pool)
	defer lockDB.Close()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	incidents := store.NewIncidentRepo(pool, pglock.New(lockDB))
	eng := engine.New(logger, incidents, chatOnly{}, engine.Config{DedupWindow: 5 * time.Minute, MaxCSMAttempts: 3,
		StateCheckInterval: time.Minute, CSMRetry: engine.CSMRetryConfig{BaseDelay: time.Second, Multiplier: 2, MaxDelay: time.Minute},
		ChatThreadingEnabled: true, DeliveryConcurrency: 8})
	p := New(logger, store.NewAlertRepo(pool), eng, Settings{Interval: time.Second, Concurrency: 64, MaxBatch: 500,
		ClaimTTL: 2 * time.Minute, DeliverySweepInterval: time.Second})

	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	start := time.Now()
	go func() { defer close(done); p.Run(runCtx) }()
	for {
		var left int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM alerts WHERE processed_at IS NULL`).Scan(&left); err != nil {
			t.Fatal(err)
		}
		if left == 0 {
			break
		}
		if time.Since(start) > 10*time.Minute {
			t.Fatalf("%d alerts still unprocessed after 10 minutes", left)
		}
		time.Sleep(200 * time.Millisecond)
	}
	elapsed := time.Since(start)
	cancel()
	<-done

	var incidentCount, alertSum, noteCount int
	if err := pool.QueryRow(ctx, `SELECT count(*), coalesce(sum(alert_count), 0) FROM incidents_processed`).Scan(&incidentCount, &alertSum); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM incident_notes`).Scan(&noteCount); err != nil {
		t.Fatal(err)
	}
	want := unique + len(hot) + 2
	if incidentCount != want || alertSum != total || noteCount != total {
		t.Fatalf("incidents = %d (want %d), alert_count sum = %d and notes = %d (want %d)", incidentCount, want, alertSum, noteCount, total)
	}
	t.Logf("%d alerts folded into %d incidents in %v: %.0f alerts/sec (round trip to Postgres %v)",
		total, incidentCount, elapsed.Round(time.Millisecond), float64(total)/elapsed.Seconds(), rtt.Round(time.Millisecond))
}
