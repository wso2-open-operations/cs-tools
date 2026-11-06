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
	"fmt"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// LongOpenCaseReportService builds the long-open cases report.
type LongOpenCaseReportService interface {
	BuildReport(ctx context.Context, req domain.LongOpenCaseReportRequest) (domain.LongOpenCaseReport, error)
}

// LongOpenCaseReportDefaultOlderThanDays is the age a case must reach before
// the report lists it when the caller does not say.
const LongOpenCaseReportDefaultOlderThanDays = 30

// LongOpenCaseReportMaxOlderThanDays bounds the age a caller may ask for.
const LongOpenCaseReportMaxOlderThanDays = 3650

// LongOpenCaseReportMaxRowsPerTable is the defensive cap on each table. The
// report is expected to hold tens of rows; the cap exists so that a defect
// in the filter can never produce a response, or a mail, of tens of
// thousands. The totals still count every matching case.
const LongOpenCaseReportMaxRowsPerTable = 5000

// LongOpenCaseReportConfig names the project types and tags the report is
// built from.
type LongOpenCaseReportConfig struct {
	// ProjectTypes are the project type names a case's project must have
	// (compared exactly, like the case list's project type filter).
	ProjectTypes []string
	// ExcludedTags are tag names (compared ignoring case) that remove a
	// case from the report.
	ExcludedTags []string
}

type longOpenCaseReportService struct {
	repo repository.LongOpenCaseReportRepository
	cfg  LongOpenCaseReportConfig
	loc  *time.Location
}

// NewLongOpenCaseReportService constructs a LongOpenCaseReportService.
func NewLongOpenCaseReportService(repo repository.LongOpenCaseReportRepository, cfg LongOpenCaseReportConfig) LongOpenCaseReportService {
	loc, err := time.LoadLocation(MigrationCaseReportTimezone)
	if err != nil {
		// The zone is a constant; a missing tz database is the only way
		// this fails, and UTC is the honest fallback.
		loc = time.UTC
	}
	return &longOpenCaseReportService{repo: repo, cfg: cfg, loc: loc}
}

func (s *longOpenCaseReportService) BuildReport(ctx context.Context, req domain.LongOpenCaseReportRequest) (domain.LongOpenCaseReport, error) {
	days := req.OlderThanDays
	if days == 0 {
		days = LongOpenCaseReportDefaultOlderThanDays
	}
	if days < 1 || days > LongOpenCaseReportMaxOlderThanDays {
		return domain.LongOpenCaseReport{}, &apierror.ValidationError{
			Msg: fmt.Sprintf("olderThanDays must be between 1 and %d", LongOpenCaseReportMaxOlderThanDays)}
	}
	projectTypes := nonBlank(s.cfg.ProjectTypes)
	if len(projectTypes) == 0 {
		return domain.LongOpenCaseReport{}, &apierror.ServiceUnavailableError{
			Msg: "the long-open cases report's project types are not configured"}
	}
	now := req.Now
	if now.IsZero() {
		now = time.Now()
	}
	createdBefore := MigrationCaseCutoff(now, days, s.loc)

	// The route is internal-only; the read runs under the system identity
	// so row-level security does not hide cases from the report.
	ctx = repository.WithSystemIdentity(ctx)
	data, err := s.repo.ListLongOpenCases(ctx, createdBefore, projectTypes, nonBlank(s.cfg.ExcludedTags), LongOpenCaseReportMaxRowsPerTable)
	if err != nil {
		return domain.LongOpenCaseReport{}, err
	}

	report := domain.LongOpenCaseReport{
		GeneratedAt:              now,
		OlderThanDays:            days,
		CreatedBefore:            createdBefore,
		Cases:                    data.Cases,
		OnboardingCases:          data.OnboardingCases,
		CasesTotal:               data.CasesTotal,
		OnboardingCasesTotal:     data.OnboardingCasesTotal,
		CasesTruncated:           data.CasesTotal > len(data.Cases),
		OnboardingCasesTruncated: data.OnboardingCasesTotal > len(data.OnboardingCases),
	}
	// An empty table serialises as an empty list, not null.
	if report.Cases == nil {
		report.Cases = []domain.LongOpenCaseReportRow{}
	}
	if report.OnboardingCases == nil {
		report.OnboardingCases = []domain.LongOpenCaseReportRow{}
	}
	return report, nil
}

// nonBlank returns the trimmed, non-empty entries of in.
func nonBlank(in []string) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		if t := strings.TrimSpace(v); t != "" {
			out = append(out, t)
		}
	}
	return out
}
