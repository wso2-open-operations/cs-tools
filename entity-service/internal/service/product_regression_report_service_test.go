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
	"fmt"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

type fakeProductRegressionReportRepo struct {
	rows        []domain.ProductRegressionReportRow
	err         error
	gotStart    time.Time
	gotEnd      time.Time
	gotTag      string
	gotProducts []string
	calls       int
}

func (f *fakeProductRegressionReportRepo) ListRegressionCases(_ context.Context, periodStart, periodEnd time.Time, tag string, products []string) ([]domain.ProductRegressionReportRow, error) {
	f.calls++
	f.gotStart, f.gotEnd, f.gotTag, f.gotProducts = periodStart, periodEnd, tag, products
	return f.rows, f.err
}

func regressionRow(number, product string, shared time.Time) domain.ProductRegressionReportRow {
	return domain.ProductRegressionReportRow{ID: "id-" + number, Number: number, Product: product, EtaSharedOn: shared}
}

// tooManyProducts is n distinct, non-blank product names: enough to reach
// the limit check rather than the empty-list one.
func tooManyProducts(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("Product %d", i)
	}
	return out
}

func colombo(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(service.ProductRegressionReportTimezone)
	if err != nil {
		t.Fatalf("load %s: %v", service.ProductRegressionReportTimezone, err)
	}
	return loc
}

func TestProductRegressionReportMonthDefaultsToLastMonth(t *testing.T) {
	loc := colombo(t)
	// The task fires at 03:30 UTC on the 1st, which is 09:00 in Colombo on
	// the 1st: the month before is the whole of the previous month.
	run := time.Date(2026, 10, 1, 3, 30, 0, 0, time.UTC)
	start, end, err := service.ProductRegressionReportMonth("", run, loc)
	if err != nil {
		t.Fatalf("month: %v", err)
	}
	if want := time.Date(2026, 9, 1, 0, 0, 0, 0, loc); !start.Equal(want) {
		t.Errorf("start = %s, want %s", start, want)
	}
	if want := time.Date(2026, 10, 1, 0, 0, 0, 0, loc); !end.Equal(want) {
		t.Errorf("end = %s, want %s", end, want)
	}
	// A retry later in the month still reports the same month.
	start2, end2, _ := service.ProductRegressionReportMonth("", run.AddDate(0, 0, 20), loc)
	if !start2.Equal(start) || !end2.Equal(end) {
		t.Errorf("retry on the 21st reported %s..%s, want the same month", start2, end2)
	}
}

func TestProductRegressionReportMonthTakesTheZoneIntoAccount(t *testing.T) {
	loc := colombo(t)
	// 20:00 UTC on 30 September is already 1 October in Colombo, so the
	// month before is September, not August.
	run := time.Date(2026, 9, 30, 20, 0, 0, 0, time.UTC)
	start, _, err := service.ProductRegressionReportMonth("", run, loc)
	if err != nil {
		t.Fatalf("month: %v", err)
	}
	if want := time.Date(2026, 9, 1, 0, 0, 0, 0, loc); !start.Equal(want) {
		t.Errorf("start = %s, want %s: the calendar is Colombo's, not UTC's", start, want)
	}
}

func TestProductRegressionReportMonthExplicit(t *testing.T) {
	loc := colombo(t)
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	start, end, err := service.ProductRegressionReportMonth("2026-07", now, loc)
	if err != nil {
		t.Fatalf("month: %v", err)
	}
	if !start.Equal(time.Date(2026, 7, 1, 0, 0, 0, 0, loc)) || !end.Equal(time.Date(2026, 8, 1, 0, 0, 0, 0, loc)) {
		t.Errorf("2026-07 = %s..%s", start, end)
	}
	for _, bad := range []string{"2026-10", "2027-01", "2026-7", "July 2026", "2026-13"} {
		_, _, err := service.ProductRegressionReportMonth(bad, now, loc)
		var ve *apierror.ValidationError
		if !errors.As(err, &ve) {
			t.Errorf("month %q: err = %v, want a validation error (unfinished or malformed month)", bad, err)
		}
	}
}

func TestGroupRegressionCases(t *testing.T) {
	shared := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	rows := []domain.ProductRegressionReportRow{
		regressionRow("CS1", "WSO2 API Manager", shared),
		regressionRow("CS2", "WSO2 API Manager", shared.AddDate(0, 0, 1)),
		regressionRow("CS3", "WSO2 Microgateway", shared),
	}
	got := service.GroupRegressionCases(rows)
	if got.TotalCases != 3 {
		t.Errorf("TotalCases = %d, want 3", got.TotalCases)
	}
	if len(got.Groups) != 2 || got.Groups[0].Product != "WSO2 API Manager" || got.Groups[1].Product != "WSO2 Microgateway" {
		t.Fatalf("groups = %+v, want API Manager then Microgateway", got.Groups)
	}
	if len(got.Groups[0].Cases) != 2 || got.Groups[0].Cases[0].Number != "CS1" || got.Groups[0].Cases[1].Number != "CS2" {
		t.Errorf("API Manager cases = %+v, want CS1 then CS2 in the order given", got.Groups[0].Cases)
	}
	empty := service.GroupRegressionCases(nil)
	if empty.Groups == nil || empty.Products == nil || empty.TotalCases != 0 {
		t.Errorf("empty report must serialise as empty lists, got %+v", empty)
	}
}

func TestProductRegressionReportBuildReport(t *testing.T) {
	loc := colombo(t)
	shared := time.Date(2026, 9, 10, 4, 0, 0, 0, time.UTC)
	repo := &fakeProductRegressionReportRepo{rows: []domain.ProductRegressionReportRow{
		regressionRow("CS1", "WSO2 Identity Server", shared)}}
	svc := service.NewProductRegressionReportService(repo, service.ProductRegressionReportConfig{Tag: "s_rg"})
	now := time.Date(2026, 10, 1, 3, 30, 0, 0, time.UTC)

	got, err := svc.BuildReport(context.Background(), domain.ProductRegressionReportRequest{
		Products: []string{" WSO2 Identity Server ", "", "WSO2 Identity Server Analytics"}, Now: now})
	if err != nil {
		t.Fatalf("BuildReport: %v", err)
	}
	if got.Month != "2026-09" {
		t.Errorf("Month = %q, want 2026-09", got.Month)
	}
	if !got.PeriodStart.Equal(time.Date(2026, 9, 1, 0, 0, 0, 0, loc)) || !got.PeriodStart.Equal(repo.gotStart) || !got.PeriodEnd.Equal(repo.gotEnd) {
		t.Errorf("period %s..%s differs from the window the repository was asked for, %s..%s", got.PeriodStart, got.PeriodEnd, repo.gotStart, repo.gotEnd)
	}
	if repo.gotTag != "s_rg" || got.Tag != "s_rg" {
		t.Errorf("tag passed = %q, reported %q, want s_rg", repo.gotTag, got.Tag)
	}
	if want := []string{"WSO2 Identity Server", "WSO2 Identity Server Analytics"}; !equalStrings(repo.gotProducts, want) || !equalStrings(got.Products, want) {
		t.Errorf("products passed = %v, reported %v, want %v trimmed with blanks dropped", repo.gotProducts, got.Products, want)
	}
	if !got.GeneratedAt.Equal(now) {
		t.Errorf("GeneratedAt = %s, want %s", got.GeneratedAt, now)
	}
	if got.TotalCases != 1 || len(got.Groups) != 1 {
		t.Errorf("report = %+v, want one group with one case", got)
	}
}

func TestProductRegressionReportRejectsBadInput(t *testing.T) {
	repo := &fakeProductRegressionReportRepo{}
	svc := service.NewProductRegressionReportService(repo, service.ProductRegressionReportConfig{Tag: "s_rg"})
	var ve *apierror.ValidationError
	for _, req := range []domain.ProductRegressionReportRequest{
		{},
		{Products: []string{" ", ""}},
		{Products: tooManyProducts(service.ProductRegressionReportMaxProducts + 1)},
		{Products: []string{"WSO2 API Manager"}, Month: "2099-01"},
	} {
		if _, err := svc.BuildReport(context.Background(), req); !errors.As(err, &ve) {
			t.Errorf("%+v: err = %v, want a validation error", req, err)
		}
	}
	if repo.calls != 0 {
		t.Errorf("repository was queried %d times for invalid requests", repo.calls)
	}

	unconfigured := service.NewProductRegressionReportService(repo, service.ProductRegressionReportConfig{})
	_, err := unconfigured.BuildReport(context.Background(), domain.ProductRegressionReportRequest{Products: []string{"WSO2 API Manager"}})
	var sue *apierror.ServiceUnavailableError
	if !errors.As(err, &sue) {
		t.Errorf("no tag configured: err = %v, want service unavailable", err)
	}
}

func TestProductRegressionReportPropagatesRepositoryErrors(t *testing.T) {
	boom := errors.New("boom")
	svc := service.NewProductRegressionReportService(&fakeProductRegressionReportRepo{err: boom}, service.ProductRegressionReportConfig{Tag: "s_rg"})
	if _, err := svc.BuildReport(context.Background(), domain.ProductRegressionReportRequest{Products: []string{"WSO2 API Manager"}}); !errors.Is(err, boom) {
		t.Errorf("err = %v, want %v", err, boom)
	}
}
