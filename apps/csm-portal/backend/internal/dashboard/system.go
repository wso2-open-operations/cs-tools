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
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/entity"
)

// The system dashboards are the dashboards every user sees, stored in the
// database (entity-service /system-dashboards) and served from there. The
// definition files DASHBOARDS_DIR points at are only a MIGRATION SOURCE: at
// startup every file's fully expanded definition is upserted into the
// database, overwriting what was there, so while a directory is mounted its
// files are authoritative. With none mounted the registry serves whatever the
// database already holds.

const (
	// SystemMigrationTimeout bounds the whole startup upsert, retries
	// included, so an unreachable entity-service delays the listener by at
	// most this long.
	SystemMigrationTimeout = 30 * time.Second

	// systemDashboardCacheTTL is how long a fetched dashboard list is served
	// from memory. A cache is only a read optimisation: it is dropped the
	// moment this process upserts, and a write by another replica shows up
	// within one TTL.
	systemDashboardCacheTTL = 15 * time.Second

	// maxSystemKeyLen and maxSystemNameLen are entity-service's limits on a
	// system dashboard's key and name.
	maxSystemKeyLen  = 255
	maxSystemNameLen = 255
)

var (
	// systemSaveAttempts and systemSaveBackoff govern the per-dashboard retry
	// of a failed upsert (entity-service may still be starting). The backoff
	// doubles each attempt. Variables so tests need not wait.
	systemSaveAttempts = 3
	systemSaveBackoff  = time.Second

	validSystemKey  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	invalidKeyRun   = regexp.MustCompile(`[^A-Za-z0-9._-]+`)
	leadingNonAlnum = regexp.MustCompile(`^[^A-Za-z0-9]+`)
)

// SystemStore is the part of the entity client the system dashboards need.
type SystemStore interface {
	ListSystemDashboards(ctx context.Context) ([]entity.SystemDashboard, error)
	SaveSystemDashboard(ctx context.Context, key, name, definition string) (entity.SystemDashboard, error)
	DeleteSystemDashboard(ctx context.Context, key string) error
}

// SystemKey derives the database key of the dashboard with this id. An id
// that already satisfies the key rule (^[A-Za-z0-9][A-Za-z0-9._-]*$, at most
// 255 characters, which covers UUIDs) is its own key, so the key is stable
// and never depends on the filename. Any other id is slugged: each run of
// characters outside [A-Za-z0-9._-] becomes one "-", leading non-alphanumerics
// are dropped and the result is cut to 255 characters. An id that slugs to
// nothing has no key and is an error.
func SystemKey(id string) (string, error) {
	key := strings.TrimSpace(id)
	if !validSystemKey.MatchString(key) {
		key = leadingNonAlnum.ReplaceAllString(invalidKeyRun.ReplaceAllString(key, "-"), "")
	}
	if len(key) > maxSystemKeyLen {
		key = key[:maxSystemKeyLen]
	}
	if !validSystemKey.MatchString(key) {
		return "", fmt.Errorf("dashboard id %q yields no valid system dashboard key", id)
	}
	return key, nil
}

// systemBacking is a Registry's database side. src is the file-backed
// registry the dashboards are migrated from, nil when none is configured.
type systemBacking struct {
	store SystemStore
	src   *Registry
	keys  map[string]string // dashboard id -> key, for src's dashboards

	syncMu sync.Mutex // serialises hot-reload syncs

	mu        sync.Mutex
	cache     []Dashboard
	fetchedAt time.Time
	saved     map[string]string // key -> definition this process last upserted

	// reconciled is the key set the last successful reconcile ran for; the
	// hot-reload path reconciles again only when the directory's key set
	// differs from it. Guarded by syncMu.
	reconciled map[string]bool
}

// NewSystemRegistry builds a registry that serves from store. src is the
// registry loaded from DASHBOARDS_DIR or DASHBOARDS_CONFIG, or nil when
// neither is set. It fails, before anything is written, when two of src's
// dashboards slug to the same key or an id has no valid key: both are
// misconfigured definitions, like any other load error.
func NewSystemRegistry(store SystemStore, src *Registry) (*Registry, error) {
	b := &systemBacking{store: store, src: src, saved: map[string]string{}}
	if src != nil {
		keys, err := systemKeys(src.Dashboards())
		if err != nil {
			return nil, err
		}
		b.keys = keys
	}
	return &Registry{system: b}, nil
}

// systemKeys maps every dashboard's id to its key, rejecting duplicates.
func systemKeys(dashboards []Dashboard) (map[string]string, error) {
	keys := make(map[string]string, len(dashboards))
	owner := make(map[string]string, len(dashboards))
	for _, d := range dashboards {
		key, err := SystemKey(d.ID)
		if err != nil {
			return nil, fmt.Errorf("dashboard definitions: %w", err)
		}
		if prev, dup := owner[key]; dup {
			return nil, fmt.Errorf("dashboard definitions: dashboard ids %q and %q both map to system dashboard key %q", prev, d.ID, key)
		}
		owner[key] = d.ID
		keys[d.ID] = key
	}
	return keys, nil
}

// Migrate upserts every dashboard of the file source into the database and
// returns how many failed. A failure is logged and the next dashboard still
// goes: the database stays the system of record, so a dashboard that could
// not be written is served as the database has it (or is absent), never from
// the file. Each upsert is retried a few times, and ctx bounds the whole run.
// A no-op without a file source.
func (r *Registry) Migrate(ctx context.Context) int {
	if r == nil || r.system == nil {
		return 0
	}
	return r.system.migrate(ctx)
}

func (b *systemBacking) migrate(ctx context.Context) int {
	if b.src == nil {
		return 0
	}
	dashboards := b.src.Dashboards()
	failed := 0
	for _, d := range dashboards {
		if err := b.upsert(ctx, b.keys[d.ID], d); err != nil {
			failed++
			slog.ErrorContext(ctx, "system dashboard upsert failed; serving what the database has",
				"id", d.ID, "key", b.keys[d.ID], "err", err)
		}
	}
	removed := 0
	// A partial migration must delete nothing: only reconcile once every
	// upsert succeeded and the migration was not cut short.
	if failed == 0 && ctx.Err() == nil {
		b.syncMu.Lock()
		removed = b.reconcile(ctx, b.keys)
		b.syncMu.Unlock()
	}
	b.invalidate()
	slog.InfoContext(ctx, "system dashboards migrated from definitions", "count", len(dashboards), "failed", failed, "removed", removed)
	return failed
}

// reconcile deletes the stored system dashboards whose key is not in keep, so a
// definition file that was removed or renamed stops being served: while files
// are mounted they are authoritative. It returns how many were deleted. It
// deletes nothing when keep is empty (a mounted but empty directory must not
// wipe the table) or when the stored list cannot be read, and a failed delete
// is logged per key without stopping the others. The caller has already
// established that every upsert succeeded. When nothing failed it records keep as
// the reconciled set, and it drops the list cache after deleting. Needs syncMu.
func (b *systemBacking) reconcile(ctx context.Context, keep map[string]string) int {
	if len(keep) == 0 {
		slog.WarnContext(ctx, "no dashboard definitions were found; keeping every stored system dashboard")
		return 0
	}
	keepKeys := make(map[string]bool, len(keep))
	for _, key := range keep {
		keepKeys[key] = true
	}
	records, err := b.store.ListSystemDashboards(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "system dashboards: not removing stale ones, the stored list could not be read", "err", err)
		return 0
	}
	removed, deleteFailed := 0, false
	for _, rec := range records {
		if keepKeys[rec.Key] {
			continue
		}
		if err := b.store.DeleteSystemDashboard(ctx, rec.Key); err != nil {
			slog.ErrorContext(ctx, "system dashboard removal failed", "key", rec.Key, "err", err)
			deleteFailed = true
			continue
		}
		slog.InfoContext(ctx, "removed system dashboard with no definition", "key", rec.Key)
		removed++
	}
	if !deleteFailed {
		b.reconciled = keepKeys
	}
	if removed > 0 {
		b.invalidate()
	}
	return removed
}

// upsert writes one dashboard, skipping the call when this process already
// wrote exactly this definition, and retrying transient failures.
func (b *systemBacking) upsert(ctx context.Context, key string, d Dashboard) error {
	raw, err := json.Marshal(d)
	if err != nil {
		return fmt.Errorf("encode definition: %w", err)
	}
	definition := string(raw)
	b.mu.Lock()
	same := b.saved[key] == definition
	b.mu.Unlock()
	if same {
		return nil
	}

	name := d.DisplayName
	if r := []rune(name); len(r) > maxSystemNameLen {
		name = string(r[:maxSystemNameLen])
	}

	backoff := systemSaveBackoff
	for attempt := 1; ; attempt++ {
		_, err = b.store.SaveSystemDashboard(ctx, key, name, definition)
		if err == nil {
			b.mu.Lock()
			b.saved[key] = definition
			b.mu.Unlock()
			return nil
		}
		if attempt >= systemSaveAttempts || !retryableSave(err) {
			return err
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("%w (giving up after attempt %d: %v)", err, attempt, ctx.Err())
		case <-time.After(backoff):
		}
		backoff *= 2
	}
}

// retryableSave reports whether another attempt can help: a rejected request
// (4xx other than 408/429) will be rejected again.
func retryableSave(err error) bool {
	var apiErr *apierror.Error
	if errors.As(err, &apiErr) {
		c := apiErr.StatusCode
		return c >= http.StatusInternalServerError || c == http.StatusRequestTimeout || c == http.StatusTooManyRequests
	}
	return true
}

func (b *systemBacking) invalidate() {
	b.mu.Lock()
	b.cache, b.fetchedAt = nil, time.Time{}
	b.mu.Unlock()
}

// syncHotReload is DASHBOARDS_HOT_RELOAD in the database-backed model: the
// file source re-reads its directory (keeping its last good set on a bad
// edit), and every definition that changed since this process last wrote it
// is upserted, so the edit shows up in what is served.
func (b *systemBacking) syncHotReload(ctx context.Context) {
	if b.src == nil || !b.src.hotReload {
		return
	}
	b.syncMu.Lock()
	defer b.syncMu.Unlock()
	dashboards := b.src.Dashboards()
	keys, err := systemKeys(dashboards)
	if err != nil {
		slog.ErrorContext(ctx, "dashboard hot reload: not syncing to the database", "err", err)
		return
	}
	b.keys = keys
	changed, failed := false, false
	for _, d := range dashboards {
		before := b.saved[keys[d.ID]]
		if err := b.upsert(ctx, keys[d.ID], d); err != nil {
			slog.ErrorContext(ctx, "dashboard hot reload: system dashboard upsert failed", "id", d.ID, "err", err)
			failed = true
			continue
		}
		changed = changed || before != b.saved[keys[d.ID]]
	}
	// Stale rows are looked for only when the directory's key set differs from
	// the one last reconciled, so an unchanged directory costs no list or
	// delete per request; and never after a failed upsert.
	if !failed && ctx.Err() == nil && !sameKeySet(b.reconciled, keys) {
		if b.reconcile(ctx, keys) > 0 {
			changed = true
		}
	}
	if changed {
		b.invalidate()
	}
}

// sameKeySet reports whether done holds exactly the keys of keys' values.
func sameKeySet(done map[string]bool, keys map[string]string) bool {
	if len(done) != len(keys) {
		return false
	}
	for _, key := range keys {
		if !done[key] {
			return false
		}
	}
	return true
}

// list returns the stored dashboards, ordered by display name then id (the
// database's key order is not meaningful to a picker). A failed read is an
// error, never an empty list. A stored definition that cannot be decoded is
// logged and left out rather than failing every other dashboard with it.
func (b *systemBacking) list(ctx context.Context) ([]Dashboard, error) {
	b.syncHotReload(ctx)

	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.fetchedAt.IsZero() && time.Since(b.fetchedAt) < systemDashboardCacheTTL {
		return b.cache, nil
	}
	records, err := b.store.ListSystemDashboards(ctx)
	if err != nil {
		return nil, fmt.Errorf("list system dashboards: %w", err)
	}
	dashboards := make([]Dashboard, 0, len(records))
	for _, rec := range records {
		var d Dashboard
		if err := json.Unmarshal([]byte(rec.Definition), &d); err != nil {
			slog.ErrorContext(ctx, "system dashboard definition is not valid JSON; skipping it", "key", rec.Key, "err", err)
			continue
		}
		if d.ID == "" {
			d.ID = rec.Key
		}
		dashboards = append(dashboards, d)
	}
	sort.SliceStable(dashboards, func(i, j int) bool {
		a, c := strings.ToLower(dashboards[i].DisplayName), strings.ToLower(dashboards[j].DisplayName)
		if a != c {
			return a < c
		}
		return dashboards[i].ID < dashboards[j].ID
	})
	b.cache, b.fetchedAt = dashboards, time.Now()
	return dashboards, nil
}
