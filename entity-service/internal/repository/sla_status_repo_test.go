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
	caseNumber, wso2CaseID, caseTitle *string
	fixedCaseType                     string
	product, severity, state          *string
}

func (f fakeSLAStatusScanRow) Scan(dest ...any) error {
	*dest[0].(*string) = f.workItemID
	*dest[1].(*string) = f.target
	*dest[2].(*float64) = f.elapsedPercent
	*dest[3].(*bool) = f.hasBreached
	*dest[4].(*string) = f.stage
	*dest[5].(**time.Time) = f.startedOn
	*dest[6].(**string) = f.caseNumber
	*dest[7].(**string) = f.wso2CaseID
	*dest[8].(**string) = f.caseTitle
	*dest[9].(*string) = f.fixedCaseType
	*dest[10].(**string) = f.product
	*dest[11].(*string) = strOrEmpty(f.severity)
	*dest[12].(**string) = f.state
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
}
