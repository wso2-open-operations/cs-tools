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

import { type CsmNavNode, type CsmNavSection, navNodeById } from "@config/csmNavItems";

/**
 * Checks whether a given navigation node is visible to a user holding `userRoles`.
 * A node with no `roles` requirement is visible to all authenticated users unless
 * one of its ancestor nodes requires roles the user lacks.
 * When `roles` are specified, the user must hold at least one matching role (case-insensitive).
 */
export function isNavNodeAuthorized(
  node: CsmNavNode,
  userRoles: string[],
): boolean {
  if (node.roles && node.roles.length > 0) {
    const normalizedUserRoles = new Set(
      userRoles.map((r) => r.trim().toLowerCase()),
    );
    const matches = node.roles.some((r) =>
      normalizedUserRoles.has(r.trim().toLowerCase()),
    );
    if (!matches) {
      return false;
    }
  }

  // If this is a nested child (e.g. "admin.user-management.users"), ensure its parent is also authorized.
  const dotIndex = node.id.lastIndexOf(".");
  if (dotIndex > 0) {
    const parentId = node.id.slice(0, dotIndex);
    const parentNode = navNodeById(parentId);
    if (parentNode && !isNavNodeAuthorized(parentNode, userRoles)) {
      return false;
    }
  }

  return true;
}

/**
 * Recursively filters the navigation section tree according to the user's roles.
 *
 * Nodes requiring roles the user does not possess are pruned. If a parent section
 * has children, its child list is also filtered.
 */
export function filterNavItemsByRoles(
  sections: CsmNavSection[],
  userRoles: string[],
): CsmNavSection[] {
  const filterNode = (node: CsmNavNode): CsmNavNode | null => {
    if (!isNavNodeAuthorized(node, userRoles)) {
      return null;
    }
    if (!node.children || node.children.length === 0) {
      return node;
    }
    const filteredChildren = node.children
      .map(filterNode)
      .filter((child): child is CsmNavNode => child !== null);

    return {
      ...node,
      children: filteredChildren,
    };
  };

  return sections
    .map((section) => filterNode(section) as CsmNavSection | null)
    .filter((section): section is CsmNavSection => section !== null);
}
