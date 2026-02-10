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
import { describe, it, expect, vi } from "vitest";
import TimeTrackingStatCards from "@components/project-details/time-tracking/TimeTrackingStatCards";

vi.mock("@components/common/stat-grid/SupportStatGrid", () => ({
  default: ({
    configs,
    stats,
    isLoading,
    isError,
  }: {
    configs: { label: string; key: string }[];
    stats: Record<string, number> | undefined;
    isLoading: boolean;
    isError?: boolean;
  }) => (
    <div data-testid="support-stat-grid">
      {isLoading && <span data-testid="loading">Loading</span>}
      {isError && <span data-testid="error">Error</span>}
      {!isLoading &&
        !isError &&
        configs.map((c) => (
          <div key={c.key}>
            <span>{c.label}</span>
            <span>{stats?.[c.key] ?? 0}</span>
          </div>
        ))}
    </div>
  ),
}));

describe("TimeTrackingStatCards", () => {
  it("should render stat grid with Total Hours, Billable Hours, Non-Billable labels and values", () => {
    const stats = {
      totalHours: 17.5,
      billableHours: 15,
      nonBillableHours: 2.5,
    };

    render(
      <TimeTrackingStatCards
        isLoading={false}
        isError={false}
        stats={stats}
      />,
    );

    expect(screen.getByTestId("support-stat-grid")).toBeInTheDocument();
    expect(screen.getByText("Total Hours")).toBeInTheDocument();
    expect(screen.getByText("Billable Hours")).toBeInTheDocument();
    expect(screen.getByText("Non-Billable")).toBeInTheDocument();
    expect(screen.getByText("17.5")).toBeInTheDocument();
    expect(screen.getByText("15")).toBeInTheDocument();
    expect(screen.getByText("2.5")).toBeInTheDocument();
  });

  it("should pass loading state to SupportStatGrid", () => {
    render(
      <TimeTrackingStatCards isLoading={true} stats={undefined} />,
    );

    expect(screen.getByTestId("loading")).toBeInTheDocument();
  });

  it("should pass error state to SupportStatGrid", () => {
    render(
      <TimeTrackingStatCards
        isLoading={false}
        isError={true}
        stats={undefined}
      />,
    );

    expect(screen.getByTestId("error")).toBeInTheDocument();
  });
});
