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
	links   domain.AttachmentLinksRequest
	confirm domain.ConfirmAttachmentUploadRequest
	deleted domain.DeleteStoredAttachmentRequest
	err     error
}

func (f *fakeStoredAttachmentSvc) DeleteAttachment(_ context.Context, req domain.DeleteStoredAttachmentRequest) error {
	f.deleted = req
	return f.err
}

func (f *fakeStoredAttachmentSvc) ResolveLinks(_ context.Context, req domain.AttachmentLinksRequest) (domain.AttachmentLinksResponse, error) {
	f.links = req
	return domain.AttachmentLinksResponse{Links: []domain.AttachmentLink{{ID: "x", Available: true, URL: "https://f/x"}}}, f.err
}

func (f *fakeStoredAttachmentSvc) InitiateUpload(context.Context, domain.InitiateAttachmentUploadRequest) (domain.InitiateAttachmentUploadResponse, error) {
	return domain.InitiateAttachmentUploadResponse{AttachmentID: "a", UploadURL: "https://f/up", FieldName: "filenames"}, f.err
}

func (f *fakeStoredAttachmentSvc) ConfirmUpload(_ context.Context, req domain.ConfirmAttachmentUploadRequest) (domain.ConfirmAttachmentUploadResponse, error) {
	f.confirm = req
	return domain.ConfirmAttachmentUploadResponse{ID: req.ID, Status: "COMPLETE"}, f.err
}

func serveStored(t *testing.T, svc *fakeStoredAttachmentSvc, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	h := NewStoredAttachmentHandler(svc)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /attachments/download", h.ResolveLinks)
	mux.HandleFunc("POST /attachments/upload", h.InitiateUpload)
	mux.HandleFunc("POST /attachments/upload/{id}/confirm", h.ConfirmUpload)
	mux.HandleFunc("POST /attachments/delete", h.DeleteAttachment)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(method, target, strings.NewReader(body)))
	return rec
}

func TestStoredAttachmentHandler_LinksAreNeverCached(t *testing.T) {
	svc := &fakeStoredAttachmentSvc{}
	rec := serveStored(t, svc, http.MethodPost, "/attachments/download", `{"kind":"work_item","ids":["a"]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q: a response carrying short-lived links must not be cached", got)
	}
	if svc.links.Kind != domain.StoredAttachmentWorkItem || len(svc.links.IDs) != 1 {
		t.Errorf("request not decoded: %+v", svc.links)
	}
}

func TestStoredAttachmentHandler_InitiateReturns201(t *testing.T) {
	rec := serveStored(t, &fakeStoredAttachmentSvc{}, http.MethodPost, "/attachments/upload",
		`{"kind":"work_item","parentId":"p","name":"a.pdf","contentType":"application/pdf","sizeBytes":5}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	if rec.Header().Get("Cache-Control") != "no-store" || !strings.Contains(rec.Body.String(), `"uploadUrl":"https://f/up"`) {
		t.Errorf("headers = %v body = %s", rec.Header(), rec.Body.String())
	}
}

func TestStoredAttachmentHandler_ConfirmTakesTheIdFromThePath(t *testing.T) {
	svc := &fakeStoredAttachmentSvc{}
	rec := serveStored(t, svc, http.MethodPost, "/attachments/upload/abc-123/confirm", `{"kind":"deployment"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if svc.confirm.ID != "abc-123" || svc.confirm.Kind != domain.StoredAttachmentDeployment {
		t.Errorf("confirm request = %+v", svc.confirm)
	}
	// An id in the body is not a field: the path is the only source.
	rec = serveStored(t, svc, http.MethodPost, "/attachments/upload/abc-123/confirm", `{"kind":"deployment","id":"other"}`)
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
		rec := serveStored(t, &fakeStoredAttachmentSvc{err: tc.err}, http.MethodPost, "/attachments/upload/00000000-0000-0000-0000-000000000001/confirm", `{"kind":"work_item"}`)
		if rec.Code != tc.want {
			t.Errorf("%s: status = %d, want %d", name, rec.Code, tc.want)
		}
	}
}

func TestStoredAttachmentHandler_DeleteReturns204WithNoBody(t *testing.T) {
	svc := &fakeStoredAttachmentSvc{}
	rec := serveStored(t, svc, http.MethodPost, "/attachments/delete", `{"kind":"work_item","id":"abc"}`)
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
		rec := serveStored(t, &fakeStoredAttachmentSvc{err: tc.err}, http.MethodPost, "/attachments/delete", `{"kind":"deployment","id":"a"}`)
		if rec.Code != tc.want {
			t.Errorf("%s: status = %d, want %d", name, rec.Code, tc.want)
		}
	}
}
