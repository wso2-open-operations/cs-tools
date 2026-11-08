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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/libs/azblobstore"
)

const blobSecretKey = "csm/projects/p/workitems/1/attachments/ATT1_secret.pdf"

type fakeBlobEntity struct {
	target, locate string
	authErr        error
	locateErr      error
	confirmErr     error
	deleteErr      error
	authBody       string
	confirmBody    string
	deleteCalls    int

	relocatedBody string
	relocateErr   error
}

func (f *fakeBlobEntity) InlineImageRelocated(_ context.Context, _ string, b []byte) error {
	f.relocatedBody = string(b)
	return f.relocateErr
}

func (f *fakeBlobEntity) AuthorizeStoredAttachmentUpload(_ context.Context, _ string, b []byte) ([]byte, error) {
	f.authBody = string(b)
	return []byte(f.target), f.authErr
}
func (f *fakeBlobEntity) LocateStoredAttachment(context.Context, []byte) ([]byte, error) {
	return []byte(f.locate), f.locateErr
}
func (f *fakeBlobEntity) LocateInlineImage(context.Context, string) ([]byte, error) {
	return []byte(f.locate), f.locateErr
}
func (f *fakeBlobEntity) ConfirmStoredAttachmentUpload(_ context.Context, _ string, b []byte) ([]byte, error) {
	f.confirmBody = string(b)
	return []byte(`{"id":"x"}`), f.confirmErr
}
func (f *fakeBlobEntity) DeleteStoredAttachment(context.Context, []byte) error {
	f.deleteCalls++
	return f.deleteErr
}

type fakeBlobStore struct {
	blocks   map[int][]byte
	data     []byte
	putKey   string
	commits  int
	deleted  []string
	commitEr error
	opened   *azblobstore.Range
}

func (s *fakeBlobStore) PutBlock(_ context.Context, key string, i int, body io.Reader, size int64) error {
	b, _ := io.ReadAll(body)
	if int64(len(b)) != size {
		return azblobstore.ErrBlockSize
	}
	if s.blocks == nil {
		s.blocks = map[int][]byte{}
	}
	s.putKey, s.blocks[i] = key, b
	return nil
}
func (s *fakeBlobStore) UncommittedBlocks(context.Context, string) ([]azblobstore.Block, error) {
	var out []azblobstore.Block
	for i := 0; i < len(s.blocks); i++ {
		out = append(out, azblobstore.Block{Index: i, Size: int64(len(s.blocks[i]))})
	}
	return out, nil
}
func (s *fakeBlobStore) Commit(context.Context, string, int, string) error {
	s.commits++
	return s.commitEr
}
func (s *fakeBlobStore) Stat(context.Context, string) (azblobstore.Info, error) {
	return azblobstore.Info{Size: int64(len(s.data)), ContentType: "application/pdf"}, nil
}
func (s *fakeBlobStore) Open(_ context.Context, _ string, rng *azblobstore.Range) (*azblobstore.Object, error) {
	s.opened = rng
	if rng == nil {
		return &azblobstore.Object{Body: io.NopCloser(bytes.NewReader(s.data)), ContentLength: int64(len(s.data))}, nil
	}
	part := s.data[rng.Offset : rng.Offset+rng.Length]
	return &azblobstore.Object{Body: io.NopCloser(bytes.NewReader(part)), ContentLength: rng.Length, Partial: true, ContentRange: "bytes 0-0/0"}, nil
}
func (s *fakeBlobStore) Delete(_ context.Context, key string) error {
	s.deleted = append(s.deleted, key)
	return nil
}

const blobTarget = `{"attachmentId":"` + storedAttID + `","name":"a.pdf","contentType":"application/pdf","sizeBytes":40000000,"maxBytes":104857600,"storageKey":"` + blobSecretKey + `"}`

func serveBlob(e *fakeBlobEntity, s *fakeBlobStore, method, target string, body io.Reader, hdr map[string]string, user bool) *httptest.ResponseRecorder {
	h := NewBlobAttachmentHandler(e, s)
	mux := http.NewServeMux()
	mux.HandleFunc("PUT /attachments/upload/{id}/chunks/{index}", h.PutChunk)
	mux.HandleFunc("GET /attachments/upload/{id}/chunks", h.ListChunks)
	mux.HandleFunc("POST /attachments/upload/{id}/confirm", h.ConfirmUpload)
	mux.HandleFunc("POST /attachments/delete", h.DeleteAttachment)
	mux.HandleFunc("GET /attachments/download/{id}/content", h.Download)
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
func TestBlobPutChunk_AcceptsEveryStoredAttachmentType(t *testing.T) {
	for _, kind := range []string{"work_item", "comment", "deployment"} {
		e, s := &fakeBlobEntity{target: blobTarget}, &fakeBlobStore{}
		rec := serveBlob(e, s, http.MethodPut, "/attachments/upload/"+storedAttID+"/chunks/0?type="+kind, strings.NewReader("hello"), nil, true)
		if rec.Code != http.StatusNoContent || !strings.Contains(e.authBody, `"`+kind+`"`) {
			t.Errorf("%s: status %d, authorize body %s", kind, rec.Code, e.authBody)
		}
	}
	rec := serveBlob(&fakeBlobEntity{target: blobTarget}, &fakeBlobStore{}, http.MethodPut, "/attachments/upload/"+storedAttID+"/chunks/0?type=case", strings.NewReader("x"), nil, true)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("an unknown type gave %d, want 400", rec.Code)
	}
}

func TestBlobPutChunk_StoresUnderTheKeyEntityGave(t *testing.T) {
	e, s := &fakeBlobEntity{target: blobTarget}, &fakeBlobStore{}
	rec := serveBlob(e, s, http.MethodPut, "/attachments/upload/"+storedAttID+"/chunks/1?type=work_item", strings.NewReader("hello"), nil, true)
	if rec.Code != http.StatusNoContent || s.putKey != blobSecretKey || string(s.blocks[1]) != "hello" {
		t.Fatalf("got %d key=%q blocks=%v", rec.Code, s.putKey, s.blocks)
	}
	if !strings.Contains(e.authBody, `"work_item"`) {
		t.Errorf("authorize body = %s", e.authBody)
	}
	if strings.Contains(rec.Body.String(), "secret") {
		t.Error("the key leaked")
	}
}

func TestBlobPutChunk_Refusals(t *testing.T) {
	for name, tc := range map[string]struct {
		target string
		path   string
		body   string
		authEr error
		user   bool
		want   int
	}{
		"no user":         {blobTarget, "/0?type=work_item", "x", nil, false, 401},
		"bad type":        {blobTarget, "/0?type=nope", "x", nil, true, 400},
		"bad index":       {blobTarget, "/x?type=work_item", "x", nil, true, 400},
		"past largest":    {blobTarget, "/99?type=work_item", "x", nil, true, 400},
		"entity refuses":  {blobTarget, "/0?type=work_item", "x", &apierror.Error{StatusCode: 403}, true, 403},
		"no key returned": {`{"attachmentId":"a"}`, "/0?type=work_item", "x", nil, true, 503},
		"chunk past max":  {strings.Replace(blobTarget, "104857600", "3", 1), "/0?type=work_item", "xxxx", nil, true, 413},
	} {
		t.Run(name, func(t *testing.T) {
			e, s := &fakeBlobEntity{target: tc.target, authErr: tc.authEr}, &fakeBlobStore{}
			rec := serveBlob(e, s, http.MethodPut, "/attachments/upload/"+storedAttID+"/chunks"+tc.path, strings.NewReader(tc.body), nil, tc.user)
			if rec.Code != tc.want || len(s.blocks) != 0 {
				t.Errorf("got %d (want %d), stored %d blocks", rec.Code, tc.want, len(s.blocks))
			}
		})
	}
}

func TestBlobConfirm_CommitsThenAsksEntityWithTheStoredSize(t *testing.T) {
	e, s := &fakeBlobEntity{target: blobTarget}, &fakeBlobStore{data: make([]byte, 1234)}
	rec := serveBlob(e, s, http.MethodPost, "/attachments/upload/"+storedAttID+"/confirm", strings.NewReader(`{"type":"work_item","chunks":3}`), nil, true)
	if rec.Code != http.StatusOK || s.commits != 1 {
		t.Fatalf("got %d commits=%d", rec.Code, s.commits)
	}
	var sent map[string]any
	_ = json.Unmarshal([]byte(e.confirmBody), &sent)
	if sent["sizeBytes"] != float64(1234) || sent["contentType"] != "application/pdf" || sent["storageKey"] != nil {
		t.Errorf("confirm body to entity = %s", e.confirmBody)
	}
}

func TestBlobConfirm_RefusalDeletesTheFileButARetryableFailureKeepsIt(t *testing.T) {
	for name, tc := range map[string]struct {
		code    int
		wantDel bool
	}{
		"refused": {400, true}, "forbidden": {403, true},
		"rate": {429, false}, "conflict": {409, false}, "upstream": {503, false},
		"unauthorized": {401, false}, "timeout": {408, false},
	} {
		t.Run(name, func(t *testing.T) {
			e, s := &fakeBlobEntity{target: blobTarget, confirmErr: &apierror.Error{StatusCode: tc.code}}, &fakeBlobStore{data: []byte("x")}
			rec := serveBlob(e, s, http.MethodPost, "/attachments/upload/"+storedAttID+"/confirm", strings.NewReader(`{"type":"work_item","chunks":1}`), nil, true)
			if (len(s.deleted) == 1) != tc.wantDel {
				t.Errorf("code=%d deleted=%v", rec.Code, s.deleted)
			}
		})
	}
}

func TestBlobConfirm_MissingChunksAndBadCounts(t *testing.T) {
	e := &fakeBlobEntity{target: blobTarget}
	rec := serveBlob(e, &fakeBlobStore{commitEr: azblobstore.ErrMissingBlocks}, http.MethodPost, "/attachments/upload/"+storedAttID+"/confirm", strings.NewReader(`{"type":"work_item","chunks":2}`), nil, true)
	if rec.Code != http.StatusConflict {
		t.Errorf("missing chunks = %d", rec.Code)
	}
	// 100 MiB is 13 chunks of 8 MiB: 13 is the most, 14 is refused.
	rec = serveBlob(e, &fakeBlobStore{}, http.MethodPost, "/attachments/upload/"+storedAttID+"/confirm", strings.NewReader(`{"type":"work_item","chunks":14}`), nil, true)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("too many chunks = %d", rec.Code)
	}
}

func TestBlobDownload_StreamsAsAnOpaqueAttachmentAndNeverShowsTheKey(t *testing.T) {
	loc := `{"id":"` + storedAttID + `","name":"re\"port\r\n.html","contentType":"text/html","available":true,"storageKey":"` + blobSecretKey + `"}`
	s := &fakeBlobStore{data: []byte("0123456789")}
	rec := serveBlob(&fakeBlobEntity{locate: loc}, s, http.MethodGet, "/attachments/download/"+storedAttID+"/content?type=work_item", nil, nil, true)
	if rec.Code != 200 || rec.Body.String() != "0123456789" {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
	h := rec.Header()
	if h.Get("Content-Type") != "application/octet-stream" || !strings.HasPrefix(h.Get("Content-Disposition"), "attachment") ||
		h.Get("X-Content-Type-Options") != "nosniff" || h.Get("Accept-Ranges") != "bytes" {
		t.Errorf("headers = %v", h)
	}
	if strings.ContainsAny(h.Get("Content-Disposition"), "\r\n") {
		t.Error("header injection")
	}
	for k, v := range h {
		if strings.Contains(strings.Join(v, ""), "secret") {
			t.Errorf("key in header %s", k)
		}
	}
}

func TestBlobDownload_RangeAndUnavailable(t *testing.T) {
	loc := `{"id":"a","name":"a.bin","available":true,"storageKey":"` + blobSecretKey + `"}`
	url := "/attachments/download/" + storedAttID + "/content?type=work_item"
	s := &fakeBlobStore{data: []byte("0123456789")}
	rec := serveBlob(&fakeBlobEntity{locate: loc}, s, http.MethodGet, url, nil, map[string]string{"Range": "bytes=2-5"}, true)
	if rec.Code != http.StatusPartialContent || rec.Body.String() != "2345" || s.opened == nil || s.opened.Offset != 2 || s.opened.Length != 4 {
		t.Errorf("range: %d %q %+v", rec.Code, rec.Body.String(), s.opened)
	}
	rec = serveBlob(&fakeBlobEntity{locate: loc}, s, http.MethodGet, url, nil, map[string]string{"Range": "bytes=50-"}, true)
	if rec.Code != http.StatusRequestedRangeNotSatisfiable || rec.Header().Get("Content-Range") != "bytes */10" {
		t.Errorf("past the end: %d %v", rec.Code, rec.Header())
	}
	for reason, want := range map[string]int{"DELETED": 410, "STORAGE_UNAVAILABLE": 503, "NOT_STORED": 404, "FAILED": 404} {
		rec = serveBlob(&fakeBlobEntity{locate: `{"available":false,"reason":"` + reason + `"}`}, s, http.MethodGet, url, nil, nil, true)
		if rec.Code != want {
			t.Errorf("%s -> %d, want %d", reason, rec.Code, want)
		}
	}
	rec = serveBlob(&fakeBlobEntity{locateErr: &apierror.Error{StatusCode: 404}}, s, http.MethodGet, url, nil, nil, true)
	if rec.Code != 404 {
		t.Errorf("entity 404 = %d", rec.Code)
	}
	rec = serveBlob(&fakeBlobEntity{locate: loc}, s, http.MethodGet, url, nil, nil, false)
	if rec.Code != 401 {
		t.Errorf("no user = %d", rec.Code)
	}
}

func TestBlobInlineImage_OnlyImageTypesAreShown(t *testing.T) {
	s := &fakeBlobStore{data: []byte("img")}
	for ct, want := range map[string]string{"image/png": "image/png", "text/html": "application/octet-stream", "image/svg+xml": "application/octet-stream"} {
		loc := `{"available":true,"contentType":"` + ct + `","storageKey":"` + blobSecretKey + `"}`
		rec := serveBlob(&fakeBlobEntity{locate: loc}, s, http.MethodGet, "/attachments/inline/"+storedAttID+"/content", nil, nil, true)
		if rec.Code != 200 || rec.Header().Get("Content-Type") != want || rec.Header().Get("Content-Disposition") != "inline" {
			t.Errorf("%s -> %d %v", ct, rec.Code, rec.Header())
		}
	}
}

func TestBlobDelete_RemovesTheFileAfterEntityDeletesTheRow(t *testing.T) {
	loc := `{"available":true,"storageKey":"` + blobSecretKey + `"}`
	body := `{"type":"work_item","id":"` + storedAttID + `"}`
	s := &fakeBlobStore{}
	rec := serveBlob(&fakeBlobEntity{locate: loc}, s, http.MethodPost, "/attachments/delete", strings.NewReader(body), nil, true)
	if rec.Code != 204 || len(s.deleted) != 1 || s.deleted[0] != blobSecretKey {
		t.Errorf("%d %v", rec.Code, s.deleted)
	}
	// Someone who may not delete it leaves the file alone.
	s = &fakeBlobStore{}
	rec = serveBlob(&fakeBlobEntity{locate: loc, deleteErr: &apierror.Error{StatusCode: 403}}, s, http.MethodPost, "/attachments/delete", strings.NewReader(body), nil, true)
	if rec.Code != 403 || len(s.deleted) != 0 {
		t.Errorf("forbidden: %d %v", rec.Code, s.deleted)
	}
}

func TestBlobListChunks(t *testing.T) {
	s := &fakeBlobStore{blocks: map[int][]byte{0: []byte("ab"), 1: []byte("c")}}
	rec := serveBlob(&fakeBlobEntity{target: blobTarget}, s, http.MethodGet, "/attachments/upload/"+storedAttID+"/chunks?type=work_item", nil, nil, true)
	var out struct {
		ChunkBytes int `json:"chunkBytes"`
		Chunks     []struct{ Index, Size int }
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if rec.Code != 200 || out.ChunkBytes != ChunkBytes || len(out.Chunks) != 2 {
		t.Errorf("%d %s", rec.Code, rec.Body.String())
	}
}

func TestParseRange(t *testing.T) {
	for _, tc := range []struct {
		h     string
		off   int64
		len   int64
		ok    bool
		unsat bool
	}{
		{"bytes=0-3", 0, 4, true, false},
		{"bytes=5-", 5, 5, true, false},
		{"bytes=-3", 7, 3, true, false},
		{"bytes=8-100", 8, 2, true, false},
		{"bytes=10-", 0, 0, false, true},
		{"bytes=0-1,4-5", 0, 0, false, false},
		{"bytes=5-2", 0, 0, false, false},
		{"items=0-1", 0, 0, false, false},
		{"bytes=abc", 0, 0, false, false},
	} {
		r, ok, un := parseRange(tc.h, 10)
		if ok != tc.ok || un != tc.unsat || (ok && (r.Offset != tc.off || r.Length != tc.len)) {
			t.Errorf("%q = %+v ok=%v unsat=%v", tc.h, r, ok, un)
		}
	}
}

const relocateTo = "csm/projects/prj_9/workitems/cs0001/inline_attachments/cia1_pic.png"

func TestBlobInlineImage_MovesADescriptionImageToItsWorkItemFolder(t *testing.T) {
	loc := `{"available":true,"contentType":"image/png","storageKey":"` + blobSecretKey + `","relocateTo":"` + relocateTo + `"}`
	s := &fakeBlobStore{data: []byte("img")}
	ent := &fakeBlobEntity{locate: loc}
	rec := serveBlob(ent, s, http.MethodGet, "/attachments/inline/"+storedAttID+"/content", nil, nil, true)
	if rec.Code != 200 || rec.Body.String() != "img" {
		t.Fatalf("response = %d %q", rec.Code, rec.Body.String())
	}
	if s.putKey != relocateTo || string(s.blocks[0]) != "img" || s.commits != 1 {
		t.Errorf("copy: key %q blocks %v commits %d", s.putKey, s.blocks, s.commits)
	}
	if ent.relocatedBody != `{"storageKey":"`+relocateTo+`"}` {
		t.Errorf("entity told %q", ent.relocatedBody)
	}
	if len(s.deleted) != 1 || s.deleted[0] != blobSecretKey {
		t.Errorf("old file removal = %v, want only %q", s.deleted, blobSecretKey)
	}
}

func TestBlobInlineImage_AFailedMoveKeepsTheOldFile(t *testing.T) {
	loc := `{"available":true,"contentType":"image/png","storageKey":"` + blobSecretKey + `","relocateTo":"` + relocateTo + `"}`
	s := &fakeBlobStore{data: []byte("img")}
	ent := &fakeBlobEntity{locate: loc, relocateErr: errors.New("entity down")}
	rec := serveBlob(ent, s, http.MethodGet, "/attachments/inline/"+storedAttID+"/content", nil, nil, true)
	if rec.Code != 200 || rec.Body.String() != "img" {
		t.Fatalf("the image must still be served: %d %q", rec.Code, rec.Body.String())
	}
	if len(s.deleted) != 0 {
		t.Errorf("the old file was removed although entity-service did not accept the new key: %v", s.deleted)
	}
}

func TestBlobInlineImage_NoMoveWithoutRelocateTo(t *testing.T) {
	loc := `{"available":true,"contentType":"image/png","storageKey":"` + blobSecretKey + `"}`
	s := &fakeBlobStore{data: []byte("img")}
	ent := &fakeBlobEntity{locate: loc}
	rec := serveBlob(ent, s, http.MethodGet, "/attachments/inline/"+storedAttID+"/content", nil, nil, true)
	if rec.Code != 200 || s.commits != 0 || len(s.deleted) != 0 || ent.relocatedBody != "" {
		t.Errorf("unexpected move: %d commits %d deleted %v told %q", rec.Code, s.commits, s.deleted, ent.relocatedBody)
	}
}
