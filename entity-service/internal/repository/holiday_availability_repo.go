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

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// HolidayAvailabilityRepository is working on a public holiday, CRE only
// (migration 0283): the Public holiday tags the Team Schedule places, the
// teams' confirmations, and the reminders sent.
type HolidayAvailabilityRepository interface {
	// CREMembers is every member of a rostered CRE team, management left
	// out, with the CRE teams they are on (the first is the one a tag is
	// filed under) and their stored location.
	CREMembers(ctx context.Context) ([]domain.SchedulePerson, error)
	// PlaceHolidayTag places one Public holiday tag. false, nil: not
	// placed, because one was placed for that person and day before (and
	// perhaps removed since) or they already have a tag that day.
	PlaceHolidayTag(ctx context.Context, p domain.HolidayTagPlacement) (bool, error)
	// RemoveCancelledHolidayTags takes away, from onOrAfter on, the tags of
	// holidays the calendar no longer has, when nobody has touched them, and
	// forgets every placement of such a holiday.
	RemoveCancelledHolidayTags(ctx context.Context, onOrAfter string) (int, error)
	// PlacedBetween is every placement in [from, to], as it stands now.
	PlacedBetween(ctx context.Context, from, to string, teamKeys []string) ([]domain.HolidayPlacedRow, error)
	// TurnsBetween is every rota turn of the teams on those days.
	TurnsBetween(ctx context.Context, from, to string, teamKeys []string) ([]domain.HolidayTurn, error)
	ConfirmationsBetween(ctx context.Context, from, to string, teamKeys []string) ([]domain.HolidayConfirmationRow, error)
	Confirm(ctx context.Context, teamKey, date, by string) (domain.HolidayConfirmationRow, error)
	// Withdraw removes a confirmation; false when there was none.
	Withdraw(ctx context.Context, teamKey, date string) (bool, error)
	// ReminderRecipients is the team's leads and its rota managers.
	ReminderRecipients(ctx context.Context, teamKey string) ([]domain.HolidayReminderRecipient, error)
	// SentStages is the reminder stages already sent, by team key
	// (lower-cased) and date.
	SentStages(ctx context.Context, from, to string) (map[string][]string, error)
	RecordReminder(ctx context.Context, teamKey, date, stage string) error
}

type holidayAvailabilityRepository struct{ db *pgxpool.Pool }

// NewHolidayAvailabilityRepository constructs a HolidayAvailabilityRepository.
func NewHolidayAvailabilityRepository(db *pgxpool.Pool) HolidayAvailabilityRepository {
	return &holidayAvailabilityRepository{db: db}
}

// HolidayStageKey is SentStages' map key.
func HolidayStageKey(teamKey, date string) string {
	return strings.ToLower(teamKey) + "|" + date
}

// CREMembers returns the active members of rostered CRE teams, management left
// out, with their team keys and stored location.
func (r *holidayAvailabilityRepository) CREMembers(ctx context.Context) ([]domain.SchedulePerson, error) {
	rows, err := r.db.Query(ctx, `
		SELECT u.id::text,
		       COALESCE(u.email, ''),
		       array_agg(t.key ORDER BY (lower(t.type) LIKE '%abt') DESC, t.key),
		       COALESCE(MAX(pl.location), '')
		  FROM team_member tm
		  JOIN team t    ON t.id = tm.team_id
		  JOIN "user" u  ON u.id = tm.user_id
		  LEFT JOIN team_schedule_person_location pl ON pl.user_id = u.id
		 WHERE t.key IS NOT NULL
		   AND `+rosteredTeamWhere+`
		   AND `+teamFamilyExpr+` = 'CRE'
		   AND u.is_active IS DISTINCT FROM FALSE
		   AND `+fmt.Sprintf(notManagement, "u.id")+`
		 GROUP BY u.id, u.email
		 ORDER BY u.id`)
	if err != nil {
		return nil, fmt.Errorf("query CRE members: %w", err)
	}
	defer rows.Close()
	var out []domain.SchedulePerson
	for rows.Next() {
		var p domain.SchedulePerson
		if err := rows.Scan(&p.UserID, &p.Email, &p.TeamKeys, &p.Location); err != nil {
			return nil, fmt.Errorf("scan CRE member: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// PlaceHolidayTag places the public holiday tag on a person for a day and
// records the placement. It returns false and places nothing when the tag was
// placed before or the person already has any tag that day.
func (r *holidayAvailabilityRepository) PlaceHolidayTag(ctx context.Context, p domain.HolidayTagPlacement) (bool, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin holiday tag: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := nameTheActor(ctx, tx, domain.HolidayTagActor); err != nil {
		return false, err
	}
	// One person at a time: two replicas placing the same day must not both
	// pass the checks below.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('team-schedule-holiday-tag:' || $1))`, p.UserID); err != nil {
		return false, fmt.Errorf("lock holiday tag: %w", err)
	}
	var placedBefore, tagged bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM team_schedule_holiday_tag_placed
		                WHERE user_id = $1::uuid AND holiday_date = $2::date),
		       EXISTS (SELECT 1 FROM team_schedule_absence
		                WHERE user_id = $1::uuid
		                  AND daterange(starts_on, ends_on, '[]') @> $2::date)`,
		p.UserID, p.Date).Scan(&placedBefore, &tagged); err != nil {
		return false, fmt.Errorf("check holiday tag: %w", err)
	}
	// Placed before: a tag a lead removed stays removed. Tagged: leave (or
	// any other tag) wins, and two tags cannot share a day.
	if placedBefore || tagged {
		return false, nil
	}
	var kindID string
	if err := tx.QueryRow(ctx,
		`SELECT id::text FROM team_schedule_absence_kind WHERE code = $1 AND is_active`,
		domain.HolidayTagKindCode).Scan(&kindID); err != nil {
		return false, fmt.Errorf("the %s tag is missing or retired (migration 0283): %w", domain.HolidayTagKindCode, err)
	}
	note := fmt.Sprintf("%s — public holiday in %s", p.Title, p.Country)
	var id string
	if err := tx.QueryRow(ctx, `
		INSERT INTO team_schedule_absence
		  (id, created_on, updated_on, created_by, updated_by, user_id, team_key, kind_id, starts_on, ends_on, note)
		VALUES (gen_random_uuid(), now(), now(), $1, $1, $2::uuid, $3, $4::uuid, $5::date, $5::date, $6)
		RETURNING id::text`,
		domain.HolidayTagActor, p.UserID, p.TeamKey, kindID, p.Date, note).Scan(&id); err != nil {
		return false, fmt.Errorf("insert holiday tag: %w", err)
	}
	if err := recordAbsenceActivity(ctx, tx, id, p.UserID, p.TeamKey, domain.HolidayTagKindCode,
		p.Date, &p.Date, "CREATED", domain.HolidayTagActor, nil, nil, nil, &note); err != nil {
		return false, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO team_schedule_holiday_tag_placed (user_id, holiday_date, country, team_key, absence_id, title)
		VALUES ($1::uuid, $2::date, $3, $4, $5::uuid, $6)`,
		p.UserID, p.Date, p.Country, p.TeamKey, id, p.Title); err != nil {
		return false, fmt.Errorf("record holiday tag: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit holiday tag: %w", err)
	}
	return true, nil
}

// RemoveCancelledHolidayTags deletes the untouched holiday tags, from the given
// date on, whose holiday is no longer stored, forgets those placements, and
// returns how many tags it removed.
func (r *holidayAvailabilityRepository) RemoveCancelledHolidayTags(ctx context.Context, onOrAfter string) (int, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin holiday tag cleanup: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := nameTheActor(ctx, tx, domain.HolidayTagActor); err != nil {
		return 0, err
	}
	// The tags still exactly as the Team Schedule placed them: one day,
	// never edited by anybody. A tag somebody changed is theirs now.
	rows, err := tx.Query(ctx, `
		SELECT a.id::text, a.user_id::text, a.team_key, to_char(a.starts_on, 'YYYY-MM-DD')
		  FROM team_schedule_holiday_tag_placed p
		  JOIN team_schedule_absence a ON a.id = p.absence_id
		 WHERE p.holiday_date >= $1::date
		   AND NOT EXISTS (SELECT 1 FROM team_schedule_public_holiday h
		                    WHERE h.country = p.country AND h.holiday_date = p.holiday_date)
		   AND a.created_by = $2 AND a.updated_by = $2
		   AND a.starts_on = p.holiday_date AND a.ends_on = p.holiday_date`,
		onOrAfter, domain.HolidayTagActor)
	if err != nil {
		return 0, fmt.Errorf("read cancelled holiday tags: %w", err)
	}
	type gone struct{ id, userID, teamKey, day string }
	var tags []gone
	for rows.Next() {
		var g gone
		if err := rows.Scan(&g.id, &g.userID, &g.teamKey, &g.day); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scan cancelled holiday tag: %w", err)
		}
		tags = append(tags, g)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("read cancelled holiday tags: %w", err)
	}
	note := "The public holiday is no longer in the calendar."
	for _, g := range tags {
		if err := recordAbsenceActivity(ctx, tx, g.id, g.userID, g.teamKey, domain.HolidayTagKindCode,
			g.day, &g.day, "DELETED", domain.HolidayTagActor, nil, nil, nil, &note); err != nil {
			return 0, err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM team_schedule_absence WHERE id = $1::uuid`, g.id); err != nil {
			return 0, fmt.Errorf("remove cancelled holiday tag: %w", err)
		}
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM team_schedule_holiday_tag_placed p
		 WHERE p.holiday_date >= $1::date
		   AND NOT EXISTS (SELECT 1 FROM team_schedule_public_holiday h
		                    WHERE h.country = p.country AND h.holiday_date = p.holiday_date)`,
		onOrAfter); err != nil {
		return 0, fmt.Errorf("forget cancelled holidays: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit holiday tag cleanup: %w", err)
	}
	return len(tags), nil
}

// PlacedBetween returns the holiday tags placed between two dates, for the
// given teams or every team when none are given, with any other tag that day.
func (r *holidayAvailabilityRepository) PlacedBetween(ctx context.Context, from, to string, teamKeys []string) ([]domain.HolidayPlacedRow, error) {
	rows, err := r.db.Query(ctx, `
		SELECT p.user_id::text, `+engineerName+`, COALESCE(u.email, ''),
		       p.team_key, COALESCE(NULLIF(t.name, ''), p.team_key),
		       to_char(p.holiday_date, 'YYYY-MM-DD'), p.country, p.title,
		       COALESCE((SELECT a.id::text FROM team_schedule_absence a
		                  WHERE a.id = p.absence_id
		                    AND daterange(a.starts_on, a.ends_on, '[]') @> p.holiday_date), ''),
		       COALESCE((SELECT k.code FROM team_schedule_absence a
		                   JOIN team_schedule_absence_kind k ON k.id = a.kind_id
		                  WHERE a.user_id = p.user_id
		                    AND a.id IS DISTINCT FROM p.absence_id
		                    AND daterange(a.starts_on, a.ends_on, '[]') @> p.holiday_date
		                    AND k.moves_to_team_key IS NULL
		                  ORDER BY a.starts_on LIMIT 1), '')
		  FROM team_schedule_holiday_tag_placed p
		  JOIN "user" u ON u.id = p.user_id
		  LEFT JOIN team t ON lower(t.key) = lower(p.team_key)
		 WHERE p.holiday_date BETWEEN $1::date AND $2::date
		   AND (cardinality($3::text[]) = 0 OR lower(p.team_key) = ANY($3::text[]))
		 ORDER BY p.holiday_date, lower(p.team_key), 2`,
		from, to, lowerKeys(teamKeys))
	if err != nil {
		return nil, fmt.Errorf("query placed holiday tags: %w", err)
	}
	defer rows.Close()
	var out []domain.HolidayPlacedRow
	for rows.Next() {
		var p domain.HolidayPlacedRow
		if err := rows.Scan(&p.UserID, &p.Name, &p.Email, &p.TeamKey, &p.TeamName,
			&p.Date, &p.Country, &p.Title, &p.AbsenceID, &p.OtherKind); err != nil {
			return nil, fmt.Errorf("scan placed holiday tag: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// TurnsBetween returns the rota turns the given teams hold between two dates,
// each marked with whether its person is away that day.
func (r *holidayAvailabilityRepository) TurnsBetween(ctx context.Context, from, to string, teamKeys []string) ([]domain.HolidayTurn, error) {
	if len(teamKeys) == 0 {
		return nil, nil
	}
	rows, err := r.db.Query(ctx, `
		SELECT a.team_key, to_char(a.rota_date, 'YYYY-MM-DD'), s.code, s.label,
		       a.user_id::text, `+engineerName+`,
		       `+absenceOn("EXISTS", "a.rota_date")+`
		  FROM team_schedule_assignment a
		  JOIN team_schedule_shift s ON s.id = a.shift_id
		  JOIN "user" u ON u.id = a.user_id
		 WHERE a.is_rotation
		   AND a.rota_date BETWEEN $1::date AND $2::date
		   AND lower(a.team_key) = ANY($3::text[])
		 ORDER BY a.rota_date, lower(a.team_key), a.starts_at, s.code`,
		from, to, lowerKeys(teamKeys))
	if err != nil {
		return nil, fmt.Errorf("query holiday turns: %w", err)
	}
	defer rows.Close()
	var out []domain.HolidayTurn
	for rows.Next() {
		var t domain.HolidayTurn
		if err := rows.Scan(&t.TeamKey, &t.Date, &t.ShiftCode, &t.ShiftLabel, &t.UserID, &t.Name, &t.Away); err != nil {
			return nil, fmt.Errorf("scan holiday turn: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ConfirmationsBetween returns the holiday confirmations between two dates,
// for the given teams or for every team when none are given.
func (r *holidayAvailabilityRepository) ConfirmationsBetween(ctx context.Context, from, to string, teamKeys []string) ([]domain.HolidayConfirmationRow, error) {
	rows, err := r.db.Query(ctx, `
		SELECT team_key, to_char(holiday_date, 'YYYY-MM-DD'), confirmed_by, confirmed_on
		  FROM team_schedule_holiday_confirmation
		 WHERE holiday_date BETWEEN $1::date AND $2::date
		   AND (cardinality($3::text[]) = 0 OR lower(team_key) = ANY($3::text[]))`,
		from, to, lowerKeys(teamKeys))
	if err != nil {
		return nil, fmt.Errorf("query holiday confirmations: %w", err)
	}
	defer rows.Close()
	var out []domain.HolidayConfirmationRow
	for rows.Next() {
		var c domain.HolidayConfirmationRow
		if err := rows.Scan(&c.TeamKey, &c.Date, &c.ConfirmedBy, &c.ConfirmedOn); err != nil {
			return nil, fmt.Errorf("scan holiday confirmation: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Confirm records, or refreshes, a team's confirmation for a holiday date by
// the given person and returns it.
func (r *holidayAvailabilityRepository) Confirm(ctx context.Context, teamKey, date, by string) (domain.HolidayConfirmationRow, error) {
	c := domain.HolidayConfirmationRow{TeamKey: teamKey, Date: date, ConfirmedBy: by}
	err := r.db.QueryRow(ctx, `
		INSERT INTO team_schedule_holiday_confirmation (team_key, holiday_date, confirmed_by, confirmed_on)
		VALUES ($1, $2::date, $3, now())
		ON CONFLICT (team_key, holiday_date) DO UPDATE
		   SET confirmed_by = EXCLUDED.confirmed_by, confirmed_on = EXCLUDED.confirmed_on
		RETURNING confirmed_on`, teamKey, date, by).Scan(&c.ConfirmedOn)
	if err != nil {
		return c, fmt.Errorf("record holiday confirmation: %w", err)
	}
	return c, nil
}

// Withdraw removes a team's confirmation for a holiday date and reports
// whether there was one to remove.
func (r *holidayAvailabilityRepository) Withdraw(ctx context.Context, teamKey, date string) (bool, error) {
	tag, err := r.db.Exec(ctx, `
		DELETE FROM team_schedule_holiday_confirmation
		 WHERE lower(team_key) = lower($1) AND holiday_date = $2::date`, teamKey, date)
	if err != nil {
		return false, fmt.Errorf("withdraw holiday confirmation: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// ReminderRecipients returns the team's active internal leads and rota
// managers who have an email, one entry per person, a lead taking precedence.
func (r *holidayAvailabilityRepository) ReminderRecipients(ctx context.Context, teamKey string) ([]domain.HolidayReminderRecipient, error) {
	// Leads by their position on the team; rota managers by their grant,
	// counted only while they are still on the team and internal (as
	// RotaMarkerTeamsFor counts them).
	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT ON (lower(u.email)) `+engineerName+`, u.email, x.role
		  FROM (
		        SELECT tm.user_id, 'lead' AS role, 0 AS rank
		          FROM team_member tm JOIN team t ON t.id = tm.team_id
		         WHERE lower(t.key) = lower($1) AND tm.role IN ('lead', 'americas_team_lead')
		        UNION ALL
		        SELECT m.user_id, 'rota_manager', 1
		          FROM team_schedule_rota_marker m
		          JOIN team t ON lower(t.key) = lower(m.team_key)
		         WHERE lower(m.team_key) = lower($1)
		           AND EXISTS (SELECT 1 FROM team_member tm WHERE tm.team_id = t.id AND tm.user_id = m.user_id)
		       ) x
		  JOIN "user" u ON u.id = x.user_id
		 WHERE COALESCE(u.email, '') <> ''
		   AND u.is_active IS DISTINCT FROM FALSE
		   AND u.user_type = 'INTERNAL'::user_type_enum
		 ORDER BY lower(u.email), x.rank`, teamKey)
	if err != nil {
		return nil, fmt.Errorf("query holiday reminder recipients: %w", err)
	}
	defer rows.Close()
	out := []domain.HolidayReminderRecipient{}
	for rows.Next() {
		var p domain.HolidayReminderRecipient
		if err := rows.Scan(&p.Name, &p.Email, &p.Role); err != nil {
			return nil, fmt.Errorf("scan holiday reminder recipient: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// SentStages returns the reminder stages already sent between two dates, keyed
// by HolidayStageKey of the team and date.
func (r *holidayAvailabilityRepository) SentStages(ctx context.Context, from, to string) (map[string][]string, error) {
	rows, err := r.db.Query(ctx, `
		SELECT team_key, to_char(holiday_date, 'YYYY-MM-DD'), stage
		  FROM team_schedule_holiday_reminder_sent
		 WHERE holiday_date BETWEEN $1::date AND $2::date`, from, to)
	if err != nil {
		return nil, fmt.Errorf("query holiday reminders sent: %w", err)
	}
	defer rows.Close()
	out := map[string][]string{}
	for rows.Next() {
		var team, day, stage string
		if err := rows.Scan(&team, &day, &stage); err != nil {
			return nil, fmt.Errorf("scan holiday reminder sent: %w", err)
		}
		k := HolidayStageKey(team, day)
		out[k] = append(out[k], stage)
	}
	return out, rows.Err()
}

// RecordReminder records that a reminder stage was sent for a team and holiday
// date; recording the same stage again changes nothing.
func (r *holidayAvailabilityRepository) RecordReminder(ctx context.Context, teamKey, date, stage string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO team_schedule_holiday_reminder_sent (team_key, holiday_date, stage, sent_on)
		VALUES (lower($1), $2::date, $3, now())
		ON CONFLICT (team_key, holiday_date, stage) DO NOTHING`, teamKey, date, stage)
	if err != nil {
		return fmt.Errorf("record holiday reminder: %w", err)
	}
	return nil
}
