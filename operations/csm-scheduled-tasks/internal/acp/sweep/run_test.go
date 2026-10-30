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

package sweep

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestRun_SinglePageEvaluatesAllProjects(t *testing.T) {
	reader := &mockEntityReader{
		searchProjectsFn: func(ctx context.Context, body []byte) ([]byte, error) {
			return []byte(`{
				"projects": [
					{"id": "p1", "endDate": null},
					{"id": "p2", "endDate": null}
				],
				"total": 2, "limit": 50, "offset": 0, "hasMore": false
			}`), nil
		},
	}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	result, err := Run(context.Background(), reader, CSMDetails(reader), updater, ntf, time.Now(), "", nil)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if result.ProjectsEvaluated != 2 {
		t.Errorf("ProjectsEvaluated = %d, want 2", result.ProjectsEvaluated)
	}
	if len(result.Failures) != 0 {
		t.Errorf("Failures = %d, want 0", len(result.Failures))
	}
	if len(reader.searchProjectsCalls) != 1 {
		t.Errorf("SearchProjects calls = %d, want 1", len(reader.searchProjectsCalls))
	}
}

// TestRun_MultiPagePaginatesUntilHasMoreFalse verifies offset increments by
// the page size across pages, and stops once hasMore is false.
func TestRun_MultiPagePaginatesUntilHasMoreFalse(t *testing.T) {
	var gotOffsets []int
	reader := &mockEntityReader{
		searchProjectsFn: func(ctx context.Context, body []byte) ([]byte, error) {
			var req searchProjectsRequest
			if err := json.Unmarshal(body, &req); err != nil {
				t.Fatalf("parse search request: %v", err)
			}
			gotOffsets = append(gotOffsets, req.Pagination.Offset)

			if req.Pagination.Offset == 0 {
				return []byte(`{
					"projects": [{"id": "p1", "endDate": null}],
					"total": 2, "limit": 50, "offset": 0, "hasMore": true
				}`), nil
			}
			return []byte(`{
				"projects": [{"id": "p2", "endDate": null}],
				"total": 2, "limit": 50, "offset": 50, "hasMore": false
			}`), nil
		},
	}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	result, err := Run(context.Background(), reader, CSMDetails(reader), updater, ntf, time.Now(), "", nil)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if result.ProjectsEvaluated != 2 {
		t.Errorf("ProjectsEvaluated = %d, want 2", result.ProjectsEvaluated)
	}
	if len(gotOffsets) != 2 || gotOffsets[0] != 0 || gotOffsets[1] != 50 {
		t.Errorf("offsets = %v, want [0 50]", gotOffsets)
	}
}

// TestRun_StopsWhenPageReturnsZeroProjects verifies the loop terminates on an
// empty page even if hasMore claims true — hasMore is a field the PR
// documents as undocumented upstream, so it must never be the sole loop
// condition. The mock fails the test itself if SearchProjects is called more
// than once, so an unbounded loop fails fast here instead of hanging.
func TestRun_StopsWhenPageReturnsZeroProjects(t *testing.T) {
	calls := 0
	reader := &mockEntityReader{
		searchProjectsFn: func(ctx context.Context, body []byte) ([]byte, error) {
			calls++
			if calls > 1 {
				t.Fatal("SearchProjects called more than once; loop did not stop on an empty page")
			}
			return []byte(`{
				"projects": [],
				"total": 1000000, "limit": 50, "offset": 0, "hasMore": true
			}`), nil
		},
	}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	result, err := Run(context.Background(), reader, CSMDetails(reader), updater, ntf, time.Now(), "", nil)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if result.ProjectsEvaluated != 0 {
		t.Errorf("ProjectsEvaluated = %d, want 0", result.ProjectsEvaluated)
	}
	if calls != 1 {
		t.Errorf("SearchProjects calls = %d, want 1", calls)
	}
}

// TestRun_BoundsIterationsByTotalWhenHasMoreNeverFalse verifies the loop
// stops using the already-parsed Total field even when hasMore never
// legitimately turns false. total=100 with a 50-project page should bound
// the loop to 2 pages. The mock fails the test itself if SearchProjects is
// called a third time, so an unbounded loop fails fast here instead of
// hanging.
func TestRun_BoundsIterationsByTotalWhenHasMoreNeverFalse(t *testing.T) {
	calls := 0
	reader := &mockEntityReader{
		searchProjectsFn: func(ctx context.Context, body []byte) ([]byte, error) {
			calls++
			if calls > 2 {
				t.Fatal("SearchProjects called more than twice; loop did not bound on Total")
			}
			projects := make([]byte, 0)
			for i := 0; i < 50; i++ {
				if i > 0 {
					projects = append(projects, ',')
				}
				projects = append(projects, []byte(fmt.Sprintf(`{"id": "p%d-%d", "endDate": null}`, calls, i))...)
			}
			return []byte(`{
				"projects": [` + string(projects) + `],
				"total": 100, "limit": 50, "offset": 0, "hasMore": true
			}`), nil
		},
	}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	result, err := Run(context.Background(), reader, CSMDetails(reader), updater, ntf, time.Now(), "", nil)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if result.ProjectsEvaluated != 100 {
		t.Errorf("ProjectsEvaluated = %d, want 100", result.ProjectsEvaluated)
	}
	if calls != 2 {
		t.Errorf("SearchProjects calls = %d, want 2", calls)
	}
}

// TestRun_OneProjectFailureDoesNotBlockTheRest verifies the two-tier failure
// design: a single project's processProject failure (malformed
// suspensionProcessState, here) is recorded in Result.Failures and the
// sweep continues to the remaining projects in the same page — it must not
// abort the whole run.
func TestRun_OneProjectFailureDoesNotBlockTheRest(t *testing.T) {
	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	firingEndDate := now.AddDate(0, 0, 89).Format(time.RFC3339) // fires the 90-day window

	reader := &mockEntityReader{
		searchProjectsFn: func(ctx context.Context, body []byte) ([]byte, error) {
			return []byte(`{
				"projects": [
					{"id": "p1", "endDate": null},
					{"id": "p2", "endDate": "` + firingEndDate + `", "suspensionProcessState": "not-an-object"},
					{"id": "p3", "endDate": null}
				],
				"total": 3, "limit": 50, "offset": 0, "hasMore": false
			}`), nil
		},
	}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	result, err := Run(context.Background(), reader, CSMDetails(reader), updater, ntf, now, "", nil)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if result.ProjectsEvaluated != 3 {
		t.Errorf("ProjectsEvaluated = %d, want 3", result.ProjectsEvaluated)
	}
	if len(result.Failures) != 1 {
		t.Fatalf("Failures = %d, want 1", len(result.Failures))
	}
	if result.Failures[0].ProjectID != "p2" {
		t.Errorf("failed ProjectID = %q, want %q", result.Failures[0].ProjectID, "p2")
	}
}

// TestRun_PageFetchFailureIsFatal verifies the other tier: a failure
// fetching a page itself stops the run and returns a non-nil error, rather
// than being folded into Result.Failures like a per-project failure.
func TestRun_PageFetchFailureIsFatal(t *testing.T) {
	reader := &mockEntityReader{
		searchProjectsFn: func(ctx context.Context, body []byte) ([]byte, error) {
			return nil, errors.New("connection refused")
		},
	}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	_, err := Run(context.Background(), reader, CSMDetails(reader), updater, ntf, time.Now(), "", nil)
	if err == nil {
		t.Fatal("Run() error = nil, want non-nil")
	}
}

// TestRun_ScopedToProjectIDFetchesOnlyThatProject verifies the
// ACP_TEST_PROJECT_ID scoping: when a non-empty projectID is passed, Run fetches
// that one project directly via GetProject and never calls the broad
// SearchProjects sweep at all — proving a scoped run can't accidentally
// touch every open project in the environment.
func TestRun_ScopedToProjectIDFetchesOnlyThatProject(t *testing.T) {
	const testProjectID = "e3e87599-1bc7-6650-182c-0dc5604bcb68"

	var gotID string
	reader := &mockEntityReader{
		getProjectFn: func(ctx context.Context, id string) ([]byte, error) {
			gotID = id
			return []byte(`{"id": "` + testProjectID + `", "account": {"id": "f213fdd1-1b4b-a650-a002-c9d3604bcbac"}, "endDate": null}`), nil
		},
		searchProjectsFn: func(ctx context.Context, body []byte) ([]byte, error) {
			t.Fatal("SearchProjects should not be called when scoped to a single project")
			return nil, nil
		},
	}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	result, err := Run(context.Background(), reader, CSMDetails(reader), updater, ntf, time.Now(), testProjectID, nil)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if gotID != testProjectID {
		t.Errorf("GetProject called with id = %q, want %q", gotID, testProjectID)
	}
	if result.ProjectsEvaluated != 1 {
		t.Errorf("ProjectsEvaluated = %d, want 1", result.ProjectsEvaluated)
	}
	if len(reader.searchProjectsCalls) != 0 {
		t.Errorf("SearchProjects calls = %d, want 0", len(reader.searchProjectsCalls))
	}
}

// TestRun_ScopedProjectFetchFailureIsFatal mirrors
// TestRun_PageFetchFailureIsFatal for the scoped path: if GetProject itself
// fails, that's fatal for the run, not a per-project soft failure — there's
// nothing else to fall back to when the one requested project can't be
// fetched at all.
func TestRun_ScopedProjectFetchFailureIsFatal(t *testing.T) {
	reader := &mockEntityReader{
		getProjectFn: func(ctx context.Context, id string) ([]byte, error) {
			return nil, errors.New("not found")
		},
	}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}

	_, err := Run(context.Background(), reader, CSMDetails(reader), updater, ntf, time.Now(), "e3e87599-1bc7-6650-182c-0dc5604bcb68", nil)
	if err == nil {
		t.Fatal("Run() error = nil, want non-nil")
	}
}

// TestRun_SkipsExcludedProjectsInBroadSweep verifies ACP_EXCLUDED_PROJECT_IDS:
// a project whose ID is in excludedProjectIDs is skipped entirely in the
// broad sweep — not evaluated, not counted in ProjectsEvaluated, no
// processProject call at all — while the other projects in the same page
// are unaffected.
func TestRun_SkipsExcludedProjectsInBroadSweep(t *testing.T) {
	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	firingEndDate := now.AddDate(0, 0, 89).Format(time.RFC3339) // fires the 90-day window

	reader := &mockEntityReader{
		searchProjectsFn: func(ctx context.Context, body []byte) ([]byte, error) {
			// p2 (the excluded one) is given a firing end date and an
			// invalid suspensionProcessState — if processProject ever
			// actually ran on it, that would produce a real, visible
			// failure. Its endDate being null instead would make
			// processProject a silent no-op either way, so the
			// zero-failures assertion below couldn't distinguish
			// "never evaluated" from "evaluated but harmless" (per
			// CodeRabbit).
			return []byte(`{
				"projects": [
					{"id": "p1", "endDate": null},
					{"id": "p2", "endDate": "` + firingEndDate + `", "suspensionProcessState": "not-an-object"},
					{"id": "p3", "endDate": null}
				],
				"total": 3, "limit": 50, "offset": 0, "hasMore": false
			}`), nil
		},
	}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}
	excluded := map[string]bool{"p2": true}

	result, err := Run(context.Background(), reader, CSMDetails(reader), updater, ntf, now, "", excluded)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if result.ProjectsEvaluated != 2 {
		t.Errorf("ProjectsEvaluated = %d, want 2 (p2 excluded)", result.ProjectsEvaluated)
	}
	if result.ProjectsExcluded != 1 {
		t.Errorf("ProjectsExcluded = %d, want 1", result.ProjectsExcluded)
	}
	if len(result.Failures) != 0 {
		t.Errorf("Failures = %d, want 0", len(result.Failures))
	}
}

// TestRun_ScopedToProjectIDSkipsExcludedProjectWithoutFetching verifies the
// ACP_TEST_PROJECT_ID + ACP_EXCLUDED_PROJECT_IDS interaction: if the scoped
// projectID is itself excluded, Run must not call GetProject at all — per
// Sajith Ekanayake's explicit requirement, excluded projects must not even
// have their details fetched, not just be skipped after fetching.
func TestRun_ScopedToProjectIDSkipsExcludedProjectWithoutFetching(t *testing.T) {
	const excludedID = "e3e87599-1bc7-6650-182c-0dc5604bcb68"

	reader := &mockEntityReader{
		getProjectFn: func(ctx context.Context, id string) ([]byte, error) {
			t.Fatal("GetProject should not be called for an excluded projectID")
			return nil, nil
		},
	}
	updater := &mockProjectUpdater{}
	ntf := &mockNotifier{}
	excluded := map[string]bool{excludedID: true}

	result, err := Run(context.Background(), reader, CSMDetails(reader), updater, ntf, time.Now(), excludedID, excluded)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if result.ProjectsEvaluated != 0 {
		t.Errorf("ProjectsEvaluated = %d, want 0", result.ProjectsEvaluated)
	}
	if result.ProjectsExcluded != 1 {
		t.Errorf("ProjectsExcluded = %d, want 1", result.ProjectsExcluded)
	}
}

// TestRun_StopsAtMaxProjectPagesWhenUpstreamNeverEnds: with total: 0 the
// "total" bound can't apply, so an upstream that always answers hasMore: true
// with a non-empty page must still be stopped by the page cap, as an error.
func TestRun_StopsAtMaxProjectPagesWhenUpstreamNeverEnds(t *testing.T) {
	calls := 0
	reader := &mockEntityReader{
		searchProjectsFn: func(ctx context.Context, body []byte) ([]byte, error) {
			calls++
			if calls > maxProjectPages+5 {
				return []byte(`{"projects":[],"hasMore":false}`), nil // test safety stop
			}
			return []byte(`{"projects":[{"id":"p","endDate":null}],"total":0,"hasMore":true}`), nil
		},
	}

	_, err := Run(context.Background(), reader, CSMDetails(reader), &mockProjectUpdater{}, &mockNotifier{}, time.Now(), "", nil)
	if err == nil {
		t.Fatal("Run() error = nil, want an error when the page cap is reached")
	}
	if calls != maxProjectPages {
		t.Errorf("SearchProjects called %d times, want %d (the page cap)", calls, maxProjectPages)
	}
}

// suspendingDetails stands in for the run's own suspensions: every project it
// is asked about leaves the Open list the search pages are cut from, as a
// suspended project does. It records each project it saw and fails it, so no
// write is attempted.
type suspendingDetails struct {
	open *[]string
	seen map[string]int
}

func (d suspendingDetails) forProject(ctx context.Context, proj project) (project, entityReader, error) {
	d.seen[proj.ID]++
	for i, id := range *d.open {
		if id == proj.ID {
			*d.open = append((*d.open)[:i], (*d.open)[i+1:]...)
			break
		}
	}
	return proj, nil, errors.New("stand-in: suspended")
}

// TestRun_SuspensionsDoNotMakeTheSweepSkipProjects: the search lists Open
// projects only, so a project suspended by the run leaves the list. Paging
// and evaluating together would then skip one project per suspension on the
// next page, and stop early on the shrinking total. Every project open when
// the run started must be evaluated exactly once.
func TestRun_SuspensionsDoNotMakeTheSweepSkipProjects(t *testing.T) {
	const n = 3*pageSize + 7
	open := make([]string, n)
	for i := range open {
		open[i] = fmt.Sprintf("p%03d", i)
	}
	reader := &mockEntityReader{
		searchProjectsFn: func(ctx context.Context, body []byte) ([]byte, error) {
			var req searchProjectsRequest
			if err := json.Unmarshal(body, &req); err != nil {
				t.Fatalf("parse search request: %v", err)
			}
			from := min(req.Pagination.Offset, len(open))
			to := min(from+req.Pagination.Limit, len(open))
			var rows []map[string]any
			for _, id := range open[from:to] {
				rows = append(rows, map[string]any{"id": id, "endDate": nil})
			}
			return json.Marshal(map[string]any{
				"projects": rows, "total": len(open), "limit": req.Pagination.Limit,
				"offset": req.Pagination.Offset, "hasMore": to < len(open),
			})
		},
	}
	details := suspendingDetails{open: &open, seen: map[string]int{}}

	result, err := Run(context.Background(), reader, details, &mockProjectUpdater{}, &mockNotifier{}, time.Now(), "", nil)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if result.ProjectsEvaluated != n {
		t.Errorf("ProjectsEvaluated = %d, want %d", result.ProjectsEvaluated, n)
	}
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("p%03d", i)
		if details.seen[id] != 1 {
			t.Errorf("project %s evaluated %d times, want 1", id, details.seen[id])
		}
	}
}

// TestRun_APageFetchFailureEvaluatesNothing: the list is complete before any
// project is evaluated, so a failure on a later page sends and writes nothing.
func TestRun_APageFetchFailureEvaluatesNothing(t *testing.T) {
	calls := 0
	reader := &mockEntityReader{
		searchProjectsFn: func(ctx context.Context, body []byte) ([]byte, error) {
			calls++
			if calls == 2 {
				return nil, errors.New("upstream down")
			}
			return []byte(`{"projects":[{"id":"p1","endDate":null}],"total":2,"hasMore":true}`), nil
		},
	}
	details := suspendingDetails{open: &[]string{}, seen: map[string]int{}}

	result, err := Run(context.Background(), reader, details, &mockProjectUpdater{}, &mockNotifier{}, time.Now(), "", nil)
	if err == nil {
		t.Fatal("Run() error = nil, want the page fetch failure")
	}
	if result.ProjectsEvaluated != 0 || len(details.seen) != 0 {
		t.Errorf("evaluated %d projects (%v) before the list was complete, want none", result.ProjectsEvaluated, details.seen)
	}
}
