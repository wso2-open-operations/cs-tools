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

import { resolveRelativeDatePlaceholder } from "@utils/resolveRelativeDatePlaceholder";
import { isCaseFieldFilterArray, type WidgetCaseFieldFilterLike } from "./widgetPreviewUrl";

/**
 * Resolves every relative-date placeholder (see
 * {@link resolveRelativeDatePlaceholder}) in a widget's case-search filters
 * (`{ filters: BeCaseFieldFilter[] }` — the same DSL `resolveTeamPlaceholder`
 * and `mergeWidgetFilters` operate on) against `now` — real wall-clock time
 * in the viewer's own browser by default, so "today" always means the
 * viewer's own local calendar day rather than UTC's.
 *
 * ServiceNow's own equivalent "Today" reports resolve against the support
 * team's session timezone (confirmed `Asia/Colombo`, UTC+5:30); the
 * entity-service resolves the same placeholders against UTC, which
 * undercounts "Created Today"/"Resolved Today" widgets by the up-to-~5.5h gap
 * between UTC midnight and Colombo midnight. Resolving here, in the viewer's
 * own browser-local time, fixes that for today's CS engineers (physically
 * Colombo-based) without hardcoding that assumption into the platform.
 *
 * Every filter value that ISN'T a recognized placeholder — a literal date, a
 * UUID, an enum value — passes through unchanged. Non-case-filter-shaped
 * `filters` (every other resourceType's flat record) also pass through
 * unchanged, same fail-open convention as `resolveTeamPlaceholder`.
 *
 * The entity-service's own placeholder resolution (`resolveRelativeDate`,
 * still UTC-based) stays in place as a dead fallback — should this function
 * ever miss a placeholder shape it doesn't yet recognize, or the value
 * reaches `/cases/search` unresolved through some other path, the backend
 * still resolves it rather than rejecting the request outright. It just
 * resolves it against UTC "today", not the viewer's — the same discrepancy
 * this function exists to close for the common case.
 */
export function resolveRelativeDateFilters(
  filters: Record<string, unknown>,
  now: Date = new Date(),
): Record<string, unknown> {
  const fieldFilters = filters.filters;
  if (!isCaseFieldFilterArray(fieldFilters)) return filters;

  let changed = false;
  const resolved: WidgetCaseFieldFilterLike[] = fieldFilters.map((entry) => {
    const values = entry.values;
    if (!values || values.length === 0) return entry;

    let entryChanged = false;
    const newValues = values.map((v) => {
      const r = resolveRelativeDatePlaceholder(v, entry.op, now);
      if (r === undefined) return v;
      entryChanged = true;
      return r;
    });

    if (!entryChanged) return entry;
    changed = true;
    return { ...entry, values: newValues };
  });

  if (!changed) return filters;
  return { ...filters, filters: resolved };
}
