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

type fakeTeamRepo struct {
	exists     bool
	existsErr  error
	members    []repository.TeamMemberRow
	membersErr error

	teams      []domain.Team
	teamsTotal int
	teamsErr   error
	gotQuery   string
	gotLimit   int
	gotOffset  int
}

func (f *fakeTeamRepo) TeamExists(context.Context, string) (bool, error) {
	return f.exists, f.existsErr
}

func (f *fakeTeamRepo) GetTeamMembers(context.Context, string) ([]repository.TeamMemberRow, error) {
	return f.members, f.membersErr
}

func (f *fakeTeamRepo) SearchTeams(_ context.Context, q string, limit, offset int) ([]domain.Team, int, error) {
	f.gotQuery, f.gotLimit, f.gotOffset = q, limit, offset
	return f.teams, f.teamsTotal, f.teamsErr
}

const testTeamID = "11111111-1111-1111-1111-111111111111"

func TestTeamService_GetTeamMembers_RequiresAuthentication(t *testing.T) {
	svc := NewTeamService(&fakeTeamRepo{exists: true})

	_, err := svc.GetTeamMembers(contextWithUserIDToken(""), testTeamID)

	var ue *apierror.UnauthorizedError
	if !errors.As(err, &ue) {
		t.Fatalf("expected UnauthorizedError for a tokenless request, got %v", err)
	}
}

func TestTeamService_GetTeamMembers_NotFound(t *testing.T) {
	svc := NewTeamService(&fakeTeamRepo{exists: false})

	_, err := svc.GetTeamMembers(contextWithUserIDToken("tok"), testTeamID)

	var nf *apierror.NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("expected NotFoundError for a missing team, got %v", err)
	}
}

func TestTeamService_GetTeamMembers_ReturnsRoster(t *testing.T) {
	role := "lead"
	email := "lead@wso2.com"
	repo := &fakeTeamRepo{
		exists: true,
		members: []repository.TeamMemberRow{
			{UserID: "u1", Name: "A Lead", Email: &email, Role: role},
		},
	}
	svc := NewTeamService(repo)

	resp, err := svc.GetTeamMembers(contextWithUserIDToken("tok"), testTeamID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Members) != 1 || resp.Members[0].ID != "u1" || resp.Members[0].Role == nil || *resp.Members[0].Role != "lead" {
		t.Fatalf("unexpected members: %+v", resp.Members)
	}
}

func TestTeamService_SearchTeams_PassesQueryAndPagination(t *testing.T) {
	repo := &fakeTeamRepo{
		teams:      []domain.Team{{ID: testTeamID, Name: "apollo", Type: "cre"}},
		teamsTotal: 7,
	}
	svc := NewTeamService(repo)

	resp, err := svc.SearchTeams(context.Background(), domain.SearchTeamsRequest{
		Filters:    &domain.SearchTeamsFilters{SearchQuery: "apo"},
		Pagination: domain.Pagination{Limit: 5, Offset: 10},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.gotQuery != "apo" || repo.gotLimit != 5 || repo.gotOffset != 10 {
		t.Fatalf("repo got (%q, %d, %d), want (apo, 5, 10)", repo.gotQuery, repo.gotLimit, repo.gotOffset)
	}
	if resp.Total != 7 || len(resp.Teams) != 1 || resp.Teams[0].Name != "apollo" || resp.Limit != 5 || resp.Offset != 10 {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestTeamService_SearchTeams_RepoError(t *testing.T) {
	boom := errors.New("boom")
	svc := NewTeamService(&fakeTeamRepo{teamsErr: boom})

	if _, err := svc.SearchTeams(context.Background(), domain.SearchTeamsRequest{}); !errors.Is(err, boom) {
		t.Fatalf("expected repo error, got %v", err)
	}
}
