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
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// PublicHolidayRepository is the Team Schedule's local copy of public
// holidays (migration 0283) and the members' locations.
type PublicHolidayRepository interface {
	// SchedulePeople is every Team Schedule member (a member of a team with
	// a key) with their team keys and stored location; userIDs narrows it
	// when non-empty.
	SchedulePeople(ctx context.Context, userIDs []string) ([]domain.SchedulePerson, error)
	// IndiaTeamKey is the team the India team rotation (absence kind
	// ALLO_IND) moves people to, "" when there is none.
	IndiaTeamKey(ctx context.Context) (string, error)
	UpsertPersonLocation(ctx context.Context, userID, location string, at time.Time) error
	// ReplaceCountryRange makes country's stored holidays in [from, to] the
	// given set, in one transaction, and returns how many days it stored.
	ReplaceCountryRange(ctx context.Context, country string, from, to time.Time, days []domain.PublicHolidayDay, at time.Time) (int, error)
	HolidaysBetween(ctx context.Context, countries []string, from, to time.Time) ([]domain.PublicHolidayDay, error)
}

type publicHolidayRepository struct{ db *pgxpool.Pool }

// NewPublicHolidayRepository constructs a PublicHolidayRepository.
func NewPublicHolidayRepository(db *pgxpool.Pool) PublicHolidayRepository {
	return &publicHolidayRepository{db: db}
}

const schedulePeopleSQL = `
SELECT u.id::text,
       COALESCE(u.email, ''),
       array_agg(DISTINCT t.key ORDER BY t.key),
       COALESCE(MAX(pl.location), '')
  FROM team_member tm
  JOIN team t ON t.id = tm.team_id
  JOIN "user" u ON u.id = tm.user_id
  LEFT JOIN team_schedule_person_location pl ON pl.user_id = u.id
 WHERE t.key IS NOT NULL
   AND (cardinality($1::uuid[]) = 0 OR u.id = ANY($1::uuid[]))
 GROUP BY u.id, u.email
 ORDER BY u.id`

func (r *publicHolidayRepository) SchedulePeople(ctx context.Context, userIDs []string) ([]domain.SchedulePerson, error) {
	if userIDs == nil {
		userIDs = []string{}
	}
	rows, err := r.db.Query(ctx, schedulePeopleSQL, userIDs)
	if err != nil {
		return nil, fmt.Errorf("query schedule people: %w", err)
	}
	defer rows.Close()
	var out []domain.SchedulePerson
	for rows.Next() {
		var p domain.SchedulePerson
		if err := rows.Scan(&p.UserID, &p.Email, &p.TeamKeys, &p.Location); err != nil {
			return nil, fmt.Errorf("scan schedule person: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *publicHolidayRepository) IndiaTeamKey(ctx context.Context) (string, error) {
	var key *string
	err := r.db.QueryRow(ctx,
		`SELECT moves_to_team_key FROM team_schedule_absence_kind WHERE code = 'ALLO_IND'`).Scan(&key)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("query india team key: %w", err)
	}
	if key == nil {
		return "", nil
	}
	return *key, nil
}

func (r *publicHolidayRepository) UpsertPersonLocation(ctx context.Context, userID, location string, at time.Time) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO team_schedule_person_location (user_id, location, fetched_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id) DO UPDATE SET location = EXCLUDED.location, fetched_at = EXCLUDED.fetched_at`,
		userID, location, at)
	if err != nil {
		return fmt.Errorf("upsert person location: %w", err)
	}
	return nil
}

// mergeHolidayDays folds several holidays on one day into one row, titles
// joined in a stable order.
func mergeHolidayDays(days []domain.PublicHolidayDay) (dates, titles []string) {
	byDate := map[string][]string{}
	for _, d := range days {
		title := strings.TrimSpace(d.Title)
		seen := false
		for _, t := range byDate[d.Date] {
			if t == title {
				seen = true
				break
			}
		}
		if !seen {
			byDate[d.Date] = append(byDate[d.Date], title)
		}
	}
	for date := range byDate {
		dates = append(dates, date)
	}
	sort.Strings(dates)
	for _, date := range dates {
		titles = append(titles, strings.Join(byDate[date], " / "))
	}
	return dates, titles
}

func (r *publicHolidayRepository) ReplaceCountryRange(ctx context.Context, country string, from, to time.Time, days []domain.PublicHolidayDay, at time.Time) (int, error) {
	dates, titles := mergeHolidayDays(days)
	if dates == nil {
		dates, titles = []string{}, []string{}
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin holiday replace: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `
		DELETE FROM team_schedule_public_holiday
		 WHERE country = $1 AND holiday_date BETWEEN $2::date AND $3::date
		   AND NOT (holiday_date = ANY($4::date[]))`,
		country, from.Format("2006-01-02"), to.Format("2006-01-02"), dates); err != nil {
		return 0, fmt.Errorf("delete stale holidays: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO team_schedule_public_holiday (country, holiday_date, title, fetched_at)
		SELECT $1, d, t, $4
		  FROM unnest($2::date[], $3::text[]) AS x(d, t)
		 WHERE d BETWEEN $5::date AND $6::date
		ON CONFLICT (country, holiday_date) DO UPDATE
		   SET title = EXCLUDED.title, fetched_at = EXCLUDED.fetched_at`,
		country, dates, titles, at, from.Format("2006-01-02"), to.Format("2006-01-02")); err != nil {
		return 0, fmt.Errorf("upsert holidays: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit holiday replace: %w", err)
	}
	stored := 0
	for _, d := range dates {
		if d >= from.Format("2006-01-02") && d <= to.Format("2006-01-02") {
			stored++
		}
	}
	return stored, nil
}

func (r *publicHolidayRepository) HolidaysBetween(ctx context.Context, countries []string, from, to time.Time) ([]domain.PublicHolidayDay, error) {
	if len(countries) == 0 {
		return nil, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT country, to_char(holiday_date, 'YYYY-MM-DD'), title
		  FROM team_schedule_public_holiday
		 WHERE country = ANY($1::text[])
		   AND holiday_date BETWEEN $2::date AND $3::date
		 ORDER BY holiday_date, country`,
		countries, from.Format("2006-01-02"), to.Format("2006-01-02"))
	if err != nil {
		return nil, fmt.Errorf("query holidays: %w", err)
	}
	defer rows.Close()
	var out []domain.PublicHolidayDay
	for rows.Next() {
		var d domain.PublicHolidayDay
		if err := rows.Scan(&d.Country, &d.Date, &d.Title); err != nil {
			return nil, fmt.Errorf("scan holiday: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
