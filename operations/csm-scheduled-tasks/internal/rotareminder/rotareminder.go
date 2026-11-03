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

// Package rotareminder emails the people on the rotation shifts in the Team
// Schedule, each with a calendar invitation for their window that has
// everybody on the window as a guest (so they can start a group chat from it).
//
//   - WeekdayReminder: the next working day's Morning (with its on-call
//     person) and Evening windows, one email per window carrying its
//     invitation. A Monday is announced on the Friday before it.
//   - WeekendNotice: runs every evening from Sunday to Thursday for the coming
//     weekend. Once the weekend is fully marked (Config.Required: by default
//     3 on LK Weekend and 1 on Americas Weekend, each day) it sends one summary
//     email to everybody on the weekend and one invitation email per team per
//     day (LK Saturday, Americas Saturday, LK Sunday, Americas Sunday), then
//     records the send so later evenings send nothing. Until then it emails
//     the family's rota admins what is missing, every evening.
//   - WeekendReminder: Friday's reminder. The summary again, the invitations
//     again as updates (same event identities), and a cancellation for
//     anybody taken off a window since the last send. With gaps it still
//     sends, listing them, and tells the rota admins.
//
// A window is a shift and its on-call variant (<CODE>_OC): they share hours,
// so they share one invitation.
//
// Which days a task runs on is its cron schedule (cmd/server/main.go). This
// package only decides, from the day it runs, which rota days it announces.
// Who is away is entity-service's decision: every rota read asks it to leave
// out people on leave (see internal/teamschedule).
package rotareminder

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	// The rota is counted in an IANA zone (Asia/Colombo). A container image
	// without tzdata could not load it; embedding the database makes the
	// dates in an email independent of the image.
	_ "time/tzdata"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/notify"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/teamschedule"
)

// RotaReader is the subset of *teamschedule.Client this package depends on.
type RotaReader interface {
	Catalogue(ctx context.Context) (teamschedule.Catalogue, error)
	WorkingAssignments(ctx context.Context, from, to, family string) ([]teamschedule.Assignment, error)
}

// EmailSender is the subset of *notify.Client this package depends on.
type EmailSender interface {
	SendEmail(ctx context.Context, to, cc []string, subject, htmlBody string) error
	SendEmailWithAttachments(ctx context.Context, to, cc []string, subject, htmlBody string, attachments []notify.Attachment) error
}

// SettingsReader is the subset of *teamschedule.Client that reads the
// reminder settings saved on the Rota Access tab.
type SettingsReader interface {
	ReminderSettingsFor(ctx context.Context, family string) (*teamschedule.ReminderSettings, error)
}

// ReminderStore is the subset of *teamschedule.Client the weekend tasks keep
// their record in, and ask who the rota admins are.
type ReminderStore interface {
	GetReminder(ctx context.Context, family, kind, periodStart string) (*teamschedule.ReminderRecord, error)
	RecordReminder(ctx context.Context, family, kind, periodStart string, invites []teamschedule.ReminderInvite, sentBy string) error
	RotaAdmins(ctx context.Context, family string) ([]teamschedule.RotaAdmin, error)
}

// Config is what both tasks are built from. Everything about who is reminded
// of what is configuration, set in cmd/server/main.go from the environment.
type Config struct {
	// Family is the Team Schedule family read (CRE today).
	Family string
	// Rotations maps each shift code this task announces to the name the
	// email shows for it ("CRE_MORNING_OC" -> "On-call"). A code that is not
	// here is not announced.
	Rotations map[string]string
	// Copy is copied on every email (a lead who wants to see them). Empty by
	// default: the emails go to the people on the shift.
	Copy []string
	// WeekendCopy is copied on the weekend summary email only, beside Copy:
	// somebody who wants to know who is on the weekend, not every weekday
	// reminder or invitation. Set on the Rota Access tab.
	WeekendCopy []string
	// RedirectTo, when set, receives every email instead of its real
	// recipients, who are listed in the body. For environments that hold the
	// real rota but must not email real people.
	RedirectTo []string
	// ScheduleURL, when set, is linked from every email and is each event's
	// link.
	ScheduleURL string
	// Invites attaches a calendar invitation to each window's email, with
	// everybody on the window as a guest.
	Invites bool
	// InvitesOff names the rotations (the shift code of a window: CRE_MORNING,
	// CRE_WEEKEND_NIGHT) whose invitations are turned off on the Rota Access
	// tab, while Invites stays on for the rest. Read through invitesFor.
	InvitesOff map[string]bool
	// OrganizerEmail is the invitations' organizer: the address the emails
	// come from (EMAIL_FROM_ADDRESS).
	OrganizerEmail string
	// Required is how many people each weekend shift needs on each day before
	// the weekend is announced ("CRE_WEEKEND" -> 3, "CRE_WEEKEND_NIGHT" -> 1).
	// Only people on the shift itself count, not its on-call variant.
	Required map[string]int
	// Support is another family's weekend turns the weekend summary also
	// lists (the SRE people on the CRE weekend). They are added to the
	// summary's recipients only: they are not counted for the minimums and
	// get no invitation. The zero value lists none.
	Support SupportRota
	// Windows gives a rotation its own hours on its own clock, instead of the
	// Team Schedule's ("CRE_WEEKEND_NIGHT" -> 08:00-18:00 America/Los_Angeles).
	Windows map[string]LocalWindow
	// Settings, when set, is read at the start of every run: what the rota
	// admins saved on the Team Schedule's Rota Access tab replaces the
	// switches, the weekend minimums, the windows and adds to Copy. Nothing
	// saved keeps this configuration.
	Settings SettingsReader
	// Manual is a "Send now" run: the saved on/off switches do not apply.
	Manual bool
	// Test is "Send a test to me": every email goes to RedirectTo (the person
	// who asked) with "[Test]" in its subject, invitations carry test event
	// ids, and the weekend's send record is neither read nor written.
	Test bool
	// Tally, when set, counts the reminder emails the run sent and could not
	// send, and notes why nothing was sent when nothing was: the result a
	// "Send now" request reports to the card.
	Tally *Tally
	// EmailsEnabled is ALERTS_ENABLED, this component's single switch for
	// every email it sends. When false a run succeeds and sends nothing.
	EmailsEnabled bool
	// Zone is the calendar rota days are counted in: "tomorrow" means
	// tomorrow there. The rota's own office zone, Asia/Colombo.
	Zone *time.Location
	// Now returns the current time; injected so tests are deterministic.
	Now func() time.Time
	// Anchor returns the scheduled firing time of the run being made (its
	// ledger period key). The rota days are counted from it, not from Now: a
	// failed run is retried every tick until its next period, so a retry of
	// Thursday's run on Friday must still announce Friday, and one of
	// Friday's weekend notice on Saturday this weekend, not the next. Nil
	// means Now.
	Anchor func() time.Time
	// Task names the task in a partial-failure alert.
	Task string
	// AlertTo and AlertCc get the alert when some windows' emails failed and
	// others went out (the engine's failure audience: the task's own
	// recipients and ALERT_RECIPIENTS). Such a run succeeds rather than fail,
	// so that its retry does not email again everybody who was already told.
	AlertTo, AlertCc []string
}

// WeekdayReminder returns the handler that emails the next working day's
// weekday windows.
func WeekdayReminder(rota RotaReader, email EmailSender, cfg Config) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		if !cfg.EmailsEnabled {
			return nil
		}
		cfg, on, err := withSettings(ctx, cfg, false)
		if err != nil || !on {
			return err
		}
		day := dayOf(cfg.anchor(), cfg.Zone)
		return announce(ctx, rota, email, cfg, []time.Time{NextWorkingDay(day)})
	}
}

// anchor is the moment the run's rota days are counted from (Config.Anchor).
func (cfg Config) anchor() time.Time {
	if cfg.Anchor != nil {
		return cfg.Anchor()
	}
	return cfg.Now()
}

// NextWorkingDay is the first Monday-to-Friday day after today: tomorrow from
// Monday to Thursday, Monday from Friday, Saturday or Sunday.
func NextWorkingDay(today time.Time) time.Time {
	d := today.AddDate(0, 0, 1)
	for d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
		d = d.AddDate(0, 0, 1)
	}
	return d
}

// ComingWeekend is the first Saturday after today and the Sunday after it:
// from Monday the Saturday five days on, from Friday tomorrow. From a Saturday
// or a Sunday it is next week's, since this weekend is already under way.
func ComingWeekend(today time.Time) (sat, sun time.Time) {
	ahead := (int(time.Saturday) - int(today.Weekday()) + 7) % 7
	if ahead == 0 {
		ahead = 7
	}
	sat = today.AddDate(0, 0, ahead)
	return sat, sat.AddDate(0, 0, 1)
}

// dayOf is the calendar day t falls on in zone, at midnight.
func dayOf(t time.Time, zone *time.Location) time.Time {
	t = t.In(zone)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, zone)
}

const dateLayout = "2006-01-02"

// window is one shift and its on-call variant on one rota day: on a weekday,
// one email.
type window struct {
	date   string
	day    time.Time
	base   string
	people []teamschedule.Assignment
}

// announce reads the rota for a weekday and emails each window that has not
// started yet, with its invitation. The run fails, and so is retried, only when no email went out: a
// retry then tells nobody twice. When some went out and some failed, the
// failed emails are reported to the alert audience and the run succeeds.
func announce(ctx context.Context, rota RotaReader, sender EmailSender, cfg Config, days []time.Time) error {
	if !cfg.EmailsEnabled {
		return nil
	}

	cat, err := rota.Catalogue(ctx)
	if err != nil {
		return fmt.Errorf("rotareminder: read the Team Schedule catalogue: %w", err)
	}
	wantedDay := make(map[string]time.Time, len(days))
	for _, d := range days {
		wantedDay[d.Format(dateLayout)] = d
	}
	from, to := days[0].Format(dateLayout), days[len(days)-1].Format(dateLayout)
	assignments, err := rota.WorkingAssignments(ctx, from, to, cfg.Family)
	if err != nil {
		return fmt.Errorf("rotareminder: read the rota for %s to %s: %w", from, to, err)
	}
	rostered := selectRostered(cfg, cat, assignments, wantedDay)
	windows := groupWindows(rostered, wantedDay)
	if len(windows) == 0 {
		slog.InfoContext(ctx, "rota reminder: nobody is rostered; nothing sent", "days", from+".."+to)
		cfg.Tally.skipped("nobody is rostered on " + dayList(days))
		return nil
	}

	// "Tomorrow" and "today" in an email are the day it is read, not the
	// anchor's: a retry can go out the day after its scheduled run.
	now := cfg.Now()
	today := dayOf(now, cfg.Zone)

	var pending []*window
	for _, w := range windows {
		if !w.start().After(now) {
			// A retry that runs late: telling people about a shift they are
			// already on (or that is over) helps nobody.
			slog.WarnContext(ctx, "rota reminder: this window has already started; not sent",
				"window", w.base, "day", w.date)
			cfg.Tally.skipped(windowTitle(cfg, w.base) + " on " + longDay(w.day) + " has already started")
			continue
		}
		pending = append(pending, w)
	}

	// One email per window, to its own people, carrying its invitation.
	type batch struct {
		email   shiftEmail
		windows []*window
	}
	var batches []batch
	for _, w := range pending {
		email, ok := buildEmail(cfg, w, today)
		if !ok {
			slog.WarnContext(ctx, "rota reminder: nobody on this window has an email address; nothing sent",
				"window", w.base, "day", w.date)
			cfg.Tally.skipped("nobody on " + windowTitle(cfg, w.base) + " on " + longDay(w.day) + " has an email address")
			continue
		}
		if cfg.invitesFor(w.base) {
			inv := windowInvite(cfg, w, now)
			email.attachments = []notify.Attachment{inviteAttachment(inv)}
		}
		batches = append(batches, batch{email, []*window{w}})
	}

	var errs []error
	var failed []failure
	sent := 0
	for _, b := range batches {
		sendTo, sendCc := b.email.to, b.email.cc
		if len(cfg.RedirectTo) > 0 {
			sendTo, sendCc = cfg.RedirectTo, nil
		}
		b.email.subject = cfg.subject(b.email.subject)
		err := sendWindowEmail(ctx, sender, sendTo, sendCc, b.email)
		cfg.Tally.count(err) // after the fallback: the email's final outcome
		if err != nil {
			var names []string
			for _, w := range b.windows {
				names = append(names, w.base+" on "+w.date)
			}
			err = fmt.Errorf("%s: %w", strings.Join(names, ", "), err)
			errs = append(errs, err)
			failed = append(failed, failure{windows: b.windows, err: err})
			continue
		}
		sent++
	}
	slog.InfoContext(ctx, "rota reminder run complete",
		"days", from+".."+to, "rostered", len(rostered), "windows", len(windows), "emails", len(batches), "sent", sent)
	if len(errs) == 0 {
		return nil
	}
	// Out of the emails tried: a window skipped above was never sent.
	err = fmt.Errorf("rotareminder: %d of %d emails failed: %w", len(errs), sent+len(errs), errors.Join(errs...))
	if sent == 0 {
		return err
	}
	slog.ErrorContext(ctx, "rota reminder: some emails failed; not retried, so the rest are not sent twice", "err", err)
	alertPartialFailure(ctx, sender, cfg, failed)
	return nil
}

// invitesFor is whether the window whose shift code is base gets a calendar
// invitation: invitations on, and not turned off for that rotation.
func (cfg Config) invitesFor(base string) bool {
	return cfg.Invites && !cfg.InvitesOff[base]
}

// span is when the window runs: its earliest start to its latest end.
func (w *window) span() (start, end time.Time) {
	start, end = w.people[0].StartsAt, w.people[0].EndsAt
	for _, a := range w.people[1:] {
		if a.StartsAt.Before(start) {
			start = a.StartsAt
		}
		if a.EndsAt.After(end) {
			end = a.EndsAt
		}
	}
	return start, end
}

// sendWindowEmail sends a weekday window's email with its invitation. An
// email service that rejects the invitation (HTTP 400) is sent the email
// again without it: the reminder matters more than the calendar entry, and a
// 400 means nothing went out, so nobody gets it twice. (A weekend invitation
// email has no fallback: it exists for its invitation, and the summary has
// already told everybody.)
func sendWindowEmail(ctx context.Context, sender EmailSender, to, cc []string, email shiftEmail) error {
	err := sender.SendEmailWithAttachments(ctx, to, cc, email.subject, email.body, email.attachments)
	var apiErr *apierror.Error
	if len(email.attachments) > 0 && errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusBadRequest {
		slog.WarnContext(ctx, "rota reminder: the email service rejected the calendar invitation; sending the email without it",
			"subject", email.subject, "err", err)
		return sender.SendEmail(ctx, to, cc, email.subject, email.body)
	}
	return err
}

// start is when the earliest person on the window starts.
func (w *window) start() time.Time {
	s := w.people[0].StartsAt
	for _, a := range w.people[1:] {
		if a.StartsAt.Before(s) {
			s = a.StartsAt
		}
	}
	return s
}

// failure is one email that did not go out, and the windows it announced.
type failure struct {
	windows []*window
	// label names what failed when it is not a window's email (the rota
	// admins' email, a cancellation, recording the send).
	label string
	err   error
}

// alertPartialFailure tells the alert audience which windows' emails failed
// in a run that otherwise went out, and whom they were for, so that a lead
// can tell those people by hand. A run like that succeeds and is not retried,
// so this is the only alert about it.
func alertPartialFailure(ctx context.Context, sender EmailSender, cfg Config, failed []failure) {
	if len(cfg.AlertTo) == 0 {
		return
	}
	data := notify.RotaReminderFailureData{Task: cfg.Task}
	for _, f := range failed {
		if len(f.windows) == 0 {
			data.Rows = append(data.Rows, []string{f.label, "-", "-", f.err.Error()})
		}
		for _, w := range f.windows {
			var names []string
			for _, a := range w.people {
				names = append(names, a.Engineer.Name)
			}
			data.Rows = append(data.Rows, []string{
				windowTitle(cfg, w.base), w.day.Format("Monday, 2 January 2006"), strings.Join(names, ", "), f.err.Error(),
			})
		}
	}
	// Plain ASCII, as the engine's own alert subject.
	subject := fmt.Sprintf("[csm-scheduled-tasks] PARTLY FAILED: %s - %d rota reminder(s) not sent", cfg.Task, len(failed))
	if err := sender.SendEmail(ctx, cfg.AlertTo, cfg.AlertCc, subject, notify.RenderRotaReminderFailure(data)); err != nil {
		slog.ErrorContext(ctx, "rota reminder: failed to send the partial-failure alert", "err", err)
	}
}

// selectRostered keeps the assignments on an announced rotation and day, in
// rota order: by day, then the catalogue's shift order, then name. A shift the
// catalogue does not list sorts after every shift it does.
func selectRostered(cfg Config, cat teamschedule.Catalogue, assignments []teamschedule.Assignment, wantedDay map[string]time.Time) []teamschedule.Assignment {
	order := make(map[string]int, len(cat.Shifts))
	for _, s := range cat.Shifts {
		order[s.Code] = s.SortOrder
	}
	rank := func(code string) int {
		if o, ok := order[code]; ok {
			return o
		}
		return math.MaxInt
	}
	var rostered []teamschedule.Assignment
	for _, a := range assignments {
		if _, ok := wantedDay[a.RotaDate]; ok && cfg.Rotations[a.ShiftCode] != "" {
			rostered = append(rostered, a)
		}
	}
	applyWindows(cfg, rostered)
	sort.SliceStable(rostered, func(i, j int) bool {
		a, b := rostered[i], rostered[j]
		if a.RotaDate != b.RotaDate {
			return a.RotaDate < b.RotaDate
		}
		if rank(a.ShiftCode) != rank(b.ShiftCode) {
			return rank(a.ShiftCode) < rank(b.ShiftCode)
		}
		return strings.ToLower(a.Engineer.Name) < strings.ToLower(b.Engineer.Name)
	})
	return rostered
}

// baseCode is the shift an on-call variant belongs with: CRE_MORNING_OC is in
// CRE_MORNING's window.
func baseCode(code string) string {
	return strings.TrimSuffix(code, "_OC")
}

// groupWindows collects the rota into windows, in rota order.
func groupWindows(rostered []teamschedule.Assignment, days map[string]time.Time) []*window {
	index := map[string]*window{}
	var out []*window
	for _, a := range rostered {
		key := a.RotaDate + "|" + baseCode(a.ShiftCode)
		w, ok := index[key]
		if !ok {
			w = &window{date: a.RotaDate, day: days[a.RotaDate], base: baseCode(a.ShiftCode)}
			index[key] = w
			out = append(out, w)
		}
		w.people = append(w.people, a)
	}
	return out
}

// without returns the distinct addresses in list that are not in exclude, so
// nobody is both a recipient and copied, or copied twice.
func without(list, exclude []string) []string {
	skip := map[string]bool{}
	for _, e := range exclude {
		skip[strings.ToLower(e)] = true
	}
	var out []string
	for _, e := range list {
		if !skip[strings.ToLower(e)] {
			out = append(out, e)
			skip[strings.ToLower(e)] = true
		}
	}
	return out
}

// subject marks a test's emails in the inbox.
func (cfg Config) subject(s string) string {
	if cfg.Test {
		return "[Test] " + s
	}
	return s
}

// inviteUID is the event id of a window's invitation; a test's are its own,
// so a test event never stands in for, or blocks, the real one.
func (cfg Config) inviteUID(rotaDate, shiftCode string) string {
	uid := inviteUID(cfg.Family, rotaDate, shiftCode)
	if cfg.Test {
		uid = strings.Replace(uid, "@", "-test@", 1)
	}
	return uid
}
