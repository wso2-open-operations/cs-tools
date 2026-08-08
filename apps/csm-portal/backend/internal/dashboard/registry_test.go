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
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// writeDefinition writes one dashboard definition file into dir and returns
// its path. name is the filename, which the loader must treat as meaningless.
func writeDefinition(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

const csDefinition = `{
  "id": "cs-overview",
  "displayName": "CS Overview",
  "type": "cs",
  "isDefault": true,
  "widgets": [
    {"id": "open-cases", "displayName": "Open Cases", "resourceType": "case", "shape": "count", "gridWidth": 3,
     "query": {"filters": [{"field": "state", "op": "in", "values": ["open"]}]}}
  ]
}`

const creDefinition = `{
  "id": "cre-team",
  "displayName": "CRE Team",
  "type": "cre",
  "isTeamBased": true,
  "targetTeam": "abt",
  "widgets": [
    {"id": "team-cases", "displayName": "Team Cases", "resourceType": "case", "shape": "count", "gridWidth": 3,
     "query": {"filters": [{"field": "state", "op": "in", "values": ["open"]}]}},
    {"id": "team-p1", "displayName": "Team P1", "resourceType": "case", "shape": "count", "gridWidth": 3,
     "query": {"filters": [{"field": "severity", "op": "in", "values": ["critical"]}]}}
  ]
}`

func TestLoadDir_HappyPath(t *testing.T) {
	dir := t.TempDir()
	// Deliberately unhelpful filenames: the loader must take id, displayName
	// and type from the content, never from the name, and must order by
	// filename so the result is deterministic.
	writeDefinition(t, dir, "02-second.json", creDefinition)
	writeDefinition(t, dir, "01-first.json", csDefinition)
	// Non-JSON siblings are ignored rather than erroring: the directory is
	// hand-maintained and will collect READMEs and editor droppings.
	writeDefinition(t, dir, "README.md", "not a dashboard")
	writeDefinition(t, dir, "notes.txt", "also not a dashboard")
	if err := os.Mkdir(filepath.Join(dir, "archive"), 0o750); err != nil {
		t.Fatalf("mkdir archive: %v", err)
	}

	got, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("LoadDir returned %d dashboards, want 2: %+v", len(got), got)
	}
	if got[0].ID != "cs-overview" || got[1].ID != "cre-team" {
		t.Fatalf("dashboard order = %q, %q; want cs-overview then cre-team (lexical filename order)", got[0].ID, got[1].ID)
	}
	if got[0].Type != TypeCS || !got[0].IsDefault || got[0].IsTeamBased {
		t.Fatalf("cs-overview = %+v; want type cs, isDefault, not team based", got[0])
	}
	if got[1].Type != TypeCRE || !got[1].IsTeamBased || got[1].TargetTeam != "abt" {
		t.Fatalf("cre-team = %+v; want type cre, team based, targetTeam abt", got[1])
	}
	if len(got[1].Widgets) != 2 {
		t.Fatalf("cre-team has %d widgets, want 2", len(got[1].Widgets))
	}
}

func TestLoadDir_EmptyDirectory(t *testing.T) {
	got, err := LoadDir(t.TempDir())
	if err != nil {
		t.Fatalf("LoadDir(empty dir) returned error: %v; an empty directory is legal", err)
	}
	if len(got) != 0 {
		t.Fatalf("LoadDir(empty dir) = %+v, want no dashboards", got)
	}
}

func TestLoadDir_MissingDirectoryIsAnError(t *testing.T) {
	_, err := LoadDir(filepath.Join(t.TempDir(), "does-not-exist"))
	if err == nil {
		t.Fatal("LoadDir(missing dir) returned no error; a misconfigured path must fail the deploy")
	}
	if !strings.Contains(err.Error(), "does-not-exist") {
		t.Fatalf("error %q does not name the offending directory", err)
	}
}

// TestLoadDir_MalformedFileFailsNamingIt is the central guarantee: a broken
// definition must never be skipped, because a skipped dashboard is invisible.
func TestLoadDir_MalformedFileFailsNamingIt(t *testing.T) {
	dir := t.TempDir()
	writeDefinition(t, dir, "good.json", csDefinition)
	writeDefinition(t, dir, "broken.json", `{"id": "broken", "displayName":`)

	got, err := LoadDir(dir)
	if err == nil {
		t.Fatalf("LoadDir returned no error for a malformed file; got %+v", got)
	}
	if !strings.Contains(err.Error(), "broken.json") {
		t.Fatalf("error %q does not name the offending file", err)
	}
	if got != nil {
		t.Fatalf("LoadDir returned %+v alongside the error; a partial load must not be served", got)
	}
}

func TestLoadDir_UnreadableFileFailsNamingIt(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: file permissions do not deny reads")
	}
	dir := t.TempDir()
	path := writeDefinition(t, dir, "locked.json", csDefinition)
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })

	_, err := LoadDir(dir)
	if err == nil {
		t.Fatal("LoadDir returned no error for an unreadable file")
	}
	if !strings.Contains(err.Error(), "locked.json") {
		t.Fatalf("error %q does not name the offending file", err)
	}
}

func TestLoadDir_DuplicateIDFailsNamingBothFiles(t *testing.T) {
	dir := t.TempDir()
	writeDefinition(t, dir, "a.json", csDefinition)
	writeDefinition(t, dir, "b.json", csDefinition)

	_, err := LoadDir(dir)
	if err == nil {
		t.Fatal("LoadDir returned no error for two files sharing one dashboard id")
	}
	for _, want := range []string{"a.json", "b.json", "cs-overview"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q does not mention %q", err, want)
		}
	}
}

func TestLoadDir_MissingIDFails(t *testing.T) {
	dir := t.TempDir()
	writeDefinition(t, dir, "anonymous.json", `{"displayName": "No Id", "type": "cs", "widgets": []}`)

	_, err := LoadDir(dir)
	if err == nil {
		t.Fatal("LoadDir returned no error for a definition with no id")
	}
	if !strings.Contains(err.Error(), "anonymous.json") {
		t.Fatalf("error %q does not name the offending file", err)
	}
}

func TestLoadDir_MissingDisplayNameFails(t *testing.T) {
	dir := t.TempDir()
	writeDefinition(t, dir, "nameless.json", `{"id": "nameless", "type": "cs", "widgets": []}`)

	_, err := LoadDir(dir)
	if err == nil {
		t.Fatal("LoadDir returned no error for a definition with no displayName")
	}
	if !strings.Contains(err.Error(), "nameless.json") {
		t.Fatalf("error %q does not name the offending file", err)
	}
}

func TestLoadDir_TypeIsRequiredAndClosed(t *testing.T) {
	t.Run("missing type", func(t *testing.T) {
		dir := t.TempDir()
		writeDefinition(t, dir, "untyped.json", `{"id": "untyped", "displayName": "Untyped", "widgets": []}`)
		_, err := LoadDir(dir)
		if err == nil || !strings.Contains(err.Error(), "untyped.json") {
			t.Fatalf("LoadDir error = %v; want a rejection naming untyped.json", err)
		}
	})

	t.Run("unknown type", func(t *testing.T) {
		dir := t.TempDir()
		writeDefinition(t, dir, "weird.json", `{"id": "weird", "displayName": "Weird", "type": "ops", "widgets": []}`)
		_, err := LoadDir(dir)
		if err == nil {
			t.Fatal("LoadDir returned no error for an unknown dashboard type")
		}
		for _, want := range []string{"weird.json", "ops"} {
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("error %q does not mention %q", err, want)
			}
		}
	})

	t.Run("every valid type is accepted", func(t *testing.T) {
		for _, tc := range []struct {
			typ         Type
			isTeamBased bool
		}{
			{TypeCRE, true},
			{TypeSRE, true},
			{TypeCS, false},
		} {
			dir := t.TempDir()
			writeDefinition(t, dir, "d.json", `{"id": "d", "displayName": "D", "type": "`+string(tc.typ)+
				`", "isTeamBased": `+boolLiteral(tc.isTeamBased)+`, "widgets": []}`)
			got, err := LoadDir(dir)
			if err != nil {
				t.Fatalf("LoadDir(type %q) returned error: %v", tc.typ, err)
			}
			if len(got) != 1 || got[0].Type != tc.typ {
				t.Fatalf("LoadDir(type %q) = %+v", tc.typ, got)
			}
		}
	})
}

func boolLiteral(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// TestLoadDir_ContradictoryCombinations pins down exactly which
// type/isDefault/isTeamBased combinations are rejected. The three fields are
// independent by product decision, which is precisely why they need this.
func TestLoadDir_ContradictoryCombinations(t *testing.T) {
	cases := []struct {
		name    string
		files   map[string]string
		wantErr []string
	}{
		{
			name: "team-scoped cre type with isTeamBased false",
			files: map[string]string{
				"a.json": `{"id": "a", "displayName": "A", "type": "cre", "isTeamBased": false, "widgets": []}`,
			},
			wantErr: []string{"a.json", "cre", "isTeamBased"},
		},
		{
			name: "team-scoped sre type with isTeamBased omitted (defaults false)",
			files: map[string]string{
				"a.json": `{"id": "a", "displayName": "A", "type": "sre", "widgets": []}`,
			},
			wantErr: []string{"a.json", "sre", "isTeamBased"},
		},
		{
			name: "organisation-wide cs type with isTeamBased true",
			files: map[string]string{
				"a.json": `{"id": "a", "displayName": "A", "type": "cs", "isTeamBased": true, "widgets": []}`,
			},
			wantErr: []string{"a.json", "cs", "isTeamBased"},
		},
		{
			name: "two defaults of the same type",
			files: map[string]string{
				"a.json": `{"id": "a", "displayName": "A", "type": "cs", "isDefault": true, "widgets": []}`,
				"b.json": `{"id": "b", "displayName": "B", "type": "cs", "isDefault": true, "widgets": []}`,
			},
			wantErr: []string{"a.json", "b.json", "isDefault"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, body := range tc.files {
				writeDefinition(t, dir, name, body)
			}
			_, err := LoadDir(dir)
			if err == nil {
				t.Fatal("LoadDir returned no error, want a rejection")
			}
			for _, want := range tc.wantErr {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("error %q does not mention %q", err, want)
				}
			}
		})
	}
}

// Two defaults of DIFFERENT types are rejected too, for now. One default per
// type is where this ends up, but only once the frontend selects on "type" --
// today CsmDashboardPage picks on isDefault + isTeamBased and the
// dashboard-list response does not even carry "type", so a second typed
// default would be resolved by nothing but LoadDir's filename ordering.
// Rejecting is the conservative half of that pair. When the frontend becomes
// type-aware, this test flips to asserting they coexist.
func TestLoadDir_RejectsASecondDefaultEvenOfADifferentType(t *testing.T) {
	dir := t.TempDir()
	writeDefinition(t, dir, "a.json", `{"id": "a", "displayName": "A", "type": "cs", "isDefault": true, "widgets": []}`)
	writeDefinition(t, dir, "b.json", `{"id": "b", "displayName": "B", "type": "cre", "isDefault": true, "isTeamBased": true, "widgets": []}`)

	_, err := LoadDir(dir)
	if err == nil {
		t.Fatal("LoadDir accepted two isDefault dashboards of different types; expected an error")
	}
	for _, want := range []string{"a.json", "b.json", "isDefault"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %q, want it to contain %q", err.Error(), want)
		}
	}
}

// A single default alongside non-default dashboards of other types is of
// course still fine -- the rule is "at most one isDefault", not "at most one
// dashboard per type".
func TestLoadDir_OneDefaultAlongsideOtherTypes(t *testing.T) {
	dir := t.TempDir()
	writeDefinition(t, dir, "a.json", `{"id": "a", "displayName": "A", "type": "cs", "isDefault": true, "widgets": []}`)
	writeDefinition(t, dir, "b.json", `{"id": "b", "displayName": "B", "type": "cre", "isTeamBased": true, "widgets": []}`)
	writeDefinition(t, dir, "c.json", `{"id": "c", "displayName": "C", "type": "sre", "isTeamBased": true, "widgets": []}`)

	got, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir returned error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("LoadDir returned %d dashboards, want 3", len(got))
	}
}

// TestLoadDir_MigratesLegacyWidgetKeys proves the deprecated-key migration
// runs on directory-loaded definitions too, not just on DASHBOARDS_CONFIG.
func TestLoadDir_MigratesLegacyWidgetKeys(t *testing.T) {
	dir := t.TempDir()
	writeDefinition(t, dir, "legacy.json", `{
	  "id": "legacy", "displayName": "Legacy", "type": "cs",
	  "widgets": [
	    {"id": "w", "displayName": "W", "resourceType": "case", "shape": "count", "gridWidth": 3,
	     "filters": {"orGroups": [[{"field": "state", "op": "in", "values": ["open"]}]]}}
	  ]
	}`)

	got, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir returned error: %v", err)
	}
	query := got[0].Widgets[0].Query
	if query == nil {
		t.Fatal("widget Query is nil; the legacy \"filters\" key was not migrated")
	}
	if _, ok := query["orGroups"]; ok {
		t.Fatalf("widget Query still carries \"orGroups\": %+v", query)
	}
	branches, ok := query["anyOf"].([]any)
	if !ok || len(branches) != 1 {
		t.Fatalf("widget Query anyOf = %+v, want one migrated branch", query["anyOf"])
	}
	if _, ok := branches[0].(map[string]any)["filters"]; !ok {
		t.Fatalf("migrated branch = %+v, want it wrapped as {\"filters\": [...]}", branches[0])
	}
}

// TestLoadDir_MigrationWarningsNameTheFile: both loaders run the migration, so
// a deprecation warning must not name DASHBOARDS_CONFIG at an operator whose
// deployment uses DASHBOARDS_DIR, and must carry the file they have to edit.
func TestLoadDir_MigrationWarningsNameTheFile(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	dir := t.TempDir()
	writeDefinition(t, dir, "legacy.json", `{
	  "id": "legacy", "displayName": "Legacy", "type": "cs",
	  "widgets": [
	    {"id": "w", "displayName": "W", "resourceType": "case", "shape": "count", "gridWidth": 3,
	     "filters": {"orGroups": [[{"field": "state", "op": "in", "values": ["open"]}]]}}
	  ]
	}`)

	if _, err := LoadDir(dir); err != nil {
		t.Fatalf("LoadDir returned error: %v", err)
	}

	logged := buf.String()
	if strings.Contains(logged, "DASHBOARDS_CONFIG") {
		t.Errorf("migration warnings name DASHBOARDS_CONFIG on the directory path:\n%s", logged)
	}
	if !strings.Contains(logged, filepath.Join(dir, "legacy.json")) {
		t.Errorf("migration warnings do not name the offending file:\n%s", logged)
	}
	for _, want := range []string{`widget key \"filters\" is deprecated`, `criteria key \"orGroups\" is deprecated`} {
		if !strings.Contains(logged, want) {
			t.Errorf("migration warnings do not contain %q:\n%s", want, logged)
		}
	}
}

// TestRegistry_DefaultModeReadsDiskExactlyOnce is the whole point of the
// default mode: the startup read is the only read, no matter how many
// requests come in.
func TestRegistry_DefaultModeReadsDiskExactlyOnce(t *testing.T) {
	dir := t.TempDir()
	writeDefinition(t, dir, "a.json", csDefinition)

	var reads atomic.Int64
	r, err := NewDirRegistry(dir, false, "")
	if err != nil {
		t.Fatalf("NewDirRegistry returned error: %v", err)
	}
	// Swap in a counting loader AFTER construction, so the count covers only
	// post-startup reads. It must stay at zero.
	r.load = func(d string) ([]Dashboard, error) {
		reads.Add(1)
		return LoadDir(d)
	}

	for i := 0; i < 5; i++ {
		if got := r.Dashboards(); len(got) != 1 {
			t.Fatalf("read %d: got %d dashboards, want 1", i, len(got))
		}
		if _, ok := r.ByID("cs-overview"); !ok {
			t.Fatalf("read %d: cs-overview not found", i)
		}
	}
	if n := reads.Load(); n != 0 {
		t.Fatalf("the registry touched the disk %d times after startup; the default mode must read exactly once", n)
	}

	// The strongest form of the same claim: with the directory gone entirely,
	// the in-memory copy still serves.
	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("remove dir: %v", err)
	}
	if got := r.Dashboards(); len(got) != 1 {
		t.Fatalf("after deleting the directory, got %d dashboards, want the 1 held in memory", len(got))
	}
}

// TestRegistry_HotReloadPicksUpChanges covers the local-development mode:
// editing a definition after startup is visible without a restart.
func TestRegistry_HotReloadPicksUpChanges(t *testing.T) {
	dir := t.TempDir()
	writeDefinition(t, dir, "a.json", csDefinition)

	r, err := NewDirRegistry(dir, true, "")
	if err != nil {
		t.Fatalf("NewDirRegistry returned error: %v", err)
	}
	if got := r.Dashboards(); len(got) != 1 || got[0].DisplayName != "CS Overview" {
		t.Fatalf("initial read = %+v", got)
	}

	// Edit an existing definition.
	writeDefinition(t, dir, "a.json", strings.Replace(csDefinition, "CS Overview", "CS Overview v2", 1))
	got := r.Dashboards()
	if len(got) != 1 || got[0].DisplayName != "CS Overview v2" {
		t.Fatalf("after editing the file, read = %+v; want the new displayName", got)
	}

	// Add a whole new definition file.
	writeDefinition(t, dir, "b.json", creDefinition)
	if got := r.Dashboards(); len(got) != 2 {
		t.Fatalf("after adding a file, got %d dashboards, want 2", len(got))
	}

	// Remove one again.
	if err := os.Remove(filepath.Join(dir, "b.json")); err != nil {
		t.Fatalf("remove b.json: %v", err)
	}
	if got := r.Dashboards(); len(got) != 1 {
		t.Fatalf("after removing a file, got %d dashboards, want 1", len(got))
	}
}

// TestRegistry_HotReloadKeepsLastKnownGoodOnError is the deliberate asymmetry
// with the startup path. Startup fails hard on a bad definition set; a
// running dev server does not, because the overwhelmingly common cause is an
// editor writing a half-finished JSON file mid-keystroke. It logs loudly and
// keeps serving what last parsed, and recovers by itself once the file is
// valid again.
func TestRegistry_HotReloadKeepsLastKnownGoodOnError(t *testing.T) {
	dir := t.TempDir()
	writeDefinition(t, dir, "a.json", csDefinition)

	r, err := NewDirRegistry(dir, true, "")
	if err != nil {
		t.Fatalf("NewDirRegistry returned error: %v", err)
	}

	writeDefinition(t, dir, "a.json", `{"id": "cs-overview", "displayName":`)
	got := r.Dashboards()
	if len(got) != 1 || got[0].DisplayName != "CS Overview" {
		t.Fatalf("with a mid-save file on disk, read = %+v; want the last known-good definitions", got)
	}

	// A contradictory (but well-formed) edit is held back the same way.
	writeDefinition(t, dir, "a.json", `{"id": "cs-overview", "displayName": "CS", "type": "cs", "isTeamBased": true, "widgets": []}`)
	if got := r.Dashboards(); len(got) != 1 || got[0].DisplayName != "CS Overview" {
		t.Fatalf("with a contradictory file on disk, read = %+v; want the last known-good definitions", got)
	}

	// And it recovers on its own once the file parses again.
	writeDefinition(t, dir, "a.json", strings.Replace(csDefinition, "CS Overview", "CS Overview fixed", 1))
	if got := r.Dashboards(); len(got) != 1 || got[0].DisplayName != "CS Overview fixed" {
		t.Fatalf("after the file was fixed, read = %+v; want the repaired definitions", got)
	}
}

// TestNewDirRegistry_FailsAtStartupInBothModes: hot-reload must not soften
// the startup contract. A broken definition set is a broken deploy either
// way.
func TestNewDirRegistry_FailsAtStartupInBothModes(t *testing.T) {
	for _, hotReload := range []bool{false, true} {
		dir := t.TempDir()
		writeDefinition(t, dir, "broken.json", `{"id": "broken", "displayName":`)
		if _, err := NewDirRegistry(dir, hotReload, ""); err == nil {
			t.Fatalf("NewDirRegistry(hotReload=%v) returned no error for a malformed definition", hotReload)
		}
	}
}

// TestNilRegistry_ServesNothing guards the pre-startup / unconfigured case:
// the handlers must degrade to an empty list rather than panicking.
func TestNilRegistry_ServesNothing(t *testing.T) {
	var r *Registry
	if got := r.Dashboards(); got != nil {
		t.Fatalf("(*Registry)(nil).Dashboards() = %+v, want nil", got)
	}
	if _, ok := r.ByID("anything"); ok {
		t.Fatal("(*Registry)(nil).ByID returned ok=true")
	}
}

// TestParseDashboardsConfig_ToleratesMissingType: the deprecated single-
// variable path predates the type field entirely, so requiring one there
// would break every already-deployed value. It warns instead. The
// contradiction rules still apply once a type IS set.
func TestParseDashboardsConfig_ToleratesMissingType(t *testing.T) {
	got, err := ParseDashboardsConfig(`[{"id":"a","displayName":"A","widgets":[]}]`)
	if err != nil {
		t.Fatalf("ParseDashboardsConfig returned error: %v", err)
	}
	if len(got) != 1 || got[0].Type != "" {
		t.Fatalf("ParseDashboardsConfig = %+v, want one dashboard with no type", got)
	}
}

// TestParseDashboardsConfig_RejectsTwoUntypedDefaults covers the one path that
// can produce untyped definitions at all: the deprecated single-variable one,
// which tolerates a missing type. Tolerating it must not also exempt those
// definitions from the one-default rule -- two untyped isDefault dashboards
// make automatic selection depend on the order they happen to appear in the
// variable, which is exactly what the rule exists to prevent.
func TestParseDashboardsConfig_RejectsTwoUntypedDefaults(t *testing.T) {
	_, err := ParseDashboardsConfig(`[
		{"id":"a","displayName":"A","isDefault":true,"widgets":[]},
		{"id":"b","displayName":"B","isDefault":true,"widgets":[]}
	]`)
	if err == nil {
		t.Fatal("ParseDashboardsConfig returned no error, want a rejection")
	}
	for _, want := range []string{"isDefault", `id "b"`} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q does not mention %q", err, want)
		}
	}
}

// TestParseDashboardsConfig_OneUntypedDefaultIsFine is the counterpart: a
// single untyped default is the already-deployed arrangement the deprecated
// path exists to keep working, and must not be swept up by the rule above.
func TestParseDashboardsConfig_OneUntypedDefaultIsFine(t *testing.T) {
	got, err := ParseDashboardsConfig(`[
		{"id":"a","displayName":"A","isDefault":true,"widgets":[]},
		{"id":"b","displayName":"B","widgets":[]}
	]`)
	if err != nil {
		t.Fatalf("ParseDashboardsConfig returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ParseDashboardsConfig returned %d dashboards, want 2", len(got))
	}
}

func TestParseDashboardsConfig_RejectsContradictions(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"cre type not team based", `[{"id":"a","displayName":"A","type":"cre","isTeamBased":false,"widgets":[]}]`},
		{"cs type team based", `[{"id":"a","displayName":"A","type":"cs","isTeamBased":true,"widgets":[]}]`},
		{"unknown type", `[{"id":"a","displayName":"A","type":"ops","widgets":[]}]`},
		{"duplicate id", `[{"id":"a","displayName":"A","widgets":[]},{"id":"a","displayName":"B","widgets":[]}]`},
		{"empty id", `[{"id":"","displayName":"A","widgets":[]}]`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseDashboardsConfig(tc.raw); err == nil {
				t.Fatal("ParseDashboardsConfig returned no error, want a rejection")
			}
		})
	}
}

// TestLoadDir_BothQueryAndLegacyFiltersWarns: when a definition carries BOTH
// the current "query" key and the deprecated "filters" key, the new one wins
// and the legacy one is dropped. That drop has to be logged for the same
// reason the sibling orGroups/anyOf drop is ("silent data loss otherwise"):
// the operator wrote a key this loader recognises and then never sees it
// again. The empty-"query" case is the one that actually bites -- an empty
// {} still wins over a populated "filters", so every widget renders 0 with
// nothing anywhere saying why.
func TestLoadDir_BothQueryAndLegacyFiltersWarns(t *testing.T) {
	cases := []struct {
		name string
		json string
	}{
		{
			name: "populated query alongside a legacy filters key",
			json: `{
			  "id": "both", "displayName": "Both", "type": "cs",
			  "widgets": [
			    {"id": "w", "displayName": "W", "resourceType": "case", "shape": "count", "gridWidth": 3,
			     "query": {"filters": [{"field": "state", "op": "in", "values": ["closed"]}]},
			     "filters": {"filters": [{"field": "state", "op": "in", "values": ["open"]}]}}
			  ]
			}`,
		},
		{
			name: "EMPTY query silently beating a populated legacy filters key",
			json: `{
			  "id": "both", "displayName": "Both", "type": "cs",
			  "widgets": [
			    {"id": "w", "displayName": "W", "resourceType": "case", "shape": "count", "gridWidth": 3,
			     "query": {},
			     "filters": {"filters": [{"field": "state", "op": "in", "values": ["open"]}]}}
			  ]
			}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			prev := slog.Default()
			slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))
			t.Cleanup(func() { slog.SetDefault(prev) })

			dir := t.TempDir()
			writeDefinition(t, dir, "both.json", tc.json)

			if _, err := LoadDir(dir); err != nil {
				t.Fatalf("LoadDir returned error: %v", err)
			}

			logged := buf.String()
			if !strings.Contains(logged, `deprecated widget key \"filters\" dropped`) {
				t.Errorf("dropping the legacy \"filters\" key logged no warning:\n%s", logged)
			}
			if !strings.Contains(logged, filepath.Join(dir, "both.json")) {
				t.Errorf("the warning does not name the offending file:\n%s", logged)
			}
		})
	}
}

// The same drop, one level down: a pie slice carrying both keys.
func TestLoadDir_SliceBothQueryAndLegacyFiltersWarns(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	dir := t.TempDir()
	writeDefinition(t, dir, "slice.json", `{
	  "id": "slice", "displayName": "Slice", "type": "cs",
	  "widgets": [
	    {"id": "w", "displayName": "W", "resourceType": "case", "shape": "pie", "gridWidth": 4,
	     "query": {"filters": [{"field": "state", "op": "in", "values": ["open"]}]},
	     "slices": [
	       {"label": "Critical",
	        "query": {},
	        "filters": {"filters": [{"field": "severity", "op": "in", "values": ["critical"]}]}}
	     ]}
	  ]
	}`)

	if _, err := LoadDir(dir); err != nil {
		t.Fatalf("LoadDir returned error: %v", err)
	}

	logged := buf.String()
	if !strings.Contains(logged, `deprecated slice key \"filters\" dropped`) {
		t.Errorf("dropping a slice's legacy \"filters\" key logged no warning:\n%s", logged)
	}
}

// Widget fields get the same fail-loud treatment the dashboard's own fields
// get. Every one of these used to load successfully and then misbehave only
// in the browser: an unknown shape renders nothing, a duplicate id collides
// as a React key and in the click-through URL, and gridWidth is interpolated
// straight into `grid-column: span N`.
func TestLoadDir_RejectsInvalidWidgets(t *testing.T) {
	const widget = `{"id": "w", "displayName": "W", "resourceType": "case", "shape": "count", "gridWidth": 3}`

	cases := []struct {
		name    string
		widgets string
		want    string
	}{
		{
			name:    "empty widget id",
			widgets: `{"id": "", "displayName": "W", "resourceType": "case", "shape": "count", "gridWidth": 3}`,
			want:    `widgets[0]: "id" is empty`,
		},
		{
			name:    "duplicate widget id",
			widgets: widget + "," + widget,
			want:    `duplicate widget id "w"`,
		},
		{
			name:    "empty widget displayName",
			widgets: `{"id": "w", "displayName": "", "resourceType": "case", "shape": "count", "gridWidth": 3}`,
			want:    `widget "w": "displayName" is empty`,
		},
		{
			name:    "typo'd resourceType",
			widgets: `{"id": "w", "displayName": "W", "resourceType": "cases", "shape": "count", "gridWidth": 3}`,
			want:    `widget "w": unknown "resourceType" "cases"`,
		},
		{
			name:    "missing resourceType",
			widgets: `{"id": "w", "displayName": "W", "shape": "count", "gridWidth": 3}`,
			want:    `widget "w": unknown "resourceType" ""`,
		},
		{
			name:    "typo'd shape",
			widgets: `{"id": "w", "displayName": "W", "resourceType": "case", "shape": "counter", "gridWidth": 3}`,
			want:    `widget "w": unknown "shape" "counter"`,
		},
		{
			name:    "gridWidth omitted entirely (zero)",
			widgets: `{"id": "w", "displayName": "W", "resourceType": "case", "shape": "count"}`,
			want:    `widget "w": "gridWidth" is 0`,
		},
		{
			name:    "gridWidth above the 12-column grid",
			widgets: `{"id": "w", "displayName": "W", "resourceType": "case", "shape": "count", "gridWidth": 13}`,
			want:    `widget "w": "gridWidth" is 13`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeDefinition(t, dir, "d.json", `{
			  "id": "d", "displayName": "D", "type": "cs",
			  "widgets": [`+tc.widgets+`]
			}`)

			_, err := LoadDir(dir)
			if err == nil {
				t.Fatal("LoadDir accepted an invalid widget; expected an error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %q, want it to contain %q", err.Error(), tc.want)
			}
			if !strings.Contains(err.Error(), filepath.Join(dir, "d.json")) {
				t.Errorf("err = %q, want it to name the offending file", err.Error())
			}
		})
	}
}

// The deprecated single-variable path gets the same widget validation: unlike
// "type", none of these fields is new, so an already-deployed value carrying
// one is already broken.
func TestParseDashboardsConfig_RejectsInvalidWidgets(t *testing.T) {
	_, err := ParseDashboardsConfig(`[{"id":"d","displayName":"D","widgets":[
	  {"id":"w","displayName":"W","resourceType":"case","shape":"counter","gridWidth":3}
	]}]`)
	if err == nil {
		t.Fatal("ParseDashboardsConfig accepted an unknown shape; expected an error")
	}
	if !strings.Contains(err.Error(), `unknown "shape" "counter"`) {
		t.Errorf("err = %q, want it to name the unknown shape", err.Error())
	}
	if !strings.Contains(err.Error(), "DASHBOARDS_CONFIG[0]") {
		t.Errorf("err = %q, want it to name the offending config index", err.Error())
	}
}

// The committed dashboards.example/ directory is what .env.example's
// DASHBOARDS_DIR points at, so `cp .env.example .env && go run ./cmd/server`
// works on a fresh clone (./dashboards is gitignored and a missing directory
// is fatal). That only holds while the example set actually validates, and
// every validation rule added here can silently invalidate it -- so load it
// for real rather than trusting that it still parses.
func TestLoadDir_ShippedExampleDirectoryIsValid(t *testing.T) {
	const dir = "../../dashboards.example"

	got, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("the committed %s does not load, so a fresh clone cannot start: %v", dir, err)
	}
	if len(got) == 0 {
		t.Fatalf("%s loaded no dashboards", dir)
	}
}
