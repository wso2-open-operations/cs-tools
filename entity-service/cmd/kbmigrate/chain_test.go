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

import "testing"

// fakeKBStore backs kbLookup with an in-memory map of sys_id -> snKnowledge,
// simulating repeated GETs against ServiceNow's kb_knowledge table.
func fakeKBStore(rows ...snKnowledge) kbLookup {
	m := make(map[string]snKnowledge, len(rows))
	for _, r := range rows {
		m[r.SysID] = r
	}
	return func(sysID string) (snKnowledge, bool, error) {
		r, ok := m[sysID]
		return r, ok, nil
	}
}

func TestWalkVersionChain_NeverRevised(t *testing.T) {
	latest := snKnowledge{SysID: "root0000000000000000000000000001", Latest: true, BaseVersion: ""}
	lookup := fakeKBStore(latest)

	history, warning := walkVersionChain(latest, lookup)
	if warning != "" {
		t.Fatalf("unexpected warning: %s", warning)
	}
	if len(history) != 0 {
		t.Fatalf("expected zero history rows for a never-revised article, got %d", len(history))
	}
}

func TestWalkVersionChain_MultiVersion(t *testing.T) {
	v1 := snKnowledge{SysID: "v1000000000000000000000000000001", BaseVersion: ""}
	v2 := snKnowledge{SysID: "v2000000000000000000000000000002", BaseVersion: v1.SysID}
	v3 := snKnowledge{SysID: "v3000000000000000000000000000003", BaseVersion: v2.SysID}
	latest := snKnowledge{SysID: "v4000000000000000000000000000004", Latest: true, BaseVersion: v3.SysID}

	lookup := fakeKBStore(v1, v2, v3, latest)

	history, warning := walkVersionChain(latest, lookup)
	if warning != "" {
		t.Fatalf("unexpected warning: %s", warning)
	}
	if len(history) != 3 {
		t.Fatalf("expected 3 history rows, got %d: %+v", len(history), history)
	}
	// Walk order is root-ward (newest-visited-first): v3, then v2, then v1.
	wantOrder := []string{v3.SysID, v2.SysID, v1.SysID}
	for i, want := range wantOrder {
		if history[i].SysID != want {
			t.Errorf("history[%d].SysID = %q, want %q", i, history[i].SysID, want)
		}
	}
}

func TestWalkVersionChain_MissingLink_TruncatesWithWarning(t *testing.T) {
	// latest points at a base_version that was never seeded into the store --
	// simulates a dangling/deleted predecessor in real SN data.
	latest := snKnowledge{
		SysID:       "v2000000000000000000000000000002",
		Latest:      true,
		BaseVersion: "missing0000000000000000000000001",
	}
	lookup := fakeKBStore(latest)

	history, warning := walkVersionChain(latest, lookup)
	if warning == "" {
		t.Fatal("expected a warning for a missing base_version link")
	}
	if len(history) != 0 {
		t.Fatalf("expected zero history rows when the first link is missing, got %d", len(history))
	}
}

func TestWalkVersionChain_PartialChainBeforeMissingLink(t *testing.T) {
	v1 := snKnowledge{SysID: "v1000000000000000000000000000001", BaseVersion: ""}
	v2 := snKnowledge{SysID: "v2000000000000000000000000000002", BaseVersion: "missing0000000000000000000000009"}
	latest := snKnowledge{SysID: "v3000000000000000000000000000003", Latest: true, BaseVersion: v2.SysID}

	lookup := fakeKBStore(v1, v2, latest)

	history, warning := walkVersionChain(latest, lookup)
	if warning == "" {
		t.Fatal("expected a warning once the walk hits the missing link")
	}
	if len(history) != 1 || history[0].SysID != v2.SysID {
		t.Fatalf("expected the walk to collect v2 before truncating, got %+v", history)
	}
}

func TestWalkVersionChain_CycleDetected(t *testing.T) {
	// a <-> b forming a 2-cycle: a data-quality anomaly, not a real chain.
	a := snKnowledge{SysID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", BaseVersion: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}
	b := snKnowledge{SysID: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", BaseVersion: a.SysID} // points back at a: cycle
	latest := snKnowledge{SysID: "cccccccccccccccccccccccccccccccc", Latest: true, BaseVersion: a.SysID}

	lookup := fakeKBStore(a, b, latest)

	history, warning := walkVersionChain(latest, lookup)
	if warning == "" {
		t.Fatal("expected a warning when a cycle is detected")
	}
	// a and b are both collected before the walk revisits a and detects the cycle.
	if len(history) != 2 || history[0].SysID != a.SysID || history[1].SysID != b.SysID {
		t.Fatalf("expected the walk to collect [a, b] before detecting the cycle, got %+v", history)
	}
}
