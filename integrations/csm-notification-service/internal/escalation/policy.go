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

// Package escalation turns the incident call-escalation specification into a
// concrete schedule of call attempts.
//
// This package is deliberately pure: it computes WHEN to call and at WHICH
// level, and knows nothing about Twilio, Redis, Kafka or who the recipients
// are. Resolving a level to actual people is a separate concern (see the
// Resolver seam in resolver.go) because it depends on organisation data that
// does not exist in this platform yet.
//
// WHICH "INCIDENT" THIS IS. A platform incident: the entity behind
// entity-service's POST /incidents, domain.IncidentView, and the incident.*
// events. It has two producers and they create the same thing — a person
// raising one through the CSM portal, and the SRE alert pipeline turning an
// Azure, Grafana, Site24x7 or OpenSearch alert into one (see
// integrations/sre-alert-ingestion-service, which posts to the same endpoint).
// An alert-born incident is escalated by this ladder exactly like any other;
// there is no separate "SRE incident" to distinguish.
//
// It is NOT a case. Cases are a different entity with their own endpoints
// (POST /cases), their own domain type (domain.CaseView) and their own event
// family (case.created, case.comment_added, case.acknowledged, and others) —
// which is also why these events cannot simply be renamed to case.*: those
// names are taken, by payloads that mean something else.
//
// Source: "Synchronizing Twilio Alerts for New Incoming Incidents Based on ABT
// Model [USER REFERENCE]", sections 3.0 (levels), 6.0 (level/shift matrix) and
// 7.0 (escalation timelines).
package escalation

import (
	"fmt"
	"sort"
	"time"
)

// Level is one rung of the escalation ladder.
//
// Level0 is a *notification* level, not an escalation level: per section 3.0 it
// only applies to incidents reported during a rotation (morning, evening or
// weekend), and its recipients differ by business unit — the rotation lead for
// Integration, the rotation members for IAM.
type Level int

// The specification names these rungs twice, in two vocabularies, and they are
// the same five rungs either way — section 3.0 and the section 5.0 rule table
// name them by role, section 7.0's timing table by position:
//
//	LEVEL_0  Rotation Lead / Rotation Members     (§7.0: "Rotation Lead/Member")
//	LEVEL_1  ABT Leads / Sub Leads                (§7.0: "Sub Team Lead")
//	LEVEL_2  ABT Team Leads / Sub Team Leads      (§7.0: "Team Lead")
//	LEVEL_3  Head of Business Unit                (§7.0: "Head of BU")
//	LEVEL_4  Head of Customer Success             (§7.0: "Head of CRE")
//
// Both names are given below because a reader coming from either half of the
// document should recognise the rung. The "Common/All ... Pool" variants in
// rules R3, R4, R7, R8, R13 and R14 are not extra rungs: they are the same
// LEVEL_1 and LEVEL_2 resolved to a shift-wide pool instead of one team,
// which is a Resolver concern (see resolver.go), not a change to this ladder.
const (
	Level0 Level = iota // rotation lead / rotation members — rotations only
	Level1              // ABT leads / sub leads
	Level2              // ABT team leads / sub team leads
	Level3              // head of business unit
	Level4              // head of customer success (head of CRE)
)

// String renders the level the way the specification's own execution summary
// does (LEVEL_0 … LEVEL_4), so logs and work notes match the documented format.
func (l Level) String() string {
	if l < Level0 || l > Level4 {
		return fmt.Sprintf("LEVEL_UNKNOWN(%d)", int(l))
	}
	return fmt.Sprintf("LEVEL_%d", int(l))
}

// LevelPolicy is one row of the section 7.0 timing table.
//
// The specification defines a level's duration as:
//
//	duration = (NotificationCount * NotificationInterval) + EscalationInterval
//
// i.e. make NotificationCount calls to each recipient, NotificationInterval
// apart, then wait EscalationInterval before moving to the next level.
type LevelPolicy struct {
	// NotificationCount is "Notifications Counts Per Recipient" (NC) — how
	// many times each recipient at this level is called.
	NotificationCount int
	// NotificationInterval is "Notification Interval" (NI) — the gap between
	// those calls.
	NotificationInterval time.Duration
	// EscalationInterval is "Escalation Interval" (TN) — the additional wait
	// after the last call before escalating to the next level.
	EscalationInterval time.Duration
}

// Duration is how long this level occupies before the next one begins.
func (p LevelPolicy) Duration() time.Duration {
	return time.Duration(p.NotificationCount)*p.NotificationInterval + p.EscalationInterval
}

// PriorityPolicy is one priority's complete ladder.
type PriorityPolicy struct {
	// SLAResponse is the response SLA for this priority, from the WSO2 support
	// policy. Recorded for traceability; InitialWait is what actually delays
	// the ladder.
	SLAResponse time.Duration
	// InitialWait is how long after the trigger the ladder starts — section
	// 7.0 expresses it as a percentage of the response SLA and also gives the
	// resolved minutes, which is what is encoded here.
	InitialWait time.Duration
	// Levels is Level0..Level4 in order.
	Levels [5]LevelPolicy
}

// Attempt is a single scheduled call, at one level, to every recipient
// resolved for that level.
type Attempt struct {
	Level Level
	// Ordinal is 1-based within its level: the Nth of NotificationCount calls.
	Ordinal int
	// After is the delay from the trigger (incident created, or priority
	// elevated) at which this call is due.
	After time.Duration
}

// minutes is a small helper so the table below reads like the specification.
func minutes(n int) time.Duration { return time.Duration(n) * time.Minute }

// DefaultPolicy is section 7.0's table, verbatim.
//
// Two rows in the source document are internally inconsistent, and both are
// encoded here as the formula computes them rather than as the document's
// stated total — with the divergence pinned by tests so it stays visible
// instead of being quietly resolved one way or the other:
//
//   - P4 / LEVEL_0 lists NC=2, NI=4, TN=0, which computes to 8 minutes, but
//     states 15 in the "Time to Next Escalation Level" column. 15 is also P4's
//     own initial wait, so this looks like a transcription slip; note the
//     document's stated 120-minute total for P4 only reconciles if this row is
//     15. A TN of 7 would reconcile both.
//   - P1's stated 40-minute total does not match its own rows, which sum to 38
//     using the same formula that reproduces P0, P2, P3 and P4 exactly.
//
// Until the specification's author confirms, the formula wins: it is the only
// interpretation that is self-consistent across every other row.
var DefaultPolicy = map[string]PriorityPolicy{
	"P0": {
		SLAResponse: 15 * time.Minute,
		InitialWait: minutes(0), // 0% of SLA response
		Levels: [5]LevelPolicy{
			{NotificationCount: 1, NotificationInterval: minutes(1), EscalationInterval: minutes(0)},
			{NotificationCount: 3, NotificationInterval: minutes(1), EscalationInterval: minutes(0)},
			{NotificationCount: 3, NotificationInterval: minutes(1), EscalationInterval: minutes(1)},
			{NotificationCount: 3, NotificationInterval: minutes(1), EscalationInterval: minutes(1)},
			{NotificationCount: 3, NotificationInterval: minutes(1), EscalationInterval: minutes(1)},
		},
	},
	"P1": {
		SLAResponse: time.Hour,
		InitialWait: minutes(6), // 10% of SLA response
		Levels: [5]LevelPolicy{
			{NotificationCount: 2, NotificationInterval: minutes(1), EscalationInterval: minutes(1)},
			{NotificationCount: 3, NotificationInterval: minutes(2), EscalationInterval: minutes(3)},
			{NotificationCount: 3, NotificationInterval: minutes(3), EscalationInterval: minutes(1)},
			{NotificationCount: 3, NotificationInterval: minutes(3), EscalationInterval: minutes(1)},
			{NotificationCount: 3, NotificationInterval: minutes(3), EscalationInterval: minutes(1)},
		},
	},
	"P2": {
		SLAResponse: 4 * time.Hour,
		InitialWait: minutes(9), // 4% of SLA response
		Levels: [5]LevelPolicy{
			{NotificationCount: 2, NotificationInterval: minutes(3), EscalationInterval: minutes(0)},
			{NotificationCount: 3, NotificationInterval: minutes(4), EscalationInterval: minutes(3)},
			{NotificationCount: 3, NotificationInterval: minutes(4), EscalationInterval: minutes(3)},
			{NotificationCount: 3, NotificationInterval: minutes(4), EscalationInterval: minutes(3)},
			{NotificationCount: 3, NotificationInterval: minutes(4), EscalationInterval: minutes(3)},
		},
	},
	"P3": {
		SLAResponse: 6 * time.Hour,
		InitialWait: minutes(12), // 4% of SLA response
		Levels: [5]LevelPolicy{
			{NotificationCount: 2, NotificationInterval: minutes(4), EscalationInterval: minutes(0)},
			{NotificationCount: 3, NotificationInterval: minutes(5), EscalationInterval: minutes(5)},
			{NotificationCount: 3, NotificationInterval: minutes(7), EscalationInterval: minutes(4)},
			{NotificationCount: 3, NotificationInterval: minutes(7), EscalationInterval: minutes(4)},
			{NotificationCount: 3, NotificationInterval: minutes(7), EscalationInterval: minutes(4)},
		},
	},
	"P4": {
		SLAResponse: 24 * time.Hour,
		InitialWait: minutes(15), // 1% of SLA response
		Levels: [5]LevelPolicy{
			{NotificationCount: 2, NotificationInterval: minutes(4), EscalationInterval: minutes(0)},
			{NotificationCount: 3, NotificationInterval: minutes(8), EscalationInterval: minutes(6)},
			{NotificationCount: 3, NotificationInterval: minutes(8), EscalationInterval: minutes(6)},
			{NotificationCount: 3, NotificationInterval: minutes(8), EscalationInterval: minutes(6)},
			{NotificationCount: 3, NotificationInterval: minutes(8), EscalationInterval: minutes(6)},
		},
	},
}

// priorityAliases maps the priority/severity labels the platform uses onto the
// P-notation the timing table is keyed by, so a caller may pass either.
//
// Two vocabularies feed this, and both must resolve:
//
//   - Case severity: CATASTROPHIC, CRITICAL, HIGH, MEDIUM, LOW.
//   - Incident priority (entity-service's domain.IncidentPriority, which is
//     ServiceNow's own priority enum): CRITICAL, HIGH, MODERATE, LOW,
//     PLANNING.
//
// They overlap everywhere except MODERATE, which is the incident spelling of
// MEDIUM. Section 10.0's worked example ("Priority - Critical (P1)") is what
// pins CRITICAL to P1 rather than to P0.
//
// PLANNING is deliberately absent: section 7.0 has no row below P4, so a
// planning-priority incident has no ladder at all. Lookup reports that as
// "not found" and the engine skips it — see Engine.start.
var priorityAliases = map[string]string{
	"CATASTROPHIC": "P0",
	"CRITICAL":     "P1",
	"HIGH":         "P2",
	"MEDIUM":       "P3",
	"MODERATE":     "P3",
	"LOW":          "P4",
}

// Lookup resolves a priority string to its policy, accepting both P-notation
// and the severity labels used elsewhere in the platform.
func Lookup(policies map[string]PriorityPolicy, priority string) (PriorityPolicy, bool) {
	if p, ok := policies[priority]; ok {
		return p, true
	}
	if alias, ok := priorityAliases[priority]; ok {
		p, found := policies[alias]
		return p, found
	}
	return PriorityPolicy{}, false
}

// Schedule expands a priority's policy into every individual call attempt,
// ordered by when it is due.
//
// includeLevel0 reflects section 3.0: the notification level applies only to
// incidents reported during a rotation. When false, the ladder still waits
// InitialWait, then starts at Level1 — Level0's own duration is skipped
// entirely rather than left as dead time.
func Schedule(p PriorityPolicy, includeLevel0 bool) []Attempt {
	attempts := make([]Attempt, 0, 16)
	elapsed := p.InitialWait

	for level := Level0; level <= Level4; level++ {
		lp := p.Levels[level]
		if level == Level0 && !includeLevel0 {
			continue
		}
		for i := 1; i <= lp.NotificationCount; i++ {
			// The first call of a level happens as the level opens; each
			// subsequent one is NotificationInterval later.
			attempts = append(attempts, Attempt{
				Level:   level,
				Ordinal: i,
				After:   elapsed + time.Duration(i-1)*lp.NotificationInterval,
			})
		}
		elapsed += lp.Duration()
	}

	sort.SliceStable(attempts, func(i, j int) bool { return attempts[i].After < attempts[j].After })
	return attempts
}

// TimeToFinalLevel is how long after the trigger LEVEL_4 opens — the document's
// "Time to Reach Final Escalation Level", i.e. the initial wait plus levels 0
// through 3, excluding LEVEL_4's own duration.
func TimeToFinalLevel(p PriorityPolicy, includeLevel0 bool) time.Duration {
	total := p.InitialWait
	for level := Level0; level <= Level3; level++ {
		if level == Level0 && !includeLevel0 {
			continue
		}
		total += p.Levels[level].Duration()
	}
	return total
}
