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

// Regression test for digiops-cs#3318: the CSM portal's "Hold auto-closure"
// action (PATCH /cases/{id} {autocloseHoldUntil}) failed on this data source
// because the service rejected the field outright. UpdateCaseFields now stores
// the hold in the case-like extension table that owns the row
// (autoclosure_step = ON_HOLD, autoclosure_state_on = the UTC day), and
// GetCaseByID reads both columns back, so the portal's "On hold until ..." chip
// and the dialog's prefill see it. Runs against a real Postgres. Skipped
// without CASE_STATS_TEST_DSN.
//
//	CASE_STATS_TEST_DSN=postgres://... go test ./internal/repository/ -run AutocloseHoldIntegration

package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	ahCaseID         = "91000000-0000-0000-0000-000000000001"
	ahEngagementID   = "91000000-0000-0000-0000-000000000002"
	ahServiceReqID   = "91000000-0000-0000-0000-000000000003"
	ahAnnouncementID = "91000000-0000-0000-0000-000000000004"
	ahMissingID      = "91000000-0000-0000-0000-0000000000ff"
)

func seedAutocloseHoldFixture(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := repository.WithSystemIdentity(context.Background())
	scoped := repository.NewScoped(pool)

	ids := []any{ahCaseID, ahEngagementID, ahServiceReqID, ahAnnouncementID}
	cleanup := func() {
		_, _ = scoped.Exec(ctx, `DELETE FROM work_item WHERE id IN ($1, $2, $3, $4)`, ids...)
	}
	cleanup()
	t.Cleanup(cleanup)

	now := time.Now().UTC()
	mustExecScoped := func(sql string, args ...any) {
		t.Helper()
		if _, err := scoped.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed scoped (%.80s): %v", sql, err)
		}
	}
	insertWorkItem := func(id, number, workItemType string) {
		mustExecScoped(`INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type)
			VALUES ($1, $2, $2, 'test', 'test', $3, $3, 'autoclose hold integration fixture', $4::work_item_type_enum)`,
			id, now, number, workItemType)
	}

	insertWorkItem(ahCaseID, "AH-TEST-0001", "CASE")
	mustExecScoped(`INSERT INTO "case" (id, state) VALUES ($1, 'OPEN')`, ahCaseID)
	insertWorkItem(ahEngagementID, "AH-TEST-0002", "ENGAGEMENT")
	mustExecScoped(`INSERT INTO engagement (id, state) VALUES ($1, 'OPEN')`, ahEngagementID)
	insertWorkItem(ahServiceReqID, "AH-TEST-0003", "SERVICE_REQUEST")
	mustExecScoped(`INSERT INTO service_request (id, state) VALUES ($1, 'OPEN')`, ahServiceReqID)
	insertWorkItem(ahAnnouncementID, "AH-TEST-0004", "ANNOUNCEMENT")
	mustExecScoped(`INSERT INTO announcement (id, announcement_type) VALUES ($1, 'GENERAL')`, ahAnnouncementID)
}

// TestAutocloseHoldIntegration_HoldRoundTripsOnEveryHoldableType proves the
// write lands in the right extension table for each type that carries the
// columns, and that the detail read returns it, as the UTC day at midnight no
// matter what time of day the caller's instant carries.
func TestAutocloseHoldIntegration_HoldRoundTripsOnEveryHoldableType(t *testing.T) {
	pool := caseStatsPool(t)
	seedAutocloseHoldFixture(t, pool)
	repo := repository.NewCaseRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())

	wantDay := time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC)

	// The day is the UTC date of the instant (see UpdateCaseRequest.AutocloseHoldUntil).
	instants := map[string]time.Time{
		// What the portal sends now: the chosen day at 00:00 UTC.
		"midnight UTC, as the portal sends it": time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC),
		// What an older portal sent for 22 Oct in Sri Lanka (end of that local
		// day as a UTC instant): still the 22nd in UTC, so a deploy that
		// reaches this service first changes nothing for it.
		"end of the day east of UTC, as an older portal sends it": time.Date(2026, 10, 22, 18, 29, 0, 0, time.UTC),
	}

	for instantName, holdUntil := range instants {
		for name, id := range map[string]string{
			"case":            ahCaseID,
			"engagement":      ahEngagementID,
			"service request": ahServiceReqID,
		} {
			t.Run(instantName+"/"+name, func(t *testing.T) {
				// A fresh row per run, so "no hold yet" is true each time.
				seedAutocloseHoldFixture(t, pool)
				before, err := repo.GetCaseByID(ctx, id, repository.SearchScope{Unrestricted: true})
				if err != nil {
					t.Fatalf("GetCaseByID before: %v", err)
				}
				if before.AutoclosureStep != nil || before.AutoclosureStateTime != nil {
					t.Fatalf("fresh case already has a hold: step=%v time=%v", before.AutoclosureStep, before.AutoclosureStateTime)
				}

				holdUntil := holdUntil
				if _, err := repo.UpdateCaseFields(ctx, domain.UpdateCaseRequest{ID: id, AutocloseHoldUntil: &holdUntil}, "", "jane.doe@example.com"); err != nil {
					t.Fatalf("UpdateCaseFields: %v", err)
				}

				cv, err := repo.GetCaseByID(ctx, id, repository.SearchScope{Unrestricted: true})
				if err != nil {
					t.Fatalf("GetCaseByID after: %v", err)
				}
				if cv.AutoclosureStep == nil || *cv.AutoclosureStep != "ON_HOLD" {
					t.Errorf("AutoclosureStep = %v, want ON_HOLD", cv.AutoclosureStep)
				}
				if cv.AutoclosureStateTime == nil || !cv.AutoclosureStateTime.Equal(wantDay) {
					t.Errorf("AutoclosureStateTime = %v, want %v", cv.AutoclosureStateTime, wantDay)
				}
			})
		}
	}
}

// TestAutocloseHoldIntegration_ExtendingAHoldMovesTheDate proves a second hold
// replaces the first rather than failing or keeping the old date.
func TestAutocloseHoldIntegration_ExtendingAHoldMovesTheDate(t *testing.T) {
	pool := caseStatsPool(t)
	seedAutocloseHoldFixture(t, pool)
	repo := repository.NewCaseRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())

	first := time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC)
	second := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)
	for _, d := range []time.Time{first, second} {
		d := d
		if _, err := repo.UpdateCaseFields(ctx, domain.UpdateCaseRequest{ID: ahCaseID, AutocloseHoldUntil: &d}, "", "jane.doe@example.com"); err != nil {
			t.Fatalf("UpdateCaseFields(%v): %v", d, err)
		}
	}
	cv, err := repo.GetCaseByID(ctx, ahCaseID, repository.SearchScope{Unrestricted: true})
	if err != nil {
		t.Fatalf("GetCaseByID: %v", err)
	}
	if cv.AutoclosureStateTime == nil || !cv.AutoclosureStateTime.Equal(second) {
		t.Errorf("AutoclosureStateTime = %v, want %v", cv.AutoclosureStateTime, second)
	}
}

// TestAutocloseHoldIntegration_HoldBesideAnotherFieldWritesBoth proves the hold
// is a plain combinable field: sent with a subject change, both land and the
// request is one transaction.
func TestAutocloseHoldIntegration_HoldBesideAnotherFieldWritesBoth(t *testing.T) {
	pool := caseStatsPool(t)
	seedAutocloseHoldFixture(t, pool)
	repo := repository.NewCaseRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())

	holdUntil := time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC)
	subject := "Subject edited with the hold"
	if _, err := repo.UpdateCaseFields(ctx, domain.UpdateCaseRequest{ID: ahCaseID, AutocloseHoldUntil: &holdUntil, Subject: &subject}, "", "jane.doe@example.com"); err != nil {
		t.Fatalf("UpdateCaseFields: %v", err)
	}
	cv, err := repo.GetCaseByID(ctx, ahCaseID, repository.SearchScope{Unrestricted: true})
	if err != nil {
		t.Fatalf("GetCaseByID: %v", err)
	}
	if cv.Subject != subject {
		t.Errorf("Subject = %q, want %q", cv.Subject, subject)
	}
	if cv.AutoclosureStep == nil || *cv.AutoclosureStep != "ON_HOLD" {
		t.Errorf("AutoclosureStep = %v, want ON_HOLD", cv.AutoclosureStep)
	}
}

// TestAutocloseHoldIntegration_AnnouncementAndUnknownIdAreRefused proves the two
// edges: an announcement has no auto-closure columns, so it is a 400 (not a
// silent no-op that reports success), and an unknown id is a 404.
func TestAutocloseHoldIntegration_AnnouncementAndUnknownIdAreRefused(t *testing.T) {
	pool := caseStatsPool(t)
	seedAutocloseHoldFixture(t, pool)
	repo := repository.NewCaseRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())
	holdUntil := time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC)

	_, err := repo.UpdateCaseFields(ctx, domain.UpdateCaseRequest{ID: ahAnnouncementID, AutocloseHoldUntil: &holdUntil}, "", "jane.doe@example.com")
	var ve *apierror.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("announcement: got %T (%v), want *apierror.ValidationError", err, err)
	}

	_, err = repo.UpdateCaseFields(ctx, domain.UpdateCaseRequest{ID: ahMissingID, AutocloseHoldUntil: &holdUntil}, "", "jane.doe@example.com")
	var nf *apierror.NotFoundError
	if !errors.As(err, &nf) {
		t.Errorf("unknown id: got %T (%v), want *apierror.NotFoundError", err, err)
	}
}

// TestAutocloseHoldIntegration_RefusedHoldWritesNothing proves the transaction
// rolls back as a unit: a hold on an announcement sent beside a subject change
// leaves the subject untouched.
func TestAutocloseHoldIntegration_RefusedHoldWritesNothing(t *testing.T) {
	pool := caseStatsPool(t)
	seedAutocloseHoldFixture(t, pool)
	repo := repository.NewCaseRepository(repository.NewScoped(pool))
	ctx := repository.WithSystemIdentity(context.Background())

	holdUntil := time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC)
	subject := "must not be written"
	if _, err := repo.UpdateCaseFields(ctx, domain.UpdateCaseRequest{ID: ahAnnouncementID, AutocloseHoldUntil: &holdUntil, Subject: &subject}, "", "jane.doe@example.com"); err == nil {
		t.Fatal("expected the hold on an announcement to be refused")
	}
	cv, err := repo.GetCaseByID(ctx, ahAnnouncementID, repository.SearchScope{Unrestricted: true})
	if err != nil {
		t.Fatalf("GetCaseByID: %v", err)
	}
	if cv.Subject == subject {
		t.Errorf("Subject was written (%q) even though the request was refused", cv.Subject)
	}
}
