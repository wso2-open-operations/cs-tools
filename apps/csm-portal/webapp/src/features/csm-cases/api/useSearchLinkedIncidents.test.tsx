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
// under vitest (same approach as useSearchChildCases's sibling hooks).
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
}));

import { useSearchLinkedIncidents } from "@features/csm-cases/api/useSearchLinkedIncidents";

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe("useSearchLinkedIncidents", () => {
  beforeEach(() => {
    postMock.mockReset();
  });

  it("searches /incidents/search filtered by parentIds and maps the rows", async () => {
    postMock.mockResolvedValue({
      incidents: [
        {
          id: "inc-1",
          number: "INC0069722",
          subject: "Example incident",
          priority: "HIGH",
          state: "IN_PROGRESS",
          assignedTo: { id: "u-1", name: "Example Engineer" },
        },
      ],
      total: 1,
      limit: 20,
      offset: 0,
    });

    const { result } = renderHook(() => useSearchLinkedIncidents("case-1"), {
      wrapper,
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(postMock).toHaveBeenCalledWith("/incidents/search", {
      filters: { parentIds: ["case-1"] },
      pagination: { offset: 0, limit: 20 },
    });
    expect(result.current.data).toEqual({
      incidents: [
        {
          id: "inc-1",
          number: "INC0069722",
          subject: "Example incident",
          priority: "HIGH",
          state: "IN_PROGRESS",
          assigneeName: "Example Engineer",
        },
      ],
      total: 1,
    });
  });

  it("filters out incidents with a null id", async () => {
    postMock.mockResolvedValue({
      incidents: [{ id: null, number: "INC0000001", subject: "No id" }],
      total: 1,
      limit: 20,
      offset: 0,
    });

    const { result } = renderHook(() => useSearchLinkedIncidents("case-1"), {
      wrapper,
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data?.incidents).toEqual([]);
  });

  it("stays disabled until a caseId is provided", () => {
    const { result } = renderHook(() => useSearchLinkedIncidents(undefined), {
      wrapper,
    });

    expect(result.current.fetchStatus).toBe("idle");
    expect(postMock).not.toHaveBeenCalled();
  });
});
