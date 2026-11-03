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
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// hoursQuerier answers the statements recomputeProjectHours issues, in order
// of kind, and records every statement.
type hoursQuerier struct {
	state     []any // dest values for the project row; nil = no such project
	lines     [][]any
	projects  []string // for the opportunity listing
	billable  map[string]int
	sql       []string
	writeArgs []any
}

type fakeRow struct {
	vals []any
	err  error
}

func (r fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for i, d := range dest {
		reflect.ValueOf(d).Elem().Set(reflect.ValueOf(r.vals[i]))
	}
	return nil
}

type fakeRows struct {
	pgx.Rows
	rows [][]any
	i    int
}

func (r *fakeRows) Next() bool { r.i++; return r.i <= len(r.rows) }
func (r *fakeRows) Scan(dest ...any) error {
	for i, d := range dest {
		reflect.ValueOf(d).Elem().Set(reflect.ValueOf(r.rows[r.i-1][i]))
	}
	return nil
}
func (r *fakeRows) Close()     {}
func (r *fakeRows) Err() error { return nil }

func (q *hoursQuerier) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	q.sql = append(q.sql, sql)
	if sql == projectHoursWriteQuery {
		q.writeArgs = args
	}
	return pgconn.NewCommandTag("UPDATE 1"), nil
}

func (q *hoursQuerier) Query(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
	q.sql = append(q.sql, sql)
	if sql == projectHoursLinesQuery {
		return &fakeRows{rows: q.lines}, nil
	}
	var rows [][]any
	for _, p := range q.projects {
		rows = append(rows, []any{p})
	}
	return &fakeRows{rows: rows}, nil
}

func (q *hoursQuerier) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	q.sql = append(q.sql, sql)
	if sql == projectHoursStateQuery {
		if q.state == nil {
			return fakeRow{err: pgx.ErrNoRows}
		}
		return fakeRow{vals: q.state}
	}
	if sql == projectHoursBillableQuery {
		// args[0] is the opportunity ids; the fixture keys billable by the
		// first one.
		return fakeRow{vals: []any{q.billable[args[0].([]string)[0]]}}
	}
	return fakeRow{err: errors.New("unexpected query")}
}

func hDate(s string) *time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return &t
}

func stateRow(status string) []any {
	return []any{status, hDate("2026-12-31"), (*time.Time)(nil), *hDate("2026-06-15"), false}
}

// linkedStateRow is a project that has linked opportunity lines, ended or not.
func linkedStateRow(status string) []any {
	r := stateRow(status)
	r[4] = true
	return r
}

func TestRecomputeProjectHours_BlankAndUnknownProjectAreNoOps(t *testing.T) {
	q := &hoursQuerier{}
	if err := recomputeProjectHours(context.Background(), q, ""); err != nil || len(q.sql) != 0 {
		t.Fatalf("blank id: err=%v statements=%d", err, len(q.sql))
	}
	if err := recomputeProjectHours(context.Background(), q, "p1"); err != nil {
		t.Fatal(err)
	}
	for _, s := range q.sql {
		if s == projectHoursWriteQuery {
			t.Error("an unknown project must not be written")
		}
	}
}

func TestRecomputeProjectHours_LocksThenWritesBothPools(t *testing.T) {
	q := &hoursQuerier{
		state: stateRow("IN_PROGRESS"),
		lines: [][]any{
			{"o1", "Development Support - 100 hours", 1.0, hDate("2026-01-01"), hDate("2026-12-31")},
			{"o1", "WSO2 Onboarding Services", 1.0, hDate("2026-02-01"), hDate("2026-12-31")},
		},
		billable: map[string]int{"o1": 230 * 60},
	}
	if err := recomputeProjectHours(context.Background(), q, "p1"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(q.sql[0], "pg_advisory_xact_lock") {
		t.Errorf("first statement is not the lock: %q", q.sql[0])
	}
	// project id, query total/consumed/remaining, onboarding flag/total/consumed/remaining.
	want := []any{"p1", 100 * 60, 30 * 60, 70 * 60, true, 200 * 60, 200 * 60, 0}
	if !reflect.DeepEqual(q.writeArgs, want) {
		t.Errorf("write args = %v, want %v", q.writeArgs, want)
	}
	// Durations only: nothing here may touch an opportunity.
	for _, s := range q.sql {
		if strings.Contains(s, "query_hour_state") {
			t.Errorf("statement touches opportunity state: %q", s)
		}
	}
}

func TestRecomputeProjectHours_NoActiveLineWritesNothing(t *testing.T) {
	q := &hoursQuerier{state: stateRow("NOT_APPLICABLE")}
	if err := recomputeProjectHours(context.Background(), q, "p1"); err != nil {
		t.Fatal(err)
	}
	if q.writeArgs != nil {
		t.Errorf("wrote %v for a project with nothing allocated", q.writeArgs)
	}
}

// A project whose linked lines have all ended is written down to zero, and its
// onboarding columns are left alone.
func TestRecomputeProjectHours_LapsedAllowanceWritesZeros(t *testing.T) {
	q := &hoursQuerier{state: linkedStateRow("NOT_APPLICABLE")}
	if err := recomputeProjectHours(context.Background(), q, "p1"); err != nil {
		t.Fatal(err)
	}
	want := []any{"p1", 0, 0, 0, false, 0, 0, 0}
	if !reflect.DeepEqual(q.writeArgs, want) {
		t.Errorf("write args = %v, want %v", q.writeArgs, want)
	}
}

func TestRecomputeProjectHours_NotApplicableLeavesOnboardingColumnsAlone(t *testing.T) {
	q := &hoursQuerier{
		state: stateRow("NOT_APPLICABLE"),
		lines: [][]any{
			{"o1", "Development Support - 10 hours", 1.0, hDate("2026-01-01"), hDate("2026-12-31")},
			{"o1", "WSO2 Onboarding Services", 1.0, hDate("2026-02-01"), hDate("2026-12-31")},
		},
		billable: map[string]int{"o1": 60},
	}
	if err := recomputeProjectHours(context.Background(), q, "p1"); err != nil {
		t.Fatal(err)
	}
	if q.writeArgs[4] != false {
		t.Errorf("onboarding write flag = %v, want false", q.writeArgs[4])
	}
}

// The opportunity and line item ingest recompute every linked project, and a
// blank id does nothing.
func TestRecomputeOpportunityProjectHours_EveryLinkedProject(t *testing.T) {
	q := &hoursQuerier{projects: []string{"pa", "pb"}}
	if err := recomputeOpportunityProjectHours(context.Background(), q, ""); err != nil || len(q.sql) != 0 {
		t.Fatalf("blank id: err=%v statements=%d", err, len(q.sql))
	}
	if err := recomputeOpportunityProjectHours(context.Background(), q, "o1"); err != nil {
		t.Fatal(err)
	}
	locks := 0
	for _, s := range q.sql {
		if strings.Contains(s, "pg_advisory_xact_lock") {
			locks++
		}
	}
	if locks != 2 {
		t.Errorf("projects recomputed = %d, want 2", locks)
	}
}

func TestRecomputeMovedProjectHours_SkipsTheNewOwner(t *testing.T) {
	q := &hoursQuerier{}
	if err := recomputeMovedProjectHours(context.Background(), q, []string{"old", "new"}, "new"); err != nil {
		t.Fatal(err)
	}
	locks := 0
	for _, s := range q.sql {
		if strings.Contains(s, "pg_advisory_xact_lock") {
			locks++
		}
	}
	if locks != 1 {
		t.Errorf("recomputed %d projects, want only the previous owner", locks)
	}
}

// failingQuerier fails every statement that mentions the marker, and offers a
// savepoint the way a pgx.Tx does.
type failingQuerier struct {
	hoursQuerier
	failOn string
	began  int
	sp     *fakeSavepoint
}

type fakeSavepoint struct {
	pgx.Tx
	q          *failingQuerier
	rolledBack bool
	committed  bool
}

func (s *fakeSavepoint) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return s.q.Exec(ctx, sql, args...)
}
func (s *fakeSavepoint) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return s.q.Query(ctx, sql, args...)
}
func (s *fakeSavepoint) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return s.q.QueryRow(ctx, sql, args...)
}
func (s *fakeSavepoint) Rollback(context.Context) error { s.rolledBack = true; return nil }
func (s *fakeSavepoint) Commit(context.Context) error   { s.committed = true; return nil }

func (q *failingQuerier) Begin(context.Context) (pgx.Tx, error) {
	q.began++
	q.sp = &fakeSavepoint{q: q}
	return q.sp, nil
}

func (q *failingQuerier) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if strings.Contains(sql, q.failOn) {
		return pgconn.CommandTag{}, errors.New("statement failed")
	}
	return q.hoursQuerier.Exec(ctx, sql, args...)
}

// A failing hours recompute must neither propagate nor leave the ingest
// transaction aborted: it runs in a savepoint that is rolled back.
func TestBestEffortProjectHours_FailureRollsBackSavepointAndContinues(t *testing.T) {
	q := &failingQuerier{failOn: "pg_advisory_xact_lock"}
	ran := false
	bestEffortProjectHours(context.Background(), q, "test", func(tx querier) error {
		ran = true
		return recomputeProjectHours(context.Background(), tx, "p1")
	})
	if !ran || q.began != 1 {
		t.Fatalf("ran=%v savepoints=%d", ran, q.began)
	}
	if !q.sp.rolledBack || q.sp.committed {
		t.Errorf("rolledBack=%v committed=%v, want rollback only", q.sp.rolledBack, q.sp.committed)
	}
}

func TestBestEffortProjectHours_SuccessReleasesSavepoint(t *testing.T) {
	q := &failingQuerier{failOn: "never"}
	q.state = stateRow("NOT_APPLICABLE")
	bestEffortProjectHours(context.Background(), q, "test", func(tx querier) error {
		return recomputeProjectHours(context.Background(), tx, "p1")
	})
	if q.sp == nil || !q.sp.committed || q.sp.rolledBack {
		t.Errorf("savepoint %+v, want released", q.sp)
	}
}

// Through a querier that cannot open a savepoint the error is still swallowed.
func TestBestEffortProjectHours_NoSavepointSupportStillSwallowsError(t *testing.T) {
	bestEffortProjectHours(context.Background(), &hoursQuerier{}, "test", func(querier) error { return errors.New("boom") })
}
