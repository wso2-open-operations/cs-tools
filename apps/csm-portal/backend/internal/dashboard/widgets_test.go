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

import "testing"

func TestParseDashboardsConfig_Empty(t *testing.T) {
	got, err := ParseDashboardsConfig("")
	if err != nil {
		t.Fatalf("ParseDashboardsConfig(\"\") returned error: %v", err)
	}
	if got != nil {
		t.Errorf("ParseDashboardsConfig(\"\") = %v, want nil", got)
	}
}

// TestParseDashboardsConfig_Malformed locks in the deliberate reversal of the
// old behaviour: malformed config used to log and return nil, silently
// emptying every dashboard in the product. It is now an error the caller
// makes fatal.
func TestParseDashboardsConfig_Malformed(t *testing.T) {
	got, err := ParseDashboardsConfig("{not valid json")
	if err == nil {
		t.Fatalf("ParseDashboardsConfig(malformed) returned no error, want a rejection")
	}
	if got != nil {
		t.Errorf("ParseDashboardsConfig(malformed) = %v, want nil", got)
	}
}

func TestParseDashboardsConfig_MalformedShape(t *testing.T) {
	// Valid JSON, but not an array of Dashboard objects — must not panic and
	// must return nil, not a zero-value slice with garbage entries.
	got, err := ParseDashboardsConfig(`{"id":"not-an-array"}`)
	if err == nil {
		t.Fatalf("ParseDashboardsConfig(wrong shape) returned no error, want a rejection")
	}
	if got != nil {
		t.Errorf("ParseDashboardsConfig(wrong shape) = %v, want nil", got)
	}
}

func TestParseDashboardsConfig_ValidRoundTrip(t *testing.T) {
	const raw = `[
		{
			"id": "sample-dashboard",
			"displayName": "Sample Dashboard",
			"isDefault": true,
			"targetTeam": "sample-team",
			"widgets": [
				{
					"id": "my-open-cases",
					"displayName": "My Open Cases",
					"resourceType": "case",
					"shape": "count",
					"gridWidth": 3,
					"query": {
						"filters": [
							{"field": "assignedUserId", "op": "in", "values": ["__current_user__"]},
							{"field": "tag", "op": "in", "values": ["example-tag"]},
							{"field": "state", "op": "in", "values": ["open", "work_in_progress"]}
						]
					}
				}
			]
		}
	]`

	got, err := ParseDashboardsConfig(raw)
	if err != nil {
		t.Fatalf("ParseDashboardsConfig returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(ParseDashboardsConfig(raw)) = %d, want 1", len(got))
	}

	d := got[0]
	if d.ID != "sample-dashboard" {
		t.Errorf("Dashboard.ID = %q, want %q", d.ID, "sample-dashboard")
	}
	if d.DisplayName != "Sample Dashboard" {
		t.Errorf("Dashboard.DisplayName = %q, want %q", d.DisplayName, "Sample Dashboard")
	}
	if !d.IsDefault {
		t.Errorf("Dashboard.IsDefault = false, want true")
	}
	if d.TargetTeam != "sample-team" {
		t.Errorf("Dashboard.TargetTeam = %q, want %q", d.TargetTeam, "sample-team")
	}
	if len(d.Widgets) != 1 {
		t.Fatalf("len(Dashboard.Widgets) = %d, want 1", len(d.Widgets))
	}

	w := d.Widgets[0]
	if w.ID != "my-open-cases" {
		t.Errorf("WidgetTemplate.ID = %q, want %q", w.ID, "my-open-cases")
	}
	if w.ResourceType != ResourceCase {
		t.Errorf("WidgetTemplate.ResourceType = %q, want %q", w.ResourceType, ResourceCase)
	}
	if w.Shape != ShapeCount {
		t.Errorf("WidgetTemplate.Shape = %q, want %q", w.Shape, ShapeCount)
	}
	if w.GridWidth != 3 {
		t.Errorf("WidgetTemplate.GridWidth = %d, want 3", w.GridWidth)
	}

	// The "__current_user__" placeholder is deliberately left exactly as
	// authored: this package no longer substitutes it (that moved to the
	// frontend, see DashboardHandler's doc comment in
	// internal/handler/dashboards.go) — it is opaque config content here,
	// same as every other filter value.
	//
	// Query is opaque to this package (see widgets.go's WidgetTemplate doc
	// comment), so the specific case-search filter DSL shape used here
	// ({"filters":[{"field","op","values"}, ...]}, see .env.example's
	// DASHBOARDS_CONFIG) is just realistic example data.
	assignedEntryValues := func(query map[string]any) ([]any, bool) {
		arr, ok := query["filters"].([]any)
		if !ok || len(arr) == 0 {
			return nil, false
		}
		entry, ok := arr[0].(map[string]any)
		if !ok {
			return nil, false
		}
		values, ok := entry["values"].([]any)
		return values, ok
	}

	assigned, ok := assignedEntryValues(w.Query)
	if !ok {
		t.Fatalf("Query has no filters[0].values entry")
	}
	if len(assigned) != 1 || assigned[0] != "__current_user__" {
		t.Errorf("Query[filters][0][values] = %v, want the unresolved placeholder [%q]", assigned, "__current_user__")
	}
}

func TestParseDashboardsConfig_PieWidgetSlicesAndDescription(t *testing.T) {
	const raw = `[
		{
			"id": "sample-team-dashboard",
			"displayName": "Sample Team Dashboard",
			"widgets": [
				{
					"id": "cases-by-severity",
					"displayName": "Cases by Severity",
					"description": "Share of active cases at each severity level.",
					"resourceType": "case",
					"shape": "pie",
					"gridWidth": 6,
					"query": {"states": ["open"]},
					"slices": [
						{"label": "Critical", "color": "error", "query": {"severities": ["critical"]}},
						{"label": "Mine", "query": {"assignedUserIds": ["__current_user__"]}}
					]
				}
			]
		}
	]`

	got, err := ParseDashboardsConfig(raw)
	if err != nil {
		t.Fatalf("ParseDashboardsConfig returned error: %v", err)
	}
	if len(got) != 1 || len(got[0].Widgets) != 1 {
		t.Fatalf("ParseDashboardsConfig(raw) = %+v, want 1 dashboard with 1 widget", got)
	}

	w := got[0].Widgets[0]
	if w.Description != "Share of active cases at each severity level." {
		t.Errorf("WidgetTemplate.Description = %q, want the configured subtitle", w.Description)
	}
	if len(w.Slices) != 2 {
		t.Fatalf("len(WidgetTemplate.Slices) = %d, want 2", len(w.Slices))
	}
	if w.Slices[0].Label != "Critical" || w.Slices[0].Color != "error" {
		t.Errorf("Slices[0] = %+v, want {Label: Critical, Color: error}", w.Slices[0])
	}

	// A slice's own criteria keep the literal "__current_user__" placeholder
	// (substitution moved to the frontend, see DashboardHandler's doc
	// comment) and stay independent of the widget's own base Query — this
	// package never merges the two.
	sliceQuery := w.Slices[1].Query
	assigned, ok := sliceQuery["assignedUserIds"].([]any)
	if !ok || len(assigned) != 1 || assigned[0] != "__current_user__" {
		t.Errorf("Slices[1].Query[assignedUserIds] = %v, want the unresolved placeholder [%q]", sliceQuery["assignedUserIds"], "__current_user__")
	}
	if _, present := sliceQuery["states"]; present {
		t.Errorf("Slices[1].Query must not carry the widget's own base query, got %v", sliceQuery)
	}
}

// TestParseDashboardsConfig_LegacyKeys covers the un-migrated deployed
// environment: DASHBOARDS_CONFIG lives in an env var, so the widget-key
// rename (filters -> query, orGroups -> anyOf) is never atomic with a config
// rollout. encoding/json leaves an unknown key's field at its zero value, so
// without this compatibility path every widget would come back with a nil
// Query and silently render 0 — no error, no failed request, nothing to
// notice. Every assertion below is on the OLD config shape producing a
// working, fully-migrated widget.
func TestParseDashboardsConfig_LegacyKeys(t *testing.T) {
	const raw = `[
		{
			"id": "legacy-dashboard",
			"displayName": "Legacy Dashboard",
			"widgets": [
				{
					"id": "critical-or-escalated",
					"displayName": "Critical or Escalated",
					"resourceType": "case",
					"shape": "count",
					"gridWidth": 3,
					"filters": {
						"filters": [{"field": "state", "op": "in", "values": ["open"]}],
						"orGroups": [
							[
								{"field": "severity", "op": "in", "values": ["catastrophic"]},
								{"field": "workState", "op": "in", "values": ["ongoing"]}
							],
							[{"field": "escalationLevel", "op": "in", "values": ["3"]}]
						]
					}
				},
				{
					"id": "cases-by-severity",
					"displayName": "Cases by Severity",
					"resourceType": "case",
					"shape": "pie",
					"gridWidth": 6,
					"filters": {"filters": [{"field": "state", "op": "in", "values": ["open"]}]},
					"slices": [
						{"label": "Mine", "filters": {"filters": [{"field": "assignedUserId", "op": "in", "values": ["__current_user__"]}]}}
					]
				}
			]
		}
	]`

	got, err := ParseDashboardsConfig(raw)
	if err != nil {
		t.Fatalf("ParseDashboardsConfig returned error: %v", err)
	}
	if len(got) != 1 || len(got[0].Widgets) != 2 {
		t.Fatalf("ParseDashboardsConfig(legacy) = %+v, want 1 dashboard with 2 widgets", got)
	}

	// 1. The legacy widget-level "filters" key lands in Query.
	w := got[0].Widgets[0]
	if w.Query == nil {
		t.Fatalf(`legacy widget "filters" was not adopted into Query (widgets would render 0 with no error)`)
	}
	if _, stillThere := w.Query["orGroups"]; stillThere {
		t.Errorf(`legacy criteria key "orGroups" survived migration: %v`, w.Query)
	}

	// 2. The legacy criteria "orGroups" becomes "anyOf", with each bare
	//    branch array wrapped in an object carrying its own "filters".
	anyOf, ok := w.Query["anyOf"].([]any)
	if !ok {
		t.Fatalf(`Query has no migrated "anyOf" array: %v`, w.Query)
	}
	if len(anyOf) != 2 {
		t.Fatalf("len(anyOf) = %d, want 2", len(anyOf))
	}
	first, ok := anyOf[0].(map[string]any)
	if !ok {
		t.Fatalf("anyOf[0] = %v, want an object", anyOf[0])
	}
	firstFilters, ok := first["filters"].([]any)
	if !ok || len(firstFilters) != 2 {
		t.Fatalf(`anyOf[0]["filters"] = %v, want the branch's 2 predicates`, first["filters"])
	}
	second, _ := anyOf[1].(map[string]any)
	secondFilters, ok := second["filters"].([]any)
	if !ok || len(secondFilters) != 1 {
		t.Fatalf(`anyOf[1]["filters"] = %v, want the branch's 1 predicate`, second["filters"])
	}

	// 3. The sibling "filters" array inside the criteria object is NOT the
	//    key that moved — it keeps its name in both shapes.
	if _, ok := w.Query["filters"].([]any); !ok {
		t.Errorf(`the criteria object's own "filters" array must be untouched by the rename: %v`, w.Query)
	}

	// 4. Legacy slice-level "filters" lands in the slice's Query and keeps
	//    the literal "__current_user__" placeholder -- this package no
	//    longer resolves it, see DashboardHandler's doc comment in
	//    internal/handler/dashboards.go. The slice's own case-table
	//    resourceType also picks up the auto-injected "type" filter (see
	//    injectImpliedTypeFilters), so the array now carries 2 entries, not
	//    just the configured one.
	pie := got[0].Widgets[1]
	if pie.Query == nil {
		t.Fatalf("legacy pie widget Query is nil")
	}
	if len(pie.Slices) != 1 || pie.Slices[0].Query == nil {
		t.Fatalf("legacy slice \"filters\" was not adopted into Query: %+v", pie.Slices)
	}
	arr, ok := pie.Slices[0].Query["filters"].([]any)
	if !ok {
		t.Fatalf("legacy slice Query has no \"filters\" array: %v", pie.Slices[0].Query)
	}
	var assignedValues []any
	for _, e := range arr {
		m, _ := e.(map[string]any)
		if m["field"] == "assignedUserId" {
			assignedValues, _ = m["values"].([]any)
		}
	}
	if len(assignedValues) != 1 || assignedValues[0] != "__current_user__" {
		t.Errorf("legacy slice assignedUserId values = %v, want the unresolved placeholder [%q]", assignedValues, "__current_user__")
	}
}

// TestParseDashboardsConfig_NewKeysWinOverLegacy proves a half-migrated
// config is not corrupted: where both spellings are present the current one
// is authoritative and the deprecated one is dropped, never merged.
func TestParseDashboardsConfig_NewKeysWinOverLegacy(t *testing.T) {
	const raw = `[
		{
			"id": "mixed-dashboard",
			"displayName": "Mixed Dashboard",
			"widgets": [
				{
					"id": "mixed",
					"displayName": "Mixed",
					"resourceType": "case",
					"shape": "count",
					"gridWidth": 3,
					"filters": {"filters": [{"field": "state", "op": "in", "values": ["closed"]}]},
					"query": {
						"filters": [{"field": "state", "op": "in", "values": ["open"]}],
						"orGroups": [[{"field": "severity", "op": "in", "values": ["high"]}]],
						"anyOf": [{"filters": [{"field": "severity", "op": "in", "values": ["catastrophic"]}]}]
					}
				}
			]
		}
	]`

	got, err := ParseDashboardsConfig(raw)
	if err != nil {
		t.Fatalf("ParseDashboardsConfig returned error: %v", err)
	}
	if len(got) != 1 || len(got[0].Widgets) != 1 {
		t.Fatalf("ParseDashboardsConfig(mixed) = %+v, want 1 dashboard with 1 widget", got)
	}
	q := got[0].Widgets[0].Query

	// resourceType "case" also picks up the auto-injected "type" filter
	// (see injectImpliedTypeFilters), so the array carries that entry
	// alongside the configured "state" one -- find the "state" entry by
	// field rather than assuming it is the only one.
	arr, ok := q["filters"].([]any)
	if !ok {
		t.Fatalf(`Query["filters"] = %v`, q["filters"])
	}
	var stateValues []any
	for _, e := range arr {
		m, _ := e.(map[string]any)
		if m["field"] == "state" {
			stateValues, _ = m["values"].([]any)
		}
	}
	if len(stateValues) != 1 || stateValues[0] != "open" {
		t.Errorf(`"query" must win over the deprecated "filters": got state values %v, want ["open"]`, stateValues)
	}

	if _, stillThere := q["orGroups"]; stillThere {
		t.Errorf(`deprecated "orGroups" must be dropped when "anyOf" is present: %v`, q)
	}
	anyOf, ok := q["anyOf"].([]any)
	if !ok || len(anyOf) != 1 {
		t.Fatalf(`Query["anyOf"] = %v`, q["anyOf"])
	}
	branch, _ := anyOf[0].(map[string]any)
	branchFilters, _ := branch["filters"].([]any)
	branchEntry, _ := branchFilters[0].(map[string]any)
	branchValues, _ := branchEntry["values"].([]any)
	if len(branchValues) != 1 || branchValues[0] != "catastrophic" {
		t.Errorf(`"anyOf" must win over the deprecated "orGroups": got %v, want ["catastrophic"]`, branchValues)
	}
}

func TestParseDashboardsConfig_WidgetSection(t *testing.T) {
	const raw = `[
		{
			"id": "sample-team-dashboard",
			"displayName": "Sample Team Dashboard",
			"widgets": [
				{"id": "team-open-cases", "displayName": "Team Open Cases", "resourceType": "case", "shape": "count", "gridWidth": 6, "query": {}},
				{"id": "escalated-incidents", "displayName": "Escalated Incidents", "section": "Escalations", "resourceType": "incident", "shape": "count", "gridWidth": 6, "query": {}}
			]
		}
	]`

	got, err := ParseDashboardsConfig(raw)
	if err != nil {
		t.Fatalf("ParseDashboardsConfig returned error: %v", err)
	}
	if len(got) != 1 || len(got[0].Widgets) != 2 {
		t.Fatalf("ParseDashboardsConfig(raw) = %+v, want 1 dashboard with 2 widgets", got)
	}

	if section := got[0].Widgets[0].Section; section != "" {
		t.Errorf("team-open-cases.Section = %q, want empty (no section configured)", section)
	}
	if section := got[0].Widgets[1].Section; section != "Escalations" {
		t.Errorf("escalated-incidents.Section = %q, want %q", section, "Escalations")
	}
}

func TestParseDashboardsConfig_WidgetColumnsAndSortBy(t *testing.T) {
	const raw = `[
		{
			"id": "sample-dashboard",
			"displayName": "Sample Dashboard",
			"widgets": [
				{
					"id": "my-patches",
					"displayName": "My Patches",
					"resourceType": "case",
					"shape": "list",
					"gridWidth": 12,
					"query": {},
					"columns": [
						{"path": "subject", "label": "Subject"},
						{"path": "project.key", "label": "Project key"},
						{"path": "bestCaseFixEta", "label": "Best case ETA", "format": "date"}
					],
					"sortBy": {"field": "updatedOn", "order": "asc"}
				},
				{
					"id": "no-columns-widget",
					"displayName": "No Columns Widget",
					"resourceType": "case",
					"shape": "list",
					"gridWidth": 12,
					"query": {}
				}
			]
		}
	]`

	got, err := ParseDashboardsConfig(raw)
	if err != nil {
		t.Fatalf("ParseDashboardsConfig returned error: %v", err)
	}
	if len(got) != 1 || len(got[0].Widgets) != 2 {
		t.Fatalf("ParseDashboardsConfig(raw) = %+v, want 1 dashboard with 2 widgets", got)
	}

	w := got[0].Widgets[0]
	wantColumns := []Column{
		{Path: "subject", Label: "Subject"},
		{Path: "project.key", Label: "Project key"},
		{Path: "bestCaseFixEta", Label: "Best case ETA", Format: "date"},
	}
	if len(w.Columns) != len(wantColumns) {
		t.Fatalf("my-patches.Columns = %+v, want %+v", w.Columns, wantColumns)
	}
	for i, want := range wantColumns {
		if w.Columns[i] != want {
			t.Errorf("my-patches.Columns[%d] = %+v, want %+v", i, w.Columns[i], want)
		}
	}
	if field, _ := w.SortBy["field"].(string); field != "updatedOn" {
		t.Errorf("my-patches.SortBy[field] = %v, want %q", w.SortBy["field"], "updatedOn")
	}
	if order, _ := w.SortBy["order"].(string); order != "asc" {
		t.Errorf("my-patches.SortBy[order] = %v, want %q", w.SortBy["order"], "asc")
	}

	// A widget with neither key configured round-trips to nil for both — the
	// existing hardcoded per-resourceType frontend renderer's own no-op path
	// depends on this staying nil, not an empty slice/map.
	noColumns := got[0].Widgets[1]
	if noColumns.Columns != nil {
		t.Errorf("no-columns-widget.Columns = %+v, want nil", noColumns.Columns)
	}
	if noColumns.SortBy != nil {
		t.Errorf("no-columns-widget.SortBy = %+v, want nil", noColumns.SortBy)
	}
}
