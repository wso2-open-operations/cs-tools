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

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import AsyncEntitySelect from "@components/AsyncEntitySelect";

interface Item {
  id: string;
  name: string;
}

const useSearch = () => ({ data: [] as Item[], isFetching: false, isError: false });

function renderSelect(required?: boolean): void {
  render(
    <AsyncEntitySelect<Item>
      id="test-select"
      label="Caller"
      value=""
      onChange={() => {}}
      required={required}
      useSearch={useSearch}
      getId={(i) => i.id}
      getLabel={(i) => i.name}
    />,
  );
}

describe("AsyncEntitySelect — required marker", () => {
  it("shows the required asterisk and marks the input required when `required` is set", () => {
    renderSelect(true);
    expect(screen.getByRole("combobox", { name: /caller/i })).toBeRequired();
    // The outlined field renders the label twice (visible label plus the
    // fieldset legend), each with its own asterisk.
    expect(screen.getAllByText("*", { exact: false }).length).toBeGreaterThan(0);
  });

  it("shows neither when `required` is not set", () => {
    renderSelect();
    expect(screen.getByRole("combobox", { name: /caller/i })).not.toBeRequired();
    expect(screen.queryByText("*", { exact: false })).not.toBeInTheDocument();
  });
});

describe("AsyncEntitySelect — pinned option", () => {
  const results: Item[] = [
    { id: "g1", name: "Platform SRE" },
    { id: "g2", name: "Network Ops" },
  ];
  const useGroupSearch = () => ({ data: results, isFetching: false, isError: false });
  const pinned = { id: "g1", label: "Platform SRE", caption: "service's support group" };

  function renderPinned(onChange = vi.fn()): ReturnType<typeof vi.fn> {
    render(
      <AsyncEntitySelect<Item>
        id="group-select"
        label="Assignment group"
        value=""
        onChange={onChange}
        useSearch={useGroupSearch}
        getId={(i) => i.id}
        getLabel={(i) => i.name}
        pinnedOption={pinned}
      />,
    );
    fireEvent.mouseDown(screen.getByRole("combobox", { name: /assignment group/i }));
    return onChange;
  }

  it("lists the pinned option first, with its caption, and not again below", () => {
    renderPinned();
    const options = screen.getAllByRole("option");
    expect(options).toHaveLength(2);
    expect(options[0]).toHaveTextContent("Platform SRE");
    expect(options[0]).toHaveTextContent("service's support group");
    expect(options[1]).toHaveTextContent("Network Ops");
  });

  it("reports the pinned id when picked", () => {
    const onChange = renderPinned();
    fireEvent.click(screen.getAllByRole("option")[0]);
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange.mock.calls[0][0]).toBe("g1");
  });

  it("drops the pinned option once the typed term no longer matches it", async () => {
    renderPinned();
    fireEvent.change(screen.getByRole("combobox", { name: /assignment group/i }), {
      target: { value: "network" },
    });
    await waitFor(() =>
      expect(screen.queryByText("service's support group")).not.toBeInTheDocument(),
    );
  });

  it("shows the external error state", () => {
    render(
      <AsyncEntitySelect<Item>
        id="group-select"
        label="Assignment group"
        value=""
        onChange={() => {}}
        useSearch={useSearch}
        getId={(i) => i.id}
        getLabel={(i) => i.name}
        error
        helperText="Refused"
      />,
    );
    expect(screen.getByRole("combobox", { name: /assignment group/i })).toHaveAttribute(
      "aria-invalid",
      "true",
    );
    expect(screen.getByText("Refused")).toBeInTheDocument();
  });
});
