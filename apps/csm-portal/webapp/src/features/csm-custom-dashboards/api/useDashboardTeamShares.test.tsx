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
  useDeleteCustomDashboardTeamShare,
  useGetCustomDashboardTeamShares,
  usePutCustomDashboardTeamShare,
  type DashboardTeamShare,
} from "@features/csm-custom-dashboards/api/useDashboardTeamShares";

const DASHBOARD = "5e636a12-0000-4000-8000-000000000001";

const rigelShare: DashboardTeamShare = { team: { key: "rigel", name: "Rigel", family: "cre-abt" }, sharedOn: "a" };
const apolloShare: DashboardTeamShare = { team: { key: "apollo", name: "Apollo", family: "sre-abt" }, sharedOn: "b" };
const teamSharesKey = [ApiQueryKeys.CSM_CUSTOM_DASHBOARD_TEAM_SHARES, DASHBOARD];

function setup() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
  return { queryClient, wrapper };
}

describe("useDashboardTeamShares", () => {
  beforeEach(() => {
    getMock.mockReset();
    putMock.mockReset();
    delMock.mockReset();
  });

  describe("useGetCustomDashboardTeamShares", () => {
    it("lists the teams the dashboard is shared with", async () => {
      getMock.mockResolvedValue({ shares: [apolloShare, rigelShare] });
      const { wrapper } = setup();
      const { result } = renderHook(() => useGetCustomDashboardTeamShares(DASHBOARD), { wrapper });
      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(getMock).toHaveBeenCalledWith(`/dashboards/custom/${DASHBOARD}/team-shares`);
      expect(result.current.data).toEqual([apolloShare, rigelShare]);
    });

    it("is null when the dashboard is not the caller's", async () => {
      getMock.mockResolvedValue(null);
      const { wrapper } = setup();
      const { result } = renderHook(() => useGetCustomDashboardTeamShares(DASHBOARD), { wrapper });
      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(result.current.data).toBeNull();
    });

    it("makes no request without a dashboard", () => {
      const { wrapper } = setup();
      const { result } = renderHook(() => useGetCustomDashboardTeamShares(undefined), { wrapper });
      expect(result.current.fetchStatus).toBe("idle");
      expect(getMock).not.toHaveBeenCalled();
    });
  });

  describe("usePutCustomDashboardTeamShare", () => {
    it("shares by team key and caches the answer", async () => {
      putMock.mockResolvedValue({ shares: [rigelShare] });
      const { queryClient, wrapper } = setup();
      const { result } = renderHook(() => usePutCustomDashboardTeamShare(), { wrapper });
      await act(async () => {
        await result.current.mutateAsync({ dashboardId: DASHBOARD, teamKey: "rigel" });
      });
      expect(putMock).toHaveBeenCalledWith(`/dashboards/custom/${DASHBOARD}/team-shares/rigel`, {});
      expect(queryClient.getQueryData(teamSharesKey)).toEqual([rigelShare]);
    });

    it("throws the backend's refusal", async () => {
      const err = new Error("That team can't be found, so the dashboard can't be shared with it.");
      putMock.mockRejectedValue(err);
      const { wrapper } = setup();
      const { result } = renderHook(() => usePutCustomDashboardTeamShare(), { wrapper });
      await act(async () => {
        await expect(result.current.mutateAsync({ dashboardId: DASHBOARD, teamKey: "orion" })).rejects.toBe(err);
      });
    });
  });

  describe("useDeleteCustomDashboardTeamShare", () => {
    it("unshares and drops the team from the cached list at once", async () => {
      delMock.mockResolvedValue(null);
      const { queryClient, wrapper } = setup();
      queryClient.setQueryData(teamSharesKey, [apolloShare, rigelShare]);
      const invalidate = vi.spyOn(queryClient, "invalidateQueries");
      const { result } = renderHook(() => useDeleteCustomDashboardTeamShare(), { wrapper });
      await act(async () => {
        await result.current.mutateAsync({ dashboardId: DASHBOARD, teamKey: "apollo" });
      });
      expect(delMock).toHaveBeenCalledWith(`/dashboards/custom/${DASHBOARD}/team-shares/apollo`);
      expect(invalidate).toHaveBeenCalledWith({ queryKey: teamSharesKey });
      await waitFor(() => expect(queryClient.getQueryData(teamSharesKey)).toEqual([rigelShare]));
    });
  });
});
