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

package service

import (
	"strings"
	"testing"
)

func TestUUIDToSysid_PathSafe(t *testing.T) {
	cases := map[string]string{
		"00000000-0000-0000-0000-000000000000": "00000000000000000000000000000000",
		"0123456789abcdef0123456789abcdef":     "0123456789abcdef0123456789abcdef",
		"CS0001":                               "CS0001",
		"":                                     "",
		".":                                    "%2E",
		"..":                                   "%2E%2E",
		"../admin":                             "..%2Fadmin",
		"a/b":                                  "a%2Fb",
		"x?y=1":                                "x%3Fy=1",
		"a b":                                  "a%20b",
	}
	for in, want := range cases {
		got := uuidToSysid(in)
		if got != want {
			t.Errorf("uuidToSysid(%q) = %q, want %q", in, got, want)
		}
		if strings.Contains(got, "/") || got == "." || got == ".." {
			t.Errorf("uuidToSysid(%q) = %q is not a single safe path segment", in, got)
		}
	}
}
