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

package main

// sysIDToUUID converts a 32-character ServiceNow sys_id to a canonical UUID
// string by inserting hyphens at the standard 8-4-4-4-12 positions. This is
// the same deterministic transform used elsewhere in this codebase (see
// internal/service/sn_id.go's sysidToUUID) to derive entity-service UUIDs
// for SN-sourced records such as cases and products — reimplemented here,
// standalone, since cmd/kbmigrate must not import internal/service.
//
// Returns the input unchanged if it is not exactly 32 hex characters, so a
// caller can detect a malformed sys_id by comparing the result's length
// (still 32) against a canonical UUID's (36).
func sysIDToUUID(sysID string) string {
	if len(sysID) != 32 || !isHexString(sysID) {
		return sysID
	}
	return sysID[0:8] + "-" + sysID[8:12] + "-" + sysID[12:16] + "-" + sysID[16:20] + "-" + sysID[20:32]
}

// isHexString reports whether every byte in s is a valid hexadecimal digit.
func isHexString(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// isCanonicalUUID reports whether s matches the
// xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx format produced by sysIDToUUID.
func isCanonicalUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	if s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
		return false
	}
	return isHexString(s[0:8]) && isHexString(s[9:13]) && isHexString(s[14:18]) && isHexString(s[19:23]) && isHexString(s[24:36])
}
