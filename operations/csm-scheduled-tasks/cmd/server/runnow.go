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

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/autocaseclosure"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/registry"
)

// allAutoCaseClosureStates is what an empty AUTO_CASE_CLOSURE_STATES means.
var allAutoCaseClosureStates = []string{"awaiting_info", "solution_proposed"}

// runNowArgs is the parsed command line of an on-demand run.
type runNowArgs struct {
	// Task is the registered task to run; whether it is registered is checked
	// by resolveRunNow once the tasks are built.
	Task string
	// Options are the narrowing arguments, zero when not given.
	Options registry.RunNowOptions
	// Given names the narrowing flags that were on the command line, in the
	// order they are reported ("--dry-run", "--limit", "--states").
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
	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("bad arguments: %w", err)
	}
	if fs.NArg() > 0 {
		return nil, fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}
	seen := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { seen[f.Name] = true })

	if !seen["run-now"] {
		return nil, errors.New("--dry-run, --limit and --states are only valid together with --run-now")
	}
	name := strings.TrimSpace(*task)
	if name == "" {
		return nil, errors.New("--run-now needs a task name, for example --run-now=auto_case_closure")
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
	if task.RunNowNarrow == nil {
		if len(a.Given) > 0 {
			return nil, nil, fmt.Errorf("%s is not supported by task %s", a.Given[0], task.Name)
		}
		return task.Handler, nil, nil
	}
	handler, settings, err := task.RunNowNarrow(a.Options)
	if err != nil {
		return nil, nil, err
	}
	return handler, settings, nil
}

// narrowAutoCaseClosure applies an on-demand run's arguments to the configured
// settings. They can only tighten: a dry run stays a dry run, the limit may not
// exceed a configured cap, and the states must be among the configured ones.
func narrowAutoCaseClosure(cfg autocaseclosure.RunConfig, a registry.RunNowOptions) (autocaseclosure.RunConfig, error) {
	if a.DryRun {
		cfg.DryRun = true
	}
	if a.Limit > 0 {
		if cfg.DailyLimit != 0 && a.Limit > cfg.DailyLimit {
			return cfg, fmt.Errorf("--limit=%d exceeds AUTO_CASE_CLOSURE_DAILY_LIMIT=%d; a run-now can only tighten the configured limit", a.Limit, cfg.DailyLimit)
		}
		cfg.DailyLimit = a.Limit
	}
	if a.States != nil {
		allowed := cfg.States
		if len(allowed) == 0 {
			allowed = allAutoCaseClosureStates
		}
		var chosen []string
		for _, s := range a.States {
			if !slices.Contains(allowed, s) {
				return cfg, fmt.Errorf("--states value %q is not in AUTO_CASE_CLOSURE_STATES (%s); a run-now can only narrow the configured states", s, strings.Join(allowed, ","))
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
		return strings.Join(allAutoCaseClosureStates, ",")
	}
	return strings.Join(states, ",")
}

// withoutTask returns tasks without the one named name: the engine's list when
// AUTO_CASE_CLOSURE_SCHEDULED=false keeps auto_case_closure off the schedule.
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
