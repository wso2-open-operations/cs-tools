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

import { renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  PAGE_RELOAD_CHECK_INTERVAL_MS,
  usePeriodicPageReload,
} from "@features/csm-dashboard/hooks/usePeriodicPageReload";
import { CS_MONITOR_PAGE_RELOAD_INTERVAL_MS } from "@features/csm-dashboard/utils/wallboardMetricStyle";

const HOUR = 60 * 60 * 1000;

describe("usePeriodicPageReload", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(2026, 10, 3, 9, 0, 0));
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it("reloads the CS Monitor page every 60 minutes", () => {
    expect(CS_MONITOR_PAGE_RELOAD_INTERVAL_MS).toBe(HOUR);
  });

  it("reloads once an hour has passed since load, not before", () => {
    const reload = vi.fn();
    renderHook(() => usePeriodicPageReload(HOUR, reload));

    vi.advanceTimersByTime(HOUR - PAGE_RELOAD_CHECK_INTERVAL_MS);
    expect(reload).not.toHaveBeenCalled();

    vi.advanceTimersByTime(PAGE_RELOAD_CHECK_INTERVAL_MS);
    expect(reload).toHaveBeenCalledTimes(1);

    // Only once per page load — the reload itself starts the next hour.
    vi.advanceTimersByTime(HOUR);
    expect(reload).toHaveBeenCalledTimes(1);
  });

  it("is not pushed back by re-renders with a fresh reload function", () => {
    // The real call site passes a new default `reload` arrow every render;
    // re-arming on that would keep moving the deadline and never reload.
    const first = vi.fn();
    const { rerender } = renderHook(({ fn }) => usePeriodicPageReload(HOUR, fn), {
      initialProps: { fn: first },
    });
    for (let i = 0; i < 9; i += 1) {
      vi.advanceTimersByTime(HOUR / 10);
      rerender({ fn: vi.fn() });
    }
    expect(first).not.toHaveBeenCalled();
    vi.advanceTimersByTime(HOUR / 10); // exactly one hour since load
    expect(first).toHaveBeenCalledTimes(1);
  });

  it("reloads at the first check after the machine wakes from sleeping past the hour", () => {
    const reload = vi.fn();
    renderHook(() => usePeriodicPageReload(HOUR, reload));
    vi.setSystemTime(new Date(2026, 10, 3, 11, 0, 0)); // slept 2h
    vi.advanceTimersByTime(PAGE_RELOAD_CHECK_INTERVAL_MS);
    expect(reload).toHaveBeenCalledTimes(1);
  });

  it("stops checking after unmount", () => {
    const reload = vi.fn();
    const { unmount } = renderHook(() => usePeriodicPageReload(HOUR, reload));
    unmount();
    vi.advanceTimersByTime(2 * HOUR);
    expect(reload).not.toHaveBeenCalled();
  });
});
