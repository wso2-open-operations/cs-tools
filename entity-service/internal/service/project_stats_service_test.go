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
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

type fakeProjectStatsRepo struct {
	billableMinutes    int
	nonBillableMinutes int
	deployments        int
	deployedProducts   int
	instances          int
	instanceErr        error
	lastDeployment     *time.Time
	outstanding        map[string]int
	slaInputs          repository.ProjectSLAStatusInputs
	conversations      []repository.StateCount
	changeRequests     []repository.StateCount
	crCurrentMonth     int
	crPastThirtyDays   int

	timeLoggedStart string
	timeLoggedEnd   string
}

func (f *fakeProjectStatsRepo) TimeLoggedMinutes(_ context.Context, _, startDate, endDate string) (int, int, error) {
	f.timeLoggedStart, f.timeLoggedEnd = startDate, endDate
	return f.billableMinutes, f.nonBillableMinutes, nil
}
func (f *fakeProjectStatsRepo) DeploymentCount(context.Context, string) (int, error) {
	return f.deployments, nil
}
func (f *fakeProjectStatsRepo) DeployedProductCount(context.Context, string) (int, error) {
	return f.deployedProducts, nil
}
func (f *fakeProjectStatsRepo) InstanceCount(context.Context, string) (int, error) {
	return f.instances, f.instanceErr
}
func (f *fakeProjectStatsRepo) LastDeploymentOn(context.Context, string) (*time.Time, error) {
	return f.lastDeployment, nil
}
func (f *fakeProjectStatsRepo) OutstandingCounts(context.Context, repository.SearchScope, string, []string, []string) (map[string]int, error) {
	return f.outstanding, nil
}
func (f *fakeProjectStatsRepo) SLAStatusInputs(context.Context, string) (repository.ProjectSLAStatusInputs, error) {
	return f.slaInputs, nil
}
func (f *fakeProjectStatsRepo) ConversationStateCounts(context.Context, string, string) ([]repository.StateCount, error) {
	return f.conversations, nil
}
func (f *fakeProjectStatsRepo) ChangeRequestStateCounts(context.Context, string) ([]repository.StateCount, error) {
	return f.changeRequests, nil
}
func (f *fakeProjectStatsRepo) ChangeRequestResolvedBuckets(context.Context, string, string) (int, int, error) {
	return f.crCurrentMonth, f.crPastThirtyDays, nil
}

func statsEnums() *fakeReferenceDataRepo {
	return &fakeReferenceDataRepo{enums: map[string][]string{
		conversationStateEnumType:  {"OPEN", "ACTIVE", "RESOLVED", "CONVERTED", "ABANDONED", "CLOSE"},
		changeRequestStateEnumType: {"NEW", "ASSESS", "AUTHORIZE", "CUSTOMER_APPROVAL", "SCHEDULED", "IMPLEMENT", "REVIEW", "CUSTOMER_REVIEW", "ROLLBACK", "CLOSED", "CANCELED"},
	}}
}

func newStatsService(repo *fakeProjectStatsRepo) ProjectStatsService {
	ref := statsEnums()
	return NewProjectStatsService(repo, ref, alwaysUnrestrictedAccess{}, NewProjectMetadataService(ref),
		NewProjectCaseStatsService(&fakeCaseStatsRepo{}, ref, alwaysUnrestrictedAccess{}))
}

// All four conditions must hold for "All Good"; any single failure -- and an
// outstanding case counts as a failure even when everything else is fine --
// makes it "Needs Attention".
func TestProjectSLAStatus(t *testing.T) {
	good := repository.ProjectSLAStatusInputs{
		HasOutstandingCase: false, HasDeployedProduct: true,
		HasActiveEndDate: true, HasCustomerAdminContact: true,
	}
	if got := projectSLAStatus(good); got != projectSLAStatusAllGood {
		t.Errorf("all conditions met = %q, want %q", got, projectSLAStatusAllGood)
	}

	tests := []struct {
		name   string
		mutate func(*repository.ProjectSLAStatusInputs)
	}{
		{"an outstanding case", func(i *repository.ProjectSLAStatusInputs) { i.HasOutstandingCase = true }},
		{"no deployed product", func(i *repository.ProjectSLAStatusInputs) { i.HasDeployedProduct = false }},
		{"no active end date", func(i *repository.ProjectSLAStatusInputs) { i.HasActiveEndDate = false }},
		{"no customer admin", func(i *repository.ProjectSLAStatusInputs) { i.HasCustomerAdminContact = false }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := good
			tt.mutate(&in)
			if got := projectSLAStatus(in); got != projectSLAStatusNeedsAttention {
				t.Errorf("%s = %q, want %q", tt.name, got, projectSLAStatusNeedsAttention)
			}
		})
	}
}

func TestGetProjectStats_MapsOutstandingCountsAndHours(t *testing.T) {
	repo := &fakeProjectStatsRepo{
		billableMinutes: 450, nonBillableMinutes: 150,
		deployments: 3, deployedProducts: 5, instances: 7,
		outstanding: map[string]int{
			"case": 4, "service_request": 2, "engagement": 1,
			"security_report_analysis": 3, "announcement": 6, "change_request": 8,
		},
		slaInputs: repository.ProjectSLAStatusInputs{
			HasDeployedProduct: true, HasActiveEndDate: true, HasCustomerAdminContact: true,
		},
	}

	resp, err := newStatsService(repo).GetProjectStats(context.Background(), testUUID)
	if err != nil {
		t.Fatalf("GetProjectStats: %v", err)
	}

	// MINUTES, not hours -- the field names are wrong on the wire and the
	// portal divides by 60 itself. 450 + 150 logged minutes.
	if resp.TotalHours != 600 || resp.BillableHours != 450 {
		t.Errorf("totalHours/billableHours = %v/%v, want 600/450 minutes", resp.TotalHours, resp.BillableHours)
	}
	if resp.SLAStatus != projectSLAStatusAllGood {
		t.Errorf("slaStatus = %q, want %q", resp.SLAStatus, projectSLAStatusAllGood)
	}
	if resp.DeploymentCount != 3 || resp.DeployedProductCount != 5 || resp.InstanceCount != 7 {
		t.Errorf("counts = %d/%d/%d, want 3/5/7", resp.DeploymentCount, resp.DeployedProductCount, resp.InstanceCount)
	}
	want := domain.ProjectStatsOutstandingCount{
		CaseCount: 4, ServiceRequestCount: 2, EngagementCount: 1,
		SraCount: 3, AnnouncementCount: 6, ChangeRequestCount: 8,
	}
	if resp.OutstandingCount != want {
		t.Errorf("outstandingCount = %+v, want %+v", resp.OutstandingCount, want)
	}
}

// deployment_node uses the live schema's column names, which a
// migrations-built database lacks, so the count degrades to zero instead of
// failing the whole dashboard -- but the failure is logged, because a
// silently-zero count is indistinguishable from a project with no instances.
func TestGetProjectStats_InstanceCountDegradesToZero(t *testing.T) {
	repo := &fakeProjectStatsRepo{
		instances: 99, instanceErr: errors.New("column dn.project_key does not exist"),
		slaInputs: repository.ProjectSLAStatusInputs{
			HasDeployedProduct: true, HasActiveEndDate: true, HasCustomerAdminContact: true,
		},
	}

	logs := captureSlog(t)

	resp, err := newStatsService(repo).GetProjectStats(context.Background(), testUUID)
	if err != nil {
		t.Fatalf("GetProjectStats must not fail on an instance-count error: %v", err)
	}
	if resp.InstanceCount != 0 {
		t.Errorf("instanceCount = %d, want 0", resp.InstanceCount)
	}
	if resp.DeployedProductCount != repo.deployedProducts {
		t.Errorf("the rest of the response must still be populated")
	}
	if out := logs.String(); !strings.Contains(out, "instance count degraded to zero") ||
		!strings.Contains(out, "dn.project_key") {
		t.Errorf("expected a warning naming the underlying error, got: %s", out)
	}
}

// A healthy instance count must not log anything -- the warning is a signal
// that something is wrong, so it has to stay quiet on the happy path.
func TestGetProjectStats_InstanceCountSuccessIsSilent(t *testing.T) {
	repo := &fakeProjectStatsRepo{instances: 7}
	logs := captureSlog(t)

	if _, err := newStatsService(repo).GetProjectStats(context.Background(), testUUID); err != nil {
		t.Fatalf("GetProjectStats: %v", err)
	}
	if out := logs.String(); strings.Contains(out, "instance count degraded") {
		t.Errorf("expected no warning on success, got: %s", out)
	}
}

// captureSlog redirects the default logger for one test and restores it after.
func captureSlog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

func TestGetProjectConversationStats_ActiveStates(t *testing.T) {
	repo := &fakeProjectStatsRepo{conversations: []repository.StateCount{
		{State: "OPEN", Count: 2},
		{State: "ACTIVE", Count: 3},
		{State: "RESOLVED", Count: 4},
		{State: "ABANDONED", Count: 1},
	}}

	resp, err := newStatsService(repo).GetProjectConversationStats(context.Background(), testUUID, "")
	if err != nil {
		t.Fatalf("GetProjectConversationStats: %v", err)
	}
	if resp.TotalCount != 10 {
		t.Errorf("totalCount = %d, want 10", resp.TotalCount)
	}
	// Only OPEN and ACTIVE are active, matching CHAT_ACTIVE_STATE_VALUES.
	if resp.ActiveCount != 5 {
		t.Errorf("activeCount = %d, want 5", resp.ActiveCount)
	}
	if got := countFor(t, resp.StateCount, "CLOSE"); got != 0 {
		t.Errorf("unseen state CLOSE = %d, want 0 but present", got)
	}
}

// A change request's active and outstanding sets genuinely differ, unlike a
// case's: NEW/ASSESS/AUTHORIZE are active but not yet outstanding.
func TestGetProjectChangeRequestStats_StateGroupings(t *testing.T) {
	repo := &fakeProjectStatsRepo{
		changeRequests: []repository.StateCount{
			{State: "NEW", Count: 1},
			{State: "ASSESS", Count: 2},
			{State: "CUSTOMER_APPROVAL", Count: 3},
			{State: "IMPLEMENT", Count: 4},
			{State: "CUSTOMER_REVIEW", Count: 5},
			{State: "CLOSED", Count: 6},
			{State: "CANCELED", Count: 7},
		},
		crCurrentMonth: 2, crPastThirtyDays: 4,
	}

	resp, err := newStatsService(repo).GetProjectChangeRequestStats(context.Background(), testUUID)
	if err != nil {
		t.Fatalf("GetProjectChangeRequestStats: %v", err)
	}

	if resp.TotalCount != 28 {
		t.Errorf("totalCount = %d, want 28", resp.TotalCount)
	}
	// NEW+ASSESS+CUSTOMER_APPROVAL+IMPLEMENT+CUSTOMER_REVIEW
	if resp.ActiveCount != 15 {
		t.Errorf("activeCount = %d, want 15", resp.ActiveCount)
	}
	// CUSTOMER_APPROVAL+IMPLEMENT+CUSTOMER_REVIEW -- NEW/ASSESS excluded
	if resp.OutstandingCount != 12 {
		t.Errorf("outstandingCount = %d, want 12", resp.OutstandingCount)
	}
	// CUSTOMER_APPROVAL+CUSTOMER_REVIEW
	if resp.ActionRequiredCount != 8 {
		t.Errorf("actionRequiredCount = %d, want 8", resp.ActionRequiredCount)
	}
	// CLOSED only -- CANCELED is terminal but not resolved.
	if resp.ResolvedCount.Total != 6 {
		t.Errorf("resolvedCount.total = %d, want 6", resp.ResolvedCount.Total)
	}
	if resp.ResolvedCount.CurrentMonth != 2 || resp.ResolvedCount.PastThirtyDays != 4 {
		t.Errorf("resolved buckets = %d/%d, want 2/4", resp.ResolvedCount.CurrentMonth, resp.ResolvedCount.PastThirtyDays)
	}
}

func TestGetProjectDeploymentStats_FormatsLastDeployment(t *testing.T) {
	when := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	repo := &fakeProjectStatsRepo{deployments: 2, lastDeployment: &when}

	resp, err := newStatsService(repo).GetProjectDeploymentStats(context.Background(), testUUID)
	if err != nil {
		t.Fatalf("GetProjectDeploymentStats: %v", err)
	}
	if resp.TotalCount != 2 {
		t.Errorf("totalCount = %d, want 2", resp.TotalCount)
	}
	if resp.LastDeploymentOn == nil || *resp.LastDeploymentOn != "2026-03-04T05:06:07Z" {
		t.Errorf("lastDeploymentOn = %v, want 2026-03-04T05:06:07Z", resp.LastDeploymentOn)
	}

	none, err := newStatsService(&fakeProjectStatsRepo{}).GetProjectDeploymentStats(context.Background(), testUUID)
	if err != nil {
		t.Fatalf("GetProjectDeploymentStats (no deployments): %v", err)
	}
	if none.LastDeploymentOn != nil {
		t.Errorf("lastDeploymentOn = %v, want nil when the project has no deployments", *none.LastDeploymentOn)
	}
}

func TestGetProjectTimeCardStats_PassesDateRangeAndSplitsHours(t *testing.T) {
	repo := &fakeProjectStatsRepo{billableMinutes: 90, nonBillableMinutes: 45}

	resp, err := newStatsService(repo).GetProjectTimeCardStats(context.Background(), testUUID, "2026-01-01", "2026-01-31")
	if err != nil {
		t.Fatalf("GetProjectTimeCardStats: %v", err)
	}
	if repo.timeLoggedStart != "2026-01-01" || repo.timeLoggedEnd != "2026-01-31" {
		t.Errorf("date range = %q..%q, want it forwarded to the repository", repo.timeLoggedStart, repo.timeLoggedEnd)
	}
	// MINUTES on the wire; the portal converts. 90 billable + 45 non-billable.
	if resp.TotalHours != 135 || resp.BillableHours != 90 || resp.NonBillableHours != 45 {
		t.Errorf("minutes = %v/%v/%v, want 135/90/45", resp.TotalHours, resp.BillableHours, resp.NonBillableHours)
	}
}

// The conversation stats endpoint takes the same createdBy parameter as case
// stats, with the same "me"-only vocabulary, resolved to the caller's email.
func TestGetProjectConversationStats_CreatedByResolvesCaller(t *testing.T) {
	repo := &fakeProjectStatsRepo{}
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "caller@wso2.com"))

	if _, err := newStatsService(repo).GetProjectConversationStats(ctx, testUUID, createdBySelf); err != nil {
		t.Fatalf("GetProjectConversationStats: %v", err)
	}

	_, err := newStatsService(repo).GetProjectConversationStats(ctx, testUUID, "someone@wso2.com")
	var validationErr *apierror.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error = %v, want a ValidationError for a non-\"me\" createdBy", err)
	}
}

// A malformed date must be a 400 naming the parameter, not an opaque 500 from
// the repository's ::date cast.
func TestGetProjectTimeCardStats_RejectsMalformedDates(t *testing.T) {
	svc := newStatsService(&fakeProjectStatsRepo{})

	for _, tt := range []struct{ name, start, end string }{
		{"bad start", "not-a-date", ""},
		{"bad end", "", "2026-13-45"},
		{"wrong layout", "01/02/2026", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.GetProjectTimeCardStats(context.Background(), testUUID, tt.start, tt.end)
			var validationErr *apierror.ValidationError
			if !errors.As(err, &validationErr) {
				t.Fatalf("error = %v, want a ValidationError", err)
			}
		})
	}

	// Both bounds absent is valid -- the range filter is optional.
	if _, err := svc.GetProjectTimeCardStats(context.Background(), testUUID, "", ""); err != nil {
		t.Errorf("no date bounds: %v", err)
	}
}

// Same IDOR guard as the case-stats service: every method on this service
// goes through requireProject, so one out-of-scope check covers all five.
func TestProjectStats_OutOfScopeProjectIsNotFound(t *testing.T) {
	access := stubAccess{scope: AccessScope{ProjectIDs: []string{"99999999-9999-9999-9999-999999999999"}}}
	ref := statsEnums()
	svc := NewProjectStatsService(&fakeProjectStatsRepo{}, ref, access,
		NewProjectMetadataService(ref), NewProjectCaseStatsService(&fakeCaseStatsRepo{}, ref, access))

	ctx := context.Background()
	calls := map[string]error{}
	_, calls["GetProjectStats"] = svc.GetProjectStats(ctx, testUUID)
	_, calls["GetProjectConversationStats"] = svc.GetProjectConversationStats(ctx, testUUID, "")
	_, calls["GetProjectDeploymentStats"] = svc.GetProjectDeploymentStats(ctx, testUUID)
	_, calls["GetProjectTimeCardStats"] = svc.GetProjectTimeCardStats(ctx, testUUID, "", "")
	_, calls["GetProjectChangeRequestStats"] = svc.GetProjectChangeRequestStats(ctx, testUUID)

	for name, err := range calls {
		var notFound *apierror.NotFoundError
		if !errors.As(err, &notFound) {
			t.Errorf("%s: error = %v, want NotFoundError for an out-of-scope project", name, err)
		}
	}
}

func TestProjectStats_UnrestrictedCallerIsAllowed(t *testing.T) {
	repo := &fakeProjectStatsRepo{deployments: 4}
	resp, err := newStatsService(repo).GetProjectDeploymentStats(context.Background(), testUUID)
	if err != nil {
		t.Fatalf("an unrestricted caller must be allowed: %v", err)
	}
	if resp.TotalCount != 4 {
		t.Errorf("totalCount = %d, want 4", resp.TotalCount)
	}
}
