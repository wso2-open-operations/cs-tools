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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package service

import (
	"context"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// recordingProjectRepo is a repository.ProjectRepository whose SearchProjects
// records the request it was called with — used to prove SearchProjects's
// exclude filters reach the repository unchanged, without needing a real
// Postgres connection.
type recordingProjectRepo struct {
	called bool
	gotReq domain.SearchProjectsRequest
	// toReturn lets a test control what SearchProjects hands back to the
	// service layer, to prove a repo-returned field survives the
	// domain.Project -> domain.ProjectView mapping (see
	// TestSearchProjects_ClosureStateReachesResponse below). Left nil,
	// SearchProjects returns no rows, matching every test above this one.
	toReturn []domain.Project
}

func (r *recordingProjectRepo) SearchProjects(_ context.Context, req domain.SearchProjectsRequest, _ repository.SearchScope) ([]domain.Project, int, error) {
	r.called = true
	r.gotReq = req
	return r.toReturn, len(r.toReturn), nil
}

func (r *recordingProjectRepo) GetProjectByID(context.Context, string, repository.SearchScope) (domain.ProjectDetailsView, error) {
	panic("GetProjectByID: not exercised by these tests")
}

func (r *recordingProjectRepo) UpdateProject(context.Context, string, domain.ProjectUpdateRequest, string) (domain.ProjectUpdateResult, error) {
	panic("UpdateProject: not exercised by these tests")
}

func (r *recordingProjectRepo) SalesforceClosureState(context.Context, string) (*string, *string, error) {
	panic("SalesforceClosureState: not exercised by these tests")
}

// TestSearchProjectsExcludeClosureStatesAndProjectKeysPassThrough locks in
// that ExcludeClosureStates, ExcludeProjectKeys, and ExcludeSubscriptionTypes
// (individually, and together) are NOT rejected — they all map onto real
// columns/joins (wso2_closure_state, key, and project_type.name via
// project.project_type_id for the last one) and reach the repository
// unchanged, for ProjectRepository.SearchProjects to apply as SQL filters.
func TestSearchProjectsExcludeClosureStatesAndProjectKeysPassThrough(t *testing.T) {
	tests := []struct {
		name    string
		req     domain.SearchProjectsRequest
		wantErr bool
	}{
		{
			name: "excludeClosureStates alone",
			req:  domain.SearchProjectsRequest{ExcludeClosureStates: []string{"Restricted", "Suspended"}},
		},
		{
			name: "excludeProjectKeys alone",
			req:  domain.SearchProjectsRequest{ExcludeProjectKeys: []string{"APEXIA"}},
		},
		{
			name: "both together",
			req: domain.SearchProjectsRequest{
				ExcludeClosureStates: []string{"Restricted"},
				ExcludeProjectKeys:   []string{"APEXIA"},
			},
		},
		{
			name: "excludeSubscriptionTypes alone",
			req:  domain.SearchProjectsRequest{ExcludeSubscriptionTypes: []domain.SubscriptionType{domain.SubscriptionTypeCloudSupport}},
		},
		{
			name: "all three together",
			req: domain.SearchProjectsRequest{
				ExcludeClosureStates:     []string{"Restricted"},
				ExcludeProjectKeys:       []string{"APEXIA"},
				ExcludeSubscriptionTypes: []domain.SubscriptionType{domain.SubscriptionTypeCloudSupport, domain.SubscriptionTypeCloudEvaluationSupport},
			},
		},
		{
			name: "neither set",
			req:  domain.SearchProjectsRequest{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &recordingProjectRepo{}
			svc := NewProjectService(repo, alwaysUnrestrictedAccess{})

			_, err := svc.SearchProjects(t.Context(), tt.req)
			if err != nil {
				t.Fatalf("SearchProjects(%+v) unexpected error: %v", tt.req, err)
			}
			if !repo.called {
				t.Fatal("expected the repository to be called, it wasn't")
			}
			if len(repo.gotReq.ExcludeClosureStates) != len(tt.req.ExcludeClosureStates) ||
				len(repo.gotReq.ExcludeProjectKeys) != len(tt.req.ExcludeProjectKeys) ||
				len(repo.gotReq.ExcludeSubscriptionTypes) != len(tt.req.ExcludeSubscriptionTypes) {
				t.Fatalf("repository received %+v, want the same exclude filters as %+v", repo.gotReq, tt.req)
			}
		})
	}
}

// TestSearchProjects_ClosureStateReachesResponse is the regression guard for
// a real bug: domain.Project already carried ClosureState from the
// repository (project.wso2_closure_state), but the service layer's
// domain.Project -> domain.ProjectView mapping never copied it across, so
// every search result's closureState silently came back null regardless of
// what the repository returned — the same class of bug this file's own
// comment on StartDate above already describes and fixes for that field.
func TestSearchProjects_ClosureStateReachesResponse(t *testing.T) {
	closureState := "Suspended"
	repo := &recordingProjectRepo{toReturn: []domain.Project{{ID: "p1", ProjectClosureFields: domain.ProjectClosureFields{ClosureState: &closureState}}}}
	svc := NewProjectService(repo, alwaysUnrestrictedAccess{})

	resp, err := svc.SearchProjects(t.Context(), domain.SearchProjectsRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Projects) != 1 || resp.Projects[0].ClosureState == nil || *resp.Projects[0].ClosureState != closureState {
		t.Fatalf("Projects[0].ClosureState = %+v, want %q", resp.Projects, closureState)
	}
}

// TestSearchProjects_GoLiveDateReachesResponse pins that domain.Project's
// GoLiveDate (project.onboarding_go_live_date, now selected by SearchProjects
// alongside OnboardingStatus) reaches the mapped ProjectView -- the same class
// of "repository returns it, service layer silently drops it" bug
// TestSearchProjects_ClosureStateReachesResponse already guards for ClosureState.
func TestSearchProjects_GoLiveDateReachesResponse(t *testing.T) {
	goLive := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	repo := &recordingProjectRepo{toReturn: []domain.Project{{ID: "p1", GoLiveDate: &goLive}, {ID: "p2"}}}
	svc := NewProjectService(repo, alwaysUnrestrictedAccess{})

	resp, err := svc.SearchProjects(t.Context(), domain.SearchProjectsRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Projects) != 2 || resp.Projects[0].GoLiveDate == nil || !resp.Projects[0].GoLiveDate.Equal(goLive) {
		t.Fatalf("Projects[0].GoLiveDate = %+v, want %v", resp.Projects, goLive)
	}
	if resp.Projects[1].GoLiveDate != nil {
		t.Fatalf("Projects[1].GoLiveDate = %v, want nil (no go-live date recorded)", resp.Projects[1].GoLiveDate)
	}
}

// TestSearchProjects_MapsParityFields pins the v1.0 fields the Postgres path now returns.
func TestSearchProjects_MapsParityFields(t *testing.T) {
	str := func(v string) *string { return &v }
	partner := true
	repo := &recordingProjectRepo{toReturn: []domain.Project{
		{
			ID: "p1", SfID: "a0X1", ActiveCasesCount: 3, OnboardingStatus: str("In-Progress"),
			Account: &domain.ProjectSearchAccountRef{ID: "a1", Name: "Acme", Region: str("EMEA"), SubRegion: str("UK"), IsPartner: &partner},
			ProjectClosureFields: domain.ProjectClosureFields{
				ClosureState: str("Suspended"), EndDateClosureState: str("Closed"),
				InvoiceDueDateClosureState: str("Pending Notified"), ComplianceViolationClosureState: str("Open"),
				ComplianceViolationDate: str("2026-09-01"),
			},
		},
		{ID: "p2"},
	}}
	svc := NewProjectService(repo, alwaysUnrestrictedAccess{})

	resp, err := svc.SearchProjects(t.Context(), domain.SearchProjectsRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := resp.Projects[0]
	if got.SfID == nil || *got.SfID != "a0X1" || got.ActiveCasesCount != 3 || *got.OnboardingStatus != "In-Progress" {
		t.Fatalf("sfId/activeCasesCount/onboardingStatus not mapped: %+v", got)
	}
	if got.Account == nil || got.Account.Name != "Acme" || *got.Account.SubRegion != "UK" || !*got.Account.IsPartner {
		t.Fatalf("account not mapped: %+v", got.Account)
	}
	if *got.InvoiceDueDateClosureState != "Pending Notified" || *got.ComplianceViolationDate != "2026-09-01" {
		t.Fatalf("closure fields not mapped: %+v", got.ProjectClosureFields)
	}
	if resp.Projects[1].SfID != nil || resp.Projects[1].Account != nil {
		t.Fatalf("empty sfId/account must map to null: %+v", resp.Projects[1])
	}
}

// TestSearchProjects_ValidatesV10Filters pins that closureStatus/sortBy/sortOrder accept
// exactly v1.0's values and that invalid ones never reach the repository.
func TestSearchProjects_ValidatesV10Filters(t *testing.T) {
	cases := []struct {
		name    string
		req     domain.SearchProjectsRequest
		wantErr bool
	}{
		{"closureStatus Suspended", domain.SearchProjectsRequest{ClosureStatus: "Suspended"}, false},
		{"closureStatus lowercase", domain.SearchProjectsRequest{ClosureStatus: "suspended"}, true},
		{"sort by endDate desc", domain.SearchProjectsRequest{SortBy: "endDate", SortOrder: "desc"}, false},
		{"sort by injected column", domain.SearchProjectsRequest{SortBy: "name; DROP TABLE project"}, true},
		{"bad sortOrder", domain.SearchProjectsRequest{SortBy: "endDate", SortOrder: "sideways"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &recordingProjectRepo{}
			_, err := NewProjectService(repo, alwaysUnrestrictedAccess{}).SearchProjects(t.Context(), tc.req)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if repo.called == tc.wantErr {
				t.Fatalf("repo.called = %v, want %v", repo.called, !tc.wantErr)
			}
		})
	}
}

// TestSearchProjects_OnboardingOwnerReachesResponse proves the owner survives
// the domain.Project -> domain.ProjectView mapping and that a project with no
// owner stays nil without affecting the total.
func TestSearchProjects_OnboardingOwnerReachesResponse(t *testing.T) {
	email := "jane.doe@example.com"
	owner := &domain.PersonRef{ID: "00000000-0000-0000-0000-000000000001", Name: "Jane Doe", Email: &email}
	repo := &recordingProjectRepo{toReturn: []domain.Project{{ID: "p1", OnboardingOwner: owner}, {ID: "p2"}}}
	svc := NewProjectService(repo, alwaysUnrestrictedAccess{})

	resp, err := svc.SearchProjects(t.Context(), domain.SearchProjectsRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Total != 2 || len(resp.Projects) != 2 {
		t.Fatalf("total/len = %d/%d, want 2/2", resp.Total, len(resp.Projects))
	}
	if resp.Projects[0].OnboardingOwner == nil || resp.Projects[0].OnboardingOwner.Name != "Jane Doe" {
		t.Errorf("Projects[0].OnboardingOwner = %+v, want Jane Doe", resp.Projects[0].OnboardingOwner)
	}
	if resp.Projects[1].OnboardingOwner != nil {
		t.Errorf("Projects[1].OnboardingOwner = %+v, want nil", resp.Projects[1].OnboardingOwner)
	}
}
