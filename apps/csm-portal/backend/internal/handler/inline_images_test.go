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

package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
)

// jsonString marshals s as a JSON string literal, for embedding into a
// hand-written test request body.
func jsonString(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func unmarshalJSON(t *testing.T, data []byte, v any) error {
	t.Helper()
	return json.Unmarshal(data, v)
}

// tinyPNGBase64 is a small, arbitrary payload used as a stand-in for real
// PNG bytes — InlineImageProcessor never validates image structure, only
// decodes/size-checks the base64 payload, so any bytes work for these tests.
func tinyPNGBase64(size int) string {
	return base64.StdEncoding.EncodeToString([]byte(strings.Repeat("a", size)))
}

func dataURIImg(mimeSubtype, b64 string) string {
	return fmt.Sprintf(`<img src="data:image/%s;base64,%s">`, mimeSubtype, b64)
}

// nextAttachmentIDFactory returns entity CreateCaseAttachment responses with
// incrementing ids, so a test can assert on exactly which attachment ids
// were created and later rolled back.
func nextAttachmentIDFactory() func(ctx context.Context, body []byte) ([]byte, error) {
	n := 0
	return func(ctx context.Context, body []byte) ([]byte, error) {
		n++
		id := fmt.Sprintf("aaaaaaaa-aaaa-aaaa-aaaa-%012d", n)
		return []byte(`{"message":"Attachment created successfully","attachment":{"id":"` + id + `","status":"complete"}}`), nil
	}
}

// ----- InlineImageProcessor.Process -----

func TestInlineImageProcessorNoImagesReturnsUnchanged(t *testing.T) {
	t.Parallel()
	p := NewInlineImageProcessor(&mockEntityCaseClient{}, &mockSftpgoClient{})

	html := "<p>just text, no images</p>"
	got, ierr := p.Process(context.Background(), "agent@example.com", "raw-jwt", "case-1", "", html)
	if ierr != nil {
		t.Fatalf("Process returned error: %v", ierr)
	}
	if got != html {
		t.Errorf("got = %q, want unchanged %q", got, html)
	}
}

// TestInlineImageProcessorSingleImage verifies the full happy path for one
// embedded image: the entity attachment row is created with status
// "pending", the bytes are written via SFTPGo, the row is only then
// confirmed "complete", and the <img> tag is rewritten to
// "/<attachmentId>.iix".
func TestInlineImageProcessorSingleImage(t *testing.T) {
	t.Parallel()
	const caseID = "11111111-1111-1111-1111-111111111111"
	var gotAttachmentBody []byte
	var confirmedIDs []string
	entity := &mockEntityCaseClient{
		createCaseAttachmentFn: func(ctx context.Context, body []byte) ([]byte, error) {
			gotAttachmentBody = body
			return []byte(`{"attachment":{"id":"22222222-2222-2222-2222-222222222222","status":"pending"}}`), nil
		},
		confirmCaseAttachmentFn: func(ctx context.Context, attachmentID string) ([]byte, error) {
			confirmedIDs = append(confirmedIDs, attachmentID)
			return []byte(`{"attachment":{"id":"` + attachmentID + `","status":"complete"}}`), nil
		},
	}
	sftpgoMock := &mockSftpgoClient{}
	p := NewInlineImageProcessor(entity, sftpgoMock)

	b64 := tinyPNGBase64(64)
	html := "<p>before</p>" + dataURIImg("png", b64) + "<p>after</p>"

	got, ierr := p.Process(context.Background(), "agent@example.com", "raw-jwt", caseID, "", html)
	if ierr != nil {
		t.Fatalf("Process returned error: %v", ierr)
	}

	want := `<p>before</p><img src="/22222222-2222-2222-2222-222222222222.iix"><p>after</p>`
	if got != want {
		t.Errorf("got = %q, want %q", got, want)
	}

	if len(sftpgoMock.mintTokenCalls) != 1 || sftpgoMock.mintTokenCalls[0] != "raw-jwt" {
		t.Errorf("mintTokenCalls = %v, want exactly one call with the raw jwt assertion", sftpgoMock.mintTokenCalls)
	}
	if len(sftpgoMock.createShareCalls) != 1 {
		t.Fatalf("createShareCalls = %v, want exactly 1", sftpgoMock.createShareCalls)
	}
	if len(sftpgoMock.uploadBytesCalls) != 1 {
		t.Fatalf("uploadBytesCalls = %v, want exactly 1", sftpgoMock.uploadBytesCalls)
	}
	// The share must be scoped to the upload's parent directory, matching
	// the storage key UploadBytes was actually given.
	if sftpgoMock.createShareCalls[0] == sftpgoMock.uploadBytesCalls[0] {
		t.Errorf("share storage key %q must be the PARENT directory of the upload path %q, not equal to it", sftpgoMock.createShareCalls[0], sftpgoMock.uploadBytesCalls[0])
	}

	var req struct {
		ReferenceID   string `json:"referenceId"`
		ReferenceType string `json:"referenceType"`
		Name          string `json:"name"`
		Type          string `json:"type"`
		Status        string `json:"status"`
		SizeBytes     int    `json:"sizeBytes"`
	}
	if err := unmarshalJSON(t, gotAttachmentBody, &req); err != nil {
		t.Fatalf("decode CreateCaseAttachment body: %v; raw: %s", err, gotAttachmentBody)
	}
	if req.ReferenceID != caseID || req.ReferenceType != "case" {
		t.Errorf("reference = %q/%q, want %q/case", req.ReferenceID, req.ReferenceType, caseID)
	}
	if req.Type != "image/png" {
		t.Errorf("Type = %q, want image/png", req.Type)
	}
	if req.Status != "pending" {
		t.Errorf("Status = %q, want pending — the row must not be durable-complete before its bytes exist", req.Status)
	}
	if len(confirmedIDs) != 1 || confirmedIDs[0] != "22222222-2222-2222-2222-222222222222" {
		t.Errorf("confirmedIDs = %v, want exactly the created attachment confirmed after upload", confirmedIDs)
	}
	if !strings.HasPrefix(req.Name, "inline-image-") || !strings.HasSuffix(req.Name, ".png") {
		t.Errorf("Name = %q, want an inline-image-<ts>-<n>.png synthetic filename", req.Name)
	}
	if req.SizeBytes != 64 {
		t.Errorf("SizeBytes = %d, want 64", req.SizeBytes)
	}
}

// TestInlineImageProcessorMultipleImages verifies two embedded images both
// get processed, each into its own attachment, and both <img> tags are
// rewritten independently in their original order.
func TestInlineImageProcessorMultipleImages(t *testing.T) {
	t.Parallel()
	entity := &mockEntityCaseClient{createCaseAttachmentFn: nextAttachmentIDFactory()}
	sftpgoMock := &mockSftpgoClient{}
	p := NewInlineImageProcessor(entity, sftpgoMock)

	html := "<p>one</p>" + dataURIImg("png", tinyPNGBase64(32)) +
		"<p>two</p>" + dataURIImg("jpeg", tinyPNGBase64(48))

	got, ierr := p.Process(context.Background(), "agent@example.com", "raw-jwt", "case-1", "", html)
	if ierr != nil {
		t.Fatalf("Process returned error: %v", ierr)
	}

	want := "<p>one</p>" + `<img src="/aaaaaaaa-aaaa-aaaa-aaaa-000000000001.iix">` +
		"<p>two</p>" + `<img src="/aaaaaaaa-aaaa-aaaa-aaaa-000000000002.iix">`
	if got != want {
		t.Errorf("got = %q, want %q", got, want)
	}
	if len(sftpgoMock.uploadBytesCalls) != 2 {
		t.Fatalf("uploadBytesCalls = %v, want 2", sftpgoMock.uploadBytesCalls)
	}
}

// TestInlineImageProcessorRejectsUnsupportedType verifies an unsupported
// inline image MIME subtype is rejected with 400 BEFORE any image (including
// any other, allowed image earlier in the same content) is uploaded —
// mirrors the backing system's reject-fast behavior.
func TestInlineImageProcessorRejectsUnsupportedType(t *testing.T) {
	t.Parallel()
	entity := &mockEntityCaseClient{}
	sftpgoMock := &mockSftpgoClient{}
	p := NewInlineImageProcessor(entity, sftpgoMock)

	html := dataURIImg("png", tinyPNGBase64(32)) + dataURIImg("svg+xml", tinyPNGBase64(32))

	_, ierr := p.Process(context.Background(), "agent@example.com", "raw-jwt", "case-1", "", html)
	if ierr == nil {
		t.Fatal("Process returned no error, want a rejection for the unsupported svg+xml type")
	}
	if ierr.status != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", ierr.status, http.StatusBadRequest)
	}
	if !strings.Contains(ierr.message, "svg+xml") {
		t.Errorf("message = %q, want it to name the unsupported type", ierr.message)
	}
	if len(sftpgoMock.uploadBytesCalls) != 0 {
		t.Errorf("uploadBytesCalls = %v, want 0 — the allowed png must never be uploaded when a later image in the same comment is unsupported", sftpgoMock.uploadBytesCalls)
	}
}

// TestInlineImageProcessorRejectsOversizedImage verifies an image over the
// 10MB limit is rejected BEFORE any entity/SFTPGo call for that image.
func TestInlineImageProcessorRejectsOversizedImage(t *testing.T) {
	t.Parallel()
	var createCalls int
	entity := &mockEntityCaseClient{
		createCaseAttachmentFn: func(ctx context.Context, body []byte) ([]byte, error) {
			createCalls++
			return []byte(`{"attachment":{"id":"22222222-2222-2222-2222-222222222222","status":"complete"}}`), nil
		},
	}
	sftpgoMock := &mockSftpgoClient{}
	p := NewInlineImageProcessor(entity, sftpgoMock)

	oversized := tinyPNGBase64(maxInlineImageSizeBytes + 1)
	html := dataURIImg("png", oversized)

	_, ierr := p.Process(context.Background(), "agent@example.com", "raw-jwt", "case-1", "", html)
	if ierr == nil {
		t.Fatal("Process returned no error, want a size rejection")
	}
	if ierr.status != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", ierr.status, http.StatusBadRequest)
	}
	if createCalls != 0 {
		t.Errorf("CreateCaseAttachment was called %d times; want 0 — size must be validated before any upstream write", createCalls)
	}
}

// TestInlineImageProcessorRollsBackOnSecondImageFailure verifies that when
// the second of two images fails (SFTPGo upload error), the FIRST image's
// already-created attachment is deleted (rollback) and the whole call fails —
// mirroring the backing system's _deleteAttachments rollback exactly.
func TestInlineImageProcessorRollsBackOnSecondImageFailure(t *testing.T) {
	t.Parallel()
	entity := &mockEntityCaseClient{createCaseAttachmentFn: nextAttachmentIDFactory()}
	var deletedIDs []string
	entity.deleteCaseAttachmentFn = func(ctx context.Context, attachmentID string) ([]byte, error) {
		deletedIDs = append(deletedIDs, attachmentID)
		return []byte(`{"message":"deleted"}`), nil
	}

	var uploadCalls int
	sftpgoMock := &mockSftpgoClient{
		uploadBytesFn: func(ctx context.Context, shareID, storageKey string, data []byte, contentType string) error {
			uploadCalls++
			if uploadCalls == 2 {
				return &apierror.Error{StatusCode: http.StatusInternalServerError, Body: "boom"}
			}
			return nil
		},
	}
	p := NewInlineImageProcessor(entity, sftpgoMock)

	html := dataURIImg("png", tinyPNGBase64(16)) + dataURIImg("png", tinyPNGBase64(16))

	_, ierr := p.Process(context.Background(), "agent@example.com", "raw-jwt", "case-1", "", html)
	if ierr == nil {
		t.Fatal("Process returned no error, want the second image's upload failure to reject the whole comment")
	}
	if ierr.status != http.StatusBadGateway {
		t.Errorf("status = %d, want %d", ierr.status, http.StatusBadGateway)
	}

	// Both attachments must be rolled back: the first image's row was fully
	// created and uploaded, and the second image's row was created (only its
	// bytes failed to upload) — but the WHOLE comment is being rejected, so
	// every attachment created anywhere in this call must be deleted, exactly
	// mirroring the backing system's _deleteAttachments(attachmentIds) call, which
	// deletes every id collected so far, not just the one that failed.
	wantDeleted := []string{
		"aaaaaaaa-aaaa-aaaa-aaaa-000000000001",
		"aaaaaaaa-aaaa-aaaa-aaaa-000000000002",
	}
	if len(deletedIDs) != len(wantDeleted) {
		t.Fatalf("deletedIDs = %v, want %v", deletedIDs, wantDeleted)
	}
	for i, want := range wantDeleted {
		if deletedIDs[i] != want {
			t.Errorf("deletedIDs[%d] = %q, want %q", i, deletedIDs[i], want)
		}
	}

	// The FIRST image's bytes are already in SFTPGo, and the second's upload
	// may have written a partial object — rollback must attempt to delete
	// both, not just the metadata rows.
	if len(sftpgoMock.removeFileCalls) != 2 {
		t.Fatalf("removeFileCalls = %v, want 2 — rollback must clean up uploaded bytes, not only rows", sftpgoMock.removeFileCalls)
	}
	if sftpgoMock.removeFileCalls[0] != sftpgoMock.uploadBytesCalls[0] || sftpgoMock.removeFileCalls[1] != sftpgoMock.uploadBytesCalls[1] {
		t.Errorf("removeFileCalls = %v, want the storage keys UploadBytes was given: %v", sftpgoMock.removeFileCalls, sftpgoMock.uploadBytesCalls)
	}
}

// TestInlineImageProcessorRollsBackOnConfirmFailure verifies the row is
// created pending and never left durable when its confirm step fails: the
// whole comment is rejected and both the row and the uploaded bytes are
// cleaned up.
func TestInlineImageProcessorRollsBackOnConfirmFailure(t *testing.T) {
	t.Parallel()
	var deletedIDs []string
	entity := &mockEntityCaseClient{
		createCaseAttachmentFn: nextAttachmentIDFactory(),
		confirmCaseAttachmentFn: func(ctx context.Context, attachmentID string) ([]byte, error) {
			return nil, &apierror.Error{StatusCode: http.StatusInternalServerError, Body: "boom"}
		},
		deleteCaseAttachmentFn: func(ctx context.Context, attachmentID string) ([]byte, error) {
			deletedIDs = append(deletedIDs, attachmentID)
			return []byte(`{"message":"deleted"}`), nil
		},
	}
	sftpgoMock := &mockSftpgoClient{}
	p := NewInlineImageProcessor(entity, sftpgoMock)

	html := dataURIImg("png", tinyPNGBase64(16))

	_, ierr := p.Process(context.Background(), "agent@example.com", "raw-jwt", "case-1", "", html)
	if ierr == nil {
		t.Fatal("Process returned no error, want the confirm failure to reject the whole comment")
	}
	if len(deletedIDs) != 1 {
		t.Errorf("deletedIDs = %v, want the unconfirmed row deleted", deletedIDs)
	}
	if len(sftpgoMock.removeFileCalls) != 1 {
		t.Errorf("removeFileCalls = %v, want the uploaded bytes deleted", sftpgoMock.removeFileCalls)
	}
}

// TestInlineImageProcessorRollbackSurvivesCancelledContext verifies rollback
// runs on a context detached from the request's cancellation: even when the
// parent context is already cancelled by the time the failure surfaces
// (caller gone, deadline hit), the row deletions and byte cleanups must still
// be attempted, on a NON-cancelled context.
func TestInlineImageProcessorRollbackSurvivesCancelledContext(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())

	var deleteCtxErrs []error
	var deletedIDs []string
	entity := &mockEntityCaseClient{
		createCaseAttachmentFn: nextAttachmentIDFactory(),
		deleteCaseAttachmentFn: func(ctx context.Context, attachmentID string) ([]byte, error) {
			deleteCtxErrs = append(deleteCtxErrs, ctx.Err())
			deletedIDs = append(deletedIDs, attachmentID)
			return []byte(`{"message":"deleted"}`), nil
		},
	}
	sftpgoMock := &mockSftpgoClient{
		uploadBytesFn: func(ctx context.Context, shareID, storageKey string, data []byte, contentType string) error {
			// Simulate the request being torn down mid-upload: cancel the
			// parent context, then fail the upload.
			cancel()
			return context.Canceled
		},
	}
	p := NewInlineImageProcessor(entity, sftpgoMock)

	html := dataURIImg("png", tinyPNGBase64(16))

	_, ierr := p.Process(ctx, "agent@example.com", "raw-jwt", "case-1", "", html)
	if ierr == nil {
		t.Fatal("Process returned no error, want the upload failure to reject the whole comment")
	}
	if len(deletedIDs) != 1 {
		t.Fatalf("deletedIDs = %v, want the created row deleted despite the cancelled parent context", deletedIDs)
	}
	for i, err := range deleteCtxErrs {
		if err != nil {
			t.Errorf("DeleteCaseAttachment call %d ran on a cancelled context (%v); rollback must use a detached context", i, err)
		}
	}
	if len(sftpgoMock.removeFileCalls) != 1 {
		t.Fatalf("removeFileCalls = %v, want the possibly-partial upload cleaned up", sftpgoMock.removeFileCalls)
	}
	for i, err := range sftpgoMock.removeFileCtxErrs {
		if err != nil {
			t.Errorf("RemoveFile call %d ran on a cancelled context (%v); rollback must use a detached context", i, err)
		}
	}
}

// TestInlineImageProcessorRollsBackOnEntityCreateFailure verifies rollback
// also fires when the SECOND image's entity-service create call itself
// fails (as opposed to the SFTPGo upload).
func TestInlineImageProcessorRollsBackOnEntityCreateFailure(t *testing.T) {
	t.Parallel()
	var createCalls int
	var deletedIDs []string
	entity := &mockEntityCaseClient{
		createCaseAttachmentFn: func(ctx context.Context, body []byte) ([]byte, error) {
			createCalls++
			if createCalls == 2 {
				return nil, &apierror.Error{StatusCode: http.StatusInternalServerError, Body: "boom"}
			}
			return []byte(`{"attachment":{"id":"aaaaaaaa-aaaa-aaaa-aaaa-000000000001","status":"complete"}}`), nil
		},
		deleteCaseAttachmentFn: func(ctx context.Context, attachmentID string) ([]byte, error) {
			deletedIDs = append(deletedIDs, attachmentID)
			return []byte(`{"message":"deleted"}`), nil
		},
	}
	sftpgoMock := &mockSftpgoClient{}
	p := NewInlineImageProcessor(entity, sftpgoMock)

	html := dataURIImg("png", tinyPNGBase64(16)) + dataURIImg("png", tinyPNGBase64(16))

	_, ierr := p.Process(context.Background(), "agent@example.com", "raw-jwt", "case-1", "", html)
	if ierr == nil {
		t.Fatal("Process returned no error, want the second image's entity create failure to reject the whole comment")
	}
	wantDeleted := "aaaaaaaa-aaaa-aaaa-aaaa-000000000001"
	if len(deletedIDs) != 1 || deletedIDs[0] != wantDeleted {
		t.Fatalf("deletedIDs = %v, want exactly [%q]", deletedIDs, wantDeleted)
	}
	if len(sftpgoMock.uploadBytesCalls) != 1 {
		t.Errorf("uploadBytesCalls = %v, want exactly 1 — the second image's upload must never be attempted once its own attachment row failed to create", sftpgoMock.uploadBytesCalls)
	}
}

// ----- CreateCaseComment wiring -----

// TestCreateCaseCommentInlineImagesDisabledByDefault verifies that with no
// InlineImageProcessor attached (every pre-existing CaseHandler, and every
// SN-backed deployment), a base64 data: URI in a comment's content passes
// through completely untouched — the exact pre-existing behavior.
func TestCreateCaseCommentInlineImagesDisabledByDefault(t *testing.T) {
	t.Parallel()
	const caseID = "11111111-1111-1111-1111-111111111111"
	var gotBody []byte
	entity := &mockEntityCaseClient{
		getCaseFn: func(ctx context.Context, id string) ([]byte, error) {
			return []byte(`{"state":"work_in_progress","workState":"ongoing","assignedEngineer":{"id":"` + testPlatformUserID + `"}}`), nil
		},
		createCaseCommentFn: func(ctx context.Context, id string, body []byte) ([]byte, error) {
			gotBody = body
			return []byte(`{}`), nil
		},
	}
	h := NewCaseHandler(entity).WithAccessGuard(viewerAccessGuard) // no WithInlineImageProcessor call

	b64 := tinyPNGBase64(16)
	content := dataURIImg("png", b64)
	reqBody := `{"type":"comment","content":` + jsonString(content) + `}`

	req := withCsEngineerUser(httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/comments", strings.NewReader(reqBody)))
	req.SetPathValue("id", caseID)
	w := httptest.NewRecorder()

	h.CreateCaseComment(w, req)

	assertStatus(t, w, http.StatusCreated)
	if string(gotBody) != reqBody {
		t.Errorf("forwarded body = %s, want unchanged %s — inline-image extraction must be a no-op when no processor is attached", gotBody, reqBody)
	}
}

// TestCreateCaseCommentExtractsInlineImageWhenEnabled verifies the full
// wiring: with an InlineImageProcessor attached, a base64 image in the
// request's "content" field is extracted before the body reaches
// entity.CreateCaseComment.
func TestCreateCaseCommentExtractsInlineImageWhenEnabled(t *testing.T) {
	t.Parallel()
	const caseID = "11111111-1111-1111-1111-111111111111"
	const attachmentID = "22222222-2222-2222-2222-222222222222"
	var gotBody []byte
	entity := &mockEntityCaseClient{
		getCaseFn: func(ctx context.Context, id string) ([]byte, error) {
			return []byte(`{"state":"work_in_progress","workState":"ongoing","assignedEngineer":{"id":"` + testPlatformUserID + `"}}`), nil
		},
		createCaseAttachmentFn: func(ctx context.Context, body []byte) ([]byte, error) {
			return []byte(`{"attachment":{"id":"` + attachmentID + `","status":"complete"}}`), nil
		},
		createCaseCommentFn: func(ctx context.Context, id string, body []byte) ([]byte, error) {
			gotBody = body
			return []byte(`{}`), nil
		},
	}
	sftpgoMock := &mockSftpgoClient{}
	h := NewCaseHandler(entity).WithInlineImageProcessor(NewInlineImageProcessor(entity, sftpgoMock)).WithAccessGuard(viewerAccessGuard)

	content := dataURIImg("png", tinyPNGBase64(16))
	reqBody := `{"type":"comment","content":` + jsonString(content) + `}`

	req := withCsEngineerUser(httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/comments", strings.NewReader(reqBody)))
	req.SetPathValue("id", caseID)
	req.Header.Set(jwtAssertionHeader, "raw-jwt")
	w := httptest.NewRecorder()

	h.CreateCaseComment(w, req)

	assertStatus(t, w, http.StatusCreated)
	if strings.Contains(string(gotBody), "base64,") {
		t.Errorf("forwarded body still carries a base64 payload: %s", gotBody)
	}
	// json.Marshal HTML-escapes '<', '>', and '"' by default, so check for
	// the unescaped id+suffix rather than a literal quoted src attribute.
	wantRef := attachmentID + `.iix`
	if !strings.Contains(string(gotBody), wantRef) {
		t.Errorf("forwarded body = %s, want it to contain %s", gotBody, wantRef)
	}
	if len(sftpgoMock.uploadBytesCalls) != 1 {
		t.Errorf("uploadBytesCalls = %v, want exactly 1", sftpgoMock.uploadBytesCalls)
	}
}

// TestCreateCaseCommentInlineImageFailureRejectsComment verifies that when
// inline-image processing fails, CreateCaseComment never calls
// entity.CreateCaseComment at all — the comment is not posted.
func TestCreateCaseCommentInlineImageFailureRejectsComment(t *testing.T) {
	t.Parallel()
	const caseID = "11111111-1111-1111-1111-111111111111"
	var createCommentCalls int
	entity := &mockEntityCaseClient{
		getCaseFn: func(ctx context.Context, id string) ([]byte, error) {
			return []byte(`{"state":"work_in_progress","workState":"ongoing","assignedEngineer":{"id":"` + testPlatformUserID + `"}}`), nil
		},
		createCaseCommentFn: func(ctx context.Context, id string, body []byte) ([]byte, error) {
			createCommentCalls++
			return []byte(`{}`), nil
		},
	}
	sftpgoMock := &mockSftpgoClient{}
	h := NewCaseHandler(entity).WithInlineImageProcessor(NewInlineImageProcessor(entity, sftpgoMock)).WithAccessGuard(viewerAccessGuard)

	// An unsupported subtype forces InlineImageProcessor to reject.
	content := dataURIImg("bmp", tinyPNGBase64(16))
	reqBody := `{"type":"comment","content":` + jsonString(content) + `}`

	req := withCsEngineerUser(httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/comments", strings.NewReader(reqBody)))
	req.SetPathValue("id", caseID)
	req.Header.Set(jwtAssertionHeader, "raw-jwt")
	w := httptest.NewRecorder()

	h.CreateCaseComment(w, req)

	assertStatus(t, w, http.StatusBadRequest)
	if createCommentCalls != 0 {
		t.Errorf("entity.CreateCaseComment was called %d times; want 0 — a rejected comment must never be posted", createCommentCalls)
	}
}
