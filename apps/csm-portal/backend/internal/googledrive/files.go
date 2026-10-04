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

package googledrive

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
)

// ErrFolderNotFound is returned by SearchFolder when no folder matches the
// given name — mirrors the Ballerina searchFolder's
// utils:getHttpNotFoundRequestResponse fallback.
var ErrFolderNotFound = errors.New("googledrive: no folder found with that name")

// DriveFile is a Google Drive file's metadata — mirrors Ballerina
// modules/types.bal's DriveFile (a portal-owned, reshaped response, not raw
// Drive API passthrough, matching what google_drive.bal already did).
type DriveFile struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	MimeType string `json:"mimeType"`
}

// DriveFolder is a Google Drive folder's metadata — mirrors Ballerina
// modules/types.bal's DriveFolder.
type DriveFolder struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// driveFile is the wire shape of one entry in a Drive v3 files.list response.
type driveFile struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	MimeType string `json:"mimeType"`
}

type filesListResponse struct {
	Files         []driveFile `json:"files"`
	NextPageToken string      `json:"nextPageToken"`
}

// ListFiles lists every file directly inside the Drive folder identified by
// folderID (non-recursive, trashed files excluded), paginating through the
// full result set — mirrors the Ballerina listFiles, which streamed every
// page via the connector's getAllFiles.
func (c *Client) ListFiles(ctx context.Context, folderID string) ([]DriveFile, error) {
	q := fmt.Sprintf("'%s' in parents and trashed=false", escapeDriveQueryValue(folderID))

	var results []DriveFile
	pageToken := ""
	for {
		params := url.Values{
			"q":        {q},
			"fields":   {"nextPageToken,files(id,name,mimeType)"},
			"pageSize": {"1000"},
			"spaces":   {"drive"},
		}
		if pageToken != "" {
			params.Set("pageToken", pageToken)
		}

		var page filesListResponse
		if err := c.get(ctx, "/files", params, &page); err != nil {
			return nil, fmt.Errorf("googledrive: list files in folder %q: %w", folderID, err)
		}

		for _, f := range page.Files {
			results = append(results, DriveFile(f))
		}

		if page.NextPageToken == "" {
			break
		}
		pageToken = page.NextPageToken
	}

	return results, nil
}

// SearchFolders returns up to limit Drive folders whose name matches
// folderName exactly, in the API's order. Returns ErrFolderNotFound when none
// match. The caller picks the first one it is allowed to show.
func (c *Client) SearchFolders(ctx context.Context, folderName string, limit int) ([]DriveFolder, error) {
	q := fmt.Sprintf("mimeType='application/vnd.google-apps.folder' and name='%s' and trashed=false",
		escapeDriveQueryValue(folderName))

	var page filesListResponse
	params := url.Values{
		"q":        {q},
		"fields":   {"files(id,name)"},
		"pageSize": {fmt.Sprint(limit)},
		"spaces":   {"drive"},
	}
	if err := c.get(ctx, "/files", params, &page); err != nil {
		return nil, fmt.Errorf("googledrive: search folder %q: %w", folderName, err)
	}

	if len(page.Files) == 0 {
		return nil, ErrFolderNotFound
	}
	folders := make([]DriveFolder, 0, len(page.Files))
	for _, f := range page.Files {
		folders = append(folders, DriveFolder{ID: f.ID, Name: f.Name})
	}
	return folders, nil
}

// Parents returns the ids of the folders directly containing fileID (empty
// for a top-level item).
func (c *Client) Parents(ctx context.Context, fileID string) ([]string, error) {
	var meta struct {
		Parents []string `json:"parents"`
	}
	params := url.Values{"fields": {"parents"}, "supportsAllDrives": {"true"}}
	if err := c.get(ctx, "/files/"+url.PathEscape(fileID), params, &meta); err != nil {
		return nil, fmt.Errorf("googledrive: parents of %q: %w", fileID, err)
	}
	return meta.Parents, nil
}

// get performs an authenticated GET against the Drive v3 API and decodes
// the JSON response into out.
func (c *Client) get(ctx context.Context, path string, params url.Values, out any) error {
	reqURL := driveAPIBaseURL + path + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return fmt.Errorf("build request GET %s: %w", path, err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxUpstreamResponseBytes+1))
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}
	if len(body) > maxUpstreamResponseBytes {
		return fmt.Errorf("read response body: response exceeds %d bytes", maxUpstreamResponseBytes)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		const maxErrBody = 256
		excerpt := body
		if len(excerpt) > maxErrBody {
			excerpt = excerpt[:maxErrBody]
		}
		return &apierror.Error{StatusCode: resp.StatusCode, Body: string(excerpt)}
	}

	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}
