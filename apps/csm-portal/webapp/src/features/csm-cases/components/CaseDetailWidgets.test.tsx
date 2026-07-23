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
import { TagsWidget } from "@features/csm-cases/components/CaseDetailWidgets";
import type { CaseTag } from "@features/csm-cases/types/csmCases";

const TAGS: CaseTag[] = [
  { id: "tag-1", label: "micro-gw" },
  { id: "tag-2", label: "ws-policy" },
];

describe("TagsWidget", () => {
  it("renders an empty state when there are no tags", () => {
    render(<TagsWidget tags={[]} />);
    expect(screen.getByText("No tags applied.")).toBeInTheDocument();
  });

  it("renders every tag as a chip", () => {
    render(<TagsWidget tags={TAGS} />);
    expect(screen.getByText("micro-gw")).toBeInTheDocument();
    expect(screen.getByText("ws-policy")).toBeInTheDocument();
  });

  it("calls onAdd when the Tag button is clicked", () => {
    const onAdd = vi.fn();
    render(<TagsWidget tags={TAGS} onAdd={onAdd} />);
    fireEvent.click(screen.getByRole("button", { name: /^tag$/i }));
    expect(onAdd).toHaveBeenCalled();
  });

  it("calls onRemove with the tag when its chip delete icon is clicked", () => {
    const onRemove = vi.fn();
    render(<TagsWidget tags={TAGS} onRemove={onRemove} />);
    const chip = screen.getByText("micro-gw").closest(".MuiChip-root");
    const deleteIcon = chip?.querySelector(".MuiChip-deleteIcon");
    expect(deleteIcon).toBeTruthy();
    fireEvent.click(deleteIcon as Element);
    expect(onRemove).toHaveBeenCalledWith(TAGS[0]);
  });

  it("omits the delete affordance when onRemove is not provided", () => {
    render(<TagsWidget tags={TAGS} />);
    const chip = screen.getByText("micro-gw").closest(".MuiChip-root");
    expect(chip?.querySelector(".MuiChip-deleteIcon")).toBeFalsy();
  });
});
