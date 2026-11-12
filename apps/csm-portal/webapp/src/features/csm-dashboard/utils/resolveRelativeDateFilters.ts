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

import {
  resolveRelativeDatePlaceholder,
  resolveRelativeDatePlaceholderToDate,
  resolveRelativeDatePlaceholderToUtcDate,
} from "@utils/resolveRelativeDatePlaceholder";
import { hasFieldFilterEntries, isFieldFilterEntry } from "./widgetPreviewUrl";

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
 * UUID, an enum value — passes through unchanged. A flat filters record
 * (every other resourceType's) has only its `startDate` / `endDate` resolved,
 * to a plain local date (a time card search's work-date bounds); every other
 * flat key passes through unchanged, same fail-open convention as
 * `resolveTeamPlaceholder`.
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
  return resolveFlatDateFilters(resolveCaseFieldDateFilters(filters, now), now);
}

/**
 * The flat date keys a non-case resourceType's filters may carry a
 * relative-date placeholder in: a time card search's `startDate` / `endDate`
 * (work date), a project search's `<date>From` / `<date>To` bounds (the
 * project's end date and its onboarding start, go-live plan, go-live,
 * onboarding expiry and production readiness completion dates) and a PLG
 * search's `registeredFrom` / `registeredTo` (the registration date), all
 * whole days and inclusive. They resolve to a plain `YYYY-MM-DD`, not an
 * instant: the time card and project searches compare dates, and the PLG
 * request body turns a plain date into that whole UTC day
 * (`plgSearchFilters`). The PLG bounds therefore resolve against the UTC
 * date (UTC_DATE_FILTER_KEYS), the others against the viewer's local one.
 */
const FLAT_DATE_FILTER_KEYS = [
  "startDate",
  "endDate",
  "endDateFrom",
  "endDateTo",
  "onboardingStartDateFrom",
  "onboardingStartDateTo",
  "goLivePlanDateFrom",
  "goLivePlanDateTo",
  "goLiveDateFrom",
  "goLiveDateTo",
  "onboardingExpiryDateFrom",
  "onboardingExpiryDateTo",
  "productionReadinessCompletionDateFrom",
  "productionReadinessCompletionDateTo",
  "registeredFrom",
  "registeredTo",
] as const;

/**
 * The flat date keys read as UTC days: a PLG search's registration date
 * bounds, which the same widget's trend line (`windowDate`) also resolves in
 * UTC. Resolved against the local date, "today" is already tomorrow's UTC day
 * for a viewer east of UTC between local midnight and UTC midnight, and the
 * count would leave out the registrations the trend counts.
 */
const UTC_DATE_FILTER_KEYS: ReadonlySet<string> = new Set(["registeredFrom", "registeredTo"]);

/**
 * Resolves relative date placeholders in the flat date filters
 * (FLAT_DATE_FILTER_KEYS) to plain dates. Returns `filters` itself when
 * nothing changes, else a copy.
 */
function resolveFlatDateFilters(filters: Record<string, unknown>, now: Date): Record<string, unknown> {
  let resolved: Record<string, unknown> | undefined;
  for (const key of FLAT_DATE_FILTER_KEYS) {
    const value = filters[key];
    if (typeof value !== "string") continue;
    const date = UTC_DATE_FILTER_KEYS.has(key)
      ? resolveRelativeDatePlaceholderToUtcDate(value, now)
      : resolveRelativeDatePlaceholderToDate(value, now);
    if (date === undefined) continue;
    resolved ??= { ...filters };
    resolved[key] = date;
  }
  return resolved ?? filters;
}

/**
 * Resolves relative date placeholders in the values of `filters.filters`
 * entries, each by its own op. Returns `filters` itself when nothing changes,
 * else a copy.
 */
function resolveCaseFieldDateFilters(filters: Record<string, unknown>, now: Date): Record<string, unknown> {
  const fieldFilters = filters.filters;
  if (!hasFieldFilterEntries(fieldFilters)) return filters;

  let changed = false;
  const resolved = fieldFilters.map((entry) => {
    if (!isFieldFilterEntry(entry)) return entry;
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
