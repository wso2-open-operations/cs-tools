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
import { MultiSelectFilter } from "./MultiSelectFilter";

const OPTIONS = [
  { value: "a", label: "Alpha" },
  { value: "b", label: "Beta" },
];

describe("MultiSelectFilter", () => {
  it("renders its label", () => {
    render(<MultiSelectFilter id="proj" label="Project" values={[]} options={OPTIONS} onChange={vi.fn()} />);
    // MUI's outlined variant also echoes the label into the fieldset's
    // notch legend for the outline-gap effect — getAllByText covers both.
    expect(screen.getAllByText("Project").length).toBeGreaterThan(0);
  });

  it("opening it shows a checkbox per option", () => {
    render(<MultiSelectFilter id="proj" label="Project" values={[]} options={OPTIONS} onChange={vi.fn()} />);
    fireEvent.mouseDown(screen.getByRole("combobox"));
    expect(screen.getAllByRole("checkbox")).toHaveLength(2);
  });

  it("ticking an unselected option calls onChange with it added", () => {
    const onChange = vi.fn();
    render(<MultiSelectFilter id="proj" label="Project" values={["b"]} options={OPTIONS} onChange={onChange} />);
    fireEvent.mouseDown(screen.getByRole("combobox"));
    fireEvent.click(screen.getByRole("option", { name: /Alpha/ }));
    expect(onChange).toHaveBeenCalledWith(["b", "a"]);
  });

  it("unticking a selected option calls onChange with it removed", () => {
    const onChange = vi.fn();
    render(<MultiSelectFilter id="proj" label="Project" values={["a", "b"]} options={OPTIONS} onChange={onChange} />);
    fireEvent.mouseDown(screen.getByRole("combobox"));
    fireEvent.click(screen.getByRole("option", { name: /Alpha/ }));
    expect(onChange).toHaveBeenCalledWith(["b"]);
  });

  it("renders selected labels comma-joined", () => {
    render(<MultiSelectFilter id="proj" label="Project" values={["a", "b"]} options={OPTIONS} onChange={vi.fn()} />);
    expect(screen.getByText("Alpha, Beta")).toBeTruthy();
  });

  it("still shows and ticks a value absent from the loaded options", () => {
    render(<MultiSelectFilter id="proj" label="Project" values={["c"]} options={OPTIONS} onChange={vi.fn()} />);
    fireEvent.mouseDown(screen.getByRole("combobox"));
    const unknownOption = screen.getByRole("option", { name: "c" });
    const checkbox = unknownOption.querySelector('input[type="checkbox"]');
    expect(checkbox).toHaveProperty("checked", true);
  });
});
