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

func openBeforeYesterdayCfg() service.LongOpenCaseReportConfig {
	cfg := longOpenCfg()
	cfg.OpenBeforeYesterdayProjectTypes = []string{" Subscription ", "", "Managed Cloud Subscription"}
	cfg.OpenBeforeYesterdayProductProjectTypes = []string{"Cloud Support", " "}
	return cfg
}

func TestOpenBeforeYesterdayBuildReport(t *testing.T) {
	repo := &fakeLongOpenCaseReportRepo{data: domain.LongOpenCaseReportData{
		Cases:      []domain.LongOpenCaseReportRow{{Number: "CS0000001", AccountName: "Example Corp"}, {Number: "CS0000002"}},
		CasesTotal: 2,
	}}
	svc := service.NewLongOpenCaseReportService(repo, openBeforeYesterdayCfg())
	// 2026-10-18 22:30 UTC is already 2026-10-19 04:00 in Colombo, so
	// yesterday starts at 2026-10-18 00:00 Colombo = 2026-10-17 18:30 UTC.
	now := time.Date(2026, 10, 18, 22, 30, 0, 0, time.UTC)

	got, err := svc.BuildReport(context.Background(), domain.LongOpenCaseReportRequest{
		Variant: domain.LongOpenCaseReportVariantOpenBeforeYesterday, Now: now})
	if err != nil {
		t.Fatalf("BuildReport: %v", err)
	}
	if want := time.Date(2026, 10, 17, 18, 30, 0, 0, time.UTC); !got.CreatedBefore.Equal(want) || !repo.gotCutoff.Equal(want) {
		t.Errorf("CreatedBefore = %s, cut-off passed = %s, want %s", got.CreatedBefore, repo.gotCutoff, want)
	}
	if !equalStrings(repo.gotTypes, []string{"Subscription", "Managed Cloud Subscription"}) ||
		!equalStrings(repo.gotProductTypes, []string{"Cloud Support"}) {
		t.Errorf("types passed = %v / %v, want the trimmed, non-blank values", repo.gotTypes, repo.gotProductTypes)
	}
	if repo.gotTags != nil {
		t.Errorf("excluded tags passed = %v, want none: this report has no tag filter", repo.gotTags)
	}
	if repo.gotLimit != service.LongOpenCaseReportMaxRowsPerTable {
		t.Errorf("limit = %d, want %d", repo.gotLimit, service.LongOpenCaseReportMaxRowsPerTable)
	}
	if got.Variant != domain.LongOpenCaseReportVariantOpenBeforeYesterday || got.OlderThanDays != 1 || !got.GeneratedAt.Equal(now) {
		t.Errorf("variant/olderThanDays/generatedAt = %q/%d/%s", got.Variant, got.OlderThanDays, got.GeneratedAt)
	}
	if len(got.Cases) != 2 || got.CasesTotal != 2 || got.CasesTruncated {
		t.Errorf("cases = %d total %d truncated %v, want 2/2/false", len(got.Cases), got.CasesTotal, got.CasesTruncated)
	}
	if got.OnboardingCases == nil || len(got.OnboardingCases) != 0 || got.OnboardingCasesTotal != 0 || got.OnboardingCasesTruncated {
		t.Errorf("onboarding table = %+v, want an empty, non-nil list with total 0", got.OnboardingCases)
	}
}

func TestOpenBeforeYesterdayCutoffIsCalendarDay(t *testing.T) {
	repo := &fakeLongOpenCaseReportRepo{}
	svc := service.NewLongOpenCaseReportService(repo, openBeforeYesterdayCfg())
	// Just after midnight in Colombo (2026-10-19 00:05 = 2026-10-18 18:35 UTC):
	// yesterday still starts at 2026-10-18 00:00 Colombo, not 24h earlier.
	now := time.Date(2026, 10, 18, 18, 35, 0, 0, time.UTC)
	if _, err := svc.BuildReport(context.Background(), domain.LongOpenCaseReportRequest{
		Variant: domain.LongOpenCaseReportVariantOpenBeforeYesterday, Now: now}); err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2026, 10, 17, 18, 30, 0, 0, time.UTC); !repo.gotCutoff.Equal(want) {
		t.Errorf("cut-off = %s, want %s", repo.gotCutoff, want)
	}
}

func TestOpenBeforeYesterdayTruncation(t *testing.T) {
	repo := &fakeLongOpenCaseReportRepo{data: domain.LongOpenCaseReportData{
		Cases: make([]domain.LongOpenCaseReportRow, 3), CasesTotal: 9000}}
	svc := service.NewLongOpenCaseReportService(repo, openBeforeYesterdayCfg())
	got, err := svc.BuildReport(context.Background(), domain.LongOpenCaseReportRequest{
		Variant: domain.LongOpenCaseReportVariantOpenBeforeYesterday})
	if err != nil {
		t.Fatal(err)
	}
	if !got.CasesTruncated || got.CasesTotal != 9000 || got.OnboardingCasesTruncated {
		t.Errorf("truncated = %v/%v total %d", got.CasesTruncated, got.OnboardingCasesTruncated, got.CasesTotal)
	}
}

func TestOpenBeforeYesterdayEmptyCasesNonNil(t *testing.T) {
	svc := service.NewLongOpenCaseReportService(&fakeLongOpenCaseReportRepo{}, openBeforeYesterdayCfg())
	got, err := svc.BuildReport(context.Background(), domain.LongOpenCaseReportRequest{
		Variant: domain.LongOpenCaseReportVariantOpenBeforeYesterday})
	if err != nil {
		t.Fatal(err)
	}
	if got.Cases == nil || got.OnboardingCases == nil {
		t.Errorf("empty tables must be non-nil: %+v", got)
	}
}

func TestOpenBeforeYesterdayValidation(t *testing.T) {
	repo := &fakeLongOpenCaseReportRepo{}
	svc := service.NewLongOpenCaseReportService(repo, openBeforeYesterdayCfg())
	for name, req := range map[string]domain.LongOpenCaseReportRequest{
		"unknown variant":    {Variant: "nope"},
		"with olderThanDays": {Variant: domain.LongOpenCaseReportVariantOpenBeforeYesterday, OlderThanDays: 5},
	} {
		_, err := svc.BuildReport(context.Background(), req)
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) {
			t.Errorf("%s: err = %v, want a validation error", name, err)
		}
	}
	unconfigured := service.NewLongOpenCaseReportService(repo, longOpenCfg())
	_, err := unconfigured.BuildReport(context.Background(), domain.LongOpenCaseReportRequest{
		Variant: domain.LongOpenCaseReportVariantOpenBeforeYesterday})
	var sue *apierror.ServiceUnavailableError
	if !errors.As(err, &sue) {
		t.Errorf("no project types: err = %v, want service unavailable", err)
	}
	if repo.calls != 0 {
		t.Errorf("repository called %d times for invalid requests, want 0", repo.calls)
	}
}

func TestOpenBeforeYesterdayPropagatesRepositoryErrors(t *testing.T) {
	boom := errors.New("boom")
	svc := service.NewLongOpenCaseReportService(&fakeLongOpenCaseReportRepo{listErr: boom}, openBeforeYesterdayCfg())
	if _, err := svc.BuildReport(context.Background(), domain.LongOpenCaseReportRequest{
		Variant: domain.LongOpenCaseReportVariantOpenBeforeYesterday}); !errors.Is(err, boom) {
		t.Errorf("err = %v, want %v", err, boom)
	}
}
