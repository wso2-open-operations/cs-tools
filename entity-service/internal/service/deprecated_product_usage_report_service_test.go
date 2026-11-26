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
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

type fakeDeprecatedUsageRepo struct {
	rows  []domain.DeprecatedProductUsageRow
	calls int
}

func (f *fakeDeprecatedUsageRepo) ListDeprecatedUsage(_ context.Context) ([]domain.DeprecatedProductUsageRow, error) {
	f.calls++
	return f.rows, nil
}

// deprecatedDeref is a report text field's value, "" for null.
func deprecatedDeref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func deprecatedRow(region, account, project, version string) domain.DeprecatedProductUsageRow {
	status := "DEPRECATED"
	return domain.DeprecatedProductUsageRow{
		Region: region, AccountID: "acc-" + account, AccountName: account, CsAbt: "Team Alpha",
		AccountManagerEmail: "am@" + account, TechnicalOwnerEmail: "to@" + account,
		ProjectID: "proj-" + project, ProjectName: project, ProjectKey: "KEY",
		ProductVersionID: "pv-" + version, ProductName: "WSO2 API Manager", Version: version,
		SupportStatus: &status, EolDate: "2022-10-22",
	}
}

func TestDeprecatedProductUsageReportDefaultsAsOfToTodayInColombo(t *testing.T) {
	repo := &fakeDeprecatedUsageRepo{}
	// 20:00 UTC on 1 Nov is already 2 Nov in Colombo (+05:30).
	now := time.Date(2026, 11, 1, 20, 0, 0, 0, time.UTC)
	got, err := service.NewDeprecatedProductUsageReportService(repo).BuildReport(context.Background(), domain.DeprecatedProductUsageReportRequest{Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if repo.calls != 1 || got.AsOf != "2026-11-02" {
		t.Fatalf("repo calls %d, reported asOf %q, want 1 and 2026-11-02", repo.calls, got.AsOf)
	}
	if got.Regions == nil || got.TotalVersions != 0 {
		t.Fatalf("empty report = %+v, want empty (non-nil) regions", got)
	}
}

func TestDeprecatedProductUsageReportRejectsABadAsOf(t *testing.T) {
	_, err := service.NewDeprecatedProductUsageReportService(&fakeDeprecatedUsageRepo{}).BuildReport(context.Background(),
		domain.DeprecatedProductUsageReportRequest{AsOf: "02/11/2026"})
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want a validation error", err)
	}
}

func TestGroupDeprecatedProductUsageNestsRegionAccountProject(t *testing.T) {
	rows := []domain.DeprecatedProductUsageRow{
		deprecatedRow("APAC", "Acme", "Acme Prod", "2.6.0"),
		deprecatedRow("APAC", "Acme", "Acme Prod", "3.2.0"),
		deprecatedRow("APAC", "Acme", "Acme Sub", "2.6.0"),
		deprecatedRow("APAC", "Beta", "Beta Prod", "4.0.0"),
		deprecatedRow("", "Gamma", "Gamma Prod", "5.0.0"),
	}
	rows[3].AccountManagerEmail = ""
	got := service.GroupDeprecatedProductUsage(rows)
	if got.TotalVersions != 5 || got.TotalAccounts != 3 || len(got.Regions) != 2 {
		t.Fatalf("totals %d versions, %d accounts, %d regions; want 5, 3, 2", got.TotalVersions, got.TotalAccounts, len(got.Regions))
	}
	apac := got.Regions[0]
	if deprecatedDeref(apac.Region) != "APAC" || len(apac.Accounts) != 2 || len(apac.Accounts[0].Projects) != 2 || len(apac.Accounts[0].Projects[0].Versions) != 2 {
		t.Fatalf("APAC grouping = %+v", apac)
	}
	if apac.Accounts[1].AccountManager != nil {
		t.Errorf("an account with no manager should have a nil accountManager, got %+v", apac.Accounts[1].AccountManager)
	}
	if apac.Accounts[0].AccountManager == nil || apac.Accounts[0].AccountManager.Email != "am@Acme" {
		t.Errorf("account manager = %+v", apac.Accounts[0].AccountManager)
	}
	if got.Regions[1].Region != nil || got.Regions[1].Accounts[0].AccountName != "Gamma" {
		t.Errorf("no-region group = %+v, want Gamma last", got.Regions[1])
	}
}

func TestGroupDeprecatedProductUsageTreatsRegionsDifferingInCaseAsOne(t *testing.T) {
	// The query orders by lower(region), so "EMEA" and "emea" rows arrive together.
	rows := []domain.DeprecatedProductUsageRow{
		deprecatedRow("EMEA", "Acme", "Acme Prod", "2.6.0"),
		deprecatedRow("emea", "Beta", "Beta Prod", "4.0.0"),
		deprecatedRow("EMEA", "Delta", "Delta Prod", "3.2.0"),
	}
	got := service.GroupDeprecatedProductUsage(rows)
	if len(got.Regions) != 1 || deprecatedDeref(got.Regions[0].Region) != "EMEA" || len(got.Regions[0].Accounts) != 3 {
		t.Fatalf("regions = %+v, want one EMEA group with 3 accounts", got.Regions)
	}
}

func TestDeprecatedProductUsageReportSendsNullNotEmptyStrings(t *testing.T) {
	row := deprecatedRow("", "Acme", "Acme Prod", "2.6.0")
	row.CsAbt, row.ProjectKey, row.AccountManagerEmail, row.EolDate = "", "", "", ""
	row.AccountManagerName = "Person One"
	got := service.GroupDeprecatedProductUsage([]domain.DeprecatedProductUsageRow{row})
	got.AsOf = "2026-11-02" // set by BuildReport
	out, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	body := string(out)
	for _, want := range []string{`"region":null`, `"csAbt":null`, `"projectKey":null`, `"eolDate":null`, `"accountManager":{"name":"Person One"}`} {
		if !strings.Contains(body, want) {
			t.Errorf("report JSON lacks %s:\n%s", want, body)
		}
	}
	if strings.Contains(body, `""`) {
		t.Errorf("report JSON has an empty string:\n%s", body)
	}
}
