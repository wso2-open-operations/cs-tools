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

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// PortalBannerFilter selects which rows List returns. An empty Status returns
// every status. Customer and CSM restrict to that portal's visibility flag.
type PortalBannerFilter struct {
	Status   string
	Customer bool
	CSM      bool
}

// PortalBannerRepository reads and writes portal_banner (migration 0245).
type PortalBannerRepository interface {
	List(ctx context.Context, filter PortalBannerFilter) ([]domain.PortalBanner, error)
	// Create inserts the banner and, when its order is already taken, shifts
	// that row and every row below it down in the same transaction.
	Create(ctx context.Context, actor string, req domain.SavePortalBannerRequest) (domain.PortalBanner, error)
	// Update replaces the banner the same way. Nil when there is no such id.
	Update(ctx context.Context, actor, id string, req domain.SavePortalBannerRequest) (*domain.PortalBanner, error)
}

type portalBannerRepo struct {
	db *pgxpool.Pool
}

// NewPortalBannerRepository constructs a Postgres-backed PortalBannerRepository.
func NewPortalBannerRepository(db *pgxpool.Pool) PortalBannerRepository {
	return &portalBannerRepo{db: db}
}

const portalBannerSelect = `SELECT id::text, status, sort_order, visible_in_customer_portal, visible_in_csm_portal,
	html, starts_on, expires_on, created_on, created_by, updated_on, updated_by
	FROM portal_banner`

func scanPortalBanner(row pgx.Row) (domain.PortalBanner, error) {
	var b domain.PortalBanner
	err := row.Scan(&b.ID, &b.Status, &b.SortOrder, &b.VisibleInCustomerPortal, &b.VisibleInCsmPortal,
		&b.HTML, &b.StartsAt, &b.ExpiresAt, &b.CreatedOn, &b.CreatedBy, &b.UpdatedOn, &b.UpdatedBy)
	return b, err
}

func (r *portalBannerRepo) List(ctx context.Context, filter PortalBannerFilter) ([]domain.PortalBanner, error) {
	rows, err := r.db.Query(ctx, portalBannerSelect+`
		WHERE ($1 = '' OR status = $1)
		  AND (NOT $2 OR visible_in_customer_portal)
		  AND (NOT $3 OR visible_in_csm_portal)
		ORDER BY sort_order, id`, filter.Status, filter.Customer, filter.CSM)
	if err != nil {
		return nil, fmt.Errorf("list portal banners: %w", err)
	}
	defer rows.Close()
	out := []domain.PortalBanner{}
	for rows.Next() {
		b, err := scanPortalBanner(rows)
		if err != nil {
			return nil, fmt.Errorf("list portal banners: scan: %w", err)
		}
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list portal banners: %w", err)
	}
	return out, nil
}

func (r *portalBannerRepo) Create(ctx context.Context, actor string, req domain.SavePortalBannerRequest) (domain.PortalBanner, error) {
	var out domain.PortalBanner
	err := r.inTx(ctx, "create portal banner", func(tx pgx.Tx) error {
		var id string
		if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text`).Scan(&id); err != nil {
			return err
		}
		order, err := placePortalBanner(ctx, tx, id, req.SortOrder, req.Status)
		if err != nil {
			return err
		}
		row := tx.QueryRow(ctx, `INSERT INTO portal_banner (
			id, status, sort_order, visible_in_customer_portal, visible_in_csm_portal,
			html, starts_on, expires_on, created_by, updated_by)
			VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, $9, $9)
			RETURNING id::text, status, sort_order, visible_in_customer_portal, visible_in_csm_portal,
				html, starts_on, expires_on, created_on, created_by, updated_on, updated_by`,
			id, req.Status, order, req.VisibleInCustomerPortal, req.VisibleInCsmPortal,
			domain.StampBannerID(req.HTML, id), req.StartsAt, req.ExpiresAt, actor)
		b, err := scanPortalBanner(row)
		out = b
		return err
	})
	return out, err
}

func (r *portalBannerRepo) Update(ctx context.Context, actor, id string, req domain.SavePortalBannerRequest) (*domain.PortalBanner, error) {
	var out *domain.PortalBanner
	err := r.inTx(ctx, "update portal banner", func(tx pgx.Tx) error {
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM portal_banner WHERE id = $1::uuid)`, id).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return nil
		}
		order, err := placePortalBanner(ctx, tx, id, req.SortOrder, req.Status)
		if err != nil {
			return err
		}
		row := tx.QueryRow(ctx, `UPDATE portal_banner SET
			status = $2, sort_order = $3, visible_in_customer_portal = $4, visible_in_csm_portal = $5,
			html = $6, starts_on = $7, expires_on = $8, updated_on = NOW(), updated_by = $9
			WHERE id = $1::uuid
			RETURNING id::text, status, sort_order, visible_in_customer_portal, visible_in_csm_portal,
				html, starts_on, expires_on, created_on, created_by, updated_on, updated_by`,
			id, req.Status, order, req.VisibleInCustomerPortal, req.VisibleInCsmPortal,
			domain.StampBannerID(req.HTML, id), req.StartsAt, req.ExpiresAt, actor)
		b, err := scanPortalBanner(row)
		if err != nil {
			return err
		}
		out = &b
		return nil
	})
	return out, err
}

// placePortalBanner locks the table, puts id at order, and renumbers every
// row 0, 1, 2, … with no gaps. An unpublished status sits after the stack.
// FOR UPDATE locks no rows when the table is empty, so two creates could both
// take sort order 0. The table lock makes those writes wait for each other.
// The returned order is the place written for id.
func placePortalBanner(ctx context.Context, tx pgx.Tx, id string, order int, status string) (int, error) {
	if _, err := tx.Exec(ctx, `LOCK TABLE portal_banner IN SHARE ROW EXCLUSIVE MODE`); err != nil {
		return 0, err
	}
	rows, err := tx.Query(ctx, `SELECT id::text, sort_order, status FROM portal_banner ORDER BY sort_order, id FOR UPDATE`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var current []bannerOrder
	for rows.Next() {
		var row bannerOrder
		var rowStatus string
		if err := rows.Scan(&row.ID, &row.SortOrder, &rowStatus); err != nil {
			return 0, err
		}
		row.Unpublished = rowStatus == domain.PortalBannerUnpublished
		current = append(current, row)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	assigned := order
	for _, row := range assignBannerOrder(current, id, order, status == domain.PortalBannerUnpublished) {
		if row.ID == id {
			assigned = row.SortOrder
			continue
		}
		if _, err := tx.Exec(ctx, `UPDATE portal_banner SET sort_order = $2 WHERE id = $1::uuid AND sort_order <> $2`, row.ID, row.SortOrder); err != nil {
			return 0, err
		}
	}
	return assigned, nil
}

func (r *portalBannerRepo) inTx(ctx context.Context, op string, fn func(tx pgx.Tx) error) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("%s: begin tx: %w", op, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("%s: commit: %w", op, err)
	}
	return nil
}
