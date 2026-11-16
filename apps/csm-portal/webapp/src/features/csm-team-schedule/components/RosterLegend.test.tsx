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

import { act, fireEvent, render } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";
import RosterLegend, { type LegendItem } from "./RosterLegend";
import { ENGAGEMENT_LEGEND_ITEM, ENGAGEMENT_MARK_LEGEND_ITEM } from "../utils/engagementAllocations";

const ITEMS: LegendItem[] = [
  { key: "s|LK", group: "Hours", code: "LK", token: "LK", label: "Regular hours" },
  { key: "k|AL", group: "Leave", code: "AL", token: "AL", label: "Annual leave" },
];

const STORE_KEY = "csm-ts.roster-legend.open";

/** Open or fold the legend the way a click on its summary does: the browser
 *  flips the element's open state, then fires toggle. */
function toggle(details: HTMLDetailsElement): void {
  act(() => {
    details.open = !details.open;
    fireEvent(details, new Event("toggle"));
  });
}

function legend(container: HTMLElement): HTMLDetailsElement {
  return container.querySelector("details.rlegend") as HTMLDetailsElement;
}

// Node 26 exposes `window.localStorage` as undefined unless the process was
// started with `--localstorage-file`, which hides jsdom's own Storage. These
// tests only need a per-test key/value store.
function installLocalStorage(): void {
  if (typeof window.localStorage?.clear === "function") return;
  const store = new Map<string, string>();
  Object.defineProperty(window, "localStorage", {
    configurable: true,
    value: {
      getItem: (key: string) => store.get(key) ?? null,
      setItem: (key: string, value: string) => {
        store.set(key, String(value));
      },
      removeItem: (key: string) => {
        store.delete(key);
      },
      clear: () => {
        store.clear();
      },
    },
  });
}

beforeEach(() => {
  installLocalStorage();
  window.localStorage.clear();
});

describe("RosterLegend", () => {
  it("opens the first time", () => {
    const { container } = render(<RosterLegend items={ITEMS} />);
    expect(legend(container).open).toBe(true);
  });

  it("stays folded once folded, and open once opened again", () => {
    const first = render(<RosterLegend items={ITEMS} />);
    toggle(legend(first.container));
    expect(window.localStorage.getItem(STORE_KEY)).toBe("0");
    first.unmount();

    const second = render(<RosterLegend items={ITEMS} />);
    expect(legend(second.container).open).toBe(false);
    toggle(legend(second.container));
    expect(window.localStorage.getItem(STORE_KEY)).toBe("1");
    second.unmount();

    const third = render(<RosterLegend items={ITEMS} />);
    expect(legend(third.container).open).toBe(true);
  });

  it("lists a customer engagement as its own group, with the bar's swatch rather than a code chip", () => {
    const { container } = render(<RosterLegend items={[...ITEMS, ENGAGEMENT_LEGEND_ITEM]} />);
    const heads = [...container.querySelectorAll(".rl-h")].map((h) => h.textContent);
    expect(heads).toEqual(["Hours", "Leave", "Engagements"]);
    expect(container.querySelectorAll(".engsw")).toHaveLength(1);
    expect(container.querySelectorAll(".rl-g li .chip")).toHaveLength(2);
  });

  it("lists the month roster's engagement as an Allo- chip, not the week bar's swatch", () => {
    const { container } = render(<RosterLegend items={[...ITEMS, ENGAGEMENT_MARK_LEGEND_ITEM]} />);
    const heads = [...container.querySelectorAll(".rl-h")].map((h) => h.textContent);
    expect(heads).toEqual(["Hours", "Leave", "Engagements"]);
    const sw = container.querySelectorAll(".engmksw");
    expect(sw).toHaveLength(1);
    expect(sw[0].className).toContain("chip sm eng");
    expect(sw[0].textContent).toBe("Allo-…");
    expect(container.querySelectorAll(".engsw")).toHaveLength(0);
  });
});
