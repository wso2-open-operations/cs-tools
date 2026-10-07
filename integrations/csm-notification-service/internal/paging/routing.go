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
	"strings"
	"unicode"
)

// Routing decides which ladders an incident climbs. It is configuration --
// the routing section of the escalation file -- rather than code, so a new way
// onto a ladder (a priority, a monitoring source, a kind of team) is a file
// change, not a release.
//
// An incident climbs every ladder named by any rule it matches. A rule matches
// when every condition it states matches; a condition it leaves out matches
// anything. Each ladder's own engine asks only about itself, so the two
// ladders are decided independently and an incident can climb both.
type Routing struct {
	Rules []RouteRule `yaml:"rules"`
}

// RouteRule is one way onto one or more ladders.
type RouteRule struct {
	// Name is reported on the ladder's log line and plan, so a reader can see
	// which rule put an incident on which ladder.
	Name string `yaml:"name"`
	// When is the rule's conditions.
	When RouteWhen `yaml:"when"`
	// Ladders the incident climbs when the rule matches: cre, sre, or both.
	Ladders []string `yaml:"ladders"`
}

// RouteWhen is a rule's conditions. Every list is an allowlist; an empty one
// matches anything. Values are compared case-insensitively.
type RouteWhen struct {
	// Team is the family of the incident's assignment group:
	//   sre   an SRE team (sre.teams.abts, or the Team Schedule's SRE family)
	//   cre   any other team the rota knows
	//   none  no assignment group, or one the rota does not know
	Team []string `yaml:"team"`
	// ContactType is how the incident was raised -- AZURE, SITE_247 and
	// SENTINEL for monitoring sources, EMAIL, PHONE and so on for people.
	// Punctuation is ignored, so SITE_247 also matches SITE_24_7.
	ContactType []string `yaml:"contactType"`
	// Priority accepts codes, labels and S-codes alike, compared in
	// P-notation: P0, CATASTROPHIC and S0 are one priority.
	Priority []string `yaml:"priority"`
	// Record is what the event is about: case (a customer case, S0-S4) or
	// incident. Empty matches both.
	Record []string `yaml:"record"`
	// ExcludeTeam refuses these team families, where Team would admit them.
	// Unlike a Team list it leaves the rule taking team-less records (see
	// AdmitsNoTeam), which is how the monitoring rule keeps "no team" while
	// refusing a CRE team's incident.
	ExcludeTeam []string `yaml:"excludeTeam"`
}

// Team families a RouteWhen.Team may name.
const (
	TeamFamilySRE  = "sre"
	TeamFamilyCRE  = "cre"
	TeamFamilyNone = "none"
)

// DefaultRouting is what a deployment gets with no routing section: the Case
// Paging rules (task-call-alert-flow/Case Paging Rules.xlsx, agreed
// 2026-10-07).
//
//	customer case S0        -> CRE + SRE
//	customer case S1-S4     -> CRE only (S4 only if cre.trigger.priorities lists it)
//	SRE incident            -> SRE only, any priority
//	incident on a CRE team  -> nobody: CRE pages from cases only
var DefaultRouting = Routing{Rules: []RouteRule{
	// Every customer case pages CRE; cre.trigger.priorities decides which
	// severities. No team condition, so a case on no ABT still pages: the
	// rule table has rows for it (R3, R4b).
	{Name: "cre-case", When: RouteWhen{Record: []string{RecordCase}}, Ladders: []string{LadderKeyCRE}},
	// Only an S0 (Catastrophic) case needs SRE too; S1 Critical and below page
	// CRE alone.
	{Name: "cre-p0", When: RouteWhen{Record: []string{RecordCase}, Priority: []string{"S0"}}, Ladders: []string{LadderKeySRE}},
	// The SRE rules sheet's "Is assigned to an SRE ABT team = Yes" rows.
	{Name: "sre-abt-team", When: RouteWhen{Record: []string{RecordIncident}, Team: []string{TeamFamilySRE}}, Ladders: []string{LadderKeySRE}},
	// The sheet's "= No" rows: raised by monitoring, on an SRE team or none.
	// A CRE team's incident is excluded: CRE work arrives as a case.
	{Name: "monitoring", When: RouteWhen{Record: []string{RecordIncident}, ContactType: []string{"AZURE", "SITE_247", "SENTINEL"},
		ExcludeTeam: []string{TeamFamilyCRE}}, Ladders: []string{LadderKeySRE}},
}}

// RouteInput is what routing decides on.
type RouteInput struct {
	Record      string // case or incident
	Team        string // sre, cre or none
	ContactType string
	Priority    string
}

func (r Routing) rules() []RouteRule {
	if r.Rules == nil {
		return DefaultRouting.Rules
	}
	return r.Rules
}

// Match returns the first rule, in file order, that matches the input and
// puts it on ladder (a ladder key, cre or sre).
func (r Routing) Match(in RouteInput, ladder string) (RouteRule, bool) {
	for _, rule := range r.rules() {
		if !contains(rule.Ladders, ladder) {
			continue
		}
		if rule.When.matches(in) {
			return rule, true
		}
	}
	return RouteRule{}, false
}

func (w RouteWhen) matches(in RouteInput) bool {
	if len(w.Record) > 0 && !contains(w.Record, in.Record) {
		return false
	}
	if len(w.Team) > 0 && !contains(w.Team, in.Team) {
		return false
	}
	if contains(w.ExcludeTeam, in.Team) {
		return false
	}
	if len(w.ContactType) > 0 && !containsContact(w.ContactType, in.ContactType) {
		return false
	}
	if len(w.Priority) > 0 && !containsPriority(w.Priority, in.Priority) {
		return false
	}
	return true
}

// AdmitsNoTeam reports whether the rule deliberately takes incidents with no
// known team: it does not constrain the team at all (the monitoring rule).
// Such a rule overrides the ladder's trigger.requireKnownTeam.
//
// A rule that names none among its teams (cre-team) only MATCHES team-less
// incidents; whether that ladder then takes them is still its own
// trigger.requireKnownTeam's call. That is how the CRE rule table keeps its
// "assigned to no ABT" rows while a deployment can still refuse such
// incidents -- the CRE side owns that decision, not routing.
func (r RouteRule) AdmitsNoTeam() bool {
	return len(r.When.Team) == 0
}

// containsPriority compares both sides in one spelling, P-notation, so a rule
// may name a priority as a code, a label or an S-code (P0, CATASTROPHIC, S0)
// and match an incident carrying any of them.
func containsPriority(list []string, priority string) bool {
	want := NormalisePriority(priority)
	if want == "" {
		return false
	}
	for _, p := range list {
		if NormalisePriority(p) == want {
			return true
		}
	}
	return false
}

// containsContact compares contact types ignoring case and punctuation, so
// the incident view's SITE_247 and the database's SITE_24_7 are the same.
func containsContact(list []string, contactType string) bool {
	want := normalizeContact(contactType)
	if want == "" {
		return false
	}
	for _, c := range list {
		if normalizeContact(c) == want {
			return true
		}
	}
	return false
}

func normalizeContact(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
