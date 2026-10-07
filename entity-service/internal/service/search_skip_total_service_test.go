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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// The search services pass SkipTotal on to their repositories and hand the
// repository's total back unchanged. Each service rebuilds or copies its
// request on the way (the case service prepares a parsed copy), so a change
// there that drops the flag would put the COUNT back with no test failing;
// the repository-level test cannot see that.

// fakeTotal is what a real repository reports: the exact total normally, and
// domain.TotalNotComputed when the request asked to skip it.
func fakeTotal(skip bool) int {
	if skip {
		return domain.TotalNotComputed
	}
	return 42
}

type skipTotalIncidentRepo struct {
	*stubIncidentRepo
	seen *domain.SearchIncidentsRequest
}

func (r skipTotalIncidentRepo) SearchIncidents(_ context.Context, req domain.SearchIncidentsRequest, _, _, _, _ []string, _, _ *bool, _, _ *time.Time) ([]domain.SearchIncidentView, int, error) {
	*r.seen = req
	return nil, fakeTotal(req.SkipTotal), nil
}

type skipTotalChangeRequestRepo struct {
	*stubChangeRequestRepo
	seen *domain.SearchChangeRequestsRequest
}

func (r skipTotalChangeRequestRepo) SearchChangeRequests(_ context.Context, req domain.SearchChangeRequestsRequest, _, _ *time.Time, _ *string, _ []string) ([]domain.SearchChangeRequestView, int, error) {
	*r.seen = req
	return nil, fakeTotal(req.SkipTotal), nil
}

type skipTotalProblemRepo struct {
	*stubProblemRepo
	seen *domain.SearchProblemsRequest
}

func (r skipTotalProblemRepo) SearchProblems(_ context.Context, req domain.SearchProblemsRequest, _, _, _ []string) ([]domain.SearchProblemView, int, error) {
	*r.seen = req
	return nil, fakeTotal(req.SkipTotal), nil
}

type skipTotalConversationRepo struct {
	*stubConversationRepo
	seen *domain.SearchConversationsRequest
}

func (r skipTotalConversationRepo) SearchConversations(_ context.Context, req domain.SearchConversationsRequest, _ string) ([]domain.SearchConversationView, int, error) {
	*r.seen = req
	return nil, fakeTotal(req.SkipTotal), nil
}

func TestSearchServicesPassSkipTotalToTheRepository(t *testing.T) {
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	page := domain.Pagination{Limit: 5}

	searches := []struct {
		name string
		// run searches with the given flag and reports what the repository saw
		// and the total the service returned.
		run func(skip bool) (sawSkip bool, total int, err error)
	}{
		{"cases", func(skip bool) (bool, int, error) {
			var seen domain.SearchCasesRequest
			repo := &stubCaseRepo{searchCases: func(_ context.Context, req domain.SearchCasesRequest) ([]domain.SearchCaseView, int, error) {
				seen = req
				return nil, fakeTotal(req.SkipTotal), nil
			}}
			svc := NewCaseService(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil)
			res, err := svc.SearchCases(ctx, domain.SearchCasesRequest{Pagination: page, SkipTotal: skip})
			return seen.SkipTotal, res.Total, err
		}},
		{"incidents", func(skip bool) (bool, int, error) {
			var seen domain.SearchIncidentsRequest
			svc := NewIncidentService(skipTotalIncidentRepo{&stubIncidentRepo{}, &seen}, nil)
			res, err := svc.SearchIncidents(ctx, domain.SearchIncidentsRequest{Pagination: page, SkipTotal: skip})
			return seen.SkipTotal, res.Total, err
		}},
		{"change requests", func(skip bool) (bool, int, error) {
			var seen domain.SearchChangeRequestsRequest
			svc := NewChangeRequestService(skipTotalChangeRequestRepo{&stubChangeRequestRepo{}, &seen}, stubUserRepo{})
			res, err := svc.SearchChangeRequests(ctx, domain.SearchChangeRequestsRequest{Pagination: page, SkipTotal: skip})
			return seen.SkipTotal, res.Total, err
		}},
		{"problems", func(skip bool) (bool, int, error) {
			var seen domain.SearchProblemsRequest
			svc := NewProblemService(skipTotalProblemRepo{&stubProblemRepo{}, &seen})
			res, err := svc.SearchProblems(ctx, domain.SearchProblemsRequest{Pagination: page, SkipTotal: skip})
			return seen.SkipTotal, res.Total, err
		}},
		{"conversations", func(skip bool) (bool, int, error) {
			var seen domain.SearchConversationsRequest
			svc := NewConversationService(skipTotalConversationRepo{&stubConversationRepo{}, &seen})
			res, err := svc.SearchConversations(ctx, domain.SearchConversationsRequest{Pagination: page, SkipTotal: skip})
			return seen.SkipTotal, res.Total, err
		}},
	}

	for _, s := range searches {
		t.Run(s.name, func(t *testing.T) {
			sawSkip, total, err := s.run(true)
			if err != nil {
				t.Fatalf("search with SkipTotal: %v", err)
			}
			if !sawSkip {
				t.Errorf("the service dropped SkipTotal before the repository")
			}
			if total != domain.TotalNotComputed {
				t.Errorf("total = %d, want the repository's %d passed through", total, domain.TotalNotComputed)
			}

			sawSkip, total, err = s.run(false)
			if err != nil {
				t.Fatalf("search without SkipTotal: %v", err)
			}
			if sawSkip {
				t.Errorf("the service set SkipTotal on a request that did not ask for it")
			}
			if total != 42 {
				t.Errorf("total = %d, want the exact total 42", total)
			}
		})
	}
}
