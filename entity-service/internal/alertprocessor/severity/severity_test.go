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

package severity

import (
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

func TestImpactUrgency(t *testing.T) {
	tests := map[string][2]string{
		"critical": {"HIGH", "HIGH"}, "major": {"MEDIUM", "HIGH"}, "minor": {"MEDIUM", "MEDIUM"},
		"warning": {"MEDIUM", "LOW"}, "typo": {"HIGH", "HIGH"},
	}
	for label, want := range tests {
		impact, urgency := ImpactUrgency(label)
		if string(impact) != want[0] || string(urgency) != want[1] {
			t.Errorf("%s: %s/%s, want %s/%s", label, impact, urgency, want[0], want[1])
		}
	}
}

func TestIsResolving(t *testing.T) {
	for label, want := range map[string]bool{"OK": true, " clear ": true, "critical": false, "typo": false} {
		if got := IsResolving(label); got != want {
			t.Errorf("IsResolving(%q) = %v, want %v", label, got, want)
		}
	}
}

func TestContactType(t *testing.T) {
	for source, want := range map[string]domain.IncidentContactType{"Azure Monitor": domain.IncidentContactTypeAzure, "site-24x7": domain.IncidentContactTypeSite247} {
		if got := ContactType(source); got == nil || *got != want {
			t.Errorf("ContactType(%q) = %v, want %s", source, got, want)
		}
	}
	if got := ContactType("prometheus"); got != nil {
		t.Errorf("ContactType(prometheus) = %s, want none", *got)
	}
}
