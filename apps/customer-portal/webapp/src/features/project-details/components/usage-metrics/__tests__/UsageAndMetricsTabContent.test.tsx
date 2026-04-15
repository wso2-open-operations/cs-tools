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
import UsageAndMetricsTabContent from "@features/project-details/components/usage-metrics/UsageAndMetricsTabContent";

vi.mock("@features/project-details/components/usage-metrics/UsageOverviewPanel", () => ({
  default: () => <div data-testid="usage-overview-panel" />,
}));

vi.mock(
  "@features/project-details/components/usage-metrics/UsageEnvironmentProductsPanel",
  () => ({
    default: () => <div data-testid="usage-environment-products-panel" />,
  }),
);

describe("UsageAndMetricsTabContent", () => {
  it("shows overview panel by default and switches to environment products", () => {
    render(<UsageAndMetricsTabContent />);

    expect(screen.getByText("Time Range:")).toBeInTheDocument();
    expect(screen.getByTestId("usage-overview-panel")).toBeInTheDocument();
    expect(
      screen.queryByTestId("usage-environment-products-panel"),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("tab", { name: "Production" }));

    expect(screen.queryByTestId("usage-overview-panel")).not.toBeInTheDocument();
    expect(screen.getByTestId("usage-environment-products-panel")).toBeInTheDocument();
  });
});
