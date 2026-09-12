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

import type { BeCaseFieldFilter } from "@api/backend/types";
import { priorityFromSeverity } from "@api/backend/mappers";
import type { Severity } from "@features/csm-dashboard/types/abtDashboard";
import { STATE_LABEL } from "@features/csm-dashboard/utils/abtDashboard";
import { beStateFromUi } from "@api/backend/mappers";
import { ALL_CASE_TYPES, CASE_TYPE_LABEL } from "@features/csm-cases/utils/caseType";
import { ALL_ISSUE_TYPES, ISSUE_TYPE_LABEL } from "@features/csm-cases/utils/caseIssueType";

/**
 * `filters.anyOf` branch field allowlist (`CaseFilterGroup` on the backend —
 * see `case_filters.go` / the `POST /cases/search` reference doc's own
 * `filters.anyOf` section). Deliberately a **much** narrower set than
 * {@link ADVANCED_FILTER_FIELDS "the top-level Advanced filters catalogue"}
 * — the backend rejects anything outside it by name (`anyOf[i].filters:
 * unsupported field: x`), so the UI must not offer a field here it can't
 * accept. `state` is `in`-only inside a branch (no `notIn`), and
 * `assignedUserId` is `in`-only (no `isEmpty`) — both narrower than their
 * top-level-filter counterparts, which is why this is its own small
 * catalogue rather than a filtered view of the top-level one.
 */
export type AnyOfFilterField =
  | "type"
  | "state"
  | "severity"
  | "engagementType"
  | "issueType"
  | "workState"
  | "projectId"
  | "deploymentId"
  | "assignedUserId"
  | "escalationLevel";

/** How an `anyOf` branch row's value input should render. Narrower than
 * {@link AdvancedFilterValueKind} — every branch field has a real, closed
 * enum, an async directory search, or (for `deploymentId`) is genuinely
 * arbitrary free text; nothing here needs the top-level catalogue's date/
 * number/current-user shapes. */
export type AnyOfValueKind =
  | "multiSelect"
  | "multiText"
  | "asyncProject"
  | "asyncAssignedUser";

export interface AnyOfFilterFieldMeta {
  field: AnyOfFilterField;
  label: string;
  /** Every branch field in this catalogue is `in`-only (see the type doc
   * comment above) — one op per field, not an ops array, since there is
   * never a choice to present. */
  op: "in";
  opLabel: string;
  valueKind: AnyOfValueKind;
  /** Fixed options for a `multiSelect` value kind. */
  options?: { value: string; label: string }[];
  placeholder?: string;
}

const ANY_OF_TYPE_OPTIONS = ALL_CASE_TYPES.map((v) => ({ value: v, label: CASE_TYPE_LABEL[v] }));

// `state` inside a branch is `in`-only. Excludes `reopened`, same as the
// bar's own `PRIMARY_STATES` (`CasesFilterBar.tsx`) — it's a valid backend
// enum value, but never a case's own `state` (only ever a `nextStates`
// transition marker, see `CaseState`'s own doc comment), so offering it here
// would suggest a filter that can never match anything.
const ANY_OF_STATE_VALUES: (keyof typeof STATE_LABEL)[] = [
  "open",
  "work_in_progress",
  "solution_proposed",
  "awaiting_info",
  "waiting_on_wso2",
  "closed",
];
const ANY_OF_STATE_OPTIONS = ANY_OF_STATE_VALUES.map((v) => ({
  value: beStateFromUi(v),
  label: STATE_LABEL[v],
}));

const ANY_OF_SEVERITY_VALUES: Severity[] = ["S0", "S1", "S2", "S3", "S4"];
const ANY_OF_SEVERITY_OPTIONS = ANY_OF_SEVERITY_VALUES.map((v) => ({
  value: priorityFromSeverity(v),
  label: v,
}));

const ANY_OF_ENGAGEMENT_TYPE_OPTIONS: { value: string; label: string }[] = [
  { value: "migration", label: "Migration" },
  { value: "consultancy", label: "Consultancy" },
  { value: "new_feature_improvement", label: "New feature / improvement" },
  { value: "follow_up", label: "Follow-up" },
  { value: "onboarding", label: "Onboarding" },
];

const ANY_OF_ISSUE_TYPE_OPTIONS = ALL_ISSUE_TYPES.map((v) => ({
  value: v,
  label: ISSUE_TYPE_LABEL[v],
}));

const ANY_OF_WORK_STATE_OPTIONS: { value: string; label: string }[] = [
  { value: "ongoing", label: "Ongoing" },
  { value: "paused", label: "Paused" },
];

const ESCALATION_LEVEL_OPTIONS: { value: string; label: string }[] = [
  "0",
  "1",
  "2",
  "3",
  "4",
  "5",
].map((v) => ({ value: v, label: v }));

/**
 * Ordered field catalogue backing the "OR groups" (`filters.anyOf`) branch
 * builder — the single source of truth for which fields are offered inside
 * a branch. Every field is real, backend-accepted, and matches the
 * `CaseFilterGroup` allowlist exactly; do not extend speculatively.
 */
export const ANY_OF_FILTER_FIELDS: AnyOfFilterFieldMeta[] = [
  {
    field: "type",
    label: "Case type",
    op: "in",
    opLabel: "is one of",
    valueKind: "multiSelect",
    options: ANY_OF_TYPE_OPTIONS,
  },
  {
    field: "state",
    label: "State",
    op: "in",
    opLabel: "is one of",
    valueKind: "multiSelect",
    options: ANY_OF_STATE_OPTIONS,
  },
  {
    field: "severity",
    label: "Severity",
    op: "in",
    opLabel: "is one of",
    valueKind: "multiSelect",
    options: ANY_OF_SEVERITY_OPTIONS,
  },
  {
    field: "engagementType",
    label: "Engagement type",
    op: "in",
    opLabel: "is one of",
    valueKind: "multiSelect",
    options: ANY_OF_ENGAGEMENT_TYPE_OPTIONS,
  },
  {
    field: "issueType",
    label: "Issue type",
    op: "in",
    opLabel: "is one of",
    valueKind: "multiSelect",
    options: ANY_OF_ISSUE_TYPE_OPTIONS,
  },
  {
    field: "workState",
    label: "Work state",
    op: "in",
    opLabel: "is one of",
    valueKind: "multiSelect",
    options: ANY_OF_WORK_STATE_OPTIONS,
  },
  {
    field: "projectId",
    label: "Project",
    op: "in",
    opLabel: "is one of",
    // Rendered by `AsyncProjectMultiSelect` (type-to-search) — same
    // component/hook the top-level "Project" bar control already uses.
    // Values are project ids.
    valueKind: "asyncProject",
  },
  {
    field: "deploymentId",
    label: "Deployment ID",
    op: "in",
    opLabel: "is one of",
    // No deployment-search component/hook exists anywhere in this webapp
    // (confirmed by search) and — unlike tags/teams/projects — there is no
    // small enumerable global set to suggest from, so this stays free text,
    // same as the top-level Advanced filters catalogue's own `deploymentId`.
    valueKind: "multiText",
    placeholder: "deployment id",
  },
  {
    field: "assignedUserId",
    label: "Assignee",
    op: "in",
    opLabel: "is one of",
    // Rendered by an async user-directory search (`/users/search`), same
    // directory the top-level "Assignee" bar control searches — values are
    // user ids (not emails), since that's what `assignedUserId` filters on
    // and picking from search results avoids a separate email→id resolution
    // step at request-build time.
    valueKind: "asyncAssignedUser",
  },
  {
    field: "escalationLevel",
    label: "Escalation level",
    op: "in",
    opLabel: "is one of",
    valueKind: "multiSelect",
    options: ESCALATION_LEVEL_OPTIONS,
  },
];

const ANY_OF_FIELD_META_BY_FIELD: Map<AnyOfFilterField, AnyOfFilterFieldMeta> = new Map(
  ANY_OF_FILTER_FIELDS.map((m) => [m.field, m]),
);

const ALL_ANY_OF_FIELDS: AnyOfFilterField[] = ANY_OF_FILTER_FIELDS.map((m) => m.field);

export function getAnyOfFilterFieldMeta(field: string): AnyOfFilterFieldMeta | undefined {
  return ANY_OF_FIELD_META_BY_FIELD.get(field as AnyOfFilterField);
}

/** One `anyOf` branch's row: an ad-hoc field/value predicate — no `op`
 * carried per-row, since every branch field is `in`-only (see
 * {@link AnyOfFilterFieldMeta}'s doc comment). */
export interface AnyOfFilterRow {
  field: AnyOfFilterField;
  values: string[];
}

/** One OR-branch: its own rows are ANDed together; branches themselves are
 * OR'd (see `filters.anyOf` in the `POST /cases/search` reference). */
export interface AnyOfBranch {
  filters: AnyOfFilterRow[];
}

export function defaultAnyOfFilterRow(): AnyOfFilterRow {
  return { field: ANY_OF_FILTER_FIELDS[0].field, values: [] };
}

export function defaultAnyOfBranch(): AnyOfBranch {
  return { filters: [defaultAnyOfFilterRow()] };
}

/** A row is only worth emitting once it carries at least one value — every
 * branch field in this catalogue requires `values` (none is a value-less op
 * like the top-level catalogue's `isEmpty`/`isNotEmpty`). */
export function isCompleteAnyOfFilterRow(row: AnyOfFilterRow): boolean {
  if (!getAnyOfFilterFieldMeta(row.field)) return false;
  return row.values.some((v) => v.trim().length > 0);
}

/** A branch is only worth emitting once it has at least one complete row —
 * the backend 400s on an empty branch (`"anyOf": [{}]`), so a branch with
 * zero complete conditions must never reach the request payload. */
export function isCompleteAnyOfBranch(branch: AnyOfBranch): boolean {
  return branch.filters.some(isCompleteAnyOfFilterRow);
}

/** Converts one complete `anyOf` row into the `BeCaseFieldFilter` shape.
 * Returns `undefined` for an incomplete row — callers should already have
 * filtered those out via {@link isCompleteAnyOfFilterRow}. */
export function anyOfFilterRowToFieldFilter(row: AnyOfFilterRow): BeCaseFieldFilter | undefined {
  const meta = getAnyOfFilterFieldMeta(row.field);
  if (!meta) return undefined;
  const values = row.values.map((v) => v.trim()).filter((v) => v.length > 0);
  if (values.length === 0) return undefined;
  return { field: row.field, op: meta.op, values };
}

/** Converts one complete branch into the `{filters: BeCaseFieldFilter[]}`
 * shape `filters.anyOf[i]` expects. Returns `undefined` for a branch with no
 * complete rows — never emits `{filters: []}`, which the backend also
 * rejects. */
export function anyOfBranchToPayload(
  branch: AnyOfBranch,
): { filters: BeCaseFieldFilter[] } | undefined {
  const filters = branch.filters
    .filter(isCompleteAnyOfFilterRow)
    .map(anyOfFilterRowToFieldFilter)
    .filter((f): f is BeCaseFieldFilter => f !== undefined);
  if (filters.length === 0) return undefined;
  return { filters };
}

const MAX_ANY_OF_BRANCHES = 10;
const MAX_ANY_OF_ROWS_PER_BRANCH = 20;
const MAX_ANY_OF_VALUE_LEN = 200;
const MAX_ANY_OF_VALUES_PER_ROW = 50;

/**
 * Parses the `anyOf` URL param (see `writeAnyOfBranchesParam`) back into
 * branches. Same "silently drop, never throw" policy as
 * `parseAdvancedFiltersParam`: a malformed param, an unknown field, or an
 * over-long/over-many value list degrades to "that row/branch is gone," not
 * a crash. An incomplete row (field picked, no value yet) — and an empty
 * branch (no rows at all, or only incomplete ones) — both round-trip here
 * unchanged: this is purely the URL-persistence boundary, the same
 * "never drop an in-progress row here" invariant `parseAdvancedFiltersParam`
 * documents, so a freshly-added branch/row never vanishes mid-edit. Only
 * {@link anyOfBranchToPayload} (the `/cases/search` request-payload
 * boundary) may drop an incomplete row or an empty branch.
 */
export function parseAnyOfBranchesParam(raw: string | null): AnyOfBranch[] {
  if (!raw) return [];
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch {
    return [];
  }
  if (!Array.isArray(parsed)) return [];

  const branches: AnyOfBranch[] = [];
  for (const branchEntry of parsed.slice(0, MAX_ANY_OF_BRANCHES)) {
    if (!Array.isArray(branchEntry)) continue;
    const rows: AnyOfFilterRow[] = [];
    for (const entry of branchEntry.slice(0, MAX_ANY_OF_ROWS_PER_BRANCH)) {
      if (!Array.isArray(entry) || entry.length < 1 || entry.length > 2) continue;
      const [field, rawValues] = entry as [unknown, unknown];
      if (typeof field !== "string") continue;
      if (!ALL_ANY_OF_FIELDS.includes(field as AnyOfFilterField)) continue;

      let values: string[] = [];
      if (rawValues !== undefined) {
        if (!Array.isArray(rawValues)) continue;
        values = rawValues
          .filter((v): v is string => typeof v === "string")
          .map((v) => v.slice(0, MAX_ANY_OF_VALUE_LEN))
          .slice(0, MAX_ANY_OF_VALUES_PER_ROW);
      }
      rows.push({ field: field as AnyOfFilterField, values });
    }
    // A branch with zero rows at all (every entry was malformed) is dropped
    // here — nothing to render — but a branch with only incomplete rows is
    // kept, same in-progress-survives-the-URL invariant as a single row.
    if (rows.length > 0) branches.push({ filters: rows });
  }
  return branches;
}

/** Serializes every branch (complete or not) as a single JSON-encoded
 * `anyOf` param value. */
export function writeAnyOfBranchesParam(branches: AnyOfBranch[]): string | null {
  if (branches.length === 0) return null;
  const nonEmptyBranches = branches.filter((b) => b.filters.length > 0);
  if (nonEmptyBranches.length === 0) return null;
  return JSON.stringify(
    nonEmptyBranches.map((b) =>
      b.filters.map((r) => (r.values.length > 0 ? [r.field, r.values] : [r.field])),
    ),
  );
}
