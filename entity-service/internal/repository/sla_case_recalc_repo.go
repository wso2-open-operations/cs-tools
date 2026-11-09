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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package repository

import (
	"context"
	"fmt"
	"time"
)

// CaseActivityEvent is one work_item_activity row -- a field's value
// changing at a point in time. See SLAEngineRepository.ListCaseActivityEvents'
// own doc comment for what this backs and its real, known gap (the
// ServiceNow-backed write path never populates this table at all).
type CaseActivityEvent struct {
	FieldName string
	OldValue  string
	NewValue  string
	CreatedOn time.Time
}

// ActiveClockForRecalc is one currently-active "sla" row for a work item,
// joined to the subset of its own sla_policy's columns
// SLAEngineRepository.ListActiveClocksForRecalc's one caller
// (SLAEngineService.RecalculateCasePauseTotals) needs to replay its
// pause/resume history.
type ActiveClockForRecalc struct {
	PolicyID        string
	Target          string
	Schedule        string
	Source          string
	StopCondition   string
	PauseCondition  string
	ResumeCondition string
	CancelCondition string
	WhenToCancel    string
	WhenToResume    string
	StartOn         time.Time
	// Stage is the row's raw sla_stage_enum label -- RecalculateCasePauseTotals
	// only ever corrects a clock currently IN_PROGRESS; see that method's
	// own doc comment for why PAUSED/terminal clocks are out of scope for
	// this first pass.
	Stage                string
	PauseSeconds         int64
	BusinessPauseSeconds int64
}

// ListCaseActivityEvents implements SLAEngineRepository.
func (r *slaEngineRepo) ListCaseActivityEvents(ctx context.Context, workItemID string, fieldNames []string) ([]CaseActivityEvent, error) {
	query := `
		SELECT field_name, COALESCE(old_value, ''), COALESCE(new_value, ''), created_on
		FROM work_item_activity
		WHERE work_item_id = $1::uuid AND field_name = ANY($2)
		ORDER BY created_on ASC`
	rows, err := r.db.Query(ctx, query, workItemID, fieldNames)
	if err != nil {
		return nil, fmt.Errorf("list case activity events: %w", err)
	}
	defer rows.Close()

	var events []CaseActivityEvent
	for rows.Next() {
		var e CaseActivityEvent
		if err := rows.Scan(&e.FieldName, &e.OldValue, &e.NewValue, &e.CreatedOn); err != nil {
			return nil, fmt.Errorf("list case activity events: scan: %w", err)
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list case activity events: rows: %w", err)
	}
	return events, nil
}

// ListActiveClocksForRecalc implements SLAEngineRepository.
func (r *slaEngineRepo) ListActiveClocksForRecalc(ctx context.Context, workItemID string) ([]ActiveClockForRecalc, error) {
	query := `
		SELECT sp.id::TEXT, sp.target::TEXT, COALESCE(sp.schedule, ''), sp.source::TEXT,
		       COALESCE(sp.stop_condition, ''), COALESCE(sp.pause_condition, ''),
		       COALESCE(sp.resume_condition, ''), COALESCE(sp.cancel_condition, ''),
		       COALESCE(sp.when_to_cancel::TEXT, 'CANCEL_CONDITIONS_ARE_MET'),
		       COALESCE(sp.when_to_resume::TEXT, 'RESUME_CONDITIONS_ARE_MET'),
		       s.start_on, s.stage::TEXT,
		       COALESCE(EXTRACT(EPOCH FROM s.pause_duration), 0)::BIGINT,
		       COALESCE(EXTRACT(EPOCH FROM s.business_pause_duration), 0)::BIGINT
		FROM sla s
		JOIN sla_policy sp ON sp.id = s.sla_policy_id
		WHERE s.work_item_id = $1::uuid AND s.is_active IS TRUE
		ORDER BY s.created_on`
	rows, err := r.db.Query(ctx, query, workItemID)
	if err != nil {
		return nil, fmt.Errorf("list active clocks for recalc: %w", err)
	}
	defer rows.Close()

	var clocks []ActiveClockForRecalc
	for rows.Next() {
		var c ActiveClockForRecalc
		if err := rows.Scan(
			&c.PolicyID, &c.Target, &c.Schedule, &c.Source,
			&c.StopCondition, &c.PauseCondition, &c.ResumeCondition, &c.CancelCondition,
			&c.WhenToCancel, &c.WhenToResume,
			&c.StartOn, &c.Stage,
			&c.PauseSeconds, &c.BusinessPauseSeconds,
		); err != nil {
			return nil, fmt.Errorf("list active clocks for recalc: scan: %w", err)
		}
		clocks = append(clocks, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list active clocks for recalc: rows: %w", err)
	}
	return clocks, nil
}

// UpdateClockPauseTotals implements SLAEngineRepository.
func (r *slaEngineRepo) UpdateClockPauseTotals(ctx context.Context, workItemID, policyID string, pauseSeconds int64, businessPauseSeconds *int64) error {
	query := `
		UPDATE sla
		SET pause_duration = make_interval(secs => $3::DOUBLE PRECISION),
		    business_pause_duration = CASE WHEN $4::BIGINT IS NULL THEN business_pause_duration
		                                   ELSE make_interval(secs => $4::DOUBLE PRECISION)
		                              END,
		    updated_on = NOW(), updated_by = $5
		WHERE work_item_id = $1::uuid AND sla_policy_id = $2::uuid`
	if _, err := r.db.Exec(ctx, query, workItemID, policyID, pauseSeconds, businessPauseSeconds, sqlActorLiteral); err != nil {
		return fmt.Errorf("update clock pause totals: %w", err)
	}
	return nil
}
