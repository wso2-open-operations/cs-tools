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

/** Marker param set only when the filters object being encoded/decoded uses
 * the case-search generic field/op/values DSL (see
 * `isCaseFieldFilterArray`) — so `parseWidgetPreviewFilters` knows to
 * reconstruct `{ filters: [...] }` rather than a flat key→values record. */
const CASE_FILTER_MARKER = "_cf";

const RESERVED_PARAMS = new Set(["w", "n", CASE_FILTER_MARKER]);

/** Placeholder swapped in for the signed-in user's own id wherever a
 * widget's (opaque, backend-resolved) filters carry it — e.g. "My Cases"
 * resolves to `assignedUserIds: ["<real uuid>"]` — so a bookmarked/shared
 * preview URL never carries a bare internal user id. */
const CURRENT_USER_SENTINEL = "@me";

/** Separates a field from a non-default op in a preview query param, e.g.
 * `tag~notIn=s_dip`. `~` is safe: every filter field name is camelCase
 * alphanumeric, so it can never appear in one. */
const OP_SEPARATOR = "~";

/** Ops that carry no `values` — they must still survive the round trip, so
 * they are encoded with an empty value (`escalation~isNotEmpty=`) rather than
 * skipped for being value-less. */
const VALUELESS_OPS = new Set(["isEmpty", "isNotEmpty"]);

function isStringArray(value: unknown): value is string[] {
  return Array.isArray(value) && value.every((v) => typeof v === "string");
}

/**
 * One entry of the case-search generic filter DSL (`BeCaseFieldFilter`),
 * structurally typed here (not imported from `types.ts`) since this file
 * works with every resourceType's opaque `Record<string, unknown>` filters,
 * not just case's.
 */
export interface WidgetCaseFieldFilterLike {
  field: string;
  op: string;
  values?: string[];
}

/**
 * True when `value` is the `filters` array of a case widget's filters object
 * (`{ filters: BeCaseFieldFilter[] }` — see `BeCaseSearchFilters`), detected
 * structurally so this file never needs to know the resourceType. Ops other
 * than `in` are common now (`notIn` tag exclusions, `isEmpty` for unassigned,
 * `isNotEmpty` for escalated), so the op is encoded in the query param
 * (`field~op`) and round-trips faithfully. It previously did NOT: every entry
 * decoded back as `op: "in"`, which inverted `notIn` — a tag EXCLUSION became
 * a tag filter — and value-less ops were dropped entirely.
 */
export function isCaseFieldFilterArray(value: unknown): value is WidgetCaseFieldFilterLike[] {
  return (
    Array.isArray(value) &&
    value.length > 0 &&
    value.every(
      (e) =>
        e !== null &&
        typeof e === "object" &&
        typeof (e as Record<string, unknown>).field === "string" &&
        typeof (e as Record<string, unknown>).op === "string",
    )
  );
}

/**
 * Builds the URL a dashboard widget tile's "View more" link points at — a
 * real, bookmarkable/shareable/refresh-safe URL (no router state): the
 * resource type is the path segment (`previewSlug`, from
 * `WIDGET_RESOURCE_CONFIG`), under the dashboard route's own static
 * `preview` prefix (`/dashboard/preview/:previewSlug` — see `App.tsx`, kept
 * distinct from `/dashboard/:dashboardId`'s open, BE-driven id space rather
 * than risking the two colliding), the widget's own id/display name are
 * `w`/`n` query params, and each filter field is its own readable query
 * param (e.g. `severities=critical`) rather than one opaque JSON blob — and
 * the signed-in user's own id, wherever it appears, is masked to `@me` (see
 * `CURRENT_USER_SENTINEL`). Read back by `parseWidgetPreviewFilters` /
 * `resolveCurrentUserSentinels` in `DashboardWidgetPreviewPage`.
 */
export function buildWidgetPreviewHref(params: {
  previewSlug: string;
  widgetId: string;
  displayName: string;
  filters: Record<string, unknown>;
  /** The signed-in user's own id, so it can be masked rather than embedded
   * verbatim in the URL. Omit if not yet known — the filter value(s) are
   * then left as-is rather than masked. */
  currentUserId?: string;
}): string {
  const q = new URLSearchParams();
  q.set("w", params.widgetId);
  q.set("n", params.displayName);
  let usesCaseFieldFilterShape = false;
  for (const [key, value] of Object.entries(params.filters)) {
    if (RESERVED_PARAMS.has(key)) continue;
    if (key === "filters" && isCaseFieldFilterArray(value)) {
      // Case widgets carry the generic field/op/values DSL nested under
      // `filters.filters` (see `BeCaseSearchFilters`/`isCaseFieldFilterArray`)
      // — flatten each entry to its own readable `field=values` query param
      // (e.g. `severity=critical,high`), matching the flat encoding below,
      // instead of surfacing one opaque JSON blob.
      usesCaseFieldFilterShape = true;
      for (const entry of value) {
        const values = entry.values ?? [];
        const op = entry.op || "in";
        // A value-less op (isEmpty/isNotEmpty) is the whole predicate, so it
        // must be emitted even with no values -- skipping it silently widened
        // e.g. "Unassigned Cases" into "all cases".
        if (values.length === 0 && !VALUELESS_OPS.has(op)) continue;
        const masked = values.map((v) =>
          v === params.currentUserId ? CURRENT_USER_SENTINEL : v,
        );
        // `in` keeps the bare `field=values` form so previously-shared links
        // still resolve; any other op is encoded as `field~op` so it survives
        // the round trip instead of silently decoding back as `in` (which
        // inverted `notIn` -- a tag EXCLUSION became a tag filter).
        q.set(op === "in" ? entry.field : `${entry.field}${OP_SEPARATOR}${op}`, masked.join(","));
      }
      continue;
    }
    if (isStringArray(value)) {
      if (value.length === 0) continue;
      const masked = value.map((v) =>
        v === params.currentUserId ? CURRENT_USER_SENTINEL : v,
      );
      q.set(key, masked.join(","));
    } else if (typeof value === "string") {
      q.set(key, value === params.currentUserId ? CURRENT_USER_SENTINEL : value);
    }
  }
  if (usesCaseFieldFilterShape) q.set(CASE_FILTER_MARKER, "1");
  return `/dashboard/preview/${params.previewSlug}?${q.toString()}`;
}

/** One human-readable "what's actually being queried" entry — a single
 * filter field and the value(s) it's currently set to, `op` set only for a
 * non-default (non-`in`) operator so a plain `field: value` reads cleanly
 * for the common case. Field names are the raw camelCase filter key (e.g.
 * `integrationCsTeam`); no friendly-label lookup exists for every filter
 * field across every resourceType, so this deliberately stays literal
 * rather than inventing a large label-mapping table for partial coverage. */
export interface WidgetFilterSummaryEntry {
  field: string;
  op?: string;
  value: string;
}

/**
 * Flattens a widget's (already fully-resolved — no `__current_team__`/`@me`
 * placeholders left in it) filters object into a readable list of active
 * filter criteria, for display on `DashboardWidgetPreviewPage` so a viewer
 * can see exactly what's being queried rather than trusting it silently.
 * Handles both filter shapes this app's widgets use: the case-search
 * generic field/op/values DSL (`{ filters: BeCaseFieldFilter[] }` — see
 * `isCaseFieldFilterArray`) and every other resourceType's flat
 * `{ fieldName: string[] }` record — the same two shapes
 * `buildWidgetPreviewHref` already branches on, reusing its own
 * value-less-op handling (`VALUELESS_OPS`) so an `isEmpty`/`isNotEmpty`
 * entry still shows up here instead of being silently skipped for
 * "having nothing to read".
 */
export function describeWidgetFilters(
  filters: Record<string, unknown>,
): WidgetFilterSummaryEntry[] {
  const entries: WidgetFilterSummaryEntry[] = [];
  const fieldFilters = filters.filters;

  if (isCaseFieldFilterArray(fieldFilters)) {
    for (const entry of fieldFilters) {
      const op = entry.op || "in";
      const values = entry.values ?? [];
      if (values.length === 0 && !VALUELESS_OPS.has(op)) continue;
      entries.push({
        field: entry.field,
        op: op === "in" ? undefined : op,
        value: values.length > 0 ? values.join(", ") : "(no value)",
      });
    }
    return entries;
  }

  for (const [key, value] of Object.entries(filters)) {
    if (RESERVED_PARAMS.has(key)) continue;
    if (isStringArray(value)) {
      if (value.length === 0) continue;
      entries.push({ field: key, value: value.join(", ") });
    } else if (typeof value === "string" && value.length > 0) {
      entries.push({ field: key, value });
    }
  }
  return entries;
}

export interface ParsedWidgetPreviewFilters {
  filters: Record<string, unknown>;
  /** True if a filter value still carries the `@me` sentinel and needs
   * `resolveCurrentUserSentinels` before it's safe to query with. */
  needsCurrentUser: boolean;
}

/** Parses every non-reserved (`w`/`n`) query param back into the widget's
 * filters object — the inverse of `buildWidgetPreviewHref`. Every value is
 * decoded as a comma-split string array (matching how every current dashboard
 * widget filter field is shaped — see `widgetResourceConfig.ts`'s
 * translators), so this never throws. */
export function parseWidgetPreviewFilters(
  searchParams: URLSearchParams,
): ParsedWidgetPreviewFilters {
  let needsCurrentUser = false;

  if (searchParams.get(CASE_FILTER_MARKER) === "1") {
    const fieldFilters: WidgetCaseFieldFilterLike[] = [];
    for (const [key, raw] of searchParams.entries()) {
      if (RESERVED_PARAMS.has(key)) continue;
      // `field~op` carries a non-default op; a bare `field` means `in`.
      const sep = key.indexOf(OP_SEPARATOR);
      const field = sep === -1 ? key : key.slice(0, sep);
      const op = sep === -1 ? "in" : key.slice(sep + OP_SEPARATOR.length);
      const values = raw === "" ? [] : raw.split(",");
      if (values.includes(CURRENT_USER_SENTINEL)) needsCurrentUser = true;
      fieldFilters.push({ field, op, values });
    }
    return { filters: { filters: fieldFilters }, needsCurrentUser };
  }

  const filters: Record<string, unknown> = {};
  for (const [key, raw] of searchParams.entries()) {
    if (RESERVED_PARAMS.has(key)) continue;

    const values = raw.split(",");
    if (values.includes(CURRENT_USER_SENTINEL)) needsCurrentUser = true;
    filters[key] = values;
  }

  return { filters, needsCurrentUser };
}

/** Substitutes the `@me` sentinel back to the signed-in user's own id —
 * see `buildWidgetPreviewHref`'s masking of that same id. Returns `filters`
 * unchanged if `currentUserId` isn't known yet (caller should hold off
 * querying in that case — see `needsCurrentUser`). */
export function resolveCurrentUserSentinels(
  filters: Record<string, unknown>,
  currentUserId: string | undefined,
): Record<string, unknown> {
  if (!currentUserId) return filters;
  const resolved: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(filters)) {
    if (key === "filters" && isCaseFieldFilterArray(value)) {
      resolved[key] = value.map((entry) => ({
        ...entry,
        values: entry.values?.map((v) =>
          v === CURRENT_USER_SENTINEL ? currentUserId : v,
        ),
      }));
      continue;
    }
    resolved[key] = Array.isArray(value)
      ? value.map((v) => (v === CURRENT_USER_SENTINEL ? currentUserId : v))
      : value === CURRENT_USER_SENTINEL
        ? currentUserId
        : value;
  }
  return resolved;
}
