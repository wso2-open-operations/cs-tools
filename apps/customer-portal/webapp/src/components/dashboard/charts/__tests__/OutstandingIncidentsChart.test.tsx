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
import { OutstandingIncidentsChart } from "../OutstandingIncidentsChart";

// Mock @wso2/oxygen-ui
vi.mock("@wso2/oxygen-ui", () => ({
  Box: ({ children, sx }: any) => <div style={sx}>{children}</div>,
  Typography: ({ children, variant }: any) => (
    <div data-testid={`typography-${variant}`}>{children}</div>
  ),
  Card: ({ children, sx }: any) => (
    <div data-testid="card" style={sx}>
      {children}
    </div>
  ),
  Skeleton: ({ variant }: any) => (
    <div data-testid="skeleton" data-variant={variant}></div>
  ),
  colors: {
    common: { white: "#ffffff" },
    blue: { 500: "#3B82F6" },
    green: { 500: "#22C55E" },
    orange: { 500: "#F97316" },
    red: { 500: "#EF4444" },
    yellow: { 600: "#EAB308" },
  },
}));

// Mock @wso2/oxygen-ui-charts-react
vi.mock("@wso2/oxygen-ui-charts-react", () => ({
  PieChart: ({ children }: any) => (
    <div data-testid="pie-chart">{children}</div>
  ),
  Pie: ({ data, paddingAngle, minAngle, children }: any) => (
    <div
      data-testid="pie"
      data-padding-angle={paddingAngle}
      data-min-angle={minAngle}
    >
      {data.map((item: any, index: number) => (
        <div key={index} data-testid="pie-segment" data-value={item.value}>
          {item.name}
        </div>
      ))}
      {children}
    </div>
  ),
  Cell: () => <div data-testid="pie-cell" />,
  ResponsiveContainer: ({ children }: any) => (
    <div data-testid="responsive-container">{children}</div>
  ),
}));

// Mock ChartLegend
vi.mock("../ChartLegend", () => ({
  ChartLegend: ({ data }: any) => (
    <div data-testid="chart-legend">
      {data.map((item: any) => (
        <span key={item.name}>{item.name}</span>
      ))}
    </div>
  ),
}));

describe("OutstandingIncidentsChart", () => {
  const mockData = {
    medium: 5,
    high: 3,
    critical: 1,
    total: 9,
  };

  it("should render title correctly", () => {
    render(<OutstandingIncidentsChart data={mockData} isLoading={false} />);
    expect(screen.getByText("Outstanding cases")).toBeInTheDocument();
  });

  it("should render skeleton when loading", () => {
    render(<OutstandingIncidentsChart data={mockData} isLoading={true} />);
    const skeletons = screen.getAllByTestId("skeleton");
    expect(skeletons.length).toBeGreaterThan(0);
  });

  it("should render chart and legend when data is loaded", () => {
    render(<OutstandingIncidentsChart data={mockData} isLoading={false} />);
    expect(screen.getByTestId("pie-chart")).toBeInTheDocument();
    expect(screen.getByTestId("chart-legend")).toBeInTheDocument();
    expect(screen.getByText(mockData.total.toString())).toBeInTheDocument();
  });

  it("should render all segments", () => {
    render(<OutstandingIncidentsChart data={mockData} isLoading={false} />);
    const segments = screen.getAllByTestId("pie-segment");
    // We expect 3 segments (Medium, High, Critical)
    expect(segments).toHaveLength(3);
  });

  it("should render all segments even with missing data values", () => {
    const incompleteData = {
      medium: 5,
      high: 3,
      total: 8,
    } as any;

    render(
      <OutstandingIncidentsChart data={incompleteData} isLoading={false} />,
    );

    const segments = screen.getAllByTestId("pie-segment");
    expect(segments).toHaveLength(3);

    // Verify that the missing value was defaulted to 0
    const values = segments.map((s) => s.getAttribute("data-value"));
    expect(values).toContain("0");
  });

  it("should display 'N/A' when data is undefined", () => {
    render(
      <OutstandingIncidentsChart data={undefined as any} isLoading={false} />,
    );
    expect(screen.getByText("N/A")).toBeInTheDocument();
  });

  it("should have correct paddingAngle and minAngle", () => {
    render(<OutstandingIncidentsChart data={mockData} isLoading={false} />);
    const pie = screen.getByTestId("pie");
    expect(pie.getAttribute("data-padding-angle")).toBe("0");
    expect(pie.getAttribute("data-min-angle")).toBe("15");
  });
});
