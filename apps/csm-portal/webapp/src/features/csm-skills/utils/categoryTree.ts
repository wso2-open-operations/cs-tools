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

import type { SkillCategory } from "@features/csm-skills/types";

export interface CategoryNode {
  category: SkillCategory;
  children: CategoryNode[];
}

/** A category in depth-first tree order, with how deep it sits (0 = top level). */
export interface FlatCategory {
  category: SkillCategory;
  depth: number;
}

const byName = (a: CategoryNode, b: CategoryNode): number =>
  a.category.name.localeCompare(b.category.name);

/**
 * Builds the category tree from the flat list. A category whose parent is
 * missing from the list (or that is its own ancestor) is treated as a root so
 * it never disappears. Siblings are sorted by name.
 */
export function buildCategoryTree(categories: readonly SkillCategory[]): CategoryNode[] {
  const nodes = new Map<string, CategoryNode>();
  for (const category of categories) nodes.set(category.id, { category, children: [] });

  const roots: CategoryNode[] = [];
  for (const node of nodes.values()) {
    const parent = node.category.parentId ? nodes.get(node.category.parentId) : undefined;
    if (parent && !isAncestor(nodes, node.category.id, parent.category.id)) parent.children.push(node);
    else roots.push(node);
  }

  const sortDeep = (list: CategoryNode[]): void => {
    list.sort(byName);
    for (const n of list) sortDeep(n.children);
  };
  sortDeep(roots);
  return roots;
}

/** True when `candidateId` is `ofId` itself or one of its ancestors (guards against a parent cycle). */
function isAncestor(nodes: Map<string, CategoryNode>, ofId: string, candidateId: string): boolean {
  const seen = new Set<string>();
  let cursor: string | null | undefined = candidateId;
  while (cursor && !seen.has(cursor)) {
    if (cursor === ofId) return true;
    seen.add(cursor);
    cursor = nodes.get(cursor)?.category.parentId;
  }
  return false;
}

/** Depth-first flattening of {@link buildCategoryTree}, for an indented option list. */
export function flattenCategoryTree(roots: readonly CategoryNode[]): FlatCategory[] {
  const out: FlatCategory[] = [];
  const walk = (list: readonly CategoryNode[], depth: number): void => {
    for (const n of list) {
      out.push({ category: n.category, depth });
      walk(n.children, depth + 1);
    }
  };
  walk(roots, 0);
  return out;
}
