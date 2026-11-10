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
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";

const postMock = vi.fn();

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
}));

import { usePlgRegistrationTrendData } from "@features/csm-dashboard/api/usePlgRegistrationTrendData";

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

describe("usePlgRegistrationTrendData", () => {
  beforeEach(() => {
    postMock.mockReset();
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it("asks the PLG trend once, with the registration search's filters, and returns Registered as the opened line", async () => {
    postMock.mockResolvedValue({
      days: 74,
      startDate: "2026-01-01",
      endDate: "2026-03-15",
      bucket: "month",
      series: [
        {
          label: "Registered",
          points: [
            { date: "2026-01-01", endDate: "2026-01-31", count: 3 },
            { date: "2026-02-01", endDate: "2026-02-28", count: 4 },
          ],
        },
      ],
    });

    const { result } = renderHook(
      () =>
        usePlgRegistrationTrendData(
          "signups",
          { productCodes: ["IAM"], unacknowledgedOnly: true, registeredFrom: "2026-01-10" },
          undefined,
          undefined,
          undefined,
          undefined,
          true,
          { startDate: "2026-01-01", endDate: "2026-03-15", bucket: "month" },
        ),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(postMock).toHaveBeenCalledTimes(1);
    expect(postMock).toHaveBeenCalledWith(
      "/plg/registrations/volume-trend",
      {
        filters: { productCodes: ["IAM"], registeredFrom: "2026-01-10T00:00:00Z" },
        unacknowledgedOnly: true,
        startDate: "2026-01-01",
        endDate: "2026-03-15",
        bucket: "month",
      },
      { signal: expect.any(AbortSignal) },
    );
    expect(result.current.points).toEqual([
      { date: "2026-01-01", endDate: "2026-01-31", opened: 3, closed: 0 },
      { date: "2026-02-01", endDate: "2026-02-28", opened: 4, closed: 0 },
    ]);
    expect(result.current.isError).toBe(false);
  });

  it("defaults to the trailing 30 days", async () => {
    postMock.mockResolvedValue({ days: 30, series: [{ label: "Registered", points: [] }] });

    const { result } = renderHook(() => usePlgRegistrationTrendData("signups", {}, undefined), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(postMock).toHaveBeenCalledWith(
      "/plg/registrations/volume-trend",
      { filters: {}, days: 30 },
      { signal: expect.any(AbortSignal) },
    );
    expect(result.current.points).toEqual([]);
  });

  it("resolves a relative registration bound and a relative window before asking", async () => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date(2026, 2, 15, 12, 0, 0));
    postMock.mockResolvedValue({ days: 74, series: [{ label: "Registered", points: [] }] });

    const { result } = renderHook(
      () =>
        usePlgRegistrationTrendData(
          "signups",
          { registeredFrom: "__startOfMonth:-1__" },
          undefined,
          undefined,
          undefined,
          undefined,
          true,
          { startDate: "__startOfYear:0__", endDate: "__today__", bucket: "month" },
        ),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(postMock).toHaveBeenCalledWith(
      "/plg/registrations/volume-trend",
      {
        filters: { registeredFrom: "2026-02-01T00:00:00Z" },
        startDate: "2026-01-01",
        endDate: "2026-03-15",
        bucket: "month",
      },
      { signal: expect.any(AbortSignal) },
    );
  });

  it("asks nothing while disabled", () => {
    const { result } = renderHook(
      () => usePlgRegistrationTrendData("signups", {}, 30, undefined, undefined, undefined, false),
      { wrapper },
    );
    expect(result.current.isLoading).toBe(true);
    expect(postMock).not.toHaveBeenCalled();
  });

  it("waits for the signed-in user before a __current_user__ owner filter is sent, then sends their id", async () => {
    postMock.mockResolvedValue({ days: 30, series: [{ label: "Registered", points: [] }] });
    const { result, rerender } = renderHook(
      ({ userId }: { userId?: string }) =>
        usePlgRegistrationTrendData("mine", { ownerIds: ["__current_user__"] }, 30, undefined, undefined, userId),
      { wrapper, initialProps: {} as { userId?: string } },
    );
    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(result.current.isLoading).toBe(true);
    expect(postMock).not.toHaveBeenCalled();

    rerender({ userId: "user-1" });
    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(postMock).toHaveBeenCalledWith(
      "/plg/registrations/volume-trend",
      { filters: { ownerIds: ["user-1"] }, days: 30 },
      { signal: expect.any(AbortSignal) },
    );
  });

  it("reports a failed request as an error", async () => {
    postMock.mockRejectedValue(new Error("boom"));

    const { result } = renderHook(() => usePlgRegistrationTrendData("signups", {}, 30), { wrapper });

    await waitFor(() => expect(result.current.isError).toBe(true), { timeout: 5000 });
    expect(result.current.points).toEqual([]);
  });
});
