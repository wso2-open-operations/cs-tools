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
const postMock = vi.fn();

// The real client reads runtime config at module load, which isn't present
// under vitest.
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ get: getMock, put: putMock, del: delMock, post: postMock }),
}));

import {
  useDeleteCustomDashboardLink,
  useGetCustomDashboardLink,
  useOpenDashboardLink,
  usePutCustomDashboardLink,
  type DashboardLink,
} from "@features/csm-custom-dashboards/api/useDashboardLink";

const DASHBOARD = "5e636a12-0000-4000-8000-000000000001";
const LINK_ID = "5e636a12-0000-4000-8000-0000000000cc";

const onLink: DashboardLink = { enabled: true, linkId: LINK_ID, openedBy: 2 };
const linkKey = [ApiQueryKeys.CSM_CUSTOM_DASHBOARD_LINK, DASHBOARD];
const sharedListKey = [ApiQueryKeys.CSM_SHARED_DASHBOARD_LIST];

function setup() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
  return { queryClient, wrapper };
}

describe("useDashboardLink", () => {
  beforeEach(() => {
    getMock.mockReset();
    putMock.mockReset();
    delMock.mockReset();
    postMock.mockReset();
  });

  describe("useGetCustomDashboardLink", () => {
    it("reads the dashboard's link", async () => {
      getMock.mockResolvedValue(onLink);
      const { wrapper } = setup();
      const { result } = renderHook(() => useGetCustomDashboardLink(DASHBOARD), { wrapper });
      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(getMock).toHaveBeenCalledWith(`/dashboards/custom/${DASHBOARD}/link`);
      expect(result.current.data).toEqual(onLink);
    });

    it("is null when the dashboard is not the caller's", async () => {
      getMock.mockResolvedValue(null);
      const { wrapper } = setup();
      const { result } = renderHook(() => useGetCustomDashboardLink(DASHBOARD), { wrapper });
      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(result.current.data).toBeNull();
    });

    it("makes no request without a dashboard", () => {
      const { wrapper } = setup();
      const { result } = renderHook(() => useGetCustomDashboardLink(undefined), { wrapper });
      expect(result.current.fetchStatus).toBe("idle");
      expect(getMock).not.toHaveBeenCalled();
    });
  });

  describe("usePutCustomDashboardLink", () => {
    it("turns the link on and caches the answer", async () => {
      putMock.mockResolvedValue(onLink);
      const { queryClient, wrapper } = setup();
      queryClient.setQueryData(linkKey, { enabled: false, openedBy: 0 });
      const { result } = renderHook(() => usePutCustomDashboardLink(), { wrapper });
      await act(async () => {
        await result.current.mutateAsync(DASHBOARD);
      });
      expect(putMock).toHaveBeenCalledWith(`/dashboards/custom/${DASHBOARD}/link`, {});
      expect(queryClient.getQueryData(linkKey)).toEqual(onLink);
    });

    it("throws the backend's refusal and keeps the cached link", async () => {
      const err = new Error("The link couldn't be turned on. Try again in a moment.");
      putMock.mockRejectedValue(err);
      const { queryClient, wrapper } = setup();
      queryClient.setQueryData(linkKey, { enabled: false, openedBy: 0 });
      const { result } = renderHook(() => usePutCustomDashboardLink(), { wrapper });
      await act(async () => {
        await expect(result.current.mutateAsync(DASHBOARD)).rejects.toBe(err);
      });
      expect(queryClient.getQueryData(linkKey)).toEqual({ enabled: false, openedBy: 0 });
    });
  });

  describe("useDeleteCustomDashboardLink", () => {
    it("turns the link off, which the cache shows at once", async () => {
      delMock.mockResolvedValue(null);
      const { queryClient, wrapper } = setup();
      queryClient.setQueryData(linkKey, onLink);
      const invalidate = vi.spyOn(queryClient, "invalidateQueries");
      const { result } = renderHook(() => useDeleteCustomDashboardLink(), { wrapper });
      await act(async () => {
        await result.current.mutateAsync(DASHBOARD);
      });
      expect(delMock).toHaveBeenCalledWith(`/dashboards/custom/${DASHBOARD}/link`);
      expect(invalidate).toHaveBeenCalledWith({ queryKey: linkKey });
      expect(queryClient.getQueryData(linkKey)).toEqual({ enabled: false, openedBy: 0 });
    });
  });

  describe("useOpenDashboardLink", () => {
    it("sends the link id in the body and reads the Shared list again", async () => {
      postMock.mockResolvedValue({ dashboardId: DASHBOARD, own: false });
      const { queryClient, wrapper } = setup();
      const invalidate = vi.spyOn(queryClient, "invalidateQueries");
      const { result } = renderHook(() => useOpenDashboardLink(), { wrapper });
      let opened: unknown;
      await act(async () => {
        opened = await result.current.mutateAsync(LINK_ID);
      });
      expect(postMock).toHaveBeenCalledWith("/dashboards/shared/link", { linkId: LINK_ID });
      expect(opened).toEqual({ dashboardId: DASHBOARD, own: false });
      expect(invalidate).toHaveBeenCalledWith({ queryKey: sharedListKey });
    });

    it("throws when the link is off, and reads nothing again", async () => {
      const err = new Error("This link doesn't open a dashboard. Its owner may have turned it off.");
      postMock.mockRejectedValue(err);
      const { queryClient, wrapper } = setup();
      const invalidate = vi.spyOn(queryClient, "invalidateQueries");
      const { result } = renderHook(() => useOpenDashboardLink(), { wrapper });
      await act(async () => {
        await expect(result.current.mutateAsync(LINK_ID)).rejects.toBe(err);
      });
      expect(invalidate).not.toHaveBeenCalled();
    });
  });
});
