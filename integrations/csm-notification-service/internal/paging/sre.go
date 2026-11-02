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
	"strings"
	"time"
)

// Ladder is which escalation ladder an incident climbs.
//
// There are two, and they share everything except who each rung is and how
// fast it climbs. The CRE ladder is the specification's: five rungs of rank
// (sub lead, lead, heads), on a clock set by the incident's priority. The SRE
// ladder follows the SRE team's own on-call tiers:
//
//	LEVEL_0  L1 support   called as soon as the incident arrives
//	LEVEL_1  L2 support   5 minutes later, if nobody has taken it
//	LEVEL_2  L3 support   5 minutes after that
//	LEVEL_3  L4 support   5 minutes after that - OPT-IN, see SREPolicy
//
// One call per rung, one person per rung, the same clock for every priority --
// and no priority gate at all: an SRE incident gets a ladder whatever its
// priority. It stops when an engineer is assigned to the incident
// (incident.assigned) or the incident leaves NEW; a public comment alone does
// not stop it.
//
// Which ladder an incident climbs is decided by its assignment group: an
// incident assigned to an SRE team climbs the SRE ladder, and everything else
// - including an incident whose team cannot be placed at all - climbs the CRE
// one. Which ladders an incident climbs is the configuration's routing
// section (routing.go): by default an SRE team's incident, a CRE team's P0,
// and any monitoring-raised incident climb this one, and the last two climb
// the CRE one as well -- side by side, each in its own engine and its own
// store namespace.
type Ladder string

const (
	// LadderCRE is the zero value on purpose: a plan stored before the SRE
	// ladder existed has no Ladder field and must keep reading as CRE.
	LadderCRE Ladder = ""
	LadderSRE Ladder = "SRE"
	// LadderSME is the Special Ops page (sme.go): ONE call to the SME on duty
	// when an SRE incident is escalated to Special Ops. Not a ladder of its
	// own -- it has no engine, no clock and no stored state -- but the plan
	// that carries its one call names it, so the card and the voice message
	// say what the call is.
	LadderSME Ladder = "SME"
)

// SRE rotas, as entity-service's team_schedule_rota spells them.
const (
	RotaSRESaaS = "SRE_SAAS"
	RotaSREIaaS = "SRE_IAAS"
	// U2 and B-Central's rotas keep the codes migration 0217 gave them when
	// they were SME rotas; migration 0242 moved them to the SRE family.
	RotaSREU2       = "SME_U2"
	RotaSREBCentral = "SME_B_CENTRAL"
)

// SREChain names the SRE chain an incident pages, for the people who read
// about it: "SaaS SRE", "IaaS SRE", "U2-SRE" or "B-Central-SRE" (named after the
// assignment group the incident comes from), "SRE" for a rota with no name here, and
// "" when the rota is unknown -- a catalogue with no rotas -- which keeps every
// card, voice message and work note exactly as it read before rotas existed.
func (rc RoutingContext) SREChain() string {
	switch strings.ToUpper(strings.TrimSpace(rc.Rota)) {
	case "":
		return ""
	case RotaSRESaaS:
		return "SaaS SRE"
	case RotaSREIaaS:
		return "IaaS SRE"
	case RotaSREU2:
		return "U2-SRE"
	case RotaSREBCentral:
		return "B-Central-SRE"
	}
	return "SRE"
}

// SRERotaResolver is implemented by a Resolver that can tell which SRE rota
// pages an incident. The engine stamps the answer on the routing context so
// the chain is named in what people read; see TeamScheduleResolver.SRERota.
type SRERotaResolver interface {
	SRERota(ctx context.Context, rc RoutingContext) (string, error)
}

// TeamFamilyResolver is implemented by a Resolver that can tell the family of
// an incident's team (sre, cre or none), which routing decides on. A resolver
// that cannot falls back to LadderClassifier, then to "cre" for any named team.
type TeamFamilyResolver interface {
	TeamFamily(ctx context.Context, rc RoutingContext) (string, error)
}

// LadderClassifier is implemented by a Resolver that can tell which ladder an
// incident belongs to. A Resolver that cannot (RosterResolver,
// StaticResolver) leaves every incident on the CRE ladder.
type LadderClassifier interface {
	LadderFor(ctx context.Context, rc RoutingContext) (Ladder, error)
}

// SREPolicyKey is the key the SRE ladder's policy is stored under in the
// policies map. It is not a priority: the SRE ladder runs one clock for every
// priority, so it has one entry rather than five.
const SREPolicyKey = "SRE"

// sreStep is the gap between two SRE rungs.
const sreStep = 5 * time.Minute

// SREPolicy is the SRE ladder's default timing: one call per rung, a rung
// every five minutes, starting the moment the incident arrives.
//
// includeL4 adds a fourth rung, L4 support. It is off by default because it is
// NOT CONFIRMED: the SRE ladder generally stops at L3, and the rota has no L4
// tier to read it from (team_schedule_tier_enum is L1, L2, L3). When on, it
// resolves to the SRE team's lead - an assumption, recorded on the resolver.
// The configuration file's sre.timing overrides all of this.
func SREPolicy(includeL4 bool) PriorityPolicy {
	return SRETiming{IncludeL4: includeL4}.Policy()
}

// PolicyFor picks the timing a trigger's ladder runs on.
//
// The CRE ladder is gated on priority: an incident whose priority has no row
// in section 7.0 (PLANNING) has no ladder at all. The SRE ladder is not --
// most SRE incidents arrive by the alert flow, and the SRE team's clock does
// not depend on priority -- so it always gets its fixed clock.
func PolicyFor(policies map[string]PriorityPolicy, t Trigger) (PriorityPolicy, bool) {
	if t.Routing.Ladder == LadderSME {
		// The SME ladder runs on sme.timing, whatever the priority.
		if sme, found := policies[SMEPolicyKey]; found {
			return sme, true
		}
		return SREPolicy(false), true
	}
	if t.Routing.Ladder != LadderSRE {
		return Lookup(policies, t.Priority)
	}
	if sre, found := policies[SREPolicyKey]; found {
		return sre, true
	}
	return SREPolicy(false), true
}

// withSREPolicy returns policies with the SRE entry set, copying rather than
// writing into the caller's map (DefaultPolicy is shared).
func withSREPolicy(policies map[string]PriorityPolicy, sre PriorityPolicy) map[string]PriorityPolicy {
	return withPolicy(policies, SREPolicyKey, sre)
}

// withPolicy returns policies with key set to p, copying the map.
func withPolicy(policies map[string]PriorityPolicy, key string, p PriorityPolicy) map[string]PriorityPolicy {
	out := make(map[string]PriorityPolicy, len(policies)+1)
	for k, v := range policies {
		out[k] = v
	}
	out[key] = p
	return out
}

// SMEPolicyKey is the SME ladder's entry in the engine's policies: its clock
// is sme.timing.
const SMEPolicyKey = "SME"

// RoleIn names who a rung is on the given ladder, for a reader in a chat space.
func (l Level) RoleIn(ladder Ladder) string {
	if ladder == LadderSME {
		if tier, ok := sreTier[l]; ok {
			return "Special Ops " + tier + " on duty"
		}
		return "Special Ops on duty"
	}
	if ladder != LadderSRE {
		return l.Role()
	}
	switch l {
	case Level0:
		return "L1 Support"
	case Level1:
		return "L2 Support"
	case Level2:
		return "L3 Support"
	case Level3:
		return "L4 Support"
	}
	return "Unknown rung"
}

// rungRole is RoleIn with the SRE chain's name in front when it is known
// ("SaaS SRE L1 Support"), so a card in a room that sees both chains says
// which one is climbing.
func rungRole(level Level, rc RoutingContext) string {
	role := level.RoleIn(rc.Ladder)
	if chain := rc.SREChain(); rc.Ladder == LadderSRE && chain != "" {
		return chain + " " + role
	}
	return role
}
