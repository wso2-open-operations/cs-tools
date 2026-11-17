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
import { describe, expect, it } from "vitest";
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
});
