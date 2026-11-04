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

const { useMultiplexedCaseStream, parseCaseUpdatedCaseId, ID_SET_DEBOUNCE_MS } = await import(
  "./useMultiplexedCaseStream"
);

const ID_A = "00000000-0000-0000-0000-00000000000a";
const ID_B = "00000000-0000-0000-0000-00000000000b";
const ID_C = "00000000-0000-0000-0000-00000000000c";

const invalidateQueriesMock = vi.fn();

// A stable QueryClient across renders, so a re-render never re-runs the
// stream effect on its own.
const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
queryClient.invalidateQueries = invalidateQueriesMock;

function wrapper({ children }: { children: ReactNode }) {
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

function renderStream(initialIds: string[], onCaseUpdated?: (caseId: string) => void) {
  return renderHook(
    ({ ids }: { ids: string[] }) => useMultiplexedCaseStream(ids, onCaseUpdated),
    { wrapper, initialProps: { ids: initialIds } },
  );
}

function emit(source: InstanceType<typeof MockEventSource>, data: string) {
  act(() => {
    source.dispatchEvent(new MessageEvent("case_updated", { data }));
  });
}

function emitError(source: InstanceType<typeof MockEventSource>, status?: number) {
  const event = new Event("error");
  if (status !== undefined) Object.assign(event, { status });
  act(() => {
    source.dispatchEvent(event);
  });
}

const invalidatedKeys = () =>
  invalidateQueriesMock.mock.calls.map(([arg]) => (arg as { queryKey: unknown[] }).queryKey);

const OLD_MS = 60_000;
/** Seeds comments + activities for a case as last fetched `ageMs` ago. */
function seedThread(id: string, ageMs: number) {
  for (const key of ["csm-case-comments", "csm-case-activities"]) {
    queryClient.setQueryData([key, id], [], { updatedAt: Date.now() - ageMs });
  }
}

const expectedInvalidationsFor = (id: string) => [
  ["csm-case-comments", id],
  ["csm-case-activities", id],
  ["csm-case-detail", id],
  ["work-item-slas", id],
  ["csm-cases"],
  ["csm-case-children"],
];

describe("parseCaseUpdatedCaseId", () => {
  it("returns the caseId of a well-formed frame", () => {
    expect(parseCaseUpdatedCaseId(JSON.stringify({ caseId: ID_A, type: "comment" }))).toBe(ID_A);
  });

  it.each([
    ["bad JSON", "{not json"],
    ["empty string", ""],
    ["a JSON string", '"abc"'],
    ["null", "null"],
    ["no caseId", "{}"],
    ["empty caseId", '{"caseId":""}'],
    ["non-string caseId", '{"caseId":42}'],
    ["non-string data", undefined],
  ])("returns undefined for %s", (_name, data) => {
    expect(parseCaseUpdatedCaseId(data)).toBeUndefined();
  });
});

describe("useMultiplexedCaseStream", () => {
  beforeEach(() => {
    mockInstances.length = 0;
    invalidateQueriesMock.mockReset().mockImplementation(() => Promise.resolve());
    queryClient.clear();
    debugMock.mockReset();
    getTokensMock.mockReset().mockResolvedValue({ token: "access-token", idToken: "id-token" });
    apiConfigMock.streamUrl = "https://stream.example.test";
    apiConfigMock.streamEnabled = true;
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  describe("gating", () => {
    it("does not connect when there are no ids", async () => {
      renderStream([]);
      await Promise.resolve();
      expect(mockInstances).toHaveLength(0);
    });

    it("does not connect when the stream base URL isn't configured", async () => {
      apiConfigMock.streamUrl = undefined;
      renderStream([ID_A]);
      await Promise.resolve();
      expect(mockInstances).toHaveLength(0);
    });

    it("does not connect when the stream is not enabled, even with a URL configured", async () => {
      apiConfigMock.streamEnabled = false;
      renderStream([ID_A]);
      await Promise.resolve();
      expect(mockInstances).toHaveLength(0);
    });
  });

  it("opens one connection to the multiplexed URL with every id and the auth headers", async () => {
    renderStream([ID_B, ID_A]);
    await waitFor(() => expect(mockInstances).toHaveLength(1));
    const source = mockInstances[0];
    expect(source.url).toBe(`https://stream.example.test/cases/activities/stream?ids=${ID_A},${ID_B}`);
    expect(source.headers).toEqual({
      Authorization: "Bearer access-token",
      "x-user-id-token": "id-token",
    });
  });

  it("de-duplicates ids and ignores their order", async () => {
    const { rerender } = renderStream([ID_B, ID_A, ID_A]);
    await waitFor(() => expect(mockInstances).toHaveLength(1));
    expect(mockInstances[0].url).toMatch(new RegExp(`ids=${ID_A},${ID_B}$`));

    rerender({ ids: [ID_A, ID_B] });
    await new Promise((r) => setTimeout(r, ID_SET_DEBOUNCE_MS + 50));
    expect(mockInstances).toHaveLength(1);
  });

  describe("event routing", () => {
    it("invalidates the event's case and reports it to the caller", async () => {
      const onCaseUpdated = vi.fn();
      renderStream([ID_A, ID_B], onCaseUpdated);
      await waitFor(() => expect(mockInstances).toHaveLength(1));

      emit(mockInstances[0], JSON.stringify({ caseId: ID_B, type: "comment", timestamp: "t" }));

      expect(invalidatedKeys()).toEqual(expectedInvalidationsFor(ID_B));
      expect(onCaseUpdated).toHaveBeenCalledTimes(1);
      expect(onCaseUpdated).toHaveBeenCalledWith(ID_B);
    });

    it("matches the caseId case-insensitively and reports the requested spelling", async () => {
      const onCaseUpdated = vi.fn();
      renderStream([ID_A], onCaseUpdated);
      await waitFor(() => expect(mockInstances).toHaveLength(1));

      emit(mockInstances[0], JSON.stringify({ caseId: ID_A.toUpperCase() }));

      expect(onCaseUpdated).toHaveBeenCalledWith(ID_A);
    });

    it("ignores bad JSON and frames without a caseId, logging at debug", async () => {
      const onCaseUpdated = vi.fn();
      renderStream([ID_A], onCaseUpdated);
      await waitFor(() => expect(mockInstances).toHaveLength(1));

      emit(mockInstances[0], "{not json");
      emit(mockInstances[0], "{}");

      expect(invalidateQueriesMock).not.toHaveBeenCalled();
      expect(onCaseUpdated).not.toHaveBeenCalled();
      expect(debugMock).toHaveBeenCalledTimes(2);
    });

    it("ignores a frame for an id that was never requested", async () => {
      const onCaseUpdated = vi.fn();
      renderStream([ID_A], onCaseUpdated);
      await waitFor(() => expect(mockInstances).toHaveLength(1));

      emit(mockInstances[0], JSON.stringify({ caseId: ID_C }));

      expect(invalidateQueriesMock).not.toHaveBeenCalled();
      expect(onCaseUpdated).not.toHaveBeenCalled();
    });

    it("uses the latest callback without reconnecting", async () => {
      const first = vi.fn();
      const second = vi.fn();
      const { rerender } = renderHook(
        ({ cb }: { cb: (id: string) => void }) => useMultiplexedCaseStream([ID_A], cb),
        { wrapper, initialProps: { cb: first } },
      );
      await waitFor(() => expect(mockInstances).toHaveLength(1));
      rerender({ cb: second });

      emit(mockInstances[0], JSON.stringify({ caseId: ID_A }));

      expect(first).not.toHaveBeenCalled();
      expect(second).toHaveBeenCalledWith(ID_A);
      expect(mockInstances).toHaveLength(1);
    });
  });

  describe("id-set changes", () => {
    it("reconnects once, debounced, closing the old stream first, with freshly fetched tokens", async () => {
      vi.useFakeTimers({ shouldAdvanceTime: true });
      const { rerender } = renderStream([ID_A]);
      await waitFor(() => expect(mockInstances).toHaveLength(1));
      const first = mockInstances[0];
      expect(getTokensMock).toHaveBeenCalledTimes(1);

      getTokensMock.mockResolvedValue({ token: "fresh-token", idToken: "id-token" });
      // A burst of changes inside the debounce window.
      rerender({ ids: [ID_A, ID_B] });
      await act(async () => {
        await vi.advanceTimersByTimeAsync(ID_SET_DEBOUNCE_MS - 100);
      });
      rerender({ ids: [ID_A, ID_B, ID_C] });
      await act(async () => {
        await vi.advanceTimersByTimeAsync(ID_SET_DEBOUNCE_MS - 100);
      });
      // Nothing yet: each change restarted the window, the old stream is intact.
      expect(mockInstances).toHaveLength(1);
      expect(first.closed).toBe(false);

      await act(async () => {
        await vi.advanceTimersByTimeAsync(200);
      });
      await waitFor(() => expect(mockInstances).toHaveLength(2));
      expect(first.closed).toBe(true);
      expect(mockInstances[1].url).toMatch(new RegExp(`ids=${ID_A},${ID_B},${ID_C}$`));
      expect(mockInstances[1].headers?.Authorization).toBe("Bearer fresh-token");
    });

    it("never holds more than one open connection across id changes", async () => {
      vi.useFakeTimers({ shouldAdvanceTime: true });
      const { rerender } = renderStream([ID_A]);
      await waitFor(() => expect(mockInstances).toHaveLength(1));
      const open = () => mockInstances.filter((m) => !m.closed).length;
      expect(open()).toBe(1);

      for (const ids of [[ID_A, ID_B], [ID_B], [ID_B, ID_C]]) {
        rerender({ ids });
        await act(async () => {
          await vi.advanceTimersByTimeAsync(ID_SET_DEBOUNCE_MS + 50);
        });
        await waitFor(() => expect(open()).toBe(1));
      }
      expect(mockInstances).toHaveLength(4);
    });

    it("closes the stream when the last id goes away", async () => {
      vi.useFakeTimers({ shouldAdvanceTime: true });
      const { rerender } = renderStream([ID_A]);
      await waitFor(() => expect(mockInstances).toHaveLength(1));

      rerender({ ids: [] });
      await act(async () => {
        await vi.advanceTimersByTimeAsync(ID_SET_DEBOUNCE_MS + 50);
      });

      expect(mockInstances[0].closed).toBe(true);
      expect(mockInstances).toHaveLength(1);
    });
  });

  describe("reconnect", () => {
    it("closes and reconnects with a fresh token after an error", async () => {
      vi.useFakeTimers({ shouldAdvanceTime: true });
      renderStream([ID_A]);
      await waitFor(() => expect(mockInstances).toHaveLength(1));
      const first = mockInstances[0];

      getTokensMock.mockResolvedValue({ token: "fresh-access-token", idToken: "id-token" });
      emitError(first, 500);
      expect(first.closed).toBe(true);

      await act(async () => {
        await vi.advanceTimersByTimeAsync(3_000);
      });

      await waitFor(() => expect(mockInstances).toHaveLength(2));
      expect(mockInstances[1].headers?.Authorization).toBe("Bearer fresh-access-token");
    });

    it("backs off exponentially on consecutive errors, and resets after a successful connection", async () => {
      vi.useFakeTimers({ shouldAdvanceTime: true });
      // Math.random() fixed at 1 makes reconnectDelay's jitter deterministic.
      const randomSpy = vi.spyOn(Math, "random").mockReturnValue(1);

      renderStream([ID_A]);
      await waitFor(() => expect(mockInstances).toHaveLength(1));

      emitError(mockInstances[0]);
      await act(async () => {
        await vi.advanceTimersByTimeAsync(2_999);
      });
      expect(mockInstances).toHaveLength(1);
      await act(async () => {
        await vi.advanceTimersByTimeAsync(2);
      });
      await waitFor(() => expect(mockInstances).toHaveLength(2));

      emitError(mockInstances[1]);
      await act(async () => {
        await vi.advanceTimersByTimeAsync(5_999);
      });
      expect(mockInstances).toHaveLength(2);
      await act(async () => {
        await vi.advanceTimersByTimeAsync(2);
      });
      await waitFor(() => expect(mockInstances).toHaveLength(3));

      act(() => {
        mockInstances[2].dispatchEvent(new Event("open"));
      });
      emitError(mockInstances[2]);
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

    it("falls back to a slow idle retry after sustained consecutive failures", async () => {
      vi.useFakeTimers({ shouldAdvanceTime: true });
      const randomSpy = vi.spyOn(Math, "random").mockReturnValue(1);

      renderStream([ID_A]);
      await waitFor(() => expect(mockInstances).toHaveLength(1));

      const backoffDelaysMs = [3_000, 6_000, 12_000, 24_000, 30_000, 30_000, 30_000, 30_000, 30_000, 30_000];
      for (const delay of backoffDelaysMs) {
        emitError(mockInstances[mockInstances.length - 1]);
        await act(async () => {
          await vi.advanceTimersByTimeAsync(delay);
        });
      }
      expect(mockInstances).toHaveLength(11);

      emitError(mockInstances[mockInstances.length - 1]);
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

    it("does not loop on a 404 from a backend without the endpoint: one slow idle retry, nothing thrown", async () => {
      vi.useFakeTimers({ shouldAdvanceTime: true });
      const randomSpy = vi.spyOn(Math, "random").mockReturnValue(1);

      renderStream([ID_A]);
      await waitFor(() => expect(mockInstances).toHaveLength(1));

      emitError(mockInstances[0], 404);
      expect(mockInstances[0].closed).toBe(true);

      // Well past the normal 30s backoff cap: still no new connection.
      await act(async () => {
        await vi.advanceTimersByTimeAsync(4 * 60_000);
      });
      expect(mockInstances).toHaveLength(1);

      // The slow idle retry (5 min) still happens, so a later deploy recovers.
      await act(async () => {
        await vi.advanceTimersByTimeAsync(60_000 + 1);
      });
      await waitFor(() => expect(mockInstances).toHaveLength(2));

      // A second 404 stays on the idle schedule rather than restarting the curve.
      emitError(mockInstances[1], 404);
      await act(async () => {
        await vi.advanceTimersByTimeAsync(4 * 60_000);
      });
      expect(mockInstances).toHaveLength(2);

      randomSpy.mockRestore();
    });

    it("retries when no tokens could be obtained instead of connecting without auth", async () => {
      vi.useFakeTimers({ shouldAdvanceTime: true });
      const randomSpy = vi.spyOn(Math, "random").mockReturnValue(1);
      getTokensMock.mockResolvedValueOnce({ token: undefined, idToken: undefined });

      renderStream([ID_A]);
      await act(async () => {
        await vi.advanceTimersByTimeAsync(10);
      });
      expect(mockInstances).toHaveLength(0);

      await act(async () => {
        await vi.advanceTimersByTimeAsync(3_000);
      });
      await waitFor(() => expect(mockInstances).toHaveLength(1));

      randomSpy.mockRestore();
    });
  });

  it("closes the connection on unmount", async () => {
    const { unmount } = renderStream([ID_A]);
    await waitFor(() => expect(mockInstances).toHaveLength(1));
    const source = mockInstances[0];

    unmount();

    expect(source.closed).toBe(true);
  });

  it("invalidates every subscribed case when a replacement connection opens, but not on the first open", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    vi.spyOn(Math, "random").mockReturnValue(1);
    const onCaseUpdated = vi.fn();
    renderStream([ID_A, ID_B], onCaseUpdated);
    await waitFor(() => expect(mockInstances).toHaveLength(1));

    act(() => {
      mockInstances[0].dispatchEvent(new Event("open"));
    });
    expect(invalidateQueriesMock).not.toHaveBeenCalled();

    emitError(mockInstances[0]);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(3_001);
    });
    await waitFor(() => expect(mockInstances).toHaveLength(2));
    act(() => {
      mockInstances[1].dispatchEvent(new Event("open"));
    });

    expect(invalidatedKeys()).toEqual([
      ["csm-case-comments", ID_A],
      ["csm-case-activities", ID_A],
      ["csm-case-detail", ID_A],
      ["work-item-slas", ID_A],
      ["csm-case-comments", ID_B],
      ["csm-case-activities", ID_B],
      ["csm-case-detail", ID_B],
      ["work-item-slas", ID_B],
      ["csm-cases"],
      ["csm-case-children"],
    ]);
    // Catching up refreshes data; it must not flag any tab.
    expect(onCaseUpdated).not.toHaveBeenCalled();
  });

  it("on an id-set change, catches up only the cases that stay subscribed", async () => {
    const { rerender } = renderStream([ID_A, ID_B]);
    await waitFor(() => expect(mockInstances).toHaveLength(1));
    act(() => {
      mockInstances[0].dispatchEvent(new Event("open"));
    });
    expect(invalidateQueriesMock).not.toHaveBeenCalled();

    // ID_B stays, ID_C is new and has old cached data from an earlier tab.
    seedThread(ID_C, OLD_MS);
    rerender({ ids: [ID_B, ID_C] });
    await waitFor(() => expect(mockInstances).toHaveLength(2), {
      timeout: ID_SET_DEBOUNCE_MS * 10,
    });
    expect(mockInstances[0].closed).toBe(true);
    act(() => {
      mockInstances[1].dispatchEvent(new Event("open"));
    });

    expect(invalidatedKeys()).toEqual([
      // Newly added ID_C: thread revalidated at settle time (before the open).
      ["csm-case-comments", ID_C],
      ["csm-case-activities", ID_C],
      ["csm-case-comments", ID_B],
      ["csm-case-activities", ID_B],
      ["csm-case-detail", ID_B],
      ["work-item-slas", ID_B],
      ["csm-cases"],
      ["csm-case-children"],
    ]);
  });

  it("still refreshes the case lists when the id set is replaced with no id in common", async () => {
    const { rerender } = renderStream([ID_A]);
    await waitFor(() => expect(mockInstances).toHaveLength(1));
    act(() => {
      mockInstances[0].dispatchEvent(new Event("open"));
    });

    seedThread(ID_B, OLD_MS);
    rerender({ ids: [ID_B] });
    await waitFor(() => expect(mockInstances).toHaveLength(2), {
      timeout: ID_SET_DEBOUNCE_MS * 10,
    });
    act(() => {
      mockInstances[1].dispatchEvent(new Event("open"));
    });

    expect(invalidatedKeys()).toEqual([
      ["csm-case-comments", ID_B],
      ["csm-case-activities", ID_B],
      ["csm-cases"],
      ["csm-case-children"],
    ]);
  });

  describe("ids newly added to the subscribed set", () => {
    const threadKeys = (id: string) => [
      ["csm-case-comments", id],
      ["csm-case-activities", id],
    ];

    it("revalidates only comments and activities of an id that re-enters the set with old cached data", async () => {
      const { rerender } = renderStream([ID_A, ID_B]);
      await waitFor(() => expect(mockInstances).toHaveLength(1));
      // First settle: nothing to revalidate.
      expect(invalidateQueriesMock).not.toHaveBeenCalled();

      rerender({ ids: [ID_B] });
      await waitFor(() => expect(mockInstances).toHaveLength(2), {
        timeout: ID_SET_DEBOUNCE_MS * 10,
      });
      // Removing an id revalidates nothing.
      expect(invalidateQueriesMock).not.toHaveBeenCalled();

      seedThread(ID_A, OLD_MS);
      rerender({ ids: [ID_B, ID_A] });
      await waitFor(() => expect(mockInstances).toHaveLength(3), {
        timeout: ID_SET_DEBOUNCE_MS * 10,
      });
      expect(invalidatedKeys()).toEqual(threadKeys(ID_A));
    });

    it("skips a newly added id with no cached data (first-time open)", async () => {
      const { rerender } = renderStream([ID_A]);
      await waitFor(() => expect(mockInstances).toHaveLength(1));

      rerender({ ids: [ID_A, ID_B] });
      await waitFor(() => expect(mockInstances).toHaveLength(2), {
        timeout: ID_SET_DEBOUNCE_MS * 10,
      });
      expect(invalidateQueriesMock).not.toHaveBeenCalled();
    });

    it("skips a newly added id whose data was fetched moments ago", async () => {
      const { rerender } = renderStream([ID_A]);
      await waitFor(() => expect(mockInstances).toHaveLength(1));

      seedThread(ID_B, 500);
      rerender({ ids: [ID_A, ID_B] });
      await waitFor(() => expect(mockInstances).toHaveLength(2), {
        timeout: ID_SET_DEBOUNCE_MS * 10,
      });
      expect(invalidateQueriesMock).not.toHaveBeenCalled();
    });

    it("does not revalidate an id that stays subscribed", async () => {
      const { rerender } = renderStream([ID_A]);
      await waitFor(() => expect(mockInstances).toHaveLength(1));

      seedThread(ID_B, OLD_MS);
      rerender({ ids: [ID_A, ID_B] });
      await waitFor(() => expect(mockInstances).toHaveLength(2), {
        timeout: ID_SET_DEBOUNCE_MS * 10,
      });
      expect(invalidatedKeys()).toEqual(threadKeys(ID_B));
    });

    it("counts an id as re-entering after every tab was closed", async () => {
      const { rerender } = renderStream([ID_A]);
      await waitFor(() => expect(mockInstances).toHaveLength(1));

      rerender({ ids: [] });
      await waitFor(() => expect(mockInstances[0].closed).toBe(true), {
        timeout: ID_SET_DEBOUNCE_MS * 10,
      });
      expect(invalidateQueriesMock).not.toHaveBeenCalled();

      seedThread(ID_A, OLD_MS);
      rerender({ ids: [ID_A] });
      await waitFor(() => expect(mockInstances).toHaveLength(2), {
        timeout: ID_SET_DEBOUNCE_MS * 10,
      });
      expect(invalidatedKeys()).toEqual(threadKeys(ID_A));
    });

    it("does nothing while the stream is gated off", async () => {
      apiConfigMock.streamEnabled = false;
      const { rerender } = renderStream([ID_A]);
      rerender({ ids: [ID_B] });
      await act(async () => {
        await new Promise((r) => setTimeout(r, ID_SET_DEBOUNCE_MS + 100));
      });
      expect(mockInstances).toHaveLength(0);
      expect(invalidateQueriesMock).not.toHaveBeenCalled();
    });
  });

  describe("page visibility", () => {
    function setDocumentVisibility(state: "visible" | "hidden") {
      Object.defineProperty(document, "visibilityState", {
        configurable: true,
        get: () => state,
      });
      document.dispatchEvent(new Event("visibilitychange"));
    }

    afterEach(() => {
      setDocumentVisibility("visible");
    });

    it("connects even while the browser page is hidden", async () => {
      setDocumentVisibility("hidden");
      renderStream([ID_A]);
      await waitFor(() => expect(mockInstances).toHaveLength(1));
    });

    it("stays open when the page is hidden and still reports updates", async () => {
      const onCaseUpdated = vi.fn();
      renderStream([ID_A, ID_B], onCaseUpdated);
      await waitFor(() => expect(mockInstances).toHaveLength(1));

      act(() => setDocumentVisibility("hidden"));
      expect(mockInstances[0].closed).toBe(false);
      expect(mockInstances).toHaveLength(1);

      emit(mockInstances[0], JSON.stringify({ caseId: ID_B }));
      expect(onCaseUpdated).toHaveBeenCalledWith(ID_B);

      act(() => setDocumentVisibility("visible"));
      expect(mockInstances).toHaveLength(1);
    });
  });
});
