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

import type { ReactElement } from "react";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { describe, it, expect } from "vitest";
import { UpdateProductGrid } from "@update-cards/UpdateProductGrid";
import type { RecommendedUpdateLevelItem } from "@features/updates/types/updates";

const renderWithRouter = (ui: ReactElement) =>
  render(<MemoryRouter>{ui}</MemoryRouter>);

const mockData: RecommendedUpdateLevelItem[] = [
  {
    productName: "Product A",
    productBaseVersion: "1.0",
    startingUpdateLevel: 1,
    recommendedUpdateLevel: 2,
    installedUpdatesCount: 1,
    installedSecurityUpdatesCount: 0,
    availableUpdatesCount: 1,
    availableSecurityUpdatesCount: 0,
    channel: "full",
    timestamp: 123,
    endingUpdateLevel: 2,
  },
];

describe("UpdateProductGrid", () => {
  it("renders loading skeletons when isLoading is true", () => {
    renderWithRouter(
      <UpdateProductGrid data={undefined} isLoading={true} isError={false} />,
    );
    const skeletons =
      screen.queryAllByRole("progressbar", { hidden: true }).length === 0
        ? document.querySelectorAll(".MuiSkeleton-root")
        : screen.queryAllByRole("progressbar", { hidden: true });
    expect(skeletons.length).toBeGreaterThan(0);
  });

  it("renders error message when isError is true", () => {
    renderWithRouter(
      <UpdateProductGrid data={undefined} isLoading={false} isError={true} />,
    );
    expect(screen.getByText("Failed to load product updates.")).toBeDefined();
  });

  it("renders product cards when data is provided", () => {
    renderWithRouter(
      <UpdateProductGrid data={mockData} isLoading={false} isError={false} />,
    );
    expect(screen.getByText("Product A")).toBeDefined();
  });
});
