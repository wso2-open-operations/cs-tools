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

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

func TestFormatSearchTimestamp(t *testing.T) {
	if got := formatSearchTimestamp(nil); got != nil {
		t.Fatalf("nil input = %q, want nil", *got)
	}
	// A non-UTC instant must be rendered in UTC, like createdOn/updatedOn.
	loc := time.FixedZone("plus0530", 5*3600+30*60)
	in := time.Date(2026, 11, 3, 10, 30, 0, 0, loc)
	got := formatSearchTimestamp(&in)
	if got == nil || *got != "2026-11-03T05:00:00Z" {
		t.Fatalf("got %v, want 2026-11-03T05:00:00Z", got)
	}
}

func TestSearchCaseViewJSON_ResolvedAndClosedOn(t *testing.T) {
	set := "2026-11-03T05:00:00Z"
	b, err := json.Marshal(domain.SearchCaseView{ResolvedOn: &set, ClosedOn: &set})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if m["resolvedOn"] != set || m["closedOn"] != set {
		t.Fatalf("resolvedOn/closedOn = %v/%v, want %q", m["resolvedOn"], m["closedOn"], set)
	}

	b, err = json.Marshal(domain.SearchCaseView{})
	if err != nil {
		t.Fatal(err)
	}
	m = map[string]any{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"resolvedOn", "closedOn"} {
		v, ok := m[k]
		if !ok || v != nil {
			t.Fatalf("%s = %v (present=%v), want explicit null", k, v, ok)
		}
	}
}
