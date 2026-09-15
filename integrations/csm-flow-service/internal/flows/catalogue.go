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

package flows

// Catalogue returns every flow that has been PORTED, whether or not it is
// enabled. All() returns the subset that actually runs in production.
//
// The two lists are separate because a port becomes live only when its
// ServiceNow counterpart is disabled in the same commit (the double-fire guard,
// docs/cutover-porting-plan.md §7) — but a port needs to be runnable before
// that, or it can only ever be verified against fakes. cmd/dryrun executes a
// flow from here against the real database without enabling it for anyone else.
//
// Adding a flow here does NOT make it fire: nothing in the consumer's hot path
// reads this list.
func Catalogue() []Flow {
	return []Flow{
		crApprovalNotice{},
		crPlanDateNotice{},
	}
}

// ByKey returns the ported flow with the given Key, enabled or not.
func ByKey(key string) (Flow, bool) {
	for _, f := range Catalogue() {
		if f.Key() == key {
			return f, true
		}
	}
	return nil, false
}

// CatalogueKeys lists every ported flow's key, for a caller that needs to tell
// the user what ByKey would accept.
func CatalogueKeys() []string {
	fs := Catalogue()
	keys := make([]string, 0, len(fs))
	for _, f := range fs {
		keys = append(keys, f.Key())
	}
	return keys
}
