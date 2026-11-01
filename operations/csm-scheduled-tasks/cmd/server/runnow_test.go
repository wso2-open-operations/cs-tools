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

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/autoclosure"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/engine"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/ledger"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/registry"
)

func TestParseRunNowArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    *runNowArgs
		wantErr string
	}{
		{name: "no args runs the schedule", args: nil, want: nil},
		{name: "plain run-now", args: []string{"--run-now=case_autoclosure"},
			want: &runNowArgs{Task: "case_autoclosure"}},
		{name: "space separated value", args: []string{"--run-now", "case_autoclosure"},
			want: &runNowArgs{Task: "case_autoclosure"}},
		{name: "all narrowing flags",
			args: []string{"--run-now=case_autoclosure", "--dry-run", "--limit=20", "--states=solution_proposed, awaiting_info"},
			want: &runNowArgs{Task: "case_autoclosure", DryRun: true, Limit: 20, States: []string{"solution_proposed", "awaiting_info"}}},
		{name: "unknown task", args: []string{"--run-now=stale_cases"}, wantErr: "not a task that can be run on demand"},
		{name: "empty task", args: []string{"--run-now="}, wantErr: "not a task that can be run on demand"},
		{name: "unknown flag", args: []string{"--run-now=case_autoclosure", "--force"}, wantErr: "bad arguments"},
		{name: "unknown flag alone", args: []string{"--bogus"}, wantErr: "bad arguments"},
		{name: "stray positional", args: []string{"--run-now=case_autoclosure", "extra"}, wantErr: "unexpected argument"},
		{name: "positional alone", args: []string{"case_autoclosure"}, wantErr: "unexpected argument"},
		{name: "dry-run without run-now", args: []string{"--dry-run"}, wantErr: "only valid together with --run-now"},
		{name: "limit without run-now", args: []string{"--limit=5"}, wantErr: "only valid together with --run-now"},
		{name: "states without run-now", args: []string{"--states=solution_proposed"}, wantErr: "only valid together with --run-now"},
		{name: "zero limit", args: []string{"--run-now=case_autoclosure", "--limit=0"}, wantErr: "must be a positive number"},
		{name: "negative limit", args: []string{"--run-now=case_autoclosure", "--limit=-3"}, wantErr: "must be a positive number"},
		{name: "non-numeric limit", args: []string{"--run-now=case_autoclosure", "--limit=abc"}, wantErr: "bad arguments"},
		{name: "empty states", args: []string{"--run-now=case_autoclosure", "--states="}, wantErr: "--states is empty"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseRunNowArgs(tc.args)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestCheckRunNow(t *testing.T) {
	a := &runNowArgs{Task: "case_autoclosure"}
	if err := checkRunNow(a, false, ""); err == nil || !strings.Contains(err.Error(), "AUTOCLOSURE_ENABLED=true") {
		t.Fatalf("disabled task must be refused, got %v", err)
	}
	if err := checkRunNow(a, true, "case_autoclosure"); err == nil || !strings.Contains(err.Error(), "TRYOUT_TASK") {
		t.Fatalf("TRYOUT_TASK combination must be refused, got %v", err)
	}
	if err := checkRunNow(a, true, "  "); err != nil {
		t.Fatalf("enabled with blank TRYOUT_TASK must pass, got %v", err)
	}
}

func TestNarrowAutoclosure(t *testing.T) {
	base := autoclosure.RunConfig{DailyLimit: 500, States: []string{"solution_proposed", "awaiting_info"}}
	tests := []struct {
		name    string
		cfg     autoclosure.RunConfig
		args    runNowArgs
		want    autoclosure.RunConfig
		wantErr string
	}{
		{name: "no narrowing keeps env", cfg: base, args: runNowArgs{}, want: base},
		{name: "dry run forced", cfg: base, args: runNowArgs{DryRun: true},
			want: autoclosure.RunConfig{DailyLimit: 500, States: base.States, DryRun: true}},
		{name: "env dry run is never loosened", cfg: autoclosure.RunConfig{DailyLimit: 5, DryRun: true}, args: runNowArgs{},
			want: autoclosure.RunConfig{DailyLimit: 5, DryRun: true}},
		{name: "limit tightens", cfg: base, args: runNowArgs{Limit: 20},
			want: autoclosure.RunConfig{DailyLimit: 20, States: base.States}},
		{name: "limit equal to env", cfg: base, args: runNowArgs{Limit: 500},
			want: base},
		{name: "limit above env", cfg: base, args: runNowArgs{Limit: 501}, wantErr: "exceeds AUTOCLOSURE_DAILY_LIMIT"},
		{name: "env no cap allows any limit", cfg: autoclosure.RunConfig{}, args: runNowArgs{Limit: 100000},
			want: autoclosure.RunConfig{DailyLimit: 100000}},
		{name: "states subset", cfg: base, args: runNowArgs{States: []string{"solution_proposed"}},
			want: autoclosure.RunConfig{DailyLimit: 500, States: []string{"solution_proposed"}}},
		{name: "states duplicates collapse", cfg: base, args: runNowArgs{States: []string{"awaiting_info", "awaiting_info"}},
			want: autoclosure.RunConfig{DailyLimit: 500, States: []string{"awaiting_info"}}},
		{name: "state outside env set", cfg: autoclosure.RunConfig{States: []string{"solution_proposed"}},
			args: runNowArgs{States: []string{"awaiting_info"}}, wantErr: "not in AUTOCLOSURE_STATES"},
		{name: "empty env states means both", cfg: autoclosure.RunConfig{}, args: runNowArgs{States: []string{"awaiting_info"}},
			want: autoclosure.RunConfig{States: []string{"awaiting_info"}}},
		{name: "unknown state", cfg: base, args: runNowArgs{States: []string{"open"}}, wantErr: "not in AUTOCLOSURE_STATES"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := narrowAutoclosure(tc.cfg, tc.args)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestParseStrictBool(t *testing.T) {
	for raw, want := range map[string]bool{"": true, "true": true, "false": false, " False ": false, "1": true, "0": false} {
		got, err := parseStrictBool("K", raw, true)
		if err != nil || got != want {
			t.Errorf("parseStrictBool(%q) = %v, %v; want %v", raw, got, err, want)
		}
	}
	for _, raw := range []string{"yes", "off", "nope", "2"} {
		if _, err := parseStrictBool("AUTOCLOSURE_SCHEDULED", raw, true); err == nil {
			t.Errorf("parseStrictBool(%q) must fail", raw)
		}
	}
}

type fakeLedger struct{ attempts []string }

func (f *fakeLedger) Attempt(_ context.Context, name string, _ time.Time, _ time.Duration) (ledger.Claim, error) {
	f.attempts = append(f.attempts, name)
	return ledger.Claim{Allowed: true}, nil
}
func (f *fakeLedger) Complete(context.Context, string, int) error                { return nil }
func (f *fakeLedger) Fail(context.Context, string, int, string, time.Time) error { return nil }

func TestUnscheduledTaskIsNotFiredByTick(t *testing.T) {
	var closureRuns, otherRuns int
	tasks := []registry.Task{
		{Name: "case_autoclosure", Schedule: "* * * * *", Handler: func(context.Context) error { closureRuns++; return nil }},
		{Name: "other", Schedule: "* * * * *", Handler: func(context.Context) error { otherRuns++; return nil }},
	}
	led := &fakeLedger{}
	eng := engine.New(withoutUnscheduled(tasks, map[string]bool{"case_autoclosure": true}), led, nil, time.Hour, nil, false)
	eng.Tick(context.Background(), time.Now())
	if closureRuns != 0 || otherRuns != 1 {
		t.Fatalf("closure ran %d times, other %d; want 0 and 1", closureRuns, otherRuns)
	}
	if !reflect.DeepEqual(led.attempts, []string{"other"}) {
		t.Fatalf("ledger was asked about %v; the unscheduled task must not be claimed", led.attempts)
	}

	// Scheduled (the default): the task fires as before.
	eng = engine.New(withoutUnscheduled(tasks, map[string]bool{}), &fakeLedger{}, nil, time.Hour, nil, false)
	eng.Tick(context.Background(), time.Now())
	if closureRuns != 1 {
		t.Fatalf("scheduled task ran %d times, want 1", closureRuns)
	}
}
