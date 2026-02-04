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
import ProjectDescription from "../ProjectDescription";

// Mock @wso2/oxygen-ui components
vi.mock("@wso2/oxygen-ui", () => ({
  Box: ({ children }: any) => <div>{children}</div>,
  Typography: ({ children }: any) => <span>{children}</span>,
  Skeleton: () => <div data-testid="skeleton" />,
}));

describe("ProjectDescription", () => {
  it("should render description text when not loading", () => {
    const description = "This is a test description.";
    render(<ProjectDescription description={description} isLoading={false} />);

    expect(screen.getByText("Description")).toBeInTheDocument();
    expect(screen.getByText(description)).toBeInTheDocument();
  });

  it("should render skeletons when loading", () => {
    render(<ProjectDescription description="test" isLoading={true} />);

    expect(screen.getByText("Description")).toBeInTheDocument();
    // Should verify skeletons are present
    const skeletons = screen.getAllByTestId("skeleton");
    expect(skeletons.length).toBeGreaterThan(0);
    // Should NOT show the description text
    expect(screen.queryByText("test")).toBeNull();
  });
});
