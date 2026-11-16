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

const getResponseMock = vi.fn();

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ getResponse: getResponseMock }),
}));

import { useMttrReportData } from "@features/csm-dashboard/api/useMttrReportData";

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

function jsonResponse(body: unknown): Response {
  return { json: () => Promise.resolve(body) } as unknown as Response;
}

describe("useMttrReportData", () => {
  beforeEach(() => {
    getResponseMock.mockReset();
  });

  it("issues a single GET /reports/mttr request, forwarding the slot's abort signal, and maps bands into ordered, non-navigable avg-hours slices", async () => {
    getResponseMock.mockResolvedValue(
      jsonResponse({
        generatedOn: "2026-11-16T00:00:00Z",
        windowMonths: 3,
        windowStart: "2026-08-16T00:00:00Z",
        windowEnd: "2026-11-16T00:00:00Z",
        creTeamIds: [],
        // Deliberately out of display order, to prove the hook sorts them.
        bands: [
          { band: "S4", caseCount: 10, avgResolutionHours: 40 },
          { band: "S1", caseCount: 2, avgResolutionHours: 5 },
          { band: "S1-S4", caseCount: 20, avgResolutionHours: 25 },
          { band: "S1-S3", caseCount: 10, avgResolutionHours: 15 },
          { band: "S2", caseCount: 3, avgResolutionHours: 10 },
          { band: "S3", caseCount: 5, avgResolutionHours: 20 },
        ],
      }),
    );

    const { result } = renderHook(
      () =>
        useMttrReportData(
          "mttr_widget",
          { months: "3" },
          { bucket: "severity" },
          undefined,
          undefined,
        ),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    expect(getResponseMock).toHaveBeenCalledTimes(1);
    expect(getResponseMock).toHaveBeenCalledWith("/reports/mttr?months=3", {
      signal: expect.any(AbortSignal),
    });

    expect(result.current.slices).toEqual([
      { label: "S1-S4", query: {}, navigable: false, value: 25 },
      { label: "S1-S3", query: {}, navigable: false, value: 15 },
      { label: "S1", query: {}, navigable: false, value: 5 },
      { label: "S2", query: {}, navigable: false, value: 10 },
      { label: "S3", query: {}, navigable: false, value: 20 },
      { label: "S4", query: {}, navigable: false, value: 40 },
    ]);
    expect(result.current.total).toBe(20);
  });

  it("shows only the requested bands, in display order, without changing the request", async () => {
    getResponseMock.mockResolvedValue(
      jsonResponse({
        generatedOn: "2026-11-16T00:00:00Z",
        windowMonths: 3,
        windowStart: "2026-08-16T00:00:00Z",
        windowEnd: "2026-11-16T00:00:00Z",
        creTeamIds: [],
        bands: [
          { band: "S1-S4", caseCount: 20, avgResolutionHours: 25 },
          { band: "S1-S3", caseCount: 10, avgResolutionHours: 15 },
          { band: "S1", caseCount: 2, avgResolutionHours: 5 },
          { band: "S2", caseCount: 3, avgResolutionHours: 10 },
          { band: "S3", caseCount: 5, avgResolutionHours: 20 },
          { band: "S4", caseCount: 10, avgResolutionHours: 40 },
        ],
      }),
    );
    const { result } = renderHook(
      () =>
        useMttrReportData(
          "mttr_bands",
          { months: "3", bands: ["S4", "S1", "S2", "S3"] },
          { bucket: "severity" },
          undefined,
          undefined,
        ),
      { wrapper },
    );
    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(getResponseMock).toHaveBeenCalledWith("/reports/mttr?months=3", {
      signal: expect.any(AbortSignal),
    });
    expect(result.current.slices.map((s) => s.label)).toEqual(["S1", "S2", "S3", "S4"]);
    expect(result.current.total).toBe(20);
  });

  it("sends days instead of months for a 7 day window and orders S0 first of the single bands", async () => {
    getResponseMock.mockResolvedValue(
      jsonResponse({
        generatedOn: "2026-11-16T00:00:00Z",
        windowMonths: 0,
        windowDays: 7,
        windowStart: "2026-11-09T00:00:00Z",
        windowEnd: "2026-11-16T00:00:00Z",
        creTeamIds: [],
        bands: [
          { band: "S1", caseCount: 2, avgResolutionHours: 5 },
          { band: "S0", caseCount: 1, avgResolutionHours: 3 },
          { band: "S1-S4", caseCount: 3, avgResolutionHours: 4 },
        ],
      }),
    );
    const { result } = renderHook(
      () =>
        useMttrReportData(
          "mttr_days",
          { days: "7", bands: ["S1", "S0"] },
          { bucket: "severity" },
          undefined,
          undefined,
        ),
      { wrapper },
    );
    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(getResponseMock).toHaveBeenCalledWith("/reports/mttr?days=7", {
      signal: expect.any(AbortSignal),
    });
    expect(result.current.slices.map((s) => s.label)).toEqual(["S0", "S1"]);
    // The header counts the cases behind the bars shown (S0 1 + S1 2), not
    // the S1-S4 band that is not shown.
    expect(result.current.total).toBe(3);
  });

  it("appends a repeated creTeamId per team", async () => {
    getResponseMock.mockResolvedValue(
      jsonResponse({
        generatedOn: "2026-11-16T00:00:00Z",
        windowMonths: 6,
        windowStart: "2026-05-16T00:00:00Z",
        windowEnd: "2026-11-16T00:00:00Z",
        creTeamIds: ["11111111-1111-1111-1111-111111111111"],
        bands: [],
      }),
    );

    renderHook(
      () =>
        useMttrReportData(
          "mttr_widget",
          { months: "6", creTeamIds: ["11111111-1111-1111-1111-111111111111"] },
          { bucket: "severity" },
          undefined,
          undefined,
        ),
      { wrapper },
    );

    await waitFor(() => expect(getResponseMock).toHaveBeenCalled());
    expect(getResponseMock).toHaveBeenCalledWith(
      "/reports/mttr?months=6&creTeamId=11111111-1111-1111-1111-111111111111",
      { signal: expect.any(AbortSignal) },
    );
  });

  it("fires no request without groupBy.bucket or months", async () => {
    const { result } = renderHook(
      () => useMttrReportData("mttr_widget", {}, undefined, undefined, undefined),
      { wrapper },
    );

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(getResponseMock).not.toHaveBeenCalled();
    expect(result.current.slices).toEqual([]);
    expect(result.current.total).toBe(0);
  });
});
