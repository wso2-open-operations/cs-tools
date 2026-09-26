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

import type { SavedFilterListKey } from "@features/saved-filter-views/legacyStorage";

/** List page for each saved-filter list. Problems is not a case-tab route. */
const LIST_PATHS: Record<SavedFilterListKey, string> = {
  cases: "/cases",
  incidents: "/operations/incidents",
  change_requests: "/operations/change-requests",
  problems: "/operations/problems",
};

export type ParsedFilterLink = { ok: true; qs: string } | { ok: false; error: string };

/**
 * Page URL another user can paste to save the same filter. The search POST
 * URL does not carry the filter, so this is the list page plus `qs`.
 */
export function shareUrl(listKey: SavedFilterListKey, qs: string, origin?: string): string {
  const base = (origin ?? window.location.origin).replace(/\/$/, "");
  const path = LIST_PATHS[listKey];
  const query = qs.replace(/^\?/, "");
  if (!query) return `${base}${path}`;
  return `${base}${path}?${query}`;
}

/**
 * Reads a pasted filter link. A full URL yields its query string (empty is
 * allowed: that view shows every record). A bare `key=value` string is kept
 * as-is. Anything else is rejected.
 */
export function qsFromPastedFilter(text: string, listKey: SavedFilterListKey): ParsedFilterLink {
  const trimmed = text.trim();
  if (!trimmed) {
    return { ok: false, error: "Paste a filter link." };
  }
  if (/^https?:\/\//i.test(trimmed)) {
    try {
      const url = new URL(trimmed);
      if (url.pathname.replace(/\/$/, "") !== LIST_PATHS[listKey]) {
        return { ok: false, error: "That link is for a different list." };
      }
      return { ok: true, qs: url.search.replace(/^\?/, "") };
    } catch {
      return { ok: false, error: "That link doesn't contain a filter." };
    }
  }
  const qs = trimmed.replace(/^\?/, "");
  if (!qs.includes("=")) {
    return { ok: false, error: "That link doesn't contain a filter." };
  }
  return { ok: true, qs };
}
