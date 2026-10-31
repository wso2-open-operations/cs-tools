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

package dto

import (
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

// PortalBannerView is one banner the customer portal may render.
// Fields outside this set stay on the entity-service response.
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

// MapPortalBannerViewList copies only the fields the customer portal renders.
func MapPortalBannerViewList(in entity.PortalBannerViewList) PortalBannerViewList {
	banners := make([]PortalBannerView, 0, len(in.Banners))
	for _, banner := range in.Banners {
		banners = append(banners, PortalBannerView{
			ID:        banner.ID,
			SortOrder: banner.SortOrder,
			HTML:      banner.HTML,
			StartsAt:  banner.StartsAt,
			ExpiresAt: banner.ExpiresAt,
		})
	}
	return PortalBannerViewList{Banners: banners}
}
