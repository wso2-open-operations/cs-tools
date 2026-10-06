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
	"sync"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// stubCaseAttachmentSNMirror is a minimal caseAttachmentSNMirror: each test
// sets only the methods it exercises, and any other call panics, which
// doubles as an assertion (e.g. "ServiceNow must not be reached a second
// time" or "the mirror must not be reached at all").
type stubCaseAttachmentSNMirror struct {
	createCaseAttachment     func(ctx context.Context, req domain.CreateAttachmentRequest) (domain.CreateAttachmentResponse, error)
	getCaseAttachmentContent func(ctx context.Context, attachmentID string) ([]byte, string, error)
	deleteCaseAttachment     func(ctx context.Context, req domain.DeleteAttachmentRequest) (domain.DeleteAttachmentResponse, error)
	updateAttachment         func(ctx context.Context, req domain.UpdateAttachmentRequest) (domain.UpdateAttachmentResponse, error)
}

func (m *stubCaseAttachmentSNMirror) CreateCaseAttachment(ctx context.Context, req domain.CreateAttachmentRequest) (domain.CreateAttachmentResponse, error) {
	if m.createCaseAttachment != nil {
		return m.createCaseAttachment(ctx, req)
	}
	panic("stubCaseAttachmentSNMirror: CreateCaseAttachment not stubbed")
}
func (m *stubCaseAttachmentSNMirror) GetCaseAttachmentContent(ctx context.Context, attachmentID string) ([]byte, string, error) {
	if m.getCaseAttachmentContent != nil {
		return m.getCaseAttachmentContent(ctx, attachmentID)
	}
	panic("stubCaseAttachmentSNMirror: GetCaseAttachmentContent not stubbed")
}
func (m *stubCaseAttachmentSNMirror) DeleteCaseAttachment(ctx context.Context, req domain.DeleteAttachmentRequest) (domain.DeleteAttachmentResponse, error) {
	if m.deleteCaseAttachment != nil {
		return m.deleteCaseAttachment(ctx, req)
	}
	panic("stubCaseAttachmentSNMirror: DeleteCaseAttachment not stubbed")
}
func (m *stubCaseAttachmentSNMirror) UpdateAttachment(ctx context.Context, req domain.UpdateAttachmentRequest) (domain.UpdateAttachmentResponse, error) {
	if m.updateAttachment != nil {
		return m.updateAttachment(ctx, req)
	}
	panic("stubCaseAttachmentSNMirror: UpdateAttachment not stubbed")
}

// newDualWriteAttachmentService builds a caseAttachmentDualWriteService whose
// embedded *caseService is backed by repo/userRepo, and whose ServiceNow
// mirror is mirror. dispatcher may be nil (DeleteCaseAttachment/UpdateAttachment
// then skip the async mirror dispatch entirely, same as every other
// dual-write service's nil-dispatcher convention).
func newDualWriteAttachmentService(repo *stubCaseRepo, userRepo stubUserRepo, mirror *stubCaseAttachmentSNMirror, dispatcher *SNWritebackDispatcher) CaseService {
	base := NewCaseService(repo, userRepo, nil, alwaysUnrestrictedAccess{}, nil)
	return NewCaseAttachmentDualWriteService(base, mirror, dispatcher)
}

func validDualWriteCreateAttachmentRequest() domain.CreateAttachmentRequest {
	return domain.CreateAttachmentRequest{
		ReferenceID:   testCaseID,
		ReferenceType: domain.ReferenceTypeCase,
		Name:          "diagnostics.log",
		Type:          "text/plain",
		File:          "data:text/plain;base64,aGVsbG8=",
	}
}

// TestCaseAttachmentDualWriteService_CreateCaseAttachment_Succeeds proves the
// SN-first/synchronous create path: on ServiceNow success, the Postgres
// metadata insert uses EXACTLY the id/sizeBytes/createdOn ServiceNow
// returned (not anything generated locally), and uploadedBy is the resolved
// actor's Postgres user id, not any ServiceNow identity string.
func TestCaseAttachmentDualWriteService_CreateCaseAttachment_Succeeds(t *testing.T) {
	const snAttachmentID = "33333333-3333-3333-3333-333333333333"
	snCreatedOn := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	downloadURL := "https://example.invalid/download"

	mirror := &stubCaseAttachmentSNMirror{
		createCaseAttachment: func(_ context.Context, req domain.CreateAttachmentRequest) (domain.CreateAttachmentResponse, error) {
			return domain.CreateAttachmentResponse{
				Message: "Attachment created successfully.",
				Attachment: domain.AttachmentDetail{
					ID:          snAttachmentID,
					SizeBytes:   5,
					CreatedOn:   snCreatedOn,
					CreatedBy:   "jane.doe@example.com",
					DownloadURL: &downloadURL,
					Status:      domain.AttachmentStatusComplete,
				},
			}, nil
		},
	}

	var mu sync.Mutex
	var gotID, gotUploadedBy string
	var gotSizeBytes int
	var gotCreatedOn time.Time
	repo := &stubCaseRepo{
		createCaseAttachmentFromSN: func(_ context.Context, req domain.CreateAttachmentRequest, id string, sizeBytes int, uploadedBy string, createdOn time.Time) (domain.Attachment, error) {
			mu.Lock()
			gotID, gotSizeBytes, gotUploadedBy, gotCreatedOn = id, sizeBytes, uploadedBy, createdOn
			mu.Unlock()
			return domain.Attachment{
				ID:        id,
				SizeBytes: sizeBytes,
				CreatedOn: createdOn,
				Status:    domain.AttachmentStatusComplete,
				CreatedBy: domain.NewUserReference(uploadedBy, "", ""),
			}, nil
		},
	}

	svc := newDualWriteAttachmentService(repo, actorUserRepo(t), mirror, nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	resp, err := svc.CreateCaseAttachment(ctx, validDualWriteCreateAttachmentRequest())
	if err != nil {
		t.Fatalf("CreateCaseAttachment returned error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if gotID != snAttachmentID {
		t.Errorf("repo got id %q, want %q", gotID, snAttachmentID)
	}
	if gotSizeBytes != 5 {
		t.Errorf("repo got sizeBytes %d, want 5", gotSizeBytes)
	}
	if gotUploadedBy != "user-jane" {
		t.Errorf("repo got uploadedBy %q, want the resolved actor id %q (not a ServiceNow identity string)", gotUploadedBy, "user-jane")
	}
	if !gotCreatedOn.Equal(snCreatedOn) {
		t.Errorf("repo got createdOn %v, want %v", gotCreatedOn, snCreatedOn)
	}
	if resp.Attachment.ID != snAttachmentID {
		t.Errorf("response id = %q, want %q", resp.Attachment.ID, snAttachmentID)
	}
	if resp.Attachment.DownloadURL == nil || *resp.Attachment.DownloadURL != downloadURL {
		t.Errorf("response DownloadURL = %v, want %q (ServiceNow's own reply, not the Postgres row)", resp.Attachment.DownloadURL, downloadURL)
	}
}

// TestCaseAttachmentDualWriteService_CreateCaseAttachment_DeploymentSkipsPostgres
// covers the real bug this skip fixes: case_attachment.case_id has a hard FK
// into "case", so inserting a deployment-referenced attachment there always
// failed with a 23503 foreign-key violation -- reported back to the caller
// as an error even though ServiceNow had already accepted the upload (the
// live-reported symptom was "the upload doesn't show as submitted, but shows
// up after a page refresh"). Neither the repo insert nor actor resolution
// (which that insert alone needed, for uploaded_by) should be reached at
// all for a deployment reference -- stubCaseRepo panics on
// CreateCaseAttachmentFromServiceNow and stubUserRepo{} (no GetUserByEmail
// configured) panics on actor resolution, so either happening fails the test.
func TestCaseAttachmentDualWriteService_CreateCaseAttachment_DeploymentSkipsPostgres(t *testing.T) {
	const snAttachmentID = "33333333-3333-3333-3333-333333333333"
	snCreatedOn := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	mirror := &stubCaseAttachmentSNMirror{
		createCaseAttachment: func(_ context.Context, req domain.CreateAttachmentRequest) (domain.CreateAttachmentResponse, error) {
			if req.ReferenceType != domain.ReferenceTypeDeployment {
				t.Fatalf("mirror got referenceType %q, want deployment", req.ReferenceType)
			}
			return domain.CreateAttachmentResponse{
				Message: "Attachment created successfully.",
				Attachment: domain.AttachmentDetail{
					ID:        snAttachmentID,
					SizeBytes: 5,
					CreatedOn: snCreatedOn,
					CreatedBy: "jane.doe@example.com",
					Status:    domain.AttachmentStatusComplete,
				},
			}, nil
		},
	}

	svc := newDualWriteAttachmentService(&stubCaseRepo{}, stubUserRepo{}, mirror, nil)
	req := domain.CreateAttachmentRequest{
		ReferenceID:   testWorkItemID,
		ReferenceType: domain.ReferenceTypeDeployment,
		Name:          "plan.pdf",
		Type:          "application/pdf",
		File:          "data:application/pdf;base64,aGVsbG8=",
	}

	resp, err := svc.CreateCaseAttachment(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateCaseAttachment returned error: %v", err)
	}
	if resp.Attachment.ID != snAttachmentID {
		t.Errorf("response id = %q, want %q", resp.Attachment.ID, snAttachmentID)
	}
	if resp.Attachment.CreatedBy != "jane.doe@example.com" {
		t.Errorf("response CreatedBy = %q, want ServiceNow's own value (no Postgres actor was resolved)", resp.Attachment.CreatedBy)
	}
}

// TestCaseAttachmentDualWriteService_CreateCaseAttachment_SNFailureLeavesPostgresUntouched
// proves that when ServiceNow rejects the attachment, nothing is written to
// Postgres at all -- stubCaseRepo panics if CreateCaseAttachmentFromServiceNow
// is called, which is exactly the assertion.
func TestCaseAttachmentDualWriteService_CreateCaseAttachment_SNFailureLeavesPostgresUntouched(t *testing.T) {
	mirror := &stubCaseAttachmentSNMirror{
		createCaseAttachment: func(context.Context, domain.CreateAttachmentRequest) (domain.CreateAttachmentResponse, error) {
			return domain.CreateAttachmentResponse{}, errors.New("sn downstream unreachable")
		},
	}
	repo := &stubCaseRepo{}
	svc := newDualWriteAttachmentService(repo, actorUserRepo(t), mirror, nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	_, err := svc.CreateCaseAttachment(ctx, validDualWriteCreateAttachmentRequest())
	if err == nil {
		t.Fatal("expected an error when ServiceNow never accepts the attachment")
	}
}

// TestCaseAttachmentDualWriteService_CreateCaseAttachment_RejectsUnauthenticatedCaller
// proves the actor is resolved BEFORE ServiceNow is ever called: neither the
// mirror nor the repository should be reached for an unauthenticated caller
// (case_attachment.uploaded_by is a real FK to "user"(id), so there is no
// point attempting the ServiceNow upload before a Postgres user is known).
func TestCaseAttachmentDualWriteService_CreateCaseAttachment_RejectsUnauthenticatedCaller(t *testing.T) {
	mirror := &stubCaseAttachmentSNMirror{
		createCaseAttachment: func(context.Context, domain.CreateAttachmentRequest) (domain.CreateAttachmentResponse, error) {
			t.Fatal("ServiceNow mirror should not be reached for an unauthenticated caller")
			return domain.CreateAttachmentResponse{}, nil
		},
	}
	repo := &stubCaseRepo{}
	svc := newDualWriteAttachmentService(repo, stubUserRepo{}, mirror, nil)
	ctx := contextWithUserIDToken("") // no x-user-id-token header

	_, err := svc.CreateCaseAttachment(ctx, validDualWriteCreateAttachmentRequest())
	var ue *apierror.UnauthorizedError
	if !errorsAsUnauthorized(err, &ue) {
		t.Fatalf("expected *apierror.UnauthorizedError, got %T: %v", err, err)
	}
}

// TestCaseAttachmentDualWriteService_CreateCaseAttachment_PostgresInsertFailureIsReported
// proves that once ServiceNow has already accepted the attachment, a
// subsequent Postgres insert failure is surfaced as a real (logged) error --
// not swallowed, and not silently treated as success -- since ServiceNow now
// has an attachment Postgres doesn't know about.
func TestCaseAttachmentDualWriteService_CreateCaseAttachment_PostgresInsertFailureIsReported(t *testing.T) {
	mirror := &stubCaseAttachmentSNMirror{
		createCaseAttachment: func(context.Context, domain.CreateAttachmentRequest) (domain.CreateAttachmentResponse, error) {
			return domain.CreateAttachmentResponse{
				Message:    "Attachment created successfully.",
				Attachment: domain.AttachmentDetail{ID: "sn-id", SizeBytes: 5, CreatedOn: time.Now(), CreatedBy: "jane.doe@example.com"},
			}, nil
		},
	}
	repo := &stubCaseRepo{
		createCaseAttachmentFromSN: func(context.Context, domain.CreateAttachmentRequest, string, int, string, time.Time) (domain.Attachment, error) {
			return domain.Attachment{}, errors.New("insert failed: connection reset")
		},
	}
	svc := newDualWriteAttachmentService(repo, actorUserRepo(t), mirror, nil)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	_, err := svc.CreateCaseAttachment(ctx, validDualWriteCreateAttachmentRequest())
	if err == nil {
		t.Fatal("expected an error to be returned when the Postgres insert fails after ServiceNow already succeeded")
	}
}

// TestCaseAttachmentDualWriteService_ConfirmCaseAttachment_Returns503 proves
// confirming is rejected in this mode -- every dual-write-created attachment
// is synchronous and immediately 'complete', so there is no pending upload
// state to confirm, same as the plain ServiceNow data source.
func TestCaseAttachmentDualWriteService_ConfirmCaseAttachment_Returns503(t *testing.T) {
	svc := newDualWriteAttachmentService(&stubCaseRepo{}, stubUserRepo{}, &stubCaseAttachmentSNMirror{}, nil)

	_, err := svc.ConfirmCaseAttachment(context.Background(), testAttachmentID)
	var sue *apierror.ServiceUnavailableError
	if !errorsAsServiceUnavailable(err, &sue) {
		t.Fatalf("expected *apierror.ServiceUnavailableError, got %T: %v", err, err)
	}
}

// TestCaseAttachmentDualWriteService_GetCaseAttachmentContent_ForwardsToMirrorUnchanged
// proves the attachmentID is forwarded to the ServiceNow mirror as-is: the
// mirror (snCaseService.GetCaseAttachmentContent) already converts it to the
// real sys_id via uuidToSysid internally, so no extra conversion must happen
// here -- double-converting would corrupt the id.
func TestCaseAttachmentDualWriteService_GetCaseAttachmentContent_ForwardsToMirrorUnchanged(t *testing.T) {
	var gotID string
	mirror := &stubCaseAttachmentSNMirror{
		getCaseAttachmentContent: func(_ context.Context, attachmentID string) ([]byte, string, error) {
			gotID = attachmentID
			return []byte("hello"), "text/plain", nil
		},
	}
	svc := newDualWriteAttachmentService(&stubCaseRepo{}, stubUserRepo{}, mirror, nil)

	content, contentType, err := svc.GetCaseAttachmentContent(context.Background(), testAttachmentID)
	if err != nil {
		t.Fatalf("GetCaseAttachmentContent returned error: %v", err)
	}
	if gotID != testAttachmentID {
		t.Errorf("mirror got id %q, want unchanged %q", gotID, testAttachmentID)
	}
	if string(content) != "hello" || contentType != "text/plain" {
		t.Errorf("got (%q, %q), want (%q, %q)", content, contentType, "hello", "text/plain")
	}
}

// TestCaseAttachmentDualWriteService_DeleteCaseAttachment_PostgresFirstThenAsyncMirror
// proves delete is Postgres-first with a best-effort, asynchronous
// ServiceNow mirror -- the mirror call must still happen, just not block the
// response, same shape as every other dual-write UPDATE/DELETE.
func TestCaseAttachmentDualWriteService_DeleteCaseAttachment_PostgresFirstThenAsyncMirror(t *testing.T) {
	req := domain.DeleteAttachmentRequest{AttachmentID: testAttachmentID}

	called := make(chan struct{})
	var mu sync.Mutex
	var gotReq domain.DeleteAttachmentRequest
	mirror := &stubCaseAttachmentSNMirror{
		deleteCaseAttachment: func(_ context.Context, r domain.DeleteAttachmentRequest) (domain.DeleteAttachmentResponse, error) {
			mu.Lock()
			gotReq = r
			mu.Unlock()
			close(called)
			return domain.DeleteAttachmentResponse{Message: "deleted"}, nil
		},
	}
	var deletedID string
	repo := &stubCaseRepo{
		deleteCaseAttachment: func(_ context.Context, id string) error {
			deletedID = id
			return nil
		},
	}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	svc := newDualWriteAttachmentService(repo, actorUserRepo(t), mirror, dispatcher)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	if _, err := svc.DeleteCaseAttachment(ctx, req); err != nil {
		t.Fatalf("DeleteCaseAttachment returned error: %v", err)
	}
	if deletedID != testAttachmentID {
		t.Fatalf("expected Postgres delete to happen synchronously with id %q, got %q", testAttachmentID, deletedID)
	}

	select {
	case <-called:
	case <-time.After(2 * time.Second):
		t.Fatal("mirror.DeleteCaseAttachment was never called")
	}

	mu.Lock()
	defer mu.Unlock()
	if gotReq.AttachmentID != testAttachmentID {
		t.Errorf("mirror got AttachmentID %q, want %q", gotReq.AttachmentID, testAttachmentID)
	}
}

// TestCaseAttachmentDualWriteService_UpdateAttachment_PostgresFirstThenAsyncMirror
// mirrors the delete test above for the rename path.
func TestCaseAttachmentDualWriteService_UpdateAttachment_PostgresFirstThenAsyncMirror(t *testing.T) {
	name := "renamed.log"
	req := domain.UpdateAttachmentRequest{
		AttachmentID:  testAttachmentID,
		ReferenceID:   testCaseID,
		ReferenceType: domain.ReferenceTypeCase,
		Name:          &name,
	}

	called := make(chan struct{})
	var mu sync.Mutex
	var gotReq domain.UpdateAttachmentRequest
	mirror := &stubCaseAttachmentSNMirror{
		updateAttachment: func(_ context.Context, r domain.UpdateAttachmentRequest) (domain.UpdateAttachmentResponse, error) {
			mu.Lock()
			gotReq = r
			mu.Unlock()
			close(called)
			return domain.UpdateAttachmentResponse{}, nil
		},
	}
	repo := &stubCaseRepo{
		updateAttachmentName: func(_ context.Context, id, gotName, updatedBy string) (time.Time, error) {
			return time.Now(), nil
		},
	}
	dispatcher := NewSNWritebackDispatcher(&recordingSNWritebackFailures{})
	svc := newDualWriteAttachmentService(repo, actorUserRepo(t), mirror, dispatcher)
	ctx := contextWithUserIDToken(fakeJWTWithEmail(t, "jane.doe@example.com"))

	if _, err := svc.UpdateAttachment(ctx, req); err != nil {
		t.Fatalf("UpdateAttachment returned error: %v", err)
	}

	select {
	case <-called:
	case <-time.After(2 * time.Second):
		t.Fatal("mirror.UpdateAttachment was never called")
	}

	mu.Lock()
	defer mu.Unlock()
	if gotReq.AttachmentID != testAttachmentID || gotReq.Name == nil || *gotReq.Name != name {
		t.Errorf("mirror got %+v, want AttachmentID=%q Name=%q", gotReq, testAttachmentID, name)
	}
}

// TestCaseAttachmentDualWriteService_SearchCaseAttachments_InheritsEmbeddedPostgresPath
// proves SearchCaseAttachments is NOT overridden -- it reaches the embedded
// *caseService's Postgres-backed implementation directly, exactly the "reads
// come from Postgres" half of this mode's policy, with no ServiceNow call
// involved at all (the mirror here panics if touched).
func TestCaseAttachmentDualWriteService_SearchCaseAttachments_InheritsEmbeddedPostgresPath(t *testing.T) {
	repo := &stubCaseRepo{
		searchCaseAttachments: func(_ context.Context, caseID string, _ domain.Pagination) ([]domain.Attachment, int, error) {
			return []domain.Attachment{{ID: testAttachmentID, ReferenceID: caseID}}, 1, nil
		},
	}
	svc := newDualWriteAttachmentService(repo, stubUserRepo{}, &stubCaseAttachmentSNMirror{}, nil)

	resp, err := svc.SearchCaseAttachments(context.Background(), domain.SearchAttachmentsRequest{
		ReferenceID:   testCaseID,
		ReferenceType: domain.ReferenceTypeCase,
	})
	if err != nil {
		t.Fatalf("SearchCaseAttachments returned error: %v", err)
	}
	if len(resp.Attachments) != 1 || resp.Attachments[0].ID != testAttachmentID {
		t.Fatalf("expected the Postgres-backed result to pass through unchanged, got %+v", resp.Attachments)
	}
}
