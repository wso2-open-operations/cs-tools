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

package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// ProductRegressionReportRepository reads the rows behind the monthly
// product regression reports.
type ProductRegressionReportRepository interface {
	// ListRegressionCases returns every case on one of the named products
	// that carries tag (compared ignoring case) and whose fix ETA was
	// shared at or after periodStart and before periodEnd, ordered by
	// product name, then by when the ETA was shared. Product names are
	// compared ignoring case and surrounding whitespace.
	ListRegressionCases(ctx context.Context, periodStart, periodEnd time.Time, tag string, products []string) ([]domain.ProductRegressionReportRow, error)
}

type productRegressionReportRepository struct {
	db *Scoped
}

// NewProductRegressionReportRepository constructs the Postgres-backed
// ProductRegressionReportRepository. It takes *Scoped, not the bare pool:
// work_item and case carry row-level security, and the report's reads run
// under the system identity the service sets.
func NewProductRegressionReportRepository(db *Scoped) ProductRegressionReportRepository {
	return &productRegressionReportRepository{db: db}
}

// listRegressionCasesSQL is the report. Notes on the joins:
//
//   - A case is a work item of type CASE with a case row. The product it
//     is filed against is work_item.product_id; a case with no product
//     cannot belong to any product's report and is left out by the inner
//     join. Names are matched after lower-casing and trimming both sides,
//     so a product that exists twice under one name (it does) is matched
//     by both rows, and the name is trimmed on the way out so they group
//     together.
//   - The deployed product is the customer's install the case concerns,
//     the "Product" column the report has always shown beside the case.
//   - The month is taken on eta_shared_on, the instant the fix ETA was
//     shared with the customer: the report lists the regressions a fix
//     was promised for that month, not the ones opened or closed then.
//   - Tag matching ignores case: tags are typed by people.
const listRegressionCasesSQL = `
SELECT wi.id::text,
       wi.number,
       COALESCE(wi.wso2_id, ''),
       COALESCE(wi.subject, ''),
       TRIM(p.name),
       COALESCE(dp.name, ''),
       wi.eta_shared_on
  FROM work_item wi
  JOIN "case" c ON c.id = wi.id
  JOIN product p ON p.id = wi.product_id
  LEFT JOIN deployed_product dp ON dp.id = wi.deployed_product_id
 WHERE wi.type = 'CASE'
   AND LOWER(TRIM(p.name)) = ANY($1::text[])
   AND wi.eta_shared_on >= $2
   AND wi.eta_shared_on < $3
   AND EXISTS (SELECT 1 FROM work_item_tag wt JOIN tag t ON t.id = wt.tag_id
                WHERE wt.work_item_id = wi.id AND LOWER(t.name) = LOWER($4))
 ORDER BY TRIM(p.name), wi.eta_shared_on, wi.number`

// ListRegressionCases runs listRegressionCasesSQL with the product names
// lower-cased and trimmed; a list with no usable name returns an empty
// list without a query.
func (r *productRegressionReportRepository) ListRegressionCases(ctx context.Context, periodStart, periodEnd time.Time, tag string, products []string) ([]domain.ProductRegressionReportRow, error) {
	names := make([]string, 0, len(products))
	for _, p := range products {
		if n := strings.ToLower(strings.TrimSpace(p)); n != "" {
			names = append(names, n)
		}
	}
	if len(names) == 0 {
		return []domain.ProductRegressionReportRow{}, nil
	}
	rows, err := r.db.Query(ctx, listRegressionCasesSQL, names, periodStart, periodEnd, tag)
	if err != nil {
		return nil, fmt.Errorf("list regression cases: %w", err)
	}
	defer rows.Close()

	out := []domain.ProductRegressionReportRow{}
	for rows.Next() {
		var row domain.ProductRegressionReportRow
		if err := rows.Scan(&row.ID, &row.Number, &row.IssueKey, &row.Subject, &row.Product, &row.DeployedProduct, &row.EtaSharedOn); err != nil {
			return nil, fmt.Errorf("scan regression case: %w", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate regression cases: %w", err)
	}
	return out, nil
}
