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

package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// fakeHolidayRepo keeps the placed tags, turns, confirmations and sends in
// memory, with the repository's rules (a placement is never repeated, a day
// already tagged is skipped).
type fakeHolidayRepo struct {
	members       []domain.SchedulePerson
	otherTag      map[string]string // userID|date -> kind code already there
	placed        []domain.HolidayPlacedRow
	turns         []domain.HolidayTurn
	confirmations map[string]domain.HolidayConfirmationRow
	sent          map[string][]string
	recipients    []domain.HolidayReminderRecipient
	removed       int
}

func (f *fakeHolidayRepo) CREMembers(context.Context) ([]domain.SchedulePerson, error) {
	return f.members, nil
}

func (f *fakeHolidayRepo) PlaceHolidayTag(_ context.Context, p domain.HolidayTagPlacement) (bool, error) {
	for _, r := range f.placed {
		if r.UserID == p.UserID && r.Date == p.Date {
			return false, nil
		}
	}
	if f.otherTag[p.UserID+"|"+p.Date] != "" {
		return false, nil
	}
	f.placed = append(f.placed, domain.HolidayPlacedRow{
		UserID: p.UserID, Name: p.UserID[:2], TeamKey: p.TeamKey, TeamName: strings.ToUpper(p.TeamKey),
		Date: p.Date, Country: p.Country, Title: p.Title, AbsenceID: "abs-" + p.UserID[:2] + p.Date,
	})
	return true, nil
}

func (f *fakeHolidayRepo) RemoveCancelledHolidayTags(context.Context, string) (int, error) {
	return f.removed, nil
}

func (f *fakeHolidayRepo) PlacedBetween(_ context.Context, from, to string, teams []string) ([]domain.HolidayPlacedRow, error) {
	var out []domain.HolidayPlacedRow
	for _, p := range f.placed {
		if p.Date < from || p.Date > to || (len(teams) > 0 && !containsFold(teams, p.TeamKey)) {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}

func (f *fakeHolidayRepo) TurnsBetween(_ context.Context, from, to string, teams []string) ([]domain.HolidayTurn, error) {
	var out []domain.HolidayTurn
	for _, t := range f.turns {
		if t.Date >= from && t.Date <= to && containsFold(teams, t.TeamKey) {
			out = append(out, t)
		}
	}
	return out, nil
}

func (f *fakeHolidayRepo) ConfirmationsBetween(_ context.Context, from, to string, _ []string) ([]domain.HolidayConfirmationRow, error) {
	var out []domain.HolidayConfirmationRow
	for _, c := range f.confirmations {
		if c.Date >= from && c.Date <= to {
			out = append(out, c)
		}
	}
	return out, nil
}

func (f *fakeHolidayRepo) Confirm(_ context.Context, team, date, by string) (domain.HolidayConfirmationRow, error) {
	if f.confirmations == nil {
		f.confirmations = map[string]domain.HolidayConfirmationRow{}
	}
	c := domain.HolidayConfirmationRow{TeamKey: team, Date: date, ConfirmedBy: by, ConfirmedOn: time.Now()}
	f.confirmations[strings.ToLower(team)+date] = c
	return c, nil
}

func (f *fakeHolidayRepo) Withdraw(_ context.Context, team, date string) (bool, error) {
	k := strings.ToLower(team) + date
	_, ok := f.confirmations[k]
	delete(f.confirmations, k)
	return ok, nil
}

func (f *fakeHolidayRepo) ReminderRecipients(context.Context, string) ([]domain.HolidayReminderRecipient, error) {
	return f.recipients, nil
}

func (f *fakeHolidayRepo) SentStages(context.Context, string, string) (map[string][]string, error) {
	return f.sent, nil
}

func (f *fakeHolidayRepo) RecordReminder(_ context.Context, team, date, stage string) error {
	if f.sent == nil {
		f.sent = map[string][]string{}
	}
	k := repository.HolidayStageKey(team, date)
	f.sent[k] = append(f.sent[k], stage)
	return nil
}

const (
	leadEmail  = "lead@example.com"
	otherEmail = "someone@example.com"
)

// leadsAtlasOnly lets leadEmail change atlas's rota and nobody anything else.
func leadsAtlasOnly(ctx context.Context, team string) error {
	if auth.IdentityFromContext(ctx).UserEmail == leadEmail && strings.EqualFold(team, "atlas") {
		return nil
	}
	return &apierror.ForbiddenError{Msg: "no"}
}

func asUser(email string) context.Context {
	return auth.WithIdentity(context.Background(), auth.Identity{Validated: true, UserEmail: email})
}

func newHolidaySvc(repo *fakeHolidayRepo, holidays *fakePublicHolidayRepo, now time.Time) *holidayAvailabilityService {
	svc := NewHolidayAvailabilityService(repo, holidays, alwaysUnrestrictedAccess{}, leadsAtlasOnly, time.UTC).(*holidayAvailabilityService)
	svc.now = func() time.Time { return now }
	return svc
}

func TestPlaceUpcoming_PlacesWeekdayHolidaysOncePerPersonAndSkipsTaggedDays(t *testing.T) {
	repo := &fakeHolidayRepo{
		members: []domain.SchedulePerson{
			{UserID: lkUser, TeamKeys: []string{"atlas"}},
			{UserID: inUser, TeamKeys: []string{"phoenix"}},
			{UserID: frUser, TeamKeys: []string{"atlas"}, Location: "France"},
		},
		otherTag: map[string]string{frUser + "|2026-11-20": "AL"},
		removed:  1,
	}
	hol := &fakePublicHolidayRepo{indiaKey: "phoenix", stored: map[string][]domain.PublicHolidayDay{
		"Sri Lanka": {{Country: "Sri Lanka", Date: "2026-11-16", Title: "Poya"}, {Country: "Sri Lanka", Date: "2026-11-21", Title: "Weekend day"},
			{Country: "Sri Lanka", Date: "2026-12-25", Title: "Too far"}},
		"India":  {{Country: "India", Date: "2026-11-17", Title: "Diwali"}},
		"France": {{Country: "France", Date: "2026-11-20", Title: "Leave wins"}},
	}}
	svc := newHolidaySvc(repo, hol, time.Date(2026, 11, 10, 3, 30, 0, 0, time.UTC))

	res, err := svc.PlaceUpcoming(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// Placed: lk on the 16th, in on the 17th. Skipped: fr on the 20th (leave).
	// Not considered: the Saturday, and the 25th (past three weeks).
	if res.Placed != 2 || res.Skipped != 1 || res.Removed != 1 {
		t.Fatalf("got %+v, want placed 2, skipped 1, removed 1", res)
	}
	again, err := svc.PlaceUpcoming(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if again.Placed != 0 || again.Skipped != 3 {
		t.Fatalf("a second run placed again: %+v", again)
	}
	for _, p := range repo.placed {
		if p.UserID == lkUser && p.TeamKey != "atlas" {
			t.Errorf("filed under %q, want the member's team", p.TeamKey)
		}
	}
}

// atlasOnPoya is atlas with two members tagged on 2026-11-16 and two turns.
func atlasOnPoya() *fakeHolidayRepo {
	return &fakeHolidayRepo{
		placed: []domain.HolidayPlacedRow{
			{UserID: lkUser, Name: "Ann", TeamKey: "atlas", TeamName: "Atlas", Date: "2026-11-16", Country: "Sri Lanka", Title: "Poya", AbsenceID: "a1"},
			{UserID: frUser, Name: "Ben", TeamKey: "atlas", TeamName: "Atlas", Date: "2026-11-16", Country: "Sri Lanka", Title: "Poya"},
			{UserID: inUser, Name: "Cal", TeamKey: "atlas", TeamName: "Atlas", Date: "2026-11-16", Country: "Sri Lanka", Title: "Poya", OtherKind: "AL"},
		},
		turns: []domain.HolidayTurn{
			{TeamKey: "atlas", Date: "2026-11-16", ShiftCode: "CRE_MORNING", ShiftLabel: "Morning", UserID: lkUser, Name: "Ann", Away: true},
			{TeamKey: "atlas", Date: "2026-11-16", ShiftCode: "CRE_EVENING", ShiftLabel: "Evening", UserID: frUser, Name: "Ben"},
		},
		recipients: []domain.HolidayReminderRecipient{{Name: "Lead", Email: leadEmail, Role: "lead"}},
	}
}

func TestSearch_MemberStatusesTurnsAndCanConfirm(t *testing.T) {
	repo := atlasOnPoya()
	svc := newHolidaySvc(repo, &fakePublicHolidayRepo{}, time.Date(2026, 11, 10, 3, 30, 0, 0, time.UTC))
	resp, err := svc.Search(asUser(leadEmail), domain.SearchHolidayAvailabilityRequest{From: "2026-11-01", To: "2026-11-30"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Count != 1 {
		t.Fatalf("got %d holidays, want 1", resp.Count)
	}
	h := resp.Holidays[0]
	want := map[string]string{"Ann": domain.HolidayMemberTagged, "Ben": domain.HolidayMemberAvailable, "Cal": domain.HolidayMemberAway}
	for _, m := range h.Members {
		if m.Status != want[m.Name] {
			t.Errorf("%s: status %s, want %s", m.Name, m.Status, want[m.Name])
		}
	}
	if h.Turns != 2 || len(h.Uncovered) != 1 || h.Uncovered[0].ShiftLabel != "Morning" {
		t.Errorf("turns %d, uncovered %+v", h.Turns, h.Uncovered)
	}
	if h.Status != domain.HolidayAvailabilityNeedsMarking || h.FullyMarked || !h.CanConfirm {
		t.Errorf("status %s, fullyMarked %v, canConfirm %v", h.Status, h.FullyMarked, h.CanConfirm)
	}
	other, err := svc.Search(asUser(otherEmail), domain.SearchHolidayAvailabilityRequest{From: "2026-11-01", To: "2026-11-30"})
	if err != nil {
		t.Fatal(err)
	}
	if other.Holidays[0].CanConfirm {
		t.Error("somebody who may not change atlas's rota was offered Confirm")
	}
}

func TestConfirm_RefusedWhileATurnIsUncoveredThenFullyMarked(t *testing.T) {
	repo := atlasOnPoya()
	svc := newHolidaySvc(repo, &fakePublicHolidayRepo{}, time.Date(2026, 11, 10, 3, 30, 0, 0, time.UTC))

	_, err := svc.Confirm(asUser(leadEmail), "atlas", "2026-11-16")
	var ce *apierror.ConflictError
	if !errors.As(err, &ce) || !strings.Contains(ce.Msg, "Morning (Ann)") {
		t.Fatalf("got %v, want a conflict naming the Morning turn", err)
	}

	// The lead makes Ann available (removes her tag): the turn is covered.
	repo.turns[0].Away = false
	repo.placed[0].AbsenceID = ""
	h, err := svc.Confirm(asUser(leadEmail), "atlas", "2026-11-16")
	if err != nil {
		t.Fatal(err)
	}
	if h.Status != domain.HolidayAvailabilityConfirmed || !h.FullyMarked || h.ConfirmedBy != leadEmail {
		t.Fatalf("got %s / %v / %q", h.Status, h.FullyMarked, h.ConfirmedBy)
	}

	// Ann is tagged again afterwards: the confirmation stays but needs attention.
	repo.turns[0].Away = true
	resp, _ := svc.Search(asUser(leadEmail), domain.SearchHolidayAvailabilityRequest{From: "2026-11-16", To: "2026-11-16"})
	if got := resp.Holidays[0].Status; got != domain.HolidayAvailabilityNeedsAttention {
		t.Errorf("after a turn lost its person: %s, want NEEDS_ATTENTION", got)
	}

	if _, err := svc.Withdraw(asUser(leadEmail), "atlas", "2026-11-16"); err != nil {
		t.Fatal(err)
	}
	var nf *apierror.NotFoundError
	if _, err := svc.Withdraw(asUser(leadEmail), "atlas", "2026-11-16"); !errors.As(err, &nf) {
		t.Errorf("second withdraw: %v, want not found", err)
	}
}

func TestConfirm_Refusals(t *testing.T) {
	repo := atlasOnPoya()
	repo.turns[0].Away = false
	svc := newHolidaySvc(repo, &fakePublicHolidayRepo{}, time.Date(2026, 11, 10, 3, 30, 0, 0, time.UTC))
	var fe *apierror.ForbiddenError
	if _, err := svc.Confirm(asUser(otherEmail), "atlas", "2026-11-16"); !errors.As(err, &fe) {
		t.Errorf("not the team's lead: %v, want forbidden", err)
	}
	if _, err := svc.Confirm(context.Background(), "atlas", "2026-11-16"); !errors.As(err, &fe) {
		t.Errorf("service credential: %v, want forbidden", err)
	}
	var ve *apierror.ValidationError
	if _, err := svc.Confirm(asUser(leadEmail), "atlas", "2026-11-01"); !errors.As(err, &ve) {
		t.Errorf("past holiday: %v, want validation", err)
	}
	var nf *apierror.NotFoundError
	if _, err := svc.Confirm(asUser(leadEmail), "atlas", "2026-11-17"); !errors.As(err, &nf) {
		t.Errorf("no holiday that day: %v, want not found", err)
	}
}

func TestHolidayReminderStage(t *testing.T) {
	d := func(s string) time.Time { v, _ := time.Parse(time.DateOnly, s); return v }
	// Monday 2026-11-30: notice Mon 16th, week Mon 23rd, two days Thu 26th,
	// one day Fri 27th.
	cases := map[string]string{
		"2026-11-15": "",
		"2026-11-16": domain.HolidayReminderNotice,
		"2026-11-22": domain.HolidayReminderNotice,
		"2026-11-23": domain.HolidayReminderWeek,
		"2026-11-25": domain.HolidayReminderWeek,
		"2026-11-26": domain.HolidayReminderTwoDays,
		"2026-11-27": domain.HolidayReminderOneDay,
		"2026-11-29": domain.HolidayReminderOneDay,
		"2026-11-30": "",
	}
	for today, want := range cases {
		if got := holidayReminderStage(d(today), d("2026-11-30")); got != want {
			t.Errorf("%s: %q, want %q", today, got, want)
		}
	}
	// A Wednesday holiday: two days is Monday, one day Tuesday.
	if got := holidayReminderStage(d("2026-12-07"), d("2026-12-09")); got != domain.HolidayReminderTwoDays {
		t.Errorf("Monday before a Wednesday: %q", got)
	}
}

func TestDueReminders_NoticeAlwaysFollowUpsOnlyUntilFullyMarked(t *testing.T) {
	repo := atlasOnPoya()
	svc := newHolidaySvc(repo, &fakePublicHolidayRepo{}, time.Date(2026, 11, 2, 3, 30, 0, 0, time.UTC))

	due, err := svc.DueReminders(context.Background(), domain.DueHolidayRemindersRequest{Today: "2026-11-02"})
	if err != nil {
		t.Fatal(err)
	}
	if len(due.Reminders) != 1 || due.Reminders[0].Stage != domain.HolidayReminderNotice ||
		due.Reminders[0].DaysBefore != 14 || len(due.Reminders[0].Recipients) != 1 {
		t.Fatalf("got %+v, want the 14-day notice to the lead", due.Reminders)
	}
	if err := svc.RecordReminder(context.Background(), "atlas", "2026-11-16", domain.HolidayReminderNotice); err != nil {
		t.Fatal(err)
	}
	again, _ := svc.DueReminders(context.Background(), domain.DueHolidayRemindersRequest{Today: "2026-11-03"})
	if len(again.Reminders) != 0 {
		t.Fatalf("the notice went twice: %+v", again.Reminders)
	}
	week, _ := svc.DueReminders(context.Background(), domain.DueHolidayRemindersRequest{Today: "2026-11-09"})
	if len(week.Reminders) != 1 || week.Reminders[0].Stage != domain.HolidayReminderWeek {
		t.Fatalf("got %+v, want the week reminder", week.Reminders)
	}

	// Fully marked: no follow-up.
	repo.turns[0].Away = false
	if _, err := svc.Confirm(asUser(leadEmail), "atlas", "2026-11-16"); err != nil {
		t.Fatal(err)
	}
	done, _ := svc.DueReminders(context.Background(), domain.DueHolidayRemindersRequest{Today: "2026-11-13"})
	if len(done.Reminders) != 0 {
		t.Fatalf("a fully marked team was reminded: %+v", done.Reminders)
	}
	if err := svc.RecordReminder(context.Background(), "atlas", "2026-11-16", "monthly"); err == nil {
		t.Error("an unknown stage was recorded")
	}
}
