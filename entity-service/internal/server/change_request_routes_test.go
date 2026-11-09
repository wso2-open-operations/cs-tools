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

package server

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestChangeRequestLinkOptionsIsInternalOnly pins the access guard on
// POST /change-requests/link-options. The response lists a project's
// deployments and its registered customer contacts for whatever project id the
// caller names, so an external caller must not be able to reach it. The guard
// itself (internalOnly) is covered by TestInternalOnly; this checks it is
// actually applied to the route.
func TestChangeRequestLinkOptionsIsInternalOnly(t *testing.T) {
	src, err := os.ReadFile("routes.go")
	if err != nil {
		t.Fatalf("read routes.go: %v", err)
	}
	re := regexp.MustCompile(`HandleFunc\("POST /change-requests/link-options",\s*([^\n]+)`)
	matches := re.FindAllStringSubmatch(string(src), -1)
	if len(matches) == 0 {
		t.Fatal("POST /change-requests/link-options is not registered in routes.go")
	}
	for _, m := range matches {
		if !strings.HasPrefix(strings.TrimSpace(m[1]), "internalOnly(") {
			t.Errorf("POST /change-requests/link-options must be wrapped in internalOnly, got %q", strings.TrimSpace(m[1]))
		}
	}
}

// TestChangeRequestCreateIsInternalOnly pins the access guard on
// POST /change-requests. A create carries fields only staff may set (the
// deviation from the implementation plan among them), and the repository relies
// on no customer identity reaching it (its create paths refuse a customer's
// deviation only as a second line). The guard itself (internalOnly) is covered
// by TestInternalOnly; this checks it is actually applied to the route.
func TestChangeRequestCreateIsInternalOnly(t *testing.T) {
	src, err := os.ReadFile("routes.go")
	if err != nil {
		t.Fatalf("read routes.go: %v", err)
	}
	re := regexp.MustCompile(`HandleFunc\("POST /change-requests",\s*([^\n]+)`)
	matches := re.FindAllStringSubmatch(string(src), -1)
	if len(matches) == 0 {
		t.Fatal("POST /change-requests is not registered in routes.go")
	}
	for _, m := range matches {
		if !strings.HasPrefix(strings.TrimSpace(m[1]), "internalOnly(") {
			t.Errorf("POST /change-requests must be wrapped in internalOnly, got %q", strings.TrimSpace(m[1]))
		}
	}
}
