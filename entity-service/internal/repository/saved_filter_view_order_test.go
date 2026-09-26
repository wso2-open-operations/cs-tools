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

package repository

import "testing"

func TestReorderIDs(t *testing.T) {
	ids := []string{"a", "b", "c", "d"}

	got, changed := reorderIDs(ids, 0, 2)
	if !changed {
		t.Fatal("expected a change")
	}
	want := []string{"b", "c", "a", "d"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("move down = %v, want %v", got, want)
		}
	}
	if ids[0] != "a" {
		t.Fatal("reorderIDs mutated the input")
	}

	got, changed = reorderIDs(ids, 3, 1)
	if !changed {
		t.Fatal("expected a change")
	}
	want = []string{"a", "d", "b", "c"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("move up = %v, want %v", got, want)
		}
	}

	if _, changed = reorderIDs(ids, 1, 1); changed {
		t.Fatal("same index should be a no-op")
	}
	got, changed = reorderIDs(ids, 0, 99)
	if !changed {
		t.Fatal("expected a clamped move")
	}
	want = []string{"b", "c", "d", "a"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("clamp = %v, want %v", got, want)
		}
	}
}
