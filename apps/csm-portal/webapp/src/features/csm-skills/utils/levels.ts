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

import type { SkillLevel, SkillLevelType } from "@features/csm-skills/types";

/** A scale's levels from lowest to highest value; levels without a value sort last, then by name. */
export function sortedLevels(levelType: SkillLevelType | undefined): SkillLevel[] {
  if (!levelType) return [];
  return [...levelType.levels].sort((a, b) => {
    const av = a.value ?? Number.POSITIVE_INFINITY;
    const bv = b.value ?? Number.POSITIVE_INFINITY;
    return av === bv ? a.name.localeCompare(b.name) : av - bv;
  });
}

/** The scale with this id, if the levels have loaded and it exists. */
export function findLevelType(
  levelTypes: readonly SkillLevelType[] | undefined,
  id: string | null | undefined,
): SkillLevelType | undefined {
  return id ? levelTypes?.find((t) => t.id === id) : undefined;
}

const CSS_HEX = /^#(?:[0-9a-f]{3}|[0-9a-f]{4}|[0-9a-f]{6}|[0-9a-f]{8})$/i;

/**
 * A level's colour hint, only when it is a plain hex colour. It is data from
 * the backend, used for a small decorative dot next to the level name (the name
 * itself is always the readable label), so anything else is ignored.
 */
export function levelDotColor(level: Pick<SkillLevel, "color"> | null | undefined): string | undefined {
  const color = level?.color?.trim();
  return color && CSS_HEX.test(color) ? color : undefined;
}

/** Every distinct level `value` across all scales, ascending, each with the level names that carry it. */
export function distinctLevelValues(
  levelTypes: readonly SkillLevelType[] | undefined,
): { value: number; names: string[] }[] {
  const byValue = new Map<number, Set<string>>();
  for (const t of levelTypes ?? []) {
    for (const l of t.levels) {
      if (l.value == null) continue;
      const names = byValue.get(l.value) ?? new Set<string>();
      names.add(l.name);
      byValue.set(l.value, names);
    }
  }
  return [...byValue.entries()]
    .sort(([a], [b]) => a - b)
    .map(([value, names]) => ({ value, names: [...names] }));
}
