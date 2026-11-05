// Package eolusagescan is the monthly EOL product usage report sub-cron: on
// the first Monday of each month it fetches, from entity-service, the
// customers still running a product version past its EOL date in an active
// Primary Production deployment, and mails them to the accounts' managers
// and technical owners and the CS leadership group, one table per region.
// It replaces a ServiceNow scheduled report. Built the way this component's
// own CLAUDE.md ("Per-task report emails") describes: its own template, its
// own render function, and its recipients wired through cmd/server/main.go.
package eolusagescan

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/eolusagereport"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/notify"
)

// TaskName is the registry (and SUB_CRON_SCHEDULES / SUB_CRON_RECIPIENTS)
// name of the task.
const TaskName = "eol_product_usage_report"

// DefaultSchedule is every Monday at 04:30 UTC (10:00 India Standard Time).
// Cron cannot say "the first Monday", so the task wakes every Monday and
// sends only when the date is the 1st to the 7th (see SendReport).
const DefaultSchedule = "30 4 * * 1"

// Subject is the report's mail subject, the subject the ServiceNow report
// had.
const Subject = "EOL Product Version Usage Monthly Report"

// Timezone is the zone the run date is taken in: the report's date, and
// whether today is in the month's first week. Colombo keeps India Standard
// Time's offset, year round, and is the zone every other report here uses.
const Timezone = "Asia/Colombo"

// ReportFetcher is the subset of *eolusagereport.Client this package depends
// on.
type ReportFetcher interface {
	Report(ctx context.Context, asOf string) (eolusagereport.Report, error)
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
	// AnyDay sends whatever the date. The schedule fires every Monday and a
	// scheduled run sends only in the month's first week; a manual run-now
	// sets this, because it is asked for on purpose.
	AnyDay bool
	// Now is the clock; nil is time.Now.
	Now func() time.Time
}

// SendReport returns a registry.Task.Handler that mails the report.
//
// The run date is today in Timezone. A scheduled run on the 8th or later
// succeeds without sending: the schedule wakes the task every Monday and
// only the first one of the month is the report's. The report is fetched as
// of the run date, so a retry later the same day lists the same versions.
//
// The audience is the task's own SUB_CRON_RECIPIENTS entry (to: the CS
// leadership group; cc) plus every listed account's manager and technical
// owner, in To. When the task has no "to", the handler succeeds without
// contacting entity-service, as the regression reports do. A report with no
// versions still sends: a short "no customers on EOL versions" mail to the
// configured to/cc only, so the leadership group knows the report ran.
func SendReport(reports ReportFetcher, email EmailSender, to, cc []string, opts Options) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		if !opts.EmailsEnabled {
			return nil
		}
		if len(to) == 0 {
			slog.Info("EOL product usage report has no recipients; skipping")
			return nil
		}

		asOf := runDate(opts.Now)
		if !opts.AnyDay && asOf.Day() > 7 {
			slog.Info("EOL product usage report: not the first Monday of the month; not sent", "date", asOf.Format("2006-01-02"))
			return nil
		}
		date := asOf.Format("2006-01-02")

		report, err := reports.Report(ctx, date)
		if err != nil {
			return fmt.Errorf("eolusagescan: fetch report as of %s: %w", date, err)
		}

		if report.TotalVersions == 0 {
			slog.Info("sending the EOL product usage report: no customers on EOL versions",
				"asOf", report.AsOf, "to", len(to), "cc", len(cc))
			body := notify.RenderEolProductUsageReport(report, opts.PortalBaseURL)
			if err := email.SendEmail(ctx, to, Cc(to, cc), Subject, body); err != nil {
				return fmt.Errorf("eolusagescan: send report: %w", err)
			}
			return nil
		}

		audience := Audience(report, to)
		slog.Info("sending the EOL product usage report",
			"asOf", report.AsOf, "versions", report.TotalVersions, "accounts", report.TotalAccounts,
			"to", len(audience), "cc", len(cc))
		body := notify.RenderEolProductUsageReport(report, opts.PortalBaseURL)
		if err := email.SendEmail(ctx, audience, Cc(audience, cc), Subject, body); err != nil {
			return fmt.Errorf("eolusagescan: send report: %w", err)
		}
		return nil
	}
}

// runDate is today, midnight, in Timezone.
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
// ignoring case and surrounding space), in the order first seen.
func Audience(r eolusagereport.Report, to []string) []string {
	var out []string
	seen := map[string]bool{}
	add := func(addr string) {
		addr = strings.TrimSpace(addr)
		key := strings.ToLower(addr)
		if addr == "" || seen[key] {
			return
		}
		seen[key] = true
		out = append(out, addr)
	}
	for _, a := range to {
		add(a)
	}
	for _, region := range r.Regions {
		for _, acc := range region.Accounts {
			if acc.AccountManager != nil {
				add(acc.AccountManager.Email)
			}
			if acc.TechnicalOwner != nil {
				add(acc.TechnicalOwner.Email)
			}
		}
	}
	return out
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
