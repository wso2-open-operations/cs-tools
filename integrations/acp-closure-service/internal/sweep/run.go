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

// Run performs one full ACP evaluation pass over every open project,
// paginating through /projects/search. A single project's processProject
// failure is logged and recorded in Result.Failures, and the sweep
// continues — one project's problem must never block the rest. A failure
// fetching a page itself is fatal for the whole run (there is no way to
// know what projects exist beyond it) and is returned as a non-nil error;
// Result still reflects whatever was evaluated before that point.
//
// If projectID is non-empty, Run is scoped to exactly that one project — it
// fetches it directly via GetProject and never calls SearchProjects at all.
// This is what backs TEST_PROJECT_ID: a scoped run cannot accidentally touch
// every open project in an environment. A GetProject failure in this mode is
// fatal, the same as a page-fetch failure in the broad-sweep mode — there is
// nothing else to fall back to when the one requested project can't be
// fetched.
//
// excludedProjectIDs backs EXCLUDED_PROJECT_IDS: any project whose ID is a
// key in this set (with value true) is skipped entirely — not fetched in
// detail, not evaluated, not counted toward failures, just logged and
// counted in Result.ProjectsExcluded. This applies uniformly to both the
// broad sweep and the TEST_PROJECT_ID-scoped path: if projectID itself is
// excluded, GetProject is never called at all. Per explicit design
// direction (Sajith Ekanayake): this exists for deliberate business-driven
// exclusions, not as a workaround for data
// bugs — a project excluded here produces zero log signal about whatever
// might be wrong with it, which is the opposite of what you want for an
// actual bug. nil is equivalent to an empty set (nothing excluded).
func Run(ctx context.Context, reader sweepReader, updater projectUpdater, ntf notifier, now time.Time, projectID string, excludedProjectIDs map[string]bool) (Result, error) {
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
		if err := processProject(ctx, reader, updater, ntf, now, proj); err != nil {
			slog.ErrorContext(ctx, "processProject failed", "projectID", proj.ID, "err", err)
			result.Failures = append(result.Failures, ProjectFailure{ProjectID: proj.ID, Err: err})
		}

		return result, nil
	}

	offset := 0
	for {
		reqBody, err := json.Marshal(searchProjectsRequest{
			Pagination:    pagination{Limit: pageSize, Offset: offset},
			ClosureStatus: "Open",
			SortBy:        "endDate",
			SortOrder:     "asc",
		})
		if err != nil {
			return result, fmt.Errorf("sweep: build search request: %w", err)
		}

		raw, err := reader.SearchProjects(ctx, reqBody)
		if err != nil {
			return result, fmt.Errorf("sweep: search projects at offset %d: %w", offset, err)
		}

		var page searchProjectsResponse
		if err := json.Unmarshal(raw, &page); err != nil {
			return result, fmt.Errorf("sweep: parse search response at offset %d: %w", offset, err)
		}

		for _, proj := range page.Projects {
			if skipExcluded(ctx, proj.ID, excludedProjectIDs, &result) {
				continue
			}

			result.ProjectsEvaluated++
			if err := processProject(ctx, reader, updater, ntf, now, proj); err != nil {
				slog.ErrorContext(ctx, "processProject failed", "projectID", proj.ID, "err", err)
				result.Failures = append(result.Failures, ProjectFailure{ProjectID: proj.ID, Err: err})
			}
		}

		if len(page.Projects) == 0 {
			break
		}

		if !page.HasMore {
			break
		}

		if page.Total > 0 && result.ProjectsEvaluated+result.ProjectsExcluded >= page.Total {
			slog.WarnContext(ctx, "sweep: pagination hit the Total bound while hasMore was still true",
				"total", page.Total, "projectsEvaluated", result.ProjectsEvaluated, "projectsExcluded", result.ProjectsExcluded)
			break
		}

		offset += pageSize
	}

	return result, nil
}

// skipExcluded reports whether id is in excludedProjectIDs, logging and
// counting the skip in result if so. Shared by both of Run's paths (the
// TEST_PROJECT_ID-scoped early check and the broad-sweep loop) so the
// log line and counter update have exactly one definition.
func skipExcluded(ctx context.Context, id string, excludedProjectIDs map[string]bool, result *Result) bool {
	if !excludedProjectIDs[id] {
		return false
	}
	slog.InfoContext(ctx, "project excluded from evaluation", "projectID", id)
	result.ProjectsExcluded++
	return true
}
