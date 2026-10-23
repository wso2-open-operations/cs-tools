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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// IsJSONNull reports whether raw is empty or the literal JSON `null` --
// encoding/json hands a json.RawMessage field a real 4-byte "null" for an
// explicit `"pauseCondition": null` in the request body, not an empty
// slice, so checking length alone treats that as "set to null" rather than
// "not set" (R19-R21's "not set never fires"). Both must mean the same
// "not set" here and in the service's own check on the required start
// condition (exported so sla_definition_service.go's validation uses the
// identical rule, not a second copy of it).
func IsJSONNull(raw json.RawMessage) bool {
	return len(raw) == 0 || string(bytes.TrimSpace(raw)) == "null"
}

// rawOrNull binds an optional predicate condition (pause/stop/cancel -- R19-
// R21, "not set never fires") as SQL NULL when the caller sent none or an
// explicit JSON null, rather than an empty byte slice (which pgx's jsonb
// codec does not accept as "no value") or the literal 'null'::jsonb (a
// third, meaningless state next to "not set" and "set").
func rawOrNull(raw json.RawMessage) any {
	if IsJSONNull(raw) {
		return nil
	}
	return raw
}

// SLADefinitionRepository is sla_definition (migration 0247) -- the admin
// policy itself. Validating a predicate tree's fields/operators, and
// whether a status transition is legal, is the service's job; this
// repository is plain data access.
type SLADefinitionRepository interface {
	Search(ctx context.Context, workItemType, status *string, limit, offset int) ([]domain.SLADefinition, int, error)
	Get(ctx context.Context, id string) (domain.SLADefinition, error)
	Create(ctx context.Context, req domain.UpsertSLADefinitionRequest, actorEmail string) (domain.SLADefinition, error)
	Update(ctx context.Context, id string, req domain.UpsertSLADefinitionRequest, actorEmail string) (domain.SLADefinition, error)
	SetStatus(ctx context.Context, id, expectedStatus, newStatus, actorEmail string) (domain.SLADefinition, error)
}

type slaDefinitionRepository struct{ db *pgxpool.Pool }

// NewSLADefinitionRepository constructs an SLADefinitionRepository.
func NewSLADefinitionRepository(db *pgxpool.Pool) SLADefinitionRepository {
	return &slaDefinitionRepository{db: db}
}

const slaDefinitionSelect = `
	SELECT id::text, name, description, work_item_type::text, status, calendar_id::text,
	       timezone_source, definition_timezone, account_timezone_fallback, start_mode,
	       target_label, EXTRACT(EPOCH FROM duration)::bigint, pause_reason,
	       start_condition, pause_condition, stop_condition, cancel_condition,
	       threshold_percentages, also_completes_definition_ids::text[],
	       version, created_on, updated_on, created_by, updated_by
	  FROM sla_definition`

func scanSLADefinition(row pgx.Row) (domain.SLADefinition, error) {
	var d domain.SLADefinition
	if err := row.Scan(
		&d.ID, &d.Name, &d.Description, &d.WorkItemType, &d.Status, &d.CalendarID,
		&d.TimezoneSource, &d.DefinitionTimezone, &d.AccountTimezoneFallback, &d.StartMode,
		&d.TargetLabel, &d.DurationSeconds, &d.PauseReason,
		&d.StartCondition, &d.PauseCondition, &d.StopCondition, &d.CancelCondition,
		&d.ThresholdPercentages, &d.AlsoCompletesDefinitionIDs,
		&d.Version, &d.CreatedOn, &d.UpdatedOn, &d.CreatedBy, &d.UpdatedBy,
	); err != nil {
		return domain.SLADefinition{}, err
	}
	return d, nil
}

func (r *slaDefinitionRepository) Search(ctx context.Context, workItemType, status *string, limit, offset int) ([]domain.SLADefinition, int, error) {
	var wit, st string
	if workItemType != nil {
		wit = *workItemType
	}
	if status != nil {
		st = *status
	}
	var total int
	if err := r.db.QueryRow(ctx, `
		SELECT count(*) FROM sla_definition
		 WHERE ($1 = '' OR work_item_type::text = $1) AND ($2 = '' OR status = $2)`,
		wit, st).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count sla definitions: %w", err)
	}
	rows, err := r.db.Query(ctx, slaDefinitionSelect+`
		 WHERE ($1 = '' OR work_item_type::text = $1) AND ($2 = '' OR status = $2)
		 ORDER BY created_on DESC, id LIMIT $3 OFFSET $4`,
		wit, st, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("search sla definitions: %w", err)
	}
	defer rows.Close()
	out := []domain.SLADefinition{}
	for rows.Next() {
		d, err := scanSLADefinition(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan sla definition: %w", err)
		}
		out = append(out, d)
	}
	return out, total, rows.Err()
}

func (r *slaDefinitionRepository) Get(ctx context.Context, id string) (domain.SLADefinition, error) {
	d, err := scanSLADefinition(r.db.QueryRow(ctx, slaDefinitionSelect+` WHERE id = $1::uuid`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.SLADefinition{}, &apierror.NotFoundError{Msg: "sla definition not found"}
	}
	if err != nil {
		return domain.SLADefinition{}, fmt.Errorf("get sla definition: %w", err)
	}
	return d, nil
}

// Create always lands the new definition in 'draft' (R14) -- a caller may
// never create one already published.
func (r *slaDefinitionRepository) Create(ctx context.Context, req domain.UpsertSLADefinitionRequest, actorEmail string) (domain.SLADefinition, error) {
	thresholds := req.ThresholdPercentages
	if len(thresholds) == 0 {
		thresholds = []int{100}
	}
	cascade := req.AlsoCompletesDefinitionIDs
	if cascade == nil {
		cascade = []string{}
	}
	d, err := scanSLADefinition(r.db.QueryRow(ctx, `
		INSERT INTO sla_definition (
			name, description, work_item_type, status, calendar_id,
			timezone_source, definition_timezone, account_timezone_fallback, start_mode,
			target_label, duration, pause_reason,
			start_condition, pause_condition, stop_condition, cancel_condition,
			threshold_percentages, also_completes_definition_ids,
			created_by, updated_by
		) VALUES (
			$1, $2, $3::work_item_type_enum, 'draft', $4::uuid,
			$5, $6, $7, $8,
			$9, ($10::bigint * INTERVAL '1 second'), $11,
			$12, $13, $14, $15,
			$16::integer[], $17::uuid[],
			$18, $18
		)
		RETURNING id::text, name, description, work_item_type::text, status, calendar_id::text,
		          timezone_source, definition_timezone, account_timezone_fallback, start_mode,
		          target_label, EXTRACT(EPOCH FROM duration)::bigint, pause_reason,
		          start_condition, pause_condition, stop_condition, cancel_condition,
		          threshold_percentages, also_completes_definition_ids::text[],
		          version, created_on, updated_on, created_by, updated_by`,
		req.Name, req.Description, req.WorkItemType, req.CalendarID,
		req.TimezoneSource, req.DefinitionTimezone, req.AccountTimezoneFallback, req.StartMode,
		req.TargetLabel, req.DurationSeconds, req.PauseReason,
		rawOrNull(req.StartCondition), rawOrNull(req.PauseCondition), rawOrNull(req.StopCondition), rawOrNull(req.CancelCondition),
		thresholds, cascade,
		actorEmail))
	if err != nil {
		return domain.SLADefinition{}, fmt.Errorf("create sla definition: %w", err)
	}
	return d, nil
}

// Update replaces every editable field and bumps version (R2's "edit"
// half -- status is changed only through SetStatus).
func (r *slaDefinitionRepository) Update(ctx context.Context, id string, req domain.UpsertSLADefinitionRequest, actorEmail string) (domain.SLADefinition, error) {
	thresholds := req.ThresholdPercentages
	if len(thresholds) == 0 {
		thresholds = []int{100}
	}
	cascade := req.AlsoCompletesDefinitionIDs
	if cascade == nil {
		cascade = []string{}
	}
	d, err := scanSLADefinition(r.db.QueryRow(ctx, `
		UPDATE sla_definition SET
			name = $2, description = $3, work_item_type = $4::work_item_type_enum, calendar_id = $5::uuid,
			timezone_source = $6, definition_timezone = $7, account_timezone_fallback = $8, start_mode = $9,
			target_label = $10, duration = ($11::bigint * INTERVAL '1 second'), pause_reason = $12,
			start_condition = $13, pause_condition = $14, stop_condition = $15, cancel_condition = $16,
			threshold_percentages = $17::integer[], also_completes_definition_ids = $18::uuid[],
			version = version + 1, updated_on = now(), updated_by = $19
		WHERE id = $1::uuid
		RETURNING id::text, name, description, work_item_type::text, status, calendar_id::text,
		          timezone_source, definition_timezone, account_timezone_fallback, start_mode,
		          target_label, EXTRACT(EPOCH FROM duration)::bigint, pause_reason,
		          start_condition, pause_condition, stop_condition, cancel_condition,
		          threshold_percentages, also_completes_definition_ids::text[],
		          version, created_on, updated_on, created_by, updated_by`,
		id, req.Name, req.Description, req.WorkItemType, req.CalendarID,
		req.TimezoneSource, req.DefinitionTimezone, req.AccountTimezoneFallback, req.StartMode,
		req.TargetLabel, req.DurationSeconds, req.PauseReason,
		rawOrNull(req.StartCondition), rawOrNull(req.PauseCondition), rawOrNull(req.StopCondition), rawOrNull(req.CancelCondition),
		thresholds, cascade,
		actorEmail))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.SLADefinition{}, &apierror.NotFoundError{Msg: "sla definition not found"}
	}
	if err != nil {
		return domain.SLADefinition{}, fmt.Errorf("update sla definition: %w", err)
	}
	return d, nil
}

// SetStatus moves the definition to newStatus only if its stored status is
// still expectedStatus -- a compare-and-swap, not a blind write. Without
// this, two concurrent requests (e.g. "Retire" and "Publish") could both
// read the same current status in the service's own pre-check, both pass
// it, and the second write would win unconditionally: a definition already
// retired could be silently moved back to published, breaking "retired is
// final." A row that exists but no longer matches expectedStatus is a real
// concurrent change, reported as ConflictError rather than overwritten;
// a row that doesn't exist at all is still NotFoundError, so the two
// callers' failure modes stay distinguishable.
func (r *slaDefinitionRepository) SetStatus(ctx context.Context, id, expectedStatus, newStatus, actorEmail string) (domain.SLADefinition, error) {
	d, err := scanSLADefinition(r.db.QueryRow(ctx, `
		UPDATE sla_definition SET status = $2, version = version + 1, updated_on = now(), updated_by = $3
		WHERE id = $1::uuid AND status = $4
		RETURNING id::text, name, description, work_item_type::text, status, calendar_id::text,
		          timezone_source, definition_timezone, account_timezone_fallback, start_mode,
		          target_label, EXTRACT(EPOCH FROM duration)::bigint, pause_reason,
		          start_condition, pause_condition, stop_condition, cancel_condition,
		          threshold_percentages, also_completes_definition_ids::text[],
		          version, created_on, updated_on, created_by, updated_by`,
		id, newStatus, actorEmail, expectedStatus))
	if errors.Is(err, pgx.ErrNoRows) {
		var exists bool
		if checkErr := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM sla_definition WHERE id = $1::uuid)`, id).Scan(&exists); checkErr != nil {
			return domain.SLADefinition{}, fmt.Errorf("check sla definition exists: %w", checkErr)
		}
		if !exists {
			return domain.SLADefinition{}, &apierror.NotFoundError{Msg: "sla definition not found"}
		}
		return domain.SLADefinition{}, &apierror.ConflictError{Msg: "sla definition status changed concurrently; reload and try again"}
	}
	if err != nil {
		return domain.SLADefinition{}, fmt.Errorf("set sla definition status: %w", err)
	}
	return d, nil
}
