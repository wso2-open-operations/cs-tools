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

type fakeEolUsageRepo struct {
	rows    []domain.EolProductUsageRow
	gotAsOf string
}

func (f *fakeEolUsageRepo) ListEolUsage(_ context.Context, asOf string) ([]domain.EolProductUsageRow, error) {
	f.gotAsOf = asOf
	return f.rows, nil
}

func eolRow(region, account, project, version string) domain.EolProductUsageRow {
	status := "EXTENDED"
	return domain.EolProductUsageRow{
		Region: region, AccountID: "acc-" + account, AccountName: account, CsAbt: "Team Alpha",
		AccountManagerEmail: "am@" + account, TechnicalOwnerEmail: "to@" + account,
		ProjectID: "proj-" + project, ProjectName: project, ProjectKey: "KEY",
		ProductVersionID: "pv-" + version, ProductName: "WSO2 API Manager", Version: version,
		SupportStatus: &status, EolDate: "2022-10-22",
	}
}

func TestEolProductUsageReportDefaultsAsOfToTodayInColombo(t *testing.T) {
	repo := &fakeEolUsageRepo{}
	// 20:00 UTC on 1 Nov is already 2 Nov in Colombo (+05:30).
	now := time.Date(2026, 11, 1, 20, 0, 0, 0, time.UTC)
	got, err := service.NewEolProductUsageReportService(repo).BuildReport(context.Background(), domain.EolProductUsageReportRequest{Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if repo.gotAsOf != "2026-11-02" || got.AsOf != "2026-11-02" {
		t.Fatalf("asOf passed %q, reported %q, want 2026-11-02", repo.gotAsOf, got.AsOf)
	}
	if got.Regions == nil || got.TotalVersions != 0 {
		t.Fatalf("empty report = %+v, want empty (non-nil) regions", got)
	}
}

func TestEolProductUsageReportRejectsABadAsOf(t *testing.T) {
	_, err := service.NewEolProductUsageReportService(&fakeEolUsageRepo{}).BuildReport(context.Background(),
		domain.EolProductUsageReportRequest{AsOf: "02/11/2026"})
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want a validation error", err)
	}
}

func TestGroupEolProductUsageNestsRegionAccountProject(t *testing.T) {
	rows := []domain.EolProductUsageRow{
		eolRow("APAC", "Acme", "Acme Prod", "2.6.0"),
		eolRow("APAC", "Acme", "Acme Prod", "3.2.0"),
		eolRow("APAC", "Acme", "Acme Sub", "2.6.0"),
		eolRow("APAC", "Beta", "Beta Prod", "4.0.0"),
		eolRow("", "Gamma", "Gamma Prod", "5.0.0"),
	}
	rows[3].AccountManagerEmail = ""
	got := service.GroupEolProductUsage(rows)
	if got.TotalVersions != 5 || got.TotalAccounts != 3 || len(got.Regions) != 2 {
		t.Fatalf("totals %d versions, %d accounts, %d regions; want 5, 3, 2", got.TotalVersions, got.TotalAccounts, len(got.Regions))
	}
	apac := got.Regions[0]
	if apac.Region != "APAC" || len(apac.Accounts) != 2 || len(apac.Accounts[0].Projects) != 2 || len(apac.Accounts[0].Projects[0].Versions) != 2 {
		t.Fatalf("APAC grouping = %+v", apac)
	}
	if apac.Accounts[1].AccountManager != nil {
		t.Errorf("an account with no manager should have a nil accountManager, got %+v", apac.Accounts[1].AccountManager)
	}
	if apac.Accounts[0].AccountManager == nil || apac.Accounts[0].AccountManager.Email != "am@Acme" {
		t.Errorf("account manager = %+v", apac.Accounts[0].AccountManager)
	}
	if got.Regions[1].Region != "" || got.Regions[1].Accounts[0].AccountName != "Gamma" {
		t.Errorf("no-region group = %+v, want Gamma last", got.Regions[1])
	}
}

func TestGroupEolProductUsageTreatsRegionsDifferingInCaseAsOne(t *testing.T) {
	// The query orders by lower(region), so "EMEA" and "emea" rows arrive together.
	rows := []domain.EolProductUsageRow{
		eolRow("EMEA", "Acme", "Acme Prod", "2.6.0"),
		eolRow("emea", "Beta", "Beta Prod", "4.0.0"),
		eolRow("EMEA", "Delta", "Delta Prod", "3.2.0"),
	}
	got := service.GroupEolProductUsage(rows)
	if len(got.Regions) != 1 || got.Regions[0].Region != "EMEA" || len(got.Regions[0].Accounts) != 3 {
		t.Fatalf("regions = %+v, want one EMEA group with 3 accounts", got.Regions)
	}
}
