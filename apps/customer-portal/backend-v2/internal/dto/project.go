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

package dto

import (
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

// ProjectSummary is one item of the portal's response for POST /projects/search.
//
// Deliberately excludes entity-service's endDateClosureState,
// invoiceDueDateClosureState, complianceViolationClosureState,
// complianceViolationDate and suspensionProcessState — these are WSO2-internal
// closure/compliance workflow fields not meant for customer display.
type ProjectSummary struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Key              string `json:"key"`
	SubscriptionType string `json:"subscriptionType"`
	// Type is what the frontend actually reads (project.type.label); see
	// projectTypeRef. subscriptionType is kept for existing consumers.
	Type         *IDLabelRef `json:"type,omitempty"`
	StartDate    *string     `json:"startDate,omitempty"`
	EndDate      *string     `json:"endDate,omitempty"`
	CreatedOn    time.Time   `json:"createdOn"`
	ClosureState *string     `json:"closureState,omitempty"`
	// No omitempty: the frontend's ProjectListItem types activeCasesCount as a
	// required number, so a project with no active cases must still send 0.
	ActiveCasesCount int `json:"activeCasesCount"`
}

// SearchProjectsResponse is the portal's response for POST /projects/search.
// TotalRecords (not Total) to match the frontend's shared pagination
// envelope.
type SearchProjectsResponse struct {
	Projects     []ProjectSummary `json:"projects"`
	TotalRecords int              `json:"totalRecords"`
	Limit        int              `json:"limit"`
	Offset       int              `json:"offset"`
	HasMore      bool             `json:"hasMore"`
}

// MapSearchProjects builds the portal response from entity-service's SearchProjectsResponse.
func MapSearchProjects(r entity.SearchProjectsResponse) SearchProjectsResponse {
	projects := make([]ProjectSummary, 0, len(r.Projects))
	for _, p := range r.Projects {
		projects = append(projects, ProjectSummary{
			ID:               p.ID,
			Name:             p.Name,
			Key:              p.Key,
			SubscriptionType: p.SubscriptionType,
			Type:             projectTypeRef(p.SubscriptionType),
			StartDate:        DateOnly(p.StartDate),
			EndDate:          DateOnly(p.EndDate),
			CreatedOn:        p.CreatedOn,
			ClosureState:     p.ClosureState,
			ActiveCasesCount: p.ActiveCasesCount,
		})
	}
	return SearchProjectsResponse{
		Projects:     projects,
		TotalRecords: r.Total,
		Limit:        r.Limit,
		Offset:       r.Offset,
		HasMore:      r.HasMore,
	}
}

// SearchProjectsFilters holds the optional filter criteria for
// POST /projects/search, matching the frontend's nested filters.searchQuery
// body shape (apps/customer-portal/webapp/src/api/useGetProjects.ts) rather
// than entity-service's own flat SearchQuery field.
type SearchProjectsFilters struct {
	SearchQuery string `json:"searchQuery,omitempty"`
}

// SearchProjectsRequest is the portal's request body for POST /projects/search.
type SearchProjectsRequest struct {
	Pagination entity.Pagination     `json:"pagination"`
	Filters    SearchProjectsFilters `json:"filters"`
}

// BuildEntitySearchProjectsRequest translates the portal's nested-filters
// request into entity-service's flat SearchProjectsRequest.
func BuildEntitySearchProjectsRequest(req SearchProjectsRequest) entity.SearchProjectsRequest {
	return entity.SearchProjectsRequest{
		Pagination:  req.Pagination,
		SearchQuery: req.Filters.SearchQuery,
	}
}

// ProjectAccount is the account summary embedded in ProjectDetails —
// SupportTier (not Tier) to match the frontend's read site
// (ProjectInformationCard.tsx: project?.account?.supportTier). HasAgent/
// HasKbReferences/ActivationDate are read as a fallback path (e.g.
// DashboardPage.tsx, SettingsAiAssistant.tsx: project.hasAgent ??
// project.account?.hasAgent) — not actually internal-only despite this
// struct's earlier doc comment claiming so.
type ProjectAccount struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	SupportTier     string  `json:"supportTier"`
	Region          *string `json:"region,omitempty"`
	HasAgent        bool    `json:"hasAgent"`
	HasKbReferences bool    `json:"hasKbReferences"`
	ActivationDate  *string `json:"activationDate,omitempty"`
	// The owning and technical contacts belong to the account, matching the
	// frontend's ProjectDetailsAccount type.
	DeactivationDate    *string `json:"deactivationDate,omitempty"`
	OwnerEmail          *string `json:"ownerEmail,omitempty"`
	TechnicalOwnerEmail *string `json:"technicalOwnerEmail,omitempty"`
}

// ProjectDetails is the portal's response for GET /projects/{id}.
//
// Deliberately excludes entity-service's SfID (Salesforce internal ID —
// must never reach the frontend) and the same closure/compliance fields
// dropped from ProjectSummary above.
type ProjectDetails struct {
	ID               string         `json:"id"`
	Account          ProjectAccount `json:"account"`
	Name             string         `json:"name"`
	Key              string         `json:"key"`
	SubscriptionType string         `json:"subscriptionType"`
	// Type is what the frontend actually reads (project.type.label); see
	// projectTypeRef. subscriptionType is kept for existing consumers.
	Type         *IDLabelRef `json:"type,omitempty"`
	StartDate    *string     `json:"startDate,omitempty"`
	EndDate      *string     `json:"endDate,omitempty"`
	CreatedOn    time.Time   `json:"createdOn"`
	UpdatedOn    time.Time   `json:"updatedOn"`
	ClosureState *string     `json:"closureState,omitempty"`

	// Query/onboarding entitlement balances and onboarding milestones, named as
	// the frontend's ProjectDetails type declares them
	// (features/project-hub/types/projects.ts). Customer-appropriate: these are
	// the customer's own entitlement and onboarding progress, and the portal has
	// always shown them — the Ballerina backend served all of them.
	//
	// omitempty on a pointer omits only nil, so a genuine zero balance still
	// serialises: "tracked, none remaining" stays distinguishable from
	// "not tracked".
	TotalQueryHours          *float64 `json:"totalQueryHours,omitempty"`
	ConsumedQueryHours       *float64 `json:"consumedQueryHours,omitempty"`
	RemainingQueryHours      *float64 `json:"remainingQueryHours,omitempty"`
	TotalOnboardingHours     *float64 `json:"totalOnboardingHours,omitempty"`
	ConsumedOnboardingHours  *float64 `json:"consumedOnboardingHours,omitempty"`
	RemainingOnboardingHours *float64 `json:"remainingOnboardingHours,omitempty"`
	GoLiveDate               *string  `json:"goLiveDate,omitempty"`
	GoLivePlanDate           *string  `json:"goLivePlanDate,omitempty"`
	OnboardingExpiryDate     *string  `json:"onboardingExpiryDate,omitempty"`
	OnboardingStatus         *string  `json:"onboardingStatus,omitempty"`
}

// MapProjectDetails builds the portal response from entity-service's ProjectDetailsView.
func MapProjectDetails(p entity.ProjectDetailsView) ProjectDetails {
	return ProjectDetails{
		ID: p.ID,
		Account: ProjectAccount{
			ID:              p.Account.ID,
			Name:            p.Account.Name,
			SupportTier:     p.Account.Tier,
			Region:          p.Account.Region,
			HasAgent:        p.Account.AgentEnabled,
			HasKbReferences: p.Account.KbReferencesEnabled,
			ActivationDate:  DateOnly(p.Account.ActivationDate),

			DeactivationDate:    DateOnly(p.Account.DeactivationDate),
			OwnerEmail:          p.Account.OwnerEmail,
			TechnicalOwnerEmail: p.Account.TechnicalOwnerEmail,
		},
		Name:             p.Name,
		Key:              p.Key,
		SubscriptionType: p.SubscriptionType,
		Type:             projectTypeRef(p.SubscriptionType),
		StartDate:        DateOnlyValue(p.StartDate),
		EndDate:          DateOnlyValue(p.EndDate),
		CreatedOn:        p.CreatedOn,
		UpdatedOn:        p.UpdatedOn,
		ClosureState:     p.ClosureState,

		TotalQueryHours:          p.TotalQueryHours,
		ConsumedQueryHours:       p.ConsumedQueryHours,
		RemainingQueryHours:      p.RemainingQueryHours,
		TotalOnboardingHours:     p.TotalOnboardingHours,
		ConsumedOnboardingHours:  p.ConsumedOnboardingHours,
		RemainingOnboardingHours: p.RemainingOnboardingHours,
		GoLiveDate:               DateOnly(p.GoLiveDate),
		GoLivePlanDate:           DateOnly(p.GoLivePlanDate),
		OnboardingExpiryDate:     DateOnly(p.OnboardingExpiryDate),
		OnboardingStatus:         p.OnboardingStatus,
	}
}

// UpdatedProject is the portal's response to PATCH /projects/{id}: the project
// as it stands after its AI chat assistant settings were changed.
type UpdatedProject struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	HasAgent        *bool  `json:"hasAgent,omitempty"`
	HasKbReferences *bool  `json:"hasKbReferences,omitempty"`
}

// MapUpdatedProject converts entity-service's project to the portal's shape,
// so the frontend contract is this package's to change rather than being
// whatever entity-service happens to return.
func MapUpdatedProject(p entity.UpdatedProjectRef) UpdatedProject {
	return UpdatedProject{
		ID:              p.ID,
		Name:            p.Name,
		HasAgent:        p.HasAgent,
		HasKbReferences: p.HasKbReferences,
	}
}
