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
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/filestore"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

const (
	saParentID = "a1000000-0000-0000-0000-000000000001"
	saAttID    = "b1000000-0000-0000-0000-000000000001"
	saEmail    = "ext@example.com"
)

// saCtx is a request context carrying a user token for email, the way the
// UserIDToken middleware leaves it.
func saCtx(t *testing.T, email string) context.Context {
	t.Helper()
	var out context.Context
	h := middleware.UserIDToken(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { out = r.Context() }))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("x-user-id-token", fakeJWTWithEmail(t, email))
	h.ServeHTTP(httptest.NewRecorder(), req)
	return out
}

// fakeStoredRepo is an in-memory StoredAttachmentRepository.
type fakeStoredRepo struct {
	parent    domain.StoredAttachmentParent
	parentErr error
	rows      map[string]domain.StoredAttachmentRow
	// workItemRows are found only by a work item lookup (a migrated comment image).
	workItemRows map[string]domain.StoredAttachmentRow
	inserted     []domain.StoredAttachmentInsert
	insertErr    error

	deleted      map[string]string // id -> deleted_by
	deleteCtx    []bool            // whether each MarkDeleted ran as the system
	deleteOK     bool
	completed    map[string]string // id -> storage key
	completeCtx  []bool            // whether each MarkComplete ran as the system
	completeOK   bool
	failed       map[string]string // id -> detail
	abandoned    []domain.AbandonedUpload
	abandonedAt  time.Time // the cutoff ListAbandoned was asked for
	abandonedSys bool
	linkWant     []string // ids LinkInlineImages will report as linked
	linkCall     struct {
		comment, workItem, email string
		ids                      []string
		sys                      bool
	}
	unlinked []domain.StoredAttachmentRow
	deferred []string // rows sent to the back of the cleanup queue
}

func newFakeStoredRepo() *fakeStoredRepo {
	return &fakeStoredRepo{
		parent:     domain.StoredAttachmentParent{ID: saParentID, Number: "CS0001", ProjectNumber: "PRJ-9"},
		rows:       map[string]domain.StoredAttachmentRow{},
		completed:  map[string]string{},
		deleted:    map[string]string{},
		failed:     map[string]string{},
		completeOK: true,
		deleteOK:   true,
	}
}

func (f *fakeStoredRepo) GetParent(_ context.Context, _ domain.StoredAttachmentKind, _ string) (domain.StoredAttachmentParent, error) {
	return f.parent, f.parentErr
}

func (f *fakeStoredRepo) InsertPending(_ context.Context, _ domain.StoredAttachmentKind, in domain.StoredAttachmentInsert) (domain.StoredAttachmentRow, error) {
	if f.insertErr != nil {
		return domain.StoredAttachmentRow{}, f.insertErr
	}
	f.inserted = append(f.inserted, in)
	size := in.SizeBytes
	ct := in.ContentType
	row := domain.StoredAttachmentRow{
		ID: saAttID, Number: "WIA0000042", Name: in.Name, ContentType: &ct, SizeBytes: &size,
		Status: "PENDING", CreatedBy: in.CreatedBy, ParentID: in.ParentID,
	}
	f.rows[row.ID] = row
	return row, nil
}

func (f *fakeStoredRepo) GetByIDs(_ context.Context, kind domain.StoredAttachmentKind, ids []string) ([]domain.StoredAttachmentRow, error) {
	var out []domain.StoredAttachmentRow
	for _, id := range ids {
		if r, ok := f.rows[id]; ok {
			out = append(out, r)
		} else if r, ok := f.workItemRows[id]; ok && kind == domain.StoredAttachmentWorkItem {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeStoredRepo) MarkComplete(ctx context.Context, _ domain.StoredAttachmentKind, id, key string, _ int64) (bool, error) {
	scope, ok := repository.CallerIdentityFromContext(ctx)
	f.completeCtx = append(f.completeCtx, ok && scope.Unrestricted)
	if !f.completeOK {
		return false, nil
	}
	f.completed[id] = key
	return true, nil
}

func (f *fakeStoredRepo) MarkDeleted(ctx context.Context, _ domain.StoredAttachmentKind, id, by string) (bool, error) {
	scope, ok := repository.CallerIdentityFromContext(ctx)
	f.deleteCtx = append(f.deleteCtx, ok && scope.Unrestricted)
	if !f.deleteOK {
		return false, nil
	}
	f.deleted[id] = by
	return true, nil
}

func (f *fakeStoredRepo) ListAbandoned(ctx context.Context, olderThan time.Time, _ int) ([]domain.AbandonedUpload, error) {
	scope, ok := repository.CallerIdentityFromContext(ctx)
	f.abandonedSys = ok && scope.Unrestricted
	f.abandonedAt = olderThan
	return f.abandoned, nil
}

func (f *fakeStoredRepo) LinkInlineImages(ctx context.Context, commentID, workItemID, email string, ids []string) ([]string, error) {
	scope, ok := repository.CallerIdentityFromContext(ctx)
	f.linkCall.comment, f.linkCall.workItem, f.linkCall.email, f.linkCall.ids = commentID, workItemID, email, ids
	f.linkCall.sys = ok && scope.Unrestricted
	return f.linkWant, nil
}

func (f *fakeStoredRepo) ListUnlinkedImages(context.Context, time.Time, int) ([]domain.StoredAttachmentRow, error) {
	return f.unlinked, nil
}

func (f *fakeStoredRepo) RecordCleanupAttempt(_ context.Context, _ domain.StoredAttachmentKind, id string) error {
	f.deferred = append(f.deferred, id)
	return nil
}

func (f *fakeStoredRepo) MarkFailed(_ context.Context, _ domain.StoredAttachmentKind, id, detail string) error {
	f.failed[id] = detail
	return nil
}

// fakeStore is an in-memory SFTPGo.
type fakeStore struct {
	shares   []filestore.ShareSpec
	shareErr error
	mkdirs   []string
	mkdirErr error
	listing  map[string][]filestore.Entry
	listErr  error
	head     []byte
	renames  [][2]string
	renameEr error
	removed  []string
	files    []string // files removed
	fileErr  error
	openErr  error
	opened   []string
	putErr   error
	// dropListing makes PutFile accept a file without it showing in the folder.
	dropListing bool
	put         []string // dir/name of each file written with PutFile
	putData     map[string][]byte
}

func newFakeStore() *fakeStore { return &fakeStore{listing: map[string][]filestore.Entry{}} }

func (f *fakeStore) CreateShare(_ context.Context, spec filestore.ShareSpec) (string, error) {
	if f.shareErr != nil {
		return "", f.shareErr
	}
	f.shares = append(f.shares, spec)
	return "share-" + spec.Name, nil
}
func (f *fakeStore) PutFile(_ context.Context, dir, name string, data []byte) error {
	if f.putErr != nil {
		return f.putErr
	}
	key := dir + "/" + name
	f.put = append(f.put, key)
	if f.putData == nil {
		f.putData = map[string][]byte{}
	}
	f.putData[key] = data
	if !f.dropListing {
		f.listing[dir] = append(f.listing[dir], filestore.Entry{Name: name, Size: int64(len(data))})
	}
	return nil
}
func (f *fakeStore) MkdirAll(_ context.Context, dir string) error {
	f.mkdirs = append(f.mkdirs, dir)
	return f.mkdirErr
}
func (f *fakeStore) ListDir(_ context.Context, dir string) ([]filestore.Entry, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	e, ok := f.listing[dir]
	if !ok {
		return nil, filestore.ErrNotFound
	}
	return e, nil
}
func (f *fakeStore) Rename(_ context.Context, from, to string) error {
	if f.renameEr != nil {
		return f.renameEr
	}
	f.renames = append(f.renames, [2]string{from, to})
	return nil
}
func (f *fakeStore) RemoveFile(_ context.Context, path string) error {
	if f.fileErr != nil {
		return f.fileErr
	}
	f.files = append(f.files, path)
	return nil
}
func (f *fakeStore) RemoveDir(_ context.Context, dir string) error {
	f.removed = append(f.removed, dir)
	return nil
}
func (f *fakeStore) Open(_ context.Context, p string) (io.ReadCloser, int64, error) {
	if f.openErr != nil {
		return nil, 0, f.openErr
	}
	f.opened = append(f.opened, p)
	return io.NopCloser(strings.NewReader("IMG")), 3, nil
}
func (f *fakeStore) ReadHead(_ context.Context, _ string, _ int) ([]byte, error) { return f.head, nil }
func (f *fakeStore) DownloadURL(id string) string                                { return "https://files.test/dl/" + id }
func (f *fakeStore) UploadURL(id string) string                                  { return "https://files.test/up/" + id }

var saNow = time.Date(2026, 10, 26, 10, 0, 0, 0, time.UTC)

func newStoredSvc(repo *fakeStoredRepo, store *fakeStore) *storedAttachmentService {
	svc := NewStoredAttachmentService(repo, store, StoredAttachmentLimits{
		MaxBytes: 1000, InlineMaxBytes: 100, ReadLinkTTL: 5 * time.Minute, UploadLinkTTL: 15 * time.Minute,
	}).(*storedAttachmentService)
	svc.now = func() time.Time { return saNow }
	return svc
}

func stringPtr(s string) *string { return &s }

// ---- links ----

func TestResolveLinks_StoredFileGetsAOneFileShare(t *testing.T) {
	repo, store := newFakeStoredRepo(), newFakeStore()
	key := "csm/projects/prj_9/workitems/cs0001/attachments/wia0000001_a.pdf"
	repo.rows[saAttID] = domain.StoredAttachmentRow{ID: saAttID, Number: "WIA0000001", Name: "a.pdf", Status: "COMPLETE", StorageKey: &key}

	resp, err := newStoredSvc(repo, store).ResolveLinks(saCtx(t, saEmail), domain.AttachmentLinksRequest{Kind: domain.StoredAttachmentWorkItem, IDs: []string{saAttID}})
	if err != nil {
		t.Fatalf("ResolveLinks: %v", err)
	}
	if len(resp.Links) != 1 || !resp.Links[0].Available || resp.Links[0].URL != "https://files.test/dl/share-dl" {
		t.Fatalf("links = %+v", resp.Links)
	}
	if want := saNow.Add(5 * time.Minute); resp.Links[0].ExpiresAt == nil || !resp.Links[0].ExpiresAt.Equal(want) {
		t.Errorf("expiresAt = %v, want %v", resp.Links[0].ExpiresAt, want)
	}
	if len(store.shares) != 1 {
		t.Fatalf("shares created = %d, want 1", len(store.shares))
	}
	sh := store.shares[0]
	if sh.Scope != filestore.ShareRead || sh.Path != key || sh.MaxUses != downloadLinkMaxUses || !sh.ExpiresAt.Equal(saNow.Add(5*time.Minute)) {
		t.Errorf("share = %+v: want a read share on exactly the stored file, 5 minute expiry", sh)
	}
}

func TestResolveLinks_PurposeSetsHowManyTimesALinkWorks(t *testing.T) {
	key := "csm/projects/prj_9/workitems/cs0001/attachments/wia0000001_a.pdf"
	for purpose, want := range map[string]int{"": 1, "download": 1, "preview": 2} {
		repo, store := newFakeStoredRepo(), newFakeStore()
		repo.rows[saAttID] = domain.StoredAttachmentRow{ID: saAttID, Number: "WIA1", Name: "a.pdf", Status: "COMPLETE", StorageKey: &key}
		_, err := newStoredSvc(repo, store).ResolveLinks(saCtx(t, saEmail), domain.AttachmentLinksRequest{Kind: domain.StoredAttachmentWorkItem, IDs: []string{saAttID}, Purpose: purpose})
		if err != nil || len(store.shares) != 1 || store.shares[0].MaxUses != want {
			t.Errorf("purpose %q: err=%v shares=%+v, want one share with %d use(s)", purpose, err, store.shares, want)
		}
	}
	_, err := newStoredSvc(newFakeStoredRepo(), newFakeStore()).ResolveLinks(saCtx(t, saEmail), domain.AttachmentLinksRequest{Kind: domain.StoredAttachmentWorkItem, IDs: []string{saAttID}, Purpose: "stream"})
	var verr *apierror.ValidationError
	if !errors.As(err, &verr) {
		t.Errorf("unknown purpose: err = %v, want a validation error", err)
	}
}

func TestResolveLinks_UnstoredFilesGetAReasonNotALink(t *testing.T) {
	repo, store := newFakeStoredRepo(), newFakeStore()
	ids := []string{
		"b1000000-0000-0000-0000-00000000000a",
		"b1000000-0000-0000-0000-00000000000b",
		"b1000000-0000-0000-0000-00000000000c",
	}
	repo.rows[ids[0]] = domain.StoredAttachmentRow{ID: ids[0], Status: "PENDING"}
	repo.rows[ids[1]] = domain.StoredAttachmentRow{ID: ids[1], Status: "FAILED"}
	repo.rows[ids[2]] = domain.StoredAttachmentRow{ID: ids[2], Status: "COMPLETE"} // no storage key

	want := []string{domain.AttachmentUnavailableNotStored, domain.AttachmentUnavailableFailed, domain.AttachmentUnavailableNotStored}
	for i, id := range ids {
		resp, err := newStoredSvc(repo, store).ResolveLinks(saCtx(t, saEmail), domain.AttachmentLinksRequest{Kind: domain.StoredAttachmentWorkItem, IDs: []string{id}})
		if err != nil {
			t.Fatalf("ResolveLinks: %v", err)
		}
		l := resp.Links[0]
		if l.Available || l.URL != "" || l.Reason != want[i] {
			t.Errorf("link %d = %+v, want unavailable with reason %s", i, l, want[i])
		}
	}
	if len(store.shares) != 0 {
		t.Errorf("created %d shares for files that are not stored", len(store.shares))
	}
}

func TestResolveLinks_AStorageFailureOnlyAffectsThatEntry(t *testing.T) {
	repo, store := newFakeStoredRepo(), newFakeStore()
	store.shareErr = errors.New("sftpgo down")
	key := "k/a.pdf"
	repo.rows[saAttID] = domain.StoredAttachmentRow{ID: saAttID, Name: "a", Status: "COMPLETE", StorageKey: &key}

	resp, err := newStoredSvc(repo, store).ResolveLinks(saCtx(t, saEmail), domain.AttachmentLinksRequest{Kind: domain.StoredAttachmentWorkItem, IDs: []string{saAttID}})
	if err != nil {
		t.Fatalf("a storage failure must not fail the whole call: %v", err)
	}
	if resp.Links[0].Available || resp.Links[0].Reason != domain.AttachmentUnavailableStorage {
		t.Errorf("link = %+v", resp.Links[0])
	}
}

func TestResolveLinks_AHiddenIdGetsNoLink(t *testing.T) {
	repo, store := newFakeStoredRepo(), newFakeStore()
	a, hidden := "b1000000-0000-0000-0000-00000000000a", "b1000000-0000-0000-0000-00000000000f"
	key := "k/x"
	repo.rows[a] = domain.StoredAttachmentRow{ID: a, Status: "COMPLETE", StorageKey: &key}

	resp, err := newStoredSvc(repo, store).ResolveLinks(saCtx(t, saEmail), domain.AttachmentLinksRequest{
		Kind: domain.StoredAttachmentComment, IDs: []string{hidden},
	})
	if err != nil {
		t.Fatalf("ResolveLinks: %v", err)
	}
	if len(resp.Links) != 0 || len(store.shares) != 0 {
		t.Errorf("links = %+v shares = %d, want none for an id the caller cannot see", resp.Links, len(store.shares))
	}
	resp, err = newStoredSvc(repo, store).ResolveLinks(saCtx(t, saEmail), domain.AttachmentLinksRequest{
		Kind: domain.StoredAttachmentComment, IDs: []string{a},
	})
	if err != nil || len(resp.Links) != 1 || resp.Links[0].ID != a {
		t.Errorf("links = %+v err = %v, want the one visible attachment", resp.Links, err)
	}
}

func TestResolveLinks_ACallerWhoKeepsAskingForLinksIsRefused(t *testing.T) {
	repo, store := newFakeStoredRepo(), newFakeStore()
	repo.rows[saAttID] = domain.StoredAttachmentRow{ID: saAttID, Number: "WIA1", Name: "a.pdf", Status: "COMPLETE", ParentID: saParentID, CreatedBy: saEmail}
	svc := newStoredSvc(repo, store)
	svc.shares.now = func() time.Time { return saNow }
	ctx := saCtx(t, saEmail)
	req := domain.AttachmentLinksRequest{Kind: domain.StoredAttachmentWorkItem, IDs: []string{saAttID}}
	for i := 0; i < maxSharesPerCallerPerMinute; i++ {
		if _, err := svc.ResolveLinks(ctx, req); err != nil {
			t.Fatalf("call %d refused early: %v", i, err)
		}
	}
	_, err := svc.ResolveLinks(ctx, req)
	var tooMany *apierror.TooManyRequestsError
	if !errors.As(err, &tooMany) {
		t.Fatalf("err = %v, want TooManyRequestsError", err)
	}
	svc.shares.now = func() time.Time { return saNow.Add(2 * time.Minute) }
	if _, err := svc.ResolveLinks(ctx, req); err != nil {
		t.Errorf("after the window: %v", err)
	}
}

func TestOpenInlineImage(t *testing.T) {
	key := "csm/projects/prj_9/workitems/cs0001/inline_attachments/cia1_pic.png"
	png := "image/png"
	svc := func(status string, ct *string, k *string, store *fakeStore) *storedAttachmentService {
		repo := newFakeStoredRepo()
		repo.rows[saAttID] = domain.StoredAttachmentRow{ID: saAttID, Number: "CIA1", Name: "pic.png", Status: status, ContentType: ct, StorageKey: k}
		return newStoredSvc(repo, store)
	}
	ctx := saCtx(t, saEmail)

	store := newFakeStore()
	c, err := svc("COMPLETE", &png, &key, store).OpenInlineImage(ctx, saAttID)
	if err != nil {
		t.Fatalf("OpenInlineImage: %v", err)
	}
	b, _ := io.ReadAll(c.Body)
	if string(b) != "IMG" || c.ContentType != "image/png" || c.Size != 3 || len(store.opened) != 1 || store.opened[0] != key {
		t.Errorf("content = %q %+v opened=%v", b, c, store.opened)
	}

	// An unusual stored type is never echoed back as the response type.
	html := "text/html"
	c, err = svc("COMPLETE", &html, &key, newFakeStore()).OpenInlineImage(ctx, saAttID)
	if err != nil || c.ContentType != "application/octet-stream" {
		t.Errorf("odd type: err=%v type=%q", err, c.ContentType)
	}

	var nf *apierror.NotFoundError
	for name, s := range map[string]*storedAttachmentService{
		"deleted":  svc("DELETED", &png, &key, newFakeStore()),
		"pending":  svc("PENDING", &png, nil, newFakeStore()),
		"no store": svc("COMPLETE", &png, nil, newFakeStore()),
	} {
		if _, err := s.OpenInlineImage(ctx, saAttID); !errors.As(err, &nf) {
			t.Errorf("%s: err = %v, want NotFoundError", name, err)
		}
	}
	// A row the caller cannot see is absent from the query, so not found.
	if _, err := newStoredSvc(newFakeStoredRepo(), newFakeStore()).OpenInlineImage(ctx, saAttID); !errors.As(err, &nf) {
		t.Errorf("hidden: err = %v, want NotFoundError", err)
	}
	// The file is gone from storage: not found. Storage down: 503.
	missing := newFakeStore()
	missing.openErr = filestore.ErrNotFound
	if _, err := svc("COMPLETE", &png, &key, missing).OpenInlineImage(ctx, saAttID); !errors.As(err, &nf) {
		t.Errorf("file gone: err = %v, want NotFoundError", err)
	}
	down := newFakeStore()
	down.openErr = errors.New("sftpgo down")
	var su *apierror.ServiceUnavailableError
	if _, err := svc("COMPLETE", &png, &key, down).OpenInlineImage(ctx, saAttID); !errors.As(err, &su) {
		t.Errorf("storage down: err = %v, want ServiceUnavailableError", err)
	}
	var verr *apierror.ValidationError
	if _, err := svc("COMPLETE", &png, &key, newFakeStore()).OpenInlineImage(ctx, "nope"); !errors.As(err, &verr) {
		t.Errorf("bad id: err = %v, want ValidationError", err)
	}
}

// An image migrated from the previous system is a work item attachment, not a
// comment one; the inline route finds it there, and only when it is an image.
func TestOpenInlineImage_MigratedImageIsAWorkItemAttachment(t *testing.T) {
	key := "csm/projects/prj_9/workitems/cs0001/attachments/wia1_pic.png"
	png, pdf := "image/png", "application/pdf"
	svc := func(ct *string, store *fakeStore) *storedAttachmentService {
		repo := newFakeStoredRepo()
		repo.workItemRows = map[string]domain.StoredAttachmentRow{
			saAttID: {ID: saAttID, Number: "WIA1", Name: "pic.png", Status: "COMPLETE", ContentType: ct, StorageKey: &key},
		}
		return newStoredSvc(repo, store)
	}
	ctx := saCtx(t, saEmail)

	store := newFakeStore()
	c, err := svc(&png, store).OpenInlineImage(ctx, saAttID)
	if err != nil {
		t.Fatalf("OpenInlineImage: %v", err)
	}
	b, _ := io.ReadAll(c.Body)
	if string(b) != "IMG" || c.ContentType != "image/png" || len(store.opened) != 1 || store.opened[0] != key {
		t.Errorf("content = %q %+v opened=%v", b, c, store.opened)
	}

	// A migrated type with parameters is still an image, sent back without them.
	withParams := "Image/PNG; charset=binary"
	pc, err := svc(&withParams, newFakeStore()).OpenInlineImage(ctx, saAttID)
	if err != nil || pc.ContentType != "image/png" {
		t.Errorf("parameterised type: err=%v type=%q, want image/png", err, pc.ContentType)
	}

	// A work item file that is not an image is never streamed here.
	var nf *apierror.NotFoundError
	other := newFakeStore()
	if _, err := svc(&pdf, other).OpenInlineImage(ctx, saAttID); !errors.As(err, &nf) || len(other.opened) != 0 {
		t.Errorf("not an image: err = %v opened=%v, want NotFoundError and nothing read", err, other.opened)
	}
	if _, err := svc(nil, newFakeStore()).OpenInlineImage(ctx, saAttID); !errors.As(err, &nf) {
		t.Errorf("no type: err = %v, want NotFoundError", err)
	}
}

func TestLimiterKey_UsersOfOneClientDoNotShareAnAllowance(t *testing.T) {
	ctxFor := func(userID, clientID, email string) context.Context {
		ctx := saCtx(t, email)
		return auth.WithIdentity(ctx, auth.Identity{UserID: userID, ClientID: clientID})
	}
	a := limiterKey(ctxFor("", "portal-backend", "a@example.com"))
	b := limiterKey(ctxFor("", "portal-backend", "b@example.com"))
	if a == b || a == "portal-backend" {
		t.Errorf("keys %q / %q: two users of one client must not share a key", a, b)
	}
	if got := limiterKey(ctxFor("user-1", "portal-backend", "a@example.com")); got != "user-1" {
		t.Errorf("with a user id the key = %q, want the user id", got)
	}
	if got := limiterKey(auth.WithIdentity(context.Background(), auth.Identity{ClientID: "m2m"})); got != "m2m" {
		t.Errorf("a machine caller's key = %q, want its client id", got)
	}
}

func TestInlineImageIDs(t *testing.T) {
	a, b := "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222"
	html := `<p>hi</p><img src="https://portal.example.com/api/attachments/inline/` + a + `/content"><img src="/attachments/inline/` + strings.ToUpper(b) + `/content">` +
		`<img src="/attachments/inline/` + a + `/content"><img src="https://other.example.com/x.png"><a href="/attachments/` + a + `/content">not an inline ref</a>`
	got := inlineImageIDs(html)
	if len(got) != 2 || got[0] != a || got[1] != b {
		t.Errorf("ids = %v, want [%s %s] (deduped, lower-cased, inline refs only)", got, a, b)
	}
	if len(inlineImageIDs("no images here")) != 0 {
		t.Error("a comment with no images must reference none")
	}
	var many strings.Builder
	for i := 0; i < 30; i++ {
		many.WriteString(fmt.Sprintf(`<img src="/attachments/inline/%08d-1111-4111-8111-111111111111/content">`, i))
	}
	if n := len(inlineImageIDs(many.String())); n != maxAttachmentLinkIDs {
		t.Errorf("got %d ids, want the cap of %d", n, maxAttachmentLinkIDs)
	}
}

func TestLinkFromContent(t *testing.T) {
	const commentID = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	repo := newFakeStoredRepo()
	repo.linkWant = []string{saAttID}
	svc := newStoredSvc(repo, newFakeStore())
	html := `<img src="/attachments/inline/` + saAttID + `/content">`

	svc.LinkFromContent(context.Background(), commentID, saParentID, saEmail, html)
	c := repo.linkCall
	if c.comment != commentID || c.workItem != saParentID || c.email != saEmail || !c.sys || len(c.ids) != 1 || c.ids[0] != saAttID {
		t.Errorf("repo call = %+v: want the comment's own work item, the author's email, the image ids, as the system", c)
	}

	repo.linkCall.comment = ""
	svc.LinkFromContent(context.Background(), commentID, saParentID, saEmail, "<p>no images</p>")
	svc.LinkFromContent(context.Background(), commentID, saParentID, "", html)
	if repo.linkCall.comment != "" {
		t.Error("nothing should be linked for a comment with no images or no author")
	}
}

func TestResolveLinks_RejectsBadRequests(t *testing.T) {
	svc := newStoredSvc(newFakeStoredRepo(), newFakeStore())
	many := make([]string, maxDownloadLinkIDs+1)
	for i := range many {
		many[i] = saAttID
	}
	for name, req := range map[string]domain.AttachmentLinksRequest{
		"unknown kind": {Kind: "case", IDs: []string{saAttID}},
		"no ids":       {Kind: domain.StoredAttachmentWorkItem},
		"too many":     {Kind: domain.StoredAttachmentWorkItem, IDs: many},
		"not a uuid":   {Kind: domain.StoredAttachmentWorkItem, IDs: []string{"nope"}},
	} {
		_, err := svc.ResolveLinks(saCtx(t, saEmail), req)
		var verr *apierror.ValidationError
		if !errors.As(err, &verr) {
			t.Errorf("%s: err = %v, want a ValidationError", name, err)
		}
	}
}

// ---- initiate ----

func initReq() domain.InitiateAttachmentUploadRequest {
	return domain.InitiateAttachmentUploadRequest{
		Kind: domain.StoredAttachmentWorkItem, ParentID: saParentID, Name: " report.pdf ", ContentType: "Application/PDF", SizeBytes: 500,
	}
}

func TestInitiateUpload_RecordsAPendingRowAndOpensAOneUseUploadShare(t *testing.T) {
	repo, store := newFakeStoredRepo(), newFakeStore()
	resp, err := newStoredSvc(repo, store).InitiateUpload(saCtx(t, saEmail), initReq())
	if err != nil {
		t.Fatalf("InitiateUpload: %v", err)
	}
	if len(repo.inserted) != 1 {
		t.Fatalf("inserted %d rows, want 1", len(repo.inserted))
	}
	in := repo.inserted[0]
	if in.CreatedBy != saEmail || in.Name != "report.pdf" || in.ContentType != "application/pdf" || in.SizeBytes != 500 {
		t.Errorf("insert = %+v", in)
	}
	staging := filestore.StagingDir(saAttID)
	if len(store.mkdirs) != 1 || store.mkdirs[0] != staging {
		t.Errorf("mkdirs = %v, want the upload's own staging folder %s", store.mkdirs, staging)
	}
	if len(store.shares) != 1 {
		t.Fatalf("shares = %d, want 1", len(store.shares))
	}
	sh := store.shares[0]
	if sh.Scope != filestore.ShareWrite || sh.Path != staging || sh.MaxUses != 1 || !sh.ExpiresAt.Equal(saNow.Add(15*time.Minute)) {
		t.Errorf("share = %+v: want a write share on the staging folder, one use, 15 minute expiry", sh)
	}
	if resp.AttachmentID != saAttID || resp.Number != "WIA0000042" || resp.UploadURL != "https://files.test/up/share-up" ||
		resp.FieldName != "filenames" || resp.MaxBytes != 1000 {
		t.Errorf("response = %+v", resp)
	}
}

func TestInitiateUpload_RefusesAClosedWorkItemForEveryone(t *testing.T) {
	repo, store := newFakeStoredRepo(), newFakeStore()
	repo.parent.Closed = true
	_, err := newStoredSvc(repo, store).InitiateUpload(saCtx(t, saEmail), initReq())
	var cerr *apierror.ConflictError
	if !errors.As(err, &cerr) {
		t.Fatalf("err = %v, want a ConflictError", err)
	}
	if len(repo.inserted) != 0 || len(store.shares) != 0 || len(store.mkdirs) != 0 {
		t.Error("a closed work item must leave no row, folder or share behind")
	}
}

func TestInitiateUpload_AParentTheCallerCannotSeeIsNotFound(t *testing.T) {
	repo := newFakeStoredRepo()
	repo.parentErr = &apierror.NotFoundError{Msg: "parent record not found"}
	_, err := newStoredSvc(repo, newFakeStore()).InitiateUpload(saCtx(t, saEmail), initReq())
	var nerr *apierror.NotFoundError
	if !errors.As(err, &nerr) {
		t.Fatalf("err = %v, want NotFoundError", err)
	}
}

func TestInitiateUpload_Validation(t *testing.T) {
	svc := newStoredSvc(newFakeStoredRepo(), newFakeStore())
	mk := func(mut func(*domain.InitiateAttachmentUploadRequest)) domain.InitiateAttachmentUploadRequest {
		r := initReq()
		mut(&r)
		return r
	}
	for name, req := range map[string]domain.InitiateAttachmentUploadRequest{
		"unknown kind":        mk(func(r *domain.InitiateAttachmentUploadRequest) { r.Kind = "case" }),
		"parent not a uuid":   mk(func(r *domain.InitiateAttachmentUploadRequest) { r.ParentID = "x" }),
		"no name":             mk(func(r *domain.InitiateAttachmentUploadRequest) { r.Name = "  " }),
		"name too long":       mk(func(r *domain.InitiateAttachmentUploadRequest) { r.Name = strings.Repeat("a", 256) }),
		"zero size":           mk(func(r *domain.InitiateAttachmentUploadRequest) { r.SizeBytes = 0 }),
		"over the file limit": mk(func(r *domain.InitiateAttachmentUploadRequest) { r.SizeBytes = 1001 }),
		"odd content type":    mk(func(r *domain.InitiateAttachmentUploadRequest) { r.ContentType = "pdf" }),
		"comment: not image": mk(func(r *domain.InitiateAttachmentUploadRequest) {
			r.Kind, r.ContentType, r.SizeBytes = domain.StoredAttachmentComment, "application/pdf", 10
		}),
		"comment: svg": mk(func(r *domain.InitiateAttachmentUploadRequest) {
			r.Kind, r.ContentType, r.SizeBytes = domain.StoredAttachmentComment, "image/svg+xml", 10
		}),
		"comment: over image limit": mk(func(r *domain.InitiateAttachmentUploadRequest) {
			r.Kind, r.ContentType, r.SizeBytes = domain.StoredAttachmentComment, "image/png", 101
		}),
	} {
		_, err := svc.InitiateUpload(saCtx(t, saEmail), req)
		var verr *apierror.ValidationError
		if !errors.As(err, &verr) {
			t.Errorf("%s: err = %v, want a ValidationError", name, err)
		}
	}
}

func TestInitiateUpload_NeedsTheCallersToken(t *testing.T) {
	_, err := newStoredSvc(newFakeStoredRepo(), newFakeStore()).InitiateUpload(context.Background(), initReq())
	var uerr *apierror.UnauthorizedError
	if !errors.As(err, &uerr) {
		t.Fatalf("err = %v, want UnauthorizedError", err)
	}
}

func TestInitiateUpload_AStorageFailureDoesNotLeaveAPendingRow(t *testing.T) {
	repo, store := newFakeStoredRepo(), newFakeStore()
	store.mkdirErr = errors.New("sftpgo down")
	_, err := newStoredSvc(repo, store).InitiateUpload(saCtx(t, saEmail), initReq())
	var serr *apierror.ServiceUnavailableError
	if !errors.As(err, &serr) {
		t.Fatalf("err = %v, want ServiceUnavailableError", err)
	}
	if strings.Contains(err.Error(), "sftpgo down") {
		t.Errorf("the storage error leaked to the caller: %v", err)
	}
	if _, ok := repo.failed[saAttID]; !ok {
		t.Error("the row was left PENDING: want it marked FAILED")
	}
}

// ---- confirm ----

// stagedUpload prepares a PENDING row owned by saEmail with one staged file.
func stagedUpload(repo *fakeStoredRepo, store *fakeStore, size int64, kind domain.StoredAttachmentKind) string {
	ct := "application/pdf"
	repo.rows[saAttID] = domain.StoredAttachmentRow{
		ID: saAttID, Number: "WIA0000042", Name: "report.pdf", ContentType: &ct, SizeBytes: &size,
		Status: "PENDING", CreatedBy: saEmail, ParentID: saParentID,
	}
	staging := filestore.StagingDir(saAttID)
	store.listing[staging] = []filestore.Entry{{Name: ".keep", Size: 1}, {Name: "whatever.pdf", Size: size}}
	return staging
}

func TestConfirmUpload_MovesTheFileToItsFinalKeyAndCompletesTheRow(t *testing.T) {
	repo, store := newFakeStoredRepo(), newFakeStore()
	staging := stagedUpload(repo, store, 500, domain.StoredAttachmentWorkItem)

	resp, err := newStoredSvc(repo, store).ConfirmUpload(saCtx(t, saEmail), domain.ConfirmAttachmentUploadRequest{Kind: domain.StoredAttachmentWorkItem, ID: saAttID})
	if err != nil {
		t.Fatalf("ConfirmUpload: %v", err)
	}
	wantKey := "csm/projects/prj_9/workitems/cs0001/attachments/wia0000042_report.pdf"
	if len(store.renames) != 1 || store.renames[0] != [2]string{staging + "/whatever.pdf", wantKey} {
		t.Errorf("renames = %v, want the staged file moved to %s", store.renames, wantKey)
	}
	if repo.completed[saAttID] != wantKey {
		t.Errorf("completed key = %q, want %q", repo.completed[saAttID], wantKey)
	}
	if len(repo.completeCtx) != 1 || !repo.completeCtx[0] {
		t.Error("MarkComplete must run as the system: UPDATE is internal-only on these tables")
	}
	if len(store.removed) != 1 || store.removed[0] != staging {
		t.Errorf("removed = %v, want the staging folder", store.removed)
	}
	if resp.Status != "COMPLETE" || resp.SizeBytes != 500 || resp.Number != "WIA0000042" {
		t.Errorf("response = %+v", resp)
	}
}

func TestConfirmUpload_RetryAfterTheMoveFinishesTheRowInsteadOfRejecting(t *testing.T) {
	repo, store := newFakeStoredRepo(), newFakeStore()
	stagedUpload(repo, store, 500, domain.StoredAttachmentWorkItem)
	// An earlier confirm moved the file but failed before marking the row.
	store.listing[filestore.StagingDir(saAttID)] = nil
	finalDir := "csm/projects/prj_9/workitems/cs0001/attachments"
	store.listing[finalDir] = []filestore.Entry{{Name: "wia0000042_report.pdf", Size: 500}}

	resp, err := newStoredSvc(repo, store).ConfirmUpload(saCtx(t, saEmail), domain.ConfirmAttachmentUploadRequest{Kind: domain.StoredAttachmentWorkItem, ID: saAttID})
	if err != nil {
		t.Fatalf("ConfirmUpload: %v", err)
	}
	if resp.Status != "COMPLETE" || resp.SizeBytes != 500 || repo.completed[saAttID] != finalDir+"/wia0000042_report.pdf" {
		t.Errorf("response = %+v completed = %v", resp, repo.completed)
	}
}

func TestConfirmUpload_ATransientListingFailureAfterAMoveDoesNotRejectTheRow(t *testing.T) {
	repo, store := newFakeStoredRepo(), newFakeStore()
	stagedUpload(repo, store, 500, domain.StoredAttachmentWorkItem)
	store.listing[filestore.StagingDir(saAttID)] = nil
	store.listErr = errors.New("sftpgo down")
	_, err := newStoredSvc(repo, store).ConfirmUpload(saCtx(t, saEmail), domain.ConfirmAttachmentUploadRequest{Kind: domain.StoredAttachmentWorkItem, ID: saAttID})
	if err == nil || repo.rows[saAttID].Status != "PENDING" {
		t.Errorf("err = %v status = %s, want an error and the row still PENDING", err, repo.rows[saAttID].Status)
	}
}

func TestConfirmUpload_DeploymentAndInlineKeysHaveTheirOwnShape(t *testing.T) {
	// deployment
	repo, store := newFakeStoredRepo(), newFakeStore()
	repo.parent = domain.StoredAttachmentParent{ID: saParentID, Number: "DEP-7", ProjectNumber: "PRJ-9"}
	stagedUpload(repo, store, 500, domain.StoredAttachmentDeployment)
	if _, err := newStoredSvc(repo, store).ConfirmUpload(saCtx(t, saEmail), domain.ConfirmAttachmentUploadRequest{Kind: domain.StoredAttachmentDeployment, ID: saAttID}); err != nil {
		t.Fatalf("deployment confirm: %v", err)
	}
	if got, want := store.renames[0][1], "csm/projects/prj_9/deployments/dep_7/wia0000042_report.pdf"; got != want {
		t.Errorf("deployment key = %q, want %q", got, want)
	}

	// inline comment image (the inline limit is 100 bytes in these tests)
	repo, store = newFakeStoredRepo(), newFakeStore()
	stagedUpload(repo, store, 50, domain.StoredAttachmentComment)
	store.head = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
	if _, err := newStoredSvc(repo, store).ConfirmUpload(saCtx(t, saEmail), domain.ConfirmAttachmentUploadRequest{Kind: domain.StoredAttachmentComment, ID: saAttID}); err != nil {
		t.Fatalf("inline confirm: %v", err)
	}
	if got, want := store.renames[0][1], "csm/projects/prj_9/workitems/cs0001/inline_attachments/wia0000042_report.pdf"; got != want {
		t.Errorf("inline key = %q, want %q", got, want)
	}
}

func TestConfirmUpload_InlineImagesAreCheckedByTheirBytes(t *testing.T) {
	for name, head := range map[string][]byte{
		"html":  []byte("<html><script>alert(1)</script></html>"),
		"svg":   []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"></svg>`),
		"pdf":   []byte("%PDF-1.7 ....."),
		"plain": []byte("just text"),
	} {
		repo, store := newFakeStoredRepo(), newFakeStore()
		staging := stagedUpload(repo, store, 50, domain.StoredAttachmentComment)
		store.head = head
		_, err := newStoredSvc(repo, store).ConfirmUpload(saCtx(t, saEmail), domain.ConfirmAttachmentUploadRequest{Kind: domain.StoredAttachmentComment, ID: saAttID})
		var verr *apierror.ValidationError
		if !errors.As(err, &verr) {
			t.Errorf("%s: err = %v, want a ValidationError", name, err)
		}
		if len(store.renames) != 0 || len(repo.completed) != 0 {
			t.Errorf("%s: a non-image was moved into place", name)
		}
		if len(store.removed) != 1 || store.removed[0] != staging {
			t.Errorf("%s: the rejected file was not removed: %v", name, store.removed)
		}
		if _, ok := repo.failed[saAttID]; !ok {
			t.Errorf("%s: the row was not marked FAILED", name)
		}
	}
}

func TestConfirmUpload_RejectsWhatDoesNotMatchTheDeclaration(t *testing.T) {
	cases := map[string]func(repo *fakeStoredRepo, store *fakeStore, staging string){
		"no file": func(_ *fakeStoredRepo, s *fakeStore, staging string) {
			s.listing[staging] = []filestore.Entry{{Name: ".keep", Size: 1}}
		},
		"no folder at all": func(_ *fakeStoredRepo, s *fakeStore, staging string) { delete(s.listing, staging) },
		"two files": func(_ *fakeStoredRepo, s *fakeStore, staging string) {
			s.listing[staging] = []filestore.Entry{{Name: "a", Size: 500}, {Name: "b", Size: 500}}
		},
		"size differs": func(_ *fakeStoredRepo, s *fakeStore, staging string) {
			s.listing[staging] = []filestore.Entry{{Name: "a", Size: 499}}
		},
		"over the limit": func(r *fakeStoredRepo, s *fakeStore, staging string) {
			big := int64(5000)
			row := r.rows[saAttID]
			row.SizeBytes = &big
			r.rows[saAttID] = row
			s.listing[staging] = []filestore.Entry{{Name: "a", Size: big}}
		},
	}
	for name, setup := range cases {
		repo, store := newFakeStoredRepo(), newFakeStore()
		staging := stagedUpload(repo, store, 500, domain.StoredAttachmentWorkItem)
		setup(repo, store, staging)
		_, err := newStoredSvc(repo, store).ConfirmUpload(saCtx(t, saEmail), domain.ConfirmAttachmentUploadRequest{Kind: domain.StoredAttachmentWorkItem, ID: saAttID})
		var verr *apierror.ValidationError
		if !errors.As(err, &verr) {
			t.Errorf("%s: err = %v, want a ValidationError", name, err)
		}
		if len(store.renames) != 0 || len(repo.completed) != 0 {
			t.Errorf("%s: something was moved into place", name)
		}
		if _, ok := repo.failed[saAttID]; !ok {
			t.Errorf("%s: the row was not marked FAILED", name)
		}
	}
}

func TestConfirmUpload_AWorkItemClosedDuringTheUploadIsRefused(t *testing.T) {
	repo, store := newFakeStoredRepo(), newFakeStore()
	staging := stagedUpload(repo, store, 500, domain.StoredAttachmentWorkItem)
	repo.parent.Closed = true

	_, err := newStoredSvc(repo, store).ConfirmUpload(saCtx(t, saEmail), domain.ConfirmAttachmentUploadRequest{Kind: domain.StoredAttachmentWorkItem, ID: saAttID})
	var cerr *apierror.ConflictError
	if !errors.As(err, &cerr) {
		t.Fatalf("err = %v, want a ConflictError", err)
	}
	if len(store.renames) != 0 || len(repo.completed) != 0 {
		t.Error("a file was accepted for a closed work item")
	}
	if len(store.removed) != 1 || store.removed[0] != staging {
		t.Errorf("the staged file was not removed: %v", store.removed)
	}
}

func TestConfirmUpload_OnlyTheUploaderCanConfirm(t *testing.T) {
	repo, store := newFakeStoredRepo(), newFakeStore()
	stagedUpload(repo, store, 500, domain.StoredAttachmentWorkItem)
	_, err := newStoredSvc(repo, store).ConfirmUpload(saCtx(t, "someone.else@example.com"), domain.ConfirmAttachmentUploadRequest{Kind: domain.StoredAttachmentWorkItem, ID: saAttID})
	var nerr *apierror.NotFoundError
	if !errors.As(err, &nerr) {
		t.Fatalf("err = %v, want NotFoundError (never Forbidden)", err)
	}
	if len(store.renames) != 0 {
		t.Error("someone else's upload was moved")
	}
}

func TestConfirmUpload_RepeatingASuccessfulConfirmTouchesNothing(t *testing.T) {
	repo, store := newFakeStoredRepo(), newFakeStore()
	stagedUpload(repo, store, 500, domain.StoredAttachmentWorkItem)
	row := repo.rows[saAttID]
	row.Status = "COMPLETE"
	repo.rows[saAttID] = row

	resp, err := newStoredSvc(repo, store).ConfirmUpload(saCtx(t, saEmail), domain.ConfirmAttachmentUploadRequest{Kind: domain.StoredAttachmentWorkItem, ID: saAttID})
	if err != nil || resp.Status != "COMPLETE" {
		t.Fatalf("resp = %+v, err = %v", resp, err)
	}
	if len(store.renames)+len(store.mkdirs)+len(store.removed) != 0 || len(repo.completed) != 0 {
		t.Error("a repeated confirm changed something")
	}
}

func TestConfirmUpload_ARejectedUploadCannotBeConfirmedAgain(t *testing.T) {
	repo, store := newFakeStoredRepo(), newFakeStore()
	stagedUpload(repo, store, 500, domain.StoredAttachmentWorkItem)
	row := repo.rows[saAttID]
	row.Status = "FAILED"
	repo.rows[saAttID] = row
	_, err := newStoredSvc(repo, store).ConfirmUpload(saCtx(t, saEmail), domain.ConfirmAttachmentUploadRequest{Kind: domain.StoredAttachmentWorkItem, ID: saAttID})
	var cerr *apierror.ConflictError
	if !errors.As(err, &cerr) {
		t.Fatalf("err = %v, want ConflictError", err)
	}
}

func TestConfirmUpload_AStorageFailureLeavesTheUploadRetryable(t *testing.T) {
	repo, store := newFakeStoredRepo(), newFakeStore()
	stagedUpload(repo, store, 500, domain.StoredAttachmentWorkItem)
	store.renameEr = errors.New("sftpgo down")

	_, err := newStoredSvc(repo, store).ConfirmUpload(saCtx(t, saEmail), domain.ConfirmAttachmentUploadRequest{Kind: domain.StoredAttachmentWorkItem, ID: saAttID})
	var serr *apierror.ServiceUnavailableError
	if !errors.As(err, &serr) {
		t.Fatalf("err = %v, want ServiceUnavailableError", err)
	}
	if _, failed := repo.failed[saAttID]; failed || len(repo.completed) != 0 || len(store.removed) != 0 {
		t.Error("a transient storage failure must leave the row PENDING and the file staged so confirm can be retried")
	}
}

func TestConfirmUpload_AConcurrentConfirmIsAConflict(t *testing.T) {
	repo, store := newFakeStoredRepo(), newFakeStore()
	stagedUpload(repo, store, 500, domain.StoredAttachmentWorkItem)
	repo.completeOK = false
	_, err := newStoredSvc(repo, store).ConfirmUpload(saCtx(t, saEmail), domain.ConfirmAttachmentUploadRequest{Kind: domain.StoredAttachmentWorkItem, ID: saAttID})
	var cerr *apierror.ConflictError
	if !errors.As(err, &cerr) {
		t.Fatalf("err = %v, want ConflictError", err)
	}
}

func TestConfirmUpload_UnknownAttachmentIsNotFound(t *testing.T) {
	_, err := newStoredSvc(newFakeStoredRepo(), newFakeStore()).ConfirmUpload(saCtx(t, saEmail), domain.ConfirmAttachmentUploadRequest{Kind: domain.StoredAttachmentWorkItem, ID: saAttID})
	var nerr *apierror.NotFoundError
	if !errors.As(err, &nerr) {
		t.Fatalf("err = %v, want NotFoundError", err)
	}
}

// captureAudit runs fn with the default logger writing JSON lines, and returns
// the lines that carry the audit marker.
func captureAudit(t *testing.T, fn func()) []map[string]any {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	defer slog.SetDefault(prev)
	fn()
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		var m map[string]any
		if json.Unmarshal([]byte(line), &m) == nil && m["audit"] == true {
			out = append(out, m)
		}
	}
	return out
}

func TestAudit_EveryLinkNamesWhoGotItAndWhichShare(t *testing.T) {
	repo, store := newFakeStoredRepo(), newFakeStore()
	key := "csm/projects/prj_9/workitems/cs0001/attachments/wia0000001_a.pdf"
	repo.rows[saAttID] = domain.StoredAttachmentRow{ID: saAttID, Name: "a.pdf", Status: "COMPLETE", StorageKey: &key}
	svc := newStoredSvc(repo, store)

	ctx := func() context.Context {
		return auth.WithIdentity(saCtx(t, saEmail), auth.Identity{Validated: true, UserEmail: saEmail, UserID: "user-uuid-1"})
	}
	lines := captureAudit(t, func() {
		if _, err := svc.ResolveLinks(ctx(), domain.AttachmentLinksRequest{Kind: domain.StoredAttachmentWorkItem, IDs: []string{saAttID}}); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.InitiateUpload(ctx(), initReq()); err != nil {
			t.Fatal(err)
		}
	})
	if len(lines) != 2 {
		t.Fatalf("audit lines = %d, want one per link (a download and an upload): %v", len(lines), lines)
	}
	for i, want := range []string{"download", "upload"} {
		l := lines[i]
		if l["action"] != want || l["callerId"] != "user-uuid-1" || l["attachmentId"] != saAttID || l["kind"] != "work_item" {
			t.Errorf("line %d = %v", i, l)
		}
		if _, has := l["actor"]; has {
			t.Errorf("line %d still carries an actor field: %v", i, l)
		}
		if strings.Contains(fmt.Sprint(l), saEmail) {
			t.Errorf("line %d carries the caller's email: %v", i, l)
		}
	}
	// What SFTPGo's own log carries, so a transfer can be matched to the line.
	if lines[0]["path"] != key || lines[1]["path"] != filestore.StagingDir(saAttID) {
		t.Errorf("paths = %v / %v, want the stored file and the staging folder", lines[0]["path"], lines[1]["path"])
	}
}

// The share id is the link's secret, so it must never reach the log in full.
func TestAudit_NeverLogsTheWholeShareId(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	defer slog.SetDefault(prev)

	auditLink(context.Background(), "download", domain.StoredAttachmentWorkItem, saAttID, "csm/x/f.pdf", "s3cr3tShareId0123456789ab", saNow)
	out := buf.String()
	if strings.Contains(out, "s3cr3tShareId0123456789ab") || strings.Contains(out, "0123456789ab") {
		t.Fatalf("the share id leaked into the log: %s", out)
	}
	if !strings.Contains(out, `"shareRef":"s3cr3t..."`) {
		t.Errorf("want a short prefix to recognise the link by: %s", out)
	}
	if shareRef("") != "..." || shareRef("ab") != "..." {
		t.Errorf("a short or empty id must not be echoed: %q %q", shareRef(""), shareRef("ab"))
	}
}

func TestAudit_NoLineWhenNoLinkWasCreated(t *testing.T) {
	repo, store := newFakeStoredRepo(), newFakeStore()
	store.shareErr = errors.New("sftpgo down")
	key := "k/a.pdf"
	repo.rows[saAttID] = domain.StoredAttachmentRow{ID: saAttID, Status: "COMPLETE", StorageKey: &key}
	lines := captureAudit(t, func() {
		_, _ = newStoredSvc(repo, store).ResolveLinks(saCtx(t, saEmail), domain.AttachmentLinksRequest{Kind: domain.StoredAttachmentWorkItem, IDs: []string{saAttID}})
	})
	if len(lines) != 0 {
		t.Errorf("a link that was not created must not be logged as created: %v", lines)
	}
}

func TestAudit_CallerIdFallsBackToTheClientIdThenDash(t *testing.T) {
	user := auth.WithIdentity(context.Background(), auth.Identity{UserID: "u1", ClientID: "c1"})
	machine := auth.WithIdentity(context.Background(), auth.Identity{ClientID: "c1"})
	for name, tc := range map[string]struct {
		ctx  context.Context
		want string
	}{"user": {user, "u1"}, "machine": {machine, "c1"}, "nobody": {context.Background(), "-"}} {
		if got := auditCaller(tc.ctx); got != tc.want {
			t.Errorf("%s: callerId = %q, want %q", name, got, tc.want)
		}
	}
}

func TestAudit_CarriesTheCorrelationId(t *testing.T) {
	var ctx context.Context
	h := middleware.CorrelationID(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { ctx = r.Context() }))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-CSM-Correlation-ID", "corr-123")
	h.ServeHTTP(httptest.NewRecorder(), req)

	lines := captureAudit(t, func() {
		auditLink(ctx, "download", domain.StoredAttachmentWorkItem, saAttID, "csm/x/f.pdf", "abcdefghijklmnop", saNow)
	})
	if len(lines) != 1 || lines[0]["correlationId"] != "corr-123" {
		t.Fatalf("lines = %v", lines)
	}
}

// ---- delete ----

const saKey = "csm/projects/prj_9/workitems/cs0001/attachments/wia0000042_report.pdf"

// storedRow prepares a COMPLETE attachment uploaded by saEmail.
func storedRow(repo *fakeStoredRepo) {
	size := int64(500)
	key := saKey
	repo.rows[saAttID] = domain.StoredAttachmentRow{
		ID: saAttID, Number: "WIA0000042", Name: "report.pdf", SizeBytes: &size,
		Status: "COMPLETE", StorageKey: &key, CreatedBy: saEmail, ParentID: saParentID,
	}
}

func deleteReq() domain.DeleteStoredAttachmentRequest {
	return domain.DeleteStoredAttachmentRequest{Kind: domain.StoredAttachmentWorkItem, ID: saAttID}
}

func TestDeleteAttachment_TheUploaderRemovesTheFileThenMarksTheRow(t *testing.T) {
	repo, store := newFakeStoredRepo(), newFakeStore()
	storedRow(repo)
	if err := newStoredSvc(repo, store).DeleteAttachment(saCtx(t, saEmail), deleteReq()); err != nil {
		t.Fatalf("DeleteAttachment: %v", err)
	}
	if len(store.files) != 1 || store.files[0] != saKey {
		t.Errorf("files removed = %v, want the stored file", store.files)
	}
	if repo.deleted[saAttID] != saEmail {
		t.Errorf("deleted_by = %q, want the caller", repo.deleted[saAttID])
	}
	if len(repo.deleteCtx) != 1 || !repo.deleteCtx[0] {
		t.Error("MarkDeleted must run as the system: UPDATE is internal-only on these tables")
	}
}

func TestDeleteAttachment_SomeoneElseCannotDeleteIt(t *testing.T) {
	repo, store := newFakeStoredRepo(), newFakeStore()
	storedRow(repo)
	err := newStoredSvc(repo, store).DeleteAttachment(saCtx(t, "someone.else@example.com"), deleteReq())
	var ferr *apierror.ForbiddenError
	if !errors.As(err, &ferr) {
		t.Fatalf("err = %v, want ForbiddenError", err)
	}
	if len(store.files) != 0 || len(repo.deleted) != 0 {
		t.Error("someone else's attachment was deleted")
	}
}

func TestDeleteAttachment_TheEmailMatchIgnoresCase(t *testing.T) {
	repo, store := newFakeStoredRepo(), newFakeStore()
	storedRow(repo)
	if err := newStoredSvc(repo, store).DeleteAttachment(saCtx(t, "EXT@Example.com"), deleteReq()); err != nil {
		t.Fatalf("err = %v", err)
	}
}

func TestDeleteAttachment_NotAllowedOnceTheWorkItemIsClosed(t *testing.T) {
	repo, store := newFakeStoredRepo(), newFakeStore()
	storedRow(repo)
	repo.parent.Closed = true
	err := newStoredSvc(repo, store).DeleteAttachment(saCtx(t, saEmail), deleteReq())
	var cerr *apierror.ConflictError
	if !errors.As(err, &cerr) {
		t.Fatalf("err = %v, want ConflictError", err)
	}
	if len(store.files) != 0 || len(repo.deleted) != 0 {
		t.Error("an attachment of a closed work item was deleted")
	}
}

func TestDeleteAttachment_AHiddenOrUnknownAttachmentIsNotFound(t *testing.T) {
	err := newStoredSvc(newFakeStoredRepo(), newFakeStore()).DeleteAttachment(saCtx(t, saEmail), deleteReq())
	var nerr *apierror.NotFoundError
	if !errors.As(err, &nerr) {
		t.Fatalf("err = %v, want NotFoundError", err)
	}
}

func TestDeleteAttachment_RepeatingASuccessfulDeleteChangesNothing(t *testing.T) {
	repo, store := newFakeStoredRepo(), newFakeStore()
	storedRow(repo)
	row := repo.rows[saAttID]
	row.Status = "DELETED"
	repo.rows[saAttID] = row
	if err := newStoredSvc(repo, store).DeleteAttachment(saCtx(t, saEmail), deleteReq()); err != nil {
		t.Fatalf("err = %v, want success", err)
	}
	if len(store.files) != 0 || len(repo.deleteCtx) != 0 {
		t.Error("a repeated delete touched something")
	}
}

func TestDeleteAttachment_OnlyAStoredAttachmentCanBeDeleted(t *testing.T) {
	for _, status := range []string{"PENDING", "FAILED", "SOURCE_MISSING"} {
		repo, store := newFakeStoredRepo(), newFakeStore()
		storedRow(repo)
		row := repo.rows[saAttID]
		row.Status = status
		repo.rows[saAttID] = row
		err := newStoredSvc(repo, store).DeleteAttachment(saCtx(t, saEmail), deleteReq())
		var cerr *apierror.ConflictError
		if !errors.As(err, &cerr) {
			t.Errorf("%s: err = %v, want ConflictError", status, err)
		}
	}
}

func TestDeleteAttachment_AFileAlreadyGoneStillCompletesTheDelete(t *testing.T) {
	repo, store := newFakeStoredRepo(), newFakeStore()
	storedRow(repo)
	store.fileErr = filestore.ErrNotFound
	if err := newStoredSvc(repo, store).DeleteAttachment(saCtx(t, saEmail), deleteReq()); err != nil {
		t.Fatalf("err = %v, want success (a retry after the file was removed)", err)
	}
	if repo.deleted[saAttID] == "" {
		t.Error("the row was not marked deleted")
	}
}

func TestDeleteAttachment_AStorageFailureLeavesTheRowCompleteSoItCanBeRetried(t *testing.T) {
	repo, store := newFakeStoredRepo(), newFakeStore()
	storedRow(repo)
	store.fileErr = errors.New("sftpgo down")
	err := newStoredSvc(repo, store).DeleteAttachment(saCtx(t, saEmail), deleteReq())
	var serr *apierror.ServiceUnavailableError
	if !errors.As(err, &serr) {
		t.Fatalf("err = %v, want ServiceUnavailableError", err)
	}
	if strings.Contains(err.Error(), "sftpgo down") {
		t.Errorf("the storage error leaked to the caller: %v", err)
	}
	if len(repo.deleted) != 0 {
		t.Error("the row was marked deleted although the file is still there")
	}
}

func TestDeleteAttachment_Validation(t *testing.T) {
	svc := newStoredSvc(newFakeStoredRepo(), newFakeStore())
	for name, req := range map[string]domain.DeleteStoredAttachmentRequest{
		"unknown kind":     {Kind: "case", ID: saAttID},
		"comment image":    {Kind: domain.StoredAttachmentComment, ID: saAttID},
		"id is not a uuid": {Kind: domain.StoredAttachmentWorkItem, ID: "nope"},
	} {
		err := svc.DeleteAttachment(saCtx(t, saEmail), req)
		var verr *apierror.ValidationError
		if !errors.As(err, &verr) {
			t.Errorf("%s: err = %v, want ValidationError", name, err)
		}
	}
	if err := svc.DeleteAttachment(context.Background(), deleteReq()); err == nil {
		t.Error("a delete without the caller's token was accepted")
	}
}

func TestResolveLinks_ADeletedAttachmentHasNoLink(t *testing.T) {
	repo, store := newFakeStoredRepo(), newFakeStore()
	storedRow(repo)
	row := repo.rows[saAttID]
	row.Status = "DELETED"
	repo.rows[saAttID] = row
	resp, err := newStoredSvc(repo, store).ResolveLinks(saCtx(t, saEmail), domain.AttachmentLinksRequest{Kind: domain.StoredAttachmentWorkItem, IDs: []string{saAttID}})
	if err != nil {
		t.Fatal(err)
	}
	if l := resp.Links[0]; l.Available || l.URL != "" || l.Reason != domain.AttachmentUnavailableDeleted {
		t.Errorf("link = %+v, want unavailable with reason DELETED", l)
	}
	if len(store.shares) != 0 {
		t.Error("a share was created for a deleted attachment")
	}
}

// pngBytes is the signature of a PNG plus padding: enough for the type check.
var pngBytes = append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 24)...)

func dataURI(subtype string, b []byte) string {
	return "data:image/" + subtype + ";base64," + base64.StdEncoding.EncodeToString(b)
}

func TestExtractInlineImages_StoresTheImageAndKeepsOnlyItsId(t *testing.T) {
	repo := newFakeStoredRepo()
	store := newFakeStore()
	svc := newStoredSvc(repo, store)
	ctx := saCtx(t, saEmail)

	html := `<p>Look:</p><img src="` + dataURI("png", pngBytes) + `" alt="x"><p>done</p>`
	out, err := svc.ExtractInlineImages(ctx, saParentID, saEmail, html)
	if err != nil {
		t.Fatalf("ExtractInlineImages: %v", err)
	}
	want := `<p>Look:</p><img src="/` + saAttID + `" alt="x"><p>done</p>`
	if out != want {
		t.Errorf("content = %q, want %q", out, want)
	}
	if len(repo.inserted) != 1 || repo.inserted[0].ParentID != saParentID || repo.inserted[0].CreatedBy != saEmail ||
		repo.inserted[0].ContentType != "image/png" || repo.inserted[0].SizeBytes != int64(len(pngBytes)) {
		t.Errorf("inserted = %+v", repo.inserted)
	}
	if len(store.put) != 1 || !strings.Contains(store.put[0], "inline_attachments") || !strings.HasSuffix(store.put[0], ".png") {
		t.Errorf("stored at %v", store.put)
	}
	if string(store.putData[store.put[0]]) != string(pngBytes) {
		t.Error("the stored bytes are not the image")
	}
	if repo.completed[saAttID] == "" || len(repo.completeCtx) != 1 || !repo.completeCtx[0] {
		t.Errorf("completed = %v (system=%v)", repo.completed, repo.completeCtx)
	}
}

func TestExtractInlineImages_LeavesAloneWhatItShouldNotTouch(t *testing.T) {
	svc := newStoredSvc(newFakeStoredRepo(), newFakeStore())
	ctx := saCtx(t, saEmail)
	for name, html := range map[string]string{
		"no image":        "<p>hello</p>",
		"a stored image":  `<img src="/` + saAttID + `">`,
		"a remote image":  `<img src="https://example.com/a.png">`,
		"a non-img data":  `<a href="data:text/plain;base64,aGk=">x</a>`,
		"no author email": "",
	} {
		author := saEmail
		if name == "no author email" {
			author, html = "", `<img src="`+dataURI("png", pngBytes)+`">`
		}
		out, err := svc.ExtractInlineImages(ctx, saParentID, author, html)
		if err != nil || out != html {
			t.Errorf("%s: out=%q err=%v, want it unchanged", name, out, err)
		}
	}
}

func TestExtractInlineImages_Refusals(t *testing.T) {
	ctx := saCtx(t, saEmail)
	var verr *apierror.ValidationError
	big := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 200)...) // over the 100 byte test limit
	for name, html := range map[string]string{
		"gif":              `<img src="` + dataURI("gif", []byte("GIF89a....")) + `">`,
		"svg":              `<img src="` + dataURI("svg+xml", []byte("<svg/>")) + `">`,
		"too large":        `<img src="` + dataURI("png", big) + `">`,
		"not really a png": `<img src="` + dataURI("png", []byte("<script>alert(1)</script>")) + `">`,
		"bad base64":       `<img src="data:image/png;base64,abc">`,
	} {
		repo, store := newFakeStoredRepo(), newFakeStore()
		_, err := newStoredSvc(repo, store).ExtractInlineImages(ctx, saParentID, saEmail, html)
		if !errors.As(err, &verr) {
			t.Errorf("%s: err = %v, want ValidationError", name, err)
		}
		if len(store.put) != 0 {
			t.Errorf("%s: a file was written", name)
		}
	}
	// jpg is accepted as an alias of jpeg when the bytes agree.
	jpeg := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 24)...)
	if _, err := newStoredSvc(newFakeStoredRepo(), newFakeStore()).ExtractInlineImages(ctx, saParentID, saEmail, `<img src="`+dataURI("jpg", jpeg)+`">`); err != nil {
		t.Errorf("jpg: %v", err)
	}
}

func TestExtractInlineImages_AHiddenWorkItemIsNotFoundAndStorageFailureStoresNothing(t *testing.T) {
	ctx := saCtx(t, saEmail)
	html := `<img src="` + dataURI("png", pngBytes) + `">`

	hidden := newFakeStoredRepo()
	hidden.parentErr = &apierror.NotFoundError{Msg: "work item not found"}
	var nf *apierror.NotFoundError
	if _, err := newStoredSvc(hidden, newFakeStore()).ExtractInlineImages(ctx, saParentID, saEmail, html); !errors.As(err, &nf) {
		t.Errorf("hidden: err = %v, want NotFoundError", err)
	}

	repo, store := newFakeStoredRepo(), newFakeStore()
	store.putErr = errors.New("sftpgo down")
	var su *apierror.ServiceUnavailableError
	if _, err := newStoredSvc(repo, store).ExtractInlineImages(ctx, saParentID, saEmail, html); !errors.As(err, &su) {
		t.Errorf("storage down: err = %v, want ServiceUnavailableError", err)
	}
	if repo.failed[saAttID] == "" || len(repo.completed) != 0 {
		t.Errorf("the row must be failed, not completed: failed=%v completed=%v", repo.failed, repo.completed)
	}
}

// A write the server accepted but put somewhere else must not leave a COMPLETE
// row whose file is missing.
func TestExtractInlineImages_AFileThatIsNotWhereItWasWrittenFailsTheComment(t *testing.T) {
	repo, store := newFakeStoredRepo(), newFakeStore()
	store.dropListing = true
	var su *apierror.ServiceUnavailableError
	_, err := newStoredSvc(repo, store).ExtractInlineImages(saCtx(t, saEmail), saParentID, saEmail, `<img src="`+dataURI("png", pngBytes)+`">`)
	if !errors.As(err, &su) {
		t.Fatalf("err = %v, want ServiceUnavailableError", err)
	}
	if len(repo.completed) != 0 || repo.failed[saAttID] == "" {
		t.Errorf("the row must be failed, not completed: failed=%v completed=%v", repo.failed, repo.completed)
	}
	// What was written is removed, so nothing is left behind unreferenced.
	if len(store.files) != 1 || !strings.Contains(store.files[0], "inline_attachments") {
		t.Errorf("the unverified file must be removed, removed = %v", store.files)
	}
	if strings.Contains(repo.failed[saAttID], "may remain") {
		t.Errorf("nothing remains, but the row says so: %q", repo.failed[saAttID])
	}
}

// When the unverified file cannot be removed either, where it is stays on the
// row so it can be cleaned up later.
func TestExtractInlineImages_AnUnremovableFileIsRecordedOnTheRow(t *testing.T) {
	repo, store := newFakeStoredRepo(), newFakeStore()
	store.dropListing = true
	store.fileErr = errors.New("sftpgo refused")
	_, err := newStoredSvc(repo, store).ExtractInlineImages(saCtx(t, saEmail), saParentID, saEmail, `<img src="`+dataURI("png", pngBytes)+`">`)
	var su *apierror.ServiceUnavailableError
	if !errors.As(err, &su) {
		t.Fatalf("err = %v, want ServiceUnavailableError", err)
	}
	if got := repo.failed[saAttID]; !strings.Contains(got, "may remain at csm/") || !strings.Contains(got, "inline_attachments") {
		t.Errorf("the row must record where the file is, got %q", got)
	}
}

func TestInlineImageIDs_FindsEveryReferenceShape(t *testing.T) {
	a, b, c := "b1000000-0000-0000-0000-00000000000a", "b1000000-0000-0000-0000-00000000000b", "b1000000-0000-0000-0000-00000000000c"
	html := `<img src="https://p.example/attachments/inline/` + a + `/content">` +
		`<img src="/` + b + `"><img src='` + c + `.iix'><img src="/` + b + `">` +
		`<img src="https://example.com/` + a + `.png">`
	got := inlineImageIDs(html)
	if len(got) != 3 || got[0] != a || got[1] != b || got[2] != c {
		t.Errorf("ids = %v, want [%s %s %s]", got, a, b, c)
	}
}
