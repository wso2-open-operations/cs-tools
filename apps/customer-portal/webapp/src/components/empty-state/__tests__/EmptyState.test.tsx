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
import EmptyState from "../EmptyState";

vi.mock("@components/empty-state/EmptyIcon", () => ({
  default: () => <div data-testid="empty-icon" />,
}));

describe("EmptyState", () => {
  it("should render the empty icon and the provided description", () => {
    const description = "Nothing to see here";
    render(<EmptyState description={description} />);

    expect(screen.getByTestId("empty-icon")).toBeInTheDocument();
    expect(screen.getByText(description)).toBeInTheDocument();
  });

  it("should not render an action when none is provided", () => {
    render(<EmptyState description="Nothing to see here" />);
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });

  it("should render the provided action below the description", () => {
    render(
      <EmptyState
        description="Nothing to see here"
        action={<button type="button">Do something</button>}
      />,
    );
    expect(
      screen.getByRole("button", { name: "Do something" }),
    ).toBeInTheDocument();
  });

  it("should render a secondary description when provided", () => {
    render(
      <EmptyState
        description="Nothing to see here"
        secondaryDescription="More context about why"
      />,
    );
    expect(screen.getByText("More context about why")).toBeInTheDocument();
  });

  it("should not render a secondary description when not provided", () => {
    render(<EmptyState description="Nothing to see here" />);
    expect(screen.queryByText("More context about why")).not.toBeInTheDocument();
  });
});
