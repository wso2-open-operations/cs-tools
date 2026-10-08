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

package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgconn"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const incidentReportTestID = "22222222-3333-4444-5555-666666666666"

type fakeIncidentReportTx struct {
	src       repository.IncidentReportSource
	srcErr    error
	createErr error
	writeErr  error

	created []repository.NewIncidentReportTask
	reports map[string]string

	tasks    []repository.NewIncidentTask
	problems []repository.NewIncidentProblem
	links    map[string]string // incident id -> problem id

	alertSrc    repository.SpecialOpsAlertSource
	alertSrcErr error
}

func (f *fakeIncidentReportTx) SpecialOpsAlertSource(context.Context, string, string, string) (repository.SpecialOpsAlertSource, error) {
	return f.alertSrc, f.alertSrcErr
}

func (f *fakeIncidentReportTx) CreateIncidentTask(_ context.Context, t repository.NewIncidentTask) (string, string, error) {
	if f.createErr != nil {
		return "", "", f.createErr
	}
	f.tasks = append(f.tasks, t)
	return "alert-task-id", "CS-PORTAL-000002", nil
}

func (f *fakeIncidentReportTx) CreateProblem(_ context.Context, p repository.NewIncidentProblem) (string, string, error) {
	if f.createErr != nil {
		return "", "", f.createErr
	}
	f.problems = append(f.problems, p)
	return "problem-id", "CS-PORTAL-000003", nil
}

func (f *fakeIncidentReportTx) LinkProblem(_ context.Context, incidentID, problemID, _ string) error {
	if f.writeErr != nil {
		return f.writeErr
	}
	if f.links == nil {
		f.links = map[string]string{}
	}
	f.links[incidentID] = problemID
	return nil
}

func (f *fakeIncidentReportTx) IncidentSource(context.Context, string) (repository.IncidentReportSource, error) {
	return f.src, f.srcErr
}

func (f *fakeIncidentReportTx) CreateReportTask(_ context.Context, t repository.NewIncidentReportTask) (string, string, error) {
	if f.createErr != nil {
		return "", "", f.createErr
	}
	f.created = append(f.created, t)
	return "task-id", "CS-PORTAL-000001", nil
}

func (f *fakeIncidentReportTx) SetIncidentReport(_ context.Context, id, report, _ string) error {
	if f.writeErr != nil {
		return f.writeErr
	}
	if f.reports == nil {
		f.reports = map[string]string{}
	}
	f.reports[id] = report
	return nil
}

func sampleIncidentSource() repository.IncidentReportSource {
	return repository.IncidentReportSource{
		IncidentID:        incidentReportTestID,
		Number:            "INC0012345",
		Priority:          strPtr("HIGH"),
		CreatedOn:         time.Date(2026, 10, 2, 8, 47, 41, 0, time.UTC),
		ServiceID:         strPtr("svc"),
		AssignmentGroupID: strPtr("grp"),
		AssignedToID:      strPtr("usr"),
	}
}

func incidentStateChange(from, to string, at time.Time) repository.IncidentReportChange {
	return repository.IncidentReportChange{
		OutboxID:   1,
		IncidentID: incidentReportTestID,
		Changes:    map[string]map[string]any{"state": {"from": from, "to": to}},
		OccurredOn: at,
	}
}

func newTestIncidentReportService(time.Time) *incidentReportService {
	return &incidentReportService{}
}

// The trigger condition of both flows: only "state changes to" In Progress or
// Resolved acts, and each acts in its own way.
func TestIncidentReport_StateTable(t *testing.T) {
	now := time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC)
	cases := []struct {
		name       string
		changes    map[string]map[string]any
		wantTask   bool
		wantReport bool
	}{
		{"new to in progress creates task", map[string]map[string]any{"state": {"from": "NEW", "to": "IN_PROGRESS"}}, true, false},
		{"on hold back to in progress creates task", map[string]map[string]any{"state": {"from": "ON_HOLD", "to": "IN_PROGRESS"}}, true, false},
		{"in progress to resolved writes report", map[string]map[string]any{"state": {"from": "IN_PROGRESS", "to": "RESOLVED"}}, false, true},
		{"to on hold does nothing", map[string]map[string]any{"state": {"from": "IN_PROGRESS", "to": "ON_HOLD"}}, false, false},
		{"to closed does nothing", map[string]map[string]any{"state": {"from": "RESOLVED", "to": "CLOSED"}}, false, false},
		{"no state change does nothing", map[string]map[string]any{"priority": {"from": "LOW", "to": "HIGH"}}, false, false},
		// The generator's own write changes incident_report only; it must not
		// be mistaken for a trigger.
		{"report write is not a trigger", map[string]map[string]any{"incident_report": {"from": nil, "to": "<p>…</p>"}}, false, false},
		{"state set to null does nothing", map[string]map[string]any{"state": {"from": "NEW", "to": nil}}, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tx := &fakeIncidentReportTx{src: sampleIncidentSource()}
			svc := newTestIncidentReportService(now)
			c := repository.IncidentReportChange{OutboxID: 1, IncidentID: incidentReportTestID, Changes: tc.changes, OccurredOn: now}
			if err := svc.HandleChange(context.Background(), tx, c); err != nil {
				t.Fatalf("HandleChange: %v", err)
			}
			if got := len(tx.created) == 1; got != tc.wantTask {
				t.Errorf("task created = %v, want %v", got, tc.wantTask)
			}
			if _, got := tx.reports[incidentReportTestID]; got != tc.wantReport {
				t.Errorf("report written = %v, want %v", got, tc.wantReport)
			}
		})
	}
}

// Every field of the Create Record step, as configured in ServiceNow.
func TestIncidentReport_TaskFields(t *testing.T) {
	now := time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC)
	tx := &fakeIncidentReportTx{src: sampleIncidentSource()}
	svc := newTestIncidentReportService(now)
	if err := svc.HandleChange(context.Background(), tx, incidentStateChange("NEW", "IN_PROGRESS", now)); err != nil {
		t.Fatalf("HandleChange: %v", err)
	}
	got := tx.created[0]
	if got.Subject != "[Incident Report] Create the incident report for INC0012345" {
		t.Errorf("subject = %q", got.Subject)
	}
	if got.IncidentID != incidentReportTestID {
		t.Errorf("incident = %q", got.IncidentID)
	}
	if got.ServiceID == nil || *got.ServiceID != "svc" ||
		got.AssignmentGroupID == nil || *got.AssignmentGroupID != "grp" ||
		got.AssignedToID == nil || *got.AssignedToID != "usr" {
		t.Errorf("service/group/assignee not copied from the incident: %+v", got)
	}
	if got.CreatedBy != "system" {
		t.Errorf("createdBy = %q, want system", got.CreatedBy)
	}
}

// An incident with no group or assignee still gets its task, with those left
// empty -- ServiceNow's data pills resolve to empty the same way.
func TestIncidentReport_TaskWithoutAssignment(t *testing.T) {
	now := time.Now()
	src := sampleIncidentSource()
	src.ServiceID, src.AssignmentGroupID, src.AssignedToID = nil, nil, nil
	tx := &fakeIncidentReportTx{src: src}
	if err := newTestIncidentReportService(now).HandleChange(context.Background(), tx, incidentStateChange("NEW", "IN_PROGRESS", now)); err != nil {
		t.Fatalf("HandleChange: %v", err)
	}
	if len(tx.created) != 1 || tx.created[0].AssignmentGroupID != nil || tx.created[0].AssignedToID != nil {
		t.Errorf("want one task with no assignment, got %+v", tx.created)
	}
}

// GOLDEN: the one report ServiceNow's Generator wrote on staging, for
// INC0015592 (created 2024-11-07 07:36:00 UTC, priority 5 - Planning), copied
// byte for byte from incident.incident_report.
func TestIncidentReport_Template(t *testing.T) {
	src := repository.IncidentReportSource{
		IncidentID: incidentReportTestID,
		Number:     "INC0015592",
		Priority:   strPtr("PLANNING"),
		CreatedOn:  time.Date(2024, 11, 7, 7, 36, 0, 0, time.UTC),
	}
	want := `<p><strong>Incident Number</strong><br />INC0015592<br /><br /><strong>Incident Severity Level</strong><br />5 - Planning<br /><br /><strong>Incident Identification Time</strong><br />2024-11-07 07:36:00<br /><br /><strong>Timeline</strong><br />-<br /><br /><strong>Affected Users or Customers</strong><br />-<br /><br /><strong>Affected Functionality</strong><br />-<br /><br /><strong>Cause(s) if known</strong><br />-<br /><br /><strong>Initial Response Actions Taken</strong><br />-<br /><br /><strong>Next Steps</strong><br />-<br /><br /></p>`
	if got := renderIncidentReport(src); got != want {
		t.Errorf("template differs from ServiceNow's stored report\n got: %s\nwant: %s", got, want)
	}
}

func TestIncidentReport_TemplatePriorityAndEscaping(t *testing.T) {
	src := sampleIncidentSource()
	src.Priority = nil
	src.Number = `INC<1>&"x"`
	got := renderIncidentReport(src)
	if !strings.Contains(got, `<strong>Incident Severity Level</strong><br />-<br />`) {
		t.Errorf("missing priority should render as -: %s", got)
	}
	if !strings.Contains(got, `INC&lt;1&gt;&amp;&#34;x&#34;`) {
		t.Errorf("number not HTML-escaped: %s", got)
	}
	for enum, label := range map[string]string{"CRITICAL": "1 - Critical", "MODERATE": "3 - Moderate", "LOW": "4 - Low", "PLANNING": "5 - Planning"} {
		src.Priority = strPtr(enum)
		if !strings.Contains(renderIncidentReport(src), label) {
			t.Errorf("priority %s should render as %q", enum, label)
		}
	}
}

// A change is acted on however late it is drained: there is no age limit, so
// a drainer outage delays the flows but never drops them.
func TestIncidentReport_LateChangeStillApplied(t *testing.T) {
	now := time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC)
	tx := &fakeIncidentReportTx{src: sampleIncidentSource()}
	svc := newTestIncidentReportService(now)
	if err := svc.HandleChange(context.Background(), tx, incidentStateChange("NEW", "IN_PROGRESS", now.Add(-30*24*time.Hour))); err != nil {
		t.Fatalf("HandleChange: %v", err)
	}
	if len(tx.created) != 1 {
		t.Errorf("a month-old change did not create a task")
	}
}

func TestIncidentReport_DeletedIncidentIsNotAnError(t *testing.T) {
	now := time.Now()
	tx := &fakeIncidentReportTx{srcErr: repository.ErrIncidentNotFound}
	if err := newTestIncidentReportService(now).HandleChange(context.Background(), tx, incidentStateChange("NEW", "IN_PROGRESS", now)); err != nil {
		t.Fatalf("want nil for a deleted incident, got %v", err)
	}
}

// A failed write must come back as an error, so the repository rolls the row
// back and it is retried -- never silently marked done.
func TestIncidentReport_WriteErrorsPropagate(t *testing.T) {
	now := time.Now()
	boom := errors.New("boom")
	svc := newTestIncidentReportService(now)
	if err := svc.HandleChange(context.Background(), &fakeIncidentReportTx{src: sampleIncidentSource(), createErr: boom}, incidentStateChange("NEW", "IN_PROGRESS", now)); !errors.Is(err, boom) {
		t.Errorf("create error: got %v", err)
	}
	if err := svc.HandleChange(context.Background(), &fakeIncidentReportTx{src: sampleIncidentSource(), writeErr: boom}, incidentStateChange("IN_PROGRESS", "RESOLVED", now)); !errors.Is(err, boom) {
		t.Errorf("report error: got %v", err)
	}
	if err := svc.HandleChange(context.Background(), &fakeIncidentReportTx{srcErr: boom}, incidentStateChange("NEW", "IN_PROGRESS", now)); !errors.Is(err, boom) {
		t.Errorf("read error: got %v", err)
	}
}

// --- drainer ---------------------------------------------------------------

type fakeIncidentReportRepo struct {
	pending  []int64
	failFor  map[int64]error
	parkAt   int
	attempts map[int64]int
	done     []int64
	failures []int64

	// schema check: missingFor[i] is what the i-th MissingSchema call
	// returns; past the end, nothing is missing.
	missingFor   [][]string
	schemaChecks int
	pendingCalls int
	pendingErr   error
	calls        []string
}

func (f *fakeIncidentReportRepo) MissingSchema(context.Context) ([]string, error) {
	f.calls = append(f.calls, "schema")
	i := f.schemaChecks
	f.schemaChecks++
	if i < len(f.missingFor) {
		return f.missingFor[i], nil
	}
	return nil, nil
}

func (f *fakeIncidentReportRepo) PendingChanges(context.Context, int) ([]int64, error) {
	f.calls = append(f.calls, "pending")
	f.pendingCalls++
	if f.pendingErr != nil {
		err := f.pendingErr
		f.pendingErr = nil // one fault, then healthy
		return nil, err
	}
	return f.pending, nil
}

func (f *fakeIncidentReportRepo) ProcessChange(ctx context.Context, id int64, fn func(context.Context, repository.IncidentReportTx, repository.IncidentReportChange) error) (bool, error) {
	if err := f.failFor[id]; err != nil {
		return false, err
	}
	f.done = append(f.done, id)
	return true, nil
}

func (f *fakeIncidentReportRepo) RecordFailure(_ context.Context, id int64, _ string, maxAttempts int) (bool, error) {
	if f.attempts == nil {
		f.attempts = map[int64]int{}
	}
	f.attempts[id]++
	f.failures = append(f.failures, id)
	return f.attempts[id] >= maxAttempts, nil
}

// One failing row is recorded and does not stop the rows after it.
func TestIncidentReportDrainer_FailureDoesNotBlockBatch(t *testing.T) {
	repo := &fakeIncidentReportRepo{pending: []int64{1, 2, 3}, failFor: map[int64]error{2: errors.New("boom")}}
	d := NewIncidentReportDrainer(repo, NewIncidentReportService(), time.Second, IncidentReportMaxAttempts)
	applied, err := d.drainOnce(context.Background())
	if err != nil {
		t.Fatalf("drainOnce: %v", err)
	}
	if applied != 2 || len(repo.done) != 2 || repo.done[0] != 1 || repo.done[1] != 3 {
		t.Errorf("applied=%d done=%v, want rows 1 and 3", applied, repo.done)
	}
	if len(repo.failures) != 1 || repo.failures[0] != 2 {
		t.Errorf("failures = %v, want [2]", repo.failures)
	}
}

// A row that keeps failing is recorded on every pass until it parks.
func TestIncidentReportDrainer_ParksAfterMaxAttempts(t *testing.T) {
	repo := &fakeIncidentReportRepo{pending: []int64{7}, failFor: map[int64]error{7: errors.New("boom")}}
	d := NewIncidentReportDrainer(repo, NewIncidentReportService(), time.Second, 3)
	for i := 0; i < 3; i++ {
		if _, err := d.drainOnce(context.Background()); err != nil {
			t.Fatalf("drainOnce: %v", err)
		}
	}
	if repo.attempts[7] != 3 {
		t.Errorf("attempts = %d, want 3", repo.attempts[7])
	}
}

// Deployed ahead of migration 0181, the drainer must not poll the outbox at
// all: it waits, re-checking, and starts by itself once the schema appears.
func TestIncidentReportDrainer_WaitsForMigrationBeforePolling(t *testing.T) {
	repo := &fakeIncidentReportRepo{missingFor: [][]string{
		{"event_outbox.last_attempt_on", "incident trigger incident_outbox"},
		{"incident trigger incident_outbox"},
	}}
	d := NewIncidentReportDrainer(repo, NewIncidentReportService(), time.Millisecond, IncidentReportMaxAttempts)
	d.SchemaRecheck = time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	d.Run(ctx)

	if repo.schemaChecks < 3 {
		t.Fatalf("schema checked %d times, want at least 3 (missing, missing, present)", repo.schemaChecks)
	}
	if repo.calls[0] != "schema" || repo.calls[1] != "schema" || repo.calls[2] != "schema" {
		t.Errorf("polled the outbox before the schema was present: calls = %v", repo.calls[:3])
	}
	if repo.pendingCalls == 0 {
		t.Error("never started polling after the schema appeared")
	}
}

// If the schema disappears under a running drainer, it goes back to waiting
// instead of failing every poll.
func TestIncidentReportDrainer_SchemaFaultReturnsToWaiting(t *testing.T) {
	repo := &fakeIncidentReportRepo{pendingErr: &pgconn.PgError{Code: "42703"}}
	d := NewIncidentReportDrainer(repo, NewIncidentReportService(), time.Millisecond, IncidentReportMaxAttempts)
	d.SchemaRecheck = time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	d.Run(ctx)

	if repo.schemaChecks < 2 {
		t.Errorf("schema checked %d times, want a re-check after the 42703 fault", repo.schemaChecks)
	}
	if repo.pendingCalls < 2 {
		t.Errorf("polled %d times, want polling to resume after the re-check", repo.pendingCalls)
	}
}

func TestIsSchemaFault(t *testing.T) {
	for code, want := range map[string]bool{"42703": true, "42P01": true, "42883": true, "23505": false, "40001": false} {
		if got := isSchemaFault(fmt.Errorf("wrapped: %w", &pgconn.PgError{Code: code})); got != want {
			t.Errorf("isSchemaFault(%s) = %v, want %v", code, got, want)
		}
	}
	if isSchemaFault(errors.New("plain")) {
		t.Error("a non-database error is not a schema fault")
	}
}
