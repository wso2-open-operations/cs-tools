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
// Schedule, one email per shift window. Two sub-crons share it:
//
//   - WeekdayReminder: the next working day's Morning (with its on-call
//     person) and Evening windows. A Monday is announced on the Friday before
//     it, not on the Sunday, when nobody reads work mail or can arrange a swap.
//   - WeekendNotice: the coming weekend's LK and America windows, one email
//     for each window on each day, on the Monday (advance notice) and the
//     Friday (reminder) before it.
//
// A window is a shift and its on-call variant (<CODE>_OC): they share hours,
// so they share one email and one event. Each email goes to the people on its
// window and lists names and rotations only; a weekend email lists the whole
// weekend's rota so everybody sees who else is on.
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
	"sort"
	"strings"
	"time"

	// The rota is counted in an IANA zone (Asia/Colombo). A container image
	// without tzdata could not load it; embedding the database makes the
	// dates in an email independent of the image.
	_ "time/tzdata"

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
	// RedirectTo, when set, receives every email instead of its real
	// recipients, who are listed in the body. For environments that hold the
	// real rota but must not email real people.
	RedirectTo []string
	// ScheduleURL, when set, is linked from every email.
	ScheduleURL string
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
		day := dayOf(cfg.anchor(), cfg.Zone)
		return announce(ctx, rota, email, cfg, []time.Time{NextWorkingDay(day)})
	}
}

// WeekendNotice returns the handler that emails the coming weekend's windows.
func WeekendNotice(rota RotaReader, email EmailSender, cfg Config) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		sat, sun := ComingWeekend(dayOf(cfg.anchor(), cfg.Zone))
		return announce(ctx, rota, email, cfg, []time.Time{sat, sun})
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

// window is one shift and its on-call variant on one rota day: one email, one
// event.
type window struct {
	date   string
	day    time.Time
	base   string
	people []teamschedule.Assignment
}

// announce reads the rota for days and sends one email per window that has
// not started yet. The run fails, and so is retried, only when no email went
// out: a retry then tells nobody twice. When some went out and some failed,
// the failed windows are reported to the alert audience and the run succeeds.
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
		return nil
	}

	// A weekend email lists the whole weekend, a weekday email its own window.
	wholeRota := len(days) > 1
	// "Tomorrow" and "today" in an email are the day it is read, not the
	// anchor's: a retry can go out the day after its scheduled run.
	now := cfg.Now()
	today := dayOf(now, cfg.Zone)

	var errs []error
	var failed []*window
	sent := 0
	for _, w := range windows {
		if !w.start().After(now) {
			// A retry that runs late: telling people about a shift they are
			// already on (or that is over) helps nobody.
			slog.WarnContext(ctx, "rota reminder: this window has already started; not sent",
				"window", w.base, "day", w.date)
			continue
		}
		email, ok := buildEmail(cfg, w, rostered, wholeRota, today)
		if !ok {
			slog.WarnContext(ctx, "rota reminder: nobody on this window has an email address; nothing sent",
				"window", w.base, "day", w.date)
			continue
		}
		sendTo, sendCc := email.to, email.cc
		if len(cfg.RedirectTo) > 0 {
			sendTo, sendCc = cfg.RedirectTo, nil
		}
		if err := sender.SendEmail(ctx, sendTo, sendCc, email.subject, email.body); err != nil {
			errs = append(errs, fmt.Errorf("%s on %s: %w", w.base, w.date, err))
			failed = append(failed, w)
			continue
		}
		sent++
	}
	slog.InfoContext(ctx, "rota reminder run complete",
		"days", from+".."+to, "rostered", len(rostered), "windows", len(windows), "sent", sent)
	if len(errs) == 0 {
		return nil
	}
	// Out of the emails tried: a window skipped above was never sent.
	err = fmt.Errorf("rotareminder: %d of %d emails failed: %w", len(errs), sent+len(errs), errors.Join(errs...))
	if sent == 0 {
		return err
	}
	slog.ErrorContext(ctx, "rota reminder: some emails failed; not retried, so the rest are not sent twice", "err", err)
	alertPartialFailure(ctx, sender, cfg, failed, errs)
	return nil
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

// alertPartialFailure tells the alert audience which windows' emails failed
// in a run that otherwise went out, and whom they were for, so that a lead
// can tell those people by hand. A run like that succeeds and is not retried,
// so this is the only alert about it.
func alertPartialFailure(ctx context.Context, sender EmailSender, cfg Config, failed []*window, errs []error) {
	if len(cfg.AlertTo) == 0 {
		return
	}
	data := notify.RotaReminderFailureData{Task: cfg.Task}
	for i, w := range failed {
		var names []string
		for _, a := range w.people {
			names = append(names, a.Engineer.Name)
		}
		data.Rows = append(data.Rows, []string{
			windowTitle(cfg, w.base), w.day.Format("Monday, 2 January 2006"), strings.Join(names, ", "), errs[i].Error(),
		})
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
