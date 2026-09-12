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

/** Carries a case widget's `anyOf` cross-field-OR branches (see
 * `isAnyOfBranchArray`) as one JSON blob, round-tripped by
 * `buildWidgetPreviewHref`/`parseWidgetPreviewFilters` below. Unlike every
 * other filter field here, an `anyOf` branch can filter on any field and
 * there can be several branches, so there's no natural one-field-one-param
 * encoding the way `severities=critical` works for a flat AND'd field —
 * this is the one deliberate exception to this file's "no opaque JSON blob"
 * approach, scoped to just this one nested construct. */
const ANY_OF_PARAM = "_anyOf";

const RESERVED_PARAMS = new Set(["w", "n", CASE_FILTER_MARKER, ANY_OF_PARAM]);

/**
 * Query param carrying a dashboard widget's own `displayName`, appended to a
 * click-through href for the two resourceTypes (`case`, `engagement`) whose
 * `buildHref` lands on a real, permanent nav page (`/cases`, `/engagements`
 * — see `caseFamilyBuildHref`'s fallback in `widgetResourceConfig.ts`)
 * rather than the generic dashboard-widget preview page above. Those nav
 * pages otherwise render a hardcoded heading ("Cases"/"Engagements"); this
 * param lets them show the originating widget's own name instead, without
 * affecting a plain nav visit (which never sets it).
 *
 * Deliberately a *different* param than `n` above: `n` belongs to the
 * dashboard-widget preview page's own URL scheme (paired with `w`, and read
 * by `parseWidgetPreviewFilters`'s caller, not this file). `wt` ("widget
 * title") is scoped to this narrower, single-purpose use — a heading
 * override, not a full preview-page identity — and deliberately excluded
 * from `RESERVED_PARAMS`/`parseWidgetPreviewFilters`'s filter parsing since
 * it never reaches that code path (`/cases`/`/engagements` have their own
 * filter parsing in `casesFiltersUrl.ts`, which never reads this key either
 * — purely cosmetic, heading-only).
 */
export const WIDGET_TITLE_PARAM = "wt";

/** Appends `WIDGET_TITLE_PARAM` (the click-through widget's own
 * `displayName`) to an already-built `/cases`/`/engagements` href, so that
 * page's heading can show it instead of falling back to its hardcoded
 * default. A no-op (returns `href` unchanged) when `displayName` is absent
 * or empty — never emits `wt=` with nothing meaningful in it. */
export function appendWidgetTitleParam(href: string, displayName: string | undefined): string {
  if (!displayName) return href;
  const [path, query = ""] = href.split("?");
  const params = new URLSearchParams(query);
  params.set(WIDGET_TITLE_PARAM, displayName);
  return `${path}?${params.toString()}`;
}

/** Reads `WIDGET_TITLE_PARAM` back off a `/cases`/`/engagements` URL's own
 * search params — the inverse of `appendWidgetTitleParam`. `undefined` when
 * absent or empty, so a caller can cleanly fall back to that page's own
 * default heading. */
export function readWidgetTitleParam(searchParams: URLSearchParams): string | undefined {
  const v = searchParams.get(WIDGET_TITLE_PARAM);
  return v && v.length > 0 ? v : undefined;
}

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

/** One `anyOf` branch: its own predicate array, ANDed within the branch, OR'd
 * against every other branch — matches the backend's `CaseFilterBranch` and
 * `widgetFilterMerge.ts`'s own (independently duplicated, not imported here
 * to avoid a circular import — `widgetFilterMerge.ts` already imports from
 * this file) structural check of the identical shape. */
export interface WidgetAnyOfBranch {
  filters: WidgetCaseFieldFilterLike[];
}

function isAnyOfBranch(value: unknown): value is WidgetAnyOfBranch {
  return (
    typeof value === "object" &&
    value !== null &&
    Array.isArray((value as { filters?: unknown }).filters)
  );
}

/** True when `value` is a case widget's `anyOf` cross-field-OR branch array
 * (`{filters: BeCaseFieldFilter[]}[]`, at least one branch). */
export function isAnyOfBranchArray(value: unknown): value is WidgetAnyOfBranch[] {
  return Array.isArray(value) && value.length > 0 && value.every(isAnyOfBranch);
}

/**
 * Builds the URL a dashboard widget tile's "View more" link points at — a
 * real, bookmarkable/shareable/refresh-safe URL (no router state): the
 * resource type is the path segment (`previewSlug`, from
 * `WIDGET_RESOURCE_CONFIG`), the widget's own id/display name are `w`/`n`
 * query params, and each filter field is its own readable query param
 * (e.g. `severities=critical`) rather than one opaque JSON blob — and the
 * signed-in user's own id, wherever it appears, is masked to `@me` (see
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
    if (key === "anyOf" && isAnyOfBranchArray(value)) {
      // No natural per-field flat encoding exists for a cross-field OR (a
      // branch can filter on any field, and there can be several branches),
      // so this round-trips as one JSON blob (see `ANY_OF_PARAM`'s doc
      // comment) rather than being silently skipped the way it was before —
      // that silent drop is exactly what let a "View more" click-through
      // land on a broader, unfiltered-by-`anyOf` result set than the tile
      // it was clicked from had counted.
      const masked = value.map((branch) => ({
        ...branch,
        filters: branch.filters.map((f) => ({
          ...f,
          values: f.values?.map((v) => (v === params.currentUserId ? CURRENT_USER_SENTINEL : v)),
        })),
      }));
      q.set(ANY_OF_PARAM, JSON.stringify(masked));
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
    } else if (typeof value === "number" && Number.isFinite(value)) {
      // A slice's own `query` can carry a plain number (e.g. the rating pie's
      // `{ rating: 5 }`, from `Math.round(avgRating)` — see
      // `useCaseFeedbackTrendData`), not just the string/string[] shape every
      // other widget's filters use. Dropped silently before this branch
      // existed: a rating-pie click-through landed on the unfiltered list
      // with no `rating` param in the URL at all.
      q.set(key, String(value));
    }
  }
  if (usesCaseFieldFilterShape) q.set(CASE_FILTER_MARKER, "1");
  // Under "/dashboard/preview/", not directly under "/dashboard/" — that
  // shape collides with the dashboard-selection route
  // (`/dashboard/:dashboardId`, see App.tsx), so this needs its own static
  // prefix rather than sharing the single-dynamic-segment shape.
  return `/dashboard/preview/${params.previewSlug}?${q.toString()}`;
}

/** One human-readable "what's actually being queried" entry — a single
 * filter field and the value(s) it's currently set to, `op` set only for a
 * non-default (non-`in`) operator so a plain `field: value` reads cleanly
 * for the common case. Field names are the raw camelCase filter key (e.g.
 * `creTeam`); no friendly-label lookup exists for every filter
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

  // `anyOf` round-trips as its own single JSON param regardless of which of
  // the two shapes below applies — a case-family widget with `anyOf` branches
  // still carries the flat `filters.filters` array (or a flat field record,
  // for a non-case-family resourceType) alongside it, so this is read once,
  // up front, and merged into whichever `filters` object gets built below.
  let anyOf: WidgetAnyOfBranch[] | undefined;
  const anyOfRaw = searchParams.get(ANY_OF_PARAM);
  if (anyOfRaw) {
    try {
      const parsed: unknown = JSON.parse(anyOfRaw);
      if (isAnyOfBranchArray(parsed)) {
        anyOf = parsed;
        if (parsed.some((b) => b.filters.some((f) => f.values?.includes(CURRENT_USER_SENTINEL)))) {
          needsCurrentUser = true;
        }
      }
    } catch {
      // A malformed/hand-edited URL drops just `anyOf` rather than throwing —
      // every other filter param still parses and queries normally.
    }
  }

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
    return {
      filters: anyOf ? { filters: fieldFilters, anyOf } : { filters: fieldFilters },
      needsCurrentUser,
    };
  }

  const filters: Record<string, unknown> = {};
  for (const [key, raw] of searchParams.entries()) {
    if (RESERVED_PARAMS.has(key)) continue;

    const values = raw.split(",");
    if (values.includes(CURRENT_USER_SENTINEL)) needsCurrentUser = true;
    filters[key] = values;
  }
  if (anyOf) filters.anyOf = anyOf;

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
    if (key === "anyOf" && isAnyOfBranchArray(value)) {
      // Same nested-substitution as `filters` above -- the generic
      // `Array.isArray` branch below only replaces a sentinel that is itself
      // one of the array's own elements, which would leave `@me` unresolved
      // inside a branch's own `filters` entries (branch objects, not bare
      // sentinel strings).
      resolved[key] = value.map((branch) => ({
        ...branch,
        filters: branch.filters.map((f) => ({
          ...f,
          values: f.values?.map((v) => (v === CURRENT_USER_SENTINEL ? currentUserId : v)),
        })),
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
