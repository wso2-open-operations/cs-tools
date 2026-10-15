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

package repository_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// These run against a real Postgres, as a non-superuser (so row level security
// is in force), reusing the announcement visibility fixtures and skipped
// without ANNOUNCEMENT_VISIBILITY_TEST_DSN:
//
//	ANNOUNCEMENT_VISIBILITY_TEST_DSN=postgres://... go test ./internal/repository/ -run AnnouncementAttachment

const (
	aaUploaderID = "d0000000-0000-0000-0000-0000000000b1"
	aaPlainCase  = "34444444-4444-4444-4444-444444444444"
)

func aaStrPtr(s string) *string { return &s }

// seedAnnouncementAttachmentUploader creates the "user" row case_attachment's
// uploaded_by foreign key needs, and a plain CASE work item in the same
// project (to prove the rule change leaves every other type as it was).
func seedAnnouncementAttachmentUploader(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	sys := repository.WithSystemIdentity(ctx)
	scoped := repository.NewScoped(pool)
	if _, err := pool.Exec(ctx, `INSERT INTO "user" (id, created_on, updated_on, user_name, email, first_name, last_name)
		VALUES ($1, NOW(), NOW(), 'aa-uploader', 'aa-uploader@test.local', 'AA', 'Uploader') ON CONFLICT (id) DO NOTHING`, aaUploaderID); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if _, err := scoped.Exec(sys, `INSERT INTO work_item (id, created_on, updated_on, created_by, updated_by, number, wso2_id, subject, type, project_id)
		VALUES ($1, NOW(), NOW(), 'test', 'test', 'AA-TEST-CASE-1', 'AA-WSO2-CASE-1', 'AA plain case', 'CASE', $2)
		ON CONFLICT (id) DO NOTHING`, aaPlainCase, avProjectID); err != nil {
		t.Fatalf("seed plain case: %v", err)
	}
	t.Cleanup(func() {
		_, _ = scoped.Exec(sys, `DELETE FROM case_attachment WHERE case_id IN ($1, $2, $3)`, avGeneralID, avSecurityID, aaPlainCase)
		_, _ = scoped.Exec(sys, `DELETE FROM work_item WHERE id = $1`, aaPlainCase)
		_, _ = pool.Exec(ctx, `DELETE FROM "user" WHERE id = $1`, aaUploaderID)
	})
}

func attachmentReq(caseID, name string) domain.CreateAttachmentRequest {
	return domain.CreateAttachmentRequest{
		ReferenceID:   caseID,
		ReferenceType: domain.ReferenceTypeCase,
		Name:          name,
		Type:          "application/pdf",
		StorageKey:    aaStrPtr("announcement/" + name),
		SizeBytes:     100,
		CreatedBy:     aaUploaderID,
		Status:        domain.AttachmentStatusComplete,
	}
}

// TestAnnouncementAttachmentVisibilityIntegration is the end-to-end proof of
// the two halves of announcement attachments: WHO may put one on an
// announcement (the insert's own condition) and WHO may read one (migration
// 100025's policies on case_attachment).
func TestAnnouncementAttachmentVisibilityIntegration(t *testing.T) {
	pool := announcementVisibilityPool(t)
	seedAnnouncementVisibilityFixtures(t, pool)
	seedAnnouncementAttachmentUploader(t, pool)
	repo := repository.NewCaseRepository(repository.NewScoped(pool))

	member := func(email string) repository.SearchScope {
		return repository.SearchScope{ProjectIDs: []string{avProjectID}, ViewerEmail: email}
	}
	internal := repository.WithCallerIdentity(context.Background(), repository.SearchScope{Unrestricted: true})

	t.Run("an internal caller can attach to a general and a security announcement", func(t *testing.T) {
		for _, id := range []string{avGeneralID, avSecurityID} {
			a, err := repo.CreateCaseAttachment(internal, attachmentReq(id, "advisory-"+id[:2]+".pdf"))
			if err != nil {
				t.Fatalf("CreateCaseAttachment(%s): %v", id, err)
			}
			if a.ReferenceID != id || a.Status != domain.AttachmentStatusComplete {
				t.Fatalf("unexpected attachment: %+v", a)
			}
		}
	})

	t.Run("the ServiceNow-sourced insert (dual-write) accepts an announcement too", func(t *testing.T) {
		a, err := repo.CreateCaseAttachmentFromServiceNow(internal, attachmentReq(avGeneralID, "from-sn.pdf"),
			"e1111111-1111-4111-8111-111111111111", 321, aaUploaderID)
		if err != nil {
			t.Fatalf("CreateCaseAttachmentFromServiceNow: %v", err)
		}
		if a.SizeBytes != 321 {
			t.Fatalf("unexpected attachment: %+v", a)
		}
	})

	t.Run("a customer cannot attach to an announcement, even one they can see", func(t *testing.T) {
		ctx := repository.WithCallerIdentity(context.Background(), member("av-full-access@test.local"))
		_, err := repo.CreateCaseAttachment(ctx, attachmentReq(avGeneralID, "customer.pdf"))
		var validation *apierror.ValidationError
		if !errors.As(err, &validation) {
			t.Fatalf("expected the same 'referenced IDs do not exist' refusal a non-owning work item gets, got %v", err)
		}
		_, err = repo.CreateCaseAttachmentFromServiceNow(ctx, attachmentReq(avGeneralID, "customer-sn.pdf"),
			"e2222222-2222-4222-8222-222222222222", 10, aaUploaderID)
		if !errors.As(err, &validation) {
			t.Fatalf("expected a refusal on the ServiceNow-sourced insert too, got %v", err)
		}
	})

	t.Run("a request with no resolved identity cannot attach to an announcement (fails closed)", func(t *testing.T) {
		_, err := repo.CreateCaseAttachment(context.Background(), attachmentReq(avGeneralID, "anonymous.pdf"))
		if err == nil {
			t.Fatal("expected an error with no caller identity")
		}
	})

	t.Run("a customer can still attach to an ordinary case of their project", func(t *testing.T) {
		ctx := repository.WithCallerIdentity(context.Background(), member("av-full-access@test.local"))
		if _, err := repo.CreateCaseAttachment(ctx, attachmentReq(aaPlainCase, "plain.pdf")); err != nil {
			t.Fatalf("the rule change must leave other work item types alone: %v", err)
		}
	})

	count := func(scope repository.SearchScope, id string) int {
		t.Helper()
		ctx := repository.WithCallerIdentity(context.Background(), scope)
		_, total, err := repo.SearchCaseAttachments(ctx, id, domain.Pagination{Limit: 50})
		if err != nil {
			t.Fatalf("SearchCaseAttachments(%s): %v", id, err)
		}
		return total
	}

	// The general announcement holds two files (one plain, one from
	// ServiceNow), the security announcement one.
	for _, tc := range []struct {
		name         string
		scope        repository.SearchScope
		wantGeneral  int
		wantSecurity int
	}{
		{"General Access sees the general announcement's files only", member("av-general@test.local"), 2, 0},
		{"Security Only sees both", member("av-secure-only@test.local"), 2, 1},
		{"Full Access sees both", member("av-full-access@test.local"), 2, 1},
		{"Lead User Group sees the general announcement's files only", member("av-lead@test.local"), 2, 0},
		{"Business Contact alone sees nothing", member("av-biz-contact-alone@test.local"), 0, 0},
		{"an unknown email sees nothing (fails closed)", member("nobody@nowhere.local"), 0, 0},
		{"an internal caller sees both", repository.SearchScope{Unrestricted: true}, 2, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := count(tc.scope, avGeneralID); got != tc.wantGeneral {
				t.Errorf("general announcement files = %d, want %d", got, tc.wantGeneral)
			}
			if got := count(tc.scope, avSecurityID); got != tc.wantSecurity {
				t.Errorf("security announcement files = %d, want %d", got, tc.wantSecurity)
			}
		})
	}

	t.Run("a file of a security announcement cannot be fetched by id by someone who cannot see the announcement", func(t *testing.T) {
		// internal: find the security announcement's file id
		ctx := internal
		list, _, err := repo.SearchCaseAttachments(ctx, avSecurityID, domain.Pagination{Limit: 10})
		if err != nil || len(list) != 1 {
			t.Fatalf("expected the one security file, got %v err=%v", list, err)
		}
		general := repository.WithCallerIdentity(context.Background(), member("av-general@test.local"))
		if _, err := repo.GetCaseAttachmentByID(general, list[0].ID); err == nil {
			t.Fatal("a General Access contact read a security announcement's file by id")
		}
		secure := repository.WithCallerIdentity(context.Background(), member("av-secure-only@test.local"))
		if _, err := repo.GetCaseAttachmentByID(secure, list[0].ID); err != nil {
			t.Fatalf("a security contact must read it: %v", err)
		}
	})
}

// --- the request's own files --------------------------------------------

func newRequest(t *testing.T, repo repository.AnnouncementRequestRepository) domain.AnnouncementRequest {
	t.Helper()
	ar, err := repo.Create(context.Background(), domain.CreateAnnouncementRequestRequest{
		Kind: domain.AnnouncementRequestKindCustomer, Subject: "s", Description: "d", CreatedBy: "u1",
	})
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	return ar
}

func file(name string, size int) domain.AnnouncementRequestAttachmentContent {
	content := make([]byte, size)
	for i := range content {
		content[i] = byte(i % 251)
	}
	return domain.AnnouncementRequestAttachmentContent{
		AnnouncementRequestAttachment: domain.AnnouncementRequestAttachment{
			Name: name, Type: "application/pdf", SizeBytes: size, CreatedBy: "u1", CreatedByEmail: aaStrPtr("u1@example.com"),
		},
		Content: content,
	}
}

func TestAnnouncementRequestAttachmentRepoIntegration(t *testing.T) {
	pool := announcementVisibilityPool(t)
	repo := repository.NewAnnouncementRequestRepository(pool)
	ctx := context.Background()

	t.Run("stores, lists in order and returns the exact bytes", func(t *testing.T) {
		ar := newRequest(t, repo)
		t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM announcement_requests WHERE id = $1`, ar.ID) })

		a, err := repo.AddAttachment(ctx, ar.ID, domain.AnnouncementRequestStateDraft, false, file("one.pdf", 70_000))
		if err != nil {
			t.Fatalf("AddAttachment: %v", err)
		}
		if a.ID == "" || a.AnnouncementRequestID != ar.ID || a.Name != "one.pdf" || a.SizeBytes != 70_000 || a.CreatedByEmail == nil {
			t.Fatalf("unexpected metadata: %+v", a)
		}
		if _, err := repo.AddAttachment(ctx, ar.ID, domain.AnnouncementRequestStateDraft, false, file("two.pdf", 10)); err != nil {
			t.Fatalf("AddAttachment: %v", err)
		}

		list, err := repo.ListAttachments(ctx, ar.ID)
		if err != nil || len(list) != 2 || list[0].Name != "one.pdf" || list[1].Name != "two.pdf" {
			t.Fatalf("expected the two files oldest first, got %+v err=%v", list, err)
		}
		got, err := repo.GetAttachmentContent(ctx, ar.ID, a.ID)
		if err != nil {
			t.Fatalf("GetAttachmentContent: %v", err)
		}
		want := file("one.pdf", 70_000)
		if string(got.Content) != string(want.Content) || got.Name != "one.pdf" || got.Type != "application/pdf" {
			t.Fatal("the bytes read back are not the bytes stored")
		}

		// A file is only reachable through its own request.
		other := newRequest(t, repo)
		t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM announcement_requests WHERE id = $1`, other.ID) })
		var notFound *apierror.NotFoundError
		if _, err := repo.GetAttachmentContent(ctx, other.ID, a.ID); !errors.As(err, &notFound) {
			t.Fatalf("expected NotFoundError reading a file through another request, got %v", err)
		}
	})

	t.Run("at most 5 files, and no two of one name", func(t *testing.T) {
		ar := newRequest(t, repo)
		t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM announcement_requests WHERE id = $1`, ar.ID) })
		for i := 1; i <= domain.MaxAnnouncementRequestAttachments; i++ {
			if _, err := repo.AddAttachment(ctx, ar.ID, domain.AnnouncementRequestStateDraft, false, file(fmt.Sprintf("f%d.pdf", i), 5)); err != nil {
				t.Fatalf("file %d: %v", i, err)
			}
		}
		var validation *apierror.ValidationError
		if _, err := repo.AddAttachment(ctx, ar.ID, domain.AnnouncementRequestStateDraft, false, file("sixth.pdf", 5)); !errors.As(err, &validation) {
			t.Fatalf("expected a validation error for a sixth file, got %v", err)
		}

		dup := newRequest(t, repo)
		t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM announcement_requests WHERE id = $1`, dup.ID) })
		if _, err := repo.AddAttachment(ctx, dup.ID, domain.AnnouncementRequestStateDraft, false, file("Advisory.PDF", 5)); err != nil {
			t.Fatalf("first: %v", err)
		}
		if _, err := repo.AddAttachment(ctx, dup.ID, domain.AnnouncementRequestStateDraft, false, file("advisory.pdf", 6)); !errors.As(err, &validation) {
			t.Fatalf("expected a validation error for a name that differs only by case, got %v", err)
		}
	})

	t.Run("concurrent adds cannot get past the limit", func(t *testing.T) {
		ar := newRequest(t, repo)
		t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM announcement_requests WHERE id = $1`, ar.ID) })
		var wg sync.WaitGroup
		var mu sync.Mutex
		ok := 0
		for i := 0; i < 12; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				if _, err := repo.AddAttachment(ctx, ar.ID, domain.AnnouncementRequestStateDraft, false, file(fmt.Sprintf("c%d.pdf", i), 5)); err == nil {
					mu.Lock()
					ok++
					mu.Unlock()
				}
			}(i)
		}
		wg.Wait()
		list, _ := repo.ListAttachments(ctx, ar.ID)
		if ok != domain.MaxAnnouncementRequestAttachments || len(list) != domain.MaxAnnouncementRequestAttachments {
			t.Fatalf("expected exactly %d files to win, got %d succeeded and %d stored", domain.MaxAnnouncementRequestAttachments, ok, len(list))
		}
	})

	t.Run("a change is conditioned on the request's state", func(t *testing.T) {
		ar := newRequest(t, repo)
		t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM announcement_requests WHERE id = $1`, ar.ID) })
		// The service saw a draft, but the request has moved on.
		if _, err := pool.Exec(ctx, `UPDATE announcement_requests SET state = 'approved' WHERE id = $1`, ar.ID); err != nil {
			t.Fatal(err)
		}
		var conflict *apierror.ConflictError
		if _, err := repo.AddAttachment(ctx, ar.ID, domain.AnnouncementRequestStateDraft, false, file("late.pdf", 5)); !errors.As(err, &conflict) {
			t.Fatalf("expected a ConflictError, got %v", err)
		}
		if list, _ := repo.ListAttachments(ctx, ar.ID); len(list) != 0 {
			t.Fatalf("a refused change must store nothing, got %+v", list)
		}
		var notFound *apierror.NotFoundError
		if _, err := repo.AddAttachment(ctx, "99999999-9999-4999-8999-999999999999", domain.AnnouncementRequestStateDraft, false, file("x.pdf", 5)); !errors.As(err, &notFound) {
			t.Fatalf("expected a NotFoundError for a missing request, got %v", err)
		}
	})

	t.Run("files are frozen once a case exists for any project; a failed delivery freezes nothing", func(t *testing.T) {
		const (
			projectFailed  = "aaaaaaaa-0000-4000-8000-000000000001"
			projectCreated = "aaaaaaaa-0000-4000-8000-000000000002"
			projectTagging = "aaaaaaaa-0000-4000-8000-000000000003"
		)
		approved := domain.AnnouncementRequestStateApproved
		ar := newRequest(t, repo)
		t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM announcement_requests WHERE id = $1`, ar.ID) })
		if _, err := pool.Exec(ctx, `UPDATE announcement_requests SET state = 'approved' WHERE id = $1`, ar.ID); err != nil {
			t.Fatal(err)
		}
		first, err := repo.AddAttachment(ctx, ar.ID, approved, false, file("one.pdf", 5))
		if err != nil {
			t.Fatalf("AddAttachment: %v", err)
		}

		// A project whose case could not be created leaves nothing to diverge from.
		failure := "downstream refused"
		if _, err := repo.UpsertDeliveries(ctx, ar.ID, []domain.RecordAnnouncementRequestDeliveryInput{
			{ProjectID: projectFailed, Status: domain.AnnouncementRequestDeliveryStatusFailed, ErrorMessage: &failure},
		}); err != nil {
			t.Fatalf("UpsertDeliveries: %v", err)
		}
		if _, err := repo.AddAttachment(ctx, ar.ID, approved, false, file("two.pdf", 5)); err != nil {
			t.Fatalf("a failed delivery must not freeze the files: %v", err)
		}

		// A case that exists carries the files as they are now.
		caseID := "case-1"
		if _, err := repo.UpsertDeliveries(ctx, ar.ID, []domain.RecordAnnouncementRequestDeliveryInput{
			{ProjectID: projectCreated, CaseID: &caseID, Status: domain.AnnouncementRequestDeliveryStatusSucceeded},
		}); err != nil {
			t.Fatalf("UpsertDeliveries: %v", err)
		}
		var conflict *apierror.ConflictError
		if _, err := repo.AddAttachment(ctx, ar.ID, approved, false, file("three.pdf", 5)); !errors.As(err, &conflict) {
			t.Fatalf("expected a ConflictError adding a file after a case exists, got %v", err)
		}
		if err := repo.DeleteAttachment(ctx, ar.ID, first.ID, approved, false); !errors.As(err, &conflict) {
			t.Fatalf("expected a ConflictError removing a file after a case exists, got %v", err)
		}
		list, err := repo.ListAttachments(ctx, ar.ID)
		if err != nil || len(list) != 2 {
			t.Fatalf("a refused change must leave the files as they were, got %+v err=%v", list, err)
		}

		// A case that exists but is short of its follow-up (the files and security
		// label are redone on retry) is just as frozen: the retry copies the files.
		tagging := newRequest(t, repo)
		t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM announcement_requests WHERE id = $1`, tagging.ID) })
		if _, err := pool.Exec(ctx, `UPDATE announcement_requests SET state = 'approved' WHERE id = $1`, tagging.ID); err != nil {
			t.Fatal(err)
		}
		retryCase := "case-2"
		if _, err := repo.UpsertDeliveries(ctx, tagging.ID, []domain.RecordAnnouncementRequestDeliveryInput{
			{ProjectID: projectTagging, CaseID: &retryCase, Status: domain.AnnouncementRequestDeliveryStatusTagFailed},
		}); err != nil {
			t.Fatalf("UpsertDeliveries: %v", err)
		}
		if _, err := repo.AddAttachment(ctx, tagging.ID, approved, false, file("late.pdf", 5)); !errors.As(err, &conflict) {
			t.Fatalf("expected a ConflictError while a follow-up is outstanding, got %v", err)
		}
	})

	pending := func(t *testing.T) domain.AnnouncementRequest {
		t.Helper()
		ar := newRequest(t, repo)
		t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM announcement_requests WHERE id = $1`, ar.ID) })
		if _, err := pool.Exec(ctx, `UPDATE announcement_requests SET
				state = 'pending_approval', resolved_project_ids = '["p1","p2"]'::jsonb, resolved_project_count = 2,
				dry_run_case_id = 'case-1', dry_run_on = NOW(), dry_run_by = 'u1',
				submitted_by = 'u1', submitted_on = NOW()
			WHERE id = $1`, ar.ID); err != nil {
			t.Fatal(err)
		}
		return ar
	}

	t.Run("a file added to a request in review sends it back to draft and clears the snapshot and dry run", func(t *testing.T) {
		ar := pending(t)
		if _, err := repo.AddAttachment(ctx, ar.ID, domain.AnnouncementRequestStatePendingApproval, true, file("late.pdf", 5)); err != nil {
			t.Fatalf("AddAttachment: %v", err)
		}
		got, err := repo.Get(ctx, ar.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.State != domain.AnnouncementRequestStateDraft || len(got.ResolvedProjectIDs) != 0 || got.DryRunCaseID != nil || got.SubmittedBy != nil {
			t.Fatalf("expected a clean draft, got state=%s resolved=%v dryRun=%v submittedBy=%v", got.State, got.ResolvedProjectIDs, got.DryRunCaseID, got.SubmittedBy)
		}
		if list, _ := repo.ListAttachments(ctx, ar.ID); len(list) != 1 {
			t.Fatalf("expected the file stored, got %+v", list)
		}
	})

	t.Run("removing a file the request never had changes nothing, a request in review stays in review", func(t *testing.T) {
		ar := pending(t)
		var notFound *apierror.NotFoundError
		err := repo.DeleteAttachment(ctx, ar.ID, "88888888-8888-4888-8888-888888888888", domain.AnnouncementRequestStatePendingApproval, true)
		if !errors.As(err, &notFound) {
			t.Fatalf("expected a NotFoundError, got %v", err)
		}
		got, _ := repo.Get(ctx, ar.ID)
		if got.State != domain.AnnouncementRequestStatePendingApproval || got.DryRunCaseID == nil {
			t.Fatalf("a refused removal must not send the request back to draft, got state=%s", got.State)
		}
	})

	t.Run("removing a file works, and sends a request in review back to draft", func(t *testing.T) {
		ar := newRequest(t, repo)
		t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM announcement_requests WHERE id = $1`, ar.ID) })
		a, err := repo.AddAttachment(ctx, ar.ID, domain.AnnouncementRequestStateDraft, false, file("gone.pdf", 5))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `UPDATE announcement_requests SET state = 'pending_approval', dry_run_case_id = 'c', submitted_by = 'u1' WHERE id = $1`, ar.ID); err != nil {
			t.Fatal(err)
		}
		if err := repo.DeleteAttachment(ctx, ar.ID, a.ID, domain.AnnouncementRequestStatePendingApproval, true); err != nil {
			t.Fatalf("DeleteAttachment: %v", err)
		}
		got, _ := repo.Get(ctx, ar.ID)
		if got.State != domain.AnnouncementRequestStateDraft || got.DryRunCaseID != nil {
			t.Fatalf("expected a draft with no dry run, got state=%s", got.State)
		}
		if list, _ := repo.ListAttachments(ctx, ar.ID); len(list) != 0 {
			t.Fatalf("expected no files left, got %+v", list)
		}
	})

	t.Run("the files go with the request", func(t *testing.T) {
		ar := newRequest(t, repo)
		if _, err := repo.AddAttachment(ctx, ar.ID, domain.AnnouncementRequestStateDraft, false, file("cascade.pdf", 5)); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `DELETE FROM announcement_requests WHERE id = $1`, ar.ID); err != nil {
			t.Fatal(err)
		}
		var n int
		if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM announcement_request_attachment WHERE announcement_request_id = $1`, ar.ID).Scan(&n); err != nil || n != 0 {
			t.Fatalf("expected the files deleted with the request, got %d err=%v", n, err)
		}
	})
}
