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
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/googledrive"
)

// driveClient abstracts the Google Drive operations used by
// FilesHandler.
type driveClient interface {
	ListFiles(ctx context.Context, folderID string) ([]googledrive.DriveFile, error)
	SearchFolders(ctx context.Context, folderName string, limit int) ([]googledrive.DriveFolder, error)
	Parents(ctx context.Context, fileID string) ([]string, error)
}

// Bounds on the folder-ancestry walk and on how many same-named folders a
// search considers.
const (
	maxDriveAncestryLookups = 64
	driveSearchCandidates   = 50
)

// FilesHandler handles HTTP requests for SupportPortalLite's Google
// Drive browsing endpoints (GET /files, GET /files/search).
//
// Both endpoints are confined to the configured root folders
// (GOOGLE_DRIVE_ROOT_FOLDER_IDS) and their descendants: the Drive account
// can usually see far more than the folders this feature is for. A folder
// outside them is answered with 403 (listing) or skipped (search); with no
// roots configured, nothing is reachable.
type FilesHandler struct {
	drive       driveClient
	accessGuard *AccessGuard
	roots       map[string]bool
}

// NewFilesHandler creates a FilesHandler backed by the given Drive client,
// confined to rootFolderIDs and their descendants. accessGuard enforces
// PermViewerAccess, SupportPortalLite's blanket audience gate.
func NewFilesHandler(drive driveClient, accessGuard *AccessGuard, rootFolderIDs []string) *FilesHandler {
	roots := make(map[string]bool, len(rootFolderIDs))
	for _, id := range rootFolderIDs {
		if id = strings.TrimSpace(id); id != "" {
			roots[id] = true
		}
	}
	return &FilesHandler{drive: drive, accessGuard: accessGuard, roots: roots}
}

// withinRoots reports whether folderID is a configured root or a descendant
// of one, walking parents breadth-first up to maxDriveAncestryLookups calls.
func (h *FilesHandler) withinRoots(ctx context.Context, folderID string) (bool, error) {
	if len(h.roots) == 0 {
		return false, nil
	}
	visited := map[string]bool{folderID: true}
	frontier := []string{folderID}
	lookups := 0
	for len(frontier) > 0 {
		id := frontier[0]
		frontier = frontier[1:]
		if h.roots[id] {
			return true, nil
		}
		if lookups >= maxDriveAncestryLookups {
			return false, nil
		}
		lookups++
		parents, err := h.drive.Parents(ctx, id)
		if err != nil {
			return false, err
		}
		for _, p := range parents {
			if !visited[p] {
				visited[p] = true
				frontier = append(frontier, p)
			}
		}
	}
	return false, nil
}

// ListFiles handles GET /files.
//
// An empty folderId is 400 (this backend's convention for a missing required
// parameter); a folder outside the configured roots is 403.
func (h *FilesHandler) ListFiles(w http.ResponseWriter, r *http.Request) {
	user, ok := requireViewerAccess(w, r, h.accessGuard)
	if !ok {
		return
	}

	folderID := strings.TrimSpace(r.URL.Query().Get("folderId"))
	if folderID == "" {
		writeError(w, http.StatusBadRequest, "folderId is required.")
		return
	}

	allowed, err := h.withinRoots(r.Context(), folderID)
	if err != nil {
		slog.ErrorContext(r.Context(), "googledrive Parents failed", "userID", user.UserID, "err", summarizeErr(err))
		mapUpstreamErrorGeneric(w, err, "Failed to list files.")
		return
	}
	if !allowed {
		writeError(w, http.StatusForbidden, ErrMsgForbidden)
		return
	}

	files, err := h.drive.ListFiles(r.Context(), folderID)
	if err != nil {
		slog.ErrorContext(r.Context(), "googledrive ListFiles failed", "userID", user.UserID, "err", summarizeErr(err))
		mapUpstreamErrorGeneric(w, err, "Failed to list files.")
		return
	}

	writeJSONValue(w, http.StatusOK, files)
}

// SearchFolder handles GET /files/search: the first folder named folderName
// that lies within the configured roots, or 404 when there is none (an empty
// folderName is 400).
func (h *FilesHandler) SearchFolder(w http.ResponseWriter, r *http.Request) {
	user, ok := requireViewerAccess(w, r, h.accessGuard)
	if !ok {
		return
	}

	folderName := strings.TrimSpace(r.URL.Query().Get("folderName"))
	if folderName == "" {
		writeError(w, http.StatusBadRequest, "folderName is required.")
		return
	}
	if len(h.roots) == 0 {
		writeError(w, http.StatusNotFound, ErrMsgNotFound)
		return
	}

	folders, err := h.drive.SearchFolders(r.Context(), folderName, driveSearchCandidates)
	if err != nil {
		if errors.Is(err, googledrive.ErrFolderNotFound) {
			writeError(w, http.StatusNotFound, ErrMsgNotFound)
			return
		}
		slog.ErrorContext(r.Context(), "googledrive SearchFolder failed", "userID", user.UserID, "err", summarizeErr(err))
		mapUpstreamErrorGeneric(w, err, "Failed to search folder.")
		return
	}

	for _, folder := range folders {
		allowed, err := h.withinRoots(r.Context(), folder.ID)
		if err != nil {
			slog.ErrorContext(r.Context(), "googledrive Parents failed", "userID", user.UserID, "err", summarizeErr(err))
			mapUpstreamErrorGeneric(w, err, "Failed to search folder.")
			return
		}
		if allowed {
			writeJSONValue(w, http.StatusOK, folder)
			return
		}
	}
	writeError(w, http.StatusNotFound, ErrMsgNotFound)
}
