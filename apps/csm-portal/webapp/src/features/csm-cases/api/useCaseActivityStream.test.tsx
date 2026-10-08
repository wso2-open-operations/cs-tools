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
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";

const { MockEventSource, mockInstances } = vi.hoisted(() => {
  const mockInstances: InstanceType<typeof MockEventSource>[] = [];
  class MockEventSource extends EventTarget {
    url: string;
    headers?: Record<string, string>;
    closed = false;
    constructor(url: string, init?: { headers?: Record<string, string> }) {
      super();
      this.url = url;
      this.headers = init?.headers;
      mockInstances.push(this);
    }
    close() {
      this.closed = true;
    }
  }
  return { MockEventSource, mockInstances };
});

vi.mock("@sanity/eventsource", () => ({ default: MockEventSource }));

const getTokensMock = vi.fn();
vi.mock("@hooks/useAuthTokens", () => ({
  useAuthTokens: () => getTokensMock,
}));

const { apiConfigMock } = vi.hoisted(() => ({
  apiConfigMock: {
    backendUrl: "https://example.test",
    streamUrl: "https://stream.example.test" as string | undefined,
    streamEnabled: true,
  },
}));
vi.mock("@config/apiConfig", () => ({ apiConfig: apiConfigMock }));

const debugMock = vi.fn();
vi.mock("@hooks/useLogger", () => ({
  useLogger: () => ({
    debug: debugMock,
    info: vi.fn(),
    warn: vi.fn(),
    error: vi.fn(),
  }),
}));

const { useCaseActivityStream } = await import("./useCaseActivityStream");
const { CaseTabVisibilityProvider } = await import(
  "@context/case-tabs/CaseTabVisibilityContext"
);

const invalidateQueriesMock = vi.fn();

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  queryClient.invalidateQueries = invalidateQueriesMock.mockImplementation(() =>
    Promise.resolve(),
  );
  return (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe("useCaseActivityStream", () => {
  beforeEach(() => {
    mockInstances.length = 0;
    invalidateQueriesMock.mockReset();
    debugMock.mockReset();
    getTokensMock.mockReset().mockResolvedValue({ token: "access-token", idToken: "id-token" });
    apiConfigMock.streamUrl = "https://stream.example.test";
    apiConfigMock.streamEnabled = true;
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it("does not connect when caseId is unset", async () => {
    renderHook(() => useCaseActivityStream(undefined), { wrapper });
    await Promise.resolve();
    expect(mockInstances).toHaveLength(0);
  });

  it("does not connect when the stream base URL isn't configured", async () => {
    apiConfigMock.streamUrl = undefined;
    renderHook(() => useCaseActivityStream("case-1"), { wrapper });
    await Promise.resolve();
    expect(mockInstances).toHaveLength(0);
  });

  it("does not connect when the stream is not enabled, even with a URL configured", async () => {
    apiConfigMock.streamEnabled = false;
    renderHook(() => useCaseActivityStream("case-1"), { wrapper });
    await Promise.resolve();
    expect(mockInstances).toHaveLength(0);
  });

  it("connects with the case's stream URL and auth headers", async () => {
    renderHook(() => useCaseActivityStream("case-1"), { wrapper });

    await waitFor(() => expect(mockInstances).toHaveLength(1));
    const source = mockInstances[0];
    expect(source.url).toBe("https://stream.example.test/cases/case-1/activities/stream");
    expect(source.headers).toEqual({
      "x-jwt-assertion": "access-token",
      "x-user-id-token": "id-token",
    });
  });

  it("invalidates the comments and activities queries on a case_updated event", async () => {
    renderHook(() => useCaseActivityStream("case-1"), { wrapper });
    await waitFor(() => expect(mockInstances).toHaveLength(1));
    const source = mockInstances[0];

    act(() => {
      source.dispatchEvent(new MessageEvent("case_updated", { data: "{}" }));
    });

    expect(invalidateQueriesMock).toHaveBeenCalledWith(
      expect.objectContaining({ queryKey: ["csm-case-comments", "case-1"] }),
    );
    expect(invalidateQueriesMock).toHaveBeenCalledWith(
      expect.objectContaining({ queryKey: ["csm-case-activities", "case-1"] }),
    );
  });

  it("closes and reconnects with a fresh token after an error", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    renderHook(() => useCaseActivityStream("case-1"), { wrapper });
    await waitFor(() => expect(mockInstances).toHaveLength(1));
    const first = mockInstances[0];

    getTokensMock.mockResolvedValue({ token: "fresh-access-token", idToken: "id-token" });
    act(() => {
      first.dispatchEvent(new Event("error"));
    });
    expect(first.closed).toBe(true);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(3_000);
    });

    await waitFor(() => expect(mockInstances).toHaveLength(2));
    expect(mockInstances[1].headers?.["x-jwt-assertion"]).toBe("fresh-access-token");
  });

  it("backs off exponentially on consecutive errors, and resets after a successful connection", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    // Math.random() fixed at 1 makes reconnectDelay's jitter deterministic:
    // delay = cap * 1 = cap, so the assertions below can pin exact timings.
    const randomSpy = vi.spyOn(Math, "random").mockReturnValue(1);

    renderHook(() => useCaseActivityStream("case-1"), { wrapper });
    await waitFor(() => expect(mockInstances).toHaveLength(1));

    // 1st error: base delay (3000ms).
    act(() => mockInstances[0].dispatchEvent(new Event("error")));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2_999);
    });
    expect(mockInstances).toHaveLength(1);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2);
    });
    await waitFor(() => expect(mockInstances).toHaveLength(2));

    // 2nd *consecutive* error (no successful open in between): delay doubles
    // to 6000ms rather than staying fixed at 3000ms.
    act(() => mockInstances[1].dispatchEvent(new Event("error")));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(5_999);
    });
    expect(mockInstances).toHaveLength(2);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2);
    });
    await waitFor(() => expect(mockInstances).toHaveLength(3));

    // A successful connection resets the backoff back to the base delay.
    act(() => {
      mockInstances[2].dispatchEvent(new Event("open"));
      mockInstances[2].dispatchEvent(new Event("error"));
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2_999);
    });
    expect(mockInstances).toHaveLength(3);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2);
    });
    await waitFor(() => expect(mockInstances).toHaveLength(4));

    randomSpy.mockRestore();
  });

  it("falls back to a slow idle retry after sustained consecutive failures, instead of hammering the backend every 30s forever", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    // Math.random() fixed at 1 makes reconnectDelay's jitter deterministic.
    const randomSpy = vi.spyOn(Math, "random").mockReturnValue(1);

    renderHook(() => useCaseActivityStream("case-1"), { wrapper });
    await waitFor(() => expect(mockInstances).toHaveLength(1));

    // 10 consecutive failures (no successful "open" in between) — delay
    // grows 3s/6s/12s/24s then caps at 30s for the remainder, same as the
    // exponential-backoff test above, just carried further.
    const backoffDelaysMs = [3_000, 6_000, 12_000, 24_000, 30_000, 30_000, 30_000, 30_000, 30_000, 30_000];
    for (const delay of backoffDelaysMs) {
      act(() => mockInstances[mockInstances.length - 1].dispatchEvent(new Event("error")));
      await act(async () => {
        await vi.advanceTimersByTimeAsync(delay);
      });
    }
    expect(mockInstances).toHaveLength(11);

    // The 11th consecutive failure (attempt index 10, past MAX_BACKOFF_ATTEMPTS)
    // must NOT reconnect after another 30s — only after the slow idle delay.
    act(() => mockInstances[mockInstances.length - 1].dispatchEvent(new Event("error")));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(30_000);
    });
    expect(mockInstances).toHaveLength(11);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(5 * 60_000 - 30_000);
    });
    await waitFor(() => expect(mockInstances).toHaveLength(12));

    randomSpy.mockRestore();
  });

  it("closes the connection on unmount", async () => {
    const { unmount } = renderHook(() => useCaseActivityStream("case-1"), { wrapper });
    await waitFor(() => expect(mockInstances).toHaveLength(1));
    const source = mockInstances[0];

    unmount();

    expect(source.closed).toBe(true);
  });

  describe("visibility", () => {
    function setDocumentVisibility(state: "visible" | "hidden") {
      Object.defineProperty(document, "visibilityState", {
        configurable: true,
        get: () => state,
      });
      document.dispatchEvent(new Event("visibilitychange"));
    }

    // A stable QueryClient (unlike `wrapper` above, which builds a new one on every
    // render) so a re-render caused by a visibility flip does not itself re-run
    // the effect.
    function renderVisibilityHook(initialVisible: boolean) {
      let tabVisible = initialVisible;
      const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
      queryClient.invalidateQueries = invalidateQueriesMock.mockImplementation(() =>
        Promise.resolve(),
      );
      const result = renderHook(() => useCaseActivityStream("case-1"), {
        wrapper: ({ children }: { children: ReactNode }) => (
          <QueryClientProvider client={queryClient}>
            <CaseTabVisibilityProvider isVisible={tabVisible}>{children}</CaseTabVisibilityProvider>
          </QueryClientProvider>
        ),
      });
      return {
        ...result,
        setTabVisible: (visible: boolean) => {
          tabVisible = visible;
          result.rerender();
        },
      };
    }

    afterEach(() => {
      setDocumentVisibility("visible");
    });

    it("does not connect while its case tab is not the active one", async () => {
      renderVisibilityHook(false);
      await Promise.resolve();
      await Promise.resolve();
      expect(mockInstances).toHaveLength(0);
      expect(invalidateQueriesMock).not.toHaveBeenCalled();
    });

    it("does not connect while the browser tab is hidden", async () => {
      setDocumentVisibility("hidden");
      renderVisibilityHook(true);
      await Promise.resolve();
      await Promise.resolve();
      expect(mockInstances).toHaveLength(0);
    });

    it("closes when the browser tab is hidden, and reconnects with one invalidation when it is shown again", async () => {
      renderVisibilityHook(true);
      await waitFor(() => expect(mockInstances).toHaveLength(1));
      expect(invalidateQueriesMock).not.toHaveBeenCalled();

      act(() => setDocumentVisibility("hidden"));
      expect(mockInstances[0].closed).toBe(true);
      expect(mockInstances).toHaveLength(1);

      act(() => setDocumentVisibility("visible"));
      await waitFor(() => expect(mockInstances).toHaveLength(2));
      expect(mockInstances[1].closed).toBe(false);
      // Comments + activities, once: events emitted while closed were not delivered.
      expect(invalidateQueriesMock).toHaveBeenCalledTimes(2);
    });

    it("closes when its case tab is deactivated, and reconnects when it is activated again", async () => {
      const { setTabVisible } = renderVisibilityHook(true);
      await waitFor(() => expect(mockInstances).toHaveLength(1));

      act(() => setTabVisible(false));
      expect(mockInstances[0].closed).toBe(true);
      expect(invalidateQueriesMock).not.toHaveBeenCalled();

      act(() => setTabVisible(true));
      await waitFor(() => expect(mockInstances).toHaveLength(2));
      expect(invalidateQueriesMock).toHaveBeenCalledTimes(2);
    });

    it("cancels a pending reconnect when it goes inactive", async () => {
      vi.useFakeTimers({ shouldAdvanceTime: true });
      renderVisibilityHook(true);
      await waitFor(() => expect(mockInstances).toHaveLength(1));

      act(() => mockInstances[0].dispatchEvent(new Event("error")));
      act(() => setDocumentVisibility("hidden"));
      await act(async () => {
        await vi.advanceTimersByTimeAsync(60_000);
      });

      expect(mockInstances).toHaveLength(1);
    });
  });
});
