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

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/teamschedule"
)

type fakeSettings struct {
	s   *teamschedule.ReminderSettings
	err error
}

func (f fakeSettings) ReminderSettingsFor(context.Context, string) (*teamschedule.ReminderSettings, error) {
	return f.s, f.err
}

func savedSettings() *teamschedule.ReminderSettings {
	return &teamschedule.ReminderSettings{
		WeekdayEnabled: true, WeekendEnabled: true, InvitesEnabled: true,
		WeekendRequired: map[string]int{"CRE_WEEKEND": 3, "CRE_WEEKEND_NIGHT": 1},
		Windows: map[string]teamschedule.ReminderWindow{
			"CRE_WEEKEND_NIGHT":    {Start: "08:00", End: "18:00", TimeZone: "America/Los_Angeles"},
			"CRE_WEEKEND_NIGHT_OC": {Start: "08:00", End: "18:00", TimeZone: "America/Los_Angeles"},
		},
	}
}

// Turned off on the Rota Access tab: the run succeeds and sends nothing,
// weekday and weekend alike.
func TestSettings_TurnedOff(t *testing.T) {
	off := savedSettings()
	off.WeekdayEnabled, off.WeekendEnabled = false, false

	mail := &fakeEmail{}
	cfg := config("2026-10-19 15:00", weekdayRotations)
	cfg.Settings = fakeSettings{s: off}
	if err := WeekdayReminder(&fakeRota{}, mail, cfg)(context.Background()); err != nil || len(mail.sent) != 0 {
		t.Errorf("weekday: err %v, sent %d", err, len(mail.sent))
	}
	wcfg := weekendConfig("2026-10-18 20:00", "rota_weekend_notice")
	wcfg.Settings = fakeSettings{s: off}
	rota := &fakeRota{assignments: fullWeekend()}
	if err := WeekendNotice(rota, &fakeStore{}, mail, wcfg)(context.Background()); err != nil || len(mail.sent) != 0 || rota.reads != 0 {
		t.Errorf("weekend: err %v, sent %d, rota read %d times", err, len(mail.sent), rota.reads)
	}
}

// What was saved replaces the configuration: a higher LK minimum makes the
// full weekend incomplete (admins asked, nothing to the rota), and the copy
// list is copied.
func TestSettings_ReplaceTheConfiguration(t *testing.T) {
	s := savedSettings()
	s.WeekendRequired["CRE_WEEKEND"] = 4
	s.CopyTo = []string{"lead@example.com"}
	store := &fakeStore{admins: []teamschedule.RotaAdmin{{Name: "Zoe Admin", Email: "zoe@example.com"}}}
	mail := &fakeEmail{}
	cfg := weekendConfig("2026-10-18 20:00", "rota_weekend_notice")
	cfg.Settings = fakeSettings{s: s}
	if err := WeekendNotice(&fakeRota{assignments: fullWeekend()}, store, mail, cfg)(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(mail.sent) != 1 || !strings.Contains(mail.sent[0].subject, "Action needed") ||
		!strings.Contains(mail.sent[0].body, "LK Weekend Rotation") {
		t.Fatalf("sent %d: %+v", len(mail.sent), mail.sent)
	}

	// Complete again, with invitations off and other hours: the summary is
	// copied to the lead and no invitation goes out.
	s.WeekendRequired["CRE_WEEKEND"] = 3
	s.InvitesEnabled = false
	s.Windows["CRE_WEEKEND_NIGHT"] = teamschedule.ReminderWindow{Start: "09:00", End: "17:00", TimeZone: "America/New_York"}
	mail = &fakeEmail{}
	if err := WeekendNotice(&fakeRota{assignments: fullWeekend()}, &fakeStore{}, mail, cfg)(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(mail.sent) != 1 {
		t.Fatalf("sent %d emails, want the summary alone", len(mail.sent))
	}
	summary := mail.sent[0]
	if strings.Join(summary.cc, ",") != "lead@example.com" || !strings.Contains(summary.body, "09:00 AM &#8211; 05:00 PM EDT") {
		t.Errorf("summary cc %v:\n%s", summary.cc, summary.body)
	}
}

// A failed read fails the run before anything is sent. With nothing saved
// for the family the automatic reminders stay off: only the switches on the
// Rota Access tab turn them on.
func TestSettings_ReadFailureAndNothingSaved(t *testing.T) {
	mail := &fakeEmail{}
	cfg := weekendConfig("2026-10-18 20:00", "rota_weekend_notice")
	cfg.Settings = fakeSettings{err: errors.New("entity-service down")}
	if err := WeekendNotice(&fakeRota{assignments: fullWeekend()}, &fakeStore{}, mail, cfg)(context.Background()); err == nil || len(mail.sent) != 0 {
		t.Errorf("err %v, sent %d", err, len(mail.sent))
	}
	cfg.Settings = fakeSettings{}
	store := &fakeStore{}
	if err := WeekendNotice(&fakeRota{assignments: fullWeekend()}, store, mail, cfg)(context.Background()); err != nil || len(mail.sent) != 0 || len(store.recorded) != 0 {
		t.Errorf("nothing saved: err %v, sent %d, recorded %d; want nothing", err, len(mail.sent), len(store.recorded))
	}
	weekday := config("2026-10-19 10:00", weekdayRotations)
	weekday.EmailsEnabled, weekday.Settings = true, fakeSettings{}
	rota := &fakeRota{assignments: []teamschedule.Assignment{
		assignment("Ann", "ann@example.com", "vega", "CRE_MORNING", "2026-10-20", "2026-10-20 06:00", "2026-10-20 09:00"),
	}}
	if err := WeekdayReminder(rota, mail, weekday)(context.Background()); err != nil || len(mail.sent) != 0 {
		t.Errorf("weekday, nothing saved: err %v, sent %d; want nothing", err, len(mail.sent))
	}
}

// Send now is asked for by hand, so it goes out although nothing is saved for
// the family, with the configured values.
func TestSettings_SendNowWorksWithNothingSaved(t *testing.T) {
	weekday, weekend := manualConfigs("2026-10-19 10:00")
	weekday.Settings, weekend.Settings = fakeSettings{}, fakeSettings{}
	queue := &fakeQueue{waiting: []teamschedule.ReminderRequest{
		{ID: "r1", Kind: "weekend"},
		{ID: "t1", Kind: "weekend", Test: true, RequestedBy: "zoe@example.com"},
	}}
	mail := &fakeEmail{}
	if err := ManualReminder(&fakeRota{assignments: fullWeekend()}, &fakeStore{}, queue, mail, weekday, weekend)(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(queue.reports) != 2 || queue.reports[0] != "r1 SENT Sent 5 emails." || queue.reports[1] != "t1 SENT Sent 5 emails to zoe@example.com (test)." {
		t.Fatalf("reports %v", queue.reports)
	}
}

// Saved settings with no minimums and no windows (entity-service stores {}
// for either when left out) keep the configured ones: the incomplete weekend
// still asks the rota admins, and the Americas hours stay on Pacific time.
func TestSettings_EmptyMapsKeepTheConfiguration(t *testing.T) {
	s := savedSettings()
	s.WeekendRequired = map[string]int{}
	s.Windows = map[string]teamschedule.ReminderWindow{}
	cfg := weekendConfig("2026-10-18 20:00", "rota_weekend_notice")
	cfg.Settings = fakeSettings{s: s}
	got, on, err := withSettings(context.Background(), cfg, true)
	if err != nil || !on {
		t.Fatalf("on %v, err %v", on, err)
	}
	if got.Required["CRE_WEEKEND"] != 3 || got.Required["CRE_WEEKEND_NIGHT"] != 1 {
		t.Errorf("Required = %v, want the configured CRE_WEEKEND=3, CRE_WEEKEND_NIGHT=1", got.Required)
	}
	if w, ok := got.Windows["CRE_WEEKEND_NIGHT"]; !ok || w.Zone.String() != "America/Los_Angeles" {
		t.Errorf("Windows = %v, want the configured America/Los_Angeles window", got.Windows)
	}
}

// Saved values override the configured ones code by code, and the saved
// invitations switch can only turn invitations off: a save that left a
// rotation, or the invitations, untouched keeps what the deployment set.
func TestSettings_OverrideCodeByCodeAndNeverTurnInvitesOn(t *testing.T) {
	cfg := weekendConfig("2026-10-18 20:00", "rota_weekend_notice")
	cfg.Required = map[string]int{"CRE_WEEKEND": 3, "CRE_WEEKEND_NIGHT": 1}
	cfg.Invites = false // the deployment has them off
	saved := &teamschedule.ReminderSettings{
		WeekdayEnabled: true, WeekendEnabled: true, InvitesEnabled: true,
		WeekendRequired: map[string]int{"CRE_WEEKEND": 4},
		Windows:         map[string]teamschedule.ReminderWindow{"CRE_WEEKEND_NIGHT": {Start: "09:00", End: "17:00", TimeZone: "America/New_York"}},
	}
	cfg.Settings = fakeSettings{s: saved}
	got, on, err := withSettings(context.Background(), cfg, true)
	if err != nil || !on {
		t.Fatalf("on %v, err %v", on, err)
	}
	if got.Required["CRE_WEEKEND"] != 4 || got.Required["CRE_WEEKEND_NIGHT"] != 1 {
		t.Errorf("required %v, want the saved LK minimum and the configured Americas one", got.Required)
	}
	if got.Invites {
		t.Error("a saved invitations switch turned invitations on where the deployment has them off")
	}
	if w := got.Windows["CRE_WEEKEND_NIGHT"]; w.Zone.String() != "America/New_York" || w.Start != 9*60 {
		t.Errorf("Americas window %+v, want the saved one", w)
	}
	if w := got.Windows["CRE_WEEKEND_NIGHT_OC"]; w.Zone == nil || w.Zone.String() != "America/Los_Angeles" {
		t.Errorf("LK On-call window %+v, want the configured one", w)
	}
	if cfg.Required["CRE_WEEKEND"] != 3 {
		t.Error("the configured minimums were changed in place")
	}
}
