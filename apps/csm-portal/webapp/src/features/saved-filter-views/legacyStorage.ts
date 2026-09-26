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

import type { BeSavedFilterListKey, BeSavedFilterView } from "@api/backend/types";

export type SavedFilterListKey = BeSavedFilterListKey;
export type SavedFilterView = BeSavedFilterView;

/** Legacy localStorage keys from the pre-Postgres saved-views client. */
export const LEGACY_SAVED_FILTER_STORAGE_KEYS: Record<SavedFilterListKey, string> = {
  cases: "csm.savedFilters.v1",
  incidents: "csm.savedFilters.incidents.v1",
  change_requests: "csm.savedFilters.changeRequests.v1",
  problems: "csm.savedFilters.problems.v1",
};

export function readLegacySavedFilterViews(listKey: SavedFilterListKey): SavedFilterView[] {
  try {
    const raw = localStorage.getItem(LEGACY_SAVED_FILTER_STORAGE_KEYS[listKey]);
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

export function writeLegacySavedFilterViews(
  listKey: SavedFilterListKey,
  views: SavedFilterView[],
): void {
  if (views.length === 0) {
    clearLegacySavedFilterViews(listKey);
    return;
  }
  try {
    localStorage.setItem(LEGACY_SAVED_FILTER_STORAGE_KEYS[listKey], JSON.stringify(views));
  } catch {
    /* ignore */
  }
}

export function clearLegacySavedFilterViews(listKey: SavedFilterListKey): void {
  try {
    localStorage.removeItem(LEGACY_SAVED_FILTER_STORAGE_KEYS[listKey]);
  } catch {
    /* ignore */
  }
}
