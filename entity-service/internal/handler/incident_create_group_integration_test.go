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

// POST /incidents' assignment group, end to end: real handler, real incident
// service, real Postgres (plain and dual-write, the ServiceNow mirror
// stubbed). Skipped without INCIDENT_CREATE_GROUP_TEST_DSN.
//
//	INCIDENT_CREATE_GROUP_TEST_DSN=postgres://... go test ./internal/handler/ -run IncidentCreateGroupIntegration

package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

const (
	cgSubject        = "incident-create-group integration test"
	cgCallerID       = "e3000000-0000-4000-8000-000000000001"
	cgGroup          = "e3000000-0000-4000-8000-000000000011"
	cgDefaultGroup   = "e3000000-0000-4000-8000-000000000012"
	cgInactiveGroup  = "e3000000-0000-4000-8000-000000000013"
	cgService        = "e3000000-0000-4000-8000-000000000021"
	cgGrouplessSvc   = "e3000000-0000-4000-8000-000000000022"
	cgDefaultService = "e3000000-0000-4000-8000-000000000023"
	cgInactiveSvc    = "e3000000-0000-4000-8000-000000000024"
)

// cgMirror records the incident the dual-write path sends to ServiceNow.
type cgMirror struct {
	service.IncidentService
	mu    sync.Mutex
	calls []domain.CreateIncidentRequest
}

// CreateIncident records the request the ServiceNow mirror received and answers with a fresh id and number.
func (m *cgMirror) CreateIncident(_ context.Context, req domain.CreateIncidentRequest) (domain.CreateIncidentResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, req)
	resp := domain.CreateIncidentResponse{}
	resp.Incident.ID = "e3000000-0000-4000-8000-0000000000a" + string(rune('0'+len(m.calls)))
	resp.Incident.Number = "INC-CG-" + string(rune('0'+len(m.calls)))
	resp.Incident.CreatedBy = "cg@test.local"
	return resp, nil
}

// newCreateGroupEnv connects to INCIDENT_CREATE_GROUP_TEST_DSN (skipping without it) and seeds the caller, the
// groups and the services these tests create incidents against, removing them again when the test ends.
func newCreateGroupEnv(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("INCIDENT_CREATE_GROUP_TEST_DSN")
	if dsn == "" {
		t.Skip("INCIDENT_CREATE_GROUP_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	sys := repository.WithSystemIdentity(context.Background())
	cleanup := func() {
		_, _ = pool.Exec(sys, `DELETE FROM comment WHERE work_item_id IN (SELECT id FROM work_item WHERE subject = $1)`, cgSubject)
		_, _ = pool.Exec(sys, `DELETE FROM event_outbox WHERE entity_id IN (SELECT id FROM work_item WHERE subject = $1)`, cgSubject)
		_, _ = pool.Exec(sys, `DELETE FROM work_item WHERE subject = $1`, cgSubject)
		_, _ = pool.Exec(sys, `DELETE FROM service WHERE id = ANY($1::uuid[])`, []string{cgService, cgGrouplessSvc, cgDefaultService, cgInactiveSvc})
		_, _ = pool.Exec(sys, `DELETE FROM "group" WHERE id = ANY($1::uuid[])`, []string{cgGroup, cgDefaultGroup, cgInactiveGroup})
		_, _ = pool.Exec(sys, `DELETE FROM "user" WHERE id = $1`, cgCallerID)
	}
	cleanup()
	t.Cleanup(cleanup)

	mustExec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(sys, sql, args...); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	mustExec(`INSERT INTO "user" (id, created_on, updated_on, user_name, email) VALUES ($1, NOW(), NOW(), 'cg-caller', 'cg-caller@test.local')`, cgCallerID)
	mustExec(`INSERT INTO "group" (id, created_on, updated_on, created_by, updated_by, name, is_active) VALUES
		($1, NOW(), NOW(), 't', 't', 'CG Support', TRUE),
		($2, NOW(), NOW(), 't', 't', 'CG Default Team', NULL),
		($3, NOW(), NOW(), 't', 't', 'CG Retired', FALSE)`, cgGroup, cgDefaultGroup, cgInactiveGroup)
	mustExec(`INSERT INTO service (id, created_on, updated_on, created_by, updated_by, name, number, support_group_id) VALUES
		($1, NOW(), NOW(), 't', 't', 'CG Service', 'CG-SVC-1', $5::uuid),
		($2, NOW(), NOW(), 't', 't', 'CG Groupless', 'CG-SVC-2', NULL),
		($3, NOW(), NOW(), 't', 't', 'CG Default', 'CG-SVC-3', $6::uuid),
		($4, NOW(), NOW(), 't', 't', 'CG Inactive', 'CG-SVC-4', $7::uuid)`,
		cgService, cgGrouplessSvc, cgDefaultService, cgInactiveSvc, cgGroup, cgDefaultGroup, cgInactiveGroup)
	return pool
}

// postIncident sends a create through the handler as an internal user.
func postIncident(t *testing.T, svc service.IncidentService, extra string, serviceID string) *httptest.ResponseRecorder {
	t.Helper()
	body := `{"subject":"` + cgSubject + `","category":"INQUIRY","serviceId":"` + serviceID + `",` +
		`"impact":"LOW","urgency":"LOW","callerId":"` + cgCallerID + `"` + extra + `}`
	req := httptest.NewRequest(http.MethodPost, "/incidents", strings.NewReader(body))
	ctx := auth.WithIdentity(repository.WithSystemIdentity(req.Context()), auth.Identity{Validated: true, UserEmail: "cg-engineer@test.local"})
	rec := httptest.NewRecorder()
	NewIncidentHandler(svc).CreateIncident(rec, req.WithContext(ctx))
	return rec
}

// storedGroupAndNote reads the newest test incident's group and work note.
func storedGroupAndNote(t *testing.T, pool *pgxpool.Pool) (group, note string) {
	t.Helper()
	sys := repository.WithSystemIdentity(context.Background())
	var g, n *string
	if err := pool.QueryRow(sys, `
		SELECT wi.assignment_group_id::text,
		       (SELECT c.content FROM comment c WHERE c.work_item_id = wi.id AND c.type = 'WORK_NOTE' ORDER BY c.created_on DESC LIMIT 1)
		FROM work_item wi WHERE wi.subject = $1 ORDER BY wi.created_on DESC LIMIT 1`, cgSubject).Scan(&g, &n); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if g != nil {
		group = *g
	}
	if n != nil {
		note = *n
	}
	return group, note
}

// countTestIncidents counts the incidents these tests created, so a refused create can be shown to write nothing.
func countTestIncidents(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(repository.WithSystemIdentity(context.Background()), `SELECT COUNT(*) FROM work_item WHERE subject = $1`, cgSubject).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}

func TestIncidentCreateGroupIntegration_Postgres(t *testing.T) {
	pool := newCreateGroupEnv(t)
	repo := repository.NewIncidentRepository(repository.NewScoped(pool))
	svc := service.WithIncidentDefaultService(service.NewIncidentServiceWithPublisher(repo, repository.NewUserRepository(pool), nil), cgDefaultService)

	for _, tc := range []struct {
		name, serviceID, extra, wantGroup, wantNote string
	}{
		{"service's group", cgService, "", cgGroup, "Assignment group set from service CG Service's support group"},
		{"default team", cgGrouplessSvc, `,"workNotes":"caller note"`, cgDefaultGroup, "caller note\n\nService CG Groupless has no support group; assigned to the default team (CG Default Team)"},
		{"chosen group", cgGrouplessSvc, `,"assignmentGroupId":"` + cgGroup + `"`, cgGroup, "Assignment group chosen by cg-engineer@test.local"},
		{"service's own group sent", cgService, `,"assignmentGroupId":"` + cgGroup + `"`, cgGroup, "Assignment group chosen by cg-engineer@test.local"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if rec := postIncident(t, svc, tc.extra, tc.serviceID); rec.Code != http.StatusCreated {
				t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
			}
			group, note := storedGroupAndNote(t, pool)
			if group != tc.wantGroup || note != tc.wantNote {
				t.Errorf("stored group %q note %q, want %q %q", group, note, tc.wantGroup, tc.wantNote)
			}
		})
	}

	for name, extra := range map[string]string{
		"inactive support group":   `,"assignmentGroupId":"` + cgInactiveGroup + `"`,
		"group supporting nothing": `,"assignmentGroupId":"e3000000-0000-4000-8000-0000000000ff"`,
		"not a UUID":               `,"assignmentGroupId":"CG Support"`,
	} {
		t.Run("refused: "+name, func(t *testing.T) {
			before := countTestIncidents(t, pool)
			if rec := postIncident(t, svc, extra, cgService); rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
			}
			if after := countTestIncidents(t, pool); after != before {
				t.Errorf("an incident was created (%d -> %d)", before, after)
			}
		})
	}

	got, err := svc.GetIncidentCreateDefaults(repository.WithSystemIdentity(context.Background()))
	if err != nil || got.DefaultGroup == nil || got.DefaultGroup.ID != cgDefaultGroup || got.DefaultGroup.Name != "CG Default Team" {
		t.Errorf("create defaults = %+v err %v", got, err)
	}
}

// Dual-write: ServiceNow and the Postgres row get the same group; a refused
// group never reaches ServiceNow.
func TestIncidentCreateGroupIntegration_DualWrite(t *testing.T) {
	pool := newCreateGroupEnv(t)
	repo := repository.NewIncidentRepository(repository.NewScoped(pool))
	mirror := &cgMirror{}
	svc := service.WithIncidentDefaultService(service.NewIncidentServiceWithSNMirror(repo, repository.NewUserRepository(pool), mirror, nil, nil), cgDefaultService)

	if rec := postIncident(t, svc, "", cgGrouplessSvc); rec.Code != http.StatusCreated {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	group, _ := storedGroupAndNote(t, pool)
	if len(mirror.calls) != 1 || mirror.calls[0].AssignmentGroupID == nil || *mirror.calls[0].AssignmentGroupID != group || group != cgDefaultGroup {
		t.Errorf("ServiceNow got %+v, Postgres stored %q, want both %s", mirror.calls, group, cgDefaultGroup)
	}
	if rec := postIncident(t, svc, `,"assignmentGroupId":"`+cgInactiveGroup+`"`, cgService); rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if len(mirror.calls) != 1 {
		t.Errorf("ServiceNow was called %d times, want 1 (the refused create never reaches it)", len(mirror.calls))
	}
}
