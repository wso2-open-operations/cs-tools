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
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// LinkWorkaroundProblem completes a problem CreateProblemFromServiceNow just
// inserted -- its group -- and links the incident to it, leaving every value
// ServiceNow gave it alone. Run with ENTITY_TEST_DATABASE_URL; seeded rows
// are deleted after.
const (
	wpGroupID    = "47777777-0000-0000-0000-0000000000f1"
	wpServiceID  = "47777777-0000-0000-0000-0000000000f2"
	wpIncidentID = "47777777-0000-0000-0000-0000000000f3"
	wpProblemID  = "47777777-0000-0000-0000-0000000000f4"
)

func TestLinkWorkaroundProblemIntegration(t *testing.T) {
	dsn := os.Getenv("ENTITY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ENTITY_TEST_DATABASE_URL not set")
	}
	ctx := WithSystemIdentity(context.Background())
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	cleanup := func() {
		for _, q := range []string{
			`UPDATE incident SET problem_id = NULL WHERE id = '` + wpIncidentID + `'`,
			`DELETE FROM work_item WHERE id IN ('` + wpProblemID + `', '` + wpIncidentID + `')`,
			`DELETE FROM event_outbox WHERE entity_id = '` + wpIncidentID + `'`,
			`DELETE FROM service WHERE id = '` + wpServiceID + `'`,
			`DELETE FROM "group" WHERE id = '` + wpGroupID + `'`,
		} {
			if _, err := pool.Exec(context.Background(), q); err != nil {
				t.Errorf("CLEANUP FAILED (%s): %v", q, err)
			}
		}
	}
	cleanup()
	t.Cleanup(func() { cleanup(); pool.Close() })
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	exec(`INSERT INTO "group" (id, created_on, updated_on, created_by, updated_by, name) VALUES ($1, NOW(), NOW(), 't', 't', 'Workaround group (test)')`, wpGroupID)
	exec(`INSERT INTO service (id, created_on, updated_on, created_by, updated_by, name, number) VALUES ($1, NOW(), NOW(), 't', 't', 'monitoring (test)', 'SVC-WP-1')`, wpServiceID)
	exec(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, subject, type)
	      VALUES ($1, NOW(), NOW(), 't', 't', 'INC-WP-0001', 'workaround link test', 'INCIDENT')`, wpIncidentID)
	exec(`INSERT INTO incident (id, state, resolution_code, service_id) VALUES ($1, 'RESOLVED', 'SOLVED_WORK_AROUND', $2)`, wpIncidentID, wpServiceID)

	repo := NewProblemRepository(NewScoped(pool))
	// What createProblemSNFirst writes: ServiceNow's id and number, the primary incident.
	incident := wpIncidentID
	if _, err := repo.CreateProblemFromServiceNow(ctx, domain.CreateProblemRequest{Subject: "Fix the root cause of INC-WP-0001", PrimaryIncidentID: &incident},
		wpProblemID, "PRB-WP-0001", "jane.doe@example.com", nil, "PLANNING"); err != nil {
		t.Fatalf("CreateProblemFromServiceNow: %v", err)
	}

	group := wpGroupID
	stored, err := repo.LinkWorkaroundProblem(ctx, wpProblemID, wpIncidentID, &group, "jane.doe@example.com")
	if err != nil {
		t.Fatalf("LinkWorkaroundProblem: %v", err)
	}
	if stored == nil || *stored != wpGroupID {
		t.Errorf("stored group = %v, want %s", stored, wpGroupID)
	}

	var number, gotGroup, impact, urgency, priority, problemIncident, incidentProblem, updatedBy string
	var gotService *string
	if err := pool.QueryRow(ctx, `
		SELECT wi.number, p.service_id::text, wi.assignment_group_id::text, p.impact::text, p.urgency::text,
		       p.priority::text, p.incident_id::text,
		       (SELECT problem_id::text FROM incident WHERE id = $2),
		       (SELECT updated_by FROM work_item WHERE id = $2)
		FROM problem p JOIN work_item wi ON wi.id = p.id WHERE p.id = $1`, wpProblemID, wpIncidentID).
		Scan(&number, &gotService, &gotGroup, &impact, &urgency, &priority, &problemIncident, &incidentProblem, &updatedBy); err != nil {
		t.Fatalf("read back: %v", err)
	}
	// ServiceNow's values stay: its number and priority, its default impact
	// and urgency, and no service (its API takes none).
	if number != "PRB-WP-0001" || gotService != nil || gotGroup != wpGroupID ||
		impact != "LOW" || urgency != "LOW" || priority != "PLANNING" {
		t.Errorf("problem = %s service %v group %s %s/%s/%s", number, gotService, gotGroup, impact, urgency, priority)
	}
	if problemIncident != wpIncidentID || incidentProblem != wpProblemID || updatedBy != "jane.doe@example.com" {
		t.Errorf("link: problem.incident_id %s, incident.problem_id %s, incident updated_by %s", problemIncident, incidentProblem, updatedBy)
	}

	// A group missing from the database leaves the problem unassigned.
	missing := "47777777-0000-0000-0000-0000000000f9"
	if stored, err := repo.LinkWorkaroundProblem(ctx, wpProblemID, wpIncidentID, &missing, "jane.doe@example.com"); err != nil || stored != nil {
		t.Fatalf("LinkWorkaroundProblem with a missing group: stored %v, err %v; want none", stored, err)
	}
	var stillGroup *string
	if err := pool.QueryRow(ctx, `SELECT assignment_group_id::text FROM work_item WHERE id = $1`, wpProblemID).Scan(&stillGroup); err != nil {
		t.Fatalf("read group: %v", err)
	}
	if stillGroup != nil {
		t.Errorf("group = %s, want none for a group the database lacks", *stillGroup)
	}

	if _, err := repo.LinkWorkaroundProblem(ctx, wpProblemID, "47777777-0000-0000-0000-0000000000fa", nil, "x@example.com"); !errors.Is(err, ErrIncidentNotFound) {
		t.Errorf("unknown incident: err = %v, want ErrIncidentNotFound", err)
	}
}
