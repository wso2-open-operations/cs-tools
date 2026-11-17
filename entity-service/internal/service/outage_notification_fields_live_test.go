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
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// TestOutageNotificationFieldsLive proves the two email opt-ins and the two
// email labels round-trip through the outage API AND reach the sweeps that
// send the emails -- the gap that left a portal-created outage unable to
// trigger either email at all.
func TestOutageNotificationFieldsLive(t *testing.T) {
	dsn := os.Getenv("PG_OUTAGE_TEST_DSN")
	if dsn == "" {
		t.Skip("PG_OUTAGE_TEST_DSN not set; this test needs a real database")
	}
	ctx := repository.WithSystemIdentity(context.Background())
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	// t.Cleanup, not defer: the per-outage deletes below are cleanups too and
	// run last-in-first-out, so the pool must be closed by one registered first.
	t.Cleanup(pool.Close)
	svc := NewOutageService(repository.NewOutageRepository(repository.NewScoped(pool)))

	var offeringID string
	if err := pool.QueryRow(ctx, `
SELECT so.id::text FROM service_offering so
 WHERE NOT EXISTS (SELECT 1 FROM cloud_monitor m WHERE m.service_offering_id = so.id)
 LIMIT 1`).Scan(&offeringID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			t.Skip("needs a service offering without a cloud monitor; this database has none")
		}
		t.Fatalf("find unmonitored offering: %v", err)
	}
	begin := time.Now().UTC().Add(-10 * time.Minute).Format(time.RFC3339)
	yes := true

	create := func(req domain.CreateOutageRequest) domain.Outage {
		t.Helper()
		req.Type, req.Begin, req.ConfigurationItemID = domain.OutageTypeOutage, begin, &offeringID
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

	// Ticked, with both labels: the response carries them back.
	on := create(domain.CreateOutageRequest{
		ShortDescription:           "notify-fields live test: opted in",
		NotifyInternalStakeholders: &yes,
		OutageCommunication:        &yes,
		Impact:                     strPtrLocal("  2 - High "),
		State:                      strPtrLocal("Investigating"),
	})
	if !on.NotifyInternalStakeholders || !on.OutageCommunication {
		t.Errorf("opt-ins not stored: notify=%v comm=%v", on.NotifyInternalStakeholders, on.OutageCommunication)
	}
	if derefLocal(on.Impact) != "2 - High" || derefLocal(on.State) != "Investigating" {
		t.Errorf("labels: impact=%q state=%q, want trimmed %q / %q", derefLocal(on.Impact), derefLocal(on.State), "2 - High", "Investigating")
	}

	// Left unticked: false and unset, exactly as before this change.
	off := create(domain.CreateOutageRequest{ShortDescription: "notify-fields live test: not opted in"})
	if off.NotifyInternalStakeholders || off.OutageCommunication || off.Impact != nil || off.State != nil {
		t.Errorf("an untouched create must stay opted out with no labels, got %+v", off)
	}

	// *** THE POINT OF THE CHANGE: the sweeps now see the ticked outage. ***
	comm, err := repository.NewOutageCommunicationRepository(pool).PendingOutages(ctx, 1000)
	if err != nil {
		t.Fatalf("communication PendingOutages: %v", err)
	}
	var sawOn, sawOff bool
	for _, o := range comm {
		switch o.OutageID {
		case on.ID:
			sawOn = true
			if o.Impact != "2 - High" || o.State != "Investigating" {
				t.Errorf("communication sweep labels: impact=%q state=%q", o.Impact, o.State)
			}
		case off.ID:
			sawOff = true
		}
	}
	if !sawOn || sawOff {
		t.Errorf("communication sweep: saw opted-in=%v (want true), saw opted-out=%v (want false)", sawOn, sawOff)
	}
	notif, err := repository.NewOutageNotificationRepository(pool).PendingOutages(ctx, 1000)
	if err != nil {
		t.Fatalf("notification PendingOutages: %v", err)
	}
	sawOn, sawOff = false, false
	for _, o := range notif {
		sawOn = sawOn || o.OutageID == on.ID
		sawOff = sawOff || o.OutageID == off.ID
	}
	if !sawOn || sawOff {
		t.Errorf("notification sweep: saw opted-in=%v (want true), saw opted-out=%v (want false)", sawOn, sawOff)
	}

	// A patch that only ticks a box is a real change, not an empty patch.
	patched, err := svc.UpdateOutage(ctx, domain.PatchOutageRequest{ID: off.ID, OutageCommunication: &yes})
	if err != nil {
		t.Fatalf("tick outageCommunication by PATCH: %v", err)
	}
	if !patched.Outage.OutageCommunication || patched.Outage.NotifyInternalStakeholders {
		t.Errorf("after PATCH: comm=%v notify=%v, want true/false", patched.Outage.OutageCommunication, patched.Outage.NotifyInternalStakeholders)
	}

	// An empty string clears a label back to unset (NULL).
	cleared, err := svc.UpdateOutage(ctx, domain.PatchOutageRequest{ID: on.ID, Impact: strPtrLocal("")})
	if err != nil {
		t.Fatalf("clear impact: %v", err)
	}
	if cleared.Outage.Impact != nil || derefLocal(cleared.Outage.State) != "Investigating" {
		t.Errorf("clear impact: impact=%v state=%q, want nil / unchanged", cleared.Outage.Impact, derefLocal(cleared.Outage.State))
	}

	// Over the column width is a 400 naming the field, not a database 500.
	_, err = svc.UpdateOutage(ctx, domain.PatchOutageRequest{ID: on.ID, State: strPtrLocal(strings.Repeat("x", 41))})
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) || !strings.Contains(ve.Msg, "state") {
		t.Errorf("41-character state: got %v, want a ValidationError naming state", err)
	}
}
