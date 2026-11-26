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
	"reflect"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

type fakeProjectAggregateRepo struct {
	buckets  []domain.AggregateBucket
	keyless  int
	called   bool
	gotReq   domain.AggregateProjectsRequest
	gotScope repository.SearchScope
}

func (f *fakeProjectAggregateRepo) AggregateProjects(_ context.Context, req domain.AggregateProjectsRequest, scope repository.SearchScope) ([]domain.AggregateBucket, int, error) {
	f.called, f.gotReq, f.gotScope = true, req, scope
	return f.buckets, f.keyless, nil
}

func bucket(key string, n int) domain.AggregateBucket {
	return domain.AggregateBucket{Key: key, Label: key, Count: n}
}

func TestAggregateProjects_RejectsBadRequests(t *testing.T) {
	cases := []struct {
		name string
		req  domain.AggregateProjectsRequest
		want string
	}{
		{"no groupBy", domain.AggregateProjectsRequest{}, "groupBy is required"},
		{"unknown groupBy", domain.AggregateProjectsRequest{GroupBy: "account"}, `groupBy: "account" is not a valid grouping`},
		{"negative maxGroups", domain.AggregateProjectsRequest{GroupBy: "region", MaxGroups: -1}, "maxGroups must be between 0 and 50"},
		{"maxGroups too large", domain.AggregateProjectsRequest{GroupBy: "region", MaxGroups: 51}, "maxGroups must be between 0 and 50"},
		{"bad team id", domain.AggregateProjectsRequest{GroupBy: "onboardingOwner",
			Filters: domain.SearchProjectsRequest{OnboardingOwnerTeamIDs: []string{"customer_onboarding"}}}, "onboardingOwnerTeamIds contains invalid UUID"},
		{"bad onboarding status", domain.AggregateProjectsRequest{GroupBy: "region",
			Filters: domain.SearchProjectsRequest{ClosureStates: []string{"Closed"}}}, `closureStates: "Closed" is not a valid closure state`},
		{"arrTodayGte", domain.AggregateProjectsRequest{GroupBy: "region",
			Filters: domain.SearchProjectsRequest{ArrTodayGte: "1"}}, "arrTodayGte is not supported"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeProjectAggregateRepo{}
			_, err := NewProjectAggregateService(repo, alwaysUnrestrictedAccess{}).AggregateProjects(t.Context(), tc.req)
			var ve *apierror.ValidationError
			if !errors.As(err, &ve) || !strings.Contains(ve.Msg, tc.want) {
				t.Fatalf("err = %v, want 400 containing %q", err, tc.want)
			}
			if repo.called {
				t.Fatal("repository must not be called for an invalid request")
			}
		})
	}
}

func TestAggregateProjects_CapsGroupsAndFoldsTheRestIntoOthers(t *testing.T) {
	repo := &fakeProjectAggregateRepo{
		buckets: []domain.AggregateBucket{bucket("a", 5), bucket("b", 3), bucket("c", 2), bucket("d", 1)},
		keyless: 4,
	}
	resp, err := NewProjectAggregateService(repo, alwaysUnrestrictedAccess{}).AggregateProjects(t.Context(),
		domain.AggregateProjectsRequest{GroupBy: "onboardingOwner", MaxGroups: 2})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !reflect.DeepEqual(resp.Groups, []domain.AggregateBucket{bucket("a", 5), bucket("b", 3)}) {
		t.Fatalf("groups = %+v", resp.Groups)
	}
	// 2 + 1 beyond the cap, plus 4 projects with no owner.
	if resp.OthersCount != 7 || resp.TotalRecords != 15 {
		t.Fatalf("others/total = %d/%d, want 7/15", resp.OthersCount, resp.TotalRecords)
	}
}

func TestAggregateProjects_DefaultsToTenGroupsAndNeverReturnsNullGroups(t *testing.T) {
	var many []domain.AggregateBucket
	for i := 0; i < 12; i++ {
		many = append(many, bucket(string(rune('a'+i)), 1))
	}
	resp, err := NewProjectAggregateService(&fakeProjectAggregateRepo{buckets: many}, alwaysUnrestrictedAccess{}).
		AggregateProjects(t.Context(), domain.AggregateProjectsRequest{GroupBy: "region"})
	if err != nil || len(resp.Groups) != 10 || resp.OthersCount != 2 || resp.TotalRecords != 12 {
		t.Fatalf("resp = %+v, err = %v", resp, err)
	}
	empty, err := NewProjectAggregateService(&fakeProjectAggregateRepo{}, alwaysUnrestrictedAccess{}).
		AggregateProjects(t.Context(), domain.AggregateProjectsRequest{GroupBy: "region"})
	if err != nil || empty.Groups == nil || len(empty.Groups) != 0 || empty.TotalRecords != 0 {
		t.Fatalf("empty resp = %+v, err = %v", empty, err)
	}
}

func TestAggregateProjects_PassesFiltersAndTheCallersScope(t *testing.T) {
	scope := AccessScope{ProjectIDs: []string{"p1"}}
	repo := &fakeProjectAggregateRepo{}
	req := domain.AggregateProjectsRequest{GroupBy: "onboardingStatus", Filters: domain.SearchProjectsRequest{
		OnboardingOwnerTeamIDs: []string{"11111111-1111-1111-1111-111111111111"}, Regions: []string{"EU"},
	}}
	if _, err := NewProjectAggregateService(repo, stubAccessServiceWithScope{scope: scope}).AggregateProjects(t.Context(), req); err != nil {
		t.Fatalf("err = %v", err)
	}
	if !reflect.DeepEqual(repo.gotReq, req) || !reflect.DeepEqual(repo.gotScope, scope) {
		t.Fatalf("repo got %+v / %+v", repo.gotReq, repo.gotScope)
	}
}
