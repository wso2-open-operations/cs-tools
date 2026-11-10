// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package muterule

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"alert-core-service/internal/csm"
)

func cond(field, value string) csm.AlertMuteRuleCondition {
	return csm.AlertMuteRuleCondition{Field: field, Value: value}
}

func rule(conds ...csm.AlertMuteRuleCondition) Rule {
	return Rule{ID: "b1", Number: "AMR0000001", Conditions: conds}
}

func TestMatches(t *testing.T) {
	alert := Fields{Service: "client-ballerinacentral-alert-integration", Environment: "Production", Source: "Site24x7",
		MetricName: "cpu", Severity: "Critical", Category: "performance", UniqueIdentifier: "host-1"}
	svc := cond("service", "client-ballerinacentral-alert-integration")
	tests := []struct {
		name string
		rule Rule
		f    Fields
		want bool
	}{
		{"service and environment group", rule(svc, cond("environment", "prod")), alert, true},
		{"environment alias prd", rule(svc, cond("environment", "PRD")), alert, true},
		{"environment alias live", rule(svc, cond("environment", " live ")), alert, true},
		{"environment other group", rule(svc, cond("environment", "staging")), alert, false},
		{"environment stg vs stage", rule(svc, cond("environment", "stg")), Fields{Service: svc.Value, Environment: "Stage"}, true},
		{"environment qa vs testing", rule(svc, cond("environment", "qa")), Fields{Service: svc.Value, Environment: "testing"}, true},
		{"environment perf vs stress", rule(svc, cond("environment", "perf")), Fields{Service: svc.Value, Environment: "STRESS"}, true},
		{"environment dev vs develop", rule(svc, cond("environment", "dev")), Fields{Service: svc.Value, Environment: "develop"}, true},
		{"environment uat", rule(svc, cond("environment", "UAT")), Fields{Service: svc.Value, Environment: "uat"}, true},
		{"environment outside groups compares as itself", rule(svc, cond("environment", " Sandbox ")), Fields{Service: svc.Value, Environment: "sandbox"}, true},
		{"environment outside groups differs", rule(svc, cond("environment", "sandbox")), Fields{Service: svc.Value, Environment: "sandbox2"}, false},
		{"environment empty vs value", rule(svc, cond("environment", "prod")), Fields{Service: svc.Value}, false},
		{"service case-insensitive trimmed", rule(cond("service", "  CLIENT-BallerinaCentral-Alert-Integration "), cond("environment", "prod")), alert, true},
		{"service differs", rule(cond("service", "other"), cond("environment", "prod")), alert, false},
		{"service is not a substring match", rule(cond("service", "client-ballerinacentral"), cond("environment", "prod")), alert, false},
		{"source", rule(svc, cond("environment", "prod"), cond("source", "site24x7")), alert, true},
		{"source differs", rule(svc, cond("environment", "prod"), cond("source", "aws")), alert, false},
		{"metric_name", rule(svc, cond("environment", "prod"), cond("metric_name", "CPU")), alert, true},
		{"severity", rule(svc, cond("environment", "prod"), cond("severity", "critical")), alert, true},
		{"severity differs", rule(svc, cond("environment", "prod"), cond("severity", "warning")), alert, false},
		{"category", rule(svc, cond("environment", "prod"), cond("category", "Performance")), alert, true},
		{"unique_identifier", rule(svc, cond("environment", "prod"), cond("unique_identifier", "HOST-1")), alert, true},
		{"unique_identifier differs", rule(svc, cond("environment", "prod"), cond("unique_identifier", "host-2")), alert, false},
		{"all must match, one misses", rule(svc, cond("environment", "prod"), cond("source", "Site24x7"), cond("metric_name", "mem")), alert, false},
		{"unknown field matches nothing", rule(svc, cond("environment", "prod"), cond("region", "us")), alert, false},
		{"no conditions matches nothing", rule(), alert, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Matches(tt.rule, tt.f); got != tt.want {
				t.Fatalf("Matches = %v, want %v", got, tt.want)
			}
		})
	}
}

func testCache(fetch Fetch, now *time.Time) *Cache {
	c := NewCache(slog.New(slog.NewTextHandler(io.Discard, nil)), fetch, 5*time.Minute)
	c.now = func() time.Time { return *now }
	return c
}

func TestCache_MatchesActiveBlocksAndFailsOpenWhenStale(t *testing.T) {
	now := time.Date(2026, 11, 6, 10, 30, 0, 0, time.UTC)
	b := Rule{ID: "b1", Number: "AMR0000001", StartOn: now.Add(-time.Hour), EndOn: now.Add(time.Hour),
		Conditions: []csm.AlertMuteRuleCondition{cond("service", "svc"), cond("environment", "prod")}}
	var fetchErr error
	c := testCache(func(context.Context) ([]Rule, error) { return []Rule{b}, fetchErr }, &now)
	f := Fields{Service: "svc", Environment: "production"}

	if _, ok := c.Match(f); ok {
		t.Fatal("an unfilled cache must match nothing")
	}
	if err := c.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got, ok := c.Match(f); !ok || got.ID != "b1" {
		t.Fatalf("Match = %+v, %v, want b1", got, ok)
	}
	if _, ok := c.Match(Fields{Service: "svc", Environment: "dev"}); ok {
		t.Fatal("a non-matching alert must not match")
	}

	fetchErr = errors.New("csm down")
	now = now.Add(4 * time.Minute)
	if err := c.Refresh(context.Background()); err == nil {
		t.Fatal("want the refresh error")
	}
	if _, ok := c.Match(f); !ok {
		t.Fatal("a failed refresh must keep the last good rules within max staleness")
	}
	now = now.Add(2 * time.Minute)
	if _, ok := c.Match(f); ok {
		t.Fatal("rules older than max staleness must match nothing (fail open)")
	}
}

func TestCache_BlockStopsMatchingAtItsEnd(t *testing.T) {
	now := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)
	b := Rule{ID: "b1", StartOn: now.Add(time.Minute), EndOn: now.Add(2 * time.Minute),
		Conditions: []csm.AlertMuteRuleCondition{cond("service", "svc"), cond("environment", "prod")}}
	c := testCache(func(context.Context) ([]Rule, error) { return []Rule{b}, nil }, &now)
	_ = c.Refresh(context.Background())
	f := Fields{Service: "svc", Environment: "prod"}
	for _, tc := range []struct {
		at   time.Duration
		want bool
	}{{0, false}, {time.Minute, true}, {2*time.Minute - time.Second, true}, {2 * time.Minute, false}} {
		now = time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC).Add(tc.at)
		if _, ok := c.Match(f); ok != tc.want {
			t.Fatalf("at +%v: Match = %v, want %v", tc.at, ok, tc.want)
		}
	}
}

func TestCache_NilMatchesNothing(t *testing.T) {
	var c *Cache
	if _, ok := c.Match(Fields{Service: "svc"}); ok {
		t.Fatal("a nil cache must match nothing")
	}
}
