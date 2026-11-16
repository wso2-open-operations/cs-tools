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

// TestTimeCardRoutePermissions pins who can reach the time card routes: every
// one is PermTimeCardsAndUpdates (CS engineer, admin, time card approver; a
// read-only Viewer gets none of them), except the aggregate, which is
// PermViewTimeCardTotals (CS engineer and admin only) until who may see
// totals per user is decided. Changing either is a decision, not a refactor:
// change this test with it.
func TestTimeCardRoutePermissions(t *testing.T) {
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
		if path != "/time-cards" && !strings.HasPrefix(path, "/time-cards/") {
			continue
		}
		name := method + " " + path
		seen[name] = true
		want := "handler.PermTimeCardsAndUpdates"
		if name == "POST /time-cards/aggregate" {
			want = "handler.PermViewTimeCardTotals"
		}
		if perm != want {
			t.Errorf("%s is registered with %s, want %s", name, perm, want)
		}
	}

	for _, name := range []string{
		"POST /time-cards/search", "POST /time-cards/aggregate", "POST /time-cards",
		"PATCH /time-cards/{id}", "DELETE /time-cards/{id}",
	} {
		if !seen[name] {
			t.Errorf("route %q is no longer registered with route(...) in main.go; update this test if it moved", name)
		}
	}
}
