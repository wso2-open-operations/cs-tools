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

package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/acp"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/registry"
)

// ACP_DAILY_LIMIT is a plain non-negative integer; unset or 0 is no cap, and
// anything else stops startup rather than silently lifting the cap.
func TestACPDailyLimitParsing(t *testing.T) {
	for _, tc := range []struct {
		raw     string
		want    int
		wantErr bool
	}{
		{"", 0, false},
		{"0", 0, false},
		{"25", 25, false},
		{"-1", 0, true},
		{"ten", 0, true},
		{"2.5", 0, true},
		{"1e3", 0, true},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			cfg, _, err := acpConfig(acpSettings{
				DryRun: true, AlertsEnabled: true, DailyLimit: tc.raw,
			})
			if tc.wantErr {
				if err == nil || !strings.Contains(err.Error(), "ACP_DAILY_LIMIT") {
					t.Fatalf("err = %v, want one naming ACP_DAILY_LIMIT", err)
				}
				return
			}
			if err != nil || cfg.DailyLimit != tc.want {
				t.Fatalf("limit = %d, err = %v, want %d", cfg.DailyLimit, err, tc.want)
			}
		})
	}
}

func TestNarrowACP(t *testing.T) {
	real := acp.Config{EmailEnabled: true, DailyLimit: 50}
	for _, tc := range []struct {
		name     string
		cfg      acp.Config
		args     registry.RunNowOptions
		hasDebug bool
		want     acp.Config
		wantErr  string
	}{
		{name: "no narrowing keeps the settings", cfg: real, want: real},
		{name: "dry run forced, emails off without a debug list", cfg: real, args: registry.RunNowOptions{DryRun: true},
			want: acp.Config{DryRun: true, EmailEnabled: false, DailyLimit: 50}},
		{name: "dry run forced keeps emails to a debug list", cfg: real, args: registry.RunNowOptions{DryRun: true}, hasDebug: true,
			want: acp.Config{DryRun: true, EmailEnabled: true, DailyLimit: 50}},
		{name: "a configured dry run is never loosened", cfg: acp.Config{DryRun: true}, want: acp.Config{DryRun: true}},
		{name: "limit tightens", cfg: real, args: registry.RunNowOptions{Limit: 10}, want: acp.Config{EmailEnabled: true, DailyLimit: 10}},
		{name: "limit equal to the configured one", cfg: real, args: registry.RunNowOptions{Limit: 50}, want: real},
		{name: "limit above the configured one is refused", cfg: real, args: registry.RunNowOptions{Limit: 51}, wantErr: "exceeds ACP_DAILY_LIMIT=50"},
		{name: "no configured limit allows one", cfg: acp.Config{EmailEnabled: true}, args: registry.RunNowOptions{Limit: 5000},
			want: acp.Config{EmailEnabled: true, DailyLimit: 5000}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := narrowACP(tc.cfg, tc.args, tc.hasDebug)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tc.wantErr)
				}
				return
			}
			if err != nil || got.DryRun != tc.want.DryRun || got.EmailEnabled != tc.want.EmailEnabled || got.DailyLimit != tc.want.DailyLimit {
				t.Fatalf("got %+v, %v; want %+v", got, err, tc.want)
			}
		})
	}
}

// The hook wired the way acpTask does: --dry-run and --limit are accepted,
// --states is refused for this task, and a loosening limit is refused through
// resolveRunNow.
func TestACPHookFlags(t *testing.T) {
	cfg := acp.Config{EmailEnabled: true, DailyLimit: 20}
	var seen acp.Config
	tasks := []registry.Task{{
		Name:    acpTaskName,
		Handler: func(context.Context) error { return nil },
		RunNowNarrow: func(o registry.RunNowOptions) (func(context.Context) error, []any, error) {
			c, err := narrowACP(cfg, o, false)
			if err != nil {
				return nil, nil, err
			}
			seen = c
			return func(context.Context) error { return nil }, []any{"dailyLimit", c.DailyLimit}, nil
		},
		RunNowFlags: []string{"--dry-run", "--limit"},
	}}

	if _, _, err := resolveRunNow(tasks, runNowArgs{Task: acpTaskName,
		Options: registry.RunNowOptions{DryRun: true, Limit: 5}, Given: []string{"--dry-run", "--limit"}}); err != nil {
		t.Fatalf("--dry-run --limit: %v", err)
	}
	if !seen.DryRun || seen.EmailEnabled || seen.DailyLimit != 5 {
		t.Errorf("effective config = %+v, want dry run, emails off, limit 5", seen)
	}
	if _, _, err := resolveRunNow(tasks, runNowArgs{Task: acpTaskName, Given: []string{"--states"}}); err == nil || !strings.Contains(err.Error(), "--states is not supported") {
		t.Errorf("--states err = %v, want refused", err)
	}
	if _, _, err := resolveRunNow(tasks, runNowArgs{Task: acpTaskName,
		Options: registry.RunNowOptions{Limit: 21}, Given: []string{"--limit"}}); err == nil || !strings.Contains(err.Error(), "exceeds ACP_DAILY_LIMIT=20") {
		t.Errorf("--limit=21 err = %v, want refused", err)
	}
}

// ACP_SCREENING is on unless set to off; anything else stops startup. The
// weekday is Mon to Sun (default Monday).
func TestACPScreeningSettings(t *testing.T) {
	for _, tc := range []struct {
		screening, weekday string
		wantOn             bool
		wantDay            time.Weekday
		wantErr            string
	}{
		{"", "", true, time.Monday, ""},
		{"on", "", true, time.Monday, ""},
		{"ON", "fri", true, time.Friday, ""},
		{"off", "", false, time.Monday, ""},
		{"Off", "Sunday", false, time.Sunday, ""},
		{"", "Wed", true, time.Wednesday, ""},
		{"maybe", "", false, 0, "ACP_SCREENING"},
		{"true", "", false, 0, "ACP_SCREENING"},
		{"on", "Someday", false, 0, "ACP_SCREENING_FULL_SWEEP_WEEKDAY"},
		{"on", "7", false, 0, "ACP_SCREENING_FULL_SWEEP_WEEKDAY"},
	} {
		t.Run(tc.screening+"/"+tc.weekday, func(t *testing.T) {
			cfg, _, err := acpConfig(acpSettings{DryRun: true, AlertsEnabled: true, Screening: tc.screening, FullSweepWeekday: tc.weekday})
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want one naming %s", err, tc.wantErr)
				}
				return
			}
			if err != nil || cfg.Screening != tc.wantOn || cfg.FullSweepWeekday != tc.wantDay {
				t.Fatalf("screening=%v day=%v err=%v, want %v %v", cfg.Screening, cfg.FullSweepWeekday, err, tc.wantOn, tc.wantDay)
			}
		})
	}
}

func TestNarrowACPFullSweep(t *testing.T) {
	cfg := acp.Config{Screening: true, DailyLimit: 10}
	got, err := narrowACP(cfg, registry.RunNowOptions{FullSweep: true, Limit: 5}, false)
	if err != nil || !got.FullSweep || got.DailyLimit != 5 || got.DryRun {
		t.Errorf("got %+v, %v; want full sweep, limit 5, not a dry run", got, err)
	}
	got, _ = narrowACP(cfg, registry.RunNowOptions{}, false)
	if got.FullSweep {
		t.Error("full sweep set without the flag")
	}
	args, err := parseRunNowArgs([]string{"--run-now=acp_closure", "--full-sweep"})
	if err != nil || !args.Options.FullSweep || len(args.Given) != 1 || args.Given[0] != "--full-sweep" {
		t.Errorf("parse = %+v, %v", args, err)
	}
	tasks := []registry.Task{{Name: acpTaskName, Handler: func(context.Context) error { return nil }}}
	if _, _, err := resolveRunNow(tasks, runNowArgs{Task: acpTaskName, Given: []string{"--full-sweep"}}); err == nil || !strings.Contains(err.Error(), "--full-sweep is not supported") {
		t.Errorf("a task without the hook: err = %v, want refused", err)
	}
}
