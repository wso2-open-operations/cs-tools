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
	"testing"
	"time"
)

// fakeSLAStatusScanRow feeds scanSLAStatus fixed values without a real
// database connection, exercising the exact Scan destination order/types
// the query in SearchActiveSLAStatuses uses.
type fakeSLAStatusScanRow struct {
	workItemID, target                string
	elapsedPercent                    float64
	hasBreached                       bool
	stage                             string
	startedOn                         *time.Time
	pausedOn                          *time.Time
	pauseDurationSeconds              *int64
	durationSeconds                   int64
	schedule                          string
	businessPauseSeconds              float64
	caseNumber, wso2CaseID, caseTitle *string
	fixedCaseType                     string
	product, severity, state          *string
	team, onboardingStatus            *string
	isEvaluation                      bool
}

func (f fakeSLAStatusScanRow) Scan(dest ...any) error {
	*dest[0].(*string) = f.workItemID
	*dest[1].(*string) = f.target
	*dest[2].(*float64) = f.elapsedPercent
	*dest[3].(*bool) = f.hasBreached
	*dest[4].(*string) = f.stage
	*dest[5].(**time.Time) = f.startedOn
	*dest[6].(**time.Time) = f.pausedOn
	*dest[7].(**int64) = f.pauseDurationSeconds
	*dest[8].(*int64) = f.durationSeconds
	*dest[9].(*string) = f.schedule
	*dest[10].(*float64) = f.businessPauseSeconds
	*dest[11].(**string) = f.caseNumber
	*dest[12].(**string) = f.wso2CaseID
	*dest[13].(**string) = f.caseTitle
	*dest[14].(*string) = f.fixedCaseType
	*dest[15].(**string) = f.product
	*dest[16].(*string) = strOrEmpty(f.severity)
	*dest[17].(**string) = f.state
	*dest[18].(**string) = f.team
	*dest[19].(**string) = f.onboardingStatus
	*dest[20].(*bool) = f.isEvaluation
	return nil
}

func strOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func TestScanSLAStatus(t *testing.T) {
	now := time.Now()

	t.Run("normal row, all fields present", func(t *testing.T) {
		row := fakeSLAStatusScanRow{
			workItemID: "case-1", target: "RESOLUTION", elapsedPercent: 260.44, hasBreached: true, stage: "IN_PROGRESS",
			startedOn: &now, caseNumber: strPtr("CS0001"), wso2CaseID: strPtr("SUB-1"), caseTitle: strPtr("Title"),
			fixedCaseType: "CASE", product: strPtr("API Manager"), severity: strPtr("S1"),
			team: strPtr("Castor"), onboardingStatus: strPtr("IN_PROGRESS"), isEvaluation: true,
			durationSeconds: 172800,
		}
		got, err := scanSLAStatus(row)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.ClockType != "resolution" {
			t.Errorf("ClockType = %q, want %q", got.ClockType, "resolution")
		}
		if got.IsPaused {
			t.Error("IsPaused = true, want false for stage IN_PROGRESS")
		}
		if got.Priority != "CRITICAL" {
			t.Errorf("Priority = %q, want CRITICAL (from S1)", got.Priority)
		}
		if got.State != "" {
			t.Errorf("State = %q, want empty (nil state column)", got.State)
		}
		if got.Team != "Castor" {
			t.Errorf("Team = %q, want %q", got.Team, "Castor")
		}
		if got.ProjectOnboardingStatus != "IN_PROGRESS" {
			t.Errorf("ProjectOnboardingStatus = %q, want %q", got.ProjectOnboardingStatus, "IN_PROGRESS")
		}
		if !got.IsEvaluationAccount {
			t.Error("IsEvaluationAccount = false, want true")
		}
		if got.DurationSeconds != 172800 {
			t.Errorf("DurationSeconds = %d, want 172800", got.DurationSeconds)
		}
	})

	t.Run("no account/project row leaves team/onboarding/evaluation at zero value", func(t *testing.T) {
		row := fakeSLAStatusScanRow{workItemID: "case-5", target: "RESPONSE", fixedCaseType: "CASE"}
		got, err := scanSLAStatus(row)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Team != "" || got.ProjectOnboardingStatus != "" || got.IsEvaluationAccount {
			t.Errorf("got Team=%q ProjectOnboardingStatus=%q IsEvaluationAccount=%v, want all zero values", got.Team, got.ProjectOnboardingStatus, got.IsEvaluationAccount)
		}
	})

	t.Run("paused stage sets IsPaused", func(t *testing.T) {
		row := fakeSLAStatusScanRow{workItemID: "case-2", target: "RESPONSE", stage: "PAUSED", fixedCaseType: "CASE"}
		got, err := scanSLAStatus(row)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !got.IsPaused {
			t.Error("IsPaused = false, want true for stage PAUSED")
		}
	})

	t.Run("NULL state column (no case-like extension row) does not error", func(t *testing.T) {
		// Regression: staging has CASE-typed work items with no "case"/
		// engagement/etc. row at all, so caseLikeStateColumn's COALESCE
		// yields SQL NULL. An earlier version of this scan used a
		// non-pointer string destination for state and panicked on this.
		row := fakeSLAStatusScanRow{workItemID: "case-3", target: "RESPONSE", fixedCaseType: "ANNOUNCEMENT", state: nil}
		got, err := scanSLAStatus(row)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.State != "" {
			t.Errorf("State = %q, want empty for a NULL state column", got.State)
		}
	})

	t.Run("unknown severity label leaves Priority empty rather than guessing", func(t *testing.T) {
		row := fakeSLAStatusScanRow{workItemID: "case-4", target: "RESPONSE", fixedCaseType: "CASE", severity: strPtr("")}
		got, err := scanSLAStatus(row)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Priority != "" {
			t.Errorf("Priority = %q, want empty for no severity", got.Priority)
		}
	})

	t.Run("currently paused: PausedOn set, PauseDurationSeconds is the already-completed total", func(t *testing.T) {
		pausedOn := now.Add(-10 * time.Minute)
		seconds := int64(3600)
		row := fakeSLAStatusScanRow{
			workItemID: "case-6", target: "WORKAROUND", fixedCaseType: "CASE", stage: "PAUSED",
			pausedOn: &pausedOn, pauseDurationSeconds: &seconds,
		}
		got, err := scanSLAStatus(row)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !got.IsPaused {
			t.Error("IsPaused = false, want true for stage PAUSED")
		}
		if got.PausedOn == nil || !got.PausedOn.Equal(pausedOn) {
			t.Errorf("PausedOn = %v, want %v", got.PausedOn, pausedOn)
		}
		if got.PauseDurationSeconds != 3600 {
			t.Errorf("PauseDurationSeconds = %d, want 3600", got.PauseDurationSeconds)
		}
	})

	t.Run("never paused: PausedOn nil, PauseDurationSeconds zero, not absent", func(t *testing.T) {
		row := fakeSLAStatusScanRow{workItemID: "case-7", target: "RESPONSE", fixedCaseType: "CASE"}
		got, err := scanSLAStatus(row)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.PausedOn != nil {
			t.Errorf("PausedOn = %v, want nil", got.PausedOn)
		}
		if got.PauseDurationSeconds != 0 {
			t.Errorf("PauseDurationSeconds = %d, want 0 for a NULL pause_duration column", got.PauseDurationSeconds)
		}
	})

	t.Run("live WSO2 Business days clock recomputes BusinessElapsedPercent instead of trusting sla_live's flat figure", func(t *testing.T) {
		// A Friday 09:00 start, read "now" at the following Monday 09:00:
		// flat wall-clock elapsed is 72h, but only one business day (Friday)
		// has actually elapsed -- sla_live's own live_elapsed_percentage (a
		// flat formula) can't know that, which is exactly the bug GetClockState
		// already corrects for and this bulk path must now match.
		friday := time.Date(2026, time.October, 30, 9, 0, 0, 0, time.UTC)
		row := fakeSLAStatusScanRow{
			workItemID: "case-8", target: "RESOLUTION", fixedCaseType: "CASE", stage: "IN_PROGRESS",
			startedOn: &friday, schedule: "WSO2 Business days",
			durationSeconds: int64((8 * time.Hour).Seconds()), elapsedPercent: 900, // sla_live's wrong flat figure
		}
		got, err := scanSLAStatus(row)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.BusinessElapsedPercent == 900 {
			t.Error("BusinessElapsedPercent = sla_live's flat figure unchanged, want the business-day-corrected value")
		}
	})

	t.Run("live 24x7 clock keeps sla_live's own flat figure untouched", func(t *testing.T) {
		row := fakeSLAStatusScanRow{
			workItemID: "case-9", target: "RESOLUTION", fixedCaseType: "CASE", stage: "IN_PROGRESS",
			startedOn: &now, schedule: "24 x 7", durationSeconds: 3600, elapsedPercent: 42,
		}
		got, err := scanSLAStatus(row)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.BusinessElapsedPercent != 42 {
			t.Errorf("BusinessElapsedPercent = %v, want unchanged 42 for a 24x7 schedule", got.BusinessElapsedPercent)
		}
	})

	t.Run("PAUSED WSO2 Business days clock keeps its frozen figure, not recomputed", func(t *testing.T) {
		row := fakeSLAStatusScanRow{
			workItemID: "case-10", target: "RESOLUTION", fixedCaseType: "CASE", stage: "PAUSED",
			startedOn: &now, schedule: "WSO2 Business days", durationSeconds: 3600, elapsedPercent: 55,
		}
		got, err := scanSLAStatus(row)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.BusinessElapsedPercent != 55 {
			t.Errorf("BusinessElapsedPercent = %v, want unchanged 55 for a PAUSED row", got.BusinessElapsedPercent)
		}
	})
}
