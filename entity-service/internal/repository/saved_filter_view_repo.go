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

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// SavedFilterViewRepository persists CSM portal saved list-filter views.
type SavedFilterViewRepository interface {
	List(ctx context.Context, userID string, listKey domain.SavedFilterListKey) ([]domain.SavedFilterView, error)
	Count(ctx context.Context, userID string, listKey domain.SavedFilterListKey) (int, error)
	Save(ctx context.Context, userID string, listKey domain.SavedFilterListKey, name, qs string) ([]domain.SavedFilterView, error)
	Delete(ctx context.Context, userID string, listKey domain.SavedFilterListKey, name string) ([]domain.SavedFilterView, error)
	Move(ctx context.Context, userID string, listKey domain.SavedFilterListKey, name string, direction domain.SavedFilterMoveDirection) ([]domain.SavedFilterView, error)
	MoveTo(ctx context.Context, userID string, listKey domain.SavedFilterListKey, name string, position int) ([]domain.SavedFilterView, error)
}

type savedFilterViewRepo struct {
	db *pgxpool.Pool
}

// NewSavedFilterViewRepository constructs a Postgres-backed repository.
func NewSavedFilterViewRepository(db *pgxpool.Pool) SavedFilterViewRepository {
	return &savedFilterViewRepo{db: db}
}

func (r *savedFilterViewRepo) List(ctx context.Context, userID string, listKey domain.SavedFilterListKey) ([]domain.SavedFilterView, error) {
	return r.list(ctx, r.db, userID, listKey)
}

func (r *savedFilterViewRepo) Count(ctx context.Context, userID string, listKey domain.SavedFilterListKey) (int, error) {
	var n int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM user_saved_filter WHERE user_id = $1 AND list_key = $2`,
		userID, string(listKey),
	).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count saved filter views: %w", err)
	}
	return n, nil
}

func (r *savedFilterViewRepo) Save(ctx context.Context, userID string, listKey domain.SavedFilterListKey, name, qs string) ([]domain.SavedFilterView, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("save saved filter view: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := lockSavedFilterList(ctx, tx, userID, listKey); err != nil {
		return nil, err
	}

	var existingID string
	err = tx.QueryRow(ctx,
		`SELECT id FROM user_saved_filter
		 WHERE user_id = $1 AND list_key = $2 AND LOWER(name) = LOWER($3)`,
		userID, string(listKey), name,
	).Scan(&existingID)
	if err != nil && err != pgx.ErrNoRows {
		return nil, fmt.Errorf("save saved filter view: lookup: %w", err)
	}

	if existingID != "" {
		if _, err := tx.Exec(ctx,
			`UPDATE user_saved_filter
			 SET name = $1, qs = $2, updated_on = NOW()
			 WHERE id = $3`,
			name, qs, existingID,
		); err != nil {
			return nil, fmt.Errorf("save saved filter view: update: %w", err)
		}
		if err := r.moveIDToFront(ctx, tx, userID, listKey, existingID); err != nil {
			return nil, err
		}
	} else {
		if _, err := tx.Exec(ctx,
			`UPDATE user_saved_filter
			 SET filter_position = filter_position + 1, updated_on = NOW()
			 WHERE user_id = $1 AND list_key = $2`,
			userID, string(listKey),
		); err != nil {
			return nil, fmt.Errorf("save saved filter view: shift positions: %w", err)
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO user_saved_filter (user_id, list_key, name, qs, filter_position)
			 VALUES ($1, $2, $3, $4, 0)`,
			userID, string(listKey), name, qs,
		); err != nil {
			return nil, fmt.Errorf("save saved filter view: insert: %w", err)
		}
	}

	views, err := r.list(ctx, tx, userID, listKey)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("save saved filter view: commit: %w", err)
	}
	return views, nil
}

func (r *savedFilterViewRepo) Delete(ctx context.Context, userID string, listKey domain.SavedFilterListKey, name string) ([]domain.SavedFilterView, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("delete saved filter view: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := lockSavedFilterList(ctx, tx, userID, listKey); err != nil {
		return nil, err
	}

	if _, err := tx.Exec(ctx,
		`DELETE FROM user_saved_filter
		 WHERE user_id = $1 AND list_key = $2 AND LOWER(name) = LOWER($3)`,
		userID, string(listKey), name,
	); err != nil {
		return nil, fmt.Errorf("delete saved filter view: %w", err)
	}
	if err := r.compactPositions(ctx, tx, userID, listKey); err != nil {
		return nil, err
	}
	views, err := r.list(ctx, tx, userID, listKey)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("delete saved filter view: commit: %w", err)
	}
	return views, nil
}

func (r *savedFilterViewRepo) Move(ctx context.Context, userID string, listKey domain.SavedFilterListKey, name string, direction domain.SavedFilterMoveDirection) ([]domain.SavedFilterView, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("move saved filter view: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := lockSavedFilterList(ctx, tx, userID, listKey); err != nil {
		return nil, err
	}

	var id string
	var pos int
	err = tx.QueryRow(ctx,
		`SELECT id, filter_position FROM user_saved_filter
		 WHERE user_id = $1 AND list_key = $2 AND LOWER(name) = LOWER($3)`,
		userID, string(listKey), name,
	).Scan(&id, &pos)
	if err == pgx.ErrNoRows {
		views, listErr := r.list(ctx, tx, userID, listKey)
		if listErr != nil {
			return nil, listErr
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("move saved filter view: commit: %w", err)
		}
		return views, nil
	}
	if err != nil {
		return nil, fmt.Errorf("move saved filter view: lookup: %w", err)
	}

	target := pos - 1
	if direction == domain.SavedFilterMoveDown {
		target = pos + 1
	}
	var neighborID string
	err = tx.QueryRow(ctx,
		`SELECT id FROM user_saved_filter
		 WHERE user_id = $1 AND list_key = $2 AND filter_position = $3`,
		userID, string(listKey), target,
	).Scan(&neighborID)
	if err == pgx.ErrNoRows {
		views, listErr := r.list(ctx, tx, userID, listKey)
		if listErr != nil {
			return nil, listErr
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("move saved filter view: commit: %w", err)
		}
		return views, nil
	}
	if err != nil {
		return nil, fmt.Errorf("move saved filter view: neighbor: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`UPDATE user_saved_filter SET filter_position = $1, updated_on = NOW() WHERE id = $2`,
		target, id,
	); err != nil {
		return nil, fmt.Errorf("move filter: set filter_position: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`UPDATE user_saved_filter SET filter_position = $1, updated_on = NOW() WHERE id = $2`,
		pos, neighborID,
	); err != nil {
		return nil, fmt.Errorf("move saved filter view: swap neighbor: %w", err)
	}

	views, err := r.list(ctx, tx, userID, listKey)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("move saved filter view: commit: %w", err)
	}
	return views, nil
}

func (r *savedFilterViewRepo) MoveTo(ctx context.Context, userID string, listKey domain.SavedFilterListKey, name string, position int) ([]domain.SavedFilterView, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("move saved filter view: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := lockSavedFilterList(ctx, tx, userID, listKey); err != nil {
		return nil, err
	}

	rows, err := tx.Query(ctx,
		`SELECT id, name FROM user_saved_filter
		 WHERE user_id = $1 AND list_key = $2
		 ORDER BY filter_position ASC`,
		userID, string(listKey),
	)
	if err != nil {
		return nil, fmt.Errorf("move saved filter view: list ids: %w", err)
	}

	var ids []string
	from := -1
	for rows.Next() {
		var id, rowName string
		if err := rows.Scan(&id, &rowName); err != nil {
			return nil, fmt.Errorf("move saved filter view: scan id: %w", err)
		}
		if strings.EqualFold(rowName, name) {
			from = len(ids)
		}
		ids = append(ids, id)
	}
	closeErr := rows.Err()
	rows.Close()
	if closeErr != nil {
		return nil, fmt.Errorf("move saved filter view: ids: %w", closeErr)
	}

	if from >= 0 {
		next, changed := reorderIDs(ids, from, position)
		if changed {
			for i, id := range next {
				if _, err := tx.Exec(ctx,
					`UPDATE user_saved_filter SET filter_position = $1, updated_on = NOW() WHERE id = $2`,
					i, id,
				); err != nil {
					return nil, fmt.Errorf("move saved filter view: set position: %w", err)
				}
			}
		}
	}

	views, err := r.list(ctx, tx, userID, listKey)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("move saved filter view: commit: %w", err)
	}
	return views, nil
}

// reorderIDs moves the id at from to the 0-based index to and returns the
// compacted order. An out-of-range to is clamped. No change returns the
// original slice and false.
func reorderIDs(ids []string, from, to int) ([]string, bool) {
	if len(ids) == 0 || from < 0 || from >= len(ids) {
		return ids, false
	}
	if to < 0 {
		to = 0
	}
	if to >= len(ids) {
		to = len(ids) - 1
	}
	if from == to {
		return ids, false
	}
	next := append([]string(nil), ids...)
	id := next[from]
	next = append(next[:from], next[from+1:]...)
	if to > len(next) {
		to = len(next)
	}
	next = append(next[:to], append([]string{id}, next[to:]...)...)
	return next, true
}

// lockSavedFilterList serializes Save/Delete/Move for one (user_id, list_key)
// for the rest of the transaction. Row FOR UPDATE cannot cover an empty list,
// so two concurrent first inserts would both claim filter_position 0.
func lockSavedFilterList(ctx context.Context, tx pgx.Tx, userID string, listKey domain.SavedFilterListKey) error {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1), hashtext($2))`, userID, string(listKey)); err != nil {
		return fmt.Errorf("lock saved filter views: %w", err)
	}
	return nil
}

type queryer interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func (r *savedFilterViewRepo) list(ctx context.Context, q queryer, userID string, listKey domain.SavedFilterListKey) ([]domain.SavedFilterView, error) {
	rows, err := q.Query(ctx,
		`SELECT name, qs FROM user_saved_filter
		 WHERE user_id = $1 AND list_key = $2
		 ORDER BY filter_position ASC`,
		userID, string(listKey),
	)
	if err != nil {
		return nil, fmt.Errorf("list saved filter views: %w", err)
	}
	defer rows.Close()

	views := make([]domain.SavedFilterView, 0)
	for rows.Next() {
		var v domain.SavedFilterView
		if err := rows.Scan(&v.Name, &v.Qs); err != nil {
			return nil, fmt.Errorf("list saved filter views: scan: %w", err)
		}
		views = append(views, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list saved filter views: rows: %w", err)
	}
	return views, nil
}

func (r *savedFilterViewRepo) moveIDToFront(ctx context.Context, tx pgx.Tx, userID string, listKey domain.SavedFilterListKey, id string) error {
	if _, err := tx.Exec(ctx,
		`UPDATE user_saved_filter
		 SET filter_position = filter_position + 1, updated_on = NOW()
		 WHERE user_id = $1 AND list_key = $2 AND id <> $3`,
		userID, string(listKey), id,
	); err != nil {
		return fmt.Errorf("save saved filter view: bump others: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`UPDATE user_saved_filter SET filter_position = 0, updated_on = NOW() WHERE id = $1`,
		id,
	); err != nil {
		return fmt.Errorf("save saved filter view: move to front: %w", err)
	}
	return r.compactPositions(ctx, tx, userID, listKey)
}

func (r *savedFilterViewRepo) compactPositions(ctx context.Context, tx pgx.Tx, userID string, listKey domain.SavedFilterListKey) error {
	rows, err := tx.Query(ctx,
		`SELECT id FROM user_saved_filter
		 WHERE user_id = $1 AND list_key = $2
		 ORDER BY filter_position ASC, updated_on DESC`,
		userID, string(listKey),
	)
	if err != nil {
		return fmt.Errorf("compact saved filter view positions: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("compact saved filter view positions: scan: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("compact saved filter view positions: rows: %w", err)
	}
	for i, id := range ids {
		if _, err := tx.Exec(ctx,
			`UPDATE user_saved_filter SET filter_position = $1 WHERE id = $2`,
			i, id,
		); err != nil {
			return fmt.Errorf("compact saved filter view positions: update: %w", err)
		}
	}
	return nil
}
