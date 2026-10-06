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

// Integration test for an incident task's assignment group: the incident
// flows store it in work_item.assignment_group_id, and the list, detail and
// "assignmentGroupId" filter read it back. Skipped unless
// INCIDENT_TASK_GATE_TEST_DSN is set.
package repository_test

import (
	"context"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const taskGroupID = "7a5c0e1d-3b2f-4c6a-9d8e-1f2a3b4c5d6e"

func TestIncidentTask_AssignmentGroup(t *testing.T) {
	pool := incidentTaskGatePool(t)
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)
	if _, err := scoped.Exec(ctx,
		`INSERT INTO "group" (id, created_on, updated_on, created_by, updated_by, name)
		 VALUES ($1, now(), now(), 'test', 'test', 'Task Group Test Team') ON CONFLICT (id) DO NOTHING`, taskGroupID); err != nil {
		t.Fatalf("seed group: %v", err)
	}
	t.Cleanup(func() {
		_, _ = scoped.Exec(ctx, `DELETE FROM work_item WHERE subject = $1`, gateTaskSubject)
		_, _ = scoped.Exec(ctx, `DELETE FROM "group" WHERE id = $1`, taskGroupID)
	})

	incID := resolvedIncident(t, pool)
	withGroup, _ := addTask(t, pool, incID, strp("OPEN"))
	withoutGroup, _ := addTask(t, pool, incID, strp("OPEN"))
	if _, err := scoped.Exec(ctx, `UPDATE work_item SET assignment_group_id = $1 WHERE id = $2`, taskGroupID, withGroup); err != nil {
		t.Fatalf("set group: %v", err)
	}
	tasks := repository.NewIncidentTaskRepository(scoped)

	detail, err := tasks.GetIncidentTask(ctx, withGroup)
	if err != nil {
		t.Fatalf("GetIncidentTask: %v", err)
	}
	if detail.AssignmentGroup == nil || detail.AssignmentGroup.ID != taskGroupID || detail.AssignmentGroup.Name != "Task Group Test Team" {
		t.Errorf("detail assignmentGroup = %+v, want the seeded group", detail.AssignmentGroup)
	}
	if d, err := tasks.GetIncidentTask(ctx, withoutGroup); err != nil || d.AssignmentGroup != nil {
		t.Errorf("task without a group: assignmentGroup = %+v (err %v), want nil", d.AssignmentGroup, err)
	}

	req := domain.SearchIncidentTasksRequest{Pagination: domain.Pagination{Limit: 10}}
	all, total, err := tasks.SearchIncidentTasks(ctx, req, nil, []string{incID}, nil)
	if err != nil || total != 2 {
		t.Fatalf("search by incident: total %d (err %v), want 2", total, err)
	}
	for _, task := range all {
		if *task.ID == withGroup && (task.AssignmentGroup == nil || task.AssignmentGroup.Name != "Task Group Test Team") {
			t.Errorf("list assignmentGroup = %+v, want the seeded group", task.AssignmentGroup)
		}
	}

	filtered, total, err := tasks.SearchIncidentTasks(ctx, req, nil, []string{incID}, []string{taskGroupID})
	if err != nil || total != 1 || len(filtered) != 1 || *filtered[0].ID != withGroup {
		t.Errorf("filter by group: total %d, %d rows (err %v), want only the grouped task", total, len(filtered), err)
	}
}
