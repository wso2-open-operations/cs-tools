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
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// SlaReportClockRow is one case opened in the report window, joined to at
// most one clock for one target. When the case has no clock every field from
// Target onward is nil.
type SlaReportClockRow struct {
	CaseID         string
	CaseNumber     string
	IssueType      string
	Severity       string
	State          string
	Opened         time.Time
	Target         *string
	Stage          *string
	HasBreached    *bool
	ElapsedSeconds *float64
	TargetSeconds  *float64
}

// SlaReportRepository reads the SLA clocks of one project's cases.
type SlaReportRepository interface {
	// Project returns the project header, or a NotFoundError.
	Project(ctx context.Context, projectID string) (domain.SlaReportProject, error)
	// Clocks returns the cases opened in [from, to] with the latest clock per
	// target. A case with no clock is returned once, with nil clock fields.
	Clocks(ctx context.Context, projectID string, from, to time.Time) ([]SlaReportClockRow, error)
}

type slaReportRepo struct {
	db *Scoped
}

// NewSlaReportRepository constructs a SlaReportRepository. sla has no row
// level security since migration 100014, so access is enforced by internalOnly
// on the route; this repository applies no project filtering beyond the one
// the report asks for.
func NewSlaReportRepository(db *Scoped) SlaReportRepository {
	return &slaReportRepo{db: db}
}

func (r *slaReportRepo) Project(ctx context.Context, projectID string) (domain.SlaReportProject, error) {
	var project domain.SlaReportProject
	err := r.db.QueryRow(ctx, `SELECT id::text, key, name FROM project WHERE id = $1::uuid`, projectID).
		Scan(&project.ID, &project.Key, &project.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.SlaReportProject{}, &apierror.NotFoundError{Msg: "project not found"}
	}
	if err != nil {
		return domain.SlaReportProject{}, fmt.Errorf("sla report: project: %w", err)
	}
	return project, nil
}

// slaReportClocksQuery reads sla_live, never sla, so elapsed time, stage and
// breach reflect this moment, and the policy's own duration (sla_policy.duration)
// is the target, never the clock's frozen sla.duration. The latest clock per
// (case, target) is taken without an is_active filter: a report must include
// the clocks that already finished. The clocks are limited to the cases of
// the window inside the CTE, so the view is not evaluated for every clock.
const slaReportClocksQuery = `
	WITH window_cases AS (
		SELECT wi.id
		FROM work_item wi
		JOIN "case" c ON c.id = wi.id
		WHERE wi.project_id = $1::uuid
		  AND wi.created_on >= $2
		  AND wi.created_on <= $3
	), clocks AS (
		SELECT DISTINCT ON (s.work_item_id, sp.target)
			s.work_item_id,
			sp.target::text AS target,
			s.live_stage::text AS stage,
			s.live_has_breached AS has_breached,
			EXTRACT(EPOCH FROM s.live_business_duration) AS elapsed_seconds,
			EXTRACT(EPOCH FROM sp.duration) AS target_seconds,
			COALESCE(s.schedule, '') AS schedule,
			s.start_on,
			COALESCE(EXTRACT(EPOCH FROM s.business_pause_duration), 0) AS business_pause_seconds
		FROM sla_live s
		JOIN sla_policy sp ON sp.id = s.sla_policy_id
		WHERE sp.target IS NOT NULL
		  AND s.work_item_id IN (SELECT id FROM window_cases)
		ORDER BY s.work_item_id, sp.target, s.start_on DESC NULLS LAST, s.updated_on DESC
	)
	SELECT
		wi.id::text,
		wi.number,
		COALESCE(c.issue_type::text, ''),
		COALESCE(c.severity::text, ''),
		COALESCE(c.state::text, ''),
		wi.created_on,
		clocks.target,
		clocks.stage,
		clocks.has_breached,
		clocks.elapsed_seconds,
		clocks.target_seconds,
		clocks.schedule,
		clocks.start_on,
		clocks.business_pause_seconds
	FROM window_cases wc
	JOIN work_item wi ON wi.id = wc.id
	JOIN "case" c ON c.id = wi.id
	LEFT JOIN clocks ON clocks.work_item_id = wi.id
	ORDER BY wi.created_on DESC, wi.number, clocks.target`

func (r *slaReportRepo) Clocks(ctx context.Context, projectID string, from, to time.Time) ([]SlaReportClockRow, error) {
	rows, err := r.db.Query(ctx, slaReportClocksQuery, projectID, from, to)
	if err != nil {
		return nil, fmt.Errorf("sla report: clocks: %w", err)
	}
	defer rows.Close()

	out := make([]SlaReportClockRow, 0)
	for rows.Next() {
		var (
			row          SlaReportClockRow
			schedule     *string
			startOn      *time.Time
			pauseSeconds *float64
		)
		if err := rows.Scan(
			&row.CaseID, &row.CaseNumber, &row.IssueType, &row.Severity, &row.State, &row.Opened,
			&row.Target, &row.Stage, &row.HasBreached, &row.ElapsedSeconds, &row.TargetSeconds,
			&schedule, &startOn, &pauseSeconds,
		); err != nil {
			return nil, fmt.Errorf("sla report: scan clock: %w", err)
		}
		correctSlaReportBusinessDay(&row, schedule, startOn, pauseSeconds)
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sla report: iterate clocks: %w", err)
	}
	return out, nil
}

// correctSlaReportBusinessDay applies the same business-day correction every
// other SLA read applies (businessDayCorrectedLiveState), so a running
// 'WSO2 Business days' clock shows the same elapsed time, stage and breach
// here as on the case SLA tab, whatever version of sla_live the database has.
// Finished, paused and cancelled clocks keep their stored figures.
func correctSlaReportBusinessDay(row *SlaReportClockRow, schedule *string, startOn *time.Time, pauseSeconds *float64) {
	if row.Stage == nil || schedule == nil || row.TargetSeconds == nil {
		return
	}
	pause := 0.0
	if pauseSeconds != nil {
		pause = *pauseSeconds
	}
	stage, breached, _, ok := businessDayCorrectedLiveState(*schedule, *row.Stage, startOn, pause, *row.TargetSeconds)
	if !ok {
		return
	}
	target := time.Duration(*row.TargetSeconds * float64(time.Second))
	elapsed, _, _, _ := businessDayElapsedSnapshot(*startOn, time.Now().UTC(), time.Duration(pause*float64(time.Second)), target)
	seconds := elapsed.Seconds()
	row.Stage = &stage
	row.HasBreached = &breached
	row.ElapsedSeconds = &seconds
}
