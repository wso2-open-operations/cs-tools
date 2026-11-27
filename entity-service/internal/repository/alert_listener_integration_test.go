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
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// Runs against a database with migrations 0308 and 100029 applied, and only when ENTITY_TEST_DATABASE_URL is set.

// listenForAlertStored opens its own LISTEN connection and returns a function that reports whether a notification
// arrives within wait.
func listenForAlertStored(t *testing.T, dsn string) func(wait time.Duration) bool {
	t.Helper()
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close(ctx) })
	if _, err := conn.Exec(ctx, "LISTEN "+pgx.Identifier{AlertStoredChannel}.Sanitize()); err != nil {
		t.Fatalf("listen: %v", err)
	}
	return func(wait time.Duration) bool {
		waitCtx, cancel := context.WithTimeout(ctx, wait)
		defer cancel()
		_, err := conn.WaitForNotification(waitCtx)
		return err == nil
	}
}

func TestAlertRepositoryIntegration_NotifiesOnlyWhenANewAlertCommits(t *testing.T) {
	pool := newAlertIntegrationPool(t)
	notified := listenForAlertStored(t, pool.Config().ConnString())
	repo := NewAlertRepository(NewScoped(pool))
	ctx := WithSystemIdentity(context.Background())

	stored := integrationAlert("ALT900000101", "it-alert-notify-1", 0)
	if n, err := repo.InsertAlerts(ctx, []domain.NewAlert{stored}); err != nil || n != 1 {
		t.Fatalf("InsertAlerts = %d, %v; want 1 stored", n, err)
	}
	if !notified(5 * time.Second) {
		t.Fatal("no alert_stored notification after a NEW alert committed")
	}

	if n, err := repo.InsertAlerts(ctx, []domain.NewAlert{stored}); err != nil || n != 0 {
		t.Fatalf("redelivery = %d, %v; want nothing stored", n, err)
	}
	if notified(300 * time.Millisecond) {
		t.Fatal("a redelivered message that stored nothing notified")
	}

	errorRow := integrationAlert("ALT900000102", "it-alert-notify-2", 0)
	errorRow.Alert, errorRow.Fingerprint, errorRow.ErrorReason, errorRow.Error = nil, "", "UnknownSource", "no transform"
	if n, err := repo.InsertAlerts(ctx, []domain.NewAlert{errorRow}); err != nil || n != 1 {
		t.Fatalf("error row = %d, %v; want 1 stored", n, err)
	}
	if notified(300 * time.Millisecond) {
		t.Fatal("an error row, which the processor never takes, notified")
	}
}

func TestAlertRepositoryIntegration_ListenerWakesOnConnectAndOnEachNotification(t *testing.T) {
	pool := newAlertIntegrationPool(t)
	ctx, cancel := context.WithCancel(context.Background())
	wakes := make(chan struct{}, 16)
	done := make(chan struct{})
	go func() {
		ListenAlertStored(ctx, pool, func() { wakes <- struct{}{} })
		close(done)
	}()
	t.Cleanup(func() { cancel(); <-done })

	waitWake := func(what string) {
		t.Helper()
		select {
		case <-wakes:
		case <-time.After(5 * time.Second):
			t.Fatalf("no wake-up %s", what)
		}
	}
	waitWake("on connect")

	repo := NewAlertRepository(NewScoped(pool))
	if _, err := repo.InsertAlerts(WithSystemIdentity(context.Background()), []domain.NewAlert{integrationAlert("ALT900000103", "it-alert-notify-3", 0)}); err != nil {
		t.Fatalf("InsertAlerts: %v", err)
	}
	waitWake("after a NEW alert committed")
}
