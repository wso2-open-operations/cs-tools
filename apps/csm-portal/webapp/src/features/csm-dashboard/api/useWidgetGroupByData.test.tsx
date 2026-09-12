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
        query: { filters: [{ field: "severity", op: "eq", values: ["critical"] }] },
        value: 3,
      },
      {
        label: "High",
        query: { filters: [{ field: "severity", op: "eq", values: ["high"] }] },
        value: 5,
      },
      { label: "Others", query: {}, navigable: false, value: 2 },
    ]);
    expect(result.current.total).toBe(10);
  });

  it("scopes a non-case resourceType's named bucket to a flat top-level key", async () => {
    postMock.mockResolvedValue({
      groups: [{ key: "P1", label: "P1", count: 4 }],
      othersCount: 0,
      totalRecords: 4,
    });

    const { result } = renderHook(
      () => useWidgetGroupByData("widget-1", "incident", {}, { field: "priority" }),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    expect(result.current.slices).toEqual([{ label: "P1", query: { priority: "P1" }, value: 4 }]);
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
        query: { filters: [{ field: "severity", op: "eq", values: ["critical"] }] },
        value: 3,
      },
      { label: "Everything else", query: {}, navigable: false, value: 4 },
    ]);
    expect(result.current.total).toBe(7);
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
        query: { filters: [{ field: "severity", op: "eq", values: ["critical"] }] },
        value: 3,
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
});
