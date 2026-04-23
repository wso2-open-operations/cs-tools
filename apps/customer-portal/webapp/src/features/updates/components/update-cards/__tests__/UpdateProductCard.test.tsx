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
import { describe, it, expect } from "vitest";
import { UpdateProductCard } from "@update-cards/UpdateProductCard";
import type { RecommendedUpdateLevelItem } from "@features/updates/types/updates";

const mockItem: RecommendedUpdateLevelItem = {
  productName: "WSO2 Identity Server",
  productBaseVersion: "6.1.0",
  startingUpdateLevel: 10,
  recommendedUpdateLevel: 15,
  installedUpdatesCount: 5,
  installedSecurityUpdatesCount: 2,
  availableUpdatesCount: 3,
  availableSecurityUpdatesCount: 2,
  channel: "full",
  timestamp: 123456789,
  endingUpdateLevel: 15,
};

describe("UpdateProductCard", () => {
  it("renders product name and version", () => {
    render(<UpdateProductCard item={mockItem} />);
    expect(screen.getByText("WSO2 Identity Server")).toBeDefined();
    expect(screen.getByText("Version 6.1.0")).toBeDefined();
  });

  it("displays correct update levels", () => {
    render(<UpdateProductCard item={mockItem} />);
    expect(screen.getByText("U10")).toBeDefined();
    expect(screen.getByText("U15")).toBeDefined();
    // Pending levels is 5, but there might be multiple or it might be confusing
    // Let's check for the label and then the value if possible, or just check the count of 5s
    const pendingValue = screen.getByText("5", { selector: "h6" });
    expect(pendingValue).toBeDefined();
  });

  it("calculates and displays percentage correctly", () => {
    render(<UpdateProductCard item={mockItem} />);
    // (10/15) * 100 = 66.666... -> 67%
    expect(screen.getByText("67% Updated")).toBeDefined();
  });

  it("displays correct installed and pending counts", () => {
    render(<UpdateProductCard item={mockItem} />);
    // Installed total: 7, Pending total: 5
    expect(screen.getByText("7")).toBeDefined();
    const pendingTotal = screen
      .getAllByText("5")
      .find((el) => el.tagName === "P" || el.tagName === "SPAN");
    expect(pendingTotal).toBeDefined();
    expect(screen.getByText("5R • 2S")).toBeDefined();
    expect(screen.getByText("3R • 2S")).toBeDefined();
  });
});
