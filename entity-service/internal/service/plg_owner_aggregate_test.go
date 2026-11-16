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
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// ownerAggregateRepo answers AggregateByOwner and records what it was asked;
// every other method is the nil interface's (never reached).
type ownerAggregateRepo struct {
	repository.OrganizationRepository
	calls     int
	filters   domain.OrganizationSearchFilters
	maxGroups int
}

func (r *ownerAggregateRepo) AggregateByOwner(_ context.Context, filters domain.OrganizationSearchFilters, maxGroups int) (domain.AggregateResponse, error) {
	r.calls++
	r.filters, r.maxGroups = filters, maxGroups
	return domain.AggregateResponse{TotalRecords: 1, Groups: []domain.AggregateBucket{{Key: "unowned", Label: "Unowned", Count: 1}}}, nil
}

func TestAggregateOrganizationsPassesTheFiltersOn(t *testing.T) {
	repo := &ownerAggregateRepo{}
	svc := &organizationService{repo: repo}
	filters := domain.OrganizationSearchFilters{ProductCodes: []string{"IAM"}, SubscriptionTiers: []domain.SubscriptionTier{domain.TierTrial}}

	got, err := svc.Aggregate(context.Background(), domain.AggregateOrganizationsRequest{Filters: filters, GroupBy: "owner", MaxGroups: 10})
	if err != nil {
		t.Fatal(err)
	}
	if repo.calls != 1 || !reflect.DeepEqual(repo.filters, filters) || repo.maxGroups != 10 {
		t.Errorf("repo asked %d times with %+v / %d, want once with the filters and 10", repo.calls, repo.filters, repo.maxGroups)
	}
	if got.TotalRecords != 1 || len(got.Groups) != 1 {
		t.Errorf("got %+v, want the repository's answer", got)
	}
}

func TestAggregateOrganizationsRefusesABadRequest(t *testing.T) {
	for name, req := range map[string]domain.AggregateOrganizationsRequest{
		"no groupBy":             {},
		"another groupBy":        {GroupBy: "lifecycleStage"},
		"a negative maximum":     {GroupBy: "owner", MaxGroups: -1},
		"too many groups":        {GroupBy: "owner", MaxGroups: 51},
		"an unknown tier":        {GroupBy: "owner", Filters: domain.OrganizationSearchFilters{SubscriptionTiers: []domain.SubscriptionTier{"ENTERPRISE"}}},
		"an owner id not a UUID": {GroupBy: "owner", Filters: domain.OrganizationSearchFilters{OwnerIDs: []string{"me"}}},
	} {
		repo := &ownerAggregateRepo{}
		_, err := (&organizationService{repo: repo}).Aggregate(context.Background(), req)
		var verr *apierror.ValidationError
		if !errors.As(err, &verr) {
			t.Errorf("%s: err = %v, want a validation error", name, err)
		}
		if repo.calls != 0 {
			t.Errorf("%s: the repository was queried", name)
		}
	}
}
