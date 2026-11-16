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
	"strings"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/notify"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/slabreachreport"
)

// TaskName is the registry (and SUB_CRON_SCHEDULES / SUB_CRON_RECIPIENTS)
// name of this task.
const TaskName = "sla_breach_weekly_report"

// Subject is the mail's subject, word for word the one the source report
// has always carried.
const Subject = "Weekly SLA Breach Report"

// ReportFetcher is the subset of *slabreachreport.Client this package
// depends on.
type ReportFetcher interface {
	Report(ctx context.Context, definitions, stages []string) (slabreachreport.Report, error)
}

// EmailSender is the subset of *notify.Client this package depends on.
type EmailSender interface {
	SendEmail(ctx context.Context, to, cc []string, subject, htmlBody string) error
}

// DefaultDefinitions are the ten SLA definitions the source report's
// filter named, in its own order.
//
// These are matched against sla_policy.name EXACTLY. They are the
// ServiceNow definition names; if Postgres carries a policy under a
// different name the report silently finds nothing, which is why
// SLA_BREACH_REPORT_DEFINITIONS can replace the whole list without a code
// change. Confirm against `SELECT DISTINCT name FROM sla_policy` before
// the first real send.
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

// DefaultStages are the clock stages the source report's "Stage is one of"
// filter selected. Replaceable by SLA_BREACH_REPORT_STAGES; an empty
// override means every stage.
func DefaultStages() []string {
	return []string{"PAUSED", "COMPLETED"}
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

// SendReport returns a registry.Task.Handler that fetches last week's SLA
// breaches and mails them to to/cc.
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
// Flip this by returning early on an empty report if the source's
// behaviour turns out to be the opposite.
//
// emailsEnabled is cmd/server/main.go's ALERTS_ENABLED. When false the
// handler succeeds without contacting entity-service.
func SendReport(reports ReportFetcher, email EmailSender, definitions, stages, to, cc []string, caseLinkBase string, emailsEnabled bool) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		if !emailsEnabled {
			return nil
		}
		if len(to) == 0 {
			slog.Info("weekly sla breach report has no recipients; skipping")
			return nil
		}
		if len(definitions) == 0 {
			// entity-service rejects an empty list rather than reporting
			// every policy; fail loudly here instead of sending a 400 on
			// every tick.
			return fmt.Errorf("slabreachscan: no SLA definitions configured")
		}

		report, err := reports.Report(ctx, definitions, stages)
		if err != nil {
			return fmt.Errorf("slabreachscan: fetch report: %w", err)
		}
		slog.Info("sending the weekly sla breach report",
			"periodStart", report.PeriodStart, "periodEnd", report.PeriodEnd,
			"onboarding", len(report.Onboarding), "cre", len(report.CRE),
			"to", len(to), "cc", len(cc))

		body := notify.RenderSLABreachReport(report, caseLinkBase)
		if err := email.SendEmail(ctx, to, cc, Subject, body); err != nil {
			return fmt.Errorf("slabreachscan: send report: %w", err)
		}
		return nil
	}
}
