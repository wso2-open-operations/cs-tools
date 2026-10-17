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

import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import "@testing-library/jest-dom/vitest";
import ScheduleSkeleton, { InlineSkeleton, TabStripSkeleton } from "./ScheduleSkeleton";

describe("ScheduleSkeleton: a view drawn in its own shape while it loads", () => {
  it("says what is loading, once, to a screen reader and on screen", () => {
    render(<ScheduleSkeleton variant="roster" label="Loading the rota…" />);
    const status = screen.getByRole("status");
    // Not aria-busy: that would hold the announcement until a false state the
    // skeleton never reaches before it is replaced.
    expect(status).not.toHaveAttribute("aria-busy");
    expect(status).toHaveTextContent("Loading the rota…");
    // The placeholder shapes are decoration, not content.
    expect(status.querySelector(".sk-body")).toHaveAttribute("aria-hidden", "true");
  });

  it("draws the roster as a row of day cells per person, a month across", () => {
    const { container } = render(<ScheduleSkeleton variant="roster" label="Loading" rows={3} />);
    expect(container.querySelectorAll(".sk-rhead .sk-dayhd")).toHaveLength(31);
    expect(container.querySelectorAll(".sk-rrow")).toHaveLength(3);
    expect(container.querySelectorAll(".sk-rrow")[0].querySelectorAll(".sk-cell")).toHaveLength(31);
  });

  it("draws the week as seven days", () => {
    const { container } = render(<ScheduleSkeleton variant="week" label="Loading" rows={2} />);
    expect(container.querySelectorAll(".sk-weekhd")).toHaveLength(7);
    expect(container.querySelectorAll(".sk-weekrow")).toHaveLength(2);
  });

  it("draws today as ladder lanes and My week as day cards", () => {
    const today = render(<ScheduleSkeleton variant="today" label="Loading" />);
    expect(today.container.querySelectorAll(".sk-lane").length).toBeGreaterThan(1);
    today.unmount();
    const mine = render(<ScheduleSkeleton variant="mine" label="Loading" />);
    expect(mine.container.querySelectorAll(".sk-daycard")).toHaveLength(7);
  });

  it("draws a list with the rows asked for", () => {
    const { container } = render(<ScheduleSkeleton variant="list" label="Loading" rows={5} />);
    expect(container.querySelectorAll(".sk-item")).toHaveLength(5);
  });
});

describe("TabStripSkeleton and InlineSkeleton", () => {
  it("holds the tab strip's place with tabs no screen reader reads", () => {
    const { container } = render(
      <div role="tablist">
        <TabStripSkeleton count={4} />
      </div>,
    );
    const tabs = container.querySelectorAll(".sk-tab");
    expect(tabs).toHaveLength(4);
    tabs.forEach((t) => expect(t).toHaveAttribute("aria-hidden", "true"));
    expect(screen.queryAllByRole("tab")).toHaveLength(0);
  });

  it("keeps the words of a loading line for a screen reader", () => {
    render(<InlineSkeleton label="Looking…" />);
    expect(screen.getByRole("status")).toHaveTextContent("Looking…");
  });
});
