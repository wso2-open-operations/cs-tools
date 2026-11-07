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

// Package timecardreport is the monthly time card report: for each configured
// project, an email to the project's contacts with the previous calendar
// month's approved effort per case. It holds the task; internal/timecards holds
// the entity-service client and wire types, and internal/notify renders them --
// the split every report sub-cron here uses.
package timecardreport

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/notify"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/timecards"
)

// TaskName is the registry (and SUB_CRON_SCHEDULES / SUB_CRON_RECIPIENTS) name
// of the task.
const TaskName = "time_card_report"

// MaxRecipients caps an explicit --to list on a manual run.
const MaxRecipients = 50

const dateLayout = "2006-01-02"

// Source is the subset of *timecards.Client this package depends on.
type Source interface {
	SearchProjects(ctx context.Context, query string) ([]timecards.Project, error)
	ProjectContacts(ctx context.Context, projectID string) ([]timecards.Contact, error)
	Report(ctx context.Context, projectID, from, to string) (timecards.Report, error)
}

// EmailSender is the subset of *notify.Client this package depends on.
type EmailSender interface {
	SendEmail(ctx context.Context, to, cc []string, subject, htmlBody string) error
}

// Config is what one run reports on and to whom.
type Config struct {
	// ProjectKeys are the projects to report on, in order. Empty means the
	// task has nothing to do (it is not an error).
	ProjectKeys []string
	// To, when non-empty, REPLACES every project's derived contact list and is
	// mailed as one message with all of them in To. Only a manual run sets it.
	To []string
	// Cc is added to every mail, derived or explicit. Only a manual run sets it.
	Cc []string
}

// ParseProjectKeys reads TIME_CARD_REPORT_PROJECT_KEYS: a comma list, spaces
// trimmed, empties and duplicates (ignoring case) dropped, order kept.
func ParseProjectKeys(raw string) []string {
	var out []string
	seen := map[string]bool{}
	for _, k := range strings.Split(raw, ",") {
		k = strings.TrimSpace(k)
		if k == "" || seen[strings.ToLower(k)] {
			continue
		}
		seen[strings.ToLower(k)] = true
		out = append(out, k)
	}
	return out
}

// PreviousMonth is the previous calendar month of period, in UTC: its first and
// last day, as plain dates. period is the task's period key (its cron firing
// time), not the clock, so every retry of one period reports the same month.
func PreviousMonth(period time.Time) (from, to time.Time) {
	p := period.UTC()
	firstOfThis := time.Date(p.Year(), p.Month(), 1, 0, 0, 0, 0, time.UTC)
	return firstOfThis.AddDate(0, -1, 0), firstOfThis.AddDate(0, 0, -1)
}

// ResolveProject finds the project whose key is key, ignoring case, among the
// results of a search that matches substrings of name, key and subscription
// type. No exact match, or more than one, is an error.
func ResolveProject(ctx context.Context, src Source, key string) (timecards.Project, error) {
	found, err := src.SearchProjects(ctx, key)
	if err != nil {
		return timecards.Project{}, fmt.Errorf("search project: %w", err)
	}
	var match []timecards.Project
	for _, p := range found {
		if strings.EqualFold(strings.TrimSpace(p.Key), key) {
			match = append(match, p)
		}
	}
	switch len(match) {
	case 0:
		return timecards.Project{}, fmt.Errorf("no project has the key %q (the search returned %d other projects)", key, len(found))
	case 1:
		return match[0], nil
	default:
		return timecards.Project{}, fmt.Errorf("%d projects have the key %q; refusing to guess", len(match), key)
	}
}

// Recipients are the contacts' addresses: every contact with a non-empty
// address, de-duplicated ignoring case, in the order given. Integration users
// (system-to-system accounts that never read mail) are left out; no other
// filter applies, as in the report this replaces. entity-service has already
// left out deactivated memberships.
func Recipients(contacts []timecards.Contact) []string {
	var out []string
	for _, c := range recipientContacts(contacts) {
		out = append(out, strings.TrimSpace(c.Email))
	}
	return out
}

// RecipientIDs returns the user ids of the contacts Recipients would mail, in
// the same order, for logging in place of their addresses. A contact with no
// linked record has no id and is left out.
func RecipientIDs(contacts []timecards.Contact) []string {
	var out []string
	for _, c := range recipientContacts(contacts) {
		if c.ID != "" {
			out = append(out, c.ID)
		}
	}
	return out
}

// recipientContacts applies Recipients' filter and de-duplication.
func recipientContacts(contacts []timecards.Contact) []timecards.Contact {
	var out []timecards.Contact
	seen := map[string]bool{}
	for _, c := range contacts {
		a := strings.TrimSpace(c.Email)
		if a == "" || c.IsCsIntegrationUser || seen[strings.ToLower(a)] {
			continue
		}
		seen[strings.ToLower(a)] = true
		out = append(out, c)
	}
	return out
}

// SendReports returns the registry.Task.Handler.
//
// periodStart is the task's period key (the cron firing the run belongs to,
// the same for every retry); the reported window is the calendar month before
// it, in UTC. With no project keys configured the handler does nothing and
// succeeds. emailsEnabled is cmd/server/main.go's ALERTS_ENABLED; when false it
// does nothing either.
//
// Each project is independent: resolve its key, find its recipients, fetch its
// report, mail it. A project with no recipients is skipped with a warning, not
// failed. A failure in one project never stops the others; the handler returns
// one error naming every project that failed.
//
// Recipients never see each other: the mail client has no Bcc, so a derived
// audience gets one mail per address. An explicit cfg.To (a manual run) is the
// operator's chosen audience and gets a single mail.
//
// *** RETRIES CAN RE-SEND. *** The handler has no memory of what it sent. When
// any project fails, the whole run is retried on a later tick (see the task's
// MaxAttempts in cmd/server/main.go) and every project is processed again, so
// contacts of the projects that had already succeeded get the mail a second
// time. A manual run writes no ledger row at all, so it can repeat a period
// that was already sent.
func SendReports(src Source, email EmailSender, periodStart func() time.Time, cfg Config, emailsEnabled bool) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		if !emailsEnabled {
			return nil
		}
		if len(cfg.ProjectKeys) == 0 {
			slog.Info("time card report has no project keys configured (TIME_CARD_REPORT_PROJECT_KEYS); nothing to do")
			return nil
		}

		from, to := PreviousMonth(periodStart())
		slog.Info("time card report",
			"from", from.Format(dateLayout), "to", to.Format(dateLayout),
			"projects", len(cfg.ProjectKeys), "explicitRecipients", len(cfg.To), "ccCount", len(cfg.Cc))

		var errs []error
		var failed []string
		for _, key := range cfg.ProjectKeys {
			if err := sendProject(ctx, src, email, key, from, to, cfg); err != nil {
				slog.Error("time card report failed for a project", "project", key, "err", err)
				errs = append(errs, fmt.Errorf("project %s: %w", key, err))
				failed = append(failed, key)
			}
		}
		if len(errs) == 0 {
			return nil
		}
		return fmt.Errorf("timecardreport: %d of %d projects failed (%s): %w",
			len(failed), len(cfg.ProjectKeys), strings.Join(failed, ", "), errors.Join(errs...))
	}
}

// sendProject reports on one project.
func sendProject(ctx context.Context, src Source, email EmailSender, key string, from, to time.Time, cfg Config) error {
	project, err := ResolveProject(ctx, src, key)
	if err != nil {
		return err
	}

	recipients := cfg.To
	var recipientIDs []string // derived audience only; an explicit --to has no ids
	if len(recipients) == 0 {
		contacts, err := src.ProjectContacts(ctx, project.ID)
		if err != nil {
			return fmt.Errorf("fetch contacts: %w", err)
		}
		recipients = Recipients(contacts)
		recipientIDs = RecipientIDs(contacts)
		if len(recipients) == 0 {
			slog.Warn("time card report project has no recipients; skipped",
				"projectId", project.ID, "contacts", len(contacts))
			return nil
		}
	}

	report, err := src.Report(ctx, project.ID, from.Format(dateLayout), to.Format(dateLayout))
	if err != nil {
		return fmt.Errorf("fetch report: %w", err)
	}
	// The window is the one this run asked for, whatever the response echoes: the
	// subject's month and the header's period come from here.
	report.From, report.To = from.Format(dateLayout), to.Format(dateLayout)
	// The report names the project as entity-service has it; the key the
	// operator configured stands in only if the response left it out.
	if report.Project.Key == "" {
		report.Project.Key = project.Key
	}
	if report.Project.Name == "" {
		report.Project.Name = project.Name
	}
	subject := notify.TimeCardReportSubject(report)
	body := notify.RenderTimeCardReport(report)

	mails := recipients
	if len(cfg.To) > 0 {
		mails = nil // one mail, below
	}
	// The figures the mail carries, in minutes, so a log line alone shows what a
	// project was sent. Counts only: no address, case or customer detail.
	summary := []any{
		"projectId", project.ID, "from", from.Format(dateLayout), "to", to.Format(dateLayout),
		"cases", len(report.Cases),
		"totalMinutes", report.Totals.TotalMinutes,
		"billableMinutes", report.Totals.BillableMinutes,
		"nonBillableMinutes", report.Totals.NonBillableMinutes,
		"patchingMinutes", report.Totals.PatchingMinutes,
	}
	slog.Info("sending the time card report",
		append(summary, "recipients", len(recipients), "recipientIds", recipientIDs, "cc", len(cfg.Cc), "mails", max(len(mails), 1))...)

	if len(cfg.To) > 0 {
		if err := email.SendEmail(ctx, cfg.To, cfg.Cc, subject, body); err != nil {
			return fmt.Errorf("send report: %w", err)
		}
		slog.Info("sent the time card report", append(summary, "mailsSent", 1, "mailsFailed", 0)...)
		return nil
	}
	sendFailures := 0
	for _, addr := range mails {
		if err := email.SendEmail(ctx, []string{addr}, cfg.Cc, subject, body); err != nil {
			sendFailures++
			slog.Error("time card report mail failed", "projectId", project.ID, "err", err)
		}
	}
	slog.Info("sent the time card report",
		append(summary, "mailsSent", len(mails)-sendFailures, "mailsFailed", sendFailures)...)
	if sendFailures > 0 {
		return fmt.Errorf("%d of %d mails failed to send", sendFailures, len(mails))
	}
	return nil
}
