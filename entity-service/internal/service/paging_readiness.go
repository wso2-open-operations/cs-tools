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
	"strconv"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// Paging readiness: would each Case Paging chain reach someone over the next
// days? The answer is read, never stored: the memberships behind the CRE
// chain, and the rota behind the SRE and SME ones. computePagingReadiness is
// the whole of the rule, a pure function over what the repository gathered,
// so it is tested without a database.
//
// It reports, per person, whether they have a callable number on their own
// profile and a paging-only number (and how its last test went), but raises
// no phone gap: the CSM portal backend turns those flags into gaps. What it
// does not check: a Chat account, and whether a night window's small hours,
// which fall on the next calendar day, are covered by someone away that next
// day. Absence is matched by rota date.

const (
	pagingReadinessDefaultDays = 7
	pagingReadinessMaxDays     = 14

	readinessError   = "error"
	readinessWarning = "warning"

	fixResponders = "responders"
	fixRota       = "rota"
	fixConfig     = "config"
	fixData       = "data"

	// The SRE rotas, as migration 0200 seeds them.
	rotaSRESaaS = "SRE_SAAS"
	rotaSREIaaS = "SRE_IAAS"
	// U2 and B-Central's rotas keep the codes migration 0217 gave them as SME
	// rotas; migration 0242 moved them to the SRE family.
	rotaSREU2       = "SME_U2"
	rotaSREBCentral = "SME_B_CENTRAL"
)

// sreGroupChains are the SRE chains named after an assignment group rather
// than a sub-team: a chain of its own once its rota is an SRE rota, and none
// while it is still an SME one (where the SME chain already covers it).
var sreGroupChains = []struct{ chain, rota, label string }{
	{"SRE_U2", rotaSREU2, "U2-SRE chain"},
	{"SRE_B_CENTRAL", rotaSREBCentral, "B-Central-SRE chain"},
}

// rotaLocation is the clock rota dates are written in (the shifts'
// authoring time zone), so "today" is the rota's today.
var rotaLocation = func() *time.Location {
	if loc, err := time.LoadLocation("Asia/Colombo"); err == nil {
		return loc
	}
	return time.FixedZone("+0530", 5*3600+30*60)
}()

// GetPagingReadiness implements PagingChainService.
func (s *pagingChainService) GetPagingReadiness(ctx context.Context, daysParam string) (domain.PagingReadinessResponse, error) {
	if err := RequireInternalCaller(ctx, s.access, "alert readiness is only available to internal staff"); err != nil {
		return domain.PagingReadinessResponse{}, err
	}
	days := pagingReadinessDefaultDays
	if raw := strings.TrimSpace(daysParam); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > pagingReadinessMaxDays {
			return domain.PagingReadinessResponse{}, &apierror.ValidationError{
				Msg: fmt.Sprintf("days must be a whole number from 1 to %d", pagingReadinessMaxDays),
			}
		}
		days = n
	}
	now := s.now()
	local := now.In(rotaLocation)
	from := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 0, days-1)
	facts, err := s.repo.ReadinessFacts(ctx, from.Format(time.DateOnly), to.Format(time.DateOnly))
	if err != nil {
		return domain.PagingReadinessResponse{}, err
	}
	return computePagingReadiness(facts, s.handoff, from, days, now), nil
}

// computePagingReadiness is the readiness rule. from is a calendar date (its
// clock is ignored); days counts from it.
func computePagingReadiness(f repository.PagingReadinessFacts, handoff *SpecialistHandoffConfig, from time.Time, days int, now time.Time) domain.PagingReadinessResponse {
	dates := make([]time.Time, days)
	for i := range dates {
		dates[i] = time.Date(from.Year(), from.Month(), from.Day()+i, 0, 0, 0, 0, time.UTC)
	}
	resp := domain.PagingReadinessResponse{
		GeneratedAt: now.UTC().Format(time.RFC3339),
		From:        dates[0].Format(time.DateOnly),
		To:          dates[len(dates)-1].Format(time.DateOnly),
		Chains: []domain.PagingReadinessChain{
			finishChain(creReadiness(f, dates)),
			finishChain(sreReadiness(f, rotaSRESaaS, rotaSRESaaS, "SaaS SRE chain", dates)),
			finishChain(sreReadiness(f, rotaSREIaaS, rotaSREIaaS, "IaaS SRE chain", dates)),
		},
	}
	for _, g := range sreGroupChains {
		if rotaFamily(f, g.rota) == familySRE {
			resp.Chains = append(resp.Chains, finishChain(sreReadiness(f, g.chain, g.rota, g.label, dates)))
		}
	}
	resp.Chains = append(resp.Chains, finishChain(smeReadiness(f, handoff, dates)))
	// Whether each person has a number on their own profile, a paging-only
	// number, and how the latter's last test went. Never the numbers.
	for ci := range resp.Chains {
		for pi := range resp.Chains[ci].People {
			p := &resp.Chains[ci].People[pi]
			p.HasProfilePhone = p.UserID != "" && f.ProfilePhones[p.UserID]
			if status, ok := f.PagingPhones[p.UserID]; ok && p.UserID != "" {
				p.HasPagingPhone = true
				if status != "" {
					st := status
					p.PagingPhoneLastTestStatus = &st
				}
			}
		}
	}
	return resp
}

// finishChain sets Ready -- no gap of severity error -- and makes the two
// lists empty rather than null.
func finishChain(c domain.PagingReadinessChain) domain.PagingReadinessChain {
	if c.Gaps == nil {
		c.Gaps = []domain.PagingReadinessGap{}
	}
	if c.People == nil {
		c.People = []domain.PagingReadinessPerson{}
	}
	c.Ready = true
	for _, g := range c.Gaps {
		if g.Severity == readinessError {
			c.Ready = false
		}
	}
	return c
}

// crePosition is a position someone holds on the CRE chain.
type crePosition struct {
	member    repository.PagingReadinessMember
	role      string
	teamKey   string
	responder bool
}

// creReadiness checks the CRE chain's standing positions: each ABT's and the
// Americas team's T1-T3 responders, each ABT's Team lead, the America lead
// and the two heads, and whether anyone holding one is away in the range
// (ON_LEAVE). It also warns about accounts whose CRE team is no ABT, whose
// cases would page no ABT's responders.
func creReadiness(f repository.PagingReadinessFacts, dates []time.Time) domain.PagingReadinessChain {
	c := domain.PagingReadinessChain{Chain: familyCRE, Label: "CRE chain"}
	byTeam := map[string][]repository.PagingReadinessMember{}
	heads := map[string][]repository.PagingReadinessMember{}
	for _, m := range f.CREMembers {
		if m.Role == memberRoleCREHead || m.Role == memberRoleCSHead {
			heads[m.Role] = append(heads[m.Role], m)
			continue
		}
		byTeam[strings.ToLower(m.TeamKey)] = append(byTeam[strings.ToLower(m.TeamKey)], m)
	}

	var positions []crePosition
	// person records someone the chain pages, with the tier that first calls
	// them (see crePagingTier) and whether their number may be missing.
	person := func(m repository.PagingReadinessMember, role string, tier int, phoneOptional bool) {
		c.People = append(c.People, domain.PagingReadinessPerson{
			UserID: m.UserID, Email: m.Email, Name: m.Name, Role: role,
			PagingTier: tier, PhoneOptional: phoneOptional,
		})
		positions = append(positions, crePosition{member: m, role: role, teamKey: m.TeamKey, responder: m.AlertTier != ""})
	}
	for _, role := range []string{memberRoleCREHead, memberRoleCSHead} {
		label := map[string]string{memberRoleCREHead: "CRE head", memberRoleCSHead: "CS head"}[role]
		tier := map[string]int{memberRoleCREHead: crePagingTierCREHead, memberRoleCSHead: crePagingTierCSHead}[role]
		if len(heads[role]) == 0 {
			c.Gaps = append(c.Gaps, domain.PagingReadinessGap{
				Code: "MISSING_HEAD", Severity: readinessError, Fix: fixResponders,
				Message: "Nobody is the " + label + ", so the chain's top tier reaches nobody.",
			})
		}
		for _, m := range heads[role] {
			person(m, label, tier, false)
		}
	}

	abtKeys := map[string]bool{}
	sawAmericas := false
	for _, t := range f.CRETeams {
		key := strings.ToLower(t.Key)
		name := t.Name
		if name == "" {
			name = t.Key
		}
		isABT := t.Type == "cre-abt"
		if isABT {
			abtKeys[key] = true
		}
		members := byTeam[key]
		if t.IsAmericas {
			sawAmericas = true
			hasLead := false
			for _, m := range members {
				if m.Role == memberRoleAmerica {
					hasLead = true
					person(m, "America lead", crePagingTierAmericaLead, false)
				}
			}
			if !hasLead {
				c.Gaps = append(c.Gaps, domain.PagingReadinessGap{
					Code: "MISSING_AMERICAS_LEAD", Severity: readinessError, Fix: fixResponders, TeamKey: t.Key,
					Message: name + " has no America lead, so the night chain's second tier reaches nobody.",
				})
			}
		}
		hasLead := false
		for _, m := range members {
			if m.Role == memberRoleLead {
				hasLead = true
				person(m, "Team lead · "+name, crePagingTierTeamLead, false)
			}
		}
		if isABT && !hasLead {
			c.Gaps = append(c.Gaps, domain.PagingReadinessGap{
				Code: "MISSING_TEAM_LEAD", Severity: readinessError, Fix: fixResponders, TeamKey: t.Key,
				Message: name + " has no Team lead.",
			})
		}
		for _, tier := range []string{"T1", "T2", "T3"} {
			held := false
			for _, m := range members {
				if m.AlertTier == tier {
					held = true
					// Only the 1st responder's number is a must: the 2nd and 3rd
					// are called with them, so the tier still reaches someone.
					person(m, tier+" responder · "+name, crePagingTierResponders, tier != "T1")
				}
			}
			if !held {
				c.Gaps = append(c.Gaps, domain.PagingReadinessGap{
					Code: "MISSING_RESPONDER", Severity: readinessError, Fix: fixResponders, TeamKey: t.Key, Tier: tier,
					Message: name + " has no " + tier + " responder.",
				})
			}
		}
	}
	if !sawAmericas {
		c.Gaps = append(c.Gaps, domain.PagingReadinessGap{
			Code: "MISSING_AMERICAS_LEAD", Severity: readinessError, Fix: fixData,
			Message: "No Americas team was found, so the night chain has no America lead and no Americas responders.",
		})
	}

	c.Gaps = append(c.Gaps, onLeaveGaps(positions, f.CREAbsences, dates)...)

	for _, a := range f.AccountCRETeams {
		if abtKeys[a.Name] {
			continue
		}
		accounts := "1 account names"
		if a.Accounts != 1 {
			accounts = strconv.Itoa(a.Accounts) + " accounts name"
		}
		c.Gaps = append(c.Gaps, domain.PagingReadinessGap{
			Code: "ACCOUNT_TEAM_NOT_ABT", Severity: readinessWarning, Fix: fixData,
			Message: fmt.Sprintf("%s CRE team %q, which is not a CRE ABT, so their cases page no ABT's responders.", accounts, a.Name),
		})
	}
	return c
}

// leaveSpan is a run of consecutive days someone is away, within the range.
type leaveSpan struct {
	from, to time.Time
	labels   []string
}

// leaveSpans merges a person's absences into runs of consecutive days,
// clipped to [first, last]. An absence that moves them to teamKey does not
// make them away from it.
func leaveSpans(absences []repository.PagingReadinessAbsence, teamKey string, first, last time.Time) []leaveSpan {
	var spans []leaveSpan
	for _, ab := range absences {
		if ab.MovesToTeam != "" && strings.EqualFold(ab.MovesToTeam, teamKey) {
			continue
		}
		start, err := time.Parse(time.DateOnly, ab.StartsOn)
		if err != nil {
			continue
		}
		end := last
		if ab.EndsOn != "" {
			if end, err = time.Parse(time.DateOnly, ab.EndsOn); err != nil {
				continue
			}
		}
		if start.Before(first) {
			start = first
		}
		if end.After(last) {
			end = last
		}
		if end.Before(start) {
			continue
		}
		if n := len(spans); n > 0 && !start.After(spans[n-1].to.AddDate(0, 0, 1)) {
			if end.After(spans[n-1].to) {
				spans[n-1].to = end
			}
			spans[n-1].labels = appendLabel(spans[n-1].labels, ab.Label)
			continue
		}
		spans = append(spans, leaveSpan{from: start, to: end, labels: appendLabel(nil, ab.Label)})
	}
	return spans
}

func appendLabel(labels []string, label string) []string {
	if label == "" {
		return labels
	}
	for _, l := range labels {
		if l == label {
			return labels
		}
	}
	return append(labels, label)
}

// onLeaveGaps is one ON_LEAVE gap per position and run of days away: an
// error for a responder, whom the first rung calls directly, and a warning
// for a lead or head, whom a later rung reaches with others beside them.
func onLeaveGaps(positions []crePosition, absences []repository.PagingReadinessAbsence, dates []time.Time) []domain.PagingReadinessGap {
	if len(dates) == 0 || len(absences) == 0 {
		return nil
	}
	byUser := map[string][]repository.PagingReadinessAbsence{}
	for _, ab := range absences {
		byUser[ab.UserID] = append(byUser[ab.UserID], ab)
	}
	for id := range byUser {
		list := byUser[id]
		sort.Slice(list, func(i, j int) bool { return list[i].StartsOn < list[j].StartsOn })
	}
	var gaps []domain.PagingReadinessGap
	for _, p := range positions {
		for _, span := range leaveSpans(byUser[p.member.UserID], p.teamKey, dates[0], dates[len(dates)-1]) {
			severity := readinessWarning
			if p.responder {
				severity = readinessError
			}
			when := "on " + span.from.Format(time.DateOnly)
			if span.to.After(span.from) {
				when = "from " + span.from.Format(time.DateOnly) + " to " + span.to.Format(time.DateOnly)
			}
			why := ""
			if len(span.labels) > 0 {
				why = " (" + strings.Join(span.labels, ", ") + ")"
			}
			name := p.member.Name
			if name == "" {
				name = p.member.Email
			}
			gaps = append(gaps, domain.PagingReadinessGap{
				Code: "ON_LEAVE", Severity: severity, Fix: fixResponders,
				TeamKey: p.teamKey, Date: span.from.Format(time.DateOnly),
				Message: fmt.Sprintf("%s, %s, is away %s%s.", name, p.role, when, why),
			})
		}
	}
	return gaps
}

// appliesOn reports whether a window with dayScope is worked on date.
func appliesOn(dayScope string, date time.Time) bool {
	weekend := date.Weekday() == time.Saturday || date.Weekday() == time.Sunday
	switch dayScope {
	case "ANY":
		return true
	case "WEEKDAY":
		return !weekend
	case "WEEKEND":
		return weekend
	}
	return false
}

// rotaFamily is the family of an active rota, or "" when there is none.
func rotaFamily(f repository.PagingReadinessFacts, code string) string {
	for _, r := range f.Rotas {
		if r.Code == code {
			return r.Family
		}
	}
	return ""
}

// rotaActive reports whether the rota exists and is active.
func rotaActive(f repository.PagingReadinessFacts, code string) bool {
	for _, r := range f.Rotas {
		if r.Code == code {
			return true
		}
	}
	return false
}

// sreReadiness checks one SRE rota, reported as chain: on every date, every zone it works that
// day (a zone with a window whose day scope covers the date, so the weekend
// folds TZ2 into TZ1) has an L1, an L2 and an L3 rostered.
func sreReadiness(f repository.PagingReadinessFacts, chain, rotaCode, label string, dates []time.Time) domain.PagingReadinessChain {
	c := domain.PagingReadinessChain{Chain: chain, Label: label}
	var windows []repository.PagingReadinessWindow
	for _, w := range f.Windows {
		if w.RotaCode == rotaCode {
			windows = append(windows, w)
		}
	}
	covered := map[string]bool{} // date|zone|tier
	tiersOf := map[string]map[string]bool{}
	var order []repository.PagingReadinessAssignment
	for _, a := range f.Assignments {
		if a.RotaCode != rotaCode {
			continue
		}
		covered[a.RotaDate+"|"+a.ZoneCode+"|"+a.Tier] = true
		key := personKey(a.Email, a.Name)
		if tiersOf[key] == nil {
			tiersOf[key] = map[string]bool{}
			order = append(order, a)
		}
		if a.Tier != "" {
			tiersOf[key][a.Tier] = true
		}
	}
	for _, a := range order {
		held := tiersOf[personKey(a.Email, a.Name)]
		role := strings.Join(sortedKeys(held), ", ")
		if role == "" {
			role = "On the rota"
		}
		c.People = append(c.People, domain.PagingReadinessPerson{
			UserID: a.UserID, Email: a.Email, Name: a.Name, Role: role, PagingTier: sreFirstTier(held),
		})
	}

	if !rotaActive(f, rotaCode) || len(windows) == 0 {
		c.Gaps = append(c.Gaps, domain.PagingReadinessGap{
			Code: "ROTA_INACTIVE", Severity: readinessError, Fix: fixConfig,
			Message: "The " + label + "'s rota has no active escalation windows, so nobody is paged from it.",
		})
		return c
	}

	type zone struct {
		code, label string
		sort        int
	}
	for _, d := range dates {
		day := d.Format(time.DateOnly)
		zones := map[string]zone{}
		for _, w := range windows {
			if appliesOn(w.DayScope, d) {
				zones[w.ZoneCode] = zone{w.ZoneCode, w.ZoneLabel, w.ZoneSortOrder}
			}
		}
		list := make([]zone, 0, len(zones))
		for _, z := range zones {
			list = append(list, z)
		}
		sort.Slice(list, func(i, j int) bool {
			if list[i].sort != list[j].sort {
				return list[i].sort < list[j].sort
			}
			return list[i].code < list[j].code
		})
		for _, z := range list {
			for _, tier := range []string{"L1", "L2", "L3"} {
				if covered[day+"|"+z.code+"|"+tier] {
					continue
				}
				name := z.label
				if name == "" {
					name = z.code
				}
				c.Gaps = append(c.Gaps, domain.PagingReadinessGap{
					Code: "TIER_UNCOVERED", Severity: readinessError, Fix: fixRota,
					Date: day, ZoneCode: z.code, Tier: tier,
					Message: fmt.Sprintf("%s has no %s rostered on %s.", name, tier, day),
				})
			}
		}
	}
	return c
}

// smeReadiness checks the SME ladder: every SME team on an SME rota has an
// L1, an L2 and an L3 on each of its windows worked each day -- someone
// rostered there with no tier counts as L1, as the ladder pages them -- and
// every team the specialist handoff dialog offers names an SME team that
// exists. A missing L1 is an error (the ladder reaches nobody); a missing L2
// or L3 is a warning (it stops climbing early).
func smeReadiness(f repository.PagingReadinessFacts, handoff *SpecialistHandoffConfig, dates []time.Time) domain.PagingReadinessChain {
	c := domain.PagingReadinessChain{Chain: familySME, Label: "SME chain"}
	smeRota := map[string]bool{}
	for _, r := range f.Rotas {
		if r.Family == familySME {
			smeRota[r.Code] = true
		}
	}
	smeTeams := map[string]bool{}
	teamName := map[string]string{}
	for _, t := range f.SMETeams {
		smeTeams[strings.ToLower(t.Key)] = true
		teamName[strings.ToLower(t.Key)] = t.Name
	}

	covered := map[string]bool{} // team|date|shift|tier
	rolesOf := map[string]map[string]bool{}
	tiersOf := map[string]map[string]bool{}
	var order []repository.PagingReadinessAssignment
	for _, a := range f.Assignments {
		if !smeRota[a.RotaCode] {
			continue
		}
		team := strings.ToLower(a.TeamKey)
		tier := a.Tier
		if tier == "" {
			tier = "L1"
		}
		covered[team+"|"+a.RotaDate+"|"+a.ShiftCode+"|"+tier] = true
		key := personKey(a.Email, a.Name)
		if rolesOf[key] == nil {
			rolesOf[key], tiersOf[key] = map[string]bool{}, map[string]bool{}
			order = append(order, a)
		}
		name := teamName[team]
		if name == "" {
			name = a.TeamKey
		}
		rolesOf[key][name+" "+tier] = true
		tiersOf[key][tier] = true
	}
	for _, a := range order {
		key := personKey(a.Email, a.Name)
		c.People = append(c.People, domain.PagingReadinessPerson{
			UserID: a.UserID, Email: a.Email, Name: a.Name, Role: strings.Join(sortedKeys(rolesOf[key]), ", "),
			PagingTier: sreFirstTier(tiersOf[key]),
		})
	}

	for _, t := range f.SMETeams {
		if !smeRota[t.RotaCode] {
			continue
		}
		name := t.Name
		if name == "" {
			name = t.Key
		}
		for _, d := range dates {
			day := d.Format(time.DateOnly)
			for _, w := range f.Windows {
				if w.RotaCode != t.RotaCode || !appliesOn(w.DayScope, d) {
					continue
				}
				for _, tier := range []string{"L1", "L2", "L3"} {
					if covered[strings.ToLower(t.Key)+"|"+day+"|"+w.ShiftCode+"|"+tier] {
						continue
					}
					severity := readinessWarning
					if tier == "L1" {
						severity = readinessError
					}
					c.Gaps = append(c.Gaps, domain.PagingReadinessGap{
						Code: "TIER_UNCOVERED", Severity: severity, Fix: fixRota,
						TeamKey: t.Key, Date: day, ShiftCode: w.ShiftCode, Tier: tier,
						Message: fmt.Sprintf("%s has no %s rostered for %s on %s.", name, tier, w.ShiftLabel, day),
					})
				}
			}
		}
	}

	if handoff != nil {
		for _, p := range handoff.Products {
			for _, t := range p.Teams {
				switch {
				case t.SMETeam == "":
					c.Gaps = append(c.Gaps, domain.PagingReadinessGap{
						Code: "HANDOFF_TEAM_UNMAPPED", Severity: readinessError, Fix: fixConfig,
						Message: fmt.Sprintf("%s's handoff team %s (%s) names no SME team, so escalating to it pages nobody.", p.Name, t.Label, t.Key),
					})
				case !smeTeams[strings.ToLower(t.SMETeam)]:
					c.Gaps = append(c.Gaps, domain.PagingReadinessGap{
						Code: "HANDOFF_TEAM_UNMAPPED", Severity: readinessError, Fix: fixConfig, TeamKey: t.SMETeam,
						Message: fmt.Sprintf("%s's handoff team %s (%s) names SME team %q, which is not an SME team in the Team Schedule.", p.Name, t.Label, t.Key, t.SMETeam),
					})
				}
			}
		}
	}
	return c
}

// personKey identifies a person in a people list: their email, or their
// name for a user row with none.
func personKey(email, name string) string {
	if email != "" {
		return strings.ToLower(email)
	}
	return "name:" + name
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// The Case Paging tier at which the CRE chain first calls each position
// (workbook, "CRE chain rules"): the responders at Tier 1, the Team leads at
// Tier 2 (the Americas Team leads at night), the America Team lead at Tier 3
// (night), the CRE head at Tier 4 and the CS head at Tier 5.
const (
	crePagingTierResponders  = 1
	crePagingTierTeamLead    = 2
	crePagingTierAmericaLead = 3
	crePagingTierCREHead     = 4
	crePagingTierCSHead      = 5
)

// sreFirstTier is the earliest SRE tier (L1 = 1, L2 = 2, L3 = 3) among those a
// person holds on the rota, or 0 when they hold none.
func sreFirstTier(held map[string]bool) int {
	for i, tier := range []string{"L1", "L2", "L3"} {
		if held[tier] {
			return i + 1
		}
	}
	return 0
}
