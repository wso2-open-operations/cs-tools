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
import { CasesTrendChart } from "@features/dashboard/components/charts/CasesTrendChart";

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
    common: { white: "#FFFFFF" },
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
        <span key={item.name}>{`${item.name}:${item.value}`}</span>
      ))}
    </div>
  ),
}));

describe("CasesTrendChart", () => {
  const baseData = {
    categories: [
      { name: "Onboarding", value: 12 },
      { name: "Migration", value: 8 },
      { name: "Services", value: 15 },
      { name: "Improvements", value: 10 },
    ],
    total: 45,
  };

  it("should render title correctly", () => {
    render(<CasesTrendChart data={baseData} isLoading={false} />);
    expect(screen.getByText(/Outstanding Engagements/i)).toBeInTheDocument();
  });

  it("should render skeleton when loading", () => {
    render(<CasesTrendChart data={baseData} isLoading={true} />);
    const skeletons = screen.getAllByTestId("skeleton");
    expect(skeletons.length).toBeGreaterThan(0);
  });

  it("should render pie chart and legend when data is loaded", () => {
    render(<CasesTrendChart data={baseData} isLoading={false} />);
    expect(screen.getByTestId("pie-chart")).toBeInTheDocument();
    expect(screen.getByTestId("chart-legend")).toBeInTheDocument();
  });

  it("should render error state correctly", () => {
    render(
      <CasesTrendChart data={baseData} isLoading={false} isError={true} />,
    );

    expect(screen.getByTestId("pie-chart")).toBeInTheDocument();
    const totalTypography = screen.getByTestId("typography-h4");
    expect(totalTypography).toHaveTextContent("--");
  });

  it("should display correct total in center from data.total", () => {
    render(<CasesTrendChart data={baseData} isLoading={false} />);
    const totalTypography = screen.getByTestId("typography-h4");
    expect(totalTypography).toHaveTextContent("45");
  });
});
