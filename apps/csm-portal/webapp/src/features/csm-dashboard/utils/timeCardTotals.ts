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

/**
 * Formats a time card totals widget's hours (`groupBy.measure: "hours"`) for
 * the charts, their tooltips and the tile's total: one decimal at most,
 * then " h" ("12.5 h", "4 h", "0 h").
 */
export function formatHours(hours: number): string {
  return `${(Math.round(hours * 10) / 10).toLocaleString("en-US")} h`;
}

/**
 * A time card "day" bucket's label: the aggregate's `YYYY-MM-DD` key as a
 * short date ("Oct 30"). The key is a calendar day, so it is read as a local
 * date (never through `new Date("YYYY-MM-DD")`, which is UTC midnight and
 * shows the previous day west of UTC). Anything else is returned unchanged.
 */
export function formatDayLabel(key: string): string {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(key);
  if (!m) return key;
  const day = new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3]));
  return day.toLocaleDateString("en-US", { month: "short", day: "numeric" });
}

/**
 * A short label for the top of an MTTR bar, where seven bars share a narrow
 * tile: whole hours from 100 up, one decimal below, no thousands separator
 * ("3436h", "82.1h", "0.3h"). The tooltip keeps the full `formatMttrHours`.
 */
export function formatMttrBarLabel(hours: number): string {
  const rounded = hours >= 100 ? Math.round(hours) : Math.round(hours * 10) / 10;
  return `${rounded}h`;
}

/** An MTTR value in days for the chart tooltip and the count tile: at most
 * two decimals, then " d" ("7.08 d", "0.3 d", "12 d"). */
export function formatMttrDays(days: number): string {
  return `${(Math.round(days * 100) / 100).toLocaleString("en-US", { maximumFractionDigits: 2 })} d`;
}

/** A short days label for the top of an MTTR bar: one decimal, no space
 * ("7.1d", "0.3d", "12d"). */
export function formatMttrDaysBarLabel(days: number): string {
  return `${Math.round(days * 10) / 10}d`;
}

/** The formatters for an MTTR widget's chosen unit (`query.unit`): the full
 * value for the tooltip and count tile, and the short bar label. */
export function mttrUnitFormatters(unit: unknown): {
  value: (n: number) => string;
  label: (n: number) => string;
} {
  return unit === "days"
    ? { value: formatMttrDays, label: formatMttrDaysBarLabel }
    : { value: formatMttrHours, label: formatMttrBarLabel };
}

/** Labels an MTTR widget's header count: "5 cases" ("1 case"). */
export function formatMttrCases(count: number): string {
  return `${count.toLocaleString("en-US")} ${count === 1 ? "case" : "cases"}`;
}

/**
 * Formats an average resolution time (MTTR, in hours) for a chart, its
 * tooltip and the tile's total: at most two decimals, thousands separated,
 * then " h" ("1,259.64 h", "214.5 h", "0 h").
 */
export function formatMttrHours(hours: number): string {
  return `${(Math.round(hours * 100) / 100).toLocaleString("en-US", { maximumFractionDigits: 2 })} h`;
}
