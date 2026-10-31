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

package autoclosure

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
)

// Stage names, as entity-service spells them.
const (
	StageReleaseHold    = "release_hold"
	StageClose          = "close"
	StageSecondReminder = "second_reminder"
	StageFirstReminder  = "first_reminder"
)

// stageOrder is the order one run works through the stages. Holds are released
// first so a ticket whose hold just ended is judged this run. The later stages
// then go furthest-along first: the daily budget is shared, and a ticket
// waiting to be closed matters more than one waiting for its first reminder. No
// ticket moves two stages in one run (each advance changes its step and time,
// so it is no candidate for the next stage until a full interval has passed).
var stageOrder = []string{StageReleaseHold, StageClose, StageSecondReminder, StageFirstReminder}

// RunConfig is what the workflow is run with. None of it is business logic, only
// the knobs: the intervals, the daily limit, which states are switched on and
// the comment wording.
type RunConfig struct {
	// IntervalDays is how long each stage waits. 0 lets entity-service use its
	// default (7).
	IntervalDays int
	// DailyLimit caps reminders and closures per UTC day across all stages. 0
	// means no cap. entity-service enforces it from its audit table, so a run
	// that is retried the same day cannot go past it.
	DailyLimit int
	// States restricts the workflow to awaiting_info and/or solution_proposed.
	// Empty means both. Solution Proposed is meant to run alone for a while
	// before Awaiting Info is switched on.
	States []string
	// Templates overrides the comment wording, keyed "<state>.<kind>".
	Templates map[string]string
	// DryRun makes the whole run decide and log without acting: no comment, no
	// step change, no closure. entity-service records a DRY_RUN audit row per
	// ticket (not counted against the daily budget); the handler spends the
	// daily limit itself as it goes, so the totals are what a real run would do.
	DryRun bool
}

// Validate refuses a configuration entity-service would reject on every call,
// so the task fails at startup (loudly, once) instead of failing and alerting on
// every daily run while processing nothing. The limits are entity-service's own
// (its OpenAPI: intervalDays 1-365, states awaiting_info / solution_proposed).
func (c RunConfig) Validate() error {
	if c.IntervalDays < 0 || c.IntervalDays > 365 {
		return fmt.Errorf("interval days %d is outside 0-365 (0 uses entity-service's default of 7)", c.IntervalDays)
	}
	if c.DailyLimit < 0 || c.DailyLimit > maxDailyLimit {
		return fmt.Errorf("daily limit %d must be between 0 and %d", c.DailyLimit, maxDailyLimit)
	}
	for _, s := range c.States {
		if s != "awaiting_info" && s != "solution_proposed" {
			return fmt.Errorf("state %q is not one of awaiting_info, solution_proposed", s)
		}
	}
	return nil
}

type workClient interface {
	Candidates(ctx context.Context, req CandidatesRequest) ([]Candidate, int, error)
	Advance(ctx context.Context, id string, req AdvanceRequest) (Outcome, error)
}

const (
	// pageSize is how many new tickets each listing pass asks for, on top of
	// the ones already attempted this run. (entity-service clamps a listing to
	// the day's remaining budget, so near the end of the budget a pass can come
	// back with only already-attempted tickets and the loop stops early; the
	// budget is nearly spent by then.)
	pageSize = 500
	// entityListMax is the most entity-service returns for one listing.
	entityListMax = 5000
	// maxPasses bounds the listing loop per stage, so a misbehaving listing can
	// never keep a run alive. The real ceiling is the smaller of this and the
	// listing cap: 20 passes of 500 would be 10,000, but a listing never returns
	// more than entityListMax, so at most 5,000 tickets are attempted per stage.
	maxPasses = 20
	// maxDailyLimit is the largest daily limit entity-service accepts
	// (5,000 x 10); a bigger value would fail every listing, every day.
	maxDailyLimit = entityListMax * 10
)

// maxErrorsReported bounds the failure alert email: a run that fails for
// thousands of tickets must not produce thousands of lines.
const maxErrorsReported = 10

// Run returns a registry.Task.Handler that works through every stage once.
//
// One ticket's failure never stops the others. Each stage's list is asked for
// again after the previous stage's tickets moved, and entity-service takes what
// today has already done off the daily budget, so the handler is safe to run
// again after a failure (the engine retries it): tickets already advanced are no
// longer due, and the limit holds across the retries.
//
// It returns an error when any listing or any ticket failed, so the engine's
// alert names the problem and the next tick retries only what is still due.
func Run(client workClient, cfg RunConfig) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		var (
			errs                                           []error
			evaluated, reminded, closed, released, skipped int
			failed, wouldAct                               int
			// quarantined is the total the stages reported this run: due
			// tickets entity-service left out for failing repeatedly. They cause no
			// error (that would alert on every retry), so the summary is where they show.
			quarantined int
			// spent counts the budget this run has used itself in a dry run (a real
			// run's use is in entity-service's audit table, which it subtracts).
			spent int
		)
	stages:
		for _, stage := range stageOrder {
			// attempted keeps a ticket that was skipped or failed from being tried
			// again by the next pass of this same stage.
			attempted := map[string]bool{}
			// Every pass of a stage repeats that stage's own count, so take the
			// maximum within the stage and add the stages together.
			stageHeld := 0
			for pass := 0; pass < maxPasses; pass++ {
				if err := ctx.Err(); err != nil {
					errs = append(errs, err)
					break stages
				}
				limit := min(len(attempted)+pageSize, entityListMax)
				daily := cfg.DailyLimit
				if cfg.DryRun && stage != StageReleaseHold && daily > 0 {
					if daily -= spent; daily <= 0 {
						break // the budget a real run would have is used up
					}
				}
				candidates, held, err := client.Candidates(ctx, CandidatesRequest{
					Stage: stage, IntervalDays: cfg.IntervalDays, Limit: limit, DailyLimit: daily, States: cfg.States,
				})
				if err != nil {
					failed++
					errs = append(errs, fmt.Errorf("list tickets due for %s: %w", stage, err))
					break
				}
				stageHeld = max(stageHeld, held)
				fresh := 0
				for _, c := range candidates {
					if attempted[c.ID] {
						continue
					}
					attempted[c.ID] = true
					fresh++
					evaluated++
					daily := cfg.DailyLimit
					if cfg.DryRun && stage != StageReleaseHold && daily > 0 {
						if daily -= spent; daily <= 0 {
							skipped++
							continue
						}
					}
					outcome, err := client.Advance(ctx, c.ID, AdvanceRequest{
						Stage: stage, IntervalDays: cfg.IntervalDays, States: cfg.States, Templates: cfg.Templates,
						DailyLimit: daily, DryRun: cfg.DryRun,
					})
					switch {
					case err != nil:
						failed++
						slog.WarnContext(ctx, "autoclosure: stage failed for a ticket", "stage", stage, "ticket", c.Number, "id", c.ID, "err", err)
						errs = append(errs, fmt.Errorf("%s %s: %w", stage, c.Number, err))
					case outcome == OutcomeDryRun:
						wouldAct++
						if stage != StageReleaseHold {
							spent++
						}
						slog.InfoContext(ctx, "autoclosure: dry run, would advance", "stage", stage, "ticket", c.Number, "id", c.ID)
					case outcome != OutcomeAdvanced:
						skipped++
					case stage == StageClose:
						closed++
					case stage == StageReleaseHold:
						released++
					default:
						reminded++
					}
				}
				if fresh == 0 {
					break // nothing new came back: this stage is done
				}
			}
			quarantined += stageHeld
		}

		slog.InfoContext(ctx, "autoclosure: run complete", "dryRun", cfg.DryRun,
			"evaluated", evaluated, "reminded", reminded, "closed", closed,
			"holdsReleased", released, "wouldAdvance", wouldAct, "skipped", skipped, "failed", failed,
			"quarantined", quarantined)
		if quarantined > 0 {
			slog.WarnContext(ctx, "autoclosure: tickets are being left out after failing repeatedly and will not be reminded or closed until someone looks at them",
				"tickets", quarantined, "see", "case_autoclosure_event, outcome FAILED")
		}

		if len(errs) == 0 {
			return nil
		}
		shown := errs
		if len(shown) > maxErrorsReported {
			shown = append(append([]error{}, errs[:maxErrorsReported]...),
				fmt.Errorf("... and %d more failures", len(errs)-maxErrorsReported))
		}
		return errors.Join(shown...)
	}
}
