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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/googledrive"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

// mockDriveClient is a test double for driveClient. parents is the folder
// tree: child id -> parent ids.
type mockDriveClient struct {
	listFilesFn     func(ctx context.Context, folderID string) ([]googledrive.DriveFile, error)
	searchFoldersFn func(ctx context.Context, folderName string, limit int) ([]googledrive.DriveFolder, error)
	parents         map[string][]string
	parentsErr      error
	parentCalls     int
}

func (m *mockDriveClient) ListFiles(ctx context.Context, folderID string) ([]googledrive.DriveFile, error) {
	return m.listFilesFn(ctx, folderID)
}

func (m *mockDriveClient) SearchFolders(ctx context.Context, folderName string, limit int) ([]googledrive.DriveFolder, error) {
	return m.searchFoldersFn(ctx, folderName, limit)
}

func (m *mockDriveClient) Parents(_ context.Context, fileID string) ([]string, error) {
	m.parentCalls++
	if m.parentsErr != nil {
		return nil, m.parentsErr
	}
	return m.parents[fileID], nil
}

// testDriveTree: root-1 > customers > folder-1; outside-1 sits under an
// unrelated top-level folder.
func testDriveTree() map[string][]string {
	return map[string][]string{
		"folder-1":  {"customers"},
		"customers": {"root-1"},
		"outside-1": {"other-top"},
	}
}

var testDriveRoots = []string{"root-1"}

func TestSplFilesHandler_ListFiles(t *testing.T) {
	t.Run("requires authentication", func(t *testing.T) {
		h := NewFilesHandler(&mockDriveClient{}, viewerAccessGuard, testDriveRoots)
		r := httptest.NewRequest(http.MethodGet, "/spl/files?folderId=abc", nil)
		w := httptest.NewRecorder()

		h.ListFiles(w, r)

		assertStatus(t, w, http.StatusUnauthorized)
	})

	t.Run("requires a role granting PermViewerAccess", func(t *testing.T) {
		h := NewFilesHandler(&mockDriveClient{}, viewerAccessGuard, testDriveRoots)
		r := httptest.NewRequest(http.MethodGet, "/spl/files?folderId=abc", nil)
		// Authenticated but holds no role granting PermViewerAccess.
		r = r.WithContext(middleware.WithUserInfo(r.Context(), &middleware.UserInfo{Email: "nobody@example.com", UserID: "u-nobody"}))
		w := httptest.NewRecorder()

		h.ListFiles(w, r)

		assertStatus(t, w, http.StatusForbidden)
	})

	t.Run("rejects empty folderId with 400", func(t *testing.T) {
		h := NewFilesHandler(&mockDriveClient{}, viewerAccessGuard, testDriveRoots)
		r := withUser(httptest.NewRequest(http.MethodGet, "/spl/files?folderId=", nil))
		w := httptest.NewRecorder()

		h.ListFiles(w, r)

		assertStatus(t, w, http.StatusBadRequest)
	})

	t.Run("returns files for a folder under a configured root", func(t *testing.T) {
		want := []googledrive.DriveFile{{ID: "f1", Name: "report.pdf", MimeType: "application/pdf"}}
		var capturedFolderID string
		mock := &mockDriveClient{
			parents: testDriveTree(),
			listFilesFn: func(_ context.Context, folderID string) ([]googledrive.DriveFile, error) {
				capturedFolderID = folderID
				return want, nil
			},
		}
		h := NewFilesHandler(mock, viewerAccessGuard, testDriveRoots)
		r := withUser(httptest.NewRequest(http.MethodGet, "/spl/files?folderId=folder-1", nil))
		w := httptest.NewRecorder()

		h.ListFiles(w, r)

		assertStatus(t, w, http.StatusOK)
		if capturedFolderID != "folder-1" {
			t.Errorf("folderID = %q, want %q", capturedFolderID, "folder-1")
		}
		got := decodeJSON[[]googledrive.DriveFile](t, w)
		if len(got) != 1 || got[0].ID != "f1" {
			t.Errorf("files = %+v, want %+v", got, want)
		}
	})

	t.Run("a root itself is listable without a parents lookup", func(t *testing.T) {
		mock := &mockDriveClient{
			listFilesFn: func(context.Context, string) ([]googledrive.DriveFile, error) { return nil, nil },
		}
		h := NewFilesHandler(mock, viewerAccessGuard, testDriveRoots)
		w := httptest.NewRecorder()
		h.ListFiles(w, withUser(httptest.NewRequest(http.MethodGet, "/spl/files?folderId=root-1", nil)))
		assertStatus(t, w, http.StatusOK)
		if mock.parentCalls != 0 {
			t.Errorf("parent lookups = %d, want 0", mock.parentCalls)
		}
	})

	t.Run("a folder outside every root is 403 and never listed", func(t *testing.T) {
		listed := false
		mock := &mockDriveClient{
			parents:     testDriveTree(),
			listFilesFn: func(context.Context, string) ([]googledrive.DriveFile, error) { listed = true; return nil, nil },
		}
		h := NewFilesHandler(mock, viewerAccessGuard, testDriveRoots)
		w := httptest.NewRecorder()
		h.ListFiles(w, withUser(httptest.NewRequest(http.MethodGet, "/spl/files?folderId=outside-1", nil)))
		assertStatus(t, w, http.StatusForbidden)
		if listed {
			t.Fatal("folder outside the roots was listed")
		}
	})

	t.Run("no configured roots means nothing is listable", func(t *testing.T) {
		mock := &mockDriveClient{parents: testDriveTree()}
		h := NewFilesHandler(mock, viewerAccessGuard, nil)
		w := httptest.NewRecorder()
		h.ListFiles(w, withUser(httptest.NewRequest(http.MethodGet, "/spl/files?folderId=folder-1", nil)))
		assertStatus(t, w, http.StatusForbidden)
	})

	t.Run("a parent cycle terminates", func(t *testing.T) {
		mock := &mockDriveClient{parents: map[string][]string{"a": {"b"}, "b": {"a"}}}
		h := NewFilesHandler(mock, viewerAccessGuard, testDriveRoots)
		w := httptest.NewRecorder()
		h.ListFiles(w, withUser(httptest.NewRequest(http.MethodGet, "/spl/files?folderId=a", nil)))
		assertStatus(t, w, http.StatusForbidden)
	})

	t.Run("maps client errors to a generic failure response", func(t *testing.T) {
		mock := &mockDriveClient{
			parents: testDriveTree(),
			listFilesFn: func(_ context.Context, _ string) ([]googledrive.DriveFile, error) {
				return nil, context.DeadlineExceeded
			},
		}
		h := NewFilesHandler(mock, viewerAccessGuard, testDriveRoots)
		r := withUser(httptest.NewRequest(http.MethodGet, "/spl/files?folderId=folder-1", nil))
		w := httptest.NewRecorder()

		h.ListFiles(w, r)

		assertStatus(t, w, http.StatusInternalServerError)
	})

	t.Run("an ancestry lookup failure is a generic failure", func(t *testing.T) {
		mock := &mockDriveClient{parentsErr: context.DeadlineExceeded}
		h := NewFilesHandler(mock, viewerAccessGuard, testDriveRoots)
		w := httptest.NewRecorder()
		h.ListFiles(w, withUser(httptest.NewRequest(http.MethodGet, "/spl/files?folderId=folder-1", nil)))
		assertStatus(t, w, http.StatusInternalServerError)
	})
}

func TestSplFilesHandler_SearchFolder(t *testing.T) {
	t.Run("requires authentication", func(t *testing.T) {
		h := NewFilesHandler(&mockDriveClient{}, viewerAccessGuard, testDriveRoots)
		r := httptest.NewRequest(http.MethodGet, "/spl/files/search?folderName=Acme", nil)
		w := httptest.NewRecorder()

		h.SearchFolder(w, r)

		assertStatus(t, w, http.StatusUnauthorized)
	})

	t.Run("rejects empty folderName with 400", func(t *testing.T) {
		h := NewFilesHandler(&mockDriveClient{}, viewerAccessGuard, testDriveRoots)
		r := withUser(httptest.NewRequest(http.MethodGet, "/spl/files/search?folderName=", nil))
		w := httptest.NewRecorder()

		h.SearchFolder(w, r)

		assertStatus(t, w, http.StatusBadRequest)
	})

	t.Run("returns the first matching folder within the roots", func(t *testing.T) {
		mock := &mockDriveClient{
			parents: testDriveTree(),
			searchFoldersFn: func(_ context.Context, folderName string, limit int) ([]googledrive.DriveFolder, error) {
				if folderName != "Acme Corp" {
					t.Errorf("folderName = %q, want %q", folderName, "Acme Corp")
				}
				if limit < 2 {
					t.Errorf("limit = %d, want several candidates", limit)
				}
				return []googledrive.DriveFolder{{ID: "outside-1", Name: "Acme Corp"}, {ID: "folder-1", Name: "Acme Corp"}}, nil
			},
		}
		h := NewFilesHandler(mock, viewerAccessGuard, testDriveRoots)
		r := withUser(httptest.NewRequest(http.MethodGet, "/spl/files/search?folderName=Acme+Corp", nil))
		w := httptest.NewRecorder()

		h.SearchFolder(w, r)

		assertStatus(t, w, http.StatusOK)
		got := decodeJSON[googledrive.DriveFolder](t, w)
		if got.ID != "folder-1" {
			t.Errorf("folder = %+v, want folder-1 (the one inside the roots)", got)
		}
	})

	t.Run("only matches outside the roots is 404", func(t *testing.T) {
		mock := &mockDriveClient{
			parents: testDriveTree(),
			searchFoldersFn: func(context.Context, string, int) ([]googledrive.DriveFolder, error) {
				return []googledrive.DriveFolder{{ID: "outside-1", Name: "Acme Corp"}}, nil
			},
		}
		h := NewFilesHandler(mock, viewerAccessGuard, testDriveRoots)
		w := httptest.NewRecorder()
		h.SearchFolder(w, withUser(httptest.NewRequest(http.MethodGet, "/spl/files/search?folderName=Acme", nil)))
		assertStatus(t, w, http.StatusNotFound)
	})

	t.Run("maps ErrFolderNotFound to 404", func(t *testing.T) {
		mock := &mockDriveClient{
			searchFoldersFn: func(context.Context, string, int) ([]googledrive.DriveFolder, error) {
				return nil, googledrive.ErrFolderNotFound
			},
		}
		h := NewFilesHandler(mock, viewerAccessGuard, testDriveRoots)
		r := withUser(httptest.NewRequest(http.MethodGet, "/spl/files/search?folderName=Nope", nil))
		w := httptest.NewRecorder()

		h.SearchFolder(w, r)

		assertStatus(t, w, http.StatusNotFound)
	})
}

// Compile-time check that *googledrive.Client satisfies driveClient.
var _ driveClient = (*googledrive.Client)(nil)
