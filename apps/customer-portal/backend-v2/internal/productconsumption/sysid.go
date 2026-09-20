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

package productconsumption

// The product-consumption service identifies projects and deployments by bare
// ServiceNow sysids — 32 hex characters, no dashes.
//
// The customer portal deals in dashed UUIDs everywhere else, because
// entity-service converts sysids on the way out (sysidToUUID) and back on the
// way in (uuidToSysid). So an id arriving from a portal URL is dashed, and
// handing it to this service unchanged means it matches nothing: the call
// still succeeds, it just describes a project that does not exist, and the
// licence flow then fails on data it cannot use.
//
// Only projectId and deploymentId get this treatment. applicationId comes FROM
// this service and goes back in whatever form it was issued.

// uuidToSysID strips the hyphens from a canonical UUID, returning anything
// else unchanged — an id that is already a bare sysid passes through
// untouched, so this is safe to apply at every boundary rather than only where
// a dashed id is expected.
//
// Mirrors entity-service's own uuidToSysid (internal/service/sn_id.go), which
// is the other half of the same conversion.
func uuidToSysID(id string) string {
	if !isCanonicalUUID(id) {
		return id
	}
	return id[0:8] + id[9:13] + id[14:18] + id[19:23] + id[24:36]
}

// isCanonicalUUID reports whether s is exactly 8-4-4-4-12 hex with hyphens in
// the canonical positions.
func isCanonicalUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, r := range s {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return false
			}
		default:
			if !isHexDigit(r) {
				return false
			}
		}
	}
	return true
}

func isHexDigit(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}
