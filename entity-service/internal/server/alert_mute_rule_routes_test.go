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

// TestAlertMuteRuleRoutesAreInternalOnly pins the access guard on every alert mute
// rule route. A rule carries created_by / updated_by / ended_by (the actor's work
// email) and the free-text search matches created_by, so no external caller may
// reach any of them. Row-level security (migration 100028) is the second layer;
// TestAlertMuteRuleIntegration_SearchStatesAndQuery pins that policy.
func TestAlertMuteRuleRoutesAreInternalOnly(t *testing.T) {
	src, err := os.ReadFile("routes.go")
	if err != nil {
		t.Fatalf("read routes.go: %v", err)
	}
	re := regexp.MustCompile(`HandleFunc\("([A-Z]+ /alert-mute-rules[^"]*)",\s*([^\n]+)`)
	matches := re.FindAllStringSubmatch(string(src), -1)
	if len(matches) < 8 {
		t.Fatalf("found %d alert mute rule routes in routes.go, want at least 8", len(matches))
	}
	for _, m := range matches {
		if !strings.HasPrefix(strings.TrimSpace(m[2]), "internalOnly(") {
			t.Errorf("%s must be wrapped in internalOnly, got %q", m[1], strings.TrimSpace(m[2]))
		}
	}
}
