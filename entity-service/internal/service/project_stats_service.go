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
	"log/slog"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// Project SLA status values, spelled exactly as ServiceNow's
// PROJECT_SLA_CONFIG -- the portal matches on the string.
const (
	projectSLAStatusAllGood        = "All Good"
	projectSLAStatusNeedsAttention = "Needs Attention"
)

// caseStatsOutstandingStates are the states that count as outstanding for a
// case-like work item: every one except CLOSED. Identical to the active set,
// which is ServiceNow's own "ACTIVE === OUTSTANDING per business
// requirement" (see projectCaseStatsService).
var caseStatsOutstandingStates = []string{
	"OPEN", "WORK_IN_PROGRESS", "AWAITING_INFO", "WAITING_ON_WSO2", "REOPENED", "SOLUTION_PROPOSED",
}

// Change-request state groupings, mirroring ServiceNow's CR_* constants.
// change_request_state_enum's labels (migration 000047) match them one for
// one, so no key translation is needed -- unlike case severity.
//
// Unlike cases, a change request's active and outstanding sets genuinely
// differ: the three earliest states (NEW/ASSESS/AUTHORIZE) are active but
// not yet outstanding.
var (
	crActiveStates         = []string{"NEW", "ASSESS", "AUTHORIZE", "CUSTOMER_APPROVAL", "SCHEDULED", "IMPLEMENT", "REVIEW", "CUSTOMER_REVIEW"}
	crOutstandingStates    = []string{"CUSTOMER_APPROVAL", "SCHEDULED", "IMPLEMENT", "REVIEW", "CUSTOMER_REVIEW"}
	crActionRequiredStates = []string{"CUSTOMER_APPROVAL", "CUSTOMER_REVIEW"}
)

const crResolvedState = "CLOSED"

// conversationActiveStates mirrors ServiceNow's CHAT_ACTIVE_STATE_VALUES.
// conversation_state_enum (migration 000057) carries the same vocabulary.
var conversationActiveStates = []string{"OPEN", "ACTIVE"}

// projectStatsService is the Postgres-backed ProjectStatsService: the whole
// seven-method interface, so routes.go can pick one implementation for both
// data sources instead of registering a subset.
//
// GetProjectMetadata and GetProjectCaseStats already had their own Postgres
// implementations (they were split out when only they were portable); this
// composes them rather than duplicating either.
//
// Two ServiceNow behaviours have no Postgres equivalent and are documented at
// the code that would otherwise reproduce them: the project-type-derived
// allowed-severity narrowing (no feature-entitlement table exists), and the
// units question on logged time (see GetProjectStats).
type projectStatsService struct {
	repo      repository.ProjectStatsRepository
	refRepo   repository.ReferenceDataRepository
	access    AccessService
	metadata  ProjectMetadataService
	caseStats ProjectCaseStatsService
}

// NewProjectStatsService constructs a Postgres-backed ProjectStatsService.
func NewProjectStatsService(
	repo repository.ProjectStatsRepository,
	refRepo repository.ReferenceDataRepository,
	access AccessService,
	metadata ProjectMetadataService,
	caseStats ProjectCaseStatsService,
) ProjectStatsService {
	return &projectStatsService{repo: repo, refRepo: refRepo, access: access, metadata: metadata, caseStats: caseStats}
}

// GetProjectMetadata implements ProjectStatsService by delegation.
func (s *projectStatsService) GetProjectMetadata(ctx context.Context, projectID string) (domain.ProjectMetadataResponse, error) {
	return s.metadata.GetProjectMetadata(ctx, projectID)
}

// GetProjectCaseStats implements ProjectStatsService by delegation.
func (s *projectStatsService) GetProjectCaseStats(ctx context.Context, projectID string, req domain.ProjectCaseStatsRequest) (domain.ProjectCaseStatsResponse, error) {
	return s.caseStats.GetProjectCaseStats(ctx, projectID, req)
}

// requireProject validates the id, confirms the caller may see the project,
// and confirms it exists -- in that order, so every method below reports a
// project the caller has no access to exactly as it reports a missing one.
// Returns the resolved scope so callers that go on to run an
// RLS-protected repository query (see repository.SearchScope) can forward the
// same identity authorizeProject already resolved, instead of resolving it
// twice.
func (s *projectStatsService) requireProject(ctx context.Context, projectID string) (AccessScope, error) {
	if err := validateUUIDs("id", []string{projectID}); err != nil {
		return AccessScope{}, err
	}
	scope, err := authorizeProject(ctx, s.access, projectID)
	if err != nil {
		return AccessScope{}, err
	}
	found, _, err := s.refRepo.GetProjectByID(ctx, projectID)
	if err != nil {
		return AccessScope{}, err
	}
	if !found {
		return AccessScope{}, &apierror.NotFoundError{Msg: "project not found"}
	}
	return scope, nil
}

// GetProjectStats implements ProjectStatsService -- ServiceNow's
// getProjectStatistics.
func (s *projectStatsService) GetProjectStats(ctx context.Context, projectID string) (domain.ProjectStatsResponse, error) {
	scope, err := s.requireProject(ctx, projectID)
	if err != nil {
		return domain.ProjectStatsResponse{}, err
	}

	// The six aggregations below share only the project id, so they run
	// concurrently rather than as six serial round trips -- the same
	// errgroup pattern SearchCases and SearchCaseComments already use for
	// their COUNT/SELECT pair. Each goroutine writes its own variable, and
	// the response is assembled afterwards, so the result is identical to
	// running them in order.
	var (
		billableMinutes, nonBillableMinutes int
		deployments, deployedProducts       int
		instances                           int
		outstanding                         map[string]int
		slaInputs                           repository.ProjectSLAStatusInputs
	)

	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		var err error
		billableMinutes, nonBillableMinutes, err = s.repo.TimeLoggedMinutes(gctx, projectID, "", "")
		return err
	})
	g.Go(func() error {
		var err error
		deployments, err = s.repo.DeploymentCount(gctx, projectID)
		return err
	})
	g.Go(func() error {
		var err error
		deployedProducts, err = s.repo.DeployedProductCount(gctx, projectID)
		return err
	})
	g.Go(func() error {
		// deployment_node is spelled with the live schema's column names,
		// which a migrations-built database does not have (see the
		// repository's own note). A failure here must not take down the
		// whole dashboard, so the count degrades to zero and the error is
		// deliberately NOT returned -- returning it would cancel gctx and
		// fail every sibling query too.
		//
		// It is still logged, because a silently-zero count is
		// indistinguishable from a project that genuinely has no instances.
		// Not when gctx is already cancelled: that means a sibling query
		// failed first and this error is just the fallout, so logging it
		// would bury the real cause under noise.
		n, err := s.repo.InstanceCount(gctx, projectID)
		if err != nil {
			if gctx.Err() == nil {
				slog.WarnContext(ctx, "project stats: instance count degraded to zero",
					"projectID", projectID, "error", err)
			}
			return nil
		}
		instances = n
		return nil
	})
	g.Go(func() error {
		var err error
		outstanding, err = s.repo.OutstandingCounts(gctx, scope, projectID, caseStatsOutstandingStates, crOutstandingStates)
		return err
	})
	g.Go(func() error {
		var err error
		slaInputs, err = s.repo.SLAStatusInputs(gctx, projectID)
		return err
	})

	if err := g.Wait(); err != nil {
		return domain.ProjectStatsResponse{}, err
	}

	return domain.ProjectStatsResponse{
		// MINUTES, despite the field names -- see loggedMinutes.
		TotalHours:           loggedMinutes(billableMinutes + nonBillableMinutes),
		BillableHours:        loggedMinutes(billableMinutes),
		SLAStatus:            projectSLAStatus(slaInputs),
		DeploymentCount:      deployments,
		DeployedProductCount: deployedProducts,
		InstanceCount:        instances,
		OutstandingCount: domain.ProjectStatsOutstandingCount{
			CaseCount:           outstanding["case"],
			ServiceRequestCount: outstanding["service_request"],
			EngagementCount:     outstanding["engagement"],
			SraCount:            outstanding["security_report_analysis"],
			AnnouncementCount:   outstanding["announcement"],
			ChangeRequestCount:  outstanding["change_request"],
		},
	}, nil
}

// projectSLAStatus reduces the four conditions to the status string.
// ServiceNow checks them in sequence and returns Needs Attention on the first
// failure; since none has a side effect, evaluating all four and combining
// them is equivalent.
func projectSLAStatus(in repository.ProjectSLAStatusInputs) string {
	if in.HasOutstandingCase || !in.HasDeployedProduct || !in.HasActiveEndDate || !in.HasCustomerAdminContact {
		return projectSLAStatusNeedsAttention
	}
	return projectSLAStatusAllGood
}

// loggedMinutes returns logged time in MINUTES, which is what every
// totalHours/billableHours/nonBillableHours field on these responses actually
// carries -- the names are wrong, and deliberately preserved.
//
// ServiceNow sums time_card.total, which TimeCardUtils populates from
// _sumMinutes (the five per-activity minute fields) and its own response
// mapper annotates as minutes; getProjectTotalsOnly and
// getProjectCaseTimeLogged then assign that sum straight to fields named
// *Hours. The customer portal compensates on the way out --
// useGetTimeCardsStats divides by 60 under the comment "Convert minutes to
// hours" -- so minutes is the real wire contract on both sides.
//
// Returning true hours here would be arithmetically correct and render every
// figure in the portal 60x too small. Renaming the fields is the actual fix,
// and it belongs in a coordinated change across ServiceNow, this service and
// the portal -- not silently in one data source.
func loggedMinutes(minutes int) float64 {
	return float64(minutes)
}

// GetProjectConversationStats implements ProjectStatsService -- ServiceNow's
// getProjectChatStats.
func (s *projectStatsService) GetProjectConversationStats(ctx context.Context, projectID, createdBy string) (domain.ProjectConversationStatsResponse, error) {
	if _, err := s.requireProject(ctx, projectID); err != nil {
		return domain.ProjectConversationStatsResponse{}, err
	}

	createdByEmail, err := resolveCreatedByFilter(ctx, createdBy)
	if err != nil {
		return domain.ProjectConversationStatsResponse{}, err
	}

	labels, err := s.refRepo.EnumLabels(ctx, []string{conversationStateEnumType})
	if err != nil {
		return domain.ProjectConversationStatsResponse{}, fmt.Errorf("project conversation stats: %w", err)
	}

	resp := domain.ProjectConversationStatsResponse{
		StateCount: zeroedCounts(labels[conversationStateEnumType]),
	}

	rows, err := s.repo.ConversationStateCounts(ctx, projectID, createdByEmail)
	if err != nil {
		return domain.ProjectConversationStatsResponse{}, err
	}
	for _, row := range rows {
		resp.TotalCount += row.Count
		incrementCount(resp.StateCount, row.State, row.Count)
		if containsString(conversationActiveStates, row.State) {
			resp.ActiveCount += row.Count
		}
	}
	return resp, nil
}

// GetProjectDeploymentStats implements ProjectStatsService.
func (s *projectStatsService) GetProjectDeploymentStats(ctx context.Context, projectID string) (domain.ProjectDeploymentStatsResponse, error) {
	if _, err := s.requireProject(ctx, projectID); err != nil {
		return domain.ProjectDeploymentStatsResponse{}, err
	}

	total, err := s.repo.DeploymentCount(ctx, projectID)
	if err != nil {
		return domain.ProjectDeploymentStatsResponse{}, err
	}

	// ServiceNow reads its last deployment from a separate
	// customer-project-deployment table; this schema has only deployment
	// itself, so the newest row's creation time is the equivalent.
	last, err := s.repo.LastDeploymentOn(ctx, projectID)
	if err != nil {
		return domain.ProjectDeploymentStatsResponse{}, err
	}
	var lastOn *string
	if last != nil {
		formatted := last.UTC().Format(time.RFC3339)
		lastOn = &formatted
	}

	return domain.ProjectDeploymentStatsResponse{TotalCount: total, LastDeploymentOn: lastOn}, nil
}

// GetProjectTimeCardStats implements ProjectStatsService. startDate/endDate
// are optional inclusive bounds on the time card's work date.
func (s *projectStatsService) GetProjectTimeCardStats(ctx context.Context, projectID, startDate, endDate string) (domain.ProjectTimeCardStatsResponse, error) {
	if err := validateStatsDate("startDate", startDate); err != nil {
		return domain.ProjectTimeCardStatsResponse{}, err
	}
	if err := validateStatsDate("endDate", endDate); err != nil {
		return domain.ProjectTimeCardStatsResponse{}, err
	}
	if _, err := s.requireProject(ctx, projectID); err != nil {
		return domain.ProjectTimeCardStatsResponse{}, err
	}

	billable, nonBillable, err := s.repo.TimeLoggedMinutes(ctx, projectID, startDate, endDate)
	if err != nil {
		return domain.ProjectTimeCardStatsResponse{}, err
	}

	// MINUTES, despite the field names -- see loggedMinutes.
	return domain.ProjectTimeCardStatsResponse{
		TotalHours:       loggedMinutes(billable + nonBillable),
		BillableHours:    loggedMinutes(billable),
		NonBillableHours: loggedMinutes(nonBillable),
	}, nil
}

// validateStatsDate rejects a malformed date before it reaches the
// repository's ::date cast, which would otherwise surface as an opaque 500
// rather than telling the caller which parameter was wrong. An empty value
// means "no bound" and is always accepted.
func validateStatsDate(name, value string) error {
	if value == "" {
		return nil
	}
	if _, err := time.Parse("2006-01-02", value); err != nil {
		return &apierror.ValidationError{Msg: name + " must be a date in YYYY-MM-DD format"}
	}
	return nil
}

// GetProjectChangeRequestStats implements ProjectStatsService -- ServiceNow's
// getProjectChangeRequestStats.
func (s *projectStatsService) GetProjectChangeRequestStats(ctx context.Context, projectID string) (domain.ProjectChangeRequestStatsResponse, error) {
	if _, err := s.requireProject(ctx, projectID); err != nil {
		return domain.ProjectChangeRequestStatsResponse{}, err
	}

	labels, err := s.refRepo.EnumLabels(ctx, []string{changeRequestStateEnumType})
	if err != nil {
		return domain.ProjectChangeRequestStatsResponse{}, fmt.Errorf("project change request stats: %w", err)
	}

	resp := domain.ProjectChangeRequestStatsResponse{
		StateCount: zeroedCounts(labels[changeRequestStateEnumType]),
	}

	rows, err := s.repo.ChangeRequestStateCounts(ctx, projectID)
	if err != nil {
		return domain.ProjectChangeRequestStatsResponse{}, err
	}
	for _, row := range rows {
		resp.TotalCount += row.Count
		incrementCount(resp.StateCount, row.State, row.Count)
		if containsString(crActiveStates, row.State) {
			resp.ActiveCount += row.Count
		}
		if containsString(crOutstandingStates, row.State) {
			resp.OutstandingCount += row.Count
		}
		if containsString(crActionRequiredStates, row.State) {
			resp.ActionRequiredCount += row.Count
		}
		if row.State == crResolvedState {
			resp.ResolvedCount.Total += row.Count
		}
	}

	currentMonth, pastThirtyDays, err := s.repo.ChangeRequestResolvedBuckets(ctx, projectID, crResolvedState)
	if err != nil {
		return domain.ProjectChangeRequestStatsResponse{}, err
	}
	resp.ResolvedCount.CurrentMonth = currentMonth
	resp.ResolvedCount.PastThirtyDays = pastThirtyDays

	return resp, nil
}
