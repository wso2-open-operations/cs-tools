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
	"strings"
	"testing"
)

// TestCommentTemplateRoutePermissions pins who can reach the saved case
// comment template routes. Listing them and managing your own needs what
// posting a comment or work note needs (PermCreateWorkNote); writing a shared
// template is admin only. The routes are the whole check: the handler does not
// look at roles itself, so a wrong permission here is a real hole.
func TestCommentTemplateRoutePermissions(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}

	reg := regexp.MustCompile(`(?m)^\s*route\("([A-Z]+) (/[^"]*)",\s*(handler\.Perm\w+),`)
	matches := reg.FindAllStringSubmatch(string(src), -1)
	if len(matches) == 0 {
		t.Fatal("found no routes in main.go; the pattern in this test is stale")
	}

	want := map[string]string{
		"GET /users/me/case-comment-templates":         "handler.PermCreateWorkNote",
		"POST /users/me/case-comment-templates":        "handler.PermCreateWorkNote",
		"PATCH /users/me/case-comment-templates/{id}":  "handler.PermCreateWorkNote",
		"DELETE /users/me/case-comment-templates/{id}": "handler.PermCreateWorkNote",
		"POST /case-comment-templates":                 "handler.PermAdmin",
		"PATCH /case-comment-templates/{id}":           "handler.PermAdmin",
		"DELETE /case-comment-templates/{id}":          "handler.PermAdmin",
	}

	seen := map[string]bool{}
	for _, m := range matches {
		name, perm := m[1]+" "+m[2], m[3]
		if !strings.Contains(m[2], "case-comment-templates") {
			continue
		}
		w, ok := want[name]
		if !ok {
			t.Errorf("%s is registered with %s but not pinned here; add it with the permission it needs", name, perm)
			continue
		}
		seen[name] = true
		if perm != w {
			t.Errorf("%s is registered with %s, want %s", name, perm, w)
		}
	}
	for name := range want {
		if !seen[name] {
			t.Errorf("route %q is no longer registered with route(...) in main.go; update this test if it moved", name)
		}
	}
}
