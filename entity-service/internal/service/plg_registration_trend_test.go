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
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// trendRepo answers RegistrationVolumeTrend from counts and records what it
// was asked; every other method is the nil interface's (never reached).
type trendRepo struct {
	repository.OrgPlatformRepository
	counts     map[string]int
	calls      int
	req        domain.SearchRegistrationsRequest
	start, end time.Time
}

func (r *trendRepo) RegistrationVolumeTrend(_ context.Context, req domain.SearchRegistrationsRequest, start, end time.Time) (map[string]int, error) {
	r.calls++
	r.req, r.start, r.end = req, start, end
	return r.counts, nil
}

func trendDay(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// TestRegistrationVolumeTrendByMonthThisYear: "this year by month" on
// 2026-03-15 is three points, January and February whole and March to the
// 15th, each summing its days, with the resolved window echoed.
func TestRegistrationVolumeTrendByMonthThisYear(t *testing.T) {
	repo := &trendRepo{counts: map[string]int{"2026-01-05": 2, "2026-01-31": 1, "2026-02-14": 4, "2026-03-15": 1}}
	svc := &pairingService{repo: repo}
	stage := []domain.LifecycleStage{domain.StageActivated}

	got, err := svc.registrationVolumeTrend(context.Background(), domain.RegistrationVolumeTrendRequest{
		UnacknowledgedOnly: true,
		Filters:            domain.OrganizationSearchFilters{ProductCodes: []string{"IAM"}, LifecycleStages: stage},
		StartDate:          "__startOfYear:0__", EndDate: "__today__", Bucket: "month",
	}, trendDay(t, "2026-03-15"))
	if err != nil {
		t.Fatal(err)
	}

	if !repo.start.Equal(trendDay(t, "2026-01-01")) || !repo.end.Equal(trendDay(t, "2026-03-15")) {
		t.Errorf("repo window = %s to %s, want 2026-01-01 to 2026-03-15", repo.start.Format("2006-01-02"), repo.end.Format("2006-01-02"))
	}
	if !repo.req.UnacknowledgedOnly || !reflect.DeepEqual(repo.req.Filters.ProductCodes, []string{"IAM"}) || !reflect.DeepEqual(repo.req.Filters.LifecycleStages, stage) {
		t.Errorf("repo request = %+v, want the filters and unacknowledgedOnly passed on", repo.req)
	}
	want := domain.CaseVolumeTrendResponse{
		Days: 74, StartDate: "2026-01-01", EndDate: "2026-03-15", Bucket: "month",
		Series: []domain.CaseVolumeTrendSeries{{Label: "Registered", Points: []domain.CaseVolumeTrendPoint{
			{Date: "2026-01-01", EndDate: "2026-01-31", Count: 3},
			{Date: "2026-02-01", EndDate: "2026-02-28", Count: 4},
			{Date: "2026-03-01", EndDate: "2026-03-15", Count: 1},
		}}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
}

// TestRegistrationVolumeTrendByWeek: weeks run Monday to Sunday, the first
// one starting at the window's first day.
func TestRegistrationVolumeTrendByWeek(t *testing.T) {
	repo := &trendRepo{counts: map[string]int{"2026-11-04": 1, "2026-11-08": 2, "2026-11-09": 5}}
	svc := &pairingService{repo: repo}

	got, err := svc.registrationVolumeTrend(context.Background(), domain.RegistrationVolumeTrendRequest{
		StartDate: "2026-11-04", EndDate: "2026-11-10", Bucket: "week",
	}, trendDay(t, "2026-11-10"))
	if err != nil {
		t.Fatal(err)
	}
	want := []domain.CaseVolumeTrendPoint{
		{Date: "2026-11-04", EndDate: "2026-11-08", Count: 3}, // Wednesday to Sunday
		{Date: "2026-11-09", EndDate: "2026-11-10", Count: 5},
	}
	if !reflect.DeepEqual(got.Series[0].Points, want) {
		t.Errorf("points = %+v, want %+v", got.Series[0].Points, want)
	}
}

// TestRegistrationVolumeTrendTrailingDays: with no window the trailing 30
// days, one point per day ending today, and no window echoed.
func TestRegistrationVolumeTrendTrailingDays(t *testing.T) {
	repo := &trendRepo{counts: map[string]int{"2026-11-09": 2}}
	svc := &pairingService{repo: repo}

	got, err := svc.registrationVolumeTrend(context.Background(), domain.RegistrationVolumeTrendRequest{}, trendDay(t, "2026-11-09"))
	if err != nil {
		t.Fatal(err)
	}
	points := got.Series[0].Points
	if got.Days != 30 || len(points) != 30 || got.StartDate != "" || got.Bucket != "" {
		t.Fatalf("got days %d, %d points, window %q/%q, want 30 day points and no window", got.Days, len(points), got.StartDate, got.Bucket)
	}
	if first, last := points[0], points[29]; first.Date != "2026-10-11" || last.Date != "2026-11-09" || last.Count != 2 || last.EndDate != "" {
		t.Errorf("first %+v, last %+v, want 2026-10-11 to 2026-11-09 (2), no endDate", first, last)
	}
}

// TestRegistrationVolumeTrendRefusesABadRequest: the case trend's window
// rules and the registration search's filter rules, before any query.
func TestRegistrationVolumeTrendRefusesABadRequest(t *testing.T) {
	for name, req := range map[string]domain.RegistrationVolumeTrendRequest{
		"a bucket without dates": {Bucket: "month"},
		"one date":               {StartDate: "2026-01-01"},
		"days with dates":        {Days: 7, StartDate: "2026-01-01", EndDate: "2026-01-07"},
		"25 months":              {StartDate: "2024-01-01", EndDate: "2026-01-31", Bucket: "month"},
		"an unknown bucket":      {StartDate: "2026-01-01", EndDate: "2026-01-31", Bucket: "quarter"},
		"an unknown stage":       {Filters: domain.OrganizationSearchFilters{LifecycleStages: []domain.LifecycleStage{"WON"}}},
	} {
		repo := &trendRepo{}
		_, err := (&pairingService{repo: repo}).registrationVolumeTrend(context.Background(), req, trendDay(t, "2026-11-09"))
		var verr *apierror.ValidationError
		if !errors.As(err, &verr) {
			t.Errorf("%s: err = %v, want a validation error", name, err)
		}
		if repo.calls != 0 {
			t.Errorf("%s: the repository was queried", name)
		}
	}
}
