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
	"fmt"
	"log/slog"
	"time"
)

// pageSize is the page size used for /projects/search. entity-service's own
// code states maxLimit = 100 (entity-service/internal/service/user_service.go),
// but that does not match live behavior: confirmed via direct Postman
// testing against staging, limit: 50 returns 200 OK while limit: 51 returns
// 400 "Invalid request payload." — the real, live maximum is 50, not 100.
const pageSize = 50

// maxProjectPages and maxSearchPages cap how many pages a paging loop will
// request, independent of anything the upstream reports. Every loop also stops
// on an empty page, hasMore: false, or reaching "total", but those all trust
// the upstream: one that keeps returning non-empty pages with hasMore: true
// and total: 0 would otherwise keep a run going until the job timeout
// (CodeRabbit, PR #2545). Reaching a cap is an error, not a quiet stop, so
// skipped data is visible. 1,000 pages is 50,000 open projects (staging has
// about 1,900); 200 pages is 10,000 links or invoices for one project or
// opportunity.
const (
	maxProjectPages = 1000
	maxSearchPages  = 200
)

// Run performs one full ACP evaluation pass over every open project,
// paginating through /projects/search. The list comes from CSM (reader),
// which alone has each project's closure states and the notices already
// sent; details supplies the rest of each project before it is decided
// (NewSalesforceDetails reads it live from Salesforce). A details failure is
// a failure of that project, like any other. A single project's processProject
// failure is logged and recorded in Result.Failures, and the sweep
// continues — one project's problem must never block the rest.
//
// The open projects are all listed first (listOpenProjects) and only then
// evaluated. Evaluating page by page would skip projects: a suspension moves
// a project out of the Open list the pages are cut from, so the next offset
// would pass over as many projects as the previous page suspended (they would
// be evaluated a weekday late), and the shrinking total could end the paging
// early. A failure fetching a page is fatal for the whole run (there is no way
// to know what projects exist beyond it) and is returned as a non-nil error
// before any project is evaluated, so nothing is sent or written by that run.
//
// If projectID is non-empty, Run is scoped to exactly that one project — it
// fetches it directly via GetProject and never calls SearchProjects at all.
// This is what backs ACP_TEST_PROJECT_ID: a scoped run cannot accidentally touch
// every open project in an environment. A GetProject failure in this mode is
// fatal, the same as a page-fetch failure in the broad-sweep mode — there is
// nothing else to fall back to when the one requested project can't be
// fetched.
//
// excludedProjectIDs backs ACP_EXCLUDED_PROJECT_IDS: any project whose ID is a
// key in this set (with value true) is skipped entirely — not fetched in
// detail, not evaluated, not counted toward failures, just logged and
// counted in Result.ProjectsExcluded. This applies uniformly to both the
// broad sweep and the ACP_TEST_PROJECT_ID-scoped path: if projectID itself is
// excluded, GetProject is never called at all. Per explicit design
// direction (Sajith Ekanayake): this exists for deliberate business-driven
// exclusions, not as a workaround for data
// bugs — a project excluded here produces zero log signal about whatever
// might be wrong with it, which is the opposite of what you want for an
// actual bug. nil is equivalent to an empty set (nothing excluded).
func Run(ctx context.Context, reader sweepReader, details DetailSource, updater projectUpdater, ntf notifier, now time.Time, projectID string, excludedProjectIDs map[string]bool) (Result, error) {
	var result Result

	if projectID != "" {
		if skipExcluded(ctx, projectID, excludedProjectIDs, &result) {
			return result, nil
		}

		raw, err := reader.GetProject(ctx, projectID)
		if err != nil {
			return result, fmt.Errorf("sweep: get project %s: %w", projectID, err)
		}

		var proj project
		if err := json.Unmarshal(raw, &proj); err != nil {
			return result, fmt.Errorf("sweep: parse project %s: %w", projectID, err)
		}

		result.ProjectsEvaluated++
		evaluate(ctx, details, updater, ntf, now, proj, &result)

		return result, nil
	}

	projects, err := listOpenProjects(ctx, reader)
	if err != nil {
		return result, err
	}

	for _, proj := range projects {
		if skipExcluded(ctx, proj.ID, excludedProjectIDs, &result) {
			continue
		}

		result.ProjectsEvaluated++
		evaluate(ctx, details, updater, ntf, now, proj, &result)
	}

	return result, nil
}

// listOpenProjects pages through /projects/search for every Open project,
// writing nothing while it does, so the list is not moved by the run's own
// suspensions (see Run). A project returned on two pages (the upstream's
// order changing under a concurrent write) is kept once. It stops on an empty
// page, hasMore: false, or reaching the reported total, and errors when the
// page cap is reached.
func listOpenProjects(ctx context.Context, reader sweepReader) ([]project, error) {
	var projects []project
	seen := map[string]bool{}
	offset, pages := 0, 0
	for {
		reqBody, err := json.Marshal(searchProjectsRequest{
			Pagination:    pagination{Limit: pageSize, Offset: offset},
			ClosureStatus: "Open",
			SortBy:        "endDate",
			SortOrder:     "asc",
		})
		if err != nil {
			return nil, fmt.Errorf("sweep: build search request: %w", err)
		}

		raw, err := reader.SearchProjects(ctx, reqBody)
		if err != nil {
			return nil, fmt.Errorf("sweep: search projects at offset %d: %w", offset, err)
		}

		var page searchProjectsResponse
		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, fmt.Errorf("sweep: parse search response at offset %d: %w", offset, err)
		}

		listed := 0
		for _, proj := range page.Projects {
			listed++
			if proj.ID != "" && seen[proj.ID] {
				continue
			}
			seen[proj.ID] = true
			projects = append(projects, proj)
		}

		if len(page.Projects) == 0 {
			break
		}

		if !page.HasMore {
			break
		}

		if page.Total > 0 && offset+listed >= page.Total {
			slog.WarnContext(ctx, "sweep: pagination hit the Total bound while hasMore was still true",
				"total", page.Total, "projectsListed", len(projects))
			break
		}

		pages++
		if pages >= maxProjectPages {
			return nil, fmt.Errorf("sweep: project search still reports more pages after %d pages; stopping", maxProjectPages)
		}
		offset += pageSize
	}
	return projects, nil
}

// skipExcluded reports whether id is in excludedProjectIDs, logging and
// counting the skip in result if so. Shared by both of Run's paths (the
// ACP_TEST_PROJECT_ID-scoped early check and the broad-sweep loop) so the
// log line and counter update have exactly one definition.
// evaluate fills in proj's details and decides it, recording a failure of
// either step against the project.
func evaluate(ctx context.Context, details DetailSource, updater projectUpdater, ntf notifier, now time.Time, proj project, result *Result) {
	full, reader, err := details.forProject(ctx, proj)
	if err == nil {
		err = processProject(ctx, reader, updater, ntf, now, full)
	} else {
		err = fmt.Errorf("read project details: %w", err)
	}
	if err != nil {
		slog.ErrorContext(ctx, "processProject failed", "projectID", proj.ID, "err", err)
		result.Failures = append(result.Failures, ProjectFailure{ProjectID: proj.ID, Err: err})
	}
}

func skipExcluded(ctx context.Context, id string, excludedProjectIDs map[string]bool, result *Result) bool {
	if !excludedProjectIDs[id] {
		return false
	}
	slog.InfoContext(ctx, "project excluded from evaluation", "projectID", id)
	result.ProjectsExcluded++
	return true
}
