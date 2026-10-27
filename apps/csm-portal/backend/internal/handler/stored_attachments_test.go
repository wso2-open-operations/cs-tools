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
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/entity"
)

const storedAttID = "11111111-1111-1111-1111-111111111111"

type fakeStoredEntity struct {
	links, upload, confirm, deleted []byte
	confirmID                       string
	err                             error
	image                           *entity.InlineImageStream
	imageID                         string
}

func (f *fakeStoredEntity) StoredAttachmentLinks(_ context.Context, b []byte) ([]byte, error) {
	f.links = b
	return []byte(`{"links":[]}`), f.err
}
func (f *fakeStoredEntity) InitiateStoredAttachmentUpload(_ context.Context, b []byte) ([]byte, error) {
	f.upload = b
	return []byte(`{"attachmentId":"a","uploadUrl":"https://f/up"}`), f.err
}
func (f *fakeStoredEntity) ConfirmStoredAttachmentUpload(_ context.Context, id string, b []byte) ([]byte, error) {
	f.confirmID, f.confirm = id, b
	return []byte(`{"status":"COMPLETE"}`), f.err
}
func (f *fakeStoredEntity) DeleteStoredAttachment(_ context.Context, b []byte) error {
	f.deleted = b
	return f.err
}
func (f *fakeStoredEntity) OpenInlineImage(_ context.Context, id string) (*entity.InlineImageStream, error) {
	f.imageID = id
	return f.image, f.err
}

func serveStoredAtt(h http.HandlerFunc, method, target, body string, user bool) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /attachments/download", h)
	mux.HandleFunc("POST /attachments/upload", h)
	mux.HandleFunc("POST /attachments/upload/{id}/confirm", h)
	mux.HandleFunc("POST /attachments/delete", h)
	mux.HandleFunc("GET /attachments/inline/{id}/content", h)
	r := httptest.NewRequest(method, target, strings.NewReader(body))
	if user {
		r = withUser(r)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, r)
	return rec
}

func TestStoredAttachments_ForwardBodiesAndStatuses(t *testing.T) {
	f := &fakeStoredEntity{}
	h := NewStoredAttachmentHandler(f)

	rec := serveStoredAtt(h.ResolveLinks, http.MethodPost, "/attachments/download", `{"type":"work_item","ids":["a"]}`, true)
	if rec.Code != http.StatusOK || string(f.links) != `{"type":"work_item","ids":["a"]}` || rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("download: %d %q cache=%q", rec.Code, f.links, rec.Header().Get("Cache-Control"))
	}
	rec = serveStoredAtt(h.InitiateUpload, http.MethodPost, "/attachments/upload", `{"type":"work_item","parentId":"p","name":"a.pdf","sizeBytes":5}`, true)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"uploadUrl"`) || len(f.upload) == 0 {
		t.Errorf("upload: %d %s", rec.Code, rec.Body.String())
	}
	rec = serveStoredAtt(h.ConfirmUpload, http.MethodPost, "/attachments/upload/"+storedAttID+"/confirm", `{"type":"work_item"}`, true)
	if rec.Code != http.StatusOK || f.confirmID != storedAttID {
		t.Errorf("confirm: %d id=%q", rec.Code, f.confirmID)
	}
	rec = serveStoredAtt(h.DeleteAttachment, http.MethodPost, "/attachments/delete", `{"type":"work_item","id":"`+storedAttID+`"}`, true)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 || len(f.deleted) == 0 {
		t.Errorf("delete: %d body=%q", rec.Code, rec.Body.String())
	}
}

func TestStoredAttachments_RequireAUserAndAValidRequest(t *testing.T) {
	f := &fakeStoredEntity{}
	h := NewStoredAttachmentHandler(f)
	for name, tc := range map[string]struct {
		fn     http.HandlerFunc
		method string
		target string
		body   string
		user   bool
		want   int
	}{
		"download no user":      {h.ResolveLinks, "POST", "/attachments/download", `{}`, false, http.StatusUnauthorized},
		"download empty body":   {h.ResolveLinks, "POST", "/attachments/download", ``, true, http.StatusBadRequest},
		"confirm bad id":        {h.ConfirmUpload, "POST", "/attachments/upload/nope/confirm", `{"type":"work_item"}`, true, http.StatusBadRequest},
		"confirm no user":       {h.ConfirmUpload, "POST", "/attachments/upload/" + storedAttID + "/confirm", `{}`, false, http.StatusUnauthorized},
		"image bad id":          {h.InlineImageContent, "GET", "/attachments/inline/nope/content", ``, true, http.StatusBadRequest},
		"image no user":         {h.InlineImageContent, "GET", "/attachments/inline/" + storedAttID + "/content", ``, false, http.StatusUnauthorized},
		"download body too big": {h.ResolveLinks, "POST", "/attachments/download", strings.Repeat("a", maxRequestBodyBytes+10), true, http.StatusRequestEntityTooLarge},
	} {
		if rec := serveStoredAtt(tc.fn, tc.method, tc.target, tc.body, tc.user); rec.Code != tc.want {
			t.Errorf("%s: status = %d, want %d", name, rec.Code, tc.want)
		}
	}
	if f.links != nil || f.imageID != "" || f.confirmID != "" {
		t.Error("the entity service was called for a request that should have been refused here")
	}
}

func TestStoredAttachments_MapEntityErrors(t *testing.T) {
	for name, tc := range map[string]struct {
		status  int
		body    string
		want    int
		message string
	}{
		"closed item":  {409, `{"message":"This case is closed, so attachments can't be added."}`, 409, "This case is closed"},
		"not uploader": {403, `{"message":"Only the person who uploaded an attachment can delete it."}`, 403, "Only the person who uploaded"},
		"rate limit":   {429, `{"message":"too many attachment links requested; try again in a minute"}`, 429, "too many attachment links"},
		"bad input":    {400, `{"message":"the file is larger than the 100 MiB limit"}`, 400, "larger than"},
		"not found":    {404, `{"message":"attachment not found"}`, 404, ErrMsgNotFound},
		"storage down": {503, `{"message":"file storage is unavailable"}`, 503, "Failed to delete"},
		"internal":     {500, `{"message":"boom: secret detail"}`, 500, "Failed to delete"},
		"raw body":     {409, `not json secret`, 409, "Failed to delete"},
	} {
		f := &fakeStoredEntity{err: &apierror.Error{StatusCode: tc.status, Body: tc.body}}
		h := NewStoredAttachmentHandler(f)
		rec := serveStoredAtt(h.DeleteAttachment, "POST", "/attachments/delete", `{"type":"work_item","id":"x"}`, true)
		if rec.Code != tc.want || !strings.Contains(rec.Body.String(), tc.message) || strings.Contains(rec.Body.String(), "secret") {
			t.Errorf("%s: status = %d body = %s, want %d containing %q and no upstream detail", name, rec.Code, rec.Body.String(), tc.want, tc.message)
		}
	}
}

// flushRecorder proves the image is streamed: bytes reach the response as the
// upstream body is read, not after it was held in memory.
type countingBody struct {
	io.Reader
	reads int
}

func (c *countingBody) Read(p []byte) (int, error) { c.reads++; return c.Reader.Read(p) }
func (c *countingBody) Close() error               { return nil }

func TestStoredAttachments_InlineImageIsStreamedWithSafeHeaders(t *testing.T) {
	body := &countingBody{Reader: io.LimitReader(zeroReader{}, 100_000)}
	f := &fakeStoredEntity{image: &entity.InlineImageStream{Body: body, ContentType: "image/PNG; charset=binary", Length: 100_000}}
	rec := serveStoredAtt(NewStoredAttachmentHandler(f).InlineImageContent, "GET", "/attachments/inline/"+storedAttID+"/content", "", true)
	if rec.Code != http.StatusOK || rec.Body.Len() != 100_000 || f.imageID != storedAttID {
		t.Fatalf("status = %d len = %d id = %q", rec.Code, rec.Body.Len(), f.imageID)
	}
	if body.reads < 2 {
		t.Errorf("body read in %d call(s): expected a streamed copy, not one buffered read", body.reads)
	}
	for k, want := range map[string]string{
		"Content-Type": "image/png", "Content-Length": "100000", "X-Content-Type-Options": "nosniff",
		"Content-Disposition": "inline", "Cache-Control": "private, max-age=300",
	} {
		if got := rec.Header().Get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
	if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "sandbox") || !strings.Contains(csp, "default-src 'none'") {
		t.Errorf("Content-Security-Policy = %q", csp)
	}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

func TestStoredAttachments_InlineImageNeutralisesAnUnknownType(t *testing.T) {
	for _, ct := range []string{"text/html", "image/svg+xml", "application/javascript", ""} {
		f := &fakeStoredEntity{image: &entity.InlineImageStream{Body: io.NopCloser(strings.NewReader("<script>")), ContentType: ct, Length: -1}}
		rec := serveStoredAtt(NewStoredAttachmentHandler(f).InlineImageContent, "GET", "/attachments/inline/"+storedAttID+"/content", "", true)
		if got := rec.Header().Get("Content-Type"); got != "application/octet-stream" {
			t.Errorf("type %q was sent as %q, want application/octet-stream", ct, got)
		}
		if rec.Header().Get("Content-Length") != "" {
			t.Errorf("Content-Length %q set although the length is unknown", rec.Header().Get("Content-Length"))
		}
	}
}

func TestStoredAttachments_InlineImageErrorsAreMapped(t *testing.T) {
	for status, want := range map[int]int{404: 404, 403: 403, 503: 503, 500: 500} {
		f := &fakeStoredEntity{err: &apierror.Error{StatusCode: status, Body: `{"message":"x"}`}}
		rec := serveStoredAtt(NewStoredAttachmentHandler(f).InlineImageContent, "GET", "/attachments/inline/"+storedAttID+"/content", "", true)
		if rec.Code != want {
			t.Errorf("entity %d -> %d, want %d", status, rec.Code, want)
		}
	}
}
