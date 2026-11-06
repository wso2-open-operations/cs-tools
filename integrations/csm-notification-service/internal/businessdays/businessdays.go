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

// Package businessdays computes a due instant under sla_policy.schedule =
// 'WSO2 Business days': Monday-Friday counts, every second of it (not an
// 8-5 window), Saturday/Sunday never counts at all, and day boundaries are
// decided in America/New_York local time -- the exact same rule
// entity-service's own internal/businessdays package applies (that package
// is this one's source of truth; keep the two in sync by hand, same
// cross-repo porting convention this service's own internal/events/
// internal/eventbus already follow against entity-service/
// apps/csm-portal/backend).
//
// entity-service computes ELAPSED business-day time (how much of
// [start, now) counts); this package computes the INVERSE -- the instant
// reached after adding a given business-day DURATION to a start instant --
// which is what RegisterClocks/reconcileClock need to schedule a
// business-day-aware due date/wake-up instead of flat wall-clock
// start.Add(duration) math. Both read the exact same calendar.
package businessdays

import (
	"log/slog"
	"time"

	// Blank-imported so America/New_York resolves even on a minimal/scratch
	// container image with no system tzdata installed -- see
	// entity-service's own internal/businessdays for the identical reasoning.
	_ "time/tzdata"
)

// locationName is the IANA zone WSO2 Business days is defined against --
// must match entity-service's own internal/businessdays.locationName
// exactly, or the two services would disagree about where a day boundary
// falls for the same clock.
const locationName = "America/New_York"

var location = loadLocation()

func loadLocation() *time.Location {
	loc, err := time.LoadLocation(locationName)
	if err != nil {
		slog.Error("businessdays: failed to load America/New_York, falling back to a fixed EST offset", "err", err)
		return time.FixedZone("EST", -5*60*60)
	}
	return loc
}

// AddBusinessDuration returns the instant reached by adding d of "WSO2
// Business days" time to start, walking forward day by day and skipping
// every Saturday/Sunday entirely -- the inverse of entity-service's own
// businessdays.Elapsed. Day boundaries use time.Date (calendar arithmetic
// that re-resolves America/New_York's real offset for each date) rather
// than a fixed 24-hour step, so a DST transition day (23 or 25 real hours)
// is handled correctly, same reasoning as entity-service's own
// startOfNextDay. d <= 0 returns start unchanged. The result is converted
// back to start's own original location purely for readability (callers
// comparing/formatting it); the instant itself -- which is all scheduling
// logic actually compares -- is identical regardless of which zone it's
// displayed in.
func AddBusinessDuration(start time.Time, d time.Duration) time.Time {
	if d <= 0 {
		return start
	}
	cur := start.In(location)
	remaining := d
	for {
		nextDay := startOfNextDay(cur)
		segmentCap := nextDay.Sub(cur)
		if isBusinessDay(cur) {
			if remaining <= segmentCap {
				return cur.Add(remaining).In(start.Location())
			}
			remaining -= segmentCap
		}
		cur = nextDay
	}
}

// startOfNextDay returns local midnight of the calendar day after t, in t's
// own location -- see entity-service's own businessdays.startOfNextDay for
// why time.Date rather than Add(24*time.Hour).
func startOfNextDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d+1, 0, 0, 0, 0, t.Location())
}

func isBusinessDay(t time.Time) bool {
	switch t.Weekday() {
	case time.Saturday, time.Sunday:
		return false
	default:
		return true
	}
}
