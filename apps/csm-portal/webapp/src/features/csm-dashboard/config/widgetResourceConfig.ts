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
  AlertOctagon,
  AlertTriangle,
  Briefcase,
  Building2,
  CheckSquare,
  Clock,
  Cog,
  FolderKanban,
  GitPullRequest,
  Handshake,
  Megaphone,
  Shield,
  ShieldAlert,
  Star,
  Users,
  ListChecks,
  type LucideIcon,
} from "@wso2/oxygen-ui-icons-react";
import type { BeCallRequestStateKey, BeWidgetResourceType } from "@api/backend/types";
import { humanizeState } from "@features/csm-dashboard/utils/abtDashboard";
import type { CaseState } from "@features/csm-dashboard/types/abtDashboard";
import {
  casesHref,
  DEFAULT_CASES_FILTERS,
  writeCasesFiltersToUrl,
} from "@features/csm-cases/utils/casesFiltersUrl";
import type { CasesFilters } from "@features/csm-cases/components/CasesFilterBar";
import {
  ALL_ONBOARDING_STATUSES,
  ONBOARDING_STATUS_NO_MATCH,
} from "@features/csm-cases/utils/onboardingStatus";
import type { Severity } from "@features/csm-dashboard/types/abtDashboard";
import {
  DEFAULT_INCIDENT_FILTERS,
  type IncidentFilters,
} from "@features/csm-operations/utils/incidents";
import { writeIncidentFiltersToUrl } from "@features/csm-operations/utils/incidentsFiltersUrl";
import {
  DEFAULT_CR_FILTERS,
  type ChangeRequestFilters,
} from "@features/csm-operations/utils/changeRequests";
import { writeChangeRequestFiltersToUrl } from "@features/csm-operations/utils/changeRequestsFiltersUrl";
import { taskStateLabel } from "@features/csm-cases/utils/taskState";
import type { BeTaskState } from "@api/backend/types";
import {
  appendWidgetTitleParam,
  buildWidgetPreviewHref,
  isAnyOfBranchArray,
} from "@features/csm-dashboard/utils/widgetPreviewUrl";

/** A resolved search-result row, typed loosely since its real shape depends
 * on `resourceType` — the label extractors below narrow what they read. */
type WidgetItem = Record<string, unknown>;

/**
 * Per-resource-type wiring for a dashboard widget: where to fetch its data,
 * how to read a list-shape row for display, and where a click on the tile
 * navigates.
 */
export interface WidgetResourceConfig {
  /** `POST` endpoint this resource's own search lives at. */
  searchEndpoint: string;
  /** `POST` endpoint for a server-side group-by aggregation, for a
   * `shape: "pie"`/`"bar"` widget configured with `groupBy` instead of
   * `slices` (see `useWidgetGroupByData`). Only the resourceTypes backed
   * by an entity-service group-by endpoint carry this — omitted means that
   * resourceType doesn't support `groupBy` widgets at all. */
  groupByEndpoint?: string;
  /** Key the response's item array is nested under. */
  itemsKey: string;
  /** Primary (bold) line for one list-shape row. */
  primaryLabel: (item: WidgetItem) => string;
  /** Optional secondary (muted) line for one list-shape row. */
  secondaryLabel?: (item: WidgetItem) => string | undefined;
  /** Where a click on this widget's tile navigates, given its (opaque,
   * already current-user-resolved) filters. `ctx` (the owning widget's id
   * and resolved display name) is only there for a resourceType with no
   * dedicated list route of its own (see `incident_task` below) to route
   * through the generic dashboard-widget preview page instead, which is the
   * only destination that can render this exact widget's own filtered
   * result set — every other resourceType ignores it. */
  buildHref: (
    filters: Record<string, unknown>,
    ctx?: { widgetId: string; displayName: string },
  ) => string;
  /** Icon shown on the tile, one per resource type (not per individual
   * widget — the backend registry doesn't carry per-widget icon metadata). */
  icon: LucideIcon;
  /** Theme palette key the icon (and nothing else — see DashboardWidgetTile's
   * hover treatment) is colored with. */
  iconColor: "primary" | "secondary" | "success" | "error" | "info" | "warning";
  /** Friendly plural URL segment for this resource type's dashboard-widget
   * "View more" preview page, e.g. `/dashboard/cases`. Distinct from that
   * resource's own tab path (`buildHref`'s destination) — this route is
   * dashboard-widget-scoped, not the resource's real list page. */
  previewSlug: string;
  /** One search-result item's own detail-page href, given the raw item —
   * used by the generic `columns`-driven list renderer (see
   * `GenericColumnList`), which has no resourceType-specific row-link logic
   * of its own the way each hardcoded renderer in `widgetListConfig.tsx`
   * does. `undefined` for a resourceType with no standalone detail route
   * (task, call_request's own record — `call_request` links to its owning
   * case instead, handled the same way the hardcoded `CallRequestWidgetList`
   * does) or when the item carries no usable id. */
  detailHref?: (item: WidgetItem) => string | undefined;
  /** Only meaningful for shape "list". Overrides the default request body
   * `useWidgetData` POSTs to `searchEndpoint` (`{ filters, pagination: {
   * offset, limit }, sortBy? }`) — every resourceType's own search contract
   * uses that shape EXCEPT `case_feedback`'s `POST /cases/feedback/search`,
   * which takes flat `page`/`pageSize` instead of `pagination.offset/limit`
   * (see that entry's own comment for why). Omitted (every other
   * resourceType) keeps `useWidgetData`'s existing request shape untouched. */
  buildSearchRequestBody?: (args: {
    filters: Record<string, unknown>;
    offset: number;
    limit: number;
    sortBy?: Record<string, unknown>;
  }) => Record<string, unknown>;
  /** Only meaningful for shape "list"/"count". Overrides how `useWidgetData`
   * reads `total`/the item page off `searchEndpoint`'s own response (default:
   * `res.total`, `res[itemsKey]`) — only `case_feedback`'s response diverges
   * (`totalRecords`/`results`, not `total`/`itemsKey`). Omitted keeps the
   * default reading untouched. */
  parseSearchResponse?: (res: Record<string, unknown>) => {
    total: number;
    items: Record<string, unknown>[];
  };
}

function asString(v: unknown): string | undefined {
  return typeof v === "string" && v.length > 0 ? v : undefined;
}

function asStringArray(v: unknown): string[] | undefined {
  return Array.isArray(v) && v.every((x) => typeof x === "string")
    ? (v as string[])
    : undefined;
}

// ---------------------------------------------------------------------------
// case — /cases, translating the opaque dashboard filters into CasesFilters.
// ---------------------------------------------------------------------------

/**
 * The dashboard/entity-service case severity values are the lowercase
 * `catastrophic|critical|high|medium|low` enum; the cases list's own filter
 * bar (and its URL encoding) uses the `S0`..`S4` codes instead. No existing
 * mapping between the two lives anywhere else in the app (the app-wide
 * `SEVERITY_LABEL` maps `S0` -> "Catastrophic", a display label, not this
 * enum) — scoped here to dashboard click-through only.
 */
const DASHBOARD_SEVERITY_TO_S_CODE: Record<string, Severity> = {
  catastrophic: "S0",
  critical: "S1",
  high: "S2",
  medium: "S3",
  low: "S4",
};

/**
 * One entry of the case-search generic filter DSL (`BeCaseFieldFilter`),
 * structurally typed here (not imported from `types.ts`) since this reads a
 * caller-opaque `Record<string, unknown>`, not a typed request body.
 */
interface CaseDashboardFieldFilter {
  field: string;
  op: string;
  values?: string[];
}

function asCaseFieldFilters(v: unknown): CaseDashboardFieldFilter[] | undefined {
  if (!Array.isArray(v)) return undefined;
  return v.every(
    (e): e is CaseDashboardFieldFilter =>
      !!e && typeof e === "object" && typeof (e as Record<string, unknown>).field === "string",
  )
    ? v
    : undefined;
}

/** Reads the `values` of the first entry matching `field` in a case widget's
 * `filters.filters` array, or `undefined` if that field isn't present. */
function caseFilterValues(
  fieldFilters: CaseDashboardFieldFilter[] | undefined,
  field: string,
): string[] | undefined {
  return fieldFilters?.find((f) => f.field === field)?.values;
}

/** Reads the first entry matching `field` AND `op` in a case widget's
 * `filters.filters` array, or `undefined` if no such combination is present
 * — used for fields where the op itself carries meaning (`tag` in vs.
 * notIn, `escalation`'s value-less isEmpty vs. isNotEmpty, and each
 * gte/lte range bound), so the wrong op is never silently matched. */
function caseFilterEntry(
  fieldFilters: CaseDashboardFieldFilter[] | undefined,
  field: string,
  op: string,
): CaseDashboardFieldFilter | undefined {
  return fieldFilters?.find((f) => f.field === field && f.op === op);
}

/**
 * Translate a dashboard widget's opaque case filters (the `POST
 * /cases/search`-shaped `{ filters: BeCaseFieldFilter[] }` body — see
 * `BeCaseSearchFilters`) into the cases list's own `CasesFilters` shape.
 * Every field the case-search DSL supports (see `caseFilterFieldSet` in
 * `case_filters.go`) now has a home in `CasesFilters` and is passed through
 * — this used to drop `taskSLABusinessElapsedPercent`, `escalationLevel`/
 * `escalation`, `creTeam`/`sreTeam`, `tag`, `projectOnboardingStatus`,
 * `projectType`, and the `createdOn`/`updatedOn`/`closedOn` date ranges,
 * which was the root cause of the click-through data-loss bug this function
 * exists to fix (a tile reading a filtered count landed on the org-wide
 * cases list because its filters had nowhere to go). `parentId` and
 * `resolutionNotes` remain genuinely dropped: no dashboard widget uses them
 * today, and `CasesFilters` has no equivalent to invent one for without
 * guessing at a UI treatment. `anyOf` (cross-field OR) is dropped here too —
 * `CasesFilters` is an AND-only model with nothing to translate it into — but
 * a widget carrying `anyOf` never reaches this function's output at all (see
 * `caseFamilyBuildHref` below, and `DashboardWidgetPreviewPage.tsx`'s own
 * `anyOf` routing check): its click-through routes to the generic,
 * filter-faithful dashboard-widget preview page instead, the same fallback
 * `incident_task` uses for a resourceType with no representable destination
 * in its own list page.
 *
 * `assignedUserId` carries the current user's own UUID (every widget that
 * sets it does so via the current-user placeholder), and
 * `CasesFilters.assignees` is email/`@me`-based with no UUID lookup
 * available here — since these widgets only ever filter "assigned to me",
 * any non-empty `assignedUserId` maps to the `@me` sentinel rather than an
 * (unresolvable) literal UUID.
 *
 * `tag`, `state`, and `projectOnboardingStatus` — the only 3 case-search
 * fields whose backend contract accepts `notIn` at all (`case_filters.go`'s
 * per-field op table) — are each matched on field *and* op together via
 * `caseFilterEntry`, never read op-blind via `caseFilterValues` plus an
 * inferred op. A `notIn` widget filter on any of these can then never
 * silently decode as its own opposite (an inclusion of exactly the values it
 * meant to exclude) — which is exactly the bug once reported against a live
 * `projectOnboardingStatus notIn` widget click-through, and the same bug the
 * dashboard preview URL shipped once for `tag` (see `casesFiltersUrl.ts`'s
 * `writeCasesFiltersToUrl` doc comment for that full story). `tag`/`state`
 * map their two ops to two distinct `CasesFilters` fields (`tags`/
 * `excludeTags`, `states`/`excludeStates`). `projectOnboardingStatus` is the
 * exception: its domain is the 4 fixed values in `onboardingStatus.ts`, so
 * instead of a third `excludeOnboardingStatuses` field, a `notIn` filter is
 * folded into `onboardingStatuses`' own complement (every known value except
 * the excluded ones) — `notIn(X)` and `in(all-but-X)` are equivalent over a
 * closed, fixed set, and this keeps the field (and its URL param) singular
 * so the "Onboarding status" bar control can show/edit it as plain selected
 * values without a second, exclude-flavored param to collide with. Every
 * other field this function reads is `in`-only per that same op table, so
 * reading it op-blind carries no equivalent risk. Likewise `escalation`'s
 * value-less `isEmpty`/`isNotEmpty` map to the explicit tri-state
 * `hasEscalation` (`false`/`true`), never silently defaulted when absent
 * (`undefined`, i.e. not touched in `out`).
 *
 * Exported (not just used internally for `buildHref`) so
 * `DashboardWidgetPreviewPage.tsx` can seed a real, editable `CasesFilterBar`
 * from a case-family widget's own opaque filters, rather than only ever
 * building a one-shot click-through URL from them.
 */
export function translateCaseDashboardFilters(
  filters: Record<string, unknown>,
): Partial<CasesFilters> {
  const out: Partial<CasesFilters> = {};
  const fieldFilters = asCaseFieldFilters(filters.filters);

  // `state` in vs. notIn -> two distinct CasesFilters fields, same reasoning
  // as `tag` below: `state` is one of only 3 case-search fields whose
  // backend contract accepts `notIn` at all (`state`, `tag`,
  // `projectOnboardingStatus` — see `case_filters.go`'s per-field op table),
  // so a `notIn` widget filter must never be read op-blind and decoded as
  // an inclusion of the very states it meant to exclude.
  const states = caseFilterEntry(fieldFilters, "state", "in")?.values;
  if (states && states.length > 0) out.states = states as CasesFilters["states"];
  const excludeStates = caseFilterEntry(fieldFilters, "state", "notIn")?.values;
  if (excludeStates && excludeStates.length > 0) {
    out.excludeStates = excludeStates as CasesFilters["excludeStates"];
  }
  const severities = caseFilterValues(fieldFilters, "severity");
  if (severities && severities.length > 0) {
    out.severities = severities
      .map((s) => DASHBOARD_SEVERITY_TO_S_CODE[s])
      .filter((s): s is Severity => Boolean(s));
  }
  const types = caseFilterValues(fieldFilters, "type");
  if (types && types.length > 0) out.caseTypes = types as CasesFilters["caseTypes"];
  const productNames = caseFilterValues(fieldFilters, "product");
  if (productNames && productNames.length > 0) out.productNames = productNames;
  const assignedUserIds = caseFilterValues(fieldFilters, "assignedUserId");
  if (assignedUserIds && assignedUserIds.length > 0) out.assignees = ["@me"];
  const engagementTypes = caseFilterValues(fieldFilters, "engagementType");
  if (engagementTypes && engagementTypes.length > 0) {
    out.engagementTypes = engagementTypes as CasesFilters["engagementTypes"];
  }
  const workStates = caseFilterValues(fieldFilters, "workState");
  if (workStates && workStates.length > 0) {
    out.workStates = workStates as CasesFilters["workStates"];
  }

  const csTeams = caseFilterValues(fieldFilters, "creTeam");
  if (csTeams && csTeams.length > 0) out.csTeams = csTeams;
  const sreTeams = caseFilterValues(fieldFilters, "sreTeam");
  if (sreTeams && sreTeams.length > 0) out.sreTeams = sreTeams;

  // `tag` in vs. notIn -> two distinct CasesFilters fields, matched by
  // field+op together so one can never be mistaken for the other.
  const tags = caseFilterEntry(fieldFilters, "tag", "in")?.values;
  if (tags && tags.length > 0) out.tags = tags;
  const excludeTags = caseFilterEntry(fieldFilters, "tag", "notIn")?.values;
  if (excludeTags && excludeTags.length > 0) out.excludeTags = excludeTags;

  // `projectOnboardingStatus` in vs. notIn -> both fold into the single
  // `onboardingStatuses` field (unlike `state`/`tag`, which each keep a
  // separate exclude field) — its domain is the 4 fixed values in
  // `onboardingStatus.ts`, so `notIn(X)` decodes to `in(all-but-X)`, its
  // complement over that closed set, rather than a second field/URL param
  // that could collide with (or be conflated with) this one. This is the
  // field the click-through sign-flip bug was originally reported against:
  // a widget's `notIn` was silently decoding as an inclusion of exactly the
  // onboarding statuses it meant to exclude — reading it op-aware and
  // complementing rather than passing the excluded values straight through
  // is what keeps that bug from recurring.
  const includedOnboardingStatuses = caseFilterEntry(
    fieldFilters,
    "projectOnboardingStatus",
    "in",
  )?.values;
  const excludedOnboardingStatuses = caseFilterEntry(
    fieldFilters,
    "projectOnboardingStatus",
    "notIn",
  )?.values;
  let onboardingStatuses = includedOnboardingStatuses;
  let excludedAllStatuses = false;
  if (excludedOnboardingStatuses && excludedOnboardingStatuses.length > 0) {
    const complement: string[] = ALL_ONBOARDING_STATUSES.filter(
      (v) => !excludedOnboardingStatuses.includes(v),
    );
    onboardingStatuses = onboardingStatuses
      ? onboardingStatuses.filter((v) => complement.includes(v))
      : complement;
    excludedAllStatuses = onboardingStatuses.length === 0;
  }
  if (excludedAllStatuses) {
    // `notIn` excluded every known value (or intersected down to none with
    // an `in` list) -- the widget's own filter can never match any case.
    // Falling through to the `length > 0` check below would drop
    // `onboardingStatuses` entirely, which this app's convention reads as
    // "unfiltered" and would show every case instead of none — the exact
    // sign-flip bug this field exists to prevent. See `ONBOARDING_STATUS_NO_MATCH`.
    out.onboardingStatuses = [ONBOARDING_STATUS_NO_MATCH];
  } else if (onboardingStatuses && onboardingStatuses.length > 0) {
    out.onboardingStatuses = onboardingStatuses;
  }

  const slaGte = caseFilterEntry(fieldFilters, "taskSLABusinessElapsedPercent", "gte")
    ?.values?.[0];
  if (slaGte !== undefined) {
    const n = Number(slaGte);
    if (Number.isInteger(n) && n >= 0) out.slaElapsedPctGte = n;
  }
  const slaLte = caseFilterEntry(fieldFilters, "taskSLABusinessElapsedPercent", "lte")
    ?.values?.[0];
  if (slaLte !== undefined) {
    const n = Number(slaLte);
    if (Number.isInteger(n) && n >= 0) out.slaElapsedPctLte = n;
  }

  // Value-less predicate: presence of the entry (matched by op alone, no
  // `values` to read) is the whole filter -- must not be skipped for
  // "having nothing to read", the exact failure mode `writeWidgetPreviewHref`
  // shipped once for `isEmpty`/`isNotEmpty` entries generally.
  if (caseFilterEntry(fieldFilters, "escalation", "isNotEmpty")) out.hasEscalation = true;
  else if (caseFilterEntry(fieldFilters, "escalation", "isEmpty")) out.hasEscalation = false;

  const escalationLevels = caseFilterValues(fieldFilters, "escalationLevel");
  if (escalationLevels && escalationLevels.length > 0) {
    out.escalationLevels = escalationLevels;
  }

  const projectTypes = caseFilterValues(fieldFilters, "projectType");
  if (projectTypes && projectTypes.length > 0) out.projectTypes = projectTypes;

  const dateRangeFields: [string, keyof CasesFilters, keyof CasesFilters][] = [
    ["createdOn", "createdOnGte", "createdOnLte"],
    ["updatedOn", "updatedOnGte", "updatedOnLte"],
    ["closedOn", "closedOnGte", "closedOnLte"],
  ];
  for (const [beField, gteKey, lteKey] of dateRangeFields) {
    const gte = caseFilterEntry(fieldFilters, beField, "gte")?.values?.[0];
    if (gte !== undefined) (out as Record<string, unknown>)[gteKey] = gte;
    const lte = caseFilterEntry(fieldFilters, beField, "lte")?.values?.[0];
    if (lte !== undefined) (out as Record<string, unknown>)[lteKey] = lte;
  }

  return out;
}

// ---------------------------------------------------------------------------
// incident / change_request / problem — all live under /operations, switched
// by `?tab=`.
// ---------------------------------------------------------------------------

function operationsHref(tab: string, params?: URLSearchParams): string {
  const out = new URLSearchParams();
  out.set("tab", tab);
  params?.forEach((value, key) => out.set(key, value));
  return `/operations?${out.toString()}`;
}

/** `securityCenterHref`'s counterpart for the Security Center section's own
 * `?tab=` tab strip (see `CsmSecurityCenterPage`) — same shape as
 * `operationsHref`, just a different base path. */
function securityCenterHref(tab: string, params?: URLSearchParams): string {
  const out = new URLSearchParams();
  out.set("tab", tab);
  params?.forEach((value, key) => out.set(key, value));
  return `/security-center?${out.toString()}`;
}

/**
 * Builds a `basePath?...` href for a case-table resourceType whose own list
 * page is a real route (not a `?tab=`-switched section) but still reuses the
 * cases list's own `CasesFilters` URL scheme under the hood (`engagement`'s
 * `/engagements` — see `CsmEngagementsPage`, built on the shared
 * `CsmIssuesView` — reads/writes the identical query params `/cases` does,
 * via `readCasesFiltersFromUrl`/`writeCasesFiltersToUrl`). The destination
 * page locks its own `caseTypes` filter itself (`lockedFilters` in
 * `CsmIssuesView`), so `translateCaseDashboardFilters`'s own `caseTypes`
 * output — always just this one type — doesn't need to be (and isn't)
 * dropped here; it's simply redundant with what the page already locks.
 */
function caseTypeListHref(
  basePath: string,
  filters: Record<string, unknown>,
  displayName?: string,
): string {
  const full: CasesFilters = {
    ...DEFAULT_CASES_FILTERS,
    ...translateCaseDashboardFilters(filters),
  };
  const qs = writeCasesFiltersToUrl(full).toString();
  const href = qs ? `${basePath}?${qs}` : basePath;
  // `displayName`, when given, is appended as WIDGET_TITLE_PARAM so the
  // destination page (a real, permanent nav page with its own hardcoded
  // heading -- see CsmEngagementsPage.tsx) can show the originating
  // widget's own name instead (digiops-cs#2914) -- see
  // appendWidgetTitleParam's own doc comment for why this is a separate,
  // cosmetic-only param from every CasesFilters field this function
  // otherwise writes.
  return appendWidgetTitleParam(href, displayName);
}

/**
 * Shared `buildHref` wrapper for every case-family resourceType (`case`,
 * `service_request`, `security_report_analysis`, `engagement` —
 * `announcement` is excluded, see its own `buildHref` comment): a widget's
 * `anyOf` cross-field-OR branches have no representation in `CasesFilters`
 * (an AND-only model), so `fallback()` — which goes through
 * `translateCaseDashboardFilters` one way or another — would silently drop
 * them and land on a broader, unfiltered-by-`anyOf` list than what the tile
 * actually counted (the bug this wrapper exists to close). Route through the
 * generic, filter-faithful dashboard-widget preview page instead whenever
 * `anyOf` is present, exactly mirroring `incident_task`'s own fallback for a
 * resourceType with no representable destination of its own.
 */
function caseFamilyBuildHref(
  previewSlug: string,
  filters: Record<string, unknown>,
  ctx: { widgetId: string; displayName: string } | undefined,
  fallback: () => string,
): string {
  if (isAnyOfBranchArray(filters.anyOf)) {
    return buildWidgetPreviewHref({
      previewSlug,
      widgetId: ctx?.widgetId ?? "",
      displayName: ctx?.displayName ?? "",
      filters,
    });
  }
  return fallback();
}

/** Dashboard incident filters already use the real `BeIncidentPriority`
 * wire values (`CRITICAL`/`HIGH`/...), same as `IncidentFilters.priorities` —
 * no translation table needed, only a type narrowing. */
function translateIncidentDashboardFilters(
  filters: Record<string, unknown>,
): Partial<IncidentFilters> {
  const out: Partial<IncidentFilters> = {};
  const priorities = asStringArray(filters.priorities);
  if (priorities) out.priorities = priorities as IncidentFilters["priorities"];
  return out;
}

/** Dashboard CR filters already use the real `BeChangeRequestState`/`Impact`
 * wire values, same as `ChangeRequestFilters` — no translation needed. */
function translateChangeRequestDashboardFilters(
  filters: Record<string, unknown>,
): Partial<ChangeRequestFilters> {
  const out: Partial<ChangeRequestFilters> = {};
  const states = asStringArray(filters.states);
  if (states) out.states = states as ChangeRequestFilters["states"];
  const impacts = asStringArray(filters.impacts);
  if (impacts) out.impacts = impacts as ChangeRequestFilters["impacts"];
  return out;
}

// ---------------------------------------------------------------------------
// call_request — POST /call-requests/search, whose flat `filters` shape is
// its own thing (assignee/state/case-state/team, not the case-search DSL).
// ---------------------------------------------------------------------------

/**
 * Filter state for the call-requests "View more" landing page's own filter
 * bar. Mirrors `SearchAllCallRequestsPayload.filters` (the CSM backend's
 * `POST /call-requests/search` contract — confirmed directly against
 * `apps/csm-portal/backend/openapi.yaml`, not inferred): `assignedUserIds`
 * (the parent case's assigned user(s), platform UUIDs — this endpoint has no
 * `@me` sentinel of its own; any `__current_user__`/`@me` placeholder in a
 * widget's own filters is already resolved to a real id upstream, before
 * `translateCallRequestDashboardFilters` below ever sees it — see
 * `resolveCurrentUserSentinels` in `DashboardWidgetPreviewPage.tsx`),
 * `states` (call-request state keys — `ALL_CALL_REQUEST_STATES` in
 * `callRequestState.ts`), `caseStates`/`excludeCaseStates` (the parent
 * case's state, in vs. not-in — confirmed independent fields on this
 * contract, "either, both, or neither may be supplied", not one field with
 * an inferred op), and `assignmentTeamIds` (the parent case's assigned CRE
 * team, `creGroupId` values — confirmed CRE-only per digiops-cs#2732 "Calls
 * To Attend"; this contract has no SRE-team equivalent field). Scoped here
 * rather than exported alongside `CasesFilters` from a shared filter-bar
 * component: call requests have no other list page of their own for this
 * shape to be shared with.
 */
export interface CallRequestWidgetFilters {
  assignedUserIds: string[];
  states: BeCallRequestStateKey[];
  caseStates: CaseState[];
  excludeCaseStates: CaseState[];
  assignmentTeamIds: string[];
}

export const DEFAULT_CALL_REQUEST_WIDGET_FILTERS: CallRequestWidgetFilters = {
  assignedUserIds: [],
  states: [],
  caseStates: [],
  excludeCaseStates: [],
  assignmentTeamIds: [],
};

/**
 * Translate a call-request widget's opaque flat filters (the same shape its
 * own tile fetch already POSTs to `/call-requests/search` — see
 * `WIDGET_RESOURCE_CONFIG.call_request`, which has no
 * `buildSearchRequestBody` override, so `useWidgetData`'s default
 * `{filters, pagination, sortBy?}` passthrough already matches this
 * endpoint's own contract) into `CallRequestWidgetFilters`, the same "seed
 * once, then fully editable" input `translateCaseDashboardFilters` builds
 * for the case-family resourceTypes. No value remapping is needed here
 * (unlike that translator's severity/state-code tables): every field this
 * endpoint accepts already uses its own wire values, so this is a
 * structural narrow, not a translation.
 */
export function translateCallRequestDashboardFilters(
  filters: Record<string, unknown>,
): Partial<CallRequestWidgetFilters> {
  const out: Partial<CallRequestWidgetFilters> = {};
  const assignedUserIds = asStringArray(filters.assignedUserIds);
  if (assignedUserIds && assignedUserIds.length > 0) out.assignedUserIds = assignedUserIds;
  const states = asStringArray(filters.states);
  if (states && states.length > 0) out.states = states as BeCallRequestStateKey[];
  const caseStates = asStringArray(filters.caseStates);
  if (caseStates && caseStates.length > 0) out.caseStates = caseStates as CaseState[];
  const excludeCaseStates = asStringArray(filters.excludeCaseStates);
  if (excludeCaseStates && excludeCaseStates.length > 0) {
    out.excludeCaseStates = excludeCaseStates as CaseState[];
  }
  const assignmentTeamIds = asStringArray(filters.assignmentTeamIds);
  if (assignmentTeamIds && assignmentTeamIds.length > 0) {
    out.assignmentTeamIds = assignmentTeamIds;
  }
  return out;
}

/**
 * Inverse of `translateCallRequestDashboardFilters`: the filter bar's own
 * `CallRequestWidgetFilters` state back into the flat `filters` object
 * `useWidgetData` POSTs to `/call-requests/search`. Empty arrays are
 * omitted rather than sent as `field: []` — same convention every other
 * resourceType's own filter-building already follows (an empty array here
 * would ask the backend to match zero call requests, not "unfiltered",
 * which is the opposite of what an untouched control means).
 */
export function callRequestWidgetFiltersToQuery(
  filters: CallRequestWidgetFilters,
): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  if (filters.assignedUserIds.length > 0) out.assignedUserIds = filters.assignedUserIds;
  if (filters.states.length > 0) out.states = filters.states;
  if (filters.caseStates.length > 0) out.caseStates = filters.caseStates;
  if (filters.excludeCaseStates.length > 0) out.excludeCaseStates = filters.excludeCaseStates;
  if (filters.assignmentTeamIds.length > 0) out.assignmentTeamIds = filters.assignmentTeamIds;
  return out;
}

/** Shared "NUMBER — Subject" primary label, used by every resource whose
 * response item carries `number`/`subject` fields (case, incident,
 * change_request, problem). */
function numberSubjectLabel(item: WidgetItem): string {
  return (
    [asString(item.number), asString(item.subject)].filter(Boolean).join(" — ") ||
    "—"
  );
}

/** Shared case-detail row link, used by every resourceType whose rows are
 * case rows (`case` and the four case-`type` resourceTypes below, all of
 * which read from `/cases/search` and return the same row shape). Row
 * navigation in a `columns`-configured list goes through `detailHref` and
 * nothing else (see `GenericColumnList`), so a case-row resource without
 * this has rows that cannot be opened. */
function caseDetailHref(item: WidgetItem): string | undefined {
  const id = asString(item.id);
  return id ? `/cases/${id}` : undefined;
}

/** Shared humanized-`state` secondary label, used by every resource whose
 * response item carries a `state` field (case, change_request, problem —
 * NOT incident, which has no state field and uses `priority` instead). */
function stateSecondaryLabel(item: WidgetItem): string | undefined {
  const state = asString(item.state);
  return state ? humanizeState(state) : undefined;
}

/** `incident_task`-only secondary label: unlike `stateSecondaryLabel`, this
 * reads the data source's own pre-humanized `stateLabel` rather than trying
 * to humanize `state` itself — `state` is a raw integer specific to the
 * underlying data source's shared task table, with no stable domain enum to
 * translate through (see `BeIncidentTaskSearchView.state`'s doc comment). */
function incidentTaskStateSecondaryLabel(item: WidgetItem): string | undefined {
  return asString(item.stateLabel);
}

export const WIDGET_RESOURCE_CONFIG: Record<
  BeWidgetResourceType,
  WidgetResourceConfig
> = {
  case: {
    searchEndpoint: "/cases/search",
    groupByEndpoint: "/cases/aggregate",
    itemsKey: "cases",
    primaryLabel: numberSubjectLabel,
    secondaryLabel: stateSecondaryLabel,
    buildHref: (filters, ctx) =>
      caseFamilyBuildHref("cases", filters, ctx, () =>
        appendWidgetTitleParam(
          casesHref(translateCaseDashboardFilters(filters)),
          ctx?.displayName,
        ),
      ),
    icon: Briefcase,
    iconColor: "primary",
    previewSlug: "cases",
    detailHref: caseDetailHref,
  },
  // service_request / security_report_analysis / announcement / engagement:
  // additional values of the case-search "type" enum (see `BeCaseType` /
  // `ALL_CASE_TYPES` in `caseType.ts`), routed to the exact same `/cases/
  // search` endpoint and response rows as `case` above — the backend
  // auto-injects the implied `type` filter for each at dashboard-load time.
  // Rows are still case rows (same `BeCaseSearchView` shape), so these reuse
  // `case`'s own primaryLabel/secondaryLabel/list renderer verbatim; only the
  // icon/color/click-through destination differ per type, mirroring
  // `CASE_TYPE_COLOR`'s own per-type palette in `caseType.ts`. Same reasoning
  // extends `groupByEndpoint`: they share `/cases/aggregate` with `case` too
  // (the implied `type` filter is just another entry in the resolved
  // `filters` posted to that endpoint, same as it is for `/cases/search`).
  service_request: {
    searchEndpoint: "/cases/search",
    groupByEndpoint: "/cases/aggregate",
    itemsKey: "cases",
    detailHref: caseDetailHref,
    primaryLabel: numberSubjectLabel,
    secondaryLabel: stateSecondaryLabel,
    buildHref: (filters, ctx) =>
      caseFamilyBuildHref("service-requests", filters, ctx, () =>
        operationsHref(
          "service_requests",
          writeCasesFiltersToUrl({
            ...DEFAULT_CASES_FILTERS,
            ...translateCaseDashboardFilters(filters),
          }),
        ),
      ),
    icon: Cog,
    iconColor: "info",
    previewSlug: "service-requests",
  },
  security_report_analysis: {
    searchEndpoint: "/cases/search",
    groupByEndpoint: "/cases/aggregate",
    itemsKey: "cases",
    detailHref: caseDetailHref,
    primaryLabel: numberSubjectLabel,
    secondaryLabel: stateSecondaryLabel,
    buildHref: (filters, ctx) =>
      caseFamilyBuildHref("security-reports", filters, ctx, () =>
        securityCenterHref(
          "security_reports",
          writeCasesFiltersToUrl({
            ...DEFAULT_CASES_FILTERS,
            ...translateCaseDashboardFilters(filters),
          }),
        ),
      ),
    icon: Shield,
    iconColor: "warning",
    previewSlug: "security-reports",
  },
  announcement: {
    searchEndpoint: "/cases/search",
    groupByEndpoint: "/cases/aggregate",
    itemsKey: "cases",
    detailHref: caseDetailHref,
    primaryLabel: numberSubjectLabel,
    secondaryLabel: stateSecondaryLabel,
    // CsmAnnouncementsPage keeps its own filters in local component state,
    // not the URL (unlike /cases, /operations, /engagements) — there is no
    // query-param scheme to land a filtered click-through on for the plain
    // case, so the fallback stays the unfiltered list, same as `problem`
    // above. An `anyOf` filter still routes through `caseFamilyBuildHref` to
    // the generic dashboard-widget preview page, though: that page resolves
    // `resourceType` from `previewSlug` generically and posts raw filters
    // straight to the search endpoint, so it needs no announcement-specific
    // filtered route of its own to render this widget's exact result set.
    buildHref: (filters, ctx) => caseFamilyBuildHref("announcements", filters, ctx, () => "/announcements"),
    icon: Megaphone,
    iconColor: "success",
    previewSlug: "announcements",
  },
  engagement: {
    searchEndpoint: "/cases/search",
    groupByEndpoint: "/cases/aggregate",
    itemsKey: "cases",
    detailHref: caseDetailHref,
    primaryLabel: numberSubjectLabel,
    secondaryLabel: stateSecondaryLabel,
    buildHref: (filters, ctx) =>
      caseFamilyBuildHref("engagements", filters, ctx, () =>
        caseTypeListHref("/engagements", filters, ctx?.displayName),
      ),
    icon: Handshake,
    iconColor: "secondary",
    previewSlug: "engagements",
  },
  incident: {
    searchEndpoint: "/incidents/search",
    groupByEndpoint: "/incidents/aggregate",
    itemsKey: "incidents",
    primaryLabel: numberSubjectLabel,
    secondaryLabel: (item) => asString(item.priority),
    buildHref: (filters) =>
      operationsHref(
        "incidents",
        writeIncidentFiltersToUrl({
          ...DEFAULT_INCIDENT_FILTERS,
          ...translateIncidentDashboardFilters(filters),
        }),
      ),
    icon: AlertTriangle,
    iconColor: "warning",
    previewSlug: "incidents",
    detailHref: (item) =>
      asString(item.id) ? `/operations/incidents/${asString(item.id)}` : undefined,
  },
  change_request: {
    searchEndpoint: "/change-requests/search",
    groupByEndpoint: "/change-requests/aggregate",
    itemsKey: "changeRequests",
    primaryLabel: numberSubjectLabel,
    secondaryLabel: stateSecondaryLabel,
    buildHref: (filters) =>
      operationsHref(
        "change_requests",
        writeChangeRequestFiltersToUrl({
          ...DEFAULT_CR_FILTERS,
          ...translateChangeRequestDashboardFilters(filters),
        }),
      ),
    icon: GitPullRequest,
    iconColor: "info",
    previewSlug: "change-requests",
    detailHref: (item) =>
      asString(item.id) ? `/operations/change-requests/${asString(item.id)}` : undefined,
  },
  problem: {
    searchEndpoint: "/problems/search",
    groupByEndpoint: "/problems/aggregate",
    itemsKey: "problems",
    primaryLabel: numberSubjectLabel,
    secondaryLabel: stateSecondaryLabel,
    // No dashboard widget filters problems today; the tab has no URL filter
    // scheme of its own yet either, so this is unfiltered.
    buildHref: () => operationsHref("problems"),
    icon: AlertOctagon,
    iconColor: "error",
    previewSlug: "problems",
    detailHref: (item) =>
      asString(item.id) ? `/operations/problems/${asString(item.id)}` : undefined,
  },
  // No standalone incident-task list page exists in this app (confirmed:
  // incident tasks are only ever viewed as part of their parent incident),
  // so unlike every other resourceType here, `buildHref` can't land on a
  // real list route of its own -- routing it to the plain incidents list
  // (`/operations?tab=incidents`) would silently drop this widget's own
  // filters and show an unrelated, unfiltered set of records. Route through
  // the generic dashboard-widget preview page instead (via `previewSlug`
  // below), which is filter-aware for every resourceType already.
  // `detailHref` still lands on the owning incident's real detail page, the
  // same fallback `call_request` uses for landing on its owning case.
  incident_task: {
    searchEndpoint: "/incident-tasks/search",
    groupByEndpoint: "/incident-tasks/aggregate",
    itemsKey: "incidentTasks",
    primaryLabel: numberSubjectLabel,
    secondaryLabel: incidentTaskStateSecondaryLabel,
    // `ctx` is always passed by the real caller (`DashboardWidgetTile`); the
    // fallback below only covers a caller that omits it (e.g. a test
    // exercising this config directly), landing on the same "open this page
    // from a dashboard widget" prompt `DashboardWidgetPreviewPage` already
    // shows for any preview link missing its widget id.
    buildHref: (filters, ctx) =>
      buildWidgetPreviewHref({
        previewSlug: "incident-tasks",
        widgetId: ctx?.widgetId ?? "",
        displayName: ctx?.displayName ?? "",
        filters,
      }),
    icon: CheckSquare,
    iconColor: "warning",
    previewSlug: "incident-tasks",
    detailHref: (item) => {
      const incidentId = nestedID(item.incident);
      return incidentId ? `/operations/incidents/${incidentId}` : undefined;
    },
  },
  account: {
    searchEndpoint: "/accounts/search",
    itemsKey: "accounts",
    primaryLabel: (item) => asString(item.name) ?? "—",
    secondaryLabel: (item) => asString(item.tier),
    buildHref: () => "/customers/accounts",
    icon: Building2,
    iconColor: "secondary",
    previewSlug: "accounts",
    detailHref: (item) =>
      asString(item.id) ? `/customers/accounts/${asString(item.id)}` : undefined,
  },
  project: {
    searchEndpoint: "/projects/search",
    itemsKey: "projects",
    primaryLabel: (item) => asString(item.name) ?? asString(item.projectKey) ?? "—",
    secondaryLabel: (item) => asString(item.subscriptionType),
    buildHref: () => "/customers/projects",
    icon: FolderKanban,
    iconColor: "secondary",
    previewSlug: "projects",
    detailHref: (item) =>
      asString(item.id) ? `/customers/projects/${asString(item.id)}` : undefined,
  },
  user: {
    searchEndpoint: "/users/search",
    itemsKey: "users",
    primaryLabel: (item) => {
      const first = asString(item.firstName);
      const last = asString(item.lastName);
      const full = [first, last].filter(Boolean).join(" ");
      return full || asString(item.userName) || asString(item.email) || "—";
    },
    secondaryLabel: (item) => asString(item.email),
    buildHref: () => "/admin/users",
    icon: Users,
    iconColor: "info",
    previewSlug: "users",
    detailHref: (item) =>
      asString(item.id) ? `/people/${encodeURIComponent(asString(item.id) ?? "")}` : undefined,
  },
  time_card: {
    searchEndpoint: "/time-cards/search",
    itemsKey: "timeCards",
    primaryLabel: (item) => {
      const caseNumber = nestedNumber(item.case);
      const workDate = asString(item.workDate);
      return [caseNumber, workDate].filter(Boolean).join(" — ") || "—";
    },
    secondaryLabel: (item) => {
      const state = asString(item.state);
      return state ? humanizeState(state) : undefined;
    },
    buildHref: () => "/time-cards",
    icon: Clock,
    iconColor: "warning",
    previewSlug: "time-cards",
    // TimeCardsTable opens a details dialog in place rather than navigating
    // — no standalone route for the generic renderer to link to either.
    detailHref: () => undefined,
  },
  product_vulnerability: {
    searchEndpoint: "/products/vulnerabilities/search",
    itemsKey: "productVulnerabilities",
    primaryLabel: (item) =>
      asString(item.cveId) ?? asString(item.vulnerabilityId) ?? "—",
    secondaryLabel: (item) =>
      asString(item.priority) ?? asString(item.productName),
    // The tab default is "security_reports" (see csmNavItems.ts), so the
    // vulnerabilities tab needs its own `?tab=` — omitting it, as this did
    // before, silently lands the click on the wrong tab.
    buildHref: () => "/security-center?tab=vulnerabilities",
    icon: ShieldAlert,
    iconColor: "error",
    previewSlug: "vulnerabilities",
    detailHref: (item) =>
      asString(item.id)
        ? `/security-center/vulnerabilities/${encodeURIComponent(asString(item.id) ?? "")}`
        : undefined,
  },
  task: {
    searchEndpoint: "/tasks/search",
    itemsKey: "tasks",
    primaryLabel: (item) => asString(item.subject) ?? "—",
    secondaryLabel: (item) => {
      const state = asString(item.state);
      return state ? taskStateLabel(state as BeTaskState) : undefined;
    },
    // Tasks have no standalone list page today (they're only ever shown
    // inside a case's own Tasks tab) -- clicking a task widget's tile stays
    // on the dashboard rather than 404ing. Revisit once/if a dedicated tasks
    // list page exists.
    buildHref: () => "/dashboard",
    icon: ListChecks,
    iconColor: "warning",
    previewSlug: "tasks",
    // Same reasoning as buildHref above: no standalone detail route exists.
    // The hardcoded TaskWidgetList opens TaskDetailDialog instead, which the
    // generic column renderer doesn't replicate — a columns-configured task
    // widget's rows render inert rather than opening that dialog.
    detailHref: () => undefined,
  },
  call_request: {
    searchEndpoint: "/call-requests/search",
    itemsKey: "callRequests",
    primaryLabel: (item) => {
      const number = asString(item.number);
      const reason = asString(item.reason);
      return [number, reason].filter(Boolean).join(" — ") || "—";
    },
    secondaryLabel: (item) => {
      const state = item.state as { label?: string } | undefined;
      return state?.label;
    },
    // No widget filters a call request by anything the cases list can render as a
    // filtered view (state keys differ entirely from case state), so the tile-level
    // "view all" click has nowhere sensible to land other than the dashboard itself
    // -- unlike a per-row click, which goes straight to the owning case (see the
    // list renderer in widgetListConfig.tsx, not this file).
    buildHref: () => "/dashboard",
    icon: Clock,
    iconColor: "info",
    previewSlug: "call-requests",
    // Same as the hardcoded CallRequestWidgetList: a call request has no
    // standalone detail page of its own, so rows link to the owning case.
    detailHref: (item) => {
      const caseId = nestedID(item.case);
      return caseId ? `/cases/${caseId}` : undefined;
    },
  },
  // Satisfaction-rating survey responses across cases (`POST
  // /cases/feedback/search` / `POST /cases/feedback/aggregate`) — its own
  // resourceType, not a case `type` variant like `service_request` etc.
  // above, because a feedback record isn't a case row at all (no
  // `number`/`subject`/`state`; see `BeCaseFeedback`). Its own two-endpoint
  // request/response contract diverges from every other resourceType's
  // shared `{pagination:{offset,limit}} -> {total, itemsKey}` shape
  // (`page`/`pageSize` -> `totalRecords`/`results`, no `total` field at
  // all), which is exactly what `buildSearchRequestBody`/
  // `parseSearchResponse` exist to adapt — see those fields' own doc
  // comments on `WidgetResourceConfig`. There is no standalone list page for
  // feedback records (only the dashboard's own list-shape widget), so
  // `buildHref`/`previewSlug` have nowhere real to land — same situation
  // `task` is in above, same fallback.
  case_feedback: {
    searchEndpoint: "/cases/feedback/search",
    groupByEndpoint: "/cases/feedback/aggregate",
    itemsKey: "results",
    buildSearchRequestBody: ({ filters, offset, limit }) => {
      // `case_feedback` has no sortable columns exposed by its own search
      // contract (no `sortBy` field on `CaseFeedbackSearchPayload`) — a
      // `sortBy` passed down from a `shape: "list"` widget config is
      // silently dropped rather than sent, the same "config is responsible
      // for a field valid for that resourceType's own contract" convention
      // `useWidgetData`'s own `sortBy` doc comment already documents for
      // every other resourceType (an invalid field there gets rejected by
      // the search endpoint itself; here there's no such endpoint-side
      // rejection to fall back on, since the field would just be ignored by
      // this request body entirely).
      //
      // `page` is 1-based (see openapi.yaml's CaseFeedbackSearchPayload) —
      // `offset`/`limit` (0-based, from `useWidgetData`) convert via
      // `Math.floor(offset / limit) + 1`, matching only the offsets
      // `useWidgetData` itself ever actually requests (page-aligned:
      // `offset` is always a multiple of `limit`, either 0 for a tile or
      // `listLimit * pageIndex` for the preview page's own pager).
      const page = limit > 0 ? Math.floor(offset / limit) + 1 : 1;
      // The dashboard-widget preview page's own URL round-trip
      // (`parseWidgetPreviewFilters`) decodes every query param as a
      // comma-split string array — the shape every other resourceType's own
      // filters actually use (case-search-DSL field values). `case_feedback`
      // is one of the few resourceTypes whose filters are a flat scalar
      // object instead (`dateFrom`/`dateTo`/`caseId`/`rating`, not arrays),
      // so a preview-page click-through (e.g. a trend-bar bucket's date
      // range, or the rating pie's `rating` slice) arrives here as
      // `{dateFrom: ["2026-07-01"], rating: ["5"]}` rather than the scalar
      // values this endpoint's own contract expects — sent as-is, the
      // backing data source rejects the array shape outright. Unwrap known
      // scalar fields back to their real type before forwarding; a
      // tile-level fetch (whose filters
      // never went through that round-trip) already has scalars here and is
      // a no-op through this same unwrap.
      const scalarFilters = { ...filters };
      for (const key of ["caseId", "dateFrom", "dateTo"] as const) {
        const v = scalarFilters[key];
        if (Array.isArray(v)) scalarFilters[key] = v[0];
      }
      if (Array.isArray(scalarFilters.rating)) {
        const parsed = Number(scalarFilters.rating[0]);
        scalarFilters.rating = Number.isNaN(parsed) ? undefined : parsed;
      }
      return { filters: scalarFilters, page, pageSize: limit };
    },
    parseSearchResponse: (res) => {
      const total = typeof res.totalRecords === "number" ? res.totalRecords : 0;
      const rawItems = res.results;
      const items = Array.isArray(rawItems) ? (rawItems as Record<string, unknown>[]) : [];
      return { total, items };
    },
    primaryLabel: (item) => {
      const ratingLabel = asString(item.ratingLabel);
      const submittedAt = asString(item.submittedAt);
      return [ratingLabel, submittedAt].filter(Boolean).join(" — ") || "—";
    },
    secondaryLabel: (item) => asString(item.comment) ?? undefined,
    // No standalone case-feedback list page exists (only this dashboard's
    // own widgets), so — like `incident_task` above — every click routes
    // through the generic dashboard-widget preview page, the only
    // destination that can render this resourceType's own filtered result
    // set.
    buildHref: (filters, ctx) =>
      buildWidgetPreviewHref({
        previewSlug: "case-feedback",
        widgetId: ctx?.widgetId ?? "",
        displayName: ctx?.displayName ?? "",
        filters,
      }),
    icon: Star,
    iconColor: "warning",
    previewSlug: "case-feedback",
    // The one link this resourceType's rows DO have: back to the owning
    // case. `caseId` is a platform UUID already (see `BeCaseFeedback`), not
    // a nested reference to resolve via `nestedID` the way `call_request`'s
    // `item.case` is.
    detailHref: (item) => {
      const caseId = asString(item.caseId);
      return caseId ? `/cases/${caseId}` : undefined;
    },
  },
};

/** Reverse lookup of `previewSlug` back to its `resourceType`, for the
 * dashboard widget "View more" preview page's own `/dashboard/:previewSlug`
 * route — the only place a URL segment needs mapping back to a resourceType. */
export function resourceTypeForPreviewSlug(
  slug: string | undefined,
): BeWidgetResourceType | undefined {
  const entry = (Object.entries(WIDGET_RESOURCE_CONFIG) as [BeWidgetResourceType, WidgetResourceConfig][]).find(
    ([, config]) => config.previewSlug === slug,
  );
  return entry?.[0];
}

function nestedNumber(v: unknown): string | undefined {
  if (v && typeof v === "object" && "number" in v) {
    return asString((v as { number?: unknown }).number);
  }
  return undefined;
}

/** Reads `id` off a nested entity reference (e.g. `CallRequestView.case`),
 * mirroring `nestedNumber` above. */
function nestedID(v: unknown): string | undefined {
  if (v && typeof v === "object" && "id" in v) {
    return asString((v as { id?: unknown }).id);
  }
  return undefined;
}
