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

// TestCustomDashboardRoutePermissions pins who can reach the custom dashboard
// routes. Reading dashboards shared with you is PermView, like the system
// dashboards: the share picker offers every internal user and a team share
// reaches every member, so a share must open for a view-only role too (its
// widgets still query with the viewer's own permissions). Generating, saving,
// deleting and sharing stay PermWrite (CS engineers and admins).
func TestCustomDashboardRoutePermissions(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}

	// route("METHOD /path", handler.PermX, ...)
	reg := regexp.MustCompile(`(?m)^\s*route\("([A-Z]+) (/[^"]*)",\s*(handler\.Perm\w+),`)
	matches := reg.FindAllStringSubmatch(string(src), -1)
	if len(matches) == 0 {
		t.Fatal("found no routes in main.go; the pattern in this test is stale")
	}

	seen := map[string]bool{}
	for _, m := range matches {
		method, path, perm := m[1], m[2], m[3]
		name := method + " " + path
		var want string
		switch {
		case path == "/dashboards/shared" || strings.HasPrefix(path, "/dashboards/shared/"):
			want = "handler.PermView"
		case path == "/dashboards/custom" || strings.HasPrefix(path, "/dashboards/custom/"),
			strings.HasPrefix(path, "/dashboards/generate"):
			want = "handler.PermWrite"
		default:
			continue
		}
		seen[name] = true
		if perm != want {
			t.Errorf("%s is registered with %s, want %s", name, perm, want)
		}
	}

	for _, name := range []string{
		"GET /dashboards/shared", "GET /dashboards/shared/{id}",
		"GET /dashboards/custom", "PUT /dashboards/custom/{id}",
		"PUT /dashboards/custom/{id}/shares/{userId}", "POST /dashboards/generate",
	} {
		if !seen[name] {
			t.Errorf("route %q is no longer registered with route(...) in main.go; update this test if it moved", name)
		}
	}
}
