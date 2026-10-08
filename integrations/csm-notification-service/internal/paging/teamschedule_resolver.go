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
	"fmt"
	"log/slog"
	"slices"
	"sort"
	"strings"
	"time"
)

// TeamScheduleResolver answers each rung from the Team Schedule, replacing the
// hand-maintained roster RosterResolver reads.
//
// # THE RUNG MODEL HERE IS AN ASSUMPTION
//
// Who each rung is was described by the CS team and has not been confirmed.
// It exists so there is a complete, valid flow to exercise end to end while
// the real one is settled, and it is expected to change. Nothing below should
// be read as the decided design.
//
//	LEVEL_0  the sub lead rostered on the rotation covering that instant
//	LEVEL_1  the incident's own ABT's sub leads, called together
//	LEVEL_2  that ABT's lead
//	LEVEL_3  the CRE head
//	LEVEL_4  the CS head
//
// LEVEL_0 takes two calls, not one: who is rostered comes from the rota, and
// what rank they hold comes from the membership behind it. They are separate
// because rank changes rarely and the rota changes daily, and because folding
// rank into the rota response would widen a shape the Team Schedule page
// already renders.
type TeamScheduleResolver struct {
	entity teamScheduleReader
	// rules is the table this resolver routes by. Held rather than read from a
	// package variable so a deployment can correct a row without a release.
	rules []Rule
	// abtTeamKeys are the ABT's teams when configuration names them outright.
	// Empty means the ABT is resolved by type instead.
	abtTeamKeys []string
	// abtType is the ABT this ladder escalates within, as team.type spells it.
	abtType string
	// americasTeamKey is the team covering the night shift.
	americasTeamKey string
	// teamLeadKeys overrides the "Team leads" pool; empty means the ABT's own
	// teams.
	teamLeadKeys []string
	// teamLeadsToCall is how many of that pool the rung calls; 0 calls all.
	teamLeadsToCall int
	// rotaMembersToCall caps the rota rungs per shift; absent or 0 means
	// everyone on duty, which is the normal case.
	rotaMembersToCall map[Shift]int
	// rotaTeamKeys narrows the rota rungs to these teams; empty means this
	// ladder's ABT teams plus Americas. See TeamKeys.RotaTeams.
	rotaTeamKeys []string
	// americasWeekendRotaShifts is the Team Schedule shift codes the weekend
	// night's rota member is taken from. See TeamKeys.AmericasWeekendRotaShifts.
	americasWeekendRotaShifts []string
	// unassignedTeamLead is how the "Team lead" rung answers when there is no
	// ABT to take a lead from: "pool" or "none". See TeamKeys.
	unassignedTeamLead string
	// unassignedTeamLeadCount is how many of the pool that case calls.
	unassignedTeamLeadCount int
	// americasLead is the single lead above the Americas team's own leads.
	americasLead Person
	// heads are the last two rungs when configuration names them outright,
	// which is the normal case: they are two people, not a team.
	heads Heads
	// tiers is the alert-duty vocabulary, and perTeam how many nominees a
	// rung takes from each team it spans (0 = all).
	tiers   []string
	perTeam int
	// history answers when somebody was last called, for the evening
	// pairing's "one other member" rule. Optional: without it the pairing
	// falls back to a stable order, which is deterministic but not fair.
	history callHistory
	// leadershipTeamKey is the team the two heads belong to. They sit outside
	// every ABT on purpose, so that a head still resolves to no ABT for
	// /users/me -- the absence the Team Schedule page reads as "belongs to
	// neither group".
	leadershipTeamKey string
	// sreTeamKeys are the SRE teams, in the order the SRE ladder breaks a tie
	// between them. See teamschedule_sre.go.
	sreTeamKeys []string
	// aliases maps an assignment group's name to the rota key it stands for,
	// for a group whose name is not simply its key ("SRE - Apollo").
	aliases map[string]string
	// phones supplies a number for each recipient. The rota holds none, so
	// without it a rung can be reached over chat but not dialled.
	phones PhoneBook
}

// teamScheduleReader is the slice of EntityClient this needs, named so tests
// can stand in for it without an HTTP server.
type teamScheduleReader interface {
	TeamMembers(ctx context.Context, teamKeys, roles, alertTiers, teamTypes []string) ([]teamMember, error)
	OnDutyAt(ctx context.Context, at time.Time) ([]onDutyAssignment, error)
	ScheduleCatalogue(ctx context.Context) (scheduleCatalogue, error)
}

// callHistory answers when each of these people was last called. Satisfied by
// *Store; nil is a valid value and means "no history to go on".
type callHistory interface {
	LastCalled(ctx context.Context, emails []string) (map[string]time.Time, error)
}

// WithHeads names the two people the top of the ladder reaches, instead of
// looking them up by role inside a leadership team.
func (r TeamScheduleResolver) WithHeads(h Heads) TeamScheduleResolver {
	r.heads = h
	return r
}

// WithAlertDuty sets the nomination vocabulary and how many nominees a rung
// takes from each team.
func (r TeamScheduleResolver) WithAlertDuty(tiers []string, perTeam int) TeamScheduleResolver {
	if len(tiers) > 0 {
		r.tiers = tiers
	}
	r.perTeam = perTeam
	return r
}

// WithCallHistory returns a copy that spreads the evening pairing's second
// call across the rota by who has gone longest without one.
func (r TeamScheduleResolver) WithCallHistory(h callHistory) TeamScheduleResolver {
	r.history = h
	return r
}

// WithPhoneBook returns a copy that fills in each recipient's number.
func (r TeamScheduleResolver) WithPhoneBook(pb PhoneBook) TeamScheduleResolver {
	r.phones = pb
	return r
}

func NewTeamScheduleResolver(entity teamScheduleReader, teams TeamKeys, rules []Rule) TeamScheduleResolver {
	if strings.TrimSpace(teams.Leadership) == "" {
		teams.Leadership = defaultLeadershipTeamKey
	}
	if len(rules) == 0 {
		rules = DefaultRules
	}
	aliases := make(map[string]string, len(teams.Aliases))
	for group, key := range teams.Aliases {
		aliases[teamKeyFor(group)] = teamKeyFor(key)
	}
	keys := teamKeysFor(teams.ABTs)
	leadKeys := make([]string, 0, len(teams.TeamLeads))
	for _, k := range teams.TeamLeads {
		if k = teamKeyFor(k); k != "" {
			leadKeys = append(leadKeys, k)
		}
	}
	if len(leadKeys) == 0 {
		leadKeys = keys
	}
	return TeamScheduleResolver{
		abtType:         strings.ToLower(strings.TrimSpace(teams.ABTType)),
		americasLead:    teams.AmericasLead,
		teamLeadsToCall: teams.TeamLeadsToCall,

		rotaMembersToCall:         normaliseRotaCaps(teams.RotaMembersToCall),
		rotaTeamKeys:              lowerKeys(teams.RotaTeams),
		americasWeekendRotaShifts: americasWeekendShifts(teams.AmericasWeekendRotaShifts),
		unassignedTeamLead:        unassignedLeadMode(teams.UnassignedTeamLead),
		unassignedTeamLeadCount:   unassignedLeadCount(teams.UnassignedTeamLeadCount),
		tiers:                     alertTiers,
		entity:                    entity,
		rules:                     rules,
		abtTeamKeys:               keys,
		teamLeadKeys:              leadKeys,
		americasTeamKey:           teamKeyFor(teams.Americas),
		leadershipTeamKey:         teams.Leadership,
		sreTeamKeys:               teamKeysFor(teams.SRE),
		aliases:                   aliases,
	}
}

func teamKeysFor(names []string) []string {
	keys := make([]string, 0, len(names))
	for _, k := range names {
		if k = teamKeyFor(k); k != "" {
			keys = append(keys, k)
		}
	}
	return keys
}

// TeamKeys names the teams the rule table refers to by role rather than by
// name. They are configuration because they are deployment facts -- there are
// seven ABTs today and there will not always be -- and because a rung that
// silently reaches nobody because a team was renamed is the failure this whole
// resolver exists to avoid.
type TeamKeys struct {
	// ABTType is the ABT this ladder escalates within, as team.type spells it:
	// cre-abt for the CRE ladder, sre-abt for the SRE one.
	//
	// Preferred over listing keys. cre-abt holds seven teams and sre-abt two,
	// and which teams those are is already recorded per team in the database
	// -- so asking for the type means adding a team to an ABT needs no config
	// change, and a team moved between ABTs cannot leave a stale key behind.
	ABTType string `yaml:"abtType"`
	// ABTs is an explicit list of team keys, for a deployment that wants to
	// name them rather than take whatever the ABT currently holds. When set it
	// wins over ABTType.
	ABTs []string `yaml:"abts"`
	// TeamLeadsToCall is how many leads the "Team leads" rung calls out of the
	// pool. Zero calls all of them.
	//
	// The rung is "at least three team leads from the pool", and the pool is
	// every ABT team's lead -- seven for cre-abt. So the sheet's count of
	// three and the pool of seven were never in conflict: one is how many are
	// called, the other is how many there are to choose from. Which three
	// rotates, by who has gone longest without a call, so the duty spreads
	// across the pool instead of always landing on the same names.
	TeamLeadsToCall int `yaml:"teamLeadsToCall"`
	// RotaMembersToCall caps the rota-sourced Level 0 rungs, per shift.
	//
	// "Rota Members" means everyone on duty, and that is what the resolver
	// returns -- so the rules sheet's counts for those rungs (2 on LK_MORNING,
	// 3 on LK_WEEKEND, 7 on LK_EVENING without an ABT) are descriptions of how
	// many people are rostered in each shift, not limits the code imposes.
	//
	// EMPTY IS THE DEFAULT AND THE RECOMMENDED SETTING. A cap means an
	// engineer who IS on duty does not get called, which is a strange thing to
	// want from a pager; it exists because these numbers decide the Twilio
	// bill and a deployment may need a hard ceiling it can set without a
	// release. Name only the shifts you want capped:
	//
	//	rotaMembersToCall:
	//	  LK_EVENING: 7
	//
	// Keyed by shift as resolver.go spells it (LK, LK_MORNING, LK_EVENING,
	// LK_WEEKEND, USA, USA_WEEKEND); an unrecognised key is an error rather
	// than a silent no-op, since a typo would otherwise read as a cap that is
	// quietly not applied. Which members are dropped rotates by
	// longest-since-called, the same fairness the "Team leads" rung uses, so a
	// cap does not always spare the same names.
	RotaMembersToCall map[string]int `yaml:"rotaMembersToCall"`
	// AmericasWeekendRotaShifts is where the weekend night's (R6) Level 0 rota
	// member comes from: the Team Schedule shift codes of the Americas weekend
	// rota. Empty means CRE_WEEKEND_NIGHT, the catalogue's "Americas weekend".
	//
	// The rung is "a rota member and the Americas nominees". It used to take
	// whoever sorted first among EVERYONE on duty at that hour, which on a
	// weekend night is mostly the separate on-call shift
	// (CRE_WEEKEND_NIGHT_OC) -- so the on-call engineer was paged first and the
	// rostered Americas weekend member not at all. The on-call shift is the
	// fallback cover, not the first responder, so it is deliberately not in
	// the default. Nobody on these shifts means the rung calls the nominees
	// alone; it never falls back to the on-call shift.
	AmericasWeekendRotaShifts []string `yaml:"americasWeekendRotaShifts"`
	// RotaTeams is whose rota a rota rung may reach. Empty -- the default --
	// means this ladder's own ABT teams plus Americas.
	//
	// The Team Schedule holds every rota, the SRE teams' included, and the
	// rota rungs used to take whoever it said was on duty. So a CRE ladder's
	// first responders on the morning, weekend and evening rotas were mostly
	// Apollo and Artemis engineers: ten of eleven on a weekday morning. SRE
	// is reached only through its own ladder, on a P0, one person per rung --
	// never as a CRE rota member. With the SRE rotas excluded the rota counts
	// match the rules sheet exactly: seven on a weekday evening, three on a
	// weekend day.
	RotaTeams []string `yaml:"rotaTeams"`
	// UnassignedTeamLead is who the "Team lead" rung reaches when the incident
	// is on no ABT -- R3, R4b, and R1 when the incident carries no team at all.
	//
	// That rung takes the incident's OWN ABT's lead, and an unassigned
	// incident has no ABT to take one from, so it used to resolve to nobody:
	// the rung was recorded NO_RECIPIENTS and skipped. Skipping is right when a
	// lead is merely unconfigured -- the ladder climbs and somebody above
	// answers -- but here the rung can never fire for ANY unassigned incident,
	// and the rungs above keep their own offsets, so the ladder simply went
	// quiet for Level 1's whole budget. On a P0 that is +0m to +4m with nobody
	// called, where the rules sheet says one call.
	//
	//	pool  one lead from TeamLeads, longest-since-called (the default)
	//	none  reach nobody and climb, the behaviour before this existed
	UnassignedTeamLead string `yaml:"unassignedTeamLead"`
	// UnassignedTeamLeadCount is how many of that pool it calls. The sheet
	// says one at Level 1 against three at Level 2, which is what keeps the
	// two rungs distinct; raising it blurs them.
	UnassignedTeamLeadCount int `yaml:"unassignedTeamLeadCount"`
	// TeamLeads overrides the pool itself. Empty means every ABT team's lead.
	//
	// It is configurable because the spreadsheet and the roster disagree and
	// only you can say which is right: the sheet counts that rung as three
	// calls everywhere it appears, while "the lead of every ABT" is seven with
	// seven ABTs. Naming three teams here makes it three; leaving it empty
	// keeps the literal reading. Either way the resolver and the sheet can be
	// made to agree without a release.
	TeamLeads []string `yaml:"teamLeads"`
	// Americas is the team covering the night shift.
	Americas string `yaml:"americas"`
	// Leadership is the team the two heads belong to.
	Leadership string `yaml:"leadership"`
	// AmericasLead overrides who the America Team lead is, by name.
	//
	// Normally left empty: the America Team lead is whoever holds role
	// americas_team_lead in the Americas team (migration 0185), set from the
	// Team Schedule. This file is public, so naming a person here publishes
	// their email -- keep it for an emergency override only.
	AmericasLead Person `yaml:"americasLead"`
	// SRE are the SRE team keys, in the order that breaks a tie when an
	// incident belongs to no SRE team and two could answer a rung. Never read
	// from the file: the SRE section names its teams as sre.teams.abts, the
	// same key the CRE section uses for its own, and cmd/server copies them
	// here for the one resolver both ladders share.
	SRE []string `yaml:"-"`
	// Aliases maps an assignment group's name to the rota key it stands for,
	// for a group whose name is not simply its key -- the alert flow assigns
	// "SRE - Apollo", the rota calls it apollo.
	Aliases map[string]string `yaml:"aliases"`
}

const defaultLeadershipTeamKey = "cre-leadership"

// Role names, as migration 0170 constrains team_member.role.
//
// roleSubLead is no longer a rung: the updated rules make LEVEL_1 the team's
// one lead and LEVEL_2 every lead, so nothing resolves to a sub lead any more.
// The value stays in the schema and here because rows still carry it, and
// removing it would be a data migration for no gain.
const (
	roleSubLead = "sub_lead"
	roleLead    = "lead"
	// roleAmericasTeamLead is the one position above the Americas team's
	// three Team leads: night LEVEL_2 (migration 0185).
	roleAmericasTeamLead = "americas_team_lead"
	roleCREHead          = "cre_head"
	roleCSHead           = "cs_head"
)

// Resolve implements Resolver by looking the incident's rule up and asking
// that rule's source for the rung.
//
// An empty slice is a valid answer everywhere below: a rung that reaches
// nobody is logged and climbed past, never an error. An error is reserved for
// not being able to ask at all -- entity-service unreachable, or refusing.
func (r TeamScheduleResolver) Resolve(ctx context.Context, level Level, rc RoutingContext) ([]Recipient, error) {
	if level < Level0 || level > Level4 {
		return nil, fmt.Errorf("escalation: no rung %s", level)
	}
	var (
		out []Recipient
		err error
	)
	if rc.Ladder == LadderSRE {
		out, err = r.resolveSRE(ctx, level, rc)
	} else {
		rule, ok := r.RuleFor(rc)
		if !ok {
			// No row covers this shift. Not an error: the ladder reports the
			// miss and climbs, which is the same shape as a rung with nobody
			// on it.
			return nil, nil
		}
		out, err = r.fromSource(ctx, rule.Levels[level], rc)
	}
	for i := range out {
		if out[i].Phone == "" {
			out[i].Phone = r.phones.NumberFor(out[i].Email)
		}
	}
	return out, err
}

// RuleFor is which row of the table an incident routes by.
//
// "Is assigned to an ABT team" is answered from the configured ABT keys, not
// from a flag on the payload. The old table needed a publisher to say, no
// publisher ever did, and every incident routed as UNKNOWN_ABT as a result.
// The question is answerable from data already in hand, so it is answered.
func (r TeamScheduleResolver) RuleFor(rc RoutingContext) (Rule, bool) {
	return r.RuleForCtx(context.Background(), rc)
}

// RuleForCtx is RuleFor with a context, since answering "is this an ABT team"
// may mean asking entity-service when the ABT is resolved by type.
func (r TeamScheduleResolver) RuleForCtx(ctx context.Context, rc RoutingContext) (Rule, bool) {
	if rc.Ladder == LadderSRE {
		// The rule table is the CRE ladder's; an SRE plan must not report one
		// of its rows.
		return Rule{}, false
	}
	// keyOf, not teamKeyFor: a case carries its team's NAME ("Rigel") and a
	// deployment whose keys are not slugs of it (DEV: rigel_abt_cre_team)
	// maps it through teams.aliases. Without the alias every ABT case read
	// as "not on an ABT" and routed by R3.
	key := r.keyOf(rc.AssignedCRETeam)
	// An incident with no team is a DEFINITE "not assigned to an ABT team",
	// not an unknown. Treating it as unknown left every ABTYes/ABTNo row
	// unmatchable, and LK and LK_EVENING have only those two rows each -- so
	// an unassigned incident during business hours matched nothing, every rung
	// resolved to nobody, and it was never paged at all. Not being on a team
	// is exactly the case R3 and R4b exist for.
	return MatchRule(r.rules, rc.Shift, r.isABT(ctx, key), true)
}

// isABT answers the rule table's "is this assigned to a team in the ABT"
// column.
//
// From the configured keys when there are some; otherwise by asking whether the
// team's own type is this ladder's ABT, which is where the grouping actually
// lives. Americas is type cre, not cre-abt, so it correctly answers no -- which
// is what sends a night incident down R5 rather than R2.
func (r TeamScheduleResolver) isABT(ctx context.Context, teamKey string) bool {
	if teamKey == "" {
		return false
	}
	for _, k := range r.abtTeamKeys {
		if k == teamKey {
			return true
		}
	}
	if len(r.abtTeamKeys) > 0 || r.abtType == "" {
		return false
	}
	members, err := r.entity.TeamMembers(ctx, []string{teamKey}, nil, nil, nil)
	if err != nil || len(members) == 0 {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(members[0].TeamType), r.abtType)
}

// fromSource answers one rung.
func (r TeamScheduleResolver) fromSource(ctx context.Context, src LevelSource, rc RoutingContext) ([]Recipient, error) {
	switch src {
	case SourceNone:
		return nil, nil

	case SourceRotaMembers:
		return r.rotaMembers(ctx, rc.At, rc.Shift)

	case SourceRotaPair:
		return r.rotaPair(ctx, rc.At, r.keyOf(rc.AssignedCRETeam))

	case SourceAlertDutyOwnABT:
		key := r.keyOf(rc.AssignedCRETeam)
		if key == "" {
			return nil, nil
		}
		return r.alertDuty(ctx, []string{key})

	case SourceAlertDutyEachABT:
		return r.oneNomineePerABTTeam(ctx)

	case SourceAlertDutyAmericas:
		return r.alertDuty(ctx, r.americasKeys())

	case SourceRotaMemberAndAlertDutyAmericas:
		rota, err := r.rotaMembersOnShifts(ctx, rc.At, r.americasWeekendRotaShifts)
		if err != nil {
			return nil, err
		}
		nominees, err := r.alertDuty(ctx, r.americasKeys())
		if err != nil {
			return nil, err
		}
		// One rota member, as the sheet's own count says, plus the nominees.
		if len(rota) > 1 {
			rota = rota[:1]
		}
		return dedupeRecipients(append(rota, nominees...)), nil

	case SourceTeamLead:
		// Not on an ABT means there is no own-team lead to take, so fall back
		// to the shared pool rather than leaving the rung dead -- see
		// TeamKeys.UnassignedTeamLead for why skipping was wrong here.
		//
		// The test is isABT, the same predicate the rule table routes by, NOT
		// an empty team string. teamKeyFor slugifies whatever the incident
		// carries, so an unmapped ServiceNow group yields a perfectly
		// non-empty key that simply belongs to no ABT -- which is the common
		// shape of an unassigned incident in practice, and the one an
		// emptiness check silently missed.
		if r.unassignedTeamLead != unassignedLeadNone && !r.isABT(ctx, r.keyOf(rc.AssignedCRETeam)) {
			pool, err := r.teamLeadPool(ctx)
			if err != nil {
				return nil, err
			}
			return r.takeLongestSinceCalled(ctx, pool, r.unassignedTeamLeadCount), nil
		}
		return r.abtMembers(ctx, rc.AssignedCRETeam, roleLead)

	case SourceAllTeamLeads:
		pool, err := r.teamLeadPool(ctx)
		if err != nil {
			return nil, err
		}
		return r.takeLongestSinceCalled(ctx, pool, r.teamLeadsToCall), nil

	case SourceAmericasTeamLeads:
		// The Americas team's three Team leads (role lead). The America Team
		// lead above them holds a role of its own (migration 0185), so it is
		// never in this list and LEVEL_2 always reaches somebody new.
		leads, err := r.leadsOf(ctx, r.americasKeys())
		if err != nil {
			return nil, err
		}
		if !r.americasLead.Set() {
			return leads, nil
		}
		out := make([]Recipient, 0, len(leads))
		for _, l := range leads {
			if !strings.EqualFold(l.Email, r.americasLead.Email) {
				out = append(out, l)
			}
		}
		return out, nil

	case SourceAmericasTeamLead:
		// The America Team lead: the Americas team's one americas_team_lead.
		// Nobody holding it means nobody to call -- the rung is recorded as
		// having no recipients and the ladder climbs, rather than guessing one
		// of the three Team leads. teams.americasLead, when set, still wins.
		if r.americasLead.Set() {
			return []Recipient{{Name: r.americasLead.Name, Email: r.americasLead.Email, Phone: r.americasLead.Phone}}, nil
		}
		keys := r.americasKeys()
		if len(keys) == 0 {
			return nil, nil
		}
		members, err := r.entity.TeamMembers(ctx, keys, []string{roleAmericasTeamLead}, nil, nil)
		if err != nil {
			return nil, err
		}
		out := recipientsOf(members)
		sortRecipients(out)
		return out, nil

	case SourceCREHead:
		if p := r.heads.CRE; p.Set() {
			return []Recipient{{Name: p.Name, Email: p.Email, Phone: p.Phone}}, nil
		}
		return r.head(ctx, roleCREHead)

	case SourceCSHead:
		if p := r.heads.CS; p.Set() {
			return []Recipient{{Name: p.Name, Email: p.Email, Phone: p.Phone}}, nil
		}
		return r.head(ctx, roleCSHead)
	}
	return nil, fmt.Errorf("escalation: unknown level source %q", src)
}

func (r TeamScheduleResolver) americasKeys() []string {
	if r.americasTeamKey == "" {
		return nil
	}
	return []string{r.americasTeamKey}
}

// alertTiers is every nomination, in the order the sheet writes them.
var alertTiers = []string{"T1", "T2", "T3"}

// rotaMembers is everybody rostered at that instant, in a stable order.
// onDutyHere is who is on duty at the instant, limited to this ladder's teams.
//
// The rota rungs read the Team Schedule's on-duty list, which covers every
// rota -- the SRE teams' included. Taken whole, a CRE ladder's first
// responders were mostly SRE engineers. See TeamKeys.RotaTeams.
func (r TeamScheduleResolver) onDutyHere(ctx context.Context, at time.Time) ([]onDutyAssignment, error) {
	onDuty, err := r.entity.OnDutyAt(ctx, at)
	if err != nil {
		return nil, err
	}
	known := map[string]bool{}
	var out []onDutyAssignment
	for _, a := range onDuty {
		key := teamKeyFor(a.TeamKey)
		ok, seen := known[key]
		if !seen {
			ok = r.rotaTeamCounts(ctx, key)
			known[key] = ok
		}
		if ok {
			out = append(out, a)
		}
	}
	return out, nil
}

// rotaTeamCounts reports whether a team's rota belongs to this ladder.
func (r TeamScheduleResolver) rotaTeamCounts(ctx context.Context, key string) bool {
	if key == "" {
		return false
	}
	if len(r.rotaTeamKeys) > 0 {
		return slices.Contains(r.rotaTeamKeys, key)
	}
	return key == r.americasTeamKey || r.isABT(ctx, key)
}

// lowerKeys normalises configured team keys the way teamKeyFor does.
func lowerKeys(in []string) []string {
	var out []string
	for _, k := range in {
		if k = teamKeyFor(k); k != "" {
			out = append(out, k)
		}
	}
	return out
}

// normaliseRotaCaps upper-cases the configured shift keys so the map can be
// looked up by Shift directly.
func normaliseRotaCaps(in map[string]int) map[Shift]int {
	if len(in) == 0 {
		return nil
	}
	out := make(map[Shift]int, len(in))
	for k, v := range in {
		out[Shift(strings.ToUpper(strings.TrimSpace(k)))] = v
	}
	return out
}

func (r TeamScheduleResolver) rotaMembers(ctx context.Context, at time.Time, shift Shift) ([]Recipient, error) {
	onDuty, err := r.onDutyHere(ctx, at)
	if err != nil {
		return nil, err
	}
	var out []Recipient
	seen := map[string]bool{}
	for _, a := range onDuty {
		if a.Engineer.UserID == "" || seen[a.Engineer.UserID] {
			continue
		}
		seen[a.Engineer.UserID] = true
		out = append(out, Recipient{
			Email: a.Engineer.Email, Name: a.Engineer.Name, ShiftCode: a.ShiftCode,
		})
	}
	sortRecipients(out)
	// Uncapped unless this shift is named in configuration -- "Rota Members"
	// is everyone on duty. See TeamKeys.RotaMembersToCall.
	if n := r.rotaMembersToCall[shift]; n > 0 && len(out) > n {
		return r.takeLongestSinceCalled(ctx, out, n), nil
	}
	return out, nil
}

// rotaMembersOnShifts is everyone on duty at `at` whose assignment is one of
// the given shift codes, in the same stable order rotaMembers uses.
func (r TeamScheduleResolver) rotaMembersOnShifts(ctx context.Context, at time.Time, codes []string) ([]Recipient, error) {
	onDuty, err := r.onDutyHere(ctx, at)
	if err != nil {
		return nil, err
	}
	var out []Recipient
	seen := map[string]bool{}
	for _, a := range onDuty {
		if a.Engineer.UserID == "" || seen[a.Engineer.UserID] {
			continue
		}
		if !slices.Contains(codes, strings.ToUpper(strings.TrimSpace(a.ShiftCode))) {
			continue
		}
		seen[a.Engineer.UserID] = true
		out = append(out, Recipient{
			Email: a.Engineer.Email, Name: a.Engineer.Name, ShiftCode: a.ShiftCode,
		})
	}
	sortRecipients(out)
	return out, nil
}

// defaultAmericasWeekendRotaShift is the catalogue's "Americas weekend" rota
// (migration 0154). Its on-call twin, CRE_WEEKEND_NIGHT_OC, is not a default.
const defaultAmericasWeekendRotaShift = "CRE_WEEKEND_NIGHT"

// americasWeekendShifts upper-cases the configured codes, defaulting to the
// Americas weekend rota.
func americasWeekendShifts(in []string) []string {
	var out []string
	for _, c := range in {
		if c = strings.ToUpper(strings.TrimSpace(c)); c != "" {
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		out = []string{defaultAmericasWeekendRotaShift}
	}
	return out
}

// rotaPair is the evening rule: the rostered member from the incident's own
// team, and one other member of the same rota.
//
// "One other" is deliberately deterministic -- the next member in the rota's
// stable order -- so the same incident always calls the same two people. An
// arbitrary pick would make a retry reach somebody different from the first
// attempt and make the rung untestable.
func (r TeamScheduleResolver) rotaPair(ctx context.Context, at time.Time, teamKey string) ([]Recipient, error) {
	onDuty, err := r.onDutyHere(ctx, at)
	if err != nil {
		return nil, err
	}

	var own, others []Recipient
	seen := map[string]bool{}
	for _, a := range onDuty {
		if a.Engineer.UserID == "" || seen[a.Engineer.UserID] {
			continue
		}
		seen[a.Engineer.UserID] = true
		rec := Recipient{Email: a.Engineer.Email, Name: a.Engineer.Name, ShiftCode: a.ShiftCode}
		if teamKey != "" && teamKeyFor(a.TeamKey) == teamKey {
			own = append(own, rec)
			continue
		}
		others = append(others, rec)
	}
	sortRecipients(own)
	sortRecipients(others)

	var out []Recipient
	if len(own) > 0 {
		out = append(out, own[0])
	}
	pool := others
	if len(pool) == 0 && len(own) > 1 {
		// Nobody from another team is on: a second member of the same team is
		// still a second pair of eyes, which is what the rule is for.
		pool = own[1:]
	}
	if second, ok := r.longestSinceCalled(ctx, pool); ok {
		out = append(out, second)
	}
	return out, nil
}

// longestSinceCalled picks whoever has gone longest without a call, so the
// evening's second call rotates around the rota instead of always landing on
// the same person.
//
// Never called counts as the longest wait of all, which is what brings
// somebody new into the rotation the first time. Ties -- including the case
// where there is no history at all -- break on email, so the answer stays
// deterministic and a retry reaches the same person as the first attempt.
func (r TeamScheduleResolver) longestSinceCalled(ctx context.Context, pool []Recipient) (Recipient, bool) {
	picked := r.takeLongestSinceCalled(ctx, pool, 1)
	if len(picked) == 0 {
		return Recipient{}, false
	}
	return picked[0], true
}

// takeLongestSinceCalled picks n from the pool, those who have gone longest
// without a call first. n of zero or more than the pool holds takes all of it.
//
// Never called counts as the longest wait of all, which is what brings
// somebody new into the rotation the first time. Ties -- including the case
// where there is no history at all -- break on email, so the answer stays
// deterministic and a retry reaches the same people as the first attempt.
func (r TeamScheduleResolver) takeLongestSinceCalled(ctx context.Context, pool []Recipient, n int) []Recipient {
	if len(pool) == 0 {
		return nil
	}
	ordered := append([]Recipient(nil), pool...)

	if r.history != nil {
		emails := make([]string, 0, len(ordered))
		for _, p := range ordered {
			emails = append(emails, p.Email)
		}
		seen, err := r.history.LastCalled(ctx, emails)
		if err != nil {
			// Fairness is not worth failing a rung over: fall back to the
			// stable order, which is still deterministic.
			slog.WarnContext(ctx, "escalation: could not read call history; the rung falls back to a stable order",
				"err", err)
		} else {
			sort.SliceStable(ordered, func(i, j int) bool {
				ai, oki := seen[ordered[i].Email]
				aj, okj := seen[ordered[j].Email]
				if oki != okj {
					return !oki // never called sorts first
				}
				if oki && !ai.Equal(aj) {
					return ai.Before(aj)
				}
				return ordered[i].Email < ordered[j].Email
			})
		}
	}

	if n <= 0 || n >= len(ordered) {
		return ordered
	}
	return ordered[:n]
}

// alertDuty is every nominee of the named teams.
func (r TeamScheduleResolver) alertDuty(ctx context.Context, teamKeys []string) ([]Recipient, error) {
	if len(teamKeys) == 0 {
		return nil, nil
	}
	members, err := r.entity.TeamMembers(ctx, teamKeys, nil, r.alertTiers(), nil)
	if err != nil {
		return nil, err
	}
	// NOT sortRecipients: takePerTeam has already put each team's nominees in
	// nomination order, and sorting by email here threw that away -- T3 was
	// called before T1 whenever T3's address happened to sort first. The
	// tiers are an order, not three interchangeable labels, so the rung must
	// offer them in it.
	return recipientsOf(r.takePerTeam(members)), nil
}

func (r TeamScheduleResolver) alertTiers() []string {
	if len(r.tiers) == 0 {
		return alertTiers
	}
	return r.tiers
}

// takePerTeam keeps at most perTeam nominees from each team, lowest tier
// first. Zero keeps all of them, which is "the team's nominees"; one makes it
// "one nominee from each team".
func (r TeamScheduleResolver) takePerTeam(members []teamMember) []teamMember {
	// perTeam <= 0 means "all of them", which is a question of how many to
	// keep, not of what order to offer them in -- it used to return the rows
	// in whatever order entity-service listed them, so "all three nominees"
	// came back unordered while "the first two" came back T1 then T2. Sort
	// either way and let only the truncation depend on perTeam.
	byTeam := map[string][]teamMember{}
	var order []string
	for _, m := range members {
		if _, seen := byTeam[m.TeamKey]; !seen {
			order = append(order, m.TeamKey)
		}
		byTeam[m.TeamKey] = append(byTeam[m.TeamKey], m)
	}
	var out []teamMember
	for _, key := range order {
		group := byTeam[key]
		sort.Slice(group, func(i, j int) bool {
			if group[i].AlertTier != group[j].AlertTier {
				return group[i].AlertTier < group[j].AlertTier
			}
			return group[i].Email < group[j].Email
		})
		if r.perTeam > 0 && len(group) > r.perTeam {
			group = group[:r.perTeam]
		}
		out = append(out, group...)
	}
	return out
}

// oneNomineePerTeam takes a single nominee from each team, lowest tier first.
//
// This is R3: an incident assigned to no ABT reaches one person in every ABT,
// so whichever team it turns out to belong to has somebody already looking.
// With seven ABTs that is seven calls -- more than the sheet's own count for
// that row, which is recorded on the rule and checked against.
func (r TeamScheduleResolver) oneNomineePerTeam(ctx context.Context, teamKeys []string) ([]Recipient, error) {
	if len(teamKeys) == 0 {
		return nil, nil
	}
	members, err := r.entity.TeamMembers(ctx, teamKeys, nil, r.alertTiers(), nil)
	if err != nil {
		return nil, err
	}

	return r.pickOnePerTeam(members), nil
}

// pickOnePerTeam keeps the lowest tier from each team, breaking ties on email
// so the choice is stable across calls and a retry reaches the same people.
func (r TeamScheduleResolver) pickOnePerTeam(members []teamMember) []Recipient {
	byTeam := map[string]teamMember{}
	var order []string
	for _, m := range members {
		cur, seen := byTeam[m.TeamKey]
		if !seen {
			order = append(order, m.TeamKey)
		}
		if !seen || m.AlertTier < cur.AlertTier ||
			(m.AlertTier == cur.AlertTier && m.Email < cur.Email) {
			byTeam[m.TeamKey] = m
		}
	}
	sort.Strings(order)
	out := make([]Recipient, 0, len(order))
	for _, key := range order {
		m := byTeam[key]
		out = append(out, Recipient{Email: m.Email, Name: m.Name})
	}
	return out
}

// leadsOf is the lead of each named team.
// teamLeadPool is everybody the "Team leads" rung may call.
// How the "Team lead" rung answers for an incident on no ABT.
const (
	unassignedLeadPool = "pool"
	unassignedLeadNone = "none"
)

// unassignedLeadMode defaults an unset value to "pool".
//
// Defaulting to the fallback rather than to the old behaviour is deliberate:
// the old behaviour is a rung that can never fire, which reads as coverage and
// is not. A deployment that wants it back says so explicitly.
func unassignedLeadMode(v string) string {
	if strings.EqualFold(strings.TrimSpace(v), unassignedLeadNone) {
		return unassignedLeadNone
	}
	// Everything else, including unset, is the pool. The resolver tests the
	// field against unassignedLeadNone rather than for equality with "pool",
	// so a resolver built as a struct literal -- as the tests and the local
	// harness do -- gets the same behaviour from its zero value.
	return unassignedLeadPool
}

// unassignedLeadCount defaults to one, which is what the rules sheet counts at
// Level 1. Zero would mean "all of them" to takeLongestSinceCalled, turning
// Level 1 into Level 2, so it is treated as unset.
func unassignedLeadCount(n int) int {
	if n <= 0 {
		return 1
	}
	return n
}

func (r TeamScheduleResolver) teamLeadPool(ctx context.Context) ([]Recipient, error) {
	if len(r.teamLeadKeys) > 0 {
		return r.leadsOf(ctx, r.teamLeadKeys)
	}
	return r.leadsOfABT(ctx)
}

// LeadPool implements LeadPoolResolver: every lead the lead tiers
// (LEVEL_1, LEVEL_2) can reach for an ABT incident.
func (r TeamScheduleResolver) LeadPool(ctx context.Context) ([]Recipient, error) {
	return r.teamLeadPool(ctx)
}

// leadsOfABT is every lead in this ladder's ABT, resolved by type so a team
// added to the ABT is reached without a config change.
func (r TeamScheduleResolver) leadsOfABT(ctx context.Context) ([]Recipient, error) {
	if r.abtType == "" {
		return nil, nil
	}
	members, err := r.entity.TeamMembers(ctx, nil, []string{roleLead}, nil, []string{r.abtType})
	if err != nil {
		return nil, err
	}
	out := recipientsOf(members)
	sortRecipients(out)
	return out, nil
}

// oneNomineePerABTTeam takes a single nominee from every team in this ABT.
func (r TeamScheduleResolver) oneNomineePerABTTeam(ctx context.Context) ([]Recipient, error) {
	if len(r.abtTeamKeys) > 0 {
		return r.oneNomineePerTeam(ctx, r.abtTeamKeys)
	}
	if r.abtType == "" {
		return nil, nil
	}
	members, err := r.entity.TeamMembers(ctx, nil, nil, r.alertTiers(), []string{r.abtType})
	if err != nil {
		return nil, err
	}
	return r.pickOnePerTeam(members), nil
}

func (r TeamScheduleResolver) leadsOf(ctx context.Context, teamKeys []string) ([]Recipient, error) {
	if len(teamKeys) == 0 {
		return nil, nil
	}
	members, err := r.entity.TeamMembers(ctx, teamKeys, []string{roleLead}, nil, nil)
	if err != nil {
		return nil, err
	}
	out := recipientsOf(members)
	sortRecipients(out)
	return out, nil
}

func sortRecipients(rs []Recipient) {
	sort.Slice(rs, func(i, j int) bool { return rs[i].Email < rs[j].Email })
}

func dedupeRecipients(rs []Recipient) []Recipient {
	seen := map[string]bool{}
	out := rs[:0]
	for _, r := range rs {
		if r.Email == "" || seen[r.Email] {
			continue
		}
		seen[r.Email] = true
		out = append(out, r)
	}
	return out
}

func (r TeamScheduleResolver) abtMembers(ctx context.Context, team, role string) ([]Recipient, error) {
	key := r.keyOf(team)
	if key == "" {
		// An incident with no team is a real, reported state, not a failure:
		// the rule table routes it to a shift-wide pool. There is no pool to
		// read here yet, so this rung reaches nobody and the ladder climbs.
		return nil, nil
	}
	members, err := r.entity.TeamMembers(ctx, []string{key}, []string{role}, nil, nil)
	if err != nil {
		return nil, err
	}
	return recipientsOf(members), nil
}

func (r TeamScheduleResolver) head(ctx context.Context, role string) ([]Recipient, error) {
	members, err := r.entity.TeamMembers(ctx, []string{r.leadershipTeamKey}, []string{role}, nil, nil)
	if err != nil {
		return nil, err
	}
	return recipientsOf(members), nil
}

// teamKeyFor turns the team an incident names into the key the rota uses.
//
// KNOWN GAP. The incident carries ServiceNow's assignment group name ("Vega",
// possibly "Team Vega"); the rota is keyed "vega". Lower-casing and trimming
// is enough for the names seeded today and is certainly not enough in general
// -- a group whose name is not simply its key in another case will resolve to
// nobody, and the rung will read as unstaffed rather than unmapped. A real
// mapping belongs wherever the group is defined, not guessed at here.
func teamKeyFor(team string) string {
	return strings.ToLower(strings.TrimSpace(team))
}

// recipientsOf carries no Phone: "user" has no phone column, so a rota
// resolved recipient can only be reached over chat today. The ladder already
// records a recipient without a number as [NO_NUMBER] and carries on, so this
// degrades rather than fails -- but a voice run needs numbers from somewhere
// before it can reach anybody.
func recipientsOf(members []teamMember) []Recipient {
	var out []Recipient
	for _, m := range members {
		out = append(out, Recipient{Email: m.Email, Name: m.Name})
	}
	return out
}
