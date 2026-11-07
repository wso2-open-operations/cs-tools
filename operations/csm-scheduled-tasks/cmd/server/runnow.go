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
	"flag"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/autoclosure"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/eolusagescan"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/incidentautoclose"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/notify"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/registry"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/regressionscan"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/worklogreport"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/worklogsreport"
)

// autoclosureDefaultSchedule is case_autoclosure's default cron: 02:30 UTC, which
// is 08:00 IST, every day including weekends (no day-of-week field).
const autoclosureDefaultSchedule = "30 2 * * *"

// allAutoclosureStates is what an empty AUTOCLOSURE_STATES means.
var allAutoclosureStates = []string{"awaiting_info", "solution_proposed"}

// runNowArgs is the parsed command line of an on-demand run.
type runNowArgs struct {
	// Task is the registered task to run; whether it is registered is checked
	// by resolveRunNow once the tasks are built.
	Task string
	// Options are the narrowing arguments, zero when not given.
	Options registry.RunNowOptions
	// Given names the narrowing flags that were on the command line, in the
	// order they are reported ("--dry-run", "--limit", "--states", "--from-date",
	// "--to-date", "--recipients", "--cc").
	Given []string
}

// parseRunNowArgs parses the process arguments (without the program name). It
// returns nil, nil when there are none, so the component runs its schedule as
// always. Anything unrecognised is an error: a typo must never fall through to
// a scheduled tick or, worse, a real run with wider settings than meant.
func parseRunNowArgs(args []string) (*runNowArgs, error) {
	if len(args) == 0 {
		return nil, nil
	}
	fs := flag.NewFlagSet("csm-scheduled-tasks", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	task := fs.String("run-now", "", "run this registered task once now, then exit")
	dryRun := fs.Bool("dry-run", false, "force a dry run (only with --run-now, only some tasks)")
	limit := fs.Int("limit", 0, "daily limit for this run (only with --run-now, only some tasks)")
	states := fs.String("states", "", "comma list of states to work (only with --run-now, only some tasks)")
	fromDate := fs.String("from-date", "", "first day of the reporting window, YYYY-MM-DD (only with --run-now, only the CS Worklogs reports)")
	toDate := fs.String("to-date", "", "last day of the reporting window, YYYY-MM-DD, inclusive (with --from-date)")
	recipients := fs.String("recipients", "", "comma list of addresses that REPLACE the report's whole audience for this run (only with --run-now, only report tasks)")
	cc := fs.String("cc", "", "comma list of Cc addresses, the only Cc of this run (needs --recipients)")
	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("bad arguments: %w", err)
	}
	if fs.NArg() > 0 {
		return nil, fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}
	seen := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { seen[f.Name] = true })

	if !seen["run-now"] {
		return nil, errors.New("--dry-run, --limit, --states, --from-date, --to-date, --recipients and --cc are only valid together with --run-now")
	}
	name := strings.TrimSpace(*task)
	if name == "" {
		return nil, errors.New("--run-now needs a task name, for example --run-now=case_autoclosure")
	}
	out := &runNowArgs{Task: name}
	if seen["dry-run"] {
		out.Options.DryRun = *dryRun
		out.Given = append(out.Given, "--dry-run")
	}
	if seen["limit"] {
		if *limit <= 0 {
			return nil, fmt.Errorf("--limit=%d must be a positive number", *limit)
		}
		out.Options.Limit = *limit
		out.Given = append(out.Given, "--limit")
	}
	if seen["states"] {
		out.Options.States = splitComma(*states)
		if len(out.Options.States) == 0 {
			return nil, errors.New("--states is empty; give a comma list such as solution_proposed")
		}
		out.Given = append(out.Given, "--states")
	}
	if seen["from-date"] || seen["to-date"] {
		if !seen["from-date"] || !seen["to-date"] {
			return nil, errors.New("--from-date and --to-date must be given together")
		}
		from, err := worklogsreport.ParseDate("--from-date", *fromDate)
		if err != nil {
			return nil, err
		}
		to, err := worklogsreport.ParseDate("--to-date", *toDate)
		if err != nil {
			return nil, err
		}
		if err := worklogsreport.ValidateWindow(from, to); err != nil {
			return nil, err
		}
		out.Options.FromDate, out.Options.ToDate = from, to
		out.Given = append(out.Given, "--from-date", "--to-date")
	}
	if seen["cc"] && !seen["recipients"] {
		return nil, errors.New("--cc needs --recipients: it only adds copies to a replaced audience")
	}
	if seen["recipients"] {
		list, err := worklogsreport.ParseAddressList("--recipients", *recipients)
		if err != nil {
			return nil, err
		}
		out.Options.Recipients = list
		out.Given = append(out.Given, "--recipients")
	}
	if seen["cc"] {
		list, err := worklogsreport.ParseAddressList("--cc", *cc)
		if err != nil {
			return nil, err
		}
		out.Options.Cc = list
		out.Given = append(out.Given, "--cc")
	}
	return out, nil
}

// checkRunNow refuses an on-demand run combined with TRYOUT_TASK (two different
// "run once" modes at once).
func checkRunNow(a *runNowArgs, tryoutTask string) error {
	if strings.TrimSpace(tryoutTask) != "" {
		return fmt.Errorf("--run-now=%s cannot be combined with TRYOUT_TASK; refusing", a.Task)
	}
	return nil
}

// resolveRunNow finds the task an on-demand run names among the registered ones
// and returns the handler to run and the settings to log. A name that is not
// registered is refused, listing the registered names; usually the task's own
// enable flag is off. The narrowing flags need the task's RunNowNarrow hook; a
// task without one refuses them rather than ignoring them.
func resolveRunNow(tasks []registry.Task, a runNowArgs) (func(ctx context.Context) error, []any, error) {
	var names []string
	var task *registry.Task
	for i := range tasks {
		names = append(names, tasks[i].Name)
		if tasks[i].Name == a.Task {
			task = &tasks[i]
		}
	}
	if task == nil {
		slices.Sort(names)
		return nil, nil, fmt.Errorf("--run-now=%q is not a registered task: either the name is wrong or the task is not registered because its enable flag is off; registered: %s",
			a.Task, strings.Join(names, ", "))
	}
	for _, g := range a.Given {
		if task.RunNowNarrow == nil || !slices.Contains(task.RunNowFlags, g) {
			return nil, nil, fmt.Errorf("%s is not supported by task %s", g, task.Name)
		}
	}
	if task.RunNowNarrow == nil {
		return task.Handler, nil, nil
	}
	handler, settings, err := task.RunNowNarrow(a.Options)
	if err != nil {
		return nil, nil, err
	}
	return handler, settings, nil
}

// narrowAutoclosure applies an on-demand run's arguments to the configured
// settings. They can only tighten: a dry run stays a dry run, the limit may not
// exceed a configured cap, and the states must be among the configured ones.
func narrowAutoclosure(cfg autoclosure.RunConfig, a registry.RunNowOptions) (autoclosure.RunConfig, error) {
	if a.DryRun {
		cfg.DryRun = true
	}
	if a.Limit > 0 {
		if cfg.DailyLimit != 0 && a.Limit > cfg.DailyLimit {
			return cfg, fmt.Errorf("--limit=%d exceeds AUTOCLOSURE_DAILY_LIMIT=%d; a run-now can only tighten the configured limit", a.Limit, cfg.DailyLimit)
		}
		cfg.DailyLimit = a.Limit
	}
	if a.States != nil {
		allowed := cfg.States
		if len(allowed) == 0 {
			allowed = allAutoclosureStates
		}
		var chosen []string
		for _, s := range a.States {
			if !slices.Contains(allowed, s) {
				return cfg, fmt.Errorf("--states value %q is not in AUTOCLOSURE_STATES (%s); a run-now can only narrow the configured states", s, strings.Join(allowed, ","))
			}
			if !slices.Contains(chosen, s) {
				chosen = append(chosen, s)
			}
		}
		cfg.States = chosen
	}
	return cfg, nil
}

// narrowIncidentAutoclose is narrowAutoclosure for incident_autoclose, which has
// no states: a dry run stays a dry run, and the limit may not exceed a
// configured daily limit (when none is configured it sets one for the run).
func narrowIncidentAutoclose(cfg incidentautoclose.Config, a registry.RunNowOptions) (incidentautoclose.Config, error) {
	if a.DryRun {
		cfg.DryRun = true
	}
	if a.Limit > 0 {
		if cfg.DailyLimit != 0 && a.Limit > cfg.DailyLimit {
			return cfg, fmt.Errorf("--limit=%d exceeds INCIDENT_AUTOCLOSE_DAILY_LIMIT=%d; a run-now can only tighten the configured limit", a.Limit, cfg.DailyLimit)
		}
		if a.Limit > incidentautoclose.MaxDailyLimit {
			return cfg, fmt.Errorf("--limit=%d exceeds the largest daily limit, %d", a.Limit, incidentautoclose.MaxDailyLimit)
		}
		cfg.DailyLimit = a.Limit
	}
	return cfg, nil
}

// describeStates is the states as logged: empty means both.
func describeStates(states []string) string {
	if len(states) == 0 {
		return strings.Join(allAutoclosureStates, ",")
	}
	return strings.Join(states, ",")
}

// reportRunNowFlags are the run-now flags every report task takes.
var reportRunNowFlags = []string{"--recipients", "--cc"}

// worklogsRunNowFlags are the run-now flags the CS Worklogs report tasks take:
// the window flags as well.
var worklogsRunNowFlags = []string{"--from-date", "--to-date", "--recipients", "--cc"}

// orRecipients is the To a report handler is built with in a recipient
// override run: the task's real To when it has one, so the test-copy notice
// can name it, and the override list otherwise, so a task that skips or fails
// on an empty audience still sends. The mail itself only ever goes to the
// override, whatever this returns (see notify.ForRunNow).
func orRecipients(realTo, recipients []string) []string {
	if len(realTo) == 0 {
		return recipients
	}
	return realTo
}

// reportRunNow is the RunNowNarrow hook of a report task that takes only
// --recipients and --cc, and the one place that decides what an override run
// is. build makes the task's handler for a sender and, when the run
// overrides the audience, the override list (nil otherwise). normal is the
// sender the scheduled handler uses (debug mail mode applied); raw is the
// real mail client. With no --recipients the handler is built with normal and
// nil, so a plain run-now behaves as today, debug list included. With
// --recipients it is built with notify.ForRunNow over raw: the mail goes to
// those addresses only (Cc is the --cc list or nothing), as a [Test copy]
// whose notice names the real To and Cc, and the task's debug list is
// ignored, because an explicit list wins over it.
func reportRunNow(normal, raw notify.Sender, build func(email notify.Sender, recipients []string) func(ctx context.Context) error,
) func(o registry.RunNowOptions) (func(ctx context.Context) error, []any, error) {
	return func(o registry.RunNowOptions) (func(ctx context.Context) error, []any, error) {
		if len(o.Recipients) == 0 {
			if len(o.Cc) > 0 {
				return nil, nil, errors.New("--cc needs --recipients: it only adds copies to a replaced audience")
			}
			return build(normal, nil), nil, nil
		}
		return build(notify.ForRunNow(raw, o.Recipients, o.Cc), o.Recipients), recipientSettings(o), nil
	}
}

// recipientSettings is what the run-now WARN logs about a replaced audience:
// counts only, like every other recipient log line here.
func recipientSettings(o registry.RunNowOptions) []any {
	return []any{"recipientsReplaced", true, "toCount", len(o.Recipients), "ccCount", len(o.Cc)}
}

// consumesCrossings wraps a run-now hook for a task that stores the state it
// compares against (the query-hour notifications). A --recipients run of it
// mails the crossings to the override list only, and the stored state moves on,
// so the real owners never get those notices. The run-now WARN names that
// before the operator can miss it.
func consumesCrossings(hook func(o registry.RunNowOptions) (func(ctx context.Context) error, []any, error),
) func(o registry.RunNowOptions) (func(ctx context.Context) error, []any, error) {
	return func(o registry.RunNowOptions) (func(ctx context.Context) error, []any, error) {
		handler, settings, err := hook(o)
		if err == nil && len(o.Recipients) > 0 {
			settings = append(settings, "warning", "crossings found by this run are mailed to the override list only and are not sent to the real owners later")
		}
		return handler, settings, err
	}
}

// worklogsRunNow is reportRunNow for a CS Worklogs report task, which also
// takes a reporting window. build makes the handler for a sender, a validated
// window (the zero Override for none) and the override list (nil for none).
// With only a window the task's normal sender is used, so a window run still
// honours the debug list; with --recipients the override sender is used and
// the debug list is ignored. With nothing the task's own handler runs.
func worklogsRunNow(cadence worklogreport.Cadence, normal, raw notify.Sender,
	build func(email notify.Sender, win worklogsreport.Override, recipients []string) func(ctx context.Context) error,
) func(o registry.RunNowOptions) (func(ctx context.Context) error, []any, error) {
	return func(o registry.RunNowOptions) (func(ctx context.Context) error, []any, error) {
		win := worklogsreport.Override{From: o.FromDate, To: o.ToDate}
		// Cc alone is caught by Validate below through the Override's own rule.
		if err := (worklogsreport.Override{From: o.FromDate, To: o.ToDate, Recipients: o.Recipients, Cc: o.Cc}).Validate(cadence); err != nil {
			return nil, nil, err
		}
		var settings []any
		if win.HasWindow() {
			settings = append(settings, "from", win.From.Format("2006-01-02"), "to", win.To.Format("2006-01-02"))
		}
		email := normal
		if len(o.Recipients) > 0 {
			email = notify.ForRunNow(raw, o.Recipients, o.Cc)
			settings = append(settings, recipientSettings(o)...)
		}
		return build(email, win, o.Recipients), settings, nil
	}
}

// reportTaskNames are the report tasks that mail an audience and so take
// --recipients and --cc. The failure alerts to ALERT_RECIPIENTS, the rota
// reminders (which mail the rota's people, not a report audience) and the
// tasks that send no report are not in it.
func reportTaskNames() []string {
	names := []string{
		"cs_worklogs_daily_report", "cs_worklogs_weekly_report", "migration_case_report",
		"open_cases_report", "stale_cases_report", "unresolved_products_digest",
		"query_hours_usage_notifications", "query_hours_weekly_report",
		eolusagescan.TaskName,
	}
	for _, v := range regressionscan.DefaultVariants() {
		names = append(names, regressionscan.TaskName(v))
	}
	return names
}

// tasksMissingReportRunNow names the tasks among want that are not registered
// or do not list both --recipients and --cc, so a report task added without
// the override is caught at startup instead of being quietly left out.
func tasksMissingReportRunNow(tasks []registry.Task, want []string) []string {
	var missing []string
	for _, name := range want {
		ok := false
		for _, t := range tasks {
			if t.Name == name && t.RunNowNarrow != nil &&
				slices.Contains(t.RunNowFlags, "--recipients") && slices.Contains(t.RunNowFlags, "--cc") {
				ok = true
			}
		}
		if !ok {
			missing = append(missing, name)
		}
	}
	return missing
}

// withoutTask returns tasks without the one named name: the engine's list when
// AUTOCLOSURE_SCHEDULED=false keeps case_autoclosure off the schedule.
func withoutTask(tasks []registry.Task, name string) []registry.Task {
	out := make([]registry.Task, 0, len(tasks))
	for _, t := range tasks {
		if t.Name != name {
			out = append(out, t)
		}
	}
	return out
}

// parseStrictBool is for settings that must refuse to start on a bad value
// rather than fall back to a default (envBool warns and uses the default).
// Unset or empty is def.
func parseStrictBool(key, raw string, def bool) (bool, error) {
	if strings.TrimSpace(raw) == "" {
		return def, nil
	}
	b, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		return def, fmt.Errorf("%s=%q is not a valid boolean", key, raw)
	}
	return b, nil
}
