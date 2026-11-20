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
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestGitInternalsEnabledParse(t *testing.T) {
	for raw, want := range map[string]bool{"true": true, " true\n": true, "": false, "false": false, "TRUE": false, "1": false, "yes": false} {
		if got := gitInternalsEnabled(raw); got != want {
			t.Errorf("gitInternalsEnabled(%q) = %v, want %v", raw, got, want)
		}
	}
	t.Setenv(gitInternalsFlag, "")
	if loadGitInternalsEnabled() {
		t.Error("expected off when unset")
	}
	t.Setenv(gitInternalsFlag, "true")
	if !loadGitInternalsEnabled() {
		t.Error("expected on for exactly \"true\"")
	}
}

// TestProductWorkItemRoutePermissions pins each product work item route to its
// permission in main.go, and that every one is registered only inside the
// GIT_INTERNALS_ENABLED branch, so with the flag off none exists.
func TestProductWorkItemRoutePermissions(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	start := strings.Index(text, "if gitInternalsOn {")
	if start < 0 {
		t.Fatal("main.go has no `if gitInternalsOn {` block")
	}
	end := start + strings.Index(text[start:], "\n\t}\n")
	block := text[start:end]

	reg := regexp.MustCompile(`(?m)^\s*route\("([A-Z]+) (/[^"]*)",\s*(handler\.Perm\w+),`)
	inBlock := map[string]string{}
	for _, m := range reg.FindAllStringSubmatch(block, -1) {
		inBlock[m[1]+" "+m[2]] = m[3]
	}
	want := map[string]string{
		"GET /cases/{id}/product-work-items":                       "handler.PermView",
		"GET /product-work-items/{id}":                             "handler.PermView",
		"POST /product-work-items/search":                          "handler.PermView",
		"POST /product-work-items/{id}/comments":                   "handler.PermWrite",
		"POST /product-work-items/{id}/board-status":               "handler.PermWrite",
		"POST /product-work-items/{id}/close":                      "handler.PermWrite",
		"POST /product-work-items/{id}/commands/{commandId}/retry": "handler.PermWrite",
		"POST /cases/{id}/product-work-items/links":                "handler.PermWrite",
		"DELETE /cases/{id}/product-work-items/{workItemId}/link":  "handler.PermWrite",
	}
	for name, perm := range want {
		if got, ok := inBlock[name]; !ok {
			t.Errorf("%s is not registered inside the GIT_INTERNALS_ENABLED block", name)
		} else if got != perm {
			t.Errorf("%s is registered with %s, want %s", name, got, perm)
		}
	}
	if len(inBlock) != len(want) {
		t.Errorf("the block registers %d routes, want %d: %v", len(inBlock), len(want), inBlock)
	}
	// None of them may also be registered outside the block.
	outside := text[:start] + text[end:]
	for name := range want {
		parts := strings.SplitN(name, " ", 2)
		if strings.Contains(outside, `route("`+parts[0]+" "+parts[1]+`"`) {
			t.Errorf("%s is also registered outside the flag block", name)
		}
	}
	if !strings.Contains(text, "gitInternalsOn := loadGitInternalsEnabled()") {
		t.Error("gitInternalsOn is not read from loadGitInternalsEnabled")
	}
}

// TestProductWorkItemRoutesDoNotConflictInTheRealMux registers every pattern
// main.go registers through route(...)/routeAll(...) in one ServeMux (which
// panics on a conflicting pair) and checks each new request reaches its own
// pattern. main() itself cannot be built in a test, so the patterns are read
// from its source; the handlers behind them are placeholders.
func TestProductWorkItemRoutesDoNotConflictInTheRealMux(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	seen := map[string]bool{}
	for _, m := range regexp.MustCompile(`(?m)^\s*(?:route|routeAll)\("([A-Z]+ [^"]+)"`).FindAllStringSubmatch(string(src), -1) {
		if seen[m[1]] {
			continue
		}
		seen[m[1]] = true
		pattern := m[1]
		mux.HandleFunc(pattern, func(http.ResponseWriter, *http.Request) {})
	}
	id1, id2 := "11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222"
	for method, want := range map[string][2]string{
		"GET /cases/" + id1 + "/product-work-items":                       {"GET", "GET /cases/{id}/product-work-items"},
		"POST /cases/" + id1 + "/product-work-items/links":                {"POST", "POST /cases/{id}/product-work-items/links"},
		"DELETE /cases/" + id1 + "/product-work-items/" + id2 + "/link":   {"DELETE", "DELETE /cases/{id}/product-work-items/{workItemId}/link"},
		"GET /product-work-items/" + id2:                                  {"GET", "GET /product-work-items/{id}"},
		"POST /product-work-items/search":                                 {"POST", "POST /product-work-items/search"},
		"POST /product-work-items/" + id2 + "/comments":                   {"POST", "POST /product-work-items/{id}/comments"},
		"POST /product-work-items/" + id2 + "/board-status":               {"POST", "POST /product-work-items/{id}/board-status"},
		"POST /product-work-items/" + id2 + "/close":                      {"POST", "POST /product-work-items/{id}/close"},
		"POST /product-work-items/" + id2 + "/commands/" + id1 + "/retry": {"POST", "POST /product-work-items/{id}/commands/{commandId}/retry"},
	} {
		path := strings.SplitN(method, " ", 2)[1]
		_, pattern := mux.Handler(httptest.NewRequest(want[0], path, nil))
		if pattern != want[1] {
			t.Errorf("%s matched %q, want %q", method, pattern, want[1])
		}
	}
}
