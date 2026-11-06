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

/**
 * The daily timesheet window, in India Standard Time.
 *
 * The CS Worklogs daily report reads the roster and leave of the day before
 * when it runs: it is due at 09:25 IST and the scheduled tasks start every 15
 * minutes, so it reads the roster at the 09:30 IST start. A change made around
 * then can be missing from the report, so leads are asked not to edit the
 * roster from 09:15 to 09:30 IST. Move these two numbers if the report's
 * schedule moves.
 */
export const ROSTER_FREEZE_START_MINUTE = 9 * 60 + 15;
export const ROSTER_FREEZE_END_MINUTE = 9 * 60 + 30;

/** IST is UTC+05:30 all year (no daylight saving). */
const IST_OFFSET_MINUTES = 5 * 60 + 30;
const DAY_MINUTES = 24 * 60;
const MINUTE_MS = 60_000;

/** The minute of the day in IST, with its fraction (0 to 1439.99). */
function istMinuteOfDay(now: Date): number {
  const utcMinutes = now.getUTCHours() * 60 + now.getUTCMinutes() + now.getUTCSeconds() / 60 + now.getUTCMilliseconds() / MINUTE_MS;
  return (utcMinutes + IST_OFFSET_MINUTES) % DAY_MINUTES;
}

/** Whether the roster should not be edited now: 09:15 to 09:30 IST, 09:30 excluded. */
export function isRosterFreeze(now: Date): boolean {
  const m = istMinuteOfDay(now);
  return m >= ROSTER_FREEZE_START_MINUTE && m < ROSTER_FREEZE_END_MINUTE;
}

/** Milliseconds until the window next opens or closes, whichever comes first. */
export function msUntilRosterFreezeChange(now: Date): number {
  const m = istMinuteOfDay(now);
  const next = [ROSTER_FREEZE_START_MINUTE, ROSTER_FREEZE_END_MINUTE, ROSTER_FREEZE_START_MINUTE + DAY_MINUTES]
    .map((b) => b - m)
    .filter((d) => d > 0);
  return Math.max(1, Math.ceil(Math.min(...next) * MINUTE_MS));
}

/** "9:30 AM" for a minute of the day. */
export function fmtIstMinute(minute: number): string {
  const h = Math.floor(minute / 60);
  const mm = String(minute % 60).padStart(2, "0");
  return `${((h + 11) % 12) + 1}:${mm} ${h < 12 ? "AM" : "PM"}`;
}
