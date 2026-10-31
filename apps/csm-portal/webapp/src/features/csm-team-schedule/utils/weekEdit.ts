/**
 * Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import type { ScheduleAssignment, ScheduleShift } from "../types";

/** What taking one person off a turn on This week writes for that day. */
export interface WeekClear {
  /** The window the day goes back to; "" leaves the day (or the zone) empty. */
  shiftCode: string;
  /** Set when only this zone's turn is cleared. */
  zoneCode?: string;
  /** How the change reads in the edit bar and the undo offer. */
  what: string;
}

/**
 * The roster's Clear, for one cell of This week. Somebody who holds a turn in
 * another zone (or another window) that day loses only this zone's turn; any
 * other day goes back to their standing window -- the one they hold most this
 * week, else their team's own, else none -- and a weekend day goes back to
 * nothing, because nobody stands on a weekend.
 */
export function weekClear(
  a: ScheduleAssignment,
  shifts: ReadonlyMap<string, ScheduleShift>,
  opts: { holdsTurnElsewhere: boolean; baseShiftCode?: string; teamDefaultShiftCode?: string },
): WeekClear {
  const zone = a.zoneCode ?? shifts.get(a.shiftCode)?.zoneCode;
  if (zone && opts.holdsTurnElsewhere) {
    return { shiftCode: "", zoneCode: zone, what: `off ${zone}${a.tier ? ` ${a.tier}` : ""}` };
  }
  const [y, m, d] = a.rotaDate.split("-").map(Number);
  const day = new Date(y, m - 1, d).getDay();
  const weekend = day === 0 || day === 6;
  return {
    shiftCode: weekend ? "" : (opts.baseShiftCode ?? opts.teamDefaultShiftCode ?? ""),
    what: `off ${shifts.get(a.shiftCode)?.label ?? a.shiftCode}`,
  };
}
