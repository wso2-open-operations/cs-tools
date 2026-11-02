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

import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { ApiQueryKeys } from "@constants/apiConstants";

const getMock = vi.fn();
const putMock = vi.fn();
const delMock = vi.fn();

// The real client reads runtime config at module load, which isn't present
// under vitest.
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ get: getMock, put: putMock, del: delMock }),
}));

import {
  useDeleteCustomDashboardShare,
  useGetCustomDashboardShares,
  useGetSharedDashboard,
  useGetSharedDashboards,
  usePutCustomDashboardShare,
  type DashboardShare,
} from "@features/csm-custom-dashboards/api/useDashboardShares";

const DASHBOARD = "5e636a12-0000-4000-8000-000000000001";
const ALICE = "00000000-0000-0000-0000-000000000011";
const BEN = "00000000-0000-0000-0000-000000000012";

const aliceShare: DashboardShare = { user: { id: ALICE, email: "alice@example.com", name: "Alice Perera" }, sharedOn: "a" };
const benShare: DashboardShare = { user: { id: BEN, email: "ben@example.com", name: "Ben Bee" }, sharedOn: "b" };
const sharesKey = [ApiQueryKeys.CSM_CUSTOM_DASHBOARD_SHARES, DASHBOARD];

function setup() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
  return { queryClient, wrapper };
}

describe("useDashboardShares", () => {
  beforeEach(() => {
    getMock.mockReset();
    putMock.mockReset();
    delMock.mockReset();
  });

  describe("useGetCustomDashboardShares", () => {
    it("lists who the dashboard is shared with", async () => {
      getMock.mockResolvedValue({ shares: [aliceShare] });
      const { wrapper } = setup();
      const { result } = renderHook(() => useGetCustomDashboardShares(DASHBOARD), { wrapper });
      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(getMock).toHaveBeenCalledWith(`/dashboards/custom/${DASHBOARD}/shares`);
      expect(result.current.data).toEqual([aliceShare]);
    });

    it("is null when the dashboard is not the caller's", async () => {
      getMock.mockResolvedValue(null);
      const { wrapper } = setup();
      const { result } = renderHook(() => useGetCustomDashboardShares(DASHBOARD), { wrapper });
      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(result.current.data).toBeNull();
    });

    it("makes no request without a dashboard", () => {
      const { wrapper } = setup();
      const { result } = renderHook(() => useGetCustomDashboardShares(undefined), { wrapper });
      expect(result.current.fetchStatus).toBe("idle");
      expect(getMock).not.toHaveBeenCalled();
    });
  });

  describe("usePutCustomDashboardShare", () => {
    it("shares by user id and caches the answer", async () => {
      putMock.mockResolvedValue({ shares: [aliceShare] });
      const { queryClient, wrapper } = setup();
      const { result } = renderHook(() => usePutCustomDashboardShare(), { wrapper });
      await act(async () => {
        await result.current.mutateAsync({ dashboardId: DASHBOARD, userId: ALICE });
      });
      expect(putMock).toHaveBeenCalledWith(`/dashboards/custom/${DASHBOARD}/shares/${ALICE}`, {});
      expect(queryClient.getQueryData(sharesKey)).toEqual([aliceShare]);
    });

    it("throws the backend's refusal", async () => {
      const err = new Error("dashboards can only be shared with internal staff");
      putMock.mockRejectedValue(err);
      const { wrapper } = setup();
      const { result } = renderHook(() => usePutCustomDashboardShare(), { wrapper });
      await act(async () => {
        await expect(result.current.mutateAsync({ dashboardId: DASHBOARD, userId: ALICE })).rejects.toBe(err);
      });
    });
  });

  describe("useDeleteCustomDashboardShare", () => {
    it("unshares and drops the person from the cached list at once", async () => {
      delMock.mockResolvedValue(null);
      const { queryClient, wrapper } = setup();
      queryClient.setQueryData(sharesKey, [aliceShare, benShare]);
      const invalidate = vi.spyOn(queryClient, "invalidateQueries");
      const { result } = renderHook(() => useDeleteCustomDashboardShare(), { wrapper });
      await act(async () => {
        await result.current.mutateAsync({ dashboardId: DASHBOARD, userId: ALICE });
      });
      expect(delMock).toHaveBeenCalledWith(`/dashboards/custom/${DASHBOARD}/shares/${ALICE}`);
      expect(queryClient.getQueryData(sharesKey)).toEqual([benShare]);
      expect(invalidate).toHaveBeenCalledWith({ queryKey: sharesKey });
    });
  });

  describe("useGetSharedDashboards", () => {
    it("lists the dashboards shared with the caller", async () => {
      const list = [{ id: DASHBOARD, name: "SLA", owner: aliceShare.user, sharedOn: "a", updatedOn: "b" }];
      getMock.mockResolvedValue({ dashboards: list });
      const { wrapper } = setup();
      const { result } = renderHook(() => useGetSharedDashboards(), { wrapper });
      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(getMock).toHaveBeenCalledWith("/dashboards/shared");
      expect(result.current.data).toEqual(list);
    });

    it("is an error, not an empty list, when the route 404s", async () => {
      getMock.mockResolvedValue(null);
      const { wrapper } = setup();
      const { result } = renderHook(() => useGetSharedDashboards(), { wrapper });
      await waitFor(() => expect(result.current.isError).toBe(true));
    });
  });

  describe("useGetSharedDashboard", () => {
    it("fetches one shared dashboard by id", async () => {
      const shared = { dashboard: { id: DASHBOARD }, owner: aliceShare.user, sharedOn: "a", updatedOn: "b" };
      getMock.mockResolvedValue(shared);
      const { wrapper } = setup();
      const { result } = renderHook(() => useGetSharedDashboard(DASHBOARD), { wrapper });
      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(getMock).toHaveBeenCalledWith(`/dashboards/shared/${DASHBOARD}`);
      expect(result.current.data).toEqual(shared);
    });

    it("is null when it is not shared with the caller", async () => {
      getMock.mockResolvedValue(null);
      const { wrapper } = setup();
      const { result } = renderHook(() => useGetSharedDashboard(DASHBOARD), { wrapper });
      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(result.current.data).toBeNull();
    });

    it("makes no request without an id", () => {
      const { wrapper } = setup();
      const { result } = renderHook(() => useGetSharedDashboard(undefined), { wrapper });
      expect(result.current.fetchStatus).toBe("idle");
      expect(getMock).not.toHaveBeenCalled();
    });
  });
});
