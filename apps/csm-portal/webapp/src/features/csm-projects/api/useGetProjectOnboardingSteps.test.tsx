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
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";

const backendGetMock = vi.fn();
vi.mock("@api/backend/client", () => ({
  useBackendApi: () => ({ get: backendGetMock }),
}));

import { useGetProjectOnboardingSteps } from "@features/csm-projects/api/useGetProjectOnboardingSteps";

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

// The flag is read from the real `window.config` at call time, so each case
// sets exactly the key it needs; the object is otherwise empty on purpose
// (nothing else in this hook's import chain reads config at module load).
function setFlag(value: unknown): void {
  window.config = { CSM_MIGRATION_ONBOARDING_STATUS_ENABLED: value } as unknown as Window["config"];
}

const RESPONSE = {
  memberships: [
    {
      membershipSfId: "a0X000000000001AAA",
      contactSfId: null,
      email: "jane.doe@example.com",
      projectContactId: null,
      steps: [],
    },
  ],
  total: 1,
  truncated: false,
};

describe("useGetProjectOnboardingSteps", () => {
  beforeEach(() => {
    backendGetMock.mockReset();
  });

  afterEach(() => {
    // @ts-expect-error -- tearing the test-only config back down
    delete window.config;
  });

  it("makes no request at all while the flag is off", () => {
    setFlag(false);
    const { result } = renderHook(() => useGetProjectOnboardingSteps("proj-1"), { wrapper });
    expect(result.current.fetchStatus).toBe("idle");
    expect(backendGetMock).not.toHaveBeenCalled();
  });

  it("treats anything other than true as off (absent, 'TRUE', 1)", () => {
    for (const value of ["false", "FALSE"]) {
      setFlag(value);
      const { result } = renderHook(() => useGetProjectOnboardingSteps("proj-1"), { wrapper });
      expect(result.current.fetchStatus).toBe("idle");
    }
    expect(backendGetMock).not.toHaveBeenCalled();
  });

  it("fetches the project's ledger once the flag is on", async () => {
    setFlag(true);
    backendGetMock.mockResolvedValue(RESPONSE);

    const { result } = renderHook(() => useGetProjectOnboardingSteps("proj-1"), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(backendGetMock).toHaveBeenCalledTimes(1);
    expect(backendGetMock).toHaveBeenCalledWith("/projects/proj-1/onboarding-steps");
    expect(result.current.data).toEqual(RESPONSE);
  });

  it("accepts the string 'true' for platforms that inject config as strings", async () => {
    setFlag("true");
    backendGetMock.mockResolvedValue(RESPONSE);

    const { result } = renderHook(() => useGetProjectOnboardingSteps("proj-1"), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(backendGetMock).toHaveBeenCalledTimes(1);
  });

  it("normalises an empty (null) body to an empty ledger", async () => {
    setFlag(true);
    backendGetMock.mockResolvedValue(null);

    const { result } = renderHook(() => useGetProjectOnboardingSteps("proj-1"), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual({ memberships: [], total: 0, truncated: false });
  });

  it("stays disabled until a project id is provided even with the flag on", () => {
    setFlag(true);
    const { result } = renderHook(() => useGetProjectOnboardingSteps(undefined), { wrapper });
    expect(result.current.fetchStatus).toBe("idle");
    expect(backendGetMock).not.toHaveBeenCalled();
  });
});
