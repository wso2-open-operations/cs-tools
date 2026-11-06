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
	"net/http"
	"net/url"
)

// ListCSMBanners calls GET /portal-banners. Entity-service returns only
// published banners flagged for the CSM portal.
func (c *CustomerEntityClient) ListCSMBanners(ctx context.Context) ([]byte, error) {
	return c.do(ctx, http.MethodGet, "/portal-banners", nil)
}

// ListManagedBanners calls GET /portal-banners/manage.
func (c *CustomerEntityClient) ListManagedBanners(ctx context.Context) ([]byte, error) {
	return c.do(ctx, http.MethodGet, "/portal-banners/manage", nil)
}

// CreatePortalBanner calls POST /portal-banners.
func (c *CustomerEntityClient) CreatePortalBanner(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/portal-banners", body)
}

// UpdatePortalBanner calls PUT /portal-banners/{id}.
func (c *CustomerEntityClient) UpdatePortalBanner(ctx context.Context, id string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPut, "/portal-banners/"+url.PathEscape(id), body)
}
