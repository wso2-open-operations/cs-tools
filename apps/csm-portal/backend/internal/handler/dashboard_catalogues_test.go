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
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/dashboard"
)

// The two builder-support catalogue endpoints. These are the only dashboard
// endpoints that serve the AUTHORED form of a definition rather than the
// resolved one, so what they must not do is expand anything: the builder
// edits preset references, it does not want them resolved away.

const cataloguePresetsJSON = `{
  "activeCaseStates": {"field": "state", "op": "in", "values": ["open", "work_in_progress"]},
  "productionImpact":  {"field": "severity", "op": "in", "values": ["catastrophic"]}
}`

const catalogueSectionsJSON = `{
  "my-work": {
    "displayName": "My Work",
    "widgets": [
      {
        "id": "my_open",
        "displayName": "My Open Cases",
        "resourceType": "case",
        "shape": "list",
        "gridWidth": 12,
        "query": {"filters": [{"preset": "activeCaseStates"}]}
      }
    ]
  }
}`

const catalogueDashboardJSON = `{
  "id": "cat-dash",
  "displayName": "Catalogue Dashboard",
  "type": "cs",
  "isDefault": true,
  "widgets": [
    {
      "id": "w1",
      "displayName": "W1",
      "resourceType": "case",
      "shape": "count",
      "gridWidth": 3,
      "query": {"filters": [{"preset": "activeCaseStates"}]}
    }
  ]
}`

// seedCatalogueRegistry installs a directory-backed registry carrying both
// shared files, and restores whatever TestMain installed afterwards so the
// rest of the package still sees its own static registry.
func seedCatalogueRegistry(t *testing.T, hotReload bool) string {
	t.Helper()
	dir := t.TempDir()
	presets := filepath.Join(dir, "_presets.json")
	sections := filepath.Join(dir, "_sections.json")
	write := func(path, body string) {
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("writing %s: %v", path, err)
		}
	}
	write(presets, cataloguePresetsJSON)
	write(sections, catalogueSectionsJSON)
	write(filepath.Join(dir, "cat-dash.json"), catalogueDashboardJSON)

	reg, err := dashboard.NewDirRegistry(dir, hotReload, presets, sections)
	if err != nil {
		t.Fatalf("NewDirRegistry: %v", err)
	}
	previous := dashboard.Active()
	dashboard.SetActive(reg)
	t.Cleanup(func() { dashboard.SetActive(previous) })
	return dir
}

func TestGetFilterPresets(t *testing.T) {
	h := NewDashboardHandler()

	t.Run("unauthenticated", func(t *testing.T) {
		w := httptest.NewRecorder()
		h.GetFilterPresets(w, httptest.NewRequest(http.MethodGet, "/dashboards/filter-presets", nil))
		assertStatus(t, w, http.StatusUnauthorized)
	})

	t.Run("lists every preset, sorted by name", func(t *testing.T) {
		seedCatalogueRegistry(t, false)
		w := httptest.NewRecorder()
		h.GetFilterPresets(w, withUser(httptest.NewRequest(http.MethodGet, "/dashboards/filter-presets", nil)))
		assertStatus(t, w, http.StatusOK)

		var got []filterPresetView
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("got %d presets, want 2: %s", len(got), w.Body.String())
		}
		// Sorted, so the order is asserted rather than searched for.
		if got[0].Name != "activeCaseStates" || got[1].Name != "productionImpact" {
			t.Fatalf("presets not sorted by name: %q, %q", got[0].Name, got[1].Name)
		}
		if got[0].Filter["field"] != "state" || got[0].Filter["op"] != "in" {
			t.Errorf("preset filter body not forwarded verbatim: %#v", got[0].Filter)
		}
	})

	t.Run("no presets configured is an empty array, not null and not an error", func(t *testing.T) {
		previous := dashboard.Active()
		dashboard.SetActive(dashboard.NewStaticRegistry(nil))
		t.Cleanup(func() { dashboard.SetActive(previous) })

		w := httptest.NewRecorder()
		h.GetFilterPresets(w, withUser(httptest.NewRequest(http.MethodGet, "/dashboards/filter-presets", nil)))
		assertStatus(t, w, http.StatusOK)
		// A `null` body would make the frontend's `.map` throw; an empty
		// array is what "this deployment has no presets" must look like.
		if body := w.Body.String(); body != "[]\n" && body != "[]" {
			t.Errorf("body = %q, want an empty JSON array", body)
		}
	})

	t.Run("hot reload picks up an edited presets file", func(t *testing.T) {
		dir := seedCatalogueRegistry(t, true)
		edited := `{"onlyOne": {"field": "state", "op": "in", "values": ["closed"]}}`
		if err := os.WriteFile(filepath.Join(dir, "_presets.json"), []byte(edited), 0o600); err != nil {
			t.Fatalf("rewriting presets: %v", err)
		}

		w := httptest.NewRecorder()
		h.GetFilterPresets(w, withUser(httptest.NewRequest(http.MethodGet, "/dashboards/filter-presets", nil)))
		assertStatus(t, w, http.StatusOK)

		var got []filterPresetView
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if len(got) != 1 || got[0].Name != "onlyOne" {
			t.Errorf("hot reload did not pick up the edit: %s", w.Body.String())
		}
	})
}

func TestGetSharedSections(t *testing.T) {
	h := NewDashboardHandler()

	t.Run("unauthenticated", func(t *testing.T) {
		w := httptest.NewRecorder()
		h.GetSharedSections(w, httptest.NewRequest(http.MethodGet, "/dashboards/sections", nil))
		assertStatus(t, w, http.StatusUnauthorized)
	})

	t.Run("lists sections with their widgets in the dashboard widget shape", func(t *testing.T) {
		seedCatalogueRegistry(t, false)
		w := httptest.NewRecorder()
		h.GetSharedSections(w, withUser(httptest.NewRequest(http.MethodGet, "/dashboards/sections", nil)))
		assertStatus(t, w, http.StatusOK)

		var got []sharedSectionView
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("got %d sections, want 1: %s", len(got), w.Body.String())
		}
		if got[0].Name != "my-work" || got[0].DisplayName != "My Work" {
			t.Errorf("name/displayName = %q/%q", got[0].Name, got[0].DisplayName)
		}
		if len(got[0].Widgets) != 1 || got[0].Widgets[0].WidgetID != "my_open" {
			t.Fatalf("widgets not forwarded: %#v", got[0].Widgets)
		}
	})

	t.Run("a section's preset references are NOT expanded", func(t *testing.T) {
		seedCatalogueRegistry(t, false)
		w := httptest.NewRecorder()
		h.GetSharedSections(w, withUser(httptest.NewRequest(http.MethodGet, "/dashboards/sections", nil)))
		assertStatus(t, w, http.StatusOK)

		var got []sharedSectionView
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		filters, ok := got[0].Widgets[0].Query["filters"].([]any)
		if !ok || len(filters) != 1 {
			t.Fatalf("query.filters missing or wrong length: %#v", got[0].Widgets[0].Query)
		}
		entry, ok := filters[0].(map[string]any)
		if !ok {
			t.Fatalf("filter entry is not an object: %#v", filters[0])
		}
		// The builder edits the authored form. If this ever starts
		// returning {field,op,values} instead, the builder silently loses
		// the ability to show (or keep) the preset reference.
		if entry["preset"] != "activeCaseStates" {
			t.Errorf("preset reference was expanded away: %#v", entry)
		}
	})

	t.Run("the dashboard endpoint still expands, unlike the section endpoint", func(t *testing.T) {
		seedCatalogueRegistry(t, false)
		w := httptest.NewRecorder()
		h.GetDashboardDetail(w, withUser(withDashboardID(
			httptest.NewRequest(http.MethodGet, "/dashboards/cat-dash", nil), "cat-dash")))
		assertStatus(t, w, http.StatusOK)

		var got dashboardDetailView
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		filters, ok := got.Widgets[0].Query["filters"].([]any)
		if !ok || len(filters) == 0 {
			t.Fatalf("query.filters missing: %#v", got.Widgets[0].Query)
		}
		entry := filters[0].(map[string]any)
		if entry["preset"] != nil {
			t.Errorf("a served dashboard must have no preset references left: %#v", entry)
		}
		if entry["field"] != "state" {
			t.Errorf("preset was not expanded into its filter: %#v", entry)
		}
	})

	t.Run("no sections configured is an empty array", func(t *testing.T) {
		previous := dashboard.Active()
		dashboard.SetActive(dashboard.NewStaticRegistry(nil))
		t.Cleanup(func() { dashboard.SetActive(previous) })

		w := httptest.NewRecorder()
		h.GetSharedSections(w, withUser(httptest.NewRequest(http.MethodGet, "/dashboards/sections", nil)))
		assertStatus(t, w, http.StatusOK)
		if body := w.Body.String(); body != "[]\n" && body != "[]" {
			t.Errorf("body = %q, want an empty JSON array", body)
		}
	})
}
