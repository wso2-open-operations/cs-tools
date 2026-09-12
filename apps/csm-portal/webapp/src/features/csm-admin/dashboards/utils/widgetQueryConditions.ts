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

import type {
  BeCaseFieldFilterOp,
  BeDashboardFilterPreset,
  BeWidgetResourceType,
} from "@api/backend/types";

/**
 * A widget's `query` (opaque to every other part of this app — see
 * `BeDashboardWidget.query`) is NOT one uniform shape across every
 * resourceType's own `POST /{resourceType}s/search` contract:
 *
 * - `case` and its four `type`-variant resourceTypes (`service_request`,
 *   `security_report_analysis`, `announcement`, `engagement`) all route to
 *   `/cases/search`, whose filters are the generic field/op/values DSL
 *   nested under `query.filters` (see `BeCaseFieldFilter`).
 * - Every other resourceType (`incident`, `change_request`, `account`, …)
 *   has its own bespoke named-field filter shape, flat under `query`
 *   itself — there is no single generic DSL for these anywhere in this
 *   app. A widget's `query` here corresponds to the INNER `filters` object
 *   of that resourceType's own search payload (e.g. `query.priorities`
 *   maps onto `BeIncidentSearchPayload.filters.priorities`, NOT
 *   `BeIncidentSearchPayload.filters.filters.priorities`) — the outer
 *   `{ filters: <query>, pagination, sortBy }` envelope is added by
 *   `useWidgetData` at request time, not stored as part of the widget's own
 *   `query`. Nesting `query` itself under one more `filters` key here would
 *   double-wrap it and produce a request the real endpoint doesn't
 *   recognize.
 *
 * This module gives the widget editor ONE condition-row UI (field,
 * operator, value(s)) that round-trips through whichever of those two
 * shapes actually matches the widget's own `resourceType`, rather than
 * forcing every resourceType's filters into the case DSL (which its real
 * search endpoint would reject) or exposing raw JSON.
 */

export type FilterConditionOp = BeCaseFieldFilterOp;

export const FILTER_CONDITION_OPS: FilterConditionOp[] = [
  "eq",
  "in",
  "notIn",
  "gte",
  "lte",
  "isEmpty",
  "isNotEmpty",
];

/** One editable filter row. `values` is ignored for `isEmpty`/`isNotEmpty`
 * (those two ops are value-less predicates — see `BeCaseFieldFilter`).
 *
 * A row is EITHER a literal field predicate or a shared-preset reference,
 * discriminated by `preset`: when `preset` is a non-empty name, `field`/`op`/
 * `values` carry no meaning and the row serializes as `{"preset": name}`.
 * Modelled as one optional key rather than a discriminated union so every
 * existing call site that constructs or patches a plain field row keeps
 * working unchanged — `isPresetCondition` is the single place that decides
 * which kind a row is. */
export interface FilterCondition {
  field: string;
  op: FilterConditionOp;
  values: string[];
  /**
   * Name of a shared filter preset this row references instead of spelling
   * the predicate out (see `BeDashboardFilterPreset`). Only meaningful for a
   * resourceType that uses the case field DSL: presets are expanded inside
   * `query.filters`, and no other resourceType's search contract has such an
   * array at all.
   */
  preset?: string;
}

/** Whether a row is a shared-preset reference rather than a literal field
 * predicate. The one authority on that distinction — see `FilterCondition`. */
export function isPresetCondition(condition: FilterCondition): boolean {
  return typeof condition.preset === "string" && condition.preset.length > 0;
}

const NO_VALUE_OPS = new Set<FilterConditionOp>(["isEmpty", "isNotEmpty"]);

/** resourceTypes that route to `/cases/search` and therefore use the
 * generic case field/op/values DSL — see this module's own doc comment. */
const CASE_FIELD_DSL_RESOURCE_TYPES = new Set<BeWidgetResourceType>([
  "case",
  "service_request",
  "security_report_analysis",
  "announcement",
  "engagement",
]);

export function usesCaseFieldFilterDsl(resourceType: BeWidgetResourceType): boolean {
  return CASE_FIELD_DSL_RESOURCE_TYPES.has(resourceType);
}

/**
 * The only two ops a non-case resourceType's own flat named-key search
 * contract is proven to support anywhere in this app (see every
 * `BeXxxSearchPayload.filters` shape in `types.ts`: `priorities`/`states`/
 * `impacts` as plain arrays, `slaViolated`/`number` etc. as plain scalars —
 * there is no generic `notIn`/`gte`/`lte`/`isEmpty`/`isNotEmpty` key
 * convention across those bespoke, per-field contracts the way the case DSL
 * has one uniform `field`/`op`/`values` shape). Offering the other five ops
 * for a non-case resourceType would let the admin build a filter this app
 * cannot serialize correctly — see `queryFromFilterConditions`'s own doc
 * comment on what happens to an unsupported op that slips through anyway
 * (legacy/hand-edited data only; the editor never creates one).
 */
const NON_CASE_SUPPORTED_OPS: FilterConditionOp[] = ["eq", "in"];

/** The operators that make sense to offer in the condition editor for a
 * given resourceType — every op for the case DSL, only `eq`/`in` for
 * anything else (see `NON_CASE_SUPPORTED_OPS`). */
export function operatorsForResourceType(
  resourceType: BeWidgetResourceType,
): FilterConditionOp[] {
  return usesCaseFieldFilterDsl(resourceType) ? FILTER_CONDITION_OPS : NON_CASE_SUPPORTED_OPS;
}

/**
 * Best-effort scalar type recovery for a condition row's freeform text
 * value(s): a non-case resourceType's own contract carries real JSON types
 * (e.g. `BeIncidentSearchPayload.filters.slaViolated: boolean`), not the
 * string this editor's text inputs always produce — writing the raw string
 * back (`"true"` instead of `true`) either fails that endpoint's own
 * validation or silently never matches. `"true"`/`"false"` (case-sensitive,
 * matching how a boolean stringifies) become real booleans, a value that
 * parses as a plain integer or decimal becomes a real number, everything
 * else stays a string. Never applied to the case field/op/values DSL, whose
 * `values` are always `string[]` on the wire regardless of the field's own
 * semantic type (see `BeCaseFieldFilter`).
 */
function coerceScalar(value: string): unknown {
  if (value === "true") return true;
  if (value === "false") return false;
  const trimmed = value.trim();
  if (trimmed.length > 0 && /^-?\d+(\.\d+)?$/.test(trimmed)) {
    const n = Number(trimmed);
    // Only converted when the number round-trips back to the EXACT same
    // text — this naturally rejects a leading-zero identifier (`"0090472"`,
    // `Number("0090472")` -> `90472` -> `String(90472)` -> `"90472"` !==
    // `"0090472"`) and a value that lost precision above
    // `Number.MAX_SAFE_INTEGER`, both of which would otherwise be silently
    // corrupted straight into the deployable widget JSON.
    if (Number.isFinite(n) && String(n) === trimmed) return n;
  }
  return value;
}

/** Every field the case-search DSL accepts (mirrors `BeCaseFieldFilterField`
 * — see `types.ts`), offered as autocomplete suggestions in the field
 * picker for a case-like resourceType. Freeform text is still accepted:
 * this is a suggestion list, not a hard allowlist, since the backend (not
 * this list) is the source of truth for what it accepts. */
export const CASE_FIELD_OPTIONS: string[] = [
  "type",
  "state",
  "severity",
  "engagementType",
  "issueType",
  "workState",
  "tag",
  "projectId",
  "deploymentId",
  "assignedUserId",
  "createdBy",
  "createdOn",
  "updatedOn",
  "closedOn",
  "product",
  "projectOnboardingStatus",
  "projectType",
  "creTeam",
  "sreTeam",
  "resolutionNotes",
  "parentId",
  "taskSLABusinessElapsedPercent",
  "escalationLevel",
  "escalation",
  "number",
  "internalId",
];

/**
 * Deterministic stringification of a filter object (keys sorted recursively),
 * so two structurally-equal predicates that merely differ in key order
 * compare equal. Same technique, and same reason, as `dashboardDrift`'s own
 * `canonicalize`: the backend's JSON key order is not something this app
 * should depend on.
 */
function canonicalFilter(value: unknown): string {
  const canonical = (v: unknown): unknown => {
    if (Array.isArray(v)) return v.map(canonical);
    if (v && typeof v === "object") {
      const out: Record<string, unknown> = {};
      for (const k of Object.keys(v as Record<string, unknown>).sort()) {
        out[k] = canonical((v as Record<string, unknown>)[k]);
      }
      return out;
    }
    return v;
  };
  return JSON.stringify(canonical(value));
}

/**
 * Reverse index from a preset's expanded predicate to its name, used to show
 * an already-deployed widget's filters as preset rows again.
 *
 * This is needed because `GET /dashboards/{id}` serves a dashboard with every
 * preset reference ALREADY EXPANDED and erased — so without this, opening a
 * deployed dashboard in the builder would show literal predicates, and saving
 * it would export those literals, silently stripping every preset reference
 * the deployed definition had.
 *
 * When two presets share an identical predicate the first by name wins, which
 * is deterministic because the catalogue endpoint returns them name-sorted.
 * The cost of this approach is the converse case: a hand-authored literal
 * predicate that happens to equal a preset's body is shown, and re-exported,
 * as that preset. That is a deliberate trade — the two are semantically
 * identical by definition, since the preset expands to exactly that predicate
 * — and it is what keeps this working with no extra field on the widget API.
 */
export function presetNameByFilterBody(
  presets: readonly BeDashboardFilterPreset[] | undefined,
): Map<string, string> {
  const index = new Map<string, string>();
  for (const preset of presets ?? []) {
    const key = canonicalFilter(preset.filter);
    if (!index.has(key)) index.set(key, preset.name);
  }
  return index;
}

function isFilterOp(v: unknown): v is FilterConditionOp {
  return typeof v === "string" && (FILTER_CONDITION_OPS as string[]).includes(v);
}

/** Reads a widget's own `query` into editable condition rows, per
 * `usesCaseFieldFilterDsl`. An unrecognized/malformed entry is skipped
 * rather than crashing the editor — the admin can always still delete/
 * retype a row that came out empty. */
export function filterConditionsFromQuery(
  resourceType: BeWidgetResourceType,
  query: Record<string, unknown> | null | undefined,
  presets?: readonly BeDashboardFilterPreset[],
): FilterCondition[] {
  if (!query) return [];

  if (usesCaseFieldFilterDsl(resourceType)) {
    const raw = query.filters;
    if (!Array.isArray(raw)) return [];
    const byBody = presetNameByFilterBody(presets);
    return raw
      .filter((e): e is Record<string, unknown> => !!e && typeof e === "object")
      .map((e): FilterCondition => {
        // An authored `{"preset": name}` entry, which only reaches here from
        // a local draft or a section definition — a dashboard served by the
        // API has had these expanded away.
        if (typeof e.preset === "string" && e.preset.length > 0) {
          return { field: "", op: "eq", values: [], preset: e.preset };
        }
        // An expanded predicate that a preset accounts for: shown as that
        // preset again, so a round-trip through this editor does not strip
        // the reference. See `presetNameByFilterBody`.
        const collapsed = byBody.get(canonicalFilter(e));
        if (collapsed !== undefined) {
          return { field: "", op: "eq", values: [], preset: collapsed };
        }
        return {
          field: typeof e.field === "string" ? e.field : "",
          op: isFilterOp(e.op) ? e.op : "eq",
          values: Array.isArray(e.values) ? e.values.map(String) : [],
        };
      })
      // A row with neither a field nor a preset carries no meaning at all
      // (a malformed entry); dropped rather than crashing the editor.
      .filter((c) => isPresetCondition(c) || c.field.length > 0);
  }

  // Every other resourceType's own search contract is flat named top-level
  // keys, not this app's field/op/values DSL — one row per key. `in` for an
  // array value (e.g. `priorities: ["HIGH"]`), `eq` for a scalar (e.g.
  // `number: "INC0090472"`).
  return Object.entries(query)
    .filter(([, v]) => v !== undefined && v !== null)
    .map(([field, v]) => ({
      field,
      op: (Array.isArray(v) ? "in" : "eq") as FilterConditionOp,
      values: Array.isArray(v) ? v.map(String) : [String(v)],
    }));
}

/** The inverse of `filterConditionsFromQuery` — serializes edited condition
 * rows back into the `query` shape that resourceType's own search endpoint
 * actually accepts. Rows with an empty `field` are dropped. */
export function queryFromFilterConditions(
  resourceType: BeWidgetResourceType,
  conditions: FilterCondition[],
): Record<string, unknown> {
  const valid = conditions.filter(
    (c) => isPresetCondition(c) || c.field.trim().length > 0,
  );

  if (usesCaseFieldFilterDsl(resourceType)) {
    if (valid.length === 0) return {};
    return {
      filters: valid.map((c) => {
        // A preset row writes the reference and nothing else: the whole
        // point is that the predicate lives in the shared catalogue, so
        // emitting field/op/values alongside it would defeat it (and the
        // backend rejects an entry carrying both).
        if (isPresetCondition(c)) return { preset: c.preset };
        return NO_VALUE_OPS.has(c.op)
          ? { field: c.field, op: c.op }
          : { field: c.field, op: c.op, values: c.values };
      }),
    };
  }

  // A preset row cannot be represented for a non-case resourceType: presets
  // are expanded inside `query.filters`, and these contracts have no such
  // array. The editor never offers one here (see the condition editor), so
  // this can only come from a resourceType switch on an existing widget.
  // Dropped for the same reason an unsupported op is, below: a visible,
  // recoverable gap beats a silently reinterpreted filter.

  const out: Record<string, unknown> = {};
  for (const c of valid) {
    if (isPresetCondition(c)) continue;
    // Non-case resourceTypes' own contracts only ever use a scalar or an
    // array (see this module's own doc comment), with no per-field op of
    // their own at all — `in` writes the array, `eq` writes a single
    // type-recovered scalar. Any OTHER op (`notIn`/`gte`/`lte`/`isEmpty`/
    // `isNotEmpty`) can only reach here from data the editor itself never
    // produces (`operatorsForResourceType` never offers them for a
    // non-case resourceType) — most likely a hand-edited deployed widget
    // JSON. There is no flat-contract encoding of those ops' real meaning
    // ("not X", a range, ...), so this row is dropped from the output
    // rather than reinterpreted as `eq`: a dropped filter is a visible,
    // recoverable gap (temporarily unenforced, admin can re-add it); a
    // `notIn` silently rewritten to `eq` on the very next save would flip
    // its real meaning ("not X" -> "is X") without the admin ever having
    // touched that row.
    if (c.op !== "eq" && c.op !== "in") continue;
    out[c.field] = c.op === "in" ? c.values.map(coerceScalar) : coerceScalar(c.values[0] ?? "");
  }
  return out;
}
