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
	"slices"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// SLABreachReportService builds the weekly SLA breach report.
type SLABreachReportService interface {
	BuildReport(ctx context.Context, req domain.SLABreachReportRequest) (domain.SLABreachReport, error)
}

// SLABreachReportTimezone is the zone the week is taken in -- the same zone
// every other scheduled report here resolves its period in.
const SLABreachReportTimezone = "Asia/Colombo"

// SLABreachReportMaxDefinitions bounds how many policy names one request
// may carry.
const SLABreachReportMaxDefinitions = 50

// slaBreachOnboardingStatuses are the project.onboarding_status values that
// put a row in the report's Onboarding section rather than the CRE one --
// the source report's own "Onboarding Status is not one of In-Progress,
// Not-Started" filter, read as a split rather than an exclusion so one
// query serves both halves.
//
// A THIRD hand-kept copy of the same list: repository's own
// slaClockStateOnboardingStatuses and csm-notification-service's
// chataudience.onboardingStatuses are the other two. Changing one without
// the others silently disagrees about which team owns a case.
var slaBreachOnboardingStatuses = []string{"IN_PROGRESS", "NOT_STARTED"}

type slaBreachReportService struct {
	repo repository.SLABreachReportRepository
	loc  *time.Location
}

// NewSLABreachReportService constructs a SLABreachReportService.
func NewSLABreachReportService(repo repository.SLABreachReportRepository) SLABreachReportService {
	loc, err := time.LoadLocation(SLABreachReportTimezone)
	if err != nil {
		// The zone is a constant; a missing tz database is the only way
		// this fails, and UTC is the honest fallback.
		loc = time.UTC
	}
	return &slaBreachReportService{repo: repo, loc: loc}
}

// SLABreachReportWeek resolves the Monday-to-Monday window the report
// covers: the full calendar week before the one `week` falls in, in loc.
//
// ServiceNow's own "Breach time on Last week" means the previous whole
// week, never a trailing seven days, so a run on any day of week N always
// reports week N-1 unchanged -- a retry or a manual re-run later the same
// week produces the identical report rather than a sliding window.
func SLABreachReportWeek(week, now time.Time, loc *time.Location) (time.Time, time.Time) {
	if week.IsZero() {
		if now.IsZero() {
			now = time.Now()
		}
		week = now
	}
	local := week.In(loc)
	// Monday as the week's first day: Go's Sunday==0 needs the shift.
	offset := (int(local.Weekday()) + 6) % 7
	thisMonday := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, -offset)
	return thisMonday.AddDate(0, 0, -7), thisMonday
}

// BuildReport validates the request, resolves the week and splits the
// week's breaches into the onboarding and CRE sections.
func (s *slaBreachReportService) BuildReport(ctx context.Context, req domain.SLABreachReportRequest) (domain.SLABreachReport, error) {
	definitions := make([]string, 0, len(req.Definitions))
	for _, d := range req.Definitions {
		if d = strings.TrimSpace(d); d != "" {
			definitions = append(definitions, d)
		}
	}
	if len(definitions) == 0 {
		// Never defaulted to "every policy": that would quietly widen the
		// report to definitions it was never meant to cover.
		return domain.SLABreachReport{}, &apierror.ValidationError{Msg: "definitions is required"}
	}
	if len(definitions) > SLABreachReportMaxDefinitions {
		return domain.SLABreachReport{}, &apierror.ValidationError{
			Msg: fmt.Sprintf("definitions may name at most %d SLA definitions", SLABreachReportMaxDefinitions)}
	}

	stages := make([]string, 0, len(req.Stages))
	for _, st := range req.Stages {
		if st = strings.ToUpper(strings.TrimSpace(st)); st != "" {
			stages = append(stages, st)
		}
	}

	from, to := SLABreachReportWeek(req.Week, req.Now, s.loc)

	rows, err := s.repo.ListBreaches(ctx, from, to, definitions, stages)
	if err != nil {
		return domain.SLABreachReport{}, err
	}

	report := domain.SLABreachReport{
		PeriodStart: from,
		PeriodEnd:   to,
		Onboarding:  make([]domain.SLABreachRow, 0),
		CRE:         make([]domain.SLABreachRow, 0),
	}
	for _, row := range rows {
		if slices.Contains(slaBreachOnboardingStatuses, row.OnboardingStatus) {
			report.Onboarding = append(report.Onboarding, row)
			continue
		}
		report.CRE = append(report.CRE, row)
	}
	return report, nil
}
