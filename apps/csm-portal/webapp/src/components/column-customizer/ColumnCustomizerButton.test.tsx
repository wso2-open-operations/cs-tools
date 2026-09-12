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
import ColumnCustomizerButton from "@components/column-customizer/ColumnCustomizerButton";
import type { ColumnOption } from "@hooks/useColumnPreferences";

const COLUMNS: ColumnOption[] = [
  { id: "a", label: "Column A" },
  { id: "b", label: "Column B" },
  { id: "c", label: "Column C" },
];

/** A minimal `DataTransfer` stand-in — jsdom doesn't implement the real
 * thing, and the component only ever calls `setData` and assigns
 * `effectAllowed`. */
function fakeDataTransfer(): DataTransfer {
  return { setData: vi.fn(), effectAllowed: "" } as unknown as DataTransfer;
}

function setup(visible: string[] = ["a", "b", "c"]) {
  const onToggle = vi.fn();
  const onMove = vi.fn();
  const onReorder = vi.fn();
  const onReset = vi.fn();
  render(
    <ColumnCustomizerButton
      allColumns={COLUMNS}
      isVisible={(id) => visible.includes(id)}
      onToggle={onToggle}
      onMove={onMove}
      onReorder={onReorder}
      onReset={onReset}
    />,
  );
  return { onToggle, onMove, onReorder, onReset };
}

describe("ColumnCustomizerButton", () => {
  it("opens the popover listing every known column on trigger click", () => {
    setup();
    fireEvent.click(screen.getByRole("button", { name: "Customise columns" }));

    expect(screen.getByText("Column A")).toBeInTheDocument();
    expect(screen.getByText("Column B")).toBeInTheDocument();
    expect(screen.getByText("Column C")).toBeInTheDocument();
  });

  it("calls onToggle when a column row is clicked", () => {
    const { onToggle } = setup();
    fireEvent.click(screen.getByRole("button", { name: "Customise columns" }));

    fireEvent.click(screen.getByText("Column B"));
    expect(onToggle).toHaveBeenCalledWith("b");
  });

  it("calls onMove when the up/down arrow keys are pressed on a column's reorder handle, without toggling that row", () => {
    const { onMove, onToggle } = setup();
    fireEvent.click(screen.getByRole("button", { name: "Customise columns" }));

    const handle = screen.getByRole("button", { name: /Reorder Column B/ });
    fireEvent.keyDown(handle, { key: "ArrowUp" });
    expect(onMove).toHaveBeenCalledWith("b", "up");

    fireEvent.keyDown(handle, { key: "ArrowDown" });
    expect(onMove).toHaveBeenCalledWith("b", "down");

    expect(onToggle).not.toHaveBeenCalled();
  });

  it("calls onReorder with the dragged column's id and the target row's index when dropped", () => {
    const { onReorder } = setup();
    fireEvent.click(screen.getByRole("button", { name: "Customise columns" }));

    const dragHandleA = screen.getByRole("button", { name: /Reorder Column A/ });
    const rowC = screen.getByText("Column C").closest("li")!;

    const dataTransfer = fakeDataTransfer();
    fireEvent.dragStart(dragHandleA, { dataTransfer });
    fireEvent.dragOver(rowC, { dataTransfer });
    fireEvent.drop(rowC, { dataTransfer });

    // "c" is at index 2 in `allColumns`.
    expect(onReorder).toHaveBeenCalledWith("a", 2);
  });

  it("does not call onReorder when a column is dragged and dropped on itself", () => {
    const { onReorder } = setup();
    fireEvent.click(screen.getByRole("button", { name: "Customise columns" }));

    const dragHandleA = screen.getByRole("button", { name: /Reorder Column A/ });
    const rowA = screen.getByText("Column A").closest("li")!;

    const dataTransfer = fakeDataTransfer();
    fireEvent.dragStart(dragHandleA, { dataTransfer });
    fireEvent.dragOver(rowA, { dataTransfer });
    fireEvent.drop(rowA, { dataTransfer });

    expect(onReorder).not.toHaveBeenCalled();
  });

  it("disables unchecking the last remaining visible column", () => {
    setup(["a"]);
    fireEvent.click(screen.getByRole("button", { name: "Customise columns" }));

    const checkboxes = screen.getAllByRole("checkbox");
    expect(checkboxes[0]).toBeDisabled();
  });

  it("calls onReset when Reset to default is clicked", () => {
    const { onReset } = setup();
    fireEvent.click(screen.getByRole("button", { name: "Customise columns" }));

    fireEvent.click(screen.getByText("Reset to default"));
    expect(onReset).toHaveBeenCalled();
  });
});
