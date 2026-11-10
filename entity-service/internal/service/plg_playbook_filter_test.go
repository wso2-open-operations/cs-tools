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

// TestValidateOrganizationFiltersChecksPlaybookAnswers: the playbook answer
// filters every PLG search, trend and aggregate shares.
func TestValidateOrganizationFiltersChecksPlaybookAnswers(t *testing.T) {
	yes, no := true, false
	const task = "ELIGIBILITY_ASSESSMENT_OUTCOME"
	for _, c := range []struct {
		name    string
		filters domain.OrganizationSearchFilters
		want    string // "" for accepted
	}{
		{"answers", domain.OrganizationSearchFilters{PlaybookTask: task, PlaybookAnswers: []string{"MONITOR", " EXCLUDED "}}, ""},
		{"unanswered", domain.OrganizationSearchFilters{PlaybookTask: task, PlaybookUnanswered: &yes}, ""},
		{"answers, unanswered false", domain.OrganizationSearchFilters{PlaybookTask: task, PlaybookAnswers: []string{"MONITOR"}, PlaybookUnanswered: &no}, ""},
		{"a task on its own", domain.OrganizationSearchFilters{PlaybookTask: task}, ""},
		{"unanswered false on its own", domain.OrganizationSearchFilters{PlaybookUnanswered: &no}, ""},
		{"100-character codes", domain.OrganizationSearchFilters{PlaybookTask: strings.Repeat("T", 100), PlaybookAnswers: []string{strings.Repeat("A", 100)}}, ""},
		{"a blank task", domain.OrganizationSearchFilters{PlaybookTask: "  ", PlaybookAnswers: []string{"MONITOR"}}, "playbookTask must be 1 to 100 characters"},
		{"a 101-character task", domain.OrganizationSearchFilters{PlaybookTask: strings.Repeat("T", 101), PlaybookAnswers: []string{"MONITOR"}}, "playbookTask must be 1 to 100 characters"},
		{"an empty answer", domain.OrganizationSearchFilters{PlaybookTask: task, PlaybookAnswers: []string{""}}, "playbookAnswers entries must be 1 to 100 characters"},
		{"a blank answer", domain.OrganizationSearchFilters{PlaybookTask: task, PlaybookAnswers: []string{"MONITOR", " "}}, "playbookAnswers entries must be 1 to 100 characters"},
		{"a 101-character answer", domain.OrganizationSearchFilters{PlaybookTask: task, PlaybookAnswers: []string{strings.Repeat("A", 101)}}, "playbookAnswers entries must be 1 to 100 characters"},
		{"answers without a task", domain.OrganizationSearchFilters{PlaybookAnswers: []string{"MONITOR"}}, "need filters.playbookTask"},
		{"unanswered without a task", domain.OrganizationSearchFilters{PlaybookUnanswered: &yes}, "need filters.playbookTask"},
		{"answers and unanswered", domain.OrganizationSearchFilters{PlaybookTask: task, PlaybookAnswers: []string{"MONITOR"}, PlaybookUnanswered: &yes}, "cannot be combined"},
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

// registrationAnswerRepo answers both registration groupings and records
// which one was asked; every other method is the nil interface's.
type registrationAnswerRepo struct {
	repository.OrgPlatformRepository
	asked     []string
	req       domain.SearchRegistrationsRequest
	maxGroups int
}

func (r *registrationAnswerRepo) AggregateRegistrationsByCountry(_ context.Context, req domain.SearchRegistrationsRequest, maxGroups int) (domain.AggregateResponse, error) {
	r.asked = append(r.asked, "country")
	r.req, r.maxGroups = req, maxGroups
	return domain.AggregateResponse{}, nil
}

func (r *registrationAnswerRepo) AggregateRegistrationsByPlaybookAnswer(_ context.Context, req domain.SearchRegistrationsRequest, maxGroups int) (domain.AggregateResponse, error) {
	r.asked = append(r.asked, "playbookAnswer")
	r.req, r.maxGroups = req, maxGroups
	return domain.AggregateResponse{TotalRecords: 3, Groups: []domain.AggregateBucket{{Key: "MONITOR", Label: "Monitor", Count: 3}}}, nil
}

func TestAggregateRegistrationsByPlaybookAnswerOrCountry(t *testing.T) {
	filters := domain.OrganizationSearchFilters{ProductCodes: []string{"IAM"}, PlaybookTask: "ELIGIBILITY_ASSESSMENT_OUTCOME"}
	for _, groupBy := range []string{"playbookAnswer", "country"} {
		repo := &registrationAnswerRepo{}
		if _, err := (&pairingService{repo: repo}).AggregateRegistrations(context.Background(),
			domain.AggregateRegistrationsRequest{UnacknowledgedOnly: true, Filters: filters, GroupBy: groupBy, MaxGroups: 6}); err != nil {
			t.Fatalf("%s: %v", groupBy, err)
		}
		want := domain.SearchRegistrationsRequest{UnacknowledgedOnly: true, Filters: filters}
		if !reflect.DeepEqual(repo.asked, []string{groupBy}) || !reflect.DeepEqual(repo.req, want) || repo.maxGroups != 6 {
			t.Errorf("groupBy %s: asked %v with %+v / %d, want %s once with %+v and 6", groupBy, repo.asked, repo.req, repo.maxGroups, groupBy, want)
		}
	}
}

func TestAggregateRegistrationsByPlaybookAnswerRefusesABadRequest(t *testing.T) {
	yes := true
	for name, req := range map[string]domain.AggregateRegistrationsRequest{
		"no task":                {GroupBy: "playbookAnswer"},
		"a blank task":           {GroupBy: "playbookAnswer", Filters: domain.OrganizationSearchFilters{PlaybookTask: "  "}},
		"an unknown groupBy":     {GroupBy: "playbookAnswers", Filters: domain.OrganizationSearchFilters{PlaybookTask: "T"}},
		"too many groups":        {GroupBy: "playbookAnswer", MaxGroups: 51, Filters: domain.OrganizationSearchFilters{PlaybookTask: "T"}},
		"answers and unanswered": {GroupBy: "playbookAnswer", Filters: domain.OrganizationSearchFilters{PlaybookTask: "T", PlaybookAnswers: []string{"A"}, PlaybookUnanswered: &yes}},
	} {
		repo := &registrationAnswerRepo{}
		_, err := (&pairingService{repo: repo}).AggregateRegistrations(context.Background(), req)
		var verr *apierror.ValidationError
		if !errors.As(err, &verr) {
			t.Errorf("%s: err = %v, want a validation error", name, err)
		}
		if len(repo.asked) != 0 {
			t.Errorf("%s: the repository was asked %v", name, repo.asked)
		}
	}
}
