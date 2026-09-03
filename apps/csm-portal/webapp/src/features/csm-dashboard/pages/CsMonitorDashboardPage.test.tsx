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
import "@testing-library/jest-dom/vitest";
import type { WallboardDashboardProps } from "@features/csm-dashboard/components/WallboardDashboard";

vi.mock("@features/csm-dashboard/components/WallboardDashboard", () => ({
  default: ({ dashboardId }: WallboardDashboardProps) => (
    <div data-testid="wallboard-dashboard">{dashboardId}</div>
  ),
}));

import CsMonitorDashboardPage from "@features/csm-dashboard/pages/CsMonitorDashboardPage";

describe("CsMonitorDashboardPage", () => {
  it("renders WallboardDashboard for the fixed 'cs-overview' dashboard id, and nothing else", () => {
    const { container } = render(<CsMonitorDashboardPage />);

    expect(screen.getByTestId("wallboard-dashboard")).toHaveTextContent("cs-overview");
    // Just the one component — no header/switcher/other chrome of its own.
    expect(container.children).toHaveLength(1);
  });
});
