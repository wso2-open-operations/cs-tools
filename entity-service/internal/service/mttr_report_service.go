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
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// MTTRReportService builds the MTTR (mean time to resolution) report.
type MTTRReportService interface {
	BuildReport(ctx context.Context, req domain.MTTRReportRequest) (domain.MTTRReport, error)
}

// mttrReportAllowedWindowMonths are the only trailing windows the report
// accepts -- the dashboard's own fixed choice of 1/3/6/12 months, not an
// open-ended day count.
var mttrReportAllowedWindowMonths = map[int]bool{1: true, 3: true, 6: true, 12: true}

// mttrReportBands names, in response order, which case_severity_enum labels
// each requested band combines. S0 (catastrophic) is deliberately never
// included -- the requirement this report was built for only ever asked
// for S1 through S4.
var mttrReportBands = []struct {
	band       string
	severities []string
}{
	{"S1-S4", []string{"S1", "S2", "S3", "S4"}},
	{"S1-S3", []string{"S1", "S2", "S3"}},
	{"S1", []string{"S1"}},
	{"S2", []string{"S2"}},
	{"S3", []string{"S3"}},
	{"S4", []string{"S4"}},
}

type mttrReportService struct {
	repo repository.MTTRReportRepository
}

// NewMTTRReportService constructs an MTTRReportService.
func NewMTTRReportService(repo repository.MTTRReportRepository) MTTRReportService {
	return &mttrReportService{repo: repo}
}

func (s *mttrReportService) BuildReport(ctx context.Context, req domain.MTTRReportRequest) (domain.MTTRReport, error) {
	if !mttrReportAllowedWindowMonths[req.WindowMonths] {
		return domain.MTTRReport{}, &apierror.ValidationError{
			Msg: "months must be one of 1, 3, 6 or 12"}
	}
	now := req.Now
	if now.IsZero() {
		now = time.Now()
	}
	now = now.UTC()
	windowEnd := now
	windowStart := windowEnd.AddDate(0, -req.WindowMonths, 0)

	creTeamIDs := nonBlank(req.CreTeamIDs)

	// The route is internal-only; the read runs under the system identity
	// so row-level security does not hide cases outside the caller's own
	// projects from the aggregate.
	ctx = repository.WithSystemIdentity(ctx)
	stats, err := s.repo.SeverityStats(ctx, windowStart, windowEnd, creTeamIDs)
	if err != nil {
		return domain.MTTRReport{}, err
	}
	bySeverity := make(map[string]domain.MTTRSeverityStat, len(stats))
	for _, st := range stats {
		bySeverity[st.Severity] = st
	}

	bands := make([]domain.MTTRReportBand, 0, len(mttrReportBands))
	for _, b := range mttrReportBands {
		bands = append(bands, combineMTTRBand(b.band, b.severities, bySeverity))
	}

	return domain.MTTRReport{
		GeneratedAt:  now,
		WindowMonths: req.WindowMonths,
		WindowStart:  windowStart,
		WindowEnd:    windowEnd,
		CreTeamIDs:   creTeamIDs,
		Bands:        bands,
	}, nil
}

// combineMTTRBand sums the case count and total resolution time of every
// severity in severities and reduces them to one average. Summing first and
// dividing once -- rather than averaging each severity's own average -- is
// what keeps a combined band (e.g. S1-S4) weighted correctly when its
// severities hold very different case counts.
func combineMTTRBand(band string, severities []string, bySeverity map[string]domain.MTTRSeverityStat) domain.MTTRReportBand {
	var count int
	var totalSec float64
	for _, sev := range severities {
		if st, ok := bySeverity[sev]; ok {
			count += st.CaseCount
			totalSec += st.TotalResolutionSec
		}
	}
	out := domain.MTTRReportBand{Band: band, CaseCount: count}
	if count > 0 {
		out.AvgResolutionHours = totalSec / float64(count) / 3600
	}
	return out
}
