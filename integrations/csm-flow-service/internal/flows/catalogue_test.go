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

import "testing"

// TestAllIsSubsetOfCatalogue keeps the two lists from drifting: a flow can be
// ported but not enabled, never enabled but not ported. If this fails, a flow
// was added to All() without being catalogued, and cmd/dryrun cannot reach the
// one flow most in need of a dry run.
func TestAllIsSubsetOfCatalogue(t *testing.T) {
	catalogued := map[string]bool{}
	for _, f := range Catalogue() {
		catalogued[f.Key()] = true
	}
	for _, f := range All() {
		if !catalogued[f.Key()] {
			t.Errorf("flow %q is enabled in All() but missing from Catalogue()", f.Key())
		}
	}
}

func TestCatalogueKeysAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, k := range CatalogueKeys() {
		if seen[k] {
			t.Errorf("duplicate flow key %q — Key() is half the idempotency key, so two flows sharing one is a bug", k)
		}
		seen[k] = true
	}
}

// TestByKeyReachesAnUnregisteredFlow is the property cmd/dryrun depends on:
// cr_approval_notice is deliberately absent from All() (the double-fire guard)
// and must still be runnable.
func TestByKeyReachesAnUnregisteredFlow(t *testing.T) {
	key := (crApprovalNotice{}).Key()
	f, ok := ByKey(key)
	if !ok {
		t.Fatalf("ByKey(%q) = _, false; a ported flow must be reachable whether or not it is enabled", key)
	}
	if f.Key() != key {
		t.Errorf("ByKey(%q).Key() = %q", key, f.Key())
	}
}

func TestByKeyUnknown(t *testing.T) {
	if _, ok := ByKey("no_such_flow"); ok {
		t.Error("ByKey returned true for a flow that does not exist")
	}
}
