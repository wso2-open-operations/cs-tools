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
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

func TestMapPortalBannerViewListKeepsTheViewAndDropsAnythingElse(t *testing.T) {
	raw := []byte(`{"banners":[{"id":"11111111-1111-4111-8111-111111111111","sortOrder":2,"html":"<p>Hi</p>","startsAt":null,"expiresAt":"2026-01-02T00:00:00Z","status":"published","createdBy":"secret"},{"id":"later","sortOrder":0,"html":"x","startsAt":"2099-01-01T00:00:00Z","expiresAt":null}]}`)
	var in entity.PortalBannerViewList
	if err := json.Unmarshal(raw, &in); err != nil {
		t.Fatal(err)
	}
	out, err := json.Marshal(MapPortalBannerViewList(in))
	if err != nil {
		t.Fatal(err)
	}
	body := string(out)
	if strings.Contains(body, "createdBy") || strings.Contains(body, "status") || strings.Contains(body, "secret") {
		t.Fatalf("response kept a field outside the view: %s", body)
	}
	var decoded PortalBannerViewList
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Banners) != 2 {
		t.Fatalf("banners = %d", len(decoded.Banners))
	}
	if decoded.Banners[0].ExpiresAt == nil || !decoded.Banners[0].ExpiresAt.Equal(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("expiresAt = %v", decoded.Banners[0].ExpiresAt)
	}
	if decoded.Banners[0].StartsAt != nil {
		t.Fatal("startsAt was set")
	}
}

func TestMapPortalBannerViewListWritesAnEmptyList(t *testing.T) {
	out, err := json.Marshal(MapPortalBannerViewList(entity.PortalBannerViewList{}))
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `{"banners":[]}` {
		t.Fatalf("body = %s", out)
	}
}
