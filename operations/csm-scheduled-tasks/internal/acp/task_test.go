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

package acp

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/acp/sweep"
)

// TestParseProjectIDs covers ACP_EXCLUDED_PROJECT_IDS parsing: a
// comma-separated list of project IDs, trimmed of surrounding whitespace,
// with empty entries dropped. An unset/empty value parses to an empty set
// rather than a set containing "" — an empty string is never a real
// project ID that should match anything.
func TestParseProjectIDs(t *testing.T) {
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
			got := ParseProjectIDs(tt.v)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseProjectIDs(%q) = %v, want %v", tt.v, got, tt.want)
			}
		})
	}
}

// TestFailuresError checks the handler fails the run whenever any project
// failed, and succeeds only when none did. The error goes into the alert
// email, so it names the failed projects but never their errors, and caps
// the list.
func TestFailuresError(t *testing.T) {
	if err := failuresError("run-1", nil); err != nil {
		t.Fatalf("no failures: got %v, want nil", err)
	}

	secret := errors.New("customer body with someone@example.com")
	err := failuresError("run-1", []sweep.ProjectFailure{
		{ProjectID: "p1", Err: secret},
		{ProjectID: "p2", Err: secret},
	})
	if err == nil {
		t.Fatal("two failures: got nil error")
	}
	msg := err.Error()
	for _, want := range []string{"run-1", "2 project(s) failed", "p1, p2"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q does not contain %q", msg, want)
		}
	}
	if strings.Contains(msg, "example.com") {
		t.Errorf("error %q leaks a project error", msg)
	}

	var many []sweep.ProjectFailure
	for i := range maxFailedIDsInError + 3 {
		many = append(many, sweep.ProjectFailure{ProjectID: fmt.Sprintf("p%d", i), Err: secret})
	}
	msg = failuresError("run-2", many).Error()
	if !strings.Contains(msg, "and 3 more") {
		t.Errorf("error %q does not count the ids left out", msg)
	}
	if got := strings.Count(msg, ", ") + 1; got != maxFailedIDsInError {
		t.Errorf("error %q lists %d ids, want %d", msg, got, maxFailedIDsInError)
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
