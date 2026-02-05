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
import { ChartLegend } from "@components/dashboard/charts/ChartLegend";

// Mock @wso2/oxygen-ui
vi.mock("@wso2/oxygen-ui", () => ({
  Box: ({ children, sx }: any) => {
    const style: any = { ...sx };
    if (style.bgcolor) {
      style.backgroundColor = style.bgcolor;
      delete style.bgcolor;
    }
    return <div style={style}>{children}</div>;
  },
  Typography: ({ children, variant }: any) => (
    <div data-testid={`typography-${variant}`}>{children}</div>
  ),
}));

describe("ChartLegend", () => {
  const mockData = [
    { name: "Item 1", value: 10, color: "#FF0000" },
    { name: "Item 2", value: 20, color: "#00FF00" },
  ];

  it("should render legend items correctly", () => {
    render(<ChartLegend data={mockData} />);

    expect(screen.getByText("Item 1")).toBeInTheDocument();
    expect(screen.getByText("Item 2")).toBeInTheDocument();
  });

  it("should render correct number of items with correct colors", () => {
    render(<ChartLegend data={mockData} />);

    const item1Text = screen.getByText("Item 1");
    const item1Parent = item1Text.closest("div")?.parentElement;
    const item1ColorBox = item1Parent?.querySelector(
      'div[style*="width: 12px"]',
    );

    expect(item1ColorBox).toBeInTheDocument();
    expect(item1ColorBox).toHaveStyle({ backgroundColor: "#FF0000" });

    const item2Text = screen.getByText("Item 2");
    expect(item2Text).toBeInTheDocument();
  });

  it("should handle empty data without issues", () => {
    const { container } = render(<ChartLegend data={[]} />);
    expect(container.firstChild).toBeInTheDocument();
    expect(
      container.querySelectorAll('div[style*="width: 12px"]'),
    ).toHaveLength(0);
  });
});
