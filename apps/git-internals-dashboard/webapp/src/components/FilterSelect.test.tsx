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

import { MenuItem } from "@mui/material";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { FilterSelect } from "./FilterSelect";

const FIXED_OPTIONS = [
  <MenuItem key="a" value="a">A</MenuItem>,
  <MenuItem key="b" value="b">B</MenuItem>,
];

describe("FilterSelect", () => {
  it("renders an extra option for a value not among its children, with no out-of-range warning", () => {
    const consoleError = vi.spyOn(console, "error").mockImplementation(() => {});

    render(
      <FilterSelect value="ghost-team" onChange={() => {}}>
        {FIXED_OPTIONS}
      </FilterSelect>,
    );

    // The synthesized MenuItem keeps the closed select showing the current
    // value instead of MUI falling back to a blank selection.
    expect(screen.getByRole("combobox")).toHaveTextContent("ghost-team");
    expect(consoleError).not.toHaveBeenCalledWith(expect.stringContaining("out-of-range value"));

    consoleError.mockRestore();
  });

  it("does not synthesize an extra option when the value is among its children", () => {
    render(
      <FilterSelect value="a" onChange={() => {}}>
        {FIXED_OPTIONS}
      </FilterSelect>,
    );

    expect(screen.getAllByText("A")).toHaveLength(1);
  });

  it("never synthesizes an extra option for the \"all\" sentinel value, even if no child has it", () => {
    const consoleError = vi.spyOn(console, "error").mockImplementation(() => {});

    render(
      <FilterSelect value="all" onChange={() => {}}>
        {FIXED_OPTIONS}
      </FilterSelect>,
    );

    expect(screen.queryByText("all")).toBeNull();

    consoleError.mockRestore();
  });
});
