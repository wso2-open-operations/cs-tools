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
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/entity"
)

// fakeSystemStore is an in-memory SystemStore. saveErrs are returned by
// successive saves (nil entries succeed); listErr fails every list.
type fakeSystemStore struct {
	mu       sync.Mutex
	rows     map[string]entity.SystemDashboard
	saves    int
	lists    int
	saveErrs []error
	listErr  error
	deleted  []string
	// deleteErrs maps a key to the error its delete returns.
	deleteErrs map[string]error
}

func newFakeSystemStore() *fakeSystemStore {
	return &fakeSystemStore{rows: map[string]entity.SystemDashboard{}}
}

func (f *fakeSystemStore) SaveSystemDashboard(_ context.Context, key, name, definition string) (entity.SystemDashboard, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.saves++
	if len(f.saveErrs) > 0 {
		err := f.saveErrs[0]
		f.saveErrs = f.saveErrs[1:]
		if err != nil {
			return entity.SystemDashboard{}, err
		}
	}
	rec := entity.SystemDashboard{Key: key, IsSystem: true, Name: name, Definition: definition}
	f.rows[key] = rec
	return rec, nil
}

func (f *fakeSystemStore) DeleteSystemDashboard(_ context.Context, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.deleteErrs[key]; err != nil {
		return err
	}
	f.deleted = append(f.deleted, key)
	delete(f.rows, key)
	return nil
}

func (f *fakeSystemStore) ListSystemDashboards(context.Context) ([]entity.SystemDashboard, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lists++
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := make([]entity.SystemDashboard, 0, len(f.rows))
	for _, r := range f.rows {
		out = append(out, r)
	}
	return out, nil
}

func noBackoff(t *testing.T) {
	t.Helper()
	prev := systemSaveBackoff
	systemSaveBackoff = time.Millisecond
	t.Cleanup(func() { systemSaveBackoff = prev })
}

func dirSource(t *testing.T, files map[string]string, hotReload bool, presets, sections string) *Registry {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		writeDefinition(t, dir, name, body)
	}
	src, err := NewDirRegistry(dir, hotReload, presets, sections)
	if err != nil {
		t.Fatalf("NewDirRegistry: %v", err)
	}
	return src
}

func TestSystemKey(t *testing.T) {
	long := strings.Repeat("a", 300)
	cases := []struct {
		id, want string
		wantErr  bool
	}{
		{id: "5e636a12-4c0c-4d86-9ee5-6f4e0eddf981", want: "5e636a12-4c0c-4d86-9ee5-6f4e0eddf981"},
		{id: "cs-overview", want: "cs-overview"},
		{id: "My Team / Overview", want: "My-Team-Overview"},
		{id: "  _x y  ", want: "x-y"},
		{id: "a.b_c-d", want: "a.b_c-d"},
		{id: long, want: strings.Repeat("a", 255)},
		{id: "", wantErr: true},
		{id: " / ", wantErr: true},
		{id: "___", wantErr: true},
	}
	for _, tc := range cases {
		got, err := SystemKey(tc.id)
		if tc.wantErr {
			if err == nil {
				t.Errorf("SystemKey(%q) = %q, want an error", tc.id, got)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("SystemKey(%q) = %q, %v; want %q", tc.id, got, err, tc.want)
		}
	}
}

func TestNewSystemRegistry_KeyProblemsAreFatal(t *testing.T) {
	t.Run("two ids slugging to one key", func(t *testing.T) {
		src := NewStaticRegistry([]Dashboard{
			{ID: "Team A", DisplayName: "A"},
			{ID: "Team-A", DisplayName: "B"},
		})
		_, err := NewSystemRegistry(newFakeSystemStore(), src)
		if err == nil || !strings.Contains(err.Error(), "Team-A") {
			t.Fatalf("err = %v, want a collision error naming the key", err)
		}
	})
	t.Run("an id with no valid key", func(t *testing.T) {
		src := NewStaticRegistry([]Dashboard{{ID: "///", DisplayName: "A"}})
		if _, err := NewSystemRegistry(newFakeSystemStore(), src); err == nil {
			t.Fatal("expected an error for an id that has no key")
		}
	})
}

func TestSystemRegistry_MigrateAndServe(t *testing.T) {
	src := dirSource(t, map[string]string{
		"a.json": csDefinition,
		"b.json": `{"id":"My Team Board","displayName":"Another","type":"cs","widgets":[` +
			`{"id":"w","displayName":"W","resourceType":"case","shape":"count","gridWidth":3,"query":{"filters":[{"field":"state","op":"in","values":["open"]}]}}]}`,
	}, false, "", "")
	store := newFakeSystemStore()
	reg, err := NewSystemRegistry(store, src)
	if err != nil {
		t.Fatalf("NewSystemRegistry: %v", err)
	}
	if failed := reg.Migrate(context.Background()); failed != 0 {
		t.Fatalf("Migrate failed %d", failed)
	}
	if _, ok := store.rows["cs-overview"]; !ok || store.rows["cs-overview"].Name != "CS Overview" {
		t.Errorf("rows = %+v, want cs-overview stored by id", store.rows)
	}
	if _, ok := store.rows["My-Team-Board"]; !ok {
		t.Errorf("rows = %+v, want the slugged key My-Team-Board", store.rows)
	}

	got, err := reg.List(context.Background())
	if err != nil || len(got) != 2 {
		t.Fatalf("List = %d dashboards, err %v; want 2", len(got), err)
	}
	// Display-name order: "Another" before "CS Overview".
	if got[0].ID != "My Team Board" || got[1].ID != "cs-overview" {
		t.Errorf("order = %s, %s", got[0].ID, got[1].ID)
	}
	d, ok, err := reg.Get(context.Background(), "cs-overview")
	if err != nil || !ok || d.DisplayName != "CS Overview" || len(d.Widgets) != 1 {
		t.Errorf("Get = %+v, %v, %v", d, ok, err)
	}
	if _, ok, _ := reg.Get(context.Background(), "nope"); ok {
		t.Error("Get found an unknown id")
	}
}

func TestSystemRegistry_StoredDefinitionIsFullyExpanded(t *testing.T) {
	root := t.TempDir()
	presets := filepath.Join(root, "presets.json")
	sections := filepath.Join(root, "sections.json")
	if err := os.WriteFile(presets, []byte(`{"openStates":{"field":"state","op":"in","values":["open"]}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sections, []byte(`{"my-work":{"displayName":"My Work","widgets":[`+
		`{"id":"mine","displayName":"Mine","resourceType":"case","shape":"count","gridWidth":3,"query":{"filters":[{"preset":"openStates"}]}}]}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	src := dirSource(t, map[string]string{"d.json": `{"id":"d","displayName":"D","type":"cs","isDefault":true,` +
		`"defaultForTeamKeys":["team-a"],` +
		`"filterPresets":{"local":{"field":"severity","op":"in","values":["high"]}},` +
		`"includeSections":[{"section":"my-work"}],` +
		`"widgets":[{"id":"w","displayName":"W","resourceType":"case","shape":"count","gridWidth":3,"query":{"filters":[{"preset":"local"}]}}]}`},
		false, presets, sections)

	store := newFakeSystemStore()
	reg, err := NewSystemRegistry(store, src)
	if err != nil {
		t.Fatal(err)
	}
	reg.Migrate(context.Background())

	def := store.rows["d"].Definition
	for _, banned := range []string{"includeSections", "filterPresets", `"preset"`, "partOf"} {
		if strings.Contains(def, banned) {
			t.Errorf("stored definition still carries %q: %s", banned, def)
		}
	}
	var stored Dashboard
	if err := json.Unmarshal([]byte(def), &stored); err != nil {
		t.Fatalf("stored definition is not a dashboard: %v", err)
	}
	if len(stored.Widgets) != 2 {
		t.Fatalf("stored widgets = %d, want the section's widget plus the dashboard's own", len(stored.Widgets))
	}
	if !stored.IsDefault || len(stored.DefaultForTeamKeys) != 1 || stored.DefaultForTeamKeys[0] != "team-a" {
		t.Errorf("stored metadata lost: %+v", stored)
	}
	// What is served equals what the file registry resolved.
	if !reflect.DeepEqual(stored, src.Dashboards()[0]) {
		t.Errorf("stored definition differs from the resolved file definition:\n%#v\n%#v", stored, src.Dashboards()[0])
	}
}

func TestSystemRegistry_ExampleDirectoryRoundTrips(t *testing.T) {
	src, err := NewDirRegistry("../../dashboards.example", false, "", "../../dashboards.example/_sections.json")
	if err != nil {
		t.Fatalf("NewDirRegistry: %v", err)
	}
	store := newFakeSystemStore()
	reg, err := NewSystemRegistry(store, src)
	if err != nil {
		t.Fatal(err)
	}
	reg.Migrate(context.Background())
	got, err := reg.List(context.Background())
	if err != nil || len(got) != len(src.Dashboards()) || len(got) == 0 {
		t.Fatalf("List = %d, err %v; want %d", len(got), err, len(src.Dashboards()))
	}
	for _, want := range src.Dashboards() {
		d, ok, _ := reg.Get(context.Background(), want.ID)
		if !ok || !reflect.DeepEqual(d, want) {
			t.Errorf("dashboard %q did not round-trip through the store", want.ID)
		}
	}
}

func TestSystemRegistry_UpsertFailureIsTolerated(t *testing.T) {
	noBackoff(t)
	src := dirSource(t, map[string]string{"a.json": csDefinition}, false, "", "")

	t.Run("transient failures are retried", func(t *testing.T) {
		store := newFakeSystemStore()
		store.saveErrs = []error{&apierror.Error{StatusCode: http.StatusServiceUnavailable}, errors.New("connection refused")}
		reg, _ := NewSystemRegistry(store, src)
		if failed := reg.Migrate(context.Background()); failed != 0 {
			t.Fatalf("failed = %d, want 0 after retries", failed)
		}
		if store.saves != 3 || len(store.rows) != 1 {
			t.Errorf("saves = %d, rows = %d", store.saves, len(store.rows))
		}
	})

	t.Run("persistent failure is counted, not fatal, and not served from the file", func(t *testing.T) {
		store := newFakeSystemStore()
		store.saveErrs = []error{errors.New("down"), errors.New("down"), errors.New("down")}
		reg, _ := NewSystemRegistry(store, src)
		if failed := reg.Migrate(context.Background()); failed != 1 {
			t.Fatalf("failed = %d, want 1", failed)
		}
		if store.saves != systemSaveAttempts {
			t.Errorf("saves = %d, want %d attempts", store.saves, systemSaveAttempts)
		}
		got, err := reg.List(context.Background())
		if err != nil || len(got) != 0 {
			t.Errorf("List = %v, %v; want the (empty) database content, not the file's", got, err)
		}
	})

	t.Run("a 4xx is not retried", func(t *testing.T) {
		store := newFakeSystemStore()
		store.saveErrs = []error{&apierror.Error{StatusCode: http.StatusForbidden}}
		reg, _ := NewSystemRegistry(store, src)
		if failed := reg.Migrate(context.Background()); failed != 1 || store.saves != 1 {
			t.Errorf("failed = %d, saves = %d; want 1 and 1", failed, store.saves)
		}
	})

	t.Run("the context bounds the retries", func(t *testing.T) {
		prev := systemSaveBackoff
		systemSaveBackoff = time.Hour
		defer func() { systemSaveBackoff = prev }()
		store := newFakeSystemStore()
		store.saveErrs = []error{errors.New("down"), errors.New("down"), errors.New("down")}
		reg, _ := NewSystemRegistry(store, src)
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		start := time.Now()
		reg.Migrate(ctx)
		if time.Since(start) > 5*time.Second {
			t.Error("Migrate did not stop at the context deadline")
		}
	})
}

func TestSystemRegistry_ServesDatabaseWithoutFiles(t *testing.T) {
	store := newFakeSystemStore()
	def, _ := json.Marshal(Dashboard{ID: "kept", DisplayName: "Kept", Type: TypeCS, Widgets: []WidgetTemplate{}})
	store.rows["kept"] = entity.SystemDashboard{Key: "kept", Name: "Kept", Definition: string(def)}
	store.rows["broken"] = entity.SystemDashboard{Key: "broken", Name: "Broken", Definition: "{not json"}

	reg, err := NewSystemRegistry(store, nil)
	if err != nil {
		t.Fatal(err)
	}
	if failed := reg.Migrate(context.Background()); failed != 0 || store.saves != 0 {
		t.Errorf("Migrate with no files wrote: failed %d, saves %d", failed, store.saves)
	}
	got, err := reg.List(context.Background())
	if err != nil || len(got) != 1 || got[0].ID != "kept" {
		t.Fatalf("List = %+v, %v; want only the decodable stored dashboard", got, err)
	}
	if reg.FilterPresets() != nil || reg.SharedSections() != nil {
		t.Error("catalogues should be empty without a file source")
	}
}

func TestSystemRegistry_ReadFailureIsAnError(t *testing.T) {
	store := newFakeSystemStore()
	store.listErr = &apierror.Error{StatusCode: http.StatusServiceUnavailable}
	reg, _ := NewSystemRegistry(store, nil)
	got, err := reg.List(context.Background())
	if err == nil || got != nil {
		t.Fatalf("List = %v, %v; want an error and no list", got, err)
	}
	var apiErr *apierror.Error
	if !errors.As(err, &apiErr) {
		t.Errorf("error %v does not unwrap to the upstream *apierror.Error", err)
	}
	if _, _, err := reg.Get(context.Background(), "x"); err == nil {
		t.Error("Get swallowed the read failure")
	}
}

func TestSystemRegistry_CacheIsShortLivedAndInvalidatedByUpsert(t *testing.T) {
	src := dirSource(t, map[string]string{"a.json": csDefinition}, false, "", "")
	store := newFakeSystemStore()
	reg, _ := NewSystemRegistry(store, src)
	ctx := context.Background()

	reg.Migrate(ctx)
	store.lists = 0 // Migrate's own reconcile read is not under test here
	_, _ = reg.List(ctx)
	_, _ = reg.List(ctx)
	if store.lists != 1 {
		t.Errorf("lists = %d, want the second read served from cache", store.lists)
	}

	reg.system.mu.Lock()
	reg.system.fetchedAt = time.Now().Add(-2 * systemDashboardCacheTTL)
	reg.system.mu.Unlock()
	_, _ = reg.List(ctx)
	if store.lists != 2 {
		t.Errorf("lists = %d, want a refetch after the TTL", store.lists)
	}

	reg.Migrate(ctx) // same definition: no write, but the cache is dropped
	store.lists = 0
	_, _ = reg.List(ctx)
	if store.lists != 1 || store.saves != 1 {
		t.Errorf("lists = %d, saves = %d; want a refetch after Migrate and no repeat write", store.lists, store.saves)
	}
}

func TestSystemRegistry_HotReloadUpsertsChangedDefinitions(t *testing.T) {
	dir := t.TempDir()
	path := writeDefinition(t, dir, "a.json", csDefinition)
	src, err := NewDirRegistry(dir, true, "", "")
	if err != nil {
		t.Fatal(err)
	}
	store := newFakeSystemStore()
	reg, _ := NewSystemRegistry(store, src)
	ctx := context.Background()
	reg.Migrate(ctx)

	got, _ := reg.List(ctx)
	if len(got) != 1 || got[0].DisplayName != "CS Overview" || store.saves != 1 {
		t.Fatalf("got %+v, saves %d", got, store.saves)
	}

	edited := strings.Replace(csDefinition, `"displayName": "CS Overview"`, `"displayName": "CS Overview v2"`, 1)
	if err := os.WriteFile(path, []byte(edited), 0o600); err != nil {
		t.Fatal(err)
	}
	got, _ = reg.List(ctx)
	if len(got) != 1 || got[0].DisplayName != "CS Overview v2" || store.saves != 2 {
		t.Errorf("after edit: %+v, saves %d; want the edit upserted and served", got, store.saves)
	}
	_, _ = reg.List(ctx)
	if store.saves != 2 {
		t.Errorf("saves = %d, an unchanged definition must not be re-written", store.saves)
	}
}

func stale(f *fakeSystemStore, keys ...string) {
	for _, k := range keys {
		f.rows[k] = entity.SystemDashboard{Key: k, Name: k, Definition: `{"id":"` + k + `","displayName":"` + k + `"}`}
	}
}

func TestSystemRegistry_ReconcileRemovesStaleRows(t *testing.T) {
	t.Run("a renamed file replaces its old row", func(t *testing.T) {
		store := newFakeSystemStore()
		stale(store, "old-id", "kept-elsewhere")
		src := dirSource(t, map[string]string{"a.json": csDefinition}, false, "", "")
		reg, _ := NewSystemRegistry(store, src)
		reg.Migrate(context.Background())
		if _, ok := store.rows["cs-overview"]; !ok {
			t.Error("new key not stored")
		}
		if _, ok := store.rows["old-id"]; ok {
			t.Error("old key still stored")
		}
		got, err := reg.List(context.Background())
		if err != nil || len(got) != 1 || got[0].ID != "cs-overview" {
			t.Errorf("List = %+v, %v; want only the migrated dashboard", got, err)
		}
		if len(store.deleted) != 2 {
			t.Errorf("deleted = %v", store.deleted)
		}
	})

	t.Run("no source mounted keeps every row", func(t *testing.T) {
		store := newFakeSystemStore()
		stale(store, "a", "b")
		reg, _ := NewSystemRegistry(store, nil)
		reg.Migrate(context.Background())
		if len(store.deleted) != 0 || len(store.rows) != 2 {
			t.Errorf("deleted = %v, rows = %d", store.deleted, len(store.rows))
		}
	})

	t.Run("a failed upsert deletes nothing", func(t *testing.T) {
		noBackoff(t)
		store := newFakeSystemStore()
		stale(store, "old-id")
		store.saveErrs = []error{errors.New("down"), errors.New("down"), errors.New("down")}
		src := dirSource(t, map[string]string{"a.json": csDefinition}, false, "", "")
		reg, _ := NewSystemRegistry(store, src)
		reg.Migrate(context.Background())
		if len(store.deleted) != 0 {
			t.Errorf("deleted = %v after a failed upsert", store.deleted)
		}
	})

	t.Run("a timed out migration deletes nothing", func(t *testing.T) {
		store := newFakeSystemStore()
		stale(store, "old-id")
		src := dirSource(t, map[string]string{"a.json": csDefinition}, false, "", "")
		reg, _ := NewSystemRegistry(store, src)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		reg.Migrate(ctx)
		if len(store.deleted) != 0 {
			t.Errorf("deleted = %v after the context ended", store.deleted)
		}
	})

	t.Run("a failed list read deletes nothing", func(t *testing.T) {
		store := newFakeSystemStore()
		stale(store, "old-id")
		store.listErr = errors.New("down")
		src := dirSource(t, map[string]string{"a.json": csDefinition}, false, "", "")
		reg, _ := NewSystemRegistry(store, src)
		if failed := reg.Migrate(context.Background()); failed != 0 {
			t.Errorf("failed = %d; a list failure is not an upsert failure", failed)
		}
		if len(store.deleted) != 0 {
			t.Errorf("deleted = %v", store.deleted)
		}
	})

	t.Run("an empty mounted directory deletes nothing", func(t *testing.T) {
		store := newFakeSystemStore()
		stale(store, "a", "b")
		reg, _ := NewSystemRegistry(store, dirSource(t, nil, false, "", ""))
		reg.Migrate(context.Background())
		if len(store.deleted) != 0 || len(store.rows) != 2 {
			t.Errorf("deleted = %v, rows = %d", store.deleted, len(store.rows))
		}
	})

	t.Run("a failed delete is tolerated and the others continue", func(t *testing.T) {
		store := newFakeSystemStore()
		stale(store, "bad", "good")
		store.deleteErrs = map[string]error{"bad": &apierror.Error{StatusCode: http.StatusInternalServerError}}
		src := dirSource(t, map[string]string{"a.json": csDefinition}, false, "", "")
		reg, _ := NewSystemRegistry(store, src)
		if failed := reg.Migrate(context.Background()); failed != 0 {
			t.Errorf("failed = %d", failed)
		}
		if _, ok := store.rows["bad"]; !ok {
			t.Error("bad should remain")
		}
		if _, ok := store.rows["good"]; ok {
			t.Error("good should be deleted despite the other failure")
		}
	})
}

func TestSystemRegistry_HotReloadReconcilesOnKeySetChange(t *testing.T) {
	dir := t.TempDir()
	writeDefinition(t, dir, "a.json", csDefinition)
	src, err := NewDirRegistry(dir, true, "", "")
	if err != nil {
		t.Fatal(err)
	}
	store := newFakeSystemStore()
	reg, _ := NewSystemRegistry(store, src)
	ctx := context.Background()
	reg.Migrate(ctx)

	// Unchanged directory: further reads must not list for reconciliation.
	_, _ = reg.List(ctx)
	before := store.lists
	reg.system.invalidate()
	_, _ = reg.List(ctx)
	if store.lists != before+1 {
		t.Errorf("lists = %d, want exactly the one serving read", store.lists-before)
	}

	// Rename the dashboard: new id in a new file, old file gone.
	if err := os.Remove(filepath.Join(dir, "a.json")); err != nil {
		t.Fatal(err)
	}
	writeDefinition(t, dir, "b.json", strings.Replace(csDefinition, `"id": "cs-overview"`, `"id": "cs-overview-2"`, 1))
	got, err := reg.List(ctx)
	if err != nil || len(got) != 1 || got[0].ID != "cs-overview-2" {
		t.Fatalf("List = %+v, %v; want only the renamed dashboard", got, err)
	}
	if _, ok := store.rows["cs-overview"]; ok {
		t.Error("old row not deleted on hot reload")
	}
}
