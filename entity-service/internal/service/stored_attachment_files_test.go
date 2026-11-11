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

package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/libs/azblobstore"
)

// fakeBlob is an in-memory FileStore.
type fakeBlob struct {
	staged     map[string]map[int][]byte // key -> index -> bytes
	committed  map[string][]byte
	types      map[string]string
	deleted    []string
	commitErr  error
	putErr     error
	opened     []string
	copyErr    error
	copyShort  bool        // Copy carries only part of the file
	copied     [][2]string // {from, to}
	discarded  []string
	discardErr error
	versions   map[string]int
	beforeCopy func() // runs just before Copy reads its source
}

func newFakeBlob() *fakeBlob {
	return &fakeBlob{staged: map[string]map[int][]byte{}, committed: map[string][]byte{}, types: map[string]string{}, versions: map[string]int{}}
}

func (b *fakeBlob) PutBlock(_ context.Context, key string, index int, body io.Reader, size int64) error {
	if b.putErr != nil {
		return b.putErr
	}
	data, _ := io.ReadAll(io.LimitReader(body, size))
	if b.staged[key] == nil {
		b.staged[key] = map[int][]byte{}
	}
	b.staged[key][index] = data
	return nil
}

func (b *fakeBlob) UncommittedBlocks(_ context.Context, key string) ([]azblobstore.Block, error) {
	var out []azblobstore.Block
	for i := 0; i < len(b.staged[key]); i++ {
		out = append(out, azblobstore.Block{Index: i, Size: int64(len(b.staged[key][i]))})
	}
	return out, nil
}

func (b *fakeBlob) Commit(_ context.Context, key string, count int, contentType string) error {
	if b.commitErr != nil {
		return b.commitErr
	}
	var all []byte
	for i := 0; i < count; i++ {
		chunk, ok := b.staged[key][i]
		if !ok {
			return azblobstore.ErrMissingBlocks
		}
		all = append(all, chunk...)
	}
	b.committed[key], b.types[key] = all, contentType
	b.versions[key]++
	return nil
}

func (b *fakeBlob) Stat(_ context.Context, key string) (azblobstore.Info, error) {
	data, ok := b.committed[key]
	if !ok {
		return azblobstore.Info{}, azblobstore.ErrNotFound
	}
	return azblobstore.Info{Size: int64(len(data)), ContentType: b.types[key], ETag: b.etag(key)}, nil
}

// etag is a value that changes whenever the file at key is committed again.
func (b *fakeBlob) etag(key string) string { return fmt.Sprintf("v%d", b.versions[key]) }

func (b *fakeBlob) Open(_ context.Context, key string, rng *azblobstore.Range) (*azblobstore.Object, error) {
	data, ok := b.committed[key]
	if !ok {
		return nil, azblobstore.ErrNotFound
	}
	b.opened = append(b.opened, key)
	info := azblobstore.Info{Size: int64(len(data)), ContentType: b.types[key]}
	if rng == nil {
		return &azblobstore.Object{Body: io.NopCloser(strings.NewReader(string(data))), Info: info, ContentLength: int64(len(data))}, nil
	}
	part := data[rng.Offset : rng.Offset+rng.Length]
	return &azblobstore.Object{Body: io.NopCloser(strings.NewReader(string(part))), Info: info, ContentLength: int64(len(part)), Partial: true}, nil
}

func (b *fakeBlob) Delete(_ context.Context, key string) error {
	b.deleted = append(b.deleted, key)
	_, had := b.committed[key]
	delete(b.committed, key)
	delete(b.staged, key)
	if !had {
		return azblobstore.ErrNotFound
	}
	return nil
}

// Discard removes a file and any chunks put for it and never committed.
func (b *fakeBlob) Discard(_ context.Context, key string) error {
	if b.discardErr != nil {
		return b.discardErr
	}
	b.discarded = append(b.discarded, key)
	delete(b.committed, key)
	delete(b.staged, key)
	return nil
}

// Copy duplicates a committed file, as Azure does inside the account.
func (b *fakeBlob) Copy(_ context.Context, from, to, srcETag string) error {
	if b.beforeCopy != nil {
		b.beforeCopy()
	}
	if b.copyErr != nil {
		return b.copyErr
	}
	if srcETag != "" && srcETag != b.etag(from) {
		return azblobstore.ErrSourceChanged
	}
	data, ok := b.committed[from]
	if !ok {
		return azblobstore.ErrNotFound
	}
	if b.copyShort {
		data = data[:len(data)/2]
	}
	b.committed[to], b.types[to] = append([]byte(nil), data...), b.types[from]
	b.copied = append(b.copied, [2]string{from, to})
	return nil
}

func newFileSvc(repo *fakeStoredRepo, store FileStore) StoredAttachmentFileService {
	return NewStoredAttachmentFileService(newBackendSvc(repo), store, 2)
}

func TestFiles_PutChunkStoresAtTheAuthorizedKey(t *testing.T) {
	repo := newFakeStoredRepo()
	pendingUpload(repo, 5)
	blob := newFakeBlob()
	svc := newFileSvc(repo, blob)

	if err := svc.PutChunk(saCtx(t, saEmail), domain.StoredAttachmentWorkItem, saAttID, 0, 5, strings.NewReader("hello")); err != nil {
		t.Fatalf("PutChunk: %v", err)
	}
	if got := string(blob.staged[workItemKey][0]); got != "hello" {
		t.Errorf("staged at %q = %q, want hello (the key is the service's own, never the caller's)", workItemKey, got)
	}

	chunks, err := svc.ListChunks(saCtx(t, saEmail), domain.StoredAttachmentWorkItem, saAttID)
	if err != nil || len(chunks.Chunks) != 1 || chunks.Chunks[0].Size != 5 || chunks.ChunkBytes != UploadChunkBytes {
		t.Errorf("ListChunks = %+v, %v", chunks, err)
	}
}

func TestFiles_PutChunkRefusals(t *testing.T) {
	put := func(repo *fakeStoredRepo, blob FileStore, email string, index int, size int64) error {
		return newFileSvc(repo, blob).PutChunk(saCtx(t, email), domain.StoredAttachmentWorkItem, saAttID, index, size, strings.NewReader("x"))
	}
	t.Run("someone else's upload is not found", func(t *testing.T) {
		repo := newFakeStoredRepo()
		pendingUpload(repo, 5)
		wantErr[*apierror.NotFoundError](t, put(repo, newFakeBlob(), "other@example.com", 0, 1), "another user")
	})
	t.Run("an upload whose item was closed stops", func(t *testing.T) {
		repo := newFakeStoredRepo()
		pendingUpload(repo, 5)
		repo.parent.Closed = true
		wantErr[*apierror.ConflictError](t, put(repo, newFakeBlob(), saEmail, 0, 1), "a closed item")
	})
	t.Run("a chunk index past the largest file is refused", func(t *testing.T) {
		repo := newFakeStoredRepo()
		pendingUpload(repo, 5)
		wantErr[*apierror.ValidationError](t, put(repo, newFakeBlob(), saEmail, 1, 1), "index beyond the limit")
	})
	t.Run("a chunk with no length or too long is refused", func(t *testing.T) {
		repo := newFakeStoredRepo()
		pendingUpload(repo, 5)
		wantErr[*apierror.ValidationError](t, put(repo, newFakeBlob(), saEmail, 0, 0), "no length")
		wantErr[*apierror.ValidationError](t, put(repo, newFakeBlob(), saEmail, 0, UploadChunkBytes+1), "over a chunk")
		// One byte more than the whole file may hold (the limit is 1000).
		wantErr[*apierror.ValidationError](t, put(repo, newFakeBlob(), saEmail, 0, 1001), "past the file limit")
	})
	t.Run("no storage configured is a 503", func(t *testing.T) {
		repo := newFakeStoredRepo()
		pendingUpload(repo, 5)
		err := newFileSvc(repo, nil).PutChunk(saCtx(t, saEmail), domain.StoredAttachmentWorkItem, saAttID, 0, 1, strings.NewReader("x"))
		wantErr[*apierror.ServiceUnavailableError](t, err, "no store")
	})
	t.Run("a storage failure says nothing about where the file is", func(t *testing.T) {
		repo := newFakeStoredRepo()
		pendingUpload(repo, 5)
		blob := newFakeBlob()
		blob.putErr = errors.New("https://acct.blob.core.windows.net/c/secret/path: 500")
		err := put(repo, blob, saEmail, 0, 1)
		var sue *apierror.ServiceUnavailableError
		if !errors.As(err, &sue) || strings.Contains(sue.Msg, "blob.core") || strings.Contains(sue.Msg, "secret") {
			t.Errorf("err = %v, want a 503 that names no storage detail", err)
		}
	})
}

func TestFiles_TransfersAreBounded(t *testing.T) {
	repo := newFakeStoredRepo()
	pendingUpload(repo, 5)
	blob := newFakeBlob()
	svc := NewStoredAttachmentFileService(newBackendSvc(repo), blob, 1)
	fs := svc.(*storedAttachmentFileService)
	release, err := fs.acquire()
	if err != nil {
		t.Fatal(err)
	}
	err = svc.PutChunk(saCtx(t, saEmail), domain.StoredAttachmentWorkItem, saAttID, 0, 1, strings.NewReader("x"))
	wantErr[*apierror.TooManyRequestsError](t, err, "every slot taken")
	release()
	if err := svc.PutChunk(saCtx(t, saEmail), domain.StoredAttachmentWorkItem, saAttID, 0, 1, strings.NewReader("x")); err != nil {
		t.Errorf("after a slot is free: %v", err)
	}
}

func TestFiles_CompleteUpload(t *testing.T) {
	stage := func(blob *fakeBlob, data string) {
		blob.staged[workItemKey] = map[int][]byte{0: []byte(data)}
	}
	t.Run("assembles the chunks, reads the real size and confirms", func(t *testing.T) {
		repo := newFakeStoredRepo()
		pendingUpload(repo, 5)
		blob := newFakeBlob()
		stage(blob, "hello")
		resp, err := newFileSvc(repo, blob).CompleteUpload(saCtx(t, saEmail), domain.CompleteAttachmentUploadRequest{Kind: domain.StoredAttachmentWorkItem, ID: saAttID, Chunks: 1})
		if err != nil || resp.Status != "COMPLETE" || resp.SizeBytes != 5 {
			t.Fatalf("CompleteUpload = %+v, %v", resp, err)
		}
		if repo.completed[saAttID] != workItemKey || string(blob.committed[workItemKey]) != "hello" {
			t.Errorf("completed = %v, committed = %q", repo.completed, blob.committed[workItemKey])
		}
		if len(blob.deleted) != 0 {
			t.Errorf("a good file was removed: %v", blob.deleted)
		}
	})
	t.Run("a file that is not the size declared is refused and removed", func(t *testing.T) {
		repo := newFakeStoredRepo()
		pendingUpload(repo, 5)
		blob := newFakeBlob()
		stage(blob, "hello world")
		_, err := newFileSvc(repo, blob).CompleteUpload(saCtx(t, saEmail), domain.CompleteAttachmentUploadRequest{Kind: domain.StoredAttachmentWorkItem, ID: saAttID, Chunks: 1})
		wantErr[*apierror.ValidationError](t, err, "a size mismatch")
		if len(blob.deleted) != 1 || blob.deleted[0] != workItemKey {
			t.Errorf("deleted = %v, want the refused file removed", blob.deleted)
		}
		if repo.failed[saAttID] == "" {
			t.Error("a refused upload must be recorded FAILED")
		}
	})
	t.Run("a confirm repeated after it succeeded answers the same", func(t *testing.T) {
		repo := newFakeStoredRepo()
		pendingUpload(repo, 5)
		blob := newFakeBlob()
		stage(blob, "hello")
		svc := newFileSvc(repo, blob)
		req := domain.CompleteAttachmentUploadRequest{Kind: domain.StoredAttachmentWorkItem, ID: saAttID, Chunks: 1}
		first, err := svc.CompleteUpload(saCtx(t, saEmail), req)
		if err != nil {
			t.Fatalf("first confirm: %v", err)
		}
		// The row is COMPLETE now, as the database would have it.
		key := workItemKey
		repo.rows[saAttID] = completeRow("COMPLETE", &key)
		again, err := svc.CompleteUpload(saCtx(t, saEmail), req)
		if err != nil || again.ID != first.ID || again.Status != "COMPLETE" {
			t.Errorf("repeated confirm = %+v, %v; want %+v", again, err, first)
		}
		if len(blob.deleted) != 0 {
			t.Errorf("a good file was removed: %v", blob.deleted)
		}
	})
	t.Run("a refusal that may pass keeps the file for a retry", func(t *testing.T) {
		repo := newFakeStoredRepo()
		pendingUpload(repo, 5)
		repo.completeOK = false // another confirm got there first
		blob := newFakeBlob()
		stage(blob, "hello")
		_, err := newFileSvc(repo, blob).CompleteUpload(saCtx(t, saEmail), domain.CompleteAttachmentUploadRequest{Kind: domain.StoredAttachmentWorkItem, ID: saAttID, Chunks: 1})
		wantErr[*apierror.ConflictError](t, err, "a concurrent confirm")
		if len(blob.deleted) != 0 {
			t.Errorf("deleted = %v, the file must stay: the other confirm needs it", blob.deleted)
		}
	})
	t.Run("a missing chunk is a conflict and nothing is confirmed", func(t *testing.T) {
		repo := newFakeStoredRepo()
		pendingUpload(repo, 5)
		blob := newFakeBlob()
		_, err := newFileSvc(repo, blob).CompleteUpload(saCtx(t, saEmail), domain.CompleteAttachmentUploadRequest{Kind: domain.StoredAttachmentWorkItem, ID: saAttID, Chunks: 1})
		wantErr[*apierror.ConflictError](t, err, "chunks not uploaded")
		if len(repo.completed) != 0 {
			t.Errorf("completed = %v", repo.completed)
		}
	})
	t.Run("a chunk count out of range is refused", func(t *testing.T) {
		repo := newFakeStoredRepo()
		pendingUpload(repo, 5)
		for _, n := range []int{0, 2} {
			_, err := newFileSvc(repo, newFakeBlob()).CompleteUpload(saCtx(t, saEmail), domain.CompleteAttachmentUploadRequest{Kind: domain.StoredAttachmentWorkItem, ID: saAttID, Chunks: n})
			wantErr[*apierror.ValidationError](t, err, "chunks out of range")
		}
	})
}

func TestFiles_OpenAttachment(t *testing.T) {
	key := workItemKey
	storedRow := func(repo *fakeStoredRepo) {
		repo.rows[saAttID] = completeRow("COMPLETE", &key)
	}
	t.Run("streams a stored file", func(t *testing.T) {
		repo := newFakeStoredRepo()
		storedRow(repo)
		blob := newFakeBlob()
		blob.committed[key], blob.types[key] = []byte("0123456789"), "application/pdf"
		f, err := newFileSvc(repo, blob).OpenAttachment(saCtx(t, saEmail), domain.StoredAttachmentWorkItem, saAttID, "")
		if err != nil {
			t.Fatalf("OpenAttachment: %v", err)
		}
		defer f.Object.Body.Close()
		if b, _ := io.ReadAll(f.Object.Body); string(b) != "0123456789" || f.Name != "report.pdf" {
			t.Errorf("body = %q name = %q", b, f.Name)
		}
	})
	t.Run("a range reads only those bytes", func(t *testing.T) {
		repo := newFakeStoredRepo()
		storedRow(repo)
		blob := newFakeBlob()
		blob.committed[key] = []byte("0123456789")
		f, err := newFileSvc(repo, blob).OpenAttachment(saCtx(t, saEmail), domain.StoredAttachmentWorkItem, saAttID, "bytes=2-4")
		if err != nil {
			t.Fatalf("OpenAttachment: %v", err)
		}
		defer f.Object.Body.Close()
		if b, _ := io.ReadAll(f.Object.Body); string(b) != "234" || !f.Object.Partial {
			t.Errorf("body = %q partial = %v, want 234", b, f.Object.Partial)
		}
	})
	t.Run("a range past the end is a 416 carrying the size", func(t *testing.T) {
		repo := newFakeStoredRepo()
		storedRow(repo)
		blob := newFakeBlob()
		blob.committed[key] = []byte("0123456789")
		_, err := newFileSvc(repo, blob).OpenAttachment(saCtx(t, saEmail), domain.StoredAttachmentWorkItem, saAttID, "bytes=50-")
		var rng *RangeNotSatisfiableError
		if !errors.As(err, &rng) || rng.Size != 10 {
			t.Errorf("err = %v, want RangeNotSatisfiableError{10}", err)
		}
	})
	t.Run("a deleted attachment is gone, one not stored is not found", func(t *testing.T) {
		repo := newFakeStoredRepo()
		repo.rows[saAttID] = completeRow("DELETED", &key)
		_, err := newFileSvc(repo, newFakeBlob()).OpenAttachment(saCtx(t, saEmail), domain.StoredAttachmentWorkItem, saAttID, "")
		var ge *apierror.GoneError
		if !errors.As(err, &ge) || ge.Code != apierror.CodeAttachmentRemoved {
			t.Errorf("err = %v, want 410 attachment_removed", err)
		}
		repo.rows[saAttID] = completeRow("PENDING", nil)
		_, err = newFileSvc(repo, newFakeBlob()).OpenAttachment(saCtx(t, saEmail), domain.StoredAttachmentWorkItem, saAttID, "")
		wantErr[*apierror.NotFoundError](t, err, "not stored")
	})
	t.Run("the transfer slot is given back when the file is closed", func(t *testing.T) {
		repo := newFakeStoredRepo()
		storedRow(repo)
		blob := newFakeBlob()
		blob.committed[key] = []byte("x")
		svc := NewStoredAttachmentFileService(newBackendSvc(repo), blob, 1)
		f, err := svc.OpenAttachment(saCtx(t, saEmail), domain.StoredAttachmentWorkItem, saAttID, "")
		if err != nil {
			t.Fatal(err)
		}
		_, err = svc.OpenAttachment(saCtx(t, saEmail), domain.StoredAttachmentWorkItem, saAttID, "")
		wantErr[*apierror.TooManyRequestsError](t, err, "the only slot is held by the open file")
		_ = f.Object.Body.Close()
		f2, err := svc.OpenAttachment(saCtx(t, saEmail), domain.StoredAttachmentWorkItem, saAttID, "")
		if err != nil {
			t.Fatalf("after close: %v", err)
		}
		_ = f2.Object.Body.Close()
	})
}

func TestFiles_OpenInlineImage(t *testing.T) {
	png := "image/png"
	key := "csm/projects/prj_9/workitems/cs0001/inline_attachments/cia1_pic.png"
	setup := func() (*fakeStoredRepo, *fakeBlob) {
		repo := newFakeStoredRepo()
		repo.rows[saAttID] = domain.StoredAttachmentRow{
			ID: saAttID, Number: "CIA1", Name: "pic.png", ContentType: &png, Status: "COMPLETE",
			StorageKey: &key, ParentID: saParentID,
		}
		blob := newFakeBlob()
		blob.committed[key] = []byte("png-bytes")
		return repo, blob
	}
	t.Run("an image of the record the email is about is returned", func(t *testing.T) {
		repo, blob := setup()
		f, err := newFileSvc(repo, blob).OpenInlineImage(saCtx(t, saEmail), saAttID, saParentID)
		if err != nil {
			t.Fatalf("OpenInlineImage: %v", err)
		}
		defer f.Object.Body.Close()
		if b, _ := io.ReadAll(f.Object.Body); string(b) != "png-bytes" || f.ContentType != "image/png" {
			t.Errorf("body = %q type = %q", b, f.ContentType)
		}
	})
	t.Run("an image of another record is not found, as one that is not there", func(t *testing.T) {
		repo, blob := setup()
		_, err := newFileSvc(repo, blob).OpenInlineImage(saCtx(t, saEmail), saAttID, "a1000000-0000-0000-0000-0000000000ff")
		wantErr[*apierror.NotFoundError](t, err, "another record's image")
		if len(blob.opened) != 0 {
			t.Errorf("the file was read (%v) before the record was checked", blob.opened)
		}
	})
	t.Run("a draft is never returned for a record", func(t *testing.T) {
		repo, blob := setup()
		row := repo.rows[saAttID]
		row.Draft = true
		repo.rows[saAttID] = row
		_, err := newFileSvc(repo, blob).OpenInlineImage(saCtx(t, saEmail), saAttID, saParentID)
		wantErr[*apierror.NotFoundError](t, err, "a draft image")
	})
	t.Run("a browser read names no record and is not filtered by one", func(t *testing.T) {
		repo, blob := setup()
		f, err := newFileSvc(repo, blob).OpenInlineImage(saCtx(t, saEmail), saAttID, "")
		if err != nil {
			t.Fatalf("OpenInlineImage: %v", err)
		}
		_ = f.Object.Body.Close()
	})
	t.Run("a description image still in drafts is moved to its work item and read from there", func(t *testing.T) {
		repo, blob := setup()
		drafts := "csm/projects/prj_9/workitems/drafts/inline_attachments/cia1_pic.png"
		row := repo.rows[saAttID]
		row.StorageKey = &drafts
		repo.rows[saAttID] = row
		blob.committed[drafts] = []byte("png-bytes")
		f, err := newFileSvc(repo, blob).OpenInlineImage(saCtx(t, saEmail), saAttID, "")
		if err != nil {
			t.Fatalf("OpenInlineImage: %v", err)
		}
		_ = f.Object.Body.Close()
		if string(blob.committed[key]) != "png-bytes" {
			t.Errorf("the copy at %q = %q", key, blob.committed[key])
		}
		if len(repo.keyUpdates) != 1 || repo.keyUpdates[0].to != key || !repo.keyUpdates[0].sys {
			t.Errorf("key updates = %+v, want the row pointed at the new key as the system", repo.keyUpdates)
		}
		if _, still := blob.committed[drafts]; still {
			t.Error("the old copy in drafts must be removed once the new key is recorded")
		}
	})
	t.Run("a failed move still shows the image from where it is", func(t *testing.T) {
		repo, blob := setup()
		drafts := "csm/projects/prj_9/workitems/drafts/inline_attachments/cia1_pic.png"
		row := repo.rows[saAttID]
		row.StorageKey = &drafts
		repo.rows[saAttID] = row
		blob.committed[drafts] = []byte("png-bytes")
		blob.putErr = errors.New("storage down")
		f, err := newFileSvc(repo, blob).OpenInlineImage(saCtx(t, saEmail), saAttID, "")
		if err != nil {
			t.Fatalf("OpenInlineImage: %v, want the image shown from the drafts folder", err)
		}
		defer f.Object.Body.Close()
		if b, _ := io.ReadAll(f.Object.Body); string(b) != "png-bytes" {
			t.Errorf("body = %q", b)
		}
		if len(repo.keyUpdates) != 0 {
			t.Errorf("key updates = %+v, none must be recorded for a failed move", repo.keyUpdates)
		}
	})
}

func TestFiles_DeleteRemovesTheRowThenTheFile(t *testing.T) {
	repo := newFakeStoredRepo()
	key := workItemKey
	repo.rows[saAttID] = completeRow("COMPLETE", &key)
	blob := newFakeBlob()
	blob.committed[key] = []byte("x")
	err := newFileSvc(repo, blob).DeleteAttachment(saCtx(t, saEmail), domain.DeleteStoredAttachmentRequest{Kind: domain.StoredAttachmentWorkItem, ID: saAttID})
	if err != nil {
		t.Fatalf("DeleteAttachment: %v", err)
	}
	if repo.deleted[saAttID] == "" {
		t.Error("the row must be marked DELETED")
	}
	if len(blob.deleted) != 1 || blob.deleted[0] != key {
		t.Errorf("deleted files = %v, want %q", blob.deleted, key)
	}

	// Someone who may not delete leaves the file alone.
	repo2 := newFakeStoredRepo()
	repo2.rows[saAttID] = completeRow("COMPLETE", &key)
	blob2 := newFakeBlob()
	blob2.committed[key] = []byte("x")
	err = newFileSvc(repo2, blob2).DeleteAttachment(saCtx(t, "other@example.com"), domain.DeleteStoredAttachmentRequest{Kind: domain.StoredAttachmentWorkItem, ID: saAttID})
	if err == nil || len(blob2.deleted) != 0 {
		t.Errorf("err = %v, deleted = %v: a refused delete must keep the file", err, blob2.deleted)
	}
}

func TestParseRange(t *testing.T) {
	for name, tc := range map[string]struct {
		header      string
		off, length int64
		ok, bad     bool
	}{
		"from to":           {"bytes=2-4", 2, 3, true, false},
		"open ended":        {"bytes=5-", 5, 5, true, false},
		"suffix":            {"bytes=-3", 7, 3, true, false},
		"end past the file": {"bytes=8-99", 8, 2, true, false},
		"start past end":    {"bytes=10-", 0, 0, false, true},
		"two ranges":        {"bytes=0-1,3-4", 0, 0, false, false},
		"not bytes":         {"items=0-1", 0, 0, false, false},
		"backwards":         {"bytes=5-2", 0, 0, false, false},
	} {
		rng, ok, bad := parseRange(tc.header, 10)
		if ok != tc.ok || bad != tc.bad || (ok && (rng.Offset != tc.off || rng.Length != tc.length)) {
			t.Errorf("%s: parseRange(%q) = %+v ok=%v bad=%v, want off=%d len=%d ok=%v bad=%v", name, tc.header, rng, ok, bad, tc.off, tc.length, tc.ok, tc.bad)
		}
	}
}
