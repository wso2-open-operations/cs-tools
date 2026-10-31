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

// PATCH and POST /change-requests sent as real JSON through the real handler,
// the real change request services and the real repository, against
// PostgreSQL. Only the previous system is stubbed (it records what it is sent).
// Covers:
//   - an explicit null clears a field, and is a PATCH on its own;
//   - a PATCH that only sets the service is accepted;
//   - priority: an unknown value is a 400 and so is planning, on PATCH and on
//     both creates (impact and risk too on create), and nothing is written;
//   - a PATCH that only sets the service or the hold is not sent to the
//     previous system.
//
// Skipped without CHANGE_REQUEST_TEST_DSN, the repository package's convention.
//
//	CHANGE_REQUEST_TEST_DSN=postgres://... go test ./internal/handler/ -run 'ChangeRequestPatchIntegration|ChangeRequestCreateLevelsIntegration' -count=1 -p 1

package handler

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
)

const (
	cpiSubject   = "cr-patch-null-service-priority integration test"
	cpiServiceID = "c9000000-0000-0000-0000-000000000001"
	cpiEmail     = "cpi-engineer@test.local"
	// cpiSentinelTitle is the title of the PATCH mirrorQuiet sends.
	cpiSentinelTitle = cpiSubject + " (sentinel)"
)

// cpiMirror stands in for the previous system's change request service. It
// records every PATCH it is sent (signalling sentinel when the sentinel arrives)
// and answers a create with a fresh id and number.
type cpiMirror struct {
	service.ChangeRequestService
	mu       sync.Mutex
	calls    []domain.PatchChangeRequestRequest
	creates  []domain.CreateChangeRequestRequest
	sentinel chan struct{}
}

func (m *cpiMirror) PatchChangeRequest(_ context.Context, id string, req domain.PatchChangeRequestRequest) (domain.PatchChangeRequestResponse, error) {
	m.mu.Lock()
	m.calls = append(m.calls, req)
	m.mu.Unlock()
	if req.Title != nil && *req.Title == cpiSentinelTitle {
		m.sentinel <- struct{}{}
	}
	return domain.PatchChangeRequestResponse{ChangeRequest: domain.ChangeRequest{SearchChangeRequestView: domain.SearchChangeRequestView{ID: id}}}, nil
}

func (m *cpiMirror) CreateChangeRequest(_ context.Context, req domain.CreateChangeRequestRequest) (domain.CreateChangeRequestResponse, error) {
	m.mu.Lock()
	m.creates = append(m.creates, req)
	m.mu.Unlock()
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return domain.CreateChangeRequestResponse{}, err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	h := hex.EncodeToString(b)
	resp := domain.CreateChangeRequestResponse{Message: "created"}
	resp.ChangeRequest.ID = fmt.Sprintf("%s-%s-%s-%s-%s", h[0:8], h[8:12], h[12:16], h[16:20], h[20:32])
	resp.ChangeRequest.Number = "CPI" + h[:12]
	resp.ChangeRequest.CreatedBy = cpiEmail
	return resp, nil
}

func (m *cpiMirror) received() []domain.PatchChangeRequestRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]domain.PatchChangeRequestRequest(nil), m.calls...)
}

func (m *cpiMirror) createsReceived() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.creates)
}

type changeRequestPatchEnv struct {
	// scoped runs the test's own reads and writes as the system, as the
	// cleanup does: the tables have row-level policies, so a plain pool query
	// by a role that does not bypass them (csm_app) sees and changes nothing.
	scoped *repository.Scoped
	sys    context.Context
	repo   repository.ChangeRequestRepository
	mirror *cpiMirror
	// mux serves PATCH and POST through the dual-write service (the create
	// writes the previous system first); plainMux serves POST through the
	// PostgreSQL-only service.
	mux, plainMux *http.ServeMux
	token         string
}

func newChangeRequestPatchEnv(t *testing.T) *changeRequestPatchEnv {
	t.Helper()
	dsn := os.Getenv("CHANGE_REQUEST_TEST_DSN")
	if dsn == "" {
		t.Skip("CHANGE_REQUEST_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	sys := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)
	// The test's change requests carry cpiSubject, or cpiSentinelTitle once
	// mirrorQuiet has renamed them.
	subjects := []string{cpiSubject, cpiSentinelTitle}
	cleanup := func() {
		_, _ = scoped.Exec(sys, `DELETE FROM sn_writeback_failures WHERE entity_type = 'change_request' AND entity_id IN (
			SELECT id FROM work_item WHERE subject = ANY($1))`, subjects)
		_, _ = scoped.Exec(sys, `DELETE FROM event_outbox WHERE entity_id IN (SELECT id FROM work_item WHERE subject = ANY($1))`, subjects)
		_, _ = scoped.Exec(sys, `DELETE FROM work_item WHERE subject = ANY($1)`, subjects)
		_, _ = pool.Exec(sys, `DELETE FROM service WHERE id = $1`, cpiServiceID)
	}
	cleanup()
	t.Cleanup(cleanup)
	if _, err := pool.Exec(sys, `INSERT INTO service (id, created_on, updated_on, created_by, updated_by, name, number)
		VALUES ($1, NOW(), NOW(), 'test', 'test', 'CPI Service', 'CPI-SVC-1')`, cpiServiceID); err != nil {
		t.Fatalf("seed service: %v", err)
	}

	repo := repository.NewChangeRequestRepository(scoped)
	users := repository.NewUserRepository(pool)
	mirror := &cpiMirror{sentinel: make(chan struct{}, 1)}
	dispatcher := service.NewSNWritebackDispatcher(repository.NewSNWritebackFailureRepository(pool))
	h := NewChangeRequestHandler(service.NewChangeRequestServiceWithSNWriteback(repo, users, mirror, dispatcher))
	plain := NewChangeRequestHandler(service.NewChangeRequestService(repo, users))

	// System identity stands in for the identity middleware's internal-caller
	// scope (the CSM BFF is an internal caller); UserIDToken is the real
	// middleware that carries the portal user's x-user-id-token.
	asSystem := func(next http.HandlerFunc) http.Handler {
		return middleware.UserIDToken(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next(w, r.WithContext(repository.WithSystemIdentity(r.Context())))
		}))
	}
	mux := http.NewServeMux()
	mux.Handle("PATCH /change-requests/{id}", asSystem(h.PatchChangeRequest))
	mux.Handle("POST /change-requests", asSystem(h.CreateChangeRequest))
	plainMux := http.NewServeMux()
	plainMux.Handle("POST /change-requests", asSystem(plain.CreateChangeRequest))

	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"email":"` + cpiEmail + `"}`))
	return &changeRequestPatchEnv{scoped: scoped, sys: sys, repo: repo, mirror: mirror, mux: mux, plainMux: plainMux, token: header + "." + payload + ".sig"}
}

func cpiStr(s string) *string { return &s }

func cpiDeref(s *string) string {
	if s == nil {
		return "NULL"
	}
	return *s
}

// create makes a Normal change request in New with the fields the null tests
// clear set: the plan, priority, category and the three free-text fields.
func (e *changeRequestPatchEnv) create(t *testing.T) string {
	t.Helper()
	normal := domain.ChangeRequestTypeNormal
	high := domain.ChangeRequestPriorityHigh
	software := domain.ChangeRequestCategorySoftware
	resp, err := e.repo.CreateChangeRequest(repository.WithSystemIdentity(context.Background()), domain.CreateChangeRequestRequest{
		Subject:                cpiSubject,
		Type:                   &normal,
		Priority:               &high,
		Category:               &software,
		ImplementationPlan:     cpiStr("<p>Roll the gateway one node at a time.</p>"),
		AffectedServicesText:   cpiStr("API gateway"),
		AffectedComponentsText: cpiStr("gateway-node-1"),
		RollbackDurationText:   cpiStr("30 minutes"),
	}, cpiEmail)
	if err != nil {
		t.Fatalf("CreateChangeRequest: %v", err)
	}
	return resp.ChangeRequest.ID
}

// send sends body to method path on mux the way the CSM BFF forwards it, and
// returns the status and the response's message.
func (e *changeRequestPatchEnv) send(t *testing.T, mux *http.ServeMux, method, path, body string) (int, string) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-user-id-token", e.token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	var out struct {
		Message string `json:"message"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out.Message
}

func (e *changeRequestPatchEnv) patch(t *testing.T, id, body string) (int, string) {
	t.Helper()
	return e.send(t, e.mux, http.MethodPatch, "/change-requests/"+id, body)
}

// mirrorQuiet returns every PATCH the previous system has been sent, once all
// of those dispatched for id have arrived. It sends one more PATCH for id that
// is mirrored (a title) and waits for it: the write-back dispatcher runs one
// change request's writes one at a time, in the order they were dispatched, so
// a write dispatched before it has finished by the time it arrives. The
// sentinel itself is left out.
func (e *changeRequestPatchEnv) mirrorQuiet(t *testing.T, id string) []domain.PatchChangeRequestRequest {
	t.Helper()
	if code, msg := e.patch(t, id, `{"title": "`+cpiSentinelTitle+`"}`); code != http.StatusOK {
		t.Fatalf("sentinel PATCH = %d %q, want 200", code, msg)
	}
	select {
	case <-e.mirror.sentinel:
	case <-time.After(5 * time.Second):
		t.Fatal("the sentinel PATCH never reached the previous system")
	}
	all := e.mirror.received()
	return all[:len(all)-1]
}

type cpiRow struct {
	ImplementationPlan, Priority, Category, AffectedServices, AffectedComponent, RollbackDuration, ServiceID *string
}

func (e *changeRequestPatchEnv) row(t *testing.T, id string) cpiRow {
	t.Helper()
	var r cpiRow
	if err := e.scoped.QueryRow(e.sys, `
		SELECT implementation_plan, priority::text, category::text, affected_services, affected_component,
		       rollback_duration, service_id::text
		  FROM change_request WHERE id = $1`, id,
	).Scan(&r.ImplementationPlan, &r.Priority, &r.Category, &r.AffectedServices, &r.AffectedComponent, &r.RollbackDuration, &r.ServiceID); err != nil {
		t.Fatalf("read back change_request: %v", err)
	}
	return r
}

func TestChangeRequestPatchIntegration_ExplicitNullClears(t *testing.T) {
	e := newChangeRequestPatchEnv(t)

	t.Run("each field on its own", func(t *testing.T) {
		for field, col := range map[string]func(cpiRow) *string{
			"implementationPlan":     func(r cpiRow) *string { return r.ImplementationPlan },
			"priority":               func(r cpiRow) *string { return r.Priority },
			"category":               func(r cpiRow) *string { return r.Category },
			"affectedServicesText":   func(r cpiRow) *string { return r.AffectedServices },
			"affectedComponentsText": func(r cpiRow) *string { return r.AffectedComponent },
			"rollbackDurationText":   func(r cpiRow) *string { return r.RollbackDuration },
		} {
			id := e.create(t)
			if col(e.row(t, id)) == nil {
				t.Fatalf("%s: the seeded change request has no value to clear", field)
			}
			if code, msg := e.patch(t, id, `{"`+field+`": null}`); code != http.StatusOK {
				t.Fatalf(`{"%s": null} = %d %q, want 200`, field, code, msg)
			}
			if got := col(e.row(t, id)); got != nil {
				t.Errorf(`{"%s": null} left %q stored`, field, *got)
			}
		}
	})

	t.Run("a null beside other fields clears it too", func(t *testing.T) {
		id := e.create(t)
		if code, msg := e.patch(t, id, `{"title": "`+cpiSubject+`", "implementationPlan": null}`); code != http.StatusOK {
			t.Fatalf("PATCH = %d %q, want 200", code, msg)
		}
		if got := e.row(t, id).ImplementationPlan; got != nil {
			t.Errorf("implementation_plan = %q, want NULL", *got)
		}
		cr, err := e.repo.GetChangeRequestByID(repository.WithSystemIdentity(context.Background()), id)
		if err != nil {
			t.Fatalf("GetChangeRequestByID: %v", err)
		}
		if cr.ImplementationPlan != nil {
			t.Errorf("the detail still shows the plan %q", *cr.ImplementationPlan)
		}
	})

	t.Run("an omitted field is left alone", func(t *testing.T) {
		id := e.create(t)
		before := e.row(t, id)
		if code, msg := e.patch(t, id, `{"title": "`+cpiSubject+`"}`); code != http.StatusOK {
			t.Fatalf("PATCH = %d %q, want 200", code, msg)
		}
		after := e.row(t, id)
		if after.ImplementationPlan == nil || *after.ImplementationPlan != *before.ImplementationPlan ||
			after.Priority == nil || *after.Priority != *before.Priority {
			t.Errorf("an omitted field changed: before %+v, after %+v", before, after)
		}
	})
}

func TestChangeRequestPatchIntegration_ServiceAloneIsAPatch(t *testing.T) {
	e := newChangeRequestPatchEnv(t)
	id := e.create(t)
	if code, msg := e.patch(t, id, `{"serviceId": "`+cpiServiceID+`"}`); code != http.StatusOK {
		t.Fatalf("PATCH {serviceId} = %d %q, want 200", code, msg)
	}
	if got := e.row(t, id).ServiceID; got == nil || *got != cpiServiceID {
		t.Fatalf("service_id = %s, want %s", cpiDeref(got), cpiServiceID)
	}
	if got := e.mirrorQuiet(t, id); len(got) != 0 {
		t.Errorf("a service-only PATCH was sent to the previous system: %+v", got)
	}
}

func TestChangeRequestPatchIntegration_PriorityIsValidated(t *testing.T) {
	e := newChangeRequestPatchEnv(t)
	id := e.create(t)
	for _, p := range []string{"planning", "urgent"} {
		code, msg := e.patch(t, id, `{"priority": "`+p+`"}`)
		if code != http.StatusBadRequest || msg != "priority must be one of critical, high, moderate or low" {
			t.Errorf("priority %q = %d %q, want the 400 naming the priorities", p, code, msg)
		}
		if got := e.row(t, id).Priority; got == nil || *got != "HIGH" {
			t.Errorf("priority %q changed the stored priority (HIGH) to %s", p, cpiDeref(got))
		}
	}
	if code, msg := e.patch(t, id, `{"priority": "low"}`); code != http.StatusOK {
		t.Fatalf("priority low = %d %q, want 200", code, msg)
	}
	if got := e.row(t, id).Priority; got == nil || *got != "LOW" {
		t.Fatalf("priority = %s, want LOW", cpiDeref(got))
	}

	// A row migrated with the previous system's priority 5 still reads back as
	// "planning": only the write refuses it.
	if tag, err := e.scoped.Exec(e.sys, `UPDATE change_request SET priority = 'PLANNING' WHERE id = $1`, id); err != nil || tag.RowsAffected() != 1 {
		t.Fatalf("seed PLANNING: %d rows, %v", tag.RowsAffected(), err)
	}
	cr, err := e.repo.GetChangeRequestByID(repository.WithSystemIdentity(context.Background()), id)
	if err != nil {
		t.Fatalf("GetChangeRequestByID: %v", err)
	}
	if cr.Priority == nil || *cr.Priority != "planning" {
		t.Errorf("a PLANNING row reads back as %s, want planning", cpiDeref(cr.Priority))
	}
}

func TestChangeRequestPatchIntegration_HoldIsNotMirrored(t *testing.T) {
	e := newChangeRequestPatchEnv(t)
	id := e.create(t)
	if code, msg := e.patch(t, id, `{"onHold": true, "onHoldReason": "waiting for the customer's window"}`); code != http.StatusOK {
		t.Fatalf("PATCH on hold = %d %q, want 200", code, msg)
	}
	if code, msg := e.patch(t, id, `{"onHold": false}`); code != http.StatusOK {
		t.Fatalf("PATCH off hold = %d %q, want 200", code, msg)
	}
	if got := e.mirrorQuiet(t, id); len(got) != 0 {
		t.Errorf("a hold-only PATCH was sent to the previous system: %+v", got)
	}
	var failures int
	if err := e.scoped.QueryRow(e.sys,
		`SELECT count(*) FROM sn_writeback_failures WHERE entity_type = 'change_request' AND entity_id = $1`, id,
	).Scan(&failures); err != nil {
		t.Fatalf("count write-back failures: %v", err)
	}
	if failures != 0 {
		t.Errorf("write-back failures = %d, want 0", failures)
	}
}

// Both creates refuse a priority, impact or risk the enum columns have no value
// for, or that this API does not set (planning), with a 400 naming the field,
// and write nothing (the create that writes the previous system first does not
// call it). A value they accept is stored.
func TestChangeRequestCreateLevelsIntegration_BothCreates(t *testing.T) {
	e := newChangeRequestPatchEnv(t)
	countRows := func() int {
		var n int
		if err := e.scoped.QueryRow(e.sys, `SELECT count(*) FROM work_item WHERE subject = $1`, cpiSubject).Scan(&n); err != nil {
			t.Fatalf("count change requests: %v", err)
		}
		return n
	}
	const priorityMsg = "priority must be one of critical, high, moderate or low"
	for name, mux := range map[string]*http.ServeMux{"PostgreSQL only": e.plainMux, "previous system first": e.mux} {
		t.Run(name, func(t *testing.T) {
			for field, want := range map[string]string{
				`"priority": "planning"`: priorityMsg,
				`"priority": "urgent"`:   priorityMsg,
				`"impact": "severe"`:     "impact must be one of high, medium or low",
				`"risk": "extreme"`:      "risk must be one of high, moderate or low",
			} {
				rowsBefore, createsBefore := countRows(), e.mirror.createsReceived()
				code, msg := e.send(t, mux, http.MethodPost, "/change-requests",
					`{"subject": "`+cpiSubject+`", "type": "normal", `+field+`}`)
				if code != http.StatusBadRequest || msg != want {
					t.Errorf("%s = %d %q, want 400 %q", field, code, msg, want)
				}
				if rowsBefore != countRows() || createsBefore != e.mirror.createsReceived() {
					t.Errorf("%s: a refused create wrote a change request or called the previous system", field)
				}
			}

			code, msg := e.send(t, mux, http.MethodPost, "/change-requests",
				`{"subject": "`+cpiSubject+`", "type": "normal", "priority": "low", "impact": "medium", "risk": "moderate"}`)
			if code != http.StatusCreated {
				t.Fatalf("a create with settable values = %d %q, want 201", code, msg)
			}
			var priority, impact, risk string
			if err := e.scoped.QueryRow(e.sys, `
				SELECT cr.priority::text, cr.impact::text, cr.risk::text
				  FROM change_request cr JOIN work_item wi ON wi.id = cr.id
				 WHERE wi.subject = $1 ORDER BY wi.created_on DESC LIMIT 1`, cpiSubject,
			).Scan(&priority, &impact, &risk); err != nil {
				t.Fatalf("read back the created change request: %v", err)
			}
			if priority != "LOW" || impact != "MEDIUM" || risk != "MODERATE" {
				t.Errorf("stored priority %s, impact %s, risk %s, want LOW, MEDIUM, MODERATE", priority, impact, risk)
			}
		})
	}
}
