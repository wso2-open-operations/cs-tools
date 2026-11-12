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

package handler

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/service"
	"github.com/wso2-open-operations/cs-tools/libs/azblobstore"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

type fakeStoredAttachmentSvc struct {
	confirm   domain.ConfirmAttachmentUploadRequest
	deleted   domain.DeleteStoredAttachmentRequest
	authorize domain.AuthorizeAttachmentUploadRequest
	locate    domain.LocateAttachmentRequest
	inlineID  string
	err       error
	// directUpload is what the upload start answers.
	directUpload bool
}

func (f *fakeStoredAttachmentSvc) AuthorizeUpload(_ context.Context, req domain.AuthorizeAttachmentUploadRequest) (domain.AttachmentUploadTarget, error) {
	f.authorize = req
	return domain.AttachmentUploadTarget{AttachmentID: req.ID, StorageKey: "csm/projects/P/workitems/W/attachments/1_a.pdf", MaxBytes: 100}, f.err
}

func (f *fakeStoredAttachmentSvc) LocateAttachment(_ context.Context, req domain.LocateAttachmentRequest) (domain.AttachmentLocation, error) {
	f.locate = req
	return domain.AttachmentLocation{ID: req.ID, Available: true, StorageKey: "csm/projects/P/workitems/W/attachments/1_a.pdf"}, f.err
}

func (f *fakeStoredAttachmentSvc) LocateInlineImage(_ context.Context, id string) (domain.AttachmentLocation, error) {
	f.inlineID = id
	return domain.AttachmentLocation{ID: id, Available: true, StorageKey: "k"}, f.err
}

func (f *fakeStoredAttachmentSvc) InlineImageRelocated(context.Context, string, domain.InlineImageRelocatedRequest) error {
	return f.err
}

func (f *fakeStoredAttachmentSvc) DeleteAttachment(_ context.Context, req domain.DeleteStoredAttachmentRequest) (domain.DeletedStoredAttachment, error) {
	f.deleted = req
	return domain.DeletedStoredAttachment{}, f.err
}

func (f *fakeStoredAttachmentSvc) InitiateUpload(context.Context, domain.InitiateAttachmentUploadRequest) (domain.InitiateAttachmentUploadResponse, error) {
	return domain.InitiateAttachmentUploadResponse{AttachmentID: "a", Number: "WIA0000001", MaxBytes: 100, DirectUpload: f.directUpload}, f.err
}

func (f *fakeStoredAttachmentSvc) ConfirmUpload(_ context.Context, req domain.ConfirmAttachmentUploadRequest) (domain.ConfirmAttachmentUploadResponse, error) {
	f.confirm = req
	return domain.ConfirmAttachmentUploadResponse{ID: req.ID, Status: "COMPLETE"}, f.err
}

func (f *fakeStoredAttachmentSvc) Limits() domain.AttachmentLimits {
	return domain.AttachmentLimits{MaxFileBytes: 1 << 30, MaxInlineImagesTotalBytes: 1 << 20, DirectTransferThresholdBytes: 16 << 20}
}

func serveStored(t *testing.T, svc *fakeStoredAttachmentSvc, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	return serveStoredWith(t, svc, &fakeFiles{}, method, target, body)
}

func serveStoredWith(t *testing.T, svc *fakeStoredAttachmentSvc, files *fakeFiles, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	h := NewStoredAttachmentHandler(svc).WithFiles(files)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /attachments/limits", h.Limits)
	mux.HandleFunc("POST /attachments/upload", h.InitiateUpload)
	mux.HandleFunc("POST /attachments/upload/{id}/confirm", h.ConfirmUpload)
	mux.HandleFunc("POST /attachments/delete", h.DeleteAttachment)
	mux.HandleFunc("PUT /attachments/upload/{id}/chunks/{index}", h.PutChunk)
	mux.HandleFunc("GET /attachments/upload/{id}/chunks", h.ListChunks)
	mux.HandleFunc("POST /attachments/{id}/upload-links", h.DirectUploadLinks)
	mux.HandleFunc("GET /attachments/download/{id}/content", h.DownloadAttachment)
	mux.HandleFunc("POST /attachments/{id}/download-links", h.DirectDownloadLink)
	mux.HandleFunc("GET /attachments/inline/{id}/content", h.InlineImageContent)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(method, target, strings.NewReader(body)))
	return rec
}

func TestStoredAttachmentHandler_InitiateReturns201(t *testing.T) {
	rec := serveStored(t, &fakeStoredAttachmentSvc{}, http.MethodPost, "/attachments/upload",
		`{"type":"work_item","parentId":"p","name":"a.pdf","contentType":"application/pdf","sizeBytes":5}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	if rec.Header().Get("Cache-Control") != "no-store" || !strings.Contains(rec.Body.String(), `"attachmentId":"a"`) {
		t.Errorf("headers = %v body = %s", rec.Header(), rec.Body.String())
	}
}

// TestStoredAttachmentHandler_InitiateSaysWhetherTheFileGoesDirect: the upload start passes on whether the service sends the file direct.
func TestStoredAttachmentHandler_InitiateSaysWhetherTheFileGoesDirect(t *testing.T) {
	for _, direct := range []bool{true, false} {
		rec := serveStoredWith(t, &fakeStoredAttachmentSvc{directUpload: direct}, &fakeFiles{}, http.MethodPost, "/attachments/upload",
			`{"type":"work_item","parentId":"p","name":"a.pdf","contentType":"application/pdf","sizeBytes":20971520}`)
		want := `"directUpload":false`
		if direct {
			want = `"directUpload":true`
		}
		if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), want) {
			t.Errorf("direct=%v: status %d body %s, want %s", direct, rec.Code, rec.Body.String(), want)
		}
	}
}

// TestStoredAttachmentHandler_DirectLinksAreNeverCached: the answers carry links, which are credentials, so they are no-store.
func TestStoredAttachmentHandler_DirectLinksAreNeverCached(t *testing.T) {
	rec := serveStoredWith(t, &fakeStoredAttachmentSvc{}, &fakeFiles{}, http.MethodPost, "/attachments/abc/upload-links?type=work_item", "")
	if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "no-store" || !strings.Contains(rec.Body.String(), `"links":["https://blob.example/abc?c=0"]`) {
		t.Errorf("upload links: status %d, Cache-Control %q, body %s", rec.Code, rec.Header().Get("Cache-Control"), rec.Body.String())
	}
	rec = serveStoredWith(t, &fakeStoredAttachmentSvc{}, &fakeFiles{}, http.MethodPost, "/attachments/abc/download-links?type=work_item", "")
	if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "no-store" || !strings.Contains(rec.Body.String(), `"url":"https://blob.example/abc?sig=x"`) {
		t.Errorf("download link: status %d, Cache-Control %q, body %s", rec.Code, rec.Header().Get("Cache-Control"), rec.Body.String())
	}
}

// TestStoredAttachmentHandler_DirectLinkRefusalsAreMapped: each refusal of the service keeps its HTTP status on both routes.
func TestStoredAttachmentHandler_DirectLinkRefusalsAreMapped(t *testing.T) {
	for name, tc := range map[string]struct {
		err  error
		want int
	}{
		"not found":    {&apierror.NotFoundError{Msg: "attachment not found"}, http.StatusNotFound},
		"not allowed":  {&apierror.ValidationError{Msg: "cannot be sent directly"}, http.StatusBadRequest},
		"gone":         {&apierror.GoneError{Msg: "removed"}, http.StatusGone},
		"storage down": {&apierror.ServiceUnavailableError{Msg: "down"}, http.StatusServiceUnavailable},
	} {
		rec := serveStoredWith(t, &fakeStoredAttachmentSvc{}, &fakeFiles{err: tc.err}, http.MethodPost, "/attachments/abc/upload-links?type=work_item", "")
		if rec.Code != tc.want {
			t.Errorf("%s: upload links gave %d, want %d", name, rec.Code, tc.want)
		}
		rec = serveStoredWith(t, &fakeStoredAttachmentSvc{}, &fakeFiles{err: tc.err}, http.MethodPost, "/attachments/abc/download-links?type=work_item", "")
		if rec.Code != tc.want {
			t.Errorf("%s: download link gave %d, want %d", name, rec.Code, tc.want)
		}
	}
}

func TestStoredAttachmentHandler_ConfirmTakesTheIdFromThePath(t *testing.T) {
	files := &fakeFiles{}
	rec := serveStoredWith(t, &fakeStoredAttachmentSvc{}, files, http.MethodPost, "/attachments/upload/abc-123/confirm", `{"type":"deployment","chunks":3}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if files.complete.ID != "abc-123" || files.complete.Kind != domain.StoredAttachmentDeployment || files.complete.Chunks != 3 {
		t.Errorf("complete request = %+v", files.complete)
	}
	// An id in the body is not a field: the path is the only source.
	rec = serveStoredWith(t, &fakeStoredAttachmentSvc{}, files, http.MethodPost, "/attachments/upload/abc-123/confirm", `{"type":"deployment","chunks":1,"id":"other"}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("an id in the body gave %d, want 400 (unknown field)", rec.Code)
	}
}

func TestStoredAttachmentHandler_MapsServiceErrorsToStatuses(t *testing.T) {
	for name, tc := range map[string]struct {
		err  error
		want int
	}{
		"not found":   {&apierror.NotFoundError{Msg: "x"}, http.StatusNotFound},
		"closed item": {&apierror.ConflictError{Msg: "closed"}, http.StatusConflict},
		"bad input":   {&apierror.ValidationError{Msg: "x"}, http.StatusBadRequest},
		"no token":    {&apierror.UnauthorizedError{Msg: "x"}, http.StatusUnauthorized},
		"storage":     {&apierror.ServiceUnavailableError{Msg: "x"}, http.StatusServiceUnavailable},
	} {
		rec := serveStoredWith(t, &fakeStoredAttachmentSvc{}, &fakeFiles{err: tc.err}, http.MethodPost, "/attachments/upload/00000000-0000-0000-0000-000000000001/confirm", `{"type":"work_item","chunks":1}`)
		if rec.Code != tc.want {
			t.Errorf("%s: status = %d, want %d", name, rec.Code, tc.want)
		}
	}
}

func TestStoredAttachmentHandler_DeleteReturns204WithNoBody(t *testing.T) {
	files := &fakeFiles{}
	rec := serveStoredWith(t, &fakeStoredAttachmentSvc{}, files, http.MethodPost, "/attachments/delete", `{"type":"work_item","id":"abc"}`)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("status = %d body = %q, want 204 and no body", rec.Code, rec.Body.String())
	}
	if files.deleted.Kind != domain.StoredAttachmentWorkItem || files.deleted.ID != "abc" {
		t.Errorf("request = %+v", files.deleted)
	}
}

func TestStoredAttachmentHandler_DeleteMapsErrors(t *testing.T) {
	for name, tc := range map[string]struct {
		err  error
		want int
	}{
		"not the uploader": {&apierror.ForbiddenError{Msg: "x"}, http.StatusForbidden},
		"closed item":      {&apierror.ConflictError{Msg: "closed"}, http.StatusConflict},
		"not found":        {&apierror.NotFoundError{Msg: "x"}, http.StatusNotFound},
		"storage":          {&apierror.ServiceUnavailableError{Msg: "x"}, http.StatusServiceUnavailable},
	} {
		rec := serveStoredWith(t, &fakeStoredAttachmentSvc{}, &fakeFiles{err: tc.err}, http.MethodPost, "/attachments/delete", `{"type":"deployment","id":"a"}`)
		if rec.Code != tc.want {
			t.Errorf("%s: status = %d, want %d", name, rec.Code, tc.want)
		}
	}
}

type fakeUploadCleanup struct {
	res domain.UploadCleanupResult
	err error
}

func (f fakeUploadCleanup) CleanupUploads(context.Context) (domain.UploadCleanupResult, error) {
	return f.res, f.err
}

func (f fakeUploadCleanup) AcknowledgeOrphans(context.Context, domain.AcknowledgeOrphansRequest) (domain.AcknowledgeOrphansResult, error) {
	return domain.AcknowledgeOrphansResult{}, f.err
}

func TestStoredAttachmentHandler_CleanupUploads(t *testing.T) {
	h := NewStoredAttachmentHandler(&fakeStoredAttachmentSvc{}).WithUploadCleanup(fakeUploadCleanup{res: domain.UploadCleanupResult{Found: 3, Cleaned: 2, Failed: 1}})
	rec := httptest.NewRecorder()
	h.CleanupUploads(rec, httptest.NewRequest(http.MethodPost, "/attachments/uploads/cleanup", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"cleaned":2`) {
		t.Errorf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	h = NewStoredAttachmentHandler(&fakeStoredAttachmentSvc{}).WithUploadCleanup(fakeUploadCleanup{err: &apierror.ForbiddenError{Msg: "x"}})
	rec = httptest.NewRecorder()
	h.CleanupUploads(rec, httptest.NewRequest(http.MethodPost, "/attachments/uploads/cleanup", nil))
	if rec.Code != http.StatusForbidden {
		t.Errorf("a restricted caller gave %d, want 403", rec.Code)
	}
}

type fakeUploadCleanupSvc struct {
	ack domain.AcknowledgeOrphansRequest
	err error
}

func (f *fakeUploadCleanupSvc) CleanupUploads(context.Context) (domain.UploadCleanupResult, error) {
	return domain.UploadCleanupResult{}, f.err
}

func (f *fakeUploadCleanupSvc) AcknowledgeOrphans(_ context.Context, req domain.AcknowledgeOrphansRequest) (domain.AcknowledgeOrphansResult, error) {
	f.ack = req
	return domain.AcknowledgeOrphansResult{Acknowledged: len(req.IDs)}, f.err
}

func TestStoredAttachmentHandler_AcknowledgeOrphans(t *testing.T) {
	cleanup := &fakeUploadCleanupSvc{}
	h := NewStoredAttachmentHandler(&fakeStoredAttachmentSvc{}).WithUploadCleanup(cleanup)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /attachments/uploads/cleanup/acknowledge", h.AcknowledgeOrphans)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/attachments/uploads/cleanup/acknowledge", strings.NewReader(`{"ids":["a","b"]}`)))
	if rec.Code != http.StatusOK || len(cleanup.ack.IDs) != 2 {
		t.Fatalf("status = %d, request = %+v", rec.Code, cleanup.ack)
	}
	if rec.Header().Get("Cache-Control") != "no-store" || !strings.Contains(rec.Body.String(), `"acknowledged":2`) {
		t.Errorf("headers = %v body = %s", rec.Header(), rec.Body.String())
	}

	// An unknown field is refused, as on every other route.
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/attachments/uploads/cleanup/acknowledge", strings.NewReader(`{"ids":["a"],"delete":true}`)))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("an unknown field gave %d, want 400", rec.Code)
	}

	// Without the cleanup service the route answers 404, like the other cleanup routes.
	bare := NewStoredAttachmentHandler(&fakeStoredAttachmentSvc{})
	mux = http.NewServeMux()
	mux.HandleFunc("POST /attachments/uploads/cleanup/acknowledge", bare.AcknowledgeOrphans)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/attachments/uploads/cleanup/acknowledge", strings.NewReader(`{"ids":["a"]}`)))
	if rec.Code != http.StatusNotFound {
		t.Errorf("without the service gave %d, want 404", rec.Code)
	}
}

// fakeFiles is a StoredAttachmentFileService that records what it was asked and
// serves a fixed file.
type fakeFiles struct {
	complete domain.CompleteAttachmentUploadRequest
	deleted  domain.DeleteStoredAttachmentRequest
	put      struct {
		kind  domain.StoredAttachmentKind
		id    string
		index int
		size  int64
		data  string
	}
	rangeHeader string
	workItemID  string
	file        string
	partial     bool
	err         error
}

// DirectUploadLinks answers with one link, or the configured error.
func (f *fakeFiles) DirectUploadLinks(_ context.Context, _ domain.StoredAttachmentKind, id string) (domain.DirectUploadLinks, error) {
	return domain.DirectUploadLinks{Links: []string{"https://blob.example/" + id + "?c=0"}, ChunkBytes: service.UploadChunkBytes}, f.err
}

// DirectDownloadLink answers with a link, or the configured error.
func (f *fakeFiles) DirectDownloadLink(_ context.Context, _ domain.StoredAttachmentKind, id string) (domain.DirectDownloadLink, error) {
	return domain.DirectDownloadLink{URL: "https://blob.example/" + id + "?sig=x", Name: "a.pdf"}, f.err
}

func (f *fakeFiles) PutChunk(_ context.Context, kind domain.StoredAttachmentKind, id string, index int, size int64, body io.Reader) error {
	b, _ := io.ReadAll(body)
	f.put.kind, f.put.id, f.put.index, f.put.size, f.put.data = kind, id, index, size, string(b)
	return f.err
}

func (f *fakeFiles) ListChunks(context.Context, domain.StoredAttachmentKind, string) (domain.UploadChunkList, error) {
	return domain.UploadChunkList{ChunkBytes: service.UploadChunkBytes, Chunks: []domain.UploadChunk{{Index: 0, Size: 5}}}, f.err
}

func (f *fakeFiles) CompleteUpload(_ context.Context, req domain.CompleteAttachmentUploadRequest) (domain.ConfirmAttachmentUploadResponse, error) {
	f.complete = req
	return domain.ConfirmAttachmentUploadResponse{ID: req.ID, Status: "COMPLETE"}, f.err
}

func (f *fakeFiles) open(name, contentType string) *service.FileDownload {
	obj := &azblobstore.Object{Body: io.NopCloser(strings.NewReader(f.file)), ContentLength: int64(len(f.file))}
	if f.partial {
		obj.Partial, obj.ContentRange = true, "bytes 0-4/10"
	}
	return &service.FileDownload{Name: name, ContentType: contentType, Object: obj}
}

func (f *fakeFiles) OpenAttachment(_ context.Context, _ domain.StoredAttachmentKind, _, rangeHeader string) (*service.FileDownload, error) {
	f.rangeHeader = rangeHeader
	if f.err != nil {
		return nil, f.err
	}
	return f.open("report \"x\".pdf", "application/pdf"), nil
}

func (f *fakeFiles) OpenInlineImage(_ context.Context, _ string, workItemID string) (*service.FileDownload, error) {
	f.workItemID = workItemID
	if f.err != nil {
		return nil, f.err
	}
	return f.open("a.png", "image/png"), nil
}

func (f *fakeFiles) DeleteAttachment(_ context.Context, req domain.DeleteStoredAttachmentRequest) error {
	f.deleted = req
	return f.err
}

func TestStoredAttachmentHandler_PutChunkPassesTheBytesAndTheIndex(t *testing.T) {
	files := &fakeFiles{}
	rec := serveStoredWith(t, &fakeStoredAttachmentSvc{}, files, http.MethodPut, "/attachments/upload/abc/chunks/2?type=work_item", "hello")
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("status = %d body = %q, want 204 and no body", rec.Code, rec.Body.String())
	}
	if files.put.id != "abc" || files.put.index != 2 || files.put.kind != domain.StoredAttachmentWorkItem || files.put.size != 5 || files.put.data != "hello" {
		t.Errorf("chunk = %+v", files.put)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control = %q", rec.Header().Get("Cache-Control"))
	}
	for _, idx := range []string{"x", "-1"} {
		if rec := serveStoredWith(t, &fakeStoredAttachmentSvc{}, &fakeFiles{}, http.MethodPut, "/attachments/upload/abc/chunks/"+idx+"?type=work_item", "x"); rec.Code != http.StatusBadRequest {
			t.Errorf("chunk index %q gave %d, want 400", idx, rec.Code)
		}
	}
	rec = serveStoredWith(t, &fakeStoredAttachmentSvc{}, &fakeFiles{err: &apierror.TooManyRequestsError{Msg: "busy"}}, http.MethodPut, "/attachments/upload/abc/chunks/0?type=work_item", "x")
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("a refused transfer gave %d, want 429", rec.Code)
	}
}

func TestStoredAttachmentHandler_ListChunks(t *testing.T) {
	rec := serveStored(t, &fakeStoredAttachmentSvc{}, http.MethodGet, "/attachments/upload/abc/chunks?type=work_item", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"chunkBytes":8388608`) || !strings.Contains(rec.Body.String(), `"index":0`) {
		t.Errorf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control = %q", rec.Header().Get("Cache-Control"))
	}
}

func TestStoredAttachmentHandler_DownloadIsAnOpaqueAttachment(t *testing.T) {
	files := &fakeFiles{file: "0123456789"}
	rec := serveStoredWith(t, &fakeStoredAttachmentSvc{}, files, http.MethodGet, "/attachments/download/abc/content?type=work_item", "")
	if rec.Code != http.StatusOK || rec.Body.String() != "0123456789" {
		t.Fatalf("status = %d body = %q", rec.Code, rec.Body.String())
	}
	h := rec.Header()
	// Whatever the stored type says, a download is never rendered by the browser.
	if h.Get("Content-Type") != "application/octet-stream" || h.Get("X-Content-Type-Options") != "nosniff" ||
		!strings.HasPrefix(h.Get("Content-Disposition"), "attachment;") || h.Get("Content-Length") != "10" ||
		h.Get("Accept-Ranges") != "bytes" || h.Get("Cache-Control") != "private, no-store" {
		t.Errorf("headers = %v", h)
	}
	// A quote in the name cannot break out of the header.
	if strings.Contains(h.Get("Content-Disposition"), `"x"`) {
		t.Errorf("Content-Disposition = %q, want the quotes neutralised", h.Get("Content-Disposition"))
	}
}

func TestStoredAttachmentHandler_DownloadHonoursARange(t *testing.T) {
	files := &fakeFiles{file: "01234", partial: true}
	req := httptest.NewRequest(http.MethodGet, "/attachments/download/abc/content?type=work_item", nil)
	req.Header.Set("Range", "bytes=0-4")
	h := NewStoredAttachmentHandler(&fakeStoredAttachmentSvc{}).WithFiles(files)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /attachments/download/{id}/content", h.DownloadAttachment)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusPartialContent || rec.Header().Get("Content-Range") != "bytes 0-4/10" || files.rangeHeader != "bytes=0-4" {
		t.Errorf("status = %d, Content-Range = %q, range passed = %q", rec.Code, rec.Header().Get("Content-Range"), files.rangeHeader)
	}

	// A range the file does not have is a 416 that says how big the file is.
	rec = serveStoredWith(t, &fakeStoredAttachmentSvc{}, &fakeFiles{err: &service.RangeNotSatisfiableError{Size: 10}}, http.MethodGet, "/attachments/download/abc/content?type=work_item", "")
	if rec.Code != http.StatusRequestedRangeNotSatisfiable || rec.Header().Get("Content-Range") != "bytes */10" {
		t.Errorf("status = %d, Content-Range = %q, want 416 and bytes */10", rec.Code, rec.Header().Get("Content-Range"))
	}
}

func TestStoredAttachmentHandler_InlineImageIsShownOnlyAsAnImage(t *testing.T) {
	files := &fakeFiles{file: "png"}
	rec := serveStoredWith(t, &fakeStoredAttachmentSvc{}, files, http.MethodGet, "/attachments/inline/img-1/content?workItemId=w-1", "")
	if rec.Code != http.StatusOK || files.workItemID != "w-1" {
		t.Fatalf("status = %d, workItemId = %q", rec.Code, files.workItemID)
	}
	h := rec.Header()
	if h.Get("Content-Type") != "image/png" || h.Get("Content-Disposition") != "inline" || h.Get("X-Content-Type-Options") != "nosniff" ||
		h.Get("Content-Security-Policy") != "default-src 'none'; sandbox" {
		t.Errorf("headers = %v", h)
	}

	// A removed image is a 410 with its own code, so the portal can tell the browser.
	gone := &fakeFiles{err: &apierror.GoneError{Msg: "this image was removed", Code: apierror.CodeAttachmentRemoved}}
	rec = serveStoredWith(t, &fakeStoredAttachmentSvc{}, gone, http.MethodGet, "/attachments/inline/img-1/content", "")
	if rec.Code != http.StatusGone || !strings.Contains(rec.Body.String(), apierror.CodeAttachmentRemoved) {
		t.Errorf("a removed image gave %d %s, want 410 with its code", rec.Code, rec.Body.String())
	}
}

func TestStoredAttachmentHandler_FileRoutesAre404WithoutStorageService(t *testing.T) {
	h := NewStoredAttachmentHandler(&fakeStoredAttachmentSvc{})
	for name, fn := range map[string]http.HandlerFunc{
		"put": h.PutChunk, "list": h.ListChunks, "download": h.DownloadAttachment, "inline": h.InlineImageContent, "confirm": h.ConfirmUpload,
	} {
		rec := httptest.NewRecorder()
		fn(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s without the file service gave %d, want 404", name, rec.Code)
		}
	}
}

// TestStoredAttachmentHandler_Limits: the sizes in force are returned as JSON and may be cached briefly.
func TestStoredAttachmentHandler_Limits(t *testing.T) {
	rec := serveStored(t, &fakeStoredAttachmentSvc{}, http.MethodGet, "/attachments/limits", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"maxFileBytes":1073741824`) || !strings.Contains(rec.Body.String(), `"maxInlineImagesTotalBytes":1048576`) || !strings.Contains(rec.Body.String(), `"directTransferThresholdBytes":16777216`) {
		t.Errorf("body = %s", rec.Body.String())
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "private, max-age=300" {
		t.Errorf("Cache-Control = %q", cc)
	}
}
