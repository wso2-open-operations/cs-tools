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
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/teamschedule"
)

var colombo = mustZone("Asia/Colombo")

func mustZone(name string) *time.Location {
	z, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return z
}

// day is midnight on a calendar day in Colombo.
func day(s string) time.Time {
	t, err := time.ParseInLocation(dateLayout, s, colombo)
	if err != nil {
		panic(err)
	}
	return t
}

// at is a wall-clock time in Colombo, "2006-01-02 15:04".
func at(s string) time.Time {
	t, err := time.ParseInLocation("2006-01-02 15:04", s, colombo)
	if err != nil {
		panic(err)
	}
	return t
}

func TestNextWorkingDay(t *testing.T) {
	for _, c := range []struct{ today, want string }{
		{"2026-10-12", "2026-10-13"}, // Monday -> Tuesday
		{"2026-10-15", "2026-10-16"}, // Thursday -> Friday
		{"2026-10-16", "2026-10-19"}, // Friday -> Monday, not Saturday
		{"2026-10-17", "2026-10-19"}, // Saturday -> Monday
		{"2026-10-18", "2026-10-19"}, // Sunday -> Monday
		{"2026-10-30", "2026-11-02"}, // Friday at the end of a month
	} {
		if got := NextWorkingDay(day(c.today)).Format(dateLayout); got != c.want {
			t.Errorf("NextWorkingDay(%s) = %s, want %s", c.today, got, c.want)
		}
	}
}

func TestComingWeekend(t *testing.T) {
	for _, c := range []struct{ today, sat string }{
		{"2026-10-12", "2026-10-17"}, // Monday: the heads-up, five days ahead
		{"2026-10-16", "2026-10-17"}, // Friday: the reminder, tomorrow
		{"2026-10-14", "2026-10-17"}, // Wednesday
		{"2026-10-17", "2026-10-24"}, // Saturday: this weekend is under way
		{"2026-10-18", "2026-10-24"}, // Sunday
		{"2026-12-28", "2027-01-02"}, // across a year end
	} {
		sat, sun := ComingWeekend(day(c.today))
		if sat.Format(dateLayout) != c.sat || sun.Format(dateLayout) != day(c.sat).AddDate(0, 0, 1).Format(dateLayout) {
			t.Errorf("ComingWeekend(%s) = %s, %s; want %s and the day after", c.today, sat.Format(dateLayout), sun.Format(dateLayout), c.sat)
		}
	}
}

// fakeRota returns a fixed rota and records what it was asked for.
type fakeRota struct {
	assignments []teamschedule.Assignment
	err         error
	reads       int
	from, to    string
	family      string
}

func (f *fakeRota) Catalogue(context.Context) (teamschedule.Catalogue, error) {
	f.reads++
	return teamschedule.Catalogue{
		Shifts: []teamschedule.Shift{
			{Code: "CRE_MORNING", Label: "Morning 6-9am", AuthoringTimeZone: "Asia/Colombo", SortOrder: 10},
			{Code: "CRE_MORNING_OC", Label: "Morning 6-9am on-call", AuthoringTimeZone: "Asia/Colombo", SortOrder: 20, IsOnCall: true},
			{Code: "CRE_REGULAR", Label: "Regular hours", AuthoringTimeZone: "Asia/Colombo", SortOrder: 30},
			{Code: "CRE_EVENING", Label: "Evening 6-9pm", AuthoringTimeZone: "Asia/Colombo", SortOrder: 40},
			{Code: "CRE_WEEKEND", Label: "Weekend rotation", AuthoringTimeZone: "Asia/Colombo", SortOrder: 60},
			{Code: "CRE_WEEKEND_NIGHT", Label: "Americas weekend", AuthoringTimeZone: "Asia/Colombo", SortOrder: 70},
			{Code: "CRE_WEEKEND_NIGHT_OC", Label: "Americas weekend on-call", AuthoringTimeZone: "Asia/Colombo", SortOrder: 80, IsOnCall: true},
		},
		Teams: []teamschedule.Team{{Key: "vega", Name: "Vega"}, {Key: "nova", Name: "Nova"}, {Key: "orion", Name: "Orion"}, {Key: "americas", Name: "Americas"}},
	}, nil
}

func (f *fakeRota) WorkingAssignments(_ context.Context, from, to, family string) ([]teamschedule.Assignment, error) {
	f.reads++
	f.from, f.to, f.family = from, to, family
	return f.assignments, f.err
}

type sentEmail struct {
	to, cc        []string
	subject, body string
}

type fakeEmail struct {
	sent   []sentEmail
	failOn string // a subject substring whose send fails
}

func (f *fakeEmail) SendEmail(_ context.Context, to, cc []string, subject, body string) error {
	if f.failOn != "" && strings.Contains(subject, f.failOn) {
		return errors.New("mail service down")
	}
	f.sent = append(f.sent, sentEmail{to: to, cc: cc, subject: subject, body: body})
	return nil
}

func assignment(name, email, team, code, rotaDate, start, end string) teamschedule.Assignment {
	return teamschedule.Assignment{
		Engineer:  teamschedule.Engineer{Name: name, Email: email},
		TeamKey:   team,
		ShiftCode: code,
		RotaDate:  rotaDate,
		StartsAt:  at(start).UTC(),
		EndsAt:    at(end).UTC(),
		IsOnCall:  strings.HasSuffix(code, "_OC"),
	}
}

var weekdayRotations = map[string]string{"CRE_MORNING": "Morning", "CRE_MORNING_OC": "On-call", "CRE_EVENING": "Evening"}
var weekendRotations = map[string]string{"CRE_WEEKEND": "LK Weekend", "CRE_WEEKEND_NIGHT": "America Weekend", "CRE_WEEKEND_NIGHT_OC": "On-call"}

// config is a task's configuration as cmd/server/main.go builds it.
func config(now string, rotations map[string]string) Config {
	return Config{
		Family:        "CRE",
		Rotations:     rotations,
		EmailsEnabled: true,
		Zone:          colombo,
		Now:           func() time.Time { return at(now) },
	}
}

// row is one rota table row as it appears in the body.
func row(cells ...string) string {
	return "<tr><td>" + strings.Join(cells, "</td><td>") + "</td></tr>"
}

// Friday 15:00 sends Monday's two windows: Morning with its on-call person in
// one email, Evening in another, each to the people on it only.
func TestWeekdayReminder_OneEmailPerWindow(t *testing.T) {
	rota := &fakeRota{assignments: []teamschedule.Assignment{
		assignment("Dee", "dee@example.com", "nova", "CRE_EVENING", "2026-10-19", "2026-10-19 18:00", "2026-10-19 21:00"),
		assignment("Ben", "ben@example.com", "nova", "CRE_MORNING_OC", "2026-10-19", "2026-10-19 06:00", "2026-10-19 09:00"),
		assignment("Cid", "cid@example.com", "vega", "CRE_REGULAR", "2026-10-19", "2026-10-19 09:00", "2026-10-19 18:00"),
		assignment("Ann", "ann@example.com", "vega", "CRE_MORNING", "2026-10-19", "2026-10-19 06:00", "2026-10-19 09:00"),
	}}
	mail := &fakeEmail{}
	if err := WeekdayReminder(rota, mail, config("2026-10-16 15:00", weekdayRotations))(context.Background()); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if rota.from != "2026-10-19" || rota.to != "2026-10-19" || rota.family != "CRE" {
		t.Fatalf("read %s..%s family %q, want Monday 2026-10-19 for CRE", rota.from, rota.to, rota.family)
	}
	if len(mail.sent) != 2 {
		t.Fatalf("sent %d emails, want Morning and Evening: %+v", len(mail.sent), mail.sent)
	}

	morning := mail.sent[0]
	if strings.Join(morning.to, ",") != "ann@example.com,ben@example.com" || len(morning.cc) != 0 {
		t.Errorf("Morning to %v cc %v, want Ann and Ben only", morning.to, morning.cc)
	}
	if want := "[CRE Rota] Morning Rotation – Monday, 19 October 2026 (06:00–09:00 IST)"; morning.subject != want {
		t.Errorf("Morning subject = %q, want %q", morning.subject, want)
	}
	for _, s := range []string{
		"<p>Dear all,</p>",
		"<p>This is an advance notice that you are rostered for the Morning rotation on Monday, 19 October 2026, 06:00&#8211;09:00 IST.</p>",
		`<tr><th align="left">Name</th><th align="left">Rotation</th></tr>`,
		row("Ann", "Morning") + "\n" + row("Ben", "On-call"),
	} {
		if !strings.Contains(morning.body, s) {
			t.Errorf("Morning body lacks %q\n%s", s, morning.body)
		}
	}
	if strings.Contains(morning.body, "Dee") || strings.Contains(morning.body, "Cid") {
		t.Error("the Morning email names somebody not on the Morning window")
	}
	evening := mail.sent[1]
	if strings.Join(evening.to, ",") != "dee@example.com" ||
		evening.subject != "[CRE Rota] Evening Rotation – Monday, 19 October 2026 (18:00–21:00 IST)" ||
		!strings.Contains(evening.body, "<p>Dear Dee,</p>") {
		t.Errorf("Evening = %+v", evening)
	}
}

// From Monday to Thursday the window is tomorrow, and the email says so.
func TestWeekdayReminder_MidWeekSaysTomorrow(t *testing.T) {
	rota := &fakeRota{assignments: []teamschedule.Assignment{
		assignment("Ann", "ann@example.com", "vega", "CRE_MORNING", "2026-10-13", "2026-10-13 06:00", "2026-10-13 09:00"),
	}}
	mail := &fakeEmail{}
	if err := WeekdayReminder(rota, mail, config("2026-10-12 15:00", weekdayRotations))(context.Background()); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if rota.from != "2026-10-13" || len(mail.sent) != 1 ||
		!strings.Contains(mail.sent[0].body, "This is a reminder that you are rostered for the Morning rotation tomorrow, Tuesday, 13 October 2026, 06:00&#8211;09:00 IST.") {
		t.Fatalf("read %s, sent %+v", rota.from, mail.sent)
	}
}

// "Today" is the day in the rota's own zone, not the server's: Thursday 20:00
// UTC is already Friday in Colombo, so the next working day is Monday.
func TestWeekdayReminder_CountsDaysInTheRotaZone(t *testing.T) {
	rota := &fakeRota{}
	cfg := config("2026-10-16 15:00", weekdayRotations)
	cfg.Now = func() time.Time { return time.Date(2026, 10, 15, 20, 0, 0, 0, time.UTC) } // Fri 01:30 IST
	if err := WeekdayReminder(rota, &fakeEmail{}, cfg)(context.Background()); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if rota.from != "2026-10-19" {
		t.Fatalf("read %s, want Monday: it is already Friday in Colombo", rota.from)
	}
}

// The weekend is one email per window per day -- LK and America (with its
// on-call) on Saturday and on Sunday -- each listing the whole weekend's rota.
// The America window runs past midnight.
func TestWeekendNotice_OneEmailPerWindowPerDay(t *testing.T) {
	roster := []teamschedule.Assignment{
		assignment("Kim", "kim@example.com", "vega", "CRE_WEEKEND", "2026-10-18", "2026-10-18 06:00", "2026-10-18 21:00"),
		assignment("Jay", "jay@example.com", "americas", "CRE_WEEKEND_NIGHT_OC", "2026-10-17", "2026-10-17 21:00", "2026-10-18 06:00"),
		assignment("Ivy", "ivy@example.com", "americas", "CRE_WEEKEND_NIGHT", "2026-10-17", "2026-10-17 21:00", "2026-10-18 06:00"),
		assignment("Hal", "hal@example.com", "orion", "CRE_WEEKEND", "2026-10-17", "2026-10-17 06:00", "2026-10-17 21:00"),
		assignment("Fay", "fay@example.com", "vega", "CRE_WEEKEND", "2026-10-17", "2026-10-17 06:00", "2026-10-17 21:00"),
	}
	for _, c := range []struct{ name, now, notice string }{
		{"Monday notice", "2026-10-12 15:00", "This is an advance notice that you are rostered for the LK Weekend rotation on Saturday, 17 October 2026, 06:00&#8211;21:00 IST."},
		{"Friday reminder", "2026-10-16 15:00", "This is a reminder that you are rostered for the LK Weekend rotation tomorrow, Saturday, 17 October 2026, 06:00&#8211;21:00 IST."},
	} {
		t.Run(c.name, func(t *testing.T) {
			rota := &fakeRota{assignments: roster}
			mail := &fakeEmail{}
			if err := WeekendNotice(rota, mail, config(c.now, weekendRotations))(context.Background()); err != nil {
				t.Fatalf("handler: %v", err)
			}
			if rota.from != "2026-10-17" || rota.to != "2026-10-18" {
				t.Fatalf("read %s..%s, want Sat 17 to Sun 18 Oct", rota.from, rota.to)
			}
			if len(mail.sent) != 3 {
				t.Fatalf("sent %d emails, want Sat LK, Sat America, Sun LK: %+v", len(mail.sent), mail.sent)
			}
			satLK, satAmerica, sunLK := mail.sent[0], mail.sent[1], mail.sent[2]

			if strings.Join(satLK.to, ",") != "fay@example.com,hal@example.com" ||
				satLK.subject != "[CRE Rota] LK Weekend Rotation – Saturday, 17 October 2026 (06:00–21:00 IST)" ||
				!strings.Contains(satLK.body, "<p>"+c.notice+"</p>") {
				t.Errorf("Saturday LK = %+v", satLK)
			}
			// Every weekend email lists the whole weekend.
			whole := row("Fay", "Saturday", "LK Weekend") + "\n" + row("Hal", "Saturday", "LK Weekend") + "\n" +
				row("Ivy", "Saturday", "America Weekend") + "\n" + row("Jay", "Saturday", "On-call") + "\n" +
				row("Kim", "Sunday", "LK Weekend")
			for _, m := range mail.sent {
				if !strings.Contains(m.body, whole) {
					t.Errorf("%q does not list the whole weekend:\n%s", m.subject, m.body)
				}
			}

			if strings.Join(satAmerica.to, ",") != "ivy@example.com,jay@example.com" ||
				satAmerica.subject != "[CRE Rota] America Weekend Rotation – Saturday, 17 October 2026 (21:00 IST to 06:00 IST on Sunday)" {
				t.Errorf("Saturday America = %q to %v", satAmerica.subject, satAmerica.to)
			}
			if strings.Join(sunLK.to, ",") != "kim@example.com" || !strings.Contains(sunLK.body, "<p>Dear Kim,</p>") {
				t.Errorf("Sunday LK = %+v", sunLK)
			}
		})
	}
}

// A weekend across a year end names the right dates.
func TestWeekendNotice_AcrossAYearEnd(t *testing.T) {
	rota := &fakeRota{assignments: []teamschedule.Assignment{
		assignment("Fay", "fay@example.com", "vega", "CRE_WEEKEND", "2034-01-01", "2034-01-01 06:00", "2034-01-01 21:00"),
	}}
	mail := &fakeEmail{}
	if err := WeekendNotice(rota, mail, config("2033-12-26 15:00", weekendRotations))(context.Background()); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if rota.from != "2033-12-31" || rota.to != "2034-01-01" || len(mail.sent) != 1 ||
		mail.sent[0].subject != "[CRE Rota] LK Weekend Rotation – Sunday, 1 January 2034 (06:00–21:00 IST)" {
		t.Fatalf("read %s..%s, sent %+v", rota.from, rota.to, mail.sent)
	}
}

// A copied address is copied once, and never as well as being a recipient.
func TestCopyIsCopiedOnce(t *testing.T) {
	rota := &fakeRota{assignments: []teamschedule.Assignment{
		assignment("Ann", "ann@example.com", "vega", "CRE_MORNING", "2026-10-13", "2026-10-13 06:00", "2026-10-13 09:00"),
	}}
	mail := &fakeEmail{}
	cfg := config("2026-10-12 15:00", weekdayRotations)
	cfg.Copy = []string{"lead@example.com", "Ann@Example.com", "lead@example.com"}
	if err := WeekdayReminder(rota, mail, cfg)(context.Background()); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if strings.Join(mail.sent[0].cc, ",") != "lead@example.com" {
		t.Fatalf("cc = %v", mail.sent[0].cc)
	}
}

// Redirected, every email goes only to the test inbox and says who it was for.
func TestRedirectSendsEverythingToTheTestInbox(t *testing.T) {
	rota := &fakeRota{assignments: []teamschedule.Assignment{
		assignment("Ann", "ann@example.com", "vega", "CRE_MORNING", "2026-10-13", "2026-10-13 06:00", "2026-10-13 09:00"),
	}}
	mail := &fakeEmail{}
	cfg := config("2026-10-12 15:00", weekdayRotations)
	cfg.Copy = []string{"lead@example.com"}
	cfg.RedirectTo = []string{"rota-test@example.com"}
	if err := WeekdayReminder(rota, mail, cfg)(context.Background()); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if len(mail.sent) != 1 || strings.Join(mail.sent[0].to, ",") != "rota-test@example.com" || len(mail.sent[0].cc) != 0 {
		t.Fatalf("sent = %+v", mail.sent)
	}
	if !strings.Contains(mail.sent[0].body, "would have been sent to: ann@example.com, lead@example.com") {
		t.Error("the redirected copy does not say who it was for")
	}
}

// With emails switched off nothing is read and nothing is sent.
func TestEmailsDisabledDoesNothing(t *testing.T) {
	rota := &fakeRota{}
	mail := &fakeEmail{}
	cfg := config("2026-10-12 15:00", weekdayRotations)
	cfg.EmailsEnabled = false
	if err := WeekdayReminder(rota, mail, cfg)(context.Background()); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if rota.reads != 0 || len(mail.sent) != 0 {
		t.Fatalf("reads %d, sent %d; want none", rota.reads, len(mail.sent))
	}
}

// An empty rota is a quiet success, and so is a window whose only person has
// no address.
func TestNobodyToEmailIsASuccess(t *testing.T) {
	for name, roster := range map[string][]teamschedule.Assignment{
		"empty rota":          nil,
		"no address":          {assignment("Ann", " ", "vega", "CRE_MORNING", "2026-10-13", "2026-10-13 06:00", "2026-10-13 09:00")},
		"only standing hours": {assignment("Cid", "cid@example.com", "vega", "CRE_REGULAR", "2026-10-13", "2026-10-13 09:00", "2026-10-13 18:00")},
	} {
		t.Run(name, func(t *testing.T) {
			mail := &fakeEmail{}
			if err := WeekdayReminder(&fakeRota{assignments: roster}, mail, config("2026-10-12 15:00", weekdayRotations))(context.Background()); err != nil {
				t.Fatalf("handler: %v", err)
			}
			if len(mail.sent) != 0 {
				t.Fatalf("sent %+v", mail.sent)
			}
		})
	}
}

// Somebody with no address is listed in the email but is not a recipient.
func TestSomebodyWithNoAddressIsListedButNotEmailed(t *testing.T) {
	rota := &fakeRota{assignments: []teamschedule.Assignment{
		assignment("Ann", "ann@example.com", "vega", "CRE_MORNING", "2026-10-13", "2026-10-13 06:00", "2026-10-13 09:00"),
		assignment("Bob", "", "nova", "CRE_MORNING_OC", "2026-10-13", "2026-10-13 06:00", "2026-10-13 09:00"),
	}}
	mail := &fakeEmail{}
	if err := WeekdayReminder(rota, mail, config("2026-10-12 15:00", weekdayRotations))(context.Background()); err != nil {
		t.Fatalf("handler: %v", err)
	}
	m := mail.sent[0]
	if strings.Join(m.to, ",") != "ann@example.com" || !strings.Contains(m.body, row("Bob", "On-call")) ||
		!strings.Contains(m.body, "<p>Dear Ann,</p>") {
		t.Fatalf("sent %+v", m)
	}
}

// One failed send does not stop the others. As the others went out, the run
// succeeds (a retry would email them all again) and the alert audience is told
// which window failed and whom it was for.
func TestAPartlyFailedRunSucceedsAndAlerts(t *testing.T) {
	rota := &fakeRota{assignments: []teamschedule.Assignment{
		assignment("Ann", "ann@example.com", "vega", "CRE_MORNING", "2026-10-13", "2026-10-13 06:00", "2026-10-13 09:00"),
		assignment("Dee", "dee@example.com", "nova", "CRE_EVENING", "2026-10-13", "2026-10-13 18:00", "2026-10-13 21:00"),
	}}
	mail := &fakeEmail{failOn: "Morning"}
	cfg := config("2026-10-12 15:00", weekdayRotations)
	cfg.Task, cfg.AlertTo, cfg.AlertCc = "rota_weekday_reminder", []string{"ops@example.com"}, []string{"lead@example.com"}
	if err := WeekdayReminder(rota, mail, cfg)(context.Background()); err != nil {
		t.Fatalf("err = %v, want success: the Evening email went out", err)
	}
	if len(mail.sent) != 2 || !strings.Contains(mail.sent[0].subject, "Evening") {
		t.Fatalf("sent = %+v, want the Evening email, then the alert", mail.sent)
	}
	alert := mail.sent[1]
	if strings.Join(alert.to, ",") != "ops@example.com" || strings.Join(alert.cc, ",") != "lead@example.com" ||
		alert.subject != "[csm-scheduled-tasks] PARTLY FAILED: rota_weekday_reminder - 1 rota reminder(s) not sent" {
		t.Fatalf("alert = %+v", alert)
	}
	for _, s := range []string{"<td>Morning Rotation</td><td>Tuesday, 13 October 2026</td><td>Ann</td>", "mail service down", "not retried"} {
		if !strings.Contains(alert.body, s) {
			t.Errorf("alert body lacks %q\n%s", s, alert.body)
		}
	}
}

// When nothing went out the run fails, so the engine retries and alerts: a
// retry then tells nobody twice.
func TestAFullyFailedRunFails(t *testing.T) {
	rota := &fakeRota{assignments: []teamschedule.Assignment{
		assignment("Ann", "ann@example.com", "vega", "CRE_MORNING", "2026-10-13", "2026-10-13 06:00", "2026-10-13 09:00"),
	}}
	mail := &fakeEmail{failOn: "Rota"}
	cfg := config("2026-10-12 15:00", weekdayRotations)
	cfg.AlertTo = []string{"ops@example.com"}
	err := WeekdayReminder(rota, mail, cfg)(context.Background())
	if err == nil || !strings.Contains(err.Error(), "1 of 1") || !strings.Contains(err.Error(), "CRE_MORNING on 2026-10-13") {
		t.Fatalf("err = %v, want the run failed, naming the Morning window", err)
	}
	if len(mail.sent) != 0 {
		t.Fatalf("sent = %+v, want nothing: the engine's own alert covers a failed run", mail.sent)
	}
}

// A retry counts its days from the run it retries (Anchor), not from the
// clock. Thursday's run, retried after midnight, still announces Friday, and
// says "today" for a window that has not begun.
func TestAWeekdayRetryAfterMidnightKeepsItsDay(t *testing.T) {
	rota := &fakeRota{assignments: []teamschedule.Assignment{
		assignment("Dee", "dee@example.com", "nova", "CRE_EVENING", "2026-10-16", "2026-10-16 18:00", "2026-10-16 21:00"),
	}}
	mail := &fakeEmail{}
	cfg := config("2026-10-16 01:00", weekdayRotations)
	cfg.Anchor = func() time.Time { return at("2026-10-15 15:00") }
	if err := WeekdayReminder(rota, mail, cfg)(context.Background()); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if rota.from != "2026-10-16" || rota.to != "2026-10-16" {
		t.Fatalf("read %s..%s, want Friday 2026-10-16, the day Thursday's run announces", rota.from, rota.to)
	}
	if len(mail.sent) != 1 || !strings.Contains(mail.sent[0].body, "rostered for the Evening rotation today, Friday, 16 October 2026") {
		t.Fatalf("sent = %+v, want one email saying today", mail.sent)
	}
}

// Friday's weekend notice, retried on Saturday, announces this weekend, not
// the next; Saturday's LK window has begun and is left out, Sunday's goes.
func TestAWeekendRetryOnSaturdayKeepsThisWeekend(t *testing.T) {
	rota := &fakeRota{assignments: []teamschedule.Assignment{
		assignment("Ann", "ann@example.com", "vega", "CRE_WEEKEND", "2026-10-17", "2026-10-17 09:00", "2026-10-17 21:00"),
		assignment("Ben", "ben@example.com", "nova", "CRE_WEEKEND", "2026-10-18", "2026-10-18 09:00", "2026-10-18 21:00"),
	}}
	mail := &fakeEmail{}
	cfg := config("2026-10-17 10:00", weekendRotations)
	cfg.Anchor = func() time.Time { return at("2026-10-16 15:00") }
	if err := WeekendNotice(rota, mail, cfg)(context.Background()); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if rota.from != "2026-10-17" || rota.to != "2026-10-18" {
		t.Fatalf("read %s..%s, want this weekend, 2026-10-17..18", rota.from, rota.to)
	}
	if len(mail.sent) != 1 || strings.Join(mail.sent[0].to, ",") != "ben@example.com" ||
		!strings.Contains(mail.sent[0].body, "rostered for the LK Weekend rotation tomorrow, Sunday, 18 October 2026") {
		t.Fatalf("sent = %+v, want only Sunday's email, to Ben, saying tomorrow", mail.sent)
	}
}

// A window that has started is never announced, even when nobody else is.
func TestAStartedWindowIsNotSent(t *testing.T) {
	rota := &fakeRota{assignments: []teamschedule.Assignment{
		assignment("Ann", "ann@example.com", "vega", "CRE_MORNING", "2026-10-16", "2026-10-16 06:00", "2026-10-16 09:00"),
	}}
	mail := &fakeEmail{}
	cfg := config("2026-10-16 06:00", weekdayRotations)
	cfg.Anchor = func() time.Time { return at("2026-10-15 15:00") }
	if err := WeekdayReminder(rota, mail, cfg)(context.Background()); err != nil || len(mail.sent) != 0 {
		t.Fatalf("err = %v, sent = %+v, want a quiet success", err, mail.sent)
	}
}

// A late retry counts its failures out of the emails it tried: the Morning
// window has started and is skipped, so the failed Evening email is 1 of 1.
func TestAFailureCountsOnlyTheEmailsTried(t *testing.T) {
	rota := &fakeRota{assignments: []teamschedule.Assignment{
		assignment("Ann", "ann@example.com", "vega", "CRE_MORNING", "2026-10-16", "2026-10-16 06:00", "2026-10-16 09:00"),
		assignment("Dee", "dee@example.com", "nova", "CRE_EVENING", "2026-10-16", "2026-10-16 18:00", "2026-10-16 21:00"),
	}}
	mail := &fakeEmail{failOn: "Evening"}
	cfg := config("2026-10-16 07:00", weekdayRotations)
	cfg.Anchor = func() time.Time { return at("2026-10-15 15:00") }
	err := WeekdayReminder(rota, mail, cfg)(context.Background())
	if err == nil || !strings.Contains(err.Error(), "1 of 1 emails failed") {
		t.Fatalf("err = %v, want 1 of 1 emails failed", err)
	}
}

// A rota read that fails fails the run.
func TestARotaReadFailureFailsTheRun(t *testing.T) {
	mail := &fakeEmail{}
	err := WeekdayReminder(&fakeRota{err: errors.New("400 unknown field")}, mail, config("2026-10-12 15:00", weekdayRotations))(context.Background())
	if err == nil || len(mail.sent) != 0 {
		t.Fatalf("err = %v, sent %d", err, len(mail.sent))
	}
}

// Names come from people, so they are escaped in the email and in the event.
func TestNamesAreEscaped(t *testing.T) {
	a := assignment("<script>alert(1)</script>", "ann@example.com", "vega", "CRE_MORNING", "2026-10-13", "2026-10-13 06:00", "2026-10-13 09:00")
	mail := &fakeEmail{}
	if err := WeekdayReminder(&fakeRota{assignments: []teamschedule.Assignment{a}}, mail, config("2026-10-12 15:00", weekdayRotations))(context.Background()); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if body := mail.sent[0].body; strings.Contains(body, "<script>") || !strings.Contains(body, "&lt;script&gt;") {
		t.Fatal("a name reached the body unescaped")
	}
}

// The Team Schedule address is written out and linked in every email; with no
// address the sentence is left out.
func TestTeamScheduleLinkIsInEveryEmail(t *testing.T) {
	const url = "https://csm.example.com/team-schedule"
	const sentence = `The latest roster is available in the Team Schedule: <a href="` + url + `">` + url + `</a>`
	ann := assignment("Ann", "ann@example.com", "vega", "CRE_MORNING", "2026-10-13", "2026-10-13 06:00", "2026-10-13 09:00")
	send := func(url string) sentEmail {
		mail := &fakeEmail{}
		cfg := config("2026-10-12 15:00", weekdayRotations)
		cfg.ScheduleURL = url
		if err := WeekdayReminder(&fakeRota{assignments: []teamschedule.Assignment{ann}}, mail, cfg)(context.Background()); err != nil {
			t.Fatalf("handler: %v", err)
		}
		return mail.sent[0]
	}
	if m := send(url); !strings.Contains(m.body, sentence) {
		t.Errorf("the link is missing:\n%s", m.body)
	}
	if m := send(""); strings.Contains(m.body, "latest roster") {
		t.Errorf("an empty address still produced the link:\n%s", m.body)
	}
}
