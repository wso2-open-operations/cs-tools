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
	"time"
)

// PortalBannerView is the entity-service banner a portal may render.
// Status and the visibility flags are already applied by GET /banners.
type PortalBannerView struct {
	ID        string     `json:"id"`
	SortOrder int        `json:"sortOrder"`
	HTML      string     `json:"html"`
	StartsAt  *time.Time `json:"startsAt"`
	ExpiresAt *time.Time `json:"expiresAt"`
}

// PortalBannerViewList is the body of GET /banners.
type PortalBannerViewList struct {
	Banners []PortalBannerView `json:"banners"`
}

// ListCustomerBanners calls GET /banners. Entity-service returns only published
// banners flagged for the customer portal.
func (c *Client) ListCustomerBanners(ctx context.Context) (PortalBannerViewList, error) {
	var out PortalBannerViewList
	if err := c.getJSON(ctx, "/banners", &out); err != nil {
		return PortalBannerViewList{}, err
	}
	return out, nil
}
