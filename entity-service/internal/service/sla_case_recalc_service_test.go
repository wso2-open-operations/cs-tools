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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

func TestRecalculateCasePauseTotals_InvalidCaseID_ValidationError(t *testing.T) {
	repo := newFakeFullSLAEngineRepo()
	svc := NewSLAEngineService(repo)

	_, err := svc.RecalculateCasePauseTotals(context.Background(), "not-a-uuid")
	var valErr *apierror.ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("err = %v, want a *apierror.ValidationError", err)
	}
}

func TestRecalculateCasePauseTotals_NoActivityHistory_HasHistoryFalse(t *testing.T) {
	repo := newFakeFullSLAEngineRepo()
	svc := NewSLAEngineService(repo)

	report, err := svc.RecalculateCasePauseTotals(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.HasHistory {
		t.Error("HasHistory = true, want false for a case with no state activity rows")
	}
	if len(report.Clocks) != 0 {
		t.Errorf("Clocks = %+v, want empty", report.Clocks)
	}
	if len(repo.pauseUpdates) != 0 {
		t.Errorf("pauseUpdates = %v, want no writes when there's no history to replay", repo.pauseUpdates)
	}
}

func TestRecalculateCasePauseTotals_PausedClock_CorrectsFrozenSnapshot(t *testing.T) {
	repo := newFakeFullSLAEngineRepo()
	now := time.Now()
	startOn := now.Add(-10 * time.Hour)
	pause1 := now.Add(-9 * time.Hour)
	resume1 := now.Add(-7 * time.Hour) // 1st pause span: 2h, completed
	pause2 := now.Add(-2 * time.Hour)  // currently still paused since here

	repo.activityEventsByCase["11111111-1111-1111-1111-111111111111"] = []repository.CaseActivityEvent{
		{FieldName: "state", OldValue: "NEW", NewValue: "Work In Progress", CreatedOn: startOn.Add(-time.Minute)},
		{FieldName: "state", OldValue: "Work In Progress", NewValue: "Awaiting Info", CreatedOn: pause1},
		{FieldName: "state", OldValue: "Awaiting Info", NewValue: "Work In Progress", CreatedOn: resume1},
		{FieldName: "state", OldValue: "Work In Progress", NewValue: "Awaiting Info", CreatedOn: pause2},
	}
	repo.activeClocksByCase["11111111-1111-1111-1111-111111111111"] = []repository.ActiveClockForRecalc{
		{
			PolicyID: "pol-1", Target: "RESOLUTION", Schedule: "24 x 7", Stage: "PAUSED", StartOn: startOn,
			PauseCondition:  "state=AWAITING_INFO",
			ResumeCondition: "state=WORK_IN_PROGRESS",
			// Deliberately wrong -- this is what the recalculation must correct.
			PauseSeconds:    0,
			PausedOn:        &pause2,
			DurationSeconds: int64((12 * time.Hour).Seconds()),
		},
	}
	svc := NewSLAEngineService(repo)

	report, err := svc.RecalculateCasePauseTotals(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(report.Clocks) != 1 {
		t.Fatalf("Clocks = %+v, want exactly one", report.Clocks)
	}
	got := report.Clocks[0]
	if got.Skipped {
		t.Fatalf("Skipped = true (%q), want a corrected result", got.SkipReason)
	}
	wantSeconds := int64((2 * time.Hour).Seconds())
	if got.NewPauseSeconds != wantSeconds {
		t.Errorf("NewPauseSeconds = %d, want %d -- only the completed pause1->resume1 span, never the still-open pause2->now span", got.NewPauseSeconds, wantSeconds)
	}
	if !got.Updated {
		t.Error("Updated = false, want true (stored total was 0, real total is 2h)")
	}
	if len(repo.pauseUpdates) != 0 {
		t.Errorf("pauseUpdates = %v, want no plain UpdateClockPauseTotals write for a PAUSED clock -- it must go through UpdateClockPausedSnapshot instead", repo.pauseUpdates)
	}
	if len(repo.pausedSnapshotUpdates) != 1 {
		t.Fatalf("pausedSnapshotUpdates = %v, want exactly one write", repo.pausedSnapshotUpdates)
	}
	// elapsed = pausedOn.Sub(startOn) - pauseSeconds = 8h - 2h = 6h; duration
	// = 12h, so remaining = 6h and percent = 50%.
	want := "11111111-1111-1111-1111-111111111111|pol-1|7200|-|50.0000|21600|21600"
	if repo.pausedSnapshotUpdates[0] != want {
		t.Errorf("pausedSnapshotUpdates[0] = %q, want %q", repo.pausedSnapshotUpdates[0], want)
	}
}

func TestRecalculateCasePauseTotals_PausedClockReplayDisagrees_Skipped(t *testing.T) {
	repo := newFakeFullSLAEngineRepo()
	now := time.Now()
	startOn := now.Add(-3 * time.Hour)
	pauseAt := startOn.Add(30 * time.Minute)
	resumeAt := startOn.Add(time.Hour)

	// History's last transition leaves the case running again -- but the
	// live engine still tracks this clock as PAUSED. A real disagreement the
	// recalculation must not paper over.
	repo.activityEventsByCase["11111111-1111-1111-1111-111111111111"] = []repository.CaseActivityEvent{
		{FieldName: "state", OldValue: "NEW", NewValue: "Work In Progress", CreatedOn: startOn.Add(-time.Minute)},
		{FieldName: "state", OldValue: "Work In Progress", NewValue: "Awaiting Info", CreatedOn: pauseAt},
		{FieldName: "state", OldValue: "Awaiting Info", NewValue: "Work In Progress", CreatedOn: resumeAt},
	}
	repo.activeClocksByCase["11111111-1111-1111-1111-111111111111"] = []repository.ActiveClockForRecalc{
		{
			PolicyID: "pol-1", Target: "RESOLUTION", Schedule: "24 x 7", Stage: "PAUSED", StartOn: startOn,
			PauseCondition:  "state=AWAITING_INFO",
			ResumeCondition: "state=WORK_IN_PROGRESS",
			PausedOn:        &pauseAt,
			DurationSeconds: int64((12 * time.Hour).Seconds()),
		},
	}
	svc := NewSLAEngineService(repo)

	report, err := svc.RecalculateCasePauseTotals(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(report.Clocks) != 1 || !report.Clocks[0].Skipped {
		t.Fatalf("Clocks = %+v, want exactly one skipped result", report.Clocks)
	}
	if len(repo.pauseUpdates) != 0 || len(repo.pausedSnapshotUpdates) != 0 {
		t.Errorf("pauseUpdates = %v, pausedSnapshotUpdates = %v, want no writes on a genuine disagreement", repo.pauseUpdates, repo.pausedSnapshotUpdates)
	}
}

func TestRecalculateCasePauseTotals_LaterSeverityChange_Skipped(t *testing.T) {
	repo := newFakeFullSLAEngineRepo()
	startOn := time.Now().Add(-3 * time.Hour)
	repo.activityEventsByCase["11111111-1111-1111-1111-111111111111"] = []repository.CaseActivityEvent{
		{FieldName: "state", OldValue: "NEW", NewValue: "Work In Progress", CreatedOn: startOn.Add(-time.Hour)},
		{FieldName: "severity", OldValue: "S2", NewValue: "S1", CreatedOn: startOn.Add(time.Hour)}, // after this clock's own start
	}
	repo.activeClocksByCase["11111111-1111-1111-1111-111111111111"] = []repository.ActiveClockForRecalc{
		{PolicyID: "pol-1", Target: "RESOLUTION", Schedule: "24 x 7", Stage: "IN_PROGRESS", StartOn: startOn},
	}
	svc := NewSLAEngineService(repo)

	report, err := svc.RecalculateCasePauseTotals(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(report.Clocks) != 1 || !report.Clocks[0].Skipped {
		t.Fatalf("Clocks = %+v, want exactly one skipped result", report.Clocks)
	}
	if len(repo.pauseUpdates) != 0 {
		t.Errorf("pauseUpdates = %v, want no writes when a later severity change makes the row untrustworthy", repo.pauseUpdates)
	}
}

func TestRecalculateCasePauseTotals_PauseThenResume_CorrectsStoredTotal(t *testing.T) {
	repo := newFakeFullSLAEngineRepo()
	now := time.Now()
	startOn := now.Add(-4 * time.Hour)
	pauseAt := now.Add(-3 * time.Hour)  // 1h after start
	resumeAt := now.Add(-1 * time.Hour) // 2h paused (pauseAt -> resumeAt)

	repo.activityEventsByCase["11111111-1111-1111-1111-111111111111"] = []repository.CaseActivityEvent{
		{FieldName: "state", OldValue: "NEW", NewValue: "Work In Progress", CreatedOn: startOn.Add(-time.Minute)},
		{FieldName: "state", OldValue: "Work In Progress", NewValue: "Awaiting Info", CreatedOn: pauseAt},
		{FieldName: "state", OldValue: "Awaiting Info", NewValue: "Work In Progress", CreatedOn: resumeAt},
	}
	repo.activeClocksByCase["11111111-1111-1111-1111-111111111111"] = []repository.ActiveClockForRecalc{
		{
			PolicyID: "pol-1", Target: "RESOLUTION", Schedule: "24 x 7", Stage: "IN_PROGRESS", StartOn: startOn,
			PauseCondition:  "state=AWAITING_INFO",
			ResumeCondition: "state=WORK_IN_PROGRESS",
			// Deliberately wrong -- this is what the recalculation must correct.
			PauseSeconds: 0,
		},
	}
	svc := NewSLAEngineService(repo)

	report, err := svc.RecalculateCasePauseTotals(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(report.Clocks) != 1 {
		t.Fatalf("Clocks = %+v, want exactly one", report.Clocks)
	}
	got := report.Clocks[0]
	if got.Skipped {
		t.Fatalf("Skipped = true (%q), want a corrected result", got.SkipReason)
	}
	wantSeconds := int64((2 * time.Hour).Seconds())
	if got.NewPauseSeconds != wantSeconds {
		t.Errorf("NewPauseSeconds = %d, want %d (2 hours paused, AWAITING_INFO -> WORK_IN_PROGRESS)", got.NewPauseSeconds, wantSeconds)
	}
	if !got.Updated {
		t.Error("Updated = false, want true (stored total was 0, real total is 2h)")
	}
	if len(repo.pauseUpdates) != 1 {
		t.Fatalf("pauseUpdates = %v, want exactly one write", repo.pauseUpdates)
	}
}

func TestRecalculateCasePauseTotals_AlreadyCorrect_NoUpdate(t *testing.T) {
	repo := newFakeFullSLAEngineRepo()
	now := time.Now()
	startOn := now.Add(-4 * time.Hour)
	pauseAt := now.Add(-3 * time.Hour)
	resumeAt := now.Add(-1 * time.Hour) // exactly 2h paused

	repo.activityEventsByCase["11111111-1111-1111-1111-111111111111"] = []repository.CaseActivityEvent{
		{FieldName: "state", OldValue: "NEW", NewValue: "Work In Progress", CreatedOn: startOn.Add(-time.Minute)},
		{FieldName: "state", OldValue: "Work In Progress", NewValue: "Awaiting Info", CreatedOn: pauseAt},
		{FieldName: "state", OldValue: "Awaiting Info", NewValue: "Work In Progress", CreatedOn: resumeAt},
	}
	repo.activeClocksByCase["11111111-1111-1111-1111-111111111111"] = []repository.ActiveClockForRecalc{
		{
			PolicyID: "pol-1", Target: "RESOLUTION", Schedule: "24 x 7", Stage: "IN_PROGRESS", StartOn: startOn,
			PauseCondition:  "state=AWAITING_INFO",
			ResumeCondition: "state=WORK_IN_PROGRESS",
			PauseSeconds:    int64((2 * time.Hour).Seconds()), // already correct
		},
	}
	svc := NewSLAEngineService(repo)

	report, err := svc.RecalculateCasePauseTotals(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Clocks[0].Updated {
		t.Error("Updated = true, want false -- the stored total already matches the replayed total")
	}
	if len(repo.pauseUpdates) != 0 {
		t.Errorf("pauseUpdates = %v, want no writes when nothing needs correcting", repo.pauseUpdates)
	}
}

func TestRecalculateCasePauseTotals_BusinessDaysSchedule_CorrectsBothTotals(t *testing.T) {
	repo := newFakeFullSLAEngineRepo()
	// A fixed Tuesday 10:00 ET start with no weekend involved, business
	// time == flat time here -- this test only needs to confirm the
	// business-day column gets computed and written for this schedule, not
	// re-prove the weekend-skipping math itself (internal/businessdays has
	// its own dedicated tests for that). Fixed to a mid-day weekday (not
	// "now" or "a week ago") and mid-day start so the whole window can
	// never cross midnight or a weekend boundary depending on when this
	// test happens to run.
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	mostRecentTuesday := time.Now().In(loc).AddDate(0, 0, -7)
	for mostRecentTuesday.Weekday() != time.Tuesday {
		mostRecentTuesday = mostRecentTuesday.AddDate(0, 0, -1)
	}
	startOn := time.Date(mostRecentTuesday.Year(), mostRecentTuesday.Month(), mostRecentTuesday.Day(), 10, 0, 0, 0, loc)
	pauseAt := startOn.Add(time.Hour)
	resumeAt := startOn.Add(3 * time.Hour) // 2h paused, both within the same business day

	// The replay runs to the real time.Now() internally -- that's fine
	// here: once resumed (paused=false), nothing more accumulates no
	// matter how long the window to "now" actually is, so this test only
	// needs pauseAt/resumeAt to land on one weekday with no weekend
	// between them, not a fixed "now".
	repo.activityEventsByCase["11111111-1111-1111-1111-111111111111"] = []repository.CaseActivityEvent{
		{FieldName: "state", OldValue: "NEW", NewValue: "Work In Progress", CreatedOn: startOn.Add(-time.Minute)},
		{FieldName: "state", OldValue: "Work In Progress", NewValue: "Awaiting Info", CreatedOn: pauseAt},
		{FieldName: "state", OldValue: "Awaiting Info", NewValue: "Work In Progress", CreatedOn: resumeAt},
	}
	repo.activeClocksByCase["11111111-1111-1111-1111-111111111111"] = []repository.ActiveClockForRecalc{
		{
			PolicyID: "pol-1", Target: "RESOLUTION", Schedule: "WSO2 Business days", Stage: "IN_PROGRESS", StartOn: startOn,
			PauseCondition:  "state=AWAITING_INFO",
			ResumeCondition: "state=WORK_IN_PROGRESS",
		},
	}
	svc := NewSLAEngineService(repo)

	report, err := svc.RecalculateCasePauseTotals(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := report.Clocks[0]
	wantSeconds := int64((2 * time.Hour).Seconds())
	if got.NewPauseSeconds != wantSeconds {
		t.Errorf("NewPauseSeconds = %d, want %d", got.NewPauseSeconds, wantSeconds)
	}
	if got.NewBusinessPauseSeconds != wantSeconds {
		t.Errorf("NewBusinessPauseSeconds = %d, want %d (no weekend in this window)", got.NewBusinessPauseSeconds, wantSeconds)
	}
	if len(repo.pauseUpdates) != 1 {
		t.Fatalf("pauseUpdates = %v, want exactly one write", repo.pauseUpdates)
	}
}

func TestRecalculateCasePauseTotals_FlatSchedule_NeverTouchesBusinessPauseColumn(t *testing.T) {
	repo := newFakeFullSLAEngineRepo()
	now := time.Now()
	startOn := now.Add(-4 * time.Hour)
	pauseAt := now.Add(-3 * time.Hour)
	resumeAt := now.Add(-1 * time.Hour)

	repo.activityEventsByCase["11111111-1111-1111-1111-111111111111"] = []repository.CaseActivityEvent{
		{FieldName: "state", OldValue: "NEW", NewValue: "Work In Progress", CreatedOn: startOn.Add(-time.Minute)},
		{FieldName: "state", OldValue: "Work In Progress", NewValue: "Awaiting Info", CreatedOn: pauseAt},
		{FieldName: "state", OldValue: "Awaiting Info", NewValue: "Work In Progress", CreatedOn: resumeAt},
	}
	repo.activeClocksByCase["11111111-1111-1111-1111-111111111111"] = []repository.ActiveClockForRecalc{
		{
			PolicyID: "pol-1", Target: "RESOLUTION", Schedule: "24 x 7", Stage: "IN_PROGRESS", StartOn: startOn,
			PauseCondition:  "state=AWAITING_INFO",
			ResumeCondition: "state=WORK_IN_PROGRESS",
			// A pre-existing (wrong, for this schedule it's simply never
			// read) business_pause_duration figure -- must survive
			// untouched.
			BusinessPauseSeconds: 999,
		},
	}
	svc := NewSLAEngineService(repo)

	if _, err := svc.RecalculateCasePauseTotals(context.Background(), "11111111-1111-1111-1111-111111111111"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.pauseUpdates) != 1 {
		t.Fatalf("pauseUpdates = %v, want exactly one write", repo.pauseUpdates)
	}
	if repo.pauseUpdates[0] != "11111111-1111-1111-1111-111111111111|pol-1|7200|-" {
		t.Errorf("pauseUpdates[0] = %q, want businessPauseSeconds arg to be nil ('-') for a non-business-days schedule", repo.pauseUpdates[0])
	}
}

func TestRecalculateCasePauseTotals_UnrecognizedStateLabel_Skipped(t *testing.T) {
	repo := newFakeFullSLAEngineRepo()
	now := time.Now()
	startOn := now.Add(-4 * time.Hour)

	// "On Hold" isn't a real case_state_enum display label (that's an
	// incident-state label) -- normalizeActivityState must refuse to guess,
	// never silently treat an unrecognized value as "not paused".
	repo.activityEventsByCase["11111111-1111-1111-1111-111111111111"] = []repository.CaseActivityEvent{
		{FieldName: "state", OldValue: "NEW", NewValue: "Work In Progress", CreatedOn: startOn.Add(-time.Minute)},
		{FieldName: "state", OldValue: "Work In Progress", NewValue: "On Hold", CreatedOn: startOn.Add(time.Hour)},
	}
	repo.activeClocksByCase["11111111-1111-1111-1111-111111111111"] = []repository.ActiveClockForRecalc{
		{
			PolicyID: "pol-1", Target: "RESOLUTION", Schedule: "24 x 7", Stage: "IN_PROGRESS", StartOn: startOn,
			PauseCondition:  "state=AWAITING_INFO",
			ResumeCondition: "state=WORK_IN_PROGRESS",
		},
	}
	svc := NewSLAEngineService(repo)

	report, err := svc.RecalculateCasePauseTotals(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(report.Clocks) != 1 || !report.Clocks[0].Skipped {
		t.Fatalf("Clocks = %+v, want exactly one skipped result", report.Clocks)
	}
	if len(repo.pauseUpdates) != 0 && len(repo.pausedSnapshotUpdates) != 0 {
		t.Errorf("want no writes for an unrecognized state label")
	}
}

func TestRecalculateCasePauseTotals_NonStatePauseCondition_Skipped(t *testing.T) {
	repo := newFakeFullSLAEngineRepo()
	now := time.Now()
	startOn := now.Add(-4 * time.Hour)

	// A real WORKAROUND-target policy's own pause_condition can reference
	// u_workaround_provided, not state -- replaying that against today's
	// live facts instead of that fact's own history would misjudge the
	// clock's past pause/resume state, so it must be skipped, not replayed.
	repo.activityEventsByCase["11111111-1111-1111-1111-111111111111"] = []repository.CaseActivityEvent{
		{FieldName: "state", OldValue: "NEW", NewValue: "Work In Progress", CreatedOn: startOn.Add(-time.Minute)},
	}
	repo.activeClocksByCase["11111111-1111-1111-1111-111111111111"] = []repository.ActiveClockForRecalc{
		{
			PolicyID: "pol-1", Target: "WORKAROUND", Schedule: "24 x 7", Stage: "IN_PROGRESS", StartOn: startOn,
			PauseCondition:  "u_workaround_providedISNOTEMPTY",
			ResumeCondition: "u_workaround_providedISEMPTY",
		},
	}
	svc := NewSLAEngineService(repo)

	report, err := svc.RecalculateCasePauseTotals(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(report.Clocks) != 1 || !report.Clocks[0].Skipped {
		t.Fatalf("Clocks = %+v, want exactly one skipped result", report.Clocks)
	}
	if len(repo.pauseUpdates) != 0 && len(repo.pausedSnapshotUpdates) != 0 {
		t.Errorf("want no writes for a non-state pause condition")
	}
}

func TestRecalculateCasePauseTotals_ReplayDisagreesWithLiveStage_Skipped(t *testing.T) {
	repo := newFakeFullSLAEngineRepo()
	now := time.Now()
	startOn := now.Add(-2 * time.Hour)

	// History's last transition leaves the case AWAITING_INFO (paused) --
	// but the live engine still tracks this clock as IN_PROGRESS. A real
	// disagreement the recalculation must not paper over.
	repo.activityEventsByCase["11111111-1111-1111-1111-111111111111"] = []repository.CaseActivityEvent{
		{FieldName: "state", OldValue: "NEW", NewValue: "Work In Progress", CreatedOn: startOn.Add(-time.Minute)},
		{FieldName: "state", OldValue: "Work In Progress", NewValue: "Awaiting Info", CreatedOn: startOn.Add(time.Hour)},
	}
	repo.activeClocksByCase["11111111-1111-1111-1111-111111111111"] = []repository.ActiveClockForRecalc{
		{
			PolicyID: "pol-1", Target: "RESOLUTION", Schedule: "24 x 7", Stage: "IN_PROGRESS", StartOn: startOn,
			PauseCondition:  "state=AWAITING_INFO",
			ResumeCondition: "state=WORK_IN_PROGRESS",
		},
	}
	svc := NewSLAEngineService(repo)

	report, err := svc.RecalculateCasePauseTotals(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(report.Clocks) != 1 || !report.Clocks[0].Skipped {
		t.Fatalf("Clocks = %+v, want exactly one skipped result", report.Clocks)
	}
	if len(repo.pauseUpdates) != 0 {
		t.Errorf("pauseUpdates = %v, want no writes on a genuine disagreement", repo.pauseUpdates)
	}
}

func TestCaseSLARecalcService_RequiresCSMPortalBackendCaller(t *testing.T) {
	repo := newFakeFullSLAEngineRepo()
	repo.activityEventsByCase["11111111-1111-1111-1111-111111111111"] = []repository.CaseActivityEvent{
		{FieldName: "state", OldValue: "NEW", NewValue: "Work In Progress", CreatedOn: time.Now().Add(-time.Hour)},
	}
	engine := NewSLAEngineService(repo)
	svc := NewCaseSLARecalcService(engine, alwaysUnrestrictedAccess{}, testCSMPortalBackendClientID)

	t.Run("wrong client id is forbidden", func(t *testing.T) {
		ctx := auth.WithIdentity(context.Background(), auth.Identity{Validated: true, UserEmail: "admin@wso2.com", ClientID: "someone-else"})
		if _, err := svc.RecalculateCasePauseTotals(ctx, "11111111-1111-1111-1111-111111111111"); err == nil {
			t.Fatal("err = nil, want a forbidden error for a caller that isn't csm-portal-backend")
		}
	})

	t.Run("no signed-in user is forbidden", func(t *testing.T) {
		ctx := auth.WithIdentity(context.Background(), auth.Identity{Validated: true, ClientID: testCSMPortalBackendClientID})
		if _, err := svc.RecalculateCasePauseTotals(ctx, "11111111-1111-1111-1111-111111111111"); err == nil {
			t.Fatal("err = nil, want a forbidden error with no signed-in person")
		}
	})

	t.Run("csm-portal-backend caller with a signed-in admin succeeds", func(t *testing.T) {
		ctx := auth.WithIdentity(context.Background(), auth.Identity{Validated: true, UserEmail: "admin@wso2.com", ClientID: testCSMPortalBackendClientID})
		report, err := svc.RecalculateCasePauseTotals(ctx, "11111111-1111-1111-1111-111111111111")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !report.HasHistory {
			t.Error("HasHistory = false, want true")
		}
	})
}
