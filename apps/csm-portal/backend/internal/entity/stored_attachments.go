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
	"fmt"
	"net/http"
	"net/url"
)

// InitiateStoredAttachmentUpload calls POST /attachments/upload. Response is raw JSON.
func (c *CustomerEntityClient) InitiateStoredAttachmentUpload(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/attachments/upload", body)
}

// ConfirmStoredAttachmentUpload calls POST /attachments/upload/{id}/confirm.
// Response is raw JSON.
func (c *CustomerEntityClient) ConfirmStoredAttachmentUpload(ctx context.Context, id string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/attachments/upload/%s/confirm", url.PathEscape(id)), body)
}

// DeleteStoredAttachment calls POST /attachments/delete (204, no body).
func (c *CustomerEntityClient) DeleteStoredAttachment(ctx context.Context, body []byte) error {
	_, err := c.do(ctx, http.MethodPost, "/attachments/delete", body)
	return err
}

// The calls below are how this backend learns where a file lives: entity-service
// decides who may touch which file and says which key to use, and this backend
// moves the bytes. Their responses carry the storage key, so they are for this
// backend alone and must never be relayed.

// AuthorizeStoredAttachmentUpload calls POST /attachments/upload/{id}/authorize.
func (c *CustomerEntityClient) AuthorizeStoredAttachmentUpload(ctx context.Context, id string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/attachments/upload/%s/authorize", url.PathEscape(id)), body)
}

// LocateStoredAttachment calls POST /attachments/locate.
func (c *CustomerEntityClient) LocateStoredAttachment(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/attachments/locate", body)
}

// LocateInlineImage calls GET /attachments/inline/{id}/location.
func (c *CustomerEntityClient) LocateInlineImage(ctx context.Context, id string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/attachments/inline/%s/location", url.PathEscape(id)), nil)
}

// InlineImageRelocated calls POST /attachments/inline/{id}/relocated (204, no body):
// the file of a description image was moved to the key entity-service named.
func (c *CustomerEntityClient) InlineImageRelocated(ctx context.Context, id string, body []byte) error {
	_, err := c.do(ctx, http.MethodPost, fmt.Sprintf("/attachments/inline/%s/relocated", url.PathEscape(id)), body)
	return err
}
