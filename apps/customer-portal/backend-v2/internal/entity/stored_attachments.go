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
	"net/http"
	"net/url"
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

// The three calls below are how entity-service decides who may touch which file
// and says where it lives; this backend moves the bytes. Their responses carry
// the storage key, so they are for this backend alone and must never be relayed.

// AuthorizeStoredAttachmentUpload calls POST /attachments/upload/{id}/authorize.
func (c *Client) AuthorizeStoredAttachmentUpload(ctx context.Context, id string, body json.RawMessage) ([]byte, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/attachments/upload/%s/authorize", url.PathEscape(id)), body)
}

// LocateStoredAttachment calls POST /attachments/locate.
func (c *Client) LocateStoredAttachment(ctx context.Context, body json.RawMessage) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/attachments/locate", body)
}

// LocateInlineImage calls GET /attachments/inline/{id}/location.
func (c *Client) LocateInlineImage(ctx context.Context, id string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/attachments/inline/%s/location", url.PathEscape(id)), nil)
}
