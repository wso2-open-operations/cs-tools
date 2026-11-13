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

import { buildCategoryTree, type CategoryNode } from "@features/csm-skills/utils/categoryTree";
import type { Skill, SkillCategory } from "@features/csm-skills/types";

/** A category in the All Skills tree, with the skills filed directly under it. */
export interface SkillTreeNode {
  /** The category id, or UNCATEGORISED_ID for skills filed nowhere. */
  id: string;
  name: string;
  children: SkillTreeNode[];
  skills: Skill[];
  /** Skills here and in every sub-category. */
  total: number;
}

export const UNCATEGORISED_ID = "__uncategorised__";

const byName = (a: Skill, b: Skill): number => a.name.localeCompare(b.name);

/**
 * Files each skill under every category it is linked to, keeps only the
 * branches that hold at least one of the given skills, and adds an
 * "Uncategorised" node for skills filed nowhere (or under a category the
 * caller cannot see).
 */
export function buildSkillTree(categories: readonly SkillCategory[], skills: readonly Skill[]): SkillTreeNode[] {
  const known = new Set(categories.map((c) => c.id));
  const byCategory = new Map<string, Skill[]>();
  const unfiled: Skill[] = [];
  for (const skill of skills) {
    const homes = skill.categories.filter((c) => known.has(c.id));
    if (homes.length === 0) unfiled.push(skill);
    for (const c of homes) byCategory.set(c.id, [...(byCategory.get(c.id) ?? []), skill]);
  }

  const convert = (node: CategoryNode): SkillTreeNode | null => {
    const children = node.children.map(convert).filter((n): n is SkillTreeNode => n !== null);
    const own = [...(byCategory.get(node.category.id) ?? [])].sort(byName);
    const total = own.length + children.reduce((sum, c) => sum + c.total, 0);
    if (total === 0) return null;
    return { id: node.category.id, name: node.category.name, children, skills: own, total };
  };

  const roots = buildCategoryTree(categories)
    .map(convert)
    .filter((n): n is SkillTreeNode => n !== null);
  if (unfiled.length > 0) {
    roots.push({ id: UNCATEGORISED_ID, name: "Uncategorised", children: [], skills: [...unfiled].sort(byName), total: unfiled.length });
  }
  return roots;
}

/** The ids of every category node, for "expand all". */
export function allNodeIds(nodes: readonly SkillTreeNode[]): string[] {
  return nodes.flatMap((n) => [n.id, ...allNodeIds(n.children)]);
}

/** The category ids on the way to the first place a skill is filed, for opening the tree at it. */
export function pathToSkill(nodes: readonly SkillTreeNode[], skillId: string): string[] | null {
  for (const n of nodes) {
    if (n.skills.some((s) => s.id === skillId)) return [n.id];
    const below = pathToSkill(n.children, skillId);
    if (below) return [n.id, ...below];
  }
  return null;
}
