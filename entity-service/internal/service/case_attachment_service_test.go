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
	"encoding/json"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

const testCaseID = "00000000-0000-0000-0000-0000000000c1"
const testAttachmentID = "00000000-0000-0000-0000-0000000000a1"
const testStorageKey = "cases/00000000-0000-0000-0000-0000000000c1/00000000-0000-0000-0000-0000000000a1"

func validCreateAttachmentRequest() domain.CreateAttachmentRequest {
	storageKey := testStorageKey
	return domain.CreateAttachmentRequest{
		ReferenceID:   testCaseID,
		ReferenceType: domain.ReferenceTypeCase,
		Name:          "diagnostics.log",
		Type:          "text/plain",
		StorageKey:    &storageKey,
		SizeBytes:     2048,
	}
}

func actorUserRepo(t *testing.T) stubUserRepo {
	t.Helper()
	return stubUserRepo{
		getUserByEmail: func(_ context.Context, email string) (domain.User, error) {
			if email != "jane.doe@example.com" {
				t.Fatalf("unexpected email looked up: %s", email)
			}
			return domain.User{ID: "user-jane", Email: "jane.doe@example.com", FirstName: "Jane", LastName: "Doe"}, nil
		},
	}
}

// TestCaseService_CreateCaseAttachment_Succeeds proves a well-formed request
// (storageKey + sizeBytes supplied, matching this data source's contract)
// reaches the repository and the response carries storageKey through.
func TestCaseService_CreateCaseAttachment_Succeeds(t *testing.T) {
	var capturedReq domain.CreateAttachmentRequest
	createdOn := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)

	repo := &stubCaseRepo{
		createCaseAttachment: func(_ context.Context, req domain.CreateAttachmentRequest) (domain.Attachment, error) {
			capturedReq = req
			return domain.Attachment{
				ID:            testAttachmentID,
				ReferenceID:   req.ReferenceID,
				ReferenceType: domain.ReferenceTypeCase,
				Name:          req.Name,
				Type:          req.Type,
				SizeBytes:     req.SizeBytes,
				CreatedBy:     domain.NewUserReference(req.CreatedBy, "", ""),
				CreatedOn:     createdOn,
				StorageKey:    req.StorageKey,
			}, nil
		},
	}

	svc := NewCaseService(repo, actorUserRepo(t), nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	resp, err := svc.CreateCaseAttachment(ctx, validCreateAttachmentRequest())
	if err != nil {
		t.Fatalf("CreateCaseAttachment returned error: %v", err)
	}
	if capturedReq.CreatedBy != "user-jane" {
		t.Fatalf("expected repo to receive resolved actor id, got %q", capturedReq.CreatedBy)
	}
	if resp.Attachment.ID != testAttachmentID {
		t.Fatalf("expected attachment id %q, got %q", testAttachmentID, resp.Attachment.ID)
	}
	if resp.Attachment.StorageKey == nil || *resp.Attachment.StorageKey != testStorageKey {
		t.Fatalf("expected storageKey %q on response, got %v", testStorageKey, resp.Attachment.StorageKey)
	}
	if resp.Attachment.CreatedBy != "jane.doe@example.com" {
		t.Fatalf("expected createdBy to be the actor's email, got %q", resp.Attachment.CreatedBy)
	}
}

// TestCaseService_CreateCaseAttachment_RequiresStorageKey proves this data
// source rejects a create request with no storageKey rather than falling
// back to a base64 payload -- there is no such fallback here, unlike
// ServiceNow.
func TestCaseService_CreateCaseAttachment_RequiresStorageKey(t *testing.T) {
	repo := &stubCaseRepo{
		createCaseAttachment: func(context.Context, domain.CreateAttachmentRequest) (domain.Attachment, error) {
			t.Fatal("repository should not be reached when storageKey is missing")
			return domain.Attachment{}, nil
		},
	}
	svc := NewCaseService(repo, actorUserRepo(t), nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	req := validCreateAttachmentRequest()
	req.StorageKey = nil

	_, err := svc.CreateCaseAttachment(ctx, req)
	var ve *apierror.ValidationError
	if !asValidationError(err, &ve) {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
}

// TestCaseService_CreateCaseAttachment_RequiresSizeBytes proves sizeBytes
// must be a positive value: this service cannot compute it (it never sees
// the file bytes for this data source).
func TestCaseService_CreateCaseAttachment_RequiresSizeBytes(t *testing.T) {
	svc := NewCaseService(&stubCaseRepo{}, actorUserRepo(t), nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	req := validCreateAttachmentRequest()
	req.SizeBytes = 0

	_, err := svc.CreateCaseAttachment(ctx, req)
	var ve *apierror.ValidationError
	if !asValidationError(err, &ve) {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
}

// TestCaseService_CreateCaseAttachment_RejectsNonCaseReferenceType proves
// this data source only models case attachments -- conversation, deployment,
// change_request, and incident have no Postgres schema backing here.
func TestCaseService_CreateCaseAttachment_RejectsNonCaseReferenceType(t *testing.T) {
	svc := NewCaseService(&stubCaseRepo{}, actorUserRepo(t), nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	req := validCreateAttachmentRequest()
	req.ReferenceType = domain.ReferenceTypeDeployment

	_, err := svc.CreateCaseAttachment(ctx, req)
	var ve *apierror.ValidationError
	if !asValidationError(err, &ve) {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
}

// TestCaseService_CreateCaseAttachment_RejectsUnauthenticatedCaller proves
// the same "must be a known, authenticated user" gate CreateCaseComment
// already enforces also protects attachment creation.
func TestCaseService_CreateCaseAttachment_RejectsUnauthenticatedCaller(t *testing.T) {
	svc := NewCaseService(&stubCaseRepo{}, stubUserRepo{}, nil)
	ctx := contextWithUserIDToken("") // no x-user-id-token header

	_, err := svc.CreateCaseAttachment(ctx, validCreateAttachmentRequest())
	var ue *apierror.UnauthorizedError
	if !errorsAsUnauthorized(err, &ue) {
		t.Fatalf("expected *apierror.UnauthorizedError, got %T: %v", err, err)
	}
}

// TestCaseService_CreateCaseAttachment_DefaultsToComplete proves every
// existing caller that doesn't specify a status (the ServiceNow path is
// unaffected since it never sets Status at all, but any pre-existing
// Postgres-path caller behaves the same way) still gets a 'complete' row --
// this field must be fully backward compatible.
func TestCaseService_CreateCaseAttachment_DefaultsToComplete(t *testing.T) {
	repo := &stubCaseRepo{
		createCaseAttachment: func(_ context.Context, req domain.CreateAttachmentRequest) (domain.Attachment, error) {
			if req.Status != domain.AttachmentStatusComplete {
				t.Fatalf("expected repo to receive status 'complete' by default, got %q", req.Status)
			}
			return domain.Attachment{
				ID:         testAttachmentID,
				Status:     req.Status,
				StorageKey: req.StorageKey,
				CreatedBy:  domain.NewUserReference(req.CreatedBy, "", ""),
			}, nil
		},
	}
	svc := NewCaseService(repo, actorUserRepo(t), nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	req := validCreateAttachmentRequest() // Status left unset
	resp, err := svc.CreateCaseAttachment(ctx, req)
	if err != nil {
		t.Fatalf("CreateCaseAttachment returned error: %v", err)
	}
	if resp.Attachment.Status != domain.AttachmentStatusComplete {
		t.Fatalf("expected response status 'complete', got %q", resp.Attachment.Status)
	}
}

// TestCaseService_CreateCaseAttachment_Pending proves an explicit
// status="pending" request reaches the repository unchanged and the response
// reports the pending status, so a caller can register a row before the
// browser has actually uploaded anything to SFTPGo.
func TestCaseService_CreateCaseAttachment_Pending(t *testing.T) {
	var capturedStatus domain.AttachmentStatus
	repo := &stubCaseRepo{
		createCaseAttachment: func(_ context.Context, req domain.CreateAttachmentRequest) (domain.Attachment, error) {
			capturedStatus = req.Status
			return domain.Attachment{
				ID:         testAttachmentID,
				Status:     req.Status,
				StorageKey: req.StorageKey,
				CreatedBy:  domain.NewUserReference(req.CreatedBy, "", ""),
			}, nil
		},
	}
	svc := NewCaseService(repo, actorUserRepo(t), nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	req := validCreateAttachmentRequest()
	req.Status = domain.AttachmentStatusPending
	resp, err := svc.CreateCaseAttachment(ctx, req)
	if err != nil {
		t.Fatalf("CreateCaseAttachment returned error: %v", err)
	}
	if capturedStatus != domain.AttachmentStatusPending {
		t.Fatalf("expected repo to receive status 'pending', got %q", capturedStatus)
	}
	if resp.Attachment.Status != domain.AttachmentStatusPending {
		t.Fatalf("expected response status 'pending', got %q", resp.Attachment.Status)
	}
}

// TestCaseService_CreateCaseAttachment_RejectsInvalidStatus proves an
// unrecognized status value is rejected rather than silently passed through
// to the database (where the CHECK constraint would catch it anyway, but the
// service should fail fast with a clear message).
func TestCaseService_CreateCaseAttachment_RejectsInvalidStatus(t *testing.T) {
	svc := NewCaseService(&stubCaseRepo{}, actorUserRepo(t), nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	req := validCreateAttachmentRequest()
	req.Status = "uploading"

	_, err := svc.CreateCaseAttachment(ctx, req)
	var ve *apierror.ValidationError
	if !asValidationError(err, &ve) {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
}

// TestCaseService_ConfirmCaseAttachment_TransitionsToComplete proves a
// pending row owned by the calling actor is transitioned to complete.
func TestCaseService_ConfirmCaseAttachment_TransitionsToComplete(t *testing.T) {
	key := testStorageKey
	var confirmedID string
	repo := &stubCaseRepo{
		getCaseAttachmentByID: func(_ context.Context, id string) (domain.Attachment, error) {
			return domain.Attachment{
				ID:         id,
				Status:     domain.AttachmentStatusPending,
				StorageKey: &key,
				CreatedBy:  domain.NewUserReference("user-jane", "jane.doe@example.com", "Jane Doe"),
			}, nil
		},
		confirmCaseAttachment: func(_ context.Context, id string) (domain.Attachment, error) {
			confirmedID = id
			return domain.Attachment{
				ID:         id,
				Status:     domain.AttachmentStatusComplete,
				StorageKey: &key,
				CreatedBy:  domain.NewUserReference("user-jane", "jane.doe@example.com", "Jane Doe"),
			}, nil
		},
	}
	svc := NewCaseService(repo, actorUserRepo(t), nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	resp, err := svc.ConfirmCaseAttachment(ctx, testAttachmentID)
	if err != nil {
		t.Fatalf("ConfirmCaseAttachment returned error: %v", err)
	}
	if confirmedID != testAttachmentID {
		t.Fatalf("expected repo to receive id %q, got %q", testAttachmentID, confirmedID)
	}
	if resp.Attachment.Status != domain.AttachmentStatusComplete {
		t.Fatalf("expected response status 'complete', got %q", resp.Attachment.Status)
	}
}

// TestCaseService_ConfirmCaseAttachment_RejectsAlreadyComplete proves
// confirming a row that is already 'complete' fails clearly (a ConflictError)
// instead of silently succeeding as a no-op.
func TestCaseService_ConfirmCaseAttachment_RejectsAlreadyComplete(t *testing.T) {
	key := testStorageKey
	repo := &stubCaseRepo{
		getCaseAttachmentByID: func(_ context.Context, id string) (domain.Attachment, error) {
			return domain.Attachment{
				ID:         id,
				Status:     domain.AttachmentStatusComplete,
				StorageKey: &key,
				CreatedBy:  domain.NewUserReference("user-jane", "jane.doe@example.com", "Jane Doe"),
			}, nil
		},
		confirmCaseAttachment: func(context.Context, string) (domain.Attachment, error) {
			t.Fatal("repository mutation should not be reached for an already-complete row")
			return domain.Attachment{}, nil
		},
	}
	svc := NewCaseService(repo, actorUserRepo(t), nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	_, err := svc.ConfirmCaseAttachment(ctx, testAttachmentID)
	var ce *apierror.ConflictError
	if !errorsAsConflict(err, &ce) {
		t.Fatalf("expected *apierror.ConflictError, got %T: %v", err, err)
	}
}

// TestCaseService_ConfirmCaseAttachment_RejectsDifferentActor proves a user
// other than the one who created the pending row cannot confirm it.
func TestCaseService_ConfirmCaseAttachment_RejectsDifferentActor(t *testing.T) {
	key := testStorageKey
	repo := &stubCaseRepo{
		getCaseAttachmentByID: func(_ context.Context, id string) (domain.Attachment, error) {
			return domain.Attachment{
				ID:         id,
				Status:     domain.AttachmentStatusPending,
				StorageKey: &key,
				CreatedBy:  domain.NewUserReference("someone-else", "someone.else@example.com", "Someone Else"),
			}, nil
		},
		confirmCaseAttachment: func(context.Context, string) (domain.Attachment, error) {
			t.Fatal("repository mutation should not be reached for a non-owning actor")
			return domain.Attachment{}, nil
		},
	}
	svc := NewCaseService(repo, actorUserRepo(t), nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	_, err := svc.ConfirmCaseAttachment(ctx, testAttachmentID)
	var fe *apierror.ForbiddenError
	if !errorsAsForbidden(err, &fe) {
		t.Fatalf("expected *apierror.ForbiddenError, got %T: %v", err, err)
	}
}

// TestCaseService_ConfirmCaseAttachment_NotFound proves confirming a
// nonexistent attachment surfaces as a NotFoundError.
func TestCaseService_ConfirmCaseAttachment_NotFound(t *testing.T) {
	repo := &stubCaseRepo{
		getCaseAttachmentByID: func(context.Context, string) (domain.Attachment, error) {
			return domain.Attachment{}, &apierror.NotFoundError{Msg: "attachment not found"}
		},
	}
	svc := NewCaseService(repo, actorUserRepo(t), nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	_, err := svc.ConfirmCaseAttachment(ctx, testAttachmentID)
	var nfe *apierror.NotFoundError
	if !errorsAsNotFound(err, &nfe) {
		t.Fatalf("expected *apierror.NotFoundError, got %T: %v", err, err)
	}
}

// TestCaseService_ConfirmCaseAttachment_RejectsUnauthenticatedCaller proves
// confirming is gated behind the same authentication check as every other
// attachment mutation.
func TestCaseService_ConfirmCaseAttachment_RejectsUnauthenticatedCaller(t *testing.T) {
	repo := &stubCaseRepo{
		getCaseAttachmentByID: func(context.Context, string) (domain.Attachment, error) {
			t.Fatal("repository should not be reached for an unauthenticated caller")
			return domain.Attachment{}, nil
		},
	}
	svc := NewCaseService(repo, stubUserRepo{}, nil)
	ctx := contextWithUserIDToken("")

	_, err := svc.ConfirmCaseAttachment(ctx, testAttachmentID)
	var ue *apierror.UnauthorizedError
	if !errorsAsUnauthorized(err, &ue) {
		t.Fatalf("expected *apierror.UnauthorizedError, got %T: %v", err, err)
	}
}

// TestCaseService_SearchCaseAttachments_ReturnsStorageKey proves the search
// path surfaces storageKey for every row, not just create.
func TestCaseService_SearchCaseAttachments_ReturnsStorageKey(t *testing.T) {
	key := testStorageKey
	repo := &stubCaseRepo{
		searchCaseAttachments: func(_ context.Context, caseID string, pagination domain.Pagination) ([]domain.Attachment, int, error) {
			if caseID != testCaseID {
				t.Fatalf("expected caseID %q, got %q", testCaseID, caseID)
			}
			return []domain.Attachment{{
				ID:            testAttachmentID,
				ReferenceID:   caseID,
				ReferenceType: domain.ReferenceTypeCase,
				Name:          "diagnostics.log",
				Type:          "text/plain",
				SizeBytes:     2048,
				CreatedBy:     domain.NewUserReference("user-jane", "jane.doe@example.com", "Jane Doe"),
				CreatedOn:     time.Now(),
				StorageKey:    &key,
			}}, 1, nil
		},
	}

	svc := NewCaseService(repo, stubUserRepo{}, nil)
	resp, err := svc.SearchCaseAttachments(context.Background(), domain.SearchAttachmentsRequest{
		ReferenceID:   testCaseID,
		ReferenceType: domain.ReferenceTypeCase,
	})
	if err != nil {
		t.Fatalf("SearchCaseAttachments returned error: %v", err)
	}
	if len(resp.Attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(resp.Attachments))
	}
	if resp.Attachments[0].StorageKey == nil || *resp.Attachments[0].StorageKey != testStorageKey {
		t.Fatalf("expected storageKey %q, got %v", testStorageKey, resp.Attachments[0].StorageKey)
	}
}

// TestCaseService_GetAttachmentByID_ReturnsStorageKeyNotContent proves the
// Postgres-backed GetAttachmentByID never fabricates base64 content: Content
// is always empty and StorageKey is populated, so a caller resolves bytes
// externally instead.
func TestCaseService_GetAttachmentByID_ReturnsStorageKeyNotContent(t *testing.T) {
	key := testStorageKey
	repo := &stubCaseRepo{
		getCaseAttachmentByID: func(_ context.Context, id string) (domain.Attachment, error) {
			if id != testAttachmentID {
				t.Fatalf("expected id %q, got %q", testAttachmentID, id)
			}
			return domain.Attachment{
				ID:            id,
				ReferenceID:   testCaseID,
				ReferenceType: domain.ReferenceTypeCase,
				Name:          "diagnostics.log",
				Type:          "text/plain",
				SizeBytes:     2048,
				CreatedBy:     domain.NewUserReference("user-jane", "jane.doe@example.com", "Jane Doe"),
				CreatedOn:     time.Now(),
				StorageKey:    &key,
			}, nil
		},
	}

	svc := NewCaseService(repo, stubUserRepo{}, nil)
	details, err := svc.GetAttachmentByID(context.Background(), testAttachmentID)
	if err != nil {
		t.Fatalf("GetAttachmentByID returned error: %v", err)
	}
	if details.Content != nil {
		t.Fatalf("expected nil Content for a Postgres-sourced attachment, got %q", *details.Content)
	}
	if details.StorageKey == nil || *details.StorageKey != testStorageKey {
		t.Fatalf("expected storageKey %q, got %v", testStorageKey, details.StorageKey)
	}
	if details.CreatedBy != "jane.doe@example.com" {
		t.Fatalf("expected createdBy email, got %q", details.CreatedBy)
	}
	if details.ReferenceID != testCaseID {
		t.Fatalf("expected referenceId %q, got %q", testCaseID, details.ReferenceID)
	}
	if details.ReferenceType == nil || *details.ReferenceType != domain.ReferenceTypeCase {
		t.Fatalf("expected referenceType %q, got %v", domain.ReferenceTypeCase, details.ReferenceType)
	}
}

// TestCaseService_GetAttachmentByID_NotFound proves a missing attachment
// surfaces as a NotFoundError, not a generic error.
func TestCaseService_GetAttachmentByID_NotFound(t *testing.T) {
	repo := &stubCaseRepo{
		getCaseAttachmentByID: func(context.Context, string) (domain.Attachment, error) {
			return domain.Attachment{}, &apierror.NotFoundError{Msg: "attachment not found"}
		},
	}
	svc := NewCaseService(repo, stubUserRepo{}, nil)

	_, err := svc.GetAttachmentByID(context.Background(), testAttachmentID)
	var nfe *apierror.NotFoundError
	if !errorsAsNotFound(err, &nfe) {
		t.Fatalf("expected *apierror.NotFoundError, got %T: %v", err, err)
	}
}

// TestCaseService_GetCaseAttachmentContent_ReturnsTypedError proves this data
// source never attempts to serve bytes for an attachment it doesn't hold --
// it returns an accurate, typed error instead of fabricating a response or
// reaching out to SFTPGo itself.
func TestCaseService_GetCaseAttachmentContent_ReturnsTypedError(t *testing.T) {
	svc := NewCaseService(&stubCaseRepo{}, stubUserRepo{}, nil)

	content, contentType, err := svc.GetCaseAttachmentContent(context.Background(), testAttachmentID)
	if content != nil {
		t.Fatalf("expected nil content, got %v", content)
	}
	if contentType != "" {
		t.Fatalf("expected empty contentType, got %q", contentType)
	}
	var sue *apierror.ServiceUnavailableError
	if !errorsAsServiceUnavailable(err, &sue) {
		t.Fatalf("expected *apierror.ServiceUnavailableError, got %T: %v", err, err)
	}
}

// TestCaseService_DeleteCaseAttachment_RemovesRow proves delete reaches the
// repository with the requested id and reports success.
func TestCaseService_DeleteCaseAttachment_RemovesRow(t *testing.T) {
	var deletedID string
	repo := &stubCaseRepo{
		deleteCaseAttachment: func(_ context.Context, id string) error {
			deletedID = id
			return nil
		},
	}
	svc := NewCaseService(repo, actorUserRepo(t), nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	resp, err := svc.DeleteCaseAttachment(ctx, domain.DeleteAttachmentRequest{AttachmentID: testAttachmentID})
	if err != nil {
		t.Fatalf("DeleteCaseAttachment returned error: %v", err)
	}
	if deletedID != testAttachmentID {
		t.Fatalf("expected repo to receive id %q, got %q", testAttachmentID, deletedID)
	}
	if resp.Message == "" {
		t.Fatal("expected a non-empty confirmation message")
	}
}

// TestCaseService_DeleteCaseAttachment_RejectsUnauthenticatedCaller proves
// deletion is gated behind the same authentication check as create.
func TestCaseService_DeleteCaseAttachment_RejectsUnauthenticatedCaller(t *testing.T) {
	repo := &stubCaseRepo{
		deleteCaseAttachment: func(context.Context, string) error {
			t.Fatal("repository should not be reached for an unauthenticated caller")
			return nil
		},
	}
	svc := NewCaseService(repo, stubUserRepo{}, nil)
	ctx := contextWithUserIDToken("")

	_, err := svc.DeleteCaseAttachment(ctx, domain.DeleteAttachmentRequest{AttachmentID: testAttachmentID})
	var ue *apierror.UnauthorizedError
	if !errorsAsUnauthorized(err, &ue) {
		t.Fatalf("expected *apierror.UnauthorizedError, got %T: %v", err, err)
	}
}

// TestCaseService_DeleteCaseAttachment_NotFound proves deleting a
// non-existent attachment surfaces as a NotFoundError.
func TestCaseService_DeleteCaseAttachment_NotFound(t *testing.T) {
	repo := &stubCaseRepo{
		deleteCaseAttachment: func(context.Context, string) error {
			return &apierror.NotFoundError{Msg: "attachment not found"}
		},
	}
	svc := NewCaseService(repo, actorUserRepo(t), nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	_, err := svc.DeleteCaseAttachment(ctx, domain.DeleteAttachmentRequest{AttachmentID: testAttachmentID})
	var nfe *apierror.NotFoundError
	if !errorsAsNotFound(err, &nfe) {
		t.Fatalf("expected *apierror.NotFoundError, got %T: %v", err, err)
	}
}

// TestCaseService_UpdateAttachment_RenamesFile proves the one mutation this
// data source supports (renaming, mirroring the ServiceNow "case" reference
// type behavior) reaches the repository and returns the actor's email.
func TestCaseService_UpdateAttachment_RenamesFile(t *testing.T) {
	updatedOn := time.Date(2026, 8, 27, 13, 0, 0, 0, time.UTC)
	var gotID, gotName, gotUpdatedBy string
	repo := &stubCaseRepo{
		updateAttachmentName: func(_ context.Context, id, name, updatedBy string) (time.Time, error) {
			gotID, gotName, gotUpdatedBy = id, name, updatedBy
			return updatedOn, nil
		},
	}
	svc := NewCaseService(repo, actorUserRepo(t), nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	name := "renamed.log"
	resp, err := svc.UpdateAttachment(ctx, domain.UpdateAttachmentRequest{
		AttachmentID:  testAttachmentID,
		ReferenceID:   testCaseID,
		ReferenceType: domain.ReferenceTypeCase,
		Name:          &name,
	})
	if err != nil {
		t.Fatalf("UpdateAttachment returned error: %v", err)
	}
	if gotID != testAttachmentID || gotName != "renamed.log" || gotUpdatedBy != "user-jane" {
		t.Fatalf("unexpected repo call: id=%q name=%q updatedBy=%q", gotID, gotName, gotUpdatedBy)
	}
	if resp.Attachment.UpdatedBy != "jane.doe@example.com" {
		t.Fatalf("expected updatedBy email, got %q", resp.Attachment.UpdatedBy)
	}
	if !resp.Attachment.UpdatedOn.Equal(updatedOn) {
		t.Fatalf("expected updatedOn %v, got %v", updatedOn, resp.Attachment.UpdatedOn)
	}
}

// TestCaseService_UpdateAttachment_RejectsDescriptionForCase mirrors the
// ServiceNow path's validateAttachmentUpdate rule: description is not a
// valid field to update for reference type "case".
func TestCaseService_UpdateAttachment_RejectsDescriptionForCase(t *testing.T) {
	svc := NewCaseService(&stubCaseRepo{}, actorUserRepo(t), nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	name := "renamed.log"
	description := json.RawMessage(`"not allowed"`)
	_, err := svc.UpdateAttachment(ctx, domain.UpdateAttachmentRequest{
		AttachmentID:  testAttachmentID,
		ReferenceID:   testCaseID,
		ReferenceType: domain.ReferenceTypeCase,
		Name:          &name,
		Description:   description,
	})
	var ve *apierror.ValidationError
	if !asValidationError(err, &ve) {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
}

// TestCaseService_UpdateAttachment_RejectsDeploymentReferenceType proves this
// data source rejects the "deployment" reference type ServiceNow allows for
// updates: deployment attachments have no Postgres schema backing here.
func TestCaseService_UpdateAttachment_RejectsDeploymentReferenceType(t *testing.T) {
	svc := NewCaseService(&stubCaseRepo{}, actorUserRepo(t), nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	name := "renamed.log"
	_, err := svc.UpdateAttachment(ctx, domain.UpdateAttachmentRequest{
		AttachmentID:  testAttachmentID,
		ReferenceID:   testCaseID,
		ReferenceType: domain.ReferenceTypeDeployment,
		Name:          &name,
	})
	var ve *apierror.ValidationError
	if !asValidationError(err, &ve) {
		t.Fatalf("expected *apierror.ValidationError, got %T: %v", err, err)
	}
}

func errorsAsUnauthorized(err error, target **apierror.UnauthorizedError) bool {
	if ue, ok := err.(*apierror.UnauthorizedError); ok {
		*target = ue
		return true
	}
	return false
}

func errorsAsNotFound(err error, target **apierror.NotFoundError) bool {
	if nfe, ok := err.(*apierror.NotFoundError); ok {
		*target = nfe
		return true
	}
	return false
}

func errorsAsServiceUnavailable(err error, target **apierror.ServiceUnavailableError) bool {
	if sue, ok := err.(*apierror.ServiceUnavailableError); ok {
		*target = sue
		return true
	}
	return false
}

func errorsAsConflict(err error, target **apierror.ConflictError) bool {
	if ce, ok := err.(*apierror.ConflictError); ok {
		*target = ce
		return true
	}
	return false
}

func errorsAsForbidden(err error, target **apierror.ForbiddenError) bool {
	if fe, ok := err.(*apierror.ForbiddenError); ok {
		*target = fe
		return true
	}
	return false
}
