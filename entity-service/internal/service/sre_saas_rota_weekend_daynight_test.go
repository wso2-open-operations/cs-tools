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
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// The weekend Day/Night shape (migration 0306): the weekday windows as
// before, SRE_WE_TZ1 retired, and the two weekend windows in its place.
var saasTestWindowsDayNight = func() map[string][2]int {
	out := map[string][2]int{}
	for code, w := range saasTestWindows {
		if code != saasShiftWeekend {
			out[code] = w
		}
	}
	out[saasShiftWeekendDay] = [2]int{360, 1080}
	out[saasShiftWeekendNight] = [2]int{1080, 1800}
	return out
}()

func saasTestInputDayNight(month time.Time) saasRotaInput {
	in := saasTestInput(month)
	in.Windows = saasTestWindowsDayNight
	in.WeekendDayNight = true
	return in
}

// saasAsTZ is a Day/Night plan written back on the TZ1+2 / TZ3 codes, so it
// can be compared with a plan generated that way.
func saasAsTZ(plan saasRotaPlan) saasRotaPlan {
	out := saasRotaPlan{Lieu: plan.Lieu}
	for _, t := range plan.Turns {
		switch t.ShiftCode {
		case saasShiftWeekendDay:
			t.ShiftCode = saasShiftWeekend
		case saasShiftWeekendNight:
			t.ShiftCode = saasShiftTZ3
		}
		out.Turns = append(out.Turns, t)
	}
	for _, w := range plan.Warnings {
		w.Message = strings.NewReplacer(" Day ", " TZ1+2 ", "Night:", "TZ3 weekend:", "Night L", "TZ3 weekend L").Replace(w.Message)
		out.Warnings = append(out.Warnings, w)
	}
	return out
}

// The leads' default (spec "Crews and tiers"): the weekend keeps today's
// crews and only their hours change. So a Day/Night month must hold exactly
// the people the TZ1+2 / TZ3 month does, slot for slot, weekdays untouched.
func TestSRESaaSRota_DayNightWeekendsHoldTheSamePeople(t *testing.T) {
	crew := map[string][]string{"apollo": {"apollo-12", "apollo-13"}, "artemis": {"artemis-11", "artemis-12"}}
	for _, month := range []time.Time{
		saasMonth(2026, time.November), saasMonth(2026, time.December), saasMonth(2027, time.January), saasMonth(2027, time.February),
	} {
		for _, withCrew := range []bool{false, true} {
			tz, dn := saasTestInput(month), saasTestInputDayNight(month)
			if withCrew {
				tz.NightCrew, dn.NightCrew = crew, crew
			}
			want := generateSRESaaSRota(tz)
			got := saasAsTZ(generateSRESaaSRota(dn))
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("%s crew=%v: the Day/Night month is not the TZ1+2 / TZ3 month on other codes\n got %d turns %d lieu %v\nwant %d turns %d lieu %v",
					month.Format("2006-01"), withCrew, len(got.Turns), len(got.Lieu), got.Warnings, len(want.Turns), len(want.Lieu), want.Warnings)
			}
		}
	}
}

// A Day/Night weekend is written only on the two weekend windows: no
// SRE_WE_TZ1 at all, no SRE_TZ3 on a Saturday or Sunday, and SRE_WE_NIGHT on
// no weekday. Friday night stays a weekday TZ3.
func TestSRESaaSRota_DayNightWritesOnlyTheWeekendWindowsAtTheWeekend(t *testing.T) {
	month := saasMonth(2026, time.November)
	plan := generateSRESaaSRota(saasTestInputDayNight(month))
	for _, x := range plan.Turns {
		weekend := saasDow(saasMustParse(x.RotaDate)) >= 5
		switch {
		case x.ShiftCode == saasShiftWeekend:
			t.Fatalf("a Day/Night month wrote %s: %+v", saasShiftWeekend, x)
		case weekend && x.ShiftCode == saasShiftTZ3:
			t.Fatalf("a weekend night on TZ3 instead of Night: %+v", x)
		case !weekend && (x.ShiftCode == saasShiftWeekendDay || x.ShiftCode == saasShiftWeekendNight):
			t.Fatalf("a weekday turn on a weekend window: %+v", x)
		}
	}
	for _, d := range saasDaysOf(month) {
		iso := saasISO(d)
		nightCode := saasShiftTZ3
		if saasDow(d) >= 5 {
			nightCode = saasShiftWeekendNight
		}
		for _, tier := range []string{saasTierL1, saasTierL2, saasTierL3} {
			got := saasTurnsBy(plan, func(x saasPlannedTurn) bool {
				return x.RotaDate == iso && x.ShiftCode == nightCode && x.Tier == tier
			})
			if len(got) != 1 {
				t.Fatalf("%s %s %s: want exactly one for the rota, got %v", iso, nightCode, tier, got)
			}
		}
		if saasDow(d) < 5 {
			continue
		}
		for _, team := range []string{"apollo", "artemis"} {
			for _, tier := range []string{saasTierL1, saasTierL2, saasTierL3} {
				got := saasTurnsBy(plan, func(x saasPlannedTurn) bool {
					return x.RotaDate == iso && x.TeamKey == team && x.ShiftCode == saasShiftWeekendDay && x.Tier == tier
				})
				if len(got) != 1 {
					t.Fatalf("%s %s Day %s: want exactly one, got %v", iso, team, tier, got)
				}
			}
		}
	}
	if len(plan.Warnings) != 0 {
		t.Fatalf("no slot should be empty: %v", plan.Warnings)
	}
}

// Nobody holds two turns at once, across midnight too: Friday's TZ3 hands
// over to Saturday's Day at 06:00, Saturday's Day to its Night at 18:00, and
// Sunday's Night to Monday's TZ1 at 06:00.
func TestSRESaaSRota_DayNightTurnsNeverOverlapAcrossDays(t *testing.T) {
	plan := generateSRESaaSRota(saasTestInputDayNight(saasMonth(2026, time.November)))
	type span struct {
		from, to int
		t        saasPlannedTurn
	}
	byUser := map[string][]span{}
	for _, x := range plan.Turns {
		if x.Tier == "" {
			continue // regular hours (SUP) sit under a turn
		}
		w := saasTestWindowsDayNight[x.ShiftCode]
		base := saasDayNumber(saasMustParse(x.RotaDate)) * 1440
		byUser[x.UserID] = append(byUser[x.UserID], span{base + w[0], base + w[1], x})
	}
	for user, spans := range byUser {
		sort.Slice(spans, func(i, j int) bool { return spans[i].from < spans[j].from })
		for i := 1; i < len(spans); i++ {
			if spans[i].from < spans[i-1].to {
				t.Fatalf("%s holds overlapping turns %+v and %+v", user, spans[i-1].t, spans[i].t)
			}
		}
	}
}

// The weekend Night is one crew for the whole rota, as TZ3 is: a tier a lead
// set by hand on any team holds that slot for every team.
func TestSaasClashesWithHand_TheWeekendNightIsOneSlotForTheRota(t *testing.T) {
	tier := saasTierL1
	byHand := []domain.ScheduleAssignment{{
		Engineer: domain.ScheduleEngineer{UserID: "artemis-03"}, TeamKey: "artemis",
		ShiftCode: saasShiftWeekendNight, Tier: &tier, RotaDate: "2026-11-07", Source: "MANUAL",
	}}
	planned := saasPlannedTurn{UserID: "apollo-05", TeamKey: "apollo", ShiftCode: saasShiftWeekendNight, Tier: saasTierL1, RotaDate: "2026-11-07"}
	if !saasClashesWithHand(planned, byHand, saasTestWindowsDayNight) {
		t.Fatal("artemis's hand-set Night L1 should hold the slot for apollo too")
	}
	planned.ShiftCode, byHand[0].ShiftCode = saasShiftWeekendDay, saasShiftWeekendDay
	if saasClashesWithHand(planned, byHand, saasTestWindowsDayNight) {
		t.Fatal("the weekend Day is per team: artemis's Day L1 is not apollo's")
	}
}

func saasCatalogueOf(windows map[string][2]int) domain.ScheduleCatalogue {
	var cat domain.ScheduleCatalogue
	for code, w := range windows {
		cat.Shifts = append(cat.Shifts, domain.ScheduleShift{Code: code, StartMinute: w[0], EndMinute: w[1]})
	}
	return cat
}

// The weekend shape is the catalogue's: Day/Night once both of its windows
// are served (switched on), TZ1+2 / TZ3 while only SRE_WE_TZ1 is, refused
// when neither shape is complete.
func TestSaasShiftWindows_TheCatalogueDecidesTheWeekendShape(t *testing.T) {
	both := map[string][2]int{}
	for c, w := range saasTestWindowsDayNight {
		both[c] = w
	}
	both[saasShiftWeekend] = saasTestWindows[saasShiftWeekend]
	onlyDay := map[string][2]int{}
	for c, w := range saasTestWindowsDayNight {
		if c != saasShiftWeekendNight {
			onlyDay[c] = w
		}
	}
	noTZ3 := map[string][2]int{}
	for c, w := range saasTestWindowsDayNight {
		if c != saasShiftTZ3 {
			noTZ3[c] = w
		}
	}
	cases := []struct {
		name     string
		windows  map[string][2]int
		dayNight bool
		refused  bool
	}{
		{"TZ1+2 / TZ3 (before the switch)", saasTestWindows, false, false},
		{"Day/Night (after the switch)", saasTestWindowsDayNight, true, false},
		{"both served", both, true, false},
		{"only one of Day and Night, and no SRE_WE_TZ1", onlyDay, false, true},
		{"no weekday TZ3", noTZ3, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, dayNight, err := saasShiftWindows(saasCatalogueOf(tc.windows))
			var unavailable *apierror.ServiceUnavailableError
			if tc.refused != errors.As(err, &unavailable) {
				t.Fatalf("refused = %v, want %v (err %v)", err != nil, tc.refused, err)
			}
			if err == nil && dayNight != tc.dayNight {
				t.Fatalf("dayNight = %v, want %v", dayNight, tc.dayNight)
			}
		})
	}
}

// After the switch a regeneration writes the weekend on the new windows and
// clears the generator's old SRE_WE_TZ1 and weekend SRE_TZ3 rows with them.
func TestGenerateMonth_AfterTheSwitchWeekendsAreDayNight(t *testing.T) {
	repo := &fakeScheduleRepo{ledTeams: []string{"apollo", "artemis"}}
	repo.assignments = saasTestNightRows(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 12, 5, 0, 0, 0, 0, time.UTC))
	writer := &fakeRotaWriter{}
	svc := newTestRotaService(repo, writer, saasTestNow)
	repo.catalogue.Shifts = saasCatalogueOf(saasTestWindowsDayNight).Shifts
	if _, err := svc.GenerateMonth(leadCtx("apollo.lead@example.com"), generateReq("2026-11")); err != nil {
		t.Fatal(err)
	}
	codes := map[string]bool{}
	for _, c := range writer.got.ShiftCodes {
		codes[c] = true
	}
	for _, c := range []string{saasShiftWeekend, saasShiftTZ3, saasShiftWeekendDay, saasShiftWeekendNight} {
		if !codes[c] {
			t.Fatalf("the replace list %v leaves out %s", writer.got.ShiftCodes, c)
		}
	}
	day, night := 0, 0
	for _, turn := range writer.got.Turns {
		weekend := saasDow(saasMustParse(turn.RotaDate)) >= 5
		if turn.ShiftCode == saasShiftWeekend || (weekend && turn.ShiftCode == saasShiftTZ3) {
			t.Fatalf("an old weekend turn was written: %+v", turn)
		}
		switch turn.ShiftCode {
		case saasShiftWeekendDay:
			day++
		case saasShiftWeekendNight:
			night++
		}
	}
	// November 2026 has 9 weekend days: Day L1-L3 for two teams, Night L1-L3 for the rota.
	if day != 9*2*3 || night != 9*3 {
		t.Fatalf("weekend Day turns %d (want %d), Night turns %d (want %d)", day, 9*2*3, night, 9*3)
	}
}

// After the switch SRE_WE_TZ1 is no longer in the catalogue, but a turn a
// lead set on it by hand can still be in the month. It is still a turn set by
// hand, which takes its person off that whole day, so the generator puts them
// on neither the Day nor the Night beside it (the lead moves the old turn).
func TestGenerateMonth_AnOldWeekendTurnSetByHandStillBlocksItsPerson(t *testing.T) {
	repo := &fakeScheduleRepo{ledTeams: []string{"apollo", "artemis"}}
	repo.assignments = saasTestNightRows(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 12, 5, 0, 0, 0, 0, time.UTC))
	tier := saasTierL1
	byHand := map[string]bool{}
	for _, m := range saasTestTeam("apollo", 14) {
		byHand[m.UserID] = true
		repo.assignments = append(repo.assignments, domain.ScheduleAssignment{
			ID: "hand-" + m.UserID, Engineer: domain.ScheduleEngineer{UserID: m.UserID}, TeamKey: "apollo",
			ShiftCode: saasShiftWeekend, Tier: &tier, RotaDate: "2026-11-07", Source: "MANUAL",
		})
	}
	writer := &fakeRotaWriter{}
	svc := newTestRotaService(repo, writer, saasTestNow)
	repo.catalogue.Shifts = saasCatalogueOf(saasTestWindowsDayNight).Shifts
	if _, err := svc.GenerateMonth(leadCtx("apollo.lead@example.com"), generateReq("2026-11")); err != nil {
		t.Fatal(err)
	}
	for _, turn := range writer.got.Turns {
		if turn.RotaDate == "2026-11-07" && byHand[turn.UserID] && turn.Tier != "" {
			t.Fatalf("%s holds SRE_WE_TZ1 by hand on 2026-11-07 and was also given %+v", turn.UserID, turn)
		}
	}
}
