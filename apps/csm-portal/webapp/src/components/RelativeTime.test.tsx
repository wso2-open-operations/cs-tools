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

import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import RelativeTime from "@components/RelativeTime";

describe("RelativeTime", () => {
  const iso = new Date(Date.now() - 60_000).toISOString();

  beforeEach(() => {
    Object.assign(navigator, {
      clipboard: { writeText: vi.fn().mockResolvedValue(undefined) },
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("renders plain (non-permalink) text with no copy button when href is absent", () => {
    render(<RelativeTime iso={iso} />);
    expect(
      screen.queryByRole("button", { name: /copy link/i }),
    ).not.toBeInTheDocument();
  });

  it("renders a permalink anchor and a copy-link button when href is provided", () => {
    render(<RelativeTime iso={iso} href="#cmt-1001-2" />);
    expect(screen.getByRole("link")).toHaveAttribute("href", "#cmt-1001-2");
    expect(
      screen.getByRole("button", { name: /copy link to this entry/i }),
    ).toBeInTheDocument();
  });

  it("copies the absolute permalink URL to the clipboard and shows a transient confirmation", async () => {
    render(<RelativeTime iso={iso} href="#cmt-1001-2" />);

    const button = screen.getByRole("button", {
      name: /copy link to this entry/i,
    });
    fireEvent.click(button);

    await waitFor(() => {
      expect(navigator.clipboard.writeText).toHaveBeenCalledWith(
        `${window.location.origin}${window.location.pathname}#cmt-1001-2`,
      );
    });

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /copied/i })).toBeInTheDocument();
    });
  });

  it("does not mark the link copied when the clipboard write is rejected", async () => {
    Object.assign(navigator, {
      clipboard: { writeText: vi.fn().mockRejectedValue(new Error("denied")) },
    });
    render(<RelativeTime iso={iso} href="#cmt-1001-2" />);

    const button = screen.getByRole("button", {
      name: /copy link to this entry/i,
    });
    fireEvent.click(button);

    await waitFor(() => {
      expect(navigator.clipboard.writeText).toHaveBeenCalledTimes(1);
    });

    // Give the rejected promise a tick to settle, then confirm no "Copied"
    // state was set — the button keeps its original label/icon.
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(
      screen.getByRole("button", { name: /copy link to this entry/i }),
    ).toBeInTheDocument();
  });

  it("does nothing when the Clipboard API is unavailable", () => {
    Object.assign(navigator, { clipboard: undefined });
    render(<RelativeTime iso={iso} href="#cmt-1001-2" />);

    const button = screen.getByRole("button", {
      name: /copy link to this entry/i,
    });
    expect(() => fireEvent.click(button)).not.toThrow();
  });

  it("updates the displayed relative text on its own, without a re-render trigger from the parent", () => {
    vi.useFakeTimers();
    try {
      const start = new Date("2026-08-22T10:00:00.000Z");
      vi.setSystemTime(start);
      const fixedIso = new Date(start.getTime() - 60_000).toISOString(); // "1m ago"

      render(<RelativeTime iso={fixedIso} />);
      expect(screen.getByText("1m ago")).toBeInTheDocument();

      // Advance the clock by 6 minutes' worth of real time with no parent
      // re-render and no user interaction — only the shared scheduler's
      // internal timer(s) fire. `advanceTimersByTime` also advances the fake
      // system clock itself, so this alone moves both "now" and the
      // scheduled fire(s) forward together.
      act(() => {
        vi.advanceTimersByTime(6 * 60_000);
      });

      expect(screen.getByText("7m ago")).toBeInTheDocument();
    } finally {
      vi.useRealTimers();
    }
  });

  it("schedules exactly one adaptive timer, firing at the next minute boundary rather than a fixed interval", () => {
    vi.useFakeTimers();
    try {
      const start = new Date("2026-08-22T10:00:00.000Z");
      vi.setSystemTime(start);
      // 58s ago: still "just now" (< 1 minute), and only 2s from crossing
      // into the "1m ago" bucket.
      const fixedIso = new Date(start.getTime() - 58_000).toISOString();

      render(<RelativeTime iso={fixedIso} />);
      expect(screen.getByText("just now")).toBeInTheDocument();
      // One timestamp mounted -> exactly one scheduled timer, not a fixed
      // fixed-cadence poller.
      expect(vi.getTimerCount()).toBe(1);

      // Advancing by less than the 2s remaining must not flip the text yet
      // — proves the scheduler isn't just waiting a fixed 20s (or any other
      // arbitrary cadence) and firing early/late relative to the boundary.
      act(() => {
        vi.advanceTimersByTime(1_000);
      });
      expect(screen.getByText("just now")).toBeInTheDocument();

      // Crossing the remaining ~1s past the boundary flips it to "1m ago".
      act(() => {
        vi.advanceTimersByTime(1_500);
      });
      expect(screen.getByText("1m ago")).toBeInTheDocument();
    } finally {
      vi.useRealTimers();
    }
  });

  it("does not schedule frequent wakeups for an old timestamp already in the day bucket", () => {
    vi.useFakeTimers();
    try {
      const start = new Date("2026-08-22T10:00:00.000Z");
      vi.setSystemTime(start);
      const fixedIso = new Date(
        start.getTime() - 3 * 24 * 60 * 60_000,
      ).toISOString(); // "3d ago"

      render(<RelativeTime iso={fixedIso} />);
      expect(screen.getByText("3d ago")).toBeInTheDocument();
      expect(vi.getTimerCount()).toBe(1);

      // A "3d ago" display only changes on the next full day boundary, so an
      // hour of elapsed time — several multiples of the old 20s fixed
      // interval — must not touch the displayed text at all.
      act(() => {
        vi.advanceTimersByTime(60 * 60_000);
      });
      expect(screen.getByText("3d ago")).toBeInTheDocument();
    } finally {
      vi.useRealTimers();
    }
  });

  it("does not show a stale/negative ('from now') time when a fresh timestamp registers after time has passed with nothing tracked", () => {
    // Reproduces the real trigger: RefreshButton/CaseSlaTable mount with no
    // `updatedAt` yet (nothing registered with the scheduler, so `now`
    // never ticks), then later re-render with a brand-new "right now"
    // timestamp once a refresh completes — e.g. a user clicking refresh a
    // couple of minutes after page load.
    vi.useFakeTimers();
    try {
      const start = new Date("2026-08-22T10:00:00.000Z");
      vi.setSystemTime(start);

      const { rerender } = render(<RelativeTime iso={null} />);
      // Nothing tracked yet -> no scheduler registration, no pending timer.
      expect(vi.getTimerCount()).toBe(0);

      // Two minutes pass with nothing registered, so the shared scheduler
      // never fires and this instance's `now` state is never refreshed.
      act(() => {
        vi.advanceTimersByTime(2 * 60_000);
      });

      // A fresh timestamp ("now", from the advanced clock's perspective)
      // registers — e.g. a refresh completing right now.
      const freshIso = new Date().toISOString();
      rerender(<RelativeTime iso={freshIso} />);

      // Must read "just now" immediately, not a transient "0m from now"
      // computed against the stale `now` left over from mount.
      expect(screen.getByText("just now")).toBeInTheDocument();
    } finally {
      vi.useRealTimers();
    }
  });

  it("clears its scheduler registration on unmount, leaving no pending timer once nothing is mounted", () => {
    vi.useFakeTimers();
    try {
      const start = new Date("2026-08-22T10:00:00.000Z");
      vi.setSystemTime(start);
      const fixedIso = new Date(start.getTime() - 60_000).toISOString();

      const { unmount } = render(<RelativeTime iso={fixedIso} />);
      expect(vi.getTimerCount()).toBe(1);

      unmount();

      expect(vi.getTimerCount()).toBe(0);
    } finally {
      vi.useRealTimers();
    }
  });
});
