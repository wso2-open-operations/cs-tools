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
import ChartLayout from "@features/dashboard/components/charts/ChartLayout";

// Mock child components
vi.mock("../ActiveCasesChart", () => ({
  ActiveCasesChart: ({ isLoading, variant }: any) => (
    <div
      data-testid="active-cases-chart"
      data-loading={isLoading}
      data-variant={variant}
    >
      Active Cases Chart
    </div>
  ),
}));

vi.mock("../CasesTrendChart", () => ({
  CasesTrendChart: ({ isLoading, data }: any) => (
    <div
      data-testid="cases-trend-chart"
      data-loading={isLoading}
      data-onboarding={data?.onboarding}
      data-migration={data?.migration}
      data-services={data?.services}
      data-improvements={data?.improvements}
      data-total={data?.total}
    >
      Cases Trend Chart
    </div>
  ),
}));

vi.mock("../OutstandingIncidentsChart", () => ({
  OutstandingIncidentsChart: ({ isLoading }: any) => (
    <div data-testid="outstanding-cases-chart" data-loading={isLoading}>
      Outstanding cases Chart
    </div>
  ),
}));

// Mock @wso2/oxygen-ui
vi.mock("@wso2/oxygen-ui", () => ({
  Grid: ({ children, container }: any) => (
    <div data-testid={container ? "grid-container" : "grid-item"}>
      {children}
    </div>
  ),
}));

describe("ChartLayout", () => {
  const mockProps = {
    outstandingCases: {
      low: 2,
      medium: 5,
      high: 3,
      critical: 1,
      catastrophic: 0,
      total: 11,
    },
    activeCases: {
      serviceRequests: 12,
      changeRequests: 8,
      total: 20,
    },
    engagements: {
      categories: [
        { name: "Onboarding", value: 1 },
        { name: "Migration", value: 2 },
        { name: "Services", value: 3 },
        { name: "Improvements", value: 4 },
      ],
      total: 10,
    },
    isLoading: false,
  };

  it("should render all chart components", () => {
    render(<ChartLayout {...mockProps} />);

    expect(screen.getByTestId("active-cases-chart")).toBeInTheDocument();
    expect(screen.getByTestId("cases-trend-chart")).toBeInTheDocument();
    expect(screen.getByTestId("outstanding-cases-chart")).toBeInTheDocument();
  });

  it("should pass isLoading prop to child components", () => {
    render(<ChartLayout {...mockProps} isLoading={true} />);

    expect(screen.getByTestId("active-cases-chart")).toHaveAttribute(
      "data-loading",
      "true",
    );
    expect(screen.getByTestId("cases-trend-chart")).toHaveAttribute(
      "data-loading",
      "true",
    );
    expect(screen.getByTestId("outstanding-cases-chart")).toHaveAttribute(
      "data-loading",
      "true",
    );
  });

  it("should omit operations chart when showOperationsChart is false", () => {
    render(<ChartLayout {...mockProps} showOperationsChart={false} />);

    expect(screen.queryByTestId("active-cases-chart")).not.toBeInTheDocument();
    expect(screen.getByTestId("outstanding-cases-chart")).toBeInTheDocument();
    expect(screen.getByTestId("cases-trend-chart")).toBeInTheDocument();
  });

  it("should pass operationsChartMode to ActiveCasesChart", () => {
    render(
      <ChartLayout {...mockProps} operationsChartMode="srOnly" />,
    );

    expect(screen.getByTestId("active-cases-chart")).toHaveAttribute(
      "data-variant",
      "srOnly",
    );
  });

  it("should default operationsChartMode to srAndCr on ActiveCasesChart", () => {
    render(<ChartLayout {...mockProps} />);

    expect(screen.getByTestId("active-cases-chart")).toHaveAttribute(
      "data-variant",
      "srAndCr",
    );
  });
});
