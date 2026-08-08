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

import { useWidgetPieData } from "@features/csm-dashboard/api/useWidgetPieData";
import { CURRENT_TEAM_PLACEHOLDER } from "@features/csm-dashboard/utils/teamFilterPlaceholder";
import { CURRENT_USER_PLACEHOLDER } from "@features/csm-dashboard/utils/currentUserFilterPlaceholder";

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

describe("useWidgetPieData", () => {
  beforeEach(() => {
    postMock.mockReset();
  });

  it("issues one search per slice, own filters merged under the widget's base filters, limit 1", async () => {
    postMock.mockImplementation((_path: string, body: { filters: Record<string, unknown> }) => {
      if (body.filters.severities === "critical") return Promise.resolve({ total: 1 });
      if (body.filters.severities === "high") return Promise.resolve({ total: 3 });
      return Promise.resolve({ total: 0 });
    });

    const { result } = renderHook(
      () =>
        useWidgetPieData("widget-1", "case", { states: ["open"] }, [
          { label: "Critical", query: { severities: "critical" } },
          { label: "High", query: { severities: "high" } },
        ]),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    expect(postMock).toHaveBeenCalledTimes(2);
    expect(postMock).toHaveBeenCalledWith("/cases/search", {
      filters: { states: ["open"], severities: "critical" },
      pagination: { offset: 0, limit: 1 },
    });
    expect(postMock).toHaveBeenCalledWith("/cases/search", {
      filters: { states: ["open"], severities: "high" },
      pagination: { offset: 0, limit: 1 },
    });
    expect(result.current.slices).toEqual([
      { label: "Critical", query: { severities: "critical" }, value: 1 },
      { label: "High", query: { severities: "high" }, value: 3 },
    ]);
    expect(result.current.total).toBe(4);
  });

  it("fires no queries and returns a zero total for an empty slices array", () => {
    const { result } = renderHook(() => useWidgetPieData("widget-1", "case", {}, []), { wrapper });

    expect(postMock).not.toHaveBeenCalled();
    expect(result.current.slices).toEqual([]);
    expect(result.current.total).toBe(0);
    expect(result.current.isLoading).toBe(false);
  });

  it("resolves __current_team__ (in either the base or a slice's own filters) after merging, using the selected team's groupId", async () => {
    postMock.mockResolvedValue({ total: 1 });

    const { result } = renderHook(
      () =>
        useWidgetPieData(
          "widget-1",
          "case",
          { filters: [{ field: "state", op: "in", values: ["open"] }] },
          [
            {
              label: "My team",
              query: {
                filters: [
                  { field: "integrationCsTeam", op: "in", values: [CURRENT_TEAM_PLACEHOLDER] },
                ],
              },
            },
          ],
          "22222222-2222-2222-2222-222222222222",
        ),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    expect(postMock).toHaveBeenCalledWith("/cases/search", {
      filters: {
        filters: [
          { field: "state", op: "in", values: ["open"] },
          {
            field: "integrationCsTeam",
            op: "in",
            values: ["22222222-2222-2222-2222-222222222222"],
          },
        ],
      },
      pagination: { offset: 0, limit: 1 },
    });
  });

  it("drops the integrationCsTeam entry rather than sending the literal placeholder when no team groupId is selected", async () => {
    postMock.mockResolvedValue({ total: 1 });

    renderHook(
      () =>
        useWidgetPieData(
          "widget-1",
          "case",
          { filters: [{ field: "state", op: "in", values: ["open"] }] },
          [
            {
              label: "My team",
              query: {
                filters: [
                  { field: "integrationCsTeam", op: "in", values: [CURRENT_TEAM_PLACEHOLDER] },
                ],
              },
            },
          ],
          undefined,
        ),
      { wrapper },
    );

    await waitFor(() => expect(postMock).toHaveBeenCalled());

    expect(postMock).toHaveBeenCalledWith("/cases/search", {
      filters: {
        filters: [{ field: "state", op: "in", values: ["open"] }],
      },
      pagination: { offset: 0, limit: 1 },
    });
  });

  it("resolves __current_user__ (in either the base or a slice's own filters) after merging, using the signed-in user's own id", async () => {
    postMock.mockResolvedValue({ total: 1 });

    const { result } = renderHook(
      () =>
        useWidgetPieData(
          "widget-1",
          "case",
          { filters: [{ field: "state", op: "in", values: ["open"] }] },
          [
            {
              label: "Assigned to me",
              query: {
                filters: [
                  { field: "assignedUserId", op: "in", values: [CURRENT_USER_PLACEHOLDER] },
                ],
              },
            },
          ],
          undefined,
          "11111111-aaaa-bbbb-cccc-000000000001",
        ),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    expect(postMock).toHaveBeenCalledWith("/cases/search", {
      filters: {
        filters: [
          { field: "state", op: "in", values: ["open"] },
          {
            field: "assignedUserId",
            op: "in",
            values: ["11111111-aaaa-bbbb-cccc-000000000001"],
          },
        ],
      },
      pagination: { offset: 0, limit: 1 },
    });
  });

  it("drops the assignedUserId entry rather than sending the literal __current_user__ placeholder when the signed-in user isn't known yet", async () => {
    postMock.mockResolvedValue({ total: 1 });

    renderHook(
      () =>
        useWidgetPieData(
          "widget-1",
          "case",
          { filters: [{ field: "state", op: "in", values: ["open"] }] },
          [
            {
              label: "Assigned to me",
              query: {
                filters: [
                  { field: "assignedUserId", op: "in", values: [CURRENT_USER_PLACEHOLDER] },
                ],
              },
            },
          ],
          undefined,
          undefined,
        ),
      { wrapper },
    );

    await waitFor(() => expect(postMock).toHaveBeenCalled());

    expect(postMock).toHaveBeenCalledWith("/cases/search", {
      filters: {
        filters: [{ field: "state", op: "in", values: ["open"] }],
      },
      pagination: { offset: 0, limit: 1 },
    });
  });

  it("surfaces isError when any one slice's search fails", async () => {
    postMock
      .mockResolvedValueOnce({ total: 2 })
      .mockRejectedValueOnce(new Error("boom"));

    const { result } = renderHook(
      () =>
        useWidgetPieData("widget-1", "case", {}, [
          { label: "A", query: { a: "1" } },
          { label: "B", query: { b: "2" } },
        ]),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isError).toBe(true));
  });
});
