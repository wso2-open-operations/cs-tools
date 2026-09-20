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

// Tests BuildRuntimeConfig: unknownStatusPolicy wiring and holiday
// day-index derivation.
package ingest

import (
	"testing"
	"time"

	"github.com/binara-sachin/git-internals-dashboard/backend/internal/config"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/sla"
)

func runtimeFixtureApp(policy config.UnknownStatusPolicy, holidays []string) *config.AppConfig {
	return &config.AppConfig{
		Taxonomy: config.Taxonomy{
			Statuses: []config.StatusEntry{
				{Name: "Open", Category: config.CategoryProductSide, AccruesSla: true},
				{Name: "WOC", Category: config.CategoryCSSide, AccruesSla: false},
			},
		},
		Budgets:  []config.BudgetEntry{{Priority: "Critical(P1)", BudgetHours: 24, Coverage: config.Coverage24x7, Rank: 1}},
		Settings: config.Settings{AtRiskThreshold: 0.75, UnknownStatusPolicy: policy},
		Holidays: holidays,
	}
}

// TestBuildRuntimeConfigDefaultPolicyPausesUnknownStatus verifies the
// "pause" policy (today's implicit behavior) does not accrue for a status
// absent from taxonomy.statuses.
func TestBuildRuntimeConfigDefaultPolicyPausesUnknownStatus(t *testing.T) {
	rt := BuildRuntimeConfig(runtimeFixtureApp(config.UnknownStatusPause, nil))
	if rt.Cfg.Accrues(strp("Some New Column")) {
		t.Errorf("expected an unknown status to pause under policy=pause")
	}
	if !rt.Cfg.Accrues(strp("Open")) {
		t.Errorf("expected a known accruing status to still accrue")
	}
}

// TestBuildRuntimeConfigAccruePolicyAccruesUnknownStatus verifies the
// "accrue" policy treats an unknown status as product-side.
func TestBuildRuntimeConfigAccruePolicyAccruesUnknownStatus(t *testing.T) {
	rt := BuildRuntimeConfig(runtimeFixtureApp(config.UnknownStatusAccrue, nil))
	if !rt.Cfg.Accrues(strp("Some New Column")) {
		t.Errorf("expected an unknown status to accrue under policy=accrue")
	}
	if rt.Cfg.Accrues(strp("WOC")) {
		t.Errorf("expected a known pausing status to still pause")
	}
}

// TestBuildRuntimeConfigAccruePolicyNeverAccruesNilStatus verifies a nil
// status (off-board — AdjustForClosure relies on this to keep pausing once
// an issue closes) never accrues, regardless of unknownStatusPolicy.
func TestBuildRuntimeConfigAccruePolicyNeverAccruesNilStatus(t *testing.T) {
	rt := BuildRuntimeConfig(runtimeFixtureApp(config.UnknownStatusAccrue, nil))
	if rt.Cfg.Accrues(nil) {
		t.Errorf("expected nil status to never accrue, even under policy=accrue")
	}
}

// TestBuildRuntimeConfigThreadsHolidaysAsDayIndices verifies a configured
// holiday date lands in Cfg.Holidays under the same day-index
// covered12x5IstMs/within12x5Ist key by.
func TestBuildRuntimeConfigThreadsHolidaysAsDayIndices(t *testing.T) {
	rt := BuildRuntimeConfig(runtimeFixtureApp(config.UnknownStatusPause, []string{"2026-01-26"}))
	want, err := time.Parse("2006-01-02", "2026-01-26")
	if err != nil {
		t.Fatalf("parse fixture date: %v", err)
	}
	if !rt.Cfg.Holidays[sla.HolidayDayIndex(want)] {
		t.Errorf("expected 2026-01-26 to be present in Cfg.Holidays")
	}
	if len(rt.Cfg.Holidays) != 1 {
		t.Errorf("expected exactly 1 holiday, got %d", len(rt.Cfg.Holidays))
	}
}
