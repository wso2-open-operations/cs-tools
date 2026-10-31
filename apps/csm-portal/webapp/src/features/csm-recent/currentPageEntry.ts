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

import {
  flattenNavNodes,
  navNodeMatchForPath,
  navNodeRoutes,
  navTabForSearch,
  navViewForSearch,
} from "@config/csmNavItems";
import type { RecentView } from "@features/csm-recent/hooks/useRecentViews";

type PageEntry = Omit<RecentView, "visitedAt" | "pinned">;

/** Title-case a path segment: "security-center" -> "Security center". */
function humanizeSegment(segment: string): string {
  const words = segment.replace(/[-_]+/g, " ").trim();
  if (!words) return "Page";
  return words.charAt(0).toUpperCase() + words.slice(1);
}

/**
 * `search` with `names` removed, once they have been resolved to a nav node
 * (the `tab` parameter) or to a named place inside one (a node's `views`).
 * Either selects the destination rather than filtering it, so counting it as a
 * filter would label the Incidents tab "Operations: 1 filter" and Standard
 * changes "Change requests: 1 filter".
 */
function searchWithout(search: string, names: readonly string[]): string {
  const params = new URLSearchParams(search);
  names.forEach((name) => params.delete(name));
  const rest = params.toString();
  return rest ? `?${rest}` : "";
}

/** `search` with every parameter whose name starts with `prefix` removed. */
function searchWithoutPrefix(search: string, prefix: string): string {
  const params = new URLSearchParams(search);
  [...new Set(params.keys())]
    .filter((name) => name.startsWith(prefix))
    .forEach((name) => params.delete(name));
  const rest = params.toString();
  return rest ? `?${rest}` : "";
}

/** `href` with each `[param, value]` set on its query, in order. */
function withParams(href: string, pairs: ReadonlyArray<readonly [string, string]>): string {
  const [path, query = ""] = href.split("?");
  const params = new URLSearchParams(query);
  pairs.forEach(([param, value]) => params.set(param, value));
  return `${path}?${params.toString()}`;
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
  // Match the most specific node, so a second-level tab pins under its own name
  // ("Accounts") rather than its section's ("Customers"). The matched prefix is
  // the node's canonical path here — a query-param tab's `href` points at the
  // section landing route, which is not what was navigated to.
  const match = navNodeMatchForPath(pathname);
  const onNavRoot = match && pathname === match.prefix;

  // A `?tab=` value names a destination, not a filter, and is more specific
  // than the section it sits on — so resolve it before the rest of the query is
  // read as filters. Its canonical href keeps the tab, so each tab pins
  // separately.
  const tab = onNavRoot ? navTabForSearch(match.node, search) : undefined;
  const node = tab ?? (onNavRoot ? match.node : undefined);
  const tabSearch = tab ? searchWithout(search, ["tab"]) : search;

  // Likewise a named place inside the page (a node's `views`: Standard changes
  // inside Change requests, and its proposals) is the destination, named after
  // itself, with its own canonical href so each place pins separately. The
  // parameters such a place keeps for the page's default one (the change
  // request list's `cr*` filters on Standard changes) are not its filters:
  // they are left out of its title and of its href alike.
  const view = node ? navViewForSearch(node, tabSearch) : undefined;
  const ignored = view?.ignoreParamPrefix;
  const tabFilters = ignored ? searchWithoutPrefix(tabSearch, ignored) : tabSearch;
  const href = pathname + (ignored ? searchWithoutPrefix(search, ignored) : search);
  const nodeHref = tab ? tab.href : match?.prefix;
  const canonicalHref =
    view && nodeHref ? withParams(nodeHref, view.params) : nodeHref;
  const label = view ? view.label : node?.label;
  const filters = view
    ? searchWithout(tabFilters, view.params.map(([param]) => param))
    : tabFilters;

  if (filters && filters !== "?") {
    const base = label ?? humanizeSegment(pathname.split("/")[1] ?? "");
    return {
      kind: "search",
      id: href,
      title: `${base}: ${summarizeQuery(filters)}`,
      href,
    };
  }

  if (label && canonicalHref) {
    return {
      kind: "page",
      id: canonicalHref,
      title: label,
      href: canonicalHref,
    };
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

/**
 * True when the current route maps onto one of the known nav pages — a section
 * landing route or a second-level tab's own route.
 */
export function isKnownPage(pathname: string): boolean {
  return flattenNavNodes().some((node) =>
    navNodeRoutes(node).includes(pathname),
  );
}
