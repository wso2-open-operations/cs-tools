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

// productFilter and versionFilter are a request's product and productVersion
// filter entries.
func productFilter(names ...string) domain.CaseFieldFilter {
	return domain.CaseFieldFilter{Field: "product", Op: "in", Values: names}
}

func versionFilter(versions ...string) domain.CaseFieldFilter {
	return domain.CaseFieldFilter{Field: "productVersion", Op: "in", Values: versions}
}

// TestParseCaseFieldFilters_ProductVersion: productVersion takes "in" with
// versions, beside a product filter naming exactly one product (in one entry
// or several, in either order).
func TestParseCaseFieldFilters_ProductVersion(t *testing.T) {
	now := time.Date(2026, time.November, 11, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name    string
		filters []domain.CaseFieldFilter
	}{
		{"product first", []domain.CaseFieldFilter{productFilter("WSO2 API Manager"), versionFilter("4.2.0", "4.3.0")}},
		{"version first", []domain.CaseFieldFilter{versionFilter("4.2.0", "4.3.0"), productFilter("WSO2 API Manager")}},
		{"the product named twice", []domain.CaseFieldFilter{productFilter("WSO2 API Manager", "WSO2 API Manager"), versionFilter("4.2.0", "4.3.0")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, err := ParseCaseFieldFilters(tc.filters, "", nil, now)
			if err != nil {
				t.Fatalf("ParseCaseFieldFilters: %v", err)
			}
			if !reflect.DeepEqual(p.ProductVersions, []string{"4.2.0", "4.3.0"}) {
				t.Errorf("ProductVersions = %v, want [4.2.0 4.3.0]", p.ProductVersions)
			}
		})
	}
}

// TestParseCaseFieldFilters_RefusesProductVersion: a version without one
// product, a version filter that is not "in" with values, and a version in an
// anyOf branch are refused.
func TestParseCaseFieldFilters_RefusesProductVersion(t *testing.T) {
	now := time.Date(2026, time.November, 11, 0, 0, 0, 0, time.UTC)
	const needsOne = "productVersion needs a product filter naming exactly one product"
	for _, tc := range []struct {
		name    string
		filters []domain.CaseFieldFilter
		want    string
	}{
		{"no product", []domain.CaseFieldFilter{versionFilter("4.3.0")}, needsOne},
		{"two products in one entry", []domain.CaseFieldFilter{productFilter("WSO2 API Manager", "API Gateway"), versionFilter("4.3.0")}, needsOne},
		{"two products in two entries", []domain.CaseFieldFilter{productFilter("WSO2 API Manager"), productFilter("API Gateway"), versionFilter("4.3.0")}, needsOne},
		{"notIn", []domain.CaseFieldFilter{productFilter("WSO2 API Manager"), {Field: "productVersion", Op: "notIn", Values: []string{"4.3.0"}}}, `field "productVersion" does not support op "notIn"`},
		{"no values", []domain.CaseFieldFilter{productFilter("WSO2 API Manager"), versionFilter()}, `field "productVersion" op "in" requires a non-empty values array`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseCaseFieldFilters(tc.filters, "", nil, now)
			var ve *apierror.ValidationError
			if !errors.As(err, &ve) || !strings.Contains(ve.Msg, tc.want) {
				t.Errorf("err = %v, want a validation error containing %q", err, tc.want)
			}
		})
	}

	for _, branch := range [][]domain.CaseFieldFilter{
		{versionFilter("4.3.0")},
		{productFilter("WSO2 API Manager"), versionFilter("4.3.0")},
	} {
		_, err := ParseCaseFieldFilterGroups([]domain.CaseFilterBranch{{Filters: branch}, {Filters: []domain.CaseFieldFilter{{Field: "state", Op: "in", Values: []string{"open"}}}}})
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) {
			t.Errorf("anyOf branch %+v: err = %v, want a validation error", branch, err)
		}
	}
}

// TestCaseService_AggregateCases_ByProductVersion: grouping by version
// reaches the repository with the product filter naming one product, and is
// refused without one or with two.
func TestCaseService_AggregateCases_ByProductVersion(t *testing.T) {
	called := false
	repo := &stubCaseRepo{aggregateCases: func(_ context.Context, req domain.SearchCasesRequest, groupBy string) ([]domain.AggregateBucket, error) {
		called = true
		if groupBy != "productVersion" || !reflect.DeepEqual(req.Parsed.ProductNames, []string{"WSO2 API Manager"}) {
			t.Errorf("groupBy = %q, products = %v, want productVersion within WSO2 API Manager", groupBy, req.Parsed.ProductNames)
		}
		return []domain.AggregateBucket{{Key: "4.3.0", Label: "4.3.0", Count: 5}, {Key: "4.2.0", Label: "4.2.0", Count: 1}}, nil
	}}
	svc := NewCaseService(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	resp, err := svc.AggregateCases(ctx, domain.AggregateCasesRequest{GroupBy: "productVersion",
		Filters: domain.SearchCasesFilters{Filters: []domain.CaseFieldFilter{productFilter("WSO2 API Manager")}}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Groups) != 2 || resp.Groups[0].Key != "4.3.0" || resp.TotalRecords != 6 {
		t.Errorf("got %+v, want the two version buckets and a total of 6", resp)
	}

	for name, filters := range map[string][]domain.CaseFieldFilter{
		"no product":   nil,
		"two products": {productFilter("WSO2 API Manager", "API Gateway")},
	} {
		called = false
		_, err := svc.AggregateCases(ctx, domain.AggregateCasesRequest{GroupBy: "productVersion", Filters: domain.SearchCasesFilters{Filters: filters}})
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) || ve.Msg != `groupBy "productVersion" needs a product filter naming exactly one product` {
			t.Errorf("%s: err = %v, want the one-product refusal", name, err)
		}
		if called {
			t.Errorf("%s: the repository was called", name)
		}
	}
}

// TestSNCaseService_RefusesProductVersion: the ServiceNow data source has no
// version filter or grouping, so it refuses both before any call.
func TestSNCaseService_RefusesProductVersion(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(http.ResponseWriter, *http.Request) {
		t.Fatalf("ServiceNow must not be called")
	})
	svc := NewServiceNowCaseService(newTestSNClient(t, mux), nil, nil, nil, nil, "", nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	versioned := domain.SearchCasesFilters{Filters: []domain.CaseFieldFilter{productFilter("WSO2 API Manager"), versionFilter("4.3.0")}}
	const filterRefused = `field "productVersion" is not supported by this data source`

	var ve *apierror.ValidationError
	_, err := svc.SearchCases(ctx, domain.SearchCasesRequest{Filters: versioned, Pagination: domain.Pagination{Limit: 10}})
	if !errors.As(err, &ve) || ve.Msg != filterRefused {
		t.Errorf("search: err = %v, want the version filter refusal", err)
	}
	_, err = svc.AggregateCases(ctx, domain.AggregateCasesRequest{GroupBy: "state", Filters: versioned})
	if !errors.As(err, &ve) || ve.Msg != filterRefused {
		t.Errorf("aggregate with a version filter: err = %v, want the version filter refusal", err)
	}
	_, err = svc.AggregateCases(ctx, domain.AggregateCasesRequest{GroupBy: "productVersion",
		Filters: domain.SearchCasesFilters{Filters: []domain.CaseFieldFilter{productFilter("WSO2 API Manager")}}})
	if !errors.As(err, &ve) || ve.Msg != `groupBy "productVersion" is not supported by this data source` {
		t.Errorf("aggregate by version: err = %v, want the version grouping refusal", err)
	}
}
