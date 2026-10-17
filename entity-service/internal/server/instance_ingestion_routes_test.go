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

// TestInstanceIngestionRoutesAreInternalOnly pins the access guard on the
// product-consumption routes: the ingestion writes have no project scoping of
// their own, the secret keys are the credentials telemetry and report
// signatures are checked with, and the provisioning status carries the
// project's client secret, so none may be reachable by an external caller (or
// by a request with no token at all).
func TestInstanceIngestionRoutesAreInternalOnly(t *testing.T) {
	src, err := os.ReadFile("routes.go")
	if err != nil {
		t.Fatalf("read routes.go: %v", err)
	}
	for _, route := range []string{
		"POST /instances/meta",
		"POST /instances/info",
		"POST /instances/usage-stats",
		"POST /instances/daily-usages/batch",
		"GET /projects/{key}/secret-key",
		"GET /deployments/by-number/{number}/consumption-secret",
		"GET /projects/{id}/consumption/status",
		"POST /projects/{id}/consumption/status",
		"PATCH /projects/{id}/consumption",
		"POST /projects/{id}/consumption",
	} {
		re := regexp.MustCompile(`HandleFunc\("` + regexp.QuoteMeta(route) + `",\s*([^\n]+)`)
		matches := re.FindAllStringSubmatch(string(src), -1)
		if len(matches) == 0 {
			t.Errorf("%s is not registered in routes.go", route)
			continue
		}
		for _, m := range matches {
			if !strings.HasPrefix(strings.TrimSpace(m[1]), "internalOnly(") {
				t.Errorf("%s must be wrapped in internalOnly, got %q", route, strings.TrimSpace(m[1]))
			}
		}
	}
}
