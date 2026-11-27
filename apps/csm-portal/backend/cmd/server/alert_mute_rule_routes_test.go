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

// TestAlertMuteRuleRoutePermissions pins the /alert-mute-rules registrations: reads
// need PermViewOperations and writes PermWrite, like every other Operations
// record, and GET /alert-mute-rules/active (alert-core's feed, served by
// csm-integration-service) is never exposed to the portal.
func TestAlertMuteRuleRoutePermissions(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"POST /alert-mute-rules/search":           "handler.PermViewOperations",
		"GET /alert-mute-rules/{id}":              "handler.PermViewOperations",
		"GET /alert-mute-rules/{id}/muted-alerts": "handler.PermViewOperations",
		"GET /alert-field-values":                 "handler.PermViewOperations",
		"GET /alert-mute-rules/muted-counts":      "handler.PermViewOperations",
		"POST /alert-mute-rules":                  "handler.PermWrite",
		"PATCH /alert-mute-rules/{id}":            "handler.PermWrite",
		"POST /alert-mute-rules/{id}/extend":      "handler.PermWrite",
		"POST /alert-mute-rules/{id}/end":         "handler.PermWrite",
		"DELETE /alert-mute-rules/{id}":           "handler.PermWrite",
	}
	got := map[string]string{}
	for _, m := range regexp.MustCompile(`(?m)^\s*route\("([A-Z]+ /alert-(?:mute-rules|field-values)[^"]*)", ([\w.]+),`).FindAllStringSubmatch(string(src), -1) {
		got[m[1]] = m[2]
	}
	for route, perm := range want {
		if got[route] != perm {
			t.Errorf("%s: registered with %q, want %q", route, got[route], perm)
		}
	}
	for route := range got {
		if _, ok := want[route]; !ok {
			t.Errorf("unexpected alert-mute-rule route %s", route)
		}
	}
	if regexp.MustCompile(`"GET /alert-mute-rules/active"`).Match(src) {
		t.Error("GET /alert-mute-rules/active must not be exposed to the portal")
	}
}
