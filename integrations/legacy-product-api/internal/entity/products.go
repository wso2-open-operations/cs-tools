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
)

// productVersionsPageSize is the page size of POST /product-versions/search,
// entity-service's maximum for that route, so a full feed (about 450 rows)
// takes a single call.
const productVersionsPageSize = 1000

// maxProductVersionPages stops a paging loop that an upstream bug keeps
// telling to continue.
const maxProductVersionPages = 50

type productVersionsSearch struct {
	Pagination struct {
		Limit  int `json:"limit"`
		Offset int `json:"offset"`
	} `json:"pagination"`
	Feed           string `json:"feed,omitempty"`
	ExcludeRetired bool   `json:"excludeRetired"`
}

type productVersionsPage struct {
	ProductVersions []json.RawMessage `json:"productVersions"`
	HasMore         bool              `json:"hasMore"`
}

// ListProductVersions reads every live product version row on feed
// ("support-matrix", "release-matrix", or "" for all live rows) through
// POST /product-versions/search, following hasMore across pages. It returns
// {"productVersions":[...]} with the rows exactly as entity-service sent them.
func (c *Client) ListProductVersions(ctx context.Context, feed string) ([]byte, error) {
	rows := []json.RawMessage{}
	req := productVersionsSearch{Feed: feed, ExcludeRetired: true}
	req.Pagination.Limit = productVersionsPageSize
	for page := 0; ; page++ {
		if page == maxProductVersionPages {
			return nil, fmt.Errorf("entity: product versions: more than %d pages", maxProductVersionPages)
		}
		body, err := json.Marshal(req)
		if err != nil {
			return nil, fmt.Errorf("entity: product versions: encode request: %w", err)
		}
		resp, err := c.do(ctx, http.MethodPost, "/product-versions/search", body)
		if err != nil {
			return nil, err
		}
		var p productVersionsPage
		if err := json.Unmarshal(resp, &p); err != nil {
			return nil, fmt.Errorf("entity: product versions: decode page: %w", err)
		}
		rows = append(rows, p.ProductVersions...)
		if !p.HasMore || len(p.ProductVersions) == 0 {
			break
		}
		req.Pagination.Offset += len(p.ProductVersions)
	}
	return json.Marshal(struct {
		ProductVersions []json.RawMessage `json:"productVersions"`
	}{rows})
}

// ListReleaseLines calls GET /release-lines. includeRetired adds retired
// lines, which a live version can still point at.
func (c *Client) ListReleaseLines(ctx context.Context, includeRetired bool) ([]byte, error) {
	path := "/release-lines"
	if includeRetired {
		path += "?includeRetired=true"
	}
	return c.do(ctx, http.MethodGet, path, nil)
}

// ListCarbonPlatformHistory calls GET /carbon-platform-history.
func (c *Client) ListCarbonPlatformHistory(ctx context.Context) ([]byte, error) {
	return c.do(ctx, http.MethodGet, "/carbon-platform-history", nil)
}

// ListCarbonKernelReleases calls GET /carbon-kernel-releases.
func (c *Client) ListCarbonKernelReleases(ctx context.Context) ([]byte, error) {
	return c.do(ctx, http.MethodGet, "/carbon-kernel-releases", nil)
}
