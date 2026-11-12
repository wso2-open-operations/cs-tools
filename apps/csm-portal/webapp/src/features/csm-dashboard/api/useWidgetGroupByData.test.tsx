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

import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it, vi, beforeEach } from "vitest";
import type { ReactNode } from "react";

const postMock = vi.fn();

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
}));

import { useWidgetGroupByData } from "@features/csm-dashboard/api/useWidgetGroupByData";
import { CURRENT_TEAM_PLACEHOLDER } from "@features/csm-dashboard/utils/teamFilterPlaceholder";
import { CURRENT_USER_PLACEHOLDER } from "@features/csm-dashboard/utils/currentUserFilterPlaceholder";

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

describe("useWidgetGroupByData", () => {
  beforeEach(() => {
    postMock.mockReset();
  });

  it("issues a single group-by request and maps buckets + a synthetic Others bucket into slices", async () => {
    postMock.mockResolvedValue({
      groups: [
        { key: "critical", label: "Critical", count: 3 },
        { key: "high", label: "High", count: 5 },
      ],
      othersCount: 2,
      totalRecords: 10,
    });

    const { result } = renderHook(
      () =>
        useWidgetGroupByData("widget-1", "case", { states: ["open"] }, {
          field: "severity",
          maxGroups: 2,
        }),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    expect(postMock).toHaveBeenCalledTimes(1);
    expect(postMock).toHaveBeenCalledWith(
      "/cases/aggregate",
      {
        filters: { states: ["open"] },
        groupBy: "severity",
        maxGroups: 2,
      },
      { signal: expect.any(AbortSignal) },
    );

    expect(result.current.slices).toEqual([
      {
        label: "Critical",
        query: { filters: [{ field: "severity", op: "in", values: ["critical"] }] },
        value: 3,
        linkId: "critical",
      },
      {
        label: "High",
        query: { filters: [{ field: "severity", op: "in", values: ["high"] }] },
        value: 5,
        linkId: "high",
      },
      { label: "Others", query: {}, navigable: false, value: 2 },
    ]);
    expect(result.current.total).toBe(10);
  });

  it("scopes a resourceType with no case-DSL/filter-array/named-array field (e.g. incident_task) to a flat top-level key", async () => {
    postMock.mockResolvedValue({
      groups: [{ key: "P1", label: "P1", count: 4 }],
      othersCount: 0,
      totalRecords: 4,
    });

    const { result } = renderHook(
      () => useWidgetGroupByData("widget-1", "incident_task", {}, { field: "priority" }),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    expect(result.current.slices).toEqual([
      { label: "P1", query: { priority: "P1" }, value: 4, linkId: "P1" },
    ]);
  });

  it("scopes an incident's state bucket to the nested filter-array shape with op 'in', not a flat key", async () => {
    postMock.mockResolvedValue({
      groups: [{ key: "NEW", label: "New", count: 4 }],
      othersCount: 0,
      totalRecords: 4,
    });

    const { result } = renderHook(
      () => useWidgetGroupByData("widget-1", "incident", {}, { field: "state" }),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    expect(result.current.slices).toEqual([
      {
        label: "New",
        query: { filters: [{ field: "state", op: "in", values: ["NEW"] }] },
        value: 4,
        linkId: "NEW",
      },
    ]);
  });

  it("scopes a problem's state bucket to the nested filter-array shape with op 'in', not a flat key", async () => {
    postMock.mockResolvedValue({
      groups: [{ key: "NEW", label: "New", count: 2 }],
      othersCount: 0,
      totalRecords: 2,
    });

    const { result } = renderHook(
      () => useWidgetGroupByData("widget-1", "problem", {}, { field: "state" }),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    expect(result.current.slices).toEqual([
      {
        label: "New",
        query: { filters: [{ field: "state", op: "in", values: ["NEW"] }] },
        value: 2,
        linkId: "NEW",
      },
    ]);
  });

  it("scopes a change_request's state bucket to the bespoke plural 'states' top-level array field", async () => {
    postMock.mockResolvedValue({
      groups: [{ key: "-5", label: "New", count: 6 }],
      othersCount: 0,
      totalRecords: 6,
    });

    const { result } = renderHook(
      () => useWidgetGroupByData("widget-1", "change_request", {}, { field: "state" }),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    expect(result.current.slices).toEqual([
      { label: "New", query: { states: ["-5"] }, value: 6, linkId: "-5" },
    ]);
  });

  it("marks the synthetic Others bucket non-navigable rather than giving it an unscoped query", async () => {
    postMock.mockResolvedValue({
      groups: [{ key: "critical", label: "Critical", count: 3 }],
      othersCount: 2,
      totalRecords: 5,
    });

    const { result } = renderHook(
      () => useWidgetGroupByData("widget-1", "case", {}, { field: "severity" }),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    const others = result.current.slices.find((s) => s.label === "Others");
    expect(others).toEqual({ label: "Others", query: {}, navigable: false, value: 2 });
  });

  it("uses the configured othersLabel instead of the default 'Others'", async () => {
    postMock.mockResolvedValue({
      groups: [{ key: "critical", label: "Critical", count: 3 }],
      othersCount: 4,
      totalRecords: 7,
    });

    const { result } = renderHook(
      () =>
        useWidgetGroupByData("widget-1", "case", {}, {
          field: "severity",
          othersLabel: "Everything else",
        }),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    expect(result.current.slices).toEqual([
      {
        label: "Critical",
        query: { filters: [{ field: "severity", op: "in", values: ["critical"] }] },
        value: 3,
        linkId: "critical",
      },
      { label: "Everything else", query: {}, navigable: false, value: 4 },
    ]);
    expect(result.current.total).toBe(7);
  });

  it("appends no synthetic bucket when hideOthers is set, even with a non-zero othersCount", async () => {
    postMock.mockResolvedValue({
      groups: [{ key: "acme", label: "Acme Corp", count: 10 }],
      othersCount: 4,
      totalRecords: 14,
    });

    const { result } = renderHook(
      () =>
        useWidgetGroupByData("widget-1", "case", {}, {
          field: "account",
          maxGroups: 1,
          hideOthers: true,
        }),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    expect(result.current.slices).toEqual([
      {
        label: "Acme Corp",
        query: { filters: [{ field: "account", op: "in", values: ["acme"] }] },
        value: 10,
        linkId: "acme",
      },
    ]);
    // total is derived from the slices actually rendered, not the response's
    // own totalRecords -- hiding Others must also drop it from the widget's
    // own displayed total, not just from the chart.
    expect(result.current.total).toBe(10);
  });

  it("appends no synthetic bucket when othersCount is 0", async () => {
    postMock.mockResolvedValue({
      groups: [{ key: "critical", label: "Critical", count: 3 }],
      othersCount: 0,
      totalRecords: 3,
    });

    const { result } = renderHook(
      () => useWidgetGroupByData("widget-1", "case", {}, { field: "severity" }),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    expect(result.current.slices).toEqual([
      {
        label: "Critical",
        query: { filters: [{ field: "severity", op: "in", values: ["critical"] }] },
        value: 3,
        linkId: "critical",
      },
    ]);
    expect(result.current.total).toBe(3);
  });

  it("fires no query and returns a zero result when groupBy is undefined", () => {
    const { result } = renderHook(
      () => useWidgetGroupByData("widget-1", "case", { states: ["open"] }, undefined),
      { wrapper },
    );

    expect(postMock).not.toHaveBeenCalled();
    expect(result.current.slices).toEqual([]);
    expect(result.current.total).toBe(0);
    expect(result.current.isLoading).toBe(false);
    expect(result.current.isError).toBe(false);
  });

  it("resolves __current_team__ in the base filters using the selected team's creGroupId", async () => {
    postMock.mockResolvedValue({ groups: [], othersCount: 0, totalRecords: 0 });

    renderHook(
      () =>
        useWidgetGroupByData(
          "widget-1",
          "case",
          { filters: [{ field: "creTeam", op: "in", values: [CURRENT_TEAM_PLACEHOLDER] }] },
          { field: "severity" },
          "22222222-2222-2222-2222-222222222222",
          undefined,
        ),
      { wrapper },
    );

    await waitFor(() => expect(postMock).toHaveBeenCalled());

    expect(postMock).toHaveBeenCalledWith(
      "/cases/aggregate",
      {
        filters: {
          filters: [
            {
              field: "creTeam",
              op: "in",
              values: ["22222222-2222-2222-2222-222222222222"],
            },
          ],
        },
        groupBy: "severity",
        maxGroups: undefined,
      },
      { signal: expect.any(AbortSignal) },
    );
  });

  it("holds the request while a __current_user__ placeholder is unresolved, rather than sending it literally", async () => {
    postMock.mockResolvedValue({ groups: [], othersCount: 0, totalRecords: 0 });

    const { result } = renderHook(
      () =>
        useWidgetGroupByData(
          "widget-1",
          "case",
          { filters: [{ field: "assignedUserId", op: "in", values: [CURRENT_USER_PLACEHOLDER] }] },
          { field: "severity" },
          undefined,
          undefined,
          undefined,
        ),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isLoading).toBe(true));
    expect(postMock).not.toHaveBeenCalled();
  });

  it("surfaces isError when the group-by request fails", async () => {
    postMock.mockRejectedValue(new Error("boom"));

    const { result } = renderHook(
      () => useWidgetGroupByData("widget-1", "case", {}, { field: "severity" }),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isError).toBe(true));
  });

  describe("time card totals", () => {
    const hoursPerUser = {
      groups: [
        { key: "u-bob", label: "Bob", count: 2, totalMinutes: 255 },
        { key: "u-ann", label: "Ann", count: 4, totalMinutes: 135 },
      ],
      othersCount: 3,
      totalRecords: 9,
      othersMinutes: 90,
      totalMinutes: 480,
    };

    it("posts to /time-cards/aggregate with measure minutes and shows hours, Others included", async () => {
      postMock.mockResolvedValue(hoursPerUser);

      const { result } = renderHook(
        () =>
          useWidgetGroupByData("hours", "time_card", { startDate: "2026-10-01", endDate: "2026-10-31" }, {
            field: "user",
            measure: "hours",
            maxGroups: 2,
          }),
        { wrapper },
      );

      await waitFor(() => expect(result.current.isLoading).toBe(false));
      expect(postMock).toHaveBeenCalledWith(
        "/time-cards/aggregate",
        {
          filters: { startDate: "2026-10-01", endDate: "2026-10-31" },
          groupBy: "user",
          maxGroups: 2,
          measure: "minutes",
        },
        { signal: expect.any(AbortSignal) },
      );
      expect(result.current.slices.map((s) => [s.label, s.value])).toEqual([
        ["Bob", 4.25],
        ["Ann", 2.25],
        ["Others", 1.5],
      ]);
      expect(result.current.total).toBe(8);
    });

    it("sends measure count and shows card counts when the widget counts", async () => {
      postMock.mockResolvedValue(hoursPerUser);

      const { result } = renderHook(
        () => useWidgetGroupByData("cards", "time_card", {}, { field: "user", measure: "count" }),
        { wrapper },
      );

      await waitFor(() => expect(result.current.isLoading).toBe(false));
      expect(postMock.mock.calls[0][1]).toEqual({ filters: {}, groupBy: "user", maxGroups: undefined, measure: "count" });
      expect(result.current.slices.map((s) => s.value)).toEqual([2, 4, 3]);
      expect(result.current.total).toBe(9);
    });

    it("sends no measure and shows counts when the widget sets none", async () => {
      postMock.mockResolvedValue(hoursPerUser);

      const { result } = renderHook(
        () => useWidgetGroupByData("by-state", "time_card", {}, { field: "state" }),
        { wrapper },
      );

      await waitFor(() => expect(result.current.isLoading).toBe(false));
      expect(postMock.mock.calls[0][1]).not.toHaveProperty("measure");
      expect(result.current.slices.map((s) => s.value)).toEqual([2, 4, 3]);
    });

    it("keeps a day grouping in the aggregate's date order, empty days at zero, labelled as short dates", async () => {
      postMock.mockResolvedValue({
        groups: [
          { key: "2026-10-30", label: "2026-10-30", count: 1, totalMinutes: 30 },
          { key: "2026-10-31", label: "2026-10-31", count: 0, totalMinutes: 0 },
          { key: "2026-11-01", label: "2026-11-01", count: 3, totalMinutes: 300 },
        ],
        othersCount: 0,
        totalRecords: 4,
        othersMinutes: 0,
        totalMinutes: 330,
      });

      const { result } = renderHook(
        () =>
          useWidgetGroupByData("per-day", "time_card", { startDate: "2026-10-30", endDate: "2026-11-01" }, {
            field: "day",
            measure: "hours",
          }),
        { wrapper },
      );

      await waitFor(() => expect(result.current.isLoading).toBe(false));
      expect(result.current.slices.map((s) => [s.label, s.value])).toEqual([
        ["Oct 30", 0.5],
        ["Oct 31", 0],
        ["Nov 1", 5],
      ]);
      // Each day opens as a one-day date range.
      expect(result.current.slices[2].query).toEqual({ startDate: "2026-11-01", endDate: "2026-11-01" });
    });

    it("formats only a day grouping's labels: a project named like a date stays as it is", async () => {
      postMock.mockResolvedValue({
        groups: [{ key: "p-1", label: "2026-10-30", count: 1, totalMinutes: 60 }],
        othersCount: 0,
        totalRecords: 1,
        othersMinutes: 0,
        totalMinutes: 60,
      });

      const { result } = renderHook(
        () => useWidgetGroupByData("per-project", "time_card", {}, { field: "project", measure: "hours" }),
        { wrapper },
      );

      await waitFor(() => expect(result.current.isLoading).toBe(false));
      expect(result.current.slices[0].label).toBe("2026-10-30");
    });

    it("resolves relative dates and the signed-in user before the request", async () => {
      postMock.mockResolvedValue({ groups: [], othersCount: 0, totalRecords: 0, othersMinutes: 0, totalMinutes: 0 });

      renderHook(
        () =>
          useWidgetGroupByData(
            "mine",
            "time_card",
            { userIds: [CURRENT_USER_PLACEHOLDER], startDate: "__daysAgo:6__", endDate: "__today__" },
            { field: "day", measure: "hours" },
            undefined,
            undefined,
            "user-123",
          ),
        { wrapper },
      );

      await waitFor(() => expect(postMock).toHaveBeenCalledTimes(1));
      const filters = postMock.mock.calls[0][1].filters as Record<string, unknown>;
      expect(filters.userIds).toEqual(["user-123"]);
      expect(filters.startDate).toMatch(/^\d{4}-\d{2}-\d{2}$/);
      expect(filters.endDate).toMatch(/^\d{4}-\d{2}-\d{2}$/);
    });
  });
});
