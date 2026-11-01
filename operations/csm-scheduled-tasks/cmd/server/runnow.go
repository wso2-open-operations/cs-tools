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
	"errors"
	"flag"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/autoclosure"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/registry"
)

// runNowTasks are the tasks --run-now accepts. Only case auto-closure: it is the
// one task an operator needs to start by hand with narrower settings than its
// schedule runs with. The report tasks have TRYOUT_TASK.
var runNowTasks = []string{"case_autoclosure"}

// allAutoclosureStates is what an empty AUTOCLOSURE_STATES means.
var allAutoclosureStates = []string{"awaiting_info", "solution_proposed"}

// runNowArgs is the parsed command line of an on-demand run.
type runNowArgs struct {
	Task string
	// DryRun forces a dry run; false leaves the configured value alone.
	DryRun bool
	// Limit is this invocation's daily limit; 0 means "not given" (a given
	// limit must be positive).
	Limit int
	// States is the subset of AUTOCLOSURE_STATES to work; nil means "not given".
	States []string
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
	task := fs.String("run-now", "", "run this task once now, then exit")
	dryRun := fs.Bool("dry-run", false, "force a dry run (only with --run-now)")
	limit := fs.Int("limit", 0, "daily limit for this run (only with --run-now)")
	states := fs.String("states", "", "comma list of states to work (only with --run-now)")
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
	if !slices.Contains(runNowTasks, name) {
		return nil, fmt.Errorf("--run-now=%q is not a task that can be run on demand; supported: %s", name, strings.Join(runNowTasks, ", "))
	}
	out := &runNowArgs{Task: name, DryRun: *dryRun}
	if seen["limit"] {
		if *limit <= 0 {
			return nil, fmt.Errorf("--limit=%d must be a positive number", *limit)
		}
		out.Limit = *limit
	}
	if seen["states"] {
		out.States = splitComma(*states)
		if len(out.States) == 0 {
			return nil, errors.New("--states is empty; give a comma list such as solution_proposed")
		}
	}
	return out, nil
}

// checkRunNow refuses an on-demand run that cannot go ahead: the task is off,
// or TRYOUT_TASK is also set (two different "run once" modes at once).
func checkRunNow(a *runNowArgs, autoclosureEnabled bool, tryoutTask string) error {
	if !autoclosureEnabled {
		return fmt.Errorf("--run-now=%s requires AUTOCLOSURE_ENABLED=true; refusing", a.Task)
	}
	if strings.TrimSpace(tryoutTask) != "" {
		return errors.New("--run-now cannot be combined with TRYOUT_TASK; refusing")
	}
	return nil
}

// narrowAutoclosure applies an on-demand run's arguments to the configured
// settings. They can only tighten: a dry run stays a dry run, the limit may not
// exceed a configured cap, and the states must be among the configured ones.
func narrowAutoclosure(cfg autoclosure.RunConfig, a runNowArgs) (autoclosure.RunConfig, error) {
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

// withoutUnscheduled returns the tasks the engine's tick may fire: every task
// except those named in unscheduled. An unscheduled task stays registered (so
// TRYOUT_TASK and --run-now still find it) but is never started by the schedule.
func withoutUnscheduled(tasks []registry.Task, unscheduled map[string]bool) []registry.Task {
	if len(unscheduled) == 0 {
		return tasks
	}
	out := make([]registry.Task, 0, len(tasks))
	for _, t := range tasks {
		if !unscheduled[t.Name] {
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
