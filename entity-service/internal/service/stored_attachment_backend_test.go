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

// newBackendSvc is the service under test. It has no file store at all: the
// portal backends hold the files, so every test here also shows the call never
// touches a file.
func newBackendSvc(repo *fakeStoredRepo) *storedAttachmentService {
	return newStoredSvc(repo)
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

// A file of exactly the limit is accepted, at the upload and at the confirm;
// only a byte more is refused.
func TestBackend_AFileOfExactlyTheLimitIsAccepted(t *testing.T) {
	req := initReq()
	req.SizeBytes = 1000
	if _, err := newBackendSvc(newFakeStoredRepo()).InitiateUpload(saCtx(t, saEmail), req); err != nil {
		t.Errorf("an attachment of exactly the limit: %v", err)
	}
	req = initReq()
	req.Kind, req.ContentType, req.SizeBytes = domain.StoredAttachmentComment, "image/png", 100
	if _, err := newBackendSvc(newFakeStoredRepo()).InitiateUpload(saCtx(t, saEmail), req); err != nil {
		t.Errorf("a comment image of exactly the limit: %v", err)
	}
	req.SizeBytes = 101
	wantErr[*apierror.ValidationError](t, errOf(newBackendSvc(newFakeStoredRepo()).InitiateUpload(saCtx(t, saEmail), req)), "a byte over the image limit")

	repo := newFakeStoredRepo()
	pendingUpload(repo, 1000)
	if _, err := newBackendSvc(repo).ConfirmUpload(saCtx(t, saEmail), confirmReq(domain.StoredAttachmentWorkItem, 1000, "")); err != nil {
		t.Errorf("confirming an attachment of exactly the limit: %v", err)
	}
}

func errOf[T any](_ T, err error) error { return err }

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

	_, err := newBackendSvc(repo).DeleteAttachment(saCtx(t, saEmail), domain.DeleteStoredAttachmentRequest{Kind: domain.StoredAttachmentWorkItem, ID: saAttID})
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
	_, err := newBackendSvc(repo).DeleteAttachment(saCtx(t, "other@example.com"), domain.DeleteStoredAttachmentRequest{Kind: domain.StoredAttachmentWorkItem, ID: saAttID})
	wantErr[*apierror.ForbiddenError](t, err, "someone else's attachment")

	repo.parent.Closed = true
	_, err = newBackendSvc(repo).DeleteAttachment(saCtx(t, saEmail), domain.DeleteStoredAttachmentRequest{Kind: domain.StoredAttachmentWorkItem, ID: saAttID})
	wantErr[*apierror.ConflictError](t, err, "a closed work item")
	if len(repo.deleted) != 0 {
		t.Error("an attachment was deleted from a closed work item")
	}
}

const (
	orphanA      = "22222222-2222-4222-8222-222222222222"
	orphanB      = "33333333-3333-4333-8333-333333333333"
	keyless      = "44444444-4444-4444-8444-444444444444"
	reportDraftA = "66666666-6666-4666-8666-666666666666"
)

// backendCleanup builds the cleanup service as backend mode does: a nil store, so
// a call to it would panic.
func backendCleanup(repo *fakeStoredRepo) *attachmentUploadCleanupService {
	svc := NewAttachmentUploadCleanupService(repo, alwaysUnrestrictedAccess{}).(*attachmentUploadCleanupService)
	svc.now = func() time.Time { return saNow }
	return svc
}

func orphanRepo() *fakeStoredRepo {
	repo := newFakeStoredRepo()
	repo.abandoned = []domain.AbandonedUpload{{Kind: domain.StoredAttachmentWorkItem, ID: "11111111-1111-4111-8111-111111111111"}}
	k1, k2 := "csm/projects/p/workitems/w/inline_attachments/a_1.png", "csm/projects/p/workitems/w/inline_attachments/a_2.png"
	repo.unlinked = []domain.StoredAttachmentRow{
		{ID: orphanA, StorageKey: &k1},
		{ID: orphanB, StorageKey: &k2},
		{ID: keyless}, // no key: nothing in storage to remove
	}
	return repo
}

func TestBackend_CleanupHandsOrphansOverWithoutMarkingThemDeleted(t *testing.T) {
	repo := orphanRepo()
	svc := backendCleanup(repo)

	res, err := svc.CleanupUploads(context.Background())
	if err != nil {
		t.Fatalf("CleanupUploads: %v", err)
	}
	if res.Found != 1 || res.Cleaned != 1 || res.Failed != 0 {
		t.Errorf("abandoned uploads: %+v", res)
	}
	if _, ok := repo.failed["11111111-1111-4111-8111-111111111111"]; !ok {
		t.Error("the abandoned row was not marked FAILED")
	}
	if len(res.Orphans) != 2 || res.Orphans[0].ID != orphanA || res.Orphans[1].ID != orphanB ||
		!strings.HasSuffix(res.Orphans[0].StorageKey, "a_1.png") || !strings.HasSuffix(res.Orphans[1].StorageKey, "a_2.png") {
		t.Errorf("Orphans = %+v, want both images that have a file", res.Orphans)
	}
	// Only the image with no file is finished here. The others stay COMPLETE and
	// unlinked, so a lost response cannot lose their keys: the next run lists them again.
	if len(repo.deleted) != 1 || repo.deleted[keyless] == "" {
		t.Errorf("deleted = %v, want only the image with no file marked DELETED", repo.deleted)
	}
	if res.ImagesRemoved != 1 || res.ImagesFailed != 0 {
		t.Errorf("ImagesRemoved = %d, ImagesFailed = %d, want 1 and 0", res.ImagesRemoved, res.ImagesFailed)
	}
}

func TestBackend_CleanupListsTheSameOrphansAgainUntilAcknowledged(t *testing.T) {
	repo := orphanRepo()
	svc := backendCleanup(repo)

	// The first response is lost. Nothing was recorded as done, so the next run
	// hands over the same two keys.
	first, err := svc.CleanupUploads(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.CleanupUploads(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Orphans) != 2 || len(second.Orphans) != 2 || first.Orphans[0] != second.Orphans[0] || first.Orphans[1] != second.Orphans[1] {
		t.Fatalf("first = %+v, second = %+v, want the same orphans each run", first.Orphans, second.Orphans)
	}

	// Once the caller has removed the files and acknowledged them, they are done.
	ack, err := svc.AcknowledgeOrphans(context.Background(), domain.AcknowledgeOrphansRequest{IDs: []string{orphanA, orphanB}})
	if err != nil {
		t.Fatalf("AcknowledgeOrphans: %v", err)
	}
	if ack.Acknowledged != 2 || ack.Skipped != 0 {
		t.Errorf("ack = %+v", ack)
	}
	if repo.deleted[orphanA] == "" || repo.deleted[orphanB] == "" {
		t.Errorf("deleted = %v, want both acknowledged images marked DELETED", repo.deleted)
	}
	if len(repo.deleteCtx) == 0 || !repo.deleteCtx[len(repo.deleteCtx)-1] {
		t.Error("MarkDeleted must run as the system: UPDATE is internal-only on these tables")
	}
}

func TestBackend_AcknowledgeOnlyTouchesImagesThatAreStillOrphans(t *testing.T) {
	repo := orphanRepo()
	svc := backendCleanup(repo)
	linkedMeanwhile := "55555555-5555-4555-8555-555555555555" // not in the unlinked list any more

	ack, err := svc.AcknowledgeOrphans(context.Background(), domain.AcknowledgeOrphansRequest{
		IDs: []string{orphanA, strings.ToUpper(orphanA), linkedMeanwhile, "b1000000-0000-0000-0000-000000000001"},
	})
	if err != nil {
		t.Fatalf("AcknowledgeOrphans: %v", err)
	}
	// A repeated id counts once; an image that is no longer an orphan, or never was,
	// is skipped, so this cannot be used to delete an arbitrary image.
	if ack.Acknowledged != 1 || ack.Skipped != 2 {
		t.Errorf("ack = %+v, want 1 acknowledged and 2 skipped", ack)
	}
	if len(repo.deleted) != 1 || repo.deleted[orphanA] == "" {
		t.Errorf("deleted = %v, want only the one real orphan", repo.deleted)
	}
}

func TestBackend_AcknowledgeValidationAndAccess(t *testing.T) {
	svc := backendCleanup(orphanRepo())
	ctx := context.Background()

	_, err := svc.AcknowledgeOrphans(ctx, domain.AcknowledgeOrphansRequest{})
	wantErr[*apierror.ValidationError](t, err, "no ids")
	_, err = svc.AcknowledgeOrphans(ctx, domain.AcknowledgeOrphansRequest{IDs: []string{"not-a-uuid"}})
	wantErr[*apierror.ValidationError](t, err, "a malformed id")
	tooMany := make([]string, uploadCleanupMaxPerRun+1)
	for i := range tooMany {
		tooMany[i] = orphanA
	}
	_, err = svc.AcknowledgeOrphans(ctx, domain.AcknowledgeOrphansRequest{IDs: tooMany})
	wantErr[*apierror.ValidationError](t, err, "too many ids")

	restricted := NewAttachmentUploadCleanupService(orphanRepo(), stubAccess{})
	_, err = restricted.AcknowledgeOrphans(ctx, domain.AcknowledgeOrphansRequest{IDs: []string{orphanA}})
	wantErr[*apierror.ForbiddenError](t, err, "a restricted caller")
}

func TestBackend_DescriptionImageRelocation(t *testing.T) {
	png := "image/png"
	drafts := "csm/projects/prj_9/workitems/drafts/inline_attachments/cia1_pic.png"
	canonical := "csm/projects/prj_9/workitems/cs0001/inline_attachments/cia1_pic.png"
	setup := func() *fakeStoredRepo {
		repo := newFakeStoredRepo()
		repo.parent = domain.StoredAttachmentParent{ID: saParentID, Number: "CS0001", ProjectNumber: "PRJ-9"}
		repo.rows[saAttID] = domain.StoredAttachmentRow{
			ID: saAttID, Number: "CIA1", Name: "pic.png", ContentType: &png, Status: "COMPLETE",
			StorageKey: &drafts, ParentID: saParentID,
		}
		return repo
	}

	t.Run("a linked description image still in drafts is told where it belongs", func(t *testing.T) {
		loc, err := newBackendSvc(setup()).LocateInlineImage(saCtx(t, saEmail), saAttID)
		if err != nil {
			t.Fatalf("LocateInlineImage: %v", err)
		}
		if loc.StorageKey != drafts || loc.RelocateTo != canonical {
			t.Errorf("location = %+v, want key %q and relocateTo %q", loc, drafts, canonical)
		}
	})

	t.Run("a failed lookup of where it belongs still shows the image where it is", func(t *testing.T) {
		repo := setup()
		repo.parentErr = errors.New("database down")
		loc, err := newBackendSvc(repo).LocateInlineImage(saCtx(t, saEmail), saAttID)
		if err != nil {
			t.Fatalf("LocateInlineImage: %v, want the location without relocateTo", err)
		}
		if !loc.Available || loc.StorageKey != drafts || loc.RelocateTo != "" {
			t.Errorf("location = %+v", loc)
		}
		// Recording a move still reports the failure.
		err = newBackendSvc(repo).InlineImageRelocated(saCtx(t, saEmail), saAttID, domain.InlineImageRelocatedRequest{StorageKey: canonical})
		if err == nil {
			t.Error("InlineImageRelocated hid the lookup failure")
		}
	})

	t.Run("an image already under its work item is not moved", func(t *testing.T) {
		repo := setup()
		repo.rows[saAttID] = domain.StoredAttachmentRow{ID: saAttID, Number: "CIA1", Name: "pic.png", ContentType: &png, Status: "COMPLETE", StorageKey: &canonical, ParentID: saParentID}
		loc, err := newBackendSvc(repo).LocateInlineImage(saCtx(t, saEmail), saAttID)
		if err != nil || loc.RelocateTo != "" {
			t.Errorf("loc = %+v, err = %v, want no relocation", loc, err)
		}
	})

	t.Run("a draft that is not linked yet is not moved", func(t *testing.T) {
		repo := setup()
		row := repo.rows[saAttID]
		row.Draft = true
		repo.rows[saAttID] = row
		loc, err := newBackendSvc(repo).LocateInlineImage(saCtx(t, saEmail), saAttID)
		if err != nil || loc.RelocateTo != "" {
			t.Errorf("loc = %+v, err = %v, want no relocation", loc, err)
		}
	})

	t.Run("relocated points the row at the computed key, as the system", func(t *testing.T) {
		repo := setup()
		err := newBackendSvc(repo).InlineImageRelocated(saCtx(t, saEmail), saAttID, domain.InlineImageRelocatedRequest{StorageKey: canonical})
		if err != nil {
			t.Fatalf("InlineImageRelocated: %v", err)
		}
		if len(repo.keyUpdates) != 1 || repo.keyUpdates[0].from != drafts || repo.keyUpdates[0].to != canonical || !repo.keyUpdates[0].sys {
			t.Errorf("key updates = %+v", repo.keyUpdates)
		}
	})

	t.Run("relocated refuses any other key", func(t *testing.T) {
		repo := setup()
		err := newBackendSvc(repo).InlineImageRelocated(saCtx(t, saEmail), saAttID, domain.InlineImageRelocatedRequest{StorageKey: "csm/projects/other/secret.bin"})
		wantErr[*apierror.ValidationError](t, err, "a key this service did not name")
		if len(repo.keyUpdates) != 0 {
			t.Errorf("key was changed: %+v", repo.keyUpdates)
		}
	})

	t.Run("a repeated relocated call answers the same", func(t *testing.T) {
		repo := setup()
		repo.rows[saAttID] = domain.StoredAttachmentRow{ID: saAttID, Number: "CIA1", Name: "pic.png", ContentType: &png, Status: "COMPLETE", StorageKey: &canonical, ParentID: saParentID}
		err := newBackendSvc(repo).InlineImageRelocated(saCtx(t, saEmail), saAttID, domain.InlineImageRelocatedRequest{StorageKey: canonical})
		if err != nil || len(repo.keyUpdates) != 0 {
			t.Errorf("err = %v, updates = %+v", err, repo.keyUpdates)
		}
	})
}

// An inline image's location names the record it belongs to, so a caller that
// reads files as an internal caller can check it against the case it is about.
func TestBackend_InlineImageLocationNamesItsParent(t *testing.T) {
	png := "image/png"
	key := "csm/projects/prj_9/workitems/cs0001/inline_attachments/cia1_pic.png"
	repo := newFakeStoredRepo()
	repo.parent = domain.StoredAttachmentParent{ID: saParentID, Number: "CS0001", ProjectNumber: "PRJ-9"}
	repo.rows[saAttID] = domain.StoredAttachmentRow{
		ID: saAttID, Number: "CIA1", Name: "pic.png", ContentType: &png, Status: "COMPLETE",
		StorageKey: &key, ParentID: saParentID,
	}
	loc, err := newBackendSvc(repo).LocateInlineImage(saCtx(t, saEmail), saAttID)
	if err != nil {
		t.Fatalf("LocateInlineImage: %v", err)
	}
	if loc.ParentID != saParentID || loc.Draft {
		t.Errorf("parent = %q draft = %v, want %q and false", loc.ParentID, loc.Draft, saParentID)
	}
}

func TestBackend_CleanupHandsOverUnlinkedReportDrafts(t *testing.T) {
	repo := orphanRepo()
	k := "csm/projects/p/workitems/drafts/attachments/r_1.pdf"
	repo.unlinkedReports = []domain.StoredAttachmentRow{{ID: reportDraftA, StorageKey: &k}}
	svc := backendCleanup(repo)

	res, err := svc.CleanupUploads(context.Background())
	if err != nil {
		t.Fatalf("CleanupUploads: %v", err)
	}
	var got *domain.OrphanedImage
	for i := range res.Orphans {
		if res.Orphans[i].ID == reportDraftA {
			got = &res.Orphans[i]
		}
	}
	if got == nil || got.Type != domain.StoredAttachmentWorkItem || got.StorageKey != k {
		t.Fatalf("orphans = %+v, want the report draft handed over as a work_item file", res.Orphans)
	}
	if repo.deleted[reportDraftA] != "" {
		t.Error("a draft with a file must not be marked DELETED before the caller removed the file")
	}
	for _, o := range res.Orphans {
		if o.ID == orphanA && o.Type != domain.StoredAttachmentComment {
			t.Errorf("a comment image must be typed comment, got %q", o.Type)
		}
	}
}

func TestBackend_CleanupUsesTheGraceAfterTheDraftsUsableAge(t *testing.T) {
	if reportDraftCleanupAge <= reportDraftUsableAge {
		t.Fatalf("cleanup age %v must be longer than the usable age %v, or a draft could be linked after its file was handed over", reportDraftCleanupAge, reportDraftUsableAge)
	}
}

func TestBackend_AcknowledgeMarksAReportDraftDeletedAsAWorkItemFile(t *testing.T) {
	repo := orphanRepo()
	k := "csm/projects/p/workitems/drafts/attachments/r_1.pdf"
	repo.unlinkedReports = []domain.StoredAttachmentRow{{ID: reportDraftA, StorageKey: &k}}
	svc := backendCleanup(repo)

	ack, err := svc.AcknowledgeOrphans(context.Background(), domain.AcknowledgeOrphansRequest{IDs: []string{reportDraftA, orphanA}})
	if err != nil {
		t.Fatalf("AcknowledgeOrphans: %v", err)
	}
	if ack.Acknowledged != 2 || ack.Skipped != 0 {
		t.Errorf("ack = %+v", ack)
	}
	if repo.deleted[reportDraftA] != "report-draft-cleanup" || repo.deleted[orphanA] != "inline-image-cleanup" {
		t.Errorf("deleted = %v, want each marked by its own cleanup", repo.deleted)
	}
}

// withBlob gives a cleanup service a file store, so it removes unlinked files itself.
func withBlob(repo *fakeStoredRepo, blob *fakeBlob) *attachmentUploadCleanupService {
	return WithFileStore(backendCleanup(repo), blob).(*attachmentUploadCleanupService)
}

func TestBackend_CleanupWithAStoreRemovesUnlinkedFilesItself(t *testing.T) {
	repo := orphanRepo()
	k := "csm/projects/p/workitems/drafts/attachments/r_1.pdf"
	repo.unlinkedReports = []domain.StoredAttachmentRow{{ID: reportDraftA, StorageKey: &k}}
	blob := newFakeBlob()
	for _, key := range []string{"csm/projects/p/workitems/w/inline_attachments/a_1.png", "csm/projects/p/workitems/w/inline_attachments/a_2.png", k} {
		blob.committed[key] = []byte("x")
	}

	res, err := withBlob(repo, blob).CleanupUploads(context.Background())
	if err != nil {
		t.Fatalf("CleanupUploads: %v", err)
	}
	if len(res.Orphans) != 0 {
		t.Errorf("orphans = %+v, want none: the files were removed here", res.Orphans)
	}
	// Two stored comment images and one keyless one, and the report draft.
	if res.ImagesRemoved != 3 || res.ImagesFailed != 0 || res.ReportDraftsRemoved != 1 || res.ReportDraftsFailed != 0 {
		t.Errorf("result = %+v", res)
	}
	if len(blob.committed) != 0 || len(blob.deleted) != 3 {
		t.Errorf("files left %v, deleted %v: every stored file must be removed", blob.committed, blob.deleted)
	}
	if repo.deleted[orphanA] != "inline-image-cleanup" || repo.deleted[orphanB] != "inline-image-cleanup" || repo.deleted[reportDraftA] != "report-draft-cleanup" {
		t.Errorf("rows marked deleted = %v", repo.deleted)
	}
}

func TestBackend_CleanupKeepsARowWhoseFileCouldNotBeRemoved(t *testing.T) {
	repo := orphanRepo()
	blob := &failingBlob{fakeBlob: newFakeBlob()}
	res, err := withBlob2(repo, blob).CleanupUploads(context.Background())
	if err != nil {
		t.Fatalf("CleanupUploads: %v", err)
	}
	if res.ImagesFailed != 2 || res.ImagesRemoved != 1 {
		t.Errorf("result = %+v, want the two files that could not be removed counted failed (the keyless row has no file)", res)
	}
	if repo.deleted[orphanA] != "" || repo.deleted[orphanB] != "" {
		t.Errorf("rows with a surviving file were marked deleted: %v", repo.deleted)
	}
	if len(repo.deferred) != 2 {
		t.Errorf("deferred = %v, want both sent to the back of the queue", repo.deferred)
	}
}

// failingBlob cannot remove anything.
type failingBlob struct{ *fakeBlob }

func (failingBlob) Delete(context.Context, string) error { return errors.New("storage down") }

func withBlob2(repo *fakeStoredRepo, blob FileStore) *attachmentUploadCleanupService {
	return WithFileStore(backendCleanup(repo), blob).(*attachmentUploadCleanupService)
}

func TestBackend_CleanupTreatsAFileAlreadyGoneAsRemoved(t *testing.T) {
	repo := orphanRepo()
	res, err := withBlob(repo, newFakeBlob()).CleanupUploads(context.Background()) // nothing in storage
	if err != nil {
		t.Fatalf("CleanupUploads: %v", err)
	}
	if res.ImagesFailed != 0 || repo.deleted[orphanA] == "" {
		t.Errorf("result = %+v, deleted = %v: a file that is already gone is done", res, repo.deleted)
	}
}

// An abandoned upload's chunks (under its staging key) and anything at its own key
// are removed with the row marked FAILED; chunks that cannot be removed keep the
// row PENDING for the next run.
func TestBackend_CleanupRemovesAnAbandonedUploadsFiles(t *testing.T) {
	newAbandoned := func() *fakeStoredRepo {
		repo := newFakeStoredRepo()
		pendingUpload(repo, 20*mib)
		repo.abandoned = []domain.AbandonedUpload{{Kind: domain.StoredAttachmentWorkItem, ID: saAttID}}
		return repo
	}
	t.Run("removed", func(t *testing.T) {
		repo, blob := newAbandoned(), newFakeBlob()
		res, err := withBlob(repo, blob).CleanupUploads(context.Background())
		if err != nil || res.Cleaned != 1 || res.Failed != 0 {
			t.Fatalf("result = %+v, %v", res, err)
		}
		if len(blob.discarded) != 2 || blob.discarded[0] != workItemKey+stagingSuffix || blob.discarded[1] != workItemKey {
			t.Errorf("discarded = %v, want the staging key then the file's own key", blob.discarded)
		}
		if repo.failed[saAttID] == "" {
			t.Error("the abandoned row was not marked FAILED")
		}
	})
	t.Run("a failure keeps the row for the next run", func(t *testing.T) {
		repo, blob := newAbandoned(), newFakeBlob()
		blob.discardErr = errors.New("storage down")
		res, err := withBlob(repo, blob).CleanupUploads(context.Background())
		if err != nil || res.Cleaned != 0 || res.Failed != 1 {
			t.Fatalf("result = %+v, %v", res, err)
		}
		if repo.failed[saAttID] != "" || len(repo.deferred) != 1 {
			t.Errorf("failed = %v, deferred = %v: the row must stay PENDING and go to the back of the queue", repo.failed, repo.deferred)
		}
	})
}
