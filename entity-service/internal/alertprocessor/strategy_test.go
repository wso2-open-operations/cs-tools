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
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/model"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

var firstSeen = time.Date(2026, 11, 26, 10, 0, 0, 0, time.UTC)

func event(severity string, after time.Duration) Event {
	return Event{
		AlertID:    "ALT000000001",
		Tier:       domain.AlertTierPriority,
		ReceivedOn: firstSeen.Add(after),
		Alert: model.Alert{
			Service: "client-prod-alert-integration", MetricName: "TEST", Severity: severity,
			Category: "service_interruption", Environment: "prod", Source: "elasticsearch", UniqueIdentifier: "r2",
		},
	}
}

func openGroup() *Group {
	return &Group{IncidentID: "inc-1", FirstSeenOn: firstSeen, LastSeenOn: firstSeen, AlertCount: 1}
}

func rule(field, value string) domain.AlertMuteRule {
	return domain.AlertMuteRule{Number: "AMR0000001", Conditions: []domain.AlertMuteRuleCondition{{Field: field, Value: value}}}
}

func TestCoreChain(t *testing.T) {
	chain := CoreChain(5 * time.Minute)
	tests := map[string]struct {
		in   Input
		want Outcome
	}{
		"new fingerprint opens":              {Input{Event: event("critical", 0)}, OutcomeOpen},
		"same fingerprint inside the window": {Input{Event: event("critical", time.Minute), Group: openGroup(), IncidentOpen: true}, OutcomeDuplicate},
		"window passed opens a new incident": {Input{Event: event("critical", 5*time.Minute), Group: openGroup(), IncidentOpen: true}, OutcomeOpen},
		"closed incident opens a new one":    {Input{Event: event("critical", time.Minute), Group: openGroup(), IncidentOpen: false}, OutcomeOpen},
		"resolving alert on open incident":   {Input{Event: event("OK", time.Hour), Group: openGroup(), IncidentOpen: true}, OutcomeResolve},
		"resolving alert with no incident":   {Input{Event: event("clear", 0)}, OutcomeIgnore},
		"mute rule stops a new incident":     {Input{Event: event("critical", 0), MuteRules: []domain.AlertMuteRule{rule("environment", "production")}}, OutcomeMute},
		"mute rule never hides a duplicate": {Input{Event: event("critical", time.Minute), Group: openGroup(), IncidentOpen: true,
			MuteRules: []domain.AlertMuteRule{rule("service", "client-prod-alert-integration")}}, OutcomeDuplicate},
		"non-matching mute rule": {Input{Event: event("critical", 0), MuteRules: []domain.AlertMuteRule{rule("service", "other")}}, OutcomeOpen},
	}
	for name, tt := range tests {
		if got := Decide(chain, tt.in).Outcome; got != tt.want {
			t.Errorf("%s: outcome = %s, want %s", name, got, tt.want)
		}
	}
}

func TestMuteRuleMatches(t *testing.T) {
	e := event("Warning", 0)
	tests := map[string]struct {
		rule domain.AlertMuteRule
		want bool
	}{
		"case-insensitive":          {rule("SEVERITY", " warning "), true},
		"environment by group":      {rule("environment", "live"), true},
		"unknown field never mutes": {rule("region", "eu"), false},
		"no conditions never mutes": {domain.AlertMuteRule{}, false},
		"every condition must match": {domain.AlertMuteRule{Conditions: []domain.AlertMuteRuleCondition{
			{Field: "source", Value: "elasticsearch"}, {Field: "service", Value: "other"}}}, false},
	}
	for name, tt := range tests {
		if got := muteRuleMatches(tt.rule, e); got != tt.want {
			t.Errorf("%s: matches = %v, want %v", name, got, tt.want)
		}
	}
}

// A strategy placed first wins, which is how a new way of grouping joins the chain.
type always struct{ outcome Outcome }

func (always) Name() string                    { return "always" }
func (a always) Decide(Input) (Decision, bool) { return Decision{Outcome: a.outcome}, true }

func TestDecideFirstStrategyWins(t *testing.T) {
	chain := append([]Strategy{always{OutcomeMute}}, CoreChain(time.Minute)...)
	d := Decide(chain, Input{Event: event("critical", 0)})
	if d.Outcome != OutcomeMute || d.Strategy != "always" {
		t.Fatalf("decision = %+v, want the first strategy's mute", d)
	}
}

func TestImpactUrgency(t *testing.T) {
	tests := map[string][2]string{
		"critical": {"HIGH", "HIGH"}, "major": {"MEDIUM", "HIGH"}, "minor": {"MEDIUM", "MEDIUM"},
		"warning": {"MEDIUM", "LOW"}, "typo": {"HIGH", "HIGH"},
	}
	for label, want := range tests {
		impact, urgency := impactUrgency(label)
		if string(impact) != want[0] || string(urgency) != want[1] {
			t.Errorf("%s: %s/%s, want %s/%s", label, impact, urgency, want[0], want[1])
		}
	}
}
