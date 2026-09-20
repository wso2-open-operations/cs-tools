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
import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { SyncButton } from "./SyncButton";

function jsonResponse(body: unknown): Response {
  return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" } });
}

function renderSyncButton() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <SyncButton />
    </QueryClientProvider>,
  );
}

describe("SyncButton", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    // @ts-expect-error -- partial config is fine for this test
    window.config = { GID_BACKEND_BASE_URL: "https://backend.example.test" };
  });

  afterEach(() => {
    vi.runOnlyPendingTimers();
    vi.useRealTimers();
    vi.unstubAllGlobals();
    // @ts-expect-error -- test-only cleanup of the global window.config
    delete window.config;
  });

  it("reverts the post-sync summary back to the last-synced text after a few seconds", async () => {
    const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.includes("/sync/status")) {
        return Promise.resolve(jsonResponse({ running: false, repos: [{ repo: "a/b", lastSyncedAt: null }], lastRun: null }));
      }
      if (url.includes("/sync/runs") && init?.method === "POST") {
        return Promise.resolve(
          jsonResponse({
            startedAt: "2026-01-01T00:00:00Z",
            finishedAt: "2026-01-01T00:00:01Z",
            repos: [{ repo: "a/b", status: "success", issuesProcessed: 14, eventsInserted: 14 }],
          }),
        );
      }
      return Promise.reject(new Error(`unexpected fetch: ${url}`));
    });
    vi.stubGlobal("fetch", fetchMock);

    renderSyncButton();

    await act(async () => {
      await vi.runOnlyPendingTimersAsync();
    });

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Sync now" }));
      await vi.advanceTimersByTimeAsync(0);
    });

    expect(screen.getByText("Synced — 14 issues, 14 events")).toBeTruthy();

    await act(async () => {
      vi.advanceTimersByTime(5000);
      await vi.runOnlyPendingTimersAsync();
    });

    expect(screen.queryByText("Synced — 14 issues, 14 events")).toBeNull();
    expect(screen.getByText(/Last synced/)).toBeTruthy();
  });
});
