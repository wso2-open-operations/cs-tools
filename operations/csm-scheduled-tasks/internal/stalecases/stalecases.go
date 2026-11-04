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

// Package stalecases is the "cases open too long" report sub-cron: it fetches
// entity-service's long-open cases report and mails it, as two tables (cases,
// and cases on onboarding projects), to this task's own recipients. It sends
// nothing on a day both tables are empty. The report logic itself (which cases
// count as ongoing, how they are split and capped) lives in entity-service;
// this package only fetches, renders and sends. The mail is sent on success
// from inside the handler, not through engine.Engine's failure-alert path; see
// this component's CLAUDE.md ("Per-task report emails").
package stalecases

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/longopenreport"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/notify"
)

// ReportFetcher is the subset of *longopenreport.Client this package depends on.
type ReportFetcher interface {
	Report(ctx context.Context, olderThanDays int) (longopenreport.Report, error)
}

// EmailSender is the subset of *notify.Client this package depends on.
type EmailSender interface {
	SendEmail(ctx context.Context, to, cc []string, subject, htmlBody string) error
}

// SendReport returns a registry.Task.Handler that fetches the long-open cases
// report for cases open more than olderThanDays and emails it to to/cc.
// Recipients are exactly this task's own SUB_CRON_RECIPIENTS entry (see
// cmd/server/main.go), deliberately not merged with the standing
// ALERT_RECIPIENTS audience, which is about failure alerting.
//
// caseLinkBase is the CSM portal's base URL (CSM_PORTAL_WEB_BASE_URL); when
// set, the case cells link to <caseLinkBase>/cases/<internal id>, else they are plain text.
//
// emailsEnabled is cmd/server/main.go's ALERTS_ENABLED, the same global kill
// switch engine.Engine.AlertsEnabled uses: it silences every email this
// component sends. If emailsEnabled is false or to is empty, the handler skips
// the fetch entirely and succeeds, without contacting entity-service.
//
// When both tables of the report are empty the handler sends nothing and
// succeeds: the report omits itself on a day with nothing to act on.
func SendReport(reports ReportFetcher, email EmailSender, olderThanDays int, to, cc []string, caseLinkBase string, emailsEnabled bool) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		if !emailsEnabled || len(to) == 0 {
			return nil
		}

		report, err := reports.Report(ctx, olderThanDays)
		if err != nil {
			return fmt.Errorf("stalecases: fetch long-open cases report: %w", err)
		}

		if len(report.Cases) == 0 && len(report.OnboardingCases) == 0 {
			slog.Info("no long-open cases; the stale cases report is not sent",
				"olderThanDays", olderThanDays)
			return nil
		}
		slog.Info("sending the stale cases report",
			"olderThanDays", olderThanDays,
			"cases", len(report.Cases), "casesTotal", report.CasesTotal,
			"onboardingCases", len(report.OnboardingCases), "onboardingCasesTotal", report.OnboardingCasesTotal,
			"to", len(to), "cc", len(cc))

		subject := fmt.Sprintf("[Action-Required][Report] Cases open for more than %d days", olderThanDays)
		body := notify.RenderLongOpenCasesReport(report, olderThanDays, caseLinkBase)
		if err := email.SendEmail(ctx, to, cc, subject, body); err != nil {
			return fmt.Errorf("stalecases: send report email: %w", err)
		}
		return nil
	}
}
