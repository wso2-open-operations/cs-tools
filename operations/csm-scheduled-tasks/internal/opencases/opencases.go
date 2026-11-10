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

// Package opencases is the "cases still in Open state" report sub-cron: it
// finds every case created before
// yesterday that has never moved out of the initial "open" state, and emails a report of them. Distinct from
// internal/stalecases (the long-open cases report: cases still open after 30 days) — this one flags a
// narrower, more urgent signal: a case nobody has even started working on
// yet. Same shape otherwise: see internal/stalecases's own package doc
// comment for why this report is sent from inside the handler itself, not
// through engine.Engine's failure-alert path.
package opencases

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/longopenreport"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/notify"
)

// Subject is the report mail's subject line.
const Subject = "[Immediate-Action-Required][Report] Opened before yesterday cases"

// ReportFetcher is the subset of *longopenreport.Client this package depends on.
type ReportFetcher interface {
	OpenBeforeYesterday(ctx context.Context) (longopenreport.Report, error)
}

// EmailSender is the subset of *notify.Client this package depends on.
type EmailSender interface {
	SendEmail(ctx context.Context, to, cc []string, subject, htmlBody string) error
}

// SendReport returns a registry.Task.Handler that fetches entity-service's
// open-before-yesterday report (cases still in "open" state created before the
// start of yesterday, the day boundary computed there in the team's time zone)
// and emails it to to/cc. Recipients are exactly this task's own
// SUB_CRON_RECIPIENTS entry (see cmd/server/main.go) — same reasoning as
// stalecases.SendReport's own doc comment: unrelated to the standing
// ALERT_RECIPIENTS failure-alert audience.
//
// emailsEnabled is cmd/server/main.go's ALERTS_ENABLED — see
// stalecases.SendReport's own doc comment for why this silences report
// emails too, not just failure alerts.
//
// If emailsEnabled is false or to is empty, the returned handler skips the
// fetch entirely and returns nil (success). When the report has no cases it
// sends nothing and succeeds: the report omits itself on a day with nothing to
// act on. When entity-service cut the list at its cap the mail still goes out
// and says so.
func SendReport(reports ReportFetcher, email EmailSender, to, cc []string, emailsEnabled bool) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		if !emailsEnabled || len(to) == 0 {
			return nil
		}

		report, err := reports.OpenBeforeYesterday(ctx)
		if err != nil {
			return fmt.Errorf("opencases: fetch open cases report: %w", err)
		}

		if len(report.Cases) == 0 {
			slog.Info("no open cases created before yesterday; the open cases report is not sent")
			return nil
		}
		slog.Info("sending the open cases report",
			"cases", len(report.Cases), "casesTotal", report.CasesTotal,
			"truncated", report.CasesTruncated, "to", len(to), "cc", len(cc))

		body := notify.RenderOpenCasesReport(notify.OpenCasesReportData{Report: report})
		if err := email.SendEmail(ctx, to, cc, Subject, body); err != nil {
			return fmt.Errorf("opencases: send report email: %w", err)
		}
		return nil
	}
}
