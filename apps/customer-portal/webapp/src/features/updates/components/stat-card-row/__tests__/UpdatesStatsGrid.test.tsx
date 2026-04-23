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
import { UpdatesStatsGrid } from "@features/updates/components/stat-card-row/UpdatesStatsGrid";
import type { RecommendedUpdateLevelItem } from "@features/updates/types/updates";

const createUpdateLevelItem = (
  overrides: Partial<RecommendedUpdateLevelItem> = {},
): RecommendedUpdateLevelItem => ({
  productName: "product",
  productBaseVersion: "1.0",
  channel: "full",
  startingUpdateLevel: 0,
  endingUpdateLevel: 10,
  installedUpdatesCount: 100,
  installedSecurityUpdatesCount: 20,
  timestamp: 0,
  recommendedUpdateLevel: 10,
  availableUpdatesCount: 50,
  availableSecurityUpdatesCount: 10,
  ...overrides,
});

const mockRecommendedUpdateLevels: RecommendedUpdateLevelItem[] = [
  createUpdateLevelItem({
    installedUpdatesCount: 200,
    installedSecurityUpdatesCount: 40,
    availableUpdatesCount: 60,
    availableSecurityUpdatesCount: 30,
  }),
  ...Array(19)
    .fill(null)
    .map(() =>
      createUpdateLevelItem({
        installedUpdatesCount: 183,
        installedSecurityUpdatesCount: 39,
        availableUpdatesCount: 55,
        availableSecurityUpdatesCount: 21,
      }),
    ),
];

// Mock StatCard to avoid deep rendering issues and focus on grid logic
vi.mock("@features/updates/components/stat-card-row/StatCard", () => ({
  StatCard: ({ label, value }: { label: string; value: string | number }) => (
    <div data-testid={`stat-card-${label}`}>
      <span data-testid="card-label">{label}</span>
      <span data-testid="card-value">{value}</span>
    </div>
  ),
}));

describe("UpdatesStatsGrid", () => {
  it("renders all stat cards with correct aggregated values", () => {
    render(
      <UpdatesStatsGrid
        data={mockRecommendedUpdateLevels}
        isLoading={false}
        isError={false}
      />,
    );

    // Check if cards are present
    expect(screen.getByTestId("stat-card-Products Tracked")).toBeDefined();
    expect(
      screen.getByTestId("stat-card-Total Updates Installed"),
    ).toBeDefined();
    expect(screen.getByTestId("stat-card-Total Updates Pending")).toBeDefined();
    expect(
      screen.getByTestId("stat-card-Security Updates Pending"),
    ).toBeDefined();

    // Verify aggregated values for 20 products
    expect(screen.getByText("20")).toBeDefined();
    expect(screen.getByText("4458")).toBeDefined();
    expect(screen.getByText("1534")).toBeDefined();
    expect(screen.getByText("429")).toBeDefined();
  });

  it("shows skeletons when loading", () => {
    render(
      <UpdatesStatsGrid data={undefined} isLoading={true} isError={false} />,
    );
    expect(screen.getAllByText("--")).toHaveLength(4);
  });

  it("renders placeholders when there is an error", () => {
    render(
      <UpdatesStatsGrid data={undefined} isLoading={false} isError={true} />,
    );

    expect(screen.getAllByText("--")).toHaveLength(4);
  });
});
