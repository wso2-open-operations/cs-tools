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

import type { BeDashboardWidgetColumnFormat } from "@api/backend/types";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";

/**
 * Resolves a dot-separated path (e.g. `"project.key"`,
 * `"project.account.tier"`) against one search-response item, walking into
 * nested objects to arbitrary depth — every resource's search response
 * embeds related entities as nested JSON objects (`{ id, name }` refs and
 * deeper), not flat records, so a widget column has to be able to reach
 * into them. Returns `undefined` for a path that doesn't resolve (a missing
 * segment, or a non-object value partway through) rather than throwing —
 * one misconfigured/unavailable column must not break the whole widget.
 */
export function resolveColumnPath(item: Record<string, unknown>, path: string): unknown {
  const segments = path.split(".").filter(Boolean);
  let current: unknown = item;
  for (const segment of segments) {
    if (current === null || current === undefined || typeof current !== "object") {
      return undefined;
    }
    current = (current as Record<string, unknown>)[segment];
  }
  return current;
}

/** Placeholder rendered for a column value that is absent, null, or an
 * unresolved path — matches the em-dash convention every other dashboard
 * list renderer in this app already uses (see `formatDate` in
 * `widgetListConfig.tsx`). */
const EMPTY_VALUE = "—";

/**
 * Formats a resolved column value for display, per `format`:
 * - `"date"`: formatted via the same date-only formatter the app's existing
 *   hardcoded list renderers use for a date column; a value that isn't a
 *   parseable date string falls back to the empty placeholder rather than
 *   rendering a raw/garbled string.
 * - omitted / `"text"` (default): rendered as plain text. Non-string
 *   primitives (number, boolean) are stringified; an object/array (a column
 *   path that resolved to a nested structure rather than a leaf value) also
 *   renders as the empty placeholder — this renderer is for scalar columns.
 */
export function formatColumnValue(
  value: unknown,
  format?: BeDashboardWidgetColumnFormat,
): string {
  if (value === null || value === undefined || value === "") {
    return EMPTY_VALUE;
  }
  if (format === "date") {
    if (typeof value !== "string") return EMPTY_VALUE;
    return (
      formatBackendTimestampForDisplay(value, {
        year: "numeric",
        month: "short",
        day: "numeric",
      }) ?? EMPTY_VALUE
    );
  }
  if (typeof value === "string" || typeof value === "number" || typeof value === "boolean") {
    return String(value);
  }
  // Object/array: not a scalar this renderer knows how to display.
  return EMPTY_VALUE;
}
