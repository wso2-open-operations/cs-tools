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

// Package worklogsreport is the CS Worklogs daily and weekly report
// sub-crons. It holds the tasks; internal/worklogreport holds the
// entity-service client and the wire types, and internal/notify renders
// them -- the same split every other report task here uses, and for the
// same mechanical reason: notify imports the client package, so a task
// that imports notify cannot live beside it without a cycle.
package worklogsreport

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/notify"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/worklogreport"
)

// ReportFetcher is the subset of *worklogreport.Client these tasks use.
type ReportFetcher interface {
	Report(ctx context.Context, cadence worklogreport.Cadence, from, to time.Time) (worklogreport.Report, error)
}

// EmailSender is the subset of *notify.Client these tasks use.
type EmailSender interface {
	SendEmail(ctx context.Context, to, cc []string, subject, htmlBody string) error
}

// ReportTimezone is the zone the reported day and week are measured in.
//
// It matters, and it is not the zone the cron fires in. ServiceNow runs
// WeeklyReportAutomation V2 at 16:00 UTC on a Monday, which is 21:30 in
// Asia/Colombo -- the timestamp on the delivered mail. The job then asks
// for the seven days before its own run date. Computing "the week that
// just ended" in UTC from a cron that fires at 16:00 UTC gives the same
// answer here, but the daily report fires at 03:55 UTC, which is already
// the next day in Colombo, so the two zones disagree about which day
// "yesterday" is. The reported range is therefore always derived in this
// zone, never in UTC and never in the host's local time.
const ReportTimezone = "Asia/Colombo"

// WeeklyRange returns the Monday-to-Sunday window a weekly run reports
// on: the seven days ending the day before now.
//
// ServiceNow's DateRange action walks back from its trigger date one day
// at a time -- `to` is the previous day and `from` six further back --
// so a Monday run covers the Monday to Sunday just gone. Its own output
// names are off by one (outputs.sat holds Sunday), which affects only
// the column headings, not the range.
func WeeklyRange(now time.Time, loc *time.Location) (from, to time.Time) {
	day := now.In(loc)
	to = time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -1)
	from = to.AddDate(0, 0, -6)
	return from, to
}

// DailyDate returns the single day a daily run reports on: yesterday.
func DailyDate(now time.Time, loc *time.Location) time.Time {
	day := now.In(loc)
	return time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -1)
}

// SendDaily returns the daily sub-cron handler. anchor supplies the
// scheduled firing time this run belongs to -- see send.
func SendDaily(reports ReportFetcher, email EmailSender, anchor func() time.Time,
	to, cc []string, source worklogreport.Source, caseLinkBase string,
	emailsEnabled bool) func(ctx context.Context) error {
	return send(reports, email, worklogreport.CadenceDaily, anchor, to, cc, source, caseLinkBase, emailsEnabled)
}

// SendWeekly returns the weekly sub-cron handler.
func SendWeekly(reports ReportFetcher, email EmailSender, anchor func() time.Time,
	to, cc []string, source worklogreport.Source, caseLinkBase string,
	emailsEnabled bool) func(ctx context.Context) error {
	return send(reports, email, worklogreport.CadenceWeekly, anchor, to, cc, source, caseLinkBase, emailsEnabled)
}

// send fetches, renders and sends one report for the period its anchor
// names.
//
// *** RECIPIENTS COME FROM CONFIGURATION, NOT FROM THE DATA. ***
//
// entity-service derives an audience from the team registry and returns
// it on the report, but it is carried for comparison only. The delivered
// mails are not derivable: their To and Cc roles are inverted between
// the two cadences, two addresses appear in both fields of the daily
// one, and several recipients hold no team membership at all. The
// ServiceNow flow hardcodes both lines in its Send Email action, and its
// GetLeadsEmails step feeds a getLeadsInfo() that V2 overrides to return
// nothing. Deriving the audience here would mail the wrong thirty-four
// people.
//
// Drift between the configured lists and the derived one is logged so a
// lead who joins and never receives the report, or one who left and
// still does, surfaces somewhere other than an inbox.
//
// An empty report is still sent. "Nobody logged time" is a real answer,
// and a day where the mail simply does not arrive is indistinguishable
// from a day where the job broke.
func send(reports ReportFetcher, email EmailSender, cadence worklogreport.Cadence,
	anchor func() time.Time, to, cc []string, source worklogreport.Source,
	caseLinkBase string, emailsEnabled bool) func(ctx context.Context) error {

	// Gate on the CLEANED list, not the raw one. {"to": [""]} has
	// length one and is no audience at all; checking len(to) let it
	// through to a send with no recipients.
	cleanTo := nonBlank(to)
	cleanCc := nonBlank(cc)

	// With the derived source the configured lists are irrelevant, so
	// an empty one is not a reason to skip -- the audience is not known
	// until the report comes back.
	skipWithoutRecipients := source != worklogreport.SourceDerived

	return func(ctx context.Context) error {
		if !emailsEnabled || (skipWithoutRecipients && len(cleanTo) == 0) {
			return nil
		}

		loc, err := time.LoadLocation(ReportTimezone)
		if err != nil {
			return fmt.Errorf("worklogsreport: load %s: %w", ReportTimezone, err)
		}

		// The range comes from the SCHEDULED FIRING, never from the
		// wall clock. The engine retries a failed period on every tick
		// until the next firing, so a daily run that fails at 03:55 UTC
		// and succeeds on a retry after 18:30 UTC -- already the next
		// day in Colombo -- would otherwise report the wrong day
		// entirely: the day it owed would never be sent, and the
		// following one would be sent twice. anchor is
		// schedule.PeriodKey for this task's cron, so it is identical
		// for every attempt at the same period.
		at := anchor()
		var from, until time.Time
		if cadence == worklogreport.CadenceWeekly {
			from, until = WeeklyRange(at, loc)
		} else {
			from = DailyDate(at, loc)
			until = from
		}

		report, err := reports.Report(ctx, cadence, from, until)
		if err != nil {
			return err
		}

		audience := worklogreport.Resolve(report.Recipients,
			worklogreport.Audience{To: cleanTo, Cc: cleanCc}, source, nil)

		// Whichever source is in use, an empty audience is an error
		// worth seeing: SendEmail treats no recipients as success, so
		// the failure mode is a report that silently never arrives.
		if len(audience.To) == 0 {
			return fmt.Errorf("worklogsreport: %s report has no recipients from source %q", cadence, source)
		}

		// The drift check the configured lists are worth having: who
		// the team registry says should receive this against who
		// actually will. Neither direction is an error -- the lists are
		// maintained by hand on purpose -- but a lead who joins and
		// never appears, or one who left and still receives it, shows
		// up here instead of in somebody's inbox.
		if missing, extra := worklogreport.Drift(report.Recipients, audience); len(missing) > 0 || len(extra) > 0 {
			// COUNTS, NEVER THE ADDRESSES. Application logs are far
			// more widely readable than a report's audience, and this
			// line was printing every derived address into them --
			// sixteen real ones on the first run. notify's redirect
			// log takes the same care for the same reason.
			//
			// Who the addresses are is still answerable: the derived
			// audience comes back on the report itself, from
			// GET /reports/worklogs, where reaching it is a deliberate
			// act by someone entitled to.
			slog.Info("worklog report audience drifts from the team registry",
				"cadence", cadence,
				"source", source,
				"derivedButNotConfigured", len(missing),
				"configuredButNotDerived", len(extra))
		}

		return email.SendEmail(ctx, audience.To, audience.Cc,
			notify.WorklogsReportSubject(report), notify.RenderWorklogsReport(report, caseLinkBase))
	}
}

// nonBlank drops empty and whitespace-only entries, so a configured list
// that is blank reads as absent rather than as an audience of one.
func nonBlank(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}
