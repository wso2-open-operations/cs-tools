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
)

// TestSupportGroupOfServiceLive checks the repository SQL against a real
// database. It seeds its own groups and services -- a service with an active
// support group, one whose support group is inactive, one with none, and a
// group that supports no service -- so every case is checked on every run
// whatever the database already holds, and deletes them afterwards.
func TestSupportGroupOfServiceLive(t *testing.T) {
	dsn := os.Getenv("PG_OUTAGE_TEST_DSN")
	if dsn == "" {
		t.Skip("PG_OUTAGE_TEST_DSN not set; this test needs a real database")
	}
	ctx := WithSystemIdentity(context.Background())
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()
	repo := NewIncidentRepository(NewScoped(pool))

	const (
		group        = "5e5e5e5e-0000-4000-8000-000000000001"
		withGroup    = "5e5e5e5e-0000-4000-8000-000000000002"
		without      = "5e5e5e5e-0000-4000-8000-000000000003"
		inactive     = "5e5e5e5e-0000-4000-8000-000000000004"
		withInactive = "5e5e5e5e-0000-4000-8000-000000000005"
		unused       = "5e5e5e5e-0000-4000-8000-000000000006"
	)
	// Registered before the inserts so a half-seeded run is still cleaned up;
	// services first, since they reference the groups.
	defer func() {
		bg := context.Background()
		if _, err := pool.Exec(bg, `DELETE FROM service WHERE id IN ($1::uuid, $2::uuid, $3::uuid)`, withGroup, without, withInactive); err != nil {
			t.Errorf("CLEANUP FAILED, delete services %s, %s, %s by hand: %v", withGroup, without, withInactive, err)
		}
		if _, err := pool.Exec(bg, `DELETE FROM "group" WHERE id IN ($1::uuid, $2::uuid, $3::uuid)`, group, inactive, unused); err != nil {
			t.Errorf("CLEANUP FAILED, delete groups %s, %s, %s by hand: %v", group, inactive, unused, err)
		}
	}()
	// is_active NULL on the first group: NULL counts as active.
	if _, err := pool.Exec(ctx, `
INSERT INTO "group" (id, created_on, updated_on, created_by, updated_by, name, is_active)
VALUES ($1::uuid, now(), now(), 'test', 'test', 'support-group live test', NULL),
       ($2::uuid, now(), now(), 'test', 'test', 'support-group live test (inactive)', FALSE),
       ($3::uuid, now(), now(), 'test', 'test', 'support-group live test (unused)', TRUE)`, group, inactive, unused); err != nil {
		t.Fatalf("seed groups: %v", err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO service (id, created_on, updated_on, created_by, updated_by, name, number, support_group_id)
VALUES ($1::uuid, now(), now(), 'test', 'test', 'support-group live test (with)',     'SVC-TEST-1', $4::uuid),
       ($2::uuid, now(), now(), 'test', 'test', 'support-group live test (without)',  'SVC-TEST-2', NULL),
       ($3::uuid, now(), now(), 'test', 'test', 'support-group live test (inactive)', 'SVC-TEST-3', $5::uuid)`,
		withGroup, without, withInactive, group, inactive); err != nil {
		t.Fatalf("seed services: %v", err)
	}

	for _, c := range []struct {
		name, serviceID string
		want            ServiceSupportGroup
	}{
		{"service with a support group", withGroup, ServiceSupportGroup{Found: true, ServiceName: "support-group live test (with)", GroupID: group, GroupName: "support-group live test"}},
		{"service without one", without, ServiceSupportGroup{Found: true, ServiceName: "support-group live test (without)"}},
		{"unknown service", "00000000-0000-0000-0000-000000000001", ServiceSupportGroup{}},
	} {
		if got, err := repo.SupportGroupOfService(ctx, c.serviceID); err != nil || got != c.want {
			t.Errorf("%s: got %+v err %v, want %+v", c.name, got, err, c.want)
		}
	}

	for _, c := range []struct {
		name, groupID string
		want          bool
	}{
		{"active (NULL) support group", group, true},
		{"inactive support group", inactive, false},
		{"group that supports no service", unused, false},
		{"unknown group", "00000000-0000-0000-0000-000000000001", false},
	} {
		if got, err := repo.IsSupportGroup(ctx, c.groupID); err != nil || got != c.want {
			t.Errorf("IsSupportGroup %s: got %v err %v, want %v", c.name, got, err, c.want)
		}
	}

	// The picker lists exactly what the create accepts.
	groups, total, err := NewGroupRepository(pool).SearchSupportGroups(ctx, "support-group live test", 50, 0)
	if err != nil {
		t.Fatalf("SearchSupportGroups: %v", err)
	}
	if total != 1 || len(groups) != 1 || groups[0].ID != group || !groups[0].Active {
		t.Errorf("SearchSupportGroups = %+v (total %d), want only the active support group %s", groups, total, group)
	}
}
