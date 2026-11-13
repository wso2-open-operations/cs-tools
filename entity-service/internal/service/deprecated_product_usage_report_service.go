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

// DeprecatedProductUsageReportService builds the monthly deprecated product
// usage report: customers running a product version whose support status is
// Deprecated in an active Primary Production deployment.
type DeprecatedProductUsageReportService interface {
	BuildReport(ctx context.Context, req domain.DeprecatedProductUsageReportRequest) (domain.DeprecatedProductUsageReport, error)
}

type deprecatedProductUsageReportService struct {
	repo repository.DeprecatedProductUsageReportRepository
	loc  *time.Location
}

// NewDeprecatedProductUsageReportService constructs a DeprecatedProductUsageReportService.
// The report date defaults to today in the same zone the other reports use.
func NewDeprecatedProductUsageReportService(repo repository.DeprecatedProductUsageReportRepository) DeprecatedProductUsageReportService {
	loc, err := time.LoadLocation(ProductRegressionReportTimezone)
	if err != nil {
		loc = time.UTC
	}
	return &deprecatedProductUsageReportService{repo: repo, loc: loc}
}

// BuildReport resolves the report date, lists the deprecated usage under the
// system identity and groups it by region, account and project.
func (s *deprecatedProductUsageReportService) BuildReport(ctx context.Context, req domain.DeprecatedProductUsageReportRequest) (domain.DeprecatedProductUsageReport, error) {
	now := req.Now
	if now.IsZero() {
		now = time.Now()
	}
	asOf := req.AsOf
	if asOf == "" {
		asOf = now.In(s.loc).Format("2006-01-02")
	} else if _, err := time.Parse("2006-01-02", asOf); err != nil {
		return domain.DeprecatedProductUsageReport{}, &apierror.ValidationError{Msg: "asOf must be a date (YYYY-MM-DD)"}
	}
	// The route is internal-only; read under the system identity so
	// row-level security on customer data never hides a row from the report.
	ctx = repository.WithSystemIdentity(ctx)
	rows, err := s.repo.ListDeprecatedUsage(ctx)
	if err != nil {
		return domain.DeprecatedProductUsageReport{}, err
	}
	report := GroupDeprecatedProductUsage(rows)
	report.GeneratedOn = now
	report.AsOf = asOf
	return report, nil
}

// GroupDeprecatedProductUsage nests the rows (already in report order) into
// regions, accounts and projects. Every list is non-nil, so an empty report
// serialises as empty lists.
func GroupDeprecatedProductUsage(rows []domain.DeprecatedProductUsageRow) domain.DeprecatedProductUsageReport {
	report := domain.DeprecatedProductUsageReport{Regions: []domain.DeprecatedProductUsageRegion{}}
	person := func(name, email string) *domain.DeprecatedProductUsagePerson {
		if email == "" && name == "" {
			return nil
		}
		return &domain.DeprecatedProductUsagePerson{Name: name, Email: email}
	}
	regionKey := "" // the current region group's spelling, as read
	for _, r := range rows {
		n := len(report.Regions)
		// The rows are ordered by lower(region): regions that differ only in
		// case are one group, shown with the first spelling met.
		if n == 0 || !strings.EqualFold(regionKey, r.Region) {
			regionKey = r.Region
			report.Regions = append(report.Regions, domain.DeprecatedProductUsageRegion{Region: deprecatedOptional(r.Region), Accounts: []domain.DeprecatedProductUsageAccount{}})
			n++
		}
		region := &report.Regions[n-1]
		a := len(region.Accounts)
		if a == 0 || region.Accounts[a-1].AccountID != r.AccountID {
			region.Accounts = append(region.Accounts, domain.DeprecatedProductUsageAccount{
				AccountID: r.AccountID, AccountName: r.AccountName, CsAbt: deprecatedOptional(r.CsAbt), Region: deprecatedOptional(r.Region),
				AccountManager: person(r.AccountManagerName, r.AccountManagerEmail),
				TechnicalOwner: person(r.TechnicalOwnerName, r.TechnicalOwnerEmail),
				Projects:       []domain.DeprecatedProductUsageProject{},
			})
			a++
			report.TotalAccounts++
		}
		account := &region.Accounts[a-1]
		p := len(account.Projects)
		if p == 0 || account.Projects[p-1].ProjectID != r.ProjectID {
			account.Projects = append(account.Projects, domain.DeprecatedProductUsageProject{
				ProjectID: r.ProjectID, ProjectName: r.ProjectName, ProjectKey: deprecatedOptional(r.ProjectKey),
				Versions: []domain.DeprecatedProductUsageVersion{},
			})
			p++
		}
		project := &account.Projects[p-1]
		project.Versions = append(project.Versions, domain.DeprecatedProductUsageVersion{
			ProductVersionID: r.ProductVersionID, ProductName: r.ProductName, Version: r.Version,
			SupportStatus: r.SupportStatus, EolDate: deprecatedOptional(r.EolDate),
		})
		report.TotalVersions++
	}
	return report
}

// deprecatedOptional is a report text field: nil when the row has none, so it
// serialises as null rather than "".
func deprecatedOptional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
