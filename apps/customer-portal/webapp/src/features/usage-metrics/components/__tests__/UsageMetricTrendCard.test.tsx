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

import type { ReactNode } from "react";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import UsageMetricTrendCard from "@features/usage-metrics/components/UsageMetricTrendCard";
import type { UsageAggregatedMetricDefinition } from "@features/project-details/types/usage";

vi.mock("@wso2/oxygen-ui-charts-react", () => ({
  LineChart: () => <div data-testid="line-chart" />,
}));

vi.mock("@wso2/oxygen-ui", () => ({
  Box: ({ children }: { children?: ReactNode }) => <div>{children}</div>,
  Card: ({ children }: { children?: ReactNode }) => (
    <div data-testid="metric-card">{children}</div>
  ),
  Typography: ({ children }: { children?: ReactNode }) => (
    <span>{children}</span>
  ),
  colors: { green: { 600: "#16A34A" }, grey: { 400: "#9CA3AF" } },
}));

describe("UsageMetricTrendCard", () => {
  const metric: UsageAggregatedMetricDefinition = {
    id: "m1",
    title: "Total Widgets",
    caption: "All regions",
    headlineValue: "42",
    deltaLabel: "+1.0%",
    stroke: "#EA580C",
    data: [
      { name: "Jan", value: 10 },
      { name: "Feb", value: 12 },
    ],
  };

  it("renders headline and chart placeholder", () => {
    render(<UsageMetricTrendCard metric={metric} />);
    expect(screen.getByText("Total Widgets")).toBeInTheDocument();
    expect(screen.getByText("42")).toBeInTheDocument();
    expect(screen.getByTestId("line-chart")).toBeInTheDocument();
  });
});
