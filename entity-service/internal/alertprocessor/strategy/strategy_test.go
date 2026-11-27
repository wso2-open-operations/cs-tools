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
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertprocessor/event"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/alertsource/model"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

var firstSeen = time.Date(2026, 11, 26, 10, 0, 0, 0, time.UTC)

func alert(severity string, after time.Duration) event.Event {
	return event.Event{
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

func TestDefault(t *testing.T) {
	chain := Default(5 * time.Minute)
	tests := map[string]struct {
		in   Input
		want Outcome
	}{
		"new fingerprint opens":                             {Input{Event: alert("critical", 0)}, Open},
		"same fingerprint inside the window":                {Input{Event: alert("critical", time.Minute), Group: openGroup(), IncidentOpen: true}, Duplicate},
		"window passed opens a new incident":                {Input{Event: alert("critical", 5*time.Minute), Group: openGroup(), IncidentOpen: true}, Open},
		"closed incident opens a new one":                   {Input{Event: alert("critical", time.Minute), Group: openGroup(), IncidentOpen: false}, Open},
		"late alert before a closed incident's last alert":  {Input{Event: alert("critical", -time.Minute), Group: openGroup(), IncidentOpen: false}, Ignore},
		"alert well before the open incident's first alert": {Input{Event: alert("critical", -10*time.Minute), Group: openGroup(), IncidentOpen: true}, Open},
		"alert just before the open incident's first alert": {Input{Event: alert("critical", -time.Minute), Group: openGroup(), IncidentOpen: true}, Duplicate},
		"resolving alert on open incident":                  {Input{Event: alert("OK", time.Hour), Group: openGroup(), IncidentOpen: true}, Resolve},
		"resolving alert with no incident":                  {Input{Event: alert("clear", 0)}, Ignore},
	}
	for name, tt := range tests {
		if got := chain.Decide(tt.in).Outcome; got != tt.want {
			t.Errorf("%s: outcome = %s, want %s", name, got, tt.want)
		}
	}
}

// always decides every alert, standing in for a plugged-in strategy such as RIA.
type always struct{ outcome Outcome }

func (always) Name() string                    { return "always" }
func (a always) Decide(Input) (Decision, bool) { return Decision{Outcome: a.outcome}, true }

func TestChainBeforeAndAfter(t *testing.T) {
	in := Input{Event: alert("critical", 0)}
	core := Default(time.Minute)
	if d := core.Before(always{Ignore}).Decide(in); d.Outcome != Ignore || d.Strategy != "always" {
		t.Fatalf("Before: decision = %+v, want the plugged-in strategy first", d)
	}
	if d := core.After(always{Ignore}).Decide(in); d.Outcome != Ignore || d.Strategy != "always" {
		t.Fatalf("After: decision = %+v, want the plugged-in strategy once core passed", d)
	}
	if d := core.Decide(in); d.Outcome != Open || len(core) != 3 {
		t.Fatalf("core changed by Before/After: %+v, %d strategies", d, len(core))
	}
}
