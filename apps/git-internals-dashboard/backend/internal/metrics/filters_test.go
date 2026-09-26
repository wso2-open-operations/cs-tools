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

package metrics

import "testing"

// TestFilterCacheKeyNoDelimiterCollision covers two (priority, abtTeam) pairs
// that would produce the same joined string under a naive "|"-concatenation
// scheme (one field's value bleeds into where the other's would start), and
// asserts CacheKey still tells them apart.
func TestFilterCacheKeyNoDelimiterCollision(t *testing.T) {
	priority1, abtTeam1 := "x", "y|"
	priority2, abtTeam2 := "x|y", ""

	f1 := Filter{Priority: &priority1, AbtTeam: &abtTeam1}
	f2 := Filter{Priority: &priority2, AbtTeam: &abtTeam2}

	if f1.CacheKey() == f2.CacheKey() {
		t.Fatalf("CacheKey collision: %q produced by both %+v and %+v", f1.CacheKey(), f1, f2)
	}
}
