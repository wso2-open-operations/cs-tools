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

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

const getMock = vi.fn();
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ get: getMock }),
}));
vi.mock("@hooks/useIdTokenClaims", () => ({
  useIdTokenClaims: () => ({ email: "jane.doe@example.com" }),
}));

import { useGetCsmCaseDetail } from "@features/csm-cases/api/useGetCsmCaseDetail";

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

describe("useGetCsmCaseDetail mapping", () => {
  it("does not invent an assignment group the case payload does not carry", async () => {
    getMock.mockResolvedValue({
      id: "00000000-0000-0000-0000-000000000000",
      number: "CS0000001",
      subject: "Example",
      state: "open",
      severity: "Low (P4)",
    });
    const { result } = renderHook(() => useGetCsmCaseDetail("case-1"), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data?.assignmentGroup).toBeUndefined();
  });
});
