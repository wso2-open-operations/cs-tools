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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/middleware"
)

// fakeBlobEntity is the entity-service side of the file routes: it records what
// this backend forwarded and answers as configured.
type fakeBlobEntity struct {
	putErr, listErr, fileErr, directErr error

	put struct {
		id, kind string
		index    int
		size     int64
		data     string
	}
	chunks string
	// directKind and directID record what a direct-link call asked for.
	directKind string
	directID   string

	// file is what a download or an inline read returns.
	file       *entity.FileResponse
	rangeAsked string
	kindAsked  string
	idAsked    string
}

func (f *fakeBlobEntity) PutStoredAttachmentChunk(_ context.Context, id, kind string, index int, size int64, body io.Reader) error {
	b, _ := io.ReadAll(body)
	f.put.id, f.put.kind, f.put.index, f.put.size, f.put.data = id, kind, index, size, string(b)
	return f.putErr
}

func (f *fakeBlobEntity) ListStoredAttachmentChunks(context.Context, string, string) ([]byte, error) {
	return []byte(f.chunks), f.listErr
}

// DirectStoredAttachmentUploadLinks records the call and answers with one link.
func (f *fakeBlobEntity) DirectStoredAttachmentUploadLinks(_ context.Context, id, kind string) ([]byte, error) {
	f.directID, f.directKind = id, kind
	return []byte(`{"links":["https://blob.example/x?sig=s"],"chunkBytes":8388608}`), f.directErr
}

// DirectStoredAttachmentDownloadLink records the call and answers with a link.
func (f *fakeBlobEntity) DirectStoredAttachmentDownloadLink(_ context.Context, id, kind string) ([]byte, error) {
	f.directID, f.directKind = id, kind
	return []byte(`{"url":"https://blob.example/x?sig=s","name":"a.pdf"}`), f.directErr
}

func (f *fakeBlobEntity) DownloadStoredAttachment(_ context.Context, kind, id, rangeHeader string) (*entity.FileResponse, error) {
	f.kindAsked, f.idAsked, f.rangeAsked = kind, id, rangeHeader
	return f.file, f.fileErr
}

func (f *fakeBlobEntity) InlineImageContent(_ context.Context, id string) (*entity.FileResponse, error) {
	f.idAsked = id
	return f.file, f.fileErr
}

// fileOf builds a streamed response with the headers entity-service sends.
func fileOf(status int, body string, hdr map[string]string) *entity.FileResponse {
	h := http.Header{}
	for k, v := range hdr {
		h.Set(k, v)
	}
	return &entity.FileResponse{StatusCode: status, Header: h, Body: io.NopCloser(strings.NewReader(body))}
}

// serveBlob sends a request as a signed-in caller. permit decides the module
// permission check (nil allows everything) and records each check.
func serveBlob(t *testing.T, e *fakeBlobEntity, stored *fakeStoredEntity, permit func(middleware.Module, middleware.Action) bool, method, target string, body io.Reader, hdr map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	sh := NewStoredAttachmentHandler(stored, func(w http.ResponseWriter, _ *http.Request, m middleware.Module, a middleware.Action) bool {
		if permit != nil && !permit(m, a) {
			writeError(w, http.StatusForbidden, "no permission")
			return false
		}
		return true
	})
	h := NewBlobAttachmentHandler(sh, e)
	mux := http.NewServeMux()
	mux.HandleFunc("PUT /attachments/upload/{id}/chunks/{index}", h.PutChunk)
	mux.HandleFunc("GET /attachments/upload/{id}/chunks", h.ListChunks)
	mux.HandleFunc("POST /attachments/upload/{id}/confirm", h.ConfirmUpload)
	mux.HandleFunc("POST /attachments/delete", h.DeleteAttachment)
	mux.HandleFunc("POST /attachments/{id}/upload-links", h.DirectUploadLinks)
	mux.HandleFunc("GET /attachments/download/{id}/content", h.Download)
	mux.HandleFunc("POST /attachments/{id}/download-links", h.DirectDownloadLink)
	mux.HandleFunc("GET /attachments/inline/{id}/content", h.InlineImage)
	req := httptest.NewRequest(method, target, body)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	req = req.WithContext(middleware.WithUserInfo(req.Context(), &middleware.UserInfo{UserID: "u1"}))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestBlobPutChunk_ForwardsTheBytesAndTheTypeToEntity(t *testing.T) {
	for _, kind := range []string{"work_item", "comment", "deployment"} {
		e := &fakeBlobEntity{}
		rec := serveBlob(t, e, &fakeStoredEntity{}, nil, http.MethodPut, "/attachments/upload/"+storedID+"/chunks/1?type="+kind, strings.NewReader("hello"), nil)
		if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
			t.Fatalf("%s: %d %q", kind, rec.Code, rec.Body.String())
		}
		if e.put.id != storedID || e.put.kind != kind || e.put.index != 1 || e.put.size != 5 || e.put.data != "hello" {
			t.Errorf("%s: forwarded %+v", kind, e.put)
		}
	}
}

func TestBlobPutChunk_Refusals(t *testing.T) {
	base := "/attachments/upload/" + storedID + "/chunks/0?type=work_item"
	for name, tc := range map[string]struct {
		url  string
		want int
	}{
		"bad type":  {"/attachments/upload/" + storedID + "/chunks/0?type=nope", 400},
		"bad index": {"/attachments/upload/" + storedID + "/chunks/x?type=work_item", 400},
		"bad id":    {"/attachments/upload/nope/chunks/0?type=work_item", 400},
	} {
		e := &fakeBlobEntity{}
		if rec := serveBlob(t, e, &fakeStoredEntity{}, nil, http.MethodPut, tc.url, strings.NewReader("x"), nil); rec.Code != tc.want || e.put.id != "" {
			t.Errorf("%s: %d (want %d), forwarded %+v", name, rec.Code, tc.want, e.put)
		}
	}
	// A chunk over 8 MiB never reaches entity-service.
	e := &fakeBlobEntity{}
	sh := NewStoredAttachmentHandler(&fakeStoredEntity{}, func(http.ResponseWriter, *http.Request, middleware.Module, middleware.Action) bool { return true })
	mux := http.NewServeMux()
	mux.HandleFunc("PUT /attachments/upload/{id}/chunks/{index}", NewBlobAttachmentHandler(sh, e).PutChunk)
	big := httptest.NewRequest(http.MethodPut, base, strings.NewReader("x"))
	big.ContentLength = ChunkBytes + 1
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, big.WithContext(middleware.WithUserInfo(big.Context(), &middleware.UserInfo{UserID: "u1"})))
	if rec.Code != http.StatusRequestEntityTooLarge || e.put.id != "" {
		t.Errorf("too big: %d, forwarded %+v", rec.Code, e.put)
	}
	// And a permission denial stops it before entity-service is asked.
	e = &fakeBlobEntity{}
	rec = serveBlob(t, e, &fakeStoredEntity{}, func(middleware.Module, middleware.Action) bool { return false }, http.MethodPut, base, strings.NewReader("x"), nil)
	if rec.Code != http.StatusForbidden || e.put.id != "" {
		t.Errorf("no permission: %d, forwarded %+v", rec.Code, e.put)
	}
}

func TestBlobPutChunk_PermissionFollowsTheTypeAndTheAction(t *testing.T) {
	var got []permitCall
	permit := func(m middleware.Module, a middleware.Action) bool {
		got = append(got, permitCall{m, a})
		return true
	}
	serveBlob(t, &fakeBlobEntity{}, &fakeStoredEntity{}, permit, http.MethodPut, "/attachments/upload/"+storedID+"/chunks/0?type=deployment", strings.NewReader("x"), nil)
	serveBlob(t, &fakeBlobEntity{file: fileOf(200, "x", nil)}, &fakeStoredEntity{}, permit, http.MethodGet, "/attachments/download/"+storedID+"/content?type=work_item", nil, nil)
	want := []permitCall{{middleware.ModuleDeployments, middleware.ActionCreate}, {middleware.ModuleCases, middleware.ActionRead}}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("permission checks = %v, want %v", got, want)
	}
}

func TestBlobPutChunk_EntityRefusalsReachTheBrowser(t *testing.T) {
	for name, tc := range map[string]struct {
		err  error
		want int
	}{
		"hidden or not theirs": {apierror.NewUpstreamError(404, nil), 404},
		"too busy":             {apierror.NewUpstreamError(429, []byte("slow down")), 429},
		"forbidden":            {apierror.NewUpstreamError(403, nil), 403},
	} {
		rec := serveBlob(t, &fakeBlobEntity{putErr: tc.err}, &fakeStoredEntity{}, nil, http.MethodPut, "/attachments/upload/"+storedID+"/chunks/0?type=work_item", strings.NewReader("x"), nil)
		if rec.Code != tc.want {
			t.Errorf("%s: %d, want %d", name, rec.Code, tc.want)
		}
	}
	rec := serveBlob(t, &fakeBlobEntity{putErr: apierror.NewUpstreamError(503, nil)}, &fakeStoredEntity{}, nil, http.MethodPut, "/attachments/upload/"+storedID+"/chunks/0?type=work_item", strings.NewReader("x"), nil)
	if rec.Code < 500 {
		t.Errorf("storage down: %d, want a 5xx", rec.Code)
	}
}

func TestBlobConfirm_ForwardsOnlyTypeAndChunks(t *testing.T) {
	stored := &fakeStoredEntity{out: []byte(`{"id":"` + storedID + `","status":"COMPLETE"}`)}
	rec := serveBlob(t, &fakeBlobEntity{}, stored, nil, http.MethodPost, "/attachments/upload/"+storedID+"/confirm", strings.NewReader(`{"type":"work_item","chunks":3,"sizeBytes":1,"id":"other"}`), nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"COMPLETE"`) {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	var sent map[string]any
	_ = json.Unmarshal([]byte(stored.gotBody), &sent)
	if len(sent) != 2 || sent["type"] != "work_item" || sent["chunks"] != float64(3) || stored.gotID != storedID {
		t.Errorf("entity got %v for %q, want only type and chunks (a size the browser claims is never forwarded)", sent, stored.gotID)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control = %q", rec.Header().Get("Cache-Control"))
	}
}

func TestBlobConfirm_BadInputAndRefusals(t *testing.T) {
	url := "/attachments/upload/" + storedID + "/confirm"
	post := func(stored *fakeStoredEntity, permit func(middleware.Module, middleware.Action) bool, path, body string) int {
		return serveBlob(t, &fakeBlobEntity{}, stored, permit, http.MethodPost, path, strings.NewReader(body), nil).Code
	}
	if c := post(&fakeStoredEntity{}, nil, url, `{"type":"nope","chunks":1}`); c != 400 {
		t.Errorf("bad type: %d", c)
	}
	if c := post(&fakeStoredEntity{}, nil, "/attachments/upload/x/confirm", `{"type":"work_item","chunks":1}`); c != 400 {
		t.Errorf("bad id: %d", c)
	}
	if c := post(&fakeStoredEntity{}, nil, url, `not json`); c != 400 {
		t.Errorf("bad body: %d", c)
	}
	stored := &fakeStoredEntity{}
	if c := post(stored, func(middleware.Module, middleware.Action) bool { return false }, url, `{"type":"work_item","chunks":1}`); c != 403 || stored.gotBody != "" {
		t.Errorf("no permission: %d, forwarded %q", c, stored.gotBody)
	}
	for _, status := range []int{400, 403, 404, 409, 429} {
		if c := post(&fakeStoredEntity{err: apierror.NewUpstreamError(status, []byte(`{"message":"entity words"}`))}, nil, url, `{"type":"work_item","chunks":1}`); c != status {
			t.Errorf("entity %d gave %d", status, c)
		}
	}
}

func TestBlobDownload_StreamsEntitysFileAndAddsTheSafetyHeaders(t *testing.T) {
	e := &fakeBlobEntity{file: fileOf(200, "0123456789", map[string]string{
		"Content-Type": "application/octet-stream", "Content-Length": "10", "Accept-Ranges": "bytes",
		"Content-Disposition": `attachment; filename=report.pdf`, "Cache-Control": "private, no-store",
		"Set-Cookie": "leak=1", "X-Storage-Key": "csm/projects/p/secret",
	})}
	rec := serveBlob(t, e, &fakeStoredEntity{}, nil, http.MethodGet, "/attachments/download/"+storedID+"/content?type=work_item", nil, nil)
	if rec.Code != 200 || rec.Body.String() != "0123456789" {
		t.Fatalf("%d %q", rec.Code, rec.Body.String())
	}
	h := rec.Header()
	if h.Get("Content-Type") != "application/octet-stream" || h.Get("X-Content-Type-Options") != "nosniff" ||
		h.Get("Content-Security-Policy") != "default-src 'none'; sandbox" || h.Get("Content-Length") != "10" ||
		!strings.HasPrefix(h.Get("Content-Disposition"), "attachment;") {
		t.Errorf("headers = %v", h)
	}
	// Only the file's own headers are passed on; nothing else entity-service sent.
	if h.Get("Set-Cookie") != "" || h.Get("X-Storage-Key") != "" {
		t.Errorf("an entity header leaked: %v", h)
	}
	if e.kindAsked != "work_item" || e.idAsked != storedID {
		t.Errorf("asked %q %q", e.kindAsked, e.idAsked)
	}
}

func TestBlobDownload_RangeAndRefusals(t *testing.T) {
	url := "/attachments/download/" + storedID + "/content?type=work_item"
	e := &fakeBlobEntity{file: fileOf(206, "234", map[string]string{"Content-Range": "bytes 2-4/10", "Content-Length": "3"})}
	rec := serveBlob(t, e, &fakeStoredEntity{}, nil, http.MethodGet, url, nil, map[string]string{"Range": "bytes=2-4"})
	if rec.Code != 206 || rec.Header().Get("Content-Range") != "bytes 2-4/10" || e.rangeAsked != "bytes=2-4" || rec.Body.String() != "234" {
		t.Errorf("%d %v asked %q", rec.Code, rec.Header(), e.rangeAsked)
	}

	rec = serveBlob(t, &fakeBlobEntity{fileErr: &entity.RangeNotSatisfiableError{ContentRange: "bytes */10"}}, &fakeStoredEntity{}, nil, http.MethodGet, url, nil, nil)
	if rec.Code != http.StatusRequestedRangeNotSatisfiable || rec.Header().Get("Content-Range") != "bytes */10" {
		t.Errorf("416: %d %v", rec.Code, rec.Header())
	}
	rec = serveBlob(t, &fakeBlobEntity{fileErr: apierror.NewUpstreamError(410, []byte(`{"errorCode":"attachment_removed"}`))}, &fakeStoredEntity{}, nil, http.MethodGet, url, nil, nil)
	if rec.Code != 410 || !strings.Contains(rec.Body.String(), "attachment_removed") {
		t.Errorf("gone: %d %s", rec.Code, rec.Body.String())
	}
	if rec := serveBlob(t, &fakeBlobEntity{fileErr: apierror.NewUpstreamError(404, nil)}, &fakeStoredEntity{}, nil, http.MethodGet, url, nil, nil); rec.Code != 404 {
		t.Errorf("not found: %d", rec.Code)
	}
	if rec := serveBlob(t, &fakeBlobEntity{}, &fakeStoredEntity{}, nil, http.MethodGet, "/attachments/download/x/content?type=work_item", nil, nil); rec.Code != 400 {
		t.Errorf("bad id: %d", rec.Code)
	}
	if rec := serveBlob(t, &fakeBlobEntity{}, &fakeStoredEntity{}, nil, http.MethodGet, "/attachments/download/"+storedID+"/content?type=case", nil, nil); rec.Code != 400 {
		t.Errorf("bad type: %d", rec.Code)
	}
	denied := func(middleware.Module, middleware.Action) bool { return false }
	if rec := serveBlob(t, &fakeBlobEntity{file: fileOf(200, "x", nil)}, &fakeStoredEntity{}, denied, http.MethodGet, url, nil, nil); rec.Code != 403 {
		t.Errorf("no permission: %d", rec.Code)
	}
}

func TestBlobInlineImage_StreamsWhatEntityShowsAsAnImage(t *testing.T) {
	e := &fakeBlobEntity{file: fileOf(200, "png", map[string]string{
		"Content-Type": "image/png", "Content-Disposition": "inline", "Cache-Control": "private, max-age=300",
	})}
	rec := serveBlob(t, e, &fakeStoredEntity{}, nil, http.MethodGet, "/attachments/inline/"+storedID+"/content", nil, nil)
	h := rec.Header()
	if rec.Code != 200 || rec.Body.String() != "png" || h.Get("Content-Type") != "image/png" || h.Get("Content-Disposition") != "inline" || h.Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("%d %v", rec.Code, h)
	}
	if e.idAsked != storedID {
		t.Errorf("asked %q", e.idAsked)
	}
	if rec := serveBlob(t, &fakeBlobEntity{fileErr: apierror.NewUpstreamError(410, nil)}, &fakeStoredEntity{}, nil, http.MethodGet, "/attachments/inline/"+storedID+"/content", nil, nil); rec.Code != 410 {
		t.Errorf("removed: %d", rec.Code)
	}
}

func TestBlobDelete_EntityDoesTheWholeDelete(t *testing.T) {
	body := `{"type":"work_item","id":"` + storedID + `"}`
	stored := &fakeStoredEntity{}
	rec := serveBlob(t, &fakeBlobEntity{}, stored, nil, http.MethodPost, "/attachments/delete", strings.NewReader(body), nil)
	if rec.Code != 204 || !strings.Contains(stored.gotBody, storedID) {
		t.Errorf("%d, entity got %q", rec.Code, stored.gotBody)
	}
	// Someone who may not delete it gets entity's refusal.
	rec = serveBlob(t, &fakeBlobEntity{}, &fakeStoredEntity{err: apierror.NewUpstreamError(403, nil)}, nil, http.MethodPost, "/attachments/delete", strings.NewReader(body), nil)
	if rec.Code != 403 {
		t.Errorf("forbidden: %d", rec.Code)
	}
}

func TestBlobListChunks(t *testing.T) {
	e := &fakeBlobEntity{chunks: `{"chunkBytes":8388608,"chunks":[{"index":0,"size":2},{"index":1,"size":1}]}`}
	rec := serveBlob(t, e, &fakeStoredEntity{}, nil, http.MethodGet, "/attachments/upload/"+storedID+"/chunks?type=work_item", nil, nil)
	var out struct {
		ChunkBytes int `json:"chunkBytes"`
		Chunks     []struct{ Index, Size int }
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if rec.Code != 200 || out.ChunkBytes != ChunkBytes || len(out.Chunks) != 2 || out.Chunks[1].Size != 1 {
		t.Errorf("%d %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control = %q", rec.Header().Get("Cache-Control"))
	}
	if rec := serveBlob(t, &fakeBlobEntity{listErr: errors.New("down")}, &fakeStoredEntity{}, nil, http.MethodGet, "/attachments/upload/"+storedID+"/chunks?type=work_item", nil, nil); rec.Code < 500 {
		t.Errorf("a failed list: %d", rec.Code)
	}
}

// The direct links are relayed as entity-service made them. They are credentials,
// so they are never cached.
func TestBlobDirectLinks_AreRelayedAndNeverCached(t *testing.T) {
	for name, tc := range map[string]struct {
		method, path, want string
	}{
		"upload":   {http.MethodPost, "/attachments/" + storedID + "/upload-links?type=work_item", `"links":["https://blob.example/x?sig=s"]`},
		"download": {http.MethodPost, "/attachments/" + storedID + "/download-links?type=work_item", `"url":"https://blob.example/x?sig=s"`},
	} {
		e := &fakeBlobEntity{}
		rec := serveBlob(t, e, &fakeStoredEntity{}, nil, tc.method, tc.path, nil, nil)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), tc.want) {
			t.Errorf("%s: %d %s", name, rec.Code, rec.Body.String())
		}
		if rec.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("%s: Cache-Control = %q", name, rec.Header().Get("Cache-Control"))
		}
		if e.directID != storedID || e.directKind != "work_item" {
			t.Errorf("%s: forwarded %q %q", name, e.directID, e.directKind)
		}
	}
}

// The module permission is checked before entity-service is asked for a link:
// creating needs the create action, a download link the read action.
func TestBlobDirectLinks_CheckThePermissionFirst(t *testing.T) {
	deny := func(middleware.Module, middleware.Action) bool { return false }
	var asked []middleware.Action
	record := func(_ middleware.Module, a middleware.Action) bool {
		asked = append(asked, a)
		return true
	}
	for name, tc := range map[string]struct {
		method, path string
		action       middleware.Action
	}{
		"upload":   {http.MethodPost, "/attachments/" + storedID + "/upload-links?type=work_item", middleware.ActionCreate},
		"download": {http.MethodPost, "/attachments/" + storedID + "/download-links?type=work_item", middleware.ActionRead},
	} {
		e := &fakeBlobEntity{}
		if rec := serveBlob(t, e, &fakeStoredEntity{}, deny, tc.method, tc.path, nil, nil); rec.Code != http.StatusForbidden || e.directID != "" {
			t.Errorf("%s: denied caller got %d, entity called = %v", name, rec.Code, e.directID != "")
		}
		asked = nil
		serveBlob(t, &fakeBlobEntity{}, &fakeStoredEntity{}, record, tc.method, tc.path, nil, nil)
		if len(asked) != 1 || asked[0] != tc.action {
			t.Errorf("%s: checked %v, want the %v action", name, asked, tc.action)
		}
	}
}

// TestBlobDirectLinks_Refusals: a bad id or type, or no caller, never reaches entity-service, and a failure of it shows no link.
func TestBlobDirectLinks_Refusals(t *testing.T) {
	for name, tc := range map[string]struct {
		method, path string
		want         int
	}{
		"upload, bad id":     {http.MethodPost, "/attachments/not-a-uuid/upload-links?type=work_item", http.StatusBadRequest},
		"upload, bad type":   {http.MethodPost, "/attachments/" + storedID + "/upload-links?type=case", http.StatusBadRequest},
		"download, bad id":   {http.MethodPost, "/attachments/not-a-uuid/download-links?type=work_item", http.StatusBadRequest},
		"download, bad type": {http.MethodPost, "/attachments/" + storedID + "/download-links?type=case", http.StatusBadRequest},
	} {
		e := &fakeBlobEntity{}
		if rec := serveBlob(t, e, &fakeStoredEntity{}, nil, tc.method, tc.path, nil, nil); rec.Code != tc.want || e.directID != "" {
			t.Errorf("%s: %d (entity called = %v), want %d", name, rec.Code, e.directID != "", tc.want)
		}
	}
	// Whatever entity-service refuses reaches the browser through the usual mapping,
	// and never with a link in it.
	e := &fakeBlobEntity{directErr: errors.New("down")}
	rec := serveBlob(t, e, &fakeStoredEntity{}, nil, http.MethodPost, "/attachments/"+storedID+"/download-links?type=work_item", nil, nil)
	if rec.Code < 500 || strings.Contains(rec.Body.String(), "blob.example") {
		t.Errorf("a failed link: %d %s", rec.Code, rec.Body.String())
	}
}

// While direct transfers are off, entity-service answers a link request with a
// 404. The browser gets that 404 (and then streams the file), and the backend does
// not log it as an error: the portal asks before every large download.
func TestDirectDownloadLinkNotAvailableIsNotAnError(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	e := &fakeBlobEntity{directErr: &apierror.Error{StatusCode: http.StatusNotFound}}
	rec := serveBlob(t, e, &fakeStoredEntity{}, nil, http.MethodPost, "/attachments/"+storedID+"/download-links?type=work_item", nil, nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
	if strings.Contains(logs.String(), "level=ERROR") {
		t.Errorf("a 404 for a link was logged as an error:\n%s", logs.String())
	}
}
