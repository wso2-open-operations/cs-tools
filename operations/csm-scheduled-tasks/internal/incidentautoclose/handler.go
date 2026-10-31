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

package incidentautoclose

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
)

const (
	// DefaultDays is ServiceNow's threshold: 399 of 400 incidents it auto-closed
	// were closed 336.1-336.8 hours (14 days) after resolved_at.
	DefaultDays = 14
	// PageSize is how many new incidents one call asks entity-service to close
	// (its own default), on top of the ones that already failed this run. It
	// is kept small enough to finish well inside entity-service's 60s
	// REQUEST_TIMEOUT (see callTimeout).
	PageSize = 200
	// maxLimit is the largest limit entity-service accepts.
	maxLimit = 1000
	// MaxPages bounds one run: 20 pages of 200 is 4,000 incidents an hour,
	// far above the steady-state rate. Hitting it is logged; the rest are
	// closed by the next hourly run.
	MaxPages = 20
	// maxFailuresReported bounds the failure alert, as in internal/autoclosure.
	maxFailuresReported = 10
	// maxErrorLen bounds each failure's text in the alert.
	maxErrorLen = 200
)

// Config is what the task runs with.
type Config struct {
	// Days is how long an incident must have been Resolved before it is closed.
	Days int
	// DryRun asks entity-service which incidents are due without closing any.
	DryRun bool
}

// ConfigFromEnv reads INCIDENT_AUTOCLOSE_DAYS and INCIDENT_AUTOCLOSE_DRY_RUN
// through getenv (os.Getenv in main). Unlike the tuning knobs main reads with a
// logged fallback, a malformed value here is an error so the process refuses to
// start: falling back to the default would turn a mistyped dry run into a real
// one, or a mistyped threshold into closing incidents on a different day.
func ConfigFromEnv(getenv func(string) string) (Config, error) {
	cfg := Config{Days: DefaultDays}
	if v := strings.TrimSpace(getenv("INCIDENT_AUTOCLOSE_DAYS")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return Config{}, fmt.Errorf("INCIDENT_AUTOCLOSE_DAYS %q must be a whole number of days, at least 1", v)
		}
		cfg.Days = n
	}
	if v := strings.TrimSpace(getenv("INCIDENT_AUTOCLOSE_DRY_RUN")); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return Config{}, fmt.Errorf("INCIDENT_AUTOCLOSE_DRY_RUN %q is not a boolean", v)
		}
		cfg.DryRun = b
	}
	return cfg, nil
}

type closer interface {
	AutoClose(ctx context.Context, req Request) (Response, error)
}

// Run returns a registry.Task.Handler that closes every incident Resolved for
// cfg.Days or more, a page at a time.
//
// It asks again while entity-service reports a full page, up to MaxPages.
// entity-service lists the oldest due first with no offset, and an incident
// that fails stays Resolved and due, so it comes back at the head of every
// later page. Each request therefore asks for PageSize plus the failures seen
// so far (up to maxLimit), so failing incidents cannot hide the ones behind
// them. A failure is reported once per run.
//
// Two cases stop it earlier, because asking again would only return the same
// rows: a dry run (nothing is closed, so the next page is this page), and a
// full page with nothing closed and no new failure (only incidents that already
// failed, at maxLimit). The second fails the run and says that later due
// incidents were not reached.
//
// It returns an error when a call fails or any incident failed, naming the
// incidents, so the engine alerts and retries. That is safe: closed incidents
// are no longer due.
func Run(client closer, cfg Config) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		req := Request{ResolvedForDays: cfg.Days, DryRun: cfg.DryRun}
		// dryRun is what entity-service actually did, which the summary reports.
		dryRun := cfg.DryRun
		closed, pages := 0, 0
		var failures []string
		failedSeen := map[string]bool{}
		more := false
		var callErr error
		for pages < MaxPages {
			if err := ctx.Err(); err != nil {
				callErr = err
				break
			}
			req.Limit = min(PageSize+len(failedSeen), maxLimit)
			resp, err := client.AutoClose(ctx, req)
			pages++
			if err != nil {
				callErr = fmt.Errorf("POST /incidents/autoclose (page %d): %w", pages, err)
				break
			}
			if dryRun && !resp.DryRun {
				// entity-service ignored the dry run: whatever it listed is closed.
				callErr = fmt.Errorf("asked for a dry run but entity-service reports a real run (%d incidents closed)", len(resp.Closed))
				dryRun = false
			}
			for _, c := range resp.Closed {
				closed++
				if dryRun {
					slog.InfoContext(ctx, "incident autoclose: dry run, would close", "incident", c.Number, "id", c.ID, "resolvedOn", c.ResolvedOn)
				} else {
					slog.InfoContext(ctx, "incident autoclose: closed", "incident", c.Number, "id", c.ID, "resolvedOn", c.ResolvedOn)
				}
			}
			newFailures := 0
			for _, f := range resp.Failed {
				if failedSeen[f.ID] {
					continue
				}
				newFailures++
				failedSeen[f.ID] = true
				slog.WarnContext(ctx, "incident autoclose: failed to close", "incident", f.Number, "id", f.ID, "err", f.Error)
				failures = append(failures, fmt.Sprintf("%s: %s", f.Number, truncate(f.Error, maxErrorLen)))
			}
			more = resp.More
			if !more || callErr != nil {
				break
			}
			if dryRun {
				slog.InfoContext(ctx, "incident autoclose: dry run shows one page only; more incidents are due", "pageSize", PageSize)
				break
			}
			if len(resp.Closed) == 0 && newFailures == 0 {
				slog.WarnContext(ctx, "incident autoclose: a full page held only incidents that already failed; later due incidents were not reached",
					"page", pages, "limit", req.Limit, "failed", len(failedSeen))
				callErr = fmt.Errorf("the oldest %d due incidents all failed and filled a page of %d; later due incidents were not reached", len(failedSeen), req.Limit)
				break
			}
		}
		if more && pages >= MaxPages && callErr == nil {
			slog.WarnContext(ctx, "incident autoclose: page cap reached with incidents still due; the next run continues",
				"pages", pages, "pageSize", PageSize, "closed", closed)
		}

		slog.InfoContext(ctx, "incident autoclose: run complete", "dryRun", dryRun, "resolvedForDays", cfg.Days,
			"pages", pages, "closed", closed, "failed", len(failures), "more", more)

		var msgs []string
		if callErr != nil {
			msgs = append(msgs, callErr.Error())
		}
		if len(failures) > 0 {
			shown := failures
			if len(shown) > maxFailuresReported {
				shown = shown[:maxFailuresReported]
			}
			line := fmt.Sprintf("%d incidents could not be closed: %s", len(failures), strings.Join(shown, "; "))
			if extra := len(failures) - len(shown); extra > 0 {
				line += fmt.Sprintf("; ... and %d more", extra)
			}
			msgs = append(msgs, line)
		}
		if len(msgs) == 0 {
			return nil
		}
		return fmt.Errorf("incident autoclose: %s", strings.Join(msgs, "; "))
	}
}

func truncate(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "..."
	}
	return s
}
