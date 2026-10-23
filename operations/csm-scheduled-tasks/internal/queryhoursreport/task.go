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

// Package queryhoursreport is the weekly query-support consumption
// report and the per-project usage notifications. It holds the tasks;
// internal/queryhours holds the entity-service client and wire types,
// and internal/notify renders them -- the split every report sub-cron
// here uses, and for the same reason: notify imports the client package.
package queryhoursreport

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/notify"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/queryhours"
)

// ReportClient is the subset of *queryhours.Client these tasks use.
type ReportClient interface {
	WeeklyReport(ctx context.Context) (queryhours.Report, error)
	Recompute(ctx context.Context) ([]queryhours.Transition, error)
}

// EmailSender is the subset of *notify.Client these tasks use.
type EmailSender interface {
	SendEmail(ctx context.Context, to, cc []string, subject, htmlBody string) error
}

// SendWeeklyReport returns the weekly sub-cron handler.
//
// *** THE AUDIENCE IS DERIVED, AND THAT IS DELIBERATE HERE. ***
//
// The CS Worklogs reports configure their recipients because their
// delivered mails are not reproducible from any table. This one is the
// opposite: ServiceNow builds the To line from the data it is
// reporting on -- the account manager and technical owner of every
// account that has exceeded its allowance OR is close to doing so,
// internal addresses only, deduped. Every one of those fields exists
// here, so deriving is both possible and correct: an account that
// stops breaching stops mailing its owners, with nobody editing a list.
//
// extraTo and cc come from configuration and are added, not
// substituted. ServiceNow Ccs three standing groups and adds a fourth
// for the exceeded case.
//
// An empty report is still sent. "No account has exhausted its query
// hours" is a real answer, and a week where the mail simply does not
// arrive is indistinguishable from a week where the job broke.
func SendWeeklyReport(reports ReportClient, email EmailSender,
	extraTo, cc, override []string, emailsEnabled bool) func(ctx context.Context) error {

	return func(ctx context.Context) error {
		if !emailsEnabled {
			return nil
		}

		report, err := reports.WeeklyReport(ctx)
		if err != nil {
			return err
		}

		to := Audience(report.Recipients, extraTo, override)
		sendCc := clean(cc)
		if len(override) > 0 {
			// An override replaces the whole audience, Cc included: a
			// test run must not merely prepend itself to a live list.
			sendCc = nil
		}
		if len(to) == 0 {
			// Nothing derived and nothing configured. Sending to nobody
			// would be recorded as success, so say so instead.
			return errors.New("queryhoursreport: weekly report has no recipients")
		}

		slog.Info("query-hour weekly report",
			"exceeded", len(report.Exceeded),
			"closeToExceed", len(report.CloseToExceed),
			"derivedRecipients", len(report.Recipients))

		return email.SendEmail(ctx, to, sendCc,
			notify.QueryHoursWeeklySubject, notify.RenderQueryHoursWeekly(report))
	}
}

// SendUsageNotifications returns the handler behind ServiceNow's
// "[WSO2][Query Hour] Usage Notifications - Project" flow.
//
// That flow triggers on a project's query-hour state CHANGING to 75%,
// 90% or 100%. We have no stored per-project state and are not adding a
// column for one, so the recompute reports the crossings instead: it
// derives each state from figures it has just written and returns only
// those that rose. Same event, detected in process rather than by a
// database trigger.
//
// One mail per crossing, to that account's manager and technical owner.
// A failure on one does not stop the rest -- a single unreachable
// recipient should not silence every other notice in the batch.
func SendUsageNotifications(reports ReportClient, email EmailSender,
	cc, override []string, emailsEnabled bool) func(ctx context.Context) error {

	return func(ctx context.Context) error {
		transitions, err := reports.Recompute(ctx)
		if err != nil {
			return err
		}
		if !emailsEnabled || len(transitions) == 0 {
			return nil
		}

		var errs []error
		for _, t := range transitions {
			to := Audience([]string{t.AccountManagerEmail, t.TechnicalOwnerEmail}, nil, override)
			if len(to) == 0 {
				// ServiceNow falls back to a named individual here. We
				// log instead: a project whose account has no internal
				// owner is a data gap worth seeing, not something to
				// route around silently.
				slog.Warn("query-hour threshold crossed with no internal owner to notify",
					"project", t.ProjectKey, "account", t.AccountName, "state", t.To)
				continue
			}
			sendCc := clean(cc)
			if len(override) > 0 {
				sendCc = nil
			}
			if err := email.SendEmail(ctx, to, sendCc,
				notify.QueryHourNotificationSubject(t), notify.RenderQueryHourNotification(t)); err != nil {
				errs = append(errs, err)
			}
		}
		return errors.Join(errs...)
	}
}

// Audience merges the derived recipients with any configured additions,
// deduped case-insensitively. A non-empty override replaces everything.
func Audience(derived, extra, override []string) []string {
	if o := clean(override); len(o) > 0 {
		return o
	}
	seen := map[string]bool{}
	var out []string
	for _, s := range append(clean(derived), clean(extra)...) {
		k := strings.ToLower(s)
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, s)
	}
	return out
}

func clean(in []string) []string {
	var out []string
	for _, s := range in {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}
