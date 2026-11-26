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

package strategy

import (
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertprocessor/event"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// MuteStrategy drops an alert that would open an incident while an active mute rule covers it.
type MuteStrategy struct{}

// Name implements Strategy.
func (MuteStrategy) Name() string { return "mute" }

// Decide implements Strategy.
func (MuteStrategy) Decide(in Input) (Decision, bool) {
	for _, rule := range in.MuteRules {
		if RuleMatches(rule, in.Event) {
			return Decision{Outcome: Mute, Reason: "mute rule " + rule.Number}, true
		}
	}
	return Decision{}, false
}

// RuleMatches requires every condition to match: environment by its group, every other field case-insensitively. A rule with no conditions, or an unknown field, matches nothing.
func RuleMatches(rule domain.AlertMuteRule, e event.Event) bool {
	if len(rule.Conditions) == 0 {
		return false
	}
	for _, c := range rule.Conditions {
		got, known := alertField(e, c.Field)
		if !known {
			return false
		}
		want := c.Value
		if normalize(c.Field) == "environment" {
			got, want = environmentGroup(got), environmentGroup(want)
		}
		if normalize(got) != normalize(want) {
			return false
		}
	}
	return true
}

func alertField(e event.Event, field string) (string, bool) {
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
