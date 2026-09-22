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

// The entry point for csm-scheduled-tasks. Unlike every other Go component
// in this repo, this is not a long-running server: Choreo invokes this
// binary fresh on its own Scheduled Task trigger, main runs exactly one
// Engine.Tick over the registered task list, and exits. There is
// deliberately no internal ticker/cron loop here — Choreo's own trigger IS
// the driver (see this component's own CLAUDE.md for the "driver cadence"
// concept).
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/adhocore/gronx"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/allocationreminder"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/engagementallocations"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/engine"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/entitycases"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/housekeeping"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/ledger"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/notify"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/opencases"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/registry"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/stalecases"
)

func main() {
	loadDotEnv(".env")
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	driverInterval := envDuration("DRIVER_INTERVAL", time.Hour)

	// Shared OAuth2 client credentials — used by the entity-service client
	// below and the email client below it, and by any future service
	// client this component grows. Mirrors
	// integrations/csm-notification-service's own OAUTH2_CLIENT_ID/
	// OAUTH2_CLIENT_SECRET/OAUTH2_TOKEN_URL convention: the real deployments
	// these point at authenticate every caller through the same shared
	// gateway app, scoped per-client via each client's own *_SCOPES var, not
	// a separate per-consumer app. mustEnv even though the email client
	// alone is optional — entity-service is not, and both clients share
	// this one credential set.
	oauthTokenURL := mustEnv("OAUTH2_TOKEN_URL")
	oauthClientID := mustEnv("OAUTH2_CLIENT_ID")
	oauthClientSecret := mustEnv("OAUTH2_CLIENT_SECRET")

	// entity-service is this component's only durable state — see
	// internal/ledger's own doc comment — so a missing CUSTOMER_ENTITY_SERVICE_BASE_URL,
	// or either URL failing httpsec's https-only check, fails startup
	// loudly rather than surfacing as a per-task error later, unlike the
	// email client below.
	entityServiceBaseURL := mustEnv("CUSTOMER_ENTITY_SERVICE_BASE_URL")
	entityServiceScopes := splitComma(os.Getenv("CUSTOMER_ENTITY_SERVICE_SCOPES"))
	ledgerClient, err := ledger.NewClient(ledger.Config{
		BaseURL:      entityServiceBaseURL,
		TokenURL:     oauthTokenURL,
		ClientID:     oauthClientID,
		ClientSecret: oauthClientSecret,
		Scopes:       entityServiceScopes,
	})
	if err != nil {
		slog.Error("failed to construct entity-service client", "err", err)
		os.Exit(1)
	}

	// Same entity-service deployment and credentials as ledgerClient above,
	// but a separate client — see internal/entitycases' own doc comment for
	// why case search isn't just another method on ledger.Client.
	entityCasesClient, err := entitycases.NewClient(entitycases.Config{
		BaseURL:      entityServiceBaseURL,
		TokenURL:     oauthTokenURL,
		ClientID:     oauthClientID,
		ClientSecret: oauthClientSecret,
		Scopes:       entityServiceScopes,
	})
	if err != nil {
		slog.Error("failed to construct entity-service case-search client", "err", err)
		os.Exit(1)
	}

	// Third entity-service client, same deployment and credentials again —
	// see internal/engagementallocations' own doc comment for why "who owes a
	// weekly status update" isn't another method on either of the other two.
	engagementAllocationsClient, err := engagementallocations.NewClient(engagementallocations.Config{
		BaseURL:      entityServiceBaseURL,
		TokenURL:     oauthTokenURL,
		ClientID:     oauthClientID,
		ClientSecret: oauthClientSecret,
		Scopes:       entityServiceScopes,
	})
	if err != nil {
		slog.Error("failed to construct entity-service engagement-allocations client", "err", err)
		os.Exit(1)
	}

	// Global kill switch for every failure alert email — see
	// engine.Engine.AlertsEnabled's own doc comment. Defaults to true (the
	// current always-alert behavior); set to false to go quiet without
	// touching ALERT_RECIPIENTS or any task's SUB_CRON_RECIPIENTS entry.
	alertsEnabled := envBool("ALERTS_ENABLED", true)

	// Standing ops/on-call audience, emailed on every failure for every
	// task in addition to that task's own registry.Task.To/Cc — see
	// engine.Engine.AlertRecipients' own doc comment. The EMAIL_BASE_URL
	// check below covers this list too, once every task's own To is known.
	alertRecipients := splitComma(os.Getenv("ALERT_RECIPIENTS"))
	emailBaseURL := os.Getenv("EMAIL_BASE_URL")

	// Email itself is not required for every deployment — EMAIL_BASE_URL is
	// read with os.Getenv, not mustEnv, matching
	// integrations/csm-notification-service's own
	// internal/notifications.EmailClient. NewClient itself still fails
	// startup if EMAIL_BASE_URL is set but not https. Authenticates with
	// the same shared OAUTH2_* credentials as ledgerClient above, not its
	// own — only BaseURL/Scopes/FromAddress are specific to this client.
	emailClient, err := notify.NewClient(notify.Config{
		BaseURL:      emailBaseURL,
		TokenURL:     oauthTokenURL,
		ClientID:     oauthClientID,
		ClientSecret: oauthClientSecret,
		Scopes:       splitComma(os.Getenv("EMAIL_SCOPES")),
		FromAddress:  os.Getenv("EMAIL_FROM_ADDRESS"),
	})
	if err != nil {
		slog.Error("failed to construct email client", "err", err)
		os.Exit(1)
	}

	// Shared by both config-driven override maps below — see
	// parseSubCronSchedules/parseSubCronRecipients's own doc comments.
	scheduleOverrides := parseSubCronSchedules(os.Getenv("SUB_CRON_SCHEDULES"))
	recipientOverrides := parseSubCronRecipients(os.Getenv("SUB_CRON_RECIPIENTS"))

	const housekeepingTaskName = "housekeeping_cleanup"
	housekeepingTo, housekeepingCc := recipientsFor(recipientOverrides, housekeepingTaskName)

	const staleCasesTaskName = "stale_cases_report"
	staleCasesTo, staleCasesCc := recipientsFor(recipientOverrides, staleCasesTaskName)
	// Fixed, not env-configurable — unlike HOUSEKEEPING_RETENTION_DAYS, there's
	// no operational reason to tune this per deployment today. Revisit as an
	// env var (mirroring envDays("HOUSEKEEPING_RETENTION_DAYS", 30)'s shape)
	// if that changes.
	const staleCaseThreshold = 30 * 24 * time.Hour

	const openCasesTaskName = "open_cases_report"
	openCasesTo, openCasesCc := recipientsFor(recipientOverrides, openCasesTaskName)
	// NOTE: this task's SUB_CRON_RECIPIENTS entry is FAILURE-ALERT ONLY. Unlike
	// the two report tasks above, its real audience is the people who owe an update,
	// resolved from the data — leaving it unset does not stop it mailing them.
	// See internal/allocationreminder's own doc comment.
	// The CSM Portal the reminder's navigation steps link to. Optional: unset
	// just means the mail names "Engagements" without linking it. Deliberately
	// this component's own variable rather than a shared one — point it at the
	// portal these recipients actually use.
	csmPortalWebBaseURL := os.Getenv("CSM_PORTAL_WEB_BASE_URL")

	const allocationReminderTaskName = "allocation_status_update_reminder"
	allocationReminderTo, allocationReminderCc := recipientsFor(recipientOverrides, allocationReminderTaskName)

	tasks := []registry.Task{
		// This component's first real sub-cron: deletes rows from
		// entity-service's scheduled_task_run table that succeeded or were
		// superseded ("fully omitted after retrying") more than
		// HOUSEKEEPING_RETENTION_DAYS ago — see internal/housekeeping's own
		// doc comment. Default schedule is daily at 03:00, a low-traffic hour
		// assuming the deployment runs with TZ=UTC (see
		// internal/schedule.PeriodKey's own doc comment — cron expressions
		// have no timezone of their own); override via SUB_CRON_SCHEDULES if
		// needed.
		{
			Name:     housekeepingTaskName,
			Schedule: scheduleFor(scheduleOverrides, housekeepingTaskName, "0 3 * * *"),
			Handler:  housekeeping.CleanupResolvedRuns(ledgerClient, envDays("HOUSEKEEPING_RETENTION_DAYS", 30)),
			To:       housekeepingTo,
			Cc:       housekeepingCc,
		},
		// Emails staleCasesTo/Cc a report of every case open more than
		// staleCaseThreshold — see internal/stalecases's own doc comment.
		// Sends nothing (but still succeeds) if staleCasesTo is empty, i.e.
		// this task isn't mentioned in SUB_CRON_RECIPIENTS. Default schedule
		// is daily at 07:00 (again, TZ=UTC-dependent — see above), ahead of
		// most business hours; override via SUB_CRON_SCHEDULES if needed.
		{
			Name:     staleCasesTaskName,
			Schedule: scheduleFor(scheduleOverrides, staleCasesTaskName, "0 7 * * *"),
			Handler: stalecases.SendReport(entityCasesClient, emailClient,
				staleCaseThreshold, staleCasesTo, staleCasesCc, alertsEnabled),
			To: staleCasesTo,
			Cc: staleCasesCc,
		},
		// Emails openCasesTo/Cc a report of every case created before
		// yesterday that's still in "open" state (nobody has moved it out of
		// initial triage) — see internal/opencases's own doc comment. Sends
		// nothing (but still succeeds) if openCasesTo is empty. Default
		// schedule is daily at 08:00 (again, TZ=UTC-dependent — see above);
		// override via SUB_CRON_SCHEDULES if needed.
		{
			Name:     openCasesTaskName,
			Schedule: scheduleFor(scheduleOverrides, openCasesTaskName, "0 8 * * *"),
			Handler:  opencases.SendReport(entityCasesClient, emailClient, openCasesTo, openCasesCc, alertsEnabled),
			To:       openCasesTo,
			Cc:       openCasesCc,
		},
		// Emails every person who has a live engagement allocation but has not
		// published their own status update for last week — the Go port of
		// ServiceNow's WeeklyAllocationStatusUpdateReminderEmailFlow. Unlike
		// the two report tasks above, the recipients come from the data, not from
		// SUB_CRON_RECIPIENTS; that entry only sets who is alerted when this
		// FAILS. See internal/allocationreminder's own doc comment.
		//
		// Default schedule is Monday at 00:00, matching the ServiceNow
		// trigger's own "Weekly on Monday at 00:00:00" — subject to the same
		// TZ=UTC caveat as every other schedule here (see "Housekeeping" in
		// this component's CLAUDE.md). The cycle it asks about is derived from
		// the ISO calendar rather than from when it happens to run, so a retry
		// later in the week still reminds about the same week.
		{
			Name:     allocationReminderTaskName,
			Schedule: scheduleFor(scheduleOverrides, allocationReminderTaskName, "0 0 * * 1"),
			Handler:  allocationreminder.SendReminders(engagementAllocationsClient, emailClient, csmPortalWebBaseURL, alertsEnabled),
			To:       allocationReminderTo,
			Cc:       allocationReminderCc,
		},
	}

	var tasksWithRecipients []string
	for _, t := range tasks {
		if !gronx.IsValid(t.Schedule) {
			slog.Error("invalid cron schedule for registered task; refusing to start", "task", t.Name, "schedule", t.Schedule)
			os.Exit(1)
		}
		if len(t.To) > 0 {
			tasksWithRecipients = append(tasksWithRecipients, t.Name)
		}
	}

	// A non-empty audience with no EMAIL_BASE_URL configured would otherwise
	// only surface the first time some task actually fails and tries to
	// send, as an opaque "invalid URL" error from a relative "/send-email"
	// path — much easier to catch here, at startup. Checked after tasks is
	// built so a per-task SUB_CRON_RECIPIENTS "to" list is covered too, not
	// just the standing ALERT_RECIPIENTS audience. Skipped entirely when
	// ALERTS_ENABLED=false — no email will ever be sent in that case, so an
	// unset EMAIL_BASE_URL isn't a misconfiguration.
	if alertsEnabled && emailBaseURL == "" && (len(alertRecipients) > 0 || len(tasksWithRecipients) > 0) {
		slog.Error("failure alert recipients are configured but EMAIL_BASE_URL is not; refusing to start since those alerts could never actually send",
			"alertRecipientsSet", len(alertRecipients) > 0, "tasksWithOwnRecipients", tasksWithRecipients)
		os.Exit(1)
	}

	eng := engine.New(tasks, ledgerClient, emailClient, driverInterval, alertRecipients, alertsEnabled)

	// No app-level execution timeout here — Choreo's own Scheduled Task
	// execution-time limit already bounds how long one invocation can run.
	// signal.NotifyContext instead cancels this context the moment Choreo
	// sends SIGTERM (whether that's from its own timeout firing, a
	// redeploy, or a manual stop), so in-flight HTTP calls to entity-service
	// abort promptly and this process can log/exit cleanly, rather than
	// being cut off mid-request with no chance to react.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	start := time.Now()
	eng.Tick(ctx, start)
	slog.Info("tick complete", "elapsed", time.Since(start).String())
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		slog.Error("required environment variable is not set", "key", key)
		os.Exit(1)
	}
	return v
}

// parseSubCronSchedules decodes SUB_CRON_SCHEDULES, a JSON object mapping a
// registered task's Name to a cron schedule override — one shared config
// value for every task in the registry, rather than a dedicated env var per
// task name that would need inventing again for every new sub-cron added
// here. A missing or malformed value logs a warning and yields no
// overrides, so every task just falls back to its own hardcoded default
// schedule — mirrors integrations/csm-notification-service's own
// parseGoogleChatSpaces (same "optional JSON env var, log and fall back on
// a bad value" shape).
func parseSubCronSchedules(raw string) map[string]string {
	if raw == "" {
		return nil
	}
	var overrides map[string]string
	if err := json.Unmarshal([]byte(raw), &overrides); err != nil {
		slog.Error("failed to parse SUB_CRON_SCHEDULES; every task will use its own hardcoded default schedule", "err", err)
		return nil
	}
	return overrides
}

// scheduleFor returns overrides[taskName] if present and non-empty,
// otherwise def. def is still what ships when SUB_CRON_SCHEDULES doesn't
// mention taskName at all — every registered task keeps a sensible
// hardcoded default in code; SUB_CRON_SCHEDULES only ever overrides it.
func scheduleFor(overrides map[string]string, taskName, def string) string {
	if s, ok := overrides[taskName]; ok && s != "" {
		return s
	}
	return def
}

// subCronRecipients is the per-task shape decoded from SUB_CRON_RECIPIENTS.
type subCronRecipients struct {
	To []string `json:"to"`
	Cc []string `json:"cc"`
}

// parseSubCronRecipients decodes SUB_CRON_RECIPIENTS, a JSON object mapping
// a registered task's Name to {"to": [...], "cc": [...]} — the config-driven
// counterpart to SUB_CRON_SCHEDULES, so a sub-cron's failure audience lives
// in .env next to its cadence, not hardcoded as registry.Task{To, Cc}
// literals in this file. A task not mentioned here just gets nil To/Cc —
// its failures still reach the standing ALERT_RECIPIENTS list (see
// engine.Engine.AlertRecipients' own doc comment), it just has no
// additional audience of its own. A missing or malformed value logs a
// warning and yields no per-task recipients at all, the same
// fail-safe-not-fail-closed shape parseSubCronSchedules uses.
func parseSubCronRecipients(raw string) map[string]subCronRecipients {
	if raw == "" {
		return nil
	}
	var overrides map[string]subCronRecipients
	if err := json.Unmarshal([]byte(raw), &overrides); err != nil {
		slog.Error("failed to parse SUB_CRON_RECIPIENTS; every task will have no per-task alert recipients", "err", err)
		return nil
	}
	return overrides
}

// recipientsFor returns overrides[taskName]'s To/Cc, or nil, nil if
// taskName isn't mentioned in overrides at all.
func recipientsFor(overrides map[string]subCronRecipients, taskName string) (to, cc []string) {
	r := overrides[taskName]
	return r.To, r.Cc
}

// envBool returns the given environment variable parsed with
// strconv.ParseBool (accepts "true"/"false"/"1"/"0"/"t"/"f", etc.), or def
// if unset or malformed.
func envBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		slog.Warn("environment variable is not a valid boolean; using default", "key", key, "value", v, "default", def)
		return def
	}
	return b
}

// envDuration returns the given environment variable parsed with
// time.ParseDuration (e.g. "1h", "5m"), or def if unset or malformed.
func envDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		slog.Warn("environment variable is not a valid positive duration; using default", "key", key, "value", v, "default", def)
		return def
	}
	return d
}

// maxRetentionDays is the largest value envDays accepts: the most whole
// days that fit in a time.Duration (an int64 count of nanoseconds) without
// overflowing. A larger value wraps around to a negative duration, which
// would turn a retention window into a future cleanup cutoff and delete
// every already-resolved row instead of none of them.
const maxRetentionDays = int64(math.MaxInt64) / int64(24*time.Hour)

// envDays returns the given environment variable, parsed as a positive
// whole number of days and converted to a time.Duration, or defDays if
// unset, malformed, or too large to convert without overflowing. A plain
// integer is friendlier for a "how many days of history to keep" setting
// than requiring Go duration syntax like "720h".
func envDays(key string, defDays int) time.Duration {
	def := time.Duration(defDays) * 24 * time.Hour
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n <= 0 || n > maxRetentionDays {
		slog.Warn("environment variable is not a valid positive number of days; using default", "key", key, "value", v, "defaultDays", defDays)
		return def
	}
	return time.Duration(n) * 24 * time.Hour
}

func splitComma(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			result = append(result, t)
		}
	}
	return result
}

// loadDotEnv reads a .env file and sets any unset environment variables
// from it. Silently ignored if the file does not exist; logs a warning for
// any other error. Mirrors integrations/csm-notification-service's own
// cmd/server/main.go helper of the same name.
func loadDotEnv(path string) {
	f, err := os.Open(path) // #nosec G304 -- path is always the hardcoded literal ".env" at the only call site
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			slog.Warn("loadDotEnv: failed to open .env file", "err", err)
		}
		return
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if len(v) >= 2 && ((v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'')) {
			v = v[1 : len(v)-1]
		}
		if _, present := os.LookupEnv(k); !present {
			_ = os.Setenv(k, v)
		}
	}
	if err := scanner.Err(); err != nil {
		slog.Warn("loadDotEnv: error reading .env file", "err", err)
	}
}
