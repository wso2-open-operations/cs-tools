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
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";

const postMock = vi.fn();

vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ post: postMock }),
}));

import { useGetCsmCaseFeedback } from "@features/csm-cases/api/useCsmCaseFeedback";

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

describe("useGetCsmCaseFeedback", () => {
  beforeEach(() => {
    postMock.mockReset();
    postMock.mockResolvedValue({ results: [] });
  });

  it("does not request feedback for a case that is not closed, and reports not loading", async () => {
    const { result } = renderHook(() => useGetCsmCaseFeedback("case-1", false), { wrapper });
    await new Promise((r) => setTimeout(r, 20));
    expect(postMock).not.toHaveBeenCalled();
    expect(result.current.fetchStatus).toBe("idle");
    expect(result.current.isLoading).toBe(false);
  });

  it("does not request feedback without a case id", async () => {
    renderHook(() => useGetCsmCaseFeedback(undefined, true), { wrapper });
    await new Promise((r) => setTimeout(r, 20));
    expect(postMock).not.toHaveBeenCalled();
  });

  it("requests feedback for a closed case", async () => {
    const { result } = renderHook(() => useGetCsmCaseFeedback("case-1", true), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(postMock).toHaveBeenCalledTimes(1);
    expect(postMock.mock.calls[0][1]).toMatchObject({ filters: { caseId: "case-1" } });
  });
});
