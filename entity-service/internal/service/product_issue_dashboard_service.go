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
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// ProductIssueDashboardTimezone is the calendar of the dashboard's periods and buckets.
const ProductIssueDashboardTimezone = "Asia/Kolkata"

// productIssueDashboardMaxDays is the longest period the dashboard accepts.
const productIssueDashboardMaxDays = 400

// ProductIssueDashboardRequest is GET /product-work-items/dashboard's query, unparsed.
type ProductIssueDashboardRequest struct {
	From, To, Bucket string
	Products         []string
}

// ProductIssueReportRequest is GET /reports/product-issues' query, unparsed.
type ProductIssueReportRequest struct {
	Period, Date string
	Products     []string
}

// ProductIssueDashboardService serves the dashboard, the report data and the lead list.
type ProductIssueDashboardService interface {
	Dashboard(ctx context.Context, req ProductIssueDashboardRequest) (domain.ProductIssueDashboard, error)
	Report(ctx context.Context, req ProductIssueReportRequest) (domain.ProductIssueDashboard, error)
	Recipients(ctx context.Context) (domain.ProductIssueRecipients, error)
}

type productIssueDashboardService struct {
	repo repository.ProductIssueDashboardRepository
	now  func() time.Time
	loc  *time.Location
}

// NewProductIssueDashboardService returns the service; now is time.Now when nil.
func NewProductIssueDashboardService(repo repository.ProductIssueDashboardRepository, now func() time.Time) ProductIssueDashboardService {
	loc, err := time.LoadLocation(ProductIssueDashboardTimezone)
	if err != nil {
		loc = time.FixedZone(ProductIssueDashboardTimezone, 5*3600+30*60) // no tz database: the offset has no DST
	}
	if now == nil {
		now = time.Now
	}
	return &productIssueDashboardService{repo: repo, now: now, loc: loc}
}

func parseDashboardDate(field, raw string, loc *time.Location) (time.Time, error) {
	t, err := time.ParseInLocation("2006-01-02", raw, loc)
	if err != nil {
		return time.Time{}, &apierror.ValidationError{Msg: field + " must be a date, formatted YYYY-MM-DD"}
	}
	return t, nil
}

// resolveDashboardPeriod is [from, to) in loc. Defaults: to = tomorrow, from = the
// Monday 12 weeks before the Monday of now's week. to must be after from, and the
// period at most 400 days.
func resolveDashboardPeriod(fromRaw, toRaw string, now time.Time, loc *time.Location) (time.Time, time.Time, error) {
	n := now.In(loc)
	today := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, loc)
	start := dashboardBucketStart(now, DashboardBucketWeek, loc).AddDate(0, 0, -12*7)
	end := today.AddDate(0, 0, 1)
	var err error
	if fromRaw != "" {
		if start, err = parseDashboardDate("from", fromRaw, loc); err != nil {
			return start, end, err
		}
	}
	if toRaw != "" {
		if end, err = parseDashboardDate("to", toRaw, loc); err != nil {
			return start, end, err
		}
	}
	if !end.After(start) {
		return start, end, &apierror.ValidationError{Msg: "to must be after from"}
	}
	if days := int(end.Sub(start).Hours()/24 + 0.5); days > productIssueDashboardMaxDays {
		return start, end, &apierror.ValidationError{Msg: fmt.Sprintf("the period must be at most %d days", productIssueDashboardMaxDays)}
	}
	return start, end, nil
}

// resolveReportPeriod is the week (Monday to Monday) or calendar month containing date;
// without a date, the previous full week or month.
func resolveReportPeriod(period, dateRaw string, now time.Time, loc *time.Location) (time.Time, time.Time, error) {
	if period != DashboardBucketWeek && period != DashboardBucketMonth {
		return time.Time{}, time.Time{}, &apierror.ValidationError{Msg: `period must be "week" or "month"`}
	}
	var start time.Time
	if dateRaw != "" {
		d, err := parseDashboardDate("date", dateRaw, loc)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
		start = dashboardBucketStart(d, period, loc)
	} else {
		cur := dashboardBucketStart(now, period, loc)
		if period == DashboardBucketMonth {
			start = cur.AddDate(0, -1, 0)
		} else {
			start = cur.AddDate(0, 0, -7)
		}
	}
	return start, dashboardBucketNext(start, period), nil
}

func (s *productIssueDashboardService) build(ctx context.Context, start, end time.Time, bucket string, products []string, now time.Time) (domain.ProductIssueDashboard, error) {
	rows, err := s.repo.ListRows(ctx)
	if err != nil {
		return domain.ProductIssueDashboard{}, err
	}
	mappings, err := s.repo.ListMappings(ctx)
	if err != nil {
		return domain.ProductIssueDashboard{}, err
	}
	return BuildProductIssueDashboard(ProductIssueDashboardInput{
		Rows: rows, Mappings: mappings, Now: now, Loc: s.loc, Start: start, End: end, Bucket: bucket, Products: products,
	}), nil
}

func (s *productIssueDashboardService) Dashboard(ctx context.Context, req ProductIssueDashboardRequest) (domain.ProductIssueDashboard, error) {
	bucket := req.Bucket
	if bucket == "" {
		bucket = DashboardBucketWeek
	}
	if bucket != DashboardBucketWeek && bucket != DashboardBucketMonth {
		return domain.ProductIssueDashboard{}, &apierror.ValidationError{Msg: `bucket must be "week" or "month"`}
	}
	now := s.now()
	start, end, err := resolveDashboardPeriod(req.From, req.To, now, s.loc)
	if err != nil {
		return domain.ProductIssueDashboard{}, err
	}
	return s.build(ctx, start, end, bucket, req.Products, now)
}

func (s *productIssueDashboardService) Report(ctx context.Context, req ProductIssueReportRequest) (domain.ProductIssueDashboard, error) {
	now := s.now()
	start, end, err := resolveReportPeriod(req.Period, req.Date, now, s.loc)
	if err != nil {
		return domain.ProductIssueDashboard{}, err
	}
	// A monthly report's trend is weekly, clipped to the month.
	return s.build(ctx, start, end, DashboardBucketWeek, req.Products, now)
}

func (s *productIssueDashboardService) Recipients(ctx context.Context) (domain.ProductIssueRecipients, error) {
	leads, err := s.repo.ListLeads(ctx)
	if err != nil {
		return domain.ProductIssueRecipients{}, err
	}
	return BuildProductIssueRecipients(leads), nil
}
