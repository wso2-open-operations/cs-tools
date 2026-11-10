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
	"log/slog"
	"strings"
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

// ineligibleSubscriptionTypes are the project types ACP never evaluates, as
// the legacy process did not: evaluation, cloud support, cloud evaluation
// support and internal projects. The values are entity-service's
// SubscriptionType enum, which the project read carries.
var ineligibleSubscriptionTypes = map[string]bool{
	"evaluation_subscription":  true,
	"cloud_support":            true,
	"cloud_evaluation_support": true,
	"internal":                 true,
}

// skipIneligible reports whether proj is outside ACP's selection (see
// ineligibleSubscriptionTypes), counting it in result. It logs nothing per
// project: Run logs the count once.
func skipIneligible(proj project, result *Result) bool {
	// The flag is read from the fresh re-read, which is what is decided from.
	// An absent flag fails closed: an entity-service without it would
	// otherwise put every exempted project back into the run.
	if proj.SuspensionProcessOverridden == nil {
		result.ProjectsFilteredOverrideUnknown++
		return true
	}
	if *proj.SuspensionProcessOverridden {
		result.ProjectsFilteredOverridden++
		return true
	}
	if proj.SubscriptionType == nil || !ineligibleSubscriptionTypes[strings.ToLower(strings.TrimSpace(*proj.SubscriptionType))] {
		return false
	}
	result.ProjectsFiltered++
	return true
}

// skipByAccount reports whether proj is outside ACP's selection by its
// account, counting it in result: an account whose status is not "Customer"
// (or has none: the legacy selection required Customer, so a missing status
// fails closed), or one that is itself a partner. A partner account is
// skipped whole, before either cascade; this is not hasPrimaryPartner (a
// customer that has a partner), which the invoice cascade still handles. It
// runs after the details are read, which is where the account comes from.
// The status test applies only when the details source read one
// (salesforceDetails).
func skipByAccount(proj project, result *Result) bool {
	if proj.accountStatusRead {
		status := strings.TrimSpace(stringValue(proj.accountStatus))
		if status == "" {
			result.ProjectsFilteredNoStatus++
			return true
		}
		if !strings.EqualFold(status, "Customer") {
			result.ProjectsFilteredStatus++
			return true
		}
	}
	if proj.Account != nil && proj.Account.IsPartner != nil && *proj.Account.IsPartner {
		result.ProjectsFilteredPartner++
		return true
	}
	return false
}

// logScreening logs what screening did, once, counts only.
func logScreening(ctx context.Context, result Result, listed int) {
	if result.ScreenCandidates == nil {
		return
	}
	slog.InfoContext(ctx, "acp: screening done",
		"projectsListed", listed,
		"projectsScreenedOut", result.ProjectsScreenedOut,
		"candidatesByDate", result.ScreenCandidates[reasonDate],
		"candidatesByRecordedState", result.ScreenCandidates[reasonState],
		"candidatesByDueInvoice", result.ScreenCandidates[reasonInvoice],
		"candidatesByFailOpen", result.ScreenCandidates[reasonOpen])
}

// logFiltered logs how many projects the selection filter skipped, once.
func logFiltered(ctx context.Context, result Result) {
	if result.ProjectsFilteredOverrideUnknown > 0 {
		slog.WarnContext(ctx, "acp: projects skipped because entity-service did not return suspensionProcessOverridden for them: it must be deployed (with its migration) before this task runs; none of these were evaluated",
			"projectsSkipped", result.ProjectsFilteredOverrideUnknown)
	}
	n := result.ProjectsFilteredOverridden + result.ProjectsFilteredOverrideUnknown + result.ProjectsFiltered + result.ProjectsFilteredStatus + result.ProjectsFilteredNoStatus + result.ProjectsFilteredPartner
	if n > 0 {
		slog.InfoContext(ctx, "acp: projects skipped by the selection filter",
			"projectsFiltered", n,
			"byProjectType", result.ProjectsFiltered,
			"byOverrideFlag", result.ProjectsFilteredOverridden,
			"byMissingOverrideFlag", result.ProjectsFilteredOverrideUnknown,
			"byAccountStatus", result.ProjectsFilteredStatus,
			"byMissingAccountStatus", result.ProjectsFilteredNoStatus,
			"byPartnerAccount", result.ProjectsFilteredPartner)
	}
}

// Run performs one full ACP evaluation pass over every open project,
// paginating through /projects/search. The list comes from CSM (reader),
// which alone has each project's closure states and the notices already
// sent; details supplies the rest of each project before it is decided
// (NewSalesforceDetails reads it live from Salesforce). A details failure is
// a failure of that project, like any other. A single project's processProject
// failure is logged and recorded in Result.Failures, and the sweep
// continues — one project's problem must never block the rest. Two
// exceptions (see evaluate): a project whose own record is unusable is
// recorded in Result.Skipped and does not fail the run, and a CSM write
// failure every project would hit (CSM refusing or not taking writes) stops
// the sweep and is returned, since every later notice would go out without
// being recorded. Each listed project is read again from CSM just before it
// is decided (rereadProject).
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
// excluded, GetProject is never called at all. Per an explicit design
// decision: this exists for deliberate business-driven
// exclusions, not as a workaround for data
// bugs — a project excluded here produces zero log signal about whatever
// might be wrong with it, which is the opposite of what you want for an
// actual bug. nil is equivalent to an empty set (nothing excluded).
//
// limit (ACP_DAILY_LIMIT) caps how many projects a run acts on (see
// Result.ProjectsActed); 0 is no cap. The list is most overdue first, so the
// cap takes those. Once it is reached the remaining projects are not
// evaluated, which is not a failure. A dry run counts the same way, so it
// shows what a capped real run would do.
func Run(ctx context.Context, reader sweepReader, details DetailSource, updater projectUpdater, ntf notifier, now time.Time, projectID string, excludedProjectIDs map[string]bool, limit int) (Result, error) {
	return RunWith(ctx, reader, details, updater, ntf, now, projectID, excludedProjectIDs, Options{Limit: limit})
}

// Options are Run's optional settings.
type Options struct {
	// Limit is ACP_DAILY_LIMIT, 0 for no cap.
	Limit int
	// Screening skips, before any re-read or Salesforce call, the listed
	// projects CSM's copy of the data shows the live path cannot act on (see
	// screen). It never applies to a single-project run.
	Screening bool
	// Repair, when set, compares what the live reads show with CSM's copy
	// and posts an event for each record that drifted (see Repair). It never
	// changes a decision. nil is off.
	Repair *Repair
}

// RunWith is Run with Options.
func RunWith(ctx context.Context, reader sweepReader, details DetailSource, updater projectUpdater, ntf notifier, now time.Time, projectID string, excludedProjectIDs map[string]bool, opts Options) (Result, error) {
	limit := opts.Limit
	var result Result
	snap := &copySnapshot{}
	rr := newRepairRun(opts.Repair, snap)
	defer func() { rr.log(ctx) }()

	if prep, ok := details.(preparer); ok {
		if err := prep.prepare(ctx); err != nil {
			return result, fmt.Errorf("sweep: prepare details: %w", err)
		}
	}

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
		if skipIneligible(proj, &result) {
			logFiltered(ctx, result)
			return result, nil
		}
		// One project: a stop has nothing left to stop, and the failure is
		// already in result.
		_ = evaluate(ctx, reader, rr, details, updater, ntf, now, proj, &result)
		logFiltered(ctx, result)

		return result, nil
	}

	projects, err := listOpenProjects(ctx, reader)
	if err != nil {
		return result, err
	}

	defer func() { logFiltered(ctx, result) }()
	var screened map[string]string
	if opts.Screening {
		screened = screen(ctx, reader, projects, now, snap).candidates
		result.ScreenCandidates = map[string]int{}
		for _, reason := range screened {
			result.ScreenCandidates[reason]++
		}
		defer func() { logScreening(ctx, result, len(projects)) }()
	}
	for i, listed := range projects {
		if skipExcluded(ctx, listed.ID, excludedProjectIDs, &result) {
			continue
		}
		if screened != nil && screened[listed.ID] == "" {
			result.ProjectsScreenedOut++
			continue
		}

		if limit > 0 && result.ProjectsActed >= limit {
			for _, rest := range projects[i:] {
				if screened == nil || screened[rest.ID] != "" {
					result.NotEvaluatedByLimit++
				}
			}
			slog.InfoContext(ctx, "acp: daily limit reached, remaining projects not evaluated",
				"limit", limit, "projectsActed", result.ProjectsActed,
				"projectsEvaluated", result.ProjectsEvaluated, "projectsNotEvaluated", result.NotEvaluatedByLimit)
			break
		}

		result.ProjectsEvaluated++
		proj, err := rereadProject(ctx, reader, listed.ID)
		if err != nil {
			slog.ErrorContext(ctx, "processProject failed", "projectID", listed.ID, "err", err)
			result.Failures = append(result.Failures, ProjectFailure{ProjectID: listed.ID, Err: err})
			continue
		}
		if skipIneligible(proj, &result) {
			continue
		}
		if err := evaluate(ctx, reader, rr, details, updater, ntf, now, proj, &result); err != nil {
			return result, fmt.Errorf("sweep: stopped at project %s, %d project(s) not evaluated: %w", listed.ID, len(projects)-i-1, err)
		}
	}

	return result, nil
}

// rereadProject reads the listed project id again from CSM, right before it
// is decided. The list is read before any project is evaluated (see Run), so
// by the time a project is reached its listed closure states and
// suspensionProcessState can be as old as the run; every record write sends
// the whole suspensionProcessState rebuilt from what was read, so deciding
// from the listing could undo a change made since, such as entity-service's
// reopen. CSM reads its own database, so a re-read sees every committed
// write.
func rereadProject(ctx context.Context, reader sweepReader, id string) (project, error) {
	raw, err := reader.GetProject(ctx, id)
	if err != nil {
		return project{}, fmt.Errorf("re-read project from CSM: %w", err)
	}
	var proj project
	if err := json.Unmarshal(raw, &proj); err != nil {
		return project{}, fmt.Errorf("parse re-read project: %w", err)
	}
	if proj.ID == "" {
		proj.ID = id
	}
	return proj, nil
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

// evaluate fills in proj's details and decides it, recording a failure of
// either step against the project. A project whose own data is unusable
// (dataError) is recorded as skipped, not failed: a retry cannot fix it, and
// failing the run for it would retry and alert on every run until someone
// fixed the record. It returns the error only when CSM is refusing writes
// (writesFailingError), which stops the run: every later project would send
// its notice and then fail to record it, so the next run would send it
// again.
func evaluate(ctx context.Context, csmReader entityReader, rr *repairRun, details DetailSource, updater projectUpdater, ntf notifier, now time.Time, proj project, result *Result) error {
	full, reader, err := details.forProject(ctx, proj)
	if err == nil && skipByAccount(full, result) {
		return nil
	}
	if err == nil {
		// The repair compares what was just read live with CSM's copy
		// (proj is the fresh CSM read). It runs after the decision for the
		// invoices, never before it, and cannot change it.
		rr.project(ctx, proj, full)
		var acted bool
		acted, err = processProject(ctx, reader, updater, ntf, now, full)
		if acted {
			result.ProjectsActed++
		}
		rr.invoices(ctx, csmReader, reader)
	} else {
		err = fmt.Errorf("read project details: %w", err)
	}
	if err == nil {
		return nil
	}
	var bad *dataError
	if errors.As(err, &bad) {
		slog.WarnContext(ctx, "project skipped: its record needs fixing", "projectID", proj.ID, "err", err)
		result.Skipped = append(result.Skipped, ProjectFailure{ProjectID: proj.ID, Err: err})
		return nil
	}
	slog.ErrorContext(ctx, "processProject failed", "projectID", proj.ID, "err", err)
	result.Failures = append(result.Failures, ProjectFailure{ProjectID: proj.ID, Err: err})
	var refused *writesFailingError
	if errors.As(err, &refused) {
		return err
	}
	return nil
}

// skipExcluded reports whether id is in excludedProjectIDs, logging and
// counting the skip in result if so. Shared by both of Run's paths (the
// ACP_TEST_PROJECT_ID-scoped early check and the broad-sweep loop) so the
// log line and counter update have exactly one definition.
func skipExcluded(ctx context.Context, id string, excludedProjectIDs map[string]bool, result *Result) bool {
	if !excludedProjectIDs[id] {
		return false
	}
	slog.InfoContext(ctx, "project excluded from evaluation", "projectID", id)
	result.ProjectsExcluded++
	return true
}
