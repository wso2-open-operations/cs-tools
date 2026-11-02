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
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/autoclosure"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/engine"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/ledger"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/registry"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/schedule"
)

// TestAutoclosureDefaultScheduleFiresDaily pins the 02:30 UTC slot, weekends
// included (2026-11-07 is a Saturday, 2026-11-08 a Sunday).
func TestAutoclosureDefaultScheduleFiresDaily(t *testing.T) {
	for _, day := range []string{"2026-11-06", "2026-11-07", "2026-11-08", "2026-11-09"} {
		now, _ := time.Parse(time.RFC3339, day+"T12:00:00Z")
		got, err := schedule.PeriodKey(autoclosureDefaultSchedule, now)
		if err != nil {
			t.Fatalf("%s: %v", day, err)
		}
		if want := day + "T02:30:00Z"; got.Format(time.RFC3339) != want {
			t.Errorf("%s: period %s, want %s", day, got.Format(time.RFC3339), want)
		}
	}
}

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
		{name: "any task name parses", args: []string{"--run-now=stale_cases_report"},
			want: &runNowArgs{Task: "stale_cases_report"}},
		{name: "space separated value", args: []string{"--run-now", "usage_rollup_daily"},
			want: &runNowArgs{Task: "usage_rollup_daily"}},
		{name: "all narrowing flags",
			args: []string{"--run-now=case_autoclosure", "--dry-run", "--limit=20", "--states=solution_proposed, awaiting_info"},
			want: &runNowArgs{Task: "case_autoclosure",
				Options: registry.RunNowOptions{DryRun: true, Limit: 20, States: []string{"solution_proposed", "awaiting_info"}},
				Given:   []string{"--dry-run", "--limit", "--states"}}},
		{name: "empty task", args: []string{"--run-now="}, wantErr: "needs a task name"},
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
	if err := checkRunNow(a, "case_autoclosure"); err == nil || !strings.Contains(err.Error(), "TRYOUT_TASK") {
		t.Fatalf("TRYOUT_TASK combination must be refused, got %v", err)
	}
	if err := checkRunNow(a, "  "); err != nil {
		t.Fatalf("blank TRYOUT_TASK must pass, got %v", err)
	}
}

// resolveFixture is a registered simple task (no narrowing hook) and one with a
// hook that records what it was given, like case_autoclosure's.
func resolveFixture(simpleRan, hookedRan *int, gotOpts *registry.RunNowOptions) []registry.Task {
	return []registry.Task{
		{Name: "simple_task", Handler: func(context.Context) error { *simpleRan++; return nil }},
		{Name: "hooked_task", Handler: func(context.Context) error { return errors.New("plain handler must not be used") },
			RunNowNarrow: func(o registry.RunNowOptions) (func(context.Context) error, []any, error) {
				*gotOpts = o
				if o.Limit > 10 {
					return nil, nil, errors.New("limit too high")
				}
				return func(context.Context) error { *hookedRan++; return nil }, []any{"limit", o.Limit}, nil
			}},
	}
}

func TestResolveRunNow(t *testing.T) {
	var simple, hooked int
	var opts registry.RunNowOptions
	tasks := resolveFixture(&simple, &hooked, &opts)
	ctx := context.Background()

	// A task without a hook runs its own handler, with no settings to add.
	h, settings, err := resolveRunNow(tasks, runNowArgs{Task: "simple_task"})
	if err != nil || len(settings) != 0 {
		t.Fatalf("simple: err=%v settings=%v", err, settings)
	}
	if err := h(ctx); err != nil || simple != 1 || hooked != 0 {
		t.Fatalf("simple handler ran %d times (hooked %d), err %v", simple, hooked, err)
	}

	// A task with a hook runs the handler the hook returns, given the options.
	args := runNowArgs{Task: "hooked_task", Options: registry.RunNowOptions{Limit: 5}, Given: []string{"--limit"}}
	h, settings, err = resolveRunNow(tasks, args)
	if err != nil || !reflect.DeepEqual(settings, []any{"limit", 5}) || opts.Limit != 5 {
		t.Fatalf("hooked: err=%v settings=%v opts=%+v", err, settings, opts)
	}
	if err := h(ctx); err != nil || hooked != 1 {
		t.Fatalf("hooked handler ran %d times, err %v", hooked, err)
	}

	// The hook may refuse a value.
	args.Options.Limit = 50
	if _, _, err := resolveRunNow(tasks, args); err == nil || !strings.Contains(err.Error(), "limit too high") {
		t.Fatalf("hook error must surface, got %v", err)
	}
}

func TestResolveRunNowUnregisteredListsRegistered(t *testing.T) {
	var a, b int
	var o registry.RunNowOptions
	tasks := resolveFixture(&a, &b, &o)
	_, _, err := resolveRunNow(tasks, runNowArgs{Task: "case_autoclosure"})
	if err == nil {
		t.Fatal("an unregistered task must be refused")
	}
	for _, want := range []string{"not a registered task", "enable flag is off", "hooked_task, simple_task"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q should contain %q", err, want)
		}
	}
}

func TestResolveRunNowNarrowingNeedsHook(t *testing.T) {
	var a, b int
	var o registry.RunNowOptions
	tasks := resolveFixture(&a, &b, &o)
	for _, flagName := range []string{"--dry-run", "--limit", "--states"} {
		_, _, err := resolveRunNow(tasks, runNowArgs{Task: "simple_task", Given: []string{flagName}})
		want := flagName + " is not supported by task simple_task"
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err = %v, want containing %q", flagName, err, want)
		}
	}
	if a != 0 {
		t.Fatalf("a refused run must not run the handler, ran %d times", a)
	}
}

// TestAutoclosureHookNarrows wires narrowAutoclosure the way main does and
// checks the flags reach it through a task's hook.
func TestAutoclosureHookNarrows(t *testing.T) {
	cfg := autoclosure.RunConfig{DailyLimit: 500, States: []string{"solution_proposed", "awaiting_info"}}
	var got autoclosure.RunConfig
	task := registry.Task{Name: "case_autoclosure",
		Handler: func(context.Context) error { return nil },
		RunNowNarrow: func(o registry.RunNowOptions) (func(context.Context) error, []any, error) {
			c, err := narrowAutoclosure(cfg, o)
			got = c
			return func(context.Context) error { return nil }, []any{"dryRun", c.DryRun}, err
		}}
	args := runNowArgs{Task: "case_autoclosure",
		Options: registry.RunNowOptions{DryRun: true, Limit: 20, States: []string{"solution_proposed"}},
		Given:   []string{"--dry-run", "--limit", "--states"}}
	if _, _, err := resolveRunNow([]registry.Task{task}, args); err != nil {
		t.Fatalf("flags must be accepted for case_autoclosure: %v", err)
	}
	if !got.DryRun || got.DailyLimit != 20 || !reflect.DeepEqual(got.States, []string{"solution_proposed"}) {
		t.Fatalf("narrowed config = %+v", got)
	}
	args.Options.Limit = 501
	if _, _, err := resolveRunNow([]registry.Task{task}, args); err == nil {
		t.Fatal("a limit above the configured cap must be refused")
	}
}

func TestNarrowAutoclosure(t *testing.T) {
	base := autoclosure.RunConfig{DailyLimit: 500, States: []string{"solution_proposed", "awaiting_info"}}
	tests := []struct {
		name    string
		cfg     autoclosure.RunConfig
		args    registry.RunNowOptions
		want    autoclosure.RunConfig
		wantErr string
	}{
		{name: "no narrowing keeps env", cfg: base, args: registry.RunNowOptions{}, want: base},
		{name: "dry run forced", cfg: base, args: registry.RunNowOptions{DryRun: true},
			want: autoclosure.RunConfig{DailyLimit: 500, States: base.States, DryRun: true}},
		{name: "env dry run is never loosened", cfg: autoclosure.RunConfig{DailyLimit: 5, DryRun: true}, args: registry.RunNowOptions{},
			want: autoclosure.RunConfig{DailyLimit: 5, DryRun: true}},
		{name: "limit tightens", cfg: base, args: registry.RunNowOptions{Limit: 20},
			want: autoclosure.RunConfig{DailyLimit: 20, States: base.States}},
		{name: "limit equal to env", cfg: base, args: registry.RunNowOptions{Limit: 500},
			want: base},
		{name: "limit above env", cfg: base, args: registry.RunNowOptions{Limit: 501}, wantErr: "exceeds AUTOCLOSURE_DAILY_LIMIT"},
		{name: "env no cap allows any limit", cfg: autoclosure.RunConfig{}, args: registry.RunNowOptions{Limit: 100000},
			want: autoclosure.RunConfig{DailyLimit: 100000}},
		{name: "states subset", cfg: base, args: registry.RunNowOptions{States: []string{"solution_proposed"}},
			want: autoclosure.RunConfig{DailyLimit: 500, States: []string{"solution_proposed"}}},
		{name: "states duplicates collapse", cfg: base, args: registry.RunNowOptions{States: []string{"awaiting_info", "awaiting_info"}},
			want: autoclosure.RunConfig{DailyLimit: 500, States: []string{"awaiting_info"}}},
		{name: "state outside env set", cfg: autoclosure.RunConfig{States: []string{"solution_proposed"}},
			args: registry.RunNowOptions{States: []string{"awaiting_info"}}, wantErr: "not in AUTOCLOSURE_STATES"},
		{name: "empty env states means both", cfg: autoclosure.RunConfig{}, args: registry.RunNowOptions{States: []string{"awaiting_info"}},
			want: autoclosure.RunConfig{States: []string{"awaiting_info"}}},
		{name: "unknown state", cfg: base, args: registry.RunNowOptions{States: []string{"open"}}, wantErr: "not in AUTOCLOSURE_STATES"},
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
	eng := engine.New(withoutTask(tasks, "case_autoclosure"), led, nil, time.Hour, nil, false)
	eng.Tick(context.Background(), time.Now())
	if closureRuns != 0 || otherRuns != 1 {
		t.Fatalf("closure ran %d times, other %d; want 0 and 1", closureRuns, otherRuns)
	}
	if !reflect.DeepEqual(led.attempts, []string{"other"}) {
		t.Fatalf("ledger was asked about %v; the unscheduled task must not be claimed", led.attempts)
	}

	// Scheduled (the default): the task fires as before.
	eng = engine.New(tasks, &fakeLedger{}, nil, time.Hour, nil, false)
	eng.Tick(context.Background(), time.Now())
	if closureRuns != 1 {
		t.Fatalf("scheduled task ran %d times, want 1", closureRuns)
	}
}
