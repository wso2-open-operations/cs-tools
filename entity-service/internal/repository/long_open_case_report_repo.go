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

// LongOpenCaseReportRepository reads the rows behind the long-open cases
// report.
type LongOpenCaseReportRepository interface {
	// ListLongOpenCases returns the cases that are still being worked on
	// (open, work in progress, waiting on WSO2 or reopened), were created on
	// or before createdBefore, belong to a project of one of projectTypes,
	// carry none of excludedTags (compared ignoring case) and have never had
	// an ETA shared. They come back in two tables split by the project's
	// onboarding status, each oldest first and cut at limit rows, with the
	// full size of each table.
	ListLongOpenCases(ctx context.Context, createdBefore time.Time, projectTypes, excludedTags []string, limit int) (domain.LongOpenCaseReportData, error)

	// ListOpenCasesBefore returns the cases in exactly the open state that
	// were created strictly before createdBefore and whose project is of one
	// of projectTypes, or of one of productProjectTypes with the case's
	// product set. Tags and ETAs are not considered. All rows come back in
	// Cases, oldest first and cut at limit, with CasesTotal the full size;
	// the onboarding table is empty.
	ListOpenCasesBefore(ctx context.Context, createdBefore time.Time, projectTypes, productProjectTypes []string, limit int) (domain.LongOpenCaseReportData, error)
}

type longOpenCaseReportRepository struct {
	db *Scoped
}

// NewLongOpenCaseReportRepository constructs the Postgres-backed
// LongOpenCaseReportRepository. It takes *Scoped, not the bare pool, so the
// read runs under the system identity the service sets.
func NewLongOpenCaseReportRepository(db *Scoped) LongOpenCaseReportRepository {
	return &longOpenCaseReportRepository{db: db}
}

// listLongOpenCasesSQL is the report. Notes on the joins:
//
//   - Arguments: $1 cut-off, $2 project type names, $3 excluded tag names,
//     $4 the onboarding_status_enum labels that put a case in the onboarding
//     table, $5 the per-table cap.
//   - A case with no project is excluded: the project type is required.
//   - Project types are matched exactly on project_type.name, as the case
//     list's projectType filter does. Tags are matched ignoring case: they
//     are typed by people.
//   - A project with no onboarding status is not onboarding, so its cases
//     land in the first table.
//   - The window functions number each table and count it in full before
//     the cap is applied, so the totals stay true when a table is cut.
var listLongOpenCasesSQL = `
WITH matched AS (
  SELECT wi.number,
         COALESCE(wi.wso2_id, '') AS internal_id,
         wi.created_on,
         wi.updated_on,
         c.state::text AS state,
         c.severity::text AS severity,
         ` + userDisplayName("ae") + ` AS assigned_engineer,
         COALESCE(cre.name, '') AS cs_team,
         COALESCE(p.onboarding_status::text = ANY($4::text[]), FALSE) AS onboarding
    FROM work_item wi
    JOIN "case" c ON c.id = wi.id
    JOIN project p ON p.id = wi.project_id
    LEFT JOIN account a ON a.id = wi.account_id
    LEFT JOIN "group" cre ON cre.id = a.cre_team_id
    LEFT JOIN "user" ae ON ae.id = wi.assigned_to_id
   WHERE wi.type = 'CASE'
     AND wi.created_on <= $1
     AND c.state IN ('OPEN', 'WORK_IN_PROGRESS', 'WAITING_ON_WSO2', 'REOPENED')
     AND wi.eta_shared_on IS NULL
     AND EXISTS (SELECT 1 FROM project_type pt
                  WHERE pt.id = p.project_type_id AND pt.name = ANY($2::text[]))
     AND NOT EXISTS (SELECT 1 FROM work_item_tag wt JOIN tag t ON t.id = wt.tag_id
                      WHERE wt.work_item_id = wi.id
                        AND LOWER(t.name) IN (SELECT LOWER(x) FROM unnest($3::text[]) AS x))
), numbered AS (
  SELECT matched.*,
         ROW_NUMBER() OVER (PARTITION BY onboarding ORDER BY created_on, number) AS rn,
         COUNT(*) OVER (PARTITION BY onboarding) AS table_total
    FROM matched
)
SELECT number, internal_id, created_on, updated_on, state, severity,
       assigned_engineer, cs_team, onboarding, table_total
  FROM numbered
 WHERE rn <= $5
 ORDER BY created_on, number`

// longOpenCaseOnboardingStatuses are the project onboarding statuses that put
// a case in the onboarding table, in the case list's own filter vocabulary.
var longOpenCaseOnboardingStatuses = []string{"In-Progress", "Not-Started"}

func (r *longOpenCaseReportRepository) ListLongOpenCases(ctx context.Context, createdBefore time.Time, projectTypes, excludedTags []string, limit int) (domain.LongOpenCaseReportData, error) {
	onboarding, err := onboardingStatusEnumLabels("onboarding status", longOpenCaseOnboardingStatuses)
	if err != nil {
		return domain.LongOpenCaseReportData{}, err
	}
	if projectTypes == nil {
		projectTypes = []string{}
	}
	if excludedTags == nil {
		excludedTags = []string{}
	}
	rows, err := r.db.Query(ctx, listLongOpenCasesSQL, createdBefore, projectTypes, excludedTags, onboarding, limit)
	if err != nil {
		return domain.LongOpenCaseReportData{}, fmt.Errorf("list long-open cases: %w", err)
	}
	defer rows.Close()

	out := domain.LongOpenCaseReportData{
		Cases:           []domain.LongOpenCaseReportRow{},
		OnboardingCases: []domain.LongOpenCaseReportRow{},
	}
	for rows.Next() {
		var (
			row        domain.LongOpenCaseReportRow
			state      string
			severity   *string
			onboarding bool
			total      int
		)
		if err := rows.Scan(&row.Number, &row.InternalID, &row.CreatedOn, &row.UpdatedOn, &state, &severity,
			&row.AssignedEngineer, &row.CsTeam, &onboarding, &total); err != nil {
			return domain.LongOpenCaseReportData{}, fmt.Errorf("scan long-open case: %w", err)
		}
		row.State = strings.ToLower(state)
		if severity != nil {
			row.Severity = string(caseSeverityFromEnum[*severity])
		}
		if onboarding {
			out.OnboardingCases = append(out.OnboardingCases, row)
			out.OnboardingCasesTotal = total
		} else {
			out.Cases = append(out.Cases, row)
			out.CasesTotal = total
		}
	}
	if err := rows.Err(); err != nil {
		return domain.LongOpenCaseReportData{}, fmt.Errorf("iterate long-open cases: %w", err)
	}
	return out, nil
}

// listOpenCasesBeforeSQL is the open-before-yesterday report. Arguments: $1
// cut-off (exclusive), $2 project type names, $3 project type names that
// also need the case's product set, $4 the cap. A case with no project is
// excluded. Project types are matched exactly, as in the long-open report.
// The product is the case's own product reference (work_item.product_id).
var listOpenCasesBeforeSQL = `
WITH matched AS (
  SELECT wi.number,
         COALESCE(wi.wso2_id, '') AS internal_id,
         wi.created_on,
         wi.updated_on,
         c.state::text AS state,
         c.severity::text AS severity,
         ` + userDisplayName("ae") + ` AS assigned_engineer,
         COALESCE(cre.name, '') AS cs_team,
         COALESCE(a.name, '') AS account_name
    FROM work_item wi
    JOIN "case" c ON c.id = wi.id
    JOIN project p ON p.id = wi.project_id
    LEFT JOIN account a ON a.id = wi.account_id
    LEFT JOIN "group" cre ON cre.id = a.cre_team_id
    LEFT JOIN "user" ae ON ae.id = wi.assigned_to_id
   WHERE wi.type = 'CASE'
     AND wi.created_on < $1
     AND c.state = 'OPEN'
     AND EXISTS (SELECT 1 FROM project_type pt
                  WHERE pt.id = p.project_type_id
                    AND (pt.name = ANY($2::text[])
                         OR (pt.name = ANY($3::text[]) AND wi.product_id IS NOT NULL)))
), numbered AS (
  SELECT matched.*,
         ROW_NUMBER() OVER (ORDER BY created_on, number) AS rn,
         COUNT(*) OVER () AS table_total
    FROM matched
)
SELECT number, internal_id, created_on, updated_on, state, severity,
       assigned_engineer, cs_team, account_name, table_total
  FROM numbered
 WHERE rn <= $4
 ORDER BY created_on, number`

func (r *longOpenCaseReportRepository) ListOpenCasesBefore(ctx context.Context, createdBefore time.Time, projectTypes, productProjectTypes []string, limit int) (domain.LongOpenCaseReportData, error) {
	if projectTypes == nil {
		projectTypes = []string{}
	}
	if productProjectTypes == nil {
		productProjectTypes = []string{}
	}
	rows, err := r.db.Query(ctx, listOpenCasesBeforeSQL, createdBefore, projectTypes, productProjectTypes, limit)
	if err != nil {
		return domain.LongOpenCaseReportData{}, fmt.Errorf("list open cases before cut-off: %w", err)
	}
	defer rows.Close()

	out := domain.LongOpenCaseReportData{
		Cases:           []domain.LongOpenCaseReportRow{},
		OnboardingCases: []domain.LongOpenCaseReportRow{},
	}
	for rows.Next() {
		var (
			row      domain.LongOpenCaseReportRow
			state    string
			severity *string
			total    int
		)
		if err := rows.Scan(&row.Number, &row.InternalID, &row.CreatedOn, &row.UpdatedOn, &state, &severity,
			&row.AssignedEngineer, &row.CsTeam, &row.AccountName, &total); err != nil {
			return domain.LongOpenCaseReportData{}, fmt.Errorf("scan open case: %w", err)
		}
		row.State = strings.ToLower(state)
		if severity != nil {
			row.Severity = string(caseSeverityFromEnum[*severity])
		}
		out.Cases = append(out.Cases, row)
		out.CasesTotal = total
	}
	if err := rows.Err(); err != nil {
		return domain.LongOpenCaseReportData{}, fmt.Errorf("iterate open cases: %w", err)
	}
	return out, nil
}
