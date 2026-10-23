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

// ProductRegressionReportService builds the monthly product regression
// reports.
type ProductRegressionReportService interface {
	BuildReport(ctx context.Context, req domain.ProductRegressionReportRequest) (domain.ProductRegressionReport, error)
}

// ProductRegressionReportTimezone is the zone the month is taken in. The
// source reports ran at 09:00 India Standard Time and took "last month" in
// that zone; Colombo keeps the same offset, year round, and is the zone
// every other report here uses.
const ProductRegressionReportTimezone = "Asia/Colombo"

// ProductRegressionReportMaxProducts bounds how many product names one
// request may carry.
const ProductRegressionReportMaxProducts = 50

// ProductRegressionReportConfig names the tag the reports are built from.
type ProductRegressionReportConfig struct {
	// Tag marks a case as a regression; compared ignoring case.
	Tag string
}

type productRegressionReportService struct {
	repo repository.ProductRegressionReportRepository
	cfg  ProductRegressionReportConfig
	loc  *time.Location
}

// NewProductRegressionReportService constructs a ProductRegressionReportService.
func NewProductRegressionReportService(repo repository.ProductRegressionReportRepository, cfg ProductRegressionReportConfig) ProductRegressionReportService {
	loc, err := time.LoadLocation(ProductRegressionReportTimezone)
	if err != nil {
		// The zone is a constant; a missing tz database is the only way
		// this fails, and UTC is the honest fallback.
		loc = time.UTC
	}
	return &productRegressionReportService{repo: repo, cfg: cfg, loc: loc}
}

// BuildReport validates the request, resolves the month, lists the cases
// under the system identity and groups them by product.
func (s *productRegressionReportService) BuildReport(ctx context.Context, req domain.ProductRegressionReportRequest) (domain.ProductRegressionReport, error) {
	products := make([]string, 0, len(req.Products))
	for _, p := range req.Products {
		if p = strings.TrimSpace(p); p != "" {
			products = append(products, p)
		}
	}
	if len(products) == 0 {
		return domain.ProductRegressionReport{}, &apierror.ValidationError{Msg: "products is required"}
	}
	if len(products) > ProductRegressionReportMaxProducts {
		return domain.ProductRegressionReport{}, &apierror.ValidationError{
			Msg: fmt.Sprintf("products may name at most %d products", ProductRegressionReportMaxProducts)}
	}
	if strings.TrimSpace(s.cfg.Tag) == "" {
		return domain.ProductRegressionReport{}, &apierror.ServiceUnavailableError{
			Msg: "the product regression report's tag is not configured"}
	}
	now := req.Now
	if now.IsZero() {
		now = time.Now()
	}
	periodStart, periodEnd, err := ProductRegressionReportMonth(req.Month, now, s.loc)
	if err != nil {
		return domain.ProductRegressionReport{}, err
	}

	// The route is internal-only; the report reads under the system
	// identity so row-level security on work_item and case does not hide
	// the cases from a report that exists to list them.
	ctx = repository.WithSystemIdentity(ctx)
	rows, err := s.repo.ListRegressionCases(ctx, periodStart, periodEnd, s.cfg.Tag, products)
	if err != nil {
		return domain.ProductRegressionReport{}, err
	}

	report := GroupRegressionCases(rows)
	report.GeneratedAt = now
	report.Month = periodStart.Format("2006-01")
	report.PeriodStart = periodStart
	report.PeriodEnd = periodEnd
	report.Tag = s.cfg.Tag
	report.Products = products
	return report, nil
}

// ProductRegressionReportMonth is the half-open window [start, end) of the
// month a report covers, in loc. month is "YYYY-MM"; empty means the
// calendar month before now in loc, which is what a run on the 1st
// reports on, and what a retry later in the month still reports on. A
// month that has not ended yet is refused: the report is of a finished
// month, and asking for the current one would silently return a partial
// list.
func ProductRegressionReportMonth(month string, now time.Time, loc *time.Location) (start, end time.Time, err error) {
	local := now.In(loc)
	if month == "" {
		first := time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, loc)
		return first.AddDate(0, -1, 0), first, nil
	}
	parsed, perr := time.ParseInLocation("2006-01", month, loc)
	if perr != nil {
		return time.Time{}, time.Time{}, &apierror.ValidationError{Msg: "month must be a calendar month as YYYY-MM"}
	}
	start = time.Date(parsed.Year(), parsed.Month(), 1, 0, 0, 0, 0, loc)
	end = start.AddDate(0, 1, 0)
	if end.After(local) {
		return time.Time{}, time.Time{}, &apierror.ValidationError{Msg: "month must be a month that has already ended"}
	}
	return start, end, nil
}

// GroupRegressionCases splits the rows into one group per product, in the
// order the products first appear (the repository orders by product name,
// so that is name order), keeping each product's rows in the order given.
// Groups is non-nil so an empty report serialises as an empty list.
func GroupRegressionCases(rows []domain.ProductRegressionReportRow) domain.ProductRegressionReport {
	report := domain.ProductRegressionReport{
		Products: []string{},
		Groups:   []domain.ProductRegressionReportGroup{},
	}
	index := map[string]int{}
	for _, row := range rows {
		i, ok := index[row.Product]
		if !ok {
			i = len(report.Groups)
			index[row.Product] = i
			report.Groups = append(report.Groups, domain.ProductRegressionReportGroup{
				Product: row.Product, Cases: []domain.ProductRegressionReportRow{}})
		}
		report.Groups[i].Cases = append(report.Groups[i].Cases, row)
		report.TotalCases++
	}
	return report
}
