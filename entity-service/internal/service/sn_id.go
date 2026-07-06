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
