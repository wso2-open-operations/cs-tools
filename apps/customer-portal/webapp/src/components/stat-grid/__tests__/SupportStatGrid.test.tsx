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
import SupportStatGrid from "@components/stat-grid/SupportStatGrid";
import { CircleAlert, Clock } from "@wso2/oxygen-ui-icons-react";

vi.mock("@wso2/oxygen-ui", async () => {
  const actual = await vi.importActual("@wso2/oxygen-ui");
  return {
    ...actual,
    StatCard: ({ label, value }: { label: string; value: any }) => (
      <div data-testid="stat-card">
        <span>{label}</span>
        <span>{value}</span>
      </div>
    ),
    Skeleton: () => <div data-testid="skeleton" />,
  };
});

describe("SupportStatGrid", () => {
  const mockConfigs = [
    {
      label: "Open Cases",
      key: "open",
      icon: CircleAlert,
      iconColor: "info" as const,
    },
    {
      label: "Active Cases",
      key: "active",
      icon: Clock,
      iconColor: "warning" as const,
    },
  ];

  const mockStats = {
    open: 10,
    active: 5,
  };

  it("should render all stat cards with correct values", () => {
    render(
      <SupportStatGrid
        isLoading={false}
        configs={mockConfigs}
        stats={mockStats}
      />,
    );

    expect(screen.getByText("Open Cases")).toBeInTheDocument();
    expect(screen.getByText("10")).toBeInTheDocument();
    expect(screen.getByText("Active Cases")).toBeInTheDocument();
    expect(screen.getByText("5")).toBeInTheDocument();
  });

  it("should render skeletons when loading", () => {
    render(
      <SupportStatGrid
        isLoading={true}
        configs={mockConfigs}
        stats={undefined}
      />,
    );

    const skeletons = screen.getAllByTestId("skeleton");
    expect(skeletons.length).toBeGreaterThan(0);
  });

  it('should handle missing stats gracefully by showing "--"', () => {
    render(
      <SupportStatGrid isLoading={false} configs={mockConfigs} stats={{}} />,
    );

    expect(screen.getAllByText("--")).toHaveLength(2);
  });
});
