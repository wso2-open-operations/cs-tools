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
import { describe, expect, it } from "vitest";
import { UpdatesStatsGrid } from "@components/updates/UpdatesStatsGrid";

const mockData = {
  productsTracked: 4,
  totalUpdatesInstalled: 70,
  totalUpdatesInstalledBreakdown: { regular: 50, security: 20 },
  totalUpdatesPending: 69,
  totalUpdatesPendingBreakdown: { regular: 37, security: 32 },
  securityUpdatesPending: 32,
};

describe("UpdatesStatsGrid", () => {
  it("should render Overall Update Status heading", () => {
    render(
      <UpdatesStatsGrid data={mockData} isLoading={false} isError={false} />,
    );
    expect(screen.getByText("Overall Update Status")).toBeInTheDocument();
  });

  it("should render all four stat labels when data is loaded", () => {
    render(
      <UpdatesStatsGrid data={mockData} isLoading={false} isError={false} />,
    );
    expect(screen.getByText("Products Tracked")).toBeInTheDocument();
    expect(screen.getByText("Total Updates Installed")).toBeInTheDocument();
    expect(screen.getByText("Total Updates Pending")).toBeInTheDocument();
    expect(screen.getByText("Security Updates Pending")).toBeInTheDocument();
  });

  it("should display stat values when data is loaded", () => {
    render(
      <UpdatesStatsGrid data={mockData} isLoading={false} isError={false} />,
    );
    expect(screen.getByText("4")).toBeInTheDocument();
    expect(screen.getByText("70")).toBeInTheDocument();
    expect(screen.getByText("69")).toBeInTheDocument();
    expect(screen.getByText("32")).toBeInTheDocument();
  });

  it("should display -- for null values", () => {
    const dataWithNulls = {
      ...mockData,
      productsTracked: null,
      totalUpdatesInstalled: null,
    };
    render(
      <UpdatesStatsGrid
        data={dataWithNulls}
        isLoading={false}
        isError={false}
      />,
    );
    const placeholders = screen.getAllByText("--");
    expect(placeholders.length).toBeGreaterThanOrEqual(2);
  });

  it("should not display stat values when loading (skeletons shown instead)", () => {
    render(
      <UpdatesStatsGrid data={mockData} isLoading={true} isError={false} />,
    );
    expect(screen.queryByText("4")).not.toBeInTheDocument();
    expect(screen.queryByText("70")).not.toBeInTheDocument();
  });
});
