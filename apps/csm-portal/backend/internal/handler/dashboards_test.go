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
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"sort"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/dashboard"
)

// testDashboardsConfigJSON is a small, entirely dummy 2-dashboard fixture —
// generic names/ids only, no real WSO2 CS-team structure — extended slightly
// beyond the DASHBOARDS_CONFIG example documented in .env.example (which has
// the same two dashboards) with two additional widgets (an "incident"-typed
// widget with empty filters and a "change_request"-typed widget) purely to
// keep this file's resourceType-diversity and scalar-filter test coverage,
// since the trimmed .env.example example doesn't need those shapes to make
// its point. dashboard.All() is populated from DASHBOARDS_CONFIG only in
// cmd/server/main.go, which tests never run, so TestMain below seeds it
// directly via the same parse function production uses — every assertion in
// this file exercises the real production parsing/lookup/resolution path,
// just with the config supplied in-process instead of via the environment.
const testDashboardsConfigJSON = `[
  {"id":"sample-dashboard","displayName":"Sample Dashboard","isDefault":true,"targetTeam":"sample-team","widgets":[
    {"id":"my-open-cases","displayName":"My Open Cases","resourceType":"case","shape":"count","gridWidth":3,"query":{"filters":[{"field":"assignedUserId","op":"in","values":["__current_user__"]},{"field":"state","op":"in","values":["open","work_in_progress"]}]}},
    {"id":"recent-cases","displayName":"Recent Cases","resourceType":"case","shape":"list","gridWidth":6,"listLimit":5,"query":{"filters":[{"field":"tag","op":"in","values":["example-tag"]},{"field":"tag","op":"notIn","values":["excluded-example-tag"]}]},"columns":[{"path":"subject","label":"Subject"},{"path":"project.key","label":"Project key"}],"sortBy":{"field":"updatedOn","order":"asc"}},
    {"id":"pending-time-cards","displayName":"Pending Time Cards","resourceType":"time_card","shape":"count","gridWidth":3,"query":{"states":["pending"]}},
    {"id":"open-vulnerabilities","displayName":"Open Vulnerabilities","resourceType":"product_vulnerability","shape":"count","gridWidth":3,"query":{"priority":"high"}}
  ]},
  {"id":"sample-team-dashboard","displayName":"Sample Team Dashboard","targetTeam":"sample-team","isTeamBased":true,"widgets":[
    {"id":"team-open-cases","displayName":"Team Open Cases","section":"Overview","resourceType":"case","shape":"count","gridWidth":4,"query":{"filters":[{"field":"severity","op":"in","values":["critical","high"]},{"field":"state","op":"in","values":["open","work_in_progress"]}]}},
    {"id":"unassigned-cases","displayName":"Unassigned Cases","section":"Overview","resourceType":"case","shape":"count","gridWidth":4,"query":{"filters":[{"field":"assignedUserId","op":"isEmpty"},{"field":"state","op":"in","values":["open"]}]}},
    {"id":"cases-by-severity","displayName":"Cases by Severity","description":"Share of active cases at each severity level.","resourceType":"case","shape":"pie","gridWidth":4,"query":{"filters":[{"field":"state","op":"in","values":["open","work_in_progress"]}]},"slices":[
      {"label":"Critical","color":"error","query":{"filters":[{"field":"severity","op":"in","values":["critical"]}]}},
      {"label":"Mine","query":{"filters":[{"field":"assignedUserId","op":"in","values":["__current_user__"]}]}}
    ]},
    {"id":"escalated-incidents","displayName":"Escalated Incidents","section":"Escalations","resourceType":"incident","shape":"count","gridWidth":4,"query":{}},
    {"id":"pending-change-requests","displayName":"Pending Change Requests","resourceType":"change_request","shape":"count","gridWidth":4,"query":{"states":["customer_approval"]}}
  ]}
]`

// filterValuesByField finds the "values" array of the first entry in a case
// widget's resolved filters (the {"filters":[{"field","op","values"}, ...]}
// shape, see .env.example's DASHBOARDS_CONFIG) whose "field" matches, and
// reports whether one was found.
func filterValuesByField(filters map[string]any, field string) ([]any, bool) {
	arr, ok := filters["filters"].([]any)
	if !ok {
		return nil, false
	}
	for _, entry := range arr {
		m, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		if m["field"] != field {
			continue
		}
		values, ok := m["values"].([]any)
		return values, ok
	}
	return nil, false
}

// TestMain installs the active dashboard registry before any test in this
// package runs. In production it is installed once at process startup by
// cmd/server/main.go, from DASHBOARDS_DIR or the deprecated DASHBOARDS_CONFIG;
// tests never invoke main(), so they seed a static registry through the same
// ParseDashboardsConfig decoder.
func TestMain(m *testing.M) {
	dashboards, err := dashboard.ParseDashboardsConfig(testDashboardsConfigJSON)
	if err != nil {
		panic(fmt.Sprintf("TestMain: seeding the dashboard registry failed: %v", err))
	}
	if len(dashboards) != 2 {
		panic(fmt.Sprintf("TestMain: seeding the dashboard registry failed, got %d dashboards, want 2", len(dashboards)))
	}
	dashboard.SetActive(dashboard.NewStaticRegistry(dashboards))
	os.Exit(m.Run())
}

// dashboardWidgetJSONKeys are the top-level JSON keys openapi.yaml's
// DashboardWidget schema declares. Kept in sync with that schema by hand;
// the tests below fail if the handler's actual response keys ever diverge
// from this set, catching an unannounced field rename/add/remove that a
// struct-only decode (which silently ignores unknown keys and zero-values
// missing ones) would miss.
//
// groupBy and listLimit are omitempty on the wire and are not included here;
// widgets that set them are checked individually where relevant.
var dashboardWidgetJSONKeys = []string{"widgetId", "displayName", "resourceType", "shape", "gridWidth", "query"}

// dashboardListItemJSONKeys are the top-level JSON keys openapi.yaml's
// DashboardListItem schema declares.
var dashboardListItemJSONKeys = []string{"id", "displayName", "isDefault", "isTeamBased"}

// dashboardDetailJSONKeys are the top-level JSON keys openapi.yaml's
// Dashboard schema declares.
var dashboardDetailJSONKeys = []string{"id", "displayName", "isDefault", "targetTeam", "isTeamBased", "widgets"}

func assertJSONKeys(t *testing.T, obj map[string]json.RawMessage, want []string, context string) {
	t.Helper()
	wantKeys := append([]string(nil), want...)
	sort.Strings(wantKeys)
	gotKeys := make([]string, 0, len(obj))
	for k := range obj {
		gotKeys = append(gotKeys, k)
	}
	sort.Strings(gotKeys)
	if !reflect.DeepEqual(gotKeys, wantKeys) {
		t.Errorf("%s JSON keys = %v, want %v", context, gotKeys, wantKeys)
	}
}

// assertJSONKeysSuperset is like assertJSONKeys but only requires every key in
// want to be present; used for widgets that additionally carry an omitempty
// field (groupBy/listLimit) beyond the base set.
func assertJSONKeysSuperset(t *testing.T, obj map[string]json.RawMessage, want []string, context string) {
	t.Helper()
	for _, k := range want {
		if _, ok := obj[k]; !ok {
			t.Errorf("%s missing expected key %q; got keys %v", context, k, keysOf(obj))
		}
	}
}

func keysOf(obj map[string]json.RawMessage) []string {
	out := make([]string, 0, len(obj))
	for k := range obj {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func withDashboardID(r *http.Request, dashboardID string) *http.Request {
	r.SetPathValue("dashboardId", dashboardID)
	return r
}

// currentUserPlaceholder is the literal string a case widget's
// assignedUserId filter carries for "the signed-in user's own cases" — see
// testDashboardsConfigJSON's "my-open-cases"/"Mine" entries. The handler no
// longer resolves it (see DashboardHandler's doc comment in dashboards.go):
// it is expected to reach the response exactly as configured, for the
// frontend to substitute client-side.
const currentUserPlaceholder = "__current_user__"

func TestGetDashboards(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewDashboardHandler()
		r := httptest.NewRequest(http.MethodGet, "/dashboards", nil)
		w := httptest.NewRecorder()
		h.GetDashboards(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("returns all dashboards in registry order with correct isDefault", func(t *testing.T) {
		h := NewDashboardHandler()
		r := withUser(httptest.NewRequest(http.MethodGet, "/dashboards", nil))
		w := httptest.NewRecorder()
		h.GetDashboards(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")

		body := w.Body.Bytes()

		var results []dashboardListItemView
		if err := json.Unmarshal(body, &results); err != nil {
			t.Fatalf("decode response body: %v; raw: %s", err, body)
		}
		if len(results) != len(dashboard.All()) {
			t.Fatalf("len(results) = %d, want %d", len(results), len(dashboard.All()))
		}

		var raw []map[string]json.RawMessage
		if err := json.Unmarshal(body, &raw); err != nil {
			t.Fatalf("decode response body as raw keys: %v; raw: %s", err, body)
		}
		for i, obj := range raw {
			assertJSONKeys(t, obj, dashboardListItemJSONKeys, fmt.Sprintf("result[%d]", i))
		}

		for i, want := range dashboard.All() {
			got := results[i]
			if got.ID != want.ID {
				t.Errorf("result[%d].ID = %q, want %q (registry order must be preserved)", i, got.ID, want.ID)
			}
			if got.DisplayName != want.DisplayName {
				t.Errorf("result[%d].DisplayName = %q, want %q", i, got.DisplayName, want.DisplayName)
			}
			if got.IsDefault != want.IsDefault {
				t.Errorf("result[%d].IsDefault = %v, want %v", i, got.IsDefault, want.IsDefault)
			}
			if got.IsTeamBased != want.IsTeamBased {
				t.Errorf("result[%d].IsTeamBased = %v, want %v", i, got.IsTeamBased, want.IsTeamBased)
			}
		}

		wantTeamBased := map[string]bool{"sample-team-dashboard": true}
		teamBasedCount := 0
		for _, res := range results {
			if res.IsTeamBased {
				teamBasedCount++
				if !wantTeamBased[res.ID] {
					t.Errorf("unexpected team-based dashboard %q, want sample-team-dashboard", res.ID)
				}
			}
		}
		if teamBasedCount != len(wantTeamBased) {
			t.Errorf("teamBasedCount = %d, want exactly %d (sample-team-dashboard)", teamBasedCount, len(wantTeamBased))
		}

		defaultCount := 0
		for _, res := range results {
			if res.IsDefault {
				defaultCount++
				if res.ID != "sample-dashboard" {
					t.Errorf("unexpected default dashboard %q, want sample-dashboard", res.ID)
				}
			}
		}
		if defaultCount != 1 {
			t.Errorf("default dashboard count = %d, want 1", defaultCount)
		}
	})
}

// TestAllDashboardsHaveWidgets is the "no more mock/empty placeholders"
// guarantee: every dashboard in the registry now has real widgets.
func TestAllDashboardsHaveWidgets(t *testing.T) {
	if len(dashboard.All()) != 2 {
		t.Fatalf("len(dashboard.All()) = %d, want 2", len(dashboard.All()))
	}
	for _, d := range dashboard.All() {
		if len(d.Widgets) == 0 {
			t.Errorf("dashboard %q has no widgets, want at least 1", d.ID)
		}
	}
}

func TestGetDashboardDetail(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewDashboardHandler()
		r := withDashboardID(httptest.NewRequest(http.MethodGet, "/dashboards/sample-dashboard", nil), "sample-dashboard")
		w := httptest.NewRecorder()
		h.GetDashboardDetail(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("unknown dashboard id returns 404", func(t *testing.T) {
		h := NewDashboardHandler()
		r := withUser(withDashboardID(httptest.NewRequest(http.MethodGet, "/dashboards/bogus", nil), "bogus"))
		w := httptest.NewRecorder()
		h.GetDashboardDetail(w, r)
		assertStatus(t, w, http.StatusNotFound)
		assertErrorMessage(t, w, ErrMsgNotFound)
	})

	t.Run("sample-dashboard returns metadata and its four widgets", func(t *testing.T) {
		h := NewDashboardHandler()
		r := withUser(withDashboardID(httptest.NewRequest(http.MethodGet, "/dashboards/sample-dashboard", nil), "sample-dashboard"))
		w := httptest.NewRecorder()
		h.GetDashboardDetail(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")

		body := w.Body.Bytes()
		t.Logf("GET /dashboards/sample-dashboard response: %s", body)

		// Decode into the real production type (dashboardDetailView, defined
		// in dashboards.go), not a duplicate ad hoc struct — a JSON tag
		// rename on the real type breaks this decode/assertions directly,
		// instead of silently zero-valuing a field in a copy that has
		// already drifted from what's actually returned.
		var result dashboardDetailView
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("decode response body: %v; raw: %s", err, body)
		}

		if result.ID != "sample-dashboard" {
			t.Errorf("ID = %q, want %q", result.ID, "sample-dashboard")
		}
		if result.DisplayName != "Sample Dashboard" {
			t.Errorf("DisplayName = %q, want %q", result.DisplayName, "Sample Dashboard")
		}
		if !result.IsDefault {
			t.Errorf("IsDefault = %v, want true", result.IsDefault)
		}
		if result.TargetTeam != "sample-team" {
			t.Errorf("TargetTeam = %q, want %q", result.TargetTeam, "sample-team")
		}
		if len(result.Widgets) != 4 {
			t.Fatalf("len(result.Widgets) = %d, want 4", len(result.Widgets))
		}

		// Confirm the actual top-level JSON keys match openapi.yaml's
		// declared Dashboard schema exactly.
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(body, &raw); err != nil {
			t.Fatalf("decode response body as raw keys: %v; raw: %s", err, body)
		}
		assertJSONKeys(t, raw, dashboardDetailJSONKeys, "response")

		// Confirm each widget's JSON keys match openapi.yaml's declared
		// DashboardWidget schema exactly (allowing the omitempty
		// groupBy/listLimit extras) — catches an added/removed field that
		// the struct decode above wouldn't (json.Unmarshal ignores unknown
		// keys and zero-values missing ones).
		var rawWidgets []map[string]json.RawMessage
		if err := json.Unmarshal(raw["widgets"], &rawWidgets); err != nil {
			t.Fatalf("decode widgets as raw keys: %v; raw: %s", err, raw["widgets"])
		}
		for i, obj := range rawWidgets {
			assertJSONKeysSuperset(t, obj, dashboardWidgetJSONKeys, fmt.Sprintf("widgets[%d]", i))
		}

		byID := make(map[string]int)
		for i, res := range result.Widgets {
			byID[res.WidgetID] = i
			if res.DisplayName == "" {
				t.Errorf("widget %s has empty displayName", res.WidgetID)
			}
		}

		wantResourceShape := map[string]struct {
			resourceType dashboard.ResourceType
			shape        dashboard.Shape
			gridWidth    int
		}{
			"my-open-cases":        {dashboard.ResourceCase, dashboard.ShapeCount, 3},
			"recent-cases":         {dashboard.ResourceCase, dashboard.ShapeList, 6},
			"pending-time-cards":   {dashboard.ResourceTimeCard, dashboard.ShapeCount, 3},
			"open-vulnerabilities": {dashboard.ResourceProductVulnerability, dashboard.ShapeCount, 3},
		}
		for id, want := range wantResourceShape {
			idx, ok := byID[id]
			if !ok {
				t.Fatalf("missing widget %q in response", id)
			}
			got := result.Widgets[idx]
			if got.ResourceType != want.resourceType {
				t.Errorf("widget %s resourceType = %q, want %q", id, got.ResourceType, want.resourceType)
			}
			if got.Shape != want.shape {
				t.Errorf("widget %s shape = %q, want %q", id, got.Shape, want.shape)
			}
			if got.GridWidth != want.gridWidth {
				t.Errorf("widget %s gridWidth = %d, want %d", id, got.GridWidth, want.gridWidth)
			}
		}

		if idx := byID["recent-cases"]; result.Widgets[idx].ListLimit != 5 {
			t.Errorf("widget recent-cases listLimit = %d, want 5", result.Widgets[idx].ListLimit)
		}

		// my-open-cases carries an assignedUserId filter (the current user's
		// open cases) — the handler no longer resolves "__current_user__"
		// (that moved to the frontend), so it must reach the response
		// exactly as configured.
		openIdx, ok := byID["my-open-cases"]
		if !ok {
			t.Fatalf("missing widget %q in response", "my-open-cases")
		}
		assigned, present := filterValuesByField(result.Widgets[openIdx].Query, "assignedUserId")
		if !present {
			t.Fatalf("widget my-open-cases filters has no assignedUserId field entry")
		}
		if len(assigned) != 1 || assigned[0] != currentUserPlaceholder {
			t.Errorf("widget my-open-cases assignedUserId values = %v, want the unresolved placeholder [%q]", assigned, currentUserPlaceholder)
		}

		// recent-cases has no assignedUserId filter entry in its template and
		// must not gain one during substitution: substituteCurrentUser only
		// rewrites values already present, it never adds entries.
		recentIdx, ok := byID["recent-cases"]
		if !ok {
			t.Fatalf("missing widget %q in response", "recent-cases")
		}
		recentFilters := result.Widgets[recentIdx].Query
		if v, present := filterValuesByField(recentFilters, "assignedUserId"); present {
			t.Errorf("widget recent-cases filters unexpectedly has an assignedUserId field entry: %v", v)
		}

		// recent-cases' columns/sortBy round-trip onto the wire verbatim —
		// the BE never resolves a column's path or interprets sortBy, both
		// are forwarded exactly as configured (see dashboard.Column doc
		// comment and WidgetTemplate.SortBy).
		recentColumns := result.Widgets[recentIdx].Columns
		wantColumns := []dashboard.Column{
			{Path: "subject", Label: "Subject"},
			{Path: "project.key", Label: "Project key"},
		}
		if len(recentColumns) != len(wantColumns) {
			t.Fatalf("widget recent-cases columns = %+v, want %+v", recentColumns, wantColumns)
		}
		for i, want := range wantColumns {
			if recentColumns[i] != want {
				t.Errorf("widget recent-cases columns[%d] = %+v, want %+v", i, recentColumns[i], want)
			}
		}
		recentSortBy := result.Widgets[recentIdx].SortBy
		if field, _ := recentSortBy["field"].(string); field != "updatedOn" {
			t.Errorf("widget recent-cases sortBy[field] = %v, want %q", recentSortBy["field"], "updatedOn")
		}
		if order, _ := recentSortBy["order"].(string); order != "asc" {
			t.Errorf("widget recent-cases sortBy[order] = %v, want %q", recentSortBy["order"], "asc")
		}

		// my-open-cases sets neither columns nor sortBy in the template
		// (testDashboardsConfigJSON above) — both must stay absent from the
		// wire (omitempty), not render as an empty array/object, so a widget
		// that never opted in is byte-for-byte unaffected by this feature.
		rawOpenCases := rawWidgets[openIdx]
		if _, present := rawOpenCases["columns"]; present {
			t.Errorf("widget my-open-cases unexpectedly has a columns key on the wire: %s", rawOpenCases["columns"])
		}
		if _, present := rawOpenCases["sortBy"]; present {
			t.Errorf("widget my-open-cases unexpectedly has a sortBy key on the wire: %s", rawOpenCases["sortBy"])
		}
	})

	t.Run("sample-team-dashboard has resource-type-diverse widgets (case, incident, change_request)", func(t *testing.T) {
		h := NewDashboardHandler()
		r := withUser(withDashboardID(httptest.NewRequest(http.MethodGet, "/dashboards/sample-team-dashboard", nil), "sample-team-dashboard"))
		w := httptest.NewRecorder()
		h.GetDashboardDetail(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")

		body := w.Body.Bytes()

		var result dashboardDetailView
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("decode response body: %v; raw: %s", err, body)
		}
		if result.ID != "sample-team-dashboard" {
			t.Errorf("ID = %q, want %q", result.ID, "sample-team-dashboard")
		}
		if result.TargetTeam != "sample-team" {
			t.Errorf("TargetTeam = %q, want %q", result.TargetTeam, "sample-team")
		}
		if len(result.Widgets) != 5 {
			t.Fatalf("len(result.Widgets) = %d, want 5", len(result.Widgets))
		}

		byID := make(map[string]dashboardWidgetView)
		for _, wd := range result.Widgets {
			byID[wd.WidgetID] = wd
		}

		wantTypes := map[string]dashboard.ResourceType{
			"team-open-cases":         dashboard.ResourceCase,
			"escalated-incidents":     dashboard.ResourceIncident,
			"pending-change-requests": dashboard.ResourceChangeRequest,
		}
		for id, wantType := range wantTypes {
			got, ok := byID[id]
			if !ok {
				t.Fatalf("missing widget %q in response", id)
			}
			if got.ResourceType != wantType {
				t.Errorf("widget %s resourceType = %q, want %q", id, got.ResourceType, wantType)
			}
		}

		changeRequest, ok := byID["pending-change-requests"]
		if !ok {
			t.Fatalf("missing widget %q in response", "pending-change-requests")
		}
		statesRaw, present := changeRequest.Query["states"]
		if !present {
			t.Fatalf("pending-change-requests query has no states key: %v", changeRequest.Query)
		}
		states, ok := statesRaw.([]any)
		if !ok || len(states) != 1 || states[0] != "customer_approval" {
			t.Errorf("pending-change-requests filters.states = %v, want [customer_approval] unmodified", statesRaw)
		}
	})

	t.Run("sample-dashboard's product_vulnerability widget has a scalar string filter", func(t *testing.T) {
		h := NewDashboardHandler()
		r := withUser(withDashboardID(httptest.NewRequest(http.MethodGet, "/dashboards/sample-dashboard", nil), "sample-dashboard"))
		w := httptest.NewRecorder()
		h.GetDashboardDetail(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")

		body := w.Body.Bytes()
		t.Logf("GET /dashboards/sample-dashboard response: %s", body)

		var result dashboardDetailView
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("decode response body: %v; raw: %s", err, body)
		}
		if len(result.Widgets) != 4 {
			t.Fatalf("len(result.Widgets) = %d, want 4", len(result.Widgets))
		}

		byID := make(map[string]dashboardWidgetView)
		for _, wd := range result.Widgets {
			byID[wd.WidgetID] = wd
		}

		vulns, ok := byID["open-vulnerabilities"]
		if !ok {
			t.Fatalf("missing widget %q in response", "open-vulnerabilities")
		}
		if vulns.ResourceType != dashboard.ResourceProductVulnerability {
			t.Errorf("open-vulnerabilities resourceType = %q, want %q", vulns.ResourceType, dashboard.ResourceProductVulnerability)
		}
		priority, present := vulns.Query["priority"]
		if !present {
			t.Fatalf("open-vulnerabilities query has no priority key: %v", vulns.Query)
		}
		if s, ok := priority.(string); !ok || s != "high" {
			t.Errorf("open-vulnerabilities filters.priority = %v (%T), want string %q", priority, priority, "high")
		}
	})

	t.Run("sample-team-dashboard's pie widget resolves description, slices, and per-slice current-user placeholders", func(t *testing.T) {
		h := NewDashboardHandler()
		r := withUser(withDashboardID(httptest.NewRequest(http.MethodGet, "/dashboards/sample-team-dashboard", nil), "sample-team-dashboard"))
		w := httptest.NewRecorder()
		h.GetDashboardDetail(w, r)

		assertStatus(t, w, http.StatusOK)
		body := w.Body.Bytes()
		t.Logf("GET /dashboards/sample-team-dashboard response: %s", body)

		var result dashboardDetailView
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("decode response body: %v; raw: %s", err, body)
		}

		byID := make(map[string]dashboardWidgetView)
		for _, wd := range result.Widgets {
			byID[wd.WidgetID] = wd
		}
		pie, ok := byID["cases-by-severity"]
		if !ok {
			t.Fatalf("missing widget %q in response", "cases-by-severity")
		}
		if pie.Description != "Share of active cases at each severity level." {
			t.Errorf("cases-by-severity Description = %q, want the configured subtitle", pie.Description)
		}
		if len(pie.Slices) != 2 {
			t.Fatalf("len(cases-by-severity.Slices) = %d, want 2", len(pie.Slices))
		}

		var critical, mine *dashboardPieSliceView
		for i := range pie.Slices {
			switch pie.Slices[i].Label {
			case "Critical":
				critical = &pie.Slices[i]
			case "Mine":
				mine = &pie.Slices[i]
			}
		}
		if critical == nil {
			t.Fatalf("missing the %q slice in cases-by-severity.Slices", "Critical")
		}
		if critical.Color != "error" {
			t.Errorf("Critical slice Color = %q, want %q", critical.Color, "error")
		}
		if _, present := filterValuesByField(critical.Query, "state"); present {
			t.Errorf("Critical slice Query must not carry the widget's own base filters, got %v", critical.Query)
		}

		if mine == nil {
			t.Fatalf("missing the %q slice in cases-by-severity.Slices", "Mine")
		}
		assigned, ok := filterValuesByField(mine.Query, "assignedUserId")
		if !ok || len(assigned) != 1 || assigned[0] != currentUserPlaceholder {
			t.Errorf("Mine slice assignedUserId = %v, want the unresolved placeholder [%q]", assigned, currentUserPlaceholder)
		}

		// Confirm the wire keys match the updated openapi.yaml schema.
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(body, &raw); err != nil {
			t.Fatalf("decode response body as raw keys: %v; raw: %s", err, body)
		}
		var rawWidgets []map[string]json.RawMessage
		if err := json.Unmarshal(raw["widgets"], &rawWidgets); err != nil {
			t.Fatalf("decode widgets as raw keys: %v; raw: %s", err, raw["widgets"])
		}
		var rawPie map[string]json.RawMessage
		for _, obj := range rawWidgets {
			var id string
			if err := json.Unmarshal(obj["widgetId"], &id); err == nil && id == "cases-by-severity" {
				rawPie = obj
			}
		}
		if rawPie == nil {
			t.Fatalf("cases-by-severity not found among raw widgets: %v", rawWidgets)
		}
		assertJSONKeysSuperset(t, rawPie, append(append([]string(nil), dashboardWidgetJSONKeys...), "description", "slices"), "cases-by-severity")
	})

	t.Run("sample-team-dashboard's escalated-incidents widget carries its configured section, unset for widgets with no section", func(t *testing.T) {
		h := NewDashboardHandler()
		r := withUser(withDashboardID(httptest.NewRequest(http.MethodGet, "/dashboards/sample-team-dashboard", nil), "sample-team-dashboard"))
		w := httptest.NewRecorder()
		h.GetDashboardDetail(w, r)

		assertStatus(t, w, http.StatusOK)
		var result dashboardDetailView
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatalf("decode response body: %v; raw: %s", err, w.Body.Bytes())
		}
		byID := make(map[string]dashboardWidgetView)
		for _, wd := range result.Widgets {
			byID[wd.WidgetID] = wd
		}

		escalated, ok := byID["escalated-incidents"]
		if !ok {
			t.Fatalf("missing widget %q in response", "escalated-incidents")
		}
		if escalated.Section != "Escalations" {
			t.Errorf("escalated-incidents.Section = %q, want %q", escalated.Section, "Escalations")
		}

		casesBySeverity, ok := byID["cases-by-severity"]
		if !ok {
			t.Fatalf("missing widget %q in response", "cases-by-severity")
		}
		if casesBySeverity.Section != "" {
			t.Errorf("cases-by-severity.Section = %q, want empty (no section configured)", casesBySeverity.Section)
		}
	})

	t.Run("every dashboard in the registry now has at least one widget", func(t *testing.T) {
		h := NewDashboardHandler()
		for _, d := range dashboard.All() {
			r := withUser(withDashboardID(httptest.NewRequest(http.MethodGet, "/dashboards/"+d.ID, nil), d.ID))
			w := httptest.NewRecorder()
			h.GetDashboardDetail(w, r)
			assertStatus(t, w, http.StatusOK)

			var result dashboardDetailView
			if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
				t.Fatalf("dashboard %s: decode response body: %v; raw: %s", d.ID, err, w.Body.Bytes())
			}
			if len(result.Widgets) == 0 {
				t.Errorf("dashboard %s has 0 widgets in the response, want > 0", d.ID)
			}
		}
	})
}

// Resolving "__current_user__" into a concrete platform user id used to be
// this handler's job (via an entity-service GET /users/me round trip) and
// had its own regression-guard tests here (a JWT-claim vs entity-resolved-id
// mix-up). That responsibility, and the entity dependency it required, moved
// to the frontend — see DashboardHandler's doc comment in dashboards.go and
// the "my-open-cases"/"Mine" assertions above in TestGetDashboardDetail,
// which now assert the placeholder reaches the response unresolved instead.
