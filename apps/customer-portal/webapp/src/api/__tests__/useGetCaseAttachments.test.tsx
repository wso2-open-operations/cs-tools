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

import { renderHook, waitFor, act } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, it, expect, vi, beforeEach } from "vitest";
import {
  useGetCaseAttachments,
  flattenCaseAttachments,
} from "@api/useGetCaseAttachments";

const mockAttachmentsPage1 = {
  attachments: [
    {
      id: "a1",
      name: "file1.txt",
      type: "text/plain",
      size: 100,
      downloadUrl: "/a1",
      createdOn: "",
      createdBy: "",
    },
    {
      id: "a2",
      name: "file2.txt",
      type: "text/plain",
      size: 200,
      downloadUrl: "/a2",
      createdOn: "",
      createdBy: "",
    },
  ],
  totalRecords: 12,
  offset: 0,
  limit: 10,
};

const mockAttachmentsPage2 = {
  attachments: [
    {
      id: "a3",
      name: "file3.txt",
      type: "text/plain",
      size: 300,
      downloadUrl: "/a3",
      createdOn: "",
      createdBy: "",
    },
  ],
  totalRecords: 12,
  offset: 10,
  limit: 10,
};

vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({
    getIdToken: vi.fn().mockResolvedValue("mock-token"),
    isSignedIn: true,
    isLoading: false,
  }),
}));

const mockAuthFetch = vi.fn().mockResolvedValue({
  ok: true,
  json: () => Promise.resolve(mockAttachmentsPage1),
});

vi.mock("@api/useAuthApiClient", () => ({
  useAuthApiClient: () => mockAuthFetch,
}));

vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({ debug: vi.fn(), error: vi.fn() }),
}));

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: false } },
});

const wrapper = ({ children }: { children: React.ReactNode }) => (
  <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
);

describe("useGetCaseAttachments", () => {
  beforeEach(() => {
    queryClient.clear();
    vi.clearAllMocks();
    mockAuthFetch.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockAttachmentsPage1),
    });
    (
      window as unknown as {
        config?: { CUSTOMER_PORTAL_BACKEND_BASE_URL?: string };
      }
    ).config = {
      CUSTOMER_PORTAL_BACKEND_BASE_URL: "https://api.test",
    };
  });

  it("should return first page of attachments from infinite query", async () => {
    const { result } = renderHook(() => useGetCaseAttachments("case-001"), {
      wrapper,
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data).toBeDefined();
    expect(result.current.data?.pages).toBeDefined();
    expect(result.current.data?.pages[0]?.attachments).toEqual(
      mockAttachmentsPage1.attachments,
    );
    expect(result.current.data?.pages[0]?.totalRecords).toBe(12);
  });

  it("should not fetch when caseId is missing", () => {
    const { result } = renderHook(() => useGetCaseAttachments(""), { wrapper });
    expect(result.current.isFetching).toBe(false);
  });

  it("should use correct query key with infinite flag", () => {
    renderHook(() => useGetCaseAttachments("case-001"), { wrapper });
    const query = queryClient.getQueryCache().findAll({
      queryKey: ["case-attachments", "case-001", "infinite"],
    })[0];
    expect(query).toBeDefined();
  });

  it("should support fetching next page", async () => {
    mockAuthFetch
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockAttachmentsPage1),
      })
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockAttachmentsPage2),
      });

    const { result } = renderHook(() => useGetCaseAttachments("case-001"), {
      wrapper,
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.hasNextPage).toBe(true);

    await act(async () => {
      await result.current.fetchNextPage();
    });

    await waitFor(() => expect(result.current.data?.pages.length).toBe(2));
    expect(result.current.data?.pages[1]?.attachments).toEqual(
      mockAttachmentsPage2.attachments,
    );
  });

  it("should flatten attachments from all pages", () => {
    const mockData = {
      pages: [mockAttachmentsPage1, mockAttachmentsPage2],
      pageParams: [0, 10],
    };

    const flattened = flattenCaseAttachments(mockData);
    expect(flattened).toHaveLength(3);
    expect(flattened[0].id).toBe("a1");
    expect(flattened[1].id).toBe("a2");
    expect(flattened[2].id).toBe("a3");
  });
});
