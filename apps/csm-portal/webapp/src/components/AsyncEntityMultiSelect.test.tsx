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

import { act, fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import AsyncEntityMultiSelect from "@components/AsyncEntityMultiSelect";

interface Item {
  id: string;
  name: string;
}

// Nothing searched yet: a value is labelled only from `knownLabels`.
const useSearch = () => ({ data: [] as Item[], isFetching: false, isError: false });

function picker(knownLabels?: Record<string, string>) {
  return (
    <AsyncEntityMultiSelect<Item>
      id="test-multi"
      label="Assignment group"
      values={["group-1"]}
      onChange={() => {}}
      useSearch={useSearch}
      getId={(i) => i.id}
      getLabel={(i) => i.name}
      knownLabels={knownLabels}
      compactSelectedValues
    />
  );
}

describe("AsyncEntityMultiSelect — known labels", () => {
  it("labels a value from knownLabels given at mount", () => {
    render(picker({ "group-1": "Choreo SRE Team" }));
    expect(screen.getByText("Choreo SRE Team")).toBeInTheDocument();
  });

  // A caller may still be loading the labels (e.g. looking up the groups a
  // restored filter selected) when the picker mounts.
  it("labels a value from knownLabels that arrive after mount, instead of showing its id", () => {
    const { rerender } = render(picker(undefined));
    expect(screen.getByText("group-1")).toBeInTheDocument();

    rerender(picker({ "group-1": "Choreo SRE Team" }));

    expect(screen.getByText("Choreo SRE Team")).toBeInTheDocument();
    expect(screen.queryByText("group-1")).not.toBeInTheDocument();
  });

  // Regression: picking another value while a restored one's name was still
  // loading cached the restored one's raw id as its label, which then
  // outranked the name once it arrived.
  it("still takes a restored value's name that arrives after another value was picked", () => {
    const useSearchCab = () => ({
      data: [{ id: "group-2", name: "CAB Approval" }],
      isFetching: false,
      isError: false,
    });
    const onChange = vi.fn();
    const withValues = (values: string[], knownLabels?: Record<string, string>) => (
      <AsyncEntityMultiSelect<Item>
        id="test-multi"
        label="Assignment group"
        values={values}
        onChange={onChange}
        useSearch={useSearchCab}
        getId={(i) => i.id}
        getLabel={(i) => i.name}
        knownLabels={knownLabels}
        compactSelectedValues
      />
    );
    const { rerender } = render(withValues(["group-1"]));

    fireEvent.mouseDown(screen.getByRole("combobox", { name: "Assignment group" }));
    fireEvent.click(screen.getByRole("option", { name: "CAB Approval" }));
    expect(onChange).toHaveBeenCalledWith(["group-1", "group-2"]);

    rerender(withValues(["group-1", "group-2"], { "group-1": "Choreo SRE Team" }));

    expect(screen.getByText("Choreo SRE Team, CAB Approval")).toBeInTheDocument();
  });
});

describe("AsyncEntityMultiSelect — typing", () => {
  // Regression: every new search result (and every render, with inline
  // getId/getLabel) rebuilt the selected-values array, and Autocomplete
  // clears what is typed whenever its value changes identity — so the text
  // vanished as soon as the results arrived.
  it("keeps what is typed when the search results change", () => {
    let results: Item[] = [];
    const useSearchLive = () => ({ data: results, isFetching: false, isError: false });
    const ui = () => (
      <AsyncEntityMultiSelect<Item>
        id="test-multi"
        label="Groups"
        values={[]}
        onChange={() => {}}
        useSearch={useSearchLive}
        getId={(i) => i.id}
        getLabel={(i) => i.name}
      />
    );
    const { rerender } = render(ui());
    const input = screen.getByRole("combobox", { name: "Groups" });

    act(() => input.focus());
    fireEvent.mouseDown(input);
    fireEvent.change(input, { target: { value: "apim" } });
    expect(input).toHaveValue("apim");

    results = [{ id: "g1", name: "APIM1" }];
    rerender(ui());

    expect(screen.getByRole("option", { name: "APIM1" })).toBeInTheDocument();
    expect(input).toHaveValue("apim");
  });
});
