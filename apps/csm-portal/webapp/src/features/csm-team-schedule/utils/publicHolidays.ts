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

import type { SchedulePublicHoliday } from "../types";

/** A person's public holiday on a day, keyed `${userId}|${YYYY-MM-DD}`. */
export function holidaysByPersonAndDay(holidays: readonly SchedulePublicHoliday[]): Map<string, SchedulePublicHoliday> {
  const out = new Map<string, SchedulePublicHoliday>();
  for (const h of holidays) out.set(`${h.userId}|${h.date}`, h);
  return out;
}

/** The hover line for a holiday, e.g. "Public holiday: Vesak (Sri Lanka)". */
export function holidayNote(h: SchedulePublicHoliday): string {
  return `Public holiday: ${h.title} (${h.country})`;
}

/** A cell's title with the person's holiday that day added, if any. */
export function withHolidayNote(title: string | undefined, h: SchedulePublicHoliday | undefined): string | undefined {
  if (!h) return title;
  return title ? `${title} · ${holidayNote(h)}` : holidayNote(h);
}
