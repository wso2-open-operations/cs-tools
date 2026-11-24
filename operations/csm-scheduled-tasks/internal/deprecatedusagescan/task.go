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

// Package deprecatedusagescan is the monthly deprecated product usage report sub-cron: on
// the first Monday of each month it fetches, from entity-service, the
// customers still running a Deprecated product version in an active
// Primary Production deployment, and mails them to the accounts' managers
// and technical owners and the CS leadership group, one table per region.
// It replaces the previous system's scheduled report. Built the way this component's
// own CLAUDE.md ("Per-task report emails") describes: its own template, its
// own render function, and its recipients wired through cmd/server/main.go.
package deprecatedusagescan

import (
	"context"
	"fmt"
	"log/slog"
	"net/mail"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/deprecatedusagereport"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/notify"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/schedule"
)

// TaskName is the registry (and SUB_CRON_SCHEDULES / SUB_CRON_RECIPIENTS)
// name of the task.
const TaskName = "deprecated_product_usage_report"

// DefaultSchedule is every Monday at 04:30 UTC (10:00 India Standard Time).
// Cron cannot say "the first Monday", so the task wakes every Monday and
// sends only when the date is the 1st to the 7th (see SendReport).
const DefaultSchedule = "30 4 * * 1"

// Subject is the report's mail subject, the subject the previous system's
// report had.
const Subject = "Deprecated Product Version Usage Monthly Report"

// Timezone is the zone the run date is taken in: the report's date, and
// whether today is in the month's first week. Colombo keeps India Standard
// Time's offset, year round, and is the zone every other report here uses.
const Timezone = "Asia/Colombo"

// ReportFetcher is the subset of *deprecatedusagereport.Client this package depends
// on.
type ReportFetcher interface {
	Report(ctx context.Context, asOf string) (deprecatedusagereport.Report, error)
}

// EmailSender is the subset of *notify.Client this package depends on.
type EmailSender interface {
	SendEmail(ctx context.Context, to, cc []string, subject, htmlBody string) error
}

// Options are the task's settings beyond its clients and audience.
type Options struct {
	// PortalBaseURL is the CSM portal's base URL; when set, account and
	// project names link to their pages. Empty renders plain text.
	PortalBaseURL string
	// EmailsEnabled is cmd/server/main.go's ALERTS_ENABLED. When false the
	// handler succeeds without contacting entity-service.
	EmailsEnabled bool
	// AllowedRecipientDomains, when set, limits the account managers and
	// technical owners added to the To list to these email domains (matched
	// exactly, ignoring case), e.g. ["wso2.com"]. The report names customers,
	// projects and versions, so an address from a user row outside these
	// domains is dropped rather than mailed. Empty applies no domain check;
	// a malformed address is dropped either way. The configured to/cc are not
	// filtered: they are set by whoever deploys the task.
	AllowedRecipientDomains []string
	// AnyDay sends whatever the date. A scheduled run sends only when its
	// date is a Monday in the month's first week (the default schedule fires
	// every Monday; an overridden schedule that fires on other days still
	// sends only on that Monday). A manual run-now sets this, because it is
	// asked for on purpose.
	AnyDay bool
	// Now is the clock; nil is time.Now.
	Now func() time.Time
	// Anchor returns the scheduled firing time of the run being made (its
	// period key, schedule.PeriodKey). The engine retries a failed run on
	// every tick until the next Monday, so whether this is the month's first
	// Monday is decided from here, not the clock: a first-Monday send that
	// fails and succeeds on a later day still sends. Nil (a manual run-now)
	// is the clock.
	Anchor func() time.Time
}

// SendReport returns a registry.Task.Handler that mails the report.
//
// Whether to send is decided by the run's scheduled Monday
// (Options.Anchor) in Timezone, or today for a manual run: a run for a
// Monday on the 8th or later succeeds without sending, as the schedule
// wakes the task every Monday and only the first one of the month is the
// report's. The report is dated the day it actually runs, in Timezone:
// unlike the EOL report, whose date drives its query, this one lists the
// versions' current status, so a first-Monday run retried on Thursday shows
// Thursday's catalogue and says so.
//
// The audience is the task's own SUB_CRON_RECIPIENTS entry (to: the CS
// leadership group; cc) plus every listed account's manager and technical
// owner, in To. When the task has no "to", the handler succeeds without
// contacting entity-service, as the regression reports do. A report with no
// versions still sends: a short "no customers on deprecated versions" mail to the
// configured to/cc only, so the leadership group knows the report ran.
func SendReport(reports ReportFetcher, email EmailSender, to, cc []string, opts Options) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		if !opts.EmailsEnabled {
			slog.Info("deprecated product usage report: emails are disabled (ALERTS_ENABLED); not sent")
			return nil
		}
		// An empty "to" switches the report off, unless the sender redirects
		// (debug mail mode or a run-now override): configuring only a debug
		// list is how the report is tried without its real recipients, so it
		// must still be built and sent there.
		if len(to) == 0 && !notify.Redirects(email) {
			slog.Info("deprecated product usage report has no recipients; skipping")
			return nil
		}

		scheduled := opts.Now
		if p, ok := schedule.PeriodFromContext(ctx); ok {
			// The period the engine claimed for this run.
			scheduled = func() time.Time { return p }
		} else if opts.Anchor != nil {
			scheduled = opts.Anchor
		}
		monday := runDate(scheduled)
		if !opts.AnyDay && (monday.Weekday() != time.Monday || monday.Day() > 7) {
			slog.Info("deprecated product usage report: not the first Monday of the month; not sent", "date", monday.Format("2006-01-02"))
			return nil
		}
		date := runDate(opts.Now).Format("2006-01-02")

		report, err := reports.Report(ctx, date)
		if err != nil {
			return fmt.Errorf("deprecatedusagescan: fetch report as of %s: %w", date, err)
		}

		if report.TotalVersions == 0 {
			slog.Info("sending the deprecated product usage report: no customers on deprecated versions",
				"asOf", report.AsOf, "to", len(to), "cc", len(cc))
			body := notify.RenderDeprecatedProductUsageReport(report, opts.PortalBaseURL)
			if err := email.SendEmail(ctx, to, Cc(to, cc), Subject, body); err != nil {
				return fmt.Errorf("deprecatedusagescan: send report: %w", err)
			}
			return nil
		}

		audience, dropped := Audience(report, to, opts.AllowedRecipientDomains)
		if dropped > 0 {
			slog.Warn("deprecated product usage report: account contacts left out of the recipients (malformed or outside the allowed domains)",
				"dropped", dropped)
		}
		slog.Info("sending the deprecated product usage report",
			"asOf", report.AsOf, "versions", report.TotalVersions, "accounts", report.TotalAccounts,
			"to", len(audience), "cc", len(cc))
		body := notify.RenderDeprecatedProductUsageReport(report, opts.PortalBaseURL)
		if err := email.SendEmail(ctx, audience, Cc(audience, cc), Subject, body); err != nil {
			return fmt.Errorf("deprecatedusagescan: send report: %w", err)
		}
		return nil
	}
}

// runDate is now()'s date, midnight, in Timezone.
func runDate(now func() time.Time) time.Time {
	if now == nil {
		now = time.Now
	}
	loc, err := time.LoadLocation(Timezone)
	if err != nil {
		// The zone is a constant; a missing tz database is the only way
		// this fails, and UTC is the honest fallback.
		loc = time.UTC
	}
	t := now().In(loc)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
}

// Audience is the report's To: the configured to first, then every listed
// account's manager and technical owner, each address once (compared
// ignoring case and surrounding space), in the order first seen. An account
// contact is added only when it is one plain address (net/mail) and, when
// allowedDomains is set, its domain is one of them; dropped counts the
// contacts left out. The configured to is taken as given.
func Audience(r deprecatedusagereport.Report, to []string, allowedDomains []string) (audience []string, dropped int) {
	seen := map[string]bool{}
	add := func(addr string) {
		addr = strings.TrimSpace(addr)
		key := strings.ToLower(addr)
		if addr == "" || seen[key] {
			return
		}
		seen[key] = true
		audience = append(audience, addr)
	}
	for _, a := range to {
		add(a)
	}
	contact := func(p *deprecatedusagereport.Person) {
		if p == nil || strings.TrimSpace(p.Email) == "" {
			return
		}
		if !allowedRecipient(p.Email, allowedDomains) {
			dropped++
			return
		}
		add(p.Email)
	}
	for _, region := range r.Regions {
		for _, acc := range region.Accounts {
			contact(acc.AccountManager)
			contact(acc.TechnicalOwner)
		}
	}
	return audience, dropped
}

// allowedRecipient reports whether addr is a single plain address (no
// display name, no list) and, when domains is non-empty, its domain is one
// of them, compared ignoring case.
func allowedRecipient(addr string, domains []string) bool {
	addr = strings.TrimSpace(addr)
	parsed, err := mail.ParseAddress(addr)
	if err != nil || parsed.Name != "" || !strings.EqualFold(parsed.Address, addr) {
		return false
	}
	at := strings.LastIndex(parsed.Address, "@")
	if at < 1 || at == len(parsed.Address)-1 {
		return false
	}
	if len(domains) == 0 {
		return true
	}
	domain := parsed.Address[at+1:]
	for _, d := range domains {
		if strings.EqualFold(strings.TrimSpace(d), domain) {
			return true
		}
	}
	return false
}

// Cc is cc without the addresses already in to (compared ignoring case), so
// nobody receives the mail twice.
func Cc(to, cc []string) []string {
	inTo := map[string]bool{}
	for _, a := range to {
		inTo[strings.ToLower(strings.TrimSpace(a))] = true
	}
	var out []string
	for _, a := range cc {
		key := strings.ToLower(strings.TrimSpace(a))
		if key == "" || inTo[key] {
			continue
		}
		inTo[key] = true
		out = append(out, strings.TrimSpace(a))
	}
	return out
}
