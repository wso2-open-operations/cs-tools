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
import UsageOverviewPanel from "@features/usage-metrics/components/UsageOverviewPanel";

vi.mock("@features/usage-metrics/components/UsageMetricTrendCard", () => ({
  default: () => <div data-testid="usage-trend-card" />,
}));

vi.mock("@api/usePostProjectDeploymentsSearch", () => ({
  usePostProjectDeploymentsSearchAll: () => ({
    data: [{ id: "dep-1", name: "Prod", type: { id: "k8s" } }],
    isLoading: false,
    isError: false,
  }),
}));

vi.mock("@features/project-details/api/usePostProjectInstancesSearch", () => ({
  default: () => ({
    data: { instances: [] },
    isLoading: false,
    isError: false,
  }),
}));

vi.mock("@features/project-details/api/usePostProjectInstancesUsagesSearch", () => ({
  default: () => ({
    data: { usages: [] },
    isLoading: false,
    isError: false,
  }),
}));

vi.mock("@features/project-details/api/usePostProjectInstancesMetricsSearch", () => ({
  default: () => ({
    data: { metrics: [] },
    isLoading: false,
    isError: false,
  }),
}));

vi.mock("@features/project-details/api/useGetProjectUsageStats", () => ({
  default: () => ({
    data: { deploymentCount: 1, deployedProductCount: 2, instanceCount: 3 },
    isLoading: false,
    isError: false,
  }),
}));

vi.mock("@features/project-details/api/usePostDeploymentInstancesSearch", () => ({
  default: () => ({
    data: { instances: [] },
    isLoading: false,
    isError: false,
  }),
}));

vi.mock("@features/project-details/api/usePostDeploymentInstancesUsagesSearch", () => ({
  default: () => ({
    data: { usages: [] },
    isLoading: false,
    isError: false,
  }),
}));

describe("UsageOverviewPanel", () => {
  it("renders environment rows and calls toggle handler", () => {
    const onToggleEnvironment = vi.fn();

    render(
      <UsageOverviewPanel
        projectId="project-1"
        dateRange={{ startDate: "2026-01-01", endDate: "2026-01-31" }}
        expandedEnvironmentIds={new Set<string>()}
        onToggleEnvironment={onToggleEnvironment}
      />,
    );

    fireEvent.click(screen.getByText("Prod"));
    expect(onToggleEnvironment).toHaveBeenCalledWith("env-dep-1");
  });
});

