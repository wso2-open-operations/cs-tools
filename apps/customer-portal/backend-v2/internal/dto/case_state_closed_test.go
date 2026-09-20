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

package dto

import "testing"

func TestIsCaseStateClosed(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected bool
	}{
		// Closed representations
		{name: "domain enum closed", input: "closed", expected: true},
		{name: "display label Closed", input: "Closed", expected: true},
		{name: "uppercase CLOSED", input: "CLOSED", expected: true},
		{name: "choice list id 3", input: "3", expected: true},
		{name: "padded closed", input: "  closed  ", expected: true},
		{name: "padded id 3", input: " 3 ", expected: true},

		// Non-closed representations (domain enum, display label, and numeric id)
		{name: "domain enum open", input: "open", expected: false},
		{name: "display label Open", input: "Open", expected: false},
		{name: "choice list id 1 (open)", input: "1", expected: false},
		{name: "domain enum work_in_progress", input: "work_in_progress", expected: false},
		{name: "display label Work In Progress", input: "Work In Progress", expected: false},
		{name: "choice list id 2 (wip)", input: "2", expected: false},
		{name: "domain enum waiting_on_wso2", input: "waiting_on_wso2", expected: false},
		{name: "choice list id 10 (waiting on wso2)", input: "10", expected: false},
		{name: "domain enum awaiting_info", input: "awaiting_info", expected: false},
		{name: "choice list id 11 (awaiting info)", input: "11", expected: false},
		{name: "domain enum solution_proposed", input: "solution_proposed", expected: false},
		{name: "display label Solution Proposed", input: "Solution Proposed", expected: false},
		{name: "choice list id 6 (solution proposed)", input: "6", expected: false},
		{name: "domain enum reopened", input: "reopened", expected: false},
		{name: "choice list id 13 (reopened)", input: "13", expected: false},

		// Edge cases
		{name: "empty string", input: "", expected: false},
		{name: "whitespace only", input: "   ", expected: false},
		{name: "unknown string", input: "foobar", expected: false},
		{name: "unknown id", input: "999", expected: false},
		{name: "substring of a closed label", input: "close", expected: false},
		{name: "closed-looking longer string", input: "closed_pending_review", expected: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := IsCaseStateClosed(tc.input)
			if got != tc.expected {
				t.Fatalf("IsCaseStateClosed(%q) = %v, want %v", tc.input, got, tc.expected)
			}
		})
	}
}
