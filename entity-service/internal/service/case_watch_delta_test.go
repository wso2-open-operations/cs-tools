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
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	deltaUserA = "11111111-1111-1111-1111-111111111111"
	deltaUserB = "22222222-2222-2222-2222-222222222222"
)

func deltaService(t *testing.T, repo *stubCaseRepo) CaseService {
	t.Helper()
	userRepo := stubUserRepo{getUserByEmail: func(context.Context, string) (domain.User, error) {
		return domain.User{ID: testDeploymentUUID, Email: "jane.doe@example.com"}, nil
	}}
	return NewCaseService(repo, userRepo, nil, alwaysUnrestrictedAccess{}, nil)
}

// An add/remove request reaches the repository as a delta and never touches
// the replace path: that is what keeps a stale caller from dropping someone
// else's watcher.
func TestCaseService_UpdateCase_WatchDelta_SendsDeltaNotReplace(t *testing.T) {
	var gotAdd, gotRemove []string
	repo := &stubCaseRepo{
		getCaseByID: func(context.Context, string, repository.SearchScope) (domain.CaseView, error) {
			return domain.CaseView{}, nil // no project: membership check skipped
		},
		changeCaseWatchers: func(_ context.Context, _ string, add, remove []string, _ string) ([]domain.WatchListUser, time.Time, error) {
			gotAdd, gotRemove = add, remove
			return []domain.WatchListUser{{ID: deltaUserA}}, time.Now(), nil
		},
		// setCaseWatchList left nil: the stub panics if the replace path runs.
	}
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	resp, err := deltaService(t, repo).UpdateCase(ctx, domain.UpdateCaseRequest{
		ID: testDeploymentUUID, AddWatchers: []string{deltaUserA}, RemoveWatchers: []string{deltaUserB},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(gotAdd) != 1 || gotAdd[0] != deltaUserA || len(gotRemove) != 1 || gotRemove[0] != deltaUserB {
		t.Errorf("repo got add=%v remove=%v", gotAdd, gotRemove)
	}
	if len(resp.Case.WatchList) != 1 || resp.Case.WatchList[0].ID != deltaUserA {
		t.Errorf("response watch list = %+v, want the resulting list", resp.Case.WatchList)
	}
}

func TestCaseService_UpdateCase_WatchDelta_Refusals(t *testing.T) {
	list := []string{deltaUserA}
	cases := map[string]domain.UpdateCaseRequest{
		"same user in both lists": {ID: testDeploymentUUID, AddWatchers: []string{deltaUserA}, RemoveWatchers: []string{deltaUserA}},
		"not a uuid":              {ID: testDeploymentUUID, AddWatchers: []string{"not-a-uuid"}},
		"with watchList":          {ID: testDeploymentUUID, AddWatchers: []string{deltaUserA}, WatchList: &list},
	}
	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			repo := &stubCaseRepo{getCaseByID: func(context.Context, string, repository.SearchScope) (domain.CaseView, error) {
				return domain.CaseView{}, nil
			}}
			_, err := deltaService(t, repo).UpdateCase(context.Background(), req)
			var ve *apierror.ValidationError
			if !asValidationError(err, &ve) {
				t.Fatalf("expected a ValidationError, got %v", err)
			}
		})
	}
}

// Only a newly added user is checked against the project's contacts; a
// removal is never refused for membership.
func TestCaseService_UpdateCase_WatchDelta_RejectsNonProjectMemberOnAdd(t *testing.T) {
	const projectID = "6fa0b42d-1bfa-a694-a002-c9d3604bcb77"
	called := false
	repo := &stubCaseRepo{
		getCaseByID: func(context.Context, string, repository.SearchScope) (domain.CaseView, error) {
			return domain.CaseView{ProjectDetails: &domain.EntityRef{ID: projectID}}, nil
		},
		changeCaseWatchers: func(context.Context, string, []string, []string, string) ([]domain.WatchListUser, time.Time, error) {
			called = true
			return nil, time.Now(), nil
		},
	}
	contactRepo := &stubProjectContactRepo{
		getProjectContactByUserID: func(context.Context, string, string, string) (repository.ProjectContactRow, error) {
			return repository.ProjectContactRow{}, &apierror.NotFoundError{Msg: "contact not found on this project"}
		},
	}
	svc := NewCaseService(repo, stubUserRepo{getUserDetail: func(context.Context, string) (domain.UserDetail, error) {
		return domain.UserDetail{UserType: domain.UserTypeExternal, Email: "outsider@example.com"}, nil
	}}, nil, alwaysUnrestrictedAccess{}, contactRepo)

	_, err := svc.UpdateCase(context.Background(), domain.UpdateCaseRequest{ID: testDeploymentUUID, AddWatchers: []string{deltaUserA}})
	var ve *apierror.ValidationError
	if !asValidationError(err, &ve) {
		t.Fatalf("expected a ValidationError, got %v", err)
	}
	if called {
		t.Error("ChangeCaseWatchers was called despite a non-member being added")
	}
}
