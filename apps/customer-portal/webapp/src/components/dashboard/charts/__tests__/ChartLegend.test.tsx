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
import { ChartLegend } from "../ChartLegend";

// Mock @wso2/oxygen-ui
vi.mock("@wso2/oxygen-ui", () => ({
  Box: ({ children, sx }: any) => <div style={sx}>{children}</div>,
  Typography: ({ children, variant }: any) => (
    <div data-testid={`typography-${variant}`}>{children}</div>
  ),
}));

describe("ChartLegend", () => {
  const mockData = [
    { name: "Item 1", value: 10, color: "#ff0000" },
    { name: "Item 2", value: 20, color: "#00ff00" },
  ];

  it("should render legend items correctly", () => {
    render(<ChartLegend data={mockData} />);

    expect(screen.getByText("Item 1")).toBeInTheDocument();
    expect(screen.getByText("Item 2")).toBeInTheDocument();
  });

  it("should render correct number of items", () => {
    const { container } = render(<ChartLegend data={mockData} />);
    // We expect 2 items, each has a color box and a text
    const colorBoxes = container.querySelectorAll(
      'div[style*="width: 12px"][style*="height: 12px"]',
    );
    expect(colorBoxes.length).toBe(2);
  });
});
