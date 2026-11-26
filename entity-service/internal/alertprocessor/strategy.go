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

package alertprocessor

import (
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// Outcome is what happens to one alert.
type Outcome string

const (
	// OutcomeOpen opens a new incident for the alert.
	OutcomeOpen Outcome = "open"
	// OutcomeDuplicate adds the alert to its fingerprint's open incident as a Duplicate note.
	OutcomeDuplicate Outcome = "duplicate"
	// OutcomeResolve adds a resolving alert to its fingerprint's open incident as an OK note.
	OutcomeResolve Outcome = "resolve"
	// OutcomeMute drops an alert an active mute rule covers.
	OutcomeMute Outcome = "mute"
	// OutcomeIgnore drops an alert that needs nothing, such as a resolving alert with no open incident.
	OutcomeIgnore Outcome = "ignore"
)

// Group is the fingerprint's state: the incident its alerts currently fold into.
type Group struct {
	Fingerprint string
	IncidentID  string
	FirstSeenOn time.Time
	LastSeenOn  time.Time
	AlertCount  int
}

// Input is everything a strategy may look at; strategies do no I/O.
type Input struct {
	Event Event
	// Group is nil for a fingerprint never seen before.
	Group *Group
	// IncidentOpen reports whether Group's incident is still open.
	IncidentOpen bool
	MuteRules    []domain.AlertMuteRule
}

// Decision is a strategy's verdict on one alert.
type Decision struct {
	Outcome Outcome
	// Strategy names the strategy that decided, for logs.
	Strategy string
	// Reason says why, for logs.
	Reason string
}

// Strategy decides an alert's outcome or passes; the first strategy in a chain to decide wins. A new way of grouping, such as RIA, is one more Strategy.
type Strategy interface {
	Name() string
	Decide(in Input) (Decision, bool)
}

// Decide runs the chain in order and opens an incident when no strategy decides.
func Decide(chain []Strategy, in Input) Decision {
	for _, s := range chain {
		if d, ok := s.Decide(in); ok {
			d.Strategy = s.Name()
			return d
		}
	}
	return Decision{Outcome: OutcomeOpen, Strategy: "default", Reason: "no strategy decided"}
}

// CoreChain is alert-core's behaviour: resolve, then fold duplicates into the open incident, then mute, then open.
func CoreChain(dedupWindow time.Duration) []Strategy {
	return []Strategy{Resolve{}, Dedup{Window: dedupWindow}, Mute{}}
}

// Resolve turns a resolving alert into an OK note on its open incident, and drops it when there is none.
type Resolve struct{}

func (Resolve) Name() string { return "resolve" }

func (Resolve) Decide(in Input) (Decision, bool) {
	if !isResolving(in.Event.Alert.Severity) {
		return Decision{}, false
	}
	if in.Group == nil || !in.IncidentOpen {
		return Decision{Outcome: OutcomeIgnore, Reason: "resolving alert with no open incident"}, true
	}
	return Decision{Outcome: OutcomeResolve, Reason: "resolving alert"}, true
}

// Dedup folds an alert into its fingerprint's open incident while the incident is younger than Window.
type Dedup struct {
	Window time.Duration
}

func (Dedup) Name() string { return "dedup" }

func (d Dedup) Decide(in Input) (Decision, bool) {
	if in.Group == nil || !in.IncidentOpen {
		return Decision{}, false
	}
	if in.Event.ReceivedOn.Sub(in.Group.FirstSeenOn) >= d.Window {
		return Decision{}, false
	}
	return Decision{Outcome: OutcomeDuplicate, Reason: "same fingerprint inside the dedup window"}, true
}

// Mute drops an alert that would open an incident while an active mute rule covers it.
type Mute struct{}

func (Mute) Name() string { return "mute" }

func (Mute) Decide(in Input) (Decision, bool) {
	for _, rule := range in.MuteRules {
		if muteRuleMatches(rule, in.Event) {
			return Decision{Outcome: OutcomeMute, Reason: "mute rule " + rule.Number}, true
		}
	}
	return Decision{}, false
}

// muteRuleMatches requires every condition to match: environment by its group, every other field case-insensitively. A rule with no conditions, or an unknown field, matches nothing.
func muteRuleMatches(rule domain.AlertMuteRule, e Event) bool {
	if len(rule.Conditions) == 0 {
		return false
	}
	for _, c := range rule.Conditions {
		got, known := alertField(e, c.Field)
		if !known {
			return false
		}
		if normalize(c.Field) == "environment" {
			got, c.Value = environmentGroup(got), environmentGroup(c.Value)
		}
		if normalize(got) != normalize(c.Value) {
			return false
		}
	}
	return true
}

func alertField(e Event, field string) (string, bool) {
	a := e.Alert
	switch normalize(field) {
	case "service":
		return a.Service, true
	case "environment":
		return a.Environment, true
	case "source":
		return a.Source, true
	case "metric_name":
		return a.MetricName, true
	case "severity":
		return a.Severity, true
	case "category":
		return a.Category, true
	case "unique_identifier":
		return a.UniqueIdentifier, true
	}
	return "", false
}

// environmentGroups maps each spelling of an environment to its group, so a rule on "prod" also mutes "production".
var environmentGroups = func() map[string]string {
	groups := map[string][]string{
		"production":  {"prod", "production", "prd", "live"},
		"staging":     {"stg", "stage", "staging"},
		"development": {"dev", "develop", "development"},
		"qa":          {"qa", "test", "testing"},
		"uat":         {"uat"},
		"stress":      {"stress", "perf", "performance"},
	}
	m := map[string]string{}
	for group, names := range groups {
		for _, n := range names {
			m[n] = group
		}
	}
	return m
}()

func environmentGroup(v string) string {
	if g, ok := environmentGroups[normalize(v)]; ok {
		return g
	}
	return normalize(v)
}

func normalize(v string) string { return strings.ToLower(strings.TrimSpace(v)) }
