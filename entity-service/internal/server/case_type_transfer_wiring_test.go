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
	"strings"
	"testing"
)

// The case type transfer only exists where routes.go wires it
// (service.WithCaseTypeTransfer): the Postgres and dual-write branches. Left
// out, a type change is refused with the old "only supported for the ServiceNow
// data source" 400, which is how it failed for every case. This fails if either
// branch loses the call.
func TestRoutes_CaseTypeTransferIsWiredForBothPostgresBranches(t *testing.T) {
	src, err := os.ReadFile("routes.go")
	if err != nil {
		t.Fatalf("read routes.go: %v", err)
	}
	if got := strings.Count(string(src), "service.WithCaseTypeTransfer(activeCaseSvc, caseRepo)"); got != 2 {
		t.Errorf("routes.go wires the case type transfer %d time(s), want 2 (dual-write and plain Postgres)", got)
	}
}
