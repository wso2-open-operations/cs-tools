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
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/middleware"
)

type fakeStoredEntity struct {
	gotBody  string
	gotID    string
	out      []byte
	err      error
	stream   io.ReadCloser
	streamCT string
	size     int64
}

func (f *fakeStoredEntity) ResolveStoredAttachmentLinks(_ context.Context, b json.RawMessage) ([]byte, error) {
	f.gotBody = string(b)
	return f.out, f.err
}
func (f *fakeStoredEntity) InitiateStoredAttachmentUpload(_ context.Context, b json.RawMessage) ([]byte, error) {
	f.gotBody = string(b)
	return f.out, f.err
}
func (f *fakeStoredEntity) ConfirmStoredAttachmentUpload(_ context.Context, id string, b json.RawMessage) ([]byte, error) {
	f.gotID, f.gotBody = id, string(b)
	return f.out, f.err
}
func (f *fakeStoredEntity) DeleteStoredAttachment(_ context.Context, b json.RawMessage) error {
	f.gotBody = string(b)
	return f.err
}
func (f *fakeStoredEntity) StreamInlineImage(_ context.Context, id string) (io.ReadCloser, string, int64, error) {
	f.gotID = id
	return f.stream, f.streamCT, f.size, f.err
}

type permitCall struct {
	module middleware.Module
	action middleware.Action
}

func serveStored(t *testing.T, f *fakeStoredEntity, allow bool, method, target, body string) (*httptest.ResponseRecorder, []permitCall) {
	t.Helper()
	var calls []permitCall
	h := NewStoredAttachmentHandler(f, func(w http.ResponseWriter, _ *http.Request, m middleware.Module, a middleware.Action) bool {
		calls = append(calls, permitCall{m, a})
		if !allow {
			writeError(w, http.StatusForbidden, ErrMsgForbidden)
		}
		return allow
	})
	mux := http.NewServeMux()
	mux.HandleFunc("POST /attachments/download", h.ResolveLinks)
	mux.HandleFunc("POST /attachments/upload", h.InitiateUpload)
	mux.HandleFunc("POST /attachments/upload/{id}/confirm", h.ConfirmUpload)
	mux.HandleFunc("POST /attachments/delete", h.DeleteAttachment)
	mux.HandleFunc("GET /attachments/inline/{id}/content", h.GetInlineImage)
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req = req.WithContext(middleware.WithUserInfo(req.Context(), &middleware.UserInfo{UserID: "u1"}))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec, calls
}

const storedID = "11111111-2222-3333-4444-555555555555"

func TestStoredAttachments_RelayBodiesAndStatuses(t *testing.T) {
	f := &fakeStoredEntity{out: []byte(`{"ok":true}`)}
	body := `{"type":"work_item","ids":["a"],"purpose":"preview"}`
	rec, calls := serveStored(t, f, true, http.MethodPost, "/attachments/download", body)
	if rec.Code != http.StatusOK || rec.Body.String() != `{"ok":true}` || f.gotBody != body {
		t.Fatalf("download: %d %q forwarded %q", rec.Code, rec.Body.String(), f.gotBody)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("links must not be cacheable")
	}
	if len(calls) != 1 || calls[0] != (permitCall{middleware.ModuleCases, middleware.ActionRead}) {
		t.Errorf("download permission = %v", calls)
	}

	rec, calls = serveStored(t, f, true, http.MethodPost, "/attachments/upload", `{"type":"deployment","parentId":"p","name":"a","sizeBytes":1}`)
	if rec.Code != http.StatusCreated || calls[0] != (permitCall{middleware.ModuleDeployments, middleware.ActionCreate}) {
		t.Errorf("upload: %d %v", rec.Code, calls)
	}

	rec, calls = serveStored(t, f, true, http.MethodPost, "/attachments/upload/"+storedID+"/confirm", `{"type":"comment"}`)
	if rec.Code != http.StatusOK || f.gotID != storedID || calls[0] != (permitCall{middleware.ModuleCases, middleware.ActionCreate}) {
		t.Errorf("confirm: %d id=%q %v", rec.Code, f.gotID, calls)
	}

	rec, calls = serveStored(t, f, true, http.MethodPost, "/attachments/delete", `{"type":"work_item","id":"x"}`)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 || calls[0].action != middleware.ActionCreate {
		t.Errorf("delete: %d %q %v", rec.Code, rec.Body.String(), calls)
	}
}

func TestStoredAttachments_RejectsBeforeCallingEntity(t *testing.T) {
	f := &fakeStoredEntity{}
	for name, tc := range map[string]struct {
		target, body string
		allow        bool
		want         int
	}{
		"unknown type":     {"/attachments/download", `{"type":"case","ids":["a"]}`, true, http.StatusBadRequest},
		"no type":          {"/attachments/download", `{"ids":["a"]}`, true, http.StatusBadRequest},
		"not json":         {"/attachments/delete", `nope`, true, http.StatusBadRequest},
		"no permission":    {"/attachments/upload", `{"type":"work_item"}`, false, http.StatusForbidden},
		"confirm bad uuid": {"/attachments/upload/not-a-uuid/confirm", `{"type":"work_item"}`, true, http.StatusBadRequest},
	} {
		rec, _ := serveStored(t, f, tc.allow, http.MethodPost, tc.target, tc.body)
		if rec.Code != tc.want || f.gotBody != "" {
			t.Errorf("%s: status %d (want %d), entity called with %q", name, rec.Code, tc.want, f.gotBody)
		}
	}
}

func TestStoredAttachments_UpstreamErrorsKeepTheirStatus(t *testing.T) {
	for status, want := range map[int]int{
		http.StatusBadRequest: 400, http.StatusForbidden: 403, http.StatusNotFound: 404,
		http.StatusConflict: 409, http.StatusTooManyRequests: 429, http.StatusServiceUnavailable: 503,
		http.StatusUnauthorized: 401,
	} {
		f := &fakeStoredEntity{err: apierror.NewUpstreamError(status, []byte(`{"code":"x","message":"upstream said so"}`))}
		rec, _ := serveStored(t, f, true, http.MethodPost, "/attachments/download", `{"type":"work_item","ids":["a"]}`)
		if rec.Code != want {
			t.Errorf("upstream %d -> %d, want %d", status, rec.Code, want)
		}
	}
}

type countingBody struct {
	r      io.Reader
	closed bool
}

func (c *countingBody) Read(p []byte) (int, error) { return c.r.Read(p) }
func (c *countingBody) Close() error               { c.closed = true; return nil }

func TestGetInlineImage_StreamsWithSafeHeaders(t *testing.T) {
	cb := &countingBody{r: strings.NewReader("PNGDATA")}
	f := &fakeStoredEntity{stream: cb, streamCT: "image/png", size: 7}
	rec, calls := serveStored(t, f, true, http.MethodGet, "/attachments/inline/"+storedID+"/content", "")
	if rec.Code != http.StatusOK || rec.Body.String() != "PNGDATA" || f.gotID != storedID || !cb.closed {
		t.Fatalf("status %d body %q id %q closed %v", rec.Code, rec.Body.String(), f.gotID, cb.closed)
	}
	for k, want := range map[string]string{
		"Content-Type": "image/png", "Content-Length": "7", "X-Content-Type-Options": "nosniff",
		"Content-Security-Policy": "default-src 'none'; sandbox", "Content-Disposition": "inline",
		"Cache-Control": "private, max-age=300",
	} {
		if got := rec.Header().Get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
	if len(calls) != 1 || calls[0] != (permitCall{middleware.ModuleCases, middleware.ActionRead}) {
		t.Errorf("permission = %v", calls)
	}
}

func TestGetInlineImage_NeutralisesUnknownTypesAndOmitsUnknownLength(t *testing.T) {
	f := &fakeStoredEntity{stream: io.NopCloser(strings.NewReader("<script>")), streamCT: "text/html; charset=utf-8", size: -1}
	rec, _ := serveStored(t, f, true, http.MethodGet, "/attachments/inline/"+storedID+"/content", "")
	if got := rec.Header().Get("Content-Type"); got != "application/octet-stream" {
		t.Errorf("Content-Type = %q, want application/octet-stream", got)
	}
	if rec.Header().Get("Content-Length") != "" {
		t.Errorf("Content-Length = %q, want none for an unknown length", rec.Header().Get("Content-Length"))
	}
	// A type with parameters is still recognised.
	f = &fakeStoredEntity{stream: io.NopCloser(strings.NewReader("x")), streamCT: "IMAGE/JPEG; q=1", size: 1}
	rec, _ = serveStored(t, f, true, http.MethodGet, "/attachments/inline/"+storedID+"/content", "")
	if got := rec.Header().Get("Content-Type"); got != "image/jpeg" {
		t.Errorf("Content-Type = %q, want image/jpeg", got)
	}
}

func TestGetInlineImage_ErrorsAndGuards(t *testing.T) {
	f := &fakeStoredEntity{err: apierror.NewUpstreamError(http.StatusNotFound, nil)}
	if rec, _ := serveStored(t, f, true, http.MethodGet, "/attachments/inline/"+storedID+"/content", ""); rec.Code != http.StatusNotFound {
		t.Errorf("not found -> %d", rec.Code)
	}
	if rec, _ := serveStored(t, &fakeStoredEntity{}, true, http.MethodGet, "/attachments/inline/nope/content", ""); rec.Code != http.StatusBadRequest {
		t.Errorf("bad id -> %d", rec.Code)
	}
	if rec, _ := serveStored(t, &fakeStoredEntity{}, false, http.MethodGet, "/attachments/inline/"+storedID+"/content", ""); rec.Code != http.StatusForbidden {
		t.Errorf("no permission -> %d", rec.Code)
	}
}
