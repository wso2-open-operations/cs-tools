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

// TestParseCaseFieldFilters_StateEntered: stateEntered keeps the states'
// display labels, lower-cased, as the state history records them, and
// stateEnteredOn bounds the change like createdOn (relative dates, a
// date-only lte through the whole day).
func TestParseCaseFieldFilters_StateEntered(t *testing.T) {
	p, err := ParseCaseFieldFilters([]domain.CaseFieldFilter{
		{Field: "stateEntered", Op: "in", Values: []string{"waiting_on_wso2", "awaiting_info"}},
		{Field: "stateEnteredOn", Op: "gte", Values: []string{"__today__"}},
		{Field: "stateEnteredOn", Op: "lte", Values: []string{"2026-08-20"}},
		{Field: "state", Op: "in", Values: []string{"work_in_progress"}},
	}, "caller@example.com", nil, fixedNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := []string{"waiting on wso2", "awaiting info"}; !reflect.DeepEqual(p.EnteredStateLabels, want) {
		t.Errorf("EnteredStateLabels = %v, want %v", p.EnteredStateLabels, want)
	}
	if want := time.Date(2026, time.August, 15, 0, 0, 0, 0, time.UTC); p.EnteredStartDate == nil || !p.EnteredStartDate.Equal(want) {
		t.Errorf("EnteredStartDate = %v, want %v", p.EnteredStartDate, want)
	}
	if want := time.Date(2026, time.August, 20, 23, 59, 59, 999999999, time.UTC); p.EnteredEndDate == nil || !p.EnteredEndDate.Equal(want) {
		t.Errorf("EnteredEndDate = %v, want %v (the whole day)", p.EnteredEndDate, want)
	}
	// The current state filter is a separate filter, kept as it is.
	if !reflect.DeepEqual(p.States, []domain.CaseState{"work_in_progress"}) {
		t.Errorf("States = %v, want [work_in_progress]", p.States)
	}

	// stateEntered alone (no window): any time.
	p, err = ParseCaseFieldFilters([]domain.CaseFieldFilter{{Field: "stateEntered", Op: "in", Values: []string{"reopened"}}}, "caller@example.com", nil, fixedNow)
	if err != nil || p.EnteredStartDate != nil || p.EnteredEndDate != nil || !reflect.DeepEqual(p.EnteredStateLabels, []string{"reopened"}) {
		t.Errorf("stateEntered alone = %+v, %v; want the label and no window", p, err)
	}
}

func TestParseCaseFieldFilters_StateEnteredRejections(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   []domain.CaseFieldFilter
		want string
	}{
		{"not a case state", []domain.CaseFieldFilter{{Field: "stateEntered", Op: "in", Values: []string{"waiting_on_wso2", "WAITING_ON_WSO2"}}}, `stateEntered value "WAITING_ON_WSO2" is not a case state`},
		{"notIn", []domain.CaseFieldFilter{{Field: "stateEntered", Op: "notIn", Values: []string{"closed"}}}, `field "stateEntered" does not support op "notIn"`},
		{"no states", []domain.CaseFieldFilter{{Field: "stateEntered", Op: "in"}}, "requires a non-empty values array"},
		{"a window alone", []domain.CaseFieldFilter{{Field: "stateEnteredOn", Op: "gte", Values: []string{"__today__"}}}, "stateEnteredOn needs a stateEntered filter"},
		{"a window op it does not take", []domain.CaseFieldFilter{{Field: "stateEntered", Op: "in", Values: []string{"closed"}}, {Field: "stateEnteredOn", Op: "eq", Values: []string{"2026-08-01"}}}, `field "stateEnteredOn" does not support op "eq"`},
		{"not a date", []domain.CaseFieldFilter{{Field: "stateEntered", Op: "in", Values: []string{"closed"}}, {Field: "stateEnteredOn", Op: "gte", Values: []string{"today"}}}, "must be an RFC3339 timestamp"},
		{"no date", []domain.CaseFieldFilter{{Field: "stateEntered", Op: "in", Values: []string{"closed"}}, {Field: "stateEnteredOn", Op: "lte"}}, "requires a non-empty values array"},
		{"a window ending before it starts", []domain.CaseFieldFilter{
			{Field: "stateEntered", Op: "in", Values: []string{"closed"}},
			{Field: "stateEnteredOn", Op: "gte", Values: []string{"2026-08-10"}},
			{Field: "stateEnteredOn", Op: "lte", Values: []string{"2026-08-09"}},
		}, "stateEnteredOn: lte value must not be before gte value"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseCaseFieldFilters(tc.in, "caller@example.com", nil, fixedNow)
			var ve *apierror.ValidationError
			if !errors.As(err, &ve) || !strings.Contains(ve.Msg, tc.want) {
				t.Fatalf("err = %v, want a ValidationError mentioning %q", err, tc.want)
			}
		})
	}

	// A window on one day, or at one instant, is not "before".
	for _, bounds := range [][2]string{{"2026-08-10", "2026-08-10"}, {"2026-08-10T10:00:00Z", "2026-08-10T10:00:00Z"}} {
		if _, err := ParseCaseFieldFilters([]domain.CaseFieldFilter{
			{Field: "stateEntered", Op: "in", Values: []string{"closed"}},
			{Field: "stateEnteredOn", Op: "gte", Values: []string{bounds[0]}},
			{Field: "stateEnteredOn", Op: "lte", Values: []string{bounds[1]}},
		}, "caller@example.com", nil, fixedNow); err != nil {
			t.Errorf("window %v: %v", bounds, err)
		}
	}
}

// TestParseCaseFieldFilterGroups_RejectsStateEntered: the state history
// filters are top level only.
func TestParseCaseFieldFilterGroups_RejectsStateEntered(t *testing.T) {
	for _, branch := range []domain.CaseFilterBranch{
		{Filters: []domain.CaseFieldFilter{{Field: "stateEntered", Op: "in", Values: []string{"waiting_on_wso2"}}}},
		{Filters: []domain.CaseFieldFilter{{Field: "stateEntered", Op: "in", Values: []string{"waiting_on_wso2"}}, {Field: "stateEnteredOn", Op: "gte", Values: []string{"__today__"}}}},
	} {
		_, err := ParseCaseFieldFilterGroups([]domain.CaseFilterBranch{branch})
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) || ve.Msg != `anyOf: fields "stateEntered" and "stateEnteredOn" are not supported inside an OR group` {
			t.Errorf("%+v: err = %v, want the OR group refusal", branch, err)
		}
	}
}

// TestSNCaseService_RefusesStateEntered: the ServiceNow data source keeps no
// state history, so the search and the aggregate refuse the filters before
// any call rather than widening the result.
func TestSNCaseService_RefusesStateEntered(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(http.ResponseWriter, *http.Request) {
		t.Fatalf("ServiceNow must not be called")
	})
	svc := NewServiceNowCaseService(newTestSNClient(t, mux), nil, nil, nil, nil, "", nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	filters := domain.SearchCasesFilters{Filters: []domain.CaseFieldFilter{
		{Field: "stateEntered", Op: "in", Values: []string{"waiting_on_wso2"}},
		{Field: "stateEnteredOn", Op: "gte", Values: []string{"__today__"}},
	}}
	const want = `fields "stateEntered" and "stateEnteredOn" are not supported by this data source`

	_, err := svc.SearchCases(ctx, domain.SearchCasesRequest{Filters: filters})
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) || ve.Msg != want {
		t.Errorf("SearchCases err = %v, want %q", err, want)
	}
	_, err = svc.AggregateCases(ctx, domain.AggregateCasesRequest{Filters: filters, GroupBy: "account"})
	if !errors.As(err, &ve) || ve.Msg != want {
		t.Errorf("AggregateCases err = %v, want %q", err, want)
	}
}
