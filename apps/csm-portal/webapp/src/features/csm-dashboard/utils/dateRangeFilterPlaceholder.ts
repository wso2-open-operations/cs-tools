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
 * Placeholder values a widget's own flat `dateFrom`/`dateTo` filter (the
 * `case_feedback` resourceType's own shape — `CaseFeedbackSearchFilters`/
 * `CaseFeedbackAggregateFilters`, never the case-search DSL) may carry to
 * mean "the dashboard's own currently-selected date range" — the
 * `case_feedback`-dashboard-scoped counterpart of `__current_team__`
 * (`teamFilterPlaceholder.ts`), resolved entirely CLIENT-SIDE the same way.
 *
 * There is no per-dashboard interactive filter control anywhere else in this
 * widget framework today — every other widget's `query`/`groupBy` is fully
 * static, resolved only through the team/current-user/relative-date
 * placeholder pipelines. This is the first widget-level control a viewer can
 * actually change at runtime (see `DateRangeFilter`), which is why it needs
 * its own placeholder pair rather than reusing an existing one: it stands for
 * page-local UI state, not identity (`__current_user__`) or dashboard
 * scoping (`__current_team__`).
 */
export const DATE_RANGE_FROM_PLACEHOLDER = "__dateRangeFrom__";
export const DATE_RANGE_TO_PLACEHOLDER = "__dateRangeTo__";

/**
 * Substitutes {@link DATE_RANGE_FROM_PLACEHOLDER}/{@link DATE_RANGE_TO_PLACEHOLDER}
 * wherever they appear as the literal value of a widget's own flat
 * `dateFrom`/`dateTo` filter keys with the dashboard's own currently-selected
 * range (see `DateRangeFilter`) — mirrors `resolveTeamPlaceholder`'s flat-field
 * resolution (`assignmentTeamIds`), not its case-DSL one: `case_feedback`'s
 * filters are always the flat `{caseId?, accountIds?, dateFrom?, dateTo?}`
 * shape, never `{filters: BeCaseFieldFilter[]}`.
 *
 * `dateRangeFrom`/`dateRangeTo` of `undefined` (no range selected — "all
 * time", the default) DROPS the corresponding key from the returned filters
 * object entirely, rather than sending the literal placeholder string (which
 * the entity service would either reject as an invalid date or, worse,
 * silently fail to parse as a filter bound) — same fail-safe "widen back to
 * unfiltered" policy `resolveTeamPlaceholder` documents for its own
 * placeholder, and for the same reason: a chart/list that's too broad is
 * visibly wrong and gets noticed; a request that 400s or silently matches
 * nothing does not read as "no filter applied" to the viewer.
 *
 * A literal `dateFrom`/`dateTo` value already present (i.e. not this
 * placeholder) is left alone — this only ever substitutes the placeholder
 * itself, never overrides a widget's own hardcoded date bound.
 */
export function resolveDateRangeFilterPlaceholder(
  filters: Record<string, unknown>,
  dateRangeFrom: string | undefined,
  dateRangeTo: string | undefined,
): Record<string, unknown> {
  let result = filters;
  if (result.dateFrom === DATE_RANGE_FROM_PLACEHOLDER) {
    if (dateRangeFrom === undefined) {
      const { dateFrom: _dropped, ...rest } = result;
      result = rest;
    } else {
      result = { ...result, dateFrom: dateRangeFrom };
    }
  }
  if (result.dateTo === DATE_RANGE_TO_PLACEHOLDER) {
    if (dateRangeTo === undefined) {
      const { dateTo: _dropped, ...rest } = result;
      result = rest;
    } else {
      result = { ...result, dateTo: dateRangeTo };
    }
  }
  return result;
}

/**
 * Whether `filters` carries either placeholder anywhere
 * {@link resolveDateRangeFilterPlaceholder} would actually resolve it — used
 * by `AgentsLandingPagePilot` to decide, purely from the loaded dashboard's
 * own widget list, whether to render the `DateRangeFilter` control at all
 * (no dashboard-level config flag exists or is needed: any dashboard whose
 * widgets reference this placeholder opts in automatically, the same
 * "derive from what's actually there" approach already used for chart vs.
 * list grouping).
 */
export function hasDateRangeFilterPlaceholder(filters: Record<string, unknown>): boolean {
  return filters.dateFrom === DATE_RANGE_FROM_PLACEHOLDER || filters.dateTo === DATE_RANGE_TO_PLACEHOLDER;
}
