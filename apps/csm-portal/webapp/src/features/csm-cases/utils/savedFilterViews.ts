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
 * Named, reusable cases-list filter sets for high-volume triage ("my open
 * S1/S2"). Stored as the serialized query string produced by
 * {@link casesFiltersUrl} — the URL stays the single source of truth, so a saved
 * view is just a label pointing at a `/cases?<qs>` shape. No second filter
 * format is invented.
 */
export interface SavedFilterView {
  name: string;
  /** Serialized filters: the `/cases?<qs>` query string (no leading `?`). */
  qs: string;
}

const STORAGE_KEY = "csm.savedFilters.v1";
const STORAGE_EVENT = "csm:saved-filters-changed";
const MAX_VIEWS = 50;

function readStorage(): SavedFilterView[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
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
    localStorage.setItem(STORAGE_KEY, JSON.stringify(views.slice(0, MAX_VIEWS)));
    // In-tab listeners (the storage event only fires cross-tab).
    window.dispatchEvent(new CustomEvent(STORAGE_EVENT));
  } catch {
    // ignore quota / serialization errors
  }
}

/** Reactive list of the user's saved views (updates across components + tabs). */
export function useSavedFilterViews(): SavedFilterView[] {
  const [views, setViews] = useState<SavedFilterView[]>(() => readStorage());
  useEffect(() => {
    const sync = () => setViews(readStorage());
    window.addEventListener(STORAGE_EVENT, sync);
    window.addEventListener("storage", sync);
    return () => {
      window.removeEventListener(STORAGE_EVENT, sync);
      window.removeEventListener("storage", sync);
    };
  }, []);
  return views;
}

/**
 * Save (or overwrite by name) a view. Most-recently-saved first. No-op for an
 * empty name. Returns the trimmed name actually stored.
 */
export function saveFilterView(name: string, qs: string): string {
  const trimmed = name.trim();
  if (!trimmed) return "";
  const current = readStorage().filter(
    (v) => v.name.toLowerCase() !== trimmed.toLowerCase(),
  );
  writeStorage([{ name: trimmed, qs }, ...current]);
  return trimmed;
}

/** Delete a saved view by name (case-insensitive). */
export function deleteFilterView(name: string): void {
  writeStorage(
    readStorage().filter(
      (v) => v.name.toLowerCase() !== name.trim().toLowerCase(),
    ),
  );
}

/**
 * Move a saved view one position up or down in display order (array index
 * order IS display order — no separate "position" field). A no-op if the
 * view isn't found, or is already at the boundary in that direction (moving
 * the first view up, or the last view down).
 */
export function moveFilterView(name: string, direction: "up" | "down"): void {
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
