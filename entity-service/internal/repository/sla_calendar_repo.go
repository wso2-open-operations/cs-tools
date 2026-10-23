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

// SLACalendarRepository is sla_calendar/sla_calendar_holiday (migration
// 0246). Exactly two rows exist (seeded by that migration) -- this
// repository never creates or retires a calendar, only edits the two seeded
// ones' working week and holiday list (see this change's own plan file,
// "Calendars are two seeded rows").
type SLACalendarRepository interface {
	List(ctx context.Context) ([]domain.SLACalendar, error)
	Get(ctx context.Context, id string) (domain.SLACalendar, error)
	UpdateWorkingWeek(ctx context.Context, id string, workingWeek []domain.SLAWorkingDay, actorEmail string) (domain.SLACalendar, error)
	AddHoliday(ctx context.Context, calendarID string, req domain.AddSLAHolidayRequest, actorEmail string) (domain.SLACalendar, error)
	DeleteHoliday(ctx context.Context, calendarID, holidayID string) error
}

type slaCalendarRepository struct{ db *pgxpool.Pool }

// NewSLACalendarRepository constructs an SLACalendarRepository.
func NewSLACalendarRepository(db *pgxpool.Pool) SLACalendarRepository {
	return &slaCalendarRepository{db: db}
}

func (r *slaCalendarRepository) holidaysOf(ctx context.Context, calendarID string) ([]domain.SLAHoliday, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, start_date::text, end_date::text, name
		  FROM sla_calendar_holiday WHERE calendar_id = $1::uuid ORDER BY start_date`, calendarID)
	if err != nil {
		return nil, fmt.Errorf("list sla calendar holidays: %w", err)
	}
	defer rows.Close()
	out := []domain.SLAHoliday{}
	for rows.Next() {
		var h domain.SLAHoliday
		if err := rows.Scan(&h.ID, &h.StartDate, &h.EndDate, &h.Name); err != nil {
			return nil, fmt.Errorf("scan sla calendar holiday: %w", err)
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func (r *slaCalendarRepository) scanCalendar(row pgx.Row) (domain.SLACalendar, error) {
	var c domain.SLACalendar
	var workingWeekRaw []byte
	if err := row.Scan(&c.ID, &c.Name, &c.IsRetired, &workingWeekRaw, &c.CreatedOn, &c.UpdatedOn); err != nil {
		return domain.SLACalendar{}, err
	}
	if err := json.Unmarshal(workingWeekRaw, &c.WorkingWeek); err != nil {
		return domain.SLACalendar{}, fmt.Errorf("decode sla_calendar.working_week: %w", err)
	}
	return c, nil
}

const slaCalendarSelect = `SELECT id::text, name, is_retired, working_week, created_on, updated_on FROM sla_calendar`

func (r *slaCalendarRepository) List(ctx context.Context) ([]domain.SLACalendar, error) {
	rows, err := r.db.Query(ctx, slaCalendarSelect+` ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list sla calendars: %w", err)
	}
	defer rows.Close()
	out := []domain.SLACalendar{}
	for rows.Next() {
		c, err := r.scanCalendar(rows)
		if err != nil {
			return nil, fmt.Errorf("scan sla calendar: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		holidays, err := r.holidaysOf(ctx, out[i].ID)
		if err != nil {
			return nil, err
		}
		out[i].Holidays = holidays
	}
	return out, nil
}

func (r *slaCalendarRepository) Get(ctx context.Context, id string) (domain.SLACalendar, error) {
	c, err := r.scanCalendar(r.db.QueryRow(ctx, slaCalendarSelect+` WHERE id = $1::uuid`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.SLACalendar{}, &apierror.NotFoundError{Msg: "sla calendar not found"}
	}
	if err != nil {
		return domain.SLACalendar{}, fmt.Errorf("get sla calendar: %w", err)
	}
	holidays, err := r.holidaysOf(ctx, c.ID)
	if err != nil {
		return domain.SLACalendar{}, err
	}
	c.Holidays = holidays
	return c, nil
}

func (r *slaCalendarRepository) UpdateWorkingWeek(ctx context.Context, id string, workingWeek []domain.SLAWorkingDay, actorEmail string) (domain.SLACalendar, error) {
	weekJSON, err := json.Marshal(workingWeek)
	if err != nil {
		return domain.SLACalendar{}, fmt.Errorf("encode sla calendar working_week: %w", err)
	}
	c, err := r.scanCalendar(r.db.QueryRow(ctx, `
		UPDATE sla_calendar
		   SET working_week = $2::jsonb, updated_on = now(), updated_by = $3
		 WHERE id = $1::uuid
		RETURNING id::text, name, is_retired, working_week, created_on, updated_on`,
		id, weekJSON, actorEmail))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.SLACalendar{}, &apierror.NotFoundError{Msg: "sla calendar not found"}
	}
	if err != nil {
		return domain.SLACalendar{}, fmt.Errorf("update sla calendar working_week: %w", err)
	}
	holidays, err := r.holidaysOf(ctx, c.ID)
	if err != nil {
		return domain.SLACalendar{}, err
	}
	c.Holidays = holidays
	return c, nil
}

func (r *slaCalendarRepository) AddHoliday(ctx context.Context, calendarID string, req domain.AddSLAHolidayRequest, actorEmail string) (domain.SLACalendar, error) {
	var exists bool
	if err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM sla_calendar WHERE id = $1::uuid)`, calendarID).Scan(&exists); err != nil {
		return domain.SLACalendar{}, fmt.Errorf("check sla calendar exists: %w", err)
	}
	if !exists {
		return domain.SLACalendar{}, &apierror.NotFoundError{Msg: "sla calendar not found"}
	}
	if _, err := r.db.Exec(ctx, `
		INSERT INTO sla_calendar_holiday (calendar_id, start_date, end_date, name, created_by, updated_by)
		VALUES ($1::uuid, $2::date, $3::date, $4, $5, $5)`,
		calendarID, req.StartDate, req.EndDate, req.Name, actorEmail); err != nil {
		return domain.SLACalendar{}, fmt.Errorf("add sla calendar holiday: %w", err)
	}
	return r.Get(ctx, calendarID)
}

func (r *slaCalendarRepository) DeleteHoliday(ctx context.Context, calendarID, holidayID string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM sla_calendar_holiday WHERE id = $1::uuid AND calendar_id = $2::uuid`, holidayID, calendarID)
	if err != nil {
		return fmt.Errorf("delete sla calendar holiday: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return &apierror.NotFoundError{Msg: "sla calendar holiday not found"}
	}
	return nil
}
