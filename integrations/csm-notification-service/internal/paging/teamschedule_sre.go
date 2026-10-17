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

package paging

import (
	"context"
	"sort"
	"strings"
	"time"
)

// The SRE half of TeamScheduleResolver. Kept apart from the CRE half because
// the two share nothing but the rota client: the CRE ladder routes by a rule
// table of shifts and ranks, the SRE ladder by on-call tier.
//
//	LEVEL_0  L1 support   whoever holds tier L1 at the trigger instant
//	LEVEL_1  L2 support   tier L2
//	LEVEL_2  L3 support   tier L3
//	LEVEL_3  L4 support   OPT-IN (timing.includeL4) and NOT CONFIRMED: the
//	                      lead of the SRE team answering the incident
//	LEVEL_4  nobody       the SRE ladder has no fifth rung
//
// Every rung reaches ONE person. Weekdays 12:00-15:00 IST the TZ1 and TZ2
// escalation windows are both live, so a tier can have two holders at once;
// the SRE team confirmed only one of them is called.

// familySRE is how the catalogue spells the SRE family, for teams and windows
// alike (entity-service folds the registry's SRE-ABT into it).
const familySRE = "SRE"

// keyOf turns an assignment group into a rota team key, honouring a
// configured alias first.
func (r TeamScheduleResolver) keyOf(team string) string {
	key := teamKeyFor(team)
	if alias, ok := r.aliases[key]; ok {
		return alias
	}
	return key
}

// isDefaultGroup reports whether the assignment group is one of
// sre.teams.defaultGroups: a group holding incidents that have no SRE team of
// their own. The group's own name is matched as well as its alias, so an
// alias for "Default" does not hide it.
func (r TeamScheduleResolver) isDefaultGroup(team string) bool {
	key := teamKeyFor(team)
	if key == "" {
		return false
	}
	return contains(r.defaultGroups, key) || contains(r.defaultGroups, r.keyOf(team))
}

func (r TeamScheduleResolver) isSRETeam(key string) bool {
	for _, k := range r.sreTeamKeys {
		if k == key {
			return true
		}
	}
	return false
}

// LadderFor implements LadderClassifier: an incident climbs the SRE ladder
// when the team it is assigned to is an SRE team, and the CRE one otherwise --
// including when it has no team, or one the rota does not know.
//
// The configured SRE team list answers first, the same way the configured
// ABT list answers the CRE table's "is this an ABT" column. Only with no list
// configured is the catalogue asked, so a deployment without the file still
// classifies.
func (r TeamScheduleResolver) LadderFor(ctx context.Context, rc RoutingContext) (Ladder, error) {
	key := r.keyOf(rc.AssignedCRETeam)
	if key == "" {
		return LadderCRE, nil
	}
	if r.isDefaultGroup(rc.AssignedCRETeam) {
		return LadderSRE, nil
	}
	if len(r.sreTeamKeys) > 0 {
		if r.isSRETeam(key) {
			return LadderSRE, nil
		}
		return LadderCRE, nil
	}
	cat, err := r.entity.ScheduleCatalogue(ctx)
	if err != nil {
		return LadderCRE, err
	}
	for _, t := range cat.Teams {
		if strings.EqualFold(t.Key, key) {
			if strings.EqualFold(t.Family, familySRE) {
				return LadderSRE, nil
			}
			return LadderCRE, nil
		}
	}
	return LadderCRE, nil
}

// TeamFamily implements TeamFamilyResolver: which family the incident's
// assignment group belongs to, for routing.
//
//	sre   an SRE team -- the configured sre.teams.abts, or the catalogue's SRE
//	      family when none are configured
//	cre   any other team the rota knows
//	none  no assignment group, or one neither the configuration nor the rota
//	      knows
func (r TeamScheduleResolver) TeamFamily(ctx context.Context, rc RoutingContext) (string, error) {
	key := r.keyOf(rc.AssignedCRETeam)
	if key == "" {
		return TeamFamilyNone, nil
	}
	if r.isSRETeam(key) || r.isDefaultGroup(rc.AssignedCRETeam) {
		return TeamFamilySRE, nil
	}
	if contains(r.abtTeamKeys, key) || key == r.americasTeamKey {
		return TeamFamilyCRE, nil
	}
	cat, err := r.entity.ScheduleCatalogue(ctx)
	if err != nil {
		return TeamFamilyNone, err
	}
	for _, t := range cat.Teams {
		if strings.EqualFold(t.Key, key) {
			if strings.EqualFold(t.Family, familySRE) {
				return TeamFamilySRE, nil
			}
			return TeamFamilyCRE, nil
		}
	}
	return TeamFamilyNone, nil
}

// sreTier is the rota tier each SRE rung reads.
var sreTier = map[Level]string{Level0: "L1", Level1: "L2", Level2: "L3"}

// ownSRETeam is the incident's own SRE team key, or "" for one that has none.
func (r TeamScheduleResolver) ownSRETeam(rc RoutingContext) string {
	own := r.keyOf(rc.AssignedCRETeam)
	if r.isDefaultGroup(rc.AssignedCRETeam) {
		// A "Default" incident has no SRE team: whoever is on duty answers.
		return ""
	}
	if !r.isSRETeam(own) && len(r.sreTeamKeys) > 0 {
		// A CRE incident climbing the SRE ladder (a P0) belongs to no SRE
		// team; every SRE team is then equally placed to answer.
		return ""
	}
	return own
}

// rotaFor decides which SRE rota pages an incident: its own team's, or
// sre.teams.defaultRota for an incident with no SRE team of its own (a case at
// S0, a monitoring alert raised with no team). "" when the catalogue names no
// rotas at all: an entity-service from before rotas, where every SRE window
// is read, as it always was.
func (r TeamScheduleResolver) rotaFor(cat scheduleCatalogue, own string) string {
	if !cat.hasRotas() {
		return ""
	}
	if own != "" {
		if rota := cat.teamRota(own); rota != "" {
			return rota
		}
	}
	return r.defaultRota
}

// SRERota implements SRERotaResolver.
func (r TeamScheduleResolver) SRERota(ctx context.Context, rc RoutingContext) (string, error) {
	cat, err := r.entity.ScheduleCatalogue(ctx)
	if err != nil {
		return "", err
	}
	return r.rotaFor(cat, r.ownSRETeam(rc)), nil
}

// SaaSSRETeam implements SpecialistResolver: whether an assignment group is a
// SaaS SRE team, the only kind whose incidents may be escalated to an SME.
//
// The team's rota answers when the catalogue has rotas. Without them, a team
// counts when it is an SRE team (sre.teams.abts, or the catalogue's SRE family
// with no list configured) and not typed sre-iaas -- team.type is what the
// rota is derived from, so the answer is the same one.
func (r TeamScheduleResolver) SaaSSRETeam(ctx context.Context, group string) (bool, error) {
	key := r.keyOf(group)
	if key == "" {
		return false, nil
	}
	cat, err := r.entity.ScheduleCatalogue(ctx)
	if err != nil {
		return false, err
	}
	if cat.hasRotas() {
		return strings.EqualFold(cat.teamRota(key), RotaSRESaaS), nil
	}
	sre := r.isSRETeam(key)
	if !sre && len(r.sreTeamKeys) == 0 {
		for _, t := range cat.Teams {
			if strings.EqualFold(t.Key, key) && strings.EqualFold(t.Family, familySRE) {
				sre = true
			}
		}
	}
	if !sre {
		return false, nil
	}
	members, err := r.entity.TeamMembers(ctx, []string{key}, nil, nil, nil)
	if err != nil {
		return false, err
	}
	for _, m := range members {
		if strings.EqualFold(m.TeamType, teamTypeSREIaaS) {
			return false, nil
		}
	}
	return true, nil
}

// teamTypeSREIaaS is the team.type of the IaaS SRE team (migration 0200).
const teamTypeSREIaaS = "sre-iaas"

// resolveSRE answers one SRE rung.
func (r TeamScheduleResolver) resolveSRE(ctx context.Context, level Level, rc RoutingContext) ([]Recipient, error) {
	own := r.ownSRETeam(rc)

	if tier, ok := sreTier[level]; ok {
		pick, found, err := r.onCallTier(ctx, rc.At, own, tier, rc.Rota)
		if err != nil || !found {
			return nil, err
		}
		return []Recipient{pick.Recipient}, nil
	}
	if level != Level3 {
		return nil, nil
	}

	// L4 is the lead of the team answering the incident: its own, or -- for
	// an incident that has none -- the team of whoever took L1.
	team := own
	if team == "" {
		pick, found, err := r.onCallTier(ctx, rc.At, "", "L1", rc.Rota)
		if err != nil || !found {
			return nil, err
		}
		team = pick.team
	}
	if team == "" {
		return nil, nil
	}
	leads, err := r.leadsOf(ctx, []string{team})
	if err != nil || len(leads) == 0 {
		return nil, err
	}
	return leads[:1], nil
}

// tierHolder is one candidate for an SRE rung.
type tierHolder struct {
	Recipient
	team    string
	zone    string
	onLeave bool
}

// preferAvailable keeps only the holders who are not on leave when there are
// any; otherwise it keeps them all. On-leave holders are only there at all
// when the ladder's onLeave is call (the on-duty read leaves them out
// otherwise), so this is "ring someone on leave only when nobody else holds
// the tier".
func preferAvailable(holders []tierHolder) []tierHolder {
	var available []tierHolder
	for _, h := range holders {
		if !h.onLeave {
			available = append(available, h)
		}
	}
	if len(available) > 0 {
		return available
	}
	return holders
}

// onCallTier picks the ONE person holding tier on an SRE window at that
// instant.
//
// In order: the incident's own team, if it has anybody on that tier (a team's
// own engineer is never skipped for another team's); then the zone whose L1
// block is live, which is the zone that owns the incident right now and is
// what makes the 12:00-15:00 overlap call one person rather than two; then the
// configured SRE team order; then email, so a retry reaches the same person.
//
// Only the windows of one rota count -- SaaS SRE and IaaS SRE each run their
// own chain, and a SaaS incident must never reach an IaaS engineer or the
// other way round. rota is the one the engine stamped; empty asks rotaFor.
// With no rotas in the catalogue at all every SRE window counts, as before.
func (r TeamScheduleResolver) onCallTier(ctx context.Context, at time.Time, ownTeam, tier, rota string) (tierHolder, bool, error) {
	cat, err := r.entity.ScheduleCatalogue(ctx)
	if err != nil {
		return tierHolder{}, false, err
	}
	if rota == "" {
		rota = r.rotaFor(cat, ownTeam)
	}
	zoneRota := cat.zoneRotas()
	type window struct{ zone, tier string }
	sre := map[string]window{}
	for _, s := range cat.Shifts {
		if !strings.EqualFold(s.Family, familySRE) {
			continue
		}
		if rota != "" && !strings.EqualFold(zoneRota[deref(s.ZoneCode)], rota) {
			continue
		}
		sre[s.Code] = window{zone: deref(s.ZoneCode), tier: deref(s.Tier)}
	}
	onDuty, err := r.entity.OnDutyAt(ctx, at, r.callOnLeave)
	if err != nil {
		return tierHolder{}, false, err
	}

	var holders []tierHolder
	l1Zone := ""
	for _, a := range onDuty {
		w, ok := sre[a.ShiftCode]
		if !ok || a.Engineer.UserID == "" {
			continue
		}
		held := deref(a.Tier)
		if held == "" {
			held = w.tier
		}
		zone := deref(a.ZoneCode)
		if zone == "" {
			zone = w.zone
		}
		if strings.EqualFold(held, "L1") && (l1Zone == "" || zone < l1Zone) {
			l1Zone = zone
		}
		if !strings.EqualFold(held, tier) {
			continue
		}
		holders = append(holders, tierHolder{
			Recipient: Recipient{Email: a.Engineer.Email, Name: a.Engineer.Name, UserID: a.Engineer.UserID, ShiftCode: a.ShiftCode},
			team:      r.keyOf(a.TeamKey),
			zone:      zone,
			onLeave:   a.OnLeave,
		})
	}
	holders = preferAvailable(holders)
	if len(holders) == 0 {
		return tierHolder{}, false, nil
	}

	if ownTeam != "" {
		var mine []tierHolder
		for _, h := range holders {
			if h.team == ownTeam {
				mine = append(mine, h)
			}
		}
		if len(mine) > 0 {
			holders = mine
		}
	}

	order := func(team string) int {
		for i, k := range r.sreTeamKeys {
			if k == team {
				return i
			}
		}
		return len(r.sreTeamKeys)
	}
	sort.SliceStable(holders, func(i, j int) bool {
		a, b := holders[i], holders[j]
		if (a.zone == l1Zone) != (b.zone == l1Zone) {
			return a.zone == l1Zone
		}
		if order(a.team) != order(b.team) {
			return order(a.team) < order(b.team)
		}
		return a.Email < b.Email
	})
	return holders[0], true, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// familySME is how the catalogue spells the Special Ops family.
const familySME = "SME"

// resolveSME answers one rung of an SME ladder: the ONE person holding its
// tier (L1, L2, L3 for LEVEL_0..LEVEL_2) on an SME-family window of the SME
// team at that instant -- several on the tier, the one longest since last
// called, then by email. Someone rostered on the window with no tier counts
// as L1: SME shifts had no tiers before, and nothing already rostered stops
// being paged.
func (r TeamScheduleResolver) resolveSME(ctx context.Context, level Level, rc RoutingContext) ([]Recipient, error) {
	tier, ok := sreTier[level]
	if !ok {
		return nil, nil
	}
	team := r.keyOf(rc.SMETeam)
	if team == "" {
		return nil, nil
	}
	cat, err := r.entity.ScheduleCatalogue(ctx)
	if err != nil {
		return nil, err
	}
	sme := map[string]string{} // SME window code -> its fixed tier, if any
	for _, s := range cat.Shifts {
		if strings.EqualFold(s.Family, familySME) {
			sme[s.Code] = deref(s.Tier)
		}
	}
	onDuty, err := r.entity.OnDutyAt(ctx, rc.At, rc.CallOnLeave)
	if err != nil {
		return nil, err
	}
	var pool, away []Recipient
	seen := map[string]bool{}
	for _, a := range onDuty {
		fixed, isSME := sme[a.ShiftCode]
		email := strings.ToLower(strings.TrimSpace(a.Engineer.Email))
		if !isSME || email == "" || seen[email] || r.keyOf(a.TeamKey) != team {
			continue
		}
		held := deref(a.Tier)
		if held == "" {
			held = fixed
		}
		if held == "" {
			held = "L1"
		}
		if !strings.EqualFold(held, tier) {
			continue
		}
		seen[email] = true
		p := Recipient{Email: a.Engineer.Email, Name: a.Engineer.Name, UserID: a.Engineer.UserID, ShiftCode: a.ShiftCode}
		if a.OnLeave {
			away = append(away, p)
			continue
		}
		pool = append(pool, p)
	}
	if len(pool) == 0 {
		// onLeave: call -- nobody available holds the tier, so ring whoever
		// does although they are away.
		pool = away
	}
	sort.SliceStable(pool, func(i, j int) bool { return pool[i].Email < pool[j].Email })
	return r.takeLongestSinceCalled(ctx, pool, 1), nil
}
