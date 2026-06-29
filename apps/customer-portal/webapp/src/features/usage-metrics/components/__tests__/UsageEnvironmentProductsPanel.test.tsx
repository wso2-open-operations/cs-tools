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

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import UsageEnvironmentProductsPanel from "@features/usage-metrics/components/UsageEnvironmentProductsPanel";

vi.mock("@features/project-details/api/usePostDeploymentInstancesUsagesSearch", () => ({
  default: () => ({
    data: { usages: [] },
    isLoading: false,
    isError: false,
  }),
}));

vi.mock("@features/project-details/api/usePostDeploymentInstancesMetricsSearch", () => ({
  default: () => ({
    data: { metrics: [] },
    isLoading: false,
    isError: false,
  }),
}));

vi.mock("@features/usage-metrics/utils/usageMetricsEnvironmentProducts", () => ({
  buildUsageProductInstanceAccordionKey: (a: string, b: string) => `${a}::${b}`,
  deriveUsageEnvironmentProducts: () => [
    {
      id: "prod-1",
      name: "Gateway",
      version: "",
      runningInstances: 1,
      metricKeys: ["TRANSACTION_COUNT"],
      summaryStats: [{ label: "Transactions", value: "10" }],
      coreMetrics: [
        { label: "Total Cores", value: "4" },
        { label: "Instances", value: "1" },
      ],
      chartTrends: [],
      instances: [],
      instanceSummary: { label: "", curr: 0, min: 0, max: 0, avg: 0, trend: [] },
      coreSummary: { label: "", curr: 0, min: 0, max: 0, avg: 0, trend: [] },
    },
  ],
}));

describe("UsageEnvironmentProductsPanel", () => {
  it("renders product rows and toggles product expansion", () => {
    const onToggleProduct = vi.fn();

    render(
      <UsageEnvironmentProductsPanel
        deploymentId="dep-1"
        projectId="project-1"
        dateRange={{ startDate: "2026-01-01", endDate: "2026-01-31" }}
        expandedProductIds={new Set<string>()}
        onToggleProduct={onToggleProduct}
      />,
    );

    fireEvent.click(screen.getByText("Gateway"));
    expect(onToggleProduct).toHaveBeenCalledWith("prod-1");
  });
});

