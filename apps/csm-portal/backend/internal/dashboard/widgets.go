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

// Package dashboard holds the pilot's config-driven dashboard widget
// templates. Each widget resolves to a search against that ResourceType's own
// /search endpoint (every resource's search payload shape is
// {filters: {...}, pagination: {...}}) — there is no generic filter DSL and
// no database backing this; the registry itself is loaded at process startup
// from a directory of per-dashboard JSON files (DASHBOARDS_DIR — see LoadDir
// and Registry in registry.go), or from the deprecated DASHBOARDS_CONFIG
// environment variable when no directory is set (see ParseDashboardsConfig).
// Both are wired up in cmd/server/main.go.
package dashboard

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
)

// ResourceType identifies which resource a widget's filters search against.
type ResourceType string

const (
	ResourceCase                 ResourceType = "case"
	ResourceIncident             ResourceType = "incident"
	ResourceChangeRequest        ResourceType = "change_request"
	ResourceAccount              ResourceType = "account"
	ResourceProject              ResourceType = "project"
	ResourceUser                 ResourceType = "user"
	ResourceTimeCard             ResourceType = "time_card"
	ResourceProblem              ResourceType = "problem"
	ResourceIncidentTask         ResourceType = "incident_task"
	ResourceProductVulnerability ResourceType = "product_vulnerability"
	ResourceCallRequest          ResourceType = "call_request"
	// ResourceServiceRequest, ResourceSecurityReportAnalysis,
	// ResourceAnnouncement and ResourceEngagement are additional values of
	// the case-search "type" field (see apps/csm-portal/backend/openapi.yaml,
	// the case Type enum) exposed as their own widget resourceType, alongside
	// ResourceCase itself. All five route to the same /cases/search endpoint
	// -- the frontend maps each resourceType to that endpoint independently
	// (nothing here derives the endpoint from the string) -- and all five get
	// an implicit "type" filter auto-injected at load time (see
	// caseTableResourceTypes and injectImpliedTypeFilters) so a dashboard
	// author never has to paste {"field":"type","op":"in","values":[...]}
	// into every widget by hand.
	ResourceServiceRequest         ResourceType = "service_request"
	ResourceSecurityReportAnalysis ResourceType = "security_report_analysis"
	ResourceAnnouncement           ResourceType = "announcement"
	ResourceEngagement             ResourceType = "engagement"
	// ResourceCaseFeedback identifies a widget whose data is case-feedback
	// (satisfaction rating) records rather than cases themselves. Like every
	// other ResourceType, this layer only validates that the value is a known
	// enum member -- it does not know, and does not need to know, that a
	// "bar"+GroupBy.Bucket widget with this resourceType resolves via the
	// webapp's own hook calling POST /cases/feedback/aggregate (see
	// GroupByConfig.Bucket's doc comment). There is no
	// resourceType->endpoint mapping anywhere server-side; the frontend picks
	// the endpoint from resourceType entirely on its own, same as every other
	// ResourceType here.
	ResourceCaseFeedback ResourceType = "case_feedback"
)

// Shape is how a widget's resolved data should be rendered.
type Shape string

const (
	ShapeCount Shape = "count" // single resolved number
	ShapeList  Shape = "list"  // top-N matching records
	ShapePie   Shape = "pie"   // one search per Slices entry (or one server-side aggregation via GroupBy), each resolved via its own total — see PieSlice and GroupByConfig
	ShapeBar   Shape = "bar"   // same resolution as ShapePie; differs only in how the frontend renders the resolved data
)

// GroupByConfig configures a Shape "pie"/"bar" widget to resolve via a single
// server-side aggregation call instead of one search per literal Slices
// entry -- the alternative for a resourceType/field with too many distinct
// values to enumerate by hand (e.g. "cases grouped by account, top 12 +
// Others"). A widget carries exactly one of Slices or GroupBy for pie/bar
// (see the registry's own load-time validation); GroupBy is resolved
// entirely client-side, same as Slices -- the frontend calls that
// ResourceType's own new POST /{resourceType}/group-by endpoint once and
// maps the response's groups/othersCount into the same per-slice shape
// Slices already produces, so the renderer itself needs no per-shape branch.
type GroupByConfig struct {
	// Field is which field to group by. Each ResourceType enforces its own
	// small allowlist server-side (see the corresponding SN Utils script
	// include's group{X}By method) -- an unsupported value is rejected by
	// that call with a 400, not caught here. Mutually exclusive with Bucket
	// (enforced at directory-load time): a widget sets exactly one.
	Field string `json:"field,omitempty"`
	// Bucket configures date-bucketed grouping instead of field-value
	// grouping -- "day", "week", "month" (temporal buckets, a real time
	// range in ascending chronological order), or one of the categorical
	// buckets "rating", "reasons_very_dissatisfied", "reasons_dissatisfied",
	// "reasons_neutral", "reasons_satisfied", "reasons_very_satisfied" (no
	// ordering guarantee) -- enforced at directory-load time. Mutually
	// exclusive with Field. This is the shape
	// a case-feedback trend/distribution widget uses (ResourceCaseFeedback,
	// Shape "bar" or "pie"): the frontend's own hook calls POST
	// /cases/feedback/aggregate with this bucket value and maps its bucketed
	// response into the same per-slice shape Slices/field-grouping already
	// produce, same client-side-resolution philosophy as the rest of this
	// type -- this layer never calls that endpoint itself, it only
	// validates and forwards the config.
	Bucket string `json:"bucket,omitempty"`
	// MaxGroups is the top-N cutoff by count, descending; the remainder is
	// summed into one "Others" bucket. Omitted/zero defers to the backing
	// group-by endpoint's own default (12). Meaningless when Bucket is set --
	// a date-bucketed aggregation has no "Others" folding.
	MaxGroups int `json:"maxGroups,omitempty"`
	// OthersLabel overrides the default "Others" label for the summed
	// remainder bucket. Omitted uses "Others". Meaningless when Bucket is
	// set, same as MaxGroups.
	OthersLabel string `json:"othersLabel,omitempty"`
}

// PieSlice is one wedge of a Shape "pie" widget. The caller resolves its
// value by issuing that widget's own ResourceType's /search with Query
// merged under the widget's own base Query (this slice's keys win on
// conflict) and pagination.limit=1, reading total off the response — the
// exact same mechanism Shape "count" uses, just once per slice.
type PieSlice struct {
	Label string `json:"label"`
	// Color is a palette key ("primary", "secondary", "success", "error",
	// "info", "warning") the frontend already uses elsewhere in this system
	// (see WidgetTemplate's own icon color convention on the frontend) — not
	// validated here, forwarded verbatim. Falls back to a fixed rotation over
	// the same palette on the frontend if omitted.
	Color string `json:"color,omitempty"`
	// Query is this slice's own search criteria (see WidgetTemplate.Query).
	Query map[string]any `json:"query"`

	// legacyFilters holds a pre-rename config's "filters" key, moved into
	// Query by migrateLegacyWidgetKeys. See WidgetTemplate.legacyFilters.
	legacyFilters map[string]any
}

// UnmarshalJSON decodes a PieSlice, accepting both the current "query" key
// and the deprecated "filters" key it replaced (see
// WidgetTemplate.UnmarshalJSON for why).
func (s *PieSlice) UnmarshalJSON(data []byte) error {
	type alias PieSlice
	var raw struct {
		alias
		LegacyFilters map[string]any `json:"filters"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*s = PieSlice(raw.alias)
	s.legacyFilters = raw.LegacyFilters
	return nil
}

// Column is one column of a Shape "list" widget's generic column renderer
// (see WidgetTemplate.Columns). It is opaque config, like Query/SortBy: the
// BE never resolves Path or interprets Format, it only forwards this struct
// to the frontend as part of the widget's own view.
type Column struct {
	// Path addresses a field on each item of that ResourceType's own
	// /search response, dot-separated to reach into nested objects/refs to
	// arbitrary depth (e.g. "project.key", "project.account.tier") — every
	// resource's search response embeds related entities as nested JSON
	// objects, not flat records. Not validated here: the frontend resolves
	// it against whatever the response actually returns, and a path that
	// resolves to nothing renders that cell empty rather than erroring the
	// whole widget.
	Path string `json:"path"`
	// Label is this column's header text.
	Label string `json:"label"`
	// Format is a rendering hint for the resolved value. "" (equivalently
	// "text", the default) renders plain text; "date" formats a date/
	// date-time string the same way the frontend's existing hardcoded list
	// renderers already format one. Not validated here — the frontend falls
	// back to plain text for a value it doesn't recognize.
	Format string `json:"format,omitempty"`
}

// WidgetTemplate is resource-agnostic: Query is opaque JSON, forwarded
// verbatim as the filters object of that ResourceType's own /search payload
// (every resource's search payload shape is {filters: {...}, pagination:
// {...}}). Any "__current_user__"/"__current_team__" placeholder string a
// filter value carries is left exactly as authored -- the BE does not
// resolve it, the frontend does, client-side (see
// apps/csm-portal/webapp/src/features/csm-dashboard/utils, e.g.
// teamFilterPlaceholder.ts for the pattern). The two things the BE does
// interpret, both once at directory-load time rather than per-request, are:
// migrating deprecated key names (see migrateLegacyWidgetKeys), and
// expanding {"preset": "key"} filter references and auto-injecting the
// implied "type" filter for case-table resourceTypes (see
// resolveDashboardFilterPresets and injectImpliedTypeFilters). Past load
// time, Query is exactly what it looks like: a literal filters object with
// nothing left to resolve on this side.
type WidgetTemplate struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	// Description is an explanatory subtitle shown under DisplayName —
	// config-owned text, not hardcoded per ResourceType/Shape on the
	// frontend.
	Description  string         `json:"description,omitempty"`
	ResourceType ResourceType   `json:"resourceType"`
	Shape        Shape          `json:"shape"`
	GridWidth    int            `json:"gridWidth"` // 1-12, CSS grid columns out of 12
	Query        map[string]any `json:"query"`
	ListLimit    int            `json:"listLimit,omitempty"` // only meaningful for Shape list; how many records to show
	// Slices and GroupBy are the two mutually exclusive ways to configure a
	// Shape pie/bar widget's data -- see GroupByConfig's doc comment. Exactly
	// one must be set for pie/bar (enforced at directory-load time); both
	// are meaningless for count/list.
	Slices  []PieSlice     `json:"slices,omitempty"`
	GroupBy *GroupByConfig `json:"groupBy,omitempty"`
	// Section groups widgets sharing the same (non-empty) value under a
	// titled sub-section within the dashboard, in the order that value
	// first appears among the dashboard's widgets — e.g. a handful of
	// "count" widgets all set to Section: "SLA Violation" render together
	// under that heading, separately from the dashboard's other widgets.
	// Widgets with no Section (the common case) render in one untitled
	// group, exactly as before this field existed.
	Section string `json:"section,omitempty"`
	// Columns is only meaningful for Shape list: an ordered set of columns
	// the frontend should render instead of that ResourceType's own
	// hardcoded list renderer. Each entry's Path is resolved against every
	// item in the resolved search response (see Column). Absent/empty
	// Columns is a no-op — the frontend falls back to its existing
	// per-ResourceType renderer exactly as before this field existed.
	Columns []Column `json:"columns,omitempty"`
	// SortBy is only meaningful for Shape list: opaque JSON, forwarded
	// verbatim as the sortBy object of that ResourceType's own /search
	// payload — same passthrough philosophy as Query/filters. The BE does
	// not validate or interpret it; an unsupported field name for that
	// ResourceType is a caller (frontend/config-author) mistake, surfaced by
	// that resource's own /search endpoint, not caught here.
	SortBy map[string]any `json:"sortBy,omitempty"`

	// legacyFilters holds a pre-rename config's "filters" key so
	// migrateLegacyWidgetKeys can move it into Query. Unexported so it can
	// never be re-emitted on the wire: the deprecated shape is accepted on
	// input only.
	legacyFilters map[string]any
}

// UnmarshalJSON decodes a WidgetTemplate, accepting both the current "query"
// key and the deprecated "filters" key it replaced. Deployed environments
// carry DASHBOARDS_CONFIG in an env var, so a rename here is not atomic with
// a config rollout — and encoding/json silently leaves an unknown key's field
// at its zero value, which would give every widget a nil Query and render 0
// everywhere with no error at all. See migrateLegacyWidgetKeys, which does
// the actual move plus the deprecation warning (it has the dashboard/widget
// ids this method does not).
func (w *WidgetTemplate) UnmarshalJSON(data []byte) error {
	type alias WidgetTemplate
	var raw struct {
		alias
		LegacyFilters map[string]any `json:"filters"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*w = WidgetTemplate(raw.alias)
	w.legacyFilters = raw.LegacyFilters
	return nil
}

// Dashboard is a single dashboard's metadata plus its widget templates.
type Dashboard struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	// Type classifies the dashboard's audience (see Type). It drives the
	// frontend's automatic dashboard selection from the caller's team
	// family. It deliberately does not replace IsDefault or IsTeamBased --
	// all three coexist -- so the three can be set to states that
	// contradict each other; validate rejects those at load time.
	Type      Type `json:"type,omitempty"`
	IsDefault bool `json:"isDefault"`
	// TargetTeam is purely descriptive metadata (e.g. for a future FE team
	// picker); it is not enforced anywhere. GET /dashboards still returns
	// every dashboard to every caller regardless of team membership.
	TargetTeam string `json:"targetTeam"`
	// IsTeamBased marks a dashboard whose FE view should offer a team
	// selector (populated from POST /teams/search) alongside the dashboard
	// switcher. This is currently UI skeleton only: selecting a team does
	// not yet scope any widget's data. Wiring a selected team into widget
	// filters (e.g. resolving its member user IDs into a case widget's
	// assignedUserIds) is deliberately deferred to a later increment.
	IsTeamBased bool             `json:"isTeamBased"`
	Widgets     []WidgetTemplate `json:"widgets"`

	// IncludeSections pulls in shared, reusable runs of widgets by name --
	// a section like "My Work" authored once in DASHBOARD_SECTIONS_FILE and
	// shared by every dashboard that needs it, instead of copy-pasted per
	// dashboard. Each entry names a section and carries the per-dashboard
	// adjustments (id prefix, heading override, extra scoping filters, and
	// whether it leads or trails this dashboard's own widgets) that let one
	// definition serve dashboards that are not identical -- see
	// SectionInclude and expandIncludedSections.
	//
	// Expanded into literal Widgets once, at load time, before every other
	// step of the pipeline, and cleared to nil afterwards: nothing
	// downstream (validation, the handler, the frontend) ever sees a
	// section reference, only the widgets it stood for.
	IncludeSections []SectionInclude `json:"includeSections,omitempty"`

	// DefaultForTeamKeys lists team registry keys (BeTeam.id on the
	// frontend, the values in this deployment's team registry -- not a
	// group id) that should land a signed-in user on this dashboard by
	// default, regardless of this dashboard's own IsDefault/IsTeamBased/
	// Type -- an identity override resolved client-side against the
	// signed-in user's own team.teamKey. Most dashboards should leave this
	// empty and rely on Type-based selection instead (see Type's own doc
	// comment); this exists for a dashboard that doesn't fit the
	// type/family model at all -- e.g. a single-purpose specialist
	// dashboard built for one specific team's own workflow rather than a
	// team-scoped ABT dashboard.
	DefaultForTeamKeys []string `json:"defaultForTeamKeys,omitempty"`

	// FilterPresets is this dashboard's own map of presetKey -> literal
	// filter fragment ({"field":...,"op":...,"values":...}), the
	// dashboard-local half of the preset mechanism (see
	// resolveDashboardFilterPresets and DASHBOARD_PRESETS_FILE's shared
	// half in registry.go). Referenced from anywhere a literal filter
	// object can appear in this dashboard's own widgets/slices --
	// query.filters, an anyOf branch's filters, or a PieSlice's own
	// query.filters -- via {"preset": "presetKey"}. A dashboard-local
	// preset shadows a same-named shared one. Every reference is expanded
	// into its fragment's literal form once, at directory-load time, so
	// nothing downstream (including the frontend and the entity service)
	// ever sees "filterPresets" or a {"preset": ...} reference: this field
	// is cleared to nil once resolution has run (see finalize).
	FilterPresets map[string]map[string]any `json:"filterPresets,omitempty"`
}

// ParseDashboardsConfig decodes DASHBOARDS_CONFIG, a JSON array of Dashboard
// objects (see the Dashboard and WidgetTemplate json tags for the expected
// shape).
//
// DASHBOARDS_CONFIG is DEPRECATED in favour of DASHBOARDS_DIR (see LoadDir),
// and is honoured only when no directory is configured. Cramming every
// dashboard into one environment variable makes a definition unreviewable in
// a diff and gives an error nothing to name; a directory of per-dashboard
// files gives both back.
//
// An empty value yields no dashboards and no error — a deployment with
// neither setting must still start. Anything else that fails is an error the
// caller is expected to make fatal: this used to log and return nil, which
// meant one stray character silently emptied every dashboard in the product
// with a single log line to show for it.
//
// Cross-field validation is the same as the directory loader's, except that
// "type" is not required here: values already deployed in this variable
// predate the field. A definition without one gets a warning and is simply
// invisible to automatic dashboard selection.
func ParseDashboardsConfig(raw string) ([]Dashboard, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	slog.Warn("DASHBOARDS_CONFIG is deprecated; move each dashboard into its own JSON file and set DASHBOARDS_DIR instead")

	var dashboards []Dashboard
	if err := json.Unmarshal([]byte(raw), &dashboards); err != nil {
		return nil, fmt.Errorf("DASHBOARDS_CONFIG: parse: %w", err)
	}

	loaded := make([]sourced, 0, len(dashboards))
	for i, d := range dashboards {
		loaded = append(loaded, sourced{dashboard: d, source: fmt.Sprintf("DASHBOARDS_CONFIG[%d]", i)})
	}
	// The deprecated single-variable path has no directory and therefore
	// neither a DASHBOARD_PRESETS_FILE nor a DASHBOARD_SECTIONS_FILE of its
	// own to read here — nil shared presets, so only a dashboard's own
	// "filterPresets" (if any) can resolve a {"preset": ...} reference on
	// this path, and nil shared sections, so an "includeSections" reference
	// on this path always fails loud with the unknown key named rather than
	// expanding to nothing.
	return finalize(loaded, false, nil, nil)
}

// migrateLegacyWidgetKeys upgrades one pre-rename dashboard definition in
// place, logging one deprecation warning per widget (or slice) it had to touch
// so a deployment still on the old shape is visible in the logs rather than
// silently working forever:
//
//   - widget/slice "filters"       -> "query"
//   - criteria "orGroups": [[..]]  -> "anyOf": [{"filters": [..]}]
//
// The new key always wins when both are present. Nothing here interprets
// filter contents beyond these two renames.
//
// Both loaders run this, so no warning names a configuration mechanism: source
// is the sourced.source of the definition being migrated -- a DASHBOARDS_DIR
// file path or a DASHBOARDS_CONFIG[i] index -- and it is logged so an operator
// is pointed at the file they actually have to edit.
func migrateLegacyWidgetKeys(d *Dashboard, source string) {
	for wi := range d.Widgets {
		w := &d.Widgets[wi]
		switch {
		case w.Query == nil && w.legacyFilters != nil:
			slog.Warn(`dashboard definitions: widget key "filters" is deprecated, rename it to "query"`,
				"source", source, "dashboardId", d.ID, "widgetId", w.ID)
			w.Query = w.legacyFilters
		case w.Query != nil && w.legacyFilters != nil:
			// Same reasoning as the orGroups/anyOf drop below: silent data loss
			// otherwise. Worth warning on even when "query" is populated, and
			// especially when it is an empty {} -- that still wins here, so a
			// widget whose real criteria are all in "filters" renders 0 with
			// nothing in the logs pointing at why.
			slog.Warn(`dashboard definitions: deprecated widget key "filters" dropped because "query" is also set; delete "filters" and keep everything in "query"`,
				"source", source, "dashboardId", d.ID, "widgetId", w.ID, "queryIsEmpty", len(w.Query) == 0)
		}
		w.legacyFilters = nil
		migrateLegacyCriteriaKeys(w.Query, source, d.ID, w.ID, "")
		for si := range w.Slices {
			s := &w.Slices[si]
			switch {
			case s.Query == nil && s.legacyFilters != nil:
				slog.Warn(`dashboard definitions: slice key "filters" is deprecated, rename it to "query"`,
					"source", source, "dashboardId", d.ID, "widgetId", w.ID, "slice", s.Label)
				s.Query = s.legacyFilters
			case s.Query != nil && s.legacyFilters != nil:
				slog.Warn(`dashboard definitions: deprecated slice key "filters" dropped because "query" is also set; delete "filters" and keep everything in "query"`,
					"source", source, "dashboardId", d.ID, "widgetId", w.ID, "slice", s.Label, "queryIsEmpty", len(s.Query) == 0)
			}
			s.legacyFilters = nil
			migrateLegacyCriteriaKeys(s.Query, source, d.ID, w.ID, s.Label)
		}
	}
}

// migrateLegacyCriteriaKeys rewrites the deprecated case-search "orGroups"
// key inside one criteria object into the current "anyOf" shape, in place.
// Each legacy branch was a bare array of filter predicates with implicit AND
// semantics; each current branch is an object carrying its own "filters"
// array. A branch that is already an object is passed through untouched, so
// a half-migrated config is not corrupted.
func migrateLegacyCriteriaKeys(query map[string]any, source, dashboardID, widgetID, slice string) {
	if query == nil {
		return
	}
	legacy, ok := query["orGroups"]
	if !ok {
		return
	}
	delete(query, "orGroups")

	attrs := []any{"source", source, "dashboardId", dashboardID, "widgetId", widgetID}
	if slice != "" {
		attrs = append(attrs, "slice", slice)
	}

	// Both drops below are silent data loss otherwise: the caller wrote a key
	// this loader recognizes and then never sees it again, with nothing in the
	// logs saying why.
	if _, exists := query["anyOf"]; exists {
		slog.Warn(`dashboard definitions: deprecated criteria key "orGroups" dropped because "anyOf" is already set`, attrs...)
		return
	}
	branches, ok := legacy.([]any)
	if !ok {
		slog.Warn(`dashboard definitions: deprecated criteria key "orGroups" is not an array; dropped`, attrs...)
		return
	}

	slog.Warn(`dashboard definitions: criteria key "orGroups" is deprecated, rename it to "anyOf" and wrap each branch as {"filters": [...]}`, attrs...)
	migrated := make([]any, 0, len(branches))
	for _, branch := range branches {
		if arr, isArray := branch.([]any); isArray {
			migrated = append(migrated, map[string]any{"filters": arr})
			continue
		}
		migrated = append(migrated, branch)
	}
	query["anyOf"] = migrated
}

// caseTableResourceTypes is every ResourceType whose /search endpoint is the
// case-search one (see ResourceCase's doc comment): the implied "type"
// filter injectImpliedTypeFilters backstops is only meaningful for these.
var caseTableResourceTypes = map[ResourceType]bool{
	ResourceCase: true, ResourceServiceRequest: true, ResourceSecurityReportAnalysis: true,
	ResourceAnnouncement: true, ResourceEngagement: true,
}

// injectImpliedTypeFilters walks every widget (and, independently, every one
// of its Slices) in d whose ResourceType is a caseTableResourceTypes member,
// ensuring its own Query carries an explicit {"field":"type","op":"in",
// "values":["<resourceType>"]} entry in its top-level "filters" array.
//
// A PieSlice's own Query is checked and injected into independently of its
// owning widget's Query, not just once at the widget level: the frontend
// resolves a slice's value by merging the slice's Query keys on top of the
// widget's own base Query (this slice's keys win on conflict, see PieSlice's
// doc comment) -- a shallow, whole-key merge, not an array-level one. A slice
// that defines its own "filters" array therefore replaces the widget's
// "filters" entirely, including whatever "type" entry was injected there,
// unless the slice's own "filters" carries its own.
//
// If a Query already has an explicit "type" field filter, it is left
// untouched (never overwritten) but warned about: after this mechanism (and
// the config rewrite it backstops) lands, no widget should still need to
// paste one in by hand.
func injectImpliedTypeFilters(d *Dashboard, source string) {
	for wi := range d.Widgets {
		w := &d.Widgets[wi]
		if !caseTableResourceTypes[w.ResourceType] {
			continue
		}
		w.Query = injectTypeFilter(w.Query, w.ResourceType, source, d.ID, w.ID, "")
		for si := range w.Slices {
			s := &w.Slices[si]
			s.Query = injectTypeFilter(s.Query, w.ResourceType, source, d.ID, w.ID, s.Label)
		}
	}
}

// injectTypeFilter returns query (creating it if nil) with an explicit
// "type" filter entry guaranteed present in its top-level "filters" array,
// for the given resourceType. See injectImpliedTypeFilters for the full
// rationale; slice is "" when called for a widget's own Query, and the
// slice's Label otherwise, purely so the warning below can name it.
func injectTypeFilter(query map[string]any, resourceType ResourceType, source, dashboardID, widgetID, slice string) map[string]any {
	if query == nil {
		query = map[string]any{}
	}
	filters, isArray := query["filters"].([]any)
	if raw, present := query["filters"]; present && !isArray {
		// A non-array "filters" is silently ignored by the loop below and
		// then overwritten by the auto-injected type filter, so the widget
		// would search with criteria the definition does not state. Nothing
		// downstream can detect that, so say so at load time.
		attrs := []any{"source", source, "dashboardId", dashboardID, "widgetId", widgetID, "got", fmt.Sprintf("%T", raw)}
		if slice != "" {
			attrs = append(attrs, "slice", slice)
		}
		slog.Warn(`dashboard definitions: widget "filters" is not an array; the configured value is discarded and replaced by the auto-injected "type" filter`, attrs...)
	}
	for _, entry := range filters {
		m, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		if m["field"] == "type" {
			attrs := []any{"source", source, "dashboardId", dashboardID, "widgetId", widgetID}
			if slice != "" {
				attrs = append(attrs, "slice", slice)
			}
			slog.Warn(`dashboard definitions: widget already has an explicit "type" filter; resourceType now auto-injects this at load time, so the explicit one is redundant -- remove it in a later config cleanup pass`, attrs...)
			return query
		}
	}
	query["filters"] = append(filters, map[string]any{
		"field": "type", "op": "in", "values": []any{string(resourceType)},
	})
	return query
}

// isPresetRef reports whether m is a filter-preset reference, the
// {"preset": "presetKey"} shape a literal filter object ({"field":
// ...,"op":...,"values":...}) may be replaced with wherever one is legal
// (see resolveDashboardFilterPresets). Any map carrying a "preset" key is
// treated as an attempted reference -- including one with extra keys or a
// non-string value -- so a malformed reference is rejected loudly by the
// caller rather than silently treated as a literal filter with a
// coincidentally-named "preset" field.
func isPresetRef(m map[string]any) bool {
	_, has := m["preset"]
	return has
}

// resolvePresetRef expands one {"preset": "presetKey"} reference (m, which
// isPresetRef has already confirmed carries a "preset" key) into a fresh
// copy of the fragment presets[key] defines, or fails loud, naming ctx (the
// dashboard/widget/slice/position the reference was found at) if the
// reference is malformed or the key is unknown.
func resolvePresetRef(m map[string]any, presets map[string]map[string]any, ctx string) (map[string]any, error) {
	if len(m) != 1 {
		return nil, fmt.Errorf("%s: preset reference %v carries extra keys; expected exactly {\"preset\": \"<key>\"}", ctx, m)
	}
	key, ok := m["preset"].(string)
	if !ok || strings.TrimSpace(key) == "" {
		return nil, fmt.Errorf("%s: preset reference %v has a non-string or empty \"preset\" key", ctx, m)
	}
	fragment, ok := presets[key]
	if !ok {
		return nil, fmt.Errorf("%s: references unknown filter preset %q; define it in this dashboard's own \"filterPresets\" or in the shared DASHBOARD_PRESETS_FILE", ctx, key)
	}
	out := make(map[string]any, len(fragment))
	for k, v := range fragment {
		out[k] = v
	}
	return out, nil
}

// resolveFilterList resolves every {"preset": "key"} entry in list (one
// "filters" array, see resolveQueryPresets) into its literal fragment,
// leaving every other entry untouched. ctx names the array for error
// messages; list itself is never mutated, a new slice is returned.
func resolveFilterList(list []any, presets map[string]map[string]any, ctx string) ([]any, error) {
	out := make([]any, len(list))
	for i, entry := range list {
		m, ok := entry.(map[string]any)
		if !ok || !isPresetRef(m) {
			out[i] = entry
			continue
		}
		resolved, err := resolvePresetRef(m, presets, fmt.Sprintf("%s[%d]", ctx, i))
		if err != nil {
			return nil, err
		}
		out[i] = resolved
	}
	return out, nil
}

// resolveQueryPresets expands every {"preset": "key"} reference inside
// query in place -- in its own top-level "filters" array, and in each
// "anyOf" branch's own "filters" array (the only two places a literal filter
// object appears in this DSL, see WidgetTemplate's doc comment). query may
// be nil, in which case there is nothing to resolve. ctx names the owning
// widget/slice for error messages.
func resolveQueryPresets(query map[string]any, presets map[string]map[string]any, ctx string) error {
	if query == nil {
		return nil
	}
	if arr, ok := query["filters"].([]any); ok {
		resolved, err := resolveFilterList(arr, presets, ctx+`: "query.filters"`)
		if err != nil {
			return err
		}
		query["filters"] = resolved
	}
	if anyOf, ok := query["anyOf"].([]any); ok {
		for i, branch := range anyOf {
			bm, ok := branch.(map[string]any)
			if !ok {
				continue
			}
			if arr, ok := bm["filters"].([]any); ok {
				resolved, err := resolveFilterList(arr, presets, fmt.Sprintf("%s: \"query.anyOf[%d].filters\"", ctx, i))
				if err != nil {
					return err
				}
				bm["filters"] = resolved
			}
		}
	}
	return nil
}

// validatePresetsNotRecursive rejects a preset whose own fragment is itself
// a {"preset": ...} reference: presets are a flat, one-level expansion, and
// silently ignoring a nested reference would leave a widget's Query holding
// an unresolved {"preset": ...} object that reaches the frontend/entity
// service looking like (and being misread as) a literal filter.
func validatePresetsNotRecursive(presets map[string]map[string]any, source, label string) error {
	for key, fragment := range presets {
		if isPresetRef(fragment) {
			return fmt.Errorf("dashboard definitions: %s: %s preset %q is itself a preset reference; a preset's fragment must be a literal filter, presets cannot reference other presets", source, label, key)
		}
	}
	return nil
}

// resolveDashboardFilterPresets expands every {"preset": "key"} reference in
// d's widgets (and their slices) into its literal fragment, once, in place.
// The effective preset set for d is sharedPresets with d.FilterPresets
// merged on top -- a dashboard-local preset shadows a same-named shared one
// on key collision, by design (a dashboard can intentionally override a
// shared default). d.FilterPresets itself is validated non-recursive first,
// same as the shared set (see LoadSharedPresets).
func resolveDashboardFilterPresets(d *Dashboard, sharedPresets map[string]map[string]any, source string) error {
	if err := validatePresetsNotRecursive(d.FilterPresets, source, `dashboard-local "filterPresets"`); err != nil {
		return err
	}

	presets := sharedPresets
	if len(d.FilterPresets) > 0 {
		merged := make(map[string]map[string]any, len(sharedPresets)+len(d.FilterPresets))
		for k, v := range sharedPresets {
			merged[k] = v
		}
		for k, v := range d.FilterPresets {
			merged[k] = v
		}
		presets = merged
	}
	// Deliberately no "presets is empty, skip resolution" short circuit even
	// though it looks like a safe optimization: a widget can carry a
	// {"preset": "key"} reference with no presets configured anywhere (a
	// typo, or a preset definition that was removed), and that must still
	// fail loud here -- resolveQueryPresets/resolvePresetRef already handles
	// a nil/empty presets map correctly (the lookup simply misses and
	// reports the unknown key), so there is nothing to gain by skipping.

	for wi := range d.Widgets {
		w := &d.Widgets[wi]
		ctx := fmt.Sprintf("dashboard definitions: %s (id %q): widget %q", source, d.ID, w.ID)
		if err := resolveQueryPresets(w.Query, presets, ctx); err != nil {
			return err
		}
		for si := range w.Slices {
			s := &w.Slices[si]
			sctx := fmt.Sprintf("%s: slice %q", ctx, s.Label)
			if err := resolveQueryPresets(s.Query, presets, sctx); err != nil {
				return err
			}
		}
	}
	return nil
}
