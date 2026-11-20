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

// FeatureOverrideRepository reads and writes features (migration 0277).
type FeatureOverrideRepository interface {
	List(ctx context.Context) (domain.FeatureOverrideList, error)
	// Put upserts one row. When markSaved is set, it also inserts the
	// saved-once mark if that row does not exist yet. Rows are never deleted.
	Put(ctx context.Context, actor string, req domain.SaveFeatureOverrideRequest) (domain.FeatureOverride, error)
}

type featureOverrideRepo struct {
	db *pgxpool.Pool
}

// NewFeatureOverrideRepository constructs a Postgres-backed FeatureOverrideRepository.
func NewFeatureOverrideRepository(db *pgxpool.Pool) FeatureOverrideRepository {
	return &featureOverrideRepo{db: db}
}

// List returns every row and whether the first save has finished. A first save
// writes its last row and the mark in one transaction, so both reads must see
// the same moment: a read-committed pair of queries could return the old rows
// next to a mark that has just been written, and the portal would stop using
// its config fallback while missing the newly saved rows. A read-only
// repeatable-read transaction fixes the snapshot at the first query.
func (r *featureOverrideRepo) List(ctx context.Context) (domain.FeatureOverrideList, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return domain.FeatureOverrideList{}, fmt.Errorf("list feature overrides: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, `SELECT nav_node_id, state, updated_on, updated_by
		FROM features ORDER BY nav_node_id`)
	if err != nil {
		return domain.FeatureOverrideList{}, fmt.Errorf("list feature overrides: %w", err)
	}
	defer rows.Close()
	out := []domain.FeatureOverride{}
	for rows.Next() {
		var row domain.FeatureOverride
		if err := rows.Scan(&row.NavNodeID, &row.State, &row.UpdatedOn, &row.UpdatedBy); err != nil {
			return domain.FeatureOverrideList{}, fmt.Errorf("list feature overrides: scan: %w", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return domain.FeatureOverrideList{}, fmt.Errorf("list feature overrides: %w", err)
	}
	rows.Close()
	var savedOnce bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM feature_saved)`).Scan(&savedOnce); err != nil {
		return domain.FeatureOverrideList{}, fmt.Errorf("list feature overrides: mark: %w", err)
	}
	return domain.FeatureOverrideList{Overrides: out, SavedOnce: savedOnce}, nil
}

func (r *featureOverrideRepo) Put(ctx context.Context, actor string, req domain.SaveFeatureOverrideRequest) (domain.FeatureOverride, error) {
	var out domain.FeatureOverride
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return out, fmt.Errorf("put feature override: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	err = tx.QueryRow(ctx, `INSERT INTO features (nav_node_id, state, updated_by)
		VALUES ($1, $2, $3)
		ON CONFLICT (nav_node_id) DO UPDATE SET
			state = EXCLUDED.state, updated_on = NOW(), updated_by = EXCLUDED.updated_by
		RETURNING nav_node_id, state, updated_on, updated_by`,
		req.NavNodeID, req.State, actor).Scan(&out.NavNodeID, &out.State, &out.UpdatedOn, &out.UpdatedBy)
	if err != nil {
		return out, fmt.Errorf("put feature override: %w", err)
	}
	if req.MarkSaved {
		if _, err := tx.Exec(ctx, `INSERT INTO feature_saved (only_row, saved_by)
			VALUES (TRUE, $1) ON CONFLICT (only_row) DO NOTHING`, actor); err != nil {
			return out, fmt.Errorf("put feature override: mark: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return out, fmt.Errorf("put feature override: %w", err)
	}
	return out, nil
}
