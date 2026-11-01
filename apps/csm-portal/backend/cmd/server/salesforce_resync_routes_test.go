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

// TestSalesforceResyncRoutePermissions pins the two Salesforce resync routes to
// handler.PermAdmin in main.go: the entity service re-checks admin, but the
// route gate is what keeps every other portal role from reaching a write that
// overwrites database rows.
func TestSalesforceResyncRoutePermissions(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	reg := regexp.MustCompile(`(?m)^\s*route\("([A-Z]+) (/[^"]*)",\s*(handler\.Perm\w+),`)
	got := map[string]string{}
	for _, m := range reg.FindAllStringSubmatch(string(src), -1) {
		got[m[1]+" "+m[2]] = m[3]
	}
	for _, name := range []string{"POST /salesforce/resync/preview", "POST /salesforce/resync/apply"} {
		perm, ok := got[name]
		if !ok {
			t.Errorf("route %q is not registered with route(...) in main.go", name)
			continue
		}
		if perm != "handler.PermAdmin" {
			t.Errorf("%s is registered with %s, want handler.PermAdmin", name, perm)
		}
	}
}
