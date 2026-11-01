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
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/teamschedule"
)

// fakeStore is entity-service's reminder record and rota admin list.
type fakeStore struct {
	records  map[string]*teamschedule.ReminderRecord
	recorded []recordCall
	admins   []teamschedule.RotaAdmin
	getErr   error
}

type recordCall struct {
	period  string
	invites []teamschedule.ReminderInvite
	sentBy  string
}

func (f *fakeStore) GetReminder(_ context.Context, family, kind, period string) (*teamschedule.ReminderRecord, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	if family != "CRE" || kind != weekendKind {
		return nil, errors.New("unexpected record key " + family + "/" + kind)
	}
	return f.records[period], nil
}

func (f *fakeStore) RecordReminder(_ context.Context, _, _, period string, invites []teamschedule.ReminderInvite, sentBy string) error {
	f.recorded = append(f.recorded, recordCall{period, invites, sentBy})
	if f.records == nil {
		f.records = map[string]*teamschedule.ReminderRecord{}
	}
	f.records[period] = &teamschedule.ReminderRecord{Invites: invites, SentBy: sentBy, SendCount: len(f.recorded)}
	return nil
}

func (f *fakeStore) RotaAdmins(context.Context, string) ([]teamschedule.RotaAdmin, error) {
	return f.admins, nil
}

// weekendConfig is the weekend tasks' configuration as main.go builds it.
func weekendConfig(now, task string) Config {
	cfg := config(now, weekendRotations)
	cfg.Invites = true
	cfg.OrganizerEmail = "rota@example.com"
	cfg.Required = map[string]int{"CRE_WEEKEND": 3, "CRE_WEEKEND_NIGHT": 1}
	windows, err := ParseWindows("CRE_WEEKEND_NIGHT=08:00-18:00 America/Los_Angeles,CRE_WEEKEND_NIGHT_OC=08:00-18:00 America/Los_Angeles")
	if err != nil {
		panic(err)
	}
	cfg.Windows = windows
	cfg.Task = task
	cfg.ScheduleURL = "https://csm.example.com/team-schedule"
	return cfg
}

// fullWeekend is the weekend of 24-25 October, fully marked: three on LK
// Weekend and one on Americas Weekend (with an on-call person on Saturday),
// each day.
func fullWeekend() []teamschedule.Assignment {
	var out []teamschedule.Assignment
	for _, d := range []string{"2026-10-24", "2026-10-25"} {
		next := map[string]string{"2026-10-24": "2026-10-25", "2026-10-25": "2026-10-26"}[d]
		for _, p := range []struct{ name, email string }{{"Ann", "ann@example.com"}, {"Ben", "ben@example.com"}, {"Cid", "cid@example.com"}} {
			out = append(out, assignment(p.name, p.email, "vega", "CRE_WEEKEND", d, d+" 06:00", d+" 21:00"))
		}
		out = append(out, assignment("Ivy", "ivy@example.com", "americas", "CRE_WEEKEND_NIGHT", d, d+" 21:00", next+" 06:00"))
	}
	out = append(out, assignment("Jay", "jay@example.com", "americas", "CRE_WEEKEND_NIGHT_OC", "2026-10-24", "2026-10-24 21:00", "2026-10-25 06:00"))
	return out
}

// unfolded is an attachment's iCalendar text with its folded lines joined.
func unfolded(content []byte) string {
	return strings.ReplaceAll(string(content), "\r\n ", "")
}

// emailWith is the sent email whose subject holds s.
func emailWith(t *testing.T, sent []sentEmail, s string) sentEmail {
	t.Helper()
	for _, m := range sent {
		if strings.Contains(m.subject, s) {
			return m
		}
	}
	var subjects []string
	for _, m := range sent {
		subjects = append(subjects, m.subject)
	}
	t.Fatalf("no email with %q in its subject; sent:\n%s", s, strings.Join(subjects, "\n"))
	return sentEmail{}
}

// On Sunday evening a fully marked weekend gets one summary email to everybody
// on it and one invitation email per team per day, each to that team, all of
// them guests of its event; the send is recorded.
func TestWeekendNotice_AFullWeekendIsAnnouncedOnce(t *testing.T) {
	store := &fakeStore{}
	rota := &fakeRota{assignments: fullWeekend()}
	mail := &fakeEmail{}
	if err := WeekendNotice(rota, store, mail, weekendConfig("2026-10-18 20:00", "rota_weekend_notice"))(context.Background()); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if rota.from != "2026-10-24" || rota.to != "2026-10-25" {
		t.Fatalf("read %s..%s, want the coming weekend", rota.from, rota.to)
	}
	if len(mail.sent) != 5 {
		t.Fatalf("sent %d emails, want the summary and four invitations", len(mail.sent))
	}

	summary := emailWith(t, mail.sent, "Weekend Rotation – Saturday, 24 and Sunday, 25 October 2026")
	if summary.subject != "[CRE Operations] Weekend Rotation – Saturday, 24 and Sunday, 25 October 2026" ||
		strings.Join(summary.to, ",") != "ann@example.com,ben@example.com,cid@example.com,ivy@example.com,jay@example.com" ||
		len(summary.attachments) != 0 ||
		!strings.Contains(summary.body, "A calendar invitation for your team&#39;s rotation follows in a separate email") {
		t.Errorf("summary = %+v", summary)
	}
	for _, want := range []string{
		"<p>Hello team,</p>",
		"allocated for weekend support the coming weekend (2026-10-24 and 2026-10-25)",
		"<b>Saturday Team</b></p>\n<p style=\"margin-top:0;\">Ann (LK Weekend)<br/>Ben (LK Weekend)<br/>Cid (LK Weekend)<br/>Ivy (Americas Weekend)<br/>Jay (LK On-call)</p>",
		"<b>Sunday Team</b></p>\n<p style=\"margin-top:0;\">Ann (LK Weekend)<br/>Ben (LK Weekend)<br/>Cid (LK Weekend)<br/>Ivy (Americas Weekend)</p>",
		"<tr><td>LK Weekend<br/><span style=\"color:#666666; font-size:12px;\">3 members</span></td><td>06:00 AM &#8211; 09:00 PM IST</td><td>00:30 &#8211; 15:30</td><td>IST General support</td></tr>",
		"<tr><td>Americas Weekend<br/><span style=\"color:#666666; font-size:12px;\">1 member</span></td><td>08:00 AM &#8211; 06:00 PM PDT</td><td>15:00 &#8211; 01:00</td><td>PDT General support</td></tr>",
		"<tr><td>LK On-call<br/><span style=\"color:#666666; font-size:12px;\">Saturday 1, Sunday 0</span></td><td>08:00 AM &#8211; 06:00 PM PDT</td><td>15:00 &#8211; 01:00</td><td>On-call</td></tr>",
		"IST = Asia/Colombo (UTC+5:30) &#8226; PDT = America/Los_Angeles (UTC&#8722;7)<br/>Local times are fixed. The Americas Weekend Rotation UTC window moves by one hour on 01 Nov 2026, when daylight saving ends.",
		"<b>Handover</b>",
		"This is an auto-generated email.",
	} {
		if !strings.Contains(summary.body, want) {
			t.Errorf("summary does not hold %q:\n%s", want, summary.body)
		}
	}

	cases := []struct{ subject, uid, to, start string }{
		{"Calendar invitation: LK Weekend Rotation – Saturday, 24 October 2026 (06:00–21:00 IST)", "cre-rota-2026-10-24-cre_weekend@csm-team-schedule", "ann@example.com,ben@example.com,cid@example.com", "DTSTART:20261024T003000Z"},
		{"Calendar invitation: Americas Weekend Rotation – Saturday, 24 October 2026", "cre-rota-2026-10-24-cre_weekend_night@csm-team-schedule", "ivy@example.com,jay@example.com", "DTSTART:20261024T150000Z"},
		{"Calendar invitation: LK Weekend Rotation – Sunday, 25 October 2026", "cre-rota-2026-10-25-cre_weekend@csm-team-schedule", "ann@example.com,ben@example.com,cid@example.com", "DTSTART:20261025T003000Z"},
		{"Calendar invitation: Americas Weekend Rotation – Sunday, 25 October 2026", "cre-rota-2026-10-25-cre_weekend_night@csm-team-schedule", "ivy@example.com", "DTSTART:20261025T150000Z"},
	}
	for _, c := range cases {
		m := emailWith(t, mail.sent, c.subject)
		if !strings.HasPrefix(m.subject, "[CRE Operations] "+c.subject) || strings.Join(m.to, ",") != c.to || len(m.attachments) != 1 {
			t.Errorf("%q to %v with %d attachments", c.subject, m.to, len(m.attachments))
			continue
		}
		a := m.attachments[0]
		ics := unfolded(a.Content)
		if a.ContentType != "text/calendar; method=REQUEST; charset=UTF-8" || !strings.Contains(ics, "METHOD:REQUEST") ||
			!strings.Contains(ics, "UID:"+c.uid) || !strings.Contains(ics, c.start) || !strings.Contains(ics, "ORGANIZER;CN=CSM Team Schedule:mailto:rota@example.com") {
			t.Errorf("%q invite:\n%s", c.subject, ics)
		}
		for _, addr := range strings.Split(c.to, ",") {
			if !strings.Contains(ics, "RSVP=FALSE:mailto:"+addr) {
				t.Errorf("%q invite does not have %s as a guest:\n%s", c.subject, addr, ics)
			}
		}
	}

	if len(store.recorded) != 1 || store.recorded[0].period != "2026-10-24" || len(store.recorded[0].invites) != 4 || store.recorded[0].sentBy != "rota_weekend_notice" {
		t.Fatalf("recorded = %+v", store.recorded)
	}

	// The next evening finds the record and sends nothing.
	mail.sent = nil
	if err := WeekendNotice(rota, store, mail, weekendConfig("2026-10-19 20:00", "rota_weekend_notice"))(context.Background()); err != nil {
		t.Fatalf("second evening: %v", err)
	}
	if len(mail.sent) != 0 || len(store.recorded) != 1 {
		t.Fatalf("second evening sent %d emails and recorded %d times, want nothing new", len(mail.sent), len(store.recorded))
	}
}

// A weekend that is not fully marked is not announced: the rota admins are
// told what is missing, every evening, until it is; then it goes out.
func TestWeekendNotice_AnIncompleteWeekendAsksTheRotaAdmins(t *testing.T) {
	short := []teamschedule.Assignment{}
	for _, a := range fullWeekend() {
		if a.Engineer.Name == "Cid" && a.RotaDate == "2026-10-24" {
			continue // Saturday's LK Weekend has two of three
		}
		if a.ShiftCode == "CRE_WEEKEND_NIGHT" && a.RotaDate == "2026-10-25" {
			continue // Sunday's Americas Weekend has nobody
		}
		short = append(short, a)
	}
	store := &fakeStore{admins: []teamschedule.RotaAdmin{{Name: "Zoe Admin", Email: "zoe@example.com"}}}
	rota := &fakeRota{assignments: short}
	mail := &fakeEmail{}
	if err := WeekendNotice(rota, store, mail, weekendConfig("2026-10-18 20:00", "rota_weekend_notice"))(context.Background()); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if len(mail.sent) != 1 || len(store.recorded) != 0 {
		t.Fatalf("sent %d emails and recorded %d times, want only the admin email", len(mail.sent), len(store.recorded))
	}
	m := mail.sent[0]
	if strings.Join(m.to, ",") != "zoe@example.com" ||
		m.subject != "[CRE Operations] Action needed: the weekend rota for Saturday, 24 and Sunday, 25 October 2026 is not complete" ||
		!strings.Contains(m.body, "<p>Dear Zoe Admin,</p>") ||
		!strings.Contains(m.body, row("Saturday, 24 October", "LK Weekend Rotation", "2", "3")) ||
		!strings.Contains(m.body, row("Sunday, 25 October", "Americas Weekend Rotation", "0", "1")) ||
		strings.Contains(m.body, "arrange a swap") {
		t.Fatalf("admin email = %+v", m)
	}

	// Once marked, the next evening announces it.
	rota.assignments = fullWeekend()
	mail.sent = nil
	if err := WeekendNotice(rota, store, mail, weekendConfig("2026-10-19 20:00", "rota_weekend_notice"))(context.Background()); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if len(mail.sent) != 5 || len(store.recorded) != 1 {
		t.Fatalf("sent %d emails and recorded %d times, want the full announcement", len(mail.sent), len(store.recorded))
	}
}

// With no rota admin, the task's alert audience is asked.
func TestWeekendNotice_NoRotaAdminAsksTheAlertAudience(t *testing.T) {
	rota := &fakeRota{assignments: fullWeekend()[:2]}
	mail := &fakeEmail{}
	cfg := weekendConfig("2026-10-18 20:00", "rota_weekend_notice")
	cfg.AlertTo = []string{"ops@example.com"}
	if err := WeekendNotice(rota, &fakeStore{}, mail, cfg)(context.Background()); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if len(mail.sent) != 1 || strings.Join(mail.sent[0].to, ",") != "ops@example.com" || !strings.Contains(mail.sent[0].subject, "Action needed") {
		t.Fatalf("sent = %+v", mail.sent)
	}
}

// Friday re-sends the summary and every invitation as an update (same
// identity, a later sequence), and withdraws the event from somebody taken
// off a window since Sunday.
func TestWeekendReminder_UpdatesAndCancels(t *testing.T) {
	store := &fakeStore{records: map[string]*teamschedule.ReminderRecord{
		"2026-10-24": {Invites: []teamschedule.ReminderInvite{
			{UID: "cre-rota-2026-10-24-cre_weekend@csm-team-schedule", Sequence: 1, Attendees: []string{"ann@example.com", "ben@example.com", "cid@example.com"}},
			{UID: "cre-rota-2026-10-25-cre_weekend@csm-team-schedule", Sequence: 1, Attendees: []string{"ann@example.com", "ben@example.com", "dan@example.com"}},
		}},
	}}
	rota := &fakeRota{assignments: fullWeekend()} // Sunday's LK Weekend now has Cid instead of Dan
	mail := &fakeEmail{}
	if err := WeekendReminder(rota, store, mail, weekendConfig("2026-10-23 15:00", "rota_weekend_reminder"))(context.Background()); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if len(mail.sent) != 6 {
		t.Fatalf("sent %d emails, want the summary, four invitations and one cancellation", len(mail.sent))
	}
	summary := emailWith(t, mail.sent, "Weekend Rotation – Saturday, 24 and Sunday, 25 October 2026")
	if !strings.Contains(summary.body, "allocated for weekend support this weekend (2026-10-24 and 2026-10-25)") {
		t.Errorf("summary body:\n%s", summary.body)
	}
	update := emailWith(t, mail.sent, "Calendar invitation: LK Weekend Rotation – Sunday, 25 October 2026")
	if ics := unfolded(update.attachments[0].Content); strings.Contains(ics, "SEQUENCE:1\r\n") || !strings.Contains(ics, "UID:cre-rota-2026-10-25-cre_weekend@csm-team-schedule") {
		t.Errorf("the update does not carry a later sequence:\n%s", ics)
	}
	cancel := emailWith(t, mail.sent, "Cancelled:")
	ics := unfolded(cancel.attachments[0].Content)
	if strings.Join(cancel.to, ",") != "dan@example.com" || cancel.attachments[0].ContentType != "text/calendar; method=CANCEL; charset=UTF-8" ||
		!strings.Contains(ics, "METHOD:CANCEL") || !strings.Contains(ics, "STATUS:CANCELLED") ||
		!strings.Contains(ics, "UID:cre-rota-2026-10-25-cre_weekend@csm-team-schedule") || !strings.Contains(ics, "mailto:dan@example.com") {
		t.Fatalf("cancellation = %+v\n%s", cancel, ics)
	}
	if len(store.recorded) != 1 {
		t.Fatalf("recorded %d times", len(store.recorded))
	}
	for _, inv := range store.recorded[0].invites {
		if inv.UID == "cre-rota-2026-10-25-cre_weekend@csm-team-schedule" {
			sort.Strings(inv.Attendees)
			if strings.Join(inv.Attendees, ",") != "ann@example.com,ben@example.com,cid@example.com" || inv.Sequence <= 1 {
				t.Errorf("Sunday LK recorded as %+v", inv)
			}
		}
	}
}

// Friday goes out even with gaps, listing them, and tells the rota admins.
func TestWeekendReminder_GoesOutWithGapsAndTellsTheAdmins(t *testing.T) {
	var short []teamschedule.Assignment
	for _, a := range fullWeekend() {
		if !(a.ShiftCode == "CRE_WEEKEND_NIGHT" && a.RotaDate == "2026-10-25") {
			short = append(short, a)
		}
	}
	store := &fakeStore{admins: []teamschedule.RotaAdmin{{Name: "Zoe Admin", Email: "zoe@example.com"}}}
	mail := &fakeEmail{}
	if err := WeekendReminder(&fakeRota{assignments: short}, store, mail, weekendConfig("2026-10-23 15:00", "rota_weekend_reminder"))(context.Background()); err != nil {
		t.Fatalf("handler: %v", err)
	}
	admin := emailWith(t, mail.sent, "Action needed")
	if !strings.Contains(admin.body, "The weekend reminder has been sent with the rotations marked so far.") {
		t.Errorf("admin email:\n%s", admin.body)
	}
	summary := emailWith(t, mail.sent, "Weekend Rotation – Saturday, 24 and Sunday, 25 October 2026")
	if !strings.Contains(summary.body, "Not yet fully marked: Sunday Americas Weekend Rotation (0 of 1).") {
		t.Errorf("summary does not list the gap:\n%s", summary.body)
	}
	if len(mail.sent) != 5 || len(store.recorded) != 1 || len(store.recorded[0].invites) != 3 {
		t.Fatalf("sent %d emails, recorded %+v; want admin, summary and three invitations", len(mail.sent), store.recorded)
	}
}

// Only the shift itself counts towards a team's headcount, not its on-call
// variant.
func TestMissingSlots_OnCallDoesNotCount(t *testing.T) {
	sat := dateOf("2026-10-24", colombo)
	rostered := []teamschedule.Assignment{
		assignment("Jay", "jay@example.com", "americas", "CRE_WEEKEND_NIGHT_OC", "2026-10-24", "2026-10-24 21:00", "2026-10-25 06:00"),
	}
	gaps := missingSlots(Config{Required: map[string]int{"CRE_WEEKEND_NIGHT": 1}}, rostered, []time.Time{sat})
	if len(gaps) != 1 || gaps[0].have != 0 || gaps[0].need != 1 {
		t.Fatalf("gaps = %+v", gaps)
	}
}

// A summary that cannot be sent fails the run and records nothing, so the
// retry sends it.
func TestWeekendNotice_AFailedSummaryFailsAndRecordsNothing(t *testing.T) {
	store := &fakeStore{}
	mail := &fakeEmail{failOn: "Weekend Rotation –"}
	err := WeekendNotice(&fakeRota{assignments: fullWeekend()}, store, mail, weekendConfig("2026-10-18 20:00", "rota_weekend_notice"))(context.Background())
	if err == nil || len(store.recorded) != 0 {
		t.Fatalf("err = %v, recorded = %+v", err, store.recorded)
	}
}

// Weekday emails carry their window's invitation, everybody on the window a
// guest.
func TestWeekdayReminder_CarriesTheWindowsInvitation(t *testing.T) {
	rota := &fakeRota{assignments: []teamschedule.Assignment{
		assignment("Ann", "ann@example.com", "vega", "CRE_MORNING", "2026-10-19", "2026-10-19 06:00", "2026-10-19 09:00"),
		assignment("Ben", "ben@example.com", "nova", "CRE_MORNING_OC", "2026-10-19", "2026-10-19 06:00", "2026-10-19 09:00"),
	}}
	mail := &fakeEmail{}
	cfg := config("2026-10-16 15:00", weekdayRotations)
	cfg.Invites, cfg.OrganizerEmail = true, "rota@example.com"
	if err := WeekdayReminder(rota, mail, cfg)(context.Background()); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if len(mail.sent) != 1 || len(mail.sent[0].attachments) != 1 {
		t.Fatalf("sent = %+v", mail.sent)
	}
	ics := unfolded(mail.sent[0].attachments[0].Content)
	for _, want := range []string{"METHOD:REQUEST", "UID:cre-rota-2026-10-19-cre_morning@csm-team-schedule", "SUMMARY:CRE Morning Rotation",
		"mailto:ann@example.com", "mailto:ben@example.com", "DTSTART:20261019T003000Z", "DTEND:20261019T033000Z"} {
		if !strings.Contains(ics, want) {
			t.Errorf("invite lacks %q:\n%s", want, ics)
		}
	}
	if !strings.Contains(mail.sent[0].body, "The calendar invitation is attached; everybody on this rotation is a guest") {
		t.Errorf("body:\n%s", mail.sent[0].body)
	}
}

// An event's identity holds nothing but letters, digits, '-' and '_'.
func TestInviteUID_IsSafeForAnyFamily(t *testing.T) {
	if got := inviteUID("CRE TEST", "2026-10-24", "CRE_WEEKEND"); got != "cre-test-rota-2026-10-24-cre_weekend@csm-team-schedule" {
		t.Fatalf("uid = %q", got)
	}
}

// Nothing from the rota or the configuration can add a line to an invitation:
// a line break or other control character in a name or in the notes is
// dropped or escaped, an address that is not a plain address is left out, and
// so is a URL that is not http(s).
func TestICS_NothingCanStartALineOfItsOwn(t *testing.T) {
	at := time.Date(2026, 10, 24, 0, 30, 0, 0, time.UTC)
	inv := Invite{
		UID: "cre-rota-2026-10-24-cre_weekend@csm-team-schedule", Summary: "CRE\r\nATTENDEE:mailto:evil@example.com",
		Description: "line one\rline two\x00\x1b", Start: at, End: at.Add(time.Hour), Stamp: at,
		OrganizerName: "CSM\nTeam", OrganizerEmail: "rota@example.com",
		Attendees: []Attendee{
			{Name: "Ann\r\nATTENDEE:mailto:evil@example.com", Email: "ann@example.com"},
			{Name: "Ben", Email: "ben@example.com\r\nATTENDEE:mailto:evil@example.com"},
			{Name: "Cid", Email: "Cid <cid@example.com>"},
		},
		URL: "javascript:alert(1)",
	}
	ics := unfolded(inv.ICS())
	lines := strings.Split(strings.TrimSuffix(ics, "\r\n"), "\r\n")
	attendees := 0
	for _, l := range lines {
		if strings.ContainsAny(l, "\r\n\x00\x1b") {
			t.Errorf("control character in %q", l)
		}
		if strings.HasPrefix(l, "ATTENDEE") {
			attendees++
		}
		if strings.HasPrefix(l, "URL:") {
			t.Errorf("unsafe URL kept: %q", l)
		}
	}
	for _, want := range []string{`SUMMARY:CRE\nATTENDEE:mailto:evil@example.com`, `DESCRIPTION:line one\nline two`,
		"ORGANIZER;CN=CSMTeam:mailto:rota@example.com", `ATTENDEE;CN="AnnATTENDEE:mailto:evil@example.com";`} {
		if !strings.Contains(ics, want) {
			t.Errorf("invite lacks %q:\n%s", want, ics)
		}
	}
	if attendees != 1 {
		t.Errorf("%d attendees, want only Ann:\n%s", attendees, ics)
	}

	inv.URL, inv.OrganizerEmail = "https://portal.example.com/team-schedule", "not an address"
	ics = unfolded(inv.ICS())
	if !strings.Contains(ics, "URL:https://portal.example.com/team-schedule\r\n") || strings.Contains(ics, "ORGANIZER") {
		t.Errorf("invite:\n%s", ics)
	}
}

// Only a plain address and an http(s) URL pass the startup checks.
func TestValidInviteAddressAndScheduleURL(t *testing.T) {
	for s, want := range map[string]bool{"rota@example.com": true, "": false, "Rota <rota@example.com>": false,
		"rota@example.com\r\nX": false, "a b@example.com": false, "rota": false} {
		if got := ValidInviteAddress(s); got != want {
			t.Errorf("ValidInviteAddress(%q) = %v", s, got)
		}
	}
	// The organizer is the sender's address, with or without its display name.
	for s, want := range map[string]string{"rota@example.com": "rota@example.com", " rota@example.com ": "rota@example.com",
		"WSO2 Customer Service <no-reply@example.com>": "no-reply@example.com", `"Rota, CSM" <rota@example.com>`: "rota@example.com",
		"": "", "rota": "", "Rota <rota>": "", "Rota <rota@example.com>\r\nX: y": "", "Rota\n <rota@example.com>": "",
		"a <b@example.com>, c <d@example.com>": "",
		// A control character at either end is refused too, not trimmed away.
		"Rota <rota@example.com>\r\n": "", "rota@example.com\n": "", "\trota@example.com": "", "\x00Rota <rota@example.com>": ""} {
		got, ok := InviteOrganizer(s)
		if got != want || ok != (want != "") {
			t.Errorf("InviteOrganizer(%q) = %q, %v; want %q", s, got, ok, want)
		}
	}
	// An address goes into an invitation as a mailto URI, a character a URI
	// cannot hold percent-encoded; an everyday address is unchanged.
	for s, want := range map[string]string{"rota@example.com": "mailto:rota@example.com",
		"first.last+tag@example.com": "mailto:first.last+tag@example.com", "ops?tag@example.com": "mailto:ops%3Ftag@example.com",
		"a#b@example.com": "mailto:a%23b@example.com", "a%b@example.com": "mailto:a%25b@example.com", "a/b=c@example.com": "mailto:a%2Fb%3Dc@example.com"} {
		if got := mailtoURI(s); got != want {
			t.Errorf("mailtoURI(%q) = %q; want %q", s, got, want)
		}
	}
	ics := string(Invite{UID: "u@x", OrganizerName: "CSM", OrganizerEmail: "ops?tag@example.com",
		Attendees: []Attendee{{Name: "Ann", Email: "ann#1@example.com"}}}.ICS())
	if !strings.Contains(ics, "ORGANIZER;CN=CSM:mailto:ops%3Ftag@example.com\r\n") || !strings.Contains(unfolded([]byte(ics)), ":mailto:ann%231@example.com\r\n") {
		t.Errorf("invite addresses are not encoded:\n%s", ics)
	}
	for s, want := range map[string]bool{"https://portal.example.com/x": true, "http://localhost:3000": true, "": false,
		"javascript:alert(1)": false, "https://": false, "https://a.example.com/x\ny": false, "/team-schedule": false} {
		if got := ValidScheduleURL(s); got != want {
			t.Errorf("ValidScheduleURL(%q) = %v", s, got)
		}
	}
}

// A weekend after the US clock change has no daylight saving note, and the
// Americas Weekend is then 08:00-18:00 PST, an hour later in UTC.
func TestWeekendNotice_AmericasHoursFollowLosAngeles(t *testing.T) {
	rota := &fakeRota{}
	for _, d := range []string{"2026-11-07", "2026-11-08"} {
		for _, p := range []string{"ann", "ben", "cid"} {
			rota.assignments = append(rota.assignments, assignment(p, p+"@example.com", "vega", "CRE_WEEKEND", d, d+" 06:00", d+" 21:00"))
		}
		// The Team Schedule still says 21:00-06:00 IST; the reminder does not.
		rota.assignments = append(rota.assignments, assignment("Ivy", "ivy@example.com", "americas", "CRE_WEEKEND_NIGHT", d, d+" 21:00", d+" 23:59"))
	}
	mail := &fakeEmail{}
	if err := WeekendNotice(rota, &fakeStore{}, mail, weekendConfig("2026-11-01 20:00", "rota_weekend_notice"))(context.Background()); err != nil {
		t.Fatal(err)
	}
	summary := emailWith(t, mail.sent, "Weekend Rotation – Saturday, 7 and Sunday, 8 November 2026")
	if !strings.Contains(summary.body, "<td>08:00 AM &#8211; 06:00 PM PST</td><td>16:00 &#8211; 02:00</td><td>PST General support</td>") ||
		strings.Contains(summary.body, "daylight saving") {
		t.Errorf("summary:\n%s", summary.body)
	}
	invite := emailWith(t, mail.sent, "Calendar invitation: Americas Weekend Rotation – Saturday, 7 November 2026 (08:00–18:00 PST)")
	ics := unfolded(invite.attachments[0].Content)
	if !strings.Contains(ics, "DTSTART:20261107T160000Z") || !strings.Contains(ics, "DTEND:20261108T020000Z") {
		t.Errorf("invite:\n%s", ics)
	}
}

func TestParseWindows(t *testing.T) {
	w, err := ParseWindows(" CRE_WEEKEND_NIGHT=08:00-18:00 America/Los_Angeles , X=22:00-06:00 Asia/Colombo")
	if err != nil {
		t.Fatal(err)
	}
	if got := w["CRE_WEEKEND_NIGHT"]; got.Start != 480 || got.End != 1080 || got.Zone.String() != "America/Los_Angeles" {
		t.Errorf("CRE_WEEKEND_NIGHT = %+v", got)
	}
	s, e, _ := w["X"].on("2026-10-24")
	if s.Format("2006-01-02 15:04") != "2026-10-24 22:00" || e.Format("2006-01-02 15:04") != "2026-10-25 06:00" {
		t.Errorf("overnight window = %v to %v", s, e)
	}
	for _, bad := range []string{"CRE=08:00-18:00", "CRE=08:00 America/Los_Angeles", "CRE=08:00-18:00 Mars/Base", "CRE=8am-6pm America/Los_Angeles", "CRE=08:00-08:00 UTC"} {
		if _, err := ParseWindows(bad); err == nil {
			t.Errorf("ParseWindows(%q) accepted", bad)
		}
	}
	if w, err := ParseWindows(""); err != nil || len(w) != 0 {
		t.Errorf("empty = %v, %v", w, err)
	}
}

// On Friday with gaps the rota admins are told only after the summary has
// gone: when it fails, the run fails and nobody is told yet, so each retry
// does not email the admins again, and their email never says a reminder
// went out that did not.
func TestWeekendReminder_AdminsAreToldOnlyAfterTheSummary(t *testing.T) {
	short := fullWeekend()[1:] // Saturday's LK Weekend is one short
	store := &fakeStore{admins: []teamschedule.RotaAdmin{{Name: "Zoe Admin", Email: "zoe@example.com"}}}
	cfg := weekendConfig("2026-10-23 15:00", "rota_weekend_reminder")

	mail := &fakeEmail{failOn: "Weekend Rotation –"}
	if err := WeekendReminder(&fakeRota{assignments: short}, store, mail, cfg)(context.Background()); err == nil {
		t.Fatal("a failed summary passed")
	}
	if len(mail.sent) != 0 {
		t.Fatalf("sent %d emails before the summary went out: %+v", len(mail.sent), mail.sent)
	}

	mail = &fakeEmail{}
	if err := WeekendReminder(&fakeRota{assignments: short}, store, mail, cfg)(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(mail.sent) < 2 || !strings.Contains(mail.sent[0].subject, "Weekend Rotation –") || !strings.Contains(mail.sent[1].subject, "Action needed") {
		var subjects []string
		for _, m := range mail.sent {
			subjects = append(subjects, m.subject)
		}
		t.Fatalf("order: %v", subjects)
	}
}
