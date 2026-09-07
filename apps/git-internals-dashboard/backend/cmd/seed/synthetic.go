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

// Deterministic fixtures used ONLY when GITHUB_TOKEN is empty (port of v3's
// seed/synthetic.ts). Emits the same (node, detail) pairs as internal/github
// so main.go has one ingestion path regardless of data source.
//
// PRIVACY: fixtures contain no titles, assignees, or actors — matching what
// the real GitHub path returns. At runtime the title proxy returns null for
// these issue numbers, so the UI shows "#<number>" only.
package main

import (
	"fmt"
	"time"

	"github.com/binara-sachin/git-internals-dashboard/backend/internal/config"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/github"
	"github.com/binara-sachin/git-internals-dashboard/backend/internal/ingest"
)

// transition is one board-status change, oldest first; hoursAgo is when the
// item entered this status (oldest = largest).
type transition struct {
	status   string
	hoursAgo float64
}

// scenario describes one synthetic issue's shape. currentStatus/
// currentStatusAgo override the empty-timeline case: when transitions is
// empty there's no last-transition status to derive currentStatus from, so
// scenarios that need a non-null current status set it explicitly.
type scenario struct {
	priority         *string // nil => no Priority/* label => NO_SLA
	state            string  // "OPEN" | "CLOSED"
	transitions      []transition
	currentStatus    *string
	currentStatusAgo *float64
}

// Expected outcomes (evaluated at "now"):
//  1. VIOLATED  2. AT_RISK/OK*  3. OK*  4. NO_SLA  5. TERMINAL  6. NO_SLA
//  7. VIOLATED*  8. TERMINAL  9. VIOLATED*
//
// * P2/P3 now accrue on a 12x5 IST business calendar, so the exact state of
// these scenarios depends on which day/time the seed runs (nights & weekends
// don't burn budget). P1 stays 24x7. P4 has no SLA (kept indefinitely).
var scenarios = []scenario{
	{ // P1, ~120h product-side => VIOLATED, still running
		priority: strp("Critical(P1)"), state: "OPEN",
		transitions: []transition{{"Open", 120}, {"In Progress", 100}, {"WOW", 50}},
	},
	{ // P2, ~20h accrued then paused on WOC => AT_RISK
		priority: strp("High(P2)"), state: "OPEN",
		transitions: []transition{{"Open", 25}, {"In Progress", 20}, {"WOC", 5}},
	},
	{ // P3, business-hours accrual of 48h budget => OK, paused
		priority: strp("Medium(P3)"), state: "OPEN",
		transitions: []transition{{"Open", 60}, {"In Progress", 40}, {"WOC", 10}},
	},
	{ // P4 has no SLA => NO_SLA (kept indefinitely)
		priority: strp("Low(P4)"), state: "OPEN",
		transitions: []transition{{"Open", 20}, {"In Progress", 3}},
	},
	{ // P1 currently Resolved => TERMINAL, excluded from alerts
		priority: strp("Critical(P1)"), state: "OPEN",
		transitions: []transition{{"Open", 200}, {"In Progress", 150}, {"Resolved", 2}},
	},
	{ // No priority label => NO_SLA, never alerted
		priority: nil, state: "OPEN",
		transitions: []transition{{"Open", 48}, {"In Progress", 10}},
	},
	{ // P2 pause+resume; 4h + 24h = ~28h => VIOLATED, running
		priority: strp("High(P2)"), state: "OPEN",
		transitions: []transition{{"Open", 40}, {"WOC", 36}, {"In Progress", 24}},
	},
	{ // P3 closed + Resolved => TERMINAL
		priority: strp("Medium(P3)"), state: "CLOSED",
		transitions: []transition{{"Open", 300}, {"In Progress", 250}, {"Resolved", 240}},
	},
	{ // P2, empty timeline, current status set ~30h ago (empty-timeline path) => VIOLATED, running
		priority: strp("High(P2)"), state: "OPEN",
		transitions: nil, currentStatus: strp("Open"), currentStatusAgo: f64p(30),
	},
}

func strp(s string) *string   { return &s }
func f64p(f float64) *float64 { return &f }

// hoursAgo formats an instant hoursAgo hours before now, matching
// JavaScript's Date.prototype.toISOString() format exactly (millisecond
// precision, always UTC "Z").
func hoursAgo(now time.Time, hours float64) string {
	return now.Add(-time.Duration(hours * float64(time.Hour))).UTC().Format("2006-01-02T15:04:05.000Z")
}

func buildEvents(now time.Time, transitions []transition) []github.StatusEvent {
	events := make([]github.StatusEvent, len(transitions))
	for i, t := range transitions {
		var previous *string
		if i > 0 {
			previous = strp(transitions[i-1].status)
		}
		events[i] = github.StatusEvent{CreatedAt: hoursAgo(now, t.hoursAgo), PreviousStatus: previous, Status: strp(t.status)}
	}
	return events
}

// syntheticRepoIssues generates one fixture issue per scenario for repo,
// numbered so different repos in the same seed run never collide
// (repoIndex+1)*1000 + (scenario index+1).
func syntheticRepoIssues(now time.Time, repo config.RepoEntry, repoIndex int) []ingest.Pair {
	pairs := make([]ingest.Pair, len(scenarios))
	for i, s := range scenarios {
		number := (repoIndex+1)*1000 + (i + 1)
		events := buildEvents(now, s.transitions)

		createdAt := hoursAgo(now, 72)
		if s.currentStatusAgo != nil {
			createdAt = hoursAgo(now, *s.currentStatusAgo)
		}
		if len(events) > 0 {
			createdAt = events[0].CreatedAt
		}
		lastTs := createdAt
		if len(events) > 0 {
			lastTs = events[len(events)-1].CreatedAt
		}

		currentStatus := s.currentStatus
		if currentStatus == nil && len(s.transitions) > 0 {
			currentStatus = strp(s.transitions[len(s.transitions)-1].status)
		}
		currentStatusTs := lastTs
		if s.currentStatus != nil {
			currentStatusTs = hoursAgo(now, *s.currentStatusAgo)
		}

		labelNames := make([]string, 0, 2)
		if s.priority != nil {
			labelNames = append(labelNames, "Priority/"+*s.priority)
		}
		labelNames = append(labelNames, "Origin/CS")

		var closedAt *string
		if s.state == "CLOSED" {
			closedAt = strp(currentStatusTs)
		}

		node := github.IssueNode{
			Number:    number,
			State:     s.state,
			URL:       fmt.Sprintf("https://github.com/%s/%s/issues/%d", repo.Owner, repo.Name, number),
			CreatedAt: createdAt,
			UpdatedAt: currentStatusTs,
			ClosedAt:  closedAt,
			Labels:    labelNames,
		}

		itemCreatedAt := createdAt
		if len(events) > 0 {
			itemCreatedAt = events[0].CreatedAt
		}

		detail := github.IssueDetail{
			Number: number,
			Events: events,
			ProjectStatuses: []github.ProjectStatus{
				{
					ProjectID:       repo.GithubProjectID,
					Status:          currentStatus,
					StatusUpdatedAt: strp(currentStatusTs),
					ItemCreatedAt:   strp(itemCreatedAt),
				},
			},
		}

		pairs[i] = ingest.Pair{Node: node, Detail: detail}
	}
	return pairs
}
