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

import { renderHook } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

const { post, get } = vi.hoisted(() => ({ post: vi.fn(), get: vi.fn() }));
vi.mock("@api/backend/client", () => ({ useBackendApi: () => ({ post, get }) }));
vi.mock("@context/current-user/CurrentUserContext", () => ({ useCurrentUser: () => ({ user: null }) }));
vi.mock("@config/apiConfig", () => ({ apiConfig: { backendUrl: "https://example.test" } }));

import {
  buildCaseTrendRequest,
  buildCountCasesRequest,
  buildSearchCasesRequest,
  createNamedFilterResolver,
  hasNamedFilter,
  toCaseTrendResult,
  toCountCasesResult,
  toSearchCasesResult,
  useCodeDashboardPortal,
} from "@features/csm-custom-dashboards/api/useCodeDashboardPortal";

const myOpenCases = [
  { field: "assignedUserId", op: "in", values: ["00000000-0000-0000-0000-000000000001"] },
  { field: "state", op: "notIn", values: ["closed"] },
];

describe("buildCountCasesRequest", () => {
  it("wraps the filters and passes groupBy through", () => {
    expect(buildCountCasesRequest({ filters: myOpenCases, groupBy: "severity" })).toEqual({
      body: { filters: { filters: myOpenCases }, groupBy: "severity" },
      grouped: true,
    });
  });

  it("groups a plain count by state and marks it ungrouped", () => {
    expect(buildCountCasesRequest({ filters: myOpenCases })).toEqual({
      body: { filters: { filters: myOpenCases }, groupBy: "state" },
      grouped: false,
    });
  });

  it("accepts no args at all", () => {
    expect(buildCountCasesRequest(undefined).body).toEqual({ filters: { filters: [] }, groupBy: "state" });
  });

  it.each(["product", "project", "region"])("passes groupBy %s through", (groupBy) => {
    expect(buildCountCasesRequest({ groupBy, maxGroups: 10 }).body).toEqual({
      filters: { filters: [] },
      groupBy,
      maxGroups: 10,
    });
  });

  it("passes groupBy productVersion through beside a product filter naming one product", () => {
    const filters = [
      { field: "state", op: "notIn", values: ["closed"] },
      { field: "product", op: "in", values: ["API Manager"] },
    ];
    expect(buildCountCasesRequest({ filters, groupBy: "productVersion" }).body).toEqual({
      filters: { filters },
      groupBy: "productVersion",
    });
  });

  it("passes a positive maxGroups through", () => {
    expect(buildCountCasesRequest({ groupBy: "account", maxGroups: 5 }).body.maxGroups).toBe(5);
  });

  it.each([
    [{ groupBy: "assignedUserId" }, /groupBy must be one of/],
    [{ groupBy: "productVersion" }, /naming exactly one product/],
    [
      { groupBy: "productVersion", filters: [{ field: "product", op: "in", values: ["API Manager", "Identity Hub"] }] },
      /naming exactly one product/,
    ],
    [
      { groupBy: "productVersion", filters: [{ field: "product", op: "notIn", values: ["API Manager"] }] },
      /naming exactly one product/,
    ],
    [{ filters: { field: "state" } }, /filters must be an array/],
    [{ groupBy: "state", maxGroups: 0 }, /maxGroups must be a positive integer/],
  ])("refuses %j", (args, message) => {
    expect(() => buildCountCasesRequest(args)).toThrow(message);
  });
});

describe("toCountCasesResult", () => {
  const response = {
    groups: [
      { key: "high", label: "high", count: 3 },
      { key: "low", label: "low", count: 1 },
    ],
    othersCount: 0,
    totalRecords: 4,
  };

  it("keeps label and count per group and the total", () => {
    expect(toCountCasesResult(response, true)).toEqual({
      total: 4,
      groups: [
        { label: "high", count: 3 },
        { label: "low", count: 1 },
      ],
    });
  });

  it("adds an Others group when buckets were folded", () => {
    expect(toCountCasesResult({ ...response, othersCount: 2, totalRecords: 6 }, true).groups).toContainEqual({
      label: "Others",
      count: 2,
    });
  });

  it("returns only the total for an ungrouped count", () => {
    expect(toCountCasesResult(response, false)).toEqual({ total: 4 });
  });

  it("reads an empty response as zero", () => {
    expect(toCountCasesResult({}, true)).toEqual({ total: 0, groups: [] });
  });
});

describe("buildSearchCasesRequest", () => {
  const escalated = [{ field: "escalation", op: "isNotEmpty" }];

  it("wraps the filters, passes sortBy through and reads the first page", () => {
    expect(
      buildSearchCasesRequest({ filters: escalated, sortBy: { field: "createdOn", order: "asc" }, limit: 5 }),
    ).toEqual({
      filters: { filters: escalated },
      sortBy: { field: "createdOn", order: "asc" },
      pagination: { offset: 0, limit: 5 },
    });
  });

  it("defaults to 10 cases, no sort and no filters", () => {
    expect(buildSearchCasesRequest(undefined)).toEqual({
      filters: { filters: [] },
      pagination: { offset: 0, limit: 10 },
    });
  });

  it("sorts descending when no order is given", () => {
    expect(buildSearchCasesRequest({ sortBy: { field: "updatedOn" } }).sortBy).toEqual({
      field: "updatedOn",
      order: "desc",
    });
  });

  it.each([
    [{ filters: { field: "state" } }, /filters must be an array/],
    [{ limit: 0 }, /limit must be an integer from 1 to 50/],
    [{ limit: 51 }, /limit must be an integer from 1 to 50/],
    [{ limit: 2.5 }, /limit must be an integer from 1 to 50/],
    [{ sortBy: { field: "subject" } }, /sortBy.field must be one of/],
    [{ sortBy: "createdOn" }, /sortBy.field must be one of/],
    [{ sortBy: { field: "createdOn", order: "up" } }, /sortBy.order must be "asc" or "desc"/],
  ])("refuses %j", (args, message) => {
    expect(() => buildSearchCasesRequest(args)).toThrow(message);
  });
});

describe("toSearchCasesResult", () => {
  it("flattens each case to plain values, related records by name", () => {
    const result = toSearchCasesResult({
      total: 6,
      cases: [
        {
          id: "00000000-0000-0000-0000-000000000013",
          number: "CS0000013",
          subject: "Gateway rejecting requests",
          description: "<p>Long body that must not be passed on</p>",
          type: "case",
          severity: "catastrophic",
          state: "work_in_progress",
          escalationLevel: "1",
          createdOn: "2026-10-20T12:25:20Z",
          resolvedOn: "2026-10-22T08:00:00Z",
          closedOn: "2026-10-23T09:30:00Z",
          account: { id: "a1", name: "Acme Corp" },
          project: { id: "p1", name: "Acme Gateway" },
          assignedEngineer: { id: "u1", email: "jane.doe@example.com", name: "Jane Doe" },
        },
      ],
    });
    expect(result).toEqual({
      total: 6,
      items: [
        {
          id: "00000000-0000-0000-0000-000000000013",
          number: "CS0000013",
          subject: "Gateway rejecting requests",
          type: "case",
          severity: "catastrophic",
          state: "work_in_progress",
          escalationLevel: "1",
          createdOn: "2026-10-20T12:25:20Z",
          updatedOn: null,
          resolvedOn: "2026-10-22T08:00:00Z",
          closedOn: "2026-10-23T09:30:00Z",
          account: "Acme Corp",
          project: "Acme Gateway",
          assignee: "Jane Doe",
        },
      ],
    });
  });

  it("reads missing related records and an empty response as null / empty", () => {
    expect(toSearchCasesResult({ cases: [{ id: "c1", assignedEngineer: null }] }).items[0]).toMatchObject({
      number: null,
      resolvedOn: null,
      closedOn: null,
      account: null,
      project: null,
      assignee: null,
    });
    expect(toSearchCasesResult({})).toEqual({ total: 0, items: [] });
  });

  it("passes on a closed time with no resolved time as it is", () => {
    const [item] = toSearchCasesResult({
      cases: [{ id: "c1", resolvedOn: null, closedOn: "2026-10-02T12:21:51Z" }],
    }).items;
    expect(item.resolvedOn).toBeNull();
    expect(item.closedOn).toBe("2026-10-02T12:21:51Z");
  });
});

describe("hasNamedFilter", () => {
  it.each(["product", "productVersion", "project", "account", "creTeam", "sreTeam", "region"])("is true for a %s filter", (field) => {
    expect(hasNamedFilter([{ field: "state", op: "in", values: ["open"] }, { field, op: "in", values: ["X"] }])).toBe(true);
  });

  it("is false for filters that name nothing", () => {
    expect(hasNamedFilter(myOpenCases)).toBe(false);
    expect(hasNamedFilter([])).toBe(false);
    expect(hasNamedFilter(["project", null, 7])).toBe(false);
  });
});

describe("createNamedFilterResolver", () => {
  const byProject = [
    { field: "state", op: "notIn", values: ["closed"] },
    { field: "project", op: "in", values: ["Example Corp Production"] },
  ];
  const resolved = [
    { field: "state", op: "notIn", values: ["closed"] },
    { field: "projectId", op: "in", values: ["proj-1"] },
  ];

  it("returns a list that names nothing as it is, without a lookup", async () => {
    const post = vi.fn();
    await expect(createNamedFilterResolver(post)(myOpenCases)).resolves.toBe(myOpenCases);
    expect(post).not.toHaveBeenCalled();
  });

  it("sends a list that names something and returns the resolved list", async () => {
    const post = vi.fn().mockResolvedValue({ filters: resolved });
    await expect(createNamedFilterResolver(post)(byProject)).resolves.toEqual(resolved);
    expect(post).toHaveBeenCalledWith({ filters: byProject });
  });

  it("looks one list up once, and another list again", async () => {
    const post = vi.fn().mockResolvedValue({ filters: resolved });
    const resolve = createNamedFilterResolver(post);
    await Promise.all([resolve(byProject), resolve(byProject)]);
    await resolve([...byProject]);
    expect(post).toHaveBeenCalledTimes(1);
    await resolve([{ field: "account", op: "in", values: ["Example Corp"] }]);
    expect(post).toHaveBeenCalledTimes(2);
  });

  it("passes a failed lookup's error on and tries again next time", async () => {
    const post = vi
      .fn()
      .mockRejectedValueOnce(new Error("No project matches No Such Project."))
      .mockResolvedValueOnce({ filters: resolved });
    const resolve = createNamedFilterResolver(post);
    await expect(resolve(byProject)).rejects.toThrow("No project matches No Such Project.");
    await expect(resolve(byProject)).resolves.toEqual(resolved);
    expect(post).toHaveBeenCalledTimes(2);
  });

  it("keeps each resolver's lookups to itself", async () => {
    const post = vi.fn().mockResolvedValue({ filters: resolved });
    await createNamedFilterResolver(post)(byProject);
    await createNamedFilterResolver(post)(byProject);
    expect(post).toHaveBeenCalledTimes(2);
  });
});

describe("buildCaseTrendRequest", () => {
  it("reads the last 30 days when no window is given", () => {
    expect(buildCaseTrendRequest(undefined)).toEqual({ filters: { filters: [] }, days: 30 });
  });

  it("passes a trailing window of days through", () => {
    expect(buildCaseTrendRequest({ filters: myOpenCases, days: 90 })).toEqual({ filters: { filters: myOpenCases }, days: 90 });
  });

  it("passes a fixed window, with or without a bucket", () => {
    expect(buildCaseTrendRequest({ startDate: "2025-12-01", endDate: "2025-12-31" })).toEqual({
      filters: { filters: [] },
      startDate: "2025-12-01",
      endDate: "2025-12-31",
    });
    expect(buildCaseTrendRequest({ startDate: "__startOfYear:0__", endDate: "__today__", bucket: "month" })).toEqual({
      filters: { filters: [] },
      startDate: "__startOfYear:0__",
      endDate: "__today__",
      bucket: "month",
    });
  });

  it.each([
    [{ filters: { field: "state" } }, /filters must be an array/],
    [{ days: 0 }, /days must be an integer from 1 to 90/],
    [{ days: 91 }, /days must be an integer from 1 to 90/],
    [{ days: 7.5 }, /days must be an integer/],
    [{ bucket: "week" }, /bucket needs startDate and endDate/],
    [{ days: 30, startDate: "2026-10-01", endDate: "2026-10-31" }, /not both/],
    [{ startDate: "2026-10-01" }, /startDate and endDate are both needed/],
    [{ endDate: "2026-10-31" }, /startDate and endDate are both needed/],
    [{ startDate: "", endDate: "2026-10-31" }, /startDate and endDate are both needed/],
    [{ startDate: "2026-10-01", endDate: "2026-10-31", bucket: "year" }, /bucket must be one of day, week, month/],
  ])("refuses %j", (args, message) => {
    expect(() => buildCaseTrendRequest(args)).toThrow(message);
  });
});

describe("toCaseTrendResult", () => {
  it("puts each period's opened and closed counts together, oldest first", () => {
    const res = {
      days: 14,
      bucket: "week" as const,
      series: [
        { label: "Closed", points: [{ date: "2026-10-05", endDate: "2026-10-11", count: 1 }, { date: "2026-10-12", endDate: "2026-10-18", count: 0 }] },
        { label: "Opened", points: [{ date: "2026-10-05", endDate: "2026-10-11", count: 6 }, { date: "2026-10-12", endDate: "2026-10-18", count: 9 }] },
      ],
    };
    expect(toCaseTrendResult(res)).toEqual({
      points: [
        { date: "2026-10-05", endDate: "2026-10-11", opened: 6, closed: 1 },
        { date: "2026-10-12", endDate: "2026-10-18", opened: 9, closed: 0 },
      ],
    });
  });

  it("leaves endDate out of a day point and reads no series as no points", () => {
    expect(toCaseTrendResult({ days: 1, series: [{ label: "Opened", points: [{ date: "2026-10-05", count: 2 }] }] })).toEqual({
      points: [{ date: "2026-10-05", opened: 2, closed: 0 }],
    });
    expect(toCaseTrendResult({ days: 1, series: [] })).toEqual({ points: [] });
  });
});

describe("useCodeDashboardPortal", () => {
  const byProject = [{ field: "project", op: "in", values: ["Example Corp Production"] }];
  const resolved = [{ field: "projectId", op: "in", values: ["proj-1"] }];

  // The backend answers the lookup with the resolved list and any case call
  // with an empty result.
  function answer(path: string) {
    return Promise.resolve(path === "/dashboards/resolved-filters" ? { filters: resolved } : {});
  }

  it.each([
    ["countCases", "/cases/aggregate"],
    ["searchCases", "/cases/search"],
    ["caseTrend", "/cases/volume-trend"],
    ["caseResolutionTime", "/cases/resolution-time"],
  ])("%s resolves named filters before calling %s", async (fn, path) => {
    post.mockReset().mockImplementation(answer);
    const { result } = renderHook(() => useCodeDashboardPortal());
    await result.current[fn]({ filters: byProject });
    expect(post.mock.calls.map((c) => c[0])).toEqual(["/dashboards/resolved-filters", path]);
    expect(post.mock.calls[1][1].filters).toEqual({ filters: resolved });
  });

  it.each([
    ["countTimeCards", "/time-cards/aggregate"],
    ["searchTimeCards", "/time-cards/search"],
  ])("%s calls %s with no lookup", async (fn, path) => {
    post.mockReset().mockImplementation(answer);
    const { result } = renderHook(() => useCodeDashboardPortal());
    await result.current[fn]({ states: ["submitted"] });
    expect(post.mock.calls.map((c) => c[0])).toEqual([path]);
    expect(post.mock.calls[0][1].filters).toEqual({ states: ["submitted"] });
  });

  it.each([
    ["countTimeCards", "/time-cards/aggregate"],
    ["searchTimeCards", "/time-cards/search"],
  ])("%s looks its teams up before calling %s", async (fn, path) => {
    post
      .mockReset()
      .mockImplementation((p: string) =>
        Promise.resolve(p === "/dashboards/resolved-filters" ? { teamIds: ["group-rigel"] } : {}),
      );
    const { result } = renderHook(() => useCodeDashboardPortal());
    await result.current[fn]({ teams: ["rigel"] });
    expect(post.mock.calls.map((c) => c[0])).toEqual(["/dashboards/resolved-filters", path]);
    expect(post.mock.calls[0][1]).toEqual({ teamIds: ["rigel"] });
    expect(post.mock.calls[1][1].filters).toEqual({ teamIds: ["group-rigel"] });
  });

  it.each([
    ["countOperations", "/change-requests/aggregate"],
    ["searchOperations", "/change-requests/search"],
  ])("%s calls the kind's own %s", async (fn, path) => {
    post.mockReset().mockImplementation(answer);
    const { result } = renderHook(() => useCodeDashboardPortal());
    await result.current[fn]({ kind: "change_request", states: ["new"] });
    expect(post.mock.calls.map((c) => c[0])).toEqual([path]);
    expect(post.mock.calls[0][1].filters).toEqual({ states: ["new"] });
  });

  it.each([
    ["searchAccounts", "/accounts/search", { team: "rigel" }],
    ["searchProjects", "/projects/search", { onboardingStatus: ["In-Progress"] }],
    ["countFeedback", "/cases/feedback/aggregate", { groupBy: "rating" }],
    ["searchFeedback", "/cases/feedback/search", { rating: 1 }],
    ["countPlg", "/plg/organizations/aggregate", { kind: "organization", groupBy: "owner" }],
    ["searchPlg", "/plg/registrations/search", { kind: "registration" }],
    ["plgTrend", "/plg/registrations/volume-trend", { days: 7 }],
  ])("%s calls %s", async (fn, path, args) => {
    post.mockReset().mockImplementation(answer);
    const { result } = renderHook(() => useCodeDashboardPortal());
    await result.current[fn](args);
    expect(post.mock.calls.map((c) => c[0])).toEqual([path]);
  });

  it.each([
    ["countPlg", "/plg/registrations/aggregate", { kind: "registration", playbookTask: "OUTCOME", groupBy: "playbookAnswer" }],
    ["searchPlg", "/plg/organizations/search", { kind: "organization", playbookTask: "OUTCOME", playbookAnswers: ["EXCLUDED"] }],
    ["plgTrend", "/plg/registrations/volume-trend", { playbookTask: "OUTCOME", playbookUnanswered: true }],
  ])("%s checks its playbook codes before calling %s", async (fn, path, args) => {
    post.mockReset().mockImplementation(answer);
    const { result } = renderHook(() => useCodeDashboardPortal());
    await result.current[fn](args);
    expect(post.mock.calls.map((c) => c[0])).toEqual(["/dashboards/resolved-filters", path]);
    expect(post.mock.calls[0][1].playbookTask).toBe("OUTCOME");
  });

  it("searchCustomerHealth sends the product's and team's own names to /customer-health/summary", async () => {
    post.mockReset().mockResolvedValue({ data: [], totalCount: 0 });
    get.mockReset().mockImplementation((path: string) =>
      Promise.resolve(path === "/products" ? ["API Gateway", "Identity Hub"] : ["Apollo", "Rigel"]),
    );
    const { result } = renderHook(() => useCodeDashboardPortal());
    await expect(result.current.searchCustomerHealth({ product: "api gateway", team: "rigel" })).resolves.toEqual({
      total: 0,
      items: [],
    });
    expect(get.mock.calls.map((c) => c[0]).sort()).toEqual(["/abt-teams", "/products"]);
    expect(post.mock.calls).toEqual([
      ["/customer-health/summary", { product: "API Gateway", abtTeam: "Rigel", offset: 0, limit: 10 }],
    ]);
  });

  it("searchCustomerHealth reads no list when it names no product or team", async () => {
    post.mockReset().mockResolvedValue({ data: [], totalCount: 3 });
    get.mockReset();
    const { result } = renderHook(() => useCodeDashboardPortal());
    await expect(result.current.searchCustomerHealth({ limit: 1 })).resolves.toEqual({ total: 3, items: [] });
    expect(get).not.toHaveBeenCalled();
    expect(post.mock.calls).toEqual([["/customer-health/summary", { offset: 0, limit: 1 }]]);
  });

  it("searchCustomerHealth does not search for an unknown team", async () => {
    post.mockReset();
    get.mockReset().mockResolvedValue(["Apollo", "Rigel"]);
    const { result } = renderHook(() => useCodeDashboardPortal());
    await expect(result.current.searchCustomerHealth({ team: "Orion" })).rejects.toThrow(
      "searchCustomerHealth: no team is called Orion; the teams are Apollo, Rigel",
    );
    expect(get.mock.calls).toEqual([["/abt-teams"]]);
    expect(post).not.toHaveBeenCalled();
  });

  it("plgSummary reads GET /plg/analytics/dashboard with its window", async () => {
    post.mockReset();
    get.mockReset().mockResolvedValue({ summary: { pairingsNeedingAttention: 4 } });
    const { result } = renderHook(() => useCodeDashboardPortal());
    await expect(result.current.plgSummary({ startDate: "2026-10-01" })).resolves.toMatchObject({ needingAttention: 4 });
    expect(get.mock.calls).toEqual([["/plg/analytics/dashboard?from=2026-10-01"]]);
    expect(post).not.toHaveBeenCalled();
  });

  it("plgSummary reads no answer as zeros", async () => {
    get.mockReset().mockResolvedValue(null);
    const { result } = renderHook(() => useCodeDashboardPortal());
    await expect(result.current.plgSummary({})).resolves.toMatchObject({ registrations: 0, byProduct: [] });
  });

  it.each([
    ["searchRota", "/team-schedule/assignments/search", { shift: "CRE_MORNING" }],
    ["searchAbsences", "/team-schedule/absences/search", { kind: "RND" }],
  ])("%s calls %s with the window and reads its own records", async (fn, path, item) => {
    post.mockReset().mockResolvedValue({ assignments: [{ shiftCode: "CRE_MORNING" }], absences: [{ kindCode: "RND" }], count: 1 });
    const { result } = renderHook(() => useCodeDashboardPortal());
    const res = (await result.current[fn]({ startDate: "2026-11-16", endDate: "2026-11-22", teams: ["rigel"] })) as {
      total: number;
      items: unknown[];
    };
    expect(res.total).toBe(1);
    expect(res.items).toEqual([expect.objectContaining(item)]);
    expect(post.mock.calls).toEqual([[path, { from: "2026-11-16", to: "2026-11-22", teamKeys: ["rigel"] }]]);
  });

  it("caseResolutionTime sends the window and grouping and reads the answer", async () => {
    post.mockReset().mockResolvedValue({
      resolvedFrom: "2026-07-01",
      resolvedTo: "2026-09-30",
      total: { count: 2, measured: 1, averageHours: 3 },
      groups: [{ key: "high", label: "high", count: 2, measured: 1, averageHours: 3 }],
    });
    const { result } = renderHook(() => useCodeDashboardPortal());
    await expect(
      result.current.caseResolutionTime({ startDate: "2026-07-01", endDate: "2026-09-30", groupBy: "severity" }),
    ).resolves.toEqual({
      startDate: "2026-07-01",
      endDate: "2026-09-30",
      count: 2,
      measured: 1,
      averageHours: 3,
      groups: [{ label: "high", count: 2, measured: 1, averageHours: 3 }],
    });
    expect(post.mock.calls).toEqual([
      [
        "/cases/resolution-time",
        { filters: { filters: [] }, resolvedFrom: "2026-07-01", resolvedTo: "2026-09-30", groupBy: "severity" },
      ],
    ]);
  });

  it("does not call a PLG search when its playbook codes fail the check", async () => {
    post.mockReset().mockRejectedValue(new Error('Unknown playbook task "OUTCOME"'));
    const { result } = renderHook(() => useCodeDashboardPortal());
    await expect(result.current.countPlg({ kind: "registration", playbookTask: "OUTCOME" })).rejects.toThrow(/Unknown playbook task/);
    expect(post.mock.calls.map((c) => c[0])).toEqual(["/dashboards/resolved-filters"]);
  });

  it("refuses the viewer's own time cards before the viewer is loaded", async () => {
    post.mockReset();
    const { result } = renderHook(() => useCodeDashboardPortal());
    await expect(result.current.countTimeCards({ mine: true })).rejects.toThrow(/not loaded/);
    expect(post).not.toHaveBeenCalled();
  });

  it("calls the case search directly when nothing is named", async () => {
    post.mockReset().mockImplementation(answer);
    const { result } = renderHook(() => useCodeDashboardPortal());
    await result.current.countCases({ filters: myOpenCases });
    expect(post.mock.calls.map((c) => c[0])).toEqual(["/cases/aggregate"]);
  });
});
