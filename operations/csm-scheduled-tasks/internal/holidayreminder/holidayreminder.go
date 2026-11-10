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

// Package holidayreminder emails CRE team leads and rota managers ahead of a
// public holiday, so they mark who works that day.
//
// On a public holiday of their country a CRE member is off: the Team
// Schedule places a "Public holiday" tag on them (the day leaves their
// timesheet and they leave the rota). A lead makes somebody available by
// removing the tag, then confirms the team. Each team is emailed 14 days
// before (always), then 7 days, 2 working days and 1 working day before
// while it is not fully marked (a Monday holiday: Thursday and Friday).
//
// Which reminders are due, and to whom, is entity-service's decision
// (POST /team-schedule/holiday-availability/reminders/due); this task sends
// them and records each one, so a reminder goes once.
package holidayreminder

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/teamschedule"
)

// Client is the subset of *teamschedule.Client this task uses.
type Client interface {
	DueHolidayReminders(ctx context.Context, today string) ([]teamschedule.DueHolidayReminder, error)
	RecordHolidayReminder(ctx context.Context, teamKey, date, stage string) error
	RotaAdmins(ctx context.Context, family string) ([]teamschedule.RotaAdmin, error)
}

// EmailSender is the subset of *notify.Client this task uses.
type EmailSender interface {
	SendEmail(ctx context.Context, to, cc []string, subject, htmlBody string) error
}

// Config is the task's configuration, set in cmd/server/main.go.
type Config struct {
	// ScheduleURL, when set, is the Team Schedule link in every email.
	ScheduleURL string
	// RedirectTo, when set, receives every email instead of its recipients,
	// who are named in the body (ROTA_REMINDER_REDIRECT_TO).
	RedirectTo []string
	// EmailsEnabled is ALERTS_ENABLED.
	EmailsEnabled bool
	// Zone is the rota's calendar (Asia/Colombo): "today" is today there.
	Zone *time.Location
	// Anchor returns the run's scheduled firing time, so a retry still sends
	// that morning's reminders. Nil means time.Now.
	Anchor func() time.Time
}

// Run returns the task's handler.
func Run(client Client, email EmailSender, cfg Config) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		if !cfg.EmailsEnabled {
			return nil
		}
		zone := cfg.Zone
		if zone == nil {
			zone = time.UTC
		}
		at := time.Now()
		if cfg.Anchor != nil {
			at = cfg.Anchor()
		}
		today := at.In(zone).Format(time.DateOnly)
		due, err := client.DueHolidayReminders(ctx, today)
		if err != nil {
			return fmt.Errorf("holiday availability: read the reminders due: %w", err)
		}
		var admins []teamschedule.RotaAdmin
		adminsRead := false
		var failed []string
		sent := 0
		for _, r := range due {
			to := emailsOf(r.Recipients)
			greeting := namesOf(r.Recipients)
			if len(to) == 0 {
				// A team with no lead or rota manager: its CRE rota admins.
				if !adminsRead {
					admins, err = client.RotaAdmins(ctx, "CRE")
					if err != nil {
						slog.WarnContext(ctx, "holiday availability: rota admins unreadable", "err", err)
					}
					adminsRead = true
				}
				for _, a := range admins {
					if strings.TrimSpace(a.Email) != "" {
						to = append(to, a.Email)
					}
				}
				greeting = "CRE rota admins"
			}
			label := r.Availability.TeamName + " " + r.Availability.Date
			if len(to) == 0 {
				slog.WarnContext(ctx, "holiday availability: nobody to remind", "team", r.Availability.TeamKey, "date", r.Availability.Date)
				failed = append(failed, label+" (nobody to send to)")
				continue
			}
			subject := Subject(r)
			body := Render(r, greeting, cfg.ScheduleURL)
			if len(cfg.RedirectTo) > 0 {
				body = "<p><i>[Redirected] Would have been sent to: " + html.EscapeString(strings.Join(to, ", ")) + "</i></p>" + body
				to = cfg.RedirectTo
			}
			if err := email.SendEmail(ctx, to, nil, subject, body); err != nil {
				slog.ErrorContext(ctx, "holiday availability: email failed", "team", r.Availability.TeamKey, "date", r.Availability.Date, "stage", r.Stage, "err", err)
				failed = append(failed, label+" ("+r.Stage+")")
				continue
			}
			if err := client.RecordHolidayReminder(ctx, r.Availability.TeamKey, r.Availability.Date, r.Stage); err != nil {
				// Sent but not recorded: the retry sends it once more.
				slog.ErrorContext(ctx, "holiday availability: sent but not recorded", "team", r.Availability.TeamKey, "date", r.Availability.Date, "stage", r.Stage, "err", err)
				failed = append(failed, label+" ("+r.Stage+", sent but not recorded)")
				continue
			}
			sent++
		}
		slog.InfoContext(ctx, "holiday availability: reminders", "due", len(due), "sent", sent, "failed", len(failed))
		if len(failed) > 0 {
			return fmt.Errorf("holiday availability: %d of %d reminders not sent: %s", len(failed), len(due), strings.Join(failed, "; "))
		}
		return nil
	}
}

func emailsOf(rs []teamschedule.HolidayRecipient) []string {
	var out []string
	for _, r := range rs {
		if e := strings.TrimSpace(r.Email); e != "" {
			out = append(out, e)
		}
	}
	return out
}

func namesOf(rs []teamschedule.HolidayRecipient) string {
	var names []string
	for _, r := range rs {
		if n := strings.TrimSpace(r.Name); n != "" {
			names = append(names, n)
		}
	}
	switch len(names) {
	case 0:
		return "team leads"
	case 1:
		return names[0]
	}
	return "all"
}

func holidayDay(date string) string {
	d, err := time.Parse(time.DateOnly, date)
	if err != nil {
		return date
	}
	return d.Format("Monday, 2 January 2006")
}

// stageLine is what the stage says in the email.
func stageLine(r teamschedule.DueHolidayReminder) string {
	switch r.Stage {
	case "notice":
		return fmt.Sprintf("The holiday is in %d days.", r.DaysBefore)
	case "week":
		return "Reminder: the holiday is in one week, and the team's availability is not marked yet."
	case "two_days":
		return "Reminder: the holiday is two working days away, and the team's availability is not marked yet."
	case "one_day":
		return "Last reminder: the holiday is on the next working day, and the team's availability is not marked yet."
	}
	return ""
}

// Subject is the email's subject.
func Subject(r teamschedule.DueHolidayReminder) string {
	day := holidayDay(r.Availability.Date)
	if r.Stage == "notice" {
		return fmt.Sprintf("[CRE] Public holiday on %s: mark who works (%s)", day, r.Availability.TeamName)
	}
	return fmt.Sprintf("[CRE] Reminder: mark who works on %s (%s)", day, r.Availability.TeamName)
}

func memberStatus(m teamschedule.HolidayMember) string {
	switch m.Status {
	case "TAGGED":
		return "Off (Public holiday)"
	case "AVAILABLE":
		return "Working"
	case "AWAY":
		if m.AwayKind != "" {
			return "Away (" + m.AwayKind + ")"
		}
		return "Away"
	}
	return m.Status
}

// Render is the email's HTML body: a plain, formal message, like the rota
// reminders.
func Render(r teamschedule.DueHolidayReminder, greeting, scheduleURL string) string {
	a := r.Availability
	e := html.EscapeString
	var b strings.Builder
	fmt.Fprintf(&b, "<p>Dear %s,</p>", e(greeting))
	fmt.Fprintf(&b, "<p><b>%s</b> (%s) is on <b>%s</b>. %s</p>",
		e(strings.Join(a.Titles, " / ")), e(strings.Join(a.Countries, ", ")), e(holidayDay(a.Date)), e(stageLine(r)))
	fmt.Fprintf(&b, "<p>The members of %s below are tagged &quot;Public holiday&quot; for that day: "+
		"the day is left out of their timesheet, and they are off the rota, so Case Alerts does not call them.</p>", e(a.TeamName))
	b.WriteString(`<table cellpadding="6" cellspacing="0" border="1" style="border-collapse:collapse;font-size:13px">`)
	b.WriteString("<tr><th align=\"left\">Name</th><th align=\"left\">That day</th></tr>")
	for _, m := range a.Members {
		fmt.Fprintf(&b, "<tr><td>%s</td><td>%s</td></tr>", e(m.Name), e(memberStatus(m)))
	}
	b.WriteString("</table>")
	if len(a.Uncovered) > 0 {
		b.WriteString("<p>These rota turns have nobody available that day:</p><ul>")
		for _, t := range a.Uncovered {
			fmt.Fprintf(&b, "<li>%s (%s)</li>", e(t.ShiftLabel), e(t.Name))
		}
		b.WriteString("</ul>")
	}
	b.WriteString("<p><b>What to do:</b> in the Team Schedule, remove the Public holiday tag from each member who works that day. " +
		"Make sure every rota turn has somebody available (or move the turn), then press <b>Confirm team availability</b> for the holiday. " +
		"Once confirmed, no more reminders are sent.</p>")
	if a.ConfirmedBy != "" && !a.FullyMarked {
		fmt.Fprintf(&b, "<p>The team was confirmed by %s, but a rota turn has since lost its person.</p>", e(a.ConfirmedBy))
	}
	if scheduleURL != "" {
		fmt.Fprintf(&b, "<p>Team Schedule: <a href=\"%s\">%s</a></p>", e(scheduleURL), e(scheduleURL))
	}
	b.WriteString("<p>Regards,<br>CSM Team Schedule</p>")
	return b.String()
}
