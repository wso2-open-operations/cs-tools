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
	"github.com/jackc/pgx/v5/pgxpool"
)

// ProjectTypeRow is one row of the project_type table (migration
// 000026_project_type_table).
type ProjectTypeRow struct {
	ID   string
	Name string
}

// ReferenceDataRepository backs the choice-list/reference-data reads shared
// by the project-metadata (GET /projects/{id}/metadata) and system-metadata
// (GET /metadata) endpoints: project_type rows and Postgres enum labels.
type ReferenceDataRepository interface {
	// ListProjectTypes returns every project_type row not explicitly marked
	// inactive, ordered by name.
	ListProjectTypes(ctx context.Context) ([]ProjectTypeRow, error)
	// GetProjectByID reports whether a project with this id exists, and its
	// linked project_type row (nil if the project has none set via
	// project.project_type_id).
	GetProjectByID(ctx context.Context, projectID string) (found bool, projectType *ProjectTypeRow, err error)
	// EnumLabels returns, for each of enumTypeNames, its ordered label list --
	// queried live from pg_catalog rather than hardcoded, so the result
	// always matches whatever the migrations currently define. A requested
	// type with no matching rows is simply absent from the returned map.
	EnumLabels(ctx context.Context, enumTypeNames []string) (map[string][]string, error)
}

type referenceDataRepo struct {
	db *pgxpool.Pool
}

// NewReferenceDataRepository constructs a ReferenceDataRepository backed by the given connection pool.
func NewReferenceDataRepository(db *pgxpool.Pool) ReferenceDataRepository {
	return &referenceDataRepo{db: db}
}

// ListProjectTypes implements ReferenceDataRepository.
func (r *referenceDataRepo) ListProjectTypes(ctx context.Context) ([]ProjectTypeRow, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, name FROM project_type WHERE is_active IS DISTINCT FROM FALSE ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list project types: %w", err)
	}
	defer rows.Close()

	var out []ProjectTypeRow
	for rows.Next() {
		var pt ProjectTypeRow
		if err := rows.Scan(&pt.ID, &pt.Name); err != nil {
			return nil, fmt.Errorf("scan project type: %w", err)
		}
		out = append(out, pt)
	}
	return out, rows.Err()
}

// GetProjectByID implements ReferenceDataRepository.
func (r *referenceDataRepo) GetProjectByID(ctx context.Context, projectID string) (bool, *ProjectTypeRow, error) {
	var ptID, ptName *string
	err := r.db.QueryRow(ctx,
		`SELECT pt.id, pt.name
		 FROM project p
		 LEFT JOIN project_type pt ON pt.id = p.project_type_id
		 WHERE p.id = $1`, projectID,
	).Scan(&ptID, &ptName)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil, nil
	}
	if err != nil {
		return false, nil, fmt.Errorf("get project by id: %w", err)
	}
	if ptID == nil {
		return true, nil, nil
	}
	return true, &ProjectTypeRow{ID: *ptID, Name: *ptName}, nil
}

// EnumLabels implements ReferenceDataRepository.
func (r *referenceDataRepo) EnumLabels(ctx context.Context, enumTypeNames []string) (map[string][]string, error) {
	// pg_type_is_visible(t.oid) scopes this to whichever single schema this
	// connection's search_path would actually resolve typname to -- the same
	// resolution an unqualified CREATE TYPE/enum reference in a migration
	// gets. This deployment's schema is neither a fixed literal (verified
	// live: it's "$user"-resolved, e.g. "csm_platform_stg_user" in staging,
	// not "public") nor discoverable from any config/compose/migration file
	// in this repo, so pg_type_is_visible is the only portable way to avoid
	// scanning every schema in the database -- without it, a same-named enum
	// type in another schema would silently merge its labels into this one's
	// map entry.
	rows, err := r.db.Query(ctx,
		`SELECT t.typname::text, e.enumlabel
		 FROM pg_type t
		 JOIN pg_enum e ON e.enumtypid = t.oid
		 WHERE pg_catalog.pg_type_is_visible(t.oid)
		   AND t.typname::text = ANY($1::text[])
		 ORDER BY t.typname, e.enumsortorder`, enumTypeNames)
	if err != nil {
		return nil, fmt.Errorf("enum labels: %w", err)
	}
	defer rows.Close()

	out := make(map[string][]string, len(enumTypeNames))
	for rows.Next() {
		var typ, label string
		if err := rows.Scan(&typ, &label); err != nil {
			return nil, fmt.Errorf("scan enum label: %w", err)
		}
		out[typ] = append(out[typ], label)
	}
	return out, rows.Err()
}
