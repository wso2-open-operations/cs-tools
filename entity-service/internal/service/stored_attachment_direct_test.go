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
	return NewStoredAttachmentFileService(newDirectLimitsSvc(repo, links), blob, 2, WithDirectTransfers(links, directMinMB))
}

// newDirectLimitsSvc is the stored attachment service under the limits routes.go
// gives it: 100 MiB for a file sent through the service, and, only when links can
// be signed, 1 GiB for one sent straight to Azure.
func newDirectLimitsSvc(repo *fakeStoredRepo, links LinkStore) *storedAttachmentService {
	limits := StoredAttachmentLimits{MaxBytes: 100 * mib, InlineTotalMaxBytes: 100}
	if links != nil && links.CanSignLinks() {
		limits.DirectMinBytes, limits.DirectMaxBytes = directMinMB, 1024*mib
	}
	svc := NewStoredAttachmentService(repo, limits).(*storedAttachmentService)
	svc.now = func() time.Time { return saNow }
	svc.shares.now = func() time.Time { return saNow }
	return svc
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
		if links.uploadKey != workItemKey+stagingSuffix || links.uploadCount != 3 || links.uploadTTL != directLinkTTL(20*mib) {
			t.Errorf("signed %q x%d for %v", links.uploadKey, links.uploadCount, links.uploadTTL)
		}
		if !got.ExpiresAt.Equal(saNow.Add(directLinkTTL(20 * mib))) {
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
		pendingUpload(repo, 2048*mib) // above the 1 GiB direct cap
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
		if got.Name != "report.pdf" || got.URL == "" || !got.ExpiresAt.Equal(saNow.Add(directLinkTTL(500))) {
			t.Errorf("link = %+v", got)
		}
		// Whatever type the row declares, the link answers as an opaque download.
		if links.downloadKey != key || links.downloadName != "report.pdf" || links.downloadType != "application/octet-stream" || links.downloadTTL != directLinkTTL(500) {
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

// TestDirectLinkTTL: the lifetime of a link grows with the size of the file.
func TestDirectLinkTTL(t *testing.T) {
	for size, want := range map[int64]time.Duration{
		1:          5 * time.Minute,
		50 * mib:   5 * time.Minute,
		50*mib + 1: 7 * time.Minute,
		100 * mib:  7 * time.Minute,
		250 * mib:  10 * time.Minute,
		500 * mib:  15 * time.Minute,
		1024 * mib: 20 * time.Minute,
	} {
		if got := directLinkTTL(size); got != want {
			t.Errorf("directLinkTTL(%d MiB) = %v, want %v", size/mib, got, want)
		}
	}
}

// A file sent straight to Azure may be larger than one sent through the service,
// up to the direct cap; nothing else gets the larger cap.
func TestDirectUploadLimits(t *testing.T) {
	start := func(links LinkStore, kind domain.StoredAttachmentKind, ct string, size int64) (domain.InitiateAttachmentUploadResponse, error) {
		return newDirectLimitsSvc(newFakeStoredRepo(), links).InitiateUpload(saCtx(t, saEmail),
			domain.InitiateAttachmentUploadRequest{Kind: kind, ParentID: saParentID, Name: "big.bin", ContentType: ct, SizeBytes: size})
	}
	work := domain.StoredAttachmentWorkItem
	t.Run("a large file is accepted up to the direct cap, which the answer reports", func(t *testing.T) {
		resp, err := start(&fakeLinks{}, work, "application/zip", 600*mib)
		if err != nil || resp.MaxBytes != 1024*mib {
			t.Fatalf("600 MiB: %+v, %v", resp, err)
		}
		if _, err := start(&fakeLinks{}, work, "application/zip", 1024*mib); err != nil {
			t.Errorf("exactly 1 GiB refused: %v", err)
		}
	})
	t.Run("a file over the direct cap is refused", func(t *testing.T) {
		_, err := start(&fakeLinks{}, work, "application/zip", 1024*mib+1)
		wantErr[*apierror.ValidationError](t, err, "over 1 GiB")
	})
	t.Run("a file that fits the ordinary cap keeps it", func(t *testing.T) {
		resp, err := start(&fakeLinks{}, work, "application/zip", 60*mib)
		if err != nil || resp.MaxBytes != 100*mib {
			t.Errorf("60 MiB: %+v, %v", resp, err)
		}
	})
	t.Run("without direct transfers the ordinary cap stands", func(t *testing.T) {
		for name, links := range map[string]LinkStore{"off": nil, "cannot sign": &fakeLinks{cannotSign: true}} {
			_, err := start(links, work, "application/zip", 600*mib)
			wantErr[*apierror.ValidationError](t, err, "600 MiB with direct "+name)
		}
	})
	t.Run("an inline image never gets the larger cap", func(t *testing.T) {
		_, err := start(&fakeLinks{}, domain.StoredAttachmentComment, "image/png", 600*mib)
		wantErr[*apierror.ValidationError](t, err, "a 600 MiB image")
	})
	t.Run("a large upload gets a link for every chunk, valid for its size", func(t *testing.T) {
		repo := newFakeStoredRepo()
		pendingUpload(repo, 600*mib)
		links := &fakeLinks{}
		got, err := newDirectSvc(repo, links).DirectUploadLinks(saCtx(t, saEmail), work, saAttID)
		if err != nil || len(got.Links) != 75 { // 600 MiB in 8 MiB chunks
			t.Fatalf("got %d links, %v", len(got.Links), err)
		}
		if links.uploadTTL != 20*time.Minute {
			t.Errorf("ttl = %v, want 20m for 600 MiB", links.uploadTTL)
		}
	})
	t.Run("the real size is held to the cap that applied when the upload started", func(t *testing.T) {
		repo := newFakeStoredRepo()
		pendingUpload(repo, 600*mib)
		svc := newDirectLimitsSvc(repo, &fakeLinks{})
		size := int64(600 * mib)
		if _, err := svc.ConfirmUpload(saCtx(t, saEmail), domain.ConfirmAttachmentUploadRequest{Kind: work, ID: saAttID, SizeBytes: &size, ContentType: "application/pdf"}); err != nil {
			t.Errorf("confirm of an accepted 600 MiB file: %v", err)
		}
		// With direct transfers off the same row is refused: it exceeds the ordinary cap.
		repo2 := newFakeStoredRepo()
		pendingUpload(repo2, 600*mib)
		_, err := newDirectLimitsSvc(repo2, nil).ConfirmUpload(saCtx(t, saEmail), domain.ConfirmAttachmentUploadRequest{Kind: work, ID: saAttID, SizeBytes: &size, ContentType: "application/pdf"})
		wantErr[*apierror.ValidationError](t, err, "confirm without direct transfers")
	})
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

// TestPendingLargeUploads: a caller can only have a few uploads that may go
// straight to storage waiting at once, which bounds what they can leave there
// unconfirmed.
func TestPendingLargeUploads(t *testing.T) {
	start := func(repo *fakeStoredRepo, links LinkStore, size int64) error {
		_, err := newDirectLimitsSvc(repo, links).InitiateUpload(saCtx(t, saEmail), domain.InitiateAttachmentUploadRequest{
			Kind: domain.StoredAttachmentWorkItem, ParentID: saParentID, Name: "big.bin", ContentType: "application/zip", SizeBytes: size,
		})
		return err
	}
	t.Run("the next large upload is refused at the limit", func(t *testing.T) {
		repo := newFakeStoredRepo()
		repo.pendingLarge = maxPendingLargeUploads
		wantErr[*apierror.TooManyRequestsError](t, start(repo, &fakeLinks{}, 20*mib), "a caller at the limit")
	})
	t.Run("one below the limit is accepted, and the count is of recent large uploads by the caller", func(t *testing.T) {
		repo := newFakeStoredRepo()
		repo.pendingLarge = maxPendingLargeUploads - 1
		if err := start(repo, &fakeLinks{}, 20*mib); err != nil {
			t.Fatalf("below the limit: %v", err)
		}
		if repo.pendingAsked.email != saEmail || repo.pendingAsked.minBytes != directMinMB ||
			!repo.pendingAsked.since.Equal(saNow.Add(-pendingLargeUploadWindow)) {
			t.Errorf("counted %+v", repo.pendingAsked)
		}
	})
	t.Run("a small upload is never counted", func(t *testing.T) {
		repo := newFakeStoredRepo()
		repo.pendingLarge = 1000
		if err := start(repo, &fakeLinks{}, 5*mib); err != nil {
			t.Fatalf("small upload: %v", err)
		}
		if repo.pendingAsked.email != "" {
			t.Error("a small upload was counted")
		}
	})
	t.Run("nothing is counted while direct transfers are off", func(t *testing.T) {
		repo := newFakeStoredRepo()
		repo.pendingLarge = 1000
		if err := start(repo, nil, 20*mib); err != nil {
			t.Fatalf("direct off: %v", err)
		}
		if repo.pendingAsked.email != "" {
			t.Error("an upload was counted with direct transfers off")
		}
	})
}
