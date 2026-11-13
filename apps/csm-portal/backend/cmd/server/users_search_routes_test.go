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
	"os"
	"regexp"
	"testing"
)

// TestUserSearchRoutePermissions pins who may reach each user search. The
// general search is open to every role and is restricted by its handler to
// internal users; the admin search is the only one that can list external
// users, so it must stay behind PermWrite. Loosening it to PermView would let
// any pasted request list customers again.
func TestUserSearchRoutePermissions(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	for pattern, want := range map[string]string{
		"POST /users/search":       "handler.PermView",
		"POST /admin/users/search": "handler.PermWrite",
	} {
		re := regexp.MustCompile(`route\("` + regexp.QuoteMeta(pattern) + `", (handler\.\w+),`)
		m := re.FindSubmatch(src)
		if m == nil {
			t.Errorf("%s is not registered with route(...)", pattern)
			continue
		}
		if string(m[1]) != want {
			t.Errorf("%s registered with %s, want %s", pattern, m[1], want)
		}
	}
}
