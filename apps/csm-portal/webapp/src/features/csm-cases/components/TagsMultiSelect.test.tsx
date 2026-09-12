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
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import TagsMultiSelect from "@features/csm-cases/components/TagsMultiSelect";
import { useSearchTags } from "@features/csm-cases/api/useSearchTags";

// Mocked directly (same approach CasesFilterBar.test.tsx / AddTagDialog.test.tsx
// use) so tests don't have to drive the real 300ms debounce.
vi.mock("@features/csm-cases/api/useSearchTags", () => ({
  useSearchTags: vi.fn(),
}));
const mockedUseSearchTags = vi.mocked(useSearchTags);

function mockTagSearchResult(
  overrides: Partial<ReturnType<typeof useSearchTags>>,
): void {
  mockedUseSearchTags.mockReturnValue({
    data: [],
    isFetching: false,
    isError: false,
    ...overrides,
  } as unknown as ReturnType<typeof useSearchTags>);
}

beforeEach(() => {
  mockTagSearchResult({});
});

function renderControl(
  props: Partial<Parameters<typeof TagsMultiSelect>[0]> = {},
): { onChange: ReturnType<typeof vi.fn> } {
  const onChange = vi.fn();
  render(
    <TagsMultiSelect
      includedValues={[]}
      excludedValues={[]}
      onChange={onChange}
      {...props}
    />,
  );
  return { onChange };
}

describe("TagsMultiSelect — tri-state cycling", () => {
  it("cycles unselected -> included -> excluded -> unselected on repeated clicks", () => {
    mockTagSearchResult({ data: [{ id: "t1", label: "urgent" }] });

    // Step 1: unselected -> included.
    const step1 = renderControl();
    fireEvent.mouseDown(screen.getByRole("combobox"));
    fireEvent.click(screen.getByText("urgent"));
    expect(step1.onChange).toHaveBeenCalledWith({ included: ["urgent"], excluded: [] });
    step1.onChange.mockClear();

    // Step 2: included -> excluded (re-render with the resulting state, as
    // the real parent would after applying step 1's onChange).
    const step2 = renderControl({ includedValues: ["urgent"] });
    fireEvent.mouseDown(screen.getAllByRole("combobox")[1]);
    fireEvent.click(screen.getAllByText("urgent").at(-1)!);
    expect(step2.onChange).toHaveBeenCalledWith({ included: [], excluded: ["urgent"] });

    // Step 3: excluded -> unselected.
    const step3 = renderControl({ excludedValues: ["urgent"] });
    fireEvent.mouseDown(screen.getAllByRole("combobox").at(-1)!);
    fireEvent.click(screen.getAllByText("urgent").at(-1)!);
    expect(step3.onChange).toHaveBeenCalledWith({ included: [], excluded: [] });
  });

  it("a freshly typed free-text tag (no matching suggestion) starts at included", () => {
    const { onChange } = renderControl();

    const input = screen.getByRole("combobox");
    fireEvent.change(input, { target: { value: "brand-new-tag" } });
    fireEvent.keyDown(input, { key: "Enter" });

    expect(onChange).toHaveBeenCalledWith({ included: ["brand-new-tag"], excluded: [] });
  });

  it("cycling one tag does not touch other already-selected tags", () => {
    mockTagSearchResult({ data: [{ id: "t1", label: "second" }] });
    const { onChange } = renderControl({
      includedValues: ["first-included"],
      excludedValues: ["first-excluded"],
    });

    fireEvent.mouseDown(screen.getByRole("combobox"));
    fireEvent.click(screen.getByText("second"));

    expect(onChange).toHaveBeenCalledWith({
      included: ["first-included", "second"],
      excluded: ["first-excluded"],
    });
  });

  // Regression: MUI reports the same "removeOption" onChange reason for
  // Enter-on-a-highlighted-already-selected-option as it does for a genuine
  // chip removal (Backspace/clear) -- the component used to treat both as a
  // full removal, silently dropping an included tag instead of cycling it to
  // excluded the way a mouse click on that same row does.
  it("pressing Enter on a keyboard-highlighted included option cycles it to excluded, not removes it", () => {
    mockTagSearchResult({ data: [{ id: "t1", label: "urgent" }] });
    const { onChange } = renderControl({ includedValues: ["urgent"] });

    const input = screen.getByRole("combobox");
    fireEvent.mouseDown(input);
    fireEvent.keyDown(input, { key: "ArrowDown" });
    fireEvent.keyDown(input, { key: "Enter" });

    expect(onChange).toHaveBeenCalledWith({ included: [], excluded: ["urgent"] });
  });
});

describe("TagsMultiSelect — chip rendering", () => {
  it("renders an included tag as a '+' prefixed neutral chip", () => {
    renderControl({ includedValues: ["urgent"] });
    const chip = screen.getByText("+ urgent").closest(".MuiChip-root");
    expect(chip).toBeInTheDocument();
    expect(chip).not.toHaveClass("MuiChip-colorError");
  });

  it("renders an excluded tag as a '-' prefixed error-tinted chip", () => {
    renderControl({ excludedValues: ["spam"] });
    const chip = screen.getByText("- spam").closest(".MuiChip-root");
    expect(chip).toBeInTheDocument();
    expect(chip).toHaveClass("MuiChip-colorError");
  });

  it("collapses to a fixed-width 'N tags' summary once more than one tag is selected, instead of rendering every chip individually", () => {
    // Real bug this guards: the control sits in a narrow filter-bar column
    // that can't fit two real Chip pills without the row's own
    // `overflow: hidden` silently clipping the second one (only the first,
    // partially truncated, stayed visible) -- reproduced with
    // tags=patch&excludeTags=am. A fixed-width summary chip can't overflow
    // regardless of label length or how many tags are selected.
    renderControl({ includedValues: ["urgent"], excludedValues: ["spam"] });
    expect(screen.getByText("2 tags")).toBeInTheDocument();
    expect(screen.queryByText("+ urgent")).not.toBeInTheDocument();
    expect(screen.queryByText("- spam")).not.toBeInTheDocument();
  });

  it("deleting the single selected chip clears it via onChange", () => {
    const { onChange } = renderControl({ includedValues: ["urgent"] });

    const chip = screen.getByText("+ urgent").closest(".MuiChip-root")!;
    fireEvent.click(chip.querySelector("svg")!);

    expect(onChange).toHaveBeenCalledWith({ included: [], excluded: [] });
  });
});
