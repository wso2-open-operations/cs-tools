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
import { UsageChartSurface } from "@features/usage-metrics/components/UsageChartSurface";

vi.mock("@wso2/oxygen-ui", () => ({
  Box: ({ children }: { children?: ReactNode }) => (
    <div data-testid="chart-surface">{children}</div>
  ),
}));

describe("UsageChartSurface", () => {
  it("renders children inside the surface", () => {
    render(
      <UsageChartSurface>
        <span>chart-mock</span>
      </UsageChartSurface>,
    );
    expect(screen.getByTestId("chart-surface")).toContainElement(
      screen.getByText("chart-mock"),
    );
  });
});
