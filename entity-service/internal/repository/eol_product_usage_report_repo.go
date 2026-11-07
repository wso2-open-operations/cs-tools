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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// EolProductUsageReportRepository reads the rows behind the monthly EOL
// product usage report.
type EolProductUsageReportRepository interface {
	// ListEolUsage returns one row per (project, product version) where an
	// active deployed product of an active Primary Production deployment of a
	// current customer runs a version whose support EOL date is before asOf
	// ("YYYY-MM-DD"),
	// ordered by region ("" last), account name, project name, EOL date,
	// product name and version.
	ListEolUsage(ctx context.Context, asOf string) ([]domain.EolProductUsageRow, error)
}

type eolProductUsageReportRepository struct {
	db *Scoped
}

// NewEolProductUsageReportRepository constructs the Postgres-backed
// EolProductUsageReportRepository. It takes *Scoped, like the other
// reports, so its reads run under the system identity the service sets.
func NewEolProductUsageReportRepository(db *Scoped) EolProductUsageReportRepository {
	return &eolProductUsageReportRepository{db: db}
}

// listEolUsageSQL is the report. Notes:
//
//   - A deployed product, deployment or project with no active flag (NULL)
//     counts as active, as the deployment pages treat it.
//   - Customers who have left are not listed: an account deleted in
//     Salesforce (deleted_on), an inactive project, or a project WSO2 has
//     closed (wso2_closure_state CLOSED). Read-only, restricted and suspended
//     projects are still customers and are listed.
//   - deployed_product.life_cycle_stage_status is deliberately not used:
//     every active product listed in a Primary Production deployment is
//     reported, whatever its ServiceNow lifecycle status says.
//   - DISTINCT ON (project, product version) lists a version once per
//     project however many Primary Production deployments run it.
//   - The CS ABT is the account's CRE team; the account manager and the
//     technical owner are the account's own, by user id.
const listEolUsageSQL = `
SELECT region, account_id, account_name, cs_abt, am_name, am_email, to_name, to_email,
       project_id, project_name, project_key, product_version_id, product_name, version,
       support_status, eol_date
FROM (
    SELECT DISTINCT ON (p.id, pv.id)
           COALESCE(TRIM(a.region), '')                      AS region,
           a.id::text                                       AS account_id,
           TRIM(a.name)                                     AS account_name,
           COALESCE(TRIM(g.name), '')                       AS cs_abt,
           COALESCE(NULLIF(TRIM(am.name), ''), TRIM(CONCAT_WS(' ', am.first_name, am.last_name)), '') AS am_name,
           COALESCE(am.email, '')                           AS am_email,
           COALESCE(NULLIF(TRIM(tow.name), ''), TRIM(CONCAT_WS(' ', tow.first_name, tow.last_name)), '') AS to_name,
           COALESCE(tow.email, '')                          AS to_email,
           p.id::text                                       AS project_id,
           TRIM(p.name)                                     AS project_name,
           COALESCE(TRIM(p.key), '')                        AS project_key,
           pv.id::text                                      AS product_version_id,
           TRIM(prod.name)                                  AS product_name,
           pv.version                                       AS version,
           pv.current_support_status::text                  AS support_status,
           to_char(pv.support_eol_date, 'YYYY-MM-DD')       AS eol_date
    FROM deployed_product dp
    JOIN deployment d       ON d.id = dp.deployment_id
    JOIN project p          ON p.id = d.project_id
    JOIN account a          ON a.id = p.account_id
    JOIN product_version pv ON pv.id = dp.version_id
    JOIN product prod       ON prod.id = pv.product_id
    LEFT JOIN "group" g     ON g.id = a.cre_team_id
    LEFT JOIN "user" am     ON am.id = a.account_manager_id
    LEFT JOIN "user" tow    ON tow.id = a.technical_owner_id
    WHERE d.type = 'PRIMARY_PRODUCTION'
      AND COALESCE(d.is_active, TRUE)
      AND COALESCE(dp.active, TRUE)
      AND COALESCE(p.is_active, TRUE)
      AND p.wso2_closure_state IS DISTINCT FROM 'CLOSED'
      AND a.deleted_on IS NULL
      AND pv.support_eol_date < $1::date
    ORDER BY p.id, pv.id
) r
ORDER BY (region = ''), lower(region), lower(account_name), account_id, lower(project_name), project_id,
         eol_date, lower(product_name), version`

// ListEolUsage implements EolProductUsageReportRepository.
func (r *eolProductUsageReportRepository) ListEolUsage(ctx context.Context, asOf string) ([]domain.EolProductUsageRow, error) {
	rows, err := r.db.Query(ctx, listEolUsageSQL, asOf)
	if err != nil {
		return nil, fmt.Errorf("list eol product usage: %w", err)
	}
	defer rows.Close()
	out := []domain.EolProductUsageRow{}
	for rows.Next() {
		var x domain.EolProductUsageRow
		if err := rows.Scan(&x.Region, &x.AccountID, &x.AccountName, &x.CsAbt, &x.AccountManagerName, &x.AccountManagerEmail,
			&x.TechnicalOwnerName, &x.TechnicalOwnerEmail, &x.ProjectID, &x.ProjectName, &x.ProjectKey,
			&x.ProductVersionID, &x.ProductName, &x.Version, &x.SupportStatus, &x.EolDate); err != nil {
			return nil, fmt.Errorf("list eol product usage: scan: %w", err)
		}
		out = append(out, x)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list eol product usage: %w", err)
	}
	return out, nil
}
