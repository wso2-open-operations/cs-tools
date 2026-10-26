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
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// SLAFieldRegistryRepository is the SLA module's field registry (migration
// 0245): which work item types may carry SLAs (sla_work_item_type) and
// which of their fields a condition may reference (sla_field_catalog_entry,
// joined with its sla_field_enablement row). The catalog itself is
// developer-seeded and never written here -- only ListWorkItemTypes and
// SetFieldEnabled are writable, and only the latter is ever called from an
// admin request.
type SLAFieldRegistryRepository interface {
	ListWorkItemTypes(ctx context.Context) ([]domain.SLAWorkItemType, error)
	ListFieldCatalog(ctx context.Context, workItemType string) ([]domain.SLAFieldCatalogEntry, error)
	SetFieldEnabled(ctx context.Context, fieldCatalogEntryID string, enabled bool, actorEmail string) (domain.SLAFieldCatalogEntry, error)
}

type slaFieldRegistryRepository struct{ db *pgxpool.Pool }

// NewSLAFieldRegistryRepository constructs an SLAFieldRegistryRepository.
func NewSLAFieldRegistryRepository(db *pgxpool.Pool) SLAFieldRegistryRepository {
	return &slaFieldRegistryRepository{db: db}
}

func (r *slaFieldRegistryRepository) ListWorkItemTypes(ctx context.Context) ([]domain.SLAWorkItemType, error) {
	rows, err := r.db.Query(ctx, `SELECT work_item_type::text, enabled FROM sla_work_item_type ORDER BY work_item_type`)
	if err != nil {
		return nil, fmt.Errorf("list sla work item types: %w", err)
	}
	defer rows.Close()
	out := []domain.SLAWorkItemType{}
	for rows.Next() {
		var t domain.SLAWorkItemType
		if err := rows.Scan(&t.WorkItemType, &t.Enabled); err != nil {
			return nil, fmt.Errorf("scan sla work item type: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

const slaFieldCatalogEntrySelect = `
	SELECT fc.id::text, fc.work_item_type::text, fc.field_key, fc.data_type, fc.enum_values,
	       fc.display_label, fc.is_state_field, COALESCE(fe.enabled, false)
	  FROM sla_field_catalog_entry fc
	  LEFT JOIN sla_field_enablement fe ON fe.field_catalog_entry_id = fc.id`

func scanSLAFieldCatalogEntry(row pgx.Row) (domain.SLAFieldCatalogEntry, error) {
	var e domain.SLAFieldCatalogEntry
	var enumValuesRaw []byte
	if err := row.Scan(&e.ID, &e.WorkItemType, &e.FieldKey, &e.DataType, &enumValuesRaw,
		&e.DisplayLabel, &e.IsStateField, &e.Enabled); err != nil {
		return domain.SLAFieldCatalogEntry{}, err
	}
	if len(enumValuesRaw) > 0 {
		if err := json.Unmarshal(enumValuesRaw, &e.EnumValues); err != nil {
			return domain.SLAFieldCatalogEntry{}, fmt.Errorf("decode sla field catalog entry enum_values: %w", err)
		}
	}
	return e, nil
}

// ListFieldCatalog returns every catalog entry, for every work item type
// when workItemType is empty.
func (r *slaFieldRegistryRepository) ListFieldCatalog(ctx context.Context, workItemType string) ([]domain.SLAFieldCatalogEntry, error) {
	query := slaFieldCatalogEntrySelect + ` WHERE ($1 = '' OR fc.work_item_type::text = $1) ORDER BY fc.work_item_type, fc.field_key`
	rows, err := r.db.Query(ctx, query, workItemType)
	if err != nil {
		return nil, fmt.Errorf("list sla field catalog: %w", err)
	}
	defer rows.Close()
	out := []domain.SLAFieldCatalogEntry{}
	for rows.Next() {
		e, err := scanSLAFieldCatalogEntry(rows)
		if err != nil {
			return nil, fmt.Errorf("scan sla field catalog entry: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// SetFieldEnabled upserts the one admin-writable row per catalog entry
// (R16) and returns the entry's full, joined view. Checks the catalog entry
// actually exists first, rather than relying on the upsert's own foreign
// key violation, so a bad id is a clean NotFoundError rather than a raw
// constraint error surfacing to the caller.
func (r *slaFieldRegistryRepository) SetFieldEnabled(ctx context.Context, fieldCatalogEntryID string, enabled bool, actorEmail string) (domain.SLAFieldCatalogEntry, error) {
	var exists bool
	if err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM sla_field_catalog_entry WHERE id = $1::uuid)`, fieldCatalogEntryID).Scan(&exists); err != nil {
		return domain.SLAFieldCatalogEntry{}, fmt.Errorf("check sla field catalog entry exists: %w", err)
	}
	if !exists {
		return domain.SLAFieldCatalogEntry{}, &apierror.NotFoundError{Msg: "sla field catalog entry not found"}
	}
	if _, err := r.db.Exec(ctx, `
		INSERT INTO sla_field_enablement (field_catalog_entry_id, enabled, updated_on, updated_by)
		VALUES ($1::uuid, $2, now(), $3)
		ON CONFLICT (field_catalog_entry_id)
		DO UPDATE SET enabled = EXCLUDED.enabled, updated_on = now(), updated_by = EXCLUDED.updated_by`,
		fieldCatalogEntryID, enabled, actorEmail); err != nil {
		return domain.SLAFieldCatalogEntry{}, fmt.Errorf("set sla field enablement: %w", err)
	}
	e, err := scanSLAFieldCatalogEntry(r.db.QueryRow(ctx, slaFieldCatalogEntrySelect+` WHERE fc.id = $1::uuid`, fieldCatalogEntryID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.SLAFieldCatalogEntry{}, &apierror.NotFoundError{Msg: "sla field catalog entry not found"}
	}
	if err != nil {
		return domain.SLAFieldCatalogEntry{}, fmt.Errorf("get sla field catalog entry after enablement: %w", err)
	}
	return e, nil
}
