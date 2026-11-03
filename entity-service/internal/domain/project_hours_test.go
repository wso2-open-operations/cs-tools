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
	"errors"
	"testing"
	"time"
)

func phDate(s string) *time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return &t
}

type phCard struct {
	date    string
	minutes int
	project string // "" = "pa"
}

// phLinks says which projects are linked to which opportunity. An opportunity
// not listed has the single project "pa".
type phLinks map[string][]string

// billableFrom serves approved billable minutes from a fixed list of cards,
// summed over the DISTINCT projects linked to the asked-for opportunities.
func billableFrom(links phLinks, cards ...phCard) BillableMinutesFunc {
	return func(opps []string, from, to time.Time) (int, error) {
		projects := map[string]bool{}
		for _, o := range opps {
			ps, ok := links[o]
			if !ok {
				ps = []string{"pa"}
			}
			for _, p := range ps {
				projects[p] = true
			}
		}
		total := 0
		for _, c := range cards {
			proj := c.project
			if proj == "" {
				proj = "pa"
			}
			d := *phDate(c.date)
			if projects[proj] && !d.Before(from) && !d.After(to) {
				total += c.minutes
			}
		}
		return total, nil
	}
}

func devLine(product string, qty float64) ProjectHoursLine {
	return ProjectHoursLine{OpportunityID: "o1", Product: product, Quantity: qty, Start: phDate("2026-01-01"), End: phDate("2026-12-31")}
}

func onbLine(start, end string) ProjectHoursLine {
	return ProjectHoursLine{OpportunityID: "o1", Product: OnboardingProductName, Quantity: 3, Start: phDate(start), End: phDate(end)}
}

func TestComputeProjectHours(t *testing.T) {
	const h = 60
	today := *phDate("2026-06-15")
	for _, tc := range []struct {
		name  string
		in    ProjectHoursInput
		links phLinks
		cards []phCard
		want  ProjectHours
	}{
		{
			name: "expired onboarding line still deducts, window closes at the line end",
			in: ProjectHoursInput{Today: today, OnboardingStatus: "IN_PROGRESS", OnboardingExpiry: phDate("2026-12-31"),
				Lines: []ProjectHoursLine{devLine("Development Support - 100 hours", 1), onbLine("2026-02-01", "2026-04-30")}},
			cards: []phCard{{"2026-03-01", 150 * h, ""}, {"2026-05-10", 25 * h, ""}},
			want: ProjectHours{HasAllocation: true, QueryTotal: 100 * h, QueryConsumed: 25*h + 0, QueryRemaining: 75 * h,
				OnboardingApplicable: true, OnboardingTotal: 200 * h, OnboardingConsumed: 150 * h, OnboardingRemaining: 50 * h},
		},
		{
			name: "project expiry earlier than the line end closes the window",
			in: ProjectHoursInput{Today: today, OnboardingStatus: "IN_PROGRESS", OnboardingExpiry: phDate("2026-03-31"),
				Lines: []ProjectHoursLine{devLine("Development Support - 100 hours", 1), onbLine("2026-02-01", "2026-12-31")}},
			cards: []phCard{{"2026-03-01", 20 * h, ""}, {"2026-04-10", 30 * h, ""}},
			want: ProjectHours{HasAllocation: true, QueryTotal: 100 * h, QueryConsumed: 30 * h, QueryRemaining: 70 * h,
				OnboardingApplicable: true, OnboardingTotal: 200 * h, OnboardingConsumed: 20 * h, OnboardingRemaining: 180 * h},
		},
		{
			name: "two projects sharing one opportunity both show the same pool",
			in: ProjectHoursInput{Today: today, OnboardingStatus: "NOT_APPLICABLE",
				Lines: []ProjectHoursLine{devLine("Development Support - 100 hours", 1)}},
			links: phLinks{"o1": {"pa", "pb"}},
			cards: []phCard{{"2026-03-01", 10 * h, "pa"}, {"2026-03-02", 15 * h, "pb"}},
			want:  ProjectHours{HasAllocation: true, QueryTotal: 100 * h, QueryConsumed: 25 * h, QueryRemaining: 75 * h},
		},
		{
			name: "a project in two opportunities: totals add, a shared project is counted once",
			in: ProjectHoursInput{Today: today, OnboardingStatus: "NOT_APPLICABLE", Lines: []ProjectHoursLine{
				devLine("Development Support - 100 hours", 1),
				{OpportunityID: "o2", Product: "Development Support - 50 hours", Quantity: 1, Start: phDate("2026-03-01"), End: phDate("2026-12-31")},
			}},
			links: phLinks{"o1": {"pa", "pb"}, "o2": {"pa", "pc"}},
			cards: []phCard{{"2026-03-05", 10 * h, "pa"}, {"2026-03-06", 5 * h, "pb"}, {"2026-03-07", 7 * h, "pc"}, {"2026-02-01", 100 * h, "pz"}},
			want:  ProjectHours{HasAllocation: true, QueryTotal: 150 * h, QueryConsumed: 22 * h, QueryRemaining: 128 * h},
		},
		{
			name:  "no onboarding",
			in:    ProjectHoursInput{Today: today, OnboardingStatus: "NOT_APPLICABLE", Lines: []ProjectHoursLine{devLine("Development Support - 100 hours", 1)}},
			cards: []phCard{{"2026-03-01", 30 * h, ""}},
			want:  ProjectHours{HasAllocation: true, QueryTotal: 100 * h, QueryConsumed: 30 * h, QueryRemaining: 70 * h},
		},
		{
			name: "onboarding partially consumed",
			in: ProjectHoursInput{Today: today, OnboardingStatus: "IN_PROGRESS", OnboardingExpiry: phDate("2026-12-31"),
				Lines: []ProjectHoursLine{devLine("Development Support - 100 hours", 1), onbLine("2026-02-01", "2026-12-31")}},
			cards: []phCard{{"2026-03-01", 50 * h, ""}},
			want: ProjectHours{HasAllocation: true, QueryTotal: 100 * h, QueryConsumed: 0, QueryRemaining: 100 * h,
				OnboardingApplicable: true, OnboardingTotal: 200 * h, OnboardingConsumed: 50 * h, OnboardingRemaining: 150 * h},
		},
		{
			name: "onboarding exhausted, overflow goes to query",
			in: ProjectHoursInput{Today: today, OnboardingStatus: "IN_PROGRESS", OnboardingExpiry: phDate("2026-12-31"),
				Lines: []ProjectHoursLine{devLine("Development Support - 100 hours", 1), onbLine("2026-02-01", "2026-12-31")}},
			cards: []phCard{{"2026-03-01", 230 * h, ""}},
			want: ProjectHours{HasAllocation: true, QueryTotal: 100 * h, QueryConsumed: 30 * h, QueryRemaining: 70 * h,
				OnboardingApplicable: true, OnboardingTotal: 200 * h, OnboardingConsumed: 200 * h, OnboardingRemaining: 0},
		},
		{
			name: "onboarding expired mid-window, later hours go to query",
			in: ProjectHoursInput{Today: today, OnboardingStatus: "IN_PROGRESS", OnboardingExpiry: phDate("2026-04-30"),
				Lines: []ProjectHoursLine{devLine("Development Support - 100 hours", 1), onbLine("2026-02-01", "2026-12-31")}},
			cards: []phCard{{"2026-03-01", 40 * h, ""}, {"2026-05-10", 25 * h, ""}},
			want: ProjectHours{HasAllocation: true, QueryTotal: 100 * h, QueryConsumed: 25 * h, QueryRemaining: 75 * h,
				OnboardingApplicable: true, OnboardingTotal: 200 * h, OnboardingConsumed: 40 * h, OnboardingRemaining: 160 * h},
		},
		{
			name: "completed, go-live cuts the window",
			in: ProjectHoursInput{Today: today, OnboardingStatus: "COMPLETED", OnboardingExpiry: phDate("2026-12-31"), GoLive: phDate("2026-03-31"),
				Lines: []ProjectHoursLine{devLine("Development Support - 100 hours", 1), onbLine("2026-02-01", "2026-12-31")}},
			cards: []phCard{{"2026-03-01", 10 * h, ""}, {"2026-04-10", 20 * h, ""}},
			want: ProjectHours{HasAllocation: true, QueryTotal: 100 * h, QueryConsumed: 20 * h, QueryRemaining: 80 * h,
				OnboardingApplicable: true, OnboardingTotal: 200 * h, OnboardingConsumed: 10 * h, OnboardingRemaining: 190 * h},
		},
		{
			name: "go-live ignored unless completed",
			in: ProjectHoursInput{Today: today, OnboardingStatus: "IN_PROGRESS", OnboardingExpiry: phDate("2026-12-31"), GoLive: phDate("2026-03-31"),
				Lines: []ProjectHoursLine{onbLine("2026-02-01", "2026-12-31")}},
			cards: []phCard{{"2026-04-10", 20 * h, ""}},
			want: ProjectHours{HasAllocation: true,
				OnboardingApplicable: true, OnboardingTotal: 200 * h, OnboardingConsumed: 20 * h, OnboardingRemaining: 180 * h},
		},
		{
			name: "not applicable: onboarding line ignored, nothing absorbed",
			in: ProjectHoursInput{Today: today, OnboardingStatus: "NOT_APPLICABLE",
				Lines: []ProjectHoursLine{devLine("Development Support - 100 hours", 1), onbLine("2026-02-01", "2026-12-31")}},
			cards: []phCard{{"2026-03-01", 50 * h, ""}},
			want:  ProjectHours{HasAllocation: true, QueryTotal: 100 * h, QueryConsumed: 50 * h, QueryRemaining: 50 * h},
		},
		{
			name:  "unset status is not applicable",
			in:    ProjectHoursInput{Today: today, Lines: []ProjectHoursLine{devLine("Development Support - 10 hours", 1), onbLine("2026-02-01", "2026-12-31")}},
			cards: []phCard{{"2026-03-01", 5 * h, ""}},
			want:  ProjectHours{HasAllocation: true, QueryTotal: 10 * h, QueryConsumed: 5 * h, QueryRemaining: 5 * h},
		},
		{
			name:  "exceeded query pool goes negative",
			in:    ProjectHoursInput{Today: today, OnboardingStatus: "NOT_APPLICABLE", Lines: []ProjectHoursLine{devLine("Development Support - 10 hours", 1)}},
			cards: []phCard{{"2026-03-01", 12*h + 30, ""}},
			want:  ProjectHours{HasAllocation: true, QueryTotal: 10 * h, QueryConsumed: 12*h + 30, QueryRemaining: -(2*h + 30)},
		},
		{
			name: "multiple development support lines and the included limit add up",
			in: ProjectHoursInput{Today: today, OnboardingStatus: "NOT_APPLICABLE", Lines: []ProjectHoursLine{
				devLine("Development Support - 50 hours", 2),
				devLine("Development Support - 25 hours", 1),
				devLine("Query support limit (included in subscription)", 40),
				devLine("Some Unknown Product", 9),
			}},
			cards: []phCard{{"2026-03-01", 10 * h, ""}},
			want:  ProjectHours{HasAllocation: true, QueryTotal: 165 * h, QueryConsumed: 10 * h, QueryRemaining: 155 * h},
		},
		{
			name: "lines outside their service dates do not count",
			in: ProjectHoursInput{Today: today, OnboardingStatus: "NOT_APPLICABLE", Lines: []ProjectHoursLine{
				{Product: "Development Support - 100 hours", Quantity: 1, Start: phDate("2026-07-01"), End: phDate("2026-12-31")},
				{Product: "Development Support - 100 hours", Quantity: 1, Start: phDate("2025-01-01"), End: phDate("2026-06-14")},
				{Product: "Development Support - 10 hours", Quantity: 1, Start: phDate("2026-06-15"), End: phDate("2026-06-15")},
			}},
			want: ProjectHours{HasAllocation: true, QueryTotal: 10 * h, QueryRemaining: 10 * h},
		},
		{
			name: "renewal: onboarding ended before the new query line starts, nothing deducted",
			in: ProjectHoursInput{Today: today, OnboardingStatus: "IN_PROGRESS", OnboardingExpiry: phDate("2026-12-31"),
				Lines: []ProjectHoursLine{
					{OpportunityID: "o1", Product: "Development Support - 100 hours", Quantity: 1, Start: phDate("2026-05-01"), End: phDate("2026-12-31")},
					onbLine("2026-02-01", "2026-04-30")}},
			cards: []phCard{{"2026-03-01", 150 * h, ""}, {"2026-05-10", 25 * h, ""}},
			want: ProjectHours{HasAllocation: true, QueryTotal: 100 * h, QueryConsumed: 25 * h, QueryRemaining: 75 * h,
				OnboardingApplicable: true, OnboardingTotal: 200 * h, OnboardingConsumed: 150 * h, OnboardingRemaining: 50 * h},
		},
		{
			name: "onboarding overlaps the start of the query line: only the overlap is deducted",
			in: ProjectHoursInput{Today: today, OnboardingStatus: "IN_PROGRESS", OnboardingExpiry: phDate("2026-12-31"),
				Lines: []ProjectHoursLine{
					{OpportunityID: "o1", Product: "Development Support - 100 hours", Quantity: 1, Start: phDate("2026-04-01"), End: phDate("2026-12-31")},
					onbLine("2026-02-01", "2026-12-31")}},
			cards: []phCard{{"2026-03-01", 50 * h, ""}, {"2026-04-10", 30 * h, ""}, {"2026-05-10", 20 * h, ""}},
			want: ProjectHours{HasAllocation: true, QueryTotal: 100 * h, QueryConsumed: 0, QueryRemaining: 100 * h,
				OnboardingApplicable: true, OnboardingTotal: 200 * h, OnboardingConsumed: 100 * h, OnboardingRemaining: 100 * h},
		},
		{
			name:  "lapsed allowance: linked lines, none in service, writes zeros",
			in:    ProjectHoursInput{Today: today, OnboardingStatus: "NOT_APPLICABLE", HasLinkedLines: true},
			cards: []phCard{{"2026-03-01", 10 * h, ""}},
			want:  ProjectHours{HasAllocation: true},
		},
		{
			name:  "linked-lines flag changes nothing when a line is in service",
			in:    ProjectHoursInput{Today: today, OnboardingStatus: "IN_PROGRESS", HasLinkedLines: true, Lines: []ProjectHoursLine{devLine("Development Support - 10 hours", 1)}},
			cards: nil,
			want:  ProjectHours{HasAllocation: true, QueryTotal: 10 * h, QueryRemaining: 10 * h},
		},
		{
			name:  "zero allocation: nothing to write",
			in:    ProjectHoursInput{Today: today, OnboardingStatus: "NOT_APPLICABLE"},
			cards: []phCard{{"2026-03-01", 10 * h, ""}},
			want:  ProjectHours{},
		},
		{
			name: "future onboarding line opens the window, hours before it start do not count",
			in: ProjectHoursInput{Today: today, OnboardingStatus: "NOT_STARTED", OnboardingExpiry: phDate("2026-12-31"),
				Lines: []ProjectHoursLine{onbLine("2026-07-01", "2026-12-31")}},
			cards: []phCard{{"2026-03-01", 10 * h, ""}},
			want: ProjectHours{HasAllocation: true,
				OnboardingApplicable: true, OnboardingTotal: 200 * h, OnboardingRemaining: 200 * h},
		},
		{
			name: "two onboarding lines still grant 200 hours",
			in: ProjectHoursInput{Today: today, OnboardingStatus: "IN_PROGRESS", OnboardingExpiry: phDate("2026-12-31"),
				Lines: []ProjectHoursLine{onbLine("2026-02-01", "2026-12-31"), onbLine("2026-03-01", "2026-12-31")}},
			cards: []phCard{{"2026-02-15", 10 * h, ""}},
			want: ProjectHours{HasAllocation: true,
				OnboardingApplicable: true, OnboardingTotal: 200 * h, OnboardingConsumed: 10 * h, OnboardingRemaining: 190 * h},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ComputeProjectHours(tc.in, billableFrom(tc.links, tc.cards...))
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("got  %+v\nwant %+v", got, tc.want)
			}
		})
	}
}

// Same inputs, same answer: a second run changes nothing, because nothing
// stored is read back.
func TestComputeProjectHoursIsIdempotent(t *testing.T) {
	in := ProjectHoursInput{Today: *phDate("2026-06-15"), OnboardingStatus: "IN_PROGRESS", OnboardingExpiry: phDate("2026-12-31"),
		Lines: []ProjectHoursLine{devLine("Development Support - 100 hours", 1), onbLine("2026-02-01", "2026-12-31")}}
	f := billableFrom(nil, phCard{"2026-03-01", 230 * 60, ""})
	a, _ := ComputeProjectHours(in, f)
	b, _ := ComputeProjectHours(in, f)
	if a != b {
		t.Errorf("%+v != %+v", a, b)
	}
}

func TestComputeProjectHoursPropagatesError(t *testing.T) {
	boom := errors.New("boom")
	_, err := ComputeProjectHours(ProjectHoursInput{Today: *phDate("2026-06-15"), OnboardingStatus: "NOT_APPLICABLE",
		Lines: []ProjectHoursLine{devLine("Development Support - 10 hours", 1)}},
		func(_ []string, _, _ time.Time) (int, error) { return 0, boom })
	if !errors.Is(err, boom) {
		t.Errorf("err = %v", err)
	}
}

// After the onboarding line has ended the figure no longer moves with later
// work, and it keeps being deducted from the query pool, run after run.
func TestComputeProjectHoursExpiredOnboardingIsStable(t *testing.T) {
	const h = 60
	in := func(today string) ProjectHoursInput {
		return ProjectHoursInput{Today: *phDate(today), OnboardingStatus: "IN_PROGRESS", OnboardingExpiry: phDate("2026-04-30"),
			Lines: []ProjectHoursLine{devLine("Development Support - 100 hours", 1), onbLine("2026-02-01", "2026-04-30")}}
	}
	cards := []phCard{{"2026-03-01", 150 * h, ""}, {"2026-05-10", 25 * h, ""}}
	first, _ := ComputeProjectHours(in("2026-06-15"), billableFrom(nil, cards...))
	again, _ := ComputeProjectHours(in("2026-06-15"), billableFrom(nil, cards...))
	later, _ := ComputeProjectHours(in("2026-09-01"), billableFrom(nil, cards...))
	if first != again || first != later {
		t.Errorf("not stable: %+v, %+v, %+v", first, again, later)
	}
	if first.OnboardingConsumed != 150*h || first.QueryConsumed != 25*h {
		t.Errorf("got %+v", first)
	}
}
