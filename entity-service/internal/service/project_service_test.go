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
}

func (r *recordingProjectRepo) SearchProjects(_ context.Context, req domain.SearchProjectsRequest, _ repository.SearchScope) ([]domain.Project, int, error) {
	r.called = true
	r.gotReq = req
	return nil, 0, nil
}

func (r *recordingProjectRepo) GetProjectByID(context.Context, string, repository.SearchScope) (domain.ProjectDetailsView, error) {
	panic("GetProjectByID: not exercised by these tests")
}

func (r *recordingProjectRepo) UpdateProject(context.Context, string, domain.ProjectUpdateRequest, string) (domain.ProjectUpdateResult, error) {
	panic("UpdateProject: not exercised by these tests")
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
