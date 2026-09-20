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

package handler

import "testing"

// TestIsAttachmentID covers both id shapes an attachment reaches this API as.
// The sysid case is the one that mattered in staging: an inline comment image is
// referenced only by "<img src='…/<sysid>.iix'>", so the frontend extracts a bare
// 32-hex sysid and asks for its content. Requiring a dashed UUID made every
// inline image fail, while the Ballerina backend accepted both (its path
// parameter is entity:IdString, an unconstrained string alias).
func TestIsAttachmentID(t *testing.T) {
	valid := map[string]string{
		"dashed uuid from the attachments list": "09dc581d-3bb2-8710-9140-4c6aa5e45afe",
		"bare sysid from an .iix image src":     "09dc581d3bb2871091404c6aa5e45afe",
		"uppercase sysid":                       "09DC581D3BB2871091404C6AA5E45AFE",
		"uppercase uuid":                        "09DC581D-3BB2-8710-9140-4C6AA5E45AFE",
	}
	for name, id := range valid {
		if !isAttachmentID(id) {
			t.Errorf("%s: isAttachmentID(%q) = false, want true", name, id)
		}
	}

	// Still rejected: anything that is neither shape must not reach an upstream
	// URL path.
	invalid := map[string]string{
		"empty":              "",
		"too short":          "09dc581d",
		"31 hex":             "09dc581d3bb2871091404c6aa5e45af",
		"33 hex":             "09dc581d3bb2871091404c6aa5e45afef",
		"non-hex":            "09dc581d3bb2871091404c6aa5e45azz",
		"path traversal":     "../../secrets",
		"slash injection":    "09dc581d3bb2871091404c6aa5e45afe/../other",
		"query injection":    "09dc581d3bb2871091404c6aa5e45afe?x=1",
		"uuid missing group": "09dc581d-3bb2-8710-4c6aa5e45afe",
	}
	for name, id := range invalid {
		if isAttachmentID(id) {
			t.Errorf("%s: isAttachmentID(%q) = true, want false", name, id)
		}
	}
}

func TestToDashedID(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"26051dbc-3baa-8f50-9140-4c6aa5e45a1c", "26051dbc-3baa-8f50-9140-4c6aa5e45a1c"},
		{"26051DBC-3BAA-8F50-9140-4C6AA5E45A1C", "26051dbc-3baa-8f50-9140-4c6aa5e45a1c"},
		{"26051dbc3baa8f5091404c6aa5e45a1c", "26051dbc-3baa-8f50-9140-4c6aa5e45a1c"},
		{"26051DBC3BAA8F5091404C6AA5E45A1C", "26051dbc-3baa-8f50-9140-4c6aa5e45a1c"},
		{"70d63bca3bd2031091404c6aa5e45a37", "70d63bca-3bd2-0310-9140-4c6aa5e45a37"},
		{"70d63bca-3bd2-0310-9140-4c6aa5e45a37", "70d63bca-3bd2-0310-9140-4c6aa5e45a37"},
		{"case-1", "case-1"},
	}
	for _, tc := range cases {
		if got := toDashedID(tc.in); got != tc.want {
			t.Errorf("toDashedID(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestIsUUIDOrSysID(t *testing.T) {
	cases := map[string]struct {
		id   string
		want bool
	}{
		"valid uuid":       {"26051dbc-3baa-8f50-9140-4c6aa5e45a1c", true},
		"uppercase uuid":   {"09DC581D-3BB2-8710-9140-4C6AA5E45AFE", true},
		"valid sysid":      {"26051dbc3baa8f5091404c6aa5e45a1c", true},
		"uppercase sysid":  {"26051DBC3BAA8F5091404C6AA5E45A1C", true},
		"empty":            {"", false},
		"malformed":        {"not-a-valid-id", false},
		"short hex":        {"26051dbc", false},
		"too short sysid":  {"09dc581d3bb2871091404c6aa5e45af", false},
		"too long sysid":   {"09dc581d3bb2871091404c6aa5e45afef", false},
		"invalid hex char": {"09dc581d3bb2871091404c6aa5e45azz", false},
		"path traversal":   {"../path-traversal", false},
		"query injection":  {"4e8431b1-1b8c-0310-0bb3-da47b04bcba6?query=1", false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := isUUIDOrSysID(tc.id); got != tc.want {
				t.Errorf("isUUIDOrSysID(%q) = %v, want %v", tc.id, got, tc.want)
			}
		})
	}
}
