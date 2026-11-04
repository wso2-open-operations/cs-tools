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

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// RotaReminderSettingsRepository stores each family's rota reminder settings
// (team_schedule_reminder_setting, migration 0242).
type RotaReminderSettingsRepository interface {
	// Get is the family's settings, or nil when none were saved.
	Get(ctx context.Context, family string) (*domain.RotaReminderSettings, error)
	// Save replaces the family's settings, as actor (recorded in the audit
	// trail).
	Save(ctx context.Context, actor string, s domain.RotaReminderSettings) (domain.RotaReminderSettings, error)
}

type rotaReminderSettingsRepository struct{ db *pgxpool.Pool }

// NewRotaReminderSettingsRepository constructs a RotaReminderSettingsRepository.
func NewRotaReminderSettingsRepository(db *pgxpool.Pool) RotaReminderSettingsRepository {
	return &rotaReminderSettingsRepository{db: db}
}

const rotaReminderSettingsColumns = `family, weekday_enabled, weekend_enabled, invites_enabled, weekend_required, windows, invite_rotations, copy_to, weekend_copy_to, updated_by, updated_on`

func scanRotaReminderSettings(row pgx.Row) (domain.RotaReminderSettings, error) {
	var out domain.RotaReminderSettings
	var required, windows, inviteRotations []byte
	if err := row.Scan(&out.Family, &out.WeekdayEnabled, &out.WeekendEnabled, &out.InvitesEnabled, &required, &windows,
		&inviteRotations, &out.CopyTo, &out.WeekendCopyTo, &out.UpdatedBy, &out.UpdatedOn); err != nil {
		return domain.RotaReminderSettings{}, err
	}
	out.WeekendRequired = map[string]int{}
	if err := json.Unmarshal(required, &out.WeekendRequired); err != nil {
		return domain.RotaReminderSettings{}, fmt.Errorf("decode weekend_required: %w", err)
	}
	out.Windows = map[string]domain.ReminderWindow{}
	if err := json.Unmarshal(windows, &out.Windows); err != nil {
		return domain.RotaReminderSettings{}, fmt.Errorf("decode windows: %w", err)
	}
	out.InviteRotations = map[string]bool{}
	if err := json.Unmarshal(inviteRotations, &out.InviteRotations); err != nil {
		return domain.RotaReminderSettings{}, fmt.Errorf("decode invite_rotations: %w", err)
	}
	if out.CopyTo == nil {
		out.CopyTo = []string{}
	}
	if out.WeekendCopyTo == nil {
		out.WeekendCopyTo = []string{}
	}
	return out, nil
}

func (r *rotaReminderSettingsRepository) Get(ctx context.Context, family string) (*domain.RotaReminderSettings, error) {
	out, err := scanRotaReminderSettings(r.db.QueryRow(ctx,
		`SELECT `+rotaReminderSettingsColumns+` FROM team_schedule_reminder_setting WHERE family = $1`, family))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get rota reminder settings: %w", err)
	}
	return &out, nil
}

func (r *rotaReminderSettingsRepository) Save(ctx context.Context, actor string, s domain.RotaReminderSettings) (domain.RotaReminderSettings, error) {
	// A nil map would be stored as JSON null, which the table refuses: none
	// is an empty object.
	if s.WeekendRequired == nil {
		s.WeekendRequired = map[string]int{}
	}
	if s.Windows == nil {
		s.Windows = map[string]domain.ReminderWindow{}
	}
	if s.InviteRotations == nil {
		s.InviteRotations = map[string]bool{}
	}
	required, err := json.Marshal(s.WeekendRequired)
	if err != nil {
		return domain.RotaReminderSettings{}, fmt.Errorf("encode weekend_required: %w", err)
	}
	windows, err := json.Marshal(s.Windows)
	if err != nil {
		return domain.RotaReminderSettings{}, fmt.Errorf("encode windows: %w", err)
	}
	inviteRotations, err := json.Marshal(s.InviteRotations)
	if err != nil {
		return domain.RotaReminderSettings{}, fmt.Errorf("encode invite_rotations: %w", err)
	}
	copyTo := s.CopyTo
	if copyTo == nil {
		copyTo = []string{}
	}
	weekendCopyTo := s.WeekendCopyTo
	if weekendCopyTo == nil {
		weekendCopyTo = []string{}
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.RotaReminderSettings{}, fmt.Errorf("save rota reminder settings: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := nameTheActor(ctx, tx, actor); err != nil {
		return domain.RotaReminderSettings{}, err
	}
	out, err := scanRotaReminderSettings(tx.QueryRow(ctx, `
		INSERT INTO team_schedule_reminder_setting
		       (family, weekday_enabled, weekend_enabled, invites_enabled, weekend_required, windows, invite_rotations, copy_to, weekend_copy_to, created_by, updated_by)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6::jsonb, $7::jsonb, $8, $9, $10, $10)
		ON CONFLICT (family) DO UPDATE
		   SET weekday_enabled  = EXCLUDED.weekday_enabled,
		       weekend_enabled  = EXCLUDED.weekend_enabled,
		       invites_enabled  = EXCLUDED.invites_enabled,
		       weekend_required = EXCLUDED.weekend_required,
		       windows          = EXCLUDED.windows,
		       invite_rotations = EXCLUDED.invite_rotations,
		       copy_to          = EXCLUDED.copy_to,
		       weekend_copy_to  = EXCLUDED.weekend_copy_to,
		       updated_by       = EXCLUDED.updated_by,
		       updated_on       = now()
		RETURNING `+rotaReminderSettingsColumns,
		s.Family, s.WeekdayEnabled, s.WeekendEnabled, s.InvitesEnabled, string(required), string(windows), string(inviteRotations), copyTo, weekendCopyTo, actor))
	if err != nil {
		return domain.RotaReminderSettings{}, fmt.Errorf("save rota reminder settings: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.RotaReminderSettings{}, fmt.Errorf("save rota reminder settings: %w", err)
	}
	return out, nil
}
