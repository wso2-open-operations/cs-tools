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
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"
	// The zone database, embedded: "today" is counted in Asia/Colombo, and a
	// slim image with no zoneinfo would otherwise make it UTC's day.
	_ "time/tzdata"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// RotaSRESaaS is the one rota that can be generated today.
const RotaSRESaaS = "SRE_SAAS"

// saasLieuNotePrefix starts the note on every lieu leave span the generator
// writes; the Saturday follows it. A regeneration finds its own spans by it.
const saasLieuNotePrefix = "Lieu leave for the weekend on-call of"

// rotaAuthoringZone is the zone the rota's days are counted in, for "today".
const rotaAuthoringZone = "Asia/Colombo"

// RotaGenerateService works out and writes a month of a rota from the
// availability leads have marked. Only the SaaS SRE rota (Apollo and
// Artemis) can be generated.
type RotaGenerateService interface {
	GenerateMonth(ctx context.Context, req domain.GenerateRotaMonthRequest) (domain.GenerateRotaMonthResponse, error)
	// NightCrew is the TZ3 people chosen for each month fromMonth..toMonth
	// (YYYY-MM), as the months' last real generations recorded them.
	NightCrew(ctx context.Context, rotaCode, fromMonth, toMonth string) (domain.RotaNightCrewResponse, error)
}

// rotaNightCrewMaxMonths bounds a night crew read: the roster shows six
// months at most.
const rotaNightCrewMaxMonths = 12

type rotaGenerateService struct {
	schedule repository.ScheduleRepository
	writer   repository.RotaGenerateRepository
	access   AccessService
	now      func() time.Time
}

// NewRotaGenerateService constructs a RotaGenerateService. It reads through
// the Team Schedule's own repository, unchanged, and writes through its own.
func NewRotaGenerateService(schedule repository.ScheduleRepository, writer repository.RotaGenerateRepository, access AccessService) RotaGenerateService {
	return &rotaGenerateService{schedule: schedule, writer: writer, access: access, now: time.Now}
}

// GenerateMonth is the "Generate month" action.
//
// Who may: an internal caller with a user token who leads one of the rota's
// teams or holds the SRE rota admin role -- the people who may edit that rota
// by hand (requireRotaWriter's rule, checked here for the rota as a whole,
// since one generation writes every team of it, the shared night rota
// included). A service credential is refused, as it is for every rota edit.
func (s *rotaGenerateService) GenerateMonth(ctx context.Context, req domain.GenerateRotaMonthRequest) (domain.GenerateRotaMonthResponse, error) {
	var out domain.GenerateRotaMonthResponse
	if err := RequireInternalCaller(ctx, s.access, "the team schedule is only available to internal staff"); err != nil {
		return out, err
	}
	actor := auth.IdentityFromContext(ctx).UserEmail
	if actor == "" {
		return out, &apierror.ForbiddenError{Msg: "generating the rota needs a user token, not a service credential"}
	}
	if !strings.EqualFold(req.RotaCode, RotaSRESaaS) {
		return out, &apierror.NotFoundError{Msg: fmt.Sprintf("rota %q cannot be generated; only %s can", req.RotaCode, RotaSRESaaS)}
	}

	month, err := time.Parse("2006-01", strings.TrimSpace(req.Month))
	if err != nil {
		return out, &apierror.ValidationError{Msg: "month must be YYYY-MM"}
	}
	monthStart := month
	monthEnd := monthStart.AddDate(0, 1, -1)
	today := s.today()
	if monthEnd.Before(today) {
		return out, &apierror.ValidationError{Msg: "that month is over; only the current month or a later one can be generated"}
	}
	from := monthStart
	if today.After(from) {
		from = today
	}
	if req.From != nil && strings.TrimSpace(*req.From) != "" {
		f, err := time.Parse("2006-01-02", strings.TrimSpace(*req.From))
		if err != nil {
			return out, &apierror.ValidationError{Msg: "from must be YYYY-MM-DD"}
		}
		if f.Before(monthStart) || f.After(monthEnd) {
			return out, &apierror.ValidationError{Msg: "from must be a day of the month being generated"}
		}
		if f.Before(today) {
			return out, &apierror.ValidationError{Msg: "from is in the past; the rota that was worked is not regenerated"}
		}
		from = f
	}

	cat, err := s.schedule.Catalogue(ctx)
	if err != nil {
		return out, err
	}
	teams, members := saasRotaTeams(cat)
	if len(teams) == 0 {
		return out, &apierror.ServiceUnavailableError{Msg: "no team is on the SRE_SAAS rota"}
	}
	crew, err := saasNightCrew(req.NightCrew, teams, members)
	if err != nil {
		return out, err
	}
	windows, dayNight, err := saasShiftWindows(cat)
	if err != nil {
		return out, err
	}
	if !hasAbsenceKind(cat, "LIEU_LEAVE") {
		return out, &apierror.ServiceUnavailableError{Msg: "the catalogue has no LIEU_LEAVE kind for the weekend on-call's days off"}
	}
	if err := s.requireGenerator(ctx, actor, teams); err != nil {
		return out, err
	}

	// What the month is worked out from: the weeks before it (the chain), the
	// month, and the Monday and Tuesday after it (the last weekend's lieu).
	readFrom := saasMondayOf(monthStart).AddDate(0, 0, -7*saasRotaHorizonWeeks)
	readTo := monthEnd.AddDate(0, 0, 3)

	assignments, err := s.schedule.SearchAssignments(ctx, domain.SearchScheduleAssignmentsRequest{
		From: saasISO(readFrom), To: saasISO(readTo), TeamKeys: teams,
	})
	if err != nil {
		return out, err
	}
	absences, err := s.schedule.SearchAbsences(ctx, domain.SearchScheduleAbsencesRequest{
		From: saasISO(readFrom), To: saasISO(readTo),
	})
	if err != nil {
		return out, err
	}

	// The weekends this run owns the lieu leave of: every weekend with a day
	// written. Their earlier copies are not availability -- they are what is
	// being replaced.
	ownedNotes := saasOwnedLieuNotes(from, monthEnd)
	// What the generator wrote before, found by its owner -- never by source
	// alone: the seed and rota imports write GENERATED rows too, and to the
	// generator those are turns set by somebody else, to keep.
	owner := repository.RotaGeneratorOwner(RotaSRESaaS)
	owned, err := s.writer.GeneratedRows(ctx, owner, teams, saasISO(readFrom), saasISO(readTo))
	if err != nil {
		return out, err
	}
	memberIDs := map[string]bool{}
	for _, m := range members {
		memberIDs[m.UserID] = true
	}
	// A tag that moves somebody to another team of this rota (the SaaS Night
	// cover) is not time away: it is where they work those nights.
	movesWithin := saasMovesWithinRota(cat, teams)
	unavailable := map[string]map[string]bool{}
	nightTeam := map[string]map[string]string{}
	for _, a := range absences {
		id := a.Engineer.UserID
		if !memberIDs[id] {
			continue
		}
		if owned.Absences[a.ID] && a.KindCode == "LIEU_LEAVE" && a.Note != nil && ownedNotes[*a.Note] {
			continue
		}
		start, err := time.Parse("2006-01-02", a.StartsOn)
		if err != nil {
			continue
		}
		end := readTo
		if a.EndsOn != nil {
			if e, err := time.Parse("2006-01-02", *a.EndsOn); err == nil && e.Before(end) {
				end = e
			}
		}
		if start.Before(readFrom) {
			start = readFrom
		}
		movedTo := movesWithin[a.KindCode]
		for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
			if movedTo != "" {
				if nightTeam[id] == nil {
					nightTeam[id] = map[string]string{}
				}
				nightTeam[id][saasISO(d)] = movedTo
				continue
			}
			if unavailable[id] == nil {
				unavailable[id] = map[string]bool{}
			}
			unavailable[id][saasISO(d)] = true
		}
	}
	// With nobody chosen for TZ3, the month's TZ3 crew is whoever a lead moved
	// onto the Night cover for any of its days. They work TZ3 only, and only
	// while the move lasts: a day of the month it does not cover, they are back
	// on their own team and left for a lead to roster, rather than put on a
	// zone the generator would then be planning around a crew for.
	if len(crew) == 0 {
		crew = saasCrewFromMoves(members, nightTeam, from, monthEnd)
	}
	for _, ids := range crew {
		for _, id := range ids {
			if len(nightTeam[id]) == 0 {
				continue
			}
			for d := from; !d.After(monthEnd); d = d.AddDate(0, 0, 1) {
				if nightTeam[id][saasISO(d)] != "" {
					continue
				}
				if unavailable[id] == nil {
					unavailable[id] = map[string]bool{}
				}
				unavailable[id][saasISO(d)] = true
			}
		}
	}

	nightOn := map[string]map[string]bool{}
	pinned := map[string]map[string]string{}
	alreadyGenerated := false
	var byHand []domain.ScheduleAssignment
	// handDay is every person-day something not the generator's is set on --
	// a turn or regular hours. The generator writes no SUP over those.
	handDay := map[string]bool{}
	inMonth := func(iso string) bool { return iso >= saasISO(monthStart) && iso <= saasISO(monthEnd) }
	for _, a := range assignments {
		id := a.Engineer.UserID
		if owned.Assignments[a.ID] {
			// The generator's own rows are what a regeneration replaces: not a
			// lead's marks, and not set by hand.
			if inMonth(a.RotaDate) && (saasIsRotaShift(a.ShiftCode) || saasIsRegularShift(a.ShiftCode)) {
				alreadyGenerated = true
			}
			continue
		}
		if saasIsRotaShift(a.ShiftCode) || saasIsRegularShift(a.ShiftCode) {
			handDay[id+"|"+a.RotaDate] = true
		}
		// One person works one zone a day. A turn someone else set is the
		// person's whole day: the generator puts them nowhere else on it.
		// Regular hours someone else set keep them to that zone. TZ3 regular
		// hours stay a night mark (and keep them to TZ3 the same way).
		if memberIDs[id] && inMonth(a.RotaDate) {
			switch {
			case saasIsRotaShift(a.ShiftCode):
				if unavailable[id] == nil {
					unavailable[id] = map[string]bool{}
				}
				unavailable[id][a.RotaDate] = true
			case a.ShiftCode == saasShiftTZ1Regular || a.ShiftCode == saasShiftTZ2Regular:
				if pinned[id] == nil {
					pinned[id] = map[string]string{}
				}
				pinned[id][a.RotaDate] = "TZ1"
				if a.ShiftCode == saasShiftTZ2Regular {
					pinned[id][a.RotaDate] = "TZ2"
				}
			}
		}
		if a.ShiftCode == saasShiftTZ3Regular && memberIDs[id] {
			if nightOn[id] == nil {
				nightOn[id] = map[string]bool{}
			}
			nightOn[id][a.RotaDate] = true
			continue
		}
		if !saasIsRotaShift(a.ShiftCode) || !inMonth(a.RotaDate) {
			continue
		}
		byHand = append(byHand, a)
	}

	plan := generateSRESaaSRota(saasRotaInput{
		Month: monthStart, From: from, Members: members,
		Unavailable: unavailable, NightOn: nightOn, Pinned: pinned, NightCrew: crew, Windows: windows,
		WeekendDayNight: dayNight,
		NightTeam:       nightTeam,
	})

	// A slot a lead filled by hand, or a turn that would overlap one the
	// person holds by hand, is left to the lead.
	var keep []saasPlannedTurn
	keptByHand := 0
	for _, t := range plan.Turns {
		if t.Tier == "" { // a SUP: only on a day nobody else has set anything for them
			if !handDay[t.UserID+"|"+t.RotaDate] {
				keep = append(keep, t)
			}
			continue
		}
		if saasClashesWithHand(t, byHand, windows) {
			keptByHand++
			continue
		}
		keep = append(keep, t)
	}

	names := map[string]string{}
	for _, m := range members {
		names[m.UserID] = m.Name
	}
	out = domain.GenerateRotaMonthResponse{
		RotaCode:         RotaSRESaaS,
		Month:            monthStart.Format("2006-01"),
		From:             saasISO(from),
		DryRun:           req.DryRun,
		AlreadyGenerated: alreadyGenerated,
		Turns:            make([]domain.GeneratedRotaTurn, 0, len(keep)),
		LieuLeave:        make([]domain.GeneratedLieuLeave, 0, len(plan.Lieu)),
		Warnings:         make([]domain.RotaGenerationWarning, 0, len(plan.Warnings)),
		Summary: domain.RotaGenerationSummary{
			// Planned is the slots worked out: the turns and SUP it would write,
			// and the slots left as somebody set them. A SUP is not planned over
			// a day somebody else set, so it is not counted there.
			Planned: len(keep) + keptByHand, KeptByHand: keptByHand, LieuPlanned: len(plan.Lieu),
		},
	}
	for _, t := range keep {
		out.Turns = append(out.Turns, domain.GeneratedRotaTurn{
			UserID: t.UserID, Name: names[t.UserID], TeamKey: t.TeamKey,
			ShiftCode: t.ShiftCode, Tier: t.Tier, RotaDate: t.RotaDate,
		})
	}
	for _, l := range plan.Lieu {
		out.LieuLeave = append(out.LieuLeave, domain.GeneratedLieuLeave{
			UserID: l.UserID, Name: names[l.UserID], TeamKey: l.TeamKey, StartsOn: l.From, EndsOn: l.To,
		})
	}
	for _, w := range plan.Warnings {
		rw := domain.RotaGenerationWarning{Message: w.Message}
		if w.Date != "" {
			d := w.Date
			rw.Date = &d
		}
		out.Warnings = append(out.Warnings, rw)
	}

	if req.DryRun {
		return out, nil
	}
	if alreadyGenerated && !req.Regenerate {
		return out, &apierror.ConflictError{Msg: fmt.Sprintf(
			"%s has already been generated; regenerate to replace the generated turns from %s on (turns set by hand are kept)",
			out.Month, out.From)}
	}

	write := repository.RotaMonthWrite{
		ActorEmail: actor,
		Owner:      owner,
		TeamKeys:   teams,
		ShiftCodes: saasWrittenShiftCodes(),
		From:       saasISO(from),
		To:         saasISO(monthEnd),
		// The month's TZ3 people, recorded with the month so the roster can
		// list them under the night cover; nobody chosen clears the record.
		RotaCode: RotaSRESaaS,
		Month:    saasISO(monthStart),
	}
	for _, team := range teams {
		for _, id := range crew[team] {
			write.NightCrew = append(write.NightCrew, repository.RotaNightCrewRow{UserID: id, TeamKey: team})
		}
	}
	for note := range ownedNotes {
		write.LieuNotes = append(write.LieuNotes, note)
	}
	sort.Strings(write.LieuNotes)
	for _, t := range keep {
		write.Turns = append(write.Turns, repository.RotaTurnRow{
			UserID: t.UserID, TeamKey: t.TeamKey, ShiftCode: t.ShiftCode, Tier: t.Tier, RotaDate: t.RotaDate,
		})
	}
	for _, l := range plan.Lieu {
		write.Lieu = append(write.Lieu, repository.RotaLieuRow{
			UserID: l.UserID, TeamKey: l.TeamKey, From: l.From, To: l.To, Note: saasLieuNote(l.Weekend),
		})
	}
	res, err := s.writer.ReplaceGeneratedMonth(ctx, write)
	if err != nil {
		return out, err
	}
	out.Summary.Written = res.Written
	out.Summary.Replaced = res.Replaced
	out.Summary.LieuWritten = res.LieuWritten
	return out, nil
}

// NightCrew implements RotaGenerateService. Anyone on the team schedule may
// read it, as they may read the rota itself.
func (s *rotaGenerateService) NightCrew(ctx context.Context, rotaCode, fromMonth, toMonth string) (domain.RotaNightCrewResponse, error) {
	var out domain.RotaNightCrewResponse
	if err := RequireInternalCaller(ctx, s.access, "the team schedule is only available to internal staff"); err != nil {
		return out, err
	}
	if !strings.EqualFold(rotaCode, RotaSRESaaS) {
		return out, &apierror.NotFoundError{Msg: fmt.Sprintf("rota %q has no night crew; only %s has", rotaCode, RotaSRESaaS)}
	}
	from, err := time.Parse("2006-01", strings.TrimSpace(fromMonth))
	if err != nil {
		return out, &apierror.ValidationError{Msg: "from must be YYYY-MM"}
	}
	to, err := time.Parse("2006-01", strings.TrimSpace(toMonth))
	if err != nil {
		return out, &apierror.ValidationError{Msg: "to must be YYYY-MM"}
	}
	if to.Before(from) {
		return out, &apierror.ValidationError{Msg: "to must not be before from"}
	}
	if to.After(from.AddDate(0, rotaNightCrewMaxMonths-1, 0)) {
		return out, &apierror.ValidationError{Msg: fmt.Sprintf("at most %d months can be read at once", rotaNightCrewMaxMonths)}
	}
	rows, err := s.writer.NightCrew(ctx, RotaSRESaaS, saasISO(from), saasISO(to))
	if err != nil {
		return out, err
	}
	byMonth := map[string][]domain.RotaNightCrewMember{}
	for _, r := range rows {
		m := r.Month[:7]
		byMonth[m] = append(byMonth[m], domain.RotaNightCrewMember{UserID: r.UserID, TeamKey: r.TeamKey})
	}
	out.RotaCode = RotaSRESaaS
	for m := from; !m.After(to); m = m.AddDate(0, 1, 0) {
		key := m.Format("2006-01")
		crew := byMonth[key]
		if crew == nil {
			crew = []domain.RotaNightCrewMember{}
		}
		out.Months = append(out.Months, domain.RotaNightCrewMonth{Month: key, Crew: crew})
	}
	return out, nil
}

// requireGenerator is the rota-writer rule for the rota as a whole: a lead of
// any of its teams, or an admin of the rota's family for any of them.
func (s *rotaGenerateService) requireGenerator(ctx context.Context, actor string, teams []string) error {
	for _, team := range teams {
		ok, err := s.schedule.LeadsTeam(ctx, actor, team)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
	}
	admin, err := s.schedule.RotaAdminTeamsFor(ctx, actor)
	if err != nil {
		return err
	}
	for _, team := range teams {
		if containsFold(admin, team) {
			return nil
		}
	}
	return &apierror.ForbiddenError{Msg: "generating the SaaS rota needs a lead of one of its teams, or an SRE rota admin"}
}

func (s *rotaGenerateService) today() time.Time {
	now := s.now()
	if loc, err := time.LoadLocation(rotaAuthoringZone); err == nil {
		now = now.In(loc)
	} else {
		slog.Warn("rota generator: zone not found, counting today in UTC", "zone", rotaAuthoringZone, "error", err)
	}
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}

// saasRotaTeams is the rota's teams, sorted, and their members.
func saasRotaTeams(cat domain.ScheduleCatalogue) ([]string, []saasRotaMember) {
	var teams []string
	var members []saasRotaMember
	for _, t := range cat.Teams {
		if t.RotaCode == nil || !strings.EqualFold(*t.RotaCode, RotaSRESaaS) {
			continue
		}
		teams = append(teams, t.Key)
		for _, m := range t.Members {
			members = append(members, saasRotaMember{
				UserID: m.UserID, Name: m.Name, Email: strings.ToLower(m.Email), TeamKey: t.Key,
				IsLead: m.Role == "lead",
			})
		}
	}
	sort.Strings(teams)
	return teams, members
}

// saasShiftWindows reads the SaaS shifts' windows from the catalogue, and
// refuses to work against one that lacks any of them. dayNight is the weekend
// shape the catalogue offers: Day/Night when both SRE_WE_DAY and SRE_WE_NIGHT
// are in it (the catalogue serves only active windows, so that is once they
// are switched on), else TZ1+2 / TZ3 on SRE_WE_TZ1. A catalogue with neither
// weekend shape is refused.
func saasShiftWindows(cat domain.ScheduleCatalogue) (windows map[string][2]int, dayNight bool, err error) {
	out := map[string][2]int{}
	for _, sh := range cat.Shifts {
		out[sh.Code] = [2]int{sh.StartMinute, sh.EndMinute}
	}
	for _, code := range saasRotaShiftCodes {
		if _, ok := out[code]; !ok {
			return nil, false, &apierror.ServiceUnavailableError{Msg: fmt.Sprintf("the catalogue has no %s shift", code)}
		}
	}
	_, day := out[saasShiftWeekendDay]
	_, night := out[saasShiftWeekendNight]
	if day && night {
		return out, true, nil
	}
	if _, ok := out[saasShiftWeekend]; ok {
		return out, false, nil
	}
	return nil, false, &apierror.ServiceUnavailableError{Msg: fmt.Sprintf(
		"the catalogue has no weekend shifts: neither %s nor both %s and %s",
		saasShiftWeekend, saasShiftWeekendDay, saasShiftWeekendNight)}
}

func hasAbsenceKind(cat domain.ScheduleCatalogue, code string) bool {
	for _, k := range cat.AbsenceKinds {
		if k.Code == code {
			return true
		}
	}
	return false
}

// saasWrittenShiftCodes is the shifts the generator writes turns on: the
// bound of what a regeneration may remove.
// saasWrittenShiftCodes is every shift a regeneration may replace the
// generator's own rows of: its turns and the SUP (regular hours) it writes.
// Only rows it owns are replaced, so a lead's own marks are never touched.
func saasWrittenShiftCodes() []string {
	return append(saasTurnShiftCodes(), saasRegularShiftCodes()...)
}

func saasTurnShiftCodes() []string {
	// Both weekend shapes: a regeneration after the switch to Day/Night must
	// clear the month's old SRE_WE_TZ1 and weekend SRE_TZ3 turns as well.
	return []string{saasShiftTZ1L1, saasShiftTZ1, saasShiftTZ2L1, saasShiftTZ2, saasShiftTZ3,
		saasShiftWeekend, saasShiftWeekendDay, saasShiftWeekendNight}
}

func saasRegularShiftCodes() []string {
	return []string{saasShiftTZ1Regular, saasShiftTZ2Regular, saasShiftTZ3Regular}
}

func saasIsRegularShift(code string) bool {
	for _, c := range saasRegularShiftCodes() {
		if c == code {
			return true
		}
	}
	return false
}

func saasIsRotaShift(code string) bool {
	for _, c := range saasTurnShiftCodes() {
		if c == code {
			return true
		}
	}
	return false
}

func saasLieuNote(saturday string) string {
	return saasLieuNotePrefix + " " + saturday
}

// saasOwnedLieuNotes names the weekends that have a day written from `from`
// to the end of the month: their lieu leave is this run's.
func saasOwnedLieuNotes(from, monthEnd time.Time) map[string]bool {
	out := map[string]bool{}
	for d := from; !d.After(monthEnd); d = d.AddDate(0, 0, 1) {
		switch saasDow(d) {
		case 5:
			out[saasLieuNote(saasISO(d))] = true
		case 6:
			out[saasLieuNote(saasISO(d.AddDate(0, 0, -1)))] = true
		}
	}
	return out
}

// saasNightCrew checks the TZ3 people a lead chose: each key a team on the
// rota, each id a member of that team, named once. Blank ids and empty lists
// are dropped. Every team's choices together are the rota's one TZ3 crew;
// with nobody chosen on any team, nights are drawn as usual.
func saasNightCrew(in map[string][]string, teams []string, members []saasRotaMember) (map[string][]string, error) {
	if len(in) == 0 {
		return nil, nil
	}
	onRota := map[string]bool{}
	for _, t := range teams {
		onRota[t] = true
	}
	teamOf := map[string]string{}
	for _, m := range members {
		if _, seen := teamOf[m.UserID]; !seen {
			teamOf[m.UserID] = m.TeamKey
		}
	}
	out := map[string][]string{}
	for team, ids := range in {
		if !onRota[team] {
			return nil, &apierror.ValidationError{Msg: fmt.Sprintf("nightCrew: %q is not a team on the %s rota", team, RotaSRESaaS)}
		}
		seen := map[string]bool{}
		for _, id := range ids {
			id = strings.TrimSpace(id)
			if id == "" || seen[id] {
				continue
			}
			if teamOf[id] != team {
				return nil, &apierror.ValidationError{Msg: fmt.Sprintf("nightCrew: %s is not a member of %s", id, team)}
			}
			seen[id] = true
			out[team] = append(out[team], id)
		}
	}
	return out, nil
}

// saasMovesWithinRota is the tags that move somebody to another team of
// this rota -- the SaaS Night cover -- and the team each moves them to. Such
// a span is where they work, not time away from the rota.
func saasMovesWithinRota(cat domain.ScheduleCatalogue, teams []string) map[string]string {
	onRota := map[string]bool{}
	for _, t := range teams {
		onRota[strings.ToLower(t)] = true
	}
	out := map[string]string{}
	for _, k := range cat.AbsenceKinds {
		if k.MovesToTeamKey == nil || !k.WorksRotaThere || !onRota[strings.ToLower(*k.MovesToTeamKey)] {
			continue
		}
		out[k.Code] = *k.MovesToTeamKey
	}
	return out
}

// saasCrewFromMoves is the TZ3 crew a month gets when nobody was chosen: every
// member a span moves onto a team of the rota for a day of [from, to], under
// their own team, in the members' order.
func saasCrewFromMoves(members []saasRotaMember, nightTeam map[string]map[string]string, from, to time.Time) map[string][]string {
	var out map[string][]string
	seen := map[string]bool{}
	for _, m := range members {
		if seen[m.UserID] || len(nightTeam[m.UserID]) == 0 {
			continue
		}
		for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
			if nightTeam[m.UserID][saasISO(d)] == "" {
				continue
			}
			if out == nil {
				out = map[string][]string{}
			}
			out[m.TeamKey] = append(out[m.TeamKey], m.UserID)
			seen[m.UserID] = true
			break
		}
	}
	return out
}

// saasClashesWithHand is true when a lead already filled this turn's slot by
// hand (same team, day, shift and tier -- any team on TZ3, which is one crew
// for the whole rota), or the person already holds a turn by hand that
// overlaps it.
func saasClashesWithHand(t saasPlannedTurn, byHand []domain.ScheduleAssignment, windows map[string][2]int) bool {
	w := windows[t.ShiftCode]
	for _, a := range byHand {
		if a.RotaDate != t.RotaDate {
			continue
		}
		tier := ""
		if a.Tier != nil {
			tier = *a.Tier
		}
		sameSlot := a.TeamKey == t.TeamKey || saasSharedShift(t.ShiftCode)
		if sameSlot && a.ShiftCode == t.ShiftCode && tier == t.Tier {
			return true
		}
		if a.Engineer.UserID == t.UserID {
			if o, ok := windows[a.ShiftCode]; ok && o[0] < w[1] && w[0] < o[1] {
				return true
			}
		}
	}
	return false
}
