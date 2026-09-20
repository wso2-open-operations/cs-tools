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

import { useEffect, useState } from "react";

/**
 * Named, reusable filter set for an Operations sub-tab (Change Requests,
 * Incidents, Problems). Same shape as the Cases list's own
 * `SavedFilterView` (`csm-cases/utils/savedFilterViews.ts`) — a saved view is
 * just a label pointing at that tab's own serialized query string, so the
 * URL stays the single source of truth and no second filter format is
 * invented.
 */
export interface SavedFilterView {
  name: string;
  /** Serialized filters: the tab's own query string (no leading `?`). */
  qs: string;
}

/** The reactive-hook + mutator surface a saved-views-backed `localStorage`
 * key exposes — one instance per Operations sub-tab, each on its own key
 * (see `changeRequestsSavedViews.ts` / `incidentsSavedViews.ts` /
 * `problemsSavedViews.ts`), so views never leak across tabs. */
export interface SavedFilterViewsStore {
  /** Reactive list of the user's saved views for this tab (updates across
   * components + browser tabs). */
  useSavedFilterViews: () => SavedFilterView[];
  /**
   * Save (or overwrite by name) a view. Most-recently-saved first. No-op for
   * an empty name. Returns the trimmed name actually stored.
   */
  saveFilterView: (name: string, qs: string) => string;
  /** Delete a saved view by name (case-insensitive). */
  deleteFilterView: (name: string) => void;
  /**
   * Move a saved view one position up or down in display order (array index
   * order IS display order — no separate "position" field). A no-op if the
   * view isn't found, or is already at the boundary in that direction.
   */
  moveFilterView: (name: string, direction: "up" | "down") => void;
}

const MAX_VIEWS = 50;

/**
 * Build an independent, `localStorage`-backed saved-views store scoped to
 * `storageKey`. One call site per Operations sub-tab — each gets its own key
 * and its own "Saved views" menu, mirroring the Cases list's
 * `savedFilterViews.ts` (which this deliberately does not touch: that file
 * keeps working exactly as today, on its own `csm.savedFilters.v1` key).
 */
export function createSavedFilterViewsStore(storageKey: string): SavedFilterViewsStore {
  const storageEvent = `csm:saved-filters-changed:${storageKey}`;

  function readStorage(): SavedFilterView[] {
    try {
      const raw = localStorage.getItem(storageKey);
      if (!raw) return [];
      const parsed: unknown = JSON.parse(raw);
      if (!Array.isArray(parsed)) return [];
      return parsed.filter(
        (v): v is SavedFilterView =>
          typeof v === "object" &&
          v !== null &&
          typeof (v as SavedFilterView).name === "string" &&
          typeof (v as SavedFilterView).qs === "string",
      );
    } catch {
      return [];
    }
  }

  function writeStorage(views: SavedFilterView[]): void {
    try {
      localStorage.setItem(storageKey, JSON.stringify(views.slice(0, MAX_VIEWS)));
      // In-tab listeners (the storage event only fires cross-tab).
      window.dispatchEvent(new CustomEvent(storageEvent));
    } catch {
      // ignore quota / serialization errors
    }
  }

  function useSavedFilterViews(): SavedFilterView[] {
    const [views, setViews] = useState<SavedFilterView[]>(() => readStorage());
    useEffect(() => {
      const sync = () => setViews(readStorage());
      window.addEventListener(storageEvent, sync);
      window.addEventListener("storage", sync);
      return () => {
        window.removeEventListener(storageEvent, sync);
        window.removeEventListener("storage", sync);
      };
    }, []);
    return views;
  }

  function saveFilterView(name: string, qs: string): string {
    const trimmed = name.trim();
    if (!trimmed) return "";
    const current = readStorage().filter(
      (v) => v.name.toLowerCase() !== trimmed.toLowerCase(),
    );
    writeStorage([{ name: trimmed, qs }, ...current]);
    return trimmed;
  }

  function deleteFilterView(name: string): void {
    writeStorage(
      readStorage().filter((v) => v.name.toLowerCase() !== name.trim().toLowerCase()),
    );
  }

  function moveFilterView(name: string, direction: "up" | "down"): void {
    const current = readStorage();
    const index = current.findIndex(
      (v) => v.name.toLowerCase() === name.trim().toLowerCase(),
    );
    if (index === -1) return;
    const target = direction === "up" ? index - 1 : index + 1;
    if (target < 0 || target >= current.length) return;
    const reordered = [...current];
    [reordered[index], reordered[target]] = [reordered[target], reordered[index]];
    writeStorage(reordered);
  }

  return { useSavedFilterViews, saveFilterView, deleteFilterView, moveFilterView };
}
