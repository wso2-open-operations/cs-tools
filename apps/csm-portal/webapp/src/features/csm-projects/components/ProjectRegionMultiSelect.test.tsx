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


import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import ProjectRegionMultiSelect from "@features/csm-projects/components/ProjectRegionMultiSelect";

const regionsResult = { data: ["APAC", "EU"] as string[] | undefined, isLoading: false, isError: false, refetch: vi.fn() };
vi.mock("@features/csm-projects/api/useAccountRegions", () => ({
  useAccountRegions: () => regionsResult,
}));

function open(): HTMLElement {
  const input = screen.getByRole("combobox");
  fireEvent.mouseDown(input);
  return screen.getByRole("listbox");
}

describe("ProjectRegionMultiSelect", () => {
  beforeEach(() => {
    regionsResult.data = ["APAC", "EU"];
    regionsResult.isError = false;
    regionsResult.refetch.mockReset();
  });

  it("offers the looked-up regions and adds a picked one", () => {
    const onChange = vi.fn();
    render(<ProjectRegionMultiSelect id="r" values={[]} onChange={onChange} />);
    fireEvent.click(within(open()).getByText("EU"));
    expect(onChange).toHaveBeenCalledWith(["EU"]);
  });

  it("adds a typed region the lookup does not have", () => {
    const onChange = vi.fn();
    render(<ProjectRegionMultiSelect id="r" values={["APAC"]} onChange={onChange} />);
    const input = screen.getByRole("combobox");
    fireEvent.change(input, { target: { value: "LATAM" } });
    fireEvent.keyDown(input, { key: "Enter" });
    expect(onChange).toHaveBeenCalledWith(["APAC", "LATAM"]);
  });

  it("treats a selected value and a suggestion differing only in case as one checked option", () => {
    const onChange = vi.fn();
    render(<ProjectRegionMultiSelect id="r" values={["apac"]} onChange={onChange} />);
    const list = open();
    expect(within(list).queryByText("APAC")).not.toBeInTheDocument();
    const option = within(list).getByText("apac").closest("li") as HTMLElement;
    expect(within(option).getByRole("checkbox")).toBeChecked();
    fireEvent.click(option);
    expect(onChange).toHaveBeenCalledWith([]);
  });

  it("says when the lookup failed, keeps the selection removable and offers a retry", () => {
    regionsResult.data = undefined;
    regionsResult.isError = true;
    const onChange = vi.fn();
    render(<ProjectRegionMultiSelect id="r" values={["APAC"]} onChange={onChange} />);
    expect(screen.getByText(/couldn.t be loaded; you can still type one/i)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(regionsResult.refetch).toHaveBeenCalled();
    fireEvent.click(within(open()).getByText("APAC"));
    expect(onChange).toHaveBeenCalledWith([]);
  });
});
