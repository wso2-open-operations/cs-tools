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
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// TestPGOutageServiceLive exercises the whole Postgres outage path against a
// real database: create, read back, search, close, journal.
//
// Skipped unless PG_OUTAGE_TEST_DSN is set, so an ordinary go test never
// opens a connection.
//
// *** IT DELETES THE OUTAGE IT CREATES. *** An outage left behind on an
// in-scope service offering becomes a live cloud status candidate, and the
// next sweep would post a real outage_begin for it to the shared dashboard.
// The cleanup is not tidiness, it is the difference between a test and an
// incident on a public page.
func TestPGOutageServiceLive(t *testing.T) {
	dsn := os.Getenv("PG_OUTAGE_TEST_DSN")
	if dsn == "" {
		t.Skip("PG_OUTAGE_TEST_DSN not set; this test needs a real database")
	}

	// WithSystemIdentity: the outage repository reads work_item (RLS-protected)
	// through the caller's identity, and this test stands in for an internal
	// caller, as the CSM portal is.
	ctx := repository.WithSystemIdentity(context.Background())
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	repo := repository.NewOutageRepository(repository.NewScoped(pool))
	svc := NewOutageService(repo)

	// An offering with no cloud monitor, so the publication gate does not
	// fire and this test never creates something publicly visible.
	var offeringID string
	err = pool.QueryRow(ctx, `
SELECT so.id::text FROM service_offering so
 WHERE NOT EXISTS (SELECT 1 FROM cloud_monitor m WHERE m.service_offering_id = so.id)
 LIMIT 1`).Scan(&offeringID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			t.Skip("needs a service offering without a cloud monitor; this database has none")
		}
		t.Fatalf("find unmonitored offering: %v", err)
	}

	begin := time.Now().UTC().Add(-90 * time.Minute).Format(time.RFC3339)
	created, err := svc.CreateOutage(ctx, domain.CreateOutageRequest{
		Type:                  domain.OutageTypeOutage,
		Begin:                 begin,
		ShortDescription:      "pg-outage-service live test",
		ConfigurationItemID:   &offeringID,
		ExternalCommunication: strPtrLocal("seeded external note"),
	})
	if err != nil {
		t.Fatalf("CreateOutage: %v", err)
	}
	id := created.Outage.ID
	defer func() {
		if _, err := pool.Exec(context.Background(), "DELETE FROM outage WHERE id = $1::uuid", id); err != nil {
			t.Errorf("CLEANUP FAILED, delete outage %s by hand: %v", id, err)
		}
	}()

	t.Logf("created %s number=%s status=%s", id, created.Outage.Number, derefLocal(created.Outage.Status))

	if created.Outage.Number == "" {
		t.Error("number was not generated")
	}
	if got := derefLocal(created.Outage.Status); got != string(domain.OutageStatusInProgress) {
		t.Errorf("status: got %q, want in_progress", got)
	}
	if created.Outage.PublishesToStatusPage {
		t.Error("an unmonitored offering must not publish")
	}

	// The seeded external note must be in the journal, not only in the column.
	detail, err := svc.GetOutageByID(ctx, id)
	if err != nil {
		t.Fatalf("GetOutageByID: %v", err)
	}
	if detail.CommunicationCounts.External != 1 {
		t.Errorf("external communication count: got %d, want 1", detail.CommunicationCounts.External)
	}

	// Closing is a PATCH of end, and it must flip the derived status and
	// produce a duration.
	end := time.Now().UTC().Format(time.RFC3339)
	endPtr := &end
	patched, err := svc.UpdateOutage(ctx, domain.PatchOutageRequest{ID: id, End: &endPtr})
	if err != nil {
		t.Fatalf("UpdateOutage: %v", err)
	}
	if got := derefLocal(patched.Outage.Status); got != string(domain.OutageStatusResolved) {
		t.Errorf("status after close: got %q, want resolved", got)
	}
	if patched.Outage.Duration == nil {
		t.Error("duration must be derived once end is set")
	} else {
		t.Logf("duration rendered as %q", *patched.Outage.Duration)
	}

	// Explicit null reopens -- the pointer-to-pointer case that is easy to
	// collapse into a no-op.
	var nilEnd *string
	reopened, err := svc.UpdateOutage(ctx, domain.PatchOutageRequest{ID: id, End: &nilEnd})
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if got := derefLocal(reopened.Outage.Status); got != string(domain.OutageStatusInProgress) {
		t.Errorf("status after reopen: got %q, want in_progress", got)
	}

	// A patch with nothing set is a validation error, not a silent no-op.
	if _, err := svc.UpdateOutage(ctx, domain.PatchOutageRequest{ID: id}); err == nil {
		t.Error("an empty patch must be rejected")
	} else if _, ok := err.(*apierror.ValidationError); !ok {
		t.Errorf("empty patch: got %T, want *apierror.ValidationError", err)
	}

	// *** THE PORTAL SENDS "YYYY-MM-DD HH:mm:ss", NOT RFC3339. *** The
	// repository used to parse RFC3339 only while create accepted both, so
	// closing an outage from the portal failed with "invalid end" while
	// creating one worked. This is that exact request.
	portalEnd := time.Now().UTC().Format("2006-01-02 15:04:05")
	portalEndPtr := &portalEnd
	closedAgain, err := svc.UpdateOutage(ctx, domain.PatchOutageRequest{ID: id, End: &portalEndPtr})
	if err != nil {
		t.Fatalf("close with the portal timestamp format: %v", err)
	}
	if got := derefLocal(closedAgain.Outage.Status); got != string(domain.OutageStatusResolved) {
		t.Errorf("status after portal-format close: got %q, want resolved", got)
	}

	// An end before the STORED begin must be refused. Only the end is sent,
	// so this passes unless the effective interval is checked against the
	// stored begin rather than against the submitted fields alone.
	bad := time.Now().UTC().Add(-48 * time.Hour).Format("2006-01-02 15:04:05")
	badPtr := &bad
	if _, err := svc.UpdateOutage(ctx, domain.PatchOutageRequest{ID: id, End: &badPtr}); err == nil {
		t.Error("an end before the stored begin must be rejected")
	} else if _, ok := err.(*apierror.ValidationError); !ok {
		t.Errorf("end before begin: got %T (%v), want *apierror.ValidationError", err, err)
	}

	found, err := svc.SearchOutages(ctx, domain.SearchOutagesRequest{
		Filters:    domain.SearchOutagesFilters{SearchTerm: created.Outage.Number},
		Pagination: domain.Pagination{Limit: 10},
	})
	if err != nil {
		t.Fatalf("SearchOutages: %v", err)
	}
	if found.Total != 1 {
		t.Errorf("search by number: got %d results, want 1", found.Total)
	}
	if !found.BeginFromDefaulted {
		t.Error("beginFromDefaulted must report the implicit six-month bound")
	}

	meta, err := svc.GetOutageMetadata(ctx)
	if err != nil {
		t.Fatalf("GetOutageMetadata: %v", err)
	}
	if len(meta.Types) != 3 || len(meta.StatusPageClouds) == 0 {
		t.Errorf("metadata looks wrong: %d types, %d clouds", len(meta.Types), len(meta.StatusPageClouds))
	}
	t.Logf("metadata clouds: %v", meta.StatusPageClouds)
}

func strPtrLocal(s string) *string { return &s }

func derefLocal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
