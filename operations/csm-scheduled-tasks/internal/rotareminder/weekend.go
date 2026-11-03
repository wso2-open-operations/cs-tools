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

package rotareminder

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/teamschedule"
)

// weekendKind is the weekend notice's record kind in entity-service: Sunday's
// to Thursday's checks and Friday's reminder share it, so each knows what the
// other sent.
const weekendKind = "weekend_notice"

// WeekendNotice returns the handler run every evening from Sunday to Thursday
// for the coming weekend. Once the weekend is fully marked it sends the
// summary and the invitations and records the send, after which it sends
// nothing more; until then it asks the rota admins to finish the rota.
func WeekendNotice(rota RotaReader, store ReminderStore, email EmailSender, cfg Config) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		return weekend(ctx, rota, store, email, cfg, false)
	}
}

// WeekendReminder returns Friday's handler: the summary and the invitations
// again (as updates), cancellations for anybody taken off a window since, and,
// when the rota still has gaps, an email to the rota admins.
func WeekendReminder(rota RotaReader, store ReminderStore, email EmailSender, cfg Config) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		return weekend(ctx, rota, store, email, cfg, true)
	}
}

// gap is one weekend shift on one day with fewer people than it needs.
type gap struct {
	day        time.Time
	base       string
	have, need int
}

// missingSlots is every required weekend shift, on every day, marked by fewer
// people than cfg.Required asks for. Only the shift itself counts, never its
// on-call variant.
func missingSlots(cfg Config, rostered []teamschedule.Assignment, days []time.Time) []gap {
	codes := make([]string, 0, len(cfg.Required))
	for code := range cfg.Required {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	var gaps []gap
	for _, d := range days {
		date := d.Format(dateLayout)
		for _, code := range codes {
			need := cfg.Required[code]
			seen := map[string]bool{}
			for _, a := range rostered {
				if a.RotaDate == date && a.ShiftCode == code {
					seen[strings.ToLower(a.Engineer.UserID+"|"+a.Engineer.Email+"|"+a.Engineer.Name)] = true
				}
			}
			if len(seen) < need {
				gaps = append(gaps, gap{day: d, base: code, have: len(seen), need: need})
			}
		}
	}
	return gaps
}

// gapList writes gaps as a sentence fragment: "Saturday LK Weekend Rotation (2
// of 3), Sunday Americas Weekend Rotation (0 of 1)".
func gapList(cfg Config, gaps []gap) string {
	var parts []string
	for _, g := range gaps {
		parts = append(parts, fmt.Sprintf("%s %s (%d of %d)", g.day.Format("Monday"), windowTitle(cfg, g.base), g.have, g.need))
	}
	return strings.Join(parts, ", ")
}

// weekend is both weekend tasks. final is Friday's reminder.
func weekend(ctx context.Context, rota RotaReader, store ReminderStore, sender EmailSender, cfg Config, final bool) error {
	if !cfg.EmailsEnabled {
		return nil
	}
	cfg, on, err := withSettings(ctx, cfg, true)
	if err != nil || !on {
		return err
	}
	sat, sun := ComingWeekend(dayOf(cfg.anchor(), cfg.Zone))
	days := []time.Time{sat, sun}
	period := sat.Format(dateLayout)

	// A test neither reads nor writes the record: it is not a send to the
	// rota, and must not change what the real reminders do.
	var rec *teamschedule.ReminderRecord
	if !cfg.Test {
		var err error
		if rec, err = store.GetReminder(ctx, cfg.Family, weekendKind, period); err != nil {
			return fmt.Errorf("rotareminder: read what was sent for the weekend of %s: %w", period, err)
		}
	}
	if !final && rec != nil {
		slog.InfoContext(ctx, "rota reminder: the weekend was already announced; nothing sent", "weekend", period, "sentOn", rec.LastSentOn)
		cfg.Tally.skipped("the weekend of " + longDay(sat) + " was already announced")
		return nil
	}

	cat, err := rota.Catalogue(ctx)
	if err != nil {
		return fmt.Errorf("rotareminder: read the Team Schedule catalogue: %w", err)
	}
	wantedDay := map[string]time.Time{sat.Format(dateLayout): sat, sun.Format(dateLayout): sun}
	assignments, err := rota.WorkingAssignments(ctx, sat.Format(dateLayout), sun.Format(dateLayout), cfg.Family)
	if err != nil {
		return fmt.Errorf("rotareminder: read the rota for the weekend of %s: %w", period, err)
	}
	rostered := selectRostered(cfg, cat, assignments, wantedDay)
	windows := groupWindows(rostered, wantedDay)
	gaps := missingSlots(cfg, rostered, days)

	var failed []failure
	if len(gaps) > 0 && !final {
		// Nothing goes to the rota until it is complete; the next evening's
		// run checks again. A failed admin email fails the run, which is
		// retried until then.
		adminErr := askRotaAdmins(ctx, store, sender, cfg, days, gaps, final)
		slog.InfoContext(ctx, "rota reminder: the weekend is not fully marked; the rota admins were asked",
			"weekend", period, "gaps", gapList(cfg, gaps))
		cfg.Tally.skipped("the weekend of " + longDay(sat) + " is not fully marked; the rota admins were asked")
		return adminErr
	}

	now := cfg.Now()
	today := dayOf(now, cfg.Zone)
	var pending, started []*window
	for _, w := range windows {
		if w.start().After(now) {
			pending = append(pending, w)
		} else {
			started = append(started, w)
		}
	}
	if len(windows) == 0 {
		slog.InfoContext(ctx, "rota reminder: nobody is rostered for the weekend; nothing sent", "weekend", period)
		cfg.Tally.skipped("nobody is rostered for the weekend of " + longDay(sat))
		return nil
	}
	if len(pending) == 0 {
		slog.InfoContext(ctx, "rota reminder: nothing of the weekend is still to come; nothing sent", "weekend", period)
		cfg.Tally.skipped("every window of the weekend of " + longDay(sat) + " has already started")
		return nil
	}

	summary, ok := buildWeekendEmail(cfg, pending, rostered, today, gaps)
	if !ok {
		slog.WarnContext(ctx, "rota reminder: nobody on the weekend rota has an email address; nothing sent", "weekend", period)
		cfg.Tally.skipped("nobody on the weekend rota of " + longDay(sat) + " has an email address")
		return nil
	}
	err = send(ctx, sender, cfg, summary)
	cfg.Tally.count(err)
	if err != nil {
		// Nothing has gone to the rota yet, so the run fails and is retried.
		return fmt.Errorf("rotareminder: the weekend summary for %s: %w", period, err)
	}
	// Friday tells the rota admins what is still missing only once the
	// summary has gone: their email says it was sent, and a failed summary
	// fails the run, whose retry would otherwise ask them again each time.
	if len(gaps) > 0 {
		if err := askRotaAdmins(ctx, store, sender, cfg, days, gaps, final); err != nil {
			failed = append(failed, failure{label: "the email to the rota admins", err: err})
		}
	}

	var invites []teamschedule.ReminderInvite
	if cfg.Invites {
		previous := map[string]teamschedule.ReminderInvite{}
		if rec != nil {
			for _, inv := range rec.Invites {
				previous[inv.UID] = inv
			}
		}
		current := map[string]bool{}
		for _, w := range pending {
			// Turned off for this rotation: nothing is sent, and an invitation
			// already sent is neither updated nor withdrawn (it stays in the
			// record, so it is not taken for a window gone from the rota).
			if !cfg.invitesFor(w.base) {
				uid := cfg.inviteUID(w.date, w.base)
				current[uid] = true
				if old, ok := previous[uid]; ok {
					invites = append(invites, old)
				}
				continue
			}
			email, inv, ok := buildInviteEmail(cfg, w, now)
			if !ok {
				continue
			}
			current[inv.UID] = true
			err := send(ctx, sender, cfg, email)
			cfg.Tally.count(err)
			if err != nil {
				failed = append(failed, failure{windows: []*window{w}, err: err})
				if old, ok := previous[inv.UID]; ok {
					invites = append(invites, old) // keep who still holds the old version
				}
				continue
			}
			invites = append(invites, teamschedule.ReminderInvite{UID: inv.UID, Sequence: inv.Sequence, Attendees: inv.attendeeEmails()})
			if old, ok := previous[inv.UID]; ok {
				if removed := without(old.Attendees, inv.attendeeEmails()); len(removed) > 0 {
					if err := send(ctx, sender, cfg, buildCancelEmail(cfg, inv, removed, now)); err != nil {
						failed = append(failed, failure{label: "the cancellation for " + inv.Summary, err: err})
					}
				}
			}
		}
		// A window that has begun is not touched; one that is gone from the
		// rota is withdrawn from everybody who was sent it.
		for _, w := range started {
			uid := cfg.inviteUID(w.date, w.base)
			current[uid] = true
			if old, ok := previous[uid]; ok {
				invites = append(invites, old)
			}
		}
		for uid, old := range previous {
			if current[uid] || len(old.Attendees) == 0 {
				continue
			}
			gone := Invite{UID: uid, Summary: cfg.Family + " weekend rotation", OrganizerName: "CSM Team Schedule", OrganizerEmail: cfg.OrganizerEmail}
			gone.Start, gone.End = removedWindowSpan(uid, cfg, sat, sun)
			if err := send(ctx, sender, cfg, buildCancelEmail(cfg, gone, old.Attendees, now)); err != nil {
				failed = append(failed, failure{label: "the cancellation of " + uid, err: err})
				invites = append(invites, old)
			}
		}
	}

	sentBy := cfg.Task
	if sentBy == "" {
		sentBy = "rota reminder"
	}
	if cfg.Test {
		slog.InfoContext(ctx, "rota reminder: weekend test sent; not recorded", "weekend", period, "to", cfg.RedirectTo)
	} else if err := store.RecordReminder(ctx, cfg.Family, weekendKind, period, invites, sentBy); err != nil {
		// The emails went out; failing the run would send them again.
		failed = append(failed, failure{label: "recording the send (the next evening may send it again)", err: err})
	}
	slog.InfoContext(ctx, "rota reminder: weekend sent", "weekend", period, "final", final,
		"windows", len(pending), "invites", len(invites), "gaps", len(gaps), "failed", len(failed))
	if len(failed) > 0 {
		var errs []error
		for _, f := range failed {
			errs = append(errs, f.err)
		}
		slog.ErrorContext(ctx, "rota reminder: some weekend emails failed; not retried, so the rest are not sent twice", "err", errors.Join(errs...))
		alertPartialFailure(ctx, sender, cfg, failed)
	}
	return nil
}

// removedWindowSpan is the hours of a window that is no longer on the rota,
// for its cancellation: the weekend day its identity names, all day. The
// identity alone says which day; a calendar matches a cancellation by its
// identity, not its hours.
func removedWindowSpan(uid string, cfg Config, sat, sun time.Time) (time.Time, time.Time) {
	day := sat
	if strings.Contains(uid, sun.Format(dateLayout)) {
		day = sun
	}
	return day, day.AddDate(0, 0, 1)
}

// askRotaAdmins emails the family's rota admins what is missing. With no rota
// admin, the task's alert audience is asked instead.
func askRotaAdmins(ctx context.Context, store ReminderStore, sender EmailSender, cfg Config, days []time.Time, gaps []gap, final bool) error {
	admins, err := store.RotaAdmins(ctx, cfg.Family)
	if err != nil {
		return fmt.Errorf("rotareminder: read the %s rota admins: %w", cfg.Family, err)
	}
	if len(admins) == 0 {
		for _, a := range cfg.AlertTo {
			admins = append(admins, teamschedule.RotaAdmin{Email: a})
		}
	}
	email, ok := buildAdminEmail(cfg, admins, days, gaps, final)
	if !ok {
		slog.WarnContext(ctx, "rota reminder: the weekend is not fully marked and there is nobody to tell", "family", cfg.Family)
		return nil
	}
	return send(ctx, sender, cfg, email)
}

// send sends one email, to the redirect inbox where reminders are redirected.
func send(ctx context.Context, sender EmailSender, cfg Config, email shiftEmail) error {
	to, cc := email.to, email.cc
	if len(cfg.RedirectTo) > 0 {
		to, cc = cfg.RedirectTo, nil
	}
	return sender.SendEmailWithAttachments(ctx, to, cc, cfg.subject(email.subject), email.body, email.attachments)
}
