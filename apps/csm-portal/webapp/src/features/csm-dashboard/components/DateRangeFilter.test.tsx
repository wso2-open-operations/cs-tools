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

import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import DateRangeFilter from "@features/csm-dashboard/components/DateRangeFilter";

// MUI's sectioned DatePicker renders each date as separate Month/Day/Year
// spinbutton spans PLUS a mirrored `aria-hidden` plain `<input>` carrying the
// same accessible name ("From"/"To") for form-submission purposes — two
// elements sharing one label, which is why `getByLabelText` (a single-match
// query) can't be used to assert the resolved value here. Reading the
// hidden input directly (in DOM order: "From" first, "To" second, matching
// this component's own render order) is the reliable way to assert the
// resolved MM/DD/YYYY text without coupling the test to MUI's internal
// section markup.
function hiddenDateInputs(container: HTMLElement): HTMLInputElement[] {
  return Array.from(container.querySelectorAll('input[aria-hidden="true"]'));
}

describe("DateRangeFilter", () => {
  it("renders From/To fields, empty, when value carries neither bound", () => {
    const { container } = render(<DateRangeFilter value={{}} onChange={vi.fn()} />);
    const [from, to] = hiddenDateInputs(container);
    expect(from).toHaveValue("");
    expect(to).toHaveValue("");
  });

  it("displays the given from/to values formatted as MM/DD/YYYY", () => {
    const { container } = render(
      <DateRangeFilter value={{ from: "2026-07-01", to: "2026-08-15" }} onChange={vi.fn()} />,
    );
    const [from, to] = hiddenDateInputs(container);
    expect(from).toHaveValue("07/01/2026");
    expect(to).toHaveValue("08/15/2026");
  });

  it("gives the From and To fields distinct accessible names", () => {
    render(<DateRangeFilter value={{}} onChange={vi.fn()} />);
    expect(screen.getAllByLabelText("From")).toHaveLength(2);
    expect(screen.getAllByLabelText("To")).toHaveLength(2);
  });

  it("renders the default label when none is given", () => {
    render(<DateRangeFilter value={{}} onChange={vi.fn()} />);
    expect(screen.getByText("Date range")).toBeInTheDocument();
  });

  it("renders a caller-supplied label instead of the default", () => {
    render(<DateRangeFilter value={{}} onChange={vi.fn()} label="Feedback submitted" />);
    expect(screen.getByText("Feedback submitted")).toBeInTheDocument();
    expect(screen.queryByText("Date range")).not.toBeInTheDocument();
  });
});
