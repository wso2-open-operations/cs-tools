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

// Two defaults of DIFFERENT types coexist: the frontend selects its landing
// dashboard from the caller's own team family against "type", so a cre
// default and an sre default (or a cs default) do not compete for one
// global slot -- each type gets its own.
func TestLoadDir_AllowsDefaultsOfDifferentTypesToCoexist(t *testing.T) {
	dir := t.TempDir()
	writeDefinition(t, dir, "a.json", `{"id": "a", "displayName": "A", "type": "cs", "isDefault": true, "widgets": []}`)
	writeDefinition(t, dir, "b.json", `{"id": "b", "displayName": "B", "type": "cre", "isDefault": true, "isTeamBased": true, "widgets": []}`)

	got, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir rejected two isDefault dashboards of different types: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("LoadDir returned %d dashboards, want 2", len(got))
	}
	if !got[0].IsDefault || got[0].Type != TypeCS {
		t.Fatalf("a = %+v; want isDefault true, type cs", got[0])
	}
	if !got[1].IsDefault || got[1].Type != TypeCRE {
		t.Fatalf("b = %+v; want isDefault true, type cre", got[1])
	}
}

// A second isDefault dashboard of the SAME type is still rejected, exactly as
// before -- the one-per-type rule is not "no rule at all".
func TestLoadDir_RejectsASecondDefaultOfTheSameType(t *testing.T) {
	dir := t.TempDir()
	writeDefinition(t, dir, "a.json", `{"id": "a", "displayName": "A", "type": "cre", "isDefault": true, "isTeamBased": true, "widgets": []}`)
	writeDefinition(t, dir, "b.json", `{"id": "b", "displayName": "B", "type": "cre", "isDefault": true, "isTeamBased": true, "widgets": []}`)

	_, err := LoadDir(dir)
	if err == nil {
		t.Fatal("LoadDir accepted two isDefault dashboards of the same type; expected an error")
	}
	for _, want := range []string{"a.json", "b.json", "isDefault", "cre"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %q, want it to contain %q", err.Error(), want)
		}
	}
}

// An untyped isDefault dashboard (only reachable via the deprecated
// DASHBOARDS_CONFIG path) does not share a "slot" with a typed isDefault
// dashboard -- it has no type to key off, so it must not collide with one.
func TestParseDashboardsConfig_UntypedDefaultDoesNotCollideWithTypedDefault(t *testing.T) {
	got, err := ParseDashboardsConfig(`[
		{"id":"a","displayName":"A","isDefault":true,"widgets":[]},
		{"id":"b","displayName":"B","type":"cs","isDefault":true,"widgets":[]}
	]`)
	if err != nil {
		t.Fatalf("ParseDashboardsConfig rejected an untyped default alongside a typed one: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ParseDashboardsConfig returned %d dashboards, want 2", len(got))
	}
	if !got[0].IsDefault || got[0].Type != "" {
		t.Fatalf("a = %+v; want isDefault true, no type", got[0])
	}
	if !got[1].IsDefault || got[1].Type != TypeCS {
		t.Fatalf("b = %+v; want isDefault true, type cs", got[1])
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
	r, err := NewDirRegistry(dir, false, "", "")
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

	r, err := NewDirRegistry(dir, true, "", "")
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

	r, err := NewDirRegistry(dir, true, "", "")
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
		if _, err := NewDirRegistry(dir, hotReload, "", ""); err == nil {
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
		{
			name: "pie widget with neither slices nor groupBy",
			widgets: `{"id": "w", "displayName": "W", "resourceType": "case", "shape": "pie", "gridWidth": 3,
			 "query": {}}`,
			want: `widget "w": shape "pie" needs either "slices" or "groupBy"`,
		},
		{
			name: "bar widget with both slices and groupBy",
			widgets: `{"id": "w", "displayName": "W", "resourceType": "case", "shape": "bar", "gridWidth": 3,
			 "query": {}, "slices": [{"label": "A", "query": {}}], "groupBy": {"field": "account"}}`,
			want: `widget "w": carries both "slices" and "groupBy"`,
		},
		{
			name: "groupBy with an empty field",
			widgets: `{"id": "w", "displayName": "W", "resourceType": "case", "shape": "pie", "gridWidth": 3,
			 "query": {}, "groupBy": {"field": ""}}`,
			want: `widget "w": "groupBy.field" is empty`,
		},
		{
			name: "bar widget groupBy with both field and bucket",
			widgets: `{"id": "w", "displayName": "W", "resourceType": "case_feedback", "shape": "bar", "gridWidth": 3,
			 "query": {}, "groupBy": {"field": "account", "bucket": "day"}}`,
			want: `widget "w": "groupBy" carries both "field" and "bucket"`,
		},
		{
			name: "bar widget groupBy with an unknown bucket",
			widgets: `{"id": "w", "displayName": "W", "resourceType": "case_feedback", "shape": "bar", "gridWidth": 3,
			 "query": {}, "groupBy": {"bucket": "quarter"}}`,
			want: `widget "w": unknown "groupBy.bucket" "quarter"`,
		},
		{
			name: "bar widget with neither slices nor groupBy",
			widgets: `{"id": "w", "displayName": "W", "resourceType": "case_feedback", "shape": "bar", "gridWidth": 3,
			 "query": {}}`,
			want: `widget "w": shape "bar" needs either "slices" or "groupBy"`,
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

// A "bar" widget grouped by date bucket (case-feedback trend) loads clean,
// same as a field-grouped pie/bar widget always has -- the new
// GroupByConfig.Bucket branch is additive, not a narrowing of what already
// loaded.
func TestLoadDir_AcceptsBarWidgetWithBucketGroupBy(t *testing.T) {
	dir := t.TempDir()
	writeDefinition(t, dir, "d.json", `{
	  "id": "d", "displayName": "D", "type": "cs",
	  "widgets": [
	    {"id": "feedback-trend", "displayName": "Feedback Trend", "resourceType": "case_feedback", "shape": "bar", "gridWidth": 6,
	     "query": {}, "groupBy": {"bucket": "week"}}
	  ]
	}`)

	dashboards, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir returned error: %v", err)
	}
	if len(dashboards) != 1 || len(dashboards[0].Widgets) != 1 {
		t.Fatalf("unexpected loaded dashboards: %+v", dashboards)
	}
	w := dashboards[0].Widgets[0]
	if w.ResourceType != ResourceCaseFeedback {
		t.Errorf("resourceType = %q, want %q", w.ResourceType, ResourceCaseFeedback)
	}
	if w.Shape != ShapeBar {
		t.Errorf("shape = %q, want %q", w.Shape, ShapeBar)
	}
	if w.GroupBy == nil || w.GroupBy.Bucket != "day" && w.GroupBy.Bucket != "week" && w.GroupBy.Bucket != "month" {
		t.Fatalf("groupBy = %+v, want a valid bucket", w.GroupBy)
	}
	if w.GroupBy.Field != "" {
		t.Errorf("groupBy.field = %q, want empty for a bucket-mode groupBy", w.GroupBy.Field)
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

// A dashboard may claim a defaultForTeamKeys entry without any conflict --
// the common case, and the one every real deployment relies on to land a
// team's members on their own specialist dashboard.
func TestLoadDir_DefaultForTeamKeysWithNoConflictIsFine(t *testing.T) {
	dir := t.TempDir()
	writeDefinition(t, dir, "a.json", `{"id": "onboarding-engineer", "displayName": "Onboarding Engineer", "type": "cs", "defaultForTeamKeys": ["customer_onboarding"], "widgets": []}`)
	writeDefinition(t, dir, "b.json", `{"id": "migration-engineer", "displayName": "Migration Engineer", "type": "cs", "defaultForTeamKeys": ["cs_migrations_team"], "widgets": []}`)

	got, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("LoadDir returned %d dashboards, want 2", len(got))
	}
}

// TestLoadDir_RejectsDuplicateDefaultForTeamKeysOwnership is the fix for the
// gap CodeRabbit flagged: validate did not track defaultForTeamKeys
// ownership at all, so two dashboards claiming the same team key would both
// load, and CsmDashboardPage's find() would silently pick whichever came
// first in dashboard list order -- an outcome driven by LoadDir's filename
// ordering rather than by any config author's intent.
//
// Ownership is tracked by dashboard id (see
// TestParseDashboardsConfig_RejectsDuplicateDefaultForTeamKeysOwnershipAcrossSharedSource
// for why source alone is not enough), so the rejection here names the
// owning dashboard's id, not its source file.
func TestLoadDir_RejectsDuplicateDefaultForTeamKeysOwnership(t *testing.T) {
	dir := t.TempDir()
	writeDefinition(t, dir, "a.json", `{"id": "onboarding-engineer", "displayName": "Onboarding Engineer", "type": "cs", "defaultForTeamKeys": ["customer_onboarding"], "widgets": []}`)
	writeDefinition(t, dir, "b.json", `{"id": "migration-engineer", "displayName": "Migration Engineer", "type": "cs", "defaultForTeamKeys": ["customer_onboarding"], "widgets": []}`)

	_, err := LoadDir(dir)
	if err == nil {
		t.Fatal("LoadDir accepted two dashboards claiming the same defaultForTeamKeys entry; expected an error")
	}
	for _, want := range []string{"b.json", "migration-engineer", "onboarding-engineer", "customer_onboarding", "defaultForTeamKeys"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %q, want it to contain %q", err.Error(), want)
		}
	}
}

// TestParseDashboardsConfig_RejectsDuplicateDefaultForTeamKeysOwnershipAcrossSharedSource
// is the regression test for the bug in the fix above (commit faa5265a6):
// ownership was tracked by l.source, which is fine for LoadDir (one source
// file per dashboard) but not for the deprecated DASHBOARDS_CONFIG path,
// where ParseDashboardsConfig decodes many distinct dashboard objects out of
// one JSON payload -- so every dashboard in that payload shares the exact
// same l.source string. Two different dashboards in that single payload
// claiming the same defaultForTeamKeys entry used to pass validation because
// prev == l.source for both; tracking by d.ID instead means they no longer
// share an owner and the second claim is correctly rejected.
func TestParseDashboardsConfig_RejectsDuplicateDefaultForTeamKeysOwnershipAcrossSharedSource(t *testing.T) {
	_, err := ParseDashboardsConfig(`[
		{"id": "onboarding-engineer", "displayName": "Onboarding Engineer", "defaultForTeamKeys": ["customer_onboarding"], "widgets": []},
		{"id": "migration-engineer", "displayName": "Migration Engineer", "defaultForTeamKeys": ["customer_onboarding"], "widgets": []}
	]`)
	if err == nil {
		t.Fatal("ParseDashboardsConfig accepted two dashboards (sharing one DASHBOARDS_CONFIG source) claiming the same defaultForTeamKeys entry; expected an error")
	}
	for _, want := range []string{"migration-engineer", "onboarding-engineer", "customer_onboarding", "defaultForTeamKeys"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %q, want it to contain %q", err.Error(), want)
		}
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

// TestLoadDir_SingleFileDashboardUnaffectedByPartOfSupport is the regression
// check for the "partOf" mechanism: a dashboard with no parts anywhere in the
// directory must load exactly as it always has.
func TestLoadDir_SingleFileDashboardUnaffectedByPartOfSupport(t *testing.T) {
	dir := t.TempDir()
	writeDefinition(t, dir, "01-cs.json", csDefinition)
	writeDefinition(t, dir, "02-cre.json", creDefinition)

	got, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("LoadDir returned %d dashboards, want 2: %+v", len(got), got)
	}
	if got[1].ID != "cre-team" || len(got[1].Widgets) != 2 {
		t.Fatalf("cre-team = %+v; want its usual 2 widgets, untouched by partOf support", got[1])
	}
}

// TestLoadDir_MergesPartFilesIntoPrimaryDashboard proves the split
// mechanism's happy path: a primary file's widgets and a part file's widgets
// end up concatenated onto one dashboard, and the part file itself
// contributes no dashboard beyond that.
func TestLoadDir_MergesPartFilesIntoPrimaryDashboard(t *testing.T) {
	dir := t.TempDir()
	writeDefinition(t, dir, "01-primary.json", `{
	  "id": "cs-overview", "displayName": "CS Overview", "type": "cs",
	  "widgets": [
	    {"id": "open-cases", "displayName": "Open Cases", "resourceType": "case", "shape": "count", "gridWidth": 3,
	     "query": {"filters": [{"field": "state", "op": "in", "values": ["open"]}]}}
	  ]
	}`)
	// Deliberately placed before the primary in lexical order, and under an
	// unrelated filename, to prove the loader groups by "partOf" rather than
	// by file adjacency or naming.
	writeDefinition(t, dir, "00-extra-widgets.json", `{
	  "partOf": "cs-overview",
	  "widgets": [
	    {"id": "closed-cases", "displayName": "Closed Cases", "resourceType": "case", "shape": "count", "gridWidth": 3,
	     "query": {"filters": [{"field": "state", "op": "in", "values": ["closed"]}]}}
	  ]
	}`)

	got, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("LoadDir returned %d dashboards, want 1 (the part file must not become its own dashboard): %+v", len(got), got)
	}
	d := got[0]
	if d.ID != "cs-overview" {
		t.Fatalf("dashboard id = %q, want cs-overview", d.ID)
	}
	if len(d.Widgets) != 2 {
		t.Fatalf("cs-overview has %d widgets, want 2 (1 primary + 1 from the part file): %+v", len(d.Widgets), d.Widgets)
	}
	ids := map[string]bool{}
	for _, w := range d.Widgets {
		ids[w.ID] = true
	}
	if !ids["open-cases"] || !ids["closed-cases"] {
		t.Fatalf("merged widget ids = %v, want both open-cases and closed-cases", ids)
	}
}

// TestLoadDir_WidgetIDCollisionAcrossPartsFailsWholeLoad mirrors
// TestLoadDir_RejectsInvalidWidgets's "duplicate widget id" case, but with
// the colliding widget split across a primary and its part instead of both
// living in one file -- the merge happens before validateWidgets runs, so
// the same check and the same error shape catch it.
func TestLoadDir_WidgetIDCollisionAcrossPartsFailsWholeLoad(t *testing.T) {
	dir := t.TempDir()
	writeDefinition(t, dir, "primary.json", `{
	  "id": "cs-overview", "displayName": "CS Overview", "type": "cs",
	  "widgets": [
	    {"id": "w", "displayName": "W", "resourceType": "case", "shape": "count", "gridWidth": 3,
	     "query": {"filters": [{"field": "state", "op": "in", "values": ["open"]}]}}
	  ]
	}`)
	writeDefinition(t, dir, "part.json", `{
	  "partOf": "cs-overview",
	  "widgets": [
	    {"id": "w", "displayName": "W dup", "resourceType": "case", "shape": "count", "gridWidth": 3,
	     "query": {"filters": [{"field": "state", "op": "in", "values": ["closed"]}]}}
	  ]
	}`)

	_, err := LoadDir(dir)
	if err == nil {
		t.Fatal("LoadDir accepted a widget id colliding across a primary and its part; expected an error")
	}
	if !strings.Contains(err.Error(), `duplicate widget id "w"`) {
		t.Fatalf("err = %q, want it to contain the same duplicate-widget-id message the single-file case uses", err.Error())
	}
	if !strings.Contains(err.Error(), filepath.Join(dir, "primary.json")) {
		t.Fatalf("err = %q, want it to name the primary file the merged widget list belongs to", err.Error())
	}
}

// TestLoadDir_OrphanPartOfFailsWholeLoad: a part file naming a dashboard id
// nothing else defines is not silently dropped -- same fail-loud rule as an
// unknown preset or section reference elsewhere in this loader.
func TestLoadDir_OrphanPartOfFailsWholeLoad(t *testing.T) {
	dir := t.TempDir()
	writeDefinition(t, dir, "cs.json", csDefinition)
	writeDefinition(t, dir, "orphan-part.json", `{
	  "partOf": "does-not-exist",
	  "widgets": [
	    {"id": "stray", "displayName": "Stray", "resourceType": "case", "shape": "count", "gridWidth": 3,
	     "query": {"filters": [{"field": "state", "op": "in", "values": ["open"]}]}}
	  ]
	}`)

	_, err := LoadDir(dir)
	if err == nil {
		t.Fatal("LoadDir accepted a part file whose partOf matched no loaded dashboard; expected an error")
	}
	for _, want := range []string{"orphan-part.json", `"partOf"`, "does-not-exist"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("err = %q, does not mention %q", err.Error(), want)
		}
	}
}

// TestLoadDir_PartFileWithUnexpectedFieldFailsWholeLoad: a part file is only
// ever read for its "partOf"/"widgets" -- any other embedded Dashboard field
// (id, displayName, filterPresets, etc) decodes successfully via
// dashboardFile's embedded Dashboard and then silently vanishes, since
// dashboardPart never carries it forward. That must be a hard load failure,
// not a silent drop, the same fail-loud rule TestLoadDir_OrphanPartOfFailsWholeLoad
// enforces for a mismatched "partOf".
func TestLoadDir_PartFileWithUnexpectedFieldFailsWholeLoad(t *testing.T) {
	dir := t.TempDir()
	writeDefinition(t, dir, "primary.json", `{
	  "id": "cs-overview", "displayName": "CS Overview", "type": "cs",
	  "widgets": [
	    {"id": "open-cases", "displayName": "Open Cases", "resourceType": "case", "shape": "count", "gridWidth": 3,
	     "query": {"filters": [{"field": "state", "op": "in", "values": ["open"]}]}}
	  ]
	}`)
	writeDefinition(t, dir, "part.json", `{
	  "partOf": "cs-overview",
	  "displayName": "This should never take effect",
	  "widgets": [
	    {"id": "closed-cases", "displayName": "Closed Cases", "resourceType": "case", "shape": "count", "gridWidth": 3,
	     "query": {"filters": [{"field": "state", "op": "in", "values": ["closed"]}]}}
	  ]
	}`)

	_, err := LoadDir(dir)
	if err == nil {
		t.Fatal("LoadDir accepted a part file carrying an unexpected field (\"displayName\"); expected an error")
	}
	for _, want := range []string{"part.json", `"partOf"`, "displayName"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("err = %q, does not mention %q", err.Error(), want)
		}
	}
}
