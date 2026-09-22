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

// Package allocationreminder is the weekly engagement status-update reminder
// sub-cron: it asks entity-service who has a live allocation but has not
// published their own status update for last week, and emails each of them a
// reminder.
//
// It is the Go port of ServiceNow's
// WeeklyAllocationStatusUpdateReminderEmailFlow. Registering it is a paired
// change with deactivating that flow, per the double-fire rule — two systems
// sending the same reminder is worse than neither.
//
// It differs from this component's other report-style sub-crons
// (internal/stalecases, internal/opencases) in the way that matters most
// operationally: those mail a fixed ops audience a table ABOUT other people,
// so an empty SUB_CRON_RECIPIENTS means there is nobody to tell and the task
// can skip its query entirely. This one mails the people who have to ACT, and
// its audience comes from the data, so SUB_CRON_RECIPIENTS cannot gate it —
// that entry configures only who is alerted when it FAILS.
package allocationreminder

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/engagementallocations"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/notify"
)

// subject is the ServiceNow flow's own Send Email subject, verbatim.
const subject = "Reminder: Send the weekly status update"

// RecipientSource is the subset of *engagementallocations.Client this package
// depends on.
type RecipientSource interface {
	StatusUpdateReminderRecipients(ctx context.Context, cycleStart time.Time) ([]engagementallocations.Recipient, error)
}

// EmailSender is the subset of *notify.Client this package depends on.
type EmailSender interface {
	SendEmail(ctx context.Context, to, cc []string, subject, htmlBody string) error
}

// lastCycleStart returns the Monday of the week BEFORE the one containing now
// — the cycle a reminder sent today is asking about.
//
// Deliberately derived from the calendar rather than ServiceNow's "today
// minus seven days". Those agree only when the job actually runs on a Monday;
// a retry on Tuesday, or a schedule override, silently shifted the original's
// whole window by a day and could ask about a cycle that never started on a
// Monday at all. Anchoring to the ISO week makes every attempt within a week
// — first try or fourth retry — ask about the same cycle, which is also what
// makes this handler idempotent per period.
func lastCycleStart(now time.Time) time.Time {
	d := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	// Go's Weekday starts at Sunday=0; shift so Monday=0.
	offset := (int(d.Weekday()) + 6) % 7
	return d.AddDate(0, 0, -offset-7)
}

// SendReminders returns a registry.Task.Handler that emails everyone who owes
// a status update for last week.
//
// portalBaseURL is the CSM Portal the recipients use, linked from the mail's
// navigation steps — see notify.RenderAllocationStatusUpdateReminder. Empty is
// valid and simply drops the hyperlink.
//
// emailsEnabled is cmd/server/main.go's ALERTS_ENABLED — the same global kill
// switch stalecases and opencases honour. When false this sends nothing and
// succeeds, so a maintenance window silences the reminder rather than failing
// the task.
//
// One email per recipient, not one email to all of them. The ServiceNow
// original joined every address into a single To line, which disclosed the
// full list of people behind on their updates to everyone on it; the body is
// identical for every recipient anyway, so nothing is lost by sending it N
// times.
//
// A send that fails for one person does not abandon the rest: every recipient
// is attempted, and the failures are collected into one error at the end. A
// task that gave up on the first bad address would leave most of the audience
// unreminded AND retry the whole batch next tick, re-mailing everyone who had
// already received it.
func SendReminders(recipients RecipientSource, email EmailSender, portalBaseURL string, emailsEnabled bool) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		if !emailsEnabled {
			return nil
		}

		cycleStart := lastCycleStart(time.Now().UTC())

		owing, err := recipients.StatusUpdateReminderRecipients(ctx, cycleStart)
		if err != nil {
			return fmt.Errorf("allocationreminder: look up recipients for cycle %s: %w",
				cycleStart.Format(time.DateOnly), err)
		}
		if len(owing) == 0 {
			slog.Info("weekly allocation status update reminder: nobody owes an update",
				"cycleStart", cycleStart.Format(time.DateOnly))
			return nil
		}

		body := notify.RenderAllocationStatusUpdateReminder(portalBaseURL)

		var failures []error
		sent := 0
		for _, r := range owing {
			if err := email.SendEmail(ctx, []string{r.Email}, nil, subject, body); err != nil {
				// The address is included because it is what makes the
				// alert actionable; these are internal WSO2 addresses.
				failures = append(failures, fmt.Errorf("%s: %w", r.Email, err))
				continue
			}
			sent++
		}

		slog.Info("weekly allocation status update reminder sent",
			"cycleStart", cycleStart.Format(time.DateOnly),
			"owing", len(owing), "sent", sent, "failed", len(failures))

		if len(failures) > 0 {
			return fmt.Errorf("allocationreminder: %d of %d reminders failed to send: %w",
				len(failures), len(owing), errors.Join(failures...))
		}
		return nil
	}
}
