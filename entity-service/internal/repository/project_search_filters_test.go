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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package repository

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

func TestBuildProjectSearchWhere_Filters(t *testing.T) {
	cases := []struct {
		name     string
		req      domain.SearchProjectsRequest
		wantSQL  string
		wantArgs []any
	}{
		{"endDateFrom inclusive", domain.SearchProjectsRequest{EndDateFrom: "2026-01-01"},
			" AND p.end_date >= $1::date", []any{"2026-01-01"}},
		{"endDateTo inclusive", domain.SearchProjectsRequest{EndDateTo: "2026-12-31"},
			" AND p.end_date <= $1::date", []any{"2026-12-31"}},
		{"onboardingStatus any of, SN spellings", domain.SearchProjectsRequest{OnboardingStatus: []string{"In-Progress", "OnHold", "completed"}},
			" AND p.onboarding_status = ANY($1::text[]::onboarding_status_enum[])", []any{[]string{"IN_PROGRESS", "ON_HOLD", "COMPLETED"}}},
		{"sfIds any of", domain.SearchProjectsRequest{SfIDs: []string{"SF001", "SF002"}},
			" AND p.sf_id = ANY($1::text[])", []any{[]string{"SF001", "SF002"}}},
		{"subRegion case-insensitive, trimmed", domain.SearchProjectsRequest{SubRegion: " na - west "},
			" AND LOWER(TRIM(a.sub_region)) = LOWER($1)", []any{"na - west"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			where, args, next, err := buildProjectSearchWhere(tc.req, SearchScope{Unrestricted: true})
			if err != nil {
				t.Fatalf("err = %v", err)
			}
			if where != "WHERE 1=1"+tc.wantSQL {
				t.Errorf("where = %q, want suffix %q", where, tc.wantSQL)
			}
			if !reflect.DeepEqual(args, tc.wantArgs) {
				t.Errorf("args = %#v, want %#v", args, tc.wantArgs)
			}
			if next != len(tc.wantArgs)+1 {
				t.Errorf("next arg = %d, want %d", next, len(tc.wantArgs)+1)
			}
		})
	}
}

func TestBuildProjectSearchWhere_CombinesWithAnd(t *testing.T) {
	req := domain.SearchProjectsRequest{
		ClosureStatus: "Open", EndDateFrom: "2026-01-01", EndDateTo: "2026-03-31",
		OnboardingStatus: []string{"Completed"}, SubRegion: "APAC",
	}
	where, args, next, err := buildProjectSearchWhere(req, SearchScope{Unrestricted: true})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	for _, frag := range []string{"wso2_closure_state::text = $1", "end_date >= $2::date", "end_date <= $3::date",
		"onboarding_status = ANY($4", "LOWER($5)"} {
		if !strings.Contains(where, frag) {
			t.Errorf("where %q missing %q", where, frag)
		}
	}
	if len(args) != 5 || next != 6 {
		t.Errorf("args = %v (next %d), want 5 args", args, next)
	}
}

func TestBuildProjectSearchWhere_InvalidOnboardingStatus(t *testing.T) {
	_, _, _, err := buildProjectSearchWhere(domain.SearchProjectsRequest{OnboardingStatus: []string{"Done"}}, SearchScope{Unrestricted: true})
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want ValidationError", err)
	}
	want := `onboardingStatus: "Done" is not a valid onboarding status; use one of Cancelled, Completed, Expired, In-Progress, Not-Applicable, Not-Started, On-Hold`
	if ve.Msg != want {
		t.Errorf("msg = %q, want %q", ve.Msg, want)
	}
}

func TestValidateClosureFields(t *testing.T) {
	s := func(v string) *string { return &v }
	ok := []domain.ProjectUpdateRequest{
		{EndDateClosureState: s("Pending Notified")},
		{InvoiceDueDateClosureState: s("Suspended & Previously Paid")},
		{ComplianceViolationClosureState: s("open")},
		{HasAgent: new(bool)},
	}
	for _, req := range ok {
		if err := validateClosureFields(req); err != nil {
			t.Errorf("validateClosureFields(%+v) = %v, want nil", req, err)
		}
	}

	cases := []struct {
		req  domain.ProjectUpdateRequest
		want string
	}{
		{domain.ProjectUpdateRequest{ComplianceViolationClosureState: s("Restricted")},
			`complianceViolationClosureState: "Restricted" is not a valid closure state; use one of Open, Suspended`},
		{domain.ProjectUpdateRequest{EndDateClosureState: s("Paused")},
			`endDateClosureState: "Paused" is not a valid closure state; use one of Closed, Closure Notices, Notified, Open, Pending Closed, Pending Closure Notices, Pending Notified, Pending Restricted, Restricted, Suspended`},
	}
	for _, tc := range cases {
		err := validateClosureFields(tc.req)
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) || ve.Msg != tc.want {
			t.Errorf("err = %v, want %q", err, tc.want)
		}
	}

	err := validateClosureFields(domain.ProjectUpdateRequest{InvoiceDueDateClosureState: s("Late")})
	if err == nil || !strings.Contains(err.Error(), `invoiceDueDateClosureState: "Late"`) ||
		!strings.Contains(err.Error(), "Pending Suspended And Previously Paid") {
		t.Errorf("err = %v, want invoiceDueDateClosureState message listing accepted values", err)
	}
}
