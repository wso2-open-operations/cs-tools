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

import "fmt"

// LevelSource says where one rung's recipients come from.
//
// It is a named source rather than a function so the rule table stays data:
// the spreadsheet the rules come from has one row per (shift, is-assigned-to-
// an-ABT) pair and one cell per rung, and this is that cell. Keeping it
// diffable against the sheet is the whole point -- the previous table drifted
// from its own specification precisely because the rules lived inside a
// switch.
type LevelSource string

const (
	// SourceNone is a rung this rule does not have.
	SourceNone LevelSource = ""

	// -- Level 0, "first responders": whoever is already watching. --

	// SourceRotaMembers is everyone rostered on the shift covering the
	// incident's instant, whatever team they are on.
	SourceRotaMembers LevelSource = "rota_members"
	// SourceRotaPair is the incident's own ABT member on that rota, plus one
	// other member of the same rota. The evening shift's rule: somebody who
	// knows the product, and a second pair of eyes.
	SourceRotaPair LevelSource = "rota_pair"
	// SourceAlertDutyOwnABT is the nominated set (T1/T2/T3) of the ABT the
	// incident is assigned to.
	SourceAlertDutyOwnABT LevelSource = "alert_duty_own_abt"
	// SourceAlertDutyEachABT is one nominee from every ABT, for an incident
	// assigned to none of them.
	SourceAlertDutyEachABT LevelSource = "alert_duty_each_abt"
	// SourceAlertDutyAmericas is the Americas team's own nominated set.
	SourceAlertDutyAmericas LevelSource = "alert_duty_americas"
	// SourceRotaMemberAndAlertDutyAmericas is the weekend night's rule: a rota
	// member and the Americas nominees together.
	SourceRotaMemberAndAlertDutyAmericas LevelSource = "rota_member_and_alert_duty_americas"

	// -- Levels 1 and 2: rank. --

	// SourceTeamLead is THE lead of the team the incident is assigned to.
	SourceTeamLead LevelSource = "team_lead"
	// SourceAllTeamLeads is the lead of every ABT, called together.
	SourceAllTeamLeads LevelSource = "all_team_leads"
	// SourceAmericasTeamLead is the Americas team's own lead -- the one above
	// its leads.
	SourceAmericasTeamLead LevelSource = "americas_team_lead"
	// SourceAmericasTeamLeads is the Americas team's own leads, plural. The
	// alternative reading of the night shift's LEVEL_1: its three leads,
	// rather than every ABT's one lead. Selectable per rule from the
	// configuration file.
	SourceAmericasTeamLeads LevelSource = "americas_team_leads"

	// -- Levels 3 and 4: the heads, outside any ABT. --

	SourceCREHead LevelSource = "cre_head"
	SourceCSHead  LevelSource = "cs_head"
)

// ABTScope is a rule's "is assigned to an ABT team" column.
type ABTScope string

const (
	ABTAny ABTScope = "any"
	ABTYes ABTScope = "yes"
	ABTNo  ABTScope = "no"
)

// Rule is one row of the escalation rules.
type Rule struct {
	// ID is the row's own name, reported on every call and in the work note so
	// a reader can check what the ladder did against the sheet.
	ID string `yaml:"id"`
	// Shift is the effective shift this row covers.
	Shift Shift `yaml:"shift"`
	// ABT is whether the row applies to an incident assigned to an ABT.
	ABT ABTScope `yaml:"assignedToABT"`
	// Levels is LEVEL_0..LEVEL_4's source, in order.
	Levels [5]LevelSource `yaml:"levels"`
	// ExpectedCalls is how many people the sheet says each rung reaches.
	//
	// Not used to select anybody — the resolver answers that from the rota and
	// the nominations. It is the sheet's own arithmetic kept beside the rule so
	// a divergence is visible: a rung that suddenly resolves to twice its
	// expected size means a rota changed, and that should surface as a warning
	// rather than as a bill. Zero means "no expectation recorded".
	ExpectedCalls [5]int `yaml:"expectedCalls"`
}

// DefaultRules is the CRE rule table, transcribed from
// "[Updated] CRE Notification Escalation rules.xlsx".
//
// Two departures from the sheet's own layout, both deliberate:
//
//   - R1 covers LK_MORNING and LK_WEEKEND on one row, but the Twilio-calls
//     sheet gives those two shifts different Level 0 counts (2 and 3). They are
//     therefore two rules here, R1a and R1b. A single row cannot carry two
//     different expectations.
//   - R4 appears twice in the sheet under one ID, once for an incident
//     assigned to an ABT and once for one that is not. They are R4a and R4b.
//
// One known divergence, recorded rather than resolved: R3's Level 0 is "one
// nominee from each ABT", which with seven ABTs is seven calls, while the
// Twilio sheet gives that row three. The rule is implemented as written and the
// expectation as counted; safety.maxCallsPerLadder is what stops it running
// away if the sheet turns out to be right.
var DefaultRules = []Rule{
	{
		ID: "R1a", Shift: ShiftLKMorning, ABT: ABTAny,
		Levels:        [5]LevelSource{SourceRotaMembers, SourceTeamLead, SourceAllTeamLeads, SourceCREHead, SourceCSHead},
		ExpectedCalls: [5]int{2, 1, 3, 1, 1},
	},
	{
		ID: "R1b", Shift: ShiftLKWeekend, ABT: ABTAny,
		Levels:        [5]LevelSource{SourceRotaMembers, SourceTeamLead, SourceAllTeamLeads, SourceCREHead, SourceCSHead},
		ExpectedCalls: [5]int{3, 1, 3, 1, 1},
	},
	{
		ID: "R2", Shift: ShiftLK, ABT: ABTYes,
		Levels:        [5]LevelSource{SourceAlertDutyOwnABT, SourceTeamLead, SourceAllTeamLeads, SourceCREHead, SourceCSHead},
		ExpectedCalls: [5]int{3, 1, 3, 1, 1},
	},
	{
		ID: "R3", Shift: ShiftLK, ABT: ABTNo,
		Levels:        [5]LevelSource{SourceAlertDutyEachABT, SourceTeamLead, SourceAllTeamLeads, SourceCREHead, SourceCSHead},
		ExpectedCalls: [5]int{7, 1, 3, 1, 1},
	},
	{
		ID: "R4a", Shift: ShiftLKEvening, ABT: ABTYes,
		Levels:        [5]LevelSource{SourceRotaPair, SourceTeamLead, SourceAllTeamLeads, SourceCREHead, SourceCSHead},
		ExpectedCalls: [5]int{2, 1, 3, 1, 1},
	},
	{
		ID: "R4b", Shift: ShiftLKEvening, ABT: ABTNo,
		Levels:        [5]LevelSource{SourceRotaMembers, SourceTeamLead, SourceAllTeamLeads, SourceCREHead, SourceCSHead},
		ExpectedCalls: [5]int{7, 1, 3, 1, 1},
	},
	{
		// Level 1 is the AMERICAS team's own leads -- it has three, and it is
		// not an ABT. An ABT has exactly one lead, so "Team leads" here could
		// never have meant an ABT's; it is the three above the night rota and
		// below the single Americas lead at Level 2. Three calls, which is
		// what the Twilio tab counts.
		ID: "R5", Shift: ShiftUSA, ABT: ABTAny,
		Levels:        [5]LevelSource{SourceAlertDutyAmericas, SourceAmericasTeamLeads, SourceAmericasTeamLead, SourceCREHead, SourceCSHead},
		ExpectedCalls: [5]int{3, 3, 1, 1, 1},
	},
	{
		ID: "R6", Shift: ShiftUSAWeekend, ABT: ABTAny,
		Levels:        [5]LevelSource{SourceRotaMemberAndAlertDutyAmericas, SourceAmericasTeamLeads, SourceAmericasTeamLead, SourceCREHead, SourceCSHead},
		ExpectedCalls: [5]int{4, 3, 1, 1, 1},
	},
}

// MatchRule picks the row an incident routes by.
//
// abtKnown distinguishes "this incident is not on an ABT team" from "we could
// not find out". Only the second leaves the yes/no rows unmatchable, and a
// caller that genuinely cannot tell should report the miss rather than guess.
//
// A missing team is the FIRST case, not the second: an incident with no team
// is definitely not on an ABT team, which is what R3 and R4b are for. Reading
// it as unknown meant LK and LK_EVENING -- whose only rows are the yes/no
// pair -- matched nothing at all, and the incident was never paged.
func MatchRule(rules []Rule, shift Shift, assignedToABT bool, abtKnown bool) (Rule, bool) {
	for _, r := range rules {
		if r.Shift != shift {
			continue
		}
		switch r.ABT {
		case ABTAny:
			return r, true
		case ABTYes:
			if abtKnown && assignedToABT {
				return r, true
			}
		case ABTNo:
			if abtKnown && !assignedToABT {
				return r, true
			}
		}
	}
	return Rule{}, false
}

// Validate checks a rule table before it is used, so a hand-edited config
// fails at startup rather than at the first incident.
func Validate(rules []Rule) error {
	if len(rules) == 0 {
		return fmt.Errorf("escalation: the rule table is empty")
	}
	seen := make(map[string]bool, len(rules))
	for _, r := range rules {
		if r.ID == "" {
			return fmt.Errorf("escalation: a rule has no id")
		}
		if seen[r.ID] {
			return fmt.Errorf("escalation: rule %s is defined twice", r.ID)
		}
		seen[r.ID] = true
		if r.Shift == "" {
			return fmt.Errorf("escalation: rule %s names no shift", r.ID)
		}
		switch r.ABT {
		case ABTAny, ABTYes, ABTNo:
		default:
			return fmt.Errorf("escalation: rule %s has assignedToABT %q; use any, yes or no", r.ID, r.ABT)
		}
		for i, src := range r.Levels {
			if !src.valid() {
				return fmt.Errorf("escalation: rule %s level %d has unknown source %q", r.ID, i, src)
			}
		}
	}
	return nil
}

func (s LevelSource) valid() bool {
	switch s {
	case SourceNone, SourceRotaMembers, SourceRotaPair, SourceAlertDutyOwnABT,
		SourceAlertDutyEachABT, SourceAlertDutyAmericas,
		SourceRotaMemberAndAlertDutyAmericas, SourceTeamLead, SourceAllTeamLeads,
		SourceAmericasTeamLead, SourceAmericasTeamLeads, SourceCREHead, SourceCSHead:
		return true
	}
	return false
}

// Describe names a rung's source in the words a reader of the chat card or the
// work note would use.
//
// It comes from the same table the routing does, so a card can never describe
// a rung the ladder did not actually call -- which is exactly what happened
// while Level.Role() carried the previous model's names and the resolver had
// moved on.
func (s LevelSource) Describe() string {
	switch s {
	case SourceRotaMembers:
		return "Rota members"
	case SourceRotaPair:
		return "Rota members (the team's own, and one other)"
	case SourceAlertDutyOwnABT:
		return "Alert duty (this ABT)"
	case SourceAlertDutyEachABT:
		return "Alert duty (one per ABT)"
	case SourceAlertDutyAmericas:
		return "Alert duty (Americas)"
	case SourceRotaMemberAndAlertDutyAmericas:
		return "Rota member and Americas alert duty"
	case SourceTeamLead:
		return "Team lead"
	case SourceAllTeamLeads:
		return "Team leads"
	case SourceAmericasTeamLead:
		return "Americas team lead"
	case SourceAmericasTeamLeads:
		return "Americas team leads"
	case SourceCREHead:
		return "CRE head"
	case SourceCSHead:
		return "CS head"
	}
	return ""
}

// RoleAt names this rule's rung, falling back to the generic label when the
// rule has no source for it.
func (r Rule) RoleAt(l Level) string {
	if l < Level0 || l > Level4 {
		return l.Role()
	}
	if name := r.Levels[l].Describe(); name != "" {
		return name
	}
	return l.Role()
}
