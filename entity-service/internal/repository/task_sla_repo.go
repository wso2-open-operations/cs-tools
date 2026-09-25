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
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"golang.org/x/sync/errgroup"
)

// TaskSlaRepository defines the read operations for sla (migration 000052),
// joined against sla_policy (migration 000051) and work_item for the task
// reference. BusinessTimeLeft/BusinessElapsedTime/TaskSlaDefinitionDetail.Duration
// are rendered from real INTERVAL columns via formatDurationSeconds -- safe to
// invent a display format for, unlike change_request_repo.go's calendar_duration
// (left nil), because the only consumer (apps/csm-portal/webapp's
// caseSlaMapping.ts/CaseSlaTable.tsx, apps/csm-portal/microapp's SlaTab.tsx)
// renders this string as an opaque label with no parsing at all ("{value}
// left"/"{value} elapsed"), so any clear human-readable rendering is safe.
// ScheduleSource/Flow/Workflow/IsEnableLogging/DurationType/ResetCondition on
// the definition detail remain nil: sla_policy has no backing column for any
// of them (confirmed against the live schema, migration 000051's full column
// list), and -- for ResetCondition specifically -- the column that does exist
// is resume_condition, a different concept from the reset_action enum this
// field would need to derive from.
type TaskSlaRepository interface {
	// SearchTaskSlas returns a filtered, paginated slice of task SLA
	// records together with the total count of matching rows before
	// pagination. projectIDs restricts to SLAs on a work item in one of
	// these projects -- nil means unrestricted (internal caller); the
	// caller is responsible for resolving this from its own AccessScope,
	// never from caller-supplied input, since SearchTaskSlasFilters carries
	// no projectIds field of its own for a customer to narrow with. This
	// filter also naturally excludes SLAs on project-less work items
	// (incident/incident_task, work_item.project_id NULL for 100% of real
	// rows) for any non-nil (external-caller) projectIDs, since NULL never
	// matches ANY(...) -- exactly the fail-closed behavior wanted here,
	// with no separate work_item-type branch needed.
	SearchTaskSlas(ctx context.Context, taskIDs []string, projectIDs []string, limit, offset int) ([]domain.TaskSlaView, int, error)
	// GetTaskSla returns the full detail of a single task SLA record by its
	// UUID, or a NotFoundError if no matching row exists OR it exists but
	// its work item's project isn't in projectIDs (nil means unrestricted).
	GetTaskSla(ctx context.Context, id string, projectIDs []string) (domain.TaskSlaDetail, error)
}

type taskSlaRepo struct {
	db *pgxpool.Pool
}

// NewTaskSlaRepository constructs a TaskSlaRepository backed by the given connection pool.
func NewTaskSlaRepository(db *pgxpool.Pool) TaskSlaRepository {
	return &taskSlaRepo{db: db}
}

const taskSlaViewColumns = `
	sla.id, sla.stage::TEXT,
	sla.work_item_id, wi.number, wi.type::TEXT,
	pol.id, pol.name, pol.target::TEXT,
	sla.business_elapsed_percentage,
	EXTRACT(EPOCH FROM sla.business_duration), EXTRACT(EPOCH FROM sla.remaining_business_duration),
	sla.start_on, sla.end_on`

const taskSlaViewJoins = `
	FROM sla
	LEFT JOIN work_item wi ON wi.id = sla.work_item_id
	LEFT JOIN sla_policy pol ON pol.id = sla.sla_policy_id`

func scanTaskSlaView(row interface{ Scan(...any) error }) (domain.TaskSlaView, error) {
	var v domain.TaskSlaView
	var (
		stage                        *string
		taskID, taskNumber, taskType *string
		polID, polName, polTarget    *string
		elapsedPct                   *float64
		businessDurationSecs         *float64
		remainingBusinessSecs        *float64
		startOn, endOn               *time.Time
	)
	if err := row.Scan(
		&v.ID, &stage,
		&taskID, &taskNumber, &taskType,
		&polID, &polName, &polTarget,
		&elapsedPct,
		&businessDurationSecs, &remainingBusinessSecs,
		&startOn, &endOn,
	); err != nil {
		return domain.TaskSlaView{}, err
	}
	if stage != nil {
		v.Stage = taskSlaStageDisplay(*stage)
	}
	if taskID != nil {
		ref := &domain.TaskSlaTaskRef{ID: taskID, Name: taskNumber}
		if taskType != nil {
			lower := strings.ToLower(*taskType)
			ref.Type = &lower
		}
		v.Task = ref
	}
	if polID != nil {
		def := &domain.TaskSlaDefinition{ID: polID, Name: polName}
		if polTarget != nil {
			lower := strings.ToLower(*polTarget)
			def.Target = &lower
		}
		v.SlaDefinition = def
	}
	v.BusinessElapsedPercentage = elapsedPct
	v.BusinessElapsedTime = formatDurationSecondsPtr(businessDurationSecs)
	v.BusinessTimeLeft = formatDurationSecondsPtr(remainingBusinessSecs)
	if startOn != nil {
		s := startOn.UTC().Format(time.RFC3339)
		v.StartTime = &s
	}
	if endOn != nil {
		s := endOn.UTC().Format(time.RFC3339)
		v.EndTime = &s
	}
	return v, nil
}

// formatDurationSecondsPtr renders an optional duration (seconds, as
// EXTRACT(EPOCH FROM ...) returns) as a compact human-readable string (e.g.
// "2 Days 3 Hours 15 Minutes"), or nil when the source column was NULL.
// Negative values (possible in principle for an already-breached SLA's
// "remaining" column) clamp to zero rather than rendering a negative
// duration -- this codebase has no "overdue by" display convention to
// invent instead.
func formatDurationSecondsPtr(seconds *float64) *string {
	if seconds == nil {
		return nil
	}
	s := formatDurationSeconds(*seconds)
	return &s
}

func formatDurationSeconds(totalSeconds float64) string {
	if totalSeconds < 0 {
		totalSeconds = 0
	}
	total := int64(totalSeconds + 0.5)
	days := total / 86400
	hours := (total % 86400) / 3600
	minutes := (total % 3600) / 60

	unit := func(n int64, label string) string {
		if n == 1 {
			return fmt.Sprintf("1 %s", label)
		}
		return fmt.Sprintf("%d %ss", n, label)
	}

	parts := make([]string, 0, 3)
	if days > 0 {
		parts = append(parts, unit(days, "Day"))
	}
	if hours > 0 {
		parts = append(parts, unit(hours, "Hour"))
	}
	if minutes > 0 || len(parts) == 0 {
		parts = append(parts, unit(minutes, "Minute"))
	}
	return strings.Join(parts, " ")
}

// taskSlaStageDisplay renders sla_stage_enum's SNAKE_UPPER_CASE labels
// (e.g. "IN_PROGRESS") as space-separated title case ("In Progress"),
// matching the ServiceNow-backed implementation's own display convention
// (view.Stage = t.Stage.Label, a human-readable SN label).
func taskSlaStageDisplay(raw string) *string {
	words := strings.Split(strings.ToLower(raw), "_")
	for i, w := range words {
		if w == "" {
			continue
		}
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	s := strings.Join(words, " ")
	return &s
}

// SearchTaskSlas implements TaskSlaRepository.
func (r *taskSlaRepo) SearchTaskSlas(ctx context.Context, taskIDs []string, projectIDs []string, limit, offset int) ([]domain.TaskSlaView, int, error) {
	where := "WHERE 1=1"
	args := []any{}
	if len(taskIDs) > 0 {
		args = append(args, taskIDs)
		where += fmt.Sprintf(" AND sla.work_item_id = ANY($%d::uuid[])", len(args))
	}
	if len(projectIDs) > 0 {
		args = append(args, projectIDs)
		where += fmt.Sprintf(" AND wi.project_id = ANY($%d::uuid[])", len(args))
	}

	countQuery := "SELECT COUNT(*) " + taskSlaViewJoins + " " + where
	dataQuery := fmt.Sprintf(
		`SELECT %s %s %s ORDER BY sla.created_on DESC, sla.id LIMIT $%d OFFSET $%d`,
		taskSlaViewColumns, taskSlaViewJoins, where, len(args)+1, len(args)+2,
	)
	dataArgs := append(append([]any{}, args...), limit, offset)

	var total int
	var views []domain.TaskSlaView

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		if err := r.db.QueryRow(egCtx, countQuery, args...).Scan(&total); err != nil {
			return fmt.Errorf("count task slas: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		rows, err := r.db.Query(egCtx, dataQuery, dataArgs...)
		if err != nil {
			return fmt.Errorf("query task slas: %w", err)
		}
		defer rows.Close()

		result := make([]domain.TaskSlaView, 0, limit)
		for rows.Next() {
			v, err := scanTaskSlaView(rows)
			if err != nil {
				return fmt.Errorf("scan task sla: %w", err)
			}
			result = append(result, v)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate task slas: %w", err)
		}
		views = result
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, 0, err
	}

	return views, total, nil
}

// GetTaskSla implements TaskSlaRepository.
func (r *taskSlaRepo) GetTaskSla(ctx context.Context, id string, projectIDs []string) (domain.TaskSlaDetail, error) {
	where := "WHERE sla.id = $1"
	args := []any{id}
	if len(projectIDs) > 0 {
		args = append(args, projectIDs)
		where += fmt.Sprintf(" AND wi.project_id = ANY($%d::uuid[])", len(args))
	}
	row := r.db.QueryRow(ctx, `
		SELECT sla.id, sla.stage::TEXT, sla.is_active,
		       sla.work_item_id, wi.number, wi.type::TEXT,
		       pol.id, pol.name, pol.target::TEXT,
		       pol.retroactive, pol.retroactive_pause,
		       pol.when_to_cancel::TEXT, pol.cancel_condition,
		       pol.when_to_resume::TEXT, pol.pause_condition,
		       pol.start_condition, pol.stop_condition,
		       pol.timezone_source::TEXT, pol.reset_action::TEXT,
		       EXTRACT(EPOCH FROM pol.duration),
		       sla.schedule, sla.timezone,
		       sla.business_elapsed_percentage,
		       EXTRACT(EPOCH FROM sla.business_duration), EXTRACT(EPOCH FROM sla.remaining_business_duration),
		       sla.start_on, sla.end_on
		FROM sla
		LEFT JOIN work_item wi ON wi.id = sla.work_item_id
		LEFT JOIN sla_policy pol ON pol.id = sla.sla_policy_id
		`+where, args...,
	)

	var (
		v                             domain.TaskSlaDetail
		stage                         *string
		taskID, taskNumber, taskType  *string
		polID, polName, polTarget     *string
		retroactive, retroactivePause *bool
		whenToCancel, cancelCondition *string
		whenToResume, pauseCondition  *string
		startCondition, stopCondition *string
		timezoneSource, resetAction   *string
		policyDurationSecs            *float64
		schedule, timezone            *string
		elapsedPct                    *float64
		businessDurationSecs          *float64
		remainingBusinessSecs         *float64
		startOn, endOn                *time.Time
	)
	err := row.Scan(
		&v.ID, &stage, &v.Active,
		&taskID, &taskNumber, &taskType,
		&polID, &polName, &polTarget,
		&retroactive, &retroactivePause,
		&whenToCancel, &cancelCondition,
		&whenToResume, &pauseCondition,
		&startCondition, &stopCondition,
		&timezoneSource, &resetAction,
		&policyDurationSecs,
		&schedule, &timezone,
		&elapsedPct,
		&businessDurationSecs, &remainingBusinessSecs,
		&startOn, &endOn,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.TaskSlaDetail{}, &apierror.NotFoundError{Msg: "task sla not found"}
	}
	if err != nil {
		return domain.TaskSlaDetail{}, fmt.Errorf("get task sla: %w", err)
	}

	if stage != nil {
		v.Stage = taskSlaStageDisplay(*stage)
	}
	if taskID != nil {
		ref := &domain.TaskSlaTaskRef{ID: taskID, Name: taskNumber}
		if taskType != nil {
			lower := strings.ToLower(*taskType)
			ref.Type = &lower
		}
		v.Task = ref
	}
	if polID != nil {
		def := &domain.TaskSlaDefinitionDetail{ID: polID, Name: polName}
		if polTarget != nil {
			lower := strings.ToLower(*polTarget)
			def.Target = &lower
		}
		def.IsRetroactiveStart = retroactive
		def.IsRetroactivePause = retroactivePause
		if whenToCancel != nil {
			lower := strings.ToLower(*whenToCancel)
			def.WhenToCancel = &lower
		}
		def.CancelCondition = cancelCondition
		if whenToResume != nil {
			lower := strings.ToLower(*whenToResume)
			def.WhenToResume = &lower
		}
		def.PauseCondition = pauseCondition
		def.StartCondition = startCondition
		def.StopCondition = stopCondition
		if timezoneSource != nil {
			lower := strings.ToLower(*timezoneSource)
			def.TimezoneSource = &lower
		}
		if resetAction != nil {
			lower := strings.ToLower(*resetAction)
			def.ResetAction = &lower
		}
		if schedule != nil {
			def.Schedule = schedule
		}
		def.Duration = formatDurationSecondsPtr(policyDurationSecs)
		v.SlaDefinition = def
	}
	if schedule != nil {
		v.Schedule = &domain.TaskSlaScheduleRef{Name: schedule, Timezone: timezone}
	}
	v.BusinessElapsedPercentage = elapsedPct
	v.BusinessElapsedTime = formatDurationSecondsPtr(businessDurationSecs)
	v.BusinessTimeLeft = formatDurationSecondsPtr(remainingBusinessSecs)
	if startOn != nil {
		s := startOn.UTC().Format(time.RFC3339)
		v.StartTime = &s
	}
	if endOn != nil {
		s := endOn.UTC().Format(time.RFC3339)
		v.EndTime = &s
	}
	return v, nil
}
