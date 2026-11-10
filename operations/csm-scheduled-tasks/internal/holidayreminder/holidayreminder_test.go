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

package holidayreminder

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/teamschedule"
)

type fakeClient struct {
	due      []teamschedule.DueHolidayReminder
	asked    string
	recorded []string
	admins   []teamschedule.RotaAdmin
}

func (f *fakeClient) DueHolidayReminders(_ context.Context, today string) ([]teamschedule.DueHolidayReminder, error) {
	f.asked = today
	return f.due, nil
}

func (f *fakeClient) RecordHolidayReminder(_ context.Context, team, date, stage string) error {
	f.recorded = append(f.recorded, team+" "+date+" "+stage)
	return nil
}

func (f *fakeClient) RotaAdmins(context.Context, string) ([]teamschedule.RotaAdmin, error) {
	return f.admins, nil
}

type sent struct {
	to            []string
	subject, body string
}

type fakeSender struct {
	mails []sent
	fail  bool
}

func (f *fakeSender) SendEmail(_ context.Context, to, _ []string, subject, body string) error {
	if f.fail {
		return errors.New("down")
	}
	f.mails = append(f.mails, sent{to, subject, body})
	return nil
}

func reminder(stage string, recipients ...teamschedule.HolidayRecipient) teamschedule.DueHolidayReminder {
	return teamschedule.DueHolidayReminder{
		Stage: stage, DaysBefore: 14,
		Availability: teamschedule.HolidayAvailability{
			TeamKey: "atlas", TeamName: "Atlas", Date: "2026-11-30",
			Titles: []string{"Il Full Moon Poya Day"}, Countries: []string{"Sri Lanka"},
			Members: []teamschedule.HolidayMember{
				{Name: "Ann <b>", Status: "TAGGED"}, {Name: "Ben", Status: "AVAILABLE"}, {Name: "Cal", Status: "AWAY", AwayKind: "AL"},
			},
			Uncovered: []teamschedule.HolidayTurn{{ShiftLabel: "Morning", Name: "Ann <b>"}},
		},
		Recipients: recipients,
	}
}

var colombo = func() *time.Location { z, _ := time.LoadLocation("Asia/Colombo"); return z }()

func TestRun_SendsAndRecordsEachReminder(t *testing.T) {
	c := &fakeClient{due: []teamschedule.DueHolidayReminder{
		reminder("notice", teamschedule.HolidayRecipient{Name: "Lead One", Email: "lead@example.com", Role: "lead"}),
	}}
	s := &fakeSender{}
	// 03:30 UTC is 09:00 in Colombo, the same calendar day.
	anchor := time.Date(2026, 11, 16, 3, 30, 0, 0, time.UTC)
	err := Run(c, s, Config{EmailsEnabled: true, Zone: colombo, Anchor: func() time.Time { return anchor },
		ScheduleURL: "https://portal.example.com/team-schedule"})(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if c.asked != "2026-11-16" {
		t.Errorf("asked for %s", c.asked)
	}
	if len(s.mails) != 1 || s.mails[0].to[0] != "lead@example.com" {
		t.Fatalf("mails %+v", s.mails)
	}
	m := s.mails[0]
	if !strings.Contains(m.subject, "Monday, 30 November 2026") || !strings.Contains(m.subject, "Atlas") {
		t.Errorf("subject %q", m.subject)
	}
	for _, want := range []string{"Dear Lead One,", "Ann &lt;b&gt;", "Off (Public holiday)", "Working", "Away (AL)",
		"Morning (Ann &lt;b&gt;)", "Confirm team availability", "https://portal.example.com/team-schedule"} {
		if !strings.Contains(m.body, want) {
			t.Errorf("body lacks %q", want)
		}
	}
	if len(c.recorded) != 1 || c.recorded[0] != "atlas 2026-11-30 notice" {
		t.Errorf("recorded %v", c.recorded)
	}
}

func TestRun_NoLeadGoesToRotaAdmins_RedirectAndFailures(t *testing.T) {
	c := &fakeClient{due: []teamschedule.DueHolidayReminder{reminder("one_day")},
		admins: []teamschedule.RotaAdmin{{Name: "Admin", Email: "admin@example.com"}}}
	s := &fakeSender{}
	err := Run(c, s, Config{EmailsEnabled: true, RedirectTo: []string{"test@example.com"}})(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if s.mails[0].to[0] != "test@example.com" || !strings.Contains(s.mails[0].body, "admin@example.com") {
		t.Errorf("redirect: %+v", s.mails[0])
	}
	if !strings.Contains(s.mails[0].subject, "Reminder") || !strings.Contains(s.mails[0].body, "Last reminder") {
		t.Errorf("one_day wording: %q", s.mails[0].subject)
	}

	failing := &fakeClient{due: []teamschedule.DueHolidayReminder{reminder("week", teamschedule.HolidayRecipient{Email: "l@example.com"})}}
	if err := Run(failing, &fakeSender{fail: true}, Config{EmailsEnabled: true})(context.Background()); err == nil {
		t.Fatal("a failed send did not fail the run")
	}
	if len(failing.recorded) != 0 {
		t.Error("a failed send was recorded")
	}

	off := &fakeClient{due: []teamschedule.DueHolidayReminder{reminder("week")}}
	if err := Run(off, &fakeSender{}, Config{EmailsEnabled: false})(context.Background()); err != nil || off.asked != "" {
		t.Error("ALERTS_ENABLED=false still asked or sent")
	}
}
