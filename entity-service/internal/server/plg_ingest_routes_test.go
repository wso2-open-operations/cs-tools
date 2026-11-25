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

// The ingest routes write registrations and set a company's owner user id, so
// only a service credential may call them.
func TestPlgIngestRoutesAreServiceOnly(t *testing.T) {
	src, err := os.ReadFile("plg_routes.go")
	if err != nil {
		t.Fatalf("read plg_routes.go: %v", err)
	}
	for _, route := range []string{"POST /plg/registrations/ingest", "POST /plg/ingest-failures"} {
		re := regexp.MustCompile(`HandleFunc\("` + regexp.QuoteMeta(route) + `",\s*([^\n]+)`)
		matches := re.FindAllStringSubmatch(string(src), -1)
		if len(matches) == 0 {
			t.Errorf("%s is not registered in plg_routes.go", route)
			continue
		}
		for _, m := range matches {
			if !strings.HasPrefix(strings.TrimSpace(m[1]), "serviceOnly(") {
				t.Errorf("%s must be wrapped in serviceOnly, got %q", route, strings.TrimSpace(m[1]))
			}
		}
	}
}
