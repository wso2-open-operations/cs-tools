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

package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// fakeLinks is a LinkStore that records what it was asked to sign.
type fakeLinks struct {
	cannotSign   bool
	err          error
	uploadKey    string
	uploadCount  int
	uploadTTL    time.Duration
	downloadKey  string
	downloadName string
	downloadType string
	downloadTTL  time.Duration
}

// CanSignLinks reports whether this fake signs, unless told it cannot.
func (f *fakeLinks) CanSignLinks() bool { return !f.cannotSign }

// UploadLinks records the request and answers with one link per chunk.
func (f *fakeLinks) UploadLinks(key string, count int, ttl time.Duration) ([]string, time.Time, error) {
	f.uploadKey, f.uploadCount, f.uploadTTL = key, count, ttl
	links := make([]string, count)
	for i := range links {
		links[i] = "https://blob.example/" + key + "?sig=x&block=" + string(rune('0'+i))
	}
	return links, saNow.Add(ttl), f.err
}

// DownloadLink records the request and answers with a link.
func (f *fakeLinks) DownloadLink(key, name, contentType string, ttl time.Duration) (string, time.Time, error) {
	f.downloadKey, f.downloadName, f.downloadType, f.downloadTTL = key, name, contentType, ttl
	return "https://blob.example/" + key + "?sig=y", saNow.Add(ttl), f.err
}

const (
	mib         = 1 << 20
	directMinMB = 8 * mib
)

// newDirectSvc builds a file service that allows files up to 100 MiB and sends
// those above 8 MiB direct, when links can sign.
func newDirectSvc(repo *fakeStoredRepo, links LinkStore) StoredAttachmentFileService {
	return newDirectSvcWith(repo, links, newFakeBlob())
}

// newDirectSvcWith is newDirectSvc over the given blob store.
func newDirectSvcWith(repo *fakeStoredRepo, links LinkStore, blob FileStore) StoredAttachmentFileService {
	svc := NewStoredAttachmentService(repo, StoredAttachmentLimits{MaxBytes: 100 * mib, InlineTotalMaxBytes: 100}).(*storedAttachmentService)
	svc.now = func() time.Time { return saNow }
	svc.shares.now = func() time.Time { return saNow }
	return NewStoredAttachmentFileService(svc, blob, 2, WithDirectTransfers(links, directMinMB))
}

// TestDirectUploadAllowed: only a file above the threshold goes direct, and only when links can be signed.
func TestDirectUploadAllowed(t *testing.T) {
	repo := newFakeStoredRepo()
	on := newDirectSvc(repo, &fakeLinks{})
	for size, want := range map[int64]bool{1: false, directMinMB: false, directMinMB + 1: true, 100 * mib: true} {
		if got := on.DirectUploadAllowed(size); got != want {
			t.Errorf("DirectUploadAllowed(%d) = %v, want %v", size, got, want)
		}
	}
	// Off when no link store is given, or when it cannot sign (an Entra app).
	for name, links := range map[string]LinkStore{"none": nil, "cannot sign": &fakeLinks{cannotSign: true}} {
		if newDirectSvc(repo, links).DirectUploadAllowed(100 * mib) {
			t.Errorf("%s: direct upload is allowed", name)
		}
	}
}

// TestDirectUploadLinks: the access checks of a chunk, made once, then one link per chunk for the upload's own key.
func TestDirectUploadLinks(t *testing.T) {
	t.Run("one link per chunk, signed for the upload's own key", func(t *testing.T) {
		repo := newFakeStoredRepo()
		pendingUpload(repo, 20*mib) // 8 + 8 + 4 MiB: 3 chunks
		links := &fakeLinks{}
		got, err := newDirectSvc(repo, links).DirectUploadLinks(saCtx(t, saEmail), domain.StoredAttachmentWorkItem, saAttID)
		if err != nil {
			t.Fatalf("DirectUploadLinks: %v", err)
		}
		if len(got.Links) != 3 || got.ChunkBytes != UploadChunkBytes {
			t.Errorf("got %d links, chunkBytes %d", len(got.Links), got.ChunkBytes)
		}
		if links.uploadKey != workItemKey+stagingSuffix || links.uploadCount != 3 || links.uploadTTL != directUploadTTL {
			t.Errorf("signed %q x%d for %v", links.uploadKey, links.uploadCount, links.uploadTTL)
		}
		if !got.ExpiresAt.Equal(saNow.Add(directUploadTTL)) {
			t.Errorf("expiresAt = %v", got.ExpiresAt)
		}
	})
	t.Run("a small file keeps going through the service", func(t *testing.T) {
		repo := newFakeStoredRepo()
		pendingUpload(repo, 5*mib)
		links := &fakeLinks{}
		_, err := newDirectSvc(repo, links).DirectUploadLinks(saCtx(t, saEmail), domain.StoredAttachmentWorkItem, saAttID)
		wantErr[*apierror.ValidationError](t, err, "a small file")
		if links.uploadCount != 0 {
			t.Error("a link was signed for a small file")
		}
	})
	t.Run("a file larger than allowed gets no links", func(t *testing.T) {
		repo := newFakeStoredRepo()
		pendingUpload(repo, 200*mib)
		_, err := newDirectSvc(repo, &fakeLinks{}).DirectUploadLinks(saCtx(t, saEmail), domain.StoredAttachmentWorkItem, saAttID)
		wantErr[*apierror.ValidationError](t, err, "a file over the limit")
	})
	t.Run("only the uploader gets links", func(t *testing.T) {
		repo := newFakeStoredRepo()
		pendingUpload(repo, 20*mib)
		links := &fakeLinks{}
		_, err := newDirectSvc(repo, links).DirectUploadLinks(saCtx(t, "someone-else@example.com"), domain.StoredAttachmentWorkItem, saAttID)
		wantErr[*apierror.NotFoundError](t, err, "another user")
		if links.uploadCount != 0 {
			t.Error("a link was signed for someone else's upload")
		}
	})
	t.Run("a finished upload gets no links", func(t *testing.T) {
		repo := newFakeStoredRepo()
		key := workItemKey
		repo.rows[saAttID] = completeRow("COMPLETE", &key)
		_, err := newDirectSvc(repo, &fakeLinks{}).DirectUploadLinks(saCtx(t, saEmail), domain.StoredAttachmentWorkItem, saAttID)
		wantErr[*apierror.ConflictError](t, err, "a confirmed upload")
	})
	t.Run("off when direct transfers are not configured", func(t *testing.T) {
		repo := newFakeStoredRepo()
		pendingUpload(repo, 20*mib)
		_, err := newDirectSvc(repo, nil).DirectUploadLinks(saCtx(t, saEmail), domain.StoredAttachmentWorkItem, saAttID)
		wantErr[*apierror.ValidationError](t, err, "direct transfers off")
	})
	t.Run("a signing failure says nothing about storage", func(t *testing.T) {
		repo := newFakeStoredRepo()
		pendingUpload(repo, 20*mib)
		_, err := newDirectSvc(repo, &fakeLinks{err: errors.New("account key secret detail")}).DirectUploadLinks(saCtx(t, saEmail), domain.StoredAttachmentWorkItem, saAttID)
		wantErr[*apierror.ServiceUnavailableError](t, err, "a signing failure")
		if err != nil && strings.Contains(err.Error(), "secret") {
			t.Errorf("error leaks detail: %v", err)
		}
	})
}

// TestDirectDownloadLink: a stored file the caller may see gets a link for its own name, and nothing else does.
func TestDirectDownloadLink(t *testing.T) {
	key := workItemKey
	t.Run("a stored file gets a download link for its own name", func(t *testing.T) {
		repo := newFakeStoredRepo()
		repo.rows[saAttID] = completeRow("COMPLETE", &key)
		links := &fakeLinks{}
		got, err := newDirectSvc(repo, links).DirectDownloadLink(saCtx(t, saEmail), domain.StoredAttachmentWorkItem, saAttID)
		if err != nil {
			t.Fatalf("DirectDownloadLink: %v", err)
		}
		if got.Name != "report.pdf" || got.URL == "" || !got.ExpiresAt.Equal(saNow.Add(directDownloadTTL)) {
			t.Errorf("link = %+v", got)
		}
		// Whatever type the row declares, the link answers as an opaque download.
		if links.downloadKey != key || links.downloadName != "report.pdf" || links.downloadType != "application/octet-stream" || links.downloadTTL != directDownloadTTL {
			t.Errorf("signed %q %q %q %v", links.downloadKey, links.downloadName, links.downloadType, links.downloadTTL)
		}
	})
	t.Run("a deleted attachment is gone", func(t *testing.T) {
		repo := newFakeStoredRepo()
		repo.rows[saAttID] = completeRow("DELETED", &key)
		links := &fakeLinks{}
		_, err := newDirectSvc(repo, links).DirectDownloadLink(saCtx(t, saEmail), domain.StoredAttachmentWorkItem, saAttID)
		wantErr[*apierror.GoneError](t, err, "a deleted attachment")
		if links.downloadKey != "" {
			t.Error("a link was signed for a deleted attachment")
		}
	})
	t.Run("an attachment the caller cannot see gets no link", func(t *testing.T) {
		repo := newFakeStoredRepo()
		links := &fakeLinks{}
		_, err := newDirectSvc(repo, links).DirectDownloadLink(saCtx(t, saEmail), domain.StoredAttachmentWorkItem, saAttID)
		wantErr[*apierror.NotFoundError](t, err, "an unknown attachment")
		if links.downloadKey != "" {
			t.Error("a link was signed for an attachment that is not there")
		}
	})
	t.Run("off when direct transfers are not configured", func(t *testing.T) {
		repo := newFakeStoredRepo()
		repo.rows[saAttID] = completeRow("COMPLETE", &key)
		_, err := newDirectSvc(repo, nil).DirectDownloadLink(saCtx(t, saEmail), domain.StoredAttachmentWorkItem, saAttID)
		wantErr[*apierror.NotFoundError](t, err, "direct transfers off")
	})
}

// storedFile is a blob store holding a committed file of size bytes at key.
func storedFile(key string, size int) *fakeBlob {
	blob := newFakeBlob()
	blob.committed[key] = make([]byte, size)
	return blob
}

// TestStagedUploads: an upload that may be sent direct is put together under a
// staging key and copied to its own key at confirm, so an upload link still valid
// after the confirm cannot change the confirmed file.
func TestStagedUploads(t *testing.T) {
	staging := workItemKey + stagingSuffix
	ctx := func() context.Context { return saCtx(t, saEmail) }
	work := domain.StoredAttachmentWorkItem

	t.Run("a chunk sent through the service lands under the staging key", func(t *testing.T) {
		repo := newFakeStoredRepo()
		pendingUpload(repo, 20*mib)
		blob := newFakeBlob()
		svc := newDirectSvcWith(repo, &fakeLinks{}, blob)
		if err := svc.PutChunk(ctx(), work, saAttID, 0, 5, strings.NewReader("hello")); err != nil {
			t.Fatalf("PutChunk: %v", err)
		}
		if string(blob.staged[staging][0]) != "hello" || blob.staged[workItemKey] != nil {
			t.Errorf("staged = %v: want the chunk under %q only", blob.staged, staging)
		}
		list, err := svc.ListChunks(ctx(), work, saAttID)
		if err != nil || len(list.Chunks) != 1 {
			t.Errorf("ListChunks = %+v, %v: a resumed upload must see the staged chunk", list, err)
		}
	})
	t.Run("a confirmed staged upload ends with the file at its key and no staging file", func(t *testing.T) {
		repo := newFakeStoredRepo()
		pendingUpload(repo, 9*mib) // direct-sized
		blob := newFakeBlob()
		big := strings.Repeat("x", 9*mib)
		blob.staged[staging] = map[int][]byte{0: []byte(big[:8*mib]), 1: []byte(big[8*mib:])}
		svc := newDirectSvcWith(repo, &fakeLinks{}, blob)
		if _, err := svc.CompleteUpload(ctx(), domain.CompleteAttachmentUploadRequest{Kind: work, ID: saAttID, Chunks: 2}); err != nil {
			t.Fatalf("CompleteUpload: %v", err)
		}
		if len(blob.committed[workItemKey]) != 9*mib {
			t.Errorf("the file at its key has %d bytes, want %d", len(blob.committed[workItemKey]), 9*mib)
		}
		if _, still := blob.committed[staging]; still {
			t.Error("the staging file was not removed")
		}
		if len(blob.copied) != 1 || blob.copied[0] != [2]string{staging, workItemKey} {
			t.Errorf("copied = %v", blob.copied)
		}
	})
	t.Run("a refused upload leaves neither file", func(t *testing.T) {
		repo := newFakeStoredRepo()
		pendingUpload(repo, 9*mib)
		blob := newFakeBlob()
		// Bigger than declared: the real size does not match, so confirm refuses it.
		blob.staged[staging] = map[int][]byte{0: []byte(strings.Repeat("x", 8*mib)), 1: []byte(strings.Repeat("x", 2*mib))}
		svc := newDirectSvcWith(repo, &fakeLinks{}, blob)
		_, err := svc.CompleteUpload(ctx(), domain.CompleteAttachmentUploadRequest{Kind: work, ID: saAttID, Chunks: 2})
		wantErr[*apierror.ValidationError](t, err, "a size that does not match")
		if _, ok := blob.committed[workItemKey]; ok {
			t.Error("a refused upload has a file at its key")
		}
		if _, ok := blob.committed[staging]; ok {
			t.Error("a refused upload left its staging file")
		}
	})
	t.Run("a copy that fails can be confirmed again", func(t *testing.T) {
		repo := newFakeStoredRepo()
		pendingUpload(repo, 9*mib)
		blob := newFakeBlob()
		blob.staged[staging] = map[int][]byte{0: []byte(strings.Repeat("x", 8*mib)), 1: []byte(strings.Repeat("x", mib))}
		blob.copyErr = errors.New("storage down")
		svc := newDirectSvcWith(repo, &fakeLinks{}, blob)
		req := domain.CompleteAttachmentUploadRequest{Kind: work, ID: saAttID, Chunks: 2}
		_, err := svc.CompleteUpload(ctx(), req)
		wantErr[*apierror.ServiceUnavailableError](t, err, "a failed copy")
		if _, ok := blob.committed[workItemKey]; ok {
			t.Error("a failed copy left a file at the key")
		}
		blob.copyErr = nil
		if _, err := svc.CompleteUpload(ctx(), req); err != nil {
			t.Fatalf("second confirm: %v", err)
		}
	})
	t.Run("a small upload is put and confirmed at its own key, as before", func(t *testing.T) {
		repo := newFakeStoredRepo()
		pendingUpload(repo, 5)
		blob := newFakeBlob()
		svc := newDirectSvcWith(repo, &fakeLinks{}, blob)
		if err := svc.PutChunk(ctx(), work, saAttID, 0, 5, strings.NewReader("hello")); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.CompleteUpload(ctx(), domain.CompleteAttachmentUploadRequest{Kind: work, ID: saAttID, Chunks: 1}); err != nil {
			t.Fatalf("CompleteUpload: %v", err)
		}
		if string(blob.committed[workItemKey]) != "hello" || len(blob.copied) != 0 {
			t.Errorf("committed = %v, copied = %v", blob.committed, blob.copied)
		}
	})
}
