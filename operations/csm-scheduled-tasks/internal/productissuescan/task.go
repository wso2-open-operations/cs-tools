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

// Package productissuescan sends the weekly and monthly product issue reports:
// one all-products mail to the configured recipients and one mail per product
// lead covering that lead's products.
package productissuescan

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/notify"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/productissuereport"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/schedule"
)

// Task names, which are also the SUB_CRON_SCHEDULES / SUB_CRON_RECIPIENTS keys.
const (
	WeeklyTaskName  = "product_issue_weekly_report"
	MonthlyTaskName = "product_issue_monthly_report"

	// WeeklySchedule is Monday 09:00 IST; cron strings run in the process
	// zone, which is UTC in deployment.
	WeeklySchedule = "30 3 * * 1"
	// MonthlySchedule is the 1st at 09:00 IST.
	MonthlySchedule = "30 3 1 * *"
)

// Cadence is a report's period length.
type Cadence string

// The two cadences.
const (
	Weekly  Cadence = "week"
	Monthly Cadence = "month"
)

// TaskName is the registry name of a cadence's task.
func TaskName(c Cadence) string {
	if c == Monthly {
		return MonthlyTaskName
	}
	return WeeklyTaskName
}

// Fetcher is the subset of *productissuereport.Client this package uses.
type Fetcher interface {
	Dashboard(ctx context.Context, period, date string, products []string) (productissuereport.Dashboard, error)
	Recipients(ctx context.Context) ([]productissuereport.Lead, error)
}

// EmailSender is the subset of *notify.Client this package uses.
type EmailSender interface {
	SendEmail(ctx context.Context, to, cc []string, subject, htmlBody string) error
}

// Options configure one report task.
type Options struct {
	Cadence Cadence
	// To and Cc are the configured all-products audience. An empty To skips
	// only that audience; product leads are still mailed.
	To, Cc []string
	// LinkBase, when set, links the dashboard and cases; see notify.
	LinkBase string
	// EmailsEnabled is ALERTS_ENABLED; when false the handler does nothing.
	EmailsEnabled bool
	// Anchor is the fallback for a run outside the engine (no period in the
	// context): the scheduled firing the run belongs to.
	Anchor func() time.Time
}

// reportZone is the calendar the reports are cut in.
func reportZone() *time.Location {
	if loc, err := time.LoadLocation("Asia/Kolkata"); err == nil {
		return loc
	}
	return time.FixedZone("IST", 5*3600+30*60)
}

// ReportDate is the date to pass to entity-service for a run anchored at
// anchor: the Monday of the week before the anchor's week (weekly) or the 1st
// of the month before the anchor's month (monthly), both in Asia/Kolkata. It
// derives from the anchor, never the clock, so every retry names one period.
func ReportDate(c Cadence, anchor time.Time) string {
	a := anchor.In(reportZone())
	day := time.Date(a.Year(), a.Month(), a.Day(), 0, 0, 0, 0, time.UTC)
	if c == Monthly {
		return day.AddDate(0, -1, -(day.Day() - 1)).Format("2006-01-02")
	}
	sinceMonday := (int(day.Weekday()) + 6) % 7
	return day.AddDate(0, 0, -sinceMonday-7).Format("2006-01-02")
}

// Send returns the registry.Task.Handler.
//
// RETRIES RE-SEND: the handler has no memory of what it mailed, so a retry
// after one audience failed mails the audiences that had succeeded again (the
// task has MaxAttempts 2, as time_card_report).
func Send(src Fetcher, email EmailSender, opt Options) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		if !opt.EmailsEnabled {
			return nil
		}
		date := ReportDate(opt.Cadence, anchorOf(ctx, opt))
		period := string(opt.Cadence)
		var errs []error

		if len(opt.To) == 0 {
			slog.Info("product issue report has no configured recipients; skipping the all-products report", "task", TaskName(opt.Cadence))
		} else {
			err := sendOne(ctx, src, email, opt, period, date, nil, opt.To, opt.Cc)
			switch {
			case isNotFound(err):
				slog.Info("product issue reports are not available in entity-service (git internals off?); nothing sent", "task", TaskName(opt.Cadence))
				return nil
			case err != nil && isPermanent(err):
				slog.Error("product issue report failed for the configured recipients (permanent; not retried)", "task", TaskName(opt.Cadence), "err", err)
			case err != nil:
				slog.Error("product issue report failed for the configured recipients", "task", TaskName(opt.Cadence), "err", err)
				errs = append(errs, fmt.Errorf("all-products report: %w", err))
			}
		}

		leads, err := src.Recipients(ctx)
		switch {
		case isNotFound(err):
			slog.Info("product issue reports are not available in entity-service (git internals off?); product leads not mailed", "task", TaskName(opt.Cadence))
			return errors.Join(errs...)
		case err != nil && isPermanent(err):
			slog.Error("product leads cannot be fetched (permanent error; not retried)", "task", TaskName(opt.Cadence), "err", err)
			return errors.Join(errs...)
		case err != nil:
			errs = append(errs, fmt.Errorf("fetch product leads: %w", err))
			return errors.Join(errs...)
		}
		for _, lead := range leads {
			addr := strings.TrimSpace(lead.Email)
			if addr == "" {
				continue
			}
			var names []string
			for _, n := range lead.ProductNames {
				if n = strings.TrimSpace(n); n != "" {
					names = append(names, n)
				}
			}
			if len(names) == 0 {
				slog.Error("product lead has no usable product names; skipped (permanent)", "task", TaskName(opt.Cadence))
				continue
			}
			if productissuereport.ContainsComma(names) {
				// The products parameter is comma-separated: a name with a comma
				// would select a different set. Refuse rather than mail the
				// lead the wrong report. Permanent: a retry cannot fix it.
				slog.Error("product lead skipped: a product name contains a comma, which the products filter cannot express (permanent)", "task", TaskName(opt.Cadence))
				continue
			}
			err := sendOne(ctx, src, email, opt, period, date, names, []string{addr}, nil)
			switch {
			case isNotFound(err):
				slog.Info("product issue reports are not available in entity-service; stopping", "task", TaskName(opt.Cadence))
				return errors.Join(errs...)
			case err != nil && isPermanent(err):
				// The address is not logged, as everywhere else here.
				slog.Error("product issue report failed for a product lead (permanent; not retried)", "task", TaskName(opt.Cadence), "products", len(names), "err", err)
			case err != nil:
				slog.Error("product issue report failed for a product lead", "task", TaskName(opt.Cadence), "products", len(names), "err", err)
				errs = append(errs, fmt.Errorf("product lead report (%d products): %w", len(names), err))
			}
		}
		return errors.Join(errs...)
	}
}

// permanentError marks a failure a retry cannot fix.
type permanentError struct{ err error }

func (p *permanentError) Error() string { return p.err.Error() }
func (p *permanentError) Unwrap() error { return p.err }

// isPermanent: an explicitly permanent error, or an upstream 4xx other than
// 404, 408 and 429. Network errors, timeouts, 5xx and send failures are
// transient. A retry (MaxAttempts 2) re-sends every audience already mailed, so
// only failures a retry might fix are returned.
func isPermanent(err error) bool {
	var pe *permanentError
	if errors.As(err, &pe) {
		return true
	}
	var ae *apierror.Error
	if errors.As(err, &ae) {
		c := ae.StatusCode
		return c >= 400 && c < 500 && c != http.StatusNotFound && c != http.StatusRequestTimeout && c != http.StatusTooManyRequests
	}
	return false
}

// sameProducts compares two product lists ignoring case and order.
func sameProducts(a, b []string) bool {
	norm := func(in []string) []string {
		var out []string
		for _, n := range in {
			if n = strings.ToLower(strings.TrimSpace(n)); n != "" {
				out = append(out, n)
			}
		}
		slices.Sort(out)
		return out
	}
	return slices.Equal(norm(a), norm(b))
}

func anchorOf(ctx context.Context, opt Options) time.Time {
	if p, ok := schedule.PeriodFromContext(ctx); ok {
		return p
	}
	if opt.Anchor != nil {
		return opt.Anchor()
	}
	return time.Now()
}

func sendOne(ctx context.Context, src Fetcher, email EmailSender, opt Options, period, date string, products, to, cc []string) error {
	d, err := src.Dashboard(ctx, period, date, products)
	if err != nil {
		return fmt.Errorf("fetch report: %w", err)
	}
	// The response must echo the filter that was asked for; otherwise the
	// server applied a different one and the mail would show the wrong scope.
	if !sameProducts(d.Products, products) {
		return &permanentError{fmt.Errorf("report scope mismatch: asked for %d products, response names %d", len(products), len(d.Products))}
	}
	slog.Info("sending a product issue report",
		"task", TaskName(opt.Cadence), "date", date, "products", len(products),
		"open", d.Totals.Open, "to", len(to), "cc", len(cc))
	subject := notify.ProductIssueReportSubject(period, d, products)
	body := notify.RenderProductIssueReport(d, period, products, opt.LinkBase)
	if err := email.SendEmail(ctx, to, cc, subject, body); err != nil {
		return fmt.Errorf("send report: %w", err)
	}
	return nil
}

func isNotFound(err error) bool {
	var ae *apierror.Error
	return errors.As(err, &ae) && ae.StatusCode == http.StatusNotFound
}
