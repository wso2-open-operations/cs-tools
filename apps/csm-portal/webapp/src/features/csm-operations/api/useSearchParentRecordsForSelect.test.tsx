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

// The real client reads runtime config at module load, which isn't present
// under vitest (same approach as useSearchIncidentsForSelect.test.tsx).
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
}));

import { useSearchParentRecordsForSelect } from "@features/csm-operations/api/useSearchParentRecordsForSelect";

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

describe("useSearchParentRecordsForSelect", () => {
  beforeEach(() => {
    postMock.mockReset();
    postMock.mockImplementation((url: string) => {
      if (url === "/cases/search") {
        return Promise.resolve({
          cases: [{ id: "sr-1", number: "CS0000001", subject: "Cluster down" }],
        });
      }
      if (url === "/incidents/search") {
        return Promise.resolve({
          incidents: [{ id: "inc-1", number: "INC0000001", subject: "Gateway 502s" }],
        });
      }
      throw new Error(`unexpected url: ${url}`);
    });
  });

  it("merges service-request and incident results, tagged with their kind", async () => {
    const { result } = renderHook(() => useSearchParentRecordsForSelect("", true), { wrapper });

    await waitFor(() => expect(result.current.data).toBeDefined());

    expect(result.current.data).toEqual([
      { kind: "service_request", id: "sr-1", number: "CS0000001", subject: "Cluster down" },
      { kind: "incident", id: "inc-1", number: "INC0000001", subject: "Gateway 502s" },
    ]);
  });

  it("is fetching while either underlying search is still in flight", () => {
    postMock.mockImplementation(() => new Promise(() => {}));
    const { result } = renderHook(() => useSearchParentRecordsForSelect("", true), { wrapper });

    expect(result.current.isFetching).toBe(true);
    expect(result.current.data).toBeUndefined();
  });

  it("reports an error when either underlying search fails", async () => {
    postMock.mockImplementation((url: string) => {
      if (url === "/cases/search") return Promise.reject(new Error("cases search down"));
      return Promise.resolve({ incidents: [] });
    });
    const { result } = renderHook(() => useSearchParentRecordsForSelect("", true), { wrapper });

    await waitFor(() => expect(result.current.isError).toBe(true));
  });

  it("forwards its projectId extra only to the service-request search", async () => {
    renderHook(() => useSearchParentRecordsForSelect("query", true, "prj-1"), { wrapper });

    await waitFor(() =>
      expect(postMock).toHaveBeenCalledWith(
        "/cases/search",
        expect.objectContaining({
          filters: expect.objectContaining({
            filters: expect.arrayContaining([
              { field: "projectId", op: "in", values: ["prj-1"] },
            ]),
          }),
        }),
      ),
    );
    expect(postMock).toHaveBeenCalledWith(
      "/incidents/search",
      expect.objectContaining({ filters: { searchQuery: "query" } }),
    );
  });
});
