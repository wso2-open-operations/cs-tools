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

import (
	"reflect"
	"testing"
	"time"
)

// TestParseExcludedProjectIDs covers EXCLUDED_PROJECT_IDS parsing: a
// comma-separated list of project IDs, trimmed of surrounding whitespace,
// with empty entries dropped. An unset/empty value parses to an empty set
// rather than a set containing "" — an empty string is never a real
// project ID that should match anything.
func TestParseExcludedProjectIDs(t *testing.T) {
	tests := []struct {
		name string
		v    string
		want map[string]bool
	}{
		{name: "empty value", v: "", want: map[string]bool{}},
		{name: "single ID", v: "abc-123", want: map[string]bool{"abc-123": true}},
		{
			name: "multiple IDs",
			v:    "abc-123,def-456,ghi-789",
			want: map[string]bool{"abc-123": true, "def-456": true, "ghi-789": true},
		},
		{
			name: "whitespace around IDs is trimmed",
			v:    " abc-123 , def-456 ,ghi-789 ",
			want: map[string]bool{"abc-123": true, "def-456": true, "ghi-789": true},
		},
		{
			name: "empty entries from stray commas are dropped",
			v:    "abc-123,,def-456,",
			want: map[string]bool{"abc-123": true, "def-456": true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseExcludedProjectIDs(tt.v)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseExcludedProjectIDs(%q) = %v, want %v", tt.v, got, tt.want)
			}
		})
	}
}

// TestParseCommaSeparatedList covers STANDING_CC_RECIPIENTS parsing —
// same trim/drop-empty convention as parseExcludedProjectIDs, but an
// ordered slice (not a set), since a recipient list's order matters for
// deterministic assertions and duplicates are meaningful.
func TestParseCommaSeparatedList(t *testing.T) {
	tests := []struct {
		name string
		v    string
		want []string
	}{
		{name: "empty value", v: "", want: nil},
		{name: "single address", v: "billing@wso2.com", want: []string{"billing@wso2.com"}},
		{
			name: "multiple addresses",
			v:    "customer-lifecycle-notification@wso2.com,billing@wso2.com",
			want: []string{"customer-lifecycle-notification@wso2.com", "billing@wso2.com"},
		},
		{
			name: "whitespace around addresses is trimmed",
			v:    " customer-lifecycle-notification@wso2.com , billing@wso2.com ",
			want: []string{"customer-lifecycle-notification@wso2.com", "billing@wso2.com"},
		},
		{
			name: "empty entries from stray commas are dropped",
			v:    "billing@wso2.com,,customer-lifecycle-notification@wso2.com,",
			want: []string{"billing@wso2.com", "customer-lifecycle-notification@wso2.com"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCommaSeparatedList(tt.v)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseCommaSeparatedList(%q) = %v, want %v", tt.v, got, tt.want)
			}
		})
	}
}

// TestExitCode verifies the process reports failure to its caller (a
// scheduled Choreo task) whenever any project failed during the sweep, and
// success only when none did.
func TestExitCode(t *testing.T) {
	tests := []struct {
		name         string
		failureCount int
		wantExitCode int
	}{
		{name: "no failures", failureCount: 0, wantExitCode: 0},
		{name: "one failure", failureCount: 1, wantExitCode: 1},
		{name: "many failures", failureCount: 7, wantExitCode: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := exitCode(tt.failureCount)
			if got != tt.wantExitCode {
				t.Errorf("exitCode(%d) = %d, want %d", tt.failureCount, got, tt.wantExitCode)
			}
		})
	}
}

// TestIsWeekend covers the weekend guard: no run (and so no email) on
// Saturday or Sunday. "Weekend" is judged in the operations timezone
// (UTC+05:30), not the container's UTC clock — the UTC day and the local
// day differ for five and a half hours around every midnight, and those are
// exactly the cases worth pinning down.
func TestIsWeekend(t *testing.T) {
	tests := []struct {
		name string
		now  time.Time
		want bool
	}{
		// 2026-09-25 is a Friday.
		{name: "Friday daytime", now: time.Date(2026, 9, 25, 4, 0, 0, 0, time.UTC), want: false},
		{name: "Saturday daytime", now: time.Date(2026, 9, 26, 4, 0, 0, 0, time.UTC), want: true},
		{name: "Sunday daytime", now: time.Date(2026, 9, 27, 4, 0, 0, 0, time.UTC), want: true},
		{name: "Monday daytime", now: time.Date(2026, 9, 28, 4, 0, 0, 0, time.UTC), want: false},
		// Friday 21:00 UTC is already Saturday 02:30 in UTC+05:30.
		{name: "Friday late UTC is Saturday locally", now: time.Date(2026, 9, 25, 21, 0, 0, 0, time.UTC), want: true},
		// Sunday 19:00 UTC is already Monday 00:30 in UTC+05:30.
		{name: "Sunday late UTC is Monday locally", now: time.Date(2026, 9, 27, 19, 0, 0, 0, time.UTC), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isWeekend(tt.now); got != tt.want {
				t.Errorf("isWeekend(%v) = %v, want %v", tt.now, got, tt.want)
			}
		})
	}
}
