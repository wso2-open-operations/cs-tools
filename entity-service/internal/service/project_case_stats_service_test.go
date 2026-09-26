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
	"errors"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

type fakeCaseStatsRepo struct {
	stateSeverity   []repository.StateSeverityCount
	engagementTypes []repository.StateEngagementTypeCount
	currentMonth    int
	pastThirtyDays  int
	windowCurrent   int
	windowPrevious  int
	avgSeconds      float64
	slaCount        int
	caseTypes       map[string]int

	// Captured filters, so a test can assert which ones each aggregation was
	// given -- several deliberately drop the caller's caseTypes/createdBy.
	stateSeverityFilter   repository.ProjectCaseStatsFilter
	engagementTypesFilter repository.ProjectCaseStatsFilter
	caseTypesFilter       repository.ProjectCaseStatsFilter
}

func (f *fakeCaseStatsRepo) StateSeverityCounts(_ context.Context, filter repository.ProjectCaseStatsFilter) ([]repository.StateSeverityCount, error) {
	f.stateSeverityFilter = filter
	return f.stateSeverity, nil
}

func (f *fakeCaseStatsRepo) StateEngagementTypeCounts(_ context.Context, filter repository.ProjectCaseStatsFilter) ([]repository.StateEngagementTypeCount, error) {
	f.engagementTypesFilter = filter
	return f.engagementTypes, nil
}

func (f *fakeCaseStatsRepo) ResolvedBuckets(context.Context, repository.ProjectCaseStatsFilter, []string) (int, int, error) {
	return f.currentMonth, f.pastThirtyDays, nil
}

func (f *fakeCaseStatsRepo) ClosedByCreatedWindow(context.Context, repository.ProjectCaseStatsFilter, string) (int, int, error) {
	return f.windowCurrent, f.windowPrevious, nil
}

func (f *fakeCaseStatsRepo) AverageResponseSeconds(context.Context, string) (float64, int, error) {
	return f.avgSeconds, f.slaCount, nil
}

func (f *fakeCaseStatsRepo) CaseTypeCounts(_ context.Context, filter repository.ProjectCaseStatsFilter) (map[string]int, error) {
	f.caseTypesFilter = filter
	return f.caseTypes, nil
}

func caseStatsEnums() *fakeReferenceDataRepo {
	return &fakeReferenceDataRepo{enums: map[string][]string{
		caseStateEnumType:      {"OPEN", "WORK_IN_PROGRESS", "AWAITING_INFO", "SOLUTION_PROPOSED", "CLOSED", "WAITING_ON_WSO2", "REOPENED"},
		caseSeverityEnumType:   {"S0", "S1", "S2", "S3", "S4"},
		engagementTypeEnumType: {"MIGRATION", "CONSULTANCY"},
	}}
}

func countFor(t *testing.T, items []domain.ChoiceListItem, id string) int {
	t.Helper()
	for _, it := range items {
		if it.ID == id {
			if it.Count == nil {
				t.Fatalf("count for %q is nil; every bucket must carry a count", id)
			}
			return *it.Count
		}
	}
	t.Fatalf("no bucket for %q; buckets are seeded from the enum's full label set", id)
	return 0
}

// The ServiceNow implementation counts SOLUTION_PROPOSED as active/outstanding
// AND as resolved, and derives actionRequired from AWAITING_INFO +
// SOLUTION_PROPOSED. All three overlaps are reproduced deliberately.
func TestGetProjectCaseStats_StateClassification(t *testing.T) {
	repo := &fakeCaseStatsRepo{stateSeverity: []repository.StateSeverityCount{
		{State: "OPEN", Severity: "S1", Count: 3},
		{State: "AWAITING_INFO", Severity: "S2", Count: 2},
		{State: "SOLUTION_PROPOSED", Severity: "S2", Count: 4},
		{State: "CLOSED", Severity: "S3", Count: 10},
	}}

	resp, err := NewProjectCaseStatsService(repo, caseStatsEnums(), alwaysUnrestrictedAccess{}).
		GetProjectCaseStats(context.Background(), testUUID, domain.ProjectCaseStatsRequest{})
	if err != nil {
		t.Fatalf("GetProjectCaseStats: %v", err)
	}

	if resp.TotalCount != 19 {
		t.Errorf("totalCount = %d, want 19", resp.TotalCount)
	}
	// Everything except CLOSED.
	if resp.ActiveCount != 9 {
		t.Errorf("activeCount = %d, want 9", resp.ActiveCount)
	}
	if resp.OutstandingCount != resp.ActiveCount {
		t.Errorf("outstandingCount = %d, want it identical to activeCount (%d)", resp.OutstandingCount, resp.ActiveCount)
	}
	// AWAITING_INFO + SOLUTION_PROPOSED.
	if resp.ActionRequiredCount != 6 {
		t.Errorf("actionRequiredCount = %d, want 6", resp.ActionRequiredCount)
	}
	// CLOSED + SOLUTION_PROPOSED -- the latter is counted as both outstanding
	// and resolved, matching ServiceNow.
	if resp.ResolvedCount.Total != 14 {
		t.Errorf("resolvedCount.total = %d, want 14", resp.ResolvedCount.Total)
	}

	if got := countFor(t, resp.SeverityCount, "S2"); got != 6 {
		t.Errorf("severityCount[S2] = %d, want 6", got)
	}
	// S3 is CLOSED-only, so it contributes to severityCount but not to the
	// outstanding breakdown.
	if got := countFor(t, resp.OutstandingSeverityCount, "S3"); got != 0 {
		t.Errorf("outstandingSeverityCount[S3] = %d, want 0", got)
	}
	if got := countFor(t, resp.StateCount, "REOPENED"); got != 0 {
		t.Errorf("stateCount[REOPENED] = %d, want 0 -- unseen states must still be present", got)
	}
}

// Only the "case" extension table has a severity column, so rows of every
// other case-like type arrive with an empty severity. They must still count
// toward the totals without landing in a severity bucket.
func TestGetProjectCaseStats_RowsWithoutSeverityStillCount(t *testing.T) {
	repo := &fakeCaseStatsRepo{stateSeverity: []repository.StateSeverityCount{
		{State: "OPEN", Severity: "", Count: 5},
	}}

	resp, err := NewProjectCaseStatsService(repo, caseStatsEnums(), alwaysUnrestrictedAccess{}).
		GetProjectCaseStats(context.Background(), testUUID, domain.ProjectCaseStatsRequest{})
	if err != nil {
		t.Fatalf("GetProjectCaseStats: %v", err)
	}

	if resp.TotalCount != 5 || resp.ActiveCount != 5 {
		t.Errorf("totalCount/activeCount = %d/%d, want 5/5", resp.TotalCount, resp.ActiveCount)
	}
	for _, item := range resp.SeverityCount {
		if item.Count != nil && *item.Count != 0 {
			t.Errorf("severityCount[%s] = %d, want 0 -- a row with no severity must not be bucketed", item.ID, *item.Count)
		}
	}
}

// averageResponseTime is reported in hours to two decimals, from a mean the
// ServiceNow implementation floors to whole seconds first.
func TestGetProjectCaseStats_AverageResponseTimeInHours(t *testing.T) {
	repo := &fakeCaseStatsRepo{avgSeconds: 5432.9, slaCount: 3}

	resp, err := NewProjectCaseStatsService(repo, caseStatsEnums(), alwaysUnrestrictedAccess{}).
		GetProjectCaseStats(context.Background(), testUUID, domain.ProjectCaseStatsRequest{})
	if err != nil {
		t.Fatalf("GetProjectCaseStats: %v", err)
	}

	// floor(5432.9) / 3600 = 1.50888... -> 1.51
	if resp.AverageResponseTime != 1.51 {
		t.Errorf("averageResponseTime = %v, want 1.51", resp.AverageResponseTime)
	}
	// ServiceNow's previous-window computation is commented out at the source,
	// so this is always 0.
	if resp.ChangeRate.AverageResponseTime != 0 {
		t.Errorf("changeRate.averageResponseTime = %v, want 0", resp.ChangeRate.AverageResponseTime)
	}
}

func TestGetProjectCaseStats_AverageResponseTimeZeroWithoutSLAs(t *testing.T) {
	repo := &fakeCaseStatsRepo{avgSeconds: 900, slaCount: 0}

	resp, err := NewProjectCaseStatsService(repo, caseStatsEnums(), alwaysUnrestrictedAccess{}).
		GetProjectCaseStats(context.Background(), testUUID, domain.ProjectCaseStatsRequest{})
	if err != nil {
		t.Fatalf("GetProjectCaseStats: %v", err)
	}
	if resp.AverageResponseTime != 0 {
		t.Errorf("averageResponseTime = %v, want 0 when no SLA contributed", resp.AverageResponseTime)
	}
}

func TestPercentChange(t *testing.T) {
	tests := []struct {
		name              string
		current, previous int
		want              float64
	}{
		{"growth", 15, 10, 50},
		{"decline", 5, 10, -50},
		{"no previous window", 7, 0, 100},
		{"both empty", 0, 0, 0},
		{"rounded to two decimals", 10, 3, 233.33},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := percentChange(tt.current, tt.previous); got != tt.want {
				t.Errorf("percentChange(%d, %d) = %v, want %v", tt.current, tt.previous, got, tt.want)
			}
		})
	}
}

// ServiceNow applies createdBy to the main and case-type aggregates but not to
// the engagement-type one, and never applies caseTypes to the case-type
// aggregate. Both are reproduced, so the two data sources report the same
// numbers for the same query.
func TestGetProjectCaseStats_FilterPropagationMatchesServiceNow(t *testing.T) {
	repo := &fakeCaseStatsRepo{}
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "caller@wso2.com"))

	_, err := NewProjectCaseStatsService(repo, caseStatsEnums(), alwaysUnrestrictedAccess{}).
		GetProjectCaseStats(ctx, testUUID, domain.ProjectCaseStatsRequest{
			CaseTypes: []string{"engagement"},
			CreatedBy: createdBySelf,
		})
	if err != nil {
		t.Fatalf("GetProjectCaseStats: %v", err)
	}

	if repo.stateSeverityFilter.CreatedBy != "caller@wso2.com" {
		t.Errorf("main aggregate createdBy = %q, want the caller's resolved email", repo.stateSeverityFilter.CreatedBy)
	}
	if len(repo.stateSeverityFilter.Types) != 1 || repo.stateSeverityFilter.Types[0] != "engagement" {
		t.Errorf("main aggregate types = %v, want [engagement]", repo.stateSeverityFilter.Types)
	}
	if repo.caseTypesFilter.CreatedBy != "caller@wso2.com" {
		t.Errorf("case-type aggregate createdBy = %q, want the caller's resolved email", repo.caseTypesFilter.CreatedBy)
	}
	// ServiceNow's engagement-type aggregate omits the createdBy filter. The
	// service hands every aggregation the same filter struct, so that
	// omission lives in the repository's own SQL -- asserted by
	// TestCaseStatsIntegration_Aggregations/EngagementCountsIgnoreCreatedBy,
	// not here.
}

// createdBy is not an email: ServiceNow accepts only the literal "me" and
// substitutes the caller's own address. Treating a supplied email as a filter
// value would return a successful response full of zeros rather than an
// error, so anything else is rejected outright.
func TestGetProjectCaseStats_CreatedByOnlyAcceptsMe(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "caller@wso2.com"))

	_, err := NewProjectCaseStatsService(&fakeCaseStatsRepo{}, caseStatsEnums(), alwaysUnrestrictedAccess{}).
		GetProjectCaseStats(ctx, testUUID, domain.ProjectCaseStatsRequest{CreatedBy: "someone@wso2.com"})

	var validationErr *apierror.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error = %v, want a ValidationError for a non-\"me\" createdBy", err)
	}
}

// An absent createdBy must not need a token at all -- the filter is optional.
func TestGetProjectCaseStats_NoCreatedByNeedsNoCaller(t *testing.T) {
	repo := &fakeCaseStatsRepo{}

	if _, err := NewProjectCaseStatsService(repo, caseStatsEnums(), alwaysUnrestrictedAccess{}).
		GetProjectCaseStats(context.Background(), testUUID, domain.ProjectCaseStatsRequest{}); err != nil {
		t.Fatalf("GetProjectCaseStats without createdBy: %v", err)
	}
	if repo.stateSeverityFilter.CreatedBy != "" {
		t.Errorf("createdBy = %q, want empty", repo.stateSeverityFilter.CreatedBy)
	}
}

// "default_case" is the in-production frontend's spelling and must normalize to
// the canonical "case" before validation, exactly as the ServiceNow path does.
func TestGetProjectCaseStats_NormalizesDefaultCaseAlias(t *testing.T) {
	repo := &fakeCaseStatsRepo{}

	_, err := NewProjectCaseStatsService(repo, caseStatsEnums(), alwaysUnrestrictedAccess{}).
		GetProjectCaseStats(context.Background(), testUUID, domain.ProjectCaseStatsRequest{
			CaseTypes: []string{"default_case"},
		})
	if err != nil {
		t.Fatalf("GetProjectCaseStats: %v", err)
	}
	if len(repo.stateSeverityFilter.Types) != 1 || repo.stateSeverityFilter.Types[0] != "case" {
		t.Errorf("types = %v, want [case] -- default_case must normalize before the repository sees it", repo.stateSeverityFilter.Types)
	}
}

func TestGetProjectCaseStats_RejectsUnknownCaseType(t *testing.T) {
	_, err := NewProjectCaseStatsService(&fakeCaseStatsRepo{}, caseStatsEnums(), alwaysUnrestrictedAccess{}).
		GetProjectCaseStats(context.Background(), testUUID, domain.ProjectCaseStatsRequest{
			CaseTypes: []string{"not_a_case_type"},
		})
	var validationErr *apierror.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error = %v, want a ValidationError", err)
	}
}

// caseTypeCount is seeded from the full CaseTypeRefs vocabulary -- the same
// id/name pairs GET /projects/{id}/metadata offers -- so a caller can join the
// two directly, and a type with no cases still reports zero.
func TestGetProjectCaseStats_CaseTypeCountCoversWholeVocabulary(t *testing.T) {
	repo := &fakeCaseStatsRepo{caseTypes: map[string]int{"case": 12, "engagement": 4}}

	resp, err := NewProjectCaseStatsService(repo, caseStatsEnums(), alwaysUnrestrictedAccess{}).
		GetProjectCaseStats(context.Background(), testUUID, domain.ProjectCaseStatsRequest{})
	if err != nil {
		t.Fatalf("GetProjectCaseStats: %v", err)
	}

	if len(resp.CaseTypeCount) != len(repository.CaseTypeRefs) {
		t.Fatalf("caseTypeCount has %d entries, want %d", len(resp.CaseTypeCount), len(repository.CaseTypeRefs))
	}
	got := map[string]int{}
	for _, item := range resp.CaseTypeCount {
		if item.Count == nil {
			t.Fatalf("caseTypeCount[%s] has a nil count", item.ID)
		}
		got[item.ID] = *item.Count
	}
	if got["case"] != 12 || got["engagement"] != 4 || got["announcement"] != 0 {
		t.Errorf("caseTypeCount = %v, want case=12 engagement=4 announcement=0", got)
	}
}

// casesTrend is a placeholder in ServiceNow (_getLast6QuarterStats is dead
// code), so this data source must emit the same stub rather than a richer
// series the other one would never produce.
func TestGetProjectCaseStats_CasesTrendIsPlaceholder(t *testing.T) {
	resp, err := NewProjectCaseStatsService(&fakeCaseStatsRepo{}, caseStatsEnums(), alwaysUnrestrictedAccess{}).
		GetProjectCaseStats(context.Background(), testUUID, domain.ProjectCaseStatsRequest{})
	if err != nil {
		t.Fatalf("GetProjectCaseStats: %v", err)
	}
	if len(resp.CasesTrend) != 1 || resp.CasesTrend[0].Period != "" || len(resp.CasesTrend[0].Severities) != 1 {
		t.Errorf("casesTrend = %+v, want the single-entry placeholder", resp.CasesTrend)
	}
}

// The project id comes from the path, so a caller could otherwise read any
// project's statistics by id. An out-of-scope project must be reported as
// NotFound -- the same answer a genuinely missing one gets, so existence is
// never revealed -- and no aggregation may run.
func TestGetProjectCaseStats_OutOfScopeProjectIsNotFound(t *testing.T) {
	repo := &fakeCaseStatsRepo{}
	access := stubAccess{scope: AccessScope{ProjectIDs: []string{"99999999-9999-9999-9999-999999999999"}}}

	_, err := NewProjectCaseStatsService(repo, caseStatsEnums(), access).
		GetProjectCaseStats(context.Background(), testUUID, domain.ProjectCaseStatsRequest{})

	var notFound *apierror.NotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("error = %v, want NotFoundError for a project outside the caller's scope", err)
	}
	if repo.stateSeverityFilter.ProjectID != "" {
		t.Errorf("repository was queried for an unauthorized project (%q) -- the check must run first",
			repo.stateSeverityFilter.ProjectID)
	}
}

func TestGetProjectCaseStats_InScopeProjectIsAllowed(t *testing.T) {
	repo := &fakeCaseStatsRepo{}
	access := stubAccess{scope: AccessScope{ProjectIDs: []string{testUUID}}}

	if _, err := NewProjectCaseStatsService(repo, caseStatsEnums(), access).
		GetProjectCaseStats(context.Background(), testUUID, domain.ProjectCaseStatsRequest{}); err != nil {
		t.Fatalf("a project inside the caller's scope must be allowed: %v", err)
	}
	if repo.stateSeverityFilter.ProjectID != testUUID {
		t.Errorf("aggregation did not run for the authorized project")
	}
}

// A failure to resolve scope must propagate, never fall through to an
// unscoped read.
func TestGetProjectCaseStats_ScopeErrorPropagates(t *testing.T) {
	repo := &fakeCaseStatsRepo{}
	sentinel := errors.New("scope resolution failed")

	_, err := NewProjectCaseStatsService(repo, caseStatsEnums(), stubAccess{err: sentinel}).
		GetProjectCaseStats(context.Background(), testUUID, domain.ProjectCaseStatsRequest{})

	if !errors.Is(err, sentinel) {
		t.Fatalf("error = %v, want the resolver's own error", err)
	}
	if repo.stateSeverityFilter.ProjectID != "" {
		t.Errorf("repository was queried despite scope resolution failing")
	}
}
