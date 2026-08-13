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
  Clock,
  Cog,
  FolderKanban,
  GitPullRequest,
  Handshake,
  Megaphone,
  Shield,
  ShieldAlert,
  Users,
  ListChecks,
  type LucideIcon,
} from "@wso2/oxygen-ui-icons-react";
import type { BeWidgetResourceType } from "@api/backend/types";
import { humanizeState } from "@features/csm-dashboard/utils/abtDashboard";
import {
  casesHref,
  DEFAULT_CASES_FILTERS,
  writeCasesFiltersToUrl,
} from "@features/csm-cases/utils/casesFiltersUrl";
import type { CasesFilters } from "@features/csm-cases/components/CasesFilterBar";
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
  /** Key the response's item array is nested under. */
  itemsKey: string;
  /** Primary (bold) line for one list-shape row. */
  primaryLabel: (item: WidgetItem) => string;
  /** Optional secondary (muted) line for one list-shape row. */
  secondaryLabel?: (item: WidgetItem) => string | undefined;
  /** Where a click on this widget's tile navigates, given its (opaque,
   * already current-user-resolved) filters. */
  buildHref: (filters: Record<string, unknown>) => string;
  /** Icon shown on the tile, one per resource type (not per individual
   * widget — the backend registry doesn't carry per-widget icon metadata). */
  icon: LucideIcon;
  /** Theme palette key the icon (and nothing else — see DashboardWidgetTile's
   * hover treatment) is colored with. */
  iconColor: "primary" | "secondary" | "success" | "error" | "info" | "warning";
  /** Friendly plural URL segment for this resource type's dashboard-widget
   * "View more" preview page, e.g. `/dashboard/preview/cases`. Distinct from
   * that resource's own tab path (`buildHref`'s destination) — this route is
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
 * `escalation`, `integrationCsTeam`, `tag`, `projectOnboardingStatus`,
 * `projectType`, and the `createdOn`/`updatedOn`/`closedOn` date ranges,
 * which was the root cause of the click-through data-loss bug this function
 * exists to fix (a tile reading a filtered count landed on the org-wide
 * cases list because its filters had nowhere to go). Only `anyOf`,
 * `parentId`, and `resolutionNotes` remain genuinely dropped: no dashboard
 * widget uses them today, and `CasesFilters` has no equivalent to invent one
 * for without guessing at a UI treatment.
 *
 * `assignedUserId` carries the current user's own UUID (every widget that
 * sets it does so via the current-user placeholder), and
 * `CasesFilters.assignees` is email/`@me`-based with no UUID lookup
 * available here — since these widgets only ever filter "assigned to me",
 * any non-empty `assignedUserId` maps to the `@me` sentinel rather than an
 * (unresolvable) literal UUID.
 *
 * `tag`'s two ops (`in`/`notIn`) map to the two distinct `CasesFilters`
 * fields `tags`/`excludeTags` — never a single field plus an inferred op —
 * so a `notIn` widget filter can never decode as `tags` (an inclusion) the
 * way the equivalent bug shipped once in the dashboard preview URL (see
 * `casesFiltersUrl.ts`'s `writeCasesFiltersToUrl` doc comment for the full
 * story). Likewise `escalation`'s value-less `isEmpty`/`isNotEmpty` map to
 * the explicit tri-state `hasEscalation` (`false`/`true`), never silently
 * defaulted when absent (`undefined`, i.e. not touched in `out`).
 */
function translateCaseDashboardFilters(
  filters: Record<string, unknown>,
): Partial<CasesFilters> {
  const out: Partial<CasesFilters> = {};
  const fieldFilters = asCaseFieldFilters(filters.filters);

  const states = caseFilterValues(fieldFilters, "state");
  if (states && states.length > 0) out.states = states as CasesFilters["states"];
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

  const csTeams = caseFilterValues(fieldFilters, "integrationCsTeam");
  if (csTeams && csTeams.length > 0) out.csTeams = csTeams;

  // `tag` in vs. notIn -> two distinct CasesFilters fields, matched by
  // field+op together so one can never be mistaken for the other.
  const tags = caseFilterEntry(fieldFilters, "tag", "in")?.values;
  if (tags && tags.length > 0) out.tags = tags;
  const excludeTags = caseFilterEntry(fieldFilters, "tag", "notIn")?.values;
  if (excludeTags && excludeTags.length > 0) out.excludeTags = excludeTags;

  const onboardingStatuses = caseFilterValues(fieldFilters, "projectOnboardingStatus");
  if (onboardingStatuses && onboardingStatuses.length > 0) {
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
function caseTypeListHref(basePath: string, filters: Record<string, unknown>): string {
  const full: CasesFilters = {
    ...DEFAULT_CASES_FILTERS,
    ...translateCaseDashboardFilters(filters),
  };
  const qs = writeCasesFiltersToUrl(full).toString();
  return qs ? `${basePath}?${qs}` : basePath;
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

export const WIDGET_RESOURCE_CONFIG: Record<
  BeWidgetResourceType,
  WidgetResourceConfig
> = {
  case: {
    searchEndpoint: "/cases/search",
    itemsKey: "cases",
    primaryLabel: numberSubjectLabel,
    secondaryLabel: stateSecondaryLabel,
    buildHref: (filters) => casesHref(translateCaseDashboardFilters(filters)),
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
  // `CASE_TYPE_COLOR`'s own per-type palette in `caseType.ts`.
  service_request: {
    searchEndpoint: "/cases/search",
    itemsKey: "cases",
    detailHref: caseDetailHref,
    primaryLabel: numberSubjectLabel,
    secondaryLabel: stateSecondaryLabel,
    buildHref: (filters) =>
      operationsHref(
        "service_requests",
        writeCasesFiltersToUrl({
          ...DEFAULT_CASES_FILTERS,
          ...translateCaseDashboardFilters(filters),
        }),
      ),
    icon: Cog,
    iconColor: "info",
    previewSlug: "service-requests",
  },
  security_report_analysis: {
    searchEndpoint: "/cases/search",
    itemsKey: "cases",
    detailHref: caseDetailHref,
    primaryLabel: numberSubjectLabel,
    secondaryLabel: stateSecondaryLabel,
    buildHref: (filters) =>
      securityCenterHref(
        "security_reports",
        writeCasesFiltersToUrl({
          ...DEFAULT_CASES_FILTERS,
          ...translateCaseDashboardFilters(filters),
        }),
      ),
    icon: Shield,
    iconColor: "warning",
    previewSlug: "security-reports",
  },
  announcement: {
    searchEndpoint: "/cases/search",
    itemsKey: "cases",
    detailHref: caseDetailHref,
    primaryLabel: numberSubjectLabel,
    secondaryLabel: stateSecondaryLabel,
    // CsmAnnouncementsPage keeps its own filters in local component state,
    // not the URL (unlike /cases, /operations, /engagements) — there is no
    // query-param scheme to land a filtered click-through on yet, so this
    // stays unfiltered, same as `problem` above.
    buildHref: () => "/announcements",
    icon: Megaphone,
    iconColor: "success",
    previewSlug: "announcements",
  },
  engagement: {
    searchEndpoint: "/cases/search",
    itemsKey: "cases",
    detailHref: caseDetailHref,
    primaryLabel: numberSubjectLabel,
    secondaryLabel: stateSecondaryLabel,
    buildHref: (filters) => caseTypeListHref("/engagements", filters),
    icon: Handshake,
    iconColor: "secondary",
    previewSlug: "engagements",
  },
  incident: {
    searchEndpoint: "/incidents/search",
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
};

/** Reverse lookup of `previewSlug` back to its `resourceType`, for the
 * dashboard widget "View more" preview page's own
 * `/dashboard/preview/:previewSlug` route — the only place a URL segment
 * needs mapping back to a resourceType. */
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
