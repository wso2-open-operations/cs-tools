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

package main

import (
	"context"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/notify"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/productissuescan"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/registry"
)

// productIssueReportTasks builds the weekly and monthly product issue report
// tasks (called only when PRODUCT_ISSUE_REPORTS_ENABLED=true) and both names,
// for the EMAIL_BASE_URL check.
//
// registry.Task.To/Cc stay nil, as for the SLA breach reports: the audience is
// leadership and product-lead mail, a failure alerts ALERT_RECIPIENTS only.
// MaxAttempts 2 as time_card_report: a retry re-sends the audiences that had
// already succeeded; the period comes from the engine's anchor, so a retry
// reports the same period.
func productIssueReportTasks(src productissuescan.Fetcher, emailClient notify.Sender, linkBase string, alertsEnabled bool,
	scheduleOverrides map[string]string, recipientOverrides map[string]subCronRecipients) (tasks []registry.Task, withRecipients []string) {
	for _, def := range []struct {
		cadence  productissuescan.Cadence
		schedule string
	}{
		{productissuescan.Weekly, productissuescan.WeeklySchedule},
		{productissuescan.Monthly, productissuescan.MonthlySchedule},
	} {
		name := productissuescan.TaskName(def.cadence)
		to, cc := recipientsFor(recipientOverrides, name)
		// Counted for the EMAIL_BASE_URL check whether or not "to" is set: the
		// product leads are an audience of their own.
		withRecipients = append(withRecipients, name)
		sched := scheduleFor(scheduleOverrides, name, def.schedule)
		opt := productissuescan.Options{
			Cadence: def.cadence, To: to, Cc: cc, LinkBase: linkBase,
			EmailsEnabled: alertsEnabled, Anchor: periodStart(sched),
		}
		tasks = append(tasks, registry.Task{
			Name:        name,
			Schedule:    sched,
			Handler:     productissuescan.Send(src, notify.ForReport(emailClient, debugFor(recipientOverrides, name)), opt),
			MaxAttempts: 2,
			// --recipients/--cc of a manual run-now only; see reportRunNow.
			RunNowFlags: reportRunNowFlags,
			RunNowNarrow: reportRunNow(notify.ForReport(emailClient, debugFor(recipientOverrides, name)), emailClient,
				func(email notify.Sender, rec []string) func(ctx context.Context) error {
					o := opt
					o.To = orRecipients(to, rec)
					return productissuescan.Send(src, email, o)
				}),
		})
	}
	return tasks, withRecipients
}
