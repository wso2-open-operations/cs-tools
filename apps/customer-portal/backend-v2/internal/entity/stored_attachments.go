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

// The stored-attachment routes are entity-service's SFTPGo-backed attachment
// API. Bodies are relayed as raw JSON: entity-service validates them and
// decides access (Postgres row-level security), so nothing here interprets them.

// ResolveStoredAttachmentLinks calls POST /attachments/download.
func (c *Client) ResolveStoredAttachmentLinks(ctx context.Context, body json.RawMessage) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/attachments/download", body)
}

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

// StreamInlineImage calls GET /attachments/inline/{id}/content and returns the
// response body unread, so a caller can copy it straight to its own response
// without holding the image in memory. The caller closes the body. size is -1
// when entity-service did not send a length.
func (c *Client) StreamInlineImage(ctx context.Context, id string) (body io.ReadCloser, contentType string, size int64, err error) {
	path := fmt.Sprintf("/attachments/inline/%s/content", url.PathEscape(id))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, "", 0, fmt.Errorf("entity: build request GET %s: %w", path, err)
	}
	if token := userIDTokenFromContext(ctx); token != "" {
		req.Header.Set("x-user-id-token", token)
	}
	if cid := correlationIDFromContext(ctx); cid != "" {
		req.Header.Set("X-CSM-Correlation-ID", cid)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", 0, fmt.Errorf("entity: GET %s: %w", path, err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		defer resp.Body.Close()
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodyBytes))
		return nil, "", 0, apierror.NewUpstreamError(resp.StatusCode, errBody)
	}
	return resp.Body, resp.Header.Get("Content-Type"), resp.ContentLength, nil
}
