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
// take -- bar only (never pie: the six bands overlap, so they can't be
// wedges of one whole) with groupBy.bucket "severity" -- with and without a
// CRE team restriction.
func TestParseGenerated_AcceptsMTTRWidgets(t *testing.T) {
	raw := mttrDashboard(
		`{"id":"mttr-3m","displayName":"MTTR (Last 3 Months)","resourceType":"mttr","shape":"bar","gridWidth":6,
		  "query":{"months":"3"},"groupBy":{"bucket":"severity"}}`,
		`{"id":"mttr-rigel","displayName":"Rigel: MTTR by Severity","resourceType":"mttr","shape":"bar","gridWidth":6,
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
// missing months, a missing groupBy, a groupBy.field, count/list/pie/line
// shapes, and slices are all refused -- an MTTR widget has no record to
// count or list, its one bucket is fixed, and its bands overlap so a pie
// would never be meaningful.
func TestParseGenerated_RejectsInvalidMTTRWidgets(t *testing.T) {
	cases := []string{
		`{"id":"bad-months","displayName":"x","resourceType":"mttr","shape":"bar","gridWidth":6,
		  "query":{"months":"2"},"groupBy":{"bucket":"severity"}}`,
		`{"id":"no-months","displayName":"x","resourceType":"mttr","shape":"bar","gridWidth":6,
		  "query":{},"groupBy":{"bucket":"severity"}}`,
		`{"id":"no-group","displayName":"x","resourceType":"mttr","shape":"bar","gridWidth":6,
		  "query":{"months":"3"},"slices":[{"label":"S1","query":{"months":"3"}}]}`,
		`{"id":"wrong-field","displayName":"x","resourceType":"mttr","shape":"bar","gridWidth":6,
		  "query":{"months":"3"},"groupBy":{"field":"severity"}}`,
		`{"id":"a-count","displayName":"x","resourceType":"mttr","shape":"count","gridWidth":3,
		  "query":{"months":"3"}}`,
		`{"id":"a-pie","displayName":"x","resourceType":"mttr","shape":"pie","gridWidth":6,
		  "query":{"months":"3"},"groupBy":{"bucket":"severity"}}`,
	}
	for i, raw := range cases {
		if _, err := ParseGenerated([]byte(mttrDashboard(raw))); err == nil {
			t.Errorf("case %d: want an error, got none\n%s", i, raw)
		}
	}
}

// TestValidateWidgets_MTTRIsBarOnly: a hand-authored dashboard file (which
// goes through validateWidgets, not ParseGenerated/generatedResourceRules)
// must refuse an mttr widget with any shape but bar, any groupBy.bucket but
// "severity" (or a groupBy.field, or no groupBy at all), and any slices --
// the six bands overlap (S1-S4/S1-S3 are sums of the individual ones), so
// they can never be a pie's wedges of one whole or a per-slice query's own
// isolated result, and the report is always grouped server-side by that one
// fixed bucket, whichever path loaded the definition.
func TestValidateWidgets_MTTRIsBarOnly(t *testing.T) {
	base := WidgetTemplate{
		ID: "w", DisplayName: "W", ResourceType: ResourceMTTR, Shape: ShapeBar, GridWidth: 6,
		Query: map[string]any{"months": "3"},
	}
	widget := func(w WidgetTemplate) Dashboard {
		return Dashboard{ID: "d", Widgets: []WidgetTemplate{w}}
	}

	pie := base
	pie.Shape = ShapePie
	pie.GroupBy = &GroupByConfig{Bucket: "severity"}
	if err := validateWidgets(widget(pie), "test.json"); err == nil || !strings.Contains(err.Error(), `only supports shape "bar"`) {
		t.Errorf("pie: err = %v, want the bar-only refusal", err)
	}

	wrongBucket := base
	wrongBucket.GroupBy = &GroupByConfig{Bucket: "month"}
	if err := validateWidgets(widget(wrongBucket), "test.json"); err == nil || !strings.Contains(err.Error(), `groups only by "groupBy.bucket" "severity"`) {
		t.Errorf("wrong bucket: err = %v, want the severity-bucket refusal", err)
	}

	byField := base
	byField.GroupBy = &GroupByConfig{Field: "severity"}
	if err := validateWidgets(widget(byField), "test.json"); err == nil || !strings.Contains(err.Error(), `groups only by "groupBy.bucket" "severity"`) {
		t.Errorf("groupBy.field: err = %v, want the severity-bucket refusal", err)
	}

	noGroupBy := base
	noGroupBy.Slices = []PieSlice{{Label: "S1", Query: map[string]any{"months": "3"}}}
	if err := validateWidgets(widget(noGroupBy), "test.json"); err == nil || !strings.Contains(err.Error(), `has no "slices"`) {
		t.Errorf("slices: err = %v, want the no-slices refusal", err)
	}

	valid := base
	valid.GroupBy = &GroupByConfig{Bucket: "severity"}
	if err := validateWidgets(widget(valid), "test.json"); err != nil {
		t.Errorf("valid bar + severity bucket: err = %v, want no error", err)
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
