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
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CaseAlertSettingsRepository reads and writes the Case Alerts settings
// (migration 0233): chat rooms, their sealed webhook URLs, the ServiceNow group
// routes, the other ladder values, and the revision row.
//
// Every write names the actor for the audit trigger in the same transaction
// (nameTheActor), as the rota tables do. The sealed URL is stored and read
// here as bytes only; sealing and opening is the service's.
type CaseAlertSettingsRepository interface {
	Revision(ctx context.Context) (CaseAlertRevisionRow, error)
	RecordApplied(ctx context.Context, by string, revision int64, mode string, summary map[string]any) error

	ListRooms(ctx context.Context) ([]CaseAlertRoomRow, error)
	// GetRoom is one room, or nil when there is none with that id.
	GetRoom(ctx context.Context, id string) (*CaseAlertRoomRow, error)
	// SaveRoom creates the room of w's scope (ladder, team) or changes it.
	SaveRoom(ctx context.Context, actor string, w CaseAlertRoomWrite) (CaseAlertRoomRow, error)
	// DeleteRoom removes a room and its URL. False when there was none.
	DeleteRoom(ctx context.Context, actor, id string) (bool, error)
	// SetSecret stores a room's sealed URL and its hint. Replacing the URL
	// clears the last test, which was of the old one. False when the room is
	// gone.
	SetSecret(ctx context.Context, actor, roomID string, s CaseAlertSecret, hint string) (bool, error)
	// Secret is a room's stored URL, or nil when none is stored.
	Secret(ctx context.Context, roomID string) (*CaseAlertSecret, error)
	// Secrets is every stored URL, by room id.
	Secrets(ctx context.Context) (map[string]CaseAlertSecret, error)
	// ClaimTest records a test card as asked for at at, only when the last
	// one was at or before notBefore (or there was none). False means a test
	// was asked for since, or the room is gone.
	ClaimTest(ctx context.Context, actor, roomID string, at, notBefore time.Time) (bool, error)
	// RecordTest stores a test card's outcome.
	RecordTest(ctx context.Context, actor, roomID, status string, at time.Time) error

	ListRoutes(ctx context.Context) ([]CaseAlertRouteRow, error)
	// SaveRoute creates the route of w's group on w's ladder, or changes it.
	SaveRoute(ctx context.Context, actor string, w CaseAlertRouteWrite) (CaseAlertRouteRow, error)
	// DeleteRoute removes a route. False when there was none.
	DeleteRoute(ctx context.Context, actor, id string) (bool, error)

	// ListValues is every saved ladder value.
	ListValues(ctx context.Context) ([]CaseAlertValueRow, error)
	// GetValue is one saved ladder value, or nil when there is none with that
	// id.
	GetValue(ctx context.Context, id string) (*CaseAlertValueRow, error)
	// SaveValues creates or replaces values of one ladder (key -> JSON), all
	// in one transaction.
	SaveValues(ctx context.Context, actor, ladder string, values map[string][]byte) error
	// DeleteValue removes a saved value. False when there was none.
	DeleteValue(ctx context.Context, actor, id string) (bool, error)

	// RosteredTeamFamily is a Team Schedule team's family (CRE, SRE, SME);
	// false when the team is not one of the schedule's.
	RosteredTeamFamily(ctx context.Context, teamKey string) (string, bool, error)
	// ActiveRotaFamily is an active rota's family; false when there is no
	// active rota with that code.
	ActiveRotaFamily(ctx context.Context, code string) (string, bool, error)
	// RouteGroups are the CSM Portal teams a route can be for: every CRE, SRE
	// and SME team by its name, leadership teams left out.
	RouteGroups(ctx context.Context) ([]CaseAlertTeamRow, error)
	// RouteTeams are the Team Schedule's CRE and SRE teams a route can go to.
	RouteTeams(ctx context.Context) ([]CaseAlertTeamRow, error)
	// SRERotas are the active SRE rotas.
	SRERotas(ctx context.Context) ([]CaseAlertRotaRow, error)
	// TeamFamilyByName is the family (CRE, SRE, SME) of the team with this
	// name, matched ignoring case; false when no CRE, SRE or SME team has it.
	TeamFamilyByName(ctx context.Context, name string) (string, bool, error)
	// IsCRETeamName reports whether any team of a CRE type (leadership teams
	// included) has this name, matched ignoring case.
	IsCRETeamName(ctx context.Context, name string) (bool, error)
	// TeamTypeExists reports whether any team has this type (cre-abt).
	TeamTypeExists(ctx context.Context, teamType string) (bool, error)
	// ShiftCodeExists reports whether the Team Schedule has an active shift
	// with this code (CRE_WEEKEND_NIGHT).
	ShiftCodeExists(ctx context.Context, code string) (bool, error)
	// ActiveUserExists reports whether an active user has this email.
	ActiveUserExists(ctx context.Context, email string) (bool, error)
}

// CaseAlertRevisionRow is the single paging_config_revision row.
type CaseAlertRevisionRow struct {
	Revision        int64
	ChangedAt       *time.Time
	AppliedRevision *int64
	AppliedAt       *time.Time
	AppliedMode     *string
	// AppliedSummary is the summary the alert service reported, as JSON; nil
	// when there is none.
	AppliedSummary []byte
}

// CaseAlertRoomRow is one paging_chat_room row, with whether a URL is stored.
type CaseAlertRoomRow struct {
	ID             string
	Ladder         string
	TeamKey        string
	Label          string
	Enabled        bool
	Teamless       bool
	Channel        string
	HasSecret      bool
	WebhookHint    *string
	WebhookSetAt   *time.Time
	WebhookSetBy   *string
	LastTestAt     *time.Time
	LastTestStatus *string
	UpdatedBy      string
	UpdatedOn      time.Time
}

// CaseAlertRoomWrite is a room as saved. TeamKey "" is the ladder's shared
// room.
type CaseAlertRoomWrite struct {
	Ladder   string
	TeamKey  string
	Label    string
	Enabled  bool
	Teamless bool
	// Channel is "" for the ladder's.
	Channel string
}

// CaseAlertSecret is a room's webhook URL: as saved (URL), or -- a row saved
// before 0237 -- sealed under CASE_ALERTS_SECRET_KEY (KeyID, Nonce,
// Ciphertext), which only a deployment holding that key can open.
type CaseAlertSecret struct {
	URL        string
	KeyID      string
	Nonce      []byte
	Ciphertext []byte
}

// CaseAlertRouteRow is one paging_group_route row.
type CaseAlertRouteRow struct {
	ID        string
	Ladder    string
	GroupName string
	TeamKey   string
	RotaCode  string
	Enabled   bool
	UpdatedBy string
	UpdatedOn time.Time
}

// CaseAlertValueRow is one paging_ladder_value row.
type CaseAlertValueRow struct {
	ID        string
	Ladder    string
	Key       string
	Value     []byte
	UpdatedBy string
	UpdatedOn time.Time
}

// CaseAlertTeamRow is a team as a route option.
type CaseAlertTeamRow struct {
	Key, Name, Family string
}

// CaseAlertRotaRow is an active SRE rota as a route option.
type CaseAlertRotaRow struct {
	Code, Label string
}

// CaseAlertRouteWrite is a route as saved; exactly one of TeamKey and RotaCode
// is set.
type CaseAlertRouteWrite struct {
	Ladder    string
	GroupName string
	TeamKey   string
	RotaCode  string
	Enabled   bool
}

type caseAlertSettingsRepository struct{ db *pgxpool.Pool }

// NewCaseAlertSettingsRepository constructs a CaseAlertSettingsRepository over
// the pool.
func NewCaseAlertSettingsRepository(db *pgxpool.Pool) CaseAlertSettingsRepository {
	return &caseAlertSettingsRepository{db: db}
}

func (r *caseAlertSettingsRepository) inTx(ctx context.Context, actor string, fn func(pgx.Tx) error) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin case alert settings change: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := nameTheActor(ctx, tx, actor); err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit case alert settings change: %w", err)
	}
	return nil
}

func (r *caseAlertSettingsRepository) Revision(ctx context.Context) (CaseAlertRevisionRow, error) {
	var v CaseAlertRevisionRow
	err := r.db.QueryRow(ctx, `
		SELECT revision, changed_at, applied_revision, applied_at, applied_mode, applied_summary
		  FROM paging_config_revision WHERE id = 1`).
		Scan(&v.Revision, &v.ChangedAt, &v.AppliedRevision, &v.AppliedAt, &v.AppliedMode, &v.AppliedSummary)
	if errors.Is(err, pgx.ErrNoRows) {
		return CaseAlertRevisionRow{}, nil
	}
	if err != nil {
		return CaseAlertRevisionRow{}, fmt.Errorf("case alert settings revision: %w", err)
	}
	return v, nil
}

func (r *caseAlertSettingsRepository) RecordApplied(ctx context.Context, by string, revision int64, mode string, summary map[string]any) error {
	var raw []byte
	if summary != nil {
		b, err := json.Marshal(summary)
		if err != nil {
			return fmt.Errorf("encode applied summary: %w", err)
		}
		raw = b
	}
	if _, err := r.db.Exec(ctx, `
		INSERT INTO paging_config_revision (id, applied_revision, applied_at, applied_mode, applied_by, applied_summary)
		VALUES (1, $1, now(), $2, $3, $4)
		ON CONFLICT (id) DO UPDATE
		   SET applied_revision = EXCLUDED.applied_revision, applied_at = EXCLUDED.applied_at,
		       applied_mode = EXCLUDED.applied_mode, applied_by = EXCLUDED.applied_by,
		       applied_summary = EXCLUDED.applied_summary`, revision, mode, by, raw); err != nil {
		return fmt.Errorf("record applied case alert settings: %w", err)
	}
	return nil
}

const caseAlertRoomColumns = `
       r.id::text, r.ladder, COALESCE(r.team_key, ''), r.label, r.enabled, r.teamless, COALESCE(r.channel, ''),
       (s.room_id IS NOT NULL), r.webhook_hint, r.webhook_set_at, r.webhook_set_by,
       r.last_test_at, r.last_test_status, r.updated_by, r.updated_on`

const caseAlertRoomFrom = `
  FROM paging_chat_room r
  LEFT JOIN paging_chat_secret s ON s.room_id = r.id`

func scanCaseAlertRoom(row pgx.Row) (CaseAlertRoomRow, error) {
	var c CaseAlertRoomRow
	err := row.Scan(&c.ID, &c.Ladder, &c.TeamKey, &c.Label, &c.Enabled, &c.Teamless, &c.Channel,
		&c.HasSecret, &c.WebhookHint, &c.WebhookSetAt, &c.WebhookSetBy,
		&c.LastTestAt, &c.LastTestStatus, &c.UpdatedBy, &c.UpdatedOn)
	return c, err
}

func (r *caseAlertSettingsRepository) ListRooms(ctx context.Context) ([]CaseAlertRoomRow, error) {
	rows, err := r.db.Query(ctx, `SELECT `+caseAlertRoomColumns+caseAlertRoomFrom+`
		 ORDER BY r.ladder, r.team_key NULLS FIRST`)
	if err != nil {
		return nil, fmt.Errorf("list case alert rooms: %w", err)
	}
	defer rows.Close()
	out := []CaseAlertRoomRow{}
	for rows.Next() {
		c, err := scanCaseAlertRoom(rows)
		if err != nil {
			return nil, fmt.Errorf("scan case alert room: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *caseAlertSettingsRepository) GetRoom(ctx context.Context, id string) (*CaseAlertRoomRow, error) {
	c, err := scanCaseAlertRoom(r.db.QueryRow(ctx, `SELECT `+caseAlertRoomColumns+caseAlertRoomFrom+`
		 WHERE r.id = $1::uuid`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get case alert room: %w", err)
	}
	return &c, nil
}

func (r *caseAlertSettingsRepository) SaveRoom(ctx context.Context, actor string, w CaseAlertRoomWrite) (CaseAlertRoomRow, error) {
	var id string
	err := r.inTx(ctx, actor, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			INSERT INTO paging_chat_room (created_by, updated_by, ladder, team_key, label, enabled, teamless, channel)
			VALUES ($1, $1, $2, NULLIF($3, ''), $4, $5, $6, NULLIF($7, ''))
			ON CONFLICT (ladder, COALESCE(team_key, '')) DO UPDATE
			   SET label = EXCLUDED.label, enabled = EXCLUDED.enabled, teamless = EXCLUDED.teamless,
			       channel = EXCLUDED.channel, updated_by = EXCLUDED.updated_by, updated_on = now()
			RETURNING id::text`, actor, w.Ladder, w.TeamKey, w.Label, w.Enabled, w.Teamless, w.Channel).Scan(&id)
		if err != nil {
			return fmt.Errorf("save case alert room: %w", err)
		}
		return nil
	})
	if err != nil {
		return CaseAlertRoomRow{}, err
	}
	c, err := r.GetRoom(ctx, id)
	if err != nil {
		return CaseAlertRoomRow{}, err
	}
	if c == nil {
		return CaseAlertRoomRow{}, fmt.Errorf("case alert room %s vanished after saving", id)
	}
	return *c, nil
}

func (r *caseAlertSettingsRepository) DeleteRoom(ctx context.Context, actor, id string) (bool, error) {
	found := false
	err := r.inTx(ctx, actor, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `DELETE FROM paging_chat_room WHERE id = $1::uuid`, id)
		if err != nil {
			return fmt.Errorf("delete case alert room: %w", err)
		}
		found = tag.RowsAffected() > 0
		return nil
	})
	return found, err
}

func (r *caseAlertSettingsRepository) SetSecret(ctx context.Context, actor, roomID string, s CaseAlertSecret, hint string) (bool, error) {
	found := false
	err := r.inTx(ctx, actor, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE paging_chat_room
			   SET webhook_hint = $2, webhook_set_at = now(), webhook_set_by = $3,
			       last_test_at = NULL, last_test_status = NULL,
			       updated_by = $3, updated_on = now()
			 WHERE id = $1::uuid`, roomID, hint, actor)
		if err != nil {
			return fmt.Errorf("mark case alert webhook set: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return nil
		}
		found = true
		if _, err := tx.Exec(ctx, `
			INSERT INTO paging_chat_secret (room_id, webhook_url, key_id, nonce, ciphertext, set_at, set_by)
			VALUES ($1::uuid, NULLIF($2, ''), NULLIF($3, ''), $4, $5, now(), $6)
			ON CONFLICT (room_id) DO UPDATE
			   SET webhook_url = EXCLUDED.webhook_url, key_id = EXCLUDED.key_id, nonce = EXCLUDED.nonce,
			       ciphertext = EXCLUDED.ciphertext, set_at = EXCLUDED.set_at, set_by = EXCLUDED.set_by`,
			roomID, s.URL, s.KeyID, s.Nonce, s.Ciphertext, actor); err != nil {
			return fmt.Errorf("store case alert webhook: %w", err)
		}
		return nil
	})
	return found, err
}

func (r *caseAlertSettingsRepository) Secret(ctx context.Context, roomID string) (*CaseAlertSecret, error) {
	var s CaseAlertSecret
	err := r.db.QueryRow(ctx, `
		SELECT COALESCE(webhook_url, ''), COALESCE(key_id, ''), nonce, ciphertext
		  FROM paging_chat_secret WHERE room_id = $1::uuid`, roomID).
		Scan(&s.URL, &s.KeyID, &s.Nonce, &s.Ciphertext)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get case alert webhook: %w", err)
	}
	return &s, nil
}

func (r *caseAlertSettingsRepository) Secrets(ctx context.Context) (map[string]CaseAlertSecret, error) {
	rows, err := r.db.Query(ctx, `
		SELECT room_id::text, COALESCE(webhook_url, ''), COALESCE(key_id, ''), nonce, ciphertext
		  FROM paging_chat_secret`)
	if err != nil {
		return nil, fmt.Errorf("list case alert webhooks: %w", err)
	}
	defer rows.Close()
	out := map[string]CaseAlertSecret{}
	for rows.Next() {
		var id string
		var s CaseAlertSecret
		if err := rows.Scan(&id, &s.URL, &s.KeyID, &s.Nonce, &s.Ciphertext); err != nil {
			return nil, fmt.Errorf("scan case alert webhook: %w", err)
		}
		out[id] = s
	}
	return out, rows.Err()
}

func (r *caseAlertSettingsRepository) ClaimTest(ctx context.Context, actor, roomID string, at, notBefore time.Time) (bool, error) {
	claimed := false
	err := r.inTx(ctx, actor, func(tx pgx.Tx) error {
		// One statement decides, so two clicks at once cannot both pass the
		// cooldown.
		tag, err := tx.Exec(ctx, `
			UPDATE paging_chat_room
			   SET last_test_at = $3
			 WHERE id = $1::uuid
			   AND (last_test_at IS NULL OR last_test_at <= $2)`, roomID, notBefore, at)
		if err != nil {
			return fmt.Errorf("claim case alert test card: %w", err)
		}
		claimed = tag.RowsAffected() > 0
		return nil
	})
	return claimed, err
}

func (r *caseAlertSettingsRepository) RecordTest(ctx context.Context, actor, roomID, status string, at time.Time) error {
	return r.inTx(ctx, actor, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			UPDATE paging_chat_room SET last_test_status = $2, last_test_at = $3
			 WHERE id = $1::uuid`, roomID, status, at); err != nil {
			return fmt.Errorf("record case alert test card: %w", err)
		}
		return nil
	})
}

const caseAlertRouteColumns = `
       id::text, ladder, group_name, COALESCE(team_key, ''), COALESCE(rota_code, ''), enabled, updated_by, updated_on`

func scanCaseAlertRoute(row pgx.Row) (CaseAlertRouteRow, error) {
	var c CaseAlertRouteRow
	err := row.Scan(&c.ID, &c.Ladder, &c.GroupName, &c.TeamKey, &c.RotaCode, &c.Enabled, &c.UpdatedBy, &c.UpdatedOn)
	return c, err
}

func (r *caseAlertSettingsRepository) ListRoutes(ctx context.Context) ([]CaseAlertRouteRow, error) {
	rows, err := r.db.Query(ctx, `SELECT `+caseAlertRouteColumns+` FROM paging_group_route ORDER BY ladder, group_name`)
	if err != nil {
		return nil, fmt.Errorf("list case alert routes: %w", err)
	}
	defer rows.Close()
	out := []CaseAlertRouteRow{}
	for rows.Next() {
		c, err := scanCaseAlertRoute(rows)
		if err != nil {
			return nil, fmt.Errorf("scan case alert route: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *caseAlertSettingsRepository) SaveRoute(ctx context.Context, actor string, w CaseAlertRouteWrite) (CaseAlertRouteRow, error) {
	var c CaseAlertRouteRow
	err := r.inTx(ctx, actor, func(tx pgx.Tx) error {
		var err error
		c, err = scanCaseAlertRoute(tx.QueryRow(ctx, `
			INSERT INTO paging_group_route (created_by, updated_by, ladder, group_name, team_key, rota_code, enabled)
			VALUES ($1, $1, $2, $3, NULLIF($4, ''), NULLIF($5, ''), $6)
			ON CONFLICT (ladder, group_name) DO UPDATE
			   SET team_key = EXCLUDED.team_key, rota_code = EXCLUDED.rota_code, enabled = EXCLUDED.enabled,
			       updated_by = EXCLUDED.updated_by, updated_on = now()
			RETURNING `+caseAlertRouteColumns, actor, w.Ladder, w.GroupName, w.TeamKey, w.RotaCode, w.Enabled))
		if err != nil {
			return fmt.Errorf("save case alert route: %w", err)
		}
		return nil
	})
	return c, err
}

func (r *caseAlertSettingsRepository) DeleteRoute(ctx context.Context, actor, id string) (bool, error) {
	found := false
	err := r.inTx(ctx, actor, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `DELETE FROM paging_group_route WHERE id = $1::uuid`, id)
		if err != nil {
			return fmt.Errorf("delete case alert route: %w", err)
		}
		found = tag.RowsAffected() > 0
		return nil
	})
	return found, err
}

const caseAlertValueColumns = `
       id::text, ladder, setting_key, setting_value, updated_by, updated_on`

func scanCaseAlertValue(row pgx.Row) (CaseAlertValueRow, error) {
	var c CaseAlertValueRow
	err := row.Scan(&c.ID, &c.Ladder, &c.Key, &c.Value, &c.UpdatedBy, &c.UpdatedOn)
	return c, err
}

func (r *caseAlertSettingsRepository) ListValues(ctx context.Context) ([]CaseAlertValueRow, error) {
	rows, err := r.db.Query(ctx, `SELECT `+caseAlertValueColumns+` FROM paging_ladder_value ORDER BY ladder, setting_key`)
	if err != nil {
		return nil, fmt.Errorf("list case alert ladder values: %w", err)
	}
	defer rows.Close()
	out := []CaseAlertValueRow{}
	for rows.Next() {
		c, err := scanCaseAlertValue(rows)
		if err != nil {
			return nil, fmt.Errorf("scan case alert ladder value: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *caseAlertSettingsRepository) GetValue(ctx context.Context, id string) (*CaseAlertValueRow, error) {
	c, err := scanCaseAlertValue(r.db.QueryRow(ctx, `SELECT `+caseAlertValueColumns+` FROM paging_ladder_value WHERE id = $1::uuid`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get case alert ladder value: %w", err)
	}
	return &c, nil
}

func (r *caseAlertSettingsRepository) SaveValues(ctx context.Context, actor, ladder string, values map[string][]byte) error {
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	// One order, so two saves at once lock rows the same way round.
	sort.Strings(keys)
	return r.inTx(ctx, actor, func(tx pgx.Tx) error {
		for _, k := range keys {
			if _, err := tx.Exec(ctx, `
				INSERT INTO paging_ladder_value (created_by, updated_by, ladder, setting_key, setting_value)
				VALUES ($1, $1, $2, $3, $4::jsonb)
				ON CONFLICT (ladder, setting_key) DO UPDATE
				   SET setting_value = EXCLUDED.setting_value, updated_by = EXCLUDED.updated_by, updated_on = now()
				 WHERE paging_ladder_value.setting_value IS DISTINCT FROM EXCLUDED.setting_value`,
				actor, ladder, k, string(values[k])); err != nil {
				return fmt.Errorf("save case alert ladder value %s: %w", k, err)
			}
		}
		return nil
	})
}

func (r *caseAlertSettingsRepository) DeleteValue(ctx context.Context, actor, id string) (bool, error) {
	found := false
	err := r.inTx(ctx, actor, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `DELETE FROM paging_ladder_value WHERE id = $1::uuid`, id)
		if err != nil {
			return fmt.Errorf("delete case alert ladder value: %w", err)
		}
		found = tag.RowsAffected() > 0
		return nil
	})
	return found, err
}

// routeGroupWhere is a CRE, SRE or SME team that is not a leadership team:
// what a case or incident can be assigned to and paged for.
const routeGroupWhere = `t.type IS NOT NULL AND lower(t.type) LIKE ANY (ARRAY['cre%', 'sre%', 'sme%'])
	AND lower(t.type) NOT LIKE '%leadership%' AND lower(COALESCE(t.name, '')) NOT LIKE '%leadership%'
	AND btrim(COALESCE(t.name, '')) <> ''`

func (r *caseAlertSettingsRepository) teamRows(ctx context.Context, sql string) ([]CaseAlertTeamRow, error) {
	rows, err := r.db.Query(ctx, sql)
	if err != nil {
		return nil, fmt.Errorf("case alert route teams: %w", err)
	}
	defer rows.Close()
	out := []CaseAlertTeamRow{}
	for rows.Next() {
		var t CaseAlertTeamRow
		if err := rows.Scan(&t.Key, &t.Name, &t.Family); err != nil {
			return nil, fmt.Errorf("scan case alert route team: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *caseAlertSettingsRepository) RouteGroups(ctx context.Context) ([]CaseAlertTeamRow, error) {
	return r.teamRows(ctx, `SELECT t.key, btrim(t.name), `+teamFamilyExpr+` FROM team t
		WHERE `+routeGroupWhere+` ORDER BY 3, lower(t.name)`)
}

func (r *caseAlertSettingsRepository) RouteTeams(ctx context.Context) ([]CaseAlertTeamRow, error) {
	return r.teamRows(ctx, `SELECT t.key, COALESCE(NULLIF(btrim(t.name), ''), t.key), `+teamFamilyExpr+` FROM team t
		WHERE `+rosteredTeamWhere+` AND `+teamFamilyExpr+` IN ('CRE', 'SRE') ORDER BY 3, lower(t.name)`)
}

func (r *caseAlertSettingsRepository) SRERotas(ctx context.Context) ([]CaseAlertRotaRow, error) {
	rows, err := r.db.Query(ctx, `
		SELECT code, COALESCE(NULLIF(btrim(label), ''), code) FROM team_schedule_rota
		 WHERE is_active AND family::text = 'SRE' ORDER BY sort_order, code`)
	if err != nil {
		return nil, fmt.Errorf("case alert SRE rotas: %w", err)
	}
	defer rows.Close()
	out := []CaseAlertRotaRow{}
	for rows.Next() {
		var c CaseAlertRotaRow
		if err := rows.Scan(&c.Code, &c.Label); err != nil {
			return nil, fmt.Errorf("scan case alert SRE rota: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *caseAlertSettingsRepository) TeamFamilyByName(ctx context.Context, name string) (string, bool, error) {
	var family string
	err := r.db.QueryRow(ctx, `SELECT `+teamFamilyExpr+` FROM team t
		WHERE lower(btrim(t.name)) = lower(btrim($1)) AND `+routeGroupWhere+` LIMIT 1`, name).Scan(&family)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("team by name: %w", err)
	}
	return family, true, nil
}

func (r *caseAlertSettingsRepository) IsCRETeamName(ctx context.Context, name string) (bool, error) {
	var ok bool
	if err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM team t
		WHERE lower(btrim(t.name)) = lower(btrim($1)) AND lower(COALESCE(t.type, '')) LIKE 'cre%')`, name).Scan(&ok); err != nil {
		return false, fmt.Errorf("cre team by name: %w", err)
	}
	return ok, nil
}

func (r *caseAlertSettingsRepository) TeamTypeExists(ctx context.Context, teamType string) (bool, error) {
	var ok bool
	if err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM team WHERE lower(type) = lower($1))`, teamType).Scan(&ok); err != nil {
		return false, fmt.Errorf("team type: %w", err)
	}
	return ok, nil
}

func (r *caseAlertSettingsRepository) ShiftCodeExists(ctx context.Context, code string) (bool, error) {
	var ok bool
	if err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM team_schedule_shift WHERE code = $1 AND is_active)`, code).Scan(&ok); err != nil {
		return false, fmt.Errorf("shift code: %w", err)
	}
	return ok, nil
}

func (r *caseAlertSettingsRepository) ActiveUserExists(ctx context.Context, email string) (bool, error) {
	var ok bool
	if err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM "user" WHERE lower(email) = lower($1) AND is_active)`, email).Scan(&ok); err != nil {
		return false, fmt.Errorf("active user: %w", err)
	}
	return ok, nil
}

func (r *caseAlertSettingsRepository) RosteredTeamFamily(ctx context.Context, teamKey string) (string, bool, error) {
	var family string
	err := r.db.QueryRow(ctx, `
		SELECT `+teamFamilyExpr+`
		  FROM team t
		 WHERE t.key = lower($1) AND `+rosteredTeamWhere, teamKey).Scan(&family)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("team family: %w", err)
	}
	return family, true, nil
}

func (r *caseAlertSettingsRepository) ActiveRotaFamily(ctx context.Context, code string) (string, bool, error) {
	var family string
	err := r.db.QueryRow(ctx, `
		SELECT family::text FROM team_schedule_rota WHERE code = $1 AND is_active`, code).Scan(&family)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("rota family: %w", err)
	}
	return family, true, nil
}
