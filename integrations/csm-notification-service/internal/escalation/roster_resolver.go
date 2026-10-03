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

package escalation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// RosterResolver resolves recipients from an operator-maintained roster.
//
// # WHY THIS EXISTS, AND WHAT IT IS NOT
//
// Section 5.0 resolves recipients from ServiceNow: sys_user_group_type for the
// BU/shift tag on each team, u_team_member_role for the escalation roles, and
// the On-Call Scheduling module for the effective shift and escalation paths
// (see the administrative guide for how each is configured). None of that is
// reachable from this service — there is no entity-service endpoint over any of
// those tables — and building one is its own piece of work.
//
// So this is a stopgap with an honest shape: the same lookup the rule table
// performs, against a roster an operator maintains by hand instead of against
// ServiceNow. It implements the rule table's *structure* faithfully — the
// team/BU/shift fallback order below is R1–R14's — but its data is only as
// fresh as whoever last edited the config. A rotation change that lands in
// ServiceNow and not here calls the wrong person.
//
// It exists because the alternative is worse: with no resolver at all the
// engine runs, schedules nothing, and pages nobody. Replace it with a
// ServiceNow-backed Resolver as soon as that data is reachable; nothing outside
// this file needs to change when that happens.
//
// AVAILABILITY FILTERING IS NOT IMPLEMENTED. Section 8.0 filters LEVEL_0 by
// whether the recipient is actually on shift, from the WSO2 team schedule's
// agent events. A hand-maintained roster has no equivalent, so LEVEL_0's
// configured recipients are called unconditionally. Every other level is
// contacted regardless of availability anyway, so this affects LEVEL_0 only.
type RosterResolver struct {
	roster Roster
}

// Roster is the configured recipient roster, in the shape of section 5.0's own
// selection inputs.
type Roster struct {
	// ByTeam is the most specific match: the recipients for a named CRE team
	// (rules R1, R2, R5, R6 — an incident with a product and an assigned
	// team).
	ByTeam map[string]LevelRoster `json:"byTeam,omitempty"`
	// ByShift covers the pooled cases — an unassigned team or a missing
	// product, where the rule table falls back to a shift-wide pool ("Common
	// ABT Leads Pool", "All USA Sub Team Leads", rules R3, R4, R7, R8,
	// R9–R14). Keyed by Shift.
	ByShift map[string]LevelRoster `json:"byShift,omitempty"`
	// Default is the last resort, used when neither of the above matches. The
	// leadership levels (LEVEL_3, LEVEL_4) belong here: the administrative
	// guide puts all three BU-head/CRE-head roles in one CRE_LEADS_GROUP, so
	// they are the same people for every team and shift.
	Default LevelRoster `json:"default,omitempty"`
}

// LevelRoster maps a level name ("LEVEL_0" … "LEVEL_4") to its recipients.
// Keyed by the string form so the config reads like the specification's own
// tables.
type LevelRoster map[string][]Recipient

// ParseRoster decodes a roster from JSON, as supplied by
// INCIDENT_ESCALATION_ROSTER. An empty string yields an empty roster, which
// resolves nobody at every level — the engine logs that and schedules nothing
// rather than failing.
func ParseRoster(raw string) (Roster, error) {
	if strings.TrimSpace(raw) == "" {
		return Roster{}, nil
	}
	var r Roster
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		return Roster{}, fmt.Errorf("escalation: parse roster: %w", err)
	}
	return r, nil
}

// NewRosterResolver constructs a RosterResolver.
func NewRosterResolver(r Roster) RosterResolver {
	return RosterResolver{roster: r}
}

// IsEmpty reports whether this roster would resolve nobody at any level, which
// cmd/server/main.go uses to refuse to start the engine on an empty
// configuration rather than run a ladder that can never call anyone.
func (r Roster) IsEmpty() bool {
	return len(r.ByTeam) == 0 && len(r.ByShift) == 0 && len(r.Default) == 0
}

// Resolve implements Resolver, applying the rule table's own specificity
// order: the incident's own team first, then the effective shift's pool, then
// the shared default.
//
// Only the *first* tier that has an entry for this level is used — the tiers
// are alternatives, not additive. Merging them would call a team's own sub lead
// and the shift-wide pool of every other team's sub leads for the same
// incident, which no rule in section 5.0 asks for.
func (rr RosterResolver) Resolve(_ context.Context, level Level, rc RoutingContext) ([]Recipient, error) {
	key := level.String()
	for _, tier := range []LevelRoster{
		rr.roster.ByTeam[rc.AssignedCRETeam],
		rr.roster.ByShift[string(rc.Shift)],
		rr.roster.Default,
	} {
		if people, ok := tier[key]; ok && len(people) > 0 {
			return people, nil
		}
	}
	return nil, nil
}
