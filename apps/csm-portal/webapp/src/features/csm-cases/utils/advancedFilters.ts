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

import type { BeCaseFieldFilter, BeCaseFieldFilterOp } from "@api/backend/types";
import { BE_CURRENT_USER_FILTER_PLACEHOLDER } from "@api/backend/types";
import { ALL_ISSUE_TYPES, ISSUE_TYPE_LABEL } from "@features/csm-cases/utils/caseIssueType";
import { resolveRelativeDatePlaceholder } from "@utils/resolveRelativeDatePlaceholder";
import {
  ENGAGEMENT_TYPE_OPTIONS,
  ONBOARDING_STATUS_OPTIONS,
  SEVERITY_OPTIONS,
  STATE_OPTIONS,
  WORK_STATE_OPTIONS,
} from "@features/csm-cases/utils/caseFilterOptions";
import { ALL_CASE_TYPES, CASE_TYPE_LABEL } from "@features/csm-cases/utils/caseType";

/**
 * The complete `POST /cases/search` field/op catalogue offered by the
 * unified "Advanced filters" builder — every field/op the backend accepts,
 * whether or not it also has a value slot on `CasesFilters` (see
 * `filterFieldAdapters.ts`'s per-field typed-adapter registry, which is what
 * makes switching between Simple and Advanced lossless: a field with a typed
 * adapter is rendered here from — and written back to — the same real
 * `CasesFilters` property the Simple grid's own dedicated control reads,
 * never a second, parallel copy of it). Every one of these is a real,
 * backend-accepted `BeCaseFieldFilterField` — see `types.ts`'s own doc
 * comment on that enum, which mirrors the entity-service's
 * `caseFilterFieldSet` exactly.
 *
 * Deliberately excludes two fields a hand-off brief once listed
 * (`accountId`, `resolvedOn`): neither appears in `BeCaseFieldFilterField`,
 * so the backend would reject them — widening that enum is a backend
 * contract change, out of scope for this FE-only builder. Flagged in
 * `PROGRESS.md`, not silently worked around.
 *
 * `tag`, `projectOnboardingStatus`, and `creTeam` were briefly excluded here
 * (2026-08-31) on the theory that a field with its own dedicated Simple-grid
 * control shouldn't also get a second, generic row — that created two
 * independent UI paths writing the same predicate. The unification that
 * follows replaces that with a single rule instead: every field lives here,
 * and a field with a typed adapter is *rendered differently* depending on
 * mode (a dedicated control in Simple, a generic row in Advanced) while
 * writing to the exact same `CasesFilters` property either way — so there is
 * only ever one UI path per field, not two.
 */
export type AdvancedFilterField =
  | "severity"
  | "state"
  | "workState"
  | "type"
  | "assignedUserId"
  | "projectId"
  | "product"
  | "creTeam"
  | "tag"
  | "engagementType"
  | "projectOnboardingStatus"
  | "projectType"
  | "issueType"
  | "sreTeam"
  | "deploymentId"
  | "number"
  | "internalId"
  | "resolutionNotes"
  | "parentId"
  | "taskSLABusinessElapsedPercent"
  | "escalationLevel"
  | "escalation"
  | "createdBy"
  | "createdOn"
  | "updatedOn"
  | "closedOn";

/** How a row's value input should render for a given field+op combination. */
export type AdvancedFilterValueKind =
  /** Free-text, comma-separated multi-value entry. Reserved for genuinely
   * arbitrary/opaque values with no enumerable suggestion source
   * (`deploymentId`). */
  | "multiText"
  /** Fixed-option multi-select. */
  | "multiSelect"
  /** A single free-text value. Reserved for genuinely arbitrary/opaque
   * identifiers (`number`, `internalId`, `parentId`). */
  | "text"
  /** A single numeric value. */
  | "number"
  /** A single date value — a literal `YYYY-MM-DD`/RFC3339 string, or one of
   * the relative-date placeholders (`__today__`, `__daysAgo:N__`, ...),
   * picked from a preset dropdown or an actual calendar date picker rather
   * than hand-typed. See {@link RELATIVE_DATE_PRESETS}. */
  | "dateOrPreset"
  /** No input at all — the op alone is the predicate (`isEmpty`/`isNotEmpty`). */
  | "none"
  /** No input — the row always means "the authenticated caller"; mirrors the
   * old `createdByMe: true` request field via
   * `BE_CURRENT_USER_FILTER_PLACEHOLDER`. */
  | "currentUser"
  /** Multi-value, backend-directory-search-backed email picker (type to
   * search, same directory `/users/search` the assignee filter already
   * searches) rather than hand-typed emails. `createdBy`'s `in` op only —
   * `assignedUserId` uses {@link AdvancedFilterValueKind.asyncAssigneeMultiSelect}
   * instead (it also offers the `@me` sentinel, `createdBy` does not). */
  | "asyncEmailMultiSelect"
  /** {@link AsyncAssigneeMultiSelect} — the `assignedUserId` row's value
   * input, same user-directory search plus the pinned `@me` sentinel the
   * Simple grid's own "Assignee" control uses. */
  | "asyncAssigneeMultiSelect"
  /** {@link AsyncProjectMultiSelect} — the `projectId` row's value input,
   * same type-to-search project picker the Simple grid's own "Project"
   * control uses. */
  | "asyncProjectMultiSelect"
  /** {@link ProductNameMultiSelect} — the `product` row's value input, same
   * type-to-search product-name picker the Simple grid's own "Product"
   * control uses. */
  | "asyncProductMultiSelect"
  /** {@link AsyncTagMultiSelect} — the `tag` row's value input (both `in`
   * and `notIn` ops), same `/tags/search` type-ahead the removed dedicated
   * "Tags" bar control used, minus its tri-state cycling (the row's own op
   * carries the include/exclude direction now). */
  | "asyncTagMultiSelect";

export interface AdvancedFilterOpMeta {
  op: BeCaseFieldFilterOp;
  /** Shown in the operator `Select`. */
  label: string;
  valueKind: AdvancedFilterValueKind;
}

export interface AdvancedFilterFieldMeta {
  field: AdvancedFilterField;
  label: string;
  ops: AdvancedFilterOpMeta[];
  /** Fixed options for a `multiSelect` value kind. */
  options?: { value: string; label: string }[];
  /** Free-text suggestions for a `multiText` value kind (open vocabulary —
   * offered, not enforced). */
  suggestions?: string[];
  /** Placeholder / helper text for a `text`/`multiText`/`number` input. */
  placeholder?: string;
}

const PROJECT_TYPE_OPTIONS: { value: string; label: string }[] = [
  "Subscription",
  "Managed Cloud Subscription",
  "Evaluation Subscription",
  "Development Support",
  "Cloud Support",
  "Cloud Evaluation Support",
  "Professional Services",
].map((v) => ({ value: v, label: v }));

const ISSUE_TYPE_OPTIONS: { value: string; label: string }[] = ALL_ISSUE_TYPES.map((v) => ({
  value: v,
  label: ISSUE_TYPE_LABEL[v],
}));

const CASE_TYPE_OPTIONS: { value: string; label: string }[] = ALL_CASE_TYPES.map((v) => ({
  value: v,
  label: CASE_TYPE_LABEL[v],
}));

const ESCALATION_LEVEL_OPTIONS: { value: string; label: string }[] = [
  "0",
  "1",
  "2",
  "3",
  "4",
  "5",
].map((v) => ({ value: v, label: v }));

/**
 * Ordered field catalogue backing the "Advanced filters" builder — the
 * single source of truth for which fields/ops/value shapes are offered.
 * Keep in sync with the field/op/value contract table this was built from
 * (see the task brief); it mirrors a live-verified backend contract, not
 * something to extend speculatively.
 */
export const ADVANCED_FILTER_FIELDS: AdvancedFilterFieldMeta[] = [
  {
    field: "severity",
    label: "Severity",
    ops: [{ op: "in", label: "is one of", valueKind: "multiSelect" }],
    options: SEVERITY_OPTIONS,
  },
  {
    field: "state",
    label: "State",
    ops: [
      { op: "in", label: "is one of", valueKind: "multiSelect" },
      { op: "notIn", label: "is not one of", valueKind: "multiSelect" },
    ],
    options: STATE_OPTIONS,
  },
  {
    field: "workState",
    label: "Work state",
    ops: [{ op: "in", label: "is one of", valueKind: "multiSelect" }],
    options: WORK_STATE_OPTIONS,
  },
  {
    field: "type",
    label: "Case type",
    ops: [{ op: "in", label: "is one of", valueKind: "multiSelect" }],
    options: CASE_TYPE_OPTIONS,
  },
  {
    field: "assignedUserId",
    label: "Assignee",
    ops: [{ op: "in", label: "is one of", valueKind: "asyncAssigneeMultiSelect" }],
  },
  {
    field: "projectId",
    label: "Project",
    ops: [{ op: "in", label: "is one of", valueKind: "asyncProjectMultiSelect" }],
  },
  {
    field: "product",
    label: "Product",
    ops: [{ op: "in", label: "is one of", valueKind: "asyncProductMultiSelect" }],
  },
  {
    // Options are supplied at render time by `AdvancedFiltersBuilder` (the
    // `creTeamOptions` prop, computed in `CasesFilterBar.tsx` from the same
    // `useTeams(true)` fetch the "CRE Team" bar control used) — fetched
    // data, not a fixed enum, same reasoning as `sreTeam` below.
    field: "creTeam",
    label: "CRE Team",
    ops: [{ op: "in", label: "is one of", valueKind: "multiSelect" }],
  },
  {
    field: "tag",
    label: "Tag",
    ops: [
      { op: "in", label: "includes", valueKind: "asyncTagMultiSelect" },
      { op: "notIn", label: "excludes", valueKind: "asyncTagMultiSelect" },
    ],
  },
  {
    field: "engagementType",
    label: "Engagement type",
    ops: [{ op: "in", label: "is one of", valueKind: "multiSelect" }],
    options: ENGAGEMENT_TYPE_OPTIONS,
  },
  {
    field: "projectOnboardingStatus",
    label: "Onboarding status",
    ops: [{ op: "in", label: "is one of", valueKind: "multiSelect" }],
    options: ONBOARDING_STATUS_OPTIONS,
  },
  {
    field: "projectType",
    label: "Project type",
    ops: [{ op: "in", label: "is one of", valueKind: "multiSelect" }],
    options: PROJECT_TYPE_OPTIONS,
  },
  {
    field: "issueType",
    label: "Issue type",
    ops: [{ op: "in", label: "is one of", valueKind: "multiSelect" }],
    options: ISSUE_TYPE_OPTIONS,
  },
  {
    // Options are supplied at render time by `AdvancedFiltersBuilder` (the
    // `sreTeamOptions` prop, computed once in `CasesFilterBar.tsx` from the
    // same `useTeams(true)` fetch the "CRE Team" (`creTeam`) bar control
    // uses) — not listed statically here, since the team registry is
    // fetched data, not a fixed enum like `projectType`/`issueType` above.
    field: "sreTeam",
    label: "SRE team",
    ops: [{ op: "in", label: "is one of", valueKind: "multiSelect" }],
  },
  {
    field: "deploymentId",
    label: "Deployment ID",
    ops: [{ op: "in", label: "is one of", valueKind: "multiText" }],
    placeholder: "deployment id",
  },
  {
    field: "number",
    label: "Case number",
    ops: [{ op: "eq", label: "is", valueKind: "text" }],
    placeholder: "e.g. CS0441174",
  },
  {
    field: "internalId",
    label: "WSO2 case ID",
    ops: [{ op: "eq", label: "is", valueKind: "text" }],
  },
  {
    field: "resolutionNotes",
    label: "Resolution notes",
    ops: [{ op: "isEmpty", label: "is empty", valueKind: "none" }],
  },
  {
    field: "parentId",
    label: "Parent case ID",
    ops: [{ op: "eq", label: "is", valueKind: "text" }],
  },
  {
    field: "taskSLABusinessElapsedPercent",
    label: "SLA business-elapsed %",
    ops: [
      { op: "gte", label: "is at least", valueKind: "number" },
      { op: "lte", label: "is at most", valueKind: "number" },
    ],
  },
  {
    field: "escalationLevel",
    label: "Escalation level",
    ops: [{ op: "in", label: "is one of", valueKind: "multiSelect" }],
    options: ESCALATION_LEVEL_OPTIONS,
  },
  {
    field: "escalation",
    label: "Escalation",
    ops: [
      { op: "isNotEmpty", label: "is active", valueKind: "none" },
      { op: "isEmpty", label: "has none", valueKind: "none" },
    ],
  },
  {
    field: "createdBy",
    label: "Created by",
    ops: [
      { op: "in", label: "email is one of", valueKind: "asyncEmailMultiSelect" },
      { op: "eq", label: "is me", valueKind: "currentUser" },
    ],
  },
  {
    field: "createdOn",
    label: "Created on",
    ops: [
      { op: "gte", label: "is on/after", valueKind: "dateOrPreset" },
      { op: "lte", label: "is on/before", valueKind: "dateOrPreset" },
    ],
  },
  {
    field: "updatedOn",
    label: "Updated on",
    ops: [
      { op: "gte", label: "is on/after", valueKind: "dateOrPreset" },
      { op: "lte", label: "is on/before", valueKind: "dateOrPreset" },
    ],
  },
  {
    field: "closedOn",
    label: "Closed on",
    ops: [
      { op: "gte", label: "is on/after", valueKind: "dateOrPreset" },
      { op: "lte", label: "is on/before", valueKind: "dateOrPreset" },
    ],
  },
];

/**
 * Fixed set of relative-date presets offered in the `dateOrPreset` value
 * input's preset dropdown — human labels for a subset of the placeholder
 * grammar {@link resolveRelativeDatePlaceholder} (and the entity-service's
 * own `resolveRelativeDate`, see `case_filters.go`) recognizes. Not
 * exhaustive of every representable offset (`__daysAgo:N__` accepts any
 * non-negative N) — these are the common ones; picking an exact calendar day
 * instead covers everything else, so the grammar itself never needs to be
 * hand-typed.
 */
export const RELATIVE_DATE_PRESETS: { value: string; label: string }[] = [
  { value: "__today__", label: "Today" },
  { value: "__daysAgo:7__", label: "7 days ago" },
  { value: "__daysAgo:30__", label: "30 days ago" },
  { value: "__daysAgo:90__", label: "90 days ago" },
  { value: "__startOfMonth:0__", label: "Start of this month" },
  { value: "__startOfMonth:-1__", label: "Start of last month" },
  { value: "__endOfMonth:0__", label: "End of this month" },
  { value: "__endOfMonth:-1__", label: "End of last month" },
  { value: "__startOfQuarter:0__", label: "Start of this quarter" },
  { value: "__startOfQuarter:-1__", label: "Start of last quarter" },
  { value: "__endOfQuarter:0__", label: "End of this quarter" },
  { value: "__endOfQuarter:-1__", label: "End of last quarter" },
];

const FIELD_META_BY_FIELD: Map<AdvancedFilterField, AdvancedFilterFieldMeta> = new Map(
  ADVANCED_FILTER_FIELDS.map((m) => [m.field, m]),
);

const ALL_ADVANCED_FIELDS: AdvancedFilterField[] = ADVANCED_FILTER_FIELDS.map((m) => m.field);

export function getAdvancedFilterFieldMeta(
  field: string,
): AdvancedFilterFieldMeta | undefined {
  return FIELD_META_BY_FIELD.get(field as AdvancedFilterField);
}

export function getAdvancedFilterOpMeta(
  field: string,
  op: string,
): AdvancedFilterOpMeta | undefined {
  return getAdvancedFilterFieldMeta(field)?.ops.find((o) => o.op === op);
}

/** One advanced-filter builder row: an ad-hoc field/op/value predicate. */
export interface AdvancedFilterRow {
  field: AdvancedFilterField;
  op: BeCaseFieldFilterOp;
  /** Empty for a value-less op (`isEmpty`/`isNotEmpty`) or the `currentUser`
   * value kind (the fixed placeholder value is filled in at request-build
   * time, not stored here). */
  values: string[];
}

export function defaultAdvancedFilterRow(): AdvancedFilterRow {
  const first = ADVANCED_FILTER_FIELDS[0];
  return { field: first.field, op: first.ops[0].op, values: [] };
}

/**
 * A row is only worth emitting (into the URL or the `/cases/search` payload)
 * once it carries everything its op needs — never an empty predicate. Ops
 * with no value requirement (`none`/`currentUser`) are complete as soon as
 * the field+op is chosen.
 */
export function isCompleteAdvancedFilterRow(row: AdvancedFilterRow): boolean {
  const opMeta = getAdvancedFilterOpMeta(row.field, row.op);
  if (!opMeta) return false;
  if (opMeta.valueKind === "none" || opMeta.valueKind === "currentUser") return true;
  return row.values.some((v) => v.trim().length > 0);
}

/**
 * Converts one complete advanced-filter row into the `/cases/search`
 * `BeCaseFieldFilter` shape. Returns `undefined` for an incomplete row (the
 * caller should already have filtered those out via
 * {@link isCompleteAdvancedFilterRow}, but this stays defensive rather than
 * emitting an empty predicate).
 */
export function advancedFilterRowToFieldFilter(
  row: AdvancedFilterRow,
): BeCaseFieldFilter | undefined {
  const opMeta = getAdvancedFilterOpMeta(row.field, row.op);
  if (!opMeta) return undefined;
  if (opMeta.valueKind === "none") {
    return { field: row.field, op: row.op };
  }
  if (opMeta.valueKind === "currentUser") {
    return { field: row.field, op: row.op, values: [BE_CURRENT_USER_FILTER_PLACEHOLDER] };
  }
  const values = row.values.map((v) => v.trim()).filter((v) => v.length > 0);
  if (values.length === 0) return undefined;
  return { field: row.field, op: row.op, values };
}

/**
 * Resolves relative-date placeholders (`__today__`, `__daysAgo:N__`, ...) in
 * any `createdOn`/`updatedOn`/`closedOn` row's values against `now`, the same
 * grammar/resolver the dedicated `createdOnGte`/`createdOnLte` bar filter
 * already uses (see `useGetCsmCases.ts`). A literal date (or anything
 * unrecognized) passes through unchanged. Returns a new array; input rows are
 * never mutated.
 */
export function resolveAdvancedFilterDateValues(
  rows: AdvancedFilterRow[],
  now: Date,
): AdvancedFilterRow[] {
  const DATE_FIELDS: ReadonlySet<AdvancedFilterField> = new Set([
    "createdOn",
    "updatedOn",
    "closedOn",
  ]);
  return rows.map((row) => {
    if (!DATE_FIELDS.has(row.field)) return row;
    if (row.op !== "gte" && row.op !== "lte") return row;
    return {
      ...row,
      values: row.values.map(
        (v) => resolveRelativeDatePlaceholder(v, row.op, now) ?? v,
      ),
    };
  });
}

const MAX_ADVANCED_FILTER_ROWS = 20;
const MAX_ADVANCED_FILTER_VALUE_LEN = 200;
const MAX_ADVANCED_FILTER_VALUES_PER_ROW = 50;

/**
 * Parses the `af` URL param (see `writeAdvancedFiltersToUrl`) back into rows.
 * Same "silently drop, never throw" policy as every other reader in
 * `casesFiltersUrl.ts`: a malformed param, an unknown field/op, or an
 * over-long/over-many value list is dropped rather than raising — a
 * hand-edited or stale URL degrades to "that one row is gone", not a crash.
 *
 * Deliberately does **not** drop an incomplete row (field+op chosen, no
 * value yet) — those must round-trip through the URL so the builder can
 * re-render an in-progress row after `setSearchParams` fires (every other
 * piece of filter state in this URL is edited the same way: write, re-read,
 * re-render). Only {@link writeCaseSearchFilters}/`buildCaseSearchFilters`
 * (the `/cases/search` request-payload boundary) may skip an incomplete row
 * — never this URL-persistence boundary.
 */
export function parseAdvancedFiltersParam(raw: string | null): AdvancedFilterRow[] {
  if (!raw) return [];
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch {
    return [];
  }
  if (!Array.isArray(parsed)) return [];

  const rows: AdvancedFilterRow[] = [];
  for (const entry of parsed.slice(0, MAX_ADVANCED_FILTER_ROWS)) {
    if (!Array.isArray(entry) || entry.length < 2 || entry.length > 3) continue;
    const [field, op, rawValues] = entry as [unknown, unknown, unknown];
    if (typeof field !== "string" || typeof op !== "string") continue;
    if (!ALL_ADVANCED_FIELDS.includes(field as AdvancedFilterField)) continue;
    const opMeta = getAdvancedFilterOpMeta(field, op);
    if (!opMeta) continue;

    let values: string[] = [];
    if (rawValues !== undefined) {
      if (!Array.isArray(rawValues)) continue;
      values = rawValues
        .filter((v): v is string => typeof v === "string")
        .map((v) => v.slice(0, MAX_ADVANCED_FILTER_VALUE_LEN))
        .slice(0, MAX_ADVANCED_FILTER_VALUES_PER_ROW);
    }
    const row: AdvancedFilterRow = { field: field as AdvancedFilterField, op: op as BeCaseFieldFilterOp, values };
    rows.push(row);
  }
  return rows;
}

/**
 * Serializes every row (complete or not) as a single JSON-encoded `af` param
 * value — `URLSearchParams` handles the percent-encoding, so no extra
 * escaping is needed here.
 *
 * An in-progress row (field/op picked, no value yet) IS persisted here: this
 * is purely the URL-persistence boundary, the same one every other filter
 * field round-trips through, and dropping incomplete rows here — rather than
 * only at the `/cases/search` request-payload boundary
 * ({@link advancedFilterRowToFieldFilter}, gated by
 * {@link isCompleteAdvancedFilterRow} in `caseSearchPayload.ts`) — is exactly
 * the bug this comment used to describe: `CsmIssuesView` treats the URL as
 * the single source of truth (`filters = readCasesFiltersFromUrl(searchParams)`),
 * so a row dropped here on write never survives the round trip back into
 * `AdvancedFiltersBuilder`'s render — the just-added "Add filter" row
 * vanished within the same tick, before the user could ever pick a value.
 * The backend never sees an empty predicate regardless, because
 * `advancedFilterRowToFieldFilter` still filters incomplete rows out at
 * request-build time.
 */
export function writeAdvancedFiltersParam(rows: AdvancedFilterRow[]): string | null {
  if (rows.length === 0) return null;
  return JSON.stringify(
    rows.map((r) => (r.values.length > 0 ? [r.field, r.op, r.values] : [r.field, r.op])),
  );
}
