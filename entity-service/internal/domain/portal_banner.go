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

package domain

import (
	"regexp"
	"strings"
	"time"
)

// bannerIDAttribute is the copy of the primary key kept on the banner root.
var bannerIDAttribute = regexp.MustCompile(`\sdata-id="[^"]*"`)

// StampBannerID writes the row's primary key into the banner HTML. Dismissal
// and Edit then use the same id the database stored.
func StampBannerID(html, id string) string {
	if id == "" || !strings.Contains(html, `data-banner="1"`) {
		return html
	}
	without := bannerIDAttribute.ReplaceAllString(html, "")
	return strings.Replace(without, `data-banner="1"`, `data-banner="1" data-id="`+id+`"`, 1)
}

// Portal banner statuses. Only published is eligible to show, and only on a
// portal whose visibility flag is true.
const (
	PortalBannerDraft       = "draft"
	PortalBannerPublished   = "published"
	PortalBannerUnpublished = "unpublished"
)

// PortalBanner is one row of portal_banner, as the Settings page lists it.
type PortalBanner struct {
	ID                      string     `json:"id"`
	Status                  string     `json:"status"`
	SortOrder               int        `json:"sortOrder"`
	VisibleInCustomerPortal bool       `json:"visibleInCustomerPortal"`
	VisibleInCsmPortal      bool       `json:"visibleInCsmPortal"`
	HTML                    string     `json:"html"`
	StartsAt                *time.Time `json:"startsAt"`
	ExpiresAt               *time.Time `json:"expiresAt"`
	CreatedOn               time.Time  `json:"createdOn"`
	CreatedBy               string     `json:"createdBy"`
	UpdatedOn               time.Time  `json:"updatedOn"`
	UpdatedBy               string     `json:"updatedBy"`
}

// PortalBannerList is the body of GET /portal-banners/manage.
type PortalBannerList struct {
	Banners []PortalBanner `json:"banners"`
}

// PortalBannerView is a banner a portal may render. Status and the visibility
// flags are already applied by the route that returned it.
type PortalBannerView struct {
	ID        string     `json:"id"`
	SortOrder int        `json:"sortOrder"`
	HTML      string     `json:"html"`
	StartsAt  *time.Time `json:"startsAt"`
	ExpiresAt *time.Time `json:"expiresAt"`
}

// PortalBannerViewList is the body of GET /banners and GET /portal-banners.
type PortalBannerViewList struct {
	Banners []PortalBannerView `json:"banners"`
}

// SavePortalBannerRequest is the body of POST /portal-banners and PUT /portal-banners/{id}.
type SavePortalBannerRequest struct {
	Status                  string     `json:"status"`
	SortOrder               int        `json:"sortOrder"`
	VisibleInCustomerPortal bool       `json:"visibleInCustomerPortal"`
	VisibleInCsmPortal      bool       `json:"visibleInCsmPortal"`
	HTML                    string     `json:"html"`
	StartsAt                *time.Time `json:"startsAt"`
	ExpiresAt               *time.Time `json:"expiresAt"`
}

// View is the portal-facing shape of a banner.
func (b PortalBanner) View() PortalBannerView {
	return PortalBannerView{
		ID:        b.ID,
		SortOrder: b.SortOrder,
		HTML:      b.HTML,
		StartsAt:  b.StartsAt,
		ExpiresAt: b.ExpiresAt,
	}
}
