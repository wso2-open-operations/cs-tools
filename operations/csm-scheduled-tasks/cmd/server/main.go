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
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/adhocore/gronx"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/announcementpublish"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/autoclosure"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/availability"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/cloudstatus"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/engine"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/entitycases"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/housekeeping"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/httpsec"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/ledger"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/migrationreport"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/migrationscan"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/notify"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/opencases"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/queryhours"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/queryhoursreport"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/registry"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/regressionreport"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/regressionscan"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/rotareminder"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/schedule"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/stalecases"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/teamschedule"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/unresolvedproducts"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/usagerollup"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/worklogreport"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/worklogsreport"
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

	// The unresolved products digest's own client: the same deployment and
	// credentials, one internal-only listing endpoint.
	unresolvedProductsClient, err := unresolvedproducts.NewClient(unresolvedproducts.Config{
		BaseURL:      entityServiceBaseURL,
		TokenURL:     oauthTokenURL,
		ClientID:     oauthClientID,
		ClientSecret: oauthClientSecret,
		Scopes:       entityServiceScopes,
	})
	if err != nil {
		slog.Error("failed to construct entity-service unresolved products client", "err", err)
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

	// The migration scan's own client: the same deployment and credentials,
	// a distinct endpoint (GET /reports/migration-cases) with its own
	// timeout.
	migrationReportClient, err := migrationreport.NewClient(migrationreport.Config{
		BaseURL:      entityServiceBaseURL,
		TokenURL:     oauthTokenURL,
		ClientID:     oauthClientID,
		ClientSecret: oauthClientSecret,
		Scopes:       entityServiceScopes,
	})
	if err != nil {
		slog.Error("failed to construct entity-service migration-scan client", "err", err)
		os.Exit(1)
	}

	// The product regression reports' own client: the same deployment and
	// credentials, a distinct endpoint (GET /reports/product-regressions).
	regressionReportClient, err := regressionreport.NewClient(regressionreport.Config{
		BaseURL:      entityServiceBaseURL,
		TokenURL:     oauthTokenURL,
		ClientID:     oauthClientID,
		ClientSecret: oauthClientSecret,
		Scopes:       entityServiceScopes,
	})
	if err != nil {
		slog.Error("failed to construct entity-service product-regression-report client", "err", err)
		os.Exit(1)
	}

	// The CS Worklogs reports' own client. Same deployment and
	// credentials, separate client for the same reason as the others: a
	// distinct endpoint whose timeout differs materially. A weekly report
	// reads every time card in a seven-day window across every delivery
	// team, so it allows ninety seconds.
	// The query-hour client. Same deployment and credentials; its own
	// client because a recompute walks every opportunity in service and
	// every project linked to each, so it needs minutes rather than the
	// seconds a read allows.
	queryHourClient, err := queryhours.NewClient(queryhours.Config{
		BaseURL:      entityServiceBaseURL,
		TokenURL:     oauthTokenURL,
		ClientID:     oauthClientID,
		ClientSecret: oauthClientSecret,
		Scopes:       entityServiceScopes,
	})
	if err != nil {
		slog.Error("failed to construct entity-service query-hour client", "err", err)
		os.Exit(1)
	}

	worklogReportClient, err := worklogreport.NewClient(worklogreport.Config{
		BaseURL:      entityServiceBaseURL,
		TokenURL:     oauthTokenURL,
		ClientID:     oauthClientID,
		ClientSecret: oauthClientSecret,
		Scopes:       entityServiceScopes,
	})
	if err != nil {
		slog.Error("failed to construct entity-service worklog-report client", "err", err)
		os.Exit(1)
	}

	// The availability sweep's own client. Same deployment and credentials
	// again, and a separate client for the same reason as the others: a
	// distinct endpoint whose timeout differs materially. This one allows
	// five minutes where the neighbouring sweeps allow sixty seconds --
	// ~146 subjects, each with a twelve-month outage query and up to eight
	// rows written, is a normal run here rather than a sign of trouble.
	//
	// *** OFF UNLESS AVAILABILITY_RECALC_ENABLED=true. *** ServiceNow's
	// "Calculate Availability" job still writes service_availability (mirrored
	// in by csm-sync-service), and that table has no unique constraint on a
	// period's natural key: with both running, matching periods can end up
	// with two rows. Turning this on is a paired change with switching
	// ServiceNow's job off -- the same shape as CLOUD_STATUS_ENABLED.
	availabilityEnabled := envBool("AVAILABILITY_RECALC_ENABLED", false)
	var availabilityClient *availability.Client
	if availabilityEnabled {
		availabilityClient, err = availability.NewClient(availability.Config{
			BaseURL:      entityServiceBaseURL,
			TokenURL:     oauthTokenURL,
			ClientID:     oauthClientID,
			ClientSecret: oauthClientSecret,
			Scopes:       entityServiceScopes,
		})
		if err != nil {
			slog.Error("failed to construct entity-service availability client", "err", err)
			os.Exit(1)
		}
	}

	// The usage rollups' own client: the same deployment and credentials, two
	// internal-only endpoints. Off by default for the reason documented on the
	// task registration below.
	usageRollupEnabled := envBool("USAGE_ROLLUP_ENABLED", false)
	var usageRollupClient *usagerollup.Client
	if usageRollupEnabled {
		usageRollupClient, err = usagerollup.NewClient(usagerollup.Config{
			BaseURL:      entityServiceBaseURL,
			TokenURL:     oauthTokenURL,
			ClientID:     oauthClientID,
			ClientSecret: oauthClientSecret,
			Scopes:       entityServiceScopes,
		})
		if err != nil {
			slog.Error("failed to construct entity-service usage rollup client", "err", err)
			os.Exit(1)
		}
	}

	// Same entity-service deployment and credentials again — a fourth,
	// separate client for the same reason entityCasesClient is its own:
	// this one's Bearer token must satisfy entity-service's own
	// AUTH_INTERNAL_CLIENT_IDS (AutoPublish is internal-caller-only), a
	// concern specific to this one task, not shared with case search or
	// the scheduled_task_run ledger.
	announcementPublishClient, err := announcementpublish.NewClient(announcementpublish.Config{
		BaseURL:      entityServiceBaseURL,
		TokenURL:     oauthTokenURL,
		ClientID:     oauthClientID,
		ClientSecret: oauthClientSecret,
		Scopes:       entityServiceScopes,
	})
	if err != nil {
		slog.Error("failed to construct entity-service announcement-publish client", "err", err)
		os.Exit(1)
	}

	// Cloud status: a fifth entity-service client, and the first outbound
	// integration this component has -- see internal/cloudstatus's package
	// doc. cloudStatusEnabled is the double-fire guard: ServiceNow's
	// `Cloud Status Event Notification Flow` is still live, and two systems
	// posting the same event to a PUBLIC status page is the most visible
	// possible way to get a cutover wrong. It stays false until that flow is
	// deactivated, and turning it on is a paired change with deactivating it.
	cloudStatusEnabled := envBool("CLOUD_STATUS_ENABLED", false)
	var cloudStatusClient *cloudstatus.Client
	var cloudStatusWebhook *cloudstatus.Webhook
	if cloudStatusEnabled {
		cloudStatusClient, err = cloudstatus.NewClient(cloudstatus.Config{
			BaseURL:      entityServiceBaseURL,
			TokenURL:     oauthTokenURL,
			ClientID:     oauthClientID,
			ClientSecret: oauthClientSecret,
			Scopes:       entityServiceScopes,
		})
		if err != nil {
			slog.Error("failed to construct entity-service cloud-status client", "err", err)
			os.Exit(1)
		}
		// *** A BAD MAP MUST STOP STARTUP, NOT BURN THE RETRY BUDGET. ***
		// parseStringMap returns nil on malformed JSON, and NewWebhook
		// accepts an empty map. With the feature enabled that combination
		// records every pending webhook as a "no dashboard URL" failure, and
		// after cloudStatusMaxAttempts entity-service stops handing the
		// event out -- so one config typo loses every outage event
		// permanently, silently, and unrecoverably. Exiting is the only
		// honest response: the component is being told to publish to a
		// public status page and cannot.
		cloudStatusURLs := parseStringMap("CLOUD_STATUS_WEBHOOK_URLS", os.Getenv("CLOUD_STATUS_WEBHOOK_URLS"))
		if len(cloudStatusURLs) == 0 {
			slog.Error("CLOUD_STATUS_ENABLED is true but CLOUD_STATUS_WEBHOOK_URLS is empty or unparseable",
				"hint", "expected a JSON object of cloud slug to base URL")
			os.Exit(1)
		}
		cloudStatusSecrets := parseStringMap("CLOUD_STATUS_WEBHOOK_SECRETS", os.Getenv("CLOUD_STATUS_WEBHOOK_SECRETS"))
		if len(cloudStatusSecrets) == 0 {
			// Unsigned posts are rejected with 401 by the dashboard, so an
			// empty secret map is the same permanent-loss failure as an
			// empty URL map.
			slog.Error("CLOUD_STATUS_ENABLED is true but CLOUD_STATUS_WEBHOOK_SECRETS is empty or unparseable",
				"hint", `expected a JSON object, e.g. {"default":"Secret <token>"}`)
			os.Exit(1)
		}

		cloudStatusWebhook, err = cloudstatus.NewWebhook(cloudstatus.WebhookConfig{
			BaseURLs: cloudStatusURLs,
			Secrets:  cloudStatusSecrets,
		})
		if err != nil {
			// Unlike the parse helpers, a bad URL here is fatal. The component
			// is being told to post to a public status page; starting up with
			// a URL that failed validation and discovering it at send time is
			// the wrong order to find out.
			slog.Error("failed to construct cloud status webhook poster", "err", err)
			os.Exit(1)
		}
	}

	// Rota reminders: emails to the people rostered on a rotation shift in the
	// Team Schedule. *** OFF UNLESS ROTA_REMINDERS_ENABLED=true. *** They
	// email real staff from the real rota, so an environment that holds the
	// rota but must not mail people turns them on only with
	// ROTA_REMINDER_REDIRECT_TO set. Registered only when enabled, like
	// cloud_status_webhooks, so the ledger shows no run for a task that is off.
	rotaRemindersEnabled := envBool("ROTA_REMINDERS_ENABLED", false)
	var teamScheduleClient *teamschedule.Client
	rotaZone := time.UTC
	if rotaRemindersEnabled {
		teamScheduleClient, err = teamschedule.NewClient(teamschedule.Config{
			BaseURL:      entityServiceBaseURL,
			TokenURL:     oauthTokenURL,
			ClientID:     oauthClientID,
			ClientSecret: oauthClientSecret,
			Scopes:       entityServiceScopes,
		})
		if err != nil {
			slog.Error("failed to construct entity-service team-schedule client", "err", err)
			os.Exit(1)
		}
		rotaZone, err = time.LoadLocation(envString("ROTA_REMINDER_TIME_ZONE", "Asia/Colombo"))
		if err != nil {
			slog.Error("ROTA_REMINDER_TIME_ZONE is not a known time zone", "err", err)
			os.Exit(1)
		}
		if u := os.Getenv("ROTA_REMINDER_SCHEDULE_URL"); u != "" && !strings.HasPrefix(u, "https://") && !strings.HasPrefix(u, "http://") {
			slog.Error("ROTA_REMINDER_SCHEDULE_URL must be an http(s) URL", "value", u)
			os.Exit(1)
		}
	}

	// Case auto-closure: reminders and automatic closure of tickets that sit in
	// Awaiting Info or Solution Proposed with no activity -- see
	// internal/autoclosure's package doc and entity-service's CLAUDE.md, "Case
	// auto-closure". OFF unless AUTOCLOSURE_ENABLED=true: it comments on and
	// closes customers' tickets, so it is never on by accident. entity-service
	// must also have AUTOCLOSURE_SYSTEM_USER_EMAIL set (else it does not
	// register the two routes this calls) and this component's client id must be
	// in its M2M_CLIENT_IDS.
	autoclosureEnabled := envBool("AUTOCLOSURE_ENABLED", false)
	var autoclosureClient *autoclosure.Client
	if autoclosureEnabled {
		autoclosureClient, err = autoclosure.NewClient(autoclosure.ClientConfig{
			BaseURL:      entityServiceBaseURL,
			TokenURL:     oauthTokenURL,
			ClientID:     oauthClientID,
			ClientSecret: oauthClientSecret,
			Scopes:       entityServiceScopes,
		})
		if err != nil {
			slog.Error("failed to construct entity-service auto-closure client", "err", err)
			os.Exit(1)
		}
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
	// Debug mail mode is per report -- see notify.ForReport. A task whose
	// SUB_CRON_RECIPIENTS entry carries a "debug" list sends its report to
	// that list only, with [Test copy] in the subject, until the list is
	// removed. Startup names every such task: left in place, a report on
	// trial looks exactly like a quiet week to its real audience.
	if onTrial := tasksWithDebugRecipients(recipientOverrides); len(onTrial) > 0 {
		slog.Warn("debug mail mode is on for these reports: they go to their debug recipients only, with [Test copy] in the subject",
			"tasks", onTrial)
	}

	const housekeepingTaskName = "housekeeping_cleanup"
	housekeepingTo, housekeepingCc := recipientsFor(recipientOverrides, housekeepingTaskName)

	const availabilityTaskName = "availability_recalculation"
	availabilityTo, availabilityCc := recipientsFor(recipientOverrides, availabilityTaskName)

	const usageRollupDailyTaskName = "usage_rollup_daily"
	usageRollupDailyTo, usageRollupDailyCc := recipientsFor(recipientOverrides, usageRollupDailyTaskName)
	const usageRollupMonthlyTaskName = "usage_rollup_monthly"
	usageRollupMonthlyTo, usageRollupMonthlyCc := recipientsFor(recipientOverrides, usageRollupMonthlyTaskName)
	const unresolvedProductsTaskName = "unresolved_products_digest"
	unresolvedProductsTo, unresolvedProductsCc := recipientsFor(recipientOverrides, unresolvedProductsTaskName)
	unresolvedProductsEmail := notify.ForReport(emailClient, debugFor(recipientOverrides, unresolvedProductsTaskName))

	const staleCasesTaskName = "stale_cases_report"
	staleCasesTo, staleCasesCc := recipientsFor(recipientOverrides, staleCasesTaskName)
	staleCasesEmail := notify.ForReport(emailClient, debugFor(recipientOverrides, staleCasesTaskName))
	// Fixed, not env-configurable — unlike HOUSEKEEPING_RETENTION_DAYS, there's
	// no operational reason to tune this per deployment today. Revisit as an
	// env var (mirroring envDays("HOUSEKEEPING_RETENTION_DAYS", 30)'s shape)
	// if that changes.
	const staleCaseThreshold = 30 * 24 * time.Hour

	const openCasesTaskName = "open_cases_report"
	// Where an issue key in a CS Worklogs mail points: the CSM portal,
	// which is where the engineers who receive these reports work. NOT
	// the customer portal -- that is a different audience and most of
	// them cannot even open it. ServiceNow linked into its own agent
	// workspace, the equivalent internal surface, and that URL dies at
	// cutover.
	//
	// Same variable and same path shape as csm-notification-service's
	// recipientlinks resolver, which already builds
	// <CSM_PORTAL_WEB_BASE_URL>/cases/{id} for internal recipients.
	//
	// Default is EMPTY, which renders the keys as plain text: better
	// than linking every engineer at a host nobody configured.
	// Validated like the other outbound URLs, so a typo stops startup
	// rather than producing a mail full of dead links.
	// Where the CS Worklogs audience comes from. "configured" is the
	// default and reproduces the delivered mails; "derived" hands the
	// decision to the team registry and WILL change who receives them,
	// so it is a deliberate switch rather than a fallback. The drift
	// line logged on every send shows the size of that change first.
	worklogRecipientSource, ok := worklogreport.ParseSource(os.Getenv("WORKLOG_RECIPIENTS_SOURCE"))
	if !ok {
		slog.Error("WORKLOG_RECIPIENTS_SOURCE must be configured or derived",
			"value", os.Getenv("WORKLOG_RECIPIENTS_SOURCE"))
		os.Exit(1)
	}

	worklogCaseLinkBase := strings.TrimRight(os.Getenv("CSM_PORTAL_WEB_BASE_URL"), "/")
	if worklogCaseLinkBase != "" {
		if err := httpsec.RequireSecureURL(worklogCaseLinkBase); err != nil {
			slog.Error("invalid CSM_PORTAL_WEB_BASE_URL", "err", err)
			os.Exit(1)
		}
	}

	const queryHoursWeeklyTaskName = "query_hours_weekly_report"
	const queryHoursNoticeTaskName = "query_hours_usage_notifications"
	const worklogsDailyTaskName = "cs_worklogs_daily_report"
	const worklogsWeeklyTaskName = "cs_worklogs_weekly_report"
	openCasesTo, openCasesCc := recipientsFor(recipientOverrides, openCasesTaskName)
	openCasesEmail := notify.ForReport(emailClient, debugFor(recipientOverrides, openCasesTaskName))
	// For the query-hour tasks these are ADDITIONS to the derived
	// audience, not the audience itself -- see the package doc.
	queryHoursWeeklyTo, queryHoursWeeklyCc := recipientsFor(recipientOverrides, queryHoursWeeklyTaskName)
	_, queryHoursNoticeCc := recipientsFor(recipientOverrides, queryHoursNoticeTaskName)
	// Both query-hour tasks go through the per-report debug wrapper,
	// like every other report here. It matters more for these two than
	// for most: their audience is DERIVED from the accounts being
	// reported on, so without a debug list a tryout would mail account
	// managers and technical owners directly.
	queryHoursWeeklyEmail := notify.ForReport(emailClient, debugFor(recipientOverrides, queryHoursWeeklyTaskName))
	queryHoursNoticeEmail := notify.ForReport(emailClient, debugFor(recipientOverrides, queryHoursNoticeTaskName))
	worklogsDailyTo, worklogsDailyCc := recipientsFor(recipientOverrides, worklogsDailyTaskName)
	worklogsWeeklyTo, worklogsWeeklyCc := recipientsFor(recipientOverrides, worklogsWeeklyTaskName)
	worklogsDailySchedule := scheduleFor(scheduleOverrides, worklogsDailyTaskName, "55 3 * * *")
	worklogsWeeklySchedule := scheduleFor(scheduleOverrides, worklogsWeeklyTaskName, "0 16 * * 1")
	worklogsDailyEmail := notify.ForReport(emailClient, debugFor(recipientOverrides, worklogsDailyTaskName))
	worklogsWeeklyEmail := notify.ForReport(emailClient, debugFor(recipientOverrides, worklogsWeeklyTaskName))

	const migrationCaseReportTaskName = "migration_case_report"
	migrationCaseTo, migrationCaseCc := recipientsFor(recipientOverrides, migrationCaseReportTaskName)
	migrationCaseEmail := notify.ForReport(emailClient, debugFor(recipientOverrides, migrationCaseReportTaskName))
	// The age a migration must reach before the weekly scan lists it. One
	// value, read once: the source report hard-coded its window in three
	// places and the CS team asked for six months in place of three.
	migrationCaseOlderThanDays := int(envDays("MIGRATION_CASE_REPORT_OLDER_THAN_DAYS", 180).Hours() / 24)
	// entity-service caps the age at ten years and answers 400 above it; a
	// larger value would fail every run until corrected, visible only in
	// the failure alerts. Refuse to start instead.
	if migrationCaseOlderThanDays > migrationCaseMaxOlderThanDays {
		slog.Error("MIGRATION_CASE_REPORT_OLDER_THAN_DAYS exceeds the maximum entity-service accepts; refusing to start",
			"value", migrationCaseOlderThanDays, "max", migrationCaseMaxOlderThanDays)
		os.Exit(1)
	}
	// The support portal's case page prefix for the links in the scan
	// (<prefix>/<case number>), the link the scan has always carried.
	// Unset renders the issue keys as plain text; a non-https value
	// refuses to start, as every URL here does.
	migrationCaseLinkBase := strings.TrimRight(os.Getenv("MIGRATION_CASE_REPORT_LINK_BASE_URL"), "/")
	if migrationCaseLinkBase != "" {
		if err := httpsec.RequireSecureURL(migrationCaseLinkBase); err != nil {
			slog.Error("invalid MIGRATION_CASE_REPORT_LINK_BASE_URL", "err", err)
			os.Exit(1)
		}
	}

	// The monthly product regression reports: one task per business unit
	// (product_regression_report_apim / _iam / _integration), each mailing
	// last month's regression cases for its own products. The product lists
	// are built in (regressionscan.DefaultVariants) and may be replaced per
	// report with PRODUCT_REGRESSION_REPORT_PRODUCTS, a JSON object of
	// report key to product names; a key that names no report, or an empty
	// list, refuses to start rather than mailing an empty report every
	// month.
	regressionVariants, err := regressionscan.ApplyProductOverrides(regressionscan.DefaultVariants(), os.Getenv("PRODUCT_REGRESSION_REPORT_PRODUCTS"))
	if err != nil {
		slog.Error("invalid PRODUCT_REGRESSION_REPORT_PRODUCTS", "err", err)
		os.Exit(1)
	}
	// An optional case page prefix for the links in the reports
	// (<prefix>/<case number>). Unset renders the case ids as plain text,
	// as the source report's PDF did; a non-https value refuses to start,
	// as every URL here does.
	regressionCaseLinkBase := strings.TrimRight(os.Getenv("PRODUCT_REGRESSION_REPORT_LINK_BASE_URL"), "/")
	if regressionCaseLinkBase != "" {
		if err := httpsec.RequireSecureURL(regressionCaseLinkBase); err != nil {
			slog.Error("invalid PRODUCT_REGRESSION_REPORT_LINK_BASE_URL", "err", err)
			os.Exit(1)
		}
	}
	// Default schedule: 09:00 on the 1st in India Standard Time, the hour
	// the reports have always arrived, which is 03:30 UTC (these cron
	// strings are UTC). Each report's schedule and recipients are looked
	// up by its own task name, like every other task here. To and Cc are
	// its SUB_CRON_RECIPIENTS entry: the report goes only to them, and
	// with no "to" the task skips its query and succeeds, as the stale and
	// open case reports do. A month with no regression cases sends nothing
	// (the source report omitted itself when empty).
	//
	// registry.Task.To/Cc are left nil DELIBERATELY, as for the migration
	// scan: the engine mails task.To plus ALERT_RECIPIENTS on every failed
	// attempt, and a failed period is retried on every tick. The audience
	// is business unit mail groups; an entity-service outage must not send
	// them an hourly FAILED notice. Failures go to ALERT_RECIPIENTS alone.
	// Because To is nil, the EMAIL_BASE_URL check below cannot see these
	// tasks' own audiences; regressionTasksWithRecipients carries them
	// there, so a deployment that names readers but no email service still
	// refuses to start instead of failing silently on the 1st.
	var regressionTasks []registry.Task
	var regressionTasksWithRecipients []string
	for _, v := range regressionVariants {
		name := regressionscan.TaskName(v)
		to, cc := recipientsFor(recipientOverrides, name)
		if len(to) > 0 {
			regressionTasksWithRecipients = append(regressionTasksWithRecipients, name)
		}
		regressionTasks = append(regressionTasks, registry.Task{
			Name:     name,
			Schedule: scheduleFor(scheduleOverrides, name, "30 3 1 * *"),
			Handler: regressionscan.SendReport(regressionReportClient,
				notify.ForReport(emailClient, debugFor(recipientOverrides, name)),
				v, to, cc, regressionCaseLinkBase, alertsEnabled),
		})
	}

	const publishScheduledAnnouncementsTaskName = "publish_scheduled_announcements"
	publishScheduledAnnouncementsTo, publishScheduledAnnouncementsCc := recipientsFor(recipientOverrides, publishScheduledAnnouncementsTaskName)

	const autoclosureTaskName = "case_autoclosure"
	autoclosureTo, autoclosureCc := recipientsFor(recipientOverrides, autoclosureTaskName)

	const cloudStatusTaskName = "cloud_status_webhooks"
	cloudStatusTo, cloudStatusCc := recipientsFor(recipientOverrides, cloudStatusTaskName)

	const rotaWeekdayTaskName = "rota_weekday_reminder"
	rotaWeekdayTo, rotaWeekdayCc := recipientsFor(recipientOverrides, rotaWeekdayTaskName)
	const rotaWeekendTaskName = "rota_weekend_notice"
	rotaWeekendTo, rotaWeekendCc := recipientsFor(recipientOverrides, rotaWeekendTaskName)
	const rotaWeekendReminderTaskName = "rota_weekend_reminder"
	rotaWeekendReminderTo, rotaWeekendReminderCc := recipientsFor(recipientOverrides, rotaWeekendReminderTaskName)
	const rotaManualTaskName = "rota_manual_reminder"
	rotaManualTo, rotaManualCc := recipientsFor(recipientOverrides, rotaManualTaskName)

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
			Handler: stalecases.SendReport(entityCasesClient, staleCasesEmail,
				staleCaseThreshold, staleCasesTo, staleCasesCc, alertsEnabled),
			To: staleCasesTo,
			Cc: staleCasesCc,
		},
		// Products that deployment nodes report but the product catalogue does not
		// know -- ServiceNow emailed one fixed address on every such miss. Daily, to
		// this task's SUB_CRON_RECIPIENTS entry; a no-op until recipients are set, and
		// silent on a day with nothing to report. The window matches the daily
		// schedule, so change both together.
		{
			Name:     unresolvedProductsTaskName,
			Schedule: scheduleFor(scheduleOverrides, unresolvedProductsTaskName, "0 6 * * *"),
			Handler: unresolvedproducts.SendDigest(unresolvedProductsClient, unresolvedProductsEmail,
				24*time.Hour, unresolvedProductsTo, unresolvedProductsCc, alertsEnabled, time.Now),
			To: unresolvedProductsTo,
			Cc: unresolvedProductsCc,
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
			Handler:  opencases.SendReport(entityCasesClient, openCasesEmail, openCasesTo, openCasesCc, alertsEnabled),
			To:       openCasesTo,
			Cc:       openCasesCc,
		},
		// Emails the weekly migration case scan — see internal/migrationscan's
		// own doc comment. The audience is the mail groups of the teams
		// entity-service is configured with, plus this task's own
		// SUB_CRON_RECIPIENTS entry; an empty audience fails the run so it is
		// noticed. Default schedule is Sunday 22:30 UTC, which is Monday
		// 04:00 in Colombo, the hour the scan has always arrived; override
		// via SUB_CRON_SCHEDULES if needed.
		//
		// To and Cc are left nil DELIBERATELY: the engine mails task.To plus
		// ALERT_RECIPIENTS on every failed attempt, and a failed period is
		// retried on every tick for a week. The report audience is several
		// team mail groups; an entity-service outage must not send them an
		// hourly FAILED notice. Failures go to ALERT_RECIPIENTS alone.
		{
			Name:     migrationCaseReportTaskName,
			Schedule: scheduleFor(scheduleOverrides, migrationCaseReportTaskName, "30 22 * * 0"),
			Handler: migrationscan.SendReport(migrationReportClient, migrationCaseEmail,
				migrationCaseOlderThanDays, migrationCaseTo, migrationCaseCc, migrationCaseLinkBase, alertsEnabled),
		},
		// The CS Worklogs reports. Both default schedules are the times
		// ServiceNow actually sends at, expressed in UTC because these
		// cron strings are UTC: WeeklyReportAutomation V2 triggers Monday
		// 16:00 UTC, which is 21:30 Asia/Colombo and the timestamp on the
		// delivered weekly mail; the daily one arrives 09:25 Colombo,
		// which is 03:55 UTC. The REPORTED range is derived in
		// Asia/Colombo regardless -- see worklogsreport.ReportTimezone --
		// because at 03:55 UTC the two zones disagree about which day
		// "yesterday" is.
		//
		// Recipients come from SUB_CRON_RECIPIENTS and the two lists are
		// NOT the same: the delivered mails have different audiences,
		// with group addresses in To on the daily and in Cc on the
		// weekly. Sends nothing (but still succeeds) if the To list is
		// empty.
		//
		// To and Cc are left nil DELIBERATELY. The engine mails
		// task.To plus ALERT_RECIPIENTS on every failed attempt, and a
		// failed period is retried on every tick until the next
		// firing. These two audiences are the whole CS leadership plus
		// several distribution lists, so an entity-service outage
		// would send them an hourly FAILED notice for up to a day
		// (daily) or a week (weekly). The report audience and the
		// alert audience are not the same people: the recipients go to
		// the handler, the failures go to ALERT_RECIPIENTS.
		// Query-hour utilisation, ported from ServiceNow's two flows.
		//
		// The weekly report's audience is DERIVED from the accounts it
		// reports on, so To and Cc are left nil here: failure alerts
		// go to ALERT_RECIPIENTS, and the report goes to the owners of
		// whichever accounts are breaching that week.
		//
		// The notification task also RECOMPUTES, which is what makes a
		// crossing detectable at all -- it compares the state it
		// derives against the one already stored. Hourly, so a breach
		// is noticed the same day rather than at the weekly.
		{
			Name:     queryHoursNoticeTaskName,
			Schedule: scheduleFor(scheduleOverrides, queryHoursNoticeTaskName, "15 * * * *"),
			Handler: queryhoursreport.SendUsageNotifications(queryHourClient, queryHoursNoticeEmail,
				queryHoursNoticeCc, nil, alertsEnabled),
		},
		{
			Name:     queryHoursWeeklyTaskName,
			Schedule: scheduleFor(scheduleOverrides, queryHoursWeeklyTaskName, "0 4 * * 1"),
			Handler: queryhoursreport.SendWeeklyReport(queryHourClient, queryHoursWeeklyEmail,
				queryHoursWeeklyTo, queryHoursWeeklyCc, nil, alertsEnabled),
		},
		{
			Name:     worklogsDailyTaskName,
			Schedule: worklogsDailySchedule,
			Handler: worklogsreport.SendDaily(worklogReportClient, worklogsDailyEmail,
				periodStart(worklogsDailySchedule),
				worklogsDailyTo, worklogsDailyCc, worklogRecipientSource,
				worklogCaseLinkBase, alertsEnabled),
		},
		{
			Name:     worklogsWeeklyTaskName,
			Schedule: worklogsWeeklySchedule,
			Handler: worklogsreport.SendWeekly(worklogReportClient, worklogsWeeklyEmail,
				periodStart(worklogsWeeklySchedule),
				worklogsWeeklyTo, worklogsWeeklyCc, worklogRecipientSource,
				worklogCaseLinkBase, alertsEnabled),
		},
		// The first sub-cron here that does real, per-row, multi-step,
		// partial-failure-tolerant work rather than a bulk delete or a
		// read-only report — see internal/announcementpublish's own doc
		// comments for the full design. Default schedule is tight (every 15
		// minutes) so it's ready the moment this component's own Choreo
		// Scheduled Task trigger cadence is tightened to match — actual
		// publish promptness is bounded by that shared trigger frequency,
		// not by this schedule string alone (see this component's own
		// CLAUDE.md, "The core mechanism").
		{
			Name:     publishScheduledAnnouncementsTaskName,
			Schedule: scheduleFor(scheduleOverrides, publishScheduledAnnouncementsTaskName, "*/15 * * * *"),
			Handler:  announcementpublish.PublishDue(announcementPublishClient),
			To:       publishScheduledAnnouncementsTo,
			Cc:       publishScheduledAnnouncementsCc,
		},
		// The two outage emails (internal-stakeholder notification and SRE
		// outage communication) used to be sub-crons here. They moved to
		// entity-service's outage notice drainer and csm-notification-service,
		// which send them seconds after the change rather than on this tick.
	}
	tasks = append(tasks, regressionTasks...)

	// Registered only when enabled, rather than registered-and-inert, so the
	// ledger shows no run at all for a task that is switched off -- an inert
	// task recording successful no-op runs every tick would read, months from
	// now, as evidence the port was working.
	//
	// Every 5 minutes: this is the only task whose output is public and
	// time-critical, and its real promptness is bounded by this component's
	// Choreo trigger cadence anyway (see CLAUDE.md, "The core mechanism").
	if cloudStatusEnabled {
		tasks = append(tasks, registry.Task{
			Name:     cloudStatusTaskName,
			Schedule: scheduleFor(scheduleOverrides, cloudStatusTaskName, "*/5 * * * *"),
			Handler:  cloudstatus.DeliverDue(cloudStatusClient, cloudStatusWebhook),
			To:       cloudStatusTo,
			Cc:       cloudStatusCc,
		})
	}

	// Recomputes every committed service offering's uptime and rewrites
	// service_availability -- the Go port of ServiceNow's "Calculate
	// Availability" job, which has run nightly since 2022 and whose
	// 212,904 rows the Cloud Status Dashboard reads on every page load.
	//
	// *** THIS IS THE PRODUCER FOR THREE ALREADY-PORTED ENDPOINTS. ***
	// /cloud-status/monitors, /availabilities and /availability-history
	// all read that table, and nothing in Postgres has ever written it:
	// csm-sync-service mirrors ServiceNow's output. At cutover the
	// dashboard's figures would simply stop advancing, with no error
	// anywhere, because reading a table nobody updates looks exactly
	// like reading a table where nothing happened.
	//
	// *** 03:00 UTC, NOT 10:00. *** ServiceNow fires at 10:00 UTC and
	// computes the PREVIOUS day under the legacy engine. v2 computes
	// TODAY, continuously, so the hour no longer carries that meaning
	// and the only thing it needs to be is quiet. 03:00 UTC is 08:30 in
	// Asia/Colombo -- before the working day, after the overnight
	// batch window.
	//
	// Registering this is a paired change with disabling ServiceNow's
	// "Calculate Availability" job: two writers on one table, keyed
	// differently, would double every subject's rows. Hence
	// AVAILABILITY_RECALC_ENABLED, default false (see the client above).
	if availabilityEnabled {
		tasks = append(tasks, registry.Task{
			Name:     availabilityTaskName,
			Schedule: scheduleFor(scheduleOverrides, availabilityTaskName, "0 3 * * *"),
			Handler:  availability.RecalculateAvailability(availabilityClient),
			To:       availabilityTo,
			Cc:       availabilityCc,
		})
	}

	// Rebuilds the daily and monthly usage rows from the hourly and daily ones --
	// the Go replacement for ServiceNow's UsageAggregator. The arithmetic is
	// set-based SQL in entity-service (migration 0245); these tasks only trigger it.
	//
	// *** OFF UNLESS USAGE_ROLLUP_ENABLED=true. *** ServiceNow's aggregation jobs
	// keep writing u_daily_usage_summary and u_monthly_usage_count, which
	// csm-sync-service mirrors into the same Postgres tables, so two writers would
	// fight over those rows (and csm-sync's delete_sync would remove rows
	// ServiceNow does not have). Turn it on in the same change that switches the
	// ServiceNow jobs and those csm-sync jobs off.
	//
	// Daily at 00:30 UTC rebuilds the last USAGE_ROLLUP_DAILY_LOOKBACK_DAYS days
	// (default 2) before today; ingestion keeps today's rows current by itself.
	// Monthly at 01:30 UTC rebuilds last month and the current one. Both are
	// idempotent, so a repeated or missed run is safe. Each handler's clock is its
	// scheduled period's start, not the wall clock: a failed run retried after
	// midnight (or a month boundary) still rebuilds the days (or months) it was for.
	if usageRollupEnabled {
		usageRollupLookbackDays := int(envDays("USAGE_ROLLUP_DAILY_LOOKBACK_DAYS", 2).Hours() / 24)
		usageRollupDailySchedule := scheduleFor(scheduleOverrides, usageRollupDailyTaskName, "30 0 * * *")
		usageRollupMonthlySchedule := scheduleFor(scheduleOverrides, usageRollupMonthlyTaskName, "30 1 * * *")
		tasks = append(tasks,
			registry.Task{
				Name:     usageRollupDailyTaskName,
				Schedule: usageRollupDailySchedule,
				Handler:  usagerollup.DailyRollup(usageRollupClient, usageRollupLookbackDays, periodStart(usageRollupDailySchedule)),
				To:       usageRollupDailyTo,
				Cc:       usageRollupDailyCc,
			},
			registry.Task{
				Name:     usageRollupMonthlyTaskName,
				Schedule: usageRollupMonthlySchedule,
				Handler:  usagerollup.MonthlyRollup(usageRollupClient, periodStart(usageRollupMonthlySchedule)),
				To:       usageRollupMonthlyTo,
				Cc:       usageRollupMonthlyCc,
			},
		)
	}

	// The three rota reminder tasks, on the days and times the rota's owners
	// asked for (Colombo has no DST; TZ=UTC-dependent like every schedule
	// here, see internal/schedule.PeriodKey):
	//   - rota_weekday_reminder, 09:30 UTC = 15:00 Colombo, Mon-Fri: the next
	//     working day's windows (Friday announces Monday), one email each with
	//     its calendar invitation;
	//   - rota_weekend_notice, 14:30 UTC = 20:00 Colombo, Sun-Thu: once the
	//     coming weekend is fully marked, its summary and one invitation per
	//     team per day, sent once; until then the rota admins are asked every
	//     evening;
	//   - rota_weekend_reminder, 09:30 UTC = 15:00 Colombo, Fri: the weekend
	//     summary again, the invitations as updates, and cancellations.
	// A task's SUB_CRON_RECIPIENTS to/cc get its failure alerts; they are
	// copied on its emails only with ROTA_REMINDER_COPY_TASK_RECIPIENTS=true.
	if rotaRemindersEnabled {
		family := envString("ROTA_REMINDER_FAMILY", "CRE")
		copyTaskRecipients := envBool("ROTA_REMINDER_COPY_TASK_RECIPIENTS", false)
		// Invitations need an organizer: the address the emails come from.
		organizer := strings.TrimSpace(os.Getenv("EMAIL_FROM_ADDRESS"))
		invites := envBool("ROTA_REMINDER_INVITES_ENABLED", true)
		if invites && !rotareminder.ValidInviteAddress(organizer) {
			slog.Warn("rota reminder calendar invitations are off: EMAIL_FROM_ADDRESS is not set or is not a plain email address, and an invitation needs an organizer")
			invites = false
		}
		scheduleURL := strings.TrimSpace(os.Getenv("ROTA_REMINDER_SCHEDULE_URL"))
		if scheduleURL != "" && !rotareminder.ValidScheduleURL(scheduleURL) {
			slog.Warn("ROTA_REMINDER_SCHEDULE_URL is not an http(s) URL; the rota reminders go without the Team Schedule link")
			scheduleURL = ""
		}
		weekendRequired := parseRequired("ROTA_REMINDER_WEEKEND_REQUIRED",
			envString("ROTA_REMINDER_WEEKEND_REQUIRED", "CRE_WEEKEND=3,CRE_WEEKEND_NIGHT=1"))
		// Each task's config carries its schedule's period (the run's rota
		// days are counted from it, so a retry announces what the scheduled
		// run would have) and the engine's failure audience for it (a run in
		// which only some emails failed reports them itself; see
		// rotareminder.Config.AlertTo).
		// A rotation worked on another clock keeps its hours there: the
		// Americas Weekend is 08:00-18:00 in Los Angeles, whatever span of
		// IST that is.
		rotaWindows, err := rotareminder.ParseWindows(envString("ROTA_REMINDER_WINDOWS", "CRE_WEEKEND_NIGHT=08:00-18:00 America/Los_Angeles,CRE_WEEKEND_NIGHT_OC=08:00-18:00 America/Los_Angeles"))
		if err != nil {
			slog.Error("ROTA_REMINDER_WINDOWS is malformed; refusing to start", "err", err)
			os.Exit(1)
		}
		rotaConfig := func(task, sched string, rotations map[string]string, to, cc []string) rotareminder.Config {
			var copied []string
			if copyTaskRecipients {
				copied = append(append(copied, to...), cc...)
			}
			return rotareminder.Config{
				Family:         family,
				Rotations:      rotations,
				Copy:           copied,
				RedirectTo:     splitComma(os.Getenv("ROTA_REMINDER_REDIRECT_TO")),
				ScheduleURL:    scheduleURL,
				Invites:        invites,
				OrganizerEmail: organizer,
				Required:       weekendRequired,
				Windows:        rotaWindows,
				Settings:       teamScheduleClient,
				EmailsEnabled:  alertsEnabled,
				Zone:           rotaZone,
				Now:            time.Now,
				Anchor:         periodStart(sched),
				Task:           task,
				AlertTo:        append(append([]string{}, to...), alertRecipients...),
				AlertCc:        cc,
			}
		}
		weekdayRotations := parseRotations("ROTA_REMINDER_WEEKDAY_SHIFTS",
			envString("ROTA_REMINDER_WEEKDAY_SHIFTS", "CRE_MORNING=Morning,CRE_MORNING_OC=On-call,CRE_EVENING=Evening"))
		weekendRotations := parseRotations("ROTA_REMINDER_WEEKEND_SHIFTS",
			envString("ROTA_REMINDER_WEEKEND_SHIFTS", "CRE_WEEKEND=LK Weekend,CRE_WEEKEND_NIGHT=Americas Weekend,CRE_WEEKEND_NIGHT_OC=LK On-call"))
		rotaWeekdaySchedule := scheduleFor(scheduleOverrides, rotaWeekdayTaskName, "30 9 * * 1-5")
		rotaWeekendSchedule := scheduleFor(scheduleOverrides, rotaWeekendTaskName, "30 14 * * 0-4")
		rotaWeekendReminderSchedule := scheduleFor(scheduleOverrides, rotaWeekendReminderTaskName, "30 9 * * 5")
		// "Send now" requests are picked up at every run: the schedule is finer
		// than the driver, so it is due at each tick.
		rotaManualSchedule := scheduleFor(scheduleOverrides, rotaManualTaskName, "*/5 * * * *")
		tasks = append(tasks,
			registry.Task{
				Name:     rotaWeekdayTaskName,
				Schedule: rotaWeekdaySchedule,
				Handler: rotareminder.WeekdayReminder(teamScheduleClient, emailClient,
					rotaConfig(rotaWeekdayTaskName, rotaWeekdaySchedule, weekdayRotations, rotaWeekdayTo, rotaWeekdayCc)),
				To: rotaWeekdayTo,
				Cc: rotaWeekdayCc,
			},
			registry.Task{
				Name:     rotaWeekendTaskName,
				Schedule: rotaWeekendSchedule,
				Handler: rotareminder.WeekendNotice(teamScheduleClient, teamScheduleClient, emailClient,
					rotaConfig(rotaWeekendTaskName, rotaWeekendSchedule, weekendRotations, rotaWeekendTo, rotaWeekendCc)),
				To: rotaWeekendTo,
				Cc: rotaWeekendCc,
			},
			registry.Task{
				Name:     rotaWeekendReminderTaskName,
				Schedule: rotaWeekendReminderSchedule,
				Handler: rotareminder.WeekendReminder(teamScheduleClient, teamScheduleClient, emailClient,
					rotaConfig(rotaWeekendReminderTaskName, rotaWeekendReminderSchedule, weekendRotations, rotaWeekendReminderTo, rotaWeekendReminderCc)),
				To: rotaWeekendReminderTo,
				Cc: rotaWeekendReminderCc,
			},
			registry.Task{
				Name:     rotaManualTaskName,
				Schedule: rotaManualSchedule,
				Handler: rotareminder.ManualReminder(teamScheduleClient, teamScheduleClient, teamScheduleClient, emailClient,
					rotaConfig(rotaManualTaskName, rotaManualSchedule, weekdayRotations, rotaManualTo, rotaManualCc),
					rotaConfig(rotaManualTaskName, rotaManualSchedule, weekendRotations, rotaManualTo, rotaManualCc)),
				To: rotaManualTo,
				Cc: rotaManualCc,
			},
		)
	}

	// Daily at 08:00 (TZ=UTC-dependent, like every schedule here), the time the
	// ServiceNow flow this replaces ran. The limit is per UTC day and enforced by
	// entity-service from its audit table, so the engine retrying a failed run
	// the same day cannot exceed it. A partial failure is retried the next tick:
	// tickets already advanced are no longer due, so only what is left is done.
	if autoclosureEnabled {
		autoclosureCfg := autoclosure.RunConfig{
			IntervalDays: envInt("AUTOCLOSURE_INTERVAL_DAYS", 7),
			DailyLimit:   envInt("AUTOCLOSURE_DAILY_LIMIT", 500),
			// Solution Proposed on its own first, then Awaiting Info: set
			// AUTOCLOSURE_STATES=solution_proposed until the first has been watched.
			States:    splitComma(os.Getenv("AUTOCLOSURE_STATES")),
			Templates: autoclosureTemplates(),
			// AUTOCLOSURE_DRY_RUN=true: decide and log, change nothing. Use it to see
			// what the first real run would do to a large backlog.
			DryRun: envBool("AUTOCLOSURE_DRY_RUN", false),
		}
		// A value entity-service would reject on every call must stop startup, not
		// fail and alert every day while no ticket is processed.
		if err := autoclosureCfg.Validate(); err != nil {
			slog.Error("invalid AUTOCLOSURE_* configuration; refusing to start", "err", err)
			os.Exit(1)
		}
		// A tryout runs the handler once, outside the ledger, against the real
		// entity-service: here that would comment on and close real tickets. Only the
		// debug recipients are protected from the alert email, not the tickets, so a
		// tryout of this task is always a dry run.
		if strings.TrimSpace(os.Getenv("TRYOUT_TASK")) == autoclosureTaskName && !autoclosureCfg.DryRun {
			slog.Warn("TRYOUT_TASK is case_autoclosure: running as a dry run, nothing will be changed; set AUTOCLOSURE_DRY_RUN=true explicitly to silence this")
			autoclosureCfg.DryRun = true
		}
		tasks = append(tasks, registry.Task{
			Name:     autoclosureTaskName,
			Schedule: scheduleFor(scheduleOverrides, autoclosureTaskName, "0 8 * * *"),
			Handler:  autoclosure.Run(autoclosureClient, autoclosureCfg),
			To:       autoclosureTo,
			Cc:       autoclosureCc,
		})
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
	// Rota reminders always have recipients (the people on the shift), so
	// turning them on with email unconfigured is the same misconfiguration.
	if alertsEnabled && emailBaseURL == "" && rotaRemindersEnabled {
		slog.Error("ROTA_REMINDERS_ENABLED is true but EMAIL_BASE_URL is not set; refusing to start since no reminder could be sent")
		os.Exit(1)
	}
	// The product regression reports keep their audience out of task.To (see
	// their registration above), so it is checked here by name instead.
	if alertsEnabled && emailBaseURL == "" && (len(alertRecipients) > 0 || len(tasksWithRecipients) > 0 || len(regressionTasksWithRecipients) > 0) {
		slog.Error("failure alert recipients are configured but EMAIL_BASE_URL is not; refusing to start since those alerts could never actually send",
			"alertRecipientsSet", len(alertRecipients) > 0, "tasksWithOwnRecipients", tasksWithRecipients,
			"reportTasksWithOwnRecipients", regressionTasksWithRecipients)
		os.Exit(1)
	}

	// A tryout runs one named task's handler once, right now, outside its
	// schedule and without touching the ledger, so the run is neither
	// recorded as the period's success nor retried. It exists to see a
	// report land -- which is why it refuses to run unless that task has
	// debug recipients: a tryout is for checking, and checking must not
	// reach the real audience. The process exits once the handler returns;
	// the engine never ticks. In Choreo: add the task's "debug" list to
	// SUB_CRON_RECIPIENTS, set TRYOUT_TASK, run the task once by hand, read
	// the log and the debug inbox, then remove both.
	if tryout := strings.TrimSpace(os.Getenv("TRYOUT_TASK")); tryout != "" {
		runTryout(tasks, tryout, debugFor(recipientOverrides, tryout), alertsEnabled)
		return
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

// parseStringMap decodes a flat JSON object of string values, the same shape
// and the same failure philosophy as parseSubCronSchedules: a malformed value
// is logged and treated as empty rather than stopping the component.
//
// The consequences differ per caller and are handled by the caller, not here.
// An unparseable CLOUD_STATUS_WEBHOOK_URLS leaves every cloud unroutable, and
// each undeliverable webhook is then recorded with a reason and surfaces in
// the failure alert -- loud, but not a crash loop, and every OTHER scheduled
// task in this component keeps running.
func parseStringMap(name, raw string) map[string]string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var out map[string]string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		slog.Error("ignoring malformed configuration value; treating it as unset", "key", name, "err", err)
		return nil
	}
	return out
}

// autoclosureTemplates reads AUTOCLOSURE_TEMPLATES. Unlike the other maps here,
// a malformed value stops startup: treating it as unset would silently post the
// built-in wording to customers instead of the wording the operator meant.
func autoclosureTemplates() map[string]string {
	raw := os.Getenv("AUTOCLOSURE_TEMPLATES")
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var out map[string]string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		slog.Error("AUTOCLOSURE_TEMPLATES is not a JSON object of strings; refusing to start", "err", err)
		os.Exit(1)
	}
	return out
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
	// Debug, when set on a report-style task, puts that report in debug
	// mail mode -- see notify.ForReport. Ignored on a task that sends no
	// report of its own. Also what TRYOUT_TASK requires.
	Debug []string `json:"debug"`
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

// debugFor returns a task's "debug" list from SUB_CRON_RECIPIENTS, trimmed
// and blank-free; nil when the task has none.
func debugFor(overrides map[string]subCronRecipients, taskName string) []string {
	var out []string
	for _, a := range overrides[taskName].Debug {
		if a = strings.TrimSpace(a); a != "" {
			out = append(out, a)
		}
	}
	return out
}

// tasksWithDebugRecipients names every task whose SUB_CRON_RECIPIENTS entry
// carries a debug list, sorted, for the startup warning.
func tasksWithDebugRecipients(overrides map[string]subCronRecipients) []string {
	var names []string
	for name := range overrides {
		if len(debugFor(overrides, name)) > 0 {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// migrationCaseMaxOlderThanDays mirrors entity-service's own cap on the
// scan's olderThanDays (ten years); see its MigrationCaseReportMaxOlderThanDays.
const migrationCaseMaxOlderThanDays = 3650

// runTryout runs the task named name once and exits the process: 0 when the
// handler returned nil, 1 when it failed or when the tryout was refused.
// It is refused unless the task has debug recipients, since the only thing
// a tryout must never do is send a real report to real people on
// somebody's say-so; and it warns, rather than refuses, when
// ALERTS_ENABLED is false, because every report task then succeeds without
// sending anything, which is a confusing way to learn nothing.
func runTryout(tasks []registry.Task, name string, debugRecipients []string, emailsEnabled bool) {
	if len(debugRecipients) == 0 {
		slog.Error("TRYOUT_TASK is set but the task has no \"debug\" list in SUB_CRON_RECIPIENTS; refusing: a tryout sends to the debug recipients only",
			"task", name)
		os.Exit(1)
	}
	var names []string
	var task *registry.Task
	for i := range tasks {
		names = append(names, tasks[i].Name)
		if tasks[i].Name == name {
			task = &tasks[i]
		}
	}
	if task == nil {
		slog.Error("TRYOUT_TASK names no registered task", "task", name, "registered", names)
		os.Exit(1)
	}
	if !emailsEnabled {
		slog.Warn("tryout with ALERTS_ENABLED=false: report tasks send nothing in that mode, so expect no email", "task", name)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	slog.Warn("tryout: running the task once, outside its schedule and the ledger",
		"task", name, "debugRecipients", len(debugRecipients))
	start := time.Now()
	if err := task.Handler(ctx); err != nil {
		slog.Error("tryout failed", "task", name, "elapsed", time.Since(start).String(), "err", err)
		os.Exit(1)
	}
	slog.Info("tryout complete", "task", name, "elapsed", time.Since(start).String())
}

// periodStart returns the scheduled firing time a run of expr made now belongs
// to: the period key the engine claims it under (schedule.PeriodKey), and so
// the same for every retry of that run. The schedule is checked at startup, so
// the error is not expected; now is the fallback all the same.
func periodStart(expr string) func() time.Time {
	return func() time.Time {
		now := time.Now()
		if p, err := schedule.PeriodKey(expr, now); err == nil {
			return p
		}
		return now
	}
}

// parseRotations reads a rota reminder's shift list, "CODE=Name,CODE=Name": the
// shift codes the task announces and the rotation name the email shows for
// each. A malformed entry stops startup: a reminder that silently drops a
// rotation is worse than one that never starts.
func parseRotations(key, raw string) map[string]string {
	out := map[string]string{}
	for _, entry := range splitComma(raw) {
		code, name, ok := strings.Cut(entry, "=")
		code, name = strings.TrimSpace(code), strings.TrimSpace(name)
		if !ok || code == "" || name == "" {
			slog.Error("malformed rota reminder shift list; expected CODE=Name,CODE=Name", "key", key, "entry", entry)
			os.Exit(1)
		}
		out[code] = name
	}
	if len(out) == 0 {
		slog.Error("empty rota reminder shift list", "key", key)
		os.Exit(1)
	}
	return out
}

// parseRequired reads the weekend's required headcounts, "CODE=N,CODE=N": how
// many people each shift needs on each weekend day before the weekend is
// announced. A malformed entry stops startup, like the shift lists.
func parseRequired(key, raw string) map[string]int {
	out := map[string]int{}
	for _, entry := range splitComma(raw) {
		code, n, ok := strings.Cut(entry, "=")
		code = strings.TrimSpace(code)
		count, err := strconv.Atoi(strings.TrimSpace(n))
		if !ok || code == "" || err != nil || count < 0 {
			slog.Error("malformed rota reminder headcount list; expected CODE=N,CODE=N", "key", key, "entry", entry)
			os.Exit(1)
		}
		out[code] = count
	}
	return out
}

// envString returns the given environment variable, or def if unset or blank.
func envString(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
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

// envInt returns the given environment variable parsed as a non-negative
// integer, or def if unset or malformed (logged, not fatal: a typo in a tuning
// knob must not stop every other task in this component from running).
func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n < 0 {
		slog.Warn("environment variable is not a valid non-negative integer; using default", "key", key, "value", v, "default", def)
		return def
	}
	return n
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
