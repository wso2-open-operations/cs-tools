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

// TestAnnouncementRequestRoutePermissions pins how each /announcement-requests
// route is registered in main.go, because the registration is the only thing
// that makes the announcement-creator role bite on them: a write route put
// back on a plain route(..., PermWrite, ...) would let any CS engineer create or
// publish an announcement again, and nothing else would notice. Reading one
// request (and its updates and deliveries) stays PermView, because a published
// announcement in the main list opens the same request dialog; the request LIST
// (the Requests tab: drafts and requests awaiting approval) is the creators'
// workspace and needs both permissions. Approving stays PermWrite on purpose (it
// only records a decision taken over email, see handler.PermCreateAnnouncement).
func TestAnnouncementRequestRoutePermissions(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}

	// route("METHOD /path", Perm, fn)   or   routeAll("METHOD /path", fn, PermA, PermB)
	reg := regexp.MustCompile(`(?m)^\s*(route|routeAll)\("([A-Z]+) (/announcement-requests[^"]*)",(.*)\)\s*$`)
	matches := reg.FindAllStringSubmatch(string(src), -1)
	if len(matches) == 0 {
		t.Fatal("found no /announcement-requests routes in main.go; the pattern in this test is stale")
	}

	for _, m := range matches {
		kind, method, path, rest := m[1], m[2], m[3], m[4]
		name := method + " " + path

		var wantKind string
		var wantPerms []string
		switch {
		case method == "GET":
			wantKind, wantPerms = "route", []string{"PermView"}
		case method == "POST" && strings.HasSuffix(path, "/approve"):
			wantKind, wantPerms = "route", []string{"PermWrite"}
		default:
			wantKind, wantPerms = "routeAll", []string{"PermWrite", "PermCreateAnnouncement"}
		}

		if kind != wantKind {
			t.Errorf("%s is registered with %s, want %s", name, kind, wantKind)
			continue
		}
		for _, perm := range wantPerms {
			if !strings.Contains(rest, "handler."+perm) {
				t.Errorf("%s must be registered with handler.%s (got: %s)", name, perm, strings.TrimSpace(rest))
			}
		}
		if len(wantPerms) == 1 {
			if n := strings.Count(rest, "handler.Perm"); n != 1 {
				t.Errorf("%s must carry exactly one permission, found %d (got: %s)", name, n, strings.TrimSpace(rest))
			}
		}
	}

	// The create route and the three that decide whether anything goes out
	// must be there at all, so deleting a line cannot pass as "no violations".
	for _, want := range []string{
		"POST /announcement-requests", "PATCH /announcement-requests/{id}",
		"POST /announcement-requests/{id}/submit", "POST /announcement-requests/{id}/publish",
	} {
		found := false
		for _, m := range matches {
			if m[2]+" "+m[3] == want {
				found = true
			}
		}
		if !found {
			t.Errorf("%s is no longer registered in main.go", want)
		}
	}
}
