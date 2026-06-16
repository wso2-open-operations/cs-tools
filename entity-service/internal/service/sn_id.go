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

import "strings"

// sysidToUUID converts a 32-character ServiceNow sysid to a standard UUID by
// inserting hyphens at the canonical 8-4-4-4-12 positions.
// Returns the input unchanged if it is not exactly 32 characters long.
func sysidToUUID(sysid string) string {
	if len(sysid) != 32 {
		return sysid
	}
	return sysid[0:8] + "-" + sysid[8:12] + "-" + sysid[12:16] + "-" + sysid[16:20] + "-" + sysid[20:32]
}

// uuidToSysid converts a UUID string to a 32-character ServiceNow sysid by
// stripping all hyphens.
func uuidToSysid(uuid string) string {
	return strings.ReplaceAll(uuid, "-", "")
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
