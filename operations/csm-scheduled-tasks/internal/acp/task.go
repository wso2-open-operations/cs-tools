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

// Package acp is the acp_closure sub-cron: the Account Closure Process,
// moved here from integrations/acp-closure-service. One run evaluates every
// open project, sends the end-date and invoice notices that are due and
// suspends the projects that reached day 0. The rules live in the
// subpackages (closure, sweep, recipients, ...), which came over unchanged;
// this file only wires them to the engine. See this component's CLAUDE.md,
// "ACP closure".
package acp

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/acp/entity"
	acpnotify "github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/acp/notify"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/acp/salesentity"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/acp/sweep"
)

// EmailSender is the send surface ACP's notices go through: this
// component's notify.Sender, normally wrapped by notify.ForReport so a
// "debug" list in SUB_CRON_RECIPIENTS redirects every notice.
type EmailSender interface {
	SendEmail(ctx context.Context, to, cc []string, subject, htmlBody string) error
}

// Clients are the services one run talks to. CSM (entity-service) has the
// open-project list, closure states and notices sent; Salesforce (through
// sales-entity-service) has the dates, people, opportunities and invoices,
// and takes State__c=Suspended on suspension.
type Clients struct {
	CSM         *entity.Client
	SalesEntity *salesentity.Client
	Email       EmailSender
}

// Config is the ACP_* configuration, read in cmd/server/main.go.
type Config struct {
	// DryRun decides, sends and logs, but writes nothing to CSM or
	// Salesforce. ACP_DRY_RUN, default true. It does not stop emails: with
	// EmailEnabled the notices are sent, and because no notice is recorded
	// they are sent again on every run.
	DryRun bool
	// EmailEnabled sends notices for real; when false they are only logged.
	// ACP_EMAIL_SEND_ENABLED (default false) and ALERTS_ENABLED together.
	EmailEnabled bool
	// AllowNonWSO2Recipients lets notices reach addresses outside
	// @wso2.com. ACP_ALLOW_NON_WSO2_RECIPIENTS, default false; production
	// only.
	AllowNonWSO2Recipients bool
	// StandingRecipients are the distribution lists added to every notice.
	// ACP_STANDING_CC_RECIPIENTS.
	StandingRecipients []string
	// TestProjectID scopes the run to one CSM project. ACP_TEST_PROJECT_ID.
	TestProjectID string
	// ExcludedProjectIDs are CSM project ids the run skips.
	// ACP_EXCLUDED_PROJECT_IDS.
	ExcludedProjectIDs map[string]bool
}

// Run returns the acp_closure handler. A sweep that could not list the
// projects, or one in which any project failed, returns an error, so the
// engine records the run as failed, alerts and retries it after the task's
// RetryBackoff. A retry is safe: notices sent and suspensions are recorded
// in CSM, so a project already handled is not handled again.
func Run(clients Clients, cfg Config) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		now := time.Now()
		runID := newRunID()
		slog.InfoContext(ctx, "acp: starting",
			"runID", runID,
			"dryRun", cfg.DryRun,
			"emailEnabled", cfg.EmailEnabled,
			"testProjectID", cfg.TestProjectID,
			"excludedProjectIDs", sortedKeys(cfg.ExcludedProjectIDs),
			"standingRecipientsCount", len(cfg.StandingRecipients),
		)

		// The schedule is weekdays only, but a retry of Friday's run can
		// land on a Saturday. Succeeding without doing anything closes that
		// period; Monday's run is a new one and catches up.
		if isWeekend(now) {
			slog.InfoContext(ctx, "acp: weekend, skipping run: no notices sent and no projects suspended", "runID", runID)
			return nil
		}

		var updater interface {
			UpdateProject(ctx context.Context, id string, body []byte) ([]byte, error)
			SuspendInSalesforce(ctx context.Context, sfID string) error
		} = sweep.NewWriter(clients.CSM, clients.SalesEntity)
		if cfg.DryRun {
			updater = &sweep.DryRunProjectUpdater{}
		}

		var ntf interface {
			Send(ctx context.Context, n acpnotify.Notice) (delivered bool, err error)
		} = &acpnotify.LoggingNotifier{Logger: slog.Default()}
		if cfg.EmailEnabled {
			ntf = &acpnotify.EmailNotifier{
				Sender:                 clients.Email,
				Logger:                 slog.Default(),
				AllowNonWSO2Recipients: cfg.AllowNonWSO2Recipients,
				StandingRecipients:     cfg.StandingRecipients,
			}
		}

		ctx = entity.WithCorrelationID(ctx, runID)
		result, err := sweep.Run(ctx, clients.CSM, sweep.NewSalesforceDetails(clients.SalesEntity), updater, ntf, now, cfg.TestProjectID, cfg.ExcludedProjectIDs)
		if err != nil {
			return fmt.Errorf("acp run %s: %w", runID, err)
		}

		slog.InfoContext(ctx, "acp: finished",
			"runID", runID,
			"dryRun", cfg.DryRun,
			"projectsEvaluated", result.ProjectsEvaluated,
			"projectsExcluded", result.ProjectsExcluded,
			"failureCount", len(result.Failures),
		)
		for _, f := range result.Failures {
			slog.ErrorContext(ctx, "acp: project failed", "runID", runID, "projectID", f.ProjectID, "err", f.Err)
		}
		return failuresError(runID, result.Failures)
	}
}

// maxFailedIDsInError caps how many project ids the failure alert lists;
// the log has every one with its error.
const maxFailedIDsInError = 20

// failuresError is the handler's error for a run in which projects failed,
// nil when none did. It names the run and the failed project ids, not
// their errors: the alert email carries this text, and the errors can
// include response bodies with customer data. The log has the full detail.
func failuresError(runID string, failures []sweep.ProjectFailure) error {
	if len(failures) == 0 {
		return nil
	}
	ids := make([]string, 0, min(len(failures), maxFailedIDsInError))
	for _, f := range failures[:min(len(failures), maxFailedIDsInError)] {
		ids = append(ids, f.ProjectID)
	}
	more := ""
	if n := len(failures) - len(ids); n > 0 {
		more = fmt.Sprintf(" and %d more", n)
	}
	return fmt.Errorf("acp run %s: %d project(s) failed: %s%s; see the log for each error", runID, len(failures), strings.Join(ids, ", "), more)
}

// operationsZone is the timezone "weekend" is judged in: UTC+05:30, the CS
// team's local time (Sri Lanka has no DST, so a fixed offset is exact and
// needs no tzdata in the container).
var operationsZone = time.FixedZone("UTC+05:30", 5*60*60+30*60)

// isWeekend reports whether now falls on a Saturday or Sunday in
// operationsZone. No emails go out on weekends, so the whole run is skipped
// — not just the sends. Skipping only the sends would break the notice
// cascade: a day-0 suspend on Saturday would flip the project to closed, and
// Monday's run would then treat its suspension notice as already handled
// and never send it. Skipping the run is safe instead because notice windows
// and suspension are threshold-based (days remaining <= window), so Monday's
// run picks up everything that came due over the weekend — a weekend day 0
// suspends on Monday.
func isWeekend(now time.Time) bool {
	switch now.In(operationsZone).Weekday() {
	case time.Saturday, time.Sunday:
		return true
	}
	return false
}

// ParseProjectIDs parses ACP_EXCLUDED_PROJECT_IDS: a comma-separated list
// of CSM project ids the run skips entirely, never fetching or evaluating
// them. Per explicit design direction (Sajith Ekanayake): this is for
// deliberate, verified business exclusions — expected to be empty in
// production almost all the time. Not a substitute for fixing genuine data
// issues; a project put here produces zero signal about whatever might
// actually be wrong with it. Entries are trimmed; empty ones are dropped.
func ParseProjectIDs(v string) map[string]bool {
	ids := map[string]bool{}
	for raw := range strings.SplitSeq(v, ",") {
		if id := strings.TrimSpace(raw); id != "" {
			ids[id] = true
		}
	}
	return ids
}

// sortedKeys returns m's keys in sorted order, so the same configuration
// logs the same way on every run.
func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// newRunID generates a UUID v4 identifying one run, forwarded as the
// correlation ID on every outbound call so a run's log lines and upstream
// requests can be tied together.
func newRunID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("acp: failed to read random bytes: " + err.Error())
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant bits
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
