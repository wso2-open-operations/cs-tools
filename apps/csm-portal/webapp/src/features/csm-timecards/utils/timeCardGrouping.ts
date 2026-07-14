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

import type { CsmTimeCard } from "@features/csm-timecards/types/timeCards";

/** Which field a `TimeCardsTable` clusters its rows by — a display-only
 * grouping, purely client-side (see `groupTimeCards`). */
export type TimeCardGroupBy = "case" | "engineer";

/** A visual cluster of cards sharing a case or an engineer, newest first. */
export interface TimeCardGroup {
  /** `caseId` or `userId`, whichever `groupBy` was grouped on. */
  key: string;
  /** Case number or engineer name, shown as the group's section header. */
  label: string;
  /** This group's cards, newest work date first. */
  cards: CsmTimeCard[];
  /** Whole minutes, summed from `cards`. */
  totalMinutes: number;
}

/**
 * Groups a flat list of cards by case or by engineer for display. Purely a
 * frontend clustering — the backend has no such concept, cards already come
 * back flat from `searchTimeCards`. Cards with an unparseable `workDate` sort
 * last within their group rather than being dropped (unlike the old weekly
 * grouping, there's no date-keyed bucket here for a bad date to break).
 */
export function groupTimeCards(cards: CsmTimeCard[], groupBy: TimeCardGroupBy): TimeCardGroup[] {
  const byKey = new Map<string, TimeCardGroup>();
  for (const c of cards) {
    const key = groupBy === "case" ? c.caseId || c.caseNumber : c.userId;
    const label = groupBy === "case" ? c.caseNumber : c.userName;
    let group = byKey.get(key);
    if (!group) {
      group = { key, label, cards: [], totalMinutes: 0 };
      byKey.set(key, group);
    }
    group.cards.push(c);
    group.totalMinutes += c.totalMinutes;
  }

  const groups = [...byKey.values()];
  for (const g of groups) {
    g.cards.sort((a, b) => b.workDate.localeCompare(a.workDate));
  }
  // Most recently active group first — by its own newest card's work date.
  groups.sort((a, b) => (b.cards[0]?.workDate ?? "").localeCompare(a.cards[0]?.workDate ?? ""));
  return groups;
}
