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
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// A shared section is a named, reusable group of widgets defined once and
// pulled into several dashboards by reference, so a personal-queue section
// like "My Work" is authored and corrected in exactly one place instead of
// being copy-pasted per dashboard and drifting apart widget by widget.
//
// It is the same idea as a filter preset (see resolveDashboardFilterPresets)
// one level up: presets deduplicate a filter predicate, sections deduplicate
// a whole run of widgets. Both are pure config-authoring conveniences,
// expanded once at load time into literal widgets, so nothing downstream --
// validation, the handler, the frontend -- ever sees a reference.

// SharedSection is one entry of the shared-section file
// (DASHBOARD_SECTIONS_FILE, conventionally "_sections.json" alongside
// DASHBOARD_PRESETS_FILE -- see LoadSharedSections).
type SharedSection struct {
	// DisplayName is the section heading every widget in this section is
	// grouped under: it is written into each expanded widget's own
	// WidgetTemplate.Section, which is what the frontend actually groups on.
	// A dashboard can override it per include (see SectionInclude.DisplayName).
	DisplayName string `json:"displayName"`
	// Widgets are the section's widget templates, in render order, authored
	// exactly as they would be inline in a dashboard's own "widgets" array.
	// Each one's own "section" key, if it carries one, is overwritten by the
	// effective DisplayName -- the section owns its heading, a widget inside
	// it cannot secede.
	Widgets []WidgetTemplate `json:"widgets"`
}

// SectionInclude is one entry of a dashboard's "includeSections": a
// reference to a SharedSection plus the per-dashboard adjustments that make
// one shared definition usable by dashboards that are not identical.
type SectionInclude struct {
	// Section is the key into the shared-section set. Unknown keys fail the
	// load, same as an unknown filter preset.
	Section string `json:"section"`
	// IDPrefix is prepended to every included widget's id. Widget ids only
	// have to be unique within a dashboard, so this is optional -- it exists
	// so a dashboard that already ships prefixed ids can adopt a shared
	// section without renaming its widgets (ids are the handle for
	// click-through URLs and React keys), and so one dashboard can include
	// the same section twice under different scopes.
	IDPrefix string `json:"idPrefix,omitempty"`
	// DisplayName overrides the section's own heading for this dashboard.
	// Empty keeps SharedSection.DisplayName.
	DisplayName string `json:"displayName,omitempty"`
	// Position places the expanded widgets relative to the dashboard's own
	// "widgets" array: "start" (the default) or "end". Section order in the
	// rendered dashboard follows the order each section's value first
	// appears among the widgets, so this is what decides whether a shared
	// section leads the page or trails it. Several includes with the same
	// position expand in the order they are written.
	Position string `json:"position,omitempty"`
	// ExtraFilters are additional filter predicates ANDed into every
	// included widget's own query.filters -- the escape hatch that lets a
	// dashboard with its own permanent scope (e.g. "only projects that have
	// finished onboarding") share a section with dashboards that do not,
	// instead of forking it. Entries may be literal filter objects or
	// {"preset": "key"} references: sections expand before preset
	// resolution, so a reference here resolves exactly as it would inline.
	//
	// Only applied to widgets whose ResourceType is a caseTableResourceTypes
	// member: those are the only ones whose query is the case-search filter
	// DSL. Every other resource has its own query shape (call_request's
	// {assignedUserIds, states, excludeCaseStates}, for instance), where a
	// case predicate is not merely useless but would be forwarded to that
	// resource's /search as an unrecognised key.
	ExtraFilters []map[string]any `json:"extraFilters,omitempty"`
}

const (
	sectionPositionStart = "start"
	sectionPositionEnd   = "end"
)

// LoadSharedSections reads path (DASHBOARD_SECTIONS_FILE -- see
// cmd/server/main.go) as a JSON object mapping sectionKey -> SharedSection.
// An empty path is legal and yields an empty, nil-error map: a deployment
// with no shared sections is the normal case, and a dashboard that
// references one anyway still fails loud at expansion time with the unknown
// key named.
//
// A path that is set but unreadable, unparseable, or that defines a section
// with no widgets or no displayName, is fatal and names the file: a section
// that silently expands to nothing is a dashboard missing a whole block of
// widgets with nothing in the logs to say why.
func LoadSharedSections(path string) (map[string]SharedSection, error) {
	if strings.TrimSpace(path) == "" {
		return map[string]SharedSection{}, nil
	}
	raw, err := os.ReadFile(path) //nolint:gosec // path is deployment configuration, not user input
	if err != nil {
		return nil, fmt.Errorf("dashboard sections: read %q: %w", path, err)
	}
	var sections map[string]SharedSection
	if err := json.Unmarshal(raw, &sections); err != nil {
		return nil, fmt.Errorf("dashboard sections: parse %q: %w", path, err)
	}
	// A document of literal `null` parses fine and leaves the map nil, which
	// would then behave as "no sections configured" -- but the path WAS set, so
	// this is a malformed file, not an absent one. Left alone it fails much
	// later and in the wrong place: every includeSections reference reports
	// "unknown section", pointing whoever reads the log at the dashboards
	// rather than at the empty file that is actually wrong. An empty object is
	// a different thing and stays legal: that really is "none configured".
	if sections == nil {
		return nil, fmt.Errorf("dashboard sections: %s: document is null; use {} for no sections, or remove DASHBOARD_SECTIONS_FILE", path)
	}
	for key, s := range sections {
		// A key is looked up by its EXACT text, but expandIncludedSections
		// trims the reference before looking it up (an include of " my-work"
		// finds "my-work"). So a definition whose own key is not already
		// trimmed can never be referenced by any include at all -- it would
		// sit in the file looking configured and silently match nothing.
		// Rejected here rather than normalized: silently trimming would make
		// two keys that differ only in whitespace collide, and quietly picking
		// a winner between them is worse than refusing the file.
		if key != strings.TrimSpace(key) {
			return nil, fmt.Errorf("dashboard sections: %s: section key %q has leading or trailing whitespace; an include is trimmed before lookup, so this key could never be referenced", path, key)
		}
		if key == "" {
			return nil, fmt.Errorf("dashboard sections: %s: has an empty section key; the key is how a dashboard references the section", path)
		}
		if strings.TrimSpace(s.DisplayName) == "" {
			return nil, fmt.Errorf("dashboard sections: %s: section %q has an empty \"displayName\"; it is the heading every widget in the section is grouped under", path, key)
		}
		if len(s.Widgets) == 0 {
			return nil, fmt.Errorf("dashboard sections: %s: section %q has no widgets", path, key)
		}
	}
	return sections, nil
}

// validateSharedSections proves every section in the catalogue is usable
// BEFORE anything can read it, whether or not a dashboard includes it.
//
// Without this, a section's widgets are only ever checked as a side effect of
// being expanded into a dashboard: finalize resolves presets and validates
// widgets after expansion, so a section nothing references is never looked at.
// It is then served by GET /dashboards/sections as though it were fine, offered
// in the builder as something to include, and fails only later — at the moment
// some author includes it, blaming their dashboard for a fault that was in the
// section file all along.
//
// Validation runs against a THROWAWAY deep copy put through the real pipeline
// (resolveDashboardFilterPresets then validateWidgets) rather than a second set
// of checks written out by hand here: a parallel implementation would drift from
// the one that actually governs a loaded dashboard, and the whole point is that
// a section passes exactly the checks its widgets will face once included. The
// copy is discarded, so the catalogue keeps its authored form — unexpanded
// {"preset": ...} references and all — which is what the builder edits.
//
// presets must be every preset a section could legitimately resolve against,
// NOT just the shared ones. A dashboard may define its own "filterPresets",
// section expansion runs before dashboard-local presets are merged in, and a
// section's reference is therefore resolved against the union -- so validating
// against the shared file alone would reject a section that works perfectly
// well once included, and take the whole deploy down with it. Referencing a
// dashboard-local preset from a shared section is still a bad idea (the
// section only works for dashboards that happen to define it) but that is a
// design smell, not something to fail a load over.
//
// Sections are visited in sorted order so a file with more than one broken
// section always reports the same one first.
func validateSharedSections(sections map[string]SharedSection, presets map[string]map[string]any, path string) error {
	names := make([]string, 0, len(sections))
	for name := range sections {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		section := sections[name]
		probe := Dashboard{ID: name, Widgets: make([]WidgetTemplate, 0, len(section.Widgets))}
		for _, w := range section.Widgets {
			probe.Widgets = append(probe.Widgets, copyWidget(w))
		}
		if err := resolveDashboardFilterPresets(&probe, presets, path); err != nil {
			return fmt.Errorf("dashboard sections: %s: section %q would fail on include: %w", path, name, err)
		}
		if err := validateWidgets(probe, path); err != nil {
			return fmt.Errorf("dashboard sections: %s: section %q would fail on include: %w", path, name, err)
		}
	}
	return nil
}

// expandIncludedSections replaces d's "includeSections" references with the
// literal widgets they name, in place, and clears the field so nothing
// downstream sees a reference.
//
// It runs first in finalize's pipeline, before deprecated-key migration,
// preset resolution and implied-type-filter injection, so an included widget
// is indistinguishable from an inline one by the time any of those run: one
// code path handles both, and a section is free to use the same deprecated
// keys and {"preset": ...} references a dashboard's own widgets can.
//
// Every include gets its own deep copy of the section's widgets. Two
// dashboards including the same section must not end up sharing the decoded
// maps behind Query/Slices, since the very next pipeline steps mutate those
// in place -- preset expansion and type-filter injection would otherwise run
// twice over the same map and leak one dashboard's ExtraFilters into
// another's widgets.
func expandIncludedSections(d *Dashboard, sections map[string]SharedSection, source string) error {
	if len(d.IncludeSections) == 0 {
		return nil
	}

	var leading, trailing []WidgetTemplate
	for i, inc := range d.IncludeSections {
		ctx := fmt.Sprintf("dashboard definitions: %s (id %q): includeSections[%d]", source, d.ID, i)

		key := strings.TrimSpace(inc.Section)
		if key == "" {
			return fmt.Errorf("%s: %q is empty; it must name a section in the shared DASHBOARD_SECTIONS_FILE", ctx, "section")
		}
		section, ok := sections[key]
		if !ok {
			return fmt.Errorf("%s: references unknown section %q; define it in the shared DASHBOARD_SECTIONS_FILE", ctx, key)
		}

		position := strings.TrimSpace(inc.Position)
		if position == "" {
			position = sectionPositionStart
		}
		if position != sectionPositionStart && position != sectionPositionEnd {
			return fmt.Errorf("%s: unknown \"position\" %q; expected %q or %q", ctx, inc.Position, sectionPositionStart, sectionPositionEnd)
		}

		heading := section.DisplayName
		if strings.TrimSpace(inc.DisplayName) != "" {
			heading = inc.DisplayName
		}

		expanded := make([]WidgetTemplate, 0, len(section.Widgets))
		for _, w := range section.Widgets {
			cw := copyWidget(w)
			cw.ID = inc.IDPrefix + cw.ID
			cw.Section = heading
			applyExtraFilters(&cw, inc.ExtraFilters)
			expanded = append(expanded, cw)
		}

		if position == sectionPositionStart {
			leading = append(leading, expanded...)
		} else {
			trailing = append(trailing, expanded...)
		}
	}

	widgets := make([]WidgetTemplate, 0, len(leading)+len(d.Widgets)+len(trailing))
	widgets = append(widgets, leading...)
	widgets = append(widgets, d.Widgets...)
	widgets = append(widgets, trailing...)
	d.Widgets = widgets
	d.IncludeSections = nil
	return nil
}

// applyExtraFilters ANDs each of extra into w's own query.filters, and
// independently into every one of its slices' query.filters.
//
// The per-slice pass is the same reasoning injectImpliedTypeFilters
// documents: the frontend merges a slice's Query over the widget's Query
// whole-key, so a slice carrying its own "filters" array replaces the
// widget's entirely -- including anything added here -- unless the slice's
// own array carries it too.
//
// Non-case widgets are skipped: see SectionInclude.ExtraFilters.
func applyExtraFilters(w *WidgetTemplate, extra []map[string]any) {
	if len(extra) == 0 || !caseTableResourceTypes[w.ResourceType] {
		return
	}
	w.Query = appendFilters(w.Query, extra)
	for si := range w.Slices {
		w.Slices[si].Query = appendFilters(w.Slices[si].Query, extra)
	}
}

// appendFilters returns query (creating it if nil) with a fresh copy of each
// entry in extra appended to its top-level "filters" array. A "filters" that
// is present but not an array is left for injectTypeFilter to warn about and
// replace, exactly as it would for an inline widget -- this does not add a
// second, differently-worded warning for the same malformed key.
func appendFilters(query map[string]any, extra []map[string]any) map[string]any {
	if query == nil {
		query = map[string]any{}
	}
	filters, _ := query["filters"].([]any)
	for _, f := range extra {
		filters = append(filters, deepCopyValue(f))
	}
	query["filters"] = filters
	return query
}

// copyWidget returns a deep copy of w: the WidgetTemplate itself is copied
// by value, and every decoded-JSON structure hanging off it (Query, SortBy,
// each slice's Query) is copied deeply, since those are the maps the rest of
// the load pipeline mutates in place. Columns and Slices are copied as fresh
// slices so an append to one dashboard's copy cannot reach another's.
func copyWidget(w WidgetTemplate) WidgetTemplate {
	out := w
	out.Query = deepCopyMap(w.Query)
	out.SortBy = deepCopyMap(w.SortBy)
	if w.Columns != nil {
		out.Columns = append([]Column(nil), w.Columns...)
	}
	if w.GroupBy != nil {
		g := *w.GroupBy
		out.GroupBy = &g
	}
	if w.Slices != nil {
		out.Slices = make([]PieSlice, len(w.Slices))
		for i, s := range w.Slices {
			cs := s
			cs.Query = deepCopyMap(s.Query)
			out.Slices[i] = cs
		}
	}
	return out
}

// deepCopyMap and deepCopyValue copy a decoded-JSON value: maps and slices
// are rebuilt, everything else (string, float64, bool, nil) is immutable and
// shared. nil in, nil out -- an absent Query stays absent rather than
// becoming an empty object that later steps would have to distinguish.
func deepCopyMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = deepCopyValue(v)
	}
	return out
}

func deepCopyValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		return deepCopyMap(t)
	case []any:
		out := make([]any, len(t))
		for i, e := range t {
			out[i] = deepCopyValue(e)
		}
		return out
	default:
		return v
	}
}
