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

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// fakeBlobEntity is the entity-service side of the file routes: it records what
// this backend forwarded and answers as configured.
type fakeBlobEntity struct {
	putErr, listErr, confirmErr, deleteErr, fileErr error

	put struct {
		id, kind string
		index    int
		size     int64
		data     string
	}
	chunks      string
	confirmBody string
	// directErr fails the two direct-link calls; directKind and directID record
	// what was asked.
	directErr   error
	directKind  string
	directID    string
	deleteCalls int
	deleteBody  string

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

func (f *fakeBlobEntity) ConfirmStoredAttachmentUpload(_ context.Context, _ string, b []byte) ([]byte, error) {
	f.confirmBody = string(b)
	return []byte(`{"id":"` + storedAttID + `","status":"COMPLETE"}`), f.confirmErr
}

func (f *fakeBlobEntity) DeleteStoredAttachment(_ context.Context, b []byte) error {
	f.deleteCalls++
	f.deleteBody = string(b)
	return f.deleteErr
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

func serveBlob(e *fakeBlobEntity, method, target string, body io.Reader, hdr map[string]string, user bool) *httptest.ResponseRecorder {
	h := NewBlobAttachmentHandler(e)
	mux := http.NewServeMux()
	mux.HandleFunc("PUT /attachments/upload/{id}/chunks/{index}", h.PutChunk)
	mux.HandleFunc("GET /attachments/upload/{id}/chunks", h.ListChunks)
	mux.HandleFunc("POST /attachments/upload/{id}/confirm", h.ConfirmUpload)
	mux.HandleFunc("POST /attachments/delete", h.DeleteAttachment)
	mux.HandleFunc("POST /attachments/{id}/upload-links", h.DirectUploadLinks)
	mux.HandleFunc("GET /attachments/download/{id}/content", h.Download)
	mux.HandleFunc("POST /attachments/{id}/download-links", h.DirectDownloadLink)
	mux.HandleFunc("GET /attachments/inline/{id}/content", h.InlineImage)
	r := httptest.NewRequest(method, target, body)
	for k, v := range hdr {
		r.Header.Set(k, v)
	}
	if user {
		r = withUser(r)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, r)
	return rec
}

// Every kind entity-service stores works the same way here: the type is only
// relayed to entity-service, which decides who may add what to which parent.
func TestBlobPutChunk_ForwardsTheBytesAndTheTypeToEntity(t *testing.T) {
	for _, kind := range []string{"work_item", "comment", "deployment"} {
		e := &fakeBlobEntity{}
		rec := serveBlob(e, http.MethodPut, "/attachments/upload/"+storedAttID+"/chunks/2?type="+kind, strings.NewReader("hello"), nil, true)
		if rec.Code != 204 || rec.Body.Len() != 0 {
			t.Fatalf("%s: %d %q", kind, rec.Code, rec.Body.String())
		}
		if e.put.id != storedAttID || e.put.kind != kind || e.put.index != 2 || e.put.size != 5 || e.put.data != "hello" {
			t.Errorf("%s: forwarded %+v", kind, e.put)
		}
	}
}

func TestBlobPutChunk_Refusals(t *testing.T) {
	url := "/attachments/upload/" + storedAttID + "/chunks/0?type=work_item"
	if rec := serveBlob(&fakeBlobEntity{}, http.MethodPut, url, strings.NewReader("x"), nil, false); rec.Code != 401 {
		t.Errorf("no user: %d", rec.Code)
	}
	if rec := serveBlob(&fakeBlobEntity{}, http.MethodPut, "/attachments/upload/nope/chunks/0?type=work_item", strings.NewReader("x"), nil, true); rec.Code != 400 {
		t.Errorf("bad id: %d", rec.Code)
	}
	if rec := serveBlob(&fakeBlobEntity{}, http.MethodPut, "/attachments/upload/"+storedAttID+"/chunks/-1?type=work_item", strings.NewReader("x"), nil, true); rec.Code != 400 {
		t.Errorf("bad index: %d", rec.Code)
	}
	if rec := serveBlob(&fakeBlobEntity{}, http.MethodPut, "/attachments/upload/"+storedAttID+"/chunks/0?type=case", strings.NewReader("x"), nil, true); rec.Code != 400 {
		t.Errorf("bad type: %d", rec.Code)
	}
	// A chunk that declares no length, or one over 8 MiB, never reaches entity-service.
	e := &fakeBlobEntity{}
	r := httptest.NewRequest(http.MethodPut, url, io.NopCloser(strings.NewReader("x")))
	r.ContentLength = -1
	rec := httptest.NewRecorder()
	mux := http.NewServeMux()
	mux.HandleFunc("PUT /attachments/upload/{id}/chunks/{index}", NewBlobAttachmentHandler(e).PutChunk)
	mux.ServeHTTP(rec, withUser(r))
	if rec.Code != http.StatusLengthRequired || e.put.id != "" {
		t.Errorf("no length: %d, forwarded %+v", rec.Code, e.put)
	}
	big := httptest.NewRequest(http.MethodPut, url, strings.NewReader("x"))
	big.ContentLength = ChunkBytes + 1
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, withUser(big))
	if rec.Code != http.StatusRequestEntityTooLarge || e.put.id != "" {
		t.Errorf("too big: %d, forwarded %+v", rec.Code, e.put)
	}
}

func TestBlobPutChunk_EntityRefusalsReachTheBrowser(t *testing.T) {
	for name, tc := range map[string]struct {
		err  error
		want int
	}{
		"hidden or not theirs": {&apierror.Error{StatusCode: 404}, 404},
		"closed item":          {&apierror.Error{StatusCode: 409, Body: `{"message":"This case is closed."}`}, 409},
		"too busy":             {&apierror.Error{StatusCode: 429, Body: `{"message":"try again"}`}, 429},
		"storage down":         {&apierror.Error{StatusCode: 503}, 503},
		"entity broke":         {&apierror.Error{StatusCode: 500}, 500},
	} {
		rec := serveBlob(&fakeBlobEntity{putErr: tc.err}, http.MethodPut, "/attachments/upload/"+storedAttID+"/chunks/0?type=work_item", strings.NewReader("x"), nil, true)
		if rec.Code != tc.want {
			t.Errorf("%s: %d, want %d", name, rec.Code, tc.want)
		}
	}
	rec := serveBlob(&fakeBlobEntity{putErr: &apierror.Error{StatusCode: 409, Body: `{"message":"This case is closed."}`}}, http.MethodPut, "/attachments/upload/"+storedAttID+"/chunks/0?type=work_item", strings.NewReader("x"), nil, true)
	if !strings.Contains(rec.Body.String(), "This case is closed.") {
		t.Errorf("body = %s, want entity's own words", rec.Body.String())
	}
}

func TestBlobConfirm_ForwardsOnlyTypeAndChunks(t *testing.T) {
	e := &fakeBlobEntity{}
	rec := serveBlob(e, http.MethodPost, "/attachments/upload/"+storedAttID+"/confirm", strings.NewReader(`{"type":"work_item","chunks":3,"sizeBytes":1,"asAdmin":true}`), nil, true)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"COMPLETE"`) {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	var sent map[string]any
	_ = json.Unmarshal([]byte(e.confirmBody), &sent)
	if len(sent) != 2 || sent["type"] != "work_item" || sent["chunks"] != float64(3) {
		t.Errorf("entity got %v, want only type and chunks (a size the browser claims is never forwarded)", sent)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control = %q", rec.Header().Get("Cache-Control"))
	}
}

func TestBlobConfirm_BadInputAndRefusals(t *testing.T) {
	url := "/attachments/upload/" + storedAttID + "/confirm"
	if rec := serveBlob(&fakeBlobEntity{}, http.MethodPost, url, strings.NewReader(`{"type":"nope","chunks":1}`), nil, true); rec.Code != 400 {
		t.Errorf("bad type: %d", rec.Code)
	}
	if rec := serveBlob(&fakeBlobEntity{}, http.MethodPost, "/attachments/upload/x/confirm", strings.NewReader(`{"type":"work_item","chunks":1}`), nil, true); rec.Code != 400 {
		t.Errorf("bad id: %d", rec.Code)
	}
	if rec := serveBlob(&fakeBlobEntity{}, http.MethodPost, url, strings.NewReader(`{"type":"work_item","chunks":1}`), nil, false); rec.Code != 401 {
		t.Errorf("no user: %d", rec.Code)
	}
	for _, status := range []int{400, 403, 409, 429} {
		e := &fakeBlobEntity{confirmErr: &apierror.Error{StatusCode: status, Body: `{"message":"entity words"}`}}
		rec := serveBlob(e, http.MethodPost, url, strings.NewReader(`{"type":"work_item","chunks":1}`), nil, true)
		if rec.Code != status || !strings.Contains(rec.Body.String(), "entity words") {
			t.Errorf("%d: got %d %s", status, rec.Code, rec.Body.String())
		}
	}
}

func TestBlobDownload_StreamsEntitysFileAndAddsTheSafetyHeaders(t *testing.T) {
	e := &fakeBlobEntity{file: fileOf(200, "0123456789", map[string]string{
		"Content-Type": "application/octet-stream", "Content-Length": "10", "Accept-Ranges": "bytes",
		"Content-Disposition": `attachment; filename=report.pdf`, "Cache-Control": "private, no-store",
		"Set-Cookie": "leak=1", "X-Storage-Key": "csm/projects/p/secret",
	})}
	rec := serveBlob(e, http.MethodGet, "/attachments/download/"+storedAttID+"/content?type=work_item", nil, nil, true)
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
	if h.Get("Set-Cookie") != "" || h.Get("X-Storage-Key") != "" || strings.Contains(rec.Body.String(), "secret") {
		t.Errorf("an entity header leaked: %v", h)
	}
	if e.kindAsked != "work_item" || e.idAsked != storedAttID {
		t.Errorf("asked %q %q", e.kindAsked, e.idAsked)
	}
}

func TestBlobDownload_RangeAndRefusals(t *testing.T) {
	url := "/attachments/download/" + storedAttID + "/content?type=work_item"
	e := &fakeBlobEntity{file: fileOf(206, "234", map[string]string{"Content-Range": "bytes 2-4/10", "Content-Length": "3"})}
	rec := serveBlob(e, http.MethodGet, url, nil, map[string]string{"Range": "bytes=2-4"}, true)
	if rec.Code != 206 || rec.Header().Get("Content-Range") != "bytes 2-4/10" || e.rangeAsked != "bytes=2-4" || rec.Body.String() != "234" {
		t.Errorf("%d %v asked %q", rec.Code, rec.Header(), e.rangeAsked)
	}

	rec = serveBlob(&fakeBlobEntity{fileErr: &entity.RangeNotSatisfiableError{ContentRange: "bytes */10"}}, http.MethodGet, url, nil, nil, true)
	if rec.Code != http.StatusRequestedRangeNotSatisfiable || rec.Header().Get("Content-Range") != "bytes */10" {
		t.Errorf("416: %d %v", rec.Code, rec.Header())
	}
	rec = serveBlob(&fakeBlobEntity{fileErr: &apierror.Error{StatusCode: 410}}, http.MethodGet, url, nil, nil, true)
	if rec.Code != 410 || !strings.Contains(rec.Body.String(), "attachment_removed") {
		t.Errorf("gone: %d %s", rec.Code, rec.Body.String())
	}
	if rec := serveBlob(&fakeBlobEntity{fileErr: &apierror.Error{StatusCode: 404}}, http.MethodGet, url, nil, nil, true); rec.Code != 404 {
		t.Errorf("not found: %d", rec.Code)
	}
	if rec := serveBlob(&fakeBlobEntity{fileErr: &apierror.Error{StatusCode: 503}}, http.MethodGet, url, nil, nil, true); rec.Code != 503 {
		t.Errorf("storage down: %d", rec.Code)
	}
	if rec := serveBlob(&fakeBlobEntity{}, http.MethodGet, url, nil, nil, false); rec.Code != 401 {
		t.Errorf("no user: %d", rec.Code)
	}
	if rec := serveBlob(&fakeBlobEntity{}, http.MethodGet, "/attachments/download/x/content?type=work_item", nil, nil, true); rec.Code != 400 {
		t.Errorf("bad id: %d", rec.Code)
	}
	if rec := serveBlob(&fakeBlobEntity{}, http.MethodGet, "/attachments/download/"+storedAttID+"/content?type=case", nil, nil, true); rec.Code != 400 {
		t.Errorf("bad type: %d", rec.Code)
	}
}

func TestBlobInlineImage_StreamsWhatEntityShowsAsAnImage(t *testing.T) {
	e := &fakeBlobEntity{file: fileOf(200, "png", map[string]string{
		"Content-Type": "image/png", "Content-Disposition": "inline", "Cache-Control": "private, max-age=300",
	})}
	rec := serveBlob(e, http.MethodGet, "/attachments/inline/"+storedAttID+"/content", nil, nil, true)
	h := rec.Header()
	if rec.Code != 200 || rec.Body.String() != "png" || h.Get("Content-Type") != "image/png" || h.Get("Content-Disposition") != "inline" || h.Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("%d %v", rec.Code, h)
	}
	if e.idAsked != storedAttID {
		t.Errorf("asked %q", e.idAsked)
	}
	if rec := serveBlob(&fakeBlobEntity{}, http.MethodGet, "/attachments/inline/"+storedAttID+"/content", nil, nil, false); rec.Code != 401 {
		t.Errorf("no user: %d", rec.Code)
	}
	if rec := serveBlob(&fakeBlobEntity{fileErr: &apierror.Error{StatusCode: 410}}, http.MethodGet, "/attachments/inline/"+storedAttID+"/content", nil, nil, true); rec.Code != 410 {
		t.Errorf("removed: %d", rec.Code)
	}
}

func TestBlobDelete_EntityDoesTheWholeDelete(t *testing.T) {
	body := `{"type":"work_item","id":"` + storedAttID + `"}`
	e := &fakeBlobEntity{}
	rec := serveBlob(e, http.MethodPost, "/attachments/delete", strings.NewReader(body), nil, true)
	if rec.Code != 204 || e.deleteCalls != 1 {
		t.Errorf("%d, %d calls", rec.Code, e.deleteCalls)
	}
	// Someone who may not delete it gets entity's refusal.
	rec = serveBlob(&fakeBlobEntity{deleteErr: &apierror.Error{StatusCode: 403, Body: `{"message":"Only the uploader."}`}}, http.MethodPost, "/attachments/delete", strings.NewReader(body), nil, true)
	if rec.Code != 403 || !strings.Contains(rec.Body.String(), "Only the uploader.") {
		t.Errorf("forbidden: %d %s", rec.Code, rec.Body.String())
	}
}

// deleteAs sends a delete with `body` as a caller holding `roles`, through a
// handler wired with the test access guard, and returns what entity-service got.
func deleteAs(t *testing.T, roles []string, body string) (*fakeBlobEntity, int) {
	t.Helper()
	e := &fakeBlobEntity{}
	h := NewBlobAttachmentHandler(e).WithAccessGuard(NewAccessGuard(testAccessConfig()))
	r := httptest.NewRequest(http.MethodPost, "/attachments/delete", strings.NewReader(body))
	r = r.WithContext(middleware.WithUserInfo(r.Context(), &middleware.UserInfo{UserID: "u", Roles: roles}))
	rec := httptest.NewRecorder()
	h.DeleteAttachment(rec, r)
	return e, rec.Code
}

func TestBlobDelete_AnAdminsDeleteCarriesTheAdminFlag(t *testing.T) {
	body := `{"type":"work_item","id":"` + storedAttID + `"}`
	e, code := deleteAs(t, []string{"test-admin"}, body)
	if code != 204 {
		t.Fatalf("code = %d", code)
	}
	var sent map[string]any
	if err := json.Unmarshal([]byte(e.deleteBody), &sent); err != nil {
		t.Fatal(err)
	}
	if sent["asAdmin"] != true || sent["id"] != storedAttID || sent["type"] != "work_item" {
		t.Errorf("sent = %v", sent)
	}
}

func TestBlobDelete_ANonAdminNeverGetsTheAdminFlag_EvenIfTheBrowserSendsIt(t *testing.T) {
	for name, body := range map[string]string{
		"plain":      `{"type":"work_item","id":"` + storedAttID + `"}`,
		"forged":     `{"type":"work_item","id":"` + storedAttID + `","asAdmin":true}`,
		"other case": `{"type":"work_item","id":"` + storedAttID + `","ASADMIN":true,"asadmin":true}`,
	} {
		for _, roles := range [][]string{{"test-cs-engineer"}, {"test-viewer"}, nil} {
			e, code := deleteAs(t, roles, body)
			if code != 204 {
				t.Fatalf("%s %v: code = %d", name, roles, code)
			}
			if strings.Contains(strings.ToLower(e.deleteBody), "asadmin") {
				t.Errorf("%s %v: entity-service got %s", name, roles, e.deleteBody)
			}
		}
	}
}

func TestBlobDelete_AnAdminsOwnFlagIsOneTrueValue_NotWhateverTheBrowserSent(t *testing.T) {
	e, _ := deleteAs(t, []string{"test-admin"}, `{"type":"work_item","id":"`+storedAttID+`","asAdmin":false,"ASADMIN":"no"}`)
	if strings.Count(strings.ToLower(e.deleteBody), "asadmin") != 1 || !strings.Contains(e.deleteBody, `"asAdmin":true`) {
		t.Errorf("entity-service got %s", e.deleteBody)
	}
}

func TestBlobDelete_ABodyThatIsNotAnObjectIsABadRequest(t *testing.T) {
	_, code := deleteAs(t, []string{"test-admin"}, `[1,2]`)
	if code != http.StatusBadRequest {
		t.Errorf("code = %d", code)
	}
}

func TestBlobListChunks(t *testing.T) {
	e := &fakeBlobEntity{chunks: `{"chunkBytes":8388608,"chunks":[{"index":0,"size":2},{"index":1,"size":1}]}`}
	rec := serveBlob(e, http.MethodGet, "/attachments/upload/"+storedAttID+"/chunks?type=work_item", nil, nil, true)
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
	if rec := serveBlob(&fakeBlobEntity{listErr: errors.New("down")}, http.MethodGet, "/attachments/upload/"+storedAttID+"/chunks?type=work_item", nil, nil, true); rec.Code != 500 {
		t.Errorf("a failed list: %d", rec.Code)
	}
}

// The direct links are relayed as entity-service made them. They are credentials,
// so they are never cached.
func TestBlobDirectLinks_AreRelayedAndNeverCached(t *testing.T) {
	for name, tc := range map[string]struct {
		method, path, want string
	}{
		"upload":   {http.MethodPost, "/attachments/" + storedAttID + "/upload-links?type=work_item", `"links":["https://blob.example/x?sig=s"]`},
		"download": {http.MethodPost, "/attachments/" + storedAttID + "/download-links?type=work_item", `"url":"https://blob.example/x?sig=s"`},
	} {
		e := &fakeBlobEntity{}
		rec := serveBlob(e, tc.method, tc.path, nil, nil, true)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), tc.want) {
			t.Errorf("%s: %d %s", name, rec.Code, rec.Body.String())
		}
		if rec.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("%s: Cache-Control = %q", name, rec.Header().Get("Cache-Control"))
		}
		if e.directID != storedAttID || e.directKind != "work_item" {
			t.Errorf("%s: forwarded %q %q", name, e.directID, e.directKind)
		}
	}
}

// TestBlobDirectLinks_Refusals: a bad id or type, or no caller, never reaches entity-service, and a failure of it shows no link.
func TestBlobDirectLinks_Refusals(t *testing.T) {
	for name, tc := range map[string]struct {
		method, path string
		user         bool
		want         int
	}{
		"upload, bad id":      {http.MethodPost, "/attachments/not-a-uuid/upload-links?type=work_item", true, http.StatusBadRequest},
		"upload, bad type":    {http.MethodPost, "/attachments/" + storedAttID + "/upload-links?type=case", true, http.StatusBadRequest},
		"upload, no caller":   {http.MethodPost, "/attachments/" + storedAttID + "/upload-links?type=work_item", false, http.StatusUnauthorized},
		"download, bad id":    {http.MethodPost, "/attachments/not-a-uuid/download-links?type=work_item", true, http.StatusBadRequest},
		"download, bad type":  {http.MethodPost, "/attachments/" + storedAttID + "/download-links?type=case", true, http.StatusBadRequest},
		"download, no caller": {http.MethodPost, "/attachments/" + storedAttID + "/download-links?type=work_item", false, http.StatusUnauthorized},
	} {
		e := &fakeBlobEntity{}
		if rec := serveBlob(e, tc.method, tc.path, nil, nil, tc.user); rec.Code != tc.want {
			t.Errorf("%s: %d, want %d", name, rec.Code, tc.want)
		}
		if e.directID != "" {
			t.Errorf("%s: entity-service was called", name)
		}
	}
	// Whatever entity-service refuses reaches the browser through the usual mapping,
	// and never with a link in it.
	e := &fakeBlobEntity{directErr: errors.New("down")}
	rec := serveBlob(e, http.MethodPost, "/attachments/"+storedAttID+"/download-links?type=work_item", nil, nil, true)
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
	rec := serveBlob(e, http.MethodPost, "/attachments/"+storedAttID+"/download-links?type=work_item", nil, nil, true)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
	if strings.Contains(logs.String(), "level=ERROR") {
		t.Errorf("a 404 for a link was logged as an error:\n%s", logs.String())
	}
}
