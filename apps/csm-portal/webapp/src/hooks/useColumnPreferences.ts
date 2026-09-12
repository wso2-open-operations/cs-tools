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

import { useCallback, useMemo, useState } from "react";

/** A single column a table can show/hide/reorder. `id` is a stable key (not
 * necessarily the display order) — `label` is what the picker shows. */
export interface ColumnOption {
  id: string;
  label: string;
}

export interface UseColumnPreferencesArgs {
  /** Identifies this particular table/view, e.g. `"engagements"` or
   * `"announcements"`. Combined with the signed-in user's id to build the
   * `localStorage` key, so two tables (or two users on a shared browser
   * profile) never clobber each other's saved layout. */
  viewId: string;
  /** Stable id for the signed-in user (falls back to email, then
   * `"anonymous"` — see `useColumnPreferencesStorageUserKey`). */
  userKey: string;
  /** Every column this table knows how to render, in the table's own default
   * order. New columns a later deploy adds just show up appended at the end
   * of a returning user's saved order, already visible if `defaultVisibleIds`
   * says so. */
  columns: ColumnOption[];
  /** Which column ids are visible the first time a user opens this table
   * (before they've ever touched the picker). */
  defaultVisibleIds: string[];
}

export interface UseColumnPreferencesResult {
  /** All known columns, in the user's chosen order — what the "customise
   * columns" popover renders, checked or not. */
  allColumns: ColumnOption[];
  /** Only the visible columns, in the user's chosen order — what the table
   * itself should render. */
  visibleColumns: ColumnOption[];
  isVisible: (id: string) => boolean;
  /** Flip a column's visibility. No-ops if this would hide the last visible
   * column — a table with zero columns is never a valid state to save. */
  toggleColumn: (id: string) => void;
  /** Move a column one slot up/down in the shared order (visible and hidden
   * columns share one order, so a column already keeps its relative slot if
   * it's later re-checked). No-ops at either end of the list. */
  moveColumn: (id: string, direction: "up" | "down") => void;
  /** Move a column directly to `targetIndex` in the shared order — for
   * drag-and-drop reordering, which can move an item several slots in one
   * gesture. Unlike `moveColumn`, this is a single atomic update rather than
   * a step repeated `moveColumn` calls: since `moveColumn` is a `useCallback`
   * closing over this render's `state`, calling it more than once in the same
   * synchronous handler (as a multi-slot drag would need to) would have every
   * call act on the same stale `state.order`, only ever producing a one-slot
   * move no matter how many times it's called. `targetIndex` is clamped to
   * the list's bounds; no-ops if `id` isn't found or already sits there. */
  reorderColumn: (id: string, targetIndex: number) => void;
  /** Back to the table's built-in default order + visibility. */
  resetToDefault: () => void;
}

function storageKey(viewId: string, userKey: string): string {
  return `csm:${userKey}:${viewId}:columns`;
}

/**
 * Stable per-user key for {@link useColumnPreferences}'s `userKey`. Prefers
 * the platform user id; falls back to email (still stable, just less
 * canonical) when the id hasn't resolved yet, and finally to a shared
 * `"anonymous"` bucket so the picker still works (just not durably scoped to
 * one person) before either is available. Colons are stripped since they're
 * the key's own field separator — emails don't contain them today, but this
 * keeps the key well-formed even if that ever changes.
 */
export function getColumnPreferencesUserKey(user?: {
  id?: string;
  email?: string;
}): string {
  const raw = user?.id || user?.email || "anonymous";
  return raw.replace(/:/g, "_");
}

interface PersistedColumnState {
  order: string[];
  visible: string[];
}

function loadPersisted(key: string): PersistedColumnState | undefined {
  try {
    const raw = window.localStorage.getItem(key);
    if (!raw) return undefined;
    const parsed = JSON.parse(raw) as Partial<PersistedColumnState>;
    if (!Array.isArray(parsed.order) || !Array.isArray(parsed.visible)) return undefined;
    return { order: parsed.order, visible: parsed.visible };
  } catch {
    // Corrupt/inaccessible storage (private browsing, hand-edited value, …) —
    // fall back to the table's own default rather than throwing.
    return undefined;
  }
}

function savePersisted(key: string, state: PersistedColumnState): void {
  try {
    window.localStorage.setItem(key, JSON.stringify(state));
  } catch {
    // Storage full/unavailable — the picker still works for this session,
    // it just won't survive a reload. Not worth surfacing to the user.
  }
}

/** Reconciles a persisted order/visible set against the table's current
 * column definitions: drops ids the table no longer knows about, and appends
 * any new ones (in their default position/visibility) the persisted state
 * predates. */
function reconcile(
  persisted: PersistedColumnState | undefined,
  columns: ColumnOption[],
  defaultVisibleIds: string[],
): PersistedColumnState {
  const knownIds = new Set(columns.map((c) => c.id));
  if (!persisted) {
    return { order: columns.map((c) => c.id), visible: [...defaultVisibleIds] };
  }
  // A hand-edited or otherwise corrupted `localStorage` value can contain
  // duplicate ids (e.g. `order: ["a", "a", "b"]`) — dedupe alongside the
  // known-id filter so a duplicate never survives into `allColumns`/
  // `visibleColumns`, where every consumer uses the id as a React list key.
  const order = [...new Set(persisted.order.filter((id) => knownIds.has(id)))];
  const orderedIds = new Set(order);
  columns.forEach((c) => {
    if (!orderedIds.has(c.id)) order.push(c.id);
  });
  const visible = [...new Set(persisted.visible.filter((id) => knownIds.has(id)))];
  const visibleIds = new Set(visible);
  columns.forEach((c) => {
    if (!orderedIds.has(c.id) && defaultVisibleIds.includes(c.id) && !visibleIds.has(c.id)) {
      visible.push(c.id);
    }
  });
  return { order, visible };
}

/**
 * Per-user, per-view "customise columns" state, persisted to `localStorage`
 * (no backend round trip — this is presentation-only and never needs to sync
 * across devices). One hook instance owns one table's column layout; render
 * `visibleColumns` in the table body and pair with `ColumnCustomizerButton`
 * (or an equivalent control) for the add/remove/reorder UI.
 */
export function useColumnPreferences({
  viewId,
  userKey,
  columns,
  defaultVisibleIds,
}: UseColumnPreferencesArgs): UseColumnPreferencesResult {
  const key = storageKey(viewId, userKey);

  const [state, setState] = useState<PersistedColumnState>(() =>
    reconcile(loadPersisted(key), columns, defaultVisibleIds),
  );

  // `key` starts wrong on first render for any caller whose `userKey` resolves
  // asynchronously (e.g. `useCurrentUser()`/`useIdTokenClaims()` are both
  // still `undefined` on mount, so `getColumnPreferencesUserKey` falls back
  // to the shared "anonymous" bucket) and then changes once the real id
  // lands. `useState`'s lazy initializer only runs once, so without this it
  // keeps serving the state it loaded under the stale key forever. Re-derive
  // `state` from `localStorage` whenever `key` actually changes, using
  // React's "adjust state during render" pattern (tracked via a second piece
  // of state, not a ref — refs must not be read/written during render)
  // instead of an effect, so there's no extra commit where the table renders
  // the wrong user's columns before catching up.
  const [prevKey, setPrevKey] = useState(key);
  if (prevKey !== key) {
    setPrevKey(key);
    setState(reconcile(loadPersisted(key), columns, defaultVisibleIds));
  }

  const update = useCallback(
    (next: PersistedColumnState) => {
      setState(next);
      savePersisted(key, next);
    },
    [key],
  );

  const columnsById = useMemo(() => {
    const map = new Map<string, ColumnOption>();
    columns.forEach((c) => map.set(c.id, c));
    return map;
  }, [columns]);

  const allColumns = useMemo(
    () => state.order.map((id) => columnsById.get(id)).filter((c): c is ColumnOption => !!c),
    [state.order, columnsById],
  );

  const visibleSet = useMemo(() => new Set(state.visible), [state.visible]);

  const visibleColumns = useMemo(
    () => allColumns.filter((c) => visibleSet.has(c.id)),
    [allColumns, visibleSet],
  );

  const isVisible = useCallback((id: string) => visibleSet.has(id), [visibleSet]);

  const toggleColumn = useCallback(
    (id: string) => {
      const currentlyVisible = state.visible.includes(id);
      if (currentlyVisible && state.visible.length <= 1) return; // never zero columns
      const visible = currentlyVisible
        ? state.visible.filter((v) => v !== id)
        : [...state.visible, id];
      update({ order: state.order, visible });
    },
    [state, update],
  );

  const moveColumn = useCallback(
    (id: string, direction: "up" | "down") => {
      const index = state.order.indexOf(id);
      if (index === -1) return;
      const swapWith = direction === "up" ? index - 1 : index + 1;
      if (swapWith < 0 || swapWith >= state.order.length) return;
      const order = [...state.order];
      [order[index], order[swapWith]] = [order[swapWith], order[index]];
      update({ order, visible: state.visible });
    },
    [state, update],
  );

  const reorderColumn = useCallback(
    (id: string, targetIndex: number) => {
      const currentIndex = state.order.indexOf(id);
      if (currentIndex === -1) return;
      const clampedIndex = Math.max(0, Math.min(targetIndex, state.order.length - 1));
      if (clampedIndex === currentIndex) return;
      const order = [...state.order];
      order.splice(currentIndex, 1);
      order.splice(clampedIndex, 0, id);
      update({ order, visible: state.visible });
    },
    [state, update],
  );

  const resetToDefault = useCallback(() => {
    update({ order: columns.map((c) => c.id), visible: [...defaultVisibleIds] });
  }, [columns, defaultVisibleIds, update]);

  return {
    allColumns,
    visibleColumns,
    isVisible,
    toggleColumn,
    moveColumn,
    reorderColumn,
    resetToDefault,
  };
}
