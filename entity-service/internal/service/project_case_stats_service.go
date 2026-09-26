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
	"fmt"
	"math"

	"golang.org/x/sync/errgroup"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// case_state_enum labels (migration 000018) this aggregation classifies by.
// Spelled as the raw enum labels because that is what caseLikeStateColumn
// returns, and what GET /projects/{id}/metadata's caseStates already exposes
// in Postgres mode -- so a caller can join stateCount to that list directly.
const (
	caseStateClosed           = "CLOSED"
	caseStateAwaitingInfo     = "AWAITING_INFO"
	caseStateSolutionProposed = "SOLUTION_PROPOSED"
)

// caseStatsResolvedStates are the states counted as resolved.
//
// SOLUTION_PROPOSED appears here AND is counted as active/outstanding below:
// that overlap is the ServiceNow implementation's
// (CASE_RESOLVED_STATE_VALUES and CASE_ACTIVE_STATE_VALUES both contain it),
// and is reproduced rather than corrected so both data sources report the
// same numbers during the migration.
var caseStatsResolvedStates = []string{caseStateClosed, caseStateSolutionProposed}

// projectCaseStatsService is the Postgres-backed ProjectCaseStatsService.
//
// It reproduces ServiceNow's ProjectStatsUtils.getProjectScopeCaseStats
// field for field, including four behaviours that read as oversights but are
// load-bearing for parity (each is also noted at the code that implements
// it):
//
//  1. activeCount and outstandingCount are the same number -- ServiceNow
//     computes both from one list of states, commented "ACTIVE ===
//     OUTSTANDING per business requirement".
//  2. changeRate.resolvedEngagements compares cases by CREATION date while
//     counting closures, and ignores the caseTypes filter.
//  3. averageResponseTime ignores both the caseTypes and createdBy filters.
//  4. changeRate.averageResponseTime is always 0 -- ServiceNow's
//     previous-window computation is commented out at the source.
//
// Two ServiceNow behaviours cannot be reproduced and are documented at their
// call sites rather than approximated: the per-project allowed-severity and
// allowed-case-type narrowing (both derived from the project type, with no
// Postgres feature-entitlement table behind them), and severity on non-case
// types (only the "case" extension table has a severity column).
type projectCaseStatsService struct {
	repo    repository.ProjectCaseStatsRepository
	refRepo repository.ReferenceDataRepository
	access  AccessService
}

// NewProjectCaseStatsService constructs a Postgres-backed ProjectCaseStatsService.
func NewProjectCaseStatsService(
	repo repository.ProjectCaseStatsRepository,
	refRepo repository.ReferenceDataRepository,
	access AccessService,
) ProjectCaseStatsService {
	return &projectCaseStatsService{repo: repo, refRepo: refRepo, access: access}
}

// GetProjectCaseStats implements ProjectCaseStatsService.
func (s *projectCaseStatsService) GetProjectCaseStats(
	ctx context.Context,
	projectID string,
	req domain.ProjectCaseStatsRequest,
) (domain.ProjectCaseStatsResponse, error) {
	if err := validateUUIDs("id", []string{projectID}); err != nil {
		return domain.ProjectCaseStatsResponse{}, err
	}

	// Normalize and validate exactly as the ServiceNow implementation does
	// (snProjectStatsService.GetProjectCaseStats), so the same caseTypes
	// value is accepted or rejected identically in both modes.
	types := make([]string, len(req.CaseTypes))
	for i, t := range req.CaseTypes {
		types[i] = normalizeCaseType(t)
		if !validCaseType[types[i]] {
			return domain.ProjectCaseStatsResponse{}, &apierror.ValidationError{Msg: "caseTypes contains invalid value: " + t}
		}
	}

	// Scope before existence: the id is caller-controlled, so a project the
	// caller may not see must be indistinguishable from one that is not there.
	if err := authorizeProject(ctx, s.access, projectID); err != nil {
		return domain.ProjectCaseStatsResponse{}, err
	}

	found, _, err := s.refRepo.GetProjectByID(ctx, projectID)
	if err != nil {
		return domain.ProjectCaseStatsResponse{}, err
	}
	if !found {
		return domain.ProjectCaseStatsResponse{}, &apierror.NotFoundError{Msg: "project not found"}
	}

	createdBy, err := resolveCreatedByFilter(ctx, req.CreatedBy)
	if err != nil {
		return domain.ProjectCaseStatsResponse{}, err
	}

	filter := repository.ProjectCaseStatsFilter{
		ProjectID: projectID,
		Types:     types,
		CreatedBy: createdBy,
	}

	// The enum lookup and the six aggregations below share only the filter,
	// so they run concurrently rather than as seven serial round trips --
	// the same errgroup pattern SearchCases and SearchCaseComments already
	// use for their COUNT/SELECT pair. Each goroutine writes its own
	// variable; the response is assembled afterwards, in a fixed order, so
	// the result is identical to running them in sequence.
	var (
		labels          map[string][]string
		stateSeverity   []repository.StateSeverityCount
		engagementTypes []repository.StateEngagementTypeCount
		currentMonth    int
		pastThirtyDays  int
		current         int
		previous        int
		avgSeconds      float64
		slaCount        int
		caseTypeCounts  map[string]int
	)

	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		var err error
		labels, err = s.refRepo.EnumLabels(gctx, []string{
			caseStateEnumType, caseSeverityEnumType, engagementTypeEnumType,
		})
		if err != nil {
			return fmt.Errorf("project case stats: %w", err)
		}
		return nil
	})
	g.Go(func() error {
		var err error
		stateSeverity, err = s.repo.StateSeverityCounts(gctx, filter)
		return err
	})
	g.Go(func() error {
		var err error
		engagementTypes, err = s.repo.StateEngagementTypeCounts(gctx, filter)
		return err
	})
	g.Go(func() error {
		var err error
		currentMonth, pastThirtyDays, err = s.repo.ResolvedBuckets(gctx, filter, caseStatsResolvedStates)
		return err
	})
	g.Go(func() error {
		var err error
		current, previous, err = s.repo.ClosedByCreatedWindow(gctx, filter, caseStateClosed)
		return err
	})
	g.Go(func() error {
		var err error
		avgSeconds, slaCount, err = s.repo.AverageResponseSeconds(gctx, projectID)
		return err
	})
	g.Go(func() error {
		var err error
		caseTypeCounts, err = s.repo.CaseTypeCounts(gctx, filter)
		return err
	})

	if err := g.Wait(); err != nil {
		return domain.ProjectCaseStatsResponse{}, err
	}

	// Every bucket is seeded from the enum's full label set at zero, so a
	// state or severity with no cases is still present in the response --
	// the ServiceNow implementation does the same via _initLabelArray, and
	// the portal renders the whole list rather than only what is populated.
	resp := domain.ProjectCaseStatsResponse{
		StateCount:                     zeroedCounts(labels[caseStateEnumType]),
		SeverityCount:                  zeroedCounts(labels[caseSeverityEnumType]),
		OutstandingSeverityCount:       zeroedCounts(labels[caseSeverityEnumType]),
		EngagementTypeCount:            zeroedCounts(labels[engagementTypeEnumType]),
		OutstandingEngagementTypeCount: zeroedCounts(labels[engagementTypeEnumType]),
		CaseTypeCount:                  make([]domain.ReferenceTableItem, 0, len(repository.CaseTypeRefs)),
		CasesTrend:                     deprecatedCasesTrend(),
	}

	for _, row := range stateSeverity {
		resp.TotalCount += row.Count
		incrementCount(resp.StateCount, row.State, row.Count)
		incrementCount(resp.SeverityCount, row.Severity, row.Count)

		// Active and outstanding are deliberately the same set: every state
		// except CLOSED. See the type's doc comment.
		if row.State != caseStateClosed {
			resp.ActiveCount += row.Count
			resp.OutstandingCount += row.Count
			incrementCount(resp.OutstandingSeverityCount, row.Severity, row.Count)
		}

		if row.State == caseStateAwaitingInfo || row.State == caseStateSolutionProposed {
			resp.ActionRequiredCount += row.Count
		}

		if containsString(caseStatsResolvedStates, row.State) {
			resp.ResolvedCount.Total += row.Count
		}
	}

	for _, row := range engagementTypes {
		incrementCount(resp.EngagementTypeCount, row.EngagementType, row.Count)
		if row.State != caseStateClosed {
			incrementCount(resp.OutstandingEngagementTypeCount, row.EngagementType, row.Count)
		}
	}

	resp.ResolvedCount.CurrentMonth = currentMonth
	resp.ResolvedCount.PastThirtyDays = pastThirtyDays

	resp.ChangeRate.ResolvedEngagements = percentChange(current, previous)

	if slaCount > 0 {
		// ServiceNow floors the per-SLA mean to whole seconds before
		// converting, so the same input yields the same hours figure here.
		resp.AverageResponseTime = roundToTwoDecimals(math.Floor(avgSeconds) / 3600)
	}
	// changeRate.averageResponseTime stays 0: ServiceNow's previous-window
	// SLA computation is commented out at the source, so it reports 0 for
	// every project. See the type's doc comment.

	for _, ref := range repository.CaseTypeRefs {
		count := caseTypeCounts[ref.ID]
		item := ref
		item.Count = &count
		resp.CaseTypeCount = append(resp.CaseTypeCount, item)
	}

	return resp, nil
}

// createdBySelf is the only value the createdBy filter accepts. It is not an
// email: ServiceNow's scripted APIs reject anything else with a 400 and then
// substitute the authenticated caller's own address, and the ServiceNow-backed
// service here forwards the caller's value to them untouched -- so "me" is
// the whole vocabulary of this parameter on both data sources.
const createdBySelf = "me"

// resolveCreatedByFilter turns the createdBy query value into the email the
// repositories actually filter on (work_item.created_by holds an email).
//
// Treating the raw value as an email would be silently wrong rather than
// loudly wrong: filtering for a literal "me" matches no rows, so the caller
// would get a successful response full of zeros instead of their own items.
func resolveCreatedByFilter(ctx context.Context, createdBy string) (string, error) {
	if createdBy == "" {
		return "", nil
	}
	if createdBy != createdBySelf {
		return "", &apierror.ValidationError{Msg: `createdBy: the only allowed value is "me"`}
	}
	return resolveCallerEmail(ctx)
}

// zeroedCounts builds a choice list of the given labels, each with a count of
// zero, using the label as both id and label -- the same vocabulary
// choiceListFromLabels gives GET /projects/{id}/metadata in Postgres mode.
func zeroedCounts(labels []string) []domain.ChoiceListItem {
	out := make([]domain.ChoiceListItem, 0, len(labels))
	for _, l := range labels {
		count := 0
		out = append(out, domain.ChoiceListItem{ID: l, Label: l, Count: &count})
	}
	return out
}

// incrementCount adds n to the entry with this id. An id with no entry --
// an empty one (a row whose type carries no severity, or which has no
// extension row at all) or a value outside the enum -- is ignored, matching
// ServiceNow's _increment, which no-ops on an unknown label.
func incrementCount(items []domain.ChoiceListItem, id string, n int) {
	if id == "" {
		return
	}
	for i := range items {
		if items[i].ID == id {
			*items[i].Count += n
			return
		}
	}
}

// deprecatedCasesTrend returns the placeholder ServiceNow itself returns for
// casesTrend. Its real implementation (_getLast6QuarterStats, a six-quarter
// severity breakdown) is dead code there -- never called, with the stub
// assigned to the response instead -- so emitting anything richer here would
// make the two data sources disagree on a field no caller can rely on.
func deprecatedCasesTrend() []domain.CasesTrend {
	count := 0
	return []domain.CasesTrend{{
		Period:     "",
		Severities: []domain.ChoiceListItem{{ID: "0", Label: "", Count: &count}},
	}}
}

// percentChange is ServiceNow's change-rate formula: the percentage change
// from previous to current, or a flat 100 when there is no previous figure
// to compare against but a current one exists.
func percentChange(current, previous int) float64 {
	if previous > 0 {
		return roundToTwoDecimals(float64(current-previous) / float64(previous) * 100)
	}
	if current > 0 {
		return 100
	}
	return 0
}

// roundToTwoDecimals mirrors ServiceNow's _roundToTwoDecimals.
func roundToTwoDecimals(v float64) float64 {
	return math.Round(v*100) / 100
}

// containsString reports whether needle is in haystack.
func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
