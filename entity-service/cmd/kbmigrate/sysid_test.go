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

func TestSysIDToUUID(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "lowercase hex sys_id",
			in:   "abcd1234abcd1234abcd1234abcd1234",
			want: "abcd1234-abcd-1234-abcd-1234abcd1234",
		},
		{
			name: "uppercase hex sys_id",
			in:   "ABCD1234ABCD1234ABCD1234ABCD1234",
			want: "ABCD1234-ABCD-1234-ABCD-1234ABCD1234",
		},
		{
			name: "all zeros",
			in:   "00000000000000000000000000000000",
			want: "00000000-0000-0000-0000-000000000000",
		},
		{
			name: "too short: returned unchanged",
			in:   "abcd1234",
			want: "abcd1234",
		},
		{
			name: "too long: returned unchanged",
			in:   "abcd1234abcd1234abcd1234abcd1234ff",
			want: "abcd1234abcd1234abcd1234abcd1234ff",
		},
		{
			name: "non-hex characters: returned unchanged",
			in:   "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz",
			want: "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz",
		},
		{
			name: "empty string: returned unchanged",
			in:   "",
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sysIDToUUID(tt.in); got != tt.want {
				t.Errorf("sysIDToUUID(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestIsCanonicalUUID(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"valid canonical uuid", "abcd1234-abcd-1234-abcd-1234abcd1234", true},
		{"sys_id, not converted", "abcd1234abcd1234abcd1234abcd1234", false},
		{"wrong hyphen positions", "abcd12345-bcd-1234-abcd-1234abcd1234", false},
		{"empty string", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isCanonicalUUID(tt.in); got != tt.want {
				t.Errorf("isCanonicalUUID(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
