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
	"fmt"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/notify"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/teamschedule"
)

// shiftEmail is one reminder email: a weekday window's, or the weekend's.
type shiftEmail struct {
	to, cc      []string
	subject     string
	body        string
	attachments []notify.Attachment
}

// recipients is the distinct addresses of people, and their names, in order.
func recipients(people []teamschedule.Assignment) (to, named []string) {
	seen := map[string]bool{}
	for _, a := range people {
		addr := strings.TrimSpace(a.Engineer.Email)
		if addr == "" || seen[strings.ToLower(addr)] {
			continue
		}
		seen[strings.ToLower(addr)] = true
		named = append(named, a.Engineer.Name)
		to = append(to, addr)
	}
	return to, named
}

// finish addresses an email to to, copies cfg.Copy, and renders data as its
// body.
func finish(cfg Config, to []string, subject string, data notify.RotaReminderData) shiftEmail {
	email := shiftEmail{to: to, cc: without(cfg.Copy, to), subject: subject}
	if len(cfg.RedirectTo) > 0 {
		data.IntendedFor = append(append([]string{}, email.to...), email.cc...)
	}
	email.body = notify.RenderRotaReminder(data)
	return email
}

// buildEmail writes a weekday window's email: to the people on it, with a
// table of them. ok is false when nobody on the window has an address.
func buildEmail(cfg Config, w *window, today time.Time) (shiftEmail, bool) {
	to, named := recipients(w.people)
	if len(to) == 0 {
		return shiftEmail{}, false
	}
	start, end := w.people[0].StartsAt, w.people[0].EndsAt
	for _, a := range w.people {
		if a.StartsAt.Before(start) {
			start = a.StartsAt
		}
		if a.EndsAt.After(end) {
			end = a.EndsAt
		}
	}

	title := windowTitle(cfg, w.base) // "Morning Rotation"
	inSentence := strings.Replace(title, " Rotation", " rotation", 1)
	when := windowText(start, end, clockOf(cfg, w.base)) // "06:00–09:00 IST"
	date := w.day.Format("Monday, 2 January 2006")       // "Tuesday, 13 October 2026"

	notice := fmt.Sprintf("This is an advance notice that you are rostered for the %s on %s, %s.", inSentence, date, when)
	if w.day.Equal(today) {
		// Only a retry that went out on the window's own day, before it began.
		notice = fmt.Sprintf("This is a reminder that you are rostered for the %s today, %s, %s.", inSentence, date, when)
	}
	if w.day.Equal(today.AddDate(0, 0, 1)) {
		notice = fmt.Sprintf("This is a reminder that you are rostered for the %s tomorrow, %s, %s.", inSentence, date, when)
	}
	if cfg.Invites {
		notice += " The calendar invitation is attached; everybody on this rotation is a guest, and no reply is needed."
	}

	data := notify.RotaReminderData{
		Greeting:    greeting(named),
		Intro:       notice,
		ScheduleURL: cfg.ScheduleURL,
		Columns:     []string{"Name", "Rotation"},
	}
	for _, a := range w.people {
		data.Rows = append(data.Rows, []string{a.Engineer.Name, cfg.Rotations[a.ShiftCode]})
	}
	return finish(cfg, to, fmt.Sprintf("[%s Operations] %s – %s (%s)", cfg.Family, title, date, when), data), true
}

// buildWeekendEmail writes the weekend's summary: to everybody on the windows
// still to come, LK and Americas alike. It lists each day's team, then defines
// each rotation once (its hours on its own clock and in UTC, and its role).
// gaps, when any, are listed too (Friday's reminder goes out with them). ok is
// false when nobody on them has an address.
func buildWeekendEmail(cfg Config, pending []*window, rostered []teamschedule.Assignment, today time.Time, gaps []gap) (shiftEmail, bool) {
	var people []teamschedule.Assignment
	var days []time.Time
	for _, w := range pending {
		people = append(people, w.people...)
		if len(days) == 0 || !days[len(days)-1].Equal(w.day) {
			days = append(days, w.day)
		}
	}
	to, _ := recipients(people)
	if len(to) == 0 {
		return shiftEmail{}, false
	}

	var dates []string
	for _, d := range days {
		dates = append(dates, d.Format(dateLayout))
	}
	which := "the coming weekend"
	if first := days[0]; !first.After(today.AddDate(0, 0, 1)) {
		which = "this weekend"
	}
	data := notify.WeekendRotaData{
		Intro: fmt.Sprintf("You have been allocated for weekend support %s (%s). Check your rotation window below and confirm "+
			"your contact details are current. If you need to swap a rotation or cannot fulfil it, inform your team lead immediately.",
			which, strings.Join(dates, " and ")),
		ScheduleURL: cfg.ScheduleURL,
	}

	// Each day's team, in rota order.
	for _, d := range days {
		day := notify.WeekendRotaDay{Title: d.Format("Monday") + " Team"}
		for _, a := range rostered {
			if a.RotaDate == d.Format(dateLayout) {
				day.People = append(day.People, fmt.Sprintf("%s (%s)", a.Engineer.Name, cfg.Rotations[a.ShiftCode]))
			}
		}
		data.Days = append(data.Days, day)
	}

	// Each rotation once, in rota order, described from its first turn.
	var codes []string
	first := map[string]teamschedule.Assignment{}
	perDay := map[string]map[string]map[string]bool{} // code -> date -> person
	for _, a := range rostered {
		if _, ok := first[a.ShiftCode]; !ok {
			first[a.ShiftCode] = a
			codes = append(codes, a.ShiftCode)
			perDay[a.ShiftCode] = map[string]map[string]bool{}
		}
		if perDay[a.ShiftCode][a.RotaDate] == nil {
			perDay[a.ShiftCode][a.RotaDate] = map[string]bool{}
		}
		perDay[a.ShiftCode][a.RotaDate][strings.ToLower(a.Engineer.UserID+"|"+a.Engineer.Email+"|"+a.Engineer.Name)] = true
	}
	zones := map[string]*time.Location{}
	var zoneOrder []string
	for _, code := range codes {
		a := first[code]
		clock := clockOf(cfg, code)
		s, e := a.StartsAt.In(clock), a.EndsAt.In(clock)
		role := zoneName(s) + " General support"
		if a.IsOnCall || strings.HasSuffix(code, "_OC") {
			role = "On-call"
		}
		data.Definitions = append(data.Definitions, notify.RotationDefinition{
			Rotation:  cfg.Rotations[code],
			Members:   memberCount(days, perDay[code]),
			LocalTime: s.Format("03:04 PM") + " – " + e.Format("03:04 PM") + " " + zoneName(s),
			UTCWindow: a.StartsAt.UTC().Format("15:04") + " – " + a.EndsAt.UTC().Format("15:04"),
			Role:      role,
		})
		if _, ok := zones[clock.String()]; !ok {
			zones[clock.String()] = clock
			zoneOrder = append(zoneOrder, clock.String())
		}
	}
	var legend []string
	for _, name := range zoneOrder {
		at := days[0].In(zones[name]).Add(12 * time.Hour)
		legend = append(legend, fmt.Sprintf("%s = %s (UTC%s)", zoneName(at), name, utcOffset(at)))
	}
	if len(legend) > 0 {
		data.ZoneNotes = append(data.ZoneNotes, strings.Join(legend, " • "))
	}
	for _, code := range codes {
		if lw, ok := cfg.Windows[code]; ok {
			if note := daylightNote(cfg, code, lw.Zone, days[0]); note != "" {
				data.ZoneNotes = append(data.ZoneNotes, note)
			}
		}
	}

	if len(gaps) > 0 {
		data.AfterTable = append(data.AfterTable, "Not yet fully marked: "+gapList(cfg, gaps)+".")
	}
	if cfg.Invites {
		data.AfterTable = append(data.AfterTable, "A calendar invitation for your team's rotation follows in a separate email, "+
			"with everybody on that rotation as a guest.")
	}
	email := shiftEmail{to: to, cc: without(cfg.Copy, to), subject: fmt.Sprintf("[%s Operations] Weekend Rotation – %s", cfg.Family, dayList(days))}
	if len(cfg.RedirectTo) > 0 {
		data.IntendedFor = append(append([]string{}, email.to...), email.cc...)
	}
	email.body = notify.RenderWeekendRota(data)
	return email, true
}

// memberCount says how many people a rotation has: "3 members", or, when the
// days differ, "Saturday 3, Sunday 2".
func memberCount(days []time.Time, byDay map[string]map[string]bool) string {
	var counts []int
	same := true
	for i, d := range days {
		counts = append(counts, len(byDay[d.Format(dateLayout)]))
		if i > 0 && counts[i] != counts[0] {
			same = false
		}
	}
	if same {
		if counts[0] == 1 {
			return "1 member"
		}
		return fmt.Sprintf("%d members", counts[0])
	}
	var parts []string
	for i, d := range days {
		parts = append(parts, fmt.Sprintf("%s %d", d.Format("Monday"), counts[i]))
	}
	return strings.Join(parts, ", ")
}

// utcOffset writes t's offset as "+5:30" or "−7".
func utcOffset(t time.Time) string {
	_, off := t.Zone()
	sign := "+"
	if off < 0 {
		sign, off = "−", -off
	}
	h, m := off/3600, off%3600/60
	if m == 0 {
		return fmt.Sprintf("%s%d", sign, h)
	}
	return fmt.Sprintf("%s%d:%02d", sign, h, m)
}

// daylightNote warns that a rotation kept on a clock with daylight saving
// moves in UTC (and IST) when that clock changes, if it changes within five
// weeks of the weekend.
func daylightNote(cfg Config, code string, zone *time.Location, from time.Time) string {
	noon := func(d time.Time) time.Time { return time.Date(d.Year(), d.Month(), d.Day(), 12, 0, 0, 0, zone) }
	start := noon(from)
	_, prev := start.Zone()
	for i := 1; i <= 35; i++ {
		d := noon(start.AddDate(0, 0, i))
		_, off := d.Zone()
		if off == prev {
			continue
		}
		change := "ends"
		if off > prev {
			change = "starts"
		}
		// Clocks change in the small hours, so the first noon on the new
		// offset is the day of the change.
		return fmt.Sprintf("Local times are fixed. The %s UTC window moves by one hour on %s, when daylight saving %s.",
			windowTitle(cfg, code), d.Format("02 Jan 2006"), change)
	}
	return ""
}

// buildInviteEmail writes the email that carries one weekend window's
// invitation: to the people on it, all of them guests of the event. ok is
// false when nobody on the window has an address.
func buildInviteEmail(cfg Config, w *window, stamp time.Time) (shiftEmail, Invite, bool) {
	to, named := recipients(w.people)
	if len(to) == 0 {
		return shiftEmail{}, Invite{}, false
	}
	start, end := w.span()
	title := windowTitle(cfg, w.base)
	inSentence := strings.Replace(title, " Rotation", " rotation", 1)
	date := w.day.Format("Monday, 2 January 2006")
	when := windowText(start, end, clockOf(cfg, w.base))

	data := notify.RotaReminderData{
		Greeting: greeting(named),
		Intro: fmt.Sprintf("This is the calendar invitation for the %s on %s, %s. Everybody on this rotation is a guest, "+
			"so you can start a group chat from the event; no reply is needed.", inSentence, date, when),
		ScheduleURL: cfg.ScheduleURL,
		Columns:     []string{"Name", "Rotation"},
	}
	for _, a := range w.people {
		data.Rows = append(data.Rows, []string{a.Engineer.Name, cfg.Rotations[a.ShiftCode]})
	}
	inv := windowInvite(cfg, w, stamp)
	email := finish(cfg, to, fmt.Sprintf("[%s Operations] Calendar invitation: %s – %s (%s)", cfg.Family, title, date, when), data)
	email.attachments = []notify.Attachment{inviteAttachment(inv)}
	return email, inv, true
}

// buildCancelEmail writes the email that withdraws an event from the people
// taken off it since it was sent.
func buildCancelEmail(cfg Config, inv Invite, emails []string, stamp time.Time) shiftEmail {
	cancel := inv
	cancel.Cancel = true
	cancel.Sequence = sequenceAt(stamp)
	cancel.Stamp = stamp
	cancel.Attendees = nil
	for _, e := range emails {
		cancel.Attendees = append(cancel.Attendees, Attendee{Email: e})
	}
	date := inv.Start.In(cfg.Zone).Format("Monday, 2 January 2006")
	data := notify.RotaReminderData{
		Greeting:    greeting(nil),
		Intro:       fmt.Sprintf("You are no longer rostered for the %s on %s, so its calendar invitation is withdrawn.", inv.Summary, date),
		ScheduleURL: cfg.ScheduleURL,
		Closing:     "If you believe this is a mistake, please contact your team lead.",
	}
	email := finish(cfg, emails, fmt.Sprintf("[%s Operations] Cancelled: %s – %s", cfg.Family, inv.Summary, date), data)
	email.attachments = []notify.Attachment{inviteAttachment(cancel)}
	return email
}

// buildAdminEmail writes the email that asks the family's rota admins to
// finish marking the weekend. final is Friday's: the reminder has already gone
// out with what was marked.
func buildAdminEmail(cfg Config, admins []teamschedule.RotaAdmin, days []time.Time, gaps []gap, final bool) (shiftEmail, bool) {
	var to, named []string
	seen := map[string]bool{}
	for _, a := range admins {
		addr := strings.TrimSpace(a.Email)
		if addr == "" || seen[strings.ToLower(addr)] {
			continue
		}
		seen[strings.ToLower(addr)] = true
		to = append(to, addr)
		named = append(named, a.Name)
	}
	if len(to) == 0 {
		return shiftEmail{}, false
	}
	when := dayList(days)
	intro := fmt.Sprintf("The %s weekend rota for %s is not fully marked in the Team Schedule, so the weekend reminder and its "+
		"calendar invitations have not been sent yet.", cfg.Family, when)
	after := "Please mark the missing rotations. The reminder goes out automatically at the next evening check once the rota is complete."
	if final {
		intro = fmt.Sprintf("The %s weekend rota for %s is still not fully marked in the Team Schedule. The weekend reminder has been "+
			"sent with the rotations marked so far.", cfg.Family, when)
		after = "Please mark the missing rotations and let the people concerned know."
	}
	data := notify.RotaReminderData{
		Greeting:    greeting(named),
		Intro:       intro,
		ScheduleURL: cfg.ScheduleURL,
		Columns:     []string{"Day", "Rotation", "Marked", "Needed"},
		AfterTable:  []string{after},
		Closing:     "This email goes to the " + cfg.Family + " rota admins.",
	}
	for _, g := range gaps {
		data.Rows = append(data.Rows, []string{g.day.Format("Monday, 2 January"), windowTitle(cfg, g.base), fmt.Sprint(g.have), fmt.Sprint(g.need)})
	}
	subject := fmt.Sprintf("[%s Operations] Action needed: the weekend rota for %s is not complete", cfg.Family, when)
	return finish(cfg, to, subject, data), true
}

// windowInvite is a window's event: its hours, everybody on it as a guest
// (the redirect inbox instead, where reminders are redirected), and a stable
// identity so a later send updates it.
func windowInvite(cfg Config, w *window, stamp time.Time) Invite {
	start, end := w.span()
	title := windowTitle(cfg, w.base)
	date := w.day.Format("Monday, 2 January 2006")
	var onIt []string
	for _, a := range w.people {
		onIt = append(onIt, fmt.Sprintf("%s (%s)", a.Engineer.Name, cfg.Rotations[a.ShiftCode]))
	}
	inv := Invite{
		UID:     cfg.inviteUID(w.date, w.base),
		Summary: cfg.Family + " " + title,
		Description: fmt.Sprintf("%s, %s, %s.\nOn this rotation: %s.\n\nNo reply is needed. If you are unable to cover your shift, "+
			"please arrange a swap with your team lead and have the Team Schedule updated accordingly.",
			title, date, windowText(start, end, clockOf(cfg, w.base)), strings.Join(onIt, ", ")),
		Start:          start,
		End:            end,
		OrganizerName:  "CSM Team Schedule",
		OrganizerEmail: cfg.OrganizerEmail,
		Sequence:       sequenceAt(stamp),
		Stamp:          stamp,
	}
	if len(cfg.RedirectTo) > 0 {
		for _, e := range cfg.RedirectTo {
			inv.Attendees = append(inv.Attendees, Attendee{Email: e})
		}
	} else {
		to, named := recipients(w.people)
		for i := range to {
			inv.Attendees = append(inv.Attendees, Attendee{Name: named[i], Email: to[i]})
		}
	}
	if ValidScheduleURL(cfg.ScheduleURL) {
		inv.Description += "\nLatest roster: " + cfg.ScheduleURL
		inv.URL = cfg.ScheduleURL
	}
	return inv
}

// inviteAttachment is an invite as the email service's attachment.
func inviteAttachment(inv Invite) notify.Attachment {
	return notify.Attachment{ContentName: "invite.ics", ContentType: inv.contentType(), Content: inv.ICS()}
}

// attendeeEmails is the invite's guests' addresses.
func (inv Invite) attendeeEmails() []string {
	var out []string
	for _, a := range inv.Attendees {
		out = append(out, a.Email)
	}
	return out
}

// dayList names rota days: "Sunday, 25 October 2026", "Saturday, 24 and
// Sunday, 25 October 2026", "Saturday, 31 October and Sunday, 1 November
// 2026", or across a year end each day in full.
func dayList(days []time.Time) string {
	full := "Monday, 2 January 2006"
	if len(days) == 1 {
		return days[0].Format(full)
	}
	a, b := days[0], days[len(days)-1]
	switch {
	case len(days) > 2 || a.Year() != b.Year():
		var parts []string
		for _, d := range days {
			parts = append(parts, d.Format(full))
		}
		return strings.Join(parts[:len(parts)-1], ", ") + " and " + parts[len(parts)-1]
	case a.Month() != b.Month():
		return a.Format("Monday, 2 January") + " and " + b.Format(full)
	default:
		return a.Format("Monday, 2") + " and " + b.Format(full)
	}
}

// shiftHours is one turn's hours in the rota's zone: "06:00–21:00", or, past
// midnight, "21:00–06:00 (next day)".
func shiftHours(start, end time.Time, zone *time.Location) string {
	s, e := start.In(zone), end.In(zone)
	h := s.Format("15:04") + "–" + e.Format("15:04")
	if s.YearDay() != e.YearDay() || s.Year() != e.Year() {
		h += " (next day)"
	}
	return h
}

// windowTitle names a window in title case: "Morning Rotation", "LK Weekend
// Rotation", "Americas Weekend Rotation". The name is the base shift's, so a
// window holding only an on-call person is still named for its shift.
func windowTitle(cfg Config, base string) string {
	name := cfg.Rotations[base]
	if name == "" {
		name = cfg.Rotations[base+"_OC"]
	}
	if strings.HasSuffix(strings.ToLower(name), "rotation") {
		return name
	}
	return name + " Rotation"
}

// greeting addresses an email by the names of the people it goes to: by full
// name when it goes to one person, "Dear all," when it goes to several.
func greeting(names []string) string {
	if len(names) == 1 && strings.TrimSpace(names[0]) != "" {
		return "Dear " + strings.TrimSpace(names[0]) + ","
	}
	return "Dear all,"
}

// windowText writes a shift window in the rota's zone: "06:00–09:00 IST", or,
// past midnight, "21:00 IST to 06:00 IST on Sunday".
func windowText(start, end time.Time, zone *time.Location) string {
	s, e := start.In(zone), end.In(zone)
	z := zoneName(s)
	if s.YearDay() == e.YearDay() && s.Year() == e.Year() {
		return fmt.Sprintf("%s–%s %s", s.Format("15:04"), e.Format("15:04"), z)
	}
	return fmt.Sprintf("%s %s to %s %s on %s", s.Format("15:04"), z, e.Format("15:04"), z, e.Format("Monday"))
}

// dateOf is a rota day (YYYY-MM-DD) in zone.
func dateOf(rotaDate string, zone *time.Location) time.Time {
	d, err := time.ParseInLocation(dateLayout, rotaDate, zone)
	if err != nil {
		return time.Time{}
	}
	return d
}

// zoneName is the name people use for t's zone. Go's tz database names
// Colombo's offset "+0530"; the office calls it IST.
func zoneName(t time.Time) string {
	switch t.Location().String() {
	case "Asia/Colombo", "Asia/Kolkata":
		return "IST"
	}
	return t.Format("MST")
}
