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
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

func resolutionDay(s string) time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return t
}

// TestResolveCaseResolutionWindow: no window is the 12 months ending today; a
// window takes dates and relative dates, inclusive, up to 24 months.
func TestResolveCaseResolutionWindow(t *testing.T) {
	today := time.Date(2026, time.November, 16, 15, 4, 0, 0, time.UTC)
	for _, tc := range []struct {
		name     string
		from, to string
		wantFrom string
		wantTo   string
	}{
		{"no window: the last 12 months", "", "", "2025-11-17", "2026-11-16"},
		{"plain dates", "2026-07-01", "2026-09-30", "2026-07-01", "2026-09-30"},
		{"relative dates", "__startOfQuarter:-1__", "__endOfQuarter:-1__", "2026-07-01", "2026-09-30"},
		{"one day", "2026-11-16", "__today__", "2026-11-16", "2026-11-16"},
		{"the longest window", "2024-11-17", "2026-11-16", "2024-11-17", "2026-11-16"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			from, to, err := resolveCaseResolutionWindow(domain.CaseResolutionTimeRequest{ResolvedFrom: tc.from, ResolvedTo: tc.to}, today)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !from.Equal(resolutionDay(tc.wantFrom)) || !to.Equal(resolutionDay(tc.wantTo)) {
				t.Errorf("window = %s to %s, want %s to %s", from.Format("2006-01-02"), to.Format("2006-01-02"), tc.wantFrom, tc.wantTo)
			}
		})
	}
}

// TestResolveCaseResolutionWindow_Refuses: one side alone, a bad date, an end
// before its start and a window over 24 months are validation errors.
func TestResolveCaseResolutionWindow_Refuses(t *testing.T) {
	today := time.Date(2026, time.November, 16, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name     string
		from, to string
		want     string
	}{
		{"only from", "2026-01-01", "", "resolvedFrom and resolvedTo are both required"},
		{"only to", "", "2026-01-01", "resolvedFrom and resolvedTo are both required"},
		{"bad from", "01/01/2026", "2026-02-01", `resolvedFrom must be a YYYY-MM-DD date or a relative date, got "01/01/2026"`},
		{"bad to", "2026-01-01", "next week", `resolvedTo must be a YYYY-MM-DD date or a relative date, got "next week"`},
		{"end before start", "2026-02-01", "2026-01-31", "resolvedTo must not be before resolvedFrom"},
		{"over 24 months", "2024-11-16", "2026-11-16", "the resolution window covers at most 24 months"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := resolveCaseResolutionWindow(domain.CaseResolutionTimeRequest{ResolvedFrom: tc.from, ResolvedTo: tc.to}, today)
			var ve *apierror.ValidationError
			if !errors.As(err, &ve) || ve.Msg != tc.want {
				t.Errorf("err = %v, want the validation error %q", err, tc.want)
			}
		})
	}
}

// TestCaseService_CaseResolutionTime: the window reaches the repository with
// an exclusive end (the day after the last), the filters and the grouping as
// asked; the total covers every resolved case, the cases with no value for
// the grouping included; each group averages its own measured cases in hours.
func TestCaseService_CaseResolutionTime(t *testing.T) {
	repo := &stubCaseRepo{caseResolutionTime: func(_ context.Context, req domain.SearchCasesRequest, groupBy string, from, to time.Time, scope repository.SearchScope) ([]domain.CaseResolutionTimeSum, error) {
		if groupBy != "severity" || !from.Equal(resolutionDay("2026-07-01")) || !to.Equal(resolutionDay("2026-10-01")) {
			t.Errorf("groupBy %q, window [%s, %s), want severity and [2026-07-01, 2026-10-01)", groupBy, from, to)
		}
		if !reflect.DeepEqual(req.Parsed.ProductNames, []string{"API Gateway"}) {
			t.Errorf("products = %v, want the request's product filter", req.Parsed.ProductNames)
		}
		if !scope.Unrestricted {
			t.Errorf("scope = %+v, want the caller's", scope)
		}
		return []domain.CaseResolutionTimeSum{
			{Key: strPtr("critical"), Label: "critical", Count: 4, Measured: 2, Seconds: 2 * 3600 * 10},
			{Key: strPtr("high"), Label: "high", Count: 3, Measured: 0},
			{Key: nil, Count: 1, Measured: 1, Seconds: 3600 * 40},
		}, nil
	}}
	svc := NewCaseService(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	resp, err := svc.CaseResolutionTime(ctx, domain.CaseResolutionTimeRequest{
		ResolvedFrom: "2026-07-01", ResolvedTo: "2026-09-30", GroupBy: "severity",
		Filters: domain.SearchCasesFilters{Filters: []domain.CaseFieldFilter{productFilter("API Gateway")}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ResolvedFrom != "2026-07-01" || resp.ResolvedTo != "2026-09-30" {
		t.Errorf("window = %s to %s, want the request's", resp.ResolvedFrom, resp.ResolvedTo)
	}
	if resp.Total.Count != 8 || resp.Total.Measured != 3 || resp.Total.AverageHours == nil || *resp.Total.AverageHours != 20 {
		t.Errorf("total = %+v, want 8 resolved, 3 measured, 20 hours on average", resp.Total)
	}
	if len(resp.Groups) != 2 || resp.Others != nil {
		t.Fatalf("groups = %+v, others = %+v, want the two severities and no others", resp.Groups, resp.Others)
	}
	if g := resp.Groups[0]; g.Key != "critical" || g.Label != "critical" || g.Count != 4 || g.Measured != 2 || g.AverageHours == nil || *g.AverageHours != 10 {
		t.Errorf("critical = %+v, want 4 resolved, 2 measured, 10 hours", g)
	}
	if g := resp.Groups[1]; g.Key != "high" || g.Count != 3 || g.Measured != 0 || g.AverageHours != nil {
		t.Errorf("high = %+v, want 3 resolved, none measured, no average", g)
	}
}

// TestCaseService_CaseResolutionTime_Ungrouped: with no groupBy the one row is
// the total, with no groups; the default window reaches the repository.
func TestCaseService_CaseResolutionTime_Ungrouped(t *testing.T) {
	repo := &stubCaseRepo{caseResolutionTime: func(_ context.Context, _ domain.SearchCasesRequest, groupBy string, from, to time.Time, _ repository.SearchScope) ([]domain.CaseResolutionTimeSum, error) {
		if groupBy != "" || to.Sub(from) < 365*24*time.Hour || to.Sub(from) > 366*24*time.Hour {
			t.Errorf("groupBy %q, window [%s, %s), want none and the last 12 months", groupBy, from, to)
		}
		return []domain.CaseResolutionTimeSum{{Count: 5, Measured: 4, Seconds: 4 * 3600 * 2.5}}, nil
	}}
	svc := NewCaseService(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	resp, err := svc.CaseResolutionTime(ctx, domain.CaseResolutionTimeRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Total.Count != 5 || resp.Total.Measured != 4 || resp.Total.AverageHours == nil || *resp.Total.AverageHours != 2.5 {
		t.Errorf("total = %+v, want 5 resolved, 4 measured, 2.5 hours", resp.Total)
	}
	if resp.Groups != nil || resp.Others != nil {
		t.Errorf("groups = %+v, others = %+v, want neither", resp.Groups, resp.Others)
	}
}

// TestCaseService_CaseResolutionTime_Others: past MaxGroups the groups are
// merged into Others, its average over its cases rather than its groups.
func TestCaseService_CaseResolutionTime_Others(t *testing.T) {
	repo := &stubCaseRepo{caseResolutionTime: func(context.Context, domain.SearchCasesRequest, string, time.Time, time.Time, repository.SearchScope) ([]domain.CaseResolutionTimeSum, error) {
		return []domain.CaseResolutionTimeSum{
			{Key: strPtr("p1"), Label: "Project One", Count: 9, Measured: 9, Seconds: 9 * 3600},
			{Key: strPtr("p2"), Label: "Project Two", Count: 6, Measured: 3, Seconds: 3 * 3600 * 4},
			{Key: strPtr("p3"), Label: "Project Three", Count: 2, Measured: 1, Seconds: 3600 * 16},
		}, nil
	}}
	svc := NewCaseService(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	resp, err := svc.CaseResolutionTime(ctx, domain.CaseResolutionTimeRequest{GroupBy: "project", MaxGroups: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Groups) != 1 || resp.Groups[0].Label != "Project One" || *resp.Groups[0].AverageHours != 1 {
		t.Errorf("groups = %+v, want Project One alone, 1 hour", resp.Groups)
	}
	// (12 + 16) hours over 4 measured cases, not the mean of 4 and 16.
	if o := resp.Others; o == nil || o.Count != 8 || o.Measured != 4 || o.AverageHours == nil || *o.AverageHours != 7 || o.Key != "" {
		t.Errorf("others = %+v, want 8 resolved, 4 measured, 7 hours", o)
	}
	if resp.Total.Count != 17 || resp.Total.Measured != 13 {
		t.Errorf("total = %+v, want every group's cases", resp.Total)
	}
}

// TestCaseService_CaseResolutionTime_DefaultMaxGroups: with no MaxGroups the
// aggregate's default (10) applies.
func TestCaseService_CaseResolutionTime_DefaultMaxGroups(t *testing.T) {
	sums := make([]domain.CaseResolutionTimeSum, 11)
	for i := range sums {
		sums[i] = domain.CaseResolutionTimeSum{Key: strPtr(string(rune('a' + i))), Count: 1}
	}
	repo := &stubCaseRepo{caseResolutionTime: func(context.Context, domain.SearchCasesRequest, string, time.Time, time.Time, repository.SearchScope) ([]domain.CaseResolutionTimeSum, error) {
		return sums, nil
	}}
	svc := NewCaseService(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	resp, err := svc.CaseResolutionTime(ctx, domain.CaseResolutionTimeRequest{GroupBy: "account"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Groups) != 10 || resp.Others == nil || resp.Others.Count != 1 || resp.Others.AverageHours != nil {
		t.Errorf("groups = %d, others = %+v, want 10 groups and one unmeasured case in others", len(resp.Groups), resp.Others)
	}
}

// TestCaseService_CaseResolutionTime_RefusesBeforeReading: a bad window or
// filter is refused without reading anything.
func TestCaseService_CaseResolutionTime_RefusesBeforeReading(t *testing.T) {
	svc := NewCaseService(&stubCaseRepo{}, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	for name, req := range map[string]domain.CaseResolutionTimeRequest{
		"one side":   {ResolvedFrom: "2026-01-01"},
		"bad filter": {Filters: domain.SearchCasesFilters{Filters: []domain.CaseFieldFilter{{Field: "nope", Op: "in", Values: []string{"x"}}}}},
	} {
		_, err := svc.CaseResolutionTime(ctx, req)
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) {
			t.Errorf("%s: err = %v, want a validation error", name, err)
		}
	}
}

// TestSNCaseService_CaseResolutionTime: the external data source has no
// business durations, so it hands the call to Postgres when it has it and is
// unavailable otherwise.
func TestSNCaseService_CaseResolutionTime(t *testing.T) {
	_, err := (&snCaseService{}).CaseResolutionTime(context.Background(), domain.CaseResolutionTimeRequest{})
	var su *apierror.ServiceUnavailableError
	if !errors.As(err, &su) || !strings.Contains(su.Msg, "only supported for the Postgres data source") {
		t.Errorf("err = %v, want unavailable without Postgres", err)
	}

	repo := &stubCaseRepo{caseResolutionTime: func(context.Context, domain.SearchCasesRequest, string, time.Time, time.Time, repository.SearchScope) ([]domain.CaseResolutionTimeSum, error) {
		return []domain.CaseResolutionTimeSum{{Count: 2}}, nil
	}}
	pg := NewCaseService(repo, stubUserRepo{}, nil, alwaysUnrestrictedAccess{}, nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))
	resp, err := (&snCaseService{pgFallback: pg}).CaseResolutionTime(ctx, domain.CaseResolutionTimeRequest{})
	if err != nil || resp.Total.Count != 2 {
		t.Errorf("resp = %+v, err = %v, want Postgres' answer", resp, err)
	}
}
