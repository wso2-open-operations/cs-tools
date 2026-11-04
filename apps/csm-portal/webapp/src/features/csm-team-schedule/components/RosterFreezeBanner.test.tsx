/**
 * Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { act, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import RosterFreezeBanner from "./RosterFreezeBanner";

const ist = (hhmm: string): Date => new Date(`2026-11-03T${hhmm}:00+05:30`);

describe("RosterFreezeBanner", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it("is not shown outside 09:15 to 09:30 IST", () => {
    vi.setSystemTime(ist("09:00"));
    render(<RosterFreezeBanner />);
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
  });

  it("says plainly not to edit the roster while it is shown", () => {
    vi.setSystemTime(ist("09:20"));
    render(<RosterFreezeBanner />);
    const bar = screen.getByRole("status");
    expect(bar).toHaveTextContent("Don’t edit the roster until 9:30 AM IST.");
    expect(bar).toHaveTextContent("The daily timesheet report is being generated from the roster (9:15 AM – 9:30 AM IST).");
  });

  it("appears at 09:15 IST and disappears at 09:30 IST on its own", () => {
    vi.setSystemTime(ist("09:14"));
    render(<RosterFreezeBanner />);
    expect(screen.queryByRole("status")).not.toBeInTheDocument();

    act(() => {
      vi.advanceTimersByTime(60_000);
    });
    expect(screen.getByRole("status")).toBeInTheDocument();

    act(() => {
      vi.advanceTimersByTime(15 * 60_000);
    });
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
  });

  it("catches up after the clock jumps, as when a laptop wakes inside the window", () => {
    vi.setSystemTime(ist("08:00"));
    render(<RosterFreezeBanner />);
    act(() => {
      vi.setSystemTime(ist("09:25"));
      vi.advanceTimersByTime(60_000);
    });
    expect(screen.getByRole("status")).toBeInTheDocument();
  });
});
