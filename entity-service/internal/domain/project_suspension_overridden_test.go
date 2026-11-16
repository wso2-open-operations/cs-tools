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

package domain

import (
	"encoding/json"
	"testing"
)

// TestProjectClosureFields_SuspensionProcessOverriddenJSON pins that the flag is
// always present on the project read views: false by default, true when set.
func TestProjectClosureFields_SuspensionProcessOverriddenJSON(t *testing.T) {
	views := map[string]any{
		"project": func(o bool) any {
			return Project{ProjectClosureFields: ProjectClosureFields{SuspensionProcessOverridden: o}}
		},
		"projectDetails": func(o bool) any {
			return ProjectDetailsView{ProjectClosureFields: ProjectClosureFields{SuspensionProcessOverridden: o}}
		},
		"projectView": func(o bool) any {
			return ProjectView{ProjectClosureFields: ProjectClosureFields{SuspensionProcessOverridden: o}}
		},
	}
	for name, mk := range views {
		build := mk.(func(bool) any)
		for _, want := range []bool{false, true} {
			raw, err := json.Marshal(build(want))
			if err != nil {
				t.Fatalf("%s: marshal: %v", name, err)
			}
			var m map[string]json.RawMessage
			if err := json.Unmarshal(raw, &m); err != nil {
				t.Fatalf("%s: unmarshal: %v", name, err)
			}
			got, ok := m["suspensionProcessOverridden"]
			if !ok {
				t.Fatalf("%s: suspensionProcessOverridden missing from %s", name, raw)
			}
			if string(got) != map[bool]string{false: "false", true: "true"}[want] {
				t.Errorf("%s: suspensionProcessOverridden = %s, want %v", name, got, want)
			}
		}
	}
}
