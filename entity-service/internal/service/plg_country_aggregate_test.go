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

// countryAggregateRepo answers both organisation groupings and records which
// one was asked; every other method is the nil interface's (never reached).
type countryAggregateRepo struct {
	repository.OrganizationRepository
	asked     []string
	filters   domain.OrganizationSearchFilters
	maxGroups int
}

func (r *countryAggregateRepo) AggregateByOwner(_ context.Context, filters domain.OrganizationSearchFilters, maxGroups int) (domain.AggregateResponse, error) {
	r.asked = append(r.asked, "owner")
	r.filters, r.maxGroups = filters, maxGroups
	return domain.AggregateResponse{}, nil
}

func (r *countryAggregateRepo) AggregateByCountry(_ context.Context, filters domain.OrganizationSearchFilters, maxGroups int) (domain.AggregateResponse, error) {
	r.asked = append(r.asked, "country")
	r.filters, r.maxGroups = filters, maxGroups
	return domain.AggregateResponse{TotalRecords: 1, Groups: []domain.AggregateBucket{{Key: "India", Label: "India", Count: 1}}}, nil
}

// registrationCountryRepo answers AggregateRegistrationsByCountry and records
// what it was asked.
type registrationCountryRepo struct {
	repository.OrgPlatformRepository
	calls     int
	req       domain.SearchRegistrationsRequest
	maxGroups int
}

func (r *registrationCountryRepo) AggregateRegistrationsByCountry(_ context.Context, req domain.SearchRegistrationsRequest, maxGroups int) (domain.AggregateResponse, error) {
	r.calls++
	r.req, r.maxGroups = req, maxGroups
	return domain.AggregateResponse{TotalRecords: 2, Groups: []domain.AggregateBucket{{Key: "unknown", Label: "Unknown", Count: 2}}}, nil
}

func TestAggregateOrganizationsByCountryOrOwner(t *testing.T) {
	filters := domain.OrganizationSearchFilters{ProductCodes: []string{"IAM"}, CountryNames: []string{"India"}}
	for _, groupBy := range []string{"country", "owner"} {
		repo := &countryAggregateRepo{}
		if _, err := (&organizationService{repo: repo}).Aggregate(context.Background(),
			domain.AggregateOrganizationsRequest{Filters: filters, GroupBy: groupBy, MaxGroups: 8}); err != nil {
			t.Fatalf("%s: %v", groupBy, err)
		}
		if !reflect.DeepEqual(repo.asked, []string{groupBy}) || !reflect.DeepEqual(repo.filters, filters) || repo.maxGroups != 8 {
			t.Errorf("groupBy %s: asked %v with %+v / %d, want %s once with the filters and 8", groupBy, repo.asked, repo.filters, repo.maxGroups, groupBy)
		}
	}
}

func TestAggregateRegistrationsPassesTheRequestOn(t *testing.T) {
	repo := &registrationCountryRepo{}
	yes := true
	filters := domain.OrganizationSearchFilters{ProductCodes: []string{"IAM"}, CountryUnknown: &yes}

	got, err := (&pairingService{repo: repo}).AggregateRegistrations(context.Background(),
		domain.AggregateRegistrationsRequest{UnacknowledgedOnly: true, Filters: filters, GroupBy: "country", MaxGroups: 5})
	if err != nil {
		t.Fatal(err)
	}
	want := domain.SearchRegistrationsRequest{UnacknowledgedOnly: true, Filters: filters}
	if repo.calls != 1 || !reflect.DeepEqual(repo.req, want) || repo.maxGroups != 5 {
		t.Errorf("repo asked %d times with %+v / %d, want once with %+v and 5", repo.calls, repo.req, repo.maxGroups, want)
	}
	if got.TotalRecords != 2 {
		t.Errorf("got %+v, want the repository's answer", got)
	}
}

func TestAggregateRegistrationsRefusesABadRequest(t *testing.T) {
	yes := true
	for name, req := range map[string]domain.AggregateRegistrationsRequest{
		"no groupBy":         {},
		"by owner":           {GroupBy: "owner"},
		"a negative maximum": {GroupBy: "country", MaxGroups: -1},
		"too many groups":    {GroupBy: "country", MaxGroups: 51},
		"an unknown stage":   {GroupBy: "country", Filters: domain.OrganizationSearchFilters{LifecycleStages: []domain.LifecycleStage{"WON"}}},
		"a blank country":    {GroupBy: "country", Filters: domain.OrganizationSearchFilters{CountryNames: []string{" "}}},
		"a country and none": {GroupBy: "country", Filters: domain.OrganizationSearchFilters{CountryNames: []string{"India"}, CountryUnknown: &yes}},
	} {
		repo := &registrationCountryRepo{}
		_, err := (&pairingService{repo: repo}).AggregateRegistrations(context.Background(), req)
		var verr *apierror.ValidationError
		if !errors.As(err, &verr) {
			t.Errorf("%s: err = %v, want a validation error", name, err)
		}
		if repo.calls != 0 {
			t.Errorf("%s: the repository was queried", name)
		}
	}
}

// TestValidateOrganizationFiltersChecksCountries: the country filters every
// PLG search, trend and aggregate shares.
func TestValidateOrganizationFiltersChecksCountries(t *testing.T) {
	yes, no := true, false
	for _, c := range []struct {
		name    string
		filters domain.OrganizationSearchFilters
		want    string // "" for accepted
	}{
		{"countries", domain.OrganizationSearchFilters{CountryNames: []string{"Sri Lanka", " india "}}, ""},
		{"no country", domain.OrganizationSearchFilters{CountryUnknown: &yes}, ""},
		{"countries, countryUnknown false", domain.OrganizationSearchFilters{CountryNames: []string{"India"}, CountryUnknown: &no}, ""},
		{"a 100-character name", domain.OrganizationSearchFilters{CountryNames: []string{strings.Repeat("a", 100)}}, ""},
		{"an empty name", domain.OrganizationSearchFilters{CountryNames: []string{""}}, "countryNames entries must be 1 to 100 characters"},
		{"a blank name", domain.OrganizationSearchFilters{CountryNames: []string{"India", "  "}}, "countryNames entries must be 1 to 100 characters"},
		{"a 101-character name", domain.OrganizationSearchFilters{CountryNames: []string{strings.Repeat("a", 101)}}, "countryNames entries must be 1 to 100 characters"},
		{"a country and none", domain.OrganizationSearchFilters{CountryNames: []string{"India"}, CountryUnknown: &yes}, "cannot be combined"},
	} {
		err := validateOrganizationFilters(&c.filters)
		switch {
		case c.want == "" && err != nil:
			t.Errorf("%s: err = %v, want accepted", c.name, err)
		case c.want != "" && (err == nil || !strings.Contains(err.Error(), c.want)):
			t.Errorf("%s: err = %v, want one mentioning %q", c.name, err, c.want)
		}
	}
}
