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
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// excludeProductFilter is a request's product notIn filter entry.
func excludeProductFilter(names ...string) domain.CaseFieldFilter {
	return domain.CaseFieldFilter{Field: "product", Op: "notIn", Values: names}
}

// TestParseCaseFieldFilters_ExcludesProducts: product notIn leaves products
// out, beside a product in filter or alone; a version still needs one
// product named by "in", which a notIn filter never is.
func TestParseCaseFieldFilters_ExcludesProducts(t *testing.T) {
	now := time.Date(2026, time.November, 11, 0, 0, 0, 0, time.UTC)
	p, err := ParseCaseFieldFilters([]domain.CaseFieldFilter{excludeProductFilter("WSO2 API Manager", "WSO2 Identity Server")}, "", nil, now)
	if err != nil {
		t.Fatalf("ParseCaseFieldFilters: %v", err)
	}
	if !reflect.DeepEqual(p.ExcludeProductNames, []string{"WSO2 API Manager", "WSO2 Identity Server"}) || len(p.ProductNames) != 0 {
		t.Errorf("ExcludeProductNames = %v, ProductNames = %v, want the two products left out and none kept", p.ExcludeProductNames, p.ProductNames)
	}

	p, err = ParseCaseFieldFilters([]domain.CaseFieldFilter{excludeProductFilter("API Gateway"), productFilter("WSO2 API Manager"), versionFilter("4.3.0")}, "", nil, now)
	if err != nil {
		t.Fatalf("a version beside one kept product and one left out: %v", err)
	}
	if !reflect.DeepEqual(p.ProductNames, []string{"WSO2 API Manager"}) || !reflect.DeepEqual(p.ExcludeProductNames, []string{"API Gateway"}) {
		t.Errorf("ProductNames = %v, ExcludeProductNames = %v", p.ProductNames, p.ExcludeProductNames)
	}

	for _, tc := range []struct {
		name    string
		filters []domain.CaseFieldFilter
		want    string
	}{
		{"a version beside a product left out only", []domain.CaseFieldFilter{excludeProductFilter("WSO2 API Manager"), versionFilter("4.3.0")}, "productVersion needs a product filter naming exactly one product"},
		{"eq", []domain.CaseFieldFilter{{Field: "product", Op: "eq", Values: []string{"WSO2 API Manager"}}}, `field "product" does not support op "eq"`},
		{"no values", []domain.CaseFieldFilter{excludeProductFilter()}, `field "product" op "notIn" requires a non-empty values array`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseCaseFieldFilters(tc.filters, "", nil, now)
			var ve *apierror.ValidationError
			if !errors.As(err, &ve) || !strings.Contains(ve.Msg, tc.want) {
				t.Errorf("err = %v, want a validation error containing %q", err, tc.want)
			}
		})
	}

	_, err = ParseCaseFieldFilterGroups([]domain.CaseFilterBranch{{Filters: []domain.CaseFieldFilter{excludeProductFilter("WSO2 API Manager")}},
		{Filters: []domain.CaseFieldFilter{{Field: "state", Op: "in", Values: []string{"open"}}}}})
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) || ve.Msg != `anyOf: field "product" (notIn) is not supported inside an OR group` {
		t.Errorf("anyOf branch: err = %v, want the OR group refusal", err)
	}
}

// TestSNCaseService_RefusesProductsLeftOut: the ServiceNow data source has no
// product exclusion, so it refuses one before any call, for a search and an
// aggregate.
func TestSNCaseService_RefusesProductsLeftOut(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(http.ResponseWriter, *http.Request) {
		t.Fatalf("ServiceNow must not be called")
	})
	svc := NewServiceNowCaseService(newTestSNClient(t, mux), nil, nil, nil, nil, "", nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	excluded := domain.SearchCasesFilters{Filters: []domain.CaseFieldFilter{excludeProductFilter("WSO2 API Manager")}}
	const refused = `field "product" (notIn) is not supported by this data source`

	var ve *apierror.ValidationError
	_, err := svc.SearchCases(ctx, domain.SearchCasesRequest{Filters: excluded, Pagination: domain.Pagination{Limit: 10}})
	if !errors.As(err, &ve) || ve.Msg != refused {
		t.Errorf("search: err = %v, want the product exclusion refusal", err)
	}
	_, err = svc.AggregateCases(ctx, domain.AggregateCasesRequest{GroupBy: "state", Filters: excluded})
	if !errors.As(err, &ve) || ve.Msg != refused {
		t.Errorf("aggregate: err = %v, want the product exclusion refusal", err)
	}
}
