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
import { UnknownStatusAlert } from "./UnknownStatusAlert";

const statusFixture = (status: string) => ({
  status,
  occurrenceCount: 1,
  firstSeenAt: "2026-01-01T00:00:00Z",
  lastSeenAt: "2026-01-01T00:00:00Z",
});

describe("UnknownStatusAlert", () => {
  it("renders nothing when the list is empty", () => {
    const { container } = render(<UnknownStatusAlert statuses={[]} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("renders nothing when statuses is undefined", () => {
    const { container } = render(<UnknownStatusAlert statuses={undefined} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("names the single unrecognized status", () => {
    render(<UnknownStatusAlert statuses={[statusFixture("Mystery Column")]} />);
    expect(screen.getByText("Unrecognized board status")).toBeInTheDocument();
    expect(screen.getByText(/"Mystery Column"/)).toBeInTheDocument();
  });

  it("uses plural phrasing and a count for more than one status", () => {
    render(<UnknownStatusAlert statuses={[statusFixture("A"), statusFixture("B")]} />);
    expect(screen.getByText("2 unrecognized board statuses")).toBeInTheDocument();
    expect(screen.getByText(/"A"/)).toBeInTheDocument();
    expect(screen.getByText(/"B"/)).toBeInTheDocument();
  });
});
