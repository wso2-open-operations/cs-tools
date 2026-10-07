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

func TestNormalizeIssueComplexity(t *testing.T) {
	testCases := []struct {
		name string
		in   string
		want string
	}{
		{"N/A maps to the enum's real label", "N/A", "NOT_APPLICABLE"},
		{"n/a is matched case-insensitively", "n/a", "NOT_APPLICABLE"},
		{"Low upper-cases unchanged", "Low", "LOW"},
		{"Medium upper-cases unchanged", "Medium", "MEDIUM"},
		{"High upper-cases unchanged", "High", "HIGH"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeIssueComplexity(tc.in); got != tc.want {
				t.Errorf("normalizeIssueComplexity(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestIssueComplexityFromEnum(t *testing.T) {
	testCases := []struct {
		name string
		in   string
		want string
	}{
		{"NOT_APPLICABLE maps back to N/A", "NOT_APPLICABLE", "N/A"},
		{"LOW maps back to Low", "LOW", "Low"},
		{"MEDIUM maps back to Medium", "MEDIUM", "Medium"},
		{"HIGH maps back to High", "HIGH", "High"},
		{"an unrecognized value passes through unchanged", "SOMETHING_ELSE", "SOMETHING_ELSE"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := issueComplexityFromEnum(tc.in); got != tc.want {
				t.Errorf("issueComplexityFromEnum(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestIssueComplexityRoundTrip locks in the full write-then-read contract:
// every value normalizeIssueComplexity can produce must come back out of
// issueComplexityFromEnum as exactly the webapp's own original wire value --
// the bug this pair of functions fixes was found by tracing exactly this
// round trip by hand.
func TestIssueComplexityRoundTrip(t *testing.T) {
	webappValues := []string{"N/A", "Low", "Medium", "High"}
	for _, original := range webappValues {
		t.Run(original, func(t *testing.T) {
			stored := normalizeIssueComplexity(original)
			got := issueComplexityFromEnum(stored)
			if got != original {
				t.Errorf("round trip for %q: stored as %q, read back as %q, want %q", original, stored, got, original)
			}
		})
	}
}
