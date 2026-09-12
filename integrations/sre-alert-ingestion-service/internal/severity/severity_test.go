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

package severity_test

import (
	"testing"

	"github.com/wso2-open-operations/cs-tools/integrations/sre-alert-ingestion-service/internal/severity"
)

func TestMapImpactUrgency(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		want severity.ImpactUrgency
	}{
		{"critical", severity.ImpactUrgency{Impact: "HIGH", Urgency: "HIGH"}},
		{"CRITICAL", severity.ImpactUrgency{Impact: "HIGH", Urgency: "HIGH"}},
		{"  critical  ", severity.ImpactUrgency{Impact: "HIGH", Urgency: "HIGH"}},
		{"major", severity.ImpactUrgency{Impact: "HIGH", Urgency: "MEDIUM"}},
		{"minor", severity.ImpactUrgency{Impact: "MEDIUM", Urgency: "MEDIUM"}},
		{"warning", severity.ImpactUrgency{Impact: "LOW", Urgency: "MEDIUM"}},
		{"ok", severity.ImpactUrgency{Impact: "LOW", Urgency: "LOW"}},
		{"", severity.ImpactUrgency{Impact: "LOW", Urgency: "LOW"}},
		{"unknown-vendor-severity", severity.ImpactUrgency{Impact: "LOW", Urgency: "LOW"}},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			if got := severity.MapImpactUrgency(tc.in); got != tc.want {
				t.Errorf("MapImpactUrgency(%q) = %+v, want %+v", tc.in, got, tc.want)
			}
		})
	}
}

func TestMapContactType(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in     string
		wantCT string
		wantOK bool
	}{
		{"azure", "AZURE", true},
		{"AZURE", "AZURE", true},
		{"site24x7", "SITE_247", true},
		{"sentinel", "SENTINEL", true},
		{"microsoft-sentinel", "SENTINEL", true},
		{"datadog", "", false},
		{"grafana", "", false},
		{"prometheus", "", false},
		{"", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			ct, ok := severity.MapContactType(tc.in)
			if ct != tc.wantCT || ok != tc.wantOK {
				t.Errorf("MapContactType(%q) = (%q, %v), want (%q, %v)", tc.in, ct, ok, tc.wantCT, tc.wantOK)
			}
		})
	}
}

func TestMapCategory(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		want string
	}{
		{"INQUIRY", "INQUIRY"},
		{"inquiry", "INQUIRY"},
		{"service_interruption", "SERVICE_INTERRUPTION"},
		{"SECURITY", "SECURITY"},
		{"", "SERVICE_INTERRUPTION"},
		{"bogus", "SERVICE_INTERRUPTION"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			if got := severity.MapCategory(tc.in); got != tc.want {
				t.Errorf("MapCategory(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
