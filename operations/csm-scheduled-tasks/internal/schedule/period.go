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

// Package schedule computes the "period key" for a sub-cron — see this
// component's own CLAUDE.md ("Period keys") for the full design. It is a
// thin wrapper around github.com/adhocore/gronx, kept as its own package so
// the rest of this codebase depends on this concept, not on gronx directly.
package schedule

import (
	"fmt"
	"time"

	"github.com/adhocore/gronx"
)

// PeriodKey returns expr's most recent scheduled firing time at or before
// now — a daily expression (e.g. "0 2 * * *") returns the same timestamp
// for every call between that firing and its next one, which is the whole
// mechanism this package exists for: every tick in between re-derives the
// identical key, so retries land on the same ledger row instead of a new
// one. See entity-service's scheduled_task_run (and this component's
// engine.Tick) for how that key is then used.
//
// expr is a standard 5-field cron expression (minute granularity), or a
// 6-field one with an optional leading seconds field (e.g. "*/30 * * * * *"
// for every 30 seconds) — gronx auto-detects which based on field count.
// Real sub-crons should stick to 5 fields; a seconds field is mainly useful
// for fast local testing of a task's own handler. Returns an error if expr
// is not a valid cron expression.
//
// now is interpreted in its own time.Time.Location() — gronx has no
// timezone concept of its own, so every schedule in this component is
// effectively "wall clock time of whatever TZ the process runs in." Every
// schedule documented elsewhere as a UTC time (e.g. housekeeping_cleanup's
// "daily at 03:00 UTC") holds only if the deployment runs with TZ=UTC; a
// non-UTC container silently shifts every period key by its own offset.
func PeriodKey(expr string, now time.Time) (time.Time, error) {
	if !gronx.IsValid(expr) {
		return time.Time{}, fmt.Errorf("schedule: invalid cron expression %q", expr)
	}
	// inclRefTime=true: if now itself is exactly a scheduled firing moment,
	// that firing IS the period this tick belongs to, not the one before it.
	return gronx.PrevTickBefore(expr, now, true)
}

// LongestGap is the longest time between two consecutive firings of expr over
// the eight days after from. It refuses an expression that restricts the day
// of month, month or year: the rest repeat every week, so eight days see every gap.
func LongestGap(expr string, from time.Time) (time.Duration, error) {
	if !gronx.IsValid(expr) {
		return 0, fmt.Errorf("schedule: invalid cron expression %q", expr)
	}
	segs, err := gronx.Segments(expr) // seconds, minute, hour, day of month, month, day of week[, year]
	if err != nil {
		return 0, err
	}
	for _, i := range []int{3, 4, 6} {
		if i < len(segs) && segs[i] != "*" && segs[i] != "?" {
			return 0, fmt.Errorf("schedule: %q is limited to certain days of the month, months or years", expr)
		}
	}
	end := from.Add(8 * 24 * time.Hour)
	prev, err := gronx.NextTickAfter(expr, from, false)
	if err != nil {
		return 0, err
	}
	var longest time.Duration
	for prev.Before(end) {
		next, err := gronx.NextTickAfter(expr, prev, false)
		if err != nil {
			return 0, err
		}
		longest = max(longest, next.Sub(prev))
		prev = next
	}
	return longest, nil
}
