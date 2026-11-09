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

package catalogue

import (
	"strings"
	"testing"
)

func TestBuiltInCatalogue(t *testing.T) {
	if text := DashboardEmbedded(); !strings.HasPrefix(text, "# CSM Portal dashboard catalogue") ||
		!strings.Contains(text, "## 1. Output contract") || !strings.Contains(text, "## 2. Dashboard object") {
		first, _, _ := strings.Cut(text, "\n")
		t.Errorf("built-in dashboard catalogue starts with %q, want the catalogue", first)
	}
}

// TestNewDashboardIsTheBuiltInCopy checks that the dashboard catalogue is the
// copy built into the binary.
func TestNewDashboardIsTheBuiltInCopy(t *testing.T) {
	if NewDashboard().Text() != DashboardEmbedded() {
		t.Error("dashboard Text() is not the built-in catalogue")
	}
}
