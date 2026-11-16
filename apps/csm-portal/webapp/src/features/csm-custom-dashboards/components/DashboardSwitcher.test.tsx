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

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import DashboardSwitcher, {
  type DashboardSwitcherEntry,
} from "@features/csm-custom-dashboards/components/DashboardSwitcher";

const ENTRIES: DashboardSwitcherEntry[] = [
  { id: "a", name: "Alpha" },
  { id: "b", name: "Beta" },
  { id: "code:c", name: "Cases Code", group: "Advanced" },
  { id: "code:d", name: "Delta Code", group: "Advanced" },
];

function open(entries: DashboardSwitcherEntry[] = ENTRIES): void {
  render(<DashboardSwitcher entries={entries} selectedId="a" onSelect={vi.fn()} />);
  fireEvent.mouseDown(screen.getByRole("combobox", { name: "Select dashboard" }));
}

/** The menu's group headings and entries in order ("# heading"), the search box left out. */
function lines(): string[] {
  return Array.from(screen.getByRole("listbox").children)
    .filter((el) => !el.querySelector("input"))
    .map((el) =>
      el.classList.contains("MuiListSubheader-root") ? `# ${el.textContent}` : (el.querySelector("span")?.textContent ?? ""),
    );
}

function heading(name: string): HTMLElement {
  const el = Array.from(screen.getByRole("listbox").children).find(
    (c) => c.classList.contains("MuiListSubheader-root") && c.textContent === name,
  );
  if (!el) throw new Error(`no heading ${name}`);
  return el as HTMLElement;
}

describe("DashboardSwitcher groups", () => {
  it("shows a group's heading once, before its first entry", () => {
    open();
    expect(lines()).toEqual(["Alpha", "Beta", "# Advanced", "Cases Code", "Delta Code"]);
  });

  it("shows no heading when no entry has a group", () => {
    open(ENTRIES.slice(0, 2));
    expect(lines()).toEqual(["Alpha", "Beta"]);
  });

  it("hides a heading while none of its entries match the search, and shows it while one does", () => {
    open();
    const search = screen.getByRole("textbox", { name: "Search dashboards" });
    fireEvent.change(search, { target: { value: "alpha" } });
    expect(heading("Advanced")).not.toBeVisible();
    fireEvent.change(search, { target: { value: "delta" } });
    expect(heading("Advanced")).toBeVisible();
  });

  it("picks a grouped entry by its id", () => {
    const onSelect = vi.fn();
    render(<DashboardSwitcher entries={ENTRIES} selectedId="a" onSelect={onSelect} />);
    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Select dashboard" }));
    fireEvent.click(screen.getByRole("option", { name: /Delta Code/ }));
    expect(onSelect).toHaveBeenCalledWith("code:d");
  });
});
