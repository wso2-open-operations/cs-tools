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

package domain

import (
	"sort"
	"time"
)

// Per-project hours: allocated, consumed and remaining time for the two
// pools a project can draw on, the onboarding pool and the query-support
// pool. ComputeProjectHours is the single place the arithmetic lives. It is
// pure (the only I/O is the billable-minutes callback the caller supplies),
// idempotent, and always derived from approved billable time cards, never
// from a previously stored consumed figure.

// OnboardingProductName is the opportunity product line that funds a
// project's onboarding.
const OnboardingProductName = "WSO2 Onboarding Services"

// OnboardingAllocationMinutes is the onboarding pool a project is granted:
// 200 hours, whatever the line's quantity.
const OnboardingAllocationMinutes = 200 * 60

// ProjectHoursLine is one product line of an opportunity linked to the
// project. Start and End are calendar dates.
type ProjectHoursLine struct {
	OpportunityID string
	Product       string
	Quantity      float64
	Start         *time.Time
	End           *time.Time
}

// ProjectHoursInput is everything ComputeProjectHours needs besides the
// time cards.
type ProjectHoursInput struct {
	// Today is the current calendar date.
	Today time.Time
	// OnboardingStatus is the project's onboarding status, empty when unset.
	OnboardingStatus string
	// OnboardingExpiry and GoLive are the project's own dates, nil when unset.
	OnboardingExpiry *time.Time
	GoLive           *time.Time
	// Lines are the product lines of the opportunities linked to the project:
	// query lines not yet ended, and every onboarding line, ended or not.
	Lines []ProjectHoursLine
	// HasLinkedLines is true when the project is linked to an opportunity that
	// has any product line at all, ended ones included. It tells a project
	// whose allowance has lapsed (zeros are correct) from one nothing is
	// linked to (the stored figures are not ours to vouch for).
	HasLinkedLines bool
}

// ProjectHours is the result, in minutes.
type ProjectHours struct {
	// HasAllocation is false only when nothing allocates hours to the project
	// and it has no linked lines at all. Nothing should be written then: the
	// stored figures are not ones this calculation can vouch for. A project
	// whose linked lines have all ended is written with zeros for the query
	// pool (HasAllocation true, OnboardingApplicable false).
	HasAllocation bool

	QueryTotal     int
	QueryConsumed  int
	QueryRemaining int // negative once the pool is overdrawn

	// OnboardingApplicable is false when no linked opportunity carries an
	// onboarding line or the status is unset or not applicable. The
	// onboarding figures are then zero and must be left untouched in storage.
	OnboardingApplicable bool
	OnboardingTotal      int
	OnboardingConsumed   int
	OnboardingRemaining  int
}

// BillableMinutesFunc returns the approved, billable minutes whose work date
// falls in [from, to] inclusive, summed over the DISTINCT projects linked to
// any of the given opportunities (a project linked through several of them is
// counted once). The allowance is granted on the opportunity and shared by its
// projects, so consumption is opportunity-wide.
type BillableMinutesFunc func(opportunityIDs []string, from, to time.Time) (int, error)

// SharedBillableMinutesFunc returns the approved, billable minutes whose work
// date falls in [from, to] inclusive, summed over the DISTINCT projects linked
// to BOTH a project in opportunity set a and one in set b (a project linked to
// both sets counts once; a project linked to only one of them counts nothing).
type SharedBillableMinutesFunc func(a, b []string, from, to time.Time) (int, error)

func dateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func minTime(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ComputeProjectHours derives a project's hours. Pools belong to opportunities
// and are shared by every project linked to them, so every project of the
// same opportunities shows the same figures.
//
// Allocated:
//   - query total: over the lines in service today (start <= today <= end), a
//     "Development Support - N hours" line grants quantity x N hours and the
//     "Query support limit (included in subscription)" line quantity x 1
//     (QueryHourProductHours);
//   - onboarding total: 200 hours when a linked opportunity carries a
//     "WSO2 Onboarding Services" line, ended or not, and the onboarding status
//     is set and not NOT_APPLICABLE.
//
// Consumed, billable approved minutes only, summed over the distinct projects
// linked to the opportunities concerned:
//   - onboarding: from the earliest onboarding line start to min(today, the
//     project's onboarding expiry, the latest onboarding line end, the go-live
//     date when COMPLETED), capped at the onboarding total. The pool fills
//     first. The window closes, so the figure is stable after expiry and keeps
//     being deducted from the query pool;
//   - query: from the earliest in-service query line start to today, less the
//     onboarding minutes that fall inside that window, never below zero. The
//     deduction is min(onboarding consumed, billable minutes in
//     [max(onboarding start, query start), onboarding end] of the projects
//     linked to both an onboarding and a query opportunity), so onboarding
//     work done before a renewed query line starts costs the new line nothing.
//     Edge case: the overlap minutes are capped at the onboarding consumed
//     figure, not apportioned by when the pool filled, so if the pool was
//     already exhausted before the query line started the deduction can still
//     be up to the overlap minutes.
//
// A project with linked lines but none in service and no applicable onboarding
// gets HasAllocation true and zeros, as the previous platform wrote. A project
// with no linked lines at all gets HasAllocation false.
//
// Remaining is total less consumed: the onboarding figure is never negative,
// the query figure goes negative when the pool is overdrawn.
func ComputeProjectHours(in ProjectHoursInput, billable BillableMinutesFunc, sharedBillable SharedBillableMinutesFunc) (ProjectHours, error) {
	today := dateOnly(in.Today)
	var out ProjectHours

	queryOpps := map[string]bool{}
	onbOpps := map[string]bool{}
	var qStart, onStart, onLastEnd *time.Time
	var onbWindowStart, onbWindowEnd time.Time
	onbWindow := false
	for _, l := range in.Lines {
		if l.Product == OnboardingProductName {
			onbOpps[l.OpportunityID] = true
			if l.Start != nil {
				s := dateOnly(*l.Start)
				if onStart == nil || s.Before(*onStart) {
					onStart = &s
				}
			}
			if l.End != nil {
				e := dateOnly(*l.End)
				if onLastEnd == nil || e.After(*onLastEnd) {
					onLastEnd = &e
				}
			}
			continue
		}
		hours, known := QueryHourProductHours[l.Product]
		if !known || l.Start == nil || l.End == nil {
			continue
		}
		start, end := dateOnly(*l.Start), dateOnly(*l.End)
		if start.After(today) || end.Before(today) {
			continue
		}
		out.QueryTotal += int(l.Quantity * float64(hours) * 60)
		out.HasAllocation = true
		queryOpps[l.OpportunityID] = true
		if qStart == nil || start.Before(*qStart) {
			qStart = &start
		}
	}

	out.OnboardingApplicable = len(onbOpps) > 0 &&
		in.OnboardingStatus != "" && in.OnboardingStatus != "NOT_APPLICABLE"
	if out.OnboardingApplicable {
		out.HasAllocation = true
		out.OnboardingTotal = OnboardingAllocationMinutes

		start := time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
		if onStart != nil {
			start = *onStart
		}
		end := today
		if in.OnboardingExpiry != nil {
			end = minTime(end, dateOnly(*in.OnboardingExpiry))
		}
		if onLastEnd != nil {
			end = minTime(end, *onLastEnd)
		}
		if in.OnboardingStatus == "COMPLETED" && in.GoLive != nil {
			end = minTime(end, dateOnly(*in.GoLive))
		}
		if !end.Before(start) {
			onbWindowStart, onbWindowEnd, onbWindow = start, end, true
			mins, err := billable(sortedKeys(onbOpps), start, end)
			if err != nil {
				return ProjectHours{}, err
			}
			out.OnboardingConsumed = min(out.OnboardingTotal, max(0, mins))
		}
		out.OnboardingRemaining = out.OnboardingTotal - out.OnboardingConsumed
	}

	if qStart != nil && !today.Before(*qStart) {
		mins, err := billable(sortedKeys(queryOpps), *qStart, today)
		if err != nil {
			return ProjectHours{}, err
		}
		deduct := 0
		if onbWindow && out.OnboardingConsumed > 0 {
			from := *qStart
			if onbWindowStart.After(from) {
				from = onbWindowStart
			}
			if !onbWindowEnd.Before(from) {
				// Only minutes in both sums come off the query figure: those
				// of projects linked to an onboarding opportunity AND to a
				// query opportunity, inside the overlap window.
				shared, err := sharedBillable(sortedKeys(onbOpps), sortedKeys(queryOpps), from, onbWindowEnd)
				if err != nil {
					return ProjectHours{}, err
				}
				deduct = min(out.OnboardingConsumed, max(0, shared))
			}
		}
		out.QueryConsumed = max(0, mins-deduct)
	}
	if !out.HasAllocation && in.HasLinkedLines {
		out.HasAllocation = true
	}
	out.QueryRemaining = out.QueryTotal - out.QueryConsumed
	return out, nil
}
