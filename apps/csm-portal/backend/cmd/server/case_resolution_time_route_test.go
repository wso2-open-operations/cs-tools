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

// TestCaseResolutionTimeRoutePermission pins POST /cases/resolution-time to
// PermView, as POST /cases/aggregate is: it reads the caller's own cases, which
// the entity service scopes to them.
func TestCaseResolutionTimeRoutePermission(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^\s*route\("POST /cases/resolution-time",\s*(handler\.Perm\w+),\s*caseHandler\.CaseResolutionTime\)`).FindStringSubmatch(string(src))
	if m == nil {
		t.Fatal(`POST /cases/resolution-time is not registered with route(...) to caseHandler.CaseResolutionTime in main.go`)
	}
	if m[1] != "handler.PermView" {
		t.Errorf("POST /cases/resolution-time is registered with %s, want handler.PermView", m[1])
	}
}
