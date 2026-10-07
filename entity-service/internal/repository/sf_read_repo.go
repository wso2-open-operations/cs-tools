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
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/db"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"golang.org/x/sync/errgroup"
)

// SalesforceReadRepository reads the Salesforce-ingested sf_opportunity,
// sf_invoice and sf_opportunity_link tables in the ServiceNow-era read shapes.
type SalesforceReadRepository interface {
	SearchOpportunities(ctx context.Context, accountID string, p domain.Pagination) ([]domain.Opportunity, int, error)
	GetOpportunity(ctx context.Context, id string) (domain.Opportunity, error)
	SearchInvoices(ctx context.Context, opportunityID string, p domain.Pagination) ([]domain.Invoice, int, error)
	GetInvoice(ctx context.Context, id string) (domain.Invoice, error)
	SearchProjectOpportunityLinks(ctx context.Context, projectID, opportunityID string, p domain.Pagination) ([]domain.ProjectOpportunityLink, int, error)
}

type sfReadRepo struct {
	db db.Pool
}

// NewSalesforceReadRepository constructs a SalesforceReadRepository backed by the given pool.
func NewSalesforceReadRepository(db db.Pool) SalesforceReadRepository {
	return &sfReadRepo{db: db}
}

const (
	sfOpportunitySelect = `SELECT o.id::text, o.name, a.id::text, a.name, o.eula_version, o.eula_version_decimal::text, o.stage
	FROM sf_opportunity o LEFT JOIN account a ON a.id = o.account_id`
	sfInvoiceSelect = `SELECT i.id::text, i.name, i.invoiced_amount::text,
	to_char(i.invoice_date, 'YYYY-MM-DD'), to_char(i.invoiced_paid_date, 'YYYY-MM-DD'),
	to_char(i.invoiced_due_date, 'YYYY-MM-DD'), to_char(i.original_invoice_due_date, 'YYYY-MM-DD'),
	o.id::text, o.name, i.classification, i.sf_id
	FROM sf_invoice i LEFT JOIN sf_opportunity o ON o.id = i.opportunity_id`
	sfLinkSelect = `SELECT l.id::text, p.id::text, p.name, o.id::text, o.name
	FROM sf_opportunity_link l
	LEFT JOIN project p ON p.id = l.project_id
	LEFT JOIN sf_opportunity o ON o.id = l.opportunity_id`
)

// sfFilter is one optional equality filter; an empty value is skipped.
type sfFilter struct {
	column string
	value  string
}

// buildSFWhere turns the non-empty filters into a WHERE clause with $n placeholders.
func buildSFWhere(filters ...sfFilter) (string, []any) {
	where := ""
	var args []any
	for _, f := range filters {
		if f.value == "" {
			continue
		}
		args = append(args, f.value)
		clause := fmt.Sprintf("%s = $%d", f.column, len(args))
		if where == "" {
			where = " WHERE " + clause
		} else {
			where += " AND " + clause
		}
	}
	return where, args
}

// entityRef builds a reference from a LEFT JOIN, nil when the joined row is absent.
func entityRef(id, name *string) *domain.EntityRef {
	if id == nil || *id == "" {
		return nil
	}
	ref := &domain.EntityRef{ID: *id}
	if name != nil {
		ref.Name = *name
	}
	return ref
}

func scanOpportunity(row pgx.Row) (domain.Opportunity, error) {
	var o domain.Opportunity
	var accountID, accountName *string
	err := row.Scan(&o.ID, &o.Name, &accountID, &accountName, &o.EulaVersion, &o.EulaVersionDecimal, &o.Stage)
	o.Account = entityRef(accountID, accountName)
	return o, err
}

func scanInvoice(row pgx.Row) (domain.Invoice, error) {
	var i domain.Invoice
	var oppID, oppName *string
	err := row.Scan(&i.ID, &i.Name, &i.InvoicedAmount, &i.InvoiceDate, &i.InvoicedPaidDate,
		&i.InvoicedDueDate, &i.InvoiceOriginalDueDate, &oppID, &oppName, &i.Classification, &i.SfID)
	i.Opportunity = entityRef(oppID, oppName)
	return i, err
}

func scanLink(row pgx.Row) (domain.ProjectOpportunityLink, error) {
	var l domain.ProjectOpportunityLink
	var projectID, projectName, oppID, oppName *string
	err := row.Scan(&l.ID, &projectID, &projectName, &oppID, &oppName)
	l.Project = entityRef(projectID, projectName)
	l.Opportunity = entityRef(oppID, oppName)
	return l, err
}

// searchSF runs the count and the page query concurrently, like the other search repos.
func searchSF[T any](ctx context.Context, db db.Pool, selectSQL, fromSQL, alias string, filters []sfFilter, p domain.Pagination, scan func(pgx.Row) (T, error)) ([]T, int, error) {
	where, args := buildSFWhere(filters...)
	countQuery := "SELECT COUNT(*) " + fromSQL + where
	dataQuery := fmt.Sprintf("%s%s ORDER BY %s.created_on DESC, %s.id LIMIT $%d OFFSET $%d",
		selectSQL, where, alias, alias, len(args)+1, len(args)+2)
	dataArgs := append(append([]any{}, args...), p.Limit, p.Offset)

	var total int
	out := make([]T, 0, p.Limit)
	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		return db.QueryRow(egCtx, countQuery, args...).Scan(&total)
	})
	eg.Go(func() error {
		rows, err := db.Query(egCtx, dataQuery, dataArgs...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			v, err := scan(rows)
			if err != nil {
				return err
			}
			out = append(out, v)
		}
		return rows.Err()
	})
	if err := eg.Wait(); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func getSF[T any](ctx context.Context, db db.Pool, selectSQL, idColumn, id, what string, scan func(pgx.Row) (T, error)) (T, error) {
	v, err := scan(db.QueryRow(ctx, selectSQL+" WHERE "+idColumn+" = $1", id))
	if errors.Is(err, pgx.ErrNoRows) {
		var zero T
		return zero, &apierror.NotFoundError{Msg: what + " not found"}
	}
	if err != nil {
		var zero T
		return zero, fmt.Errorf("get %s: %w", what, err)
	}
	return v, nil
}

// SearchOpportunities implements SalesforceReadRepository.
func (r *sfReadRepo) SearchOpportunities(ctx context.Context, accountID string, p domain.Pagination) ([]domain.Opportunity, int, error) {
	out, total, err := searchSF(ctx, r.db, sfOpportunitySelect, "FROM sf_opportunity o", "o",
		[]sfFilter{{"o.account_id", accountID}}, p, scanOpportunity)
	if err != nil {
		return nil, 0, fmt.Errorf("search opportunities: %w", err)
	}
	return out, total, nil
}

// GetOpportunity implements SalesforceReadRepository.
func (r *sfReadRepo) GetOpportunity(ctx context.Context, id string) (domain.Opportunity, error) {
	return getSF(ctx, r.db, sfOpportunitySelect, "o.id", id, "opportunity", scanOpportunity)
}

// SearchInvoices implements SalesforceReadRepository.
func (r *sfReadRepo) SearchInvoices(ctx context.Context, opportunityID string, p domain.Pagination) ([]domain.Invoice, int, error) {
	out, total, err := searchSF(ctx, r.db, sfInvoiceSelect, "FROM sf_invoice i", "i",
		[]sfFilter{{"i.opportunity_id", opportunityID}}, p, scanInvoice)
	if err != nil {
		return nil, 0, fmt.Errorf("search invoices: %w", err)
	}
	return out, total, nil
}

// GetInvoice implements SalesforceReadRepository.
func (r *sfReadRepo) GetInvoice(ctx context.Context, id string) (domain.Invoice, error) {
	return getSF(ctx, r.db, sfInvoiceSelect, "i.id", id, "invoice", scanInvoice)
}

// SearchProjectOpportunityLinks implements SalesforceReadRepository.
func (r *sfReadRepo) SearchProjectOpportunityLinks(ctx context.Context, projectID, opportunityID string, p domain.Pagination) ([]domain.ProjectOpportunityLink, int, error) {
	out, total, err := searchSF(ctx, r.db, sfLinkSelect, "FROM sf_opportunity_link l", "l",
		[]sfFilter{{"l.project_id", projectID}, {"l.opportunity_id", opportunityID}}, p, scanLink)
	if err != nil {
		return nil, 0, fmt.Errorf("search project-opportunity links: %w", err)
	}
	return out, total, nil
}
