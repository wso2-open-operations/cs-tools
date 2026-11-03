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
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
)

// The Send now queue: one request waits at a time, a second press is refused
// while it waits and for the cooldown after, the task claims the oldest and
// reports it, a claim left unreported is claimed again. Needs migrations 0155
// and 0242; skipped unless ENTITY_TEST_DATABASE_URL is set. Uses SME and
// refuses to run where SME requests exist, so it never touches real ones.
func TestRotaReminderRequestIntegration_Queue(t *testing.T) {
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
	repo := NewRotaReminderRequestRepository(pool)
	if existing, err := repo.List(ctx, "SME", 1); err != nil {
		t.Fatal(err)
	} else if len(existing) > 0 {
		t.Skip("SME reminder requests already exist here; not touching them")
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM team_schedule_reminder_request WHERE family = 'SME'`)
	})

	first, err := repo.Create(ctx, "ann@example.com", "SME", "weekend", false, 10*time.Minute)
	if err != nil || first.Status != "QUEUED" || first.RequestedBy != "ann@example.com" {
		t.Fatalf("first: %+v, %v", first, err)
	}
	var conflict *apierror.ConflictError
	if _, err := repo.Create(ctx, "ben@example.com", "SME", "weekend", false, 0); !errors.As(err, &conflict) {
		t.Errorf("while waiting: %v", err)
	}
	if other, err := repo.Create(ctx, "ben@example.com", "SME", "weekday", false, 10*time.Minute); err != nil || other.Kind != "weekday" {
		t.Errorf("another kind: %+v, %v", other, err)
	}
	// A test is its own queue per person: not held up by the waiting real
	// weekend, refused a second time within its gap, and nobody else's.
	mine, err := repo.Create(ctx, "ann@example.com", "SME", "weekend", true, 2*time.Minute)
	if err != nil || !mine.Test {
		t.Fatalf("a test while a real one waits: %+v, %v", mine, err)
	}
	if _, err := repo.Create(ctx, "ann@example.com", "SME", "weekend", true, 2*time.Minute); !errors.As(err, &conflict) {
		t.Errorf("a second test of mine while the first waits: %v", err)
	}
	if theirs, err := repo.Create(ctx, "ben@example.com", "SME", "weekend", true, 2*time.Minute); err != nil || !theirs.Test {
		t.Errorf("somebody else's test: %+v, %v", theirs, err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM team_schedule_reminder_request WHERE family = 'SME' AND test`); err != nil {
		t.Fatal(err)
	}

	claimed, err := repo.Claim(ctx, "csm-scheduled-tasks", "SME", 30*time.Minute)
	if err != nil || claimed == nil || claimed.ID != first.ID || claimed.Status != "SENDING" || claimed.ClaimedOn == nil {
		t.Fatalf("claim: %+v, %v", claimed, err)
	}
	// The weekday one is next; the weekend one, being sent, is not claimed
	// again until it is stale.
	next, err := repo.Claim(ctx, "csm-scheduled-tasks", "SME", 30*time.Minute)
	if err != nil || next == nil || next.Kind != "weekday" {
		t.Fatalf("second claim: %+v, %v", next, err)
	}
	if none, err := repo.Claim(ctx, "csm-scheduled-tasks", "SME", 30*time.Minute); err != nil || none != nil {
		t.Fatalf("nothing waiting: %+v, %v", none, err)
	}
	if again, err := repo.Claim(ctx, "csm-scheduled-tasks", "SME", 0); err != nil || again == nil || again.ID != first.ID {
		t.Fatalf("a stale claim is taken again: %+v, %v", again, err)
	}

	done, err := repo.Report(ctx, "csm-scheduled-tasks", "SME", first.ID, "SENT", "Sent 5 emails.")
	if err != nil || done.Status != "SENT" || done.HandledOn == nil || done.Result != "Sent 5 emails." {
		t.Fatalf("report: %+v, %v", done, err)
	}
	var notFound *apierror.NotFoundError
	if _, err := repo.Report(ctx, "csm-scheduled-tasks", "SME", first.ID, "FAILED", "late"); !errors.As(err, &notFound) {
		t.Errorf("a second report: %v", err)
	}
	var tooMany *apierror.TooManyRequestsError
	if _, err := repo.Create(ctx, "ann@example.com", "SME", "weekend", false, 10*time.Minute); !errors.As(err, &tooMany) {
		t.Errorf("within the cooldown: %v", err)
	}
	if again, err := repo.Create(ctx, "ann@example.com", "SME", "weekend", false, 0); err != nil || again.Status != "QUEUED" {
		t.Errorf("after the cooldown: %+v, %v", again, err)
	}
	listed, err := repo.List(ctx, "SME", 5)
	if err != nil || len(listed) != 3 || listed[0].Status != "QUEUED" {
		t.Errorf("list: %+v, %v", listed, err)
	}

	// A request that failed sent nobody anything, so it does not start the
	// cooldown: the button can be pressed again at once.
	if _, err := pool.Exec(ctx, `DELETE FROM team_schedule_reminder_request WHERE family = 'SME'`); err != nil {
		t.Fatal(err)
	}
	failed, err := repo.Create(ctx, "ann@example.com", "SME", "weekend", false, 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if c, err := repo.Claim(ctx, "csm-scheduled-tasks", "SME", 2*time.Hour); err != nil || c == nil || c.ID != failed.ID {
		t.Fatalf("claim the request that will fail: %+v, %v", c, err)
	}
	if _, err := repo.Report(ctx, "csm-scheduled-tasks", "SME", failed.ID, "FAILED", "mail service down"); err != nil {
		t.Fatal(err)
	}
	if retry, err := repo.Create(ctx, "ann@example.com", "SME", "weekend", false, 10*time.Minute); err != nil || retry.Status != "QUEUED" {
		t.Errorf("a retry after a failed request: %+v, %v", retry, err)
	}
}

// A weekday request sends the Morning and the Evening, so it and a Morning or
// Evening request refuse each other in either order, while they wait and for
// the cooldown after; Morning and Evening do not overlap, and a test never
// holds up a real request. Same database and SME guard as the queue test.
func TestRotaReminderRequestIntegration_OverlappingKinds(t *testing.T) {
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
	repo := NewRotaReminderRequestRepository(pool)
	if existing, err := repo.List(ctx, "SME", 1); err != nil {
		t.Fatal(err)
	} else if len(existing) > 0 {
		t.Skip("SME reminder requests already exist here; not touching them")
	}
	clear := func() {
		t.Helper()
		if _, err := pool.Exec(ctx, `DELETE FROM team_schedule_reminder_request WHERE family = 'SME'`); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM team_schedule_reminder_request WHERE family = 'SME'`)
	})
	var conflict *apierror.ConflictError
	var tooMany *apierror.TooManyRequestsError

	// weekday waiting: morning and evening are refused, a test is not.
	if _, err := repo.Create(ctx, "ann@example.com", "SME", "weekday", false, 10*time.Minute); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"morning", "evening"} {
		if _, err := repo.Create(ctx, "ben@example.com", "SME", kind, false, 10*time.Minute); !errors.As(err, &conflict) {
			t.Errorf("%s while a weekday request waits: %v", kind, err)
		}
	}
	if _, err := repo.Create(ctx, "ann@example.com", "SME", "morning", true, 2*time.Minute); err != nil {
		t.Errorf("a morning test while a real weekday request waits: %v", err)
	}

	// morning waiting: weekday is refused, evening is not.
	clear()
	if _, err := repo.Create(ctx, "ann@example.com", "SME", "morning", false, 10*time.Minute); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Create(ctx, "ben@example.com", "SME", "weekday", false, 10*time.Minute); !errors.As(err, &conflict) {
		t.Errorf("weekday while a morning request waits: %v", err)
	}
	if _, err := repo.Create(ctx, "ben@example.com", "SME", "evening", false, 10*time.Minute); err != nil {
		t.Errorf("evening while a morning request waits: %v", err)
	}

	// weekday sent a moment ago: morning is inside its cooldown.
	clear()
	sent, err := repo.Create(ctx, "ann@example.com", "SME", "weekday", false, 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Claim(ctx, "csm-scheduled-tasks", "SME", 2*time.Hour); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Report(ctx, "csm-scheduled-tasks", "SME", sent.ID, "SENT", "Sent 2 emails."); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Create(ctx, "ben@example.com", "SME", "morning", false, 10*time.Minute); !errors.As(err, &tooMany) {
		t.Errorf("morning within a sent weekday request's cooldown: %v", err)
	}
}
