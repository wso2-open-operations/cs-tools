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
	"context"
	"encoding/json"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// The two dashboard endpoints ported from ServiceNow's /availabilities and
// /history resources.
//
// These pin the behaviours a live diff cannot: the ones that only show up on
// data the dev instance does not currently have. The diff against the live
// API (cmd/availdiff) covers agreement on real data; this covers the edges.

type fakeDashboardRepo struct {
	monitors    []repository.MonitorRow
	history     []repository.MonitorRow
	parentAvail map[string][]repository.ParentAvailabilityRow
	daily       []repository.DailyAvailabilityRow

	dailyFrom, dailyTo, dailyTZ string

	detail                      *repository.IncidentDetailRow
	comments                    []repository.OutageCommentRow
	gotDetailID, gotDetailCloud string

	// registry overrides the seeded clouds when a test needs its own.
	registry *fixtureClouds
}

func (f *fakeDashboardRepo) Monitors(context.Context, string) ([]repository.MonitorRow, error) {
	return f.monitors, nil
}
func (f *fakeDashboardRepo) Availabilities(context.Context, []string) ([]repository.AvailabilityRow, error) {
	return nil, nil
}
func (f *fakeDashboardRepo) OngoingOutages(context.Context, []string) ([]repository.OngoingOutageRow, error) {
	return nil, nil
}
func (f *fakeDashboardRepo) Incidents(context.Context, string, time.Time) ([]repository.IncidentRow, error) {
	return nil, nil
}
func (f *fakeDashboardRepo) ParentAvailabilities(_ context.Context, parentID string) ([]repository.ParentAvailabilityRow, error) {
	return f.parentAvail[parentID], nil
}
func (f *fakeDashboardRepo) DailyAvailability(_ context.Context, _ []string, tz, from, to string) ([]repository.DailyAvailabilityRow, error) {
	f.dailyTZ, f.dailyFrom, f.dailyTo = tz, from, to
	return f.daily, nil
}
func (f *fakeDashboardRepo) MonitorsForHistory(context.Context, string) ([]repository.MonitorRow, error) {
	return f.history, nil
}
func (f *fakeDashboardRepo) IncidentDetail(_ context.Context, id, cloud string) (*repository.IncidentDetailRow, error) {
	f.gotDetailID, f.gotDetailCloud = id, cloud
	return f.detail, nil
}
func (f *fakeDashboardRepo) OutageComments(context.Context, string) ([]repository.OutageCommentRow, error) {
	return f.comments, nil
}

// TestAvailabilityWeightsSumPerRegion is the invariant calculateAgentManager
// states in its own comment: "Weights within a region must total 100."
//
// The code cannot see which offering belongs to which region -- that is a
// database fact -- so what is checkable here is the whole-cloud total, which
// must be 100 per region. A weight mistyped by a digit fails this.
func TestAvailabilityWeightsSumPerRegion(t *testing.T) {
	for cloud, plan := range seededCloudPlans {
		if !plan.Weighted() {
			continue
		}
		var sum float64
		for _, w := range plan.Weights {
			sum += w
		}
		want := 100 * float64(len(plan.Regions))
		// The weights carry two decimals (13.75, 7.5, 4.5), so compare with
		// a tolerance rather than exactly.
		if diff := sum - want; diff > 1e-9 || diff < -1e-9 {
			t.Errorf("%s: weights sum to %v, want %v (%d regions x 100)",
				cloud, sum, want, len(plan.Regions))
		}
	}
}

// TestAvailabilityPlansCoverEveryCloud guards the two lists drifting apart:
// validateCloud accepts seven slugs and every one needs a plan, or the
// endpoint 400s on a cloud the other endpoints serve.
func TestAvailabilityPlansCoverEveryCloud(t *testing.T) {
	for slug := range seededCloudOfferings {
		if _, ok := seededCloudPlans[slug]; !ok {
			t.Errorf("cloud %q is accepted but has no availability plan", slug)
		}
	}
	for slug := range seededCloudPlans {
		if _, ok := seededCloudOfferings[slug]; !ok {
			t.Errorf("cloud %q has a plan but is not an accepted cloud", slug)
		}
	}
}

// TestJSToFixed pins the rounding against JavaScript's toFixed, including the
// tie rule Go's strconv gets differently.
func TestJSToFixed(t *testing.T) {
	cases := []struct {
		in   float64
		want string
		why  string
	}{
		{100, "100.000", "trailing zeros are kept -- this is the string form"},
		{0, "0.000", ""},
		{99.8224, "99.822", "below the half, rounds down"},
		{99.8226, "99.823", "above the half, rounds up"},

		// EXACT ties, where ECMA-262 and Go's strconv genuinely disagree.
		// Both of these are representable to the bit -- 1/16 and 5/16 -- so
		// the half is real rather than an artefact of decimal notation.
		// JavaScript picks the larger n; Go rounds half to even.
		{0.0625, "0.063", "62.5 -> 63 in JS, 62 in Go"},
		{0.3125, "0.313", "312.5 -> 313 in JS, 312 in Go"},

		// And one that LOOKS like a tie and is not: the nearest double to
		// 99.9995 sits just below it, so JavaScript rounds down here too.
		// Worth pinning, because assuming otherwise is the obvious mistake.
		{99.9995, "99.999", "not actually a tie once it is a float64"},
	}
	for _, c := range cases {
		if got := domain.JSToFixed(c.in, 3); got != c.want {
			t.Errorf("JSToFixed(%v) = %q, want %q (%s)", c.in, got, c.want, c.why)
		}
	}

	// The tie cases are only worth carrying if strconv really differs.
	for _, in := range []float64{0.0625, 0.3125} {
		if strconv.FormatFloat(in, 'f', 3, 64) == domain.JSToFixed(in, 3) {
			t.Errorf("strconv and JSToFixed agree on %v; this case no longer "+
				"guards anything and the custom rounding may be unnecessary", in)
		}
	}
}

// TestTrimJSNumber pins what makes asgardeo's figure arrive as 100 rather
// than "100.000": parseFloat drops the trailing zeros before JSON sees it.
func TestTrimJSNumber(t *testing.T) {
	cases := map[string]string{
		"100.000": "100",
		"99.822":  "99.822",
		"99.800":  "99.8",
		"0.000":   "0",
	}
	for in, want := range cases {
		if got := domain.TrimJSNumber(in); got != want {
			t.Errorf("TrimJSNumber(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestAvailabilitiesWireTypes is the difference that survives every test that
// only checks numbers: six clouds emit a JSON string, asgardeo a bare number.
func TestAvailabilitiesWireTypes(t *testing.T) {
	repo := &fakeDashboardRepo{parentAvail: map[string][]repository.ParentAvailabilityRow{}}
	svc := NewCloudStatusDashboardService(repo)

	// asgardeo: unweighted, two offerings, both perfect -> the number 100.
	as := seededCloudPlans["asgardeo"]
	for _, r := range as.Regions {
		repo.parentAvail[r.ParentID] = []repository.ParentAvailabilityRow{
			{ServiceOfferingID: "a", Window: "LAST_7_DAYS", Availability: 100},
			{ServiceOfferingID: "b", Window: "LAST_7_DAYS", Availability: 100},
		}
	}
	got, err := svc.Availabilities(context.Background(), "asgardeo")
	if err != nil {
		t.Fatalf("asgardeo: %v", err)
	}
	b, _ := json.Marshal(got[as.Regions[0].Key][0].Availability)
	if string(b) != "100" {
		t.Errorf("asgardeo availability marshalled as %s, want the bare number 100", b)
	}

	// moesif: weighted, every offering at 100 -> the string "100.000".
	mo := seededCloudPlans["moesif"]
	rows := make([]repository.ParentAvailabilityRow, 0, len(mo.Weights))
	for id := range mo.Weights {
		rows = append(rows, repository.ParentAvailabilityRow{
			ServiceOfferingID: id, Window: "LAST_7_DAYS", Availability: 100,
		})
	}
	repo.parentAvail[mo.Regions[0].ParentID] = rows
	got, err = svc.Availabilities(context.Background(), "moesif")
	if err != nil {
		t.Fatalf("moesif: %v", err)
	}
	b, _ = json.Marshal(got[mo.Regions[0].Key][0].Availability)
	if string(b) != `"100.000"` {
		t.Errorf("moesif availability marshalled as %s, want the string \"100.000\"", b)
	}
}

// TestAvailabilitiesSkipsUnmappedOffering is the NaN trap.
//
// In ServiceNow an offering absent from map_weights makes running_count NaN
// for good, and the whole region publishes "NaN". Here it is skipped, so one
// unweighted offering costs its own share and nothing else.
func TestAvailabilitiesSkipsUnmappedOffering(t *testing.T) {
	mo := seededCloudPlans["moesif"]
	rows := []repository.ParentAvailabilityRow{
		{ServiceOfferingID: "not-in-any-weight-map", Window: "LAST_7_DAYS", Availability: 42},
	}
	for id := range mo.Weights {
		rows = append(rows, repository.ParentAvailabilityRow{
			ServiceOfferingID: id, Window: "LAST_7_DAYS", Availability: 100,
		})
	}
	repo := &fakeDashboardRepo{parentAvail: map[string][]repository.ParentAvailabilityRow{
		mo.Regions[0].ParentID: rows,
	}}
	got, err := NewCloudStatusDashboardService(repo).Availabilities(context.Background(), "moesif")
	if err != nil {
		t.Fatalf("moesif: %v", err)
	}
	if v := got[mo.Regions[0].Key][0].Availability.Value; v != "100.000" {
		t.Errorf("availability = %q, want \"100.000\" -- an unmapped offering must not poison the region", v)
	}
}

// TestAvailabilitiesAllFourWindows guards the window list and its order: the
// monitors endpoint needed two, this one needs four and the frontend renders
// them in this sequence.
func TestAvailabilitiesAllFourWindows(t *testing.T) {
	mo := seededCloudPlans["moesif"]
	repo := &fakeDashboardRepo{parentAvail: map[string][]repository.ParentAvailabilityRow{}}
	got, err := NewCloudStatusDashboardService(repo).Availabilities(context.Background(), "moesif")
	if err != nil {
		t.Fatalf("moesif: %v", err)
	}
	want := []string{"Last 7 days", "Last 30 days", "Last 90 days", "Last 12 months"}
	windows := got[mo.Regions[0].Key]
	if len(windows) != len(want) {
		t.Fatalf("got %d windows, want %d", len(windows), len(want))
	}
	for i, w := range want {
		if windows[i].Duration != w {
			t.Errorf("window %d = %q, want %q", i, windows[i].Duration, w)
		}
	}
}

// TestHistoryTodayFill covers the script's own "workaround to handle the
// missing data", including the case it deliberately does not cover.
func TestHistoryTodayFill(t *testing.T) {
	const today = "2026-09-29"

	t.Run("appends today when absent", func(t *testing.T) {
		in := []domain.AvailabilityHistoryPoint{{Availability: 99, Date: "2026-09-27"}}
		got := historyFor(in, today)
		if len(got) != 2 || got[1].Date != today || got[1].Availability != 100 {
			t.Errorf("got %+v, want a synthetic 100 at %s appended", got, today)
		}
	})

	t.Run("does not duplicate today when present", func(t *testing.T) {
		in := []domain.AvailabilityHistoryPoint{{Availability: 98, Date: today}}
		got := historyFor(in, today)
		if len(got) != 1 || got[0].Availability != 98 {
			t.Errorf("got %+v, want the real value kept and no fill", got)
		}
	})

	t.Run("does not fabricate a point for a monitor with no data", func(t *testing.T) {
		// The script pushes the fill inside its row loop, so a monitor with
		// zero rows gets an empty history rather than a fabricated 100.
		got := historyFor(nil, today)
		if len(got) != 0 {
			t.Errorf("got %+v, want an empty history", got)
		}
		if got == nil {
			t.Error("history must be an empty array, not null -- the frontend iterates it")
		}
	})
}

// TestHistoryCapKeepsMostRecent pins which end of the array the cap takes.
func TestHistoryCapKeepsMostRecent(t *testing.T) {
	in := make([]domain.AvailabilityHistoryPoint, 0, 95)
	for i := 0; i < 95; i++ {
		d := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, i)
		in = append(in, domain.AvailabilityHistoryPoint{Availability: float64(i), Date: d.Format("2006-01-02")})
	}
	got := historyFor(in, "2026-09-29")
	if len(got) != domain.AvailabilityHistoryDays {
		t.Fatalf("got %d points, want %d", len(got), domain.AvailabilityHistoryDays)
	}
	// Oldest dropped, newest kept: slice(-90) on a chronological array.
	if got[0].Date != "2026-06-07" {
		t.Errorf("first kept date = %s, want 2026-06-07 (the oldest six dropped)", got[0].Date)
	}
}

// TestHistoryWindowAndTimezone pins the two values the live diff had to
// correct, so neither silently regresses.
//
// The window is 92 calendar days, not 90 -- gs.beginningOfLast90Days() is
// today minus 91 -- and the labelling is Asia/Colombo, which is where the
// 18:30Z buckets are cut.
func TestHistoryWindowAndTimezone(t *testing.T) {
	repo := &fakeDashboardRepo{}
	svc := NewCloudStatusDashboardService(repo).(*cloudStatusDashboardService)
	svc.now = func() time.Time { return time.Date(2026, 9, 29, 8, 45, 0, 0, time.UTC) }

	if _, err := svc.AvailabilityHistory(context.Background(), "moesif"); err != nil {
		t.Fatalf("history: %v", err)
	}
	if repo.dailyTZ != "Asia/Colombo" {
		t.Errorf("timezone = %q, want Asia/Colombo", repo.dailyTZ)
	}
	if repo.dailyFrom != "2026-06-30" {
		t.Errorf("window start = %q, want 2026-06-30 (today minus 91)", repo.dailyFrom)
	}
	if repo.dailyTo != "2026-09-29" {
		t.Errorf("window end = %q, want 2026-09-29 (today)", repo.dailyTo)
	}
}

// TestHistoryGroupsInRowOrder guards the append-by-row-order assembly: the
// repository's ORDER BY decides the page, and a map would lose it.
func TestHistoryGroupsInRowOrder(t *testing.T) {
	repo := &fakeDashboardRepo{
		history: []repository.MonitorRow{
			{Region: "us", Group: "Login", Name: "Console", ServiceOfferingID: "o1"},
			{Region: "us", Group: "Runtime", Name: "Invocation", ServiceOfferingID: "o2"},
			{Region: "us", Group: "Login", Name: "SSO", ServiceOfferingID: "o3"},
		},
		daily: []repository.DailyAvailabilityRow{
			{ServiceOfferingID: "o1", Date: "2026-09-29", Availability: 100},
		},
	}
	got, err := NewCloudStatusDashboardService(repo).AvailabilityHistory(context.Background(), "moesif")
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	groups := got["us"]
	if len(groups) != 2 {
		t.Fatalf("got %d groups, want 2 -- a repeated group must gain a subgroup, not a new group", len(groups))
	}
	if groups[0].DisplayName != "Login" || groups[1].DisplayName != "Runtime" {
		t.Errorf("groups = %q,%q; want Login,Runtime in row order",
			groups[0].DisplayName, groups[1].DisplayName)
	}
	if len(groups[0].Subgroups) != 2 {
		t.Errorf("Login has %d subgroups, want 2", len(groups[0].Subgroups))
	}
}

// TestIncidentDetailShapes covers the two payloads and the gate between
// them. The sparse one is the majority case on real data, so getting it
// wrong would change most incident pages rather than an edge case.
func TestIncidentDetailShapes(t *testing.T) {
	base := &repository.IncidentDetailRow{
		ID:               "79adad2d-1b45-fa10-0bb3-da47b04bcb46",
		Begin:            "2025-11-06 09:54:17",
		End:              "2025-11-10 04:49:30",
		Type:             "DEGRADATION",
		ShortDescription: "Asgardeo Login Flow Outage",
		IncidentState:    "IN_PROGRESS",
	}

	t.Run("qualifying incident yields the full detail", func(t *testing.T) {
		repo := &fakeDashboardRepo{detail: base, comments: []repository.OutageCommentRow{
			{Comment: "newest", CreatedOn: "2025-11-07 02:36:44"},
		}}
		got, err := NewCloudStatusDashboardService(repo).
			IncidentDetail(context.Background(), base.ID, "asgardeo")
		if err != nil {
			t.Fatalf("detail: %v", err)
		}
		d, ok := got.(domain.CloudStatusIncidentDetail)
		if !ok {
			t.Fatalf("got %T, want the full detail shape", got)
		}
		// Dashless on the wire -- the frontend puts this straight in a URL.
		if d.ID != "79adad2d1b45fa100bb3da47b04bcb46" {
			t.Errorf("id = %q, want the 32-hex sys_id form", d.ID)
		}
		// degradation renders as "Degraded", not "Degradation".
		if d.Type != "Degraded" {
			t.Errorf("type = %q, want %q", d.Type, "Degraded")
		}
		if d.Status != "Resolved" {
			t.Errorf("status = %q, want Resolved (end is set)", d.Status)
		}
		if len(d.Comments) != 1 {
			t.Errorf("got %d comments, want 1", len(d.Comments))
		}
		if d.Attachments == nil {
			t.Error("attachments must be an array, not null")
		}
	})

	for _, state := range []string{"", "NEW", "ON_HOLD", "CANCELED"} {
		t.Run("state "+state+" yields attachments only", func(t *testing.T) {
			row := *base
			row.IncidentState = state
			repo := &fakeDashboardRepo{detail: &row}
			got, err := NewCloudStatusDashboardService(repo).
				IncidentDetail(context.Background(), base.ID, "asgardeo")
			if err != nil {
				t.Fatalf("detail: %v", err)
			}
			if _, ok := got.(domain.CloudStatusIncidentAttachmentsOnly); !ok {
				t.Errorf("got %T, want the attachments-only shape", got)
			}
		})
	}

	t.Run("a missing outage is nil, which the handler renders as 404", func(t *testing.T) {
		got, err := NewCloudStatusDashboardService(&fakeDashboardRepo{}).
			IncidentDetail(context.Background(), base.ID, "asgardeo")
		if err != nil || got != nil {
			t.Errorf("got (%v, %v), want (nil, nil)", got, err)
		}
	})

	t.Run("a 32-hex id is accepted and converted for the query", func(t *testing.T) {
		repo := &fakeDashboardRepo{}
		_, _ = NewCloudStatusDashboardService(repo).
			IncidentDetail(context.Background(), "79adad2d1b45fa100bb3da47b04bcb46", "asgardeo")
		if repo.gotDetailID != "79adad2d-1b45-fa10-0bb3-da47b04bcb46" {
			t.Errorf("queried with %q, want the dashed uuid -- links made before "+
				"the cutover must still resolve", repo.gotDetailID)
		}
	})
}

// TestIncidentListStatusLabels pins the two labels the incident list uses.
//
// The ongoing one is "In Progress", matching ServiceNow and matching the
// detail endpoint. It is pinned here because live data cannot check it: every
// incident on the dev instance inside the window carries an end date, so a
// diff only ever exercises the Resolved branch.
func TestIncidentListStatusLabels(t *testing.T) {
	if got := incidentStatus(""); got != "In Progress" {
		t.Errorf("ongoing status = %q, want %q", got, "In Progress")
	}
	if got := incidentStatus("2026-09-22 09:26:53"); got != "Resolved" {
		t.Errorf("ended status = %q, want %q", got, "Resolved")
	}
	// The list and the detail must agree; they are separate code paths
	// rendering the same field.
	if incidentStatus("") != domain.IncidentDetailStatus("") ||
		incidentStatus("x") != domain.IncidentDetailStatus("x") {
		t.Error("the list and detail endpoints disagree on the status label")
	}
}

// TestIncidentMonthKeysOnEveryDayOfTheYear guards the rollover CodeRabbit
// found: seeding from today rather than the first of the month makes
// AddDate normalise a day the target month does not have, so on the 29th,
// 30th and 31st the response carried fewer than six keys and silently lost
// a month of incidents from the public page.
func TestIncidentMonthKeysOnEveryDayOfTheYear(t *testing.T) {
	repo := &fakeDashboardRepo{}
	svc := NewCloudStatusDashboardService(repo).(*cloudStatusDashboardService)

	day := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	for ; day.Year() == 2026; day = day.AddDate(0, 0, 1) {
		d := day
		svc.now = func() time.Time { return d }

		resp, err := svc.Incidents(context.Background(), "choreo")
		if err != nil {
			t.Fatalf("%s: %v", d.Format("2006-01-02"), err)
		}
		if len(resp) != incidentMonths {
			t.Fatalf("%s: got %d month keys, want %d -- keys: %v",
				d.Format("2006-01-02"), len(resp), incidentMonths, keysOf(resp))
		}
		// And they must be the six consecutive months ending with this one.
		want := map[string]bool{}
		first := time.Date(d.Year(), d.Month(), 1, 0, 0, 0, 0, time.UTC)
		for i := 0; i < incidentMonths; i++ {
			want[incidentMonthKey(first.AddDate(0, -i, 0))] = true
		}
		for k := range resp {
			if !want[k] {
				t.Fatalf("%s: unexpected key %q; want %v",
					d.Format("2006-01-02"), k, keysOf2(want))
			}
		}
	}
}

func keysOf(m domain.CloudStatusIncidentsResponse) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func keysOf2(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// TestIncidentDetailRejectsMalformedID covers the 500-vs-404 problem: an id
// that cannot name a row reached Postgres as $1::uuid and raised "invalid
// input syntax", which surfaced as a server error on a public endpoint.
func TestIncidentDetailRejectsMalformedID(t *testing.T) {
	for _, id := range []string{"abc", "not-a-uuid", "../../etc/passwd",
		"79adad2d1b45fa100bb3da47b04bcb4", "zzzzzzzz-1b45-fa10-0bb3-da47b04bcb46"} {
		repo := &fakeDashboardRepo{}
		got, err := NewCloudStatusDashboardService(repo).
			IncidentDetail(context.Background(), id, "asgardeo")
		if err != nil {
			t.Errorf("id %q returned an error (%v); a malformed id is a 404, not a 500", id, err)
		}
		if got != nil {
			t.Errorf("id %q returned %v, want nil so the handler renders 404", id, got)
		}
		if repo.gotDetailID != "" {
			t.Errorf("id %q reached the database as %q; it should be rejected first",
				id, repo.gotDetailID)
		}
	}

	// Both well-formed spellings must still get through.
	for _, id := range []string{
		"79adad2d-1b45-fa10-0bb3-da47b04bcb46",
		"79adad2d1b45fa100bb3da47b04bcb46",
	} {
		repo := &fakeDashboardRepo{}
		if _, err := NewCloudStatusDashboardService(repo).
			IncidentDetail(context.Background(), id, "asgardeo"); err != nil {
			t.Errorf("id %q: %v", id, err)
		}
		if repo.gotDetailID != "79adad2d-1b45-fa10-0bb3-da47b04bcb46" {
			t.Errorf("id %q queried as %q, want the dashed uuid", id, repo.gotDetailID)
		}
	}
}
