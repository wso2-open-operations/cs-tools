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

func slaReportTestRow(stage string, breached bool, elapsedSeconds, targetSeconds float64) SlaReportClockRow {
	return SlaReportClockRow{
		Stage:          &stage,
		HasBreached:    &breached,
		ElapsedSeconds: &elapsedSeconds,
		TargetSeconds:  &targetSeconds,
	}
}

func TestCorrectSlaReportBusinessDay_LongRunningBusinessClockIsBreached(t *testing.T) {
	row := slaReportTestRow("IN_PROGRESS", false, 3600, 24*3600)
	schedule := "WSO2 Business days"
	started := time.Now().UTC().AddDate(0, 0, -60)
	pause := 0.0

	correctSlaReportBusinessDay(&row, &schedule, &started, &pause)

	if *row.Stage != "BREACHED" || !*row.HasBreached {
		t.Fatalf("a business-day clock 60 days old must be breached, got stage %s breached %v", *row.Stage, *row.HasBreached)
	}
	if *row.ElapsedSeconds < 24*3600 {
		t.Fatalf("elapsed %v must be at least the 24h target", *row.ElapsedSeconds)
	}
}

func TestCorrectSlaReportBusinessDay_FreshBusinessClockIsNotBreached(t *testing.T) {
	// Ten minutes old: running inside a 24h target whether or not today is a business day.
	row := slaReportTestRow("BREACHED", true, 999999, 24*3600)
	schedule := "WSO2 Business days"
	started := time.Now().UTC().Add(-10 * time.Minute)
	pause := 0.0

	correctSlaReportBusinessDay(&row, &schedule, &started, &pause)

	if *row.Stage != "IN_PROGRESS" || *row.HasBreached {
		t.Fatalf("a 10 minute old clock is not breached, got stage %s breached %v", *row.Stage, *row.HasBreached)
	}
	if *row.ElapsedSeconds > 10*60+5 {
		t.Fatalf("elapsed %v cannot exceed the 10 minutes since it started", *row.ElapsedSeconds)
	}
}

func TestCorrectSlaReportBusinessDay_LeavesEverythingElseAlone(t *testing.T) {
	started := time.Now().UTC().AddDate(0, 0, -60)
	pause := 0.0
	business := "WSO2 Business days"
	allDays := "24 x 7"

	cases := []struct {
		name     string
		stage    string
		schedule *string
		start    *time.Time
	}{
		{"wall-clock schedule", "IN_PROGRESS", &allDays, &started},
		{"finished clock", "ACHIEVED", &business, &started},
		{"paused clock", "PAUSED", &business, &started},
		{"cancelled clock", "CANCELLED", &business, &started},
		{"no schedule", "IN_PROGRESS", nil, &started},
		{"no start time", "IN_PROGRESS", &business, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			row := slaReportTestRow(tc.stage, false, 1234, 24*3600)
			correctSlaReportBusinessDay(&row, tc.schedule, tc.start, &pause)
			if *row.Stage != tc.stage || *row.HasBreached || *row.ElapsedSeconds != 1234 {
				t.Fatalf("row was changed: stage %s breached %v elapsed %v", *row.Stage, *row.HasBreached, *row.ElapsedSeconds)
			}
		})
	}
}

func TestCorrectSlaReportBusinessDay_NoClockOrTargetIsLeftAlone(t *testing.T) {
	started := time.Now().UTC().AddDate(0, 0, -60)
	schedule := "WSO2 Business days"

	// a case with no clock: every clock field is nil
	row := SlaReportClockRow{}
	correctSlaReportBusinessDay(&row, &schedule, &started, nil)
	if row.Stage != nil || row.HasBreached != nil || row.ElapsedSeconds != nil {
		t.Fatalf("a row without a clock must stay empty: %+v", row)
	}

	// a clock whose policy has no duration cannot be judged
	stage := "IN_PROGRESS"
	row = SlaReportClockRow{Stage: &stage}
	correctSlaReportBusinessDay(&row, &schedule, &started, nil)
	if row.HasBreached != nil || row.ElapsedSeconds != nil {
		t.Fatalf("a clock without a target must stay as read: %+v", row)
	}
}

func TestFillSlaReportFinishedElapsed(t *testing.T) {
	start := time.Date(2026, 3, 2, 9, 0, 0, 0, time.UTC) // a Monday
	end := start.Add(5 * time.Hour)
	flat := "24x7"
	business := slaScheduleWSO2BusinessDays
	pause := 3600.0
	zero := 0.0

	t.Run("flat clock uses end minus start less its pause", func(t *testing.T) {
		row := slaReportTestRow("ACHIEVED", false, 0, 3600)
		row.ElapsedSeconds = nil
		fillSlaReportFinishedElapsed(&row, &flat, &start, &end, &zero, &pause)
		if row.ElapsedSeconds == nil || *row.ElapsedSeconds != 4*3600 {
			t.Fatalf("got %v, want 14400", row.ElapsedSeconds)
		}
	})
	t.Run("business clock uses business time", func(t *testing.T) {
		row := slaReportTestRow("COMPLETED", false, 0, 3600)
		row.ElapsedSeconds = nil
		fillSlaReportFinishedElapsed(&row, &business, &start, &end, &zero, &zero)
		if row.ElapsedSeconds == nil || *row.ElapsedSeconds <= 0 || *row.ElapsedSeconds > 5*3600 {
			t.Fatalf("got %v, want within (0, 18000]", row.ElapsedSeconds)
		}
	})
	t.Run("stored figure, unfinished stage or missing end are left alone", func(t *testing.T) {
		stored := slaReportTestRow("ACHIEVED", false, 99, 3600)
		fillSlaReportFinishedElapsed(&stored, &flat, &start, &end, &zero, &zero)
		if *stored.ElapsedSeconds != 99 {
			t.Fatalf("stored figure changed to %v", *stored.ElapsedSeconds)
		}
		running := slaReportTestRow("IN_PROGRESS", false, 0, 3600)
		running.ElapsedSeconds = nil
		fillSlaReportFinishedElapsed(&running, &flat, &start, &end, &zero, &zero)
		noEnd := slaReportTestRow("ACHIEVED", false, 0, 3600)
		noEnd.ElapsedSeconds = nil
		fillSlaReportFinishedElapsed(&noEnd, &flat, &start, nil, &zero, &zero)
		if running.ElapsedSeconds != nil || noEnd.ElapsedSeconds != nil {
			t.Fatal("an unfinished clock or one with no end time must stay empty")
		}
	})
}
