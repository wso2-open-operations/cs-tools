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
	"math"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/slacalendar"
)

// productIssueAtRiskPercentage is the share of the budget at or above which a running,
// unbreached product issue clock is AT_RISK (owner decision P13).
const productIssueAtRiskPercentage = 75.0

// computeProductIssueSLA is a product issue's SLA standing at now, computed from its
// newest resolution clock row (nil: no clock). It never reads the sla_live view.
//
// consumed = calendar.Elapsed(start_on, X) - business_pause_duration, where X is end_on
// for a finished clock, paused_on for a paused one and now for a running one (a paused
// row's business_pause_duration excludes its open segment). A CANCELLED clock counts as
// no clock; COMPLETED is a finished one. A running or paused clock whose consumed time
// reached the budget is BREACHED even when the stored has_breached flag is stale.
func computeProductIssueSLA(c *domain.ProductIssueClock, priority *string, now time.Time) domain.ProductIssueSLA {
	if c == nil || c.Stage == "CANCELLED" {
		return domain.ProductIssueSLA{State: domain.ProductIssueSLANone, Priority: priority}
	}
	finished := c.Stage == "ACHIEVED" || c.Stage == "COMPLETED"
	paused := c.Stage == "PAUSED"
	x := now
	switch {
	case finished && c.EndOn != nil:
		x = *c.EndOn
	case finished:
		x = c.StartOn
	case paused && c.PausedOn != nil:
		x = *c.PausedOn
	}
	cal, _ := slacalendar.For(c.Schedule)
	consumed := cal.Elapsed(c.StartOn, x).Seconds() - c.BusinessPauseSeconds
	if consumed < 0 || x.Before(c.StartOn) {
		consumed = 0
	}
	budget := c.BudgetSeconds
	pct := 0.0
	if budget > 0 {
		pct = consumed / budget * 100
	} else if consumed > 0 {
		pct = 100
	}
	breached := c.HasBreached || (!finished && budget > 0 && consumed >= budget)

	out := domain.ProductIssueSLA{
		BudgetSeconds:    int64(budget),
		ConsumedSeconds:  int64(consumed),
		RemainingSeconds: int64(math.Max(0, budget-consumed)),
		Percentage:       math.Round(pct*100) / 100,
		Running:          !finished && !paused,
		Priority:         priority,
		PolicyName:       &c.PolicyName,
	}
	if c.Schedule != "" {
		out.Schedule = &c.Schedule
	}
	if paused && c.PausedOn != nil {
		t := *c.PausedOn
		out.PausedSince = &t
	}
	switch {
	case breached:
		out.State = domain.ProductIssueSLABreached
	case finished:
		out.State = domain.ProductIssueSLAMet
	case paused:
		out.State = domain.ProductIssueSLAPaused
	case pct >= productIssueAtRiskPercentage:
		out.State = domain.ProductIssueSLAAtRisk
	default:
		out.State = domain.ProductIssueSLAOK
	}
	return out
}

// applyStanding sets SLA on every summary from its loaded clock and drops the clock.
func applyStanding(items []domain.ProductIssueSummary, now time.Time) {
	for i := range items {
		items[i].SLA = computeProductIssueSLA(items[i].Clock, items[i].Priority, now)
		items[i].Clock = nil
	}
}
