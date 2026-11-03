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
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiQueryKeys } from "@constants/apiConstants";
import { IdleStateContext } from "@providers/IdleStateContext";
import { useCaseActivityStream } from "@features/support/api/useCaseActivityStream";

type Listener = () => void;

const { MockEventSource } = vi.hoisted(() => {
  class MockEventSource {
    static instances: MockEventSource[] = [];
    listeners = new Map<string, Listener[]>();
    close = vi.fn();

    constructor(
      public readonly url: string,
      public readonly init: { headers: Record<string, string> },
    ) {
      MockEventSource.instances.push(this);
    }

    addEventListener(type: string, fn: Listener): void {
      this.listeners.set(type, [...(this.listeners.get(type) ?? []), fn]);
    }

    emit(type: string): void {
      (this.listeners.get(type) ?? []).forEach((fn) => fn());
    }
  }
  return { MockEventSource };
});

vi.mock("@sanity/eventsource", () => ({ default: MockEventSource }));

const mockGetIdToken = vi.fn();
vi.mock("@asgardeo/react", () => ({
  useAsgardeo: () => ({ getIdToken: mockGetIdToken }),
}));

vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({ debug: vi.fn(), error: vi.fn(), warn: vi.fn(), info: vi.fn() }),
}));

const mockApiConfig = vi.hoisted(() => ({
  streamEnabled: true as boolean,
  streamUrl: "https://stream.example.com" as string | undefined,
}));
vi.mock("@config/apiConfig", () => ({ apiConfig: mockApiConfig }));

const INVALIDATED_KEYS = [
  ApiQueryKeys.CASE_COMMENTS,
  ApiQueryKeys.CASE_DETAILS,
  ApiQueryKeys.PROJECT_CASES,
  ApiQueryKeys.CASES_STATS,
  ApiQueryKeys.SUPPORT_STATS,
  ApiQueryKeys.PROJECT_STATS,
];

const idleRef = { current: false };

function setup(caseId: string | undefined, idle = false) {
  const queryClient = new QueryClient();
  const spy = vi.spyOn(queryClient, "invalidateQueries");
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>
      <IdleStateContext.Provider value={idleRef.current}>{children}</IdleStateContext.Provider>
    </QueryClientProvider>
  );
  idleRef.current = idle;
  const hook = renderHook(() => useCaseActivityStream(caseId), { wrapper });
  return { spy, ...hook };
}

const invalidatedPrefixes = (spy: ReturnType<typeof setup>["spy"]) =>
  spy.mock.calls.map(([filters]) => (filters?.queryKey as unknown[])[0]);

describe("useCaseActivityStream", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    MockEventSource.instances = [];
    mockGetIdToken.mockResolvedValue("id-token");
    mockApiConfig.streamEnabled = true;
    mockApiConfig.streamUrl = "https://stream.example.com";
  });

  it("connects with auth headers to the case stream URL", async () => {
    setup("case 1");
    await waitFor(() => expect(MockEventSource.instances).toHaveLength(1));
    const [source] = MockEventSource.instances;
    expect(source.url).toBe("https://stream.example.com/cases/case%201/activities/stream");
    expect(source.init.headers).toEqual({
      Authorization: "Bearer id-token",
      "x-user-id-token": "id-token",
    });
  });

  it("invalidates comments, details, case lists and derived stats on case_updated", async () => {
    const { spy } = setup("case-1");
    await waitFor(() => expect(MockEventSource.instances).toHaveLength(1));

    MockEventSource.instances[0].emit("case_updated");

    expect(invalidatedPrefixes(spy)).toEqual(INVALIDATED_KEYS);
  });

  it("invalidates the same set on every connection open", async () => {
    const { spy } = setup("case-1");
    await waitFor(() => expect(MockEventSource.instances).toHaveLength(1));

    MockEventSource.instances[0].emit("open");

    expect(invalidatedPrefixes(spy)).toEqual(INVALIDATED_KEYS);
  });

  it("does not invalidate unrelated keys", async () => {
    const { spy } = setup("case-1");
    await waitFor(() => expect(MockEventSource.instances).toHaveLength(1));

    MockEventSource.instances[0].emit("case_updated");

    expect(invalidatedPrefixes(spy)).not.toContain(ApiQueryKeys.PROJECTS);
    expect(invalidatedPrefixes(spy)).not.toContain(ApiQueryKeys.DEPLOYMENTS);
  });

  it("does not connect when streaming is disabled or the URL is unset", async () => {
    mockApiConfig.streamEnabled = false;
    setup("case-1");
    mockApiConfig.streamEnabled = true;
    mockApiConfig.streamUrl = undefined;
    setup("case-1");
    await new Promise((r) => setTimeout(r, 20));
    expect(MockEventSource.instances).toHaveLength(0);
  });

  it("does not connect without a case id", async () => {
    setup(undefined);
    await new Promise((r) => setTimeout(r, 20));
    expect(MockEventSource.instances).toHaveLength(0);
  });

  it("closes the stream on unmount", async () => {
    const { unmount } = setup("case-1");
    await waitFor(() => expect(MockEventSource.instances).toHaveLength(1));
    unmount();
    expect(MockEventSource.instances[0].close).toHaveBeenCalled();
  });

  it("does not connect while idle", async () => {
    setup("case-1", true);
    await new Promise((r) => setTimeout(r, 20));
    expect(MockEventSource.instances).toHaveLength(0);
    expect(mockGetIdToken).not.toHaveBeenCalled();
  });

  it("closes the open stream when idle starts and reconnects fresh when it clears", async () => {
    const { rerender } = setup("case-1");
    await waitFor(() => expect(MockEventSource.instances).toHaveLength(1));
    const first = MockEventSource.instances[0];

    idleRef.current = true;
    rerender();
    expect(first.close).toHaveBeenCalled();
    await new Promise((r) => setTimeout(r, 20));
    expect(MockEventSource.instances).toHaveLength(1);

    idleRef.current = false;
    rerender();
    await waitFor(() => expect(MockEventSource.instances).toHaveLength(2));
  });

  it("cancels a pending reconnect when idle starts", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    try {
      const { rerender } = setup("case-1");
      await waitFor(() => expect(MockEventSource.instances).toHaveLength(1));
      MockEventSource.instances[0].emit("error");

      idleRef.current = true;
      rerender();
      await vi.advanceTimersByTimeAsync(10 * 60_000);
      expect(MockEventSource.instances).toHaveLength(1);
    } finally {
      vi.useRealTimers();
    }
  });
});
