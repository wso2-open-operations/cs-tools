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
	"bytes"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
)

// filterField finds the "values" of the first entry in a "filters" array
// whose "field" matches, and reports whether one was found.
func filterField(filters []any, field string) ([]any, bool) {
	for _, entry := range filters {
		m, ok := entry.(map[string]any)
		if !ok || m["field"] != field {
			continue
		}
		values, ok := m["values"].([]any)
		return values, ok
	}
	return nil, false
}

// TestFilterPresets_DashboardLocalResolvesAtLoadTime confirms a dashboard's
// own "filterPresets" is expanded into its literal fragment inside
// query.filters, once, at LoadDir time — no "preset" key or "filterPresets"
// object should survive into the loaded Dashboard.
func TestFilterPresets_DashboardLocalResolvesAtLoadTime(t *testing.T) {
	dir := t.TempDir()
	writeDefinition(t, dir, "d.json", `{
	  "id": "d", "displayName": "D", "type": "cs",
	  "filterPresets": {
	    "not-closed": {"field": "state", "op": "notIn", "values": ["closed", "resolved"]}
	  },
	  "widgets": [
	    {"id": "w", "displayName": "W", "resourceType": "case", "shape": "count", "gridWidth": 3,
	     "query": {"filters": [{"preset": "not-closed"}]}}
	  ]
	}`)

	got, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("LoadDir returned %d dashboards, want 1", len(got))
	}
	d := got[0]
	if d.FilterPresets != nil {
		t.Errorf("Dashboard.FilterPresets = %+v, want nil (cleared once resolved, never visible past load)", d.FilterPresets)
	}

	filters, ok := d.Widgets[0].Query["filters"].([]any)
	if !ok {
		t.Fatalf("widget Query has no \"filters\" array: %v", d.Widgets[0].Query)
	}
	values, ok := filterField(filters, "state")
	if !ok {
		t.Fatalf("widget Query filters has no \"state\" field entry (preset did not expand): %v", filters)
	}
	if len(values) != 2 || values[0] != "closed" || values[1] != "resolved" {
		t.Errorf("expanded preset state values = %v, want [closed resolved]", values)
	}
}

// TestFilterPresets_ExpandInAnyOfBranchAndPieSlice confirms preset
// references resolve in the two other places a literal filter object can
// appear: an "anyOf" branch's own "filters", and a PieSlice's own
// "query.filters".
func TestFilterPresets_ExpandInAnyOfBranchAndPieSlice(t *testing.T) {
	dir := t.TempDir()
	writeDefinition(t, dir, "d.json", `{
	  "id": "d", "displayName": "D", "type": "cs",
	  "filterPresets": {
	    "critical": {"field": "severity", "op": "in", "values": ["critical"]}
	  },
	  "widgets": [
	    {"id": "w", "displayName": "W", "resourceType": "case", "shape": "count", "gridWidth": 3,
	     "query": {"anyOf": [{"filters": [{"preset": "critical"}]}]}},
	    {"id": "p", "displayName": "P", "resourceType": "case", "shape": "pie", "gridWidth": 6,
	     "query": {"filters": []},
	     "slices": [{"label": "Critical", "query": {"filters": [{"preset": "critical"}]}}]}
	  ]
	}`)

	got, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir returned error: %v", err)
	}

	anyOf, ok := got[0].Widgets[0].Query["anyOf"].([]any)
	if !ok || len(anyOf) != 1 {
		t.Fatalf("widget Query anyOf = %v, want one branch", got[0].Widgets[0].Query["anyOf"])
	}
	branch, _ := anyOf[0].(map[string]any)
	branchFilters, _ := branch["filters"].([]any)
	values, ok := filterField(branchFilters, "severity")
	if !ok || len(values) != 1 || values[0] != "critical" {
		t.Errorf("anyOf[0].filters severity = %v, want [critical] (preset did not expand)", values)
	}

	sliceFilters, _ := got[0].Widgets[1].Slices[0].Query["filters"].([]any)
	sliceValues, ok := filterField(sliceFilters, "severity")
	if !ok || len(sliceValues) != 1 || sliceValues[0] != "critical" {
		t.Errorf("slice Query filters severity = %v, want [critical] (preset did not expand)", sliceValues)
	}
}

// TestFilterPresets_UnknownKeyFailsLoudNamingDashboardWidgetAndKey mirrors
// registry.go's existing unknown-resourceType/unknown-shape error style: name
// the dashboard, the widget, and here specifically the missing preset key.
func TestFilterPresets_UnknownKeyFailsLoudNamingDashboardWidgetAndKey(t *testing.T) {
	dir := t.TempDir()
	writeDefinition(t, dir, "d.json", `{
	  "id": "d", "displayName": "D", "type": "cs",
	  "widgets": [
	    {"id": "w", "displayName": "W", "resourceType": "case", "shape": "count", "gridWidth": 3,
	     "query": {"filters": [{"preset": "does-not-exist"}]}}
	  ]
	}`)

	_, err := LoadDir(dir)
	if err == nil {
		t.Fatal("LoadDir returned no error for a reference to an undefined preset")
	}
	for _, want := range []string{filepath.Join(dir, "d.json"), `id "d"`, `widget "w"`, "does-not-exist"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err.Error(), want)
		}
	}
}

// TestFilterPresets_DashboardLocalShadowsSharedOnCollision is the precedence
// rule spelled out in the design: a dashboard-local preset wins over a
// same-named shared one, so a dashboard can deliberately override a shared
// default without editing the shared file.
func TestFilterPresets_DashboardLocalShadowsSharedOnCollision(t *testing.T) {
	dir := t.TempDir()
	presetsPath := filepath.Join(dir, "_presets.json")
	writeDefinition(t, dir, "_presets.json", `{
	  "not-closed": {"field": "state", "op": "notIn", "values": ["closed"]}
	}`)
	writeDefinition(t, dir, "d.json", `{
	  "id": "d", "displayName": "D", "type": "cs",
	  "filterPresets": {
	    "not-closed": {"field": "state", "op": "notIn", "values": ["closed", "wont_fix"]}
	  },
	  "widgets": [
	    {"id": "w", "displayName": "W", "resourceType": "case", "shape": "count", "gridWidth": 3,
	     "query": {"filters": [{"preset": "not-closed"}]}}
	  ]
	}`)

	sharedPresets, err := LoadSharedPresets(presetsPath)
	if err != nil {
		t.Fatalf("LoadSharedPresets returned error: %v", err)
	}
	got, err := loadDir(dir, sharedPresets, nil)
	if err != nil {
		t.Fatalf("loadDir returned error: %v", err)
	}

	filters, _ := got[0].Widgets[0].Query["filters"].([]any)
	values, ok := filterField(filters, "state")
	if !ok {
		t.Fatalf("widget Query filters has no \"state\" field entry: %v", filters)
	}
	// The dashboard-local fragment (["closed", "wont_fix"]), not the shared
	// one (["closed"]), must win.
	if len(values) != 2 || values[0] != "closed" || values[1] != "wont_fix" {
		t.Errorf("expanded preset state values = %v, want the dashboard-local fragment [closed wont_fix], not the shared one", values)
	}
}

// TestFilterPresets_SharedFileAlsoResolves confirms the non-collision case:
// a preset defined only in the shared file still resolves for a dashboard
// that references it and defines no "filterPresets" of its own.
func TestFilterPresets_SharedFileAlsoResolves(t *testing.T) {
	dir := t.TempDir()
	presetsPath := filepath.Join(dir, "_presets.json")
	writeDefinition(t, dir, "_presets.json", `{
	  "not-closed": {"field": "state", "op": "notIn", "values": ["closed"]}
	}`)
	writeDefinition(t, dir, "d.json", `{
	  "id": "d", "displayName": "D", "type": "cs",
	  "widgets": [
	    {"id": "w", "displayName": "W", "resourceType": "case", "shape": "count", "gridWidth": 3,
	     "query": {"filters": [{"preset": "not-closed"}]}}
	  ]
	}`)

	sharedPresets, err := LoadSharedPresets(presetsPath)
	if err != nil {
		t.Fatalf("LoadSharedPresets returned error: %v", err)
	}
	got, err := loadDir(dir, sharedPresets, nil)
	if err != nil {
		t.Fatalf("loadDir returned error: %v", err)
	}
	filters, _ := got[0].Widgets[0].Query["filters"].([]any)
	values, ok := filterField(filters, "state")
	if !ok || len(values) != 1 || values[0] != "closed" {
		t.Errorf("expanded shared preset state values = %v, want [closed]", values)
	}
}

// TestLoadSharedPresets_MissingPathIsLegal mirrors LoadDir's own empty-
// directory legality: an unset/empty DASHBOARD_PRESETS_FILE must not fail
// startup.
func TestLoadSharedPresets_MissingPathIsLegal(t *testing.T) {
	got, err := LoadSharedPresets("")
	if err != nil {
		t.Fatalf("LoadSharedPresets(\"\") returned error: %v", err)
	}
	if got != nil {
		t.Errorf("LoadSharedPresets(\"\") = %v, want nil", got)
	}
}

// TestLoadSharedPresets_UnreadableOrMalformedFailsNamingFile.
func TestLoadSharedPresets_UnreadableOrMalformedFailsNamingFile(t *testing.T) {
	dir := t.TempDir()
	path := writeDefinition(t, dir, "presets.json", `{"broken": `)

	_, err := LoadSharedPresets(path)
	if err == nil {
		t.Fatal("LoadSharedPresets returned no error for a malformed file")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error %q does not name the offending file", err.Error())
	}
}

// TestFilterPresets_RecursivePresetRejected covers both halves of "presets
// are not recursive": a shared preset whose own fragment is a {"preset":...}
// reference, and a dashboard-local one, must both fail loud at load rather
// than being silently passed through as an unresolved reference.
func TestFilterPresets_RecursivePresetRejected(t *testing.T) {
	t.Run("shared", func(t *testing.T) {
		dir := t.TempDir()
		path := writeDefinition(t, dir, "presets.json", `{
		  "a": {"preset": "b"},
		  "b": {"field": "state", "op": "in", "values": ["open"]}
		}`)
		_, err := LoadSharedPresets(path)
		if err == nil {
			t.Fatal("LoadSharedPresets returned no error for a preset referencing another preset")
		}
		if !strings.Contains(err.Error(), `"a"`) {
			t.Errorf("error %q does not name the offending preset key", err.Error())
		}
	})

	t.Run("dashboard-local", func(t *testing.T) {
		dir := t.TempDir()
		writeDefinition(t, dir, "d.json", `{
		  "id": "d", "displayName": "D", "type": "cs",
		  "filterPresets": {"a": {"preset": "b"}},
		  "widgets": [
		    {"id": "w", "displayName": "W", "resourceType": "case", "shape": "count", "gridWidth": 3,
		     "query": {"filters": []}}
		  ]
		}`)
		_, err := LoadDir(dir)
		if err == nil {
			t.Fatal("LoadDir returned no error for a dashboard-local preset referencing another preset")
		}
		if !strings.Contains(err.Error(), `"a"`) {
			t.Errorf("error %q does not name the offending preset key", err.Error())
		}
	})
}

// TestFilterPresets_MalformedReferenceRejected covers the two ways a
// {"preset": ...} object can be malformed: a non-string/empty "preset"
// value, and extra keys alongside "preset" (ambiguous — is it a reference or
// a literal filter that happens to have a "preset" field?).
func TestFilterPresets_MalformedReferenceRejected(t *testing.T) {
	cases := []struct {
		name  string
		entry string
	}{
		{"non-string preset value", `{"preset": 1}`},
		{"empty preset value", `{"preset": ""}`},
		{"extra keys alongside preset", `{"preset": "not-closed", "field": "state"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeDefinition(t, dir, "d.json", `{
			  "id": "d", "displayName": "D", "type": "cs",
			  "filterPresets": {"not-closed": {"field": "state", "op": "notIn", "values": ["closed"]}},
			  "widgets": [
			    {"id": "w", "displayName": "W", "resourceType": "case", "shape": "count", "gridWidth": 3,
			     "query": {"filters": [`+tc.entry+`]}}
			  ]
			}`)
			if _, err := LoadDir(dir); err == nil {
				t.Fatalf("LoadDir accepted a malformed preset reference %s; expected an error", tc.entry)
			}
		})
	}
}

// TestResourceTypeAutoInject_AllFiveCaseTableValues confirms every one of
// the five case-table resourceType values gets its own implied "type"
// filter auto-injected, with the resourceType string itself as the value.
func TestResourceTypeAutoInject_AllFiveCaseTableValues(t *testing.T) {
	for _, rt := range []ResourceType{
		ResourceCase, ResourceServiceRequest, ResourceSecurityReportAnalysis,
		ResourceAnnouncement, ResourceEngagement,
	} {
		t.Run(string(rt), func(t *testing.T) {
			dir := t.TempDir()
			writeDefinition(t, dir, "d.json", `{
			  "id": "d", "displayName": "D", "type": "cs",
			  "widgets": [
			    {"id": "w", "displayName": "W", "resourceType": "`+string(rt)+`", "shape": "count", "gridWidth": 3,
			     "query": {"filters": [{"field": "state", "op": "in", "values": ["open"]}]}}
			  ]
			}`)
			got, err := LoadDir(dir)
			if err != nil {
				t.Fatalf("LoadDir returned error: %v", err)
			}
			filters, ok := got[0].Widgets[0].Query["filters"].([]any)
			if !ok {
				t.Fatalf("widget Query has no \"filters\" array: %v", got[0].Widgets[0].Query)
			}
			values, ok := filterField(filters, "type")
			if !ok || len(values) != 1 || values[0] != string(rt) {
				t.Errorf("auto-injected type filter values = %v, want [%q]", values, rt)
			}
		})
	}
}

// TestResourceTypeAutoInject_NonCaseTableResourceTypeUntouched confirms the
// auto-inject is scoped to the five case-table values only — a widget of any
// other resourceType (e.g. "time_card") must not gain a "type" filter it
// never asked for.
func TestResourceTypeAutoInject_NonCaseTableResourceTypeUntouched(t *testing.T) {
	dir := t.TempDir()
	writeDefinition(t, dir, "d.json", `{
	  "id": "d", "displayName": "D", "type": "cs",
	  "widgets": [
	    {"id": "w", "displayName": "W", "resourceType": "time_card", "shape": "count", "gridWidth": 3,
	     "query": {"states": ["pending"]}}
	  ]
	}`)
	got, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir returned error: %v", err)
	}
	if _, present := got[0].Widgets[0].Query["filters"]; present {
		t.Errorf("non-case-table widget Query unexpectedly gained a \"filters\" key: %v", got[0].Widgets[0].Query)
	}
}

// TestResourceTypeAutoInject_ExplicitTypeFilterLeftAloneButWarns is the
// transition safety net: a widget already carrying its own explicit "type"
// filter (pre-migration config) must not have it overwritten or duplicated,
// but the load must warn, since after the config rewrite this pass
// backstops, no widget should still need one.
func TestResourceTypeAutoInject_ExplicitTypeFilterLeftAloneButWarns(t *testing.T) {
	var buf bytes.Buffer
	prevLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { slog.SetDefault(prevLogger) })

	dir := t.TempDir()
	writeDefinition(t, dir, "d.json", `{
	  "id": "d", "displayName": "D", "type": "cs",
	  "widgets": [
	    {"id": "w", "displayName": "W", "resourceType": "case", "shape": "count", "gridWidth": 3,
	     "query": {"filters": [{"field": "type", "op": "in", "values": ["case"]}]}}
	  ]
	}`)
	got, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir returned error: %v", err)
	}

	filters, _ := got[0].Widgets[0].Query["filters"].([]any)
	if len(filters) != 1 {
		t.Fatalf("widget Query filters = %v, want the single explicit \"type\" entry left untouched, not duplicated", filters)
	}
	values, ok := filterField(filters, "type")
	if !ok || len(values) != 1 || values[0] != "case" {
		t.Errorf("explicit type filter values = %v, want [case] (unmodified)", values)
	}

	if !strings.Contains(buf.String(), "already has an explicit") {
		t.Errorf("expected a warning about the redundant explicit \"type\" filter, got log output: %s", buf.String())
	}
}

// TestResourceTypeAutoInject_SliceGetsItsOwnInjection covers the merge
// nuance called out in injectImpliedTypeFilters' doc comment: a PieSlice
// that defines its own "filters" array shadows the widget's base "filters"
// entirely on the frontend (a shallow, whole-key merge), so the slice needs
// its own injected "type" entry independently of the widget's.
func TestResourceTypeAutoInject_SliceGetsItsOwnInjection(t *testing.T) {
	dir := t.TempDir()
	writeDefinition(t, dir, "d.json", `{
	  "id": "d", "displayName": "D", "type": "cs",
	  "widgets": [
	    {"id": "w", "displayName": "W", "resourceType": "engagement", "shape": "pie", "gridWidth": 6,
	     "query": {"filters": [{"field": "state", "op": "in", "values": ["open"]}]},
	     "slices": [{"label": "Mine", "query": {"filters": [{"field": "assignedUserId", "op": "in", "values": ["__current_user__"]}]}}]}
	  ]
	}`)
	got, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir returned error: %v", err)
	}
	sliceFilters, _ := got[0].Widgets[0].Slices[0].Query["filters"].([]any)
	values, ok := filterField(sliceFilters, "type")
	if !ok || len(values) != 1 || values[0] != "engagement" {
		t.Errorf("slice auto-injected type filter values = %v, want [engagement]", values)
	}
}
