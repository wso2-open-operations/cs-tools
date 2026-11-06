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

package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

type fakeLongOpenCaseReportRepo struct {
	data      domain.LongOpenCaseReportData
	gotCutoff time.Time
	gotTypes  []string
	gotTags   []string
	gotLimit  int
	calls     int
	listErr   error
}

func (f *fakeLongOpenCaseReportRepo) ListLongOpenCases(_ context.Context, createdBefore time.Time, projectTypes, excludedTags []string, limit int) (domain.LongOpenCaseReportData, error) {
	f.calls++
	f.gotCutoff, f.gotTypes, f.gotTags, f.gotLimit = createdBefore, projectTypes, excludedTags, limit
	return f.data, f.listErr
}

func longOpenCfg() service.LongOpenCaseReportConfig {
	return service.LongOpenCaseReportConfig{
		ProjectTypes: []string{" Subscription ", "", "Cloud Support"},
		ExcludedTags: []string{"tag-a", " ", "tag-b"},
	}
}

func TestLongOpenCaseReportBuildReport(t *testing.T) {
	repo := &fakeLongOpenCaseReportRepo{data: domain.LongOpenCaseReportData{
		Cases:                []domain.LongOpenCaseReportRow{{Number: "CASE0001"}},
		OnboardingCases:      []domain.LongOpenCaseReportRow{{Number: "CASE0002"}},
		CasesTotal:           1,
		OnboardingCasesTotal: 1,
	}}
	svc := service.NewLongOpenCaseReportService(repo, longOpenCfg())
	// Sunday 2026-10-18 22:30 UTC is already Monday 2026-10-19 04:00 in
	// Colombo. 30 days before that is 2026-09-19; the cut-off is the start
	// of that day in Colombo, which is 2026-09-18 18:30 UTC.
	now := time.Date(2026, 10, 18, 22, 30, 0, 0, time.UTC)

	got, err := svc.BuildReport(context.Background(), domain.LongOpenCaseReportRequest{Now: now})
	if err != nil {
		t.Fatalf("BuildReport: %v", err)
	}
	if got.OlderThanDays != 30 {
		t.Errorf("OlderThanDays = %d, want the default 30", got.OlderThanDays)
	}
	if want := time.Date(2026, 9, 18, 18, 30, 0, 0, time.UTC); !got.CreatedBefore.Equal(want) || !repo.gotCutoff.Equal(want) {
		t.Errorf("CreatedBefore = %s, cut-off passed = %s, want %s", got.CreatedBefore, repo.gotCutoff, want)
	}
	if !got.GeneratedAt.Equal(now) {
		t.Errorf("GeneratedAt = %s, want %s", got.GeneratedAt, now)
	}
	if !equalStrings(repo.gotTypes, []string{"Subscription", "Cloud Support"}) || !equalStrings(repo.gotTags, []string{"tag-a", "tag-b"}) {
		t.Errorf("config passed = %v / %v, want the trimmed, non-blank values", repo.gotTypes, repo.gotTags)
	}
	if repo.gotLimit != service.LongOpenCaseReportMaxRowsPerTable {
		t.Errorf("limit passed = %d, want %d", repo.gotLimit, service.LongOpenCaseReportMaxRowsPerTable)
	}
	if len(got.Cases) != 1 || len(got.OnboardingCases) != 1 || got.CasesTotal != 1 || got.OnboardingCasesTotal != 1 ||
		got.CasesTruncated || got.OnboardingCasesTruncated {
		t.Errorf("report = %+v, want one row per table, totals 1/1, nothing truncated", got)
	}
}

func TestLongOpenCaseReportExplicitAgeAndEmptyTables(t *testing.T) {
	svc := service.NewLongOpenCaseReportService(&fakeLongOpenCaseReportRepo{}, longOpenCfg())
	got, err := svc.BuildReport(context.Background(), domain.LongOpenCaseReportRequest{OlderThanDays: 7, Now: time.Now()})
	if err != nil {
		t.Fatalf("BuildReport: %v", err)
	}
	if got.OlderThanDays != 7 {
		t.Errorf("OlderThanDays = %d, want 7", got.OlderThanDays)
	}
	if got.Cases == nil || got.OnboardingCases == nil {
		t.Errorf("empty tables must be non-nil so they serialise as [], got %+v", got)
	}
}

func TestLongOpenCaseReportTruncationFlags(t *testing.T) {
	repo := &fakeLongOpenCaseReportRepo{data: domain.LongOpenCaseReportData{
		Cases:                make([]domain.LongOpenCaseReportRow, 2),
		CasesTotal:           9000,
		OnboardingCasesTotal: 0,
	}}
	svc := service.NewLongOpenCaseReportService(repo, longOpenCfg())
	got, err := svc.BuildReport(context.Background(), domain.LongOpenCaseReportRequest{})
	if err != nil {
		t.Fatalf("BuildReport: %v", err)
	}
	if !got.CasesTruncated || got.OnboardingCasesTruncated || got.CasesTotal != 9000 {
		t.Errorf("truncated = %v/%v total %d, want true/false and the full total", got.CasesTruncated, got.OnboardingCasesTruncated, got.CasesTotal)
	}
}

func TestLongOpenCaseReportRejectsBadAgeAndMissingProjectTypes(t *testing.T) {
	repo := &fakeLongOpenCaseReportRepo{}
	svc := service.NewLongOpenCaseReportService(repo, longOpenCfg())
	for _, days := range []int{-1, service.LongOpenCaseReportMaxOlderThanDays + 1} {
		_, err := svc.BuildReport(context.Background(), domain.LongOpenCaseReportRequest{OlderThanDays: days})
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) {
			t.Errorf("olderThanDays=%d: err = %v, want a validation error", days, err)
		}
	}
	if repo.calls != 0 {
		t.Errorf("repository called %d times for invalid requests, want 0", repo.calls)
	}

	unconfigured := service.NewLongOpenCaseReportService(repo, service.LongOpenCaseReportConfig{ProjectTypes: []string{" "}})
	_, err := unconfigured.BuildReport(context.Background(), domain.LongOpenCaseReportRequest{})
	var sue *apierror.ServiceUnavailableError
	if !errors.As(err, &sue) {
		t.Errorf("no project types: err = %v, want service unavailable", err)
	}
}

func TestLongOpenCaseReportPropagatesRepositoryErrors(t *testing.T) {
	boom := errors.New("boom")
	svc := service.NewLongOpenCaseReportService(&fakeLongOpenCaseReportRepo{listErr: boom}, longOpenCfg())
	if _, err := svc.BuildReport(context.Background(), domain.LongOpenCaseReportRequest{}); !errors.Is(err, boom) {
		t.Errorf("err = %v, want %v", err, boom)
	}
}
