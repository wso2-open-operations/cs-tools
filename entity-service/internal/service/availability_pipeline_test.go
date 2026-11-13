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
	"strconv"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// Producer meets consumer, in one process.
//
// *** THE TWO HALVES WERE BUILT ELEVEN MONTHS APART AND HAVE NEVER MET. ***
// The dashboard endpoints were ported in #2081 and read service_availability;
// the sweep that writes it is new. Everything between them is agreement by
// convention: the period-type spellings, the subject the rows are keyed on,
// and the offering ids the weight map is indexed by. Each is a silent
// failure if it drifts -- a mismatched enum returns no rows, and a
// mismatched id contributes zero to a weighted average without erroring.
//
// This runs the real sweep, captures exactly what it would have written,
// feeds those rows to the real dashboard service, and checks the published
// figure. No database, so the SQL itself is still unproven -- but every
// contract either side of it is.

// --- fake write side -------------------------------------------------------

type captureAvailabilityRepo struct {
	subjects []repository.AvailabilitySubject
	outages  map[string][]repository.AvailabilityOutageRow
	written  []repository.ComputedAvailabilityRow
}

func (c *captureAvailabilityRepo) Subjects(context.Context) ([]repository.AvailabilitySubject, error) {
	return c.subjects, nil
}

func (c *captureAvailabilityRepo) OutagesFor(_ context.Context, subjectID string, begin, end time.Time) ([]repository.AvailabilityOutageRow, error) {
	var out []repository.AvailabilityOutageRow
	for _, o := range c.outages[subjectID] {
		// Mirror the SQL's overlap predicate, so the fake cannot be more
		// generous than the real query.
		if o.Begin.Before(end) && o.End.After(begin) {
			out = append(out, o)
		}
	}
	return out, nil
}

func (c *captureAvailabilityRepo) UpsertFixed(_ context.Context, rows []repository.ComputedAvailabilityRow) error {
	c.written = append(c.written, rows...)
	return nil
}

func (c *captureAvailabilityRepo) ReplaceRolling(_ context.Context, _, _ string, _ []string, rows []repository.ComputedAvailabilityRow) error {
	c.written = append(c.written, rows...)
	return nil
}

// --- fake read side, served from what the sweep wrote ----------------------

type replayDashboardRepo struct {
	repository.CloudStatusDashboardRepository
	rows     []repository.ComputedAvailabilityRow
	children map[string][]string // parent -> offering ids
}

// ParentAvailabilities is the only method /availabilities uses. It projects
// the captured rows the way parentAvailabilitiesSQL does: offerings under one
// parent, restricted to the four rolling windows.
func (r *replayDashboardRepo) ParentAvailabilities(_ context.Context, parentID string) ([]repository.ParentAvailabilityRow, error) {
	wanted := map[string]bool{}
	for _, w := range domain.AvailabilityWindows {
		wanted[w.Enum] = true
	}
	kids := map[string]bool{}
	for _, id := range r.children[parentID] {
		kids[id] = true
	}

	// DISTINCT ON (offering, type), newest period wins — the fake must not
	// be more forgiving than parentAvailabilitiesSQL, which carries that
	// clause precisely because the mirror holds two rows per pair and
	// summing both would publish roughly double.
	best := map[string]repository.ComputedAvailabilityRow{}
	for _, row := range r.rows {
		if row.ServiceOfferingID == nil || !kids[*row.ServiceOfferingID] || !wanted[row.Type] {
			continue
		}
		key := *row.ServiceOfferingID + "\x00" + row.Type
		if cur, seen := best[key]; seen && !row.End.After(cur.End) {
			continue
		}
		best[key] = row
	}

	out := make([]repository.ParentAvailabilityRow, 0, len(best))
	for _, row := range best {
		out = append(out, repository.ParentAvailabilityRow{
			ServiceOfferingID: *row.ServiceOfferingID,
			Window:            row.Type,
			Availability:      row.AbsoluteAvail,
		})
	}
	return out, nil
}

// --- the test --------------------------------------------------------------

func TestAvailability_SweepFeedsTheDashboard(t *testing.T) {
	// *** ASGARDEO, BECAUSE IT IS THE UNWEIGHTED PLAN. ***
	// The six other clouds combine their offerings with a hardcoded weight
	// map partitioned into regions summing to 100 each. Reconstructing one
	// of those partitions from the flat map is guesswork — a first attempt
	// greedily took offerings until the total PASSED 100 and published
	// 106.190%, and a corrected greedy pass lands on 98.75. Neither is a
	// real region, so neither is a fixture worth asserting against.
	//
	// Asgardeo dispatches to the plain `calculate` — an arithmetic mean over
	// whatever rows it is given — so the expected figure follows from the
	// inputs alone and holds for any set of offerings. That exercises every
	// link this test is for (period types, offering ids, the sweep's output
	// reaching the published number) without inventing a partition. The
	// weighted path has its own coverage in cloud_status_availability_test.
	plan, ok := seededCloudPlans["asgardeo"]
	if !ok {
		t.Fatal("no asgardeo availability plan")
	}
	if plan.Weighted() {
		t.Fatal("asgardeo is weighted now; this fixture assumes the plain mean")
	}
	region := plan.Regions[0]

	regionOfferings := []string{
		"aaaaaaaa-0000-4000-8000-000000000001",
		"aaaaaaaa-0000-4000-8000-000000000002",
		"aaaaaaaa-0000-4000-8000-000000000003",
		"aaaaaaaa-0000-4000-8000-000000000004",
	}

	const commitment = "11111111-1111-4111-8111-111111111111"
	write := &captureAvailabilityRepo{outages: map[string][]repository.AvailabilityOutageRow{}}
	for i := range regionOfferings {
		id := regionOfferings[i]
		write.subjects = append(write.subjects, repository.AvailabilitySubject{
			ServiceOfferingID:   &id,
			ServiceCommitmentID: commitment,
			TargetPercent:       100,
			Timezone:            "GMT",
		})
	}

	// One offering had a one-hour outage yesterday; the rest were clean.
	now := time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC)
	down := regionOfferings[0]
	write.outages[down] = []repository.AvailabilityOutageRow{{
		Begin: now.AddDate(0, 0, -1).Add(-2 * time.Hour),
		End:   now.AddDate(0, 0, -1).Add(-1 * time.Hour),
		Type:  "OUTAGE",
	}}

	svc, err := NewAvailabilityService(write, DefaultAvailabilityTimezone)
	if err != nil {
		t.Fatalf("construct sweep: %v", err)
	}
	res, err := svc.Sweep(context.Background(), now)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if res.Failed != 0 {
		t.Fatalf("sweep reported %d failed subjects", res.Failed)
	}
	if res.Subjects != len(regionOfferings) {
		t.Fatalf("sweep saw %d subjects, want %d", res.Subjects, len(regionOfferings))
	}
	t.Logf("sweep wrote %d rows for %d subjects", res.Rows, res.Subjects)

	// *** THE CONTRACT CHECK. *** Every window the dashboard asks for must
	// be among what the sweep wrote. A missing one returns no rows and the
	// published figure silently becomes the unweighted default.
	byType := map[string]int{}
	for _, row := range write.written {
		byType[row.Type]++
	}
	for _, w := range domain.AvailabilityWindows {
		if byType[w.Enum] == 0 {
			t.Errorf("sweep wrote no %s rows, but /availabilities queries that window", w.Enum)
		}
	}

	read := &replayDashboardRepo{
		rows:     write.written,
		children: map[string][]string{region.ParentID: regionOfferings},
	}
	dash := NewCloudStatusDashboardService(read)

	resp, err := dash.Availabilities(context.Background(), "asgardeo")
	if err != nil {
		t.Fatalf("Availabilities: %v", err)
	}

	windows, ok := resp[region.Key]
	if !ok {
		ks := make([]string, 0, len(resp))
		for k := range resp {
			ks = append(ks, k)
		}
		t.Fatalf("response has no %q region; keys are %v", region.Key, ks)
	}
	if len(windows) != len(domain.AvailabilityWindows) {
		t.Fatalf("got %d windows, want %d", len(windows), len(domain.AvailabilityWindows))
	}

	// Hand-computed from the one-hour outage above: (100*(N-3600)/N + 300)/4.
	expected := map[string]string{
		"Last 7 days":    "99.851",
		"Last 30 days":   "99.965",
		"Last 90 days":   "99.988",
		"Last 12 months": "99.997",
	}

	for _, w := range windows {
		if w.Availability.Value == "" {
			t.Errorf("%s: empty availability", w.Duration)
			continue
		}
		// *** NaN IS THE FAILURE MODE THAT MATTERS. *** ServiceNow's own
		// script produces it whenever a weight key does not match an
		// offering id -- availability * undefined -- and it reaches the
		// status page as the string "NaN". If the sweep ever wrote an id
		// the weight map does not carry, this is where it shows up.
		if w.Availability.Value == "NaN" || w.Availability.Value == "0.000" {
			t.Errorf("%s: availability is %q — the offering ids the sweep wrote "+
				"do not line up with the weight map keys", w.Duration, w.Availability.Value)
		}
		// *** OVER 100% IS A REAL, VISIBLE FAILURE. *** It means the
		// offerings Postgres groups under a parent are not the partition the
		// weight map assumes — either a duplicate row counted twice, or an
		// offering whose weight belongs to another region. The number goes
		// to the customer-facing status page either way.
		if v, err := strconv.ParseFloat(w.Availability.Value, 64); err == nil && v > 100 {
			t.Errorf("%s: availability is %v%% — above 100, so the offerings under "+
				"the parent are not the weight map's region partition", w.Duration, v)
		}
		t.Logf("  %-14s %s", w.Duration, w.Availability.Value)

		// *** AND THE EXACT FIGURE, HAND-COMPUTED. *** One of four offerings
		// was down for an hour; the other three were clean. Asgardeo takes a
		// plain mean, so for a window of N seconds the published value is
		//     (100*(N-3600)/N + 300) / 4
		// rounded to three places. Checking the number rather than merely
		// "not NaN" is what makes this a test of the pipeline instead of a
		// test that it ran.
		if want, ok := expected[w.Duration]; ok && w.Availability.Value != want {
			t.Errorf("%s: published %s, hand arithmetic says %s",
				w.Duration, w.Availability.Value, want)
		}
	}
}

// service_availability has no CI column, so a commitment held by a CI must
// fail visibly rather than write rows with no subject; offerings beside it
// are still written.
func TestAvailabilitySweep_CIOnlySubjectFails(t *testing.T) {
	str := func(s string) *string { return &s }
	repo := &captureAvailabilityRepo{subjects: []repository.AvailabilitySubject{
		{ServiceOfferingID: str("o-1"), ServiceCommitmentID: "c1", TargetPercent: 100},
		{CmdbCiID: str("ci-1"), ServiceCommitmentID: "c2", TargetPercent: 100},
	}}
	svc, err := NewAvailabilityService(repo, "UTC")
	if err != nil {
		t.Fatal(err)
	}
	res, err := svc.Sweep(context.Background(), time.Date(2026, 10, 2, 3, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if res.Failed != 1 {
		t.Errorf("failed = %d, want 1 (the CI-only subject)", res.Failed)
	}
	for _, r := range repo.written {
		if r.ServiceOfferingID == nil {
			t.Fatalf("a row was written with no offering: %+v", r)
		}
	}
	if len(repo.written) == 0 {
		t.Error("the offering subject beside it was not written")
	}
}

// *** A COMMITMENT ON ANY SCHEDULE OTHER THAN "24 x 7" MUST FAIL, NOT GUESS. ***
// Its spans are not synced, so computing it as always-on would publish a
// wrong figure silently. The sweep counts it as failed and still writes the
// subjects it can compute.
func TestAvailabilitySweep_ScheduleGuard(t *testing.T) {
	str := func(s string) *string { return &s }
	sched := str("38fa64ed-c0a8-0164-00f4-a5724b0434b8")
	repo := &captureAvailabilityRepo{subjects: []repository.AvailabilitySubject{
		{ServiceOfferingID: str("o-null"), ServiceCommitmentID: "c1", TargetPercent: 100},
		{ServiceOfferingID: str("o-24x7"), ServiceCommitmentID: "c2", TargetPercent: 100, ScheduleID: sched, ScheduleName: str("24 x 7")},
		{ServiceOfferingID: str("o-24X7"), ServiceCommitmentID: "c3", TargetPercent: 100, ScheduleID: sched, ScheduleName: str("24X7")},
		{ServiceOfferingID: str("o-hours"), ServiceCommitmentID: "c4", TargetPercent: 100, ScheduleID: str("other"), ScheduleName: str("8-5 weekdays")},
		{ServiceOfferingID: str("o-gone"), ServiceCommitmentID: "c5", TargetPercent: 100, ScheduleID: str("missing")},
	}}
	svc, err := NewAvailabilityService(repo, "UTC")
	if err != nil {
		t.Fatal(err)
	}
	res, err := svc.Sweep(context.Background(), time.Date(2026, 10, 2, 3, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if res.Failed != 2 {
		t.Errorf("failed = %d, want 2 (the business-hours schedule and the unresolvable one)", res.Failed)
	}
	wrote := map[string]bool{}
	for _, r := range repo.written {
		wrote[*r.ServiceOfferingID] = true
	}
	for _, ok := range []string{"o-null", "o-24x7", "o-24X7"} {
		if !wrote[ok] {
			t.Errorf("%s: no rows written, want them computed as 24x7", ok)
		}
	}
	for _, bad := range []string{"o-hours", "o-gone"} {
		if wrote[bad] {
			t.Errorf("%s: rows written for a schedule that is not modelled", bad)
		}
	}
}
