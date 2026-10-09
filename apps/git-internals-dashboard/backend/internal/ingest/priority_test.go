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

package ingest

import "testing"

func TestExtractPriority(t *testing.T) {
	aliases := map[string]string{"High": "High(P2)", "Normal (P3)": "Medium(P3)"}
	cases := []struct {
		name   string
		labels []string
		want   string // "" = nil
	}{
		{"canonical passes through", []string{"Priority/High(P2)"}, "High(P2)"},
		{"alias normalized", []string{"Priority/High"}, "High(P2)"},
		{"alias with spaces normalized", []string{"Type/Bug", "Priority/Normal (P3)"}, "Medium(P3)"},
		{"unaliased variant stored as written", []string{"Priority/Low(P4)"}, "Low(P4)"},
		{"first Priority label wins", []string{"Priority/High", "Priority/Critical(P1)"}, "High(P2)"},
		{"no priority label", []string{"urgent", "P3"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractPriority(tc.labels, aliases)
			switch {
			case tc.want == "" && got != nil:
				t.Fatalf("got %q, want nil", *got)
			case tc.want != "" && (got == nil || *got != tc.want):
				t.Fatalf("got %v, want %q", got, tc.want)
			}
		})
	}
	if got := extractPriority([]string{"Priority/High"}, nil); got == nil || *got != "High" {
		t.Fatalf("nil aliases must leave tier unchanged, got %v", got)
	}
}
