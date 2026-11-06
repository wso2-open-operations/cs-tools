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

// Package businessdays computes elapsed wall-clock time under
// sla_policy.schedule = 'WSO2 Business days': Monday-Friday counts, every
// second of it (not an 8-5 window -- that's a different, unsupported
// schedule value, see sla_engine_service.go), Saturday/Sunday never counts
// at all, and day boundaries are decided in America/New_York local time (an
// explicit product decision, not a guess -- WSO2's support operation is
// anchored there regardless of where a given case's participants sit).
//
// Go's standard library has no business-day calendar helper, so this is
// written from scratch. The one subtlety worth real care is the DST
// transition: America/New_York's "spring forward" day is 23 hours long and
// its "fall back" day is 25 hours long. Computing day boundaries with
// time.Date (which resolves a wall-clock Y/M/D/H/M/S against the zone's real
// offset table) and measuring elapsed time with Time.Sub (which operates on
// absolute instants) gets this right automatically -- a naive
// start.Add(24*time.Hour) loop would not, silently losing or gaining an hour
// on the transition day. See the tests for all four required cases.
package businessdays

import (
	"log/slog"
	"time"

	// Blank-imported so America/New_York resolves even on a minimal/
	// scratch container image with no system tzdata installed -- this
	// embeds the IANA database directly in the binary. Costs a small,
	// fixed amount of binary size in exchange for never depending on the
	// deploy environment having /usr/share/zoneinfo.
	_ "time/tzdata"
)

// locationName is the IANA zone WSO2 Business days is defined against --
// deliberately America/New_York (US/Eastern with DST), confirmed by
// explicit product decision, not "whatever the server's local zone is".
const locationName = "America/New_York"

// location is loaded once at package init. A load failure (a Go runtime
// with no tzdata at all -- shouldn't happen given this binary blank-imports
// time/tzdata below, but defended regardless rather than letting every SLA
// computation panic) falls back to a fixed -5h offset (EST, no DST) and is
// logged once -- an approximation that is wrong only on the handful of days
// a year DST actually shifts the local offset, which is a far smaller
// error than crashing the whole SLA engine.
var location = loadLocation()

func loadLocation() *time.Location {
	loc, err := time.LoadLocation(locationName)
	if err != nil {
		slog.Error("businessdays: failed to load America/New_York, falling back to a fixed EST offset", "err", err)
		return time.FixedZone("EST", -5*60*60)
	}
	return loc
}

// Elapsed returns how much of [start, end) counts as "WSO2 Business days"
// time: every second that falls on a Monday-Friday calendar day in
// America/New_York, none of the time on a Saturday or Sunday there. end
// before or equal to start returns zero (never negative) -- the same
// "clock skew/near-simultaneous calls must not go negative" posture
// sla_engine_repo.go's own GREATEST(0, ...) floors apply everywhere else in
// this engine.
func Elapsed(start, end time.Time) time.Duration {
	if !end.After(start) {
		return 0
	}
	start = start.In(location)
	end = end.In(location)

	var total time.Duration
	cur := start
	for cur.Before(end) {
		nextDay := startOfNextDay(cur)
		segmentEnd := nextDay
		if end.Before(segmentEnd) {
			segmentEnd = end
		}
		if isBusinessDay(cur) {
			if d := segmentEnd.Sub(cur); d > 0 {
				total += d
			}
		}
		cur = segmentEnd
	}
	return total
}

// startOfNextDay returns local midnight of the calendar day after t, in t's
// own location. Built via time.Date (calendar arithmetic that re-resolves
// the zone offset for the new date) rather than t.Add(24*time.Hour) (fixed
// absolute-time arithmetic) specifically so a day that is actually 23 or 25
// hours long, because of a DST transition, is represented correctly --
// Add(24*Hour) would land on 11pm or 1am local time instead of midnight on
// a transition day, silently shifting every later day boundary by an hour.
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
