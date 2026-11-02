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
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/registry"
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
	recipients := fs.String("recipients", "", "comma list of addresses that REPLACE the report's To audience for this run (only with --run-now, only the CS Worklogs reports)")
	cc := fs.String("cc", "", "comma list of Cc addresses for the replaced audience (needs --recipients)")
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

// describeStates is the states as logged: empty means both.
func describeStates(states []string) string {
	if len(states) == 0 {
		return strings.Join(allAutoclosureStates, ",")
	}
	return strings.Join(states, ",")
}

// worklogsRunNow is the RunNowNarrow hook of a CS Worklogs report task. build
// makes the handler for a validated override. debug is the task's debug list
// from SUB_CRON_RECIPIENTS: with one set every mail is redirected to it, so a
// recipient override could never take effect and is refused rather than
// silently ignored. With no override at all the task's own handler runs, so a
// plain run-now is unchanged.
func worklogsRunNow(cadence worklogreport.Cadence, debug []string, plain func(ctx context.Context) error,
	build func(ov worklogsreport.Override) func(ctx context.Context) error,
) func(o registry.RunNowOptions) (func(ctx context.Context) error, []any, error) {
	return func(o registry.RunNowOptions) (func(ctx context.Context) error, []any, error) {
		ov := worklogsreport.Override{From: o.FromDate, To: o.ToDate, Recipients: o.Recipients, Cc: o.Cc}
		if !ov.Active() {
			return plain, nil, nil
		}
		if err := ov.Validate(cadence); err != nil {
			return nil, nil, err
		}
		if ov.HasRecipients() && len(debug) > 0 {
			return nil, nil, errors.New("--recipients cannot be used while this task has a debug list in SUB_CRON_RECIPIENTS: debug mail mode would redirect the mail and ignore the override; remove the debug list first")
		}
		var settings []any
		if ov.HasWindow() {
			settings = append(settings, "from", ov.From.Format("2006-01-02"), "to", ov.To.Format("2006-01-02"))
		}
		if ov.HasRecipients() {
			// Counts only, like every other recipient log line here.
			settings = append(settings, "recipientsReplaced", true, "toCount", len(ov.Recipients), "ccCount", len(ov.Cc))
		}
		return build(ov), settings, nil
	}
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

// worklogsRunNowFlags are the run-now flags the CS Worklogs report tasks take.
var worklogsRunNowFlags = []string{"--from-date", "--to-date", "--recipients", "--cc"}
