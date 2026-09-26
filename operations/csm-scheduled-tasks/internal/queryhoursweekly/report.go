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

// Package queryhoursweekly is the weekly query-support consumption report
// sub-cron. It holds the task; internal/queryhoursreport holds the
// entity-service client and the wire types it returns.
//
// The split mirrors internal/entitycases and internal/opencases, and exists
// for the same mechanical reason: internal/notify imports the client package
// to render its types, so a task that imports notify cannot live beside them
// without a cycle.
package queryhoursweekly

import (
	"context"
	"fmt"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/notify"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/queryhoursreport"
)

// Subject is the email's subject line, verbatim from the ServiceNow original.
const Subject = "Weekly Query Support Consumption Report"

// ReportFetcher is the subset of *Client this package depends on.
type ReportFetcher interface {
	WeeklyReport(ctx context.Context) (queryhoursreport.Report, error)
}

// EmailSender is the subset of *notify.Client this package depends on.
type EmailSender interface {
	SendEmail(ctx context.Context, to, cc []string, subject, htmlBody string) error
}

// SendReport returns the sub-cron handler: fetch the report, render it, send
// it.
//
// *** RECIPIENTS COME FROM CONFIGURATION, NOT FROM THE DATA. ***
//
// ServiceNow derived this email's To line: a hardcoded seed address, plus the
// account manager and technical owner of every account in the EXCEEDED table
// only (accounts merely going to exceed contributed nobody). That rule is
// reproduced in the API response — entity-service reports both addresses per
// account — but deliberately not acted on here, for a reason worth recording:
// the ServiceNow copy available for inspection provably is not the one
// sending production's mail. Its Send Email step carries two Cc addresses
// where the real message carries three, and its derivation caps at roughly
// thirty-three recipients where the real message reaches about forty-eight.
// Something else is running. Deriving an audience from a rule we cannot
// verify would email around fifty people on a guess, so this task uses its
// own SUB_CRON_RECIPIENTS entry like every other report here, and the derived
// addresses sit in the payload for whoever settles the question later.
//
// emailsEnabled is ALERTS_ENABLED. As with the other report tasks, false
// skips the fetch entirely rather than fetching and discarding — and so does
// an empty `to`, since a report nobody receives is work with no consumer.
//
// An empty report is still sent. "No account has exhausted its query hours"
// is a real answer, and a week where the mail simply does not arrive is
// indistinguishable from a week where the job silently broke.
func SendReport(reports ReportFetcher, email EmailSender, to, cc []string, salesforceBaseURL string, emailsEnabled bool) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		if !emailsEnabled || len(to) == 0 {
			return nil
		}

		report, err := reports.WeeklyReport(ctx)
		if err != nil {
			return fmt.Errorf("queryhoursreport: fetch weekly report: %w", err)
		}

		body := notify.RenderQueryHoursWeeklyReport(notify.QueryHoursWeeklyReportData{
			Report:            report,
			SalesforceBaseURL: salesforceBaseURL,
		})
		if err := email.SendEmail(ctx, to, cc, Subject, body); err != nil {
			return fmt.Errorf("queryhoursreport: send report email: %w", err)
		}
		return nil
	}
}
