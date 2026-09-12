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

import type { CaseState, Severity } from "@features/csm-dashboard/types/abtDashboard";
import type { BeCaseType, BeCaseWorkState, BeEngagementType } from "@api/backend/types";
import type { CasesFilters } from "@features/csm-cases/components/CasesFilterBar";
import {
  ADVANCED_FILTER_FIELDS,
  defaultAdvancedFilterRow,
  isCompleteAdvancedFilterRow,
  type AdvancedFilterField,
  type AdvancedFilterRow,
} from "@features/csm-cases/utils/advancedFilters";

/**
 * The per-field "one canonical state, two presentations" adapter registry —
 * this is what makes switching between the Simple grid and the unified
 * Advanced-mode row list lossless. For every `CasesFilters` field that
 * already has a real, typed slot (the ~20 fields listed in `CasesFilters`'s
 * own doc comment), `get`/`set` read/write that real property directly —
 * there is no second, parallel "advanced filter value" for these fields, so
 * a value picked in one mode is immediately visible, in the same shape, in
 * the other. A field/op pair with NO entry here (the 7 fields with no typed
 * `CasesFilters` slot: `deploymentId`, `number`, `internalId`,
 * `resolutionNotes`, `parentId`, `createdBy`, `issueType`) falls back to the
 * existing generic `filters.advancedFilters` array mechanism, exactly as it
 * worked before this unification.
 *
 * `values` here is always the row's UI-domain string form (e.g. `Severity`
 * literals like `"S1"`, not the backend's `priority` encoding) — the same
 * shape the Simple grid's own dedicated controls already read/write.
 * Backend-form translation (severity→priority, UI state→BE state, ...)
 * happens once, downstream, in `caseSearchPayload.ts` — unchanged by this
 * file, since that already reads straight off the same typed `CasesFilters`
 * properties these adapters target.
 */
interface TypedFieldAdapter {
  get: (f: CasesFilters) => string[];
  /** Applies a complete row's values to the real `CasesFilters` property. */
  set: (f: CasesFilters, values: string[]) => CasesFilters;
  /** Resets the real `CasesFilters` property back to its "unset" value —
   * used when a row is removed, or when it's edited away from this
   * field/op entirely. Defaults to `set(f, [])`, which is exactly right
   * for every array/scalar-valued field; only the value-less `escalation`
   * ops (whose `set` doesn't accept an empty-values "clear" — see below)
   * override it. */
  clear: (f: CasesFilters) => CasesFilters;
}

function simpleAdapter(
  get: (f: CasesFilters) => string[],
  set: (f: CasesFilters, values: string[]) => CasesFilters,
): TypedFieldAdapter {
  return { get, set, clear: (f) => set(f, []) };
}

/** `state`/`in` and `state`/`notIn` share this: work sub-state only applies
 * server-side when `work_in_progress` is the *sole* included state (see
 * `caseSearchPayload.ts`'s own matching guard) — mirror that invariant here
 * so editing either side of the state row can't silently strand an
 * inapplicable `workStates` value that only a chip could then reveal. */
function pruneWorkStatesIfNotSoleWip(f: CasesFilters): BeCaseWorkState[] {
  const soleWip =
    f.states.length === 1 && f.states[0] === "work_in_progress" && f.excludeStates.length === 0;
  return soleWip ? f.workStates : [];
}

function key(field: string, op: string): string {
  return `${field}:${op}`;
}

/** Parses an SLA-percent row value, treating a non-numeric input (a
 * hand-edited or stale `af` URL, e.g. `af=[["taskSLABusinessElapsedPercent",
 * "gte",["abc"]]]`) as unset rather than writing `NaN` into `CasesFilters` —
 * `NaN` isn't `null`, so it would otherwise propagate into the active-filter
 * count, the URL, and the `/cases/search` request body. */
function parseSlaPct(values: string[]): number | null {
  if (values.length === 0) return null;
  const n = Number(values[0]);
  return Number.isFinite(n) ? n : null;
}

const TYPED_ADAPTERS: Record<string, TypedFieldAdapter> = {
  [key("severity", "in")]: simpleAdapter(
    (f) => f.severities,
    (f, v) => ({ ...f, severities: v as Severity[] }),
  ),
  [key("state", "in")]: simpleAdapter(
    (f) => f.states,
    (f, v) => {
      const next = { ...f, states: v as CaseState[] };
      return { ...next, workStates: pruneWorkStatesIfNotSoleWip(next) };
    },
  ),
  [key("state", "notIn")]: simpleAdapter(
    (f) => f.excludeStates,
    (f, v) => {
      const next = { ...f, excludeStates: v as CaseState[] };
      return { ...next, workStates: pruneWorkStatesIfNotSoleWip(next) };
    },
  ),
  [key("workState", "in")]: simpleAdapter(
    (f) => f.workStates,
    (f, v) => ({ ...f, workStates: v as BeCaseWorkState[] }),
  ),
  [key("type", "in")]: simpleAdapter(
    (f) => f.caseTypes,
    (f, v) => ({ ...f, caseTypes: v as BeCaseType[] }),
  ),
  [key("assignedUserId", "in")]: simpleAdapter(
    (f) => f.assignees,
    (f, v) => ({ ...f, assignees: v }),
  ),
  [key("projectId", "in")]: simpleAdapter(
    (f) => f.projects,
    (f, v) => ({ ...f, projects: v }),
  ),
  [key("product", "in")]: simpleAdapter(
    (f) => f.productNames,
    (f, v) => ({ ...f, productNames: v }),
  ),
  [key("creTeam", "in")]: simpleAdapter(
    (f) => f.csTeams,
    (f, v) => ({ ...f, csTeams: v }),
  ),
  [key("sreTeam", "in")]: simpleAdapter(
    (f) => f.sreTeams,
    (f, v) => ({ ...f, sreTeams: v }),
  ),
  [key("tag", "in")]: simpleAdapter(
    (f) => f.tags,
    (f, v) => ({ ...f, tags: v }),
  ),
  [key("tag", "notIn")]: simpleAdapter(
    (f) => f.excludeTags,
    (f, v) => ({ ...f, excludeTags: v }),
  ),
  [key("projectOnboardingStatus", "in")]: simpleAdapter(
    (f) => f.onboardingStatuses,
    (f, v) => ({ ...f, onboardingStatuses: v }),
  ),
  [key("engagementType", "in")]: simpleAdapter(
    (f) => f.engagementTypes,
    (f, v) => ({ ...f, engagementTypes: v as BeEngagementType[] }),
  ),
  [key("taskSLABusinessElapsedPercent", "gte")]: simpleAdapter(
    (f) => (f.slaElapsedPctGte !== null ? [String(f.slaElapsedPctGte)] : []),
    (f, v) => ({ ...f, slaElapsedPctGte: parseSlaPct(v) }),
  ),
  [key("taskSLABusinessElapsedPercent", "lte")]: simpleAdapter(
    (f) => (f.slaElapsedPctLte !== null ? [String(f.slaElapsedPctLte)] : []),
    (f, v) => ({ ...f, slaElapsedPctLte: parseSlaPct(v) }),
  ),
  [key("escalationLevel", "in")]: simpleAdapter(
    (f) => f.escalationLevels,
    (f, v) => ({ ...f, escalationLevels: v }),
  ),
  // `escalation` is value-less (`isNotEmpty`/`isEmpty` — the op alone is the
  // whole predicate, see `hasEscalation`'s own doc comment on `CasesFilters`)
  // — a row's mere presence with this field/op means "active"/"has none"
  // regardless of its (always-empty) `values`, so `set` can't reuse the
  // `simpleAdapter` "set([]) clears" convention: `set(f, [])` here still
  // means "activate this op", not "clear". `clear` is overridden instead.
  [key("escalation", "isNotEmpty")]: {
    get: (f) => (f.hasEscalation === true ? ["true"] : []),
    set: (f) => ({ ...f, hasEscalation: true }),
    clear: (f) => ({ ...f, hasEscalation: null }),
  },
  [key("escalation", "isEmpty")]: {
    get: (f) => (f.hasEscalation === false ? ["true"] : []),
    set: (f) => ({ ...f, hasEscalation: false }),
    clear: (f) => ({ ...f, hasEscalation: null }),
  },
  [key("projectType", "in")]: simpleAdapter(
    (f) => f.projectTypes,
    (f, v) => ({ ...f, projectTypes: v }),
  ),
  [key("createdOn", "gte")]: simpleAdapter(
    (f) => (f.createdOnGte !== null ? [f.createdOnGte] : []),
    (f, v) => ({ ...f, createdOnGte: v[0] ?? null }),
  ),
  [key("createdOn", "lte")]: simpleAdapter(
    (f) => (f.createdOnLte !== null ? [f.createdOnLte] : []),
    (f, v) => ({ ...f, createdOnLte: v[0] ?? null }),
  ),
  [key("updatedOn", "gte")]: simpleAdapter(
    (f) => (f.updatedOnGte !== null ? [f.updatedOnGte] : []),
    (f, v) => ({ ...f, updatedOnGte: v[0] ?? null }),
  ),
  [key("updatedOn", "lte")]: simpleAdapter(
    (f) => (f.updatedOnLte !== null ? [f.updatedOnLte] : []),
    (f, v) => ({ ...f, updatedOnLte: v[0] ?? null }),
  ),
  [key("closedOn", "gte")]: simpleAdapter(
    (f) => (f.closedOnGte !== null ? [f.closedOnGte] : []),
    (f, v) => ({ ...f, closedOnGte: v[0] ?? null }),
  ),
  [key("closedOn", "lte")]: simpleAdapter(
    (f) => (f.closedOnLte !== null ? [f.closedOnLte] : []),
    (f, v) => ({ ...f, closedOnLte: v[0] ?? null }),
  ),
};

function getTypedAdapter(field: string, op: string): TypedFieldAdapter | undefined {
  return TYPED_ADAPTERS[key(field, op)];
}

/** One row in the unified Advanced-mode row list — a normal
 * {@link AdvancedFilterRow} plus where it actually lives: `"typed"` means
 * it's a live view of a real `CasesFilters` property (via the registry
 * above); `"array"` means it's a normal entry in `filters.advancedFilters`
 * (`arrayIndex` is its position there — required for `"array"` rows,
 * unused for `"typed"` ones). */
export interface UnifiedFilterRow extends AdvancedFilterRow {
  origin: "typed" | "array";
  arrayIndex?: number;
}

/**
 * Builds the unified Advanced-mode row list: one row per non-empty typed
 * field/op (in catalogue order), followed by every entry in
 * `filters.advancedFilters` (in its own order) — the untyped escape hatch,
 * unchanged. Purely a *view* over `filters`; never stored, always
 * recomputed from it, which is exactly why Simple↔Advanced switching cannot
 * drop or duplicate anything: there is nothing else to go stale.
 */
export function filtersToAdvancedRows(filters: CasesFilters): UnifiedFilterRow[] {
  const rows: UnifiedFilterRow[] = [];
  for (const fieldMeta of ADVANCED_FILTER_FIELDS) {
    for (const opMeta of fieldMeta.ops) {
      const adapter = getTypedAdapter(fieldMeta.field, opMeta.op);
      if (!adapter) continue;
      const raw = adapter.get(filters);
      if (raw.length === 0) continue;
      rows.push({
        origin: "typed",
        field: fieldMeta.field,
        op: opMeta.op,
        values: opMeta.valueKind === "none" ? [] : raw,
      });
    }
  }
  filters.advancedFilters.forEach((row, arrayIndex) => {
    rows.push({ ...row, origin: "array", arrayIndex });
  });
  return rows;
}

/**
 * Applies an edit to one unified row (including changing its own field —
 * the row's field-picker dropdown — which this handles the same as any
 * other edit: clear the old identity, then re-derive where the new one
 * lives). A row that stays in `filters.advancedFilters` before and after
 * the edit is updated **in place** at its existing array index rather than
 * removed-and-reappended, so an in-progress row being typed into never
 * jumps position (and never loses input focus) on every keystroke — it only
 * moves position at the one real transition: the edit that makes it
 * complete-and-typed (promoted out of the array) or the edit that makes an
 * already-typed row incomplete/re-fielded to an untyped field (demoted back
 * into the array, appended at the end — there is no "original" array
 * position to restore a typed row to).
 */
export function updateUnifiedRow(
  filters: CasesFilters,
  current: UnifiedFilterRow,
  nextRow: AdvancedFilterRow,
): CasesFilters {
  const complete = isCompleteAdvancedFilterRow(nextRow);
  const newAdapter = getTypedAdapter(nextRow.field, nextRow.op);
  const staysInArrayInPlace =
    current.origin === "array" && !(complete && newAdapter) && current.arrayIndex !== undefined;

  if (staysInArrayInPlace) {
    return {
      ...filters,
      advancedFilters: filters.advancedFilters.map((r, i) =>
        i === current.arrayIndex ? nextRow : r,
      ),
    };
  }

  let next = filters;
  if (current.origin === "typed") {
    const oldAdapter = getTypedAdapter(current.field, current.op);
    if (oldAdapter) next = oldAdapter.clear(next);
  } else if (current.origin === "array" && current.arrayIndex !== undefined) {
    next = {
      ...next,
      advancedFilters: next.advancedFilters.filter((_, i) => i !== current.arrayIndex),
    };
  }

  if (complete && newAdapter) {
    next = newAdapter.set(next, nextRow.values);
  } else {
    next = { ...next, advancedFilters: [...next.advancedFilters, nextRow] };
  }
  return next;
}

/** Removes one unified row entirely — clears the real `CasesFilters`
 * property for a `"typed"` row, splices `filters.advancedFilters` for an
 * `"array"` one. */
export function removeUnifiedRow(filters: CasesFilters, row: UnifiedFilterRow): CasesFilters {
  if (row.origin === "typed") {
    const adapter = getTypedAdapter(row.field, row.op);
    return adapter ? adapter.clear(filters) : filters;
  }
  if (row.arrayIndex !== undefined) {
    return {
      ...filters,
      advancedFilters: filters.advancedFilters.filter((_, i) => i !== row.arrayIndex),
    };
  }
  return filters;
}

/** Appends a brand-new, blank row — always lands in `filters.advancedFilters`
 * (an unset field/op has nothing for a typed adapter to represent yet; see
 * `filtersToAdvancedRows`, which only shows a typed row once it's non-empty)
 * exactly like the "Add filter" button did before this unification. */
export function addBlankUnifiedRow(filters: CasesFilters): CasesFilters {
  return { ...filters, advancedFilters: [...filters.advancedFilters, defaultAdvancedFilterRow()] };
}

/**
 * Folds any `filters.advancedFilters` entry that targets a now-typed
 * field/op (e.g. a `af` URL param authored — by hand, or by an older build
 * of this page — before `severity`/`state`/`tag`/... had typed adapters)
 * into the real `CasesFilters` property instead, so it never lingers as a
 * dangling duplicate of the same predicate. Called once at the URL-read
 * boundary (`readCasesFiltersFromUrl`); every edit made through
 * {@link updateUnifiedRow} already keeps this invariant by construction, so
 * this is purely a defensive normalization of *external* input, not
 * something the builder itself needs to re-run.
 */
export function normalizeCasesFilters(filters: CasesFilters): CasesFilters {
  let next = filters;
  const kept: AdvancedFilterRow[] = [];
  for (const row of filters.advancedFilters) {
    const adapter = getTypedAdapter(row.field, row.op);
    if (adapter && isCompleteAdvancedFilterRow(row)) {
      next = adapter.set(next, row.values);
    } else {
      kept.push(row);
    }
  }
  return { ...next, advancedFilters: kept };
}

/**
 * Fields the Simple grid's dedicated controls do NOT cover (after removing
 * Tags — it's Advanced-only now, see `CasesFilterBar.tsx`). Filters
 * expressible entirely within the Simple grid's fields (`severities`,
 * `states`, `caseTypes`, `assignees`, `workStates`, `projects`,
 * `engagementTypes`, `productNames`, `csTeams`, `onboardingStatuses`) can
 * still be represented in Simple mode regardless of this check — those
 * fields simply aren't in this list.
 *
 * `excludeStates` DOES gate Simple mode now, same reasoning as `tags`/
 * `excludeTags`: the Simple grid's "State" control used to be a tri-state
 * toggle that read/wrote both `states` AND `excludeStates` directly, but
 * that control was removed (Simple mode's "State" field is now a plain
 * include-only multi-select, matching every other Simple field) — so an
 * active `excludeStates` value is only representable via the Advanced-mode
 * `state`/`notIn` row now, same as a `tag`/`notIn` value.
 */
export function isSimpleRepresentable(filters: CasesFilters): boolean {
  return (
    filters.tags.length === 0 &&
    filters.excludeTags.length === 0 &&
    filters.excludeStates.length === 0 &&
    filters.sreTeams.length === 0 &&
    filters.projectTypes.length === 0 &&
    filters.escalationLevels.length === 0 &&
    filters.hasEscalation === null &&
    filters.slaElapsedPctGte === null &&
    filters.slaElapsedPctLte === null &&
    filters.createdOnGte === null &&
    filters.createdOnLte === null &&
    filters.updatedOnGte === null &&
    filters.updatedOnLte === null &&
    filters.closedOnGte === null &&
    filters.closedOnLte === null &&
    filters.advancedFilters.length === 0 &&
    filters.anyOfBranches.length === 0
  );
}

// Re-exported so callers that only need the field-catalogue type don't have
// to also import `advancedFilters.ts` directly.
export type { AdvancedFilterField };
