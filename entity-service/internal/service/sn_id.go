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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package service

import (
	"fmt"
	"strconv"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
)

// maxExactNumberLen bounds an exact-match record-number filter
// (incident/problem/change-request `number`) before it reaches the
// ServiceNow client. Deliberately a length cap only, not a format/prefix
// check: the case search's own number/internalId filters (case_filters.go)
// don't validate format either, and every entity type's real number prefix
// is an SN configuration detail this layer shouldn't hardcode assumptions
// about.
const maxExactNumberLen = 50

// validateExactNumber checks an optional exact-match number filter value,
// returning nil for an absent (nil) filter. Mirrors validateSearchQuery's
// length-cap style (user_service.go) for the free-text path.
func validateExactNumber(fieldName string, number *string) error {
	if number == nil {
		return nil
	}
	if utf8.RuneCountInString(*number) > maxExactNumberLen {
		return &apierror.ValidationError{Msg: fmt.Sprintf("%s cannot exceed %d characters", fieldName, maxExactNumberLen)}
	}
	return nil
}

// sysidToUUID converts a 32-character ServiceNow sysid to a standard UUID by
// inserting hyphens at the canonical 8-4-4-4-12 positions.
// Returns the input unchanged if it is not exactly 32 hex characters.
func sysidToUUID(sysid string) string {
	if len(sysid) != 32 || !isHex(sysid) {
		return sysid
	}
	return sysid[0:8] + "-" + sysid[8:12] + "-" + sysid[12:16] + "-" + sysid[16:20] + "-" + sysid[20:32]
}

// uuidToSysid converts a canonical UUID string (xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx)
// to a 32-character ServiceNow sysid by stripping the hyphens.
// Returns the input unchanged if it is not a canonical UUID.
func uuidToSysid(uuid string) string {
	if !isCanonicalUUID(uuid) {
		return uuid
	}
	return uuid[0:8] + uuid[9:13] + uuid[14:18] + uuid[19:23] + uuid[24:36]
}

// snParentIDFilter converts an optional parentId filter UUID to a sysid,
// returning "" (omitted via omitempty) when nil.
func snParentIDFilter(uuid *string) string {
	if uuid == nil {
		return ""
	}
	return uuidToSysid(*uuid)
}

// stringPtrValue dereferences an optional string filter value, returning ""
// (omitted via omitempty) when nil. Unlike snParentIDFilter, no sysid
// conversion applies: number and internalId are opaque ServiceNow values,
// not UUIDs.
func stringPtrValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// uuidsToSysids converts a slice of UUID strings to sysids.
// Returns the original slice unchanged if it is empty.
func uuidsToSysids(uuids []string) []string {
	if len(uuids) == 0 {
		return uuids
	}
	out := make([]string, len(uuids))
	for i, u := range uuids {
		out[i] = uuidToSysid(u)
	}
	return out
}

// snIncidentStateKeysFromStrings converts parsedIncidentFilters.
// IncidentStateKeys (already validated as non-negative integers by
// parseIncidentFilterNonNegativeIntString in incident_filters.go) into the
// raw ServiceNow numeric keys snIncidentFilters.IncidentStateKeys expects.
// This is the one place that conversion happens -- incident_filters.go's
// parsedIncidentFilters stays data-source-agnostic and never commits to
// Go's int type for a ServiceNow-only raw value. The strconv.Atoi error is
// ignored: every value has already been validated at parse time.
func snIncidentStateKeysFromStrings(keys []string) []int {
	if len(keys) == 0 {
		return nil
	}
	out := make([]int, len(keys))
	for i, k := range keys {
		n, _ := strconv.Atoi(k)
		out[i] = n
	}
	return out
}

// isHex reports whether every byte in s is a valid hexadecimal digit.
func isHex(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// isCanonicalUUID reports whether s matches the xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx format.
func isCanonicalUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	if s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
		return false
	}
	return isHex(s[0:8]) && isHex(s[9:13]) && isHex(s[14:18]) && isHex(s[19:23]) && isHex(s[24:36])
}
