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

// What a production ticket looks like around a type transfer: other tickets that
// point at it, change requests and child cases hanging off it, a lot of history,
// and a Migration ticket that has to keep working as one afterwards. Same setup
// and skip rule as case_type_transfer_repo_integration_test.go.

package repository_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	ctRelatedFromID = "92000000-0000-0000-0000-000000000010" // a case that names another as its related case
	ctChildID       = "92000000-0000-0000-0000-000000000011"
	ctTagID         = "92000000-0000-0000-0000-000000000012"
	ctBulkMarker    = "ct-bulk"
)

// cleanupExtras removes what a test adds beyond the fixture. It is registered
// after the fixture's own cleanup, so it runs first: the children and history
// reference the work items the fixture then deletes.
func (f *ctFixture) cleanupExtras(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		for _, sql := range []string{
			`DELETE FROM comment WHERE created_by = '` + ctBulkMarker + `'`,
			`DELETE FROM time_card WHERE created_by = '` + ctBulkMarker + `'`,
			`DELETE FROM case_attachment WHERE storage_key = '` + ctBulkMarker + `'`,
			`DELETE FROM work_item_watcher WHERE work_item_id IN ('` + ctIncidentID + `', '` + ctQueryID + `')`,
			`DELETE FROM work_item_tag WHERE tag_id = '` + ctTagID + `'`,
			`DELETE FROM tag WHERE id = '` + ctTagID + `'`,
			`DELETE FROM change_request WHERE id IN (SELECT id FROM work_item WHERE created_by = '` + ctBulkMarker + `')`,
			`DELETE FROM work_item WHERE created_by = '` + ctBulkMarker + `'`,
			`DELETE FROM work_item WHERE id IN ('` + ctRelatedFromID + `', '` + ctChildID + `')`,
		} {
			_, _ = f.scoped.Exec(f.ctx, sql)
		}
	})
}

func (f *ctFixture) count(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := f.scoped.QueryRow(f.ctx, sql, args...).Scan(&n); err != nil {
		t.Fatalf("count (%.80s): %v", sql, err)
	}
	return n
}

// relatedKeyTargetsCase reports whether the database still has the original
// foreign key from "case".related_case_id into "case" (migration 0211 not applied).
func (f *ctFixture) relatedKeyTargetsCase(t *testing.T) bool {
	t.Helper()
	var old bool
	if err := f.scoped.QueryRow(f.ctx, `
		SELECT EXISTS (
		  SELECT 1 FROM pg_constraint con
		   WHERE con.conrelid = to_regclass('"case"') AND con.contype = 'f'
		     AND con.confrelid = to_regclass('"case"')
		     AND con.conkey = ARRAY[(SELECT attnum FROM pg_attribute
		                              WHERE attrelid = to_regclass('"case"') AND attname = 'related_case_id' AND NOT attisdropped)])`).Scan(&old); err != nil {
		t.Fatalf("inspect the related case key: %v", err)
	}
	return old
}

// The use case: a customer opens an incident or a query, it turns out to be a
// migration, and THAT ticket becomes an engagement. Tickets related to it are not
// converted and keep their link to it. Tickets it relates to are left as they are.
//
// This runs against a database with or without migration 0211 and asserts what is
// right for each: with it, the conversion goes through and the related ticket is
// byte-for-byte unchanged and still related; without it, replacing the "case" row
// would clear the other ticket's link, so the conversion is refused (before the
// remote step) and nothing is changed.
func TestCaseTypeTransferIntegration_OnlyTheTicketConvertsRelatedTicketsStayRelated(t *testing.T) {
	f := newCTFixture(t)
	f.cleanupExtras(t)
	cases := repository.NewCaseRepository(f.scoped)
	scope := repository.SearchScope{Unrestricted: true}
	now := time.Now().UTC()

	// ctQueryID (an S4 Query) is the ticket being converted. A related ticket names it;
	// it names ctIncidentID (a different ticket) as its own related case.
	f.exec(t, `INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type)
		VALUES ($1, $2, $2, 'test', 'test', 'CT-TEST-0010', 'CT-TEST-0010', 'relates to the query', 'CASE')`, ctRelatedFromID, now)
	f.exec(t, `INSERT INTO "case" (id, severity, issue_type, state, related_case_id) VALUES ($1, 'S3', 'ERROR', 'OPEN', $2)`, ctRelatedFromID, ctQueryID)
	f.exec(t, `UPDATE "case" SET related_case_id = $1 WHERE id = $2`, ctIncidentID, ctQueryID)

	rowOf := func(id string) string {
		var v string
		if err := f.scoped.QueryRow(f.ctx, `SELECT row_to_json(w)::TEXT || (SELECT COALESCE(row_to_json(c)::TEXT, '') FROM "case" c WHERE c.id = w.id) FROM work_item w WHERE w.id = $1`, id).Scan(&v); err != nil {
			t.Fatalf("read %s: %v", id, err)
		}
		return v
	}
	relatedBefore, targetBefore := rowOf(ctRelatedFromID), rowOf(ctIncidentID)

	remoteCalled := false
	_, err := f.repo.TransferCaseType(f.ctx, ctEngagementPlan(ctQueryID), func(context.Context, repository.CaseTypeTransferBefore) (*repository.CaseTypeTransferRemoteResult, error) {
		remoteCalled = true
		return nil, nil
	})

	if f.relatedKeyTargetsCase(t) {
		var ce *apierror.ConflictError
		if !errors.As(err, &ce) || !strings.Contains(ce.Msg, "CT-TEST-0010") || !strings.Contains(ce.Msg, "0211") {
			t.Fatalf("without migration 0211 want a ConflictError naming the related ticket and the migration, got %v", err)
		}
		if remoteCalled {
			t.Errorf("ServiceNow was asked about a conversion that was going to be refused")
		}
		f.assertOnlyIn(t, ctQueryID, "case")
		if got := rowOf(ctRelatedFromID); got != relatedBefore {
			t.Errorf("the related ticket was touched by a refused conversion")
		}
		t.Log("database still has the old related-case key: conversion refused, nothing changed")
		return
	}

	if err != nil {
		t.Fatalf("converting the ticket: %v", err)
	}
	if !remoteCalled {
		t.Errorf("the remote step did not run")
	}
	f.assertOnlyIn(t, ctQueryID, "engagement")

	// The related ticket was not converted, and nothing about it changed.
	f.assertOnlyIn(t, ctRelatedFromID, "case")
	if got := f.workItemType(t, ctRelatedFromID); got != "CASE" {
		t.Errorf("the related ticket's type = %s, want it left as CASE", got)
	}
	if got := rowOf(ctRelatedFromID); got != relatedBefore {
		t.Errorf("the related ticket changed:\n before: %s\n after:  %s", relatedBefore, got)
	}
	// It is still related to the ticket that was converted, which now reads as an engagement.
	view, err := cases.GetCaseByID(f.ctx, ctRelatedFromID, scope)
	if err != nil {
		t.Fatalf("GetCaseByID of the related ticket: %v", err)
	}
	if view.RelatedCase == nil || view.RelatedCase.ID != ctQueryID || view.RelatedCase.Number != "CT-TEST-0002" ||
		view.RelatedCase.Type == nil || *view.RelatedCase.Type != "engagement" {
		t.Errorf("the related ticket's link = %+v, want the converted ticket (CT-TEST-0002) as an engagement", view.RelatedCase)
	}
	// The ticket the converted one used to relate to is untouched, and still a case.
	f.assertOnlyIn(t, ctIncidentID, "case")
	if got := rowOf(ctIncidentID); got != targetBefore {
		t.Errorf("the ticket the converted one related to changed")
	}
}

// A case is related only to another case-like ticket. The key now points at
// work_item, which holds every kind of ticket, so the write checks the type.
func TestCaseTypeTransferIntegration_RelatedCaseMustBeACaseLikeTicket(t *testing.T) {
	f := newCTFixture(t)
	f.cleanupExtras(t)
	cases := repository.NewCaseRepository(f.scoped)
	now := time.Now().UTC()
	f.exec(t, `INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, subject, type)
		VALUES ($1, $2, $2, 'test', 'test', 'CT-TEST-0007', 'a change request', 'CHANGE_REQUEST')`, ctChangeReqID, now)
	f.exec(t, `INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, subject, type)
		VALUES ($1, $2, $2, 'test', 'test', 'CT-TEST-0008', 'an incident', 'INCIDENT')`, ctIncidentWIID, now)

	relate := func(target string) error {
		_, err := cases.UpdateCaseFields(f.ctx, domain.UpdateCaseRequest{ID: ctIncidentID, RelatedCaseID: &target}, "", ctActor)
		return err
	}
	for name, id := range map[string]string{"an announcement": ctAnnouncementID, "a change request": ctChangeReqID, "an incident": ctIncidentWIID, "a missing id": ctMissingID} {
		var ve *apierror.ValidationError
		if err := relate(id); !errors.As(err, &ve) {
			t.Errorf("relating a case to %s: want a ValidationError, got %v", name, err)
		}
	}
	if err := relate(ctQueryID); err != nil { // another case
		t.Errorf("relating a case to another case: %v", err)
	}
	if !f.relatedKeyTargetsCase(t) {
		// An engagement (a converted ticket) can be related to once the key points at work_item.
		if err := relate(ctEngagementID); err != nil {
			t.Errorf("relating a case to an engagement: %v", err)
		}
	}
}

// The customer's attachments are on the ticket, not on its type: after the
// conversion the engagement lists them (the ones uploaded through the portal and
// the ones that came from ServiceNow alike) and each can be opened by id. A
// still-uploading one stays out of the list, as it does for any ticket.
func TestCaseTypeTransferIntegration_AttachmentsAreAvailableOnTheConvertedTicket(t *testing.T) {
	f := newCTFixture(t)
	f.cleanupExtras(t)
	cases := repository.NewCaseRepository(f.scoped)

	portalKey := ctBulkMarker
	fromPortal, err := cases.CreateCaseAttachment(f.ctx, domain.CreateAttachmentRequest{
		ReferenceID: ctQueryID, ReferenceType: domain.ReferenceTypeCase, Name: "customer-logs.zip", Type: "application/zip",
		StorageKey: &portalKey, SizeBytes: 2048, CreatedBy: f.userID, Status: domain.AttachmentStatusComplete,
	})
	if err != nil {
		t.Fatalf("seed a portal attachment: %v", err)
	}
	fromSN, err := cases.CreateCaseAttachmentFromServiceNow(f.ctx, domain.CreateAttachmentRequest{
		ReferenceID: ctQueryID, ReferenceType: domain.ReferenceTypeCase, Name: "config.xml", Type: "text/xml",
	}, "92000000-0000-0000-0000-0000000000b1", 512, f.userID)
	if err != nil {
		t.Fatalf("seed a ServiceNow attachment: %v", err)
	}
	f.exec(t, `INSERT INTO case_attachment (id, case_id, storage_key, filename, mime_type, size_bytes, uploaded_by, status)
		VALUES (gen_random_uuid(), $1, $2, 'still-uploading.bin', 'application/octet-stream', 1, $3, 'pending')`, ctQueryID, ctBulkMarker, f.userID)

	if _, err := f.repo.TransferCaseType(f.ctx, ctEngagementPlan(ctQueryID), nil); err != nil {
		t.Fatalf("TransferCaseType: %v", err)
	}

	listed, total, err := cases.SearchCaseAttachments(f.ctx, ctQueryID, domain.Pagination{Limit: 50, Offset: 0})
	if err != nil {
		t.Fatalf("SearchCaseAttachments on the converted ticket: %v", err)
	}
	names := map[string]bool{}
	for _, a := range listed {
		names[a.Name] = true
		if a.ReferenceID != ctQueryID {
			t.Errorf("attachment %s belongs to %s, want the converted ticket", a.Name, a.ReferenceID)
		}
	}
	// the fixture's own attachment (a.txt), the portal one and the ServiceNow one; not the pending one
	for _, want := range []string{"a.txt", "customer-logs.zip", "config.xml"} {
		if !names[want] {
			t.Errorf("%s is not listed on the converted ticket (listed: %v)", want, names)
		}
	}
	if names["still-uploading.bin"] || total != 3 {
		t.Errorf("a pending upload must stay out of the list: total=%d names=%v", total, names)
	}

	for _, id := range []string{fromPortal.ID, fromSN.ID} {
		got, err := cases.GetCaseAttachmentByID(f.ctx, id)
		if err != nil || got.ReferenceID != ctQueryID {
			t.Errorf("attachment %s cannot be opened on the converted ticket: %+v, %v", id, got, err)
		}
	}
	got, _ := cases.GetCaseAttachmentByID(f.ctx, fromSN.ID)
	if got.StorageKey != nil {
		t.Errorf("a ServiceNow attachment gained a storage key by being moved: %v", *got.StorageKey)
	}
}

// A change request, a child case, a watcher, a tag and comments hang off a case
// by its work item. None of them is read or written by a transfer.
func TestCaseTypeTransferIntegration_LinkedItemsAreNotTouched(t *testing.T) {
	f := newCTFixture(t)
	f.cleanupExtras(t)
	now := time.Now().UTC()

	// A change request linked to the incident, as 1 in 13 tickets have on staging.
	f.exec(t, `INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, subject, type, parent_id)
		VALUES ('92000000-0000-0000-0000-000000000020', $1, $1, $2, 'test', 'CT-CHG-0001', 'a change', 'CHANGE_REQUEST', $3)`, now, ctBulkMarker, ctIncidentID)
	f.exec(t, `INSERT INTO change_request (id, state) VALUES ('92000000-0000-0000-0000-000000000020', 'ASSESS')`)
	// A child case.
	f.exec(t, `INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type, parent_id)
		VALUES ($1, $2, $2, 'test', 'test', 'CT-TEST-0011', 'CT-TEST-0011', 'child', 'SERVICE_REQUEST', $3)`, ctChildID, now, ctIncidentID)
	f.exec(t, `INSERT INTO service_request (id, state) VALUES ($1, 'OPEN')`, ctChildID)
	// A watcher, a tag, comments.
	f.exec(t, `INSERT INTO work_item_watcher (id, work_item_id, user_id) VALUES (gen_random_uuid(), $1, $2)`, ctIncidentID, f.userID)
	f.exec(t, `INSERT INTO tag (id, created_on, updated_on, name) VALUES ($1, $2, $2, 'ct-tag')`, ctTagID, now)
	f.exec(t, `INSERT INTO work_item_tag (id, created_on, updated_on, work_item_id, tag_id) VALUES (gen_random_uuid(), $1, $1, $2, $3)`, now, ctIncidentID, ctTagID)
	f.exec(t, `INSERT INTO comment (id, created_on, created_by, type, work_item_id, content)
		SELECT gen_random_uuid(), $1, $2, 'COMMENT', $3, 'comment ' || g FROM generate_series(1, 12) g`, now, ctBulkMarker, ctIncidentID)

	snapshot := func() map[string]any {
		out := map[string]any{}
		scan := func(key, sql string) {
			var v any
			if err := f.scoped.QueryRow(f.ctx, sql).Scan(&v); err != nil {
				t.Fatalf("snapshot %s: %v", key, err)
			}
			out[key] = v
		}
		scan("change request row", `SELECT (SELECT row_to_json(w)::TEXT FROM work_item w WHERE id = '92000000-0000-0000-0000-000000000020') || (SELECT row_to_json(c)::TEXT FROM change_request c WHERE id = '92000000-0000-0000-0000-000000000020')`)
		scan("child row", `SELECT row_to_json(w)::TEXT || (SELECT row_to_json(s)::TEXT FROM service_request s WHERE id = '`+ctChildID+`') FROM work_item w WHERE id = '`+ctChildID+`'`)
		scan("watchers", `SELECT COUNT(*) FROM work_item_watcher WHERE work_item_id = '`+ctIncidentID+`'`)
		scan("tags", `SELECT COUNT(*) FROM work_item_tag WHERE work_item_id = '`+ctIncidentID+`'`)
		scan("comments", `SELECT COUNT(*) FROM comment WHERE work_item_id = '`+ctIncidentID+`'`)
		scan("attachments", `SELECT COUNT(*) FROM case_attachment WHERE case_id = '`+ctIncidentID+`'`)
		scan("time cards", `SELECT COUNT(*) FROM time_card WHERE case_id = '`+ctIncidentID+`'`)
		scan("the incident's own project, deployment, account", `SELECT concat_ws('|', project_id, deployment_id, deployed_product_id, account_id, number, wso2_id, subject, created_by, created_on) FROM work_item WHERE id = '`+ctIncidentID+`'`)
		return out
	}
	before := snapshot()

	if _, err := f.repo.TransferCaseType(f.ctx, ctEngagementPlan(ctIncidentID), nil); err != nil {
		t.Fatalf("TransferCaseType: %v", err)
	}
	after := snapshot()
	for key, want := range before {
		if got := after[key]; got != want {
			t.Errorf("%s changed by the transfer:\n before: %v\n after:  %v", key, want, got)
		}
	}

	// The change request still hangs off the same work item, which is now an engagement.
	var parentType string
	if err := f.scoped.QueryRow(f.ctx, `SELECT p.type::TEXT FROM work_item cr JOIN work_item p ON p.id = cr.parent_id WHERE cr.id = '92000000-0000-0000-0000-000000000020'`).Scan(&parentType); err != nil || parentType != "ENGAGEMENT" {
		t.Errorf("the change request's parent = %q (%v), want the same ticket, now an ENGAGEMENT", parentType, err)
	}
}

// A very large ticket moves as cheaply as a small one: the transfer touches the
// ticket's own two rows (and its time cards, when it crosses the S4 line), never
// its history. Everything the history is made of is still there afterwards, and
// the S4 line re-marks every time card in one statement.
func TestCaseTypeTransferIntegration_ALargeTicketMovesWholeAndQuickly(t *testing.T) {
	const (
		comments    = 5000
		attachments = 400
		timeCards   = 1500
		children    = 150
	)
	f := newCTFixture(t)
	f.cleanupExtras(t)
	now := time.Now().UTC()

	f.exec(t, `INSERT INTO comment (id, created_on, created_by, type, work_item_id, content)
		SELECT gen_random_uuid(), $1, $2, 'COMMENT', $3, repeat('x', 400) FROM generate_series(1, $4) g`, now, ctBulkMarker, ctQueryID, comments)
	f.exec(t, `INSERT INTO case_attachment (id, case_id, storage_key, filename, mime_type, size_bytes, uploaded_by, status)
		SELECT gen_random_uuid(), $1, $2, 'f' || g, 'text/plain', 1, $3, 'complete' FROM generate_series(1, $4) g`, ctQueryID, ctBulkMarker, f.userID, attachments)
	f.exec(t, `INSERT INTO time_card (id, created_on, updated_on, created_by, updated_by, case_id, user_id, work_date, is_billable, state)
		SELECT gen_random_uuid(), $1, $1, $2, $2, $3, $4, CURRENT_DATE - g, TRUE, 'SUBMITTED' FROM generate_series(1, $5) g`, now, ctBulkMarker, ctQueryID, f.userID, timeCards)
	f.exec(t, `INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type, parent_id)
		SELECT gen_random_uuid(), $1, $1, $2, $2, 'CT-BULK-' || g, 'CT-BULK-' || g, 'child ' || g, 'CASE', $3 FROM generate_series(1, $4) g`, now, ctBulkMarker, ctQueryID, children)

	started := time.Now()
	if _, err := f.repo.TransferCaseType(f.ctx, ctEngagementPlan(ctQueryID), nil); err != nil {
		t.Fatalf("TransferCaseType: %v", err)
	}
	elapsed := time.Since(started)
	t.Logf("transferred a Query with %d comments, %d attachments, %d time cards and %d child cases in %s", comments, attachments, timeCards, children, elapsed)
	if elapsed > 10*time.Second {
		t.Errorf("the transfer took %s", elapsed)
	}

	f.assertOnlyIn(t, ctQueryID, "engagement")
	if got := f.count(t, `SELECT COUNT(*) FROM comment WHERE work_item_id = $1`, ctQueryID); got != comments {
		t.Errorf("comments = %d, want %d", got, comments)
	}
	if got := f.count(t, `SELECT COUNT(*) FROM case_attachment WHERE case_id = $1`, ctQueryID); got != attachments+1 {
		t.Errorf("attachments = %d, want %d (the fixture's own plus %d)", got, attachments+1, attachments)
	}
	if got := f.count(t, `SELECT COUNT(*) FROM work_item WHERE parent_id = $1`, ctQueryID); got != children {
		t.Errorf("child cases = %d, want %d", got, children)
	}
	if got := f.count(t, `SELECT COUNT(*) FROM time_card WHERE case_id = $1`, ctQueryID); got != timeCards+1 {
		t.Errorf("time cards = %d, want %d", got, timeCards+1)
	}
	// The Query left the S4 line, so every one of its time cards was re-marked.
	if got := f.count(t, `SELECT COUNT(*) FROM time_card WHERE case_id = $1 AND is_billable`, ctQueryID); got != 0 {
		t.Errorf("%d time card(s) still billable after the Query left S4", got)
	}
}

// A Query or an Incident moved to Engagement (Migration) is, from then on, a
// Migration ticket like any other: it reads back as one, moves through its states,
// takes the fields an engagement takes, and closes.
func TestCaseTypeTransferIntegration_AMigrationTicketWorksAsOneAfterwards(t *testing.T) {
	for _, tc := range []struct {
		name string
		id   string
	}{{"Incident (S2)", ctIncidentID}, {"Query (S4)", ctQueryID}} {
		t.Run(tc.name, func(t *testing.T) {
			f := newCTFixture(t)
			cases := repository.NewCaseRepository(f.scoped)
			scope := repository.SearchScope{Unrestricted: true}

			before, err := cases.GetCaseByID(f.ctx, tc.id, scope)
			if err != nil {
				t.Fatalf("GetCaseByID before: %v", err)
			}
			if _, err := f.repo.TransferCaseType(f.ctx, ctEngagementPlan(tc.id), nil); err != nil {
				t.Fatalf("TransferCaseType: %v", err)
			}

			cv, err := cases.GetCaseByID(f.ctx, tc.id, scope)
			if err != nil {
				t.Fatalf("GetCaseByID after: %v", err)
			}
			if cv.Type == nil || *cv.Type != "engagement" {
				t.Fatalf("reads back as type=%v, want engagement", cv.Type)
			}
			if cv.Severity != nil || cv.IssueType != nil {
				t.Errorf("an engagement has no severity or issue type, got %v / %v", cv.Severity, cv.IssueType)
			}
			// The measure of "works as a Migration ticket" is a native engagement: the
			// transferred one must read the same way. (GetCaseByID does not return
			// engagementType for ANY engagement on this data source -- only the list
			// search selects it -- so that is not something a transfer can or should
			// change; the stored type and payment type are asserted in the other tests.)
			native, err := cases.GetCaseByID(f.ctx, ctEngagementID, scope)
			if err != nil {
				t.Fatalf("GetCaseByID of the native engagement: %v", err)
			}
			if (cv.EngagementType == nil) != (native.EngagementType == nil) || (cv.Type == nil) != (native.Type == nil) || *cv.Type != *native.Type {
				t.Errorf("reads differently from a native engagement: engagementType %v vs %v, type %v vs %v", cv.EngagementType, native.EngagementType, cv.Type, native.Type)
			}
			var storedType, storedPayment string
			if err := f.scoped.QueryRow(f.ctx, `SELECT type::TEXT, payment_type::TEXT FROM engagement WHERE id = $1`, tc.id).Scan(&storedType, &storedPayment); err != nil || storedType != "MIGRATION" || storedPayment != "FOC" {
				t.Errorf("stored engagement type/payment type = %s/%s (%v), want MIGRATION/FOC", storedType, storedPayment, err)
			}
			if cv.Number != before.Number || cv.ID != before.ID || cv.State == nil || before.State == nil || *cv.State != *before.State {
				t.Errorf("number/id/state changed: %s %s %v -> %s %s %v", before.Number, before.ID, before.State, cv.Number, cv.ID, cv.State)
			}

			// It moves through its states, as an engagement does.
			for _, next := range []domain.CaseState{domain.CaseStateWorkInProgress, domain.CaseStateAwaitingInfo, domain.CaseStateWorkInProgress} {
				next := next
				if _, _, err := cases.UpdateCase(f.ctx, domain.UpdateCaseRequest{ID: tc.id, State: &next}, nil); err != nil {
					t.Fatalf("move to %s: %v", next, err)
				}
				got, err := cases.GetCaseByID(f.ctx, tc.id, scope)
				if err != nil || got.State == nil || *got.State != next {
					t.Fatalf("after moving to %s the ticket reads %v (%v)", next, got.State, err)
				}
			}
			// It takes the plain fields an engagement takes.
			subject := "Migration of the customer's deployment"
			if _, err := cases.UpdateCaseFields(f.ctx, domain.UpdateCaseRequest{ID: tc.id, Subject: &subject}, "", ctActor); err != nil {
				t.Fatalf("update subject: %v", err)
			}
			// And it closes, which stamps when.
			closed := domain.CaseStateClosed
			if _, _, err := cases.UpdateCase(f.ctx, domain.UpdateCaseRequest{ID: tc.id, State: &closed}, nil); err != nil {
				t.Fatalf("close: %v", err)
			}
			final, err := cases.GetCaseByID(f.ctx, tc.id, scope)
			if err != nil {
				t.Fatalf("GetCaseByID after closing: %v", err)
			}
			if final.State == nil || *final.State != domain.CaseStateClosed || final.Subject != subject {
				t.Errorf("after closing: state=%v subject=%q", final.State, final.Subject)
			}
			if got := f.count(t, `SELECT COUNT(*) FROM engagement WHERE id = $1 AND closed_on IS NOT NULL`, tc.id); got != 1 {
				t.Errorf("the engagement has no closed_on after closing")
			}
		})
	}
}
