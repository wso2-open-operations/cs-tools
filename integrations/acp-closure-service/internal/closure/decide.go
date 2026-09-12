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

// Package closure implements the ACP Phase 1 decision logic: given a
// project's subscription end date and the last notice window already
// recorded for it, decide what (if anything) is due now. This package is
// pure — no I/O, no entity-service calls, no notion of action sequencing.
package closure

import "time"

// NoticeWindow is one of the confirmed ACP Phase 1 notice thresholds,
// expressed as days-remaining. NoticeWindow0 is a terminal marker: it means
// the day-0 notice email has been sent, not merely that zero days remain.
type NoticeWindow int

const (
	NoticeWindow90 NoticeWindow = 90
	NoticeWindow60 NoticeWindow = 60
	NoticeWindow30 NoticeWindow = 30
	NoticeWindow15 NoticeWindow = 15
	NoticeWindow7  NoticeWindow = 7
	NoticeWindow0  NoticeWindow = 0
)

// IsTerminal reports whether w is the terminal day-0/suspension window, as
// opposed to a day-count reminder window (90/60/30/15/7). The single
// canonical definition of this check — callers that need to branch on it
// (e.g. sweep.go's subject/body builders) should call this rather than
// independently comparing against NoticeWindow0.
func (w NoticeWindow) IsTerminal() bool {
	return w == NoticeWindow0
}

// noticeWindows is the notify cascade, ordered widest (earliest) to
// narrowest (latest). NoticeWindow0 is deliberately excluded: its notify
// gating and its suspend signal are handled separately in Decide, since
// suspend has no idempotency signal of its own in this package.
var noticeWindows = []NoticeWindow{NoticeWindow90, NoticeWindow60, NoticeWindow30, NoticeWindow15, NoticeWindow7}

// Decision is what a single evaluation of one project reports. It says only
// what is due, never who should receive it or how actions should be
// sequenced — those are the responsibility of other packages.
type Decision struct {
	DaysRemaining int
	Window        NoticeWindow
	Fires         bool
	ShouldNotify  bool
	ShouldSuspend bool
}

// Decide reports what should happen now for a project, given the current
// time, its subscription end date, and the last notice window already
// recorded for it (nil if none has fired yet).
//
// ShouldSuspend has no idempotency signal of its own here: the real signal
// (closureStatus on the entity-service Project) isn't a parameter this
// function receives, so it reports true on every evaluation once
// daysRemaining <= 0, regardless of lastNoticeWindow. The caller is
// responsible for checking closureStatus before actually invoking suspend.
func Decide(now, endDate time.Time, lastNoticeWindow *NoticeWindow) Decision {
	daysRemaining := daysBetween(now, endDate)

	if daysRemaining <= 0 {
		alreadyNotified := lastNoticeWindow != nil && *lastNoticeWindow <= NoticeWindow0
		return Decision{
			DaysRemaining: daysRemaining,
			Window:        NoticeWindow0,
			Fires:         true,
			ShouldNotify:  !alreadyNotified,
			ShouldSuspend: true,
		}
	}

	window, fires := dueWindow(daysRemaining, lastNoticeWindow)
	if !fires {
		return Decision{DaysRemaining: daysRemaining}
	}

	return Decision{
		DaysRemaining: daysRemaining,
		Window:        window,
		Fires:         true,
		ShouldNotify:  true,
	}
}

// dueWindow finds the narrowest (latest) notice window that daysRemaining
// has reached but that lastNoticeWindow hasn't already covered.
func dueWindow(daysRemaining int, lastNoticeWindow *NoticeWindow) (NoticeWindow, bool) {
	var due NoticeWindow
	found := false

	for _, w := range noticeWindows {
		if daysRemaining > int(w) {
			continue
		}
		if lastNoticeWindow != nil && *lastNoticeWindow <= w {
			continue
		}
		due = w
		found = true
	}

	return due, found
}

// StubEndDate is the single seam through which callers will eventually read
// a project's real subscription end date. entity-service's POST
// /projects/search does not yet return endDate in its response (tracked:
//, entity-service team). Swap this function's body for a
// direct field read once it ships — no other code in this package needs to
// change.
func StubEndDate(projectID string) time.Time {
	return time.Time{}
}

// daysBetween returns the number of whole days remaining between now and
// endDate, rounded up so a project isn't treated as "further out" than it
// actually is.
func daysBetween(now, endDate time.Time) int {
	d := endDate.Sub(now)
	days := d.Hours() / 24
	if days != float64(int(days)) {
		if days > 0 {
			days = float64(int(days)) + 1
		} else {
			days = float64(int(days))
		}
	}
	return int(days)
}
