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
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/autoclosure"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/engine"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/entitycases"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/ledger"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/longopenreport"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/migrationreport"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/migrationscan"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/notify"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/opencases"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/queryhours"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/queryhoursreport"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/registry"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/schedule"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/stalecases"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/worklogreport"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/worklogsreport"
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
		{name: "recipients and cc parse",
			args: []string{"--run-now=stale_cases_report", "--recipients=jane.doe@example.com", "--cc=john.roe@example.com"},
			want: &runNowArgs{Task: "stale_cases_report",
				Options: registry.RunNowOptions{Recipients: []string{"jane.doe@example.com"}, Cc: []string{"john.roe@example.com"}},
				Given:   []string{"--recipients", "--cc"}}},
		{name: "bad recipient", args: []string{"--run-now=stale_cases_report", "--recipients=bad"}, wantErr: "not a plain email address"},
		{name: "cc without recipients", args: []string{"--run-now=stale_cases_report", "--cc=john.roe@example.com"}, wantErr: "--cc needs --recipients"},
		{name: "recipients without run-now", args: []string{"--recipients=jane.doe@example.com"}, wantErr: "only valid together with --run-now"},
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
			RunNowFlags: []string{"--dry-run", "--limit", "--states"},
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
		Handler:     func(context.Context) error { return nil },
		RunNowFlags: []string{"--dry-run", "--limit", "--states"},
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

type recordingSender struct {
	calls         int
	to, cc        []string
	subject, body string
}

func (r *recordingSender) SendEmail(_ context.Context, to, cc []string, subject, body string) error {
	r.calls++
	r.to, r.cc, r.subject, r.body = to, cc, subject, body
	return nil
}

// bodyOf is a minimal report body, so the notice goes just inside it.
const bodyOf = "<html><body><p>report</p></body></html>"

// reportFixture builds the way main does: a normal sender in debug mail mode
// and the raw one, and a handler that mails a fixed report to the To and Cc it
// is built with.
func reportFixture(realTo, realCc []string, debug []string) (hook func(registry.RunNowOptions) (func(context.Context) error, []any, error), raw, debugInbox *recordingSender) {
	raw = &recordingSender{}
	normal := notify.ForReport(raw, debug)
	hook = reportRunNow(normal, raw, func(email notify.Sender, rec []string) func(context.Context) error {
		return func(ctx context.Context) error {
			return email.SendEmail(ctx, orRecipients(realTo, rec), realCc, "Weekly", bodyOf)
		}
	})
	return hook, raw, raw
}

func TestReportRunNowOverrideReplacesWholeAudience(t *testing.T) {
	ctx := context.Background()
	real, realCc := []string{"real@example.com"}, []string{"realcc@example.com"}

	// Recipients only: no Cc at all, real To and Cc only in the banner.
	hook, raw, _ := reportFixture(real, realCc, nil)
	h, settings, err := hook(registry.RunNowOptions{Recipients: []string{"jane.doe@example.com"}})
	if err != nil || h(ctx) != nil {
		t.Fatalf("err=%v", err)
	}
	if !reflect.DeepEqual(raw.to, []string{"jane.doe@example.com"}) || len(raw.cc) != 0 {
		t.Errorf("to=%v cc=%v, want only the given To and no Cc", raw.to, raw.cc)
	}
	if raw.subject != "[Test copy] Weekly" {
		t.Errorf("subject = %q", raw.subject)
	}
	for _, want := range []string{"real@example.com", "realcc@example.com"} {
		if !strings.Contains(raw.body, want) {
			t.Errorf("banner lacks %s", want)
		}
	}
	if !reflect.DeepEqual(settings, []any{"recipientsReplaced", true, "toCount", 1, "ccCount", 0}) {
		t.Errorf("settings = %v", settings)
	}

	// With --cc: that Cc only.
	h, _, _ = hook(registry.RunNowOptions{Recipients: []string{"jane.doe@example.com"}, Cc: []string{"john.roe@example.com"}})
	_ = h(ctx)
	if !reflect.DeepEqual(raw.cc, []string{"john.roe@example.com"}) {
		t.Errorf("cc = %v, want only the --cc list", raw.cc)
	}

	// A task with no audience of its own still sends in an override run.
	hook, raw, _ = reportFixture(nil, nil, nil)
	h, _, _ = hook(registry.RunNowOptions{Recipients: []string{"jane.doe@example.com"}})
	if err := h(ctx); err != nil || raw.calls != 1 || !reflect.DeepEqual(raw.to, []string{"jane.doe@example.com"}) {
		t.Errorf("empty-audience task: err=%v calls=%d to=%v", err, raw.calls, raw.to)
	}
}

func TestReportRunNowRecipientsWinOverDebugList(t *testing.T) {
	ctx := context.Background()
	hook, raw, _ := reportFixture([]string{"real@example.com"}, nil, []string{"debug@example.com"})

	// With --recipients the debug inbox is not addressed.
	h, _, err := hook(registry.RunNowOptions{Recipients: []string{"jane.doe@example.com"}})
	if err != nil {
		t.Fatalf("a debug list must no longer refuse --recipients: %v", err)
	}
	_ = h(ctx)
	if !reflect.DeepEqual(raw.to, []string{"jane.doe@example.com"}) || strings.Contains(raw.body, "Debug mail mode") {
		t.Errorf("to=%v, the override must win over the debug list", raw.to)
	}

	// Without it, a run-now still honours the debug list, as today.
	h, settings, err := hook(registry.RunNowOptions{})
	if err != nil || len(settings) != 0 {
		t.Fatalf("plain: err=%v settings=%v", err, settings)
	}
	_ = h(ctx)
	if !reflect.DeepEqual(raw.to, []string{"debug@example.com"}) || raw.subject != "[Test copy] Weekly" {
		t.Errorf("plain run-now to=%v subject=%q, want the debug inbox", raw.to, raw.subject)
	}

	// --cc alone is refused by the hook as well as the parser.
	if _, _, err := hook(registry.RunNowOptions{Cc: []string{"john.roe@example.com"}}); err == nil ||
		!strings.Contains(err.Error(), "--cc needs --recipients") {
		t.Errorf("--cc alone must be refused, got %v", err)
	}
}

func TestReportTasksTakeRecipientFlags(t *testing.T) {
	want := []string{
		"cs_worklogs_daily_report", "cs_worklogs_weekly_report", "migration_case_report",
		"open_cases_report", "stale_cases_report", "unresolved_products_digest",
		"query_hours_usage_notifications", "query_hours_weekly_report",
		"product_regression_report_apim", "product_regression_report_iam", "product_regression_report_integration",
	}
	got := reportTaskNames()
	slices.Sort(got)
	slices.Sort(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("reportTaskNames = %v, want %v", got, want)
	}
	hook := func(registry.RunNowOptions) (func(context.Context) error, []any, error) { return nil, nil, nil }
	var tasks []registry.Task
	for _, n := range want {
		flags := reportRunNowFlags
		if strings.HasPrefix(n, "cs_worklogs") {
			flags = worklogsRunNowFlags
		}
		tasks = append(tasks, registry.Task{Name: n, RunNowNarrow: hook, RunNowFlags: flags})
	}
	if m := tasksMissingReportRunNow(tasks, want); len(m) != 0 {
		t.Fatalf("missing: %v", m)
	}
	// A task without the hook, without a flag, or unregistered is reported.
	tasks[0].RunNowNarrow = nil
	tasks[1].RunNowFlags = []string{"--recipients"}
	tasks = tasks[:len(tasks)-1]
	if m := tasksMissingReportRunNow(tasks, want); len(m) != 3 {
		t.Fatalf("missing = %v, want 3", m)
	}
}

func TestWorklogsRunNowRecipientsAndWindow(t *testing.T) {
	ctx := context.Background()
	raw := &recordingSender{}
	normal := notify.ForReport(raw, []string{"debug@example.com"})
	var gotWin worklogsreport.Override
	var gotRec []string
	hook := worklogsRunNow(worklogreport.CadenceWeekly, normal, raw,
		func(email notify.Sender, win worklogsreport.Override, rec []string) func(context.Context) error {
			return func(ctx context.Context) error {
				gotWin, gotRec = win, rec
				return email.SendEmail(ctx, orRecipients([]string{"real@example.com"}, rec), nil, "Worklogs", bodyOf)
			}
		})

	// Recipients win over the debug list; no Cc.
	h, _, err := hook(registry.RunNowOptions{Recipients: []string{"jane.doe@example.com"}})
	if err != nil || h(ctx) != nil {
		t.Fatalf("err=%v", err)
	}
	if !reflect.DeepEqual(raw.to, []string{"jane.doe@example.com"}) || len(raw.cc) != 0 || raw.subject != "[Test copy] Worklogs" || gotWin.HasWindow() {
		t.Errorf("to=%v cc=%v subject=%q win=%v", raw.to, raw.cc, raw.subject, gotWin)
	}
	// A window alone still goes to the debug list.
	day := time.Date(2030, 1, 7, 0, 0, 0, 0, time.UTC)
	h, settings, err := hook(registry.RunNowOptions{FromDate: day, ToDate: day.AddDate(0, 0, 6)})
	if err != nil || h(ctx) != nil || !reflect.DeepEqual(raw.to, []string{"debug@example.com"}) || !gotWin.HasWindow() || gotRec != nil {
		t.Fatalf("window only: err=%v to=%v rec=%v", err, raw.to, gotRec)
	}
	if len(settings) != 4 {
		t.Errorf("settings = %v", settings)
	}
	// Cc alone and a bad window are refused.
	if _, _, err := hook(registry.RunNowOptions{Cc: []string{"john.roe@example.com"}}); err == nil {
		t.Error("--cc alone must be refused")
	}
	if _, _, err := hook(registry.RunNowOptions{FromDate: day, ToDate: day.AddDate(0, 0, -1)}); err == nil {
		t.Error("an inverted window must be refused")
	}
}

// The real handlers, built the way main builds them for an override run.
func TestOverrideRunsOfRealReportHandlers(t *testing.T) {
	ctx := context.Background()
	rec := []string{"jane.doe@example.com"}
	cases := map[string]func(email notify.Sender) func(context.Context) error{
		"stale": func(email notify.Sender) func(context.Context) error {
			return stalecases.SendReport(fakeStale{}, email, 30, orRecipients(nil, rec), nil, true)
		},
		"open": func(email notify.Sender) func(context.Context) error {
			return opencases.SendReport(fakeOpen{}, email, orRecipients(nil, rec), nil, true)
		},
		"migration": func(email notify.Sender) func(context.Context) error {
			return migrationscan.SendReport(fakeMigration{}, email, 180, orRecipients(nil, rec), nil, "", true)
		},
		"queryhours weekly": func(email notify.Sender) func(context.Context) error {
			return queryhoursreport.SendWeeklyReport(fakeQueryHours{}, email, orRecipients(nil, rec), nil, nil, true)
		},
	}
	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			raw := &recordingSender{}
			// The task has debug mode on and no audience of its own.
			hook := reportRunNow(notify.ForReport(raw, []string{"debug@example.com"}), raw,
				func(email notify.Sender, _ []string) func(context.Context) error { return build(email) })
			h, _, err := hook(registry.RunNowOptions{Recipients: rec})
			if err != nil {
				t.Fatal(err)
			}
			if err := h(ctx); err != nil {
				t.Fatal(err)
			}
			if raw.calls != 1 || !reflect.DeepEqual(raw.to, rec) || len(raw.cc) != 0 || !strings.HasPrefix(raw.subject, "[Test copy] ") {
				t.Errorf("calls=%d to=%v cc=%v subject=%q", raw.calls, raw.to, raw.cc, raw.subject)
			}
		})
	}
}

type fakeStale struct{}

// fakeStale returns one case so the report is not empty (an empty report is
// not sent at all).
func (fakeStale) Report(context.Context, int) (longopenreport.Report, error) {
	return longopenreport.Report{
		Cases: []longopenreport.Row{{Number: "CS0000001"}}, CasesTotal: 1,
	}, nil
}

type fakeOpen struct{}

func (fakeOpen) SearchCasesInStateCreatedBeforeYesterday(context.Context, string) ([]entitycases.Case, error) {
	return nil, nil
}

type fakeMigration struct{}

func (fakeMigration) Report(context.Context, int) (migrationreport.Report, error) {
	return migrationreport.Report{}, nil
}

type fakeQueryHours struct{}

func (fakeQueryHours) WeeklyReport(context.Context) (queryhours.Report, error) {
	return queryhours.Report{}, nil
}
func (fakeQueryHours) Recompute(context.Context) ([]queryhours.Transition, error) { return nil, nil }

func TestConsumesCrossingsWarnsOnlyWithRecipients(t *testing.T) {
	hook := consumesCrossings(func(o registry.RunNowOptions) (func(context.Context) error, []any, error) {
		return func(context.Context) error { return nil }, []any{"base", true}, nil
	})
	_, plain, err := hook(registry.RunNowOptions{})
	if err != nil || len(plain) != 2 {
		t.Fatalf("plain run: settings=%v err=%v, want the base settings only", plain, err)
	}
	_, withRec, err := hook(registry.RunNowOptions{Recipients: []string{"jane.doe@example.com"}})
	if err != nil {
		t.Fatalf("recipients run: %v", err)
	}
	if len(withRec) != 4 || withRec[2] != "warning" {
		t.Fatalf("recipients run settings = %v, want a trailing warning key", withRec)
	}
}
