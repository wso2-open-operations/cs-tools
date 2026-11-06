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

// TestFeatureOverrideRoutePermissions pins who reaches the shared feature state.
// Every staff member's shell reads it, so GET is PermView. Changing it is a
// Settings action for an admin, so PUT is PermAdmin. Entity-service does not
// re-check the role, so this registration is the only admin gate.
func TestFeatureOverrideRoutePermissions(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	reg := regexp.MustCompile(`(?m)^\s*route\("([A-Z]+) (/features)",\s*(handler\.Perm\w+),`)
	got := map[string]string{}
	for _, m := range reg.FindAllStringSubmatch(string(src), -1) {
		got[m[1]] = m[3]
	}
	if got["GET"] != "handler.PermView" {
		t.Errorf("GET /features is registered with %q, want handler.PermView", got["GET"])
	}
	if got["PUT"] != "handler.PermAdmin" {
		t.Errorf("PUT /features is registered with %q, want handler.PermAdmin", got["PUT"])
	}
	if len(got) != 2 {
		t.Errorf("expected exactly GET and PUT /features, found %v", got)
	}
}
