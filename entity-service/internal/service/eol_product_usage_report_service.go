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
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// EolProductUsageReportService builds the monthly EOL product usage report:
// customers running a product version past its EOL date in an active
// Primary Production deployment.
type EolProductUsageReportService interface {
	BuildReport(ctx context.Context, req domain.EolProductUsageReportRequest) (domain.EolProductUsageReport, error)
}

type eolProductUsageReportService struct {
	repo repository.EolProductUsageReportRepository
	loc  *time.Location
}

// NewEolProductUsageReportService constructs an EolProductUsageReportService.
// The report date defaults to today in the same zone the other reports use.
func NewEolProductUsageReportService(repo repository.EolProductUsageReportRepository) EolProductUsageReportService {
	loc, err := time.LoadLocation(ProductRegressionReportTimezone)
	if err != nil {
		loc = time.UTC
	}
	return &eolProductUsageReportService{repo: repo, loc: loc}
}

// BuildReport resolves the report date, lists the EOL usage under the
// system identity and groups it by region, account and project.
func (s *eolProductUsageReportService) BuildReport(ctx context.Context, req domain.EolProductUsageReportRequest) (domain.EolProductUsageReport, error) {
	now := req.Now
	if now.IsZero() {
		now = time.Now()
	}
	asOf := req.AsOf
	if asOf == "" {
		asOf = now.In(s.loc).Format("2006-01-02")
	} else if _, err := time.Parse("2006-01-02", asOf); err != nil {
		return domain.EolProductUsageReport{}, &apierror.ValidationError{Msg: "asOf must be a date (YYYY-MM-DD)"}
	}
	// The route is internal-only; read under the system identity so
	// row-level security on customer data never hides a row from the report.
	ctx = repository.WithSystemIdentity(ctx)
	rows, err := s.repo.ListEolUsage(ctx, asOf)
	if err != nil {
		return domain.EolProductUsageReport{}, err
	}
	report := GroupEolProductUsage(rows)
	report.GeneratedOn = now
	report.AsOf = asOf
	return report, nil
}

// GroupEolProductUsage nests the rows (already in report order) into
// regions, accounts and projects. Every list is non-nil, so an empty report
// serialises as empty lists.
func GroupEolProductUsage(rows []domain.EolProductUsageRow) domain.EolProductUsageReport {
	report := domain.EolProductUsageReport{Regions: []domain.EolProductUsageRegion{}}
	person := func(name, email string) *domain.EolProductUsagePerson {
		if email == "" && name == "" {
			return nil
		}
		return &domain.EolProductUsagePerson{Name: name, Email: email}
	}
	regionKey := "" // the current region group's spelling, as read
	for _, r := range rows {
		n := len(report.Regions)
		// The rows are ordered by lower(region): regions that differ only in
		// case are one group, shown with the first spelling met.
		if n == 0 || !strings.EqualFold(regionKey, r.Region) {
			regionKey = r.Region
			report.Regions = append(report.Regions, domain.EolProductUsageRegion{Region: eolOptional(r.Region), Accounts: []domain.EolProductUsageAccount{}})
			n++
		}
		region := &report.Regions[n-1]
		a := len(region.Accounts)
		if a == 0 || region.Accounts[a-1].AccountID != r.AccountID {
			region.Accounts = append(region.Accounts, domain.EolProductUsageAccount{
				AccountID: r.AccountID, AccountName: r.AccountName, CsAbt: eolOptional(r.CsAbt), Region: eolOptional(r.Region),
				AccountManager: person(r.AccountManagerName, r.AccountManagerEmail),
				TechnicalOwner: person(r.TechnicalOwnerName, r.TechnicalOwnerEmail),
				Projects:       []domain.EolProductUsageProject{},
			})
			a++
			report.TotalAccounts++
		}
		account := &region.Accounts[a-1]
		p := len(account.Projects)
		if p == 0 || account.Projects[p-1].ProjectID != r.ProjectID {
			account.Projects = append(account.Projects, domain.EolProductUsageProject{
				ProjectID: r.ProjectID, ProjectName: r.ProjectName, ProjectKey: eolOptional(r.ProjectKey),
				Versions: []domain.EolProductUsageVersion{},
			})
			p++
		}
		project := &account.Projects[p-1]
		project.Versions = append(project.Versions, domain.EolProductUsageVersion{
			ProductVersionID: r.ProductVersionID, ProductName: r.ProductName, Version: r.Version,
			SupportStatus: r.SupportStatus, EolDate: r.EolDate,
		})
		report.TotalVersions++
	}
	return report
}

// eolOptional is a report text field: nil when the row has none, so it
// serialises as null rather than "".
func eolOptional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
