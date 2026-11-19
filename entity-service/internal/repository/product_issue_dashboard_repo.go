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
	"encoding/json"
	"fmt"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// ProductIssueDashboardRepository reads what the product issue dashboard and
// reports compute from. Every method runs under the system identity: the routes
// are internal-only.
type ProductIssueDashboardRepository interface {
	// ListRows returns one row per PRODUCT_ISSUE work item with a github_issue_link
	// that is not DELETED, each with its newest resolution clock.
	ListRows(ctx context.Context) ([]domain.ProductIssueDashboardRow, error)
	// ListMappings returns every product_repo_mapping row, active or not.
	ListMappings(ctx context.Context) ([]domain.ProductIssueDashboardMapping, error)
	// ListLeads returns every (product, lead email) of the active mapping rows.
	ListLeads(ctx context.Context) ([]domain.ProductIssueLead, error)
}

type productIssueDashboardRepo struct {
	db *Scoped
}

// NewProductIssueDashboardRepository returns the Postgres implementation.
func NewProductIssueDashboardRepository(db *Scoped) ProductIssueDashboardRepository {
	return &productIssueDashboardRepo{db: db}
}

// productIssueDashboardRowsSQL: the issue is aliased pwi because caseLikeJoins and
// caseLikeStateColumn read the case's work item as wi. The case is open when it is a
// case-like type and its state is not CLOSED (a case-like row with no state is open,
// the convention of autoclosure_repo.go).
const productIssueDashboardRowsSQL = `
	SELECT pwi.id::TEXT, pwi.number, pwi.subject,
	       g.owner, g.repository, g.issue_number, g.html_url, g.github_state, g.board_status,
	       g.board_project_number, g.labels,
	       COALESCE(g.github_created_on, pwi.created_on), g.github_closed_on, g.last_synced_on,
	       pi.priority, pi.opened_by_email,
	       COALESCE((SELECT e.waits_on FROM work_item_status_event e
	                  WHERE e.work_item_id = pwi.id AND e.raw_status IS NOT NULL AND e.to_status = g.board_status
	                  ORDER BY e.occurred_at DESC, e.created_on DESC LIMIT 1), ''),
	       (SELECT MAX(e.occurred_at) FROM work_item_status_event e
	         WHERE e.work_item_id = pwi.id AND e.raw_status IS NOT NULL),
	       (SELECT MAX(e.occurred_at) FROM work_item_status_event e
	         WHERE e.work_item_id = pwi.id AND e.raw_status IS NULL AND e.to_status = 'REOPENED'),
	       (SELECT MAX(e.occurred_at) FROM work_item_status_event e
	         WHERE e.work_item_id = pwi.id AND e.to_status = 'WAITING_ON_CS'),
	       (SELECT MAX(e.occurred_at) FROM work_item_status_event e
	         WHERE e.work_item_id = pwi.id AND e.to_status = 'RESOLVED'),
	       (SELECT MAX(e.occurred_at) FROM work_item_status_event e
	         WHERE e.work_item_id = pwi.id AND e.raw_status IS NOT NULL AND e.to_status = 'CLOSED'),
	       wi.id::TEXT, wi.number, wi.subject,
	       COALESCE(wi.type = ANY(` + caseLikeWorkItemTypes + `) AND ` + caseLikeStateColumn + ` IS DISTINCT FROM 'CLOSED', FALSE)
	  FROM work_item pwi
	  JOIN product_issue pi ON pi.id = pwi.id
	  JOIN github_issue_link g ON g.work_item_id = pwi.id
	  LEFT JOIN work_item_link l ON l.from_id = pwi.id AND l.kind = 'AFFECTS' AND l.is_active
	  LEFT JOIN work_item wi ON wi.id = l.to_id
	  LEFT JOIN "case" c ON c.id = wi.id
	  ` + caseLikeJoins + `
	 WHERE pwi.type = 'PRODUCT_ISSUE' AND g.link_state <> 'DELETED'
	 ORDER BY pwi.number, pwi.id`

func (r *productIssueDashboardRepo) ListRows(ctx context.Context) ([]domain.ProductIssueDashboardRow, error) {
	ctx = WithSystemIdentity(ctx)
	rows, err := r.db.Query(ctx, productIssueDashboardRowsSQL)
	if err != nil {
		return nil, fmt.Errorf("list product issue dashboard rows: %w", err)
	}
	defer rows.Close()
	out := []domain.ProductIssueDashboardRow{}
	for rows.Next() {
		var d domain.ProductIssueDashboardRow
		var labels []byte
		var caseID, caseNumber, caseSubject *string
		var caseOpen bool
		if err := rows.Scan(&d.ID, &d.Number, &d.Title,
			&d.Owner, &d.Repository, &d.IssueNumber, &d.HTMLURL, &d.GitHubState, &d.BoardStatus,
			&d.BoardProjectNumber, &labels,
			&d.OpenedOn, &d.GitHubClosedOn, &d.LastSyncedOn,
			&d.Priority, &d.OpenedByEmail, &d.BoardWaitsOn,
			&d.LatestBoardEventOn, &d.LatestReopenEventOn, &d.LatestWOCEventOn, &d.LatestResolvedEventOn, &d.LatestClosedEventOn,
			&caseID, &caseNumber, &caseSubject, &caseOpen); err != nil {
			return nil, fmt.Errorf("scan product issue dashboard row: %w", err)
		}
		d.Labels = []string{}
		if len(labels) > 0 {
			if err := json.Unmarshal(labels, &d.Labels); err != nil {
				return nil, fmt.Errorf("decode labels: %w", err)
			}
		}
		if caseID != nil {
			d.Case = &domain.ProductIssueDashboardCase{
				ID: *caseID, Number: stringOrEmpty(caseNumber), Subject: stringOrEmpty(caseSubject), Open: caseOpen,
			}
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()

	ids := make([]string, len(out))
	for i := range out {
		ids[i] = out[i].ID
	}
	clocks, err := loadProductIssueClocks(ctx, r.db, ids)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Clock = clocks[out[i].ID]
	}
	return out, nil
}

func (r *productIssueDashboardRepo) ListMappings(ctx context.Context) ([]domain.ProductIssueDashboardMapping, error) {
	ctx = WithSystemIdentity(ctx)
	rows, err := r.db.Query(ctx, `
		SELECT product_name, owner, repository, github_label, is_active, board_project_number
		  FROM product_repo_mapping
		 ORDER BY product_name`)
	if err != nil {
		return nil, fmt.Errorf("list product repo mappings: %w", err)
	}
	defer rows.Close()
	out := []domain.ProductIssueDashboardMapping{}
	for rows.Next() {
		var m domain.ProductIssueDashboardMapping
		if err := rows.Scan(&m.ProductName, &m.Owner, &m.Repository, &m.GithubLabel, &m.IsActive, &m.BoardProjectNumber); err != nil {
			return nil, fmt.Errorf("scan product repo mapping: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *productIssueDashboardRepo) ListLeads(ctx context.Context) ([]domain.ProductIssueLead, error) {
	ctx = WithSystemIdentity(ctx)
	rows, err := r.db.Query(ctx, `
		SELECT product_name, lead_emails
		  FROM product_repo_mapping
		 WHERE is_active
		 ORDER BY product_name`)
	if err != nil {
		return nil, fmt.Errorf("list product leads: %w", err)
	}
	defer rows.Close()
	out := []domain.ProductIssueLead{}
	for rows.Next() {
		var name string
		var emails []string
		if err := rows.Scan(&name, &emails); err != nil {
			return nil, fmt.Errorf("scan product leads: %w", err)
		}
		for _, e := range emails {
			out = append(out, domain.ProductIssueLead{ProductName: name, Email: e})
		}
	}
	return out, rows.Err()
}
