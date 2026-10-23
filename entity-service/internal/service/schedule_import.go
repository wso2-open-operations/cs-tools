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
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// rotaImportFamily is the one family a rota import is accepted for today.
const rotaImportFamily = "SRE"

// rotaImportMaxEntries bounds one file: a month of a busy rota written as ranges is a few hundred entries.
const rotaImportMaxEntries = 1000

// rotaImportMaxProblems is how many file problems one refusal lists before it stops counting.
const rotaImportMaxProblems = 25

// ImportRota implements ScheduleService.
func (s *scheduleService) ImportRota(ctx context.Context, rotaCode string, req domain.ImportRotaRequest, dryRun bool) (domain.ImportRotaResponse, error) {
	var out domain.ImportRotaResponse
	if err := s.requireInternalCaller(ctx); err != nil {
		return out, err
	}
	actor := auth.IdentityFromContext(ctx).UserEmail
	if actor == "" {
		return out, &apierror.ForbiddenError{Msg: "importing a rota needs a user token, not a service credential"}
	}

	cat, err := s.repo.Catalogue(ctx)
	if err != nil {
		return out, err
	}
	plan, err := planRotaImport(cat, rotaCode, req)
	if err != nil {
		return out, err
	}

	checks := newImportWriterChecks(s)
	for i := range plan.write.Assignments {
		a := &plan.write.Assignments[i]
		if _, err := checks.overSpan(ctx, a.TeamKey, a.UserID, a.From, a.To, false); err != nil {
			return out, apierror.Prefixed(fmt.Sprintf("assignments[%d]", i), err)
		}
	}
	for i := range plan.write.Absences {
		a := &plan.write.Absences[i]
		home, err := checks.overSpan(ctx, a.TeamKey, a.UserID, a.From, a.To, true)
		if err != nil {
			return out, apierror.Prefixed(fmt.Sprintf("absences[%d]", i), err)
		}
		a.HomeTeamKey = home
	}
	// A replace clears every team of the rota, so the caller must be able to write each of them.
	if plan.write.Replace {
		for _, team := range plan.write.TeamKeys {
			if err := checks.writer(ctx, team); err != nil {
				return out, err
			}
		}
	}

	plan.write.DryRun = dryRun
	res, err := s.repo.ImportRota(ctx, plan.write, actor)
	if err != nil {
		return out, err
	}

	out = domain.ImportRotaResponse{
		RotaCode:         plan.write.RotaCode,
		From:             plan.write.From,
		To:               plan.write.To,
		Mode:             plan.mode,
		DryRun:           dryRun,
		Cleared:          res.Cleared,
		LeaveWrittenOver: res.LeaveWrittenOver,
		SkippedDays:      []domain.ImportRotaSkippedDays{},
	}
	for i, r := range res.Assignments {
		out.Applied += r.Applied
		out.Skipped += r.Skipped
		if len(r.SkippedDates) > 0 {
			out.SkippedDays = append(out.SkippedDays, domain.ImportRotaSkippedDays{Index: i, Dates: r.SkippedDates})
		}
	}
	for _, r := range res.Absences {
		out.AbsencesCreated += r.Created
		out.AbsencesRemoved += r.Removed
		out.AbsencesTrimmed += r.Trimmed
	}
	return out, nil
}

// importWriterChecks answers the picker's writer rule once per team and once per person and team, not once per entry.
type importWriterChecks struct {
	s      *scheduleService
	teams  map[string]error
	member map[[2]string]bool
}

func newImportWriterChecks(s *scheduleService) *importWriterChecks {
	return &importWriterChecks{s: s, teams: map[string]error{}, member: map[[2]string]bool{}}
}

// writer is requireRotaWriter for one team, asked once.
func (c *importWriterChecks) writer(ctx context.Context, team string) error {
	key := strings.ToLower(team)
	if err, ok := c.teams[key]; ok {
		return err
	}
	err := c.s.requireRotaWriter(ctx, team)
	c.teams[key] = err
	return err
}

// overSpan is requireRotaWriterOverSpan: a member of the team is the writer rule for the team, and anyone else goes through the full rule (a move).
func (c *importWriterChecks) overSpan(ctx context.Context, team, userID, from, to string, homeLeadMayManage bool) (string, error) {
	key := [2]string{strings.ToLower(team), userID}
	member, ok := c.member[key]
	if !ok {
		m, err := c.s.repo.UserInTeam(ctx, userID, team)
		if err != nil {
			return "", err
		}
		c.member[key], member = m, m
	}
	if !member {
		return c.s.requireRotaWriterOverSpan(ctx, team, userID, from, to, homeLeadMayManage)
	}
	return team, c.writer(ctx, team)
}

// rotaImportPlan is a file checked against the catalogue and turned into the writes it stands for.
type rotaImportPlan struct {
	mode  string
	write repository.RotaImportWrite
}

// planRotaImport checks the whole file against the catalogue and lists every problem it finds, not just the first.
func planRotaImport(cat domain.ScheduleCatalogue, rotaCode string, req domain.ImportRotaRequest) (rotaImportPlan, error) {
	var plan rotaImportPlan
	var problems []string
	problem := func(format string, args ...any) {
		problems = append(problems, fmt.Sprintf(format, args...))
	}

	if req.SchemaVersion != domain.RotaImportSchemaVersion {
		return plan, &apierror.ValidationError{Msg: fmt.Sprintf("schemaVersion must be %d", domain.RotaImportSchemaVersion)}
	}
	code := strings.ToUpper(strings.TrimSpace(req.RotaCode))
	if !strings.EqualFold(code, strings.TrimSpace(rotaCode)) {
		return plan, &apierror.ValidationError{Msg: fmt.Sprintf("rotaCode %q does not match the rota being imported (%s)", req.RotaCode, rotaCode)}
	}
	var rota *domain.ScheduleRota
	for i := range cat.Rotas {
		if strings.EqualFold(cat.Rotas[i].Code, code) {
			rota = &cat.Rotas[i]
		}
	}
	if rota == nil {
		return plan, &apierror.NotFoundError{Msg: fmt.Sprintf("no rota %q", code)}
	}
	if !strings.EqualFold(rota.Family, rotaImportFamily) {
		return plan, &apierror.ValidationError{Msg: fmt.Sprintf("only %s rotas can be imported; %s is a %s rota", rotaImportFamily, rota.Code, rota.Family)}
	}
	if !strings.EqualFold(strings.TrimSpace(req.Family), rota.Family) {
		return plan, &apierror.ValidationError{Msg: fmt.Sprintf("family must be %s for %s", rota.Family, rota.Code)}
	}

	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	if mode != domain.RotaImportReplace && mode != domain.RotaImportMerge {
		return plan, &apierror.ValidationError{Msg: fmt.Sprintf("mode must be %q or %q", domain.RotaImportReplace, domain.RotaImportMerge)}
	}
	from, to, err := parseImportSpan(req.Period.From, req.Period.To)
	if err != nil {
		return plan, &apierror.ValidationError{Msg: "period: " + err.Error()}
	}
	if to.Sub(from) > 366*24*time.Hour {
		return plan, &apierror.ValidationError{Msg: "period: longer than a year"}
	}
	total := len(req.Assignments) + len(req.Absences)
	if total == 0 {
		return plan, &apierror.ValidationError{Msg: "the file has no assignments and no absences"}
	}
	if total > rotaImportMaxEntries {
		return plan, &apierror.ValidationError{Msg: fmt.Sprintf("the file has %d entries; at most %d can be imported at once", total, rotaImportMaxEntries)}
	}

	// The rota's own teams, its zones, and everyone the catalogue knows by email.
	teams := map[string]string{}
	var teamKeys []string
	for _, t := range cat.Teams {
		if t.RotaCode != nil && strings.EqualFold(*t.RotaCode, rota.Code) {
			teams[strings.ToLower(t.Key)] = t.Key
			teamKeys = append(teamKeys, t.Key)
		}
	}
	sort.Strings(teamKeys)
	if len(teamKeys) == 0 {
		return plan, &apierror.ServiceUnavailableError{Msg: fmt.Sprintf("no team is on the %s rota", rota.Code)}
	}
	zones := map[string]bool{}
	for _, z := range cat.Zones {
		if z.RotaCode != nil && strings.EqualFold(*z.RotaCode, rota.Code) {
			zones[z.Code] = true
		}
	}
	shifts := map[string]domain.ScheduleShift{}
	for _, sh := range cat.Shifts {
		if strings.EqualFold(sh.Family, rota.Family) && sh.ZoneCode != nil && zones[*sh.ZoneCode] {
			shifts[sh.Code] = sh
		}
	}
	kinds := map[string]domain.ScheduleAbsenceKind{}
	for _, k := range cat.AbsenceKinds {
		if !k.Retired && (k.Family == nil || strings.EqualFold(*k.Family, rota.Family)) {
			kinds[k.Code] = k
		}
	}
	people := map[string]string{}
	ambiguous := map[string]bool{}
	for _, t := range cat.Teams {
		for _, m := range t.Members {
			email := strings.ToLower(strings.TrimSpace(m.Email))
			if email == "" {
				continue
			}
			if id, ok := people[email]; ok && id != m.UserID {
				ambiguous[email] = true
			}
			people[email] = m.UserID
		}
	}
	person := func(where, email string) string {
		e := strings.ToLower(strings.TrimSpace(email))
		switch {
		case e == "":
			problem("%s: email is required", where)
		case ambiguous[e]:
			problem("%s: %s belongs to more than one user", where, email)
		case people[e] == "":
			problem("%s: nobody on the team schedule has the email %s", where, email)
		default:
			return people[e]
		}
		return ""
	}
	team := func(where, key string) string {
		k, ok := teams[strings.ToLower(strings.TrimSpace(key))]
		if !ok {
			problem("%s: %q is not a team on the %s rota", where, key, rota.Code)
		}
		return k
	}

	plan.mode = mode
	plan.write = repository.RotaImportWrite{
		RotaCode: rota.Code,
		TeamKeys: teamKeys,
		From:     from.Format(time.DateOnly),
		To:       to.Format(time.DateOnly),
		Replace:  mode == domain.RotaImportReplace,
	}

	for i, a := range req.Assignments {
		where := fmt.Sprintf("assignments[%d]", i)
		w := domain.ApplyScheduleRangeRequest{
			UserID:    person(where, a.Email),
			TeamKey:   team(where, a.TeamKey),
			ShiftCode: strings.ToUpper(strings.TrimSpace(a.ShiftCode)),
			Note:      trimmedOrNil(a.Note),
		}
		if _, ok := shifts[w.ShiftCode]; !ok {
			problem("%s: %q is not a window of the %s rota", where, a.ShiftCode, rota.Code)
		}
		if a.Tier != nil && strings.TrimSpace(*a.Tier) != "" {
			t := strings.ToUpper(strings.TrimSpace(*a.Tier))
			if t != "L1" && t != "L2" && t != "L3" {
				problem("%s: tier must be L1, L2 or L3", where)
			}
			w.Tier = &t
		}
		af, at, err := parseImportSpan(a.From, a.To)
		switch {
		case err != nil:
			problem("%s: %s", where, err)
		case af.Before(from) || at.After(to):
			problem("%s: %s to %s is outside the period %s to %s", where, a.From, a.To, plan.write.From, plan.write.To)
		default:
			w.From, w.To = af.Format(time.DateOnly), at.Format(time.DateOnly)
		}
		plan.write.Assignments = append(plan.write.Assignments, w)
	}

	for i, a := range req.Absences {
		where := fmt.Sprintf("absences[%d]", i)
		w := domain.ApplyScheduleAbsenceRequest{
			UserID:      person(where, a.Email),
			TeamKey:     team(where, a.TeamKey),
			KindCode:    strings.ToUpper(strings.TrimSpace(a.KindCode)),
			Note:        trimmedOrNil(a.Note),
			AllocatedTo: trimmedOrNil(a.AllocatedTo),
		}
		if _, ok := kinds[w.KindCode]; !ok {
			problem("%s: %q is not a kind of time away offered on the %s rota", where, a.KindCode, rota.Family)
		}
		if w.AllocatedTo != nil && utf8.RuneCountInString(*w.AllocatedTo) > maxAllocatedToLength {
			problem("%s: allocatedTo is at most %d characters", where, maxAllocatedToLength)
		}
		af, at, err := parseImportSpan(a.From, a.To)
		switch {
		case err != nil:
			problem("%s: %s", where, err)
		case at.Before(from) || af.After(to):
			problem("%s: %s to %s does not overlap the period %s to %s", where, a.From, a.To, plan.write.From, plan.write.To)
		default:
			w.From, w.To = af.Format(time.DateOnly), at.Format(time.DateOnly)
		}
		plan.write.Absences = append(plan.write.Absences, w)
	}

	// Entries that would replace one another are only looked for once every entry reads cleanly on its own.
	if len(problems) == 0 {
		problems = append(problems, assignmentOverlaps(plan.write.Assignments, shifts)...)
		problems = append(problems, absenceOverlaps(plan.write.Absences, kinds)...)
	}
	if len(problems) > 0 {
		more := ""
		if len(problems) > rotaImportMaxProblems {
			more = fmt.Sprintf("; and %d more", len(problems)-rotaImportMaxProblems)
			problems = problems[:rotaImportMaxProblems]
		}
		return rotaImportPlan{}, &apierror.ValidationError{Msg: "the rota file has problems: " + strings.Join(problems, "; ") + more}
	}
	return plan, nil
}

// worksOn says whether a window is worked on a day, by its day scope.
func worksOn(sh domain.ScheduleShift, d time.Time) bool {
	weekend := d.Weekday() == time.Saturday || d.Weekday() == time.Sunday
	switch sh.DayScope {
	case "WEEKDAY":
		return !weekend
	case "WEEKEND":
		return weekend
	}
	return true
}

// displaces says whether applying b after a on one day replaces a, by the picker's own rule: a turn replaces turns in its zone or at its hours, regular hours replace regular hours.
func displaces(a, b domain.ScheduleShift) bool {
	switch {
	case a.IsRotation && b.IsRotation:
		sameZone := a.ZoneCode != nil && b.ZoneCode != nil && *a.ZoneCode == *b.ZoneCode
		return sameZone || (a.StartMinute < b.EndMinute && b.StartMinute < a.EndMinute)
	case !a.IsRotation && !b.IsRotation:
		return true
	}
	return false
}

// assignmentOverlaps names every pair of entries where a later one would silently replace an earlier one for the same person and team, with the first day it happens.
func assignmentOverlaps(entries []domain.ApplyScheduleRangeRequest, shifts map[string]domain.ScheduleShift) []string {
	byDay := map[string][]int{}
	for i, e := range entries {
		f, t, err := parseImportSpan(e.From, e.To)
		if err != nil {
			continue
		}
		for d := f; !d.After(t); d = d.AddDate(0, 0, 1) {
			if worksOn(shifts[e.ShiftCode], d) {
				key := e.UserID + "|" + strings.ToLower(e.TeamKey) + "|" + d.Format(time.DateOnly)
				byDay[key] = append(byDay[key], i)
			}
		}
	}
	first := map[[2]int]string{}
	for key, idx := range byDay {
		day := key[strings.LastIndex(key, "|")+1:]
		for x := 0; x < len(idx); x++ {
			for y := x + 1; y < len(idx); y++ {
				i, j := idx[x], idx[y]
				if !displaces(shifts[entries[i].ShiftCode], shifts[entries[j].ShiftCode]) {
					continue
				}
				if d, ok := first[[2]int{i, j}]; !ok || day < d {
					first[[2]int{i, j}] = day
				}
			}
		}
	}
	return overlapMessages(first, func(i, j int, day string) string {
		return fmt.Sprintf("assignments[%d] overlaps assignments[%d]: the same person on %s and %s on %s, and the later entry would replace the earlier",
			j, i, entries[i].ShiftCode, entries[j].ShiftCode, day)
	})
}

// absenceOverlaps names every pair of spans for one person where a later one would shorten or split an earlier one; time away inside a move listed before it is how the roster cuts a move, so that is allowed.
func absenceOverlaps(entries []domain.ApplyScheduleAbsenceRequest, kinds map[string]domain.ScheduleAbsenceKind) []string {
	isMove := func(e domain.ApplyScheduleAbsenceRequest) bool { return kinds[e.KindCode].MovesToTeamKey != nil }
	first := map[[2]int]string{}
	for i := range entries {
		for j := i + 1; j < len(entries); j++ {
			a, b := entries[i], entries[j]
			if a.UserID != b.UserID || a.From > b.To || b.From > a.To {
				continue
			}
			if isMove(a) && !isMove(b) {
				continue
			}
			first[[2]int{i, j}] = max(a.From, b.From)
		}
	}
	return overlapMessages(first, func(i, j int, day string) string {
		hint := "and the later entry would shorten or split the earlier"
		if isMove(entries[j]) && !isMove(entries[i]) {
			hint = "list the move first, so the time away inside it is kept"
		}
		return fmt.Sprintf("absences[%d] overlaps absences[%d]: the same person from %s; %s", j, i, day, hint)
	})
}

// overlapMessages renders overlapping pairs in file order.
func overlapMessages(pairs map[[2]int]string, render func(i, j int, day string) string) []string {
	keys := make([][2]int, 0, len(pairs))
	for k := range pairs {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(a, b int) bool {
		if keys[a][1] != keys[b][1] {
			return keys[a][1] < keys[b][1]
		}
		return keys[a][0] < keys[b][0]
	})
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, render(k[0], k[1], pairs[k]))
	}
	return out
}

// parseImportSpan reads a from..to pair of YYYY-MM-DD days, both inclusive.
func parseImportSpan(from, to string) (time.Time, time.Time, error) {
	f, err := time.Parse(time.DateOnly, strings.TrimSpace(from))
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("from must be YYYY-MM-DD")
	}
	t, err := time.Parse(time.DateOnly, strings.TrimSpace(to))
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("to must be YYYY-MM-DD")
	}
	if t.Before(f) {
		return time.Time{}, time.Time{}, fmt.Errorf("to is before from")
	}
	return f, t, nil
}

// trimmedOrNil is a trimmed copy of an optional text, or nil when it is absent or blank.
func trimmedOrNil(v *string) *string {
	if v == nil || strings.TrimSpace(*v) == "" {
		return nil
	}
	t := strings.TrimSpace(*v)
	return &t
}
