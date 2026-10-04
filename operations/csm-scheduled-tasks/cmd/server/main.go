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
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/announcementpublish"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/availability"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/cloudstatus"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/engine"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/entitycases"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/entityhttp"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/housekeeping"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/ledger"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/notify"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/opencases"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/outagecomm"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/outagecommtask"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/outagenotify"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/outagenotifytask"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/registry"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/reportguard"
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

	// One authenticated transport for every entity-service client below:
	// they share the credentials and the scopes, so they share the token —
	// one client-credentials grant per invocation instead of one per client.
	// Each client still applies its own request timeout on top.
	entityTransport, err := entityhttp.NewTransport(entityhttp.Credentials{
		TokenURL:     oauthTokenURL,
		ClientID:     oauthClientID,
		ClientSecret: oauthClientSecret,
		Scopes:       entityServiceScopes,
	})
	if err != nil {
		slog.Error("failed to construct entity-service transport", "err", err)
		os.Exit(1)
	}

	ledgerClient, err := ledger.NewClient(ledger.Config{
		BaseURL:      entityServiceBaseURL,
		TokenURL:     oauthTokenURL,
		ClientID:     oauthClientID,
		ClientSecret: oauthClientSecret,
		Scopes:       entityServiceScopes,
		Transport:    entityTransport,
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
		Transport:    entityTransport,
	})
	if err != nil {
		slog.Error("failed to construct entity-service case-search client", "err", err)
		os.Exit(1)
	}

	// Outage internal-stakeholder notification. Its own narrow client: the
	// sweep is a completely different question from case search or the
	// query-hour recompute, and shares no endpoint with either.
	outageNotifyClient, err := outagenotify.NewClient(outagenotify.Config{
		BaseURL:      entityServiceBaseURL,
		TokenURL:     oauthTokenURL,
		ClientID:     oauthClientID,
		ClientSecret: oauthClientSecret,
		Scopes:       entityServiceScopes,
		Transport:    entityTransport,
	})
	if err != nil {
		slog.Error("failed to construct entity-service outage-notification client", "err", err)
		os.Exit(1)
	}

	// Outage COMMUNICATION -- the SRE-facing declaration/resolution pair,
	// and a different legacy workflow from the stakeholder notifier above.
	// Its own client because it is its own endpoint; the two sweeps answer
	// different questions and will diverge.
	outageCommClient, err := outagecomm.NewClient(outagecomm.Config{
		BaseURL:      entityServiceBaseURL,
		TokenURL:     oauthTokenURL,
		ClientID:     oauthClientID,
		ClientSecret: oauthClientSecret,
		Scopes:       entityServiceScopes,
		Transport:    entityTransport,
	})
	if err != nil {
		slog.Error("failed to construct entity-service outage-communication client", "err", err)
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
		Transport:    entityTransport,
	})
	if err != nil {
		slog.Error("failed to construct entity-service announcement-publish client", "err", err)
		os.Exit(1)
	}

	// Cloud status: a fifth entity-service client, and the first outbound
	// integration this component has -- see internal/cloudstatus's package
	// doc. cloudStatusEnabled is the double-fire guard: the legacy cloud
	// status notification workflow is still live, and two systems
	// posting the same event to a PUBLIC status page is the most visible
	// possible way to get a cutover wrong. It stays false until that workflow is
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
			Transport:    entityTransport,
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
	//
	// Its own transport, not entityTransport: EMAIL_SCOPES is a separate
	// scope set, so it needs its own token — the second (and last) grant per
	// invocation.
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

	const outageNotifyTaskName = "outage_internal_notification"
	outageNotifyTo, outageNotifyCc := recipientsFor(recipientOverrides, outageNotifyTaskName)

	const availabilityTaskName = "availability_recalculation"
	availabilityTo, availabilityCc := recipientsFor(recipientOverrides, availabilityTaskName)

	const outageCommTaskName = "outage_communication"
	outageCommTo, outageCommCc := recipientsFor(recipientOverrides, outageCommTaskName)

	const staleCasesTaskName = "stale_cases_report"
	staleCasesTo, staleCasesCc := recipientsFor(recipientOverrides, staleCasesTaskName)
	// Fixed, not env-configurable — unlike HOUSEKEEPING_RETENTION_DAYS, there's
	// no operational reason to tune this per deployment today. Revisit as an
	// env var (mirroring envDays("HOUSEKEEPING_RETENTION_DAYS", 30)'s shape)
	// if that changes.
	const staleCaseThreshold = 30 * 24 * time.Hour

	const openCasesTaskName = "open_cases_report"
	openCasesTo, openCasesCc := recipientsFor(recipientOverrides, openCasesTaskName)

	const publishScheduledAnnouncementsTaskName = "publish_scheduled_announcements"
	publishScheduledAnnouncementsTo, publishScheduledAnnouncementsCc := recipientsFor(recipientOverrides, publishScheduledAnnouncementsTaskName)

	const cloudStatusTaskName = "cloud_status_webhooks"
	cloudStatusTo, cloudStatusCc := recipientsFor(recipientOverrides, cloudStatusTaskName)

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
				reportguard.New(ledgerClient, staleCasesTaskName),
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
			Handler: opencases.SendReport(entityCasesClient, emailClient,
				reportguard.New(ledgerClient, openCasesTaskName),
				openCasesTo, openCasesCc, alertsEnabled),
			To: openCasesTo,
			Cc: openCasesCc,
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
		// The internal-stakeholder outage notice.
		//
		// Every 5 minutes, not hourly: an outage declaration that arrives an
		// hour late has missed the event it is announcing. The sweep is cheap
		// — only outages opted into notification and not yet resolved — so the
		// cadence costs little.
		//
		// SUB_CRON_RECIPIENTS here is the REPORT AUDIENCE, not failure alerts:
		// an empty `to` skips the sweep entirely, which matters because
		// sweeping marks decisions as sent and would consume notices nobody
		// receives.
		//
		// NOT yet paired with retiring the legacy workflow. Registering this
		// is a paired change with turning off the legacy internal-stakeholder
		// outage e-mail, per the double-fire rule.
		{
			Name:     outageNotifyTaskName,
			Schedule: scheduleFor(scheduleOverrides, outageNotifyTaskName, "*/5 * * * *"),
			Handler: outagenotifytask.SendNotices(
				outageNotifyClient, emailClient, outageNotifyTo, outageNotifyCc, alertsEnabled,
			),
			To: outageNotifyTo,
			Cc: outageNotifyCc,
		},
		// The SECOND outage notifier. Same cadence, different flow: this one
		// announces an outage to the SRE group and then announces its
		// resolution, where the task above mails internal stakeholders a
		// one-line notice.
		//
		// *** IT IS SAFE TO DEPLOY BEFORE IT IS CONFIGURED. *** With no
		// SUB_CRON_RECIPIENTS entry the `to` list is empty, and the handler
		// returns before it sweeps -- so no email goes out AND no
		// communication-log row is written. That ordering matters: sweeping
		// with nowhere to deliver would mark outages as announced to nobody
		// and they would never be announced again.
		//
		// It is also inert until the upstream data mirror exposes
		// outage.outage_communication:
		// without that column the repository degrades to "nothing to send".
		{
			Name:     outageCommTaskName,
			Schedule: scheduleFor(scheduleOverrides, outageCommTaskName, "*/5 * * * *"),
			Handler: outagecommtask.SendCommunications(
				outageCommClient, emailClient, outageCommTo, outageCommCc, alertsEnabled,
			),
			To: outageCommTo,
			Cc: outageCommCc,
		},
	}

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

	// DRIVER_INTERVAL is the one value here that must match an external
	// setting (the Choreo trigger), and everything retry-related is derived
	// from it: a task's default retry backoff and the ledger's orphan
	// window. A schedule tighter than the driver can never be honoured, and
	// worse, a failed period of such a task is superseded by its next period
	// before its retry ever comes due — so the mismatch is fatal at startup
	// rather than a silently wrong retry policy in production.
	if err := engine.ValidateCadence(tasks, driverInterval, time.Now()); err != nil {
		slog.Error("DRIVER_INTERVAL is incompatible with the registered schedules; refusing to start",
			"driverInterval", driverInterval.String(), "err", err)
		os.Exit(1)
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
	// How many handlers may run at once. The engine starts tasks shortest-
	// interval first regardless, so the five-minute outage/status tasks are
	// never queued behind a slow daily one; a second worker additionally
	// keeps a single slow handler (one announcement auto-publish can take
	// minutes) from holding the rest of the tick. 1 makes the tick strictly
	// sequential if that ever proves necessary.
	eng.Concurrency = envInt("TASK_CONCURRENCY", 2)

	// No app-level execution timeout here — Choreo's own Scheduled Task
	// execution-time limit already bounds how long one invocation can run.
	// signal.NotifyContext instead cancels this context the moment Choreo
	// sends SIGTERM (whether that's from its own timeout firing, a
	// redeploy, or a manual stop), so the in-flight handler aborts promptly
	// and no further task is claimed. The engine's own record-back and
	// alert calls do NOT run on this context — they run on a short,
	// detached one (engine.bookkeepingContext), so the interrupted run is
	// still recorded as failed and the alert still goes out.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// The exit status is the one signal the scheduler's own run history can
	// see. A tick in which any handler failed, any ledger call failed, or
	// the run was interrupted exits non-zero; "not due" and "claim denied"
	// are ordinary outcomes and exit 0. See engine.Engine.Tick.
	start := time.Now()
	if err := eng.Tick(ctx, start); err != nil {
		slog.Error("tick finished with failures; exiting non-zero so the scheduler records a failed run",
			"elapsed", time.Since(start).String(), "err", err)
		stop()
		os.Exit(1)
	}
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

// envInt returns the given environment variable parsed as a positive
// integer, or def if unset, malformed, or less than 1.
func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		slog.Warn("environment variable is not a positive integer; using default", "key", key, "value", v, "default", def)
		return def
	}
	return n
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
