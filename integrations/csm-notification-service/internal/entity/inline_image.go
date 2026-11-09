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

	"github.com/wso2-open-operations/cs-tools/integrations/csm-notification-service/internal/apierror"
)

// InlineImage is an inline comment image streamed from entity-service. The caller
// must close Body.
type InlineImage struct {
	// ContentType is the media type entity-service serves it as: one of the four
	// image types, or application/octet-stream for anything else.
	ContentType string
	// ContentLength is the file's size, or -1 when the response did not say.
	ContentLength int64
	Body          io.ReadCloser
}

// ReadInlineImage calls GET /attachments/inline/{id}/content?workItemId= as this
// service's own (internal) caller. Entity-service is the only service that reads
// the file store, and it returns the image only when it is linked to workItemID
// (never another record's, never a draft): this service is an internal caller that
// no access rule filters, so that check is what keeps another customer's image out
// of an email. workItemID must not be empty, or entity-service would not filter at
// all. A removed, unknown or foreign image is an *apierror.Error with the status
// only.
func (c *CustomerEntityClient) ReadInlineImage(ctx context.Context, id, workItemID string) (*InlineImage, error) {
	if workItemID == "" {
		return nil, fmt.Errorf("entity: ReadInlineImage needs the record the image must belong to")
	}
	path := "/attachments/inline/" + url.PathEscape(id) + "/content?workItemId=" + url.QueryEscape(workItemID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("entity: build request GET inline image: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("entity: GET inline image: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		// The body is never kept: it names the image and the record.
		return nil, &apierror.Error{StatusCode: resp.StatusCode, Body: "response omitted"}
	}
	return &InlineImage{ContentType: resp.Header.Get("Content-Type"), ContentLength: resp.ContentLength, Body: resp.Body}, nil
}
