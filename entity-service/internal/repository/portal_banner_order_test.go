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

package repository

import "testing"

func TestAssignBannerOrder(t *testing.T) {
	rows := []bannerOrder{{ID: "a", SortOrder: 0}, {ID: "b", SortOrder: 1}}

	first := assignBannerOrder(nil, "a", 0, false)
	if len(first) != 1 || first[0].SortOrder != 0 {
		t.Fatalf("first banner = %+v", first)
	}

	under := assignBannerOrder(rows, "c", 2, false)
	if orderOf(under, "a") != 0 || orderOf(under, "b") != 1 || orderOf(under, "c") != 2 {
		t.Fatalf("append = %+v", under)
	}

	above := assignBannerOrder(rows, "c", 0, false)
	if orderOf(above, "c") != 0 || orderOf(above, "a") != 1 || orderOf(above, "b") != 2 {
		t.Fatalf("insert at 0 = %+v", above)
	}

	same := assignBannerOrder(rows, "b", 1, false)
	if orderOf(same, "a") != 0 || orderOf(same, "b") != 1 {
		t.Fatalf("keeping its own order shifted the stack: %+v", same)
	}

	moved := assignBannerOrder(rows, "b", 0, false)
	if orderOf(moved, "b") != 0 || orderOf(moved, "a") != 1 {
		t.Fatalf("move to 0 = %+v", moved)
	}

	three := []bannerOrder{{ID: "a", SortOrder: 0}, {ID: "b", SortOrder: 1}, {ID: "c", SortOrder: 2}}
	unpublished := assignBannerOrder(three, "b", 1, true)
	if orderOf(unpublished, "a") != 0 || orderOf(unpublished, "c") != 1 || orderOf(unpublished, "b") != 2 || !unpublishedBanner(unpublished, "b") {
		t.Fatalf("unpublish closes the gap: %+v", unpublished)
	}
	for i, row := range unpublished {
		if row.SortOrder != i {
			t.Fatalf("orders are not consecutive: %+v", unpublished)
		}
	}
}

func unpublishedBanner(rows []bannerOrder, id string) bool {
	for _, row := range rows {
		if row.ID == id {
			return row.Unpublished
		}
	}
	return false
}

func orderOf(rows []bannerOrder, id string) int {
	for _, row := range rows {
		if row.ID == id {
			return row.SortOrder
		}
	}
	return -1
}
