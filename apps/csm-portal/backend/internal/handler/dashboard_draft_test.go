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

package handler

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/dashboard"
)

// authoredDraftDashboard is a definition as an author (or the model) writes
// it: no "type" filters on the case-family widgets, one explicit type filter
// that is not the implied one, and a non-case widget whose own type filter
// must survive. It uses every widget field the served form carries.
const authoredDraftDashboard = `{
  "id": "ai-draft-round-trip",
  "displayName": "Draft Round Trip",
  "type": "cre",
  "isDefault": false,
  "targetTeam": "",
  "isTeamBased": true,
  "widgets": [
    {"id": "open", "displayName": "Open", "description": "Open cases", "resourceType": "case", "shape": "count",
     "gridWidth": 3, "query": {"filters": [{"field": "state", "op": "in", "values": ["open"]}]}},
    {"id": "everything", "displayName": "Everything", "resourceType": "service_request", "shape": "count",
     "gridWidth": 3, "query": {}},
    {"id": "mixed", "displayName": "Cases and engagements", "resourceType": "case", "shape": "count", "gridWidth": 3,
     "query": {"filters": [{"field": "type", "op": "in", "values": ["case", "engagement"]}]}},
    {"id": "by-state", "displayName": "By state", "resourceType": "case", "shape": "pie", "gridWidth": 6,
     "query": {"filters": [{"field": "assignedUserId", "op": "in", "values": ["__current_user__"]}]},
     "inlineDrilldown": true, "inlineLabels": true, "section": "Mine",
     "slices": [
       {"label": "Open", "color": "info", "query": {"filters": [{"field": "state", "op": "in", "values": ["open"]}]}},
       {"label": "Closed", "query": {"filters": [{"field": "state", "op": "in", "values": ["closed"]}]}}
     ]},
    {"id": "grouped", "displayName": "By account", "resourceType": "case", "shape": "bar", "gridWidth": 6,
     "query": {}, "groupBy": {"field": "account"}},
    {"id": "recent", "displayName": "Recent", "resourceType": "engagement", "shape": "list", "gridWidth": 12,
     "query": {"filters": [{"field": "state", "op": "notIn", "values": ["closed"]}]}, "listLimit": 10,
     "columns": [{"path": "number", "label": "Number"}, {"path": "subject", "label": "Subject"}], "sortBy": {"field": "createdOn", "order": "desc"}},
    {"id": "trend", "displayName": "Trend", "resourceType": "case", "shape": "line", "gridWidth": 12,
     "query": {}, "days": 30}
  ]
}`

// servedDraft loads authored the way the registry does and returns it in the
// served form, as the webapp holds it and sends it back as "current".
func servedDraft(t *testing.T, authored string) (dashboard.Dashboard, json.RawMessage) {
	t.Helper()
	loaded, err := dashboard.ParseDashboardsConfig("[" + authored + "]")
	if err != nil {
		t.Fatalf("load the authored dashboard: %v", err)
	}
	d := loaded[0]
	served, err := json.Marshal(dashboardDetailView{
		ID: d.ID, DisplayName: d.DisplayName, Type: d.Type, IsDefault: d.IsDefault,
		TargetTeam: d.TargetTeam, IsTeamBased: d.IsTeamBased, Widgets: widgetViews(d.Widgets),
	})
	if err != nil {
		t.Fatal(err)
	}
	return d, served
}

func TestDraftToDefinition_RoundTrip(t *testing.T) {
	loaded, served := servedDraft(t, authoredDraftDashboard)
	if !strings.Contains(string(served), `"widgetId"`) || !strings.Contains(string(served), `"values":["case"]`) {
		t.Fatalf("served form lacks widgetId or the injected type filter, so the test proves nothing: %s", served)
	}

	def, err := draftToDefinition(served)
	if err != nil {
		t.Fatalf("draftToDefinition: %v", err)
	}

	// Reads as authored: same JSON value (key order aside).
	if got, want := asJSONValue(t, def), asJSONValue(t, []byte(authoredDraftDashboard)); !reflect.DeepEqual(got, want) {
		t.Errorf("definition is not the authored dashboard\n got: %s\nwant: %s", def, authoredDraftDashboard)
	}

	// Loads back to the dashboard the draft was served from.
	reloaded, err := dashboard.ParseDashboardsConfig("[" + string(def) + "]")
	if err != nil {
		t.Fatalf("load the definition: %v", err)
	}
	if !reflect.DeepEqual(reloaded[0], loaded) {
		t.Errorf("reloaded dashboard differs from the one served\n got: %+v\nwant: %+v", reloaded[0], loaded)
	}
}

// A distinct-count widget keeps its distinctBy through the draft: without it,
// a follow-up edit would turn "distinct accounts" into a plain record count.
func TestDraftToDefinition_KeepsDistinctBy(t *testing.T) {
	const authored = `{"id": "ai-distinct", "displayName": "Distinct", "type": "cs", "isDefault": false, "targetTeam": "",
  "isTeamBased": false, "widgets": [{"id": "accounts", "displayName": "Accounts with open cases", "resourceType": "case",
  "shape": "count", "gridWidth": 3, "query": {"filters": [{"field": "state", "op": "in", "values": ["open"]}]},
  "distinctBy": {"field": "account", "maxGroups": 1000}}]}`
	loaded, served := servedDraft(t, authored)
	if !strings.Contains(string(served), `"distinctBy"`) {
		t.Fatalf("served form lacks distinctBy, so the test proves nothing: %s", served)
	}
	def, err := draftToDefinition(served)
	if err != nil {
		t.Fatalf("draftToDefinition: %v", err)
	}
	if got, want := asJSONValue(t, def), asJSONValue(t, []byte(authored)); !reflect.DeepEqual(got, want) {
		t.Errorf("definition lost or changed distinctBy\n got: %s\nwant: %s", def, authored)
	}
	reloaded, err := dashboard.ParseDashboardsConfig("[" + string(def) + "]")
	if err != nil {
		t.Fatalf("load the definition: %v", err)
	}
	if !reflect.DeepEqual(reloaded[0], loaded) {
		t.Errorf("reloaded dashboard differs from the one served\n got: %+v\nwant: %+v", reloaded[0], loaded)
	}
}

func TestDraftToDefinition_NoDraft(t *testing.T) {
	for _, current := range []string{"", "  ", "null", " null "} {
		def, err := draftToDefinition(json.RawMessage(current))
		if err != nil || def != nil {
			t.Errorf("draftToDefinition(%q) = %s, %v; want nil, nil", current, def, err)
		}
	}
	if def, err := draftToDefinition(nil); err != nil || def != nil {
		t.Errorf("draftToDefinition(nil) = %s, %v; want nil, nil", def, err)
	}
}

func TestDraftToDefinition_RejectsWhatWasNotServed(t *testing.T) {
	cases := map[string]string{
		"file format id":  `{"id":"ai-x","displayName":"X","widgets":[{"id":"w1","displayName":"W"}]}`,
		"unknown field":   `{"id":"ai-x","displayName":"X","restricted":true,"widgets":[]}`,
		"not an object":   `["ai-x"]`,
		"broken JSON":     `{"id":`,
		"trailing object": `{"id":"ai-x","widgets":[]} {"id":"ai-y"}`,
	}
	for name, current := range cases {
		if _, err := draftToDefinition(json.RawMessage(current)); !errors.Is(err, errDraftInvalid) {
			t.Errorf("%s: err = %v, want errDraftInvalid", name, err)
		}
	}
}

func TestStripImpliedTypeFilters_OnlyTheInjectedOne(t *testing.T) {
	implied := map[string]any{"field": "type", "op": "in", "values": []any{"case"}}
	d := dashboard.Dashboard{Widgets: []dashboard.WidgetTemplate{
		{ID: "a", ResourceType: dashboard.ResourceCase, Query: map[string]any{"filters": []any{implied}}},
		{ID: "b", ResourceType: dashboard.ResourceCase, Query: map[string]any{"filters": []any{
			map[string]any{"field": "type", "op": "in", "values": []any{"engagement"}},
			map[string]any{"field": "type", "op": "notIn", "values": []any{"case"}},
			map[string]any{"field": "type", "op": "in", "values": []any{"case"}, "extra": true},
		}}},
		{ID: "c", ResourceType: dashboard.ResourceType("incident"), Query: map[string]any{"filters": []any{
			map[string]any{"field": "type", "op": "in", "values": []any{"incident"}},
		}}},
		{ID: "d", ResourceType: dashboard.ResourceCase, Query: map[string]any{"filters": "not an array"}},
		{ID: "e", ResourceType: dashboard.ResourceCase},
	}}
	dashboard.StripImpliedTypeFilters(&d)

	if _, has := d.Widgets[0].Query["filters"]; has {
		t.Errorf("a: filters = %v, want removed (only the implied filter was there)", d.Widgets[0].Query["filters"])
	}
	if got := len(d.Widgets[1].Query["filters"].([]any)); got != 3 {
		t.Errorf("b: %d filters left, want all 3 (none is exactly the implied one)", got)
	}
	if got := len(d.Widgets[2].Query["filters"].([]any)); got != 1 {
		t.Errorf("c: %d filters left, want 1 (not a case-family widget)", got)
	}
	if d.Widgets[3].Query["filters"] != "not an array" {
		t.Errorf("d: filters = %v, want left alone", d.Widgets[3].Query["filters"])
	}
	if d.Widgets[4].Query != nil {
		t.Errorf("e: query = %v, want still nil", d.Widgets[4].Query)
	}
}

func asJSONValue(t *testing.T, raw []byte) any {
	t.Helper()
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	return v
}
