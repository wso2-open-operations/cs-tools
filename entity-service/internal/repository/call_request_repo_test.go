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
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// TestCallRequestStatesMatchMigration diffs the domain states against the
// customer_call_state_enum labels in the real migration file, so a label
// added, renamed, or misspelled there (e.g. CANCELED vs CANCELLED) fails here
// rather than at runtime.
func TestCallRequestStatesMatchMigration(t *testing.T) {
	sql, err := os.ReadFile("../../migrations/000072_customer_call_table.up.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	m := regexp.MustCompile(`(?s)CREATE TYPE customer_call_state_enum AS ENUM \((.*?)\)`).FindSubmatch(sql)
	if m == nil {
		t.Fatal("customer_call_state_enum definition not found in migration")
	}
	var want []string
	for _, l := range regexp.MustCompile(`'([A-Z_0-9]+)'`).FindAllSubmatch(m[1], -1) {
		want = append(want, string(l[1]))
	}

	var got []string
	for state := range callRequestStateLabels {
		got = append(got, callRequestStateToEnum(state))
	}
	sort.Strings(want)
	sort.Strings(got)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("domain call request states %v != migration enum labels %v", got, want)
	}
}

func TestCallRequestStateRoundTrip(t *testing.T) {
	for state, label := range callRequestStateLabels {
		got := CallRequestStateFromEnum(callRequestStateToEnum(state))
		if got.ID != string(state) || got.Label != label {
			t.Errorf("round trip %q = %+v, want id %q label %q", state, got, state, label)
		}
	}
	if got := CallRequestStateFromEnum("NOT_A_STATE"); got != (domain.CallRequestState{}) {
		t.Errorf("unknown enum label = %+v, want zero value", got)
	}
}

func TestDecodeFinalTimes(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{"null column", "", []string{}},
		{"json null", "null", []string{}},
		{"string array", `["2026-10-01T10:00:00Z","2026-10-02T10:00:00Z"]`, []string{"2026-10-01T10:00:00Z", "2026-10-02T10:00:00Z"}},
		{"unexpected object shape degrades to empty", `{"a":1}`, []string{}},
		{"unexpected array-of-object shape degrades to empty", `[{"t":"x"}]`, []string{}},
		{"invalid json degrades to empty", `not json`, []string{}},
	}
	for _, tt := range tests {
		got := decodeFinalTimes([]byte(tt.raw))
		if got == nil {
			t.Errorf("%s: got nil, want non-nil so it serializes as []", tt.name)
		}
		if strings.Join(got, "|") != strings.Join(tt.want, "|") {
			t.Errorf("%s: got %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestParseActualDurationMin(t *testing.T) {
	str := func(s string) *string { return &s }
	tests := []struct {
		in   *string
		want int // 0 => nil
	}{
		{nil, 0},
		{str("40"), 40},
		{str(" 15 "), 15},
		{str("0"), 0},
		{str("-3"), 0},
		{str("1 Hour 30 Minutes"), 0},
		{str(""), 0},
	}
	for _, tt := range tests {
		got := parseActualDurationMin(tt.in)
		if tt.want == 0 {
			if got != nil {
				t.Errorf("parseActualDurationMin(%v) = %d, want nil", tt.in, *got)
			}
			continue
		}
		if got == nil || *got != tt.want {
			t.Errorf("parseActualDurationMin(%q) = %v, want %d", *tt.in, got, tt.want)
		}
	}
}

func TestCaseStatesToEnumsMatchCaseEnum(t *testing.T) {
	got := caseStatesToEnums([]domain.CaseState{domain.CaseStateWaitingOnWSO2, domain.CaseStateWorkInProgress, domain.CaseStateClosed})
	want := []string{"WAITING_ON_WSO2", "WORK_IN_PROGRESS", "CLOSED"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("got %v, want %v", got, want)
	}
}
