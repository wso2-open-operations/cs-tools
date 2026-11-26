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
	"fmt"
	"math"
	"sort"
	"time"
)

// The SRE SaaS month generator: a port of the Apps Script the Apollo and
// Artemis leads ran over their rota sheet ("ABT on-call rotation generator").
// Pure: it reads only what it is given and touches no database, so a month is
// the same month however often it is worked out.
//
// What the sheet called a role becomes a turn on one of the SaaS shifts the
// catalogue already has (migration 0154):
//
//	weekday L1, per team and zone      SRE_TZ1_L1 / SRE_TZ2_L1 (tier L1)
//	weekday L2, per team and zone      SRE_TZ1 / SRE_TZ2, tier L2
//	weekend on-call / escalation       SRE_WE_TZ1, tier L1 / L2, per team
//	L3, per team, TZ1 and TZ2          the lead on TZ1 (the weekend window at a
//	                                   weekend); TZ2 a teammate free that day,
//	                                   since one person works one zone a day
//	TZ3 L1 / L2 / L3, every night      SRE_TZ3, tier L1 / L2 / L3: ONE crew for
//	                                   the whole rota, not one per team
//
// Weekends have a second shape, Day/Night (06:00-18:00 and 18:00-06:00 IST):
// the same people on SRE_WE_DAY instead of SRE_WE_TZ1, and the night crew on
// SRE_WE_NIGHT instead of SRE_TZ3. Only the hours differ, so only the codes a
// weekend is written on change (saasWeekendShifts). Which shape a run uses is
// saasRotaInput.WeekendDayNight, which the service reads off the catalogue:
// the two Day/Night windows are seeded inactive (migration 0306) and switched
// on, with SRE_WE_TZ1 retired and SRE_TZ3 made weekday-only, in the week
// before the first Day/Night weekend. Weekdays are the same in both shapes.
//
// TZ3 is the night cover. Its three people answer an Apollo and an Artemis
// incident alike -- the pager calls whoever holds the tier when the
// incident's own team holds none -- so a night has one L1, one L2 and one
// L3, drawn from every team's night people, weekends included. Each turn is
// still written on the person's own team, so it shows on their row.
//
// The script's CR pair is not written: the portal has no CR turn. It still
// decides who is L2 each day (one of the week's pair) and keeps the pair out
// of the L1 rotation, exactly as the sheet did. Nobody is written as RnD
// either: a weekday with nothing on it is a working day already.
const (
	saasShiftTZ1L1        = "SRE_TZ1_L1"
	saasShiftTZ1          = "SRE_TZ1"
	saasShiftTZ2L1        = "SRE_TZ2_L1"
	saasShiftTZ2          = "SRE_TZ2"
	saasShiftTZ3          = "SRE_TZ3"
	saasShiftWeekend      = "SRE_WE_TZ1"
	saasShiftWeekendDay   = "SRE_WE_DAY"
	saasShiftWeekendNight = "SRE_WE_NIGHT"
	saasShiftTZ3Regular   = "SRE_TZ3_REGULAR"
	saasShiftTZ1Regular   = "SRE_TZ1_REGULAR"
	saasShiftTZ2Regular   = "SRE_TZ2_REGULAR"

	saasTierL1 = "L1"
	saasTierL2 = "L2"
	saasTierL3 = "L3"
)

// saasRotaShiftCodes is every shift the generator writes or reads on either
// weekend shape, so the service can refuse to run against a catalogue that
// lacks one. The weekend's own codes are checked apart (saasWeekendShapes):
// the catalogue holds one shape or the other, never needs both.
var saasRotaShiftCodes = []string{
	saasShiftTZ1L1, saasShiftTZ1, saasShiftTZ2L1, saasShiftTZ2,
	saasShiftTZ3, saasShiftTZ3Regular, saasShiftTZ1Regular, saasShiftTZ2Regular,
}

// saasWeekendShifts is what one weekend shape is written on: the day crew's
// window and the night crew's, with the zone names its warnings use.
type saasWeekendShifts struct {
	day, night         string // shift codes
	dayZone, nightZone string // names in warnings: "TZ1+2" / "TZ3 weekend", or "Day" / "Night"
}

var (
	saasWeekendTZ = saasWeekendShifts{
		day: saasShiftWeekend, night: saasShiftTZ3, dayZone: "TZ1+2", nightZone: "TZ3 weekend",
	}
	saasWeekendDayNight = saasWeekendShifts{
		day: saasShiftWeekendDay, night: saasShiftWeekendNight, dayZone: "Day", nightZone: "Night",
	}
)

// saasSharedShift is whether each tier of the shift is one person for the
// whole rota (the night crew), not one per team: TZ3, and the weekend Night
// that takes its place at the weekend (both shared_across_teams in the
// catalogue).
func saasSharedShift(code string) bool {
	return code == saasShiftTZ3 || code == saasShiftWeekendNight
}

// saasRotaEpoch is the script's week zero, Monday 1 January 2024. Week
// numbers never reset, so the rotation keeps moving across months instead of
// starting each month from the top of the list.
var saasRotaEpoch = time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)

// saasNightTiers is how many people a night needs: TZ3 L1, L2 and L3. It is
// also the size of a week's night rotation when nobody has TZ3 hours marked.
const saasNightTiers = 3

// saasRotaHorizonWeeks is how far before the month the week-by-week chain is
// started. The chain only remembers last week's CR pair, so a few weeks is
// enough for the first week of the month to land where a longer run would.
const saasRotaHorizonWeeks = 3

// saasRotaMember is one member of a SaaS team.
type saasRotaMember struct {
	UserID  string
	Name    string
	Email   string
	TeamKey string
	IsLead  bool
}

// saasRotaInput is everything one month needs.
type saasRotaInput struct {
	// Month is any day of the month to generate; only its year and month are read.
	Month time.Time
	// From is the first day turns are written for. Zero means the first of
	// the month. Days before it are still worked out, so the rotation is the
	// same whichever day a regeneration starts from.
	From time.Time
	// Members of every SaaS team.
	Members []saasRotaMember
	// Unavailable is user id -> ISO date -> true, for every day the person has
	// leave, an allocation (RnD, ENG, a customer) or any other time away.
	Unavailable map[string]map[string]bool
	// NightOn is user id -> ISO date -> true, for every weekday the person's
	// regular hours are on TZ3. When anybody is marked in a week, that week's
	// nights are drawn from them alone, whichever team they are on. When
	// nobody is, the engineers take turns on nights (see nightWeek).
	NightOn map[string]map[string]bool
	// NightCrew is team key -> the people a lead chose for TZ3 for the month.
	// Every team's choices together are the night crew: the rota's TZ3 L1, L2
	// and L3 go round them night by night, every week, marks or not, and they
	// work no other zone. Nobody else is put on TZ3: a tier the crew cannot
	// cover on a night is left empty and warned about, for a lead to fill.
	NightCrew map[string][]string
	// Pinned is user id -> ISO date -> zone ("TZ1" or "TZ2"), for a day someone
	// else set the person's regular hours on: they work that zone or nothing.
	Pinned map[string]map[string]string
	// NightTeam is user id -> ISO date -> the team of this rota a span has
	// moved them to that day (the SaaS Night cover). Their TZ3 turns that day
	// are filed under it, as the roster shows them there.
	NightTeam map[string]map[string]string
	// Windows is shift code -> [start, end) minute of the day, from the catalogue.
	Windows map[string][2]int
	// WeekendDayNight writes weekends on SRE_WE_DAY / SRE_WE_NIGHT instead of
	// SRE_WE_TZ1 / SRE_TZ3. The people are the same either way.
	WeekendDayNight bool
}

// saasPlannedTurn is one turn the month should hold.
type saasPlannedTurn struct {
	UserID    string
	TeamKey   string
	ShiftCode string
	Tier      string
	RotaDate  string
}

// saasPlannedLieu is the lieu leave a weekend on-call earns. Weekend is the
// Saturday it was earned on, which names the span so a regeneration can find
// it again.
type saasPlannedLieu struct {
	UserID  string
	TeamKey string
	From    string
	To      string
	Weekend string
}

// saasRotaWarning is a slot the generator could not fill, or a gap a lead
// should look at before the month starts.
type saasRotaWarning struct {
	Date    string
	Message string
}

// saasRotaPlan is the month, worked out.
type saasRotaPlan struct {
	Turns    []saasPlannedTurn
	Lieu     []saasPlannedLieu
	Warnings []saasRotaWarning
}

type saasWeekPlan struct {
	pools map[string][]string  // zone -> user ids
	cr    map[string][2]string // zone -> the week's CR pair
}

type saasGenerator struct {
	in         saasRotaInput
	byID       map[string]saasRotaMember
	teams      []string            // sorted team keys
	teamPeople map[string][]string // team -> user ids, sorted by email then id
	everyone   []string            // every team's people, interleaved team by team
	leads      map[string][]string // team -> lead ids, same order
	weekPlans  map[string]map[int]saasWeekPlan
	unavail    map[string]map[string]bool // a copy: lieu is added to it as weekends are passed
	plan       saasRotaPlan
	monthStart time.Time
	monthEnd   time.Time
	from       time.Time
	lieuSeen   map[string]bool     // user|saturday, so a weekend's lieu is recorded once
	lieuDays   map[string][]string // user|saturday -> the lieu days it gave, worked out once
	// defaultNight is set when nobody has TZ3 regular hours marked in the
	// month: each week then takes its night people from every team's
	// engineers (nightWeek). A week with nobody marked takes them the same way
	// even when other weeks are marked (nightByRotation).
	defaultNight bool
	nightWeeks   map[int][]string // week -> its night people, when rotating
	crew         []string         // the chosen TZ3 people, every team's, members only
	inCrew       map[string]bool  // user id -> on the chosen TZ3 crew
	we           saasWeekendShifts
}

// generateSRESaaSRota works out one month of the SaaS rota.
func generateSRESaaSRota(in saasRotaInput) saasRotaPlan {
	g := newSaasGenerator(in)
	g.run()
	return g.plan
}

func newSaasGenerator(in saasRotaInput) *saasGenerator {
	monthStart := time.Date(in.Month.Year(), in.Month.Month(), 1, 0, 0, 0, 0, time.UTC)
	monthEnd := monthStart.AddDate(0, 1, -1)
	from := monthStart
	if !in.From.IsZero() {
		f := saasDay(in.From)
		if f.After(from) {
			from = f
		}
	}
	g := &saasGenerator{
		in:         in,
		byID:       map[string]saasRotaMember{},
		teamPeople: map[string][]string{},
		leads:      map[string][]string{},
		weekPlans:  map[string]map[int]saasWeekPlan{},
		unavail:    map[string]map[string]bool{},
		monthStart: monthStart,
		monthEnd:   monthEnd,
		from:       from,
		lieuSeen:   map[string]bool{},
		lieuDays:   map[string][]string{},
		nightWeeks: map[int][]string{},
		we:         saasWeekendTZ,
	}
	if in.WeekendDayNight {
		g.we = saasWeekendDayNight
	}
	members := append([]saasRotaMember(nil), in.Members...)
	sort.SliceStable(members, func(i, j int) bool {
		if members[i].Email != members[j].Email {
			return members[i].Email < members[j].Email
		}
		return members[i].UserID < members[j].UserID
	})
	for _, m := range members {
		if _, dup := g.byID[m.UserID]; dup {
			continue // on both teams: the first team in email order keeps them
		}
		g.byID[m.UserID] = m
		g.teamPeople[m.TeamKey] = append(g.teamPeople[m.TeamKey], m.UserID)
		if m.IsLead {
			g.leads[m.TeamKey] = append(g.leads[m.TeamKey], m.UserID)
		}
	}
	for team := range g.teamPeople {
		g.teams = append(g.teams, team)
	}
	sort.Strings(g.teams)
	g.everyone = g.interleave(func(team string) []string { return g.teamPeople[team] })
	for user, days := range in.Unavailable {
		cp := make(map[string]bool, len(days))
		for d, v := range days {
			cp[d] = v
		}
		g.unavail[user] = cp
	}
	g.defaultNight = !g.anyNightIn(monthStart, monthEnd)
	g.inCrew = map[string]bool{}
	g.crew = g.interleave(func(team string) []string {
		var ids []string
		for _, p := range in.NightCrew[team] {
			if m, ok := g.byID[p]; ok && m.TeamKey == team && !g.inCrew[p] {
				ids = append(ids, p)
				g.inCrew[p] = true
			}
		}
		return ids
	})
	return g
}

// interleave lists each team's people from of, one from every team in turn
// (apollo's first, artemis's first, apollo's second, ...), so a rotation over
// the list takes from every team alike.
func (g *saasGenerator) interleave(of func(team string) []string) []string {
	lists := make([][]string, len(g.teams))
	most := 0
	for i, team := range g.teams {
		lists[i] = of(team)
		if len(lists[i]) > most {
			most = len(lists[i])
		}
	}
	var out []string
	for i := 0; i < most; i++ {
		for _, l := range lists {
			if i < len(l) {
				out = append(out, l[i])
			}
		}
	}
	return out
}

func (g *saasGenerator) run() {
	for _, team := range g.teams {
		if len(g.leads[team]) == 0 {
			g.warn("", fmt.Sprintf("team %s has no lead, so its L3 is taken from its engineers", team))
		}
	}

	// The chain starts on the Monday a few weeks before the month, so the first
	// week is worked out with the week before it in hand, as the sheet's was.
	start := saasMondayOf(g.monthStart).AddDate(0, 0, -7*saasRotaHorizonWeeks)
	for d := start; !d.After(g.monthEnd); d = d.AddDate(0, 0, 1) {
		emit := !d.Before(g.from)
		wi := saasWeek(d)
		if dow := saasDow(d); dow >= 5 {
			g.weekend(d, emit)
		} else {
			g.weekday(d, wi, dow, emit)
		}
	}
}

// weekday is one Monday-to-Friday of every team and of the shared night rota.
func (g *saasGenerator) weekday(d time.Time, wi, dow int, emit bool) {
	iso := saasISO(d)
	zones := []string{"TZ1", "TZ2"}
	dayTaken := map[string]bool{} // everyone on a day turn today, every team
	zoneOf := map[string]string{} // the zone of each one's turn today, for their SUP
	for _, team := range g.teams {
		wp := g.planForWeek(team, wi)
		// taken is who already holds a day turn today: one turn each.
		taken := map[string]bool{}
		l1s, l2s := map[string]string{}, map[string]string{}
		for _, zone := range zones {
			cr := wp.cr[zone]
			li := (wi + dow) % 2
			l2 := cr[li]
			if l2 == "" || g.offFor(l2, iso, zone) {
				if alt := cr[1-li]; alt != "" && !g.offFor(alt, iso, zone) {
					l2 = alt
				} else {
					l2 = ""
				}
			}
			l2s[zone] = l2
			if l2 != "" {
				taken[l2] = true
			}
		}
		for _, zone := range zones {
			cr := wp.cr[zone]
			var l1pool []string
			for _, p := range wp.pools[zone] {
				if p != cr[0] && p != cr[1] && !g.offFor(p, iso, zone) && !taken[p] {
					l1pool = append(l1pool, p)
				}
			}
			l1 := saasFirst(saasRotate(l1pool, wi+dow))
			if l1 == "" {
				// The zone's half is used up; anyone free on the team takes it.
				l1 = g.spare(team, zone, wi, dow, iso, taken)
			}
			if l1 != "" {
				taken[l1] = true
			}
			l1s[zone] = l1
		}
		for _, zone := range zones {
			if l2s[zone] == "" {
				if l2 := g.spare(team, zone, wi, dow, iso, taken); l2 != "" {
					taken[l2] = true
					l2s[zone] = l2
				}
			}
		}
		for p := range taken {
			dayTaken[p] = true
		}
		for _, zone := range zones {
			for _, p := range []string{l1s[zone], l2s[zone]} {
				if p != "" {
					zoneOf[p] = zone
				}
			}
		}
		if !emit {
			continue
		}
		for _, zone := range zones {
			l1Shift, l2Shift := saasShiftTZ1L1, saasShiftTZ1
			if zone == "TZ2" {
				l1Shift, l2Shift = saasShiftTZ2L1, saasShiftTZ2
			}
			g.turn(l1s[zone], team, l1Shift, saasTierL1, iso, fmt.Sprintf("%s %s L1", team, zone))
			g.turn(l2s[zone], team, l2Shift, saasTierL2, iso, fmt.Sprintf("%s %s L2", team, zone))
		}
	}

	// TZ3, one crew for every team: the night people go round L1, L2 and L3
	// one step a night, so each of them takes every tier in turn.
	crew := len(g.crew) > 0
	rot := crew || g.nightByRotation(wi)
	var present []string
	for _, p := range g.nightPool(wi) {
		if (rot || g.in.NightOn[p][iso]) && !g.offFor(p, iso, "TZ3") {
			present = append(present, p)
		}
	}
	seq := saasRotate(present, saasDayNumber(d))
	if crew && len(seq) < saasNightTiers && emit {
		g.warnCrewShort(iso, "TZ3", len(seq))
	}
	if !crew && len(seq) < saasNightTiers {
		// Fewer night people are free tonight than there are tiers (leave, or
		// marks that stop part-way through a week): the rest come from anyone
		// free and not on a day turn today, leads last. A chosen crew is never
		// topped up this way: TZ3 is theirs alone.
		seq = append(seq, g.nightSpares(wi, dow, iso, dayTaken, seq)...)
	}
	for i, tier := range []string{saasTierL1, saasTierL2, saasTierL3} {
		p := saasAt(seq, i)
		if p != "" {
			dayTaken[p] = true
			zoneOf[p] = "TZ3"
		}
		if emit {
			g.turn(p, "", saasShiftTZ3, tier, iso, g.nightSlot("TZ3 "+tier))
		}
	}
	if !emit {
		return
	}
	for _, team := range g.teams {
		wp := g.planForWeek(team, wi)
		g.l3(team, []saasL3Zone{
			{saasShiftTZ1, "TZ1", wp.pools["TZ1"]},
			{saasShiftTZ2, "TZ2", wp.pools["TZ2"]},
		}, g.nightPool(wi), wi+dow, iso, dayTaken, zoneOf)
	}
	g.regularHours(wi, iso, zoneOf)
}

// regularHours is the SUP of everyone not away on a weekday: the regular
// hours of the one zone they work that day, beside any L1-L3 they hold.
// Somebody on a turn works that turn's zone; otherwise night people's is
// TZ3, a lead's TZ1, everyone else's the half of the team they are in that
// week. Never all three zones: nobody works TZ1, TZ2 and TZ3 in a day.
func (g *saasGenerator) regularHours(wi int, iso string, zoneOf map[string]string) {
	night := map[string]bool{}
	for _, p := range g.nightPool(wi) {
		night[p] = true
	}
	for _, team := range g.teams {
		wp := g.planForWeek(team, wi)
		tz2 := map[string]bool{}
		for _, p := range wp.pools["TZ2"] {
			tz2[p] = true
		}
		for _, p := range g.teamPeople[team] {
			if g.isOff(p, iso) || g.in.Pinned[p][iso] != "" || g.in.NightTeam[p][iso] != "" {
				// Away, their regular hours set by someone else, or on the
				// Night cover, where they work TZ3 turns only.
				continue
			}
			code := saasShiftTZ1Regular
			switch {
			case zoneOf[p] == "TZ3":
				code = saasShiftTZ3Regular
			case zoneOf[p] == "TZ2":
				code = saasShiftTZ2Regular
			case zoneOf[p] == "TZ1":
				code = saasShiftTZ1Regular
			case night[p]:
				code = saasShiftTZ3Regular
			case g.byID[p].IsLead:
				code = saasShiftTZ1Regular
			case tz2[p]:
				code = saasShiftTZ2Regular
			}
			g.turn(p, team, code, "", iso, "")
		}
	}
}

// weekend is a Saturday or Sunday. Both days of a weekend get the same people,
// picked from those free on both, and each on-call earns the Monday and
// Tuesday after it off.
func (g *saasGenerator) weekend(d time.Time, emit bool) {
	sat := d
	if saasDow(d) == 6 {
		sat = d.AddDate(0, 0, -1)
	}
	satWeek := saasWeek(sat)
	iso := saasISO(d)
	taken := map[string]bool{} // everyone on a weekend turn today
	for _, team := range g.teams {
		oncall, esc := g.weekendCrew(team, g.we.dayZone, g.dayPool(team, satWeek), sat, satWeek, nil)
		if emit {
			g.turn(oncall, team, g.we.day, saasTierL1, iso, fmt.Sprintf("%s weekend on-call", team))
			g.turn(esc, team, g.we.day, saasTierL2, iso, fmt.Sprintf("%s weekend escalation", team))
		}
		g.lieuFor(oncall, sat, emit)
		for _, p := range []string{oncall, esc} {
			if p != "" {
				taken[p] = true
			}
		}
	}
	// The weekend nights, one crew for every team as on a weekday: the night
	// people first, then anyone free both days, leads last. Its L1 is the
	// night's on-call and earns the lieu.
	nights := g.weekendNights(sat, satWeek, taken)
	if len(g.crew) > 0 && len(nights) < saasNightTiers && emit {
		g.warnCrewShort(iso, g.we.nightZone, len(nights))
	}
	for i, tier := range []string{saasTierL1, saasTierL2, saasTierL3} {
		p := saasAt(nights, i)
		if p != "" {
			taken[p] = true
		}
		if emit {
			g.turn(p, "", g.we.night, tier, iso, g.nightSlot(g.we.nightZone+" "+tier))
		}
	}
	g.lieuFor(saasAt(nights, 0), sat, emit)
	if !emit {
		return
	}
	for _, team := range g.teams {
		g.l3(team, []saasL3Zone{
			{g.we.day, g.we.dayZone, g.dayPool(team, satWeek)},
		}, g.nightPool(satWeek), satWeek, iso, taken, map[string]string{})
	}
}

// weekendNights is a weekend's TZ3 L1, L2 and L3: the week's night people
// free both days first, then anyone else free both days (every team, taken
// in turn), leads last in each. With a chosen crew only the crew is picked,
// so it can come back with fewer than three. Nobody in skip is picked.
func (g *saasGenerator) weekendNights(sat time.Time, satWeek int, skip map[string]bool) []string {
	satISO, sunISO := saasISO(sat), saasISO(sat.AddDate(0, 0, 1))
	picked := map[string]bool{}
	var out []string
	from := func(pool []string) {
		var avail, leads []string
		for _, p := range pool {
			if skip[p] || picked[p] || g.offFor(p, satISO, "TZ3") || g.offFor(p, sunISO, "TZ3") {
				continue
			}
			if g.byID[p].IsLead {
				leads = append(leads, p)
			} else {
				avail = append(avail, p)
			}
		}
		for _, p := range append(saasRotate(avail, satWeek), saasRotate(leads, satWeek)...) {
			if len(out) < saasNightTiers {
				out = append(out, p)
				picked[p] = true
			}
		}
	}
	from(g.nightPool(satWeek))
	if len(out) < saasNightTiers && len(g.crew) == 0 {
		from(g.everyone)
	}
	return out
}

// warnCrewShort says, once for the night, which tiers the chosen crew leaves
// empty. The empty slots themselves are not warned again (nightSlot).
func (g *saasGenerator) warnCrewShort(iso, zone string, free int) {
	tiers := map[int]string{1: "L3 is", 2: "L2 and L3 are", 3: "L1, L2 and L3 are"}[saasNightTiers-free]
	who := fmt.Sprintf("only %d of the chosen TZ3 people is free", free)
	if free == 0 {
		who = "none of the chosen TZ3 people is free"
	} else if free > 1 {
		who = fmt.Sprintf("only %d of the chosen TZ3 people are free", free)
	}
	g.warn(iso, fmt.Sprintf("%s: %s, so %s left empty; nobody else is put on TZ3", zone, who, tiers))
}

// nightSlot is the slot name a TZ3 turn warns under when it is empty: none
// with a chosen crew, whose short night warnCrewShort already reported.
func (g *saasGenerator) nightSlot(slot string) string {
	if len(g.crew) > 0 {
		return ""
	}
	return slot
}

// weekendCrew is a weekend's on-call and escalation for one team: from pool
// first, then anyone else on the team free both days, leads last. Nobody in
// skip is picked.
func (g *saasGenerator) weekendCrew(team, zone string, pool []string, sat time.Time, satWeek int, skip map[string]bool) (string, string) {
	var first []string
	for _, p := range pool {
		if !skip[p] {
			first = append(first, p)
		}
	}
	a, b := g.weekendPick(first, zone, sat, satWeek)
	if a != "" && b != "" {
		return a, b
	}
	var rest []string
	for _, p := range g.teamPeople[team] {
		if !skip[p] && p != a && p != b && saasIndexOf(first, p) < 0 {
			rest = append(rest, p)
		}
	}
	more, more2 := g.weekendPick(rest, zone, sat, satWeek)
	fill := []string{}
	for _, p := range []string{more, more2} {
		if p != "" {
			fill = append(fill, p)
		}
	}
	if a == "" && len(fill) > 0 {
		a, fill = fill[0], fill[1:]
	}
	if b == "" && len(fill) > 0 {
		b = fill[0]
	}
	return a, b
}

// weekendPick is the script's weekendPick_: the free people rotated by the
// Saturday's week, the first on call and the second the escalation. A lead is
// picked only when nobody else is free, because the weekend window's L3 is theirs.
func (g *saasGenerator) weekendPick(pool []string, zone string, sat time.Time, satWeek int) (string, string) {
	satISO, sunISO := saasISO(sat), saasISO(sat.AddDate(0, 0, 1))
	var avail, leads []string
	for _, p := range pool {
		if g.offFor(p, satISO, zone) || g.offFor(p, sunISO, zone) {
			continue
		}
		if g.byID[p].IsLead {
			leads = append(leads, p)
		} else {
			avail = append(avail, p)
		}
	}
	seq := append(saasRotate(avail, satWeek), saasRotate(leads, satWeek)...)
	return saasAt(seq, 0), saasAt(seq, 1)
}

// nightSpares is, in order, everyone on any team who could take a night the
// night people cannot: not away, not on a day turn today, not already picked.
// Leads come last; their L3 is on TZ1.
func (g *saasGenerator) nightSpares(wi, dow int, iso string, dayTaken map[string]bool, picked []string) []string {
	skip := map[string]bool{}
	for _, p := range picked {
		skip[p] = true
	}
	var engineers, leads []string
	for _, p := range g.everyone {
		if skip[p] || dayTaken[p] || g.offFor(p, iso, "TZ3") {
			continue
		}
		if g.byID[p].IsLead {
			leads = append(leads, p)
		} else {
			engineers = append(engineers, p)
		}
	}
	return append(saasRotate(engineers, wi+dow), saasRotate(leads, wi+dow)...)
}

// spare is anyone on the team free for a day turn, for a slot the zone's own
// half cannot fill: every engineer counts unless marked away, on nights that
// week, or already holding a day turn today. A lead only when nobody else is,
// because their TZ1 L3 goes to someone else if they take a turn.
func (g *saasGenerator) spare(team, zone string, wi, dow int, iso string, taken map[string]bool) string {
	var engineers, leads []string
	for _, p := range g.dayPool(team, wi) {
		if g.offFor(p, iso, zone) || taken[p] {
			continue
		}
		if g.byID[p].IsLead {
			leads = append(leads, p)
		} else {
			engineers = append(engineers, p)
		}
	}
	return saasFirstOf(saasRotate(engineers, wi+dow), saasRotate(leads, wi+dow))
}

// lieuFor marks the Monday and Tuesday after a weekend on-call off, and
// records it once. A day the person already has off is left as it is.
//
// The days are worked out on the weekend's first call (the Saturday) and kept,
// so a weekend that starts in the month before still records its lieu when
// its Sunday is the first day written -- the Saturday's call marked the days
// off already, and working them out again would find none.
func (g *saasGenerator) lieuFor(user string, sat time.Time, emit bool) {
	if user == "" {
		return
	}
	key := user + "|" + saasISO(sat)
	added, worked := g.lieuDays[key]
	if !worked {
		for off := 2; off <= 3; off++ {
			iso := saasISO(sat.AddDate(0, 0, off))
			if g.isOff(user, iso) {
				continue
			}
			if g.unavail[user] == nil {
				g.unavail[user] = map[string]bool{}
			}
			g.unavail[user][iso] = true
			added = append(added, iso)
		}
		g.lieuDays[key] = added
	}
	if !emit || len(added) == 0 {
		return
	}
	if g.lieuSeen[key] {
		return
	}
	g.lieuSeen[key] = true
	g.plan.Lieu = append(g.plan.Lieu, saasPlannedLieu{
		UserID: user, TeamKey: g.byID[user].TeamKey, From: added[0], To: added[len(added)-1],
		Weekend: saasISO(sat),
	})
}

// saasL3Zone is one zone a team needs an L3 for: the shift it is written on,
// the zone's name, and who belongs on it first (the zone's half).
type saasL3Zone struct {
	shift string
	zone  string
	pool  []string
}

// l3 is a team's L3 for each of its day zones, in order (TZ3's L3 is the
// night crew's, not a team's). One person works one zone a day, so nobody
// already on a turn today is picked, and nobody is L3 on two zones: the lead
// takes the first zone (TZ1, or the weekend window, beside their regular
// hours), and every other zone goes to whoever is free -- another lead first,
// then the zone's own people, then the rest of the team, the week's night
// people last. A zone nobody can take is warned about.
func (g *saasGenerator) l3(team string, zones []saasL3Zone, night []string, rot int, iso string, taken map[string]bool, zoneOf map[string]string) {
	isNight := map[string]bool{}
	for _, p := range night {
		isNight[p] = true
	}
	for _, z := range zones {
		free := func(p string) bool { return !taken[p] && !g.offFor(p, iso, z.zone) }
		var leads, own, day, nightRest []string
		for _, p := range g.leads[team] {
			if free(p) {
				leads = append(leads, p)
			}
		}
		inPool := map[string]bool{}
		for _, p := range z.pool {
			if free(p) && !g.byID[p].IsLead {
				own = append(own, p)
				inPool[p] = true
			}
		}
		for _, p := range g.teamPeople[team] {
			if !free(p) || g.byID[p].IsLead || inPool[p] {
				continue
			}
			if isNight[p] {
				nightRest = append(nightRest, p)
			} else {
				day = append(day, p)
			}
		}
		seq := append(leads, saasRotate(own, rot)...)
		seq = append(seq, saasRotate(day, rot)...)
		seq = append(seq, saasRotate(nightRest, rot)...)
		p := saasAt(seq, 0)
		if p == "" {
			g.warn(iso, fmt.Sprintf("%s %s L3 is empty: nobody is free", team, z.zone))
			continue
		}
		taken[p] = true
		zoneOf[p] = z.zone
		g.turn(p, team, z.shift, saasTierL3, iso, "")
	}
}

// planForWeek is the script's computeWeek_, run from the start of the chain
// so a week is always worked out with the one before it.
func (g *saasGenerator) planForWeek(team string, wi int) saasWeekPlan {
	plans := g.weekPlans[team]
	if plans == nil {
		plans = map[int]saasWeekPlan{}
		g.weekPlans[team] = plans
	}
	if wp, ok := plans[wi]; ok {
		return wp
	}
	// Leads are kept out of the halves -- so out of the week's L2 pair and the
	// L1 rotation -- and are only drawn as a spare when nobody else is free:
	// L3 on TZ1 is theirs, and any turn they take moves it to someone else.
	var pool []string
	for _, p := range g.dayPool(team, wi) {
		if !g.byID[p].IsLead {
			pool = append(pool, p)
		}
	}
	rotated := saasRotate(pool, wi*2)
	half := int(math.Ceil(float64(len(rotated)) / 2))
	pools := map[string][]string{
		"TZ1": append([]string(nil), rotated[:half]...),
		"TZ2": append([]string(nil), rotated[half:]...),
	}
	weekdays := saasWeekdaysOf(wi)
	prevCR := map[string]bool{}
	if prev, ok := plans[wi-1]; ok {
		for _, zone := range []string{"TZ1", "TZ2"} {
			for _, n := range prev.cr[zone] {
				if n != "" {
					prevCR[n] = true
				}
			}
		}
	}
	isAvail := func(n string) bool {
		for _, iso := range weekdays {
			if g.isOff(n, iso) {
				return false
			}
		}
		return true
	}
	fresh := func(zone string) []string {
		var out []string
		for _, n := range pools[zone] {
			if isAvail(n) && !prevCR[n] {
				out = append(out, n)
			}
		}
		return out
	}

	// The script's balancing: when one half cannot field two fresh CRs and the
	// other has spares, swap one person across, so neither zone has to repeat
	// last week's CR.
	for pass := 0; pass < 2; pass++ {
		f1, f2 := fresh("TZ1"), fresh("TZ2")
		var poor, rich string
		switch {
		case len(f1) < 2 && len(f2) > 2:
			poor, rich = "TZ1", "TZ2"
		case len(f2) < 2 && len(f1) > 2:
			poor, rich = "TZ2", "TZ1"
		}
		if poor == "" {
			break
		}
		freshRich := fresh(rich)
		give := freshRich[len(freshRich)-1] // the rich half's spare fresh person
		take := ""
		for _, n := range pools[poor] { // prefer trading away someone on leave
			if !isAvail(n) {
				take = n
				break
			}
		}
		if take == "" {
			for _, n := range pools[poor] { // else last week's CR
				if prevCR[n] {
					take = n
					break
				}
			}
		}
		if take == "" && len(pools[poor]) > 0 {
			take = pools[poor][len(pools[poor])-1]
		}
		if take == "" || give == take {
			break
		}
		pools[rich][saasIndexOf(pools[rich], give)] = take
		pools[poor][saasIndexOf(pools[poor], take)] = give
	}

	wp := saasWeekPlan{pools: pools, cr: map[string][2]string{}}
	for _, zone := range []string{"TZ1", "TZ2"} {
		pool := pools[zone]
		var avail, pref []string
		for _, n := range pool {
			if isAvail(n) {
				avail = append(avail, n)
				if !prevCR[n] {
					pref = append(pref, n)
				}
			}
		}
		cr0 := saasFirstOf(pref, avail, pool)
		cr1 := saasFirstOther(cr0, pref, avail, pool)
		wp.cr[zone] = [2]string{cr0, cr1}
	}
	plans[wi] = wp
	return wp
}

// dayPool is a team's people for a week, less the week's night people.
func (g *saasGenerator) dayPool(team string, wi int) []string {
	night := map[string]bool{}
	for _, p := range g.nightPool(wi) {
		night[p] = true
	}
	var out []string
	for _, p := range g.teamPeople[team] {
		if !night[p] {
			out = append(out, p)
		}
	}
	return out
}

// nightPool is the week's night people, every team's: who the TZ3 turns are
// drawn from. A chosen crew when there is one; else everyone with TZ3
// regular hours marked that week; else the week's rotation (nightWeek).
func (g *saasGenerator) nightPool(wi int) []string {
	if len(g.crew) > 0 {
		return g.crew
	}
	if g.nightByRotation(wi) {
		return g.nightWeek(wi)
	}
	return g.markedNightPeople(wi)
}

// markedNightPeople is everyone on any team with TZ3 regular hours marked on
// a weekday of the week.
func (g *saasGenerator) markedNightPeople(wi int) []string {
	weekdays := saasWeekdaysOf(wi)
	var out []string
	for _, p := range g.everyone {
		for _, iso := range weekdays {
			if g.in.NightOn[p][iso] {
				out = append(out, p)
				break
			}
		}
	}
	return out
}

// nightByRotation is whether the week's nights go round the engineers: when
// nobody has TZ3 regular hours marked that month, or nobody that week --
// marks stop part-way through a month, and the nights after them still need
// people.
func (g *saasGenerator) nightByRotation(wi int) bool {
	return g.defaultNight || len(g.markedNightPeople(wi)) == 0
}

// nightWeek is the week's night people when nobody has TZ3 regular hours
// marked: one per tier, engineers free every weekday of the week, taken from
// every team in turn and rotating week by week, passing over last week's
// people while anybody else is free. Leads are left out: their L3 is on TZ1.
// Anyone a lead marks away (leave, RnD, ENG, any allocation) is not picked,
// which is how a lead keeps somebody off nights.
func (g *saasGenerator) nightWeek(wi int) []string {
	if set, ok := g.nightWeeks[wi]; ok {
		return set
	}
	prev := map[string]bool{}
	for _, p := range g.nightWeeks[wi-1] {
		prev[p] = true
	}
	weekdays := saasWeekdaysOf(wi)
	var engineers []string
	for _, p := range g.everyone {
		if !g.byID[p].IsLead {
			engineers = append(engineers, p)
		}
	}
	var pref, rest []string
	for _, p := range saasRotate(engineers, wi*saasNightTiers) {
		free := true
		for _, iso := range weekdays {
			if g.isOff(p, iso) {
				free = false
				break
			}
		}
		if !free {
			continue
		}
		if prev[p] {
			rest = append(rest, p)
		} else {
			pref = append(pref, p)
		}
	}
	set := append(pref, rest...)
	if len(set) > saasNightTiers {
		set = set[:saasNightTiers]
	}
	g.nightWeeks[wi] = set
	return set
}

func (g *saasGenerator) anyNightIn(from, to time.Time) bool {
	for _, days := range g.in.NightOn {
		for iso, on := range days {
			if !on {
				continue
			}
			d, err := time.Parse("2006-01-02", iso)
			if err == nil && !d.Before(from) && !d.After(to) {
				return true
			}
		}
	}
	return false
}

// turn records a turn, or warns that the slot is empty. A turn for a night
// person carries the team they are on that night: the one a span moved them
// to (the SaaS Night cover), or their own.
func (g *saasGenerator) turn(user, team, shift, tier, iso, slot string) {
	if user == "" {
		if slot != "" {
			g.warn(iso, fmt.Sprintf("%s is empty: nobody is free", slot))
		}
		return
	}
	if team == "" {
		team = g.byID[user].TeamKey
		if moved := g.in.NightTeam[user][iso]; moved != "" {
			team = moved
		}
	}
	t := saasPlannedTurn{UserID: user, TeamKey: team, ShiftCode: shift, Tier: tier, RotaDate: iso}
	g.plan.Turns = append(g.plan.Turns, t)
}

func (g *saasGenerator) isOff(user, iso string) bool {
	return g.unavail[user][iso]
}

// offFor is whether user cannot work zone on iso: away, or the day already
// set to another zone by someone else -- regular hours a lead set (Pinned), or
// TZ3 hours marked (NightOn). One person works one zone a day.
func (g *saasGenerator) offFor(user, iso, zone string) bool {
	if g.isOff(user, iso) {
		return true
	}
	pin := g.in.Pinned[user][iso]
	if pin == "" && (g.inCrew[user] || g.in.NightOn[user][iso]) {
		pin = "TZ3"
	}
	return pin != "" && pin != zone
}

func (g *saasGenerator) warn(iso, msg string) {
	g.plan.Warnings = append(g.plan.Warnings, saasRotaWarning{Date: iso, Message: msg})
}

// saasDayNumber is the number of days since the epoch Monday, so a rotation
// by it moves one step every day, across weekends and months.
func saasDayNumber(d time.Time) int {
	return int(saasDay(d).Sub(saasRotaEpoch).Hours() / 24)
}

// saasWeek is the number of whole weeks since the epoch Monday.
func saasWeek(d time.Time) int {
	days := int(saasDay(d).Sub(saasRotaEpoch).Hours() / 24)
	if days < 0 {
		return (days - 6) / 7
	}
	return days / 7
}

// weekdaysOf is the Monday-to-Friday of a week, even when it spans two months.
func saasWeekdaysOf(wi int) []string {
	monday := saasRotaEpoch.AddDate(0, 0, wi*7)
	out := make([]string, 5)
	for i := range out {
		out[i] = saasISO(monday.AddDate(0, 0, i))
	}
	return out
}

func saasMondayOf(d time.Time) time.Time {
	return saasDay(d).AddDate(0, 0, -saasDow(d))
}

// mondayIndex is the day of the week counted from Monday (0) to Sunday (6).
func saasDow(d time.Time) int {
	return (int(d.Weekday()) + 6) % 7
}

func saasDay(d time.Time) time.Time {
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
}

func saasISO(d time.Time) string {
	return d.Format("2006-01-02")
}

// rotate is the script's rotate_: the list started from position n, wrapping.
func saasRotate(a []string, n int) []string {
	if len(a) == 0 {
		return nil
	}
	n = ((n % len(a)) + len(a)) % len(a)
	out := make([]string, 0, len(a))
	out = append(out, a[n:]...)
	return append(out, a[:n]...)
}

func saasFirst(a []string) string { return saasAt(a, 0) }

func saasAt(a []string, i int) string {
	if i < len(a) {
		return a[i]
	}
	return ""
}

func saasFirstOf(lists ...[]string) string {
	for _, l := range lists {
		if len(l) > 0 {
			return l[0]
		}
	}
	return ""
}

func saasFirstOther(not string, lists ...[]string) string {
	for _, l := range lists {
		for _, n := range l {
			if n != not {
				return n
			}
		}
	}
	return ""
}

func saasIndexOf(a []string, s string) int {
	for i, v := range a {
		if v == s {
			return i
		}
	}
	return -1
}
