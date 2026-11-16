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
package dashboard

import (
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"
)

// mttrDashboard wraps widgets in a generated cs dashboard.
func mttrDashboard(widgets ...string) string {
	return `{"displayName":"MTTR","type":"cs","isDefault":false,"targetTeam":"","isTeamBased":false,"widgets":[` + strings.Join(widgets, ",") + `]}`
}

// TestParseGenerated_AcceptsMTTRWidgets: the only shape an MTTR widget may
// take -- pie/bar with groupBy.bucket "severity" -- with and without a CRE
// team restriction.
func TestParseGenerated_AcceptsMTTRWidgets(t *testing.T) {
	raw := mttrDashboard(
		`{"id":"mttr-3m","displayName":"MTTR (Last 3 Months)","resourceType":"mttr","shape":"bar","gridWidth":6,
		  "query":{"months":"3"},"groupBy":{"bucket":"severity"}}`,
		`{"id":"mttr-rigel","displayName":"Rigel: MTTR by Severity","resourceType":"mttr","shape":"pie","gridWidth":6,
		  "query":{"months":"12","creTeamIds":["rigel"]},"groupBy":{"bucket":"severity"}}`,
	)
	d, err := ParseGenerated([]byte(raw))
	if err != nil {
		t.Fatalf("ParseGenerated: %v", err)
	}
	if len(d.Widgets) != 2 || d.Widgets[0].ResourceType != ResourceMTTR {
		t.Fatalf("widgets = %+v", d.Widgets)
	}
	if g := d.Widgets[0].GroupBy; g == nil || g.Bucket != "severity" || g.Field != "" {
		t.Errorf("groupBy = %+v, want bucket \"severity\", no field", g)
	}
}

// TestParseGenerated_RejectsInvalidMTTRWidgets: an unknown months value, a
// missing groupBy, a groupBy.field, count/list/line shapes, and slices are
// all refused -- an MTTR widget has no record to count or list, and its one
// bucket is fixed.
func TestParseGenerated_RejectsInvalidMTTRWidgets(t *testing.T) {
	cases := []string{
		`{"id":"bad-months","displayName":"x","resourceType":"mttr","shape":"bar","gridWidth":6,
		  "query":{"months":"2"},"groupBy":{"bucket":"severity"}}`,
		`{"id":"no-group","displayName":"x","resourceType":"mttr","shape":"bar","gridWidth":6,
		  "query":{"months":"3"},"slices":[{"label":"S1","query":{"months":"3"}}]}`,
		`{"id":"wrong-field","displayName":"x","resourceType":"mttr","shape":"bar","gridWidth":6,
		  "query":{"months":"3"},"groupBy":{"field":"severity"}}`,
		`{"id":"a-count","displayName":"x","resourceType":"mttr","shape":"count","gridWidth":3,
		  "query":{"months":"3"}}`,
	}
	for i, raw := range cases {
		if _, err := ParseGenerated([]byte(mttrDashboard(raw))); err == nil {
			t.Errorf("case %d: want an error, got none\n%s", i, raw)
		}
	}
}

// TestParseGenerated_TheCatalogueMTTRExamplesAreValid: every MTTR widget the
// catalogue shows the model (section 5f) passes this backend's own check, so
// the catalogue cannot teach a widget the save would refuse.
func TestParseGenerated_TheCatalogueMTTRExamplesAreValid(t *testing.T) {
	if _, err := os.Stat(aiIntegrationService); errors.Is(err, fs.ErrNotExist) {
		t.Skipf("the AI integration service (%s) is not in this checkout", aiIntegrationService)
	}
	raw, err := os.ReadFile(serviceCatalogue)
	if err != nil {
		t.Fatalf("read the AI integration service's catalogue (%s): %v", serviceCatalogue, err)
	}
	_, section, ok := strings.Cut(string(raw), "### 5f. MTTR")
	if !ok {
		t.Fatal("catalogue has no section 5f")
	}
	section, _, _ = strings.Cut(section, "## 6.")
	var widgets []string
	for rest := section; ; {
		_, after, found := strings.Cut(rest, "```json\n")
		if !found {
			break
		}
		block, next, _ := strings.Cut(after, "```")
		if strings.Contains(block, `"resourceType": "mttr"`) {
			widgets = append(widgets, block)
		}
		rest = next
	}
	if len(widgets) < 2 {
		t.Fatalf("found %d mttr examples in section 5f, want at least 2", len(widgets))
	}
	for i, w := range widgets {
		if _, err := ParseGenerated([]byte(mttrDashboard(w))); err != nil {
			t.Errorf("section 5f example %d is rejected: %v\n%s", i+1, err, w)
		}
	}
}
