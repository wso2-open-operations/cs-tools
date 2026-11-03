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
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/notify"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/registry"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/worklogreport"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/worklogsreport"
)

func day(s string) time.Time {
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return d
}

func TestParseRunNowArgsWorklogFlags(t *testing.T) {
	base := "--run-now=cs_worklogs_weekly_report"
	good, err := parseRunNowArgs([]string{base, "--from-date=2030-01-07", "--to-date=2030-01-14",
		"--recipients=jane.doe@example.com,john.roe@example.com", "--cc=team@example.com"})
	if err != nil {
		t.Fatalf("valid example: %v", err)
	}
	want := &runNowArgs{Task: "cs_worklogs_weekly_report",
		Options: registry.RunNowOptions{FromDate: day("2030-01-07"), ToDate: day("2030-01-14"),
			Recipients: []string{"jane.doe@example.com", "john.roe@example.com"}, Cc: []string{"team@example.com"}},
		Given: []string{"--from-date", "--to-date", "--recipients", "--cc"}}
	if !reflect.DeepEqual(good, want) {
		t.Fatalf("got %+v, want %+v", good, want)
	}

	// A span of exactly 92 days passes; 93 does not.
	if _, err := parseRunNowArgs([]string{base, "--from-date=2030-01-01", "--to-date=2030-04-02"}); err != nil {
		t.Errorf("92 days must pass: %v", err)
	}

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"from-date without run-now", []string{"--from-date=2030-01-01", "--to-date=2030-01-02"}, "only valid together with --run-now"},
		{"recipients without run-now", []string{"--recipients=jane.doe@example.com"}, "only valid together with --run-now"},
		{"cc without run-now", []string{"--cc=jane.doe@example.com"}, "only valid together with --run-now"},
		{"only from-date", []string{base, "--from-date=2030-01-01"}, "must be given together"},
		{"only to-date", []string{base, "--to-date=2030-01-01"}, "must be given together"},
		{"malformed from-date", []string{base, "--from-date=01/02/2030", "--to-date=2030-01-02"}, "YYYY-MM-DD"},
		{"impossible date", []string{base, "--from-date=2030-02-30", "--to-date=2030-03-01"}, "YYYY-MM-DD"},
		{"timestamp not date", []string{base, "--from-date=2030-01-01T00:00:00Z", "--to-date=2030-01-02"}, "YYYY-MM-DD"},
		{"empty to-date", []string{base, "--from-date=2030-01-01", "--to-date="}, "YYYY-MM-DD"},
		{"to before from", []string{base, "--from-date=2030-01-05", "--to-date=2030-01-04"}, "is before"},
		{"93 days", []string{base, "--from-date=2030-01-01", "--to-date=2030-04-03"}, "at most 92"},
		{"cc alone", []string{base, "--cc=jane.doe@example.com"}, "--cc needs --recipients"},
		{"empty recipients", []string{base, "--recipients="}, "empty entry"},
		{"trailing comma", []string{base, "--recipients=jane.doe@example.com,"}, "empty entry"},
		{"space after comma", []string{base, "--recipients=jane.doe@example.com, john.roe@example.com"}, "whitespace"},
		{"display name", []string{base, "--recipients=Jane <jane.doe@example.com>"}, "whitespace"},
		{"display name no space", []string{base, "--recipients=Jane<jane.doe@example.com>"}, "not a plain email"},
		{"no at sign", []string{base, "--recipients=jane.doe"}, "not a plain email"},
		{"no domain dot", []string{base, "--recipients=jane@localhost"}, "no valid domain"},
		{"malformed cc", []string{base, "--recipients=jane.doe@example.com", "--cc=nope"}, "--cc entry"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseRunNowArgs(tc.args)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want containing %q", err, tc.wantErr)
			}
		})
	}
}

// The new flags are refused for any task that does not declare them, and the
// old narrowing flags are refused for the worklog tasks.
func TestResolveRunNowWorklogFlagsNeedDeclaration(t *testing.T) {
	var s, h int
	var o registry.RunNowOptions
	tasks := resolveFixture(&s, &h, &o) // simple_task: no hook; hooked_task: dry-run/limit/states
	tasks = append(tasks, registry.Task{Name: "cs_worklogs_weekly_report",
		Handler:      func(context.Context) error { return nil },
		RunNowNarrow: func(registry.RunNowOptions) (func(context.Context) error, []any, error) { return nil, nil, nil },
		RunNowFlags:  worklogsRunNowFlags})
	for _, task := range []string{"simple_task", "hooked_task"} {
		for _, f := range worklogsRunNowFlags {
			_, _, err := resolveRunNow(tasks, runNowArgs{Task: task, Given: []string{f}})
			if want := f + " is not supported by task " + task; err == nil || !strings.Contains(err.Error(), want) {
				t.Errorf("%s %s: err = %v, want containing %q", task, f, err, want)
			}
		}
	}
	for _, f := range []string{"--dry-run", "--limit", "--states"} {
		_, _, err := resolveRunNow(tasks, runNowArgs{Task: "cs_worklogs_weekly_report", Given: []string{f}})
		if err == nil || !strings.Contains(err.Error(), "not supported by task cs_worklogs_weekly_report") {
			t.Errorf("%s on a worklog task: err = %v", f, err)
		}
	}
	if _, _, err := resolveRunNow(tasks, runNowArgs{Task: "cs_worklogs_weekly_report", Given: []string{"--from-date", "--to-date"}}); err != nil {
		t.Errorf("worklog task must accept its flags: %v", err)
	}
}

// TRYOUT_TASK is refused with --run-now whatever the flags, so the override
// flags can never weaken the tryout's debug-recipients rule.
func TestWorklogFlagsAreRefusedWithTryout(t *testing.T) {
	a, err := parseRunNowArgs([]string{"--run-now=cs_worklogs_weekly_report", "--recipients=jane.doe@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if err := checkRunNow(a, "cs_worklogs_weekly_report"); err == nil || !strings.Contains(err.Error(), "TRYOUT_TASK") {
		t.Fatalf("recipient override with TRYOUT_TASK must be refused, got %v", err)
	}
	if err := checkRunNow(a, ""); err != nil {
		t.Fatalf("no tryout must pass: %v", err)
	}
}

func TestWorklogsRunNowHook(t *testing.T) {
	var built int
	var got worklogsreport.Override
	hook := func(cadence worklogreport.Cadence) func(registry.RunNowOptions) (func(context.Context) error, []any, error) {
		raw := &recordingSender{}
		return worklogsRunNow(cadence, raw, raw,
			func(_ notify.Sender, win worklogsreport.Override, _ []string) func(context.Context) error {
				got = win
				return func(context.Context) error { built++; return nil }
			})
	}
	ctx := context.Background()

	// No override: the handler is built with the zero window, like the scheduled one.
	h, settings, err := hook(worklogreport.CadenceWeekly)(registry.RunNowOptions{})
	if err != nil || len(settings) != 0 {
		t.Fatalf("plain: %v %v", err, settings)
	}
	_ = h(ctx)
	if built != 1 || got.HasWindow() {
		t.Fatalf("built=%d override=%+v", built, got)
	}

	// Weekly: any valid window passes as is.
	opts := registry.RunNowOptions{FromDate: day("2030-01-07"), ToDate: day("2030-01-09"),
		Recipients: []string{"jane.doe@example.com"}}
	h, settings, err = hook(worklogreport.CadenceWeekly)(opts)
	if err != nil {
		t.Fatalf("weekly: %v", err)
	}
	_ = h(ctx)
	if built != 2 || !got.From.Equal(day("2030-01-07")) || !got.To.Equal(day("2030-01-09")) {
		t.Fatalf("built=%d override=%+v", built, got)
	}
	// Addresses are never in the logged settings.
	for _, v := range settings {
		if s, ok := v.(string); ok && strings.Contains(s, "@") {
			t.Errorf("settings leak an address: %v", settings)
		}
	}

	// Daily: a single day passes, a range is refused.
	if _, _, err := hook(worklogreport.CadenceDaily)(registry.RunNowOptions{FromDate: day("2030-01-07"), ToDate: day("2030-01-07")}); err != nil {
		t.Errorf("daily single day: %v", err)
	}
	if _, _, err := hook(worklogreport.CadenceDaily)(opts); err == nil || !strings.Contains(err.Error(), "single day") {
		t.Errorf("daily range must be refused, got %v", err)
	}
}
