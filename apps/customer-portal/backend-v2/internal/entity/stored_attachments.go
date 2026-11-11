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

package entity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/apierror"
)

// The stored-attachment routes are entity-service's attachment API. Bodies are relayed as raw JSON: entity-service validates them and
// decides access (Postgres row-level security), so nothing here interprets them.

// InitiateStoredAttachmentUpload calls POST /attachments/upload.
func (c *Client) InitiateStoredAttachmentUpload(ctx context.Context, body json.RawMessage) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/attachments/upload", body)
}

// ConfirmStoredAttachmentUpload calls POST /attachments/upload/{id}/confirm.
func (c *Client) ConfirmStoredAttachmentUpload(ctx context.Context, id string, body json.RawMessage) ([]byte, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/attachments/upload/%s/confirm", url.PathEscape(id)), body)
}

// DeleteStoredAttachment calls POST /attachments/delete (204, no body).
func (c *Client) DeleteStoredAttachment(ctx context.Context, body json.RawMessage) error {
	_, err := c.do(ctx, http.MethodPost, "/attachments/delete", body)
	return err
}

// The calls below move the attachment files. Entity-service is the only service
// that talks to Azure Blob Storage: this backend forwards what the browser sent
// and streams the answer back. Entity-service runs the caller's access check for
// every call, using the user token forwarded with the request.

// maxFileErrorBody caps how much of an entity-service error body is kept.
const maxFileErrorBody = 4096

// FileResponse is a file streamed from entity-service. The caller must close Body.
type FileResponse struct {
	StatusCode int
	Header     http.Header
	Body       io.ReadCloser
}

// RangeNotSatisfiableError is entity-service's 416: the Range asked for bytes the
// file does not have. ContentRange ("bytes */<size>") is forwarded to the browser.
type RangeNotSatisfiableError struct{ ContentRange string }

// Error implements the error interface.
func (e *RangeNotSatisfiableError) Error() string { return "entity: range not satisfiable" }

// fileRequest builds an authenticated request for the file client.
func (c *Client) fileRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("entity: build request %s %s: %w", method, path, err)
	}
	if token := userIDTokenFromContext(ctx); token != "" {
		req.Header.Set("x-user-id-token", token)
	}
	if id := correlationIDFromContext(ctx); id != "" {
		req.Header.Set("X-CSM-Correlation-ID", id)
	}
	return req, nil
}

// failure turns a non-2xx response into an error and closes its body.
func failure(resp *http.Response) error {
	defer resp.Body.Close()
	excerpt, _ := io.ReadAll(io.LimitReader(resp.Body, maxFileErrorBody))
	if resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
		return &RangeNotSatisfiableError{ContentRange: resp.Header.Get("Content-Range")}
	}
	return apierror.NewUpstreamError(resp.StatusCode, excerpt)
}

// PutStoredAttachmentChunk calls PUT /attachments/upload/{id}/chunks/{index}?type=
// with the chunk's bytes (204, no body). size is the chunk's length.
func (c *Client) PutStoredAttachmentChunk(ctx context.Context, id, kind string, index int, size int64, body io.Reader) error {
	path := fmt.Sprintf("/attachments/upload/%s/chunks/%d?type=%s", url.PathEscape(id), index, url.QueryEscape(kind))
	req, err := c.fileRequest(ctx, http.MethodPut, path, body)
	if err != nil {
		return err
	}
	req.ContentLength = size
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, err := c.fileHTTP.Do(req)
	if err != nil {
		return fmt.Errorf("entity: PUT chunk: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return failure(resp)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.Body.Close()
}

// ListStoredAttachmentChunks calls GET /attachments/upload/{id}/chunks?type=.
// Response is raw JSON.
func (c *Client) ListStoredAttachmentChunks(ctx context.Context, id, kind string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/attachments/upload/%s/chunks?type=%s", url.PathEscape(id), url.QueryEscape(kind)), nil)
}

// StoredAttachmentLimits calls GET /attachments/limits: the largest file and the
// largest total of inline images entity-service accepts. Response is raw JSON.
func (c *Client) StoredAttachmentLimits(ctx context.Context) ([]byte, error) {
	return c.do(ctx, http.MethodGet, "/attachments/limits", nil)
}

// DirectStoredAttachmentUploadLinks calls POST /attachments/{id}/upload-links?type=:
// one signed link per chunk, for a large file to go straight to Azure. Response is
// raw JSON, and the links in it are credentials.
func (c *Client) DirectStoredAttachmentUploadLinks(ctx context.Context, id, kind string) ([]byte, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/attachments/%s/upload-links?type=%s", url.PathEscape(id), url.QueryEscape(kind)), nil)
}

// DirectStoredAttachmentDownloadLink calls POST /attachments/{id}/download-links?type=:
// a signed link to download a large file straight from Azure. Response is raw
// JSON, and the link in it is a credential.
func (c *Client) DirectStoredAttachmentDownloadLink(ctx context.Context, id, kind string) ([]byte, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/attachments/%s/download-links?type=%s", url.PathEscape(id), url.QueryEscape(kind)), nil)
}

// DownloadStoredAttachment calls GET /attachments/download/{id}/content?type=,
// passing a Range header on. The caller streams and closes the body.
func (c *Client) DownloadStoredAttachment(ctx context.Context, kind, id, rangeHeader string) (*FileResponse, error) {
	path := fmt.Sprintf("/attachments/download/%s/content?type=%s", url.PathEscape(id), url.QueryEscape(kind))
	return c.openFile(ctx, path, rangeHeader)
}

// InlineImageContent calls GET /attachments/inline/{id}/content. The caller
// streams and closes the body.
func (c *Client) InlineImageContent(ctx context.Context, id string) (*FileResponse, error) {
	return c.openFile(ctx, fmt.Sprintf("/attachments/inline/%s/content", url.PathEscape(id)), "")
}

// openFile issues a GET for a file and returns the open response.
func (c *Client) openFile(ctx context.Context, path, rangeHeader string) (*FileResponse, error) {
	req, err := c.fileRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	if rangeHeader != "" {
		req.Header.Set("Range", rangeHeader)
	}
	resp, err := c.fileHTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("entity: GET file: %w", err)
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return nil, failure(resp)
	}
	return &FileResponse{StatusCode: resp.StatusCode, Header: resp.Header, Body: resp.Body}, nil
}
