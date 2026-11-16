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

package slabreachscan

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/notify"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/slabreachreport"
)

// Variant is one of the ServiceNow weekly SLA breach reports.
//
// There are two, and they are SEPARATE report definitions, not two
// sections of one: they share their ten SLA definitions but differ in
// both their stage filter and their onboarding-status filter, and those
// status sets are not complements (the onboarding report takes
// IN_PROGRESS only; the other excludes IN_PROGRESS and NOT_STARTED, so a
// NOT_STARTED case appears in neither). Each is its own task and its own
// email, which is also how the source is configured.
type Variant struct {
	// Key names the variant in config and in the task name; lower-case,
	// stable, never shown to a reader.
	Key string
	// Subject is the mail's subject, word for word the source report's
	// own title.
	Subject string
	// Heading titles the table in the body.
	Heading string
	// Intro is the one line naming whose cases these are.
	Intro string
	// Stages are the sla.stage values this report covers.
	Stages []string
	// OnboardingStatuses is ServiceNow's "Onboarding Status is one of".
	OnboardingStatuses []string
	// ExcludeOnboardingStatuses is its "is not one of".
	ExcludeOnboardingStatuses []string
}

// CREVariant is the plain "Weekly SLA Breach Report": the CRE team's
// cases.
//
// Stage {PAUSED, COMPLETED} and onboarding status not one of
// {IN_PROGRESS, NOT_STARTED}, exactly as the source report's filter is
// configured.
func CREVariant() Variant {
	return Variant{
		Key:                       "cre",
		Subject:                   "Weekly SLA Breach Report",
		Heading:                   "Weekly SLA Breach Report",
		Intro:                     "Cases handled by the CRE team.",
		Stages:                    []string{"PAUSED", "COMPLETED"},
		ExcludeOnboardingStatuses: []string{"IN_PROGRESS", "NOT_STARTED"},
	}
}

// TaskName is the registry (and SUB_CRON_SCHEDULES / SUB_CRON_RECIPIENTS)
// name of a variant's task. The CRE report keeps the unsuffixed name it
// was registered under.
func TaskName(v Variant) string {
	if v.Key == "cre" {
		return "sla_breach_weekly_report"
	}
	return "sla_breach_weekly_report_" + v.Key
}

// ReportFetcher is the subset of *slabreachreport.Client this package
// depends on.
type ReportFetcher interface {
	Report(ctx context.Context, definitions, stages, onboardingStatuses, excludeOnboardingStatuses []string, week time.Time) (slabreachreport.Report, error)
}

// EmailSender is the subset of *notify.Client this package depends on.
type EmailSender interface {
	SendEmail(ctx context.Context, to, cc []string, subject, htmlBody string) error
}

// DefaultDefinitions are the ten SLA definitions both source reports'
// filters name, in their own order.
//
// These are matched against sla_policy.name EXACTLY. They are the
// ServiceNow definition names; if Postgres carries a policy under a
// different name the report silently finds nothing, which is why
// SLA_BREACH_REPORT_DEFINITIONS can replace the whole list without a
// code change. Confirm against `SELECT DISTINCT name FROM sla_policy`
// before the first real send.
func DefaultDefinitions() []string {
	return []string{
		"P1 - Resolution (Open Source)",
		"P1 - Response (Open Source)",
		"P1 - Workaround (Open Source)",
		"P2 - Resolution (Open Source)",
		"P2 - Response (Open Source)",
		"P2 - Workaround (Open Source)",
		"P3 - Resolution (Open Source)",
		"P3 - Response (Open Source)",
		"P3 - Workaround (Open Source)",
		"Query - Response (Open Source)",
	}
}

// Definitions resolves the shared definition list, overridable wholesale
// by SLA_BREACH_REPORT_DEFINITIONS.
func Definitions() []string {
	if override := ParseList(os.Getenv("SLA_BREACH_REPORT_DEFINITIONS")); len(override) > 0 {
		return override
	}
	return DefaultDefinitions()
}

// ApplyStageOverride replaces a variant's stage filter from
// SLA_BREACH_REPORT_<KEY>_STAGES when that variable is set. Set-but-empty
// means every stage, which is a real choice, so this tests for presence
// rather than for a non-empty value.
func ApplyStageOverride(v Variant) Variant {
	name := "SLA_BREACH_REPORT_" + strings.ToUpper(v.Key) + "_STAGES"
	if raw, set := os.LookupEnv(name); set {
		v.Stages = ParseList(raw)
	}
	return v
}

// ParseList splits a comma-separated override into a trimmed list.
// Returns nil for an empty or whitespace-only value, which every caller
// reads as "keep the default".
func ParseList(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

// RunNowWeek turns a manual run's --from-date/--to-date into the single
// week the report covers.
//
// The report is always a whole Monday-to-Monday week, so the two dates
// have to name the same one; a range spanning a boundary has no answer
// this report could give, and silently reporting one of the two weeks
// would be worse than refusing. Both zero means "not given", and the
// caller reports last week as usual.
func RunNowWeek(from, to time.Time, loc *time.Location) (time.Time, error) {
	if from.IsZero() && to.IsZero() {
		return time.Time{}, nil
	}
	if from.IsZero() || to.IsZero() {
		return time.Time{}, fmt.Errorf("--from-date and --to-date must be given together")
	}
	if to.Before(from) {
		return time.Time{}, fmt.Errorf("--to-date %s is before --from-date %s",
			to.Format("2006-01-02"), from.Format("2006-01-02"))
	}
	if mondayOf(from, loc) != mondayOf(to, loc) {
		return time.Time{}, fmt.Errorf(
			"--from-date %s and --to-date %s are in different weeks; this report always covers one whole Monday-to-Monday week",
			from.Format("2006-01-02"), to.Format("2006-01-02"))
	}
	return from, nil
}

// mondayOf is the date of the Monday starting the week that t falls in,
// as a plain YYYY-MM-DD string for comparison.
func mondayOf(t time.Time, loc *time.Location) string {
	local := t.In(loc)
	offset := (int(local.Weekday()) + 6) % 7
	return local.AddDate(0, 0, -offset).Format("2006-01-02")
}

// SendReport returns a registry.Task.Handler that fetches last week's SLA
// breaches for one variant and mails them to to/cc.
//
// The audience is this task's own SUB_CRON_RECIPIENTS entry, nothing
// else, as for the regression reports. With no "to" the handler succeeds
// without contacting entity-service: no report to send is no reason to
// run the query.
//
// Unlike the regression reports, a week with NO breaches is still sent.
// A clean week is the result this report exists to show, and silence
// would be indistinguishable from the task being broken -- the exact
// confusion the regression reports' own "expect silence" caveat causes.
//
// A zero week is the scheduled behaviour: entity-service resolves last
// week itself. A non-zero week reports the week containing that date,
// which is how a manual run covers a week that was missed -- or, on an
// environment whose data has stopped, a week that actually has rows.
//
// emailsEnabled is cmd/server/main.go's ALERTS_ENABLED. When false the
// handler succeeds without contacting entity-service.
func SendReport(reports ReportFetcher, email EmailSender, v Variant, definitions, to, cc []string, caseLinkBase string, emailsEnabled bool, week time.Time) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		if !emailsEnabled {
			return nil
		}
		if len(to) == 0 {
			slog.Info("sla breach report has no recipients; skipping", "report", v.Key)
			return nil
		}
		if len(definitions) == 0 {
			// entity-service rejects an empty list rather than reporting
			// every policy; fail loudly here instead of sending a 400 on
			// every tick.
			return fmt.Errorf("slabreachscan %s: no SLA definitions configured", v.Key)
		}

		report, err := reports.Report(ctx, definitions, v.Stages, v.OnboardingStatuses, v.ExcludeOnboardingStatuses, week)
		if err != nil {
			return fmt.Errorf("slabreachscan %s: fetch report: %w", v.Key, err)
		}
		slog.Info("sending the weekly sla breach report",
			"report", v.Key, "periodStart", report.PeriodStart, "periodEnd", report.PeriodEnd,
			"rows", len(report.Rows), "to", len(to), "cc", len(cc))

		body := notify.RenderSLABreachReport(report, v.Heading, v.Intro, caseLinkBase)
		if err := email.SendEmail(ctx, to, cc, v.Subject, body); err != nil {
			return fmt.Errorf("slabreachscan %s: send report: %w", v.Key, err)
		}
		return nil
	}
}
