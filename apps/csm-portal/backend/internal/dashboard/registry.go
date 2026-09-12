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
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Type classifies a dashboard by the audience it is built for. It is the
// field automatic dashboard selection keys off: a caller whose team family is
// cre-abt/cre lands on the default TypeCRE dashboard, sre-abt/sre on the
// default TypeSRE one, and a caller with no team at all on the default
// TypeCS one. validate enforces at most one IsDefault dashboard per Type, so
// a default of each type can coexist without either resolving by nothing
// more than file ordering.
type Type string

const (
	// TypeCRE is a Customer Renewal & Expansion team dashboard. Team-scoped.
	TypeCRE Type = "cre"
	// TypeSRE is a Site Reliability Engineering team dashboard. Team-scoped.
	TypeSRE Type = "sre"
	// TypeCS is a CS-organisation-wide dashboard, not scoped to a team.
	TypeCS Type = "cs"
)

var validTypes = map[Type]bool{TypeCRE: true, TypeSRE: true, TypeCS: true}

// definitionExt is the only file extension the definition directory loader
// considers. Everything else in the directory (README, .yaml, editor swap
// files, subdirectories) is ignored rather than erroring: the directory is
// expected to be hand-maintained.
const definitionExt = ".json"

// sourced pairs a decoded dashboard with where it came from, so every
// validation error can name the offending file (or the offending index in the
// deprecated single-variable config) instead of just an id.
type sourced struct {
	dashboard Dashboard
	source    string
}

// dashboardPart is a widgets-only fragment split out of a dashboard whose
// full definition would otherwise exceed the deploy path's per-file size
// limit (see loadDir's doc comment on "partOf"). It carries no dashboard
// metadata of its own -- id, displayName, type and everything else always
// come from the primary file it belongs to; a part only ever contributes
// more entries to that primary's Widgets.
type dashboardPart struct {
	partOf  string
	widgets []WidgetTemplate
	source  string
}

// dashboardFile is the decode target for one *.json file in DASHBOARDS_DIR.
// A file is a PART of another dashboard's definition, not a dashboard in its
// own right, exactly when it carries a non-empty "partOf" -- every other
// field decodes into the embedded Dashboard as before, unused for a part
// file except Widgets. See mergeParts.
type dashboardFile struct {
	Dashboard
	PartOf string `json:"partOf,omitempty"`
}

// partFileAllowedKeys is every JSON key a part file (one with "partOf" set)
// is allowed to carry — see rejectUnexpectedPartFields.
var partFileAllowedKeys = map[string]bool{"partOf": true, "widgets": true}

// rejectUnexpectedPartFields is a hard load failure for a part file that
// also sets any embedded Dashboard field beyond Widgets -- e.g. "id" or
// "filterPresets". dashboardFile decodes those fields into f.Dashboard like
// any other file, but loadDir's part-file branch only ever reads f.Widgets
// back out (see mergeParts): every other field a part file's author sets
// would otherwise decode successfully and then vanish without a trace,
// which is exactly the "silently misroutes/drops data" failure mode
// mergeParts's own doc comment says this loader refuses to tolerate
// elsewhere. Re-decoding raw into a generic key set (rather than comparing
// f.Dashboard's fields against their zero values) is deliberate: a
// zero-value comparison can't tell "the author wrote isDefault: false" apart
// from "the author never mentioned isDefault at all", but a raw key lookup
// can.
func rejectUnexpectedPartFields(raw []byte, path string) error {
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(raw, &keys); err != nil {
		return fmt.Errorf("dashboard definitions: parse %q: %w", path, err)
	}
	for key := range keys {
		if !partFileAllowedKeys[key] {
			return fmt.Errorf(
				"dashboard definitions: %s: a part file (\"partOf\" set) may only carry \"partOf\" and \"widgets\", found unexpected field %q",
				path, key,
			)
		}
	}
	return nil
}

// mergeParts folds every part's widgets onto its primary dashboard's own
// Widgets, in the order the parts were encountered (which is loadDir's
// lexical filename order, same determinism guarantee LoadDir already makes
// for dashboards themselves).
//
// A part file exists purely so a dashboard whose full widget set no longer
// fits Choreo's confirmed ~20KB per-file deploy limit can keep growing:
// splitting it across files is invisible past this point -- finalize and
// validate see one Dashboard with one concatenated Widgets slice, identical
// to a dashboard that was always small enough to live in one file. That is
// also why merging happens before finalize runs: a part's widgets go through
// the exact same key-migration/preset-expansion/type-injection/validation
// pipeline as any other widget, including the existing duplicate-widget-id
// check (validateWidgets), which is what catches a widget id colliding
// across a primary and its part -- no separate collision check is needed
// here.
//
// A part whose "partOf" does not match any loaded primary dashboard's id is
// a hard load failure, same fail-loud philosophy as everything else this
// loader rejects (a dropped/misrouted set of widgets is exactly the kind of
// silent gap this package refuses to tolerate elsewhere).
func mergeParts(loaded []sourced, parts []dashboardPart) ([]sourced, error) {
	if len(parts) == 0 {
		return loaded, nil
	}

	byID := make(map[string]int, len(loaded))
	for i, l := range loaded {
		byID[l.dashboard.ID] = i
	}

	for _, p := range parts {
		i, ok := byID[p.partOf]
		if !ok {
			return nil, fmt.Errorf("dashboard definitions: %s: \"partOf\" %q does not match any loaded dashboard id", p.source, p.partOf)
		}
		loaded[i].dashboard.Widgets = append(loaded[i].dashboard.Widgets, p.widgets...)
	}

	return loaded, nil
}

// Registry holds the dashboard definitions a running process serves.
//
// Two modes, chosen at construction:
//
//   - default (hotReload false): the definitions are read once, at startup,
//     and every subsequent read is served from memory. There is no disk I/O
//     on the request path at all. This is the deployed behaviour.
//   - hot-reload (hotReload true): every read re-reads the directory, so
//     editing a definition file is picked up without restarting. Intended for
//     local development only; it puts a directory scan plus a file read per
//     definition on every request.
type Registry struct {
	mu     sync.RWMutex
	cached []Dashboard

	dir       string
	hotReload bool

	// load is the directory reader, injectable so tests can count how many
	// times the registry actually touches the disk. nil for a static
	// registry.
	load func(dir string) ([]Dashboard, error)

	// presets and sections are the shared catalogues the last successful
	// load read, retained ONLY so the builder-facing endpoints can list
	// what an author may reference (see FilterPresets/SharedSections).
	// Nothing on the dashboard-serving path reads them: by the time a
	// Dashboard reaches cached, every {"preset": ...} reference and every
	// includeSections entry has already been expanded away.
	presets  map[string]map[string]any
	sections map[string]SharedSection

	// loadCatalogues re-reads just the two shared files, for hot-reload
	// mode. Deliberately separate from load rather than folded into it:
	// load's signature is injected by tests that count disk reads, and the
	// catalogue endpoints are not on the dashboard-serving path, so there
	// is no reason to make every dashboard read carry this too. nil for a
	// static registry.
	loadCatalogues func() (map[string]map[string]any, map[string]SharedSection, error)
}

// NewStaticRegistry wraps an already-decoded set of dashboards. It never
// touches the disk. Used by tests and by the deprecated DASHBOARDS_CONFIG
// path, which has no directory to re-read.
func NewStaticRegistry(dashboards []Dashboard) *Registry {
	return &Registry{cached: append([]Dashboard(nil), dashboards...)}
}

// NewDirRegistry builds a registry over a directory of per-dashboard JSON
// files. It always performs the initial load eagerly and returns the error,
// in both modes: a definition set that is broken at startup is a broken
// deploy, and the caller is expected to make it fatal. hotReload only governs
// what happens on subsequent reads.
//
// presetsFile is the shared filter-preset file (see LoadSharedPresets and
// DASHBOARD_PRESETS_FILE in cmd/server/main.go) and sectionsFile the shared
// section file (see LoadSharedSections and DASHBOARD_SECTIONS_FILE); "" for
// either means none is configured, which is legal, same as an unset
// DASHBOARDS_DIR. Both are re-read on every load alongside dir, so
// hot-reload mode also picks up an edited presets or sections file without a
// restart.
func NewDirRegistry(dir string, hotReload bool, presetsFile, sectionsFile string) (*Registry, error) {
	load := func(d string) ([]Dashboard, error) {
		sharedPresets, err := LoadSharedPresets(presetsFile)
		if err != nil {
			return nil, err
		}
		sharedSections, err := LoadSharedSections(sectionsFile)
		if err != nil {
			return nil, err
		}
		return loadDirWithSections(d, sharedPresets, sharedSections, sectionsFile)
	}
	loadCatalogues := func() (map[string]map[string]any, map[string]SharedSection, error) {
		presets, err := LoadSharedPresets(presetsFile)
		if err != nil {
			return nil, nil, err
		}
		sections, err := LoadSharedSections(sectionsFile)
		if err != nil {
			return nil, nil, err
		}
		return presets, sections, nil
	}

	r := &Registry{dir: dir, hotReload: hotReload, load: load, loadCatalogues: loadCatalogues}
	dashboards, err := r.load(dir)
	if err != nil {
		return nil, err
	}
	// Cannot fail here: load has already parsed and validated both files.
	presets, sections, err := r.loadCatalogues()
	if err != nil {
		return nil, err
	}
	r.cached = dashboards
	r.presets = presets
	r.sections = sections
	return r, nil
}

// Dashboards returns the registry's dashboards in their deterministic order.
//
// In hot-reload mode a failed re-read does NOT take the service down and does
// NOT start returning an empty list: it logs the error loudly and keeps
// serving the last known-good set. The startup load has already proven the
// directory is valid, so a failure here is almost always an editor mid-save
// writing half a JSON file, and killing a dev server (or blanking its
// dashboards) on every keystroke would make the loop unusable. The loud log
// keeps the failure visible, which is the part that matters. The startup path
// is unaffected and still fails hard.
func (r *Registry) Dashboards() []Dashboard {
	if r == nil {
		return nil
	}
	if r.hotReload && r.load != nil {
		if dashboards, err := r.load(r.dir); err != nil {
			slog.Error("dashboard definitions: hot reload failed; continuing to serve the last known-good definitions",
				"dir", r.dir, "err", err)
		} else {
			r.mu.Lock()
			r.cached = dashboards
			r.mu.Unlock()
		}
	}

	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.cached
}

// ByID looks a dashboard up by id, returning ok=false if the id is not in the
// registry. It goes through Dashboards, so it honours hot-reload too.
func (r *Registry) ByID(id string) (Dashboard, bool) {
	for _, d := range r.Dashboards() {
		if d.ID == id {
			return d, true
		}
	}
	return Dashboard{}, false
}

// FilterPresets returns the shared filter-preset catalogue the registry
// loaded, keyed by preset name.
//
// This exists for the dashboard builder, which needs to offer an author the
// presets they may reference by name. It is NOT used to serve a dashboard:
// preset references are expanded at load time and erased, so nothing on that
// path needs the catalogue.
//
// The returned map is the registry's own — callers must not mutate it. A
// static registry (the deprecated DASHBOARDS_CONFIG path) has no catalogue
// and returns nil, which callers must treat as "none configured" rather than
// an error: a deployment with no presets file is legal.
func (r *Registry) FilterPresets() map[string]map[string]any {
	if r == nil {
		return nil
	}
	r.refreshCatalogues()
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.presets
}

// SharedSections returns the shared reusable-section catalogue the registry
// loaded, keyed by section name. Same contract as FilterPresets.
func (r *Registry) SharedSections() map[string]SharedSection {
	if r == nil {
		return nil
	}
	r.refreshCatalogues()
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.sections
}

// refreshCatalogues re-reads the two shared files in hot-reload mode, with
// the same "keep the last known-good set on failure" behaviour Dashboards
// documents and for the same reason: an editor mid-save must not blank the
// builder's picker.
func (r *Registry) refreshCatalogues() {
	if !r.hotReload || r.loadCatalogues == nil {
		return
	}
	presets, sections, err := r.loadCatalogues()
	if err != nil {
		slog.Error("dashboard shared catalogues: hot reload failed; continuing to serve the last known-good catalogues",
			"dir", r.dir, "err", err)
		return
	}
	r.mu.Lock()
	r.presets = presets
	r.sections = sections
	r.mu.Unlock()
}

// active is the registry the HTTP handlers serve from. It is installed once
// during startup by cmd/server/main.go (and by tests). A nil active registry
// serves no dashboards rather than panicking: GET /dashboards returns an
// empty list and GET /dashboards/{id} 404s, exactly as an empty registry did
// before this type existed.
var (
	activeMu sync.RWMutex
	active   *Registry
)

// SetActive installs the registry the handlers read from.
func SetActive(r *Registry) {
	activeMu.Lock()
	defer activeMu.Unlock()
	active = r
}

// Active returns the installed registry, which may be nil.
func Active() *Registry {
	activeMu.RLock()
	defer activeMu.RUnlock()
	return active
}

// All returns every dashboard the active registry serves.
func All() []Dashboard { return Active().Dashboards() }

// ByID looks a dashboard up by id in the active registry.
func ByID(id string) (Dashboard, bool) { return Active().ByID(id) }

// FilterPresets returns the active registry's shared filter-preset catalogue.
func FilterPresets() map[string]map[string]any { return Active().FilterPresets() }

// SharedSections returns the active registry's shared reusable-section catalogue.
func SharedSections() map[string]SharedSection { return Active().SharedSections() }

// LoadDir reads every *.json file in dir whose name does not start with "_"
// as one dashboard definition. The filename is otherwise not significant in
// any way: id, displayName, type and the rest all come from the file's
// content, and files are processed in lexical filename order purely so the
// resulting order (which is what the frontend's dashboard picker shows) is
// deterministic.
//
// The leading-underscore exclusion exists because the shared filter-presets
// file (DASHBOARD_PRESETS_FILE, conventionally "_presets.json" — see
// LoadSharedPresets) lives in the same directory as the dashboard
// definitions it is shared across, not a separate one, and is not itself a
// dashboard: without the exclusion, LoadDir would try to decode it as one
// and fail on its missing "id"/"displayName".
//
// Every failure is an error naming the offending file, and none of them is
// recoverable by skipping the file. A dropped dashboard is invisible: the
// picker simply has one fewer entry, with nothing anywhere saying why. That
// is the same failure class as a silently-ignored filter, which this project
// has been bitten by repeatedly, so an unreadable file, a malformed one, a
// missing id, a duplicate id, a bad type and a contradictory type/isTeamBased
// combination all fail the whole load.
//
// An empty directory is legal and yields no dashboards: a deployment that has
// not authored any definitions yet must still start and serve every other
// endpoint. A missing directory is not legal — it is a misconfigured path.
//
// A dashboard whose widget count outgrows Choreo's confirmed ~20KB per-file
// deploy limit for this directory can be split across more than one file: the
// PRIMARY file is unchanged (a complete object carrying id/type/displayName/
// widgets/etc), and any number of additional PART files carry only
// {"partOf": "<dashboard id>", "widgets": [...]} -- no other dashboard
// metadata. Every part's widgets are appended to its primary's Widgets before
// validation runs (see mergeParts), so a widget-id collision between a
// primary and any of its parts fails the whole load exactly like a collision
// within one file already does, and a part whose "partOf" names no loaded
// dashboard is also a hard load failure. Filenames still carry no meaning:
// nothing about a part's own filename ties it to its primary, only the
// "partOf" value does.
//
// LoadDir never applies shared filter presets (see LoadSharedPresets) — it is
// the plain, no-shared-presets form kept for the many callers (including this
// package's own tests) that only care about a directory of definitions.
// NewDirRegistry is the production path and always goes through loadDir with
// whatever DASHBOARD_PRESETS_FILE resolves to, which may itself be an empty
// map.
func LoadDir(dir string) ([]Dashboard, error) {
	return loadDir(dir, nil, nil)
}

// loadDir is LoadDir's shared-presets- and shared-sections-aware
// counterpart.
func loadDir(dir string, sharedPresets map[string]map[string]any, sharedSections map[string]SharedSection) ([]Dashboard, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("dashboard definitions: read directory %q: %w", dir, err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.EqualFold(filepath.Ext(entry.Name()), definitionExt) {
			continue
		}
		if strings.HasPrefix(entry.Name(), "_") {
			// Reserved for non-dashboard config living alongside the
			// definitions, e.g. the shared filter-presets file — see
			// LoadDir's doc comment.
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)

	loaded := make([]sourced, 0, len(names))
	var parts []dashboardPart
	for _, name := range names {
		path := filepath.Join(dir, name)
		raw, err := os.ReadFile(path) // #nosec G304 -- path is deployment configuration, not user input
		if err != nil {
			return nil, fmt.Errorf("dashboard definitions: read %q: %w", path, err)
		}
		var f dashboardFile
		if err := json.Unmarshal(raw, &f); err != nil {
			return nil, fmt.Errorf("dashboard definitions: parse %q: %w", path, err)
		}
		if f.PartOf != "" {
			if err := rejectUnexpectedPartFields(raw, path); err != nil {
				return nil, err
			}
			parts = append(parts, dashboardPart{partOf: f.PartOf, widgets: f.Widgets, source: path})
			continue
		}
		loaded = append(loaded, sourced{dashboard: f.Dashboard, source: path})
	}

	loaded, err = mergeParts(loaded, parts)
	if err != nil {
		return nil, err
	}

	return finalize(loaded, true, sharedPresets, sharedSections)
}

// loadDirWithSections is loadDir plus catalogue-level validation of every
// shared section, including the ones no dashboard references.
//
// The validation has to happen HERE rather than beside LoadSharedSections,
// because it needs the decoded dashboards: a section may legitimately
// reference a preset that only a dashboard's own "filterPresets" defines, so
// the set a section can resolve against is the union of the shared file and
// every dashboard's local presets. See validateSharedSections.
func loadDirWithSections(dir string, sharedPresets map[string]map[string]any, sharedSections map[string]SharedSection, sectionsSource string) ([]Dashboard, error) {
	if len(sharedSections) > 0 {
		resolvable, err := resolvablePresets(dir, sharedPresets)
		if err != nil {
			return nil, err
		}
		if err := validateSharedSections(sharedSections, resolvable, sectionsSource); err != nil {
			return nil, err
		}
	}
	return loadDir(dir, sharedPresets, sharedSections)
}

// resolvablePresets is the shared presets plus every dashboard-local
// "filterPresets" in dir, which together are what a section's preset
// reference can resolve against once it is expanded into a dashboard.
//
// A name defined in more than one place collapses to one entry: this set is
// only ever asked "does this name resolve anywhere", never "what does it
// expand to", so which definition wins does not matter here -- that is
// decided per dashboard by resolveDashboardFilterPresets.
func resolvablePresets(dir string, sharedPresets map[string]map[string]any) (map[string]map[string]any, error) {
	out := make(map[string]map[string]any, len(sharedPresets))
	for k, v := range sharedPresets {
		out[k] = v
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("dashboard definitions: read directory %q: %w", dir, err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), definitionExt) ||
			strings.HasPrefix(entry.Name(), "_") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		raw, err := os.ReadFile(path) //nolint:gosec // path is deployment configuration, not user input
		if err != nil {
			return nil, fmt.Errorf("dashboard definitions: read %q: %w", path, err)
		}
		var d Dashboard
		if err := json.Unmarshal(raw, &d); err != nil {
			return nil, fmt.Errorf("dashboard definitions: parse %q: %w", path, err)
		}
		for k, v := range d.FilterPresets {
			out[k] = v
		}
	}
	return out, nil
}

// LoadSharedPresets reads path (DASHBOARD_PRESETS_FILE — see
// cmd/server/main.go) as a JSON object mapping presetKey -> literal filter
// fragment ({"field":...,"op":...,"values":...}) — the same shape a
// dashboard's own top-level "filterPresets" object uses (see
// Dashboard.FilterPresets). An empty path is legal and yields an empty,
// nil-error map: a deployment that has not authored a shared presets file
// yet must still start, same "must still start" philosophy LoadDir's empty
// directory already gets. A configured path that cannot be read or parsed,
// or that contains a preset whose own fragment is itself a {"preset": ...}
// reference (presets cannot reference other presets), is fatal and names the
// file.
func LoadSharedPresets(path string) (map[string]map[string]any, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	raw, err := os.ReadFile(path) // #nosec G304 -- path is deployment configuration, not user input
	if err != nil {
		return nil, fmt.Errorf("dashboard filter presets: read %q: %w", path, err)
	}
	var presets map[string]map[string]any
	if err := json.Unmarshal(raw, &presets); err != nil {
		return nil, fmt.Errorf("dashboard filter presets: parse %q: %w", path, err)
	}
	if err := validatePresetsNotRecursive(presets, path, "shared"); err != nil {
		return nil, err
	}
	return presets, nil
}

// finalize runs the shared post-decode pipeline over a decoded set:
// shared-section expansion first (so every later step sees one flat widget
// list), then deprecated-key migration (so everything after sees the current
// shape), then filter-preset expansion and implied-"type"-filter injection
// (so validation and every caller downstream see fully literal, complete
// filters), then cross-field validation. requireType is true for the
// directory loader, where every definition is authored against the current
// schema, and false for the deprecated DASHBOARDS_CONFIG path, whose
// already-deployed values predate the type field entirely. sharedPresets is
// the DASHBOARD_PRESETS_FILE set (see LoadSharedPresets) and sharedSections
// the DASHBOARD_SECTIONS_FILE set (see LoadSharedSections); nil/empty is
// legal for either and means none is configured, same as an unset
// DASHBOARDS_DIR.
func finalize(loaded []sourced, requireType bool, sharedPresets map[string]map[string]any, sharedSections map[string]SharedSection) ([]Dashboard, error) {
	// Section expansion runs before everything else so that from here down
	// an included widget is indistinguishable from an inline one -- key
	// migration, preset resolution, type injection and validation all see
	// one flat widget list and need no notion of where a widget came from.
	for i := range loaded {
		if err := expandIncludedSections(&loaded[i].dashboard, sharedSections, loaded[i].source); err != nil {
			return nil, err
		}
	}

	for i := range loaded {
		migrateLegacyWidgetKeys(&loaded[i].dashboard, loaded[i].source)
	}

	for i := range loaded {
		d := &loaded[i].dashboard
		if err := resolveDashboardFilterPresets(d, sharedPresets, loaded[i].source); err != nil {
			return nil, err
		}
		injectImpliedTypeFilters(d, loaded[i].source)
		// Presets are a pure config-authoring convenience: once every
		// {"preset": ...} reference in this dashboard has been expanded,
		// nothing downstream (validation, the handler, the frontend) has any
		// use for the raw preset definitions themselves, and they must never
		// be visible past load time.
		d.FilterPresets = nil
	}

	if err := validate(loaded, requireType); err != nil {
		return nil, err
	}

	dashboards := make([]Dashboard, 0, len(loaded))
	for _, l := range loaded {
		dashboards = append(dashboards, l.dashboard)
	}
	return dashboards, nil
}

// validate enforces every rule that cannot be expressed in the JSON shape.
//
// The type / isDefault / isTeamBased trio is deliberately three independent
// fields rather than two derived from one, which means they can be set to
// states that contradict each other. Those states are rejected here, loudly
// and by filename, rather than normalised silently — a dashboard that quietly
// behaves as something other than what its file says is worse than a failed
// deploy:
//
//   - type cre or sre with isTeamBased false. Both are team-scoped by
//     definition: the frontend auto-selects them from the caller's own team
//     and offers a team picker. Without isTeamBased there is no picker, so
//     the dashboard can never be scoped to the team it claims to target.
//   - type cs with isTeamBased true. cs is the organisation-wide dashboard,
//     and it is what a caller with no team at all falls back to. A team
//     picker on it contradicts both roles.
//   - more than one isDefault dashboard of the same type -- or, on the
//     deprecated untyped DASHBOARDS_CONFIG path, more than one untyped
//     isDefault dashboard. One isDefault dashboard per type is legal and by
//     design: the frontend selects its landing dashboard from the caller's
//     own team family against Type, so a cre default and an sre default (and
//     a cs default) can all coexist without any of them resolving by nothing
//     more than LoadDir's filename ordering. An untyped default does not
//     share a "slot" with any typed default -- it has no type to key off --
//     so it only ever collides with another untyped default, which is only
//     reachable via the deprecated single-variable path.
func validate(loaded []sourced, requireType bool) error {
	byID := make(map[string]string, len(loaded))
	defaultByType := make(map[Type]string, len(loaded))
	untypedDefaultSource := ""
	defaultForTeamKeyOwner := make(map[string]string, len(loaded))

	for _, l := range loaded {
		d := l.dashboard

		if strings.TrimSpace(d.ID) == "" {
			return fmt.Errorf("dashboard definitions: %s: \"id\" is empty; the id is the dashboard's identity and is never derived from the filename", l.source)
		}
		if prev, dup := byID[d.ID]; dup {
			return fmt.Errorf("dashboard definitions: %s: duplicate dashboard id %q, already defined by %s", l.source, d.ID, prev)
		}
		byID[d.ID] = l.source

		if strings.TrimSpace(d.DisplayName) == "" {
			return fmt.Errorf("dashboard definitions: %s (id %q): \"displayName\" is empty", l.source, d.ID)
		}

		if err := validateWidgets(d, l.source); err != nil {
			return err
		}

		// CsmDashboardPage selects a caller's landing dashboard by matching
		// its team key against DefaultForTeamKeys, taking the first list
		// entry with find. If two dashboards claimed the same team key, that
		// choice would silently fall to list order instead of config intent.
		// Track which dashboard claims each key and reject a second claim
		// from a different dashboard. Ownership is keyed by dashboard id, not
		// source file: the deprecated DASHBOARDS_CONFIG path can decode
		// multiple distinct dashboard objects from one source value, so two
		// different dashboards sharing that source would otherwise both look
		// like the same owner and never trip this check.
		for _, teamKey := range d.DefaultForTeamKeys {
			if prev, claimed := defaultForTeamKeyOwner[teamKey]; claimed && prev != d.ID {
				return fmt.Errorf("dashboard definitions: %s (id %q): defaultForTeamKeys key %q is already claimed by %s; each team key must resolve to exactly one dashboard",
					l.source, d.ID, teamKey, prev)
			}
			defaultForTeamKeyOwner[teamKey] = d.ID
		}

		// Before the type branch below, which skips the rest of the loop for an
		// untyped definition: an untyped isDefault dashboard counts here too,
		// so two of them on the deprecated DASHBOARDS_CONFIG path are caught
		// rather than left to file ordering. It is kept in its own bucket,
		// separate from defaultByType, so it never collides with a typed
		// default -- there is no type to key on, so there is no shared slot.
		if d.IsDefault {
			if d.Type == "" {
				if untypedDefaultSource != "" {
					return fmt.Errorf("dashboard definitions: %s (id %q): a second untyped \"isDefault\" dashboard; %s already claims the untyped default slot, and selection needs exactly one. This is only reachable via the deprecated DASHBOARDS_CONFIG path, which predates \"type\"",
						l.source, d.ID, untypedDefaultSource)
				}
				untypedDefaultSource = l.source
			} else if prev, claimed := defaultByType[d.Type]; claimed {
				return fmt.Errorf("dashboard definitions: %s (id %q, type %q): a second \"isDefault\" dashboard of type %q; %s already claims that type's default, and selection needs exactly one default per type",
					l.source, d.ID, d.Type, d.Type, prev)
			} else {
				defaultByType[d.Type] = l.source
			}
		}

		if d.Type == "" {
			if requireType {
				return fmt.Errorf("dashboard definitions: %s (id %q): \"type\" is required; expected one of %q, %q, %q",
					l.source, d.ID, TypeCRE, TypeSRE, TypeCS)
			}
			slog.Warn("dashboard definitions: no \"type\" set; automatic dashboard selection cannot classify this dashboard",
				"source", l.source, "dashboardId", d.ID)
			continue
		}
		if !validTypes[d.Type] {
			return fmt.Errorf("dashboard definitions: %s (id %q): unknown \"type\" %q; expected one of %q, %q, %q",
				l.source, d.ID, d.Type, TypeCRE, TypeSRE, TypeCS)
		}

		switch {
		case (d.Type == TypeCRE || d.Type == TypeSRE) && !d.IsTeamBased:
			return fmt.Errorf("dashboard definitions: %s (id %q): contradictory configuration: \"type\": %q is team-scoped but \"isTeamBased\" is false; set isTeamBased true or change the type to %q",
				l.source, d.ID, d.Type, TypeCS)
		case d.Type == TypeCS && d.IsTeamBased:
			return fmt.Errorf("dashboard definitions: %s (id %q): contradictory configuration: \"type\": %q is organisation-wide but \"isTeamBased\" is true; set isTeamBased false or change the type to %q or %q",
				l.source, d.ID, d.Type, TypeCRE, TypeSRE)
		}
	}

	return nil
}

// validWidgetResourceTypes and validWidgetShapes mirror the enums the public
// schema declares for WidgetTemplate.resourceType and .shape. A value outside
// them is not a degraded widget, it is a dead one: the frontend routes on
// resourceType to pick the search endpoint and switches on shape to pick a
// renderer, so an unknown value renders nothing at all.
var validWidgetResourceTypes = map[ResourceType]bool{
	ResourceCase: true, ResourceIncident: true, ResourceChangeRequest: true,
	ResourceAccount: true, ResourceProject: true, ResourceUser: true,
	ResourceTimeCard: true, ResourceProblem: true, ResourceIncidentTask: true,
	ResourceProductVulnerability: true,
	ResourceCallRequest:          true,
	// The remaining four case-table values (see ResourceServiceRequest's doc
	// comment) — same /cases/search endpoint as ResourceCase, distinguished
	// only by the auto-injected "type" filter (caseTableResourceTypes,
	// injectImpliedTypeFilters).
	ResourceServiceRequest: true, ResourceSecurityReportAnalysis: true,
	ResourceAnnouncement: true, ResourceEngagement: true,
	ResourceCaseFeedback: true,
}

var validWidgetShapes = map[Shape]bool{
	ShapeCount: true, ShapeList: true, ShapePie: true, ShapeBar: true,
}

// validGroupByBuckets are the legal values of GroupByConfig.Bucket -- the
// same enum POST /cases/feedback/aggregate documents on the entity-service
// side (see AggregateFeedbackRequest.bucket in openapi.yaml). Kept here
// rather than derived from that spec because this layer never calls that
// endpoint; it only validates widget config shape.
var validGroupByBuckets = map[string]bool{
	"day": true, "week": true, "month": true, "rating": true,
	"reasons_very_dissatisfied": true, "reasons_dissatisfied": true, "reasons_neutral": true,
	"reasons_satisfied": true, "reasons_very_satisfied": true,
}

// validateWidgets applies the loader's own fail-loud rationale one level down.
// Dashboard-level fields were already rejected by filename; a widget with a
// typo'd resourceType or shape, a blank or duplicated id, or a gridWidth
// outside 1-12 used to load perfectly happily and then misbehave silently in
// the browser -- an unknown shape renders nothing, a duplicate id collides as
// a React key and in the click-through URL, and gridWidth is interpolated
// straight into `grid-column: span N`, so 0 or 13 is broken layout rather
// than an error.
//
// This runs on the deprecated DASHBOARDS_CONFIG path too, unlike the "type"
// requirement: type is a genuinely new field that already-deployed values
// predate, whereas every field checked here has always been required for the
// widget to work at all.
func validateWidgets(d Dashboard, source string) error {
	seen := make(map[string]bool, len(d.Widgets))

	for i, w := range d.Widgets {
		// The widget id is the only handle an error further down has, so it is
		// checked first and reported positionally when it is missing.
		if strings.TrimSpace(w.ID) == "" {
			return fmt.Errorf("dashboard definitions: %s (id %q): widgets[%d]: \"id\" is empty", source, d.ID, i)
		}
		if seen[w.ID] {
			return fmt.Errorf("dashboard definitions: %s (id %q): duplicate widget id %q", source, d.ID, w.ID)
		}
		seen[w.ID] = true

		if strings.TrimSpace(w.DisplayName) == "" {
			return fmt.Errorf("dashboard definitions: %s (id %q): widget %q: \"displayName\" is empty", source, d.ID, w.ID)
		}
		if !validWidgetResourceTypes[w.ResourceType] {
			return fmt.Errorf("dashboard definitions: %s (id %q): widget %q: unknown \"resourceType\" %q", source, d.ID, w.ID, w.ResourceType)
		}
		if !validWidgetShapes[w.Shape] {
			return fmt.Errorf("dashboard definitions: %s (id %q): widget %q: unknown \"shape\" %q; expected one of %q, %q, %q, %q",
				source, d.ID, w.ID, w.Shape, ShapeCount, ShapeList, ShapePie, ShapeBar)
		}
		if w.GridWidth < 1 || w.GridWidth > 12 {
			return fmt.Errorf("dashboard definitions: %s (id %q): widget %q: \"gridWidth\" is %d; it is a column count out of 12 and must be between 1 and 12",
				source, d.ID, w.ID, w.GridWidth)
		}
		if (w.Shape == ShapePie || w.Shape == ShapeBar) && len(w.Slices) > 0 && w.GroupBy != nil {
			return fmt.Errorf("dashboard definitions: %s (id %q): widget %q: carries both \"slices\" and \"groupBy\"; a %q/%q widget must use exactly one",
				source, d.ID, w.ID, ShapePie, ShapeBar)
		}
		if (w.Shape == ShapePie || w.Shape == ShapeBar) && len(w.Slices) == 0 && w.GroupBy == nil {
			return fmt.Errorf("dashboard definitions: %s (id %q): widget %q: shape %q needs either \"slices\" or \"groupBy\"",
				source, d.ID, w.ID, w.Shape)
		}
		if w.GroupBy != nil {
			hasField := strings.TrimSpace(w.GroupBy.Field) != ""
			hasBucket := strings.TrimSpace(w.GroupBy.Bucket) != ""
			switch {
			case hasField && hasBucket:
				return fmt.Errorf("dashboard definitions: %s (id %q): widget %q: \"groupBy\" carries both \"field\" and \"bucket\"; a groupBy widget must use exactly one",
					source, d.ID, w.ID)
			case !hasField && !hasBucket:
				return fmt.Errorf("dashboard definitions: %s (id %q): widget %q: \"groupBy.field\" is empty",
					source, d.ID, w.ID)
			case hasBucket && !validGroupByBuckets[w.GroupBy.Bucket]:
				return fmt.Errorf("dashboard definitions: %s (id %q): widget %q: unknown \"groupBy.bucket\" %q; expected one of \"day\", \"week\", \"month\" (time buckets), \"rating\", \"reasons_very_dissatisfied\", \"reasons_dissatisfied\", \"reasons_neutral\", \"reasons_satisfied\", \"reasons_very_satisfied\" (categorical buckets)",
					source, d.ID, w.ID, w.GroupBy.Bucket)
			}
		}
	}

	return nil
}
