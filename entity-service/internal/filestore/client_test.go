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

package filestore

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fakeSFTPGo is just enough of SFTPGo's user API for the client: a token
// endpoint behind basic auth, and bearer-protected share/dir/file endpoints.
type fakeSFTPGo struct {
	t        *testing.T
	srv      *httptest.Server
	tokens   atomic.Int32
	valid    atomic.Value // the token currently accepted
	handlers map[string]http.HandlerFunc
}

func newFake(t *testing.T) *fakeSFTPGo {
	f := &fakeSFTPGo{t: t, handlers: map[string]http.HandlerFunc{}}
	f.valid.Store("")
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/user/token" {
			u, p, ok := r.BasicAuth()
			if !ok || u != "svc" || p != "secret" {
				http.Error(w, `{"error":"bad credentials"}`, http.StatusUnauthorized)
				return
			}
			n := f.tokens.Add(1)
			tok := "tok-" + string(rune('0'+n))
			f.valid.Store(tok)
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": tok})
			return
		}
		if r.Header.Get("Authorization") != "Bearer "+f.valid.Load().(string) {
			http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
			return
		}
		h, ok := f.handlers[r.Method+" "+r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		h(w, r)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeSFTPGo) client() *Client {
	return NewClient(f.srv.URL, "https://files.example.com/", "svc", "secret")
}

func TestCreateShare_SendsOneFileScopeAndReadsTheIdFromTheHeader(t *testing.T) {
	f := newFake(t)
	var got map[string]any
	f.handlers["POST /api/v2/user/shares"] = func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.Header().Set("X-Object-Id", "share123")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"message":"Share created"}`))
	}
	exp := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	id, err := f.client().CreateShare(context.Background(), ShareSpec{
		Name: "dl", Scope: ShareRead, Path: "csm/a/b.txt", MaxUses: 7, ExpiresAt: exp,
	})
	if err != nil {
		t.Fatalf("CreateShare: %v", err)
	}
	if id != "share123" {
		t.Errorf("id = %q, want share123", id)
	}
	paths, _ := got["paths"].([]any)
	if len(paths) != 1 || paths[0] != "/csm/a/b.txt" {
		t.Errorf("paths = %v, want exactly [/csm/a/b.txt]", got["paths"])
	}
	if got["scope"] != float64(ShareRead) || got["max_tokens"] != float64(7) {
		t.Errorf("scope/max_tokens = %v/%v", got["scope"], got["max_tokens"])
	}
	if got["expires_at"] != float64(exp.UnixMilli()) {
		t.Errorf("expires_at = %v, want %d (ms epoch)", got["expires_at"], exp.UnixMilli())
	}
}

func TestCreateShare_NoIdHeaderIsAnError(t *testing.T) {
	f := newFake(t)
	f.handlers["POST /api/v2/user/shares"] = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}
	if _, err := f.client().CreateShare(context.Background(), ShareSpec{Path: "x", Scope: ShareRead}); err == nil {
		t.Fatal("want an error when SFTPGo returns no share id")
	}
}

func TestURLs_UseThePublicAddressAndAskForTheRawFile(t *testing.T) {
	c := NewClient("http://internal:8080", "https://files.example.com/", "u", "p")
	if got, want := c.DownloadURL("abc"), "https://files.example.com/api/v2/shares/abc?compress=false"; got != want {
		t.Errorf("DownloadURL = %q, want %q", got, want)
	}
	if got, want := c.UploadURL("abc"), "https://files.example.com/api/v2/shares/abc"; got != want {
		t.Errorf("UploadURL = %q, want %q", got, want)
	}
	// With no public address the internal one is used.
	if got := NewClient("http://internal:8080/", "", "u", "p").UploadURL("z"); got != "http://internal:8080/api/v2/shares/z" {
		t.Errorf("fallback UploadURL = %q", got)
	}
}

func TestMkdirAllRenameRemoveDir_SendTheDocumentedCalls(t *testing.T) {
	f := newFake(t)
	var calls []string
	record := func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery)
		w.WriteHeader(http.StatusOK)
	}
	f.handlers["POST /api/v2/user/dirs"] = record
	f.handlers["PATCH /api/v2/user/files"] = record
	f.handlers["DELETE /api/v2/user/dirs"] = record

	c := f.client()
	ctx := context.Background()
	if err := c.MkdirAll(ctx, "csm/final/x"); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := c.Rename(ctx, "csm/staging/a/up.txt", "csm/final/x/wia1_up.txt"); err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if err := c.RemoveDir(ctx, "csm/staging/a"); err != nil {
		t.Fatalf("RemoveDir: %v", err)
	}
	want := []string{
		"POST /api/v2/user/dirs?mkdir_parents=true&path=%2Fcsm%2Ffinal%2Fx",
		"PATCH /api/v2/user/files?path=%2Fcsm%2Fstaging%2Fa%2Fup.txt&target=%2Fcsm%2Ffinal%2Fx%2Fwia1_up.txt",
		"DELETE /api/v2/user/dirs?path=%2Fcsm%2Fstaging%2Fa",
	}
	if len(calls) != len(want) {
		t.Fatalf("calls = %v, want %v", calls, want)
	}
	for i := range want {
		if calls[i] != want[i] {
			t.Errorf("call %d = %q, want %q", i, calls[i], want[i])
		}
	}
}

func TestRemoveFile_SendsTheDocumentedCallAndTreatsAMissingFileAsNotFound(t *testing.T) {
	f := newFake(t)
	var got string
	f.handlers["DELETE /api/v2/user/files"] = func(w http.ResponseWriter, r *http.Request) {
		got = r.Method + " " + r.URL.Path + "?" + r.URL.RawQuery
		if r.URL.Query().Get("path") == "/csm/gone.pdf" {
			http.Error(w, `{"message":"not found"}`, http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
	c := f.client()
	if err := c.RemoveFile(context.Background(), "csm/p/wia1_a.pdf"); err != nil {
		t.Fatalf("RemoveFile: %v", err)
	}
	if want := "DELETE /api/v2/user/files?path=%2Fcsm%2Fp%2Fwia1_a.pdf"; got != want {
		t.Errorf("call = %q, want %q", got, want)
	}
	if err := c.RemoveFile(context.Background(), "csm/gone.pdf"); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestListDir_ParsesEntriesAndTellsFoldersApart(t *testing.T) {
	f := newFake(t)
	f.handlers["GET /api/v2/user/dirs"] = func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"name":"up.txt","size":6,"mode":420},{"name":"sub","size":0,"mode":2147484141}]`))
	}
	entries, err := f.client().ListDir(context.Background(), "csm/staging/a")
	if err != nil {
		t.Fatalf("ListDir: %v", err)
	}
	if len(entries) != 2 || entries[0].Name != "up.txt" || entries[0].Size != 6 || entries[0].IsDir() {
		t.Errorf("file entry = %+v", entries)
	}
	if !entries[1].IsDir() {
		t.Errorf("folder entry not recognised: %+v", entries[1])
	}
}

func TestNotFoundIsAnErrNotFound(t *testing.T) {
	f := newFake(t)
	f.handlers["GET /api/v2/user/dirs"] = func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"file does not exist"}`, http.StatusNotFound)
	}
	_, err := f.client().ListDir(context.Background(), "csm/none")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestReadHead_AsksForARangeAndReadsAtMostThatMany(t *testing.T) {
	f := newFake(t)
	var rng string
	f.handlers["GET /api/v2/user/files"] = func(w http.ResponseWriter, r *http.Request) {
		rng = r.Header.Get("Range")
		// A server that ignores the range and sends everything.
		_, _ = w.Write([]byte(strings.Repeat("A", 5000)))
	}
	head, err := f.client().ReadHead(context.Background(), "csm/x", 512)
	if err != nil {
		t.Fatalf("ReadHead: %v", err)
	}
	if rng != "bytes=0-511" {
		t.Errorf("Range = %q, want bytes=0-511", rng)
	}
	if len(head) != 512 {
		t.Errorf("read %d bytes, want 512", len(head))
	}
}

func TestToken_IsReusedAndRefreshedOnceOn401(t *testing.T) {
	f := newFake(t)
	f.handlers["POST /api/v2/user/dirs"] = func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }
	c := f.client()
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		if err := c.MkdirAll(ctx, "a"); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
	}
	if n := f.tokens.Load(); n != 1 {
		t.Fatalf("token fetched %d times for 3 calls, want 1", n)
	}

	// SFTPGo revokes the cached token: the next call refreshes once and works.
	f.valid.Store("revoked")
	if err := c.MkdirAll(ctx, "a"); err != nil {
		t.Fatalf("MkdirAll after revoke: %v", err)
	}
	if n := f.tokens.Load(); n != 2 {
		t.Errorf("token fetched %d times, want 2 (one refresh)", n)
	}
}

func TestBadCredentials_FailWithoutLeakingThem(t *testing.T) {
	f := newFake(t)
	c := NewClient(f.srv.URL, "", "svc", "WRONG-PASSWORD")
	err := c.MkdirAll(context.Background(), "a")
	if err == nil {
		t.Fatal("want an error for bad credentials")
	}
	if strings.Contains(err.Error(), "WRONG-PASSWORD") {
		t.Errorf("error leaks the password: %v", err)
	}
}

func TestPersistent401_DoesNotLoop(t *testing.T) {
	f := newFake(t)
	var hits atomic.Int32
	f.handlers["POST /api/v2/user/dirs"] = func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		http.Error(w, `{"error":"nope"}`, http.StatusUnauthorized)
	}
	if err := f.client().MkdirAll(context.Background(), "a"); err == nil {
		t.Fatal("want an error")
	}
	if n := hits.Load(); n != 2 {
		t.Errorf("endpoint hit %d times, want 2 (original + one retry)", n)
	}
}

func TestClient_RefusesARedirectToAnotherOrigin(t *testing.T) {
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("the redirect was followed to %s carrying Authorization=%q", r.URL, r.Header.Get("Authorization"))
	}))
	defer other.Close()
	f := newFake(t)
	f.handlers["GET /api/v2/user/dirs"] = func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+"/steal", http.StatusFound)
	}
	if _, err := f.client().ListDir(context.Background(), "csm/x"); err == nil {
		t.Error("ListDir followed a redirect to another origin without an error")
	}
}

func TestMkdirAll_AFolderThatAlreadyExistsIsNotAnError(t *testing.T) {
	f := newFake(t)
	f.handlers["POST /api/v2/user/dirs"] = func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"the destination already exists Unable to create directory \"/csm/a\""}`, http.StatusConflict)
	}
	if err := f.client().MkdirAll(context.Background(), "csm/a"); err != nil {
		t.Errorf("MkdirAll on an existing folder = %v, want nil", err)
	}
}

func TestOpen_StreamsTheFileAndMapsAMissingOne(t *testing.T) {
	f := newFake(t)
	f.handlers["GET /api/v2/user/files"] = func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("path") != "/csm/a/pic.png" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte("IMG"))
	}
	c := f.client()
	body, size, err := c.Open(context.Background(), "csm/a/pic.png")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer body.Close()
	b, _ := io.ReadAll(body)
	if string(b) != "IMG" || size != 3 {
		t.Errorf("got %q size %d", b, size)
	}
	if _, _, err := c.Open(context.Background(), "csm/a/none.png"); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing file err = %v, want ErrNotFound", err)
	}
}

func TestListAndDeleteShares(t *testing.T) {
	f := newFake(t)
	f.handlers["GET /api/v2/user/shares"] = func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("limit") != "100" || r.URL.Query().Get("offset") != "200" {
			t.Errorf("query = %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`[{"id":"a","name":"dl","scope":1,"expires_at":1700000000000,"max_tokens":1,"used_tokens":1}]`))
	}
	var deleted string
	f.handlers["DELETE /api/v2/user/shares/a"] = func(w http.ResponseWriter, r *http.Request) { deleted = "a" }
	c := f.client()
	shares, err := c.ListShares(context.Background(), 100, 200)
	if err != nil || len(shares) != 1 || shares[0].ID != "a" || shares[0].UsedTokens != 1 || shares[0].ExpiresAt != 1700000000000 {
		t.Fatalf("ListShares = %+v, %v", shares, err)
	}
	if err := c.DeleteShare(context.Background(), "a"); err != nil || deleted != "a" {
		t.Errorf("DeleteShare: %v (deleted %q)", err, deleted)
	}
	if err := c.DeleteShare(context.Background(), "zzz"); !errors.Is(err, ErrNotFound) {
		t.Errorf("a missing share err = %v, want ErrNotFound", err)
	}
}

func TestMkdirAll_AFolderThatAlreadyExistsIsNotAnErrorEvenOnA500(t *testing.T) {
	f := newFake(t)
	f.handlers["POST /api/v2/user/dirs"] = func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"failure Unable to create directory"}`, http.StatusInternalServerError)
	}
	f.handlers["GET /api/v2/user/dirs"] = func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`[]`)) }
	if err := f.client().MkdirAll(context.Background(), "csm/a"); err != nil {
		t.Errorf("MkdirAll on an existing folder = %v, want nil", err)
	}
	// A real failure still fails: the folder cannot be listed either.
	g := newFake(t)
	g.handlers["POST /api/v2/user/dirs"] = f.handlers["POST /api/v2/user/dirs"]
	if err := g.client().MkdirAll(context.Background(), "csm/a"); err == nil {
		t.Error("MkdirAll succeeded although the folder neither exists nor could be created")
	}
}

func TestPutFile_SendsOneMultipartFileIntoTheFolder(t *testing.T) {
	f := newFake(t)
	var query, field, name string
	var body []byte
	f.handlers["POST /api/v2/user/files/upload"] = func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("not multipart: %v", err)
		}
		for k, files := range r.MultipartForm.File {
			field, name = k, files[0].Filename
			fh, _ := files[0].Open()
			body, _ = io.ReadAll(fh)
		}
		w.WriteHeader(http.StatusCreated)
	}
	if err := f.client().PutFile(context.Background(), "csm/projects/p/workitems/w/inline_attachments", "cia1_pic.png", []byte("IMG")); err != nil {
		t.Fatalf("PutFile: %v", err)
	}
	if query != "mkdir_parents=true&path=%2Fcsm%2Fprojects%2Fp%2Fworkitems%2Fw%2Finline_attachments" {
		t.Errorf("query = %q", query)
	}
	if field != "filenames" || name != "cia1_pic.png" || string(body) != "IMG" {
		t.Errorf("field=%q name=%q body=%q", field, name, body)
	}
}
