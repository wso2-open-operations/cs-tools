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

// InlineImageLocation is where entity-service says an inline image's file is.
// StorageKey is internal: it is used to read the file and never leaves this
// process.
type InlineImageLocation struct {
	Available   bool    `json:"available"`
	ContentType *string `json:"contentType"`
	SizeBytes   *int64  `json:"sizeBytes"`
	StorageKey  string  `json:"storageKey"`
	// ParentID is the work item the image belongs to, or the project while Draft
	// is true. Empty from an entity-service that does not send it yet.
	ParentID string `json:"parentId"`
	Draft    bool   `json:"draft"`
}

// LocateInlineImage calls GET /attachments/inline/{id}/location as this
// service's own (internal) caller, so the lookup is not limited to one
// customer's rows. A removed or unknown image is an *apierror.Error with the
// status only.
func (c *CustomerEntityClient) LocateInlineImage(ctx context.Context, id string) (InlineImageLocation, error) {
	body, err := c.do(ctx, http.MethodGet, "/attachments/inline/"+url.PathEscape(id)+"/location", nil)
	if err != nil {
		return InlineImageLocation{}, err
	}
	var loc InlineImageLocation
	if err := json.Unmarshal(body, &loc); err != nil {
		return InlineImageLocation{}, fmt.Errorf("entity: decode LocateInlineImage response: %w", err)
	}
	return loc, nil
}
