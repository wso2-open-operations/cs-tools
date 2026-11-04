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

import type { RotaReminderKind } from "../types";

/** The clock the reminders run on: "today" is today in Colombo, wherever the reader is. */
const ZONE = "Asia/Colombo";

const DAY_MS = 86_400_000;

const WEEKDAYS = ["Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"];
const MONTHS = [
  "January",
  "February",
  "March",
  "April",
  "May",
  "June",
  "July",
  "August",
  "September",
  "October",
  "November",
  "December",
];

const calendarDay = new Intl.DateTimeFormat("en-US", {
  timeZone: ZONE,
  year: "numeric",
  month: "2-digit",
  day: "2-digit",
});

/** Midnight UTC of the calendar day it is in Colombo at `now`, in ms. */
function colomboToday(now: Date): number {
  const parts = calendarDay.formatToParts(now);
  const part = (type: string): number => Number(parts.find((p) => p.type === type)?.value);
  return Date.UTC(part("year"), part("month") - 1, part("day"));
}

function iso(ms: number): string {
  return new Date(ms).toISOString().slice(0, 10);
}

/**
 * The days a reminder is about, as YYYY-MM-DD, counted from today in Asia/Colombo.
 *
 * morning, evening and weekday: the next working day after today (Saturday and
 * Sunday are skipped, so Friday gives Monday).
 * weekend: Monday to Friday give the coming Saturday and Sunday; Saturday
 * gives today and tomorrow; Sunday gives yesterday and today.
 */
export function reminderDays(kind: RotaReminderKind, now: Date): string[] {
  const today = colomboToday(now);
  const weekday = new Date(today).getUTCDay();

  if (kind !== "weekend") {
    const ahead = weekday === 5 ? 3 : weekday === 6 ? 2 : 1;
    return [iso(today + ahead * DAY_MS)];
  }
  if (weekday === 6) return [iso(today), iso(today + DAY_MS)];
  if (weekday === 0) return [iso(today - DAY_MS), iso(today)];
  const saturday = today + (6 - weekday) * DAY_MS;
  return [iso(saturday), iso(saturday + DAY_MS)];
}

interface DayParts {
  weekday: string;
  day: number;
  month: string;
}

function dayParts(date: string): DayParts {
  const [y, m, d] = date.split("-").map(Number);
  const ms = Date.UTC(y ?? 0, (m ?? 1) - 1, d ?? 1);
  const at = new Date(ms);
  return { weekday: WEEKDAYS[at.getUTCDay()] ?? "", day: at.getUTCDate(), month: MONTHS[at.getUTCMonth()] ?? "" };
}

/**
 * The days in words: "Tuesday, 20 October", or for two days in one month
 * "Saturday, 24 and Sunday, 25 October".
 */
export function describeDays(days: readonly string[]): string {
  const parts = days.map(dayParts);
  const [a, b] = parts;
  if (parts.length === 2 && a && b && a.month === b.month) {
    return `${a.weekday}, ${a.day} and ${b.weekday}, ${b.day} ${a.month}`;
  }
  return parts.map((p) => `${p.weekday}, ${p.day} ${p.month}`).join(" and ");
}
