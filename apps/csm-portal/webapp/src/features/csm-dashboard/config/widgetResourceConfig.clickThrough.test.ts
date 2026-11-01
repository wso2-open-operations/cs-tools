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
 * A count tile and the list it links to must show the same records. The tile
 * counts with the widget's own filters; the list is rebuilt from a URL the
 * click-through writes, and any filter the URL scheme (or the list) has no
 * place for used to vanish, so the list held more than the tile counted.
 *
 * This suite pins the contract from both ends: for each widget shape that
 * occurs on a real overview dashboard, either the list the click opens sends
 * a search request equal to the widget's own, or the click opens the preview
 * page, which applies the widget's filters unchanged.
 */

import { describe, expect, it } from "vitest";
import type { BeCaseFieldFilter, BeWidgetResourceType } from "@api/backend/types";
import {
  translateCaseDashboardFiltersChecked,
  WIDGET_RESOURCE_CONFIG,
} from "@features/csm-dashboard/config/widgetResourceConfig";
import { readCasesFiltersFromUrl } from "@features/csm-cases/utils/casesFiltersUrl";
import { buildCaseSearchFilters } from "@features/csm-cases/utils/caseSearchPayload";
import { resolveAdvancedFilterDateValues } from "@features/csm-cases/utils/advancedFilters";
import { parseWidgetPreviewFilters } from "@features/csm-dashboard/utils/widgetPreviewUrl";

function hrefParams(href: string): URLSearchParams {
  const [, qs] = href.split("?");
  return new URLSearchParams(qs ?? "");
}

type Entry = { field: string; op: string; values?: string[] };

const entry = (field: string, op: string, ...values: string[]): Entry => ({
  field,
  op,
  ...(values.length > 0 ? { values } : {}),
});

/** One comparable string per predicate. Several `notIn` entries on a field
 * mean "none of these" and are the same predicate as one merged entry. */
function normalize(entries: Entry[] | undefined): string[] {
  const notIn = new Map<string, Set<string>>();
  const out: string[] = [];
  for (const e of entries ?? []) {
    const values = e.values ?? [];
    if (e.op === "notIn") {
      const merged = notIn.get(e.field) ?? new Set<string>();
      values.forEach((v) => merged.add(v));
      notIn.set(e.field, merged);
    } else {
      out.push(`${e.field}|${e.op}|${[...values].sort().join(",")}`);
    }
  }
  notIn.forEach((values, field) => out.push(`${field}|notIn|${[...values].sort().join(",")}`));
  return out.sort();
}

// ---------------------------------------------------------------------------
// Widget shapes of a real overview dashboard (ids and names are made up).
// ---------------------------------------------------------------------------

const PROJECT_TYPES = [
  "Subscription",
  "Cloud Support",
  "Managed Cloud Subscription",
  "Development Support",
];
const ONBOARDING_FILTERED_OUT = ["Cancelled", "Completed", "Expired", "Not-Applicable", "OnHold"];
const ACTIVE_STATES = ["open", "work_in_progress", "waiting_on_wso2", "reopened"];
const SERVICE_IDS = Array.from(
  { length: 50 },
  (_, i) => `00000000-0000-4000-8000-${String(i).padStart(12, "0")}`,
);
const TODAY_START = "2026-10-12T18:30:00.000Z";
const THIRTY_DAYS_AGO_END = "2026-09-13T18:29:59.999Z";

interface WidgetShape {
  name: string;
  resourceType: BeWidgetResourceType;
  filters: Record<string, unknown>;
  /** Where the click should land. */
  lands: "list" | "preview";
}

const caseScope = [
  entry("projectType", "in", ...PROJECT_TYPES),
  entry("projectOnboardingStatus", "in", ...ONBOARDING_FILTERED_OUT),
  entry("type", "in", "case"),
];

const WIDGETS: WidgetShape[] = [
  {
    name: "open cases",
    resourceType: "case",
    filters: { filters: [entry("state", "in", "open"), ...caseScope] },
    lands: "list",
  },
  {
    name: "SLA violations (breached SLA, still being worked)",
    resourceType: "case",
    filters: {
      filters: [
        entry("slaBreached", "eq", "true"),
        entry("state", "in", "open", "work_in_progress", "waiting_on_wso2"),
        ...caseScope,
      ],
    },
    lands: "list",
  },
  {
    name: "resolved today",
    resourceType: "case",
    filters: { filters: [entry("resolvedOn", "gte", TODAY_START), ...caseScope] },
    lands: "list",
  },
  {
    name: "created today",
    resourceType: "case",
    filters: { filters: [entry("createdOn", "gte", TODAY_START), ...caseScope] },
    lands: "list",
  },
  {
    name: "older than 30 days, reopened included, patches excluded",
    resourceType: "case",
    filters: {
      filters: [
        entry("state", "in", ...ACTIVE_STATES),
        entry("createdOn", "lte", THIRTY_DAYS_AGO_END),
        entry("tag", "notIn", "patch"),
        entry("projectType", "in", ...PROJECT_TYPES.slice(0, 3)),
        entry("projectOnboardingStatus", "in", ...ONBOARDING_FILTERED_OUT),
        entry("type", "in", "case"),
      ],
    },
    lands: "list",
  },
  {
    name: "waiting on us, patches AND dip tickets excluded (two tag exclusions)",
    resourceType: "case",
    filters: {
      filters: [
        entry("state", "in", "waiting_on_wso2"),
        entry("projectType", "in", ...PROJECT_TYPES),
        entry("projectOnboardingStatus", "in", ...ONBOARDING_FILTERED_OUT),
        entry("tag", "notIn", "patch"),
        entry("tag", "notIn", "s_dip"),
        entry("type", "in", "case"),
      ],
    },
    lands: "list",
  },
  {
    name: "incidents at WSO2 by severity, reopened included",
    resourceType: "case",
    filters: {
      filters: [
        entry("severity", "in", "catastrophic", "critical", "high", "medium"),
        entry("tag", "notIn", "patch"),
        entry("state", "in", ...ACTIVE_STATES),
        entry("projectType", "in", ...PROJECT_TYPES.slice(0, 3)),
        entry("type", "in", "case"),
      ],
    },
    lands: "list",
  },
  {
    name: "escalated cases (account escalation, which the list cannot hold)",
    resourceType: "case",
    filters: {
      filters: [
        entry("accountEscalationActive", "eq", "true"),
        entry("state", "in", ...ACTIVE_STATES),
        entry("tag", "notIn", "patch"),
        entry("tag", "notIn", "s_dip"),
        entry("type", "in", "case"),
      ],
    },
    lands: "preview",
  },
  {
    name: "migration engagements",
    resourceType: "engagement",
    filters: {
      filters: [
        entry("state", "in", ...ACTIVE_STATES),
        entry("engagementType", "in", "migration"),
        entry("projectType", "in", ...PROJECT_TYPES.slice(0, 3)),
        entry("type", "in", "engagement"),
      ],
    },
    lands: "list",
  },
  {
    name: "security reports past their SLA",
    resourceType: "security_report_analysis",
    filters: {
      filters: [
        entry("taskSLABusinessElapsedPercent", "gte", "100"),
        entry("state", "notIn", "solution_proposed", "closed"),
        entry("type", "in", "security_report_analysis"),
      ],
    },
    lands: "list",
  },
  {
    name: "open service requests",
    resourceType: "service_request",
    filters: { filters: [entry("type", "in", "service_request"), entry("state", "in", "open")] },
    lands: "list",
  },
  {
    name: "onboarding cases, two tag exclusions and reopened",
    resourceType: "case",
    filters: {
      filters: [
        entry("state", "in", "open", "work_in_progress", "waiting_on_wso2", "awaiting_info", "reopened"),
        entry("projectOnboardingStatus", "in", "In-Progress", "Not-Started", "OnHold"),
        entry("tag", "notIn", "patch"),
        entry("tag", "notIn", "s_dip"),
        entry("type", "in", "case"),
      ],
    },
    lands: "list",
  },
  {
    name: "new incidents of a team's business services",
    resourceType: "incident",
    filters: { filters: [entry("state", "in", "NEW"), entry("businessServiceId", "in", ...SERVICE_IDS)] },
    lands: "preview",
  },
  {
    name: "incident SLA violations",
    resourceType: "incident",
    filters: {
      filters: [
        entry("incidentStateKeys", "in", "1", "2"),
        entry("madeSla", "eq", "true"),
        entry("businessServiceId", "in", ...SERVICE_IDS),
      ],
    },
    lands: "preview",
  },
  {
    name: "approved change requests (approval outcome)",
    resourceType: "change_request",
    filters: { filters: [entry("approval", "eq", "approved")] },
    lands: "preview",
  },
  {
    name: "change requests waiting in a state",
    resourceType: "change_request",
    filters: { states: ["customer_approval"] },
    lands: "list",
  },
  {
    name: "open problems",
    resourceType: "problem",
    filters: { filters: [entry("state", "in", "NEW")] },
    lands: "preview",
  },
];

const ctx = { widgetId: "widget-1", displayName: "A widget" };

describe("a dashboard tile and the list it opens show the same records", () => {
  it.each(WIDGETS.filter((w) => w.lands === "list" && w.resourceType !== "change_request"))(
    "$name: the list sends the widget's own search request",
    ({ resourceType, filters }) => {
      const href = WIDGET_RESOURCE_CONFIG[resourceType].buildHref(filters, ctx);
      expect(href.startsWith("/dashboard/preview/")).toBe(false);

      const listFilters = readCasesFiltersFromUrl(hrefParams(href));
      // Rows with a relative date are resolved by the list before the request.
      const resolved = {
        ...listFilters,
        advancedFilters: resolveAdvancedFilterDateValues(listFilters.advancedFilters, new Date()),
      };
      const sent: BeCaseFieldFilter[] = buildCaseSearchFilters(resolved, "", undefined).filters ?? [];

      expect(normalize(sent)).toEqual(normalize((filters as { filters: Entry[] }).filters));
    },
  );

  it.each(WIDGETS.filter((w) => w.lands === "preview"))(
    "$name: opens the preview page, which carries every filter unchanged",
    ({ resourceType, filters }) => {
      const href = WIDGET_RESOURCE_CONFIG[resourceType].buildHref(filters, ctx);
      expect(href.startsWith("/dashboard/preview/")).toBe(true);

      const params = hrefParams(href);
      expect(params.get("w")).toBe(ctx.widgetId);
      expect(params.get("n")).toBe(ctx.displayName);
      const carried = parseWidgetPreviewFilters(params).filters as { filters?: Entry[] };
      expect(normalize(carried.filters)).toEqual(normalize((filters as { filters: Entry[] }).filters));
    },
  );

  it("change requests that only filter by state still open their own list", () => {
    const href = WIDGET_RESOURCE_CONFIG.change_request.buildHref({ states: ["customer_approval"] }, ctx);
    expect(href.startsWith("/operations?")).toBe(true);
    expect(hrefParams(href).get("tab")).toBe("change_requests");
  });
});

describe("what the list now holds", () => {
  it("carries `slaBreached` as a visible Advanced row and sends it", () => {
    const href = WIDGET_RESOURCE_CONFIG.case.buildHref({
      filters: [entry("slaBreached", "eq", "true"), entry("state", "in", "open")],
    });
    const parsed = readCasesFiltersFromUrl(hrefParams(href));

    expect(parsed.advancedFilters).toEqual([{ field: "slaBreached", op: "eq", values: ["true"] }]);
    expect(buildCaseSearchFilters(parsed, "", undefined).filters).toContainEqual({
      field: "slaBreached",
      op: "eq",
      values: ["true"],
    });
  });

  it("carries `resolvedOn` bounds as Advanced rows", () => {
    const href = WIDGET_RESOURCE_CONFIG.case.buildHref({
      filters: [
        entry("resolvedOn", "gte", "2026-10-01T00:00:00.000Z"),
        entry("resolvedOn", "lte", "2026-10-31T23:59:59.999Z"),
      ],
    });

    expect(readCasesFiltersFromUrl(hrefParams(href)).advancedFilters).toEqual([
      { field: "resolvedOn", op: "gte", values: ["2026-10-01T00:00:00.000Z"] },
      { field: "resolvedOn", op: "lte", values: ["2026-10-31T23:59:59.999Z"] },
    ]);
  });

  it("merges several tag exclusions into one list instead of keeping the first", () => {
    const href = WIDGET_RESOURCE_CONFIG.case.buildHref({
      filters: [entry("tag", "notIn", "patch"), entry("tag", "notIn", "s_dip")],
    });

    expect(readCasesFiltersFromUrl(hrefParams(href)).excludeTags).toEqual(["patch", "s_dip"]);
  });

  it("merges several state exclusions the same way", () => {
    const href = WIDGET_RESOURCE_CONFIG.case.buildHref({
      filters: [entry("state", "notIn", "closed"), entry("state", "notIn", "solution_proposed")],
    });

    expect(readCasesFiltersFromUrl(hrefParams(href)).excludeStates).toEqual([
      "closed",
      "solution_proposed",
    ]);
  });

  it("keeps the `reopened` state the list used to drop from its URL", () => {
    const href = WIDGET_RESOURCE_CONFIG.case.buildHref({
      filters: [entry("state", "in", "open", "reopened")],
    });
    const parsed = readCasesFiltersFromUrl(hrefParams(href));

    expect(parsed.states).toEqual(["open", "reopened"]);
    expect(buildCaseSearchFilters(parsed, "", undefined).filters).toContainEqual({
      field: "state",
      op: "in",
      values: ["open", "reopened"],
    });
  });
});

describe("translateCaseDashboardFiltersChecked — `lossless`", () => {
  const lossless = (filters: Record<string, unknown>): boolean =>
    translateCaseDashboardFiltersChecked(filters).lossless;

  it("is true for a widget with no filters at all, and for one whose entries all translate", () => {
    expect(lossless({})).toBe(true);
    expect(lossless({ filters: [entry("state", "in", "open"), entry("tag", "notIn", "patch")] })).toBe(true);
  });

  it("ignores an entry that carries no values: it constrains nothing", () => {
    expect(lossless({ filters: [entry("state", "in"), entry("severity", "in")] })).toBe(true);
  });

  it.each([
    ["a field the list has no filter for", { filters: [entry("accountEscalationActive", "eq", "true")] }],
    ["a parent case filter", { filters: [entry("parentId", "eq", "CS0000001")] }],
    ["a second `in` entry on one field, which is an intersection", {
      filters: [entry("state", "in", "open", "closed"), entry("state", "in", "open")],
    }],
    ["a bound given twice", {
      filters: [entry("createdOn", "gte", "2026-01-01"), entry("createdOn", "gte", "2026-02-01")],
    }],
    ["a severity the list has no code for", { filters: [entry("severity", "in", "catastrophic", "unheard-of")] }],
    ["an `slaBreached` that is not a boolean", { filters: [entry("slaBreached", "eq", "maybe")] }],
    ["an OR group", { filters: [], anyOf: [{ filters: [entry("state", "in", "open")] }] }],
    ["a flat legacy key beside the entries", { filters: [], states: ["open"] }],
    ["entries that are not an array", { filters: "state=open" }],
    ["a state the list's URL reader does not know", { filters: [entry("state", "in", "no-such-state")] }],
    ["a work state without the single in-progress state", { filters: [entry("workState", "in", "paused")] }],
  ])("is false for %s", (_label, filters) => {
    expect(lossless(filters as Record<string, unknown>)).toBe(false);
  });
});

describe("a case widget that counts distinct accounts", () => {
  const filters = { filters: [entry("state", "in", "open")] };

  it("opens the preview page grouped by account, even when its filters fit the case list", () => {
    const href = WIDGET_RESOURCE_CONFIG.case.buildHref(filters, {
      ...ctx,
      distinctByField: "account",
    });
    const params = hrefParams(href);

    expect(href.startsWith("/dashboard/preview/cases?")).toBe(true);
    expect(params.get("gb")).toBe("account");
    expect(params.get("w")).toBe(ctx.widgetId);
    // `gb` is a view option, never read back as a filter.
    expect(JSON.stringify(parseWidgetPreviewFilters(params).filters)).not.toContain("gb");
  });

  it("keeps every filter, including the ones the case list cannot hold", () => {
    const escalated = {
      filters: [entry("accountEscalationActive", "eq", "true"), entry("state", "in", "open")],
    };
    const params = hrefParams(
      WIDGET_RESOURCE_CONFIG.case.buildHref(escalated, { ...ctx, distinctByField: "account" }),
    );
    expect(params.get("gb")).toBe("account");
    expect(parseWidgetPreviewFilters(params).filters).toEqual(escalated);
  });

  it("is unchanged for a widget with no distinctBy, or one on another field", () => {
    expect(WIDGET_RESOURCE_CONFIG.case.buildHref(filters, ctx).startsWith("/cases?")).toBe(true);
    const other = WIDGET_RESOURCE_CONFIG.case.buildHref(filters, { ...ctx, distinctByField: "state" });
    expect(other.startsWith("/cases?")).toBe(true);
  });
});

describe("incident, change request and problem click-throughs", () => {
  it("incidents filtered by priority alone still open the incidents list", () => {
    const href = WIDGET_RESOURCE_CONFIG.incident.buildHref({ priorities: ["CRITICAL"] }, ctx);
    expect(href.startsWith("/operations?")).toBe(true);
  });

  it("any other incident filter opens the preview page, with a generic title when no widget is at hand", () => {
    const href = WIDGET_RESOURCE_CONFIG.incident.buildHref({ filters: [entry("state", "in", "NEW")] });
    const params = hrefParams(href);

    expect(href.startsWith("/dashboard/preview/incidents?")).toBe(true);
    expect(params.get("w")).toBeTruthy();
    expect(params.get("n")).toBeTruthy();
  });

  it("a problem widget with no filters opens the plain problems tab", () => {
    expect(WIDGET_RESOURCE_CONFIG.problem.buildHref({}, ctx)).toBe("/operations?tab=problems");
  });

  it("an empty filter value does not count as a filter", () => {
    expect(
      WIDGET_RESOURCE_CONFIG.problem.buildHref({ filters: [], states: [], note: "" }, ctx),
    ).toBe("/operations?tab=problems");
  });
});
