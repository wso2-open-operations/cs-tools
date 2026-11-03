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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// newBackendSvc is the service in ATTACHMENT_STORAGE_MODE=backend. It has no file
// store at all, so a call that reached for one would panic on the nil: every test
// here therefore also shows the call never touches a file.
func newBackendSvc(repo *fakeStoredRepo) *storedAttachmentService {
	svc := NewBackendStoredAttachmentService(repo, StoredAttachmentLimits{MaxBytes: 1000, InlineMaxBytes: 100}).(*storedAttachmentService)
	svc.now = func() time.Time { return saNow }
	svc.shares.now = func() time.Time { return saNow }
	return svc
}

const workItemKey = "csm/projects/prj_9/workitems/cs0001/attachments/wia0000042_report.pdf"

// pendingUpload puts a PENDING row the caller started into the fake repository.
func pendingUpload(repo *fakeStoredRepo, size int64) {
	ct := "application/pdf"
	repo.rows[saAttID] = domain.StoredAttachmentRow{
		ID: saAttID, Number: "WIA0000042", Name: "report.pdf", ContentType: &ct, SizeBytes: &size,
		Status: "PENDING", CreatedBy: saEmail, ParentID: saParentID,
	}
}

func completeRow(status string, key *string) domain.StoredAttachmentRow {
	ct := "application/pdf"
	size := int64(500)
	return domain.StoredAttachmentRow{
		ID: saAttID, Number: "WIA0000042", Name: "report.pdf", ContentType: &ct, SizeBytes: &size,
		Status: status, StorageKey: key, CreatedBy: saEmail, ParentID: saParentID,
	}
}

func int64Ptr(n int64) *int64 { return &n }

func wantErr[T error](t *testing.T, err error, what string) {
	t.Helper()
	var target T
	if !errors.As(err, &target) {
		t.Fatalf("%s: err = %v, want %T", what, err, target)
	}
}

// ---- starting an upload ----

func TestBackend_InitiateUploadRecordsARowAndOpensNothing(t *testing.T) {
	repo := newFakeStoredRepo()
	svc := newBackendSvc(repo)

	resp, err := svc.InitiateUpload(saCtx(t, saEmail), initReq())
	if err != nil {
		t.Fatalf("InitiateUpload: %v", err)
	}
	if len(repo.inserted) != 1 || repo.inserted[0].CreatedBy != saEmail {
		t.Errorf("inserted = %+v, want one row created by the caller", repo.inserted)
	}
	if resp.AttachmentID != saAttID || resp.Number != "WIA0000042" || resp.MaxBytes != 1000 {
		t.Errorf("response = %+v", resp)
	}
	// There is no link: the portal backend holds the file.
	if resp.UploadURL != "" || resp.FieldName != "" {
		t.Errorf("response carries a link: %+v", resp)
	}
	if want := saNow.Add(abandonedUploadAge); !resp.ExpiresAt.Equal(want) {
		t.Errorf("ExpiresAt = %v, want %v (when an unconfirmed upload may be cleaned up)", resp.ExpiresAt, want)
	}
}

func TestBackend_InitiateUploadKeepsItsChecks(t *testing.T) {
	repo := newFakeStoredRepo()
	repo.parent.Closed = true
	wantErr[*apierror.ConflictError](t, errOf(newBackendSvc(repo).InitiateUpload(saCtx(t, saEmail), initReq())), "closed work item")

	repo = newFakeStoredRepo()
	req := initReq()
	req.SizeBytes = 1001
	wantErr[*apierror.ValidationError](t, errOf(newBackendSvc(repo).InitiateUpload(saCtx(t, saEmail), req)), "over the limit")
	if len(repo.inserted) != 0 {
		t.Error("a refused upload left a row behind")
	}
}

func errOf[T any](_ T, err error) error { return err }

func TestBackend_SFTPGoOnlyCallsAreNotAvailable(t *testing.T) {
	repo := newFakeStoredRepo()
	svc := newBackendSvc(repo)
	ctx := saCtx(t, saEmail)

	_, err := svc.ResolveLinks(ctx, domain.AttachmentLinksRequest{Kind: domain.StoredAttachmentWorkItem, IDs: []string{saAttID}})
	wantErr[*apierror.NotFoundError](t, err, "ResolveLinks")
	_, err = svc.OpenInlineImage(ctx, saAttID)
	wantErr[*apierror.NotFoundError](t, err, "OpenInlineImage")
}

func TestSFTPGoMode_BackendCallsAreNotAvailable(t *testing.T) {
	svc := newStoredSvc(newFakeStoredRepo(), newFakeStore())
	ctx := saCtx(t, saEmail)

	_, err := svc.AuthorizeUpload(ctx, domain.AuthorizeAttachmentUploadRequest{Kind: domain.StoredAttachmentWorkItem, ID: saAttID})
	wantErr[*apierror.NotFoundError](t, err, "AuthorizeUpload")
	_, err = svc.LocateAttachment(ctx, domain.LocateAttachmentRequest{Kind: domain.StoredAttachmentWorkItem, ID: saAttID})
	wantErr[*apierror.NotFoundError](t, err, "LocateAttachment")
	_, err = svc.LocateInlineImage(ctx, saAttID)
	wantErr[*apierror.NotFoundError](t, err, "LocateInlineImage")
}

// ---- authorizing a chunk ----

func TestBackend_AuthorizeUploadReturnsTheKeyToWriteTo(t *testing.T) {
	repo := newFakeStoredRepo()
	pendingUpload(repo, 500)

	target, err := newBackendSvc(repo).AuthorizeUpload(saCtx(t, saEmail), domain.AuthorizeAttachmentUploadRequest{Kind: domain.StoredAttachmentWorkItem, ID: saAttID})
	if err != nil {
		t.Fatalf("AuthorizeUpload: %v", err)
	}
	if target.StorageKey != workItemKey {
		t.Errorf("StorageKey = %q, want %q", target.StorageKey, workItemKey)
	}
	if target.AttachmentID != saAttID || target.SizeBytes != 500 || target.MaxBytes != 1000 || target.ContentType != "application/pdf" || target.Name != "report.pdf" {
		t.Errorf("target = %+v", target)
	}
}

func TestBackend_AuthorizeUploadKeyShapeFollowsTheKind(t *testing.T) {
	repo := newFakeStoredRepo()
	pendingUpload(repo, 50)
	svc := newBackendSvc(repo)

	target, err := svc.AuthorizeUpload(saCtx(t, saEmail), domain.AuthorizeAttachmentUploadRequest{Kind: domain.StoredAttachmentComment, ID: saAttID})
	if err != nil {
		t.Fatalf("comment: %v", err)
	}
	if !strings.Contains(target.StorageKey, "/inline_attachments/") || target.MaxBytes != 100 {
		t.Errorf("comment target = %+v, want an inline_attachments key and the inline limit", target)
	}

	repo.parent = domain.StoredAttachmentParent{ID: saParentID, Number: "DEP-7", ProjectNumber: "PRJ-9"}
	target, err = svc.AuthorizeUpload(saCtx(t, saEmail), domain.AuthorizeAttachmentUploadRequest{Kind: domain.StoredAttachmentDeployment, ID: saAttID})
	if err != nil {
		t.Fatalf("deployment: %v", err)
	}
	// The key builder lowercases a number and turns "-" into "_", as for "PRJ-9".
	if !strings.Contains(target.StorageKey, "/deployments/dep_7/") {
		t.Errorf("deployment key = %q, want it under /deployments/dep_7/", target.StorageKey)
	}
}

func TestBackend_AuthorizeUploadRefusals(t *testing.T) {
	req := domain.AuthorizeAttachmentUploadRequest{Kind: domain.StoredAttachmentWorkItem, ID: saAttID}

	t.Run("not the uploader", func(t *testing.T) {
		repo := newFakeStoredRepo()
		pendingUpload(repo, 500)
		_, err := newBackendSvc(repo).AuthorizeUpload(saCtx(t, "other@example.com"), req)
		wantErr[*apierror.NotFoundError](t, err, "someone else's upload")
	})
	t.Run("not visible to the caller", func(t *testing.T) {
		_, err := newBackendSvc(newFakeStoredRepo()).AuthorizeUpload(saCtx(t, saEmail), req)
		wantErr[*apierror.NotFoundError](t, err, "an unknown or hidden id")
	})
	t.Run("already confirmed", func(t *testing.T) {
		repo := newFakeStoredRepo()
		repo.rows[saAttID] = completeRow("COMPLETE", stringPtr(workItemKey))
		_, err := newBackendSvc(repo).AuthorizeUpload(saCtx(t, saEmail), req)
		wantErr[*apierror.ConflictError](t, err, "a confirmed upload")
	})
	t.Run("rejected", func(t *testing.T) {
		repo := newFakeStoredRepo()
		repo.rows[saAttID] = completeRow("FAILED", nil)
		_, err := newBackendSvc(repo).AuthorizeUpload(saCtx(t, saEmail), req)
		wantErr[*apierror.ConflictError](t, err, "a failed upload")
	})
	t.Run("work item closed meanwhile", func(t *testing.T) {
		repo := newFakeStoredRepo()
		pendingUpload(repo, 500)
		repo.parent.Closed = true
		_, err := newBackendSvc(repo).AuthorizeUpload(saCtx(t, saEmail), req)
		wantErr[*apierror.ConflictError](t, err, "a closed parent")
	})
	t.Run("parent no longer visible", func(t *testing.T) {
		repo := newFakeStoredRepo()
		pendingUpload(repo, 500)
		repo.parentErr = &apierror.NotFoundError{Msg: "parent not found"}
		_, err := newBackendSvc(repo).AuthorizeUpload(saCtx(t, saEmail), req)
		wantErr[*apierror.NotFoundError](t, err, "access removed part-way")
	})
	t.Run("bad input", func(t *testing.T) {
		svc := newBackendSvc(newFakeStoredRepo())
		_, err := svc.AuthorizeUpload(saCtx(t, saEmail), domain.AuthorizeAttachmentUploadRequest{Kind: "nope", ID: saAttID})
		wantErr[*apierror.ValidationError](t, err, "bad kind")
		_, err = svc.AuthorizeUpload(saCtx(t, saEmail), domain.AuthorizeAttachmentUploadRequest{Kind: domain.StoredAttachmentWorkItem, ID: "not-a-uuid"})
		wantErr[*apierror.ValidationError](t, err, "bad id")
		_, err = svc.AuthorizeUpload(context.Background(), req)
		wantErr[*apierror.UnauthorizedError](t, err, "no user token")
	})
}

// ---- confirming ----

func confirmReq(kind domain.StoredAttachmentKind, size int64, contentType string) domain.ConfirmAttachmentUploadRequest {
	return domain.ConfirmAttachmentUploadRequest{Kind: kind, ID: saAttID, SizeBytes: int64Ptr(size), ContentType: contentType}
}

func TestBackend_ConfirmCompletesTheRowWithoutTouchingStorage(t *testing.T) {
	repo := newFakeStoredRepo()
	pendingUpload(repo, 500)

	resp, err := newBackendSvc(repo).ConfirmUpload(saCtx(t, saEmail), confirmReq(domain.StoredAttachmentWorkItem, 500, ""))
	if err != nil {
		t.Fatalf("ConfirmUpload: %v", err)
	}
	if repo.completed[saAttID] != workItemKey {
		t.Errorf("completed key = %q, want %q", repo.completed[saAttID], workItemKey)
	}
	if len(repo.completeCtx) != 1 || !repo.completeCtx[0] {
		t.Error("MarkComplete must run as the system: UPDATE is internal-only on these tables")
	}
	if resp.Status != "COMPLETE" || resp.SizeBytes != 500 || resp.Number != "WIA0000042" {
		t.Errorf("response = %+v", resp)
	}
}

func TestBackend_ConfirmRejectsWhatDoesNotMatchTheUpload(t *testing.T) {
	cases := map[string]struct {
		size int64
		want string
	}{
		"size differs from the declaration": {499, "does not match"},
		"over the limit":                    {1001, "larger than"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			repo := newFakeStoredRepo()
			pendingUpload(repo, 500)
			_, err := newBackendSvc(repo).ConfirmUpload(saCtx(t, saEmail), confirmReq(domain.StoredAttachmentWorkItem, tc.size, ""))
			var verr *apierror.ValidationError
			if !errors.As(err, &verr) || !strings.Contains(verr.Msg, tc.want) {
				t.Fatalf("err = %v, want a ValidationError containing %q", err, tc.want)
			}
			if _, ok := repo.failed[saAttID]; !ok {
				t.Error("the rejected upload was not recorded FAILED, so it could be confirmed later")
			}
			if len(repo.completed) != 0 {
				t.Error("a rejected upload was completed")
			}
		})
	}
}

func TestBackend_ConfirmNeedsTheSize(t *testing.T) {
	repo := newFakeStoredRepo()
	pendingUpload(repo, 500)
	svc := newBackendSvc(repo)

	for name, size := range map[string]*int64{"missing": nil, "zero": int64Ptr(0), "negative": int64Ptr(-5)} {
		req := domain.ConfirmAttachmentUploadRequest{Kind: domain.StoredAttachmentWorkItem, ID: saAttID, SizeBytes: size}
		wantErr[*apierror.ValidationError](t, errOf(svc.ConfirmUpload(saCtx(t, saEmail), req)), name+" size")
	}
	// A malformed request is the caller's mistake, not a reason to throw the upload away.
	if _, failed := repo.failed[saAttID]; failed {
		t.Error("a request with no size marked the upload FAILED")
	}
}

func TestBackend_ConfirmChecksACommentImageByTheTypeTheBackendFound(t *testing.T) {
	for _, ct := range []string{"image/png", "image/jpeg", "IMAGE/WEBP", "image/png; charset=binary"} {
		repo := newFakeStoredRepo()
		pendingUpload(repo, 50)
		if _, err := newBackendSvc(repo).ConfirmUpload(saCtx(t, saEmail), confirmReq(domain.StoredAttachmentComment, 50, ct)); err != nil {
			t.Errorf("%q refused: %v", ct, err)
		}
	}

	for _, ct := range []string{"", "text/html", "image/svg+xml", "image/gif", "application/pdf"} {
		repo := newFakeStoredRepo()
		pendingUpload(repo, 50)
		_, err := newBackendSvc(repo).ConfirmUpload(saCtx(t, saEmail), confirmReq(domain.StoredAttachmentComment, 50, ct))
		wantErr[*apierror.ValidationError](t, err, "comment image of type "+ct)
		if _, ok := repo.failed[saAttID]; !ok {
			t.Errorf("%q: the rejected image was not recorded FAILED", ct)
		}
	}

	// Other kinds keep whatever type they declared: only comment images are sniffed.
	repo := newFakeStoredRepo()
	pendingUpload(repo, 50)
	if _, err := newBackendSvc(repo).ConfirmUpload(saCtx(t, saEmail), confirmReq(domain.StoredAttachmentWorkItem, 50, "")); err != nil {
		t.Errorf("a work item attachment needs no reported type: %v", err)
	}
}

func TestBackend_ConfirmRefusesAWorkItemClosedDuringTheUpload(t *testing.T) {
	repo := newFakeStoredRepo()
	pendingUpload(repo, 500)
	repo.parent.Closed = true

	_, err := newBackendSvc(repo).ConfirmUpload(saCtx(t, saEmail), confirmReq(domain.StoredAttachmentWorkItem, 500, ""))
	wantErr[*apierror.ConflictError](t, err, "a closed parent")
	if _, ok := repo.failed[saAttID]; !ok {
		t.Error("the upload to a closed work item was not recorded FAILED")
	}
}

func TestBackend_ConfirmIsTheUploadersAlone(t *testing.T) {
	repo := newFakeStoredRepo()
	pendingUpload(repo, 500)
	_, err := newBackendSvc(repo).ConfirmUpload(saCtx(t, "other@example.com"), confirmReq(domain.StoredAttachmentWorkItem, 500, ""))
	wantErr[*apierror.NotFoundError](t, err, "someone else's confirm")
	if len(repo.completed) != 0 {
		t.Error("someone else confirmed the upload")
	}
}

func TestBackend_ConfirmRepeatedAndConcurrent(t *testing.T) {
	// A retry after a timeout answers the same and writes nothing.
	repo := newFakeStoredRepo()
	repo.rows[saAttID] = completeRow("COMPLETE", stringPtr(workItemKey))
	resp, err := newBackendSvc(repo).ConfirmUpload(saCtx(t, saEmail), confirmReq(domain.StoredAttachmentWorkItem, 500, ""))
	if err != nil || resp.Status != "COMPLETE" || resp.SizeBytes != 500 {
		t.Fatalf("repeat = %+v, %v", resp, err)
	}
	if len(repo.completeCtx) != 0 {
		t.Error("a repeated confirm wrote again")
	}

	// Another request won the race to confirm.
	repo = newFakeStoredRepo()
	pendingUpload(repo, 500)
	repo.completeOK = false
	_, err = newBackendSvc(repo).ConfirmUpload(saCtx(t, saEmail), confirmReq(domain.StoredAttachmentWorkItem, 500, ""))
	wantErr[*apierror.ConflictError](t, err, "a concurrent confirm")

	// A rejected upload cannot be confirmed later.
	repo = newFakeStoredRepo()
	repo.rows[saAttID] = completeRow("FAILED", nil)
	_, err = newBackendSvc(repo).ConfirmUpload(saCtx(t, saEmail), confirmReq(domain.StoredAttachmentWorkItem, 500, ""))
	wantErr[*apierror.ConflictError](t, err, "a failed upload")
}

// ---- locating a file to read ----

func TestBackend_LocateAttachmentReturnsTheKey(t *testing.T) {
	repo := newFakeStoredRepo()
	repo.rows[saAttID] = completeRow("COMPLETE", stringPtr(workItemKey))

	loc, err := newBackendSvc(repo).LocateAttachment(saCtx(t, saEmail), domain.LocateAttachmentRequest{Kind: domain.StoredAttachmentWorkItem, ID: saAttID})
	if err != nil {
		t.Fatalf("LocateAttachment: %v", err)
	}
	if !loc.Available || loc.StorageKey != workItemKey || loc.Name != "report.pdf" || loc.Number != "WIA0000042" {
		t.Errorf("location = %+v", loc)
	}
	if loc.ContentType == nil || *loc.ContentType != "application/pdf" || loc.SizeBytes == nil || *loc.SizeBytes != 500 {
		t.Errorf("metadata missing: %+v", loc)
	}
}

func TestBackend_LocateAttachmentGivesAReasonNotAKeyWhenThereIsNoFile(t *testing.T) {
	cases := map[string]struct {
		row    domain.StoredAttachmentRow
		reason string
	}{
		"deleted":            {completeRow("DELETED", stringPtr(workItemKey)), domain.AttachmentUnavailableDeleted},
		"failed":             {completeRow("FAILED", nil), domain.AttachmentUnavailableFailed},
		"never confirmed":    {completeRow("PENDING", nil), domain.AttachmentUnavailableNotStored},
		"complete, no key":   {completeRow("COMPLETE", nil), domain.AttachmentUnavailableNotStored},
		"complete, blank":    {completeRow("COMPLETE", stringPtr("")), domain.AttachmentUnavailableNotStored},
		"source was missing": {completeRow("SOURCE_MISSING", nil), domain.AttachmentUnavailableNotStored},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			repo := newFakeStoredRepo()
			repo.rows[saAttID] = tc.row
			loc, err := newBackendSvc(repo).LocateAttachment(saCtx(t, saEmail), domain.LocateAttachmentRequest{Kind: domain.StoredAttachmentWorkItem, ID: saAttID})
			if err != nil {
				t.Fatalf("LocateAttachment: %v", err)
			}
			if loc.Available || loc.StorageKey != "" || loc.Reason != tc.reason {
				t.Errorf("location = %+v, want unavailable with %s and no key", loc, tc.reason)
			}
		})
	}
}

func TestBackend_LocateAttachmentHidesWhatTheCallerCannotSee(t *testing.T) {
	svc := newBackendSvc(newFakeStoredRepo())
	_, err := svc.LocateAttachment(saCtx(t, saEmail), domain.LocateAttachmentRequest{Kind: domain.StoredAttachmentWorkItem, ID: saAttID})
	wantErr[*apierror.NotFoundError](t, err, "an unknown or hidden id")

	_, err = svc.LocateAttachment(saCtx(t, saEmail), domain.LocateAttachmentRequest{Kind: "nope", ID: saAttID})
	wantErr[*apierror.ValidationError](t, err, "bad kind")
	_, err = svc.LocateAttachment(saCtx(t, saEmail), domain.LocateAttachmentRequest{Kind: domain.StoredAttachmentWorkItem, ID: "x"})
	wantErr[*apierror.ValidationError](t, err, "bad id")
}

func TestBackend_LocateAttachmentIsChargedToTheCallersBudget(t *testing.T) {
	repo := newFakeStoredRepo()
	repo.rows[saAttID] = completeRow("COMPLETE", stringPtr(workItemKey))
	svc := newBackendSvc(repo)
	ctx := saCtx(t, saEmail)
	req := domain.LocateAttachmentRequest{Kind: domain.StoredAttachmentWorkItem, ID: saAttID}

	for i := 0; i < maxSharesPerCallerPerMinute; i++ {
		if _, err := svc.LocateAttachment(ctx, req); err != nil {
			t.Fatalf("call %d refused early: %v", i, err)
		}
	}
	_, err := svc.LocateAttachment(ctx, req)
	wantErr[*apierror.TooManyRequestsError](t, err, "the call over the budget")

	svc.shares.now = func() time.Time { return saNow.Add(2 * time.Minute) }
	if _, err := svc.LocateAttachment(ctx, req); err != nil {
		t.Errorf("after the window: %v", err)
	}
}

func TestBackend_LocateInlineImage(t *testing.T) {
	key := "csm/projects/prj_9/workitems/cs0001/inline_attachments/cia1_pic.png"
	png := "image/png"

	t.Run("a comment image", func(t *testing.T) {
		repo := newFakeStoredRepo()
		repo.rows[saAttID] = domain.StoredAttachmentRow{ID: saAttID, Number: "CIA1", Name: "pic.png", ContentType: &png, Status: "COMPLETE", StorageKey: &key}
		loc, err := newBackendSvc(repo).LocateInlineImage(saCtx(t, saEmail), saAttID)
		if err != nil {
			t.Fatalf("LocateInlineImage: %v", err)
		}
		if !loc.Available || loc.StorageKey != key || loc.ContentType == nil || *loc.ContentType != "image/png" {
			t.Errorf("location = %+v", loc)
		}
	})

	t.Run("a type that is not an image is sent as octet-stream", func(t *testing.T) {
		repo := newFakeStoredRepo()
		odd := "text/html"
		repo.rows[saAttID] = domain.StoredAttachmentRow{ID: saAttID, Name: "x", ContentType: &odd, Status: "COMPLETE", StorageKey: &key}
		loc, err := newBackendSvc(repo).LocateInlineImage(saCtx(t, saEmail), saAttID)
		if err != nil {
			t.Fatalf("LocateInlineImage: %v", err)
		}
		if loc.ContentType == nil || *loc.ContentType != "application/octet-stream" {
			t.Errorf("ContentType = %v, want application/octet-stream", loc.ContentType)
		}
	})

	t.Run("a migrated image is a work item attachment", func(t *testing.T) {
		repo := newFakeStoredRepo()
		migrated := "image/jpeg; charset=binary"
		repo.workItemRows = map[string]domain.StoredAttachmentRow{saAttID: {ID: saAttID, Name: "m.jpg", ContentType: &migrated, Status: "COMPLETE", StorageKey: &key}}
		loc, err := newBackendSvc(repo).LocateInlineImage(saCtx(t, saEmail), saAttID)
		if err != nil {
			t.Fatalf("LocateInlineImage: %v", err)
		}
		if loc.ContentType == nil || *loc.ContentType != "image/jpeg" {
			t.Errorf("ContentType = %v, want only the media type", loc.ContentType)
		}
	})

	t.Run("a work item file that is not an image is not shown", func(t *testing.T) {
		repo := newFakeStoredRepo()
		pdf := "application/pdf"
		repo.workItemRows = map[string]domain.StoredAttachmentRow{saAttID: {ID: saAttID, Name: "a.pdf", ContentType: &pdf, Status: "COMPLETE", StorageKey: &key}}
		_, err := newBackendSvc(repo).LocateInlineImage(saCtx(t, saEmail), saAttID)
		wantErr[*apierror.NotFoundError](t, err, "a pdf")
	})

	t.Run("a removed image is gone", func(t *testing.T) {
		repo := newFakeStoredRepo()
		repo.rows[saAttID] = domain.StoredAttachmentRow{ID: saAttID, Name: "pic.png", ContentType: &png, Status: "DELETED", StorageKey: &key}
		_, err := newBackendSvc(repo).LocateInlineImage(saCtx(t, saEmail), saAttID)
		wantErr[*apierror.GoneError](t, err, "a deleted image")
	})

	t.Run("not stored, hidden or malformed", func(t *testing.T) {
		repo := newFakeStoredRepo()
		repo.rows[saAttID] = domain.StoredAttachmentRow{ID: saAttID, Name: "pic.png", ContentType: &png, Status: "PENDING"}
		svc := newBackendSvc(repo)
		_, err := svc.LocateInlineImage(saCtx(t, saEmail), saAttID)
		wantErr[*apierror.NotFoundError](t, err, "an upload still pending")
		_, err = svc.LocateInlineImage(saCtx(t, saEmail), "b1000000-0000-0000-0000-0000000000ff")
		wantErr[*apierror.NotFoundError](t, err, "an unknown id")
		_, err = svc.LocateInlineImage(saCtx(t, saEmail), "nope")
		wantErr[*apierror.ValidationError](t, err, "a malformed id")
	})
}

// ---- the rest of the flow, without a file store ----

func TestBackend_DeleteMarksTheRowAndLeavesTheFileToTheBackend(t *testing.T) {
	repo := newFakeStoredRepo()
	repo.rows[saAttID] = completeRow("COMPLETE", stringPtr(workItemKey))

	err := newBackendSvc(repo).DeleteAttachment(saCtx(t, saEmail), domain.DeleteStoredAttachmentRequest{Kind: domain.StoredAttachmentWorkItem, ID: saAttID})
	if err != nil {
		t.Fatalf("DeleteAttachment: %v", err)
	}
	if repo.deleted[saAttID] != saEmail {
		t.Errorf("deleted = %v, want the row marked deleted by the caller", repo.deleted)
	}
	if len(repo.deleteCtx) != 1 || !repo.deleteCtx[0] {
		t.Error("MarkDeleted must run as the system: UPDATE is internal-only on these tables")
	}
}

func TestBackend_DeleteKeepsItsRules(t *testing.T) {
	repo := newFakeStoredRepo()
	repo.rows[saAttID] = completeRow("COMPLETE", stringPtr(workItemKey))
	err := newBackendSvc(repo).DeleteAttachment(saCtx(t, "other@example.com"), domain.DeleteStoredAttachmentRequest{Kind: domain.StoredAttachmentWorkItem, ID: saAttID})
	wantErr[*apierror.ForbiddenError](t, err, "someone else's attachment")

	repo.parent.Closed = true
	err = newBackendSvc(repo).DeleteAttachment(saCtx(t, saEmail), domain.DeleteStoredAttachmentRequest{Kind: domain.StoredAttachmentWorkItem, ID: saAttID})
	wantErr[*apierror.ConflictError](t, err, "a closed work item")
	if len(repo.deleted) != 0 {
		t.Error("an attachment was deleted from a closed work item")
	}
}

func TestBackend_EmbeddedImagesAreLeftAsWritten(t *testing.T) {
	// 1x1 PNG as a data URI, the shape a pasted image has.
	content := `<p>hi</p><img src="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="/>`
	repo := newFakeStoredRepo()

	got, err := newBackendSvc(repo).ExtractInlineImages(saCtx(t, saEmail), saParentID, saEmail, content)
	if err != nil {
		t.Fatalf("ExtractInlineImages: %v", err)
	}
	if got != content {
		t.Errorf("content was changed: %q", got)
	}
	if len(repo.inserted) != 0 {
		t.Error("a row was written for an image this service cannot store")
	}
}

func TestBackend_CleanupMarksRowsAndReportsTheKeysItDropped(t *testing.T) {
	repo := newFakeStoredRepo()
	repo.abandoned = []domain.AbandonedUpload{{Kind: domain.StoredAttachmentWorkItem, ID: "11111111-1111-4111-8111-111111111111"}}
	k1, k2 := "csm/projects/p/workitems/w/inline_attachments/a_1.png", "csm/projects/p/workitems/w/inline_attachments/a_2.png"
	repo.unlinked = []domain.StoredAttachmentRow{
		{ID: "22222222-2222-4222-8222-222222222222", StorageKey: &k1},
		{ID: "33333333-3333-4333-8333-333333333333", StorageKey: &k2},
		{ID: "44444444-4444-4444-8444-444444444444"}, // no key: nothing to report
	}
	// A nil store is how backend mode builds it; a call to it would panic.
	svc := NewAttachmentUploadCleanupService(repo, nil, alwaysUnrestrictedAccess{}).(*attachmentUploadCleanupService)
	svc.now = func() time.Time { return saNow }

	res, err := svc.CleanupUploads(context.Background())
	if err != nil {
		t.Fatalf("CleanupUploads: %v", err)
	}
	if res.Found != 1 || res.Cleaned != 1 || res.Failed != 0 || res.ImagesRemoved != 3 || res.ImagesFailed != 0 {
		t.Errorf("result = %+v", res)
	}
	if _, ok := repo.failed["11111111-1111-4111-8111-111111111111"]; !ok {
		t.Error("the abandoned row was not marked FAILED")
	}
	if len(res.OrphanedKeys) != 2 || res.OrphanedKeys[0] != k1 || res.OrphanedKeys[1] != k2 {
		t.Errorf("OrphanedKeys = %v, want the two keys of the images that were dropped", res.OrphanedKeys)
	}
}

func TestSFTPGoModeCleanupReportsNoKeys(t *testing.T) {
	repo := newFakeStoredRepo()
	k := "csm/projects/p/workitems/w/inline_attachments/a_1.png"
	repo.unlinked = []domain.StoredAttachmentRow{{ID: "22222222-2222-4222-8222-222222222222", StorageKey: &k}}
	svc := NewAttachmentUploadCleanupService(repo, &fakeUploadStore{}, alwaysUnrestrictedAccess{}).(*attachmentUploadCleanupService)
	svc.now = func() time.Time { return saNow }

	res, err := svc.CleanupUploads(context.Background())
	if err != nil {
		t.Fatalf("CleanupUploads: %v", err)
	}
	if res.ImagesRemoved != 1 || len(res.OrphanedKeys) != 0 {
		t.Errorf("result = %+v: with SFTPGo the file is removed here, so there is nothing to report", res)
	}
}
