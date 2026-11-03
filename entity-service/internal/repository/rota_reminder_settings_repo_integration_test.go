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
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// A family's reminder settings: nothing until saved, a save is read back as
// saved, a second save replaces it, and both are in the audit trail with who
// made them. Needs migrations 0155 and 0242; skipped unless
// ENTITY_TEST_DATABASE_URL is set. Uses SME and refuses to run where SME
// settings already exist, so it never overwrites real ones.
func TestRotaReminderSettingsIntegration_SaveAndRead(t *testing.T) {
	dsn := os.Getenv("ENTITY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ENTITY_TEST_DATABASE_URL is not set; skipping the live-database tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	repo := NewRotaReminderSettingsRepository(pool)
	if s, err := repo.Get(ctx, "SME"); err != nil {
		t.Fatal(err)
	} else if s != nil {
		t.Skip("SME reminder settings already exist here; not touching them")
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM team_schedule_reminder_setting WHERE family = 'SME'`)
	})

	first, err := repo.Save(ctx, "ann@example.com", domain.RotaReminderSettings{
		Family: "SME", WeekdayEnabled: true, WeekendEnabled: false, InvitesEnabled: true,
		WeekendRequired: map[string]int{"SME_WEEKEND": 2},
		Windows:         map[string]domain.ReminderWindow{"SME_NIGHT": {Start: "22:00", End: "06:00", TimeZone: "Asia/Colombo"}},
		InviteRotations: map[string]bool{"SME_NIGHT": false},
		CopyTo:          []string{"lead@example.com"},
		WeekendCopyTo:   []string{"head@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := repo.Get(ctx, "SME")
	if err != nil || got == nil || got.WeekendEnabled || got.WeekendRequired["SME_WEEKEND"] != 2 ||
		got.Windows["SME_NIGHT"].End != "06:00" || got.CopyTo[0] != "lead@example.com" || got.UpdatedBy != "ann@example.com" ||
		len(got.InviteRotations) != 1 || got.InviteRotations["SME_NIGHT"] ||
		len(got.WeekendCopyTo) != 1 || got.WeekendCopyTo[0] != "head@example.com" {
		t.Fatalf("read back %+v, %v", got, err)
	}
	second, err := repo.Save(ctx, "ben@example.com", domain.RotaReminderSettings{Family: "SME", WeekdayEnabled: false, WeekendEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if second.WeekdayEnabled || !second.WeekendEnabled || len(second.WeekendRequired) != 0 || len(second.CopyTo) != 0 ||
		len(second.InviteRotations) != 0 || len(second.WeekendCopyTo) != 0 ||
		second.UpdatedBy != "ben@example.com" || second.UpdatedOn.Before(first.UpdatedOn) {
		t.Fatalf("replaced with %+v", second)
	}
	var actors int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM team_schedule_audit
		WHERE table_name = 'team_schedule_reminder_setting' AND actor IN ('ann@example.com', 'ben@example.com')
		  AND new_row ->> 'family' = 'SME'`).Scan(&actors); err != nil {
		t.Fatalf("audit: %v", err)
	}
	if actors < 2 {
		t.Errorf("audit rows = %d, want the two saves", actors)
	}
}
