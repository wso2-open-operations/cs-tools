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

import type {
  CsmTimeCard,
  CsmTimeSheet,
  TimeSheetState,
} from "@features/csm-timecards/types/timeCards";
import { weekEndOf, weekStartOf } from "@features/csm-timecards/utils/timeSheetWeek";
import { roundHours } from "@features/csm-timecards/utils/timeCardTotals";

/** Roll a week's cards up into a single display status. */
export function sheetStatus(cards: CsmTimeCard[]): TimeSheetState {
  if (cards.some((c) => c.state === "rejected")) return "rejected";
  if (cards.every((c) => c.state === "approved" || c.state === "processed")) {
    return "approved";
  }
  return "submitted";
}

/**
 * Group a flat list of cards into weekly sheets (Mon–Sun), newest first.
 * `weekStartOf` throws (`RangeError: Invalid time value`) if `createdOn`
 * isn't a parseable date — confirmed live: at least one real card has one
 * (blank or otherwise malformed). Skip that one card rather than losing the
 * whole group of otherwise-good cards to it — the same "one bad record
 * shouldn't sink the rest" issue already worked around for search
 * pagination (see `searchTimeCards` in `useTimeSheets.ts`), just on the
 * client this time.
 */
export function groupIntoSheets(
  cards: CsmTimeCard[],
  userId: string,
  userName: string,
): CsmTimeSheet[] {
  const byWeek = new Map<string, CsmTimeCard[]>();
  for (const c of cards) {
    let wk: string;
    try {
      wk = weekStartOf(c.createdOn);
    } catch {
      continue;
    }
    const bucket = byWeek.get(wk);
    if (bucket) bucket.push(c);
    else byWeek.set(wk, [c]);
  }
  const sheets: CsmTimeSheet[] = [];
  for (const [weekStart, weekCards] of byWeek) {
    weekCards.sort((a, b) => b.createdOn.localeCompare(a.createdOn));
    sheets.push({
      id: `${userId}:${weekStart}`,
      userId,
      userName,
      weekStart,
      weekEnd: weekEndOf(weekStart),
      state: sheetStatus(weekCards),
      cards: weekCards,
      totalHours: roundHours(weekCards.reduce((sum, c) => sum + c.totalHours, 0)),
    });
  }
  return sheets.sort((a, b) => b.weekStart.localeCompare(a.weekStart));
}
