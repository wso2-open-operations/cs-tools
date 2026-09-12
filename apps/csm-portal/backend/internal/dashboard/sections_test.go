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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sharedSectionsFile writes a shared-section file into dir and returns its
// path, mirroring writeDefinition for the DASHBOARD_SECTIONS_FILE half.
func sharedSectionsFile(t *testing.T, dir, body string) string {
	t.Helper()
	return writeDefinition(t, dir, "_sections.json", body)
}

// oneCaseSection is a minimal shared section: two widgets, one of them a
// non-case resource, so extraFilters' case-only scoping is observable.
const oneCaseSection = `{
  "my-work": {
    "displayName": "My Work",
    "widgets": [
      {"id": "my_cases", "displayName": "My Cases", "resourceType": "case", "shape": "list", "gridWidth": 12,
       "query": {"filters": [{"field": "assignedUserId", "op": "in", "values": ["__current_user__"]}]}},
      {"id": "my_call_requests", "displayName": "My Call Requests", "resourceType": "call_request", "shape": "list", "gridWidth": 12,
       "query": {"assignedUserIds": ["__current_user__"]}}
    ]
  }
}`

func loadWithSections(t *testing.T, dir, sectionsBody string) []Dashboard {
	t.Helper()
	path := sharedSectionsFile(t, dir, sectionsBody)
	sections, err := LoadSharedSections(path)
	if err != nil {
		t.Fatalf("LoadSharedSections returned error: %v", err)
	}
	got, err := loadDir(dir, nil, sections)
	if err != nil {
		t.Fatalf("loadDir returned error: %v", err)
	}
	return got
}

// TestSections_ExpandsIntoLiteralWidgets is the core contract: an
// includeSections reference becomes real widgets carrying the section's
// heading, and no reference survives into the loaded Dashboard.
func TestSections_ExpandsIntoLiteralWidgets(t *testing.T) {
	dir := t.TempDir()
	writeDefinition(t, dir, "d.json", `{
	  "id": "d", "displayName": "D", "type": "cs",
	  "includeSections": [{"section": "my-work"}],
	  "widgets": [
	    {"id": "own", "displayName": "Own", "resourceType": "case", "shape": "count", "gridWidth": 3, "section": "Team"}
	  ]
	}`)

	d := loadWithSections(t, dir, oneCaseSection)[0]

	if d.IncludeSections != nil {
		t.Errorf("Dashboard.IncludeSections = %+v, want nil (cleared once expanded, never visible past load)", d.IncludeSections)
	}
	gotIDs := make([]string, 0, len(d.Widgets))
	for _, w := range d.Widgets {
		gotIDs = append(gotIDs, w.ID)
	}
	want := "my_cases,my_call_requests,own"
	if strings.Join(gotIDs, ",") != want {
		t.Errorf("widget ids = %v, want [%s] (position defaults to start)", gotIDs, want)
	}
	for _, w := range d.Widgets[:2] {
		if w.Section != "My Work" {
			t.Errorf("widget %q section = %q, want %q", w.ID, w.Section, "My Work")
		}
	}
	if d.Widgets[2].Section != "Team" {
		t.Errorf("the dashboard's own widget section = %q, want %q (untouched)", d.Widgets[2].Section, "Team")
	}
}

// TestSections_PositionEndTrailsOwnWidgets covers the other placement, which
// is what decides whether the shared section leads or trails the page.
func TestSections_PositionEndTrailsOwnWidgets(t *testing.T) {
	dir := t.TempDir()
	writeDefinition(t, dir, "d.json", `{
	  "id": "d", "displayName": "D", "type": "cs",
	  "includeSections": [{"section": "my-work", "position": "end"}],
	  "widgets": [
	    {"id": "own", "displayName": "Own", "resourceType": "case", "shape": "count", "gridWidth": 3}
	  ]
	}`)

	d := loadWithSections(t, dir, oneCaseSection)[0]
	if d.Widgets[0].ID != "own" || d.Widgets[len(d.Widgets)-1].ID != "my_call_requests" {
		t.Errorf("widget order = %q..%q, want own first and the section last", d.Widgets[0].ID, d.Widgets[len(d.Widgets)-1].ID)
	}
}

// TestSections_IDPrefixAndDisplayNameOverride covers the two per-dashboard
// adjustments that let a dashboard adopt a shared section without renaming
// its widgets or accepting the shared heading.
func TestSections_IDPrefixAndDisplayNameOverride(t *testing.T) {
	dir := t.TempDir()
	writeDefinition(t, dir, "d.json", `{
	  "id": "d", "displayName": "D", "type": "cs",
	  "includeSections": [{"section": "my-work", "idPrefix": "ob_", "displayName": "My Queue"}],
	  "widgets": []
	}`)

	d := loadWithSections(t, dir, oneCaseSection)[0]
	if d.Widgets[0].ID != "ob_my_cases" {
		t.Errorf("widget id = %q, want %q", d.Widgets[0].ID, "ob_my_cases")
	}
	if d.Widgets[0].Section != "My Queue" {
		t.Errorf("widget section = %q, want the per-include override %q", d.Widgets[0].Section, "My Queue")
	}
}

// TestSections_ExtraFiltersAreCaseOnly is the reason extraFilters exists (a
// dashboard with its own permanent scope can still share the section) and
// the reason it is gated (a case predicate on a call_request query would be
// forwarded to /call-requests/search as an unrecognised key).
func TestSections_ExtraFiltersAreCaseOnly(t *testing.T) {
	dir := t.TempDir()
	writeDefinition(t, dir, "d.json", `{
	  "id": "d", "displayName": "D", "type": "cs",
	  "includeSections": [{"section": "my-work", "extraFilters": [
	    {"field": "projectOnboardingStatus", "op": "in", "values": ["Completed"]}
	  ]}],
	  "widgets": []
	}`)

	d := loadWithSections(t, dir, oneCaseSection)[0]

	caseFilters, ok := d.Widgets[0].Query["filters"].([]any)
	if !ok {
		t.Fatalf("case widget Query has no \"filters\" array: %v", d.Widgets[0].Query)
	}
	values, ok := filterField(caseFilters, "projectOnboardingStatus")
	if !ok {
		t.Fatalf("case widget did not receive the extra filter: %v", caseFilters)
	}
	if len(values) != 1 || values[0] != "Completed" {
		t.Errorf("extra filter values = %v, want [Completed]", values)
	}

	if raw, present := d.Widgets[1].Query["filters"]; present {
		t.Errorf("call_request widget Query gained a \"filters\" key %v; extraFilters must only touch case-search widgets", raw)
	}
}

// TestSections_ExtraFilterPresetReferencesResolve pins the ordering
// guarantee: sections expand before preset resolution, so a {"preset": ...}
// reference written in extraFilters is expanded like any inline one rather
// than reaching the frontend as a literal object with a "preset" key.
func TestSections_ExtraFilterPresetReferencesResolve(t *testing.T) {
	dir := t.TempDir()
	writeDefinition(t, dir, "d.json", `{
	  "id": "d", "displayName": "D", "type": "cs",
	  "includeSections": [{"section": "my-work", "extraFilters": [{"preset": "onboarded"}]}],
	  "widgets": []
	}`)
	sections, err := LoadSharedSections(sharedSectionsFile(t, dir, oneCaseSection))
	if err != nil {
		t.Fatalf("LoadSharedSections returned error: %v", err)
	}
	presets := map[string]map[string]any{
		"onboarded": {"field": "projectOnboardingStatus", "op": "in", "values": []any{"Completed"}},
	}

	got, err := loadDir(dir, presets, sections)
	if err != nil {
		t.Fatalf("loadDir returned error: %v", err)
	}
	filters, _ := got[0].Widgets[0].Query["filters"].([]any)
	if _, ok := filterField(filters, "projectOnboardingStatus"); !ok {
		t.Fatalf("preset reference in extraFilters did not expand: %v", filters)
	}
	for _, entry := range filters {
		if m, ok := entry.(map[string]any); ok && isPresetRef(m) {
			t.Errorf("an unresolved preset reference survived into the widget query: %v", m)
		}
	}
}

// TestSections_TwoDashboardsDoNotShareState is the deep-copy guarantee. The
// pipeline mutates Query maps in place (preset expansion, type injection),
// so a shallow copy would let one dashboard's extraFilters appear in the
// other's widgets — silently, and only for whichever loaded second.
func TestSections_TwoDashboardsDoNotShareState(t *testing.T) {
	dir := t.TempDir()
	writeDefinition(t, dir, "a.json", `{
	  "id": "a", "displayName": "A", "type": "cs",
	  "includeSections": [{"section": "my-work", "extraFilters": [
	    {"field": "projectOnboardingStatus", "op": "in", "values": ["Completed"]}
	  ]}],
	  "widgets": []
	}`)
	writeDefinition(t, dir, "b.json", `{
	  "id": "b", "displayName": "B", "type": "cre", "isTeamBased": true,
	  "includeSections": [{"section": "my-work"}],
	  "widgets": []
	}`)

	got := loadWithSections(t, dir, oneCaseSection)
	byID := map[string]Dashboard{}
	for _, d := range got {
		byID[d.ID] = d
	}
	bFilters, _ := byID["b"].Widgets[0].Query["filters"].([]any)
	if _, leaked := filterField(bFilters, "projectOnboardingStatus"); leaked {
		t.Errorf("dashboard b's widget carries dashboard a's extraFilters: %v", bFilters)
	}
	aFilters, _ := byID["a"].Widgets[0].Query["filters"].([]any)
	if _, ok := filterField(aFilters, "projectOnboardingStatus"); !ok {
		t.Errorf("dashboard a lost its own extraFilters: %v", aFilters)
	}
}

// TestSections_UnknownSectionFailsLoad — a section that silently expanded to
// nothing would be a dashboard missing a whole block of widgets with nothing
// in the logs, the same failure class the loader already refuses for an
// unknown preset.
func TestSections_UnknownSectionFailsLoad(t *testing.T) {
	dir := t.TempDir()
	writeDefinition(t, dir, "d.json", `{
	  "id": "d", "displayName": "D", "type": "cs",
	  "includeSections": [{"section": "nope"}],
	  "widgets": []
	}`)
	sections, err := LoadSharedSections(sharedSectionsFile(t, dir, oneCaseSection))
	if err != nil {
		t.Fatalf("LoadSharedSections returned error: %v", err)
	}

	_, err = loadDir(dir, nil, sections)
	if err == nil {
		t.Fatal("loadDir returned no error for an unknown section reference")
	}
	if !strings.Contains(err.Error(), "nope") || !strings.Contains(err.Error(), "d.json") {
		t.Errorf("error %q does not name both the unknown key and the file to edit", err)
	}
}

// TestSections_BadPositionFailsLoad — an unrecognised position would
// silently fall back to one of the two placements and reorder the page.
func TestSections_BadPositionFailsLoad(t *testing.T) {
	dir := t.TempDir()
	writeDefinition(t, dir, "d.json", `{
	  "id": "d", "displayName": "D", "type": "cs",
	  "includeSections": [{"section": "my-work", "position": "middle"}],
	  "widgets": []
	}`)
	sections, err := LoadSharedSections(sharedSectionsFile(t, dir, oneCaseSection))
	if err != nil {
		t.Fatalf("LoadSharedSections returned error: %v", err)
	}

	if _, err := loadDir(dir, nil, sections); err == nil {
		t.Fatal("loadDir returned no error for an unknown position")
	}
}

// TestSections_DuplicateWidgetIDsAcrossIncludesFailLoad — including one
// section twice without distinct idPrefixes collides on widget id, which the
// existing per-dashboard uniqueness check must still catch after expansion.
func TestSections_DuplicateWidgetIDsAcrossIncludesFailLoad(t *testing.T) {
	dir := t.TempDir()
	writeDefinition(t, dir, "d.json", `{
	  "id": "d", "displayName": "D", "type": "cs",
	  "includeSections": [{"section": "my-work"}, {"section": "my-work"}],
	  "widgets": []
	}`)
	sections, err := LoadSharedSections(sharedSectionsFile(t, dir, oneCaseSection))
	if err != nil {
		t.Fatalf("LoadSharedSections returned error: %v", err)
	}

	_, err = loadDir(dir, nil, sections)
	if err == nil {
		t.Fatal("loadDir returned no error for a section included twice with no idPrefix")
	}
	if !strings.Contains(err.Error(), "duplicate widget id") {
		t.Errorf("error %q does not name the duplicate widget id", err)
	}
}

// TestSections_IncludedWidgetsGetTheImpliedTypeFilter proves an included
// widget is treated exactly like an inline one by the rest of the pipeline —
// the whole reason expansion runs first.
func TestSections_IncludedWidgetsGetTheImpliedTypeFilter(t *testing.T) {
	dir := t.TempDir()
	writeDefinition(t, dir, "d.json", `{
	  "id": "d", "displayName": "D", "type": "cs",
	  "includeSections": [{"section": "engagements"}],
	  "widgets": []
	}`)

	d := loadWithSections(t, dir, `{
	  "engagements": {
	    "displayName": "My Work",
	    "widgets": [
	      {"id": "my_engagements", "displayName": "My Engagements", "resourceType": "engagement",
	       "shape": "list", "gridWidth": 12, "query": {"filters": []}}
	    ]
	  }
	}`)[0]

	filters, _ := d.Widgets[0].Query["filters"].([]any)
	values, ok := filterField(filters, "type")
	if !ok {
		t.Fatalf("included widget did not get the auto-injected type filter: %v", filters)
	}
	if len(values) != 1 || values[0] != "engagement" {
		t.Errorf("injected type values = %v, want [engagement]", values)
	}
}

// TestLoadSharedSections_EmptyPathIsLegal — a deployment with no shared
// sections is the normal case and must still boot.
func TestLoadSharedSections_EmptyPathIsLegal(t *testing.T) {
	got, err := LoadSharedSections("")
	if err != nil {
		t.Fatalf("LoadSharedSections(\"\") returned error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("LoadSharedSections(\"\") = %v, want empty", got)
	}
}

// TestLoadSharedSections_RejectsEmptySection — a section with no widgets, or
// no heading, expands to a hole in the page rather than an error.
func TestLoadSharedSections_RejectsEmptySection(t *testing.T) {
	for name, body := range map[string]string{
		"no widgets":     `{"my-work": {"displayName": "My Work", "widgets": []}}`,
		"no displayName": `{"my-work": {"widgets": [{"id": "w", "displayName": "W", "resourceType": "case", "shape": "count", "gridWidth": 3}]}}`,
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := sharedSectionsFile(t, dir, body)
			if _, err := LoadSharedSections(path); err == nil {
				t.Fatalf("LoadSharedSections returned no error for a section with %s", name)
			}
		})
	}
}

// TestLoadSharedSections_UnreadableFileIsFatal — a configured-but-missing
// file must not degrade to "no sections", which would take every including
// dashboard's section down with it.
func TestLoadSharedSections_UnreadableFileIsFatal(t *testing.T) {
	if _, err := LoadSharedSections(filepath.Join(t.TempDir(), "absent.json")); err == nil {
		t.Fatal("LoadSharedSections returned no error for a missing file")
	}
}

// TestLoadSharedSectionsRejectsUnreferenceableKeys covers the two ways a
// sections file can look configured while being unusable: a null document, and
// a key that no include could ever match.
func TestLoadSharedSectionsRejectsUnreferenceableKeys(t *testing.T) {
	write := func(t *testing.T, body string) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), "_sections.json")
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("writing sections file: %v", err)
		}
		return path
	}

	const oneGoodSection = `{"my-work":{"displayName":"My Work","widgets":[` +
		`{"id":"w","displayName":"W","resourceType":"case","shape":"count","gridWidth":3,` +
		`"query":{"filters":[{"field":"state","op":"in","values":["open"]}]}}]}}`

	t.Run("a null document is rejected, not treated as no sections", func(t *testing.T) {
		// The path WAS configured, so null is a malformed file. Treating it as
		// "none configured" defers the failure to every includeSections
		// reference, each blaming the dashboard instead of this file.
		_, err := LoadSharedSections(write(t, `null`))
		if err == nil {
			t.Fatal("expected an error for a null sections document")
		}
		if !strings.Contains(err.Error(), "null") {
			t.Errorf("error should name the problem: %v", err)
		}
	})

	t.Run("an empty object stays legal", func(t *testing.T) {
		got, err := LoadSharedSections(write(t, `{}`))
		if err != nil {
			t.Fatalf("an empty object is 'none configured', not an error: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("got %d sections, want 0", len(got))
		}
	})

	t.Run("a whitespace-padded key is rejected", func(t *testing.T) {
		// expandIncludedSections trims the include before lookup, so this key
		// is unreachable by any reference.
		_, err := LoadSharedSections(write(t, `{" my-work":{"displayName":"X","widgets":[]}}`))
		if err == nil {
			t.Fatal("expected an error for a whitespace-padded section key")
		}
		if !strings.Contains(err.Error(), "whitespace") {
			t.Errorf("error should name the problem: %v", err)
		}
	})

	t.Run("an empty key is rejected", func(t *testing.T) {
		_, err := LoadSharedSections(write(t, `{"":{"displayName":"X","widgets":[]}}`))
		if err == nil {
			t.Fatal("expected an error for an empty section key")
		}
	})

	t.Run("a canonical key still loads", func(t *testing.T) {
		got, err := LoadSharedSections(write(t, oneGoodSection))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("got %d sections, want 1", len(got))
		}
	})
}

// TestValidateSharedSectionsRejectsUnreferencedBreakage proves a broken section
// fails the load even when no dashboard includes it — the case that previously
// slipped through to the catalogue and only surfaced when an author included it.
func TestValidateSharedSectionsRejectsUnreferencedBreakage(t *testing.T) {
	dir := t.TempDir()
	presetsPath := filepath.Join(dir, "_presets.json")
	sectionsPath := filepath.Join(dir, "_sections.json")
	// One valid dashboard that references NOTHING, so the only way a section
	// gets looked at is the catalogue-level validation under test.
	dashPath := filepath.Join(dir, "d.json")

	writeFile := func(t *testing.T, path, body string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("writing %s: %v", path, err)
		}
	}
	writeFile(t, presetsPath, `{"activeCaseStates":{"field":"state","op":"in","values":["open"]}}`)
	writeFile(t, dashPath, `{"id":"d","displayName":"D","type":"cs","isDefault":true,"widgets":[`+
		`{"id":"w","displayName":"W","resourceType":"case","shape":"count","gridWidth":3,`+
		`"query":{"filters":[{"field":"state","op":"in","values":["open"]}]}}]}`)

	section := func(widget string) string {
		return `{"orphan":{"displayName":"Orphan","widgets":[` + widget + `]}}`
	}

	t.Run("an unknown preset reference in an unincluded section is fatal", func(t *testing.T) {
		writeFile(t, sectionsPath, section(
			`{"id":"w1","displayName":"W1","resourceType":"case","shape":"count","gridWidth":3,`+
				`"query":{"filters":[{"preset":"noSuchPreset"}]}}`))
		_, err := NewDirRegistry(dir, false, presetsPath, sectionsPath)
		if err == nil {
			t.Fatal("expected the load to fail on the unreferenced section's bad preset")
		}
		if !strings.Contains(err.Error(), "orphan") {
			t.Errorf("error should name the offending section: %v", err)
		}
	})

	t.Run("a structurally invalid widget in an unincluded section is fatal", func(t *testing.T) {
		// gridWidth 99 is out of the 1-12 column range.
		writeFile(t, sectionsPath, section(
			`{"id":"w1","displayName":"W1","resourceType":"case","shape":"count","gridWidth":99,`+
				`"query":{"filters":[{"field":"state","op":"in","values":["open"]}]}}`))
		_, err := NewDirRegistry(dir, false, presetsPath, sectionsPath)
		if err == nil {
			t.Fatal("expected the load to fail on the unreferenced section's bad widget")
		}
		if !strings.Contains(err.Error(), "orphan") {
			t.Errorf("error should name the offending section: %v", err)
		}
	})

	t.Run("a duplicate widget id within one section is fatal", func(t *testing.T) {
		writeFile(t, sectionsPath, section(
			`{"id":"dup","displayName":"A","resourceType":"case","shape":"count","gridWidth":3,`+
				`"query":{"filters":[{"field":"state","op":"in","values":["open"]}]}},`+
				`{"id":"dup","displayName":"B","resourceType":"case","shape":"count","gridWidth":3,`+
				`"query":{"filters":[{"field":"state","op":"in","values":["open"]}]}}`))
		if _, err := NewDirRegistry(dir, false, presetsPath, sectionsPath); err == nil {
			t.Fatal("expected the load to fail on duplicate widget ids inside a section")
		}
	})

	t.Run("a valid unincluded section loads, and keeps its preset reference authored", func(t *testing.T) {
		writeFile(t, sectionsPath, section(
			`{"id":"w1","displayName":"W1","resourceType":"case","shape":"count","gridWidth":3,`+
				`"query":{"filters":[{"preset":"activeCaseStates"}]}}`))
		reg, err := NewDirRegistry(dir, false, presetsPath, sectionsPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// The validation probe must not have mutated the catalogue: the
		// builder edits the authored form, so the reference has to survive.
		got := reg.SharedSections()["orphan"]
		filters, ok := got.Widgets[0].Query["filters"].([]any)
		if !ok || len(filters) != 1 {
			t.Fatalf("query.filters missing: %#v", got.Widgets[0].Query)
		}
		entry, ok := filters[0].(map[string]any)
		if !ok || entry["preset"] != "activeCaseStates" {
			t.Errorf("validation expanded the catalogue's preset reference: %#v", filters[0])
		}
	})
}

// TestSharedSectionMayReferenceDashboardLocalPreset proves catalogue validation
// resolves a section's preset reference against the UNION of the shared file
// and every dashboard's own "filterPresets" — the set the reference actually
// resolves against once the section is expanded. Validating against the shared
// file alone rejected a section that works, and failed the whole load.
func TestSharedSectionMayReferenceDashboardLocalPreset(t *testing.T) {
	dir := t.TempDir()
	presetsPath := filepath.Join(dir, "_presets.json")
	sectionsPath := filepath.Join(dir, "_sections.json")
	write := func(t *testing.T, path, body string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("writing %s: %v", path, err)
		}
	}
	sectionWidget := func(presetName string) string {
		return `{"orphan":{"displayName":"Orphan","widgets":[` +
			`{"id":"w","displayName":"W","resourceType":"case","shape":"count","gridWidth":3,` +
			`"query":{"filters":[{"preset":"` + presetName + `"}]}}]}}`
	}

	// The shared file is deliberately EMPTY of the preset the section wants;
	// only the dashboard defines it, in its own filterPresets.
	write(t, presetsPath, `{}`)
	write(t, filepath.Join(dir, "d.json"), `{"id":"d","displayName":"D","type":"cs","isDefault":true,`+
		`"filterPresets":{"localOnly":{"field":"state","op":"in","values":["open"]}},`+
		`"widgets":[{"id":"dw","displayName":"DW","resourceType":"case","shape":"count","gridWidth":3,`+
		`"query":{"filters":[{"preset":"localOnly"}]}}]}`)

	t.Run("a dashboard-local preset satisfies a section reference", func(t *testing.T) {
		write(t, sectionsPath, sectionWidget("localOnly"))
		if _, err := NewDirRegistry(dir, false, presetsPath, sectionsPath); err != nil {
			t.Fatalf("a section referencing a dashboard-local preset must load, got: %v", err)
		}
	})

	t.Run("a name defined nowhere is still fatal", func(t *testing.T) {
		write(t, sectionsPath, sectionWidget("definedNowhere"))
		_, err := NewDirRegistry(dir, false, presetsPath, sectionsPath)
		if err == nil {
			t.Fatal("expected the load to fail on a preset defined in no file at all")
		}
		if !strings.Contains(err.Error(), "orphan") {
			t.Errorf("error should still name the offending section: %v", err)
		}
	})
}
