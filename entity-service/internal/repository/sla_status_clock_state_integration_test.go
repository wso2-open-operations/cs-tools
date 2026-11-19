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

// GetClockState returns an incident's current assignment group name (and
// nothing for a case with the same group), while TeamEmail/TeamLeadName stay
// the account CRE team's for both. Same DSN and skip-when-unset pattern as the
// other SLA integration tests:
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run ClockStateIntegration

package repository_test

import (
	"context"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	clockStateManagerID  = "48888888-0000-0000-0000-000000000001"
	clockStateGroupID    = "48888888-0000-0000-0000-000000000002"
	clockStateIncidentID = "48888888-0000-0000-0000-000000000003"
	clockStateCaseID     = "48888888-0000-0000-0000-000000000004"
	clockStatePolicyID   = "48888888-0000-0000-0000-000000000005"
)

func TestClockStateIntegration_IncidentAssignmentGroupName(t *testing.T) {
	pool := caseStatsPool(t)
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)

	mustExec := func(sql string, args ...any) {
		t.Helper()
		if _, err := scoped.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed: %v\n%s", err, sql)
		}
	}
	cleanup := func() {
		for _, sql := range []string{
			`DELETE FROM sla WHERE work_item_id IN ($1::uuid, $2::uuid)`,
			`DELETE FROM work_item WHERE id IN ($1::uuid, $2::uuid)`,
		} {
			_, _ = scoped.Exec(ctx, sql, clockStateIncidentID, clockStateCaseID)
		}
		_, _ = scoped.Exec(ctx, `DELETE FROM sla_policy WHERE id = $1::uuid`, clockStatePolicyID)
		_, _ = scoped.Exec(ctx, `DELETE FROM "group" WHERE id = $1::uuid`, clockStateGroupID)
		_, _ = scoped.Exec(ctx, `DELETE FROM "user" WHERE id = $1::uuid`, clockStateManagerID)
	}
	cleanup()
	t.Cleanup(cleanup)

	mustExec(`INSERT INTO "user" (id, created_on, updated_on, user_name, name, email)
	          VALUES ($1, now(), now(), 'clock-state-test-manager', 'Clock State Manager', 'manager@example.com')`, clockStateManagerID)
	mustExec(`INSERT INTO "group" (id, created_on, updated_on, created_by, updated_by, name, group_email, manager_id)
	          VALUES ($1, now(), now(), 'clock-state-test', 'clock-state-test', 'Clock State SRE Group', 'sre-group@example.com', $2)`,
		clockStateGroupID, clockStateManagerID)
	for _, wi := range []struct{ id, number, typ string }{
		{clockStateIncidentID, "CLOCKSTATEINC1", "INCIDENT"},
		{clockStateCaseID, "CLOCKSTATECASE1", "CASE"},
	} {
		mustExec(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, subject, type, assignment_group_id)
		          VALUES ($1, now(), now(), 'clock-state-test', 'clock-state-test', $2, 'clock state test', $3::work_item_type_enum, $4)`,
			wi.id, wi.number, wi.typ, clockStateGroupID)
	}
	mustExec(`INSERT INTO sla_policy (id, created_on, updated_on, created_by, updated_by, name, target, duration)
	          VALUES ($1, now(), now(), 'clock-state-test', 'clock-state-test', 'Clock State Resolution', 'RESOLUTION'::sla_policy_target_enum, INTERVAL '4 hours')`,
		clockStatePolicyID)
	for _, id := range []string{clockStateIncidentID, clockStateCaseID} {
		mustExec(`INSERT INTO sla (id, created_on, updated_on, created_by, updated_by, work_item_id, sla_policy_id, is_active, stage, start_on)
		          VALUES (gen_random_uuid(), now(), now(), 'clock-state-test', 'clock-state-test', $1, $2, TRUE, 'IN_PROGRESS'::sla_stage_enum, now())`,
			id, clockStatePolicyID)
	}

	repo := repository.NewSLAStatusRepository(scoped)

	incident, err := repo.GetClockState(ctx, clockStateIncidentID, "RESOLUTION", "")
	if err != nil {
		t.Fatalf("GetClockState(incident): %v", err)
	}
	if !incident.Found {
		t.Fatal("GetClockState(incident): Found = false, want true")
	}
	if incident.TeamEmail != "" || incident.TeamLeadName != "" {
		t.Errorf("incident TeamEmail/TeamLeadName = %q/%q, want empty: still the account CRE team's (none here), never the assignment group's", incident.TeamEmail, incident.TeamLeadName)
	}
	if incident.AssignmentGroupName != "Clock State SRE Group" {
		t.Errorf("incident AssignmentGroupName = %q, want the incident's current assignment group", incident.AssignmentGroupName)
	}

	c, err := repo.GetClockState(ctx, clockStateCaseID, "RESOLUTION", "")
	if err != nil {
		t.Fatalf("GetClockState(case): %v", err)
	}
	if !c.Found {
		t.Fatal("GetClockState(case): Found = false, want true")
	}
	if c.TeamEmail != "" || c.TeamLeadName != "" {
		t.Errorf("case TeamEmail/TeamLeadName = %q/%q, want empty: a case's team is its account CRE team, never its assignment group", c.TeamEmail, c.TeamLeadName)
	}
	if c.AssignmentGroupName != "" {
		t.Errorf("case AssignmentGroupName = %q, want empty (incidents only)", c.AssignmentGroupName)
	}
}
