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

type fakeMTTRReportRepo struct {
	stats      []domain.MTTRSeverityStat
	gotStart   time.Time
	gotEnd     time.Time
	gotTeamIDs []string
	calls      int
	statsErr   error
}

func (f *fakeMTTRReportRepo) SeverityStats(_ context.Context, windowStart, windowEnd time.Time, creTeamIDs []string) ([]domain.MTTRSeverityStat, error) {
	f.calls++
	f.gotStart, f.gotEnd, f.gotTeamIDs = windowStart, windowEnd, creTeamIDs
	return f.stats, f.statsErr
}

func bandByName(bands []domain.MTTRReportBand, name string) domain.MTTRReportBand {
	for _, b := range bands {
		if b.Band == name {
			return b
		}
	}
	return domain.MTTRReportBand{}
}

func TestMTTRReportCombinesBandsAndResolvesWindow(t *testing.T) {
	repo := &fakeMTTRReportRepo{stats: []domain.MTTRSeverityStat{
		{Severity: "S1", CaseCount: 2, TotalResolutionSec: 14400}, // avg 2h
		{Severity: "S2", CaseCount: 3, TotalResolutionSec: 43200}, // avg 4h
		{Severity: "S4", CaseCount: 5, TotalResolutionSec: 18000}, // avg 1h
		// S3 deliberately absent, as if no case of that severity matched.
	}}
	svc := service.NewMTTRReportService(repo)
	now := time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC)

	got, err := svc.BuildReport(context.Background(), domain.MTTRReportRequest{
		WindowMonths: 3,
		CreTeamIDs:   []string{" 11111111-1111-1111-1111-111111111111 ", "", "22222222-2222-2222-2222-222222222222"},
		Now:          now,
	})
	if err != nil {
		t.Fatalf("BuildReport: %v", err)
	}
	if got.WindowMonths != 3 || !got.GeneratedOn.Equal(now) {
		t.Errorf("WindowMonths/GeneratedOn = %d/%s, want 3/%s", got.WindowMonths, got.GeneratedOn, now)
	}
	wantStart := time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC)
	if !got.WindowStart.Equal(wantStart) || !got.WindowEnd.Equal(now) {
		t.Errorf("window = [%s, %s), want [%s, %s)", got.WindowStart, got.WindowEnd, wantStart, now)
	}
	if !repo.gotStart.Equal(wantStart) || !repo.gotEnd.Equal(now) {
		t.Errorf("repo window passed = [%s, %s), want [%s, %s)", repo.gotStart, repo.gotEnd, wantStart, now)
	}
	wantTeamIDs := []string{"11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222"}
	if !equalStrings(repo.gotTeamIDs, wantTeamIDs) || !equalStrings(got.CreTeamIDs, wantTeamIDs) {
		t.Errorf("team ids passed = %v, echoed = %v, want the trimmed, non-blank values", repo.gotTeamIDs, got.CreTeamIDs)
	}

	cases := []struct {
		band      string
		wantCount int
		wantAvg   float64
	}{
		{"S1-S4", 10, 2.1},
		{"S1-S3", 5, 3.2},
		{"S0", 0, 0},
		{"S1", 2, 2},
		{"S2", 3, 4},
		{"S3", 0, 0},
		{"S4", 5, 1},
	}
	for _, c := range cases {
		b := bandByName(got.Bands, c.band)
		if b.CaseCount != c.wantCount {
			t.Errorf("band %s: CaseCount = %d, want %d", c.band, b.CaseCount, c.wantCount)
		}
		if diff := b.AvgResolutionHours - c.wantAvg; diff > 1e-9 || diff < -1e-9 {
			t.Errorf("band %s: AvgResolutionHours = %v, want %v", c.band, b.AvgResolutionHours, c.wantAvg)
		}
	}
}

func TestMTTRReportDayWindow(t *testing.T) {
	repo := &fakeMTTRReportRepo{}
	svc := service.NewMTTRReportService(repo)
	now := time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC)
	got, err := svc.BuildReport(context.Background(), domain.MTTRReportRequest{WindowDays: 7, Now: now})
	if err != nil {
		t.Fatalf("BuildReport: %v", err)
	}
	wantStart := time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)
	if got.WindowDays != 7 || got.WindowMonths != 0 || !got.WindowStart.Equal(wantStart) || !repo.gotStart.Equal(wantStart) {
		t.Errorf("got days=%d months=%d start=%s, want 7/0/%s", got.WindowDays, got.WindowMonths, got.WindowStart, wantStart)
	}
	for _, req := range []domain.MTTRReportRequest{
		{WindowDays: 3},
		{WindowDays: 7, WindowMonths: 3},
	} {
		if _, err := svc.BuildReport(context.Background(), req); err == nil {
			t.Errorf("request %+v accepted, want a validation error", req)
		}
	}
}

func TestMTTRReportRejectsUnsupportedWindow(t *testing.T) {
	repo := &fakeMTTRReportRepo{}
	svc := service.NewMTTRReportService(repo)
	for _, months := range []int{0, 2, 24, -1} {
		_, err := svc.BuildReport(context.Background(), domain.MTTRReportRequest{WindowMonths: months})
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) {
			t.Errorf("months=%d: err = %v, want a validation error", months, err)
		}
	}
	if repo.calls != 0 {
		t.Errorf("repository called %d times for invalid requests, want 0", repo.calls)
	}
}

// TestMTTRReportRejectsMalformedCreTeamID: entity-service is a direct,
// internal-only HTTP route -- not only reachable through the portal
// backend's own creTeamId UUID check -- so a malformed id must be rejected
// here too, before it ever reaches SQL expecting a uuid[].
func TestMTTRReportRejectsMalformedCreTeamID(t *testing.T) {
	repo := &fakeMTTRReportRepo{}
	svc := service.NewMTTRReportService(repo)
	_, err := svc.BuildReport(context.Background(), domain.MTTRReportRequest{
		WindowMonths: 3,
		CreTeamIDs:   []string{"not-a-uuid"},
	})
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("err = %v, want a validation error", err)
	}
	if repo.calls != 0 {
		t.Errorf("repository called %d times for a malformed creTeamId, want 0", repo.calls)
	}
}

func TestMTTRReportNoDataIsZeroNotNaN(t *testing.T) {
	svc := service.NewMTTRReportService(&fakeMTTRReportRepo{})
	got, err := svc.BuildReport(context.Background(), domain.MTTRReportRequest{WindowMonths: 1})
	if err != nil {
		t.Fatalf("BuildReport: %v", err)
	}
	for _, b := range got.Bands {
		if b.CaseCount != 0 || b.AvgResolutionHours != 0 {
			t.Errorf("band %s = %+v, want zero count and zero average with no data", b.Band, b)
		}
	}
}

func TestMTTRReportPropagatesRepositoryErrors(t *testing.T) {
	boom := errors.New("boom")
	svc := service.NewMTTRReportService(&fakeMTTRReportRepo{statsErr: boom})
	if _, err := svc.BuildReport(context.Background(), domain.MTTRReportRequest{WindowMonths: 6}); !errors.Is(err, boom) {
		t.Errorf("err = %v, want %v", err, boom)
	}
}
