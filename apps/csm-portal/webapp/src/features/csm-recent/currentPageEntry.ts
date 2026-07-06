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

import { CSM_NAV_ITEMS, navItemForPath } from "@config/csmNavItems";
import type { RecentView } from "@features/csm-recent/hooks/useRecentViews";

type PageEntry = Omit<RecentView, "visitedAt" | "pinned">;

/** Title-case a path segment: "security-center" -> "Security center". */
function humanizeSegment(segment: string): string {
  const words = segment.replace(/[-_]+/g, " ").trim();
  if (!words) return "Page";
  return words.charAt(0).toUpperCase() + words.slice(1);
}

/** Short, human summary of a filter query string for a "search" label. */
function summarizeQuery(search: string): string {
  const params = new URLSearchParams(search);
  const q = params.get("q") || params.get("query") || params.get("search");
  if (q) return `“${q}”`;
  const keys = [...params.keys()].filter((k) => params.get(k));
  if (keys.length === 0) return "filtered";
  return `${keys.length} filter${keys.length > 1 ? "s" : ""}`;
}

/**
 * Build a pinnable Recent View for an arbitrary current route, so "Pin this
 * page" works anywhere — not just on case/project/account detail pages (those
 * already record richer entries on visit).
 *
 * - A known nav page with a filter query -> a "search" (e.g. a saved Cases view)
 * - A known nav page with no query        -> a "page"
 * - Anything else                          -> a best-effort "page"
 *
 * Detail routes are deliberately handled by their own recorders; callers should
 * prefer an existing recorded entry whose `href` matches before falling back to
 * this. `id` is the full href so distinct filtered views pin separately.
 */
export function currentPageEntry(pathname: string, search: string): PageEntry {
  const href = pathname + search;
  const nav = navItemForPath(pathname);
  const onNavRoot = nav && pathname === nav.path;

  if (search && search !== "?") {
    const base = onNavRoot ? nav.label : humanizeSegment(pathname.split("/")[1] ?? "");
    return {
      kind: "search",
      id: href,
      title: `${base}: ${summarizeQuery(search)}`,
      href,
    };
  }

  if (onNavRoot) {
    return { kind: "page", id: nav.path, title: nav.label, href: nav.path };
  }

  // Unknown route (or a sub-route we don't have a recorder for): label from the
  // first path segment so the pin is still recognisable.
  const seg = pathname.split("/").filter(Boolean)[0] ?? "";
  return {
    kind: "page",
    id: pathname,
    title: humanizeSegment(seg) || "Page",
    href: pathname,
  };
}

/** True when the current route maps onto one of the known nav pages. */
export function isKnownPage(pathname: string): boolean {
  return CSM_NAV_ITEMS.some((i) => pathname === i.path);
}
