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

// bannerOrder is one row's place in the stack.
type bannerOrder struct {
	ID          string
	SortOrder   int
	Unpublished bool
}

// assignBannerOrder puts id into the stack and rewrites every place as
// 0, 1, 2, … with no gaps. An unpublished banner sits after the stack. A
// banner that is not unpublished is inserted at order, and the banners at
// that place and below move down by one.
func assignBannerOrder(rows []bannerOrder, id string, order int, unpublished bool) []bannerOrder {
	active := make([]bannerOrder, 0, len(rows)+1)
	parked := make([]bannerOrder, 0)
	for _, row := range rows {
		if row.ID == id {
			continue
		}
		if row.Unpublished {
			parked = append(parked, row)
		} else {
			active = append(active, row)
		}
	}
	target := bannerOrder{ID: id, Unpublished: unpublished}
	if unpublished {
		parked = append(parked, target)
	} else {
		if order < 0 {
			order = 0
		}
		if order > len(active) {
			order = len(active)
		}
		active = append(active, bannerOrder{})
		copy(active[order+1:], active[order:])
		active[order] = target
	}
	out := append(active, parked...)
	for i := range out {
		out[i].SortOrder = i
	}
	return out
}
