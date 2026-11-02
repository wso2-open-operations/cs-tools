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

// Package chataudience decides which Google Chat audience(s) a case's team,
// evaluation-account status, onboarding status, and the current time
// resolve to — shared by internal/slaengine (SLA breach alerts) and, in
// the future, a customer-frustration-detector alert, per explicit product
// direction: those are the only two Chat-sending event types that route by
// team/audience. Every other Chat-sending event (case.created/
// case.acknowledged/case.severity_changed/incident.created) routes by
// product instead, to a single configured default space, and has no need
// of this package.
package chataudience

import (
	"slices"
	"strings"
	"time"
)

// Audience keys not tied to a specific CRE team name — the standing,
// always-available Chat audiences a case can resolve to on top of (or
// instead of) its own team.
const (
	IncidentMonitor = "Incident Monitor"
	Onboarding      = "Onboarding"
	Americas        = "Americas"
	Evaluation      = "Evaluation"
	// Migration is the standing audience a frustration-detection Chat alert
	// routes to instead of the usual team/Incident Monitor audience, when
	// the comment landed on a migration engagement (case type "engagement",
	// engagement type "migration") -- see dispatch.checkFrustration, which
	// decides this before ever calling Resolve, since it replaces Resolve's
	// whole team-first decision rather than adding to it.
	Migration = "Migration"
	// SLADefault is the audience an SLA breach alert (internal/slaengine)
	// substitutes for IncidentMonitor in Resolve's own return value, before
	// posting -- never returned by Resolve itself. The Incident Monitor
	// Chat space has more than one incoming webhook pointed at it (one for
	// the general incident feed, one dedicated to SLA breach cards, one for
	// something else), a single Google Chat space supports perfectly well,
	// so this does not change WHICH space an SLA alert lands in, only which
	// of that space's own webhooks posts it -- a routing identity, not a
	// destination change. Every other Resolve outcome (a real team's own
	// space, Evaluation, Onboarding, Americas) is unaffected: only the
	// literal "no team space, fall back to the shared room" case is
	// retargeted, and only for this one caller.
	SLADefault = "SLA Default"
)

// onboardingStatuses are the entity-service project.onboarding_status raw
// values that add the Onboarding Chat audience on top of whichever
// audience the case's own CreTeam resolves to. A policy decision kept
// here, not in entity-service, so it can change without a redeploy there.
var onboardingStatuses = map[string]bool{
	"IN_PROGRESS": true,
	"NOT_STARTED": true,
}

// istLocation is a fixed +5:30 offset with no DST — IST never observes one,
// so a fixed zone is exact and needs no tzdata lookup.
var istLocation = time.FixedZone("IST", 5*3600+30*60)

// The Americas overnight coverage window (9 PM–6 AM IST, wrapping past
// midnight) and the weekend boundary (6 AM IST).
const (
	americasWindowStartMinutes = 21 * 60
	americasWindowEndMinutes   = 6 * 60
	weekendBoundaryMinutes     = 6 * 60
)

// isAmericasCoverageWindow reports whether ist (already converted to IST)
// falls in the 9 PM–6 AM overnight window — an OR of two ranges, not a
// single bounded one, since the window wraps past midnight.
func isAmericasCoverageWindow(ist time.Time) bool {
	minutesOfDay := ist.Hour()*60 + ist.Minute()
	return minutesOfDay >= americasWindowStartMinutes || minutesOfDay < americasWindowEndMinutes
}

// isWeekendCoverageWindow reports whether ist falls between 6 AM IST
// Saturday and 6 AM IST Monday.
func isWeekendCoverageWindow(ist time.Time) bool {
	minutesOfDay := ist.Hour()*60 + ist.Minute()
	switch ist.Weekday() {
	case time.Saturday:
		return minutesOfDay >= weekendBoundaryMinutes
	case time.Sunday:
		return true
	case time.Monday:
		return minutesOfDay < weekendBoundaryMinutes
	default:
		return false
	}
}

// Resolve decides which Chat audience(s) an alert should post to, from the
// case's own team, evaluation-account status, project onboarding status,
// and the current time.
//
// hasAudience answers "does this team have a configured Chat space" —
// notifications.GoogleChatClient.HasAudienceSpace in production. A team
// with no space of its own falls back to the shared Incident Monitor
// audience, the same as no team being assigned at all — both mean "there's
// nowhere specific to send this," just for different reasons.
//
// now is passed in (not read via time.Now() internally) so tests can drive
// a fixed clock.
func Resolve(team string, isEvaluationAccount bool, onboardingStatus string, now time.Time, hasAudience func(string) bool) []string {
	// Evaluation Subscription cases belong to the Evaluation audience
	// alone — none of the team/onboarding/Americas/weekend rules below
	// apply.
	if isEvaluationAccount {
		return []string{Evaluation}
	}

	var audiences []string
	add := func(audience string) {
		if !slices.Contains(audiences, audience) {
			audiences = append(audiences, audience)
		}
	}

	if team != "" && hasAudience(team) {
		add(team)
	} else {
		add(IncidentMonitor)
	}

	if onboardingStatuses[strings.ToUpper(strings.TrimSpace(onboardingStatus))] {
		add(Onboarding)
	}

	ist := now.In(istLocation)
	if isAmericasCoverageWindow(ist) {
		add(Americas)
	}
	if isWeekendCoverageWindow(ist) {
		add(IncidentMonitor)
	}

	return audiences
}
