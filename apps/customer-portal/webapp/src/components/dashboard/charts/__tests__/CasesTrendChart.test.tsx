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
import { CasesTrendChart } from "../CasesTrendChart";

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
  BarChart: ({ children }: any) => (
    <div data-testid="bar-chart">{children}</div>
  ),
  Bar: () => <div data-testid="bar-series" />,
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

describe("CasesTrendChart", () => {
  const mockData = [
    { name: "Jan", TypeA: 10, TypeB: 20, TypeC: 30, TypeD: 40 },
    { name: "Feb", TypeA: 15, TypeB: 25, TypeC: 35, TypeD: 45 },
  ];

  it("should render title correctly", () => {
    render(<CasesTrendChart data={mockData} isLoading={false} />);
    expect(screen.getByText("Cases trend")).toBeInTheDocument();
  });

  it("should render skeleton when loading", () => {
    render(<CasesTrendChart data={mockData} isLoading={true} />);
    const skeletons = screen.getAllByTestId("skeleton");
    expect(skeletons.length).toBeGreaterThan(0);
  });

  it("should render chart and legend when data is loaded", () => {
    render(<CasesTrendChart data={mockData} isLoading={false} />);
    expect(screen.getByTestId("bar-chart")).toBeInTheDocument();
    expect(screen.getByTestId("chart-legend")).toBeInTheDocument();
  });

  it("should not crash and render empty chart when data is undefined", () => {
    render(<CasesTrendChart data={undefined as any} isLoading={false} />);
    expect(screen.getByTestId("bar-chart")).toBeInTheDocument();
  });
});
