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
 * Regression coverage for the empty-array filter guard: a DSL entry with
 * `values: []` must leave the corresponding `CasesFilters` field unset
 * rather than setting an explicit empty filter (which the cases list would
 * treat as "match nothing" instead of "no constraint") — see the
 * CodeRabbit finding this closes.
 */

import { describe, expect, it } from "vitest";
import {
  callRequestWidgetFiltersToQuery,
  DEFAULT_CALL_REQUEST_WIDGET_FILTERS,
  translateCallRequestDashboardFilters,
  WIDGET_RESOURCE_CONFIG,
} from "@features/csm-dashboard/config/widgetResourceConfig";
import { readCasesFiltersFromUrl } from "@features/csm-cases/utils/casesFiltersUrl";
import { WIDGET_TITLE_PARAM } from "@features/csm-dashboard/utils/widgetPreviewUrl";

function hrefParams(href: string): URLSearchParams {
  const [, qs] = href.split("?");
  return new URLSearchParams(qs ?? "");
}

describe("WIDGET_RESOURCE_CONFIG.case.buildHref", () => {
  it("omits states/severities/types/products from the href when the DSL entry's values are empty", () => {
    const href = WIDGET_RESOURCE_CONFIG.case.buildHref({
      filters: [
        { field: "state", op: "in", values: [] },
        { field: "severity", op: "in", values: [] },
        { field: "type", op: "in", values: [] },
        { field: "product", op: "in", values: [] },
      ],
    });

    const params = hrefParams(href);
    expect(params.has("states")).toBe(false);
    expect(params.has("severities")).toBe(false);
    expect(params.has("types")).toBe(false);
    expect(params.has("products")).toBe(false);
  });

  it("still sets each field when the DSL entry carries real values", () => {
    const href = WIDGET_RESOURCE_CONFIG.case.buildHref({
      filters: [
        { field: "state", op: "in", values: ["open"] },
        { field: "severity", op: "in", values: ["critical"] },
        { field: "type", op: "in", values: ["case"] },
        { field: "product", op: "in", values: ["API Manager"] },
      ],
    });

    const params = hrefParams(href);
    expect(params.get("states")).toBe("open");
    expect(params.get("types")).toBe("case");
    expect(params.get("products")).toBe("API Manager");
    // Severity is remapped from the dashboard label to the case-list's own
    // S-code, so just assert it was set at all (severity-mapping specifics
    // aren't this fix's concern).
    expect(params.has("severities")).toBe(true);
  });

  it("carries engagementType and workState through to the cases list (previously dropped)", () => {
    // Regression: a case widget filtering by engagementType (e.g. "Engagements
    // In Progress") clicked through to an unfiltered cases list, because this
    // mapping didn't exist at all -- not a translation bug, a missing one.
    const href = WIDGET_RESOURCE_CONFIG.case.buildHref({
      filters: [
        { field: "engagementType", op: "in", values: ["migration", "onboarding"] },
        { field: "workState", op: "in", values: ["paused"] },
      ],
    });

    const params = hrefParams(href);
    expect(params.get("engagementTypes")).toBe("migration,onboarding");
    expect(params.get("workStates")).toBe("paused");
  });

  it("omits engagementTypes/workStates when the DSL entry's values are empty", () => {
    const href = WIDGET_RESOURCE_CONFIG.case.buildHref({
      filters: [
        { field: "engagementType", op: "in", values: [] },
        { field: "workState", op: "in", values: [] },
      ],
    });

    const params = hrefParams(href);
    expect(params.has("engagementTypes")).toBe(false);
    expect(params.has("workStates")).toBe(false);
  });
});

/**
 * Regression: the motivating bug for this whole feature. A widget filtering
 * `creTeam in [<team>]` + `tag notIn [s_dip]` + `state in [...]`
 * clicked through to `/cases?states=...` with the team and tag conditions
 * silently dropped — a tile reading 2 landed on a list of 30 (the org-wide
 * figure). Confirmed live three times before this fix. This suite proves the
 * full round trip end to end: `translateCaseDashboardFilters` ->
 * `casesHref` -> `readCasesFiltersFromUrl` — not just that the href contains
 * the right substring.
 */
describe("WIDGET_RESOURCE_CONFIG.case — previously-dropped fields", () => {
  it("carries creTeam, tag notIn, projectOnboardingStatus, escalation, escalationLevel, projectType, and SLA%/date ranges through to the href", () => {
    const href = WIDGET_RESOURCE_CONFIG.case.buildHref({
      filters: [
        { field: "creTeam", op: "in", values: ["team-abt"] },
        { field: "tag", op: "notIn", values: ["s_dip"] },
        { field: "projectOnboardingStatus", op: "in", values: ["in_progress"] },
        { field: "escalation", op: "isNotEmpty" },
        { field: "escalationLevel", op: "in", values: ["L1"] },
        { field: "projectType", op: "in", values: ["enterprise"] },
        { field: "taskSLABusinessElapsedPercent", op: "gte", values: ["80"] },
        { field: "createdOn", op: "gte", values: ["2026-01-01"] },
      ],
    });
    const parsed = readCasesFiltersFromUrl(hrefParams(href));

    expect(parsed.csTeams).toEqual(["team-abt"]);
    expect(parsed.excludeTags).toEqual(["s_dip"]);
    expect(parsed.tags).toEqual([]); // must NOT be inverted into an inclusion
    expect(parsed.onboardingStatuses).toEqual(["in_progress"]);
    expect(parsed.hasEscalation).toBe(true);
    expect(parsed.escalationLevels).toEqual(["L1"]);
    expect(parsed.projectTypes).toEqual(["enterprise"]);
    expect(parsed.slaElapsedPctGte).toBe(80);
    expect(parsed.createdOnGte).toBe("2026-01-01");
  });

  it("carries sreTeam through to the href the same way creTeam does (CodeRabbit #3801153841/#3801153843)", () => {
    const href = WIDGET_RESOURCE_CONFIG.case.buildHref({
      filters: [{ field: "sreTeam", op: "in", values: ["team-sre-abt"] }],
    });
    const parsed = readCasesFiltersFromUrl(hrefParams(href));

    expect(parsed.sreTeams).toEqual(["team-sre-abt"]);
  });

  it("creTeam and sreTeam survive together on the same widget, independently", () => {
    const href = WIDGET_RESOURCE_CONFIG.case.buildHref({
      filters: [
        { field: "creTeam", op: "in", values: ["team-abt"] },
        { field: "sreTeam", op: "in", values: ["team-sre-abt"] },
      ],
    });
    const parsed = readCasesFiltersFromUrl(hrefParams(href));

    expect(parsed.csTeams).toEqual(["team-abt"]);
    expect(parsed.sreTeams).toEqual(["team-sre-abt"]);
  });

  it("the org-wide-figure regression: team + tag-exclusion + state survive together, unchanged, end to end", () => {
    const href = WIDGET_RESOURCE_CONFIG.case.buildHref({
      filters: [
        { field: "creTeam", op: "in", values: ["team-abt"] },
        { field: "tag", op: "notIn", values: ["s_dip"] },
        { field: "state", op: "in", values: ["open", "work_in_progress"] },
      ],
    });
    const parsed = readCasesFiltersFromUrl(hrefParams(href));

    expect(parsed.csTeams).toEqual(["team-abt"]);
    expect(parsed.excludeTags).toEqual(["s_dip"]);
    expect(parsed.states).toEqual(["open", "work_in_progress"]);
  });

  // Regression: reported live against `abt_overall_open_incident`'s
  // `projectOnboardingStatus notIn ["In-Progress"]` -- the click-through
  // landed on the Cases list with an "Onboarding: In-progress" INCLUDE chip
  // (the exact opposite of the widget's own filter), because the values were
  // read op-blind and dumped into the same field an `in` filter would use.
  // `state` has the identical backend-supported notIn and was audited to
  // have the same latent bug, fixed alongside onboarding status.
  //
  // Unlike `state`/`tag`, `projectOnboardingStatus` doesn't get its own
  // second `exclude...` field: its domain is the 4 fixed values in
  // `onboardingStatus.ts`, so `notIn(X)` decodes to `onboardingStatuses`
  // holding `in`(every other known value) — the complement — rather than a
  // separate field that could collide with (or be conflated with) the plain
  // `onboardingStatuses` one the bar's own control edits.
  it("projectOnboardingStatus notIn decodes to onboardingStatuses' complement, never an inclusion of the excluded value", () => {
    const href = WIDGET_RESOURCE_CONFIG.case.buildHref({
      filters: [
        { field: "projectOnboardingStatus", op: "notIn", values: ["In-Progress"] },
      ],
    });
    const parsed = readCasesFiltersFromUrl(hrefParams(href));

    expect(parsed.onboardingStatuses.sort()).toEqual(
      ["Not-Started", "Completed", "Not-Applicable"].sort(),
    );
  });

  it("state notIn decodes to excludeStates, never states", () => {
    const href = WIDGET_RESOURCE_CONFIG.case.buildHref({
      filters: [{ field: "state", op: "notIn", values: ["closed"] }],
    });
    const parsed = readCasesFiltersFromUrl(hrefParams(href));

    expect(parsed.excludeStates).toEqual(["closed"]);
    expect(parsed.states).toEqual([]);
  });

  it("state in and state notIn survive together, independently, on the same widget", () => {
    const href = WIDGET_RESOURCE_CONFIG.case.buildHref({
      filters: [
        { field: "state", op: "in", values: ["open", "work_in_progress"] },
        { field: "state", op: "notIn", values: ["closed"] },
      ],
    });
    const parsed = readCasesFiltersFromUrl(hrefParams(href));

    expect(parsed.states).toEqual(["open", "work_in_progress"]);
    expect(parsed.excludeStates).toEqual(["closed"]);
  });

  it("projectOnboardingStatus in and notIn on the same widget intersect into onboardingStatuses' complement", () => {
    const href = WIDGET_RESOURCE_CONFIG.case.buildHref({
      filters: [
        { field: "projectOnboardingStatus", op: "in", values: ["Completed"] },
        { field: "projectOnboardingStatus", op: "notIn", values: ["In-Progress"] },
      ],
    });
    const parsed = readCasesFiltersFromUrl(hrefParams(href));

    // "Completed" is in the `in` list and isn't excluded, so it survives the
    // intersection with the notIn complement.
    expect(parsed.onboardingStatuses).toEqual(["Completed"]);
  });

  // Regression (CodeRabbit): a `notIn` excluding every one of the 4 known
  // values leaves the complement genuinely empty. The naive fix (`if
  // (onboardingStatuses.length > 0) out.onboardingStatuses = ...`) would
  // then leave the field unset entirely -- which this app's convention
  // reads as "unfiltered" -- silently showing every case instead of the
  // zero the widget's own filter actually calls for. The exact same
  // sign-flip failure mode this field's whole design exists to prevent, just
  // reached via the one degenerate input the complement conversion itself
  // introduced.
  it("projectOnboardingStatus notIn excluding all 4 known values resolves to a no-match filter, never an unfiltered one", () => {
    const href = WIDGET_RESOURCE_CONFIG.case.buildHref({
      filters: [
        {
          field: "projectOnboardingStatus",
          op: "notIn",
          values: ["In-Progress", "Not-Started", "Completed", "Not-Applicable"],
        },
      ],
    });
    const parsed = readCasesFiltersFromUrl(hrefParams(href));

    // Whatever the exact sentinel is, the field must be actively filtered
    // (non-empty) and must not equal any real onboarding-status choice.
    expect(parsed.onboardingStatuses.length).toBeGreaterThan(0);
    expect(parsed.onboardingStatuses).not.toEqual(
      expect.arrayContaining(["In-Progress", "Not-Started", "Completed", "Not-Applicable"]),
    );
  });

  it("a disjoint projectOnboardingStatus in/notIn pair (in minus notIn's complement is empty) also resolves to a no-match filter", () => {
    const href = WIDGET_RESOURCE_CONFIG.case.buildHref({
      filters: [
        { field: "projectOnboardingStatus", op: "in", values: ["Completed"] },
        { field: "projectOnboardingStatus", op: "notIn", values: ["Completed"] },
      ],
    });
    const parsed = readCasesFiltersFromUrl(hrefParams(href));

    expect(parsed.onboardingStatuses.length).toBeGreaterThan(0);
    expect(parsed.onboardingStatuses).not.toContain("Completed");
  });

  it("hasEscalation:false (isEmpty) round-trips distinctly from isNotEmpty", () => {
    const href = WIDGET_RESOURCE_CONFIG.case.buildHref({
      filters: [{ field: "escalation", op: "isEmpty" }],
    });
    const parsed = readCasesFiltersFromUrl(hrefParams(href));
    expect(parsed.hasEscalation).toBe(false);
  });

  it("gte and lte on the same date field both survive independently", () => {
    const href = WIDGET_RESOURCE_CONFIG.case.buildHref({
      filters: [
        { field: "updatedOn", op: "gte", values: ["2026-01-01"] },
        { field: "updatedOn", op: "lte", values: ["2026-06-30"] },
      ],
    });
    const parsed = readCasesFiltersFromUrl(hrefParams(href));
    expect(parsed.updatedOnGte).toBe("2026-01-01");
    expect(parsed.updatedOnLte).toBe("2026-06-30");
  });

  it("`abt_sla_at_risk` (>=80% elapsed) and `abt_sla_violations` (>=100% elapsed) now produce distinct hrefs, each carrying its own threshold", () => {
    // Mirrors the two real widgets' `filters` verbatim (reference/dashboard-config.json,
    // team placeholder already resolved to a concrete groupId — the same
    // shape `DashboardWidgetTile` passes to `buildHref` after
    // `resolveTeamPlaceholder`). Before the data-layer commit these two
    // hrefs were byte-identical because `taskSLABusinessElapsedPercent` was
    // dropped entirely — see the cases-list-advanced-filters task record.
    const teamFilters = [
      { field: "creTeam", op: "in", values: ["22222222-2222-2222-2222-222222222222"] },
      { field: "state", op: "in", values: ["open", "work_in_progress", "waiting_on_wso2"] },
    ];
    const atRiskHref = WIDGET_RESOURCE_CONFIG.case.buildHref({
      filters: [
        ...teamFilters,
        { field: "taskSLABusinessElapsedPercent", op: "gte", values: ["80"] },
      ],
    });
    const violationsHref = WIDGET_RESOURCE_CONFIG.case.buildHref({
      filters: [
        ...teamFilters,
        { field: "taskSLABusinessElapsedPercent", op: "gte", values: ["100"] },
      ],
    });

    expect(atRiskHref).not.toBe(violationsHref);

    const atRiskParsed = readCasesFiltersFromUrl(hrefParams(atRiskHref));
    const violationsParsed = readCasesFiltersFromUrl(hrefParams(violationsHref));
    expect(atRiskParsed.slaElapsedPctGte).toBe(80);
    expect(violationsParsed.slaElapsedPctGte).toBe(100);
    // Both still carry the shared team/state constraints — only the
    // threshold differs.
    expect(atRiskParsed.csTeams).toEqual(violationsParsed.csTeams);
    expect(atRiskParsed.states).toEqual(violationsParsed.states);
  });
});

/**
 * service_request / security_report_analysis / announcement / engagement:
 * additional case-table resourceTypes (see `BeWidgetResourceType`), all
 * routing to the same /cases/search endpoint and response shape as `case`
 * (only the click-through destination differs per type).
 */
describe("WIDGET_RESOURCE_CONFIG — case-table resourceTypes beyond `case`", () => {
  it("all route to /cases/search and read the cases[] items key, same as case", () => {
    for (const type of [
      "service_request",
      "security_report_analysis",
      "announcement",
      "engagement",
    ] as const) {
      expect(WIDGET_RESOURCE_CONFIG[type].searchEndpoint).toBe("/cases/search");
      expect(WIDGET_RESOURCE_CONFIG[type].itemsKey).toBe("cases");
    }
  });

  it("service_request's buildHref lands on the operations service-requests tab with translated filters", () => {
    const href = WIDGET_RESOURCE_CONFIG.service_request.buildHref({
      filters: [{ field: "state", op: "in", values: ["open"] }],
    });
    expect(href.startsWith("/operations?")).toBe(true);
    const params = hrefParams(href);
    expect(params.get("tab")).toBe("service_requests");
    expect(params.get("states")).toBe("open");
  });

  it("security_report_analysis's buildHref lands on the security center security-reports tab with translated filters", () => {
    const href = WIDGET_RESOURCE_CONFIG.security_report_analysis.buildHref({
      filters: [{ field: "state", op: "in", values: ["open"] }],
    });
    expect(href.startsWith("/security-center?")).toBe(true);
    const params = hrefParams(href);
    expect(params.get("tab")).toBe("security_reports");
    expect(params.get("states")).toBe("open");
  });

  it("engagement's buildHref lands on /engagements with translated filters", () => {
    const href = WIDGET_RESOURCE_CONFIG.engagement.buildHref({
      filters: [{ field: "state", op: "in", values: ["open"] }],
    });
    expect(href.startsWith("/engagements?")).toBe(true);
    const params = hrefParams(href);
    expect(params.get("states")).toBe("open");
  });

  // digiops-cs#2914: case/engagement click-throughs land on real, permanent
  // nav pages (/cases, /engagements) with a hardcoded heading — the widget's
  // own displayName must be carried through as WIDGET_TITLE_PARAM so that
  // page can show it instead, since several widgets all drill through to the
  // same page.
  it("case's buildHref carries the widget's displayName as WIDGET_TITLE_PARAM", () => {
    const href = WIDGET_RESOURCE_CONFIG.case.buildHref(
      { filters: [{ field: "state", op: "in", values: ["open"] }] },
      { widgetId: "outstanding_cases", displayName: "Total Outstanding" },
    );
    const params = hrefParams(href);
    expect(params.get(WIDGET_TITLE_PARAM)).toBe("Total Outstanding");
    // The rest of the translated filter is unaffected by the addition.
    expect(params.get("states")).toBe("open");
  });

  it("case's buildHref omits WIDGET_TITLE_PARAM when no ctx/displayName is given", () => {
    const href = WIDGET_RESOURCE_CONFIG.case.buildHref({
      filters: [{ field: "state", op: "in", values: ["open"] }],
    });
    expect(hrefParams(href).has(WIDGET_TITLE_PARAM)).toBe(false);
  });

  it("engagement's buildHref carries the widget's displayName as WIDGET_TITLE_PARAM", () => {
    const href = WIDGET_RESOURCE_CONFIG.engagement.buildHref(
      { filters: [{ field: "state", op: "in", values: ["open"] }] },
      { widgetId: "migration_summary_outstanding", displayName: "Total Outstanding" },
    );
    expect(href.startsWith("/engagements?")).toBe(true);
    const params = hrefParams(href);
    expect(params.get(WIDGET_TITLE_PARAM)).toBe("Total Outstanding");
    expect(params.get("states")).toBe("open");
  });

  it("engagement's buildHref omits WIDGET_TITLE_PARAM when no ctx/displayName is given", () => {
    const href = WIDGET_RESOURCE_CONFIG.engagement.buildHref({
      filters: [{ field: "state", op: "in", values: ["open"] }],
    });
    expect(hrefParams(href).has(WIDGET_TITLE_PARAM)).toBe(false);
  });

  it("announcement's buildHref is the unfiltered /announcements page (no URL filter scheme exists there yet)", () => {
    expect(
      WIDGET_RESOURCE_CONFIG.announcement.buildHref({
        filters: [{ field: "state", op: "in", values: ["open"] }],
      }),
    ).toBe("/announcements");
  });

  // Regression test: incident_task has no dedicated list route of its own
  // (only ever viewed as part of its parent incident), so its buildHref used
  // to fall back to the plain, unfiltered incidents tab -- silently dropping
  // this widget's own filters and landing the user on an unrelated result
  // set. It must route through the generic dashboard-widget preview page
  // instead, which is filter-aware, using the widgetId/displayName context
  // DashboardWidgetTile passes at call time.
  it("incident_task's buildHref routes to the widget preview page with widget context, not the unfiltered incidents tab", () => {
    const href = WIDGET_RESOURCE_CONFIG.incident_task.buildHref(
      { assignmentGroupIds: ["grp-1"] },
      { widgetId: "widget-42", displayName: "My Incident Tasks" },
    );
    expect(href.startsWith("/dashboard/preview/incident-tasks?")).toBe(true);
    const params = hrefParams(href);
    expect(params.get("w")).toBe("widget-42");
    expect(params.get("n")).toBe("My Incident Tasks");
  });

  // Regression (digiops-cs#2880): a widget's `anyOf` cross-field OR branches
  // have no representation in `CasesFilters` (an AND-only model), so
  // `translateCaseDashboardFilters` used to silently drop them -- a tile
  // reading a count restricted by `anyOf` landed its "View all" click on a
  // broader, unfiltered-by-`anyOf` cases list than what it actually counted.
  // Each case-family type must route to the widget preview page instead
  // (mirroring `incident_task` above) whenever `anyOf` is present, carrying
  // the widget context and round-tripping the OR branches through the URL
  // rather than dropping them.
  describe("anyOf routes to the widget preview page instead of a lossy CasesFilters translation", () => {
    const anyOfFilters = {
      filters: [{ field: "state", op: "in", values: ["open"] }],
      anyOf: [
        { filters: [{ field: "severity", op: "in", values: ["catastrophic", "critical"] }] },
        { filters: [{ field: "type", op: "in", values: ["security_report_analysis"] }] },
      ],
    };
    const ctx = { widgetId: "widget-99", displayName: "WOW P0/P1" };

    it("case", () => {
      const href = WIDGET_RESOURCE_CONFIG.case.buildHref(anyOfFilters, ctx);
      expect(href.startsWith("/dashboard/preview/cases?")).toBe(true);
      const params = hrefParams(href);
      expect(params.get("w")).toBe("widget-99");
      expect(params.get("n")).toBe("WOW P0/P1");
      expect(params.has("_anyOf")).toBe(true);
    });

    it("service_request", () => {
      const href = WIDGET_RESOURCE_CONFIG.service_request.buildHref(anyOfFilters, ctx);
      expect(href.startsWith("/dashboard/preview/service-requests?")).toBe(true);
    });

    it("security_report_analysis", () => {
      const href = WIDGET_RESOURCE_CONFIG.security_report_analysis.buildHref(anyOfFilters, ctx);
      expect(href.startsWith("/dashboard/preview/security-reports?")).toBe(true);
    });

    it("engagement", () => {
      const href = WIDGET_RESOURCE_CONFIG.engagement.buildHref(anyOfFilters, ctx);
      expect(href.startsWith("/dashboard/preview/engagements?")).toBe(true);
    });

    it("a case-family widget with no anyOf still lands on its own list page, unaffected", () => {
      const href = WIDGET_RESOURCE_CONFIG.case.buildHref(
        { filters: [{ field: "state", op: "in", values: ["open"] }] },
        ctx,
      );
      expect(href.startsWith("/cases?")).toBe(true);
    });
  });

  it("each of the four has its own distinct icon from `case` and from each other", () => {
    const icons = [
      WIDGET_RESOURCE_CONFIG.case.icon,
      WIDGET_RESOURCE_CONFIG.service_request.icon,
      WIDGET_RESOURCE_CONFIG.security_report_analysis.icon,
      WIDGET_RESOURCE_CONFIG.announcement.icon,
      WIDGET_RESOURCE_CONFIG.engagement.icon,
    ];
    expect(new Set(icons).size).toBe(icons.length);
  });
});

describe("WIDGET_RESOURCE_CONFIG.case_feedback.buildSearchRequestBody", () => {
  // The dashboard-widget preview page's URL round-trip (parseWidgetPreviewFilters)
  // decodes every query param as a comma-split string array — the shape every
  // other resourceType's filters use, but not case_feedback's own flat scalar
  // contract (dateFrom/dateTo/caseId/rating). A trend-bar or rating-pie slice's
  // click-through arrives here exactly as that parser produces it.
  const build = WIDGET_RESOURCE_CONFIG.case_feedback.buildSearchRequestBody!;

  it("unwraps array-wrapped dateFrom/dateTo/rating back to scalars", () => {
    const body = build({
      filters: { dateFrom: ["2026-08-01"], dateTo: ["2026-08-31"], rating: ["5"] },
      offset: 0,
      limit: 10,
    }) as { filters: Record<string, unknown> };

    expect(body.filters).toEqual({ dateFrom: "2026-08-01", dateTo: "2026-08-31", rating: 5 });
  });

  it("leaves already-scalar filters untouched (a tile-level fetch, no URL round trip)", () => {
    const body = build({
      filters: { dateFrom: "2026-08-01", rating: 5 },
      offset: 0,
      limit: 10,
    }) as { filters: Record<string, unknown> };

    expect(body.filters).toEqual({ dateFrom: "2026-08-01", rating: 5 });
  });
});

/**
 * `translateCallRequestDashboardFilters`/`callRequestWidgetFiltersToQuery` —
 * the call-requests "View more" landing's own seed/query pair, mirroring
 * `translateCaseDashboardFilters` for the much smaller (4-field, no op
 * choice except caseStates/excludeCaseStates) `/call-requests/search`
 * contract confirmed against `apps/csm-portal/backend/openapi.yaml`'s
 * `SearchAllCallRequestsPayload`.
 */
describe("translateCallRequestDashboardFilters", () => {
  it("passes every field of a widget's flat call-request filters through unchanged", () => {
    const out = translateCallRequestDashboardFilters({
      assignedUserIds: ["11111111-1111-1111-1111-111111111111"],
      states: ["scheduled", "pending_on_wso2"],
      caseStates: ["open", "work_in_progress"],
      excludeCaseStates: ["closed"],
      assignmentTeamIds: ["22222222-2222-2222-2222-222222222222"],
    });

    expect(out).toEqual({
      assignedUserIds: ["11111111-1111-1111-1111-111111111111"],
      states: ["scheduled", "pending_on_wso2"],
      caseStates: ["open", "work_in_progress"],
      excludeCaseStates: ["closed"],
      assignmentTeamIds: ["22222222-2222-2222-2222-222222222222"],
    });
  });

  it("omits a field entirely when its widget value is an empty array, rather than seeding an explicit empty filter", () => {
    const out = translateCallRequestDashboardFilters({
      assignedUserIds: [],
      states: ["scheduled"],
    });

    expect(out).toEqual({ states: ["scheduled"] });
    expect(out.assignedUserIds).toBeUndefined();
  });

  it("returns an empty object for a widget with no call-request filters at all", () => {
    expect(translateCallRequestDashboardFilters({})).toEqual({});
  });

  it("ignores a non-array/non-string value for a field rather than throwing", () => {
    expect(translateCallRequestDashboardFilters({ states: "scheduled" })).toEqual({});
  });
});

describe("callRequestWidgetFiltersToQuery", () => {
  it("omits every empty-array field so an untouched control means 'unfiltered', not 'match nothing'", () => {
    expect(callRequestWidgetFiltersToQuery(DEFAULT_CALL_REQUEST_WIDGET_FILTERS)).toEqual({});
  });

  it("includes only the fields that carry a value", () => {
    const query = callRequestWidgetFiltersToQuery({
      ...DEFAULT_CALL_REQUEST_WIDGET_FILTERS,
      states: ["concluded"],
      excludeCaseStates: ["closed"],
    });

    expect(query).toEqual({ states: ["concluded"], excludeCaseStates: ["closed"] });
  });

  it("round-trips a full filter set through translate -> toQuery unchanged", () => {
    const widgetFilters = {
      assignedUserIds: ["11111111-1111-1111-1111-111111111111"],
      states: ["scheduled"],
      caseStates: ["open"],
      excludeCaseStates: ["closed"],
      assignmentTeamIds: ["22222222-2222-2222-2222-222222222222"],
    };
    const seeded = {
      ...DEFAULT_CALL_REQUEST_WIDGET_FILTERS,
      ...translateCallRequestDashboardFilters(widgetFilters),
    };

    expect(callRequestWidgetFiltersToQuery(seeded)).toEqual(widgetFilters);
  });
});
