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

import { listFeatureNodes, type FeatureNodeInfo, type StoredFeatureState } from "@config/featureFlags";

/** `nav-node id -> state`: only the pages that have an entry. A page with none follows its section. */
export type FeatureMap = Record<string, StoredFeatureState>;

export type FeatureFilter = "all" | StoredFeatureState | "changed";

export const STATE_LABEL: Record<StoredFeatureState, string> = {
  enabled: "Available",
  preview: "Preview",
};

/** What picking each state does, shown under its name in the menus. Different words for a section and a page. */
export const STATE_HELP: Record<"section" | "page", Record<StoredFeatureState, string>> = {
  section: {
    enabled: "Everyone can open this section.",
    preview:
      "Admins can open it, with a Preview tag. Everyone else sees it greyed out unless a page in it is Available.",
  },
  page: {
    enabled: "Everyone can open this page, even if its section is in preview.",
    preview: "Admins can open it, with a Preview tag. Everyone else sees it greyed out.",
  },
};

/**
 * Every page's state after inheritance: its own entry, else its section's (and
 * so on up), else Available. This is what the list shows. The menu rule that
 * keeps a Preview section open because one of its pages is Available is not a
 * state: it is said in words by {@link describeFeatureRow}.
 */
export function effectiveFeatureStates(
  nodes: readonly FeatureNodeInfo[],
  map: FeatureMap,
): Map<string, StoredFeatureState> {
  const byId = new Map(nodes.map((node) => [node.id, node]));
  const out = new Map<string, StoredFeatureState>();
  const resolve = (node: FeatureNodeInfo): StoredFeatureState => {
    const known = out.get(node.id);
    if (known) return known;
    const own = map[node.id];
    const parent = node.parentId ? byId.get(node.parentId) : undefined;
    const state = own ?? (parent ? resolve(parent) : "enabled");
    out.set(node.id, state);
    return state;
  };
  for (const node of nodes) resolve(node);
  return out;
}

/** A section and the rows to show under it. */
export interface FeatureSectionView {
  section: FeatureNodeInfo;
  /** Everything below the section, in nav order, after search and filter. */
  rows: FeatureNodeInfo[];
}

/**
 * The list the page draws: every section with its pages, narrowed by the
 * search box and the Show filter. A page matches a search through its own name
 * or any ancestor's, so typing a section's name keeps all of its pages in
 * view. A section stays when it matches or any page under it does.
 */
export function filterFeatureTree(
  nodes: readonly FeatureNodeInfo[],
  map: FeatureMap,
  changed: ReadonlySet<string>,
  query: string,
  filter: FeatureFilter,
): FeatureSectionView[] {
  const states = effectiveFeatureStates(nodes, map);
  const byId = new Map(nodes.map((node) => [node.id, node]));
  const needle = query.trim().toLowerCase();

  const passesFilter = (node: FeatureNodeInfo): boolean => {
    if (filter === "all") return true;
    if (filter === "changed") return changed.has(node.id);
    return states.get(node.id) === filter;
  };
  const passesQuery = (node: FeatureNodeInfo): boolean => {
    if (!needle) return true;
    for (let current: FeatureNodeInfo | undefined = node; current; current = current.parentId ? byId.get(current.parentId) : undefined) {
      if (current.label.toLowerCase().includes(needle)) return true;
    }
    return false;
  };
  const matches = (node: FeatureNodeInfo): boolean => passesFilter(node) && passesQuery(node);
  const filtering = needle !== "" || filter !== "all";

  const out: FeatureSectionView[] = [];
  for (const section of nodes.filter((node) => node.depth === 0)) {
    const below: FeatureNodeInfo[] = [];
    const walk = (parentId: string): void => {
      for (const node of nodes) {
        if (node.parentId !== parentId) continue;
        below.push(node);
        walk(node.id);
      }
    };
    walk(section.id);
    const shown = filtering ? below.filter(matches) : below;
    if (matches(section) || shown.length > 0) out.push({ section, rows: shown });
  }
  return out;
}

/** The page's own starting point: every page in the nav tree. */
export function allFeatureNodes(): FeatureNodeInfo[] {
  return listFeatureNodes();
}
