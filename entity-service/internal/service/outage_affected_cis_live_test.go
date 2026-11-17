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

package service

import (
	"context"
	"errors"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

func TestOutageAffectedCIsLive(t *testing.T) {
	dsn := os.Getenv("PG_OUTAGE_TEST_DSN")
	if dsn == "" {
		t.Skip("PG_OUTAGE_TEST_DSN not set; this test needs a real database")
	}
	ctx := repository.WithSystemIdentity(context.Background())
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	svc := NewOutageService(repository.NewOutageRepository(repository.NewScoped(pool)))

	// Two offerings with a monitor (A, C) and two without (main, B).
	// array_agg is NULL (or has fewer than two elements) when the database
	// does not hold that many offerings, so these scan into pointers and the
	// test skips rather than fails on a database without that ambient data.
	var mainP, bP, aP, cP *string
	if err := pool.QueryRow(ctx, `
SELECT (array_agg(so.id::text ORDER BY so.id))[1], (array_agg(so.id::text ORDER BY so.id))[2]
  FROM service_offering so
 WHERE NOT EXISTS (SELECT 1 FROM cloud_monitor m WHERE m.service_offering_id = so.id)`).Scan(&mainP, &bP); err != nil {
		t.Fatalf("pick unmonitored offerings: %v", err)
	}
	if mainP == nil || bP == nil {
		t.Skip("needs two service offerings without a cloud monitor; this database does not have them")
	}
	if err := pool.QueryRow(ctx, `
SELECT (array_agg(x ORDER BY x))[1], (array_agg(x ORDER BY x))[2]
  FROM (SELECT DISTINCT m.service_offering_id::text x FROM cloud_monitor m
         WHERE m.cloud_offering IS NOT NULL
           AND NOT EXISTS (SELECT 1 FROM outage_affected_ci ac JOIN outage o ON o.id = ac.outage_id
                            WHERE ac.ci_id = m.service_offering_id AND o.end_on IS NULL)) s`).Scan(&aP, &cP); err != nil {
		t.Fatalf("pick monitored offerings: %v", err)
	}
	if aP == nil || cP == nil {
		t.Skip("needs two monitored service offerings with no open outage; this database does not have them")
	}
	main, b, a, c := *mainP, *bP, *aP, *cP
	yes := true
	create := func(req domain.CreateOutageRequest) domain.Outage {
		t.Helper()
		req.Type = domain.OutageTypeOutage
		req.Begin = time.Now().UTC().Add(-10 * time.Minute).Format(time.RFC3339)
		req.ShortDescription = "affected-cis live test"
		req.ConfigurationItemID = &main
		out, err := svc.CreateOutage(ctx, req)
		if err != nil {
			t.Fatalf("CreateOutage: %v", err)
		}
		t.Cleanup(func() {
			if _, err := pool.Exec(context.Background(), "DELETE FROM outage WHERE id = $1::uuid", out.Outage.ID); err != nil {
				t.Errorf("CLEANUP FAILED, delete outage %s by hand: %v", out.Outage.ID, err)
			}
		})
		return out.Outage
	}
	ids := func(o domain.Outage) []string {
		var out []string
		for _, ci := range o.AffectedConfigurationItems {
			out = append(out, ci.ID)
		}
		sort.Strings(out)
		return out
	}
	want := func(v ...string) []string { sort.Strings(v); return v }
	monitor := func(offering string) string {
		var st string
		if err := pool.QueryRow(ctx, "SELECT status::text FROM cloud_monitor WHERE service_offering_id = $1::uuid LIMIT 1", offering).Scan(&st); err != nil {
			t.Fatalf("read monitor: %v", err)
		}
		return st
	}
	setMonitor := func(offering, st string) {
		if _, err := pool.Exec(ctx, "UPDATE cloud_monitor SET status = $2::cloud_monitor_status_enum WHERE service_offering_id = $1::uuid", offering, st); err != nil {
			t.Fatalf("set monitor: %v", err)
		}
	}
	t.Cleanup(func() { setMonitor(a, "OPERATIONAL"); setMonitor(c, "OPERATIONAL") })

	// A monitored affected CI makes the outage public: consent required.
	_, err = svc.CreateOutage(ctx, domain.CreateOutageRequest{Type: domain.OutageTypeOutage,
		Begin: time.Now().UTC().Format(time.RFC3339), ShortDescription: "x", ConfigurationItemID: &main,
		AffectedConfigurationItemIDs: []string{a}})
	var conflict *apierror.ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("monitored affected CI without consent: got %v, want 409", err)
	}

	// Create with A and B (B twice: de-duplicated).
	o := create(domain.CreateOutageRequest{AffectedConfigurationItemIDs: []string{a, b, b}, AcknowledgePublicPublication: &yes})
	if got := ids(o); len(got) != 2 || got[0] != want(a, b)[0] || got[1] != want(a, b)[1] {
		t.Fatalf("created affected = %v, want %v", got, want(a, b))
	}

	// The drainer would have turned A red; simulate it, then remove A, add C.
	setMonitor(a, "PARTIAL_OUTAGE")
	var updatedBefore time.Time
	_ = pool.QueryRow(ctx, "SELECT updated_on FROM outage WHERE id = $1::uuid", o.ID).Scan(&updatedBefore)
	next := []string{b, c}
	patched, err := svc.UpdateOutage(ctx, domain.PatchOutageRequest{ID: o.ID, AffectedConfigurationItemIDs: &next, AcknowledgePublicPublication: &yes})
	if err != nil {
		t.Fatalf("replace affected: %v", err)
	}
	if got := ids(patched.Outage); len(got) != 2 || got[0] != want(b, c)[0] || got[1] != want(b, c)[1] {
		t.Errorf("after replace = %v, want %v", got, want(b, c))
	}
	if st := monitor(a); st != "OPERATIONAL" {
		t.Errorf("removed CI's monitor = %s, want OPERATIONAL (nothing else would ever reset it)", st)
	}
	var updatedAfter time.Time
	_ = pool.QueryRow(ctx, "SELECT updated_on FROM outage WHERE id = $1::uuid", o.ID).Scan(&updatedAfter)
	if !updatedAfter.Equal(updatedBefore) {
		t.Error("an affected-CI-only edit moved outage.updated_on, which would send an internal Update email")
	}
	var outboxed int
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM event_outbox WHERE entity_type = 'outage_affected_ci'
	   AND entity_id IN (SELECT id FROM outage_affected_ci WHERE outage_id = $1::uuid AND ci_id = $2::uuid)`, o.ID, c).Scan(&outboxed)
	if outboxed == 0 {
		t.Error("adding C wrote no outbox row, so the status page would never hear of it")
	}

	// Re-saving the same set needs no consent: nothing new publishes.
	if _, err := svc.UpdateOutage(ctx, domain.PatchOutageRequest{ID: o.ID, AffectedConfigurationItemIDs: &next}); err != nil {
		t.Errorf("re-save unchanged affected set without consent: %v", err)
	}

	// A ServiceNow-synced affected CI can be a service, not an offering. It
	// shows on the read, and re-saving the list with it kept must not fail.
	var svcID string
	if err := pool.QueryRow(ctx, "SELECT id::text FROM service LIMIT 1").Scan(&svcID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			t.Skip("the rest of this test needs a row in the service table; this database has none")
		}
		t.Fatalf("pick a service: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO outage_affected_ci (id, outage_id, ci_id, created_on, updated_on, created_by, updated_by)
	   VALUES (gen_random_uuid(), $1::uuid, $2::uuid, NOW(), NOW(), 'sync', 'sync')`, o.ID, svcID); err != nil {
		t.Fatalf("seed a synced service-class affected CI: %v", err)
	}
	withSvc := []string{b, c, svcID}
	resaved, err := svc.UpdateOutage(ctx, domain.PatchOutageRequest{ID: o.ID, AffectedConfigurationItemIDs: &withSvc})
	if err != nil {
		t.Fatalf("re-save keeping a synced service-class CI: %v", err)
	}
	var sawService bool
	for _, ci := range resaved.Outage.AffectedConfigurationItems {
		if ci.ID == svcID && ci.ClassName == "cmdb_ci_service" {
			sawService = true
		}
	}
	if !sawService {
		t.Errorf("synced service-class affected CI missing from the read: %+v", resaved.Outage.AffectedConfigurationItems)
	}

	// Another ongoing outage on C keeps C red when this one drops it.
	other := create(domain.CreateOutageRequest{AffectedConfigurationItemIDs: []string{c}, AcknowledgePublicPublication: &yes})
	_ = other
	setMonitor(c, "PARTIAL_OUTAGE")
	empty := []string{}
	cleared, err := svc.UpdateOutage(ctx, domain.PatchOutageRequest{ID: o.ID, AffectedConfigurationItemIDs: &empty})
	if err != nil {
		t.Fatalf("clear affected: %v", err)
	}
	if len(cleared.Outage.AffectedConfigurationItems) != 0 {
		t.Errorf("after clear = %v, want none", ids(cleared.Outage))
	}
	if st := monitor(c); st != "PARTIAL_OUTAGE" {
		t.Errorf("C is still affected by another ongoing outage, but its monitor became %s", st)
	}

	// An id that is not a service offering is a 400, not a dangling row.
	bogus := []string{"00000000-0000-0000-0000-000000000001"}
	_, err = svc.UpdateOutage(ctx, domain.PatchOutageRequest{ID: o.ID, AffectedConfigurationItemIDs: &bogus})
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("unknown affected CI: got %v, want a ValidationError", err)
	}
}
