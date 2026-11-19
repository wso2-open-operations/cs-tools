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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package server

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestProductIssueSyncRoutesAreGatedAndServiceOnly pins two things about the
// reconciliation and backfill routes: they sit inside the git internals block
// (registered only when the flag is on, with an engineering entity URL and a
// database), and each is wrapped in serviceOnly (a user token, internal or not,
// gets 403: every call writes to many work items).
func TestProductIssueSyncRoutesAreGatedAndServiceOnly(t *testing.T) {
	b, err := os.ReadFile("routes.go")
	if err != nil {
		t.Fatalf("read routes.go: %v", err)
	}
	src := string(b)
	gate := strings.Index(src, "if cfg.GitInternalsActive() && db != nil")
	end := strings.Index(src[gate+1:], `mux.HandleFunc("GET /cases/{id}/escalations"`)
	if gate < 0 || end < 0 {
		t.Fatal("the git internals block was not found in routes.go")
	}
	block := src[gate : gate+1+end]
	for _, route := range []string{"POST /product-work-items/reconcile", "POST /product-work-items/backfill"} {
		re := regexp.MustCompile(`mux\.HandleFunc\("` + regexp.QuoteMeta(route) + `",\s*serviceOnly\(`)
		if !re.MatchString(block) {
			t.Errorf("%s must be registered inside the git internals block and wrapped in serviceOnly", route)
		}
		if strings.Count(src, `"`+route+`"`) != 1 {
			t.Errorf("%s must be registered exactly once", route)
		}
	}
}
