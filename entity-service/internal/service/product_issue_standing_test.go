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

package service

import (
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

func tm(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func tmp(s string) *time.Time { t := tm(s); return &t }

func TestProductIssueStanding(t *testing.T) {
	const h = 3600.0
	p1 := "P1"
	// 24x7: a 24h budget starting Mon 2026-11-16 00:00Z.
	c247 := func(stage string, mod func(*domain.ProductIssueClock)) *domain.ProductIssueClock {
		c := &domain.ProductIssueClock{PolicyName: "Product Issue P1", Schedule: "CS 24x7", Stage: stage,
			StartOn: tm("2026-11-16T00:00:00Z"), BudgetSeconds: 24 * h}
		if mod != nil {
			mod(c)
		}
		return c
	}
	// 12x5 IST: Mon 09:00 IST = 03:30Z; 24h budget.
	c125 := func(stage string, mod func(*domain.ProductIssueClock)) *domain.ProductIssueClock {
		c := &domain.ProductIssueClock{PolicyName: "Product Issue P2", Schedule: "CS 12x5 IST", Stage: stage,
			StartOn: tm("2026-11-16T03:30:00Z"), BudgetSeconds: 24 * h}
		if mod != nil {
			mod(c)
		}
		return c
	}
	cases := []struct {
		name                string
		clock               *domain.ProductIssueClock
		now                 string
		state               string
		consumed, remaining int64
		pct                 float64
		running             bool
		pausedSince         string
	}{
		{"no clock", nil, "2026-11-16T10:00:00Z", "NONE", 0, 0, 0, false, ""},
		{"cancelled is none", c247("CANCELLED", nil), "2026-11-16T10:00:00Z", "NONE", 0, 0, 0, false, ""},
		{"24x7 ok", c247("IN_PROGRESS", nil), "2026-11-16T06:00:00Z", "OK", 6 * 3600, 18 * 3600, 25, true, ""},
		{"24x7 just below 75%", c247("IN_PROGRESS", nil), "2026-11-16T17:59:59Z", "OK", 17*3600 + 59*60 + 59, 6*3600 + 1, 75, true, ""},
		{"24x7 at 75% boundary", c247("IN_PROGRESS", nil), "2026-11-16T18:00:00Z", "AT_RISK", 18 * 3600, 6 * 3600, 75, true, ""},
		{"24x7 pause time excluded", c247("IN_PROGRESS", func(c *domain.ProductIssueClock) { c.BusinessPauseSeconds = 10 * h }),
			"2026-11-16T20:00:00Z", "OK", 10 * 3600, 14 * 3600, 41.67, true, ""},
		{"24x7 running past budget, stale flag", c247("IN_PROGRESS", nil), "2026-11-17T02:00:00Z", "BREACHED", 26 * 3600, 0, 108.33, true, ""},
		{"24x7 running breached flag", c247("IN_PROGRESS", func(c *domain.ProductIssueClock) { c.HasBreached = true }),
			"2026-11-16T01:00:00Z", "BREACHED", 3600, 23 * 3600, 4.17, true, ""},
		{"24x7 paused", c247("PAUSED", func(c *domain.ProductIssueClock) { c.PausedOn = tmp("2026-11-16T10:00:00Z") }),
			"2026-11-18T10:00:00Z", "PAUSED", 10 * 3600, 14 * 3600, 41.67, false, "2026-11-16T10:00:00Z"},
		{"24x7 paused at 90% is still paused", c247("PAUSED", func(c *domain.ProductIssueClock) { c.PausedOn = tmp("2026-11-16T22:00:00Z") }),
			"2026-11-20T10:00:00Z", "PAUSED", 22 * 3600, 2 * 3600, 91.67, false, "2026-11-16T22:00:00Z"},
		{"24x7 paused breached", c247("PAUSED", func(c *domain.ProductIssueClock) {
			c.PausedOn = tmp("2026-11-17T01:00:00Z")
			c.HasBreached = true
		}), "2026-11-18T10:00:00Z", "BREACHED", 25 * 3600, 0, 104.17, false, "2026-11-17T01:00:00Z"},
		{"24x7 met", c247("ACHIEVED", func(c *domain.ProductIssueClock) { c.EndOn = tmp("2026-11-16T12:00:00Z") }),
			"2026-11-30T00:00:00Z", "MET", 12 * 3600, 12 * 3600, 50, false, ""},
		{"24x7 finished breached", c247("ACHIEVED", func(c *domain.ProductIssueClock) {
			c.EndOn = tmp("2026-11-17T06:00:00Z")
			c.HasBreached = true
		}), "2026-11-30T00:00:00Z", "BREACHED", 30 * 3600, 0, 125, false, ""},
		{"12x5 ok", c125("IN_PROGRESS", nil), "2026-11-16T09:30:00Z", "OK", 6 * 3600, 18 * 3600, 25, true, ""},
		{"12x5 night is not counted", c125("IN_PROGRESS", nil), "2026-11-17T03:30:00Z", "OK", 12 * 3600, 12 * 3600, 50, true, ""},
		{"12x5 weekend is not counted", c125("IN_PROGRESS", nil), "2026-11-21T16:00:00Z", "AT_RISK", 60 * 3600, 0, 250, true, ""}, // replaced below
		{"12x5 paused", c125("PAUSED", func(c *domain.ProductIssueClock) {
			c.PausedOn = tmp("2026-11-16T09:30:00Z")
			c.BusinessPauseSeconds = 1 * h
		}), "2026-11-19T00:00:00Z", "PAUSED", 5 * 3600, 19 * 3600, 20.83, false, "2026-11-16T09:30:00Z"},
		{"12x5 met", c125("ACHIEVED", func(c *domain.ProductIssueClock) { c.EndOn = tmp("2026-11-16T15:30:00Z") }),
			"2026-11-30T00:00:00Z", "MET", 12 * 3600, 12 * 3600, 50, false, ""},
	}
	// Friday 2026-11-20 09:00-21:00 IST holds the 5th business day; start Fri 03:30Z and
	// run to Monday: only 12h of Friday counts.
	cases[15] = struct {
		name                string
		clock               *domain.ProductIssueClock
		now                 string
		state               string
		consumed, remaining int64
		pct                 float64
		running             bool
		pausedSince         string
	}{"12x5 weekend is not counted", c125("IN_PROGRESS", func(c *domain.ProductIssueClock) { c.StartOn = tm("2026-11-20T03:30:00Z") }),
		"2026-11-23T03:30:00Z", "OK", 12 * 3600, 12 * 3600, 50, true, ""}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := computeProductIssueSLA(tc.clock, &p1, tm(tc.now))
			if got.State != tc.state || got.ConsumedSeconds != tc.consumed || got.RemainingSeconds != tc.remaining ||
				got.Percentage != tc.pct || got.Running != tc.running {
				t.Fatalf("got %+v", got)
			}
			if (got.PausedSince == nil) != (tc.pausedSince == "") || (got.PausedSince != nil && !got.PausedSince.Equal(tm(tc.pausedSince))) {
				t.Errorf("pausedSince = %v, want %q", got.PausedSince, tc.pausedSince)
			}
			if got.Priority == nil || *got.Priority != "P1" {
				t.Errorf("priority = %v", got.Priority)
			}
			if tc.clock != nil && tc.state != "NONE" && (got.PolicyName == nil || got.Schedule == nil || got.BudgetSeconds != 24*3600) {
				t.Errorf("policy fields = %+v", got)
			}
		})
	}
}

func TestApplyStandingClearsClock(t *testing.T) {
	items := []domain.ProductIssueSummary{{ID: "a"}, {ID: "b", Clock: &domain.ProductIssueClock{Schedule: "CS 24x7", Stage: "IN_PROGRESS",
		StartOn: tm("2026-11-16T00:00:00Z"), BudgetSeconds: 3600}}}
	applyStanding(items, tm("2026-11-16T00:30:00Z"))
	if items[0].SLA.State != "NONE" || items[1].SLA.State != "OK" || items[1].Clock != nil || items[1].SLA.Percentage != 50 {
		t.Fatalf("%+v", items)
	}
}
