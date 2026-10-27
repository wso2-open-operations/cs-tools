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
	"io"
	"net/http"
	"net/url"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
)

// StoredAttachmentLinks calls POST /attachments/download: metadata and a
// short-lived link for each attachment file held in SFTPGo. Response is raw JSON.
func (c *CustomerEntityClient) StoredAttachmentLinks(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/attachments/download", body)
}

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

// InlineImageStream is a comment image being streamed from the entity service.
// The caller closes Body. Length is -1 when the upstream did not say.
type InlineImageStream struct {
	Body        io.ReadCloser
	ContentType string
	Length      int64
}

// OpenInlineImage calls GET /attachments/inline/{id}/content and returns the
// response body unread, so a large image is streamed through instead of being
// held in memory. A non-2xx answer is returned as an *apierror.Error.
func (c *CustomerEntityClient) OpenInlineImage(ctx context.Context, id string) (*InlineImageStream, error) {
	path := fmt.Sprintf("/attachments/inline/%s/content", url.PathEscape(id))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("entity: build request GET %s: %w", path, err)
	}
	if token := UserIDTokenFromContext(ctx); token != "" {
		req.Header.Set("x-user-id-token", token)
	}
	if cid := correlationIDFromContext(ctx); cid != "" {
		req.Header.Set("X-CSM-Correlation-ID", cid)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("entity: GET %s: %w", path, err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		defer resp.Body.Close()
		excerpt, _ := io.ReadAll(io.LimitReader(resp.Body, maxEntityErrBody))
		return nil, &apierror.Error{StatusCode: resp.StatusCode, Body: string(excerpt)}
	}
	return &InlineImageStream{Body: resp.Body, ContentType: resp.Header.Get("Content-Type"), Length: resp.ContentLength}, nil
}
