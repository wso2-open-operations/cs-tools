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

// markCheckConfig is rota_weekday_mark_check's configuration as main.go
// builds it: the weekday rotations, Evening needing 3 here.
func markCheckConfig(now string) Config {
	cfg := config(now, weekdayRotations)
	cfg.WeekdayRequired = map[string]int{"CRE_EVENING": 3}
	cfg.Task = "rota_weekday_mark_check"
	return cfg
}

// evening is n people on the Evening of rotaDate, one per team name given.
func evening(rotaDate string, names ...string) []teamschedule.Assignment {
	var out []teamschedule.Assignment
	for _, n := range names {
		out = append(out, assignment(n, strings.ToLower(n)+"@example.com", "vega", "CRE_EVENING",
			rotaDate, rotaDate+" 18:00", rotaDate+" 21:00"))
	}
	return out
}

// Saturday 17:00 checks Monday (whose reminder goes out on Sunday at 09:00):
// an Evening with 2 of 3 marked emails the rota admins, and only them.
func TestWeekdayMarkCheck_AShortRotationTellsTheRotaAdmins(t *testing.T) {
	rota := &fakeRota{assignments: evening("2026-10-19", "Ann", "Ben")}
	store := &fakeStore{admins: []teamschedule.RotaAdmin{{Name: "Ria", Email: "ria@example.com"}, {Name: "Raj", Email: "raj@example.com"}}}
	mail := &fakeEmail{}
	if err := WeekdayMarkCheck(rota, store, mail, markCheckConfig("2026-10-17 17:00"))(context.Background()); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if rota.from != "2026-10-19" || rota.to != "2026-10-19" {
		t.Fatalf("read %s..%s, want Monday 2026-10-19", rota.from, rota.to)
	}
	if len(mail.sent) != 1 {
		t.Fatalf("sent %d emails, want one to the rota admins: %+v", len(mail.sent), mail.sent)
	}
	got := mail.sent[0]
	if strings.Join(got.to, ",") != "ria@example.com,raj@example.com" || len(got.cc) != 0 {
		t.Errorf("to %v cc %v, want the two rota admins only", got.to, got.cc)
	}
	if want := "[CRE] Action needed: the weekday rota for Monday, 19 October is not complete"; got.subject != want {
		t.Errorf("subject = %q, want %q", got.subject, want)
	}
	for _, s := range []string{
		"go out on Sunday, 18 October at 09:00 IST, to the people marked by then",
		row("Monday, 19 October", "Evening Rotation", "2", "3"),
		"You are copied on each reminder",
	} {
		if !strings.Contains(got.body, s) {
			t.Errorf("body lacks %q\n%s", s, got.body)
		}
	}
}

// A fully marked day sends nothing.
func TestWeekdayMarkCheck_AFullyMarkedDaySendsNothing(t *testing.T) {
	rota := &fakeRota{assignments: evening("2026-10-19", "Ann", "Ben", "Cid")}
	store := &fakeStore{admins: []teamschedule.RotaAdmin{{Name: "Ria", Email: "ria@example.com"}}}
	mail := &fakeEmail{}
	if err := WeekdayMarkCheck(rota, store, mail, markCheckConfig("2026-10-17 17:00"))(context.Background()); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if len(mail.sent) != 0 {
		t.Errorf("sent %d emails for a fully marked day: %+v", len(mail.sent), mail.sent)
	}
}

// Wednesday checks Friday. The same person twice counts once, and an on-call
// person does not count toward the shift's people.
func TestWeekdayMarkCheck_WednesdayChecksFridayAndCountsPeopleOnce(t *testing.T) {
	people := evening("2026-10-23", "Ann", "Ben", "Ann")
	people = append(people, assignment("Oz", "oz@example.com", "nova", "CRE_EVENING_OC", "2026-10-23", "2026-10-23 18:00", "2026-10-23 21:00"))
	rota := &fakeRota{assignments: people}
	store := &fakeStore{admins: []teamschedule.RotaAdmin{{Name: "Ria", Email: "ria@example.com"}}}
	mail := &fakeEmail{}
	if err := WeekdayMarkCheck(rota, store, mail, markCheckConfig("2026-10-21 17:00"))(context.Background()); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if rota.from != "2026-10-23" {
		t.Fatalf("read %s, want Friday 2026-10-23", rota.from)
	}
	if len(mail.sent) != 1 || !strings.Contains(mail.sent[0].body, row("Friday, 23 October", "Evening Rotation", "2", "3")) {
		t.Errorf("want one email showing 2 of 3 on Friday's Evening, got %+v", mail.sent)
	}
}

// With no rota admin, the task's alert audience is told instead.
func TestWeekdayMarkCheck_NoRotaAdminTellsTheAlertAudience(t *testing.T) {
	rota := &fakeRota{assignments: evening("2026-10-19", "Ann")}
	cfg := markCheckConfig("2026-10-17 17:00")
	cfg.AlertTo = []string{"ops@example.com"}
	mail := &fakeEmail{}
	if err := WeekdayMarkCheck(rota, &fakeStore{}, mail, cfg)(context.Background()); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if len(mail.sent) != 1 || strings.Join(mail.sent[0].to, ",") != "ops@example.com" {
		t.Errorf("want one email to the alert audience, got %+v", mail.sent)
	}
}

// No weekday headcount configured: nothing is read or sent.
func TestWeekdayMarkCheck_NoHeadcountChecksNothing(t *testing.T) {
	rota := &fakeRota{}
	cfg := markCheckConfig("2026-10-17 17:00")
	cfg.WeekdayRequired = nil
	mail := &fakeEmail{}
	if err := WeekdayMarkCheck(rota, &fakeStore{}, mail, cfg)(context.Background()); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if rota.reads != 0 || len(mail.sent) != 0 {
		t.Errorf("reads %d, sent %d; want neither", rota.reads, len(mail.sent))
	}
}

// A required code the reminders do not announce is not checked: nobody the
// reminders reach could ever mark it.
func TestWeekdayMarkCheck_AnUnannouncedRotationIsNotChecked(t *testing.T) {
	rota := &fakeRota{}
	cfg := markCheckConfig("2026-10-17 17:00")
	cfg.Rotations = map[string]string{"CRE_MORNING": "Morning"}
	mail := &fakeEmail{}
	if err := WeekdayMarkCheck(rota, &fakeStore{admins: []teamschedule.RotaAdmin{{Email: "ria@example.com"}}}, mail, cfg)(context.Background()); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if rota.reads != 0 || len(mail.sent) != 0 {
		t.Errorf("reads %d, sent %d; want neither for an Evening the reminders never announce", rota.reads, len(mail.sent))
	}
}

// With no rota admin and no alert To, the alert Cc list is told.
func TestWeekdayMarkCheck_NoAdminAndNoAlertToTellsTheAlertCc(t *testing.T) {
	rota := &fakeRota{assignments: evening("2026-10-19", "Ann")}
	cfg := markCheckConfig("2026-10-17 17:00")
	cfg.AlertCc = []string{"watch@example.com"}
	mail := &fakeEmail{}
	if err := WeekdayMarkCheck(rota, &fakeStore{}, mail, cfg)(context.Background()); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if len(mail.sent) != 1 || strings.Join(mail.sent[0].to, ",") != "watch@example.com" {
		t.Errorf("want one email to the alert Cc list, got %+v", mail.sent)
	}
}

// A retry that runs after the reminder went out sends nothing; one that runs
// after midnight but before it names the reminder's own day.
func TestWeekdayMarkCheck_ALateRetry(t *testing.T) {
	cfg := markCheckConfig("2026-10-22 10:00") // Thursday 10:00, after Thursday's reminder
	cfg.Anchor = func() time.Time { return at("2026-10-21 17:00") }
	rota := &fakeRota{assignments: evening("2026-10-23", "Ann")}
	mail := &fakeEmail{}
	if err := WeekdayMarkCheck(rota, &fakeStore{admins: []teamschedule.RotaAdmin{{Email: "ria@example.com"}}}, mail, cfg)(context.Background()); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if rota.reads != 0 || len(mail.sent) != 0 {
		t.Errorf("reads %d, sent %d after the reminder went out; want neither", rota.reads, len(mail.sent))
	}

	cfg = markCheckConfig("2026-10-22 01:00") // Thursday 01:00, before Thursday's reminder
	cfg.Anchor = func() time.Time { return at("2026-10-21 17:00") }
	mail = &fakeEmail{}
	if err := WeekdayMarkCheck(rota, &fakeStore{admins: []teamschedule.RotaAdmin{{Email: "ria@example.com"}}}, mail, cfg)(context.Background()); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if len(mail.sent) != 1 || !strings.Contains(mail.sent[0].body, "go out on Thursday, 22 October at 09:00 IST") {
		t.Errorf("want one email naming Thursday's reminder, got %+v", mail.sent)
	}
}

// fakeAdmins answers who the rota admins are, or fails.
type fakeAdmins struct {
	admins []teamschedule.RotaAdmin
	err    error
}

func (f fakeAdmins) RotaAdmins(context.Context, string) ([]teamschedule.RotaAdmin, error) {
	return f.admins, f.err
}

// Every weekday reminder copies the rota admins, the email with the
// invitation included, without making them guests of the event; the plain
// copy for cfg.Copy does not go to them again.
func TestWeekdayReminder_CopiesTheRotaAdminsNotAsGuests(t *testing.T) {
	rota := &fakeRota{assignments: evening("2026-10-19", "Ann")}
	cfg := config("2026-10-16 15:00", weekdayRotations)
	cfg.Invites, cfg.OrganizerEmail = true, "rota@example.com"
	cfg.Copy = []string{"lead@example.com", "ria@example.com"}
	cfg.InviteCopy = []string{"cal@example.com"}
	cfg.Admins = fakeAdmins{admins: []teamschedule.RotaAdmin{{Name: "Ria", Email: "ria@example.com"}, {Name: "Ann", Email: "ann@example.com"}}}
	mail := &fakeEmail{}
	if err := WeekdayReminder(rota, mail, cfg)(context.Background()); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if len(mail.sent) != 2 {
		t.Fatalf("sent %d emails, want the reminder and one copy: %+v", len(mail.sent), mail.sent)
	}
	reminder, copied := mail.sent[0], mail.sent[1]
	// Ann is on the rotation, so she is not copied as an admin as well.
	if strings.Join(reminder.to, ",") != "ann@example.com" || strings.Join(reminder.cc, ",") != "cal@example.com,ria@example.com" {
		t.Errorf("reminder to %v cc %v, want Ann, Cc the invitation copy and the rota admin", reminder.to, reminder.cc)
	}
	if len(reminder.attachments) != 1 {
		t.Fatalf("reminder has %d attachments, want the invitation", len(reminder.attachments))
	}
	ics := string(reminder.attachments[0].Content)
	if !strings.Contains(ics, "mailto:ann@example.com") || strings.Contains(ics, "ria@example.com") {
		t.Errorf("invitation guests wrong: Ann must be a guest, the rota admin must not\n%s", ics)
	}
	if strings.Join(copied.to, ",") != "lead@example.com" {
		t.Errorf("plain copy to %v, want the lead only (the rota admin already has the reminder)", copied.to)
	}
}

// A failed rota admin read does not hold back the reminders.
func TestWeekdayReminder_AFailedAdminReadStillSends(t *testing.T) {
	rota := &fakeRota{assignments: evening("2026-10-19", "Ann")}
	cfg := config("2026-10-16 15:00", weekdayRotations)
	cfg.Admins = fakeAdmins{err: errors.New("entity-service down")}
	mail := &fakeEmail{}
	if err := WeekdayReminder(rota, mail, cfg)(context.Background()); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if len(mail.sent) != 1 || len(mail.sent[0].cc) != 0 {
		t.Errorf("want the reminder sent without copies, got %+v", mail.sent)
	}
}

// "Send a test to me" copies nobody, the rota admins included.
func TestAsTestDropsTheRotaAdmins(t *testing.T) {
	cfg := config("2026-10-16 15:00", weekdayRotations)
	cfg.Admins = fakeAdmins{admins: []teamschedule.RotaAdmin{{Email: "ria@example.com"}}}
	cfg.AdminCopy = []string{"ria@example.com"}
	got := asTest(cfg, "me@example.com")
	if got.Admins != nil || got.AdminCopy != nil || adminCopy(context.Background(), got) != nil {
		t.Errorf("a test still copies the rota admins: %+v / %v", got.Admins, got.AdminCopy)
	}
}
