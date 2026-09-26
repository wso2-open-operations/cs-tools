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

import "fmt"

// Filter is the set of optional population filters BuildOverview and
// BuildTimeseries accept: "owner/name" repo, priority label, and ABT team.
// A nil field means "unfiltered on that dimension".
type Filter struct {
	Repo     *string
	Priority *string
	AbtTeam  *string
}

// CacheKey renders f as a stable string for use in a TTLCache key, one
// field per "|"-separated segment, empty string for a nil field. Each segment
// is rendered with %q so a literal "|" or `"` inside a free-text field (e.g.
// Priority or AbtTeam) can never be mistaken for the segment delimiter.
func (f Filter) CacheKey() string {
	return fmt.Sprintf("%q|%q|%q", derefOr(f.Repo, ""), derefOr(f.Priority, ""), derefOr(f.AbtTeam, ""))
}

// derefOr returns *s, or fallback if s is nil.
func derefOr(s *string, fallback string) string {
	if s == nil {
		return fallback
	}
	return *s
}
