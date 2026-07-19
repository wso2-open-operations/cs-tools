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

vi.mock("@features/project-details/api/usePostDeploymentProductsSearch", () => ({
  usePostDeploymentProductsSearchAll: () => ({
    data: [
      {
        id: "prod-1",
        description: null,
        product: { id: "p-1", label: "Gateway" },
        deployment: { id: "dep-1", label: "Dep 1" },
        version: "1.0.0",
      },
    ],
    isLoading: false,
    isError: false,
  }),
}));

vi.mock("@features/project-details/api/usePostDeploymentProductMetricsSearch", () => ({
  default: () => ({
    data: {
      product: { id: "prod-1", label: "Gateway" },
      summary: { dateRange: { start: "", end: "" }, totalInstances: 1, minCores: 4, maxCores: 4, avgCores: 4 },
      chartData: [],
    },
    isLoading: false,
  }),
}));

vi.mock("@features/project-details/api/usePostDeploymentProductUsageCountsSearch", () => ({
  default: () => ({
    data: {
      product: { id: "prod-1", label: "Gateway" },
      summary: {
        dateRange: { start: "", end: "" },
        countTypes: { TRANSACTION_COUNT: { aggregation: "max", min: 10, max: 10, avg: 10 } },
      },
      chartData: [],
    },
    isLoading: false,
  }),
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

    fireEvent.click(screen.getByText("Gateway 1.0.0"));
    expect(onToggleProduct).toHaveBeenCalledWith("prod-1");
  });
});
