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

package service_test

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

// Rule-level tests for the per-date attribution the weekly report applies
// and the allocation, severity and member rules the two cadences share.
// They run on a fake repository, so they need no database. The week is
// Monday 2026-10-26 to Sunday 2026-11-01.

type fakeWorklogRepo struct {
	roster []repository.WorklogRosterRow
	times  []repository.WorklogTimeRow
	leave  []repository.WorklogLeaveRow
	moves  []repository.WorklogMoveRow
	// crew is the weekend crew and turns the rotation-turn holders, both
	// whatever the range.
	crew  []string
	turns []string
}

// MoveSpans returns the fake's move spans, whatever the range.
func (f *fakeWorklogRepo) MoveSpans(context.Context, time.Time, time.Time) ([]repository.WorklogMoveRow, error) {
	return f.moves, nil
}

func (f *fakeWorklogRepo) Roster(context.Context, []string, []string) ([]repository.WorklogRosterRow, error) {
	return f.roster, nil
}
func (f *fakeWorklogRepo) TimeRows(context.Context, time.Time, time.Time) ([]repository.WorklogTimeRow, error) {
	return f.times, nil
}
func (f *fakeWorklogRepo) Leave(context.Context, time.Time, time.Time) ([]repository.WorklogLeaveRow, error) {
	return f.leave, nil
}
func (f *fakeWorklogRepo) RosteredUserIDs(context.Context, time.Time, time.Time) ([]string, error) {
	return nil, nil
}
func (f *fakeWorklogRepo) WeekendCrewUserIDs(context.Context, time.Time, time.Time) ([]string, error) {
	return f.crew, nil
}
func (f *fakeWorklogRepo) RotationTurnUserIDs(context.Context, time.Time, time.Time) ([]string, error) {
	return f.turns, nil
}
func (f *fakeWorklogRepo) TeamContacts(context.Context, []string, []string) ([]repository.WorklogTeamContacts, error) {
	return nil, nil
}

func wd(s string) time.Time {
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return d
}

func person(id, team string) repository.WorklogRosterRow {
	return repository.WorklogRosterRow{TeamKey: team, TeamName: team, TeamType: "cre-abt",
		UserID: id, Email: id + "@example.com", Name: "Jane Doe " + id, Role: "engineer"}
}

func card(user, item, date, state, severity string, minutes int) repository.WorklogTimeRow {
	return repository.WorklogTimeRow{UserID: user, ItemID: item, IssueKey: item, ItemType: "CASE",
		WorkDate: wd(date), State: state, Severity: severity, Minutes: minutes, Entries: 1}
}

func absence(user, code, bucket, from, to string) repository.WorklogLeaveRow {
	return repository.WorklogLeaveRow{UserID: user, ShortCode: code, Bucket: bucket,
		StartsOn: wd(from), EndsOn: wd(to)}
}

func build(t *testing.T, repo *fakeWorklogRepo, cadence domain.WorklogReportCadence, from, to string) domain.WorklogReport {
	t.Helper()
	svc := service.NewWorklogReportService(repo, nil, nil)
	r, err := svc.BuildReport(context.Background(),
		domain.WorklogReportRequest{Cadence: cadence, From: wd(from), To: wd(to)})
	if err != nil {
		t.Fatalf("BuildReport: %v", err)
	}
	return r
}

func dev(r domain.WorklogReport, id string) (domain.WorklogDeveloper, bool) {
	for _, tr := range r.Teams {
		for _, d := range tr.Developers {
			if d.Email == id+"@example.com" {
				return d, true
			}
		}
	}
	return domain.WorklogDeveloper{}, false
}

func TestWorklogWeeklyCountsWeekendCards(t *testing.T) {
	// Weekend time counts on the weekly, whoever logged it: nobody here is
	// on a weekend rota (the fake has no crew). Worked Days stays weekdays.
	// u2 is on leave every weekday and logs only on the Sunday, and is
	// still listed with that time.
	repo := &fakeWorklogRepo{
		roster: []repository.WorklogRosterRow{person("u1", "alpha"), person("u2", "alpha")},
		leave:  []repository.WorklogLeaveRow{absence("u2", "AL/CL", "LEAVE", "2026-10-26", "2026-10-30")},
		times: []repository.WorklogTimeRow{
			card("u1", "A-1", "2026-10-27", "SUBMITTED", "S4", 60),
			card("u1", "A-2", "2026-10-31", "SUBMITTED", "S4", 120), // Saturday
			card("u1", "A-3", "2026-11-01", "APPROVED", "S4", 180),  // Sunday
			card("u2", "A-4", "2026-11-01", "SUBMITTED", "S4", 90),  // Sunday
		},
	}
	r := build(t, repo, domain.WorklogReportCadenceWeekly, "2026-10-26", "2026-11-01")
	d, _ := dev(r, "u1")
	if d.TotalHours != 6 || d.PendingIssues != 2 || d.ApprovedIssues != 1 || d.WorkedDays != 5 || d.AvgTimePerDay != 1.2 {
		t.Errorf("u1 = %+v; want 6h (weekend included), 2 pending + 1 approved issues, 5 worked days, 1.2 h/day", d)
	}
	if d, ok := dev(r, "u2"); !ok || d.TotalHours != 1.5 || d.WorkedDays != 0 || d.AvgTimePerDay != 0 {
		t.Errorf("u2 = %+v ok=%v; want listed with 1.5h, 0 worked days, 0 h/day", d, ok)
	}
	if r.Totals.TotalHours != 7.5 || r.Totals.Issues != 4 {
		t.Errorf("totals hours=%v issues=%d, want 7.5 and 4", r.Totals.TotalHours, r.Totals.Issues)
	}
}

func TestWorklogWeeklyCountsTimeLoggedOnAbsentDays(t *testing.T) {
	// One card per weekday, 60 minutes each. Each absence covers Tuesday
	// and Wednesday. A deliberate departure from the previous system: the
	// cards on the absent days are STILL counted (5h, 5 issues), while the
	// absence still reduces Worked Days to 3.
	cards := func(u string) []repository.WorklogTimeRow {
		var out []repository.WorklogTimeRow
		for i, day := range []string{"2026-10-26", "2026-10-27", "2026-10-28", "2026-10-29", "2026-10-30"} {
			out = append(out, card(u, u+"-"+string(rune('a'+i)), day, "SUBMITTED", "S4", 60))
		}
		return out
	}
	for _, tc := range []struct {
		name string
		row  repository.WorklogLeaveRow
	}{
		{"annual leave", absence("u1", "AL/CL", "LEAVE", "2026-10-27", "2026-10-28")},
		// A half day removes the WHOLE day on the weekly.
		{"half day morning", absence("u1", "HD-AM", "LEAVE", "2026-10-27", "2026-10-28")},
		{"half day evening", absence("u1", "HD-PM", "LEAVE", "2026-10-27", "2026-10-28")},
		{"exclusion", absence("u1", "EXC", "EXCLUDED", "2026-10-27", "2026-10-28")},
		{"allocation", absence("u1", "ENG_ALLOC", "ALLOCATION", "2026-10-27", "2026-10-28")},
		// Open-ended and starting before the range.
		{"open ended", repository.WorklogLeaveRow{UserID: "u1", ShortCode: "EXC", Bucket: "EXCLUDED",
			StartsOn: wd("2026-10-01")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeWorklogRepo{
				roster: []repository.WorklogRosterRow{person("u1", "alpha")},
				times:  cards("u1"),
				leave:  []repository.WorklogLeaveRow{tc.row},
			}
			r := build(t, repo, domain.WorklogReportCadenceWeekly, "2026-10-26", "2026-11-01")
			d, ok := dev(r, "u1")
			if !ok {
				t.Fatal("u1 missing: a person with counted time is listed whatever their absences")
			}
			if d.TotalHours != 5 || d.PendingIssues != 5 {
				t.Errorf("hours=%v issues=%d, want 5h: absent-day cards are counted", d.TotalHours, d.PendingIssues)
			}
			wantDays := 3.0
			if tc.name == "open ended" {
				wantDays = 0
			}
			if d.WorkedDays != wantDays {
				t.Errorf("worked days = %v, want %v", d.WorkedDays, wantDays)
			}
		})
	}
}

func TestWorklogWeeklyOverlappingAbsencesCountADayOnce(t *testing.T) {
	repo := &fakeWorklogRepo{
		roster: []repository.WorklogRosterRow{person("u1", "alpha")},
		leave: []repository.WorklogLeaveRow{
			absence("u1", "AL/CL", "LEAVE", "2026-10-27", "2026-10-28"),
			absence("u1", "ENG_ALLOC", "ALLOCATION", "2026-10-28", "2026-10-29"),
		},
	}
	r := build(t, repo, domain.WorklogReportCadenceWeekly, "2026-10-26", "2026-11-01")
	d, _ := dev(r, "u1")
	if d.WorkedDays != 2 {
		t.Errorf("worked days = %v, want 2 (Tue-Thu blocked once each)", d.WorkedDays)
	}
}

func TestWorklogWeeklyListingRule(t *testing.T) {
	repo := &fakeWorklogRepo{
		roster: []repository.WorklogRosterRow{person("u1", "alpha"), person("u2", "alpha"), person("u3", "alpha")},
		leave: []repository.WorklogLeaveRow{
			absence("u2", "AL/CL", "LEAVE", "2026-10-26", "2026-10-30"),
			absence("u3", "AL/CL", "LEAVE", "2026-10-26", "2026-10-30"),
		},
		times: []repository.WorklogTimeRow{card("u2", "B-1", "2026-10-27", "SUBMITTED", "S4", 60)},
	}
	r := build(t, repo, domain.WorklogReportCadenceWeekly, "2026-10-26", "2026-11-01")
	if _, ok := dev(r, "u1"); !ok {
		t.Error("u1 has five free weekdays and no time; the weekly still lists them")
	}
	// u2 is absent all week but logged a card: listed, time counted.
	if d, ok := dev(r, "u2"); !ok || d.TotalHours != 1 || d.WorkedDays != 0 {
		t.Errorf("u2 = %+v ok=%v, want listed with 1h and 0 worked days", d, ok)
	}
	// u3 is absent all week and logged nothing: not listed.
	if _, ok := dev(r, "u3"); ok {
		t.Error("u3 has no surviving weekday and no card; must not be listed")
	}
	if r.Totals.Members != 2 || r.Totals.TotalHours != 1 {
		t.Errorf("members=%d hours=%v, want 2 and 1", r.Totals.Members, r.Totals.TotalHours)
	}
}

func TestWorklogWeeklyAveragePeoplePerDay(t *testing.T) {
	// u1 has five available weekdays, u2 three: eight member-days over a
	// span of six (the gap between Monday and Sunday), not over five or 7.
	repo := &fakeWorklogRepo{
		roster: []repository.WorklogRosterRow{person("u1", "alpha"), person("u2", "alpha")},
		leave:  []repository.WorklogLeaveRow{absence("u2", "AL/CL", "LEAVE", "2026-10-28", "2026-10-29")},
	}
	r := build(t, repo, domain.WorklogReportCadenceWeekly, "2026-10-26", "2026-11-01")
	if r.Totals.AveragePeoplePerDay != 1.33 {
		t.Errorf("average people per day = %v, want 1.33 (8 member-days over 6)", r.Totals.AveragePeoplePerDay)
	}
}

func TestWorklogIncidentQuerySplitBySeverity(t *testing.T) {
	repo := &fakeWorklogRepo{
		roster: []repository.WorklogRosterRow{person("u1", "alpha")},
		times: []repository.WorklogTimeRow{
			card("u1", "C-0", "2026-10-26", "SUBMITTED", "S0", 60),
			card("u1", "C-1", "2026-10-26", "SUBMITTED", "S1", 60),
			card("u1", "C-2", "2026-10-27", "SUBMITTED", "S2", 60),
			card("u1", "C-3", "2026-10-27", "APPROVED", "S3", 60),
			card("u1", "C-4", "2026-10-28", "SUBMITTED", "S4", 120),
			card("u1", "C-N", "2026-10-28", "SUBMITTED", "", 30), // no severity stays a Query
			{UserID: "u1", ItemID: "E-1", IssueKey: "E-1", ItemType: "ENGAGEMENT",
				WorkDate: wd("2026-10-29"), State: "SUBMITTED", Minutes: 60, Entries: 1},
		},
	}
	for _, cadence := range []domain.WorklogReportCadence{domain.WorklogReportCadenceWeekly, domain.WorklogReportCadenceDaily} {
		from, to := "2026-10-26", "2026-11-01"
		if cadence == domain.WorklogReportCadenceDaily {
			// The daily is one date; put every card on it.
			from, to = "2026-10-28", "2026-10-28"
			repo2 := *repo
			repo2.times = nil
			for _, c := range repo.times {
				c.WorkDate = wd("2026-10-28")
				repo2.times = append(repo2.times, c)
			}
			r := build(t, &repo2, cadence, from, to)
			if r.Totals.IncidentIssues != 4 || r.Totals.QueryIssues != 2 {
				t.Errorf("daily incidents=%d queries=%d, want 4 and 2", r.Totals.IncidentIssues, r.Totals.QueryIssues)
			}
			continue
		}
		r := build(t, repo, cadence, from, to)
		got := map[string]domain.WorklogTypeBucket{}
		for _, b := range r.ByType {
			got[b.Type] = b
		}
		if b := got["INCIDENT"]; b.Issues != 4 || b.TotalHours != 4 || b.Label != "Incident" {
			t.Errorf("incident bucket = %+v, want 4 case-days over 4h", b)
		}
		if b := got["CASE"]; b.Issues != 2 || b.TotalHours != 2.5 || b.Label != "Query" {
			t.Errorf("query bucket = %+v, want 2 case-days over 2.5h", b)
		}
		if b := got["ENGAGEMENT"]; b.Issues != 1 {
			t.Errorf("engagement bucket = %+v, want it untouched", b)
		}
		if r.Totals.IncidentIssues != 4 || r.Totals.QueryIssues != 2 {
			t.Errorf("incidents=%d queries=%d, want 4 and 2", r.Totals.IncidentIssues, r.Totals.QueryIssues)
		}
	}
}

func TestWorklogDailyHalfDayStillPresentAndAllocationExcludes(t *testing.T) {
	// The daily is NOT changed by the weekly's per-date rules: a half day
	// is 0.5 of a present member whose hours count in full; an allocation
	// removes the person from the report altogether.
	repo := &fakeWorklogRepo{
		roster: []repository.WorklogRosterRow{person("u1", "alpha"), person("u2", "alpha"), person("u3", "alpha")},
		leave: []repository.WorklogLeaveRow{
			absence("u1", "HD-AM", "LEAVE", "2026-10-28", "2026-10-28"),
			absence("u2", "ENG_ALLOC", "ALLOCATION", "2026-10-20", "2026-11-20"),
		},
		times: []repository.WorklogTimeRow{
			card("u1", "D-1", "2026-10-28", "SUBMITTED", "S4", 60),
			card("u2", "D-2", "2026-10-28", "SUBMITTED", "S4", 60),
		},
	}
	r := build(t, repo, domain.WorklogReportCadenceDaily, "2026-10-28", "2026-10-28")
	if d, ok := dev(r, "u1"); !ok || d.TotalHours != 1 || d.Leave != "HD" {
		t.Errorf("u1 = %+v ok=%v, want listed with 1h and Leave HD", d, ok)
	}
	if _, ok := dev(r, "u2"); ok {
		t.Error("u2 is on a confirmed allocation and must not appear on the daily")
	}
	if r.Totals.Members != 2 || r.Totals.NonLeaveMembers != 1.5 {
		t.Errorf("members=%d nonLeave=%v, want 2 and 1.5 (u1 half, u3 whole)", r.Totals.Members, r.Totals.NonLeaveMembers)
	}
}

func TestWorklogWeeklyLogsTimeOfPeopleOnNoTeam(t *testing.T) {
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	defer slog.SetDefault(old)

	repo := &fakeWorklogRepo{
		roster: []repository.WorklogRosterRow{person("u1", "alpha")},
		times: []repository.WorklogTimeRow{
			card("u1", "F-1", "2026-10-27", "SUBMITTED", "S4", 60),
			card("ghost1", "F-2", "2026-10-27", "SUBMITTED", "S4", 90),
			card("ghost1", "F-3", "2026-10-28", "APPROVED", "S4", 30),
			card("ghost2", "F-4", "2026-10-28", "SUBMITTED", "S4", 60),
			card("ghost2", "F-5", "2026-10-28", "REJECTED", "S4", 600), // not report time
		},
	}
	r := build(t, repo, domain.WorklogReportCadenceWeekly, "2026-10-26", "2026-11-01")
	if r.Totals.TotalHours != 1 {
		t.Errorf("total hours = %v, want only u1's 1h", r.Totals.TotalHours)
	}
	out := buf.String()
	if !strings.Contains(out, "people=2") || !strings.Contains(out, "hours=3") {
		t.Errorf("expected a warning naming 2 people and 3 hours, got %q", out)
	}
}

func TestWorklogWeeklyAveragesShareOneDivisor(t *testing.T) {
	// A team of three: approved 6.75h and pending 5.25h. Both averages
	// divide by the 3 members: 2.25 approved and 4.00 total. (The previous
	// system prints 0.00 for approved and divides by members less one.)
	repo := &fakeWorklogRepo{
		roster: []repository.WorklogRosterRow{person("u1", "alpha"), person("u2", "alpha"), person("u3", "alpha"), person("s1", "solo"), person("e1", "empty")},
		times: []repository.WorklogTimeRow{
			card("u1", "G-1", "2026-10-27", "APPROVED", "S4", 405),  // 6.75h
			card("u2", "G-2", "2026-10-27", "SUBMITTED", "S4", 315), // 5.25h
			card("s1", "G-3", "2026-10-27", "APPROVED", "S4", 90),   // 1.5h
		},
		leave: []repository.WorklogLeaveRow{absence("e1", "AL/CL", "LEAVE", "2026-10-26", "2026-10-30")},
	}
	r := build(t, repo, domain.WorklogReportCadenceWeekly, "2026-10-26", "2026-11-01")
	byTeam := map[string]domain.WorklogTeamReport{}
	for _, tr := range r.Teams {
		byTeam[tr.TeamKey] = tr
	}
	a := byTeam["alpha"]
	if a.Members != 3 || a.MembersForAverage != 3 {
		t.Errorf("alpha members=%d forAvg=%v, want 3 and 3", a.Members, a.MembersForAverage)
	}
	if a.AvgApprovedHoursPerPerson != 2.25 || a.AvgTotalHoursPerPerson != 4 {
		t.Errorf("alpha avg approved=%v total=%v, want 2.25 and 4.00", a.AvgApprovedHoursPerPerson, a.AvgTotalHoursPerPerson)
	}
	s := byTeam["solo"]
	if s.MembersForAverage != 1 || s.AvgApprovedHoursPerPerson != 1.5 || s.AvgTotalHoursPerPerson != 1.5 {
		t.Errorf("solo = %+v, want a one-member team to divide by 1", s)
	}
	// The header shares the formula: total hours over members.
	if r.Totals.Members != 4 || r.Totals.AvgHoursPerPerson != 3.38 {
		t.Errorf("header members=%d avg=%v, want 4 and 3.38 (13.5h / 4)", r.Totals.Members, r.Totals.AvgHoursPerPerson)
	}
	// A team that does not appear at all (nobody counts) is simply absent,
	// never a NaN row.
	if _, ok := byTeam["empty"]; ok {
		t.Error("a team whose only member is absent all week and logged nothing must not appear")
	}
}

func TestWorklogDailyAveragesShareOnePresenceDivisor(t *testing.T) {
	// Wednesday: u1 present 1.0 (approved 60), u2 half day 0.5 (pending
	// 60), u3 on full leave 0 (a card that does not count). Presence 1.5;
	// approved 1.00h and total 2.00h over it.
	repo := &fakeWorklogRepo{
		roster: []repository.WorklogRosterRow{person("u1", "alpha"), person("u2", "alpha"), person("u3", "alpha")},
		leave: []repository.WorklogLeaveRow{
			absence("u2", "HD-AM", "LEAVE", "2026-10-28", "2026-10-28"),
			absence("u3", "AL/CL", "LEAVE", "2026-10-28", "2026-10-28"),
		},
		times: []repository.WorklogTimeRow{
			card("u1", "H-1", "2026-10-28", "APPROVED", "S4", 60),
			card("u2", "H-2", "2026-10-28", "SUBMITTED", "S4", 60),
			card("u3", "H-3", "2026-10-28", "SUBMITTED", "S4", 600),
		},
	}
	r := build(t, repo, domain.WorklogReportCadenceDaily, "2026-10-28", "2026-10-28")
	a := r.Teams[0]
	if a.MembersForAverage != 1.5 {
		t.Errorf("forAvg = %v, want 1.5 (present + half)", a.MembersForAverage)
	}
	if a.AvgApprovedHoursPerPerson != 0.67 || a.AvgTotalHoursPerPerson != 1.33 {
		t.Errorf("avg approved=%v total=%v, want 0.67 and 1.33 (1h and 2h over 1.5)",
			a.AvgApprovedHoursPerPerson, a.AvgTotalHoursPerPerson)
	}
}

// byTypeSum adds up the type breakdown.
func byTypeSum(r domain.WorklogReport) (hours float64, issues int) {
	for _, b := range r.ByType {
		hours += b.TotalHours
		issues += b.Issues
	}
	return hours, issues
}

func TestWorklogDailyByTypeCoversOnlyIncludedPeople(t *testing.T) {
	// u2 is on a confirmed allocation and so is not on the daily; their
	// cards must not reach the type breakdown either.
	repo := &fakeWorklogRepo{
		roster: []repository.WorklogRosterRow{person("u1", "alpha"), person("u2", "alpha")},
		leave:  []repository.WorklogLeaveRow{absence("u2", "ENG_ALLOC", "ALLOCATION", "2026-10-20", "2026-11-20")},
		times: []repository.WorklogTimeRow{
			card("u1", "K-1", "2026-10-28", "SUBMITTED", "S4", 60),
			card("u1", "K-2", "2026-10-28", "APPROVED", "S1", 120),
			card("u2", "K-3", "2026-10-28", "SUBMITTED", "S4", 300),
			card("u2", "K-4", "2026-10-28", "SUBMITTED", "S2", 240),
		},
	}
	r := build(t, repo, domain.WorklogReportCadenceDaily, "2026-10-28", "2026-10-28")
	hours, issues := byTypeSum(r)
	if r.Totals.TotalHours != 3 || hours != 3 {
		t.Errorf("total hours = %v, byType hours = %v, want both 3 (u2 is excluded)", r.Totals.TotalHours, hours)
	}
	if r.Totals.Issues != 2 || issues != 2 {
		t.Errorf("total issues = %d, byType issues = %d, want both 2", r.Totals.Issues, issues)
	}
}

func TestWorklogWeeklyByTypeEqualsTotal(t *testing.T) {
	// A weekend card, a person with no team, an absent-all-week person who
	// logged nothing, and ordinary listed people: the breakdown must add up
	// to the total exactly as the listed members do.
	repo := &fakeWorklogRepo{
		roster: []repository.WorklogRosterRow{person("u1", "alpha"), person("u2", "beta"), person("u3", "beta")},
		leave:  []repository.WorklogLeaveRow{absence("u3", "AL/CL", "LEAVE", "2026-10-26", "2026-10-30")},
		times: []repository.WorklogTimeRow{
			card("u1", "M-1", "2026-10-27", "SUBMITTED", "S4", 75),
			card("u1", "M-2", "2026-10-28", "APPROVED", "S0", 45),
			card("u2", "M-3", "2026-10-29", "SUBMITTED", "S3", 90),
			card("u1", "M-4", "2026-10-31", "SUBMITTED", "S4", 600), // weekend
			card("ghost", "M-5", "2026-10-27", "SUBMITTED", "S4", 500),
		},
	}
	r := build(t, repo, domain.WorklogReportCadenceWeekly, "2026-10-26", "2026-11-01")
	hours, issues := byTypeSum(r)
	if d := hours - r.Totals.TotalHours; d > 0.01 || d < -0.01 {
		t.Errorf("byType hours = %v, total = %v, want them equal", hours, r.Totals.TotalHours)
	}
	if issues != r.Totals.Issues {
		t.Errorf("byType issues = %d, total issues = %d, want them equal (one day per item)", issues, r.Totals.Issues)
	}
}

// MOVED MEMBERS. A Team Schedule move keeps the person a member of their
// own team, so the report files them under the team the move names.

// move builds a move span onto team (an empty to is open-ended).
func move(user, team, teamType, from, to string) repository.WorklogMoveRow {
	m := repository.WorklogMoveRow{UserID: user, TeamKey: team, TeamName: team, TeamType: teamType,
		StartsOn: wd(from)}
	if to != "" {
		m.EndsOn = wd(to)
	}
	return m
}

// teamOf returns the one team section a developer is listed under, and fails
// the test when they are under none or more than one.
func teamOf(t *testing.T, r domain.WorklogReport, id string) string {
	t.Helper()
	var found []string
	for _, tr := range r.Teams {
		for _, d := range tr.Developers {
			if d.Email == id+"@example.com" {
				found = append(found, tr.TeamKey)
			}
		}
	}
	if len(found) != 1 {
		t.Fatalf("%s is under %v, want exactly one team", id, found)
	}
	return found[0]
}

// The daily report lists a member under the team a move covering its day
// names, with their hours; somebody with no move stays where they are.
func TestWorklogDailyFilesMovedMemberUnderTheMoveTeam(t *testing.T) {
	repo := &fakeWorklogRepo{
		roster: []repository.WorklogRosterRow{person("u1", "vega"), person("u2", "vega")},
		times:  []repository.WorklogTimeRow{card("u1", "A-1", "2026-10-28", "APPROVED", "S4", 60)},
		moves:  []repository.WorklogMoveRow{move("u1", "americas", "cre-abt", "2026-10-26", "2026-11-06")},
	}
	r := build(t, repo, domain.WorklogReportCadenceDaily, "2026-10-28", "2026-10-28")
	if got := teamOf(t, r, "u1"); got != "americas" {
		t.Errorf("moved member under %q, want americas", got)
	}
	if got := teamOf(t, r, "u2"); got != "vega" {
		t.Errorf("member with no move under %q, want vega", got)
	}
	if d, _ := dev(r, "u1"); d.TotalHours != 1 {
		t.Errorf("moved member's hours = %v, want 1 (their time moves with them)", d.TotalHours)
	}
}

// A move that has not started by the report day leaves the home team.
func TestWorklogDailyKeepsHomeTeamOutsideTheMoveSpan(t *testing.T) {
	repo := &fakeWorklogRepo{
		roster: []repository.WorklogRosterRow{person("u1", "vega")},
		moves:  []repository.WorklogMoveRow{move("u1", "americas", "cre-abt", "2026-10-29", "")},
	}
	r := build(t, repo, domain.WorklogReportCadenceDaily, "2026-10-28", "2026-10-28")
	if got := teamOf(t, r, "u1"); got != "vega" {
		t.Errorf("span starting the next day moved them to %q, want vega", got)
	}
}

// The weekly report moves a member only when the move covers more than half
// of the week's weekdays: 3 of 5 moves them, 2 of 5 does not.
func TestWorklogWeeklyMovesOnAMajorityOfWeekdays(t *testing.T) {
	repo := &fakeWorklogRepo{
		roster: []repository.WorklogRosterRow{person("u1", "vega"), person("u2", "vega")},
		moves: []repository.WorklogMoveRow{
			move("u1", "migrations", "cre", "2026-10-28", ""),           // Wed-Fri: 3 of 5
			move("u2", "migrations", "cre", "2026-10-29", "2026-11-01"), // Thu-Fri: 2 of 5
		},
	}
	r := build(t, repo, domain.WorklogReportCadenceWeekly, "2026-10-26", "2026-11-01")
	if got := teamOf(t, r, "u1"); got != "migrations" {
		t.Errorf("3 of 5 weekdays moved: under %q, want migrations", got)
	}
	if got := teamOf(t, r, "u2"); got != "vega" {
		t.Errorf("2 of 5 weekdays moved: under %q, want vega", got)
	}
}

// A move to a team of another type, or to an excluded team, is ignored.
func TestWorklogIgnoresMoveToTeamOutsideTheScope(t *testing.T) {
	repo := &fakeWorklogRepo{
		roster: []repository.WorklogRosterRow{person("u1", "vega"), person("u2", "vega")},
		moves: []repository.WorklogMoveRow{
			move("u1", "saas_night", "sre", "2026-10-26", ""),
			move("u2", "hidden", "cre-abt", "2026-10-26", ""),
		},
	}
	svc := service.NewWorklogReportService(repo, []string{"cre-abt", "cre"}, []string{"hidden"})
	r, err := svc.BuildReport(context.Background(), domain.WorklogReportRequest{
		Cadence: domain.WorklogReportCadenceDaily, From: wd("2026-10-28"), To: wd("2026-10-28")})
	if err != nil {
		t.Fatalf("BuildReport: %v", err)
	}
	if got := teamOf(t, r, "u1"); got != "vega" {
		t.Errorf("move to an SRE team: under %q, want vega", got)
	}
	if got := teamOf(t, r, "u2"); got != "vega" {
		t.Errorf("move to an excluded team: under %q, want vega", got)
	}
}

// A move span never adds somebody who is not on the roster.
func TestWorklogMoveNeverAddsSomebodyOffTheRoster(t *testing.T) {
	repo := &fakeWorklogRepo{
		roster: []repository.WorklogRosterRow{person("u1", "vega")},
		moves:  []repository.WorklogMoveRow{move("u9", "americas", "cre-abt", "2026-10-26", "")},
	}
	r := build(t, repo, domain.WorklogReportCadenceDaily, "2026-10-28", "2026-10-28")
	if _, ok := dev(r, "u9"); ok {
		t.Error("a move span added somebody who is not on the roster")
	}
}

func TestWorklogDailyKeepsAllocatedPersonOnARotationTurn(t *testing.T) {
	// An allocation drops a person from the daily, but not on a day they
	// hold a rotation turn: the 6-9 morning or evening is rota work, and
	// its time must show. An exclusion from the rota still drops them.
	repo := &fakeWorklogRepo{
		roster: []repository.WorklogRosterRow{
			person("u1", "alpha"), person("u2", "alpha"), person("u3", "alpha"), person("u4", "alpha"),
		},
		leave: []repository.WorklogLeaveRow{
			absence("u1", "ALLO_EXT", "ALLOCATION", "2026-10-20", "2026-11-20"),
			absence("u2", "ALLO_INT", "ALLOCATION", "2026-10-20", "2026-11-20"),
			absence("u3", "EXCLUDED", "EXCLUDED", "2026-10-28", "2026-10-28"),
		},
		turns: []string{"u1", "u3"},
		times: []repository.WorklogTimeRow{
			card("u1", "D-1", "2026-10-28", "SUBMITTED", "S4", 180),
			card("u2", "D-2", "2026-10-28", "SUBMITTED", "S4", 60),
			card("u3", "D-3", "2026-10-28", "SUBMITTED", "S4", 60),
		},
	}
	r := build(t, repo, domain.WorklogReportCadenceDaily, "2026-10-28", "2026-10-28")
	if d, ok := dev(r, "u1"); !ok || d.TotalHours != 3 || d.Leave != "" {
		t.Errorf("u1 = %+v ok=%v, want listed with 3h: allocated but on a rotation turn", d, ok)
	}
	if _, ok := dev(r, "u2"); ok {
		t.Error("u2 is allocated with no turn that day and must not appear")
	}
	if _, ok := dev(r, "u3"); ok {
		t.Error("u3 is excluded from the rota; a turn must not bring them back")
	}
	if r.Totals.Members != 2 || r.Totals.TotalHours != 3 || r.Totals.NonLeaveMembers != 2 {
		t.Errorf("members=%d total=%v nonLeave=%v, want 2, 3 and 2 (u1 and u4)",
			r.Totals.Members, r.Totals.TotalHours, r.Totals.NonLeaveMembers)
	}
}

func TestWorklogWeekendDailyListsCrewNotLeaveAlone(t *testing.T) {
	// Saturday 2026-10-31. The weekend daily lists the weekend crew
	// (allocated crew included, on their turn), plus anyone who logged
	// time. Somebody on leave who is not on the weekend rota is not listed.
	repo := &fakeWorklogRepo{
		roster: []repository.WorklogRosterRow{
			person("crewAlloc", "alpha"), person("crewIdle", "alpha"), person("crewLeave", "alpha"),
			person("offLeave", "beta"), person("offLogged", "beta"), person("offIdle", "beta"),
		},
		leave: []repository.WorklogLeaveRow{
			absence("crewAlloc", "ALLO_INT", "ALLOCATION", "2026-10-31", "2026-11-30"),
			absence("crewLeave", "AL", "LEAVE", "2026-10-31", "2026-10-31"),
			absence("offLeave", "ML", "LEAVE", "2026-10-01", "2026-12-31"),
		},
		crew:  []string{"crewAlloc", "crewIdle", "crewLeave"},
		turns: []string{"crewAlloc", "crewIdle", "crewLeave"},
		times: []repository.WorklogTimeRow{
			card("crewAlloc", "W-1", "2026-10-31", "SUBMITTED", "S4", 540),
			card("offLogged", "W-2", "2026-10-31", "APPROVED", "S4", 60),
		},
	}
	r := build(t, repo, domain.WorklogReportCadenceDaily, "2026-10-31", "2026-10-31")
	if d, ok := dev(r, "crewAlloc"); !ok || d.TotalHours != 9 {
		t.Errorf("crewAlloc = %+v ok=%v, want listed with 9h: allocated, but on the weekend turn", d, ok)
	}
	if _, ok := dev(r, "crewIdle"); !ok {
		t.Error("crewIdle is on the weekend rota and must be listed, logged time or not")
	}
	if d, ok := dev(r, "crewLeave"); !ok || d.Leave != "FD" {
		t.Errorf("crewLeave = %+v ok=%v, want listed and marked FD", d, ok)
	}
	if _, ok := dev(r, "offLeave"); ok {
		t.Error("offLeave is on leave and not on the weekend rota; leave alone must not list them")
	}
	if _, ok := dev(r, "offLogged"); !ok {
		t.Error("offLogged logged time on the Saturday and must be listed")
	}
	if _, ok := dev(r, "offIdle"); ok {
		t.Error("offIdle is not on the weekend rota and logged nothing")
	}
	if r.Totals.Members != 4 || r.Totals.OnLeaveMembers != 1 {
		t.Errorf("members=%d onLeave=%d, want 4 and 1", r.Totals.Members, r.Totals.OnLeaveMembers)
	}
}
