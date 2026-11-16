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

import { useCaseVolumeTrendData } from "@features/csm-dashboard/api/useCaseVolumeTrendData";

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

describe("useCaseVolumeTrendData", () => {
  beforeEach(() => {
    postMock.mockReset();
  });

  it("issues a single request and zips the two series into one per-date point array", async () => {
    postMock.mockResolvedValue({
      days: 2,
      series: [
        {
          label: "Opened",
          points: [
            { date: "2026-09-01", count: 3 },
            { date: "2026-09-02", count: 1 },
          ],
        },
        {
          label: "Closed",
          points: [
            { date: "2026-09-01", count: 0 },
            { date: "2026-09-02", count: 2 },
          ],
        },
      ],
    });

    const { result } = renderHook(
      () => useCaseVolumeTrendData("case_volume_trend", { filters: [] }, 2),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    expect(postMock).toHaveBeenCalledTimes(1);
    expect(postMock).toHaveBeenCalledWith(
      "/cases/volume-trend",
      { filters: { filters: [] }, days: 2 },
      { signal: expect.any(AbortSignal) },
    );

    expect(result.current.points).toEqual([
      { date: "2026-09-01", opened: 3, closed: 0 },
      { date: "2026-09-02", opened: 1, closed: 2 },
    ]);
    expect(result.current.isError).toBe(false);
  });

  it("defaults days to 30 when the widget carries none", async () => {
    postMock.mockResolvedValue({ days: 30, series: [] });

    const { result } = renderHook(
      () => useCaseVolumeTrendData("case_volume_trend", {}, undefined),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    expect(postMock).toHaveBeenCalledWith(
      "/cases/volume-trend",
      { filters: {}, days: 30 },
      { signal: expect.any(AbortSignal) },
    );
    expect(result.current.points).toEqual([]);
  });

  it("sends a fixed window instead of days, and keeps each week's last day", async () => {
    postMock.mockResolvedValue({
      days: 31,
      startDate: "2025-12-01",
      endDate: "2025-12-31",
      bucket: "week",
      series: [
        {
          label: "Opened",
          points: [
            { date: "2025-12-01", endDate: "2025-12-07", count: 2 },
            { date: "2025-12-29", endDate: "2025-12-31", count: 1 },
          ],
        },
        {
          label: "Closed",
          points: [
            { date: "2025-12-01", endDate: "2025-12-07", count: 0 },
            { date: "2025-12-29", endDate: "2025-12-31", count: 3 },
          ],
        },
      ],
    });

    const { result } = renderHook(
      () =>
        useCaseVolumeTrendData("dec_2025", {}, undefined, undefined, undefined, undefined, true, {
          startDate: "2025-12-01",
          endDate: "2025-12-31",
          bucket: "week",
        }),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    expect(postMock).toHaveBeenCalledWith(
      "/cases/volume-trend",
      { filters: {}, startDate: "2025-12-01", endDate: "2025-12-31", bucket: "week" },
      { signal: expect.any(AbortSignal) },
    );
    expect(result.current.points).toEqual([
      { date: "2025-12-01", endDate: "2025-12-07", opened: 2, closed: 0 },
      { date: "2025-12-29", endDate: "2025-12-31", opened: 1, closed: 3 },
    ]);
  });

  it("sends a relative window as UTC calendar days", async () => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date(Date.UTC(2026, 10, 6, 9, 0, 0)));
    try {
      postMock.mockResolvedValue({ days: 365, series: [] });

      const { result } = renderHook(
        () =>
          useCaseVolumeTrendData("this_year", {}, undefined, undefined, undefined, undefined, true, {
            startDate: "__startOfYear:0__",
            endDate: "__endOfYear:0__",
            bucket: "month",
          }),
        { wrapper },
      );

      await waitFor(() => expect(result.current.isLoading).toBe(false));

      expect(postMock).toHaveBeenCalledWith(
        "/cases/volume-trend",
        { filters: {}, startDate: "2026-01-01", endDate: "2026-12-31", bucket: "month" },
        { signal: expect.any(AbortSignal) },
      );
    } finally {
      vi.useRealTimers();
    }
  });

  // rksk's two cases: the window's days are the service's UTC days, whatever
  // the viewer's own date is.
  it.each([
    // Sunday 18:00 in UTC-8 is already Monday 02:00 UTC: the new week.
    ["America/Los_Angeles", Date.UTC(2026, 10, 9, 2, 0, 0), "2026-11-09", "2026-11-15"],
    // Monday 02:00 in UTC+5:30 is still Sunday 20:30 UTC: the old week.
    ["Asia/Colombo", Date.UTC(2026, 10, 8, 20, 30, 0), "2026-11-02", "2026-11-08"],
  ])("sends this week by UTC days for a viewer in %s", async (timeZone, instant, start, end) => {
    vi.stubEnv("TZ", timeZone);
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date(instant));
    try {
      postMock.mockResolvedValue({ days: 7, series: [] });

      const { result } = renderHook(
        () =>
          useCaseVolumeTrendData("this_week", {}, undefined, undefined, undefined, undefined, true, {
            startDate: "__startOfWeek:0__",
            endDate: "__endOfWeek:0__",
          }),
        { wrapper },
      );

      await waitFor(() => expect(result.current.isLoading).toBe(false));

      expect(postMock).toHaveBeenCalledWith(
        "/cases/volume-trend",
        { filters: {}, startDate: start, endDate: end },
        { signal: expect.any(AbortSignal) },
      );
    } finally {
      vi.useRealTimers();
      vi.unstubAllEnvs();
    }
  });

  it("holds the request while a __current_user__ placeholder hasn't resolved yet", () => {
    const { result } = renderHook(
      () =>
        useCaseVolumeTrendData(
          "case_volume_trend",
          { filters: [{ field: "assignee", op: "eq", value: "__current_user__" }] },
          30,
          undefined,
          undefined,
          undefined,
        ),
      { wrapper },
    );

    expect(result.current.isLoading).toBe(true);
    expect(postMock).not.toHaveBeenCalled();
  });

  it("reports an error when the request fails", async () => {
    postMock.mockRejectedValue(new Error("boom"));

    const { result } = renderHook(
      () => useCaseVolumeTrendData("case_volume_trend", {}, 7),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    expect(result.current.isError).toBe(true);
    expect(result.current.points).toEqual([]);
  });

  it("does not fire when enabled is false", () => {
    const { result } = renderHook(
      () => useCaseVolumeTrendData("case_volume_trend", {}, 30, undefined, undefined, undefined, false),
      { wrapper },
    );

    expect(result.current.isLoading).toBe(true);
    expect(postMock).not.toHaveBeenCalled();
  });
});
