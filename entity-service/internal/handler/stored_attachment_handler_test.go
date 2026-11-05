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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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

func (f *fakeStoredAttachmentSvc) DeleteAttachment(_ context.Context, req domain.DeleteStoredAttachmentRequest) error {
	f.deleted = req
	return f.err
}

func (f *fakeStoredAttachmentSvc) InitiateUpload(context.Context, domain.InitiateAttachmentUploadRequest) (domain.InitiateAttachmentUploadResponse, error) {
	return domain.InitiateAttachmentUploadResponse{AttachmentID: "a", Number: "WIA0000001", MaxBytes: 100}, f.err
}

func (f *fakeStoredAttachmentSvc) ConfirmUpload(_ context.Context, req domain.ConfirmAttachmentUploadRequest) (domain.ConfirmAttachmentUploadResponse, error) {
	f.confirm = req
	return domain.ConfirmAttachmentUploadResponse{ID: req.ID, Status: "COMPLETE"}, f.err
}

func serveStored(t *testing.T, svc *fakeStoredAttachmentSvc, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	h := NewStoredAttachmentHandler(svc)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /attachments/upload", h.InitiateUpload)
	mux.HandleFunc("POST /attachments/upload/{id}/confirm", h.ConfirmUpload)
	mux.HandleFunc("POST /attachments/delete", h.DeleteAttachment)
	mux.HandleFunc("POST /attachments/upload/{id}/authorize", h.AuthorizeUpload)
	mux.HandleFunc("POST /attachments/locate", h.LocateAttachment)
	mux.HandleFunc("GET /attachments/inline/{id}/location", h.InlineImageLocation)
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

func TestStoredAttachmentHandler_ConfirmTakesTheIdFromThePath(t *testing.T) {
	svc := &fakeStoredAttachmentSvc{}
	rec := serveStored(t, svc, http.MethodPost, "/attachments/upload/abc-123/confirm", `{"type":"deployment"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if svc.confirm.ID != "abc-123" || svc.confirm.Kind != domain.StoredAttachmentDeployment {
		t.Errorf("confirm request = %+v", svc.confirm)
	}
	// An id in the body is not a field: the path is the only source.
	rec = serveStored(t, svc, http.MethodPost, "/attachments/upload/abc-123/confirm", `{"type":"deployment","id":"other"}`)
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
		rec := serveStored(t, &fakeStoredAttachmentSvc{err: tc.err}, http.MethodPost, "/attachments/upload/00000000-0000-0000-0000-000000000001/confirm", `{"type":"work_item"}`)
		if rec.Code != tc.want {
			t.Errorf("%s: status = %d, want %d", name, rec.Code, tc.want)
		}
	}
}

func TestStoredAttachmentHandler_DeleteReturns204WithNoBody(t *testing.T) {
	svc := &fakeStoredAttachmentSvc{}
	rec := serveStored(t, svc, http.MethodPost, "/attachments/delete", `{"type":"work_item","id":"abc"}`)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("status = %d body = %q, want 204 and no body", rec.Code, rec.Body.String())
	}
	if svc.deleted.Kind != domain.StoredAttachmentWorkItem || svc.deleted.ID != "abc" {
		t.Errorf("request = %+v", svc.deleted)
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
		rec := serveStored(t, &fakeStoredAttachmentSvc{err: tc.err}, http.MethodPost, "/attachments/delete", `{"type":"deployment","id":"a"}`)
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

func TestStoredAttachmentHandler_AuthorizeTakesTheIdFromThePath(t *testing.T) {
	svc := &fakeStoredAttachmentSvc{}
	rec := serveStored(t, svc, http.MethodPost, "/attachments/upload/abc-123/authorize", `{"type":"work_item"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if svc.authorize.ID != "abc-123" || svc.authorize.Kind != domain.StoredAttachmentWorkItem {
		t.Errorf("authorize request = %+v", svc.authorize)
	}
	// The response carries an internal storage key, so it must never be cached.
	if rec.Header().Get("Cache-Control") != "no-store" || !strings.Contains(rec.Body.String(), `"storageKey"`) {
		t.Errorf("headers = %v body = %s", rec.Header(), rec.Body.String())
	}
	// An id in the body is not a field: the path is the only source.
	rec = serveStored(t, svc, http.MethodPost, "/attachments/upload/abc-123/authorize", `{"type":"work_item","id":"other"}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("an id in the body gave %d, want 400 (unknown field)", rec.Code)
	}
}

func TestStoredAttachmentHandler_LocateReturnsTheKeyUncached(t *testing.T) {
	svc := &fakeStoredAttachmentSvc{}
	rec := serveStored(t, svc, http.MethodPost, "/attachments/locate", `{"type":"deployment","id":"d-1"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if svc.locate.Kind != domain.StoredAttachmentDeployment || svc.locate.ID != "d-1" {
		t.Errorf("locate request = %+v", svc.locate)
	}
	if rec.Header().Get("Cache-Control") != "no-store" || !strings.Contains(rec.Body.String(), `"storageKey"`) {
		t.Errorf("headers = %v body = %s", rec.Header(), rec.Body.String())
	}
}

func TestStoredAttachmentHandler_InlineImageLocation(t *testing.T) {
	svc := &fakeStoredAttachmentSvc{}
	rec := serveStored(t, svc, http.MethodGet, "/attachments/inline/img-1/location", "")
	if rec.Code != http.StatusOK || svc.inlineID != "img-1" {
		t.Fatalf("status = %d, id = %q", rec.Code, svc.inlineID)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", rec.Header().Get("Cache-Control"))
	}

	// A removed image is a 410 here too, so the backend can tell the browser.
	gone := &fakeStoredAttachmentSvc{err: &apierror.GoneError{Msg: "this image was removed", Code: apierror.CodeAttachmentRemoved}}
	rec = serveStored(t, gone, http.MethodGet, "/attachments/inline/img-1/location", "")
	if rec.Code != http.StatusGone {
		t.Errorf("a removed image gave %d, want 410", rec.Code)
	}
}

func TestStoredAttachmentHandler_BackendCallsMapErrors(t *testing.T) {
	for name, tc := range map[string]struct {
		err  error
		want int
	}{
		"hidden or not theirs": {&apierror.NotFoundError{Msg: "attachment not found"}, http.StatusNotFound},
		"already confirmed":    {&apierror.ConflictError{Msg: "this upload was already confirmed"}, http.StatusConflict},
		"over the link budget": {&apierror.TooManyRequestsError{Msg: "slow down"}, http.StatusTooManyRequests},
	} {
		t.Run(name, func(t *testing.T) {
			svc := &fakeStoredAttachmentSvc{err: tc.err}
			if rec := serveStored(t, svc, http.MethodPost, "/attachments/upload/x/authorize", `{"type":"work_item"}`); rec.Code != tc.want {
				t.Errorf("authorize status = %d, want %d", rec.Code, tc.want)
			}
			if rec := serveStored(t, svc, http.MethodPost, "/attachments/locate", `{"type":"work_item","id":"x"}`); rec.Code != tc.want {
				t.Errorf("locate status = %d, want %d", rec.Code, tc.want)
			}
		})
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
