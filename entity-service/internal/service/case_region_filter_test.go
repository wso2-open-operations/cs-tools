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
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// regionFilter is a request's region filter entry with the given op.
func regionFilter(op string, regions ...string) domain.CaseFieldFilter {
	return domain.CaseFieldFilter{Field: "region", Op: op, Values: regions}
}

// TestParseCaseFieldFilters_Regions: region in keeps regions and notIn
// leaves them out, alone or together; several entries of one op add up; any
// other op, no values, a blank value and an anyOf branch are refused.
func TestParseCaseFieldFilters_Regions(t *testing.T) {
	now := time.Date(2026, time.November, 12, 0, 0, 0, 0, time.UTC)
	p, err := ParseCaseFieldFilters([]domain.CaseFieldFilter{
		regionFilter("in", "APAC"), regionFilter("in", "EMEA"), regionFilter("notIn", "North America"),
	}, "", nil, now)
	if err != nil {
		t.Fatalf("ParseCaseFieldFilters: %v", err)
	}
	if !reflect.DeepEqual(p.Regions, []string{"APAC", "EMEA"}) || !reflect.DeepEqual(p.ExcludeRegions, []string{"North America"}) {
		t.Errorf("Regions = %v, ExcludeRegions = %v, want [APAC EMEA] kept and [North America] left out", p.Regions, p.ExcludeRegions)
	}

	for _, tc := range []struct {
		name    string
		filters []domain.CaseFieldFilter
		want    string
	}{
		{"eq", []domain.CaseFieldFilter{regionFilter("eq", "APAC")}, `field "region" does not support op "eq"`},
		{"no values", []domain.CaseFieldFilter{regionFilter("in")}, `field "region" op "in" requires a non-empty values array`},
		{"a blank value", []domain.CaseFieldFilter{regionFilter("notIn", "APAC", "  ")}, "region values must not be blank"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseCaseFieldFilters(tc.filters, "", nil, now)
			var ve *apierror.ValidationError
			if !errors.As(err, &ve) || !strings.Contains(ve.Msg, tc.want) {
				t.Errorf("err = %v, want a validation error containing %q", err, tc.want)
			}
		})
	}

	for _, op := range []string{"in", "notIn"} {
		_, err := ParseCaseFieldFilterGroups([]domain.CaseFilterBranch{{Filters: []domain.CaseFieldFilter{regionFilter(op, "APAC")}},
			{Filters: []domain.CaseFieldFilter{{Field: "state", Op: "in", Values: []string{"open"}}}}})
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) || ve.Msg != `anyOf: field "region" is not supported inside an OR group` {
			t.Errorf("anyOf branch (%s): err = %v, want the OR group refusal", op, err)
		}
	}
}

// TestSNCaseService_RefusesRegions: the other data source has no account
// region on its case search, so it refuses a region filter before any call,
// for a search and an aggregate, whichever op.
func TestSNCaseService_RefusesRegions(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(http.ResponseWriter, *http.Request) {
		t.Fatalf("the data source must not be called")
	})
	svc := NewServiceNowCaseService(newTestSNClient(t, mux), nil, nil, nil, nil, "", nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	const refused = `field "region" is not supported by this data source`

	for _, op := range []string{"in", "notIn"} {
		filters := domain.SearchCasesFilters{Filters: []domain.CaseFieldFilter{regionFilter(op, "APAC")}}
		var ve *apierror.ValidationError
		_, err := svc.SearchCases(ctx, domain.SearchCasesRequest{Filters: filters, Pagination: domain.Pagination{Limit: 10}})
		if !errors.As(err, &ve) || ve.Msg != refused {
			t.Errorf("search (%s): err = %v, want the region refusal", op, err)
		}
		_, err = svc.AggregateCases(ctx, domain.AggregateCasesRequest{GroupBy: "state", Filters: filters})
		if !errors.As(err, &ve) || ve.Msg != refused {
			t.Errorf("aggregate (%s): err = %v, want the region refusal", op, err)
		}
	}
}

// TestCaseService_AggregateCases_ByRegion: the relational service passes a
// region grouping to the repository and returns its buckets as they are.
func TestCaseService_AggregateCases_ByRegion(t *testing.T) {
	repo := &stubCaseRepo{aggregateCases: func(_ context.Context, _ domain.SearchCasesRequest, groupBy string) ([]domain.AggregateBucket, error) {
		if groupBy != "region" {
			t.Errorf("groupBy = %q, want region", groupBy)
		}
		return []domain.AggregateBucket{{Key: "apac", Label: "APAC", Count: 4}, {Key: "emea", Label: "EMEA", Count: 1}}, nil
	}}
	svc := NewCaseService(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	resp, err := svc.AggregateCases(ctx, domain.AggregateCasesRequest{GroupBy: "region"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Groups) != 2 || resp.Groups[0].Key != "apac" || resp.Groups[0].Label != "APAC" || resp.TotalRecords != 5 {
		t.Errorf("got %+v, want the two region buckets and a total of 5", resp)
	}
}

// TestSNCaseService_AggregateCases_RefusesRegion: the other data source has
// no account region, so it refuses a region grouping before any call.
func TestSNCaseService_AggregateCases_RefusesRegion(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(http.ResponseWriter, *http.Request) {
		t.Fatalf("the data source must not be called")
	})
	svc := NewServiceNowCaseService(newTestSNClient(t, mux), nil, nil, nil, nil, "", nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	_, err := svc.AggregateCases(ctx, domain.AggregateCasesRequest{GroupBy: "region"})
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) || ve.Msg != `groupBy "region" is not supported by this data source` {
		t.Errorf("err = %v, want the region grouping refusal", err)
	}
}
