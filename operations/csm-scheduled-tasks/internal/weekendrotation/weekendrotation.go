// Package weekendrotation is the "you are on weekend support" notice sub-cron:
// it looks up who is rostered for the coming Saturday and Sunday and emails
// them.
//
// It replaces the ServiceNow flow "Dispatch Email Notification for Weekend
// Team", whose GetWeekendTeam action is the specification this follows:
//
//	if (dayname == 'Monday' | dayname == 'Thursday') {
//	    var saturday = new TeamScheduleLoader()._get_nextSaturdayDate();
//	    var sunday   = new TeamScheduleLoader()._get_nextSundayDate();
//	    var saturdayuserlist = new TeamScheduleLoader().getWeekendTeam(saturday);
//	    var sundayuserlist   = new TeamScheduleLoader().getWeekendTeam(sunday);
//	    ...  // emails = "a@x,b@x,"  emailbody = Saturday Team / Sunday Team
//	}
//	outputs.dispatchemail = dispatchemail;
//
// Behaviour kept: both weekend days are looked up separately and reported as
// two groups, each person shown with their roster note, and the notice goes
// out twice a week (Monday and Thursday) rather than daily -- the flow's own
// trigger is daily at 08:30, but the action gates dispatch to those two days
// and the flow's If step keys on outputs.dispatchemail.
//
// ONE BEHAVIOUR DELIBERATELY CHANGED: an empty roster. The action joins
// recipients into a string ("a@x," per member), so an empty roster yields ""
// and the flow's Send Email step fails with "Email validation failed: Email
// has no recipients." That is not a rare edge -- the instance has no weekend
// roster dated later than 2025-08-05, so every Monday and Thursday dispatch
// now fails, 2 for 2 in the last eight runs observed. SendNotice treats an
// unstaffed weekend as a success that sends nothing: a weekend with nobody on
// it is a real state, and failing would retry all week and alert on something
// no retry can fix. Noticing an unstaffed weekend is a monitoring concern.
package weekendrotation

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/entityrotations"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/notify"
)

// RotationSearcher is the subset of *entityrotations.Client this package needs.
type RotationSearcher interface {
	SearchRotations(ctx context.Context, rotationTypes []string, date string) ([]entityrotations.Member, error)
}

// EmailSender is the subset of *notify.Client this package needs.
type EmailSender interface {
	SendEmail(ctx context.Context, to, cc []string, subject, htmlBody string) error
}

// dateLayout is the YYYY-MM-DD form entity-service's roster API takes.
const dateLayout = "2006-01-02"

// NextSaturday returns the coming Saturday relative to from, in from's own
// location. A Saturday maps to itself, so a notice sent on the Saturday
// morning is still about that day rather than skipping a week; every other day
// moves forward, never back.
func NextSaturday(from time.Time) time.Time {
	daysAhead := (int(time.Saturday) - int(from.Weekday()) + 7) % 7
	return time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, from.Location()).
		AddDate(0, 0, daysAhead)
}

// SendNotice returns a registry.Task.Handler that emails the coming weekend's
// rostered team to tell them they are on duty.
//
// Recipients are the rostered people themselves, plus whatever extra to/cc this
// task's SUB_CRON_RECIPIENTS entry supplies (a lead who wants visibility, say).
// That is the inverse of the report-style sub-crons in this component, where
// the configured recipients ARE the audience — here they are an addition to an
// audience the roster determines.
//
// emailsEnabled is cmd/server/main.go's ALERTS_ENABLED, the same global kill
// switch every other email in this component honours. When false the handler
// still succeeds and sends nothing.
//
// An EMPTY ROSTER IS A SUCCESS, not an error: a weekend with nobody rostered
// is a real state (a holiday, or a roster not yet filled), and failing here
// would retry all week and alert on something no retry can fix. It is logged by
// the engine as a normal successful run. Detecting an unstaffed weekend is a
// monitoring concern, not this task's job — and notably it is the state the
// ServiceNow flow silently sat in for over a year.
func SendNotice(
	rotations RotationSearcher,
	email EmailSender,
	now func() time.Time,
	to, cc []string,
	emailsEnabled bool,
) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		if !emailsEnabled {
			return nil
		}

		saturday := NextSaturday(now())
		sunday := saturday.AddDate(0, 0, 1)
		weekendTypes := []string{entityrotations.TypeWeekend, entityrotations.TypeWeekendNight}

		var members []entityrotations.Member
		for _, day := range []time.Time{saturday, sunday} {
			found, err := rotations.SearchRotations(ctx, weekendTypes, day.Format(dateLayout))
			if err != nil {
				return fmt.Errorf("weekendrotation: search roster for %s: %w", day.Format(dateLayout), err)
			}
			members = append(members, found...)
		}
		if len(members) == 0 {
			return nil
		}

		recipients := mergeRecipients(to, memberEmails(members))
		if len(recipients) == 0 {
			return nil
		}

		subject := fmt.Sprintf("You have been allocated for weekend support (%s – %s)",
			saturday.Format(dateLayout), sunday.Format(dateLayout))
		body := notify.RenderWeekendRotationNotice(notify.WeekendRotationNoticeData{
			Saturday: saturday.Format(dateLayout),
			Sunday:   sunday.Format(dateLayout),
			Members:  toNoticeMembers(members),
		})
		if err := email.SendEmail(ctx, recipients, cc, subject, body); err != nil {
			return fmt.Errorf("weekendrotation: send notice email: %w", err)
		}
		return nil
	}
}

// memberEmails returns each rostered person's address, lower-cased and
// de-duplicated: one person covering both the day and night shift, or both
// days, must be mailed once, not four times.
func memberEmails(members []entityrotations.Member) []string {
	seen := make(map[string]bool, len(members))
	out := make([]string, 0, len(members))
	for _, m := range members {
		addr := strings.ToLower(strings.TrimSpace(m.Email))
		if addr == "" || seen[addr] {
			continue
		}
		seen[addr] = true
		out = append(out, addr)
	}
	sort.Strings(out)
	return out
}

// mergeRecipients unions the configured extra recipients with the roster's,
// de-duplicated case-insensitively and ordered so the result is stable.
func mergeRecipients(configured, roster []string) []string {
	seen := make(map[string]bool, len(configured)+len(roster))
	out := make([]string, 0, len(configured)+len(roster))
	for _, group := range [][]string{roster, configured} {
		for _, addr := range group {
			norm := strings.ToLower(strings.TrimSpace(addr))
			if norm == "" || seen[norm] {
				continue
			}
			seen[norm] = true
			out = append(out, norm)
		}
	}
	return out
}

// toNoticeMembers converts to the template's own shape, so internal/notify
// never imports the client package.
func toNoticeMembers(members []entityrotations.Member) []notify.WeekendRotationMember {
	out := make([]notify.WeekendRotationMember, 0, len(members))
	for _, m := range members {
		out = append(out, notify.WeekendRotationMember{
			Name: m.Name, Email: m.Email, Date: m.Date, Type: m.Type, Notes: m.Notes,
		})
	}
	return out
}
