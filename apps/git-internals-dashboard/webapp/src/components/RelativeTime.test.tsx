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

import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { RelativeTime } from "./RelativeTime";

const TICK_MS = 60_000;

describe("RelativeTime", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it("shows the relative text derived from now", () => {
    const iso = new Date(Date.now() - 5 * 60_000).toISOString();
    render(<RelativeTime iso={iso} />);
    expect(screen.getByText(/^\d+m ago$/)).toBeTruthy();
  });

  it("shows the tooltip with the absolute timestamp on hover", async () => {
    render(<RelativeTime iso="2026-01-01T09:00:00Z" />);
    fireEvent.mouseOver(screen.getByText(/ago|from now|just now/));
    const tooltip = await screen.findByRole("tooltip");
    expect(tooltip.textContent).toContain("Jan 1, 2026");
  });

  it("renders no tooltip when iso itself is null", () => {
    render(<RelativeTime iso={null} />);
    expect(screen.getByText("—")).toBeTruthy();
    fireEvent.mouseOver(screen.getByText("—"));
    expect(screen.queryByRole("tooltip")).toBeNull();
  });

  it("advances the displayed text as the shared ticker ticks a minute forward", () => {
    vi.useFakeTimers();
    const start = Date.now();
    const iso = new Date(start - 30_000).toISOString();

    render(<RelativeTime iso={iso} />);
    // The shared ticker's module-level "now" is set at whenever this module
    // was first imported, not this test's fake clock — one tick resyncs it.
    act(() => {
      vi.advanceTimersByTime(TICK_MS);
    });
    const firstText = screen.getByText(/^\d+m ago$/).textContent!;
    const firstMinutes = Number(/\d+/.exec(firstText)![0]);

    act(() => {
      vi.advanceTimersByTime(TICK_MS);
    });
    const secondText = screen.getByText(/^\d+m ago$/).textContent!;
    const secondMinutes = Number(/\d+/.exec(secondText)![0]);

    expect(secondMinutes).toBeGreaterThan(firstMinutes);
  });
});
