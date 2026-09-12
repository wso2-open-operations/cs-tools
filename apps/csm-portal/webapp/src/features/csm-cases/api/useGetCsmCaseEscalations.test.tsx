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

const getMock = vi.fn();
const postMock = vi.fn();

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ get: getMock, post: postMock }),
}));

import { useGetCsmCaseEscalations } from "@features/csm-cases/api/useGetCsmCaseEscalations";

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe("useGetCsmCaseEscalations", () => {
  beforeEach(() => {
    getMock.mockReset();
    postMock.mockReset();
  });

  it("GETs /cases/{id}/escalations and maps the response to CaseEscalationHistory", async () => {
    getMock.mockResolvedValue({
      escalations: [
        {
          id: "esc-1",
          case: { id: "case-1", name: "CASE-1" },
          currentLevel: { id: "2", label: "EL2" },
          previousLevel: { id: "1", label: "EL1" },
          createdBy: "jane.doe@example.com",
          createdOn: "2026-08-01T00:00:00Z",
          updatedOn: "2026-08-01T00:00:00Z",
          reason: "Customer escalated via phone.",
        },
      ],
      total: 1,
      currentNotifiedUsers: [
        { id: "u-1", userName: "jdoe", name: "Jane Doe", email: "jane.doe@example.com" },
      ],
    });

    const { result } = renderHook(() => useGetCsmCaseEscalations("case-1"), {
      wrapper,
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(getMock).toHaveBeenCalledWith("/cases/case-1/escalations");
    expect(result.current.data).toEqual({
      escalations: [
        {
          id: "esc-1",
          currentLevel: "2",
          previousLevel: "1",
          createdBy: "jane.doe@example.com",
          createdOn: "2026-08-01T00:00:00Z",
          reason: "Customer escalated via phone.",
        },
      ],
      currentNotifiedUsers: [
        { id: "u-1", name: "Jane Doe", email: "jane.doe@example.com" },
      ],
    });
  });

  it("returns an empty history, not undefined, when the response has no escalations key", async () => {
    getMock.mockResolvedValue(null);

    const { result } = renderHook(() => useGetCsmCaseEscalations("case-1"), {
      wrapper,
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual({
      escalations: [],
      currentNotifiedUsers: [],
    });
  });

  it("does not call the backend without a case id", () => {
    renderHook(() => useGetCsmCaseEscalations(undefined), { wrapper });
    expect(getMock).not.toHaveBeenCalled();
  });
});
